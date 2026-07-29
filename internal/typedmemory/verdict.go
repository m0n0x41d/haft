package typedmemory

import (
	"bytes"
	"fmt"
)

const admissionEnvelopeDomain = "typed-memory-admission-envelope.v1"

type ValidationVerdictKind string

const (
	ValidationValid           ValidationVerdictKind = "valid"
	ValidationInvalid         ValidationVerdictKind = "invalid"
	ValidationUnderdetermined ValidationVerdictKind = "underdetermined"
)

type ValidationVerdict interface {
	Kind() ValidationVerdictKind
	validationVerdictVariant()
}

type Valid interface {
	ValidationVerdict
	// SemanticChangeDigest identifies the normalized validated effect set. It
	// is neither the original request digest nor the admission-envelope digest.
	SemanticChangeDigest() SHA256Digest
	ChangeSet() ValidatedMemoryChangeSet
	AdmissionBatch() AdmissionBatch
	validVerdictVariant()
}

type validVerdict struct {
	changeSet            ValidatedMemoryChangeSet
	admissionBatch       AdmissionBatch
	semanticChangeDigest SHA256Digest
}

// AdmissionBatch is an opaque capability produced only by successful pure
// validation. Its zero value is deliberately non-admissible. Effect adapters
// can inspect IsValid before writing, but callers cannot construct a valid
// batch or replace its candidate, semantic change set, or exact admission
// basis. The request, semantic, and admission-envelope digests are deliberately
// distinct identities.
type AdmissionBatch struct {
	candidate               MemoryChangeSet
	changeSet               ValidatedMemoryChangeSet
	basis                   AdmissionBasis
	requestDigest           SHA256Digest
	semanticChangeDigest    SHA256Digest
	canonicalEnvelope       []byte
	admissionEnvelopeDigest SHA256Digest
	sealed                  bool
}

// StoredAdmissionEnvelopeInput supplies the exact durable carrier fields
// needed to reconstruct an admission envelope without parsing project data
// back into domain objects.
type StoredAdmissionEnvelopeInput struct {
	RequestDigest  SHA256Digest
	SemanticDigest SHA256Digest
	SemanticBytes  []byte
	BasisKind      AdmissionBasisKind
	BasisDigest    SHA256Digest
	BasisBytes     []byte
	EnvelopeDigest SHA256Digest
	EnvelopeBytes  []byte
}

// VerifyStoredAdmissionEnvelope reconstructs the domain-framed admission
// envelope from its durable child carriers and verifies both its exact bytes
// and digest. It does not replace validation of each child carrier's own
// bytes-to-digest identity.
func VerifyStoredAdmissionEnvelope(input StoredAdmissionEnvelopeInput) error {
	digestsValid := input.RequestDigest.valid() &&
		input.SemanticDigest.valid() &&
		input.BasisDigest.valid() &&
		input.EnvelopeDigest.valid()
	if !digestsValid {
		return fmt.Errorf("stored admission envelope requires canonical SHA256 digests")
	}
	if err := VerifyStoredAdmissionBasisDomain(input.BasisKind, input.BasisBytes); err != nil {
		return err
	}
	writer := newCanonicalWriter(admissionEnvelopeDomain)
	writer.addString(input.RequestDigest.String())
	writer.addString(input.SemanticDigest.String())
	writer.addBytes(input.SemanticBytes)
	writer.addString(input.BasisDigest.String())
	writer.addBytes(input.BasisBytes)
	if !canonicalValueMatches(writer, input.EnvelopeBytes, input.EnvelopeDigest) {
		return fmt.Errorf("stored admission envelope does not correlate its exact child carriers")
	}
	return nil
}

func (batch AdmissionBatch) IsValid() bool {
	if !batch.sealed ||
		!batch.candidate.valid() ||
		!batch.changeSet.valid() ||
		!validAdmissionBasis(batch.basis) {
		return false
	}
	if !validAdmissionCorrelation(batch.candidate, batch.changeSet, batch.basis) {
		return false
	}
	requestDigest, err := batch.candidate.Digest()
	if err != nil || requestDigest != batch.requestDigest {
		return false
	}
	semanticBytes, err := batch.changeSet.CanonicalBytes()
	if err != nil {
		return false
	}
	semanticDigest, err := batch.changeSet.CanonicalDigest()
	if err != nil || semanticDigest != batch.semanticChangeDigest {
		return false
	}
	writer := canonicalAdmissionEnvelope(
		requestDigest,
		semanticDigest,
		semanticBytes,
		batch.basis,
	)
	return canonicalValueMatches(
		writer,
		batch.canonicalEnvelope,
		batch.admissionEnvelopeDigest,
	)
}

func (batch AdmissionBatch) ChangeSet() ValidatedMemoryChangeSet {
	if !batch.IsValid() {
		return ValidatedMemoryChangeSet{}
	}
	return newValidatedMemoryChangeSet(batch.changeSet.changes)
}

