// Package desired compares configured expectations for Ours jobs against live
// launchd state (Phase 1 — drift).
package desired

import (
	"path"
	"sort"

	"github.com/A404coder/deployboard/internal/inventory"
)

// Expected statuses a desired entry may require.
const (
	StatusRunning   = "running"
	StatusDisabled  = "disabled"
	StatusScheduled = "scheduled"
)

// Reason classifies why a job drifted.
type Reason string

const (
	ReasonStatus      Reason = "status"
	ReasonDisabled    Reason = "disabled"
	ReasonRestartRate Reason = "restart_rate"
	ReasonProbe       Reason = "probe"
)

// JobRule is a label-pattern expectation.
type JobRule struct {
	Match          string `json:"match"`
	Status         string `json:"status"`
	MaxRestartRate int    `json:"max_restart_rate,omitempty"`
	ProbePort      int    `json:"probe_port,omitempty"`
}

// GroupRule is a group-name expectation (used when no JobRule matches).
type GroupRule struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	MaxRestartRate int    `json:"max_restart_rate,omitempty"`
	ProbePort      int    `json:"probe_port,omitempty"`
}

// Config is the top-level "desired" block in config.json.
type Config struct {
	Jobs   []JobRule   `json:"jobs,omitempty"`
	Groups []GroupRule `json:"groups,omitempty"`
}

// Expectation is the resolved desired state for one job.
type Expectation struct {
	Status         string
	MaxRestartRate int
	ProbePort      int
	Source         string // "job" | "group"
}

// JobView is the subset of a launchd job needed for evaluation.
type JobView struct {
	Label          string
	Category       string
	Group          string
	Status         string
	Disabled       bool
	RestartsRecent int
	ProbePort      int
	ProbeOK        bool
	HasProbe       bool
}

// Drift is one Ours job whose live state does not match its expectation.
type Drift struct {
	Label          string   `json:"label"`
	Group          string   `json:"group,omitempty"`
	ExpectedStatus string   `json:"expectedStatus"`
	ActualStatus   string   `json:"actualStatus"`
	Disabled       bool     `json:"disabled"`
	Reasons        []Reason `json:"reasons"`
	AlignAction    string   `json:"alignAction,omitempty"`
	Source         string   `json:"source,omitempty"`
}

// Empty reports whether any desired rules are configured.
func (c Config) Empty() bool {
	return len(c.Jobs) == 0 && len(c.Groups) == 0
}

// matchLabel reports whether pattern matches label (path.Match + exact).
func matchLabel(pattern, label string) bool {
	if pattern == "" {
		return false
	}
	if pattern == label {
		return true
	}
	ok, err := path.Match(pattern, label)
	return err == nil && ok
}

// Resolve returns the expectation for a job, if any rule applies.
// Job rules win over group rules; first match in each list wins.
func (c Config) Resolve(label, group string) (Expectation, bool) {
	for _, r := range c.Jobs {
		if matchLabel(r.Match, label) {
			return Expectation{
				Status:         normalizeStatus(r.Status),
				MaxRestartRate: r.MaxRestartRate,
				ProbePort:      r.ProbePort,
				Source:         "job",
			}, true
		}
	}
	if group != "" {
		for _, r := range c.Groups {
			if r.Name == group {
				return Expectation{
					Status:         normalizeStatus(r.Status),
					MaxRestartRate: r.MaxRestartRate,
					ProbePort:      r.ProbePort,
					Source:         "group",
				}, true
			}
		}
	}
	return Expectation{}, false
}

func normalizeStatus(s string) string {
	switch s {
	case StatusRunning, StatusDisabled, StatusScheduled:
		return s
	default:
		return StatusRunning
	}
}

