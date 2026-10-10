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
