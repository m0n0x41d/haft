package fpf

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

type QueryMode string

const (
	QueryModeConcern QueryMode = "concern"
	QueryModeLookup  QueryMode = "lookup"
	QueryModeInspect QueryMode = "inspect"
)

// SourceUnitRole is a publication role, not a search tier. Role order never
// implies pattern applicability, priority, causal order, or work order.
type SourceUnitRole string

const (
	SourceUnitRolePracticalUseCard SourceUnitRole = "practical_use_card"
	SourceUnitRolePreface          SourceUnitRole = "preface"
	SourceUnitRoleTOCRow           SourceUnitRole = "toc_row"
	SourceUnitRolePatternBody      SourceUnitRole = "pattern_body"
	SourceUnitRolePatternSection   SourceUnitRole = "pattern_section"
	SourceUnitRolePatternScope     SourceUnitRole = "pattern_scope"
)

var sourceUnitRoleOrder = []SourceUnitRole{
	SourceUnitRolePracticalUseCard,
	SourceUnitRolePreface,
	SourceUnitRoleTOCRow,
	SourceUnitRolePatternBody,
	SourceUnitRolePatternSection,
	SourceUnitRolePatternScope,
}

type SourceProvenance struct {
	SourcePath     string `json:"source_path"`
	StartLine      int    `json:"start_line"`
	EndLine        int    `json:"end_line"`
	ContentHash    string `json:"content_hash"`
	SourceRevision string `json:"source_revision"`
}

// SourceRelationKind is copied from an explicit relation label in the
// upstream FPF ToC. It does not imply transitive closure, applicability,
// causal order, or work order.
type SourceRelationKind string

const (
	SourceRelationKindBuildsOn        SourceRelationKind = "builds_on"
	SourceRelationKindPrerequisiteFor SourceRelationKind = "prerequisite_for"
	SourceRelationKindCoordinatesWith SourceRelationKind = "coordinates_with"
	SourceRelationKindConstrains      SourceRelationKind = "constrains"
	SourceRelationKindInforms         SourceRelationKind = "informs"
	SourceRelationKindUsedBy          SourceRelationKind = "used_by"
	SourceRelationKindRefines         SourceRelationKind = "refines"
	SourceRelationKindSpecialisedBy   SourceRelationKind = "specialised_by"
)

func isSourceRelationKind(kind SourceRelationKind) bool {
	switch kind {
	case SourceRelationKindBuildsOn,
		SourceRelationKindPrerequisiteFor,
		SourceRelationKindCoordinatesWith,
		SourceRelationKindConstrains,
		SourceRelationKindInforms,
		SourceRelationKindUsedBy,
		SourceRelationKindRefines,
		SourceRelationKindSpecialisedBy:
		return true
	default:
		return false
	}
}

type SourceRelationOrigin string

const SourceRelationOriginTOCExplicit SourceRelationOrigin = "toc_explicit_relation"

type SourceRelationTargetClass string

const (
	SourceRelationTargetClassLocalPattern       SourceRelationTargetClass = "local_pattern"
	SourceRelationTargetClassAuthoredNonlocal   SourceRelationTargetClass = "authored_nonlocal_reference"
	SourceRelationTargetClassUnresolvedAuthored SourceRelationTargetClass = "unresolved_authored_reference"
)

// SourceRelation is a derived cross-publication projection: the canonical
// pattern_body owns the relation, while Provenance continues to point at the
// exact ToC row that authored it. Subject identity is the owning UnitID;
// TargetPatternID remains stable when a planned ToC row gains a body.
type SourceRelation struct {
	Kind            SourceRelationKind        `json:"kind"`
	TargetPatternID string                    `json:"target_pattern_id"`
	TargetClass     SourceRelationTargetClass `json:"target_class"`
	Origin          SourceRelationOrigin      `json:"origin"`
	Provenance      SourceProvenance          `json:"provenance"`
}

// SourceUnit is derived from upstream FPF publications. AuthoredPhrases and
// Keywords are copied or structurally extracted from those publications; they
// are not Haft-authored routes.
type SourceUnit struct {
	UnitID            string           `json:"unit_id"`
	SourceID          string           `json:"source_id,omitempty"`
	Role              SourceUnitRole   `json:"source_role"`
	Title             string           `json:"title"`
	Body              string           `json:"body"`
	PatternID         string           `json:"pattern_id,omitempty"`
	ParentPatternID   string           `json:"parent_pattern_id,omitempty"`
	PublicationStatus string           `json:"publication_status,omitempty"`
	DirectRefs        []string         `json:"direct_refs,omitempty"`
	Relations         []SourceRelation `json:"relations,omitempty"`
	AuthoredPhrases   []string         `json:"authored_phrases,omitempty"`
	Keywords          []string         `json:"keywords,omitempty"`
	UseCues           SourceUseCues    `json:"use_cues,omitempty"`
	Provenance        SourceProvenance `json:"provenance"`
}

// SourceUseCues preserves source-owned practical-use distinctions. Empty
// fields mean that the source role does not publish that cue.
type SourceUseCues struct {
	ConditionText   string `json:"condition_text,omitempty"`
	FirstResultText string `json:"first_result_text,omitempty"`
	StopReturnText  string `json:"stop_return_text,omitempty"`
}

// ResponseBudget bounds carrier volume without truncating an exact SourceUnit.
// MaxExcerptCharacters is the strict per-candidate total source-text budget
// across the excerpt and projected practical-use cues. A zero field means the
// operator did not constrain that dimension.
type ResponseBudget struct {
	MaxCandidatesPerRole     int `json:"max_candidates_per_role,omitempty"`
	MaxTotalCandidates       int `json:"max_total_candidates,omitempty"`
	MaxExcerptCharacters     int `json:"max_excerpt_characters,omitempty"`
	MaxRelationsPerCandidate int `json:"max_relations_per_candidate,omitempty"`
}

// CandidateProbe preserves the operator concern and optional contextual
// distinctions. The index uses them only to produce candidates.
type CandidateProbe struct {
	Text            string
	EntityOfConcern string
	KnownContext    []string
	IntendedUse     string
}

// SourceLexemeFrequencies is source-corpus evidence used to distinguish
// discriminative query terms from ubiquitous question scaffolding. The index
// reports facts; the pure query core owns the sufficiency threshold.
type SourceLexemeFrequencies struct {
	TotalSourceUnits  int
	DocumentFrequency map[string]int
}

// SourceProbePhrase preserves whether source evidence matched the operator's
// exact normalized probe or a weaker phrase compressed by removing generic
// question scaffold. The two kinds have different witness thresholds.
type SourceProbePhrase struct {
	ProbeField string
	Value      string
	Kind       SourcePhraseKind
}

type SourcePhraseKind string

const (
	SourcePhraseKindExactProbeSpan     SourcePhraseKind = "exact_probe_span"
	SourcePhraseKindScaffoldCompressed SourcePhraseKind = "scaffold_compressed"
)

// QueryRequest is a closed polymorphic request family. Each concrete request
// owns its dispatch, keeping mode branching out of the query core.
type QueryRequest interface {
	Mode() QueryMode
	execute(QueryIndex, []CandidateProducer) (QueryResult, error)
	isQueryRequest()
}

type ConcernQuery struct {
	Text            string         `json:"query"`
	EntityOfConcern string         `json:"entity_of_concern,omitempty"`
	KnownContext    []string       `json:"known_context,omitempty"`
	IntendedUse     string         `json:"intended_use,omitempty"`
	ResponseBudget  ResponseBudget `json:"response_budget,omitempty"`
}

func (ConcernQuery) Mode() QueryMode { return QueryModeConcern }
func (ConcernQuery) isQueryRequest() {}
func (request ConcernQuery) execute(index QueryIndex, optionalProducers []CandidateProducer) (QueryResult, error) {
	probe := CandidateProbe{
		Text:            request.Text,
		EntityOfConcern: request.EntityOfConcern,
		KnownContext:    request.KnownContext,
		IntendedUse:     request.IntendedUse,
	}
	return queryCandidates(index, optionalProducers, probe, request.ResponseBudget)
}

type LookupQuery struct {
	Identifier     string           `json:"identifier"`
	Roles          []SourceUnitRole `json:"source_roles,omitempty"`
	ResponseBudget ResponseBudget   `json:"response_budget,omitempty"`
}

func (LookupQuery) Mode() QueryMode { return QueryModeLookup }
func (LookupQuery) isQueryRequest() {}
func (request LookupQuery) execute(index QueryIndex, optionalProducers []CandidateProducer) (QueryResult, error) {
	identifier := strings.TrimSpace(request.Identifier)
	if identifier == "" {
		return nil, fmt.Errorf("lookup identifier is required")
	}
	if err := validateResponseBudget(request.ResponseBudget); err != nil {
		return nil, err
	}

	roles, err := NormalizeSourceUnitRoles(request.Roles)
	if err != nil {
		return nil, err
	}

	unit, found, err := index.LookupExact(identifier, roles)
	if err != nil {
		return nil, err
	}
	if found {
		return newExactHit(identifier, unit), nil
	}

	probe := CandidateProbe{Text: identifier}
	candidateRoles, err := normalizeConcernRoles(request.Roles)
	if err != nil {
		return nil, err
	}
	result, _, err := queryCandidatesForNormalizedRoles(
		index,
		optionalProducers,
		probe,
		candidateRoles,
		request.ResponseBudget,
	)
	return result, err
}

