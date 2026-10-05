package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/A404coder/deployboard/internal/alerts"
	"github.com/A404coder/deployboard/internal/diagnose"
	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/metrics"
)

// fakeTelegram records what the handlers passed through. Its status map has no
// token field, mirroring the real adapter's contract.
type fakeTelegram struct {
	savedToken string
	savedChat  string
	forgot     bool
	status     map[string]any
	testErr    error
	testStatus int
}

func (f *fakeTelegram) Status() (map[string]any, error) {
	if f.status == nil {
		return map[string]any{"configured": false, "chat_id": "42"}, nil
	}
	return f.status, nil
}

func (f *fakeTelegram) Save(token, chatID string) (map[string]any, error) {
	f.savedToken, f.savedChat = token, chatID
	if token == "boom" {
		return nil, errors.New("keychain write failed: ***")
	}
	f.status = map[string]any{"configured": true, "chat_id": chatID}
	return f.status, nil
}

func (f *fakeTelegram) Forget() (map[string]any, error) {
	f.forgot = true
	f.status = map[string]any{"configured": false}
	return f.status, nil
}

func (f *fakeTelegram) Test(ctx context.Context) (int, error) {
	if f.testErr != nil {
		return 0, f.testErr
	}
	return f.testStatus, nil
}

// fakeControl records classify calls.
type fakeControl struct {
	label    string
	category string
	reloads  int
}

func (f *fakeControl) ClassifyCategory(label, category string) (inventory.Report, error) {
	f.label, f.category = label, category
	return inventory.Report{Ours: 1, Sources: map[string]int{"ours_pattern": 1}}, nil
}
func (f *fakeControl) ReloadConfig() error { f.reloads++; return nil }

// fakeMetricsSource is an empty exposition source.
type fakeMetricsSource struct{}

func (fakeMetricsSource) MetricsInput() metrics.Input { return metrics.Input{Version: "test"} }

// reportOnly satisfies InventoryReporter (GET /api/inventory).
type reportOnly struct{}

func (reportOnly) InventoryReport() (inventory.Report, error) {
	return inventory.Report{Ours: 7, Other: 1, Noise: 100}, nil
}

// stubAccess is an in-memory AccessControl: it records writes and can simulate
// the --read-only hard lock without a config file on disk.
type stubAccess struct {
	readOnly bool
	locked   bool
	err      error
	writes   []bool
}

func (s *stubAccess) ReadOnly() bool { return s.readOnly }
func (s *stubAccess) Locked() bool   { return s.locked }
func (s *stubAccess) LockReason() string {
	if s.locked {
		return "the server was started with --read-only"
	}
	return ""
}
func (s *stubAccess) Source() string {
	if s.locked {
		return "flag"
	}
	return "config"
}
func (s *stubAccess) SetReadOnly(v bool) error {
	if s.err != nil {
		return s.err
	}
	if s.locked {
		return ErrAccessLocked
	}
	s.writes = append(s.writes, v)
	s.readOnly = v
	return nil
}

func forkRouterA(t *testing.T, tg TelegramSettings, control InventoryControl, access AccessControl) http.Handler {
	t.Helper()
	mock := &mockJobService{}
	return NewRouterWithFork(mock, &diagnose.Engine{}, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>Deployboard</body></html>")},
	}, ForkDeps{
		Version:   "test",
		Access:    access,
		Inventory: reportOnly{},
		Control:   control,
		Metrics:   fakeMetricsSource{},
		Alerts:    &fakeAlerts{},
		Telegram:  tg,
	})
}

func forkRouter(t *testing.T, tg TelegramSettings, control InventoryControl, readOnly bool) http.Handler {
	t.Helper()
	return forkRouterA(t, tg, control, &stubAccess{readOnly: readOnly})
}

type fakeAlerts struct{}

func (f *fakeAlerts) Available() bool                               { return false }
func (f *fakeAlerts) Snapshot() map[string]alerts.LabelState        { return map[string]alerts.LabelState{} }
func (f *fakeAlerts) Counts() (int, int)                            { return 0, 0 }
func (f *fakeAlerts) SetEnabled(string, bool) alerts.LabelState     { return alerts.LabelState{} }
func (f *fakeAlerts) BulkSet([]string, bool) int                    { return 0 }
func (f *fakeAlerts) History(int) []alerts.Entry                    { return nil }
func (f *fakeAlerts) SendTest(context.Context, string) (int, error) { return 0, nil }

