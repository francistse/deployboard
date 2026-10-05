// Package inventory classifies launchd jobs into the fork's "deployed
// applications" view: an explicit allowlist of labels you actually
// deployed, a hidden list for vendor/OS noise, and everything else.
//
// Upstream launch-pilot classifies by domain ("Mine" = user domain and not
// com.apple.*), which on a real workstation pulls in Dropbox, Chrome, Spotify,
// Tailscale, Docker and every updater. The allowlist is what makes the default
// view answer "which of MY applications are up" instead of "list 556 jobs".
package inventory

import (
	"fmt"
	"path"
	"sort"
)

// Category is the bucket a job falls into.
type Category string

const (
	// CategoryOurs is a label matched by Config.Ours — a deployed application.
	CategoryOurs Category = "ours"
	// CategoryNoise is a label matched by Config.Hidden — OS/vendor background.
	CategoryNoise Category = "noise"
	// CategoryOther is anything matched by neither list.
	CategoryOther Category = "other"
)

// Group is a named set of label patterns shown as one section in the UI.
type Group struct {
	Name    string   `json:"name"`
	Match   []string `json:"match"`
	NoAlert bool     `json:"noAlert,omitempty"`
}

// Config is the classification configuration (config.json → "inventory").
type Config struct {
	Groups               []Group  `json:"groups"`
	Ours                 []string `json:"ours"`
	Hidden               []string `json:"hidden"`
	DeriveRoots          []string `json:"derive_roots"`
	RestartWarnThreshold int      `json:"restart_warn_threshold"`
}

// DefaultConfig is the classification used when no config file is present.
//
// It is deliberately generic: a vendor/OS denylist plus a set of common project
// roots. The `ours` allowlist starts empty because path derivation covers the
// common case — anything it misses appears as "unclassified" in the UI, where one
// click pins it. Add your own namespaces to `ours` if you prefer explicit rules.
func DefaultConfig() Config {
	return Config{
		RestartWarnThreshold: 50,
		Groups: []Group{
			{Name: "Example App", Match: []string{"com.example.app.*"}},
			{Name: "Infra", Match: []string{"com.example.infra.*", "com.ollama.*"}},
		},
		Ours: []string{},
		Hidden: []string{
			"com.apple.*", "application.*", "com.google.*", "com.dropbox.*",
			"com.spotify.*", "com.docker.*", "com.github.CopilotForCode.*",
			"com.github.CopilotForXcode.*", "io.tailscale.*", "com.todesktop.*",
			"com.microsoft.*", "com.openssh.*", "com.protonvpn.*", "com.fobwifi.*",
			"org.virtualbox.*", "com.getdropbox.*", "ch.protonvpn.*",
		},
		// Any job whose plist log path, working directory, program or arguments
		// live under one of these roots is classified `ours` automatically, with
		// categorySource "derived_path". Tune to your own layout.
		DeriveRoots: []string{
			"~/Projects", "~/src", "~/code", "~/Developer", "~/work",
		},
	}
}

// Classifier answers category and group questions for labels.
type Classifier struct {
	cfg Config
}

// New builds a Classifier, applying defaults for any missing field so a partial
// config file cannot silently disable the view.
func New(cfg Config) *Classifier {
	if len(cfg.Ours) == 0 {
		cfg.Ours = DefaultConfig().Ours
	}
	if len(cfg.Hidden) == 0 {
		cfg.Hidden = DefaultConfig().Hidden
	}
	if len(cfg.Groups) == 0 {
		cfg.Groups = DefaultConfig().Groups
	}
	if len(cfg.DeriveRoots) == 0 {
		// Silent-disable guard: a config.json written before derive_roots
		// existed must not turn path derivation off without saying so.
		cfg.DeriveRoots = DefaultConfig().DeriveRoots
	}
	if cfg.RestartWarnThreshold <= 0 {
		cfg.RestartWarnThreshold = DefaultConfig().RestartWarnThreshold
	}
	return &Classifier{cfg: cfg}
}