type InspectQuery struct {
	Identifier string           `json:"identifier"`
	Roles      []SourceUnitRole `json:"source_roles,omitempty"`
}

func (InspectQuery) Mode() QueryMode { return QueryModeInspect }
func (InspectQuery) isQueryRequest() {}
func (request InspectQuery) execute(index QueryIndex, _ []CandidateProducer) (QueryResult, error) {
	identifier := strings.TrimSpace(request.Identifier)
	if identifier == "" {
		return nil, fmt.Errorf("inspect identifier is required")
	}

	roles, err := NormalizeSourceUnitRoles(request.Roles)
	if err != nil {
		return nil, err
	}

	unit, found, err := index.InspectExact(identifier, roles)
	if err != nil {
		return nil, err
	}
	if !found {
		return newQueryAbstention(
			identifier,
			ReasonExactSourceUnitNotFound,
			[]string{"exact SourceID or UnitID in the requested source roles"},
		), nil
	}
	return newExactHit(identifier, unit), nil
}

type QueryResultKind string

const (
	QueryResultKindExactHit     QueryResultKind = "exact_hit"
	QueryResultKindCandidateSet QueryResultKind = "candidate_set"
	QueryResultKindAbstained    QueryResultKind = "abstained"

	// ReasonExactSourceUnitNotFound is the stable abstention reason returned
	// when inspect mode cannot resolve an exact source identity.
	ReasonExactSourceUnitNotFound = "exact_source_unit_not_found"
)

type QueryResult interface {
	ResultKind() QueryResultKind
	isQueryResult()
}

// ExactHit contains one exactly addressable source unit. Direct-reference
// hydration remains a candidate/inspect concern and cannot make exact identity
// ambiguous.
type ExactHit struct {
	Kind       QueryResultKind `json:"kind"`
	Identifier string          `json:"identifier"`
	Unit       SourceUnit      `json:"unit"`
}

func (ExactHit) ResultKind() QueryResultKind { return QueryResultKindExactHit }
func (ExactHit) isQueryResult()              {}

type RetrievalTier string

const (
	RetrievalTierExactSource    RetrievalTier = "exact_source"
	RetrievalTierAuthoredPhrase RetrievalTier = "authored_phrase"
	RetrievalTierHeadingKeyword RetrievalTier = "heading_keyword"
	RetrievalTierRoleLocalFTS   RetrievalTier = "role_local_fts"

	retrievalTierOptionalCandidate RetrievalTier = "optional_candidate"
)

const sourceFieldExactIdentifierOrTitle = "exact_source_identifier_or_title"

// MatchGround says why a source unit entered the candidate set. Retrieval
// grounds are not evidence that a pattern applies.
type MatchGround struct {
	Tier         RetrievalTier        `json:"tier"`
	ProbeField   string               `json:"probe_field"`
	SourceField  string               `json:"source_field"`
	MatchedValue string               `json:"matched_value"`
	PhraseKind   SourcePhraseKind     `json:"phrase_kind,omitempty"`
	Evidence     *MatchGroundEvidence `json:"evidence,omitempty"`
}

// MatchGroundEvidence keeps projected navigation candidates honest: the
// candidate provenance describes the returned card/ToC row, while this value
// identifies the exact source unit that supplied the match.
type MatchGroundEvidence struct {
	UnitID             string           `json:"unit_id"`
	PatternID          string           `json:"pattern_id"`
	SourceRole         SourceUnitRole   `json:"source_role"`
	Provenance         SourceProvenance `json:"provenance"`
	ProjectionRelation string           `json:"projection_relation"`
}

type SourceCandidate struct {
	Source       CandidateSourceUnit `json:"source"`
	MatchGrounds []MatchGround       `json:"match_grounds"`
}

// CandidateSourceUnit is a compact carrier. Full source bodies are returned
// only by exact/inspect; candidates keep source cues, a bounded excerpt, and
// exact provenance for deliberate hydration.
type CandidateSourceUnit struct {
	UnitID             string                       `json:"unit_id"`
	SourceID           string                       `json:"source_id,omitempty"`
	SourceRole         SourceUnitRole               `json:"source_role"`
	Title              string                       `json:"title"`
	Excerpt            string                       `json:"excerpt,omitempty"`
	ExcerptTruncated   bool                         `json:"excerpt_truncated"`
	UseCues            *SourceUseCues               `json:"use_cues,omitempty"`
	UseCuesTruncated   bool                         `json:"use_cues_truncated,omitempty"`
	PatternID          string                       `json:"pattern_id,omitempty"`
	ParentPatternID    string                       `json:"parent_pattern_id,omitempty"`
	PublicationStatus  string                       `json:"publication_status,omitempty"`
	DirectRefs         []string                     `json:"direct_refs,omitempty"`
	RelationProjection *CandidateRelationProjection `json:"relation_projection,omitempty"`
	Provenance         SourceProvenance             `json:"provenance"`
}

// CandidateRelationProjection exposes authored adjacency without pretending
// that a published ToC row owns the derived relation set. CanonicalUnitID is
// the adjacency owner (pattern_body when published, toc_row while planned);
// each relation's provenance remains the authority source.
type CandidateRelationProjection struct {
	SubjectPatternID string           `json:"subject_pattern_id"`
	CanonicalUnitID  string           `json:"canonical_unit_id"`
	Relations        []SourceRelation `json:"relations"`
	Truncated        bool             `json:"truncated"`
	OmittedAtLeast   int              `json:"omitted_at_least"`
}

// RetrievedCandidate is internal-to-port material before response projection.
// It deliberately retains the full unit so the core, not the adapter, applies
// the response budget.
type RetrievedCandidate struct {
	Unit               SourceUnit
	RelationProjection *CandidateRelationProjection
	MatchGrounds       []MatchGround
}

type SourceCandidateGroup struct {
	Role       SourceUnitRole    `json:"source_role"`
	Candidates []SourceCandidate `json:"candidates"`
}

type CandidateTruncation struct {
	Applied            bool           `json:"applied"`
	Budget             ResponseBudget `json:"budget"`
	IncludedCandidates int            `json:"included_candidates"`
	OmittedAtLeast     int            `json:"omitted_at_least"`
	Basis              []string       `json:"basis,omitempty"`
}

type CandidateSet struct {
	Kind       QueryResultKind        `json:"kind"`
	Concern    string                 `json:"concern"`
	Groups     []SourceCandidateGroup `json:"groups"`
	Truncation CandidateTruncation    `json:"truncation"`
}

func (CandidateSet) ResultKind() QueryResultKind { return QueryResultKindCandidateSet }
func (CandidateSet) isQueryResult()              {}

type Abstained struct {
	Kind         QueryResultKind `json:"kind"`
	Query        string          `json:"query"`
	Reason       string          `json:"reason"`
	MissingBasis []string        `json:"missing_basis"`
}

func (Abstained) ResultKind() QueryResultKind { return QueryResultKindAbstained }
func (Abstained) isQueryResult()              {}

// CandidateBatch is one observable producer result. Truncated/OmittedAtLeast
// let the effect adapter report that its own bounded retrieval omitted rows.
type CandidateBatch struct {
	Candidates     []RetrievedCandidate
	Truncated      bool
	OmittedAtLeast int
	OmittedBasis   []string
}

// QueryIndex is the effect boundary. Its three candidate producers mirror the
// source grammar and cannot collapse back into one opaque semantic router.
type QueryIndex interface {
	LookupExact(identifier string, roles []SourceUnitRole) (SourceUnit, bool, error)
	SearchSourceProbePhrases(phrases []SourceProbePhrase, roles []SourceUnitRole) (CandidateBatch, error)
	SearchAuthoredPhrases(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error)
	SearchHeadingsAndKeywords(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error)
	SearchRoleLocalFTS(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error)
	SourceLexemeFrequencies(lexemes []string, roles []SourceUnitRole) (SourceLexemeFrequencies, error)
	NavigationCandidatesForPatterns(patternIDs []string) (CandidateBatch, error)
	RelationProjectionsForPatterns(patternIDs []string) (map[string]CandidateRelationProjection, error)
	InspectExact(identifier string, roles []SourceUnitRole) (SourceUnit, bool, error)
}

// CandidateProducer is the explicit optional recall seam. The source-native
// authored-phrase, heading/keyword, and role-local FTS producers remain the
// default QueryIndex contract. A future dense or other experimental producer
// must be supplied deliberately through QueryWithCandidateProducers, return
// existing UnitIDs with observable match grounds, and cannot select a winner.
// The core re-hydrates every optional hit from QueryIndex, so a producer cannot
// invent or rewrite source content or provenance.
type CandidateProducer interface {
	ProducerID() string
	ProduceCandidates(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error)
}

const (
	queryProducerExactSource    = "exact_source"
	queryProducerSourcePhrase   = "source_phrase"
	queryProducerAuthoredPhrase = "authored_phrase"
	queryProducerHeadingKeyword = "heading_keyword"
	queryProducerRoleLocalFTS   = "role_local_fts"
)