func (batch AdmissionBatch) Basis() AdmissionBasis {
	if !batch.IsValid() {
		return nil
	}
	basis, err := copyAdmissionBasis(batch.basis)
	if err != nil {
		return nil
	}
	return basis
}

func (batch AdmissionBatch) RequestDigest() SHA256Digest {
	if !batch.IsValid() {
		return SHA256Digest{}
	}
	return batch.requestDigest
}

func (batch AdmissionBatch) SemanticChangeDigest() SHA256Digest {
	if !batch.IsValid() {
		return SHA256Digest{}
	}
	return batch.semanticChangeDigest
}

func (batch AdmissionBatch) CanonicalEnvelopeBytes() []byte {
	if !batch.IsValid() {
		return nil
	}
	return append([]byte(nil), batch.canonicalEnvelope...)
}

func (batch AdmissionBatch) AdmissionEnvelopeDigest() SHA256Digest {
	if !batch.IsValid() {
		return SHA256Digest{}
	}
	return batch.admissionEnvelopeDigest
}

func newValidVerdict(
	candidate MemoryChangeSet,
	changeSet ValidatedMemoryChangeSet,
	basis AdmissionBasis,
) (Valid, error) {
	if !candidate.valid() {
		return nil, fmt.Errorf("Valid verdict requires a valid non-empty candidate change set")
	}
	if !changeSet.valid() {
		return nil, fmt.Errorf("Valid verdict requires a valid non-empty validated change set")
	}
	if !validAdmissionBasis(basis) {
		return nil, fmt.Errorf("Valid verdict requires a sealed exact admission basis")
	}
	if !validAdmissionCorrelation(candidate, changeSet, basis) {
		return nil, fmt.Errorf("Valid verdict candidate, semantic change set, and admission basis do not correlate")
	}
	requestDigest, err := candidate.Digest()
	if err != nil {
		return nil, fmt.Errorf("Valid verdict requires a canonical request digest: %w", err)
	}
	semanticBytes, err := changeSet.CanonicalBytes()
	if err != nil {
		return nil, fmt.Errorf("Valid verdict requires canonical semantic bytes: %w", err)
	}
	semanticDigest, err := changeSet.CanonicalDigest()
	if err != nil {
		return nil, fmt.Errorf("Valid verdict requires a canonical semantic digest: %w", err)
	}
	sealedBasis, err := copyAdmissionBasis(basis)
	if err != nil {
		return nil, fmt.Errorf("Valid verdict requires an immutable admission basis: %w", err)
	}
	writer := canonicalAdmissionEnvelope(
		requestDigest,
		semanticDigest,
		semanticBytes,
		sealedBasis,
	)
	batch := AdmissionBatch{
		candidate:               copyMemoryChangeSet(candidate),
		changeSet:               newValidatedMemoryChangeSet(changeSet.changes),
		basis:                   sealedBasis,
		requestDigest:           requestDigest,
		semanticChangeDigest:    semanticDigest,
		canonicalEnvelope:       writer.bytes(),
		admissionEnvelopeDigest: writer.digest(),
		sealed:                  true,
	}
	if !batch.IsValid() {
		return nil, fmt.Errorf("Valid verdict could not seal a self-validating admission envelope")
	}
	return validVerdict{
		changeSet:            newValidatedMemoryChangeSet(changeSet.changes),
		admissionBatch:       batch,
		semanticChangeDigest: semanticDigest,
	}, nil
}

func (validVerdict) Kind() ValidationVerdictKind { return ValidationValid }

func (verdict validVerdict) SemanticChangeDigest() SHA256Digest {
	return verdict.semanticChangeDigest
}

func (verdict validVerdict) ChangeSet() ValidatedMemoryChangeSet { return verdict.changeSet }

func (verdict validVerdict) AdmissionBatch() AdmissionBatch {
	return verdict.admissionBatch
}

func (validVerdict) validationVerdictVariant() {}

func (validVerdict) validVerdictVariant() {}

func canonicalAdmissionEnvelope(
	requestDigest SHA256Digest,
	semanticDigest SHA256Digest,
	semanticBytes []byte,
	basis AdmissionBasis,
) canonicalWriter {
	writer := newCanonicalWriter(admissionEnvelopeDomain)
	writer.addString(requestDigest.String())
	writer.addString(semanticDigest.String())
	writer.addBytes(semanticBytes)
	writer.addString(basis.Digest().String())
	writer.addBytes(basis.CanonicalBytes())
	return writer
}

func copyMemoryChangeSet(candidate MemoryChangeSet) MemoryChangeSet {
	return MemoryChangeSet{changes: append([]MemoryChange(nil), candidate.changes...)}
}

