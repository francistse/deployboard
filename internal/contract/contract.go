// Package contract evaluates health contracts for Ours groups/labels (Phase 2).
package contract

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/A404coder/deployboard/internal/inventory"
)

// Spec is one health contract from config.json "contracts".
type Spec struct {
	Group string     `json:"group,omitempty"`
	Match string     `json:"match,omitempty"`
	HTTP  *HTTPSpec  `json:"http,omitempty"`
	TCP   *TCPSpec   `json:"tcp,omitempty"`
	Exec  *ExecSpec  `json:"exec,omitempty"`
}

// HTTPSpec checks an HTTP path on a port.
type HTTPSpec struct {
	Port         int    `json:"port"`
	Path         string `json:"path"`
	ExpectStatus int    `json:"expect_status"`
}

// TCPSpec checks a TCP connect.
type TCPSpec struct {
	Port int `json:"port"`
}

// ExecSpec runs a user script (must resolve under derive_roots).
type ExecSpec struct {
	Command        []string `json:"command"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// Result is one contract evaluation.
type Result struct {
	Group   string `json:"group,omitempty"`
	Match   string `json:"match,omitempty"`
	Kind    string `json:"kind"` // http | tcp | exec
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Latency float64 `json:"latencySeconds,omitempty"`
}

// Config is the top-level contracts list.
type Config []Spec

// JobView is enough job info to decide which contracts apply.
type JobView struct {
	Label    string
	Category string
	Group    string
}

// Evaluate runs all applicable contracts for the given Ours jobs.
func (c Config) Evaluate(ctx context.Context, jobs []JobView, deriveRoots []string, client *http.Client) []Result {
	if len(c) == 0 {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	roots := expandRoots(deriveRoots)
	out := make([]Result, 0)
	seen := map[string]bool{} // dedupe identical group/match+kind

	oursGroups := map[string]bool{}
	oursLabels := map[string]bool{}
	for _, j := range jobs {
		if j.Category != string(inventory.CategoryOurs) {
			continue
		}
		oursLabels[j.Label] = true
		if j.Group != "" {
			oursGroups[j.Group] = true
		}
	}

	for _, spec := range c {
		if spec.Group != "" && !oursGroups[spec.Group] {
			continue
		}
		if spec.Match != "" {
			matched := false
			for label := range oursLabels {
				if inventory.Match([]string{spec.Match}, label) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if spec.Group == "" && spec.Match == "" {
			continue
		}
		results := runSpec(ctx, spec, roots, client)
		for _, r := range results {
			key := r.Group + "|" + r.Match + "|" + r.Kind
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].Group != out[k].Group {
			return out[i].Group < out[k].Group
		}
		if out[i].Match != out[k].Match {
			return out[i].Match < out[k].Match
		}
		return out[i].Kind < out[k].Kind
	})
	return out
}

func runSpec(ctx context.Context, spec Spec, roots []string, client *http.Client) []Result {
	var out []Result
	base := Result{Group: spec.Group, Match: spec.Match}
	if spec.HTTP != nil {
		r := base
		r.Kind = "http"
		r.OK, r.Detail, r.Latency = checkHTTP(ctx, client, *spec.HTTP)
		out = append(out, r)
	}
	if spec.TCP != nil {
		r := base
		r.Kind = "tcp"
		r.OK, r.Detail, r.Latency = checkTCP(ctx, *spec.TCP)
		out = append(out, r)
	}
	if spec.Exec != nil {
		r := base
		r.Kind = "exec"
		r.OK, r.Detail, r.Latency = checkExec(ctx, *spec.Exec, roots)
		out = append(out, r)
	}
	return out
}

func checkHTTP(ctx context.Context, client *http.Client, s HTTPSpec) (bool, string, float64) {
	path := s.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	expect := s.ExpectStatus
	if expect == 0 {
		expect = 200
	}
	start := time.Now()
	url := "http://127.0.0.1:" + strconv.Itoa(s.Port) + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error(), 0
	}
	resp, err := client.Do(req)
	lat := time.Since(start).Seconds()
	if err != nil {
		// try IPv6
		url6 := "http://[::1]:" + strconv.Itoa(s.Port) + path
		req6, err6 := http.NewRequestWithContext(ctx, http.MethodGet, url6, nil)
		if err6 != nil {
			return false, err.Error(), lat
		}
		resp, err = client.Do(req6)
		lat = time.Since(start).Seconds()
		if err != nil {
			return false, err.Error(), lat
		}
	}
	defer resp.Body.Close()
	ok := resp.StatusCode == expect
	return ok, fmt.Sprintf("status=%d expect=%d", resp.StatusCode, expect), lat
}

func checkTCP(ctx context.Context, s TCPSpec) (bool, string, float64) {
	start := time.Now()
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port)))
	lat := time.Since(start).Seconds()
	if err != nil {
		conn, err = d.DialContext(ctx, "tcp", net.JoinHostPort("::1", strconv.Itoa(s.Port)))
		lat = time.Since(start).Seconds()
		if err != nil {
			return false, err.Error(), lat
		}
	}
	_ = conn.Close()
	return true, "connected", lat
}

func checkExec(ctx context.Context, s ExecSpec, roots []string) (bool, string, float64) {
	if len(s.Command) == 0 {
		return false, "empty command", 0
	}
	bin := s.Command[0]
	abs, err := filepath.Abs(bin)
	if err != nil {
		return false, err.Error(), 0
	}
	if !underRoots(abs, roots) {
		return false, "exec path escapes derive_roots", 0
	}
	timeout := time.Duration(s.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(cctx, abs, s.Command[1:]...)
	cmd.Dir = filepath.Dir(abs)
	out, err := cmd.CombinedOutput()
	lat := time.Since(start).Seconds()
	detail := strings.TrimSpace(string(out))
	if len(detail) > 200 {
		detail = detail[:200]
	}
	if err != nil {
		if detail == "" {
			detail = err.Error()
		}
		return false, detail, lat
	}
	return true, detail, lat
}

func expandRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	home, _ := os.UserHomeDir()
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.HasPrefix(r, "~/") && home != "" {
			r = filepath.Join(home, r[2:])
		}
		abs, err := filepath.Abs(r)
		if err == nil {
			out = append(out, abs)
		}
	}
	return out
}

func underRoots(abs string, roots []string) bool {
	abs = filepath.Clean(abs)
	for _, root := range roots {
		root = filepath.Clean(root)
		if abs == root || strings.HasPrefix(abs, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
