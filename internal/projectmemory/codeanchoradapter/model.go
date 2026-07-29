// Package codeanchoradapter maps one exact repository locator and one or more
// explicit semantic links into Haft-local typed-memory relations. It does not
// infer a link from proximity, affected_files, backlinks, or symbol search.
package codeanchoradapter

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/adaptersource"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type LocatorBasis interface {
	codeAnchorLocatorBasisVariant()
}

type ExactLocator struct {
	locator typedmemorycandidatecodec.CodeAnchorLocator
}

func NewExactLocator(
	locator typedmemorycandidatecodec.CodeAnchorLocator,
) ExactLocator {
	return ExactLocator{locator: locator}
}

func (locator ExactLocator) Value() typedmemorycandidatecodec.CodeAnchorLocator {
	return locator.locator
}

func (ExactLocator) codeAnchorLocatorBasisVariant() {}

type MissingLocator struct {
	missing []MissingBasis
}

func NewMissingLocator(
	missing []MissingBasis,
) (MissingLocator, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return MissingLocator{}, err
	}
	return MissingLocator{missing: normalized}, nil
}

func (locator MissingLocator) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), locator.missing...)
}

func (MissingLocator) codeAnchorLocatorBasisVariant() {}

type ReferenceBinding interface {
	codeAnchorReferenceBindingVariant()
}

type ExactReferenceBinding struct {
	reference typedmemory.PersistedRef
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	basis     typedmemory.ResolutionBasisRef
}

func NewExactReferenceBinding(
	resolution typedmemory.ResolvedStrongReference,
) (ExactReferenceBinding, error) {
	reference, ok := resolution.Reference().(typedmemory.PersistedRef)
	if !ok {
		return ExactReferenceBinding{}, fmt.Errorf(
			"code-anchor semantic target must be an exact persisted reference",
		)
	}
	if reference.ReferenceID().String() != resolution.Entity().String() {
		return ExactReferenceBinding{}, fmt.Errorf(
			"code-anchor semantic target reference and stable EntityID differ",
		)
	}
	if _, err := typedmemory.NewBoundedContextRef(
		resolution.Context().String(),
	); err != nil {
		return ExactReferenceBinding{}, fmt.Errorf(
			"code-anchor semantic target context: %w",
			err,
		)
	}
	if _, err := typedmemory.NewResolutionBasisRef(
		resolution.Basis().String(),
	); err != nil {
		return ExactReferenceBinding{}, fmt.Errorf(
			"code-anchor semantic target resolution basis: %w",
			err,
		)
	}
	return ExactReferenceBinding{
		reference: reference,
		entity:    resolution.Entity(),
		context:   resolution.Context(),
		basis:     resolution.Basis(),
	}, nil
}

func (binding ExactReferenceBinding) Reference() typedmemory.PersistedRef {
	return binding.reference
}

func (binding ExactReferenceBinding) Entity() typedmemory.EntityID {
	return binding.entity
}

func (binding ExactReferenceBinding) Context() typedmemory.BoundedContextRef {
	return binding.context
}

func (binding ExactReferenceBinding) Basis() typedmemory.ResolutionBasisRef {
	return binding.basis
}

func (ExactReferenceBinding) codeAnchorReferenceBindingVariant() {}

type UnsettledReferenceBinding struct {
	missing []MissingBasis
}

func NewUnsettledReferenceBinding(
	missing []MissingBasis,
) (UnsettledReferenceBinding, error) {
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return UnsettledReferenceBinding{}, err
	}
	return UnsettledReferenceBinding{missing: normalized}, nil
}

func (binding UnsettledReferenceBinding) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), binding.missing...)
}

func (UnsettledReferenceBinding) codeAnchorReferenceBindingVariant() {}

type SemanticLink interface {
	codeAnchorSemanticLinkVariant()
	assertionID() typedmemory.AssertionID
	targetBinding() ReferenceBinding
	sortKey() string
}

type ClaimLink struct {
	assertion typedmemory.AssertionID
	target    ReferenceBinding
}

func NewClaimLink(
	assertion typedmemory.AssertionID,
	target ReferenceBinding,
) (ClaimLink, error) {
	if _, err := typedmemory.NewAssertionID(assertion.String()); err != nil {
		return ClaimLink{}, fmt.Errorf("code-realizes-claim assertion: %w", err)
	}
	if !referenceBindingPresent(target) {
		return ClaimLink{}, fmt.Errorf(
			"code-realizes-claim target binding is required",
		)
	}
	return ClaimLink{assertion: assertion, target: target}, nil
}

func (link ClaimLink) AssertionID() typedmemory.AssertionID {
	return link.assertion
}

func (link ClaimLink) Target() ReferenceBinding { return link.target }

