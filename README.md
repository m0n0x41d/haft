```
 ______     ______     __  __     ______     __     ______     __         ______       
/\  ___\   /\  == \   /\ \/\ \   /\  ___\   /\ \   /\  == \   /\ \       /\  ___\      
\ \ \____  \ \  __<   \ \ \_\ \  \ \ \____  \ \ \  \ \  __<   \ \ \____  \ \  __\      
 \ \_____\  \ \_\ \_\  \ \_____\  \ \_____\  \ \_\  \ \_____\  \ \_____\  \ \_____\    
  \/_____/   \/_/ /_/   \/_____/   \/_____/   \/_/   \/_____/   \/_____/   \/_____/    
                                                                                       
                      ______     ______     _____     ______                           
                     /\  ___\   /\  __ \   /\  __-.  /\  ___\                          
                     \ \ \____  \ \ \/\ \  \ \ \/\ \ \ \  __\                          
                      \ \_____\  \ \_____\  \ \____-  \ \_____\                        
                       \/_____/   \/_____/   \/____/   \/_____/                        

```

**First Principles Framework for Claude Code** — structured reasoning with auditable evidence trails.

## What Is This?

A set of slash commands for Claude Code that implement **hypothesis-driven reasoning** with explicit evidence and assurance levels. Based on the [First Principles Framework](https://github.com/ailev/FPF) created by Anatoly Levenchuk.
You can read more about FPF on its repository README.md file.

### Okay, what is the actual project exactly?

This project consists of several Claude Code commands and my personal CLAUDE.md file.
Mainly I am a software engineer, which is the reason why this CLAUDE.md is so influenced by software engineering and design principles.

But the main part of this repository is the Claude Code commands that try to provide a minimal and partially implemented FPF reasoning workflow for you.

However, this thing might help you not only with software architecture. Just install Claude Code and these commands, create a repository with the documents related to your personal or business endeavors, and give it a try.

All of this is designed with commands and not with subagents, because you MUST be in the loop at every step and act as an external reviewer-transformer and guide for the reasoning process.

#### What is this lame name?

A crucible is a container made of heat-resistant material used for melting and heating substances at very high temperatures to cast valuable and robust things.

FPF itself is the methodology and framework of melting.
The rails for it, which are attempted to be provided with Claude Code commands, are the crucible for melting.
The materials you are melting are epistemes, hypotheses that the FPF pushes to become... decisions, which are the valuable and robust things.

Read more below and in the original FPF repo README.md. It is okay and expected that it will take some time to get the idea.

Sorry for being bad at naming things.

## When to Use FPF

### ✅ Use FPF When

| Situation | Why FPF Helps |
|-----------|---------------|
| **Architectural decisions** | Long-term consequences need documented rationale |
| **Multiple viable approaches** | Systematic comparison prevents bias |
| **Team decisions** | Auditable trail for async review |
| **Unfamiliar territory** | Structured research reduces risk |
| **Past decisions revisited** | Evidence helps re-evaluate |
| **Complex trade-offs** | WLNK analysis surfaces hidden risks |

**Examples:**

- "Should we use Redis or Memcached?"
- "Microservices vs monolith for this feature?"
- "Which auth strategy fits our threat model?"
- "Buy vs build for this component?"
- "What are the most impactful technical debts in this project and how we can fix it without breaking changes and bloodshed?"

### ❌ Don't Use FPF When

| Situation | What to Do Instead |
|-----------|---------------------|
| **Quick fixes** | Just fix it |
| **Obvious solutions** | Implement directly |
| **Easily reversible** | Try it, iterate |
| **Time-critical** | Use inline Decision Framework |
| **Well-understood patterns** | Apply known patterns |

**Examples:**

- "Fix this null pointer exception" → Just fix
- "Add a button to the form" → Just add
- "Update dependency version" → Just update
- "Debug this bug <PASTEING TRACEBACK>" -> Just paste in CC without fpf commands
- "Help me build a solo entrepreneur 1 billion dollar business" -> No luck here with such zero effort from your side (perhaps...)

### ⚖️ Decision Heuristic to Kick Off With

```
Is this decision:
  - Hard to reverse? → Consider FPF
  - Affecting >1 sprint of work? → Consider FPF
  - Involving multiple unknowns? → Consider FPF
  - Likely to be questioned later? → Consider FPF
  
If none of the above → Skip FPF, use inline Decision Framework
```

---

## Installation

```bash
# Clone the repo
git clone https://github.com/m0n0x41d/crucible-code.git

# Install to your project
./install.sh /path/to/your/project

# Install globally
./install.sh -g

# Or manually copy
cp -r commands/ /path/to/project/.claude/commands/
```

Add the FPF section to your `CLAUDE.md` (see CLAUDE.md in this repo), or adopt my whole CLAUDE.md — I believe it will make your workflow better :)
If you decide to adopt the whole CLAUDE.md, make sure you have configured context7 MCP and PiecesOS with its MCP in Claude Code configs.

