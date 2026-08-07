package neighborhood

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type RequiredBasisRef struct{ value string }
type LegacyRecordRef struct{ value string }
type IdentityResolutionRef struct{ value string }
type ContextBridgeRef struct{ value string }

func NewRequiredBasisRef(raw string) (RequiredBasisRef, error) {
	value, err := exactReference("required basis", raw)
	if err != nil {
		return RequiredBasisRef{}, err
	}
	return RequiredBasisRef{value: value}, nil
}

func NewLegacyRecordRef(raw string) (LegacyRecordRef, error) {
	value, err := exactReference("legacy record", raw)
	if err != nil {
		return LegacyRecordRef{}, err
	}
	return LegacyRecordRef{value: value}, nil
}

func NewIdentityResolutionRef(raw string) (IdentityResolutionRef, error) {
	value, err := exactReference("identity resolution", raw)
	if err != nil {
		return IdentityResolutionRef{}, err
	}
	return IdentityResolutionRef{value: value}, nil
}

func NewContextBridgeRef(raw string) (ContextBridgeRef, error) {
	value, err := exactReference("context bridge", raw)
	if err != nil {
		return ContextBridgeRef{}, err
	}
	return ContextBridgeRef{value: value}, nil
}

func (ref RequiredBasisRef) String() string      { return ref.value }
func (ref LegacyRecordRef) String() string       { return ref.value }
func (ref IdentityResolutionRef) String() string { return ref.value }
func (ref ContextBridgeRef) String() string      { return ref.value }

type FacetBasisIssueKind string

const (
	IssueMissingTypeBasis           FacetBasisIssueKind = "missing_type_basis"
	IssueMissingCorrespondenceBasis FacetBasisIssueKind = "missing_correspondence_basis"
	IssueUnresolvedLegacyIdentity   FacetBasisIssueKind = "unresolved_legacy_identity"
	IssueStaleDerivedProjection     FacetBasisIssueKind = "stale_derived_projection"
	IssueExplicitBridgeRequired     FacetBasisIssueKind = "explicit_bridge_required"
)

type FacetBasisIssue interface {
	Kind() FacetBasisIssueKind
	Facet() FacetKind
	isFacetBasisIssue()
}

type MissingTypeBasisIssue struct {
	facet    FacetKind
	required RequiredBasisRef
}

func NewMissingTypeBasisIssue(
	facet FacetKind,
	required RequiredBasisRef,
) (MissingTypeBasisIssue, error) {
	issue := MissingTypeBasisIssue{
		facet:    facet,
		required: required,
	}
	if !facet.Valid() || required.String() == "" {
		return MissingTypeBasisIssue{}, fmt.Errorf(
			"missing-type-basis issue is invalid",
		)
	}
	return issue, nil
}

func (MissingTypeBasisIssue) Kind() FacetBasisIssueKind {
	return IssueMissingTypeBasis
}

func (issue MissingTypeBasisIssue) Facet() FacetKind {
	return issue.facet
}

func (issue MissingTypeBasisIssue) Required() RequiredBasisRef {
	return issue.required
}

func (MissingTypeBasisIssue) isFacetBasisIssue() {}

type MissingCorrespondenceBasisIssue struct {
	facet    FacetKind
	required ProjectionCorrespondenceManifestRef
}

func NewMissingCorrespondenceBasisIssue(
	facet FacetKind,
	required ProjectionCorrespondenceManifestRef,
) (MissingCorrespondenceBasisIssue, error) {
	issue := MissingCorrespondenceBasisIssue{
		facet:    facet,
		required: required,
	}
	if !facet.Valid() || required.String() == "" {
		return MissingCorrespondenceBasisIssue{}, fmt.Errorf(
			"missing-correspondence-basis issue is invalid",
		)
	}
	return issue, nil
}

func (MissingCorrespondenceBasisIssue) Kind() FacetBasisIssueKind {
	return IssueMissingCorrespondenceBasis
}

func (issue MissingCorrespondenceBasisIssue) Facet() FacetKind {
	return issue.facet
}

func (issue MissingCorrespondenceBasisIssue) Required() ProjectionCorrespondenceManifestRef {
	return issue.required
}

func (MissingCorrespondenceBasisIssue) isFacetBasisIssue() {}

type UnresolvedLegacyIdentityIssue struct {
	facet      FacetKind
	legacy     LegacyRecordRef
	resolution IdentityResolutionRef
}

func NewUnresolvedLegacyIdentityIssue(
	facet FacetKind,
	legacy LegacyRecordRef,
	resolution IdentityResolutionRef,
) (UnresolvedLegacyIdentityIssue, error) {
	issue := UnresolvedLegacyIdentityIssue{
		facet:      facet,
		legacy:     legacy,
		resolution: resolution,
	}
	if !facet.Valid() ||
		legacy.String() == "" ||
		resolution.String() == "" {
		return UnresolvedLegacyIdentityIssue{}, fmt.Errorf(
			"unresolved-legacy-identity issue is invalid",
		)
	}
	return issue, nil
}

func (UnresolvedLegacyIdentityIssue) Kind() FacetBasisIssueKind {
	return IssueUnresolvedLegacyIdentity
}

func (issue UnresolvedLegacyIdentityIssue) Facet() FacetKind {
	return issue.facet
}

func (issue UnresolvedLegacyIdentityIssue) LegacyRef() LegacyRecordRef {
	return issue.legacy
}

func (issue UnresolvedLegacyIdentityIssue) ResolutionRef() IdentityResolutionRef {
	return issue.resolution
}

func (UnresolvedLegacyIdentityIssue) isFacetBasisIssue() {}

