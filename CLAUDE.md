# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

This repository is the development repository for the **ws-dev CLI**. It is a single Go binary that runs apps and tasks inside git worktrees (e.g. those created by `claude -w` under `.claude/worktrees/<name>/`). Per-repository settings live in `~/.config/ws-dev/config.yml`, keyed by the repo's git remote, so nothing ws-dev-specific is committed into the repo.

- `cmd/ws-dev/` — entry point (cobra root)
- `internal/` — feature modules (listed below)

## Build and test

```bash
mise exec -- go build -o ws-dev ./cmd/ws-dev
mise exec -- go test ./...
mise exec -- go test ./internal/mcp -run TestSearchLog     # run a single test
```

Go is managed via `mise` (install with `mise use -g go@latest`). `golangci-lint` / `goreleaser` are also pinned in `.mise.toml`, so `mise install` brings everything in.

## Lint / Format

`golangci-lint` handles both fmt and lint (do not use `go fmt` / `gofumpt` directly). The Makefile is the primary entry point:

```bash
make fmt        # golangci-lint fmt ./...
make lint       # golangci-lint run ./...
make vet        # go vet ./...
make check      # fmt + vet + lint + test in one go
```

Run `make check` after code changes before committing. Configuration lives in `.golangci.yml`.

## Release

Distribution is handled by GoReleaser + GitHub Actions. Pushing a `v*` tag triggers `.github/workflows/release.yml`, which builds linux/darwin x amd64/arm64 binaries plus a checksum according to `.goreleaser.yaml` and uploads them to GitHub Releases.

- Windows is not a build target (`procman` depends on `syscall.Setpgid` / `syscall.Kill`).
- Version information is injected into `internal/cmd.{version,commit,date}` via ldflags and can be verified with `ws-dev version`.

Local verification:

```bash
make release-check      # syntax check of .goreleaser.yaml
make release-snapshot   # cross-build into dist/ without a tag (for smoke testing)
make clean              # remove dist/ and the ws-dev binary
```

Release procedure:

```bash
# With main up to date
git tag v0.1.0
git push origin v0.1.0
# -> Actions runs and artifacts are uploaded to Releases
```

Fetching from another machine:

```bash
gh release download v0.1.0 -R hiyamamo/ws-dev -p 'ws-dev_*_linux_amd64.tar.gz'
tar xzf ws-dev_*_linux_amd64.tar.gz && sudo mv ws-dev /usr/local/bin/
```

## Package layout and responsibilities

| Package | Responsibility |
|---------|----------------|
| `internal/cmd` | Cobra subcommand definitions (init/server/status/logs/run/tasks/mcp/update/version). `context.go` resolves the repo config + worktree (replaces the old workspace lookup). `update.go` self-updates from the latest GitHub release (checksum-verified, atomic replace; the API call is authenticated via `$GH_TOKEN` / `$GITHUB_TOKEN` / `gh auth token` to avoid the 60/hour unauthenticated rate limit). |
| `internal/config` | Parses `~/.config/ws-dev/config.yml` (`repos:` map). `Lookup(remote)` / `NormalizeRemote` match repos by git remote regardless of ssh/https form; `Load` rejects two keys normalizing to the same repo (map order would otherwise pick one nondeterministically). `DefaultPath()` honors `$WS_DEV_CONFIG` / `$XDG_CONFIG_HOME`. |
| `internal/git` | Thin git wrappers (`Remote`, `CommonDir`, `Worktrees`) plus pure helpers (`ParseWorktrees`, `ResolveWorktree`, `CurrentWorktree`, `MainRoot`). |
| `internal/tasks` | Runs template-expanded task commands prefixed with `exec_wrapper`, inheriting stdio, with the shared `WS_DEV_*` env. Also home of the shared `Vars` / `Expand` / `Env` helpers used by `procman`. |
| `internal/procman` | Procfile-equivalent parallel process manager. Expands `{{.Worktree}}` etc. via `tasks.Expand`, places each process in its own pgid via setpgid, and cleans up with SIGTERM/SIGKILL (`killAll`, shared by signal, start-failure, and crash paths). An abnormal (non-zero) process exit stops the whole run and makes `Run` return an error, so `ws-dev server` exits non-zero; clean exits leave the rest running. `RunSetup` runs the config's `setup` commands (via `sh -c`) before the processes start. |
| `internal/mcp` | stdio JSON-RPC MCP server implementation (`list_logs` / `tail_log` / `truncate_log` / `search_log`) |

## Agent skills (keep in sync with the CLI)

This repo ships agent skills under `skills/`:

| Skill | Covers |
|-------|--------|
| `skills/ws-dev-cli/` | The terminal CLI: `init` / `server` (+ `-b` / `stop`) / `logs` / `run` / `tasks` / `update`, plus `config.yml` (`processes` / `tasks` / `setup` / `exec_wrapper`, template vars, `WS_DEV_*` env, resolution orders). |
| `skills/ws-dev-logs/` | The MCP log tools (`list_logs` / `tail_log` / `search_log` / `truncate_log`) exposed by `ws-dev mcp`. |

**Whenever you change the user-facing surface, update the matching skill in the
same change.** A change is "user-facing" when it adds/renames/removes a command,
flag, MCP tool, config key, template var, environment variable, or default, or
when it changes a resolution order or any documented behavior. Concretely:

