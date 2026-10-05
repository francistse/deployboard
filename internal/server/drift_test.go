package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/A404coder/deployboard/internal/diagnose"
)

type driftStub struct {
	list []DriftEntry
	res  DriftAlignResult
	err  error
}

func (d driftStub) ListDrift() ([]DriftEntry, error) { return d.list, d.err }
func (d driftStub) AlignDrift(label, action string) (DriftAlignResult, error) {
	if d.err != nil {
		return DriftAlignResult{}, d.err
	}
	r := d.res
	r.Label = label
	if action != "" {
		r.Action = action
	} else if r.Action == "" {
		r.Action = "start"
	}
	r.OK = true
	return r, nil
}

func TestDriftListHandler(t *testing.T) {
	deps := ForkDeps{
		Drift: driftStub{list: []DriftEntry{{
			Label: "com.example.web", ExpectedStatus: "running", ActualStatus: "stopped",
			Reasons: []string{"status"}, AlignAction: "start",
		}}},
	}
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, deps)
	req := httptest.NewRequest(http.MethodGet, "/api/drift", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Count  int          `json:"count"`
		Drifts []DriftEntry `json:"drifts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || body.Drifts[0].Label != "com.example.web" {
		t.Fatalf("%+v", body)
	}
}

func TestDriftAlignHandler(t *testing.T) {
	deps := ForkDeps{
		Drift: driftStub{res: DriftAlignResult{Action: "start", OK: true}},
	}
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, deps)
	body := bytes.NewBufferString(`{"label":"com.example.web"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/drift/align", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDriftAlignReadOnly(t *testing.T) {
	deps := ForkDeps{
		Access: &stubAccess{readOnly: true},
		Drift:  driftStub{res: DriftAlignResult{OK: true, Action: "start"}},
	}
	h := NewRouterWithFork(&mockJobService{}, &diagnose.Engine{}, fstest.MapFS{}, deps)
	body := bytes.NewBufferString(`{"label":"com.example.web"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/drift/align", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s want 403", rec.Code, rec.Body.String())
	}
}
