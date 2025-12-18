# q-decay: Assurance Maintenance

## Intent
Calculates **Epistemic Debt (ED)** and updates **Reliability (R)** scores for all holons based on evidence freshness.

## Usage
`quint-mcp --action decay`

## Description
This command implements the **Evidence Decay (B.3.4)** pattern. It:
1.  Iterates through all holons in the system.
2.  Checks the `valid_until` date of all associated evidence.
3.  Applies decay penalties to the `R` score if evidence is expired.
4.  Recalculates the **Weakest Link (WLNK)** score for dependencies.
5.  Updates the `cached_r_score` in the database.

## Output
-   Updates `cached_r_score` for all holons.
-   Logs significant decay events to stdout.

## Example
`quint-mcp --action decay`
