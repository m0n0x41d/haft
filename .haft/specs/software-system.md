# Haft Software System Spec — active v9 carrier

This carrier contains the active, baselined SoftwareSystemSpec sections for
the admitted Haft software-project profile. It is a specification description,
not a DecisionRecord, performed Work, migration receipt, or claim that the
described behavior is already implemented. Normative content is carried only
by the typed claims inside each active `yaml spec-section` block.

## SS.role.001 FPF-aware project-memory software role

```yaml spec-section
id: SS.role.001
spec: software-system
system_frame: software_system
kind: software.role
title: FPF-aware project-memory software role
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on: []
target_refs:
  - TS.role.001
  - TS.boundary.001
claims:
  - id: SS.role.001.L1
    class: L
    statement: HaftSoftwareSystem is the single whole shipped target system; TargetSystemSpec and SoftwareSystemSpec are distinct specification views of that same system, and their carriers are not additional system identities. In the HaftProject bounded context for one configured project scope, HaftSoftwareSystem fills the holder slot of the ProjectGovernanceSubstrate role assignment; Haft Core is an internal component rather than a target-system identity or role holder.
    scope:
      - haft-software-role
  - id: SS.role.001.L3
    class: L
    statement: While HaftSoftwareSystem holds the ProjectGovernanceSubstrate role, source-grounded FPF retrieval and reliance-bearing typed project memory are capabilities realized by HaftSoftwareSystem in that context; the ProjectGovernanceSubstrate role contract contains those capability qualification conditions, while neither the role nor the role assignment owns, supplies, or performs them.
    scope:
      - haft-software-capabilities
  - id: SS.role.001.A1
    class: A
    statement: A retrieval result is admissible as FPF source material only when its source unit, revision, content hash, and location provenance resolve to the pinned FPF publication.
    scope:
      - fpf-source-delivery
    support_refs:
      - TS.role.001.L1
      - TS.role.001.A1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.1"
    governing_pattern_refs:
      - E.11.PUA
      - E.11.PUR
  - id: SS.role.001.L2
    class: L
    statement: FPF source is authoritative for FPF meaning; retrieval rank, graph order, source order, and timestamps do not constitute pattern selection, causality, chronology, or project-work order.
    scope:
      - authority-boundary
```

## SS.allocation.001 Product responsibility allocation

```yaml spec-section
id: SS.allocation.001
spec: software-system
system_frame: software_system
kind: software.responsibility_allocation
title: Product responsibility allocation
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.role.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.allocation.001.L1
    class: L
    statement: The human principal owns value choices, the exact requested effect, project-profile declaration, selection of incompatible project-memory schema transitions and rollbacks, binding decisions, WorkCommission authorization, and specification lifecycle gates. A host may route only the principal's direct unambiguous request and must not model hidden mental intent. Haft Core owns both the fixed package-default Genesis memory basis and automatic activation of every exact proven-compatible bundled ProjectTypeEnv successor; neither exposes an operator schema choice.
    scope:
      - human-principal
  - id: SS.allocation.001.L2
    class: L
    statement: The host caller owns the current question, candidate construction, FPF pattern applicability judgement, recognition of whether current Work supplies a concrete durability-requiring receiving use, and routing of direct operator requests to their exact effect-specific interfaces. The receiving use may be operator-named or agent-inferred; when exact stable identity is recoverable, its non-binding EntityOfConcern establishment requires no separate operator permission. Host routing records provenance but creates no authority or U.SpeechAct proof.
    scope:
      - host-caller
  - id: SS.allocation.001.L3
    class: L
    statement: HaftSoftwareSystem owns deterministic parsing, validation, authority checks, semantic admission, canonical persistence, and read-only projections of admitted project state.
    scope:
      - haft-core
  - id: SS.allocation.001.L4
    class: L
    statement: OpenSleigh performs the dated runtime Work admitted by a WorkCommission and emits runtime status observations; CommissionService records the corresponding RuntimeRun lifecycle result. Neither component owns human commission authority or semantic admission.
    scope:
      - opensleigh-runtime
  - id: SS.allocation.001.L5
    class: L
    statement: The FPF Source Query component owns source-unit indexing, retrieval, exact hydration, and retrieval provenance; it does not own pattern applicability or selection.
    scope:
      - fpf-source-query
  - id: SS.allocation.001.L6
    class: L
    statement: The TypeEnv Compiler owns deterministic source compilation and coverage reporting for immutable FPF base artifacts B and symbolic Local-Practice extension artifacts E, canonical linkage of the exact E dependency DAG, binding of an exact runtime evaluation basis X, derivation and verification of composite artifact C, and final C-bound lowering. A separate TypeEnv Stage service owns read-only compatibility, profile-fit, and existing-assertion revalidation against one exact current snapshot. Compilation, linkage, mechanism registration, composite derivation, lowering, diff, revalidation, and staging neither admit instance memory nor select a project head; a registered codec, evaluator, or carrier-membership mechanism contributes only to X and does not admit its ValueKind, ValueShape, CodecRef, relation signature, E artifact, or C artifact.
    scope:
      - typeenv-compiler
  - id: SS.allocation.001.L7
    class: L
    statement: AdmissionService owns pure semantic validation, authority dispatch, transactional revalidation, and canonical semantic commit. Through a disjoint TypeEnv-head operation, it also owns the authorized atomic compare-and-swap that selects one already-derived C as ProjectTypeEnvHead and records the exact use, effect, and receipt; generic MemoryChangeSet admission cannot invoke that operation.
    scope:
      - admission-service
  - id: SS.allocation.001.L8
    class: L
    statement: ProjectionService owns read-only neighborhood, recall, status, coverage, interface-discovery, and carrier projections; it does not own canonical semantic writes.
    scope:
      - projection-service
  - id: SS.allocation.001.L9
    class: L
    statement: SpecLifecycleService owns SpecSection parsing, structural checks, lifecycle projections, and baseline recording after an explicit human gate.
    scope:
      - spec-lifecycle-service
  - id: SS.allocation.001.L10
    class: L
    statement: CommissionService owns WorkCommission validation and audited lifecycle transitions; it does not own human authorization or OpenSleigh execution.
    scope:
      - commission-service
  - id: SS.allocation.001.L11
    class: L
    statement: Host adapters own transport and presentation of kernel contracts; they do not own semantic validation, persistence, or binding authority.
    scope:
      - host-adapters
  - id: SS.allocation.001.L12
    class: L
    statement: EvidenceService owns recording contextual Evidence relations, their EvidenceRecords and target-claim attachments, and currentness and refresh diagnostics; it does not own the Work that produces observations or supporting epistemes, nor the observation activity that supplies their possible basis.
    scope:
      - evidence-service
  - id: SS.allocation.001.L13
    class: L
    statement: ProfileResolver owns configured-profile decoding, read-only suggestions, and per-capability applicability calculation; it does not own profile declaration authority.
    scope:
      - profile-resolver
  - id: SS.allocation.001.L14
    class: L
    statement: ProfileAuthorRole@OnboardingContext is the U.Role value used to attribute bounded profile-classification Work. An exact U.RoleAssignment assigns that role to a host-agent U.System in the onboarding context and is an input to Work-admission and attribution; it neither performs nor proves Work and grants no profile-declaration authority. A dated ProfileOnboardingWork occurrence names that host-agent U.System as its actual performer, names the exact covering assignment, relates the Work to that assignment through performedUnderAssignment, and names its containing system through executedWithin; it produces only a ProfileClassificationResult. HaftSoftwareSystem, through Haft Core and AdmissionService, separately validates and persists an admitted profile declaration.
    scope:
      - profile-onboarding
      - host-agent
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L2
      - SS.allocation.001.L3
      - SS.allocation.001.L7
      - SS.allocation.001.L13
    governing_pattern_refs:
      - A.2.1
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: SS.allocation.001.D1
    class: D
    statement: HaftSoftwareSystem must reject generated prose, quotations, pasted third-party text, an agent proposal or recommendation, tool output, model-supplied references, schema visibility, possession of a skill or tool call, a carrier, and a shape-valid ProfileDeclarationReceiptV1 as substitutes for an exact host-routed operator request or the disjoint automatic singleton basis required by the profile-declaration contract.
    scope:
      - authority-boundary
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L3
      - TS.boundary.001.A6
      - "legacy:ES.agent-policy.001"
      - "legacy:ES.effect-boundaries.001"
    governing_pattern_refs:
      - A.2.8
      - A.2.9
      - A.7
      - A.15
```

## SS.functional.profile.001 Project profile and capability applicability

```yaml spec-section
id: SS.functional.profile.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: Project profile and capability applicability
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.role.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.functional.profile.001.L1
    class: L
    statement: A ConfiguredProjectProfile is either Auto or Declared with one ProfileDeclarationPayload containing a non-empty set of stable realization scopes, one matching ProfileDeclarationReceiptV1, and exactly one ProfileAdmissionOrigin of detector_default, host_routed_operator_request, or the readable legacy provenance explicit_operator or legacy_unknown; the receipt and origin are admission provenance and not declaration authority.
    scope:
      - project-profile
  - id: SS.functional.profile.001.L2
    class: L
    statement: Capability applicability is a tri-state result of Required, NotApplicable, or Underdetermined with an explicit basis for the named capability and scope set.
    scope:
      - capability-applicability
  - id: SS.functional.profile.001.L3
    class: L
    statement: TargetSystemSpec resolves to Required for every scope in an integrity-valid admitted Declared profile regardless of whether its optional EntityRef is present. SoftwareSystemSpec migration and software-specific readiness resolve to Required for a relevant SoftwareRealization scope and NotApplicable for a relevant NonSoftwareRealization scope; either capability is Underdetermined only when no integrity-valid admitted Declared basis exists. ProfileDeclarationReceiptV1 records the admission-time authority and Work lineage but is neither current permission nor authority by itself.
    scope:
      - target-system-spec
      - software-system-spec
      - readiness
    support_refs:
      - SS.functional.profile.001.L1
      - SS.functional.profile.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#7.2.2"
    governing_pattern_refs:
      - A.1
      - A.7
  - id: SS.functional.profile.001.D1
    class: D
    statement: ProfileResolver must keep a provenance-bearing repository suggestion separate from a canonical profile and must not use the suggestion as binding applicability authority outside the exact automatic_supported_singleton_init admission contract.
    scope:
      - profile-inference
    support_refs:
      - SS.allocation.001.L13
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
      - TS.boundary.001.A4
    governing_pattern_refs:
      - A.1
      - A.7
  - id: SS.functional.profile.001.L5
    class: L
    statement: InitialProfileBootstrapDecision is a pure closed decision over existing-profile presence, review origin, detector completeness, confidence, and scope cardinality; only ApplySupportedSingleton carries a scope and admission payload, while KeepExisting and HumanReviewRequired carry no profile mutation.
    scope:
      - initial-profile-bootstrap
      - profile-inference
    support_refs:
      - SS.functional.profile.001.L1
      - SS.functional.profile.001.L2
      - TS.environment.001.L8
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.functional.profile.001.L4
    class: L
    statement: A mixed project is represented by separate stable scopes; repository paths and file extensions are heuristic repository-classification signals that neither constitute a typed Evidence relation by themselves nor define scope identity.
    scope:
      - mixed-projects
  - id: SS.functional.profile.001.E1
    class: E
    statement: Under design-contract inspection on 2026-07-14, the profile algebra and applicability signature carried by master-plan section 4.4 support SS.functional.profile.001.L1 through L4 for the human specification reviewer in the contract-coherence viewpoint; this observation is not an admitted supporting episteme for an Evidence relation about runtime implementation.
    scope:
      - profile-design-evidence
    support_refs:
      - SS.functional.profile.001.L1
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - SS.functional.profile.001.L4
    evidence_refs:
      - "carrier:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.interfaces.profile.001 Project profile interfaces

```yaml spec-section
id: SS.interfaces.profile.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: Project profile interfaces
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.profile.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.interfaces.profile.001.L1
    class: L
    statement: The dedicated `.haft/project-profile.yaml` configuration-card carrier is a human-readable projection of ConfiguredProjectProfile with its exact canonical ledger revision, ProfileDeclarationReceiptV1, and ProfileAdmissionOrigin; absence of both a canonical admitted profile and this carrier decodes as Auto for backward compatibility, while absence or drift of only the projection creates projection debt.
    scope:
      - project-config
  - id: SS.interfaces.profile.001.D1
    class: D
    statement: ProfileResolver must resolve the canonical ConfiguredProjectProfile, ProfileDeclarationAdmissionRecord, ProfileOnboardingWorkRecord, path-specific AuthorityResolutionRecord, exact AuthorityUseRecord, and ProfileAdmissionOrigin from the kernel-owned ProfileAdmissionLedger. Host-routed operator application preserves its authorization-content singleUseKey invariants; automatic bootstrap preserves its detector-policy-observation and deterministic-resolution invariants. YAML bytes or a shape-valid receipt cannot substitute for those canonical records.
    scope:
      - project-config
    support_refs:
      - SS.allocation.001.L13
      - SS.interfaces.profile.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#P0PA"
    governing_pattern_refs:
      - A.7
  - id: SS.interfaces.profile.001.D2
    class: D
    statement: ProjectionService must expose each Applicability result with the exact ConfiguredProjectProfile ledger revision, profile basis, and ProfileAdmissionOrigin; it must return Underdetermined for Auto or for a Declared profile whose payload, receipt, path-specific authority resolution and use, durable Work record, admission record, committed-result back-reference, origin, or revision linkage does not resolve, and it must not infer, refresh, or replay declaration authority from a repository suggestion or a shape-valid receipt. An eligible legacy project remains honestly Underdetermined until haft init performs the automatic bootstrap and exposes haft init --core-only as recovery.
    scope:
      - status
      - readiness
    support_refs:
      - SS.allocation.001.L8
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - TS.boundary.001.A4
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
    governing_pattern_refs:
      - A.7
  - id: SS.interfaces.profile.001.A1
    class: A
    statement: Suppression of repetitive missing-SoftwareSystemSpec reminders in compact orientation is admissible only for a high-confidence, non-conflicting non-software repository suggestion for the current scope.
    scope:
      - status-orientation
    support_refs:
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - TS.boundary.001.A4
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
    governing_pattern_refs:
      - A.1
      - A.7
  - id: SS.interfaces.profile.001.D3
    class: D
    statement: ProjectionService must enforce SS.interfaces.profile.001.A1 when rendering compact orientation, retain one neutral profile-confirmation cue whenever suppression occurs, leave binding applicability, readiness, and specification lifecycle state unchanged, and otherwise retain the ordinary reminder posture.
    scope:
      - status-orientation
    support_refs:
      - SS.allocation.001.L8
      - SS.interfaces.profile.001.A1
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.4"
    governing_pattern_refs:
      - A.7
  - id: SS.interfaces.profile.001.D4
    class: D
    statement: HaftSoftwareSystem must open the canonical profile-admission ledger relative to the supplied project root when the server starts, resolve its admitted profile revision and basis, and read `.haft/project-profile.yaml` as the matching human-readable projection rather than as an independent authority source.
    scope:
      - project-config
      - server-startup
    support_refs:
      - SS.allocation.001.L3
      - SS.allocation.001.L13
      - SS.interfaces.profile.001.L1
      - SS.interfaces.profile.001.D1
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.interfaces.profile.001.D5
    class: D
    statement: ProjectionService must write or rebuild `.haft/project-profile.yaml` from a committed canonical profile revision and reread the exact bytes; projection failure leaves the canonical admission unchanged and records durable projection debt.
    scope:
      - project-config
      - projection-debt
    support_refs:
      - SS.interfaces.profile.001.L1
      - SS.interfaces.profile.001.D1
      - TS.boundary.001.L7
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.interfaces.profile.001.D6
    class: D
    statement: haft_method must select the sole scope of a singleton canonical profile even when a caller passes a task, thread, commission, Work, or other non-scope identifier as scope_id, explicitly diagnose that unnecessary selector as ignored, require one exact scope_id only after scope_choice_required for multi-scope profiles, and create no MethodRun when the selected scope makes the MethodPack NotApplicable.
    scope:
      - methodpack
      - scope-selection
    support_refs:
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - SS.interfaces.profile.001.D2
    governing_pattern_refs:
      - A.6.B
      - A.7
