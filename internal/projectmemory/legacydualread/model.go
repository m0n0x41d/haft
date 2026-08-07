// Package legacydualread defines the sealed, read-only migration view that
// keeps current typed-memory observations and legacy compatibility data
// together without turning legacy labels into typed identity or relations.
package legacydualread

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const (
	ViewSchemaVersionV1           = "haft.legacy-dual-read-view/v1"
	IdentityBridgeSchemaVersionV1 = "haft.legacy-identity-bridge/v1"
	SemanticPostureLegacyUnbound  = "legacy_unbound"
)

type MappingCarrierBasis struct {
	ref     typedmemory.CarrierRef
	edition typedmemory.CarrierEdition
	digest  typedmemory.SHA256Digest
}

func NewMappingCarrierBasis(
	ref typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	digest typedmemory.SHA256Digest,
) (MappingCarrierBasis, error) {
	parsedRef, refErr := typedmemory.NewCarrierRef(ref.String())
	parsedEdition, editionErr := typedmemory.NewCarrierEdition(edition.String())
	parsedDigest, digestErr := typedmemory.NewSHA256Digest(digest.String())
	if refErr != nil ||
		editionErr != nil ||
		digestErr != nil ||
		parsedRef != ref ||
		parsedEdition != edition ||
		parsedDigest != digest {
		return MappingCarrierBasis{}, fmt.Errorf(
			"identity-bridge mapping carrier basis is invalid",
		)
	}
	return MappingCarrierBasis{
		ref:     parsedRef,
		edition: parsedEdition,
		digest:  parsedDigest,
	}, nil
}

func (basis MappingCarrierBasis) Ref() typedmemory.CarrierRef {
	return basis.ref
}

func (basis MappingCarrierBasis) Edition() typedmemory.CarrierEdition {
	return basis.edition
}

func (basis MappingCarrierBasis) Digest() typedmemory.SHA256Digest {
	return basis.digest
}

func (basis MappingCarrierBasis) valid() bool {
	rebuilt, err := NewMappingCarrierBasis(
		basis.ref,
		basis.edition,
		basis.digest,
	)
	return err == nil && rebuilt == basis
}

type IdentityBridge struct {
	project   projectidentity.ProjectID
	legacy    legacyimport.LegacyIdentityRef
	entity    typedmemory.EntityID
	context   typedmemory.BoundedContextRef
	basis     MappingCarrierBasis
	canonical []byte
	digest    typedmemory.SHA256Digest
}

type IdentityBridgeInput struct {
	Project projectidentity.ProjectID
	Legacy  legacyimport.LegacyIdentityRef
	Entity  typedmemory.EntityID
	Context typedmemory.BoundedContextRef
	Basis   MappingCarrierBasis
}

func NewIdentityBridge(input IdentityBridgeInput) (IdentityBridge, error) {
	project, projectErr := projectidentity.ParseProjectID(input.Project.String())
	legacy, legacyErr := legacyimport.NewLegacyIdentityRef(
		input.Legacy.String(),
	)
	entity, entityErr := typedmemory.NewEntityID(input.Entity.String())
	context, contextErr := typedmemory.NewBoundedContextRef(
		input.Context.String(),
	)
	valid := projectErr == nil &&
		legacyErr == nil &&
		entityErr == nil &&
		contextErr == nil &&
		input.Basis.valid() &&
		project == input.Project &&
		legacy == input.Legacy &&
		entity == input.Entity &&
		context == input.Context
	if !valid {
		return IdentityBridge{}, fmt.Errorf(
			"legacy identity bridge input is invalid",
		)
	}
	dto := identityBridgeDTO{
		SchemaVersion:  IdentityBridgeSchemaVersionV1,
		ProjectID:      project.String(),
		LegacyIdentity: legacy.String(),
		EntityID:       entity.String(),
		BoundedContext: context.String(),
		MappingCarrier: mappingCarrierDTOOf(input.Basis),
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return IdentityBridge{}, fmt.Errorf(
			"encode legacy identity bridge: %w",
			err,
		)
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return IdentityBridge{}, err
	}
	return IdentityBridge{
		project:   project,
		legacy:    legacy,
		entity:    entity,
		context:   context,
		basis:     input.Basis,
		canonical: canonical,
		digest:    digest,
	}, nil
}

func (bridge IdentityBridge) ProjectID() projectidentity.ProjectID {
	return bridge.project
}

func (bridge IdentityBridge) LegacyIdentity() legacyimport.LegacyIdentityRef {
	return bridge.legacy
}

func (bridge IdentityBridge) EntityID() typedmemory.EntityID {
	return bridge.entity
}

func (bridge IdentityBridge) BoundedContext() typedmemory.BoundedContextRef {
	return bridge.context
}

