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
	agents []Agent
	prs    map[string]PR
	sel    int // index into agents (display order)
	w, h   int
	prBusy bool
	prAt   time.Time
	mode   int
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
		case "v", "tab":
			m.mode = (m.mode + 1) % len(modes)
		case "1", "2", "3", "4":
			m.mode = int(msg.String()[0] - '1')
		case "j", "down":
			m.sel = min(m.sel+m.vstep(), len(m.agents)-1)
		case "k", "up":
			m.sel = max(m.sel-m.vstep(), 0)
		case "l", "right":
			m.sel = min(m.sel+1, len(m.agents)-1)
		case "h", "left":
			m.sel = max(m.sel-1, 0)
		case "enter":
			return m, m.jump(m.sel)
		}
	case tea.MouseClickMsg:
		ms := msg.Mouse()
		if ms.Button == tea.MouseLeft {
			_, hits := m.render()
			y := ms.Y - headH + m.scroll(hits)
			for _, h := range hits {
				if y >= h.y0 && y < h.y1 && ms.X >= h.x0 && ms.X < h.x1 {
					m.sel = h.ai
					return m, m.jump(m.sel)
				}
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
		s += " " + trunc(a.Tool, 14)
	}
	return c(p[0].(int), s)
}
