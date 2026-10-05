package launchd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/plist"
)

// execCall records a single command invocation for test assertions.
type execCall struct {
	name string
	args []string
}

func argsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// newTestService creates a Service with injectable list output and plist scan results.
func newTestService(listOutput string, listErr error, plists []plist.ScanResult) *Service {
	return &Service{
		uid:     501,
		domain:  "gui/501",
		homeDir: "/Users/testuser",
		runList: func() (string, error) {
			return listOutput, listErr
		},
		scanPlist: func() []plist.ScanResult {
			return plists
		},
	}
}

// newActionTestService wires a Service whose exec calls are recorded, so the
// stop/start/enable/disable semantics can be asserted without touching launchd.
// `loaded` controls what `launchctl print` reports for the label.
func newActionTestService(plists []plist.ScanResult, loaded bool) (*Service, *[]execCall) {
	calls := &[]execCall{}
	s := newTestService("PID\tStatus\tLabel\n", nil, plists)
	s.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	s.statFn = func(string) (os.FileInfo, error) { return nil, errors.New("no file") }
	s.runExec = func(name string, args ...string) (*ExecResult, error) {
		*calls = append(*calls, execCall{name: name, args: args})
		return &ExecResult{Stdout: "", Stderr: ""}, nil
	}
	s.runPrint = func(domain, label string) (string, string, error) {
		if !loaded {
			return "", "Could not find service " + label, errors.New("exit 113")
		}
		return "\tstate = running\n\tpid = 73087\n\truns = 28\n\tlast exit code = 0\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}
	s.runDisabled = func(string) (string, error) { return "{\n}", nil }
	// A classifier is required for enrich() to run at all (it layers category,
	// runs and the disabled status on top of upstream's job list).
	s.SetEnrichment(inventory.New(inventory.DefaultConfig()), time.Minute)
	return s, calls
}

func hasCall(calls []execCall, want ...string) bool {
	for _, c := range calls {
		if c.name == "launchctl" && argsEqual(c.args, want) {
			return true
		}
	}
	return false
}

func callIndex(calls []execCall, want ...string) int {
	for i, c := range calls {
		if c.name == "launchctl" && argsEqual(c.args, want) {
			return i
		}
	}
	return -1
}

// A KeepAlive job cannot be stopped with a signal — launchd respawns it. Stop
// must unload it instead, or the button looks broken (or worse, looks fine).
func TestStop_KeepAliveJobUnloadsInsteadOfSignalling(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.keepalive.plist",
		Data: plist.PlistData{
			Label:            "com.example.keepalive",
			ProgramArguments: []string{"/usr/local/bin/web"},
			RunAtLoad:        true,
			KeepAlive:        true,
		},
	}}
	svc, calls := newActionTestService(plists, true)

	if err := svc.Stop("com.example.keepalive"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !hasCall(*calls, "bootout", "gui/501/com.example.keepalive") {
		t.Errorf("expected a bootout, got %v", *calls)
	}
	if hasCall(*calls, "kill", "SIGTERM", "gui/501/com.example.keepalive") {
		t.Errorf("a KeepAlive job must not be signalled — launchd would restart it: %v", *calls)
	}
}

// A job without KeepAlive keeps upstream behaviour: SIGTERM, job stays loaded.
func TestStop_PlainJobSignals(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.oneshot.plist",
		Data: plist.PlistData{
			Label:            "com.example.oneshot",
			ProgramArguments: []string{"/usr/local/bin/oneshot"},
		},
	}}
	svc, calls := newActionTestService(plists, true)

	if err := svc.Stop("com.example.oneshot"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !hasCall(*calls, "kill", "SIGTERM", "gui/501/com.example.oneshot") {
		t.Errorf("expected SIGTERM, got %v", *calls)
	}
	if hasCall(*calls, "bootout", "gui/501/com.example.oneshot") {
		t.Errorf("a plain job should stay loaded: %v", *calls)
	}
}

// A loaded job with no pid and no KeepAlive has nothing to stop (a schedule that
// is not due): that is a no-op, not a failed signal.
func TestStop_IdleJobIsANoOp(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.idle.plist",
		Data: plist.PlistData{Label: "com.example.idle", ProgramArguments: []string{"/usr/local/bin/idle"}},
	}}
	var calls []execCall
	svc := newTestServiceWithExec("PID\tStatus\tLabel\n", plists, &calls, nil)
	svc.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	svc.statFn = func(string) (os.FileInfo, error) { return nil, errors.New("no file") }
	svc.runPrint = func(domain, label string) (string, string, error) {
		// Loaded, but no pid: launchd knows it, nothing is running.
		return "\tstate = not running\n\truns = 3\n\tlast exit code = 0\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}
	svc.runDisabled = func(string) (string, error) { return "{\n}", nil }
	svc.SetEnrichment(inventory.New(inventory.DefaultConfig()), time.Minute)

	if err := svc.Stop("com.example.idle"); err != nil {
		t.Fatalf("Stop on an idle scheduled job: %v, want nil", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected no launchctl call, got %v", calls)
	}
}

// The restart-loop case: KeepAlive, pid 0 right now, still being restarted by
// launchd. Only an unload stops it, so Stop must bootout and not sit on its hands
// because there is no pid at this instant.
func TestStop_KeepAliveJobBetweenRestartsStillUnloads(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.flap.plist",
		Data: plist.PlistData{
			Label:            "com.example.flap",
			ProgramArguments: []string{"/usr/local/bin/flap"},
			KeepAlive:        true,
			RunAtLoad:        true,
		},
	}}
	var calls []execCall
	svc := newTestServiceWithExec("PID\tStatus\tLabel\n", plists, &calls, nil)
	svc.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	svc.statFn = func(string) (os.FileInfo, error) { return nil, errors.New("no file") }
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "\tstate = not running\n\truns = 412\n\tlast exit code = 1\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}
	svc.runDisabled = func(string) (string, error) { return "{\n}", nil }
	svc.SetEnrichment(inventory.New(inventory.DefaultConfig()), time.Minute)

	job, err := svc.GetJob("com.example.flap")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.PID != 0 {
		t.Fatalf("fixture should have no pid, got %d", job.PID)
	}

	if err := svc.Stop("com.example.flap"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !hasCall(calls, "bootout", "gui/501/com.example.flap") {
		t.Errorf("a KeepAlive job with no pid must still be unloaded, got %v", calls)
	}
}

