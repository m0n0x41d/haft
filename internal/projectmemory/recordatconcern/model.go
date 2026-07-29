// Package recordatconcern owns the pure task-oriented mapping from one explicit
// record-at-concern draft to a typed-memory candidate. It performs no selection, storage,
// admission, authority, CLI, MCP, or legacy-carrier effects.
package recordatconcern

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/adaptersource"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type DraftInput struct {
	ProjectID      projectidentity.ProjectID
	RecordEntity   typedmemory.EntityID
	RecordLocalRef typedmemory.BatchLocalRef
	RecordLabel    typedmemory.EntityLabel
	AssertionID    typedmemory.AssertionID
	ContextSlice   typedmemory.ContextSlice
	ClaimGraph     ClaimGraphBasis
	Provenance     typedmemory.ProvenanceRef
}

type Draft struct {
	projectID      projectidentity.ProjectID
	recordEntity   typedmemory.EntityID
	recordLocalRef typedmemory.BatchLocalRef
	recordLabel    typedmemory.EntityLabel
	assertionID    typedmemory.AssertionID
	contextSlice   typedmemory.ContextSlice
	claimGraph     ClaimGraphBasis
	provenance     typedmemory.ProvenanceRef
}

func NewDraft(input DraftInput) (Draft, error) {
	if _, err := projectidentity.ParseProjectID(input.ProjectID.String()); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft project: %w", err)
	}
	if err := requireExactEntityID(input.RecordEntity); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft record entity: %w", err)
	}
	if err := requireExactBatchLocalRef(input.RecordLocalRef); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft record local reference: %w", err)
	}
	if err := requireExactEntityLabel(input.RecordLabel); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft record label: %w", err)
	}
	if err := requireExactAssertionID(input.AssertionID); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft assertion: %w", err)
	}
	if err := requireExactContextSlice(input.ContextSlice); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft context slice: %w", err)
	}
	if !claimGraphBasisPresent(input.ClaimGraph) {
		return Draft{}, fmt.Errorf("record-at-concern draft ClaimGraph posture is required")
	}
	if err := requireExactProvenance(input.Provenance); err != nil {
		return Draft{}, fmt.Errorf("record-at-concern draft provenance: %w", err)
	}
	return Draft{
		projectID:      input.ProjectID,
		recordEntity:   input.RecordEntity,
		recordLocalRef: input.RecordLocalRef,
		recordLabel:    input.RecordLabel,
		assertionID:    input.AssertionID,
		contextSlice:   input.ContextSlice,
		claimGraph:     input.ClaimGraph,
		provenance:     input.Provenance,
	}, nil
}

func (draft Draft) ProjectID() projectidentity.ProjectID {
	return draft.projectID
}

func (draft Draft) RecordEntity() typedmemory.EntityID {
	return draft.recordEntity
}

func (draft Draft) RecordLocalRef() typedmemory.BatchLocalRef {
	return draft.recordLocalRef
}

func (draft Draft) RecordLabel() typedmemory.EntityLabel {
	return draft.recordLabel
}

func (draft Draft) AssertionID() typedmemory.AssertionID {
	return draft.assertionID
}

func (draft Draft) ContextSlice() typedmemory.ContextSlice {
	return draft.contextSlice
}

func (draft Draft) ClaimGraph() ClaimGraphBasis {
	return draft.claimGraph
}

func (draft Draft) Provenance() typedmemory.ProvenanceRef {
	return draft.provenance
}

// ClaimGraphBasis is the closed input posture for the by-value project-record claim.
// Missing basis is distinct from malformed graph content and therefore remains
// an Underdetermined adapter result rather than a constructor error.
type ClaimGraphBasis interface {
	recordClaimGraphBasisVariant()
}

type ExactClaimGraph struct {
	graph typedmemory.ClaimGraphValue
}

func NewExactClaimGraph(
	graph typedmemory.ClaimGraphValue,
) (ExactClaimGraph, error) {
	if graph == nil {
		return ExactClaimGraph{}, fmt.Errorf("exact project-record ClaimGraph is required")
	}
	return ExactClaimGraph{graph: graph}, nil
}

func (graph ExactClaimGraph) Value() typedmemory.ClaimGraphValue {
	return graph.graph
}

func (ExactClaimGraph) recordClaimGraphBasisVariant() {}

type MissingClaimGraph struct {
	missing []MissingBasis
}

