// Package projectgraphobservation owns pure immutable values that describe one
// exact typed-memory graph snapshot. Storage adapters establish currentness;
// these values grant no read, mutation, Stage-selection, or authority capability.
package projectgraphobservation

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const activeAtObservedRevision = "active_at_observed_revision"

// ExactTargetMemberOfFactView is the read-only capability for evaluating one
// persisted-snapshot MemberOf request against an exact target C and graph
// snapshot. Storage adapters mint it from current entity/observable facts;
// historical judgements under another C are not part of this port.
//
// The capability grants no graph read, mutation, Stage selection, or readiness
// policy. Pure consumers must correlate both exposed coordinates before using
// a judgement.
type ExactTargetMemberOfFactView interface {
	TargetTypeEnv() typedmemory.TypeEnvRef
	GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis
	EvaluateMemberOf(
		typedmemory.MemberOfEvaluationRequest,
	) typedmemory.MemberOfJudgement
}

// ExactTargetKindClassificationFactView is the current read-only C.3.2
// capability for one exact target C and graph snapshot. It is intentionally
// disjoint from historical MemberOf: callers cannot reinterpret one judgement
// family as the other.
type ExactTargetKindClassificationFactView interface {
	TargetTypeEnv() typedmemory.TypeEnvRef
	GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis
	EvaluateKindClassification(
		typedmemory.KindClassificationRequest,
	) typedmemory.KindClassificationJudgement
}

// ExactTargetReferenceKindFactView is the closed target-C reference-kind
// evaluation posture. A target exposes historical MemberOf, current direct
// KindClassification, or explicitly no reference-kind mechanism. The empty
// posture is a real coordinate-bearing variant, never a nil/fallback value.
type ExactTargetReferenceKindFactView interface {
	TargetTypeEnv() typedmemory.TypeEnvRef
	GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis
	exactTargetReferenceKindFactViewVariant()
}

// ExactTargetMemberOfReferenceKindFacts is the sealed compatibility variant
// for a historical target C.
type ExactTargetMemberOfReferenceKindFacts struct {
	view ExactTargetMemberOfFactView
}

func NewExactTargetMemberOfReferenceKindFacts(
	view ExactTargetMemberOfFactView,
) (ExactTargetMemberOfReferenceKindFacts, error) {
	if view == nil {
		return ExactTargetMemberOfReferenceKindFacts{}, fmt.Errorf(
			"exact target MemberOf fact view is required",
		)
	}
	if err := view.GraphSnapshotBasis().Verify(); err != nil {
		return ExactTargetMemberOfReferenceKindFacts{}, fmt.Errorf(
			"exact target MemberOf graph basis: %w",
			err,
		)
	}
	if _, err := typedmemory.ParseTypeEnvRef(view.TargetTypeEnv().String()); err != nil {
		return ExactTargetMemberOfReferenceKindFacts{}, fmt.Errorf(
			"exact target MemberOf TypeEnv is invalid: %w",
			err,
		)
	}
	return ExactTargetMemberOfReferenceKindFacts{view: view}, nil
}

func (facts ExactTargetMemberOfReferenceKindFacts) TargetTypeEnv() typedmemory.TypeEnvRef {
	return facts.view.TargetTypeEnv()
}

func (facts ExactTargetMemberOfReferenceKindFacts) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return facts.view.GraphSnapshotBasis()
}

func (facts ExactTargetMemberOfReferenceKindFacts) MemberOfFacts() ExactTargetMemberOfFactView {
	return facts.view
}

func (ExactTargetMemberOfReferenceKindFacts) exactTargetReferenceKindFactViewVariant() {}

// ExactTargetKindClassificationReferenceKindFacts is the sealed current
// target-C variant.
type ExactTargetKindClassificationReferenceKindFacts struct {
	view ExactTargetKindClassificationFactView
}

func NewExactTargetKindClassificationReferenceKindFacts(
	view ExactTargetKindClassificationFactView,
) (ExactTargetKindClassificationReferenceKindFacts, error) {
	if view == nil {
		return ExactTargetKindClassificationReferenceKindFacts{}, fmt.Errorf(
			"exact target KindClassification fact view is required",
		)
	}
	if err := view.GraphSnapshotBasis().Verify(); err != nil {
		return ExactTargetKindClassificationReferenceKindFacts{}, fmt.Errorf(
			"exact target KindClassification graph basis: %w",
			err,
		)
	}
	if _, err := typedmemory.ParseTypeEnvRef(view.TargetTypeEnv().String()); err != nil {
		return ExactTargetKindClassificationReferenceKindFacts{}, fmt.Errorf(
			"exact target KindClassification TypeEnv is invalid: %w",
			err,
		)
	}
	return ExactTargetKindClassificationReferenceKindFacts{view: view}, nil
}

