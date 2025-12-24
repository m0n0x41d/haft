# The FPF Engine

Quint Code implements the **[First Principles Framework (FPF)](https://github.com/ailev/FPF)** — a methodology for structured reasoning developed by Anatoly Levenchuk.

## The ADI Cycle

The workflow follows three inference modes:

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
- Generate a **Design Rationale Record (DRR)**
- DRR captures: decision, alternatives considered, evidence, and expiry conditions

## Commands Reference

| Command | Phase | What It Does |
|---|---|---|
| `/q-internalize` | Entry | **Start here.** Initialize, update context, show state, surface issues. |
| `/q1-hypothesize` | Abduction | Generate L0 hypotheses for a problem. |
| `/q1-add` | Abduction | Manually add your own L0 hypothesis. |
| `/q2-verify` | Deduction | Verify logic and constraints, promoting claims from L0 to L1. |
| `/q3-validate` | Induction | Gather empirical evidence, promoting claims from L1 to L2. |
| `/q4-audit` | Audit | Run an assurance audit and calculate trust scores. |
| `/q5-decide` | Decision | Select the winning hypothesis and create a Design Rationale Record. |
| `/q-implement` | Implementation | Transform DRR into implementation directive with constraints. |
| `/q-resolve` | Resolution | Record decision outcome (implemented, abandoned, superseded). |
| `/q-query` | Utility | Search the project's knowledge base. |
| `/q-reset` | Utility | Discard the current reasoning cycle. |

### Entry Point: /q-internalize

Start every session with `/q-internalize`. It handles:

- **Initialization**: Creates `.quint/` structure if needed
- **Context refresh**: Detects and updates stale project context
- **State loading**: Shows current phase, knowledge counts, recent work
- **Issue surfacing**: Decaying evidence, open decisions pending resolution
- **Guidance**: Phase-appropriate next action suggestions

### Decision Resolution: /q-resolve

Decisions are plans. Reality is what happens. `/q-resolve` bridges the gap:

- **Implemented**: Link to commit, PR, or file where the decision was realized
- **Abandoned**: Record why the decision was dropped
- **Superseded**: Link to the newer decision that replaced this one

Use `quint_search` with `status_filter="open"` to find decisions awaiting resolution.

## Dependency Discovery

When you propose a hypothesis, Quint Code automatically searches for related existing holons using FTS5 semantic matching.

### How It Works

1. You call `/q1-hypothesize` with "Rate limiting using Redis"
2. `quint_propose` extracts keywords and searches existing DRRs, L1, L2 holons
3. If matches found, output shows:

```
⚠️ POTENTIAL DEPENDENCIES DETECTED

Related holons found (ranked by relevance):
  • redis-cache-drr [DRR] Redis Cache Layer
  • redis-connection [L2] Redis Connection Pool

Consider linking with:
  quint_link(source_id="rate-limiter", target_id="redis-cache-drr")
```

### Post-Creation Linking

If you missed `depends_on` during creation, use `quint_link`:

```
quint_link(source_id="my-hypothesis", target_id="existing-drr")
```

This creates:
- **ComponentOf** relation (for `kind=system`)
- **ConstituentOf** relation (for `kind=episteme`)
- **WLNK applies**: Your hypothesis inherits the R_eff ceiling from dependencies

### Why This Matters

- **Architectural coupling visible**: Dependencies are tracked, not implicit
- **WLNK propagation**: If a dependency fails, dependent hypotheses are affected
- **Inherited constraints**: Implementation inherits invariants from dependencies

## Implementation Phase

After a decision (DRR) is created, `/q-implement` transforms it into executable work.

### How It Works

1. DRR includes a **contract** with:
   - Invariants (MUST be true)
   - Anti-patterns (MUST NOT happen)
   - Acceptance criteria (verify before closing)
   - Affected scope (files/modules impacted)

2. `/q-implement` returns an **implementation directive** that programs your planning:

```markdown
## Invariants to Implement
- Cache misses must fall through to DB transparently
- TTL must be configurable per entity type

## Inherited from redis-connection-drr:
- Connection pool must be bounded
- Reconnection must be exponential backoff

## Final Verification
Before calling quint_resolve, verify:
- [ ] No hardcoded TTL values
- [ ] Connection pool limits respected
```

3. You implement using your normal workflow (TodoWrite, etc.)

4. When done, call `quint_resolve` with `criteria_verified=true`

### WLNK for Constraints

Dependencies propagate not just R_eff but also constraints:

```
DRR-jwt-auth (invariant: "tokens stateless")
    ↓ depends_on
DRR-cache-redis
    → Must also satisfy: "tokens stateless"
```

This ensures architectural decisions cascade correctly.

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

- [FPF Repository](https://github.com/ailev/FPF)
