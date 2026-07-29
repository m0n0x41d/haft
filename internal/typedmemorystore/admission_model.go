package typedmemorystore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type ObservableInputBlob = memberofevaluation.ObservableInputBlob

func NewObservableInputBlob(
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
	content []byte,
) (ObservableInputBlob, error) {
	return memberofevaluation.NewObservableInputBlob(reference, digest, content)
}

type MemberOfEvaluationInput = memberofevaluation.MemberOfEvaluationInput

func newMemberOfEvaluationInput(
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
	request typedmemory.MemberOfEvaluationRequest,
	observables []ObservableInputBlob,
	universe PersistedEntityUniverse,
) (MemberOfEvaluationInput, error) {
	return memberofevaluation.NewMemberOfEvaluationInput(
		project,
		environment,
		request,
		observables,
		universe,
	)
}

type MemberOfEvaluationEngine = memberofevaluation.MemberOfEvaluationEngine
type SnapshotObservableInputSelector = memberofevaluation.SnapshotObservableInputSelector
type SnapshotObservableInputSelection = memberofevaluation.SnapshotObservableInputSelection
type SnapshotObservableInputsSelected = memberofevaluation.SnapshotObservableInputsSelected
type SnapshotObservableInputsNotApplicable = memberofevaluation.SnapshotObservableInputsNotApplicable
type SnapshotObservableInputsUnavailable = memberofevaluation.SnapshotObservableInputsUnavailable

// KindClassificationVisibility is the delivery-side view used to obtain
// governed candidate features. It is not part of the four-input C.3.2
// classification request and cannot create a classification result itself.
type KindClassificationVisibility interface {
	GraphRevision() typedmemory.GraphRevision
	kindClassificationVisibilityVariant()
}

type SnapshotKindClassificationVisibility struct {
	revision typedmemory.GraphRevision
	entity   typedmemory.EntityID
	context  typedmemory.BoundedContextRef
	basis    typedmemory.ResolutionBasisRef
}

func NewSnapshotKindClassificationVisibility(
	revision typedmemory.GraphRevision,
	entity typedmemory.EntityID,
	context typedmemory.BoundedContextRef,
	basis typedmemory.ResolutionBasisRef,
) (SnapshotKindClassificationVisibility, error) {
	resolution, err := typedmemory.NewExactEntityResolution(entity, context, basis)
	if err != nil {
		return SnapshotKindClassificationVisibility{}, fmt.Errorf(
			"snapshot classification visibility requires exact entity presence: %w",
			err,
		)
	}
	return SnapshotKindClassificationVisibility{
		revision: revision,
		entity:   resolution.Entity(),
		context:  resolution.Context(),
		basis:    resolution.Basis(),
	}, nil
}

func (visibility SnapshotKindClassificationVisibility) GraphRevision() typedmemory.GraphRevision {
	return visibility.revision
}

func (visibility SnapshotKindClassificationVisibility) Entity() typedmemory.EntityID {
	return visibility.entity
}

func (visibility SnapshotKindClassificationVisibility) Context() typedmemory.BoundedContextRef {
	return visibility.context
}

func (visibility SnapshotKindClassificationVisibility) ResolutionBasis() typedmemory.ResolutionBasisRef {
	return visibility.basis
}

func (SnapshotKindClassificationVisibility) kindClassificationVisibilityVariant() {}

type ProspectiveKindClassificationVisibility struct {
	revision                 typedmemory.GraphRevision
	evaluationChangeOrdinal  uint64
	declarationChangeOrdinal uint64
	declaration              typedmemory.DeclareEntity
	localReference           typedmemory.LocalRef
	persistedReference       typedmemory.PersistedRef
	orderedCandidatePrefix   []typedmemory.MemoryChange
}

func newProspectiveKindClassificationVisibility(
	candidate typedmemory.MemoryChangeSet,
	revision typedmemory.GraphRevision,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.SameBatchDeclarationResolution,
) (ProspectiveKindClassificationVisibility, error) {
	changes := candidate.Changes()
	declarationOrdinal := resolution.DeclarationChangeOrdinal()
	if declarationOrdinal >= evaluationChangeOrdinal ||
		evaluationChangeOrdinal >= uint64(len(changes)) {
		return ProspectiveKindClassificationVisibility{}, fmt.Errorf(
			"prospective classification declaration must precede relation evaluation",
		)
	}
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(
		candidate,
		evaluationChangeOrdinal,
	)
	if err != nil {
		return ProspectiveKindClassificationVisibility{}, err
	}
	return newProspectiveKindClassificationVisibilityFromPrefix(
		prefix,
		revision,
		evaluationChangeOrdinal,
		resolution,
	)
}

