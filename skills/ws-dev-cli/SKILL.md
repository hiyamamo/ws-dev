---
name: ws-dev-cli
description: Drive the `ws-dev` CLI to run apps and tasks inside git worktrees — start/stop the process server, follow logs, run configured tasks, and set up `~/.config/ws-dev/config.yml`. Use when starting or stopping a worktree's processes, debugging why `ws-dev server` won't come up, running a one-off task, or configuring processes/tasks/setup for a repo. Triggers include "start the server in this worktree", "ws-dev server won't start", "stop the dev server", "run the console task", "how do I configure ws-dev", and "start the app in the background". For inspecting log *contents* via the MCP tools, use the ws-dev-logs skill instead.
---

# ws-dev-cli

`ws-dev` is a single Go binary that runs a Procfile-like set of processes,
user-defined tasks, and a built-in MCP server — all scoped to a git worktree
(e.g. those `claude -w` creates under `.claude/worktrees/<name>/`). Per-repo
settings live in `~/.config/ws-dev/config.yml`, keyed by the repo's git remote,
so nothing ws-dev-specific is committed into the repo.

This skill covers the terminal CLI. For reading/searching log *contents* through
the MCP server, use the **ws-dev-logs** skill.

## Mental model (read first)

- **One server per repository.** `ws-dev server` records its PID under the
  shared git dir (`<git-common-dir>/ws-dev/`, inside `.git`, never committed and
  shared by every worktree). Starting a new server **stops any prior one first**
  (SIGTERM, then SIGKILL on timeout). Running two worktrees' servers in parallel
  is not supported, to avoid port conflicts. `ws-dev status` reports whether a
  server is currently running and for which worktree.
- **Worktree argument is optional and positional.** A worktree is resolved by
  directory basename against `git worktree list`; an ambiguous basename is an
  error. When omitted, commands default to the repository's **main worktree
  (root)** — except `ws-dev logs`, which first falls back to the most recent
  `ws-dev server` run.
- **The repo must be registered** in `config.yml` (keyed by git remote) for
  `server` / `run` / `tasks` to do anything. `ws-dev mcp` is the exception — it
  is config-free. Run `ws-dev init` to create the file and print the key.

## Commands

| Command | What it does |
|---------|--------------|
| `ws-dev init` | Create `~/.config/ws-dev/config.yml` (if missing) and print this repo's config key. |
| `ws-dev server [<worktree>]` | Start all `processes` in parallel in the worktree (stops any prior server first). Foreground; streams output. |
| `ws-dev server -b [<worktree>]` | Same, but **detached** — returns immediately; output goes to `<log-dir>/server.log`. |
| `ws-dev server stop` | Stop the currently running server. |
| `ws-dev status` | Show whether the server is running (pid) and for which worktree. |
| `ws-dev logs [<worktree>] [<name>]` | List `*.log`, or tail `<name>.log`. |
| `ws-dev run [<worktree>] <task> [args...]` | Run a task defined under `tasks:`; extra args pass through. |
| `ws-dev tasks` | List tasks defined for this repo. |
| `ws-dev mcp` | stdio MCP server for log ops (see the **ws-dev-logs** skill). |
| `ws-dev update` | Replace the running binary with the latest GitHub release (`--force` to reinstall). Uses `$GH_TOKEN` / `$GITHUB_TOKEN` / `gh auth token` to authenticate the API call. |
| `ws-dev version` | Print the version. |

### `ws-dev server` — start the app

```bash
ws-dev server                 # processes in the repo root worktree (foreground)
ws-dev server feature-x       # processes in the feature-x worktree (foreground)
ws-dev server -b feature-x    # detached; returns immediately
ws-dev server stop            # stop whatever server is running for this repo
```

Flags:

- `--port-base <n>` — base port exposed to processes as `{{.PortBase}}` /
  `$WS_DEV_PORT_BASE`. Default: `3000` (or `$WS_DEV_PORT_BASE`).
- `--log-dir <path>` — log directory relative to the worktree (overrides config
  and `$WS_DEV_LOG_DIR`).
- `-b`, `--background` — detach and return immediately.

Order of events on each start: stop prior server → run `setup` commands → start
processes. A failing `setup` step aborts the start (see **Setup commands**).

**Crash behavior:** if any process exits abnormally (non-zero status), the
whole server shuts down and `ws-dev server` exits non-zero — no half-dead
servers. A process that exits cleanly (status 0, e.g. a one-shot build step)
leaves the others running.

**Foreground vs background:** the foreground run stops only when you Ctrl-C it or
another `ws-dev server` / `ws-dev server stop` arrives. `-b` re-execs itself in a
new session and exits; everything (including `setup`) then runs in the detached
child, with combined output in `server.log`. Per-process output still goes to
`<log-dir>/<name>.log`. Check a background server with `ws-dev status`; stop it
with `ws-dev server stop`.

### `ws-dev logs` — find and tail logs

```bash
ws-dev logs                       # list logs of the running/most-recent server
ws-dev logs feature-x             # list logs for a specific worktree
ws-dev logs feature-x web         # last 100 lines of web.log
ws-dev logs feature-x web -n 500  # last 500 lines
ws-dev logs feature-x web -f      # follow (tail -f)
```

Flags: `-n`/`--lines` (default 100), `-f`/`--follow`, `--log-dir`. With no
`<name>`, it lists `*.log` newest-mtime first with sizes. `-f` is the right
answer for *live* following; the MCP `tail_log` tool is only a snapshot. `-f`
keeps following across truncation (e.g. after the MCP `truncate_log` tool) by
restarting from the top when the file shrinks.

