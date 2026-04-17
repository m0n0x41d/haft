# FPF Patterns: Cross-Cutting (All Phases)

## X-WLNK: Weakest Link Principle
**Trigger:** Aggregating components; building assurance
**Spec:** B.3.4.4, B.1.1
**Core:** verify

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
**Source:** Levenchuk FPF F.18 Naming Protocol (slideument slide 5), adapted for haft

Score candidate names on four dimensions: (1) SemanticFidelity = precision, no over-promise; (2) CognitiveErgonomics = readability, brevity, memorability; (3) OperationalAffordance = usability in instructions/forms/tests; (4) AliasRisk = collision/confusion with existing terms. Keep both technical name (high fidelity) and plain name (high ergonomics). See CHR-11 for the full pipeline that operationalizes this.

## X-GLOSSARY: Glossary as Operational Infrastructure
**Trigger:** Multiple roles or sessions share terms; silent drift risk
**Source:** Levenchuk FPF E.10 LEX-BUNDLE + F.13 Lexical Continuity (semiotics slideument slide 16), adapted for haft

Live term bank: surface forms, normalized terms, aliases, canonical ID, local definitions, relations, validity windows, change history. Term changes are decision artifacts, not undocumented renames. Attach checks to CI pipelines.

## X-BITTER-LESSON: Scaling-Law Lens
**Trigger:** Choosing between hand-tuned method vs general method with more budget
**Source:** Levenchuk FPF seminar (slideument slides 59-60 — SLL + BLP + NQD), adapted for haft

Prefer method with better scaling slopes (sensitivity to data/compute/freedom) over method with better current performance if budgets are comparable. Use scale-audit at 2+ points to detect elasticity class. General methods + more budget usually win over specialized heuristics.

## X-STATEMENT-TYPE: Classify Every Load-Bearing Sentence
**Trigger:** Text carrying project weight (requirement, spec, decision, explanation, incident report, PR description, chat message used as source-of-truth)
**Source:** Levenchuk semiotics slideument slides 10, 20 — "document carries rule + promise + report + evidence simultaneously; unmixed statement types required for audit and handoff"

Every load-bearing sentence classifies as exactly one of:
- **Rule** — a law, definition, or norm that holds by decree. Defines what something is or must be.
- **Promise** — a commitment by a specific agent to a specific counterparty with scope and horizon. Carries authority.
- **Explanation** — descriptive claim about how something works. No obligation, no decision rights.
- **Gate** — admissibility condition that must hold before proceeding. Binary pass/fail.
- **Evidence** — observable carrier (log, measurement, run, artifact hash) supporting some other claim.

Mixed statements are L1 semiotic errors. Typical failures:
- "The service SHOULD X" — is this a rule, a gate, a promise, or an explanation? Different owners, different consequences.
- Explanation doc being used to accept deployment (explanation → gate shift is invalid without re-authoring)
- Rule written as evidence ("telemetry shows p95 < 200ms" ≠ "p95 < 200ms is the SLO")

When writing load-bearing text, tag each sentence. When reading, refuse to act on mixed sentences until they are decomposed (see CHR-10 for boundary-specific decomposition via L/A/D/E). Do not let one sentence silently hold multiple types — that is where accountability drops and semantic regression hides.

## X-FANOUT-AUDIT: Concept Rename Must Sweep All Carriers
**Trigger:** Renaming or redefining a project concept — deprecating "waves" for "campaigns", replacing "service" with "workstream", consolidating kinds
**Source:** Levenchuk semiotics slideument slide 5 — rework case where waves appeared in Portfolio, Architectural pressure, Deferred waves, Backlog, filenames, manifests; >10 review round-trips

A concept does not live in one place. When renaming or unifying, audit all its carriers before declaring done:
- **Prose carriers** — main documents, comments, explanations
- **Filenames and path segments** — often stale refs hide here
- **Manifests and registries** — yaml/json/config files cite concept names
- **Review bundles and handoff surfaces** — reviewer-facing derivatives lag behind source
- **Provenance and archive surfaces** — historical docs may still be routable-by-use even when formally "not current owner"
- **Test cases and fixtures** — names embedded in test data
- **Schema and type definitions** — code-level references
- **Downstream dependents** — other projects importing the concept

Typical failure: localized fix misses reviewer-facing or archive surfaces → concept resurrects via routing-by-use → rework cascades. Run a fixed-point pass: rename, scan all carrier classes, rename again, repeat until no occurrences remain. Declare done only after one clean sweep with zero hits. Combine with F.13 Lexical Continuity for graceful deprecation of the old name.