func (link ClaimLink) assertionID() typedmemory.AssertionID {
	return link.assertion
}

func (link ClaimLink) targetBinding() ReferenceBinding { return link.target }

func (link ClaimLink) sortKey() string {
	return "claim\x00" + referenceBindingSortKey(link.target) +
		"\x00" + link.assertion.String()
}

func (ClaimLink) codeAnchorSemanticLinkVariant() {}

type WorkLink struct {
	assertion typedmemory.AssertionID
	target    ReferenceBinding
}

func NewWorkLink(
	assertion typedmemory.AssertionID,
	target ReferenceBinding,
) (WorkLink, error) {
	if _, err := typedmemory.NewAssertionID(assertion.String()); err != nil {
		return WorkLink{}, fmt.Errorf("code-changed-by-work assertion: %w", err)
	}
	if !referenceBindingPresent(target) {
		return WorkLink{}, fmt.Errorf(
			"code-changed-by-work target binding is required",
		)
	}
	return WorkLink{assertion: assertion, target: target}, nil
}

func (link WorkLink) AssertionID() typedmemory.AssertionID {
	return link.assertion
}

func (link WorkLink) Target() ReferenceBinding { return link.target }

func (link WorkLink) assertionID() typedmemory.AssertionID {
	return link.assertion
}

func (link WorkLink) targetBinding() ReferenceBinding { return link.target }

func (link WorkLink) sortKey() string {
	return "work\x00" + referenceBindingSortKey(link.target) +
		"\x00" + link.assertion.String()
}

func (WorkLink) codeAnchorSemanticLinkVariant() {}

type DraftInput struct {
	ProjectID             projectidentity.ProjectID
	AnchorEntity          typedmemory.EntityID
	AnchorLocalRef        typedmemory.BatchLocalRef
	AnchorLabel           typedmemory.EntityLabel
	DefinitionAssertionID typedmemory.AssertionID
	ContextSlice          typedmemory.ContextSlice
	Locator               LocatorBasis
	Links                 []SemanticLink
	Provenance            typedmemory.ProvenanceRef
}

type Draft struct {
	projectID             projectidentity.ProjectID
	anchorEntity          typedmemory.EntityID
	anchorLocalRef        typedmemory.BatchLocalRef
	anchorLabel           typedmemory.EntityLabel
	definitionAssertionID typedmemory.AssertionID
	contextSlice          typedmemory.ContextSlice
	locator               LocatorBasis
	links                 []SemanticLink
	provenance            typedmemory.ProvenanceRef
}

func NewDraft(input DraftInput) (Draft, error) {
	project, err := projectidentity.ParseProjectID(input.ProjectID.String())
	if err != nil || project != input.ProjectID {
		return Draft{}, fmt.Errorf("code-anchor draft project is invalid")
	}
	if _, err := typedmemory.NewEntityID(input.AnchorEntity.String()); err != nil {
		return Draft{}, fmt.Errorf("code-anchor draft entity: %w", err)
	}
	if _, err := typedmemory.NewBatchLocalRef(
		input.AnchorLocalRef.String(),
	); err != nil {
		return Draft{}, fmt.Errorf("code-anchor draft local reference: %w", err)
	}
	if _, err := typedmemory.NewEntityLabel(input.AnchorLabel.String()); err != nil {
		return Draft{}, fmt.Errorf("code-anchor draft label: %w", err)
	}
	if _, err := typedmemory.NewAssertionID(
		input.DefinitionAssertionID.String(),
	); err != nil {
		return Draft{}, fmt.Errorf(
			"code-anchor definition assertion: %w",
			err,
		)
	}
	if _, err := typedmemory.DecodeCanonicalContextSlice(
		input.ContextSlice.CanonicalBytes(),
	); err != nil {
		return Draft{}, fmt.Errorf("code-anchor ContextSlice: %w", err)
	}
	if !locatorBasisPresent(input.Locator) {
		return Draft{}, fmt.Errorf("code-anchor locator posture is required")
	}
	links, err := normalizeSemanticLinks(input.Links)
	if err != nil {
		return Draft{}, err
	}
	if _, err := typedmemory.NewProvenanceRef(
		input.Provenance.String(),
	); err != nil {
		return Draft{}, fmt.Errorf("code-anchor provenance: %w", err)
	}
	for _, link := range links {
		if link.assertionID() == input.DefinitionAssertionID {
			return Draft{}, fmt.Errorf(
				"code-anchor definition and semantic link repeat assertion %q",
				input.DefinitionAssertionID.String(),
			)
		}
	}
	return Draft{
		projectID:             input.ProjectID,
		anchorEntity:          input.AnchorEntity,
		anchorLocalRef:        input.AnchorLocalRef,
		anchorLabel:           input.AnchorLabel,
		definitionAssertionID: input.DefinitionAssertionID,
		contextSlice:          input.ContextSlice,
		locator:               input.Locator,
		links:                 links,
		provenance:            input.Provenance,
	}, nil
}

