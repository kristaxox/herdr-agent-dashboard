package main

import "testing"

// smoke test: every view renders, with and without agents, and hits stay in range
func TestRenderViews(t *testing.T) {
	m := model{w: 120, h: 40, prs: map[string]PR{"p2": {547, "OPEN", "pass"}}}
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