func newProspectiveKindClassificationVisibilityFromPrefix(
	prefix typedmemory.OrderedCandidatePrefix,
	revision typedmemory.GraphRevision,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.SameBatchDeclarationResolution,
) (ProspectiveKindClassificationVisibility, error) {
	changes := prefix.Changes()
	declarationOrdinal := resolution.DeclarationChangeOrdinal()
	if declarationOrdinal >= evaluationChangeOrdinal ||
		evaluationChangeOrdinal != uint64(len(changes)) {
		return ProspectiveKindClassificationVisibility{}, fmt.Errorf(
			"prospective classification prefix must end at relation evaluation",
		)
	}
	declaration, declared := changes[declarationOrdinal].(typedmemory.DeclareEntity)
	if !declared {
		return ProspectiveKindClassificationVisibility{}, fmt.Errorf(
			"prospective classification declaration ordinal does not contain DeclareEntity",
		)
	}
	declarationBytes, err := declaration.CanonicalBytes()
	if err != nil {
		return ProspectiveKindClassificationVisibility{}, err
	}
	resolutionBytes := resolution.DeclarationCanonicalBytes()
	if !bytes.Equal(declarationBytes, resolutionBytes) ||
		declaration.LocalRef() != resolution.LocalReference().BatchLocalRef() ||
		declaration.Entity() != resolution.Entity() ||
		declaration.Context() != resolution.Context() {
		return ProspectiveKindClassificationVisibility{}, fmt.Errorf(
			"prospective classification resolution differs from the exact candidate declaration",
		)
	}
	return ProspectiveKindClassificationVisibility{
		revision:                 revision,
		evaluationChangeOrdinal:  evaluationChangeOrdinal,
		declarationChangeOrdinal: declarationOrdinal,
		declaration:              declaration,
		localReference:           resolution.LocalReference(),
		persistedReference:       resolution.PersistedReference(),
		orderedCandidatePrefix:   append([]typedmemory.MemoryChange(nil), changes[:evaluationChangeOrdinal]...),
	}, nil
}

func (visibility ProspectiveKindClassificationVisibility) GraphRevision() typedmemory.GraphRevision {
	return visibility.revision
}

func (visibility ProspectiveKindClassificationVisibility) EvaluationChangeOrdinal() uint64 {
	return visibility.evaluationChangeOrdinal
}

func (visibility ProspectiveKindClassificationVisibility) DeclarationChangeOrdinal() uint64 {
	return visibility.declarationChangeOrdinal
}

func (visibility ProspectiveKindClassificationVisibility) Declaration() typedmemory.DeclareEntity {
	return visibility.declaration
}

func (visibility ProspectiveKindClassificationVisibility) LocalReference() typedmemory.LocalRef {
	return visibility.localReference
}

func (visibility ProspectiveKindClassificationVisibility) PersistedReference() typedmemory.PersistedRef {
	return visibility.persistedReference
}

func (visibility ProspectiveKindClassificationVisibility) OrderedCandidatePrefix() []typedmemory.MemoryChange {
	return append([]typedmemory.MemoryChange(nil), visibility.orderedCandidatePrefix...)
}

func (ProspectiveKindClassificationVisibility) kindClassificationVisibilityVariant() {}

type KindClassificationAdmissionInput struct {
	project     projectledger.ProjectID
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
	request     typedmemory.KindClassificationRequest
	visibility  KindClassificationVisibility
	sources     []KindClassificationSourceBlob
}

func NewKindClassificationAdmissionInput(
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	request typedmemory.KindClassificationRequest,
	visibility KindClassificationVisibility,
	sources []KindClassificationSourceBlob,
) (KindClassificationAdmissionInput, error) {
	parsedProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return KindClassificationAdmissionInput{}, fmt.Errorf(
			"kind-classification admission project is invalid",
		)
	}
	parsedTypeEnv, err := typedmemory.ParseTypeEnvRef(environment.Ref().String())
	if err != nil || parsedTypeEnv != environment.Ref() ||
		!request.Valid() || request.LocalKind().TypeEnv() != environment.Ref() {
		return KindClassificationAdmissionInput{}, fmt.Errorf(
			"kind-classification admission requires an exact environment and request",
		)
	}
	signature, found := environment.KindClassificationSignatureDefinition(
		request.LocalKind(),
	)
	if !found || signature.Ref() != request.SignatureEdition() {
		return KindClassificationAdmissionInput{}, fmt.Errorf(
			"kind-classification admission request has no exact current KindSignature",
		)
	}
	if codecs.Len() == 0 {
		return KindClassificationAdmissionInput{}, fmt.Errorf(
			"kind-classification admission requires an exact codec registry",
		)
	}
	if !validKindClassificationVisibility(request, visibility) {
		return KindClassificationAdmissionInput{}, fmt.Errorf(
			"kind-classification admission visibility is invalid or uncorrelated",
		)
	}
	normalizedSources, err := normalizeKindClassificationSourceBlobs(sources)
	if err != nil {
		return KindClassificationAdmissionInput{}, err
	}
	return KindClassificationAdmissionInput{
		project:     project,
		environment: environment,
		codecs:      codecs,
		request:     request,
		visibility:  visibility,
		sources:     normalizedSources,
	}, nil
}

