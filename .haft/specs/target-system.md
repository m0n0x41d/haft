# Target System Spec — active v9 carrier

Status: current specification carrier. Lifecycle authority belongs to each
typed `spec-section`: `TS.environment.001`, `TS.role.001`, and
`TS.boundary.001` are active and baselined; `TS.placeholder.001` remains an
explicit draft placeholder.

Only the typed `claims` entries inside each active `spec-section` block are
normative statements. Surrounding prose is an informative traversal of those
claims; it creates no additional law, admissibility gate, duty, evidence
assertion, method, migration authority, or performed Work.

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

This placeholder only reserves a parseable carrier for onboarding. It is not
an active target-system claim. Its historical disposition belongs in the
reviewed migration packet and apply receipt, not in an unparsed `supersedes`
field.

## TS.environment.001 Profile-applicable project environment

```yaml spec-section
id: TS.environment.001
spec: target-system
kind: target.environment
title: Profile-applicable project environment
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
    description: Human confirms that the environment change is scoped by an admitted project-profile declaration produced through the explicitly invoked onboarding path rather than by the repository container.
  - kind: runtime
    description: Software, non-software, mixed, and undetermined fixtures expose the expected applicability, FPF Query, and typed project-memory behavior through CLI or MCP surfaces.
claims:
  - id: TS.environment.001.L1
    class: L
    statement: A project realization scope is identified by a stable ScopeID and explicit profile basis, not by a repository path or file extension.
    scope:
      - project-profile
      - realization-scope
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.environment.001.L5
    class: L
    statement: TargetSystemSpec applicability for a scope is derived from an explicit target relation whose EntityOfConcernSlot is filled by an EntityRef with a KindClassificationJudgement=true for the applicable local system kind under an exact KindSignature edition, bounded context, and context slice.
    scope:
      - target-system-applicability
    governing_pattern_refs:
      - A.6.B
      - A.7
      - C.3.2
  - id: TS.environment.001.L6
    class: L
    statement: SoftwareSystemSpec applicability and software-engineering readiness pressure for a scope are derived from a SoftwareRealization scope whose selected target EntityRef has a KindClassificationJudgement=true for the applicable local software-system kind under an exact KindSignature edition, bounded context, and context slice.
    scope:
      - software-system-applicability
    governing_pattern_refs:
      - A.6.B
      - A.7
      - C.3.2
  - id: TS.environment.001.L2
    class: L
    statement: An Applicability result for one capability and ScopeID is exactly one of Required, NotApplicable, or Underdetermined and carries its profile basis or missing basis.
    scope:
      - capability-applicability
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.environment.001.L3
    class: L
    statement: NotApplicable is a successful applicability result for the excluded capability and is not readiness debt.
    scope:
      - capability-applicability
    governing_pattern_refs:
      - A.6.B
  - id: TS.environment.001.L4
    class: L
    statement: Underdetermined denotes missing applicability basis and is neither a Required nor a NotApplicable result.
    scope:
      - capability-applicability
    governing_pattern_refs:
      - A.6.B
  - id: TS.environment.001.D1
    class: D
    statement: HaftSoftwareSystem, acting in the ProjectGovernanceSubstrate role, must expose source-native FPF retrieval and typed project memory for every admitted project scope.
    scope:
      - fpf-query
      - typed-project-memory
    support_refs:
      - TS.environment.001.L1
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.environment.001.D2
    class: D
    statement: HaftSoftwareSystem, through its applicability-resolution capability allocated to the Haft Core component, must supply the same canonical Applicability result for the selected ScopeID to the init, status, readiness, and execution-preflight adapters.
    scope:
      - applicability-consumers
    support_refs:
      - TS.environment.001.L5
      - TS.environment.001.L6
      - TS.environment.001.L2
    governing_pattern_refs:
      - A.6.B
```

