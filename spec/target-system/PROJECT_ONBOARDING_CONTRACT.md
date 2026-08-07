# Project Onboarding Contract

> Product contract for turning any existing or greenfield repository into a
> harnessable project.

## Purpose

Haft's onboarding flow exists to create the missing harness around a project.
It is not a quick README summarizer and not a lightweight task generator.

The onboarding agent must help the human principal build a large, formal,
parseable specification set:

```text
repository
  -> .haft initialized
  -> TargetSystemSpec
  -> SoftwareSystemSpec
  -> TermMap
  -> SpecCoverage
  -> Decisions
  -> WorkCommissions
  -> Harness runtime execution
```

This is the product distinction from runner-only systems. Runner-only systems
assume the project is already ready for harness engineering. Haft constructs
that readiness.

## Actors

| Actor | Role | May decide |
|-------|------|------------|
| Human principal | Owns value, scope, authority, and trade-offs | Product intent, target-system role, acceptance, autonomy envelope |
| Onboarding agent | Reads repo/docs/code, asks questions, drafts specs, points out gaps | Nothing irreversible; may propose structured sections |
| Haft core | Parses specs, validates structure, stores artifacts, computes coverage | Deterministic admissibility only |
| Operator cockpit | Primary human surface for onboarding and readiness | UI does not invent semantics |
| MCP host agent | Embedded Claude Code/Codex surface during coding/reasoning | May create drafts/notes/commissions only inside explicit tool contracts |
| CLI Harness | Operator surface for runtime prepare/run/status/result/apply/cancel | Does not author product meaning |
| Harness runtime | Executes WorkCommissions | No spec authoring authority |

## E2E Flow

### Scenario: Add an existing project

```gherkin
Scenario: Operator cockpit adds an existing repository
  Given a user selects a repository path
  When Haft adds the project
  Then Haft verifies the path exists
  And detects whether ".haft/" exists
  And detects supported v7 host-agent configuration surfaces
  And shows one of:
    | state          | meaning                                      |
    | ready          | .haft exists and spec check passes           |
    | needs_init     | no .haft directory or missing base config    |
    | needs_onboard  | .haft exists but ProjectSpecificationSet is missing or incomplete |
    | missing        | path no longer exists                        |
```

### Scenario: Initialize project harness

```gherkin
Scenario: Init creates the local harness carrier
  Given a project is "needs_init"
  When the user runs "Initialize Haft"
  Then Haft creates ".haft/"
  And configures selected supported host agents for MCP:
    | selection        | host        | carrier                     |
    | --claude         | Claude Code | .mcp.json or supported local config |
    | --codex/--all    | Codex       | .codex/config.toml           |
    | --grok           | Grok CLI    | .grok/config.toml            |
  And installs ".agents/skills" only when "--agents" is explicit
  And bare TTY init selects no host until the operator toggles one
  And bare non-TTY init fails before writes
  And does not configure experimental/deferred hosts unless explicitly requested
  And creates ".haft/workflow.md"
  And creates parseable draft spec carriers that do not claim active product meaning:
    | file                              |
    | .haft/specs/target-system.md       |
    | .haft/specs/software-system.md     |
    | .haft/specs/term-map.md            |
  And does not create fake decisions
```

v7 host support is intentionally narrow:

| Host | Status | Reason |
|------|--------|--------|
| Claude Code | supported | Primary embedded coding-agent surface for local projects |
| Codex | supported | Primary embedded coding-agent surface for Codex CLI/App workflows |
| Grok CLI | experimental | Project `.grok/config.toml` MCP + skills; overrides Claude/Cursor compat shadowing |
| Cursor | experimental/deferred | May remain installable, not v7 acceptance target |
| Gemini CLI | experimental/deferred | May remain installable, not v7 acceptance target |
| JetBrains Air | experimental/deferred | May remain installable, not v7 acceptance target |
| Generic MCP client | experimental/deferred | Protocol-compatible does not imply product support |


### Scenario: Onboard target system first

```gherkin
Scenario: Onboarding agent drafts TargetSystemSpec
  Given a project has .haft initialized
  When the user starts onboarding
  Then Haft starts an onboarding conversation with the selected host agent
  And the agent reads repository carriers:
    | carrier examples |
    | README, docs, package manifests, API schemas, tests, source tree |
  And the agent explains why formal target specification is required for harness engineering
  And the agent asks for missing value/scope/acceptance decisions
  And the agent drafts TargetSystemSpec sections with stable ids
  And the human principal approves or edits load-bearing target statements
  And Haft parses the resulting spec before marking target spec ready
```

Target spec readiness requires:

- Environment-change statement.
- Target-system role.
- Boundary and out-of-scope statements.
- Term map entries for load-bearing terms.
- External actors and scenarios, or explicit "not yet known" sections.
- Acceptance/evidence requirements.
- Risks/WLNK/refresh triggers.

### Scenario: Software spec starts only after target spec is admissible

```gherkin
Scenario: Software spec depends on target spec
  Given TargetSystemSpec has not passed structural validation
  When the user asks Haft to build SoftwareSystemSpec
  Then Haft blocks full software spec activation
  And shows the missing target sections
  And allows only draft software sections, not active software contracts
```

```gherkin
Scenario: Onboarding agent drafts SoftwareSystemSpec
  Given TargetSystemSpec passes structural validation
  When the user starts software-system onboarding
  Then the agent models the required software role, functional behavior, interfaces, and constraints
  And adds responsibility allocation, procedural behavior, and selected structure when the project needs those sections
  And drafts SoftwareSystemSpec sections with stable ids
  And each software section references target sections where relevant
  And Haft parses and validates the software spec
  And the agent does not put repository workflow, CI, agent policy, release policy, or harness runtime rules into SoftwareSystemSpec
```

