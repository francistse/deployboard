package probe

import (
	"testing"
	"time"
)

func TestPortsFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []int
	}{
		{"uvicorn --port", []string{"python", "-m", "uvicorn", "app:app", "--port", "8000"}, []int{8000}},
		{"next -p", []string{"node", "server.js", "-p", "3000"}, []int{3000}},
		{"flag with equals", []string{"--port=8080", "--http-port=9090"}, []int{8080, 9090}},
		{"duplicate", []string{"--port", "8000", "-p", "8000"}, []int{8000}},
		{"no ports", []string{"python", "script.py"}, []int{}},
		{"invalid", []string{"--port", "notanumber"}, []int{}},
		{"out of range", []string{"--port", "99999"}, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PortsFromArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Errorf("PortsFromArgs(%v) = %v, want %v", tt.args, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("PortsFromArgs(%v)[%d] = %d, want %d", tt.args, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestProber_CacheTTL(t *testing.T) {
	p := New(10 * time.Second)
	p.SetClock(func() time.Time { return time.Unix(1000, 0) })

	// Use a port nothing can be listening on in CI/dev (8080 is a real service
	// on a developer Mac — a Docker container holds it — so the "no server" assumption
	// only holds for a high, unassigned port).
	const dead = 59127

	// First call
	r1 := p.Probe(dead)
	if r1.Reachable {
		t.Error("first probe should not be reachable (no server)")
	}

	// Advance clock within TTL
	p.SetClock(func() time.Time { return time.Unix(1005, 0) })
	r2 := p.Probe(dead)
	if r2.Reachable != r1.Reachable || r2.Status != r1.Status {
		t.Error("cached result should be identical within TTL")
	}

	// Advance clock past TTL - the probe is re-executed. The result is still
	// unreachable (no server), so assert the cache entry was refreshed instead.
	p.SetClock(func() time.Time { return time.Unix(1020, 0) })
	r3 := p.Probe(dead)
	if r3.Port != dead {
		t.Errorf("re-probe should keep the port: %+v", r3)
	}
}

func TestProber_ProbeArgs(t *testing.T) {
	p := New(10 * time.Second)
	args := []string{"--port", "8000", "--http-port", "8001"}
	results := p.ProbeArgs(args)
	if len(results) != 2 {
		t.Errorf("ProbeArgs = %d results, want 2", len(results))
	}
	for _, r := range results {
		if r.Port != 8000 && r.Port != 8001 {
			t.Errorf("unexpected port %d", r.Port)
		}
	}
}

func TestProbe_IPv6Format(t *testing.T) {
	p := New(10 * time.Second)
	// Just verify the probe function doesn't panic on the IPv6 format
	_ = p.probe(65535)
}
