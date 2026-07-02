---
name: h-explore
description: |
  Generates 3–5 genuinely distinct candidate solution variants for a framed problem — each variant differs in KIND (not just degree), carries an explicit weakest-link so weak options surface before implementation, and optionally marks stepping-stones that open future search space. Make sure to use this skill whenever the user asks "what are our options", "how could we do X", "brainstorm approaches", "give me alternatives", "different ways to X", "what variants should we consider", "what else could we try", or whenever they are about to commit to one approach without having generated alternatives. Also use when a problem is framed but only one solution sits on the table. NOT for comparing existing options head-to-head (use h-compare). NOT for hypothesis testing on a failure (use h-diagnose).
when_to_use: |
  Problem is framed (or frame inline) and only one approach is on the table, OR the user explicitly asks for options. For evaluating existing variants use h-compare.
argument-hint: "[exploration topic or problem reference]"
allowed-tools: Bash Agent mcp__haft__haft_problem mcp__haft__haft_solution mcp__haft__haft_query
---

# h-explore — Generate distinct variants with NQD discipline

You are running the FPF exploration workflow. The point is rivalrous candidate generation: 3-5 variants that differ in KIND, each with a named weakest_link, and at least one stepping-stone (or an explicit rationale for not having one).

## Compact interface discovery

If you need the exact compact contract, run `haft interface solution.explore --json`.
Use it as discovery; do not paste long MCP schemas or CLI help into the
session. For large payloads prefer:

```bash
haft artifact create solution.explore --input-file <input.json> --json
```

`mcp__haft__haft_solution(action="explore", ...)` remains the compatible fallback.

## PatternUse Gateway

Before substantive reasoning or work-shaping, call
`mcp__haft__haft_query(action="pattern_use", mode="compact", query="<operator concern>")`.
Skip only for mechanical/status/exact-lookup work where no FPF pattern choice is
material. If `should_use_pattern=true` and this skill needs detail, request
`mode="full"` and carry the returned output shape, boundary, evidence need, and
blocked stronger use. Do not inline the FPF catalog in this skill. PatternUse
is not approval, not evidence, not a DecisionRecord, not MethodPack, and not a
gate.

## Step 1 — Ensure the problem is framed (agent infers first, asks only on real ambiguity)

If no `problem_ref` is in the operator's request:
- **First**: call `mcp__haft__haft_query(action="status")` and read recent active problems. If exactly one matches the operator's current topic, use it and tell the operator "exploring against prob-XXX (the recent X problem) — say so if you meant a different one."
- **Second**: if multiple plausible matches exist, surface 2-3 candidates and ask which one (legitimate ambiguity).
- **Third**: if the operator describes a problem inline and no recent match exists, call `mcp__haft__haft_problem(action="frame", ...)` first per the h-frame procedure — the agent does the framing, then proceeds to explore.

Without a problem reference the exploration floats. But asking the operator to pick from a list before trying inference is delegation back; default is infer-then-act.

## Step 2 — Generate variants in parallel for diversity (optional but recommended)

For genuine diversity force the candidates to come from distinct directions. Spawn 3-5 Agent subagents IN THE SAME MESSAGE, each instructed to produce ONE variant from a different conceptual direction:

```
Agent(
  description="Generate variant #1 (data-flow restructure direction)",
  prompt="
    Problem: <stabilized signal + acceptance from the ProblemCard>

    Your direction (assigned, distinct from siblings): <data flow restructure>
    Other agents are producing variants from other directions
    (caching, batching, infrastructure swap, etc.). Do NOT step on their
    territory — give the operator the variant that comes from YOUR
    direction.

    Return EXACTLY:
    title: <short variant name>
    description: <2-3 sentence sketch of the approach>
    novelty_marker: <what makes this different from typical AI suggestions>
    weakest_link: <what bounds this option's quality if pursued — the
      Achilles' heel, NOT the title repeated>
    stepping_stone: true | false
    stepping_stone_basis: <if true, what future search space this opens>
    risks: [<risk>, ...]
    strengths: [<strength>, ...]
  "
)
```

Parallel directions to consider (pick 3-5 that fit the problem):
- Data-flow restructure (avoid the need for the current operation)
- Algorithmic alternative (same operation, different algorithm)
- Infrastructure swap (different runtime/service/library)
- Caching/batching/queuing (smooth load patterns)
- Architectural extraction (move responsibility to different layer)
- Workflow restructure (change when/how operation is triggered)
- Stepping stone (suboptimal now but opens novel future path)

## Step 3 — Alternative: serial generation (when parallel overkill)

For lightweight exploration (tactical mode, <5 minute task) you can generate variants directly without subagents. Still enforce:
- ≥2 variants (kernel rejects fewer)
- Each variant has weakest_link (kernel rejects empty)
- Each variant has novelty_marker (kernel rejects empty)
- Variants differ in KIND, not degree (your judgment; kernel emits soft warning if titles look similar)

## Step 4 — Early spec-fit probe before recording variants

If the project has active SpecSections and any variant may change project
behavior, duties, boundaries, evidence requirements, or governed code, run a
read-only probe before calling `haft_solution(action="explore")`:

```
mcp__haft__haft_query(
  action="spec_fit_probe",
  probe={
    "problem_signal": "<ProblemCard signal>",
    "scope": "<portfolio scope>",
    "variants": [
      {"id":"V1","title":"<title>","description":"<description>"},
      {"id":"V2","title":"<title>","description":"<description>"}
    ]
  }
)
```