---

## Commands Reference

### Core Cycle Commands

| # | Command | Phase | Input | Output |
|---|---------|-------|-------|--------|
| 0 | `/fpf:0-init` | Setup | — | `.fpf/` structure |
| 1 | `/fpf:1-hypothesize` | Abduction | Problem statement | Hypotheses in `L0/` |
| 2 | `/fpf:2-check` | Deduction | L0 hypotheses | Verified in `L1/` |
| 3a | `/fpf:3-test` | Induction | L1 hypotheses | Internal evidence |
| 3b | `/fpf:3-research` | Induction | L1 hypotheses | External evidence |
| 4 | `/fpf:4-audit` | Bias-Audit | All evidence | Risk assessment |
| 5 | `/fpf:5-decide` | Decision | L2 hypotheses | DRR document |

### Utility Commands

| Command | Purpose |
|---------|---------|
| `/fpf:status` | Show current phase and next steps |
| `/fpf:query <topic>` | Search knowledge base |
| `/fpf:decay` | Check evidence freshness |

---

## The ADI Cycle

```
                        ┌────────────────────────────────────────┐
                        │            Problem Statement           │
                        └───────────────────┬────────────────────┘
                                            │
                                            ▼
         ┌──────────────────────────────────────────────────────────────┐
         │                    1. ABDUCTION (Hypothesize)                │
         │                    Generate competing hypotheses             │
         │                    Output: L0/ (unverified ideas)            │
         └───────────────────────────────┬──────────────────────────────┘
                                         │
                                         ▼
         ┌──────────────────────────────────────────────────────────────┐
         │                    2. DEDUCTION (Check)                      │
         │                    Verify logical consistency                │
         │                    Output: L1/ (logically sound)             │
         └───────────────────────────────┬──────────────────────────────┘
                                         │
                          ┌──────────────┴──────────────┐
                          ▼                             ▼
         ┌─────────────────────────┐     ┌─────────────────────────┐
         │   3a. INDUCTION (Test)  │     │ 3b. INDUCTION (Research)│
         │   Internal evidence     │     │   External evidence     │
         │   Run code, benchmarks  │     │   Web, docs, papers     │
         └────────────┬────────────┘     └────────────┬────────────┘
                      │                               │
                      └───────────────┬───────────────┘
                                      │
                                      ▼
         ┌──────────────────────────────────────────────────────────────┐
         │                    4. AUDIT (Optional but Recommended)       │
         │                    WLNK analysis, bias check, congruence     │
         │                    Output: Risk assessment                   │
         └───────────────────────────────┬──────────────────────────────┘
                                         │
                                         ▼
         ┌──────────────────────────────────────────────────────────────┐
         │                    5. DECIDE                                 │
         │                    Create DRR, archive session               │
         │                    Output: L2/ (verified), decisions/DRR     │
         └──────────────────────────────────────────────────────────────┘
```

---

## Assurance Levels

| Level | Name | Meaning | How to Reach |
|-------|------|---------|--------------|
| **L0** | Observation | Unverified hypothesis or note | `/fpf:1-hypothesize` |
| **L1** | Reasoned | Passed logical consistency check | `/fpf:2-check` |
| **L2** | Verified | Empirically tested and confirmed | `/fpf:3-test` or `/fpf:3-research` |
| **Invalid** | Disproved | Was wrong — kept for learning | Failed at any stage |

### The WLNK Principle (Weakest Link)

**Critical:** Assurance level = minimum of all supporting evidence, NEVER average.

