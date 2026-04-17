# FPF Patterns: Decide Phase

## DEC-01: Decision Record Structure
**Trigger:** Option chosen; preserving reasoning for review
**Spec:** E.9, B.4, A.12
**Core:** true

Four components: (1) Problem frame = why needed, (2) Decision = what chosen, (3) Rationale = why this over others (filters, evidence, policy), (4) Consequences = what changes, risks, enables. Traceable to code/spec.

## DEC-02: Pre-Conditions
**Trigger:** Ensuring prerequisites before implementation
**Spec:** C.11.4.2.5, B.3.5.1

Hard constraints that must hold before implementation: "Service X supports Y API", "Budget approved", "Training complete". If a pre-condition fails, decision may need reopening.

## DEC-03: Post-Conditions
**Trigger:** Defining "success" or "completion"
**Spec:** B.4, A.3

Testable outcomes after implementation: "Service migrated", "Codebase uses new library", "Team knows protocol". Post-conditions feed into Evidence phase.

## DEC-04: Invariants
**Trigger:** Non-negotiable properties (safety, compliance, data integrity)
**Spec:** B.1.1, B.3.4.4, A.15
**Core:** true

Load-bearing constraints that hold before, during, and after implementation: "Encryption always on", "No silent failures", "Backward compatibility". Invariant violation = grounds for rollback.

## DEC-05: Rollback Plan
**Trigger:** Insurance against being stuck with bad choice
**Spec:** B.4, C.11, E.9
**Core:** true

Answers: (1) Triggers = what evidence indicates the decision was wrong? (2) Steps = how to revert? (3) Blast radius = what breaks on rollback? (4) Timeline = how long is rollback feasible? Document reversibility honestly.

## DEC-06: Predictions (Testable Claims)
**Trigger:** Setting up test regime for Evidence phase
**Spec:** B.5, C.11.4.2.4, B.3.5.1
**Core:** true

Each decision implies predictions: "If we choose X, we see Y within Z under conditions K." Falsifiable and measurable. These become measurement targets for verification.

## DEC-07: Why Not Others
**Trigger:** Recording rejection reasons for alternatives
**Spec:** C.11.4.2.3, B.5.2

For each rejected alternative: key strength, key weakness, why it lost (which criterion failed), under what evidence it would be reconsidered. Prevents resurrection without explicit reason.

## DEC-08: Counterargument Preservation
**Trigger:** Decision is polished; strongest objection must be visible
**Spec:** A.12, B.5.2
**Core:** true

Persist the strongest argument AGAINST the chosen option. Not a strawman — a genuine attack. The agent that generates a decision cannot be its sole validator (External Transformer Principle). Counterargument is a self-deception check.

## DEC-09: Reversibility as Design Constraint
**Trigger:** Implementing a decision; structural principle for fast iteration
**Source:** Haft operational pattern

Default to reversible changes: canary rollout, small diffs, explicit rollback plan, error budgets, blast-radius limits. Reversibility lets you shift between goals without catastrophic sunk cost. Make rollback as automated as deployment.
