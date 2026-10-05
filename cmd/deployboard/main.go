package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/A404coder/deployboard/internal/alerts"
	"github.com/A404coder/deployboard/internal/diagnose"
	"github.com/A404coder/deployboard/internal/inventory"
	"github.com/A404coder/deployboard/internal/launchd"
	"github.com/A404coder/deployboard/internal/metrics"
	"github.com/A404coder/deployboard/internal/probe"
	"github.com/A404coder/deployboard/internal/retire"
	"github.com/A404coder/deployboard/internal/server"
	"github.com/A404coder/deployboard/web"
)

// Version is set by goreleaser at build time.
var Version = "dev"

// FileConfig is the optional config.json shape. Missing fields fall back to the
// built-in defaults, so a partial file can never silently disable a feature.
type FileConfig struct {
	Port      int              `json:"port"`
	ReadOnly  bool             `json:"read_only"`
	ProbeTTL  int              `json:"probe_ttl_seconds"`
	NoProbe   bool             `json:"no_probe"`
	PrintTTL  int              `json:"print_ttl_seconds"`
	Inventory inventory.Config `json:"inventory"`
	Alerts    alerts.Config    `json:"alerts"`
}

// Config holds the effective CLI + file configuration.
type Config struct {
	Port     int
	NoOpen   bool
	ReadOnly bool
	// ReadOnlyFlag records that --read-only was passed. That flag is a hard
	// lock the UI may not undo; read_only in config.json is a soft default the
	// settings panel may flip.
	ReadOnlyFlag bool
	NoProbe      bool
	RecentWindow time.Duration
	PrintTTL     time.Duration
	ProbeTTL     time.Duration
	ConfigPath   string
	// SelfLabel is this process's launchd label. Empty means derive it from
	// XPC_SERVICE_NAME, which launchd sets for a LaunchAgent. A plist is the
	// wrong place for it: the same binary is also `go run` from a terminal.
	SelfLabel string
	Inventory inventory.Config
	Alerts    alerts.Config
}

// Addr returns the listen address string. The dashboard is localhost-only by
// design: it has no auth and reports the machine's internals.
func (c Config) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", c.Port)
}

// Bounds for --recent-window per spec (1m to 24h).
const (
	minRecentWindow = time.Minute
	maxRecentWindow = 24 * time.Hour
)

// loadFileConfig reads config.json; a missing file yields zero values (defaults).
func loadFileConfig(path string) (FileConfig, error) {
	var fc FileConfig
	if path == "" {
		return fc, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fc, nil
		}
		return fc, err
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return fc, fmt.Errorf("config %s: %w", path, err)
	}
	return fc, nil
}

// parseFlags parses command-line arguments into a Config and validates them.
// Returns a non-nil versionRequested when --version was passed.
func parseFlags(args []string) (cfg Config, versionRequested bool, err error) {
	fs := flag.NewFlagSet("deployboard", flag.ContinueOnError)
	fs.IntVar(&cfg.Port, "port", 0, "listen port (0 = random available port)")
	fs.BoolVar(&cfg.NoOpen, "no-open", false, "skip auto-opening browser")
	fs.DurationVar(&cfg.RecentWindow, "recent-window", launchd.DefaultRecentWindow,
		"how long a just-finished job is shown as 'completed' (1m–24h)")
	fs.StringVar(&cfg.ConfigPath, "config", "config.json", "path to config.json (missing = built-in defaults)")
	fs.BoolVar(&cfg.ReadOnlyFlag, "read-only", false, "refuse reload/start/stop and lock write mode (monitoring mode)")
	fs.BoolVar(&cfg.NoProbe, "no-probe", false, "skip HTTP port probes")
	fs.DurationVar(&cfg.ProbeTTL, "probe-ttl", 10*time.Second, "how long a port probe result is reused")
	fs.DurationVar(&cfg.PrintTTL, "print-ttl", launchd.DefaultPrintTTL,
		"how long a `launchctl print` result is reused")
	fs.StringVar(&cfg.SelfLabel, "self-label", "", "launchd label of this process (empty = $XPC_SERVICE_NAME)")
	telegramTest := fs.Bool("telegram-test", false, "send one Telegram test message and exit")
	telegramStatus := fs.Bool("telegram-status", false, "report whether the Telegram token is readable and exit")
	versionFlag := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	fileCfg, err := loadFileConfig(cfg.ConfigPath)
	if err != nil {
		return cfg, false, err
	}
	cfg.Inventory = fileCfg.Inventory
	cfg.Alerts = fileCfg.Alerts
	if fileCfg.Port > 0 && cfg.Port == 0 {
		cfg.Port = fileCfg.Port
	}
	cfg.ReadOnly = cfg.ReadOnlyFlag || fileCfg.ReadOnly
	cfg.NoProbe = cfg.NoProbe || fileCfg.NoProbe

	if *versionFlag {
		return cfg, true, nil
	}
	if *telegramStatus {
		checkTelegramStatus(cfg.Alerts)
		return cfg, true, nil
	}
	if *telegramTest {
		runTelegramTest(cfg.Alerts)
		return cfg, true, nil
	}
	if cfg.RecentWindow < minRecentWindow || cfg.RecentWindow > maxRecentWindow {
		return cfg, false, fmt.Errorf("--recent-window must be between 1m and 24h, got %v", cfg.RecentWindow)
	}
	return cfg, false, nil
}

