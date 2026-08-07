// Package architecturep2s owns the pure, read-only P12A projection. It keeps
// architecture descriptions, project records, performed Work claims, actual
// change, production claims, observed structure, evaluation, and target
// effect in separate positions. The package grants no persistence, authority,
// selection, or public-memory capability.
package architecturep2s

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const readModelSchemaV1 = "haft.architecture-p2s-read-model/v1"

type PositionKind string

const (
	PositionHolonPosture            PositionKind = "holon_posture"
	PositionProblemPressure         PositionKind = "problem_pressure"
	PositionArchitectureDescription PositionKind = "architecture_description"
	PositionSelectedStructure       PositionKind = "selected_structure"
	PositionArchitectureCandidate   PositionKind = "architecture_candidate"
	PositionAlternatives            PositionKind = "alternatives"
	PositionComparison              PositionKind = "comparison"
	PositionDecision                PositionKind = "decision"
	PositionExpectedStructure       PositionKind = "expected_structure"
	PositionWorkRecord              PositionKind = "work_record"
	PositionPerformedWork           PositionKind = "performed_work"
	PositionActualChange            PositionKind = "actual_change"
	PositionWorkToChange            PositionKind = "work_to_change"
	PositionProductionWork          PositionKind = "production_work"
	PositionEntityInception         PositionKind = "entity_inception"
	PositionProductionCompletion    PositionKind = "production_completion"
	PositionActualStructure         PositionKind = "actual_structure"
	PositionStructureDescription    PositionKind = "structure_description"
	PositionStructureEvaluation     PositionKind = "structure_evaluation"
	PositionGrounding               PositionKind = "grounding"
	PositionEvidence                PositionKind = "evidence"
	PositionConformance             PositionKind = "conformance"
	PositionTargetEffect            PositionKind = "target_effect"
)

var positionKinds = []PositionKind{
	PositionHolonPosture,
	PositionProblemPressure,
	PositionArchitectureDescription,
	PositionSelectedStructure,
	PositionArchitectureCandidate,
	PositionAlternatives,
	PositionComparison,
	PositionDecision,
	PositionExpectedStructure,
	PositionWorkRecord,
	PositionPerformedWork,
	PositionActualChange,
	PositionWorkToChange,
	PositionProductionWork,
	PositionEntityInception,
	PositionProductionCompletion,
	PositionActualStructure,
	PositionStructureDescription,
	PositionStructureEvaluation,
	PositionGrounding,
	PositionEvidence,
	PositionConformance,
	PositionTargetEffect,
}

func PositionKinds() []PositionKind {
	return append([]PositionKind(nil), positionKinds...)
}