```

## SS.procedural.profile-onboarding.001 Profile declaration during onboarding

```yaml spec-section
id: SS.procedural.profile-onboarding.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: Profile declaration during onboarding
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.functional.profile.001
  - SS.interfaces.profile.001
target_refs:
  - TS.environment.001
  - TS.boundary.001
claims:
  - id: SS.procedural.profile-onboarding.001.L1
    class: L
    statement: ProfileOnboardingMethod is the U.Method for inspecting supplied project basis and producing a ProfileClassificationResult; ProfileOnboardingMethodDescription is the separate U.MethodDescription that describes its inputs, constraints, and Candidate-or-Underdetermined result union, and is not the method, authorization, performed Work, Work record, or persistence.
    scope:
      - profile-onboarding
      - method-description
    support_refs:
      - SS.allocation.001.L14
      - SS.functional.profile.001.L1
      - SS.functional.profile.001.L2
    governing_pattern_refs:
      - A.2.1
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: SS.procedural.profile-onboarding.001.L2
    class: L
    statement: Each execution is a separate dated U.Work occurrence with enactsMethod=ProfileOnboardingMethod, methodDescriptionRef=ProfileOnboardingMethodDescription, canonical concrete parameter bindings for that MethodDescription edition, actualPerformerSystem=the host-agent U.System, coveringRoleAssignment=the exact ProfileAuthorRoleAssignment, performedUnderAssignment linking that Work to the covering assignment, executedWithin naming the containing U.System, an explicit Work interval, onboarding bounded context, explicit inputs, outputs, resources, affected candidate-classification episteme, StatePlaneRef with pre/post references or a declared delta predicate, and outcome. A separate durable ProfileOnboardingWorkRecord describes that occurrence by WorkRef. Its basisObservationWindow is distinct from the Work interval, and its outcome is only CandidatePayloadProduced{payloadDigest, observedBasisDigest} or ClassificationUnderdetermined{missingBasisDigest}; it does not embed CandidateProvenance or a declaration-admission result.
    scope:
      - profile-onboarding
      - performed-work
    support_refs:
      - SS.allocation.001.L14
      - SS.procedural.profile-onboarding.001.L1
    governing_pattern_refs:
      - A.2.1
      - A.2.8
      - A.2.9
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: SS.procedural.profile-onboarding.001.L3
    class: L
    statement: ProfileClassificationResult is exactly Candidate carrying one ProfileDeclarationCandidate, or Underdetermined carrying a durable ProfileOnboardingWorkRecordRef and a non-empty missing-basis set. ProfileDeclarationCandidate contains one ProfileDeclarationPayload and one CandidateProvenance; the result does not duplicate CandidateProvenance and contains no profile-admission state.
    scope:
      - profile-onboarding
      - onboarding-result
    support_refs:
      - SS.functional.profile.001.L1
      - SS.functional.profile.001.L2
      - SS.procedural.profile-onboarding.001.L1
      - SS.procedural.profile-onboarding.001.L2
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.procedural.profile-onboarding.001.L4
    class: L
    statement: ProfileDeclarationPayload contains only the proposed stable RealizationScopes. Its canonical payload digest excludes CandidateProvenance, authority and Work references, observed basis, ProfileDeclarationReceiptV1, ledger revision, and projection digest.
    scope:
      - profile-onboarding
      - candidate-payload
    support_refs:
      - SS.functional.profile.001.L1
      - SS.procedural.profile-onboarding.001.L3
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.procedural.profile-onboarding.001.L5
    class: L
    statement: ProfileDeclarationAdmissionResult is exactly Admitted with the Declared ConfiguredProjectProfile, ProfileDeclarationReceiptV1, canonical ProfileDeclarationAdmissionRecordRef and exact ledger revision; NotAdmitted with typed violated or missing basis and no state mutation; or WriteFailed with an effect-failure reference and no admitted profile. ProfileDeclarationAdmissionRecord is the durable admission proof; no separate untyped admission-proof reference exists.
    scope:
      - profile-admission
      - admission-result
    support_refs:
      - SS.procedural.profile-onboarding.001.L2
      - SS.procedural.profile-onboarding.001.L3
      - SS.procedural.profile-onboarding.001.L4
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.procedural.profile-onboarding.001.A1
    class: A
    statement: A Candidate is admissible for profile declaration only when its payload is non-empty and non-contradictory; its payload digest equals canonical ProfileDeclarationPayload; its CandidateProvenance and observed-basis digests validate; its WorkRecordRef resolves to the Work described by SS.procedural.profile-onboarding.001.L2 with matching outcome digests and RoleAssignment interval coverage; and exactly one disjoint provenance branch is admitted under TS.boundary.001.A6 together with TS.boundary.001.A5 for a host-routed direct operator request or TS.boundary.001.A7 for automatic initial singleton bootstrap.
    scope:
      - profile-onboarding
      - profile-admissibility
    support_refs:
      - SS.procedural.profile-onboarding.001.L1
      - SS.procedural.profile-onboarding.001.L2
      - SS.procedural.profile-onboarding.001.L3
      - SS.procedural.profile-onboarding.001.L4
      - SS.procedural.profile-onboarding.001.L5
      - SS.functional.profile.001.L2
      - SS.functional.profile.001.L3
      - TS.boundary.001.L5
      - TS.boundary.001.A4
      - TS.boundary.001.A5
      - TS.boundary.001.A6
      - TS.boundary.001.A7
    governing_pattern_refs:
      - A.6.B
      - A.2.1
      - A.2.8
      - A.2.9
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: SS.procedural.profile-onboarding.001.D3
    class: D
    statement: HaftSoftwareSystem, through a Haft Core Work-record adapter, must validate the complete SS.procedural.profile-onboarding.001.L2 Work shape—including exact method anchors, canonical concrete parameter bindings, explicit inputs, outputs, resources and affected referent, actual performer system, covering RoleAssignment, performedUnderAssignment, executedWithin, assignment interval coverage, StatePlane pre/post references or declared delta predicate, and outcome digests—then durably persist and reread the immutable ProfileOnboardingWorkRecord before exposing ProfileClassificationResult.Candidate. After the Work record is reread, the current canonical profile-ledger revision becomes the expected admission revision; a validation, persistence, or reread failure returns an effect failure and exposes no Candidate.
    scope:
      - profile-onboarding
      - work-record
    support_refs:
      - SS.procedural.profile-onboarding.001.L2
      - SS.procedural.profile-onboarding.001.L3
    governing_pattern_refs:
      - A.3.1
      - A.3.2
      - A.7
      - A.15.1
  - id: SS.procedural.profile-onboarding.001.D1
    class: D
    statement: HaftSoftwareSystem must have the host adapter supply one exact HostRoutedOperatorRequest binding the profile effect, reviewed candidate, payload digest and project scope without asserting hidden intent or independently proven U.SpeechAct; have Haft Core seal and persist the exact host-routed basis and resolution; have AdmissionService verify that request, payload, CandidateProvenance, durable ProfileOnboardingWorkRecord, project binding, and expected ledger revision; and keep pure profile preparation non-binding. Inside one canonical ProfileAdmissionLedger transaction AdmissionService must revalidate those exact coordinates, compare the expected revision, require the single-use key to be unused, derive the new revision, finalize ProfileDeclarationReceiptV1 and the Declared ConfiguredProjectProfile, install the admission record, and persist exactly one authority-use record pointing to the committed result. The automatic supported-singleton branch remains disjoint and records detector_default. Only a committed branch returns Admitted; `.haft/project-profile.yaml` is projected afterward under SS.interfaces.profile.001.D5.
    scope:
      - profile-onboarding
      - project-config
    support_refs:
      - SS.allocation.001.L3
      - SS.allocation.001.L7
      - SS.allocation.001.L11
      - SS.allocation.001.L13
      - SS.procedural.profile-onboarding.001.A1
      - SS.interfaces.profile.001.L1
      - SS.interfaces.profile.001.D1
    governing_pattern_refs:
      - A.6.B
      - A.2.8
      - A.2.9
      - A.7
      - A.15.1
  - id: SS.procedural.profile-onboarding.001.D2
    class: D
    statement: When classification is mixed or insufficient, confidence is unsupported, the observation is truncated, scope cardinality is not one, or a human-authored or foreign review exists, HaftSoftwareSystem must return HumanReviewRequired and must not invoke automatic admission; any existing ConfiguredProjectProfile revision remains unchanged. Failed candidate validation, path-specific authority resolution, transaction-time validation, expected-revision comparison, or uniqueness constraint returns ProfileDeclarationAdmissionResult.NotAdmitted with an exact typed reason and creates no admission or use row. An actual storage, I/O, or commit effect failure returns WriteFailed. Every non-Admitted branch preserves the existing profile revision and origin; later detector conflict is exposed as profile drift rather than implicit profile mutation.
    scope:
      - profile-inference
      - profile-drift
    support_refs:
      - SS.functional.profile.001.D1
      - SS.procedural.profile-onboarding.001.A1
      - SS.procedural.profile-onboarding.001.L3
      - SS.interfaces.profile.001.D1
      - TS.boundary.001.A4
    governing_pattern_refs:
      - A.7
  - id: SS.procedural.profile-onboarding.001.D4
    class: D
    statement: For ApplySupportedSingleton, haft init must bind the exact detector snapshot, WorkInput, payload, and contingent profile-dependent carriers in its plan; revalidate the observation digest before any write and again before admission; install or migrate core storage; admit and project origin=detector_default through the automatic authority path; install applicable specification and MethodPack carriers only after successful admission; remove an unchanged Haft-generated review only after admission and carrier success; then continue default-memory and host-adapter installation.
    scope:
      - initial-profile-bootstrap
      - init-ordering
      - projection-debt
    support_refs:
      - SS.functional.profile.001.L5
      - SS.interfaces.profile.001.D1
      - SS.interfaces.profile.001.D5
      - SS.procedural.profile-onboarding.001.A1
      - TS.boundary.001.A7
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.procedural.profile-onboarding.001.D5
    class: D
    statement: Automatic bootstrap must never change an existing canonical profile. A direct, unambiguous operator request may supersede an existing profile only when its current origin is detector_default and records host_routed_operator_request provenance; further host-routed changes and legacy explicit_operator or legacy_unknown profiles require the separate profile-change contract and are never silently reclassified.
    scope:
      - profile-change
      - profile-origin
    support_refs:
      - SS.functional.profile.001.L1
      - SS.procedural.profile-onboarding.001.D4
      - TS.environment.001.L7
    governing_pattern_refs:
      - A.6.B
      - A.7
```

## SS.functional.query.001 Source-native FPF Query

```yaml spec-section
id: SS.functional.query.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: Source-native FPF Query
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.role.001
target_refs:
  - TS.role.001
  - TS.boundary.001
claims:
  - id: SS.functional.query.001.L1
    class: L
    statement: FPF Query reads practical-use cards, Preface units, Table-of-Contents rows, pattern bodies, and pattern sections derived from one pinned FPF publication.
    scope:
      - fpf-query
  - id: SS.functional.query.001.L2
    class: L
    statement: Query results are ExactHit, CandidateSet, or Abstained source-retrieval results and do not encode applicability, governing-pattern selection, an Evidence relation, or authorization.
    scope:
      - fpf-query-result
  - id: SS.functional.query.001.D1
    class: D
    statement: The FPF Source Query component must preserve several candidates when the source basis is ambiguous and must group retrieval by source role rather than merge unlike roles into one pseudo-precise score.
    scope:
      - concern-query
    support_refs:
      - SS.allocation.001.L5
      - TS.role.001.A1
      - "code:internal/fpf/source_query.go"
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.1"
    governing_pattern_refs:
      - E.11.PUA
      - E.11.PUR
  - id: SS.functional.query.001.A1
    class: A
    statement: Exact lookup is admissible only for an exact PatternID, exact authored name, or exact resolvable source-unit identifier; broadening an exact inspect identifier into a concern search is inadmissible.
    scope:
      - exact-lookup
      - inspect
    support_refs:
      - SS.functional.query.001.L1
      - SS.functional.query.001.L2
      - "code:internal/fpf/source_query.go"
      - TS.role.001.L1
    governing_pattern_refs:
      - E.11.PUR
```

## SS.interfaces.query.001 FPF Query CLI and MCP contract

```yaml spec-section
id: SS.interfaces.query.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: FPF Query CLI and MCP contract
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.query.001
target_refs:
  - TS.role.001
