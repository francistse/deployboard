package alerts

import (
	"os"
	"testing"
	"time"

	"github.com/A404coder/deployboard/internal/launchd"
)

func TestLoadState_Empty(t *testing.T) {
	s, err := LoadState("")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if s.Labels == nil {
		t.Error("Labels should not be nil")
	}
}

func TestState_SetEnabled_PersistsExplicit(t *testing.T) {
	s := NewState("")
	ls := s.SetEnabled("com.test", true)
	if !ls.Enabled || ls.Explicit == nil || !*ls.Explicit {
		t.Errorf("SetEnabled should mark explicit: %+v", ls)
	}
	ls2 := s.Get("com.test")
	if ls2.Explicit == nil || !*ls2.Explicit {
		t.Error("explicit should persist in state")
	}
}

func TestState_SetEnabled_FalseThenTrue(t *testing.T) {
	s := NewState("")
	s.SetEnabled("com.test", false)
	ls := s.Get("com.test")
	if ls.Enabled || ls.Explicit == nil || *ls.Explicit {
		t.Errorf("explicit false should be stored: %+v", ls)
	}
	s.SetEnabled("com.test", true)
	ls = s.Get("com.test")
	if !ls.Enabled || ls.Explicit == nil || !*ls.Explicit {
		t.Errorf("explicit true should overwrite: %+v", ls)
	}
}

func TestState_SaveLoadRoundTrip(t *testing.T) {
	tmp := "alerts_test.json"
	defer func() { _ = os.Remove(tmp) }()

	s := NewState(tmp)
	s.SetEnabled("a", true)
	s.SetEnabled("b", false)
	s.SetEnabled("c", true) // explicit true
	// "d" is non-explicit: do not call SetEnabled
	ls := s.Get("d")
	ls.Enabled = true
	ls.LastStatus = "running"
	s.Set("d", ls)

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, err := LoadState(tmp)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if explicit, set := s2.Get("a").ExplicitEnable(); !set || !explicit {
		t.Errorf("a explicit true lost: %+v", s2.Get("a"))
	}
	if explicit, set := s2.Get("b").ExplicitEnable(); !set || explicit {
		t.Errorf("b explicit false lost: %+v", s2.Get("b"))
	}
	if explicit, set := s2.Get("c").ExplicitEnable(); !set || !explicit {
		t.Errorf("c explicit true lost: %+v", s2.Get("c"))
	}
	if _, set := s2.Get("d").ExplicitEnable(); set {
		t.Errorf("d should not be explicit: %+v", s2.Get("d"))
	}
}

func TestLabelState_Muted(t *testing.T) {
	now := time.Now()
	ls := LabelState{MutedUntil: nil}
	if ls.Muted(now) {
		t.Error("nil MutedUntil should not be muted")
	}
	future := now.Add(time.Hour).Unix()
	ls.MutedUntil = &future
	if !ls.Muted(now) {
		t.Error("future MutedUntil should be muted")
	}
	past := now.Add(-time.Hour).Unix()
	ls.MutedUntil = &past
	if ls.Muted(now) {
		t.Error("past MutedUntil should not be muted")
	}
}

func TestParseDisabledOutput(t *testing.T) {
	out := `
		disabled services = {
			"com.example.worker.run" => disabled
			"com.example.worker.bot" => disabled
			"com.example.infra.one" => enabled
		}
	`
	disabled := launchd.ParseDisabledOutput(out)
	if !disabled["com.example.worker.run"] || !disabled["com.example.worker.bot"] {
		t.Errorf("disabled labels not parsed: %v", disabled)
	}
	if disabled["com.example.infra.one"] {
		t.Error("enabled label should not be in disabled map")
	}
}

func TestParsePrintOutput(t *testing.T) {
	running := `
		state = running
		pid = 1234
		runs = 42
		last exit code = 0
		path = /Users/foo/Library/LaunchAgents/com.test.plist
	`
	info, err := launchd.ParsePrintOutput(running)
	if err != nil {
		t.Fatalf("ParsePrintOutput running: %v", err)
	}
	if !info.Running() {
		t.Error("Running() should be true")
	}
	if info.PID != 1234 || info.Runs != 42 || !info.HasExit || info.LastExitCode != 0 {
		t.Errorf("running parse mismatch: %+v", info)
	}

	neverExited := `
		state = running
		pid = 5678
		runs = 1
		path = /Users/foo/Library/LaunchAgents/com.test.plist
	`
	info2, err := launchd.ParsePrintOutput(neverExited)
	if err != nil {
		t.Fatalf("ParsePrintOutput never exited: %v", err)
	}
	if !info2.Running() {
		t.Error("Running() should be true")
	}
	if info2.HasExit || info2.Runs != 1 {
		t.Errorf("never exited parse mismatch: %+v", info2)
	}

	notLoaded := `
	Could not find service "gui/501/com.missing"
`
	_, err = launchd.ParsePrintOutput(notLoaded)
	if err != launchd.ErrNotLoaded {
		t.Errorf("not loaded should return ErrNotLoaded, got: %v", err)
	}
}