// alertConfig normalises the alert config with defaults for a partial file.
func alertConfig(c alerts.Config) alerts.Config {
	def := alerts.DefaultConfig()
	if c.StateFile == "" {
		c.StateFile = def.StateFile
	}
	if c.CooldownSeconds == 0 {
		c.CooldownSeconds = def.CooldownSeconds
	}
	if c.RunStormDelta == 0 {
		c.RunStormDelta = def.RunStormDelta
	}
	if c.Quiet.Start == "" {
		c.Quiet = def.Quiet
	}
	if c.Telegram.KeychainService == "" {
		c.Telegram.KeychainService = def.Telegram.KeychainService
	}
	if c.Telegram.KeychainAccount == "" {
		c.Telegram.KeychainAccount = def.Telegram.KeychainAccount
	}
	if c.Telegram.TimeoutSeconds == 0 {
		c.Telegram.TimeoutSeconds = def.Telegram.TimeoutSeconds
	}
	if c.Telegram.ChatID == "" {
		c.Telegram.ChatID = def.Telegram.ChatID
	}
	// A config file that never mentions alerts must not disable them.
	if !c.Enabled && c.StateFile == def.StateFile && c.Telegram.ChatID == "" {
		c.Enabled = def.Enabled
		c.Telegram.Enabled = def.Telegram.Enabled
	}
	return c
}

// checkTelegramStatus prints whether the token is readable, never its value.
func checkTelegramStatus(c alerts.Config) {
	c = alertConfig(c)
	ok := alerts.KeychainTokenReadable(c.Telegram.KeychainService, c.Telegram.KeychainAccount,
		time.Duration(c.Telegram.TimeoutSeconds)*time.Second)
	fmt.Printf("keychain service=%s account=%s readable=%v\n",
		c.Telegram.KeychainService, c.Telegram.KeychainAccount, ok)
	if !ok {
		fmt.Println("store it with: security add-generic-password -U -s " +
			c.Telegram.KeychainService + " -a " + c.Telegram.KeychainAccount +
			` -w "$TELEGRAM_BOT_TOKEN" -T /usr/bin/security`)
	}
}

// runTelegramTest sends one test message and reports the HTTP status.
func runTelegramTest(c alerts.Config) {
	c = alertConfig(c)
	token, err := alerts.ReadKeychainToken(c.Telegram.KeychainService, c.Telegram.KeychainAccount,
		time.Duration(c.Telegram.TimeoutSeconds)*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "deployboard:", err)
		os.Exit(1)
	}
	sender := alerts.NewSender(token, c.Telegram.ChatID, time.Duration(c.Telegram.TimeoutSeconds)*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	status, err := sender.Send(ctx, "🟢 Deployboard test alert — Telegram wiring works.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "deployboard:", alerts.Redact(token, err.Error()))
		os.Exit(1)
	}
	fmt.Printf("telegram test sent (status=%d)\n", status)
}