// Stopping something that is already down is a no-op, not an error.
func TestStop_AlreadyUnloadedIsNotAnError(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.gone.plist",
		Data: plist.PlistData{
			Label:            "com.example.gone",
			ProgramArguments: []string{"/usr/local/bin/gone"},
		},
	}}
	svc, _ := newActionTestService(plists, false)

	if err := svc.Stop("com.example.gone"); err != nil {
		t.Errorf("Stop on an unloaded job: %v, want nil", err)
	}
}

// Start has to re-enable a retired label first, otherwise bootstrap fails.
func TestStart_BootstrapsUnloadedJobAfterEnabling(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.web.plist",
		Data: plist.PlistData{
			Label:            "com.example.web",
			ProgramArguments: []string{"/usr/local/bin/web"},
			KeepAlive:        true,
		},
	}}
	svc, calls := newActionTestService(plists, false)

	if err := svc.Start("com.example.web"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	enableAt := callIndex(*calls, "enable", "gui/501/com.example.web")
	bootAt := callIndex(*calls, "bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.web.plist")
	if enableAt < 0 || bootAt < 0 {
		t.Fatalf("expected enable + bootstrap, got %v", *calls)
	}
	if enableAt > bootAt {
		t.Errorf("enable must come before bootstrap: %v", *calls)
	}
}

// A loaded job is kickstarted rather than bootstrapped twice.
func TestStart_KickstartsLoadedJob(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.web.plist",
		Data: plist.PlistData{Label: "com.example.web", ProgramArguments: []string{"/usr/local/bin/web"}},
	}}
	svc, calls := newActionTestService(plists, true)

	if err := svc.Start("com.example.web"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !hasCall(*calls, "kickstart", "gui/501/com.example.web") {
		t.Errorf("expected kickstart, got %v", *calls)
	}
	if hasCall(*calls, "bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.web.plist") {
		t.Errorf("a loaded job must not be bootstrapped again: %v", *calls)
	}
}

func TestDisable_DisablesThenUnloads(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.retired.plist",
		Data: plist.PlistData{Label: "com.example.retired", ProgramArguments: []string{"/usr/local/bin/x"}},
	}}
	svc, calls := newActionTestService(plists, true)

	if err := svc.Disable("com.example.retired"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	disAt := callIndex(*calls, "disable", "gui/501/com.example.retired")
	outAt := callIndex(*calls, "bootout", "gui/501/com.example.retired")
	if disAt < 0 {
		t.Fatalf("expected launchctl disable, got %v", *calls)
	}
	if outAt < 0 || outAt < disAt {
		t.Errorf("disable must come before bootout, so it cannot restart in between: %v", *calls)
	}
}

