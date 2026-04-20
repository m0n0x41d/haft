# Claude Code provider

The `claudecode` provider lets haft's interactive agent use the `claude` CLI
(Claude Code) as its LLM backend. This means **Pro / Max subscribers can drive
`haft agent` without setting `ANTHROPIC_API_KEY`** — auth is owned by the CLI.

## When to use it

- You already have a Claude Pro or Max subscription and don't want to pay
  per-token on top of it.
- You want a quick reasoning loop (`haft` bare mode, `/h-reason` brainstorming)
  without the Anthropic SDK.

## When **not** to use it

- You need haft's per-tool hooks, permission model, or cycle-tracking to run
  for every tool call. With this provider, tool execution happens inside the
  `claude` subprocess — haft's outer loop only sees the final assistant text
  after all rounds are done. Use `anthropic` or `openai` providers when you
  need tool-level governance.
- You need image input or fine-grained token accounting.

## Setup

```sh
# 1. Install Claude Code and sign in.
# https://docs.claude.com/en/docs/claude-code
claude login

# 2. Point haft at the claudecode provider.
cat > ~/.haft/config.yaml <<'YAML'
model: claude-code
YAML

# 3. Verify.
haft doctor
#   ✓ Claude Code CLI: 1.x.y (/usr/local/bin/claude)
```

No API key block is required in `config.yaml` — auth is delegated to the CLI.

### Max / Pro billing

As of the [Apr 2026 fix](https://github.com/anthropics/claude-code/issues/43333),
`claude -p` draws from an active Max/Pro subscription when the CLI is signed
in via OAuth and no `ANTHROPIC_API_KEY` is present. This provider **unsets
`ANTHROPIC_API_KEY` in the child process environment** before exec'ing
`claude`, so a stray export in your shell won't silently route you to
per-token API billing. The parent process env is untouched.

If you explicitly want API-key billing instead (e.g. for higher rate limits
or an Anthropic org account), use the `anthropic` provider instead —
`model: claude-sonnet-4-20250514` etc. — which will read your key directly.

## Model IDs

| haft model id        | CLI `--model` forwarded | Notes                         |
| -------------------- | ----------------------- | ----------------------------- |
| `claude-code`        | *(none — CLI default)*  | Whatever Claude Code prefers. |
| `claude-code:opus`   | `opus`                  | Alias → latest Opus.          |
| `claude-code:sonnet` | `sonnet`                | Alias → latest Sonnet.        |
| `claude-code:haiku`  | `haiku`                 | Alias → latest Haiku.         |

A full model name also works: `claude-code:claude-opus-4-5` forwards
`--model claude-opus-4-5` to the CLI.

## How it works

Each turn, haft flattens the conversation into `(system_prompt, user_prompt)`
and invokes:

```sh
claude -p \
  --output-format stream-json --verbose \
  --no-session-persistence \
  --mcp-config <tmpfile>   \      # points at `haft serve`
  --permission-mode bypassPermissions \
  --add-dir <project_root> \
  --system-prompt "<system>" \
  [--model <submodel>]
```

with the user prompt on stdin.

- `<tmpfile>` is generated per turn and contains an `mcpServers.haft` entry
  telling the CLI to spawn the current `haft` binary in `serve` mode with
  `QUINT_PROJECT_ROOT` pointing at the detected project root (discovered by
  walking up from `cwd` looking for `.haft/`). The tmpfile is deleted after
  the turn via `defer`.
- The model sees haft's artifact tools as `mcp__haft__haft_note`,
  `mcp__haft__haft_problem`, etc., **plus** the CLI's built-in Read/Write/
  Bash/etc. Tool execution happens inside the CLI subprocess; haft's outer
  agent loop receives the final assistant text after all round-trips finish.
- Opt out with `HAFT_CLAUDECODE_NO_MCP=1` — the provider falls back to
  `--allowed-tools ''` (text-only, no built-ins, no haft tools).

## Limitations

1. **Tools bypass haft's outer loop.** Permission callbacks, hooks, and
   cycle tracking do not fire per tool call when this provider is used.
2. **Each turn is a fresh CLI invocation.** No session reuse. On long
   conversations this can add ~200–500ms of CLI startup overhead per turn,
   plus another ~200ms to spawn `haft serve` inside the CLI.
3. **Doctor check is best-effort.** `haft doctor` only verifies `claude` is on
   PATH, not that you're actually signed in. Run `claude login` once if the
   first turn errors out with an auth failure.
4. **No image input** (for now — the CLI supports it, haft's converter
   doesn't surface `ImagePart` to the CLI yet).

## Follow-up work

### Session reuse via `--resume` (highest-impact)

Each turn currently pays ~5–10s for fresh `claude` + `haft serve` spawn.
`--resume <session_id>` would cut that to ~1–2s on turn 2+. It's the dominant
cost and is listed separately because it's not a small change:

- **Provider becomes stateful.** Today `LLMProvider.Stream` is a pure function
  of `(ctx, messages, tools, handler)`. Adding a session_id field changes that
  contract; `anthropic.go` / `openai.go` don't have one. Either we add a
  lifecycle method to the interface (`Close()` at minimum, arguably `Reset()`)
  or we keep the session TTL implicit and accept stale-session errors on
  occasional rebuild.
- **Message delta vs full history.** With `--resume`, the CLI owns the
  conversation. Re-sending the full flattened history every turn would
  double-count context. The provider needs to track "what did I send on the
  last turn" and forward only the new user message. A simple approach: on
  resume, send only the last `RoleUser` message's text; drop the
  `flattenConversation` path. But that makes divergence (haft's agent loop
  edited a prior message, for instance) silently wrong.
- **Drop `--no-session-persistence`.** Needed so the session can be found on
  turn 2. Consequence: `~/.claude/` accumulates haft sessions. A cleanup
  strategy (TTL, or explicit `claude session rm`) is wanted.
- **Fallback on stale session.** `--resume` errors on expired/deleted sessions.
  The provider should catch this and retry as a fresh turn — silent to the
  agent loop.

A clean shape is likely a follow-up PR that introduces an optional
`StatefulProvider` sub-interface and has only `ClaudeCodeProvider` (and maybe
future providers that can benefit) implement it.

### Other

- Propagate the CLI's `result` event token counts into `Message.Tokens`.
- Surface tool-use progress as StreamDelta `Thinking` chunks so the outer
  UI can show "Claude is calling haft_note…" rather than a silent pause.
- Image support by encoding `ImagePart` into stream-json input blocks.
