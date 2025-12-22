# Advanced: FPF Deep Dive

This document covers the theoretical foundations and fine-tuning options for Quint Code's reasoning engine.

## Agent Configuration

For best results, we recommend adding FPF context to your AI's system instructions.

### Option 1: Use Our Reference File

Copy the [`CLAUDE.md`](../CLAUDE.md) from this repository as a starting point for your project's agent instructions. It's optimized for software engineering work with FPF.

### Option 2: Add the Glossary

At minimum, copy the **FPF Glossary** section (below) to your:
- `CLAUDE.md` (Claude Code)
- `.cursorrules` or `AGENTS.md` (Cursor)
- Agent system prompts (other tools)

This helps the AI understand FPF concepts without re-explanation each session.

---

## FPF Glossary

### Knowledge Layers (Epistemic Status)

| Layer | Name | Meaning | How to reach |
|-------|------|---------|--------------|
| **L0** | Conjecture | Unverified hypothesis or note | `/q1-hypothesize` |
| **L1** | Substantiated | Passed logical consistency check | `/q2-verify` |
| **L2** | Corroborated | Empirically tested and confirmed | `/q3-validate` |
| **Invalid** | Falsified | Failed verification (kept for learning) | FAIL verdict |

### Core Terms

**Holon** — A knowledge unit (hypothesis, decision, evidence) stored in `.quint/`. Holons have identity, layer, kind, and assurance scores.

**Kind** — Classification of holon:
- `system` — Code, architecture, technical implementation
- `episteme` — Process, documentation, methodology

**Scope (G)** — Where a claim applies. Example: "Redis caching" might have scope "read-heavy endpoints, >1000 RPS".

**R_eff (Effective Reliability)** — Computed trust score (0-1). Calculated via `/q4-audit`, never estimated.

**WLNK (Weakest Link)** — R_eff = min(evidence_scores), never average. A chain is only as strong as its weakest link.

**CL (Congruence Level)** — How well evidence transfers across contexts:
- **CL3:** Same context (internal test) — no penalty
- **CL2:** Similar context (related project) — minor penalty
- **CL1:** Different context (external docs) — significant penalty

**DRR (Design Rationale Record)** — Persisted decision with context, rationale, and consequences. Created via `/q5-decide`.

**Bounded Context** — The vocabulary and constraints of your project. Recorded in `.quint/context.md`.

**Epistemic Debt** — Accumulated staleness when evidence expires. Managed via `/q-decay`.

**Transformer Mandate** — Systems cannot transform themselves. AI generates options; humans decide. Autonomous architectural decisions = protocol violation.

### State Machine

```
IDLE → ABDUCTION → DEDUCTION → INDUCTION → DECISION → IDLE
       (q1)         (q2)         (q3)        (q4→q5)
```

Each phase has preconditions. Skipping phases blocks the next tool.

---

## Assurance Calculations

### WLNK (Weakest Link Principle)

The assurance of a claim is never an average of its evidence — it reflects its most fragile dependency.

If you have three pieces of evidence supporting a hypothesis — two with high reliability (`R=0.9`) and one with low reliability (`R=0.2`) — the effective reliability is capped by the weakest link: `R=0.2`.

This prevents trust inflation and ensures weak points are always visible.

### Congruence Penalty

External evidence (documentation, benchmarks, research) is only valuable if it's relevant to your situation. The **Congruence Level (CL)** rates how well external evidence matches your **Bounded Context**.

| Level | Example | Penalty |
|-------|---------|---------|
| CL3 (High) | Benchmark on identical hardware/OS/versions | None |
| CL2 (Medium) | Benchmark on similar configuration | Minor |
| CL1 (Low) | General principle from a blog post | Significant |

The assurance calculator applies congruence penalties, reducing effective reliability of evidence that isn't a perfect match.

### Evidence Decay

Evidence expires. That benchmark from six months ago? The library has been updated twice since then.

Every piece of evidence has a `valid_until` date. When evidence expires, the decision it supports becomes **questionable** — not necessarily wrong, just unverified.

The `/q-decay` command shows what's stale and offers three options:
- **Refresh** — Re-run tests to get fresh evidence
- **Deprecate** — Downgrade the hypothesis if the decision needs rethinking
- **Waive** — Accept the risk temporarily with documented rationale

See [Evidence Freshness](evidence-freshness.md) for the full guide.

---

For workflow details and command reference, see [Quick Reference](fpf-engine.md).

## Further Reading

- [FPF Repository](https://github.com/ailev/FPF)
