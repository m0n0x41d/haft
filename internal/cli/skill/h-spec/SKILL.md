---
name: h-spec
description: |
  Primary spec lifecycle interface for a haft project: inspect typed SpecSection lifecycle, draft or clarify ProjectSpecificationSet carriers, handle operator requests like "update specs", "write this into the specs", "запиши в спеки", "обнови спеку", "spec status", "spec next", "stale spec section", "approve spec", "rebaseline spec", or "reopen spec". Use this skill when the operator wants specs as living project governance, not module/file decision mapping or semantic fanout repair. It always calls `mcp__haft__haft_spec_section(action="lifecycle")` first and treats markdown carriers as carriers, not authority. For missing `.haft/` bootstrap, use h-onboard; for semantic fanout repair, use h-semio-review.
when_to_use: |
  Operator asks about spec lifecycle, onboarding progress, spec readiness, spec status, stale/drifted SpecSections, or asks the agent to record a clarification into specs. For module/file decision mapping use h-spec-cover. For semantic fanout repair use h-semio-review.
argument-hint: "[optional: spec topic, clarification, or section id]"
allowed-tools: Bash Read Grep Glob Write Edit mcp__haft__haft_query mcp__haft__haft_spec_section mcp__haft__haft_problem
---

# h-spec — Spec lifecycle interface

You are the operator-facing interface for Haft specs. Specs are living
project-governance descriptions, but they are not self-acting agents.

TRIGGER on: "show spec lifecycle status", "spec lifecycle status", "spec status", "spec next", "update spec", "write this into specs", "запиши в спеки", "обнови спеку", "approve spec", "rebaseline spec", "reopen spec", "stale spec section".

Hold the FPF distinction explicitly:

- **Object:** the project capability, environment, role, boundary, or term being described
- **Description:** the SpecSection claim about that object
- **Carrier:** `.haft/specs/*.md` with fenced `yaml spec-section` blocks
- **Authority:** the haft kernel projection and human gates, not ad hoc markdown reading

## Step 1 — Ask the kernel for lifecycle

Call the typed lifecycle projection first:

```
mcp__haft__haft_spec_section(action="lifecycle")
```

If this action is unavailable because the installed haft binary is older, fall
back to:

```
mcp__haft__haft_spec_section(action="next_step")
```

and, when shell access is available:

```
haft spec status
haft spec next --json
```

Do not infer lifecycle state by grepping `.haft/specs/` before this call.

## Step 2 — Interpret the projection

Use the projection fields directly:

- `state` — readiness bucket (`ready`, `needs_action`, `needs_human_gate`, `needs_triage`)
- `action` — next lifecycle projection action (`draft`, `clarify`, `approve`, `triage`, `none`).
  `rebaseline` and `reopen` are mutation commands surfaced under
  `allowed_commands` when `action=triage`; they are not lifecycle projection
  states.
- `object` — what is being acted on (`SpecSection` or `SpecSectionBaseline`)
- `carrier` — the file to edit or review
- `section_id` / `section_kind` — the stable identity of the section
- `workflow_intent` — the typed drafting contract: phase, expected fields, checks, prompt for user, context for agent
- `human_gate` — the gate the agent must not silently cross

If `state=ready` or `action=none`, specs are not blocking the current work.
Answer the operator's question from the carriers and graph, but do not edit
unless they explicitly ask.

## Step 3 — Draft or clarify carriers

For `action=draft` or `action=clarify`:

1. Read `workflow_intent.context_for_agent`, `expected_fields`, `checks`, and
   `carrier`.
2. Read the repository context needed to draft honestly: README, build/test
   config, docs, existing `.haft/specs/`, and the relevant source entry points.
3. Ask at most 1-3 clarifying questions only when repository evidence cannot
   answer a field without invention.
4. Edit or create the carrier with a fenced `yaml spec-section` block. Keep
   new or uncertain sections as `status: draft`.
5. Run `haft spec check` when shell access is available.
6. Call `mcp__haft__haft_spec_section(action="lifecycle")` again and surface
   the new next action.

Use the kernel's expected fields and checks as the contract. Do not maintain a
separate local template in this skill.

## Step 4 — Handle "запиши" / "write this into specs"

When the operator clearly says to record a clarification, e.g. "запиши",
"запиши это в спеки", "update the spec", or "put this into the target spec":

1. Treat that as permission to edit the relevant spec carrier.
2. Preserve the object/description/carrier distinction in the edit.
3. If the target section is ambiguous, call lifecycle first and ask one narrow
   routing question instead of guessing between target, enabling, and term-map.
4. Run `haft spec check`.
5. Call lifecycle again.

Important: "запиши" authorizes a carrier edit. It does **not** authorize
`approve`, `rebaseline`, or `reopen` unless the operator explicitly says that
baseline state should change.

## Step 5 — Human gates

For `action=approve`:

- Present the carrier and section id.
- Ask for explicit operator approval if it was not already given.
- If the section is still `status: draft` and the operator approves it, change
  only that section to `status: active`, then call lifecycle again.
- When lifecycle still says `action=approve`, call:

```
mcp__haft__haft_spec_section(
  action="approve",
  section_id="<section-id>",
  approved_by="agent"
)
```

For `action=triage`:

- Surface the blocking findings.
- Offer exactly the admissible choices: rebaseline, reopen, rollback carrier
  edit, deprecate/supersede when appropriate.
- Only call `rebaseline` or `reopen` after explicit operator instruction with
  a reason:

```
mcp__haft__haft_spec_section(
  action="rebaseline",
  section_id="<section-id>",
  approved_by="agent",
  reason="<operator-approved reason>"
)
```

```
mcp__haft__haft_spec_section(
  action="reopen",
  section_id="<section-id>",
  reason="<operator-approved reason>"
)
```

## Step 6 — Present results

Keep the status short and grounded:

- Current lifecycle state and next action
- Object and carrier
- Section id / kind, paired with a human-readable label when available
- What was edited, if anything
- Checks run and their result
- Human gate still open, if any

## What NOT to do

- DO NOT treat `.haft/specs/*.md` as the authority. It is the carrier; the
  kernel projection is the lifecycle contract.
- DO NOT create feature-local spec markdown as a second authority. Work links
  to SpecSections, DecisionRecords, WorkCommissions, RuntimeRuns, and code
  through the graph.
- DO NOT auto-approve, auto-rebaseline, or auto-reopen from a vague "looks
  good". These are human gates.
- DO NOT use h-spec for module/file decision coverage. Use h-spec-cover.
- DO NOT use h-spec for rename/deprecation fanout or semantic consistency
  audits. Use h-semio-review.
- DO NOT bind a DecisionRecord from this skill. Use manual `/h-decide`.
- DO NOT ask the operator to author blank specs from scratch when repository
  evidence is available. The agent drafts; the operator corrects and approves.

## FPF references

- A.7 — Object / Description / Carrier
- A.1 — Target system vs enabling system
- F.17 — Unified Term Sheet
- B.3.4 — Evidence Decay and stale claims
- E.14 — Human-Centric Working-Model

Look up via `mcp__haft__haft_query(action="fpf", query="A.7")`.
