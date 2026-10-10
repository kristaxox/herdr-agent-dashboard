package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestGenMocks(t *testing.T) {
	out := os.Getenv("MOCK_OUT")
	if out == "" {
		t.Skip()
	}
	cfg.CardMinWidth = 30
	m := model{w: 96, prs: map[string]PR{"p2": {5, "OPEN", "pass"}, "p3": {7, "OPEN", "fail"}, "p4": {9, "OPEN", "pending"}},
		open: map[string]bool{}, details: map[string]Detail{}, sel: 1}
	m.agents = []Agent{
		{Name: "orchestrator", PaneID: "p1", Status: "idle", Repo: "demo", Model: "opus-5-5", Branch: "main", Primary: true},
		{Name: "abc-123", PaneID: "p2", Status: "working", Repo: "demo", Model: "opus-5-5", Branch: "feat/abc-123-login", Phase: "writing", Tool: "Edit"},
		{Name: "abc-456", PaneID: "p3", Status: "blocked", Repo: "demo", Model: "sonnet-5-5", Branch: "feat/abc-456-api", Phase: "tool", Tool: "Bash"},
		{Name: "abc-789", PaneID: "p4", Status: "idle", Repo: "demo", Model: "sonnet-5-5", Branch: "feat/abc-789-docs", Phase: "monitoring", Monitors: 1},
		{Name: "xyz-001", PaneID: "p5", Status: "done", Repo: "widgets", Model: "opus-5-5", Branch: "main"},
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for i, name := range modes {
		m.mode = i
		lines, _ := m.render()
		m.h = len(lines) + headH + footH
		v := ansi.ReplaceAllString(m.View().Content, "")
		var ls []string
		for _, l := range strings.Split(v, "\n") {
			ls = append(ls, strings.TrimRight(l, " "))
		}
		os.WriteFile(out+"/"+name+".txt", []byte(strings.TrimRight(strings.Join(ls, "\n"), "\n")+"\n"), 0o600)
	}
}
