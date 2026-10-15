package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Agent struct {
	Name, PaneID, Status, Cwd, Session, Repo string
	WS                                       int  // workspace order
	Primary                                  bool // lives in the repo's primary checkout, i.e. owns the linked worktrees
	Branch, Model, Phase, Tool               string
	Monitors                                 int      // live Monitor tasks, from the session log
	MonDescs                                 []string // what they watch (the Monitor call's description), oldest first
	Kids                                     []Agent  // running Claude subagents (Agent tool calls); flattened in after this agent
	Parent, SubType, Log                     string   // subagents only: parent's name, subagent_type, path of its own session log
	Workspace, Tab, Title                    string
	PR                                       PR
}

type PR struct {
	Num   int
	State string // OPEN/MERGED/CLOSED, "" = none
	CI    string // pass/fail/pending/none
}

func herdrBin() string {
	if b := os.Getenv("HERDR_BIN_PATH"); b != "" {
		return b
	}
	return "herdr"
}

func snapshot() ([]Agent, error) {
	out, err := exec.Command(herdrBin(), "api", "snapshot").Output()
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Snapshot struct {
				Agents []struct {
					Name        string `json:"name"`
					PaneID      string `json:"pane_id"`
					Status      string `json:"agent_status"`
					Cwd         string `json:"cwd"`
					WorkspaceID string `json:"workspace_id"`
					TabID       string `json:"tab_id"`
					Title       string `json:"terminal_title_stripped"`
					Session     struct {
						Value string `json:"value"`
					} `json:"agent_session"`
				} `json:"agents"`
				Workspaces []struct {
					ID       string `json:"workspace_id"`
					Number   int    `json:"number"`
					Worktree *struct {
						RepoName string `json:"repo_name"`
						Linked   bool   `json:"is_linked_worktree"`
					} `json:"worktree"`
				} `json:"workspaces"`
			} `json:"snapshot"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, err
	}
	type wsInfo struct {
		n       int
		repo    string
		primary bool
	}
	ws := map[string]wsInfo{}
	for _, w := range r.Result.Snapshot.Workspaces {
		repo, primary := "", false
		if w.Worktree != nil {
			repo, primary = w.Worktree.RepoName, !w.Worktree.Linked
		}
		ws[w.ID] = wsInfo{w.Number, repo, primary}
	}
	var as []Agent
	for _, a := range r.Result.Snapshot.Agents {
		w := ws[a.WorkspaceID]
		repo := w.repo
		if repo == "" {
			repo = "(no repo)"
		}
		as = append(as, Agent{Name: a.Name, PaneID: a.PaneID, Status: a.Status, Cwd: a.Cwd,
			Session: a.Session.Value, Repo: repo, WS: w.n, Primary: w.primary,
			Workspace: a.WorkspaceID, Tab: a.TabID, Title: a.Title})
	}
	return as, nil
}

// fast fields: cheap enough to refresh every tick
func enrichFast(a *Agent) {
	if out, err := exec.Command("git", "-C", a.Cwd, "branch", "--show-current").Output(); err == nil {
		a.Branch = strings.TrimSpace(string(out))
	}
	buf, session := sessionTail(a.Cwd, a.Session)
	a.Model = lastModel(buf)
	a.Phase, a.Tool = "", ""
	a.MonDescs = activeMonitors(buf, time.Now())
	a.Monitors = len(a.MonDescs)
	a.Kids = subAgents(*a, session, activeSubagents(buf))
	if a.Status == "working" {
		a.Phase, a.Tool = lastPhase(buf)
	} else if len(a.Kids) > 0 {
		a.Phase = "subagents" // idle in herdr's eyes, but background subagents are running
	} else if a.Monitors > 0 {
		a.Phase = "monitoring" // idle in herdr's eyes, but waiting on a monitor
	}
}

var modelRe = regexp.MustCompile(`"model":"([^"]+)"`)

// claude slug: every / and . in cwd becomes -
func claudeSlug(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
}

var contRe = regexp.MustCompile(`"continuedInSessionId":"([^"]+)"`)

func projectDir(cwd string) string {
	home, _ := os.UserHomeDir()
	return home + "/.claude/projects/" + claudeSlug(cwd) + "/"
}

// sessionTail reads the end of the session log and returns it with the id of the
// log it came from. When a session was continued (context compaction/clear) the old
// log ends with a pointer to the new one, but herdr keeps reporting the old id, so
// follow the chain.
func sessionTail(cwd, session string) ([]byte, string) {
	var buf []byte
	read := ""
	for hop := 0; hop < 8 && session != ""; hop++ {
		b, ok := tailFile(projectDir(cwd) + session + ".jsonl")
		if !ok {
			break
		}
		buf, read = b, session
		m := contRe.FindSubmatch(b)
		if m == nil || string(m[1]) == session {
			break
		}
		session = string(m[1])
	}
	return buf, read
}