func copyAdmissionBasis(basis AdmissionBasis) (AdmissionBasis, error) {
	switch value := basis.(type) {
	case snapshotOnlyBasis:
		return NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
			TypeEnv:       value.TypeEnv(),
			GraphRevision: value.GraphRevision(),
			Observations:  value.SnapshotObservations(),
		})
	case contextSliceMembershipBasis:
		return NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
			TypeEnv:                      value.TypeEnv(),
			GraphRevision:                value.GraphRevision(),
			Observations:                 value.SnapshotObservations(),
			ReferenceFillerAdmissionUses: value.ReferenceFillerAdmissionUses(),
		})
	case contextSliceClassificationBasis:
		return NewContextSliceClassificationBasis(
			ContextSliceClassificationBasisInput{
				TypeEnv:       value.TypeEnv(),
				GraphRevision: value.GraphRevision(),
				Observations:  value.SnapshotObservations(),
				ClassificationReferenceFillerAdmissionUses: value.ClassificationReferenceFillerAdmissionUses(),
			},
		)
	default:
		return nil, fmt.Errorf("unsupported admission basis variant %T", basis)
	}
}

func validAdmissionCorrelation(
	candidate MemoryChangeSet,
	changeSet ValidatedMemoryChangeSet,
	basis AdmissionBasis,
) bool {
	if len(candidate.changes) != len(changeSet.changes) {
		return false
	}
	for ordinal := range candidate.changes {
		if !validChangeCorrelation(
			uint64(ordinal),
			candidate,
			candidate.changes[ordinal],
			changeSet.changes[ordinal],
			basis,
		) {
			return false
		}
	}
	if !validObservationCorrelation(candidate, basis) {
		return false
	}
	return validReferenceUseClosure(changeSet, basis)
}

func validChangeCorrelation(
	ordinal uint64,
	candidateSet MemoryChangeSet,
	candidate MemoryChange,
	validated ValidatedMemoryChange,
	basis AdmissionBasis,
) bool {
	switch value := candidate.(type) {
	case DeclareEntity:
		admitted, ok := validated.(ValidatedDeclareEntity)
		return ok && sameEntityDeclaration(value, admitted.Change())
	case ApplyIdentityChange:
		admitted, ok := validated.(ValidatedIdentityChange)
		return ok && sameAdmissibleIdentityChange(value.Change(), admitted.Change())
	case InstantiateRelation:
		admitted, ok := validated.(ValidatedRelationInstance)
		return ok && validRelationCorrelation(
			ordinal,
			candidateSet,
			value.Relation(),
			admitted.Relation(),
			basis,
		)
	case AssertRelation:
		admitted, ok := validated.(ValidatedRelationalAssertion)
		return ok && validRelationalAssertionCorrelation(
			ordinal,
			candidateSet,
			value.Assertion(),
			admitted.Assertion(),
			basis,
		)
	case RetractAssertion:
		admitted, ok := validated.(ValidatedRetraction)
		return ok && sameRetraction(value, admitted.Change())
	default:
		return false
	}
}

func sameEntityDeclaration(
	candidate DeclareEntity,
	admitted AdmittedEntityDeclaration,
) bool {
	return candidate.Entity() == admitted.Entity() &&
		candidate.Context() == admitted.Context() &&
		candidate.Label() == admitted.Label() &&
		candidate.Provenance() == admitted.Provenance()
}

func sameAdmissibleIdentityChange(candidate IdentityChange, admitted IdentityChange) bool {
	switch value := candidate.(type) {
	case AdmitAlias:
		other, ok := admitted.(AdmitAlias)
		return ok && value == other
	case SupersedeAlias:
		other, ok := admitted.(SupersedeAlias)
		return ok && value == other
	default:
		return false
	}
}

func sameRetraction(candidate RetractAssertion, admitted RetractAssertion) bool {
	return candidate.Assertion() == admitted.Assertion() &&
		candidate.Reason() == admitted.Reason() &&
		candidate.Provenance() == admitted.Provenance()
}

func validRelationCorrelation(
	ordinal uint64,
	candidateSet MemoryChangeSet,
	candidate RelationInstantiation,
	admitted RelationInstance,
	basis AdmissionBasis,
) bool {
	return validRelationalCarrierCorrelation(
		ordinal,
		candidateSet,
		candidate,
		admitted,
		basis,
	)
}

func validRelationalAssertionCorrelation(
	ordinal uint64,
	candidateSet MemoryChangeSet,
	candidate RelationalAssertionCandidate,
	admitted RelationalAssertion,
	basis AdmissionBasis,
) bool {
	return sameAssertionModality(candidate.Modality(), admitted.Modality()) &&
		validRelationalCarrierCorrelation(
			ordinal,
			candidateSet,
			candidate,
			admitted,
			basis,
		)
}

func validRelationalCarrierCorrelation(
	ordinal uint64,
	candidateSet MemoryChangeSet,
	candidate candidateRelationalCarrier,
	admitted validatedRelationalCarrier,
	basis AdmissionBasis,
) bool {
	if candidate.Assertion() != admitted.Assertion() ||
		candidate.Signature() != admitted.Signature() ||
		candidate.Signature().TypeEnv() != basis.TypeEnv() ||
		!sameContextSlice(candidate.Slice(), admitted.Slice()) ||
		candidate.Provenance() != admitted.Provenance() {
		return false
	}
	candidateBindings := candidate.Bindings()
	admittedBindings := admitted.Bindings()
	if len(candidateBindings) != len(admittedBindings) {
		return false
	}
	for index := range candidateBindings {
		if !validBindingCorrelation(
			ordinal,
			candidateSet,
			candidate,
			candidateBindings[index],
			admittedBindings[index],
			basis,
		) {
			return false
		}
	}
	return true
}

