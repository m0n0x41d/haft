# FPF Patterns: Characterize Phase

## CHR-01: Indicator Role Taxonomy
**Trigger:** Defining what gets measured; preventing metric hijacking
**Spec:** B.3.4.1, C.11.4.2.2

Mark each indicator as constraint (hard limit, must pass), target (what you're optimizing, 1-3 max), or observation (watch for risks, don't optimize). Unmarked indicators corrupt your goal. Anti-Goodhart: explicitly call out what you will NOT turn into a KPI.

## CHR-02: Characterization Protocol Pipeline
**Trigger:** Every time you need comparable data across variants
**Source:** Haft operational pattern (derived from engineering practice)

Follow the pipeline: normalize > indicatorize > score > fold (optional) > compare > select. Start with unbounded characteristics (what could you measure?), filter to explicit indicators for this cycle via marked rule, apply norms/units uniformly, avoid single magic KPI, output partial order or Pareto set. Never average disparate scales; preserve multidimensionality.

## CHR-03: Formality Scale (F)
**Trigger:** Comparing rigor levels across different claims or models
**Spec:** B.3.4.1, C.2.3

Formality is ordinal: F0 (informal prose) > F1 (structured narrative) > F2 (formalizable schema) > F3 (proof-grade formalism). Higher formality caps assurance: if one premise is F0, the whole chain is capped at F0. Never average formality; use min. Monotone: raising F is always safe.

## CHR-04: Assurance Tuple (F-G-R-CL)
**Trigger:** Reporting confidence in a result; comparing assurance across work streams
**Spec:** B.3.4.2

An assurance tuple for a typed claim: (F_eff, G_eff, R_eff, Notes). Notes include Sources, Cutset (weakest link), CL_min (integration bottleneck), and Evidence Graph Ref. This is a snapshot of trust in one specific claim under one specific context.

## CHR-05: Proof Spine and Critical Path
**Trigger:** Reliability is too low; need to know where to invest effort
**Spec:** B.3.5.1, B.3.5.2, B.3.5.3

Identify the component(s) or edge(s) with the lowest R or CL — that cutset caps R_eff. Improving anything outside the cutset will not raise reliability. Invest in the cutset first.

## CHR-06: Dual-Language Projection
**Trigger:** Communicating across role boundaries; hybrid teams
**Source:** Haft operational pattern

Maintain two views: engineering language (models, tests, reproducibility, causality) and management language (agents, budgets, coordination, roles). Same problem, two dialects. Use engineering language for evidence/verification; management language for coordination/autonomy.

## CHR-07: Term Disambiguation as Artifact
**Trigger:** Ambiguous domain terms in problem statements or comparison specs
**Source:** Haft operational pattern (derived from semiotics practice)

Create a Disambiguation Record: term surface form, candidate senses, chosen sense + confidence, alternatives rejected with reasons, evidence, valid_until date, affected artifacts. Link forward into decision artifacts. Don't embed term choices in prose — persist them as structured records.

## CHR-08: L1/L2/L3 Ambiguity Control
**Trigger:** Prose or specs risk term confusion; essential before summaries
**Source:** Haft operational pattern (derived from semiotics practice)

L1 = deterministic triggers (vague adjectives, underspecified relations, acronyms, numbers without units). L2 = persistent term bindings with IDs, candidate-sense storage. L3 = watchdog checks: entity/numeric/unit preservation in summaries, support verification. Don't skip L1; it catches 60-70% of problems.

## CHR-09: Characterization Passport and Parity Plan
**Trigger:** Before any solution comparison; ensures reproducibility
**Source:** Haft operational pattern

Characterization Passport = explicit rules for this cycle: which characteristics, indicators, role viewpoints, norms/units, comparison windows, validity period. Parity Plan = checklist that comparison is fair: equal budgets, time windows, eval protocol version, data freshness, minimum repeats. Without these, "better/worse" dissolves into opinion.