// QueryEvaluation keeps the canonical result and the producers that were
// actually eligible for that execution together. Its fields are private so a
// caller cannot fabricate diagnostic producer identity independently of Query.
type QueryEvaluation struct {
	result      QueryResult
	producerIDs []string
}

func (evaluation QueryEvaluation) Result() QueryResult {
	return cloneQueryResult(evaluation.result)
}

func (evaluation QueryEvaluation) ProducerIDs() []string {
	return append([]string(nil), evaluation.producerIDs...)
}

func (evaluation QueryEvaluation) canonicalResult() QueryResult {
	return evaluation.result
}

func newQueryEvaluation(
	result QueryResult,
	producerIDs []string,
) QueryEvaluation {
	return QueryEvaluation{
		result:      cloneQueryResult(result),
		producerIDs: append([]string(nil), producerIDs...),
	}
}

func cloneQueryRequest(request QueryRequest) (QueryRequest, error) {
	switch typed := request.(type) {
	case ConcernQuery:
		typed.KnownContext = append([]string(nil), typed.KnownContext...)
		return typed, nil
	case LookupQuery:
		typed.Roles = append([]SourceUnitRole(nil), typed.Roles...)
		return typed, nil
	case InspectQuery:
		typed.Roles = append([]SourceUnitRole(nil), typed.Roles...)
		return typed, nil
	case *ConcernQuery:
		if typed == nil {
			return nil, fmt.Errorf("query request is required")
		}
		cloned := *typed
		cloned.KnownContext = append([]string(nil), typed.KnownContext...)
		return cloned, nil
	case *LookupQuery:
		if typed == nil {
			return nil, fmt.Errorf("query request is required")
		}
		cloned := *typed
		cloned.Roles = append([]SourceUnitRole(nil), typed.Roles...)
		return cloned, nil
	case *InspectQuery:
		if typed == nil {
			return nil, fmt.Errorf("query request is required")
		}
		cloned := *typed
		cloned.Roles = append([]SourceUnitRole(nil), typed.Roles...)
		return cloned, nil
	default:
		return nil, fmt.Errorf("unsupported FPF query request %T", request)
	}
}

func cloneQueryResult(result QueryResult) QueryResult {
	switch typed := result.(type) {
	case ExactHit:
		typed.Unit = cloneSourceUnit(typed.Unit)
		return typed
	case CandidateSet:
		typed.Groups = cloneSourceCandidateGroups(typed.Groups)
		typed.Truncation.Basis = append([]string(nil), typed.Truncation.Basis...)
		return typed
	case Abstained:
		typed.MissingBasis = append([]string(nil), typed.MissingBasis...)
		return typed
	default:
		return nil
	}
}

func cloneSourceCandidateGroups(groups []SourceCandidateGroup) []SourceCandidateGroup {
	cloned := make([]SourceCandidateGroup, len(groups))
	for groupIndex, group := range groups {
		group.Candidates = cloneSourceCandidates(group.Candidates)
		cloned[groupIndex] = group
	}
	return cloned
}

func cloneSourceCandidates(candidates []SourceCandidate) []SourceCandidate {
	cloned := make([]SourceCandidate, len(candidates))
	for candidateIndex, candidate := range candidates {
		candidate.Source = cloneCandidateSourceUnit(candidate.Source)
		candidate.MatchGrounds = cloneMatchGrounds(candidate.MatchGrounds)
		cloned[candidateIndex] = candidate
	}
	return cloned
}

func cloneCandidateSourceUnit(unit CandidateSourceUnit) CandidateSourceUnit {
	unit.DirectRefs = append([]string(nil), unit.DirectRefs...)
	if unit.UseCues != nil {
		cues := *unit.UseCues
		unit.UseCues = &cues
	}
	if unit.RelationProjection != nil {
		unit.RelationProjection = cloneCandidateRelationProjection(*unit.RelationProjection)
	}
	return unit
}

func cloneMatchGrounds(grounds []MatchGround) []MatchGround {
	cloned := make([]MatchGround, len(grounds))
	for groundIndex, ground := range grounds {
		if ground.Evidence != nil {
			evidence := *ground.Evidence
			ground.Evidence = &evidence
		}
		cloned[groundIndex] = ground
	}
	return cloned
}

var defaultResponseBudget = ResponseBudget{
	MaxCandidatesPerRole:     5,
	MaxTotalCandidates:       10,
	MaxExcerptCharacters:     480,
	MaxRelationsPerCandidate: 12,
}

func Query(index QueryIndex, request QueryRequest) (QueryResult, error) {
	evaluation, err := EvaluateQuery(index, request)
	if err != nil {
		return nil, err
	}
	return evaluation.Result(), nil
}

func QueryWithCandidateProducers(index QueryIndex, request QueryRequest, optionalProducers []CandidateProducer) (QueryResult, error) {
	evaluation, err := EvaluateQueryWithCandidateProducers(index, request, optionalProducers)
	if err != nil {
		return nil, err
	}
	return evaluation.Result(), nil
}

func EvaluateQuery(index QueryIndex, request QueryRequest) (QueryEvaluation, error) {
	return EvaluateQueryWithCandidateProducers(index, request, nil)
}

func EvaluateQueryWithCandidateProducers(
	index QueryIndex,
	request QueryRequest,
	optionalProducers []CandidateProducer,
) (QueryEvaluation, error) {
	if index == nil {
		return QueryEvaluation{}, fmt.Errorf("query index is required")
	}
	if request == nil {
		return QueryEvaluation{}, fmt.Errorf("query request is required")
	}
	for _, producer := range optionalProducers {
		if producer == nil {
			return QueryEvaluation{}, fmt.Errorf("optional candidate producer is required")
		}
		if strings.TrimSpace(producer.ProducerID()) == "" {
			return QueryEvaluation{}, fmt.Errorf("optional candidate producer id is required")
		}
	}
	producers := append([]CandidateProducer(nil), optionalProducers...)
	result, err := request.execute(index, producers)
	if err != nil {
		return QueryEvaluation{}, err
	}
	producerIDs := queryEvaluationProducerIDs(request, result, producers)
	return newQueryEvaluation(result, producerIDs), nil
}

func queryEvaluationProducerIDs(
	request QueryRequest,
	result QueryResult,
	optionalProducers []CandidateProducer,
) []string {
	if request.Mode() == QueryModeInspect ||
		(request.Mode() == QueryModeLookup && result.ResultKind() == QueryResultKindExactHit) {
		return []string{queryProducerExactSource}
	}
	producerIDs := []string{
		queryProducerExactSource,
		queryProducerSourcePhrase,
		queryProducerAuthoredPhrase,
		queryProducerHeadingKeyword,
		queryProducerRoleLocalFTS,
	}
	for _, producer := range optionalProducers {
		producerIDs = append(producerIDs, producer.ProducerID())
	}
	return dedupeStrings(producerIDs)
}

func queryCandidates(index QueryIndex, optionalProducers []CandidateProducer, probe CandidateProbe, requestedBudget ResponseBudget) (QueryResult, error) {
	primaryRoles := []SourceUnitRole{
		SourceUnitRolePracticalUseCard,
		SourceUnitRoleTOCRow,
	}
	primary, optionalBatches, err := queryCandidatesForNormalizedRoles(
		index,
		optionalProducers,
		probe,
		primaryRoles,
		requestedBudget,
	)
	if err != nil {
		return nil, err
	}
	if primary.ResultKind() != QueryResultKindAbstained {
		candidates := primary.(CandidateSet)
		if candidateSetHasExactOrPhraseGround(candidates) {
			return primary, nil
		}
		navigation, err := queryNavigationFromPatternBodyPhraseEvidence(
			index,
			probe,
			requestedBudget,
			optionalBatches,
			"primary_candidates_have_only_token_union_grounds",
			[]string{"an exact source anchor, authored phrase, or contiguous source phrase"},
		)
		if err != nil {
			return nil, err
		}
		if navigation.ResultKind() != QueryResultKindAbstained {
			return navigation, nil
		}
		return primary, nil
	}
	primaryAbstention := primary.(Abstained)
	return queryNavigationFromPatternBodyPhraseEvidence(
		index,
		probe,
		requestedBudget,
		optionalBatches,
		primaryAbstention.Reason,
		primaryAbstention.MissingBasis,
	)
}

func candidateSetHasExactOrPhraseGround(candidates CandidateSet) bool {
	for _, group := range candidates.Groups {
		for _, candidate := range group.Candidates {
			for _, ground := range candidate.MatchGrounds {
				if matchGroundHasExactOrPhraseSourceGround(ground) {
					return true
				}
			}
		}
	}
	return false
}

