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

System assurance equals the minimum of its evidence assurance levels, never the average.

If you have three pieces of evidence at L2, L2, and L0 — your claim is L0.

### Congruence

External evidence (documentation, benchmarks, research) must be evaluated for how well it matches your specific context:

- **High**: Direct match to your tech stack, scale, and constraints
- **Medium**: Similar but not identical context
- **Low**: General principle, may not apply

### Validity (Evidence Decay)

Evidence expires. A benchmark from 2 years ago may not reflect current library performance. Use `/q-decay` to check freshness and flag stale evidence for re-validation.
