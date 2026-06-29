// Package ui implements the pimux Bubble Tea dashboard: a live, grouped list of
// pi agents across tmux sessions with a pane preview and jump/seen/interrupt
// actions.
package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/raphapr/pimux/internal/agent"
	"github.com/raphapr/pimux/internal/tmux"
)

type tickMsg time.Time
type agentsMsg struct {
	agents []agent.Agent
	err    error
}
type previewMsg struct {
	pane string
	body string
}

// Model is the dashboard state.
type Model struct {
	agents       []agent.Agent
	cursor       int
	width        int
	height       int
	preview      string
	previewPane  string
	filter       string
	filtering    bool
	confirmKill  bool
	err          error
	pollEvery    time.Duration
	captureLines int
}

// New returns a dashboard model with default polling.
func New() Model {
	return Model{pollEvery: time.Second, captureLines: 80}
}

// Init starts the first load and the poll ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCmd(), tickCmd(m.pollEvery))
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		a, err := tmux.List(ctx)
		return agentsMsg{agents: a, err: err}
	}
}

func previewCmd(pane string, lines int) tea.Cmd {
	if pane == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return previewMsg{pane: pane, body: tmux.Capture(ctx, pane, lines)}
	}
}

// visible returns the sorted, filtered agent list.
func (m Model) visible() []agent.Agent {
	out := make([]agent.Agent, 0, len(m.agents))
	f := strings.ToLower(strings.TrimSpace(m.filter))
	for _, a := range m.agents {
		if f != "" {
			hay := strings.ToLower(a.Session + " " + a.Project + " " + a.Msg + " " + string(a.State))
			if !strings.Contains(hay, f) {
				continue
			}
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return agent.Less(out[i], out[j]) })
	return out
}

func (m Model) selected() (agent.Agent, bool) {
	v := m.visible()
	if m.cursor < 0 || m.cursor >= len(v) {
		return agent.Agent{}, false
	}
	return v[m.cursor], true
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		sel, _ := m.selected()
		return m, tea.Batch(loadCmd(), tickCmd(m.pollEvery), previewCmd(sel.PaneID, m.captureLines))

	case agentsMsg:
		m.err = msg.err
		m.agents = msg.agents
		if n := len(m.visible()); m.cursor >= n {
			m.cursor = max(0, n-1)
		}
		return m, nil

	case previewMsg:
		sel, ok := m.selected()
		if ok && msg.pane == sel.PaneID {
			m.preview = msg.body
			m.previewPane = msg.pane
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Confirm-kill sub-state takes precedence.
	if m.confirmKill {
		if msg.String() == "y" {
			m.confirmKill = false
			if sel, ok := m.selected(); ok {
				return m, runOnceCmd(tmux.InterruptArgs(sel.PaneID))
			}
		}
		m.confirmKill = false
		return m, nil
	}

	if m.filtering {
		switch msg.Type {
		case tea.KeyEsc:
			m.filtering = false
			m.filter = ""
		case tea.KeyEnter:
			m.filtering = false
		case tea.KeyBackspace:
			if m.filter != "" {
				m.filter = m.filter[:len(m.filter)-1]
			}
		case tea.KeyRunes, tea.KeySpace:
			m.filter += string(msg.Runes)
		}
		if m.cursor >= len(m.visible()) {
			m.cursor = max(0, len(m.visible())-1)
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
		sel, _ := m.selected()
		return m, previewCmd(sel.PaneID, m.captureLines)
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		sel, _ := m.selected()
		return m, previewCmd(sel.PaneID, m.captureLines)
	case "/":
		m.filtering = true
		return m, nil
	case "r":
		return m, loadCmd()
	case "d":
		if sel, ok := m.selected(); ok {
			return m, runCmd(tmux.MarkSeenArgs(sel.PaneID))
		}
	case "x":
		if _, ok := m.selected(); ok {
			m.confirmKill = true
		}
	case "enter":
		if sel, ok := m.selected(); ok {
			return m, tea.Sequence(runCmd(tmux.JumpArgs(sel)), tea.Quit)
		}
	}
	return m, nil
}

func runCmd(cmds [][]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tmux.Run(ctx, cmds)
		return loadCmd()()
	}
}

func runOnceCmd(args []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tmux.RunOne(ctx, args)
		return loadCmd()()
	}
}

