package main

import (
	"fmt"
	"os"
	"os/exec"
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
	// expandable cards, keyed by pane id (maps are shared across model copies)
	open     map[string]bool
	details  map[string]Detail
	inflight map[string]bool
	// double-click detection
	lastClick time.Time
	lastAI    int
}

const headH, footH = 2, 1

func main() {
	// plugin panes can inherit a thin PATH; make sure bd, gh and git are findable
	loadConfig(configPath())
	for _, p := range cfg.ExtraPath {
		os.Setenv("PATH", os.Getenv("PATH")+":"+expandHome(p))
	}
	if _, err := tea.NewProgram(model{mode: cfg.viewIndex(), prs: map[string]PR{}, open: map[string]bool{}, details: map[string]Detail{}, inflight: map[string]bool{}}).Run(); err != nil {
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
		if a.Primary != b.Primary {
			return a.Primary // owners first within a repo
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

func tick() tea.Cmd {
	return tea.Tick(sec(cfg.PollSeconds), func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Init() tea.Cmd { return tea.Batch(snapCmd, tick()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tickMsg:
		return m, tea.Batch(snapCmd, tick())
	case detailMsg:
		m.details[msg.pane] = msg.d
		delete(m.inflight, msg.pane)
	case snapMsg:
		if msg != nil {
			m.agents = []Agent(msg)
			m.sel = min(m.sel, max(len(m.agents)-1, 0))
		}
		cmds := m.detailCmds()
		if !m.prBusy && time.Since(m.prAt) > sec(cfg.PRRefreshSecs) && len(m.agents) > 0 {
			m.prBusy = true
			as := m.agents
			cmds = append(cmds, func() tea.Msg { return prMsg(fetchPRs(as)) })
		}
		return m, tea.Batch(cmds...)
	case prMsg:
		m.prs, m.prBusy, m.prAt = map[string]PR(msg), false, time.Now()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "v", "tab":
			m.mode = (m.mode + 1) % len(modes)
		case "1", "2", "3", "4", "5":
			m.mode = int(msg.String()[0] - '1')
		case "j", "down":
			m.sel = min(m.sel+m.vstep(), len(m.agents)-1)
		case "k", "up":
			m.sel = max(m.sel-m.vstep(), 0)
		case "l", "right":
			m.sel = min(m.sel+1, len(m.agents)-1)
		case "h", "left":
			m.sel = max(m.sel-1, 0)
		case "e", "space", " ":
			if m.sel < len(m.agents) {
				m.toggle(m.agents[m.sel].PaneID)
			}
			return m, tea.Batch(m.detailCmds()...)
		case "E":
			all := false
			for _, a := range m.agents {
				all = all || !m.open[a.PaneID]
			}
			for _, a := range m.agents {
				m.open[a.PaneID] = all
			}
			return m, tea.Batch(m.detailCmds()...)
		case "enter":
			return m, m.jump(m.sel)
		}
	case tea.MouseClickMsg:
		ms := msg.Mouse()
		if ms.Button == tea.MouseLeft {
			_, hits := m.render()
			y := ms.Y - headH + m.scroll(hits)
			for _, tog := range []bool{true, false} { // triangles win over the card under them
				for _, h := range hits {
					if h.tog == tog && y >= h.y0 && y < h.y1 && ms.X >= h.x0 && ms.X < h.x1 {
						m.sel = h.ai
						dbl := !tog && h.ai == m.lastAI && time.Since(m.lastClick) < time.Duration(cfg.DoubleClickMs)*time.Millisecond
						m.lastClick, m.lastAI = time.Now(), h.ai
						if dbl {
							return m, m.jump(m.sel)
						}
						if tog || cfg.ExpandOnClick {
							m.toggle(m.agents[h.ai].PaneID) // single click (or the triangle) opens/closes details
						}
						return m, tea.Batch(m.detailCmds()...)
					}
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
	if a.Monitors > 0 {
		if a.Phase == "monitoring" {
			s += fmt.Sprintf(" ×%d", a.Monitors)
		} else {
			s += fmt.Sprintf(" +◉%d", a.Monitors) // busy, and monitors are also running
		}
	}
	return c(p[0].(int), s)
}

func (m model) toggle(pane string) { m.open[pane] = !m.open[pane] }

// detailCmds fetches details for expanded cards that have none or are stale.
func (m model) detailCmds() []tea.Cmd {
	var cmds []tea.Cmd
	for _, a := range m.agents {
		if d, ok := m.details[a.PaneID]; !m.open[a.PaneID] || m.inflight[a.PaneID] || (ok && time.Since(d.At) < sec(cfg.DetailRefresh)) {
			continue
		}
		m.inflight[a.PaneID] = true
		a := a
		cmds = append(cmds, func() tea.Msg { return detailMsg{a.PaneID, fetchDetail(a)} })
	}
	return cmds
}
