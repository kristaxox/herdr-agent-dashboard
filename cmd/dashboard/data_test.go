package main

import "testing"

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
