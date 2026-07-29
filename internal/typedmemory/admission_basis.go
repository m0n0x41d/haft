package typedmemory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

const (
	admissionSnapshotBasisDomain               = "admission-snapshot-basis.v1"
	snapshotOnlyAdmissionBasisDomain           = "admission-basis.snapshot-only.v1"
	contextSliceMembershipAdmissionBasisDomain = "admission-basis.context-slice-membership.v2"
	entityAbsentObservationDomain              = "admission-observation.entity-absent.v1"
	entityExactObservationDomain               = "admission-observation.entity-exact.v1"
	aliasUnboundObservationDomain              = "admission-observation.alias-unbound.v1"
	aliasBoundObservationDomain                = "admission-observation.alias-bound.v1"
	assertionAbsentObservationDomain           = "admission-observation.assertion-absent.v1"
	assertionActiveObservationDomain           = "admission-observation.assertion-active.v1"
	sameBatchDeclarationResolutionDomain       = "admission-reference-resolution.same-batch-declaration.v2"
	snapshotReferenceResolutionDomain          = "admission-reference-resolution.snapshot.v1"
	relationFillerCoordinateDomain             = "admission-relation-filler-coordinate.v1"
	disjointNotMemberUseDomain                 = "admission-disjoint-not-member-use.v1"
	disjointEntailmentUseDomain                = "admission-disjoint-entailment-use.v1"
	referenceFillerAdmissionUseDomain          = "admission-reference-filler-use.v2"
	classificationReferenceFillerUseDomain     = "admission-classification-reference-filler-use.v1"
	contextSliceClassificationBasisDomain      = "admission-basis.context-slice-classification.v1"
)

// AdmissionSnapshotObservationKind identifies one positive, exact snapshot
// fact consumed by admission. Unknown, candidate, conflicting, or unresolved
// snapshot outcomes have no variant in this algebra.
type AdmissionSnapshotObservationKind uint8

const (
	EntityAbsentAdmissionObservation AdmissionSnapshotObservationKind = iota + 1
	EntityExactAdmissionObservation
	AliasUnboundAdmissionObservation
	AliasBoundAdmissionObservation
	AssertionAbsentAdmissionObservation
	AssertionActiveAdmissionObservation
)

func (kind AdmissionSnapshotObservationKind) String() string {
	switch kind {
	case EntityAbsentAdmissionObservation:
		return "entity_absent"
	case EntityExactAdmissionObservation:
		return "entity_exact"
	case AliasUnboundAdmissionObservation:
		return "alias_unbound"
	case AliasBoundAdmissionObservation:
		return "alias_bound"
	case AssertionAbsentAdmissionObservation:
		return "assertion_absent"
	case AssertionActiveAdmissionObservation:
		return "assertion_active"
	default:
		return ""
	}
}

// AdmissionSnapshotObservation is a closed positive-observation algebra. Its
// change ordinal points into the candidate MemoryChangeSet committed by the
// admission envelope; it is not a project, causal, or WorkPlan order.
type AdmissionSnapshotObservation interface {
	Kind() AdmissionSnapshotObservationKind
	ChangeOrdinal() uint64
	CanonicalBytes() []byte
	Digest() SHA256Digest
	admissionSnapshotObservationVariant()
}

type EntityAbsentObservation interface {
	AdmissionSnapshotObservation
	Resolution() AbsentEntityResolution
	entityAbsentObservationVariant()
}

type EntityExactObservation interface {
	AdmissionSnapshotObservation
	Resolution() ExactEntityResolution
	entityExactObservationVariant()
}

type AliasUnboundObservation interface {
	AdmissionSnapshotObservation
	Resolution() UnboundAliasResolution
	aliasUnboundObservationVariant()
}

type AliasBoundObservation interface {
	AdmissionSnapshotObservation
	Resolution() BoundAliasResolution
	aliasBoundObservationVariant()
}

type AssertionAbsentObservation interface {
	AdmissionSnapshotObservation
	State() AbsentAssertionState
	assertionAbsentObservationVariant()
}

type AssertionActiveObservation interface {
	AdmissionSnapshotObservation
	State() ActiveAssertion
	assertionActiveObservationVariant()
}

type entityAbsentObservation struct {
	changeOrdinal uint64
	resolution    AbsentEntityResolution
	canonical     []byte
	digest        SHA256Digest
}

