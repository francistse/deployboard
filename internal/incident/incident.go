// Package incident stores an append-only JSONL timeline of Ours events (Phase 2).
package incident

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Kind classifies a timeline event.
type Kind string

const (
	KindStatus      Kind = "status"
	KindProbe       Kind = "probe"
	KindContract    Kind = "contract"
	KindAction      Kind = "action"
	KindDrift       Kind = "drift"
	KindFingerprint Kind = "fingerprint"
)

// Event is one JSONL line.
type Event struct {
	At      time.Time `json:"at"`
	Kind    Kind      `json:"kind"`
	Label   string    `json:"label,omitempty"`
	Group   string    `json:"group,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Hash    string    `json:"hash,omitempty"`
	Count   int       `json:"count,omitempty"`
}

// Store appends events to ~/.config/deployboard/incidents.jsonl.
type Store struct {
	path string
	mu   sync.Mutex
}

// DefaultPath is the default JSONL location.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "incidents.jsonl"
	}
	return filepath.Join(home, ".config", "deployboard", "incidents.jsonl")
}

// Open returns a store bound to path (empty → default).
func Open(path string) *Store {
	if path == "" {
		path = DefaultPath()
	}
	return &Store{path: path}
}

// Path returns the on-disk path.
func (s *Store) Path() string { return s.path }

// Append writes one event (creates parent dirs). Caps file at ~5 MiB by truncating oldest half.
func (s *Store) Append(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	line, err := json.Marshal(ev)
	if err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.maybeRotateLocked()
}

const maxBytes = 5 * 1024 * 1024

func (s *Store) maybeRotateLocked() error {
	info, err := os.Stat(s.path)
	if err != nil || info.Size() < maxBytes {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	half := len(data) / 2
	for half < len(data) && data[half] != '\n' {
		half++
	}
	if half < len(data) {
		half++
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data[half:], 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Since returns events at or after since (UTC), newest last.
func (s *Store) Since(since time.Time) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	out := make([]Event, 0)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.At.Before(since) {
			continue
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

// Fingerprint hashes exit code + trailing stderr for crash grouping.
func Fingerprint(exitCode int, stderrTail string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", exitCode)
	lines := strings.Split(stderrTail, "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	for _, line := range lines {
		fmt.Fprintln(h, strings.TrimSpace(line))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// GroupFingerprints collapses identical fingerprint events in a window.
type FingerprintGroup struct {
	Hash  string    `json:"hash"`
	Label string    `json:"label"`
	Count int       `json:"count"`
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
	Detail string   `json:"detail,omitempty"`
}

// GroupByFingerprint aggregates KindFingerprint events.
func GroupByFingerprint(events []Event) []FingerprintGroup {
	type agg struct {
		g FingerprintGroup
	}
	m := map[string]*agg{}
	order := make([]string, 0)
	for _, ev := range events {
		if ev.Kind != KindFingerprint || ev.Hash == "" {
			continue
		}
		key := ev.Label + "|" + ev.Hash
		a, ok := m[key]
		if !ok {
			a = &agg{g: FingerprintGroup{
				Hash: ev.Hash, Label: ev.Label, Count: 0,
				First: ev.At, Last: ev.At, Detail: ev.Detail,
			}}
			m[key] = a
			order = append(order, key)
		}
		a.g.Count++
		if ev.At.Before(a.g.First) {
			a.g.First = ev.At
		}
		if ev.At.After(a.g.Last) {
			a.g.Last = ev.At
		}
	}
	out := make([]FingerprintGroup, 0, len(order))
	for _, k := range order {
		out = append(out, m[k].g)
	}
	return out
}
