package launchd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/plist"
)

// tailFile reads the last n lines from a file using a circular buffer,
// avoiding loading the entire file into memory for large log files.
func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	ring := make([]string, n)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	idx := 0
	count := 0
	for scanner.Scan() {
		ring[idx] = scanner.Text()
		idx = (idx + 1) % n
		count++
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	if count <= n {
		return strings.Join(ring[:count], "\n"), nil
	}
	result := make([]string, 0, n)
	result = append(result, ring[idx:]...)
	result = append(result, ring[:idx]...)
	return strings.Join(result, "\n"), nil
}

// ReadLogs reads the stdout and stderr log files for a job, returning the last
// n lines of each. Returns a message when no log paths are configured in the
// plist. Returns null stdout/stderr when log files don't exist on disk.
func (s *Service) ReadLogs(label string, lines int) (*LogOutput, error) {
	job, err := s.GetJob(label)
	if err != nil {
		return nil, err
	}

	out := &LogOutput{
		Label:      job.Label,
		StdoutPath: job.StandardOutPath,
		StderrPath: job.StandardErrPath,
	}

	// No log paths configured at all.
	if job.StandardOutPath == "" && job.StandardErrPath == "" {
		out.Message = "No log paths configured in plist"
		return out, nil
	}

	// Read stdout if configured.
	if job.StandardOutPath != "" {
		content, err := tailFile(job.StandardOutPath, lines)
		if err == nil {
			out.Stdout = &content
			out.StdoutAvailable = true
		}
	}

	// Read stderr if configured.
	if job.StandardErrPath != "" {
		content, err := tailFile(job.StandardErrPath, lines)
		if err == nil {
			out.Stderr = &content
			out.StderrAvailable = true
		}
	}

	return out, nil
}

// Service provides high-level operations on launchd user-domain jobs.
// It merges data from launchctl list (Layer 1) with plist files (Layer 2).
type Service struct {
	uid     int
	domain  string // "gui/<UID>"
	homeDir string
	cache   *plist.Cache
	window  time.Duration
	// selfLabel is this process's launchd label. Empty means we are not
	// running as the dashboard job, so Reload bootouts whoever was asked.
	selfLabel string

	// Overridable for testing.
	runList   func() (string, error)
	scanPlist func() []plist.ScanResult
	runExec   func(name string, args ...string) (*ExecResult, error)
	nowFn     func() time.Time
	statFn    func(string) (os.FileInfo, error)
	// exitFn ends the process on a KeepAlive self-restart. The default is
	// os.Exit; tests replace it so the suite is not the thing that dies.
	exitFn func(int)
	// startDetached spawns a helper in its own session and does not wait.
	// A self-restart without KeepAlive uses it so kickstart outlives us.
	startDetached func(name string, args ...string) error

	// processLookup resolves a pid's uptime (nil = uptime column stays empty).
	processLookup ProcessLookup
	procs         processCache
	// history is the in-memory restart-rate window.
	history runHistory
	// retirement answers "when did the dashboard retire this job?".
	retirement func(label string) (time.Time, bool)

	// Fork enrichment. Nil funcs leave upstream ListJobs behaviour unchanged.
	classifier  *inventory.Classifier
	threshold   int
	printTTL    time.Duration
	runPrint    func(domain, label string) (stdout, stderr string, err error)
	runDisabled func(domain string) (stdout string, err error)

	mu          sync.Mutex
	disabledAt  time.Time
	disabledMap map[string]bool
	printCache  map[string]printCacheEntry
}

// NewService creates a Service configured for the current user with the
// default recent-completion window.
func NewService() *Service {
	return NewServiceWithWindow(DefaultRecentWindow)
}

// NewServiceWithWindow creates a Service with an explicit completion window.
func NewServiceWithWindow(window time.Duration) *Service {
	uid := os.Getuid()
	home, _ := os.UserHomeDir()
	s := &Service{
		uid:           uid,
		domain:        fmt.Sprintf("gui/%d", uid),
		homeDir:       home,
		cache:         plist.NewCache(),
		window:        window,
		nowFn:         time.Now,
		statFn:        os.Stat,
		exitFn:        os.Exit,
		startDetached: startDetachedCmd,
	}
	s.runList = s.defaultRunList
	s.scanPlist = s.defaultScanPlist
	s.runExec = func(name string, args ...string) (*ExecResult, error) {
		return runCmd(name, args...)
	}
	return s
}