func (input KindClassificationAdmissionInput) ProjectID() projectledger.ProjectID {
	return input.project
}

func (input KindClassificationAdmissionInput) Environment() typedmemory.TypeEnv {
	return input.environment
}

func (input KindClassificationAdmissionInput) Codecs() typedmemory.CodecRegistry {
	return input.codecs
}

func (input KindClassificationAdmissionInput) Request() typedmemory.KindClassificationRequest {
	return input.request
}

func (input KindClassificationAdmissionInput) Visibility() KindClassificationVisibility {
	return input.visibility
}

func (input KindClassificationAdmissionInput) Sources() []KindClassificationSourceBlob {
	return append([]KindClassificationSourceBlob(nil), input.sources...)
}

func validKindClassificationVisibility(
	request typedmemory.KindClassificationRequest,
	visibility KindClassificationVisibility,
) bool {
	switch value := visibility.(type) {
	case SnapshotKindClassificationVisibility:
		candidate, exact := request.Candidate().(typedmemory.ExactKindEntityCandidate)
		return exact &&
			value.entity == candidate.EntityID() &&
			value.context == request.ContextSlice().Context() &&
			value.basis.String() != ""
	case ProspectiveKindClassificationVisibility:
		candidate, exact := request.Candidate().(typedmemory.ExactKindEntityCandidate)
		evaluationOrdinal, ordinalExact := sliceIndexFromUint64(
			value.evaluationChangeOrdinal,
		)
		return exact &&
			ordinalExact &&
			value.evaluationChangeOrdinal > value.declarationChangeOrdinal &&
			value.declaration.Entity() == candidate.EntityID() &&
			value.declaration.Context() == request.ContextSlice().Context() &&
			value.localReference.BatchLocalRef() == value.declaration.LocalRef() &&
			value.persistedReference.ReferenceID().String() == candidate.EntityID().String() &&
			len(value.orderedCandidatePrefix) == evaluationOrdinal
	default:
		return false
	}
}

// KindClassificationVisibilitySourceCoordinate deterministically identifies
// the exact entity-presence delivery basis used to derive current governed
// features. It is not Evidence and is not an external source blob.
func KindClassificationVisibilitySourceCoordinate(
	visibility KindClassificationVisibility,
) (typedmemory.CarrierRef, typedmemory.SHA256Digest, bool) {
	fields := []string(nil)
	switch value := visibility.(type) {
	case SnapshotKindClassificationVisibility:
		fields = []string{
			"snapshot",
			fmt.Sprintf("%d", value.revision.Value()),
			value.entity.String(),
			value.context.String(),
			value.basis.String(),
		}
	case ProspectiveKindClassificationVisibility:
		declarationDigest, err := value.declaration.Digest()
		if err != nil {
			return typedmemory.CarrierRef{}, typedmemory.SHA256Digest{}, false
		}
		fields = []string{
			"prospective",
			fmt.Sprintf("%d", value.revision.Value()),
			fmt.Sprintf("%d", value.evaluationChangeOrdinal),
			fmt.Sprintf("%d", value.declarationChangeOrdinal),
			value.declaration.Entity().String(),
			value.declaration.Context().String(),
			declarationDigest.String(),
		}
	default:
		return typedmemory.CarrierRef{}, typedmemory.SHA256Digest{}, false
	}
	hash := sha256.New()
	writeClassificationVisibilityField(hash, "kind-classification-visibility-source.v1")
	for _, field := range fields {
		writeClassificationVisibilityField(hash, field)
	}
	digest, err := typedmemory.NewSHA256Digest(fmt.Sprintf("sha256:%x", hash.Sum(nil)))
	if err != nil {
		return typedmemory.CarrierRef{}, typedmemory.SHA256Digest{}, false
	}
	reference, err := typedmemory.NewCarrierRef(
		"kind-classification-visibility:" + digest.String(),
	)
	if err != nil {
		return typedmemory.CarrierRef{}, typedmemory.SHA256Digest{}, false
	}
	return reference, digest, true
}

