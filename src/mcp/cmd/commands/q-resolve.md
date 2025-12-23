---
description: "Record decision outcome"
pre: "DRR exists in knowledge base"
post: "resolution evidence recorded"
invariant: "bridges plan (DRR) to reality (implementation)"
required_tools: ["quint_resolve"]
---

# Resolve Decision

You are the **Observer** closing the decision lifecycle. This command records what happened to a decision after it was made.

## Purpose

Decisions are plans. Reality is what happens. `quint_resolve` bridges the gap by recording:

1. **Implemented** - The decision was executed (with reference to where)
2. **Abandoned** - The decision was dropped (with explanation why)
3. **Superseded** - A newer decision replaced this one (with link to replacement)

## When to Use

| Situation | Resolution | Reference |
|-----------|------------|-----------|
| Feature shipped | `implemented` | `commit:SHA`, `pr:NUM`, or `file:PATH` |
| Requirements changed | `abandoned` | Explanation in `notes` |
| Better approach found | `superseded` | New decision ID in `superseded_by` |

## Action

Call `quint_resolve` with the decision ID and resolution type:

```
quint_resolve(
    decision_id="DRR-auth-jwt-tokens",
    resolution="implemented",
    reference="commit:abc1234"
)
```

## Required Parameters by Resolution Type

| Resolution | Required | Optional |
|------------|----------|----------|
| `implemented` | `reference` (commit, PR, or file) | `valid_until` |
| `abandoned` | `notes` (explanation) | - |
| `superseded` | `superseded_by` (new DRR ID) | `notes` |

## Examples

### Implementation Complete

```
> quint_resolve(
    decision_id="DRR-caching-redis",
    resolution="implemented",
    reference="pr:42"
)

Resolution recorded:
  Decision: DRR-caching-redis
  Status: implemented
  Reference: pr:42
  Evidence: implementation-DRR-caching-redis-20240115
```

### Decision Abandoned

```
> quint_resolve(
    decision_id="DRR-auth-oauth2",
    resolution="abandoned",
    notes="Requirements changed: now using internal SSO"
)

Resolution recorded:
  Decision: DRR-auth-oauth2
  Status: abandoned
  Reason: Requirements changed: now using internal SSO
  Evidence: abandonment-DRR-auth-oauth2-20240115
```

### Decision Superseded

```
> quint_resolve(
    decision_id="DRR-cache-inmemory",
    resolution="superseded",
    superseded_by="DRR-cache-redis",
    notes="Scale requirements exceeded in-memory capacity"
)

Resolution recorded:
  Decision: DRR-cache-inmemory
  Status: superseded
  Replaced by: DRR-cache-redis
  Evidence: supersession-DRR-cache-inmemory-20240115
```

## Finding Open Decisions

Use `quint_internalize` to see decisions awaiting resolution:

```
=== QUINT INTERNALIZE ===
...
Open Decisions (awaiting resolution):
  - DRR-auth-jwt: JWT Authentication (2d ago)
  - DRR-cache-redis: Redis Caching (1w ago)
```

Or search explicitly:

```
quint_search(query="*", status_filter="open")
```

## Decision States

```
                    +-------------+
                    |    DRR      |
                    |  (created)  |
                    +------+------+
                           |
          +----------------+----------------+
          |                |                |
          v                v                v
   +------+------+  +------+------+  +------+------+
   | implemented |  |  abandoned  |  | superseded  |
   +-------------+  +-------------+  +------+------+
                                            |
                                            v
                                     +------+------+
                                     |  newer DRR  |
                                     +-------------+
```

## Why Resolve Matters

1. **Closes the Loop**: Without resolution, decisions are dangling intentions
2. **Searchable History**: Find what was implemented, what was dropped, and why
3. **Evidence Trail**: Each resolution creates evidence linked to the DRR
4. **Validity Tracking**: `valid_until` enables periodic re-verification

## Checkpoint

Before resolving a decision:
- [ ] Verified the decision ID exists
- [ ] Confirmed the resolution type matches reality
- [ ] Have the required parameter (reference/notes/superseded_by)
- [ ] If superseding, the new DRR exists
