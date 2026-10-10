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

func tri(open bool) string {
	if open {
		return c(214, "▾")
	}
	return c(245, "▸")
}

func indent(ls []string, n int) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = strings.Repeat(" ", n) + l
	}
	return out
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
		l1 := fmt.Sprintf(" %s %s %s %s %s  %s  %s", c(edge, br), tri(m.open[a.PaneID]), m.dot(a), fit(a.Name, 16), c(col, fit(a.Status, 8)), c(245, a.Model), phaseLabel(a))
		l2 := fmt.Sprintf(" %s     %s  %s  %s", c(edge, vert), c(110, a.Branch), ciGlyph(pr), m.beadTag(a))
		hits = append(hits, rect{4, len(lines), 6, len(lines) + 1, i, true}, rect{0, len(lines), m.w, len(lines) + 2, i, false})
		lines = append(lines, sel(l1, i == m.sel), sel(l2, i == m.sel))
		if m.open[a.PaneID] {
			lines = append(lines, indent(m.drawer(a, m.w-6), 5)...)
		}
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
					text = fmt.Sprintf("%s─ %s %s %s  %s  %s", c(ec, "─"), m.dot(a), fit(a.Name, 16), c(col, fit(a.Status, 8)), c(245, a.Model), phaseLabel(a))
				} else {
					text = fmt.Sprintf("   %s  %s  %s", c(110, a.Branch), ciGlyph(m.prs[a.PaneID]), m.beadTag(a))
				}
				if !agentRow {
					t = trunk(r < lastRow, r < lastRow, false, left) // the hub may land on a detail line
				}
				text = sel(text, ai == m.sel)
			}
			line := hub + spacer + c(ec, t) + text
			if ai >= 0 {
				hits = append(hits, rect{hubW + 2, len(lines), m.w, len(lines) + 1, ai, false})
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
		start = end
	}
	return lines, hits
}

// ---- grid: cards, Agent Grid style ----

func (m model) cols() int { return max(m.w/cfg.CardMinWidth, 1) }

