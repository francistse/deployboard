package launchd

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/A404coder/deployboard/internal/plist"
)

// Sentinel errors returned by Service methods.
var (
	ErrNotFound     = errors.New("job not found")
	ErrInvalidLabel = errors.New("invalid label")
	// ErrSelfRestart means Reload refused to bootout this process's own job.
	// Bootout removes the job from launchd and SIGTERMs us before bootstrap
	// can run, and KeepAlive does not apply to a job that is no longer loaded.
	// Callers treat this as success and then call RestartSelf.
	ErrSelfRestart = errors.New("refusing to bootout the dashboard's own job")
)

// SelfRestartNote is the API copy for a reload of this process. There is no
// verified block: the handler is about to exit so launchd can respawn the job.
const SelfRestartNote = "restarting the dashboard itself — launchd respawns it; the page reconnects in a few seconds"

// LabelRe validates launchd job labels: alphanumeric, dots, hyphens, underscores.
var LabelRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidLabel reports whether label contains only safe characters for launchctl args.
func ValidLabel(label string) bool {
	return LabelRe.MatchString(label)
}

// ValidateLabel checks that a label contains only safe characters for launchctl args.
func ValidateLabel(label string) error {
	if !ValidLabel(label) {
		return fmt.Errorf("%w: %q", ErrInvalidLabel, label)
	}
	return nil
}

// JobStatus represents the runtime state of a launchd job.
type JobStatus string

const (
	StatusRunning   JobStatus = "running"
	StatusScheduled JobStatus = "scheduled"
	StatusCompleted JobStatus = "completed"
	StatusStopped   JobStatus = "stopped"
	StatusError     JobStatus = "error"
	StatusOffline   JobStatus = "offline"
	StatusDisabled  JobStatus = "disabled"
)

// Job holds merged data from launchctl list + plist file for a single launchd job.
type Job struct {
	Label                 string                `json:"label"`
	PID                   int                   `json:"pid"`
	LastExitStatus        int                   `json:"lastExitStatus"`
	Status                JobStatus             `json:"status"`
	PlistPath             string                `json:"plistPath"`
	Program               string                `json:"program"`
	ProgramArgs           []string              `json:"programArgs"`
	StandardOutPath       string                `json:"standardOutPath"`
	StandardErrPath       string                `json:"standardErrPath"`
	WorkingDirectory      string                `json:"workingDirectory,omitempty"`
	RunAtLoad             bool                  `json:"runAtLoad"`
	KeepAlive             bool                  `json:"keepAlive"`
	Domain                string                `json:"domain"`
	NextRunAt             *time.Time            `json:"nextRunAt,omitempty"`
	LastRunAt             *time.Time            `json:"lastRunAt,omitempty"`
	StartInterval         int                   `json:"startInterval,omitempty"`
	StartCalendarInterval []plist.CalendarEntry `json:"startCalendarInterval,omitempty"`

	// Fork additions. Empty when launchctl print enrichment is not enabled.
	Category       string `json:"category"`
	CategorySource string `json:"categorySource,omitempty"`
	Group          string `json:"group"`
	Runs           int    `json:"runs"`
	Disabled       bool   `json:"disabled"`
	PrintState     string `json:"printState"`
	RestartWarn    bool   `json:"restartWarn"`
	// UptimeSeconds is how long the current process has been running, from `ps`.
	// Zero when the job is not running or the lookup is disabled.
	UptimeSeconds int `json:"uptimeSeconds,omitempty"`
	// RestartsRecent + WindowMinutes describe churn observed by the dashboard
	// itself (the launchd runs counter is cumulative, so it cannot answer
	// "is this flapping right now?").
	RestartsRecent int   `json:"restartsRecent,omitempty"`
	WindowMinutes  int   `json:"windowMinutes,omitempty"`
	RunsSeries     []int `json:"runsSeries,omitempty"`
	// RetiredAt is when the dashboard disabled this job (see internal/retire).
	RetiredAt *time.Time `json:"retiredAt,omitempty"`
	// Self is true when this job is the dashboard process itself. Stop and
	// Disable are still offered — they mean "keep it down" — but bootout of
	// this label drops the page until the next login or a manual bootstrap.
	Self bool `json:"self,omitempty"`
	// CompanionSource is set for synthetic read-only rows (e.g. "cron").
	CompanionSource string `json:"source,omitempty"`
	// Program holds a display command for companions when not a real plist.
	CompanionCommand string `json:"companionCommand,omitempty"`

	// HasExit is true when `launchctl print` reported a last exit code.
	// Omitted from JSON; metrics skip the series when it is false.
	HasExit bool `json:"-"`
}

// DefaultRecentWindow is the default --recent-window value: how long after
// lastRunAt a scheduled job is still considered "completed".
const DefaultRecentWindow = 10 * time.Minute

// DeriveStatus determines the JobStatus for an online (launchctl-listed) job
// from PID, exit code, plist schedule shape, last-run heuristic, and the
// configured recent-completion window.
//
// Offline (plist present but not in `launchctl list`) is assigned by the
// service during the offline merge pass, not here.
func DeriveStatus(pid, exitStatus int, p plist.PlistData, lastRunAt *time.Time, now time.Time, window time.Duration) JobStatus {
	if pid > 0 {
		return StatusRunning
	}
	if exitStatus != 0 {
		return StatusError
	}
	if lastRunAt != nil {
		delta := now.Sub(*lastRunAt)
		if delta >= 0 && delta <= window {
			return StatusCompleted
		}
	}
	if p.StartInterval > 0 || len(p.StartCalendarInterval) > 0 || p.RunAtLoad {
		return StatusScheduled
	}
	return StatusStopped
}

// LogOutput holds the result of reading a job's stdout/stderr log files.
type LogOutput struct {
	Label           string  `json:"label"`
	Stdout          *string `json:"stdout"`
	Stderr          *string `json:"stderr"`
	StdoutPath      string  `json:"stdoutPath"`
	StderrPath      string  `json:"stderrPath"`
	StdoutAvailable bool    `json:"stdoutAvailable"`
	StderrAvailable bool    `json:"stderrAvailable"`
	Message         string  `json:"message,omitempty"`
}
