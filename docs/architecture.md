# Architecture

## Surface vs. Grounding

Quint Code strictly separates the **User Experience** from the **Assurance Layer**.

### Surface (What You See)

Clean, concise summaries in the chat. When you run a command like `/q1-hypothesize`, you get a readable output that shows:

- Generated hypotheses with brief descriptions
- Current assurance levels
- Recommended next steps

The surface layer is optimized for cognitive load — you shouldn't need to parse JSON or navigate file trees during active reasoning.

### Grounding (What Is Stored)

Behind the surface, detailed structures are persisted:

```
.quint/
├── knowledge/
│   ├── L0/          # Unverified observations and hypotheses
│   ├── L1/          # Logically verified claims
│   ├── L2/          # Empirically validated claims
│   └── invalid/     # Disproved claims (kept for learning)
├── evidence/        # Supporting documents, test results, research
├── drr/             # Design Rationale Records (final decisions)
├── agents/          # Persona definitions
├── context.md       # Bounded context snapshot
└── quint.db         # SQLite database for queries and state
```

This ensures you have a rigorous audit trail without cluttering your thinking process.

## Agents vs. Personas

In FPF terms, an **Agent** is a system playing a specific **Role**. Quint Code operationalizes this as **Personas**:

- **No Invisible Threads:** Unlike "autonomous agents" that run in the background, Quint Code Personas (e.g., *Abductor*, *Auditor*) run entirely within your visible chat thread.
- **You Are the Transformer:** You execute the command. The AI adopts the Persona to help you reason constraints-first.
- **Strict Distinction:** We call them **Personas** in the CLI to avoid confusion, but they are architecturally **Agential Roles** (A.13) defined in `.quint/agents/`.

## The Transformer Mandate

A system cannot transform itself. This is why:

1. **AI generates options** — hypotheses, evidence, analysis
2. **Human decides** — selects the winner, approves the DRR

The AI can recommend, but architectural decisions flow through human judgment. This isn't a limitation; it's the design.

## State Model: Repository as Bounded Context

The source of truth is the repository itself — not an in-memory FSM variable:

- `.quint/knowledge/L0/`, `L1/`, `L2/` — hypothesis files
- `.quint/quint.db` — SQLite with relations and evidence
- `.quint/decisions/` — finalized DRRs

The FSM phase shown in `/q-internalize` is a **projection** of this state, not an independent variable. It's computed from what exists, not stored separately.

### Semantic Preconditions vs Phase Gates

Traditional state machines gate operations by phase: "you're in DEDUCTION, so you can't call decide()". This creates sync problems — the FSM can drift from reality.

Quint uses **semantic preconditions** instead. Each command checks actual artifacts:

| Command | Checks | Blocks if |
|---------|--------|-----------|
| `quint_verify` | hypothesis exists in L0 | not found in L0 |
| `quint_test` | hypothesis in L1 or L2 | still in L0 |
| `quint_audit` | hypothesis in L2 | not in L2 |
| `quint_decide` | at least one L2 exists | no L2 holons |

### What You Can and Can't Do

**Cannot do (logically invalid):**
- Verify a hypothesis that doesn't exist
- Test something that hasn't been verified
- Decide without any validated options

**Can do freely:**
- Multiple `propose` in a row
- Verify hypotheses in any order
- Re-test L2 to refresh evidence
- Work on multiple decision contexts in parallel

### Why This Design

1. **No sync issues** — can't have "FSM thinks X, files say Y"
2. **Multiple sessions** — all agents see the same files, no state conflicts
3. **Git-friendly** — `git reset` resets "state" too (because state IS the files)
4. **Idempotent** — preconditions always check current reality, not cached assumptions

This is "functional core" applied to state management: the state is immutable snapshots in the filesystem, the FSM phase is a derived view.

## Knowledge Assurance Levels

| Level | Name | Meaning | Promotion Path |
|-------|------|---------|----------------|
| L0 | Observation | Unverified hypothesis or note | → `/q2-verify` |
| L1 | Substantiated | Passed logical consistency check | → `/q3-validate` |
| L2 | Verified | Empirically tested and confirmed | Terminal |
| Invalid | Disproved | Failed verification (kept for learning) | Terminal |

### WLNK (Weakest Link Principle)

The assurance of a claim is never an average of its evidence; it is a reflection of its most fragile dependency.

If you have three pieces of evidence supporting a hypothesis—two with high reliability (`R=0.9`) and one with low reliability (`R=0.2`)—the effective reliability of your claim is not the average. It is capped by the weakest link: `R=0.2`. Quint Code's assurance calculator strictly enforces this conservative principle, preventing trust inflation and ensuring that weak points in an argument are always visible.

### Congruence (CL)

External evidence (documentation, benchmarks, research) is only valuable if it is relevant to your specific situation. The **Congruence Level (CL)** is a rating of how well that external evidence matches your project's **Bounded Context**.

- **High (CL3):** A benchmark run on the exact same hardware, OS, and software versions as your production environment.
- **Medium (CL2):** A benchmark run on a similar, but not identical, configuration.
- **Low (CL1):** A general architectural principle described in a blog post.

Quint's assurance calculator applies a **Congruence Penalty** based on the CL, reducing the effective reliability of evidence that isn't a perfect match for your context.

### Validity (Evidence Freshness)

Evidence expires. That benchmark you ran six months ago? The library has been updated twice since then. Your numbers might not be accurate anymore.

Every piece of evidence has a `valid_until` date. When evidence expires, the decision it supports becomes **questionable** — not necessarily wrong, just unverified. The `/q-decay` command shows you what's stale and lets you:

- **Refresh** — Re-run tests to get fresh proof
- **Deprecate** — Downgrade the hypothesis if the decision needs rethinking
- **Waive** — Accept the risk temporarily with documented rationale

This makes hidden risk visible. You know exactly which decisions are operating on outdated assumptions.

See [Evidence Freshness](evidence-freshness.md) for the full guide.
