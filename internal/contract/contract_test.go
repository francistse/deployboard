package contract

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUnderRoots(t *testing.T) {
	root := filepath.Clean("/tmp/projects")
	ok := underRoots(filepath.Join(root, "app", "check.sh"), []string{root})
	if !ok {
		t.Fatal("expected under root")
	}
	if underRoots("/etc/passwd", []string{root}) {
		t.Fatal("must reject escape")
	}
}

func TestEvaluate_SkipsNonOurs(t *testing.T) {
	c := Config{{
		Group: "G",
		HTTP:  &HTTPSpec{Port: 1, Path: "/", ExpectStatus: 200},
	}}
	jobs := []JobView{{Label: "x", Category: "noise", Group: "G"}}
	if got := c.Evaluate(context.Background(), jobs, nil, nil); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestEvaluate_GroupMatch(t *testing.T) {
	c := Config{{
		Group: "App",
		TCP:   &TCPSpec{Port: 1},
	}}
	jobs := []JobView{{Label: "com.app.web", Category: "ours", Group: "App"}}
	got := c.Evaluate(context.Background(), jobs, nil, nil)
	if len(got) != 1 || got[0].Kind != "tcp" || got[0].OK {
		t.Fatalf("got %#v", got)
	}
}

func TestEvaluate_ExecJail(t *testing.T) {
	c := Config{{
		Match: "com.app.*",
		Exec:  &ExecSpec{Command: []string{"/etc/passwd"}, TimeoutSeconds: 1},
	}}
	jobs := []JobView{{Label: "com.app.web", Category: "ours", Group: "App"}}
	got := c.Evaluate(context.Background(), jobs, []string{"/tmp/projects"}, nil)
	if len(got) != 1 || got[0].OK || got[0].Detail == "" {
		t.Fatalf("got %#v", got)
	}
}