// --- rendering -----------------------------------------------------------

var (
	stWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	stBlocked = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	stDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	stIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray
	stStale   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Strikethrough(true)
	stHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	stCursor  = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))
	stDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stTitle   = lipgloss.NewStyle().Bold(true)
)

func dot(a agent.Agent) (string, lipgloss.Style) {
	if a.Stale {
		return "✗", stStale
	}
	switch a.State {
	case agent.Blocked:
		return "⚠", stBlocked
	case agent.Working:
		return "⚙", stWorking
	case agent.Done:
		return "✓", stDone
	default:
		return "○", stIdle
	}
}

func stateLabel(a agent.Agent, now int64) string {
	if a.Stale {
		return "stale (process gone)"
	}
	age := agent.Humanize(now, a.TS)
	base := string(a.State)
	if a.State == agent.Blocked && a.Msg != "" {
		base = "waiting: " + a.Msg
	} else if a.Msg != "" {
		base = string(a.State) + " · " + a.Msg
	}
	if age != "" {
		return base + " " + age
	}
	return base
}

// View renders the dashboard.
func (m Model) View() string {
	v := m.visible()
	now := time.Now().UnixMilli()

	var b strings.Builder
	title := stTitle.Render("pimux")
	b.WriteString(fmt.Sprintf("%s  %s\n", title, stDim.Render(fmt.Sprintf("%d agent(s)", len(v)))))
	if m.err != nil {
		b.WriteString(stBlocked.Render("error: "+m.err.Error()) + "\n")
	}
	b.WriteString("\n")

	if len(v) == 0 {
		b.WriteString(stDim.Render("No pi agents reporting. Start pi in a tmux pane.\n"))
	}

	lastSession := ""
	for i, a := range v {
		if a.Session != lastSession {
			b.WriteString(stHeader.Render(a.Session) + "\n")
			lastSession = a.Session
		}
		glyph, gs := dot(a)
		line := fmt.Sprintf("  %s %-14s %s", gs.Render(glyph), trunc(a.Project, 14), stateLabel(a, now))
		if i == m.cursor {
			line = stCursor.Render(trunc(strings.TrimRight(line, " "), maxInt(20, m.width-1)))
		}
		b.WriteString(line + "\n")
	}

	// Preview of the selected pane.
	if sel, ok := m.selected(); ok && m.preview != "" {
		b.WriteString("\n" + stDim.Render(strings.Repeat("─", maxInt(10, m.width-1))) + "\n")
		b.WriteString(stDim.Render("preview "+sel.Session+" "+sel.PaneID) + "\n")
		b.WriteString(previewTail(m.preview, m.previewHeight()) + "\n")
	}

	// Footer.
	b.WriteString("\n")
	if m.confirmKill {
		b.WriteString(stBlocked.Render("interrupt this agent (send C-c)? y/N"))
	} else if m.filtering {
		b.WriteString(stTitle.Render("filter: ") + m.filter + stDim.Render("  (enter=keep esc=clear)"))
	} else {
		b.WriteString(stDim.Render("enter=jump  j/k=move  /=filter  d=seen  x=interrupt  r=refresh  q=quit"))
	}
	return b.String()
}

func (m Model) previewHeight() int {
	if m.height <= 0 {
		return 8
	}
	h := m.height - len(m.visible()) - 8
	if h < 4 {
		h = 4
	}
	if h > 20 {
		h = 20
	}
	return h
}

func previewTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return stDim.Render(strings.Join(lines, "\n"))
}

func trunc(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
