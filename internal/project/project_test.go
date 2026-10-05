package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/A404coder/deployboard/internal/desired"
	"github.com/A404coder/deployboard/internal/inventory"
)

func TestLoadJSONAndYAML(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "deployboard.json")
	if err := os.WriteFile(jsonPath, []byte(`{"ours":["com.demo.*"],"group":"Demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(jsonPath)
	if err != nil || len(f.Ours) != 1 || f.Group != "Demo" {
		t.Fatalf("%+v %v", f, err)
	}
	yamlPath := filepath.Join(dir, "deployboard.yaml")
	if err := os.WriteFile(yamlPath, []byte("ours:\n  - com.yaml.*\ngroup: Yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	yf, err := Load(yamlPath)
	if err != nil || yf.Group != "Yaml" {
		t.Fatalf("%+v %v", yf, err)
	}
}

func TestScanAndApplyWins(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "app")
	if err := os.Mkdir(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, "deployboard.yaml")
	body := "ours:\n  - com.proj.*\ngroup: Proj\ndesired:\n  jobs:\n    - match: com.proj.*\n      status: running\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 1 || len(m.Ours) != 1 {
		t.Fatalf("%+v", m)
	}
	base := inventory.Config{Ours: []string{"com.global.*"}, Groups: []inventory.Group{{Name: "G", Match: []string{"com.global.*"}}}}
	inv := ApplyInventory(base, m)
	if inv.Ours[0] != "com.proj.*" {
		t.Fatalf("project ours should prepend: %v", inv.Ours)
	}
	des := ApplyDesired(desired.Config{}, m)
	if len(des.Jobs) != 1 || des.Jobs[0].Match != "com.proj.*" {
		t.Fatalf("%+v", des)
	}
}