func NewMissingClaimGraph(
	missing []MissingBasis,
) (MissingClaimGraph, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return MissingClaimGraph{}, err
	}
	return MissingClaimGraph{missing: normalized}, nil
}

func (graph MissingClaimGraph) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), graph.missing...)
}

func (MissingClaimGraph) recordClaimGraphBasisVariant() {}

type RuntimeBasis interface {
	recordRuntimeBasisVariant()
}

type ExactRuntimeBasis struct {
	project            projectidentity.ProjectID
	graphRevision      typedmemory.GraphRevision
	environment        typedmemory.TypeEnv
	codecs             typedmemory.CodecRegistry
	runtimeBasis       typedmemorystore.SelectedRuntimeBasisDigest
	registryCoordinate typedmemorystore.ExactTargetRegistryCoordinateDigest
	sourceMode         adaptersource.Mode
	registration       recordmembershipregistration.RegistrationArtifactV1
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

type exactRuntimeReadyBuilder struct {
	project            projectidentity.ProjectID
	graphRevision      typedmemory.GraphRevision
	environment        typedmemory.TypeEnv
	codecs             typedmemory.CodecRegistry
	runtimeBasis       typedmemorystore.SelectedRuntimeBasisDigest
	registryCoordinate typedmemorystore.ExactTargetRegistryCoordinateDigest
	sourceMode         adaptersource.Mode
	registration       recordmembershipregistration.RegistrationArtifactV1
}

func (builder exactRuntimeCoordinatesBuilder) SetRegistrationPolicy(
	registration recordmembershipregistration.RegistrationArtifactV1,
) exactRuntimeReadyBuilder {
	return exactRuntimeReadyBuilder{
		project:            builder.project,
		graphRevision:      builder.graphRevision,
		environment:        builder.environment,
		codecs:             builder.codecs,
		runtimeBasis:       builder.runtimeBasis,
		registryCoordinate: builder.registryCoordinate,
		sourceMode:         adaptersource.HistoricalMembership(),
		registration:       registration,
	}
}

// SetCurrentKindClassification selects the current C.3 source posture. It does
// not accept a historical registration policy and therefore cannot silently
// reintroduce MemberOf into a current candidate path.
func (builder exactRuntimeCoordinatesBuilder) SetCurrentKindClassification() exactRuntimeReadyBuilder {
	return exactRuntimeReadyBuilder{
		project:            builder.project,
		graphRevision:      builder.graphRevision,
		environment:        builder.environment,
		codecs:             builder.codecs,
		runtimeBasis:       builder.runtimeBasis,
		registryCoordinate: builder.registryCoordinate,
		sourceMode:         adaptersource.CurrentKindClassification(),
	}
}

func (builder exactRuntimeReadyBuilder) Build() (ExactRuntimeBasis, error) {
	project, err := projectidentity.ParseProjectID(builder.project.String())
	if err != nil || project != builder.project {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact record-at-concern runtime basis requires a canonical project",
		)
	}
	ref, err := typedmemory.ParseTypeEnvRef(builder.environment.Ref().String())
	if err != nil || ref != builder.environment.Ref() {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact record-at-concern runtime basis requires a canonical selected TypeEnv",
		)
	}
	if builder.runtimeBasis.Digest().String() == "" ||
		builder.registryCoordinate.Digest().String() == "" {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact record-at-concern runtime basis requires selected X and registry coordinates",
		)
	}
	if err := builder.sourceMode.Verify(); err != nil {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact record-at-concern runtime basis source mode: %w",
			err,
		)
	}
	if builder.sourceMode.IsHistoricalMembership() {
		if err := builder.registration.Verify(); err != nil {
			return ExactRuntimeBasis{}, fmt.Errorf(
				"exact record-at-concern runtime basis registration policy: %w",
				err,
			)
		}
	}
	return ExactRuntimeBasis(builder), nil
}

func (basis ExactRuntimeBasis) ProjectID() projectidentity.ProjectID {
	return basis.project
}

func (basis ExactRuntimeBasis) GraphRevision() typedmemory.GraphRevision {
	return basis.graphRevision
}

func (basis ExactRuntimeBasis) Environment() typedmemory.TypeEnv {
	return basis.environment
}

func (basis ExactRuntimeBasis) Codecs() typedmemory.CodecRegistry {
	return basis.codecs
}

func (basis ExactRuntimeBasis) SelectedRuntimeBasis() typedmemorystore.SelectedRuntimeBasisDigest {
	return basis.runtimeBasis
}

