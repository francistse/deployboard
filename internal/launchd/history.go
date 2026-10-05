package launchd

import (
	"sync"
	"time"
)

// Restart-rate windows. The runs counter from `launchctl print` is cumulative
// since launchd loaded the job, which says nothing about *recent* churn: a job
// with 1207 runs is only alarming if it added any today. So the dashboard keeps
// its own short observation window and reports deltas.
const (
	// HistoryInterval is the sampling period: one observation per job per minute.
	HistoryInterval = time.Minute
	// HistorySamples is how many observations are kept (30 minutes of history).
	HistorySamples = 30
)

type runSample struct {
	at   time.Time
	runs int
}

// runHistory is an in-memory observation window per label.
//
// Deliberately not persisted: after a dashboard restart the window refills in a
// minute, and a stale series from yesterday would be a lie. The Prometheus
// endpoint remains the place to get long-term history.
type runHistory struct {
	mu      sync.Mutex
	samples map[string][]runSample
}

// observe records the current runs counter, at most once per HistoryInterval.
func (h *runHistory) observe(label string, runs int, now time.Time) {
	if label == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.samples == nil {
		h.samples = map[string][]runSample{}
	}
	series := h.samples[label]
	if n := len(series); n > 0 && now.Sub(series[n-1].at) < HistoryInterval {
		// Same sampling slot: update in place so a fast poll does not fill the
		// window with duplicates.
		series[n-1].runs = runs
		h.samples[label] = series
		return
	}
	series = append(series, runSample{at: now, runs: runs})
	if len(series) > HistorySamples {
		series = series[len(series)-HistorySamples:]
	}
	h.samples[label] = series
}

// series returns the positive deltas between consecutive observations, oldest
// first — the restart counts per sampling interval.
func (h *runHistory) series(label string) []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	series := h.samples[label]
	if len(series) < 2 {
		return nil
	}
	out := make([]int, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		d := series[i].runs - series[i-1].runs
		if d < 0 {
			d = 0 // the job was reloaded: the counter starts again
		}
		out = append(out, d)
	}
	return out
}

// window reports the restarts observed in the window and how many minutes it
// covers, so the UI can say "4 restarts in the last 12m" without inventing a
// rate from a single sample.
func (h *runHistory) window(label string) (restarts int, minutes int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	series := h.samples[label]
	if len(series) < 2 {
		return 0, 0
	}
	total := 0
	for i := 1; i < len(series); i++ {
		if d := series[i].runs - series[i-1].runs; d > 0 {
			total += d
		}
	}
	span := series[len(series)-1].at.Sub(series[0].at)
	if span < time.Minute {
		span = time.Minute
	}
	return total, int(span.Minutes())
}

// forget drops a label's window (used when a job is reloaded by hand).
func (h *runHistory) forget(label string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.samples, label)
}
