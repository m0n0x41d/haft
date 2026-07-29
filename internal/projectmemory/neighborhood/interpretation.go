package neighborhood

import (
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// Independent postures must never collapse into one freshness or status field.
type SemanticPosture string
type LifecyclePosture string
type EvidenceCurrentness string
type ProjectionFreshness string

const (
	SemanticTypedActive     SemanticPosture = "typed_active"
	SemanticTypedHistorical SemanticPosture = "typed_historical"
	SemanticInvalid         SemanticPosture = "invalid"
	SemanticUnderdetermined SemanticPosture = "underdetermined"
	SemanticLegacyUnbound   SemanticPosture = "legacy_unbound"
)

const (
	LifecycleActive     LifecyclePosture = "active"
	LifecycleSuperseded LifecyclePosture = "superseded"
	LifecycleRetracted  LifecyclePosture = "retracted"
	LifecycleHistorical LifecyclePosture = "historical"
)

const (
	EvidenceCurrent EvidenceCurrentness = "current"
	EvidenceStale   EvidenceCurrentness = "stale"
	EvidenceExpired EvidenceCurrentness = "expired"
	EvidenceUnknown EvidenceCurrentness = "unknown"
)

const (
	ProjectionCurrent     ProjectionFreshness = "current"
	ProjectionStale       ProjectionFreshness = "stale"
	ProjectionRebuilding  ProjectionFreshness = "rebuilding"
	ProjectionUnavailable ProjectionFreshness = "unavailable"
)

type ItemPostures struct {
	semantic   SemanticPosture
	lifecycle  LifecyclePosture
	evidence   EvidenceCurrentness
	projection ProjectionFreshness
}

func NewItemPostures(
	semantic SemanticPosture,
	lifecycle LifecyclePosture,
	evidence EvidenceCurrentness,
	projection ProjectionFreshness,
) (ItemPostures, bool) {
	postures := ItemPostures{
		semantic:   semantic,
		lifecycle:  lifecycle,
		evidence:   evidence,
		projection: projection,
	}
	return postures, postures.Valid()
}

func (postures ItemPostures) Semantic() SemanticPosture {
	return postures.semantic
}

func (postures ItemPostures) Lifecycle() LifecyclePosture {
	return postures.lifecycle
}

func (postures ItemPostures) Evidence() EvidenceCurrentness {
	return postures.evidence
}

func (postures ItemPostures) Projection() ProjectionFreshness {
	return postures.projection
}

func (postures ItemPostures) Valid() bool {
	return slices.Contains(
		[]SemanticPosture{
			SemanticTypedActive,
			SemanticTypedHistorical,
			SemanticInvalid,
			SemanticUnderdetermined,
			SemanticLegacyUnbound,
		},
		postures.semantic,
	) &&
		slices.Contains(
			[]LifecyclePosture{
				LifecycleActive,
				LifecycleSuperseded,
				LifecycleRetracted,
				LifecycleHistorical,
			},
			postures.lifecycle,
		) &&
		slices.Contains(
			[]EvidenceCurrentness{
				EvidenceCurrent,
				EvidenceStale,
				EvidenceExpired,
				EvidenceUnknown,
			},
			postures.evidence,
		) &&
		slices.Contains(
			[]ProjectionFreshness{
				ProjectionCurrent,
				ProjectionStale,
				ProjectionRebuilding,
				ProjectionUnavailable,
			},
			postures.projection,
		)
}

type StructureInterpretation string
type IdentityInterpretation string
type RelationalRecordsInterpretation string
type RankingInterpretation string
type TruthInterpretation string
type ApplicabilityInterpretation string
type AuthorityInterpretation string
type WorkOrderInterpretation string
type CompletenessInterpretation string

const (
	StructureExactAtSnapshot StructureInterpretation = "exact_at_snapshot"
	StructureDiscoveryOnly   StructureInterpretation = "discovery_only"
	StructureUnavailable     StructureInterpretation = "unavailable"
)

const (
	IdentityExact      IdentityInterpretation = "exact"
	IdentityUnresolved IdentityInterpretation = "unresolved"
)

const (
	RelationalRecordsAssertionsExactAtSnapshot   RelationalRecordsInterpretation = "assertions_exact_at_snapshot"
	RelationalRecordsOccurrencesExactAtSnapshot  RelationalRecordsInterpretation = "occurrences_exact_at_snapshot"
	RelationalRecordsLegacyUnqualifiedAssertions RelationalRecordsInterpretation = "legacy_unqualified_assertions"
	RelationalRecordsCandidateAssertions         RelationalRecordsInterpretation = "candidate_assertions"
	RelationalRecordsHeterogeneous               RelationalRecordsInterpretation = "heterogeneous_relational_records"
	RelationalRecordsUnavailable                 RelationalRecordsInterpretation = "unavailable"
)

// RelationInterpretation and the Relations* constants are source-compatible
// aliases for pre-P12R callers. The public read contract no longer emits the
// ambiguous `relations` field or the old `exact_at_snapshot` value.
//
// Deprecated: use RelationalRecordsInterpretation and the
// RelationalRecords* constants.
type RelationInterpretation = RelationalRecordsInterpretation

const (
	RelationsExactAtSnapshot = RelationalRecordsAssertionsExactAtSnapshot
	RelationsCandidateOnly   = RelationalRecordsCandidateAssertions
	RelationsUnavailable     = RelationalRecordsUnavailable
)

type RelationalRecordItemPostureKind string

const (
	RelationalRecordItemAssertionExact             RelationalRecordItemPostureKind = "assertion_exact"
	RelationalRecordItemOccurrenceExact            RelationalRecordItemPostureKind = "occurrence_exact"
	RelationalRecordItemLegacyUnqualifiedAssertion RelationalRecordItemPostureKind = "legacy_unqualified_assertion"
	RelationalRecordItemCandidateAssertion         RelationalRecordItemPostureKind = "candidate_assertion"
)

// RelationalRecordItemPosture is sealed: callers may inspect a posture derived
// by the kernel, but cannot construct one and thereby upgrade a legacy row or
// candidate into an exact assertion or occurrence.
type RelationalRecordItemPosture struct {
	kind             RelationalRecordItemPostureKind
	explicitModality typedmemory.AssertionModalityKind
}

func (posture RelationalRecordItemPosture) Kind() RelationalRecordItemPostureKind {
	return posture.kind
}

func (posture RelationalRecordItemPosture) Valid() bool {
	switch posture.kind {
	case RelationalRecordItemAssertionExact:
		return validExactAssertionModality(posture.explicitModality)
	case RelationalRecordItemOccurrenceExact,
		RelationalRecordItemLegacyUnqualifiedAssertion,
		RelationalRecordItemCandidateAssertion:
		return posture.explicitModality == ""
	default:
		return false
	}
}

func (posture RelationalRecordItemPosture) ExplicitModality() (
	typedmemory.AssertionModalityKind,
	bool,
) {
	if posture.kind != RelationalRecordItemAssertionExact ||
		!validExactAssertionModality(posture.explicitModality) {
		return "", false
	}
	return posture.explicitModality, true
}

func legacyUnqualifiedAssertionItemPosture() RelationalRecordItemPosture {
	return RelationalRecordItemPosture{
		kind: RelationalRecordItemLegacyUnqualifiedAssertion,
	}
}

func exactAssertionItemPosture(
	modality typedmemory.AssertionModalityKind,
) RelationalRecordItemPosture {
	return RelationalRecordItemPosture{
		kind:             RelationalRecordItemAssertionExact,
		explicitModality: modality,
	}
}

func validExactAssertionModality(
	modality typedmemory.AssertionModalityKind,
) bool {
	switch modality {
	case typedmemory.AssertionModalityAffirmsObtaining,
		typedmemory.AssertionModalityDeniesObtaining,
		typedmemory.AssertionModalityObtainingUnknown:
		return true
	default:
		return false
	}
}

const (
	RankingNotApplicable RankingInterpretation = "not_applicable"
	RankingDiscoveryOnly RankingInterpretation = "discovery_only"
)

const TruthNotImplied TruthInterpretation = "not_implied"
const ApplicabilityNotImplied ApplicabilityInterpretation = "not_implied"
const AuthorityNotGranted AuthorityInterpretation = "not_granted"
const WorkOrderNotImplied WorkOrderInterpretation = "not_implied"

const (
	CompletenessFacetLocal   CompletenessInterpretation = "facet_local"
	CompletenessCandidateSet CompletenessInterpretation = "candidate_set"
	CompletenessUnavailable  CompletenessInterpretation = "unavailable"
)

// InterpretationContract is kernel-derived. No public builder or setter
// exists, so callers cannot turn a candidate, stale read, or missing basis into
// exact structure or authority.
type InterpretationContract struct {
	structure             StructureInterpretation
	identity              IdentityInterpretation
	relationalRecords     RelationalRecordsInterpretation
	ranking               RankingInterpretation
	truth                 TruthInterpretation
	applicability         ApplicabilityInterpretation
	authority             AuthorityInterpretation
	workOrder             WorkOrderInterpretation
	completeness          CompletenessInterpretation
	hydrateBeforeReliance bool
}

func (contract InterpretationContract) Structure() StructureInterpretation {
	return contract.structure
}

func (contract InterpretationContract) Identity() IdentityInterpretation {
	return contract.identity
}

func (contract InterpretationContract) RelationalRecords() RelationalRecordsInterpretation {
	return contract.relationalRecords
}

// Relations preserves source compatibility for Go callers while the canonical
// response uses `relational_records`.
//
// Deprecated: use RelationalRecords.
func (contract InterpretationContract) Relations() RelationInterpretation {
	return contract.RelationalRecords()
}

func (contract InterpretationContract) Ranking() RankingInterpretation {
	return contract.ranking
}

func (contract InterpretationContract) Truth() TruthInterpretation {
	return contract.truth
}

func (contract InterpretationContract) Applicability() ApplicabilityInterpretation {
	return contract.applicability
}

func (contract InterpretationContract) Authority() AuthorityInterpretation {
	return contract.authority
}

func (contract InterpretationContract) WorkOrder() WorkOrderInterpretation {
	return contract.workOrder
}

func (contract InterpretationContract) Completeness() CompletenessInterpretation {
	return contract.completeness
}

func (contract InterpretationContract) HydrateBeforeReliance() bool {
	return contract.hydrateBeforeReliance
}

func (contract InterpretationContract) Valid() bool {
	structureValid := slices.Contains(
		[]StructureInterpretation{
			StructureExactAtSnapshot,
			StructureDiscoveryOnly,
			StructureUnavailable,
		},
		contract.structure,
	)
	identityValid := slices.Contains(
		[]IdentityInterpretation{
			IdentityExact,
			IdentityUnresolved,
		},
		contract.identity,
	)
	relationalRecordsValid := slices.Contains(
		[]RelationalRecordsInterpretation{
			RelationalRecordsAssertionsExactAtSnapshot,
			RelationalRecordsOccurrencesExactAtSnapshot,
			RelationalRecordsLegacyUnqualifiedAssertions,
			RelationalRecordsCandidateAssertions,
			RelationalRecordsHeterogeneous,
			RelationalRecordsUnavailable,
		},
		contract.relationalRecords,
	)
	rankingValid := slices.Contains(
		[]RankingInterpretation{
			RankingNotApplicable,
			RankingDiscoveryOnly,
		},
		contract.ranking,
	)
	completenessValid := slices.Contains(
		[]CompletenessInterpretation{
			CompletenessFacetLocal,
			CompletenessCandidateSet,
			CompletenessUnavailable,
		},
		contract.completeness,
	)
	return structureValid &&
		identityValid &&
		relationalRecordsValid &&
		rankingValid &&
		completenessValid &&
		contract.truth == TruthNotImplied &&
		contract.applicability == ApplicabilityNotImplied &&
		contract.authority == AuthorityNotGranted &&
		contract.workOrder == WorkOrderNotImplied
}

func interpretationForExactNeighborhood(
	postures []ItemPostures,
	relationalPostures []RelationalRecordItemPosture,
) InterpretationContract {
	contract := InterpretationContract{
		structure:             StructureExactAtSnapshot,
		identity:              IdentityExact,
		relationalRecords:     relationalRecordsInterpretationForItems(relationalPostures),
		ranking:               RankingNotApplicable,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessFacetLocal,
		hydrateBeforeReliance: hasRelianceGap(postures),
	}
	return contract
}

func relationalRecordsInterpretationForItems(
	postures []RelationalRecordItemPosture,
) RelationalRecordsInterpretation {
	if len(postures) == 0 {
		return RelationalRecordsUnavailable
	}
	seen := make(map[RelationalRecordItemPostureKind]struct{}, len(postures))
	for _, posture := range postures {
		if !posture.Valid() {
			return RelationalRecordsUnavailable
		}
		seen[posture.Kind()] = struct{}{}
	}
	if len(seen) > 1 {
		return RelationalRecordsHeterogeneous
	}
	for posture := range seen {
		return relationalRecordsInterpretationForItem(posture)
	}
	return RelationalRecordsUnavailable
}

func relationalRecordsInterpretationForItem(
	posture RelationalRecordItemPostureKind,
) RelationalRecordsInterpretation {
	values := map[RelationalRecordItemPostureKind]RelationalRecordsInterpretation{
		RelationalRecordItemAssertionExact:             RelationalRecordsAssertionsExactAtSnapshot,
		RelationalRecordItemOccurrenceExact:            RelationalRecordsOccurrencesExactAtSnapshot,
		RelationalRecordItemLegacyUnqualifiedAssertion: RelationalRecordsLegacyUnqualifiedAssertions,
		RelationalRecordItemCandidateAssertion:         RelationalRecordsCandidateAssertions,
	}
	value, found := values[posture]
	if !found {
		return RelationalRecordsUnavailable
	}
	return value
}

func interpretationForRetryOrAbstention() InterpretationContract {
	return InterpretationContract{
		structure:             StructureUnavailable,
		identity:              IdentityUnresolved,
		relationalRecords:     RelationalRecordsUnavailable,
		ranking:               RankingNotApplicable,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessUnavailable,
		hydrateBeforeReliance: true,
	}
}

// InterpretationForScopedCandidates is kernel-derived for recall inside one
// exact EntityOfConcern/context. Identity is exact, while ranked relations
// remain discovery candidates and require hydration before reliance.
func InterpretationForScopedCandidates() InterpretationContract {
	return InterpretationContract{
		structure:             StructureDiscoveryOnly,
		identity:              IdentityExact,
		relationalRecords:     RelationalRecordsCandidateAssertions,
		ranking:               RankingDiscoveryOnly,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessCandidateSet,
		hydrateBeforeReliance: true,
	}
}

// InterpretationForEntityCandidates is kernel-derived for unresolved identity
// discovery. It cannot contain or imply current project relations.
func InterpretationForEntityCandidates() InterpretationContract {
	return InterpretationContract{
		structure:             StructureDiscoveryOnly,
		identity:              IdentityUnresolved,
		relationalRecords:     RelationalRecordsUnavailable,
		ranking:               RankingDiscoveryOnly,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessCandidateSet,
		hydrateBeforeReliance: true,
	}
}

// InterpretationForExactEntityResolution says only that identity was resolved
// inside one exact snapshot-bound index scope. It returns no current project
// relations, truth, applicability, authority, or Work order.
func InterpretationForExactEntityResolution() InterpretationContract {
	return InterpretationContract{
		structure:             StructureDiscoveryOnly,
		identity:              IdentityExact,
		relationalRecords:     RelationalRecordsUnavailable,
		ranking:               RankingNotApplicable,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessCandidateSet,
		hydrateBeforeReliance: true,
	}
}

// InterpretationForKnownAbsent limits absence to the named complete
// resolution indexes and snapshot. It does not assert global non-existence.
func InterpretationForKnownAbsent() InterpretationContract {
	return InterpretationContract{
		structure:             StructureDiscoveryOnly,
		identity:              IdentityUnresolved,
		relationalRecords:     RelationalRecordsUnavailable,
		ranking:               RankingNotApplicable,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessCandidateSet,
		hydrateBeforeReliance: true,
	}
}

// InterpretationForReadAbstention is shared by separate read-result families
// without exposing a caller-settable InterpretationContract builder.
func InterpretationForReadAbstention() InterpretationContract {
	return interpretationForRetryOrAbstention()
}

func hasRelianceGap(postures []ItemPostures) bool {
	for _, posture := range postures {
		if !posture.Valid() ||
			posture.semantic != SemanticTypedActive ||
			posture.lifecycle != LifecycleActive ||
			posture.evidence != EvidenceCurrent ||
			posture.projection != ProjectionCurrent {
			return true
		}
	}
	return false
}