// Evaluate returns drifts for Ours jobs that have a desired expectation.
func (c Config) Evaluate(jobs []JobView) []Drift {
	if c.Empty() {
		return nil
	}
	out := make([]Drift, 0)
	for _, j := range jobs {
		if j.Category != string(inventory.CategoryOurs) {
			continue
		}
		exp, ok := c.Resolve(j.Label, j.Group)
		if !ok {
			continue
		}
		if d, drifted := compare(j, exp); drifted {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].Group != out[k].Group {
			return out[i].Group < out[k].Group
		}
		return out[i].Label < out[k].Label
	})
	return out
}

func compare(j JobView, exp Expectation) (Drift, bool) {
	d := Drift{
		Label:          j.Label,
		Group:          j.Group,
		ExpectedStatus: exp.Status,
		ActualStatus:   j.Status,
		Disabled:       j.Disabled,
		Source:         exp.Source,
		Reasons:        nil,
	}
	actualEffective := j.Status
	if j.Disabled && j.Status != "running" {
		actualEffective = StatusDisabled
	}

	switch exp.Status {
	case StatusRunning:
		if actualEffective != StatusRunning {
			d.Reasons = append(d.Reasons, ReasonStatus)
			if j.Disabled || actualEffective == StatusDisabled {
				d.Reasons = append(d.Reasons, ReasonDisabled)
			}
		}
	case StatusDisabled:
		if !j.Disabled && actualEffective != StatusDisabled {
			d.Reasons = append(d.Reasons, ReasonDisabled)
			d.Reasons = append(d.Reasons, ReasonStatus)
		}
	case StatusScheduled:
		if actualEffective != StatusScheduled && actualEffective != "completed" {
			// completed is acceptable for a just-finished scheduled job
			d.Reasons = append(d.Reasons, ReasonStatus)
		}
	}

	if exp.MaxRestartRate > 0 && j.RestartsRecent > exp.MaxRestartRate {
		d.Reasons = append(d.Reasons, ReasonRestartRate)
	}

	if exp.ProbePort > 0 {
		if !j.HasProbe || j.ProbePort != exp.ProbePort || !j.ProbeOK {
			// If live probe is a different port but we have a probe and it failed, still flag.
			if j.HasProbe && !j.ProbeOK {
				d.Reasons = append(d.Reasons, ReasonProbe)
			} else if !j.HasProbe || j.ProbePort == exp.ProbePort {
				d.Reasons = append(d.Reasons, ReasonProbe)
			} else if !j.ProbeOK {
				d.Reasons = append(d.Reasons, ReasonProbe)
			}
		}
	}

	if len(d.Reasons) == 0 {
		return Drift{}, false
	}
	d.AlignAction = suggestAlign(exp.Status, j)
	d.Reasons = uniqueReasons(d.Reasons)
	return d, true
}

func uniqueReasons(in []Reason) []Reason {
	seen := map[Reason]bool{}
	out := make([]Reason, 0, len(in))
	for _, r := range in {
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// suggestAlign picks the launchd action that moves toward the expectation.
func suggestAlign(expected string, j JobView) string {
	switch expected {
	case StatusDisabled:
		if !j.Disabled {
			return "disable"
		}
		return ""
	case StatusRunning:
		// Start enables a retired job, then bootstraps/kickstarts it.
		if j.Disabled || j.Status != StatusRunning {
			return "start"
		}
		return ""
	case StatusScheduled:
		if j.Disabled {
			return "enable"
		}
		return ""
	}
	return ""
}

// AlignActionFor returns the action for a label given current evaluation, or "".
func (c Config) AlignActionFor(jobs []JobView, label string) (string, *Drift, error) {
	drifts := c.Evaluate(jobs)
	for i := range drifts {
		if drifts[i].Label == label {
			return drifts[i].AlignAction, &drifts[i], nil
		}
	}
	return "", nil, nil
}

// DesiredUp reports 1 when the job has a desired status of running (for metrics).
func DesiredUp(exp Expectation) float64 {
	if exp.Status == StatusRunning {
		return 1
	}
	return 0
}