- Code under `internal/cmd/` (commands/flags) → update `skills/ws-dev-cli/SKILL.md`.
- Code under `internal/mcp/` (MCP tools/args) → update `skills/ws-dev-logs/SKILL.md`.
- Code under `internal/config/` (config schema) → update `skills/ws-dev-cli/SKILL.md`.
- Keep `README.md` and this file consistent too.

A `PostToolUse` hook (`.claude/hooks/skill-sync-reminder.sh`, wired in
`.claude/settings.json`) prints a reminder when you edit those packages, so this
doesn't depend on memory alone. The skill is still the source of truth for how
the feature is used — don't just append; revise the affected sections so it reads
as if the feature always existed.

## Key design points

### One server per repository

`ws-dev server` keeps state under `<git-common-dir>/ws-dev/` (i.e. inside the shared `.git`, identical across all worktrees and never committed). If the previous ws-dev process is still alive it stops it with `SIGTERM` (then `SIGKILL` on timeout) before starting the new one. Running multiple worktrees in parallel is not supported (to avoid port conflicts).

- `<git-common-dir>/ws-dev/server.pid` — our own PID plus its start time (`ps -o lstart=`), compared on read so a recycled pid (e.g. after a reboot) is not mistaken for a live server
- `<git-common-dir>/ws-dev/current-worktree` — most recent worktree (used by `ws-dev logs` when omitted)

`ws-dev status` reads these two files (liveness via signal 0) to report whether a server is running and for which worktree; it needs no config entry, so it works in unregistered repos too.

### Background mode

`ws-dev server -b` / `--background` starts the server detached and returns immediately. The foreground process validates the config + worktree (so misconfiguration surfaces synchronously), then re-execs itself without `-b` in a new session (`Setsid`) with the child's combined output redirected to `<log-dir>/server.log`. The detached child runs the normal foreground flow, so it owns the pid file and process lifecycle exactly as a foreground run would; `ws-dev server stop` (or another `ws-dev server`) stops it via the recorded pid like any other run. Per-process logs still go to `<log-dir>/<name>.log`. Because `setup` runs in the foreground flow, it executes in the detached child (its output lands in `server.log`), keeping the `-b` parent's return immediate.

### Worktree resolution

A worktree name is resolved against `git worktree list` by directory basename (`git.ResolveWorktree`); an ambiguous basename is an error. When the name is omitted, commands default to the repository's main worktree (root). `ws-dev logs` still prefers the recorded `current-worktree` (the most recent server run) over the root default, since only one server runs per repo.

### Config lookup by remote

`config.Lookup(remote)` normalizes both the configured `repos:` keys and the actual `git remote get-url origin` via `NormalizeRemote`, so `git@github.com:owner/repo.git` and `https://github.com/owner/repo` match the same entry (canonical form: `github.com/owner/repo`, lowercased). `mcp` does NOT do this lookup — it is config-free and resolves the log dir relative to cwd.

### Log directory resolution order

Command-line flag `--log-dir` -> environment variable `WS_DEV_LOG_DIR` -> `log_dir` in config -> default `log`. The base is the worktree directory. This order is consistent across `server` / `logs` / `mcp`; `mcp` resolves relative to `cwd` (expected to be inside the worktree).

### Template expansion (processes / tasks / setup)

`processes.<name>.cmd`, `tasks.<name>`, and `setup` entries are evaluated as a Go `text/template` (`tasks.Expand`):

- `{{.Worktree}}` — worktree name
- `{{.PortBase}}` — `--port-base` / `$WS_DEV_PORT_BASE` / default 3000
- `{{.Root}}` — absolute main worktree root
- `{{.Dir}}` — absolute worktree directory (process cwd)

After expansion, the string is split into argv via shell-like whitespace splitting (`tasks.fields`). Quoting support is minimal (`"..."` / `'...'` only; no escapes).

### Setup commands

`setup` (a `[]string` on `RepoConfig`) lists commands `ws-dev server` runs before any process starts, via `procman.RunSetup`. Each entry is template-expanded with the same `Vars` as process cmds (`tasks.Expand`) and run with `sh -c` in the worktree dir, stdio inherited, with the same `WS_DEV_*` env. The first non-zero exit aborts the start and is returned (so the server never comes up on a broken environment). Unlike processes/tasks, setup is **not** wrapped by `exec_wrapper`: bootstrap steps like `direnv allow` / `mise trust` must run as-is (wrapping `direnv allow` in `direnv exec .` would be circular). A step needing the toolchain includes the wrapper itself (e.g. `mise exec -- pnpm install`). `RunSetup` is invoked from `runServer` just before `procman.Run`, sharing the same `procman.Opts`.

## Manual verification

```bash
# Use $WS_DEV_CONFIG to point at a throwaway config so ~/.config is untouched.
export WS_DEV_CONFIG=/tmp/ws-config/config.yml

cd /path/to/a/real/repo
/path/to/ws-dev init             # creates the config + prints this repo's key
# Add a `repos:` entry under that key with processes/tasks (and optional setup).

claude -w branch-a               # or: git worktree add .claude/worktrees/branch-a -b branch-a
/path/to/ws-dev server branch-a  # start processes in the worktree (foreground)
/path/to/ws-dev server -b branch-a  # start detached; returns immediately
/path/to/ws-dev logs             # logs of the running/most-recent server
/path/to/ws-dev logs branch-a web -f
/path/to/ws-dev run branch-a console
/path/to/ws-dev server stop
```

MCP standalone test (works inside any worktree, even unconfigured):
```bash
cd .claude/worktrees/branch-a
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | ws-dev mcp
```