// SetSelfLabel records which launchd label is this process.
// An explicit value wins. Otherwise launchd's XPC_SERVICE_NAME is used, which
// a LaunchAgent actually sets. "0" is launchd's placeholder for "no name", so
// it counts as unknown — treating it as a label would make a dev `go run`
// look like a job called "0".
func (s *Service) SetSelfLabel(explicit string) {
	s.selfLabel = resolveSelfLabel(explicit)
}

// SelfLabel returns the label SetSelfLabel resolved, or "" when this process
// is not a known launchd job.
func (s *Service) SelfLabel() string { return s.selfLabel }

// IsSelf reports whether label is this process's own launchd job.
func (s *Service) IsSelf(label string) bool {
	return s.selfLabel != "" && label == s.selfLabel
}

func resolveSelfLabel(explicit string) string {
	if explicit != "" {
		return explicit
	}
	env := os.Getenv("XPC_SERVICE_NAME")
	if env != "" && env != "0" {
		return env
	}
	return ""
}

// SetSourcesForTest replaces launchctl list, the plist scan, and command
// execution so a test outside this package can drive ListJobs without a live
// launchd. It also stubs print and print-disabled, which SetEnrichment would
// otherwise point at the real binaries, and it disarms process exit.
func (s *Service) SetSourcesForTest(
	list func() (string, error),
	scan func() []plist.ScanResult,
	execFn func(name string, args ...string) (*ExecResult, error),
) {
	if list != nil {
		s.runList = list
	}
	if scan != nil {
		s.scanPlist = scan
	}
	if execFn != nil {
		s.runExec = execFn
	}
	s.runPrint = func(_, label string) (string, string, error) {
		return "", "Could not find service " + label, ErrNotLoaded
	}
	s.runDisabled = func(string) (string, error) { return "{\n}", nil }
	s.exitFn = func(code int) {
		panic(fmt.Sprintf("os.Exit(%d) in test", code))
	}
	s.startDetached = func(name string, args ...string) error {
		return fmt.Errorf("detached spawn in test: %s %v", name, args)
	}
}