func queryCandidatesForNormalizedRoles(
	index QueryIndex,
	optionalProducers []CandidateProducer,
	probe CandidateProbe,
	roles []SourceUnitRole,
	requestedBudget ResponseBudget,
) (QueryResult, []CandidateBatch, error) {
	probe = normalizeCandidateProbe(probe)
	if probe.Text == "" {
		return nil, nil, fmt.Errorf("concern text is required")
	}
	if err := validateResponseBudget(requestedBudget); err != nil {
		return nil, nil, err
	}
	exact, err := exactSourceCandidateBatch(index, probe, roles)
	if err != nil {
		return nil, nil, err
	}
	derivedPhrases := derivedSourcePhrases(probe)
	derived, err := index.SearchSourceProbePhrases(derivedPhrases, roles)
	if err != nil {
		return nil, nil, err
	}

	authored, err := index.SearchAuthoredPhrases(probe, roles)
	if err != nil {
		return nil, nil, err
	}

	headings, err := index.SearchHeadingsAndKeywords(probe, roles)
	if err != nil {
		return nil, nil, err
	}

	fts, err := index.SearchRoleLocalFTS(probe, roles)
	if err != nil {
		return nil, nil, err
	}
	frequencies, err := index.SourceLexemeFrequencies(nonScaffoldProbeLexemes(probe), roles)
	if err != nil {
		return nil, nil, fmt.Errorf("load source lexeme frequencies: %w", err)
	}
	if frequencies.TotalSourceUnits <= 0 {
		return nil, nil, fmt.Errorf("source lexeme frequency corpus is empty for requested roles")
	}
	rawSourceCandidateCount := len(exact.Candidates) + len(derived.Candidates) + len(authored.Candidates) + len(headings.Candidates) + len(fts.Candidates)
	grounding := newSourceGroundingEvaluator(probe, frequencies)
	batches := []CandidateBatch{
		exact,
		grounding.filter(derived),
		grounding.filter(authored),
		grounding.filter(headings),
		grounding.filter(fts),
	}
	optionalBatchStart := len(batches)
	for _, producer := range optionalProducers {
		batch, err := producer.ProduceCandidates(probe, append([]SourceUnitRole(nil), roles...))
		if err != nil {
			return nil, nil, fmt.Errorf("optional candidate producer %s: %w", producer.ProducerID(), err)
		}
		batch, err = hydrateOptionalCandidateBatch(index, producer.ProducerID(), batch, roles)
		if err != nil {
			return nil, nil, err
		}
		batches = append(batches, batch)
	}
	batches, err = attachCandidateRelationProjections(index, batches)
	if err != nil {
		return nil, nil, err
	}
	optionalBatches := append([]CandidateBatch(nil), batches[optionalBatchStart:]...)

	budget := normalizeResponseBudget(requestedBudget)
	groups, truncation := assembleCandidateGroups(roles, budget, batches...)
	if len(groups) == 0 {
		reason := "no_source_derived_candidates"
		if rawSourceCandidateCount > 0 {
			reason = "insufficient_source_grounded_match"
		}
		missingBasis := []string{
			"exact source id, title, authored cue, derived source phrase, or at least two non-scaffold probe lexemes with sufficient same-unit source-corpus IDF weight",
			sourceLexemeFrequencyBasis(probe, frequencies),
		}
		return newQueryAbstention(
			probe.Text,
			reason,
			missingBasis,
		), optionalBatches, nil
	}

	return CandidateSet{
		Kind:       QueryResultKindCandidateSet,
		Concern:    probe.Text,
		Groups:     groups,
		Truncation: truncation,
	}, optionalBatches, nil
}

func queryNavigationFromPatternBodyPhraseEvidence(
	index QueryIndex,
	probe CandidateProbe,
	requestedBudget ResponseBudget,
	optionalBatches []CandidateBatch,
	primaryReason string,
	primaryMissingBasis []string,
) (QueryResult, error) {
	probe = normalizeCandidateProbe(probe)
	if err := validateResponseBudget(requestedBudget); err != nil {
		return nil, err
	}
	phrases := derivedSourcePhrases(probe)
	if len(phrases) == 0 {
		return newPatternBodyPhraseAbstention(probe.Text, primaryReason, primaryMissingBasis), nil
	}

	bodyBatch, err := index.SearchSourceProbePhrases(
		phrases,
		[]SourceUnitRole{SourceUnitRolePatternBody},
	)
	if err != nil {
		return nil, err
	}
	witnesses := filterSupportedPatternBodyPhrases(bodyBatch)
	if len(witnesses.Candidates) == 0 {
		return newPatternBodyPhraseAbstention(probe.Text, primaryReason, primaryMissingBasis), nil
	}

	patternIDs := candidatePatternIDs(witnesses.Candidates)
	navigation, err := index.NavigationCandidatesForPatterns(patternIDs)
	if err != nil {
		return nil, fmt.Errorf("project pattern-body phrase evidence to navigation: %w", err)
	}
	navigation = projectPatternBodyPhraseGrounds(navigation, witnesses)
	if len(navigation.Candidates) == 0 {
		return newQueryAbstention(
			probe.Text,
			"source_phrase_has_no_authored_navigation_projection",
			[]string{"pattern-body evidence must project by exact PatternID or authored DirectRef"},
		), nil
	}

	roles := []SourceUnitRole{SourceUnitRolePracticalUseCard, SourceUnitRoleTOCRow}
	expansion, err := index.SearchHeadingsAndKeywords(probe, roles)
	if err != nil {
		return nil, fmt.Errorf("expand admitted source phrase into navigation: %w", err)
	}
	expansion = markNavigationExpansionGrounds(expansion)
	expansion = boundNavigationExpansion(expansion, roles, defaultResponseBudget.MaxCandidatesPerRole)
	batches, err := attachCandidateRelationProjections(index, []CandidateBatch{navigation, expansion})
	if err != nil {
		return nil, err
	}
	batches = append(batches, optionalBatches...)
	budget := normalizeResponseBudget(requestedBudget)
	groups, truncation := assembleCandidateGroups(roles, budget, batches...)
	if len(groups) == 0 {
		return newPatternBodyPhraseAbstention(probe.Text, primaryReason, primaryMissingBasis), nil
	}
	return CandidateSet{
		Kind:       QueryResultKindCandidateSet,
		Concern:    probe.Text,
		Groups:     groups,
		Truncation: truncation,
	}, nil
}

func markNavigationExpansionGrounds(batch CandidateBatch) CandidateBatch {
	marked := batch
	marked.Candidates = make([]RetrievedCandidate, 0, len(batch.Candidates))
	for _, candidate := range batch.Candidates {
		grounds := make([]MatchGround, 0, len(candidate.MatchGrounds))
		for _, ground := range candidate.MatchGrounds {
			if ground.SourceField != "source_id" && ground.SourceField != "heading" && ground.SourceField != "keywords" {
				continue
			}
			ground.SourceField = "expansion_after_source_admission"
			ground.Evidence = &MatchGroundEvidence{
				UnitID:             candidate.Unit.UnitID,
				PatternID:          candidate.Unit.PatternID,
				SourceRole:         candidate.Unit.Role,
				Provenance:         candidate.Unit.Provenance,
				ProjectionRelation: "source_field_partial_match",
			}
			grounds = append(grounds, ground)
		}
		if len(grounds) == 0 {
			continue
		}
		candidate.MatchGrounds = grounds
		marked.Candidates = append(marked.Candidates, candidate)
	}
	return marked
}

func boundNavigationExpansion(batch CandidateBatch, roles []SourceUnitRole, maxPerRole int) CandidateBatch {
	bounded := batch
	bounded.Candidates = make([]RetrievedCandidate, 0)
	omitted := 0
	for _, role := range roles {
		roleCandidates := candidatesForRole(batch.Candidates, role)
		visible := roleCandidates[:min(len(roleCandidates), maxPerRole)]
		bounded.Candidates = append(bounded.Candidates, visible...)
		omitted += len(roleCandidates) - len(visible)
	}
	if omitted > 0 {
		bounded.Truncated = true
		bounded.OmittedAtLeast += omitted
		bounded.OmittedBasis = appendUnique(bounded.OmittedBasis, "navigation_expansion_limit")
	}
	return bounded
}

func filterSupportedPatternBodyPhrases(batch CandidateBatch) CandidateBatch {
	support := make(map[string]map[string]struct{})
	for _, candidate := range batch.Candidates {
		for _, ground := range candidate.MatchGrounds {
			if ground.SourceField != sourceFieldDerivedPhrase {
				continue
			}
			phrase := sourcePhraseSupportKey(ground)
			patterns := support[phrase]
			if patterns == nil {
				patterns = make(map[string]struct{})
				support[phrase] = patterns
			}
			patterns[candidate.Unit.PatternID] = struct{}{}
		}
	}

	filtered := CandidateBatch{
		Candidates:     make([]RetrievedCandidate, 0, len(batch.Candidates)),
		Truncated:      batch.Truncated,
		OmittedAtLeast: batch.OmittedAtLeast,
		OmittedBasis:   append([]string(nil), batch.OmittedBasis...),
	}
	for _, candidate := range batch.Candidates {
		grounds := make([]MatchGround, 0, len(candidate.MatchGrounds))
		for _, ground := range candidate.MatchGrounds {
			phrase := sourcePhraseSupportKey(ground)
			minimumWitnesses := minimumPatternBodyPhraseWitnesses
			if ground.PhraseKind == SourcePhraseKindExactProbeSpan {
				minimumWitnesses = 1
			}
			if len(support[phrase]) < minimumWitnesses {
				continue
			}
			grounds = append(grounds, ground)
		}
		if len(grounds) == 0 {
			continue
		}
		candidate.MatchGrounds = grounds
		filtered.Candidates = append(filtered.Candidates, candidate)
	}
	return filtered
}

func candidatePatternIDs(candidates []RetrievedCandidate) []string {
	patternIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		patternIDs = append(patternIDs, candidate.Unit.PatternID)
	}
	return normalizeNonEmptyStrings(patternIDs)
}

