# FPF Patterns: Characterize Phase

## CHR-01: Indicator Role Taxonomy
**Trigger:** Defining what gets measured; preventing metric hijacking
**Spec:** B.3.4.1, C.11.4.2.2
**Core:** frame,characterize

Mark each indicator as constraint (hard limit, must pass), target (what you're optimizing, 1-3 max), or observation (watch for risks, don't optimize). Unmarked indicators corrupt your goal. Anti-Goodhart: explicitly call out what you will NOT turn into a KPI.

## CHR-02: Characterization Protocol Pipeline
**Trigger:** Every time you need comparable data across variants
**Source:** Levenchuk FPF seminar (slideument), adapted for haft
**Core:** true

Follow the pipeline: normalize > indicatorize > score > fold (optional) > compare > select. Start with unbounded characteristics (what could you measure?), filter to explicit indicators for this cycle via marked rule, apply norms/units uniformly, avoid single magic KPI, output partial order or Pareto set. Never average disparate scales; preserve multidimensionality.

## CHR-03: Formality Scale (F)
**Trigger:** Comparing rigor levels across different claims or models
**Spec:** B.3.4.1, C.2.3

Formality is ordinal: F0 (informal prose) > F1 (structured narrative) > F2 (formalizable schema) > F3 (proof-grade formalism). Higher formality caps assurance: if one premise is F0, the whole chain is capped at F0. Never average formality; use min. Monotone: raising F is always safe.

## CHR-04: Assurance Tuple (F-G-R-CL)
**Trigger:** Reporting confidence in a result; comparing assurance across work streams
**Spec:** B.3.4.2
**Core:** true

An assurance tuple for a typed claim: (F_eff, G_eff, R_eff, Notes). Notes include Sources, Cutset (weakest link), CL_min (integration bottleneck), and Evidence Graph Ref. This is a snapshot of trust in one specific claim under one specific context.

## CHR-05: Proof Spine and Critical Path
**Trigger:** Reliability is too low; need to know where to invest effort
**Spec:** B.3.5.1, B.3.5.2, B.3.5.3

Identify the component(s) or edge(s) with the lowest R or CL — that cutset caps R_eff. Improving anything outside the cutset will not raise reliability. Invest in the cutset first.

## CHR-06: Dual-Language Projection
**Trigger:** Communicating across role boundaries; hybrid teams
**Source:** Levenchuk FPF seminar (slideument slide 4 — engineering vs management language), adapted for haft

Maintain two views: engineering language (models, tests, reproducibility, causality) and management language (agents, budgets, coordination, roles). Same problem, two dialects. Use engineering language for evidence/verification; management language for coordination/autonomy.

## CHR-07: Term Disambiguation as Artifact
**Trigger:** Ambiguous domain terms in problem statements or comparison specs
**Source:** Levenchuk semiotics slideument (slides 4, 12) + F.18 Naming Protocol, adapted for haft

Create a Disambiguation Record: term surface form, candidate senses, chosen sense + confidence, alternatives rejected with reasons, evidence, valid_until date, affected artifacts. Link forward into decision artifacts. Don't embed term choices in prose — persist them as structured records.

## CHR-08: L1/L2/L3 Ambiguity Control
**Trigger:** Prose or specs risk term confusion; essential before summaries
**Source:** Levenchuk semiotics slideument (slides 10, 16) + A.7 Strict Distinction, adapted for haft

L1 = deterministic triggers (vague adjectives, underspecified relations, acronyms, numbers without units). L2 = persistent term bindings with IDs, candidate-sense storage. L3 = watchdog checks: entity/numeric/unit preservation in summaries, support verification. Don't skip L1; it catches 60-70% of problems.

## CHR-09: Characterization Passport and Parity Plan
**Trigger:** Before any solution comparison; ensures reproducibility
**Source:** Haft operational pattern
**Core:** true

Characterization Passport = explicit rules for this cycle: which characteristics, indicators, role viewpoints, norms/units, comparison windows, validity period. Parity Plan = checklist that comparison is fair: equal budgets, time windows, eval protocol version, data freshness, minimum repeats. Without these, "better/worse" dissolves into opinion.

## CHR-10: Boundary Norm Square (L / A / D / E)
**Trigger:** Reviewing a boundary statement — requirement, contract, SLA, API doc, policy — that mixes multiple concerns in one sentence
**Source:** Levenchuk FPF A.6.B Boundary Norm Square (semiotics slideument slides 33-34), adapted for haft
**Core:** characterize

Decompose a mixed boundary formulation into four quadrants before treating it as a single rule:
- **L (Law / Definition)** — what is being defined; the meaning of the terms used
- **A (Admissibility / Gate)** — under what condition the object/action is admissible; a hard condition
- **D (Deontics / Obligation)** — who has a duty to do what; permission / prohibition / requirement on an agent
- **E (Evidence / Work-Effect)** — what counts as evidence; what observable effect or carrier establishes that something happened

