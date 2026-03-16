# Example: Evidence Governance Architecture Session

This document captures a real quint-code session where we designed the evidence governance system for quint-code itself. It demonstrates the full FPF cycle, including hypothesis evolution, context management, and DRR creation.

## The Problem

The session started with a fundamental question: **What is the real value of quint-code?**

The insight: DRR invariants (Laws) are established via evidence. When evidence becomes stale, those invariants become uncertain. The precious thing quint-code should monitor is: **"Are the invariants we discovered still true?"**

This led to investigating FPF B.3.4 which defines three governance actions for stale evidence:
- **Refresh** — produce new evidence
- **Deprecate** — downgrade assurance level
- **Waive** — explicit temporary acceptance of staleness

## Phase 1: Initial Hypotheses

We created a decision context and proposed 8 hypotheses exploring different architectural approaches:

```
/q-internalize
quint_context(title="Evidence-Centric Architecture: Internalize, Implement, Evidence Graph")
```

| # | Hypothesis | Approach |
|---|------------|----------|
| H1 | Evidence Graph First | Radical: abolish internalize, build EPV-DAG core |
| H2 | Minimal Evidence Paths | Conservative: extend existing, keep internalize lean |
| H3 | Implement-Centric | Evidence graph serves implementation |
| H4 | Carrier-First SCR/RSCR | File hash tracking as foundation |
| H5 | Evidence Lifecycle Auto-Supersede | Same carrier → supersede old evidence |
| H6 | Explicit quint_refresh | New tool for refresh action |
| H7 | Smart Internalize | Actionable staleness report |
| H8 | Batch Carrier Refresh | Multiple files per quint_test |

**Lesson:** Generate diverse options including both conservative and radical approaches. Don't converge too early.

## Phase 2-3: Verification and Validation

Each hypothesis went through `/q2-verify` (logical checks) and `/q3-validate` (empirical validation):

```
quint_verify(hypothesis_id="carrier-first-scr-rscr...", checks_json={...}, verdict="PASS")
quint_test(hypothesis_id="carrier-first-scr-rscr...", test_type="internal", verdict="PASS")
```

**Results after first round:**
- H1, H3, H6 → Failed or invalidated (too radical, wrong focus, unnecessary)
- H2, H4, H5, H7, H8 → Promoted to L2

## Key Insight: New Hypotheses Mid-Cycle

During investigation, the user realized we needed to track **DRR Laws → Evidence** relationships. This spawned three new hypotheses in the same context:

| # | Hypothesis | Purpose |
|---|------------|---------|
| H9 | Invariant Validity Chain | Laws tracked via evidence freshness |
| H10 | Deprecate Action | Auto-downgrade when evidence stale |
| H11 | Waive Action | Explicit temporary acceptance |

**Lesson:** FPF cycles aren't linear. New insights during investigation should generate new hypotheses in the same context. Use `quint_propose` with the same `decision_context`.

```
quint_propose(
    title="Invariant Validity Chain...",
    decision_context="dc-evidence-centric-architecture-internalize-implement-evidence-graph",
    ...
)
```

## The Branching Point: Implementation Strategy

When we reached H11 (Waive Action), a question arose: **Do we need a new `quint_waive` tool?**

This spawned a **sub-decision** exploring implementation options:

```
quint_context(title="Waive Action Implementation Strategy")
```

| Option | Approach |
|--------|----------|
| W1 | Extend quint_resolve with waive resolution |
| W2 | Extend quint_test with acknowledge mode |
| W3 | No explicit waive (agent discipline) |
| W4 | Unified quint_reconcile tool |

After verification:
- W1 → **Failed** (quint_resolve is DRR-only, waive applies to any holon)
- W2, W3, W4 → Passed to L2

## Context Merging

The user recognized that W4 (quint_reconcile) wasn't a separate decision—it was the **implementation strategy for H11**. Instead of two DRRs, we should merge:

```
quint_reset(reason="Merging into main context - waive will be implemented via quint_reconcile")
```

Then we proposed H12 in the **main context** as a refinement:

```
quint_propose(
    title="Waive via quint_reconcile: Unified Governance Tool",
    decision_context="dc-evidence-centric-architecture-internalize-implement-evidence-graph",
    content="Instead of new quint_waive tool, rename quint_resolve → quint_reconcile..."
)
```

**Lesson:** If a sub-decision is really "how to implement X from parent context", merge it back. One DRR is cleaner than two coupled DRRs.

