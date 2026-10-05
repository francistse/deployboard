package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadConfigFile reads config.json and returns just the "inventory" section.
// A missing file yields the built-in defaults.
func LoadConfigFile(path string) (Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	var wrapper struct {
		Inventory Config `json:"inventory"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return wrapper.Inventory, nil
}

// SaveConfigFile writes the inventory section back into config.json, preserving
// every other key (port, alerts, read_only …) byte-for-byte as parsed values.
// The write is atomic (temp + rename) so a watcher never sees a half file.
func SaveConfigFile(path string, cfg Config) error {
	if path == "" {
		return fmt.Errorf("no config path configured")
	}
	raw, err := readRaw(path)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw["inventory"] = encoded
	return writeRaw(path, raw)
}

// SetReadOnly writes the top-level "read_only" key, preserving every other key.
// This is the switch the UI's write-access toggler flips: the setting is a
// property of the file, so the server picks it up through the same hot-reload
// path as a hand edit, and a restart keeps whatever the user chose.
func SetReadOnly(path string, readOnly bool) error {
	if path == "" {
		return fmt.Errorf("no config path configured")
	}
	raw, err := readRaw(path)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(readOnly)
	if err != nil {
		return err
	}
	raw["read_only"] = encoded
	return writeRaw(path, raw)
}

// readRaw reads config.json into a key-preserving map. A missing file is fine:
// the result is an empty map and the caller adds its own keys.
func readRaw(path string) (map[string]json.RawMessage, error) {
	raw := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return raw, nil
}

// writeRaw serialises the map and swaps it in atomically.
func writeRaw(path string, raw map[string]json.RawMessage) error {
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WithPattern returns a copy of the config with the pattern added to the given
// category ("ours" or "hidden"). Duplicates are ignored.
func (c Config) WithPattern(cat Category, pattern string) Config {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return c
	}
	switch cat {
	case CategoryOurs:
		if !contains(c.Ours, pattern) {
			c.Ours = append(append([]string{}, c.Ours...), pattern)
			sort.Strings(c.Ours)
		}
	case CategoryNoise:
		if !contains(c.Hidden, pattern) {
			c.Hidden = append(append([]string{}, c.Hidden...), pattern)
			sort.Strings(c.Hidden)
		}
	}
	return c
}

// WithoutPattern removes the pattern from both lists (the "Auto" action).
func (c Config) WithoutPattern(pattern string) Config {
	c.Ours = remove(c.Ours, pattern)
	c.Hidden = remove(c.Hidden, pattern)
	return c
}

// WithoutPatternFrom removes the pattern from one category only, so a pin can
// move from `ours` to `hidden` without touching the other list.
func (c Config) WithoutPatternFrom(cat Category, pattern string) Config {
	switch cat {
	case CategoryOurs:
		c.Ours = remove(c.Ours, pattern)
	case CategoryNoise:
		c.Hidden = remove(c.Hidden, pattern)
	}
	return c
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func remove(list []string, v string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s != v {
			out = append(out, s)
		}
	}
	return out
}

// FileStamp is a cheap change detector for hot reload (mtime + size).
type FileStamp struct {
	ModTime int64
	Size    int64
}

// Stamp returns the current stamp of a file, or a zero stamp when unreadable.
func Stamp(path string) FileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return FileStamp{}
	}
	return FileStamp{ModTime: info.ModTime().UnixNano(), Size: info.Size()}
}