func (kind PositionKind) valid() bool {
	for _, candidate := range positionKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

type ResolutionKind string

const (
	ResolutionDirectClaim     ResolutionKind = "direct_claim"
	ResolutionMissing         ResolutionKind = "missing"
	ResolutionNotApplicable   ResolutionKind = "not_applicable"
	ResolutionUnderdetermined ResolutionKind = "underdetermined"
)

type ProjectionBasis struct {
	project         string
	entityOfConcern Reference
	context         string
	typeEnv         string
	graphSnapshot   string
	graphRevision   uint64
}

type ProjectionBasisInput struct {
	Project         string
	EntityOfConcern Reference
	Context         string
	TypeEnv         string
	GraphSnapshot   string
	GraphRevision   uint64
}

func NewProjectionBasis(input ProjectionBasisInput) (ProjectionBasis, error) {
	if input.Project == "" ||
		!input.EntityOfConcern.valid() ||
		input.Context == "" ||
		input.TypeEnv == "" ||
		input.GraphSnapshot == "" {
		return ProjectionBasis{}, fmt.Errorf(
			"architecture P2S projection basis is incomplete",
		)
	}
	return ProjectionBasis{
		project:         input.Project,
		entityOfConcern: input.EntityOfConcern,
		context:         input.Context,
		typeEnv:         input.TypeEnv,
		graphSnapshot:   input.GraphSnapshot,
		graphRevision:   input.GraphRevision,
	}, nil
}

func (basis ProjectionBasis) Project() string { return basis.project }

func (basis ProjectionBasis) EntityOfConcern() Reference {
	return basis.entityOfConcern
}

func (basis ProjectionBasis) Context() string { return basis.context }

func (basis ProjectionBasis) TypeEnv() string { return basis.typeEnv }

func (basis ProjectionBasis) GraphSnapshot() string {
	return basis.graphSnapshot
}

func (basis ProjectionBasis) GraphRevision() uint64 {
	return basis.graphRevision
}

func (basis ProjectionBasis) valid() bool {
	_, err := NewProjectionBasis(ProjectionBasisInput{
		Project:         basis.project,
		EntityOfConcern: basis.entityOfConcern,
		Context:         basis.context,
		TypeEnv:         basis.typeEnv,
		GraphSnapshot:   basis.graphSnapshot,
		GraphRevision:   basis.graphRevision,
	})
	return err == nil
}

type Reference struct {
	kind string
	id   string
}

func NewReference(kind string, id string) (Reference, error) {
	if kind == "" || id == "" {
		return Reference{}, fmt.Errorf(
			"architecture P2S reference requires kind and identity",
		)
	}
	return Reference{kind: kind, id: id}, nil
}

func (reference Reference) Kind() string { return reference.kind }

func (reference Reference) ID() string { return reference.id }

func (reference Reference) Key() string {
	return reference.kind + "|" + reference.id
}

func (reference Reference) valid() bool {
	_, err := NewReference(reference.kind, reference.id)
	return err == nil
}

type SourceReturn struct {
	patternID       string
	returnCondition string
}

func NewSourceReturn(
	patternID string,
	returnCondition string,
) (SourceReturn, error) {
	if patternID == "" || returnCondition == "" {
		return SourceReturn{}, fmt.Errorf(
			"architecture P2S source return requires pattern and condition",
		)
	}
	return SourceReturn{
		patternID:       patternID,
		returnCondition: returnCondition,
	}, nil
}

func (source SourceReturn) PatternID() string { return source.patternID }

func (source SourceReturn) ReturnCondition() string {
	return source.returnCondition
}

func (source SourceReturn) valid() bool {
	_, err := NewSourceReturn(source.patternID, source.returnCondition)
	return err == nil
}

type ClaimModality string

const (
	ClaimAffirmsObtaining  ClaimModality = "affirms_obtaining"
	ClaimDeniesObtaining   ClaimModality = "denies_obtaining"
	ClaimObtainingUnknown  ClaimModality = "obtaining_unknown"
	ClaimLegacyUnqualified ClaimModality = "legacy_unqualified_assertion"
)

func (modality ClaimModality) valid() bool {
	switch modality {
	case ClaimAffirmsObtaining,
		ClaimDeniesObtaining,
		ClaimObtainingUnknown,
		ClaimLegacyUnqualified:
		return true
	default:
		return false
	}
}

// ClaimWitness is one exact direct local claim. It remains an assertion
// witness: even AffirmsObtaining does not become truth, an occurrence, or an
// authority receipt in this read model.
type ClaimWitness struct {
	assertionID string
	signature   string
	modality    ClaimModality
	patternID   string
	provenance  string
	originEvent string
	references  []Reference
}

type ClaimWitnessInput struct {
	AssertionID string
	Signature   string
	Modality    ClaimModality
	PatternID   string
	Provenance  string
	OriginEvent string
	References  []Reference
}

func NewClaimWitness(input ClaimWitnessInput) (ClaimWitness, error) {
	references, err := canonicalReferences(input.References)
	if err != nil {
		return ClaimWitness{}, err
	}
	if input.AssertionID == "" ||
		input.Signature == "" ||
		!input.Modality.valid() ||
		input.PatternID == "" ||
		input.Provenance == "" ||
		input.OriginEvent == "" ||
		len(references) == 0 {
		return ClaimWitness{}, fmt.Errorf(
			"architecture P2S claim witness is incomplete",
		)
	}
	return ClaimWitness{
		assertionID: input.AssertionID,
		signature:   input.Signature,
		modality:    input.Modality,
		patternID:   input.PatternID,
		provenance:  input.Provenance,
		originEvent: input.OriginEvent,
		references:  references,
	}, nil
}

func (witness ClaimWitness) AssertionID() string { return witness.assertionID }

func (witness ClaimWitness) Signature() string { return witness.signature }

func (witness ClaimWitness) Modality() ClaimModality { return witness.modality }

func (witness ClaimWitness) PatternID() string { return witness.patternID }

func (witness ClaimWitness) Provenance() string { return witness.provenance }

func (witness ClaimWitness) OriginEvent() string { return witness.originEvent }

func (witness ClaimWitness) References() []Reference {
	return append([]Reference(nil), witness.references...)
}

func (witness ClaimWitness) key() string {
	return witness.assertionID + "|" + witness.signature + "|" + witness.patternID
}

type SourceDock struct {
	assertionID string
	signature   string
	provenance  string
	originEvent string
	references  []Reference
}

type SourceDockInput struct {
	AssertionID string
	Signature   string
	Provenance  string
	OriginEvent string
	References  []Reference
}

func NewSourceDock(input SourceDockInput) (SourceDock, error) {
	references, err := canonicalReferences(input.References)
	if err != nil {
		return SourceDock{}, err
	}
	if input.AssertionID == "" ||
		input.Signature == "" ||
		input.Provenance == "" ||
		input.OriginEvent == "" ||
		len(references) == 0 {
		return SourceDock{}, fmt.Errorf(
			"architecture P2S source dock is incomplete",
		)
	}
	return SourceDock{
		assertionID: input.AssertionID,
		signature:   input.Signature,
		provenance:  input.Provenance,
		originEvent: input.OriginEvent,
		references:  references,
	}, nil
}

func (dock SourceDock) AssertionID() string { return dock.assertionID }

func (dock SourceDock) Signature() string { return dock.signature }

func (dock SourceDock) Provenance() string { return dock.provenance }

func (dock SourceDock) OriginEvent() string { return dock.originEvent }

func (dock SourceDock) References() []Reference {
	return append([]Reference(nil), dock.references...)
}

func (dock SourceDock) key() string {
	return dock.assertionID + "|" + dock.signature
}

type Position interface {
	Kind() PositionKind
	Resolution() ResolutionKind
	SourceReturn() SourceReturn
	positionVariant()
	canonical() canonicalPosition
}

type DirectClaimPosition struct {
	kind   PositionKind
	source SourceReturn
	claims []ClaimWitness
	docks  []SourceDock
}

func NewDirectClaimPosition(
	kind PositionKind,
	source SourceReturn,
	claims []ClaimWitness,
	docks []SourceDock,
) (DirectClaimPosition, error) {
	canonicalClaims, err := canonicalClaimWitnesses(claims)
	if err != nil {
		return DirectClaimPosition{}, err
	}
	canonicalDocks, err := canonicalSourceDocks(docks)
	if err != nil {
		return DirectClaimPosition{}, err
	}
	if !kind.valid() || !source.valid() || len(canonicalClaims) == 0 {
		return DirectClaimPosition{}, fmt.Errorf(
			"direct architecture P2S position is incomplete",
		)
	}
	return DirectClaimPosition{
		kind:   kind,
		source: source,
		claims: canonicalClaims,
		docks:  canonicalDocks,
	}, nil
}

func (position DirectClaimPosition) Kind() PositionKind { return position.kind }

func (DirectClaimPosition) Resolution() ResolutionKind {
	return ResolutionDirectClaim
}

func (position DirectClaimPosition) SourceReturn() SourceReturn {
	return position.source
}

func (position DirectClaimPosition) Claims() []ClaimWitness {
	return append([]ClaimWitness(nil), position.claims...)
}

func (position DirectClaimPosition) SourceDocks() []SourceDock {
	return append([]SourceDock(nil), position.docks...)
}

func (DirectClaimPosition) positionVariant() {}

func (position DirectClaimPosition) canonical() canonicalPosition {
	return canonicalPositionFromParts(
		position.kind,
		ResolutionDirectClaim,
		position.source,
		position.claims,
		position.docks,
		"",
		"",
	)
}

type MissingPosition struct {
	kind   PositionKind
	source SourceReturn
	docks  []SourceDock
}

func NewMissingPosition(
	kind PositionKind,
	source SourceReturn,
	docks []SourceDock,
) (MissingPosition, error) {
	canonicalDocks, err := canonicalSourceDocks(docks)
	if err != nil {
		return MissingPosition{}, err
	}
	if !kind.valid() || !source.valid() {
		return MissingPosition{}, fmt.Errorf(
			"missing architecture P2S position is incomplete",
		)
	}
	return MissingPosition{kind: kind, source: source, docks: canonicalDocks}, nil
}

func (position MissingPosition) Kind() PositionKind { return position.kind }

func (MissingPosition) Resolution() ResolutionKind { return ResolutionMissing }

func (position MissingPosition) SourceReturn() SourceReturn {
	return position.source
}

func (position MissingPosition) SourceDocks() []SourceDock {
	return append([]SourceDock(nil), position.docks...)
}

func (MissingPosition) positionVariant() {}

func (position MissingPosition) canonical() canonicalPosition {
	return canonicalPositionFromParts(
		position.kind,
		ResolutionMissing,
		position.source,
		nil,
		position.docks,
		"",
		"",
	)
}

type NotApplicablePosition struct {
	kind     PositionKind
	source   SourceReturn
	basisRef string
	reason   string
}

func NewNotApplicablePosition(
	kind PositionKind,
	source SourceReturn,
	basisRef string,
	reason string,
) (NotApplicablePosition, error) {
	if !kind.valid() || !source.valid() || basisRef == "" || reason == "" {
		return NotApplicablePosition{}, fmt.Errorf(
			"not-applicable architecture P2S position is incomplete",
		)
	}
	return NotApplicablePosition{
		kind:     kind,
		source:   source,
		basisRef: basisRef,
		reason:   reason,
	}, nil
}

func (position NotApplicablePosition) Kind() PositionKind {
	return position.kind
}

func (NotApplicablePosition) Resolution() ResolutionKind {
	return ResolutionNotApplicable
}

func (position NotApplicablePosition) SourceReturn() SourceReturn {
	return position.source
}

func (position NotApplicablePosition) BasisRef() string {
	return position.basisRef
}

func (position NotApplicablePosition) Reason() string { return position.reason }

func (NotApplicablePosition) positionVariant() {}

func (position NotApplicablePosition) canonical() canonicalPosition {
	return canonicalPositionFromParts(
		position.kind,
		ResolutionNotApplicable,
		position.source,
		nil,
		nil,
		position.reason,
		position.basisRef,
	)
}

type UnderdeterminedPosition struct {
	kind       PositionKind
	source     SourceReturn
	reason     string
	candidates []ClaimWitness
	docks      []SourceDock
}

func NewUnderdeterminedPosition(
	kind PositionKind,
	source SourceReturn,
	reason string,
	candidates []ClaimWitness,
	docks []SourceDock,
) (UnderdeterminedPosition, error) {
	canonicalCandidates, err := canonicalClaimWitnesses(candidates)
	if err != nil {
		return UnderdeterminedPosition{}, err
	}
	canonicalDocks, err := canonicalSourceDocks(docks)
	if err != nil {
		return UnderdeterminedPosition{}, err
	}
	if !kind.valid() || !source.valid() || reason == "" {
		return UnderdeterminedPosition{}, fmt.Errorf(
			"underdetermined architecture P2S position is incomplete",
		)
	}
	return UnderdeterminedPosition{
		kind:       kind,
		source:     source,
		reason:     reason,
		candidates: canonicalCandidates,
		docks:      canonicalDocks,
	}, nil
}

func (position UnderdeterminedPosition) Kind() PositionKind {
	return position.kind
}

func (UnderdeterminedPosition) Resolution() ResolutionKind {
	return ResolutionUnderdetermined
}

func (position UnderdeterminedPosition) SourceReturn() SourceReturn {
	return position.source
}

func (position UnderdeterminedPosition) Reason() string {
	return position.reason
}

func (position UnderdeterminedPosition) Candidates() []ClaimWitness {
	return append([]ClaimWitness(nil), position.candidates...)
}

func (position UnderdeterminedPosition) SourceDocks() []SourceDock {
	return append([]SourceDock(nil), position.docks...)
}

func (UnderdeterminedPosition) positionVariant() {}

func (position UnderdeterminedPosition) canonical() canonicalPosition {
	return canonicalPositionFromParts(
		position.kind,
		ResolutionUnderdetermined,
		position.source,
		position.candidates,
		position.docks,
		position.reason,
		"",
	)
}

type ReadModel struct {
	basis          ProjectionBasis
	positions      []Position
	canonicalBytes []byte
	digest         string
}

func NewReadModel(
	basis ProjectionBasis,
	positions []Position,
) (ReadModel, error) {
	if !basis.valid() {
		return ReadModel{}, fmt.Errorf(
			"architecture P2S read model basis is invalid",
		)
	}
	canonicalPositions, err := canonicalPositions(positions)
	if err != nil {
		return ReadModel{}, err
	}
	canonical, err := encodeReadModel(basis, canonicalPositions)
	if err != nil {
		return ReadModel{}, err
	}
	digestBytes := sha256.Sum256(canonical)
	digest := "sha256:" + hex.EncodeToString(digestBytes[:])
	return ReadModel{
		basis:          basis,
		positions:      canonicalPositions,
		canonicalBytes: canonical,
		digest:         digest,
	}, nil
}

func (model ReadModel) Basis() ProjectionBasis { return model.basis }

func (model ReadModel) Positions() []Position {
	return append([]Position(nil), model.positions...)
}

func (model ReadModel) Position(kind PositionKind) (Position, bool) {
	for _, position := range model.positions {
		if position.Kind() == kind {
			return position, true
		}
	}
	return nil, false
}

func (model ReadModel) CanonicalBytes() []byte {
	return append([]byte(nil), model.canonicalBytes...)
}

func (model ReadModel) Digest() string { return model.digest }

type canonicalReadModel struct {
	Schema    string              `json:"schema"`
	Basis     canonicalBasis      `json:"basis"`
	Positions []canonicalPosition `json:"positions"`
}

type canonicalBasis struct {
	Project         string             `json:"project"`
	EntityOfConcern canonicalReference `json:"entity_of_concern"`
	Context         string             `json:"context"`
	TypeEnv         string             `json:"type_env"`
	GraphSnapshot   string             `json:"graph_snapshot"`
	GraphRevision   uint64             `json:"graph_revision"`
}

type canonicalReference struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type canonicalClaim struct {
	AssertionID string               `json:"assertion_id"`
	Signature   string               `json:"relation_declaration_fragment"`
	Modality    ClaimModality        `json:"modality"`
	PatternID   string               `json:"pattern_id"`
	Provenance  string               `json:"provenance"`
	OriginEvent string               `json:"origin_event"`
	References  []canonicalReference `json:"references"`
}

type canonicalDock struct {
	AssertionID string               `json:"assertion_id"`
	Signature   string               `json:"relation_declaration_fragment"`
	Provenance  string               `json:"provenance"`
	OriginEvent string               `json:"origin_event"`
	References  []canonicalReference `json:"references"`
}

type canonicalPosition struct {
	Kind            PositionKind     `json:"kind"`
	Resolution      ResolutionKind   `json:"resolution"`
	PatternID       string           `json:"pattern_id"`
	ReturnCondition string           `json:"return_condition"`
	Claims          []canonicalClaim `json:"claims,omitempty"`
	SourceDocks     []canonicalDock  `json:"source_docks,omitempty"`
	Reason          string           `json:"reason,omitempty"`
	BasisRef        string           `json:"basis_ref,omitempty"`
}

func canonicalPositionFromParts(
	kind PositionKind,
	resolution ResolutionKind,
	source SourceReturn,
	claims []ClaimWitness,
	docks []SourceDock,
	reason string,
	basisRef string,
) canonicalPosition {
	return canonicalPosition{
		Kind:            kind,
		Resolution:      resolution,
		PatternID:       source.PatternID(),
		ReturnCondition: source.ReturnCondition(),
		Claims:          canonicalClaims(claims),
		SourceDocks:     canonicalDocks(docks),
		Reason:          reason,
		BasisRef:        basisRef,
	}
}

func canonicalClaims(values []ClaimWitness) []canonicalClaim {
	result := make([]canonicalClaim, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalClaim{
			AssertionID: value.AssertionID(),
			Signature:   value.Signature(),
			Modality:    value.Modality(),
			PatternID:   value.PatternID(),
			Provenance:  value.Provenance(),
			OriginEvent: value.OriginEvent(),
			References:  canonicalReferenceValues(value.References()),
		})
	}
	return result
}