func validBindingCorrelation(
	ordinal uint64,
	candidateSet MemoryChangeSet,
	relation candidateRelationalCarrier,
	candidate CandidateSlotBinding,
	admitted SlotBinding,
	basis AdmissionBasis,
) bool {
	if candidate.Name() != admitted.Name() {
		return false
	}
	candidateFillers := candidate.Fillers()
	admittedFillers := admitted.Fillers()
	if len(candidateFillers) != len(admittedFillers) {
		return false
	}
	matched := make([]bool, len(candidateFillers))
	for fillerOrdinal, filler := range admittedFillers {
		index := matchingCandidateFiller(
			ordinal,
			uint64(fillerOrdinal),
			candidateSet,
			relation,
			candidate.Name(),
			candidateFillers,
			matched,
			filler,
			basis,
		)
		if index < 0 {
			return false
		}
		matched[index] = true
	}
	return true
}

func matchingCandidateFiller(
	changeOrdinal uint64,
	fillerOrdinal uint64,
	candidateSet MemoryChangeSet,
	relation candidateRelationalCarrier,
	slot SlotKindID,
	candidates []CandidateSlotFiller,
	matched []bool,
	admitted SlotFiller,
	basis AdmissionBasis,
) int {
	for index, candidate := range candidates {
		if matched[index] {
			continue
		}
		if validFillerCorrelation(
			changeOrdinal,
			fillerOrdinal,
			candidateSet,
			relation,
			slot,
			candidate,
			admitted,
			basis,
		) {
			return index
		}
	}
	return -1
}

func validFillerCorrelation(
	changeOrdinal uint64,
	fillerOrdinal uint64,
	candidateSet MemoryChangeSet,
	relation candidateRelationalCarrier,
	slot SlotKindID,
	candidate CandidateSlotFiller,
	admitted SlotFiller,
	basis AdmissionBasis,
) bool {
	switch candidateValue := candidate.(type) {
	case ByValueCandidate:
		admittedValue, ok := admitted.(ValueFiller)
		return ok &&
			candidateValue.Value().ValueKind().TypeEnv() == basis.TypeEnv() &&
			validTypedValueCorrelation(candidateValue.Value(), admittedValue.Value())
	case ByReferenceCandidate:
		admittedReference, ok := admitted.(ReferenceFiller)
		if !ok || candidateValue.Reference().RefKind().TypeEnv() != basis.TypeEnv() {
			return false
		}
		use, found := exactReferenceFillerUse(
			changeOrdinal,
			fillerOrdinal,
			relation.Assertion(),
			relation.Signature(),
			relation.Slice(),
			slot,
			admittedReference,
			basis,
		)
		if !found {
			return false
		}
		switch value := use.(type) {
		case membershipReferenceFillerCorrelation:
			return referenceResolutionMatchesCandidate(
				candidateSet,
				changeOrdinal,
				candidateValue.Reference(),
				admittedReference,
				value.use.Resolution(),
				value.use.RequiredMembership().EvaluationView(),
			)
		case classificationReferenceFillerCorrelation:
			return classificationReferenceResolutionMatchesCandidate(
				candidateSet,
				changeOrdinal,
				candidateValue.Reference(),
				admittedReference,
				value.use.Resolution(),
			)
		default:
			return false
		}
	default:
		return false
	}
}

func validTypedValueCorrelation(
	candidate TypedValueCandidate,
	admitted VerifiedTypedValue,
) bool {
	if candidate.ValueKind() != admitted.ValueKind() ||
		candidate.ValueShape() != admitted.ValueShape() ||
		candidate.Codec() != admitted.Codec() {
		return false
	}
	asserted, present := candidate.AssertedDigest().Digest()
	return !present || asserted == admitted.Digest()
}

type referenceFillerAdmissionCorrelation interface {
	referenceFillerAdmissionCorrelationVariant()
}

type membershipReferenceFillerCorrelation struct {
	use ReferenceFillerAdmissionUse
}

func (membershipReferenceFillerCorrelation) referenceFillerAdmissionCorrelationVariant() {}

type classificationReferenceFillerCorrelation struct {
	use ClassificationReferenceFillerAdmissionUse
}

func (classificationReferenceFillerCorrelation) referenceFillerAdmissionCorrelationVariant() {}

