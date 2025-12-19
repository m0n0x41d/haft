# q-decay: Assurance Maintenance

## Intent
Identifies **Epistemic Debt (ED)** by finding holons with expired evidence that need re-validation.

## Action (Run-Time)
1.  Call `quint_check_decay` to get a report of all holons with expired evidence.
2.  Review the output - it shows:
    -   Which holons have stale evidence
    -   How many evidence items are expired
    -   How many days overdue
3.  Present findings to the user with recommendations.

## Tool Guide

### `quint_check_decay`
Scans all evidence and identifies expired items. Implements **Evidence Decay (B.3.4)**.
-   *No parameters required.*
-   *Returns:* Markdown report listing:
    -   Holons with expired evidence
    -   Count of expired evidence per holon
    -   Days overdue
    -   Recommendation to run `/q3-validate` for affected holons

## Example
User: `/q-decay`

Agent calls:
1.  `quint_check_decay()`

If expired evidence is found, recommend:
-   Run `/q3-validate <hypothesis_id>` to refresh evidence
-   Or run `/q4-audit <hypothesis_id>` to reassess reliability
