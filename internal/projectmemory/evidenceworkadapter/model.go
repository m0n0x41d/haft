// Package evidenceworkadapter maps one explicit, claim-bound local evidence use
// and its producing occurrence into Haft typed memory. Its values deliberately
// do not claim exact FPF Evidence or exact U.Work classification.
package evidenceworkadapter

import (
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/adaptersource"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type NewEntityIdentity struct {
	entity typedmemory.EntityID
	local  typedmemory.BatchLocalRef
	label  typedmemory.EntityLabel
}

func NewEntityIdentityValue(
	entity typedmemory.EntityID,
	local typedmemory.BatchLocalRef,
	label typedmemory.EntityLabel,
) (NewEntityIdentity, error) {
	if _, err := typedmemory.NewEntityID(entity.String()); err != nil {
		return NewEntityIdentity{}, fmt.Errorf("new entity identity: %w", err)
	}
	if _, err := typedmemory.NewBatchLocalRef(local.String()); err != nil {
		return NewEntityIdentity{}, fmt.Errorf("new entity local reference: %w", err)
	}
	if _, err := typedmemory.NewEntityLabel(label.String()); err != nil {
		return NewEntityIdentity{}, fmt.Errorf("new entity label: %w", err)
	}
	return NewEntityIdentity{
		entity: entity,
		local:  local,
		label:  label,
	}, nil
}

func (identity NewEntityIdentity) Entity() typedmemory.EntityID {
	return identity.entity
}

func (identity NewEntityIdentity) LocalRef() typedmemory.BatchLocalRef {
	return identity.local
}

func (identity NewEntityIdentity) Label() typedmemory.EntityLabel {
	return identity.label
}

type exactReferenceBinding struct {
	reference typedmemory.PersistedRef
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	basis     typedmemory.ResolutionBasisRef
}

func newExactReferenceBinding(
	resolution typedmemory.ResolvedStrongReference,
	expectedRefKind string,
) (exactReferenceBinding, error) {
	reference, ok := resolution.Reference().(typedmemory.PersistedRef)
	if !ok {
		return exactReferenceBinding{}, fmt.Errorf(
			"Evidence/Work reference must be an exact persisted reference",
		)
	}
	if reference.ReferenceID().String() != resolution.Entity().String() {
		return exactReferenceBinding{}, fmt.Errorf(
			"Evidence/Work reference and stable EntityID differ",
		)
	}
	if reference.RefKind().ID().String() != expectedRefKind {
		return exactReferenceBinding{}, fmt.Errorf(
			"Evidence/Work reference kind = %s, want %s",
			reference.RefKind().ID(),
			expectedRefKind,
		)
	}
	if _, err := typedmemory.NewBoundedContextRef(
		resolution.Context().String(),
	); err != nil {
		return exactReferenceBinding{}, fmt.Errorf(
			"Evidence/Work reference context: %w",
			err,
		)
	}
	if _, err := typedmemory.NewResolutionBasisRef(
		resolution.Basis().String(),
	); err != nil {
		return exactReferenceBinding{}, fmt.Errorf(
			"Evidence/Work resolution basis: %w",
			err,
		)
	}
	return exactReferenceBinding{
		reference: reference,
		entity:    resolution.Entity(),
		context:   resolution.Context(),
		basis:     resolution.Basis(),
	}, nil
}

type ExactConcernReference struct{ exactReferenceBinding }

func NewExactConcernReference(
	resolution typedmemory.ResolvedStrongReference,
) (ExactConcernReference, error) {
	binding, err := newExactReferenceBinding(resolution, "U.EntityRef")
	return ExactConcernReference{exactReferenceBinding: binding}, err
}

type ExactPerformerReference struct{ exactReferenceBinding }

func NewExactPerformerReference(
	resolution typedmemory.ResolvedStrongReference,
) (ExactPerformerReference, error) {
	binding, err := newExactReferenceBinding(resolution, "U.EntityRef")
	return ExactPerformerReference{exactReferenceBinding: binding}, err
}

type ExactProjectClaimReference struct{ exactReferenceBinding }

func NewExactProjectClaimReference(
	resolution typedmemory.ResolvedStrongReference,
) (ExactProjectClaimReference, error) {
	binding, err := newExactReferenceBinding(
		resolution,
		"Haft.ProjectClaimRef",
	)
	return ExactProjectClaimReference{exactReferenceBinding: binding}, err
}