type classificationVisibilityHashWriter interface {
	Write([]byte) (int, error)
}

func writeClassificationVisibilityField(
	writer classificationVisibilityHashWriter,
	value string,
) {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	_, _ = writer.Write(length)
	_, _ = writer.Write([]byte(value))
}

type KindClassificationAdmissionEngine interface {
	EvaluateKindClassification(
		context.Context,
		KindClassificationAdmissionInput,
	) (typedmemory.KindClassificationJudgement, error)
}

// ExactKindClassificationAdmissionEngine exposes the immutable evaluator
// identity set behind a callable admission engine. The additional coordinate
// lets a target-C snapshot loader reject an engine built for another X instead
// of trusting package wiring or Go concrete-type identity.
type ExactKindClassificationAdmissionEngine interface {
	KindClassificationAdmissionEngine
	ExactKindClassificationRegistry() kindclassificationruntime.Registry
}

func NewSnapshotObservableInputsSelected(
	blobs []ObservableInputBlob,
) (SnapshotObservableInputsSelected, error) {
	return memberofevaluation.NewSnapshotObservableInputsSelected(blobs)
}

func NewSnapshotObservableInputsNotApplicable() SnapshotObservableInputsNotApplicable {
	return memberofevaluation.NewSnapshotObservableInputsNotApplicable()
}

func NewSnapshotObservableInputsUnavailable() SnapshotObservableInputsUnavailable {
	return memberofevaluation.NewSnapshotObservableInputsUnavailable()
}

// ObservableInputContentProvider resolves immutable bytes before the semantic
// event exists. The v46 event table records an exact copy after validation; it
// is historical closure, not a staging area from which the adapter may invent
// or infer bytes.
type ObservableInputContentProvider interface {
	LoadObservableInput(
		context.Context,
		projectledger.ProjectID,
		typedmemory.ObservableInputRef,
		typedmemory.SHA256Digest,
	) (ObservableInputBlob, error)
}

type StrongReferenceResolutionInput struct {
	project       projectledger.ProjectID
	environment   typedmemory.TypeEnv
	graphRevision typedmemory.GraphRevision
	reference     typedmemory.PersistedRef
	context       typedmemory.BoundedContextRef
	universe      PersistedEntityUniverse
}

func newStrongReferenceResolutionInput(
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
	graphRevision typedmemory.GraphRevision,
	reference typedmemory.PersistedRef,
	contextRef typedmemory.BoundedContextRef,
	universe PersistedEntityUniverse,
) StrongReferenceResolutionInput {
	return StrongReferenceResolutionInput{
		project:       project,
		environment:   environment,
		graphRevision: graphRevision,
		reference:     reference,
		context:       contextRef,
		universe:      universe,
	}
}

func (input StrongReferenceResolutionInput) Project() projectledger.ProjectID {
	return input.project
}

func (input StrongReferenceResolutionInput) Environment() typedmemory.TypeEnv {
	return input.environment
}

func (input StrongReferenceResolutionInput) GraphRevision() typedmemory.GraphRevision {
	return input.graphRevision
}

func (input StrongReferenceResolutionInput) Reference() typedmemory.PersistedRef {
	return input.reference
}

func (input StrongReferenceResolutionInput) Context() typedmemory.BoundedContextRef {
	return input.context
}

func (input StrongReferenceResolutionInput) PersistedEntityUniverse() PersistedEntityUniverse {
	return input.universe
}

type StrongReferenceResolutionEngine interface {
	ResolveStrongReference(
		context.Context,
		StrongReferenceResolutionInput,
	) (typedmemory.StrongReferenceResolution, error)
}

type ObservableInputBlobUnavailableError struct {
	reference typedmemory.ObservableInputRef
	digest    typedmemory.SHA256Digest
}

func newObservableInputBlobUnavailableError(
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) ObservableInputBlobUnavailableError {
	return ObservableInputBlobUnavailableError{
		reference: reference,
		digest:    digest,
	}
}

func (failure ObservableInputBlobUnavailableError) Error() string {
	return fmt.Sprintf(
		"%s: reference=%s digest=%s",
		ErrObservableInputBlobRequired.Error(),
		failure.reference.String(),
		failure.digest.String(),
	)
}

func (failure ObservableInputBlobUnavailableError) Unwrap() error {
	return ErrObservableInputBlobRequired
}

func (failure ObservableInputBlobUnavailableError) Reference() typedmemory.ObservableInputRef {
	return failure.reference
}

func (failure ObservableInputBlobUnavailableError) Digest() typedmemory.SHA256Digest {
	return failure.digest
}