claims:
  - id: SS.interfaces.query.001.L1
    class: L
    statement: The canonical MCP query interface is haft_query with action fpf and mode concern, lookup, or inspect; CLI exposes equivalent fpf query, lookup, and inspect operations.
    scope:
      - mcp
      - cli
  - id: SS.interfaces.query.001.L2
    class: L
    statement: A canonical FPF Query execution retains the exact source-unit and result provenance plus retrieval internals required for integrity, replay, audit, and diagnostics, then publishes one closed public view independently of retrieval mode. The default working view carries stable source identity and the source-owned semantics needed to continue work; explicit trace and diagnostic views carry provenance and retrieval internals respectively.
    scope:
      - query-provenance-shape
      - query-publication-view
  - id: SS.interfaces.query.001.D1
    class: D
    statement: The FPF Source Query component must validate and retain canonical provenance before publication; default working responses must recursively exclude repository-local paths, line ranges, content hashes, source revisions, raw match grounds, producer/debug witnesses, and repeated relation provenance while preserving source role, stable unit identity, PatternID when present, title, publication status, bounded source-owned text or practical-use cues, authored relation kind and target, ambiguity, abstention, and truncation posture. Exact inspect must retain the complete authoritative source body.
    scope:
      - query-provenance
      - query-working-view
    support_refs:
      - SS.allocation.001.L5
      - SS.functional.query.001.L1
      - "code:internal/fpf/source_meta.go"
      - "code:internal/fpf/source_query.go"
      - "code:internal/fpf/source_query_public.go"
    governing_pattern_refs:
      - E.11.PUR
    evidence_refs:
      - SS.interfaces.query.001.E1
  - id: SS.interfaces.query.001.D2
    class: D
    statement: Host adapters must not add a Haft-authored pattern route catalog, a recommended pattern field, a required next action, or a retrieval-score-selected winner to the public query schema.
    scope:
      - query-schema
    support_refs:
      - SS.allocation.001.L11
      - TS.role.001.A1
      - SS.functional.query.001.L2
    governing_pattern_refs:
      - E.11.PUA
      - A.7
  - id: SS.interfaces.query.001.D3
    class: D
    statement: Equivalent CLI and MCP requests must use the same public projection and encoder. Explicit trace must deduplicate and exactly reconstruct the canonical source snapshot and provenance through an opaque trace reference, replay against changed source, request, or canonical result must return a typed mismatch, and raw retrieval grounds or producer internals must appear only in explicit diagnostic view.
    scope:
      - query-trace
      - query-diagnostic
      - query-replay
      - cli-mcp-parity
    support_refs:
      - SS.interfaces.query.001.L1
      - SS.interfaces.query.001.L2
      - SS.interfaces.query.001.D1
      - "code:internal/fpf/source_query_public.go"
      - "code:internal/cli/fpf.go"
      - "code:internal/cli/serve.go"
    governing_pattern_refs:
      - E.11.PUR
    evidence_refs:
      - SS.interfaces.query.001.E1
  - id: SS.interfaces.query.001.E1
    class: E
    statement: Under current-worktree source inspection and focused projection, token, downstream-project, CLI, and MCP tests on 2026-07-22, canonical QueryResult values retain validated provenance while the shared public encoder produces separated working, trace, diagnostic, and replay-mismatch carriers. This is current-worktree evidence only and does not claim that the installed live MCP has been replaced or that final P14 acceptance passed.
    scope:
      - query-provenance-evidence
      - query-publication-evidence
    support_refs:
      - SS.functional.query.001.L1
      - SS.interfaces.query.001.L2
      - SS.interfaces.query.001.D1
      - SS.interfaces.query.001.D3
    evidence_refs:
      - "carrier:internal/fpf/source_query_public.go"
      - "carrier:internal/fpf/source_query_public_test.go"
      - "carrier:internal/fpf/source_query_token_gate_test.go"
      - "carrier:internal/cli/fpf.go"
      - "carrier:internal/cli/serve.go"
      - "command:go test ./internal/fpf -run 'Test(Working|Trace|Diagnostic|Canonical|QueryPublication|Published|FPFQueryWorking)' -count=1"
      - "command:go test ./internal/cli -run 'Test(FPF|HandleQuintQuery_FPF|EmbeddedFPF|SourceQuery|InterfaceCatalog.*FPF|Interface.*FPF)' -count=1"
    valid_until: 2026-07-29
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.functional.memory.001 Typed EntityOfConcern-centered project memory

```yaml spec-section
id: SS.functional.memory.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: Typed EntityOfConcern-centered project memory
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.profile.001
  - SS.role.001
target_refs:
  - TS.role.001
  - TS.boundary.001
claims:
  - id: SS.functional.memory.001.L1
    class: L
    statement: Canonical project-memory writes are non-empty MemoryChangeSets whose v2 closed MemoryChange algebra contains only declare_entity, identity_change, assert_relation against a signature present and executable in the exact selected project basis, or retract_assertion. An assert_relation change carries an explicit affirms_obtaining, denies_obtaining, or obtaining_unknown modality; omission is not an assertion. The v2 identity_change union contains only context-bound admit_alias and supersede_alias operations; merge and split are not v2 MemoryChange variants. No MemoryChange may declare or admit a kind, relation signature, ValueShape, CodecRef, codec implementation, or schema extension.
    scope:
      - typed-memory-write-model
  - id: SS.functional.memory.001.L2
    class: L
    statement: One stable EntityID identifies each project entity; context-kind availability, KindClassificationJudgements, optional KindExtension representations, and relation uses are interpreted in an explicit bounded context rather than encoded as competing identities. ContextKindAvailability is a derived TypeEnv projection and is neither a candidate classification nor durable FPF U-kind admission under E.24.UK.
    scope:
      - entity-identity
  - id: SS.functional.memory.001.L3
    class: L
    statement: EntityOfConcernSlot is the named SlotKind in a governed relation signature; its filler has ValueKind U.Entity and, in reference mode, the filled instance stores entityOfConcernRef of RefKind U.EntityRef. BoundedContextRef is a separate context reference for interpretation and is neither the filler nor part of entityOfConcernRef; slot, filler, reference, and context remain distinct, and EntityOfConcern is not a universal stored node kind.
    scope:
      - entity-of-concern
    support_refs:
      - TS.boundary.001.L6
    governing_pattern_refs:
      - A.6.5
      - A.7
  - id: SS.functional.memory.001.D1
    class: D
    statement: AdmissionService must retain the governed relation instance with its exact entityOfConcernRef slot content and separate BoundedContextRef whenever the source-pinned relation signature declares EntityOfConcernSlot.
    scope:
      - project-memory
    support_refs:
      - SS.allocation.001.L7
      - SS.functional.memory.001.L3
      - SS.constraints.typed-memory.001.A1
      - TS.boundary.001.L6
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.3"
    governing_pattern_refs:
      - C.2.1
      - A.6.5
  - id: SS.functional.memory.001.L4
    class: L
    statement: A canonical relation write preserves named n-ary slots; an arbitrary free-string source-target-type edge is not an equivalent semantic write.
    scope:
      - relation-instance
  - id: SS.functional.memory.001.L5
    class: L
    statement: Ordinary bounded conversation creates no persistent project-memory artifact.
    scope:
      - reliance-gated-persistence
  - id: SS.functional.memory.001.L6
    class: L
    statement: A ByValue SlotFiller in canonical project memory is a VerifiedTypedValue constructed only by the pure verifier and binds one exact ValueKindRef, ValueShapeRef, immutable CodecRef, canonical byte sequence, and domain-separated length-prefixed digest; arbitrary JSON, caller-asserted verification, and a registered codec without an active TypeEnv binding are not equivalent values.
    scope:
      - by-value
      - canonical-value
    support_refs:
      - SS.procedural.typeenv.001.L4
      - SS.procedural.typeenv.001.L5
    governing_pattern_refs:
      - A.6.5
      - C.2.1
  - id: SS.functional.memory.001.L7
    class: L
    statement: An OpaqueStoredValue preserves the exact historical ValueKindRef, ValueShapeRef, CodecRef, canonical bytes, digest, and provenance when its codec implementation or governing rule is unavailable; it is inspectable historical content, not a VerifiedTypedValue and not admissible for a new write.
    scope:
      - historical-by-value
    support_refs:
      - SS.functional.memory.001.L6
      - SS.procedural.typeenv.001.D2
    governing_pattern_refs:
      - A.7
      - A.10
  - id: SS.functional.memory.001.A1
    class: A
    statement: Persistence is admissible only after an explicit save request or when a concrete receiving use, operator-named or agent-inferred from current Work, requires addressable replay, transfer, audit, automation, delayed feedback, expensive feedback, or costly reversal. If memory resolution establishes known absence and stable identity, bounded context, and aliases are recoverable, the host caller establishes the minimum non-binding EntityOfConcern without a separate permission prompt and records the exact receiving use as request provenance. Known absence alone, generic possible future usefulness, or an empty graph is not sufficient. This persistence grants no decision, commission, specification-lifecycle, evidence-truth, or other binding authority.
    scope:
      - reliance-gated-persistence
    support_refs:
      - SS.role.001.L1
      - TS.role.001.L5
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.7
      - C.2.1
  - id: SS.functional.memory.001.A2
    class: A
    statement: A ProblemCard is admissible for persistence only after an explicit save request or when a concrete later decision, specification, handoff, multi-session use, or verification use, operator-named or agent-inferred from current Work, requires its stable identity.
    scope:
      - problem-card
    support_refs:
      - SS.functional.memory.001.A1
      - "legacy:ES.work-methods.001"
    governing_pattern_refs:
      - E.18.1
      - A.7
  - id: SS.functional.memory.001.A3
    class: A
    statement: A SolutionPortfolio is admissible for persistence only when its alternatives or comparison require a stable identity for a concrete receiving use, operator-named or agent-inferred from current Work.
    scope:
      - solution-portfolio
    support_refs:
      - SS.functional.memory.001.A1
      - "legacy:ES.work-methods.001"
    governing_pattern_refs:
      - A.7
```

## SS.functional.recall.001 EntityOfConcern-centered recall

```yaml spec-section
id: SS.functional.recall.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: EntityOfConcern-centered recall
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.memory.001
target_refs:
  - TS.role.001
claims:
  - id: SS.functional.recall.001.L1
    class: L
    statement: Recall is a read-only projection that resolves an exact EntityRef together with its BoundedContextRef into typed graph matches and independently identified lexical or semantic candidates.
    scope:
      - project-memory-recall
  - id: SS.functional.recall.001.D1
    class: D
    statement: ProjectionService must return candidate provenance, relation path, bounded context, TypeEnv basis, producer identity, and explicit truncation for recall candidates without treating rank as truth, applicability, authority, or a next project action.
    scope:
      - recall-result
    support_refs:
      - SS.allocation.001.L8
      - SS.functional.memory.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#P10A"
    governing_pattern_refs:
      - C.2.1
      - A.7
  - id: SS.functional.recall.001.L2
    class: L
    statement: Recall orders exact EntityID, exact artifact-reference, and exact typed-relation matches before broad candidate producers; insufficient or conflicting basis remains visible as Abstained or Underdetermined.
    scope:
      - recall-admission
  - id: SS.functional.recall.001.L3
    class: L
    statement: When no exact EntityRef and BoundedContextRef resolve, recall returns entity candidates or Underdetermined and never manufactures an EntityOfConcern slot filling.
    scope:
      - unresolved-concern
  - id: SS.functional.recall.001.L4
    class: L
    statement: A derived recall projection keeps the last known good index readable until a replacement rebuild completes and invalidates the projection after a committed corpus change.
    scope:
      - recall-rebuild
  - id: SS.functional.recall.001.E1
    class: E
    statement: Under current-worktree source inspection and hybrid-recall tests on 2026-07-14, the existing recall projection preserves a serving index during rebuild and invalidates it after corpus change for the human specification reviewer in the projection-resilience viewpoint; this observation is not an admitted supporting episteme for an Evidence relation about the future typed EntityOfConcern recall contract.
    scope:
      - recall-rebuild-evidence
    support_refs:
      - SS.functional.recall.001.L4
    evidence_refs:
      - "carrier:internal/recall/hybrid.go"
      - "carrier:internal/recall/hybrid_test.go"
      - "command:go test ./internal/recall -count=1"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.interfaces.memory.001 Typed memory and recall interfaces

```yaml spec-section
id: SS.interfaces.memory.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: Typed memory and recall interfaces
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.memory.001
  - SS.functional.recall.001
  - SS.interfaces.profile.001
target_refs:
  - TS.role.001
claims:
  - id: SS.interfaces.memory.001.L1
    class: L
    statement: The low-level haft.memory.v2 surface exposes stable validate and admit operations, while the read projection exposes resolve, EntityOfConcern neighborhood, and recall; CLI and MCP project the same closed action-specific result unions. The task-level haft.entity.v1 establish surface owns the complete agent path from a persistence-authorized absent identity through conflict checking, validation, admission, exact post-commit resolution, and a canonical U.EntityRef.
    scope:
      - mcp
      - cli
  - id: SS.interfaces.memory.001.L2
    class: L
    statement: A public haft.memory.v2 admit request contains contract_version, action=admit, an exact_project basis carrying type_env_digest and graph_revision, authority_class=non_binding_semantic_assertion, idempotency_key, request_provenance_ref, and one non-empty closed change_set. Bounded-context data belongs to the exact change that requires it. The request has no schema-change, project-basis-selection, decision-binding, commission, or specification-lifecycle variant.
    scope:
      - typed-memory-write-request
  - id: SS.interfaces.memory.001.L3
    class: L
    statement: The task-level haft_onboard MCP interface exposes only status and profile_prepare. Haft init atomically installs the package-default project-memory basis and reconciles an existing head to every exact proven-compatible bundled ProjectTypeEnv successor before reporting success, without an enable, defer, review, or schema-selection question and without requiring a canonical project profile for Genesis. Status and repository detection write no state; profile_prepare may materialize or reuse only a non-binding profile review carrier and never applies it or binds a decision. A reviewed profile is consumed through haft onboard profile apply only after one direct, unambiguous operator request; no skill name or second confirmation is required. A legacy or partial default-memory installation reports needs_init and is repaired by rerunning haft init. An incompatible, incomplete, stale, or underdetermined successor leaves the current head unchanged and returns an exact diagnostic; manual rollback and incompatible model selection remain outside the onboarding surface.
    scope:
      - onboarding-interface
      - review-carrier
      - authority-boundary
    support_refs:
      - SS.interfaces.profile.001
      - SS.role.001.L1
      - "code:internal/fpf/onboard_schema.go"
      - "carrier:internal/cli/skill/h-onboard/SKILL.md"
  - id: SS.interfaces.memory.001.D1
    class: D
    statement: Host adapters must preserve strong opaque identifiers, the exact public basis object, idempotency key for writes, named slot bindings, bounded-context reference where the selected change requires it, provenance, and response-budget or truncation controls in the low-level typed-memory contract. The normal agent-facing haft.entity.v1 establishment request contains exactly action=establish plus entity_id, label, bounded_context_ref, aliases, persistence_reason, request_provenance_ref, and idempotency_key; it contains no contract_version, MemoryChangeSet, graph revision, schema coordinate, or selectable project-basis component. A ByValue request carries a closed candidate envelope naming exact ValueKindRef, ValueShapeRef, CodecRef, and input bytes; it cannot carry arbitrary JSON or claim that a caller-constructed value is VerifiedTypedValue.
    scope:
      - typed-memory-schema
    support_refs:
      - SS.allocation.001.L11
      - SS.interfaces.memory.001.L2
      - SS.functional.memory.001.L1
      - SS.functional.memory.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#4.3"
    governing_pattern_refs:
      - A.6.5
      - C.2.1
  - id: SS.interfaces.memory.001.D2
    class: D
    statement: Host adapters must use a stable source-independent MCP meta-schema and must not snapshot the current FPF kind set as a hard-coded transport enum.
    scope:
      - mcp-schema
    support_refs:
      - SS.allocation.001.L11
      - SS.functional.memory.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.3"
    governing_pattern_refs:
      - C.3.1
      - A.7
  - id: SS.interfaces.memory.001.A1
    class: A
    statement: A generic admit request is admissible only when every member belongs to the closed instance-level MemoryChange algebra and the request contains no context-kind-availability or other TypeEnv-schema change, relation-signature declaration, ValueShape or CodecRef declaration, codec or evaluator registration, symbolic E declaration, ProjectTypeEnvHead Genesis, Transition, rollback, or other head-selection effect, DecisionRecord creation or supersession, WorkCommission creation, or SpecSection lifecycle effect.
    scope:
      - generic-admit
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L7
      - TS.boundary.001.A3
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.7
      - A.15
  - id: SS.interfaces.memory.001.D3
    class: D
    statement: AdmissionService must distinguish an authority or schema effect excluded by SS.interfaces.memory.001.A1, which returns a typed rejected result naming the dedicated manual or gated interface, from unavailable exact project basis, a signature not present or executable there, or a missing codec, evaluator, ValueKind binding, or compiled rule, which returns Underdetermined with a readable inspect, init-repair, compile, validate, or retry affordance. Neither branch may auto-declare a non-default schema, register a mechanism, derive a trusted source, change the selected project-memory schema, or reinterpret historical assertions.
    scope:
      - generic-admit-rejection
    support_refs:
      - SS.allocation.001.L7
      - SS.interfaces.memory.001.A1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.7
      - A.15