func TestEnable_OnlyEnables(t *testing.T) {
	svc, calls := newActionTestService(nil, false)

	if err := svc.Enable("com.example.retired"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !hasCall(*calls, "enable", "gui/501/com.example.retired") {
		t.Errorf("expected launchctl enable, got %v", *calls)
	}
	if hasCall(*calls, "kickstart", "gui/501/com.example.retired") {
		t.Errorf("Enable must not start the job — that is Start's job: %v", *calls)
	}
}

func TestListJobs_MergesPlistData(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"584\t0\tcom.example.myapp\n" +
		"-\t0\tcom.example.stopped\n" +
		"-\t78\tcom.example.broken\n"

	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{
				Label:             "com.example.myapp",
				Program:           "/usr/local/bin/myapp",
				ProgramArguments:  []string{"/usr/local/bin/myapp", "--daemon"},
				StandardOutPath:   "/tmp/myapp.stdout.log",
				StandardErrorPath: "/tmp/myapp.stderr.log",
				RunAtLoad:         true,
				KeepAlive:         false,
			},
		},
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.stopped.plist",
			Data: plist.PlistData{
				Label:            "com.example.stopped",
				ProgramArguments: []string{"/usr/local/bin/stopped"},
				RunAtLoad:        false,
			},
		},
	}

	svc := newTestService(listOutput, nil, plists)
	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Build label→job map for easier assertions.
	byLabel := make(map[string]Job)
	for _, j := range jobs {
		byLabel[j.Label] = j
	}

	// com.example.myapp — running, merged with plist
	app := byLabel["com.example.myapp"]
	if app.PID != 584 {
		t.Errorf("myapp PID: got %d, want 584", app.PID)
	}
	if app.Status != StatusRunning {
		t.Errorf("myapp Status: got %q, want %q", app.Status, StatusRunning)
	}
	if app.Program != "/usr/local/bin/myapp" {
		t.Errorf("myapp Program: got %q, want %q", app.Program, "/usr/local/bin/myapp")
	}
	if len(app.ProgramArgs) != 2 || app.ProgramArgs[1] != "--daemon" {
		t.Errorf("myapp ProgramArgs: got %v", app.ProgramArgs)
	}
	if app.StandardOutPath != "/tmp/myapp.stdout.log" {
		t.Errorf("myapp StandardOutPath: got %q", app.StandardOutPath)
	}
	if app.StandardErrPath != "/tmp/myapp.stderr.log" {
		t.Errorf("myapp StandardErrPath: got %q", app.StandardErrPath)
	}
	if !app.RunAtLoad {
		t.Error("myapp RunAtLoad: expected true")
	}
	if app.KeepAlive {
		t.Error("myapp KeepAlive: expected false")
	}
	if app.PlistPath != "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist" {
		t.Errorf("myapp PlistPath: got %q", app.PlistPath)
	}
	if app.Domain != "user" {
		t.Errorf("myapp Domain: got %q, want %q", app.Domain, "user")
	}

	// com.example.stopped — stopped, merged with plist (no Program field, uses ProgramArguments[0])
	stopped := byLabel["com.example.stopped"]
	if stopped.Status != StatusStopped {
		t.Errorf("stopped Status: got %q, want %q", stopped.Status, StatusStopped)
	}
	if stopped.Program != "/usr/local/bin/stopped" {
		t.Errorf("stopped Program: got %q, want %q", stopped.Program, "/usr/local/bin/stopped")
	}

	// com.example.broken — error, no matching plist
	broken := byLabel["com.example.broken"]
	if broken.Status != StatusError {
		t.Errorf("broken Status: got %q, want %q", broken.Status, StatusError)
	}
	if broken.LastExitStatus != 78 {
		t.Errorf("broken LastExitStatus: got %d, want 78", broken.LastExitStatus)
	}
	if broken.PlistPath != "" {
		t.Errorf("broken PlistPath: expected empty, got %q", broken.PlistPath)
	}
}

func TestListJobs_NoMatchingPlist(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"100\t0\tcom.example.noplist\n"

	svc := newTestService(listOutput, nil, nil)
	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]
	if job.Label != "com.example.noplist" {
		t.Errorf("Label: got %q", job.Label)
	}
	if job.PID != 100 {
		t.Errorf("PID: got %d, want 100", job.PID)
	}
	if job.Status != StatusRunning {
		t.Errorf("Status: got %q, want %q", job.Status, StatusRunning)
	}
	if job.Program != "" {
		t.Errorf("Program: expected empty, got %q", job.Program)
	}
	if job.PlistPath != "" {
		t.Errorf("PlistPath: expected empty, got %q", job.PlistPath)
	}
}

func TestListJobs_CommandError(t *testing.T) {
	svc := newTestService("", errors.New("launchctl failed"), nil)
	_, err := svc.ListJobs()
	if err == nil {
		t.Fatal("expected error from ListJobs when command fails")
	}
}

func TestListJobs_EmptyOutput(t *testing.T) {
	svc := newTestService("PID\tStatus\tLabel\n", nil, nil)
	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobs_DomainDetection(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"100\t0\tcom.example.user\n" +
		"200\t0\tcom.example.global\n"

	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.user.plist",
			Data: plist.PlistData{Label: "com.example.user"},
		},
		{
			Path: "/Library/LaunchAgents/com.example.global.plist",
			Data: plist.PlistData{Label: "com.example.global"},
		},
	}

	svc := newTestService(listOutput, nil, plists)
	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}

	byLabel := make(map[string]Job)
	for _, j := range jobs {
		byLabel[j.Label] = j
	}

	if byLabel["com.example.user"].Domain != "user" {
		t.Errorf("user domain: got %q, want %q", byLabel["com.example.user"].Domain, "user")
	}
	if byLabel["com.example.global"].Domain != "global" {
		t.Errorf("global domain: got %q, want %q", byLabel["com.example.global"].Domain, "global")
	}
}

func TestListJobs_ProgramFallbackToArgs(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"-\t0\tcom.example.argsonly\n"

	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.argsonly.plist",
			Data: plist.PlistData{
				Label:            "com.example.argsonly",
				ProgramArguments: []string{"/usr/bin/env", "python3", "script.py"},
			},
		},
	}

	svc := newTestService(listOutput, nil, plists)
	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// When Program is empty, should use ProgramArguments[0]
	if jobs[0].Program != "/usr/bin/env" {
		t.Errorf("Program: got %q, want %q", jobs[0].Program, "/usr/bin/env")
	}
	if len(jobs[0].ProgramArgs) != 3 {
		t.Errorf("ProgramArgs length: got %d, want 3", len(jobs[0].ProgramArgs))
	}
}

