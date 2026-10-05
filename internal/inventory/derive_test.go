package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyWithSource_PatternsWin(t *testing.T) {
	cfg := Config{
		Ours:        []string{"com.example.app.*"},
		Hidden:      []string{"com.apple.*"},
		DeriveRoots: []string{"/Users/example/Projects"},
	}
	cl := New(cfg)

	// An explicit hidden pattern must beat a stray path under a project root.
	cat, src := cl.ClassifyWithSource(Signal{
		Label:     "com.apple.thing",
		LogPath:   "/Users/example/Projects/whatever.log",
		PlistPath: "/Users/example/Library/LaunchAgents/com.apple.thing.plist",
	})
	if cat != CategoryNoise || src != SourceHiddenPattern {
		t.Errorf("hidden pattern should win over derived path: cat=%s src=%s", cat, src)
	}

	cat, src = cl.ClassifyWithSource(Signal{Label: "com.example.app.uat.api"})
	if cat != CategoryOurs || src != SourceOursPattern {
		t.Errorf("ours pattern expected: cat=%s src=%s", cat, src)
	}
}

func TestClassifyWithSource_DerivedFromEachPath(t *testing.T) {
	root := "/Users/example/Projects"
	cl := New(Config{Ours: []string{"com.explicit.*"}, Hidden: []string{"com.apple.*"}, DeriveRoots: []string{root}})

	cases := []struct {
		name string
		sig  Signal
		want Source
	}{
		{"log path", Signal{Label: "com.new.app", LogPath: root + "/app/logs/api.log"}, SourceDerivedPath},
		{"err log path", Signal{Label: "com.new.app", ErrLogPath: root + "/x/err.log"}, SourceDerivedPath},
		{"working directory", Signal{Label: "com.new.app", WorkingDir: root + "/app"}, SourceDerivedPath},
		{"program", Signal{Label: "com.new.app", Program: root + "/x/.venv/bin/python"}, SourceDerivedPath},
		{"program args", Signal{Label: "com.new.app", ProgramArgs: []string{"node", root + "/apps/web/next", "dev"}}, SourceDerivedPath},
		{"root itself", Signal{Label: "com.new.app", WorkingDir: root}, SourceDerivedPath},
		{"sibling not under root", Signal{Label: "com.new.app", WorkingDir: "/Users/example/Projects2"}, SourceDefault},
		{"home but not a root", Signal{Label: "com.new.app", LogPath: "/Users/example/Library/Logs/x.log"}, SourceDefault},
		{"interpreter only", Signal{Label: "com.new.app", Program: "/Users/example/.local/node/bin/node"}, SourceDefault},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat, src := cl.ClassifyWithSource(c.sig)
			if src != c.want {
				t.Errorf("src = %s, want %s (cat=%s)", src, c.want, cat)
			}
			if c.want == SourceDerivedPath && cat != CategoryOurs {
				t.Errorf("derived path must be ours, got %s", cat)
			}
		})
	}
}

func TestUnderRoot_IgnoresDegenerateRoots(t *testing.T) {
	cl := New(Config{Ours: []string{"zzz"}, Hidden: []string{"com.apple.*"}, DeriveRoots: []string{"/", ".", ""}})
	if _, src := cl.ClassifyWithSource(Signal{Label: "com.x", LogPath: "/anything/at/all"}); src != SourceDefault {
		t.Errorf("degenerate roots must not match everything, got %s", src)
	}
}

func TestUnderRoot_TildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	cl := New(Config{Ours: []string{"zzz"}, Hidden: []string{"com.apple.*"}, DeriveRoots: []string{"~/Projects"}})

	if _, src := cl.ClassifyWithSource(Signal{Label: "com.x", LogPath: home + "/Projects/app/logs/x.log"}); src != SourceDerivedPath {
		t.Errorf("~/ root must expand and match, got %s", src)
	}
	if _, src := cl.ClassifyWithSource(Signal{Label: "com.x", LogPath: home + "/other/x.log"}); src != SourceDefault {
		t.Errorf("path outside the expanded root must not match, got %s", src)
	}
}

func TestExpandHome(t *testing.T) {
	cases := []struct{ in, home, want string }{
		{"~/x", "/Users/f", "/Users/f/x"},
		{"~", "/Users/f", "/Users/f"},
		{"/abs/path", "/Users/f", "/abs/path"},
		{"~/x", "", "~/x"},
	}
	for _, c := range cases {
		if got := expandHome(c.in, c.home); got != c.want {
			t.Errorf("expandHome(%q, %q) = %q, want %q", c.in, c.home, got, c.want)
		}
	}
}

