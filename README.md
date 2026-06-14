# ws-dev

Run apps and tasks inside git worktrees. `ws-dev` starts a Procfile-like set of
processes, runs user-defined tasks, and exposes a built-in MCP server for log
operations — all scoped to a git worktree. Per-repository settings live in a
single `~/.config/ws-dev/config.yml`, keyed by the repo's git remote, so nothing
ws-dev-specific has to be committed into the repo itself.

This pairs naturally with `claude -w`, which creates worktrees under
`.claude/worktrees/<name>/`.

## Install

```bash
go install github.com/hiyamamo/ws-dev/cmd/ws-dev@latest
```

This installs to `$(go env GOPATH)/bin` (or `$GOBIN`). Make sure that directory
comes **before** any older copy on your `PATH` — see "Verify the install" below.

Or download a prebuilt binary from GitHub Releases with `gh`:

```bash
# Example for linux / amd64. Substitute OS / arch as needed (linux|darwin x amd64|arm64).
gh release download -R hiyamamo/ws-dev -p 'ws-dev_*_linux_amd64.tar.gz'
tar xzf ws-dev_*_linux_amd64.tar.gz
sudo mv ws-dev /usr/local/bin/
```

To fetch a specific version, specify the tag like `gh release download v0.1.0 -R hiyamamo/ws-dev -p '...'`.

Or build from source:

```bash
go build -o ws-dev ./cmd/ws-dev
```

### Verify the install

```bash
which -a ws-dev      # which binary actually runs (and any shadowing copies)
ws-dev version       # should print the version you just installed
```

If `ws-dev version` shows an unexpectedly old version, you are almost certainly
running a stale binary that shadows the freshly installed one. A common case on
macOS: a previously downloaded release binary lives in `/usr/local/bin/ws-dev`,
which precedes `$(go env GOPATH)/bin` on `PATH`. Remove the old copy
(`rm $(command -v ws-dev)`) and re-run, or reorder your `PATH`.

Note on version reporting: prebuilt release binaries carry the exact `vX.Y.Z`
tag (stamped via ldflags). A `go install ...@vX.Y.Z` build reports that module
version via Go's embedded build info; a local `go build` reports `dev` with the
VCS commit/time.

### Updating

```bash
ws-dev update          # download the latest release and replace this binary in place
ws-dev update --force  # reinstall even if already up to date
```

`update` downloads the release asset for your OS/arch, verifies it against the
release `checksums.txt`, and atomically swaps the running binary. If it lives in
a protected directory (e.g. `/usr/local/bin`), re-run with `sudo`. (For
`go install` setups, `go install ...@latest` remains an alternative.)

## Development setup

After cloning this repo, install the toolchain via `mise` and enable the lefthook git hooks (runs `secretlint` on staged files before each commit):

```bash
mise install
mise exec -- lefthook install
```

`mise install` provisions Go, golangci-lint, goreleaser, lefthook, and Node.js (used by `npx secretlint`).

## Quick start

```bash
cd /path/to/your-repo
ws-dev init                       # create ~/.config/ws-dev/config.yml + print this repo's key
# Edit the config: add an entry under `repos:` with `processes` / `tasks`.

claude -w feature-x               # create a worktree at .claude/worktrees/feature-x
ws-dev server feature-x           # start all processes in that worktree, stream logs
ws-dev logs                       # list logs of the running/most-recent server
ws-dev logs feature-x web -f      # follow a specific log
ws-dev run feature-x console      # run a task defined in config
ws-dev mcp                        # stdio MCP server (run inside the worktree)
```

The worktree name may be omitted when the command is run from inside the worktree
(it is inferred from the path). `ws-dev logs` additionally falls back to the most
recent `ws-dev server` run.

## Commands

| Command | Description |
|---------|-------------|
| `ws-dev init` | Create `~/.config/ws-dev/config.yml` (if missing) and print the config key for the current repo. |
| `ws-dev server [<worktree>]` | Start `processes` in parallel inside the worktree. Any prior `ws-dev server` for the repo is stopped first. |
| `ws-dev server stop` | Stop the current server. |
| `ws-dev logs [<worktree>] [<name>]` | List `*.log` or tail a specific log. |
| `ws-dev run [<worktree>] <task> [args...]` | Run a task defined under `tasks:`; extra args pass through. |
| `ws-dev tasks` | List tasks defined for the current repo. |
| `ws-dev mcp` | Run the stdio MCP server (log operations for `$PWD/$log_dir`). |
| `ws-dev update` | Replace the running binary with the latest GitHub release (verifies the checksum). `--force` to reinstall. |

