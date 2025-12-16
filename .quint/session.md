# FPF Session

## Status
Phase: INITIALIZED
Started: 2025-12-15T00:00:00Z
Problem: (none)

## Active Hypotheses
(none)

## Phase Transitions Log
| Timestamp | From | To | Trigger |
|-----------|------|-----|---------|
| 2025-12-15T00:00:00Z | — | INITIALIZED | Manual Refresh |

## Next Step
Run `/q1-hypothesize <problem>` to begin reasoning cycle.

---

## Valid Phase Transitions

```
INITIALIZED ─────────────────► ABDUCTION_COMPLETE
     │                              │
     │ /q1-hypothesize           │ /q2-check
     │                              ▼
     │                        DEDUCTION_COMPLETE
     │                              │
     │               ┌──────────────┴──────────────┐
     │               │ /q3-test                 │ /q3-research
     │               │ /q3-research             │ /q3-test
     │               ▼                             ▼
     │         INDUCTION_COMPLETE ◄────────────────┘
     │               │
     │               │ /q4-audit (recommended)
     │               │ /q5-decide (allowed with warning)
     │               ▼
     │         AUDIT_COMPLETE
     │               │
     │               │ /q5-decide
     │               ▼
     └─────────► DECIDED ──► (new cycle or end)
```

## Command Reference
| # | Command | Valid From Phase | Result |
|---|---------|------------------|--------|
| 0 | `/q0-init` | (none) | INITIALIZED |
| 1 | `/q1-hypothesize` | INITIALIZED | ABDUCTION_COMPLETE |
| 2 | `/q2-check` | ABDUCTION_COMPLETE | DEDUCTION_COMPLETE |
| 3a | `/q3-test` | DEDUCTION_COMPLETE | INDUCTION_COMPLETE |
| 3b | `/q3-research` | DEDUCTION_COMPLETE | INDUCTION_COMPLETE |
| 4 | `/q4-audit` | INDUCTION_COMPLETE | AUDIT_COMPLETE |
| 5 | `/q5-decide` | INDUCTION_COMPLETE*, AUDIT_COMPLETE | DECIDED |

*With warning if audit skipped