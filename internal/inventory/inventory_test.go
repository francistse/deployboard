package inventory

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		label    string
		want     bool
	}{
		{"exact match", []string{"com.example.app.uat.api"}, "com.example.app.uat.api", true},
		{"wildcard suffix", []string{"com.example.app.*"}, "com.example.app.uat.api", true},
		{"wildcard middle", []string{"com.*.api"}, "com.example.web.api", true},
		{"no match", []string{"com.example.app.*"}, "com.example.web.api", false},
		{"empty patterns", []string{}, "anything", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.patterns, tt.label); got != tt.want {
				t.Errorf("Match(%v, %q) = %v, want %v", tt.patterns, tt.label, got, tt.want)
			}
		})
	}
}

func TestClassifier_Classify(t *testing.T) {
	cfg := Config{
		Ours:   []string{"com.example.app.*", "com.example.web.*"},
		Hidden: []string{"com.apple.*", "application.*"},
	}
	cl := New(cfg)

	tests := []struct {
		name  string
		label string
		want  Category
	}{
		{"ours via wildcard", "com.example.app.uat.api", CategoryOurs},
		{"ours bare", "com.example.web.web", CategoryOurs},
		{"hidden com.apple", "com.apple.Safari", CategoryNoise},
		{"hidden application", "application.io.tailscale", CategoryNoise},
		{"other", "com.random.service", CategoryOther},
		{"empty label", "", CategoryOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cl.Classify(tt.label); got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}

func TestClassifier_GroupName(t *testing.T) {
	cfg := Config{
		Groups: []Group{
			{Name: "Example App", Match: []string{"com.example.app.*"}},
			{Name: "AI Feng Shui", Match: []string{"com.example.web.*"}},
		},
	}
	cl := New(cfg)

	tests := []struct {
		name  string
		label string
		want  string
	}{
		{"first match", "com.example.app.api", "Example App"},
		{"second match", "com.example.web.web", "AI Feng Shui"},
		{"no match", "com.random.service", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cl.GroupName(tt.label); got != tt.want {
				t.Errorf("GroupName(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}

func TestClassifier_BuildReport(t *testing.T) {
	cfg := Config{
		Groups: []Group{
			{Name: "App", Match: []string{"com.example.app.*"}},
			{Name: "Web", Match: []string{"com.example.web.*"}},
		},
		Ours:   []string{"com.example.app.*", "com.example.web.*"},
		Hidden: []string{"com.apple.*"},
	}
	cl := New(cfg)

	labels := []string{"com.example.app.uat.api", "com.example.app.demo.web", "com.example.web.api", "com.apple.Safari", "com.random.thing"}
	rep := cl.BuildReportLabels(labels)

	if rep.Ours != 3 {
		t.Errorf("Ours = %d, want 3", rep.Ours)
	}
	if rep.Noise != 1 {
		t.Errorf("Noise = %d, want 1", rep.Noise)
	}
	if rep.Other != 1 {
		t.Errorf("Other = %d, want 1", rep.Other)
	}
	if len(rep.Groups) != 2 {
		t.Errorf("Groups = %d, want 2", len(rep.Groups))
	}
	for _, g := range rep.Groups {
		if g.Name == "App" && g.Count != 2 {
			t.Errorf("App count = %d, want 2", g.Count)
		}
		if g.Name == "Web" && g.Count != 1 {
			t.Errorf("Web count = %d, want 1", g.Count)
		}
	}
	// unmatched should only contain non-apple other
	if len(rep.Unmatched) != 1 || rep.Unmatched[0] != "com.random.thing" {
		t.Errorf("Unmatched = %v, want [com.random.thing]", rep.Unmatched)
	}
}

func TestDefaultConfig_RestartWarnThreshold(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RestartWarnThreshold != 50 {
		t.Errorf("DefaultConfig.RestartWarnThreshold = %d, want 50", cfg.RestartWarnThreshold)
	}
	// `ours` is intentionally empty (path derivation covers the common case),
	// but the denylist and the derive roots must be populated.
	if len(cfg.Hidden) == 0 || len(cfg.Groups) == 0 || len(cfg.DeriveRoots) == 0 {
		t.Errorf("DefaultConfig should populate hidden, groups and derive roots")
	}
}

func TestNew_DefaultsForEmptyConfig(t *testing.T) {
	cl := New(Config{})
	if len(cl.Config().DeriveRoots) == 0 {
		t.Error("New(Config{}) should populate DeriveRoots from DefaultConfig")
	}
	if len(cl.Config().Hidden) == 0 {
		t.Error("New(Config{}) should populate the vendor denylist from DefaultConfig")
	}
	if cl.RestartWarnThreshold() != 50 {
		t.Error("New(Config{}) should default RestartWarnThreshold")
	}
}