func sourcePhraseSupportKey(ground MatchGround) string {
	phrase := normalizeSourceGroundingValue(ground.MatchedValue)
	return string(ground.PhraseKind) + "\x00" + phrase
}

func projectPatternBodyPhraseGrounds(navigation CandidateBatch, witnesses CandidateBatch) CandidateBatch {
	projected := navigation
	projected.Candidates = make([]RetrievedCandidate, 0, len(navigation.Candidates))
	for _, candidate := range navigation.Candidates {
		grounds := make([]MatchGround, 0)
		for _, witness := range witnesses.Candidates {
			relation := navigationProjectionRelation(candidate.Unit, witness.Unit.PatternID)
			if relation == "" {
				continue
			}
			for _, ground := range witness.MatchGrounds {
				ground.SourceField = sourceFieldPatternBodyDerivedPhrase
				ground.Evidence = &MatchGroundEvidence{
					UnitID:             witness.Unit.UnitID,
					PatternID:          witness.Unit.PatternID,
					SourceRole:         witness.Unit.Role,
					Provenance:         witness.Unit.Provenance,
					ProjectionRelation: relation,
				}
				grounds = append(grounds, ground)
			}
		}
		if len(grounds) == 0 {
			continue
		}
		candidate.MatchGrounds = mergeMatchGrounds(candidate.MatchGrounds, grounds)
		projected.Candidates = append(projected.Candidates, candidate)
	}
	return projected
}

func navigationProjectionRelation(unit SourceUnit, patternID string) string {
	if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == patternID {
		return "same_pattern_id"
	}
	if unit.Role == SourceUnitRolePracticalUseCard && stringSliceContains(unit.DirectRefs, patternID) {
		return "authored_direct_ref"
	}
	return ""
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func newPatternBodyPhraseAbstention(query, primaryReason string, primaryMissingBasis []string) Abstained {
	missingBasis := append([]string(nil), primaryMissingBasis...)
	missingBasis = append(missingBasis,
		"default concern results are navigation units, not pattern bodies",
		"a derived phrase needs one navigation-source witness or at least two canonical pattern-body witnesses with an exact PatternID/DirectRef projection",
	)
	return newQueryAbstention(
		query,
		primaryReason,
		missingBasis,
	)
}

func sourceLexemeFrequencyBasis(probe CandidateProbe, frequencies SourceLexemeFrequencies) string {
	parts := make([]string, 0)
	for _, lexeme := range nonScaffoldProbeLexemes(probe) {
		parts = append(parts, fmt.Sprintf("%s=%d", lexeme, frequencies.DocumentFrequency[lexeme]))
	}
	return fmt.Sprintf(
		"source-role corpus=%d units; required same-unit IDF weight=%.1f across at least %d non-scaffold lexemes; document frequencies: %s",
		frequencies.TotalSourceUnits,
		minimumSourceGroundedLexemeWeight,
		minimumDistinctSourceGroundedLexemes,
		strings.Join(parts, ", "),
	)
}

func attachCandidateRelationProjections(index QueryIndex, batches []CandidateBatch) ([]CandidateBatch, error) {
	patternIDs := make([]string, 0)
	for _, batch := range batches {
		for _, candidate := range batch.Candidates {
			if candidate.Unit.PatternID != "" {
				patternIDs = append(patternIDs, candidate.Unit.PatternID)
			}
		}
	}
	patternIDs = dedupeStrings(patternIDs)
	if len(patternIDs) == 0 {
		return batches, nil
	}

	projections, err := index.RelationProjectionsForPatterns(patternIDs)
	if err != nil {
		return nil, fmt.Errorf("load candidate relation projections: %w", err)
	}
	for patternID, projection := range projections {
		if patternID != projection.SubjectPatternID || strings.TrimSpace(projection.CanonicalUnitID) == "" {
			return nil, fmt.Errorf("relation projection %s has inconsistent subject or canonical unit", patternID)
		}
		for _, relation := range projection.Relations {
			if !isSourceRelationKind(relation.Kind) || relation.Origin != SourceRelationOriginTOCExplicit {
				return nil, fmt.Errorf("relation projection %s contains invalid source relation", patternID)
			}
		}
	}

	attached := make([]CandidateBatch, 0, len(batches))
	for _, batch := range batches {
		copyBatch := batch
		copyBatch.Candidates = make([]RetrievedCandidate, 0, len(batch.Candidates))
		for _, candidate := range batch.Candidates {
			projection, exists := projections[candidate.Unit.PatternID]
			if exists {
				candidate.RelationProjection = cloneCandidateRelationProjection(projection)
			}
			copyBatch.Candidates = append(copyBatch.Candidates, candidate)
		}
		attached = append(attached, copyBatch)
	}
	return attached, nil
}

func cloneCandidateRelationProjection(projection CandidateRelationProjection) *CandidateRelationProjection {
	projection.Relations = cloneSourceRelations(projection.Relations)
	return &projection
}

const minimumDistinctSourceGroundedLexemes = 2
const minimumSourceGroundedLexemeWeight = 5.0
const minimumPatternBodyPhraseWitnesses = 2

const sourceFieldDerivedPhrase = "title_body_source_phrase"
const sourceFieldPatternBodyDerivedPhrase = "pattern_body_source_phrase"
const sourceFieldTitleBodyToken = "title_body_token"
const sourceFieldOptionalCandidateMatch = "optional_candidate_match"

func matchGroundHasExactOrPhraseSourceGround(ground MatchGround) bool {
	return ground.Tier == RetrievalTierExactSource ||
		ground.Tier == RetrievalTierAuthoredPhrase ||
		ground.SourceField == sourceFieldDerivedPhrase
}

var genericQuestionScaffoldLexemes = map[string]struct{}{
	"how": {}, "what": {}, "when": {}, "where": {}, "why": {}, "who": {}, "which": {},
	"should": {}, "would": {}, "could": {}, "does": {}, "did": {}, "this": {}, "that": {},
	"these": {}, "those": {}, "here": {}, "there": {}, "the": {}, "and": {}, "from": {},
	"for": {}, "with": {}, "its": {}, "your": {}, "an": {}, "of": {}, "to": {}, "in": {},
	"on": {}, "at": {}, "by": {}, "as": {}, "or": {}, "be": {}, "do": {}, "is": {}, "it": {},
	"we": {}, "us": {}, "my": {}, "me": {}, "use": {}, "using": {}, "used": {},
	"как": {}, "что": {}, "когда": {},
	"где": {}, "почему": {}, "кто": {}, "какой": {}, "какая": {}, "нужно": {}, "следует": {},
	"это": {}, "этот": {}, "эта": {}, "здесь": {}, "там": {}, "для": {}, "или": {},
}

// sourceGroundingEvaluator is an information-sufficiency gate, not an intent
// classifier. It uses producer-emitted source grounds and never tokenizes a
// complete pattern body during a query.
type sourceGroundingEvaluator struct {
	probeValues  []string
	probeLexemes []string
	probeSet     map[string]struct{}
	frequencies  SourceLexemeFrequencies
}

func newSourceGroundingEvaluator(probe CandidateProbe, frequencies SourceLexemeFrequencies) *sourceGroundingEvaluator {
	values := candidateProbeValues(probe)
	probeValues := make([]string, 0, len(values))
	for _, value := range values {
		probeValues = append(probeValues, normalizeSourceGroundingValue(value))
	}
	probeLexemes := nonScaffoldProbeLexemes(probe)
	probeSet := make(map[string]struct{}, len(probeLexemes))
	for _, lexeme := range probeLexemes {
		probeSet[lexeme] = struct{}{}
	}
	return &sourceGroundingEvaluator{
		probeValues:  probeValues,
		probeLexemes: probeLexemes,
		probeSet:     probeSet,
		frequencies:  frequencies,
	}
}

func (evaluator *sourceGroundingEvaluator) filter(batch CandidateBatch) CandidateBatch {
	filtered := CandidateBatch{
		Candidates:     make([]RetrievedCandidate, 0, len(batch.Candidates)),
		Truncated:      batch.Truncated,
		OmittedAtLeast: batch.OmittedAtLeast,
		OmittedBasis:   append([]string(nil), batch.OmittedBasis...),
	}
	for _, candidate := range batch.Candidates {
		if evaluator.allows(candidate) {
			filtered.Candidates = append(filtered.Candidates, candidate)
		}
	}
	return filtered
}

func (evaluator *sourceGroundingEvaluator) allows(candidate RetrievedCandidate) bool {
	exactValues := sourceUnitExactGroundingValues(candidate.Unit)
	for _, value := range evaluator.probeValues {
		if _, exact := exactValues[value]; exact {
			return true
		}
	}
	for _, ground := range candidate.MatchGrounds {
		if ground.SourceField == sourceFieldDerivedPhrase {
			return true
		}
	}
	if len(evaluator.probeLexemes) < minimumDistinctSourceGroundedLexemes {
		return false
	}

	groundedLexemes := evaluator.groundedLexemes(candidate.MatchGrounds)
	grounded := 0
	weight := 0.0
	for _, lexeme := range evaluator.probeLexemes {
		if _, exists := groundedLexemes[lexeme]; !exists {
			continue
		}
		documentFrequency := evaluator.frequencies.DocumentFrequency[lexeme]
		if documentFrequency <= 0 {
			continue
		}
		grounded++
		weight += math.Log(
			float64(evaluator.frequencies.TotalSourceUnits+1) /
				float64(documentFrequency+1),
		)
		if grounded >= minimumDistinctSourceGroundedLexemes && weight >= minimumSourceGroundedLexemeWeight {
			return true
		}
	}
	return false
}

func (evaluator *sourceGroundingEvaluator) groundedLexemes(grounds []MatchGround) map[string]struct{} {
	grounded := make(map[string]struct{})
	for _, ground := range grounds {
		if !matchGroundCarriesSourceLexemes(ground.SourceField) {
			continue
		}
		for _, lexeme := range meaningfulSourceLexemes(ground.MatchedValue) {
			if _, requested := evaluator.probeSet[lexeme]; requested {
				grounded[lexeme] = struct{}{}
			}
		}
	}
	return grounded
}

func matchGroundCarriesSourceLexemes(sourceField string) bool {
	return sourceField == "authored_phrases" ||
		sourceField == "heading" ||
		sourceField == "keywords" ||
		sourceField == sourceFieldTitleBodyToken
}

func sourceUnitExactGroundingValues(unit SourceUnit) map[string]struct{} {
	values := []string{unit.UnitID, unit.SourceID, unit.PatternID, unit.Title}
	values = append(values, unit.AuthoredPhrases...)
	values = append(values, unit.Keywords...)
	exact := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeSourceGroundingValue(value)
		if normalized != "" {
			exact[normalized] = struct{}{}
		}
	}
	return exact
}

func normalizeSourceGroundingValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.ToLower(value)
}

func nonScaffoldProbeLexemes(probe CandidateProbe) []string {
	lexemes := make([]string, 0)
	for _, value := range candidateProbeValues(probe) {
		lexemes = append(lexemes, nonScaffoldSourceLexemes(value)...)
	}
	return dedupeStrings(lexemes)
}

func nonScaffoldSourceLexemes(value string) []string {
	lexemes := make([]string, 0)
	for _, lexeme := range orderedSourceRecallLexemes(value) {
		if _, scaffold := genericQuestionScaffoldLexemes[lexeme]; !scaffold {
			lexemes = append(lexemes, lexeme)
		}
	}
	return lexemes
}

type sourceExactAnchor struct {
	ProbeField string
	Value      string
}

func exactSourceCandidateBatch(index QueryIndex, probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error) {
	anchors := exactSourceAnchors(probe)
	candidates := make([]RetrievedCandidate, 0)
	for _, anchor := range anchors {
		for _, role := range roles {
			unit, found, err := index.LookupExact(anchor.Value, []SourceUnitRole{role})
			if err != nil {
				return CandidateBatch{}, fmt.Errorf("look up exact source anchor %q in role %s: %w", anchor.Value, role, err)
			}
			if !found {
				continue
			}
			candidates = append(candidates, RetrievedCandidate{
				Unit: unit,
				MatchGrounds: []MatchGround{{
					Tier:         RetrievalTierExactSource,
					ProbeField:   anchor.ProbeField,
					SourceField:  sourceFieldExactIdentifierOrTitle,
					MatchedValue: anchor.Value,
				}},
			})
		}
	}
	return CandidateBatch{Candidates: candidates}, nil
}

func exactSourceAnchors(probe CandidateProbe) []sourceExactAnchor {
	inputs := []sourceExactAnchor{
		{ProbeField: "text", Value: probe.Text},
		{ProbeField: "entity_of_concern", Value: probe.EntityOfConcern},
	}
	for _, context := range probe.KnownContext {
		inputs = append(inputs, sourceExactAnchor{ProbeField: "known_context", Value: context})
	}
	inputs = append(inputs, sourceExactAnchor{ProbeField: "intended_use", Value: probe.IntendedUse})

	anchors := make([]sourceExactAnchor, 0, len(inputs)*2)
	seen := make(map[string]struct{})
	appendAnchor := func(anchor sourceExactAnchor) {
		anchor.Value = normalizeExactSourceAnchor(anchor.Value)
		if anchor.Value == "" {
			return
		}
		key := anchor.ProbeField + "\x00" + strings.ToLower(anchor.Value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		anchors = append(anchors, anchor)
	}
	for _, input := range inputs {
		appendAnchor(input)
		for _, token := range exactSourceIdentifierTokens(input.Value) {
			appendAnchor(sourceExactAnchor{
				ProbeField: input.ProbeField,
				Value:      token,
			})
		}
	}
	return anchors
}

func normalizeExactSourceAnchor(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`'\"“”‘’()[]{}<>,;!?/")
	return strings.Join(strings.Fields(value), " ")
}

func exactSourceIdentifierTokens(value string) []string {
	fields := strings.FieldsFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || strings.ContainsRune("`'\"“”‘’()[]{}<>,;!?/|", character)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		field = normalizeExactSourceAnchor(field)
		if !looksLikeExactSourceIdentifier(field) {
			continue
		}
		tokens = append(tokens, field)
	}
	return dedupeStrings(tokens)
}

func looksLikeExactSourceIdentifier(value string) bool {
	if strings.ContainsAny(value, ".@:_") {
		return true
	}
	if !strings.Contains(value, "-") {
		return false
	}
	return value == strings.ToUpper(value)
}

func derivedSourcePhrases(probe CandidateProbe) []SourceProbePhrase {
	inputs := []SourceProbePhrase{
		{ProbeField: "text", Value: probe.Text},
		{ProbeField: "entity_of_concern", Value: probe.EntityOfConcern},
	}
	for _, context := range probe.KnownContext {
		inputs = append(inputs, SourceProbePhrase{ProbeField: "known_context", Value: context})
	}
	inputs = append(inputs, SourceProbePhrase{ProbeField: "intended_use", Value: probe.IntendedUse})

	phrases := make([]SourceProbePhrase, 0, len(inputs)*2)
	seen := make(map[string]struct{})
	exactValues := make(map[string]struct{})
	appendPhrase := func(field, phrase string, kind SourcePhraseKind) {
		lexemes := orderedSourceRecallLexemes(phrase)
		if len(dedupeStrings(lexemes)) < minimumDistinctSourceGroundedLexemes {
			return
		}
		key := field + "\x00" + phrase + "\x00" + string(kind)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		phrases = append(phrases, SourceProbePhrase{
			ProbeField: field,
			Value:      phrase,
			Kind:       kind,
		})
		if kind == SourcePhraseKindExactProbeSpan {
			exactValues[field+"\x00"+phrase] = struct{}{}
		}
	}
	for _, input := range inputs {
		exactValue := normalizeSourceGroundingValue(input.Value)
		appendPhrase(input.ProbeField, exactValue, SourcePhraseKindExactProbeSpan)
		for _, exactSpan := range contiguousNonScaffoldSourcePhrases(input.Value) {
			appendPhrase(input.ProbeField, exactSpan, SourcePhraseKindExactProbeSpan)
		}
		compressedLexemes := nonScaffoldSourceLexemes(input.Value)
		compressedValue := strings.Join(compressedLexemes, " ")
		_, alreadyExact := exactValues[input.ProbeField+"\x00"+compressedValue]
		if compressedValue != exactValue && !alreadyExact {
			appendPhrase(input.ProbeField, compressedValue, SourcePhraseKindScaffoldCompressed)
		}
	}
	return phrases
}

// contiguousNonScaffoldSourcePhrases preserves exact lexical spans from the
// operator's query after removing only scaffold runs at their boundaries.
// Unlike scaffold compression, it never joins meaningful words that were
// separated by question scaffolding, so one canonical source occurrence is a
// truthful exact-span witness rather than a synthetic phrase coincidence.
func contiguousNonScaffoldSourcePhrases(value string) []string {
	result := make([]string, 0)
	run := make([]string, 0)
	flush := func() {
		if len(dedupeStrings(run)) >= minimumDistinctSourceGroundedLexemes {
			result = append(result, strings.Join(run, " "))
		}
		run = run[:0]
	}
	for _, lexeme := range orderedSourceRecallLexemes(value) {
		if _, scaffold := genericQuestionScaffoldLexemes[lexeme]; scaffold {
			flush()
			continue
		}
		run = append(run, lexeme)
	}
	flush()
	return dedupeStrings(result)
}

func candidateProbeValues(probe CandidateProbe) []string {
	values := []string{probe.Text, probe.EntityOfConcern}
	values = append(values, probe.KnownContext...)
	values = append(values, probe.IntendedUse)
	return normalizeNonEmptyStrings(values)
}

func meaningfulSourceLexemes(value string) []string {
	return dedupeStrings(orderedMeaningfulSourceLexemes(value))
}

func orderedMeaningfulSourceLexemes(value string) []string {
	lexemes := orderedSourceRecallLexemes(value)
	meaningful := make([]string, 0, len(lexemes))
	for _, lexeme := range lexemes {
		if len([]rune(lexeme)) >= 3 {
			meaningful = append(meaningful, lexeme)
		}
	}
	return meaningful
}

func orderedSourceRecallLexemes(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	})
	lexemes := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			lexemes = append(lexemes, field)
		}
	}
	return lexemes
}

