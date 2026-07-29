// Package scopedrecall owns pure discovery projections inside one already
// exact EntityOfConcern/context. It cannot resolve an unknown concern, mutate
// typed memory, assert project relations, or grant authority.
package scopedrecall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const RecallUnitSchemaV1 = "haft.recall-unit/v1"

type ExactRecallScope struct {
	entity  typedmemory.PersistedRef
	context typedmemory.BoundedContextRef
	profile neighborhood.ProjectionProfileRef
}

func NewExactRecallScope(
	entity typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
	profile neighborhood.ProjectionProfileRef,
) (ExactRecallScope, error) {
	scope := ExactRecallScope{
		entity:  entity,
		context: context,
		profile: profile,
	}
	if !scope.Valid() {
		return ExactRecallScope{}, fmt.Errorf("exact recall scope is invalid")
	}
	return scope, nil
}

func (scope ExactRecallScope) Entity() typedmemory.PersistedRef {
	return scope.entity
}

func (scope ExactRecallScope) Context() typedmemory.BoundedContextRef {
	return scope.context
}

func (scope ExactRecallScope) ProfileRef() neighborhood.ProjectionProfileRef {
	return scope.profile
}

func (scope ExactRecallScope) Valid() bool {
	if scope.entity.ReferenceID().String() == "" ||
		scope.entity.RefKind().String() == "" {
		return false
	}
	context, contextErr := typedmemory.NewBoundedContextRef(
		scope.context.String(),
	)
	profile, profileFound := neighborhood.LookupProjectionProfile(
		scope.profile,
	)
	return contextErr == nil &&
		context == scope.context &&
		profileFound &&
		profile.Ref() == scope.profile
}

type RecallUnit struct {
	id                    typedmemory.SHA256Digest
	scope                 ExactRecallScope
	snapshot              neighborhood.SnapshotBasis
	projectionBasisDigest typedmemory.SHA256Digest
	facet                 neighborhood.FacetKind
	reference             typedmemory.PersistedRef
	itemKind              neighborhood.ItemKind
	text                  neighborhood.ReadableItemText
	postures              neighborhood.ItemPostures
	provenance            typedmemory.ProvenanceRef
	witnesses             []neighborhood.RelationPathWitness
	contentDigest         typedmemory.SHA256Digest
	projectionSchema      string
}

func (unit RecallUnit) ID() typedmemory.SHA256Digest {
	return unit.id
}

func (unit RecallUnit) Scope() ExactRecallScope {
	return unit.scope
}

func (unit RecallUnit) SnapshotBasis() neighborhood.SnapshotBasis {
	return unit.snapshot
}

func (unit RecallUnit) ProjectionBasisDigest() typedmemory.SHA256Digest {
	return unit.projectionBasisDigest
}

func (unit RecallUnit) Facet() neighborhood.FacetKind {
	return unit.facet
}

func (unit RecallUnit) Reference() typedmemory.PersistedRef {
	return unit.reference
}

func (unit RecallUnit) ItemKind() neighborhood.ItemKind {
	return unit.itemKind
}

func (unit RecallUnit) Text() neighborhood.ReadableItemText {
	return unit.text
}

func (unit RecallUnit) Postures() neighborhood.ItemPostures {
	return unit.postures
}

func (unit RecallUnit) Provenance() typedmemory.ProvenanceRef {
	return unit.provenance
}

func (unit RecallUnit) InclusionWitnesses() []neighborhood.RelationPathWitness {
	return append([]neighborhood.RelationPathWitness{}, unit.witnesses...)
}

func (unit RecallUnit) ContentDigest() typedmemory.SHA256Digest {
	return unit.contentDigest
}

func (unit RecallUnit) ProjectionSchemaVersion() string {
	return unit.projectionSchema
}

func (unit RecallUnit) Valid() bool {
	if !unit.scope.Valid() ||
		!unit.snapshot.Valid() ||
		unit.scope.Entity().RefKind().TypeEnv() != unit.snapshot.TypeEnv() ||
		!unit.facet.Valid() ||
		!unit.itemKind.Valid() ||
		unit.text.String() == "" ||
		!unit.postures.Valid() ||
		len(unit.witnesses) == 0 ||
		unit.projectionSchema == "" {
		return false
	}
	provenance, provenanceErr := typedmemory.NewProvenanceRef(
		unit.provenance.String(),
	)
	content, contentErr := recallUnitContentDigest(unit)
	identity, identityErr := recallUnitIdentityDigest(unit)
	return provenanceErr == nil &&
		provenance == unit.provenance &&
		contentErr == nil &&
		identityErr == nil &&
		content == unit.contentDigest &&
		identity == unit.id
}

