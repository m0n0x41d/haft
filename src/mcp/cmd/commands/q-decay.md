# q-decay: Evidence Freshness Management

## Intent

Manages **evidence freshness** by identifying stale decisions and providing governance actions. Implements FPF B.3.4 (Evidence Decay).

**Key principle:** Evidence is perishable. Decisions built on expired evidence carry hidden risk.

---

## Quick Concepts

### What is "stale" evidence?

Every piece of evidence has a `valid_until` date. A benchmark from 6 months ago may no longer reflect current system performance. A security audit from before a major dependency update doesn't account for new vulnerabilities.

When evidence expires, the decision it supports becomes **questionable** — not necessarily wrong, just unverified.

### What is "waiving"?

**Waiving = "I know this evidence is stale, I accept the risk temporarily."**

Use it when:
- You're about to launch and don't have time to re-run all tests
- The evidence is only slightly expired and probably still valid
- You have a scheduled date to refresh it properly

A waiver is NOT ignoring the problem — it's **explicitly documenting** that you know about the risk and accept it until a specific date.

### The Three Actions

| Situation | Action | What it does |
|-----------|--------|--------------|
| Evidence is old but decision is still good | **Refresh** | Re-run the test, get fresh evidence |
| Decision is obsolete, needs rethinking | **Deprecate** | Downgrade hypothesis, restart evaluation |
| Accept risk temporarily | **Waive** | Record the risk acceptance with deadline |

---

## Natural Language Usage

**You don't need to memorize evidence IDs or tool parameters.** Just describe what you want.

### Example Workflow

```
User: /q-decay

Agent shows report:
## Evidence Freshness Report

### STALE (1 holon requires action)

#### Use Redis for Caching (L2)
| ID | Type | Status | Details |
|----|------|--------|---------|
| ev-benchmark-2024-06-15 | validation | EXPIRED | 180 days overdue |
| ev-load-test-2024-06-20 | validation | EXPIRED | 175 days overdue |

Actions:
  → Refresh: /q3-validate hypothesis-redis-caching
  → Deprecate: Tell me to deprecate this decision
  → Waive: Tell me which evidence to waive and until when
```

Now the user can respond naturally:

```
User: Waive the benchmark until February, we'll re-run it after the migration.
```

The agent reads the context, finds the matching evidence ID, and calls:
```
quint_check_decay(
    waive_id: "ev-benchmark-2024-06-15",
    waive_until: "2025-02-01",
    waive_rationale: "Will re-run after the migration"
)
```

**You don't need to copy IDs. Just reference what you mean.**

### More Natural Examples

```
User: /q-decay

[sees report with stale evidence]

User: We're launching next week. Waive everything until January 15th,
      we have a full test cycle scheduled for January 20th.

Agent: I'll waive both pieces of expired evidence until 2025-01-15
       with rationale "Launch deadline, full test cycle scheduled Jan 20"

       [calls quint_check_decay for each]

---

User: The vendor API is being discontinued. Deprecate that decision.

Agent: Deprecating hypothesis-vendor-api from L2 to L1.
       You'll need to run /q1-hypothesize to explore alternatives.

---

User: The load test is only 2 weeks overdue, I think it's still valid.
      Can we just refresh it?

Agent: Running /q3-validate to refresh the evidence for hypothesis-redis-caching.
       [calls quint_test]
```

---

## How Evidence IDs Work

Evidence IDs are generated automatically when tests run:
- Format: `ev-{type}-{date}` (e.g., `ev-benchmark-2024-06-15`)
- Shown in the freshness report
- **You don't need to remember them** — describe what you want, the agent finds the right ID

If you want to be explicit, you can use the full syntax:
```
User: /q-decay --waive ev-benchmark-2024-06-15 --until 2025-02-01 --rationale "Migration pending"
```

But natural language works fine:
```
User: Waive the benchmark until February, migration is pending.
```

---

## Detailed Actions

### Refresh (via /q3-validate)

**When:** Evidence is stale, but the decision is still correct. You just need fresh proof.

**What happens:**
1. Agent calls `quint_test` on the hypothesis
2. New evidence is recorded with a fresh `valid_until` date
3. Holon status changes from STALE to FRESH

**Example:**
```
User: The Redis benchmark is 6 months old but Redis is still the right choice.
      Can we refresh the evidence?

Agent: Running validation for hypothesis-redis-caching...
       [executes benchmark]
       Evidence refreshed. Hypothesis remains at L2 with fresh evidence valid until 2025-06-21.
```

### Deprecate

**When:** The decision itself is questionable. The world has changed, you need to reconsider.

**What happens:**
1. Hypothesis moves down one level (L2→L1 or L1→L0)
2. Audit log records who deprecated it and why
3. You're prompted to run `/q1-hypothesize` to explore alternatives

**Example:**
```
User: AWS announced they're deprecating this service. Deprecate our decision to use it.

Agent: Deprecated: hypothesis-aws-service L2 → L1

       This decision now requires re-evaluation.
       Next step: Run /q1-hypothesize to explore alternatives.
```

### Waive

**When:** You know the evidence is stale, you accept the risk temporarily.

**What happens:**
1. Waiver is recorded with: who, until when, why
2. Evidence no longer shows as STALE (shows as WAIVED)
3. When waiver expires, evidence returns to STALE status
4. Full audit trail preserved

**Example:**
```
User: We're launching Tuesday. Waive the security audit until the 15th,
      we have a re-audit scheduled for the 20th.

Agent: Waiver recorded:
       - Evidence: ev-security-audit-2024-03
       - Waived until: 2025-01-15
       - Rationale: Launch deadline. Re-audit scheduled for the 20th.

       ⚠️ This evidence returns to EXPIRED status after 2025-01-15.
```

---

## Tool Reference (for understanding, not memorization)

### `quint_check_decay`

The agent translates your natural language into these parameters:

| Parameter | What it means |
|-----------|--------------|
| (none) | Show the freshness report |
| `deprecate` | Which hypothesis to downgrade |
| `waive_id` | Which evidence to waive |
| `waive_until` | When the waiver expires (YYYY-MM-DD) |
| `waive_rationale` | Why you're accepting this risk |

---

## WLNK Principle

A holon is **STALE** if *any* of its evidence is expired (and not waived).

This is the Weakest Link (WLNK) principle: reliability = min(all evidence). One stale piece makes the whole decision questionable.

---

## Audit Trail

All actions are logged for accountability:

| Action | What's Recorded |
|--------|-----------------|
| Deprecate | from_layer, to_layer, who, when |
| Waive | evidence_id, until_date, rationale, who, when |

Waivers are stored in a dedicated table — you can query "who waived what and why" at any time.

---

## Common Workflows

### Weekly Maintenance
```
/q-decay                    # See what's stale
# For each stale item, tell the agent: refresh, deprecate, or waive
```

### Pre-Release
```
/q-decay                    # Check for stale decisions
# Either refresh evidence or explicitly waive with documented rationale
# Waiver rationales become part of release documentation
```

### After Major Change
```
# Dependency update, API change, security advisory...
/q-decay                    # See what's affected
# Deprecate obsolete decisions
# Start new hypothesis cycle for replacements
```
