# FPF Patterns: Cross-Cutting (All Phases)

## X-WLNK: Weakest Link Principle
**Trigger:** Aggregating components; building assurance
**Spec:** B.3.4.4, B.1.1

Never average. Take the minimum. If five components have Rs = [0.9, 0.9, 0.9, 0.9, 0.3], system R_eff is capped at 0.3. The weak link dominates. Invest effort in the weakest component first.

## X-MONO: Monotonicity (Improvements Are Safe)
**Trigger:** Making an improvement; confirming it can't break something
**Spec:** B.3.4.1, B.3.4.4, B.1.1

Monotone characteristics: F (formality), R (reliability when all else fixed), G (claim scope when adding supported regions). Raising these always improves or maintains assurance; never degrades. Non-monotone properties (complexity, cost) tracked separately.

## X-IDEM: Idempotence
**Trigger:** Re-running a process; checking for side effects
**Spec:** B.1.1, A.4

Idempotent decisions safe to repeat: applying once or twice yields same state. Prefer idempotent decisions in distributed/error-prone systems. Re-checking a claim doesn't invalidate the prior check.

## X-SCOPE: Scope Discipline
**Trigger:** Scope creeping or undefined
**Spec:** A.2.3, A.1.1, B.3.4.2

Every claim has explicit scope: where it holds (boundaries), under what conditions (context K), at what time (window). "This algorithm is always fast" = scope inflation. Prevents false universality.

## X-DESIGNRUN: Design-Time vs Run-Time
**Trigger:** Mixing specification statements with behavior observations
**Spec:** A.4, B.4, B.3.4.2

Design-time: specification, formal model, intended behavior. Run-time: deployed system, observed behavior. Assurance tuples reported SEPARATELY. Never compose one score from design-time verification and run-time validation as if measuring the same thing.

## X-TRANSFORMER: External Agent Mandate
**Trigger:** System claims self-improvement; need accountability
**Spec:** A.3, A.12, B.4, CC-B4.3

A Transformer is the external agent that Observes, Refines, Deploys. A holon does not evolve itself. Essential for accountability. Prevents confusion between capability (what it can do) and autonomy (whether it decides to change itself). The human remains the decision-maker.

## X-TERM-QUALITY: Term Evolution Quality Model
**Trigger:** Inventing or refining domain terminology
**Source:** Haft operational pattern (derived from semiotics practice)

Score candidate names on four dimensions: (1) SemanticFidelity = precision, no over-promise; (2) CognitiveErgonomics = readability, brevity, memorability; (3) OperationalAffordance = usability in instructions/forms/tests; (4) AliasRisk = collision/confusion with existing terms. Keep both technical name (high fidelity) and plain name (high ergonomics).

## X-GLOSSARY: Glossary as Operational Infrastructure
**Trigger:** Multiple roles or sessions share terms; silent drift risk
**Source:** Haft operational pattern (derived from semiotics practice)

Live term bank: surface forms, normalized terms, aliases, canonical ID, local definitions, relations, validity windows, change history. Term changes are decision artifacts, not undocumented renames. Attach checks to CI pipelines.

## X-BITTER-LESSON: Scaling-Law Lens
**Trigger:** Choosing between hand-tuned method vs general method with more budget
**Source:** Haft operational pattern

Prefer method with better scaling slopes (sensitivity to data/compute/freedom) over method with better current performance if budgets are comparable. Use scale-audit at 2+ points to detect elasticity class. General methods + more budget usually win over specialized heuristics.