type StaleDerivedProjectionIssue struct {
	facet           FacetKind
	projection      ProjectionRef
	observedVersion ProjectionVersion
	requiredVersion ProjectionVersion
}

func NewStaleDerivedProjectionIssue(
	facet FacetKind,
	projection ProjectionRef,
	observedVersion ProjectionVersion,
	requiredVersion ProjectionVersion,
) (StaleDerivedProjectionIssue, error) {
	issue := StaleDerivedProjectionIssue{
		facet:           facet,
		projection:      projection,
		observedVersion: observedVersion,
		requiredVersion: requiredVersion,
	}
	if !facet.Valid() ||
		projection.String() == "" ||
		observedVersion.String() == "" ||
		requiredVersion.String() == "" ||
		observedVersion == requiredVersion {
		return StaleDerivedProjectionIssue{}, fmt.Errorf(
			"stale-derived-projection issue is invalid",
		)
	}
	return issue, nil
}

func (StaleDerivedProjectionIssue) Kind() FacetBasisIssueKind {
	return IssueStaleDerivedProjection
}

func (issue StaleDerivedProjectionIssue) Facet() FacetKind {
	return issue.facet
}

func (issue StaleDerivedProjectionIssue) ProjectionRef() ProjectionRef {
	return issue.projection
}

func (issue StaleDerivedProjectionIssue) ObservedVersion() ProjectionVersion {
	return issue.observedVersion
}

func (issue StaleDerivedProjectionIssue) RequiredVersion() ProjectionVersion {
	return issue.requiredVersion
}

func (StaleDerivedProjectionIssue) isFacetBasisIssue() {}

type BridgeKnowledgeKind string

const (
	BridgeUnknown BridgeKnowledgeKind = "unknown"
	BridgeKnown   BridgeKnowledgeKind = "known"
)

type BridgeKnowledge interface {
	Kind() BridgeKnowledgeKind
	isBridgeKnowledge()
}

type UnknownBridge struct{}

func (UnknownBridge) Kind() BridgeKnowledgeKind { return BridgeUnknown }
func (UnknownBridge) isBridgeKnowledge()        {}

type KnownBridge struct{ ref ContextBridgeRef }

func NewKnownBridge(ref ContextBridgeRef) (KnownBridge, error) {
	if ref.String() == "" {
		return KnownBridge{}, fmt.Errorf("known bridge requires exact reference")
	}
	return KnownBridge{ref: ref}, nil
}

func (KnownBridge) Kind() BridgeKnowledgeKind { return BridgeKnown }
func (bridge KnownBridge) Ref() ContextBridgeRef {
	return bridge.ref
}
func (KnownBridge) isBridgeKnowledge() {}

type ExplicitBridgeRequiredIssue struct {
	facet  FacetKind
	source typedmemory.BoundedContextRef
	target typedmemory.BoundedContextRef
	bridge BridgeKnowledge
}

func NewExplicitBridgeRequiredIssue(
	facet FacetKind,
	source typedmemory.BoundedContextRef,
	target typedmemory.BoundedContextRef,
	bridge BridgeKnowledge,
) (ExplicitBridgeRequiredIssue, error) {
	issue := ExplicitBridgeRequiredIssue{
		facet:  facet,
		source: source,
		target: target,
		bridge: bridge,
	}
	if !issue.valid() {
		return ExplicitBridgeRequiredIssue{}, fmt.Errorf(
			"explicit-bridge-required issue is invalid",
		)
	}
	return issue, nil
}

func (ExplicitBridgeRequiredIssue) Kind() FacetBasisIssueKind {
	return IssueExplicitBridgeRequired
}

func (issue ExplicitBridgeRequiredIssue) Facet() FacetKind {
	return issue.facet
}

func (issue ExplicitBridgeRequiredIssue) SourceContext() typedmemory.BoundedContextRef {
	return issue.source
}

func (issue ExplicitBridgeRequiredIssue) TargetContext() typedmemory.BoundedContextRef {
	return issue.target
}

func (issue ExplicitBridgeRequiredIssue) Bridge() BridgeKnowledge {
	return issue.bridge
}

func (ExplicitBridgeRequiredIssue) isFacetBasisIssue() {}

func (issue ExplicitBridgeRequiredIssue) valid() bool {
	source, sourceErr := typedmemory.NewBoundedContextRef(issue.source.String())
	target, targetErr := typedmemory.NewBoundedContextRef(issue.target.String())
	if sourceErr != nil ||
		targetErr != nil ||
		source != issue.source ||
		target != issue.target ||
		source == target ||
		!issue.facet.Valid() ||
		issue.bridge == nil {
		return false
	}
	_, unknown := issue.bridge.(UnknownBridge)
	known, knownOK := issue.bridge.(KnownBridge)
	return unknown != knownOK &&
		(!knownOK || known.Ref().String() != "")
}

func validFacetBasisIssue(issue FacetBasisIssue) bool {
	if issue == nil || !issue.Facet().Valid() {
		return false
	}
	switch value := issue.(type) {
	case MissingTypeBasisIssue:
		return value.Required().String() != ""
	case MissingCorrespondenceBasisIssue:
		return value.Required().String() != ""
	case UnresolvedLegacyIdentityIssue:
		return value.LegacyRef().String() != "" &&
			value.ResolutionRef().String() != ""
	case StaleDerivedProjectionIssue:
		return value.ProjectionRef().String() != "" &&
			value.ObservedVersion() != value.RequiredVersion()
	case ExplicitBridgeRequiredIssue:
		return value.valid()
	default:
		return false
	}
}