type ExactCarrierEditionReference struct{ exactReferenceBinding }

func NewExactCarrierEditionReference(
	resolution typedmemory.ResolvedStrongReference,
) (ExactCarrierEditionReference, error) {
	binding, err := newExactReferenceBinding(
		resolution,
		"Haft.CarrierEditionRef",
	)
	return ExactCarrierEditionReference{exactReferenceBinding: binding}, err
}

type ExactClaimGraph struct {
	graph typedmemory.ClaimGraphValue
}

func NewExactClaimGraph(
	graph typedmemory.ClaimGraphValue,
) (ExactClaimGraph, error) {
	shapeID, err := typedmemory.NewShapeID(
		"Haft.Shape.ClaimGraphV1",
	)
	if err != nil {
		return ExactClaimGraph{}, err
	}
	shape, err := typedmemory.NewValueShapeRef(shapeID, mustDigest('0'))
	if err != nil {
		return ExactClaimGraph{}, err
	}
	codec, err := typedmemory.NewClaimGraphCodecV1(shape)
	if err != nil {
		return ExactClaimGraph{}, err
	}
	if _, ok := codec.EncodeInput(graph).(typedmemory.CanonicalizedCodecValue); !ok {
		return ExactClaimGraph{}, fmt.Errorf(
			"Evidence/Work ClaimGraph is outside the closed ClaimGraph algebra",
		)
	}
	return ExactClaimGraph{graph: graph}, nil
}

func (graph ExactClaimGraph) Value() typedmemory.ClaimGraphValue {
	return graph.graph
}

type DraftInput struct {
	ProjectID                projectidentity.ProjectID
	EvidenceRecord           NewEntityIdentity
	SupportingEpistemeRecord NewEntityIdentity
	WorkRecord               NewEntityIdentity
	PerformedWorkOccurrence  NewEntityIdentity
	SupportingAssertionID    typedmemory.AssertionID
	WorkAssertionID          typedmemory.AssertionID
	EvidenceUseAssertionID   typedmemory.AssertionID
	ContextSlice             typedmemory.ContextSlice
	Concern                  ExactConcernReference
	Performer                ExactPerformerReference
	TargetClaim              ExactProjectClaimReference
	ProvenanceCarrierEdition ExactCarrierEditionReference
	Qualifier                typedmemorycandidatecodec.EvidenceUseQualifier
	Interval                 typedmemorycandidatecodec.PerformedInterval
	SupportingClaimGraph     ExactClaimGraph
	WorkClaimGraph           ExactClaimGraph
	Provenance               typedmemory.ProvenanceRef
}

type Draft struct {
	projectID                projectidentity.ProjectID
	evidenceRecord           NewEntityIdentity
	supportingRecord         NewEntityIdentity
	workRecord               NewEntityIdentity
	occurrence               NewEntityIdentity
	supportingAssertion      typedmemory.AssertionID
	workAssertion            typedmemory.AssertionID
	evidenceAssertion        typedmemory.AssertionID
	contextSlice             typedmemory.ContextSlice
	concern                  ExactConcernReference
	performer                ExactPerformerReference
	targetClaim              ExactProjectClaimReference
	provenanceCarrierEdition ExactCarrierEditionReference
	qualifier                typedmemorycandidatecodec.EvidenceUseQualifier
	interval                 typedmemorycandidatecodec.PerformedInterval
	supportingClaimGraph     ExactClaimGraph
	workClaimGraph           ExactClaimGraph
	provenance               typedmemory.ProvenanceRef
}

