---
description: "Onboard a project into Haft v7 specs and readiness"
---

# Project Onboarding

Onboard the repository into Haft v7. The goal is not to summarize the repo and
not to create generic notes. The goal is to make the project harnessable by
producing parseable authority carriers grounded in formal systems engineering:

```text
TargetSystemSpec
  -> EnablingSystemSpec
  -> TermMap
  -> SpecCoverage gaps
  -> DecisionRecords
  -> WorkCommissions
  -> RuntimeRuns
  -> Evidence
```

The Haft Core ships a typed onboarding method (`internal/project/specflow`).
You do not invent the order of phases or the required fields — Core dictates
both. You apply FPF and slideument patterns inside your reasoning to draft
quality content. **FPF citations stay inside your head; never write
`FRAME-XX`, `CHR-XX`, etc. into a `.haft/specs/*` YAML carrier.**

## Driver: ask Core for the next step, do not guess

```text
haft_spec_section(action="next_step", project_root="<repo>")
```

Or, in shell-only environments:

```bash
haft spec onboard --json
```

The response is a typed WorkflowIntent:

| Field | What you do with it |
|------|---------------------|
| `phase` | Stable id such as `target.environment.draft`. Do not skip ahead. |
| `prompt_for_user` | Show this to the human verbatim. It is the question they answer. |
| `context_for_agent` | Reasoning context for you. Names which FPF patterns to apply. |
| `expected_fields` | YAML keys the new SpecSection MUST carry. |
| `checks` | Structural validators that will run on the result (e.g. `require_statement_type`). |
| `blocking_findings` | If non-empty, the section already exists but fails validation. Resolve these before drafting new content. |
| `terminal` | When true, every registered phase is satisfied. Stop. |

## Mandatory FPF retrieval — every phase, no shortcuts

Before drafting a SpecSection for any phase you MUST retrieve the relevant
FPF patterns and apply them in your reasoning. The phase's
`context_for_agent` names which patterns are load-bearing for that phase.

```text
haft_query(action="fpf", query="FRAME-09")            # full pattern text
haft_query(action="fpf", query="CHR-10", explain=true) # match rationale
haft_query(action="fpf", query="boundary norm square") # semantic search
```

Default mandatory retrievals per phase:

| Phase | Retrieve and apply |
|------|-------------------|
| `target.environment.draft` | FRAME-01 (signal typing), FRAME-09 (role/capability/method/work distinction), CHR-12 (umbrella-word resolution), X-STATEMENT-TYPE |
| `target.role.draft` | FRAME-09 (strict distinction quad), CHR-11 (relational precision pipeline), X-SCOPE |
| `target.boundary.draft` | CHR-10 (Boundary Norm Square, L/A/D/E), FRAME-02 (scope/out-of-scope), X-FANOUT-AUDIT |
| any term work | CHR-12 (umbrella specializations), CHR-11 (relational precision) |
| any invariant or illegal-state | DEC-04 (invariants), CHR-10 (admissibility/deontics), X-STATEMENT-TYPE |
| any acceptance/evidence | VER-01 (evidence graph), VER-07 (refresh triggers) |

If `context_for_agent` names a pattern you do not recognize, retrieve it.
Skipping retrieval and writing from memory is a violation of the onboarding
contract: spec quality drifts toward generic prose without the patterns.

Use h-reason discipline before each section: frame the target claim,
identify the weakest link, then write the YAML. Quick framing inside your
own thought is fine; full `/h-frame` is only required for cross-cutting
architecture decisions, not per-section drafting.

## Phase 0: Readiness gate

1. Check `.haft/` exists. If not, tell the user to run `haft init` and stop.
2. Run `haft spec check`. If the carriers are malformed, fix structural
   findings before drafting new content.
3. Call `haft_spec_section(action="next_step")`. The first phase will
   surface here automatically.

Do not start broad harness/runtime execution while readiness is
`needs_onboard`. A tactical exception must be explicit and recorded with an
operator reason via `haft harness run --tactical-override-reason "..."`.

The supported v7 host-agent surfaces are Claude Code and Codex. Cursor,
Gemini CLI, JetBrains Air, and other MCP clients remain experimental
carriers; do not assume their tooling matches this contract.

## Phase loop

Repeat until the WorkflowIntent returns `terminal: true`:

1. Call `haft_spec_section(action="next_step")`.
2. Read `prompt_for_user` to the human; let them decide the load-bearing
   content (target role, environment change, etc.).
3. Read `context_for_agent`. Retrieve every named FPF pattern via
   `haft_query(action="fpf", ...)`. Apply them in your reasoning.