func (facts ExactTargetKindClassificationReferenceKindFacts) TargetTypeEnv() typedmemory.TypeEnvRef {
	return facts.view.TargetTypeEnv()
}

func (facts ExactTargetKindClassificationReferenceKindFacts) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return facts.view.GraphSnapshotBasis()
}

func (facts ExactTargetKindClassificationReferenceKindFacts) KindClassificationFacts() ExactTargetKindClassificationFactView {
	return facts.view
}

func (ExactTargetKindClassificationReferenceKindFacts) exactTargetReferenceKindFactViewVariant() {}

// ExactTargetNoReferenceKindFacts is the sealed posture for a target whose X
// declares neither reference-kind mechanism. It allows targets without
// reference relation semantics to remain explicit while mixed runtimes still
// fail closed.
type ExactTargetNoReferenceKindFacts struct {
	target typedmemory.TypeEnvRef
	basis  projecttypeenvselection.ProjectGraphSnapshotBasis
}

func NewExactTargetNoReferenceKindFacts(
	target typedmemory.TypeEnvRef,
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
) (ExactTargetNoReferenceKindFacts, error) {
	if _, err := typedmemory.ParseTypeEnvRef(target.String()); err != nil {
		return ExactTargetNoReferenceKindFacts{}, fmt.Errorf(
			"exact target no-reference-kind TypeEnv is invalid: %w",
			err,
		)
	}
	if err := basis.Verify(); err != nil {
		return ExactTargetNoReferenceKindFacts{}, fmt.Errorf(
			"exact target no-reference-kind graph basis: %w",
			err,
		)
	}
	return ExactTargetNoReferenceKindFacts{
		target: target,
		basis:  basis,
	}, nil
}

func (facts ExactTargetNoReferenceKindFacts) TargetTypeEnv() typedmemory.TypeEnvRef {
	return facts.target
}

func (facts ExactTargetNoReferenceKindFacts) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return facts.basis
}

func (ExactTargetNoReferenceKindFacts) exactTargetReferenceKindFactViewVariant() {}

// CurrentAssertionCarrierKind identifies the exact closed carrier recovered
// for one durable assertion origin. A legacy RelationInstance and an explicit
// v3 RelationalAssertion are intentionally disjoint: the latter is never
// lowered into the former and therefore never acquires occurrence semantics.
type CurrentAssertionCarrierKind string

const (
	CurrentLegacyRelationCarrier        CurrentAssertionCarrierKind = "legacy_relation_instance"
	CurrentRelationalAssertionV3Carrier CurrentAssertionCarrierKind = "relational_assertion_v3"
)

// CurrentAssertionCarrier is the closed structural view shared by the two
// exact durable assertion carriers. Its common fields are safe for structural
// revalidation; callers that need carrier-specific meaning must type-switch on
// the two variants below.
type CurrentAssertionCarrier interface {
	Kind() CurrentAssertionCarrierKind
	AssertionID() typedmemory.AssertionID
	RelationDeclarationFragmentRef() typedmemory.TypedRelationDeclarationFragmentRef
	// Signature is the historical carrier accessor for the same coordinate.
	Signature() typedmemory.RelationSignatureRef
	RelationDeclarationPosture() typedmemory.RelationDeclarationPosture
	Slice() typedmemory.ContextSlice
	Context() typedmemory.BoundedContextRef
	Bindings() []typedmemory.SlotBinding
	Provenance() typedmemory.ProvenanceRef
	currentAssertionCarrierVariant()
}

// CurrentLegacyRelation is the compatibility variant for historical exact
// RelationInstance reads.
type CurrentLegacyRelation struct {
	relation typedmemory.RelationInstance
}

func (carrier CurrentLegacyRelation) Kind() CurrentAssertionCarrierKind {
	return CurrentLegacyRelationCarrier
}

func (carrier CurrentLegacyRelation) AssertionID() typedmemory.AssertionID {
	return carrier.relation.Assertion()
}

