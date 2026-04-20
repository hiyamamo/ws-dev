---
name: ws-dev-logs
description: Inspect and manage process logs produced by `ws-dev server` through the ws-dev MCP server. Use when debugging crashes, tailing process output, searching for errors/warnings across log files, or clearing a noisy log before reproducing an issue. Triggers include "why did my web process crash", "tail the worker log", "search the log for <pattern>", and "clear the log and retry".
---

# ws-dev-logs

This skill guides Claude in using the four MCP tools that `ws-dev mcp` exposes for the current repo clone. Use it whenever the user is working inside a `ws-dev` workspace (`repos/<repo>-<label>/`) and wants to look at, search, or reset the logs produced by `ws-dev server`.

## Prerequisites

The MCP server must be registered in the clone's `.claude/settings.local.json`, pointing at the `ws-dev` binary with `cwd` set to the clone directory. If `list_logs`, `tail_log`, `truncate_log`, and `search_log` are not visible, ask the user to configure the MCP server per the `## MCP` section of the repo's README and retry.

The server runs in the clone's current working directory. Its log directory resolves in this order: `--log-dir` flag > `$WS_DEV_LOG_DIR` > `log`. Tool `name` arguments are just the basename inside that directory (the `.log` suffix is appended automatically — `web` and `web.log` both work).

## Tool selection

Pick tools in this order when you don't already know what you're looking for:

- **`list_logs`** — Start here. No arguments. Returns every `*.log` in the log directory sorted newest-mtime first, with size. Use to confirm which processes have written output and how fresh it is. Skip if the user explicitly names a log.

- **`tail_log`** — Last N lines of `<name>.log`. Default `lines: 100`. Bump to 500–1000 when chasing a stack trace or when the user says "give me more". This is a snapshot of the current tail, not a live follow — for live following, point the user at the terminal command `ws-dev logs <label> <name> -f`.

- **`search_log`** — RE2 regex search over the full file. Use this instead of `tail_log` when the event of interest might be older than the tail window, or when the user already knows a string/pattern. Supports `context` (lines before and after each match, default 0), `ignore_case` (default false), and `max_matches` (default 50). If you hit the `max_matches` cap, either narrow the regex or raise the limit — don't silently return a truncated view.

- **`truncate_log`** — **Destructive. In-place. No backup. Not recoverable.** Only call when the user has explicitly asked to clear a log (e.g. "wipe the web log and retry"). Never call pre-emptively or as cleanup. After truncating, tell the user to restart the process with `ws-dev server <label>` — the MCP server does not restart processes.

## Common workflows

### 1. Investigate a process crash

```
list_logs
→ pick the suspicious <name> by mtime or size
tail_log  name=<name>  lines=500
→ if the crash cause isn't in the tail:
search_log  name=<name>  pattern="(?i)error|panic|fatal|traceback"  context=5
```

Report back the match(es) with line numbers and a one-line summary of the likely cause before suggesting a fix.

### 2. Find a specific error message

```
search_log  name=<name>  pattern="<regex>"  ignore_case=true  context=3
```

Use `ignore_case=true` unless the user explicitly wants exact case. If the first run returns 50 matches (the default cap), either tighten the regex with more specific anchors/keywords, or raise `max_matches` explicitly — don't assume the first 50 are the interesting ones.

### 3. Reset a log before reproducing

```
# Confirm with the user first — this is not reversible.
truncate_log  name=<name>
# Then instruct the user:
#   "Restart the process with `ws-dev server <label>` and reproduce the issue."
```

Do not chain `truncate_log` with process restarts automatically; the MCP server has no control over processes.

## Argument reference

All tools accept `name` as a string; the `.log` suffix is added if missing.

| Tool | Required | Optional (default) |
|------|----------|--------------------|
| `list_logs` | — | — |
| `tail_log` | `name` | `lines` (100) |
| `truncate_log` | `name` | — |
| `search_log` | `name`, `pattern` | `max_matches` (50), `context` (0), `ignore_case` (false) |

## Caveats

- **Scope is one clone.** The MCP tools only see the log directory of the clone they run in. For cross-clone comparisons, the user must run Claude inside each clone separately.
- **No live follow.** `tail_log` returns a snapshot. For continuous tailing, the terminal command `ws-dev logs <label> <name> -f` is the right answer.
- **RE2, not PCRE.** Patterns passed to `search_log` are Go RE2: no backreferences, no lookaround. `(?i)` inline flag works and is equivalent to `ignore_case=true`.
- **`truncate_log` frees disk in place.** The inode is preserved, so processes that hold the file open keep writing to it after truncation — this is usually what you want.
