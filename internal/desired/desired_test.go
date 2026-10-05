package desired

import (
	"reflect"
	"testing"
)

func TestResolve_JobWinsOverGroup(t *testing.T) {
	c := Config{
		Jobs:   []JobRule{{Match: "com.example.app.api", Status: StatusDisabled}},
		Groups: []GroupRule{{Name: "Example App", Status: StatusRunning}},
	}
	exp, ok := c.Resolve("com.example.app.api", "Example App")
	if !ok || exp.Status != StatusDisabled || exp.Source != "job" {
		t.Fatalf("got %+v ok=%v", exp, ok)
	}
}

func TestResolve_GroupFallback(t *testing.T) {
	c := Config{
		Groups: []GroupRule{{Name: "Infra", Status: StatusRunning, ProbePort: 5432}},
	}
	exp, ok := c.Resolve("com.example.infra.db", "Infra")
	if !ok || exp.Status != StatusRunning || exp.ProbePort != 5432 || exp.Source != "group" {
		t.Fatalf("got %+v ok=%v", exp, ok)
	}
}

func TestResolve_Glob(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.app.*", Status: StatusRunning}}}
	if _, ok := c.Resolve("com.example.app.web", ""); !ok {
		t.Fatal("expected glob match")
	}
	if _, ok := c.Resolve("com.other.app.web", ""); ok {
		t.Fatal("should not match")
	}
}

func TestEvaluate_OursOnly(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.*", Status: StatusRunning}}}
	jobs := []JobView{
		{Label: "com.example.web", Category: "ours", Status: "stopped"},
		{Label: "com.example.noise", Category: "noise", Status: "stopped"},
		{Label: "com.vendor.x", Category: "other", Status: "stopped"},
	}
	d := c.Evaluate(jobs)
	if len(d) != 1 || d[0].Label != "com.example.web" {
		t.Fatalf("got %#v", d)
	}
	if d[0].AlignAction != "start" {
		t.Fatalf("align=%q", d[0].AlignAction)
	}
}

func TestEvaluate_DisabledExpectation(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.worker", Status: StatusDisabled}}}
	jobs := []JobView{
		{Label: "com.example.worker", Category: "ours", Status: "running", Disabled: false},
	}
	d := c.Evaluate(jobs)
	if len(d) != 1 || d[0].AlignAction != "disable" {
		t.Fatalf("got %#v", d)
	}
	if !hasReason(d[0], ReasonDisabled) {
		t.Fatalf("reasons=%v", d[0].Reasons)
	}
}

func TestEvaluate_NoDriftWhenAligned(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.web", Status: StatusRunning}}}
	jobs := []JobView{
		{Label: "com.example.web", Category: "ours", Status: "running"},
	}
	if d := c.Evaluate(jobs); len(d) != 0 {
		t.Fatalf("unexpected drift %#v", d)
	}
}

func TestEvaluate_RestartRate(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.web", Status: StatusRunning, MaxRestartRate: 3}}}
	jobs := []JobView{
		{Label: "com.example.web", Category: "ours", Status: "running", RestartsRecent: 10},
	}
	d := c.Evaluate(jobs)
	if len(d) != 1 || !hasReason(d[0], ReasonRestartRate) {
		t.Fatalf("got %#v", d)
	}
}

func TestEvaluate_Probe(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.web", Status: StatusRunning, ProbePort: 8080}}}
	jobs := []JobView{
		{Label: "com.example.web", Category: "ours", Status: "running", HasProbe: true, ProbePort: 8080, ProbeOK: false},
	}
	d := c.Evaluate(jobs)
	if len(d) != 1 || !hasReason(d[0], ReasonProbe) {
		t.Fatalf("got %#v", d)
	}
}

func TestEvaluate_StartWhenDisabled(t *testing.T) {
	c := Config{Jobs: []JobRule{{Match: "com.example.web", Status: StatusRunning}}}
	jobs := []JobView{
		{Label: "com.example.web", Category: "ours", Status: "disabled", Disabled: true},
	}
	d := c.Evaluate(jobs)
	if len(d) != 1 || d[0].AlignAction != "start" {
		t.Fatalf("got %#v", d)
	}
}

func TestUniqueReasons(t *testing.T) {
	got := uniqueReasons([]Reason{ReasonStatus, ReasonDisabled, ReasonStatus})
	want := []Reason{ReasonStatus, ReasonDisabled}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func hasReason(d Drift, r Reason) bool {
	for _, x := range d.Reasons {
		if x == r {
			return true
		}
	}
	return false
}
