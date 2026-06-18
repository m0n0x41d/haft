# Enabling System Spec

## ES.placeholder.001 Enabling system placeholder

```yaml spec-section
id: ES.placeholder.001
kind: creator-role
title: Enabling system placeholder
statement_type: explanation
claim_layer: carrier
owner: human
status: draft
valid_until: null
depends_on: []
supersedes: []
terms: []
target_refs: []
evidence_required: []
```

This placeholder only reserves a parseable carrier for onboarding. It is not active enabling-system governance.

## ES.architecture.001 Layered governance substrate architecture

```yaml spec-section
id: ES.architecture.001
spec: enabling-system
kind: enabling.architecture
title: Layered governance substrate architecture
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - TS.boundary.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that these layers match the maintained Haft architecture and do not let surfaces bypass core/governance boundaries.
  - kind: runtime
    description: CLI and MCP behavior remains routed through kernel validation rather than direct raw carrier or SQLite mutation.
```

Haft's enabling architecture is layered by responsibility. Each layer may depend
only on the layer directly below it; outer surfaces must not bypass the kernel
or mutate raw persistence as an alternate authority path.

### Layers

- Persistence owns SQLite project state, `.haft/` markdown carriers, global
  project registry state, and embedded FPF corpus data.
- Codebase Core owns repository/module/file/symbol/test references derived from
  project files.
- Specification Core owns target/enabling spec parsing, term maps, spec checks,
  section baselines, and spec coverage edges.
- Reasoning Core owns ProblemCards, SolutionPortfolios, DecisionRecords, Notes,
  evidence, refresh state, R_eff, Pareto comparison data, and FPF retrieval.
- Flow owns onboarding, sync, MethodRuns, WorkCommissions, harness execution,
  isolated worktrees, apply paths, and verification loops.
- Governor owns freshness scans, drift detection, stale evidence reporting,
  impact propagation, and problem surfacing.
- Surfaces own operator interaction through CLI, MCP, skills/slash commands,
  and desktop views. Surfaces present and route kernel results; they do not
  become independent governance authority.

### Dependency Rule

Persistence -> Codebase Core -> Specification Core -> Reasoning Core -> Flow
-> Governor -> Surfaces.

No higher layer may reach around the layer below it to reinterpret carriers,
skip validation, or directly create authority-bearing governance state.

## ES.work-methods.001 Load-bearing artifact production rules

```yaml spec-section
id: ES.work-methods.001
spec: enabling-system
kind: enabling.work_methods
title: Load-bearing artifact production rules
statement_type: duty
claim_layer: work
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.architecture.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that binding artifacts remain human-gated and that checks close each work method.
  - kind: runtime
    description: CLI/MCP commands reject missing required fields and spec lifecycle commands surface human gates before baselines.
```

Haft work methods produce load-bearing governance artifacts through typed
commands and explicit checks. A chat statement, markdown paragraph, or agent
intention is not enough to create authority-bearing state.

### Production Rules

- SpecSections are drafted by an agent or human after
  `haft_spec_section(action="lifecycle")` names the phase, carrier, expected
  fields, and checks. The close check is `haft spec check` plus lifecycle status
  moving to either the next draft phase or a human approval gate.
- SpecSection baselines are produced only after human review of an active
  section. The close check is `haft_spec_section(action="approve",
  section_id=...)` returning a recorded hash.
- ProblemCards are produced by framing a signal with constraints, acceptance,
  problem type, and reversibility. The close check is kernel creation of a
  stable `prob-*` artifact.
- SolutionPortfolios are produced by exploration of distinct variants with
  weakest links. The close check is kernel creation of a stable `sol-*`
  artifact with variant identities.
- DecisionRecords are produced only after explicit human invocation of
  `/h-decide` or an equivalent manual CLI/MCP command. The close check is a
  stable `dec-*` artifact with invariants, affected files, predictions,
  rollback, and refresh triggers.
- WorkCommissions are produced only after explicit human invocation of
  `/h-commission` or an equivalent manual CLI/MCP command against an admissible
  DecisionRecord. The close check is a stable `wc-*` artifact with scope,
  autonomy envelope, delivery policy, and evidence requirements.
