package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type tickMsg struct{}
type snapMsg []Agent
type prMsg map[string]PR

type model struct {
	agents   []Agent
	prs      map[string]PR
	sel      int // index into agents (display order)
	w, h     int
	prBusy   bool
	prAt     time.Time
	rowAgent []int // rendered line -> agent index, -1 = none
}

const headH, footH = 2, 1

func main() {
	if _, err := tea.NewProgram(model{prs: map[string]PR{}}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func snapCmd() tea.Msg {
	as, err := snapshot()
	if err != nil {
		return snapMsg(nil)
	}
	var wg = make(chan struct{}, len(as))
	for i := range as {
		go func(a *Agent) { enrichFast(a); wg <- struct{}{} }(&as[i])
	}
	for range as {
		<-wg
	}
	sort.SliceStable(as, func(i, j int) bool {
		a, b := as[i], as[j]
		if a.WS != b.WS && a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if ra, rb := rank(a.Status), rank(b.Status); ra != rb {
			return ra < rb
		}
		return a.Name < b.Name
	})
	return snapMsg(as)
}

func rank(s string) int {
	switch s {
	case "blocked":
		return 0
	case "working":
		return 1
	case "done":
		return 2
	}
	return 3
}

func tick() tea.Cmd { return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} }) }

func (m model) Init() tea.Cmd { return tea.Batch(snapCmd, tick()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(snapCmd, tick())
	case snapMsg:
		if msg != nil {
			m.agents = []Agent(msg)
			m.sel = min(m.sel, max(len(m.agents)-1, 0))
		}
		if !m.prBusy && time.Since(m.prAt) > 30*time.Second && len(m.agents) > 0 {
			m.prBusy = true
			as := m.agents
			return m, func() tea.Msg { return prMsg(fetchPRs(as)) }
		}
	case prMsg:
		m.prs, m.prBusy, m.prAt = map[string]PR(msg), false, time.Now()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.sel = min(m.sel+1, len(m.agents)-1)
		case "k", "up":
			m.sel = max(m.sel-1, 0)
		case "enter":
			return m, m.jump(m.sel)
		}
	case tea.MouseClickMsg:
		ms := msg.Mouse()
		if ms.Button == tea.MouseLeft {
			if i := ms.Y - headH + m.scroll(); i >= 0 && i < len(m.rowAgent) && m.rowAgent[i] >= 0 {
				m.sel = m.rowAgent[i]
				return m, m.jump(m.sel)
			}
		}
	}
	return m, nil
}

func (m model) jump(i int) tea.Cmd {
	if i < 0 || i >= len(m.agents) {
		return nil
	}
	pane := m.agents[i].PaneID
	return func() tea.Msg {
		exec.Command(herdrBin(), "agent", "focus", pane).Run()
		return tea.Quit()
	}
}

// ---- rendering ----

func c(code int, s string) string { return fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", code, s) }

func statusColor(s string) int {
	switch s {
	case "working":
		return 208
	case "blocked":
		return 205
	case "done":
		return 78
	}
	return 245
}

func fit(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:max(n-1, 0)]) + "…"
	}
	return s + strings.Repeat(" ", n-len(r))
}

var beadRe = regexp.MustCompile(`^[a-z]+-[a-z0-9]+$`)

func ciGlyph(p PR) string {
	if p.Num == 0 {
		return c(240, "no PR")
	}
	g := map[string][2]any{"pass": {42, "✓"}, "fail": {205, "✗"}, "pending": {214, "●"}, "none": {245, "–"}}[p.CI]
	return fmt.Sprintf("#%d %s", p.Num, c(g[0].(int), g[1].(string)))
}

func (m *model) body(w int) []string {
	var lines []string
	m.rowAgent = m.rowAgent[:0]
	add := func(s string, ai int) { lines = append(lines, s); m.rowAgent = append(m.rowAgent, ai) }
	repo := ""
	for i, a := range m.agents {
		if a.Repo != repo {
			repo = a.Repo
			n := 0
			for _, b := range m.agents {
				if b.Repo == repo {
					n++
				}
			}
			add("", -1)
			add(c(111, fmt.Sprintf("◉ %s", repo))+c(240, fmt.Sprintf("  %d agents", n)), -1)
		}
		last := i+1 == len(m.agents) || m.agents[i+1].Repo != repo
		br, vert := "├─", "│"
		if last {
			br, vert = "└─", " "
		}
		col := statusColor(a.Status)
		pr := m.prs[a.PaneID]
		edge := 240
		if a.Status == "blocked" || pr.CI == "fail" {
			edge = 205
		}
		bead := ""
		if beadRe.MatchString(a.Name) {
			bead = c(141, "bead "+a.Name)
		}
		l1 := fmt.Sprintf(" %s %s %s %s  %s  %s", c(edge, br), c(col, "⬢"), fit(a.Name, 16), c(col, fit(a.Status, 8)), c(245, a.Model), phaseLabel(a))
		l2 := fmt.Sprintf(" %s     %s  %s  %s", c(edge, vert), c(110, a.Branch), ciGlyph(pr), bead)
		if i == m.sel {
			l1, l2 = "\x1b[7m"+l1+"\x1b[27m", "\x1b[7m"+l2+"\x1b[27m"
		}
		add(l1, i)
		add(l2, i)
	}
	return lines
}

// scroll keeps the selected agent's two rows in view; needs rowAgent from body()
func (m model) scroll() int {
	avail, off := max(m.h-headH-footH, 1), 0
	for i, ai := range m.rowAgent {
		if ai == m.sel && i+2 > off+avail {
			off = i + 2 - avail
		}
	}
	return off
}

var phaseStyle = map[string][2]any{
	"thinking":   {141, "◐ thinking"},
	"writing":    {214, "✎ writing"},
	"tool":       {45, "⚙ tool"},
	"monitoring": {111, "◉ monitoring"},
}

func phaseLabel(a Agent) string {
	p, ok := phaseStyle[a.Phase]
	if !ok {
		return ""
	}
	s := p[1].(string)
	if a.Tool != "" {
		s += " " + a.Tool
	}
	return c(p[0].(int), s)
}

func (m model) View() tea.View {
	var b strings.Builder
	work, blocked, fail := 0, 0, 0
	for _, a := range m.agents {
		switch a.Status {
		case "working":
			work++
		case "blocked":
			blocked++
		}
		if m.prs[a.PaneID].CI == "fail" {
			fail++
		}
	}
	fmt.Fprintf(&b, " %s  %s   %s   %s\n\n", c(111, "HERDR CLAUDE DASHBOARD"),
		c(208, fmt.Sprintf("ACTIVE %d/%d", work, len(m.agents))),
		c(205, fmt.Sprintf("BLOCKED %d", blocked)), c(205, fmt.Sprintf("CI FAIL %d", fail)))
	lines := m.body(m.w)
	avail := max(m.h-headH-footH, 1)
	off := m.scroll()
	for i := off; i < min(off+avail, len(lines)); i++ {
		b.WriteString(lines[i] + "\n")
	}
	b.WriteString(strings.Repeat("\n", max(avail-(min(off+avail, len(lines))-off), 0)))
	b.WriteString(c(240, " ↑↓/jk move · enter/click jump · q close"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