func NewDraft(input DraftInput) (Draft, error) {
	project, err := projectidentity.ParseProjectID(input.ProjectID.String())
	if err != nil || project != input.ProjectID {
		return Draft{}, fmt.Errorf("Evidence/Work project is invalid")
	}
	identities := []NewEntityIdentity{
		input.EvidenceRecord,
		input.SupportingEpistemeRecord,
		input.WorkRecord,
		input.PerformedWorkOccurrence,
	}
	if err := requireDistinctIdentities(identities); err != nil {
		return Draft{}, err
	}
	assertions := []typedmemory.AssertionID{
		input.SupportingAssertionID,
		input.WorkAssertionID,
		input.EvidenceUseAssertionID,
	}
	if err := requireDistinctAssertions(assertions); err != nil {
		return Draft{}, err
	}
	if _, err := typedmemory.DecodeCanonicalContextSlice(
		input.ContextSlice.CanonicalBytes(),
	); err != nil {
		return Draft{}, fmt.Errorf("Evidence/Work ContextSlice: %w", err)
	}
	references := []exactReferenceBinding{
		input.Concern.exactReferenceBinding,
		input.Performer.exactReferenceBinding,
		input.TargetClaim.exactReferenceBinding,
		input.ProvenanceCarrierEdition.exactReferenceBinding,
	}
	for _, reference := range references {
		if reference.context != input.ContextSlice.Context() {
			return Draft{}, fmt.Errorf(
				"Evidence/Work references must share the ContextSlice context",
			)
		}
	}
	if input.Interval == nil ||
		reflect.ValueOf(input.Interval).IsZero() {
		return Draft{}, fmt.Errorf(
			"Evidence/Work performed interval is required",
		)
	}
	if _, err := typedmemory.NewProvenanceRef(
		input.Provenance.String(),
	); err != nil {
		return Draft{}, fmt.Errorf("Evidence/Work provenance: %w", err)
	}
	return Draft{
		projectID:                input.ProjectID,
		evidenceRecord:           input.EvidenceRecord,
		supportingRecord:         input.SupportingEpistemeRecord,
		workRecord:               input.WorkRecord,
		occurrence:               input.PerformedWorkOccurrence,
		supportingAssertion:      input.SupportingAssertionID,
		workAssertion:            input.WorkAssertionID,
		evidenceAssertion:        input.EvidenceUseAssertionID,
		contextSlice:             input.ContextSlice,
		concern:                  input.Concern,
		performer:                input.Performer,
		targetClaim:              input.TargetClaim,
		provenanceCarrierEdition: input.ProvenanceCarrierEdition,
		qualifier:                input.Qualifier,
		interval:                 input.Interval,
		supportingClaimGraph:     input.SupportingClaimGraph,
		workClaimGraph:           input.WorkClaimGraph,
		provenance:               input.Provenance,
	}, nil
}

func requireDistinctIdentities(
	identities []NewEntityIdentity,
) error {
	entities := make(map[string]struct{}, len(identities))
	locals := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		if _, err := NewEntityIdentityValue(
			identity.entity,
			identity.local,
			identity.label,
		); err != nil {
			return err
		}
		if _, exists := entities[identity.entity.String()]; exists {
			return fmt.Errorf(
				"Evidence/Work new entity identities must be distinct",
			)
		}
		if _, exists := locals[identity.local.String()]; exists {
			return fmt.Errorf(
				"Evidence/Work local references must be distinct",
			)
		}
		entities[identity.entity.String()] = struct{}{}
		locals[identity.local.String()] = struct{}{}
	}
	return nil
}

func requireDistinctAssertions(
	assertions []typedmemory.AssertionID,
) error {
	seen := make(map[string]struct{}, len(assertions))
	for _, assertion := range assertions {
		if _, err := typedmemory.NewAssertionID(assertion.String()); err != nil {
			return fmt.Errorf("Evidence/Work assertion: %w", err)
		}
		if _, exists := seen[assertion.String()]; exists {
			return fmt.Errorf(
				"Evidence/Work assertion identities must be distinct",
			)
		}
		seen[assertion.String()] = struct{}{}
	}
	return nil
}

type RuntimeBasis interface {
	evidenceWorkRuntimeBasisVariant()
}

type ExactRuntimeBasis struct {
	project            projectidentity.ProjectID
	graphRevision      typedmemory.GraphRevision
	environment        typedmemory.TypeEnv
	codecs             typedmemory.CodecRegistry
	runtimeBasis       typedmemorystore.SelectedRuntimeBasisDigest
	registryCoordinate typedmemorystore.ExactTargetRegistryCoordinateDigest
	sourceMode         adaptersource.Mode
	recordRegistration recordmembershipregistration.RegistrationArtifactV1
	workRegistration   recordmembershipregistration.RegistrationArtifactV1
}