func BuildRecallUnits(
	result neighborhood.ExactNeighborhood,
) ([]RecallUnit, error) {
	if !result.Valid() {
		return nil, fmt.Errorf(
			"RecallUnit projection requires an exact neighborhood",
		)
	}
	scope, err := NewExactRecallScope(
		result.ViewContext().Entity(),
		result.ViewContext().Context(),
		result.ViewContext().ProfileRef(),
	)
	if err != nil {
		return nil, err
	}
	units := make([]RecallUnit, 0)
	for _, facet := range result.Facets() {
		for _, item := range facet.Items() {
			unit, buildErr := buildRecallUnit(
				scope,
				result.SnapshotBasis(),
				result.ProjectionBasis(),
				facet.Kind(),
				item,
			)
			if buildErr != nil {
				return nil, buildErr
			}
			units = append(units, unit)
		}
	}
	sort.Slice(units, func(left int, right int) bool {
		return units[left].ID().String() < units[right].ID().String()
	})
	return units, nil
}

func buildRecallUnit(
	scope ExactRecallScope,
	snapshot neighborhood.SnapshotBasis,
	projection neighborhood.ProjectionBasis,
	facet neighborhood.FacetKind,
	item neighborhood.NeighborhoodItem,
) (RecallUnit, error) {
	if _, found := projection.ItemBasisFor(item.Coordinate()); !found {
		return RecallUnit{}, fmt.Errorf(
			"RecallUnit item has no total projection basis",
		)
	}
	unit := RecallUnit{
		scope:                 scope,
		snapshot:              snapshot,
		projectionBasisDigest: projection.Digest(),
		facet:                 facet,
		reference:             item.Reference(),
		itemKind:              item.ItemKind(),
		text:                  item.Text(),
		postures:              item.Postures(),
		provenance:            item.Provenance(),
		witnesses:             item.WhyIncluded(),
		projectionSchema:      projection.ProjectionSchemaVersion(),
	}
	content, err := recallUnitContentDigest(unit)
	if err != nil {
		return RecallUnit{}, err
	}
	unit.contentDigest = content
	identity, err := recallUnitIdentityDigest(unit)
	if err != nil {
		return RecallUnit{}, err
	}
	unit.id = identity
	if !unit.Valid() {
		return RecallUnit{}, fmt.Errorf("built RecallUnit is invalid")
	}
	return unit, nil
}

type recallUnitContentCanonicalV1 struct {
	ReferenceKind string                   `json:"reference_kind"`
	ReferenceID   string                   `json:"reference_id"`
	Facet         string                   `json:"facet"`
	ItemKind      string                   `json:"item_kind"`
	Text          string                   `json:"text"`
	Postures      map[string]string        `json:"postures"`
	Provenance    string                   `json:"provenance_ref"`
	Witnesses     []recallWitnessCanonical `json:"inclusion_witnesses"`
}

type recallUnitIdentityCanonicalV1 struct {
	Schema                string            `json:"schema"`
	Scope                 map[string]string `json:"scope"`
	Snapshot              map[string]any    `json:"snapshot_basis"`
	ProjectionBasisDigest string            `json:"projection_basis_digest"`
	ProjectionSchema      string            `json:"projection_schema_version"`
	ContentDigest         string            `json:"content_digest"`
}

type recallWitnessCanonical struct {
	Assertion         string `json:"assertion_id"`
	Signature         string `json:"signature_id"`
	Context           string `json:"bounded_context_ref"`
	Slot              string `json:"slot_kind_id"`
	TargetKind        string `json:"target_reference_kind"`
	TargetID          string `json:"target_reference_id"`
	Provenance        string `json:"provenance_ref"`
	AdmissionEventRef string `json:"admission_event_ref"`
}

func recallUnitContentDigest(
	unit RecallUnit,
) (typedmemory.SHA256Digest, error) {
	carrier := recallUnitContentCanonicalV1{
		ReferenceKind: unit.reference.RefKind().String(),
		ReferenceID:   unit.reference.ReferenceID().String(),
		Facet:         string(unit.facet),
		ItemKind:      string(unit.itemKind),
		Text:          unit.text.String(),
		Postures: map[string]string{
			"semantic":             string(unit.postures.Semantic()),
			"lifecycle":            string(unit.postures.Lifecycle()),
			"evidence_currentness": string(unit.postures.Evidence()),
			"projection_freshness": string(unit.postures.Projection()),
		},
		Provenance: unit.provenance.String(),
		Witnesses:  encodeRecallWitnesses(unit.witnesses),
	}
	return digestCanonical(carrier)
}

