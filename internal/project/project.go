// Package project scans derive_roots for deployboard.yaml / deployboard.json (Phase 3).
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/A404coder/deployboard/internal/contract"
	"github.com/A404coder/deployboard/internal/desired"
	"github.com/A404coder/deployboard/internal/inventory"
	"gopkg.in/yaml.v3"
)

// File is the on-disk project registration schema.
type File struct {
	Ours       []string        `json:"ours" yaml:"ours"`
	Labels     []string        `json:"labels" yaml:"labels"` // alias for ours
	Group      string          `json:"group" yaml:"group"`
	GroupMatch []string        `json:"group_match" yaml:"group_match"`
	Desired    desired.Config  `json:"desired" yaml:"desired"`
	Contracts  contract.Config `json:"contracts" yaml:"contracts"`
	CronMatch  []string        `json:"cron_match" yaml:"cron_match"`
	Path       string          `json:"-" yaml:"-"`
}

// Merge is the combined overlay from all discovered project files.
type Merge struct {
	Ours      []string
	Groups    []inventory.Group
	Desired   desired.Config
	Contracts contract.Config
	CronMatch []string
	Files     []string
}

// Scan walks each derive_root and its immediate children for deployboard.yaml/json.
func Scan(deriveRoots []string) (Merge, error) {
	var m Merge
	roots := expandRoots(deriveRoots)
	seen := map[string]bool{}
	for _, root := range roots {
		entries := []string{root}
		children, _ := os.ReadDir(root)
		for _, c := range children {
			if c.IsDir() {
				entries = append(entries, filepath.Join(root, c.Name()))
			}
		}
		for _, dir := range entries {
			for _, name := range []string{"deployboard.yaml", "deployboard.yml", "deployboard.json"} {
				path := filepath.Join(dir, name)
				if seen[path] {
					continue
				}
				info, err := os.Stat(path)
				if err != nil || info.IsDir() {
					continue
				}
				f, err := Load(path)
				if err != nil {
					return m, fmt.Errorf("%s: %w", path, err)
				}
				seen[path] = true
				m = m.overlay(f)
				m.Files = append(m.Files, path)
			}
		}
	}
	return m, nil
}

// Load parses one project file.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &f); err != nil {
			return File{}, err
		}
	default:
		if err := yaml.Unmarshal(data, &f); err != nil {
			return File{}, err
		}
	}
	f.Path = path
	return f, nil
}

func (m Merge) overlay(f File) Merge {
	ours := append([]string{}, f.Ours...)
	ours = append(ours, f.Labels...)
	m.Ours = appendUnique(m.Ours, ours)
	if f.Group != "" {
		match := f.GroupMatch
		if len(match) == 0 {
			match = ours
		}
		if len(match) > 0 {
			m.Groups = append(m.Groups, inventory.Group{Name: f.Group, Match: match})
		}
	}
	m.Desired.Jobs = append(m.Desired.Jobs, f.Desired.Jobs...)
	m.Desired.Groups = append(m.Desired.Groups, f.Desired.Groups...)
	m.Contracts = append(m.Contracts, f.Contracts...)
	m.CronMatch = appendUnique(m.CronMatch, f.CronMatch)
	return m
}

// ApplyInventory merges project overlays onto a base inventory config.
// Project ours/groups are prepended so they win first-match.
func ApplyInventory(base inventory.Config, m Merge) inventory.Config {
	out := base
	out.Ours = appendUnique(m.Ours, base.Ours)
	out.Groups = append(append([]inventory.Group{}, m.Groups...), base.Groups...)
	return out
}

// ApplyDesired prepends project desired rules (first match wins in desired.Resolve).
func ApplyDesired(base desired.Config, m Merge) desired.Config {
	out := base
	out.Jobs = append(append([]desired.JobRule{}, m.Desired.Jobs...), base.Jobs...)
	out.Groups = append(append([]desired.GroupRule{}, m.Desired.Groups...), base.Groups...)
	return out
}

// ApplyContracts prepends project contracts.
func ApplyContracts(base contract.Config, m Merge) contract.Config {
	return append(append(contract.Config{}, m.Contracts...), base...)
}

func appendUnique(dst, src []string) []string {
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range src {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		dst = append(dst, s)
	}
	return dst
}

func expandRoots(roots []string) []string {
	home, _ := os.UserHomeDir()
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.HasPrefix(r, "~/") && home != "" {
			r = filepath.Join(home, r[2:])
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			out = append(out, abs)
		}
	}
	return out
}
