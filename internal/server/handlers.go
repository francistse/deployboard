package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/A404coder/deployboard/internal/diagnose"
	"github.com/A404coder/deployboard/internal/launchd"
)

// writeJSON marshals v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response: {"error": "<message>"}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

const maxLogLines = 10000

func listJobsHandler(svc JobService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs, err := svc.ListJobs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"jobs":      jobs,
			"count":     len(jobs),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func getJobHandler(svc JobService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}
		job, err := svc.GetJob(label)
		if err != nil {
			if errors.Is(err, launchd.ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, job)
	}
}

func actionHandler(svc JobService, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}

		var err error
		var note string
		switch action {
		case "reload":
			err = svc.Reload(label)
		case "start":
			// A retired job is re-enabled by Start, so say so when the job was
			// disabled before the click.
			if job, jerr := svc.GetJob(label); jerr == nil && job.Disabled {
				note = "re-enabled and started (it was disabled)"
			}
			err = svc.Start(label)
		case "stop":
			// Tell the caller which mechanism was used: killing a KeepAlive job
			// is a restart, so Stop unloads it instead.
			if job, jerr := svc.GetJob(label); jerr == nil && job.KeepAlive {
				note = "unloaded from launchd — it has KeepAlive, so a signal alone would have restarted it"
			}
			err = svc.Stop(label)
		case "disable":
			note = "retired: launchd will not start it again, including at login"
			err = svc.Disable(label)
		case "enable":
			note = "no longer retired — press Start to bring it up"
			err = svc.Enable(label)
		default:
			writeError(w, http.StatusNotFound, "unknown action: "+action)
			return
		}

		if err != nil {
			// ErrSelfRestart is the success path for our own label: the body
			// has to be on the wire before RestartSelf exits or kickstarts us,
			// or the browser only sees a dropped connection.
			if errors.Is(err, launchd.ErrSelfRestart) {
				writeJSON(w, http.StatusOK, map[string]any{
					"ok":     true,
					"label":  label,
					"action": action,
					"note":   launchd.SelfRestartNote,
					"self":   true,
				})
				finishSelfRestart(w, svc.RestartSelf)
				return
			}
			status := http.StatusInternalServerError
			if errors.Is(err, launchd.ErrNotFound) || errors.Is(err, launchd.ErrInvalidLabel) {
				status = http.StatusBadRequest
			}
			writeJSON(w, status, map[string]any{
				"ok":     false,
				"label":  label,
				"action": action,
				"error":  err.Error(),
			})
			return
		}
		resp := map[string]any{
			"ok":     true,
			"label":  label,
			"action": action,
		}
		if note != "" {
			resp["note"] = note
		}
		// A 200 from launchctl is not the same as the intended state. Re-read the
		// job (the action dropped the memos) and report what is actually true, so
		// "succeeded" never means "we sent a command".
		resp["verified"] = verifyAction(svc, label, action)
		writeJSON(w, http.StatusOK, resp)
	}
}

// verifyAction re-reads a job immediately after an action and reports whether the
// world matches the intent. This is what turns a silent no-op (a KeepAlive job
// that was unloaded and came straight back, a signal sent to a label launchd had
// already dropped) into something the UI can show.
func verifyAction(svc JobService, label, action string) map[string]any {
	job, err := svc.GetJob(label)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	loaded := job.Status != launchd.StatusOffline && job.Status != launchd.StatusDisabled
	out := map[string]any{
		"status":    string(job.Status),
		"pid":       job.PID,
		"loaded":    loaded,
		"disabled":  job.Disabled,
		"keepAlive": job.KeepAlive,
	}
	var ok bool
	var verdict string
	switch action {
	case "stop":
		ok = job.PID == 0
		if !ok {
			verdict = "still running after the stop — something restarted it (KeepAlive?), or the signal was ignored"
		}
	case "start", "reload":
		ok = job.PID > 0 || job.Status == launchd.StatusScheduled || job.Status == launchd.StatusCompleted
		if !ok {
			verdict = "not running yet — check the job's logs, it may be exiting immediately"
		}
	case "disable":
		ok = job.Disabled
		if !ok {
			verdict = "launchd does not report this label as disabled"
		}
	case "enable":
		ok = !job.Disabled
		if !ok {
			verdict = "launchd still reports this label as disabled"
		}
	default:
		ok = true
	}
	out["ok"] = ok
	if verdict != "" {
		out["verdict"] = verdict
	}
	return out
}

// flushResponse pushes bytes already written. ResponseController is the
// current API; a writer that only implements Flusher (older tests, the SSE
// path) is the fallback. An unflushed buffer dies with this process on a
// self-restart, and the client never sees the note.
func flushResponse(w http.ResponseWriter) {
	if err := http.NewResponseController(w).Flush(); err == nil {
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// finishSelfRestart flushes, then asks launchd to bring this process back.
// The error cannot change the status line: the client already has 200.
func finishSelfRestart(w http.ResponseWriter, restart func() error) {
	flushResponse(w)
	if restart == nil {
		return
	}
	if err := restart(); err != nil {
		log.Printf("deployboard: self-restart: %v", err)
	}
}

func getLogsHandler(svc JobService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}

		lines := 200
		if q := r.URL.Query().Get("lines"); q != "" {
			n, err := strconv.Atoi(q)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, "invalid lines parameter")
				return
			}
			if n > maxLogLines {
				n = maxLogLines
			}
			lines = n
		}

		logs, err := svc.ReadLogs(label, lines)
		if err != nil {
			if errors.Is(err, launchd.ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, logs)
	}
}

func diagnoseHandler(svc JobService, diag *diagnose.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.PathValue("label")
		if !launchd.ValidLabel(label) {
			writeError(w, http.StatusBadRequest, "invalid label format")
			return
		}

		job, err := svc.GetJob(label)
		if err != nil {
			if errors.Is(err, launchd.ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		report := diag.Diagnose(job)
		writeJSON(w, http.StatusOK, report)
	}
}

// sseHandler returns an SSE handler using the default 5-second push interval.
func sseHandler(svc JobService) http.HandlerFunc {
	return sseHandlerWithInterval(svc, defaultSSEInterval)
}