func exactReferenceFillerUse(
	changeOrdinal uint64,
	fillerOrdinal uint64,
	assertion AssertionID,
	signature RelationSignatureRef,
	slice ContextSlice,
	slot SlotKindID,
	filler ReferenceFiller,
	basis AdmissionBasis,
) (referenceFillerAdmissionCorrelation, bool) {
	switch value := basis.(type) {
	case ContextSliceMembershipBasis:
		for _, use := range value.ReferenceFillerAdmissionUses() {
			coordinate := use.Coordinate()
			query := use.RequiredMembership().Query()
			if referenceFillerCoordinateMatches(
				coordinate,
				changeOrdinal,
				fillerOrdinal,
				assertion,
				signature,
				slice,
				slot,
				filler,
			) &&
				query.EntityID() == filler.Entity() &&
				coordinate.RequiredValueKind() == query.ValueKind() &&
				sameContextSlice(query.ContextSlice(), slice) {
				return membershipReferenceFillerCorrelation{use: use}, true
			}
		}
	case ContextSliceClassificationBasis:
		for _, use := range value.ClassificationReferenceFillerAdmissionUses() {
			coordinate := use.Coordinate()
			request := use.RequiredClassification().Request()
			candidate, exactEntity := request.Candidate().(ExactKindEntityCandidate)
			if referenceFillerCoordinateMatches(
				coordinate,
				changeOrdinal,
				fillerOrdinal,
				assertion,
				signature,
				slice,
				slot,
				filler,
			) &&
				exactEntity &&
				candidate.EntityID() == filler.Entity() &&
				coordinate.RequiredValueKind() == request.LocalKind().ValueKind() &&
				sameContextSlice(request.ContextSlice(), slice) {
				return classificationReferenceFillerCorrelation{use: use}, true
			}
		}
	}
	return nil, false
}

func referenceFillerCoordinateMatches(
	coordinate RelationFillerCoordinate,
	changeOrdinal uint64,
	fillerOrdinal uint64,
	assertion AssertionID,
	signature RelationSignatureRef,
	slice ContextSlice,
	slot SlotKindID,
	filler ReferenceFiller,
) bool {
	return coordinate.ChangeOrdinal() == changeOrdinal &&
		coordinate.FillerOrdinal() == fillerOrdinal &&
		coordinate.Assertion() == assertion &&
		coordinate.Signature() == signature &&
		sameContextSlice(coordinate.ContextSlice(), slice) &&
		coordinate.Slot() == slot &&
		coordinate.Reference() == filler.Reference() &&
		coordinate.Entity() == filler.Entity()
}

func referenceResolutionMatchesCandidate(
	candidateSet MemoryChangeSet,
	evaluationChangeOrdinal uint64,
	candidate StrongRef,
	filler ReferenceFiller,
	resolution AdmissionReferenceResolution,
	evaluationView MemberOfEvaluationView,
) bool {
	if resolution.PersistedReference() != filler.Reference() ||
		resolution.Entity() != filler.Entity() {
		return false
	}
	switch value := candidate.(type) {
	case PersistedRef:
		resolved, ok := resolution.(SnapshotReferenceResolution)
		_, persistedView := evaluationView.(PersistedSnapshotView)
		return ok && persistedView && value == resolved.PersistedReference()
	case LocalRef:
		resolved, ok := resolution.(SameBatchDeclarationResolution)
		if !ok || value != resolved.LocalReference() {
			return false
		}
		declarationOrdinal := resolved.DeclarationChangeOrdinal()
		if declarationOrdinal >= evaluationChangeOrdinal ||
			evaluationChangeOrdinal >= uint64(len(candidateSet.changes)) {
			return false
		}
		declaration, ok := candidateSet.changes[declarationOrdinal].(DeclareEntity)
		view, prospective := evaluationView.(ProspectiveBatchView)
		prefix, prefixErr := ComputeOrderedCandidatePrefix(candidateSet, evaluationChangeOrdinal)
		declarationBytes, declarationErr := canonicalMemoryChange(declaration)
		return ok && prospective && prefixErr == nil && declarationErr == nil &&
			declaration.LocalRef() == value.BatchLocalRef() &&
			declaration.Entity() == resolved.Entity() &&
			declaration.Context() == resolved.Context() &&
			declaration.Label() == resolved.Declaration().Label() &&
			declaration.Provenance() == resolved.Declaration().Provenance() &&
			resolved.DeclarationDigest() == view.DeclarationDigest() &&
			bytes.Equal(declarationBytes, resolved.DeclarationCanonicalBytes()) &&
			prefix.Digest() == view.OrderedCandidatePrefix().Digest() &&
			bytes.Equal(prefix.CanonicalBytes(), view.OrderedCandidatePrefix().CanonicalBytes())
	default:
		return false
	}
}

