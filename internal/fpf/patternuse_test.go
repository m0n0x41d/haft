package fpf

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecommendPatternUseWithoutSemanticEvidenceDoesNotSelectCompiledRoutes(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{name: "name", query: "Choose a name for this project/system/process."},
		{name: "architecture", query: "Propose an architecture for this system."},
		{name: "next action", query: "What should I do next?"},
		{name: "sota", query: "Survey SoTA for solving this problem."},
		{name: "debug", query: "Debug this unclear failure."},
		{name: "compare", query: "Compare these two implementation options."},
		{name: "document proof", query: "Does this document prove the claim?"},
		{name: "strict distinction", query: "Clarify what is the object, description, carrier, and evidence here."},
		{name: "public api", query: "Plan a public API change."},
		{name: "product commitment", query: "Should we commit to this product direction?"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := RecommendPatternUse(tc.query)

			if err := record.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v\n%#v", err, record)
			}
			if record.Authority != PatternUseAuthority {
				t.Fatalf("authority = %q", record.Authority)
			}
			if record.SupportLevel != PatternUseSupportMissing {
				t.Fatalf("support = %q, want missing", record.SupportLevel)
			}
			if record.MatchedRouteID != "e11_patternuse_fallback" {
				t.Fatalf("matched route = %q, want fallback", record.MatchedRouteID)
			}
			if record.RouteMatchStrategy != PatternUseRouteMatchStrategyNone {
				t.Fatalf("route strategy = %q", record.RouteMatchStrategy)
			}
			if record.HasAuthorityViolation() {
				t.Fatalf("recommendation has authority violation:\n%#v", record)
			}
		})
	}
}

func TestRecommendPatternUseWithSemanticRouteMatchCanonicalRoutes(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		routeID   string
		wantRef   string
		wantShape string
	}{
		{name: "name", query: "Choose a name for this project/system/process.", routeID: "f18_naming_namecard", wantRef: "F.18", wantShape: "nameCard"},
		{name: "architecture", query: "Propose an architecture for this system.", routeID: "c30_architecture_structures", wantRef: "C.30", wantShape: "architecture question card"},
		{name: "next action", query: "What should I do next?", routeID: "e11_e10_next_move", wantRef: "E.11.PUR + E.10.MOVE", wantShape: "PatternUseRecommendation or E.8 action card"},
		{name: "sota", query: "Survey SoTA for solving this problem.", routeID: "e8_sota_evidence_pack", wantRef: "E.8 + A.10/B.3", wantShape: "SoTA evidence pack"},
		{name: "debug", query: "Debug this unclear failure.", routeID: "diagnosis_rival_hypotheses", wantRef: "abductive diagnosis / rival hypotheses", wantShape: "diagnosis ProblemCard"},
		{name: "compare", query: "Compare these two implementation options.", routeID: "characterize_compare_parity", wantRef: "characterize plus compare", wantShape: "dimensions, scales, parity plan, Pareto front"},
		{name: "proof", query: "Does this document prove the claim?", routeID: "a10_b3_a7_evidence_proof", wantRef: "A.10 plus B.3 plus A.7", wantShape: "evidence gap note"},
		{name: "strict distinction", query: "Clarify what is the object, description, carrier, and evidence here.", routeID: "a7_strict_distinction", wantRef: "A.7", wantShape: "strict distinction table"},
		{name: "public api", query: "Plan a public API change.", routeID: "api_boundary_decision", wantRef: "A.6 plus E.9 plus A.15", wantShape: "API boundary note"},
		{name: "commitment", query: "Should we commit to this product direction?", routeID: "e9_commitment_human_gate", wantRef: "E.9 plus compare/evidence gate", wantShape: "DecisionRecord candidate"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := RecommendPatternUseWithSemanticRouteAndIntentMatch(
				PatternUseRequest{Query: tc.query},
				PatternUseRouteMatch{
					RouteID:  tc.routeID,
					Strategy: PatternUseRouteMatchStrategySemanticCompiledRoute,
					Score:    0.71,
					Margin:   0.11,
					Contract: "local/embeddinggemma-300m/256",
				},
				PatternUseIntentLaneMatch{
					Lane:     PatternUseIntentApplyPattern,
					Strategy: PatternUseIntentMatchStrategySemanticLane,
					Score:    0.70,
					Margin:   0.10,
					Contract: "local/embeddinggemma-300m/256",
				},
			)

			if err := record.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if record.RecommendedPatternUse.PatternRef != tc.wantRef {
				t.Fatalf("pattern = %q, want %q", record.RecommendedPatternUse.PatternRef, tc.wantRef)
			}
			if record.MatchedRouteID != tc.routeID {
				t.Fatalf("route = %q, want %q", record.MatchedRouteID, tc.routeID)
			}
			if !strings.Contains(record.RequiredOutputShape.CarrierKind, tc.wantShape) {
				t.Fatalf("shape = %q, want fragment %q", record.RequiredOutputShape.CarrierKind, tc.wantShape)
			}
			if record.IntentLane != PatternUseIntentApplyPattern {
				t.Fatalf("intent lane = %q", record.IntentLane)
			}
		})
	}
}

