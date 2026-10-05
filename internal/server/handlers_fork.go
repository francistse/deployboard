package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/A404coder/deployboard/internal/alerts"
	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/launchd"
	"github.com/A404coder/deployboard/internal/metrics"
)

// InventoryReporter supplies the inventory summary (category + group counts).
type InventoryReporter interface {
	InventoryReport() (inventory.Report, error)
}

// InventoryControl adds the write-back the UI's classify button and hot-reload
// endpoint need.
type InventoryControl interface {
	ClassifyCategory(label, category string) (inventory.Report, error)
	ReloadConfig() error
}

// MetricsSource supplies the Prometheus exposition input.
type MetricsSource interface {
	MetricsInput() metrics.Input
}

// AlertsControl is the alert surface the HTTP handlers expose. It is an
// interface so the server package does not own alert state, and so tests can
// inject a stub without a Keychain.
type AlertsControl interface {
	Available() bool
	Snapshot() map[string]alerts.LabelState
	Counts() (enabled, disabled int)
	SetEnabled(label string, enabled bool) alerts.LabelState
	BulkSet(labels []string, enabled bool) int
	History(limit int) []alerts.Entry
	SendTest(ctx context.Context, label string) (int, error)
}

// ErrAccessLocked is returned by SetReadOnly when start-up arguments (the
// --read-only flag) forbid write mode. The HTTP layer turns it into a 403 with
// the lock reason, so the UI can explain rather than silently fail.
var ErrAccessLocked = errors.New("write mode is locked by --read-only")

// AccessControl is the live read-only state plus the one write the UI is allowed
// to make to it. It is an interface (not a bool) because the value can change
// while the server runs: the settings toggler flips config.json and the hot
// reload applies it without a restart.
type AccessControl interface {
	// ReadOnly reports the current mode. Consulted per request.
	ReadOnly() bool
	// Locked reports whether start-up arguments forbid write mode. A locked
	// server refuses to leave read-only, no matter what the UI asks for.
	Locked() bool
	// LockReason explains a lock, for the settings panel.
	LockReason() string
	// Source names where the current value came from ("flag", "config", "default").
	Source() string
	// SetReadOnly persists a new value and applies it immediately.
	SetReadOnly(readOnly bool) error
}

// GroupActionResult is one label's outcome inside a bulk group action.
type GroupActionResult struct {
	Label string `json:"label"`
	OK    bool   `json:"ok"`
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
	// Self is true when this entry is a reload of the dashboard's own job.
	// The handler writes the response, then RestartSelf — doing it inside
	// GroupAction would exit before the client could read the body.
	Self bool `json:"self,omitempty"`
}

// GroupActioner applies one action to every label in an inventory group, so
// "restart the whole stack" is one click instead of five.
type GroupActioner interface {
	GroupAction(group, action string) ([]GroupActionResult, error)
}

// ForkDeps carries the optional fork-only dependencies. A nil field simply
// disables its endpoint, so upstream behaviour is preserved when deps are zero.
type ForkDeps struct {
	Version   string
	Access    AccessControl
	Inventory InventoryReporter
	Control   InventoryControl
	Groups    GroupActioner
	Metrics   MetricsSource
	Alerts    AlertsControl
	Telegram  TelegramSettings
	// Jobs restarts the dashboard after a group reload that includes this
	// process. Nil skips that step.
	Jobs JobService
}

// classifyHandler implements POST /api/inventory/classify — the UI's
// "Ours" / "Hide" / "Auto" buttons. It writes config.json and returns the fresh
// report so the caller can re-render without a second round trip.
func classifyHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Control == nil {
			writeError(w, http.StatusNotFound, "inventory control not enabled")
			return
		}
		var body struct {
			Label    string `json:"label"`
			Category string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if !launchd.ValidLabel(body.Label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}
		rep, err := deps.Control.ClassifyCategory(body.Label, body.Category)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "label": body.Label, "category": body.Category, "report": rep})
	}
}

