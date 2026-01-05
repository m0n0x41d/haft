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

To find decisions awaiting resolution, run `/q-query` or ask: *"What decisions are still open?"*

## Dependency Discovery

When you propose a hypothesis, Quint Code automatically searches for related existing knowledge.

### How It Works

1. You run `/q1-hypothesize` with your problem
2. The system searches existing decisions and hypotheses for related concepts
3. If matches found, you'll see suggestions:

```
⚠️ POTENTIAL DEPENDENCIES DETECTED

Related holons found:
  • Redis Cache Layer [DRR]
  • Redis Connection Pool [L2]

You might want to link your new hypothesis to these.
```

### Declaring Dependencies

When creating hypotheses, tell the agent about dependencies:

```
User: Generate hypotheses for rate limiting.
      This will depend on our existing Redis cache decision.

Agent: [Creates hypothesis with dependency on redis-cache-drr]
       Your new hypothesis inherits the reliability score from Redis cache.
```

If you forgot to mention dependencies during creation, just say:

```
User: Link my rate-limiter hypothesis to the redis-cache decision.

Agent: Linked. Rate-limiter's reliability is now capped by redis-cache's score.
```

### Why Dependencies Matter

Dependencies enable **WLNK (Weakest Link)** — your hypothesis can't be more reliable than its dependencies. If your API depends on an auth module with R=0.7, your API's reliability is capped at 0.7.

This makes architectural coupling visible:
- **System** dependencies become "components" (cache-layer is part of api-service)
- **Epistemic** dependencies become "constituents" (benchmark supports claim)

### How Relations Work

Relations flow **source → target**:

| Relation | What It Means | Example |
|----------|--------------|---------|
| `componentOf` | Part of a system | cache-layer is part of api-service |
| `constituentOf` | Supports a claim | benchmark supports performance-claim |
| `memberOf` | Belongs to group | option-A belongs to caching-decision |
| `selects` | Decision picks winner | DRR selects redis-caching |
| `rejects` | Decision rejects loser | DRR rejects memcached |
| `closes` | Resolves context | implementation closes DRR |

You don't need to remember these — just describe what depends on what, and the agent handles the rest.

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

2. `/q-implement` returns an **implementation directive** — a checklist for the work:

```markdown
## Invariants to Implement
- Cache misses must fall through to DB transparently
- TTL must be configurable per entity type

## Inherited from Redis Connection decision:
- Connection pool must be bounded
- Reconnection must be exponential backoff

## Final Verification
Before closing, verify:
- [ ] No hardcoded TTL values
- [ ] Connection pool limits respected
```

3. You implement the feature normally.

4. When done, run `/q-resolve` to close the decision:

```
User: Mark the caching decision as implemented, linked to PR #42.

Agent: Decision closed. Evidence recorded linking DRR to implementation.
```

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