func TestGetJob_Found(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"584\t0\tcom.example.myapp\n" +
		"-\t0\tcom.example.other\n"

	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{
				Label:   "com.example.myapp",
				Program: "/usr/local/bin/myapp",
			},
		},
	}

	svc := newTestService(listOutput, nil, plists)
	job, err := svc.GetJob("com.example.myapp")
	if err != nil {
		t.Fatalf("GetJob() error: %v", err)
	}

	if job.Label != "com.example.myapp" {
		t.Errorf("Label: got %q", job.Label)
	}
	if job.PID != 584 {
		t.Errorf("PID: got %d, want 584", job.PID)
	}
	if job.Program != "/usr/local/bin/myapp" {
		t.Errorf("Program: got %q", job.Program)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n" +
		"584\t0\tcom.example.myapp\n"

	svc := newTestService(listOutput, nil, nil)
	_, err := svc.GetJob("com.example.nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent label")
	}
}

func TestGetJob_CommandError(t *testing.T) {
	svc := newTestService("", errors.New("launchctl failed"), nil)
	_, err := svc.GetJob("com.example.any")
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}

func TestDetectDomain(t *testing.T) {
	svc := &Service{homeDir: "/Users/testuser"}

	tests := []struct {
		name      string
		plistPath string
		want      string
	}{
		{
			"user agent",
			"/Users/testuser/Library/LaunchAgents/com.example.plist",
			"user",
		},
		{
			"global agent",
			"/Library/LaunchAgents/com.example.plist",
			"global",
		},
		{
			"unknown path defaults to user",
			"/some/random/path/com.example.plist",
			"user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.detectDomain(tt.plistPath)
			if got != tt.want {
				t.Errorf("detectDomain(%q) = %q, want %q", tt.plistPath, got, tt.want)
			}
		})
	}
}

func TestDetectDomain_GlobalPathNotConfusedWithUser(t *testing.T) {
	// Ensure /Library/LaunchAgents is not matched as user when home is /Library
	svc := &Service{homeDir: "/Users/testuser"}
	userPath := filepath.Join(svc.homeDir, "Library", "LaunchAgents", "test.plist")
	globalPath := "/Library/LaunchAgents/test.plist"

	if got := svc.detectDomain(userPath); got != "user" {
		t.Errorf("user path: got %q, want %q", got, "user")
	}
	if got := svc.detectDomain(globalPath); got != "global" {
		t.Errorf("global path: got %q, want %q", got, "global")
	}
}

// ---------------------------------------------------------------------------
// S05: validateLabel
// ---------------------------------------------------------------------------