func TestTelegramStatus_NeverCarriesTheToken(t *testing.T) {
	tg := &fakeTelegram{status: map[string]any{"configured": true, "chat_id": "42", "keychain_service": "svc"}}
	rec := httptest.NewRecorder()
	forkRouter(t, tg, &fakeControl{}, false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/telegram", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{`"bot_token"`, `"token"`, `"secret"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response must not contain %s: %s", forbidden, body)
		}
	}
}

func TestTelegramSave_ForwardsTokenAndDoesNotEchoIt(t *testing.T) {
	tg := &fakeTelegram{}
	req := httptest.NewRequest(http.MethodPost, "/api/settings/telegram",
		strings.NewReader(`{"bot_token":"123:ABC","chat_id":"999"}`))
	rec := httptest.NewRecorder()
	forkRouter(t, tg, &fakeControl{}, false).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if tg.savedToken != "123:ABC" || tg.savedChat != "999" {
		t.Errorf("handler did not forward token/chat: %q %q", tg.savedToken, tg.savedChat)
	}
	if strings.Contains(rec.Body.String(), "123:ABC") {
		t.Error("response echoed the token back")
	}
}

func TestTelegramSave_KeychainErrorStaysRedacted(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/settings/telegram", strings.NewReader(`{"bot_token":"boom"}`))
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{}, &fakeControl{}, false).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "***") {
		t.Errorf("error should stay redacted: %s", rec.Body.String())
	}
}

func TestTelegramSave_BadJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{}, &fakeControl{}, false).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/settings/telegram", strings.NewReader("{oops")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestTelegramForget(t *testing.T) {
	tg := &fakeTelegram{status: map[string]any{"configured": true}}
	rec := httptest.NewRecorder()
	forkRouter(t, tg, &fakeControl{}, false).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/settings/telegram", nil))
	if rec.Code != http.StatusOK || !tg.forgot {
		t.Errorf("forget not applied: code=%d forgot=%v", rec.Code, tg.forgot)
	}
}

func TestTelegramTestEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{testStatus: 200}, &fakeControl{}, false).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/settings/telegram/test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Errorf("ok = %v, want true", body["ok"])
	}

	// Delivery failure → 502, and the message must not leak a token.
	rec2 := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{testErr: errors.New("telegram sendMessage failed: status=401 Unauthorized")},
		&fakeControl{}, false).ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/settings/telegram/test", nil))
	if rec2.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec2.Code)
	}
}

func TestTelegramRoutesAbsentWhenDepMissing(t *testing.T) {
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/telegram", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when the dep is nil", rec.Code)
	}
}

func TestAccessStatus_ReportsModeLockAndSource(t *testing.T) {
	rec := httptest.NewRecorder()
	forkRouterA(t, &fakeTelegram{}, &fakeControl{}, &stubAccess{readOnly: true}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/access", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["read_only"] != true || got["write_mode"] != false || got["locked"] != false {
		t.Errorf("read_only=%v write_mode=%v locked=%v, want true/false/false", got["read_only"], got["write_mode"], got["locked"])
	}
	if got["source"] != "config" {
		t.Errorf("source = %v, want config", got["source"])
	}
}

