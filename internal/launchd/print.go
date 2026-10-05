package launchd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/A404coder/deployboard/internal/inventory"
)

// ErrNotLoaded reports that `launchctl print` cannot find the service in the
// target domain — the job is not loaded (distinct from "loaded but idle").
var ErrNotLoaded = errors.New("job not loaded")

// PrintInfo is the subset of `launchctl print gui/<uid>/<label>` this fork uses.
//
// `launchctl list` (upstream's data source) has neither a restart counter nor a
// disabled flag, so a job that has silently restarted over 12k times looks healthy
// there. `launchctl print` reports `runs` and a real `state`.
type PrintInfo struct {
	State        string // "running" | "not running" | "" (unknown)
	PID          int
	Runs         int  // restart counter since the job was loaded
	HasExit      bool // true when the "last exit code" line was present
	LastExitCode int
	PlistPath    string
}

// Running reports whether launchd considers the job running with a live pid.
func (p PrintInfo) Running() bool { return p.State == "running" && p.PID > 0 }

// printCacheEntry memoises one label's PrintInfo for a short TTL so a UI poll
// does not spawn one subprocess per label.
type printCacheEntry struct {
	at   time.Time
	info PrintInfo
	ok   bool
}

var (
	rePrintDisabled = regexp.MustCompile(`"([^"]+)"\s*=>\s*(disabled|enabled)`)
	rePrintPID      = regexp.MustCompile(`(?m)^\s*pid\s*=\s*(\d+)\s*$`)
	rePrintRuns     = regexp.MustCompile(`(?m)^\s*runs\s*=\s*(\d+)\s*$`)
	rePrintExit     = regexp.MustCompile(`(?m)^\s*last exit code\s*=\s*(-?\d+)\s*$`)
	rePrintPath     = regexp.MustCompile(`(?m)^\s*path\s*=\s*(.+?)\s*$`)
	rePrintState    = regexp.MustCompile(`(?m)^\s*state\s*=\s*([^\n]+?)\s*$`)
)

// ParsePrintOutput parses `launchctl print` output into a PrintInfo.
//
// It returns ErrNotLoaded when launchd reports the service is unknown, which is
// how a plist that exists on disk but is not bootstrapped is recognised.
func ParsePrintOutput(out string) (PrintInfo, error) {
	if strings.Contains(out, "Could not find service") ||
		strings.Contains(out, "Could not find specified service") {
		return PrintInfo{}, ErrNotLoaded
	}

	var info PrintInfo
	// `state = running` also appears nested under per-service sub-blocks; the
	// first top-level occurrence is the job's own state.
	if m := rePrintState.FindStringSubmatch(out); m != nil {
		info.State = strings.TrimSpace(m[1])
	}
	if m := rePrintPID.FindStringSubmatch(out); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			info.PID = n
		}
	}
	if m := rePrintRuns.FindStringSubmatch(out); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			info.Runs = n
		}
	}
	if m := rePrintExit.FindStringSubmatch(out); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			info.HasExit = true
			info.LastExitCode = n
		}
	}
	if m := rePrintPath.FindStringSubmatch(out); m != nil {
		info.PlistPath = strings.TrimSpace(m[1])
	}
	if info.State == "" && info.PID == 0 && info.Runs == 0 && !info.HasExit && info.PlistPath == "" {
		return PrintInfo{}, ErrNotLoaded
	}
	return info, nil
}

// ParseDisabledOutput parses `launchctl print-disabled gui/<uid>` into a map of
// label → true for every label explicitly marked `disabled`.
func ParseDisabledOutput(out string) map[string]bool {
	disabled := make(map[string]bool)
	for _, m := range rePrintDisabled.FindAllStringSubmatch(out, -1) {
		if m[2] == "disabled" {
			disabled[m[1]] = true
		}
	}
	return disabled
}

// defaultRunPrint runs `launchctl print <domain>/<label>`.
func (s *Service) defaultRunPrint(domain, label string) (string, string, error) {
	res, err := runCmd("launchctl", "print", domain+"/"+label)
	if res != nil {
		// launchctl writes "Could not find service ..." here for unknown jobs,
		// so pass both streams through even when the command failed.
		return res.Stdout, res.Stderr, err
	}
	return "", "", err
}