func TestValidateLabel(t *testing.T) {
	tests := []struct {
		label string
		valid bool
	}{
		// Valid labels
		{"com.apple.Finder", true},
		{"com.example.my-app_v2", true},
		{"com.example.myapp", true},
		{"myapp", true},
		{"a", true},
		{"A.B.C-d_e.123", true},

		// Invalid labels — injection attempts
		{"", false},
		{"com.example;rm -rf /", false},
		{"com.example.$(evil)", false},
		{"com.example evil", false},
		{"com.example/../etc/passwd", false},
		{"com.example.`evil`", false},
		{"com.example|evil", false},
		{"com.example&evil", false},
		{"com.example\nevil", false},
		{"com.example\tevil", false},
		{"label with spaces", false},
	}

	for _, tt := range tests {
		name := tt.label
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			err := ValidateLabel(tt.label)
			if tt.valid && err != nil {
				t.Errorf("ValidateLabel(%q) unexpected error: %v", tt.label, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ValidateLabel(%q) expected error, got nil", tt.label)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// S05: Reload
// ---------------------------------------------------------------------------

// newTestServiceWithExec extends newTestService with a runExec capture.
func newTestServiceWithExec(listOutput string, plists []plist.ScanResult, calls *[]execCall, execErr func(int) error) *Service {
	svc := newTestService(listOutput, nil, plists)
	callIdx := 0
	svc.runExec = func(name string, args ...string) (*ExecResult, error) {
		*calls = append(*calls, execCall{name, args})
		idx := callIdx
		callIdx++
		if execErr != nil {
			if err := execErr(idx); err != nil {
				return &ExecResult{}, err
			}
		}
		return &ExecResult{}, nil
	}
	return svc
}

// newLoadedTestService is newTestServiceWithExec plus a `launchctl print` that
// reports the label as loaded, so action errors propagate instead of being
// swallowed as "the job is already down".
func newLoadedTestService(plists []plist.ScanResult, calls *[]execCall, execErr func(int) error) *Service {
	svc := newTestServiceWithExec("PID\tStatus\tLabel\n", plists, calls, execErr)
	svc.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	svc.statFn = func(string) (os.FileInfo, error) { return nil, errors.New("no file") }
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "\tstate = running\n\tpid = 73087\n\truns = 28\n\tlast exit code = 0\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}
	svc.runDisabled = func(string) (string, error) { return "{\n}", nil }
	svc.SetEnrichment(inventory.New(inventory.DefaultConfig()), time.Minute)
	return svc
}

// Regression: Stop unloads the job, so the memoised `launchctl print` must be
// dropped — otherwise Start believes the job is still loaded, kickstarts a
// label launchd no longer knows, and fails with "Could not find service".
func TestStart_AfterStopDoesNotTrustThePrintCache(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.keepalive.plist",
		Data: plist.PlistData{
			Label:            "com.example.keepalive",
			ProgramArguments: []string{"/usr/local/bin/web"},
			KeepAlive:        true,
		},
	}}
	var calls []execCall
	loaded := true
	svc := newTestServiceWithExec("PID\tStatus\tLabel\n", plists, &calls, nil)
	svc.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	svc.statFn = func(string) (os.FileInfo, error) { return nil, errors.New("no file") }
	svc.runPrint = func(domain, label string) (string, string, error) {
		if !loaded {
			return "", "Could not find service " + label, errors.New("exit 113")
		}
		return "\tstate = running\n\tpid = 73087\n\truns = 1\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}
	svc.runDisabled = func(string) (string, error) { return "{\n}", nil }

	// Prime the cache the way a UI poll would.
	if _, ok := svc.printInfoFor("com.example.keepalive"); !ok {
		t.Fatal("fixture should start out loaded")
	}

	if err := svc.Stop("com.example.keepalive"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	loaded = false // launchd unloaded it; the next print would report that

	if err := svc.Start("com.example.keepalive"); err != nil {
		t.Fatalf("Start after Stop: %v", err)
	}
	if !hasCall(calls, "bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.keepalive.plist") {
		t.Errorf("expected a bootstrap after the unload, got %v", calls)
	}
}

func TestDisable_RefreshesTheDisabledStatus(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.retired.plist",
		Data: plist.PlistData{Label: "com.example.retired", ProgramArguments: []string{"/usr/local/bin/x"}},
	}}
	// Disabled and unloaded: launchd print has nothing to say about this label.
	svc, _ := newActionTestService(plists, false)

	// Prime the (empty) disabled map, as a poll before the click would.
	svc.runDisabled = func(string) (string, error) { return "{\n}", nil }
	if _, err := svc.ListJobs(); err != nil {
		t.Fatalf("ListJobs: %v", err)
	}

	if err := svc.Disable("com.example.retired"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	// launchd now reports it disabled; the cache must not hide that.
	svc.runDisabled = func(string) (string, error) {
		return "{\n  \"com.example.retired\" => disabled\n}", nil
	}

	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs after Disable: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.Label == "com.example.retired" {
			found = true
			if j.Status != StatusDisabled {
				t.Errorf("status after Disable = %q, want %q (the disabled lookup must be re-read)", j.Status, StatusDisabled)
			}
		}
	}
	if !found {
		t.Fatalf("the job disappeared from the listing: %v", jobs)
	}
}

func TestReload_Success(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n584\t0\tcom.example.myapp\n"
	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{Label: "com.example.myapp"},
		},
	}

	var calls []execCall
	svc := newTestServiceWithExec(listOutput, plists, &calls, nil)
	// Loaded per launchctl print: kickstart -k, never bootout. bootout is
	// async, so a bootstrap right after it races the teardown (EIO, exit 5)
	// and leaves the job down.
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "\tstate = running\n\tpid = 584\n\truns = 4\n\tpath = /Users/testuser/Library/LaunchAgents/" + label + ".plist\n", "", nil
	}

	err := svc.Reload("com.example.myapp")
	if err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d: %v", len(calls), calls)
	}
	if calls[0].name != "launchctl" {
		t.Errorf("call 0 name: got %q", calls[0].name)
	}
	wantKick := []string{"kickstart", "-k", "gui/501/com.example.myapp"}
	if !argsEqual(calls[0].args, wantKick) {
		t.Errorf("kickstart args: got %v, want %v", calls[0].args, wantKick)
	}
	if hasCall(calls, "bootout", "gui/501/com.example.myapp") ||
		hasCall(calls, "bootstrap", "gui/501", plists[0].Path) {
		t.Errorf("a loaded job must not bootout or bootstrap: %v", calls)
	}
}

// A plist on disk that launchd does not have loaded still bootouts (ignored)
// and then bootstraps. kickstart would only fail with "Could not find service".
func TestReload_NotLoadedBootoutsThenBootstraps(t *testing.T) {
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
		Data: plist.PlistData{Label: "com.example.myapp"},
	}}
	svc, calls := newActionTestService(plists, false)

	if err := svc.Reload("com.example.myapp"); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	bootout := callIndex(*calls, "bootout", "gui/501/com.example.myapp")
	bootstrap := callIndex(*calls, "bootstrap", "gui/501", plists[0].Path)
	if bootout < 0 || bootstrap < 0 || bootout > bootstrap {
		t.Fatalf("want bootout then bootstrap, got %v", *calls)
	}
	if hasCall(*calls, "kickstart", "-k", "gui/501/com.example.myapp") {
		t.Fatalf("an unloaded job must not be kickstarted, got %v", *calls)
	}
}

// print said loaded, but kickstart failed — the memo can be stale. Fall
// through to bootout then bootstrap rather than reporting the kickstart error.
func TestReload_KickstartFailureFallsBack(t *testing.T) {
	const label = "com.example.myapp"
	plistPath := "/Users/testuser/Library/LaunchAgents/" + label + ".plist"
	plists := []plist.ScanResult{{
		Path: plistPath,
		Data: plist.PlistData{Label: label},
	}}
	var calls []execCall
	svc := newTestServiceWithExec("PID\tStatus\tLabel\n584\t0\t"+label+"\n", plists, &calls, func(idx int) error {
		if idx == 0 {
			return errors.New("Could not find service")
		}
		return nil
	})
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "\tstate = running\n\tpid = 584\n\truns = 4\n\tpath = " + plistPath + "\n", "", nil
	}

	if err := svc.Reload(label); err != nil {
		t.Fatalf("Reload() = %v, want fallback success", err)
	}
	kick := callIndex(calls, "kickstart", "-k", "gui/501/"+label)
	bootout := callIndex(calls, "bootout", "gui/501/"+label)
	bootstrap := callIndex(calls, "bootstrap", "gui/501", plistPath)
	if kick < 0 || bootout < 0 || bootstrap < 0 || !(kick < bootout && bootout < bootstrap) {
		t.Fatalf("want kickstart, then bootout, then bootstrap, got %v", calls)
	}
}