// configStore owns config.json: hot reload (mtime watch) and the write-back the
// UI's classify button needs. All access is serialised so a reload can never
// race a save.
type configStore struct {
	path string
	mu   sync.Mutex
	cfg  inventory.Config
	svc  *launchd.Service
	ttl  time.Duration
	// engine receives Telegram setting changes picked up from the file, so a
	// chat_id edited by hand takes effect without a restart.
	engine *alerts.Engine
	// access receives read_only edits found in the file (the watcher path).
	access *accessController
}

// adoptAccess forwards a read_only value read from disk to the access
// controller, so editing config.json by hand and flipping the UI toggler end up
// in exactly the same place.
func (s *configStore) adoptAccess(readOnly bool) {
	if s.access != nil {
		s.access.adoptFileValue(readOnly)
	}
}

func newConfigStore(path string, cfg inventory.Config, svc *launchd.Service, ttl time.Duration, engine *alerts.Engine) *configStore {
	return &configStore{path: path, cfg: cfg, svc: svc, ttl: ttl, engine: engine}
}

// Current returns the in-memory inventory config.
func (s *configStore) Current() inventory.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// apply (re)builds the classifier and hands it to the launchd service.
func (s *configStore) apply(cfg inventory.Config) {
	s.cfg = cfg
	s.svc.SetEnrichment(inventory.New(cfg), s.ttl)
}

// Reload re-reads config.json from disk and applies it.
func (s *configStore) Reload() error {
	cfg, err := inventory.LoadConfigFile(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apply(cfg)
	return nil
}

// Classify pins (or unpins) a label's category in config.json, then applies it.
//
// category: "ours" | "noise" | "auto" (auto removes the manual pin so the
// derived path rule decides again).
func (s *configStore) Classify(label, category string) error {
	if label == "" {
		return fmt.Errorf("label is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	switch category {
	case string(inventory.CategoryOurs):
		s.cfg = s.cfg.WithPattern(inventory.CategoryOurs, label)
		s.cfg = s.cfg.WithoutPatternFrom(inventory.CategoryNoise, label)
	case string(inventory.CategoryNoise):
		s.cfg = s.cfg.WithPattern(inventory.CategoryNoise, label)
		s.cfg = s.cfg.WithoutPatternFrom(inventory.CategoryOurs, label)
	case "auto":
		s.cfg = s.cfg.WithoutPatternFrom(inventory.CategoryOurs, label)
		s.cfg = s.cfg.WithoutPatternFrom(inventory.CategoryNoise, label)
	default:
		return fmt.Errorf("category must be ours|noise|auto, got %q", category)
	}

	if err := inventory.SaveConfigFile(s.path, s.cfg); err != nil {
		return err
	}
	s.apply(s.cfg)
	return nil
}

// watch polls config.json and applies external edits within ~2s, so editing the
// file by hand no longer needs a restart.
func (s *configStore) watch(ctx context.Context, every time.Duration) {
	if s.path == "" {
		return
	}
	stamp := inventory.Stamp(s.path)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next := inventory.Stamp(s.path)
			if next == stamp {
				continue
			}
			stamp = next
			if err := s.Reload(); err != nil {
				fmt.Fprintln(os.Stderr, "deployboard: config reload:", err)
				continue
			}
			if fileCfg, err := loadFileConfig(s.path); err == nil {
				s.pushTelegram(alertConfig(fileCfg.Alerts).Telegram)
				s.adoptAccess(fileCfg.ReadOnly)
			}
			fmt.Fprintf(os.Stderr, "deployboard: config.json reloaded (%s)\n", s.path)
		}
	}
}

// telegramSettings adapts the alert engine + Keychain to the settings UI.
// It is the only place a token crosses an HTTP boundary, and it only ever goes
// IN (browser → loopback → Keychain); nothing here returns it.
type telegramSettings struct {
	app   *appState
	store *configStore
}

// Status reports whether a token exists and is readable — never the token.
func (t telegramSettings) Status() (map[string]any, error) {
	conf := t.app.alerts.TelegramConfig()
	timeout := time.Duration(conf.TimeoutSeconds) * time.Second
	readable := alerts.KeychainTokenReadable(conf.KeychainService, conf.KeychainAccount, timeout)
	return map[string]any{
		"configured":       readable,
		"readable":         readable,
		"chat_id":          conf.ChatID,
		"keychain_service": conf.KeychainService,
		"keychain_account": conf.KeychainAccount,
		"alerts_enabled":   t.app.alerts.Enabled(),
		"store_hint": "security add-generic-password -U -s " + conf.KeychainService +
			" -a " + conf.KeychainAccount + ` -w "$TELEGRAM_BOT_TOKEN" -T /usr/bin/security`,
	}, nil
}

