package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Check struct{ Name, State string } // State: pass/fail/pending

type Detail struct {
	At time.Time
	// bead
	BeadTitle, BeadStatus, BeadType string
	BeadDesc                        string
	BeadPrio                        int
	BeadDeps                        []string
	// pr
	PRTitle, PRURL, Review, Mergeable string
	PRNum                             int
	Draft                             bool
	Add, Del, Files                   int
	Checks                            []Check
	// git
	Dirty, Ahead, Behind int
	Commit, CommitAge    string
	// session
	Ctx    int // tokens in context on the last turn
	LastAt time.Time
	Tools  []string // newest last
	Said   string
	Notes  []string // lookups that failed, shown in the drawer instead of hiding them
}

type detailMsg struct {
	pane string
	d    Detail
}

func run(dir, name string, args ...string) string {
	out, _ := runWith(dir, nil, name, args...)
	return out
}

func runWith(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// beadsEnv: bd finds its database through BEADS_DIR, which a plugin pane may not
// inherit from the user's shell. Use the pane's value if set, else a file named
// beads-dir in the plugin config dir, else the common global locations.
func beadsEnv() []string {
	if os.Getenv("BEADS_DIR") != "" {
		return nil
	}
	home, _ := os.UserHomeDir()
	var cands []string
	if cfg := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); cfg != "" {
		if b, err := os.ReadFile(cfg + "/beads-dir"); err == nil {
			cands = append(cands, strings.TrimSpace(string(b)))
		}
	}
	cands = append(cands, home+"/.beads/beadsDB", home+"/.beads")
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return []string{"BEADS_DIR=" + c}
		}
	}
	return nil
}

func fetchDetail(a Agent) Detail {
	d := Detail{At: time.Now()}
	if beadRe.MatchString(a.Name) {
		var bs []struct {
			Title    string `json:"title"`
			Desc     string `json:"description"`
			Status   string `json:"status"`
			Type     string `json:"issue_type"`
			Priority int    `json:"priority"`
			Deps     []struct {
				ID, Title string
			} `json:"dependencies"`
		}
		raw, _ := runWith(a.Cwd, beadsEnv(), "bd", "show", a.Name, "--json")
		if json.Unmarshal([]byte(raw), &bs) == nil && len(bs) > 0 {
			b := bs[0]
			d.BeadTitle, d.BeadStatus, d.BeadType, d.BeadPrio, d.BeadDesc = b.Title, b.Status, b.Type, b.Priority, strings.TrimSpace(b.Desc)
			for _, x := range b.Deps {
				d.BeadDeps = append(d.BeadDeps, x.ID+" "+x.Title)
			}
		}
	}
	var pr struct {
		Number                                int
		Title, URL, ReviewDecision, Mergeable string
		IsDraft                               bool
		Additions, Deletions, ChangedFiles    int
		Checks                                []struct {
			Name, Context, Status, Conclusion, State string
		} `json:"statusCheckRollup"`
	}
	if json.Unmarshal([]byte(run(a.Cwd, "gh", "pr", "view", "--json",
		"title,url,isDraft,reviewDecision,mergeable,additions,deletions,changedFiles,statusCheckRollup")), &pr) == nil {
		d.PRNum = pr.Number
		d.PRTitle, d.PRURL, d.Review, d.Mergeable, d.Draft = pr.Title, pr.URL, pr.ReviewDecision, pr.Mergeable, pr.IsDraft
		d.Add, d.Del, d.Files = pr.Additions, pr.Deletions, pr.ChangedFiles
		for _, c := range pr.Checks {
			d.Checks = append(d.Checks, Check{firstOf(c.Name, c.Context), collapseCI([][2]string{{c.Status + c.State, c.Conclusion}})})
		}
	}
	if s := run(a.Cwd, "git", "status", "--porcelain"); s != "" {
		d.Dirty = len(strings.Split(s, "\n"))
	}
	base := "@{u}"
	if run(a.Cwd, "git", "rev-parse", "--verify", "-q", "@{u}") == "" {
		base = "origin/main"
	}
	fmt.Sscanf(run(a.Cwd, "git", "rev-list", "--left-right", "--count", base+"...HEAD"), "%d %d", &d.Behind, &d.Ahead)
	if c := strings.SplitN(run(a.Cwd, "git", "log", "-1", "--format=%s|%cr"), "|", 2); len(c) == 2 {
		d.Commit, d.CommitAge = c[0], c[1]
	}
	parseSession(sessionTail(a.Cwd, a.Session), &d)
	return d
}

func firstOf(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// parseSession reads the tail of a Claude session log: recent tool trail, last
// assistant text, context size and time of last activity.
func parseSession(buf []byte, d *Detail) {
	type block struct {
		Type, Text, Name string
		Input            map[string]any
	}
	for _, l := range strings.Split(string(buf), "\n") {
		var e struct {
			Timestamp string
			Message   struct {
				Role    string
				Content json.RawMessage
				Usage   struct {
					In, Create, Read int `json:"-"`
					InputTokens      int `json:"input_tokens"`
					CacheCreate      int `json:"cache_creation_input_tokens"`
					CacheRead        int `json:"cache_read_input_tokens"`
				}
			}
		}
		if json.Unmarshal([]byte(l), &e) != nil || e.Message.Role == "" {
			continue // partial first line or non-message entry
		}
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			d.LastAt = t
		}
		if u := e.Message.Usage; e.Message.Role == "assistant" && u.InputTokens+u.CacheCreate+u.CacheRead > 0 {
			d.Ctx = u.InputTokens + u.CacheCreate + u.CacheRead
		}
		var bs []block
		if json.Unmarshal(e.Message.Content, &bs) != nil || e.Message.Role != "assistant" {
			continue
		}
		for _, b := range bs {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					d.Said = strings.TrimSpace(b.Text)
				}
			case "tool_use":
				d.Tools = append(d.Tools, toolLabel(b.Name, b.Input))
			}
		}
	}
	if len(d.Tools) > 6 {
		d.Tools = d.Tools[len(d.Tools)-6:]
	}
}