func (bridge IdentityBridge) MappingCarrier() MappingCarrierBasis {
	return bridge.basis
}

func (bridge IdentityBridge) CanonicalBytes() []byte {
	return append([]byte(nil), bridge.canonical...)
}

func (bridge IdentityBridge) Digest() typedmemory.SHA256Digest {
	return bridge.digest
}

func (bridge IdentityBridge) valid() bool {
	rebuilt, err := NewIdentityBridge(IdentityBridgeInput{
		Project: bridge.project,
		Legacy:  bridge.legacy,
		Entity:  bridge.entity,
		Context: bridge.context,
		Basis:   bridge.basis,
	})
	return err == nil &&
		rebuilt.digest == bridge.digest &&
		bytes.Equal(rebuilt.canonical, bridge.canonical)
}

type TypedTarget struct {
	entity  typedmemory.EntityID
	context typedmemory.BoundedContextRef
}

func newTypedTarget(
	entity typedmemory.EntityID,
	context typedmemory.BoundedContextRef,
) TypedTarget {
	return TypedTarget{entity: entity, context: context}
}

func (target TypedTarget) EntityID() typedmemory.EntityID {
	return target.entity
}

func (target TypedTarget) BoundedContext() typedmemory.BoundedContextRef {
	return target.context
}

func (target TypedTarget) key() string {
	return target.entity.String() + "\x00" + target.context.String()
}

type IdentityResolutionKind string

const (
	ResolutionExact       IdentityResolutionKind = "exact"
	ResolutionUnbound     IdentityResolutionKind = "unbound"
	ResolutionAmbiguous   IdentityResolutionKind = "ambiguous"
	ResolutionUnavailable IdentityResolutionKind = "identity_unavailable"
)

type IdentityResolution interface {
	Kind() IdentityResolutionKind
	identityResolutionVariant()
}

type ExactIdentityResolution struct {
	legacy  legacyimport.LegacyIdentityRef
	target  TypedTarget
	bridges []IdentityBridge
}

func newExactIdentityResolution(
	legacy legacyimport.LegacyIdentityRef,
	target TypedTarget,
	bridges []IdentityBridge,
) ExactIdentityResolution {
	owned := canonicalBridges(bridges)
	return ExactIdentityResolution{
		legacy:  legacy,
		target:  target,
		bridges: owned,
	}
}

func (ExactIdentityResolution) Kind() IdentityResolutionKind {
	return ResolutionExact
}

func (ExactIdentityResolution) identityResolutionVariant() {}

func (resolution ExactIdentityResolution) LegacyIdentity() legacyimport.LegacyIdentityRef {
	return resolution.legacy
}

func (resolution ExactIdentityResolution) Target() TypedTarget {
	return resolution.target
}

func (resolution ExactIdentityResolution) Bridges() []IdentityBridge {
	return append([]IdentityBridge(nil), resolution.bridges...)
}

type UnboundIdentityResolution struct {
	legacy legacyimport.LegacyIdentityRef
}

func newUnboundIdentityResolution(
	legacy legacyimport.LegacyIdentityRef,
) UnboundIdentityResolution {
	return UnboundIdentityResolution{legacy: legacy}
}

func (UnboundIdentityResolution) Kind() IdentityResolutionKind {
	return ResolutionUnbound
}

func (UnboundIdentityResolution) identityResolutionVariant() {}

func (resolution UnboundIdentityResolution) LegacyIdentity() legacyimport.LegacyIdentityRef {
	return resolution.legacy
}

type AmbiguousIdentityResolution struct {
	legacy     legacyimport.LegacyIdentityRef
	candidates []TypedTarget
}

func newAmbiguousIdentityResolution(
	legacy legacyimport.LegacyIdentityRef,
	candidates []TypedTarget,
) AmbiguousIdentityResolution {
	owned := append([]TypedTarget(nil), candidates...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].key() < owned[right].key()
	})
	return AmbiguousIdentityResolution{
		legacy:     legacy,
		candidates: owned,
	}
}

func (AmbiguousIdentityResolution) Kind() IdentityResolutionKind {
	return ResolutionAmbiguous
}

func (AmbiguousIdentityResolution) identityResolutionVariant() {}

func (resolution AmbiguousIdentityResolution) LegacyIdentity() legacyimport.LegacyIdentityRef {
	return resolution.legacy
}

func (resolution AmbiguousIdentityResolution) Candidates() []TypedTarget {
	return append([]TypedTarget(nil), resolution.candidates...)
}

type IdentityUnavailableResolution struct{}

func (IdentityUnavailableResolution) Kind() IdentityResolutionKind {
	return ResolutionUnavailable
}

func (IdentityUnavailableResolution) identityResolutionVariant() {}