func canonicalDocks(values []SourceDock) []canonicalDock {
	result := make([]canonicalDock, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalDock{
			AssertionID: value.AssertionID(),
			Signature:   value.Signature(),
			Provenance:  value.Provenance(),
			OriginEvent: value.OriginEvent(),
			References:  canonicalReferenceValues(value.References()),
		})
	}
	return result
}

func canonicalReferenceValues(values []Reference) []canonicalReference {
	result := make([]canonicalReference, 0, len(values))
	for _, value := range values {
		result = append(result, canonicalReference{
			Kind: value.Kind(),
			ID:   value.ID(),
		})
	}
	return result
}

func canonicalReferences(values []Reference) ([]Reference, error) {
	result := append([]Reference(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Key() < result[right].Key()
	})
	for index, value := range result {
		if !value.valid() {
			return nil, fmt.Errorf(
				"architecture P2S reference %d is invalid",
				index,
			)
		}
		if index > 0 && result[index-1].Key() == value.Key() {
			return nil, fmt.Errorf(
				"architecture P2S references repeat %q",
				value.Key(),
			)
		}
	}
	return result, nil
}

func canonicalClaimWitnesses(
	values []ClaimWitness,
) ([]ClaimWitness, error) {
	result := append([]ClaimWitness(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	for index, value := range result {
		_, err := NewClaimWitness(ClaimWitnessInput{
			AssertionID: value.assertionID,
			Signature:   value.signature,
			Modality:    value.modality,
			PatternID:   value.patternID,
			Provenance:  value.provenance,
			OriginEvent: value.originEvent,
			References:  value.references,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"architecture P2S claim witness %d: %w",
				index,
				err,
			)
		}
		if index > 0 && result[index-1].key() == value.key() {
			return nil, fmt.Errorf(
				"architecture P2S claim witnesses repeat %q",
				value.key(),
			)
		}
	}
	return result, nil
}

func canonicalSourceDocks(values []SourceDock) ([]SourceDock, error) {
	result := append([]SourceDock(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	for index, value := range result {
		_, err := NewSourceDock(SourceDockInput{
			AssertionID: value.assertionID,
			Signature:   value.signature,
			Provenance:  value.provenance,
			OriginEvent: value.originEvent,
			References:  value.references,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"architecture P2S source dock %d: %w",
				index,
				err,
			)
		}
		if index > 0 && result[index-1].key() == value.key() {
			return nil, fmt.Errorf(
				"architecture P2S source docks repeat %q",
				value.key(),
			)
		}
	}
	return result, nil
}

func canonicalPositions(values []Position) ([]Position, error) {
	if len(values) != len(positionKinds) {
		return nil, fmt.Errorf(
			"architecture P2S read model requires exactly %d positions",
			len(positionKinds),
		)
	}
	result := append([]Position(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].Kind() < result[right].Kind()
	})
	for index, position := range result {
		if position == nil || !position.Kind().valid() {
			return nil, fmt.Errorf(
				"architecture P2S position %d is invalid",
				index,
			)
		}
		if index > 0 && result[index-1].Kind() == position.Kind() {
			return nil, fmt.Errorf(
				"architecture P2S positions repeat %q",
				position.Kind(),
			)
		}
	}
	return result, nil
}

func encodeReadModel(
	basis ProjectionBasis,
	positions []Position,
) ([]byte, error) {
	canonicalPositions := make(
		[]canonicalPosition,
		0,
		len(positions),
	)
	for _, position := range positions {
		canonicalPositions = append(
			canonicalPositions,
			position.canonical(),
		)
	}
	payload := canonicalReadModel{
		Schema: readModelSchemaV1,
		Basis: canonicalBasis{
			Project: basis.Project(),
			EntityOfConcern: canonicalReference{
				Kind: basis.EntityOfConcern().Kind(),
				ID:   basis.EntityOfConcern().ID(),
			},
			Context:       basis.Context(),
			TypeEnv:       basis.TypeEnv(),
			GraphSnapshot: basis.GraphSnapshot(),
			GraphRevision: basis.GraphRevision(),
		},
		Positions: canonicalPositions,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(
			"encode architecture P2S read model: %w",
			err,
		)
	}
	return encoded, nil
}