func toolLabel(name string, in map[string]any) string {
	for _, k := range []string{"file_path", "command", "pattern", "description", "path", "url"} {
		if v, ok := in[k].(string); ok && v != "" {
			v = strings.Join(strings.Fields(v), " ")
			return name + " " + trunc(v, 48)
		}
	}
	return name
}

// ---- rendering ----

func wrap(s string, w int) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			if line != "" && len([]rune(line))+1+len([]rune(word)) > w {
				out = append(out, line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		out = append(out, line)
	}
	return out
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

func ciDot(s string) string {
	switch s {
	case "pass":
		return c(42, "✓")
	case "fail":
		return c(205, "✗")
	}
	return c(214, "●")
}

// drawer renders the verbose view for one agent as a bordered box of width w.
func (m model) drawer(a Agent, w int) []string {
	in := w - 4
	d, ok := m.details[a.PaneID]
	var rows [][2]string // label, value (may contain newlines)
	add := func(label, v string) { rows = append(rows, [2]string{label, v}) }
	if !ok {
		add("", "loading details…")
	} else {
		if a.Name != "" && d.BeadTitle != "" {
			v := fmt.Sprintf("%s  %s", a.Name, d.BeadTitle)
			v += fmt.Sprintf("\nP%d · %s · %s", d.BeadPrio, d.BeadType, d.BeadStatus)
			for _, dep := range d.BeadDeps {
				v += "\ndepends on " + dep
			}
			add("BEAD", v)
			if d.BeadDesc != "" {
				add("DESCRIPTION", d.BeadDesc)
			}
		}
		if d.PRTitle != "" {
			state := strings.ToLower(firstOf(d.Review, "no review yet"))
			if d.Draft {
				state = "draft · " + state
			}
			v := fmt.Sprintf("#%d  %s\n%s · +%d −%d in %d files · %s", d.PRNum, d.PRTitle,
				state, d.Add, d.Del, d.Files, strings.ToLower(d.Mergeable))
			add("PR", v+"\n"+d.PRURL)
			var cs []string
			for _, ch := range d.Checks {
				cs = append(cs, ciDot(ch.State)+" "+ch.Name)
			}
			if len(cs) > 0 {
				add("CHECKS", "\x00"+strings.Join(cs, "\x00")) // \x00 marks pre-split items: wrap between, not inside
			}
		}
		git := fmt.Sprintf("%s · %d uncommitted · ↑%d ahead ↓%d behind", a.Branch, d.Dirty, d.Ahead, d.Behind)
		if d.Commit != "" {
			git += fmt.Sprintf("\nlast commit: %s (%s)", d.Commit, d.CommitAge)
		}
		add("GIT", git)
		sess := fmt.Sprintf("%s · context ~%dk tokens · last activity %s\nsession %s", firstOf(a.Model, "model unknown"), d.Ctx/1000, ago(d.LastAt), a.Session)
		add("SESSION", sess)
		if len(d.Tools) > 0 {
			add("RECENT", strings.Join(d.Tools, "\n"))
		}
		if d.Said != "" {
			add("SAID", trunc(strings.Join(strings.Fields(d.Said), " "), 500))
		}
	}
	add("HERDR", fmt.Sprintf("pane %s · workspace %s (%s) · tab %s\n%s\n%s", a.PaneID, a.Workspace, a.Repo, a.Tab, a.Title, a.Cwd))
	title := " ▾ details · " + a.Name + " "
	out := []string{c(214, "╭─"+title) + c(214, strings.Repeat("─", max(w-3-len([]rune(title)), 0))+"╮")}
	const lw, maxDesc = 12, 18
	for _, r := range rows {
		ls := wrapRow(r[1], in-lw)
		if r[0] == "DESCRIPTION" && len(ls) > maxDesc {
			ls = append(ls[:maxDesc], c(240, fmt.Sprintf("… %d more lines (bd show %s)", len(ls)-maxDesc, a.Name)))
		}
		for i, l := range ls {
			label := ""
			if i == 0 {
				label = c(110, r[0])
			}
			out = append(out, c(214, "│ ")+padTo(label, lw)+padTo(l, in-lw)+c(214, " │"))
		}
	}
	return append(out, c(214, "╰"+strings.Repeat("─", w-2)+"╯"))
}

// wrapRow wraps plain text, or packs \x00-separated items without splitting any item.
func wrapRow(v string, w int) []string {
	if !strings.HasPrefix(v, "\x00") {
		return wrap(v, w)
	}
	var out []string
	line := ""
	for _, it := range strings.Split(v, "\x00")[1:] {
		if line != "" && vlen(line)+3+vlen(it) > w {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += "   "
		}
		line += it
	}
	return append(out, line)
}