func (carrier CurrentLegacyRelation) Signature() typedmemory.RelationSignatureRef {
	return carrier.relation.Signature()
}

func (carrier CurrentLegacyRelation) RelationDeclarationFragmentRef() typedmemory.TypedRelationDeclarationFragmentRef {
	return carrier.relation.Signature()
}

func (CurrentLegacyRelation) RelationDeclarationPosture() typedmemory.RelationDeclarationPosture {
	return typedmemory.RelationDeclarationTypedFragment
}

func (carrier CurrentLegacyRelation) Slice() typedmemory.ContextSlice {
	return carrier.relation.Slice()
}

func (carrier CurrentLegacyRelation) Context() typedmemory.BoundedContextRef {
	return carrier.relation.Context()
}

func (carrier CurrentLegacyRelation) Bindings() []typedmemory.SlotBinding {
	return carrier.relation.Bindings()
}

func (carrier CurrentLegacyRelation) Provenance() typedmemory.ProvenanceRef {
	return carrier.relation.Provenance()
}

func (carrier CurrentLegacyRelation) Relation() typedmemory.RelationInstance {
	return carrier.relation
}

func (CurrentLegacyRelation) currentAssertionCarrierVariant() {}

// CurrentRelationalAssertion is the exact v3 carrier. Its explicit modality
// remains assertion content and cannot designate or prove an occurrence.
type CurrentRelationalAssertion struct {
	assertion typedmemory.RelationalAssertion
}

func (carrier CurrentRelationalAssertion) Kind() CurrentAssertionCarrierKind {
	return CurrentRelationalAssertionV3Carrier
}

func (carrier CurrentRelationalAssertion) AssertionID() typedmemory.AssertionID {
	return carrier.assertion.Assertion()
}

func (carrier CurrentRelationalAssertion) Signature() typedmemory.RelationSignatureRef {
	return carrier.assertion.Signature()
}

func (carrier CurrentRelationalAssertion) RelationDeclarationFragmentRef() typedmemory.TypedRelationDeclarationFragmentRef {
	return carrier.assertion.Signature()
}

func (CurrentRelationalAssertion) RelationDeclarationPosture() typedmemory.RelationDeclarationPosture {
	return typedmemory.RelationDeclarationTypedFragment
}

func (carrier CurrentRelationalAssertion) Slice() typedmemory.ContextSlice {
	return carrier.assertion.Slice()
}

func (carrier CurrentRelationalAssertion) Context() typedmemory.BoundedContextRef {
	return carrier.assertion.Context()
}

func (carrier CurrentRelationalAssertion) Bindings() []typedmemory.SlotBinding {
	return carrier.assertion.Bindings()
}

func (carrier CurrentRelationalAssertion) Provenance() typedmemory.ProvenanceRef {
	return carrier.assertion.Provenance()
}

func (carrier CurrentRelationalAssertion) Assertion() typedmemory.RelationalAssertion {
	return carrier.assertion
}

func (CurrentRelationalAssertion) currentAssertionCarrierVariant() {}

// CurrentAssertionPosture is the posture of an assertion at one exact graph
// snapshot. CurrentActiveAssertionSet admits only the active variant.
type CurrentAssertionPosture struct {
	value       string
	carrierKind CurrentAssertionCarrierKind
	modality    typedmemory.AssertionModalityKind
}

func (posture CurrentAssertionPosture) String() string {
	return posture.value
}

// ExplicitModality returns the sealed v3 assertion posture when one was
// present in the durable carrier. Legacy bytes return false rather than being
// reinterpreted as any v3 modality.
func (posture CurrentAssertionPosture) ExplicitModality() (
	typedmemory.AssertionModalityKind,
	bool,
) {
	if posture.carrierKind != CurrentRelationalAssertionV3Carrier {
		return "", false
	}
	return posture.modality, true
}

// CurrentActiveAssertion is one exact active assertion carrier plus its
// durable origin. It does not establish validity under a future TypeEnv and a
// v3 assertion is never converted into a legacy RelationInstance.
type CurrentActiveAssertion struct {
	carrier        CurrentAssertionCarrier
	canonicalBytes []byte
	digest         typedmemory.SHA256Digest
	originEvent    projecttypeenvselection.GraphEventRef
	originRevision typedmemory.GraphRevision
	changeOrdinal  uint64
	posture        CurrentAssertionPosture
}

