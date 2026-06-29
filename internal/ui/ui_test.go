package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/raphapr/pimux/internal/agent"
)

func seed() Model {
	m := New()
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(Model)
	mm, _ = m.Update(agentsMsg{agents: []agent.Agent{
		{PaneID: "%1", Session: "ls-n8n", Window: 1, State: agent.Done, Project: "ls-n8n", Cmd: "pi"},
		{PaneID: "%2", Session: "rfp", Window: 1, State: agent.Blocked, Project: "rfp", Cmd: "pi", Msg: "approve"},
	}})
	return mm.(Model)
}

func TestViewRendersAndSorts(t *testing.T) {
	m := seed()
	if len(m.visible()) != 2 {
		t.Fatalf("want 2 visible, got %d", len(m.visible()))
	}
	// blocked sorts first
	if first, _ := m.selected(); first.State != agent.Blocked {
		t.Fatalf("blocked agent should be first, got %s", first.State)
	}
	out := m.View()
	for _, want := range []string{"pimux", "rfp", "ls-n8n", "jump"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view missing %q\n%s", want, out)
		}
	}
}

func TestCursorNavigation(t *testing.T) {
	m := seed()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = mm.(Model)
	if m.cursor != 1 {
		t.Fatalf("cursor should move to 1, got %d", m.cursor)
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = mm.(Model)
	if m.cursor != 0 {
		t.Fatalf("cursor should move back to 0, got %d", m.cursor)
	}
}

func TestFilter(t *testing.T) {
	m := seed()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = mm.(Model)
	if !m.filtering {
		t.Fatal("/ should enter filter mode")
	}
	for _, r := range "rfp" {
		mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mm.(Model)
	}
	if len(m.visible()) != 1 {
		t.Fatalf("filter 'rfp' should show 1 agent, got %d", len(m.visible()))
	}
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(Model)
	if m.filtering || m.filter != "" {
		t.Fatal("esc should clear filter")
	}
	if len(m.visible()) != 2 {
		t.Fatal("clearing filter should restore all agents")
	}
}

func TestQuitKeyEmitsQuit(t *testing.T) {
	m := seed()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should produce tea.QuitMsg")
	}
}

func TestInterruptConfirmGate(t *testing.T) {
	m := seed()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = mm.(Model)
	if !m.confirmKill {
		t.Fatal("x should arm the confirm-kill gate")
	}
	// Anything other than 'y' cancels without action.
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = mm.(Model)
	if m.confirmKill {
		t.Fatal("n should cancel the confirm-kill gate")
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
