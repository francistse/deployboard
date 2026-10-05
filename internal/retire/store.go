// Package retire keeps a small local log of jobs the user retired (or brought
// back) from the dashboard.
//
// launchd only stores the override itself ("disabled"); it has no memory of who
// did it or when. So a job you retired three weeks ago is indistinguishable from
// one that was disabled before the machine was set up. This file supplies the
// missing half, and it is deliberately a plain JSON file next to alerts.json:
// inspectable, greppable, deletable.
package retire

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one retirement decision.
type Entry struct {
	Action string    `json:"action"` // "disable" | "enable"
	At     time.Time `json:"at"`
	Source string    `json:"source"` // "dashboard" | "cli"
}

// Log is a file-backed history keyed by launchd label, newest decision wins.
type Log struct {
	path string

	mu      sync.Mutex
	entries map[string]Entry
}

// DefaultPath is where the log lives when no path is configured.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "retirements.json"
	}
	return filepath.Join(home, ".config", "deployboard", "retirements.json")
}

// legacyPath is the previous product path, read for one release when the new
// file is not there yet.
func legacyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "launch-pilot", "retirements.json")
}

var legacyRetireLogOnce sync.Once

// resolvePath uses the previous retirement log when the new default is absent
// so a start mid-migration does not drop history. The chosen legacy path is
// logged once. An explicit path is never redirected.
func resolvePath(path string) string {
	if path == "" || path != DefaultPath() {
		return path
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	legacy := legacyPath()
	if legacy == "" {
		return path
	}
	info, err := os.Stat(legacy)
	if err != nil || info.IsDir() {
		return path
	}
	legacyRetireLogOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "deployboard: retirement log: using %s\n", legacy)
	})
	return legacy
}

// Open loads the log, tolerating a missing or unreadable file: a dashboard must
// still start when its bookkeeping is gone.
func Open(path string) *Log {
	if path == "" {
		path = DefaultPath()
	}
	path = resolvePath(path)
	l := &Log{path: path, entries: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return l
	}
	var stored map[string]Entry
	if err := json.Unmarshal(data, &stored); err != nil {
		return l
	}
	l.entries = stored
	return l
}

// Path returns the file this log writes to.
func (l *Log) Path() string { return l.path }

// Record stores a decision. Failures are returned so a caller can decide whether
// to care — the launchd action itself has already happened either way.
func (l *Log) Record(label, action string, at time.Time) error {
	if label == "" || action == "" {
		return fmt.Errorf("retire: label and action are required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[label] = Entry{Action: action, At: at.UTC(), Source: "dashboard"}
	return l.save()
}

// Get returns the newest decision for a label.
func (l *Log) Get(label string) (Entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[label]
	return e, ok
}

// DisabledSince returns when a label was retired, and whether that is still the
// latest decision. A job that was re-enabled has no answer.
func (l *Log) DisabledSince(label string) (time.Time, bool) {
	e, ok := l.Get(label)
	if !ok || e.Action != "disable" {
		return time.Time{}, false
	}
	return e.At, true
}

// Snapshot returns a copy of every entry, for diagnostics and tests.
func (l *Log) Snapshot() map[string]Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Entry, len(l.entries))
	for k, v := range l.entries {
		out[k] = v
	}
	return out
}

// save writes atomically (temp + rename) so a reader never sees a half file.
func (l *Log) save() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, l.path)
}