Use `variant_spec_fit` to annotate risks before recording:

- `relates_existing` — variant likely fits existing active SpecSections.
- `spec_gap` — variant probably needs an h-spec draft section/spec delta.
- `conflict` / `outside_current_spec` — keep it visible as a spec-changing
  variant; do not hide that burden until h-decide.
- `no_signal` — do not claim spec coverage.

This is advisory and read-only. It does not approve specs and does not replace
decision-time `spec_binding_preflight`.

## Step 5 — Call kernel

```
mcp__haft__haft_solution(
  action="explore",
  problem_ref="<prob-...>",
  variants=[
    {
      "title": "<variant 1 name>",
      "description": "<approach>",
      "novelty_marker": "<distinct from siblings>",
      "weakest_link": "<what bounds quality>",
      "stepping_stone": false,
      "risks": ["<risk>"],
      "strengths": ["<strength>"]
    },
    // ... 3-5 variants
  ],
  no_stepping_stone_rationale="<required if no variant has stepping_stone=true>",
  mode="<inherit from problem; tactical OK for low-stakes>"
)
```

The kernel returns the SolutionPortfolio ID. Soft warnings may flag disguised duplicates (titles too similar), missing parity_rules for 3+ variants, or weakest_links that just repeat titles — read and self-correct if needed.

## Step 6 — Present to operator

Surface:
- Each variant with its weakest_link and novelty_marker
- Identify any that look weak on the surface but might be stepping stones
- Recommend next step:
  - `/h-compare` to evaluate variants against declared dimensions and pick a Pareto front
  - More exploration if variants converge too tightly (operator may want a wider net)

**Re-grounding discipline (FPF A.7).** Every bare variant label and
artifact ID in your operator-facing text (`V1`, `V2`, `sol-...`, returned
portfolio refs) MUST be paired with its human-readable title or key
claim on first mention in any paragraph/table/summary. Bare `V3 dominates
V1` is opaque after a long session; `V3 (data-flow restructure) dominates
V1 (algorithmic alternative)` restores the object behind the carrier.
Keep IDs — they are needed for traceability — but never let them stand
alone. See CLAUDE.md Critical Reminders for the project-wide rule.

## Curation gate — flag uncertain reasoning (dec-20260603-732219b6)

Your drafted rationale (weakest-links, risks, strengths, novelty markers) is
broad-but-noisy — most points help, a small fraction mislead. When you present
it, do not give the operator a flat wall to over-read or rubber-stamp. Bucket
each argument by your confidence and lead with the doubtful ones:

- **⚠ Uncertain — scrutinize** — arguments you are NOT sure are correct or
  load-bearing. Surface FIRST and prominently.
- **Helpful (secondary)** / **overlaps the obvious** — list compactly, skim-only.

Honesty rule (an invariant of this decision): never down-rank a low-confidence
argument into "helpful" to look tidy, and never hide it — false tidiness makes
the operator curate LESS carefully than a flat list would. If nothing is
genuinely uncertain, say so; do not invent an uncertain item to fill the bucket.

## Final step — say it in plain words (MANDATORY, goes LAST)

The recorded portfolio — variants, weakest-links, novelty markers, IDs — is for traceability. It is NOT how you explain the options to the operator. ALWAYS end your reply with a short plain-language section the operator can read on its own. The most common failure of this skill is dumping the variant table plus IDs and stopping — the operator then cannot tell what the real choice is.

End with an `## In plain words` header, then:
- **What we're actually deciding** — one sentence.
- **The real options** — 2-4 of them in plain words, one line each: what it does + the main catch.
- **Which you'd lean toward and why** — in human terms.
- **What I need from you** — the one question to move forward.

Hard rules for this section:
- ZERO artifact IDs (`sol-…`, `prob-…`, `V1`/`V2`). Name each option by what it DOES.
- ZERO undefined FPF jargon (NQD, stepping-stone, weakest-link, Pareto) — replace with plain words.
- Short. If the operator cannot choose from THIS section alone, it failed.

## Ceremony

This workflow inherits its mode from the framed problem. If you are exploring
without a frame and a build will follow, right-size the effort first:
`mcp__haft__haft_query(action="ceremony", files=[...])` — the floor recommends a
mode and never lets a high-risk change run tactical.

## What NOT to do

- Do not produce 2-3 variants of the same approach (cache LRU vs cache LFU vs cache TTL — all caching). Force at least one out-of-kind alternative.
- Do not name weakest_link with the variant title verbatim. The weakest_link is the FAILURE MODE, not the feature description.
- Do not stop at one variant; if the operator can only think of one, prompt them to find a true alternative or document explicitly that no rival exists.
- Do not skip novelty_marker — without it the agent (or future operator) cannot tell why this variant was worth exploring.
- Do not record stepping-stone variants without `stepping_stone_basis` — bare claim is theatre.
- Do not commit to a chosen variant in this skill; `/h-decide` is where commitments are recorded and is manual-only per Transformer Mandate.

## FPF spec references

- B.5.2 — Abductive loop (parent procedure)
- B.5.2.1 — Creative Abduction with NQD (forced diversity)
- C.18 — Open-ended Search Calculus (NQD-CAL)
- EXP-01 (abductive loop), EXP-04 (WLNK per variant), EXP-05 (stepping stones), EXP-07 (Pareto/portfolio thinking), EXP-08 (NQD novelty)

Look up via `mcp__haft__haft_query(action="fpf", query="C.18")`.
