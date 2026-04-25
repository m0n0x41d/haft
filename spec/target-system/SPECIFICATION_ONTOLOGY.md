# Specification Ontology

> Reading order: after TERM_MAP and before ARTIFACT_ONTOLOGY.
>
> This document defines the spec-first product layer. In Haft, a large,
> formal, parseable specification is not "documentation". It is the engineering
> harness that makes a project safe for delegated AI work.

## Central Claim

Haft does not assume a repository is ready for harness engineering.

Haft makes a project harnessable by constructing and maintaining a
**ProjectSpecificationSet**:

```text
ProjectSpecificationSet
  = TargetSystemSpec
  + EnablingSystemSpec
  + TermMap
  + SpecCoverage
  + WorkflowPolicy
```

The harness runtime may execute WorkCommissions only after the project has at
least the relevant specification sections, term definitions, decision links,
scope, and evidence requirements needed to make the work admissible.

## Target vs Enabling System

Every onboarded project has two different systems in scope:

| System | What it is | Specification role |
|--------|------------|--------------------|
| **Target system** | The software/product/service whose behavior must change in its environment | Describes what must be true in the world, what role the product plays, boundaries, interfaces, invariants, terms, and acceptance |
| **Enabling system** | The engineering system that builds, changes, verifies, and operates the target system | Describes repository architecture, build/test methods, delivery policies, agent usage, harness runtime, CI, hooks, and evidence methods |

The target spec is written first. The enabling spec is downstream: it exists to
produce and maintain the target system. A project may have an incomplete
enabling spec, but Haft must not let enabling-system mechanics silently define
the target-system purpose.

## ProjectSpecificationSet

**Definition:** The set of parseable project-local specs that make a repository
harnessable.

Required files in canonical form:

```text
.haft/specs/target-system.md
.haft/specs/enabling-system.md
.haft/specs/term-map.md
.haft/workflow.md
```

Optional derived files:

```text
.haft/specs/coverage.md
.haft/specs/open-questions.md
.haft/specs/spec-check.json
```

Rules:

- Specs are git-tracked local exchange carriers.
- Structured spec objects are parsed into Haft SQLite before they can govern
  decisions or commissions.
- The markdown files are carriers, not acting agents and not semantic authority
  by themselves.
- Init-time carriers may contain parseable draft placeholders with
  `claim_layer: carrier` and `status: draft`; validators must not treat those
  placeholders as active product or enabling-system claims.
- `haft spec check` validates carriers, parses them into canonical objects, and
  reports missing, stale, conflicting, or uncovered sections.

## TargetSystemSpec

**Definition:** Parseable specification of what the target system must do in its
environment.

TargetSystemSpec has two canonical parts. This prevents repo architecture from
silently replacing product purpose.

```text
TargetSystemSpec
  A. Concept of Use / black-box
  B. Concept of System / white-box
```

### A. Concept of Use / black-box

Describes the target system from outside: what changes in the environment,
who/what interacts with it, what scenarios matter, which boundaries are
load-bearing, and what observable evidence counts.

Required section kinds:

| Section kind | Required content |
|--------------|------------------|
| `environment-change` | What must change in the environment of the target system |
| `method` | The method or mode by which the environment is changed |
| `target-role` | The role the target system plays in that change |
| `external-actors` | Human, system, and organizational actors outside the target system |
| `scenarios` | Operational scenarios and black-box behavior |
| `boundaries` | In-scope/out-of-scope, external systems, authority boundaries |
| `acceptance` | Observable post-conditions and evidence required |

### B. Concept of System / white-box

Describes the target system from inside: what it is made of, how it operates,
which interfaces exist, which invariants must hold, and what risks bound its
quality.

Minimum required sections:

| Section kind | Required content |
|--------------|------------------|
| `materials` | What the target system is made from: entities, data, protocols, resources, constraints |
| `target-work-methods` | Methods of operation inside the target system |
| `term-map` | Terms used by the target spec, with forbidden meanings |
| `interfaces` | Public API/UI/protocol/event surfaces |
| `invariants` | Conditions that must always hold |
| `risks` | Weakest links and refresh triggers |

