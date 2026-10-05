package alerts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Kind classifies why an alert fired.
type Kind string

const (
	KindError       Kind = "error"
	KindOffline     Kind = "offline"
	KindUnreachable Kind = "unreachable"
	KindRunStorm    Kind = "run_storm"
	KindRecovery    Kind = "recovery"
)

// JobState is one job's observed state for one evaluation pass.
type JobState struct {
	Label        string
	Category     string // "ours" | "other" | "noise"
	Group        string
	Status       string // launchd status incl. "disabled"
	Disabled     bool
	Runs         int
	HasExit      bool
	LastExitCode int
	ProbeOK      bool
	HasProbe     bool
	LogPath      string
}

// Entry is one recorded send (in-memory ring buffer, no payload storage).
type Entry struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	SentAt int64  `json:"sentAt"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Engine evaluates transitions, applies cooldown/quiet hours, sends Telegram
// messages and persists per-label state.
type Engine struct {
	cfg     Config
	state   *State
	sender  *Sender
	nowFn   func() time.Time
	enabled bool // false when the token is unavailable

	mu      sync.Mutex
	history []Entry
	warned  bool
}

// NewEngine loads state and resolves the Telegram token from the Keychain.
// A missing token disables sending (WarnOnce explains it) — never an error.
func NewEngine(cfg Config, state *State) *Engine {
	e := &Engine{cfg: cfg, state: state, nowFn: time.Now}
	e.Configure(cfg.Telegram)
	return e
}

// Configure (re)resolves the Telegram credentials. Called at startup, after the
// settings UI writes a token, and on a slow retry so a token stored out-of-band
// (with `security add-generic-password`) starts working without a restart.
//
// It never returns the token; a failure leaves alerts disabled.
func (e *Engine) Configure(conf TelegramConf) bool {
	e.mu.Lock()
	e.cfg.Telegram = conf
	e.mu.Unlock()

	if !e.cfg.Enabled || !conf.Enabled {
		e.mu.Lock()
		e.sender = nil
		e.enabled = false
		e.mu.Unlock()
		return false
	}
	timeout := time.Duration(conf.TimeoutSeconds) * time.Second
	token, err := ReadKeychainToken(conf.KeychainService, conf.KeychainAccount, timeout)
	if err != nil {
		e.mu.Lock()
		e.sender = nil
		e.enabled = false
		e.mu.Unlock()
		return false
	}
	e.mu.Lock()
	e.sender = NewSender(token, conf.ChatID, timeout)
	e.enabled = true
	e.mu.Unlock()
	return true
}

// TelegramConfig returns the current (non-secret) Telegram settings.
func (e *Engine) TelegramConfig() TelegramConf {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg.Telegram
}

// SetChatID updates the destination chat id in memory (settings UI).
func (e *Engine) SetChatID(chatID string) {
	e.mu.Lock()
	e.cfg.Telegram.ChatID = chatID
	if e.sender != nil {
		e.sender.ChatID = chatID
	}
	e.mu.Unlock()
}

// SetSender injects a sender (tests).
func (e *Engine) SetSender(s *Sender) {
	e.sender = s
	e.enabled = s != nil && s.Token != ""
}

// Enabled reports whether alerts can actually be delivered.
func (e *Engine) Enabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enabled
}

// WarnOnce returns a single WARN line to print when alerts are unavailable.
func (e *Engine) WarnOnce() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.warned || e.enabled {
		return ""
	}
	e.warned = true
	return "telegram alerts disabled: keychain item not found"
}

// History returns the most recent sends, newest first.
func (e *Engine) History(limit int) []Entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit <= 0 || limit > len(e.history) {
		limit = len(e.history)
	}
	out := make([]Entry, 0, limit)
	for i := len(e.history) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, e.history[i])
	}
	return out
}

// effectiveEnabled reports whether a label should alert right now.
//
// Defaults: on for `ours`, off for `other`/`noise`/`disabled`. An explicit
// toggle in the state file always wins.
func (e *Engine) effectiveEnabled(j JobState) bool {
	if j.Category == "noise" || j.Disabled || j.Status == "disabled" {
		return false
	}
	if ls, ok := e.state.Snapshot()[j.Label]; ok {
		if explicit, set := ls.ExplicitEnable(); set {
			return explicit
		}
	}
	if e.cfg.DefaultEnabled != nil {
		return *e.cfg.DefaultEnabled
	}
	return j.Category == "ours"
}

// Evaluate inspects one pass of job states and sends at most one alert per job.
// It always updates the persisted last-status/last-runs fingerprints.
func (e *Engine) Evaluate(ctx context.Context, jobs []JobState) []Entry {
	e.mu.Lock()
	now := e.nowFn()
	cfg := e.cfg
	cooldown := cfg.cooldown()
	quiet := inQuietHours(now, cfg)
	sender := e.sender
	enabled := e.enabled && e.sender != nil
	token := ""
	if sender != nil {
		token = sender.Token
	}
	e.mu.Unlock()

	sent := make([]Entry, 0)
	for _, j := range jobs {
		prev := e.state.Get(j.Label)

		if !e.effectiveEnabled(j) {
			// Keep the fingerprint fresh so re-enabling does not fire a stale alert.
			prev.LastStatus = j.Status
			prev.LastRuns = j.Runs
			if j.HasProbe {
				ok := j.ProbeOK
				prev.LastProbeOK = &ok
			}
			e.state.Set(j.Label, prev)
			continue
		}

		kind, text, fire := classify(cfg, j, prev, now, quiet)
		if fire {
			if kind != KindRecovery && prev.LastSentAt > 0 && now.Unix()-prev.LastSentAt < int64(cooldown.Seconds()) {
				fire = false
			}
		}

		if fire {
			status := 0
			var err error
			if enabled {
				status, err = sender.Send(ctx, text)
			} else {
				err = ErrNoToken
			}
			entry := Entry{Label: j.Label, Kind: string(kind), Status: j.Status, SentAt: now.Unix(), OK: err == nil}
			if err != nil {
				entry.Detail = Redact(token, err.Error())
			} else {
				entry.Detail = fmt.Sprintf("status=%d", status)
				prev.LastSentAt = now.Unix()
			}
			e.record(entry)
			sent = append(sent, entry)
		}

		prev.LastStatus = j.Status
		prev.LastRuns = j.Runs
		if j.HasProbe {
			ok := j.ProbeOK
			prev.LastProbeOK = &ok
		}
		e.state.Set(j.Label, prev)
	}
	return sent
}

// classify decides whether this pass is a transition worth sending.
func classify(cfg Config, j JobState, prev LabelState, now time.Time, quiet bool) (Kind, string, bool) {
	// Probe flips take precedence: a listening port that stops answering is the
	// symptom users actually notice.
	if j.HasProbe && prev.LastProbeOK != nil && *prev.LastProbeOK && !j.ProbeOK {
		if quiet {
			return KindUnreachable, "", false
		}
		return KindUnreachable, formatAlert(KindUnreachable, j, now), true
	}
	// Probe recovery: was unreachable, now reachable. Never suppressed by quiet hours/cooldown.
	if j.HasProbe && prev.LastProbeOK != nil && !*prev.LastProbeOK && j.ProbeOK {
		if cfg.NotifyRecovery && prev.LastStatus != "" {
			return KindRecovery, formatAlert(KindRecovery, j, now), true
		}
	}
	// Restart storm: runs jumped by the configured delta since the last pass.
	if cfg.RunStormDelta > 0 && prev.LastRuns > 0 && j.Runs-prev.LastRuns >= cfg.RunStormDelta {
		if quiet {
			return KindRunStorm, "", false
		}
		return KindRunStorm, formatAlert(KindRunStorm, j, now), true
	}
	if j.Status != prev.LastStatus {
		switch j.Status {
		case "error":
			if quiet {
				return KindError, "", false
			}
			return KindError, formatAlert(KindError, j, now), true
		case "offline":
			if quiet {
				return KindOffline, "", false
			}
			return KindOffline, formatAlert(KindOffline, j, now), true
		case "running":
			// Recovery is never suppressed by quiet hours or cooldown.
			if cfg.NotifyRecovery && prev.LastStatus != "" {
				return KindRecovery, formatAlert(KindRecovery, j, now), true
			}
		}
	}
	return "", "", false
}

// format renders the alert text. Plain text only (no Markdown/HTML) so a label
// or path can never break delivery.
func formatAlert(kind Kind, j JobState, now time.Time) string {
	icon := "🔴"
	switch kind {
	case KindRecovery:
		icon = "🟢"
	case KindRunStorm:
		icon = "🌀"
	case KindUnreachable:
		icon = "🔌"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", icon, strings.ToUpper(string(kind)))
	if j.Group != "" {
		fmt.Fprintf(&b, "%s · %s\n", j.Group, j.Label)
	} else {
		fmt.Fprintf(&b, "%s\n", j.Label)
	}
	fmt.Fprintf(&b, "status: %s", j.Status)
	if j.HasExit {
		fmt.Fprintf(&b, " · last exit %d", j.LastExitCode)
	}
	if j.Runs > 0 {
		fmt.Fprintf(&b, "\nrestarts: %d", j.Runs)
	}
	if j.HasProbe && !j.ProbeOK {
		fmt.Fprintf(&b, "\nport not answering")
	}
	if j.LogPath != "" {
		fmt.Fprintf(&b, "\nlog: %s", j.LogPath)
	}
	fmt.Fprintf(&b, "\n%s", now.Format("2006-01-02 15:04:05 -0700"))
	return b.String()
}

// record appends to the ring buffer (max 50 entries).
func (e *Engine) record(entry Entry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, entry)
	if len(e.history) > 50 {
		e.history = e.history[len(e.history)-50:]
	}
}

// inQuietHours reports whether now falls inside the configured window.
// A window that wraps midnight (23:30–08:00) is handled.
func inQuietHours(now time.Time, cfg Config) bool {
	start, end := cfg.Quiet.Start, cfg.Quiet.End
	if start == "" || end == "" {
		return false
	}
	loc := time.Local
	if cfg.Quiet.Timezone != "" {
		if l, err := time.LoadLocation(cfg.Quiet.Timezone); err == nil {
			loc = l
		}
	}
	t := now.In(loc)
	cur := t.Hour()*60 + t.Minute()
	sm, ok1 := parseHHMM(start)
	em, ok2 := parseHHMM(end)
	if !ok1 || !ok2 || sm == em {
		return false
	}
	if sm < em {
		return cur >= sm && cur < em
	}
	return cur >= sm || cur < em
}

func parseHHMM(s string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// SendTestAlert sends one message for a specific job, bypassing cooldown and
// quiet hours (it is an explicit user action). Errors are redacted of the token.
func (e *Engine) SendTestAlert(ctx context.Context, j JobState) (int, error) {
	e.mu.Lock()
	sender := e.sender
	e.mu.Unlock()
	if sender == nil {
		return 0, ErrNoToken
	}
	text := formatAlert(KindRecovery, j, e.nowFn())
	status, err := sender.Send(ctx, "🧪 test alert\n"+text)
	if err != nil {
		err = errors.New(Redact(sender.Token, err.Error()))
	}
	e.record(Entry{Label: j.Label, Kind: "test", Status: j.Status, SentAt: e.nowFn().Unix(),
		OK: err == nil, Detail: fmt.Sprintf("status=%d", status)})
	return status, err
}

// SendTest sends a single ping with the configured credentials, used by the
// settings UI's "Send test" button. Errors are redacted of the token.
func (e *Engine) SendTest(ctx context.Context) (int, error) {
	e.mu.Lock()
	sender := e.sender
	e.mu.Unlock()
	if sender == nil {
		return 0, ErrNoToken
	}
	status, err := sender.Send(ctx, "🟢 Deployboard test alert — Telegram wiring works.")
	if err != nil {
		err = errors.New(Redact(sender.Token, err.Error()))
	}
	e.record(Entry{Label: "(settings)", Kind: "test", SentAt: e.nowFn().Unix(), OK: err == nil,
		Detail: fmt.Sprintf("status=%d", status)})
	return status, err
}

// enabledSet reports whether the state file holds an explicit user toggle.
func (ls LabelState) EnabledSet() bool { return ls.Explicit != nil }

// ExplicitEnable returns the user's explicit choice, when one exists.
func (ls LabelState) ExplicitEnable() (bool, bool) {
	if ls.Explicit == nil {
		return false, false
	}
	return *ls.Explicit, true
}
