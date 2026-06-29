// Package tmux wraps the tmux CLI: discovery, pane capture, and the user
// actions (jump, mark-seen, interrupt). Command-building is split from
// execution so the argument vectors can be unit-tested.
package tmux

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"github.com/raphapr/pimux/internal/agent"
)

// ListArgs builds `tmux list-panes -a -F <format>`.
func ListArgs() []string {
	return []string{"list-panes", "-a", "-F", agent.ListFormat}
}

// CaptureArgs builds `tmux capture-pane -p -t <pane> -S -<lines>` (visible tail).
func CaptureArgs(paneID string, lines int) []string {
	if lines <= 0 {
		lines = 40
	}
	return []string{"capture-pane", "-p", "-t", paneID, "-S", "-" + strconv.Itoa(lines)}
}

// JumpArgs builds the command vectors to focus an agent's pane: switch the
// attached client to its session, select its window, then its pane.
func JumpArgs(a agent.Agent) [][]string {
	target := a.Session + ":" + strconv.Itoa(a.Window)
	return [][]string{
		{"switch-client", "-t", a.Session},
		{"select-window", "-t", target},
		{"select-pane", "-t", a.PaneID},
	}
}

// MarkSeenArgs clears the blue "done" marker on a pane (done -> idle).
func MarkSeenArgs(paneID string) [][]string {
	return [][]string{
		{"set-option", "-p", "-t", paneID, "@pimux_state", "idle"},
		{"set-option", "-w", "-t", paneID, "@pimux_win_state", "idle"},
	}
}

// InterruptArgs sends Ctrl-C to a pane (used only behind a confirm prompt).
func InterruptArgs(paneID string) []string {
	return []string{"send-keys", "-t", paneID, "C-c"}
}

// List runs tmux and returns parsed pi agents.
func List(ctx context.Context) ([]agent.Agent, error) {
	out, err := run(ctx, ListArgs())
	if err != nil {
		return nil, err
	}
	return agent.Parse(out), nil
}

// Capture returns the visible tail of a pane, or "" on error.
func Capture(ctx context.Context, paneID string, lines int) string {
	out, err := run(ctx, CaptureArgs(paneID, lines))
	if err != nil {
		return ""
	}
	return out
}

// Run executes a sequence of tmux command vectors, stopping at the first error.
func Run(ctx context.Context, cmds [][]string) error {
	for _, c := range cmds {
		if _, err := run(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// RunOne executes a single tmux command vector.
func RunOne(ctx context.Context, args []string) error {
	_, err := run(ctx, args)
	return err
}

func run(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}