## Dependency Linking

H12 built on the analysis from W4. We linked them:

```
quint_link(
    source_id="waive-via-quint-reconcile-unified-governance-tool",
    target_id="unified-quint-reconcile-all-holon-state-transitions"
)
```

This enables:
- WLNK propagation (H12's R_eff ≤ W4's R_eff)
- Audit trail showing architectural coupling
- Inherited constraints from dependency

## Final Architecture

The winning set formed a coherent system:

```
FOUNDATION
└── H4: Carrier-First SCR/RSCR (file hash tracking)

LIFECYCLE
├── H5: Auto-Supersede (same carrier → replace old evidence)
└── H8: Batch Refresh (multiple files per test)

GOVERNANCE (FPF B.3.4 triad)
├── Refresh: via quint_test (existing)
├── H10: Deprecate (auto-set NeedsReverification)
└── H12: Waive via quint_reconcile (unified tool)

INVARIANT TRACKING
└── H9: Law → Evidence validity chain

SURFACE
├── H2: Minimal Evidence Paths (lean internalize)
└── H7: Smart Internalize (actionable warnings)
```

## The DRR

Final decision created via:

```
quint_decide(
    title="Evidence-Centric Architecture with quint_reconcile Governance",
    winner_id="waive-via-quint-reconcile-unified-governance-tool",
    rejected_ids=["evidence-graph-first...", "implement-centric...", "explicit-refresh...", "waive-action-explicit..."],
    contract={
        "laws": [...],
        "admissibility": [...],
        "acceptance_criteria": [...],
        "evidence": [...],
        "affected_scope": [...]
    }
)
```

### L/A/D/E Contract Highlights

**Laws (MUST be true):**
- quint_reconcile validates action is FPF-legal for holon type
- DRR holons only accept: implemented, abandoned, superseded
- Waiver evidence must have valid_until (time-bounded)

**Admissibility (MUST NOT happen):**
- No new MCP tools for governance actions
- No waive action on DRR type holons
- No permanent waivers

**Acceptance Criteria:**
- quint_reconcile(DRR, implemented) → existing behavior
- quint_reconcile(hypothesis, waive, reason, valid_until) → waiver evidence
- quint_reconcile(DRR, waive) → error with valid actions list

## Lessons Learned

### 1. Hypotheses Evolve

Don't treat Phase 1 as "generate all ideas upfront". New insights during verification/validation should spawn new hypotheses. The FPF cycle is iterative within a context.

### 2. Context Scope Matters

If you create a sub-context and realize it's actually "how to implement hypothesis X from parent", merge back. One DRR with rich detail beats two loosely coupled DRRs.

### 3. Link Dependencies Explicitly

When one hypothesis builds on another's analysis, use `quint_link`. This enables:
- WLNK propagation
- Audit trail
- Constraint inheritance

### 4. Rejection is Valuable

H11 (new quint_waive tool) was rejected in favor of H12 (quint_reconcile). The rejection rationale is documented in the DRR, preventing future re-litigation of "why didn't we just add a new tool?"

### 5. L/A/D/E Contract Prevents Drift

The contract captures not just what to build, but:
- **Laws** — invariants that must remain true
- **Admissibility** — anti-patterns to avoid
- **Acceptance Criteria** — how to verify completion
- **Evidence** — test strategies

This prevents implementation drift and provides clear acceptance gates.

## Session Statistics

- **Hypotheses proposed:** 12 (H1-H12) + 4 (W1-W4)
- **Promoted to L2:** 9
- **Rejected/Invalid:** 7
- **Decision contexts:** 2 (1 merged back)
- **Final DRR:** 1 comprehensive decision
- **Affected files:** 7

## Resulting Workflow (Post-Implementation)

```
Evidence becomes stale (valid_until exceeded or carrier file changed)
    ↓
internalize surfaces warning with Law validity impact
    ↓
Agent chooses governance action:
    ├── /q-reconcile <holon> refresh  → re-run verification
    ├── /q-reconcile <holon> waive "reason" 30d  → time-bounded acceptance
    └── /q-reconcile <holon> deprecate  → mark needs re-verification
    ↓
DRR invariants remain tracked and monitored
```

---

*This session demonstrates quint-code's meta-capability: using FPF to design FPF tooling. The evidence governance system was designed using the very evidence governance principles it implements.*
