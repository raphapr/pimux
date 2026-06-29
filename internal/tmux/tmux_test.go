package tmux

import (
	"strings"
	"testing"

	"github.com/raphapr/pimux/internal/agent"
)

func TestListArgs(t *testing.T) {
	a := ListArgs()
	if a[0] != "list-panes" || a[1] != "-a" || a[2] != "-F" {
		t.Fatalf("bad list args: %v", a)
	}
	if a[3] != agent.ListFormat {
		t.Fatal("list format mismatch")
	}
}

func TestCaptureArgsDefaultsLines(t *testing.T) {
	a := CaptureArgs("%5", 0)
	joined := strings.Join(a, " ")
	if !strings.Contains(joined, "capture-pane -p -t %5 -S -40") {
		t.Fatalf("bad capture args: %v", a)
	}
	b := CaptureArgs("%5", 80)
	if b[len(b)-1] != "-80" {
		t.Fatalf("want -80 got %v", b)
	}
}

func TestJumpArgs(t *testing.T) {
	ag := agent.Agent{PaneID: "%7", Session: "alpha", Window: 2}
	cmds := JumpArgs(ag)
	if len(cmds) != 3 {
		t.Fatalf("want 3 jump cmds, got %d", len(cmds))
	}
	if cmds[0][0] != "switch-client" || cmds[0][2] != "alpha" {
		t.Fatalf("bad switch-client: %v", cmds[0])
	}
	if cmds[1][0] != "select-window" || cmds[1][2] != "alpha:2" {
		t.Fatalf("bad select-window: %v", cmds[1])
	}
	if cmds[2][0] != "select-pane" || cmds[2][2] != "%7" {
		t.Fatalf("bad select-pane: %v", cmds[2])
	}
}

func TestMarkSeenArgs(t *testing.T) {
	cmds := MarkSeenArgs("%7")
	if len(cmds) != 2 {
		t.Fatalf("want 2 cmds, got %d", len(cmds))
	}
	if cmds[0][len(cmds[0])-1] != "idle" {
		t.Fatalf("mark-seen should set idle: %v", cmds[0])
	}
}

func TestInterruptArgs(t *testing.T) {
	a := InterruptArgs("%9")
	if a[0] != "send-keys" || a[2] != "%9" || a[len(a)-1] != "C-c" {
		t.Fatalf("bad interrupt args: %v", a)
	}
}
