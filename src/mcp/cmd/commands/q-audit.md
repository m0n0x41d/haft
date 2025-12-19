# q-audit: Assurance Visualization

## Intent
Visualizes the **Assurance Tree** for a holon, showing dependencies, **Reliability (R)** scores, and **Congruence Level (CL)** penalties.

## Action (Run-Time)
1.  Call `quint_audit_tree` with `holon_id` to get the visual dependency tree.
2.  Call `quint_calculate_r` with `holon_id` to get detailed R_eff breakdown.
3.  Present the results to the user.

## Tool Guide

### `quint_audit_tree`
Visualizes the assurance tree. Implements **Trust & Assurance Calculus (B.3)**.
-   **holon_id**: The root holon to audit.
-   *Returns:* ASCII tree with:
    -   R-scores (e.g., `[R: 0.85]`)
    -   Congruence Levels (e.g., `-- (CL: 2) -->`)
    -   Penalty warnings (e.g., `! Evidence expired`)

### `quint_calculate_r`
Computes R_eff with detailed breakdown.
-   **holon_id**: The holon to calculate.
-   *Returns:* Markdown report with R_eff, self score, weakest link, decay penalties.

## Example
User: `/q-audit system-auth`

Agent calls:
1.  `quint_audit_tree(holon_id: "system-auth")`
2.  `quint_calculate_r(holon_id: "system-auth")`