- RuntimeRuns are produced by Flow-layer harness execution from an admissible
  WorkCommission. The close check is a terminal run or commission verdict plus
  preserved workspace/result evidence.
- Evidence records are produced by verification, measurement, tests, review, or
  runtime observation attached to the relevant artifact. The close check is
  `haft_decision(action="evidence")` or the corresponding evidence command
  returning an evidence id, verdict, congruence level, and expiry.

### Non-Production Rules

- Freeform chat does not produce a spec, decision, commission, baseline, or
  evidence record.
- Agent-written rationale does not bind a DecisionRecord or WorkCommission.
- Markdown carriers do not become authority until parsed and accepted by the
  kernel lifecycle.
- Commits, pushes, PRs, external comments, and release actions require explicit
  operator authorization outside ordinary spec drafting.

## ES.effect-boundaries.001 Mutation authority boundaries

```yaml spec-section
id: ES.effect-boundaries.001
spec: enabling-system
kind: enabling.effect_boundaries
title: Mutation authority boundaries
statement_type: duty
claim_layer: work
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.architecture.001
supersedes: []
terms: []
target_refs:
  - ES.effect-boundaries.definition
  - ES.effect-boundaries.admissibility
  - ES.effect-boundaries.duties
  - ES.effect-boundaries.evidence
evidence_required:
  - kind: review
    description: Human confirms that mutation authority remains explicit for specs, decisions, commissions, repository files, external systems, and runtime workspaces.
  - kind: runtime
    description: Kernel commands reject missing approval gates and scope envelopes before authority-bearing mutations.
```

Haft's enabling system separates read, draft, approve, execute, apply, and
publish effects. A surface may present or route a request, but authority-bearing
mutation belongs to the kernel command, CLI operation, or explicit operator
action that closes the relevant gate.

### Effect Boundaries

- Spec carrier drafts may edit `.haft/specs/*.md` after lifecycle identifies
  the target carrier and phase. Active SpecSection baselines require explicit
  approval or rebaseline through `haft_spec_section`.
- SQLite governance state may be mutated only through typed Haft commands such
  as problem, solution, decision, note, commission, refresh, method, and spec
  lifecycle actions. Surfaces must not write raw DB rows as a parallel path.
- DecisionRecords and WorkCommissions require explicit human invocation because
  they bind choice or execution authority.
- Repository code, tests, docs, and changelog files may be edited by the coding
  agent only inside the operator's requested work scope and local permissions.
  Commits require explicit operator request.
- Harness runtime work happens in isolated worktrees or scoped workspaces and
  must pass commission scope, freshness, lockset, and autonomy-envelope checks.
- Applying harness output to the operator checkout is a separate effect from
  executing the work. Auto-apply is allowed only when the commission delivery
  policy and autonomy envelope admit it.
- External effects such as push, pull request creation, external comments,
  tracker writes, email, or calendar changes require explicit operator
  authorization and the corresponding surface/tool contract.

### Read-Only Effects

- `haft_query`, status, coverage, interface discovery, and spec lifecycle
  projection calls are read-only unless they explicitly name a mutation action.
- Markdown carrier text, chat summaries, and generated plans are descriptions.
  They do not become authority until a typed command validates and records the
  corresponding object or baseline.

## ES.agent-policy.001 Host agent authority policy

```yaml spec-section
id: ES.agent-policy.001
spec: enabling-system
kind: enabling.agent_policy
title: Host agent authority policy
statement_type: duty
claim_layer: work
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.effect-boundaries.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that supported agents and human gates match current project policy.
  - kind: runtime
    description: Skill and MCP surfaces keep `h-decide` and `h-commission` manual-only and route validation through the kernel.
```

Haft supports host coding agents as surfaces over the same governance graph, not
as independent authorities. Claude Code and Codex are supported host-agent
surfaces for this project. Cursor, Gemini CLI, and OpenCode are experimental
configuration targets and must not be treated as equally stable execution or
governance surfaces until their runtime contracts are verified.

