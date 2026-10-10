# herdr-claude-dashboard

A full-tab dashboard for the Claude Code agents running in [Herdr](https://herdr.dev).
It draws your agents as a topology: each repo's primary-checkout agent (for example an
orchestrator) on top, the agents in its linked worktrees hanging below, one card per
agent. Each card shows status, what the agent is doing right now, model, git branch,
PR and CI state, and its bead. Click a card to open a verbose details drawer;
double-click to jump to that agent's pane.

> Status: early. Claude Code agents only. macOS and Linux.

## Views

Switch with `v` / `Tab`, or press `1`-`5`.

| Key | View | What it is |
| --- | --- | --- |
| `1` | `tree` | Repo header with agents hanging below (`├─` / `└─`). |
| `2` | `graph` (default) | Top-down topology. Owner on top, a bus line, drops into cards. |
| `3` | `grid` | Bordered cards in columns, working agents first. |
| `4` | `table` | One dense line per agent. |
| `5` | `fan` | The original left-to-right layout: repo hub fanning out to nodes. |

```
              ╭─ demo ─────────────────── ▸ ╮
              │ ⬢ orchestrator   idle       │
              │ main                        │
              ╰─────────────────────────────╯
                            │
        ╭───────────────────┴───────────────────╮
        │                                       │
╭─ demo ───────────── ▸ ╮               ╭─ demo ───────────── ▾ ╮
│ ⬢ abc-123   working   │               │ ⬢ abc-456   blocked   │
│ opus-5-5  ✎ writing   │               │ sonnet-5-5            │
│ feat/abc-123          │               │ feat/abc-456          │
│ #5 ✓  bead abc-123    │               │ #7 ✗  bead abc-456    │
╰───────────────────────╯               ╰───────────────────────╯
```

A pink connector means something under that repo is blocked or has failing CI.

## Install

Requirements: Herdr 0.9.0 or newer, Go 1.26+ (the install builds the binary), `git`.
Optional: [`gh`](https://cli.github.com) for PR and CI, [`bd`](https://github.com/gastownhall/beads)
for bead titles and descriptions. Anything missing just leaves that field blank.

```sh
herdr plugin install kristaxox/herdr-claude-dashboard
herdr plugin action invoke kristaxox.claude-dashboard.open
```

The open action creates a dedicated dashboard tab. Remove it with
`herdr plugin uninstall kristaxox.claude-dashboard`.

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
herdr plugin config-dir kristaxox.claude-dashboard
```

[`config.example.toml`](config.example.toml) lists every key with its default. Set
`CLAUDE_DASHBOARD_CONFIG` to read a config from another path. A missing or unreadable
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
| `beads_dir` | unset | `BEADS_DIR` for `bd`, see below |
| `extra_path` | `~/.local/bin`, `~/go/bin`, `/opt/homebrew/bin`, `/usr/local/bin` | appended to `PATH` |
| `owner_mode` | `"primary"` | `primary`: primary-checkout agents own linked-worktree agents; `none`: plain repo hub |
| `session_tail_kb` | `1024` | how much of each session log to read |
| `card_min_width` | `40` | cards per row = terminal width / this |
| `description_lines` | `18` | bead description lines in the drawer |
| `said_chars` | `500` | length of the agent's last message |
| `recent_tools` | `6` | tool calls listed under RECENT |

Colors, status glyphs and the double-click window's feel are not themeable yet.

## How it works, and what we learned

Notes for anyone building a similar plugin, or debugging this one.

**A plugin pane is a terminal program.** Herdr plugins have no native UI, so this is a
TUI (Go, Bubble Tea v2). The manifest declares one pane with `placement = "tab"` and a
`global` action that opens it. Jumping runs `herdr agent focus <pane>` and exits, which
closes the tab. `herdr plugin link` does not run the build step, so rebuild with
`go build -o bin/dashboard ./cmd/dashboard` after changing code, and reopen the tab,
because an open tab keeps running the old binary.

**The pane does not inherit your shell environment.** Two things bit us:

- `PATH` can be thin, so `bd`, `gh` and `git` are not found. The dashboard appends
  `extra_path` at startup.
- `bd` locates its database through `BEADS_DIR`, usually exported from a shell profile.
  Without it, `bd show` fails with exit 1 ("no beads database found") from any directory,
  so bead fields silently stay empty. The dashboard uses `$BEADS_DIR` if present, else
  `beads_dir` from the config, else `~/.beads/beadsDB`, then `~/.beads`.

**Herdr's status and what the agent is doing are different things.** Herdr reports
`idle`, `working`, `blocked` or `done`. For `working` agents the dashboard reads the
newest entry of the Claude session log
(`~/.claude/projects/<cwd with / and . replaced by ->/<session id>.jsonl`). Each log line
holds one content block, so the last line tells you the phase: a `thinking` block is
thinking; a `text` block or an `Edit` / `Write` / `NotebookEdit` call is writing; a
`Monitor` / `ScheduleWakeup` / `TaskOutput` call is monitoring; any other tool call is a
tool; a `tool_result` means the agent is about to think.

**Idle can still mean busy.** An agent waiting on a background `Monitor` is `idle` to
Herdr. The dashboard counts live monitors from the log: a `Monitor started (task ...,
expires in Nm ...)` tool result starts one, and it ends at its expiry time or when a
`<task-notification>` for that task carries a `<status>`. Monitor *event* notifications
have no status and do not end it. The patterns are anchored on the tool-result content so
a monitor line that merely appears in quoted output is not counted.

**Ownership is not in bd.** Beads record `created_by` and dependencies, not who spawned
a bead, and agents created through `herdr worktree create` leave no parent link. Herdr
does know which workspace is a repo's primary checkout and which are linked worktrees of
it, so the graph view treats the primary-checkout agent as the owner of the linked
worktrees' agents. That matches the common "orchestrator in the main checkout, workers in
worktrees" setup. Repos without that split fall back to a plain hub box. Set
`owner_mode = "none"` to always use the hub.

**Bead id = agent name** is a convention, not something Herdr or bd enforces. Change
`bead_pattern` if your agents are named differently. An agent that owns several beads is
only matched on the one in its name.

**CI is collapsed to one mark.** Any failed / cancelled / timed-out / action-required
check is `fail`; otherwise any unfinished check is `pending`; otherwise `pass`; no checks
is `none`. Both check runs and commit statuses are understood.

**`gh` is the expensive call.** PR and CI are fetched in parallel in the background and
cached, and the UI never waits on them (a field still loading is blank). Raise
`pr_refresh_seconds` if you run many agents or hit API limits.

**The session log format is Claude Code's, not a stable API.** Model, phase, monitors,
context size and the drawer's session rows all depend on it. If a Claude Code update
changes the format, those fields degrade to blank rather than breaking the dashboard.

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

## Listing on the Herdr marketplace

Per the [marketplace docs](https://herdr.dev/docs/marketplace/), a repository is indexed
automatically (refreshing every 30 minutes) when it is public, not a fork or archived,
has the GitHub topic `herdr-plugin`, and its default branch contains a parseable
`herdr-plugin.toml`. This repo's manifest declares the required `id`, `name`, `version`
and `min_herdr_version`, plus `platforms`.

```sh
gh repo edit --add-topic herdr-plugin
```

Bump `version` in `herdr-plugin.toml` for each release; the listing shows it.

## License

[MIT](LICENSE)