```

## SS.interfaces.code-graph.001 Fused code-graph Explore and orientation contract

```yaml spec-section
id: SS.interfaces.code-graph.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: Fused code-graph Explore and orientation contract
statement_type: definition
claim_layer: object
owner: human
status: draft
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.interfaces.memory.001
target_refs:
  - TS.role.001
  - TS.boundary.001
claims:
  - id: SS.interfaces.code-graph.001.L1
    class: L
    statement: The canonical Explore interface accepts exactly one seed shape, either an exact symbol with its optional file and line coordinates or a bounded concern query; both and neither are invalid. Exact-symbol, multi-symbol, and concern execution produce one canonical ExploreEnvelope before presentation, and a concern candidate order is advisory rather than an automatically selected identity.
    scope:
      - code-graph-explore
      - concern-discovery
    support_refs:
      - SS.allocation.001.L8
      - SS.allocation.001.L11
      - "code:internal/codeintel/explore_public.go"
      - "plan:.context/haft-v9-graph-ast-improvements-plan.md#HG7"
  - id: SS.interfaces.code-graph.001.L2
    class: L
    statement: A canonical ExploreEnvelope retains the exact request basis, published code-index epoch and coverage, typed seed resolution, typed traversal outcome when one exists, candidate set, source hops, fused reasoning context, retrieval diagnostics, and replay basis; working, trace, and diagnostic are pure action-scoped projections of that envelope rather than independent presenter classifications.
    scope:
      - code-graph-envelope
      - code-graph-publication-view
    support_refs:
      - SS.interfaces.code-graph.001.L1
      - "code:internal/codeintel/outcome.go"
      - "code:internal/codeintel/explore_public.go"
  - id: SS.interfaces.code-graph.001.D1
    class: D
    statement: The default working projection must expose at most five concern candidates, omit raw lexical and graph-rank vectors together with retrieval-origin, artifact-support, per-edge provenance, and full exclusion details, state that ranking is advisory and identity was not auto-selected, retain compact index coverage and any limiting reason, and include freshness-revalidated source only within the fixed Explore source and encoded-payload budgets. Fused reasoning context is relevance evidence, not proof that an exact active artifact governs the target; stronger authority use resolves the exact governing set or artifact status and scope. Trace restores bounded provenance under the same replay basis. Truncation must name the stronger return view rather than silently changing the semantic outcome.
    scope:
      - code-graph-working-view
      - response-budget
    support_refs:
      - SS.interfaces.code-graph.001.L2
      - "code:internal/codeintel/explore_public.go"
      - "code:internal/codeintel/explore_public_test.go"
  - id: SS.interfaces.code-graph.001.D2
    class: D
    statement: Equivalent CLI and MCP Explore requests must execute through the same canonical application, projection, and compact JSON encoder. An opaque trace reference binds the index snapshot, typed request, and canonical result; replay after any of those changes returns a typed replay_mismatch instead of presenting a different result as the prior trace.
    scope:
      - code-graph-trace
      - code-graph-replay
      - cli-mcp-parity
    support_refs:
      - SS.interfaces.code-graph.001.L2
      - "code:internal/codeintel/explore_public.go"
      - "code:internal/cli/graph.go"
      - "code:internal/cli/serve.go"
  - id: SS.interfaces.code-graph.001.D3
    class: D
    statement: CandidateSet and unresolved Explore results must not contain a fabricated traversal path; budget stops and unavailable capabilities remain incomplete or unavailable, and static absence may be stated only under an exact complete index basis. Exact-symbol Explore retains freshness-revalidated source and typed path semantics without passing through concern ranking.
    scope:
      - code-graph-outcomes
      - code-graph-coverage
      - exact-symbol-compatibility
    support_refs:
      - SS.interfaces.code-graph.001.L1
      - SS.interfaces.code-graph.001.L2
      - "code:internal/codeintel/outcome.go"
      - "code:internal/codeintel/explore.go"
      - "code:internal/codeintel/explore_public.go"
  - id: SS.interfaces.code-graph.001.D4
    class: D
    statement: Managed agent carriers and the read-only session audit must treat code-graph orientation and structured-memory orientation as separate conditional uses. Area or flow orientation uses Explore; a non-mechanical edit where recorded governance may be material uses code_context or impact on the actual target; purely mechanical work may be not_applicable. Candidate rank, file or module proximity, displayed invariant relevance, empty caller lists, and incomplete traversal do not establish exact authority or safety. Context-heavy or multi-session reliance uses exact typed-memory resolution and bounded neighborhood hydration when available; unavailable setup is visible and non-blocking for unrelated authorized Work, known_absent does not authorize persistence, and a persistence-authorized new EntityOfConcern routes through haft.entity.v1 rather than caller-authored low-level admission.
    scope:
      - agent-orientation
      - code-graph-orientation
      - typed-memory-orientation
      - memory-admission-boundary
    support_refs:
      - SS.interfaces.memory.001.L1
      - SS.interfaces.memory.001.A1
      - SS.interfaces.hosts.001.L1
      - "code:internal/cli/codeintel_doctrine.go"
      - "code:internal/cli/session_audit.go"
      - "carrier:internal/cli/skill/h-reason/SKILL.md"
    governing_pattern_refs:
      - E.11.PUA
  - id: SS.interfaces.code-graph.001.E1
    class: E
    statement: Under current-worktree focused tests on 2026-07-26, one canonical Explore execution produced deterministic working, trace, diagnostic, and replay-mismatch projections; MCP and CLI shared one encoder; exact-symbol source and path semantics remained present; and session audit fixtures distinguished not_applicable, used, unavailable, and incorrectly_skipped separately for code graph and typed memory. This is current-worktree evidence only and does not claim that the installed live MCP has been replaced or that final P14 acceptance passed.
    scope:
      - code-graph-current-worktree-evidence
      - agent-orientation-current-worktree-evidence
    support_refs:
      - SS.interfaces.code-graph.001.D1
      - SS.interfaces.code-graph.001.D2
      - SS.interfaces.code-graph.001.D3
      - SS.interfaces.code-graph.001.D4
    evidence_refs:
      - "carrier:internal/codeintel/explore_public.go"
      - "carrier:internal/codeintel/explore_public_test.go"
      - "carrier:internal/cli/graph.go"
      - "carrier:internal/cli/serve_concern_test.go"
      - "carrier:internal/cli/session_audit.go"
      - "carrier:internal/cli/session_audit_test.go"
      - "command:go test ./internal/codeintel -count=1"
      - "command:go test ./internal/cli -run 'TestHandleQuintQueryExplore|Test.*Session.*Audit|TestBuildSessionGraphAudit|TestSessionAudit' -count=1"
    valid_until: 2026-08-02
```

## SS.interfaces.hosts.001 Host-adapter contract

```yaml spec-section
id: SS.interfaces.hosts.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: Host-adapter contract
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.interfaces.profile.001
  - SS.interfaces.query.001
  - SS.interfaces.memory.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.interfaces.hosts.001.L1
    class: L
    statement: A host adapter projects the common Haft CLI, MCP, skill, and managed-instruction contracts into a supported coding-agent host without becoming an independent governance authority.
    scope:
      - host-adapter
  - id: SS.interfaces.hosts.001.A1
    class: A
    statement: A host integration is admissible as supported only when current Evidence relations grounded in admitted runtime supporting epistemes cover its configuration loading, tool contract, direct-request routing boundary, manual WorkCommission boundary, and structured error behavior.
    scope:
      - host-support
    support_refs:
      - SS.allocation.001.L11
      - "legacy:ES.agent-policy.001"
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.interfaces.hosts.001.D1
    class: D
    statement: Host adapters must route semantic writes and binding requests through the canonical kernel interfaces and must not write raw SQLite rows or carrier bytes as a parallel authority path.
    scope:
      - host-mutation-boundary
    support_refs:
      - SS.allocation.001.L11
      - SS.allocation.001.L7
      - SS.constraints.authority.001.A1
    governing_pattern_refs:
      - A.7
  - id: SS.interfaces.hosts.001.D2
    class: D
    statement: Managed host instructions must present source-first FPF use, independent capabilities, conditional persistence, direct operator-request routing for effect-specific human gates, and the separate manual WorkCommission gate without embedding a second FPF route catalog.
    scope:
      - managed-host-carrier
    support_refs:
      - SS.allocation.001.L11
      - SS.role.001.L2
      - SS.functional.memory.001.A1
      - SS.interfaces.query.001.D2
    governing_pattern_refs:
      - E.11.PUA
      - A.7
  - id: SS.interfaces.hosts.001.D3
    class: D
    statement: Managed hosts may invoke h-decide implicitly only when the operator directly and unambiguously requests one exact binding effect, subject, selected option, and scope. A manual h-decide token remains a compatible shortcut and is never an authorization receipt. The host records host_routed_operator_request without claiming independent proof of U.SpeechAct. Quotations, pasted third-party text, agent recommendations, hypotheticals, and tool output do not bind. MCP DecisionRecord creation remains fail-closed with operator_confirmation_required until a separately specified verifiable host receipt exists; h-commission remains manual-only.
    scope:
      - host-request-routing-boundary
      - decision-binding-policy
      - mcp-boundary
    support_refs:
      - SS.interfaces.hosts.001.A1
      - SS.constraints.authority.001.L2
      - SS.constraints.authority.001.A3
      - SS.constraints.authority.001.A4
    governing_pattern_refs:
      - A.2.9
      - A.7
      - A.15
```

## SS.procedural.spec-lifecycle.001 SpecSection lifecycle and approval baseline

```yaml spec-section
id: SS.procedural.spec-lifecycle.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: SpecSection lifecycle and approval baseline
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.constraints.authority.001
  - SS.interfaces.hosts.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.spec-lifecycle.001.L1
    class: L
    statement: A Markdown spec-section block is a carrier projection; its parsed canonical SpecSection is the governed description, and neither object becomes approval authority by existing.
    scope:
      - spec-carrier-boundary
  - id: SS.procedural.spec-lifecycle.001.L2
    class: L
    statement: A SpecSectionApprovalBaseline identifies one exact SpecSection edition by canonical section hash, section ID, project ID, approver, and capture time.
    scope:
      - spec-approval-baseline
  - id: SS.procedural.spec-lifecycle.001.D1
    class: D
    statement: SpecLifecycleService must expose the current document kind, section kind, carrier, expected fields, checks, local lifecycle action, and human gate before a draft edit.
    scope:
      - spec-draft
    support_refs:
      - SS.allocation.001.L9
      - "code:internal/project/specflow/projection.go"
      - "code:internal/project/specflow/phases.go"
    governing_pattern_refs:
      - A.7
  - id: SS.procedural.spec-lifecycle.001.A1
    class: A
    statement: A draft SpecSection is admissible for lifecycle review only after its typed carrier parses and the deterministic structural checks for its declared phase pass.
    scope:
      - spec-check
    support_refs:
      - SS.procedural.spec-lifecycle.001.L1
      - "code:internal/project/speccheck.go"
      - "code:internal/project/specflow/checks.go"
    governing_pattern_refs:
      - A.6.B
      - A.7
  - id: SS.procedural.spec-lifecycle.001.A2
    class: A
    statement: SpecSection approval is admissible only for the exact reviewed active section bytes after explicit human approval through the dedicated lifecycle interface.
    scope:
      - spec-approve
    support_refs:
      - SS.allocation.001.L1
      - SS.constraints.authority.001.A1
      - "code:internal/cli/serve_spec_section.go"
    governing_pattern_refs:
      - A.7
  - id: SS.procedural.spec-lifecycle.001.D2
    class: D
    statement: SpecLifecycleService must record an approved SpecSection as a SpecSectionApprovalBaseline containing the canonical section hash, section ID, project ID, approver, and capture time.
    scope:
      - spec-approval-baseline
    support_refs:
      - SS.allocation.001.L9
      - SS.procedural.spec-lifecycle.001.A2
      - SS.procedural.spec-lifecycle.001.L2
      - "code:internal/project/specflow/baseline.go"
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.procedural.spec-lifecycle.001.A3
    class: A
    statement: Stronger product use of a SpecSection carrier is admissible if and only if parsing, lifecycle acceptance, current status, required approval baseline, and any source-currentness basis required by its governing-pattern claims all resolve for the exact section edition.
    scope:
      - spec-authority
    support_refs:
      - SS.procedural.spec-lifecycle.001.A1
      - SS.procedural.spec-lifecycle.001.A2
      - SS.procedural.spec-lifecycle.001.L2
    governing_pattern_refs:
      - A.7
      - A.10
  - id: SS.procedural.spec-lifecycle.001.L3
    class: L
    statement: A SpecSourceCurrentnessAssessment is a read-only comparison between one exact SpecSection edition and the exact prior and current FPF source identities and direct pattern bodies on which its claims rely; it classifies affected occurrences as current source meaning, Haft local-practice or API vocabulary, sealed legacy compatibility spelling, historical citation, or unrelated homonym, and is neither implementation evidence nor lifecycle approval.
    scope:
      - spec-source-currentness
    support_refs:
      - SS.procedural.spec-lifecycle.001.L1
      - SS.procedural.spec-lifecycle.001.L2
    governing_pattern_refs:
      - A.7
      - B.3
  - id: SS.procedural.spec-lifecycle.001.D3
    class: D
    statement: When a governing FPF source identity advances, the h-spec host surface must recover the exact current direct pattern body, produce a SpecSourceCurrentnessAssessment, identify every affected active claim and implementation or wire compatibility surface, and route any semantic claim change through an explicit before/after edition without blind token replacement or automatic approve, reopen, or rebaseline.
    scope:
      - h-spec
      - spec-source-currentness
    support_refs:
      - SS.allocation.001.L9
      - SS.procedural.spec-lifecycle.001.L3
      - SS.procedural.spec-lifecycle.001.A3
    governing_pattern_refs:
      - A.7
      - B.3
  - id: SS.procedural.spec-lifecycle.001.D4
    class: D
    statement: Spec checks and semantic reviews must report their exact level and source basis and must not present a green structural or existing-claim review as proof that the section remains compatible with a newer FPF source revision.
    scope:
      - spec-check
      - spec-review
      - source-currentness
    support_refs:
      - SS.procedural.spec-lifecycle.001.L3
      - SS.procedural.spec-lifecycle.001.D3
    governing_pattern_refs:
      - A.7
      - B.3
