package alerts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigUsesDeployboardIdentity(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Telegram.KeychainService != "deployboard-telegram" {
		t.Fatalf("KeychainService = %q, want deployboard-telegram", cfg.Telegram.KeychainService)
	}
	if strings.Contains(cfg.StateFile, "launch-pilot") {
		t.Fatalf("StateFile = %q, still under the previous config dir", cfg.StateFile)
	}
	wantSuffix := filepath.Join(".config", "deployboard", "alerts.json")
	if cfg.StateFile != "alerts.json" && !strings.HasSuffix(cfg.StateFile, wantSuffix) {
		t.Fatalf("StateFile = %q, want suffix %q", cfg.StateFile, wantSuffix)
	}
}

func TestLoadStateReadsLegacyFileWhenNewIsAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, ".config", "launch-pilot")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "alerts.json")
	body := []byte(`{"version":1,"labels":{"com.example.app":{"enabled":true,"explicit":true}}}`)
	if err := os.WriteFile(legacy, body, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadState(defaultStateFile())
	if err != nil {
		t.Fatal(err)
	}
	if s.path != legacy {
		t.Fatalf("path = %q, want legacy %q", s.path, legacy)
	}
	ls := s.Get("com.example.app")
	if !ls.Enabled || ls.Explicit == nil || !*ls.Explicit {
		t.Fatalf("legacy label state = %+v", ls)
	}
	s.SetEnabled("com.example.app", false)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "deployboard", "alerts.json")); !os.IsNotExist(err) {
		t.Fatalf("Save must keep writing the legacy file, stat err = %v", err)
	}
	again, err := LoadState(defaultStateFile())
	if err != nil {
		t.Fatal(err)
	}
	if again.path != legacy {
		t.Fatalf("reloaded path = %q, want legacy %q", again.path, legacy)
	}
	if again.Get("com.example.app").Enabled {
		t.Fatal("Save did not persist the legacy file")
	}
}

func TestLoadStatePrefersNewFileOverLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	newDir := filepath.Join(home, ".config", "deployboard")
	oldDir := filepath.Join(home, ".config", "launch-pilot")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(newDir, "alerts.json")
	if err := os.WriteFile(newPath, []byte(`{"version":1,"labels":{"from-new":{"enabled":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "alerts.json"), []byte(`{"version":1,"labels":{"from-old":{"enabled":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadState(defaultStateFile())
	if err != nil {
		t.Fatal(err)
	}
	if s.path != newPath {
		t.Fatalf("path = %q, want %q", s.path, newPath)
	}
	snap := s.Snapshot()
	if _, ok := snap["from-new"]; !ok {
		t.Fatalf("snapshot = %+v, want from-new", snap)
	}
	if _, ok := snap["from-old"]; ok {
		t.Fatal("legacy file must not be read when the new file exists")
	}
}
