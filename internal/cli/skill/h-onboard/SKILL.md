---
name: h-onboard
description: |
  Compatibility/bootstrap entrypoint for first-time haft spec setup. Use when the repository has no `.haft/` directory, `.haft/` exists but has no active SpecSections, or the operator says "set up haft here", "onboard this project", "initialize FPF", "first time using haft in this repo", "let's add haft to this project", or "scaffold haft for this codebase". In projects with haft already installed, this skill immediately routes through h-spec and `mcp__haft__haft_spec_section(action="lifecycle")`; it does not maintain a separate onboarding template. For ongoing spec updates use h-spec directly.
when_to_use: |
  Repo has no `.haft/` directory, OR `.haft/` exists but spec lifecycle is not ready. Existing haft projects with spec-update, "запиши в спеки", approve, rebaseline, or stale-section requests should use h-spec directly.
argument-hint: "[optional: short project description]"
allowed-tools: Bash Read Grep Glob Write Edit mcp__haft__haft_problem mcp__haft__haft_query mcp__haft__haft_spec_section
---

# h-onboard — Bootstrap route into h-spec

This skill is the old operator entrypoint name. It is now a compatibility
wrapper around the typed spec lifecycle.

Do **not** use the old three-prose-file onboarding method. The current method
is driven by `SpecSection` phases, `WorkflowIntent`, and `SpecSectionBaseline`
human gates. The primary ongoing interface is `/h-spec`.

## Step 1 — Check whether haft is initialized

If `.haft/` is missing, tell the operator that project setup starts with:

```
haft init
```

`haft init` installs the project graph, MCP config, and skills. Do not hand-roll
`.haft/specs/` directories or local markdown carriers before initialization.

If `.haft/` exists, continue.

## Step 2 — Ask the lifecycle projection

Call:

```
mcp__haft__haft_spec_section(action="lifecycle")
```

If the installed haft binary is older and does not support `lifecycle`, call:

```
mcp__haft__haft_spec_section(action="next_step")
```

and use:

```
haft spec status
haft spec next --json
```

as the CLI fallback when shell access is available.

## Step 3 — Route by lifecycle state

- `ready` / `action=none`: onboarding is already complete. Stop and suggest
  `/h-status` for the dashboard or `/h-spec` for ongoing spec edits.
- `needs_action` with `action=draft` or `action=clarify`: follow the h-spec
  drafting procedure. Read `workflow_intent.context_for_agent`,
  `expected_fields`, `checks`, and `carrier`; draft or fix the fenced
  `yaml spec-section` block; run `haft spec check`; call lifecycle again.
- `needs_human_gate` with `action=approve`: present the active section and ask
  for explicit approval. Only then call `haft_spec_section(action="approve")`.
- `needs_triage` with `action=triage`: surface drift/staleness findings and ask
  the operator to choose rebaseline, reopen, rollback, deprecate, or supersede.

## Step 4 — Discover before drafting

For draft/clarify actions, read enough project evidence to avoid invention:

- README and top-level docs
- build/test config (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`,
  Makefile, CI files, etc.)
- source entry points (`cmd/`, `src/`, `internal/`, `lib/`, or equivalent)
- existing `.haft/specs/` carriers
- ADRs, docs, and previous decision artifacts when present

Ask at most 1-3 questions only when this evidence cannot determine a required
field. The agent drafts from evidence; the operator corrects and approves.

## Step 5 — Optional bootstrap ProblemCard

If this is the first spec bootstrap and there is no existing onboarding
ProblemCard, record one tactical ProblemCard:

```
mcp__haft__haft_problem(
  action="frame",
  problem_type="synthesis",
  title="Project onboarding: <repo name>",
  signal="Project lacks ready SpecSection lifecycle; agent is drafting initial typed spec carriers for operator review",
  acceptance="Spec lifecycle returns ready after operator-reviewed active SpecSections have baselines",
  blast_radius="<project scope>",
  reversibility="high",
  mode="tactical"
)
```

Do not duplicate this card on every run. Search first when unsure.

## What NOT to do

- DO NOT maintain separate h-onboard templates. Use lifecycle
  `workflow_intent` as the typed contract.
- DO NOT ask the operator to author blank specs from scratch.
- DO NOT auto-approve, auto-rebaseline, or auto-reopen.
- DO NOT bind DecisionRecords during onboarding. Use manual `/h-decide`.
- DO NOT recommend `haft agent` or `haft desktop`; v8 is skills + CLI + MCP.

## FPF references

- A.1 — Holonic Foundation: target system vs enabling system
- A.7 — Object / Description / Carrier
- F.17 — Unified Term Sheet
- E.14 — Human-Centric Working-Model

Look up via `mcp__haft__haft_query(action="fpf", query="A.1")`.
