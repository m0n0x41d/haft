---
description: "Reset the FPF cycle"
pre: "none"
post: "context abandoned or session ended, audit log entry created"
invariant: "no DRR created - this is operational, not a decision"
required_tools: ["quint_reset"]
---

# Reset Cycle

You are the **Observer** ending the current reasoning session or abandoning a decision context.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| reason | No | Why the reset is happening |
| context_id | No* | Specific context to abandon |
| abandon_all | No* | Abandon all open contexts (requires confirmation) |

*One of `context_id` or `abandon_all` can be specified. If neither, session ends without abandoning contexts.

## When to Use

| Situation | Use Reset? | Parameters |
|-----------|-----------|------------|
| Session complete, no decision made | Yes | `reason` only |
| Pivoting to different problem | Yes | `context_id` to abandon old context |
| Starting completely fresh | Yes | `abandon_all=true` |
| Decision made and recorded | No - use /q5-decide | |

## Action

### End session (default)

```
quint_reset(reason="Session complete")
```

### Abandon specific context

```
quint_reset(context_id="dc-caching-strategy", reason="Pivoting to authentication")
```

### Abandon all contexts

```
quint_reset(abandon_all=true, reason="Starting fresh")
```

## What Happens

### Default (no context parameters)
1. Current stage recorded to audit log
2. Knowledge state (L0/L1/L2/DRR counts) captured
3. Open decisions noted (they remain open)
4. Session ends

### With context_id
1. Context marked as `abandoned` in database
2. Hypotheses in context remain (not deleted)
3. Context no longer appears in active contexts
4. Audit log entry created

### With abandon_all
1. All open contexts marked as `abandoned`
2. All hypotheses remain (not deleted)
3. Active context count goes to 0
4. Audit log entry created

## What Does NOT Happen

- No DRR created (reset is not a decision)
- No hypotheses deleted (knowledge preserved)
- No evidence modified

## After Reset

The knowledge base remains intact. Next session can:
- Continue with remaining contexts: `/q-internalize`
- Start fresh exploration: `/q1-hypothesize`
