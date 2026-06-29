package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/raphapr/pimux/internal/agent"
)

func seed() Model {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	m = mm.(Model)
	mm, _ = m.Update(agentsMsg{agents: []agent.Agent{
		{PaneID: "%1", Session: "alpha", Window: 1, WindowName: "main", State: agent.Working, Project: "alpha", Model: "sonnet", Cmd: "pi", Msg: "running", TS: 2000, Path: "/repos/alpha"},
		{PaneID: "%2", Session: "beta", Window: 1, WindowName: "review", State: agent.Blocked, Project: "beta", Model: "opus", Cmd: "pi", Msg: "approve", TS: 3000, Path: "/repos/beta"},
		{PaneID: "%3", Session: "alpha", Window: 2, WindowName: "tests", State: agent.Done, Project: "alpha", Model: "sonnet", Cmd: "pi", Msg: "done", TS: 1000, Path: "/repos/alpha"},
	}})
	return mm.(Model)
}

func TestViewRendersSideBySideSessionGroupedList(t *testing.T) {
	m := seed()
	out := m.View()
	for _, want := range []string{"search>", "beta", "alpha", "main", "tests", "session content", "grouped"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view missing %q\n%s", want, out)
		}
	}
}

func TestGenericWindowNameFallsBackToIndex(t *testing.T) {
	m := seed()
	m.agents = []agent.Agent{
		{PaneID: "%1", Session: "gamma", Window: 1, WindowName: "Window", State: agent.Working, Project: "gamma", Model: "sonnet", Cmd: "pi", TS: 2000},
		{PaneID: "%2", Session: "gamma", Window: 2, WindowName: "Window", State: agent.Idle, Project: "gamma", Model: "sonnet", Cmd: "pi", TS: 1000},
	}
	out := m.View()
	for _, want := range []string{" 1 ", " 2 "} {
		if !strings.Contains(out, want) {
			t.Fatalf("view missing generic-name fallback %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "Window") {
		t.Fatalf("generic tmux window name should not render\n%s", out)
	}
}

func TestLayoutUsesCompactSidebarAndWidePreview(t *testing.T) {
	m := seed()
	leftW, rightW := m.layoutWidths()
	if leftW != 32 {
		t.Fatalf("left width = %d, want compact 32", leftW)
	}
	if rightW != 68 {
		t.Fatalf("right width = %d, want remaining 68", rightW)
	}
	for i, line := range strings.Split(strings.TrimRight(m.renderSidebar(leftW), "\n"), "\n") {
		if got := lipgloss.Width(line); got != leftW {
			t.Fatalf("sidebar line %d width = %d, want %d; line=%q", i, got, leftW, line)
		}
	}
}

func TestSortCycleAndMostRecentDefault(t *testing.T) {
	m := seed()
	if m.sortMode != agent.Grouped {
		t.Fatalf("default sort = %s, want grouped", m.sortMode)
	}
	if first, _ := m.selected(); first.PaneID != "%2" {
		t.Fatalf("grouped mode should start at most recent session beta, got %+v", first)
	}

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mm.(Model)
	if m.sortMode != agent.PriorityMode {
		t.Fatalf("tab should switch to priority, got %s", m.sortMode)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mm.(Model)
	if m.sortMode != agent.Recent {
		t.Fatalf("second tab should switch to recent, got %s", m.sortMode)
	}
}

func TestAlwaysActiveFuzzyFilterAndEscClearQuit(t *testing.T) {
	m := seed()
	for _, r := range "alpha" {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}
	if m.query != "alpha" {
		t.Fatalf("query = %q, want alpha", m.query)
	}
	if got := len(m.visibleAgents()); got != 2 {
		t.Fatalf("alpha should match two alpha panes, got %d", got)
	}
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(Model)
	if cmd != nil {
		t.Fatal("esc with a query should clear, not quit")
	}
	if m.query != "" {
		t.Fatalf("esc should clear query, got %q", m.query)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc with empty query should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("esc with empty query should produce tea.QuitMsg")
	}
}

func TestNonModalVimChordNavigation(t *testing.T) {
	m := seed()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = mm.(Model)
	if m.cursor != 1 {
		t.Fatalf("ctrl+j should move down, got cursor %d", m.cursor)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = mm.(Model)
	if m.cursor != 0 {
		t.Fatalf("ctrl+k should move up, got cursor %d", m.cursor)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = mm.(Model)
	if m.query != "j" || m.cursor != 0 {
		t.Fatalf("plain j should type into query, got query=%q cursor=%d", m.query, m.cursor)
	}
}

func TestMarkSeenAndInterruptConfirmGate(t *testing.T) {
	m := seed()
	m.cursor = 2 // done alpha tests
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("ctrl+d should return mark-seen command")
	}

	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = mm.(Model)
	if !m.confirmKill {
		t.Fatal("ctrl+x should arm confirm-kill")
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = mm.(Model)
	if m.confirmKill {
		t.Fatal("n should cancel confirm-kill")
	}
}

func TestEmptyState(t *testing.T) {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(Model)
	out := m.View()
	if !strings.Contains(out, "No pi agents") {
		t.Fatalf("empty view should hint how to start; got:\n%s", out)
	}
}
