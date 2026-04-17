# FPF Patterns: Compare Phase

## CMP-01: Parity Enforcement
**Trigger:** Comparing 2+ options; avoiding bias
**Spec:** C.11.4.2.3
**Core:** true

Parity rules state what must be equal across variants: same budget, same assumptions, same scope, same evidence standards. If one variant is evaluated under optimistic assumptions and another under pessimistic ones, the comparison is rigged. Document the parity baseline explicitly.

## CMP-02: Selection Policy (Declared Up Front)
**Trigger:** Multiple viable options; preventing cherry-picking
**Spec:** C.11.4.2.3
**Core:** true

Declare the selection policy BEFORE scoring: first to meet all constraints? Best on primary dimension? Least worst on critical risk? Threshold-based? Declare the rule, then apply mechanically. Separates subjective judgment (what matters?) from objective evaluation (does it satisfy the rule?).

## CMP-03: Pareto Front Identification
**Trigger:** Multiple tradeoffs between options
**Spec:** C.11.4.2.3
**Core:** true

An option is dominated if another is better or equal on ALL dimensions. Pareto-front options cannot be improved on one dimension without worsening another. Identify the non-dominated set explicitly. Dominated variants eliminated with rationale. Exposes real tradeoffs.

## CMP-04: Dominated Variant Notes
**Trigger:** Variant X clearly worse than Y; recording the decision
**Spec:** C.11.4.2.3
**Core:** true

Make a dominated-variant note: which variant dominated it, on which dimensions, by how much. Prevents later resurrection of bad ideas. Example: "V2 dominated V1 on cost (10% cheaper), speed (20% faster), and reliability (CL2 vs CL1). V1 had no compensating advantage."

## CMP-05: Incomparable Pairs
**Trigger:** Two variants good on different dimensions; no common metric
**Spec:** C.11.4.2.1a

Some pairs cannot be ranked under any single criterion without imposing external weight. Mark explicitly as incomparable. Document why. This prevents forcing a false total order.

## CMP-06: Evidence Congruence Across Options
**Trigger:** Evidence quality varies across variants
**Spec:** B.3.4.1, B.3.4.4
**Core:** true

Mark Congruence Level of evidence relative to each option: CL3 (exact context), CL2 (similar), CL1 (related), CL0 (opposed). Low CL = lower trust even if raw evidence is strong. Prevents overconfidence from out-of-context testing.

## CMP-07: Entity/Numeric Preservation Watchdog
**Trigger:** After summarization or comparison reports
**Source:** Levenchuk semiotics slideument (slide 10 — "в handoff уходит смысл"), adapted for haft

Extract and compare: named entities, numbers, units, IDs, negation markers. If a summary omits a number or conflates entities, flag it. Use QA/NLI as second layer. Structured IE diff on engineering facts. Prevents silent loss of critical detail in comparison reports.
