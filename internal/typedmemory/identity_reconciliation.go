package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	DiagnosticReconciliationBasisUnresolved DiagnosticCode = "reconciliation_basis_unresolved"
	DiagnosticReconciliationBasisMismatch   DiagnosticCode = "reconciliation_basis_mismatch"
)

// IdentityReconciliationOperation names the semantic identity effect whose
// reviewed basis was resolved. It does not grant authority to perform that
// effect; authority remains outside pure typed-memory validation.
type IdentityReconciliationOperation string

const (
	ReconciliationMergeEntities IdentityReconciliationOperation = "merge_entities"
	ReconciliationSplitEntity   IdentityReconciliationOperation = "split_entity"
)

func (operation IdentityReconciliationOperation) valid() bool {
	switch operation {
	case ReconciliationMergeEntities, ReconciliationSplitEntity:
		return true
	default:
		return false
	}
}

// ReconciliationBasisResolution is a sealed snapshot result. A syntactically
// valid ReconciliationBasisRef is never semantic evidence by itself.
type ReconciliationBasisResolution interface {
	Basis() ReconciliationBasisRef
	Context() BoundedContextRef
	reconciliationBasisResolutionVariant()
}

// ResolvedReconciliationBasis is the immutable semantic basis correlated to
// one exact identity operation at one graph and TypeEnv revision.
type ResolvedReconciliationBasis struct {
	basis         ReconciliationBasisRef
	operation     IdentityReconciliationOperation
	context       BoundedContextRef
	primary       EntityID
	related       []EntityID
	graphRevision GraphRevision
	typeEnv       TypeEnvRef
	payloadDigest SHA256Digest
	provenance    ProvenanceRef
}

func NewResolvedReconciliationBasis(
	basis ReconciliationBasisRef,
	operation IdentityReconciliationOperation,
	context BoundedContextRef,
	primary EntityID,
	related []EntityID,
	graphRevision GraphRevision,
	typeEnv TypeEnvRef,
	payloadDigest SHA256Digest,
	provenance ProvenanceRef,
) (ResolvedReconciliationBasis, error) {
	if !basis.valid() ||
		!operation.valid() ||
		!context.valid() ||
		!primary.valid() ||
		!typeEnv.valid() ||
		!payloadDigest.valid() ||
		!provenance.valid() {
		return ResolvedReconciliationBasis{}, fmt.Errorf(
			"resolved reconciliation basis requires basis, operation, context, primary entity, TypeEnv, payload digest, and provenance",
		)
	}
	participants, err := normalizeDistinctEntities(related, primary)
	if err != nil {
		return ResolvedReconciliationBasis{}, err
	}
	if err := validateReconciliationCardinality(operation, participants); err != nil {
		return ResolvedReconciliationBasis{}, err
	}
	return ResolvedReconciliationBasis{
		basis:         basis,
		operation:     operation,
		context:       context,
		primary:       primary,
		related:       participants,
		graphRevision: graphRevision,
		typeEnv:       typeEnv,
		payloadDigest: payloadDigest,
		provenance:    provenance,
	}, nil
}

func (resolution ResolvedReconciliationBasis) Basis() ReconciliationBasisRef {
	return resolution.basis
}

func (resolution ResolvedReconciliationBasis) Operation() IdentityReconciliationOperation {
	return resolution.operation
}

func (resolution ResolvedReconciliationBasis) Context() BoundedContextRef {
	return resolution.context
}

func (resolution ResolvedReconciliationBasis) Primary() EntityID { return resolution.primary }

func (resolution ResolvedReconciliationBasis) Related() []EntityID {
	return append([]EntityID(nil), resolution.related...)
}

func (resolution ResolvedReconciliationBasis) GraphRevision() GraphRevision {
	return resolution.graphRevision
}

func (resolution ResolvedReconciliationBasis) TypeEnvRef() TypeEnvRef {
	return resolution.typeEnv
}

func (resolution ResolvedReconciliationBasis) PayloadDigest() SHA256Digest {
	return resolution.payloadDigest
}

func (resolution ResolvedReconciliationBasis) Provenance() ProvenanceRef {
	return resolution.provenance
}

// CanonicalBytes is the complete immutable reviewed basis carrier. The
// referenced review payload remains a distinct carrier identified by
// PayloadDigest; these bytes bind its digest and provenance to one exact
// operation, participant set, context, graph revision, and TypeEnv.
func (resolution ResolvedReconciliationBasis) CanonicalBytes() []byte {
	if !resolution.valid() {
		return nil
	}
	writer := canonicalResolvedReconciliationBasis(resolution)
	return writer.bytes()
}