func tailFile(path string) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	st, _ := f.Stat()
	tail := int64(cfg.SessionTailKB) << 10 // ponytail: tail only (monitors live up to ~30m); raise session_tail_kb if a model/monitor line is missed
	off := max(st.Size()-tail, 0)
	b := make([]byte, st.Size()-off)
	f.ReadAt(b, off)
	return b, true
}

func lastModel(buf []byte) string {
	ms := modelRe.FindAllSubmatch(buf, -1)
	if len(ms) == 0 {
		return ""
	}
	return strings.TrimPrefix(string(ms[len(ms)-1][1]), "claude-")
}

var toolRe = regexp.MustCompile(`"type":"tool_use"[^}]*?"name":"([^"]+)"|"name":"([^"]+)"[^}]*?"type":"tool_use"`)

// lastPhase classifies the newest assistant/user log line:
// thinking | writing | tool | monitoring. Each log line holds one content block.
func lastPhase(buf []byte) (phase, tool string) {
	lines := strings.Split(string(buf), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		switch {
		case strings.Contains(l, `"type":"tool_result"`):
			return "thinking", ""
		case strings.Contains(l, `"type":"tool_use"`):
			name := ""
			if m := toolRe.FindStringSubmatch(l); m != nil {
				name = m[1] + m[2]
			}
			switch name {
			case "Edit", "Write", "NotebookEdit":
				return "writing", name
			case "Monitor", "ScheduleWakeup", "TaskOutput":
				return "monitoring", name
			}
			return "tool", name
		case strings.Contains(l, `"role":"assistant"`) && strings.Contains(l, `"type":"thinking"`):
			return "thinking", ""
		case strings.Contains(l, `"role":"assistant"`) && strings.Contains(l, `"type":"text"`):
			return "writing", ""
		}
	}
	return "", ""
}

// slow fields: gh calls, run in parallel in the background
func fetchPRs(as []Agent) map[string]PR {
	res := map[string]PR{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, a := range as {
		if a.Parent != "" {
			continue // a subagent shares its parent's PR
		}
		wg.Add(1)
		go func(a Agent) {
			defer wg.Done()
			pr := prFor(a.Cwd)
			mu.Lock()
			res[a.PaneID] = pr
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	return res
}

func prFor(cwd string) PR {
	cmd := exec.Command("gh", "pr", "view", "--json", "number,state,statusCheckRollup")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return PR{}
	}
	var r struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Checks []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"` // StatusContext
		} `json:"statusCheckRollup"`
	}
	if json.Unmarshal(out, &r) != nil {
		return PR{}
	}
	var cs [][2]string
	for _, c := range r.Checks {
		cs = append(cs, [2]string{c.Status + c.State, c.Conclusion})
	}
	return PR{r.Number, r.State, collapseCI(cs)}
}

// collapseCI: any failure -> fail, else any unfinished -> pending, else pass, no checks -> none.
// each check is {status-or-state, conclusion}
func collapseCI(cs [][2]string) string {
	if len(cs) == 0 {
		return "none"
	}
	pending := false
	for _, c := range cs {
		switch strings.ToUpper(c[1]) {
		case "FAILURE", "TIMED_OUT", "CANCELLED", "STARTUP_FAILURE", "ACTION_REQUIRED":
			return "fail"
		case "SUCCESS", "NEUTRAL", "SKIPPED":
		default:
			switch strings.ToUpper(c[0]) {
			case "FAILURE", "ERROR":
				return "fail"
			case "SUCCESS":
			default:
				pending = true
			}
		}
	}
	if pending {
		return "pending"
	}
	return "pass"
}

var (
	monStartRe = regexp.MustCompile(`"content":"Monitor started \(task (\w+), expires in (\d+)m`) // anchored: output quoted elsewhere in a log must not match
	tsRe       = regexp.MustCompile(`"timestamp":"([^"]+)"`)
	taskIDRe   = regexp.MustCompile(`<task-id>(\w+)</task-id>`)
	monCallRe  = regexp.MustCompile(`"id":"(toolu_\w+)","name":"Monitor","input":\{.*?"description":"((?:[^"\\]|\\.)*)"`)
	toolIDRe   = regexp.MustCompile(`"tool_use_id":"(toolu_\w+)"`)
)

