# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository overview

This repository is the development repository for the **ws-dev CLI**. It is a single Go binary that provides a workflow for cloning the same repository multiple times and developing in parallel.

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
| `internal/cmd` | Cobra subcommand definitions (init/clone/link/unlink/server/logs/run/mcp) |
| `internal/config` | Parses `ws-dev.yml`; `RepoName()` extracts the basename from the URL |
| `internal/workspace` | Locates `ws-dev.yml` (walking cwd up to parents); `RepoDir(label)` etc. |
| `internal/links` | Creates relative symlinks from `links/` into `repos/<repo>-<label>/`. Existing directories are replaced. |
| `internal/tasks` | Runs commands prefixed with `exec_wrapper`, inheriting stdio |
| `internal/procman` | Procfile-equivalent parallel process manager. Expands `{{.Label}}` etc. via `text/template`, places each process in its own pgid via setpgid, and cleans up with SIGTERM/SIGKILL. |
| `internal/mcp` | stdio JSON-RPC MCP server implementation (`list_logs` / `tail_log` / `truncate_log` / `search_log`) |

## Key design points

### The server assumes a single label

`ws-dev server <label>` reads `.ws-dev/server.pid`, and if the previous ws-dev process is still alive it stops it with `SIGTERM` before starting the new process. Running labels in parallel is not supported (to avoid port conflicts).

- `.ws-dev/server.pid` — our own PID
- `.ws-dev/current-label` — most recent label (used by `ws-dev logs` when omitted)

### Log directory resolution order

Command-line flag `--log-dir` -> environment variable `WS_DEV_LOG_DIR` -> `log_dir` in `ws-dev.yml` -> default `log`.

This order is consistent across `server` / `logs` / `mcp`. `mcp` resolves relative to `cwd` (which is expected to be inside each clone).

### Directory naming

`repos/<repo-name>-<label>/`. `<repo-name>` is inferred from the basename of the `repo:` URL (with `.git` stripped). The `<label>` that the user passes is a short name (e.g. `fix-login`); the CLI adds the prefix internally.

### Process template expansion

`processes.<name>.cmd` is evaluated as a Go `text/template`:

- `{{.Label}}` — the label
- `{{.PortBase}}` — `--port-base` / `$WS_DEV_PORT_BASE` / default 3000
- `{{.Workspace}}` — absolute workspace path

After expansion, the string is split into argv via shell-like whitespace splitting (`tasks.fields`). Quoting support is minimal (`"..."` / `'...'` only; no escapes).

## Manual verification

```bash
# Exercise everything in a sample workspace
mkdir -p /tmp/ws-play && cd /tmp/ws-play
/path/to/ws-dev init sample && cd sample
# Edit ws-dev.yml (point repo at a real repo, fill in processes)
../ws-dev clone branch-a
../ws-dev link branch-a
../ws-dev server branch-a       # in another terminal
../ws-dev logs                  # most recent label
../ws-dev logs branch-a web -f  # follow
../ws-dev run branch-a console  # tasks.console
```

MCP standalone test:
```bash
cd repos/<repo>-branch-a
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | ws-dev mcp
```