func recallUnitIdentityDigest(
	unit RecallUnit,
) (typedmemory.SHA256Digest, error) {
	carrier := recallUnitIdentityCanonicalV1{
		Schema: RecallUnitSchemaV1,
		Scope: map[string]string{
			"entity_reference_kind": unit.scope.Entity().
				RefKind().
				String(),
			"entity_reference_id": unit.scope.Entity().
				ReferenceID().
				String(),
			"bounded_context_ref": unit.scope.Context().String(),
			"projection_profile_ref": unit.scope.
				ProfileRef().
				String(),
		},
		Snapshot: map[string]any{
			"graph_revision": unit.snapshot.GraphRevision().Value(),
			"type_env_ref":   unit.snapshot.TypeEnv().String(),
			"type_env_digest": unit.snapshot.
				TypeEnvDigest().
				String(),
		},
		ProjectionBasisDigest: unit.projectionBasisDigest.String(),
		ProjectionSchema:      unit.projectionSchema,
		ContentDigest:         unit.contentDigest.String(),
	}
	return digestCanonical(carrier)
}

func encodeRecallWitnesses(
	values []neighborhood.RelationPathWitness,
) []recallWitnessCanonical {
	result := make([]recallWitnessCanonical, 0, len(values))
	for _, value := range values {
		result = append(result, recallWitnessCanonical{
			Assertion:         value.Assertion().String(),
			Signature:         value.Signature().String(),
			Context:           value.Context().String(),
			Slot:              value.Slot().String(),
			TargetKind:        value.Target().RefKind().String(),
			TargetID:          value.Target().ReferenceID().String(),
			Provenance:        value.Provenance().String(),
			AdmissionEventRef: value.AdmissionEventRef(),
		})
	}
	return result
}

func digestCanonical(value any) (typedmemory.SHA256Digest, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode scoped-recall canonical value: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

type ScopedCorpus struct {
	scope    ExactRecallScope
	snapshot neighborhood.SnapshotBasis
	units    []RecallUnit
}

func NewScopedCorpus(
	scope ExactRecallScope,
	snapshot neighborhood.SnapshotBasis,
	allUnits []RecallUnit,
) (ScopedCorpus, error) {
	if !scope.Valid() || !snapshot.Valid() {
		return ScopedCorpus{}, fmt.Errorf(
			"scoped recall corpus coordinate is invalid",
		)
	}
	current := make([]RecallUnit, 0)
	staleFound := false
	for _, unit := range allUnits {
		if !unit.Valid() {
			return ScopedCorpus{}, fmt.Errorf(
				"scoped recall corpus contains invalid unit",
			)
		}
		if unit.Scope() != scope {
			continue
		}
		if unit.SnapshotBasis() != snapshot {
			staleFound = true
			continue
		}
		current = append(current, unit)
	}
	if staleFound {
		return ScopedCorpus{}, StaleCorpusBasis{
			scope:    scope,
			required: snapshot,
		}
	}
	sort.Slice(current, func(left int, right int) bool {
		return current[left].ID().String() < current[right].ID().String()
	})
	return ScopedCorpus{
		scope:    scope,
		snapshot: snapshot,
		units:    current,
	}, nil
}

func (corpus ScopedCorpus) Scope() ExactRecallScope {
	return corpus.scope
}

func (corpus ScopedCorpus) SnapshotBasis() neighborhood.SnapshotBasis {
	return corpus.snapshot
}

func (corpus ScopedCorpus) Units() []RecallUnit {
	return append([]RecallUnit{}, corpus.units...)
}

type StaleCorpusBasis struct {
	scope    ExactRecallScope
	required neighborhood.SnapshotBasis
}

func (basis StaleCorpusBasis) Error() string {
	return "scoped recall corpus contains a stale unit for the exact scope"
}

func (basis StaleCorpusBasis) Scope() ExactRecallScope {
	return basis.scope
}

func (basis StaleCorpusBasis) RequiredSnapshot() neighborhood.SnapshotBasis {
	return basis.required
}