// Save stores a new token into the macOS Keychain and/or updates the chat id.
// The token is written straight to the Keychain and dropped from memory here.
func (t telegramSettings) Save(botToken, chatID string) (map[string]any, error) {
	conf := t.app.alerts.TelegramConfig()
	timeout := time.Duration(conf.TimeoutSeconds) * time.Second

	if botToken != "" {
		if err := alerts.StoreKeychainToken(conf.KeychainService, conf.KeychainAccount, botToken, timeout); err != nil {
			return nil, err // already redacted by the keychain layer
		}
	}
	if chatID != "" {
		conf.ChatID = chatID
		if err := t.store.SetTelegramChatID(chatID); err != nil {
			return nil, err
		}
		t.app.alerts.SetChatID(chatID)
	}
	// Re-resolve the sender so alerts start working without a restart.
	t.app.alerts.Configure(conf)
	return t.Status()
}

// Forget removes the Keychain item and disables sending.
func (t telegramSettings) Forget() (map[string]any, error) {
	conf := t.app.alerts.TelegramConfig()
	timeout := time.Duration(conf.TimeoutSeconds) * time.Second
	if err := alerts.DeleteKeychainToken(conf.KeychainService, conf.KeychainAccount, timeout); err != nil {
		return nil, err
	}
	t.app.alerts.Configure(conf) // now fails to resolve → alerts disabled
	return t.Status()
}

// Test sends one message with the stored credentials.
func (t telegramSettings) Test(ctx context.Context) (int, error) {
	return t.app.alerts.SendTest(ctx)
}

// SetTelegramChatID persists alerts.telegram.chat_id into config.json while
// leaving every other key alone.
func (s *configStore) SetTelegramChatID(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw := map[string]any{}
	if data, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("config %s: %w", s.path, err)
		}
	}
	alertsSec, _ := raw["alerts"].(map[string]any)
	if alertsSec == nil {
		alertsSec = map[string]any{}
	}
	tg, _ := alertsSec["telegram"].(map[string]any)
	if tg == nil {
		tg = map[string]any{}
	}
	tg["chat_id"] = chatID
	alertsSec["telegram"] = tg
	raw["alerts"] = alertsSec

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// pushTelegram hands the reloaded Telegram settings to the engine.
func (s *configStore) pushTelegram(conf alerts.TelegramConf) {
	if s.engine != nil {
		s.engine.Configure(conf)
	}
}

// telegramConf returns the Telegram settings as currently on disk, falling back
// to the given value when the file cannot be read.
func (s *configStore) telegramConf(fallback alerts.TelegramConf) alerts.TelegramConf {
	fileCfg, err := loadFileConfig(s.path)
	if err != nil {
		return fallback
	}
	return alertConfig(fileCfg.Alerts).Telegram
}

// printBanner writes the startup banner announcing the running URL.
func printBanner(w io.Writer, url string) {
	fmt.Fprintf(w, "Deployboard running at %s\n", url)
}

// accessController owns the read-only switch at runtime.
//
// Two levels, deliberately:
//   - --read-only on the command line is a HARD lock. It exists for a box that
//     must not be able to kill a service, so the UI cannot undo it.
//   - read_only in config.json is a SOFT default the dashboard's own settings
//     panel may flip. The write goes through config.json + the hot-reload path,
//     so the choice survives a restart and a hand edit works identically.
type accessController struct {
	mu       sync.Mutex
	readOnly bool
	locked   bool
	store    *configStore
}

func newAccessController(readOnly bool, locked bool, store *configStore) *accessController {
	return &accessController{readOnly: readOnly, locked: locked, store: store}
}

func (a *accessController) ReadOnly() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.readOnly
}

func (a *accessController) Locked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.locked
}

// LockReason explains a lock in words the settings panel can show verbatim.
func (a *accessController) LockReason() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.locked {
		return ""
	}
	return "the server was started with --read-only; remove that flag (and restart) to allow start/stop/reload"
}

