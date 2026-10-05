package incident

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incidents.jsonl")
	s := Open(path)
	now := time.Now().UTC()
	if err := s.Append(Event{At: now.Add(-time.Hour), Kind: KindStatus, Label: "a", Detail: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Event{At: now, Kind: KindDrift, Label: "b", Detail: "new"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Since(now.Add(-30 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Label != "b" {
		t.Fatalf("%+v", got)
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint(1, "line1\nline2\n")
	b := Fingerprint(1, "line1\nline2\n")
	c := Fingerprint(2, "line1\nline2\n")
	if a != b || a == c {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
}

func TestGroupByFingerprint(t *testing.T) {
	now := time.Now().UTC()
	evs := []Event{
		{At: now, Kind: KindFingerprint, Label: "x", Hash: "abc", Detail: "boom"},
		{At: now.Add(time.Minute), Kind: KindFingerprint, Label: "x", Hash: "abc"},
		{At: now, Kind: KindStatus, Label: "x"},
	}
	g := GroupByFingerprint(evs)
	if len(g) != 1 || g[0].Count != 2 {
		t.Fatalf("%+v", g)
	}
}

func TestMissingFile(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "missing.jsonl"))
	got, err := s.Since(time.Now().Add(-time.Hour))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v err %v", got, err)
	}
	_ = os.RemoveAll(t.TempDir())
}
