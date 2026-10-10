package main

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type rect struct {
	x0, y0, x1, y1, ai int
	tog                bool // the expand triangle, not the card
}

var modes = []string{"tree", "graph", "grid", "table", "fan"}

func (m model) render() ([]string, []rect) {
	switch modes[m.mode] {
	case "graph":
		return m.graphView()
	case "fan":
		return m.fanView()
	case "grid":
		return m.gridView()
	case "table":
		return m.tableView()
	}
	return m.treeView()
}

// vstep: grid moves a whole card row on up/down, other views one agent
func (m model) vstep() int {
	if modes[m.mode] == "grid" || modes[m.mode] == "graph" {
		return m.cols()
	}
	return 1
}

// scroll keeps the selected agent fully in view
func (m model) scroll(hits []rect) int {
	avail, off := max(m.h-headH-footH, 1), 0
	extra := 0 // keep the open drawer in view too, but never scroll the card itself off the top
	if m.sel < len(m.agents) && m.open[m.agents[m.sel].PaneID] {
		extra = len(m.drawer(m.agents[m.sel], m.w-1))
	}
	for _, h := range hits {
		if h.ai == m.sel && !h.tog && h.y1+extra > off+avail {
			off = min(h.y1+extra-avail, h.y0)
		}
	}
	return off
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func vlen(s string) int { return len([]rune(ansiRe.ReplaceAllString(s, ""))) }

func padTo(s string, n int) string {
	if d := n - vlen(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:max(n-1, 0)]) + "…"
	}
	return s
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
	fmt.Fprintf(&b, " %s  %s   %s   %s   %s\n\n", c(111, "HERDR CLAUDE DASHBOARD"),
		c(208, fmt.Sprintf("ACTIVE %d/%d", work, len(m.agents))),
		c(205, fmt.Sprintf("BLOCKED %d", blocked)), c(205, fmt.Sprintf("CI FAIL %d", fail)),
		c(240, "view: "+modes[m.mode]))
	lines, hits := m.render()
	avail := max(m.h-headH-footH, 1)
	off := m.scroll(hits)
	end := min(off+avail, len(lines))
	for i := off; i < end; i++ {
		b.WriteString(lines[i] + "\n")
	}
	b.WriteString(strings.Repeat("\n", max(avail-(end-off), 0)))
	b.WriteString(c(240, " ↑↓←→/hjkl move · click details · double-click/enter jump · v/tab or 1-5 switch view · e/space expand · E all · q close"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
