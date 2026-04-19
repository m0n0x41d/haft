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

- You need haft's artifact tools (`haft_note`, `haft_problem`, `haft_decision`,
  etc.) to be callable by the model. This MVP does not translate haft's tool
  schemas to the CLI surface — the model only emits text. Tool-driven agent
  loops should stay on the `anthropic` or `openai` providers until follow-up
  PRs land the `--mcp-config` bridge.
- You need image input or fine-grained token accounting. The CLI does not
  surface Anthropic-native token counts to stdout yet.

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
  --allowed-tools '' \          # disable CLI's built-in tools
  --no-session-persistence \
  --append-system-prompt "<system>" \
  [--model <submodel>]
```

with the user prompt on stdin. The provider parses NDJSON events from stdout,
forwards every `text` block as a `StreamDelta`, and returns the concatenated
response as an assistant `Message` once the `result` event arrives.

Tool-use events are ignored — haft's tool schemas aren't passed to the CLI in
this MVP, so the model will never emit `tool_use`.

## Limitations

1. **No tool-use.** Agent loops that require `haft_*` tools will get text-only
   responses. Use `anthropic` or `openai` for those flows.
2. **Each turn is a fresh CLI invocation.** No session reuse. On long
   conversations this can add ~200–500ms of CLI startup overhead per turn.
3. **Doctor check is best-effort.** `haft doctor` only verifies `claude` is on
   PATH, not that you're actually signed in. Run `claude login` once if the
   first turn errors out with an auth failure.

## Follow-up work

- Translate `agent.ToolSchema` → `--mcp-config` entries so haft's artifact
  tools are callable.
- Parse `tool_use` / `tool_result` stream-json events.
- Session reuse via `--resume` to amortize startup cost.
- Propagate token counts from the CLI's `result` event.
