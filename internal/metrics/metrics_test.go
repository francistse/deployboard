package metrics

import (
	"strings"
	"testing"
)

func TestEscapeLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "abc", "abc"},
		{"backslash", `a\b`, `a\\b`},
		{"quote", `a"b`, `a\"b`},
		{"newline", "a\nb", "a\\nb"},
		{"all", `a\b"c\nd`, `a\\b\"c\\nd`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeLabel(tt.in); got != tt.want {
				t.Errorf("EscapeLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRender_ValidExposition(t *testing.T) {
	in := Input{
		Version: "test-1.0",
		Jobs: []JobMetric{
			{
				Label: "com.test.a", Category: "ours", Group: "G1", Status: "running",
				Runs: 5, HasExit: true, LastExitCode: 0, Disabled: false, AlertEnabled: true,
				HasProbe: true, ProbePort: 8080, ProbeOK: true, ProbeStatus: 200, ProbeLatencyS: 0.012,
			},
			{
				Label: "com.test.b", Category: "noise", Group: "", Status: "stopped",
				Runs: 0, HasExit: false, Disabled: false, AlertEnabled: false,
			},
			{
				Label: "com.test.c", Category: "ours", Group: "G1", Status: "disabled",
				Runs: 120, HasExit: true, LastExitCode: 1, Disabled: true, AlertEnabled: false,
			},
		},
	}
	out := Render(in)

	// Basic structure checks
	if !strings.HasPrefix(out, "# HELP ") {
		t.Errorf("missing HELP at start: %q", out[:100])
	}
	if !strings.Contains(out, "# TYPE ") {
		t.Error("missing TYPE lines")
	}

	// Single trailing newline
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Errorf("must end with exactly one newline: ends with %q", out[len(out)-10:])
	}

	// Families sorted
	families := strings.Split(out, "\n\n")
	if len(families) < 2 {
		t.Error("expected multiple families")
	}

	// Check specific metric families exist
	expectedFamilies := []string{
		"deployboard_up",
		"deployboard_scrape_duration_seconds",
		"deployboard_jobs_total",
		"deployboard_job_up",
		"deployboard_job_runs",
		"deployboard_job_disabled",
		"deployboard_job_last_exit_code",
		"deployboard_alerts_enabled",
		"deployboard_job_probe_reachable",
		"deployboard_job_probe_status",
		"deployboard_job_probe_latency_seconds",
		"deployboard_version",
	}
	for _, fam := range expectedFamilies {
		if !strings.Contains(out, fam) {
			t.Errorf("missing family %s", fam)
		}
	}

	// No duplicate series (labelBlock must be unique per family)
	lines := strings.Split(out, "\n")
	seen := map[string]int{}
	for _, l := range lines {
		if strings.HasPrefix(l, "deployboard_") && strings.Contains(l, "{") {
			if seen[l] > 0 {
				t.Errorf("duplicate series: %s", l)
			}
			seen[l]++
		}
	}

	// Check label escaping works for a value with quote in group name
	escaped := EscapeLabel(`a"b`)
	if !strings.Contains(escaped, `\"`) {
		t.Error("quotes in labels should be escaped")
	}
}

func TestRender_NoProbeOmitsProbeFamilies(t *testing.T) {
	in := Input{
		Jobs: []JobMetric{
			{Label: "a", Category: "ours", Status: "running", HasProbe: false},
		},
	}
	out := Render(in)
	if strings.Contains(out, "deployboard_job_probe_") {
		t.Error("probe families should be absent when HasProbe=false")
	}
}

func TestRender_DisabledJobNoExit(t *testing.T) {
	in := Input{
		Jobs: []JobMetric{
			{Label: "disabled", Category: "ours", Status: "disabled", Disabled: true, HasExit: false},
		},
	}
	out := Render(in)
	if strings.Contains(out, "deployboard_job_last_exit_code") {
		t.Error("last_exit_code should be omitted when HasExit=false")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"}, {1, "1"}, {1.5, "1.5"}, {1.123456789, "1.123457"},
		{1.100000, "1.1"}, {1e-7, "0"}, // %.6f rounds 1e-7 to 0.000000 -> trims to "0"
	}
	for _, tt := range tests {
		if got := formatValue(tt.in); got != tt.want {
			t.Errorf("formatValue(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBoolVal(t *testing.T) {
	if got := boolVal(true); got != 1 {
		t.Errorf("boolVal(true) = %v, want 1", got)
	}
	if got := boolVal(false); got != 0 {
		t.Errorf("boolVal(false) = %v, want 0", got)
	}
}
