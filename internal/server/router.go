package server

import (
	"io/fs"
	"net/http"

	"github.com/A404coder/deployboard/internal/diagnose"
	"github.com/A404coder/deployboard/internal/launchd"
)

// JobService defines the launchd operations used by API handlers.
type JobService interface {
	ListJobs() ([]launchd.Job, error)
	GetJob(label string) (*launchd.Job, error)
	Reload(label string) error
	// RestartSelf brings this process back after a self reload. The handler
	// calls it only after the response has been written and flushed.
	RestartSelf() error
	Start(label string) error
	Stop(label string) error
	Disable(label string) error
	Enable(label string) error
	ReadLogs(label string, lines int) (*launchd.LogOutput, error)
}

// NewRouter creates the HTTP handler with all API routes and static file serving.
func NewRouter(svc JobService, diag *diagnose.Engine, webFS fs.FS) http.Handler {
	return NewRouterWithFork(svc, diag, webFS, ForkDeps{})
}

// NewRouterWithFork adds the fork's routes (/metrics, /api/inventory,
// /api/alerts*) and the read-only guard. A nil dep leaves its route unregistered,
// so the upstream route table is unchanged when ForkDeps is zero.
func NewRouterWithFork(svc JobService, diag *diagnose.Engine, webFS fs.FS, deps ForkDeps) http.Handler {
	mux := http.NewServeMux()

	// Jobs API
	mux.HandleFunc("GET /api/jobs", listJobsHandler(svc))
	mux.HandleFunc("GET /api/jobs/{label}", getJobHandler(svc))

	// Job actions — refused while the server is in read-only (monitoring) mode.
	// The guard asks the live AccessControl, so the settings toggler applies
	// without a restart.
	reload := actionHandler(svc, "reload")
	start := actionHandler(svc, "start")
	stop := actionHandler(svc, "stop")
	disable := actionHandler(svc, "disable")
	enable := actionHandler(svc, "enable")
	if deps.Access != nil {
		guard := deps.Access
		reload, start, stop = readOnlyGuard(guard, reload), readOnlyGuard(guard, start), readOnlyGuard(guard, stop)
		disable, enable = readOnlyGuard(guard, disable), readOnlyGuard(guard, enable)
	}
	mux.HandleFunc("POST /api/jobs/{label}/reload", reload)
	mux.HandleFunc("POST /api/jobs/{label}/start", start)
	mux.HandleFunc("POST /api/jobs/{label}/stop", stop)
	mux.HandleFunc("POST /api/jobs/{label}/disable", disable)
	mux.HandleFunc("POST /api/jobs/{label}/enable", enable)

	// Logs + diagnostics
	mux.HandleFunc("GET /api/jobs/{label}/logs", getLogsHandler(svc))
	mux.HandleFunc("GET /api/jobs/{label}/diagnose", diagnoseHandler(svc, diag))

	// SSE real-time push
	mux.HandleFunc("GET /api/events", sseHandler(svc))

	// Liveness probe for the install script / launchd KeepAlive checks.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// Fork routes
	if deps.Inventory != nil {
		mux.HandleFunc("GET /api/inventory", inventoryHandler(deps))
	}
	if deps.Control != nil {
		mux.HandleFunc("POST /api/inventory/classify", classifyHandler(deps))
		mux.HandleFunc("POST /api/inventory/reload", reloadConfigHandler(deps))
	}
	if deps.Groups != nil {
		group := groupActionHandler(deps)
		if deps.Access != nil {
			group = readOnlyGuard(deps.Access, group)
		}
		mux.HandleFunc("POST /api/inventory/group-action", group)
	}
	if deps.Metrics != nil {
		mux.HandleFunc("GET /metrics", metricsHandler(deps))
	}
	if deps.Alerts != nil {
		mux.HandleFunc("GET /api/alerts", alertsSnapshotHandler(deps))
		mux.HandleFunc("POST /api/alerts", alertBulkHandler(deps))
		mux.HandleFunc("GET /api/alerts/history", alertHistoryHandler(deps))
		mux.HandleFunc("POST /api/alerts/{label}", alertToggleHandler(deps))
		mux.HandleFunc("POST /api/alerts/{label}/test", alertTestHandler(deps))
	}
	if deps.Access != nil {
		mux.HandleFunc("GET /api/settings/access", accessStatusHandler(deps))
		mux.HandleFunc("POST /api/settings/access", accessToggleHandler(deps))
	}
	if deps.Telegram != nil {
		mux.HandleFunc("GET /api/settings/telegram", telegramStatusHandler(deps))
		mux.HandleFunc("POST /api/settings/telegram", telegramSaveHandler(deps))
		mux.HandleFunc("DELETE /api/settings/telegram", telegramForgetHandler(deps))
		mux.HandleFunc("POST /api/settings/telegram/test", telegramSettingsTestHandler(deps))
	}
	if deps.Drift != nil {
		mux.HandleFunc("GET /api/drift", driftListHandler(deps))
		align := driftAlignHandler(deps)
		if deps.Access != nil {
			align = readOnlyGuard(deps.Access, align)
		}
		mux.HandleFunc("POST /api/drift/align", align)
	}
	if deps.Contracts != nil {
		mux.HandleFunc("GET /api/contracts", contractsHandler(deps))
	}
	if deps.Incidents != nil {
		mux.HandleFunc("GET /api/incidents", incidentsHandler(deps))
	}

	// Static files — embedded frontend
	mux.Handle("GET /", http.FileServerFS(webFS))

	return mux
}