func TestRecommendPatternUseCompactGateway(t *testing.T) {
	record := RecommendPatternUseCompact("Choose a better name for haft if any possible")

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v\n%#v", err, record)
	}
	if record.RecordKind != PatternUseGatewayRecordKind {
		t.Fatalf("record kind = %q", record.RecordKind)
	}
	if record.ShouldUsePattern != PatternUseShouldUseAbstain {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
	if record.RecommendedPatternUse.PatternRef != "E.11.PUR" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.SuggestedHaftSurface != "none" {
		t.Fatalf("surface = %q", record.SuggestedHaftSurface)
	}
	if !strings.Contains(record.FullRecommendationCommand, `mode="full"`) {
		t.Fatalf("full command = %q", record.FullRecommendationCommand)
	}
}

func TestRecommendPatternUseCompactGatewayAbstainsOnMissingSignal(t *testing.T) {
	record := RecommendPatternUseCompact("")

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.ShouldUsePattern != PatternUseShouldUseAbstain {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
	if record.SupportLevel != PatternUseSupportMissing {
		t.Fatalf("support = %q", record.SupportLevel)
	}
}

func TestDefaultPatternUseIndexIsSeedCoverageNotFullCatalog(t *testing.T) {
	index := DefaultPatternUseIndex()

	if err := index.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v\n%#v", err, index)
	}
	if index.FullFPFCatalogCovered {
		t.Fatal("seed index must not claim full FPF catalog coverage")
	}
	if index.Coverage != PatternUseSeedIndexCoverage {
		t.Fatalf("coverage = %q", index.Coverage)
	}
	if len(index.RouteCards) < 8 {
		t.Fatalf("route card count = %d, want seed corpus", len(index.RouteCards))
	}
	if !patternUseIndexHasRoute(index, "f18_naming_namecard") {
		t.Fatal("seed index missing naming route")
	}
	if index.SemanticRouteDocumentCount == 0 {
		t.Fatal("semantic route documents must be exposed in index summary")
	}
	if index.EmbeddingContract != nil {
		t.Fatalf("default index embedding_contract = %#v, want nil without baked embedding count", index.EmbeddingContract)
	}
}

func TestPatternUseIndexRequiresEmbeddingContractWithRouteEmbeddings(t *testing.T) {
	index := PatternUseIndexWithRetrievalAndSemanticContract(
		66,
		81,
		PatternUseEmbeddingContractFor("local", "embeddinggemma-300m-q", 256),
	)

	if err := index.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if index.EmbeddingContract == nil {
		t.Fatal("embedding_contract must be present when semantic route embeddings are present")
	}
	if index.EmbeddingContract.Provider != "local" {
		t.Fatalf("provider = %q", index.EmbeddingContract.Provider)
	}
	if index.EmbeddingContract.Model != "embeddinggemma-300m-q" {
		t.Fatalf("model = %q", index.EmbeddingContract.Model)
	}
	if index.EmbeddingContract.Dim != 256 {
		t.Fatalf("dim = %d", index.EmbeddingContract.Dim)
	}
	if index.SemanticRouteEmbeddingModel != "local/embeddinggemma-300m-q/256" {
		t.Fatalf("semantic_route_embedding_model = %q", index.SemanticRouteEmbeddingModel)
	}

	index.EmbeddingContract = nil
	if err := index.Validate(); err == nil {
		t.Fatal("Validate succeeded without embedding_contract")
	}
}