func (resolution ResolvedReconciliationBasis) Digest() SHA256Digest {
	if !resolution.valid() {
		return SHA256Digest{}
	}
	writer := canonicalResolvedReconciliationBasis(resolution)
	return writer.digest()
}

func (ResolvedReconciliationBasis) reconciliationBasisResolutionVariant() {}

func (resolution ResolvedReconciliationBasis) valid() bool {
	if !resolution.basis.valid() ||
		!resolution.operation.valid() ||
		!resolution.context.valid() ||
		!resolution.primary.valid() ||
		!resolution.typeEnv.valid() ||
		!resolution.payloadDigest.valid() ||
		!resolution.provenance.valid() {
		return false
	}
	normalized, err := normalizeDistinctEntities(resolution.related, resolution.primary)
	if err != nil || !exactEntitySequence(normalized, resolution.related) {
		return false
	}
	return validateReconciliationCardinality(resolution.operation, normalized) == nil
}

func canonicalResolvedReconciliationBasis(
	resolution ResolvedReconciliationBasis,
) canonicalWriter {
	writer := newCanonicalWriter("resolved-identity-reconciliation-basis.v1")
	writer.addString(resolution.basis.String())
	writer.addString(string(resolution.operation))
	writer.addString(resolution.context.String())
	writer.addString(resolution.primary.String())
	for _, entity := range resolution.related {
		writer.addString(entity.String())
	}
	writer.addUint64(resolution.graphRevision.Value())
	writer.addString(resolution.typeEnv.String())
	writer.addString(resolution.payloadDigest.String())
	writer.addString(resolution.provenance.String())
	return writer
}

// ReviewedIdentityReconciliationAdmission is a sealed, pure admission for one
// merge or split. It proves only that an exact reviewed basis correlates with
// the requested identity effect. It does not mutate project memory and cannot
// be produced from a score, candidate rank, alias similarity, or an arbitrary
// ReconciliationBasisRef.
type ReviewedIdentityReconciliationAdmission struct {
	change    IdentityChange
	basis     ResolvedReconciliationBasis
	canonical []byte
	digest    SHA256Digest
}

