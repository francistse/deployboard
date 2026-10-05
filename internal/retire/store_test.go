package retire

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordAndDisabledSince(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retirements.json")
	log := Open(path)
	when := time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)

	if at, ok := log.DisabledSince("com.example.app"); ok {
		t.Fatalf("unknown label reported retired at %v", at)
	}

	if err := log.Record("com.example.app", "disable", when); err != nil {
		t.Fatalf("Record: %v", err)
	}
	at, ok := log.DisabledSince("com.example.app")
	if !ok {
		t.Fatal("label should be retired")
	}
	if !at.Equal(when) {
		t.Errorf("retired at %v, want %v", at, when)
	}

	// Re-enabling must clear the retired state: the row should stop claiming it
	// was switched off, even though the history keeps the older entry.
	if err := log.Record("com.example.app", "enable", when.Add(time.Hour)); err != nil {
		t.Fatalf("Record enable: %v", err)
	}
	if _, ok := log.DisabledSince("com.example.app"); ok {
		t.Error("an enabled job must not report as retired")
	}
	if e, ok := log.Get("com.example.app"); !ok || e.Action != "enable" {
		t.Errorf("latest entry = %+v, want action enable", e)
	}
}

func TestRecordPersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "retirements.json")
	when := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	first := Open(path)
	if err := first.Record("com.example.old", "disable", when); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// A restart of the dashboard must not lose the retirement history.
	second := Open(path)
	at, ok := second.DisabledSince("com.example.old")
	if !ok {
		t.Fatal("retirement did not survive a reload")
	}
	if !at.Equal(when.UTC()) {
		t.Errorf("reloaded at %v, want %v", at, when.UTC())
	}
	if len(second.Snapshot()) != 1 {
		t.Errorf("snapshot = %v, want one entry", second.Snapshot())
	}
}

func TestOpenToleratesGarbageAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"broken.json": "{not json",
		"empty.json":  "",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		log := Open(path)
		if _, ok := log.DisabledSince("com.example.app"); ok {
			t.Errorf("%s: expected an empty log, not a wrong answer", name)
		}
		if err := log.Record("com.example.app", "disable", time.Now()); err != nil {
			t.Errorf("%s: Record on a repaired log: %v", name, err)
		}
	}

	missing := Open(filepath.Join(dir, "does-not-exist.json"))
	if _, ok := missing.DisabledSince("x"); ok {
		t.Error("missing file should behave like an empty log")
	}
}

func TestRecordRejectsEmptyInput(t *testing.T) {
	log := Open(filepath.Join(t.TempDir(), "retirements.json"))
	if err := log.Record("", "disable", time.Now()); err == nil {
		t.Error("empty label should be rejected")
	}
	if err := log.Record("com.example.app", "", time.Now()); err == nil {
		t.Error("empty action should be rejected")
	}
}

func TestOpenReadsLegacyFileWhenNewIsAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, ".config", "launch-pilot")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "retirements.json")
	body := []byte(`{"com.example.app":{"action":"disable","at":"2026-03-04T10:30:00Z","source":"dashboard"}}` + "\n")
	if err := os.WriteFile(legacy, body, 0o644); err != nil {
		t.Fatal(err)
	}

	log := Open("")
	if log.Path() != legacy {
		t.Fatalf("path = %q, want legacy %q", log.Path(), legacy)
	}
	at, ok := log.DisabledSince("com.example.app")
	if !ok {
		t.Fatal("legacy retirement was not read")
	}
	want := time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)
	if !at.Equal(want) {
		t.Fatalf("retired at %v, want %v", at, want)
	}
	if err := log.Record("com.example.other", "disable", want); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "deployboard", "retirements.json")); !os.IsNotExist(err) {
		t.Fatalf("Record must keep writing the legacy file, stat err = %v", err)
	}
}

func TestOpenPrefersNewFileOverLegacy(t *testing.T) {
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
	newPath := filepath.Join(newDir, "retirements.json")
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.WriteFile(newPath, []byte(`{"from-new":{"action":"disable","at":"2026-01-02T03:04:05Z","source":"dashboard"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "retirements.json"), []byte(`{"from-old":{"action":"disable","at":"2026-01-02T03:04:05Z","source":"dashboard"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	log := Open(DefaultPath())
	if log.Path() != newPath {
		t.Fatalf("path = %q, want %q", log.Path(), newPath)
	}
	if _, ok := log.DisabledSince("from-new"); !ok {
		t.Fatal("new file was not read")
	}
	if _, ok := log.DisabledSince("from-old"); ok {
		t.Fatal("legacy file must not be read when the new file exists")
	}
	if _, ok := log.DisabledSince("from-new"); !ok || !log.Snapshot()["from-new"].At.Equal(when) {
		t.Fatalf("from-new = %+v", log.Snapshot()["from-new"])
	}
}

func TestOpenDoesNotFallbackForCustomPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, ".config", "launch-pilot")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "retirements.json"), []byte(`{"com.example.app":{"action":"disable","at":"2026-01-01T00:00:00Z","source":"dashboard"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(home, "custom.json")
	log := Open(custom)
	if log.Path() != custom {
		t.Fatalf("path = %q, want %q", log.Path(), custom)
	}
	if _, ok := log.Get("com.example.app"); ok {
		t.Fatal("custom path must not read the legacy file")
	}
}