func classificationReferenceResolutionMatchesCandidate(
	candidateSet MemoryChangeSet,
	evaluationChangeOrdinal uint64,
	candidate StrongRef,
	filler ReferenceFiller,
	resolution AdmissionReferenceResolution,
) bool {
	if resolution.PersistedReference() != filler.Reference() ||
		resolution.Entity() != filler.Entity() {
		return false
	}
	switch value := candidate.(type) {
	case PersistedRef:
		resolved, snapshot := resolution.(SnapshotReferenceResolution)
		return snapshot && value == resolved.PersistedReference()
	case LocalRef:
		resolved, sameBatch := resolution.(SameBatchDeclarationResolution)
		if !sameBatch || value != resolved.LocalReference() {
			return false
		}
		declarationOrdinal := resolved.DeclarationChangeOrdinal()
		if declarationOrdinal >= evaluationChangeOrdinal ||
			evaluationChangeOrdinal >= uint64(len(candidateSet.changes)) {
			return false
		}
		declaration, declared := candidateSet.changes[declarationOrdinal].(DeclareEntity)
		if !declared {
			return false
		}
		declarationBytes, err := declaration.CanonicalBytes()
		if err != nil {
			return false
		}
		declarationDigest, err := declaration.Digest()
		if err != nil {
			return false
		}
		return declaration.LocalRef() == value.BatchLocalRef() &&
			declaration.Entity() == resolved.Entity() &&
			declaration.Context() == resolved.Context() &&
			declaration.Label() == resolved.Declaration().Label() &&
			declaration.Provenance() == resolved.Declaration().Provenance() &&
			declarationDigest == resolved.DeclarationDigest() &&
			bytes.Equal(declarationBytes, resolved.DeclarationCanonicalBytes())
	default:
		return false
	}
}

func validObservationCorrelation(candidate MemoryChangeSet, basis AdmissionBasis) bool {
	for _, observation := range basis.SnapshotObservations() {
		ordinal := observation.ChangeOrdinal()
		if ordinal >= uint64(len(candidate.changes)) {
			return false
		}
		if !observationMatchesCandidate(
			observation,
			candidate.changes[ordinal],
			ordinal,
			basis,
		) {
			return false
		}
	}
	for ordinal, change := range candidate.changes {
		if !hasRequiredObservations(
			uint64(ordinal),
			candidate,
			change,
			basis.SnapshotObservations(),
		) {
			return false
		}
	}
	return true
}

func observationMatchesCandidate(
	observation AdmissionSnapshotObservation,
	candidate MemoryChange,
	ordinal uint64,
	basis AdmissionBasis,
) bool {
	switch value := observation.(type) {
	case EntityAbsentObservation:
		change, ok := candidate.(DeclareEntity)
		resolution := value.Resolution()
		return ok &&
			resolution.Entity() == change.Entity() &&
			resolution.Context() == change.Context()
	case EntityExactObservation:
		return entityExactObservationMatchesCandidate(value.Resolution(), candidate, ordinal, basis)
	case AliasUnboundObservation:
		return aliasUnboundObservationMatchesCandidate(value.Resolution(), candidate)
	case AliasBoundObservation:
		return aliasBoundObservationMatchesCandidate(value.Resolution(), candidate)
	case AssertionAbsentObservation:
		switch change := candidate.(type) {
		case InstantiateRelation:
			return value.State().Assertion() == change.Relation().Assertion()
		case AssertRelation:
			return value.State().Assertion() == change.Assertion().Assertion()
		default:
			return false
		}
	case AssertionActiveObservation:
		change, ok := candidate.(RetractAssertion)
		return ok && value.State().Assertion() == change.Assertion()
	default:
		return false
	}
}

func entityExactObservationMatchesCandidate(
	resolution ExactEntityResolution,
	candidate MemoryChange,
	ordinal uint64,
	basis AdmissionBasis,
) bool {
	switch change := candidate.(type) {
	case ApplyIdentityChange:
		return identityChangeNamesEntity(change.Change(), resolution.Entity(), resolution.Context())
	case InstantiateRelation:
		return admissionBasisContainsResolvedEntity(
			basis,
			ordinal,
			resolution,
			change.Relation().Signature(),
			change.Relation().Slice(),
		)
	case AssertRelation:
		return admissionBasisContainsResolvedEntity(
			basis,
			ordinal,
			resolution,
			change.Assertion().Signature(),
			change.Assertion().Slice(),
		)
	default:
		return false
	}
}

func admissionBasisContainsResolvedEntity(
	basis AdmissionBasis,
	ordinal uint64,
	resolution ExactEntityResolution,
	signature RelationSignatureRef,
	slice ContextSlice,
) bool {
	switch value := basis.(type) {
	case ContextSliceMembershipBasis:
		for _, use := range value.ReferenceFillerAdmissionUses() {
			if admissionUseNamesResolvedEntity(
				use.Coordinate(),
				use.Resolution(),
				ordinal,
				resolution,
				signature,
				slice,
			) {
				return true
			}
		}
	case ContextSliceClassificationBasis:
		for _, use := range value.ClassificationReferenceFillerAdmissionUses() {
			if admissionUseNamesResolvedEntity(
				use.Coordinate(),
				use.Resolution(),
				ordinal,
				resolution,
				signature,
				slice,
			) {
				return true
			}
		}
	}
	return false
}

func admissionUseNamesResolvedEntity(
	coordinate RelationFillerCoordinate,
	useResolution AdmissionReferenceResolution,
	ordinal uint64,
	resolution ExactEntityResolution,
	signature RelationSignatureRef,
	slice ContextSlice,
) bool {
	return coordinate.ChangeOrdinal() == ordinal &&
		coordinate.Entity() == resolution.Entity() &&
		coordinate.Signature() == signature &&
		sameContextSlice(coordinate.ContextSlice(), slice) &&
		useResolution.Context() == resolution.Context()
}