func (s *Service) defaultRunList() (string, error) {
	result, err := runCmd("launchctl", "list")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (s *Service) defaultScanPlist() []plist.ScanResult {
	return plist.ScanAll(plist.ScanDirs(), s.cache)
}

// ListJobs returns all launchd jobs visible to the current user, with plist
// data merged when a matching plist file is found. Plist files whose Label
// is not present in launchctl output are appended as synthetic
// StatusOffline jobs so the UI can surface unloaded plists.
func (s *Service) ListJobs() ([]Job, error) {
	output, err := s.runList()
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	entries := ParseListOutput(output)

	// Build label → plist lookup from scanned plist files.
	plistMap := make(map[string]plist.ScanResult)
	for _, sr := range s.scanPlist() {
		plistMap[sr.Data.Label] = sr
	}

	now := s.now()
	jobs := make([]Job, 0, len(entries)+len(plistMap))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		seen[e.Label] = struct{}{}
		sr, hasPlist := plistMap[e.Label]
		var lastRunAt *time.Time
		if hasPlist {
			if t := plist.LastRunAt(sr.Data.StandardOutPath, sr.Data.StandardErrorPath, s.stat()); !t.IsZero() {
				lastRunAt = &t
			}
		}

		job := Job{
			Label:          e.Label,
			PID:            e.PID,
			LastExitStatus: e.LastExitStatus,
			Status:         DeriveStatus(e.PID, e.LastExitStatus, sr.Data, lastRunAt, now, s.window),
			LastRunAt:      lastRunAt,
		}

		if hasPlist {
			job.PlistPath = sr.Path
			job.Program = sr.Data.Program
			job.ProgramArgs = sr.Data.ProgramArguments
			job.StandardOutPath = sr.Data.StandardOutPath
			job.StandardErrPath = sr.Data.StandardErrorPath
			job.WorkingDirectory = sr.Data.WorkingDirectory
			job.RunAtLoad = sr.Data.RunAtLoad
			job.KeepAlive = sr.Data.KeepAlive
			job.Domain = s.detectDomain(sr.Path)
			job.StartInterval = sr.Data.StartInterval
			job.StartCalendarInterval = sr.Data.StartCalendarInterval
			if next := computeNextRun(sr.Data, lastRunAt, now); !next.IsZero() {
				n := next
				job.NextRunAt = &n
			}

			if job.Program == "" && len(job.ProgramArgs) > 0 {
				job.Program = job.ProgramArgs[0]
			}
		}

		jobs = append(jobs, job)
	}

	// Offline merge: plists present on disk but absent from launchctl list.
	for label, sr := range plistMap {
		if _, ok := seen[label]; ok {
			continue
		}
		var lastRunAt *time.Time
		if t := plist.LastRunAt(sr.Data.StandardOutPath, sr.Data.StandardErrorPath, s.stat()); !t.IsZero() {
			lastRunAt = &t
		}
		job := Job{
			Label:                 label,
			PID:                   0,
			Status:                StatusOffline,
			PlistPath:             sr.Path,
			Program:               sr.Data.Program,
			ProgramArgs:           sr.Data.ProgramArguments,
			StandardOutPath:       sr.Data.StandardOutPath,
			StandardErrPath:       sr.Data.StandardErrorPath,
			WorkingDirectory:      sr.Data.WorkingDirectory,
			RunAtLoad:             sr.Data.RunAtLoad,
			KeepAlive:             sr.Data.KeepAlive,
			Domain:                s.detectDomain(sr.Path),
			StartInterval:         sr.Data.StartInterval,
			StartCalendarInterval: sr.Data.StartCalendarInterval,
			LastRunAt:             lastRunAt,
		}
		if next := computeNextRun(sr.Data, lastRunAt, now); !next.IsZero() {
			n := next
			job.NextRunAt = &n
		}
		if job.Program == "" && len(job.ProgramArgs) > 0 {
			job.Program = job.ProgramArgs[0]
		}
		jobs = append(jobs, job)
	}

	s.enrich(jobs)
	return jobs, nil
}

// now returns the service's clock, defaulting to time.Now for zero-value services.
func (s *Service) now() time.Time {
	if s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}

// stat returns the service's stat function, defaulting to os.Stat.
func (s *Service) stat() func(string) (os.FileInfo, error) {
	if s.statFn == nil {
		return os.Stat
	}
	return s.statFn
}

// computeNextRun picks the next fire time from StartCalendarInterval entries or
// StartInterval, returning zero time if neither is configured.
func computeNextRun(data plist.PlistData, lastRun *time.Time, now time.Time) time.Time {
	if len(data.StartCalendarInterval) > 0 {
		return plist.NextCalendarFire(data.StartCalendarInterval, now)
	}
	if data.StartInterval > 0 {
		last := time.Time{}
		if lastRun != nil {
			last = *lastRun
		}
		return plist.NextIntervalFire(data.StartInterval, last, now)
	}
	return time.Time{}
}

