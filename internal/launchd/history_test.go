package launchd

import (
	"testing"
	"time"
)

func TestHistory_ObservesOncePerIntervalAndComputesDeltas(t *testing.T) {
	var h runHistory
	base := time.Unix(1700000000, 0)

	h.observe("com.example.web", 10, base)
	// A UI poll seconds later must update the same slot, not add a sample.
	h.observe("com.example.web", 10, base.Add(5*time.Second))
	if got := len(h.series("com.example.web")); got != 0 {
		t.Fatalf("one sample should yield no deltas, got %d", got)
	}

	h.observe("com.example.web", 13, base.Add(time.Minute))
	h.observe("com.example.web", 16, base.Add(2*time.Minute))
	got := h.series("com.example.web")
	want := []int{3, 3}
	if len(got) != len(want) {
		t.Fatalf("series = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("series = %v, want %v", got, want)
		}
	}
}

func TestHistory_RestartOfTheCounterIsNotNegativeChurn(t *testing.T) {
	var h runHistory
	base := time.Unix(1700000000, 0)

	// A reload resets launchd's runs counter; that is not a negative restart.
	h.observe("com.example.web", 500, base)
	h.observe("com.example.web", 0, base.Add(time.Minute))
	h.observe("com.example.web", 2, base.Add(2*time.Minute))

	series := h.series("com.example.web")
	for _, d := range series {
		if d < 0 {
			t.Fatalf("series contains a negative delta: %v", series)
		}
	}
	restarts, minutes := h.window("com.example.web")
	if restarts != 2 {
		t.Errorf("restarts = %d, want 2 (the counter reset itself is not churn)", restarts)
	}
	if minutes != 2 {
		t.Errorf("window = %dm, want 2m", minutes)
	}
}

func TestHistory_WindowIsHonestAboutShortObservations(t *testing.T) {
	var h runHistory
	// A single sample cannot describe a rate: the UI must be told "no window yet"
	// rather than shown a number invented from one data point.
	h.observe("com.example.web", 7, time.Unix(1700000000, 0))
	if restarts, minutes := h.window("com.example.web"); restarts != 0 || minutes != 0 {
		t.Errorf("window = (%d, %dm), want (0, 0) before there is a second sample", restarts, minutes)
	}
}

func TestHistory_ForgetDropsTheWindow(t *testing.T) {
	var h runHistory
	base := time.Unix(1700000000, 0)
	h.observe("com.example.web", 1, base)
	h.observe("com.example.web", 4, base.Add(time.Minute))
	h.forget("com.example.web")
	if series := h.series("com.example.web"); series != nil {
		t.Errorf("series after forget = %v, want nil", series)
	}
}