func TestBuildReport_SourcesAndDerived(t *testing.T) {
	root := "/Users/example/Projects"
	cl := New(Config{Ours: []string{"com.explicit.*"}, Hidden: []string{"com.apple.*"}, DeriveRoots: []string{root}})

	rep := cl.BuildReport([]Signal{
		{Label: "com.explicit.one"},
		{Label: "com.auto.two", LogPath: root + "/app/logs/x.log"},
		{Label: "com.apple.Safari"},
		{Label: "com.unknown.three"},
	})

	if rep.Ours != 2 || rep.Noise != 1 || rep.Other != 1 {
		t.Errorf("counts: ours=%d noise=%d other=%d", rep.Ours, rep.Noise, rep.Other)
	}
	if len(rep.DerivedOurs) != 1 || rep.DerivedOurs[0] != "com.auto.two" {
		t.Errorf("DerivedOurs = %v", rep.DerivedOurs)
	}
	if len(rep.Unmatched) != 1 || rep.Unmatched[0] != "com.unknown.three" {
		t.Errorf("Unmatched = %v", rep.Unmatched)
	}
	if rep.Sources[string(SourceOursPattern)] != 1 || rep.Sources[string(SourceDerivedPath)] != 1 {
		t.Errorf("Sources = %v", rep.Sources)
	}
}

func TestConfigMutations(t *testing.T) {
	c := New(Config{Ours: []string{"a"}, Hidden: []string{"b"}}).Config()

	c = c.WithPattern(CategoryOurs, "com.new.*")
	if !contains(c.Ours, "com.new.*") {
		t.Error("WithPattern(ours) should add")
	}
	c = c.WithPattern(CategoryOurs, "com.new.*") // duplicate
	n := 0
	for _, p := range c.Ours {
		if p == "com.new.*" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("duplicate pattern added %d times", n)
	}

	c = c.WithPattern(CategoryNoise, "com.hide.*")
	if !contains(c.Hidden, "com.hide.*") {
		t.Error("WithPattern(noise) should add to hidden")
	}

	c = c.WithoutPatternFrom(CategoryOurs, "com.new.*")
	if contains(c.Ours, "com.new.*") {
		t.Error("WithoutPatternFrom(ours) should remove")
	}
	if !contains(c.Hidden, "com.hide.*") {
		t.Error("WithoutPatternFrom(ours) must not touch hidden")
	}

	c = c.WithoutPattern("com.hide.*")
	if contains(c.Hidden, "com.hide.*") {
		t.Error("WithoutPattern should remove from both")
	}
}

func TestSaveConfigFile_PreservesOtherSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := `{
  "port": 9410,
  "read_only": true,
  "alerts": {"enabled": true, "telegram": {"chat_id": "12345"}},
  "inventory": {"ours": ["a"], "hidden": ["b"]}
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	cfg = cfg.WithPattern(CategoryOurs, "com.pinned.*")
	if err := SaveConfigFile(path, cfg); err != nil {
		t.Fatalf("SaveConfigFile: %v", err)
	}

	reloaded, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !contains(reloaded.Ours, "com.pinned.*") {
		t.Error("pin not persisted")
	}

	// Other sections must survive untouched.
	raw, _ := os.ReadFile(path)
	body := string(raw)
	for _, want := range []string{`"port": 9410`, `"read_only": true`, `"chat_id": "12345"`} {
		if !strings.Contains(body, want) {
			t.Errorf("other config key lost: %s", want)
		}
	}
}

func TestSetReadOnly_PreservesOtherKeysAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := `{
  "port": 9410,
  "read_only": true,
  "alerts": {"telegram": {"chat_id": "12345"}},
  "inventory": {"ours": ["com.example.*"]}
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SetReadOnly(path, false); err != nil {
		t.Fatalf("SetReadOnly: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, `"read_only": false`) {
		t.Errorf("read_only not flipped: %s", body)
	}
	// Everything else must survive the write, including nested sections.
	for _, want := range []string{`"port": 9410`, `"chat_id": "12345"`, `"ours"`, `"com.example.*"`} {
		if !strings.Contains(body, want) {
			t.Errorf("other config key lost: %s", want)
		}
	}

	// The inventory loader must still parse the file the toggler wrote.
	if _, err := LoadConfigFile(path); err != nil {
		t.Errorf("LoadConfigFile after SetReadOnly: %v", err)
	}
}

func TestSetReadOnly_CreatesTheFileWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := SetReadOnly(path, true); err != nil {
		t.Fatalf("SetReadOnly on a missing file: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"read_only": true`) {
		t.Errorf("expected a fresh file with read_only true, got %s", raw)
	}
}

func TestLoadConfigFile_MissingUsesDefaults(t *testing.T) {
	cfg, err := LoadConfigFile(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(cfg.DeriveRoots) == 0 || len(cfg.Hidden) == 0 {
		t.Error("missing file should fall back to DefaultConfig")
	}
}

func TestStamp_DetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := Stamp(path)
	if a.Size == 0 {
		t.Fatal("stamp should be non-zero for an existing file")
	}
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b := Stamp(path)
	if a == b {
		t.Error("stamp should change after a write")
	}
}