```

## SS.procedural.typeenv.001 TypeEnv compilation and evolution

```yaml spec-section
id: SS.procedural.typeenv.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: TypeEnv compilation and evolution
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.query.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.typeenv.001.L1
    class: L
    statement: An FPFBaseTypeEnv is an immutable content-addressed compilation of explicitly covered FPF kind, relation-signature, slot, bridge, and constraint declarations plus a coverage manifest and exact FPF source provenance; it contains no rule inferred from examples, retrieval rank, or lexical proximity.
    scope:
      - type-environment
  - id: SS.procedural.typeenv.001.L2
    class: L
    statement: A newly bundled FPF base or Haft extension is only a candidate; bundling, compiling, validating, diffing, or staging it does not make it part of the project-active environment.
    scope:
      - typeenv-candidate
  - id: SS.procedural.typeenv.001.L3
    class: L
    statement: A ProjectTypeEnvExtensionArtifact is the self-reference-free content-addressed symbolic compilation E of one exact versioned Haft Typed-Memory Local-Practice/DPF carrier. It preserves exact carrier identity and edition, content digest, base TypeEnvRef, bounded context, A.6.0 four-row source coordinates, exact SignatureManifest closure, A.6.5 SlotSpecs, per-symbol provenance, compiler schema, and the exact declaration/use plus Applicability grounds from which the linker may derive ContextKindAvailability while TypeEnv-bearing positions remain symbolic. ContextKindAvailability is not an authored fifth Signature row or a standalone FPF object. Mutable project state, compatibility and revalidation results, a later Stage, a lowered TypeEnvExtensionProposal, and MemoryChangeSet instance writes are not part of E identity.
    scope:
      - project-typeenv-extension
    governing_pattern_refs:
      - A.6.0
      - A.6.5
      - E.4.DPF
  - id: SS.procedural.typeenv.001.L4
    class: L
    statement: A ProjectTypeEnvCompositeArtifact derives its TypeEnvRef C from one exact verified FPFBaseTypeEnv B, a canonically topologically ordered and closed DAG of exact symbolic extension artifacts E, and one exact RuntimeEvaluationBasisArtifact X before any TypeEnv-bearing runtime declaration is lowered. No caller supplies C. Final lowering rebuilds both selected base declarations and linked extension declarations at C and materializes one canonical ContextKindAvailability per exact context-kind coordinate from the linked declaration/use, Applicability, provider, and required bridge grounds; copying or relabelling the already lowered B environment is invalid. A project-active TypeEnv is the exact C selected by the project TypeEnv head through the dedicated head-selection effect.
    scope:
      - project-active-typeenv
    support_refs:
      - SS.procedural.typeenv.001.L1
      - SS.procedural.typeenv.001.L3
      - SS.procedural.typeenv.001.A1
    governing_pattern_refs:
      - A.6.0
      - C.3.1
  - id: SS.procedural.typeenv.001.L6
    class: L
    statement: RuntimeEvaluationBasisArtifact is an immutable content-addressed Haft-local realization identity containing only exact codec, evaluator, and carrier-membership mechanism pins whose semantics can change Valid, Invalid, or Underdetermined. Changing B, E, or X changes C; current project records, graph revision, compatibility or revalidation results, project head, human intent, authority use, and ProjectTypeEnvHeadSelectionReceiptV1 are excluded from X and C identity.
    scope:
      - runtime-evaluation-basis
      - project-typeenv-composite
    support_refs:
      - SS.procedural.typeenv.001.L3
      - SS.procedural.typeenv.001.L4
    governing_pattern_refs:
      - A.6.0
      - A.6.1
  - id: SS.procedural.typeenv.001.L5
    class: L
    statement: CodecRegistry maps an immutable CodecRef to a pure decode, shape-check, normalize, and canonical-encode implementation, while exact C selected by the current ProjectTypeEnvHead separately binds a context-available ValueKindRef to exactly one ValueShapeRef and CodecRef. Codec registration provides executable mechanism only and does not establish ContextKindAvailability, a KindClassificationJudgement, candidate features, a value shape, relation signature, KindExtension representation, or project write.
    scope:
      - codec-registry
      - typeenv-admission
    support_refs:
      - SS.procedural.typeenv.001.L4
    governing_pattern_refs:
      - A.6.0
      - A.6.5
  - id: SS.procedural.typeenv.001.A2
    class: A
    statement: An FPF source unit is admissible as TypeEnv compiler input only when it belongs to the pinned current FPF publication and resolves an exact UnitID, revision, content hash, and line range.
    scope:
      - typeenv-compiler-input
    support_refs:
      - SS.procedural.typeenv.001.L1
      - SS.functional.query.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.6.5
      - A.7
  - id: SS.procedural.typeenv.001.D1
    class: D
    statement: TypeEnv Compiler must enforce SS.procedural.typeenv.001.A2 for every emitted FPF base rule and must attach the admitted source UnitID, revision, content hash, line range, compiler rule identifier, and PatternID when the source unit has one.
    scope:
      - typeenv-compiler
    support_refs:
      - SS.allocation.001.L6
      - SS.procedural.typeenv.001.A2
      - SS.functional.query.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.6.5
      - C.3.1
  - id: SS.procedural.typeenv.001.A3
    class: A
    statement: A Haft local-practice carrier is admissible as ProjectTypeEnvExtension compiler input only when strict parsing and exact symbolic linking establish valid UTF-8 and semantic scalars, exact carrier identity and canonical edition, content digest, compiler schema, bounded context, A.6.0 four-row Signature Block, SignatureManifest id and version, real imported SignatureIDs, exact equality between manifest provides and explicit exported symbols, A.6.5 SlotSpecs, per-symbol source ranges, one exact base dependency, and no import cycle, missing import, transitive redeclaration, duplicate or conflicting provide, ghost dependency, base mismatch, or incomplete declaration. Runtime codec, evaluator, and carrier-membership availability is not a condition of symbolic E compilation and is evaluated only through X, composite executability, and Stage.
    scope:
      - project-typeenv-extension-input
    support_refs:
      - SS.procedural.typeenv.001.L3
      - SS.functional.query.001.L1
    governing_pattern_refs:
      - A.6.0
      - A.6.5
      - E.4.DPF
  - id: SS.procedural.typeenv.001.D3
    class: D
    statement: "TypeEnv Compiler must enforce SS.procedural.typeenv.001.A3, treat base_type_env_ref as a separate exact dependency rather than fabricating a fpf.core source SignatureManifest, resolve source imports only through real linked Local-Practice exports, and preserve for every emitted project-local symbol the exact carrier and edition, content digest, four-row location, manifest basis, declaration range, compiler rule, bounded context, and base TypeEnvRef. After exact provider resolution, the linker must derive ContextKindAvailability only from explicit kind declarations or uses under the Signature Applicability context: exact base-provider use and same-context imports may supply grounds, while cross-context local reuse requires an exact KindBridge and a fresh target-side classification. It must aggregate a canonical nonempty provenance-ground set per context-kind coordinate and must not accept spelling, entity presence, a classification result, a KindExtension row, or an authored kind_admission field as a ground. It must keep source-export symbols distinct from runtime KindSignature, classification, and optional extension coordinates and must not place compatibility or existing-assertion revalidation results inside symbolic E identity."
    scope:
      - project-typeenv-extension-compiler
    support_refs:
      - SS.allocation.001.L6
      - SS.procedural.typeenv.001.A3
    governing_pattern_refs:
      - A.6.0
      - A.6.5
      - C.3.1
  - id: SS.procedural.typeenv.001.A1
    class: A
    statement: ProjectTypeEnvHeadSelectionRequest is the only admissible head-selection proposal and contains exact ProjectID; predecessor as Genesis carrying only NoPriorHeadProofRef or Transition carrying only ExactPriorHead with distinct HeadRevision, prior C and project; target carrying exact B, canonically ordered exact E DAG, exact X, exact verified successor C and exact ProjectTypeEnvStageRef; distinct ExpectedGraphRevision; and IdempotencyKey. All proof, predecessor, Stage, C, graph-revision and project fields must agree exactly, and verified C must authenticate the same B, E DAG and X. Default Genesis atomically reproves absence and is a package-owned haft init effect under SS.constraints.authority.001.A6. Every successor proven compatible under SS.constraints.authority.001.A5 advances automatically without an operator request; rollback or explicit selection outside that predicate requires one exact host-routed direct operator request for the selected C and scope. Missing or corrupt prior state is neither Genesis nor Transition, and generic memory admission cannot create or move a head or reinterpret history.
    scope:
      - project-typeenv-head-selection
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L6
      - SS.procedural.typeenv.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#7.3"
    governing_pattern_refs:
      - C.3.1
      - A.7
  - id: SS.procedural.typeenv.001.D2
    class: D
    statement: After exact FPF or Local-Practice source changes, TypeEnv Compiler must produce new immutable B, E, and C candidates, while a separate Stage service produces an immutable content-addressed ProjectTypeEnvStage artifact and record binding exact project, C, current-project snapshot digest, GraphRevision, profile-ledger revision and digest, compatibility result, existing-assertion revalidation result, and source provenance. The Stage carrier remains a separate publication object. Neither candidate construction nor Stage mutates the project head, and historical values and assertions remain inspectable against their originally admitted composite without silently applying new rules or codecs.
    scope:
      - fpf-evolution
      - historical-inspection
    support_refs:
      - SS.allocation.001.L6
      - SS.procedural.typeenv.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#P12"
    governing_pattern_refs:
      - C.3.1
      - A.10
  - id: SS.procedural.typeenv.001.D4
    class: D
    statement: Parse, compile, link, seal, decode, verify, lower, diff, revalidate, persist immutable candidates, inspect, and stage operations for FPF base and ProjectTypeEnvExtension candidates must be read-only and non-binding with respect to canonical project memory and C selected by the current ProjectTypeEnvHead; they cannot create a project head, grant admission capability, or make a signature absent from the selected C writable.
    scope:
      - typeenv-candidate-operations
    support_refs:
      - SS.allocation.001.L6
      - SS.procedural.typeenv.001.L2
      - SS.procedural.typeenv.001.A1
    governing_pattern_refs:
      - A.7
  - id: SS.procedural.typeenv.001.A4
    class: A
    statement: A project TypeEnv composite is executable or stageable for project-head selection only when the closed schema-change algebra covers every declared effect; every current kind-classification use resolves an exact context-local KindSignatureDefinition whose exact EntityOfConcern is the local kind and whose content pins the candidate ValueKind, direct-feature criterion, ContextSlice conditions, effective ReferenceScheme, named assumptions and dependencies, U.Formality, and any optional ExtentRule; every evaluation request separately pins the exact candidate, local kind, signature edition, and ContextSlice; all mandatory value shapes, codecs, X-pinned evaluator registrations, governed candidate-feature inputs, resolved-reference constraints, and value invariants are exact and available; any KindExtension is an optional representation derived from true judgements rather than an EntitySet prerequisite; and every runtime TypeEnv-scoped reference lowers to C. Missing definition, realization, evaluation basis, governed feature input, unresolved reference basis, implicit scalar catalog, unsupported declaration, or evaluator mismatch is Underdetermined or a staging rejection, never false classification or implicit admission.
    scope:
      - project-typeenv-executability
      - typeenv-stage
    support_refs:
      - SS.procedural.typeenv.001.L4
      - SS.procedural.typeenv.001.L5
      - SS.procedural.typeenv.001.L6
    governing_pattern_refs:
      - A.6.1
      - A.6.5
      - C.3.2
  - id: SS.procedural.typeenv.001.D6
    class: D
    statement: One pure kind-classification registry keyed by exact RuleRef must supply the same deterministic evaluator to admission and current-project revalidation. The evaluator returns true or false only by applying the pinned KindSignature criterion to direct governed candidate features for the exact candidate, local kind, signature edition, and ContextSlice. A trusted immutable-store or adapter delivery may carry exact governed record features after producer-policy, carrier, project, entity, context, variant, edition, digest, mapping-manifest, and adapter-version verification, but delivery bytes are not Evidence and do not make the criterion true merely by existing. Missing, malformed, unsupported, untrusted, cross-context, producer-policy-invalid, registration-mismatched, digest-mismatched, unavailable-dependency, or out-of-domain basis returns unknown; the receiving admission guard fails closed without rewriting unknown to false. Sealed MemberOf, MemberOfUndefined, EntitySetDefinition, RecordMembershipSourceV1, TrustedRecordMembershipSourceDeliveryV1, and RecordMembershipEvaluatorRegistration editions remain decode-and-replay compatibility objects only and cannot be emitted as the current FPF classification model; structural DecisionRecord classification grants no approval or authority.
    scope:
      - project-record-membership
      - membership-evaluator
    support_refs:
      - SS.procedural.typeenv.001.A4
    governing_pattern_refs:
      - C.3.2
      - A.7