// reloadConfigHandler implements POST /api/inventory/reload (manual hot reload).
func reloadConfigHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Control == nil {
			writeError(w, http.StatusNotFound, "inventory control not enabled")
			return
		}
		if err := deps.Control.ReloadConfig(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// readOnlyGuard rejects mutating actions while the server is in monitoring mode.
// The decision is made per request against the live value, so flipping the
// settings toggler takes effect on the next click rather than the next restart.
func readOnlyGuard(access AccessControl, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if access == nil || !access.ReadOnly() {
			next(w, r)
			return
		}
		writeJSON(w, http.StatusForbidden, map[string]any{
			"ok":    false,
			"error": "read-only mode",
			"hint":  "enable write mode in Settings → Access, or start the server without --read-only",
			"locked": func() bool {
				return access.Locked()
			}(),
		})
	}
}

// accessStatusHandler implements GET /api/settings/access.
func accessStatusHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusNotFound, "access control not enabled")
			return
		}
		writeJSON(w, http.StatusOK, accessStatus(deps.Access))
	}
}

// accessToggleHandler implements POST /api/settings/access — the write-mode
// switch. It writes config.json (hot-reloaded in ~2s) and answers with the new
// state. A server started with --read-only refuses: that flag is the way to
// guarantee a box cannot be talked into killing a service.
func accessToggleHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusNotFound, "access control not enabled")
			return
		}
		var body struct {
			ReadOnly *bool `json:"read_only"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ReadOnly == nil {
			writeError(w, http.StatusBadRequest, "body must be {\"read_only\": true|false}")
			return
		}
		if err := deps.Access.SetReadOnly(*body.ReadOnly); err != nil {
			if errors.Is(err, ErrAccessLocked) {
				writeJSON(w, http.StatusForbidden, map[string]any{
					"ok":    false,
					"error": err.Error(),
					"hint":  deps.Access.LockReason(),
				})
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, accessStatus(deps.Access))
	}
}

// groupActionHandler implements POST /api/inventory/group-action.
//
// It is a mutating route, so the router wraps it in the read-only guard: a
// monitoring box must not be able to stop a whole group either.
func groupActionHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Groups == nil {
			writeError(w, http.StatusNotFound, "group actions not enabled")
			return
		}
		var body struct {
			Group  string `json:"group"`
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		results, err := deps.Groups.GroupAction(body.Group, body.Action)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		succeeded, failed, skipped := 0, 0, 0
		restartSelf := false
		for _, res := range results {
			if body.Action == "reload" && res.Self && res.OK {
				restartSelf = true
			}
			switch {
			case res.Error != "":
				failed++
			case res.Note != "" && !res.OK:
				skipped++
			default:
				succeeded++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        failed == 0,
			"group":     body.Group,
			"action":    body.Action,
			"results":   results,
			"succeeded": succeeded,
			"failed":    failed,
			"skipped":   skipped,
		})
		// After the body is written. Restarting inside GroupAction would
		// kill this process before the counts reached the browser, and a
		// bootout of our own label would leave the group half-reloaded
		// with nothing to bring the dashboard back.
		if restartSelf && deps.Jobs != nil {
			finishSelfRestart(w, deps.Jobs.RestartSelf)
		}
	}
}

// accessStatus is the JSON shape both access endpoints return.
func accessStatus(a AccessControl) map[string]any {
	return map[string]any{
		"ok":          true,
		"read_only":   a.ReadOnly(),
		"locked":      a.Locked(),
		"lock_reason": a.LockReason(),
		"source":      a.Source(),
		"write_mode":  !a.ReadOnly(),
	}
}

// inventoryHandler serves the category/group summary.
func inventoryHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Inventory == nil {
			writeError(w, http.StatusNotFound, "inventory not enabled")
			return
		}
		rep, err := deps.Inventory.InventoryReport()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rep)
	}
}

// metricsHandler serves the Prometheus exposition.
func metricsHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Metrics == nil {
			writeError(w, http.StatusNotFound, "metrics not enabled")
			return
		}
		start := time.Now()
		in := deps.Metrics.MetricsInput()
		in.Duration = time.Since(start)
		if in.Version == "" {
			in.Version = deps.Version
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Render(in)))
	}
}

// alertsSnapshotHandler returns per-label alert state and counts.
func alertsSnapshotHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Alerts == nil {
			writeError(w, http.StatusNotFound, "alerts not enabled")
			return
		}
		enabled, disabled := deps.Alerts.Counts()
		writeJSON(w, http.StatusOK, map[string]any{
			"available": deps.Alerts.Available(),
			"enabled":   enabled,
			"disabled":  disabled,
			"labels":    deps.Alerts.Snapshot(),
		})
	}
}

// alertToggleHandler toggles one label (POST /api/alerts/{label}).
func alertToggleHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Alerts == nil {
			writeError(w, http.StatusNotFound, "alerts not enabled")
			return
		}
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
			writeError(w, http.StatusBadRequest, "body must be {\"enabled\": true|false}")
			return
		}
		ls := deps.Alerts.SetEnabled(label, *body.Enabled)
		writeJSON(w, http.StatusOK, map[string]any{"label": label, "state": ls})
	}
}

// alertBulkHandler toggles many labels at once (POST /api/alerts).
func alertBulkHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Alerts == nil {
			writeError(w, http.StatusNotFound, "alerts not enabled")
			return
		}
		var body struct {
			Labels  []string `json:"labels"`
			Enabled *bool    `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil || len(body.Labels) == 0 {
			writeError(w, http.StatusBadRequest, "body must be {\"labels\": [...], \"enabled\": true|false}")
			return
		}
		for _, label := range body.Labels {
			if !launchd.ValidLabel(label) {
				writeError(w, http.StatusBadRequest, "invalid label format: "+label)
				return
			}
		}
		n := deps.Alerts.BulkSet(body.Labels, *body.Enabled)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": n, "enabled": *body.Enabled})
	}
}

