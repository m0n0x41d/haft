# The FPF Engine

Quint Code implements the **First Principles Framework (FPF)** — a methodology for structured reasoning developed by [Anatoly Levenchuk](https://ailev.livejournal.com/).

## The ADI Cycle

The workflow follows the Canonical Reasoning Cycle (Pattern B.5), consisting of three inference modes:

### 1. Abduction (`/q1-hypothesize`)

**What:** Generate plausible, competing hypotheses.

**How it works:**
- You pose a problem or question
- The AI (as *Abductor* persona) generates 3-5 candidate explanations or solutions
- Each hypothesis is stored in `L0/` (unverified observations)
- No hypothesis is privileged — anchoring bias is the enemy

**Output:** Multiple L0 claims, each with:
- Clear statement of the hypothesis
- Initial reasoning for plausibility
- Identified assumptions and constraints

### 2. Deduction (`/q2-verify`)

**What:** Logically verify the hypotheses against constraints and typing.

**How it works:**
- The AI (as *Verifier* persona) checks each L0 hypothesis for:
  - Internal logical consistency
  - Compatibility with known constraints
  - Type correctness (does the solution fit the problem shape?)
- Hypotheses that pass are promoted to `L1/`
- Hypotheses that fail are moved to `invalid/` with explanation

**Output:** L1 claims (logically sound) or invalidation records.

### 3. Induction (`/q3-validate`)

**What:** Gather empirical evidence through tests or research.

**How it works:**
- For **internal** claims: run tests, measure performance, verify behavior
- For **external** claims: research documentation, benchmarks, case studies
- Evidence is attached with:
  - Source and date (for decay tracking)
  - Congruence rating (how well does external evidence match our context?)
- Claims that pass validation are promoted to `L2/`

**Output:** L2 claims (empirically verified) with evidence chain.

## Post-Cycle: Audit and Decision

### 4. Audit (`/q4-audit`)

Compute trust scores using:

- **WLNK (Weakest Link):** Assurance = min(evidence levels)
- **Congruence Check:** Is external evidence applicable to our context?
- **Bias Detection:** Are we anchoring on early hypotheses?

### 5. Decision (`/q5-decide`)

- Select the winning hypothesis
- Generate a **Design Rationale Record (DRR)** (Pattern E.9)
- DRR captures: decision, alternatives considered, evidence, and expiry conditions

## Commands Reference

| Command | Phase | What It Does |
|---------|-------|--------------|
| `/q0-init` | Setup | Initialize `.quint/` and record Bounded Context |
| `/q1-hypothesize` | Abduction | Generate 3-5 L0 hypotheses |
| `/q1-add` | Abduction | Inject a user-provided hypothesis into L0 |
| `/q2-verify` | Deduction | Verify logic/types, promote L0 → L1 |
| `/q3-validate` | Induction | Run tests or research, promote L1 → L2 |
| `/q4-audit` | Audit | WLNK analysis and bias check |
| `/q5-decide` | Decision | Finalize and create DRR |
| `/q-status` | Utility | Show current phase and state |
| `/q-query` | Utility | Search knowledge base |
| `/q-decay` | Maintenance | Check for expired evidence |

## When to Use FPF

**Use it for:**
- Architectural decisions with long-term consequences
- Multiple viable approaches requiring systematic evaluation
- Decisions that need an auditable reasoning trail
- Building up project knowledge over time

**Skip it for:**
- Quick fixes with obvious solutions
- Easily reversible decisions
- Time-critical situations where the overhead isn't justified

## Further Reading

- [Anatoly Levenchuk's work on Systems Thinking](https://ailev.livejournal.com/)
- FPF Specification (Patterns A.13, B.5, E.9, E.14)