```

## SS.constraints.typed-memory.001 Typed memory admissibility invariants

```yaml spec-section
id: SS.constraints.typed-memory.001
spec: software-system
system_frame: software_system
kind: software.constraints
title: Typed memory admissibility invariants
statement_type: admissibility
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.interfaces.memory.001
  - SS.procedural.typeenv.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.constraints.typed-memory.001.L1
    class: L
    statement: "Validation has exactly three semantic outcomes: Valid when every covered rule is satisfied and required basis exists, Invalid only on a positive contradiction with a known compiled rule or known codec contract, and Underdetermined when a signature, ContextKindAvailability, bridge, codec implementation, TypeEnv binding, source rule, reference, or compiler coverage is missing."
    scope:
      - validation-result
  - id: SS.constraints.typed-memory.001.A1
    class: A
    statement: A relation instance is admissible only when every required named slot has an allowed cardinality and exact ValueKind or RefKind; every referenced SlotKind, ValueKind, and RefKind has ContextKindAvailability in the relation's exact BoundedContext; every filler whose signature requires concrete kind classification has a separately evaluated KindClassificationJudgement=true for the exact candidate, local kind, KindSignature edition, and explicit current ContextSlice; the source-pinned signature is present and executable in exact C selected by the current ProjectTypeEnvHead; every explicit constraint in C is satisfied; and every ByValue filler is a verifier-created VerifiedTypedValue for the exact ValueKindRef, ValueShapeRef, and CodecRef binding in C. A false or unknown classification makes this receiving admission inadmissible but remains respectively false or unknown rather than becoming the guard result.
    scope:
      - relation-admission
    support_refs:
      - SS.procedural.typeenv.001.L1
      - SS.procedural.typeenv.001.L4
      - SS.functional.memory.001.L1
      - SS.functional.memory.001.L2
    governing_pattern_refs:
      - A.6.5
      - C.3.1
      - C.3.2
      - A.22.CGUS
  - id: SS.constraints.typed-memory.001.D1
    class: D
    statement: ProjectionService and host adapters must not report type conformance as truth, an Evidence relation, an admitted supporting episteme, authorization, causal force, work completion, or FPF pattern applicability.
    scope:
      - validation-semantics
    support_refs:
      - SS.allocation.001.L8
      - SS.allocation.001.L11
      - TS.boundary.001.L1
      - TS.boundary.001.L4
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.7
      - A.10
      - C.28
  - id: SS.constraints.typed-memory.001.L2
    class: L
    statement: Missing ContextKindAvailability, bridge, source rule, relation signature present and executable in C selected by the current ProjectTypeEnvHead, reference, codec implementation, ValueKind-to-ValueShape-and-Codec binding, or compiler coverage yields Underdetermined with repair diagnostics and never Valid or Invalid; unavailable context-kind basis has reason kind_unavailable_in_context, and a missing project-local signature has reason signature_not_in_selected_composite.
    scope:
      - open-world-uncertainty
  - id: SS.constraints.typed-memory.001.A2
    class: A
    statement: A ByValue filler is admissible only as a VerifiedTypedValue privately constructed after the pure verifier resolves the exact ValueKindRef, ValueShapeRef, and CodecRef binding in C selected by the current ProjectTypeEnvHead, executes the registered pure codec, verifies canonical round-trip and shape, and recomputes the domain-separated length-prefixed digest over the refs and canonical bytes. A caller flag, arbitrary JSON value, mutable codec identity, or registry presence alone is not admissible.
    scope:
      - by-value-admission
    support_refs:
      - SS.functional.memory.001.L6
      - SS.procedural.typeenv.001.L4
      - SS.procedural.typeenv.001.L5
    governing_pattern_refs:
      - A.6.5
      - C.2.1
  - id: SS.constraints.typed-memory.001.D2
    class: D
    statement: The supported C.2.1 EpistemeConstitutionRelation signature must require exactly one ClaimGraphSlot with ValueKind U.ClaimGraph, ByValue reference mode, the dedicated ClaimGraph ValueShape, and ClaimGraphCodecV1; other C.2.1 relation signatures retain only their source-declared participants and must not gain a ClaimGraphSlot by family-wide inference. The codec must canonicalize unordered node and edge sets, preserve explicitly governed order within ordered values, and reject duplicate node identities and dangling endpoints.
    scope:
      - claim-graph-codec
    support_refs:
      - SS.constraints.typed-memory.001.A2
      - SS.procedural.typeenv.001.L5
    governing_pattern_refs:
      - C.2.1
      - A.6.5
  - id: SS.constraints.typed-memory.001.L3
    class: L
    statement: Unknown codec implementation, binding absent from C selected by the current ProjectTypeEnvHead, or uncompiled rule is Underdetermined; known malformed canonical bytes, typed-value digest mismatch, duplicate or dangling ClaimGraph data, or explicit SlotSpec mismatch is Invalid; an unavailable historical codec yields OpaqueStoredValue for inspect-only reads and never a new admissible write.
    scope:
      - by-value-failure-posture
    support_refs:
      - SS.constraints.typed-memory.001.L1
      - SS.constraints.typed-memory.001.A2
      - SS.functional.memory.001.L7
    governing_pattern_refs:
      - A.7
      - A.10
```

## SS.procedural.memory-admission.001 Transactional semantic admission

```yaml spec-section
id: SS.procedural.memory-admission.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: Transactional semantic admission
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.constraints.typed-memory.001
  - SS.interfaces.memory.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.memory-admission.001.L1
    class: L
    statement: Admission is the product procedure that normalizes a ChangeSet, resolves the exact active TypeEnv, validates purely, checks authority, revalidates under a write transaction with graph-revision compare-and-swap, and commits one semantic event.
    scope:
      - admission-service
  - id: SS.procedural.memory-admission.001.A1
    class: A
    statement: Admit is eligible to write only for a Valid ChangeSet with exact active TypeEnv, matching expected graph revision, allowed authority class, and a syntactically valid idempotency key.
    scope:
      - admission-preconditions
    support_refs:
      - SS.procedural.memory-admission.001.L1
      - SS.constraints.typed-memory.001.L1
      - SS.interfaces.memory.001.L2
    governing_pattern_refs:
      - A.6.5
      - A.7
  - id: SS.procedural.memory-admission.001.D1
    class: D
    statement: AdmissionService must write no semantic rows for Invalid, Underdetermined, stale-TypeEnv, stale-graph, unauthorized, or malformed input and must return typed diagnostics for the violated or missing basis.
    scope:
      - admission-failure
    support_refs:
      - SS.allocation.001.L7
      - SS.procedural.memory-admission.001.A1
      - SS.constraints.typed-memory.001.L1
      - SS.constraints.typed-memory.001.L2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    governing_pattern_refs:
      - A.6.5
      - C.3.1
  - id: SS.procedural.memory-admission.001.A2
    class: A
    statement: Replay of a prior committed admit result is admissible iff the idempotency key and canonical ChangeSet digest both match that result; reuse of the same key with a different digest is inadmissible.
    scope:
      - idempotency
    support_refs:
      - SS.procedural.memory-admission.001.L1
      - SS.procedural.memory-admission.001.A1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    governing_pattern_refs:
      - A.6.5
      - A.7
  - id: SS.procedural.memory-admission.001.D2
    class: D
    statement: AdmissionService must enforce SS.procedural.memory-admission.001.A2 by replaying its admitted replay branch and returning its typed no-write rejection branch on a digest mismatch.
    scope:
      - idempotency
    support_refs:
      - SS.allocation.001.L7
      - SS.procedural.memory-admission.001.A2
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    governing_pattern_refs:
      - A.7
  - id: SS.procedural.memory-admission.001.D3
    class: D
    statement: AdmissionService must record assertions as append-only semantic events and must represent correction through explicit retraction or supersession rather than destructive reinterpretation.
    scope:
      - semantic-history
    support_refs:
      - SS.allocation.001.L7
      - SS.functional.memory.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.procedural.memory-admission.001.E1
    class: E
    statement: Under design-contract inspection on 2026-07-14, master-plan sections 4.1 through 4.3 and 6.2 carry the pure-validation, authority, transaction, idempotency, and no-write failure contract supporting SS.procedural.memory-admission.001.L1 and A1 for the human specification reviewer in the admission-design viewpoint; this observation is not an admitted supporting episteme for an Evidence relation about implementation.
    scope:
      - admission-design-evidence
    support_refs:
      - SS.procedural.memory-admission.001.L1
      - SS.procedural.memory-admission.001.A1
      - SS.constraints.typed-memory.001.L1
    evidence_refs:
      - "carrier:.context/haft-v9-typed-memory-e2e-master-plan.md#4.1"
      - "carrier:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.constraints.authority.001 Binding authority and effect boundaries

