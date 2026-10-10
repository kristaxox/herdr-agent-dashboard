package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

type Agent struct {
	Name, PaneID, Status, Cwd, Session, Repo string
	WS                                       int  // workspace order
	Primary                                  bool // lives in the repo's primary checkout, i.e. owns the linked worktrees
	Branch, Model, Phase, Tool               string
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
	buf := sessionTail(a.Cwd, a.Session)
	a.Model = lastModel(buf)
	a.Phase, a.Tool = "", ""
	if a.Status == "working" {
		a.Phase, a.Tool = lastPhase(buf)
	}
}

var modelRe = regexp.MustCompile(`"model":"([^"]+)"`)

// claude slug: every / and . in cwd becomes -
func claudeSlug(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
}

func sessionTail(cwd, session string) []byte {
	home, _ := os.UserHomeDir()
	f, err := os.Open(home + "/.claude/projects/" + claudeSlug(cwd) + "/" + session + ".jsonl")
	if err != nil {
		return nil
	}
	defer f.Close()
	st, _ := f.Stat()
	const tail = 256 << 10 // ponytail: last 256KB only, widen if a model line is ever missed
	off := max(st.Size()-tail, 0)
	buf := make([]byte, st.Size()-off)
	f.ReadAt(buf, off)
	return buf
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
