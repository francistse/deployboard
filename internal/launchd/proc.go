package launchd

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProcessInfo is what `ps` can tell us about a live job's process.
type ProcessInfo struct {
	Elapsed time.Duration // how long the process has been running
	Since   time.Time     // when it started, derived from Elapsed
}

// ProcessLookup is the injectable `ps` call (nil disables uptime enrichment).
type ProcessLookup func(pid int) (ProcessInfo, bool)

// parseElapsed parses the elapsed-time field of `ps -o etime=`.
//
// Formats seen in the wild: "MM:SS", "HH:MM:SS", "D-HH:MM:SS". A pid that
// exited between the listing and the call simply fails to parse, which is fine:
// uptime is decoration, not correctness.
func parseElapsed(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	days := 0
	if i := strings.IndexByte(s, '-'); i > 0 {
		d, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, false
		}
		days = d
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	nums := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0, false
		}
		nums = append(nums, n)
	}
	var h, m, sec int
	switch len(nums) {
	case 2: // MM:SS
		m, sec = nums[0], nums[1]
	case 3: // HH:MM:SS
		h, m, sec = nums[0], nums[1], nums[2]
	}
	return time.Duration(days)*24*time.Hour +
		time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second, true
}

// runPS is the default lookup: one `ps` per running job, memoised by the caller.
func runPS(pid int) (ProcessInfo, bool) {
	if pid <= 0 {
		return ProcessInfo{}, false
	}
	res, err := runCmd("ps", "-o", "etime=", "-p", strconv.Itoa(pid))
	if err != nil || res == nil {
		return ProcessInfo{}, false
	}
	elapsed, ok := parseElapsed(res.Stdout)
	if !ok {
		return ProcessInfo{}, false
	}
	return ProcessInfo{Elapsed: elapsed, Since: time.Now().Add(-elapsed)}, true
}

// PSLookup is the default ProcessLookup: one `ps` call per pid, cached by the
// service. Exported so the runtime can opt in without knowing the internals.
func PSLookup(pid int) (ProcessInfo, bool) { return runPS(pid) }

// processCache memoises a pid's uptime for a short TTL so a UI poll does not
// fork `ps` per running job on every refresh.
type processCache struct {
	mu      sync.Mutex
	entries map[int]processCacheEntry
}

type processCacheEntry struct {
	info ProcessInfo
	ok   bool
	at   time.Time
}

func (c *processCache) get(pid int, now time.Time, ttl time.Duration, lookup ProcessLookup) (ProcessInfo, bool) {
	if lookup == nil || pid <= 0 {
		return ProcessInfo{}, false
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[int]processCacheEntry{}
	}
	if e, ok := c.entries[pid]; ok && now.Sub(e.at) < ttl {
		c.mu.Unlock()
		return e.info, e.ok
	}
	c.mu.Unlock()

	info, ok := lookup(pid)

	c.mu.Lock()
	c.entries[pid] = processCacheEntry{info: info, ok: ok, at: now}
	// Keep the map from growing without bound on a machine with many pids.
	if len(c.entries) > 512 {
		for k := range c.entries {
			if now.Sub(c.entries[k].at) > ttl {
				delete(c.entries, k)
			}
		}
	}
	c.mu.Unlock()
	return info, ok
}

// invalidate drops one pid, so an action is reflected immediately.
func (c *processCache) invalidate(pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, pid)
}