// Config returns the effective configuration.
func (c *Classifier) Config() Config { return c.cfg }

// RestartWarnThreshold is the `runs` value at which a job is flagged as churning.
func (c *Classifier) RestartWarnThreshold() int {
	if c == nil {
		return DefaultConfig().RestartWarnThreshold
	}
	return c.cfg.RestartWarnThreshold
}

// Match reports whether label matches any pattern in patterns.
// Patterns use path.Match semantics; a bare label matches itself.
func Match(patterns []string, label string) bool {
	for _, p := range patterns {
		if p == label {
			return true
		}
		if ok, err := path.Match(p, label); err == nil && ok {
			return true
		}
	}
	return false
}

// Classify returns the category for a label: Ours, then Noise, else Other.
// The path-aware variant lives in derive.go (ClassifyWithSource).
func (c *Classifier) classifyLabelOnly(label string) Category {
	if Match(c.cfg.Ours, label) {
		return CategoryOurs
	}
	if Match(c.cfg.Hidden, label) {
		return CategoryNoise
	}
	return CategoryOther
}

// GroupName returns the first configured group whose patterns match the label,
// or "" when nothing matches.
func (c *Classifier) GroupName(label string) string {
	for _, g := range c.cfg.Groups {
		if Match(g.Match, label) {
			return g.Name
		}
	}
	return ""
}

// GroupReport is one group's membership and health summary.
type GroupReport struct {
	Name  string   `json:"name"`
	Match []string `json:"match"`
	Count int      `json:"count"`
}

// Report summarises an inventory: category counts, the labels nothing matched
// (so a new deployment is never silently invisible), per-group counts, and how
// each job was classified.
type Report struct {
	Ours        int                    `json:"ours"`
	Other       int                    `json:"other"`
	Noise       int                    `json:"noise"`
	Groups      []GroupReport          `json:"groups"`
	Unmatched   []string               `json:"unmatched"`
	Sources     map[string]int         `json:"sources"`
	DerivedOurs []string               `json:"derivedOurs,omitempty"`
	Paths       map[string]GroupReport `json:"-"`
}

// BuildReport summarises the given jobs (path-aware).
func (c *Classifier) BuildReport(signals []Signal) Report {
	rep := Report{
		Groups:  make([]GroupReport, 0, len(c.cfg.Groups)),
		Sources: map[string]int{},
	}
	for _, g := range c.cfg.Groups {
		rep.Groups = append(rep.Groups, GroupReport{Name: g.Name, Match: g.Match})
	}
	for _, sig := range signals {
		cat, src := c.ClassifyWithSource(sig)
		rep.Sources[string(src)]++
		switch cat {
		case CategoryOurs:
			rep.Ours++
			if src == SourceDerivedPath {
				rep.DerivedOurs = append(rep.DerivedOurs, sig.Label)
			}
		case CategoryNoise:
			rep.Noise++
		default:
			rep.Other++
			rep.Unmatched = append(rep.Unmatched, sig.Label)
		}
		if name := c.GroupName(sig.Label); name != "" {
			for i := range rep.Groups {
				if rep.Groups[i].Name == name {
					rep.Groups[i].Count++
				}
			}
		}
	}
	sort.Strings(rep.Unmatched)
	sort.Strings(rep.DerivedOurs)
	return rep
}

// BuildReportLabels is the label-only convenience wrapper.
func (c *Classifier) BuildReportLabels(labels []string) Report {
	sigs := make([]Signal, 0, len(labels))
	for _, l := range labels {
		sigs = append(sigs, Signal{Label: l})
	}
	return c.BuildReport(sigs)
}

// LabelListFromConfig is a helper for tests and docs: the effective ours+hidden
// patterns as a stable string.
func (c *Classifier) LabelListFromConfig() string {
	return fmt.Sprintf("ours=%v hidden=%v", c.cfg.Ours, c.cfg.Hidden)
}
