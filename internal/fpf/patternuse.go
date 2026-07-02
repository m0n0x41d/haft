package fpf

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	PatternUseSchemaVersion       = 1
	PatternUseIndexRecordKind     = "pattern_use_index"
	PatternUseRecordKind          = "pattern_use_recommendation"
	PatternUseGatewayRecordKind   = "pattern_use_gateway"
	PatternUseIndexAuthority      = "read_only_pattern_use_index_not_normative_fpf_source"
	PatternUseAuthority           = "advisory_pattern_use_record"
	PatternUseFullMode            = PatternUseMode("full")
	PatternUseCompactMode         = PatternUseMode("compact")
	PatternUseShouldUseTrue       = PatternUseShouldUsePattern("true")
	PatternUseShouldUseFalse      = PatternUseShouldUsePattern("false")
	PatternUseShouldUseAbstain    = PatternUseShouldUsePattern("abstain")
	PatternUseSuggestedSurfaceNil = "none"
	PatternUseSeedIndexCoverage   = "seed_route_cards_not_full_fpf_catalog"
)

const (
	PatternUseRouteMatchStrategyNone                  = "none"
	PatternUseRouteMatchStrategyDeterministicCue      = "deterministic_cue"
	PatternUseRouteMatchStrategySemanticCompiledRoute = "semantic_compiled_route"
	PatternUseRouteMatchStrategyRetrievedUncompiled   = "retrieved_uncompiled"
	PatternUseIntentMatchStrategySemanticLane         = "semantic_intent_lane"
)

const (
	PatternUseSemanticTopK          = 5
	PatternUseSemanticMinSimilarity = 0.24
	PatternUseSemanticMinMargin     = 0.03
	PatternUseSemanticNegativeSlack = 0.02
)

type PatternUseMode string

type PatternUseShouldUsePattern string

type PatternUseSupportLevel string

const (
	PatternUseSupportImplementedSubstrate PatternUseSupportLevel = "implemented_substrate"
	PatternUseSupportPromptLevel          PatternUseSupportLevel = "prompt_level"
	PatternUseSupportRetrievedUncompiled  PatternUseSupportLevel = "retrieved_uncompiled"
	PatternUseSupportMissing              PatternUseSupportLevel = "missing"
	PatternUseSupportAbstain              PatternUseSupportLevel = "abstain"
)

type PatternUseRequest struct {
	Query             string   `json:"query"`
	Mode              string   `json:"mode,omitempty"`
	ProjectRef        string   `json:"project_ref,omitempty"`
	BoundedContextRef string   `json:"bounded_context_ref,omitempty"`
	SourceRefs        []string `json:"source_refs,omitempty"`
}

type PatternUseContext struct {
	ProjectRef        string   `json:"project_ref"`
	BoundedContextRef string   `json:"bounded_context_ref"`
	SourceRefs        []string `json:"source_refs"`
}

type PatternUseReferenceSet struct {
	PatternRefs []string `json:"pattern_refs,omitempty"`
	SurfaceRefs []string `json:"surface_refs,omitempty"`
	RouteRefs   []string `json:"route_refs,omitempty"`
	CatalogRefs []string `json:"catalog_refs,omitempty"`
}

type CandidatePatternUse struct {
	PatternRef          string                `json:"pattern_ref"`
	Title               string                `json:"title"`
	ApplicabilityReason string                `json:"applicability_reason"`
	SourceTier          string                `json:"source_tier,omitempty"`
	SourceReason        string                `json:"source_reason,omitempty"`
	Summary             string                `json:"summary,omitempty"`
	Snippet             string                `json:"snippet,omitempty"`
	SourceCard          *PatternUseSourceCard `json:"source_card,omitempty"`
}

type ApplicablePatternUse struct {
	PatternRef string `json:"pattern_ref"`
	Title      string `json:"title"`
}

type PatternUseRef struct {
	PatternRef string `json:"pattern_ref"`
	Title      string `json:"title"`
}