Informatively, the environment claim is scope-specific: a repository path is
not a scope identity, and a target or software specification follows the exact
target relation, context-local kind-classification basis, and Applicability result rather than the mere
presence of a repository (`TS.environment.001.L1`,
`TS.environment.001.L5`, `TS.environment.001.L6`).

The changed environment makes source-native FPF retrieval and typed project
memory available through HaftSoftwareSystem while it holds the
ProjectGovernanceSubstrate role (`TS.environment.001.D1`). These are
capabilities of the role holder, not capabilities owned by the role, additional
roles, or a prescribed sequence of project work.

The three-valued Applicability result explains why an excluded software
carrier is not debt and why missing basis remains visible without becoming a
fabricated classification (`TS.environment.001.L2`,
`TS.environment.001.L3`, `TS.environment.001.L4`). All adapters consume that
same result (`TS.environment.001.D2`).

### Observable Change

- Each `ScopeID` has one profile basis (`TS.environment.001.L1`).
- Each capability and scope has one three-valued result
  (`TS.environment.001.L2`).
- `NotApplicable` is not debt (`TS.environment.001.L3`).
- `Underdetermined` retains missing basis (`TS.environment.001.L4`).
- The HaftSoftwareSystem adapters receive one canonical result
  (`TS.environment.001.D2`).

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
    description: Human confirms the single Haft role holder and context, and confirms that source retrieval, applicability, semantic admission, projection, and execution mechanics remain distinct capabilities or behaviors.
  - kind: runtime
    description: CLI and MCP tests show source provenance, tri-state validation, authority rejection, canonical semantic writes, and separately observable projection state.
claims:
  - id: TS.role.001.L5
    class: L
    statement: HaftSoftwareSystem is the whole shipped target system and holder of the canonical ProjectGovernanceSubstrate role in one profile-selected project BoundedContext; GovernanceSubstrate is only a display alias for that same role.
    scope:
      - haft-software-system
      - project-governance-substrate-role
      - project-bounded-context
    support_refs:
      - TS.environment.001.L1
      - TS.environment.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.L1
    class: L
    statement: HaftSoftwareSystem, while holding the ProjectGovernanceSubstrate role in a project BoundedContext, has the source-native FPF Query capability over pinned README, Preface, Table of Contents, and full pattern source units; the Haft Core component bears the implementation-responsibility allocation for this capability, and that part-whole relation does not assign the whole-system role to the component.
    scope:
      - fpf-query
    support_refs:
      - TS.role.001.L5
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.L2
    class: L
    statement: HaftSoftwareSystem, while holding the ProjectGovernanceSubstrate role in a project BoundedContext, has the typed-project-memory capability over semantic entities, ContextKindAvailability projections, KindClassificationJudgements, optional KindExtension representations, n-ary relations, assertions, authority receipts, and evidence references; the Haft Core component bears the implementation-responsibility allocation for this capability, and that part-whole relation does not assign the whole-system role to the component.
    scope:
      - typed-project-memory
    support_refs:
      - TS.role.001.L5
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.L3
    class: L
    statement: TargetSystemSpec and SoftwareSystemSpec are distinct specification descriptions of the same HaftSoftwareSystem under target-role and software-realization viewpoints respectively; their publication carriers are distinct from those descriptions and from the system and create neither identity nor viewpoint.
    scope:
      - haft-software-system
      - haft-core-component
      - target-system-spec-carrier
      - software-system-spec-carrier
    support_refs:
      - TS.role.001.L5
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.L4
    class: L
    statement: The value of the ProjectGovernanceSubstrate role is that source-grounded orientation and reliance-bearing project memory remain available under explicit type and authority boundaries in its profile-selected project scope.
    scope:
      - project-governance-substrate-role
      - role-value
    support_refs:
      - TS.role.001.L1
      - TS.role.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.A1
    class: A
    statement: An FPF Query result is admissible as retrieved source material but not as pattern applicability, pattern selection, evidence, approval, or work order.
    scope:
      - fpf-query-result
    support_refs:
      - TS.role.001.L1
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.A2
    class: A
    statement: A typed-memory mutation is semantically admissible only under the project-active TypeEnv, current graph revision, declared bounded context, and applicable scope; this semantic predicate neither establishes nor substitutes for any action-specific authority basis required by the mutation's direct contract.
    scope:
      - semantic-admission
    support_refs:
      - TS.role.001.L2
    governing_pattern_refs:
      - A.2.8
      - A.2.9
      - A.6.B
      - A.7
  - id: TS.role.001.D1
    class: D
    statement: HaftSoftwareSystem, through its FPF Query capability allocated to the Haft Core component, must keep exact source identity and provenance kernel-recoverable while returning a bounded default working description with stable source identity, source role, and explicit ambiguity, truncation, or abstention posture. Exact provenance and replay must be addressable through an explicit trace/audit description, and raw retrieval internals only through an explicit diagnostic description.
    scope:
      - fpf-query-provenance
      - fpf-query-working-projection
    support_refs:
      - TS.role.001.A1
    governing_pattern_refs:
      - A.6.B
  - id: TS.role.001.A3
    class: A
    statement: Persistence of a typed-memory ChangeSet is admissible only when semantic validation returns Valid; Invalid and Underdetermined are non-persisting results.
    scope:
      - typed-memory-validation
    support_refs:
      - TS.role.001.A2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.role.001.D3
    class: D
    statement: HaftSoftwareSystem, through its semantic-persistence capability allocated to the Haft Core component, must persist an admitted ChangeSet in the canonical semantic store.
    scope:
      - canonical-memory
    support_refs:
      - TS.role.001.A2
      - TS.role.001.A3
    governing_pattern_refs:
      - A.6.B
      - A.7