```
Evidence A: L2 (internal benchmark)
Evidence B: L2 (official docs)  
Evidence C: L1 (blog post, low congruence)

Combined Assurance: L1 (limited by weakest)
```

If you have 10 strong sources and 1 weak source, your assurance is capped by the weak source.

---

## Key Concepts

### Evidence Types

| Type | Source | Congruence Needed |
|------|--------|-------------------|
| **Internal** | Your tests, benchmarks | No (direct) |
| **External** | Web, docs, papers | Yes (assess context match) |

### Congruence Levels (for External Evidence)

| Level | Meaning | Example |
|-------|---------|---------|
| **High** | Direct match to our context | Same tech, similar scale |
| **Medium** | Partial match | Same tech, different scale |
| **Low** | Weak match | Related tech, different context |

Low-congruence evidence is flagged in audit — use with caution.

### Evidence Validity

Evidence expires. Set `valid_until` dates:

| Evidence Type | Typical Validity |
|---------------|------------------|
| Benchmarks | 3-6 months |
| API tests | Until next version |
| External docs | 6-12 months |
| Blog posts | 1-2 years |

Use `/fpf:decay` to check freshness.

### Scope

Every hypothesis and evidence should have scope:

```yaml
scope:
  applies_to: "Read-heavy workload, <10k RPS"
  not_valid_for: "Write-heavy, real-time requirements"
```

This prevents misapplying knowledge outside its valid context.

---

## Directory Structure

```
.fpf/
├── knowledge/
│   ├── L0/              # Hypotheses, observations (unverified)
│   ├── L1/              # Logically verified (not empirically tested)
│   ├── L2/              # Empirically tested and confirmed
│   └── invalid/         # Disproved (kept for learning)
├── evidence/            # Test results, research findings
├── decisions/           # DRRs (Design Rationale Records)
├── sessions/            # Archived reasoning cycles
└── session.md           # Current cycle state
```

---

## Examples

### Example 1: Database Selection

**Problem:** "Should we use PostgreSQL or MongoDB for our new service?"

```bash
# Start the cycle
/fpf:1-hypothesize "Database selection for user profile service: PostgreSQL vs MongoDB"
```

**Generated hypotheses (L0/):**

- H1: PostgreSQL — relational model fits our structured data
- H2: MongoDB — flexibility for evolving schema
- H3: PostgreSQL + JSONB — hybrid approach

```bash
# Check logical consistency
/fpf:2-check
```

**Results:**

- H1: ✓ PASS → L1 (consistent with our data model)
- H2: ⚠ CONDITIONAL → L0 (need to verify query patterns)
- H3: ✓ PASS → L1 (addresses flexibility concern)

```bash
# Gather evidence
/fpf:3-research  # External: PostgreSQL vs MongoDB comparisons
/fpf:3-test      # Internal: Prototype with sample data
```

**Evidence created:**

- `evidence/2025-01-15-postgres-benchmark.md` (internal, L2)
- `evidence/2025-01-15-mongodb-research.md` (external, congruence: medium)

```bash
# Critical review
/fpf:4-audit
```

**Audit findings:**

- WLNK: MongoDB research has medium congruence (different scale)
- Gap: No testing of migration path
- Recommendation: PostgreSQL+JSONB has strongest evidence

```bash
# Finalize
/fpf:5-decide
```

**Output:** `decisions/DRR-001-database-selection.md`

---

### Example 2: Caching Strategy

**Problem:** "How should we implement caching for our API?"

```bash
/fpf:1-hypothesize "Caching strategy for /products API endpoint"
```

**Hypotheses:**

- H1: Redis with 5min TTL
- H2: In-memory cache (application level)
- H3: CDN edge caching

```bash
/fpf:2-check
```

H3 fails: Our data is user-specific, CDN won't help → moved to `invalid/`

```bash
/fpf:3-test --hypothesis h1
/fpf:3-test --hypothesis h2
```

**Results:**

- H1: 3ms p99, handles 10k RPS ✓
- H2: 1ms p99, but memory pressure at scale ⚠

```bash
/fpf:4-audit
```

**WLNK Analysis:**

- H1 has stronger evidence (tested at expected scale)
- H2 has scope limitation (not valid for >5k RPS)