type LegacyCarrierRead struct {
	subject        legacyimport.SemanticSubjectRef
	classification legacyimport.ClassificationKind
	carrier        legacyimport.CarrierSnapshot
	resolution     IdentityResolution
}

func (read LegacyCarrierRead) Subject() legacyimport.SemanticSubjectRef {
	return read.subject
}

func (read LegacyCarrierRead) Classification() legacyimport.ClassificationKind {
	return read.classification
}

func (read LegacyCarrierRead) Carrier() legacyimport.CarrierSnapshot {
	return read.carrier
}

func (read LegacyCarrierRead) IdentityResolution() IdentityResolution {
	return read.resolution
}

type LegacyAssociationRead struct {
	subject        legacyimport.SemanticSubjectRef
	classification legacyimport.ClassificationKind
	carrier        legacyimport.CarrierSnapshot
	observation    legacyimport.AssociationObservation
	source         IdentityResolution
	target         IdentityResolution
}

func (read LegacyAssociationRead) Subject() legacyimport.SemanticSubjectRef {
	return read.subject
}

func (read LegacyAssociationRead) Classification() legacyimport.ClassificationKind {
	return read.classification
}

func (read LegacyAssociationRead) SemanticPosture() string {
	return SemanticPostureLegacyUnbound
}

func (read LegacyAssociationRead) Carrier() legacyimport.CarrierSnapshot {
	return read.carrier
}

func (read LegacyAssociationRead) Observation() legacyimport.AssociationObservation {
	return read.observation
}

func (read LegacyAssociationRead) SourceResolution() IdentityResolution {
	return read.source
}

func (read LegacyAssociationRead) TargetResolution() IdentityResolution {
	return read.target
}

type IssueKind string

const (
	IssueIdentityCollision  IssueKind = "legacy_identity_collision"
	IssueBridgeTargetAbsent IssueKind = "bridge_target_absent"
	IssueBridgeSourceAbsent IssueKind = "bridge_legacy_source_absent"
)

type CoalescingIssue interface {
	Kind() IssueKind
	issueDTO() coalescingIssueDTO
	coalescingIssueVariant()
}

type IdentityCollisionIssue struct {
	legacy     legacyimport.LegacyIdentityRef
	candidates []TypedTarget
	bridges    []IdentityBridge
}

func newIdentityCollisionIssue(
	legacy legacyimport.LegacyIdentityRef,
	candidates []TypedTarget,
	bridges []IdentityBridge,
) IdentityCollisionIssue {
	resolution := newAmbiguousIdentityResolution(legacy, candidates)
	return IdentityCollisionIssue{
		legacy:     legacy,
		candidates: resolution.candidates,
		bridges:    canonicalBridges(bridges),
	}
}

func (IdentityCollisionIssue) Kind() IssueKind {
	return IssueIdentityCollision
}

func (IdentityCollisionIssue) coalescingIssueVariant() {}

func (issue IdentityCollisionIssue) LegacyIdentity() legacyimport.LegacyIdentityRef {
	return issue.legacy
}

func (issue IdentityCollisionIssue) Candidates() []TypedTarget {
	return append([]TypedTarget(nil), issue.candidates...)
}

func (issue IdentityCollisionIssue) Bridges() []IdentityBridge {
	return append([]IdentityBridge(nil), issue.bridges...)
}

func (issue IdentityCollisionIssue) issueDTO() coalescingIssueDTO {
	return coalescingIssueDTO{
		Kind:       string(issue.Kind()),
		LegacyRef:  issue.legacy.String(),
		Candidates: targetDTOs(issue.candidates),
		BridgeRefs: bridgeDigestStrings(issue.bridges),
	}
}

type BridgeTargetAbsentIssue struct {
	bridge IdentityBridge
}

func newBridgeTargetAbsentIssue(
	bridge IdentityBridge,
) BridgeTargetAbsentIssue {
	return BridgeTargetAbsentIssue{bridge: bridge}
}

func (BridgeTargetAbsentIssue) Kind() IssueKind {
	return IssueBridgeTargetAbsent
}

func (BridgeTargetAbsentIssue) coalescingIssueVariant() {}

func (issue BridgeTargetAbsentIssue) Bridge() IdentityBridge {
	return issue.bridge
}

func (issue BridgeTargetAbsentIssue) issueDTO() coalescingIssueDTO {
	return coalescingIssueDTO{
		Kind:      string(issue.Kind()),
		LegacyRef: issue.bridge.LegacyIdentity().String(),
		Candidates: targetDTOs([]TypedTarget{newTypedTarget(
			issue.bridge.EntityID(),
			issue.bridge.BoundedContext(),
		)}),
		BridgeRefs: []string{issue.bridge.Digest().String()},
	}
}

type BridgeLegacySourceAbsentIssue struct {
	bridge IdentityBridge
}