### Agent Permissions

- Agents may read status, lifecycle projections, interface contracts, code
  context, FPF retrieval results, and artifact graph queries.
- Agents may draft non-binding carriers such as SpecSections, ProblemCards,
  SolutionPortfolios, Notes, MethodRuns, and evidence only through the relevant
  skill or kernel command.
- Agents may edit repository files inside the operator's requested work scope
  and local permissions. Commits, pushes, pull requests, external comments, and
  release actions require explicit operator authorization.
- Agents must use MethodPack before non-trivial code work and close the method
  run with verification evidence before claiming completion.

### Human Gates

- DecisionRecords require explicit human invocation of `/h-decide` or an
  equivalent manual CLI/MCP action.
- WorkCommissions require explicit human invocation of `/h-commission` or an
  equivalent manual CLI/MCP action.
- SpecSection baselines require human review followed by approve or rebaseline.
- Product scope, external promises, security/compliance/legal/finance/privacy
  choices, destructive migrations, and public interface changes remain human
  decisions.

### Agent Duties

- Agents must distinguish description, plan, and work.
- Agents must not present generated rationale as runtime evidence.
- Agents must surface drift, stale evidence, missing baselines, and human gates
  instead of silently bypassing them.
- Agents must not use markdown carriers or chat history as authority until the
  kernel parses, validates, and records the corresponding object.

## ES.commission-policy.001 Bounded commission authorization

```yaml spec-section
id: ES.commission-policy.001
spec: enabling-system
kind: enabling.commission_policy
title: Bounded commission authorization
statement_type: duty
claim_layer: work
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.agent-policy.001
  - ES.effect-boundaries.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that commission creation remains bounded authorization and does not imply execution or apply authority.
  - kind: runtime
    description: WorkCommission creation, claim, drain, apply, requeue, and cancel paths enforce scope, freshness, lockset, lease, evidence, and autonomy-envelope checks.
```

A WorkCommission is bounded execution authorization from an active
DecisionRecord. It describes what work may be attempted, where, under which
tools and time budget, with which evidence requirements, and under which
delivery policy. It is not the RuntimeRun and does not execute by itself.

### Creation Rules

- WorkCommissions require explicit human invocation of `/h-commission` or an
  equivalent manual CLI/MCP action.
- A commission must reference an active DecisionRecord. Stale, superseded,
  deprecated, drifted, or low-evidence decisions require operator review before
  commissioning.
- Default scope derives from the DecisionRecord's affected files and module
  context. Any wider allowed path, explicit forbidden path exception, or
  multi-commission slice requires operator-visible scope text.
- The commission carries `allowed_paths`, `forbidden_paths`, lockset,
  evidence requirements, delivery policy, valid-until, and an autonomy-envelope
  snapshot. Missing scope is not admissible execution authority.
- The default delivery policy is `workspace_patch_manual`. Auto-apply requires
  explicit operator policy and a passing autonomy-envelope result at apply time.

### Execution And Retirement Rules

- Commission creation stops before execution. Harness execution is a separate
  operator-invoked action.
- Runnable intake must reject expired commissions, stale leases beyond the age
  cap, lockset overlap, failed freshness, failed scope checks, and failed or
  checkpoint-required autonomy-envelope results.
- Runtime results terminalize the commission with preserved evidence and an
  inspectable workspace/result carrier.
- Requeue and cancel are explicit lifecycle transitions with audit history;
  neither deletes the commission record.
- Applying a completed workspace patch to the operator checkout is a separate
  local git effect and remains revertable. Remote push, PR, merge, or external
  publication is out of scope for commission auto-apply.

## ES.runtime-policy.001 Harness runtime lifecycle policy

```yaml spec-section
id: ES.runtime-policy.001
spec: enabling-system
kind: enabling.runtime_policy
title: Harness runtime lifecycle policy
statement_type: duty
claim_layer: work
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.commission-policy.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that runtime lifecycle ownership, isolation, and observation rules match current harness behavior.
  - kind: runtime
    description: Harness run/status/result/apply surfaces expose runtime state, isolated workspace results, and operator recovery actions.
```

