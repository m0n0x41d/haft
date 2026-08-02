# Haft v9 Term Map — active carrier

This is the current term-map carrier for the profile-aware
`ProjectSpecificationSet` and typed project-memory contract. Its entries define
load-bearing vocabulary but do not approve a SpecSection, select a
`ProjectTypeEnvHead`,
bind a DecisionRecord, authorize a migration, or prove implementation. They
preserve still-valid vocabulary, repair stale fixed target/enabling
assumptions, and name the terms used by the current `SoftwareSystemSpec`.

```yaml term-map
status: active
entries:
  - term: Target system
    category: fpf-boundary
    definition: >-
      A system selected as the target of a current project relation: the system
      whose externally meaningful characteristics or behavior the project is
      intended to change or realize. The relation must be explicit; a repository,
      specification carrier, or software implementation is not automatically the
      target system.
    aliases:
      - target-system
    not:
      - The host coding agent.
      - The implementation plan.
      - Every EntityOfConcern.
    owners:
      - human
  - term: Enabling system
    category: fpf-boundary
    definition: >-
      A system or system arrangement that provides capabilities used to create,
      change, operate, or sustain a target system in a stated bounded context. It
      is distinct from both the target system and descriptions or carriers of
      either system.
    aliases:
      - enabling-system
    not:
      - The governed object merely because work is performed on it.
      - A synonym for SoftwareSystemSpec.
    owners:
      - human
  - term: ProjectGovernanceSubstrate
    category: haft-role
    definition: >-
      A project-local U.Role value held by HaftSoftwareSystem in one exact
      project BoundedContext. HaftSoftwareSystem is the stable identity of the
      whole shipped software system; Haft Core is an internal component of that
      system, not an alias for the whole system and not a separate holder of this
      role. While HaftSoftwareSystem holds the role, source-native FPF retrieval,
      validation, typed project memory, and projection are capabilities of that
      system-in-context. The role may require those capabilities as qualification
      conditions, but the role itself does not own, supply, or perform them.
    aliases:
      - Governance substrate
      - substrate
    not:
      - A coding agent.
      - A CI runner.
      - A product manager.
      - HaftSoftwareSystem itself.
      - Haft Core or another implementation component.
      - A container that owns capabilities.
    owners:
      - human
  - term: Carrier
    category: authority
    definition: >-
      A material or digital representation that carries a description for
      reading, transfer, parsing, or publication. A carrier is distinct from the
      described object, the episteme it presents, and the authority that admits
      or approves a claim.
    aliases:
      - markdown carrier
    not:
      - Authority by itself.
      - The described object.
      - An acting system.
    owners:
      - human
  - term: SpecSection
    category: specification
    definition: >-
      An addressable typed specification claim unit parsed from an applicable
      specification carrier and governed by explicit lifecycle checks. Its kind,
      claim class, dependencies, and applicability determine its place in a
      ProjectSpecificationSet; it does not inherently belong to a fixed
      target/enabling pair. Its depends_on references name other SpecSections
      whose specification or support claims are required to interpret and review
      this section; they do not prescribe execution.
    aliases:
      - spec section
    not:
      - Freeform markdown prose.
      - Proof that the described behavior exists at runtime.
      - A causal or chronological order inferred from depends_on.
      - A project-work or MethodDescription step order inferred from depends_on.
      - A global next-action order inferred from depends_on.
    owners:
      - human
  - term: Baseline
    category: specification
    definition: >-
      A recorded approved digest of an active SpecSection edition used to detect
      later drift without treating the carrier as proof of runtime behavior.
    aliases:
      - SpecSectionBaseline
    not:
      - A casual review comment.
      - Runtime evidence.
    owners:
      - human
  - term: DecisionRecord
    category: reasoning
    definition: >-
      A human-gated binding artifact that records a bounded choice, its option
      set, selection basis, rationale, consequences, evidence, and return or
      refresh conditions.
    aliases:
      - DRR
      - decision record
    not:
      - An agent recommendation.
      - A retrieval result.
    owners:
      - human
  - term: HostRoutedOperatorRequest
    category: authority
    definition: >-
      The effect-specific provenance value created when a host recognizes a
      direct, unambiguous operator request and routes its exact effect, subject,
      payload digest, and request digest to the kernel effect boundary. It says
      only what the host routed: the kernel does not claim to have independently
      established a mental intent or a U.SpeechAct occurrence. Decision binding,
      manual profile application, and ProjectTypeEnvHead selection each require
      a fresh request for their own exact effect and subject.
    aliases:
      - host-routed operator request
      - host_routed_operator_request
    not:
      - A skill-token invocation or authorization receipt.
      - A quotation, tool output, agent recommendation, or the agent's own
        proposal.
      - Independent kernel proof of U.SpeechAct or hidden operator intent.
      - Reusable authority for another effect, subject, option, or scope.
    owners:
      - human
  - term: WorkCommission
    category: execution
    definition: >-
      A bounded human authorization to attempt named work under an explicit
      scope, delivery policy, evidence requirement, and autonomy envelope.
    aliases:
      - commission
    not:
      - A runtime process.
      - Proof that commissioned work was performed.
      - Permission to publish externally unless explicitly granted.
    owners:
      - human
  - term: RuntimeRun
    category: execution
    definition: >-
      A dated occurrence of performed Work in which the runtime harness executes
      an admissible WorkCommission in an identified isolated workspace or
      worktree. The occurrence exists in time and remains distinct from every
      description, result record, and carrier that reports it.
    aliases:
      - runtime run
    not:
      - The commission itself.
      - A WorkPlan.
      - A RuntimeRunRecord or result carrier.
    owners:
      - human
  - term: RuntimeRunRecord
    category: execution-record
    definition: >-
      An addressable project-memory description of one RuntimeRun, including its
      run identity, WorkCommission reference, observed timestamps, state,
      outputs, verdicts, and provenance. JSON, Markdown, database rows, and UI
      renderings are carriers of the record; neither the record nor its carriers
      are the performed Work occurrence.
    aliases:
      - runtime run record
      - runtime result record
    not:
      - The RuntimeRun occurrence.
      - Evidence merely because it reports a result.
      - Proof that the Work satisfied its acceptance conditions.
    owners:
      - human
  - term: Evidence
    category: verification
    definition: >-
      A contextual evidence-use relation connecting an admitted supporting
      U.Episteme to a target claim in a BoundedContext as support or weakening,
      with explicit polarity, provenance, and a freshness boundary. When a
      production trace is current, performed Work and observation conditions
      remain distinct provenance values; a separate assertion and Admission
      establish the contextual Evidence relation. Evidence remains distinct from
      records and carriers that present the relation.
    aliases:
      - epistemic support
    not:
      - Documentation alone.
      - Confidence prose.
      - An EvidenceRecord or its carrier.
      - Proof by existence alone.
    owners:
      - human
  - term: EvidenceRecord
    category: verification-record
    definition: >-
      An addressable project-memory description of one evidence-use assertion,
      recording the supporting episteme, target claim, BoundedContext,
      provenance, polarity, freshness, and any assessment. A file, database row,
      or rendered view is a Carrier of that record, not the Evidence relation or
      supporting episteme itself.
    aliases:
      - evidence record
    not:
      - Evidence itself.
      - The performed Work occurrence or observation conditions that yielded
        the supporting episteme.
      - Automatic acceptance of the target claim.
    owners:
      - human
  - term: R_eff
    category: verification
    definition: >-
      Effective reliability of a claim computed from the weakest supported
      evidence link rather than by averaging unrelated strengths.
    aliases:
      - effective reliability
    not:
      - Confidence by prose volume.
      - A substitute for inspecting evidence provenance.
    owners:
      - human
  - term: Project profile
    category: project-profile
    definition: >-
      An umbrella name for the project-local profile family. Use the exact
      ConfiguredProjectProfile, ResolvedProjectProfile, or
      ProjectProfileSuggestion term whenever configuration input, resolved
      state, or detector output matters.
    aliases:
      - project-profile
    not:
      - An FPF kind.
      - A repository-language label.
      - Proof that the repository itself is the realized object.
      - A wire-level substitute for the exact profile variant.
    owners:
      - human
  - term: ConfiguredProjectProfile
    category: project-profile
    definition: >-
      The canonical project-profile state decoded as either Auto or Declared
      with one ProfileDeclarationPayload, an exact ProfileDeclarationReceiptV1,
      and a resolvable ProfileDeclarationAdmissionRecord. In the v1 onboarding
      path HaftSoftwareSystem transforms and admits a candidate produced by a
      host-agent system during bounded onboarding Work. Auto requests resolution
      but contains no inferred classification and grants no binding applicability
      by itself. The YAML configuration card is a projection of this state.
    aliases:
      - configured project profile
    not:
      - A ProjectProfileSuggestion.
      - A detector conclusion stored as authority.
      - A Boolean software-project switch.
    owners:
      - human
  - term: ResolvedProjectProfile
    category: project-profile
    definition: >-
      The pure resolution result: either Classified with a non-empty set of
      RealizationScopes and exact ProfileBasis, or Undetermined with missing
      basis and at most a non-binding ProjectProfileSuggestion. Only an admitted
      Declared profile whose ProfileDeclarationAdmissionRecord and linked Work
      and authority-resolution records resolve with integrity can establish
      binding Required or NotApplicable results; the attached receipt describes
      that historical admission but does not grant authority or require the
      one-pass permission to remain current on later reads.
    aliases:
      - resolved project profile
    not:
      - The ConfiguredProjectProfile input.
      - Silent mutation of project configuration.
      - Proof obtained from repository heuristics alone.
    owners:
      - human
  - term: ProfileDeclarationAuthorityBasis
    category: project-profile
    definition: >-
      The action-specific provenance basis required for current
      profile-declaration admission. The explicit-operator variant binds one
      exact HostRoutedOperatorRequest to the reviewed work input and project;
      the automatic singleton-bootstrap variant records detector_default and
      the exact deterministic policy basis. Neither variant claims independent
      proof of a U.SpeechAct. Older speech-act and permission tuples remain
      sealed, readable history and cannot authorize a current admission.
    aliases:
      - profile declaration authority basis
    not:
      - A ProfileDeclarationReceiptV1.
      - The operator message or a skill-token invocation.
      - A generated rationale or model-supplied reference.
      - A repository-classification signal.
      - A generic authority basis for decisions, commissions, spec lifecycle,
        or ProjectTypeEnvHead selection.
    owners:
      - human
  - term: ProfileDeclarationAuthorizationContent
    category: project-profile
    definition: >-
      A legacy v1-v3 utterance-description episteme retained only to decode and
      audit sealed profile-admission history. It carried the bounded action,
      project, method, role, observation and validity coordinates used by that
      historical generation. Current explicit profile application uses an exact
      HostRoutedOperatorRequest; automatic singleton bootstrap uses
      detector_default. New authorization-content rows are not admitted.
    aliases:
      - profile declaration authorization content
    not:
      - A current HostRoutedOperatorRequest.
      - Current profile-declaration authority.
      - A ProfileDeclarationReceiptV1.
    owners:
      - human
  - term: ProfileDeclarationPermission
    category: project-profile
    definition: >-
      A legacy v1-v3 project-local U.Commitment retained only as part of sealed,
      readable profile-admission history. Current admission never creates or
      reuses this permission: explicit application is host-routed and automatic
      singleton bootstrap is justified by its deterministic system policy.
    aliases:
      - profile declaration permission
    not:
      - ProfileDeclarationAuthorizationContent.
      - A current profile-application request.
      - ProfileOnboardingWork.
    owners:
      - human
  - term: ProjectProfileSuggestion
    category: project-profile
    definition: >-
      A read-only detector result containing suggested RealizationScopes,
      positive and negative repository signals, detector version, confidence
      posture, and conflicting detector signals. It is non-binding orientation
      input and may supply observed basis to host-routed profile application or
      deterministic singleton bootstrap Work, but it never acts or mutates
      ConfiguredProjectProfile by itself.
    aliases:
      - project profile suggestion
    not:
      - A binding ResolvedProjectProfile classification.
      - Permission to create, delete, or reinterpret specification carriers.
      - ContextKindAvailability, a KindClassificationJudgement, or durable FPF U-kind admission.
    owners:
      - human
  - term: ProfileDeclarationCandidate
    category: project-profile
    definition: >-
      Pre-admission declaration data produced by a dated execution of
      ProfileOnboardingMethod. It contains exactly one ProfileDeclarationPayload
      and one CandidateProvenance whose payloadDigest hashes that payload alone.
      It is input to validation and is not yet ConfiguredProjectProfile.
    aliases:
      - onboarding profile candidate
    not:
      - An admitted Declared profile.
      - A ProfileDeclarationReceiptV1.
      - Permission to persist itself.
    owners:
      - human
  - term: ProfileDeclarationPayload
    category: project-profile
    definition: >-
      The non-self-referential semantic payload of a profile candidate: a
      non-empty set of proposed stable RealizationScopes and their ProfileBasis.
      It excludes CandidateProvenance, authority records, receipts, admission
      proofs, and carrier revision, so its canonical digest can be computed
      before candidate admission.
    aliases:
      - profile candidate payload
    not:
      - ProfileDeclarationCandidate.
      - CandidateProvenance.
      - A persisted Declared profile.
    owners:
      - human
  - term: CandidateProvenance
    category: project-profile
    definition: >-
      Pre-admission provenance for a ProfileDeclarationCandidate. It references
      the exact ProfileOnboardingWorkRecord and ProfileDeclarationAuthorityBasis
      and carries project root, classifier and policy versions, session,
      canonical ProfileDeclarationPayload digest, observed-basis digest, and
      its own canonical provenance digest. RoleAssignment, executing system,
      Work interval, and basis-observation window live canonically in the Work
      record rather than being duplicated here. It contains no admission state,
      ledger revision, or projection digest.
    aliases:
      - profile candidate provenance
    not:
      - A ProfileDeclarationReceiptV1.
      - ProfileDeclarationAuthorityBasis.
      - Proof that a candidate was admitted or persisted.
    owners:
      - human
  - term: ProfileOnboardingWorkRecord
    category: project-profile
    definition: >-
      An immutable description record that references one dated U.Work
      occurrence in which a host-agent system executes ProfileOnboardingMethod.
      It carries WorkRef, enactsMethod, methodDescriptionRef, performedBy as the
      exact ProfileAuthorRole U.RoleAssignment, executedWithin as the concrete
      host-agent U.System, onboarding context, Work interval, canonical concrete
      parameter bindings for the named MethodDescription edition, explicit
      inputs, outputs, resources and affected classification episteme, a
      separately named basisObservationWindow, StatePlaneRef with pre/post
      references or a declared delta predicate, and exactly one outcome:
      CandidatePayloadProduced{payloadDigest, observedBasisDigest} or
      ClassificationUnderdetermined{missingBasisDigest}. It is persisted before
      a Candidate is exposed and does not point to CandidateProvenance.
    aliases:
      - profile onboarding work record
    not:
      - The HostRoutedOperatorRequest or automatic-bootstrap policy basis.
      - ProfileOnboardingMethod or its U.MethodDescription.
      - Proof of profile persistence.
    owners:
      - human
  - term: ProfileClassificationResult
    category: project-profile
    definition: >-
      The result union of ProfileOnboardingMethod: Candidate carrying one
      ProfileDeclarationCandidate, or Underdetermined carrying a durable
      ProfileOnboardingWorkRecordRef and a non-empty missing-basis set. It
      describes bounded classification before canonical admission and remains
      separate from ConfiguredProjectProfile state,
      ProfileDeclarationAdmissionResult, and per-capability Applicability.
    aliases:
      - profile classification result
    not:
      - ConfiguredProjectProfile.Auto or Declared.
      - Capability Applicability.Underdetermined.
      - Proof that candidate persistence occurred.
    owners:
      - human
  - term: ProfileDeclarationAdmissionResult
    category: project-profile
    definition: >-
      The later admission-operation result union: Admitted carries the Declared
      ConfiguredProjectProfile, ProfileDeclarationReceiptV1,
      ProfileDeclarationAdmissionRecordRef, and exact ledger revision;
      NotAdmitted carries typed violated or missing basis; WriteFailed carries
      an effect-failure reference. The latter two branches perform no canonical
      profile write and consume no singleUseKey. It never rewrites the earlier
      ProfileClassificationResult.
    aliases:
      - profile admission result
    not:
      - ProfileClassificationResult.
      - Capability Applicability.
      - A repository detector suggestion.
    owners:
      - human
  - term: ProfileDeclarationAdmissionRecord
    category: project-profile
    definition: >-
      The durable proof produced only by a successful canonical admission-ledger
      transaction. It binds the ProfileDeclarationPayload, CandidateProvenance,
      ProfileOnboardingWorkRecordRef, ProfileDeclarationAuthorityBasis,
      exact AuthorityResolutionRecord reference and digest,
      ProfileDeclarationReceiptV1, expected and new ledger revisions, and the
      singleUseKey that the transaction will consume through an
      AuthorityUseRecord. It does not contain the later AuthorityUseRecord
      digest; the same transaction installs the canonical Declared profile and
      a use record that points back to this admission record. The YAML
      configuration card is a later projection and is not part of this atomic
      boundary.
    aliases:
      - profile admission record
    not:
      - The admission Work occurrence.
      - The permission U.Commitment.
      - A YAML carrier acting as authority.
    owners:
      - human
  - term: AuthorityUseRecord
    category: authority
    definition: >-
      The canonical single-use record written in the same admission-ledger
      transaction as one ProfileDeclarationAdmissionRecord. Its unique
      singleUseKey equals the exact key from the admitted authority basis. It
      also binds action kind,
      project-binding digest, authorization-envelope digest,
      authority-record reference and digest, exact admission-request digest,
      verifier identity and version, committed result reference and digest, and
      timestamps. The unique key makes same-key/different-request a replay
      conflict; the record is not the operator request, bootstrap policy, or
      receipt.
    aliases:
      - authority use record
    not:
      - HostRoutedOperatorRequest or detector_default policy.
      - A caller-supplied single-use Boolean.
      - A ProfileDeclarationReceiptV1.
    owners:
      - human
  - term: AuthorityResolutionRecord
    category: authority
    definition: >-
      The canonical record behind one current admitted profile-authority
      resolution. The host-routed variant binds the exact
      HostRoutedOperatorRequest, reviewed work input, action envelope, project
      binding, verifier, and resolution time. The automatic variant binds the
      detector_default singleton policy and its exact observed basis. It neither
      consumes the singleUseKey nor proves that profile admission committed;
      legacy speech-act resolutions are decode-only history.
    aliases:
      - authority resolution record
    not:
      - AuthorityUseRecord.
      - ProfileDeclarationAdmissionRecord.
      - A caller-supplied receipt.
    owners:
      - human
  - term: ProfileAdmissionLedger
    category: authority
    definition: >-
      The kernel-owned canonical transactional store for authority
      provenance bases, AuthorityResolutionRecords, immutable
      ProfileOnboardingWorkRecords, AuthorityUseRecords,
      ProfileDeclarationAdmissionRecords, admitted ConfiguredProjectProfile
      revisions, and projection debt. After persisting and rereading the Work
      record, the current canonical profile revision is captured as the expected
      admission revision. The later admission
      transaction revalidates the exact authority-resolution record, request or
      automatic policy basis, action, project binding, and envelope at its judgement
      time, then performs expected-revision comparison, uniqueness-enforced
      singleUseKey consumption, authority-use recording, and canonical profile
      admission.
      The YAML configuration card is a recoverable projection of this ledger.
    aliases:
      - profile admission ledger
    not:
      - "`.haft/project-profile.yaml`."
      - A ProfileDeclarationReceiptV1.
      - A host-generated authority claim.
    owners:
      - human
  - term: ProfileDeclarationReceiptV1
    category: project-profile
    definition: >-
      An admission provenance record finalized for a Declared
      ConfiguredProjectProfile only inside a successful admission operation. It
      identifies the exact AuthorityResolutionRecord,
      ProfileDeclarationAuthorityBasis, ProfileOnboardingWorkRecordRef,
      CandidateProvenance digest, payload digest, observed-basis digest, ledger
      revision, and recording time. Admission finalizes it only inside the
      canonical transaction that consumes the singleUseKey and installs the
      profile. A pre-admission candidate carries CandidateProvenance instead.
      The receipt is neither the HostRoutedOperatorRequest, automatic-bootstrap
      policy, Work, declaration, admission proof, nor YAML carrier.
    aliases:
      - profile declaration receipt
      - onboarding profile receipt
      - ProfileDeclarationReceipt
    not:
      - A ProjectProfileSuggestion.
      - A detector score.
      - An operator request, automatic policy, or authority by itself.
      - The declaration or classification Work it records.
      - The `.haft/project-profile.yaml` carrier.
      - A carrier acting as an agent.
      - Authority for a binding decision, specification lifecycle change, or
        ProjectTypeEnvHead selection.
    owners:
      - human
  - term: Realization scope
    category: project-profile
    definition: >-
      One stable project-local scope in which an identified object is realized,
      classified as software or non-software and carrying the basis needed for
      capability applicability. Its ScopeID is an opaque identity, not a path or
      file-extension inference.
    aliases:
      - realization-scope
    not:
      - A universal taxonomy of project kinds.
      - A repository directory.
      - A project phase.
    owners:
      - human
  - term: ScopeID
    category: project-profile
    definition: >-
      A stable opaque project-local identity for one RealizationScope. It
      preserves scope identity across profile editions and may later be related
      to an Entity identity through an explicit typed relation.
    aliases:
      - scope ID
    not:
      - A repository path.
      - An EntityRef.
      - A software or non-software classification by itself.
    owners:
      - human
  - term: Applicability
    category: project-profile
    definition: >-
      The tri-state result Required, NotApplicable, or Underdetermined for one
      named project capability or specification carrier in one or more
      realization scopes, evaluated from profile basis before readiness. This is
      project-capability applicability, not applicability of an FPF pattern to a
      working situation.
    aliases:
      - project capability applicability
      - specification applicability
    not:
      - FPF pattern applicability.
      - A retrieval score.
      - A Boolean software-project flag.
    owners:
      - human
  - term: FPF Query
    category: fpf-query
    definition: >-
      Haft's read-only source-native retrieval capability over a pinned FPF
      publication. It resolves exact source units or returns source candidates,
      retains exact provenance and retrieval internals in the canonical query
      execution, and publishes a bounded working, trace, or diagnostic
      description for the current use. It does not decide which pattern applies
      or replace the full pattern body.
    aliases:
      - fpf-query
      - source-native query
    not:
      - A pattern router.
      - A governing-pattern selector.
      - A type checker.
    owners:
      - human
  - term: SourceUnit
    category: fpf-query
    definition: >-
      A derived, exactly addressable unit of an FPF publication containing its
      source-owned text, publication role, identifiers, direct references, line
      range, content hash, and source revision.
    aliases:
      - source unit
    not:
      - A Haft-authored summary of FPF meaning.
      - A project-memory assertion.
    owners:
      - human
  - term: FPF Query publication view
    category: fpf-query
    definition: >-
      A closed public description projected from one already-validated canonical
      FPF Query execution independently of concern, lookup, or inspect mode.
      Working is the default agent carrier without internal provenance or raw
      retrieval grounds; trace reconstructs canonical provenance and replay
      coordinates; diagnostic exposes retrieval internals. A publication view
      does not alter retrieval semantics or the canonical source result.
    aliases:
      - query publication view
      - working query view
      - trace query view
      - diagnostic query view
    not:
      - A retrieval mode.
      - A pattern applicability or selection result.
      - Evidence that hidden canonical provenance is absent.
    owners:
      - human
  - term: ExactHit
    category: fpf-query
    definition: >-
      An FPF Query result containing one SourceUnit resolved by an exact
      identifier or exact unambiguous name. Its canonical execution retains
      source provenance; a working publication view need not reproduce that
      provenance inline.
    aliases:
      - exact hit
    not:
      - Proof that the resolved pattern applies.
      - A recommended next action.
    owners:
      - human
  - term: CandidateSet
    category: fpf-query
    definition: >-
      An FPF Query result that groups several source candidates by publication
      role without selecting a winner or placing roles on one semantic score
      scale. Its canonical execution retains match grounds and provenance; the
      working publication view preserves source semantics, response budget, and
      truncation while diagnostic and trace expose those internal descriptions
      only when requested.
    aliases:
      - candidate set
    not:
      - A pattern selection.
      - A work sequence.
      - Evidence of applicability.
    owners:
      - human
  - term: Abstained
    category: retrieval
    definition: >-
      An explicit read result stating that the available source or project-
      memory basis is insufficient to return a trustworthy exact result or
      candidate set. A working publication view preserves a bounded missing
      basis; attempted producers and retrieval details belong to diagnostic
      view.
    aliases:
      - abstention result
    not:
      - NotApplicable.
      - Invalid.
      - A fabricated empty success.
    owners:
      - human
  - term: Entity
    category: typed-memory
    definition: >-
      A stable project identity for a referenced value. Identity is shared
      across bounded contexts, while ContextKindAvailability,
      KindClassificationJudgements, optional KindExtension representations,
      and relation uses remain context-local; identity alone
      establishes neither kind nor truth.
    aliases:
      - entity identity
    not:
      - A duplicate node identity per bounded context.
      - ContextKindAvailability or a KindClassificationJudgement.
      - An EntityOfConcern by default.
    owners:
      - human
  - term: ContextKindAvailability
    category: typed-memory
    definition: >-
      A derived internal TypeEnv projection stating only that one exact U.Kind
      value is available for typed use in one exact BoundedContext. The linker
      derives it after exact provider resolution from explicit kind
      declarations or uses together with Signature Applicability, preserving a
      canonical nonempty set of carrier, edition, digest, source-range,
      compiler-rule, provider, and required-bridge grounds. Exact base-provider
      use and same-context imports may ground it; cross-context local reuse
      requires an exact KindBridge. It is not authored as a fifth A.6.0
      Signature row and is not a standalone FPF object.
    aliases:
      - context kind availability
      - kind available in context
    not:
      - A KindClassificationJudgement for a concrete candidate.
      - Slot compatibility or a RelationInstance.
      - A.6.1 mechanism admissibility.
      - E.24.UK durable public U-kind admission.
      - ProjectTypeEnvHead selection, authority, truth, or evidence.
      - A value inferred from spelling, labels, graph presence, or retrieval.
    owners:
      - human
  - term: EntityOfConcern
    category: typed-memory
    definition: >-
      The U.Entity value about which a governed episteme or description makes
      claims. EntityOfConcernSlot is the local SlotKind; the EntityOfConcern value
      is its U.Entity filler; and entityOfConcernRef is the U.EntityRef stored as
      slot content when the SlotSpec uses reference mode. BoundedContextRef is a
      separate member of DescriptionContext and is not part of the
      EntityOfConcern value, EntityOfConcernSlot, or entityOfConcernRef. A recall
      projection may key an exact read by the pair <EntityOfConcernRef,
      BoundedContextRef>, but that read key does not redefine EntityOfConcern or
      manufacture an Entity from query text, labels, or retrieval candidates.
    aliases:
      - entity of concern
      - current concern filler
    not:
      - A U.EntityOfConcern kind.
      - Every project-memory node.
      - A permanent label on an Entity.
      - A new Entity inferred from words when no exact EntityRef resolves.
      - A context-free concern identity.
      - The pair used as an exact recall key.
    owners:
      - human
  - term: BoundedContext
    category: typed-memory
    definition: >-
      The exact context in which ContextKindAvailability,
      KindClassificationJudgements, relation-slot interpretations,
      constraints, aliases, bridges, and current uses have meaning.
      Cross-context use requires an explicit bridge and a fresh target-side
      classification rather than score-based or name-based merging.
    aliases:
      - bounded context
    not:
      - A filesystem namespace.
      - A universal project phase.
      - Permission to merge identities or meanings.
    owners:
      - human
  - term: KindClassificationJudgement
    category: typed-memory
    definition: >-
      Haft's current product representation of the C.3.2 classification
      judgment for one exact candidate, local U.Kind, KindSignature edition,
      and ContextSlice. Its closed result is true, false, or unknown. The
      evaluator applies the signature criterion to direct governed candidate
      features; evidence may support a separate classification assertion but
      does not create the judgment truth. A receiving guard may fail closed on
      unknown without rewriting it to false.
    aliases:
      - classification judgment
      - kind classification judgement
    not:
      - ContextKindAvailability for typed use of a kind in a context.
      - E.24.UK durable public U-kind admission.
      - An A.14 MemberOf relation occurrence.
      - A direct classification-relation occurrence.
      - Slot compatibility or a RelationInstance.
      - A judgement inferred from an entity label, filename, or ID prefix.
      - Evidence that claims about the Entity are true.
      - A guard decision or admission result.
    owners:
      - human
  - term: KindExtension
    category: typed-memory
    definition: >-
      An optional set-valued representation of candidates whose exact
      KindClassificationJudgement is true for one pinned KindSignature edition
      and ContextSlice. It exists only for a named set-consuming use and may
      change when candidate state or slice changes without changing the local
      kind or signature.
    aliases:
      - kind extension representation
    not:
      - U.EntitySet.
      - An A.14 membership relation.
      - A collection holon.
      - A direct classification relation or source of candidate truth.
      - A prerequisite for evaluating one exact candidate.
    owners:
      - human
  - term: MemberOf
    category: typed-memory-legacy
    definition: >-
      A sealed legacy Haft API, artifact, and wire spelling used by earlier
      TypeEnv editions for candidate classification. Existing records may be
      decoded, inspected, and replayed against the exact historical edition,
      but new current editions emit KindClassificationJudgement and do not
      attribute this spelling to current FPF C.3 or A.14.
    aliases:
      - legacy membership judgement
    not:
      - Current FPF C.3 classification meaning.
      - Permission to reinterpret historical results under a newer signature.
      - A new-write spelling for KindClassificationJudgement.
    owners:
      - human
  - term: RelationSignature
    category: typed-memory
    definition: >-
      The stronger FPF U.Signature declaration episteme for one direct relation
      kind as its exact EntityOfConcern. It carries the participant meanings
      and SlotSpecs together with the obtaining predicate and laws,
      applicability, occurrence-identity rule, complete canonical ClaimGraph,
      and effective declaration ReferenceScheme. Current Haft local-practice
      relation carriers do not satisfy this complete boundary.
    aliases:
      - relation signature
    not:
      - An arbitrary source-target edge label.
      - A TypedRelationDeclarationFragment merely because it has a
        RelationSignatureRef or the historical relation_signature source token.
      - A relational assertion or an obtaining relation occurrence.
      - A work order.
    owners:
      - human
  - term: TypedRelationDeclarationFragment
    category: typed-memory
    definition: >-
      The canonical v9 posture for Haft local-practice relation schemas: one
      exact edition-bound symbol/ref, declared BoundedContexts, named A.6.5
      SlotSpecs, separately named cardinality and structural constraints, and
      exact provenance. It supports only the local structural assertion checks
      declared by that edition. Sealed 1.0.0/1.1.0 carriers and wire bytes may
      retain RelationSignature, RelationSignatureRef, define_relation_signature,
      or relation_signature as compatibility spellings for this same limited
      payload.
    aliases:
      - typed relation declaration fragment
      - relation declaration fragment
      - relation signature (historical edition compatibility only)
    not:
      - A complete FPF RelationSignature declaration episteme.
      - An obtaining predicate, applicability rule, or occurrence-identity
        evaluator.
      - Permission to admit a durable FPF relation kind.
      - Evidence that a structurally valid assertion is true or obtains.
    owners:
      - human
  - term: RelationalAssertion
    category: typed-memory
    definition: >-
      One positive, negative, or unknown project assertion under an exact
      TypedRelationDeclarationFragment, BoundedContext, named typed slot
      fillers, and provenance. Structural validation preserves its explicit
      modality but establishes neither direct-predicate truth nor an obtaining
      occurrence.
    aliases:
      - relational assertion
    not:
      - An untyped JSON edge.
      - A complete FPF RelationSignature.
      - An occurrence designation or proof that the asserted relation obtains.
    owners:
      - human
  - term: RelationInstance
    category: typed-memory
    definition: >-
      The historical haft.memory.v1/v2 carrier and wire spelling retained for
      exact replay of older project rows. It is decoded only as a
      LegacyUnqualifiedRelationalAssertion under its origin TypeEnv; its old
      bytes do not imply an explicit modality or an obtaining occurrence.
    aliases:
      - relation instance
    not:
      - The current fresh-write form; new writes use RelationalAssertion.
      - A complete FPF RelationSignature or RelationOccurrence.
      - Permission to infer positive modality, predicate truth, or occurrence
        identity from replay order or row identity.
    owners:
      - human
  - term: TypeEnv
    category: typed-memory
    definition: >-
      An immutable content-addressed compiled semantic environment. An
      FPFBaseTypeEnv contains only explicitly covered FPF declarations with
      exact source provenance and coverage posture; a project composite derives
      its identity from that exact base, a closed symbolic extension DAG, and an
      exact runtime evaluation basis. The environment used for new project
      admission is the already-derived composite selected by the current
      ProjectTypeEnvHead. Neither form contains rules inferred from examples,
      ranking, or lexical proximity.
    aliases:
      - type environment
    not:
      - A hand-written shadow FPF ontology.
      - The FPF Query index.
      - Every rule expressible by the FPF source.
    owners:
      - human
  - term: head-selected TypeEnv
    category: typed-memory
    definition: >-
      The exact composite environment used for new project-memory admission:
      one exact FPFBaseTypeEnv, a canonically resolved content-addressed DAG of
      symbolic ProjectTypeEnvExtensionArtifacts, and one exact
      RuntimeEvaluationBasisArtifact, selected by the current
      ProjectTypeEnvHead. Bundling, compilation, linking, lowering, or staging
      does not select the composite; only a successful dedicated head-selection
      CAS Work changes which already-derived composite governs new admission.
    aliases:
      - current head-selected TypeEnv
    not:
      - The newest bundled TypeEnv by default.
      - A mutable state stored inside B, E, X, or C.
      - A reason to reinterpret historical assertions silently.
    owners:
      - human
  - term: MemoryChangeSet
    category: typed-memory
    definition: >-
      A non-empty immutable collection whose closed instance-level change
      algebra contains only DeclareEntity, IdentityChange,
      InstantiateRelation against a signature in the exact composite selected
      by the current ProjectTypeEnvHead, or
      RetractAssertion. It cannot establish ContextKindAvailability or declare relation signatures,
      ValueShapes, CodecRefs, codec implementations, or TypeEnv extensions.
    aliases:
      - change set
      - ChangeSet
    not:
      - An arbitrary list of free-string graph edges.
      - A schema change or ProjectTypeEnvHead selection request.
      - A WorkPlan.
      - Evidence that the change has been admitted.
    owners:
      - human
  - term: IdentityChange
    category: typed-memory
    definition: >-
      A closed instance-level MemoryChange for admitting or superseding a
      context-bound alias or for an explicitly reviewed, append-only merge or
      split of entity identities. It preserves provenance and history and does
      not declare a kind, signature, or codec.
    aliases:
      - identity change
    not:
      - A fuzzy-search identity merge.
      - A schema mutation.
      - Permission to erase historical EntityRefs.
    owners:
      - human
  - term: Haft Typed-Memory Local-Practice/DPF carrier
    category: typed-memory
    definition: >-
      The versioned project-local declaration carrier for Haft artifact and
      code-anchor kinds, relation signatures, laws, applicability, ValueShapes,
      and CodecRef bindings. It preserves the A.6.0 SignatureManifest and four
      conceptual rows SubjectBlock, Vocabulary, Laws, and Applicability, with
      A.6.5 SlotSpecs inside Vocabulary.
    aliases:
      - typed-memory local-practice carrier
    not:
      - FPF Core source.
      - A MemoryChangeSet.
      - Evidence that an E artifact is selected by the current project head
        merely because the carrier is present in the repository.
    owners:
      - human
  - term: ProjectTypeEnvExtensionCandidate
    category: typed-memory
    definition: >-
      One verified ProjectTypeEnvExtensionArtifact considered as input to
      deterministic composite construction. It includes symbolic declarations
      and exact source, manifest, base, and compiler provenance but no
      current-state compatibility result, existing-assertion revalidation
      result, authority, or head-selection receipt. Candidate is a use in a
      build operation, not a lifecycle state stored on E; compile, inspect, and
      storage remain non-binding.
    aliases:
      - project TypeEnv extension candidate
    not:
      - Evidence that any ProjectTypeEnvHead-selected composite contains it.
      - A generic memory write.
      - Authority to select a composite containing it as the project head.
    owners:
      - human
  - term: ExtensionSelectedByHead
    category: typed-memory
    definition: >-
      A read-only derived relation or view that holds for one exact symbolic E
      artifact if and only if the exact head selects C and C's verified ordered
      E DAG contains that exact E ref. It is recomputed from the immutable head,
      C, and E-DAG identities and stores no mutable status on E.
      E participates in C identity before any human act; Genesis or Transition
      selects a whole already-derived C and never inserts, removes, enables, or
      disables E inside that C. Rollback is a Transition selecting a previously
      admitted C.
    aliases:
      - extension selected by current head
    not:
      - A lifecycle state or field on ProjectTypeEnvExtensionArtifact.
      - Evidence from compilation, storage, or Stage alone.
      - A MemoryChangeSet member.
      - A separately mutable member of an existing composite.
      - An Entity with its own EntityID, write, authority basis, or receipt.
    owners:
      - human
  - term: ProjectTypeEnvExtensionIR
    category: typed-memory
    definition: >-
      The self-reference-free symbolic compiler result for one exact
      Local-Practice carrier. TypeEnv-bearing positions remain symbolic until a
      composite TypeEnvRef exists; the IR carries exact source declarations,
      dependencies, exports, coordinates, and provenance but no digest claim,
      current project result, authority, or head-selection state.
    aliases:
      - symbolic extension IR
    not:
      - A runtime TypeEnv.
      - A staged compatibility result.
      - An ExtensionSelectedByHead relation or any project-head posture.
    owners:
      - human
  - term: ProjectTypeEnvExtensionArtifact
    category: typed-memory
    definition: >-
      The immutable content-addressed sealed E artifact whose canonical bytes
      exactly encode and verify one ProjectTypeEnvExtensionIR and its source
      identity. Its E ref participates in composite identity; its presence does
      not select a composite containing it or make any signature writable.
    aliases:
      - symbolic extension artifact
    not:
      - A post-composite TypeEnvExtensionProposal.
      - A compatibility or revalidation report.
      - Authority to select a composite containing it.
    owners:
      - human
  - term: LinkedProjectTypeEnvCompositeIR
    category: typed-memory
    definition: >-
      A self-reference-free deterministic linkage of one exact base and a
      closed canonical DAG of verified extension artifacts, including exact
      dependency-provider resolution, manifest closure, collision diagnostics,
      and explicit coverage gaps. It precedes runtime-basis binding and does not
      yet assert a composite TypeEnvRef.
    aliases:
      - linked project TypeEnv IR
    not:
      - Caller ordering of extension files.
      - A ProjectTypeEnvStage or head-selected TypeEnv.
      - A human authorization record.
    owners:
      - human
  - term: RuntimeEvaluationBasisArtifact
    category: typed-memory
    definition: >-
      An immutable content-addressed Haft-local realization identity that pins
      only the exact codec, evaluator, and carrier-membership mechanisms needed
      to reproduce Valid, Invalid, or Underdetermined results for a composite.
    aliases:
      - runtime evaluation basis
    not:
      - Current project records or graph revision.
      - A compatibility or revalidation result.
      - Human intent, authority, a project head, or head-selection receipt.
    owners:
      - human
  - term: ProjectTypeEnvCompositeArtifact
    category: typed-memory
    definition: >-
      The immutable content-addressed composite C derived from one exact
      FPFBaseTypeEnv, a canonical verified extension-artifact DAG, and one exact
      RuntimeEvaluationBasisArtifact. Its verified artifact authenticates the
      TypeEnvRef used to lower all runtime TypeEnv-scoped references.
    aliases:
      - project TypeEnv composite
    not:
      - A caller-supplied TypeEnvRef.
      - A current-state Stage result.
      - The current project head or head-selection receipt.
    owners:
      - human
  - term: ProjectTypeEnvStage
    category: typed-memory
    definition: >-
      An immutable content-addressed review artifact whose canonical Stage
      record binds ProjectTypeEnvStageRef, exact ProjectID, exact target
      ProjectTypeEnvCompositeRef, project-snapshot ref and digest,
      ExpectedGraphRevision, every mutable-basis revision and digest relied on
      including the project-profile ledger, compatibility-diff ref and digest,
      existing-assertion-revalidation ref and digest, profile-fit ref and
      digest, source provenance, schema edition, producer edition, canonical
      bytes, and content digest. ProjectTypeEnvStageCarrierRef names only a
      publication or serialization carrier and is not the Stage artifact or
      record. Staging is read-only and confers no ProjectTypeEnvHead-selection
      authority.
    aliases:
      - TypeEnv stage
      - ProjectTypeEnvStageArtifact
    not:
      - Part of symbolic E or composite C identity.
      - A project TypeEnv head.
      - A human authorization act.
      - Its ProjectTypeEnvStageCarrierRef.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionPredecessor
    category: typed-memory
    definition: >-
      The closed predecessor sum used by a
      ProjectTypeEnvHeadSelectionRequest. Its only variants are
      ProjectTypeEnvGenesisPredecessor carrying exactly one NoPriorHeadProofRef
      and ProjectTypeEnvTransitionPredecessor carrying exactly one
      ExpectedPriorHead. Absence, an empty ref, revision zero, an unreadable or
      corrupt row, and caller inference are not additional variants.
    aliases:
      - TypeEnv head-selection predecessor
    not:
      - A nullable prior-head field.
      - The common project or ExpectedGraphRevision fields of the request.
    owners:
      - human
  - term: ProjectTypeEnvGenesisPredecessor
    category: typed-memory
    definition: >-
      The Genesis predecessor variant containing one exact NoPriorHeadProofRef.
      The proof itself binds the same ProjectID and ExpectedGraphRevision held
      once at the enclosing request level, and the constructor and storage CAS
      recheck exact equality.
    aliases:
      - Genesis predecessor
    not:
      - A missing ProjectTypeEnvHead row by itself.
      - A Transition with a null prior head.
    owners:
      - human
  - term: ProjectTypeEnvTransitionPredecessor
    category: typed-memory
    definition: >-
      The Transition predecessor variant containing one ExactPriorHead whose
      strong value binds exact ProjectID, prior ProjectTypeEnvCompositeRef, and
      ProjectTypeEnvHeadRevision. The revision is a head revision and cannot be
      supplied, compared, or interpreted as GraphRevision.
    aliases:
      - Transition predecessor
    not:
      - A generic ExpectedRevision.
      - ProjectTypeEnvGenesisPredecessor.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionTarget
    category: typed-memory
    definition: >-
      The exact already-derived target tuple containing ExactBaseTypeEnvRef,
      canonically ordered exact ProjectTypeEnvExtensionArtifact refs,
      ExactRuntimeEvaluationBasisRef, ExactVerifiedCompositeTypeEnvRef, and
      ExactProjectTypeEnvStageRef. The verified C must authenticate the same B,
      ordered E DAG, and X, and Stage must bind that same project, C, and the
      enclosing request's ExpectedGraphRevision.
    aliases:
      - TypeEnv head-selection target
    not:
      - Instructions to add or remove E inside an existing C.
      - A caller-selected TypeEnvRef without exact B, E, X, C, and Stage proof.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionRequest
    category: typed-memory
    definition: >-
      The canonical closed head-selection proposal. Common fields occur once:
      exact ProjectID, one ProjectTypeEnvHeadSelectionPredecessor, one
      ProjectTypeEnvHeadSelectionTarget, distinct ExpectedGraphRevision, and
      IdempotencyKey as the action's single-use identity. A domain-separated
      digest covers the canonical request bytes. Genesis and Transition differ
      only through the closed predecessor variant; rollback is a Transition to
      a previously admitted C through a fresh Stage. The request is data for a
      separately routed system CAS Work and is not authority or Work.
    aliases:
      - ProjectTypeEnvHeadSelectionRequestV1
      - TypeEnv head-selection request
    not:
      - A MemoryChangeSet.
      - A HostRoutedOperatorRequest or system CAS Work occurrence.
      - A request with common project, target, graph revision, or idempotency
        fields duplicated inside both predecessor variants.
    owners:
      - human
  - term: ProjectTypeEnvGenesis
    category: typed-memory
    definition: >-
      The dedicated system-performed CAS U.Work effect that creates one
      project's first ProjectTypeEnvHead from an exact
      ProjectTypeEnvGenesisPredecessor and target after the host routes one exact
      operator request for the reviewed selection. HaftSoftwareSystem performs
      the CAS Work through an exact RoleAssignment; the routed request records
      provenance but does not perform the head mutation.
    aliases:
      - TypeEnv genesis
    not:
      - haft init, compilation, linking, or staging.
      - A transition from an existing head.
      - A generic MemoryChangeSet.
      - The HostRoutedOperatorRequest.
    owners:
      - human
  - term: NoPriorHeadProof
    category: typed-memory
    definition: >-
      A storage-boundary proof for one exact project and expected graph revision
      that no ProjectTypeEnvHead exists. It is produced by the authoritative
      storage check and cannot be represented by an empty ref, revision zero,
      caller assertion, or corrupted row.
    aliases:
      - no prior TypeEnv head proof
    not:
      - A missing or unreadable head row.
      - A boolean supplied by a caller.
      - Permission to select a composite as the project head.
    owners:
      - human
  - term: ProjectTypeEnvTransition
    category: typed-memory
    definition: >-
      The dedicated system-performed CAS U.Work effect that moves an existing
      exact ProjectTypeEnvHead to an already-derived successor C under one
      ProjectTypeEnvTransitionPredecessor, exact target, and separately recorded
      HostRoutedOperatorRequest. HaftSoftwareSystem performs the CAS Work through
      an exact RoleAssignment; rollback is a fresh Transition selecting a
      previously admitted C through another exact request.
    aliases:
      - TypeEnv transition
    not:
      - Genesis with a missing prior ref.
      - Silent adoption of the newest bundled extension.
      - Reinterpretation of historical assertions.
      - The HostRoutedOperatorRequest.
    owners:
      - human
  - term: ProjectTypeEnvHead
    category: typed-memory
    definition: >-
      The current authoritative project pointer selecting one exact
      ProjectTypeEnvCompositeArtifact for new admission and carrying its own
      ProjectTypeEnvHeadRevision. The head records the project, selected C, head
      revision, and committing Work/result provenance. Historical composites
      and assertions remain immutable and addressable; head revision and
      project GraphRevision are distinct strong values.
    aliases:
      - current TypeEnv head
    not:
      - The composite artifact itself.
      - A bundled default or latest-version alias.
      - Evidence that historical assertions satisfy the new environment.
    owners:
      - human
  - term: ProjectTypeEnvHeadRevision
    category: typed-memory
    definition: >-
      A strong project-local monotonic revision of successful
      ProjectTypeEnvHead selections. Transition compares the exact prior head
      and this revision; Genesis has no fabricated prior revision. It is never
      interchangeable with GraphRevision, ExpectedGraphRevision, a Git commit,
      carrier edition, or TypeEnv digest even when one atomic transaction
      records both a new head revision and a new graph revision.
    aliases:
      - HeadRevision
      - TypeEnv head revision
    not:
      - GraphRevision.
      - A nullable or zero sentinel for NoPriorHeadProof.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionReceiptV1
    category: typed-memory
    definition: >-
      The immutable content-addressed durable record created exactly once by
      the original successful host-routed Genesis or Transition CAS commit. It
      binds exact ProjectID, selection-request ref and digest,
      HostRoutedOperatorRequest coordinates, reviewed-content ref and digest,
      authority-resolution and authority-use refs, CAS WorkRef and
      CAS-Work-record ref, the closed
      predecessor variant, exact B and ordered E DAG and X and C and Stage
      target, ExpectedGraphRevision, committed ProjectTypeEnvHead and
      ProjectTypeEnvHeadRevision, committed GraphRevision, IdempotencyKey,
      committed result, canonical bytes, and digest. Its strong reference is
      ProjectTypeEnvHeadSelectionReceiptRef; any
      ProjectTypeEnvHeadSelectionReceiptCarrierRef is only a separate
      serialization or publication carrier.
    aliases:
      - ProjectTypeEnvHeadSelectionReceipt
      - TypeEnv head-selection receipt
    not:
      - Hidden human intent or independent proof of a U.SpeechAct.
      - Part of composite identity.
      - Proof that later project records remain compatible.
      - ProjectTypeEnvActivationReceipt except as an explicitly legacy
        decode-or-migration alias outside the current canonical model.
      - Its ProjectTypeEnvHeadSelectionReceiptCarrierRef.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionClosureV1
    category: typed-memory
    definition: >-
      The immutable content-addressed aggregate proving one complete committed
      head-selection transaction. It contains exact refs and digests for the
      selection request, HostRoutedOperatorRequest, reviewed content, host-routed
      authority resolution, authority-use record, CAS Work occurrence and record,
      closed predecessor, B and ordered E DAG and X and C and Stage target,
      ExpectedGraphRevision, committed head and head revision, committed
      GraphRevision, IdempotencyKey, receipt ref and digest, and committed result.
      Missing, mismatched, or corrupt members make the closure unusable for
      replay; partial rows are not successful selection evidence.
    aliases:
      - TypeEnv head-selection committed closure
    not:
      - A plan or request to perform selection.
      - A substitute for the host-routed request or system CAS Work.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionReplayResult
    category: typed-memory
    definition: >-
      The closed replay result
      ReplayedExistingClosure{ClosureRef, ReceiptRef} or
      ReplayConflict{IdempotencyKey, ExistingRequestDigest,
      PresentedRequestDigest, ExistingReviewedContentDigest,
      PresentedReviewedContentDigest}. Exact replay first verifies the existing
      ProjectTypeEnvHeadSelectionClosureV1 and returns its existing canonical
      bytes and refs without current-authority revalidation, second key
      consumption, new Work, head mutation, receipt creation, or semantic write.
      The same key with any different exact request or reviewed-content
      digest is ReplayConflict with zero writes.
    aliases:
      - TypeEnv head-selection replay result
    not:
      - Re-execution of the original CAS Work.
      - Permission to repair an incomplete or corrupt closure during replay.
      - A new receipt timestamp or backdated Work occurrence.
    owners:
      - human
  - term: DescriptionRef
    category: fpf-boundary
    definition: >-
      A strong reference to a description episteme or claim content, represented
      here only as ClaimIdRef or EpistemeRef. For head selection it identifies
      the reviewed selection description carried by
      ProjectTypeEnvHeadSelectionAuthorizationContent. It does not identify the
      U.SpeechAct occurrence, its Work occurrence, or any physical or serialized
      carrier.
    aliases:
      - utterance-description ref
    not:
      - SpeechActRef.
      - WorkRef.
      - CarrierRef.
    owners:
      - human
  - term: CarrierRef
    category: fpf-boundary
    definition: >-
      A reference to an observable publication, message, terminal capture,
      file, log, or other carrier that bears or traces a description. When
      relied on it includes the exact edition, digest, or source-return basis
      required by the governing contract. It is not the Description episteme,
      U.SpeechAct occurrence, performed U.Work, authority judgement, or evidence
      conclusion merely because it makes one observable.
    aliases:
      - observable carrier ref
    not:
      - DescriptionRef.
      - SpeechActRef.
      - WorkRef.
    owners:
      - human
  - term: SpeechActRef
    category: authority
    definition: >-
      A strong reference to the identity of one actual U.SpeechAct occurrence,
      which is communicative U.Work. It is not a ref to its description record,
      utterance DescriptionRef, or CarrierRef. Reliance is direct only in the
      act's judgement BoundedContext; use for checking, provenance, or authority
      judgement in another context cites an explicit BridgeRef or named context
      policy licensing that direction and surfaces congruence and loss as
      required by A.2.9 SA-C6.
    aliases:
      - speech-act occurrence ref
    not:
      - DescriptionRef.
      - CarrierRef.
      - Deontic permission inferred from an act-type name.
    owners:
      - human
  - term: WorkRef
    category: fpf-boundary
    definition: >-
      A strong reference to the identity of one dated performed U.Work
      occurrence. It is distinct from a Work record that describes the
      occurrence, a Method, a MethodDescription, a WorkPlan, and a carrier.
      ProjectTypeEnvHeadCASWorkRecord contains the exact WorkRef of the original
      system CAS effect; replay returns that ref and never fabricates a second
      occurrence.
    aliases:
      - performed-work occurrence ref
    not:
      - ProjectTypeEnvHeadCASWorkRecordRef.
      - MethodRef or MethodDescriptionRef.
      - CarrierRef.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionAuthorizationContent
    category: authority
    definition: >-
      The immutable reviewed selection-description episteme, addressed by one
      exact DescriptionRef, for exactly one Genesis or Transition request.
      Rollback is represented as Transition. It binds the exact request ref and
      digest, project, closed predecessor, ordered E DAG, exact B, X, verified
      C, Stage, ExpectedGraphRevision, compatibility, revalidation and profile
      posture, intended head update, bounded judgement context, action kind,
      validity window, and IdempotencyKey. Verified C transitively
      authenticates the same B, E and X, but the content keeps those identities
      explicit for human review. Any CarrierRef that bears this content is a
      separate observable carrier and is not the content. Current execution
      binds this content and its selection request inside one exact
      HostRoutedOperatorRequest; the content alone has no authority.
    aliases:
      - TypeEnv head-selection content
    not:
      - A U.SpeechAct occurrence.
      - A permission or current authority judgement.
      - A ProjectTypeEnvHeadSelectionReceiptV1.
      - The CarrierRef that bears it.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionAuthorityResolution
    category: authority
    definition: >-
      A TypeEnv-specific current judgement at the CAS boundary that one exact
      HostRoutedOperatorRequest binds the exact selection-request and reviewed
      content payload, project binding, action, validity window, and expected
      predecessor. The kernel verifies those coordinates but deliberately does
      not claim independent proof of hidden operator intent or U.SpeechAct.
      Resolution is revalidated at transaction time and neither consumes the
      single-use key nor proves that CAS committed.
    aliases:
      - TypeEnv head-selection authority resolution
    not:
      - The generic profile AuthorityResolutionRecord.
      - ProjectTypeEnvHeadSelectionAuthorityUseRecord.
      - ProjectTypeEnvHeadSelectionReceiptV1.
    owners:
      - human
  - term: ProjectTypeEnvHeadSelectionAuthorityUseRecord
    category: authority
    definition: >-
      The TypeEnv-specific single-use record atomically written only by the
      original successful ProjectTypeEnvHead update. It binds the exact
      host-routed operator request, authority resolution, reviewed-content
      DescriptionRef and digest, selection-request ref and digest,
      IdempotencyKey, closed predecessor, B, ordered E DAG, X, C, Stage,
      ExpectedGraphRevision, committed
      ProjectTypeEnvHeadRevision and GraphRevision, verifier, original CAS
      WorkRef, receipt ref, and committed result. An exact idempotent replay
      returns the existing record without current-authority revalidation,
      another consumption, new Work, or another write; the same key with a
      different exact request or content digest is ReplayConflict. The record
      is neither the operator request nor a reusable permission.
    aliases:
      - TypeEnv authority-use record
    not:
      - The profile-specific AuthorityUseRecord.
      - A caller-supplied used flag.
      - Authority for a later Transition.
    owners:
      - human
  - term: ProjectTypeEnvHeadCASWorkRecord
    category: authority
    definition: >-
      The durable description containing the exact WorkRef of the one dated
      system CAS effect Work that compares the closed predecessor and
      ExpectedGraphRevision and atomically installs one
      ProjectTypeEnvHead result together with its authority-use record and
      ProjectTypeEnvHeadSelectionReceiptV1. As a reliance-bearing U.Work
      description it identifies the exact enactsMethod U.MethodRef and TypeEnv-head CAS
      methodDescriptionRef; performedBy as a U.RoleAssignmentRef whose holder is
      HaftSoftwareSystem and whose interval covers the Work interval;
      executedWithin U.SystemRef; bounded judgement context; Work interval;
      StatePlaneRef with exact pre/post head-and-revision refs or declared delta
      predicate; concrete parameter bindings, input/output refs, resource-ledger
      ref, outcome and audit-trace refs; affected project, closed predecessor,
      target ProjectTypeEnvHead slot, C, and Stage referents; exact request,
      HostRoutedOperatorRequest, DescriptionRef content, authority-resolution
      and authority-use refs;
      predecessor comparison; and committed head revision, graph revision,
      receipt, and result. The record is created only with the original CAS Work
      commit and returned unchanged on exact replay; replay creates no new Work
      occurrence. The record is not the Work occurrence, and the CAS Work is
      distinct from the host-routed operator request.
    aliases:
      - TypeEnv head CAS Work record
    not:
      - HostRoutedOperatorRequest.
      - ProjectTypeEnvStage.
      - Part of B, E, X, or C identity.
    owners:
      - human
  - term: ProjectRecordCarrierV1
    category: typed-memory
    definition: >-
      An immutable content-addressed candidate-feature carrier binding one stable
      EntityID, BoundedContextRef, carrier edition and one closed project-record
      variant. It is an evaluator input only: sealing or naming the carrier
      does not establish classification truth, Evidence, claim truth, approval,
      or authority.
    aliases:
      - project-record carrier
    not:
      - The represented ProjectRecord.
      - A caller-supplied U.Kind assertion.
      - Proof of a DecisionRecord approval.
    owners:
      - human
  - term: EntityRecordCarrierBindingV1
    category: typed-memory
    definition: >-
      An immutable content-addressed binding from one exact ProjectRecordCarrierV1
      to project, EntityID, bounded context, closed record variant, carrier ref,
      edition, schema and digest, exact mapping-manifest ref, and adapter
      version. It establishes correlation, not classification truth.
    aliases:
      - entity-record carrier binding
    not:
      - A KindClassificationJudgement.
      - A trusted source delivery by itself.
      - An authority record.
    owners:
      - human
  - term: RecordMembershipSourceV1
    category: typed-memory-legacy
    definition: >-
      A sealed legacy immutable correlation of an exact
      ProjectRecordCarrierV1 and EntityRecordCarrierBindingV1 for one project,
      EntityID, bounded context, variant, mapping manifest and adapter version.
      Historical TypeEnv editions may decode and replay it. Canonical
      verification makes its bytes internally exact but self-sealing does not
      make their producer trusted, make a current criterion true or false, or
      authorize emission under the current classification model.
    aliases:
      - record-membership source
    not:
      - TrustedRecordMembershipSourceDeliveryV1.
      - A current KindClassificationJudgement.
      - Approval or evidence of claim truth.
    owners:
      - human
  - term: RuleRef
    category: typed-memory
    definition: >-
      An exact immutable identifier of one deterministic validation or
      kind-classification rule implementation whose registration is pinned by the
      RuntimeEvaluationBasisArtifact. Equal labels without equal RuleRef and
      mechanism identity are not the same evaluation basis.
    aliases:
      - rule reference
    not:
      - A free-form diagnostic label.
      - A relation-signature ref.
      - Permission to run a binding effect.
    owners:
      - human
  - term: RecordMembershipEvaluatorRegistration
    category: typed-memory-legacy
    definition: >-
      The sealed legacy X-pinned registration that mapped one RuleRef to the
      earlier ProjectRecord MemberOf evaluator and trusted source-delivery
      boundary. Historical TypeEnv editions may decode and replay it, but new
      current editions register a kind-classification evaluator that applies a
      KindSignature criterion to governed candidate features. Either form
      supplies executable mechanism only and admits no kind, carrier, relation,
      classification truth, or project write.
    aliases:
      - record membership evaluator registration
    not:
      - ProjectRecord classification truth or Evidence.
      - A TypeEnv head-selection effect.
      - A generic plugin name.
    owners:
      - human
  - term: RecordAdapterMappingManifest
    category: typed-memory
    definition: >-
      A versioned immutable adapter contract whose content-addressed
      MappingManifestRef identifies the accepted domain input, closed
      ProjectRecord carrier variant, emitted Haft-local kind and relation
      coordinates, required bounded-context and authority posture, adapter
      version, and exact round-trip fixtures. The current classification
      evaluator, or a sealed legacy membership evaluator during exact replay,
      compares the source binding to this exact ref; a filename or adapter
      label is not equivalent.
    aliases:
      - record mapping manifest
      - MappingManifestRef
    not:
      - A RecordMembershipSourceV1.
      - Evidence that an adapter actually ran.
      - Permission to persist a record.
    owners:
      - human
  - term: TrustedRecordMembershipSourceDeliveryV1
    category: typed-memory-legacy
    definition: >-
      A sealed legacy non-serializable in-process capability produced only by the trusted
      immutable-store or adapter boundary after exact RecordMembershipSourceV1
      verification and producer-policy checks. It may be consumed only while
      replaying its exact historical TypeEnv edition. Current classification
      uses a trusted governed-feature delivery; neither delivery is Evidence or
      makes the candidate criterion true merely by existing.
    aliases:
      - trusted record-membership delivery
    not:
      - RecordMembershipSourceV1 bytes alone.
      - A public constructor argument.
      - Durable approval or authority.
    owners:
      - human
  - term: ClassificationUnknown
    category: typed-memory
    definition: >-
      The KindClassificationJudgement result returned when missing evidence, an
      unavailable declared dependency, out-of-domain input, or another missing
      or mismatched required basis prevents the pinned evaluator from
      determining true or false. It carries explicit missing basis and a repair
      pointer. A receiving guard may refuse use, but the result remains unknown.
    aliases:
      - unknown classification
    not:
      - A false classification.
      - A candidate state.
      - A guard or admission result.
      - A generic runtime error.
      - Permission to admit the proposed relation.
    owners:
      - human
  - term: MemberOfUndefined
    category: typed-memory-legacy
    definition: >-
      The sealed legacy Haft result spelling used by earlier MemberOf evaluator
      editions for non-settlement. It remains decodable and replayable against
      its exact historical TypeEnv edition and maps only to the legacy result;
      new current editions emit ClassificationUnknown without retroactively
      rewriting stored bytes.
    aliases:
      - legacy undefined membership
    not:
      - A current FPF C.3 classification name.
      - A negative classification proof.
      - Permission to admit the proposed relation.
    owners:
      - human
  - term: symbolic TypeEnv position
    category: typed-memory
    definition: >-
      A TypeEnv-scoped position in ProjectTypeEnvExtensionIR that names its
      semantic dependency without embedding the not-yet-derived composite
      TypeEnvRef. It becomes concrete only during verified composite lowering.
    aliases:
      - symbolic TypeEnv reference
    not:
      - The base TypeEnvRef substituted as a shortcut.
      - A caller-supplied composite ref.
      - An unresolved free-form string.
    owners:
      - human
  - term: lowered runtime declaration
    category: typed-memory
    definition: >-
      A concrete executable declaration rebuilt from verified base or extension
      source after C is derived, with every TypeEnv-scoped reference bound to
      that exact C while source provenance continues to name its exact B or E
      dependency.
    aliases:
      - C-bound declaration
    not:
      - A relabelled declaration copied from B.
      - The symbolic source declaration.
      - Evidence of ProjectTypeEnvHead selection by itself.
    owners:
      - human
  - term: ValueShape
    category: typed-memory
    definition: >-
      One member of the closed scalar, record, sum, ordered-sequence,
      unordered-set, or dedicated ClaimGraph value-shape algebra used by the
      pure verifier. Arbitrary JSON is not a ValueShape.
    aliases:
      - value shape
    not:
      - A dynamic map.
      - A codec implementation.
      - Project admission by itself.
    owners:
      - human
  - term: ValueKindRef
    category: typed-memory
    definition: >-
      A strong reference to one admitted ValueKind in one exact TypeEnv digest.
    aliases:
      - value-kind reference
    not:
      - A free kind label.
      - A ValueShapeRef.
    owners:
      - human
  - term: ValueShapeRef
    category: typed-memory
    definition: >-
      A strong content-addressed reference to one exact ValueShape declaration.
    aliases:
      - value-shape reference
    not:
      - A ValueKindRef.
      - A CodecRef.
    owners:
      - human
  - term: CodecRef
    category: typed-memory
    definition: >-
      An immutable reference containing CodecID, canonicalization version, and
      codec-spec digest. Changed canonicalization semantics require a new
      CodecRef rather than mutating the meaning of stored bytes.
    aliases:
      - codec reference
    not:
      - A mutable implementation name.
      - TypeEnv admission by itself.
    owners:
      - human
  - term: CodecRegistry
    category: typed-memory
    definition: >-
      The executable mechanism mapping an exact CodecRef to a pure decode,
      shape-check, normalize, and canonical-encode implementation. Registration
      supplies mechanism only; the exact composite TypeEnv selected by the
      current ProjectTypeEnvHead separately admits the exact ValueKindRef,
      ValueShapeRef, and CodecRef binding.
    aliases:
      - codec registry
    not:
      - A TypeEnv extension.
      - A kind or value admission.
    owners:
      - human
  - term: ByValue
    category: typed-memory
    definition: >-
      The A.6.5 slot reference mode that stores a verifier-created
      VerifiedTypedValue directly in the relation instance rather than a
      StrongRef. Canonical project memory accepts no arbitrary decoded JSON in
      this position.
    aliases:
      - by-value filler
    not:
      - A RefKind.
      - A caller-verified value.
      - Permission to omit the exact ValueKindRef, ValueShapeRef, or CodecRef.
    owners:
      - human
  - term: VerifiedTypedValue
    category: typed-memory
    definition: >-
      A value privately constructed by the pure verifier after exact binding to
      the composite TypeEnv selected by the current ProjectTypeEnvHead and codec
      round-trip checks. It binds ValueKindRef, ValueShapeRef, CodecRef,
      canonical bytes, and a domain-separated length-prefixed digest over all
      of them.
    aliases:
      - verified typed value
    not:
      - Arbitrary JSON.
      - A caller assertion that validation occurred.
      - A registered codec without admission in the head-selected composite.
    owners:
      - human
  - term: ClaimGraphCodecV1
    category: typed-memory
    definition: >-
      The dedicated canonical codec required for the exactly one ByValue
      ClaimGraphSlot in every supported C.2.1 episteme signature. It
      canonicalizes unordered typed ClaimNode and ClaimEdge sets, preserves
      explicitly governed order, and rejects duplicate node identities and
      dangling endpoints.
    aliases:
      - claim-graph codec v1
    not:
      - A generic JSON codec.
      - A project relation signature.
      - Evidence that claims are true.
    owners:
      - human
  - term: OpaqueStoredValue
    category: typed-memory
    definition: >-
      An inspect-only historical value preserving exact kind, shape, codec,
      canonical bytes, digest, and provenance when its codec implementation or
      governing rule is unavailable. It cannot be silently revalidated or used
      as a new admissible write.
    aliases:
      - opaque stored value
    not:
      - A VerifiedTypedValue.
      - Invalid merely because old mechanism is unavailable.
      - Permission to discard historical bytes.
    owners:
      - human
  - term: Admission
    category: typed-memory
    definition: >-
      The operation that combines source-backed semantic validation, applicable
      authority checks, and atomic canonical persistence against the exact
      composite TypeEnv selected by the current ProjectTypeEnvHead and
      ExpectedGraphRevision. Validation alone is read-only and is not Admission.
    aliases:
      - semantic admission
    not:
      - Validation alone.
      - Automatic authority to bind decisions, commissions, or spec lifecycle.
      - Projection success.
    owners:
      - human
  - term: GraphRevision
    category: typed-memory
    definition: >-
      A strong monotonic project-local revision of canonical semantic memory
      used for compare-and-swap admission, read provenance, and
      derived-projection invalidation. It is distinct from
      ProjectTypeEnvHeadRevision: a head-selection transaction may record both,
      but their values, domains, comparison rules, and meanings never collapse.
    aliases:
      - graph revision
      - project graph revision
    not:
      - ProjectTypeEnvHeadRevision.
      - A Git commit.
      - A causal or work-order counter.
      - A carrier edition.
    owners:
      - human
  - term: ExpectedGraphRevision
    category: typed-memory
    definition: >-
      The strong compare-and-swap expectation for one exact project
      GraphRevision. ProjectTypeEnvStage and
      ProjectTypeEnvHeadSelectionRequest carry this value and must agree with
      each other and with NoPriorHeadProof when Genesis is requested. It is not
      a generic revision field and cannot carry or be compared as
      ProjectTypeEnvHeadRevision.
    aliases:
      - expected graph revision
    not:
      - ExpectedPriorHead revision.
      - ProjectTypeEnvHeadRevision.
      - A zero sentinel for Genesis.
    owners:
      - human
  - term: ExactNeighborhood
    category: memory-recall
    definition: >-
      A read result containing the typed project-memory neighborhood for one
      exact Entity reference and BoundedContext at one graph revision and
      TypeEnv digest, recovered before any relevance ranking.
    aliases:
      - exact neighborhood
    not:
      - A ranked candidate set.
      - A global project story.
      - A next-action prescription.
    owners:
      - human
  - term: RecallCandidateSet
    category: memory-recall
    definition: >-
      A project-memory recall result containing unresolved Entity candidates and
      memory candidates with exact contexts, retrieval bases, witnesses,
      producer versions, digests, and truncation. A caller must resolve an exact
      Entity and context before treating recalled relations as current structure.
    aliases:
      - recall candidate set
    not:
      - An admitted relation.
      - A merged cross-context identity.
      - A recommendation or work order.
    owners:
      - human
  - term: RecallUnit
    category: memory-recall
    definition: >-
      A rebuildable derived search projection over canonical or legacy project
      memory, carrying Entity and context identity, typed relation posture,
      searchable text, provenance, semantic posture, graph revision, TypeEnv and
      content digests, and projection schema version.
    aliases:
      - recall unit
    not:
      - A canonical semantic assertion.
      - A source of ContextKindAvailability, KindClassificationJudgement, or durable U-kind admission.
      - An FPF SourceUnit.
    owners:
      - human
  - term: Projection debt
    category: typed-memory
    definition: >-
      A durable retryable record that canonical semantic Admission succeeded but
      one or more derived carriers, graph views, or recall indexes were not
      projected successfully.
    aliases:
      - projection-debt
    not:
      - A rollback of the semantic commit.
      - Permission to hide stale projections.
      - A semantic validation failure.
    owners:
      - human
  - term: TargetSystemSpec
    category: specification
    definition: >-
      A typed human-readable specification description of a scope whose current
      EntityOfConcern is explicitly admitted as a target system. It states the
      intended observable change, target role, boundaries, interfaces,
      invariants, risks, and acceptance evidence without becoming the target
      system or performed project Work.
    aliases:
      - target system spec
    not:
      - The target system itself.
      - A universal carrier for every EntityOfConcern.
      - A requirement for a scope whose target-system applicability is
        NotApplicable or Underdetermined.
    owners:
      - human
  - term: SoftwareSystemSpec
    category: specification
    definition: >-
      The typed human-readable specification description of software that
      realizes an applicable project scope: its role, responsibility allocation,
      externally meaningful behavior, interfaces, constraints, and selected
      structure. It describes the software object, not performed engineering
      work or team, agent, release, and evidence-production policy.
    aliases:
      - software system spec
    not:
      - The software system itself.
      - A universal requirement for non-software realization scopes.
      - An engineering process or delivery-policy carrier.
    owners:
      - human
  - term: ProjectSpecificationSet
    category: specification
    definition: >-
      A profile-aware project-local projection consisting of the TermMap, only
      the concern and realization specification carriers applicable to current
      scopes, and the provenance of profile and applicability results.
    aliases:
      - project specification set
      - PSS
    not:
      - A universal fixed target-software-term-map triple.
      - The Haft runtime.
      - Proof that specified behavior exists in reality.
    owners:
      - human
```

Review must compare these meanings with the final `SoftwareSystemSpec` draft
and exact FPF source before any lifecycle action. Approval, rebaseline,
migration apply, and `ProjectTypeEnvHead` selection remain separate explicit
human gates.