```

Informatively, HaftSoftwareSystem is the whole shipped target system and holds one canonical
`ProjectGovernanceSubstrate` role in one profile-selected project scope;
`GovernanceSubstrate` is only its display alias (`TS.role.001.L5`). The
TargetSystemSpec and SoftwareSystemSpec are distinct specification
descriptions/views of that same system under the target-role and
software-realization viewpoints respectively. Their publication carriers are
separately distinct and create neither identity nor viewpoint
(`TS.role.001.L3`). Haft Core remains an internal component whose allocation
does not transfer the whole-system role (`TS.role.001.L1`,
`TS.role.001.L2`). The role value is stated separately from the capabilities
that realize it (`TS.role.001.L4`).

FPF Query is a capability of HaftSoftwareSystem in that context, not a
capability owned or supplied by the role or inherited by a component merely by
parthood. Its result remains
source material rather than applicability, selection, evidence, approval, or
work order
(`TS.role.001.L1`, `TS.role.001.A1`, `TS.role.001.D1`).

Typed project memory is the other capability of HaftSoftwareSystem required by
the role contract. Its canonical values and mutation boundary are stated by
`TS.role.001.L2`,
`TS.role.001.A2`, `TS.role.001.A3`, and `TS.role.001.D3`.

Nothing in the role or its two capabilities defines a universal workflow. This
sentence explains the retrieval-result boundary in `TS.role.001.A1`; it does
not introduce a MethodDescription or report performed Work.

### Role Boundaries

- Holder: HaftSoftwareSystem (`TS.role.001.L5`).
- Context: one profile-selected project scope (`TS.role.001.L5`).
- Role: canonical `ProjectGovernanceSubstrate`; `GovernanceSubstrate` is its
  display alias (`TS.role.001.L5`).
- Role value: source-grounded orientation plus reliance-bearing project memory
  under explicit type and authority boundaries (`TS.role.001.L4`).
- Capabilities of HaftSoftwareSystem required by the role contract:
  source-native FPF
  Query (`TS.role.001.L1`) and typed project memory (`TS.role.001.L2`).
- Software realization and carrier boundary: `TS.role.001.L3`.
- Mutation boundary: `TS.role.001.A2` and `TS.role.001.A3`.

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
  - TS.boundary.001.L1
  - TS.boundary.001.A3
  - TS.boundary.001.D1
  - TS.boundary.001.E1
evidence_required:
  - kind: review
    description: Human confirms the profile, FPF authority, semantic admission, projection, runtime, and operator-authority boundaries.
  - kind: runtime
    description: CLI and MCP surfaces reject inapplicable or ill-typed writes, preserve explicit human gates, and report projection debt without confusing it with semantic rollback.
claims:
  - id: TS.boundary.001.L1
    class: L
    statement: The canonical project-memory write model consists of admitted semantic records; graph, Markdown, status, search, and code-link structures are derived projections or carriers.
    scope:
      - canonical-write-model
      - projections
    support_refs:
      - TS.role.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L2
    class: L
    statement: OpenSleigh is the shipped execution-mechanics runtime of HaftSoftwareSystem and is not the runtime of the product or other EntityOfConcern that a user constructs with HaftSoftwareSystem.
    scope:
      - haft-runtime
      - user-target-runtime
    support_refs:
      - TS.role.001.L3
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L3
    class: L
    statement: A RuntimeRun is a dated occurrence of commissioned Work executed through OpenSleigh; a RuntimeRunRecord is the description of that occurrence; a result carrier carries or represents the RuntimeRunRecord and is distinct from both the record and the Work occurrence.
    scope:
      - runtime-run
      - runtime-run-record
      - performed-work
    support_refs:
      - TS.boundary.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L4
    class: L
    statement: An observation or episteme supplies a possible basis; Evidence is the context-bound use of that basis in a support-or-weakening relation to a claim; an EvidenceRecord describes that evidence-use relation; an evidence carrier carries or represents the EvidenceRecord and is distinct from the record, relation, and supporting basis.
    scope:
      - evidence
      - evidence-record
      - evidence-carrier
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L5
    class: L
    statement: Each authority-gated mutation uses an action-specific authority-basis type named by its direct contract. For one-pass onboarding profile declaration, ProfileDeclarationAuthorityBasis consists of references to the authorizing U.SpeechAct occurrence, its ProfileDeclarationAuthorizationContent utterance description, the ProfileDeclarationPermission U.Commitment instituted by that act under a named context policy, and that policy. The act, content, instituted permission, authority-basis tuple, later ProfileDeclarationReceiptV1, and their carriers are distinct, and no basis for one action kind authorizes another.
    scope:
      - profile-authority
      - capability-applicability
    support_refs:
      - TS.environment.001.L5
      - TS.environment.001.L6
      - TS.environment.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.2.8
      - A.2.9
      - A.7
  - id: TS.boundary.001.A4
    class: A
    statement: A repository-inferred project-profile suggestion is admissible for non-binding orientation only and cannot establish binding capability applicability.
    scope:
      - profile-orientation
      - capability-applicability
    support_refs:
      - TS.boundary.001.L5
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.A5
    class: A
    statement: >-
      An onboarding ProfileDeclarationCandidate is authority-admissible only
      when its ProfileDeclarationAuthorityBasis resolves all of the following
      in one judgement context: a conforming authorizing U.SpeechAct; an
      utterance-referenced ProfileDeclarationAuthorizationContent whose project
      root, action kind, ProfileAuthorRoleAssignment, method-description
      reference, classifier and policy versions, session, allowed Work window,
      basis-observation window, validity window, and single-use key match the
      request; a current ProfileDeclarationPermission U.Commitment instituted
      by that act under the named policy; and a durable
      ProfileOnboardingWorkRecord whose performedBy, executedWithin, Work
      interval, RoleAssignment interval coverage, basis-observation window,
      payload digest, and observed-basis digest satisfy that permission and
      content.
    scope:
      - profile-onboarding-authority
      - profile-admissibility
    support_refs:
      - TS.boundary.001.L5
      - TS.boundary.001.A4
    governing_pattern_refs:
      - A.2.1
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: TS.boundary.001.L6
    class: L
    statement: EntityOfConcernSlot is a SlotKind whose EntityOfConcern filler is a U.Entity represented project-locally by entityOfConcernRef of kind U.EntityRef; BoundedContextRef remains a separate member of DescriptionContext.
    scope:
      - entity-of-concern
      - bounded-context
    support_refs:
      - TS.role.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L8
    class: L
    statement: A project-memory recall key has the form of an entityOfConcernRef and BoundedContextRef pair for retrieval; this key does not redefine EntityOfConcern, make BoundedContextRef part of U.Entity identity, or introduce an EntityOfConcern kind.
    scope:
      - entity-of-concern-recall
      - bounded-context
    support_refs:
      - TS.boundary.001.L6
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.A3
    class: A
    statement: A new semantic assertion is type-admissible only when every required ContextKindAvailability, KindClassificationJudgement=true, relation signature, slot constraint, source reference, and applicability result resolves under the exact project-active TypeEnv; a false or unknown classification remains distinct from the receiving admission refusal, and TypeEnv does not resolve project authorization acts or their records.
    scope:
      - semantic-admission
    support_refs:
      - TS.role.001.A2
      - TS.boundary.001.L5
      - TS.boundary.001.L6
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.A6
    class: A
    statement: A project-state mutation classified as authority-gated by its direct contract is authority-admissible only when the common authority verifier returns Admitted for that contract's exact action-specific authority basis and the same action kind, project scope, canonical request-payload digest, judgement context, and validity window. For profile declaration the authorization content's singleUseKey is unused in the canonical admission ledger at judgement time; semantic or type validity and an authority basis admitted for another action kind do not satisfy this predicate.
    scope:
      - authority-admission
      - semantic-mutation
    support_refs:
      - TS.boundary.001.L5
      - TS.boundary.001.A5
    governing_pattern_refs:
      - A.2.1
      - A.2.8
      - A.2.9
      - A.3.1
      - A.3.2
      - A.6.B
      - A.7
      - A.15.1
  - id: TS.boundary.001.D1
    class: D
    statement: HaftSoftwareSystem, through its semantic-validation capability allocated to the Haft Core component, must return Underdetermined with exact missing-basis diagnostics when required type or applicability basis is absent.
    scope:
      - validation-result
    support_refs:
      - TS.boundary.001.A3
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.L7
    class: L
    statement: Failure of an asynchronous carrier or read-projection update leaves a committed canonical semantic record unchanged and creates durable projection debt.
    scope:
      - projection-debt
      - semantic-commit
    support_refs:
      - TS.boundary.001.L1
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: TS.boundary.001.D3
    class: D
    statement: HaftSoftwareSystem, through its authority-gate capability allocated to the Haft Core component, must reject a project-profile declaration unless both TS.boundary.001.A5 and TS.boundary.001.A6 are satisfied; generated rationale, model-supplied references, schema-visible fields, tool possession, carriers, and shape-valid receipts satisfy neither predicate.
    scope:
      - human-authority
    support_refs:
      - TS.boundary.001.L5
      - TS.boundary.001.A5
      - TS.boundary.001.A6
    governing_pattern_refs:
      - A.6.B
      - A.2.9
      - A.7
  - id: TS.boundary.001.E1
    class: E
    statement: Under exact-carrier semantic review, completed migration-receipt inspection, and active-section review on 2026-07-15, the law, admissibility, and deontic boundary claims selected by TS.boundary.001.target_refs were found internally coherent for the human specification reviewer in the boundary-contract viewpoint; this observation is not runtime evidence that the future typed-memory implementation already realizes those claims.
    scope:
      - boundary-contract-evidence
    support_refs:
      - TS.boundary.001.L1
      - TS.boundary.001.A3
      - TS.boundary.001.D1
    evidence_refs:
      - "carrier:.context/haft-v9-migration-v2/semantic-review.m1-final-candidate.md"
      - "carrier:.haft/spec-migration-v2.b3887ad7825d964872fa4fdcefa8c68c8f8ab25fec4b4191c38100f8c73272a3.receipt.json"
      - "command:haft spec review --json"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

Informatively, HaftSoftwareSystem's boundary contains its Query and typed-memory
capabilities (`TS.role.001.L1`, `TS.role.001.L2`) plus their semantic and
authority gates (`TS.boundary.001.A3`, `TS.boundary.001.A5`,
`TS.boundary.001.D1`, `TS.boundary.001.D3`). The explicit admitted profile
defines binding applicability. During explicitly invoked onboarding Work an
acting host agent may classify the repository basis and propose declaration
data; HaftSoftwareSystem admits and materializes the declaration only after the
authorizing U.SpeechAct, its utterance content, the permission it instituted,
and the action-specific authority basis resolve. Raw
repository inference remains orientation only (`TS.boundary.001.L5`,
`TS.boundary.001.A4`, `TS.boundary.001.A5`, `TS.boundary.001.D3`).

The canonical/projection distinction and projection-failure invariant are
fully stated by `TS.boundary.001.L1` and `TS.boundary.001.L7`. This prose adds
no alternate persistence or recovery requirement.

OpenSleigh belongs to the shipped HaftSoftwareSystem boundary while remaining
distinct from the user's target runtime (`TS.boundary.001.L2`). The dated Work
occurrence, its RuntimeRunRecord description, and the result carrier that
carries that record remain distinct. The observation or episteme basis, the
contextual Evidence relation, its EvidenceRecord description, and the evidence
carrier that carries that record likewise remain distinct (`TS.boundary.001.L3`,
`TS.boundary.001.L4`).

The remaining boundary explanation follows from the Query-result restriction
and explicit authority gate (`TS.role.001.A1`, `TS.boundary.001.D3`):
HaftSoftwareSystem does not become FPF authority, the human principal, or the
runtime of the user's target merely by carrying descriptions or operating
OpenSleigh.

### Boundary Perspectives

- Canonical versus projected values: `TS.boundary.001.L1`.
- OpenSleigh versus the user's target runtime: `TS.boundary.001.L2`.
- Work occurrence versus RuntimeRunRecord versus result carrier:
  `TS.boundary.001.L3`.
- Observation or episteme basis versus contextual Evidence relation versus
  EvidenceRecord versus evidence carrier: `TS.boundary.001.L4`.
- Profile and orientation boundary: `TS.boundary.001.L5` and
  `TS.boundary.001.A4`.
- Semantic and authority gates: `TS.boundary.001.A3`,
  `TS.boundary.001.D1`, and `TS.boundary.001.D3`.

### Exclusions

- Repository inference cannot bind applicability (`TS.boundary.001.A4`).
- `NotApplicable` is not debt (`TS.environment.001.L3`).
- `Underdetermined` preserves missing basis (`TS.environment.001.L4`,
  `TS.boundary.001.D1`).
- Query output cannot become selection or approval (`TS.role.001.A1`).
- `EntityOfConcernSlot`, its `U.Entity` filler, `entityOfConcernRef`, and the
  separate `BoundedContextRef` retain their declared types
  (`TS.boundary.001.L6`, `TS.boundary.001.L8`).
- A carrier or projection is not the canonical semantic record
  (`TS.boundary.001.L1`).
- Binding mutations require operator authority (`TS.boundary.001.D3`).
- A RuntimeRunRecord is not performed Work, and a result carrier is neither
  that record nor Work. An EvidenceRecord describes the contextual
  evidence-use relation, while an evidence carrier is distinct from that
  record, the Evidence relation, and its observation or episteme basis
  (`TS.boundary.001.L3`, `TS.boundary.001.L4`).