Classic ambiguous forms to decompose:
- "Clients MUST send header X; otherwise request invalid, system writes NotAdmissible" — mixes A (admissibility gate), D (client duty), and E (audit log carrier). Split into three separate claims.
- "Service guarantees 99.9% availability and MUST keep p95 latency < 200ms; operators alert on breach" — mixes D (provider obligation), E (metric definitions + carriers), D (operator duty), E (alert evidence). Name who promises, what metric, which viewpoint, which carrier.
- "System aligned enough for deployment" — mixes L (what alignment means), A (deployment gate), D (who decides), E (what evidence). Refuse to treat this as a decision until all four are separated.

Do not let one sentence carry rule + promise + evidence simultaneously. Split into multiple claims with explicit owner and review point. Only then is the text admissible for audit, automation, or handoff.

## CHR-11: Relational Precision Restoration Pipeline
**Trigger:** Load-bearing word in a spec or decision is ambiguous ("quality", "done", "service", "same", "based on", "надо", "готово") — a single renaming won't fix it because the ontology underneath is unclear
**Source:** Levenchuk FPF A.6.P Relational Precision Restoration. Structure here is the slideument slide 35 didactic 5-step compression; the canonical normative structure in FPF-Spec A.6.P:4 is four layers (4.1 Stable lens → 4.2 Kind-explicit relation tokens → 4.3 Slot-explicit qualified relation records → 4.4 Change-class lexicon → 4.5 Lexical guardrails). Use the 5-step form below as an entry heuristic; retrieve A.6.P:4 via `haft_query(action="fpf", query="A.6.P")` for conformant authoring.

Five-step pipeline (didactic form) when a lexical fix alone isn't enough — the word is an umbrella that spans multiple kinds in the current situation:
1. **Detect the umbrella** — name the specific trigger word that is pulling weight without precision. Do not proceed by replacement-guessing.
2. **Ground by context** — describe the actual relations of the objects in the situation (Wittgenstein: meaning is given by use — by the object's relations to other objects). What does this word connect to in *this* project, not in general? (Canonical: A.6.P:4.1 stable lens + A.6.P:4.2 kind-explicit relation tokens.)
3. **Pick a math lens** — choose the formal lens that makes reasoning about these relations tractable (order, partial order, algebra, graph, measure, state machine, ...). The lens constrains what can be said coherently. (Canonical: A.6.P:4.1.)
4. **Restore a coherent ontology** — separate the kinds the umbrella was collapsing. Multiple abstraction layers may emerge. Record the distinctions. (Canonical: A.6.P:4.3 slot-explicit qualified relation records.)
5. **Pick the lexicon** — name each distinguished kind via F.18 Naming Protocol (dual-name: technical + plain; Pareto over SemanticFidelity / CognitiveErgonomics / OperationalAffordance / AliasRisk — see X-TERM-QUALITY). (Canonical: A.6.P:4.4 change-class lexicon + A.6.P:4.5 lexical guardrails.)

Without steps 2-4, step 5 just reshuffles umbrellas. Without step 1, you don't even know there's a problem. For specific umbrella families (quality, action, service, sameness, wholeness), see CHR-12 for shortcut specializations.

## CHR-12: Umbrella-Word Family Specializations
**Trigger:** A specific known family of overloaded words appears (quality, action-invitation, service, sameness, wholeness, relation-slot, basedness)
**Source:** Levenchuk FPF A.6.P specializations (semiotics slideument slide 35), adapted for haft

CHR-11 is the general pipeline. When the umbrella falls into a known family, apply the family-specific specialization — it encodes which kinds tend to be collapsed and how to split them:
- **A.6.Q — Quality Terms** ("quality", "готово", "надёжно", "качественно", "понятно", -ility words). Translate vague quality talk into evaluative burden with owner and scale. Every quality claim needs: who evaluates, on what scale, with what evidence.
- **A.6.A — Action Invitations** ("надо", "лучше", "можно", "сделай", "метод предлагает"). Distinguish fact / requirement / completed work / action invitation. Shows where text is pressing on action but is not yet method, work, or instruction. Prepares handoff to execution-facing owner.
- **A.6.5 — Relation Slots** ("slot", "argument", "value", "reference"). Repairs confusion between position, value, and id.
- **A.6.6 — Basedness** ("based on", "anchored in", "опирается на", "rebase"). Clarifies what kind of dependency is actually asserted.
- **A.6.8 — Service Polysemy** ("service", "server", "service provider", "SLA", "SLO", "access point", "promise"). Separates accountable provider / running instance / interface description / obligation.
- **A.6.9 — Cross-Context Sameness** ("same", "equivalent", "align", "mapping", "совпадает"). Distinguishes mathematical identity, physical identity, context-bridge equivalence, and mere aliasing.
- **A.6.H — Wholeness** ("whole", "part", "integrity", "boundary", "completeness"). Separates part-of from instance-of from scope-of from coverage-of.

When in doubt which family applies, fall back to CHR-11 (full pipeline). When multiple families apply to one sentence, run them in order: sameness → wholeness → service → relation-slot → quality → action.