```bash
/fpf:5-decide
```

**Decision:** Redis (H1) selected. DRR created with full rationale.

---

### Example 3: Quick Query (No Full Cycle)

Later, someone asks: "Why did we choose Redis?"

```bash
/fpf:query redis caching
```

**Output:**

```
## Knowledge Query: "redis caching"

### Verified (L2)
**redis-caching-decision** `.fpf/knowledge/L2/redis-caching.md`
> Redis selected for /products API caching.
> Evidence: benchmark evidence/2025-01-15-redis-benchmark.md
> Decided in: DRR-001

### Related Decisions
**DRR-001: Caching Strategy** `.fpf/decisions/DRR-001-caching.md`
> Selected Redis over in-memory due to scale requirements.
```

---

### Example 4: Evidence Decay Check

A few months later, check if evidence is still valid:

```bash
/fpf:decay
```

**Output:**

```
## Evidence Decay Report

**Scanned:** 12 evidence files
**Healthy:** 8 (valid, >30 days remaining)
**Warning:** 2 (expiring within 30 days)
**Expired:** 1 (past valid_until)
**No window:** 1 (validity not set)

### Action Required

**Immediate (expired):**
- [ ] `.fpf/evidence/redis-benchmark.md` — refresh (last run 6 months ago)

**Soon (expiring):**
- [ ] `.fpf/evidence/auth-flow-test.md` — plan refresh by 2025-02-01

### Knowledge Impact
2 L2 claims may need review if evidence not refreshed.
```

---

## Core Principles

### 1. Transformer Mandate

Claude Code generates options with evidence. **Human (THIS IS YOU) assesses intermediate steps and decides — highlight something, introduce additional context, or push to the next FPF phase right away.**
A system cannot transform itself — external decision-maker required.

### 2. Evidence Anchoring

Every decision traces back to evidence. No "trust me" allowed.

### 3. Falsifiability

Hypotheses must specify what would disprove them. Unfalsifiable claims are useless.

### 4. WLNK (Weakest Link)

Assurance = min(evidence), it is never average. One weak link spoils everything

### 5. Bounded Validity

Knowledge has scope and expiry. Context matters. Evidence decays.

### 6. Explicit Over Hidden

Assumptions, risks, and limitations are documented, not buried.

---

## Integration with Normal Workflow

FPF is **optional**. Your normal Claude Code workflow continues.

**Normal mode:** Inline Decision Framework for quick decisions

```
DECISION: [What]
OPTIONS: [A, B]
RECOMMENDATION: [Pick]
```

**FPF mode:** When you need persistent, auditable reasoning

```bash
/fpf:1-hypothesize "complex problem"
# ... full cycle ...
/fpf:5-decide
```

Switch between modes as needed. Use the right tool for the job.

---

## Troubleshooting

### "Too much overhead for this decision, I am waiting too long while FPF does all this reasoning!"

→ Don't use FPF. Just work as usual with Claude, or adopt inline Decision Framework from CLAUDE.md in this repository.

### "Hypotheses all look the same"

→ Force diversity: require conservative, innovative, and minimal options. The hypotheses look the same because your prompt for the first step was too narrow, with not much explanation and few inputs.

### "Evidence from different contexts"

→ Assess congruence. Low congruence = treat with caution, verify internally.

### "Old evidence, not sure if valid"

→ Run `/fpf:decay` command. Refresh, deprecate, or waive/delete with justification.

### "Audit found blockers"

→ Resolve before `/fpf:5-decide`. Blockers exist for a reason. This is the whole goal of FPF after all — to lead reasoning to consistent decisions!

### "Need to revisit old decision"

→ Run `/fpf:query`. Check DRR validity conditions. Start new cycle if needed.

#### Cheat For Passionate And Attentive Minds

If you find yourself in a situation where you feel that FPF is great but are still struggling to understand the approach and how to work with it... here is a cheat for you:

- Clone the original FPF spec repo.
- Init crucible-code IN it.
- Start with pure Claude Code commands AND with hypotheses about FPF itself, aiming to learn it.

Good luck

---

## License

All rights to FPF belong to Anatoly Levenchuk. This project inevitably inherits any licenses associated with FPF.
This project does not impose any additional proprietary licenses.