type PatternUseSourceCard struct {
	BodyKind    string `json:"body_kind"`
	SourceRef   string `json:"source_ref"`
	FPFCommit   string `json:"fpf_commit"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	RootNodeID  string `json:"root_node_id"`
	ContentHash string `json:"content_hash"`
	NodeCount   int    `json:"node_count"`
	Body        string `json:"body,omitempty"`
}

type WrongPatternBoundary struct {
	TemptingPatternOrMove string `json:"tempting_pattern_or_move"`
	WhyWrongNow           string `json:"why_wrong_now"`
}

type RequiredOutputShape struct {
	CarrierKind      string   `json:"carrier_kind"`
	RequiredSections []string `json:"required_sections"`
}

type RequiredEvidenceOrSoTA struct {
	Requirement           string `json:"requirement"`
	FreshnessOrSourceRule string `json:"freshness_or_source_rule"`
}

type BlockedStrongerUse struct {
	BlockedUse       string `json:"blocked_use"`
	UnblockCondition string `json:"unblock_condition"`
}

type CloseoutOrVerificationExpectation struct {
	Expectation string `json:"expectation"`
}

type PatternUseRecommendation struct {
	SchemaVersion                     int                                 `json:"schema_version"`
	RecordKind                        string                              `json:"record_kind"`
	Authority                         string                              `json:"authority"`
	ProjectConcernRef                 string                              `json:"project_concern_ref"`
	UseContext                        PatternUseContext                   `json:"use_context"`
	PatternUseRefs                    PatternUseReferenceSet              `json:"pattern_use_refs,omitempty"`
	CandidatePatternUseSet            []CandidatePatternUse               `json:"candidate_pattern_use_set"`
	ApplicablePatternUseSet           []ApplicablePatternUse              `json:"applicable_pattern_use_set"`
	RecommendedPatternUse             PatternUseRef                       `json:"recommended_pattern_use"`
	ReasonForRecommendation           string                              `json:"reason_for_recommendation"`
	WrongPatternBoundary              []WrongPatternBoundary              `json:"wrong_pattern_boundary"`
	RequiredOutputShape               RequiredOutputShape                 `json:"required_output_shape"`
	RequiredEvidenceOrSoTA            []RequiredEvidenceOrSoTA            `json:"required_evidence_or_sota"`
	BlockedStrongerUse                []BlockedStrongerUse                `json:"blocked_stronger_use"`
	CloseoutOrVerificationExpectation []CloseoutOrVerificationExpectation `json:"closeout_or_verification_expectation"`
	SupportLevel                      PatternUseSupportLevel              `json:"support_level"`
	SuggestedHaftSurface              string                              `json:"suggested_haft_surface,omitempty"`
	SuggestedMethodRefs               []string                            `json:"suggested_method_refs,omitempty"`
	NextGoverningPatternRefs          []string                            `json:"next_governing_pattern_refs"`
	MatchedRouteID                    string                              `json:"matched_route_id,omitempty"`
	MatchedRecognitionCues            []string                            `json:"matched_recognition_cues,omitempty"`
	RouteMatchStrategy                string                              `json:"route_match_strategy,omitempty"`
	RouteMatchScore                   float64                             `json:"route_match_score,omitempty"`
	RouteMatchMargin                  float64                             `json:"route_match_margin,omitempty"`
	RouteMatchContract                string                              `json:"route_match_contract,omitempty"`
	IntentLane                        PatternUseIntentLane                `json:"intent_lane,omitempty"`
	IntentMatchStrategy               string                              `json:"intent_match_strategy,omitempty"`
	IntentMatchScore                  float64                             `json:"intent_match_score,omitempty"`
	IntentMatchMargin                 float64                             `json:"intent_match_margin,omitempty"`
	IntentMatchContract               string                              `json:"intent_match_contract,omitempty"`
	AuthorityBoundary                 []string                            `json:"authority_boundary"`
}

type PatternUseCompactRecommendation struct {
	SchemaVersion             int                        `json:"schema_version"`
	RecordKind                string                     `json:"record_kind"`
	Authority                 string                     `json:"authority"`
	ProjectConcernRef         string                     `json:"project_concern_ref"`
	ShouldUsePattern          PatternUseShouldUsePattern `json:"should_use_pattern"`
	PatternUseRefs            PatternUseReferenceSet     `json:"pattern_use_refs,omitempty"`
	CandidatePatternUseSet    []CandidatePatternUse      `json:"candidate_pattern_use_set,omitempty"`
	RecommendedPatternUse     PatternUseRef              `json:"recommended_pattern_use"`
	SuggestedHaftSurface      string                     `json:"suggested_haft_surface"`
	SuggestedMethodRefs       []string                   `json:"suggested_method_refs,omitempty"`
	SupportLevel              PatternUseSupportLevel     `json:"support_level"`
	OneLineReason             string                     `json:"one_line_reason"`
	OneLineBoundary           string                     `json:"one_line_boundary"`
	FullRecommendationCommand string                     `json:"full_recommendation_command"`
	MatchedRouteID            string                     `json:"matched_route_id,omitempty"`
	MatchedRecognitionCues    []string                   `json:"matched_recognition_cues,omitempty"`
	RouteMatchStrategy        string                     `json:"route_match_strategy,omitempty"`
	IntentLane                PatternUseIntentLane       `json:"intent_lane,omitempty"`
	IntentMatchStrategy       string                     `json:"intent_match_strategy,omitempty"`
	AuthorityBoundary         []string                   `json:"authority_boundary"`
}

type PatternUseIndex struct {
	SchemaVersion                int                          `json:"schema_version"`
	RecordKind                   string                       `json:"record_kind"`
	Authority                    string                       `json:"authority"`
	Source                       string                       `json:"source"`
	Coverage                     string                       `json:"coverage"`
	FullFPFCatalogCovered        bool                         `json:"full_fpf_catalog_covered"`
	CompiledRouteCardCount       int                          `json:"compiled_route_card_count"`
	SemanticRouteDocumentCount   int                          `json:"semantic_route_document_count"`
	SemanticRouteEmbeddingCount  int                          `json:"semantic_route_embedding_count"`
	SemanticIntentDocumentCount  int                          `json:"semantic_intent_document_count"`
	SemanticIntentEmbeddingCount int                          `json:"semantic_intent_embedding_count"`
	SemanticRouteEmbeddingModel  string                       `json:"semantic_route_embedding_model,omitempty"`
	EmbeddingContract            *PatternUseEmbeddingContract `json:"embedding_contract,omitempty"`
	RetrievablePatternCardCount  int                          `json:"retrievable_pattern_card_count"`
	CoverageNotes                []string                     `json:"coverage_notes"`
	RouteCards                   []PatternUseRouteCard        `json:"route_cards"`
	RetrievalStrategy            []string                     `json:"retrieval_strategy"`
	AuthorityBoundary            []string                     `json:"authority_boundary"`
}

type PatternUseEmbeddingContract struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Dim      int    `json:"dim"`
	Label    string `json:"label"`
}

type PatternUseRouteCard struct {
	ID                                string                              `json:"id"`
	RecognitionCues                   []string                            `json:"recognition_cues"`
	SemanticTrigger                   string                              `json:"semantic_trigger,omitempty"`
	PositiveExamples                  []string                            `json:"positive_examples,omitempty"`
	NegativeExamples                  []string                            `json:"negative_examples,omitempty"`
	CandidatePatternUseSet            []CandidatePatternUse               `json:"candidate_pattern_use_set"`
	ApplicablePatternUseSet           []ApplicablePatternUse              `json:"applicable_pattern_use_set"`
	RecommendedPatternUse             PatternUseRef                       `json:"recommended_pattern_use"`
	ReasonForRecommendation           string                              `json:"reason_for_recommendation"`
	WrongPatternBoundary              []WrongPatternBoundary              `json:"wrong_pattern_boundary"`
	RequiredOutputShape               RequiredOutputShape                 `json:"required_output_shape"`
	RequiredEvidenceOrSoTA            []RequiredEvidenceOrSoTA            `json:"required_evidence_or_sota"`
	BlockedStrongerUse                []BlockedStrongerUse                `json:"blocked_stronger_use"`
	CloseoutOrVerificationExpectation []CloseoutOrVerificationExpectation `json:"closeout_or_verification_expectation"`
	SupportLevel                      PatternUseSupportLevel              `json:"support_level"`
	SuggestedHaftSurface              string                              `json:"suggested_haft_surface,omitempty"`
	SuggestedMethodRefs               []string                            `json:"suggested_method_refs,omitempty"`
	NextGoverningPatternRefs          []string                            `json:"next_governing_pattern_refs"`
}

type patternUseRouteScore struct {
	route    PatternUseRouteCard
	cues     []string
	score    int
	strategy string
	match    PatternUseRouteMatch
}

type PatternUseRouteMatch struct {
	RouteID            string   `json:"route_id"`
	Strategy           string   `json:"strategy"`
	Score              float64  `json:"score"`
	Margin             float64  `json:"margin"`
	Contract           string   `json:"contract"`
	MatchedDocumentIDs []string `json:"matched_document_ids,omitempty"`
}

type PatternUseIntentLane string

const (
	PatternUseIntentApplyPattern     PatternUseIntentLane = "apply_pattern"
	PatternUseIntentExplainPattern   PatternUseIntentLane = "explain_pattern"
	PatternUseIntentComparePatterns  PatternUseIntentLane = "compare_patterns"
	PatternUseIntentAuditRouter      PatternUseIntentLane = "audit_router"
	PatternUseIntentCatalogMeta      PatternUseIntentLane = "catalog_meta"
	PatternUseIntentMechanicalLookup PatternUseIntentLane = "mechanical_lookup"
	PatternUseIntentStatusLookup     PatternUseIntentLane = "status_lookup"
	PatternUseIntentUnknown          PatternUseIntentLane = "unknown"
)

type PatternUseIntentLaneCard struct {
	ID               PatternUseIntentLane `json:"id"`
	SemanticTrigger  string               `json:"semantic_trigger"`
	PositiveExamples []string             `json:"positive_examples,omitempty"`
	NegativeExamples []string             `json:"negative_examples,omitempty"`
}

type PatternUseIntentLaneMatch struct {
	Lane               PatternUseIntentLane `json:"lane"`
	Strategy           string               `json:"strategy"`
	Score              float64              `json:"score"`
	Margin             float64              `json:"margin"`
	Contract           string               `json:"contract"`
	MatchedDocumentIDs []string             `json:"matched_document_ids,omitempty"`
}

var patternUseAuthorityBoundary = []string{
	"read_only_recommendation_not_decision",
	"not_work_commission",
	"not_methodrun_gate_result",
	"not_evidence_creation",
	"not_approval",
	"not_gate_passage",
	"not_claim_truth",
	"not_global_truth",
	"not_publication",
}

func RecommendPatternUse(query string) PatternUseRecommendation {
	return RecommendPatternUseWithContext(PatternUseRequest{Query: query})
}

func NormalizePatternUseMode(mode string) (PatternUseMode, error) {
	normalized := strings.TrimSpace(strings.ToLower(mode))
	if normalized == "" {
		return PatternUseCompactMode, nil
	}
	switch PatternUseMode(normalized) {
	case PatternUseCompactMode, PatternUseFullMode:
		return PatternUseMode(normalized), nil
	default:
		return "", fmt.Errorf("unsupported pattern_use mode %q (want compact or full)", mode)
	}
}

func RecommendPatternUseWithContext(request PatternUseRequest) PatternUseRecommendation {
	score := patternUseRouteScore{
		route:    fallbackPatternUseRouteCard(),
		score:    0,
		strategy: PatternUseRouteMatchStrategyNone,
	}
	recommendation := recommendationFromPatternUseRoute(request, score)
	return recommendation.withValidatedShape()
}

func RecommendPatternUseWithSemanticRouteMatch(
	request PatternUseRequest,
	match PatternUseRouteMatch,
) PatternUseRecommendation {
	route, ok := patternUseRouteCardByID(DefaultPatternUseRouteCards(), match.RouteID)
	if !ok {
		return RecommendPatternUseWithContext(request)
	}
	if route.ID == "e11_patternuse_fallback" {
		return RecommendPatternUseWithContext(request)
	}

	score := patternUseRouteScore{
		route:    route,
		score:    1,
		strategy: firstNonEmptyPatternUseString(match.Strategy, PatternUseRouteMatchStrategySemanticCompiledRoute),
		match:    match,
	}
	record := recommendationFromPatternUseRoute(request, score)
	record.MatchedRecognitionCues = []string{}
	return record.withValidatedShape()
}

func RecommendPatternUseWithSemanticRouteAndIntentMatch(
	request PatternUseRequest,
	routeMatch PatternUseRouteMatch,
	intentMatch PatternUseIntentLaneMatch,
) PatternUseRecommendation {
	record := RecommendPatternUseWithSemanticRouteMatch(request, routeMatch)
	return WithPatternUseIntentMatch(record, intentMatch)
}

func WithPatternUseIntentMatch(
	record PatternUseRecommendation,
	match PatternUseIntentLaneMatch,
) PatternUseRecommendation {
	record.IntentLane = match.Lane
	record.IntentMatchStrategy = strings.TrimSpace(match.Strategy)
	record.IntentMatchScore = match.Score
	record.IntentMatchMargin = match.Margin
	record.IntentMatchContract = strings.TrimSpace(match.Contract)
	return record.withValidatedShape()
}

func RecommendPatternUseCompact(query string) PatternUseCompactRecommendation {
	return RecommendPatternUseCompactWithContext(PatternUseRequest{Query: query})
}

func RecommendPatternUseCompactWithContext(request PatternUseRequest) PatternUseCompactRecommendation {
	record := RecommendPatternUseWithContext(request)
	return compactRecommendationFromPatternUse(record)
}

func CompactPatternUseRecommendation(record PatternUseRecommendation) PatternUseCompactRecommendation {
	return compactRecommendationFromPatternUse(record.withValidatedShape())
}

func DefaultPatternUseIndex() PatternUseIndex {
	return PatternUseIndexWithRetrievablePatternCardCount(0)
}

func PatternUseIndexWithRetrievablePatternCardCount(patternCardCount int) PatternUseIndex {
	return PatternUseIndexWithRetrievalAndSemanticSummary(patternCardCount, 0, "")
}

func PatternUseIndexWithRetrievalAndSemanticSummary(
	patternCardCount int,
	semanticRouteEmbeddingCount int,
	semanticRouteEmbeddingModel string,
) PatternUseIndex {
	contract := PatternUseEmbeddingContractFromLabel(semanticRouteEmbeddingModel)
	return PatternUseIndexWithRetrievalAndSemanticContract(patternCardCount, semanticRouteEmbeddingCount, contract)
}

func PatternUseIndexWithRetrievalAndSemanticContract(
	patternCardCount int,
	semanticRouteEmbeddingCount int,
	contract PatternUseEmbeddingContract,
) PatternUseIndex {
	return PatternUseIndexWithRetrievalRouteAndIntentSemanticContract(
		patternCardCount,
		semanticRouteEmbeddingCount,
		0,
		contract,
	)
}

func PatternUseIndexWithRetrievalRouteAndIntentSemanticContract(
	patternCardCount int,
	semanticRouteEmbeddingCount int,
	semanticIntentEmbeddingCount int,
	contract PatternUseEmbeddingContract,
) PatternUseIndex {
	routeCards := seedPatternUseRouteCards()
	routeDocuments := PatternUseRouteEmbeddingDocuments(routeCards)
	intentDocuments := PatternUseIntentEmbeddingDocuments(DefaultPatternUseIntentLaneCards())
	return PatternUseIndex{
		SchemaVersion:                PatternUseSchemaVersion,
		RecordKind:                   PatternUseIndexRecordKind,
		Authority:                    PatternUseIndexAuthority,
		Source:                       "compiled_seed_route_cards_plus_optional_embedded_fpf_retrieval",
		Coverage:                     PatternUseSeedIndexCoverage,
		CompiledRouteCardCount:       len(routeCards),
		SemanticRouteDocumentCount:   len(routeDocuments),
		SemanticRouteEmbeddingCount:  semanticRouteEmbeddingCount,
		SemanticIntentDocumentCount:  len(intentDocuments),
		SemanticIntentEmbeddingCount: semanticIntentEmbeddingCount,
		SemanticRouteEmbeddingModel:  contract.labelForEmbeddingCount(max(semanticRouteEmbeddingCount, semanticIntentEmbeddingCount)),
		EmbeddingContract:            contract.pointerForEmbeddingCount(max(semanticRouteEmbeddingCount, semanticIntentEmbeddingCount)),
		RetrievablePatternCardCount:  patternCardCount,
		CoverageNotes: []string{
			"compiled seed route cards are high-confidence task routes with concrete output shapes",
			"semantic route documents are baked release artifacts for compiled PatternUse route selection",
			"semantic intent lane documents are baked release artifacts for apply/explain/catalog/audit separation",
			"retrievable FPF pattern cards are recall candidates only, not compiled PatternUse route cards",
			"full_fpf_catalog_covered remains false because V1 does not compile every FPF card into a route card",
		},
		RouteCards: routeCards,
		RetrievalStrategy: []string{
			"exact reference extraction identifies refs but does not select or apply routes",
			"semantic intent lane match separates apply/explain/catalog/audit/mechanical concerns",
			"semantic route match over baked compiled PatternUse route documents",
			"on seed miss, shell surfaces may retrieve top FPF pattern-card candidates from the embedded FPF search index",
			"retrieved_uncompiled candidates require full-card applicability checking before use",
			"skill and host carriers must call the gateway rather than inline route-card lists",
		},
		AuthorityBoundary: append([]string(nil), patternUseAuthorityBoundary...),
	}
}

func PatternUseEmbeddingContractFromLabel(label string) PatternUseEmbeddingContract {
	trimmed := strings.TrimSpace(label)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 {
		return PatternUseEmbeddingContract{Label: trimmed}
	}
	dim, ok := parsePositiveInt(parts[2])
	if !ok {
		return PatternUseEmbeddingContract{Label: trimmed}
	}
	return PatternUseEmbeddingContract{
		Provider: strings.TrimSpace(parts[0]),
		Model:    strings.TrimSpace(parts[1]),
		Dim:      dim,
		Label:    trimmed,
	}
}

func PatternUseEmbeddingContractFor(provider string, model string, dim int) PatternUseEmbeddingContract {
	trimmedProvider := strings.TrimSpace(provider)
	trimmedModel := strings.TrimSpace(model)
	label := strings.Join([]string{trimmedProvider, trimmedModel, fmt.Sprintf("%d", dim)}, "/")
	return PatternUseEmbeddingContract{
		Provider: trimmedProvider,
		Model:    trimmedModel,
		Dim:      dim,
		Label:    label,
	}
}

func parsePositiveInt(raw string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return parsed, parsed > 0
}

func DefaultPatternUseRouteCards() []PatternUseRouteCard {
	index := DefaultPatternUseIndex()
	return clonePatternUseRouteCards(index.RouteCards)
}

func seedPatternUseRouteCards() []PatternUseRouteCard {
	return []PatternUseRouteCard{
		namingPatternUseRouteCard(),
		architecturePatternUseRouteCard(),
		nextMovePatternUseRouteCard(),
		sotaPatternUseRouteCard(),
		diagnosisPatternUseRouteCard(),
		comparePatternUseRouteCard(),
		strictDistinctionPatternUseRouteCard(),
		evidenceProofPatternUseRouteCard(),
		publicAPIChangePatternUseRouteCard(),
		commitmentPatternUseRouteCard(),
		workPlanPerformedWorkPatternUseRouteCard(),
		agentActionAdmissibilityPatternUseRouteCard(),
		specLifecycleAuthorityPatternUseRouteCard(),
		layerBoundaryPatternUseRouteCard(),
		fallbackPatternUseRouteCard(),
	}
}

func (index PatternUseIndex) Validate() error {
	if index.SchemaVersion != PatternUseSchemaVersion {
		return fmt.Errorf("schema_version must be %d", PatternUseSchemaVersion)
	}
	if strings.TrimSpace(index.RecordKind) != PatternUseIndexRecordKind {
		return fmt.Errorf("record_kind must be %q", PatternUseIndexRecordKind)
	}
	if strings.TrimSpace(index.Authority) != PatternUseIndexAuthority {
		return fmt.Errorf("authority must be %q", PatternUseIndexAuthority)
	}
	if strings.TrimSpace(index.Coverage) != PatternUseSeedIndexCoverage {
		return fmt.Errorf("coverage must be %q", PatternUseSeedIndexCoverage)
	}
	if index.FullFPFCatalogCovered {
		return fmt.Errorf("full_fpf_catalog_covered must be false for the seed index")
	}
	if len(index.RouteCards) == 0 {
		return fmt.Errorf("route_cards is required")
	}
	if index.CompiledRouteCardCount != len(index.RouteCards) {
		return fmt.Errorf("compiled_route_card_count must equal len(route_cards)")
	}
	if index.SemanticRouteDocumentCount < 0 {
		return fmt.Errorf("semantic_route_document_count must be non-negative")
	}
	if index.SemanticRouteEmbeddingCount < 0 {
		return fmt.Errorf("semantic_route_embedding_count must be non-negative")
	}
	if index.SemanticIntentDocumentCount < 0 {
		return fmt.Errorf("semantic_intent_document_count must be non-negative")
	}
	if index.SemanticIntentEmbeddingCount < 0 {
		return fmt.Errorf("semantic_intent_embedding_count must be non-negative")
	}
	if index.SemanticRouteEmbeddingCount > 0 || index.SemanticIntentEmbeddingCount > 0 {
		if index.EmbeddingContract == nil {
			return fmt.Errorf("embedding_contract is required when semantic embedding counts are positive")
		}
		if err := index.EmbeddingContract.Validate(); err != nil {
			return fmt.Errorf("embedding_contract: %w", err)
		}
	}
	if index.RetrievablePatternCardCount < 0 {
		return fmt.Errorf("retrievable_pattern_card_count must be non-negative")
	}
	if len(index.CoverageNotes) == 0 {
		return fmt.Errorf("coverage_notes is required")
	}
	if !patternUseIndexHasRoute(index, "e11_patternuse_fallback") {
		return fmt.Errorf("fallback route card is required")
	}
	return nil
}

func (contract PatternUseEmbeddingContract) pointerForEmbeddingCount(count int) *PatternUseEmbeddingContract {
	if count <= 0 {
		return nil
	}
	return &contract
}

func (contract PatternUseEmbeddingContract) labelForEmbeddingCount(count int) string {
	if count <= 0 {
		return ""
	}
	return contract.Label
}

func (contract PatternUseEmbeddingContract) Validate() error {
	if strings.TrimSpace(contract.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(contract.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if contract.Dim <= 0 {
		return fmt.Errorf("dim must be positive")
	}
	if strings.TrimSpace(contract.Label) == "" {
		return fmt.Errorf("label is required")
	}
	expectedLabel := strings.Join(
		[]string{
			strings.TrimSpace(contract.Provider),
			strings.TrimSpace(contract.Model),
			fmt.Sprintf("%d", contract.Dim),
		},
		"/",
	)
	if strings.TrimSpace(contract.Label) != expectedLabel {
		return fmt.Errorf("label must equal %q", expectedLabel)
	}
	return nil
}

func (record PatternUseCompactRecommendation) Validate() error {
	if record.SchemaVersion != PatternUseSchemaVersion {
		return fmt.Errorf("schema_version must be %d", PatternUseSchemaVersion)
	}
	if strings.TrimSpace(record.RecordKind) != PatternUseGatewayRecordKind {
		return fmt.Errorf("record_kind must be %q", PatternUseGatewayRecordKind)
	}
	if strings.TrimSpace(record.Authority) != PatternUseAuthority {
		return fmt.Errorf("authority must be %q", PatternUseAuthority)
	}
	switch record.ShouldUsePattern {
	case PatternUseShouldUseTrue, PatternUseShouldUseFalse, PatternUseShouldUseAbstain:
	default:
		return fmt.Errorf("should_use_pattern must be true, false, or abstain")
	}
	if strings.TrimSpace(record.RecommendedPatternUse.PatternRef) == "" {
		return fmt.Errorf("recommended_pattern_use.pattern_ref is required")
	}
	if strings.TrimSpace(record.SuggestedHaftSurface) == "" {
		return fmt.Errorf("suggested_haft_surface is required")
	}
	if strings.TrimSpace(record.OneLineReason) == "" {
		return fmt.Errorf("one_line_reason is required")
	}
	if strings.TrimSpace(record.OneLineBoundary) == "" {
		return fmt.Errorf("one_line_boundary is required")
	}
	if strings.TrimSpace(record.FullRecommendationCommand) == "" {
		return fmt.Errorf("full_recommendation_command is required")
	}
	if record.SupportLevel == "" {
		return fmt.Errorf("support_level is required")
	}
	if record.MatchedRouteID == PatternUseRetrievedFallbackRouteID &&
		record.SupportLevel != PatternUseSupportRetrievedUncompiled {
		return fmt.Errorf("retrieval fallback route must use support_level %q", PatternUseSupportRetrievedUncompiled)
	}
	if record.SupportLevel == PatternUseSupportRetrievedUncompiled {
		if record.MatchedRouteID != PatternUseRetrievedFallbackRouteID {
			return fmt.Errorf("retrieved_uncompiled support requires matched_route_id %q", PatternUseRetrievedFallbackRouteID)
		}
		if len(record.CandidatePatternUseSet) == 0 {
			return fmt.Errorf("retrieved_uncompiled support requires candidate_pattern_use_set")
		}
	}
	return nil
}

func (record PatternUseRecommendation) Validate() error {
	if record.SchemaVersion != PatternUseSchemaVersion {
		return fmt.Errorf("schema_version must be %d", PatternUseSchemaVersion)
	}
	if strings.TrimSpace(record.RecordKind) != PatternUseRecordKind {
		return fmt.Errorf("record_kind must be %q", PatternUseRecordKind)
	}
	if strings.TrimSpace(record.Authority) != PatternUseAuthority {
		return fmt.Errorf("authority must be %q", PatternUseAuthority)
	}
	if strings.TrimSpace(record.RecommendedPatternUse.PatternRef) == "" {
		return fmt.Errorf("recommended_pattern_use.pattern_ref is required")
	}
	if strings.TrimSpace(record.RecommendedPatternUse.Title) == "" {
		return fmt.Errorf("recommended_pattern_use.title is required")
	}
	if strings.TrimSpace(record.ReasonForRecommendation) == "" {
		return fmt.Errorf("reason_for_recommendation is required")
	}
	if len(record.WrongPatternBoundary) == 0 {
		return fmt.Errorf("wrong_pattern_boundary is required")
	}
	if strings.TrimSpace(record.RequiredOutputShape.CarrierKind) == "" {
		return fmt.Errorf("required_output_shape.carrier_kind is required")
	}
	if len(record.RequiredOutputShape.RequiredSections) == 0 {
		return fmt.Errorf("required_output_shape.required_sections is required")
	}
	if len(record.RequiredEvidenceOrSoTA) == 0 {
		return fmt.Errorf("required_evidence_or_sota is required")
	}
	if len(record.BlockedStrongerUse) == 0 {
		return fmt.Errorf("blocked_stronger_use is required")
	}
	if len(record.CloseoutOrVerificationExpectation) == 0 {
		return fmt.Errorf("closeout_or_verification_expectation is required")
	}
	if record.SupportLevel == "" {
		return fmt.Errorf("support_level is required")
	}
	if record.SupportLevel == PatternUseSupportRetrievedUncompiled &&
		len(record.CandidatePatternUseSet) == 0 {
		return fmt.Errorf("retrieved_uncompiled support requires candidate_pattern_use_set")
	}
	if record.MatchedRouteID == PatternUseRetrievedFallbackRouteID &&
		record.SupportLevel != PatternUseSupportRetrievedUncompiled {
		return fmt.Errorf("retrieval fallback route must use support_level %q", PatternUseSupportRetrievedUncompiled)
	}
	if record.SupportLevel == PatternUseSupportRetrievedUncompiled {
		if record.MatchedRouteID != PatternUseRetrievedFallbackRouteID {
			return fmt.Errorf("retrieved_uncompiled support requires matched_route_id %q", PatternUseRetrievedFallbackRouteID)
		}
		if record.RequiredOutputShape.CarrierKind != "retrieved_pattern_applicability_card" {
			return fmt.Errorf("retrieved_uncompiled support requires retrieved_pattern_applicability_card output shape")
		}
	}
	return nil
}

func (record PatternUseRecommendation) HasAuthorityViolation() bool {
	text := strings.ToLower(patternUseRecommendationText(record))
	for _, forbidden := range []string{
		"approval granted",
		"decision created",
		"decisionrecord created",
		"workcommission created",
		"methodrun closed",
		"evidence created",
		"gate passed",
		"claim proven",
		"published",
	} {
		if strings.Contains(text, forbidden) {
			return true
		}
	}
	return false
}

func recommendationFromPatternUseRoute(
	request PatternUseRequest,
	score patternUseRouteScore,
) PatternUseRecommendation {
	concern, useContext := patternUseConcernAndContext(request, nil)

	return PatternUseRecommendation{
		SchemaVersion:                     PatternUseSchemaVersion,
		RecordKind:                        PatternUseRecordKind,
		Authority:                         PatternUseAuthority,
		ProjectConcernRef:                 concern,
		UseContext:                        useContext,
		PatternUseRefs:                    patternUseRefsForRoute(request.Query, score.route),
		CandidatePatternUseSet:            cloneCandidatePatternUses(score.route.CandidatePatternUseSet),
		ApplicablePatternUseSet:           cloneApplicablePatternUses(score.route.ApplicablePatternUseSet),
		RecommendedPatternUse:             score.route.RecommendedPatternUse,
		ReasonForRecommendation:           score.route.ReasonForRecommendation,
		WrongPatternBoundary:              cloneWrongPatternBoundaries(score.route.WrongPatternBoundary),
		RequiredOutputShape:               cloneRequiredOutputShape(score.route.RequiredOutputShape),
		RequiredEvidenceOrSoTA:            cloneRequiredEvidenceOrSoTA(score.route.RequiredEvidenceOrSoTA),
		BlockedStrongerUse:                cloneBlockedStrongerUse(score.route.BlockedStrongerUse),
		CloseoutOrVerificationExpectation: cloneCloseoutOrVerificationExpectations(score.route.CloseoutOrVerificationExpectation),
		SupportLevel:                      score.route.SupportLevel,
		SuggestedHaftSurface:              firstNonEmptyPatternUseString(score.route.SuggestedHaftSurface, suggestedHaftSurfaceForPatternUseRoute(score.route.ID)),
		SuggestedMethodRefs:               dedupePatternUseStrings(score.route.SuggestedMethodRefs),
		NextGoverningPatternRefs:          dedupePatternUseStrings(score.route.NextGoverningPatternRefs),
		MatchedRouteID:                    score.route.ID,
		MatchedRecognitionCues:            dedupePatternUseStrings(score.cues),
		RouteMatchStrategy:                firstNonEmptyPatternUseString(score.strategy, PatternUseRouteMatchStrategyNone),
		RouteMatchScore:                   score.match.Score,
		RouteMatchMargin:                  score.match.Margin,
		RouteMatchContract:                strings.TrimSpace(score.match.Contract),
		AuthorityBoundary:                 append([]string(nil), patternUseAuthorityBoundary...),
	}
}

func patternUseRefsForRoute(query string, route PatternUseRouteCard) PatternUseReferenceSet {
	refs := ExtractPatternUseRefs(query)
	if route.ID != "" && route.ID != "e11_patternuse_fallback" {
		refs.RouteRefs = append(refs.RouteRefs, route.ID)
	}
	refs.PatternRefs = append(refs.PatternRefs, patternUseRoutePatternRefs(route)...)
	return refs.withValidatedShape()
}

func patternUseRoutePatternRefs(route PatternUseRouteCard) []string {
	refs := []string{}
	for _, candidate := range route.CandidatePatternUseSet {
		refs = append(refs, candidate.PatternRef)
	}
	for _, applicable := range route.ApplicablePatternUseSet {
		refs = append(refs, applicable.PatternRef)
	}
	refs = append(refs, route.RecommendedPatternUse.PatternRef)
	refs = append(refs, route.NextGoverningPatternRefs...)
	return dedupePatternUseStrings(refs)
}

func patternUseConcernAndContext(request PatternUseRequest, sourceRefs []string) (string, PatternUseContext) {
	concern := strings.TrimSpace(request.Query)
	if concern == "" {
		concern = "missing_operator_concern"
	}
	projectRef := strings.TrimSpace(request.ProjectRef)
	if projectRef == "" {
		projectRef = "current_project"
	}
	contextRef := strings.TrimSpace(request.BoundedContextRef)
	if contextRef == "" {
		contextRef = "operator_task"
	}
	refs := append([]string{}, request.SourceRefs...)
	refs = append(refs, sourceRefs...)
	return concern, PatternUseContext{
		ProjectRef:        projectRef,
		BoundedContextRef: contextRef,
		SourceRefs:        dedupePatternUseStrings(refs),
	}
}

func (record PatternUseRecommendation) withValidatedShape() PatternUseRecommendation {
	if record.CandidatePatternUseSet == nil {
		record.CandidatePatternUseSet = []CandidatePatternUse{}
	}
	if record.ApplicablePatternUseSet == nil {
		record.ApplicablePatternUseSet = []ApplicablePatternUse{}
	}
	if record.UseContext.SourceRefs == nil {
		record.UseContext.SourceRefs = []string{}
	}
	record.PatternUseRefs = record.PatternUseRefs.withValidatedShape()
	if record.NextGoverningPatternRefs == nil {
		record.NextGoverningPatternRefs = []string{}
	}
	if record.SuggestedMethodRefs == nil {
		record.SuggestedMethodRefs = []string{}
	}
	if record.SuggestedHaftSurface == "" {
		record.SuggestedHaftSurface = PatternUseSuggestedSurfaceNil
	}
	if record.MatchedRecognitionCues == nil {
		record.MatchedRecognitionCues = []string{}
	}
	if record.RouteMatchStrategy == "" {
		record.RouteMatchStrategy = PatternUseRouteMatchStrategyNone
	}
	return record
}

func compactRecommendationFromPatternUse(record PatternUseRecommendation) PatternUseCompactRecommendation {
	shouldUse := PatternUseShouldUseTrue
	if record.SupportLevel == PatternUseSupportMissing ||
		record.SupportLevel == PatternUseSupportAbstain ||
		record.MatchedRouteID == "e11_patternuse_fallback" {
		shouldUse = PatternUseShouldUseAbstain
	}
	candidates := []CandidatePatternUse(nil)
	if record.SupportLevel == PatternUseSupportRetrievedUncompiled {
		candidates = compactCandidatePatternUses(record.CandidatePatternUseSet)
	}

	return PatternUseCompactRecommendation{
		SchemaVersion:             PatternUseSchemaVersion,
		RecordKind:                PatternUseGatewayRecordKind,
		Authority:                 PatternUseAuthority,
		ProjectConcernRef:         record.ProjectConcernRef,
		ShouldUsePattern:          shouldUse,
		PatternUseRefs:            record.PatternUseRefs.withValidatedShape(),
		CandidatePatternUseSet:    candidates,
		RecommendedPatternUse:     record.RecommendedPatternUse,
		SuggestedHaftSurface:      firstNonEmptyPatternUseString(record.SuggestedHaftSurface, PatternUseSuggestedSurfaceNil),
		SuggestedMethodRefs:       dedupePatternUseStrings(record.SuggestedMethodRefs),
		SupportLevel:              record.SupportLevel,
		OneLineReason:             firstSentencePatternUse(record.ReasonForRecommendation),
		OneLineBoundary:           firstPatternUseBoundary(record),
		FullRecommendationCommand: patternUseFullRecommendationCommand(record.ProjectConcernRef),
		MatchedRouteID:            record.MatchedRouteID,
		MatchedRecognitionCues:    dedupePatternUseStrings(record.MatchedRecognitionCues),
		RouteMatchStrategy:        record.RouteMatchStrategy,
		IntentLane:                record.IntentLane,
		IntentMatchStrategy:       record.IntentMatchStrategy,
		AuthorityBoundary:         append([]string(nil), patternUseAuthorityBoundary...),
	}
}

func suggestedHaftSurfaceForPatternUseRoute(routeID string) string {
	switch routeID {
	case "f18_naming_namecard":
		return "inline"
	case "c30_architecture_structures":
		return "h-explore"
	case "e11_e10_next_move":
		return "h-reason"
	case "e8_sota_evidence_pack":
		return "inline"
	case "diagnosis_rival_hypotheses":
		return "h-diagnose"
	case "characterize_compare_parity":
		return "h-compare"
	case "a10_b3_a7_evidence_proof":
		return "h-verify"
	case "a7_strict_distinction":
		return "inline"
	case "api_boundary_decision":
		return "h-frame"
	case "e9_commitment_human_gate":
		return "h-compare"
	case "a15_work_plan_performed_work_boundary":
		return "h-reason"
	case "e16_agent_action_admissibility":
		return "h-reason"
	case "spec_lifecycle_authority":
		return "h-spec"
	case "e4_layer_boundary":
		return "h-reason"
	default:
		return PatternUseSuggestedSurfaceNil
	}
}

func firstPatternUseBoundary(record PatternUseRecommendation) string {
	if len(record.BlockedStrongerUse) > 0 {
		return strings.TrimSpace(record.BlockedStrongerUse[0].BlockedUse)
	}
	if len(record.WrongPatternBoundary) > 0 {
		return strings.TrimSpace(record.WrongPatternBoundary[0].WhyWrongNow)
	}
	return "No stronger use without a full PatternUseRecommendation."
}

func firstSentencePatternUse(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "No confident pattern-use route was selected."
	}
	for _, separator := range []string{". ", "\n"} {
		if index := strings.Index(text, separator); index >= 0 {
			return strings.TrimSpace(text[:index+1])
		}
	}
	return text
}

func patternUseFullRecommendationCommand(concern string) string {
	concern = strings.TrimSpace(concern)
	if concern == "" {
		concern = "missing_operator_concern"
	}
	return fmt.Sprintf("haft_query(action=\"pattern_use\", mode=\"full\", query=%q)", concern)
}

func patternUseRecommendationText(record PatternUseRecommendation) string {
	parts := []string{
		record.RecordKind,
		record.Authority,
		record.ProjectConcernRef,
		record.RecommendedPatternUse.PatternRef,
		record.RecommendedPatternUse.Title,
		record.ReasonForRecommendation,
		record.RequiredOutputShape.CarrierKind,
	}
	for _, item := range record.WrongPatternBoundary {
		parts = append(parts, item.TemptingPatternOrMove, item.WhyWrongNow)
	}
	for _, item := range record.RequiredEvidenceOrSoTA {
		parts = append(parts, item.Requirement, item.FreshnessOrSourceRule)
	}
	for _, item := range record.BlockedStrongerUse {
		parts = append(parts, item.BlockedUse, item.UnblockCondition)
	}
	for _, item := range record.CloseoutOrVerificationExpectation {
		parts = append(parts, item.Expectation)
	}
	return strings.Join(parts, " ")
}

func normalizePatternUseQuery(query string) string {
	replacer := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		"-", " ",
		"_", " ",
		".", " ",
		",", " ",
		":", " ",
		";", " ",
		"?", " ",
		"!", " ",
		"\"", " ",
		"'", " ",
		"(", " ",
		")", " ",
		"/", " ",
	)
	normalized := replacer.Replace(strings.ToLower(query))
	fields := strings.Fields(normalized)
	return strings.Join(fields, " ")
}

func firstNonEmptyPatternUseString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func dedupePatternUseStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func clonePatternUseRouteCards(values []PatternUseRouteCard) []PatternUseRouteCard {
	out := make([]PatternUseRouteCard, 0, len(values))
	for _, value := range values {
		out = append(out, clonePatternUseRouteCard(value))
	}
	return out
}

func clonePatternUseRouteCard(value PatternUseRouteCard) PatternUseRouteCard {
	return PatternUseRouteCard{
		ID:                                value.ID,
		RecognitionCues:                   dedupePatternUseStrings(value.RecognitionCues),
		SemanticTrigger:                   strings.TrimSpace(value.SemanticTrigger),
		PositiveExamples:                  dedupePatternUseStrings(value.PositiveExamples),
		NegativeExamples:                  dedupePatternUseStrings(value.NegativeExamples),
		CandidatePatternUseSet:            cloneCandidatePatternUses(value.CandidatePatternUseSet),
		ApplicablePatternUseSet:           cloneApplicablePatternUses(value.ApplicablePatternUseSet),
		RecommendedPatternUse:             value.RecommendedPatternUse,
		ReasonForRecommendation:           value.ReasonForRecommendation,
		WrongPatternBoundary:              cloneWrongPatternBoundaries(value.WrongPatternBoundary),
		RequiredOutputShape:               cloneRequiredOutputShape(value.RequiredOutputShape),
		RequiredEvidenceOrSoTA:            cloneRequiredEvidenceOrSoTA(value.RequiredEvidenceOrSoTA),
		BlockedStrongerUse:                cloneBlockedStrongerUse(value.BlockedStrongerUse),
		CloseoutOrVerificationExpectation: cloneCloseoutOrVerificationExpectations(value.CloseoutOrVerificationExpectation),
		SupportLevel:                      value.SupportLevel,
		SuggestedHaftSurface:              value.SuggestedHaftSurface,
		SuggestedMethodRefs:               dedupePatternUseStrings(value.SuggestedMethodRefs),
		NextGoverningPatternRefs:          dedupePatternUseStrings(value.NextGoverningPatternRefs),
	}
}

func patternUseRouteCardByID(routes []PatternUseRouteCard, routeID string) (PatternUseRouteCard, bool) {
	for _, route := range routes {
		if route.ID != routeID {
			continue
		}
		return clonePatternUseRouteCard(route), true
	}
	return PatternUseRouteCard{}, false
}

func patternUseIndexHasRoute(index PatternUseIndex, routeID string) bool {
	for _, route := range index.RouteCards {
		if route.ID != routeID {
			continue
		}
		return true
	}
	return false
}

func cloneCandidatePatternUses(values []CandidatePatternUse) []CandidatePatternUse {
	out := make([]CandidatePatternUse, 0, len(values))
	for _, value := range values {
		value.SourceCard = clonePatternUseSourceCard(value.SourceCard)
		out = append(out, value)
	}
	return out
}

func compactCandidatePatternUses(values []CandidatePatternUse) []CandidatePatternUse {
	out := cloneCandidatePatternUses(values)
	for index := range out {
		out[index].SourceCard = nil
	}
	return out
}

func clonePatternUseSourceCard(value *PatternUseSourceCard) *PatternUseSourceCard {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneApplicablePatternUses(values []ApplicablePatternUse) []ApplicablePatternUse {
	return append([]ApplicablePatternUse(nil), values...)
}

func cloneWrongPatternBoundaries(values []WrongPatternBoundary) []WrongPatternBoundary {
	return append([]WrongPatternBoundary(nil), values...)
}

func cloneRequiredOutputShape(value RequiredOutputShape) RequiredOutputShape {
	return RequiredOutputShape{
		CarrierKind:      value.CarrierKind,
		RequiredSections: append([]string(nil), value.RequiredSections...),
	}
}

func cloneRequiredEvidenceOrSoTA(values []RequiredEvidenceOrSoTA) []RequiredEvidenceOrSoTA {
	return append([]RequiredEvidenceOrSoTA(nil), values...)
}

func cloneBlockedStrongerUse(values []BlockedStrongerUse) []BlockedStrongerUse {
	return append([]BlockedStrongerUse(nil), values...)
}

func cloneCloseoutOrVerificationExpectations(values []CloseoutOrVerificationExpectation) []CloseoutOrVerificationExpectation {
	return append([]CloseoutOrVerificationExpectation(nil), values...)
}

func candidate(patternRef string, title string, reason string) CandidatePatternUse {
	return CandidatePatternUse{
		PatternRef:          patternRef,
		Title:               title,
		ApplicabilityReason: reason,
	}
}

func applicable(patternRef string, title string) ApplicablePatternUse {
	return ApplicablePatternUse{
		PatternRef: patternRef,
		Title:      title,
	}
}

func recommended(patternRef string, title string) PatternUseRef {
	return PatternUseRef{
		PatternRef: patternRef,
		Title:      title,
	}
}

func namingPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "f18_naming_namecard",
		RecognitionCues: []string{
			"f 18",
			"f18",
			"namecard",
			"name card",
			"choose a name",
			"choose a better name",
			"call this",
			"better term",
			"project system process",
			"just give me the name",
		},
		SemanticTrigger: "Choose a good name for a project, system, process, subsystem, concept, artifact, or method using a nameCard with entity boundary and collision checks.",
		PositiveExamples: []string{
			"Choose a good name for this project/system/process.",
			"Choose a name for this project/system/process.",
			"Choose a better name for haft if any possible",
			"именуй нормально",
			"подбери хорошее имя для этого процесса",
			"给这个系统起个好名字",
			"We need a better term for this subsystem before the README rewrite.",
		},
		NegativeExamples: []string{
			"explain what nameCard means",
			"rename a local variable mechanically",
			"what is the term in this equation",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("F.18", "nameCard", "The task asks for a name/term rather than a generic brainstorm."),
			candidate("A.7", "Strict distinction", "The named entity must be separated from descriptions and carriers."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("F.18", "nameCard"),
			applicable("A.7", "Strict distinction"),
		},
		RecommendedPatternUse:   recommended("F.18", "nameCard"),
		ReasonForRecommendation: "Naming quality depends on the entity of concern, boundaries, near-collisions, and intended use, so a nameCard is the primary output.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "brainstorm names directly", WhyWrongNow: "A list of names skips EntityOfConcern, collision checks, and use-context constraints."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "nameCard",
			RequiredSections: []string{"EntityOfConcern", "candidate_name", "why_this_name", "collision_checks", "wrong_names_or_near_misses", "usage_sentence"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Check domain vocabulary and nearby existing names before preferring a candidate.", FreshnessOrSourceRule: "Use current repo/domain carriers or explicitly label the check absent."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No naming authority without EntityOfConcern and collision checks.", UnblockCondition: "Provide the entity boundary and run at least local collision/ambiguity checks."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close by showing the chosen name in one real usage sentence and naming the nearest rejected alternative."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"A.7", "A.6"},
	}
}

func architecturePatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "c30_architecture_structures",
		RecognitionCues: []string{
			"architecture",
			"architect",
			"system shape",
			"interfaces",
			"selected structures",
			"adr",
			"design this system",
			"start coding",
			"sketch the system",
		},
		SemanticTrigger: "Propose or evaluate software/system architecture by naming selected structures, boundaries, interfaces, rejected structures, and ADR candidates.",
		PositiveExamples: []string{
			"Propose an architecture for this system.",
			"Sketch the system shape and where the interfaces should be.",
			"распиши архитектуру и границы модулей",
			"Предложить архитектуру механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать; нужна структура и границы, без маркетинга.",
			"Предложи архитектуру для механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать. Не пиши маркетинг, дай структуру и границы.",
			"спроектируй структуру и границы механизма выбора паттерна рассуждения",
			"为这个系统设计架构和接口边界",
			"为推理模式选择器设计结构和边界",
		},
		NegativeExamples: []string{
			"fix this small typo without changing design",
			"what does ADR stand for",
			"show the current architecture file",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("C.30", "architecture selected structures", "The task asks for architecture, structure, or interfaces."),
			candidate("E.9", "DecisionRecord boundary", "Architecture choices often become decisions but are not binding by description."),
			candidate("A.6", "Boundary and relation precision", "Interface boundaries must be explicit."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("C.30", "architecture selected structures"),
			applicable("A.6", "Boundary and relation precision"),
		},
		RecommendedPatternUse:   recommended("C.30", "architecture selected structures"),
		ReasonForRecommendation: "Architecture work should name selected structures, boundaries, and ADR candidates instead of producing a free-form proposal.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "architecture prose as implementation authority", WhyWrongNow: "A structure description does not authorize code changes or settle ADRs without a decision path."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "architecture question card plus selected structures and ADR candidates",
			RequiredSections: []string{"architecture_question", "selected_structures", "boundary_interfaces", "rejected_structures", "ADR_candidates", "verification_plan"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "State the load-bearing architectural assumptions and evidence needed to test them.", FreshnessOrSourceRule: "Use current repo/API constraints or mark assumptions design-time only."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Architecture description does not authorize implementation work.", UnblockCondition: "Bind an explicit decision or commission before code changes that rely on the architecture."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with ADR candidates and the verification command or probe each selected structure needs."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"graph-preflight-before-governed-edit", "functional-core-imperative-shell", "domain-port-before-adapter"},
		NextGoverningPatternRefs: []string{"E.9", "A.6", "A.15"},
	}
}

func nextMovePatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e11_e10_next_move",
		RecognitionCues: []string{
			"what should i do next",
			"what should we do next",
			"what s the next move",
			"next move",
			"next useful move",
			"stuck",
			"pick the next",
			"what to do",
			"что делать",
		},
		SemanticTrigger: "The operator asks what to do next, which next move is admissible, or how to recover progress from a stuck state.",
		PositiveExamples: []string{
			"What should I do next?",
			"What's the next move?",
			"I am stuck; pick the next useful move.",
			"что делать дальше",
			"下一步该做什么",
		},
		NegativeExamples: []string{
			"execute the next item in the already approved checklist",
			"show me the next calendar event",
			"what is the next line in this file",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.11.PUR", "PatternUseRecommendation", "The operator asks which pattern-use move should be applied now."),
			candidate("E.10.MOVE", "next-move repair", "The request is for a next move rather than a full plan."),
			candidate("E.8", "pattern-shaped output", "The answer should be a concrete pattern-shaped action card."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.11.PUR", "PatternUseRecommendation"),
			applicable("E.10.MOVE", "next-move repair"),
		},
		RecommendedPatternUse:   recommended("E.11.PUR + E.10.MOVE", "PatternUseRecommendation with next-move repair"),
		ReasonForRecommendation: "A next-action request needs direct governed value recovery and one admissible move, not a new root workflow.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "mint a root Move artifact or generic task list", WhyWrongNow: "The move is only useful relative to the current concern, blocked stronger uses, and evidence boundary."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "PatternUseRecommendation or E.8 action card",
			RequiredSections: []string{"current_concern", "recommended_pattern_use", "one_next_move", "wrong_move_boundary", "blocked_stronger_use", "closeout_probe"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name the current concern and evidence gap that makes this the next move.", FreshnessOrSourceRule: "Use current session/repo state or mark the recommendation as low-support."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Recover direct governed value; do not mint a root Move kind.", UnblockCondition: "If a durable decision/work artifact is needed, route to E.9 or MethodPack explicitly."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close by stating the smallest observable result that tells us the move helped."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"E.8", "A.15"},
	}
}

func sotaPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e8_sota_evidence_pack",
		RecognitionCues: []string{
			"sota",
			"state of the art",
			"survey",
			"current practice",
			"current best practice",
			"find current practice",
			"research",
			"latest",
			"before we design",
		},
		SemanticTrigger: "Survey current state of the art, current practice, or evidence before making a design or engineering recommendation.",
		PositiveExamples: []string{
			"Survey SoTA for solving this problem.",
			"Find current practice before we design this.",
			"найди SoTA и свежие источники по этой задаче",
			"调研这个工程问题的最新实践",
		},
		NegativeExamples: []string{
			"explain an old local note without checking current sources",
			"run the existing tests",
			"format this file",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.8", "pattern-shaped output", "The task asks for research output shaped as a reusable pattern/evidence pack."),
			candidate("A.10/B.3", "evidence and freshness boundary", "SoTA claims need current sources and freshness labels."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.8", "pattern-shaped output"),
			applicable("A.10", "evidence boundary"),
			applicable("B.3", "evidence decay/freshness"),
		},
		RecommendedPatternUse:   recommended("E.8 + A.10/B.3", "SoTA evidence pack"),
		ReasonForRecommendation: "SoTA work is not generic advice; it needs current evidence, source quality, applicability boundaries, and near misses.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "answer from memory", WhyWrongNow: "SoTA is freshness-sensitive and stale recall is not current evidence."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "SoTA evidence pack",
			RequiredSections: []string{"question", "source_set", "current_consensus", "near_misses", "applicability_boundary", "freshness", "open_uncertainties"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Use current primary or authoritative sources where the claim can have changed.", FreshnessOrSourceRule: "Browse or cite current sources; record dates and source roles."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No design recommendation from stale SoTA recall alone.", UnblockCondition: "Collect current sources and map the applicability boundary first."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with what changed the design choice, what stayed uncertain, and when the evidence expires."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"A.10", "B.3", "E.9"},
	}
}

func diagnosisPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "diagnosis_rival_hypotheses",
		RecognitionCues: []string{
			"debug",
			"failure",
			"failing",
			"unclear failure",
			"flaky",
			"nobody knows why",
			"root cause",
			"doesn t work",
			"test is flaky",
		},
		SemanticTrigger: "Diagnose an unclear failure, flaky test, regression, incident, or unknown root cause through rival hypotheses and discriminating probes.",
		PositiveExamples: []string{
			"Debug this unclear failure.",
			"The test is flaky and nobody knows why.",
			"разберись почему это падает, причина непонятна",
			"诊断这个不清楚的失败原因",
		},
		NegativeExamples: []string{
			"apply this already known one-line fix",
			"show me the failure log only",
			"write a test for an already diagnosed bug",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("abductive diagnosis / rival hypotheses", "diagnosis with discriminating probes", "The task is a failure with unclear cause."),
			candidate("B.5.2", "parallel rival explanations", "Diagnosis should keep rival hypotheses visible until evidence separates them."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("abductive diagnosis / rival hypotheses", "diagnosis with discriminating probes"),
		},
		RecommendedPatternUse:   recommended("abductive diagnosis / rival hypotheses", "diagnosis ProblemCard plus discriminating probes"),
		ReasonForRecommendation: "Unclear failures should be diagnosed through rival hypotheses and discriminating probes before selecting a fix.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "first plausible fix", WhyWrongNow: "A plausible fix can hide competing causes and turn diagnosis into confirmation search."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "diagnosis ProblemCard plus discriminating probes",
			RequiredSections: []string{"observed_failure", "rival_hypotheses", "discriminating_probes", "evidence_weight", "next_probe", "fix_gate"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Collect failure reproduction evidence before claiming root cause.", FreshnessOrSourceRule: "Use current logs/tests/traces from the failing context."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No root-cause claim before a discriminating probe separates rivals.", UnblockCondition: "Run the probe or label the fix as speculative."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with the winning hypothesis, losing rivals, and the verification command that would fail without the fix."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"systematic-debugging-before-fix", "verification-before-completion"},
		NextGoverningPatternRefs: []string{"h-diagnose", "A.10"},
	}
}

func comparePatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "characterize_compare_parity",
		RecognitionCues: []string{
			"compare",
			"which is better",
			"two implementation options",
			"two designs",
			"tradeoff",
			"trade off",
		},
		SemanticTrigger: "Compare two or more serious variants under explicit dimensions, parity, evidence roles, and Pareto-front discipline.",
		PositiveExamples: []string{
			"Compare these two implementation options.",
			"Which of these two designs is better?",
			"сравни эти варианты по честным критериям",
			"比较这两个实现方案",
		},
		NegativeExamples: []string{
			"make the button label better",
			"choose the only available fix",
			"list files changed in this branch",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("characterize plus compare", "dimensions, parity, Pareto front", "The task asks for a comparison rather than a scalar opinion."),
			candidate("A.10/B.3", "evidence boundary", "Scores need explicit measurement/evidence roles."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("characterize", "comparison dimensions"),
			applicable("compare", "parity and Pareto front"),
		},
		RecommendedPatternUse:   recommended("characterize plus compare", "dimensions, scales, parity plan, Pareto front"),
		ReasonForRecommendation: "Comparison needs a declared characteristic space and parity before scoring, not a hidden scalar winner.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "pick a winner immediately", WhyWrongNow: "Without parity and dimensions, the winner encodes hidden values and incomparable tradeoffs."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "dimensions, scales, parity plan, Pareto front",
			RequiredSections: []string{"dimensions", "indicator_roles", "parity_plan", "scores", "dominated_variants", "pareto_front", "incomparables"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "State score evidence and missing-data policy per dimension.", FreshnessOrSourceRule: "Use same-context measurements where available; otherwise mark CL/freshness limits."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No scalar recommendation before parity and dimensions.", UnblockCondition: "Declare dimensions, pinned conditions, and missing-data policy."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with Pareto-front variants and the selection policy needed for any advisory recommendation."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"E.9", "A.10"},
	}
}

func evidenceProofPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "a10_b3_a7_evidence_proof",
		RecognitionCues: []string{
			"document prove",
			"doc proves",
			"document proves",
			"spec says",
			"can we rely",
			"prove the claim",
			"proof",
			"evidence",
			"carrier",
			"rely on it",
		},
		SemanticTrigger: "Review whether a claim is actually supported by evidence, proof, source freshness, congruence, and object-description-carrier boundaries.",
		PositiveExamples: []string{
			"Does this document prove the claim?",
			"The spec says it, so can we rely on it?",
			"это доказательство или просто описание в документе",
			"В документе написано, что механизм работает. Значит ли это, что мы доказали его работоспособность?",
			"документ утверждает, что механизм работает, но это доказательство или только описание?",
			"这个文档能证明这个结论吗",
			"文档说机制可用，这算证明还是只是描述?",
		},
		NegativeExamples: []string{
			"open the proof document",
			"copy the evidence section verbatim",
			"write documentation for a proven behavior",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("A.10", "evidence boundary", "The task asks whether a claim is supported."),
			candidate("B.3", "evidence freshness/decay", "Evidence needs freshness and congruence labels."),
			candidate("A.7", "strict distinction", "A document/carrier must not be confused with the object or proof relation."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("A.10", "evidence boundary"),
			applicable("B.3", "evidence freshness/decay"),
			applicable("A.7", "strict distinction"),
		},
		RecommendedPatternUse:   recommended("A.10 plus B.3 plus A.7", "evidence gap note / proof-boundary review"),
		ReasonForRecommendation: "The concern is whether a carrier supports a claim, so the proof relation and freshness/congruence must be explicit.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "treat carrier text as proof", WhyWrongNow: "A carrier can describe a claim without measuring, proving, or authorizing it."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "evidence gap note / proof-boundary review",
			RequiredSections: []string{"claim", "carrier", "object", "evidence_relation", "freshness", "congruence", "blocked_stronger_use"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Identify the evidence relation and congruence level for the claim.", FreshnessOrSourceRule: "Use current same-context evidence for strong reliance; otherwise mark weaker use only."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Carrier is not proof; require evidence relation.", UnblockCondition: "Attach same-context evidence or explicitly downgrade to documentary support."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with allowed use, blocked stronger use, and the missing evidence needed for stronger reliance."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"verification-before-completion"},
		NextGoverningPatternRefs: []string{"E.9", "A.15"},
	}
}

func strictDistinctionPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "a7_strict_distinction",
		RecognitionCues: []string{
			"object description carrier",
			"description carrier evidence",
			"strict distinction",
			"clarify what is the object",
			"dashboard the product state",
			"just a view",
			"product state",
		},
		SemanticTrigger: "Separate object, description, carrier, evidence, authority, and relation when the concern has category confusion.",
		PositiveExamples: []string{
			"Clarify what is the object, description, carrier, and evidence here.",
			"Is this dashboard the product state or just a view?",
			"раздели объект, описание, носитель и доказательство",
			"区分对象、描述、载体和证据",
		},
		NegativeExamples: []string{
			"change the UI view component",
			"show product state from the database",
			"update the carrier file path",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("A.7", "strict distinction", "The task explicitly asks to separate object, description, carrier, and evidence."),
			candidate("A.6", "boundary precision", "Relations between these things need typed boundaries."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("A.7", "strict distinction"),
			applicable("A.6", "boundary precision"),
		},
		RecommendedPatternUse:   recommended("A.7", "strict distinction table"),
		ReasonForRecommendation: "The concern is category confusion, so separating object, description, carrier, evidence, and authority is the primary move.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "answer with the most convenient label", WhyWrongNow: "A convenient label can collapse carrier/view/object/evidence and cause false authority."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "strict distinction table",
			RequiredSections: []string{"object", "description", "carrier", "evidence", "authority", "relation"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name which relation is observed and which relation is only inferred.", FreshnessOrSourceRule: "Use current carrier/object references where available."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No authority or state claim from a view/carrier alone.", UnblockCondition: "Identify the object state source and evidence relation."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with one sentence naming the safe use and one blocked stronger use."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"A.6", "A.10"},
	}
}

func publicAPIChangePatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "api_boundary_decision",
		RecognitionCues: []string{
			"public api",
			"api change",
			"endpoint",
			"stable now",
			"tell users",
			"breaking change",
			"interface stable",
			"api promise",
		},
		SemanticTrigger: "Plan or evaluate a public API, endpoint, compatibility boundary, breaking change, or external stability promise.",
		PositiveExamples: []string{
			"Plan a public API change.",
			"Can we tell users this endpoint is stable now?",
			"спланируй изменение публичного API и миграцию",
			"规划这个公共 API 的兼容性变更",
		},
		NegativeExamples: []string{
			"rename an internal helper function",
			"call the API once to inspect a response",
			"update a private test fixture",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("A.6", "boundary and relation precision", "Public API work changes a boundary relation."),
			candidate("E.9", "DecisionRecord boundary", "Public promises require an explicit decision gate."),
			candidate("A.15", "role/method/work distinction", "A plan or review is not delivery or authorization."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("A.6", "boundary and relation precision"),
			applicable("E.9", "DecisionRecord boundary"),
			applicable("A.15", "role/method/work distinction"),
		},
		RecommendedPatternUse:   recommended("A.6 plus E.9 plus A.15", "API boundary note plus decision candidate plus verification plan"),
		ReasonForRecommendation: "A public API change affects external boundary promises, so boundary typing and an explicit human decision gate are required.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "treat plan/review as public stability", WhyWrongNow: "A plan or positive review is not a shipped compatibility promise."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "API boundary note plus decision candidate plus verification plan",
			RequiredSections: []string{"current_contract", "proposed_contract", "compatibility_boundary", "migration_path", "decision_candidate", "verification_plan", "external_promise_gate"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Verify compatibility, migration, and affected callers before external stability claims.", FreshnessOrSourceRule: "Use current API tests/callers/docs from this repo and label gaps."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No external stability promise without explicit human gate and compatibility evidence.", UnblockCondition: "Bind a decision and verify the public contract against current callers/tests."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with the compatibility verdict, migration notes, and whether a DecisionRecord is required."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"behavior-first-testing", "graph-preflight-before-governed-edit"},
		NextGoverningPatternRefs: []string{"E.9", "A.10"},
	}
}

func commitmentPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e9_commitment_human_gate",
		RecognitionCues: []string{
			"commit to this product direction",
			"commit this direction",
			"commitment",
			"approve the direction",
			"approval",
			"decide now",
			"product direction",
			"binding choice",
			"review is positive",
		},
		SemanticTrigger: "Prepare a human-gated binding decision or commitment candidate for product direction, external promise, approval, or resource choice.",
		PositiveExamples: []string{
			"Should we commit to this product direction?",
			"This review is positive; should we approve the direction?",
			"можем ли мы уже принять это направление как решение",
			"我们现在可以承诺这个产品方向吗",
		},
		NegativeExamples: []string{
			"commit the current git changes",
			"show the latest review comment",
			"approve a CI job retry",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.9", "DecisionRecord boundary", "The task asks for a binding direction/commitment."),
			candidate("compare/evidence gate", "comparison and evidence before choice", "A positive review does not replace selection policy and evidence."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.9", "DecisionRecord boundary"),
			applicable("A.10", "evidence boundary"),
		},
		RecommendedPatternUse:   recommended("E.9 plus compare/evidence gate", "DecisionRecord candidate, not binding decision"),
		ReasonForRecommendation: "A product direction commitment is a human value/resource decision; the router can prepare a candidate, not bind it.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "treat positive review as approval", WhyWrongNow: "Review evidence can inform a decision but cannot transform itself into operator commitment."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "DecisionRecord candidate, not binding decision",
			RequiredSections: []string{"problem_frame", "option_set", "selection_policy", "evidence_summary", "risks", "rollback", "human_gate"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Summarize decision evidence and missing counterevidence before any binding choice.", FreshnessOrSourceRule: "Use current review/product evidence and preserve uncertainty."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Binding decision requires explicit human DecisionRecord action.", UnblockCondition: "Operator invokes the manual decision path with accepted selection policy and rollback."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with a decision candidate and a clear human gate, not a claim that the direction is approved."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"A.15", "A.10"},
	}
}

func workPlanPerformedWorkPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "a15_work_plan_performed_work_boundary",
		RecognitionCues: []string{
			"plan actual work",
			"performed work",
			"description work",
			"only described",
			"just a plan",
		},
		SemanticTrigger: "Separate described plan, method description, carrier text, evidence, and actually performed work before claiming progress or done work.",
		PositiveExamples: []string{
			"Is this plan actual work or just a plan?",
			"Did you do the work or only describe the plan?",
			"Ты сделал работу или только описал план?",
			"это выполненная работа или только описание намерений?",
			"这是已经完成的工作，还是只是计划说明？",
		},
		NegativeExamples: []string{
			"run the exact test command now",
			"open the MethodRun file",
			"show me the current plan document",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("A.15", "role/method/work distinction", "The concern asks whether a role, method, plan, carrier, or performed work is being confused."),
			candidate("E.18", "principle-to-work carry-through", "The concern needs carry-through from principle or plan into actual work and evidence."),
			candidate("A.7", "strict distinction", "Plan text, carrier, work, and evidence must be separated."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("A.15", "role/method/work distinction"),
			applicable("E.18", "principle-to-work carry-through"),
			applicable("A.7", "strict distinction"),
		},
		RecommendedPatternUse:   recommended("A.15 plus E.18/E.18.1 plus A.7", "work/plan/evidence distinction"),
		ReasonForRecommendation: "The task asks whether description, plan, method, and performed work are being collapsed, so the output must separate them before any progress claim.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "treat a plan as performed work", WhyWrongNow: "A plan can guide work but does not change the target system or create evidence by itself."},
			{TemptingPatternOrMove: "treat a test list as verification", WhyWrongNow: "A test list is a carrier for intended checks, not evidence that the checks ran."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "work_plan_evidence_distinction_card",
			RequiredSections: []string{"claimed_progress", "plan_or_description", "performed_work", "carrier", "evidence", "blocked_stronger_use", "next_verification"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name the observable work evidence separately from the plan or description carrier.", FreshnessOrSourceRule: "Use current files, command output, runtime state, or MethodRun closeout; otherwise label the claim design-time only."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Description is not work; plan is not evidence.", UnblockCondition: "Show the actual changed object or current same-context evidence before claiming delivery."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with one sentence for what was only planned, one for what actually changed, and the evidence ref."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"verification-before-completion", "problem-closure-hygiene"},
		NextGoverningPatternRefs: []string{"A.15", "E.18", "E.18.1", "A.7", "A.10"},
	}
}

func agentActionAdmissibilityPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e16_agent_action_admissibility",
		RecognitionCues: []string{
			"agent tool-call",
			"tool-call sequence",
			"risky change",
			"may the agent",
			"allowed actions",
		},
		SemanticTrigger: "Plan admissible AI-agent actions for a risky change by separating available tools, allowed actions, required gates, evidence, and forbidden authority moves.",
		PositiveExamples: []string{
			"Plan the AI agent tool-call sequence for this risky change.",
			"What tools may the agent call before editing?",
			"Can the agent do this without asking?",
			"Можно ли агенту самому вызвать эти инструменты и править файлы?",
			"为这个有风险的修改规划代理可以调用哪些工具",
		},
		NegativeExamples: []string{
			"what tools are installed?",
			"run a harmless lookup",
			"list available MCP tools",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.16", "agent autonomy/admissibility", "The task asks what the agent may do and what needs a gate."),
			candidate("A.15", "role/method/work distinction", "A tool plan, role, and performed work must stay distinct."),
			candidate("A.10", "evidence boundary", "Authority and evidence must be separated from tool availability."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.16", "agent autonomy/admissibility"),
			applicable("A.15", "role/method/work distinction"),
			applicable("A.10", "evidence boundary"),
		},
		RecommendedPatternUse:   recommended("E.16 plus A.15 plus A.10", "admissible agent action plan"),
		ReasonForRecommendation: "The concern is not which tool exists, but which action is admissible under authority, evidence, and work-method boundaries.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "tool availability as permission", WhyWrongNow: "A callable tool only proves capability, not authorization, budget, or safety."},
			{TemptingPatternOrMove: "PatternUse as MethodPack gate", WhyWrongNow: "PatternUse can suggest a method handoff but cannot open or close MethodRuns."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "admissible_agent_action_plan",
			RequiredSections: []string{"operator_goal", "allowed_read_actions", "edit_preconditions", "forbidden_authority_moves", "methodpack_handoff_condition", "evidence_needed", "stop_condition"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name the authority source for any mutating action and the evidence needed before closeout.", FreshnessOrSourceRule: "Use current operator instruction, MethodRun, WorkCommission, or explicit gate; otherwise block stronger use."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Tool plan is not permission to mutate.", UnblockCondition: "Open the relevant MethodRun or get explicit operator authority before non-reversible mutation."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with allowed actions, blocked actions, and the MethodPack/verification handoff if implementation starts."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		SuggestedMethodRefs:      []string{"graph-preflight-before-governed-edit", "verification-before-completion"},
		NextGoverningPatternRefs: []string{"E.16", "A.15", "A.10", "E.9"},
	}
}

func specLifecycleAuthorityPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "spec_lifecycle_authority",
		RecognitionCues: []string{
			"write this into specs",
			"approve spec",
			"rebaseline spec",
			"specsection",
			"spec lifecycle",
		},
		SemanticTrigger: "Route spec draft, carrier edit, approval, rebaseline, reopen, and binding-preflight concerns without confusing carrier edits with authority.",
		PositiveExamples: []string{
			"Write this into specs.",
			"Approve or rebaseline this SpecSection.",
			"Should we approve this spec section or keep it draft?",
			"Нужно ребейзлайнить эту секцию спеки?",
			"把这个写进规格，但不要把草稿当成批准",
		},
		NegativeExamples: []string{
			"explain what a spec is",
			"edit unrelated docs",
			"does this document prove the claim?",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("SpecSection lifecycle", "spec draft/approve/rebaseline/reopen lifecycle", "The concern asks for spec lifecycle routing or authority."),
			candidate("A.7", "strict distinction", "Spec carrier, section state, approval, and evidence must stay distinct."),
			candidate("E.9", "DecisionRecord boundary", "Approval and binding changes may require explicit human gate."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("SpecSection lifecycle", "spec draft/approve/rebaseline/reopen lifecycle"),
			applicable("A.7", "strict distinction"),
			applicable("E.9", "DecisionRecord boundary"),
			applicable("A.15", "role/method/work distinction"),
		},
		RecommendedPatternUse:   recommended("SpecSection lifecycle plus A.7 plus E.9 plus A.15", "spec lifecycle routing card"),
		ReasonForRecommendation: "The task concerns spec lifecycle authority; a carrier edit, draft, approval, rebaseline, and evidence relation must be routed separately.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "carrier edit as approval", WhyWrongNow: "Writing text into a spec carrier does not approve or rebaseline the SpecSection."},
			{TemptingPatternOrMove: "rebaseline without human gate", WhyWrongNow: "Rebaseline changes authority/currentness and must use the lifecycle gate."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "spec_lifecycle_routing_card",
			RequiredSections: []string{"requested_change", "carrier_edit", "approval_gate", "rebaseline_gate", "evidence_relation", "blocked_stronger_use", "next_h_spec_action"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name current SpecSection state and required lifecycle action before changing authority.", FreshnessOrSourceRule: "Use current spec lifecycle data or mark the action as draft-only."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Spec carrier edit is not approval or rebaseline.", UnblockCondition: "Use h-spec lifecycle action with explicit operator gate for approval/rebaseline."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with the h-spec action to call next and the authority move that remains blocked."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"SpecSection", "A.7", "E.9", "A.15"},
	}
}

func layerBoundaryPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e4_layer_boundary",
		RecognitionCues: []string{
			"patternuse methodpack",
			"methodpack dpf",
			"fpf dpf lpf",
			"route cards for all",
			"all fpf cards",
		},
		SemanticTrigger: "Separate FPF source, DPF source pack, LPF local practice, TPF/workflow mechanics, PatternAtlas, PatternUse, and MethodPack when product architecture terms are being collapsed.",
		PositiveExamples: []string{
			"Should PatternUse become MethodPack?",
			"Should MethodPack become SWE-DPF?",
			"Do we need all FPF cards as route cards?",
			"[$h-reason] Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack. Не делай коммитов, нужен reasoning carrier.",
			"Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack.",
			"Separate FPF source cards, DPF source packs, PatternUseGateway, and MethodPack before changing the router.",
			"Где граница FPF, DPF, LPF, TPF и MethodPack?",
			"Надо ли компилировать все 250 FPF карточек в route cards?",
			"需要把所有 FPF 模式卡都编译成路由卡吗",
		},
		NegativeExamples: []string{
			"use the existing MethodPack gate",
			"read this local AGENTS rule",
			"how many route cards are currently in the index?",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.4", "framework layer boundary", "The concern asks how FPF, DPF, local practice, and work mechanics relate."),
			candidate("A.15", "role/method/work distinction", "Framework source, method, role, and work harness must not collapse."),
			candidate("E.11", "PatternUse boundary", "PatternUse selects pattern-use shape without becoming the source catalog or work harness."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.4", "framework layer boundary"),
			applicable("E.4.FPF", "FPF source framework"),
			applicable("E.4.DPF", "domain pattern framework"),
			applicable("A.15", "role/method/work distinction"),
			applicable("E.11", "PatternUse boundary"),
		},
		RecommendedPatternUse:   recommended("E.4 plus A.15 plus E.11", "framework layer boundary card"),
		ReasonForRecommendation: "The task is a layer-boundary question; the answer must separate source frameworks, local practice, route selection, and work/evidence harnesses.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "compiled route catalog as ontology authority", WhyWrongNow: "Route cards support recurring task behavior; they do not define the FPF/DPF source ontology."},
			{TemptingPatternOrMove: "MethodPack as FPF or DPF source", WhyWrongNow: "MethodPack is a work/evidence harness and may cite source refs, but it is not the source framework."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "framework_layer_boundary_card",
			RequiredSections: []string{"concern", "fpf_source", "dpf_source_pack", "lpf_context", "patternatlas_source_index", "patternuse_gateway", "methodpack_harness", "blocked_collapse", "next_slice"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name which artifact is source, route selector, local carrier, or work harness before recommending product changes.", FreshnessOrSourceRule: "Use current repo surfaces and C0 term sheet; mark unresolved product choices as design-time."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "Do not turn FPF catalog coverage into compiled route support or MethodPack authority.", UnblockCondition: "Promote behavior families only with route/audit evidence and keep source packs separate."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close with a layer table and the one runtime slice, if any, that should change next."},
		},
		SupportLevel:             PatternUseSupportImplementedSubstrate,
		NextGoverningPatternRefs: []string{"E.4", "E.4.FPF", "E.4.DPF", "A.15", "E.11"},
	}
}

func fallbackPatternUseRouteCard() PatternUseRouteCard {
	return PatternUseRouteCard{
		ID: "e11_patternuse_fallback",
		RecognitionCues: []string{
			"pattern",
			"fpf",
			"apply",
		},
		CandidatePatternUseSet: []CandidatePatternUse{
			candidate("E.11.PUR", "PatternUseRecommendation", "No sharper route dominates; recover the current concern first."),
			candidate("A.7", "strict distinction", "Ambiguous concerns often hide object/description/carrier confusion."),
		},
		ApplicablePatternUseSet: []ApplicablePatternUse{
			applicable("E.11.PUR", "PatternUseRecommendation"),
		},
		RecommendedPatternUse:   recommended("E.11.PUR", "PatternUseRecommendation"),
		ReasonForRecommendation: "The query lacks enough route signal for a stronger card, so start by making the current concern and blocked stronger uses explicit.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{TemptingPatternOrMove: "pretend a confident specialized pattern was selected", WhyWrongNow: "The signal is under-specified and needs concern recovery first."},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind:      "PatternUseRecommendation",
			RequiredSections: []string{"current_concern", "candidate_pattern_use_set", "recommended_pattern_use", "wrong_pattern_boundary", "blocked_stronger_use", "closeout_probe"},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{Requirement: "Name the missing context before stronger pattern use.", FreshnessOrSourceRule: "Ask for or inspect current context rather than filling gaps from generic recall."},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{BlockedUse: "No specialized pattern-use claim without enough concern signal.", UnblockCondition: "Provide entity, boundary, decision/evidence/work intent, or failure signal."},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close by asking for the smallest missing context or routing to a sharper card if it becomes clear."},
		},
		SupportLevel:             PatternUseSupportMissing,
		NextGoverningPatternRefs: []string{"A.7", "A.6", "A.15"},
	}
}