The harness runtime executes WorkCommissions; it does not create product
authority or decide that work should exist. CLI and desktop/operator surfaces
own starting, observing, stopping, applying, requeueing, and cancelling runtime
work. The MCP plugin may expose typed state transitions and projections, but it
must not become the owner of unattended long-running execution.

### Lifecycle Rules

- `haft harness run` is the normal single-commission runtime start surface.
- `haft harness run --drain --concurrency N` is opt-in batch execution and must
  preserve single-run behavior when drain mode is absent.
- `haft harness status` and result/detail commands expose active, terminal,
  stale, blocked, and recoverable runtime state.
- Operator intervention remains available through apply, requeue, cancel, or
  inspect actions without direct SQLite surgery or process-kill dependency.
- Runtime startup must fail or ask for explicit override when project specs,
  source decisions, commission scope, freshness, lockset, lease, or
  autonomy-envelope gates are not admissible.

### Isolation And Observation Rules

- Each RuntimeRun executes in an isolated workspace or worktree tied to a
  specific WorkCommission.
- Runtime prompts receive the relevant DecisionRecord context, scope, and
  invariants, not unrestricted repository authority.
- A terminal runtime result must preserve enough evidence to inspect what ran,
  what changed, which checks passed or failed, and what apply/recovery action is
  available.
- Applying a runtime result to the operator checkout is separate from executing
  the runtime and remains governed by delivery policy and local git evidence.
- Open-Sleigh is the execution mechanics layer; Haft Core remains the authority
  over commissions, scopes, evidence, and governance state.

## ES.evidence-policy.001 Evidence freshness and verification policy

```yaml spec-section
id: ES.evidence-policy.001
spec: enabling-system
kind: enabling.evidence_policy
title: Evidence freshness and verification policy
statement_type: duty
claim_layer: evidence
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - ES.work-methods.001
  - ES.runtime-policy.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: type
    description: Spec and artifact carriers must parse into canonical typed objects before they can govern work.
  - kind: L1
    description: Deterministic carrier/schema checks must pass before spec readiness or artifact admissibility is claimed.
  - kind: L2
    description: Integration or interface checks must demonstrate that CLI/MCP surfaces expose the intended contract.
  - kind: L3
    description: Runtime or end-to-end evidence is required for claims about actual harness execution, apply behavior, or external side effects.
  - kind: manual
    description: Human review is required for value choices, authority gates, scope expansion, public interface changes, and baselines.
```

Haft evidence policy treats verification as baseline -> measurement -> evidence
-> recorded verdict. A claim is not verified because it is documented; it is
verified only when evidence is attached to the relevant artifact with a verdict,
congruence level, and freshness boundary.

### Evidence Rules

- Unit, parser, schema, and carrier checks are admissible for structural claims
  about typed artifacts and deterministic validators.
- CLI/MCP smoke tests and interface-contract tests are admissible for surface
  contract claims.
- RuntimeRun, harness result, workspace diff, terminal verdict, and apply
  evidence are required for claims about actual autonomous execution.
- Human review evidence is required for target purpose, value trade-offs,
  authority gates, baseline approval, and scope expansion.
- Evidence must carry enough detail to rerun or inspect the observation: command
  or carrier, observed result, verdict, and expiry.
- R_eff is governed by the weakest supported evidence link; weaker or expired
  evidence must not be averaged away by stronger adjacent evidence.

### Freshness Rules

- SpecSections and evidence-bearing artifacts must carry `valid_until` when the
  claim depends on project state, runtime behavior, dependencies, or operator
  policy.
- Expired evidence triggers refresh, waive with new evidence, reopen, or
  supersede. It does not silently remain current.
- Drift in baselined files or SpecSections triggers review before the affected
  claim is reused as authority.
- New runtime behavior, new host-agent surfaces, new public interfaces, or new
  autonomy modes require fresh evidence before they can be used as governance
  assumptions.