Flags:
- `--log-dir <path>` — override log directory (falls back to `$WS_DEV_LOG_DIR`, then `log_dir` in config, then `log`).
- `--port-base <n>` — base port exposed to processes as `{{.PortBase}}` / `$WS_DEV_PORT_BASE`.
- `-n <lines>` / `-f` for `ws-dev logs`.

### One server per repository

`ws-dev server` records its PID under the repo's shared git directory
(`<git-common-dir>/ws-dev/`, i.e. inside `.git`, never committed and shared by
all worktrees). Starting a new server stops any prior `ws-dev server` for the
same repo first (SIGTERM, then SIGKILL on timeout). Running multiple worktrees
in parallel is not supported, to avoid port conflicts.

## `~/.config/ws-dev/config.yml`

```yaml
repos:
  # Keyed by git remote. ssh and https forms of the same repo match
  # interchangeably (git@github.com:owner/repo.git == https://github.com/owner/repo).
  github.com/owner/repo:
    log_dir: log

    # Optional wrapper applied to every process/task.
    # exec_wrapper: ["direnv", "exec", ".", "mise", "exec", "--"]

    # Commands run before any process starts (see "Setup commands" below).
    setup:
      - "direnv allow"
      - "mise trust"
      - "pnpm install"

    processes:
      web:
        cmd: "bundle exec rails s -b 0.0.0.0 -p {{.PortBase}}"
      worker:
        cmd: "bin/rails resque:work"
        env:
          QUEUE: "*"

    tasks:
      console: "bundle exec rails console"
      bundle-install: "bundle install"
```

Config file path resolves in this order: `$WS_DEV_CONFIG` > `$XDG_CONFIG_HOME/ws-dev/config.yml` > `~/.config/ws-dev/config.yml`.

Template variables available inside `processes.<name>.cmd`:
- `{{.Worktree}}` — worktree name
- `{{.PortBase}}` — base port
- `{{.Root}}` — main worktree root (absolute)
- `{{.Dir}}` — the worktree directory where the process runs (absolute)

Each process receives the following environment variables:
- `WS_DEV_WORKTREE`, `WS_DEV_ROOT`, `WS_DEV_DIR`, `WS_DEV_PORT_BASE`, `WS_DEV_LOG_DIR`
- Plus anything listed under `processes.<name>.env`.

### Setup commands

`setup` is a list of commands that `ws-dev server` runs **before** starting any
process, in the worktree directory, via the shell (`sh -c`). Use it to prepare
the environment so the server reliably comes up — for example authorizing
direnv/mise and installing dependencies:

```yaml
    setup:
      - "direnv allow"
      - "mise trust"
      - "pnpm install"
```

- Commands run sequentially in the listed order; the **first non-zero exit
  aborts** the start, so the server never comes up on a broken environment.
- They run **as written** — `setup` is *not* wrapped by `exec_wrapper`, because
  bootstrap steps like `direnv allow` must run directly (wrapping them would be
  circular). A step that needs the project toolchain can include the wrapper
  itself, e.g. `"mise exec -- pnpm install"` or `"direnv exec . pnpm install"`.
- The same template vars as processes are available (`{{.Worktree}}`,
  `{{.PortBase}}`, `{{.Root}}`, `{{.Dir}}`), and the same `WS_DEV_*` environment
  variables are exported.
- With `ws-dev server -b`, setup runs in the detached child, so its output is
  captured in `server.log` and the foreground call still returns immediately.

## MCP

`ws-dev mcp` is a stdio JSON-RPC server exposing four tools that operate on the current directory's `log/`:

- `list_logs` — enumerate `*.log` with size and mtime.
- `tail_log` — last N lines of `<name>.log`.
- `truncate_log` — truncate `<name>.log` to 0 bytes.
- `search_log` — regex search (RE2) with optional context lines.

It is intentionally config-free: it resolves the log directory relative to its
working directory (the worktree), so it works in any worktree even before the
repo is registered in `config.yml`. Register it in each worktree's
`.claude/settings.local.json` via Claude Code, pointing at the `ws-dev` binary
with `cwd` set to the worktree.

### Claude Code skill

This repo ships an agent-skill at `skills/ws-dev-logs/` that teaches Claude when to reach for each MCP tool and how to chain them (crash investigation, error search, reset-and-retry). Install it with [`gh skill`](https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli/) (GitHub CLI v2.90.0+):

```bash
# Per-worktree (recommended):
cd .claude/worktrees/<name>
gh skill install hiyamamo/ws-dev ws-dev-logs --agent claude-code --scope project

# Or once, globally for all Claude Code sessions:
gh skill install hiyamamo/ws-dev ws-dev-logs --agent claude-code --scope user
```

Provenance (repo, ref, tree SHA) is written into the installed `SKILL.md`'s frontmatter, so `gh skill update` can pick up changes later.
