// Package metrics renders the fork's Prometheus exposition.
//
// Nothing else in this niche exposes launchd state as time series (the six
// comparable projects — launch-pilot, ground-control, launchd-svc-panel,
// mac-agents-manager-ai, justinpaulson/status, agent-deck — all have zero
// mentions of prometheus//metrics/grafana). That is the point of these metrics:
// a panel can show "down now", only a series shows "started churning at 03:10".
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// JobMetric is one launchd job as exposed to Prometheus.
type JobMetric struct {
	Label         string
	Category      string
	Group         string
	Status        string
	Runs          int
	HasExit       bool
	LastExitCode  int
	Disabled      bool
	AlertEnabled  bool
	ProbePort     int
	ProbeOK       bool
	ProbeStatus   int
	ProbeLatencyS float64
	HasProbe      bool
}

// Input is everything the exposition needs.
type Input struct {
	Version  string
	Jobs     []JobMetric
	Duration time.Duration
}

// EscapeLabel escapes a Prometheus label value: backslash, double quote, newline.
func EscapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}

type sample struct {
	name   string
	help   string
	typ    string
	labels []string
	value  float64
}

// Render produces a valid Prometheus text exposition (v0.0.4): one HELP+TYPE per
// family, families sorted by name, exactly one blank line between families, and
// a single trailing newline. Series are deduplicated before output.
func Render(in Input) string {
	var samples []sample

	byStatus := map[string]int{}
	byCategory := map[string]int{}
	for _, j := range in.Jobs {
		byStatus[j.Status]++
		if j.Category != "" {
			byCategory[j.Category]++
		}
	}

	// Bounded-cardinality aggregates first.
	statusKeys := sortedKeys(byStatus)
	for _, k := range statusKeys {
		samples = append(samples, sample{"deployboard_jobs_total", "Number of launchd jobs by status.", "gauge",
			[]string{"status", k}, float64(byStatus[k])})
	}
	catKeys := sortedKeys(byCategory)
	for _, k := range catKeys {
		samples = append(samples, sample{"deployboard_jobs_total", "Number of launchd jobs by status.", "gauge",
			[]string{"category", k}, float64(byCategory[k])})
	}

	// Per-job series.
	for _, j := range in.Jobs {
		base := []string{"label", j.Label, "category", j.Category, "group", j.Group}
		up := 0.0
		if j.Status == "running" {
			up = 1
		}
		samples = append(samples,
			sample{"deployboard_job_up", "1 when the launchd job is running.", "gauge", base, up},
			sample{"deployboard_job_runs", "Restart counter (launchctl print 'runs').", "gauge", base, float64(j.Runs)},
			sample{"deployboard_job_disabled", "1 when the job is explicitly disabled.", "gauge", base, boolVal(j.Disabled)},
			sample{"deployboard_alerts_enabled", "1 when Telegram alerts are enabled for the job.", "gauge",
				[]string{"label", j.Label, "category", j.Category}, boolVal(j.AlertEnabled)},
		)
		if j.HasExit {
			samples = append(samples, sample{"deployboard_job_last_exit_code",
				"Last exit code reported by launchctl print.", "gauge", base, float64(j.LastExitCode)})
		}
		// Probe series are omitted entirely when the job exposes no port.
		if j.HasProbe {
			pl := []string{"label", j.Label, "port", fmt.Sprintf("%d", j.ProbePort)}
			samples = append(samples,
				sample{"deployboard_job_probe_reachable", "1 when the job's port answered.", "gauge", pl, boolVal(j.ProbeOK)},
				sample{"deployboard_job_probe_status", "HTTP status code observed on the job's port.", "gauge", pl, float64(j.ProbeStatus)},
				sample{"deployboard_job_probe_latency_seconds", "Probe latency in seconds.", "gauge", pl, j.ProbeLatencyS},
			)
		}
	}

	// Process-level series.
	samples = append(samples,
		sample{"deployboard_up", "1 when the Deployboard exporter served this scrape.", "gauge", nil, 1},
		sample{"deployboard_scrape_duration_seconds", "Time spent building this exposition.", "gauge",
			nil, in.Duration.Seconds()},
		sample{"deployboard_version", "Deployboard build version as a constant 1 series.", "gauge",
			[]string{"version", in.Version}, 1},
	)

	return renderSamples(samples)
}

// renderSamples groups by metric name (family), sorts families and label sets,
// drops duplicate series, and writes the exposition text.
func renderSamples(samples []sample) string {
	type family struct {
		help   string
		typ    string
		series map[string]float64 // rendered label block → value (dedupe)
	}
	families := map[string]*family{}
	for _, s := range samples {
		f, ok := families[s.name]
		if !ok {
			f = &family{help: s.help, typ: s.typ, series: map[string]float64{}}
			families[s.name] = f
		}
		f.series[labelBlock(s.labels)] = s.value
	}

	names := make([]string, 0, len(families))
	for n := range families {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, n := range names {
		f := families[n]
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# HELP %s %s\n", n, f.help)
		fmt.Fprintf(&b, "# TYPE %s %s\n", n, f.typ)

		keys := make([]string, 0, len(f.series))
		for k := range f.series {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s%s %s\n", n, k, formatValue(f.series[k]))
		}
	}
	return b.String()
}

// labelBlock renders `{k="v",k2="v2"}` (empty string when there are no labels).
func labelBlock(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i+1 < len(labels); i += 2 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `%s="%s"`, labels[i], EscapeLabel(labels[i+1]))
	}
	b.WriteString("}")
	return b.String()
}

// formatValue prints integers without a decimal point and floats compactly.
func formatValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}

func boolVal(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
