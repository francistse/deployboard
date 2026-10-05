package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/A404coder/deployboard/internal/diagnose"
	"github.com/A404coder/deployboard/internal/launchd"
)

// fakeGroups records the group actions the handler forwards.
type fakeGroups struct {
	group  string
	action string
	result []GroupActionResult
	err    error
}

func (f *fakeGroups) GroupAction(group, action string) ([]GroupActionResult, error) {
	f.group, f.action = group, action
	return f.result, f.err
}

func groupRouter(t *testing.T, groups GroupActioner) http.Handler {
	t.Helper()
	return NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{Groups: groups})
}

func TestGroupAction_ForwardsAndSummarises(t *testing.T) {
	g := &fakeGroups{result: []GroupActionResult{
		{Label: "com.example.a", OK: true, Note: "unloaded"},
		{Label: "com.example.b", OK: true},
		{Label: "com.example.c", Note: "not running"},
		{Label: "com.example.d", Error: "boom"},
	}}
	rec := httptest.NewRecorder()
	groupRouter(t, g).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inventory/group-action",
		strings.NewReader(`{"group":"Example App — UAT","action":"stop"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if g.group != "Example App — UAT" || g.action != "stop" {
		t.Errorf("forwarded %q/%q, want the group and action verbatim", g.group, g.action)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != false {
		t.Error("a group action with one failure must not report overall ok")
	}
	if body["succeeded"] != float64(2) || body["skipped"] != float64(1) || body["failed"] != float64(1) {
		t.Errorf("summary = %v/%v/%v, want 2 succeeded / 1 skipped / 1 failed",
			body["succeeded"], body["skipped"], body["failed"])
	}
}

func TestGroupAction_RejectsBadBodyAndUnknownDep(t *testing.T) {
	rec := httptest.NewRecorder()
	groupRouter(t, &fakeGroups{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/inventory/group-action", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body: status = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inventory/group-action", nil))
	// With no GroupActioner the route is not registered at all (the upstream
	// convention for an absent dep), so the static handler answers 405.
	if rec.Code == http.StatusOK {
		t.Errorf("status = %d, want the route to be absent", rec.Code)
	}
}

// A bulk action is as mutating as a single one: read-only mode must stop it.
func TestGroupAction_BlockedInReadOnlyMode(t *testing.T) {
	g := &fakeGroups{result: []GroupActionResult{{Label: "com.example.a", OK: true}}}
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{
		Groups: g,
		Access: &stubAccess{readOnly: true},
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inventory/group-action",
		strings.NewReader(`{"group":"Example","action":"stop"}`)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 in read-only mode", rec.Code)
	}
	if g.group != "" {
		t.Errorf("the guard must run before the dependency: forwarded %q", g.group)
	}
}

func TestVerifyAction_ReportsTheRealState(t *testing.T) {
	jobs := []launchd.Job{
		{Label: "com.example.stillup", Status: launchd.StatusRunning, PID: 4242, KeepAlive: true},
		{Label: "com.example.down", Status: launchd.StatusOffline, PID: 0},
		{Label: "com.example.retired", Status: launchd.StatusDisabled, Disabled: true},
		{Label: "com.example.slowstart", Status: launchd.StatusStopped, PID: 0},
	}
	svc := &mockJobService{jobs: jobs}

	// Stop that did not take: must not claim success.
	v := verifyAction(svc, "com.example.stillup", "stop")
	if v["ok"] != false {
		t.Errorf("stop on a running job verified ok=%v, want false", v["ok"])
	}
	if verdict, _ := v["verdict"].(string); verdict == "" {
		t.Error("a failed verification should explain itself")
	}

	// Stop that did take.
	if v := verifyAction(svc, "com.example.down", "stop"); v["ok"] != true {
		t.Errorf("stop on an unloaded job verified ok=%v, want true", v["ok"])
	}

	// Retire / un-retire.
	if v := verifyAction(svc, "com.example.retired", "disable"); v["ok"] != true {
		t.Errorf("disable verified ok=%v, want true", v["ok"])
	}
	if v := verifyAction(svc, "com.example.retired", "enable"); v["ok"] != false {
		t.Errorf("enable on a still-disabled job verified ok=%v, want false", v["ok"])
	}

	// Start that did not come up: a job exiting immediately is a real case.
	v = verifyAction(svc, "com.example.slowstart", "start")
	if v["ok"] != false {
		t.Errorf("start verified ok=%v, want false", v["ok"])
	}
	if verdict, _ := v["verdict"].(string); !strings.Contains(verdict, "logs") {
		t.Errorf("verdict should point at the logs, got %q", verdict)
	}

	// A job that vanished from the listing is reported, not panicked on.
	v = verifyAction(svc, "com.example.missing", "stop")
	if v["ok"] != false || v["error"] == nil {
		t.Errorf("missing job: %v, want ok=false with an error", v)
	}
}

// A group reload that includes the dashboard writes the summary first, then
// RestartSelf. Doing it the other way round drops the response on the floor.
func TestGroupAction_SelfReloadRestartsAfterResponse(t *testing.T) {
	g := &fakeGroups{result: []GroupActionResult{
		{Label: "com.example.board.web", OK: true},
		{Label: "com.deployboard.launch-pilot", OK: true, Self: true, Note: launchd.SelfRestartNote},
	}}
	mock := &mockJobService{}
	rec := httptest.NewRecorder()
	mock.beforeRestart = func() {
		if !strings.Contains(rec.Body.String(), launchd.SelfRestartNote) {
			t.Errorf("RestartSelf ran before the group body was written: %s", rec.Body.String())
		}
	}
	h := NewRouterWithFork(mock, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{Groups: g, Jobs: mock})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inventory/group-action",
		strings.NewReader(`{"group":"Deployboard","action":"reload"}`)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if mock.restartSelfCalls != 1 {
		t.Fatalf("RestartSelf calls = %d, want 1", mock.restartSelfCalls)
	}
}