// Source names where the current value came from, for the settings panel.
func (a *accessController) Source() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.locked {
		return "flag"
	}
	if a.readOnly {
		return "config"
	}
	if a.store != nil && a.store.path != "" {
		return "config"
	}
	return "default"
}

// SetReadOnly persists the new mode to config.json and applies it immediately.
func (a *accessController) SetReadOnly(readOnly bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.locked {
		return server.ErrAccessLocked
	}
	if a.store != nil && a.store.path != "" {
		if err := inventory.SetReadOnly(a.store.path, readOnly); err != nil {
			return err
		}
	}
	a.readOnly = readOnly
	return nil
}

// adoptFileValue applies a read_only value that changed on disk (the watcher
// path), unless the process is hard-locked by --read-only.
func (a *accessController) adoptFileValue(readOnly bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.locked {
		return
	}
	a.readOnly = readOnly
}

// appState is the fork's runtime: launchd service, prober, alert engine and the
// adapters the HTTP layer consumes.
type appState struct {
	svc     *launchd.Service
	prober  *probe.Prober
	alerts  *alerts.Engine
	state   *alerts.State
	store   *configStore
	access  *accessController
	retire  *retire.Log
	jobs    *recordingService
	version string
}

// recordingService decorates the launchd service so a retirement triggered from
// the dashboard is also written to the local retirement log. launchd keeps only
// the override itself, so without this the UI could never answer "when did I
// switch this off, and was it me?".
type recordingService struct {
	*launchd.Service
	log *retire.Log
}

func (r recordingService) Disable(label string) error {
	if err := r.Service.Disable(label); err != nil {
		return err
	}
	if err := r.log.Record(label, "disable", time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "deployboard: retirement log:", err)
	}
	return nil
}

func (r recordingService) Enable(label string) error {
	if err := r.Service.Enable(label); err != nil {
		return err
	}
	if err := r.log.Record(label, "enable", time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "deployboard: retirement log:", err)
	}
	return nil
}

// RestartSelf delegates. A self-restart is launchd respawning this process,
// not a retirement, so it is not written to the retirement log.
func (r recordingService) RestartSelf() error {
	return r.Service.RestartSelf()
}

// applyJobAction is the single dispatch point shared by the HTTP handler and the
// bulk path, so both mean exactly the same thing by "stop".
func applyJobAction(svc server.JobService, job launchd.Job, action string) error {
	switch action {
	case "reload":
		return svc.Reload(job.Label)
	case "start":
		return svc.Start(job.Label)
	case "stop":
		return svc.Stop(job.Label)
	case "disable":
		return svc.Disable(job.Label)
	case "enable":
		return svc.Enable(job.Label)
	}
	return fmt.Errorf("unknown action %q", action)
}

// actionNote explains, in the API response, what an action actually did.
func actionNote(job launchd.Job, action string) string {
	switch action {
	case "stop":
		if job.KeepAlive {
			return "unloaded from launchd — it has KeepAlive, so a signal alone would have restarted it"
		}
	case "start":
		if job.Disabled {
			return "re-enabled and started (it was disabled)"
		}
	case "disable":
		return "retired: launchd will not start it again, including at login"
	case "enable":
		return "no longer retired — press Start to bring it up"
	}
	return ""
}