func TestEngine_EffectiveEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultEnabled = nil // use category defaults

	s := NewState("")
	e := NewEngine(cfg, s)

	tests := []struct {
		name   string
		job    JobState
		want   bool
		reason string
	}{
		{"ours default on", JobState{Label: "a", Category: "ours", Disabled: false, Status: "running"}, true, ""},
		{"other default off", JobState{Label: "b", Category: "other", Disabled: false, Status: "running"}, false, ""},
		{"noise always off", JobState{Label: "c", Category: "noise", Disabled: false, Status: "running"}, false, ""},
		{"disabled always off", JobState{Label: "d", Category: "ours", Disabled: true, Status: "disabled"}, false, ""},
		{"explicit on overrides other", JobState{Label: "e", Category: "other", Disabled: false, Status: "running"}, true, "explicit on"},
		{"explicit off overrides ours", JobState{Label: "f", Category: "ours", Disabled: false, Status: "running"}, false, "explicit off"},
	}

	// Seed explicit state
	s.SetEnabled("e", true)
	s.SetEnabled("f", false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := e.effectiveEnabled(tt.job); got != tt.want {
				t.Errorf("%s: effectiveEnabled = %v, want %v (%s)", tt.name, got, tt.want, tt.reason)
			}
		})
	}
}

func TestEngine_RunStormThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RunStormDelta = 25
	cfg.CooldownSeconds = 0
	cfg.Quiet.Start = ""
	s := NewState("")

	prev := LabelState{LastStatus: "running", LastRuns: 100}
	s.Set("storm.test", prev)

	// Just at threshold - should fire (delta = 25, need >= 25)
	job := JobState{Label: "storm.test", Category: "ours", Status: "running", Runs: 125, Disabled: false}
	if _, _, fire := classify(cfg, job, prev, time.Now(), false); !fire {
		t.Error("run storm at exactly delta should fire")
	}

	// Below threshold - should NOT fire
	job2 := JobState{Label: "storm.test", Category: "ours", Status: "running", Runs: 124, Disabled: false}
	if _, _, fire := classify(cfg, job2, prev, time.Now(), false); fire {
		t.Error("run storm below delta should not fire")
	}
}

func TestEngine_CooldownSuppresses(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 900
	cfg.RunStormDelta = 1000 // disable run storm
	cfg.Quiet.Start = ""
	s := NewState("")

	now := time.Now()
	prev := LabelState{LastStatus: "error", LastRuns: 0, LastSentAt: now.Unix() - 600} // sent 10 min ago
	s.Set("cool.test", prev)

	job := JobState{Label: "cool.test", Category: "ours", Status: "error", Runs: 0, Disabled: false}
	if _, _, fire := classify(cfg, job, prev, now, false); fire {
		t.Error("cooldown should suppress repeat error")
	}
}

func TestEngine_RecoveryNotSuppressedByCooldown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 900
	cfg.NotifyRecovery = true
	cfg.Quiet.Start = ""
	s := NewState("")

	now := time.Now()
	prev := LabelState{LastStatus: "error", LastRuns: 0, LastSentAt: now.Unix() - 600}
	s.Set("recover.test", prev)

	job := JobState{Label: "recover.test", Category: "ours", Status: "running", Runs: 0, Disabled: false}
	kind, _, fire := classify(cfg, job, prev, now, false)
	if !fire || kind != KindRecovery {
		t.Errorf("recovery should fire even in cooldown: kind=%s fire=%v", kind, fire)
	}
}

func TestEngine_QuietHoursSuppressNew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 0
	cfg.Quiet.Start = "23:00"
	cfg.Quiet.End = "08:00"
	cfg.Quiet.Timezone = "UTC"
	s := NewState("")

	// 02:00 UTC = in quiet hours
	now := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	prev := LabelState{LastStatus: "running", LastRuns: 0}
	s.Set("quiet.test", prev)

	job := JobState{Label: "quiet.test", Category: "ours", Status: "error", Runs: 0, Disabled: false}
	_, _, fire := classify(cfg, job, prev, now, true)
	if fire {
		t.Error("quiet hours should suppress new alerts")
	}
}

func TestEngine_QuietHoursAllowRecovery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 0
	cfg.NotifyRecovery = true
	cfg.Quiet.Start = "23:00"
	cfg.Quiet.End = "08:00"
	cfg.Quiet.Timezone = "UTC"
	s := NewState("")

	now := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	prev := LabelState{LastStatus: "error", LastRuns: 0}
	s.Set("quiet.test", prev)

	job := JobState{Label: "quiet.test", Category: "ours", Status: "running", Runs: 0, Disabled: false}
	kind, _, fire := classify(cfg, job, prev, now, true)
	if !fire || kind != KindRecovery {
		t.Errorf("recovery should fire in quiet hours: kind=%s fire=%v", kind, fire)
	}
}

func TestEngine_ProbeFlipUnreachable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 0
	cfg.Quiet.Start = ""
	s := NewState("")

	prev := LabelState{LastStatus: "running", LastRuns: 0, LastProbeOK: func() *bool { b := true; return &b }()}
	s.Set("probe.test", prev)

	now := time.Now()
	job := JobState{Label: "probe.test", Category: "ours", Status: "running", Runs: 0, Disabled: false, HasProbe: true, ProbeOK: false}
	kind, _, fire := classify(cfg, job, prev, now, false)
	if !fire || kind != KindUnreachable {
		t.Errorf("probe flip should fire unreachable: kind=%s fire=%v", kind, fire)
	}
}

func TestEngine_ProbeFlipRecovery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CooldownSeconds = 0
	cfg.NotifyRecovery = true
	cfg.Quiet.Start = ""
	s := NewState("")

	prev := LabelState{LastStatus: "running", LastRuns: 0, LastProbeOK: func() *bool { b := false; return &b }()}
	s.Set("probe.test", prev)

	now := time.Now()
	job := JobState{Label: "probe.test", Category: "ours", Status: "running", Runs: 0, Disabled: false, HasProbe: true, ProbeOK: true}
	kind, _, fire := classify(cfg, job, prev, now, false)
	if !fire || kind != KindRecovery {
		t.Errorf("probe recovery should fire: kind=%s fire=%v", kind, fire)
	}
}