```yaml spec-section
id: SS.constraints.authority.001
spec: software-system
system_frame: software_system
kind: software.constraints
title: Binding authority and effect boundaries
statement_type: admissibility
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.procedural.memory-admission.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.constraints.authority.001.A1
    class: A
    statement: DecisionRecord creation or supersession, project-profile application, SpecSection lifecycle acts, incompatible ProjectTypeEnvTransition selection, and rollback are admissible only from a direct unambiguous operator request routed to the exact effect-specific interface. WorkCommission creation remains manual-only. Haft Core may perform the fixed package-default profile and Genesis effects plus an exact proven-compatible ProjectTypeEnv successor activation admitted by their deterministic policies. Compilation, staging, generic admission, an earlier skill token, request, or authority use cannot authorize another effect.
    scope:
      - binding-actions
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L3
      - SS.procedural.typeenv.001.A1
      - SS.interfaces.memory.001.A1
      - "legacy:ES.effect-boundaries.001"
      - "legacy:ES.agent-policy.001"
    governing_pattern_refs:
      - A.7
      - A.15
  - id: SS.constraints.authority.001.D1
    class: D
    statement: ProjectionService must keep Query, status, inspect, neighborhood, recall, coverage, interface discovery, validate, lifecycle projection, TypeEnv candidate build and verification, compatibility diff, revalidation, and Stage inspection read-only with respect to canonical project state; repeated observation or use never acquires project-head selection authority.
    scope:
      - read-only-surfaces
    support_refs:
      - SS.allocation.001.L8
      - TS.boundary.001.L1
      - SS.interfaces.query.001.L1
      - SS.interfaces.memory.001.L1
    governing_pattern_refs:
      - A.7
  - id: SS.constraints.authority.001.D2
    class: D
    statement: ProjectionService must record durable retryable projection debt after a post-commit carrier projection failure and must not roll back or conceal the semantic commit.
    scope:
      - projection-boundary
    support_refs:
      - SS.allocation.001.L8
      - SS.procedural.memory-admission.001.L1
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.2"
    governing_pattern_refs:
      - A.7
      - B.3
  - id: SS.constraints.authority.001.A2
    class: A
    statement: A standard or deep DecisionRecord is admissible for binding only when it carries a stable problem basis, canonical option set, selected result, comparison basis, declared choice rule, rationale and counterargument, rejected alternatives, invariants, evidence requirements, weakest link, rollback, refresh triggers, implementation footprint, predictions, and one exact host-routed operator request matching the binding payload, subject and project scope.
    scope:
      - decision-record-close-contract
    support_refs:
      - SS.allocation.001.L1
      - "code:internal/artifact/decision.go"
      - "code:internal/cli/serve.go"
      - "code:internal/cli/mcp_binding.go"
      - "legacy:ES.work-methods.001"
    governing_pattern_refs:
      - A.7
      - A.15
      - A.10
  - id: SS.constraints.authority.001.L2
    class: L
    statement: HostRoutedOperatorRequest is an effect-specific provenance record stating that the host recognized a direct operator request. It binds effect kind, subject reference, exact payload digest and request digest. It does not model hidden operator intent, independently prove U.SpeechAct, or acquire authority from a skill token. Project-local authority mode switches do not exist.
    scope:
      - operator-request-provenance
      - authority-generation
    support_refs:
      - SS.allocation.001.L1
      - SS.interfaces.hosts.001.L1
    governing_pattern_refs:
      - A.2.9
      - A.7
  - id: SS.constraints.authority.001.A3
    class: A
    statement: DecisionRecord binding is admissible when the operator directly and unambiguously requests one exact effect, subject, selected option and scope, and the host-routed request digest matches the exact reviewed decision payload. The CLI input-file command is an internal effect sink and binds without a second confirmation. A manual h-decide invocation is only a compatible route hint.
    scope:
      - decision-binding-policy
      - host-routed-operator-request
    support_refs:
      - SS.constraints.authority.001.A1
      - SS.constraints.authority.001.L2
      - SS.interfaces.hosts.001.D2
    governing_pattern_refs:
      - A.2.9
      - A.7
      - A.15
  - id: SS.constraints.authority.001.A4
    class: A
    statement: MCP DecisionRecord binding remains fail-closed with operator_confirmation_required because current MCP input cannot prove host conversation provenance. The current CLI has no terminal phrase, authority mode switch, or decision-resume surface. Project-local `.haft/config.yaml` is ignored; init removes only the exact known generated authority-only carrier after digest revalidation and preserves all changed or unrecognized bytes with a legacy warning.
    scope:
      - mcp-binding-boundary
      - legacy-project-config
    support_refs:
      - SS.constraints.authority.001.A1
      - SS.constraints.authority.001.L2
    governing_pattern_refs:
      - A.2.9
      - A.7
      - A.15
  - id: SS.constraints.authority.001.A5
    class: A
    statement: "A ProjectTypeEnvTransition is admissible through exactly one of two disjoint current authority generations. The compatible_successor_policy generation automatically admits the exact bundled successor only when the transaction-current request has a Transition predecessor, the project/root binding and Stage are exact, existing-assertion revalidation is clean, the project profile is Compatible, no installed projection profile is blocked, the authorization content is current, and the head, graph, profile, runtime, B, ordered E DAG, X and C still match at CAS time; it creates no HostRoutedOperatorRequest or human-review claim. The host_routed_operator_request generation remains available for rollback or explicit selection outside that predicate and requires one exact direct operator request whose effect, subject and payload digest bind the same request and content. Both generations produce distinct append-only resolutions and single-use authority-use records, and the current activation delta plus typed-memory graph event must record the same exact generation as the authority-use record. Legacy manual_type_env_activation activation carriers and graph rows remain decode-and-replay history only. Incompatible, incomplete, stale, or underdetermined automatic attempts write no head. Quotations, recommendations, tool output, visible hashes, staged rows, another lifecycle act, or an earlier request authorize neither branch."
    scope:
      - project-typeenv-head-selection-authority
      - project-typeenv-transition
    support_refs:
      - SS.constraints.authority.001.A1
      - SS.procedural.typeenv.001.A1
      - SS.procedural.typeenv.001.L4
    governing_pattern_refs:
      - A.2.1
      - A.2.9
      - A.15.1
      - A.7
  - id: SS.constraints.authority.001.A6
    class: A
    statement: Default ProjectTypeEnvGenesis is admissible only as an idempotent internal haft init effect for a project with no current head, using the exact package-bundled B, ordered E DAG, X, verified C, clean assertion revalidation, and a compatible or underdetermined current profile assessment. The effect must reprove project identity, Genesis absence, graph revision, installed source/runtime exactness, and committed readiness before init succeeds. The same init reconciliation must automatically select an already-headed bundled successor only through SS.constraints.authority.001.A5's exact compatible-successor predicate. Neither path exposes a memory schema, enable/defer choice, review carrier, DecisionRecord, SpeechAct claim, or authority for incompatible selection or rollback.
    scope:
      - default-project-memory-genesis
      - haft-init
    support_refs:
      - SS.interfaces.memory.001.L3
      - SS.procedural.typeenv.001.A1
      - "code:internal/cli/init_default_memory.go"
    governing_pattern_refs:
      - A.7
      - A.15.1
  - id: SS.constraints.authority.001.D3
    class: D
    statement: >-
      The dedicated TypeEnv-head transaction must branch on the authorization content's single-use key and exact request/content digest before current-authority revalidation or consumption. Exact replay verifies and returns the existing closure byte-identically without another authority use, Work occurrence, head mutation, or semantic write; a digest mismatch is ReplayConflict with zero writes. For an absent key, the transaction revalidates Stage identity, project, C, snapshot, graph and profile revisions, compatibility and assertions, then resolves exactly one current authority generation: HostRoutedOperatorRequest plus HostRoutedSelectionResolution, or CompatibleSuccessorPolicy plus CompatibleSuccessorResolution. It reproves Genesis absence or compares Transition's exact predecessor; performs exactly one CAS Work; records one generation-specific authority use; updates the head; and atomically finalizes the Work record, history, receipt and closure or writes none. Authorization content, authority resolution, authority use, CAS Work, head and receipt remain distinct; the compatible branch has no operator request or review claim, and no skill token or carrier substitutes for a missing basis. Legacy authority generations remain readable history but cannot be inserted or reused for current selection.
    scope:
      - project-typeenv-head-selection-effect
      - project-typeenv-authority-use
    support_refs:
      - SS.constraints.authority.001.A6
      - SS.constraints.authority.001.A5
      - SS.procedural.typeenv.001.A1
      - SS.allocation.001.L7
    governing_pattern_refs:
      - A.2.1
      - A.15.1
      - A.7
  - id: SS.constraints.authority.001.L1
    class: L
    statement: WorkCommission completion projection distinguishes completed from completed_with_projection_debt and carries a retryable projection-debt payload for the latter state.
    scope:
      - workcommission-projection-debt
  - id: SS.constraints.authority.001.E1
    class: E
    statement: Under current-worktree source inspection and WorkCommission projection tests on 2026-07-14, the completion carrier instantiates SS.constraints.authority.001.L1 for the human specification reviewer in the effect-boundary viewpoint; this observation is not an admitted supporting episteme for an Evidence relation about the future typed-memory projection adapter.
    scope:
      - projection-debt-evidence
    support_refs:
      - SS.constraints.authority.001.L1
    evidence_refs:
      - "carrier:internal/workcommission/lifecycle.go#StateCompletedWithProjectionDebt"
      - "carrier:internal/workcommission/projection_test.go"
      - "carrier:internal/cli/serve_commission_test.go"
      - "command:go test ./internal/workcommission ./internal/cli -run 'ProjectionDebt|Completion' -count=1"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.procedural.commission.001 WorkCommission product lifecycle

```yaml spec-section
id: SS.procedural.commission.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: WorkCommission product lifecycle
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.constraints.authority.001
  - SS.allocation.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.commission.001.L1
    class: L
    statement: A WorkCommission is a bounded authorization record for attempted work and remains distinct from the selected DecisionRecord, intended WorkPlan, performed RuntimeRun, observations or supporting epistemes produced by that run, and any Evidence relation or EvidenceRecord subsequently recorded from them.
    scope:
      - work-commission
  - id: SS.procedural.commission.001.A1
    class: A
    statement: A WorkCommission is admissible for creation only after explicit human invocation against an eligible DecisionRecord and with explicit allowed paths, forbidden paths, lockset, evidence requirements, validity window, delivery policy, and autonomy-envelope snapshot.
    scope:
      - commission-creation
    support_refs:
      - "legacy:ES.commission-policy.001"
      - "code:internal/workcommission/lifecycle.go"
      - SS.procedural.commission.001.L1
      - SS.constraints.authority.001.A1
    governing_pattern_refs:
      - A.15
      - A.15.2
  - id: SS.procedural.commission.001.A4
    class: A
    statement: Runnable intake of a WorkCommission is admissible iff its state is queued or ready; its validity window has not expired; its source DecisionRecord, problem, and specification revisions are fresh; its scope, lockset, lease, and autonomy-envelope checks pass; and no required human checkpoint is open.
    scope:
      - commission-runnability
    support_refs:
      - SS.procedural.commission.001.L1
      - "legacy:ES.commission-policy.001"
      - "code:internal/cli/commission.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.procedural.commission.001.A5
    class: A
    statement: Repair or override after SS.procedural.commission.001.A4 rejects a WorkCommission is admissible only through the dedicated interface when the request identifies each failed A4 predicate, supplies the replacement basis or bounded override, and carries the required operator authority receipt.
    scope:
      - commission-repair
    support_refs:
      - SS.procedural.commission.001.A4
      - SS.allocation.001.L1
      - "legacy:ES.commission-policy.001"
      - "code:internal/cli/commission.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.procedural.commission.001.D1
    class: D
    statement: CommissionService must stop commission creation before execution and must record claim, requeue, cancel, retirement, and result attachment as explicit audited lifecycle transitions.
    scope:
      - commission-lifecycle
    support_refs:
      - SS.allocation.001.L10
      - "legacy:ES.commission-policy.001"
      - "code:internal/workcommission/lifecycle.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.procedural.commission.001.D2
    class: D
    statement: CommissionService must keep a WorkCommission non-runnable unless SS.procedural.commission.001.A4 is satisfied and must route repair or override through SS.procedural.commission.001.A5 with exact failed-predicate diagnostics.
    scope:
      - commission-intake
    support_refs:
      - SS.allocation.001.L10
      - SS.procedural.commission.001.A4
      - SS.procedural.commission.001.A5
      - "legacy:ES.commission-policy.001"
      - "code:internal/cli/commission.go"
    governing_pattern_refs:
      - A.15
  - id: SS.procedural.commission.001.L2
    class: L
    statement: The default commission scope is derived from the source DecisionRecord affected files and module context.
    scope:
      - commission-scope
  - id: SS.procedural.commission.001.A2
    class: A
    statement: A widened allowed path, forbidden-path exception, or additional commission slice is admissible only when its operator-visible scope text identifies the widening.
    scope:
      - commission-scope-widening
    support_refs:
      - SS.allocation.001.L1
      - SS.procedural.commission.001.L2
      - "code:internal/cli/serve_commission.go"
    governing_pattern_refs:
      - A.15
  - id: SS.procedural.commission.001.L3
    class: L
    statement: DeliveryPolicy distinguishes workspace_patch_manual from workspace_patch_auto_on_pass; the project-selected default is policy outside this SoftwareSystemSpec.
    scope:
      - commission-delivery-policy
  - id: SS.procedural.commission.001.A3
    class: A
    statement: Automatic local apply is admissible only for workspace_patch_auto_on_pass when the current apply-time autonomy envelope, scope, checks, and delivery decision all admit it.
    scope:
      - commission-auto-apply
    support_refs:
      - SS.procedural.commission.001.L3
      - "code:internal/workcommission/lifecycle.go"
      - "code:internal/cli/harness.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.procedural.commission.001.L4
    class: L
    statement: A local workspace apply is a discrete reversible local git effect and does not imply remote push, pull-request, merge, or publication authority.
    scope:
      - commission-local-apply
```

## SS.interfaces.runtime.001 Commission and runtime interfaces

```yaml spec-section
id: SS.interfaces.runtime.001
spec: software-system
system_frame: software_system
kind: software.interfaces
title: Commission and runtime interfaces
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.procedural.commission.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.interfaces.runtime.001.L1
    class: L
    statement: Dedicated commission interfaces expose creation, inspection, claim, requeue, cancel, and lifecycle results; harness interfaces expose run, bounded drain, status, result inspection, and local apply behavior.
    scope:
      - cli
      - mcp
  - id: SS.interfaces.runtime.001.D1
    class: D
    statement: Host adapters must expose stable identities, source DecisionRecord and WorkCommission references, lifecycle state, scope posture, lease or lock posture when relevant, supporting-episteme, Evidence-relation, and EvidenceRecord references, workspace reference, and recoverable operations in commission and RuntimeRunRecord results.
    scope:
      - runtime-result-contract
    support_refs:
      - SS.allocation.001.L11
      - "code:internal/cli/serve_commission.go"
      - "code:internal/cli/harness.go"
      - "code:internal/workcommission/projection.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.interfaces.runtime.001.D2
    class: D
    statement: Host adapters must keep commission execution, local workspace apply, and remote publication as distinct effects and must not present a RuntimeRunRecord as remote publication authority.
    scope:
      - effect-boundary
    support_refs:
      - SS.allocation.001.L11
      - SS.procedural.commission.001.L4
      - SS.constraints.authority.001.A1
      - "legacy:ES.runtime-policy.001"
    governing_pattern_refs:
      - A.7
      - A.15
  - id: SS.interfaces.runtime.001.L2
    class: L
    statement: A human operator acts through CLI commands to start, observe, stop, recover, apply, requeue, or cancel runtime work; the MCP adapter exposes bounded typed lifecycle operations and projections to an authorized caller. The CLI and MCP surfaces are interfaces rather than owners or performers of that work, and the MCP adapter is not an unattended long-running executor.
    scope:
      - runtime-surface-ownership
    support_refs:
      - SS.allocation.001.L1
      - SS.allocation.001.L4
      - SS.allocation.001.L10
      - SS.allocation.001.L11
  - id: SS.interfaces.runtime.001.L3
    class: L
    statement: Single-commission `haft harness run` is the default execution surface; queue drain is an explicit opt-in `--drain` mode with bounded concurrency.
    scope:
      - runtime-entry-mode
```

## SS.constraints.runtime.001 Runtime admission and isolation constraints

```yaml spec-section
id: SS.constraints.runtime.001
spec: software-system
system_frame: software_system
kind: software.constraints
title: Runtime admission and isolation constraints
statement_type: admissibility
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.profile.001
  - SS.interfaces.runtime.001
  - SS.procedural.commission.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.constraints.runtime.001.L1
    class: L
    statement: Each RuntimeRun is interpreted in a bounded context that identifies exactly one admitted WorkCommission, one isolated workspace or worktree, one source checkout state, and one execution-attempt identity.
    scope:
      - runtime-bounded-context
  - id: SS.constraints.runtime.001.A1
    class: A
    statement: Runtime startup for a WorkCommission is admissible only when SS.procedural.commission.001.A4 admits that same WorkCommission in the bounded execution context defined by SS.constraints.runtime.001.L1.
    scope:
      - runtime-start
    support_refs:
      - SS.procedural.commission.001.A4
      - SS.constraints.runtime.001.L1
      - "legacy:ES.runtime-policy.001"
      - "code:internal/cli/harness.go"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.constraints.runtime.001.A2
    class: A
    statement: A drain-mode commission start is admissible only under the same per-commission admission contract used by single-run mode and the declared concurrency bound.
    scope:
      - runtime-drain
    support_refs:
      - SS.constraints.runtime.001.A1
      - SS.interfaces.runtime.001.L3
      - "code:internal/cli/harness.go"
    governing_pattern_refs:
      - A.15
  - id: SS.constraints.runtime.001.L2
    class: L
    statement: Normal runtime observation and recovery are available through typed lifecycle interfaces and do not require direct SQLite mutation or process-kill access.
    scope:
      - runtime-recovery-boundary
```

## SS.procedural.runtime.001 OpenSleigh RuntimeRun lifecycle

```yaml spec-section
id: SS.procedural.runtime.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: OpenSleigh RuntimeRun lifecycle
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.constraints.runtime.001
  - SS.interfaces.runtime.001
  - SS.constraints.authority.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.runtime.001.L1
    class: L
    statement: A RuntimeRun is the dated Work occurrence in which OpenSleigh executes one admitted WorkCommission in an isolated workspace; it remains distinct from the authorization record.
    scope:
      - runtime-run
  - id: SS.procedural.runtime.001.L2
    class: L
    statement: A RuntimeRunRecord is an addressable project-memory description of one RuntimeRun and is not the dated Work occurrence itself; JSON documents, Markdown documents, database rows, and UI projections that represent or publish the record are carriers distinct from both the record and the Work occurrence.
    scope:
      - runtime-run-record
  - id: SS.procedural.runtime.001.L3
    class: L
    statement: A terminal RuntimeRunRecord identifies the RuntimeRun, executed WorkCommission, bounded context, workspace or worktree, source checkout state, execution-attempt identity, start and end timestamps, provenance, observed changes, check verdicts, terminal or failure state, and available recovery or local-apply operation.
    scope:
      - runtime-terminal-record-shape
    support_refs:
      - SS.procedural.runtime.001.L2
      - SS.constraints.runtime.001.L1
  - id: SS.procedural.runtime.001.L4
    class: L
    statement: A WorkspacePatchApply is a discrete reversible local effect whose inputs include a patch reference, RuntimeRunRecord reference, delivery-policy decision, and current-checkout state observation; it is neither the RuntimeRun nor its record and carries no remote publication authority.
    scope:
      - runtime-apply-effect
    support_refs:
      - SS.procedural.commission.001.L4
  - id: SS.procedural.runtime.001.D1
    class: D
    statement: CommissionService must submit a terminal RuntimeRunRecord conforming to SS.procedural.runtime.001.L3 through AdmissionService from validated OpenSleigh observation carriers and commission lifecycle events.
    scope:
      - runtime-terminal-result
    support_refs:
      - SS.allocation.001.L10
      - SS.allocation.001.L7
      - SS.procedural.runtime.001.L2
      - SS.procedural.runtime.001.L3
      - SS.procedural.memory-admission.001.A1
      - "legacy:ES.runtime-policy.001"
      - SS.interfaces.runtime.001.D1
    governing_pattern_refs:
      - A.15.1
      - A.10
  - id: SS.procedural.runtime.001.D2
    class: D
    statement: CommissionService must preserve the WorkspacePatchApply boundary in SS.procedural.runtime.001.L4 and must not admit automatic local apply unless SS.procedural.commission.001.A3 is satisfied.
    scope:
      - runtime-apply
    support_refs:
      - SS.allocation.001.L10
      - SS.procedural.runtime.001.L4
      - SS.procedural.commission.001.L3
      - SS.procedural.commission.001.A3
      - SS.interfaces.runtime.001.L2
      - "legacy:ES.runtime-policy.001"
    governing_pattern_refs:
      - A.15
      - A.10
  - id: SS.procedural.runtime.001.E1
    class: E
    statement: Under current-worktree source inspection and focused WorkCommission and harness tests on 2026-07-14, RuntimeStatusWriter source defines a JSON status carrier, and the focused harness tests observe a scope-checked local git apply as an effect distinct from runtime execution; these observations support the carrier distinction in SS.procedural.runtime.001.L2 and the separate-local-effect portion of L4 for the human specification reviewer in the runtime-contract viewpoint. They are not admitted supporting epistemes for Evidence relations about existence, admission, or the complete L3 shape of a RuntimeRunRecord, an actual RuntimeRun, the D1 or D2 duties, remote-publication authority, or production-code drain readiness.
    scope:
      - runtime-contract-evidence
    support_refs:
      - SS.procedural.runtime.001.L2
      - SS.procedural.runtime.001.L4
    evidence_refs:
      - "carrier:open-sleigh/lib/open_sleigh/runtime_status_writer.ex"
      - "carrier:internal/cli/harness.go"
      - "carrier:internal/cli/harness_test.go"
      - "carrier:internal/workcommission/lifecycle_test.go"
      - "command:go test ./internal/workcommission ./internal/cli -run 'WorkCommission|Harness|Projection' -count=1"
    valid_until: 2026-07-21
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.functional.evidence-recording.001 Typed evidence recording

