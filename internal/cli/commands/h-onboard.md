---
description: "Onboard a project into Haft v7 specs and readiness"
---

# Project Onboarding

Onboard the repository into Haft v7. The goal is not to create generic notes.
The goal is to make the project harnessable by producing parseable authority
carriers:

```text
TargetSystemSpec
  -> EnablingSystemSpec
  -> TermMap
  -> SpecCoverage gaps
  -> DecisionRecords
  -> WorkCommissions
  -> RuntimeRuns
  -> Evidence
```

Use h-reason discipline before editing artifacts: frame the target system,
separate target-system claims from enabling-system mechanics, identify the
weakest link, then make a small reversible artifact change.

## Phase 0: Readiness

1. Check whether `.haft/` exists.
2. Check whether `.haft/specs/target-system.md`,
   `.haft/specs/enabling-system.md`, and `.haft/specs/term-map.md` exist.
3. Run `haft spec check`.
4. If the project is `needs_init`, tell the user to run `haft init` first.
5. If the project is `needs_onboard`, continue with the spec drafting loop.

Do not start broad harness/runtime execution while the project is
`needs_onboard`. A tactical exception must be explicit and recorded with an
operator reason.

## Phase 1: TargetSystemSpec

Draft or refine `.haft/specs/target-system.md` first. Each load-bearing claim
must have a fenced `yaml spec-section` block.

Minimum target sections:

- environment change: what must change in the project's environment;
- target role: what the target system does for external actors;
- boundaries: in-scope and out-of-scope behavior;
- interfaces: externally visible contracts;
- invariants: what must remain true;
- acceptance/evidence: how the target behavior is observed;
- risks/WLNK: the weakest link and known failure modes.

Do not derive target purpose from repo folders, frameworks, or agent plans.
Those are enabling-system facts unless they describe externally required
behavior.

## Phase 2: EnablingSystemSpec

Draft or refine `.haft/specs/enabling-system.md` second.

Minimum enabling sections:

- creator graph: human principal, host agents, Haft Core, CLI, Desktop,
  harness runtime, CI, external carriers;
- work methods: how specs, decisions, commissions, runtime runs, and evidence
  are produced;
- effect boundaries: what each actor/surface may mutate;
- agent policy: v7 product support is Claude Code and Codex; other hosts are
  deferred or experimental;
- commission policy: WorkCommission is bounded authorization, not execution;
- runtime policy: CLI/Desktop start the harness runtime, plugin mode does not
  own long-running lifecycle;
- evidence policy: no verified coverage without evidence and freshness.

## Phase 3: TermMap

Draft or refine `.haft/specs/term-map.md`.

Capture load-bearing terms only. Each term must distinguish object,
description, and carrier when relevant. Required early terms usually include:

- HarnessableProject
- TargetSystemSpec
- EnablingSystemSpec
- SpecSection
- SpecCoverage
- DecisionRecord
- WorkCommission
- RuntimeRun
- Evidence
- ExternalProjection

## Phase 4: Check And Report

Run `haft spec check` again and report:

```text
readiness: ready | needs_init | needs_onboard | missing
spec check: clean | blocked
target sections: N active / M total
enabling sections: N active / M total
term-map entries: N
remaining gaps:
- ...
next safe action:
- ...
```

Only after active target and enabling sections plus term-map entries pass
`haft spec check` should the project move toward spec planning, decisions, and
commissions.

$ARGUMENTS
