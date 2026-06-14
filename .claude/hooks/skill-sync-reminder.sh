#!/usr/bin/env sh
# PostToolUse hook: when ws-dev's user-facing surface is edited, remind Claude to
# keep the agent skills under skills/ in sync. Reads the hook payload (JSON) on
# stdin and inspects tool_input.file_path. Emits a PostToolUse additionalContext
# reminder only when an editor tool touched internal/cmd, internal/mcp, or
# internal/config. Never blocks; exits 0 in all cases.

input=$(cat)

# Pull out tool_input.file_path without assuming key order (compact JSON).
file=$(printf '%s' "$input" \
  | grep -oE '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' \
  | head -n1 \
  | sed -E 's/.*"file_path"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/')

case "$file" in
  */internal/cmd/*|internal/cmd/*)
    skill="skills/ws-dev-cli/SKILL.md (CLI commands/flags/config)" ;;
  */internal/config/*|internal/config/*)
    skill="skills/ws-dev-cli/SKILL.md (config schema)" ;;
  */internal/mcp/*|internal/mcp/*)
    skill="skills/ws-dev-logs/SKILL.md (MCP log tools)" ;;
  *)
    exit 0 ;;
esac

reminder="You edited ws-dev's user-facing surface ($file). If this changed a command, flag, MCP tool, config key, template var, env var, or default, update the matching agent skill in the same change: $skill. Also keep README.md and CLAUDE.md consistent. Revise the affected sections rather than just appending."

# Escape for safe embedding in a JSON string.
escaped=$(printf '%s' "$reminder" | sed 's/\\/\\\\/g; s/"/\\"/g')

printf '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"%s"}}\n' "$escaped"
exit 0