// defaultRunDisabled runs `launchctl print-disabled <domain>` once for all labels.
func (s *Service) defaultRunDisabled(domain string) (string, error) {
	res, err := runCmd("launchctl", "print-disabled", domain)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

// disabledLookup returns the disabled-label set, memoised for printTTL.
func (s *Service) disabledLookup() map[string]bool {
	if s.runDisabled == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disabledMap != nil && s.now().Sub(s.disabledAt) < s.printTTL {
		return s.disabledMap
	}
	out, err := s.runDisabled(s.domain)
	if err != nil {
		// Keep the previous map rather than pretending nothing is disabled.
		if s.disabledMap != nil {
			return s.disabledMap
		}
		return nil
	}
	s.disabledMap = ParseDisabledOutput(out)
	s.disabledAt = s.now()
	return s.disabledMap
}

// SetProcessLookup enables the uptime column. Nil leaves it empty.
func (s *Service) SetProcessLookup(lookup ProcessLookup) {
	s.processLookup = lookup
}

// SetRetirementLookup lets the enrichment report when the dashboard retired a
// job, which launchd itself does not remember.
func (s *Service) SetRetirementLookup(fn func(label string) (time.Time, bool)) {
	s.retirement = fn
}

// invalidateActionCaches drops the memoised launchd state after an action.
//
// Both caches (per-label `launchctl print`, per-domain `print-disabled`) are
// short-lived on purpose so a poll does not spawn a subprocess per job — but an
// action changes exactly the state they hold. Without this, the UI reports the
// pre-click world for up to printTTL: Start decides the job is "still loaded"
// and kickstarts a service that no longer exists, and a Disable keeps reading
// as `offline` instead of `disabled`.
func (s *Service) invalidateActionCaches(label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.printCache != nil {
		delete(s.printCache, label)
	}
	s.disabledMap = nil
	s.disabledAt = time.Time{}
	// A reload restarts the process, so the uptime for the old pid is stale and
	// the runs counter starts over.
	s.procs.invalidate(0)
	s.procs.mu.Lock()
	s.procs.entries = nil
	s.procs.mu.Unlock()
	s.history.forget(label)
}

// printInfoFor returns the memoised PrintInfo for one label.
func (s *Service) printInfoFor(label string) (PrintInfo, bool) {
	if s.runPrint == nil {
		return PrintInfo{}, false
	}
	s.mu.Lock()
	if s.printCache == nil {
		s.printCache = make(map[string]printCacheEntry)
	}
	if e, ok := s.printCache[label]; ok && s.now().Sub(e.at) < s.printTTL {
		s.mu.Unlock()
		return e.info, e.ok
	}
	s.mu.Unlock()

	stdout, stderr, _ := s.runPrint(s.domain, label)
	info, perr := ParsePrintOutput(stdout + stderr)
	ok := perr == nil

	s.mu.Lock()
	s.printCache[label] = printCacheEntry{at: s.now(), info: info, ok: ok}
	s.mu.Unlock()
	return info, ok
}

// enrich layers the fork's data onto upstream's job list: category, group,
// `runs` from launchctl print, the disabled flag, and the disabled status.
//
// Jobs in the noise bucket are never printed for (no subprocess storm on a
// 556-job machine); they still get a category so the UI can hide them.
func (s *Service) enrich(jobs []Job) {
	// Identity does not depend on classification. A dashboard with enrichment
	// turned off is still its own job, and Stop/Disable copy has to know that.
	for i := range jobs {
		jobs[i].Self = s.IsSelf(jobs[i].Label)
	}
	if s.classifier == nil {
		return
	}
	disabled := s.disabledLookup()
	threshold := s.classifier.RestartWarnThreshold()

	for i := range jobs {
		job := &jobs[i]
		cat, src := s.classifier.ClassifyWithSource(inventory.Signal{
			Label:       job.Label,
			Program:     job.Program,
			ProgramArgs: job.ProgramArgs,
			LogPath:     job.StandardOutPath,
			ErrLogPath:  job.StandardErrPath,
			WorkingDir:  job.WorkingDirectory,
			PlistPath:   job.PlistPath,
		})
		job.Category = string(cat)
		job.CategorySource = string(src)
		job.Group = s.classifier.GroupName(job.Label)

		if job.Category == string(inventory.CategoryNoise) {
			continue
		}
		if disabled[job.Label] {
			job.Disabled = true
		}
		if info, ok := s.printInfoFor(job.Label); ok {
			job.Runs = info.Runs
			job.PrintState = info.State
			job.HasExit = info.HasExit
			if info.HasExit {
				job.LastExitStatus = info.LastExitCode
			}
			if info.PID > 0 && job.PID == 0 {
				job.PID = info.PID
			}
			if info.PlistPath != "" && job.PlistPath == "" {
				job.PlistPath = info.PlistPath
			}
		}
		if job.Runs >= threshold {
			job.RestartWarn = true
		}
		// Uptime: one `ps` per running job, memoised. Noise jobs never get here.
		if job.PID > 0 {
			if info, ok := s.procs.get(job.PID, s.now(), s.printTTL, s.processLookup); ok {
				job.UptimeSeconds = int(info.Elapsed.Seconds())
			}
		}
		// Restart churn over the dashboard's own window, sampled once a minute.
		s.history.observe(job.Label, job.Runs, s.now())
		if restarts, minutes := s.history.window(job.Label); minutes > 0 {
			job.RestartsRecent = restarts
			job.WindowMinutes = minutes
			job.RunsSeries = s.history.series(job.Label)
		}
		if s.retirement != nil {
			if at, ok := s.retirement(job.Label); ok {
				at := at
				job.RetiredAt = &at
			}
		}
		applyDisabledStatus(job)
	}
}

// applyDisabledStatus implements the fork's status layering: a loaded job that
// is explicitly disabled and has no live pid is `disabled`, not `error` or
// `offline`. A disabled job that is somehow running keeps `running` (the
// Disabled flag carries the intent).
func applyDisabledStatus(job *Job) {
	if !job.Disabled || job.PID != 0 {
		return
	}
	job.Status = StatusDisabled
}

// Domain returns the launchd domain this service queries ("gui/<uid>").
func (s *Service) Domain() string { return s.domain }

// Classifier returns the configured classifier (nil when enrichment is off).
func (s *Service) Classifier() *inventory.Classifier { return s.classifier }

// SetPrintTTL overrides the print/disabled memo TTL (tests + flags).
func (s *Service) SetPrintTTL(d time.Duration) {
	if d <= 0 {
		d = DefaultPrintTTL
	}
	s.printTTL = d
}

// SetEnrichment enables the fork's enrichment with an explicit classifier and
// print-memo TTL, wiring the default launchctl runners when none are injected.
func (s *Service) SetEnrichment(c *inventory.Classifier, ttl time.Duration) {
	s.classifier = c
	s.SetPrintTTL(ttl)
	if s.runPrint == nil {
		s.runPrint = s.defaultRunPrint
	}
	if s.runDisabled == nil {
		s.runDisabled = s.defaultRunDisabled
	}
}

// DefaultPrintTTL is how long a `launchctl print` result is reused.
const DefaultPrintTTL = 15 * time.Second

// formatRuns renders a restart counter for the CLI table.
func formatRuns(n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}
