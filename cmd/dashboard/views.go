package main

import (
	"fmt"
	"strings"
)

func (m model) edgeColor(a Agent) int {
	if a.Status == "blocked" || m.prs[a.PaneID].CI == "fail" {
		return 205
	}
	return 240
}

func (m model) beadTag(a Agent) string {
	if beadRe.MatchString(a.Name) {
		return c(141, "bead "+a.Name)
	}
	return ""
}

func sel(s string, on bool) string {
	if on {
		return "\x1b[7m" + s + "\x1b[27m"
	}
	return s
}

// ---- tree: repo header, agents hanging below with ├─ └─ ----

func (m model) treeView() ([]string, []rect) {
	var lines []string
	var hits []rect
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
			lines = append(lines, "", c(111, "◉ "+repo)+c(240, fmt.Sprintf("  %d agents", n)))
		}
		last := i+1 == len(m.agents) || m.agents[i+1].Repo != repo
		br, vert := "├─", "│"
		if last {
			br, vert = "└─", " "
		}
		col, pr, edge := statusColor(a.Status), m.prs[a.PaneID], m.edgeColor(a)
		l1 := fmt.Sprintf(" %s %s %s %s  %s  %s", c(edge, br), c(col, "⬢"), fit(a.Name, 16), c(col, fit(a.Status, 8)), c(245, a.Model), phaseLabel(a))
		l2 := fmt.Sprintf(" %s     %s  %s  %s", c(edge, vert), c(110, a.Branch), ciGlyph(pr), m.beadTag(a))
		hits = append(hits, rect{0, len(lines), m.w, len(lines) + 2, i})
		lines = append(lines, sel(l1, i == m.sel), sel(l2, i == m.sel))
	}
	return lines, hits
}

// ---- fan: repo hub box on the left fanning out to agent nodes ----

const hubW = 22

// trunk picks the box-drawing glyph for the fan column from its four arms.
func trunk(up, down, right, left bool) string {
	switch {
	case up && down && right && left:
		return "┼"
	case up && down && left:
		return "┤"
	case up && down && right:
		return "├"
	case down && right && left:
		return "┬"
	case up && right && left:
		return "┴"
	case down && right:
		return "╭"
	case up && right:
		return "╰"
	case up && down:
		return "│"
	case right && left, right:
		return "─"
	}
	return " "
}