func identityChangeNamesEntity(
	change IdentityChange,
	entity EntityID,
	context BoundedContextRef,
) bool {
	switch value := change.(type) {
	case AdmitAlias:
		return value.Entity() == entity && value.Context() == context
	case SupersedeAlias:
		return value.Entity() == entity && value.Context() == context
	default:
		return false
	}
}

func aliasUnboundObservationMatchesCandidate(
	resolution UnboundAliasResolution,
	candidate MemoryChange,
) bool {
	change, ok := candidate.(ApplyIdentityChange)
	if !ok {
		return false
	}
	switch value := change.Change().(type) {
	case AdmitAlias:
		return value.Alias() == resolution.Alias() && value.Context() == resolution.Context()
	case SupersedeAlias:
		return value.Replacement() == resolution.Alias() && value.Context() == resolution.Context()
	default:
		return false
	}
}

func aliasBoundObservationMatchesCandidate(
	resolution BoundAliasResolution,
	candidate MemoryChange,
) bool {
	change, ok := candidate.(ApplyIdentityChange)
	if !ok {
		return false
	}
	value, ok := change.Change().(SupersedeAlias)
	return ok &&
		value.Entity() == resolution.Entity() &&
		value.OldAlias() == resolution.Alias() &&
		value.Context() == resolution.Context()
}

func hasRequiredObservations(
	ordinal uint64,
	candidate MemoryChangeSet,
	change MemoryChange,
	observations []AdmissionSnapshotObservation,
) bool {
	switch value := change.(type) {
	case DeclareEntity:
		return hasEntityAbsentObservation(ordinal, value, observations)
	case ApplyIdentityChange:
		return hasIdentityChangeObservations(
			ordinal,
			candidate,
			value.Change(),
			observations,
		)
	case InstantiateRelation:
		return hasAssertionAbsentObservation(ordinal, value.Relation().Assertion(), observations)
	case AssertRelation:
		return hasAssertionAbsentObservation(ordinal, value.Assertion().Assertion(), observations)
	case RetractAssertion:
		return hasAssertionActiveObservation(ordinal, value.Assertion(), observations)
	default:
		return false
	}
}

func hasEntityAbsentObservation(
	ordinal uint64,
	change DeclareEntity,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(EntityAbsentObservation)
		if ok &&
			value.ChangeOrdinal() == ordinal &&
			value.Resolution().Entity() == change.Entity() &&
			value.Resolution().Context() == change.Context() {
			return true
		}
	}
	return false
}

func hasIdentityChangeObservations(
	ordinal uint64,
	candidate MemoryChangeSet,
	change IdentityChange,
	observations []AdmissionSnapshotObservation,
) bool {
	switch value := change.(type) {
	case AdmitAlias:
		return (hasExactEntityObservation(
			ordinal,
			value.Entity(),
			value.Context(),
			observations,
		) || hasPriorEntityDeclaration(
			candidate,
			ordinal,
			value.Entity(),
			value.Context(),
		)) &&
			hasUnboundAliasObservation(ordinal, value.Alias(), value.Context(), observations)
	case SupersedeAlias:
		return hasExactEntityObservation(ordinal, value.Entity(), value.Context(), observations) &&
			hasBoundAliasObservation(ordinal, value.OldAlias(), value.Entity(), value.Context(), observations) &&
			hasUnboundAliasObservation(ordinal, value.Replacement(), value.Context(), observations)
	default:
		return false
	}
}

func hasPriorEntityDeclaration(
	candidate MemoryChangeSet,
	ordinal uint64,
	entity EntityID,
	context BoundedContextRef,
) bool {
	if ordinal > uint64(len(candidate.changes)) {
		return false
	}
	for index := uint64(0); index < ordinal; index++ {
		declaration, ok := candidate.changes[index].(DeclareEntity)
		if !ok {
			continue
		}
		if declaration.Entity() == entity &&
			declaration.Context() == context {
			return true
		}
	}
	return false
}

func hasExactEntityObservation(
	ordinal uint64,
	entity EntityID,
	context BoundedContextRef,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(EntityExactObservation)
		if ok &&
			value.ChangeOrdinal() == ordinal &&
			value.Resolution().Entity() == entity &&
			value.Resolution().Context() == context {
			return true
		}
	}
	return false
}

func hasUnboundAliasObservation(
	ordinal uint64,
	alias EntityAlias,
	context BoundedContextRef,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(AliasUnboundObservation)
		if ok &&
			value.ChangeOrdinal() == ordinal &&
			value.Resolution().Alias() == alias &&
			value.Resolution().Context() == context {
			return true
		}
	}
	return false
}

func hasBoundAliasObservation(
	ordinal uint64,
	alias EntityAlias,
	entity EntityID,
	context BoundedContextRef,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(AliasBoundObservation)
		if ok &&
			value.ChangeOrdinal() == ordinal &&
			value.Resolution().Alias() == alias &&
			value.Resolution().Entity() == entity &&
			value.Resolution().Context() == context {
			return true
		}
	}
	return false
}

