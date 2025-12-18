# Architecture

## Surface vs. Grounding (Pattern E.14)

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

## Knowledge Assurance Levels

| Level | Name | Meaning | Promotion Path |
|-------|------|---------|----------------|
| L0 | Observation | Unverified hypothesis or note | → `/q2-verify` |
| L1 | Reasoned | Passed logical consistency check | → `/q3-validate` |
| L2 | Verified | Empirically tested and confirmed | Terminal |
| Invalid | Disproved | Failed verification (kept for learning) | Terminal |

### WLNK (Weakest Link Principle)

**FPF Pattern:** B.1 Invariant Quintet

The assurance of a claim is never an average of its evidence; it is a reflection of its most fragile dependency.

If you have three pieces of evidence supporting a hypothesis—two with high reliability (`R=0.9`) and one with low reliability (`R=0.2`)—the effective reliability of your claim is not the average. It is capped by the weakest link: `R=0.2`. Quint Code's assurance calculator strictly enforces this conservative principle, preventing trust inflation and ensuring that weak points in an argument are always visible.

### Congruence (CL)

**FPF Pattern:** B.3 Trust & Assurance Calculus

External evidence (documentation, benchmarks, research) is only valuable if it is relevant to your specific situation. The **Congruence Level (CL)** is a rating of how well that external evidence matches your project's **Bounded Context**.

- **High (CL3):** A benchmark run on the exact same hardware, OS, and software versions as your production environment.
- **Medium (CL2):** A benchmark run on a similar, but not identical, configuration.
- **Low (CL1):** A general architectural principle described in a blog post.

Quint's assurance calculator applies a **Congruence Penalty** based on the CL, reducing the effective reliability of evidence that isn't a perfect match for your context.

### Validity (Evidence Decay)

**FPF Pattern:** B.3.4 Evidence Decay & Epistemic Debt

Evidence is perishable. A performance benchmark from two years ago is less trustworthy than one from last week because the context (libraries, hardware, compilers) has likely changed.

Every piece of evidence in Quint has a `valid_until` date. The `/q-decay` command scans for expired evidence, and the assurance calculator automatically penalizes the reliability of claims that depend on it. This system makes the "staleness" of knowledge visible and manageable, preventing you from making critical decisions based on outdated information.
