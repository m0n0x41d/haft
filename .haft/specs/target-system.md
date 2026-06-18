# Target System Spec

## TS.placeholder.001 Target system placeholder

```yaml spec-section
id: TS.placeholder.001
kind: environment-change
title: Target system placeholder
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

This placeholder only reserves a parseable carrier for onboarding. It is not an active target-system claim.

## TS.environment.001 Harnessable repository environment

```yaml spec-section
id: TS.environment.001
spec: target-system
kind: target.environment
title: Harnessable repository environment
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-09-18
depends_on: []
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that this environment-change statement still matches Haft's product intent.
  - kind: runtime
    description: A repository using Haft exposes parseable specs, decisions, commissions, and evidence through CLI or MCP surfaces.
```

Haft changes a software repository from an ad hoc agent workspace into a
principal-led FPF work environment with parseable governance objects. After the
target system is in use, the repository has a `.haft/` artifact graph containing
target and enabling spec carriers, a term map, problem frames, decisions,
bounded commissions, baselines, and evidence records that humans and agents can
query before and after work.

### Observable Change

- A project can run deterministic spec checks over `.haft/specs/*.md` carriers.
- A project can expose governance state through CLI and MCP surfaces.
- A project can link implementation work to DecisionRecords, WorkCommissions,
  baselines, and evidence instead of relying on chat history or unstructured
  markdown.

## TS.role.001 Governance substrate for admissible work

```yaml spec-section
id: TS.role.001
spec: target-system
kind: target.role
title: Governance substrate for admissible work
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - TS.environment.001
supersedes: []
terms: []
target_refs: []
evidence_required:
  - kind: review
    description: Human confirms that the role statement separates Haft's assigned role from host-agent capability and runtime work.
```

Haft's target-system role is governance substrate: it makes proposed software
work admissible for principal-led execution by preserving problem frames,
comparisons, binding decisions, bounded commissions, baselines, and evidence in
a queryable project graph.

Haft is not the coding agent and not the product under construction in an
onboarded repository. Host agents and humans do the work; Haft supplies the
typed governance boundary that says which work is framed, decided, bounded,
verified, stale, or blocked.

### Role Boundaries

- Role: governance substrate for admissible work.
- Capability: expose typed CLI/MCP/skill surfaces over the artifact graph.
- Method: require explicit frames, decisions, scopes, baselines, and evidence.
- Work: parse carriers, validate commands, record artifacts, report status, and
  surface drift or stale claims.

## TS.boundary.001 Governance boundary of Haft

```yaml spec-section
id: TS.boundary.001
spec: target-system
kind: target.boundary
title: Governance boundary of Haft
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-09-18
depends_on:
  - TS.role.001
supersedes: []
terms: []
target_refs:
  - TS.boundary.law
  - TS.boundary.admissibility
  - TS.boundary.duties
  - TS.boundary.evidence
evidence_required:
  - kind: review
    description: Human confirms that the boundary does not assign product, agent, CI, legal, or human-value authority to Haft.
  - kind: runtime
    description: CLI and MCP surfaces reject missing required governance fields and expose drift, stale evidence, and human gates.
```

Haft is in scope as the governance substrate over project-local FPF artifacts:
spec sections, term maps, problem frames, solution portfolios, DecisionRecords,
WorkCommissions, baselines, evidence, refresh reports, and the CLI/MCP/skill
surfaces that create, validate, query, and measure those objects.

Haft is out of scope as a coding agent, CI runner, issue tracker, legal
authority, product runtime, product manager, or replacement for the human
principal. Haft may expose that work is framed, decided, bounded, verified,
stale, drifted, or blocked; it must not silently make external promises,
product-scope choices, security/compliance judgments, or value trade-offs.

### Boundary Perspectives

- Boundary definition: project-local Haft specs, artifacts, workflow policy,
  CLI/MCP schemas, and kernel validators define what Haft governs.
- Admitted actors and objects: the human principal, host coding agents using
  Haft tools, CLI users, MCP clients, and parseable artifact carriers.
- Duties: the kernel validates required fields and authority gates; skills route
  the work method; the human principal approves binding choices and baselines.
- Evidence: `haft spec check`, `haft_query(action="status")`,
  `haft_refresh(action="scan")`, and DecisionRecord evidence/baseline commands
  show whether the boundary is visible and enforced.

### Exclusions

- Haft does not write product code by itself.
- Haft does not approve specs, decisions, commissions, or baselines without an
  explicit operator gate.
- Haft does not treat markdown text as authority until the kernel parses and
  validates the carrier.
- Haft does not replace repository tests, CI, runtime monitoring, or human
  review; it records and routes their evidence.
