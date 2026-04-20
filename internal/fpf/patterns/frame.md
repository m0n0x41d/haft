# FPF Patterns: Frame Phase

**Valid-until:** 2026-10-18

When this date passes, review the patterns below: are they still attributed correctly, do hint Core markers still match agent behavior, are new framing patterns from current FPF/slideument work missing? Refresh + bump this date. Per VER-08 self-applied to the pattern set.

## FRAME-01: Signal Explicitness
**Trigger:** Starting a new problem; something is broken, unclear, or anomalous
**Spec:** B.4.1, B.5.2.0
**Core:** true

Make the triggering signal explicit and typed. Is it an anomaly (failure, mismatch)? An opportunity (promising opening)? A probe cue (question needing testing)? A pre-articulation cue (weak signal not yet shaped)? Typed signals prevent drift into vague problem-ness and let later stages know what they're receiving.

## FRAME-02: Scope and Boundary
**Trigger:** Scope confusion or scope creep; unclear what's being addressed
**Spec:** A.1.1, A.2.3
**Core:** true

Explicitly declare the scope: what holon or problem region is being addressed, under what context assumptions, at what temporal/spatial boundary. Equally important: name what is explicitly OUT-of-scope. This prevents silent scope inflation.

## FRAME-03: Acceptance Criteria
**Trigger:** Problem is clear but "done" is undefined
**Spec:** B.5.1, A.2.3, C.11
**Core:** true

Declare acceptance criteria early: what observable condition or measurement signals the problem is resolved? Criteria must be measurable (or at least decidable) and aligned to the declared problem scope. This bridges framing to evidence-gathering.

## FRAME-04: Assumption Surfacing
**Trigger:** Disagreement without clear source; decisions fail because assumptions weren't shared
**Spec:** B.3, B.3.4.2, C.2.5

State context assumptions (K): what is taken as given (environment, prior behavior, operating regime)? What would falsify one assumption? Making these explicit prevents downstream reasoning from collapsing when reality doesn't match the silent assumption.

## FRAME-05: Problem Typing
**Trigger:** Before exploring, classify the problem to select the right exploration strategy
**Spec:** B.5.2.0
**Core:** true

Classify: Optimization (working system, want better on a known dimension), Diagnosis (something's broken, don't know why), Search (need to find something that doesn't exist yet), Synthesis (combine existing elements into something new). Each type suggests different exploration strategies.

## FRAME-06: Three-Factory Awareness
**Trigger:** When structuring work across teams or delegation levels
**Source:** Levenchuk FPF seminar (slideument slides 1/4/8 — problem factory, solution factory, factory of factories), adapted for haft

Work flows through three interlinked factories: Problem Factory (problematization), Solution Factory (variant generation + selection), Factory of Factories (org development). Each has identical structure (characterize > indicatorize > compare > select) but operates at different scales. Use when planning autonomous delegation.

## FRAME-07: Goldilocks Problem Selection
**Trigger:** Filtering backlog; ensuring continuous growth without burnout
**Source:** Levenchuk FPF seminar (slideument slide 7, zone of proximal development), adapted for haft

Select problems in the zone of proximal development — beyond current capability but reachable with effort (not impossible, not trivial). Use measurability, blast-radius reversibility, and stepping-stone potential as tie-breakers. Regular ritual to re-evaluate the portfolio between "too hard", "just right", and "too easy" zones.

## FRAME-08: Reading Checklist (Pre-Reasoning Hygiene)
**Trigger:** Before processing ANY incoming artifact — note, prompt, dashboard, requirements doc, explanation, chat message
**Source:** Levenchuk semiotics slideument slide 11 — six questions to run automatically before treating any artifact as a source of truth
**Core:** frame

Six questions to run on any input artifact before reasoning from it. If any answer is unclear, stop and fix the artifact (or re-frame) before proceeding.
1. **What is the object of the conversation?** If the artifact talks about everything, it is about nothing. Name the specific holon/system/epistemic-object under discussion.
2. **In what context does this hold?** Or is it context-soup mixing design-time / run-time / different scopes / different roles?
3. **What statement type carries each load-bearing sentence?** Apply the X-STATEMENT-TYPE classification (rule / promise / explanation / gate / evidence). Mixed = L1 semiotic error — decompose via CHR-10 (boundary) or X-STATEMENT-TYPE before proceeding.
4. **What is ad-hoc lexicon vs. term?** Local convenient words should not be treated as stable terms. If in doubt, disambiguate (CHR-07) before relying on the word.
5. **Where does same-thing re-expression end and reinterpretation begin?** Summaries, rewordings, and "clarifications" silently shift meaning. Flag the transition.
6. **For what result is this needed?** Or is it stale? Artifacts without a downstream consumer are candidates for deprecation (VER-08).

Skip this checklist and you inherit the incoming artifact's semio-errors silently. Run it especially before summarizing, before generating patches from prose, and before treating explanations as gates. Question 3 is the classification entry point; X-STATEMENT-TYPE is the taxonomy authority.

## FRAME-09: Strict Distinction Quad (Role / Capability / Method / Work)
**Trigger:** Statements mix "assigned" / "can do" / "should do" / "did"; decisions fail because these were treated as the same thing
**Source:** Levenchuk semiotics slideument slide 13 (A.7 Strict Distinction / Clarity Lattice), adapted for haft

Four fundamentally distinct objects — with 4×N distinct descriptions each:
- **Role** — mask worn by a system or epistem; who acts in what capacity. Assignment is not capability.
- **Capability** — what the system can do within the admissible operating range. Having capability is not execution.
- **Method** — how it should be done (design-time specification). Specified method is not performed work.
- **Work** — what actually happened when resources were consumed (run-time).

Plus three cross-cutting descriptors:
- **Promise content** — what is promised outward
- **Evidence role** — what counts as evidence for a claim
- **Scope** — where the statement holds at all

Typical errors this catches:
- "Assigned role X → therefore has capability X" (no — role ≠ capability)
- "Method specified → therefore work will happen" (no — method ≠ work)
- "One successful run → capability proven" (no — a single work instance does not close capability claim)
- Treating object descriptions as if they were the objects themselves
- Mixing design-time method specs with run-time work observations in the same assurance tuple

Use this as a finer-grained replacement for the binary design-time / run-time split in X-DESIGNRUN when the mix-up involves agency and enactment, not just timing.