func TestReload_IgnoresBootoutError(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n584\t0\tcom.example.myapp\n"
	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{Label: "com.example.myapp"},
		},
	}

	var calls []execCall
	svc := newTestServiceWithExec(listOutput, plists, &calls, func(idx int) error {
		if idx == 0 {
			return errors.New("not loaded")
		}
		return nil
	})
	// print says not loaded, so Reload takes bootout (error ignored) then bootstrap.
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "", "Could not find service " + label, errors.New("exit 113")
	}

	err := svc.Reload("com.example.myapp")
	if err != nil {
		t.Fatalf("Reload() should succeed when bootout fails: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 exec calls, got %d", len(calls))
	}
}

func TestReload_BootstrapFails(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n584\t0\tcom.example.myapp\n"
	plists := []plist.ScanResult{
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{Label: "com.example.myapp"},
		},
	}

	var calls []execCall
	svc := newTestServiceWithExec(listOutput, plists, &calls, func(idx int) error {
		if idx == 1 {
			return errors.New("bootstrap failed: path not found")
		}
		return nil
	})
	svc.runPrint = func(domain, label string) (string, string, error) {
		return "", "Could not find service " + label, errors.New("exit 113")
	}

	err := svc.Reload("com.example.myapp")
	if err == nil {
		t.Fatal("Reload() should fail when bootstrap fails")
	}
	if !strings.Contains(err.Error(), "job left unloaded") {
		t.Errorf("bootstrap failure must say the job was left unloaded, got %v", err)
	}
}

func TestReload_JobNotFound(t *testing.T) {
	listOutput := "PID\tStatus\tLabel\n584\t0\tcom.example.other\n"
	var calls []execCall
	svc := newTestServiceWithExec(listOutput, nil, &calls, nil)

	err := svc.Reload("com.example.nonexistent")
	if err == nil {
		t.Fatal("Reload() should fail when job not found")
	}
	if len(calls) != 0 {
		t.Errorf("should not exec any commands, got %d calls", len(calls))
	}
}

func TestReload_NoPlistPath(t *testing.T) {
	// Job exists in launchctl list but has no matching plist file.
	listOutput := "PID\tStatus\tLabel\n584\t0\tcom.example.noplist\n"
	var calls []execCall
	svc := newTestServiceWithExec(listOutput, nil, &calls, nil)

	err := svc.Reload("com.example.noplist")
	if err == nil {
		t.Fatal("Reload() should fail when plist path is empty")
	}
	if len(calls) != 0 {
		t.Errorf("should not exec any commands, got %d calls", len(calls))
	}
}

func TestReload_InvalidLabel(t *testing.T) {
	var calls []execCall
	svc := newTestServiceWithExec("", nil, &calls, nil)

	err := svc.Reload("invalid;label")
	if err == nil {
		t.Fatal("Reload() should reject invalid label")
	}
	if len(calls) != 0 {
		t.Errorf("should not exec any commands for invalid label, got %d calls", len(calls))
	}
}

// ---------------------------------------------------------------------------
// S05: Start
// ---------------------------------------------------------------------------

func TestStart_Success(t *testing.T) {
	var calls []execCall
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
		Data: plist.PlistData{Label: "com.example.myapp"},
	}}
	svc := newTestServiceWithExec("", plists, &calls,
		func(idx int) error {
			if idx == 0 {
				return fmt.Errorf("enable failed")
			}
			return nil
		})
	// enable fails, but Start must not give up: a label that needs no enabling
	// is the common case, so the error is ignored and bootstrap still runs.
	if err := svc.Start("com.example.myapp"); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected enable + bootstrap, got %d calls: %v", len(calls), calls)
	}
	if calls[0].name != "launchctl" {
		t.Errorf("command: got %q", calls[0].name)
	}
	if want := []string{"enable", "gui/501/com.example.myapp"}; !argsEqual(calls[0].args, want) {
		t.Errorf("first call: got %v, want %v", calls[0].args, want)
	}
	want := []string{"bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist"}
	if !argsEqual(calls[1].args, want) {
		t.Errorf("second call: got %v, want %v", calls[1].args, want)
	}
}

func TestStart_InvalidLabel(t *testing.T) {
	var calls []execCall
	svc := newTestServiceWithExec("", nil, &calls, nil)

	err := svc.Start("$(evil)")
	if err == nil {
		t.Fatal("Start() should reject invalid label")
	}
	if len(calls) != 0 {
		t.Errorf("should not exec any commands, got %d calls", len(calls))
	}
}