func NewEntityAbsentObservation(
	changeOrdinal uint64,
	resolution AbsentEntityResolution,
) (EntityAbsentObservation, error) {
	if !validAbsentEntityResolution(resolution) {
		return nil, fmt.Errorf("entity-absent admission observation requires an exact positive resolution")
	}
	writer := canonicalEntityAbsentObservation(changeOrdinal, resolution)
	return entityAbsentObservation{
		changeOrdinal: changeOrdinal,
		resolution:    resolution,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (entityAbsentObservation) Kind() AdmissionSnapshotObservationKind {
	return EntityAbsentAdmissionObservation
}

func (observation entityAbsentObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation entityAbsentObservation) Resolution() AbsentEntityResolution {
	return observation.resolution
}

func (observation entityAbsentObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation entityAbsentObservation) Digest() SHA256Digest { return observation.digest }

func (entityAbsentObservation) admissionSnapshotObservationVariant() {}

func (entityAbsentObservation) entityAbsentObservationVariant() {}

type entityExactObservation struct {
	changeOrdinal uint64
	resolution    ExactEntityResolution
	canonical     []byte
	digest        SHA256Digest
}

func NewEntityExactObservation(
	changeOrdinal uint64,
	resolution ExactEntityResolution,
) (EntityExactObservation, error) {
	if !validExactEntityResolution(resolution) {
		return nil, fmt.Errorf("entity-exact admission observation requires an exact positive resolution")
	}
	writer := canonicalEntityExactObservation(changeOrdinal, resolution)
	return entityExactObservation{
		changeOrdinal: changeOrdinal,
		resolution:    resolution,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (entityExactObservation) Kind() AdmissionSnapshotObservationKind {
	return EntityExactAdmissionObservation
}

func (observation entityExactObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation entityExactObservation) Resolution() ExactEntityResolution {
	return observation.resolution
}

func (observation entityExactObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation entityExactObservation) Digest() SHA256Digest { return observation.digest }

func (entityExactObservation) admissionSnapshotObservationVariant() {}

func (entityExactObservation) entityExactObservationVariant() {}

type aliasUnboundObservation struct {
	changeOrdinal uint64
	resolution    UnboundAliasResolution
	canonical     []byte
	digest        SHA256Digest
}

func NewAliasUnboundObservation(
	changeOrdinal uint64,
	resolution UnboundAliasResolution,
) (AliasUnboundObservation, error) {
	if !validUnboundAliasResolution(resolution) {
		return nil, fmt.Errorf("alias-unbound admission observation requires an exact positive resolution")
	}
	writer := canonicalAliasUnboundObservation(changeOrdinal, resolution)
	return aliasUnboundObservation{
		changeOrdinal: changeOrdinal,
		resolution:    resolution,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (aliasUnboundObservation) Kind() AdmissionSnapshotObservationKind {
	return AliasUnboundAdmissionObservation
}

func (observation aliasUnboundObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation aliasUnboundObservation) Resolution() UnboundAliasResolution {
	return observation.resolution
}

func (observation aliasUnboundObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation aliasUnboundObservation) Digest() SHA256Digest { return observation.digest }

func (aliasUnboundObservation) admissionSnapshotObservationVariant() {}

func (aliasUnboundObservation) aliasUnboundObservationVariant() {}

type aliasBoundObservation struct {
	changeOrdinal uint64
	resolution    BoundAliasResolution
	canonical     []byte
	digest        SHA256Digest
}

func NewAliasBoundObservation(
	changeOrdinal uint64,
	resolution BoundAliasResolution,
) (AliasBoundObservation, error) {
	if !validBoundAliasResolution(resolution) {
		return nil, fmt.Errorf("alias-bound admission observation requires an exact positive resolution")
	}
	writer := canonicalAliasBoundObservation(changeOrdinal, resolution)
	return aliasBoundObservation{
		changeOrdinal: changeOrdinal,
		resolution:    resolution,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (aliasBoundObservation) Kind() AdmissionSnapshotObservationKind {
	return AliasBoundAdmissionObservation
}

func (observation aliasBoundObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation aliasBoundObservation) Resolution() BoundAliasResolution {
	return observation.resolution
}

func (observation aliasBoundObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation aliasBoundObservation) Digest() SHA256Digest { return observation.digest }

func (aliasBoundObservation) admissionSnapshotObservationVariant() {}

func (aliasBoundObservation) aliasBoundObservationVariant() {}

type assertionAbsentObservation struct {
	changeOrdinal uint64
	state         AbsentAssertionState
	canonical     []byte
	digest        SHA256Digest
}

func NewAssertionAbsentObservation(
	changeOrdinal uint64,
	state AbsentAssertionState,
) (AssertionAbsentObservation, error) {
	if !validAbsentAssertionState(state) {
		return nil, fmt.Errorf("assertion-absent admission observation requires an exact positive state")
	}
	writer := canonicalAssertionAbsentObservation(changeOrdinal, state)
	return assertionAbsentObservation{
		changeOrdinal: changeOrdinal,
		state:         state,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (assertionAbsentObservation) Kind() AdmissionSnapshotObservationKind {
	return AssertionAbsentAdmissionObservation
}

func (observation assertionAbsentObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation assertionAbsentObservation) State() AbsentAssertionState {
	return observation.state
}

func (observation assertionAbsentObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation assertionAbsentObservation) Digest() SHA256Digest { return observation.digest }

func (assertionAbsentObservation) admissionSnapshotObservationVariant() {}

func (assertionAbsentObservation) assertionAbsentObservationVariant() {}

type assertionActiveObservation struct {
	changeOrdinal uint64
	state         ActiveAssertion
	canonical     []byte
	digest        SHA256Digest
}

func NewAssertionActiveObservation(
	changeOrdinal uint64,
	state ActiveAssertion,
) (AssertionActiveObservation, error) {
	if !validActiveAssertionState(state) {
		return nil, fmt.Errorf("assertion-active admission observation requires an exact positive state")
	}
	writer := canonicalAssertionActiveObservation(changeOrdinal, state)
	return assertionActiveObservation{
		changeOrdinal: changeOrdinal,
		state:         state,
		canonical:     writer.bytes(),
		digest:        writer.digest(),
	}, nil
}

func (assertionActiveObservation) Kind() AdmissionSnapshotObservationKind {
	return AssertionActiveAdmissionObservation
}

func (observation assertionActiveObservation) ChangeOrdinal() uint64 {
	return observation.changeOrdinal
}

func (observation assertionActiveObservation) State() ActiveAssertion {
	return observation.state
}

func (observation assertionActiveObservation) CanonicalBytes() []byte {
	return append([]byte(nil), observation.canonical...)
}

func (observation assertionActiveObservation) Digest() SHA256Digest { return observation.digest }

func (assertionActiveObservation) admissionSnapshotObservationVariant() {}

func (assertionActiveObservation) assertionActiveObservationVariant() {}

func canonicalEntityAbsentObservation(
	changeOrdinal uint64,
	resolution AbsentEntityResolution,
) canonicalWriter {
	writer := newCanonicalWriter(entityAbsentObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(resolution.Entity().String())
	writer.addString(resolution.Context().String())
	writer.addString(resolution.Basis().String())
	return writer
}

func canonicalEntityExactObservation(
	changeOrdinal uint64,
	resolution ExactEntityResolution,
) canonicalWriter {
	writer := newCanonicalWriter(entityExactObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(resolution.Entity().String())
	writer.addString(resolution.Context().String())
	writer.addString(resolution.Basis().String())
	return writer
}

func canonicalAliasUnboundObservation(
	changeOrdinal uint64,
	resolution UnboundAliasResolution,
) canonicalWriter {
	writer := newCanonicalWriter(aliasUnboundObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(resolution.Alias().String())
	writer.addString(resolution.Context().String())
	writer.addString(resolution.Basis().String())
	return writer
}

func canonicalAliasBoundObservation(
	changeOrdinal uint64,
	resolution BoundAliasResolution,
) canonicalWriter {
	writer := newCanonicalWriter(aliasBoundObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(resolution.Alias().String())
	writer.addString(resolution.Entity().String())
	writer.addString(resolution.Context().String())
	writer.addString(resolution.Basis().String())
	return writer
}

func canonicalAssertionAbsentObservation(
	changeOrdinal uint64,
	state AbsentAssertionState,
) canonicalWriter {
	writer := newCanonicalWriter(assertionAbsentObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(state.Assertion().String())
	writer.addString(state.Basis().String())
	return writer
}

func canonicalAssertionActiveObservation(
	changeOrdinal uint64,
	state ActiveAssertion,
) canonicalWriter {
	writer := newCanonicalWriter(assertionActiveObservationDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(state.Assertion().String())
	writer.addString(state.Basis().String())
	return writer
}

func validAbsentEntityResolution(resolution AbsentEntityResolution) bool {
	return resolution.Entity().valid() &&
		resolution.Context().valid() &&
		resolution.Basis().valid()
}

func validExactEntityResolution(resolution ExactEntityResolution) bool {
	return resolution.Entity().valid() &&
		resolution.Context().valid() &&
		resolution.Basis().valid()
}

func validUnboundAliasResolution(resolution UnboundAliasResolution) bool {
	return resolution.Alias().valid() &&
		resolution.Context().valid() &&
		resolution.Basis().valid()
}

func validBoundAliasResolution(resolution BoundAliasResolution) bool {
	return resolution.Alias().valid() &&
		resolution.Entity().valid() &&
		resolution.Context().valid() &&
		resolution.Basis().valid()
}

func validAbsentAssertionState(state AbsentAssertionState) bool {
	return state.Assertion().valid() && state.Basis().valid()
}

func validActiveAssertionState(state ActiveAssertion) bool {
	return state.Assertion().valid() && state.Basis().valid()
}

func validAdmissionSnapshotObservation(observation AdmissionSnapshotObservation) bool {
	switch value := observation.(type) {
	case entityAbsentObservation:
		writer := canonicalEntityAbsentObservation(value.changeOrdinal, value.resolution)
		return validAbsentEntityResolution(value.resolution) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	case entityExactObservation:
		writer := canonicalEntityExactObservation(value.changeOrdinal, value.resolution)
		return validExactEntityResolution(value.resolution) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	case aliasUnboundObservation:
		writer := canonicalAliasUnboundObservation(value.changeOrdinal, value.resolution)
		return validUnboundAliasResolution(value.resolution) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	case aliasBoundObservation:
		writer := canonicalAliasBoundObservation(value.changeOrdinal, value.resolution)
		return validBoundAliasResolution(value.resolution) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	case assertionAbsentObservation:
		writer := canonicalAssertionAbsentObservation(value.changeOrdinal, value.state)
		return validAbsentAssertionState(value.state) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	case assertionActiveObservation:
		writer := canonicalAssertionActiveObservation(value.changeOrdinal, value.state)
		return validActiveAssertionState(value.state) &&
			canonicalValueMatches(writer, value.canonical, value.digest)
	default:
		return false
	}
}

func normalizeAdmissionSnapshotObservations(
	values []AdmissionSnapshotObservation,
) ([]AdmissionSnapshotObservation, error) {
	result := append([]AdmissionSnapshotObservation(nil), values...)
	for _, observation := range result {
		if !validAdmissionSnapshotObservation(observation) {
			return nil, fmt.Errorf("admission snapshot basis contains an invalid or non-positive observation")
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftPosition := admissionObservationPositionKey(result[left])
		rightPosition := admissionObservationPositionKey(result[right])
		if leftPosition != rightPosition {
			return leftPosition < rightPosition
		}
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]AdmissionSnapshotObservation, 0, len(result))
	for _, observation := range result {
		if len(normalized) == 0 ||
			admissionObservationPositionKey(normalized[len(normalized)-1]) != admissionObservationPositionKey(observation) {
			normalized = append(normalized, observation)
			continue
		}
		previous := normalized[len(normalized)-1]
		if bytes.Equal(previous.CanonicalBytes(), observation.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"admission snapshot basis has conflicting positive observations for %s",
			admissionObservationPositionKey(observation),
		)
	}
	return normalized, nil
}

func admissionObservationPositionKey(observation AdmissionSnapshotObservation) string {
	changeOrdinal := fmt.Sprintf("%d", observation.ChangeOrdinal())
	switch value := observation.(type) {
	case entityAbsentObservation:
		return exactTupleKey(
			"admission-observation-position.entity",
			changeOrdinal,
			value.resolution.Entity().String(),
			value.resolution.Context().String(),
		)
	case entityExactObservation:
		return exactTupleKey(
			"admission-observation-position.entity",
			changeOrdinal,
			value.resolution.Entity().String(),
			value.resolution.Context().String(),
		)
	case aliasUnboundObservation:
		return exactTupleKey(
			"admission-observation-position.alias",
			changeOrdinal,
			value.resolution.Alias().String(),
			value.resolution.Context().String(),
		)
	case aliasBoundObservation:
		return exactTupleKey(
			"admission-observation-position.alias",
			changeOrdinal,
			value.resolution.Alias().String(),
			value.resolution.Context().String(),
		)
	case assertionAbsentObservation:
		return exactTupleKey(
			"admission-observation-position.assertion",
			changeOrdinal,
			value.state.Assertion().String(),
		)
	case assertionActiveObservation:
		return exactTupleKey(
			"admission-observation-position.assertion",
			changeOrdinal,
			value.state.Assertion().String(),
		)
	default:
		return ""
	}
}

// AdmissionReferenceResolutionKind separates a same-batch declaration proof
// from an exact immutable-snapshot resolution. Unresolved references have no
// admission-resolution variant.
type AdmissionReferenceResolutionKind uint8

const (
	SameBatchDeclarationAdmissionResolution AdmissionReferenceResolutionKind = iota + 1
	SnapshotAdmissionResolution
)

func (kind AdmissionReferenceResolutionKind) String() string {
	switch kind {
	case SameBatchDeclarationAdmissionResolution:
		return "same_batch_declaration"
	case SnapshotAdmissionResolution:
		return "snapshot"
	default:
		return ""
	}
}

type AdmissionReferenceResolution interface {
	Kind() AdmissionReferenceResolutionKind
	PersistedReference() PersistedRef
	Entity() EntityID
	Context() BoundedContextRef
	CanonicalBytes() []byte
	Digest() SHA256Digest
	admissionReferenceResolutionVariant()
}

type SameBatchDeclarationResolution interface {
	AdmissionReferenceResolution
	LocalReference() LocalRef
	DeclarationChangeOrdinal() uint64
	Declaration() DeclareEntity
	DeclarationCanonicalBytes() []byte
	DeclarationDigest() SHA256Digest
	sameBatchDeclarationResolutionVariant()
}

type SnapshotReferenceResolution interface {
	AdmissionReferenceResolution
	ResolutionBasis() ResolutionBasisRef
	snapshotReferenceResolutionVariant()
}

type SameBatchDeclarationResolutionInput struct {
	LocalReference           LocalRef
	DeclarationChangeOrdinal uint64
	Declaration              DeclareEntity
	PersistedReference       PersistedRef
}

type sameBatchDeclarationResolution struct {
	localReference           LocalRef
	declarationChangeOrdinal uint64
	declaration              DeclareEntity
	declarationBytes         []byte
	declarationDigest        SHA256Digest
	persistedReference       PersistedRef
	entity                   EntityID
	context                  BoundedContextRef
	canonical                []byte
	digest                   SHA256Digest
}

func NewSameBatchDeclarationResolution(
	input SameBatchDeclarationResolutionInput,
) (SameBatchDeclarationResolution, error) {
	if !validStrongRef(input.LocalReference) ||
		!input.Declaration.validMemoryChange() ||
		!validStrongRef(input.PersistedReference) {
		return nil, fmt.Errorf("same-batch reference resolution requires exact local, declaration, and persisted identities")
	}
	if input.LocalReference.BatchLocalRef() != input.Declaration.LocalRef() {
		return nil, fmt.Errorf("same-batch local reference does not name the supplied declaration")
	}
	if input.LocalReference.RefKind() != input.PersistedReference.RefKind() {
		return nil, fmt.Errorf("same-batch reference lowering changed RefKind")
	}
	if input.PersistedReference.ReferenceID().String() != input.Declaration.Entity().String() {
		return nil, fmt.Errorf("same-batch persisted reference is not the stable declared EntityID")
	}
	declarationBytes, err := input.Declaration.CanonicalBytes()
	if err != nil {
		return nil, fmt.Errorf("canonicalize same-batch declaration: %w", err)
	}
	declarationDigest, err := input.Declaration.Digest()
	if err != nil {
		return nil, fmt.Errorf("digest same-batch declaration: %w", err)
	}
	writer := canonicalSameBatchDeclarationResolution(
		input.LocalReference,
		input.DeclarationChangeOrdinal,
		declarationBytes,
		declarationDigest,
		input.Declaration.Provenance(),
		input.PersistedReference,
		input.Declaration.Entity(),
		input.Declaration.Context(),
	)
	return sameBatchDeclarationResolution{
		localReference:           input.LocalReference,
		declarationChangeOrdinal: input.DeclarationChangeOrdinal,
		declaration:              input.Declaration,
		declarationBytes:         append([]byte(nil), declarationBytes...),
		declarationDigest:        declarationDigest,
		persistedReference:       input.PersistedReference,
		entity:                   input.Declaration.Entity(),
		context:                  input.Declaration.Context(),
		canonical:                writer.bytes(),
		digest:                   writer.digest(),
	}, nil
}

func (sameBatchDeclarationResolution) Kind() AdmissionReferenceResolutionKind {
	return SameBatchDeclarationAdmissionResolution
}

func (resolution sameBatchDeclarationResolution) LocalReference() LocalRef {
	return resolution.localReference
}

func (resolution sameBatchDeclarationResolution) DeclarationChangeOrdinal() uint64 {
	return resolution.declarationChangeOrdinal
}

func (resolution sameBatchDeclarationResolution) Declaration() DeclareEntity {
	return resolution.declaration
}

func (resolution sameBatchDeclarationResolution) DeclarationCanonicalBytes() []byte {
	return append([]byte(nil), resolution.declarationBytes...)
}

func (resolution sameBatchDeclarationResolution) DeclarationDigest() SHA256Digest {
	return resolution.declarationDigest
}

func (resolution sameBatchDeclarationResolution) PersistedReference() PersistedRef {
	return resolution.persistedReference
}

func (resolution sameBatchDeclarationResolution) Entity() EntityID { return resolution.entity }

func (resolution sameBatchDeclarationResolution) Context() BoundedContextRef {
	return resolution.context
}

func (resolution sameBatchDeclarationResolution) CanonicalBytes() []byte {
	return append([]byte(nil), resolution.canonical...)
}

func (resolution sameBatchDeclarationResolution) Digest() SHA256Digest {
	return resolution.digest
}

func (sameBatchDeclarationResolution) admissionReferenceResolutionVariant() {}

func (sameBatchDeclarationResolution) sameBatchDeclarationResolutionVariant() {}

type snapshotReferenceResolution struct {
	reference PersistedRef
	entity    EntityID
	context   BoundedContextRef
	basis     ResolutionBasisRef
	canonical []byte
	digest    SHA256Digest
}

func NewSnapshotReferenceResolution(
	resolution ResolvedStrongReference,
) (SnapshotReferenceResolution, error) {
	reference, persisted := resolution.Reference().(PersistedRef)
	if !persisted ||
		!validStrongRef(reference) ||
		!resolution.Entity().valid() ||
		!resolution.Context().valid() ||
		!resolution.Basis().valid() {
		return nil, fmt.Errorf("snapshot admission resolution requires one exact persisted-reference result")
	}
	writer := canonicalSnapshotReferenceResolution(
		reference,
		resolution.Entity(),
		resolution.Context(),
		resolution.Basis(),
	)
	return snapshotReferenceResolution{
		reference: reference,
		entity:    resolution.Entity(),
		context:   resolution.Context(),
		basis:     resolution.Basis(),
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (snapshotReferenceResolution) Kind() AdmissionReferenceResolutionKind {
	return SnapshotAdmissionResolution
}

func (resolution snapshotReferenceResolution) PersistedReference() PersistedRef {
	return resolution.reference
}

func (resolution snapshotReferenceResolution) Entity() EntityID { return resolution.entity }

func (resolution snapshotReferenceResolution) Context() BoundedContextRef {
	return resolution.context
}

func (resolution snapshotReferenceResolution) ResolutionBasis() ResolutionBasisRef {
	return resolution.basis
}

func (resolution snapshotReferenceResolution) CanonicalBytes() []byte {
	return append([]byte(nil), resolution.canonical...)
}

func (resolution snapshotReferenceResolution) Digest() SHA256Digest { return resolution.digest }

func (snapshotReferenceResolution) admissionReferenceResolutionVariant() {}

func (snapshotReferenceResolution) snapshotReferenceResolutionVariant() {}

func canonicalSameBatchDeclarationResolution(
	localReference LocalRef,
	declarationChangeOrdinal uint64,
	declarationBytes []byte,
	declarationDigest SHA256Digest,
	declarationProvenance ProvenanceRef,
	persistedReference PersistedRef,
	entity EntityID,
	context BoundedContextRef,
) canonicalWriter {
	writer := newCanonicalWriter(sameBatchDeclarationResolutionDomain)
	writer.addString(localReference.RefKind().String())
	writer.addString(localReference.BatchLocalRef().String())
	writer.addUint64(declarationChangeOrdinal)
	writer.addBytes(declarationBytes)
	writer.addString(declarationDigest.String())
	writer.addString(declarationProvenance.String())
	writer.addString(persistedReference.RefKind().String())
	writer.addString(persistedReference.ReferenceID().String())
	writer.addString(entity.String())
	writer.addString(context.String())
	return writer
}

func canonicalSnapshotReferenceResolution(
	reference PersistedRef,
	entity EntityID,
	context BoundedContextRef,
	basis ResolutionBasisRef,
) canonicalWriter {
	writer := newCanonicalWriter(snapshotReferenceResolutionDomain)
	writer.addString(reference.RefKind().String())
	writer.addString(reference.ReferenceID().String())
	writer.addString(entity.String())
	writer.addString(context.String())
	writer.addString(basis.String())
	return writer
}

func validAdmissionReferenceResolution(resolution AdmissionReferenceResolution) bool {
	switch value := resolution.(type) {
	case sameBatchDeclarationResolution:
		return validSameBatchDeclarationResolution(value)
	case snapshotReferenceResolution:
		return validSnapshotReferenceResolution(value)
	default:
		return false
	}
}

func validSameBatchDeclarationResolution(
	resolution sameBatchDeclarationResolution,
) bool {
	declarationBytes, err := resolution.declaration.CanonicalBytes()
	if err != nil {
		return false
	}
	declarationDigest, err := resolution.declaration.Digest()
	if err != nil {
		return false
	}
	if !validStrongRef(resolution.localReference) ||
		!validStrongRef(resolution.persistedReference) ||
		!resolution.declaration.validMemoryChange() ||
		!resolution.declarationDigest.valid() ||
		!resolution.entity.valid() ||
		!resolution.context.valid() ||
		resolution.localReference.BatchLocalRef() != resolution.declaration.LocalRef() ||
		resolution.localReference.RefKind() != resolution.persistedReference.RefKind() ||
		resolution.persistedReference.ReferenceID().String() != resolution.entity.String() ||
		resolution.declaration.Entity() != resolution.entity ||
		resolution.declaration.Context() != resolution.context ||
		declarationDigest != resolution.declarationDigest ||
		!bytes.Equal(declarationBytes, resolution.declarationBytes) {
		return false
	}
	writer := canonicalSameBatchDeclarationResolution(
		resolution.localReference,
		resolution.declarationChangeOrdinal,
		resolution.declarationBytes,
		resolution.declarationDigest,
		resolution.declaration.Provenance(),
		resolution.persistedReference,
		resolution.entity,
		resolution.context,
	)
	return canonicalValueMatches(writer, resolution.canonical, resolution.digest)
}

func validSnapshotReferenceResolution(resolution snapshotReferenceResolution) bool {
	if !validStrongRef(resolution.reference) ||
		!resolution.entity.valid() ||
		!resolution.context.valid() ||
		!resolution.basis.valid() {
		return false
	}
	writer := canonicalSnapshotReferenceResolution(
		resolution.reference,
		resolution.entity,
		resolution.context,
		resolution.basis,
	)
	return canonicalValueMatches(writer, resolution.canonical, resolution.digest)
}

type RelationFillerCoordinate interface {
	ChangeOrdinal() uint64
	Assertion() AssertionID
	Signature() RelationSignatureRef
	ContextSlice() ContextSlice
	Slot() SlotKindID
	FillerOrdinal() uint64
	FillerDigest() SHA256Digest
	Reference() PersistedRef
	Entity() EntityID
	RequiredValueKind() ValueKindRef
	CanonicalBytes() []byte
	Digest() SHA256Digest
	relationFillerCoordinateVariant()
}

type RelationFillerCoordinateInput struct {
	TypeEnv       TypeEnv
	ChangeOrdinal uint64
	Relation      RelationInstance
	Slot          SlotKindID
	FillerOrdinal uint64
}

type RelationalAssertionFillerCoordinateInput struct {
	TypeEnv       TypeEnv
	ChangeOrdinal uint64
	Assertion     RelationalAssertion
	Slot          SlotKindID
	FillerOrdinal uint64
}

type relationFillerCoordinate struct {
	changeOrdinal     uint64
	assertion         AssertionID
	signature         RelationSignatureRef
	contextSlice      ContextSlice
	slot              SlotKindID
	fillerOrdinal     uint64
	reference         PersistedRef
	entity            EntityID
	requiredValueKind ValueKindRef
	fillerDigest      SHA256Digest
	canonical         []byte
	digest            SHA256Digest
}

func NewRelationFillerCoordinate(
	input RelationFillerCoordinateInput,
) (RelationFillerCoordinate, error) {
	if err := validateTypeEnv(input.TypeEnv); err != nil {
		return nil, fmt.Errorf("relation-filler coordinate requires a valid exact TypeEnv: %w", err)
	}
	if !input.Relation.valid() || !input.Slot.valid() {
		return nil, fmt.Errorf("relation-filler coordinate requires a final relation and exact slot")
	}
	return newRelationFillerCoordinate(
		input.TypeEnv,
		input.ChangeOrdinal,
		input.Relation,
		input.Slot,
		input.FillerOrdinal,
	)
}

// NewRelationalAssertionFillerCoordinate seals reference-filler admission
// evidence for a v3 assertion. Its coordinate deliberately omits modality:
// reference resolution proves only the typed participant designation, never
// the direct obtaining predicate.
func NewRelationalAssertionFillerCoordinate(
	input RelationalAssertionFillerCoordinateInput,
) (RelationFillerCoordinate, error) {
	if err := validateTypeEnv(input.TypeEnv); err != nil {
		return nil, fmt.Errorf("relational-assertion filler coordinate requires a valid exact TypeEnv: %w", err)
	}
	if !input.Assertion.valid() || !input.Slot.valid() {
		return nil, fmt.Errorf(
			"relational-assertion filler coordinate requires a final assertion and exact slot",
		)
	}
	return newRelationFillerCoordinate(
		input.TypeEnv,
		input.ChangeOrdinal,
		input.Assertion,
		input.Slot,
		input.FillerOrdinal,
	)
}

func newRelationFillerCoordinate(
	environment TypeEnv,
	changeOrdinal uint64,
	relation validatedRelationalCarrier,
	slotID SlotKindID,
	fillerOrdinal uint64,
) (RelationFillerCoordinate, error) {
	fragmentRef := relation.RelationDeclarationFragmentRef()
	if fragmentRef.TypeEnv() != environment.Ref() {
		return nil, fmt.Errorf("final relation and coordinate TypeEnv do not match")
	}
	fragment, exists := environment.TypedRelationDeclarationFragment(fragmentRef)
	if !exists || !fragment.valid() || !relationFragmentAllowsContext(fragment, relation.Slice().Context()) {
		return nil, fmt.Errorf(
			"final typed relation declaration fragment is unavailable or does not declare its ContextSlice context",
		)
	}
	slot, exists := fragment.Slot(slotID)
	if !exists || !slot.valid() {
		return nil, fmt.Errorf(
			"final typed relation declaration fragment does not define the requested slot",
		)
	}
	target, byReference := slot.Target().(ReferenceSlotTarget)
	if !byReference {
		return nil, fmt.Errorf("relation-filler admission coordinate requires a by-reference slot target")
	}
	filler, err := finalReferenceFiller(relation.Bindings(), slotID, fillerOrdinal)
	if err != nil {
		return nil, err
	}
	if filler.Reference().RefKind() != target.ReferenceKind() {
		return nil, fmt.Errorf("final reference filler does not match the fragment slot's exact RefKind")
	}
	fillerDigest := digestAdmissionBytes(canonicalSlotFiller(filler))
	writer := canonicalRelationFillerCoordinate(
		changeOrdinal,
		relation.Assertion(),
		fragmentRef,
		relation.Slice(),
		slotID,
		fillerOrdinal,
		target.ValueKind(),
		fillerDigest,
	)
	return relationFillerCoordinate{
		changeOrdinal:     changeOrdinal,
		assertion:         relation.Assertion(),
		signature:         fragmentRef,
		contextSlice:      relation.Slice(),
		slot:              slotID,
		fillerOrdinal:     fillerOrdinal,
		reference:         filler.Reference(),
		entity:            filler.Entity(),
		requiredValueKind: target.ValueKind(),
		fillerDigest:      fillerDigest,
		canonical:         writer.bytes(),
		digest:            writer.digest(),
	}, nil
}

func (coordinate relationFillerCoordinate) ChangeOrdinal() uint64 {
	return coordinate.changeOrdinal
}

func (coordinate relationFillerCoordinate) Assertion() AssertionID {
	return coordinate.assertion
}

func (coordinate relationFillerCoordinate) Signature() RelationSignatureRef {
	return coordinate.signature
}

func (coordinate relationFillerCoordinate) ContextSlice() ContextSlice {
	return coordinate.contextSlice
}

func (coordinate relationFillerCoordinate) Slot() SlotKindID { return coordinate.slot }

func (coordinate relationFillerCoordinate) FillerOrdinal() uint64 {
	return coordinate.fillerOrdinal
}

func (coordinate relationFillerCoordinate) FillerDigest() SHA256Digest {
	return coordinate.fillerDigest
}

func (coordinate relationFillerCoordinate) Reference() PersistedRef {
	return coordinate.reference
}

func (coordinate relationFillerCoordinate) Entity() EntityID { return coordinate.entity }

func (coordinate relationFillerCoordinate) RequiredValueKind() ValueKindRef {
	return coordinate.requiredValueKind
}

func (coordinate relationFillerCoordinate) CanonicalBytes() []byte {
	return append([]byte(nil), coordinate.canonical...)
}

func (coordinate relationFillerCoordinate) Digest() SHA256Digest { return coordinate.digest }

func (relationFillerCoordinate) relationFillerCoordinateVariant() {}

func canonicalRelationFillerCoordinate(
	changeOrdinal uint64,
	assertion AssertionID,
	signature RelationSignatureRef,
	contextSlice ContextSlice,
	slot SlotKindID,
	fillerOrdinal uint64,
	requiredValueKind ValueKindRef,
	fillerDigest SHA256Digest,
) canonicalWriter {
	writer := newCanonicalWriter(relationFillerCoordinateDomain)
	writer.addUint64(changeOrdinal)
	writer.addString(assertion.String())
	writer.addString(signature.String())
	writer.addString(contextSlice.Ref().String())
	writer.addBytes(contextSlice.CanonicalBytes())
	writer.addString(slot.String())
	writer.addUint64(fillerOrdinal)
	writer.addString(requiredValueKind.String())
	writer.addString(fillerDigest.String())
	return writer
}

func validRelationFillerCoordinate(coordinate RelationFillerCoordinate) bool {
	value, supported := coordinate.(relationFillerCoordinate)
	if !supported ||
		!value.assertion.valid() ||
		!value.signature.valid() ||
		!value.contextSlice.valid() ||
		!value.slot.valid() ||
		!validStrongRef(value.reference) ||
		!value.entity.valid() ||
		!value.requiredValueKind.valid() ||
		value.signature.TypeEnv() != value.requiredValueKind.TypeEnv() ||
		value.reference.RefKind().TypeEnv() != value.requiredValueKind.TypeEnv() ||
		!value.fillerDigest.valid() {
		return false
	}
	filler := newReferenceFiller(value.reference, value.entity)
	expectedFillerDigest := digestAdmissionBytes(canonicalSlotFiller(filler))
	if expectedFillerDigest != value.fillerDigest {
		return false
	}
	writer := canonicalRelationFillerCoordinate(
		value.changeOrdinal,
		value.assertion,
		value.signature,
		value.contextSlice,
		value.slot,
		value.fillerOrdinal,
		value.requiredValueKind,
		value.fillerDigest,
	)
	return canonicalValueMatches(writer, value.canonical, value.digest)
}

func finalReferenceFiller(
	bindings []SlotBinding,
	slot SlotKindID,
	fillerOrdinal uint64,
) (ReferenceFiller, error) {
	for _, binding := range bindings {
		if binding.Name() != slot {
			continue
		}
		fillers := binding.Fillers()
		if fillerOrdinal >= uint64(len(fillers)) {
			return ReferenceFiller{}, fmt.Errorf("relation-filler ordinal is outside the final canonical slot binding")
		}
		filler, byReference := fillers[fillerOrdinal].(ReferenceFiller)
		if !byReference || !filler.validSlotFiller() {
			return ReferenceFiller{}, fmt.Errorf("relation-filler coordinate points to a non-reference filler")
		}
		return filler, nil
	}
	return ReferenceFiller{}, fmt.Errorf("relation-filler coordinate points to an absent final slot binding")
}

func relationFragmentAllowsContext(
	fragment TypedRelationDeclarationFragment,
	context BoundedContextRef,
) bool {
	for _, candidate := range fragment.Contexts() {
		if candidate == context {
			return true
		}
	}
	return false
}

// DisjointCounterUse is the sealed admission-use algebra for one counter
// position of an exact KindDisjoint constraint. A direct evaluator result and
// a deductive use of the constraint are distinct bases: the latter never
// fabricates a MemberOfNotMember judgement.
type DisjointCounterUse interface {
	Kind() DisjointCounterUseKind
	Constraint() ConstraintID
	CounterQuery() MemberOfQuery
	EvaluationView() MemberOfEvaluationView
	CounterRequest() MemberOfEvaluationRequest
	CanonicalBytes() []byte
	Digest() SHA256Digest
	disjointCounterUseVariant()
}

type DisjointCounterUseKind uint8

const (
	DirectNotMemberDisjointCounterUse DisjointCounterUseKind = iota + 1
	EntailedDisjointCounterUse
)

func (kind DisjointCounterUseKind) String() string {
	switch kind {
	case DirectNotMemberDisjointCounterUse:
		return "direct_not_member"
	case EntailedDisjointCounterUse:
		return "disjoint_entailment"
	default:
		return ""
	}
}

// DirectNotMemberUse carries the exact negative result of the evaluator for
// the counter kind. It is the legacy DisjointNotMemberUse under a precise sum
// variant name.
type DirectNotMemberUse interface {
	DisjointCounterUse
	Judgement() MemberOfNotMember
	directNotMemberUseVariant()
}

// DisjointNotMemberUse preserves the direct-only public surface while callers
// migrate atomically to DisjointCounterUse. It cannot contain an entailment.
type DisjointNotMemberUse = DirectNotMemberUse

// DisjointEntailmentUse records the exact positive membership and exact
// KindDisjoint declaration from which exclusion of one other operand follows.
// It deliberately has no Judgement method.
type DisjointEntailmentUse interface {
	DisjointCounterUse
	ConstraintRule() KindDisjointConstraint
	ConstraintDigest() SHA256Digest
	SupportingMembership() MemberOfMember
	MatchedOperand() KindID
	ExcludedOperand() KindID
	disjointEntailmentUseVariant()
}

type disjointNotMemberUse struct {
	constraint ConstraintID
	judgement  MemberOfNotMember
	canonical  []byte
	digest     SHA256Digest
}

func NewDisjointNotMemberUse(
	constraint ConstraintID,
	judgement MemberOfNotMember,
) (DisjointNotMemberUse, error) {
	return NewDirectNotMemberUse(constraint, judgement)
}

func NewDirectNotMemberUse(
	constraint ConstraintID,
	judgement MemberOfNotMember,
) (DirectNotMemberUse, error) {
	if !constraint.valid() || !validMemberOfJudgement(judgement) {
		return nil, fmt.Errorf("disjoint membership use requires a constraint and exact NotMember judgement")
	}
	writer := canonicalDisjointNotMemberUse(constraint, judgement)
	return disjointNotMemberUse{
		constraint: constraint,
		judgement:  judgement,
		canonical:  writer.bytes(),
		digest:     writer.digest(),
	}, nil
}

func (disjointNotMemberUse) Kind() DisjointCounterUseKind {
	return DirectNotMemberDisjointCounterUse
}

func (use disjointNotMemberUse) Constraint() ConstraintID { return use.constraint }

func (use disjointNotMemberUse) Judgement() MemberOfNotMember { return use.judgement }

func (use disjointNotMemberUse) CounterQuery() MemberOfQuery {
	return use.judgement.Query()
}

func (use disjointNotMemberUse) EvaluationView() MemberOfEvaluationView {
	return use.judgement.EvaluationView()
}

func (use disjointNotMemberUse) CounterRequest() MemberOfEvaluationRequest {
	return use.judgement.EvaluationRequest()
}

func (use disjointNotMemberUse) CanonicalBytes() []byte {
	return append([]byte(nil), use.canonical...)
}

func (use disjointNotMemberUse) Digest() SHA256Digest { return use.digest }

func (disjointNotMemberUse) disjointCounterUseVariant() {}

func (disjointNotMemberUse) directNotMemberUseVariant() {}

type DisjointEntailmentUseInput struct {
	TypeEnv              TypeEnv
	Constraint           KindDisjointConstraint
	SupportingMembership MemberOfMember
	MatchedOperand       KindID
	ExcludedOperand      KindID
}

type disjointEntailmentUse struct {
	constraint       KindDisjointConstraint
	constraintDigest SHA256Digest
	supporting       MemberOfMember
	matched          KindID
	excluded         KindID
	counterQuery     MemberOfQuery
	canonical        []byte
	digest           SHA256Digest
}

func NewDisjointEntailmentUse(
	input DisjointEntailmentUseInput,
) (DisjointEntailmentUse, error) {
	counterQuery, err := validateDisjointEntailmentUseInput(input)
	if err != nil {
		return nil, err
	}
	constraintDigest := digestAdmissionBytes(input.Constraint.CanonicalBytes())
	writer := canonicalDisjointEntailmentUse(
		input.Constraint,
		constraintDigest,
		input.SupportingMembership,
		input.MatchedOperand,
		input.ExcludedOperand,
		counterQuery,
	)
	return disjointEntailmentUse{
		constraint:       input.Constraint,
		constraintDigest: constraintDigest,
		supporting:       input.SupportingMembership,
		matched:          input.MatchedOperand,
		excluded:         input.ExcludedOperand,
		counterQuery:     counterQuery,
		canonical:        writer.bytes(),
		digest:           writer.digest(),
	}, nil
}

func (disjointEntailmentUse) Kind() DisjointCounterUseKind {
	return EntailedDisjointCounterUse
}

func (use disjointEntailmentUse) Constraint() ConstraintID {
	return use.constraint.ID()
}

func (use disjointEntailmentUse) ConstraintRule() KindDisjointConstraint {
	return cloneKindDisjointConstraint(use.constraint)
}

func (use disjointEntailmentUse) ConstraintDigest() SHA256Digest {
	return use.constraintDigest
}

func (use disjointEntailmentUse) SupportingMembership() MemberOfMember {
	return use.supporting
}

func (use disjointEntailmentUse) MatchedOperand() KindID { return use.matched }

func (use disjointEntailmentUse) ExcludedOperand() KindID { return use.excluded }

func (use disjointEntailmentUse) CounterQuery() MemberOfQuery {
	return use.counterQuery
}

func (use disjointEntailmentUse) EvaluationView() MemberOfEvaluationView {
	return use.supporting.EvaluationView()
}

func (use disjointEntailmentUse) CounterRequest() MemberOfEvaluationRequest {
	request, _ := NewMemberOfEvaluationRequest(
		use.counterQuery,
		use.supporting.EvaluationView(),
	)
	return request
}

func (use disjointEntailmentUse) CanonicalBytes() []byte {
	return append([]byte(nil), use.canonical...)
}

func (use disjointEntailmentUse) Digest() SHA256Digest { return use.digest }

func (disjointEntailmentUse) disjointCounterUseVariant() {}

func (disjointEntailmentUse) disjointEntailmentUseVariant() {}

func validateDisjointEntailmentUseInput(
	input DisjointEntailmentUseInput,
) (MemberOfQuery, error) {
	if err := validateTypeEnv(input.TypeEnv); err != nil {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment requires a valid exact TypeEnv: %w", err)
	}
	if !validMemberOfJudgement(input.SupportingMembership) {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment requires an exact positive MemberOf judgement")
	}
	if !validConstraintRule(input.Constraint) ||
		!exactKindDisjointConstraint(input.TypeEnv, input.Constraint) {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment requires the exact KindDisjoint constraint from the TypeEnv")
	}
	query := input.SupportingMembership.Query()
	if query.ValueKind().TypeEnv() != input.TypeEnv.Ref() ||
		input.SupportingMembership.EvaluationView().TypeEnv() != input.TypeEnv.Ref() {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment support belongs to another TypeEnv")
	}
	matches := matchingDisjointOperands(
		input.TypeEnv,
		input.Constraint,
		query.ValueKind().ID(),
	)
	if len(matches) != 1 || matches[0] != input.MatchedOperand {
		return MemberOfQuery{}, fmt.Errorf("supporting ValueKind must be below exactly the declared matched operand")
	}
	if input.ExcludedOperand == input.MatchedOperand ||
		!kindDisjointConstraintContains(input.Constraint, input.ExcludedOperand) {
		return MemberOfQuery{}, fmt.Errorf("excluded kind must be another exact operand of the KindDisjoint constraint")
	}
	counterKind, err := NewValueKindRef(input.TypeEnv.Ref(), input.ExcludedOperand)
	if err != nil {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment counter kind is invalid: %w", err)
	}
	counterQuery, err := NewMemberOfQuery(
		query.EntityID(),
		counterKind,
		query.ContextSlice(),
	)
	if err != nil {
		return MemberOfQuery{}, fmt.Errorf("disjoint entailment counter query is invalid: %w", err)
	}
	return counterQuery, nil
}

func exactKindDisjointConstraint(
	environment TypeEnv,
	wanted KindDisjointConstraint,
) bool {
	for _, rule := range environment.Constraints() {
		if rule.ID() != wanted.ID() {
			continue
		}
		actual, disjoint := rule.(KindDisjointConstraint)
		return disjoint && bytes.Equal(actual.CanonicalBytes(), wanted.CanonicalBytes())
	}
	return false
}

func matchingDisjointOperands(
	environment TypeEnv,
	constraint KindDisjointConstraint,
	valueKind KindID,
) []KindID {
	matches := make([]KindID, 0, 1)
	for _, operand := range constraint.Kinds() {
		if environment.IsSubkind(valueKind, operand) {
			matches = append(matches, operand)
		}
	}
	return matches
}

func kindDisjointConstraintContains(
	constraint KindDisjointConstraint,
	kind KindID,
) bool {
	for _, operand := range constraint.Kinds() {
		if operand == kind {
			return true
		}
	}
	return false
}

func cloneKindDisjointConstraint(
	constraint KindDisjointConstraint,
) KindDisjointConstraint {
	return KindDisjointConstraint{
		id:         constraint.id,
		kinds:      constraint.Kinds(),
		provenance: constraint.provenance,
	}
}

func canonicalDisjointNotMemberUse(
	constraint ConstraintID,
	judgement MemberOfNotMember,
) canonicalWriter {
	writer := newCanonicalWriter(disjointNotMemberUseDomain)
	writer.addString(constraint.String())
	writer.addBytes(judgement.CanonicalBytes())
	return writer
}

func canonicalDisjointEntailmentUse(
	constraint KindDisjointConstraint,
	constraintDigest SHA256Digest,
	supporting MemberOfMember,
	matched KindID,
	excluded KindID,
	counterQuery MemberOfQuery,
) canonicalWriter {
	writer := newCanonicalWriter(disjointEntailmentUseDomain)
	writer.addBytes(constraint.CanonicalBytes())
	writer.addString(constraintDigest.String())
	writer.addBytes(supporting.CanonicalBytes())
	writer.addString(matched.String())
	writer.addString(excluded.String())
	writer.addBytes(counterQuery.CanonicalBytes())
	return writer
}

func validDisjointNotMemberUse(use DisjointNotMemberUse) bool {
	value, supported := use.(disjointNotMemberUse)
	if !supported ||
		!value.constraint.valid() ||
		!validMemberOfJudgement(value.judgement) {
		return false
	}
	writer := canonicalDisjointNotMemberUse(value.constraint, value.judgement)
	return canonicalValueMatches(writer, value.canonical, value.digest)
}

func validDisjointEntailmentUse(use DisjointEntailmentUse) bool {
	value, supported := use.(disjointEntailmentUse)
	if !supported ||
		!validConstraintRule(value.constraint) ||
		!validMemberOfJudgement(value.supporting) ||
		!value.matched.valid() ||
		!value.excluded.valid() ||
		value.matched == value.excluded ||
		!kindDisjointConstraintContains(value.constraint, value.matched) ||
		!kindDisjointConstraintContains(value.constraint, value.excluded) ||
		!value.counterQuery.valid() {
		return false
	}
	constraintDigest := digestAdmissionBytes(value.constraint.CanonicalBytes())
	if constraintDigest != value.constraintDigest {
		return false
	}
	supportQuery := value.supporting.Query()
	if supportQuery.EntityID() != value.counterQuery.EntityID() ||
		!sameContextSlice(supportQuery.ContextSlice(), value.counterQuery.ContextSlice()) ||
		supportQuery.ValueKind().TypeEnv() != value.counterQuery.ValueKind().TypeEnv() ||
		value.counterQuery.ValueKind().ID() != value.excluded ||
		value.supporting.EvaluationView().TypeEnv() != value.counterQuery.ValueKind().TypeEnv() {
		return false
	}
	writer := canonicalDisjointEntailmentUse(
		value.constraint,
		value.constraintDigest,
		value.supporting,
		value.matched,
		value.excluded,
		value.counterQuery,
	)
	return canonicalValueMatches(writer, value.canonical, value.digest)
}

func validDisjointCounterUse(use DisjointCounterUse) bool {
	switch value := use.(type) {
	case disjointNotMemberUse:
		return validDisjointNotMemberUse(value)
	case disjointEntailmentUse:
		return validDisjointEntailmentUse(value)
	default:
		return false
	}
}

func normalizeDisjointCounterUses(
	values []DisjointCounterUse,
) ([]DisjointCounterUse, error) {
	result := append([]DisjointCounterUse(nil), values...)
	for _, use := range result {
		if !validDisjointCounterUse(use) {
			return nil, fmt.Errorf("reference-filler admission use contains an invalid disjoint counter basis")
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftConstraint := result[left].Constraint().String()
		rightConstraint := result[right].Constraint().String()
		if leftConstraint != rightConstraint {
			return leftConstraint < rightConstraint
		}
		leftKind := result[left].CounterQuery().ValueKind().String()
		rightKind := result[right].CounterQuery().ValueKind().String()
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]DisjointCounterUse, 0, len(result))
	for _, use := range result {
		if len(normalized) == 0 ||
			!sameDisjointCounterPosition(normalized[len(normalized)-1], use) {
			normalized = append(normalized, use)
			continue
		}
		previous := normalized[len(normalized)-1]
		if bytes.Equal(previous.CanonicalBytes(), use.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"constraint %q has conflicting disjoint counter admission uses for ValueKind %q",
			use.Constraint().String(),
			use.CounterQuery().ValueKind().String(),
		)
	}
	return normalized, nil
}

func sameDisjointCounterPosition(
	left DisjointCounterUse,
	right DisjointCounterUse,
) bool {
	return left.Constraint() == right.Constraint() &&
		left.CounterQuery().ValueKind() == right.CounterQuery().ValueKind()
}

func validateExactDisjointCounterUseSet(
	environment TypeEnv,
	required MemberOfMember,
	uses []DisjointCounterUse,
) error {
	if err := validateTypeEnv(environment); err != nil {
		return fmt.Errorf("disjoint counter coverage requires a valid exact TypeEnv: %w", err)
	}
	if !validMemberOfJudgement(required) {
		return fmt.Errorf("disjoint counter coverage requires an exact positive MemberOf judgement")
	}
	query := required.Query()
	if query.ValueKind().TypeEnv() != environment.Ref() ||
		required.EvaluationView().TypeEnv() != environment.Ref() {
		return fmt.Errorf("required MemberOf support belongs to another TypeEnv")
	}
	normalized, err := normalizeDisjointCounterUses(uses)
	if err != nil {
		return err
	}
	if len(normalized) != len(uses) {
		return fmt.Errorf("disjoint counter uses must already be exact and duplicate-free")
	}
	for _, use := range normalized {
		if err := validateDisjointCounterCorrelation(environment, required, use); err != nil {
			return err
		}
	}
	expected, err := expectedDisjointUsePositions(environment, query.ValueKind())
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(normalized))
	for _, use := range normalized {
		actual = append(actual, disjointUsePositionKey(
			use.Constraint(),
			use.CounterQuery().ValueKind(),
		))
	}
	sort.Strings(actual)
	if len(expected) != len(actual) {
		return fmt.Errorf("disjoint counter uses do not cover the exact expected counter-kind set")
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return fmt.Errorf("disjoint counter uses do not cover the exact expected counter-kind set")
		}
	}
	return nil
}

func validateDisjointCounterCorrelation(
	environment TypeEnv,
	required MemberOfMember,
	use DisjointCounterUse,
) error {
	requiredRequest := required.EvaluationRequest()
	counterRequest := use.CounterRequest()
	if !requiredRequest.valid() || !counterRequest.valid() {
		return fmt.Errorf("disjoint counter use lacks an exact MemberOf evaluation request")
	}
	requiredQuery := requiredRequest.Query()
	counterQuery := counterRequest.Query()
	if counterQuery.EntityID() != requiredQuery.EntityID() ||
		!sameContextSlice(counterQuery.ContextSlice(), requiredQuery.ContextSlice()) ||
		counterQuery.ValueKind().TypeEnv() != requiredQuery.ValueKind().TypeEnv() ||
		counterQuery.ValueKind() == requiredQuery.ValueKind() {
		return fmt.Errorf("disjoint counter use is not correlated to the required MemberOf query")
	}
	if !sameMemberOfEvaluationView(requiredRequest.View(), counterRequest.View()) {
		return fmt.Errorf("disjoint counter use does not use the exact required MemberOf evaluation view")
	}
	entailed, isEntailed := use.(DisjointEntailmentUse)
	if !isEntailed {
		return nil
	}
	support := entailed.SupportingMembership()
	if support.Digest() != required.Digest() ||
		!bytes.Equal(support.CanonicalBytes(), required.CanonicalBytes()) {
		return fmt.Errorf("disjoint entailment does not use the exact required positive MemberOf support")
	}
	rebuiltCounter, err := validateDisjointEntailmentUseInput(DisjointEntailmentUseInput{
		TypeEnv:              environment,
		Constraint:           entailed.ConstraintRule(),
		SupportingMembership: support,
		MatchedOperand:       entailed.MatchedOperand(),
		ExcludedOperand:      entailed.ExcludedOperand(),
	})
	if err != nil ||
		rebuiltCounter.Digest() != counterQuery.Digest() ||
		!bytes.Equal(rebuiltCounter.CanonicalBytes(), counterQuery.CanonicalBytes()) {
		return fmt.Errorf("disjoint entailment does not match the exact current constraint and counter query")
	}
	return nil
}

type ReferenceFillerAdmissionUse interface {
	Coordinate() RelationFillerCoordinate
	Resolution() AdmissionReferenceResolution
	RequiredMembership() MemberOfMember
	DisjointMemberships() []DisjointCounterUse
	CanonicalBytes() []byte
	Digest() SHA256Digest
	referenceFillerAdmissionUseVariant()
}

type ReferenceFillerAdmissionUseInput struct {
	TypeEnv             TypeEnv
	Coordinate          RelationFillerCoordinate
	Resolution          AdmissionReferenceResolution
	RequiredMembership  MemberOfMember
	DisjointMemberships []DisjointCounterUse
}

type referenceFillerAdmissionUse struct {
	coordinate RelationFillerCoordinate
	resolution AdmissionReferenceResolution
	required   MemberOfMember
	disjoint   []DisjointCounterUse
	canonical  []byte
	digest     SHA256Digest
}

func NewReferenceFillerAdmissionUse(
	input ReferenceFillerAdmissionUseInput,
) (ReferenceFillerAdmissionUse, error) {
	if err := validateTypeEnv(input.TypeEnv); err != nil {
		return nil, fmt.Errorf("reference-filler admission use requires a valid exact TypeEnv: %w", err)
	}
	if !validRelationFillerCoordinate(input.Coordinate) {
		return nil, fmt.Errorf("reference-filler admission use requires an exact final relation-filler coordinate")
	}
	if !validAdmissionReferenceResolution(input.Resolution) {
		return nil, fmt.Errorf("reference-filler admission use requires a defined reference resolution")
	}
	if !validMemberOfJudgement(input.RequiredMembership) {
		return nil, fmt.Errorf("reference-filler admission use requires a defined positive MemberOf judgement")
	}
	disjoint, err := normalizeDisjointCounterUses(input.DisjointMemberships)
	if err != nil {
		return nil, err
	}
	if err := validateReferenceFillerUseCorrelation(
		input.TypeEnv,
		input.Coordinate,
		input.Resolution,
		input.RequiredMembership,
		disjoint,
	); err != nil {
		return nil, err
	}
	writer := canonicalReferenceFillerAdmissionUse(
		input.Coordinate,
		input.Resolution,
		input.RequiredMembership,
		disjoint,
	)
	return referenceFillerAdmissionUse{
		coordinate: input.Coordinate,
		resolution: input.Resolution,
		required:   input.RequiredMembership,
		disjoint:   disjoint,
		canonical:  writer.bytes(),
		digest:     writer.digest(),
	}, nil
}

func (use referenceFillerAdmissionUse) Coordinate() RelationFillerCoordinate {
	return use.coordinate
}

func (use referenceFillerAdmissionUse) Resolution() AdmissionReferenceResolution {
	return use.resolution
}

func (use referenceFillerAdmissionUse) RequiredMembership() MemberOfMember {
	return use.required
}

func (use referenceFillerAdmissionUse) DisjointMemberships() []DisjointCounterUse {
	return append([]DisjointCounterUse(nil), use.disjoint...)
}

func (use referenceFillerAdmissionUse) CanonicalBytes() []byte {
	return append([]byte(nil), use.canonical...)
}

func (use referenceFillerAdmissionUse) Digest() SHA256Digest { return use.digest }

func (referenceFillerAdmissionUse) referenceFillerAdmissionUseVariant() {}

func canonicalReferenceFillerAdmissionUse(
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required MemberOfMember,
	disjoint []DisjointCounterUse,
) canonicalWriter {
	writer := newCanonicalWriter(referenceFillerAdmissionUseDomain)
	writer.addBytes(coordinate.CanonicalBytes())
	writer.addBytes(resolution.CanonicalBytes())
	writer.addBytes(required.CanonicalBytes())
	writer.addUint64(uint64(len(disjoint)))
	for _, use := range disjoint {
		writer.addBytes(use.CanonicalBytes())
	}
	return writer
}

func validateReferenceFillerUseCorrelation(
	environment TypeEnv,
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required MemberOfMember,
	disjoint []DisjointCounterUse,
) error {
	if err := validateReferenceFillerUseStructure(
		coordinate,
		resolution,
		required,
		disjoint,
	); err != nil {
		return err
	}
	query := required.Query()
	if environment.Ref() != query.ValueKind().TypeEnv() {
		return fmt.Errorf("required MemberOf ValueKind, reference filler, and exact TypeEnv do not match")
	}
	return validateExactDisjointCounterUseSet(environment, required, disjoint)
}

func validateReferenceFillerUseStructure(
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	required MemberOfMember,
	disjoint []DisjointCounterUse,
) error {
	if coordinate.Reference() != resolution.PersistedReference() ||
		coordinate.Entity() != resolution.Entity() {
		return fmt.Errorf("reference resolution does not identify the exact admitted relation filler")
	}
	query := required.Query()
	if query.EntityID() != coordinate.Entity() {
		return fmt.Errorf("required MemberOf judgement is for another stable EntityID")
	}
	if query.ContextSlice().Context() != resolution.Context() ||
		!sameContextSlice(query.ContextSlice(), coordinate.ContextSlice()) {
		return fmt.Errorf("required MemberOf judgement does not use the resolved final-relation ContextSlice")
	}
	if query.ValueKind() != coordinate.RequiredValueKind() {
		return fmt.Errorf("required MemberOf judgement does not use the final slot ValueKind")
	}
	if err := validateResolutionEvaluationView(coordinate, resolution, required.EvaluationView()); err != nil {
		return err
	}
	for _, use := range disjoint {
		disjointRequest := use.CounterRequest()
		if !disjointRequest.valid() {
			return fmt.Errorf("disjoint counter use lacks an exact MemberOf evaluation request")
		}
		disjointQuery := disjointRequest.Query()
		if disjointQuery.EntityID() != query.EntityID() ||
			!sameContextSlice(disjointQuery.ContextSlice(), query.ContextSlice()) ||
			disjointQuery.ValueKind().TypeEnv() != query.ValueKind().TypeEnv() ||
			disjointQuery.ValueKind() == query.ValueKind() {
			return fmt.Errorf("disjoint NotMember use is not correlated to the required MemberOf query")
		}
		if !sameMemberOfEvaluationView(required.EvaluationView(), disjointRequest.View()) {
			return fmt.Errorf("disjoint counter use does not use the exact required MemberOf evaluation view")
		}
	}
	return nil
}

func validateResolutionEvaluationView(
	coordinate RelationFillerCoordinate,
	resolution AdmissionReferenceResolution,
	evaluationView MemberOfEvaluationView,
) error {
	switch value := resolution.(type) {
	case sameBatchDeclarationResolution:
		view, prospective := evaluationView.(ProspectiveBatchView)
		if !prospective {
			return fmt.Errorf("same-batch reference resolution requires a prospective MemberOf evaluation view")
		}
		if view.EvaluationChangeOrdinal() != coordinate.ChangeOrdinal() ||
			view.DeclarationChangeOrdinal() != value.DeclarationChangeOrdinal() ||
			view.LocalReference() != value.LocalReference() ||
			view.PersistedReference() != value.PersistedReference() ||
			view.Declaration().Entity() != value.Entity() ||
			view.Declaration().Context() != value.Context() ||
			view.DeclarationDigest() != value.DeclarationDigest() ||
			!bytes.Equal(view.DeclarationCanonicalBytes(), value.DeclarationCanonicalBytes()) {
			return fmt.Errorf("same-batch reference resolution and prospective MemberOf view do not match exactly")
		}
	case snapshotReferenceResolution:
		if _, persisted := evaluationView.(PersistedSnapshotView); !persisted {
			return fmt.Errorf("snapshot reference resolution requires a persisted-snapshot MemberOf evaluation view")
		}
	default:
		return fmt.Errorf("unsupported reference resolution %T", resolution)
	}
	return nil
}

func expectedDisjointUsePositions(
	environment TypeEnv,
	required ValueKindRef,
) ([]string, error) {
	positions := make([]string, 0)
	for _, rule := range environment.Constraints() {
		constraint, disjoint := rule.(KindDisjointConstraint)
		if !disjoint {
			continue
		}
		matched := make([]KindID, 0, 1)
		for _, operand := range constraint.Kinds() {
			if environment.IsSubkind(required.ID(), operand) {
				matched = append(matched, operand)
			}
		}
		if len(matched) > 1 {
			return nil, fmt.Errorf("required ValueKind is below multiple operands of one disjoint constraint")
		}
		if len(matched) == 0 {
			continue
		}
		for _, operand := range constraint.Kinds() {
			if operand == matched[0] {
				continue
			}
			counter, err := NewValueKindRef(environment.Ref(), operand)
			if err != nil {
				return nil, fmt.Errorf("disjoint counter-kind is invalid: %w", err)
			}
			positions = append(positions, disjointUsePositionKey(constraint.ID(), counter))
		}
	}
	sort.Strings(positions)
	return positions, nil
}

func disjointUsePositionKey(
	constraint ConstraintID,
	counter ValueKindRef,
) string {
	return exactTupleKey(
		"admission-disjoint-use-position",
		constraint.String(),
		counter.String(),
	)
}

func validReferenceFillerAdmissionUse(use ReferenceFillerAdmissionUse) bool {
	value, supported := use.(referenceFillerAdmissionUse)
	if !supported ||
		!validRelationFillerCoordinate(value.coordinate) ||
		!validAdmissionReferenceResolution(value.resolution) ||
		!validMemberOfJudgement(value.required) {
		return false
	}
	disjoint, err := normalizeDisjointCounterUses(value.disjoint)
	if err != nil || len(disjoint) != len(value.disjoint) {
		return false
	}
	if err := validateReferenceFillerUseStructure(
		value.coordinate,
		value.resolution,
		value.required,
		disjoint,
	); err != nil {
		return false
	}
	writer := canonicalReferenceFillerAdmissionUse(
		value.coordinate,
		value.resolution,
		value.required,
		disjoint,
	)
	return canonicalValueMatches(writer, value.canonical, value.digest)
}

func normalizeReferenceFillerAdmissionUses(
	values []ReferenceFillerAdmissionUse,
) ([]ReferenceFillerAdmissionUse, error) {
	result := append([]ReferenceFillerAdmissionUse(nil), values...)
	for _, use := range result {
		if !validReferenceFillerAdmissionUse(use) {
			return nil, fmt.Errorf("context-slice membership basis contains an invalid reference-filler use")
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftPosition := relationFillerPositionKey(result[left].Coordinate())
		rightPosition := relationFillerPositionKey(result[right].Coordinate())
		if leftPosition != rightPosition {
			return leftPosition < rightPosition
		}
		coordinateComparison := bytes.Compare(
			result[left].Coordinate().CanonicalBytes(),
			result[right].Coordinate().CanonicalBytes(),
		)
		if coordinateComparison != 0 {
			return coordinateComparison < 0
		}
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]ReferenceFillerAdmissionUse, 0, len(result))
	for _, use := range result {
		if len(normalized) == 0 ||
			relationFillerPositionKey(normalized[len(normalized)-1].Coordinate()) != relationFillerPositionKey(use.Coordinate()) {
			normalized = append(normalized, use)
			continue
		}
		previous := normalized[len(normalized)-1]
		if !sameRelationFillerCoordinate(previous.Coordinate(), use.Coordinate()) {
			return nil, fmt.Errorf("one relation-filler position has conflicting final filler coordinates")
		}
		if bytes.Equal(previous.CanonicalBytes(), use.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf("one relation-filler coordinate has conflicting admission evidence")
	}
	return normalized, nil
}

func relationFillerPositionKey(coordinate RelationFillerCoordinate) string {
	return exactTupleKey(
		"admission-relation-filler-position",
		fmt.Sprintf("%d", coordinate.ChangeOrdinal()),
		coordinate.Assertion().String(),
		coordinate.Slot().String(),
		fmt.Sprintf("%d", coordinate.FillerOrdinal()),
	)
}

type AdmissionBasisKind uint8

const (
	SnapshotOnlyAdmissionBasis AdmissionBasisKind = iota + 1
	ContextSliceMembershipAdmissionBasis
	ContextSliceClassificationAdmissionBasis
)

const (
	snapshotOnlyAdmissionBasisKindName           = "snapshot_only"
	contextSliceMembershipAdmissionBasisKindName = "context_slice_membership"
	contextSliceClassificationBasisKindName      = "context_slice_classification"
)

func (kind AdmissionBasisKind) String() string {
	switch kind {
	case SnapshotOnlyAdmissionBasis:
		return snapshotOnlyAdmissionBasisKindName
	case ContextSliceMembershipAdmissionBasis:
		return contextSliceMembershipAdmissionBasisKindName
	case ContextSliceClassificationAdmissionBasis:
		return contextSliceClassificationBasisKindName
	default:
		return ""
	}
}

// ParseAdmissionBasisKind is the storage-boundary parser for the closed
// admission-basis kind algebra. Durable text must cross this boundary before
// it can participate in domain verification.
func ParseAdmissionBasisKind(value string) (AdmissionBasisKind, error) {
	switch value {
	case snapshotOnlyAdmissionBasisKindName:
		return SnapshotOnlyAdmissionBasis, nil
	case contextSliceMembershipAdmissionBasisKindName:
		return ContextSliceMembershipAdmissionBasis, nil
	case contextSliceClassificationBasisKindName:
		return ContextSliceClassificationAdmissionBasis, nil
	default:
		return 0, fmt.Errorf("unknown admission basis kind %q", value)
	}
}

// VerifyStoredAdmissionBasisDomain proves that durable canonical basis bytes
// carry and fully consume the exact canonical structure owned by their strong
// AdmissionBasisKind. It deliberately parses framing and domain identity
// rather than searching raw bytes for a kind-shaped string.
func VerifyStoredAdmissionBasisDomain(
	kind AdmissionBasisKind,
	canonical []byte,
) error {
	_, err := parseStoredAdmissionBasisCoordinates(kind, canonical)
	return err
}

// VerifyStoredAdmissionBasisCoordinates verifies the complete durable carrier
// shape and its exact snapshot coordinates. The expected coordinates are
// strong domain values; caller strings never participate in the comparison.
func VerifyStoredAdmissionBasisCoordinates(
	kind AdmissionBasisKind,
	expectedTypeEnv TypeEnvRef,
	expectedRevision GraphRevision,
	canonical []byte,
) error {
	if !expectedTypeEnv.valid() {
		return fmt.Errorf("stored admission basis requires an exact expected TypeEnv reference")
	}
	coordinates, err := parseStoredAdmissionBasisCoordinates(kind, canonical)
	if err != nil {
		return err
	}
	if coordinates.typeEnv != expectedTypeEnv {
		return fmt.Errorf(
			"stored %s admission basis TypeEnv %q does not match expected %q",
			kind.String(),
			coordinates.typeEnv.String(),
			expectedTypeEnv.String(),
		)
	}
	if coordinates.revision != expectedRevision {
		return fmt.Errorf(
			"stored %s admission basis graph revision %d does not match expected %d",
			kind.String(),
			coordinates.revision.Value(),
			expectedRevision.Value(),
		)
	}
	return nil
}

func admissionBasisDomain(kind AdmissionBasisKind) (string, error) {
	switch kind {
	case SnapshotOnlyAdmissionBasis:
		return snapshotOnlyAdmissionBasisDomain, nil
	case ContextSliceMembershipAdmissionBasis:
		return contextSliceMembershipAdmissionBasisDomain, nil
	case ContextSliceClassificationAdmissionBasis:
		return contextSliceClassificationBasisDomain, nil
	default:
		return "", fmt.Errorf("unknown admission basis kind %d", kind)
	}
}

type storedAdmissionBasisCoordinates struct {
	typeEnv  TypeEnvRef
	revision GraphRevision
}

func parseStoredAdmissionBasisCoordinates(
	kind AdmissionBasisKind,
	canonical []byte,
) (storedAdmissionBasisCoordinates, error) {
	domain, err := admissionBasisDomain(kind)
	if err != nil {
		return storedAdmissionBasisCoordinates{}, err
	}
	reader, err := newDomainReader(canonical, domain)
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf(
			"stored %s admission basis has the wrong canonical domain: %w",
			kind.String(),
			err,
		)
	}
	snapshotBytes, err := reader.readBytes()
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf(
			"stored %s admission basis snapshot: %w",
			kind.String(),
			err,
		)
	}
	if len(snapshotBytes) == 0 {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf(
			"stored %s admission basis snapshot is empty",
			kind.String(),
		)
	}
	if err := consumeStoredAdmissionBasisTail(kind, reader); err != nil {
		return storedAdmissionBasisCoordinates{}, err
	}
	coordinates, err := parseStoredAdmissionSnapshotCoordinates(snapshotBytes)
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf(
			"stored %s admission basis snapshot: %w",
			kind.String(),
			err,
		)
	}
	return coordinates, nil
}

func consumeStoredAdmissionBasisTail(
	kind AdmissionBasisKind,
	reader *canonicalReader,
) error {
	switch kind {
	case SnapshotOnlyAdmissionBasis:
		return reader.requireEnd()
	case ContextSliceMembershipAdmissionBasis:
		if err := consumeCountedCanonicalFields(reader, "reference-filler admission use"); err != nil {
			return err
		}
		return reader.requireEnd()
	case ContextSliceClassificationAdmissionBasis:
		if err := consumeCountedCanonicalFields(reader, "classification reference-filler admission use"); err != nil {
			return err
		}
		return reader.requireEnd()
	default:
		return fmt.Errorf("unknown admission basis kind %d", kind)
	}
}

func parseStoredAdmissionSnapshotCoordinates(
	canonical []byte,
) (storedAdmissionBasisCoordinates, error) {
	reader, err := newDomainReader(canonical, admissionSnapshotBasisDomain)
	if err != nil {
		return storedAdmissionBasisCoordinates{}, err
	}
	typeEnvRaw, err := reader.readString()
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf("TypeEnv reference: %w", err)
	}
	typeEnv, err := ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf(
			"stored admission basis TypeEnv reference: %w",
			err,
		)
	}
	revisionValue, err := reader.readUint64()
	if err != nil {
		return storedAdmissionBasisCoordinates{}, fmt.Errorf("graph revision: %w", err)
	}
	if err := consumeCountedCanonicalFields(reader, "snapshot observation"); err != nil {
		return storedAdmissionBasisCoordinates{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return storedAdmissionBasisCoordinates{}, err
	}
	return storedAdmissionBasisCoordinates{
		typeEnv:  typeEnv,
		revision: NewGraphRevision(revisionValue),
	}, nil
}

func consumeCountedCanonicalFields(
	reader *canonicalReader,
	label string,
) error {
	count, err := reader.readCount()
	if err != nil {
		return fmt.Errorf("%s count: %w", label, err)
	}
	if count == 0 {
		return fmt.Errorf("stored admission basis requires at least one %s", label)
	}
	for index := uint64(0); index < count; index++ {
		value, readErr := reader.readBytes()
		if readErr != nil {
			return fmt.Errorf("%s %d: %w", label, index, readErr)
		}
		if len(value) == 0 {
			return fmt.Errorf("%s %d is empty", label, index)
		}
	}
	return nil
}

// AdmissionBasis is the sealed exact evidence carried by a successful
// admission. Snapshot-only evidence, historical MemberOf evidence, and
// current C.3.2 classification evidence are distinct variants so one cannot
// be replayed as another.
type AdmissionBasis interface {
	Kind() AdmissionBasisKind
	TypeEnv() TypeEnvRef
	GraphRevision() GraphRevision
	SnapshotObservations() []AdmissionSnapshotObservation
	CanonicalBytes() []byte
	Digest() SHA256Digest
	admissionBasisVariant()
}

type SnapshotOnlyBasis interface {
	AdmissionBasis
	snapshotOnlyBasisVariant()
}

type ContextSliceMembershipBasis interface {
	AdmissionBasis
	ReferenceFillerAdmissionUses() []ReferenceFillerAdmissionUse
	contextSliceMembershipBasisVariant()
}

type ContextSliceClassificationBasis interface {
	AdmissionBasis
	ClassificationReferenceFillerAdmissionUses() []ClassificationReferenceFillerAdmissionUse
	contextSliceClassificationBasisVariant()
}

type SnapshotOnlyBasisInput struct {
	TypeEnv       TypeEnvRef
	GraphRevision GraphRevision
	Observations  []AdmissionSnapshotObservation
}

type ContextSliceMembershipBasisInput struct {
	TypeEnv                      TypeEnvRef
	GraphRevision                GraphRevision
	Observations                 []AdmissionSnapshotObservation
	ReferenceFillerAdmissionUses []ReferenceFillerAdmissionUse
}

type admissionSnapshotBasis struct {
	typeEnv      TypeEnvRef
	revision     GraphRevision
	observations []AdmissionSnapshotObservation
	canonical    []byte
	digest       SHA256Digest
}

type snapshotOnlyBasis struct {
	snapshot  admissionSnapshotBasis
	canonical []byte
	digest    SHA256Digest
}

func NewSnapshotOnlyBasis(input SnapshotOnlyBasisInput) (SnapshotOnlyBasis, error) {
	snapshot, err := newAdmissionSnapshotBasis(
		input.TypeEnv,
		input.GraphRevision,
		input.Observations,
	)
	if err != nil {
		return nil, err
	}
	writer := newCanonicalWriter(snapshotOnlyAdmissionBasisDomain)
	writer.addBytes(snapshot.canonical)
	return snapshotOnlyBasis{
		snapshot:  snapshot,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (snapshotOnlyBasis) Kind() AdmissionBasisKind { return SnapshotOnlyAdmissionBasis }

func (basis snapshotOnlyBasis) TypeEnv() TypeEnvRef { return basis.snapshot.typeEnv }

func (basis snapshotOnlyBasis) GraphRevision() GraphRevision {
	return basis.snapshot.revision
}

func (basis snapshotOnlyBasis) SnapshotObservations() []AdmissionSnapshotObservation {
	return append([]AdmissionSnapshotObservation(nil), basis.snapshot.observations...)
}

func (basis snapshotOnlyBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis snapshotOnlyBasis) Digest() SHA256Digest { return basis.digest }

func (snapshotOnlyBasis) admissionBasisVariant() {}

func (snapshotOnlyBasis) snapshotOnlyBasisVariant() {}

type contextSliceMembershipBasis struct {
	snapshot  admissionSnapshotBasis
	uses      []ReferenceFillerAdmissionUse
	canonical []byte
	digest    SHA256Digest
}

func NewContextSliceMembershipBasis(
	input ContextSliceMembershipBasisInput,
) (ContextSliceMembershipBasis, error) {
	snapshot, err := newAdmissionSnapshotBasis(
		input.TypeEnv,
		input.GraphRevision,
		input.Observations,
	)
	if err != nil {
		return nil, err
	}
	uses, err := normalizeReferenceFillerAdmissionUses(input.ReferenceFillerAdmissionUses)
	if err != nil {
		return nil, err
	}
	if len(uses) == 0 {
		return nil, fmt.Errorf("context-slice membership admission basis requires at least one reference-filler use")
	}
	if err := validateAdmissionUseSet(input.TypeEnv, input.GraphRevision, snapshot.observations, uses); err != nil {
		return nil, err
	}
	writer := canonicalContextSliceMembershipBasis(snapshot, uses)
	return contextSliceMembershipBasis{
		snapshot:  snapshot,
		uses:      uses,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (contextSliceMembershipBasis) Kind() AdmissionBasisKind {
	return ContextSliceMembershipAdmissionBasis
}

func (basis contextSliceMembershipBasis) TypeEnv() TypeEnvRef {
	return basis.snapshot.typeEnv
}

func (basis contextSliceMembershipBasis) GraphRevision() GraphRevision {
	return basis.snapshot.revision
}

func (basis contextSliceMembershipBasis) SnapshotObservations() []AdmissionSnapshotObservation {
	return append([]AdmissionSnapshotObservation(nil), basis.snapshot.observations...)
}

func (basis contextSliceMembershipBasis) ReferenceFillerAdmissionUses() []ReferenceFillerAdmissionUse {
	return append([]ReferenceFillerAdmissionUse(nil), basis.uses...)
}

func (basis contextSliceMembershipBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis contextSliceMembershipBasis) Digest() SHA256Digest { return basis.digest }

func (contextSliceMembershipBasis) admissionBasisVariant() {}

func (contextSliceMembershipBasis) contextSliceMembershipBasisVariant() {}

func newAdmissionSnapshotBasis(
	typeEnv TypeEnvRef,
	revision GraphRevision,
	observations []AdmissionSnapshotObservation,
) (admissionSnapshotBasis, error) {
	if !typeEnv.valid() {
		return admissionSnapshotBasis{}, fmt.Errorf("admission basis requires an exact TypeEnv reference")
	}
	normalized, err := normalizeAdmissionSnapshotObservations(observations)
	if err != nil {
		return admissionSnapshotBasis{}, err
	}
	if err := validateSnapshotObservationConsistency(normalized); err != nil {
		return admissionSnapshotBasis{}, err
	}
	if len(normalized) == 0 {
		return admissionSnapshotBasis{}, fmt.Errorf("admission basis requires at least one positive snapshot observation")
	}
	writer := canonicalAdmissionSnapshotBasis(typeEnv, revision, normalized)
	return admissionSnapshotBasis{
		typeEnv:      typeEnv,
		revision:     revision,
		observations: normalized,
		canonical:    writer.bytes(),
		digest:       writer.digest(),
	}, nil
}

func validateSnapshotObservationConsistency(
	observations []AdmissionSnapshotObservation,
) error {
	postures := map[string]AdmissionSnapshotObservationKind{}
	for _, observation := range observations {
		var entity EntityID
		var context BoundedContextRef
		var kind AdmissionSnapshotObservationKind
		switch value := observation.(type) {
		case entityAbsentObservation:
			entity = value.resolution.Entity()
			context = value.resolution.Context()
			kind = EntityAbsentAdmissionObservation
		case entityExactObservation:
			entity = value.resolution.Entity()
			context = value.resolution.Context()
			kind = EntityExactAdmissionObservation
		default:
			continue
		}
		key := exactTupleKey("admission-snapshot-entity-posture", entity.String(), context.String())
		previous, exists := postures[key]
		if exists && previous != kind {
			return fmt.Errorf("one pre-state snapshot cannot prove entity %q both absent and exact in context %q", entity.String(), context.String())
		}
		postures[key] = kind
	}
	return nil
}

func canonicalAdmissionSnapshotBasis(
	typeEnv TypeEnvRef,
	revision GraphRevision,
	observations []AdmissionSnapshotObservation,
) canonicalWriter {
	writer := newCanonicalWriter(admissionSnapshotBasisDomain)
	writer.addString(typeEnv.String())
	writer.addUint64(revision.Value())
	writer.addUint64(uint64(len(observations)))
	for _, observation := range observations {
		writer.addBytes(observation.CanonicalBytes())
	}
	return writer
}

func canonicalContextSliceMembershipBasis(
	snapshot admissionSnapshotBasis,
	uses []ReferenceFillerAdmissionUse,
) canonicalWriter {
	writer := newCanonicalWriter(contextSliceMembershipAdmissionBasisDomain)
	writer.addBytes(snapshot.canonical)
	writer.addUint64(uint64(len(uses)))
	for _, use := range uses {
		writer.addBytes(use.CanonicalBytes())
	}
	return writer
}

func validateAdmissionUseSet(
	typeEnv TypeEnvRef,
	graphRevision GraphRevision,
	observations []AdmissionSnapshotObservation,
	uses []ReferenceFillerAdmissionUse,
) error {
	for _, use := range uses {
		view := use.RequiredMembership().EvaluationView()
		if use.RequiredMembership().Query().ValueKind().TypeEnv() != typeEnv ||
			use.Coordinate().Reference().RefKind().TypeEnv() != typeEnv ||
			view.TypeEnv() != typeEnv ||
			view.PreStateGraphRevision() != graphRevision {
			return fmt.Errorf("reference-filler admission use belongs to another TypeEnv")
		}
		for _, disjoint := range use.DisjointMemberships() {
			counterRequest := disjoint.CounterRequest()
			if !counterRequest.valid() ||
				counterRequest.Query().ValueKind().TypeEnv() != typeEnv ||
				!sameMemberOfEvaluationView(view, counterRequest.View()) {
				return fmt.Errorf("disjoint counter admission use belongs to another TypeEnv")
			}
		}
		if !hasSupportingAssertionAbsence(observations, use.Coordinate()) {
			return fmt.Errorf("reference-filler admission use lacks the correlated assertion-absence observation")
		}
		if _, snapshotResolution := use.Resolution().(snapshotReferenceResolution); snapshotResolution &&
			hasEntityAbsenceAtSnapshot(observations, use.Resolution().Entity(), use.Resolution().Context()) {
			return fmt.Errorf("persisted reference resolution contradicts an entity-absence observation in the same pre-state snapshot")
		}
		if err := validateSameBatchDeclarationObservation(observations, use.Resolution()); err != nil {
			return err
		}
	}
	return nil
}

func hasEntityAbsenceAtSnapshot(
	observations []AdmissionSnapshotObservation,
	entity EntityID,
	context BoundedContextRef,
) bool {
	for _, observation := range observations {
		value, absent := observation.(entityAbsentObservation)
		if absent &&
			value.resolution.Entity() == entity &&
			value.resolution.Context() == context {
			return true
		}
	}
	return false
}

func hasSupportingAssertionAbsence(
	observations []AdmissionSnapshotObservation,
	coordinate RelationFillerCoordinate,
) bool {
	for _, observation := range observations {
		value, supported := observation.(assertionAbsentObservation)
		if !supported {
			continue
		}
		if value.changeOrdinal == coordinate.ChangeOrdinal() &&
			value.state.Assertion() == coordinate.Assertion() {
			return true
		}
	}
	return false
}

func validateSameBatchDeclarationObservation(
	observations []AdmissionSnapshotObservation,
	resolution AdmissionReferenceResolution,
) error {
	value, sameBatch := resolution.(sameBatchDeclarationResolution)
	if !sameBatch {
		return nil
	}
	for _, observation := range observations {
		candidate, supported := observation.(entityAbsentObservation)
		if !supported {
			continue
		}
		resolved := candidate.resolution
		if candidate.changeOrdinal == value.declarationChangeOrdinal &&
			resolved.Entity() == value.entity &&
			resolved.Context() == value.context {
			return nil
		}
	}
	return fmt.Errorf("same-batch reference resolution lacks the correlated declaration entity-absence observation")
}

func validAdmissionSnapshotBasis(snapshot admissionSnapshotBasis) bool {
	if !snapshot.typeEnv.valid() ||
		!snapshot.digest.valid() ||
		len(snapshot.canonical) == 0 {
		return false
	}
	observations, err := normalizeAdmissionSnapshotObservations(snapshot.observations)
	if err != nil ||
		len(observations) == 0 ||
		len(observations) != len(snapshot.observations) {
		return false
	}
	writer := canonicalAdmissionSnapshotBasis(snapshot.typeEnv, snapshot.revision, observations)
	return canonicalValueMatches(writer, snapshot.canonical, snapshot.digest)
}

func validAdmissionBasis(basis AdmissionBasis) bool {
	switch value := basis.(type) {
	case snapshotOnlyBasis:
		if !validAdmissionSnapshotBasis(value.snapshot) {
			return false
		}
		writer := newCanonicalWriter(snapshotOnlyAdmissionBasisDomain)
		writer.addBytes(value.snapshot.canonical)
		return canonicalValueMatches(writer, value.canonical, value.digest)
	case contextSliceMembershipBasis:
		if !validAdmissionSnapshotBasis(value.snapshot) {
			return false
		}
		uses, err := normalizeReferenceFillerAdmissionUses(value.uses)
		if err != nil || len(uses) == 0 || len(uses) != len(value.uses) {
			return false
		}
		if err := validateAdmissionUseSet(value.snapshot.typeEnv, value.snapshot.revision, value.snapshot.observations, uses); err != nil {
			return false
		}
		writer := canonicalContextSliceMembershipBasis(value.snapshot, uses)
		return canonicalValueMatches(writer, value.canonical, value.digest)
	case contextSliceClassificationBasis:
		if !validAdmissionSnapshotBasis(value.snapshot) {
			return false
		}
		uses, err := normalizeClassificationReferenceFillerAdmissionUses(value.uses)
		if err != nil || len(uses) == 0 || len(uses) != len(value.uses) {
			return false
		}
		if err := validateClassificationAdmissionUseSet(
			value.snapshot.typeEnv,
			value.snapshot.observations,
			uses,
		); err != nil {
			return false
		}
		writer := newCanonicalWriter(contextSliceClassificationBasisDomain)
		writer.addBytes(value.snapshot.canonical)
		writer.addUint64(uint64(len(uses)))
		for _, use := range uses {
			writer.addBytes(use.CanonicalBytes())
		}
		return canonicalValueMatches(writer, value.canonical, value.digest)
	default:
		return false
	}
}

func sameRelationFillerCoordinate(
	left RelationFillerCoordinate,
	right RelationFillerCoordinate,
) bool {
	return left.Digest() == right.Digest() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func sameContextSlice(left ContextSlice, right ContextSlice) bool {
	return left.Digest() == right.Digest() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func canonicalValueMatches(
	writer canonicalWriter,
	canonical []byte,
	digest SHA256Digest,
) bool {
	return digest.valid() &&
		writer.digest() == digest &&
		bytes.Equal(writer.bytes(), canonical)
}

func digestAdmissionBytes(value []byte) SHA256Digest {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	return SHA256Digest{value: sha256Prefix + encoded}
}