// alertTestHandler sends a test alert for one label.
func alertTestHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Alerts == nil {
			writeError(w, http.StatusNotFound, "alerts not enabled")
			return
		}
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}
		status, err := deps.Alerts.SendTest(r.Context(), label)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "label": label, "telegramStatus": status})
	}
}

// alertHistoryHandler returns the recent send log.
func alertHistoryHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Alerts == nil {
			writeError(w, http.StatusNotFound, "alerts not enabled")
			return
		}
		limit := 50
		if q := r.URL.Query().Get("limit"); q != "" {
			n, err := strconv.Atoi(q)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, "invalid limit parameter")
				return
			}
			limit = n
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": deps.Alerts.History(limit)})
	}
}

// TelegramSettings is the token-management surface for the settings UI. The
// token itself is never read back out: Status reports only whether an item
// exists and is readable, and Save/Forget return the same shape.
type TelegramSettings interface {
	Status() (map[string]any, error)
	Save(botToken, chatID string) (map[string]any, error)
	Forget() (map[string]any, error)
	Test(ctx context.Context) (int, error)
}

// telegramStatusHandler implements GET /api/settings/telegram.
func telegramStatusHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Telegram == nil {
			writeError(w, http.StatusNotFound, "telegram settings not enabled")
			return
		}
		st, err := deps.Telegram.Status()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// telegramSaveHandler implements POST /api/settings/telegram.
//
// The token is accepted over loopback only (the server binds 127.0.0.1), written
// straight to the macOS Keychain, and never echoed back or logged.
func telegramSaveHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Telegram == nil {
			writeError(w, http.StatusNotFound, "telegram settings not enabled")
			return
		}
		var body struct {
			BotToken string `json:"bot_token"`
			ChatID   string `json:"chat_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		st, err := deps.Telegram.Save(body.BotToken, body.ChatID)
		if err != nil {
			// err is built by the keychain layer and already redacted.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// telegramForgetHandler implements DELETE /api/settings/telegram.
func telegramForgetHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Telegram == nil {
			writeError(w, http.StatusNotFound, "telegram settings not enabled")
			return
		}
		st, err := deps.Telegram.Forget()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

// telegramSettingsTestHandler implements POST /api/settings/telegram/test.
func telegramSettingsTestHandler(deps ForkDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Telegram == nil {
			writeError(w, http.StatusNotFound, "telegram settings not enabled")
			return
		}
		status, err := deps.Telegram.Test(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "telegramStatus": status})
	}
}

// errAlertsUnavailable is returned by the CLI/test paths when no sender exists.
var errAlertsUnavailable = errors.New("alerts unavailable")
