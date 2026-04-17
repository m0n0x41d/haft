# FPF Patterns: Frame Phase

## FRAME-01: Signal Explicitness
**Trigger:** Starting a new problem; something is broken, unclear, or anomalous
**Spec:** B.4.1, B.5.2.0

Make the triggering signal explicit and typed. Is it an anomaly (failure, mismatch)? An opportunity (promising opening)? A probe cue (question needing testing)? A pre-articulation cue (weak signal not yet shaped)? Typed signals prevent drift into vague problem-ness and let later stages know what they're receiving.

## FRAME-02: Scope and Boundary
**Trigger:** Scope confusion or scope creep; unclear what's being addressed
**Spec:** A.1.1, A.2.3

Explicitly declare the scope: what holon or problem region is being addressed, under what context assumptions, at what temporal/spatial boundary. Equally important: name what is explicitly OUT-of-scope. This prevents silent scope inflation.

## FRAME-03: Acceptance Criteria
**Trigger:** Problem is clear but "done" is undefined
**Spec:** B.5.1, A.2.3, C.11

Declare acceptance criteria early: what observable condition or measurement signals the problem is resolved? Criteria must be measurable (or at least decidable) and aligned to the declared problem scope. This bridges framing to evidence-gathering.

## FRAME-04: Assumption Surfacing
**Trigger:** Disagreement without clear source; decisions fail because assumptions weren't shared
**Spec:** B.3, B.3.4.2, C.2.5

State context assumptions (K): what is taken as given (environment, prior behavior, operating regime)? What would falsify one assumption? Making these explicit prevents downstream reasoning from collapsing when reality doesn't match the silent assumption.

## FRAME-05: Problem Typing
**Trigger:** Before exploring, classify the problem to select the right exploration strategy
**Spec:** B.5.2.0

Classify: Optimization (working system, want better on a known dimension), Diagnosis (something's broken, don't know why), Search (need to find something that doesn't exist yet), Synthesis (combine existing elements into something new). Each type suggests different exploration strategies.

## FRAME-06: Three-Factory Awareness
**Trigger:** When structuring work across teams or delegation levels
**Source:** Haft operational pattern (derived from engineering management practice)

Work flows through three interlinked factories: Problem Factory (problematization), Solution Factory (variant generation + selection), Factory of Factories (org development). Each has identical structure (characterize > indicatorize > compare > select) but operates at different scales. Use when planning autonomous delegation.

## FRAME-07: Goldilocks Problem Selection
**Trigger:** Filtering backlog; ensuring continuous growth without burnout
**Source:** Haft operational pattern

Select problems that are 10-20% beyond current capability (not impossible, not trivial). Use measurability, blast-radius reversibility, and stepping-stone potential as tie-breakers. Regular ritual to re-evaluate portfolio between "too hard", "just right", and "too easy" zones.