func hasAssertionAbsentObservation(
	ordinal uint64,
	assertion AssertionID,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(AssertionAbsentObservation)
		if ok && value.ChangeOrdinal() == ordinal && value.State().Assertion() == assertion {
			return true
		}
	}
	return false
}

func hasAssertionActiveObservation(
	ordinal uint64,
	assertion AssertionID,
	observations []AdmissionSnapshotObservation,
) bool {
	for _, observation := range observations {
		value, ok := observation.(AssertionActiveObservation)
		if ok && value.ChangeOrdinal() == ordinal && value.State().Assertion() == assertion {
			return true
		}
	}
	return false
}

func validReferenceUseClosure(
	changeSet ValidatedMemoryChangeSet,
	basis AdmissionBasis,
) bool {
	expected := 0
	for ordinal, change := range changeSet.changes {
		var relation validatedRelationalCarrier
		switch value := change.(type) {
		case ValidatedRelationInstance:
			relation = value.Relation()
		case ValidatedRelationalAssertion:
			relation = value.Assertion()
		default:
			continue
		}
		for _, binding := range relation.Bindings() {
			for fillerOrdinal, filler := range binding.Fillers() {
				reference, ok := filler.(ReferenceFiller)
				if !ok {
					continue
				}
				expected++
				_, found := exactReferenceFillerUse(
					uint64(ordinal),
					uint64(fillerOrdinal),
					relation.Assertion(),
					relation.Signature(),
					relation.Slice(),
					binding.Name(),
					reference,
					basis,
				)
				if !found {
					return false
				}
			}
		}
	}
	if expected == 0 {
		_, hasMembership := basis.(ContextSliceMembershipBasis)
		_, hasClassification := basis.(ContextSliceClassificationBasis)
		return !hasMembership && !hasClassification
	}
	switch value := basis.(type) {
	case ContextSliceMembershipBasis:
		return len(value.ReferenceFillerAdmissionUses()) == expected
	case ContextSliceClassificationBasis:
		return len(value.ClassificationReferenceFillerAdmissionUses()) == expected
	default:
		return false
	}
}

type Invalid interface {
	ValidationVerdict
	Diagnostics() []Diagnostic
	invalidVerdictVariant()
}

type invalidVerdict struct {
	diagnostics []Diagnostic
}

func newInvalidVerdict(diagnostics []Diagnostic) (Invalid, error) {
	if !hasDiagnosticPosture(diagnostics, DiagnosticInvalid) {
		return nil, fmt.Errorf("Invalid verdict requires a known-contradiction diagnostic")
	}
	if !allDiagnosticsValid(diagnostics) {
		return nil, fmt.Errorf("Invalid verdict contains an invalid diagnostic")
	}
	return invalidVerdict{diagnostics: copyDiagnostics(diagnostics)}, nil
}

func (invalidVerdict) Kind() ValidationVerdictKind { return ValidationInvalid }

func (verdict invalidVerdict) Diagnostics() []Diagnostic {
	return copyDiagnostics(verdict.diagnostics)
}

func (invalidVerdict) validationVerdictVariant() {}

func (invalidVerdict) invalidVerdictVariant() {}

type Underdetermined interface {
	ValidationVerdict
	Diagnostics() []Diagnostic
	underdeterminedVerdictVariant()
}

type underdeterminedVerdict struct {
	diagnostics []Diagnostic
}

func newUnderdeterminedVerdict(diagnostics []Diagnostic) (Underdetermined, error) {
	if len(diagnostics) == 0 {
		return nil, fmt.Errorf("Underdetermined verdict requires missing-basis diagnostics")
	}
	if !allDiagnosticsHavePosture(diagnostics, DiagnosticUnderdetermined) {
		return nil, fmt.Errorf("Underdetermined verdict may contain only missing-basis diagnostics")
	}
	if !allDiagnosticsValid(diagnostics) {
		return nil, fmt.Errorf("Underdetermined verdict contains an invalid diagnostic")
	}
	return underdeterminedVerdict{diagnostics: copyDiagnostics(diagnostics)}, nil
}

func (underdeterminedVerdict) Kind() ValidationVerdictKind { return ValidationUnderdetermined }

func (verdict underdeterminedVerdict) Diagnostics() []Diagnostic {
	return copyDiagnostics(verdict.diagnostics)
}

func (underdeterminedVerdict) validationVerdictVariant() {}

func (underdeterminedVerdict) underdeterminedVerdictVariant() {}

func hasDiagnosticPosture(diagnostics []Diagnostic, posture DiagnosticPosture) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Posture() == posture {
			return true
		}
	}
	return false
}

func allDiagnosticsHavePosture(diagnostics []Diagnostic, posture DiagnosticPosture) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Posture() != posture {
			return false
		}
	}
	return true
}

func allDiagnosticsValid(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if !diagnostic.valid() {
			return false
		}
	}
	return true
}