### `ws-dev run` / `ws-dev tasks` — one-off tasks

```bash
ws-dev tasks                              # list defined tasks
ws-dev run console                        # run `console` at the repo root
ws-dev run feature-x console             # run `console` in the feature-x worktree
ws-dev run feature-x migrate VERSION=20240101   # extra args pass through
```

Arguments are positional: `[<worktree>] <task> [args...]`. The first arg is
treated as a worktree **only** when it is not itself a defined task and the next
arg is. So `ws-dev run console` runs at the root; `ws-dev run <wt> console`
targets a worktree. Tasks are wrapped by `exec_wrapper` (if configured) and
inherit your terminal's stdio.

### `ws-dev init` — register a repo

```bash
cd /path/to/your-repo
ws-dev init     # creates the config if missing, prints the `repos:` key to add
```

Then edit `config.yml` and add an entry under `repos:` for the printed key.

## `~/.config/ws-dev/config.yml`

```yaml
repos:
  # Keyed by git remote. ssh and https forms match interchangeably
  # (git@github.com:owner/repo.git == https://github.com/owner/repo), so
  # add only ONE entry per repo — two keys for the same repo are rejected.
  github.com/owner/repo:
    log_dir: log                       # log dir per worktree (default: log)

    # Optional wrapper applied to every process AND task (not to setup).
    exec_wrapper: ["direnv", "exec", ".", "mise", "exec", "--"]

    # Commands run by `ws-dev server` BEFORE any process starts.
    setup:
      - "direnv allow"
      - "mise trust"
      - "mise exec -- pnpm install"

    processes:                         # started by `ws-dev server`
      web:
        cmd: "bundle exec rails s -b 0.0.0.0 -p {{.PortBase}}"
      worker:
        cmd: "bin/rails resque:work"
        env:
          QUEUE: "*"

    tasks:                             # run by `ws-dev run <task>`
      console: "bundle exec rails console"
      bundle-install: "bundle install"
```

**Template variables** available in `processes.<name>.cmd`, `tasks`, and `setup`:

- `{{.Worktree}}` — worktree name
- `{{.PortBase}}` — base port
- `{{.Root}}` — main worktree root (absolute)
- `{{.Dir}}` — the worktree directory where the command runs (absolute)

After expansion the string is split into argv by shell-like whitespace
(only `"..."` / `'...'` quoting, no escapes).

**Environment exported** to every process/task: `WS_DEV_WORKTREE`, `WS_DEV_ROOT`,
`WS_DEV_DIR`, `WS_DEV_PORT_BASE`, `WS_DEV_LOG_DIR`, plus anything under
`processes.<name>.env`.

### Setup commands

`setup` runs sequentially before any process starts, in the worktree dir, via
`sh -c`. The **first non-zero exit aborts** the start, so the server never comes
up on a broken environment. Unlike processes/tasks, `setup` is **not** wrapped by
`exec_wrapper` — bootstrap steps like `direnv allow` / `mise trust` must run
as-is. A step needing the toolchain includes the wrapper itself (e.g.
`"mise exec -- pnpm install"`). With `-b`, `setup` runs in the detached child and
its output lands in `server.log`.

## Resolution order (memorize)

- **Log directory:** `--log-dir` flag → `$WS_DEV_LOG_DIR` → `log_dir` in config →
  `log`. Base is the worktree directory.
- **Config file path:** `$WS_DEV_CONFIG` → `$XDG_CONFIG_HOME/ws-dev/config.yml` →
  `~/.config/ws-dev/config.yml`.
- **Port base:** `--port-base` → `$WS_DEV_PORT_BASE` → `3000`.

## Troubleshooting

- **"server won't start" / aborts immediately** — almost always a failing
  `setup` step (it aborts before processes start). In background mode the error
  is in `server.log`; run foreground (`ws-dev server <wt>`) to see it inline.
- **Server stopped by itself** — one process exited non-zero, which stops the
  whole server by design. `ws-dev logs <wt> <name>` (or `server.log` for `-b`
  runs) shows which one and why.
- **A previous app is still holding the port** — a new `ws-dev server` stops the
  prior one automatically, but a process started outside ws-dev won't be. Use
  `ws-dev server stop` and check for stragglers.
- **"ambiguous worktree" error** — two worktrees share a basename; pass the one
  intended, or rename. Resolution is by basename, not full path.
- **Nothing happens / "no tasks"** — the repo isn't registered. Run `ws-dev init`
  and add the printed key under `repos:`.
- **Wrong/old binary** — `which -a ws-dev` and `ws-dev version`; a stale copy
  earlier on `PATH` shadows a fresh install. `ws-dev update` self-updates.
- **`ws-dev update` hits GitHub rate limit** — unauthenticated requests are
  capped at 60/hour per IP. Set `$GH_TOKEN` or `$GITHUB_TOKEN`, or run
  `gh auth login` so `gh auth token` returns a token; `update` picks them up
  automatically (env var → `gh auth token`).

## Caveats

- **Scope is one repo, one server.** Don't try to run servers for two worktrees
  at once.
- **Foreground `server` blocks the terminal.** Use `-b` for fire-and-forget, or
  background the shell job yourself.
- **`setup` is shell, processes/tasks are argv.** `setup` entries go through
  `sh -c` (pipes/`&&` work); process/task `cmd` are whitespace-split into argv
  (no shell features beyond minimal quoting).