// GetJob returns a single job by label. Returns an error wrapping ErrNotFound
// if the label is not present in the current job list.
func (s *Service) GetJob(label string) (*Job, error) {
	jobs, err := s.ListJobs()
	if err != nil {
		return nil, err
	}

	for i := range jobs {
		if jobs[i].Label == label {
			return &jobs[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrNotFound, label)
}

// Reload restarts a job.
//
// A job that is already loaded is restarted with `kickstart -k`: launchd
// terminates it and starts it again in one message. `bootout` is asynchronous,
// so a bootstrap that follows immediately races the teardown and fails with
// EIO (exit 5). The bootout has already removed the job, so it is left down.
// bootout-then-bootstrap is only for a job launchd does not have loaded, or
// for a kickstart that failed because that fresh print was stale.
func (s *Service) Reload(label string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}
	// bootout of our own label SIGTERMs this process before bootstrap runs,
	// and launchd will not respawn a job that bootout removed. The caller
	// answers the client, then RestartSelf lets launchd do the restart.
	if s.IsSelf(label) {
		return fmt.Errorf("reload %s: %w", label, ErrSelfRestart)
	}

	// Fresh read, same as Start: a cached "loaded" would kickstart a label
	// launchd already dropped.
	s.invalidateActionCaches(label)
	defer s.invalidateActionCaches(label)

	if _, loaded := s.printInfoFor(label); loaded {
		if _, err := s.runExec("launchctl", "kickstart", "-k", s.domain+"/"+label); err == nil {
			return nil
		}
		// Fall through: the memoised loaded state can be wrong, the same way
		// Start already assumes it can be.
	}

	job, err := s.GetJob(label)
	if err != nil {
		return fmt.Errorf("reload %s: %w", label, err)
	}
	if job.PlistPath == "" {
		return fmt.Errorf("reload %s: no plist path available", label)
	}

	// bootout — ignore the error (the job may not be loaded).
	s.runExec("launchctl", "bootout", s.domain+"/"+label)

	_, err = s.runExec("launchctl", "bootstrap", s.domain, job.PlistPath)
	if err != nil {
		// bootout already ran and its error was ignored, so a failed bootstrap
		// leaves the job unloaded. Say so: the UI otherwise only shows
		// "reload failed" while the job is actually down.
		return fmt.Errorf("reload %s: job left unloaded: %w", label, err)
	}
	return nil
}

// RestartSelf asks launchd to bring this process back. It never bootouts:
// that would delete the job out from under us. A KeepAlive job comes back
// because we exit; anything else needs a detached kickstart, because this
// process will not be around to run it.
func (s *Service) RestartSelf() error {
	label := s.selfLabel
	if label == "" {
		return fmt.Errorf("self-restart: dashboard label is unknown")
	}
	// The kickstart helper is a shell one-liner. A label that failed this
	// check could break out of it; launchd labels never need to.
	if err := ValidateLabel(label); err != nil {
		return fmt.Errorf("self-restart: %w", err)
	}
	job, err := s.GetJob(label)
	if err != nil {
		return fmt.Errorf("self-restart %s: %w", label, err)
	}
	if job.KeepAlive {
		fmt.Println("deployboard: self-restart — exiting so launchd respawns the job")
		exit := s.exitFn
		if exit == nil {
			exit = os.Exit
		}
		exit(0)
		return nil
	}
	spawn := s.startDetached
	if spawn == nil {
		spawn = startDetachedCmd
	}
	// The helper sleeps so the HTTP response can leave the socket before
	// kickstart replaces us. Its own session is what keeps it alive after
	// this process is gone.
	target := s.domain + "/" + label
	if err := spawn("sh", "-c", "sleep 0.5; exec launchctl kickstart -k "+target); err != nil {
		return fmt.Errorf("self-restart %s: %w", label, err)
	}
	return nil
}

// detachedCommand builds the kickstart helper. Setsid is the whole point:
// a child in our session dies with us, and then nothing runs kickstart.
func detachedCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

func startDetachedCmd(name string, args ...string) error {
	return detachedCommand(name, args...).Start()
}

// Start brings a job up and keeps it up.
//
// Order matters: a label that was retired with `launchctl disable` cannot be
// bootstrapped (launchd answers "Operation not permitted" / EBUSY), so Start
// re-enables first, then either kickstarts a loaded job or bootstraps a plist
// that is not loaded yet.
func (s *Service) Start(label string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}

	// Ignore the error: enabling an already-enabled label is not a failure.
	s.runExec("launchctl", "enable", s.domain+"/"+label)
	// Read the loaded state fresh: a Stop seconds earlier unloaded this job, and
	// a cached "loaded" would send us down the kickstart path for a label
	// launchd no longer knows about.
	s.invalidateActionCaches(label)
	defer s.invalidateActionCaches(label)

	if _, loaded := s.printInfoFor(label); loaded {
		if _, err := s.runExec("launchctl", "kickstart", s.domain+"/"+label); err == nil {
			return nil
		}
		// Fall through to bootstrap: the memoised state can still be wrong when
		// something else (a login, another tool) unloaded the job in between.
	}

	job, err := s.GetJob(label)
	if err != nil {
		return fmt.Errorf("start %s: %w", label, err)
	}
	if job.PlistPath == "" {
		return fmt.Errorf("start %s: no plist path available", label)
	}
	if _, err := s.runExec("launchctl", "bootstrap", s.domain, job.PlistPath); err != nil {
		return fmt.Errorf("start %s: %w", label, err)
	}
	return nil
}