type ExactRuntimeBasisBuilder struct {
	project projectidentity.ProjectID
}

func NewExactRuntimeBasisBuilder(
	project projectidentity.ProjectID,
) ExactRuntimeBasisBuilder {
	return ExactRuntimeBasisBuilder{project: project}
}

type exactRuntimeRevisionBuilder struct {
	project       projectidentity.ProjectID
	graphRevision typedmemory.GraphRevision
}

func (builder ExactRuntimeBasisBuilder) SetGraphRevision(
	revision typedmemory.GraphRevision,
) exactRuntimeRevisionBuilder {
	return exactRuntimeRevisionBuilder{
		project:       builder.project,
		graphRevision: revision,
	}
}

type exactRuntimeEnvironmentBuilder struct {
	project       projectidentity.ProjectID
	graphRevision typedmemory.GraphRevision
	environment   typedmemory.TypeEnv
}

func (builder exactRuntimeRevisionBuilder) SetEnvironment(
	environment typedmemory.TypeEnv,
) exactRuntimeEnvironmentBuilder {
	return exactRuntimeEnvironmentBuilder{
		project:       builder.project,
		graphRevision: builder.graphRevision,
		environment:   environment,
	}
}

type exactRuntimeCodecsBuilder struct {
	project       projectidentity.ProjectID
	graphRevision typedmemory.GraphRevision
	environment   typedmemory.TypeEnv
	codecs        typedmemory.CodecRegistry
}

func (builder exactRuntimeEnvironmentBuilder) SetCodecs(
	codecs typedmemory.CodecRegistry,
) exactRuntimeCodecsBuilder {
	return exactRuntimeCodecsBuilder{
		project:       builder.project,
		graphRevision: builder.graphRevision,
		environment:   builder.environment,
		codecs:        codecs,
	}
}

type exactRuntimeCoordinatesBuilder struct {
	project            projectidentity.ProjectID
	graphRevision      typedmemory.GraphRevision
	environment        typedmemory.TypeEnv
	codecs             typedmemory.CodecRegistry
	runtimeBasis       typedmemorystore.SelectedRuntimeBasisDigest
	registryCoordinate typedmemorystore.ExactTargetRegistryCoordinateDigest
}

func (builder exactRuntimeCodecsBuilder) SetSelectedRuntimeCoordinates(
	runtimeBasis typedmemorystore.SelectedRuntimeBasisDigest,
	registryCoordinate typedmemorystore.ExactTargetRegistryCoordinateDigest,
) exactRuntimeCoordinatesBuilder {
	return exactRuntimeCoordinatesBuilder{
		project:            builder.project,
		graphRevision:      builder.graphRevision,
		environment:        builder.environment,
		codecs:             builder.codecs,
		runtimeBasis:       runtimeBasis,
		registryCoordinate: registryCoordinate,
	}
}

type exactRuntimeRecordPolicyBuilder struct {
	exactRuntimeCoordinatesBuilder
	record recordmembershipregistration.RegistrationArtifactV1
}

func (builder exactRuntimeCoordinatesBuilder) SetRecordRegistrationPolicy(
	policy recordmembershipregistration.RegistrationArtifactV1,
) exactRuntimeRecordPolicyBuilder {
	return exactRuntimeRecordPolicyBuilder{
		exactRuntimeCoordinatesBuilder: builder,
		record:                         policy,
	}
}

type exactRuntimeReadyBuilder struct {
	exactRuntimeCoordinatesBuilder
	sourceMode adaptersource.Mode
	record     recordmembershipregistration.RegistrationArtifactV1
	work       recordmembershipregistration.RegistrationArtifactV1
}

func (builder exactRuntimeRecordPolicyBuilder) SetPerformedWorkRegistrationPolicy(
	policy recordmembershipregistration.RegistrationArtifactV1,
) exactRuntimeReadyBuilder {
	return exactRuntimeReadyBuilder{
		exactRuntimeCoordinatesBuilder: builder.exactRuntimeCoordinatesBuilder,
		sourceMode:                     adaptersource.HistoricalMembership(),
		record:                         builder.record,
		work:                           policy,
	}
}

