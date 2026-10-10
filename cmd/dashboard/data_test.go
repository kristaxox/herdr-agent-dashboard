package main

import (
	"testing"
	"time"
)

func TestCollapseCI(t *testing.T) {
	cases := map[string][][2]string{
		"none":    {},
		"pass":    {{"COMPLETED", "SUCCESS"}, {"COMPLETED", "SKIPPED"}},
		"fail":    {{"COMPLETED", "SUCCESS"}, {"COMPLETED", "FAILURE"}},
		"pending": {{"COMPLETED", "SUCCESS"}, {"IN_PROGRESS", ""}},
	}
	for want, cs := range cases {
		if got := collapseCI(cs); got != want {
			t.Errorf("%s: got %s", want, got)
		}
	}
}

func TestSlugAndPhase(t *testing.T) {
	if got := claudeSlug("/home/u/.herdr/worktrees/x"); got != "-home-u--herdr-worktrees-x" {
		t.Error(got)
	}
	tool := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{}}]}}`)
	if p, n := lastPhase(tool); p != "tool" || n != "Bash" {
		t.Error(p, n)
	}
	edit := []byte(`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit"}]}}`)
	if p, _ := lastPhase(edit); p != "writing" {
		t.Error(p)
	}
	res := []byte(`{"message":{"role":"user","content":[{"type":"tool_result"}]}}`)
	if p, _ := lastPhase(res); p != "thinking" {
		t.Error(p)
	}
}

func TestCountMonitors(t *testing.T) {
	start := func(id, ts string) string {
		return `{"timestamp":"` + ts + `","message":{"content":[{"type":"tool_result","content":"Monitor started (task ` + id + `, expires in 30m unless"}]}}` + "\n"
	}
	end := func(id string) string {
		return `{"message":{"content":"<task-notification><task-id>` + id + `</task-id><status>completed</status></task-notification>"}}` + "\n"
	}
	event := `{"message":{"content":"<task-notification><task-id>b</task-id><summary>Monitor event</summary></task-notification>"}}` + "\n"
	now, _ := time.Parse(time.RFC3339, "2026-01-01T12:00:00Z")
	buf := start("a", "2026-01-01T11:50:00Z") + start("b", "2026-01-01T11:40:00Z") + event + start("c", "2026-01-01T11:00:00Z") + start("d", "2026-01-01T11:55:00Z") + end("d")
	// a running; b running (events don't end it); c expired; d ended
	if got := countMonitors([]byte(buf), now); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}