// GroupAction applies one action to every job in a group (the UI's bulk row
// buttons). Jobs with nothing to do are reported as skipped rather than failed,
// so "stop all" on a half-idle stack is not a wall of red.
func (a *appState) GroupAction(group, action string) ([]server.GroupActionResult, error) {
	if strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("group is required")
	}
	switch action {
	case "reload", "start", "stop", "disable", "enable":
	default:
		return nil, fmt.Errorf("action must be reload|start|stop|disable|enable, got %q", action)
	}

	all, err := a.svc.ListJobs()
	if err != nil {
		return nil, err
	}
	results := make([]server.GroupActionResult, 0, len(all))
	// A reload of this process has to be last. bootout of our own label
	// would kill the handler while the rest of the group was still queued,
	// and launchd would not bring the dashboard back.
	var selfLabel string
	for _, job := range all {
		if job.Group != group || job.Category == string(inventory.CategoryNoise) {
			continue
		}
		if action == "reload" && a.svc.IsSelf(job.Label) {
			selfLabel = job.Label
			continue
		}
		res := server.GroupActionResult{Label: job.Label}
		loaded := job.Status != launchd.StatusOffline && job.Status != launchd.StatusDisabled
		switch action {
		case "stop":
			// A KeepAlive job that is between restarts has pid 0 but is very much
			// alive in launchd's eyes — the restart loop is the thing to stop, so
			// only "not loaded at all" or "idle and nothing brings it back" skip.
			switch {
			case !loaded:
				res.Note = "not loaded"
				results = append(results, res)
				continue
			case !job.KeepAlive && job.PID == 0:
				res.Note = "not running"
				results = append(results, res)
				continue
			}
		case "start":
			if job.PID > 0 {
				res.Note = "already running"
				results = append(results, res)
				continue
			}
		case "disable":
			if job.Disabled {
				res.Note = "already retired"
				results = append(results, res)
				continue
			}
		case "enable":
			if !job.Disabled {
				res.Note = "not retired"
				results = append(results, res)
				continue
			}
		}
		if err := applyJobAction(a.jobs, job, action); err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		res.OK = true
		res.Note = actionNote(job, action)
		results = append(results, res)
	}
	if selfLabel != "" {
		results = append(results, server.GroupActionResult{
			Label: selfLabel,
			OK:    true,
			Note:  launchd.SelfRestartNote,
			Self:  true,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no jobs found in group %q", group)
	}
	return results, nil
}

// inventorySignals turns the current job list into classifier signals (the
// path-aware inputs), which is what /api/inventory reports on.
func (a *appState) inventorySignals() []inventory.Signal {
	jobs, err := a.svc.ListJobs()
	if err != nil {
		return nil
	}
	out := make([]inventory.Signal, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, inventory.Signal{
			Label:       j.Label,
			Program:     j.Program,
			ProgramArgs: j.ProgramArgs,
			LogPath:     j.StandardOutPath,
			ErrLogPath:  j.StandardErrPath,
			WorkingDir:  j.WorkingDirectory,
			PlistPath:   j.PlistPath,
		})
	}
	return out
}

// metricsInput converts the current job list (plus probes) into metrics input.
func (a *appState) MetricsInput() metrics.Input {
	jobs, err := a.svc.ListJobs()
	if err != nil {
		return metrics.Input{Version: a.version}
	}
	in := metrics.Input{Version: a.version, Jobs: make([]metrics.JobMetric, 0, len(jobs))}
	for _, j := range jobs {
		m := metrics.JobMetric{
			Label: j.Label, Category: j.Category, Group: j.Group, Status: string(j.Status),
			Runs: j.Runs, HasExit: j.HasExit, LastExitCode: j.LastExitStatus,
			Disabled: j.Disabled, AlertEnabled: a.alerts != nil && a.alertEnabled(j),
		}
		if a.prober != nil && j.Category != string(inventory.CategoryNoise) {
			for _, res := range a.prober.ProbeArgs(j.ProgramArgs) {
				_ = res
				// One port per series keeps cardinality predictable; the first
				// inferred port is the service's own listen port.
				m.HasProbe = true
				m.ProbePort = res.Port
				m.ProbeOK = res.Reachable
				m.ProbeStatus = res.Status
				m.ProbeLatencyS = res.Latency.Seconds()
				in.Jobs = append(in.Jobs, m)
				m.HasProbe = false
			}
			if !m.HasProbe {
				in.Jobs = append(in.Jobs, m)
			}
			continue
		}
		in.Jobs = append(in.Jobs, m)
	}
	return in
}

// alertEnabled reports whether the engine would alert for this job.
func (a *appState) alertEnabled(j launchd.Job) bool {
	if a.alerts == nil {
		return false
	}
	ls := a.state.Get(j.Label)
	if explicit, set := ls.ExplicitEnable(); set {
		return explicit
	}
	if j.Category != string(inventory.CategoryOurs) || j.Disabled || j.Status == launchd.StatusDisabled {
		return false
	}
	return true
}

// InventoryReport summarises the current inventory for /api/inventory.
func (a *appState) InventoryReport() (inventory.Report, error) {
	signals := a.inventorySignals()
	if signals == nil {
		if _, err := a.svc.ListJobs(); err != nil {
			return inventory.Report{}, err
		}
	}
	return a.svc.Classifier().BuildReport(signals), nil
}