func (m model) gridView() ([]string, []rect) {
	cols := m.cols()
	cw := max(m.w/cols-1, 30)
	in := cw - 2
	var lines []string
	var hits []rect
	for i := 0; i < len(m.agents); i += cols {
		block := make([][]string, 0, cols)
		for j := i; j < min(i+cols, len(m.agents)); j++ {
			x := (j - i) * (cw + 1)
			block = append(block, m.card(m.agents[j], j == m.sel, cw, in))
			hits = append(hits, rect{x + cw - 4, len(lines), x + cw - 1, len(lines) + 1, j, true}, rect{x, len(lines), x + cw, len(lines) + 6, j, false})
		}
		lines = append(lines, joinCards(block)...)
		for j := i; j < min(i+cols, len(m.agents)); j++ {
			if m.open[m.agents[j].PaneID] {
				lines = append(lines, m.drawer(m.agents[j], m.w-1)...)
			}
		}
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

// comet is the head-to-tail colour ramp of the working animation (256-colour cyan -> blue).
var comet = [...]int{51, 45, 39, 33, 27, 25}

// alarm is the blocked ramp: dark to bright amber to near-white, one step per frame (~0.5s).
var alarm = [...]int{236, 94, 172, 214, 226, 231, 226, 214, 172, 94}

// calm is the done ramp: slow green swell, 6 frames a step (~4s). Tops out at 34
// (0,175,0), the 256-colour step nearest 75% of full green.
var calm = [...]int{22, 22, 28, 34, 34, 28, 22, 22}

// pulseColor is the colour of a blocked or done agent's pulse at a frame.
func pulseColor(status string, frame int) (int, bool) {
	switch status {
	case "blocked":
		return alarm[frame%len(alarm)], true
	case "done":
		return calm[frame/6%len(calm)], true
	}
	return 0, false
}

// borderColor colours border cell p of a ring of n cells at the given frame.
// Working cards get a comet circling clockwise over the base colour, blocked
// blocked cards flash amber, done cards swell green, everything else keeps the base.
func borderColor(a Agent, base, p, n, frame int) int {
	if !cfg.Animate {
		if a.Status == "working" || a.Status == "blocked" {
			return statusColor(a.Status)
		}
		return base
	}
	switch a.Status {
	case "blocked", "done":
		col, _ := pulseColor(a.Status, frame)
		return col
	case "working":
		if d := ((frame-p)%n + n) % n; d < len(comet) {
			return comet[d]
		}
	}
	return base
}

// dot is the agent's hexagon: working sweeps the comet ramp back and forth, blocked flashes amber, done swells green.
func (m model) dot(a Agent) string {
	col := statusColor(a.Status)
	if cfg.Animate {
		switch a.Status {
		case "blocked", "done":
			col, _ = pulseColor(a.Status, m.frame)
		case "working":
			i := m.frame / 2 % (2*len(comet) - 2)
			if i >= len(comet) {
				i = 2*len(comet) - 2 - i
			}
			col = comet[i]
		}
	}
	return c(col, "⬢")
}

func (m model) card(a Agent, on bool, cw, in int) []string {
	col := statusColor(a.Status)
	bc := 240
	tl, tr, bl, br, h, v := "╭", "╮", "╰", "╯", "─", "│"
	if on {
		tl, tr, bl, br, h, v, bc = "╔", "╗", "╚", "╝", "═", "║", 255
	}
	// the border is a ring of n cells: top left->right, right side down,
	// bottom right->left, left side up. bd(p) is the colour of ring cell p.
	rows := 6
	n := 2*cw + 2*(rows-2)
	bd := func(p int) int { return borderColor(a, bc, p, n, m.frame) }
	bc1 := func(p int, s string) string { return c(bd(p), s) }
	title := " " + trunc(a.Repo, in-6) + " "
	if on {
		title = " ▶ " + trunc(a.Repo, in-8) + " "
	}
	top := bc1(0, tl) + bc1(1, h) + c(111, title)
	for x := 2 + vlen(title); x < cw-4; x++ {
		top += bc1(x, h)
	}
	top += bc1(cw-4, " ") + tri(m.open[a.PaneID]) + bc1(cw-2, " ") + bc1(cw-1, tr)
	row := func(r int, s string) string {
		return bc1(n-r, v) + padTo(s, in) + bc1(cw-1+r, v) // left side runs up: r=4 -> 2cw+4 .. r=1 -> 2cw+7
	}
	bottom := ""
	for x := 0; x < cw; x++ {
		s := h
		switch x {
		case 0:
			s = bl
		case cw - 1:
			s = br
		}
		bottom += bc1(cw+4+(cw-1-x), s)
	}
	pr := m.prs[a.PaneID]
	return []string{
		top,
		row(1, fmt.Sprintf(" %s %s %s", m.dot(a), fit(a.Name, in-14), c(col, fit(a.Status, 9)))),
		row(2, " "+c(245, a.Model)+"  "+phaseLabel(a)),
		row(3, " "+c(110, trunc(a.Branch, in-2))),
		row(4, " "+ciGlyph(pr)+"  "+m.beadTag(a)),
		bottom,
	}
}

// ---- table: one dense line per agent ----

func (m model) tableView() ([]string, []rect) {
	lines := []string{c(240, " "+fit("", 4)+fit("AGENT", 16)+fit("STATUS", 9)+fit("PHASE", 22)+fit("MODEL", 16)+fit("REPO", 14)+fit("BRANCH", 36)+"PR / CI"), ""}
	var hits []rect
	for i, a := range m.agents {
		col := statusColor(a.Status)
		l := fmt.Sprintf(" %s %s %s%s%s%s%s%s%s", tri(m.open[a.PaneID]), m.dot(a), fit(a.Name, 16), c(col, fit(a.Status, 9)),
			padTo(phaseLabel(a), 22), fit(a.Model, 16), fit(a.Repo, 14), c(110, fit(a.Branch, 36)), ciGlyph(m.prs[a.PaneID]))
		hits = append(hits, rect{1, len(lines), 3, len(lines) + 1, i, true}, rect{0, len(lines), m.w, len(lines) + 1, i, false})
		lines = append(lines, sel(l, i == m.sel))
		if m.open[a.PaneID] {
			lines = append(lines, indent(m.drawer(a, m.w-4), 3)...)
		}
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
		// owners (primary checkout) sit on top; linked-worktree agents hang below.
		// No owner or no workers: fall back to a hub box over everyone.
		var owners, kids []int
		for i := start; i < end; i++ {
			if m.agents[i].Primary {
				owners = append(owners, i)
			} else {
				kids = append(kids, i)
			}
		}
		useOwner := cfg.OwnerMode != "none" && len(owners) > 0 && len(kids) > 0
		if !useOwner {
			owners, kids = nil, nil
			for i := start; i < end; i++ {
				kids = append(kids, i)
			}
		}
		ec := 240
		for _, i := range kids {
			if m.edgeColor(m.agents[i]) == 205 {
				ec = 205
			}
		}
		for ci := 0; ci < len(kids); ci += cols {
			chunk := kids[ci:min(ci+cols, len(kids))]
			k := len(chunk)
			xs := make([]int, k)
			for j := range xs {
				xs[j] = j*(cw+1) + cw/2
			}
			xc := (xs[0] + xs[k-1]) / 2
			if ci == 0 && useOwner {
				n := min(len(owners), cols)
				left := max(xc-(n*(cw+1)-1)/2, 0)
				block := make([][]string, n)
				for j := 0; j < n; j++ {
					block[j] = m.card(m.agents[owners[j]], owners[j] == m.sel, cw, cw-2)
					x := left + j*(cw+1)
					hits = append(hits, rect{x + cw - 4, len(lines), x + cw - 1, len(lines) + 1, owners[j], true}, rect{x, len(lines), x + cw, len(lines) + 6, owners[j], false})
				}
				lines = append(lines, indent(joinCards(block), left)...)
				for _, o := range owners[:n] {
					if m.open[m.agents[o].PaneID] {
						lines = append(lines, m.drawer(m.agents[o], m.w-1)...)
					}
				}
			} else if ci == 0 {
				pad := strings.Repeat(" ", max(xc-hubW/2, 0))
				repo := m.agents[start].Repo
				body := padTo(c(111, "◉ "+trunc(repo, hubW-10))+c(240, fmt.Sprintf(" %d", end-start)), hubW-4)
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
				drops[x] = cell{'│', m.edgeColor(m.agents[chunk[j]])}
			}
			lines = append(lines, rowStr(drops))
			block := make([][]string, k)
			for j, ai := range chunk {
				block[j] = m.card(m.agents[ai], ai == m.sel, cw, cw-2)
				x := j * (cw + 1)
				hits = append(hits, rect{x + cw - 4, len(lines), x + cw - 1, len(lines) + 1, ai, true}, rect{x, len(lines), x + cw, len(lines) + 6, ai, false})
			}
			lines = append(lines, joinCards(block)...)
			for _, ai := range chunk {
				if m.open[m.agents[ai].PaneID] {
					lines = append(lines, m.drawer(m.agents[ai], m.w-1)...)
				}
			}
		}
		lines = append(lines, "")
		start = end
	}
	return lines, hits
}
