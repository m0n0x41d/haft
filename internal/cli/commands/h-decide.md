---
description: "Finalize a decision with full rationale"
---

# Decide

Create a DecisionRecord — the crown jewel artifact. Must include what was chosen, why, and what to watch for.

Use `haft_decision` tool with `action="decide"` and:
- `selected_title`: name of chosen variant (required)
- `why_selected`: rationale (required)
- `problem_ref`: problem ID
- `portfolio_ref`: portfolio ID
- `invariants`: what MUST hold at all times
- `post_conditions`: checklist after (definition of done)
- `admissibility`: what is NOT acceptable
- `weakest_link`: what bounds reliability
- `valid_until`: expiry date (YYYY-MM-DD)
- `affected_files`: files affected

## Verification Gate (before recording)

Before calling `haft_decision`, run a verification check. The agent that generates a decision cannot be its sole validator (FPF A.12 — External Transformer Principle).

### Tactical mode — one challenge

Present ONE line to the user:

> **Counter-argument:** [strongest argument against this decision in one sentence]
>
> Proceed? (If the counter-argument kills the decision → go back to /h-explore)

Then record the decision. The counter-argument goes into `weakest_link` if it's stronger than the current one.

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

**Incorporate findings:** The counter-argument and tail scenarios should be captured in `weakest_link`, `admissibility`, or `rollback.triggers` — don't just display them, persist them in the decision record.

## Evidence workflow after deciding

- Use `haft_decision(action="evidence", artifact_ref="<decision-id>", evidence_content="...", evidence_type="benchmark|test|research|audit", evidence_verdict="supports|weakens|refutes")` when you have explicit supporting or contradictory artifacts.
- Use `haft_decision(action="baseline", decision_ref="<decision-id>")` before `measure` when the decision has `affected_files`.
- Use `haft_decision(action="measure", ...)` for post-implementation outcome. Attached evidence complements measure; it does not replace it.

$ARGUMENTS
