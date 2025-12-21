---
description: "Search knowledge base"
required_tools: ["quint_calculate_r", "quint_audit_tree"]
---

# Query Knowledge

Search the FPF knowledge base and display holon details with assurance information.

## Action (Run-Time)

1. **Search** `.quint/knowledge` and `.quint/decisions` by user query.
2. **For each found holon**, display:
   - Basic info: title, layer (L0/L1/L2), kind, scope
   - If layer >= L1: call `quint_calculate_r` → show R_eff
   - If has dependencies: call `quint_audit_tree` → show dependency graph
   - Evidence summary if exists
3. **Present results** in table format.

## Output Format

```
## Search Results for "<query>"

| Holon | Layer | Kind | R_eff |
|-------|-------|------|-------|
| redis-caching | L2 | system | 0.85 |
| cdn-edge | L2 | system | 0.72 |

### redis-caching
[redis-caching R:0.85] Use Redis for Caching
  --(CL:3)-->
    [perf-test R:0.90] Performance Test Evidence

R_eff Breakdown:
- Self Score: 1.00
- Weakest Link: perf-test (0.90)
- Final: 0.85
```

## Tool Guide

### `quint_calculate_r`
Computes R_eff with detailed breakdown.
- **holon_id**: The holon to calculate.
- *Returns:* R_eff score, self score, weakest link, decay penalties.

### `quint_audit_tree`
Visualizes the assurance tree.
- **holon_id**: The root holon to audit.
- *Returns:* ASCII tree with R-scores, CL levels, and penalty warnings.

## Examples

**Search by keyword:**
```
/q-query caching
→ Finds all holons matching "caching"
→ Shows R_eff for each L1+ holon
```

**Query specific holon:**
```
/q-query redis-caching
→ Shows full details for redis-caching
→ Displays dependency tree
→ Shows R_eff breakdown
```

**Query decisions:**
```
/q-query DRR
→ Lists all Design Rationale Records
→ Shows what each DRR selected/rejected
```
