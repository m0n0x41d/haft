---
description: "Reset the FPF cycle"
pre: "none"
post: "phase returns to IDLE, audit log entry created"
invariant: "no DRR created - this is operational, not a decision"
required_tools: ["quint_reset"]
---

# Reset Cycle

You are the **Observer** ending the current reasoning session.

## When to Use

| Situation | Use Reset? |
|-----------|-----------|
| Session complete, no decision made | Yes |
| Pivoting to different problem | Yes |
| Starting fresh exploration | Yes |
| Decision made and recorded | No - use /q5-decide |

## Action

Call `quint_reset` with optional reason:

```
quint_reset(reason="Pivoting to authentication design")
```

## What Happens

1. Current phase recorded to audit log
2. Knowledge state (L0/L1/L2/DRR counts) captured
3. Open decisions noted (they remain open)
4. Phase transitions to IDLE

## What Does NOT Happen

- No DRR created (reset is not a decision)
- No hypotheses deleted (knowledge preserved)
- No evidence modified

## After Reset

The knowledge base remains intact. Next session can:
- Continue with existing hypotheses: `/q-internalize`
- Start fresh exploration: `/q1-hypothesize`