func TestStart_ExecFails(t *testing.T) {
	var calls []execCall
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
		Data: plist.PlistData{Label: "com.example.myapp"},
	}}
	// idx 0 is the ignored `enable`; idx 1 is the bootstrap that must surface.
	svc := newTestServiceWithExec("", plists, &calls, func(idx int) error {
		if idx == 1 {
			return fmt.Errorf("bootstrap failed")
		}
		return nil
	})

	err := svc.Start("com.example.myapp")
	if err == nil {
		t.Fatal("Start() should propagate the bootstrap error")
	}
}

// ---------------------------------------------------------------------------
// S05: Stop
// ---------------------------------------------------------------------------

func TestStop_Success(t *testing.T) {
	var calls []execCall
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
		Data: plist.PlistData{Label: "com.example.myapp"},
	}}
	svc := newLoadedTestService(plists, &calls, nil)

	if err := svc.Stop("com.example.myapp"); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d: %v", len(calls), calls)
	}
	if calls[0].name != "launchctl" {
		t.Errorf("command: got %q", calls[0].name)
	}
	want := []string{"kill", "SIGTERM", "gui/501/com.example.myapp"}
	if !argsEqual(calls[0].args, want) {
		t.Errorf("args: got %v, want %v", calls[0].args, want)
	}
}

func TestStop_InvalidLabel(t *testing.T) {
	var calls []execCall
	svc := newTestServiceWithExec("", nil, &calls, nil)

	err := svc.Stop("label with spaces")
	if err == nil {
		t.Fatal("Stop() should reject invalid label")
	}
	if len(calls) != 0 {
		t.Errorf("should not exec any commands, got %d calls", len(calls))
	}
}

func TestStop_ExecFails(t *testing.T) {
	var calls []execCall
	plists := []plist.ScanResult{{
		Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
		Data: plist.PlistData{Label: "com.example.myapp"},
	}}
	// The job is loaded, so a failed kill is a real failure, not "already down".
	svc := newLoadedTestService(plists, &calls, func(int) error {
		return fmt.Errorf("kill failed")
	})

	err := svc.Stop("com.example.myapp")
	if err == nil {
		t.Fatal("Stop() should propagate exec error for a loaded job")
	}
}

func selfPlist(label string, keepAlive bool) plist.ScanResult {
	return plist.ScanResult{
		Path: "/Users/testuser/Library/LaunchAgents/" + label + ".plist",
		Data: plist.PlistData{
			Label:            label,
			ProgramArguments: []string{"/usr/local/bin/launch-pilot"},
			RunAtLoad:        true,
			KeepAlive:        keepAlive,
		},
	}
}

func TestSetSelfLabel_Resolution(t *testing.T) {
	t.Run("explicit wins", func(t *testing.T) {
		t.Setenv("XPC_SERVICE_NAME", "com.from.env")
		svc := NewService()
		svc.SetSelfLabel("com.explicit")
		if svc.SelfLabel() != "com.explicit" {
			t.Fatalf("SelfLabel = %q, want the flag", svc.SelfLabel())
		}
	})
	t.Run("env when the flag is empty", func(t *testing.T) {
		t.Setenv("XPC_SERVICE_NAME", "com.deployboard.launch-pilot")
		svc := NewService()
		svc.SetSelfLabel("")
		if svc.SelfLabel() != "com.deployboard.launch-pilot" {
			t.Fatalf("SelfLabel = %q, want XPC_SERVICE_NAME", svc.SelfLabel())
		}
		if !svc.IsSelf("com.deployboard.launch-pilot") || svc.IsSelf("com.example.other") {
			t.Fatalf("IsSelf did not follow the resolved label %q", svc.SelfLabel())
		}
	})
	t.Run("zero is launchd's unnamed placeholder", func(t *testing.T) {
		t.Setenv("XPC_SERVICE_NAME", "0")
		svc := NewService()
		svc.SetSelfLabel("")
		if svc.SelfLabel() != "" || svc.IsSelf("0") {
			t.Fatalf("SelfLabel = %q, a placeholder must not become the dashboard", svc.SelfLabel())
		}
	})
}

// Reloading our own label must not touch launchctl. bootout would SIGTERM
// this process before bootstrap, and the job would stay gone.
func TestReload_SelfPerformsNoLaunchctl(t *testing.T) {
	const label = "com.deployboard.launch-pilot"
	svc, calls := newActionTestService([]plist.ScanResult{selfPlist(label, true)}, true)
	svc.SetSelfLabel(label)

	err := svc.Reload(label)
	if !errors.Is(err, ErrSelfRestart) {
		t.Fatalf("Reload(self) = %v, want ErrSelfRestart", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("self reload must not call launchctl, got %v", *calls)
	}
}

// Another loaded label is kickstarted. Knowing our own label must not make
// Reload bootout that job — or ours.
func TestReload_OtherLoadedKickstartsWithoutBootout(t *testing.T) {
	const self = "com.deployboard.launch-pilot"
	const other = "com.example.myapp"
	plists := []plist.ScanResult{
		selfPlist(self, true),
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{Label: other, ProgramArguments: []string{"/usr/local/bin/web"}},
		},
	}
	svc, calls := newActionTestService(plists, true)
	svc.SetSelfLabel(self)

	if err := svc.Reload(other); err != nil {
		t.Fatalf("Reload(other) = %v", err)
	}
	if !hasCall(*calls, "kickstart", "-k", "gui/501/"+other) {
		t.Fatalf("expected kickstart -k, got %v", *calls)
	}
	if hasCall(*calls, "bootout", "gui/501/"+other) || hasCall(*calls, "bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist") {
		t.Fatalf("a loaded job must not bootout or bootstrap, got %v", *calls)
	}
	if hasCall(*calls, "bootout", "gui/501/"+self) || hasCall(*calls, "kickstart", "-k", "gui/501/"+self) {
		t.Fatalf("reloading another job must not touch self, got %v", *calls)
	}
}