func (builder exactRuntimeCoordinatesBuilder) SetCurrentKindClassification() exactRuntimeReadyBuilder {
	return exactRuntimeReadyBuilder{
		exactRuntimeCoordinatesBuilder: builder,
		sourceMode:                     adaptersource.CurrentKindClassification(),
	}
}

func (builder exactRuntimeReadyBuilder) Build() (ExactRuntimeBasis, error) {
	project, err := projectidentity.ParseProjectID(builder.project.String())
	if err != nil || project != builder.project {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact Evidence/Work runtime requires a canonical project",
		)
	}
	if err := builder.sourceMode.Verify(); err != nil {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"Evidence/Work adapter source mode: %w",
			err,
		)
	}
	if builder.sourceMode.IsHistoricalMembership() {
		if err := builder.record.Verify(); err != nil {
			return ExactRuntimeBasis{}, fmt.Errorf(
				"Evidence/Work record registration: %w",
				err,
			)
		}
		if err := builder.work.Verify(); err != nil {
			return ExactRuntimeBasis{}, fmt.Errorf(
				"Evidence/Work occurrence registration: %w",
				err,
			)
		}
	}
	return ExactRuntimeBasis{
		project:            builder.project,
		graphRevision:      builder.graphRevision,
		environment:        builder.environment,
		codecs:             builder.codecs,
		runtimeBasis:       builder.runtimeBasis,
		registryCoordinate: builder.registryCoordinate,
		sourceMode:         builder.sourceMode,
		recordRegistration: builder.record,
		workRegistration:   builder.work,
	}, nil
}

func (ExactRuntimeBasis) evidenceWorkRuntimeBasisVariant() {}

func (basis ExactRuntimeBasis) SourceMode() adaptersource.Mode {
	return basis.sourceMode
}

type MissingRuntimeBasis struct {
	missing []MissingBasis
}

func NewMissingRuntimeBasis(
	missing []MissingBasis,
) (MissingRuntimeBasis, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return MissingRuntimeBasis{}, err
	}
	return MissingRuntimeBasis{missing: normalized}, nil
}

func (basis MissingRuntimeBasis) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), basis.missing...)
}

func (MissingRuntimeBasis) evidenceWorkRuntimeBasisVariant() {}

type MissingBasis struct {
	name   string
	repair typedmemory.RepairPointer
}

func NewMissingBasis(
	name string,
	repair typedmemory.RepairPointer,
) (MissingBasis, error) {
	if name == "" {
		return MissingBasis{}, fmt.Errorf(
			"Evidence/Work missing-basis name is required",
		)
	}
	parsed, err := typedmemory.NewRepairPointer(repair.String())
	if err != nil || parsed != repair {
		return MissingBasis{}, fmt.Errorf(
			"Evidence/Work missing basis requires an exact repair pointer",
		)
	}
	return MissingBasis{name: name, repair: repair}, nil
}

func (basis MissingBasis) Name() string { return basis.name }

func (basis MissingBasis) Repair() typedmemory.RepairPointer {
	return basis.repair
}

type Violation struct {
	code    string
	message string
}

func (violation Violation) Code() string { return violation.code }

func (violation Violation) Message() string { return violation.message }

type Result interface {
	evidenceWorkAdapterResultVariant()
}

type ValidCandidate interface {
	Result
	ChangeSet() typedmemory.MemoryChangeSet
	RecordMembershipSources() []recordcarrier.RecordMembershipSourceV1
	OccurrenceMembershipSource() carrierfamily.MembershipSourceV1
	RecordClassificationSources() []recordcarrier.RecordClassificationSourceV1
	OccurrenceClassificationSource() carrierfamily.ClassificationSourceV1
	MappingManifestRef() recordmapping.MappingManifestRef
	AdapterVersion() recordmapping.AdapterVersion
	RelationDeclarationFragmentIDs() []typedmemory.SignatureID
	// RelationSignatureIDs is the historical API spelling for the same
	// edition-bound fragment coordinates.
	RelationSignatureIDs() []typedmemory.SignatureID
	validEvidenceWorkCandidateResult()
}

