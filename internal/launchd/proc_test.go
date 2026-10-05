package launchd

import (
	"testing"
	"time"
)

func TestParseElapsed(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"00:07", 7 * time.Second, true},
		{"03:12", 3*time.Minute + 12*time.Second, true},
		{"01:02:03", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"3-01:02:03", 3*24*time.Hour + time.Hour + 2*time.Minute + 3*time.Second, true},
		{"  12:34 ", 12*time.Minute + 34*time.Second, true},
		{"", 0, false},
		{"nope", 0, false},
		{"12", 0, false},
		{"1:2:3:4", 0, false},
	}
	for _, c := range cases {
		got, ok := parseElapsed(c.in)
		if ok != c.ok {
			t.Errorf("parseElapsed(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseElapsed(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestProcessCache_MemoisesAndInvalidates(t *testing.T) {
	calls := 0
	lookup := func(pid int) (ProcessInfo, bool) {
		calls++
		return ProcessInfo{Elapsed: time.Minute, Since: time.Unix(1700000000, 0)}, true
	}
	var c processCache
	now := time.Unix(1700000000, 0)

	if _, ok := c.get(42, now, 15*time.Second, lookup); !ok {
		t.Fatal("first lookup should succeed")
	}
	if _, ok := c.get(42, now.Add(time.Second), 15*time.Second, lookup); !ok {
		t.Fatal("second lookup inside the TTL should succeed")
	}
	if calls != 1 {
		t.Errorf("lookup ran %d times inside the TTL, want 1", calls)
	}

	c.get(42, now.Add(30*time.Second), 15*time.Second, lookup)
	if calls != 2 {
		t.Errorf("lookup ran %d times, want 2 after the TTL expired", calls)
	}

	c.invalidate(42)
	c.get(42, now.Add(31*time.Second), 15*time.Second, lookup)
	if calls != 3 {
		t.Errorf("lookup ran %d times, want 3 after invalidation", calls)
	}

	if _, ok := c.get(0, now, 15*time.Second, lookup); ok {
		t.Error("pid 0 has no process info")
	}
}