func (basis ExactRuntimeBasis) RegistryCoordinate() typedmemorystore.ExactTargetRegistryCoordinateDigest {
	return basis.registryCoordinate
}

func (basis ExactRuntimeBasis) RegistrationPolicy() recordmembershipregistration.RegistrationArtifactV1 {
	return basis.registration
}

func (basis ExactRuntimeBasis) SourceMode() adaptersource.Mode {
	return basis.sourceMode
}

func (ExactRuntimeBasis) recordRuntimeBasisVariant() {}

type MissingRuntimeBasis struct {
	missing []MissingBasis
}

func NewMissingRuntimeBasis(missing []MissingBasis) (MissingRuntimeBasis, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return MissingRuntimeBasis{}, err
	}
	return MissingRuntimeBasis{missing: normalized}, nil
}

func (basis MissingRuntimeBasis) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), basis.missing...)
}

func (MissingRuntimeBasis) recordRuntimeBasisVariant() {}

type ConcernBinding interface {
	recordConcernBindingVariant()
}

type ExactConcernBinding struct {
	reference typedmemory.PersistedRef
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	basis     typedmemory.ResolutionBasisRef
}

func NewExactConcernBinding(
	resolution typedmemory.ResolvedStrongReference,
) (ExactConcernBinding, error) {
	reference, ok := resolution.Reference().(typedmemory.PersistedRef)
	if !ok {
		return ExactConcernBinding{}, fmt.Errorf("EntityOfConcern resolution must contain a persisted reference")
	}
	if err := requireExactPersistedRef(reference); err != nil {
		return ExactConcernBinding{}, err
	}
	if err := requireExactEntityID(resolution.Entity()); err != nil {
		return ExactConcernBinding{}, err
	}
	if err := requireExactBoundedContext(resolution.Context()); err != nil {
		return ExactConcernBinding{}, err
	}
	if err := requireExactResolutionBasis(resolution.Basis()); err != nil {
		return ExactConcernBinding{}, err
	}
	return ExactConcernBinding{
		reference: reference,
		entity:    resolution.Entity(),
		context:   resolution.Context(),
		basis:     resolution.Basis(),
	}, nil
}

func (binding ExactConcernBinding) Reference() typedmemory.PersistedRef {
	return binding.reference
}

func (binding ExactConcernBinding) Entity() typedmemory.EntityID { return binding.entity }

func (binding ExactConcernBinding) Context() typedmemory.BoundedContextRef {
	return binding.context
}

func (binding ExactConcernBinding) Basis() typedmemory.ResolutionBasisRef {
	return binding.basis
}

func (ExactConcernBinding) recordConcernBindingVariant() {}

type UnsettledConcernBinding struct {
	missing []MissingBasis
}

func NewUnsettledConcernBinding(
	missing []MissingBasis,
) (UnsettledConcernBinding, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return UnsettledConcernBinding{}, err
	}
	return UnsettledConcernBinding{missing: normalized}, nil
}

func (binding UnsettledConcernBinding) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), binding.missing...)
}

func (UnsettledConcernBinding) recordConcernBindingVariant() {}

type MissingBasis struct {
	name   string
	repair typedmemory.RepairPointer
}

func NewMissingBasis(name string, repair typedmemory.RepairPointer) (MissingBasis, error) {
	if name == "" {
		return MissingBasis{}, fmt.Errorf("record-at-concern adapter missing-basis name is required")
	}
	parsed, err := typedmemory.NewRepairPointer(repair.String())
	if err != nil || parsed != repair {
		return MissingBasis{}, fmt.Errorf("record-at-concern adapter missing basis requires an exact repair pointer")
	}
	return MissingBasis{name: name, repair: repair}, nil
}

func (basis MissingBasis) Name() string { return basis.name }

func (basis MissingBasis) Repair() typedmemory.RepairPointer { return basis.repair }

type Violation struct {
	code    string
	message string
}

func (violation Violation) Code() string { return violation.code }

func (violation Violation) Message() string { return violation.message }

type Result interface {
	recordAdapterResultVariant()
}