// The whole point of the toggler: enabling write mode takes effect for the very
// next action, without a restart.
func TestAccessToggle_EnablesWriteModeImmediately(t *testing.T) {
	access := &stubAccess{readOnly: true}
	h := forkRouterA(t, &fakeTelegram{}, &fakeControl{}, access)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/jobs/com.test/stop", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("stop while read-only: status = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/settings/access",
		strings.NewReader(`{"read_only":false}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle: status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if len(access.writes) != 1 || access.writes[0] != false {
		t.Fatalf("writes = %v, want one write of false", access.writes)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/jobs/com.test/stop", nil))
	if rec.Code == http.StatusForbidden {
		t.Errorf("stop after enabling write mode: status = 403, want the action to go through")
	}
}

func TestAccessToggle_RefusedWhenLockedByFlag(t *testing.T) {
	access := &stubAccess{readOnly: true, locked: true}
	h := forkRouterA(t, &fakeTelegram{}, &fakeControl{}, access)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/settings/access",
		strings.NewReader(`{"read_only":false}`)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 when locked by --read-only", rec.Code)
	}
	if len(access.writes) != 0 || !access.readOnly {
		t.Errorf("a locked server must not change mode: writes=%v readOnly=%v", access.writes, access.readOnly)
	}
	if body := rec.Body.String(); !strings.Contains(body, "locked by --read-only") {
		t.Errorf("403 body should explain the lock, got %s", body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/access", nil))
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["locked"] != true || got["lock_reason"] == "" {
		t.Errorf("status should report locked + a reason, got %v", got)
	}
}

func TestAccessToggle_RejectsMalformedBody(t *testing.T) {
	access := &stubAccess{}
	h := forkRouterA(t, &fakeTelegram{}, &fakeControl{}, access)
	for _, body := range []string{`{}`, `{"read_only":null}`, `not json`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/settings/access", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
	if len(access.writes) != 0 {
		t.Errorf("no write expected, got %v", access.writes)
	}
}

// Read-only mode must still allow every read path the UI needs.
func TestReadOnlyGuard_ReadOnlyStillServesInventoryAndMetrics(t *testing.T) {
	h := forkRouter(t, &fakeTelegram{}, &fakeControl{}, true)
	for _, path := range []string{"/api/inventory", "/metrics", "/api/settings/access", "/api/jobs"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 in read-only mode", path, rec.Code)
		}
	}
}

func TestAccessEndpoints_AreAbsentWithoutTheDep(t *testing.T) {
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, ForkDeps{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/access", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when the dep is nil", rec.Code)
	}
}

func TestReadOnlyGuard_BlocksMutations(t *testing.T) {
	h := forkRouter(t, &fakeTelegram{}, &fakeControl{}, true)
	// disable/enable are mutating too: retiring a job from a monitoring box is
	// exactly the kind of side effect read-only mode exists to prevent.
	for _, action := range []string{"reload", "start", "stop", "disable", "enable"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/jobs/com.test/"+action, nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403 in read-only mode", action, rec.Code)
		}
	}
}

func TestReadOnlyGuard_AllowsReads(t *testing.T) {
	h := forkRouter(t, &fakeTelegram{}, &fakeControl{}, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthz: status = %d, want 200", rec.Code)
	}
}

func TestMetricsEndpoint_ContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{}, &fakeControl{}, false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "deployboard_up 1") {
		t.Errorf("exposition missing deployboard_up: %s", rec.Body.String())
	}
}

func TestInventoryEndpoint(t *testing.T) {
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{}, &fakeControl{}, false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/inventory", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rep map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep["ours"] != float64(7) {
		t.Errorf("ours = %v, want 7", rep["ours"])
	}
}

func TestClassifyEndpoint_ValidatesAndForwards(t *testing.T) {
	ctl := &fakeControl{}
	h := forkRouter(t, &fakeTelegram{}, ctl, false)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inventory/classify",
		strings.NewReader(`{"label":"bad label!","category":"ours"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid label: status = %d, want 400", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/inventory/classify",
		strings.NewReader(`{"label":"com.ok.thing","category":"noise"}`)))
	if rec2.Code != http.StatusOK {
		t.Fatalf("valid label: status = %d, want 200 (%s)", rec2.Code, rec2.Body.String())
	}
	if ctl.label != "com.ok.thing" || ctl.category != "noise" {
		t.Errorf("classify not forwarded: %q %q", ctl.label, ctl.category)
	}
}

func TestReloadConfigEndpoint(t *testing.T) {
	ctl := &fakeControl{}
	rec := httptest.NewRecorder()
	forkRouter(t, &fakeTelegram{}, ctl, false).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/inventory/reload", nil))
	if rec.Code != http.StatusOK || ctl.reloads != 1 {
		t.Errorf("reload: code=%d reloads=%d", rec.Code, ctl.reloads)
	}
}
