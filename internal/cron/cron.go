// Package cron parses the user crontab for read-only companion rows (Phase 3).
package cron

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"
)

// Entry is one crontab line that matched a declared pattern.
type Entry struct {
	Schedule string
	Command  string
	Status   string // "scheduled" heuristic
	Source   string // always "cron"
	Label    string // synthetic label
}

// MatchCommand reports whether command contains any of the substrings.
func MatchCommand(command string, patterns []string) bool {
	if command == "" || len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" && strings.Contains(command, p) {
			return true
		}
	}
	return false
}

// ParseLines parses crontab -l style text into matching companion entries.
func ParseLines(text string, patterns []string) []Entry {
	var out []Entry
	sc := bufio.NewScanner(strings.NewReader(text))
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		sched := strings.Join(fields[:5], " ")
		cmd := strings.Join(fields[5:], " ")
		if !MatchCommand(cmd, patterns) {
			continue
		}
		n++
		out = append(out, Entry{
			Schedule: sched,
			Command:  cmd,
			Status:   "scheduled",
			Source:   "cron",
			Label:    "cron.companion." + sanitize(cmd, n),
		})
	}
	return out
}

func sanitize(cmd string, n int) string {
	s := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '.'
		}
	}, cmd)
	if len(s) > 40 {
		s = s[:40]
	}
	return strings.Trim(s, ".") + "." + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// LoadUserCrontab runs `crontab -l` (best-effort; empty on error).
func LoadUserCrontab() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "crontab", "-l")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}