4. Read repo carriers (README, manifests, tests, source structure) to ground
   the section in reality. Do not derive target purpose from folder names or
   frameworks alone — those are enabling-system facts unless they describe
   externally required behavior.
5. Draft the YAML `spec-section` block in the carrier file named by
   `document_kind`. Populate every field listed in `expected_fields`.
   Required SoTA fields on every claim:
   - `statement_type` — one of: rule, promise, gate, explanation, evidence,
     definition. (Mixed types are an L1 error per X-STATEMENT-TYPE; decompose.)
   - `claim_layer` — one of: object, method, work.
   - `valid_until` — RFC3339 or `YYYY-MM-DD`. Refresh discipline is at the
     claim level, not only at evidence.
   - `target_refs` — for boundary sections, enumerate at least four
     stakeholder perspectives (CHR-10 corners).
   - `evidence_required[].kind` — for invariants and illegal states, declare
     guard location: `type | L1 | L2 | L3 | L4 | DB | E2E | manual`.
6. Run `haft spec check`. If `checks` from the WorkflowIntent emit findings,
   resolve them in the carrier before approving.
7. Mark the section active in the YAML (`status: active`) only after the
   human approves the load-bearing claims. Status flips are the explicit
   approval moment.
8. Record the SpecSectionBaseline immediately after the status flip:
   `haft_spec_section(action="approve", section_id=<id>, approved_by="human")`.
   Without a baseline the section reports `spec_section_needs_baseline`
   in `haft spec check` and stays blocking in `next_step`. The baseline
   is what makes drift detection meaningful for hand-edited carriers.
9. If `haft spec check` later reports `spec_section_drifted` for this
   section, triage with the operator:
   - intentional evolution → `haft_spec_section(action="rebaseline",
     section_id=<id>, reason="...")` (reason is required);
   - the section needs review → `haft_spec_section(action="reopen",
     section_id=<id>, reason="...")` to drop the baseline and re-enter
     the onboarding loop;
   - mistaken edit → revert the YAML change in the carrier; the next
     `haft spec check` should be clean against the existing baseline.
10. Repeat from step 1.

## Phase content reference

The exact phase set is owned by Core; this list is informational so you know
what to expect. Always trust `next_step` over this README.

### TargetSystemSpec phases

- `target.environment.draft` — what must change in the project's environment
  for the system to be useful. Apply FRAME-09 and CHR-12 to resolve umbrella
  words like "quality", "better", "scalable".
- `target.role.draft` — what role the target system plays in producing the
  environment change. FRAME-09 strict distinction: assigned vs can-do vs
  should-do vs did.
- `target.boundary.draft` — in-scope and out-of-scope, plus four CHR-10
  perspectives (Law, Admissibility, Deontics, Evidence) named in
  `target_refs`.

(Future phases not yet shipped: target interfaces, target invariants,
target acceptance/evidence, enabling-system architecture, enabling tests,
agent policy, runtime policy, evidence policy. Core will surface them via
`next_step` as they ship; do not draft them speculatively.)

## TermMap discipline

Each term must distinguish object, description, and carrier when relevant.
Apply CHR-12 (umbrella specializations) before adding a term: if the term
is umbrella-shaped (quality / action / service / sameness / wholeness /
relation-slot / basedness), specialize it before writing the entry.

Required early terms usually include:

- HarnessableProject
- TargetSystemSpec
- EnablingSystemSpec
- SpecSection
- SpecCoverage
- DecisionRecord
- WorkCommission
- RuntimeRun
- Evidence
- ExternalProjection

## What NOT to do

- Do NOT cite FPF patterns inside `.haft/specs/*` YAML. Patterns shape your
  reasoning; the carrier records the resolved claim.
- Do NOT skip `haft_query(action="fpf", ...)` retrieval before drafting. The
  whole point of the v7 method is that specs are SoTA system engineering,
  not BDD prose.
- Do NOT advance phases by guessing. The Core registry is authoritative.
- Do NOT mark a section `status: active` without the human's approval.
- Do NOT widen scope ("while we're here, let me also draft enabling specs")
  if `next_step` did not request them.
- Do NOT write target purpose from folder names or framework choices.

## Final report

When `next_step` returns `terminal: true` for the target-system spine,
produce the report:

```text
readiness: ready | needs_init | needs_onboard | missing
spec check: clean | blocked
target sections: N active / M total
enabling sections: N active / M total (or "phases pending")
term-map entries: N
remaining gaps:
- ...
next safe action:
- ...
```

Only after active target sections plus term-map entries pass `haft spec
check` should the project move toward `haft spec plan`, DecisionRecords,
and WorkCommissions.

$ARGUMENTS
