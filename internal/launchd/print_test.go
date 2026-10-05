package launchd

import (
	"testing"
	"time"

	"github.com/A404coder/deployboard/internal/inventory"
)

func TestParsePrintOutput_Running(t *testing.T) {
	out := `
		state = running
		pid = 73087
		runs = 28
		last exit code = 0
		path = /Users/example/Library/LaunchAgents/com.example.app.uat.api.plist
	`
	info, err := ParsePrintOutput(out)
	if err != nil {
		t.Fatalf("ParsePrintOutput: %v", err)
	}
	if !info.Running() {
		t.Error("Running() should be true")
	}
	if info.PID != 73087 || info.Runs != 28 || !info.HasExit || info.LastExitCode != 0 {
		t.Errorf("running fields: %+v", info)
	}
	if info.PlistPath != "/Users/example/Library/LaunchAgents/com.example.app.uat.api.plist" {
		t.Errorf("PlistPath: %q", info.PlistPath)
	}
}

func TestParsePrintOutput_NeverExited(t *testing.T) {
	out := `
		state = running
		pid = 813
		runs = 1
		path = /Users/example/Library/LaunchAgents/com.example.web.web.plist
	`
	info, err := ParsePrintOutput(out)
	if err != nil {
		t.Fatalf("ParsePrintOutput: %v", err)
	}
	if !info.Running() {
		t.Error("Running() should be true")
	}
	if info.HasExit {
		t.Error("HasExit should be false when no 'last exit code' line")
	}
	if info.Runs != 1 || info.PID != 813 {
		t.Errorf("fields: %+v", info)
	}
}

func TestParsePrintOutput_NotRunningWithExitCode(t *testing.T) {
	out := `
		state = not running
		pid = 0
		runs = 12207
		last exit code = 1
	`
	info, err := ParsePrintOutput(out)
	if err != nil {
		t.Fatalf("ParsePrintOutput: %v", err)
	}
	if info.Running() {
		t.Error("Running() should be false")
	}
	if info.Runs != 12207 || !info.HasExit || info.LastExitCode != 1 {
		t.Errorf("fields: %+v", info)
	}
}

func TestParsePrintOutput_NotLoaded(t *testing.T) {
	out := `Could not find service "gui/501/com.missing"`
	_, err := ParsePrintOutput(out)
	if err != ErrNotLoaded {
		t.Errorf("expected ErrNotLoaded, got: %v", err)
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
	disabled := ParseDisabledOutput(out)
	if !disabled["com.example.worker.run"] || !disabled["com.example.worker.bot"] {
		t.Errorf("disabled labels missing: %v", disabled)
	}
	if disabled["com.example.infra.one"] {
		t.Error("enabled label should not be in disabled map")
	}
}

func TestApplyDisabledStatus(t *testing.T) {
	job := &Job{
		Label:    "com.test",
		Status:   StatusError,
		Disabled: true,
		PID:      0,
	}
	applyDisabledStatus(job)
	if job.Status != StatusDisabled {
		t.Errorf("disabled + no pid -> disabled, got %s", job.Status)
	}

	job2 := &Job{
		Label:    "com.test2",
		Status:   StatusError,
		Disabled: true,
		PID:      123,
	}
	applyDisabledStatus(job2)
	if job2.Status != StatusError {
		t.Errorf("disabled + pid -> should stay error, got %s", job2.Status)
	}

	job3 := &Job{
		Label:    "com.test3",
		Status:   StatusStopped,
		Disabled: false,
		PID:      0,
	}
	applyDisabledStatus(job3)
	if job3.Status != StatusStopped {
		t.Errorf("not disabled -> should stay stopped, got %s", job3.Status)
	}
}

func TestService_Enrich_RespectsNoise(t *testing.T) {
	svc := NewServiceWithWindow(10 * time.Minute)
	svc.SetEnrichment(inventory.New(inventory.Config{Ours: []string{"com.ours.*"}, Hidden: []string{"com.noise.*"}}), 15*time.Second)

	jobs := []Job{
		{Label: "com.ours.api", Status: "running", PID: 1},
		{Label: "com.noise.service", Status: "running", PID: 2},
	}
	svc.enrich(jobs)

	if jobs[0].Category != "ours" {
		t.Errorf("ours job category = %s, want ours", jobs[0].Category)
	}
	if jobs[0].Runs == 0 && jobs[0].PrintState == "" {
		// no print runner injected, so no enrichment — this is fine for the test
	}
	if jobs[1].Category != "noise" {
		t.Errorf("noise job category = %s, want noise", jobs[1].Category)
	}
	// noise jobs should NOT have Runs/PrintState populated (no print runner)
	if jobs[1].Runs != 0 || jobs[1].PrintState != "" {
		t.Errorf("noise job should not be printed: Runs=%d PrintState=%q", jobs[1].Runs, jobs[1].PrintState)
	}
}