// ClassifyCategory pins a label's category in config.json (UI classify button).
func (a *appState) ClassifyCategory(label, category string) (inventory.Report, error) {
	if a.store == nil {
		return inventory.Report{}, fmt.Errorf("config store unavailable")
	}
	if err := a.store.Classify(label, category); err != nil {
		return inventory.Report{}, err
	}
	signals := a.inventorySignals()
	return a.svc.Classifier().BuildReport(signals), nil
}

// ReloadConfig re-reads config.json on demand (POST /api/inventory/reload).
func (a *appState) ReloadConfig() error {
	if a.store == nil {
		return fmt.Errorf("config store unavailable")
	}
	return a.store.Reload()
}

// jobStates converts launchd jobs into alert evaluation inputs, including probe
// outcomes for the jobs that expose a port.
func (a *appState) jobStates() []alerts.JobState {
	jobs, err := a.svc.ListJobs()
	if err != nil {
		return nil
	}
	out := make([]alerts.JobState, 0, len(jobs))
	for _, j := range jobs {
		if j.Category == string(inventory.CategoryNoise) {
			continue
		}
		js := alerts.JobState{
			Label: j.Label, Category: j.Category, Group: j.Group, Status: string(j.Status),
			Disabled: j.Disabled, Runs: j.Runs, HasExit: j.HasExit, LastExitCode: j.LastExitStatus,
			LogPath: j.StandardOutPath,
		}
		if a.prober != nil {
			for _, r := range a.prober.ProbeArgs(j.ProgramArgs) {
				js.HasProbe = true
				js.ProbeOK = js.ProbeOK || r.Reachable
			}
		}
		out = append(out, js)
	}
	return out
}

// alertLoop evaluates transitions on an interval until ctx is done.
func (a *appState) alertLoop(ctx context.Context, every time.Duration) {
	if a.alerts == nil {
		return
	}
	if msg := a.alerts.WarnOnce(); msg != "" {
		fmt.Fprintln(os.Stderr, "deployboard:", msg)
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticks++
			// A token may be stored after startup (settings UI or
			// the CLI); re-resolve it periodically instead of
			// requiring a restart.
			if ticks%12 == 0 && !a.alerts.Enabled() {
				conf := a.alerts.TelegramConfig()
				if a.store != nil {
					conf = a.store.telegramConf(conf)
				}
				a.alerts.Configure(conf)
			}
			a.alerts.Evaluate(ctx, a.jobStates())
			if err := a.state.Save(); err != nil {
				fmt.Fprintln(os.Stderr, "deployboard: persist alerts state:", err)
			}
		}
	}
}

// alertControl adapts the alert engine to the server's AlertsControl interface.
type alertControl struct {
	app *appState
}

func (c alertControl) Available() bool { return c.app.alerts != nil && c.app.alerts.Enabled() }

func (c alertControl) Snapshot() map[string]alerts.LabelState { return c.app.state.Snapshot() }

func (c alertControl) Counts() (int, int) {
	enabled, disabled := 0, 0
	for _, j := range c.app.jobStates() {
		if c.app.alertEnabledLabel(j.Label, j.Category, j.Disabled, j.Status) {
			enabled++
		} else {
			disabled++
		}
	}
	return enabled, disabled
}

func (c alertControl) SetEnabled(label string, enabled bool) alerts.LabelState {
	ls := c.app.state.SetEnabled(label, enabled)
	if err := c.app.state.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "deployboard: persist alerts state:", err)
	}
	return ls
}

func (c alertControl) BulkSet(labels []string, enabled bool) int {
	for _, l := range labels {
		c.app.state.SetEnabled(l, enabled)
	}
	if err := c.app.state.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "deployboard: persist alerts state:", err)
	}
	return len(labels)
}

func (c alertControl) History(limit int) []alerts.Entry {
	if c.app.alerts == nil {
		return nil
	}
	return c.app.alerts.History(limit)
}

