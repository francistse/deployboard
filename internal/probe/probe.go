// Package probe checks whether a launchd job's HTTP port is actually answering.
//
// launchd can report a job "running" while the process is wedged (a dev server
// that booted before its port was bindable, a FastAPI app stuck on a migration).
// The probe is the difference between "launchd thinks it's up" and "a request
// succeeds".
package probe

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is the outcome of probing one port.
type Result struct {
	Port      int
	Reachable bool
	Status    int
	Latency   time.Duration
}

// Prober probes ports with a TTL cache so a UI poll or a /metrics scrape does
// not hammer the services.
type Prober struct {
	Client *http.Client
	TTL    time.Duration

	nowFn func() time.Time

	mu    sync.Mutex
	cache map[int]cacheEntry
}

type cacheEntry struct {
	at  time.Time
	res Result
}

// New builds a Prober with a 3-second per-request timeout.
func New(ttl time.Duration) *Prober {
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	return &Prober{
		Client: &http.Client{Timeout: 3 * time.Second},
		TTL:    ttl,
		nowFn:  time.Now,
		cache:  map[int]cacheEntry{},
	}
}

// SetClock overrides the clock (tests).
func (p *Prober) SetClock(fn func() time.Time) { p.nowFn = fn }

// PortsFromArgs extracts TCP ports from a ProgramArguments slice.
// Recognises `--port N`, `-p N`, `--port=N`, `-p=N` and `--http-port N`.
func PortsFromArgs(args []string) []int {
	var out []int
	seen := map[int]bool{}
	add := func(v string) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 || n > 65535 {
			return
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	flags := map[string]bool{"--port": true, "-p": true, "--http-port": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flags[arg] && i+1 < len(args) {
			add(args[i+1])
			continue
		}
		for _, f := range []string{"--port=", "-p=", "--http-port="} {
			if strings.HasPrefix(arg, f) {
				add(strings.TrimPrefix(arg, f))
			}
		}
	}
	return out
}

// Probe returns the cached-or-fresh result for one port.
func (p *Prober) Probe(port int) Result {
	p.mu.Lock()
	if e, ok := p.cache[port]; ok && p.nowFn().Sub(e.at) < p.TTL {
		p.mu.Unlock()
		return e.res
	}
	p.mu.Unlock()

	res := p.probe(port)

	p.mu.Lock()
	p.cache[port] = cacheEntry{at: p.nowFn(), res: res}
	p.mu.Unlock()
	return res
}

// probe tries IPv4 then IPv6 loopback. Any HTTP status below 500 counts as
// reachable — a 404 on `/` is normal for FastAPI, and 401/403 means the service
// is up but guarded.
func (p *Prober) probe(port int) Result {
	res := Result{Port: port}
	for _, host := range []string{"127.0.0.1", "[::1]"} {
		start := p.nowFn()
		req, err := http.NewRequest(http.MethodGet, "http://"+host+":"+strconv.Itoa(port)+"/", nil)
		if err != nil {
			continue
		}
		resp, err := p.Client.Do(req)
		latency := p.nowFn().Sub(start)
		if err != nil {
			continue
		}
		status := resp.StatusCode
		resp.Body.Close()
		res.Status = status
		res.Latency = latency
		res.Reachable = status < 500
		return res
	}
	return res
}

// ProbeArgs probes every port found in a ProgramArguments slice.
func (p *Prober) ProbeArgs(args []string) []Result {
	ports := PortsFromArgs(args)
	out := make([]Result, 0, len(ports))
	for _, port := range ports {
		out = append(out, p.Probe(port))
	}
	return out
}
