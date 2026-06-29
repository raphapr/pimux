// Package agent models a pi agent discovered from tmux pane options and the
// pure parsing/sorting logic the TUI and --json mode share.
package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// State is the @pimux_state value written by the reporter extension.
type State string

const (
	Working State = "working"
	Blocked State = "blocked"
	Done    State = "done" // finished, unseen
	Idle    State = "idle"
)

// ListFormat is the -F format passed to `tmux list-panes -a`. Field order must
// match ParseLine below.
const ListFormat = "#{pane_id}|#{session_name}|#{window_index}|#{window_name}|#{pane_pid}|" +
	"#{pane_current_command}|#{pane_current_path}|#{@pimux_state}|#{@pimux_project}|" +
	"#{@pimux_model}|#{@pimux_msg}|#{@pimux_ts}|#{@pimux_session}"

const fieldCount = 13

// Agent is one pi pane and its reported state.
type Agent struct {
	PaneID      string `json:"pane_id"`
	Session     string `json:"session"` // tmux session name (the "project")
	Window      int    `json:"window"`
	WindowName  string `json:"window_name"`
	PID         int    `json:"pid"`
	Cmd         string `json:"cmd"`
	Path        string `json:"path"`
	State       State  `json:"state"`
	Project     string `json:"project"` // @pimux_project (cwd basename)
	Model       string `json:"model"`
	Msg         string `json:"msg"`
	TS          int64  `json:"ts"`
	SessionPath string `json:"session_path"` // @pimux_session (jsonl path)
	// Stale is true when the option still reports an agent but the foreground
	// process is no longer pi (crashed/exited without clearing options).
	Stale bool `json:"stale"`
}

// ParseLine parses one `tmux list-panes` line. ok is false when the line is not
// a pi agent (no @pimux_state) or is malformed.
func ParseLine(line string) (Agent, bool) {
	if line == "" {
		return Agent{}, false
	}
	f := strings.Split(line, "|")
	if len(f) != fieldCount {
		return Agent{}, false
	}
	state := strings.TrimSpace(f[7])
	if state == "" {
		return Agent{}, false // not a pi agent
	}
	a := Agent{
		PaneID:      f[0],
		Session:     f[1],
		Window:      atoi(f[2]),
		WindowName:  f[3],
		PID:         atoi(f[4]),
		Cmd:         f[5],
		Path:        f[6],
		State:       State(state),
		Project:     f[8],
		Model:       f[9],
		Msg:         f[10],
		TS:          atoi64(f[11]),
		SessionPath: f[12],
	}
	// Foreground command no longer pi => the reported state is stale.
	if a.Cmd != "pi" {
		a.Stale = true
	}
	return a, true
}

// Parse parses full `tmux list-panes` output into pi agents only.
func Parse(out string) []Agent {
	var agents []Agent
	for _, line := range strings.Split(out, "\n") {
		if a, ok := ParseLine(line); ok {
			agents = append(agents, a)
		}
	}
	return agents
}

// Priority orders states for display using herdr's attention order: blocked
// first (needs you), then done (finished/unseen), working, then idle. Stale
// sinks to the bottom.
func Priority(a Agent) int {
	if a.Stale {
		return 5
	}
	switch a.State {
	case Blocked:
		return 0
	case Done:
		return 1
	case Working:
		return 2
	case Idle:
		return 3
	default:
		return 4
	}
}

// Less reports whether a should sort before b.
func Less(a, b Agent) bool {
	if pa, pb := Priority(a), Priority(b); pa != pb {
		return pa < pb
	}
	if a.Session != b.Session {
		return a.Session < b.Session
	}
	return a.Window < b.Window
}

// Humanize renders a compact "time since ts" like "12s", "3m", "2h".
func Humanize(nowMS, tsMS int64) string {
	if tsMS <= 0 {
		return ""
	}
	d := (nowMS - tsMS) / 1000
	if d < 0 {
		d = 0
	}
	switch {
	case d < 60:
		return fmt.Sprintf("%ds", d)
	case d < 3600:
		return fmt.Sprintf("%dm", d/60)
	default:
		return fmt.Sprintf("%dh", d/3600)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