func (c alertControl) SendTest(ctx context.Context, label string) (int, error) {
	if c.app.alerts == nil {
		return 0, fmt.Errorf("alerts unavailable")
	}
	var target *alerts.JobState
	for _, j := range c.app.jobStates() {
		if j.Label == label {
			js := j
			target = &js
			break
		}
	}
	if target == nil {
		return 0, fmt.Errorf("unknown label: %s", label)
	}
	return c.app.alerts.SendTestAlert(ctx, *target)
}

// alertEnabledLabel mirrors appState.alertEnabled for a bare JobState.
func (a *appState) alertEnabledLabel(label, category string, disabled bool, status string) bool {
	ls := a.state.Get(label)
	if explicit, set := ls.ExplicitEnable(); set {
		return explicit
	}
	if category != string(inventory.CategoryOurs) || disabled || status == string(launchd.StatusDisabled) {
		return false
	}
	return true
}

// newApp builds the runtime from the effective config.
func newApp(cfg Config) (*appState, error) {
	svc := launchd.NewServiceWithWindow(cfg.RecentWindow)
	svc.SetSelfLabel(cfg.SelfLabel)
	classifier := inventory.New(cfg.Inventory)
	svc.SetEnrichment(classifier, cfg.PrintTTL)

	aCfg := alertConfig(cfg.Alerts)
	state, err := alerts.LoadState(aCfg.StateFile)
	if err != nil {
		return nil, err
	}
	app := &appState{
		svc:     svc,
		state:   state,
		alerts:  alerts.NewEngine(aCfg, state),
		version: Version,
	}
	app.store = newConfigStore(cfg.ConfigPath, classifier.Config(), svc, cfg.PrintTTL, app.alerts)
	app.access = newAccessController(cfg.ReadOnly, cfg.ReadOnlyFlag, app.store)
	app.store.access = app.access
	app.retire = retire.Open("")
	svc.SetProcessLookup(launchd.PSLookup)
	svc.SetRetirementLookup(app.retire.DisabledSince)
	app.jobs = &recordingService{Service: svc, log: app.retire}
	if !cfg.NoProbe {
		app.prober = probe.New(cfg.ProbeTTL)
	}
	return app, nil
}

// newServer creates an http.Server wired with the full router (launchd service,
// diagnose engine, fork routes, embedded frontend).
func newServer(cfg Config, app *appState) (*http.Server, error) {
	diag := &diagnose.Engine{}
	deps := server.ForkDeps{
		Version:   Version,
		Access:    app.access,
		Inventory: app,
		Control:   app,
		Groups:    app,
		Metrics:   app,
		Alerts:    alertControl{app: app},
		Telegram:  telegramSettings{app: app, store: app.store},
		Jobs:      app.jobs,
	}
	router := server.NewRouterWithFork(app.jobs, diag, web.FS, deps)
	return &http.Server{Handler: router}, nil
}

func main() {
	cfg, versionRequested, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "deployboard:", err)
		os.Exit(1)
	}
	if versionRequested {
		fmt.Println("deployboard", Version)
		return
	}

	app, err := newApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deployboard: %v\n", err)
		os.Exit(1)
	}

	srv, err := newServer(cfg, app)
	if err != nil {
		fmt.Fprintf(os.Stderr, "deployboard: %v\n", err)
		os.Exit(1)
	}

	ln, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		fmt.Fprintf(os.Stderr, "deployboard: listen: %v\n", err)
		os.Exit(1)
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
	printBanner(os.Stdout, url)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.alertLoop(ctx, 5*time.Second)
	}()

	// Hot reload: config.json edits (by hand or by the UI classify button) are
	// applied within ~2s instead of needing a restart.
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.store.watch(ctx, 2*time.Second)
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "deployboard: serve: %v\n", err)
		}
	}()

	if !cfg.NoOpen {
		if err := exec.Command("open", url).Start(); err != nil {
			fmt.Fprintf(os.Stderr, "deployboard: open browser: %v\n", err)
		}
	}

	<-ctx.Done()
	stop() // restore default signal behavior so a second Ctrl-C aborts cleanly

	fmt.Println("\nShutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		// SSE clients hold long-lived connections, so Shutdown routinely hits the
		// deadline. Close() drops them and is the correct way to finish exiting —
		// not a failure worth a non-zero exit code.
		_ = srv.Close()
	}
	wg.Wait()
	if err := app.state.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "deployboard: persist alerts state: %v\n", err)
	}
}