type CurrentActiveAssertionInput struct {
	Relation       typedmemory.RelationInstance
	CanonicalBytes []byte
	Digest         typedmemory.SHA256Digest
	OriginEvent    projecttypeenvselection.GraphEventRef
	OriginRevision typedmemory.GraphRevision
	ChangeOrdinal  uint64
}

type CurrentActiveRelationalAssertionInput struct {
	Assertion      typedmemory.RelationalAssertion
	CanonicalBytes []byte
	Digest         typedmemory.SHA256Digest
	OriginEvent    projecttypeenvselection.GraphEventRef
	OriginRevision typedmemory.GraphRevision
	ChangeOrdinal  uint64
}

func NewCurrentActiveAssertion(
	input CurrentActiveAssertionInput,
) (CurrentActiveAssertion, error) {
	relation, err := typedmemory.DecodeCanonicalRelationInstance(input.CanonicalBytes)
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion canonical relation: %w",
			err,
		)
	}
	reencoded, err := relation.CanonicalBytes()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion re-encode: %w",
			err,
		)
	}
	digest, err := relation.Digest()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion digest: %w",
			err,
		)
	}
	inputCanonical, err := input.Relation.CanonicalBytes()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion input relation: %w",
			err,
		)
	}
	matches := bytes.Equal(reencoded, input.CanonicalBytes) &&
		bytes.Equal(inputCanonical, input.CanonicalBytes) &&
		digest == input.Digest
	if !matches {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion input differs from canonical relation",
		)
	}
	return newCurrentActiveAssertion(
		CurrentLegacyRelation{relation: relation},
		input.CanonicalBytes,
		digest,
		input.OriginEvent,
		input.OriginRevision,
		input.ChangeOrdinal,
		CurrentAssertionPosture{
			value:       activeAtObservedRevision,
			carrierKind: CurrentLegacyRelationCarrier,
		},
	)
}

func NewCurrentActiveRelationalAssertion(
	input CurrentActiveRelationalAssertionInput,
) (CurrentActiveAssertion, error) {
	assertion, err := typedmemory.DecodeCanonicalRelationalAssertion(
		input.CanonicalBytes,
	)
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion canonical v3 assertion: %w",
			err,
		)
	}
	reencoded, err := assertion.CanonicalBytes()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion v3 re-encode: %w",
			err,
		)
	}
	digest, err := assertion.Digest()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion v3 digest: %w",
			err,
		)
	}
	inputCanonical, err := input.Assertion.CanonicalBytes()
	if err != nil {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion input v3 assertion: %w",
			err,
		)
	}
	matches := bytes.Equal(reencoded, input.CanonicalBytes) &&
		bytes.Equal(inputCanonical, input.CanonicalBytes) &&
		digest == input.Digest &&
		assertion.Modality().Kind() == input.Assertion.Modality().Kind()
	if !matches {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion input differs from canonical v3 assertion",
		)
	}
	return newCurrentActiveAssertion(
		CurrentRelationalAssertion{assertion: assertion},
		input.CanonicalBytes,
		digest,
		input.OriginEvent,
		input.OriginRevision,
		input.ChangeOrdinal,
		CurrentAssertionPosture{
			value:       activeAtObservedRevision,
			carrierKind: CurrentRelationalAssertionV3Carrier,
			modality:    assertion.Modality().Kind(),
		},
	)
}

func newCurrentActiveAssertion(
	carrier CurrentAssertionCarrier,
	canonicalBytes []byte,
	digest typedmemory.SHA256Digest,
	originEvent projecttypeenvselection.GraphEventRef,
	originRevision typedmemory.GraphRevision,
	changeOrdinal uint64,
	posture CurrentAssertionPosture,
) (CurrentActiveAssertion, error) {
	event, err := projecttypeenvselection.ParseGraphEventRef(originEvent.String())
	if err != nil || event != originEvent {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion origin event is required",
		)
	}
	if originRevision.Value() == 0 {
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion origin revision must be non-zero",
		)
	}
	return CurrentActiveAssertion{
		carrier:        carrier,
		canonicalBytes: append([]byte(nil), canonicalBytes...),
		digest:         digest,
		originEvent:    event,
		originRevision: originRevision,
		changeOrdinal:  changeOrdinal,
		posture:        posture,
	}, nil
}

