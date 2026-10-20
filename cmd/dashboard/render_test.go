package main

import (
	"fmt"
	"strings"
	"testing"
)

// smoke test: every view renders, with and without agents, and hits stay in range
func TestRenderViews(t *testing.T) {
	m := model{w: 120, h: 40, prs: map[string]PR{"p2": {547, "OPEN", "pass"}}, open: map[string]bool{"p2": true, "p4": true}, details: map[string]Detail{"p2": {Checks: []Check{{"build", "pass"}, {"e2e", "fail"}}, PRTitle: "t", Said: "hi\n\nthere"}}}
	for _, agents := range [][]Agent{
		nil,
		{
			{Name: "orchestrator", PaneID: "p1", Status: "idle", Repo: "demo", Primary: true},
			{Name: "abc-123", PaneID: "p2", Status: "working", Repo: "demo", Phase: "writing", Tool: "Edit"},
			{Name: "look around", PaneID: "p2/toolu_x", Status: "working", Repo: "demo", Parent: "abc-123", SubType: "Explore"},
			{Name: "solo", PaneID: "p4", Status: "done", Repo: "other"},
		},
	} {
		m.agents = agents
		for i := range modes {
			m.mode = i
			lines, hits := m.render()
			for _, h := range hits {
				if h.ai < 0 || h.ai >= len(agents) || h.y1 > len(lines) {
					t.Errorf("%s: bad hit %+v", modes[i], h)
				}
			}
		}
	}
}

// the comet is a contiguous run of border cells, the pulse recolours the border, neither changes card size
func TestCardAnimation(t *testing.T) {
	cw := 40
	work, blocked := Agent{Name: "a", Status: "working", Repo: "r"}, Agent{Name: "b", Status: "blocked", Repo: "r"}
	m := model{prs: map[string]PR{}, open: map[string]bool{}}
	for f := 0; f < 2*cw+8; f++ {
		m.frame = f
		card := m.card(work, false, cw, cw-2)
		if len(card) != 6 {
			t.Fatalf("frame %d: %d rows", f, len(card))
		}
		for _, l := range card {
			if vlen(l) != cw {
				t.Fatalf("frame %d: row width %d, want %d", f, vlen(l), cw)
			}
		}
		joined := strings.Join(card, "")
		seen := false // the head hides while it passes under the title, so any comet colour will do
		for _, code := range comet {
			seen = seen || strings.Contains(joined, fmt.Sprintf("\x1b[38;5;%dm", code))
		}
		if !seen {
			t.Fatalf("frame %d: no comet cells", f)
		}
	}
	m.frame = 0
	a := m.card(blocked, false, cw, cw-2)
	m.frame = 6
	b := m.card(blocked, false, cw, cw-2)
	if strings.Join(a, "") == strings.Join(b, "") {
		t.Error("blocked card does not pulse")
	}
	done := Agent{Name: "c", Status: "done", Repo: "r"}
	m.frame = 0
	a = m.card(done, false, cw, cw-2)
	m.frame = 18
	b = m.card(done, false, cw, cw-2)
	if strings.Join(a, "") == strings.Join(b, "") {
		t.Error("done card does not pulse")
	}
}

// a selected row keeps its background from end to end: nothing inside it may reset it
func TestSelectedRowStaysInverted(t *testing.T) {
	m := model{w: 120, h: 40, prs: map[string]PR{}, open: map[string]bool{}, details: map[string]Detail{}, mode: 0,
		agents: []Agent{{Name: "a", PaneID: "p1", Status: "working", Repo: "r", Branch: "main"}}}
	for _, mode := range []int{0, 3} { // tree, table
		m.mode = mode
		lines, _ := m.render()
		found := false
		for _, l := range lines {
			if strings.Contains(l, "\x1b[48;5;237m") {
				found = true
				if strings.Contains(l, "\x1b[0m") || strings.Count(l, "\x1b[49m") != 1 {
					t.Errorf("%s: selected row resets attributes: %q", modes[mode], l)
				}
			}
		}
		if !found {
			t.Errorf("%s: no selected row", modes[mode])
		}
	}
}

// graph hangs a subagent's card below its parent's card, and every card stays clickable
func TestGraphSubagentUnderParent(t *testing.T) {
	m := model{w: 120, h: 40, prs: map[string]PR{}, open: map[string]bool{}, details: map[string]Detail{}, mode: 1,
		agents: []Agent{
			{Name: "orchestrator", PaneID: "p1", Status: "idle", Repo: "r", Primary: true},
			{Name: "w", PaneID: "p2", Status: "working", Repo: "r"},
			{Name: "sub", PaneID: "p2/toolu_x", Status: "working", Repo: "r", Parent: "w", SubType: "Explore"},
			{Name: "w2", PaneID: "p3", Status: "idle", Repo: "r"},
		}}
	top := map[int]int{}
	_, hits := m.render()
	for _, h := range hits {
		if !h.tog {
			top[h.ai] = h.y0
		}
	}
	if len(top) != 4 {
		t.Fatalf("cards with hits: %v", top)
	}
	if top[2] <= top[1] || top[1] != top[3] {
		t.Errorf("sub at y=%d, parent at %d, sibling at %d", top[2], top[1], top[3])
	}
}