This is intentionally large and formal. The product value is not low ceremony.
The value is making delegated engineering work admissible.

## EnablingSystemSpec

**Definition:** Parseable specification of how the project is built, changed,
verified, and governed.

Minimum required sections:

| Section kind | Required content |
|--------------|------------------|
| `creator-role` | What role the enabling system plays for the target system |
| `creator-graph` | Human principal, onboarding agent, coding agent, verifier, CI, harness runtime, repo/DB/spec carriers, and optional external tracker carrier |
| `creator-actors` | Human principal, agents, CI, harness runtime, external carriers |
| `method-boundaries` | Who may draft, approve, execute, verify, publish, revise, or cancel |
| `work-methods` | Reasoning, design, implementation, verification, review, release methods |
| `repo-architecture` | Modules, layers, dependency rules, ownership surfaces |
| `effect-boundaries` | Filesystem, DB, network, agent, tracker, terminal, build/test effects |
| `test-strategy` | Behavior/interface/spec tests, contract tests, E2E, prohibited test shapes |
| `agent-policy` | Which agents may act, through which surfaces, with which permissions |
| `surface-policy` | Desktop, MCP, and CLI responsibilities and non-authority constraints |
| `autonomy-envelope-policy` | Checkpointed default, batch/YOLO approval, budget and one-way-door restrictions |
| `commission-policy` | Scope, lockset, evidence, projection, and autonomy defaults |
| `runtime-policy` | Harness runtime install/start/observe/apply/cancel model |
| `hooks-and-ci` | Local hooks, CI checks, spec check, evidence refresh |
| `release-policy` | Branch, PR, tag, changelog, release gates |

The enabling spec may reference target spec sections, but it must not redefine
target-system goals. If an enabling section changes what the product is for,
Haft must route back to TargetSystemSpec revision.

## SpecSection

**Definition:** One addressable unit in a parseable spec.

Canonical fields:

```yaml
id: TS.environment-change.001
kind: environment-change
title: Short human-readable name
statement_type: definition | admissibility | duty | evidence | explanation
claim_layer: object | description | carrier | work | evidence
owner: human | haft | agent | ci | external-carrier
status: draft | active | deprecated | superseded
carrier_claim_allowed: false
valid_until: 2026-07-24
depends_on: []
supersedes: []
terms: []
target_refs: []
evidence_required: []
```

Rules:

- `id` is stable. Renaming text must not change the id.
- `statement_type` is required for every load-bearing section.
- `claim_layer` is required so validators can detect object/description/carrier/work/evidence confusion.
- Mixed statement types are illegal. Split the section instead.
- `owner` names who can change the fact, not who can describe it.
- Active target-system sections must not use `claim_layer: carrier` for product
  object claims unless `carrier_claim_allowed: true` is explicit. This is a
  deterministic authority-boundary guard, not proof that the section is true.
- `valid_until` is required for sections that depend on context, market,
  architecture, dependencies, or operational assumptions.

## Strict Markdown Carrier Format

Canonical section carrier:

````markdown
## TS.environment-change.001 Short title

```yaml spec-section
id: TS.environment-change.001
kind: environment-change
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-07-24
terms:
  - PaymentPlan
evidence_required:
  - kind: review
    description: Human confirms the environment-change statement still matches product intent.
```

Prose body in controlled natural language.

### Invariants

- ...

### Acceptance

- ...
````

The prose body is allowed because humans need readable specs. The YAML block is
required because agents and validators need a canonical object.

## TermMap

**Definition:** A parseable vocabulary for the target and enabling specs.

Canonical entry:

```yaml
term: WorkCommission
domain: enabling
definition: Human-authorized bounded permission to execute a DecisionRecord in a declared Scope.
not:
  - DecisionRecord
  - RuntimeRun
  - tracker ticket
aliases:
  - commission
owners:
  - haft
```

Rules:

- A term must have exactly one definition in one domain.
- If the same word is needed in target and enabling domains with different
  meanings, create domain-qualified terms.
- Ambiguous terms such as service, process, component, quality, simple,
  scalable, done, and validated require explicit disambiguation before they may
  appear in load-bearing sections.

## SpecCoverage

**Definition:** The relation that connects specifications to reasoning,
execution, code, tests, and evidence.

Canonical edge types:

| Edge | Meaning |
|------|---------|
| `spec_section -> ProblemCard` | This problem frames a gap, contradiction, or change in the section |
| `spec_section -> DecisionRecord` | This decision selects how the section is satisfied or changed |
| `DecisionRecord -> WorkCommission` | This decision has bounded execution work |
| `WorkCommission -> RuntimeRun` | This commission was attempted by a runtime |
| `RuntimeRun -> EvidencePack` | This run produced evidence |
| `spec_section -> file/module/function` | This code surface implements or supports the section |
| `spec_section -> test` | This test provides behavioral/interface/spec evidence for the section |

Coverage states are derived, not stored:

| State | Derivation |
|-------|------------|
| `uncovered` | Active spec section has no DecisionRecord and no evidence |
| `reasoned` | Active spec section has one or more active DecisionRecords |
| `commissioned` | At least one active WorkCommission exists |
| `implemented` | Evidence shows code was changed in scope |
| `verified` | Evidence satisfies required checks and has not decayed |
| `stale` | Evidence or linked decision/spec section is expired or drifted |

Spec coverage is not test coverage. Test coverage is one evidence carrier.
Spec coverage asks whether the architecture statement is governed, implemented,
and verified.

## Semantic Architecture

**Definition:** The explicit relation model that keeps the target and enabling
systems from drifting into term confusion.

Minimum relation kinds:

| Relation | Meaning |
|----------|---------|
| `is-made-of` | Composition/material relation |
| `changes-environment-by` | Target behavior relation |
| `depends-on` | Runtime or design dependency |
| `governs` | A spec/decision constrains a code/module/runtime surface |
| `verifies` | Evidence checks a claim/spec section |
| `projects-to` | Local/external carrier relation |
| `supersedes` | Lifecycle replacement |
| `blocks` | Admissibility relation |

Haft must preserve object/description/carrier distinction:

- Target system object is not its spec.
- Spec carrier is not the parsed spec object.
- DecisionRecord is not implementation.
- RuntimeRun is not evidence by itself.
- External tracker text is not semantic authority.

## Validator Levels

| Level | Name | Responsibility |
|-------|------|----------------|
| L0 | Parse | Markdown/YAML syntax, IDs, required fields |
| L1 | Structural | Required sections, unique terms, valid links, no mixed statement types |
| L1.5 | Deterministic shape/authority guard | TermMap entry shape, optional section field shape, duplicate aliases, and obvious carrier/object authority confusion |
| L2 | Semantic | target/enabling split, ambiguous term use, owner/authority consistency, spec coverage gaps |
| L3 | Runtime | stale/drift detection, evidence decay, commission freshness, code/test/spec link health |

Current `haft spec check` covers deterministic L0/L1/L1.5 checks only. It is
not an LLM review, not proof of product correctness, and not L3 runtime
evidence. The product promise still requires L2/L3 to become first-class;
without L2/L3, large specs become documentation again.

## Compilation Chain

The intended chain is mechanical:

```text
Strict markdown carriers
  -> Parsed ProjectSpecificationSet
  -> SpecSections + TermMap + SemanticArchitecture
  -> SpecCoverage graph
  -> ProblemCards for gaps/conflicts
  -> DecisionRecords for chosen changes
  -> WorkCommissions for bounded execution
  -> RuntimeRuns
  -> EvidencePacks
  -> refreshed SpecCoverage
```

No downstream object may invent upstream authority:

- WorkCommission may not invent a DecisionRecord.
- DecisionRecord may not invent a spec section it claims to satisfy.
- RuntimeRun may not expand WorkCommission Scope.
- Evidence may support or weaken a section, but it does not rewrite the section.