// ValidCandidate is an unforgeable accepted pure mapping result. Its concrete
// implementation remains private so a zero value cannot claim that candidate,
// carrier and mapping coordinates were correlated.
type ValidCandidate interface {
	Result
	ChangeSet() typedmemory.MemoryChangeSet
	Carrier() recordcarrier.ProjectRecordCarrierV1
	CarrierBinding() recordcarrier.EntityRecordCarrierBindingV1
	MembershipSource() recordcarrier.RecordMembershipSourceV1
	ClassificationSource() recordcarrier.RecordClassificationSourceV1
	MappingManifestRef() recordmapping.MappingManifestRef
	AdapterVersion() recordmapping.AdapterVersion
	RelationDeclarationFragmentID() typedmemory.SignatureID
	// RelationSignatureID is the historical API spelling for the same
	// edition-bound fragment coordinate.
	RelationSignatureID() typedmemory.SignatureID
	validRecordCandidateResult()
}

type validCandidateResult struct {
	changeSet            typedmemory.MemoryChangeSet
	carrier              recordcarrier.ProjectRecordCarrierV1
	binding              recordcarrier.EntityRecordCarrierBindingV1
	membershipSource     recordcarrier.RecordMembershipSourceV1
	classificationSource recordcarrier.RecordClassificationSourceV1
	manifest             recordmapping.MappingManifestRef
	adapter              recordmapping.AdapterVersion
	signature            typedmemory.SignatureID
}

func (candidate validCandidateResult) ChangeSet() typedmemory.MemoryChangeSet {
	return candidate.changeSet
}

func (candidate validCandidateResult) Carrier() recordcarrier.ProjectRecordCarrierV1 {
	return candidate.carrier
}

func (candidate validCandidateResult) CarrierBinding() recordcarrier.EntityRecordCarrierBindingV1 {
	return candidate.binding
}

func (candidate validCandidateResult) MembershipSource() recordcarrier.RecordMembershipSourceV1 {
	return candidate.membershipSource
}

func (candidate validCandidateResult) ClassificationSource() recordcarrier.RecordClassificationSourceV1 {
	return candidate.classificationSource
}

func (candidate validCandidateResult) MappingManifestRef() recordmapping.MappingManifestRef {
	return candidate.manifest
}

func (candidate validCandidateResult) AdapterVersion() recordmapping.AdapterVersion {
	return candidate.adapter
}

func (candidate validCandidateResult) RelationSignatureID() typedmemory.SignatureID {
	return candidate.signature
}

func (candidate validCandidateResult) RelationDeclarationFragmentID() typedmemory.SignatureID {
	return candidate.signature
}

func (validCandidateResult) recordAdapterResultVariant() {}

func (validCandidateResult) validRecordCandidateResult() {}

type Invalid interface {
	Result
	Violations() []Violation
	invalidRecordAdapterResult()
}

type invalid struct {
	violations []Violation
}

func (result invalid) Violations() []Violation {
	return append([]Violation(nil), result.violations...)
}

func (invalid) recordAdapterResultVariant() {}

func (invalid) invalidRecordAdapterResult() {}

type Underdetermined interface {
	Result
	MissingBasis() []MissingBasis
	underdeterminedRecordAdapterResult()
}

type underdetermined struct {
	missing []MissingBasis
}

func (result underdetermined) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), result.missing...)
}

func (underdetermined) recordAdapterResultVariant() {}

func (underdetermined) underdeterminedRecordAdapterResult() {}

func invalidResult(code string, message string) Invalid {
	return invalid{violations: []Violation{{code: code, message: message}}}
}

func underdeterminedResult(missing ...MissingBasis) Underdetermined {
	normalized, _ := normalizeMissingBasis(missing)
	return underdetermined{missing: normalized}
}

func claimGraphBasisPresent(basis ClaimGraphBasis) bool {
	switch value := basis.(type) {
	case ExactClaimGraph:
		return value.graph != nil
	case MissingClaimGraph:
		return len(value.missing) > 0
	default:
		return false
	}
}

func normalizeMissingBasis(values []MissingBasis) ([]MissingBasis, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("record-at-concern adapter requires at least one missing basis")
	}
	owned := append([]MissingBasis(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name < owned[right].name
	})
	for index, value := range owned {
		if value.name == "" {
			return nil, fmt.Errorf("record-at-concern adapter missing basis at index %d is incomplete", index)
		}
		parsed, err := typedmemory.NewRepairPointer(value.repair.String())
		if err != nil || parsed != value.repair {
			return nil, fmt.Errorf("record-at-concern adapter missing basis %q has no exact repair pointer", value.name)
		}
		if index > 0 && owned[index-1].name == value.name {
			return nil, fmt.Errorf("record-at-concern adapter repeats missing basis %q", value.name)
		}
	}
	return owned, nil
}
