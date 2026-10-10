# Contributing

Thanks for taking a look. Issues and pull requests are welcome. This is a small,
single-maintainer project: PRs are reviewed and merged by [@kristaxox](https://github.com/kristaxox).

## Before you start

- For anything bigger than a bug fix or a small tweak, open an issue first so we can
  agree on the approach before you spend time on it.
- The dashboard reads Claude Code's session logs, so it is **Claude Code only** today.
  Support for another agent is welcome, but it needs a reader for that agent's session
  format. Please discuss it in an issue first.
- Keep it lazy: prefer the standard library and the existing helpers over new
  dependencies or abstractions.

## Reporting a bug

Include:

- `herdr --version`, your OS, and your terminal.
- Which view you were in, and what you expected versus what you saw.
- Whether the agent in question is a Claude Code agent.
- For a missing or wrong field (model, phase, bead, PR, CI), the output of the matching
  command from the agent's directory: `bd show <id> --json`, `gh pr view --json number,state`,
  `git branch --show-current`.

Please do not paste session logs, tokens or private repository names. Redact anything
you would not post publicly, and use demo data for screenshots.

## Making a change

You need Go 1.26 or newer and a Herdr 0.9.0+ install. Optional: `gh` and `bd` to see PR,
CI and bead fields.

```sh
go build -o bin/dashboard ./cmd/dashboard
go vet ./... && go test ./...
gofmt -l .                     # should print nothing
herdr plugin link .            # once; re-run after manifest changes
herdr plugin action invoke kristaxox.agent-dashboard.open
```

`herdr plugin link` does not build, and an open dashboard tab keeps running the old
binary, so rebuild and reopen the tab after each change.

- Branch from `main` and open a pull request; direct pushes to `main` are not accepted.
- Keep each PR focused. Match the surrounding code's style and comment density.
- Add or update a test for non-trivial logic. The existing tests are small and
  stdlib-only (`go test`), so follow that pattern.
- If you change a layout, regenerate the README mocks and include them:
  `MOCK_OUT=/some/dir go test ./cmd/dashboard -run GenMocks`.
- If you add a setting, put it in `config.go`, `config.example.toml` and the README
  config table.
- Commit messages: a short imperative summary line, then a blank line and the reason for
  the change if it is not obvious.

By contributing you agree that your work is licensed under the [MIT License](LICENSE).
