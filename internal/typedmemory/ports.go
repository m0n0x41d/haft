package typedmemory

import "fmt"

type StrongReferenceResolution interface {
	Reference() StrongRef
	Context() BoundedContextRef
	strongReferenceResolutionVariant()
}

type ResolvedStrongReference struct {
	reference StrongRef
	entity    EntityID
	context   BoundedContextRef
	basis     ResolutionBasisRef
}

func NewResolvedStrongReference(
	reference StrongRef,
	entity EntityID,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) (ResolvedStrongReference, error) {
	if !validStrongRef(reference) ||
		!entity.valid() ||
		!context.valid() ||
		!basis.valid() {
		return ResolvedStrongReference{}, fmt.Errorf("resolved strong reference, stable entity, context, and resolution basis are required")
	}
	return ResolvedStrongReference{
		reference: reference,
		entity:    entity,
		context:   context,
		basis:     basis,
	}, nil
}

func (resolution ResolvedStrongReference) Reference() StrongRef { return resolution.reference }

func (resolution ResolvedStrongReference) Entity() EntityID { return resolution.entity }

func (resolution ResolvedStrongReference) Context() BoundedContextRef { return resolution.context }

func (resolution ResolvedStrongReference) Basis() ResolutionBasisRef { return resolution.basis }

func (ResolvedStrongReference) strongReferenceResolutionVariant() {}

type UnresolvedStrongReference struct {
	reference StrongRef
	context   BoundedContextRef
	repair    RepairPointer
}

func NewUnresolvedStrongReference(
	reference StrongRef,
	context BoundedContextRef,
	repair RepairPointer,
) (UnresolvedStrongReference, error) {
	if !validStrongRef(reference) || !context.valid() || !repair.valid() {
		return UnresolvedStrongReference{}, fmt.Errorf("unresolved reference, context, and repair pointer are required")
	}
	return UnresolvedStrongReference{reference: reference, context: context, repair: repair}, nil
}

func (resolution UnresolvedStrongReference) Reference() StrongRef { return resolution.reference }

func (resolution UnresolvedStrongReference) Context() BoundedContextRef { return resolution.context }

func (resolution UnresolvedStrongReference) Repair() RepairPointer { return resolution.repair }

func (UnresolvedStrongReference) strongReferenceResolutionVariant() {}

// MissingContextBridgeResolution reports that the reference is known in a
// source context but cannot be used in the requested target context without an
// exact TypeEnv bridge. It never authorizes the bridge by itself.
type MissingContextBridgeResolution struct {
	reference     StrongRef
	sourceContext BoundedContextRef
	targetContext BoundedContextRef
	sourceKind    KindID
	targetKind    KindID
	repair        RepairPointer
}

func NewMissingContextBridgeResolution(
	reference StrongRef,
	sourceContext BoundedContextRef,
	targetContext BoundedContextRef,
	sourceKind KindID,
	targetKind KindID,
	repair RepairPointer,
) (MissingContextBridgeResolution, error) {
	if !validStrongRef(reference) ||
		!sourceContext.valid() ||
		!targetContext.valid() ||
		sourceContext == targetContext ||
		!sourceKind.valid() ||
		!targetKind.valid() ||
		!repair.valid() {
		return MissingContextBridgeResolution{}, fmt.Errorf("missing bridge resolution requires reference, distinct contexts, kinds, and repair")
	}
	return MissingContextBridgeResolution{
		reference:     reference,
		sourceContext: sourceContext,
		targetContext: targetContext,
		sourceKind:    sourceKind,
		targetKind:    targetKind,
		repair:        repair,
	}, nil
}

func (resolution MissingContextBridgeResolution) Reference() StrongRef {
	return resolution.reference
}

func (resolution MissingContextBridgeResolution) Context() BoundedContextRef {
	return resolution.targetContext
}

func (resolution MissingContextBridgeResolution) SourceContext() BoundedContextRef {
	return resolution.sourceContext
}

func (resolution MissingContextBridgeResolution) SourceKind() KindID {
	return resolution.sourceKind
}

func (resolution MissingContextBridgeResolution) TargetKind() KindID {
	return resolution.targetKind
}

func (resolution MissingContextBridgeResolution) Repair() RepairPointer {
	return resolution.repair
}

func (MissingContextBridgeResolution) strongReferenceResolutionVariant() {}

type AssertionState interface {
	Assertion() AssertionID
	assertionStateVariant()
}

type ActiveAssertion struct {
	assertion AssertionID
	basis     RuleRef
}

func NewActiveAssertion(
	assertion AssertionID,
	basis RuleRef,
) (ActiveAssertion, error) {
	if !assertion.valid() || !basis.valid() {
		return ActiveAssertion{}, fmt.Errorf("active assertion ID and observation rule are required")
	}
	return ActiveAssertion{assertion: assertion, basis: basis}, nil
}

func (state ActiveAssertion) Assertion() AssertionID { return state.assertion }

func (state ActiveAssertion) Basis() RuleRef { return state.basis }

func (ActiveAssertion) assertionStateVariant() {}