func (draft Draft) ProjectID() projectidentity.ProjectID {
	return draft.projectID
}

func (draft Draft) AnchorEntity() typedmemory.EntityID {
	return draft.anchorEntity
}

func (draft Draft) AnchorLocalRef() typedmemory.BatchLocalRef {
	return draft.anchorLocalRef
}

func (draft Draft) AnchorLabel() typedmemory.EntityLabel {
	return draft.anchorLabel
}

func (draft Draft) DefinitionAssertionID() typedmemory.AssertionID {
	return draft.definitionAssertionID
}

func (draft Draft) ContextSlice() typedmemory.ContextSlice {
	return draft.contextSlice
}

func (draft Draft) Locator() LocatorBasis { return draft.locator }

func (draft Draft) Links() []SemanticLink {
	return append([]SemanticLink(nil), draft.links...)
}

func (draft Draft) Provenance() typedmemory.ProvenanceRef {
	return draft.provenance
}

type RuntimeBasis interface {
	codeAnchorRuntimeBasisVariant()
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
			"exact code-anchor runtime basis requires a canonical project",
		)
	}
	typeEnv, err := typedmemory.ParseTypeEnvRef(
		builder.environment.Ref().String(),
	)
	if err != nil || typeEnv != builder.environment.Ref() {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact code-anchor runtime basis requires a canonical selected TypeEnv",
		)
	}
	if builder.runtimeBasis.Digest().String() == "" ||
		builder.registryCoordinate.Digest().String() == "" {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact code-anchor runtime basis requires selected X and registry coordinates",
		)
	}
	if err := builder.sourceMode.Verify(); err != nil {
		return ExactRuntimeBasis{}, fmt.Errorf(
			"exact code-anchor runtime source mode: %w",
			err,
		)
	}
	if builder.sourceMode.IsHistoricalMembership() {
		if err := builder.registration.Verify(); err != nil {
			return ExactRuntimeBasis{}, fmt.Errorf(
				"exact code-anchor runtime registration policy: %w",
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

func (ExactRuntimeBasis) codeAnchorRuntimeBasisVariant() {}

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

func (MissingRuntimeBasis) codeAnchorRuntimeBasisVariant() {}

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
			"code-anchor adapter missing-basis name is required",
		)
	}
	parsed, err := typedmemory.NewRepairPointer(repair.String())
	if err != nil || parsed != repair {
		return MissingBasis{}, fmt.Errorf(
			"code-anchor adapter missing basis requires an exact repair pointer",
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
	codeAnchorAdapterResultVariant()
}

type ValidCandidate interface {
	Result
	ChangeSet() typedmemory.MemoryChangeSet
	Carrier() carrierfamily.CarrierV1
	CarrierBinding() carrierfamily.EntityCarrierBindingV1
	MembershipSource() carrierfamily.MembershipSourceV1
	ClassificationSource() carrierfamily.ClassificationSourceV1
	MappingManifestRef() recordmapping.MappingManifestRef
	AdapterVersion() recordmapping.AdapterVersion
	RelationDeclarationFragmentIDs() []typedmemory.SignatureID
	// RelationSignatureIDs is the historical API spelling for the same
	// edition-bound fragment coordinates.
	RelationSignatureIDs() []typedmemory.SignatureID
	validCodeAnchorCandidateResult()
}

type validCandidateResult struct {
	changeSet            typedmemory.MemoryChangeSet
	carrier              carrierfamily.CarrierV1
	binding              carrierfamily.EntityCarrierBindingV1
	membershipSource     carrierfamily.MembershipSourceV1
	classificationSource carrierfamily.ClassificationSourceV1
	manifest             recordmapping.MappingManifestRef
	adapter              recordmapping.AdapterVersion
	signatures           []typedmemory.SignatureID
}

func (candidate validCandidateResult) ChangeSet() typedmemory.MemoryChangeSet {
	return candidate.changeSet
}

func (candidate validCandidateResult) Carrier() carrierfamily.CarrierV1 {
	return candidate.carrier
}

func (candidate validCandidateResult) CarrierBinding() carrierfamily.EntityCarrierBindingV1 {
	return candidate.binding
}

func (candidate validCandidateResult) MembershipSource() carrierfamily.MembershipSourceV1 {
	return candidate.membershipSource
}

func (candidate validCandidateResult) ClassificationSource() carrierfamily.ClassificationSourceV1 {
	return candidate.classificationSource
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

func (validCandidateResult) codeAnchorAdapterResultVariant() {}

func (validCandidateResult) validCodeAnchorCandidateResult() {}

type Invalid interface {
	Result
	Violations() []Violation
	invalidCodeAnchorAdapterResult()
}

type invalid struct {
	violations []Violation
}

func (result invalid) Violations() []Violation {
	return append([]Violation(nil), result.violations...)
}

func (invalid) codeAnchorAdapterResultVariant() {}

func (invalid) invalidCodeAnchorAdapterResult() {}

type Underdetermined interface {
	Result
	MissingBasis() []MissingBasis
	underdeterminedCodeAnchorAdapterResult()
}

type underdetermined struct {
	missing []MissingBasis
}

func (result underdetermined) MissingBasis() []MissingBasis {
	return append([]MissingBasis(nil), result.missing...)
}

func (underdetermined) codeAnchorAdapterResultVariant() {}

func (underdetermined) underdeterminedCodeAnchorAdapterResult() {}

func invalidResult(code string, message string) Invalid {
	return invalid{violations: []Violation{{
		code:    code,
		message: message,
	}}}
}

func underdeterminedResult(missing ...MissingBasis) Underdetermined {
	normalized, _ := normalizeMissingBasis(missing)
	return underdetermined{missing: normalized}
}

func mustMissingBasis(name string, repair string) MissingBasis {
	pointer, _ := typedmemory.NewRepairPointer(repair)
	basis, _ := NewMissingBasis(name, pointer)
	return basis
}

func normalizeMissingBasis(
	values []MissingBasis,
) ([]MissingBasis, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"code-anchor adapter requires at least one missing basis",
		)
	}
	owned := append([]MissingBasis(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name < owned[right].name
	})
	for index, value := range owned {
		if value.name == "" {
			return nil, fmt.Errorf(
				"code-anchor adapter missing basis at index %d is incomplete",
				index,
			)
		}
		parsed, err := typedmemory.NewRepairPointer(value.repair.String())
		if err != nil || parsed != value.repair {
			return nil, fmt.Errorf(
				"code-anchor adapter missing basis %q has no exact repair pointer",
				value.name,
			)
		}
		if index > 0 && owned[index-1].name == value.name {
			return nil, fmt.Errorf(
				"code-anchor adapter repeats missing basis %q",
				value.name,
			)
		}
	}
	return owned, nil
}

func normalizeSemanticLinks(
	values []SemanticLink,
) ([]SemanticLink, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"code-anchor draft requires at least one explicit claim or performed-work link",
		)
	}
	owned := append([]SemanticLink(nil), values...)
	for index, value := range owned {
		if !semanticLinkPresent(value) {
			return nil, fmt.Errorf(
				"code-anchor semantic link at index %d is invalid",
				index,
			)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].sortKey() < owned[right].sortKey()
	})
	assertions := make(map[string]struct{}, len(owned))
	for _, value := range owned {
		assertion := value.assertionID().String()
		if _, duplicate := assertions[assertion]; duplicate {
			return nil, fmt.Errorf(
				"code-anchor semantic links repeat assertion %q",
				assertion,
			)
		}
		assertions[assertion] = struct{}{}
	}
	return owned, nil
}