func (m model) fanView() ([]string, []rect) {
	var lines []string
	var hits []rect
	for start := 0; start < len(m.agents); {
		end := start
		for end < len(m.agents) && m.agents[end].Repo == m.agents[start].Repo {
			end++
		}
		grp := m.agents[start:end]
		rows := max(2*len(grp), 3)
		hm := (rows - 1) / 2 // hub text row
		alert := false
		for _, a := range grp {
			alert = alert || m.edgeColor(a) == 205
		}
		ec := 240
		if alert {
			ec = 205
		}
		off := 0 // lone agent: shift down so it sits on the hub's text row
		if len(grp) == 1 {
			off = 1
		}
		lastRow := off + 2*(len(grp)-1)
		for r := 0; r < rows; r++ {
			hub := strings.Repeat(" ", hubW)
			switch r {
			case hm - 1:
				hub = c(111, "╭"+strings.Repeat("─", hubW-2)+"╮")
			case hm:
				hub = c(111, "│ ") + padTo(c(111, "◉ "+trunc(grp[0].Repo, hubW-12))+c(240, fmt.Sprintf(" %d", len(grp))), hubW-4) + c(111, " ├")
			case hm + 1:
				hub = c(111, "╰"+strings.Repeat("─", hubW-2)+"╯")
			}
			agentRow := r >= off && (r-off)%2 == 0 && r <= lastRow
			left := r == hm
			spacer := " "
			if left {
				spacer = c(ec, "─")
			}
			t := trunk(r > off && r <= lastRow, r >= off && r < lastRow, agentRow, left)
			text := ""
			ai := -1
			if r >= off && r <= lastRow+1 {
				ai = start + (r-off)/2
				a := grp[(r-off)/2]
				col := statusColor(a.Status)
				if agentRow {
					text = fmt.Sprintf("%s─ %s %s %s  %s  %s", c(ec, "─"), c(col, "⬢"), fit(a.Name, 16), c(col, fit(a.Status, 8)), c(245, a.Model), phaseLabel(a))
				} else {
					text = fmt.Sprintf("   %s  %s  %s", c(110, a.Branch), ciGlyph(m.prs[a.PaneID]), m.beadTag(a))
				}
				if !agentRow {
					t = trunk(r < lastRow, r < lastRow, false, false)
				}
				text = sel(text, ai == m.sel)
			}
			line := hub + spacer + c(ec, t) + text
			if ai >= 0 {
				hits = append(hits, rect{hubW + 2, len(lines), m.w, len(lines) + 1, ai})
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
		start = end
	}
	return lines, hits
}

// ---- grid: cards, Agent Grid style ----

func (m model) cols() int { return max(m.w/40, 1) }

func (m model) gridView() ([]string, []rect) {
	cols := m.cols()
	cw := max(m.w/cols-1, 30)
	in := cw - 2
	var lines []string
	var hits []rect
	for i := 0; i < len(m.agents); i += cols {
		block := make([][]string, 0, cols)
		for j := i; j < min(i+cols, len(m.agents)); j++ {
			block = append(block, m.card(m.agents[j], j == m.sel, cw, in))
			hits = append(hits, rect{(j - i) * (cw + 1), len(lines), (j-i)*(cw+1) + cw, len(lines) + 6, j})
		}
		lines = append(lines, joinCards(block)...)
	}
	return lines, hits
}

func joinCards(block [][]string) []string {
	out := make([]string, 6)
	for r := range out {
		out[r] = strings.Join(func() []string {
			row := make([]string, len(block))
			for k, cd := range block {
				row[k] = cd[r]
			}
			return row
		}(), " ")
	}
	return out
}

func (m model) card(a Agent, on bool, cw, in int) []string {
	col := statusColor(a.Status)
	bc := 240
	if a.Status == "working" || a.Status == "blocked" {
		bc = col
	}
	tl, tr, bl, br, h, v := "╭", "╮", "╰", "╯", "─", "│"
	if on {
		tl, tr, bl, br, h, v, bc = "╔", "╗", "╚", "╝", "═", "║", 255
	}
	title := " " + trunc(a.Repo, in-6) + " "
	if on {
		title = " ▶ " + trunc(a.Repo, in-8) + " "
	}
	top := c(bc, tl+h) + c(111, title) + c(bc, strings.Repeat(h, max(in-1-vlen(title), 0))+tr)
	row := func(s string) string { return c(bc, v) + padTo(s, in) + c(bc, v) }
	pr := m.prs[a.PaneID]
	return []string{
		top,
		row(fmt.Sprintf(" %s %s %s", c(col, "⬢"), fit(a.Name, in-14), c(col, fit(a.Status, 9)))),
		row(" " + c(245, a.Model) + "  " + phaseLabel(a)),
		row(" " + c(110, trunc(a.Branch, in-2))),
		row(" " + ciGlyph(pr) + "  " + m.beadTag(a)),
		c(bc, bl+strings.Repeat(h, in)+br),
	}
}

// ---- table: one dense line per agent ----

func (m model) tableView() ([]string, []rect) {
	lines := []string{c(240, " "+fit("", 2)+fit("AGENT", 16)+fit("STATUS", 9)+fit("PHASE", 22)+fit("MODEL", 16)+fit("REPO", 14)+fit("BRANCH", 36)+"PR / CI"), ""}
	var hits []rect
	for i, a := range m.agents {
		col := statusColor(a.Status)
		l := fmt.Sprintf(" %s %s%s%s%s%s%s%s", c(col, "⬢"), fit(a.Name, 16), c(col, fit(a.Status, 9)),
			padTo(phaseLabel(a), 22), fit(a.Model, 16), fit(a.Repo, 14), c(110, fit(a.Branch, 36)), ciGlyph(m.prs[a.PaneID]))
		hits = append(hits, rect{0, len(lines), m.w, len(lines) + 1, i})
		lines = append(lines, sel(l, i == m.sel))
	}
	return lines, hits
}

// ---- graph: top-down. repo hub on top, bus line, drops into grid cards ----

type cell struct {
	r   rune
	col int
}

func rowStr(cs map[int]cell) string {
	mx := -1
	for x := range cs {
		mx = max(mx, x)
	}
	var b strings.Builder
	for x := 0; x <= mx; x++ {
		if k, ok := cs[x]; ok {
			b.WriteString(c(k.col, string(k.r)))
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func (m model) graphView() ([]string, []rect) {
	cols := m.cols()
	cw := max(m.w/cols-1, 30)
	var lines []string
	var hits []rect
	for start := 0; start < len(m.agents); {
		end := start
		for end < len(m.agents) && m.agents[end].Repo == m.agents[start].Repo {
			end++
		}
		grp := m.agents[start:end]
		ec := 240
		for _, a := range grp {
			if m.edgeColor(a) == 205 {
				ec = 205
			}
		}
		for ci := 0; ci < len(grp); ci += cols {
			chunk := grp[ci:min(ci+cols, len(grp))]
			k := len(chunk)
			xs := make([]int, k)
			for j := range xs {
				xs[j] = j*(cw+1) + cw/2
			}
			xc := (xs[0] + xs[k-1]) / 2
			if ci == 0 {
				pad := strings.Repeat(" ", max(xc-hubW/2, 0))
				body := padTo(c(111, "◉ "+trunc(grp[0].Repo, hubW-10))+c(240, fmt.Sprintf(" %d", len(grp))), hubW-4)
				lines = append(lines,
					pad+c(111, "╭"+strings.Repeat("─", hubW-2)+"╮"),
					pad+c(111, "│ ")+body+c(111, " │"),
					pad+c(111, "╰"+strings.Repeat("─", hubW-2)+"╯"))
			}
			stem := '│'
			if ci > 0 {
				stem = '┆' // continuation of the same repo onto another row
			}
			lines = append(lines, rowStr(map[int]cell{xc: {stem, ec}}))
			if k > 1 {
				bus := map[int]cell{}
				for x := xs[0]; x <= xs[k-1]; x++ {
					bus[x] = cell{'─', ec}
				}
				bus[xc] = cell{'┴', ec}
				for j, x := range xs {
					r := '┬'
					switch {
					case j == 0:
						r = '╭'
					case j == k-1:
						r = '╮'
					}
					if x == xc {
						r = '┼'
					}
					bus[x] = cell{r, ec}
				}
				lines = append(lines, rowStr(bus))
			}
			drops := map[int]cell{}
			for j, x := range xs {
				drops[x] = cell{'│', m.edgeColor(chunk[j])}
			}
			lines = append(lines, rowStr(drops))
			block := make([][]string, k)
			for j, a := range chunk {
				ai := start + ci + j
				block[j] = m.card(a, ai == m.sel, cw, cw-2)
				hits = append(hits, rect{j * (cw + 1), len(lines), j*(cw+1) + cw, len(lines) + 6, ai})
			}
			lines = append(lines, joinCards(block)...)
		}
		lines = append(lines, "")
		start = end
	}
	return lines, hits
}