func (assertion CurrentActiveAssertion) AssertionID() typedmemory.AssertionID {
	return assertion.carrier.AssertionID()
}

// Relation preserves the historical legacy API. Callers consuming a mixed
// current graph must inspect Carrier instead; v3 assertions intentionally do
// not manufacture a RelationInstance.
func (assertion CurrentActiveAssertion) Relation() typedmemory.RelationInstance {
	relation, _ := assertion.LegacyRelation()
	return relation
}

func (assertion CurrentActiveAssertion) LegacyRelation() (
	typedmemory.RelationInstance,
	bool,
) {
	carrier, ok := assertion.carrier.(CurrentLegacyRelation)
	if !ok {
		return typedmemory.RelationInstance{}, false
	}
	return carrier.Relation(), true
}

func (assertion CurrentActiveAssertion) RelationalAssertion() (
	typedmemory.RelationalAssertion,
	bool,
) {
	carrier, ok := assertion.carrier.(CurrentRelationalAssertion)
	if !ok {
		return typedmemory.RelationalAssertion{}, false
	}
	return carrier.Assertion(), true
}

func (assertion CurrentActiveAssertion) Carrier() CurrentAssertionCarrier {
	return assertion.carrier
}

func (assertion CurrentActiveAssertion) CanonicalBytes() []byte {
	return append([]byte(nil), assertion.canonicalBytes...)
}

func (assertion CurrentActiveAssertion) Digest() typedmemory.SHA256Digest {
	return assertion.digest
}

func (assertion CurrentActiveAssertion) OriginEvent() projecttypeenvselection.GraphEventRef {
	return assertion.originEvent
}

func (assertion CurrentActiveAssertion) OriginRevision() typedmemory.GraphRevision {
	return assertion.originRevision
}

func (assertion CurrentActiveAssertion) ChangeOrdinal() uint64 {
	return assertion.changeOrdinal
}

func (assertion CurrentActiveAssertion) Posture() CurrentAssertionPosture {
	return assertion.posture
}

func (assertion CurrentActiveAssertion) Verify() error {
	canonical, err := reconstructCurrentActiveAssertion(assertion)
	if err != nil {
		return err
	}
	if canonical.posture != assertion.posture {
		return fmt.Errorf("current active assertion posture is inconsistent")
	}
	return nil
}

func reconstructCurrentActiveAssertion(
	assertion CurrentActiveAssertion,
) (CurrentActiveAssertion, error) {
	switch carrier := assertion.carrier.(type) {
	case CurrentLegacyRelation:
		return NewCurrentActiveAssertion(CurrentActiveAssertionInput{
			Relation:       carrier.Relation(),
			CanonicalBytes: assertion.canonicalBytes,
			Digest:         assertion.digest,
			OriginEvent:    assertion.originEvent,
			OriginRevision: assertion.originRevision,
			ChangeOrdinal:  assertion.changeOrdinal,
		})
	case CurrentRelationalAssertion:
		return NewCurrentActiveRelationalAssertion(
			CurrentActiveRelationalAssertionInput{
				Assertion:      carrier.Assertion(),
				CanonicalBytes: assertion.canonicalBytes,
				Digest:         assertion.digest,
				OriginEvent:    assertion.originEvent,
				OriginRevision: assertion.originRevision,
				ChangeOrdinal:  assertion.changeOrdinal,
			},
		)
	default:
		return CurrentActiveAssertion{}, fmt.Errorf(
			"current active assertion carrier is not a closed variant",
		)
	}
}

// CurrentActiveAssertionSet is an immutable, canonically ordered active view
// at one exact project graph revision.
type CurrentActiveAssertionSet struct {
	project   projectidentity.ProjectID
	revision  typedmemory.GraphRevision
	relations []CurrentActiveAssertion
}

