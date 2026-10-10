# herdr-agent-dashboard

A full-tab dashboard for the Claude Code agents running in [Herdr](https://herdr.dev).
It draws your agents as a topology: each repo's primary-checkout agent (for example an
orchestrator) on top, the agents in its linked worktrees hanging below, one card per
agent. Each card shows status, what the agent is doing right now, model, git branch,
PR and CI state, and its bead. Click a card to open a verbose details drawer;
double-click to jump to that agent's pane.

> Status: early. macOS and Linux.

> **Claude Code only, for now.** Model, activity phase, live monitors, context size, the
> recent-tools list and the agent's last message are all read from Claude Code's session
> logs (`~/.claude/projects/...`). Other agents Herdr knows about should still appear with
> Herdr's status, branch and PR / CI, but none of the log-derived fields. That has not been
> tested. Support for another agent means a reader for its session format; see
> [Limitations](#limitations-and-ideas).

## Views

Switch with `v` / `Tab`, or press `1`-`5`.

| Key | View | What it is |
| --- | --- | --- |
| `1` | `tree` | Repo header with agents hanging below (`├─` / `└─`). |
| `2` | `graph` (default) | Top-down topology. Owner on top, a bus line, drops into cards. |
| `3` | `grid` | Bordered cards in columns, working agents first. |
| `4` | `table` | One dense line per agent. |
| `5` | `fan` | The original left-to-right layout: repo hub fanning out to nodes. |

Each mock below is rendered by the real code from demo data in a 96-column terminal (`card_min_width = 30`). `▸` is the expand triangle, `◉ monitoring ×1` is an agent herdr reports as idle but with a live monitor, and `#9 ●` is a PR whose CI is still running.

### 1. tree

A repo header with its agents hanging below. The densest view that still shows branch, PR and bead.

```
 HERDR AGENT DASHBOARD  ACTIVE 1/5   BLOCKED 1   CI FAIL 1   view: tree


◉ demo  4 agents
 ├─ ▸ ⬢ orchestrator     idle      opus-5-5
 │     main  no PR
 ├─ ▸ ⬢ abc-123          working   opus-5-5  ✎ writing Edit
 │     feat/abc-123-login  #5 ✓  bead abc-123
 ├─ ▸ ⬢ abc-456          blocked   sonnet-5-5  ⚙ tool Bash
 │     feat/abc-456-api  #7 ✗  bead abc-456
 └─ ▸ ⬢ abc-789          idle      sonnet-5-5  ◉ monitoring ×1
       feat/abc-789-docs  #9 ●  bead abc-789

◉ widgets  1 agents
 └─ ▸ ⬢ xyz-001          done      opus-5-5
       main  no PR  bead xyz-001
```

### 2. graph (default)

Top-down topology. The primary-checkout agent (here, an orchestrator) sits on top, a bus line fans out to its workers, and a repo with no owner gets a hub box instead. A pink connector means something under that repo is blocked or has failing CI. The selected card has the double border.

```
 HERDR AGENT DASHBOARD  ACTIVE 1/5   BLOCKED 1   CI FAIL 1   view: graph

                                ╭─ demo ─────────────────── ▸ ╮
                                │ ⬢ orchestrator    idle      │
                                │ opus-5-5                    │
                                │ main                        │
                                │ no PR                       │
                                ╰─────────────────────────────╯
                                               │
               ╭───────────────────────────────┼───────────────────────────────╮
               │                               │                               │
╔═ ▶ demo ═════════════════ ▸ ╗ ╭─ demo ─────────────────── ▸ ╮ ╭─ demo ─────────────────── ▸ ╮
║ ⬢ abc-123         working   ║ │ ⬢ abc-456         blocked   │ │ ⬢ abc-789         idle      │
║ opus-5-5  ✎ writing Edit    ║ │ sonnet-5-5  ⚙ tool Bash     │ │ sonnet-5-5  ◉ monitoring ×1 │
║ feat/abc-123-login          ║ │ feat/abc-456-api            │ │ feat/abc-789-docs           │
║ #5 ✓  bead abc-123          ║ │ #7 ✗  bead abc-456          │ │ #9 ●  bead abc-789          │
╚═════════════════════════════╝ ╰─────────────────────────────╯ ╰─────────────────────────────╯

    ╭────────────────────╮
    │ ◉ widgets 1        │
    ╰────────────────────╯
               │
               │
╭─ widgets ──────────────── ▸ ╮
│ ⬢ xyz-001         done      │
│ opus-5-5                    │
│ main                        │
│ no PR  bead xyz-001         │
╰─────────────────────────────╯
```

### 3. grid

Bordered cards in columns, as many per row as fit.

```
 HERDR AGENT DASHBOARD  ACTIVE 1/5   BLOCKED 1   CI FAIL 1   view: grid

╭─ demo ─────────────────── ▸ ╮ ╔═ ▶ demo ═════════════════ ▸ ╗ ╭─ demo ─────────────────── ▸ ╮
│ ⬢ orchestrator    idle      │ ║ ⬢ abc-123         working   ║ │ ⬢ abc-456         blocked   │
│ opus-5-5                    │ ║ opus-5-5  ✎ writing Edit    ║ │ sonnet-5-5  ⚙ tool Bash     │
│ main                        │ ║ feat/abc-123-login          ║ │ feat/abc-456-api            │
│ no PR                       │ ║ #5 ✓  bead abc-123          ║ │ #7 ✗  bead abc-456          │
╰─────────────────────────────╯ ╚═════════════════════════════╝ ╰─────────────────────────────╯
╭─ demo ─────────────────── ▸ ╮ ╭─ widgets ──────────────── ▸ ╮
│ ⬢ abc-789         idle      │ │ ⬢ xyz-001         done      │
│ sonnet-5-5  ◉ monitoring ×1 │ │ opus-5-5                    │
│ feat/abc-789-docs           │ │ main                        │
│ #9 ●  bead abc-789          │ │ no PR  bead xyz-001         │
╰─────────────────────────────╯ ╰─────────────────────────────╯
```

### 4. table

One line per agent, for lots of agents.

```
 HERDR AGENT DASHBOARD  ACTIVE 1/5   BLOCKED 1   CI FAIL 1   view: table

     AGENT           STATUS   PHASE                 MODEL           REPO          BRANCH                              PR / CI

 ▸ ⬢ orchestrator    idle                           opus-5-5        demo          main                                no PR
 ▸ ⬢ abc-123         working  ✎ writing Edit        opus-5-5        demo          feat/abc-123-login                  #5 ✓
 ▸ ⬢ abc-456         blocked  ⚙ tool Bash           sonnet-5-5      demo          feat/abc-456-api                    #7 ✗
 ▸ ⬢ abc-789         idle     ◉ monitoring ×1       sonnet-5-5      demo          feat/abc-789-docs                   #9 ●
 ▸ ⬢ xyz-001         done                           opus-5-5        widgets       main                                no PR
```

### 5. fan

The original left-to-right layout: a repo hub fanning out to its agents. It has no details drawer.

```
 HERDR AGENT DASHBOARD  ACTIVE 1/5   BLOCKED 1   CI FAIL 1   view: fan

                       ╭── ⬢ orchestrator     idle      opus-5-5
                       │   main  no PR
╭────────────────────╮ ├── ⬢ abc-123          working   opus-5-5  ✎ writing Edit
│ ◉ demo 4           ├─┤   feat/abc-123-login  #5 ✓  bead abc-123
╰────────────────────╯ ├── ⬢ abc-456          blocked   sonnet-5-5  ⚙ tool Bash
                       │   feat/abc-456-api  #7 ✗  bead abc-456
                       ╰── ⬢ abc-789          idle      sonnet-5-5  ◉ monitoring ×1
                           feat/abc-789-docs  #9 ●  bead abc-789

╭────────────────────╮
│ ◉ widgets 1        ├──── ⬢ xyz-001          done      opus-5-5
╰────────────────────╯     main  no PR  bead xyz-001
```

## Install

Requirements: Herdr 0.9.0 or newer, Go 1.26+ (the install builds the binary), `git`.
Optional: [`gh`](https://cli.github.com) for PR and CI, [`bd`](https://github.com/gastownhall/beads)
for bead titles and descriptions. Anything missing just leaves that field blank.

```sh
herdr plugin install kristaxox/herdr-agent-dashboard
herdr plugin action invoke kristaxox.agent-dashboard.open
```

The open action creates a dedicated dashboard tab. Remove it with
`herdr plugin uninstall kristaxox.agent-dashboard`.

## Using it

| Input | Does |
| --- | --- |
| click a card | select it and toggle its details drawer |
| double-click a card, or `Enter` | jump to the agent's pane and close the dashboard |
| click the `▸`/`▾` triangle, or `e` / `Space` | toggle the selected card's drawer |
| `E` | open all drawers, or close them all if all are open |
| arrows / `hjkl` | move the selection (up/down move a whole card row in `graph` and `grid`) |
| `v` / `Tab`, `1`-`5` | switch view |
| `q` / `Esc` | close |

The header shows `ACTIVE n/total`, how many agents are blocked, and how many PRs have
failing CI.

### The details drawer

Opening a card shows a full-width drawer under its row:

- **BEAD**: id, title, priority, type, status, and each dependency with its title.
- **DESCRIPTION**: the bead's description (capped, with a pointer to `bd show` for the rest).
- **PR**: number, title, review state or draft, `+/-` lines and file count, mergeable state, URL.
- **CHECKS**: every CI check by name with a pass / fail / pending mark.
- **GIT**: branch, uncommitted file count, commits ahead / behind, last commit and its age.
- **SESSION**: model, context size in tokens, time since last activity, session id.
- **RECENT**: the last few tool calls, each with its target (file, command or pattern).
- **SAID**: the agent's last message.
- **HERDR**: pane, workspace and tab ids, terminal title, working directory.

## The workflow this was built around

The dashboard is not tied to one setup, but its defaults assume this one. If yours
differs, see the [config](#configuration) notes at the end of this section.

- **One orchestrator** agent runs in a repo's primary checkout (the main clone).
- **One agent per unit of work.** For each task the orchestrator:
  1. creates a bead with `bd create`, putting the full brief in the body;
  2. creates a worktree and workspace with `herdr worktree create --branch <prefix>/<bead>-<slug>`;
  3. starts a Claude agent in that pane with `herdr agent start <bead-id> --kind claude`,
     so **the agent's name is the bead id**;
  4. prompts the agent to read its bead and get on with it.
- Each worker therefore has its own linked worktree, its own branch (the bead id is in
  the branch name) and usually its own PR.
- **Beads are the brief and the tracker, not the org chart.** They are not created with
  `--parent` or grouped under an epic, and a plan bead is referenced from the text of the
  beads that follow it. The occasional `discovered-from` dependency links a follow-up to
  where it was found. Beads live in one global database, pointed at by `BEADS_DIR`.

What that means for the dashboard:

| In that setup | The dashboard |
| --- | --- |
| orchestrator in the primary checkout, workers in linked worktrees | draws the orchestrator above its workers (`owner_mode = "primary"`), since bd holds no ownership |
| agent name = bead id | tags each card with its bead and loads its title, description and dependencies |
| one global bead database | needs `BEADS_DIR` (or `beads_dir`), which a plugin pane does not inherit |
| workers run long jobs behind monitors | shows `◉ monitoring` on agents that herdr reports as idle |

If you do things differently:

- Different agent naming: change `bead_pattern`, or set it to something that never
  matches to hide bead fields.
- No orchestrator or no worktrees: set `owner_mode = "none"` for a plain hub box per repo.
- An agent that works several beads is only matched on the one in its name.

## Where each field comes from

| Field | Source | Refresh |
| --- | --- | --- |
| name, status, pane, workspace | `herdr api snapshot` | every `poll_seconds` |
| activity (thinking / writing / tool / monitoring) | tail of the Claude session log | every `poll_seconds` |
| model | last `"model"` in the session log | every `poll_seconds` |
| branch | `git branch --show-current` in the agent's cwd | every `poll_seconds` |
| PR number, CI | `gh pr view --json ...` in the agent's cwd, run in the background | every `pr_refresh_seconds` |
| bead id | the agent's name, matched against `bead_pattern` | n/a |
| bead title, description, dependencies | `bd show <id> --json` | when a drawer is open, every `detail_refresh_seconds` |
| drawer PR / checks / git / session | `gh`, `git`, session log | when a drawer is open, every `detail_refresh_seconds` |

Closed cards cost nothing beyond the cheap per-poll fields.

## Configuration

Optional. Create `config.toml` in the plugin config directory:

```sh
herdr plugin config-dir kristaxox.agent-dashboard
```

[`config.example.toml`](config.example.toml) lists every key with its default. Set
`AGENT_DASHBOARD_CONFIG` to read a config from another path. A missing or unreadable
file leaves the defaults; an invalid `bead_pattern` or `default_view` falls back to the
default.

| Key | Default | Meaning |
| --- | --- | --- |
| `default_view` | `"graph"` | `tree`, `graph`, `grid`, `table` or `fan` |
| `poll_seconds` | `2` | agent list, branch, model and activity refresh |
| `pr_refresh_seconds` | `30` | PR and CI refresh (one `gh` call per agent) |
| `detail_refresh_seconds` | `15` | refresh for open drawers |
| `double_click_ms` | `400` | double-click window |
| `expand_on_click` | `true` | single click toggles the drawer; `false` = click only selects |
| `bead_pattern` | `'^[a-z]+-[a-z0-9]+$'` | which agent names are bead ids |
| `beads_dir` | unset | `BEADS_DIR` for `bd`; a plugin pane does not inherit it from your shell |
| `extra_path` | `~/.local/bin`, `~/go/bin`, `/opt/homebrew/bin`, `/usr/local/bin` | appended to `PATH` |
| `owner_mode` | `"primary"` | `primary`: primary-checkout agents own linked-worktree agents; `none`: plain repo hub |
| `session_tail_kb` | `1024` | how much of each session log to read |
| `card_min_width` | `40` | cards per row = terminal width / this |
| `description_lines` | `18` | bead description lines in the drawer |
| `said_chars` | `500` | length of the agent's last message |
| `recent_tools` | `6` | tool calls listed under RECENT |

Colors, status glyphs and the double-click window's feel are not themeable yet.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| No bead title or description | `bd` not on `PATH` or `BEADS_DIR` not resolved. Set `beads_dir` / `extra_path`. Test with `env -u BEADS_DIR bd show <id>` from an agent's directory. |
| Model, activity or monitors blank | Not a Claude agent, or the session log is not at the expected path. |
| PR column says "no PR" | `gh` not installed or not logged in, or the branch has no PR. |
| Changes do not show up | Rebuild the binary, then close and reopen the tab. |
| Action not found | `herdr plugin link .` (or install) again after changing the manifest. |

## Limitations and ideas

- Claude Code agents only; other agents show herdr's status but no model, phase or session rows.
- Polling, not `events.subscribe`.
- No filtering or search yet, and no left-hand summary rail.
- `fan` has no details drawer.
- Possible: dotted edges for bead dependencies, token cost, per-agent state history, themes.

## Development

```sh
go build -o bin/dashboard ./cmd/dashboard
go test ./...
MOCK_OUT=/some/dir go test ./cmd/dashboard -run GenMocks   # regenerate the README view mocks
herdr plugin link .      # once; re-run after manifest changes
```

| File | Role |
| --- | --- |
| `herdr-plugin.toml` | manifest: tab pane, open action, build step |
| `cmd/dashboard/main.go` | model, update loop, input |
| `cmd/dashboard/frame.go` | header, footer, scrolling, hit-testing |
| `cmd/dashboard/views.go` | tree, graph, grid, table and fan renderers, cards |
| `cmd/dashboard/data.go` | snapshot, git / model / phase / monitor enrichment, `gh` PR + CI |
| `cmd/dashboard/detail.go` | drawer data (bead, PR, git, session) and rendering |
| `cmd/dashboard/config.go` | config file |

## Contributing

Issues and PRs are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