func TestPatternUseRouteEmbeddingDocumentsAreDeterministic(t *testing.T) {
	first := PatternUseRouteEmbeddingDocuments(DefaultPatternUseRouteCards())
	second := PatternUseRouteEmbeddingDocuments(DefaultPatternUseRouteCards())

	if len(first) == 0 {
		t.Fatal("expected route embedding documents")
	}
	if len(first) != len(second) {
		t.Fatalf("document counts differ: %d vs %d", len(first), len(second))
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("document %d differs:\n%#v\n%#v", index, first[index], second[index])
		}
	}
}

func TestPatternUseRouteEmbeddingDocumentsIncludePlaygroundRegressionPrompts(t *testing.T) {
	documents := PatternUseRouteEmbeddingDocuments(DefaultPatternUseRouteCards())
	cases := []struct {
		routeID string
		prompt  string
	}{
		{
			routeID: "c30_architecture_structures",
			prompt:  "Предложить архитектуру механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать; нужна структура и границы, без маркетинга.",
		},
		{
			routeID: "c30_architecture_structures",
			prompt:  "Предложи архитектуру для механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать. Не пиши маркетинг, дай структуру и границы.",
		},
		{
			routeID: "a10_b3_a7_evidence_proof",
			prompt:  "В документе написано, что механизм работает. Значит ли это, что мы доказали его работоспособность?",
		},
	}

	for _, tc := range cases {
		t.Run(tc.routeID, func(t *testing.T) {
			if !patternUseRouteDocumentsContainPrompt(documents, tc.routeID, tc.prompt) {
				t.Fatalf("route document corpus missing prompt for %s:\n%s", tc.routeID, tc.prompt)
			}
		})
	}
}

func TestPatternUseIntentEmbeddingDocumentsIncludeAuditApplyPrompts(t *testing.T) {
	documents := PatternUseIntentEmbeddingDocuments(DefaultPatternUseIntentLaneCards())
	prompts := []string{
		"What should I do next?",
		"Survey SoTA for solving this problem.",
		"Compare these two implementation options.",
		"Clarify what is the object, description, carrier, and evidence here.",
		"The spec says it, so can we rely on it?",
		"This review is positive; should we approve the direction?",
	}

	for _, prompt := range prompts {
		t.Run(prompt, func(t *testing.T) {
			if !patternUseIntentDocumentsContainPrompt(documents, PatternUseIntentApplyPattern, prompt) {
				t.Fatalf("intent document corpus missing apply prompt:\n%s", prompt)
			}
		})
	}
}

func TestPatternUseRouteEmbeddingDocumentHashChangesWithExamples(t *testing.T) {
	routes := DefaultPatternUseRouteCards()
	before := PatternUseRouteEmbeddingDocuments(routes)

	routes[0].PositiveExamples = append(routes[0].PositiveExamples, "new naming example")
	after := PatternUseRouteEmbeddingDocuments(routes)

	if len(before) == len(after) {
		t.Fatalf("expected appended example to add a route document, still have %d", len(after))
	}
	if before[0].ContentHash != after[0].ContentHash {
		t.Fatalf("synopsis hash changed unexpectedly: %s vs %s", before[0].ContentHash, after[0].ContentHash)
	}
}

func patternUseRouteDocumentsContainPrompt(
	documents []PatternUseRouteEmbeddingDocument,
	routeID string,
	prompt string,
) bool {
	for _, document := range documents {
		if document.RouteID != routeID {
			continue
		}
		if document.DocumentKind != PatternUseRouteDocumentKindPositiveExample {
			continue
		}
		if document.Text == prompt {
			return true
		}
	}
	return false
}

func patternUseIntentDocumentsContainPrompt(
	documents []PatternUseIntentEmbeddingDocument,
	laneID PatternUseIntentLane,
	prompt string,
) bool {
	for _, document := range documents {
		if document.LaneID != laneID {
			continue
		}
		if document.DocumentKind != PatternUseRouteDocumentKindPositiveExample {
			continue
		}
		if document.Text == prompt {
			return true
		}
	}
	return false
}