func NewCurrentActiveAssertionSet(
	project projectidentity.ProjectID,
	revision typedmemory.GraphRevision,
	relations []CurrentActiveAssertion,
) (CurrentActiveAssertionSet, error) {
	canonicalProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return CurrentActiveAssertionSet{}, fmt.Errorf(
			"current active assertion set requires an exact project",
		)
	}
	owned := append([]CurrentActiveAssertion(nil), relations...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].AssertionID().String() <
			owned[right].AssertionID().String()
	})
	for index, assertion := range owned {
		if err := assertion.Verify(); err != nil {
			return CurrentActiveAssertionSet{}, fmt.Errorf(
				"current active assertion %d: %w",
				index,
				err,
			)
		}
		if assertion.OriginRevision().Value() > revision.Value() {
			return CurrentActiveAssertionSet{}, fmt.Errorf(
				"current active assertion origin exceeds observed graph revision",
			)
		}
		if index > 0 &&
			owned[index-1].AssertionID() == assertion.AssertionID() {
			return CurrentActiveAssertionSet{}, fmt.Errorf(
				"current active assertion set repeats an assertion ID",
			)
		}
	}
	if revision.Value() == 0 && len(owned) != 0 {
		return CurrentActiveAssertionSet{}, fmt.Errorf(
			"revision-zero graph cannot contain active assertions",
		)
	}
	return CurrentActiveAssertionSet{
		project:   canonicalProject,
		revision:  revision,
		relations: owned,
	}, nil
}

func (set CurrentActiveAssertionSet) Project() projectidentity.ProjectID {
	return set.project
}

func (set CurrentActiveAssertionSet) GraphRevision() typedmemory.GraphRevision {
	return set.revision
}

func (set CurrentActiveAssertionSet) Relations() []CurrentActiveAssertion {
	return append([]CurrentActiveAssertion(nil), set.relations...)
}

func (set CurrentActiveAssertionSet) Verify() error {
	canonical, err := NewCurrentActiveAssertionSet(
		set.project,
		set.revision,
		set.relations,
	)
	if err != nil {
		return err
	}
	if len(canonical.relations) != len(set.relations) {
		return fmt.Errorf("current active assertion set cardinality changed")
	}
	for index, assertion := range canonical.relations {
		if assertion.AssertionID() != set.relations[index].AssertionID() {
			return fmt.Errorf("current active assertion set order is not canonical")
		}
	}
	return nil
}

// CurrentProjectGraphObservation binds the exact graph closure, the active
// TypeEnv retained for diagnostics, and the active relation set. The active
// TypeEnv must not substitute for a target TypeEnv during revalidation.
type CurrentProjectGraphObservation struct {
	basis          projecttypeenvselection.ProjectGraphSnapshotBasis
	activeTypeEnv  typedmemory.TypeEnvRef
	activeRelation CurrentActiveAssertionSet
}

func NewCurrentProjectGraphObservation(
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
	activeTypeEnv typedmemory.TypeEnvRef,
	active CurrentActiveAssertionSet,
) (CurrentProjectGraphObservation, error) {
	if err := basis.Verify(); err != nil {
		return CurrentProjectGraphObservation{}, fmt.Errorf(
			"current project graph basis: %w",
			err,
		)
	}
	canonicalTypeEnv, err := typedmemory.ParseTypeEnvRef(activeTypeEnv.String())
	if err != nil || canonicalTypeEnv != activeTypeEnv {
		return CurrentProjectGraphObservation{}, fmt.Errorf(
			"current project graph active TypeEnv is required",
		)
	}
	if err := active.Verify(); err != nil {
		return CurrentProjectGraphObservation{}, fmt.Errorf(
			"current project graph active assertions: %w",
			err,
		)
	}
	if basis.Project() != active.Project() ||
		basis.GraphRevision() != active.GraphRevision() {
		return CurrentProjectGraphObservation{}, fmt.Errorf(
			"current project graph basis and active assertions differ",
		)
	}
	return CurrentProjectGraphObservation{
		basis:          basis,
		activeTypeEnv:  canonicalTypeEnv,
		activeRelation: active,
	}, nil
}

func (observation CurrentProjectGraphObservation) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return observation.basis
}

func (observation CurrentProjectGraphObservation) ActiveTypeEnv() typedmemory.TypeEnvRef {
	return observation.activeTypeEnv
}

func (observation CurrentProjectGraphObservation) ActiveAssertions() CurrentActiveAssertionSet {
	return observation.activeRelation
}

func (observation CurrentProjectGraphObservation) Verify() error {
	canonical, err := NewCurrentProjectGraphObservation(
		observation.basis,
		observation.activeTypeEnv,
		observation.activeRelation,
	)
	if err != nil {
		return err
	}
	if canonical.basis.Ref() != observation.basis.Ref() {
		return fmt.Errorf("current project graph observation basis changed")
	}
	return nil
}
