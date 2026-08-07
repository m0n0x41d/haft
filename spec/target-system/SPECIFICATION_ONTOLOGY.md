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
  + SoftwareSystemSpec
  + TermMap
```

SpecCoverage and WorkflowPolicy are related governance objects, not members of
the specification set. The harness runtime may execute WorkCommissions only
after the project has the relevant specification sections, term definitions,
decision links, scope, workflow policy, and evidence requirements needed to
make the work admissible.

## Target System, Software System, and Enabling System

This three-part model is Haft local practice for Agentic SWE. It is not a set
of normative kinds defined by FPF A.1. A described subject may be typed with
FPF primitives such as `U.Entity`, `U.Holon`, or `U.System` and scoped through
an `EntityOfConcernSlot`; Haft's target/enabling distinction remains a
project-practice boundary.

Every onboarded software project must keep three systems distinct:

| System | What it is | Specification role |
|--------|------------|--------------------|
| **Target system** | The product or socio-technical system whose behavior must change in the world | TargetSystemSpec describes the desired environment change, product role, actors, scenarios, boundaries, and acceptance |
| **Software system** | The idealized software system that realizes part of the target-system role | SoftwareSystemSpec describes software role, responsibility allocation, functional and procedural behavior, interfaces, constraints, and selected structure |
| **Enabling system** | The people, agents, methods, repository workflows, tests, CI, and harness runtime that create and maintain the software | Not a ProjectSpecificationSet member; its current policies live in workflow, MethodPack, agent-discipline, decision, commission, and runtime carriers |

TargetSystemSpec supplies the intent and boundary claims that
SoftwareSystemSpec may reference. This dependency does not prescribe an
authoring sequence. When a software claim would change target purpose, the
principal must establish or revise the target claim explicitly rather than let
repository structure define it. Enabling-system mechanics must not be smuggled
into SoftwareSystemSpec.

## ProjectSpecificationSet

**Definition:** The set of parseable project-local specs that make a repository
harnessable.

Required files in canonical form:

```text
.haft/specs/target-system.md
.haft/specs/software-system.md
.haft/specs/term-map.md
```

`.haft/workflow.md` is required for governed work, but it is a workflow-policy
carrier rather than a member of ProjectSpecificationSet.

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
  placeholders as active target-system or software-system claims.
- `haft spec check` validates carriers, parses them into canonical objects, and
  reports missing, stale, conflicting, or uncovered sections.

## TargetSystemSpec

**Definition:** Parseable specification of what the target system must do in its
environment.

TargetSystemSpec describes the product from outside the software realization.
This prevents software and repository architecture from silently replacing
product purpose.

```text
TargetSystemSpec
  = desired environment change
  + method and target role
  + actors and scenarios
  + boundaries and acceptance
  + risks and target-domain terms
```

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
| `risks` | Weakest links and refresh triggers |

This is intentionally large and formal. The product value is not low ceremony.
The value is making delegated engineering work admissible.

## SoftwareSystemSpec

**Definition:** Parseable specification of the idealized software system that
realizes the target-system role.

Required readiness sections:

| Section kind | Required content |
|--------------|------------------|
| `software.role` | The role software plays in realizing the target-system change |
| `software.functional_behavior` | Capabilities and externally observable behavior the software must provide |
| `software.interfaces` | Public API, UI, protocol, event, and integration contracts |
| `software.constraints` | Quality, security, compliance, compatibility, performance, and operational constraints |

Conditional sections:

| Section kind | Include when |
|--------------|--------------|
| `software.responsibility_allocation` | Responsibility crosses software, human, or external-system boundaries |
| `software.procedural_behavior` | Order, state transitions, retries, failure, or recovery behavior matters |
| `software.selected_structure` | Load-bearing internal structure already selected as part of the software contract |

SoftwareSystemSpec may reference TargetSystemSpec sections, but it must not
redefine target-system goals. It also must not absorb build commands, agent
roles, review policy, CI, release mechanics, or harness runtime rules; those
belong to the enabling system.

Important applied choices recorded in SoftwareSystemSpec should link to a
DecisionRecord that preserves alternatives, rationale, and consequences. The
spec carries the selected contract; the DecisionRecord carries why that
contract was selected. Haft does not infer or bind those decisions merely from
prose in a section.

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

**Definition:** A parseable vocabulary for the target and software specs.

Canonical entry:

```yaml
term: WorkCommission
category: enabling
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

- A term must have exactly one definition in one category.
- If the same word is needed in target and software categories with different
  meanings, create category-qualified terms.
- Legacy term maps that still use `domain` parse as a compatibility alias for
  `category`; new carriers should use `category`.
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
| `commissioned` | At least one active recoverable WorkCommission exists; terminal WorkCommission carriers remain graph edges but do not by themselves prove implementation or verification |
| `implemented` | Evidence shows code was changed in scope |
| `verified` | Evidence satisfies required checks and has not decayed |
| `stale` | Evidence or linked decision/spec section is expired or drifted |

Spec coverage is not test coverage. Test coverage is one evidence carrier.
Spec coverage asks whether the specification statement is governed, implemented,
and verified.

## Semantic Architecture

**Definition:** The explicit relation model that keeps the target, software,
and enabling systems from drifting into term confusion.

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

The model is relation-first. Text order, graph insertion order, card order, and
skill order do not establish causal, temporal, method, planning, or
performed-work order. Such order exists only when an explicit causal claim,
`U.MethodDescription`, `ImplementationPlan`, `WorkCommission`, or work relation
states it. Relation-first does not mean acausal: causal claims remain valid,
but Haft does not infer them from layout or adjacency.

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
| L2 | Semantic | target/software/enabling boundary, ambiguous term use, owner/authority consistency, spec coverage gaps |
| L3 | Runtime | stale/drift detection, evidence decay, commission freshness, code/test/spec link health |

Current `haft spec check` covers deterministic L0/L1/L1.5 checks only. It is
not an LLM review, not proof of product correctness, and not L3 runtime
evidence. The product promise still requires L2/L3 to become first-class;
without L2/L3, large specs become documentation again.

## Relations and Explicit Work Plans

The intended model is a typed relation graph, not a universal compilation
sequence:

```text
strict markdown carrier --parses-as--> ProjectSpecificationSet
SpecSection --governs--> DecisionRecord / code / test
DecisionRecord --may-authorize--> WorkCommission
WorkCommission --may-start--> RuntimeRun
EvidencePack --supports-or-weakens--> claim / SpecSection
all current relations --derive--> SpecCoverage
```

These links become mechanically checkable once their inputs exist. They do not
automatically detect every underdetermined applied choice, bind a
DecisionRecord, or prescribe execution order. Planning remains a separate
description: only an explicit `ImplementationPlan` or `WorkCommission` may
state dependencies and scheduling, and neither is performed work.

No downstream object may invent upstream authority:

- WorkCommission may not invent a DecisionRecord.
- DecisionRecord may not invent a spec section it claims to satisfy.
- RuntimeRun may not expand WorkCommission Scope.
- Evidence may support or weaken a section, but it does not rewrite the section.
