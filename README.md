# ws-dev

Parallel workspace manager for multi-clone development. Clone the same repository multiple times under one directory, share environment files via symlinks, and manage processes/tasks from a single `ws-dev.yml`.

## Install

```bash
go install github.com/hiyamamo/ws-dev/cmd/ws-dev@latest
```

Or build from source:

```bash
go build -o ws-dev ./cmd/ws-dev
```

## Quick start

```bash
ws-dev init myproj
cd myproj
# Edit ws-dev.yml: set `repo`, fill in `processes`, `tasks`, `links`.
# Place shared env files under links/ (e.g. links/.envrc).

ws-dev clone branch-a
ws-dev link branch-a
ws-dev server branch-a            # starts all processes, streams logs
ws-dev logs                       # list logs for the most recent label
ws-dev logs branch-a web -f       # follow a specific log
ws-dev run branch-a console       # run a task defined in ws-dev.yml
ws-dev mcp                        # stdio MCP server (run inside repos/<repo>-<label>)
```

## Commands

| Command | Description |
|---------|-------------|
| `ws-dev init <name>` | Scaffold `<name>/` with `ws-dev.yml`, `repos/`, `links/`, `.gitignore`. |
| `ws-dev clone <label>` | Clone the configured repo into `repos/<repo-name>-<label>/`. |
| `ws-dev link <label>` | Symlink each path in `links/` to the matching path in the repo. |
| `ws-dev unlink <label>` | Remove those symlinks (non-symlink files are left alone). |
| `ws-dev server <label>` | Start `processes` in parallel. Any prior `ws-dev server` is stopped first. |
| `ws-dev server stop` | Stop the current server. |
| `ws-dev logs [<label>] [<name>]` | List `*.log` or tail a specific log. Label defaults to the most recent `server` run. |
| `ws-dev run <label> <task> [args...]` | Run a task defined under `tasks:`; extra args pass through. |
| `ws-dev mcp` | Run the stdio MCP server (log operations for `$PWD/$log_dir`). |

Flags:
- `--log-dir <path>` — override log directory (falls back to `$WS_DEV_LOG_DIR`, then `log_dir` in config, then `log`).
- `--port-base <n>` — base port exposed to processes as `{{.PortBase}}` / `$WS_DEV_PORT_BASE`.
- `-n <lines>` / `-f` for `ws-dev logs`.

## `ws-dev.yml`

```yaml
repo: https://github.com/owner/repo
log_dir: log

# Optional wrapper applied to every process/task.
# exec_wrapper: ["direnv", "exec", ".", "mise", "exec", "--"]

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

links:
  - .envrc
  - .claude/settings.local.json
  - storage   # directories are linked recursively (existing dir is replaced)
```

Template variables available inside `processes.<name>.cmd`: `{{.Label}}`, `{{.PortBase}}`, `{{.Workspace}}`.

Each process receives the following environment variables:
- `WS_DEV_LABEL`, `WS_DEV_WORKSPACE`, `WS_DEV_PORT_BASE`, `WS_DEV_LOG_DIR`
- Plus anything listed under `processes.<name>.env`.

## MCP

`ws-dev mcp` is a stdio JSON-RPC server exposing four tools that operate on the current directory's `log/`:

- `list_logs` — enumerate `*.log` with size and mtime.
- `tail_log` — last N lines of `<name>.log`.
- `truncate_log` — truncate `<name>.log` to 0 bytes.
- `search_log` — regex search (RE2) with optional context lines.

Register it in each clone's `.claude/settings.local.json` via Claude Code, pointing at the `ws-dev` binary with `cwd` set to the repo.

## Examples

See `examples/rails/` for the Rails setup that this tool was extracted from.