func TestRecommendPatternUseCarriesAdvisoryMethodPackBridge(t *testing.T) {
	record := RecommendPatternUseWithSemanticRouteAndIntentMatch(
		PatternUseRequest{Query: "Debug this unclear failure."},
		PatternUseRouteMatch{
			RouteID:  "diagnosis_rival_hypotheses",
			Strategy: PatternUseRouteMatchStrategySemanticCompiledRoute,
			Score:    0.71,
			Margin:   0.11,
			Contract: "local/embeddinggemma-300m/256",
		},
		PatternUseIntentLaneMatch{
			Lane:     PatternUseIntentApplyPattern,
			Strategy: PatternUseIntentMatchStrategySemanticLane,
			Score:    0.70,
			Margin:   0.10,
			Contract: "local/embeddinggemma-300m/256",
		},
	)

	if !patternUseStringsContain(record.SuggestedMethodRefs, "systematic-debugging-before-fix") {
		t.Fatalf("method refs = %#v", record.SuggestedMethodRefs)
	}
	if !patternUseStringsContain(record.SuggestedMethodRefs, "verification-before-completion") {
		t.Fatalf("method refs = %#v", record.SuggestedMethodRefs)
	}
	if !patternUseStringsContain(record.AuthorityBoundary, "not_methodrun_gate_result") {
		t.Fatalf("authority boundary missing MethodRun gate guard: %#v", record.AuthorityBoundary)
	}
	if !patternUseStringsContain(record.AuthorityBoundary, "not_gate_passage") {
		t.Fatalf("authority boundary missing gate passage guard: %#v", record.AuthorityBoundary)
	}
	if record.HasAuthorityViolation() {
		t.Fatalf("method bridge must stay advisory: %#v", record)
	}

	compact := CompactPatternUseRecommendation(record)
	if !patternUseStringsContain(compact.SuggestedMethodRefs, "systematic-debugging-before-fix") {
		t.Fatalf("compact method refs = %#v", compact.SuggestedMethodRefs)
	}
	if !patternUseStringsContain(compact.AuthorityBoundary, "not_methodrun_gate_result") {
		t.Fatalf("compact authority boundary missing MethodRun gate guard: %#v", compact.AuthorityBoundary)
	}
	if !patternUseStringsContain(compact.AuthorityBoundary, "not_gate_passage") {
		t.Fatalf("compact authority boundary missing gate passage guard: %#v", compact.AuthorityBoundary)
	}
}