// An unloaded other label still bootouts then bootstraps, and still does not
// touch the dashboard's own job.
func TestReload_OtherUnloadedStillBootoutsThenBootstraps(t *testing.T) {
	const self = "com.deployboard.launch-pilot"
	const other = "com.example.myapp"
	plists := []plist.ScanResult{
		selfPlist(self, true),
		{
			Path: "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist",
			Data: plist.PlistData{Label: other, ProgramArguments: []string{"/usr/local/bin/web"}},
		},
	}
	svc, calls := newActionTestService(plists, false)
	svc.SetSelfLabel(self)

	if err := svc.Reload(other); err != nil {
		t.Fatalf("Reload(other) = %v", err)
	}
	bootout := callIndex(*calls, "bootout", "gui/501/"+other)
	bootstrap := callIndex(*calls, "bootstrap", "gui/501", "/Users/testuser/Library/LaunchAgents/com.example.myapp.plist")
	if bootout < 0 || bootstrap < 0 || bootout > bootstrap {
		t.Fatalf("want bootout then bootstrap, got %v", *calls)
	}
	if hasCall(*calls, "bootout", "gui/501/"+self) {
		t.Fatalf("reloading another job must not bootout self, got %v", *calls)
	}
}

func TestListJobs_MarksTheSelfRow(t *testing.T) {
	const label = "com.deployboard.launch-pilot"
	svc, _ := newActionTestService([]plist.ScanResult{selfPlist(label, true)}, true)
	svc.SetSelfLabel(label)

	jobs, err := svc.ListJobs()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, job := range jobs {
		if job.Label == label {
			found = true
			if !job.Self {
				t.Fatal("the dashboard row must be marked self")
			}
		} else if job.Self {
			t.Fatalf("job %s marked self", job.Label)
		}
	}
	if !found {
		t.Fatal("self job missing from the list")
	}
}

func TestRestartSelf_KeepAliveExitsWithoutLaunchctl(t *testing.T) {
	const label = "com.deployboard.launch-pilot"
	svc, calls := newActionTestService([]plist.ScanResult{selfPlist(label, true)}, true)
	svc.SetSelfLabel(label)
	var code *int
	svc.exitFn = func(c int) {
		got := c
		code = &got
	}
	svc.startDetached = func(name string, args ...string) error {
		t.Errorf("KeepAlive restart must not spawn a helper, got %s %v", name, args)
		return nil
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	restartErr := svc.RestartSelf()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if restartErr != nil {
		t.Fatal(restartErr)
	}
	if code == nil || *code != 0 {
		t.Fatalf("exitFn = %v, want 0", code)
	}
	if len(*calls) != 0 {
		t.Fatalf("KeepAlive self-restart must not call launchctl, got %v", *calls)
	}
	if !strings.Contains(buf.String(), "deployboard: self-restart — exiting so launchd respawns the job") {
		t.Fatalf("stdout = %q, want the self-restart line", buf.String())
	}
}

func TestRestartSelf_WithoutKeepAliveSpawnsKickstart(t *testing.T) {
	const label = "com.deployboard.launch-pilot"
	svc, calls := newActionTestService([]plist.ScanResult{selfPlist(label, false)}, true)
	svc.SetSelfLabel(label)
	svc.exitFn = func(code int) {
		t.Errorf("exitFn(%d) called; a job without KeepAlive is restarted by kickstart", code)
	}
	var spawned []execCall
	svc.startDetached = func(name string, args ...string) error {
		spawned = append(spawned, execCall{name: name, args: args})
		return nil
	}

	if err := svc.RestartSelf(); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Fatalf("kickstart goes through the detached helper, not runExec, got %v", *calls)
	}
	if len(spawned) != 1 || spawned[0].name != "sh" || len(spawned[0].args) != 2 || spawned[0].args[0] != "-c" {
		t.Fatalf("spawn = %+v, want sh -c", spawned)
	}
	want := "sleep 0.5; exec launchctl kickstart -k gui/501/" + label
	if spawned[0].args[1] != want {
		t.Fatalf("script = %q, want %q", spawned[0].args[1], want)
	}
}

func TestRestartSelf_UnknownJob(t *testing.T) {
	svc, calls := newActionTestService(nil, false)
	svc.SetSelfLabel("com.deployboard.launch-pilot")
	svc.exitFn = func(int) { t.Error("must not exit when the job is unknown") }
	err := svc.RestartSelf()
	if err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("RestartSelf = %v, want ErrNotFound", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("unknown job must not call launchctl, got %v", *calls)
	}
}

func TestDetachedCommand_NewSession(t *testing.T) {
	cmd := detachedCommand("sh", "-c", "sleep 0.5; exec launchctl kickstart -k gui/501/com.deployboard.launch-pilot")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("the kickstart helper must be its own session, or it dies with us")
	}
}
