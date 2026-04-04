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
- `portfolio_ref`: portfolio ID
- `invariants`: what MUST hold at all times
- `post_conditions`: checklist after (definition of done)
- `admissibility`: what is NOT acceptable
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

## Evidence workflow after deciding

- Use `haft_decision(action="evidence", artifact_ref="<decision-id>", evidence_content="...", evidence_type="benchmark|test|research|audit", evidence_verdict="supports|weakens|refutes", valid_until="...")` when you have explicit supporting or contradictory artifacts and their freshness matters.
- Use `haft_decision(action="baseline", decision_ref="<decision-id>")` before `measure` when the decision has `affected_files`.
- Use `haft_decision(action="measure", ...)` for post-implementation outcome. Attached evidence complements measure; it does not replace it.

$ARGUMENTS