func TestRecommendPatternUseWithRetrievedCandidatesDoesNotPromoteSeedWithoutSemanticRoute(t *testing.T) {
	record := RecommendPatternUseWithRetrievedCandidates(
		PatternUseRequest{Query: "Choose a name for this project/system/process."},
		[]PatternUseRetrievedCandidate{
			{PatternRef: "A.6.B", Title: "Boundary Norm Square"},
		},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.RecommendedPatternUse.PatternRef != "A.6.B" {
		t.Fatalf("pattern = %q, want retrieved candidate", record.RecommendedPatternUse.PatternRef)
	}
	if record.SupportLevel != PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.MatchedRouteID != PatternUseRetrievedFallbackRouteID {
		t.Fatalf("route = %q", record.MatchedRouteID)
	}
}

func TestRecommendPatternUseWithRetrievedCandidatesReturnsUncompiledFallback(t *testing.T) {
	record := RecommendPatternUseWithRetrievedCandidates(
		PatternUseRequest{Query: "Use boundary norm square admissibility for this concern."},
		[]PatternUseRetrievedCandidate{
			{
				PatternRef:   "A.6.B",
				Title:        "Boundary Norm Square",
				Summary:      "Boundary norm square separates source, claim, use, and authority.",
				Snippet:      "Use the square to keep boundary deontics admissible.",
				SourceTier:   SpecSearchTierFTS,
				SourceReason: "FTS match on boundary norm square",
				SourceRef:    "embedded-fpf-test",
			},
		},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v\n%#v", err, record)
	}
	if record.SupportLevel != PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.MatchedRouteID != PatternUseRetrievedFallbackRouteID {
		t.Fatalf("route = %q", record.MatchedRouteID)
	}
	if record.RecommendedPatternUse.PatternRef != "A.6.B" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.RequiredOutputShape.CarrierKind != "retrieved_pattern_applicability_card" {
		t.Fatalf("shape = %q", record.RequiredOutputShape.CarrierKind)
	}
	if !strings.Contains(patternUseBlockedText(record), "Top-k FPF retrieval is not a compiled PatternUse route card") {
		t.Fatalf("blocked stronger use missing retrieval boundary:\n%s", patternUseBlockedText(record))
	}
	if len(record.CandidatePatternUseSet) != 1 {
		t.Fatalf("candidates = %#v", record.CandidatePatternUseSet)
	}
	if record.CandidatePatternUseSet[0].SourceTier != SpecSearchTierFTS {
		t.Fatalf("candidate source tier = %#v", record.CandidatePatternUseSet[0])
	}
	if !patternUseStringsContain(record.UseContext.SourceRefs, "embedded-fpf-test") {
		t.Fatalf("source refs = %#v", record.UseContext.SourceRefs)
	}
	if !patternUseStringsContain(record.AuthorityBoundary, "not_methodrun_gate_result") {
		t.Fatalf("retrieval authority boundary missing MethodRun gate guard: %#v", record.AuthorityBoundary)
	}
	if !patternUseStringsContain(record.AuthorityBoundary, "not_gate_passage") {
		t.Fatalf("retrieval authority boundary missing gate passage guard: %#v", record.AuthorityBoundary)
	}

	compact := RecommendPatternUseCompactWithRetrievedCandidates(
		PatternUseRequest{Query: "Use boundary norm square admissibility for this concern."},
		[]PatternUseRetrievedCandidate{{PatternRef: "A.6.B", Title: "Boundary Norm Square"}},
	)
	if err := compact.Validate(); err != nil {
		t.Fatalf("compact Validate returned error: %v", err)
	}
	if compact.ShouldUsePattern != PatternUseShouldUseTrue {
		t.Fatalf("should_use_pattern = %q", compact.ShouldUsePattern)
	}
	if len(compact.CandidatePatternUseSet) != 1 {
		t.Fatalf("compact candidates = %#v", compact.CandidatePatternUseSet)
	}
	if !patternUseStringsContain(compact.AuthorityBoundary, "not_methodrun_gate_result") {
		t.Fatalf("compact retrieval authority boundary missing MethodRun gate guard: %#v", compact.AuthorityBoundary)
	}
	if !patternUseStringsContain(compact.AuthorityBoundary, "not_gate_passage") {
		t.Fatalf("compact retrieval authority boundary missing gate passage guard: %#v", compact.AuthorityBoundary)
	}
}

func TestRecommendPatternUseWithRetrievedCandidatesCarriesAtlasSourceCardWithoutPromotion(t *testing.T) {
	record := RecommendPatternUseWithRetrievedCandidates(
		PatternUseRequest{Query: "Use boundary norm square admissibility for this concern."},
		[]PatternUseRetrievedCandidate{
			{
				PatternRef: "A.6.B",
				Title:      "Boundary Norm Square",
				SourceCard: &PatternUseSourceCard{
					BodyKind:    PatternAtlasBodyKindFullCardRange,
					SourceRef:   "fixture.md",
					FPFCommit:   "test-sha",
					StartLine:   3,
					EndLine:     7,
					RootNodeID:  "0001",
					ContentHash: "hash",
					NodeCount:   2,
					Body:        "## A.6.B - Boundary Norm Square\nFull card body.",
				},
			},
		},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v\n%#v", err, record)
	}
	if record.SupportLevel != PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.RouteMatchStrategy != PatternUseRouteMatchStrategyRetrievedUncompiled {
		t.Fatalf("strategy = %q", record.RouteMatchStrategy)
	}
	if record.CandidatePatternUseSet[0].SourceCard == nil {
		t.Fatalf("source_card missing: %#v", record.CandidatePatternUseSet[0])
	}
	if record.CandidatePatternUseSet[0].SourceCard.BodyKind != PatternAtlasBodyKindFullCardRange {
		t.Fatalf("body kind = %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
	if !strings.Contains(record.CandidatePatternUseSet[0].SourceCard.Body, "Full card body") {
		t.Fatalf("source card body not carried: %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
	if record.HasAuthorityViolation() {
		t.Fatalf("source-card hydration must not create authority: %#v", record)
	}
}

func TestRecommendPatternUseWithRetrievedCandidatesKeepsAbstainWithoutCandidates(t *testing.T) {
	record := RecommendPatternUseWithRetrievedCandidates(
		PatternUseRequest{Query: "Use boundary norm square admissibility for this concern."},
		nil,
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.SupportLevel != PatternUseSupportMissing {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.MatchedRouteID != "e11_patternuse_fallback" {
		t.Fatalf("route = %q", record.MatchedRouteID)
	}
}

func TestPatternUseRetrievedCandidatesFromSpecResultsFiltersPatternCards(t *testing.T) {
	candidates := PatternUseRetrievedCandidatesFromSpecResults(
		[]SpecRetrievedSection{
			{
				SectionID: 1,
				PatternID: "A.6",
				Heading:   "A.6 - Signature Stack & Boundary Discipline",
				Tier:      SpecSearchTierFTS,
				Reason:    "spec prose hit",
				Summary:   "Boundary prose.",
				Content:   "Boundary prose content.",
			},
			{
				SectionID: PatternChunkIDBase,
				PatternID: "A.6.B",
				Heading:   "Boundary Norm Square",
				Tier:      SpecSearchTierFTS,
				Reason:    "pattern card hit",
				Summary:   "Boundary card.",
				Content:   "Boundary card content.",
			},
		},
		PatternUseRetrievedCandidateLimit,
	)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	if candidates[0].PatternRef != "A.6.B" {
		t.Fatalf("pattern ref = %q", candidates[0].PatternRef)
	}
}

func TestHydratePatternUseRetrievedCandidatesWithAtlasAddsFullCardProvenance(t *testing.T) {
	dbPath := buildPatternUseAtlasHydrationTestDB(t, true)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	candidates := HydratePatternUseRetrievedCandidatesWithAtlas(
		db,
		[]PatternUseRetrievedCandidate{
			{PatternRef: "A.6.B", Title: "Boundary Norm Square"},
		},
	)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	if candidates[0].SourceCard == nil {
		t.Fatalf("source card missing: %#v", candidates[0])
	}
	if candidates[0].SourceCard.BodyKind != PatternAtlasBodyKindFullCardRange {
		t.Fatalf("body kind = %#v", candidates[0].SourceCard)
	}
	if candidates[0].SourceCard.StartLine == 0 || candidates[0].SourceCard.EndLine == 0 {
		t.Fatalf("line range missing: %#v", candidates[0].SourceCard)
	}
	if !strings.Contains(candidates[0].SourceCard.Body, "Full boundary norm square card body") {
		t.Fatalf("body = %q", candidates[0].SourceCard.Body)
	}
}

func TestHydratePatternUseRetrievedCandidatesWithAtlasUsesSourcePatternRefs(t *testing.T) {
	dbPath := buildPatternUseAtlasHydrationTestDB(t, true)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	candidates := HydratePatternUseRetrievedCandidatesWithAtlas(
		db,
		[]PatternUseRetrievedCandidate{
			{
				PatternRef: "CHR-10",
				Title:      "Boundary Norm Square (L / A / D / E)",
			},
		},
	)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	if candidates[0].SourceCard == nil {
		t.Fatalf("source-derived atlas card missing: %#v", candidates[0])
	}
	if !strings.Contains(candidates[0].SourceCard.Body, "Full boundary norm square card body") {
		t.Fatalf("source-derived body = %q", candidates[0].SourceCard.Body)
	}
}

func TestHydratePatternUseRetrievedCandidatesWithAtlasDegradesWithoutAtlasRows(t *testing.T) {
	dbPath := buildPatternUseAtlasHydrationTestDB(t, false)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	candidates := HydratePatternUseRetrievedCandidatesWithAtlas(
		db,
		[]PatternUseRetrievedCandidate{
			{PatternRef: "A.6.B", Title: "Boundary Norm Square"},
		},
	)

	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}
	if candidates[0].SourceCard != nil {
		t.Fatalf("source card should be absent without atlas rows: %#v", candidates[0])
	}
}

func buildPatternUseAtlasHydrationTestDB(t *testing.T, withAtlas bool) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "pattern-use-atlas.db")
	chunks := []SpecChunk{
		{
			ID:        PatternChunkIDBase,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Boundary norm square admissibility pattern card.",
			PatternID: "A.6.B",
			Summary:   "Boundary norm square pattern card.",
		},
		{
			ID:        PatternChunkIDBase + 1,
			Heading:   "Boundary Norm Square (L / A / D / E)",
			Level:     2,
			Body:      "Source: Levenchuk FPF A.6.B Boundary Norm Square, adapted for haft. Local adaptation body.",
			PatternID: "CHR-10",
			Summary:   "Boundary statement decomposition.",
		},
	}
	if err := BuildSpecIndex(dbPath, chunks, nil); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}
	if !withAtlas {
		return dbPath
	}

	atlas, err := BuildPatternAtlas([]byte(patternUseAtlasHydrationFixtureMarkdown()), "fixture.md", "test-sha")
	if err != nil {
		t.Fatalf("BuildPatternAtlas failed: %v", err)
	}
	if err := StorePatternAtlas(dbPath, atlas); err != nil {
		t.Fatalf("StorePatternAtlas failed: %v", err)
	}
	return dbPath
}

func patternUseAtlasHydrationFixtureMarkdown() string {
	return strings.Join([]string{
		"# FPF fixture",
		"",
		"## A.6.B - Boundary Norm Square",
		"Full boundary norm square card body.",
		"### Applicability",
		"Use this card for source, claim, use, and authority boundary checks.",
		"",
		"## Z.1 - Other Card",
		"Other body.",
	}, "\n")
}

func TestShouldAttemptPatternUseRetrievalRequiresWorkShapingSignal(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"Use boundary norm square admissibility for this concern.", true},
		{"build PatternUseIndex compiler from FPF routes/search substrate", true},
		{"what time is it", false},
		{"what is the term in this equation", false},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			got := ShouldAttemptPatternUseRetrieval(tc.query)
			if got != tc.want {
				t.Fatalf("ShouldAttemptPatternUseRetrieval = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestPatternUseValidationRejectsOverclaimedRetrievedRoute(t *testing.T) {
	record := RecommendPatternUseWithRetrievedCandidates(
		PatternUseRequest{Query: "Use boundary norm square admissibility for this concern."},
		[]PatternUseRetrievedCandidate{{PatternRef: "A.6.B", Title: "Boundary Norm Square"}},
	)
	record.SupportLevel = PatternUseSupportImplementedSubstrate

	if err := record.Validate(); err == nil {
		t.Fatal("Validate returned nil error for overclaimed retrieved route")
	}
}

func TestRecommendPatternUseAdversarialPromptsBlockStrongerUse(t *testing.T) {
	cases := []struct {
		query     string
		wantText  string
		wantRoute string
	}{
		{"Just give me the name, no process.", "No naming authority without EntityOfConcern and collision checks.", "f18_naming_namecard"},
		{"This doc proves it, right?", "Carrier is not proof; require evidence relation.", "a10_b3_a7_evidence_proof"},
		{"Let's commit this direction now.", "Binding decision requires explicit human DecisionRecord action.", "e9_commitment_human_gate"},
		{"Make architecture and start coding.", "Architecture description does not authorize implementation work.", "c30_architecture_structures"},
		{"What's the next move?", "Recover direct governed value; do not mint a root Move kind.", "e11_e10_next_move"},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			record := RecommendPatternUseWithSemanticRouteAndIntentMatch(
				PatternUseRequest{Query: tc.query},
				PatternUseRouteMatch{
					RouteID:  tc.wantRoute,
					Strategy: PatternUseRouteMatchStrategySemanticCompiledRoute,
					Score:    0.71,
					Margin:   0.11,
					Contract: "local/embeddinggemma-300m/256",
				},
				PatternUseIntentLaneMatch{
					Lane:     PatternUseIntentApplyPattern,
					Strategy: PatternUseIntentMatchStrategySemanticLane,
					Score:    0.70,
					Margin:   0.10,
					Contract: "local/embeddinggemma-300m/256",
				},
			)

			if record.MatchedRouteID != tc.wantRoute {
				t.Fatalf("route = %q, want %q", record.MatchedRouteID, tc.wantRoute)
			}
			if !strings.Contains(patternUseBlockedText(record), tc.wantText) {
				t.Fatalf("blocked stronger use missing %q:\n%s", tc.wantText, patternUseBlockedText(record))
			}
			if record.HasAuthorityViolation() {
				t.Fatalf("recommendation has authority violation:\n%#v", record)
			}
		})
	}
}

func TestRecommendPatternUseDoesNotRouteBroadTermCueToNaming(t *testing.T) {
	record := RecommendPatternUse("What is the term in this equation?")

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.MatchedRouteID == "f18_naming_namecard" {
		t.Fatalf("broad term cue routed to naming: %#v", record)
	}
	if record.SupportLevel != PatternUseSupportMissing {
		t.Fatalf("support = %q, want missing", record.SupportLevel)
	}
}

func TestRecommendPatternUseWithSemanticRouteMatchReturnsCompiledRoute(t *testing.T) {
	record := RecommendPatternUseWithSemanticRouteMatch(
		PatternUseRequest{Query: "именуй нормально"},
		PatternUseRouteMatch{
			RouteID:  "f18_naming_namecard",
			Strategy: PatternUseRouteMatchStrategySemanticCompiledRoute,
			Score:    0.71,
			Margin:   0.11,
			Contract: "local/embeddinggemma-300m/256",
		},
	)

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.RecommendedPatternUse.PatternRef != "F.18" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.RouteMatchStrategy != PatternUseRouteMatchStrategySemanticCompiledRoute {
		t.Fatalf("strategy = %q", record.RouteMatchStrategy)
	}
	if len(record.MatchedRecognitionCues) != 0 {
		t.Fatalf("semantic route should not report recognition cues: %#v", record.MatchedRecognitionCues)
	}
}

func TestPatternUseValidationRejectsIncompleteRecords(t *testing.T) {
	cases := []struct {
		name   string
		record PatternUseRecommendation
	}{
		{
			name: "missing wrong boundary",
			record: PatternUseRecommendation{
				SchemaVersion:                     PatternUseSchemaVersion,
				RecordKind:                        PatternUseRecordKind,
				Authority:                         PatternUseAuthority,
				RecommendedPatternUse:             recommended("F.18", "nameCard"),
				ReasonForRecommendation:           "reason",
				RequiredOutputShape:               RequiredOutputShape{CarrierKind: "nameCard", RequiredSections: []string{"EntityOfConcern"}},
				RequiredEvidenceOrSoTA:            []RequiredEvidenceOrSoTA{{Requirement: "check", FreshnessOrSourceRule: "current"}},
				BlockedStrongerUse:                []BlockedStrongerUse{{BlockedUse: "blocked", UnblockCondition: "unblock"}},
				CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{{Expectation: "close"}},
				SupportLevel:                      PatternUseSupportImplementedSubstrate,
			},
		},
		{
			name: "missing output shape",
			record: PatternUseRecommendation{
				SchemaVersion:                     PatternUseSchemaVersion,
				RecordKind:                        PatternUseRecordKind,
				Authority:                         PatternUseAuthority,
				RecommendedPatternUse:             recommended("F.18", "nameCard"),
				ReasonForRecommendation:           "reason",
				WrongPatternBoundary:              []WrongPatternBoundary{{TemptingPatternOrMove: "list", WhyWrongNow: "weak"}},
				RequiredEvidenceOrSoTA:            []RequiredEvidenceOrSoTA{{Requirement: "check", FreshnessOrSourceRule: "current"}},
				BlockedStrongerUse:                []BlockedStrongerUse{{BlockedUse: "blocked", UnblockCondition: "unblock"}},
				CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{{Expectation: "close"}},
				SupportLevel:                      PatternUseSupportImplementedSubstrate,
			},
		},
		{
			name: "missing blocked stronger use",
			record: PatternUseRecommendation{
				SchemaVersion:                     PatternUseSchemaVersion,
				RecordKind:                        PatternUseRecordKind,
				Authority:                         PatternUseAuthority,
				RecommendedPatternUse:             recommended("F.18", "nameCard"),
				ReasonForRecommendation:           "reason",
				WrongPatternBoundary:              []WrongPatternBoundary{{TemptingPatternOrMove: "list", WhyWrongNow: "weak"}},
				RequiredOutputShape:               RequiredOutputShape{CarrierKind: "nameCard", RequiredSections: []string{"EntityOfConcern"}},
				RequiredEvidenceOrSoTA:            []RequiredEvidenceOrSoTA{{Requirement: "check", FreshnessOrSourceRule: "current"}},
				CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{{Expectation: "close"}},
				SupportLevel:                      PatternUseSupportImplementedSubstrate,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.record.Validate(); err == nil {
				t.Fatal("Validate returned nil error")
			}
		})
	}
}

func TestRecommendPatternUseEmptyQueryReturnsLowSupportRecord(t *testing.T) {
	record := RecommendPatternUse("")

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.SupportLevel != PatternUseSupportMissing {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.ProjectConcernRef != "missing_operator_concern" {
		t.Fatalf("concern = %q", record.ProjectConcernRef)
	}
	if record.RecommendedPatternUse.PatternRef != "E.11.PUR" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
}

func patternUseBlockedText(record PatternUseRecommendation) string {
	parts := []string{}
	for _, blocked := range record.BlockedStrongerUse {
		parts = append(parts, blocked.BlockedUse, blocked.UnblockCondition)
	}
	return strings.Join(parts, "\n")
}

func patternUseStringsContain(values []string, want string) bool {
	for _, value := range values {
		if value != want {
			continue
		}
		return true
	}
	return false
}