func semanticLinkPresent(link SemanticLink) bool {
	switch value := link.(type) {
	case ClaimLink:
		return value.assertion.String() != "" &&
			referenceBindingPresent(value.target)
	case WorkLink:
		return value.assertion.String() != "" &&
			referenceBindingPresent(value.target)
	default:
		return false
	}
}

func locatorBasisPresent(basis LocatorBasis) bool {
	switch value := basis.(type) {
	case ExactLocator:
		return true
	case MissingLocator:
		return len(value.missing) > 0
	default:
		return false
	}
}

func referenceBindingPresent(binding ReferenceBinding) bool {
	switch value := binding.(type) {
	case ExactReferenceBinding:
		return value.reference.ReferenceID().String() != "" &&
			value.entity.String() != "" &&
			value.context.String() != "" &&
			value.basis.String() != ""
	case UnsettledReferenceBinding:
		return len(value.missing) > 0
	default:
		return false
	}
}

func referenceBindingSortKey(binding ReferenceBinding) string {
	switch value := binding.(type) {
	case ExactReferenceBinding:
		return "exact\x00" + value.reference.ReferenceKey()
	case UnsettledReferenceBinding:
		fields := make([]string, 0, len(value.missing))
		for _, missing := range value.missing {
			fields = append(
				fields,
				missing.name+"\x00"+missing.repair.String(),
			)
		}
		return "unsettled\x00" + fmt.Sprint(fields)
	default:
		return "invalid"
	}
}
