package inventory

import (
	"os"
	"path/filepath"
	"strings"
)

// Source explains why a label landed in its category, so the UI can show
// "auto (path)" vs "listed" and you know what is worth pinning down.
type Source string

const (
	// SourceOursPattern — matched an explicit pattern in inventory.ours.
	SourceOursPattern Source = "ours_pattern"
	// SourceHiddenPattern — matched an explicit pattern in inventory.hidden.
	SourceHiddenPattern Source = "hidden_pattern"
	// SourceDerivedPath — matched no pattern, but its files live under a
	// configured project root (`derive_roots`), so it is one of ours.
	SourceDerivedPath Source = "derived_path"
	// SourceDefault — nothing matched: shown as "Other".
	SourceDefault Source = "default"
)

// Signal is everything the classifier may look at for one job.
//
// The label is not always enough: his deployments are identified by living in
// his project tree, and the plist's program is usually just the interpreter
// (`node`, `.venv/bin/python`). The log path, working directory and arguments
// are what actually carry the project path.
type Signal struct {
	Label       string
	Program     string
	ProgramArgs []string
	LogPath     string
	ErrLogPath  string
	WorkingDir  string
	PlistPath   string
}

// haystacks returns every string that may contain a project path.
func (s Signal) haystacks() []string {
	out := make([]string, 0, 4+len(s.ProgramArgs))
	if s.LogPath != "" {
		out = append(out, s.LogPath)
	}
	if s.ErrLogPath != "" {
		out = append(out, s.ErrLogPath)
	}
	if s.WorkingDir != "" {
		out = append(out, s.WorkingDir)
	}
	if s.Program != "" {
		out = append(out, s.Program)
	}
	out = append(out, s.ProgramArgs...)
	return out
}

// ClassifyWithSource classifies a job and reports which rule decided it.
//
// Order matters and is deliberate:
//  1. explicit `ours` pattern — the user said it out loud, it wins;
//  2. explicit `hidden` pattern — vendor/OS jobs stay noise even if a stray
//     path would match;
//  3. derived: any path under `derive_roots` → ours (this is what removes the
//     whitelist maintenance burden);
//  4. everything else → other, which the UI surfaces as "unclassified".
func (c *Classifier) ClassifyWithSource(sig Signal) (Category, Source) {
	if Match(c.cfg.Ours, sig.Label) {
		return CategoryOurs, SourceOursPattern
	}
	if Match(c.cfg.Hidden, sig.Label) {
		return CategoryNoise, SourceHiddenPattern
	}
	if c.underRoot(sig) {
		return CategoryOurs, SourceDerivedPath
	}
	return CategoryOther, SourceDefault
}

// underRoot reports whether any of the job's paths starts with a derive root.
// Roots may use a leading `~/` (people write it that way in config.json), which
// filepath.Clean does NOT expand — so expand it here.
func (c *Classifier) underRoot(sig Signal) bool {
	roots := c.cfg.DeriveRoots
	if len(roots) == 0 {
		return false
	}
	home, _ := os.UserHomeDir()
	for _, hay := range sig.haystacks() {
		clean := expandHome(filepath.Clean(hay), home)
		for _, root := range roots {
			r := expandHome(filepath.Clean(root), home)
			if r == "" || r == "." || r == "/" {
				continue
			}
			if clean == r || strings.HasPrefix(clean, r+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// expandHome resolves a leading "~" against home; any other path is returned
// unchanged.
func expandHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}

// Classify keeps the label-only entry point used by tests and simple callers.
func (c *Classifier) Classify(label string) Category {
	cat, _ := c.ClassifyWithSource(Signal{Label: label})
	return cat
}

// DeriveRoots returns the configured project roots.
func (c *Classifier) DeriveRoots() []string {
	if c == nil {
		return nil
	}
	return c.cfg.DeriveRoots
}