type AbsentAssertionState struct {
	assertion AssertionID
	basis     RuleRef
}

func NewAbsentAssertionState(
	assertion AssertionID,
	basis RuleRef,
) (AbsentAssertionState, error) {
	if !assertion.valid() || !basis.valid() {
		return AbsentAssertionState{}, fmt.Errorf("absent assertion and observation basis are required")
	}
	return AbsentAssertionState{assertion: assertion, basis: basis}, nil
}

func (state AbsentAssertionState) Assertion() AssertionID { return state.assertion }

func (state AbsentAssertionState) Basis() RuleRef { return state.basis }

func (AbsentAssertionState) assertionStateVariant() {}

type RetractedAssertionState struct {
	assertion AssertionID
	rule      RuleRef
}

func NewRetractedAssertionState(
	assertion AssertionID,
	rule RuleRef,
) (RetractedAssertionState, error) {
	if !assertion.valid() || !rule.valid() {
		return RetractedAssertionState{}, fmt.Errorf("retracted assertion and rule are required")
	}
	return RetractedAssertionState{assertion: assertion, rule: rule}, nil
}

func (state RetractedAssertionState) Assertion() AssertionID { return state.assertion }

func (state RetractedAssertionState) Rule() RuleRef { return state.rule }

func (RetractedAssertionState) assertionStateVariant() {}

type UnknownAssertionState struct {
	assertion AssertionID
	repair    RepairPointer
}

func NewUnknownAssertionState(
	assertion AssertionID,
	repair RepairPointer,
) (UnknownAssertionState, error) {
	if !assertion.valid() || !repair.valid() {
		return UnknownAssertionState{}, fmt.Errorf("unknown assertion and repair pointer are required")
	}
	return UnknownAssertionState{assertion: assertion, repair: repair}, nil
}

func (state UnknownAssertionState) Assertion() AssertionID { return state.assertion }

func (state UnknownAssertionState) Repair() RepairPointer { return state.repair }

func (UnknownAssertionState) assertionStateVariant() {}

// MemorySnapshot is an immutable fact view supplied by an outer adapter. It
// answers only the bases needed by pure validation; it performs no I/O itself.
type MemorySnapshot interface {
	GraphRevision() GraphRevision
	TypeEnvRef() TypeEnvRef
	ResolveEntity(EntityID, BoundedContextRef) EntityResolution
	ResolveReference(StrongRef, BoundedContextRef) StrongReferenceResolution
	EvaluateMemberOf(MemberOfEvaluationRequest) MemberOfJudgement
	AssertionState(AssertionID) AssertionState
	ResolveAlias(EntityAlias, BoundedContextRef) AliasAvailability
	ResolveReconciliationBasis(
		ReconciliationBasisRef,
		BoundedContextRef,
	) ReconciliationBasisResolution
}

// KindClassificationSnapshot is the current C.3.2 evaluation capability. It
// stays separate from MemorySnapshot so sealed historical snapshot adapters
// can replay their exact MemberOf contract without pretending to implement
// current classification.
type KindClassificationSnapshot interface {
	EvaluateKindClassification(
		KindClassificationRequest,
	) KindClassificationJudgement
}

// KindClassificationAdmissionSnapshot evaluates the same four-input current
// request while retaining the exact delivery visibility of this admission
// use. The visibility context is not a fifth C.3.2 semantic input; it prevents
// a same-batch declaration from being mislabeled as a persisted-snapshot fact
// and makes commit-time revalidation byte-identical.
type KindClassificationAdmissionSnapshot interface {
	KindClassificationSnapshot
	EvaluateKindClassificationForAdmission(
		KindClassificationRequest,
		AdmissionReferenceResolution,
		uint64,
		OrderedCandidatePrefix,
	) KindClassificationJudgement
}

// DisjointEntailmentSnapshot is the sealed historical exact-proof port used by
// a legacy admission transaction after it has revalidated a KindDisjoint
// entailment against its exact TypeEnv and positive MemberOf support. The
// support is part of lookup identity: distinct positive memberships may share
// one counter request and constraint without becoming the same proof. This
// port is not the current C.3 classification contract.
type DisjointEntailmentSnapshot interface {
	ResolveDisjointEntailment(
		MemberOfEvaluationRequest,
		ConstraintID,
		MemberOfMember,
	) (DisjointEntailmentUse, bool)
}

// SnapshotPort and TransactionPort mark the effect boundary required by later
// admission work. P5 defines the capabilities but supplies no implementation.
type SnapshotPort interface {
	Load(TypeEnvRef) (MemorySnapshot, error)
}

type MemoryTransaction interface {
	Snapshot() MemorySnapshot
	// Append accepts the opaque AdmissionBatch produced by a successful pure
	// validation. Implementations reject !batch.IsValid(); a wrapped Valid
	// interface cannot forge or replace this concrete capability.
	Append(AdmissionBatch) (GraphRevision, error)
}

type TransactionPort interface {
	Begin(GraphRevision, TypeEnvRef) (MemoryTransaction, error)
}