```yaml spec-section
id: SS.functional.evidence-recording.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: Typed evidence recording
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.allocation.001
  - SS.functional.memory.001
  - SS.procedural.memory-admission.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.functional.evidence-recording.001.L1
    class: L
    statement: An EvidenceRecord carries a stable record ID, supporting-episteme reference, target-claim reference, BoundedContextRef, provenance, polarity, freshness boundary, assessment, and inspectable carrier reference when one exists.
    scope:
      - evidence-record
  - id: SS.functional.evidence-recording.001.L2
    class: L
    statement: The verification projection keeps baseline, measurement, EvidenceRecord, and recorded verdict as distinct linked values in that order of justification; the sequence does not claim that the carrier performed the observation.
    scope:
      - verification-projection
  - id: SS.functional.evidence-recording.001.L3
    class: L
    statement: Evidence is a contextual evidence-use relation between an admitted supporting episteme and a target claim; EvidenceRecord describes that relation and is neither the supporting episteme nor Evidence itself.
    scope:
      - evidence-carrier-boundary
  - id: SS.functional.evidence-recording.001.A1
    class: A
    statement: An EvidenceRecord is admissible only when its supporting episteme, target claim, BoundedContextRef, polarity, provenance, and freshness resolve and its assessment tier matches the intended evidence use.
    scope:
      - evidence-attachment
    support_refs:
      - SS.functional.evidence-recording.001.L1
      - SS.functional.evidence-recording.001.L3
      - "legacy:ES.evidence-policy.001"
    governing_pattern_refs:
      - A.2.4
      - A.10
      - B.3
  - id: SS.functional.evidence-recording.001.D1
    class: D
    statement: EvidenceService must preserve enough observation input, command or carrier reference, environment basis, and observed output to rerun or independently inspect the recorded observation when the observation kind permits replay.
    scope:
      - evidence-replay
    support_refs:
      - SS.allocation.001.L12
      - SS.functional.evidence-recording.001.L1
      - "legacy:ES.evidence-policy.001"
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.functional.evidence-recording.001.D2
    class: D
    statement: EvidenceService must attach the EvidenceRecord to the exact governed claim or artifact rather than treating an adjacent successful check as project-wide proof.
    scope:
      - evidence-subject-binding
    support_refs:
      - SS.allocation.001.L12
      - SS.functional.evidence-recording.001.A1
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.constraints.evidence.001 Provenance, freshness, and evidence constraints

```yaml spec-section
id: SS.constraints.evidence.001
spec: software-system
system_frame: software_system
kind: software.constraints
title: Provenance, freshness, and evidence constraints
statement_type: admissibility
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.functional.recall.001
  - SS.functional.evidence-recording.001
  - SS.procedural.memory-admission.001
  - SS.procedural.runtime.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.constraints.evidence.001.L1
    class: L
    statement: An observation or admitted episteme supplies potential basis; Evidence is the explicit contextual evidence-use relation linking an admitted supporting episteme to a target claim with provenance, polarity, and freshness, while EvidenceRecord describes that relation.
    scope:
      - evidence
  - id: SS.constraints.evidence.001.A1
    class: A
    statement: An evidence-use relation is admissible for stronger use only when the supporting episteme, target claim, BoundedContextRef, polarity, provenance, freshness, and assessment tier match the intended use.
    scope:
      - evidence-admission
    support_refs:
      - SS.constraints.evidence.001.L1
      - SS.functional.evidence-recording.001.L1
      - SS.functional.evidence-recording.001.L3
      - "legacy:ES.evidence-policy.001"
      - TS.boundary.001.L4
    governing_pattern_refs:
      - A.2.4
      - A.10
      - B.3
  - id: SS.constraints.evidence.001.D1
    class: D
    statement: EvidenceService must preserve the exact TypeEnv, compiler rule, source unit, PatternID, revision, content hash, and source range supplied by validation and admission diagnostics when those fields exist.
    scope:
      - semantic-diagnostics
    support_refs:
      - SS.allocation.001.L12
      - SS.procedural.typeenv.001.L1
      - SS.constraints.typed-memory.001.L1
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.constraints.evidence.001.D2
    class: D
    statement: EvidenceService must reject a WorkCommission, WorkPlan, carrier, or test description as sufficient by itself to admit an Evidence relation supporting claims of actual commission execution, workspace effects, local apply behavior, or external effects.
    scope:
      - runtime-evidence
    support_refs:
      - SS.allocation.001.L12
      - SS.constraints.evidence.001.A1
      - SS.procedural.runtime.001.L1
      - "legacy:ES.evidence-policy.001"
    governing_pattern_refs:
      - A.15.1
      - A.15.2
      - A.10
  - id: SS.constraints.evidence.001.D3
    class: D
    statement: EvidenceService must surface expired or drifted support as refresh, reopen, revalidation, or supersession need instead of silently treating it as current or averaging it away.
    scope:
      - evidence-freshness
    support_refs:
      - SS.allocation.001.L12
      - "legacy:ES.evidence-policy.001"
      - "plan:.context/haft-v9-typed-memory-e2e-master-plan.md#6.1"
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.procedural.evidence-refresh.001 Evidence freshness and refresh

```yaml spec-section
id: SS.procedural.evidence-refresh.001
spec: software-system
system_frame: software_system
kind: software.procedural_behavior
title: Evidence freshness and refresh
statement_type: duty
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.constraints.evidence.001
  - SS.functional.evidence-recording.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.procedural.evidence-refresh.001.L1
    class: L
    statement: Currentness resolves separately for an Evidence evidence-use relation and its admitted supporting episteme from their validity boundaries, governed-target state, and provenance; an EvidenceRecord records or describes the relation and is neither Evidence nor the supporting basis.
    scope:
      - evidence-freshness
  - id: SS.procedural.evidence-refresh.001.D1
    class: D
    statement: EvidenceService must evaluate the currentness of the Evidence relation and its admitted supporting episteme before admitting stronger reuse under SS.procedural.evidence-refresh.001.A1.
    scope:
      - evidence-reuse
    support_refs:
      - SS.allocation.001.L12
      - SS.functional.evidence-recording.001.L1
      - SS.procedural.evidence-refresh.001.L1
      - SS.procedural.evidence-refresh.001.A1
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.procedural.evidence-refresh.001.A1
    class: A
    statement: Continued stronger use is admissible only while both the Evidence relation and its admitted supporting episteme remain current for the exact target claim and BoundedContextRef, or after a fresh supporting episteme is admitted and a replacement Evidence relation is established; an EvidenceRecord alone is inadmissible as a substitute because it only records or describes that relation.
    scope:
      - evidence-continued-use
    support_refs:
      - SS.constraints.evidence.001.L1
      - SS.constraints.evidence.001.A1
      - SS.functional.evidence-recording.001.A1
      - SS.procedural.evidence-refresh.001.L1
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.procedural.evidence-refresh.001.D2
    class: D
    statement: EvidenceService must require fresh admitted supporting epistemes and contract-matched Evidence relations before a new runtime mode, host integration, public interface, or autonomy mode is presented as supported.
    scope:
      - capability-evidence-refresh
    support_refs:
      - SS.allocation.001.L12
      - SS.procedural.evidence-refresh.001.A1
      - SS.interfaces.hosts.001.A1
      - SS.constraints.runtime.001.A1
      - "legacy:ES.evidence-policy.001"
    governing_pattern_refs:
      - A.10
      - B.3
  - id: SS.procedural.evidence-refresh.001.D3
    class: D
    statement: EvidenceService must expose refresh, reopen, revalidation, or supersession as distinct repair postures for expired or drifted support and must preserve the prior record as history.
    scope:
      - evidence-repair
    support_refs:
      - SS.allocation.001.L12
      - SS.procedural.evidence-refresh.001.L1
      - SS.procedural.memory-admission.001.D3
    governing_pattern_refs:
      - A.10
      - B.3
```

## SS.functional.status.001 Compact status and exact drill-down

```yaml spec-section
id: SS.functional.status.001
spec: software-system
system_frame: software_system
kind: software.functional_behavior
title: Compact status and exact drill-down
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.interfaces.profile.001
  - SS.procedural.spec-lifecycle.001
  - SS.procedural.evidence-refresh.001
  - SS.functional.memory.001
target_refs:
  - TS.boundary.001
claims:
  - id: SS.functional.status.001.L1
    class: L
    statement: BaselineKind distinguishes SpecSectionApprovalBaseline, PreWorkReferenceSnapshot, VerifiedStateSnapshot, and UnknownLegacyBaseline without coercing one meaning into another.
    scope:
      - baseline-kind
  - id: SS.functional.status.001.L2
    class: L
    statement: Project status is a simultaneous projection of open concerns, alternatives, decisions, active work, specification applicability and lifecycle, Evidence-relation and supporting-episteme currentness, drift, commissions, and projection debt rather than one global phase.
    scope:
      - status-facets
  - id: SS.functional.status.001.D1
    class: D
    statement: ProjectionService must keep the default status compact and operator-actionable while exposing BaselineKind, exact artifact identity, provenance, and full finding detail through drill-down projections.
    scope:
      - compact-status
    support_refs:
      - SS.allocation.001.L8
      - SS.functional.status.001.L1
      - "code:internal/project/specflow/baseline.go"
      - "code:internal/overseer/status.go"
    governing_pattern_refs:
      - A.7
      - A.10
  - id: SS.functional.status.001.D2
    class: D
    statement: ProjectionService must keep module decision coverage, exact file-link gaps, interface discovery, and lifecycle inspection read-only and must label stale code-index or incomplete-basis limits.
    scope:
      - status-read-projections
    support_refs:
      - SS.allocation.001.L8
      - SS.constraints.authority.001.D1
      - "code:internal/project/speccoverage.go"
      - "code:internal/cli/interface.go"
    governing_pattern_refs:
      - A.7
      - A.10
  - id: SS.functional.status.001.L3
    class: L
    statement: A status-local next-action field is scoped to one named method, diagnostic, or lifecycle result; the status model has no terminal DECIDED state and no universal project NextAction.
    scope:
      - status-navigation
```

## SS.structure.001 Core-first layered product structure

```yaml spec-section
id: SS.structure.001
spec: software-system
system_frame: software_system
kind: software.selected_structure
title: Core-first layered product structure
statement_type: definition
claim_layer: object
owner: human
status: active
valid_until: 2026-10-31
depends_on:
  - SS.interfaces.profile.001
  - SS.interfaces.query.001
  - SS.interfaces.hosts.001
  - SS.procedural.spec-lifecycle.001
  - SS.procedural.typeenv.001
  - SS.procedural.memory-admission.001
  - SS.constraints.authority.001
  - SS.constraints.runtime.001
  - SS.constraints.evidence.001
  - SS.procedural.evidence-refresh.001
  - SS.functional.status.001
target_refs:
  - TS.role.001
  - TS.boundary.001
claims:
  - id: SS.structure.001.L1
    class: L
    statement: Haft uses a pure immutable algebraic core for profile, identifiers, kinds, relation signatures, named slot bindings, ChangeSets, TypeEnv, validation, and result unions; side effects enter only through outer adapters.
    scope:
      - functional-core
  - id: SS.structure.001.L2
    class: L
    statement: The selected layers are algebraic domain types, immutable TypeEnv, pure normalization and validation, AdmissionService orchestration, persistence and source adapters, task-oriented projections, and CLI, MCP, skill, and host surfaces.
    scope:
      - product-layers
  - id: SS.structure.001.L5
    class: L
    statement: Each selected product layer depends only on the layer directly below it; outer surfaces, graph projections, Markdown carriers, and SQLite adapters have no bypass around AdmissionService and no authority to reinterpret semantic rules.
    scope:
      - dependency-rule
  - id: SS.structure.001.L3
    class: L
    statement: "The FPF source index and the TypeEnv compiler share exact source-unit provenance infrastructure but remain distinct products: one supports navigation and one supplies an explicit admission basis."
    scope:
      - fpf-source-products
  - id: SS.structure.001.L6
    class: L
    statement: Canonical new semantic writes use one AdmissionService and additive typed SQLite tables; legacy artifacts remain dual-read compatibility inputs, new writes are single-write-new, and unresolved legacy meanings remain explicit.
    scope:
      - persistence-migration
  - id: SS.structure.001.L4
    class: L
    statement: OpenSleigh is the shipped execution-mechanics component of HaftSoftwareSystem; Haft Core is an internal component that implements core semantic, TypeEnv, admission, commission, and project-memory responsibilities. Neither component is a second target system or ProjectGovernanceSubstrate role holder.
    scope:
      - runtime-allocation
  - id: SS.structure.001.L7
    class: L
    statement: The canonical v1 runtime has no dependency on TypeDB, RDF, Datalog, a writable free-edge graph, or a second handwritten Haft ontology of FPF rules.
    scope:
      - architecture-exclusions
```