func hydrateOptionalCandidateBatch(index QueryIndex, producerID string, batch CandidateBatch, roles []SourceUnitRole) (CandidateBatch, error) {
	hydrated := CandidateBatch{
		Candidates:     make([]RetrievedCandidate, 0, len(batch.Candidates)),
		Truncated:      batch.Truncated,
		OmittedAtLeast: batch.OmittedAtLeast,
		OmittedBasis:   append([]string(nil), batch.OmittedBasis...),
	}
	for _, candidate := range batch.Candidates {
		if strings.TrimSpace(candidate.Unit.UnitID) == "" {
			return CandidateBatch{}, fmt.Errorf("optional candidate producer %s returned a unit without unit id", producerID)
		}
		if len(candidate.MatchGrounds) == 0 {
			return CandidateBatch{}, fmt.Errorf("optional candidate producer %s returned unit %s without match grounds", producerID, candidate.Unit.UnitID)
		}
		grounds, err := canonicalizeOptionalMatchGrounds(
			index,
			producerID,
			candidate.Unit.UnitID,
			candidate.MatchGrounds,
		)
		if err != nil {
			return CandidateBatch{}, err
		}
		unit, found, err := index.LookupExact(candidate.Unit.UnitID, roles)
		if err != nil {
			return CandidateBatch{}, fmt.Errorf("hydrate optional candidate producer %s unit %s: %w", producerID, candidate.Unit.UnitID, err)
		}
		if !found {
			return CandidateBatch{}, fmt.Errorf("optional candidate producer %s returned unknown unit %s in requested source roles", producerID, candidate.Unit.UnitID)
		}
		hydrated.Candidates = append(hydrated.Candidates, RetrievedCandidate{
			Unit:         unit,
			MatchGrounds: grounds,
		})
	}
	return hydrated, nil
}

func canonicalizeOptionalMatchGrounds(
	index QueryIndex,
	producerID string,
	candidateUnitID string,
	grounds []MatchGround,
) ([]MatchGround, error) {
	canonical := make([]MatchGround, 0, len(grounds))
	for _, ground := range grounds {
		if strings.TrimSpace(string(ground.Tier)) == "" ||
			strings.TrimSpace(ground.ProbeField) == "" ||
			strings.TrimSpace(ground.SourceField) == "" {
			return nil, fmt.Errorf(
				"optional candidate producer %s returned unit %s with incomplete match ground",
				producerID,
				candidateUnitID,
			)
		}
		if ground.Tier == RetrievalTierExactSource {
			return nil, fmt.Errorf(
				"optional candidate producer %s returned unit %s with reserved retrieval tier %s",
				producerID,
				candidateUnitID,
				RetrievalTierExactSource,
			)
		}
		// Authored-phrase tiers and contiguous title/body phrase fields are
		// source-native witnesses when emitted by the core index. Optional
		// producers may keep their candidate and match diagnostic, but cannot
		// use either discriminator to suppress source-body fallback.
		if ground.Tier == RetrievalTierAuthoredPhrase {
			ground.Tier = retrievalTierOptionalCandidate
		}
		if ground.SourceField == sourceFieldDerivedPhrase {
			ground.SourceField = sourceFieldOptionalCandidateMatch
		}
		evidence, err := canonicalizeOptionalMatchGroundEvidence(
			index,
			producerID,
			candidateUnitID,
			ground.Evidence,
		)
		if err != nil {
			return nil, err
		}
		ground.Evidence = evidence
		canonical = append(canonical, ground)
	}
	return canonical, nil
}

func canonicalizeOptionalMatchGroundEvidence(
	index QueryIndex,
	producerID string,
	candidateUnitID string,
	evidence *MatchGroundEvidence,
) (*MatchGroundEvidence, error) {
	if evidence == nil {
		return nil, nil
	}
	evidenceUnitID := strings.TrimSpace(evidence.UnitID)
	if evidenceUnitID == "" {
		return nil, fmt.Errorf(
			"optional candidate producer %s returned unit %s with evidence lacking unit id",
			producerID,
			candidateUnitID,
		)
	}
	evidenceRoles := append([]SourceUnitRole(nil), sourceUnitRoleOrder...)
	unit, found, err := index.LookupExact(evidenceUnitID, evidenceRoles)
	if err != nil {
		return nil, fmt.Errorf(
			"hydrate optional candidate producer %s evidence %s: %w",
			producerID,
			evidenceUnitID,
			err,
		)
	}
	if !found || unit.UnitID != evidenceUnitID {
		return nil, fmt.Errorf(
			"optional candidate producer %s returned unit %s with unknown evidence unit %s",
			producerID,
			candidateUnitID,
			evidenceUnitID,
		)
	}
	if err := validateSourceProvenance(unit); err != nil {
		return nil, fmt.Errorf(
			"hydrate optional candidate producer %s evidence %s: %w",
			producerID,
			evidenceUnitID,
			err,
		)
	}
	return &MatchGroundEvidence{
		UnitID:             unit.UnitID,
		PatternID:          unit.PatternID,
		SourceRole:         unit.Role,
		Provenance:         unit.Provenance,
		ProjectionRelation: strings.TrimSpace(evidence.ProjectionRelation),
	}, nil
}

func assembleCandidateGroups(roles []SourceUnitRole, budget ResponseBudget, batches ...CandidateBatch) ([]SourceCandidateGroup, CandidateTruncation) {
	merged, producerOmissions, producerBasis := mergeCandidateBatches(batches)
	selected := selectCandidatesWithinResponseBudget(merged, roles, budget)
	groups := make([]SourceCandidateGroup, 0, len(roles))
	included := 0
	omitted := producerOmissions
	basis := append([]string(nil), producerBasis...)

	for _, role := range roles {
		roleCandidates := candidatesForRole(merged, role)
		visible := candidatesForRole(selected, role)
		omitted += len(roleCandidates) - len(visible)
		if len(roleCandidates) > len(visible) {
			basis = appendUnique(basis, "response_budget")
		}
		if len(visible) == 0 {
			continue
		}
		projected := projectCandidates(
			visible,
			budget.MaxExcerptCharacters,
			budget.MaxRelationsPerCandidate,
		)
		groups = append(groups, SourceCandidateGroup{
			Role:       role,
			Candidates: projected,
		})
		included += len(visible)
	}

	return groups, CandidateTruncation{
		Applied:            omitted > 0,
		Budget:             budget,
		IncludedCandidates: included,
		OmittedAtLeast:     omitted,
		Basis:              dedupeStrings(basis),
	}
}

func selectCandidatesWithinResponseBudget(candidates []RetrievedCandidate, roles []SourceUnitRole, budget ResponseBudget) []RetrievedCandidate {
	selected := make([]RetrievedCandidate, 0, min(len(candidates), budget.MaxTotalCandidates))
	selectedIDs := make(map[string]struct{})
	roleCounts := make(map[SourceUnitRole]int, len(roles))
	appendCandidate := func(candidate RetrievedCandidate) {
		if len(selected) >= budget.MaxTotalCandidates {
			return
		}
		if roleCounts[candidate.Unit.Role] >= budget.MaxCandidatesPerRole {
			return
		}
		if _, exists := selectedIDs[candidate.Unit.UnitID]; exists {
			return
		}
		selected = append(selected, candidate)
		selectedIDs[candidate.Unit.UnitID] = struct{}{}
		roleCounts[candidate.Unit.Role]++
	}
	for _, candidate := range candidates {
		if !candidateHasTier(candidate, RetrievalTierExactSource) {
			continue
		}
		appendCandidate(candidate)
	}
	for _, role := range roles {
		for _, candidate := range candidatesForRole(candidates, role) {
			appendCandidate(candidate)
		}
	}
	return selected
}

func candidateHasTier(candidate RetrievedCandidate, tier RetrievalTier) bool {
	for _, ground := range candidate.MatchGrounds {
		if ground.Tier == tier {
			return true
		}
	}
	return false
}

func mergeCandidateBatches(batches []CandidateBatch) ([]RetrievedCandidate, int, []string) {
	merged := make([]RetrievedCandidate, 0)
	positions := make(map[string]int)
	omitted := 0
	basis := make([]string, 0)

	for _, batch := range batches {
		if batch.Truncated {
			omitted += max(1, batch.OmittedAtLeast)
			basis = append(basis, batch.OmittedBasis...)
		}
		for _, candidate := range batch.Candidates {
			position, exists := positions[candidate.Unit.UnitID]
			if !exists {
				positions[candidate.Unit.UnitID] = len(merged)
				merged = append(merged, candidate)
				continue
			}
			if merged[position].RelationProjection == nil && candidate.RelationProjection != nil {
				merged[position].RelationProjection = cloneCandidateRelationProjection(*candidate.RelationProjection)
			}
			merged[position].MatchGrounds = mergeMatchGrounds(
				merged[position].MatchGrounds,
				candidate.MatchGrounds,
			)
		}
	}
	return merged, omitted, dedupeStrings(basis)
}

func mergeMatchGrounds(left, right []MatchGround) []MatchGround {
	merged := append([]MatchGround(nil), left...)
	seen := make(map[string]struct{}, len(merged))
	for _, ground := range merged {
		seen[matchGroundKey(ground)] = struct{}{}
	}
	for _, ground := range right {
		key := matchGroundKey(ground)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, ground)
	}
	return merged
}

