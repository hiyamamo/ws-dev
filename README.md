# ws-dev

Parallel workspace manager for multi-clone development. Clone the same repository multiple times under one directory, share environment files via symlinks, and manage processes/tasks from a single `ws-dev.yml`.

## Install

```bash
go install github.com/hiyamamo/ws-dev/cmd/ws-dev@latest
```

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
| `ws-dev link [<label>]` | Symlink each path in `links/` to the matching path in the repo. |
| `ws-dev unlink [<label>]` | Remove those symlinks (non-symlink files are left alone). |
| `ws-dev server [<label>]` | Start `processes` in parallel. Any prior `ws-dev server` is stopped first. |
| `ws-dev server stop` | Stop the current server. |
| `ws-dev logs [<label>] [<name>]` | List `*.log` or tail a specific log. |
| `ws-dev run [<label>] <task> [args...]` | Run a task defined under `tasks:`; extra args pass through. |
| `ws-dev mcp` | Run the stdio MCP server (log operations for `$PWD/$log_dir`). |

`<label>` may be omitted when the command is run from inside `repos/<repo-name>-<label>/` (or any subdirectory) — it is inferred from the path. `ws-dev logs` also falls back to the most recent `ws-dev server` run when neither is available.

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

### Claude Code skill

This repo ships an agent-skill at `skills/ws-dev-logs/` that teaches Claude when to reach for each MCP tool and how to chain them (crash investigation, error search, reset-and-retry). Install it with [`gh skill`](https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli/) (GitHub CLI v2.90.0+):

```bash
# Per-clone (recommended):
cd repos/<repo>-<label>
gh skill install hiyamamo/ws-dev ws-dev-logs --agent claude-code --scope project

# Or once, globally for all Claude Code sessions:
gh skill install hiyamamo/ws-dev ws-dev-logs --agent claude-code --scope user
```

Provenance (repo, ref, tree SHA) is written into the installed `SKILL.md`'s frontmatter, so `gh skill update` can pick up changes later.

