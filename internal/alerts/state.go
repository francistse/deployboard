// Package alerts turns launchd state transitions into Telegram notifications,
// with a per-application toggle so you can silence one app without losing
// alerting everywhere else.
//
// Design rules that matter:
//   - the bot token is read from the macOS Keychain at send time and never
//     passed through a file, an env var, a log line or an error message;
//   - a missing Keychain item degrades to "alerts disabled" with one WARN,
//     never a crash or a retry storm;
//   - alerts fire on transitions only (with cooldown + quiet hours), so a
//     flapping job cannot spam the chat.
package alerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Config is the alert configuration (config.json → "alerts").
type Config struct {
	Enabled         bool          `json:"enabled"`
	Cooldown        time.Duration `json:"-"`
	CooldownSeconds int           `json:"cooldown_seconds"`
	NotifyRecovery  bool          `json:"notify_on_recovery"`
	RunStormDelta   int           `json:"run_storm_delta"`
	StateFile       string        `json:"state_file"`
	DefaultEnabled  *bool         `json:"default_enabled,omitempty"`
	Quiet           QuietHours    `json:"quiet_hours"`
	Telegram        TelegramConf  `json:"telegram"`
}

// QuietHours suppresses new alerts inside a daily window (never recoveries).
type QuietHours struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// TelegramConf holds the non-secret Telegram settings. The token is NOT here.
type TelegramConf struct {
	Enabled         bool   `json:"enabled"`
	ChatID          string `json:"chat_id"`
	KeychainService string `json:"keychain_service"`
	KeychainAccount string `json:"keychain_account"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

// DefaultConfig mirrors the shipped config.json.
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		CooldownSeconds: 900,
		NotifyRecovery:  true,
		RunStormDelta:   25,
		StateFile:       defaultStateFile(),
		Quiet:           QuietHours{Start: "23:30", End: "08:00", Timezone: "Asia/Hong_Kong"},
		Telegram: TelegramConf{
			Enabled:         true,
			KeychainService: "deployboard-telegram",
			KeychainAccount: "bot_token",
			TimeoutSeconds:  10,
		},
	}
}

func defaultStateFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "alerts.json"
	}
	return filepath.Join(home, ".config", "deployboard", "alerts.json")
}

// legacyStateFile is the previous product path, read for one release when the
// new file is not there yet.
func legacyStateFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "launch-pilot", "alerts.json")
}

var legacyAlertsLogOnce sync.Once

// resolveStatePath uses the previous alerts file when the new default is absent
// so a start mid-migration does not reset per-app toggles. The chosen legacy
// path is logged once. A custom state_file is never redirected.
func resolveStatePath(path string) string {
	if path == "" || path != defaultStateFile() {
		return path
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	legacy := legacyStateFile()
	if legacy == "" {
		return path
	}
	info, err := os.Stat(legacy)
	if err != nil || info.IsDir() {
		return path
	}
	legacyAlertsLogOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "deployboard: alerts state: using %s\n", legacy)
	})
	return legacy
}

// cooldown returns the effective cooldown duration.
func (c Config) cooldown() time.Duration {
	switch {
	case c.Cooldown > 0:
		return c.Cooldown
	case c.CooldownSeconds > 0:
		return time.Duration(c.CooldownSeconds) * time.Second
	default:
		return 900 * time.Second
	}
}

// LabelState is one application's alert state, persisted across restarts.
type LabelState struct {
	Enabled    bool   `json:"enabled"`
	MutedUntil *int64 `json:"mutedUntil"`
	LastStatus string `json:"lastStatus,omitempty"`
	LastRuns   int    `json:"lastRuns,omitempty"`
	LastSentAt int64  `json:"lastSentAt,omitempty"`
	// LastProbeOK is nil until a probe has been observed for this label.
	LastProbeOK *bool `json:"lastProbeOK,omitempty"`
	// Explicit is set only by a user toggle, so a recorded status never
	// masquerades as a decision (see Engine.effectiveEnabled).
	Explicit *bool `json:"explicit,omitempty"`
}

// State is the persisted alert state file.
type State struct {
	Version   int                   `json:"version"`
	Labels    map[string]LabelState `json:"labels"`
	UpdatedAt int64                 `json:"updatedAt"`

	path string
	mu   sync.Mutex
}

// NewState returns an empty in-memory state bound to path.
func NewState(path string) *State {
	return &State{Version: 1, Labels: map[string]LabelState{}, path: path}
}

// LoadState reads the state file; a missing file is not an error.
func LoadState(path string) (*State, error) {
	path = resolveStatePath(path)
	s := NewState(path)
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return s, fmt.Errorf("alerts state %s: %w", path, err)
	}
	if s.Labels == nil {
		s.Labels = map[string]LabelState{}
	}
	s.path = path
	return s, nil
}

// Save writes the state atomically (temp file + rename).
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now().Unix()
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get returns the state for a label (zero value when unknown).
func (s *State) Get(label string) LabelState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Labels[label]
}

// Set stores a label's state.
func (s *State) Set(label string, ls LabelState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Labels == nil {
		s.Labels = map[string]LabelState{}
	}
	s.Labels[label] = ls
}

// SetEnabled toggles alerting for one label, returning the new state. It marks
// the choice explicit so later evaluations treat it as a decision, not a default.
func (s *State) SetEnabled(label string, enabled bool) LabelState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Labels == nil {
		s.Labels = map[string]LabelState{}
	}
	ls := s.Labels[label]
	on := enabled
	ls.Enabled = enabled
	ls.Explicit = &on
	ls.MutedUntil = nil
	s.Labels[label] = ls
	return ls
}

// Snapshot returns a copy of all label states (safe for JSON encoding).
func (s *State) Snapshot() map[string]LabelState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]LabelState, len(s.Labels))
	for k, v := range s.Labels {
		out[k] = v
	}
	return out
}

// Labels returns every label present in the state file, sorted.
func (s *State) Labels_() []string {
	snap := s.Snapshot()
	out := make([]string, 0, len(snap))
	for k := range snap {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Muted reports whether a label is currently muted.
func (ls LabelState) Muted(now time.Time) bool {
	return ls.MutedUntil != nil && *ls.MutedUntil > now.Unix()
}