// Stop makes a job not run, using the mechanism that actually holds for it.
//
// This is the difference the dashboard has to make for the user: `launchctl kill
// SIGTERM` on a job with KeepAlive is not a stop, it is a restart — launchd
// respawns it immediately (a dev web server doing that is exactly how a job
// reaches thousands of runs). So:
//
//   - KeepAlive (or RunAtLoad with KeepAlive): bootout. The job leaves launchd's
//     job list and stays down until Start (bootstrap) or the next login. This
//     covers a job that is between restarts right now (pid 0 but loaded): it is
//     exactly the case where a signal is useless and only an unload helps.
//   - anything else: SIGTERM, leaving the job loaded (upstream behaviour). A job
//     that is loaded but idle (pid 0, e.g. a schedule that is not due) has
//     nothing to stop, which is a no-op, not a failure.
//
// Stopping an already-unloaded job is not an error either: the desired state holds.
func (s *Service) Stop(label string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}

	s.invalidateActionCaches(label)
	defer s.invalidateActionCaches(label)

	job, err := s.GetJob(label)
	if err != nil {
		return fmt.Errorf("stop %s: %w", label, err)
	}

	if !job.KeepAlive {
		// Nothing running and nothing that comes back by itself: the desired
		// state already holds, so this is a no-op rather than a failed signal.
		if job.PID == 0 {
			return nil
		}
		if _, err := s.runExec("launchctl", "kill", "SIGTERM", s.domain+"/"+label); err != nil {
			// A job that is already unloaded has no process to signal.
			if _, loaded := s.printInfoFor(label); !loaded {
				return nil
			}
			return fmt.Errorf("stop %s: %w", label, err)
		}
		return nil
	}

	// Bootout is idempotent from the caller's point of view: a job that is not
	// loaded is already in the state the caller asked for.
	s.runExec("launchctl", "bootout", s.domain+"/"+label)
	return nil
}

// Disable retires a job: launchd will not start it again, including at login and
// after a reboot, and the dashboard shows it as `disabled` rather than broken.
// The plist file is left alone — this is a launchd override, not an edit.
func (s *Service) Disable(label string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}
	if _, err := s.runExec("launchctl", "disable", s.domain+"/"+label); err != nil {
		return fmt.Errorf("disable %s: %w", label, err)
	}
	defer s.invalidateActionCaches(label)
	// Unload it too, otherwise a running instance keeps running until it exits.
	s.runExec("launchctl", "bootout", s.domain+"/"+label)
	return nil
}

// Enable undoes Disable. It does not start the job; that is Start's job, so the
// two buttons stay independent and predictable.
func (s *Service) Enable(label string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}
	if _, err := s.runExec("launchctl", "enable", s.domain+"/"+label); err != nil {
		return fmt.Errorf("enable %s: %w", label, err)
	}
	s.invalidateActionCaches(label)
	return nil
}

// detectDomain determines whether a plist path belongs to the user or global domain.
func (s *Service) detectDomain(plistPath string) string {
	userDir := filepath.Join(s.homeDir, "Library", "LaunchAgents")
	if strings.HasPrefix(plistPath, userDir) {
		return "user"
	}
	if strings.HasPrefix(plistPath, "/Library/LaunchAgents") {
		return "global"
	}
	return "user"
}
