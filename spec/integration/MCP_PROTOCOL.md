# MCP Integration

> How AI agents interact with Haft via MCP (Model Context Protocol).

## Stability Contract

The 6 MCP tools are the **stable API**. When dev merges to main:

- Tool names don't change
- Required parameters don't change
- Return shapes are additive (new fields ok, removing fields = breaking)
- Existing behavior preserved

## Tool Surface

| Tool | Actions | Mode mapping |
|------|---------|-------------|
| `haft_problem` | frame, characterize, select, close | Understand |
| `haft_solution` | explore, compare | Explore, Choose |
| `haft_decision` | decide, apply, measure, evidence, baseline | Execute, Verify |
| `haft_query` | status, search, list, coverage, related, fpf, view | Utility |
| `haft_refresh` | scan, drift, waive, reopen, supersede, deprecate | Verify |
| `haft_note` | (single action) | Note |

## Host Agents

| Agent | Config location | Init flag |
|-------|----------------|-----------|
| Claude Code | `.mcp.json` | `--claude` (default) |
| Cursor | `.cursor/mcp.json` | `--cursor` |
| Gemini CLI | `~/.gemini/settings.json` | `--gemini` |
| Codex CLI / App | `.codex/config.toml` | `--codex` |
| JetBrains Air | `.codex/config.toml` | `--air` |

All use same binary (`haft serve`), same protocol (JSON-RPC over stdin/stdout), same tools.

## Environment

| Variable | Purpose |
|----------|---------|
| `HAFT_PROJECT_ROOT` | Project directory (set by init, passed to serve) |

## What Agents See

When a host agent calls a Haft tool, it receives:
- Structured response with artifact data
- NavStrip showing current state and available next actions
- Cross-project recall results (when framing problems)
- Warnings for validation issues
- Refresh reminders (if >5 days since last scan)
