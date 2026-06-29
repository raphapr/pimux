// Package ui implements the pimux Bubble Tea dashboard: a live, grouped list of
// pi agents across tmux sessions with a session-content preview and jump/seen /
// interrupt actions.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/raphapr/pimux/internal/agent"
	"github.com/raphapr/pimux/internal/session"
	"github.com/raphapr/pimux/internal/tmux"
)

type tickMsg time.Time
type agentsMsg struct {
	agents []agent.Agent
	err    error
}
type previewMsg struct {
	pane  string
	lines []session.Line
}

// Model is the dashboard state.
type Model struct {
	agents      []agent.Agent
	cursor      int
	width       int
	height      int
	query       string
	sortMode    agent.SortMode
	preview     []session.Line
	previewPane string
	confirmKill bool
	err         error
	pollEvery   time.Duration
}

// New returns a dashboard model with default polling.
func New() Model {
	return Model{pollEvery: time.Second, sortMode: agent.Grouped}
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

func previewCmd(a agent.Agent) tea.Cmd {
	if a.PaneID == "" || a.SessionPath == "" {
		return nil
	}
	return func() tea.Msg {
		return previewMsg{pane: a.PaneID, lines: session.TranscriptTail(a.SessionPath, 14)}
	}
}

func (m Model) visibleAgents() []agent.Agent {
	ordered := agent.Order(m.agents, m.sortMode)
	filtered, _ := agent.Filter(ordered, m.query)
	return filtered
}

func (m Model) selected() (agent.Agent, bool) {
	v := m.visibleAgents()
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
		return m, tea.Batch(loadCmd(), tickCmd(m.pollEvery), previewCmd(sel))

	case agentsMsg:
		m.err = msg.err
		m.agents = msg.agents
		m.clampCursor()
		sel, _ := m.selected()
		return m, previewCmd(sel)

	case previewMsg:
		sel, ok := m.selected()
		if ok && msg.pane == sel.PaneID {
			m.preview = msg.lines
			m.previewPane = msg.pane
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmKill {
		switch msg.String() {
		case "y":
			m.confirmKill = false
			if sel, ok := m.selected(); ok {
				return m, runOnceCmd(tmux.InterruptArgs(sel.PaneID))
			}
		case "n", "esc":
			m.confirmKill = false
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.query != "" {
			m.query = ""
			m.clampCursor()
			return m, nil
		}
		return m, tea.Quit
	case "ctrl+j", "ctrl+n", "down":
		m.move(1)
		return m, m.selectedPreviewCmd()
	case "ctrl+k", "ctrl+p", "up":
		m.move(-1)
		return m, m.selectedPreviewCmd()
	case "ctrl+u":
		m.query = ""
		m.clampCursor()
		return m, nil
	case "ctrl+w":
		m.deleteWord()
		m.clampCursor()
		return m, nil
	case "ctrl+d":
		if sel, ok := m.selected(); ok {
			return m, runCmd(tmux.MarkSeenArgs(sel.PaneID))
		}
	case "ctrl+x":
		if _, ok := m.selected(); ok {
			m.confirmKill = true
		}
	case "ctrl+r":
		return m, loadCmd()
	case "tab":
		m.cycleSort()
		m.clampCursor()
		return m, m.selectedPreviewCmd()
	case "enter":
		if sel, ok := m.selected(); ok {
			return m, tea.Sequence(runCmd(tmux.JumpArgs(sel)), tea.Quit)
		}
	case "backspace":
		m.query = dropLastRune(m.query)
		m.clampCursor()
		return m, nil
	case " ":
		m.query += " "
		m.clampCursor()
		return m, nil
	}

	if msg.Type == tea.KeyRunes {
		m.query += string(msg.Runes)
		m.clampCursor()
	}
	return m, nil
}

func (m *Model) clampCursor() {
	n := len(m.visibleAgents())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) move(delta int) {
	n := len(m.visibleAgents())
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

func (m *Model) cycleSort() {
	switch m.sortMode {
	case agent.Grouped:
		m.sortMode = agent.PriorityMode
	case agent.PriorityMode:
		m.sortMode = agent.Recent
	default:
		m.sortMode = agent.Grouped
	}
}

func (m Model) selectedPreviewCmd() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	return previewCmd(sel)
}

func (m *Model) deleteWord() {
	m.query = strings.TrimRight(m.query, " ")
	for m.query != "" {
		r, size := utf8.DecodeLastRuneInString(m.query)
		if r == utf8.RuneError && size == 0 {
			break
		}
		if r == ' ' {
			break
		}
		m.query = m.query[:len(m.query)-size]
	}
}

func dropLastRune(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	if size <= 0 {
		return ""
	}
	return s[:len(s)-size]
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
	mochaBase     = lipgloss.Color("#1e1e2e")
	mochaSurface0 = lipgloss.Color("#313244")
	mochaOverlay0 = lipgloss.Color("#6c7086")
	mochaOverlay1 = lipgloss.Color("#7f849c")
	mochaText     = lipgloss.Color("#cdd6f4")
	mochaRed      = lipgloss.Color("#f38ba8")
	mochaYellow   = lipgloss.Color("#f9e2af")
	mochaTeal     = lipgloss.Color("#94e2d5")
	mochaGreen    = lipgloss.Color("#a6e3a1")
	mochaMauve    = lipgloss.Color("#cba6f7")

	stWorking = lipgloss.NewStyle().Foreground(mochaYellow)
	stBlocked = lipgloss.NewStyle().Foreground(mochaRed)
	stDone    = lipgloss.NewStyle().Foreground(mochaTeal)
	stIdle    = lipgloss.NewStyle().Foreground(mochaGreen)
	stStale   = lipgloss.NewStyle().Foreground(mochaOverlay0).Strikethrough(true)
	stHeader  = lipgloss.NewStyle().Foreground(mochaOverlay0).Bold(true)
	stCursor  = lipgloss.NewStyle().Background(mochaSurface0).Foreground(mochaText)
	stDim     = lipgloss.NewStyle().Foreground(mochaOverlay0)
	stMuted   = lipgloss.NewStyle().Foreground(mochaOverlay1)
	stTitle   = lipgloss.NewStyle().Foreground(mochaText).Bold(true)
	stAccent  = lipgloss.NewStyle().Foreground(mochaMauve).Bold(true)
)

func dot(a agent.Agent) (string, lipgloss.Style) {
	if a.Stale {
		return "✗", stStale
	}
	switch a.State {
	case agent.Blocked:
		return "●", stBlocked
	case agent.Working:
		return "●", stWorking
	case agent.Done:
		return "●", stDone
	default:
		return "○", stIdle
	}
}

func stateLabel(a agent.Agent, now int64) string {
	if a.Stale {
		return "stale"
	}
	age := agent.Humanize(now, a.TS)
	base := string(a.State)
	if a.State == agent.Blocked && a.Msg != "" {
		base = "blocked · " + a.Msg
	} else if a.Msg != "" {
		base = string(a.State) + " · " + a.Msg
	}
	if age != "" {
		return base + " · " + age
	}
	return base
}

// View renders the dashboard.
func (m Model) View() string {
	v := m.visibleAgents()
	if len(v) == 0 {
		return m.emptyView()
	}

	leftW, rightW := m.layoutWidths()
	sidebar := m.renderSidebar(leftW)
	preview := m.renderPreview(rightW)
	if m.width > 0 && m.width < 70 {
		return sidebar + "\n" + stDim.Render(strings.Repeat("─", maxInt(10, m.width))) + "\n" + preview
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, preview)
}

func (m Model) emptyView() string {
	if strings.TrimSpace(m.query) != "" {
		return stTitle.Render("pimux") + "  " + stDim.Render("No pi agents match “"+m.query+"”.")
	}
	return stTitle.Render("pimux") + "  " + stDim.Render("No pi agents reporting. Start pi in a tmux pane.")
}

func (m Model) layoutWidths() (int, int) {
	w := m.width
	if w <= 0 {
		w = 100
	}
	left := w * 32 / 100
	if left < 26 {
		left = 26
	}
	if left > 34 {
		left = 34
	}
	right := w - left
	if right < 24 {
		right = 24
	}
	return left, right
}

func (m Model) renderSidebar(width int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render("pimux") + "  " + stDim.Render(fmt.Sprintf("%d agent(s)", len(m.visibleAgents()))) + "  " + stMuted.Render(string(m.sortMode)) + "\n")
	if m.err != nil {
		b.WriteString(stBlocked.Render("error: "+m.err.Error()) + "\n")
	}
	b.WriteString(stAccent.Render("search>") + " " + m.query + "▏\n")

	rows := m.displayRows(width)
	for _, row := range rows {
		line := row.text
		if row.selected {
			line = stCursor.Render(truncDisplay(strings.TrimRight(line, " "), width))
		}
		b.WriteString(line + "\n")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

type displayRow struct {
	text     string
	selected bool
}

func (m Model) displayRows(width int) []displayRow {
	v := m.visibleAgents()
	selected, _ := m.selected()
	if m.sortMode != agent.Grouped {
		rows := make([]displayRow, 0, len(v))
		for _, a := range v {
			rows = append(rows, displayRow{text: m.agentLine(a, a.Session, width, false), selected: a.PaneID == selected.PaneID})
		}
		return rows
	}

	var rows []displayRow
	for i := 0; i < len(v); {
		sessionName := v[i].Session
		j := i
		for j < len(v) && v[j].Session == sessionName {
			j++
		}
		group := v[i:j]
		if len(group) == 1 {
			a := group[0]
			rows = append(rows, displayRow{text: m.agentLine(a, a.Session, width, false), selected: a.PaneID == selected.PaneID})
		} else {
			roll := group[0]
			glyph, gs := dot(roll)
			rows = append(rows, displayRow{text: fmt.Sprintf(" %s %-18s %s", gs.Render(glyph), trunc(sessionName, 18), stDim.Render(fmt.Sprintf("%d agents", len(group))))})
			for _, a := range group {
				label := windowLabel(a)
				rows = append(rows, displayRow{text: m.agentLine(a, label, width, true), selected: a.PaneID == selected.PaneID})
			}
		}
		i = j
	}
	return rows
}

func windowLabel(a agent.Agent) string {
	if a.WindowName == "" || a.WindowName == "Window" {
		return fmt.Sprintf("%d", a.Window)
	}
	return a.WindowName
}

func (m Model) agentLine(a agent.Agent, label string, width int, child bool) string {
	glyph, gs := dot(a)
	prefix := " "
	if child {
		prefix = "   "
	}
	state := string(a.State)
	if a.Stale {
		state = "stale"
	}
	line := fmt.Sprintf("%s%s %-16s %s", prefix, gs.Render(glyph), trunc(label, 16), stDim.Render(state))
	return truncDisplay(line, width)
}

func (m Model) renderPreview(width int) string {
	sel, ok := m.selected()
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString(stAccent.Render(sel.Project) + " · " + stTitle.Render(sel.Model) + "\n")
	b.WriteString(stDim.Render(sel.PaneID+" · "+sel.Path) + "\n")
	b.WriteString(m.stateLine(sel) + "\n")
	b.WriteString(stDim.Render(strings.Repeat("─", maxInt(10, width))) + "\n")
	b.WriteString(stHeader.Render("session content") + "\n")
	if sel.PaneID == m.previewPane && len(m.preview) > 0 {
		for _, line := range m.preview {
			b.WriteString(renderTranscriptLine(line, width) + "\n")
		}
	} else {
		b.WriteString(stDim.Render("No session content loaded yet.") + "\n")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) stateLine(a agent.Agent) string {
	glyph, gs := dot(a)
	return gs.Render(glyph) + " " + stateLabel(a, time.Now().UnixMilli())
}

func renderTranscriptLine(line session.Line, width int) string {
	role := line.Role
	if role == "toolResult" || role == "bashExecution" {
		role = "tool"
	}
	return truncDisplay(stDim.Render("▸ ")+stMuted.Render(fmt.Sprintf("%-9s", role))+" "+line.Text, width)
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

func truncDisplay(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	plain := []rune(s)
	if len(plain) <= n {
		return s
	}
	return string(plain[:maxInt(1, n-1)]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
