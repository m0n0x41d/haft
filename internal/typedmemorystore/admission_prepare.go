package typedmemorystore

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type preparedAdmission struct {
	batch                typedmemory.AdmissionBatch
	basis                typedmemory.AdmissionBasis
	candidate            typedmemory.MemoryChangeSet
	requestBytes         []byte
	requestDigest        typedmemory.SHA256Digest
	semanticBytes        []byte
	semanticDigest       typedmemory.SHA256Digest
	envelopeBytes        []byte
	envelopeDigest       typedmemory.SHA256Digest
	requestProvenanceRef string
	eventKind            string
	changes              []preparedAdmissionChange
}

type preparedAdmissionChange struct {
	ordinal uint64
	change  typedmemory.ValidatedMemoryChange
}

func prepareGenericAdmission(request CommitRequest) (preparedAdmission, error) {
	batch := request.admissionBatch
	if !batch.IsValid() {
		return preparedAdmission{}, ErrAdmissionBatchRequired
	}
	basis := batch.Basis()
	if basis == nil {
		return preparedAdmission{}, ErrInvalidAdmissionBatch
	}
	if basis.TypeEnv() != request.expectedTypeEnv ||
		basis.GraphRevision() != request.expectedRevision {
		return preparedAdmission{}, ErrAdmissionBasisMismatch
	}
	if basis.Kind() == typedmemory.ContextSliceClassificationAdmissionBasis &&
		!request.ContractVersion().IsV2() {
		return preparedAdmission{}, fmt.Errorf(
			"%w: current kind classification requires admission contract v2",
			ErrUnsupportedBatch,
		)
	}
	requestBytes, err := request.candidate.CanonicalBytes()
	if err != nil {
		return preparedAdmission{}, fmt.Errorf("canonicalize typed-memory request: %w", err)
	}
	requestDigest, err := request.candidate.Digest()
	if err != nil {
		return preparedAdmission{}, fmt.Errorf("digest typed-memory request: %w", err)
	}
	if requestDigest != batch.RequestDigest() {
		return preparedAdmission{}, ErrRequestDigestMismatch
	}
	semantic := batch.ChangeSet()
	semanticBytes, err := semantic.CanonicalBytes()
	if err != nil {
		return preparedAdmission{}, fmt.Errorf("canonicalize admitted typed-memory changes: %w", err)
	}
	semanticDigest, err := semantic.CanonicalDigest()
	if err != nil {
		return preparedAdmission{}, fmt.Errorf("digest admitted typed-memory changes: %w", err)
	}
	if semanticDigest != batch.SemanticChangeDigest() {
		return preparedAdmission{}, ErrInvalidAdmissionBatch
	}
	if err := validateAdmissionContractFamilies(
		request.ContractVersion(),
		request.candidate.Changes(),
		semantic.Changes(),
	); err != nil {
		return preparedAdmission{}, err
	}
	changes, eventKind, err := prepareAdmissionChanges(semantic.Changes())
	if err != nil {
		return preparedAdmission{}, err
	}
	envelopeDigest := batch.AdmissionEnvelopeDigest()
	requestProvenance := request.requestProvenance.String()
	if requestProvenance == "" {
		return preparedAdmission{}, fmt.Errorf("typed-memory request provenance is required")
	}
	return preparedAdmission{
		batch:                batch,
		basis:                basis,
		candidate:            request.candidate,
		requestBytes:         append([]byte(nil), requestBytes...),
		requestDigest:        requestDigest,
		semanticBytes:        append([]byte(nil), semanticBytes...),
		semanticDigest:       semanticDigest,
		envelopeBytes:        batch.CanonicalEnvelopeBytes(),
		envelopeDigest:       envelopeDigest,
		requestProvenanceRef: requestProvenance,
		eventKind:            eventKind,
		changes:              changes,
	}, nil
}

func validateAdmissionContractFamilies(
	version AdmissionContractVersion,
	candidates []typedmemory.MemoryChange,
	validated []typedmemory.ValidatedMemoryChange,
) error {
	if len(candidates) != len(validated) {
		return ErrInvalidAdmissionBatch
	}
	for index := range candidates {
		candidate := candidates[index]
		admitted := validated[index]
		if err := validateAdmissionContractFamily(version, candidate, admitted); err != nil {
			return fmt.Errorf("admission change %d: %w", index, err)
		}
	}
	return nil
}

func validateAdmissionContractFamily(
	version AdmissionContractVersion,
	candidate typedmemory.MemoryChange,
	validated typedmemory.ValidatedMemoryChange,
) error {
	if version.IsV1() {
		if _, assertion := candidate.(typedmemory.AssertRelation); assertion {
			return fmt.Errorf("%w: v1 cannot carry AssertRelation", ErrUnsupportedBatch)
		}
		if _, assertion := validated.(typedmemory.ValidatedRelationalAssertion); assertion {
			return fmt.Errorf("%w: v1 cannot admit a relational assertion", ErrUnsupportedBatch)
		}
		return nil
	}
	if version.IsV2() {
		if _, legacy := candidate.(typedmemory.InstantiateRelation); legacy {
			return fmt.Errorf("%w: v2 cannot carry legacy InstantiateRelation", ErrUnsupportedBatch)
		}
		if _, legacy := validated.(typedmemory.ValidatedRelationInstance); legacy {
			return fmt.Errorf("%w: v2 cannot admit a legacy RelationInstance", ErrUnsupportedBatch)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown admission contract version", ErrUnsupportedBatch)
}

func prepareAdmissionChanges(
	changes []typedmemory.ValidatedMemoryChange,
) ([]preparedAdmissionChange, string, error) {
	prepared := make([]preparedAdmissionChange, 0, len(changes))
	kinds := make([]string, 0, len(changes))
	for index, change := range changes {
		kind, err := classifyAdmittedChange(change)
		if err != nil {
			return nil, "", err
		}
		prepared = append(prepared, preparedAdmissionChange{
			ordinal: uint64(index),
			change:  change,
		})
		kinds = append(kinds, kind)
	}
	if len(prepared) == 0 {
		return nil, "", ErrInvalidAdmissionBatch
	}
	eventKind := "mixed_change_set"
	if len(kinds) == 1 {
		eventKind = kinds[0]
	}
	return prepared, eventKind, nil
}

func classifyAdmittedChange(
	change typedmemory.ValidatedMemoryChange,
) (string, error) {
	switch value := change.(type) {
	case typedmemory.ValidatedDeclareEntity:
		return "declare_entity", nil
	case typedmemory.ValidatedIdentityChange:
		return classifyAdmittedIdentityChange(value.Change())
	case typedmemory.ValidatedRelationInstance:
		return "instantiate_relation", nil
	case typedmemory.ValidatedRelationalAssertion:
		return "assert_relation", nil
	case typedmemory.ValidatedRetraction:
		return "retract_assertion", nil
	default:
		return "", fmt.Errorf("%w: unsupported admitted change %T", ErrUnsupportedBatch, change)
	}
}

func classifyAdmittedIdentityChange(
	change typedmemory.IdentityChange,
) (string, error) {
	switch change.(type) {
	case typedmemory.AdmitAlias:
		return "admit_alias", nil
	case typedmemory.SupersedeAlias:
		return "supersede_alias", nil
	case typedmemory.MergeEntities, typedmemory.SplitEntity:
		return "", ErrManualIdentityReconciliationRequired
	default:
		return "", fmt.Errorf("%w: unsupported identity change %T", ErrUnsupportedBatch, change)
	}
}