func matchGroundKey(ground MatchGround) string {
	evidenceKey := ""
	if ground.Evidence != nil {
		evidenceKey = ground.Evidence.UnitID + "\x00" + ground.Evidence.ProjectionRelation
	}
	return string(ground.Tier) + "\x00" + ground.ProbeField + "\x00" + ground.SourceField + "\x00" + ground.MatchedValue + "\x00" + string(ground.PhraseKind) + "\x00" + evidenceKey
}

func candidatesForRole(candidates []RetrievedCandidate, role SourceUnitRole) []RetrievedCandidate {
	filtered := make([]RetrievedCandidate, 0)
	for _, candidate := range candidates {
		if candidate.Unit.Role == role {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func projectCandidates(candidates []RetrievedCandidate, excerptLimit, relationLimit int) []SourceCandidate {
	projected := make([]SourceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		text := projectCandidateSourceText(candidate.Unit, excerptLimit)
		projected = append(projected, SourceCandidate{
			Source: CandidateSourceUnit{
				UnitID:             candidate.Unit.UnitID,
				SourceID:           candidate.Unit.SourceID,
				SourceRole:         candidate.Unit.Role,
				Title:              candidate.Unit.Title,
				Excerpt:            text.Excerpt,
				ExcerptTruncated:   text.ExcerptTruncated,
				UseCues:            text.UseCues,
				UseCuesTruncated:   text.UseCuesTruncated,
				PatternID:          candidate.Unit.PatternID,
				ParentPatternID:    candidate.Unit.ParentPatternID,
				PublicationStatus:  candidate.Unit.PublicationStatus,
				DirectRefs:         append([]string(nil), candidate.Unit.DirectRefs...),
				RelationProjection: projectCandidateRelations(candidate.RelationProjection, relationLimit),
				Provenance:         candidate.Unit.Provenance,
			},
			MatchGrounds: append([]MatchGround(nil), candidate.MatchGrounds...),
		})
	}
	return projected
}

func projectCandidateRelations(projection *CandidateRelationProjection, limit int) *CandidateRelationProjection {
	if projection == nil {
		return nil
	}
	projected := *projection
	visible := min(len(projection.Relations), max(0, limit))
	projected.Relations = cloneSourceRelations(projection.Relations[:visible])
	projected.Truncated = len(projection.Relations) > visible
	projected.OmittedAtLeast = len(projection.Relations) - visible
	return &projected
}

func cloneSourceRelations(relations []SourceRelation) []SourceRelation {
	return append([]SourceRelation(nil), relations...)
}

type projectedCandidateSourceText struct {
	Excerpt          string
	ExcerptTruncated bool
	UseCues          *SourceUseCues
	UseCuesTruncated bool
}

func projectCandidateSourceText(unit SourceUnit, totalLimit int) projectedCandidateSourceText {
	values := []string{unit.Body}
	if unit.Role == SourceUnitRolePracticalUseCard {
		values = append(values,
			unit.UseCues.ConditionText,
			unit.UseCues.FirstResultText,
			unit.UseCues.StopReturnText,
		)
	}
	allocations := splitSourceTextBudget(values, totalLimit)
	excerpt, excerptTruncated := boundedSourceText(values[0], allocations[0])
	projection := projectedCandidateSourceText{
		Excerpt:          excerpt,
		ExcerptTruncated: excerptTruncated,
	}
	if unit.Role != SourceUnitRolePracticalUseCard {
		return projection
	}

	condition, conditionTruncated := boundedSourceText(values[1], allocations[1])
	firstResult, firstResultTruncated := boundedSourceText(values[2], allocations[2])
	stopReturn, stopReturnTruncated := boundedSourceText(values[3], allocations[3])
	projection.UseCues = &SourceUseCues{
		ConditionText:   condition,
		FirstResultText: firstResult,
		StopReturnText:  stopReturn,
	}
	projection.UseCuesTruncated = conditionTruncated || firstResultTruncated || stopReturnTruncated
	return projection
}

func splitSourceTextBudget(values []string, totalLimit int) []int {
	allocations := make([]int, len(values))
	nonEmpty := make([]int, 0, len(values))
	for index, value := range values {
		if strings.TrimSpace(value) != "" {
			nonEmpty = append(nonEmpty, index)
		}
	}
	if len(nonEmpty) == 0 || totalLimit <= 0 {
		return allocations
	}

	base := totalLimit / len(nonEmpty)
	remainder := totalLimit % len(nonEmpty)
	for position, index := range nonEmpty {
		allocations[index] = base
		if position < remainder {
			allocations[index]++
		}
	}
	return allocations
}

func boundedSourceText(value string, limit int) (string, bool) {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed, false
	}
	if limit <= 0 {
		return "", true
	}
	if limit == 1 {
		return "…", true
	}
	excerpt := strings.TrimSpace(string(runes[:limit-1]))
	return excerpt + "…", true
}

func normalizeCandidateProbe(probe CandidateProbe) CandidateProbe {
	probe.Text = strings.TrimSpace(probe.Text)
	probe.EntityOfConcern = strings.TrimSpace(probe.EntityOfConcern)
	probe.IntendedUse = strings.TrimSpace(probe.IntendedUse)
	probe.KnownContext = normalizeNonEmptyStrings(probe.KnownContext)
	return probe
}

func normalizeResponseBudget(budget ResponseBudget) ResponseBudget {
	if budget.MaxCandidatesPerRole <= 0 {
		budget.MaxCandidatesPerRole = defaultResponseBudget.MaxCandidatesPerRole
	}
	if budget.MaxTotalCandidates <= 0 {
		budget.MaxTotalCandidates = defaultResponseBudget.MaxTotalCandidates
	}
	if budget.MaxExcerptCharacters <= 0 {
		budget.MaxExcerptCharacters = defaultResponseBudget.MaxExcerptCharacters
	}
	if budget.MaxRelationsPerCandidate <= 0 {
		budget.MaxRelationsPerCandidate = defaultResponseBudget.MaxRelationsPerCandidate
	}
	budget.MaxCandidatesPerRole = min(budget.MaxCandidatesPerRole, 50)
	budget.MaxTotalCandidates = min(budget.MaxTotalCandidates, 100)
	budget.MaxExcerptCharacters = min(budget.MaxExcerptCharacters, 4000)
	budget.MaxRelationsPerCandidate = min(budget.MaxRelationsPerCandidate, 100)
	return budget
}

func validateResponseBudget(budget ResponseBudget) error {
	if budget.MaxCandidatesPerRole < 0 {
		return fmt.Errorf("max candidates per role must be non-negative")
	}
	if budget.MaxTotalCandidates < 0 {
		return fmt.Errorf("max total candidates must be non-negative")
	}
	if budget.MaxExcerptCharacters < 0 {
		return fmt.Errorf("max excerpt characters must be non-negative")
	}
	if budget.MaxRelationsPerCandidate < 0 {
		return fmt.Errorf("max relations per candidate must be non-negative")
	}
	return nil
}

func normalizeConcernRoles(roles []SourceUnitRole) ([]SourceUnitRole, error) {
	if len(roles) == 0 {
		return []SourceUnitRole{
			SourceUnitRolePracticalUseCard,
			SourceUnitRoleTOCRow,
		}, nil
	}
	return NormalizeSourceUnitRoles(roles)
}

func NormalizeSourceUnitRoles(roles []SourceUnitRole) ([]SourceUnitRole, error) {
	if len(roles) == 0 {
		return append([]SourceUnitRole(nil), sourceUnitRoleOrder...), nil
	}

	requested := make(map[SourceUnitRole]struct{}, len(roles))
	for _, role := range roles {
		if !isSourceUnitRole(role) {
			return nil, fmt.Errorf("unsupported source unit role %q", role)
		}
		requested[role] = struct{}{}
	}

	normalized := make([]SourceUnitRole, 0, len(requested))
	for _, role := range sourceUnitRoleOrder {
		if _, exists := requested[role]; exists {
			normalized = append(normalized, role)
		}
	}
	return normalized, nil
}

func isSourceUnitRole(role SourceUnitRole) bool {
	for _, known := range sourceUnitRoleOrder {
		if role == known {
			return true
		}
	}
	return false
}

func validateUniqueSourceIDs(units []SourceUnit) error {
	seen := make(map[string]string)
	for _, unit := range units {
		sourceID := strings.ToLower(strings.TrimSpace(unit.SourceID))
		if sourceID == "" {
			continue
		}
		if existing, exists := seen[sourceID]; exists {
			return fmt.Errorf("source id %q is ambiguous between %s and %s", unit.SourceID, existing, unit.UnitID)
		}
		seen[sourceID] = unit.UnitID
	}
	return nil
}

func normalizeNonEmptyStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	return dedupeStrings(normalized)
}

func newExactHit(identifier string, unit SourceUnit) ExactHit {
	return ExactHit{
		Kind:       QueryResultKindExactHit,
		Identifier: identifier,
		Unit:       unit,
	}
}

func newQueryAbstention(query, reason string, missingBasis []string) Abstained {
	return Abstained{
		Kind:         QueryResultKindAbstained,
		Query:        query,
		Reason:       reason,
		MissingBasis: append([]string(nil), missingBasis...),
	}
}