type validCandidateResult struct {
	changeSet                      typedmemory.MemoryChangeSet
	recordSources                  []recordcarrier.RecordMembershipSourceV1
	occurrenceSource               carrierfamily.MembershipSourceV1
	recordClassificationSources    []recordcarrier.RecordClassificationSourceV1
	occurrenceClassificationSource carrierfamily.ClassificationSourceV1
	manifest                       recordmapping.MappingManifestRef
	adapter                        recordmapping.AdapterVersion
	signatures                     []typedmemory.SignatureID
}

func (candidate validCandidateResult) ChangeSet() typedmemory.MemoryChangeSet {
	return candidate.changeSet
}

func (candidate validCandidateResult) RecordMembershipSources() []recordcarrier.RecordMembershipSourceV1 {
	return append(
		[]recordcarrier.RecordMembershipSourceV1(nil),
		candidate.recordSources...,
	)
}

func (candidate validCandidateResult) OccurrenceMembershipSource() carrierfamily.MembershipSourceV1 {
	return candidate.occurrenceSource
}

func (candidate validCandidateResult) RecordClassificationSources() []recordcarrier.RecordClassificationSourceV1 {
	return append(
		[]recordcarrier.RecordClassificationSourceV1(nil),
		candidate.recordClassificationSources...,
	)
}

func (candidate validCandidateResult) OccurrenceClassificationSource() carrierfamily.ClassificationSourceV1 {
	return candidate.occurrenceClassificationSource
}

func (candidate validCandidateResult) MappingManifestRef() recordmapping.MappingManifestRef {
	return candidate.manifest
}

func (candidate validCandidateResult) AdapterVersion() recordmapping.AdapterVersion {
	return candidate.adapter
}

func (candidate validCandidateResult) RelationSignatureIDs() []typedmemory.SignatureID {
	return append([]typedmemory.SignatureID(nil), candidate.signatures...)
}

func (candidate validCandidateResult) RelationDeclarationFragmentIDs() []typedmemory.SignatureID {
	return append([]typedmemory.SignatureID(nil), candidate.signatures...)
}

func (validCandidateResult) evidenceWorkAdapterResultVariant() {}

func (validCandidateResult) validEvidenceWorkCandidateResult() {}

type Invalid interface {
	Result
	Violations() []Violation
}

type invalid struct {
	violations []Violation
}

func (result invalid) Violations() []Violation {
	return append([]Violation(nil), result.violations...)
}

func (invalid) evidenceWorkAdapterResultVariant() {}

type Underdetermined interface {
	Result
	MissingBasis() []MissingBasis
}

type underdetermined struct {
	missing []MissingBasis
}

func (result underdetermined) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), result.missing...)
}

func (underdetermined) evidenceWorkAdapterResultVariant() {}

func invalidResult(code string, message string) invalid {
	return invalid{violations: []Violation{{
		code:    code,
		message: message,
	}}}
}

func underdeterminedResult(basis MissingBasis) underdetermined {
	return underdetermined{missing: []MissingBasis{basis}}
}

func mustMissingBasis(
	name string,
	repair string,
) MissingBasis {
	pointer, err := typedmemory.NewRepairPointer(repair)
	if err != nil {
		panic(err)
	}
	basis, err := NewMissingBasis(name, pointer)
	if err != nil {
		panic(err)
	}
	return basis
}

func normalizeMissingBasis(
	values []MissingBasis,
) ([]MissingBasis, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"Evidence/Work missing-basis set must not be empty",
		)
	}
	byKey := make(map[string]MissingBasis, len(values))
	for _, value := range values {
		parsed, err := NewMissingBasis(value.name, value.repair)
		if err != nil {
			return nil, err
		}
		key := parsed.name + "\x00" + parsed.repair.String()
		byKey[key] = parsed
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sortStrings(keys)
	result := make([]MissingBasis, 0, len(keys))
	for _, key := range keys {
		result = append(result, byKey[key])
	}
	return result, nil
}

func sortStrings(values []string) {
	for left := 0; left < len(values); left++ {
		for right := left + 1; right < len(values); right++ {
			if values[right] < values[left] {
				values[left], values[right] = values[right], values[left]
			}
		}
	}
}

func mustDigest(fill byte) typedmemory.SHA256Digest {
	raw := make([]byte, 64)
	for index := range raw {
		raw[index] = fill
	}
	digest, err := typedmemory.NewSHA256Digest("sha256:" + string(raw))
	if err != nil {
		panic(err)
	}
	return digest
}
