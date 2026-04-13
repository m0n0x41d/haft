---
description: "Finalize a decision with full rationale"
---

# Decide

Create a DecisionRecord — the crown jewel artifact. Must include what was chosen, why, and what to watch for.

Use `haft_decision` tool with `action="decide"` and:
- `selected_title`: name of chosen variant (required)
- `why_selected`: rationale (required)
- `selection_policy`: explicit rule used to choose among the compared variants (required)
- `counterargument`: strongest argument against the chosen option (required)
- `why_not_others`: at least one key rejected alternative with the reason it lost (required)
- `problem_ref`: problem ID
- `problem_refs`: additional problem IDs this decision also addresses when one decision closes multiple linked problem statements
- `portfolio_ref`: portfolio ID
- `pre_conditions`: conditions that must already hold before implementation starts
- `invariants`: what MUST hold at all times
- `post_conditions`: checklist after (definition of done)
- `admissibility`: what is NOT acceptable
- `evidence_requirements`: explicit evidence the implementation/review loop must gather
- `refresh_triggers`: concrete future conditions that should trigger re-evaluation
- `search_keywords`: compact retrieval aliases for later recall
- `predictions`: falsifiable claims to verify later; each item MUST include `claim`, `observable`, and `threshold`
- `weakest_link`: selected variant weakest link; what most plausibly breaks this choice (required)
- `rollback.triggers`: at least one concrete trigger that would force reversal (required)
- `rollback.steps`: rollback actions to take when a trigger fires
- `valid_until`: expiry date (YYYY-MM-DD)
- `affected_files`: files affected

The tool should reject DecisionRecords that omit these anti-self-deception fields. A polished winner story is not enough.

## Verification Gate (before recording)

Before calling `haft_decision`, run a verification check. The agent that generates a decision cannot be its sole validator (FPF A.12 — External Transformer Principle).

### Tactical mode — one challenge

Present ONE line to the user:

> **Counter-argument:** [strongest argument against this decision in one sentence]
>
> Proceed? (If the counter-argument kills the decision → go back to /h-explore)

Then record the decision. Persist the strongest attack in `counterargument` either way; `weakest_link` names the bounding fragility, while `counterargument` preserves the strongest adversarial case against the choice.

### Standard/deep mode — full verification

Present ALL five probes to the user BEFORE calling the tool:

**1. Deductive consequences** (FPF B.5:4.2)
> "If this decision is correct, these 3 things must be true:"
> - [consequence 1]
> - [consequence 2]
> - [consequence 3]
> Check: do any contradict known constraints or existing decisions?

**2. Strongest counter-argument** (FPF A.12)
> "The strongest argument against this decision:"
> [genuine counter-argument — not a strawman]

**3. Self-evidence check** (FPF CC-A12.5)
> "Evidence sourced solely from this conversation: [yes/no]"
> If yes: flag it. Evidence from the same agent session that generated the options is CL1 at best, not CL3.

**4. Tail failure scenarios** (Verbalized Sampling, τ<0.10)
> "Low-probability, high-impact failure modes (<10% each):"
> - [scenario 1]
> - [scenario 2]

**5. WLNK challenge**
> "Stated weakest link: [X]. Is this actually the weakest, or is there something weaker hiding behind it?"

After presenting: if the user confirms, record the decision. If any probe kills the decision, go back to /h-explore.

**Incorporate findings:** Persist the strongest attack in `counterargument`, keep the bounding fragility in `weakest_link`, record the selection rule in `selection_policy`, keep at least one concrete rejected alternative in `why_not_others`, and capture reversal conditions in `rollback.triggers` — don't just display them, persist them in the decision record.

## ALIVE decision contract

When the decision is important enough to govern implementation rather than merely document a choice, persist the operational contract too:

- `pre_conditions`: use for prerequisites such as "benchmark reproduced in CI" or "schema freeze approved"
- `evidence_requirements`: use for proof obligations such as "p99 latency < 20ms in replay benchmark"
- `refresh_triggers`: use for future drift/reopen triggers such as "error budget breached for two releases"
- `predictions`: use for measurable expectations that `haft_decision(action="measure")` can later confirm or weaken

Good prediction example:

```text
predictions=[
  {
    "claim": "Throughput stays above 100k events/sec",
    "observable": "throughput",
    "threshold": "> 100k events/sec"
  }
]
```

## Evidence workflow after deciding

- Use `haft_decision(action="evidence", artifact_ref="<decision-id>", evidence_content="...", evidence_type="benchmark|test|research|audit", evidence_verdict="supports|weakens|refutes", claim_refs=[...], claim_scope=[...], valid_until="...")` when you have explicit supporting or contradictory artifacts and their freshness matters.
- Use `haft_decision(action="baseline", decision_ref="<decision-id>")` before `measure` when the decision has `affected_files`.
- Use `haft_decision(action="measure", ...)` for post-implementation outcome. Attached evidence complements measure; it does not replace it.

Attach evidence to the narrowest justified scope:

- `claim_refs`: explicit claim identifiers when the evidence targets a named claim
- `claim_scope`: semantic scope labels when the evidence targets a slice such as `latency`, `throughput`, or `migration-safety`

Use `claim_scope` even when stable claim IDs are not available yet. This keeps later audit/projection views anchored to the same semantic state instead of free-form narration.

$ARGUMENTS