func NewReviewedIdentityReconciliationAdmission(
	change IdentityChange,
	basis ResolvedReconciliationBasis,
) (ReviewedIdentityReconciliationAdmission, error) {
	if !basis.valid() {
		return ReviewedIdentityReconciliationAdmission{}, fmt.Errorf(
			"reviewed identity reconciliation requires an exact resolved basis",
		)
	}
	if err := requireReviewedIdentityChangeCorrelation(change, basis); err != nil {
		return ReviewedIdentityReconciliationAdmission{}, err
	}
	writer := canonicalReviewedIdentityReconciliation(change, basis)
	return ReviewedIdentityReconciliationAdmission{
		change:    change,
		basis:     basis,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (admission ReviewedIdentityReconciliationAdmission) Change() IdentityChange {
	return admission.change
}

func (admission ReviewedIdentityReconciliationAdmission) Basis() ResolvedReconciliationBasis {
	return admission.basis
}

func (admission ReviewedIdentityReconciliationAdmission) CanonicalBytes() []byte {
	if !admission.valid() {
		return nil
	}
	return append([]byte(nil), admission.canonical...)
}

func (admission ReviewedIdentityReconciliationAdmission) Digest() SHA256Digest {
	if !admission.valid() {
		return SHA256Digest{}
	}
	return admission.digest
}

func (admission ReviewedIdentityReconciliationAdmission) valid() bool {
	if !admission.basis.valid() || !validIdentityChangeVariant(admission.change) {
		return false
	}
	if requireReviewedIdentityChangeCorrelation(admission.change, admission.basis) != nil {
		return false
	}
	writer := canonicalReviewedIdentityReconciliation(admission.change, admission.basis)
	return canonicalValueMatches(writer, admission.canonical, admission.digest)
}

func requireReviewedIdentityChangeCorrelation(
	change IdentityChange,
	basis ResolvedReconciliationBasis,
) error {
	switch value := change.(type) {
	case MergeEntities:
		matches := basis.operation == ReconciliationMergeEntities &&
			basis.basis == value.basis &&
			basis.context == value.context &&
			basis.primary == value.survivor &&
			exactEntitySequence(basis.related, value.merged)
		if !matches {
			return fmt.Errorf("reviewed merge basis does not match the exact merge effect")
		}
	case SplitEntity:
		matches := basis.operation == ReconciliationSplitEntity &&
			basis.basis == value.basis &&
			basis.context == value.context &&
			basis.primary == value.source &&
			exactEntitySequence(basis.related, value.targets)
		if !matches {
			return fmt.Errorf("reviewed split basis does not match the exact split effect")
		}
	default:
		return fmt.Errorf("reviewed identity reconciliation admits only merge or split")
	}
	return nil
}

func canonicalReviewedIdentityReconciliation(
	change IdentityChange,
	basis ResolvedReconciliationBasis,
) canonicalWriter {
	changeBytes, _ := canonicalIdentityChange(change)
	writer := newCanonicalWriter("reviewed-identity-reconciliation-admission.v1")
	writer.addBytes(changeBytes)
	writer.addBytes(basis.CanonicalBytes())
	return writer
}

// VerifyStoredReviewedIdentityReconciliation verifies the durable carrier
// without granting a caller the ability to construct a sealed admission from
// stored bytes. Storage readers use it to detect corruption before replay.
func VerifyStoredReviewedIdentityReconciliation(
	change IdentityChange,
	basis ResolvedReconciliationBasis,
	canonical []byte,
	digest SHA256Digest,
) error {
	admission, err := NewReviewedIdentityReconciliationAdmission(change, basis)
	if err != nil {
		return err
	}
	if admission.digest != digest || !bytes.Equal(admission.canonical, canonical) {
		return fmt.Errorf("stored reviewed identity reconciliation carrier is not canonical")
	}
	return nil
}

// MissingReconciliationBasis reports that the exact basis was not available
// in the immutable snapshot. It is missing knowledge, not negative evidence.
type MissingReconciliationBasis struct {
	basis   ReconciliationBasisRef
	context BoundedContextRef
}

func NewMissingReconciliationBasis(
	basis ReconciliationBasisRef,
	context BoundedContextRef,
) (MissingReconciliationBasis, error) {
	if !basis.valid() || !context.valid() {
		return MissingReconciliationBasis{}, fmt.Errorf(
			"missing reconciliation basis requires the queried basis and context",
		)
	}
	return MissingReconciliationBasis{basis: basis, context: context}, nil
}

func (resolution MissingReconciliationBasis) Basis() ReconciliationBasisRef {
	return resolution.basis
}

func (resolution MissingReconciliationBasis) Context() BoundedContextRef {
	return resolution.context
}

func (MissingReconciliationBasis) reconciliationBasisResolutionVariant() {}

// ConflictingReconciliationBasis reports multiple immutable payloads for the
// same queried basis. The digests are evidence of contradiction, not a score.
type ConflictingReconciliationBasis struct {
	basis          ReconciliationBasisRef
	context        BoundedContextRef
	payloadDigests []SHA256Digest
}

func NewConflictingReconciliationBasis(
	basis ReconciliationBasisRef,
	context BoundedContextRef,
	payloadDigests []SHA256Digest,
) (ConflictingReconciliationBasis, error) {
	if !basis.valid() || !context.valid() {
		return ConflictingReconciliationBasis{}, fmt.Errorf(
			"conflicting reconciliation basis requires the queried basis and context",
		)
	}
	digests, err := normalizeDistinctDigests(payloadDigests)
	if err != nil {
		return ConflictingReconciliationBasis{}, err
	}
	if len(digests) < 2 {
		return ConflictingReconciliationBasis{}, fmt.Errorf(
			"conflicting reconciliation basis requires at least two payload digests",
		)
	}
	return ConflictingReconciliationBasis{
		basis:          basis,
		context:        context,
		payloadDigests: digests,
	}, nil
}

func (resolution ConflictingReconciliationBasis) Basis() ReconciliationBasisRef {
	return resolution.basis
}

func (resolution ConflictingReconciliationBasis) Context() BoundedContextRef {
	return resolution.context
}

func (resolution ConflictingReconciliationBasis) PayloadDigests() []SHA256Digest {
	return append([]SHA256Digest(nil), resolution.payloadDigests...)
}

func (ConflictingReconciliationBasis) reconciliationBasisResolutionVariant() {}

func validateReconciliationCardinality(
	operation IdentityReconciliationOperation,
	related []EntityID,
) error {
	switch operation {
	case ReconciliationMergeEntities:
		if len(related) < 1 {
			return fmt.Errorf("merge reconciliation basis requires at least one merged entity")
		}
	case ReconciliationSplitEntity:
		if len(related) < 2 {
			return fmt.Errorf("split reconciliation basis requires at least two target entities")
		}
	default:
		return fmt.Errorf("unknown reconciliation operation %q", operation)
	}
	return nil
}

func normalizeDistinctDigests(values []SHA256Digest) ([]SHA256Digest, error) {
	result := append([]SHA256Digest(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	for index, digest := range result {
		if !digest.valid() {
			return nil, fmt.Errorf("reconciliation conflict contains an invalid payload digest")
		}
		if index > 0 && digest == result[index-1] {
			return nil, fmt.Errorf("reconciliation conflict repeats payload digest %q", digest.String())
		}
	}
	return result, nil
}
