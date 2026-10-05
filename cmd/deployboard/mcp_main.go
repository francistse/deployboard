package main

import (
	"fmt"
	"strings"

	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/launchd"
	"github.com/A404coder/deployboard/internal/mcp"
	"github.com/A404coder/deployboard/internal/metrics"
	"github.com/A404coder/deployboard/internal/server"
)

// mcpAPI adapts appState to the Ours-scoped MCP tool surface.
type mcpAPI struct {
	app    *appState
	jobs   server.JobService
	access *accessController
}

func (m mcpAPI) ReadOnly() bool {
	if m.access != nil {
		return m.access.ReadOnly()
	}
	return false
}

func (m mcpAPI) OursStatus() (any, error) {
	jobs, err := m.jobs.ListJobs()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for _, j := range jobs {
		if j.Category != string(inventory.CategoryOurs) {
			continue
		}
		out = append(out, map[string]any{
			"label": j.Label, "status": j.Status, "group": j.Group,
			"disabled": j.Disabled, "pid": j.PID, "source": j.CompanionSource,
		})
	}
	return map[string]any{"jobs": out, "count": len(out)}, nil
}

func (m mcpAPI) DriftList() (any, error) {
	list, err := m.app.ListDrift()
	if err != nil {
		return nil, err
	}
	return map[string]any{"drifts": list, "count": len(list)}, nil
}

func (m mcpAPI) JobIncident(hours float64) (any, error) {
	events, fps, err := m.app.ListIncidents(hours)
	if err != nil {
		return nil, err
	}
	return map[string]any{"events": events, "fingerprints": fps, "hours": hours}, nil
}

func (m mcpAPI) JobAction(label, action string, confirm bool) (any, error) {
	if !launchd.ValidLabel(label) {
		return nil, fmt.Errorf("invalid label")
	}
	job, err := m.jobs.GetJob(label)
	if err != nil {
		return nil, err
	}
	if job.Category != string(inventory.CategoryOurs) {
		return nil, fmt.Errorf("refusing non-Ours label %s", label)
	}
	if strings.HasPrefix(label, "cron.companion.") {
		return nil, fmt.Errorf("cron companions are read-only")
	}
	switch action {
	case "start", "stop", "enable", "disable", "reload":
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}
	preview := map[string]any{
		"label": label, "action": action, "confirm": confirm,
		"dry_run": !confirm, "current_status": job.Status, "disabled": job.Disabled,
	}
	if !confirm {
		return preview, nil
	}
	if m.ReadOnly() {
		return nil, fmt.Errorf("server is read-only")
	}
	if err := applyJobAction(m.jobs, *job, action); err != nil {
		return nil, err
	}
	verified := verifyActionLocal(m.jobs, label, action)
	preview["verified"] = verified
	preview["dry_run"] = false
	return preview, nil
}

func (m mcpAPI) MetricsSnapshot() (any, error) {
	in := m.app.MetricsInput()
	return map[string]any{"text": metrics.Render(in)}, nil
}

func runMCP(app *appState) error {
	facade := jobFacade{inner: app.jobs, app: app}
	srv := &mcp.Server{API: mcpAPI{app: app, jobs: facade, access: app.access}}
	return srv.Run()
}