### Scenario: Spec check produces operator work

```gherkin
Scenario: Spec check finds gaps
  Given a ProjectSpecificationSet exists
  When Haft runs "spec check"
  Then it returns:
    | result kind          |
    | parse errors         |
    | missing required sections |
    | ambiguous terms      |
    | target/software/enabling boundary confusion |
    | uncovered spec sections |
    | stale sections       |
    | conflicting sections |
  And each result includes the exact spec section id and suggested next action
```

### Scenario: Create decisions from specification

```gherkin
Scenario: Important applied choices receive DecisionRecords
  Given TargetSystemSpec and SoftwareSystemSpec are structurally valid
  And a software section contains or depends on an important applied choice
  When the user uses whatever independent analysis capabilities that concern needs
  And directly and unambiguously requests that h-decide bind the choice
  Then the DecisionRecord references the software and target sections it governs
  And the selected software contract is reflected in SoftwareSystemSpec
  And Haft does not treat every section or bullet as a decision
  And Haft does not infer human approval from spec prose
```

Rules:

- DecisionRecord is not WorkPlan.
- A DecisionRecord may govern one or more spec sections.
- A large spec may produce many decisions, but each decision must represent a
  meaningful choice, boundary, or implementation policy.
- Pure implementation chores should become WorkCommissions under an existing
  decision, not new decisions.

### Scenario: Create WorkCommissions from decisions

```gherkin
Scenario: Commission selected decisions
  Given one or more active DecisionRecords reference spec sections
  When the user creates WorkCommissions
  Then Haft derives default scope and evidence requirements from:
    | source |
    | DecisionRecord affected files/modules |
    | SpecCoverage code/test links |
    | .haft/workflow.md |
    | applicable MethodPack gates |
  And the user may narrow or widen scope explicitly
  And each WorkCommission stores spec section refs in its snapshot
```

### Scenario: Harness runtime executes only commissioned work

```gherkin
Scenario: Runtime consumes WorkCommissions
  Given WorkCommissions are queued
  When a separately operated external runner starts
  Then the runner requests runnable commissions from Haft
  And does not poll external trackers for authority
  And does not choose new spec work by itself
  And submits RuntimeRun lifecycle observations and evidence candidates to Haft
  And Haft updates SpecCoverage after evidence is accepted
```

## Onboarding UX Contract

The onboarding flow is intentionally deep. The UX goal is not to hide that
depth. The UX goal is to make the depth navigable, observable, and useful.

Required operator surfaces:

| Surface | Must show |
|---------|-----------|
| Project readiness | init/spec/coverage/runtime readiness states |
| Target spec workspace | section tree, missing sections, term gaps, human approval points |
| Software spec workspace | software role, responsibility allocation, behavior, interfaces, constraints, selected structure |
| Spec coverage | spec sections grouped by uncovered/reasoned/commissioned/verified/stale |
| Decision trace | identified applied choices, linked DecisionRecords, and missing-link cues |
| Runtime cockpit | runnable/running/blocked/completed commissions and evidence |

## Surface Workflow Contract

Desktop buttons, MCP tool/slash calls, and CLI commands are surfaces over the
same typed workflow model. They must not send free prompts that silently mutate
semantic state.

Canonical form:

```text
Surface action
  -> WorkflowIntent
  -> PromptStage or CommandStage
  -> ArtifactProposal or ArtifactMutation
  -> DeterministicCheck
  -> DerivedReadinessOrCoverageState
```

Examples:

| Surface action | Typed workflow | Artifact transition |
|----------------|----------------|---------------------|
| `Draft Target Spec` | `onboarding.target_spec.draft` | repo carriers -> draft `SpecSection` blocks |
| `Approve Target Section` | `onboarding.target_spec.approve` | draft section -> active section |
| `Record Decided Choice` | host-routed `/h-decide` workflow | direct unambiguous operator choice + section refs -> DecisionRecord |
| `Create WorkCommission` | `commission.create` | DecisionRecord + scope -> WorkCommission |
| `Delegate externally` | runner-selected lifecycle integration | runnable WorkCommission -> externally performed RuntimeRun -> recorded lifecycle result |
| `Review Evidence` | `evidence.review` | evidence carrier -> derived SpecCoverage state |

Invalid shape:

```text
Button -> opaque prompt -> agent writes arbitrary markdown -> UI shows ready
```

The agent may explain the value of formal specs, but UI must not rely on
marketing prose to compensate for missing state. The operator must be able to
see what is incomplete and what action is next.

## Human Confirmation Points

The human principal must explicitly approve:

- target-system role and environment-change statements;
- boundary and out-of-scope statements;
- term definitions that carry product or architecture meaning;
- software responsibility allocation, interfaces, constraints, and selected structure;
- creation of active DecisionRecords from spec drafts;
- WorkCommission scope widening;
- AutonomyEnvelope approval for high-autonomy external-runner execution;
- any one-way-door action: merge, release, tag, external terminal status.

## Acceptance Criteria for MVP

The smallest honest product proof is:

1. Add an existing repo in the operator cockpit.
2. Initialize `.haft` and host-agent MCP config.
3. Produce parseable target/software spec carriers and a TermMap.
4. Run spec check and see deterministic readiness/gap output.
5. Create at least one DecisionRecord linked to spec section ids.
6. Create one WorkCommission linked to that DecisionRecord and spec refs.
7. Have a separately operated runner perform the WorkCommission and report its
   lifecycle result through the runner-neutral interface.
8. Attach evidence.
9. Show SpecCoverage moved from `commissioned` to `verified` for the relevant section.

Anything less is a task runner with reasoning artifacts nearby, not Haft's
target product.