func newBridgeLegacySourceAbsentIssue(
	bridge IdentityBridge,
) BridgeLegacySourceAbsentIssue {
	return BridgeLegacySourceAbsentIssue{bridge: bridge}
}

func (BridgeLegacySourceAbsentIssue) Kind() IssueKind {
	return IssueBridgeSourceAbsent
}

func (BridgeLegacySourceAbsentIssue) coalescingIssueVariant() {}

func (issue BridgeLegacySourceAbsentIssue) Bridge() IdentityBridge {
	return issue.bridge
}

func (issue BridgeLegacySourceAbsentIssue) issueDTO() coalescingIssueDTO {
	return coalescingIssueDTO{
		Kind:      string(issue.Kind()),
		LegacyRef: issue.bridge.LegacyIdentity().String(),
		Candidates: targetDTOs([]TypedTarget{newTypedTarget(
			issue.bridge.EntityID(),
			issue.bridge.BoundedContext(),
		)}),
		BridgeRefs: []string{issue.bridge.Digest().String()},
	}
}

type View struct {
	directory    typedmemorystore.CurrentEntityDirectory
	graph        typedmemorystore.CurrentProjectGraphObservation
	legacyReport legacyimport.DryRunReport
	bridges      []IdentityBridge
	carriers     []LegacyCarrierRead
	associations []LegacyAssociationRead
	issues       []CoalescingIssue
	canonical    []byte
	digest       typedmemory.SHA256Digest
}

func (view View) TypedEntityDirectory() typedmemorystore.CurrentEntityDirectory {
	return view.directory
}

func (view View) TypedGraphObservation() typedmemorystore.CurrentProjectGraphObservation {
	return view.graph
}

func (view View) LegacyReport() legacyimport.DryRunReport {
	return view.legacyReport
}

func (view View) IdentityBridges() []IdentityBridge {
	return append([]IdentityBridge(nil), view.bridges...)
}

func (view View) LegacyCarriers() []LegacyCarrierRead {
	return append([]LegacyCarrierRead(nil), view.carriers...)
}

func (view View) LegacyAssociations() []LegacyAssociationRead {
	return append([]LegacyAssociationRead(nil), view.associations...)
}

func (view View) Issues() []CoalescingIssue {
	return append([]CoalescingIssue(nil), view.issues...)
}

func (view View) CanonicalBytes() []byte {
	return append([]byte(nil), view.canonical...)
}

func (view View) Digest() typedmemory.SHA256Digest {
	return view.digest
}

type identityBridgeDTO struct {
	SchemaVersion  string            `json:"schema_version"`
	ProjectID      string            `json:"project_id"`
	LegacyIdentity string            `json:"legacy_identity"`
	EntityID       string            `json:"entity_id"`
	BoundedContext string            `json:"bounded_context"`
	MappingCarrier mappingCarrierDTO `json:"mapping_carrier"`
}

type mappingCarrierDTO struct {
	Ref     string `json:"ref"`
	Edition string `json:"edition"`
	Digest  string `json:"digest"`
}

func mappingCarrierDTOOf(basis MappingCarrierBasis) mappingCarrierDTO {
	return mappingCarrierDTO{
		Ref:     basis.Ref().String(),
		Edition: basis.Edition().String(),
		Digest:  basis.Digest().String(),
	}
}

type typedTargetDTO struct {
	EntityID       string `json:"entity_id"`
	BoundedContext string `json:"bounded_context"`
}

func targetDTOs(values []TypedTarget) []typedTargetDTO {
	result := make([]typedTargetDTO, 0, len(values))
	for _, target := range values {
		result = append(result, typedTargetDTO{
			EntityID:       target.EntityID().String(),
			BoundedContext: target.BoundedContext().String(),
		})
	}
	return result
}

type coalescingIssueDTO struct {
	Kind       string           `json:"kind"`
	LegacyRef  string           `json:"legacy_ref"`
	Candidates []typedTargetDTO `json:"candidates,omitempty"`
	BridgeRefs []string         `json:"bridge_digests"`
}

func canonicalBridges(values []IdentityBridge) []IdentityBridge {
	owned := append([]IdentityBridge(nil), values...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].Digest().String() < owned[right].Digest().String()
	})
	result := make([]IdentityBridge, 0, len(owned))
	for _, bridge := range owned {
		if len(result) > 0 &&
			result[len(result)-1].Digest() == bridge.Digest() {
			continue
		}
		result = append(result, bridge)
	}
	return result
}

func bridgeDigestStrings(values []IdentityBridge) []string {
	result := make([]string, 0, len(values))
	for _, bridge := range values {
		result = append(result, bridge.Digest().String())
	}
	sort.Strings(result)
	return result
}

func digestBytes(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"construct dual-read digest: %w",
			err,
		)
	}
	return digest, nil
}