// countMonitors counts Monitor tasks that have started, haven't hit their expiry
// and haven't reported a final <status> notification. Monitor *events* carry no
// <status>, so they don't end a task.
func activeMonitors(buf []byte, now time.Time) []string {
	type mon struct {
		until time.Time
		desc  string
		at    time.Time
	}
	descs := map[string]string{} // tool_use id -> description
	started := map[string]mon{}
	ended := map[string]bool{}
	for _, l := range strings.Split(string(buf), "\n") {
		switch {
		case strings.Contains(l, `"name":"Monitor"`):
			if m := monCallRe.FindStringSubmatch(l); m != nil {
				descs[m[1]] = unescape(m[2])
			}
		case strings.Contains(l, `"content":"Monitor started (`):
			m, t := monStartRe.FindStringSubmatch(l), tsRe.FindStringSubmatch(l)
			if m == nil || t == nil {
				continue
			}
			ts, err := time.Parse(time.RFC3339, t[1])
			if err != nil {
				continue
			}
			mins := 0
			fmt.Sscanf(m[2], "%d", &mins)
			d := ""
			if id := toolIDRe.FindStringSubmatch(l); id != nil {
				d = descs[id[1]]
			}
			started[m[1]] = mon{ts.Add(time.Duration(mins) * time.Minute), d, ts}
		case strings.Contains(l, `"content":"<task-notification>`) && strings.Contains(l, "<status>"):
			if m := taskIDRe.FindStringSubmatch(l); m != nil {
				ended[m[1]] = true
			}
		}
	}
	var live []mon
	for id, m := range started {
		if !ended[id] && now.Before(m.until) {
			live = append(live, m)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].at.Before(live[j].at) })
	out := make([]string, len(live))
	for i, m := range live {
		out[i] = firstOf(m.desc, "(no description)")
	}
	return out
}

var (
	subCallRe   = regexp.MustCompile(`"id":"(toolu_\w+)","name":"(?:Agent|Task)","input":\{(.*)`)
	subDescRe   = regexp.MustCompile(`"description":"((?:[^"\\]|\\.)*)"`)
	subTypeRe   = regexp.MustCompile(`"subagent_type":"((?:[^"\\]|\\.)*)"`)
	notifToolRe = regexp.MustCompile(`<tool-use-id>(toolu_\w+)</tool-use-id>`)
)

type sub struct{ id, typ, desc string }

// activeSubagents lists Agent tool calls that haven't finished, oldest first.
// A foreground call ends with its tool_result; a background one gets an
// "Async agent launched" result first and ends with a <status> task-notification.
// ponytail: a session killed mid-subagent leaves it "running" until it scrolls out of session_tail_kb.
func activeSubagents(buf []byte) []sub {
	var order []string
	desc := map[string]sub{}
	for _, l := range strings.Split(string(buf), "\n") {
		switch {
		case strings.Contains(l, `"type":"tool_use"`):
			if m := subCallRe.FindStringSubmatch(l); m != nil {
				d, t := "", "general-purpose"
				if x := subDescRe.FindStringSubmatch(m[2]); x != nil {
					d = unescape(x[1])
				}
				if x := subTypeRe.FindStringSubmatch(m[2]); x != nil {
					t = unescape(x[1])
				}
				order = append(order, m[1])
				desc[m[1]] = sub{m[1], t, firstOf(d, "(no description)")}
			}
		case strings.Contains(l, `"type":"tool_result"`) && !strings.Contains(l, "Async agent launched"):
			for _, m := range toolIDRe.FindAllStringSubmatch(l, -1) {
				delete(desc, m[1])
			}
		case strings.Contains(l, "<task-notification>") && strings.Contains(l, "<status>"):
			if m := notifToolRe.FindStringSubmatch(l); m != nil {
				delete(desc, m[1])
			}
		}
	}
	var out []sub
	for _, id := range order {
		if d, ok := desc[id]; ok {
			out = append(out, d)
		}
	}
	return out
}

// subAgents turns running Agent calls into cards. Claude Code writes each
// subagent's log to <session>/subagents/agent-<id>.jsonl, next to a .meta.json
// naming the tool call that started it; that log gives the model and phase.
func subAgents(p Agent, session string, subs []sub) []Agent {
	if len(subs) == 0 {
		return nil
	}
	logs := map[string]string{} // tool_use id -> subagent log
	metas, _ := filepath.Glob(projectDir(p.Cwd) + session + "/subagents/*.meta.json")
	for _, f := range metas {
		var meta struct {
			ToolUseID string `json:"toolUseId"`
		}
		if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &meta) == nil {
			logs[meta.ToolUseID] = strings.TrimSuffix(f, ".meta.json") + ".jsonl"
		}
	}
	var out []Agent
	for _, s := range subs {
		k := Agent{Name: s.desc, SubType: s.typ, Parent: p.Name, PaneID: p.PaneID + "/" + s.id, Status: "working",
			Cwd: p.Cwd, Repo: p.Repo, WS: p.WS, Branch: p.Branch, Workspace: p.Workspace, Tab: p.Tab, Log: logs[s.id]}
		if buf, ok := tailFile(k.Log); ok {
			k.Model = lastModel(buf)
			k.Phase, k.Tool = lastPhase(buf)
			k.Session = strings.TrimSuffix(filepath.Base(k.Log), ".jsonl")
		}
		out = append(out, k)
	}
	return out
}

func unescape(s string) string {
	if u, err := strconv.Unquote(`"` + s + `"`); err == nil {
		return u
	}
	return s
}
