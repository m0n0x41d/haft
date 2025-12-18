# q-audit: Assurance Visualization

## Intent
Visualizes the **Assurance Tree** for a holon, showing dependencies, **Reliability (R)** scores, and **Congruence Level (CL)** penalties.

## Usage
`quint-mcp --action audit --target_id <holon_id>`

## Description
This command implements the **Trust & Assurance Calculus (B.3)** visualization. It:
1.  Recursively traverses the dependency graph starting from `<holon_id>`.
2.  Calculates the `R_score` for each node using the **Weakest Link** principle.
3.  Displays the tree with:
    -   R-scores (e.g., `[R: 0.85]`)
    -   Congruence Levels (e.g., `-- (CL: 2) -->`)
    -   Penalty warnings (e.g., `! Evidence expired`)

## Output
An ASCII tree representation of the holon's assurance status.

## Example
`quint-mcp --action audit --target_id system-auth`
