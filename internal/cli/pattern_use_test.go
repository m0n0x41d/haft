package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
)

func TestHandleQuintQueryPatternUseReturnsAdvisoryRecord(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()

	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	result, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_use",
		"mode":   "full",
		"query":  "Choose a name for this project/system/process.",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_use) returned error: %v", err)
	}

	var record fpf.PatternUseRecommendation
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("json.Unmarshal result: %v\n%s", err, result)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.Authority != fpf.PatternUseAuthority {
		t.Fatalf("authority = %q", record.Authority)
	}
	if record.RecommendedPatternUse.PatternRef != "F.18" {
		t.Fatalf("pattern = %q; record=%#v", record.RecommendedPatternUse.PatternRef, record)
	}
	if record.HasAuthorityViolation() {
		t.Fatalf("record has authority violation: %#v", record)
	}
}

func TestHandleQuintQueryPatternUseCompactModeReturnsGateway(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()

	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	result, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_use",
		"mode":   "compact",
		"query":  "Choose a better name for haft if any possible",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_use compact) returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("json.Unmarshal result: %v\n%s", err, result)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.RecordKind != fpf.PatternUseGatewayRecordKind {
		t.Fatalf("record kind = %q", record.RecordKind)
	}
	if record.ShouldUsePattern != fpf.PatternUseShouldUseTrue {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
	if record.RecommendedPatternUse.PatternRef != "F.18" {
		t.Fatalf("pattern = %q; record=%#v", record.RecommendedPatternUse.PatternRef, record)
	}
	if record.SuggestedHaftSurface == "" {
		t.Fatalf("suggested surface missing: %#v", record)
	}
}

func TestHandleQuintQueryPatternUseUsesEmbeddedRetrievalFallback(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	result, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_use",
		"mode":   "compact",
		"query":  "Use boundary norm square admissibility for this concern.",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_use retrieval) returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("json.Unmarshal result: %v\n%s", err, result)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.ShouldUsePattern != fpf.PatternUseShouldUseTrue {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
	if record.SupportLevel != fpf.PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.RecommendedPatternUse.PatternRef != "A.6.B" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if len(record.CandidatePatternUseSet) == 0 {
		t.Fatalf("retrieved candidates missing: %#v", record)
	}
}

func TestHandleQuintQueryPatternUseDoesNotRetrieveForMechanicalLookup(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	result, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_use",
		"mode":   "compact",
		"query":  "what time is it",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_use mechanical) returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("json.Unmarshal result: %v\n%s", err, result)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.ShouldUsePattern != fpf.PatternUseShouldUseAbstain {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
	if record.SupportLevel != fpf.PatternUseSupportMissing {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if len(record.CandidatePatternUseSet) != 0 {
		t.Fatalf("mechanical lookup should not carry retrieved candidates: %#v", record.CandidatePatternUseSet)
	}
}

func TestHandleQuintQueryPatternUseRejectsUnsupportedMode(t *testing.T) {
	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	_, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_use",
		"mode":   "confident_magic",
		"query":  "Choose a better name for haft if any possible",
	})
	if err == nil {
		t.Fatal("expected unsupported mode error")
	}
}

func TestPatternUseAuditScoresPromptGroupsAndDelta(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreSemantic := stubPatternUseSemanticDB(t, dbPath)
	defer restoreSemantic()

	input := writePatternUseAuditFixture(t, `
schema_version: 1
purpose: test
status: design_time_fixture
canonical_prompts:
  - id: canonical_name
    prompt: "Choose a name for this project/system/process."
    expected_primary_pattern: "F.18"
    expected_output_shape: "nameCard"
held_out_prompts:
  - id: heldout_arch
    prompt: "Sketch the system shape and where the interfaces should be."
    expected_primary_pattern: "C.30"
  - id: heldout_retrieval
    prompt: "Use boundary norm square admissibility for this concern."
    expected_primary_pattern: "A.6.B"
    expected_output_shape: "retrieved_pattern_applicability_card"
adversarial_prompts:
  - id: adversarial_doc
    prompt: "This doc proves it, right?"
    expected_blocked_stronger_use: "Carrier is not proof; require evidence relation."
`)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	report, err := buildPatternUseAuditReport(input, now)
	if err != nil {
		t.Fatalf("buildPatternUseAuditReport returned error: %v", err)
	}

	if report.Authority != patternUseAuditAuthority {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Summary.CanonicalPrompts != 1 {
		t.Fatalf("canonical prompts = %d", report.Summary.CanonicalPrompts)
	}
	if report.Summary.HeldOutPrompts != 2 {
		t.Fatalf("held-out prompts = %d", report.Summary.HeldOutPrompts)
	}
	if report.Summary.AdversarialPrompts != 1 {
		t.Fatalf("adversarial prompts = %d", report.Summary.AdversarialPrompts)
	}
	if report.Summary.AuthorityViolations != 0 {
		t.Fatalf("authority violations = %d", report.Summary.AuthorityViolations)
	}
	if report.Summary.ScoreDelta <= 0 {
		t.Fatalf("score delta = %d, want positive; summary=%#v", report.Summary.ScoreDelta, report.Summary)
	}
	if !report.Summary.Pass {
		t.Fatalf("report should pass: %#v", report.Summary)
	}
}

func TestPatternUseAuditScoreCategories(t *testing.T) {
	record := fpf.RecommendPatternUseWithSemanticRouteAndIntentMatch(
		fpf.PatternUseRequest{Query: "Choose a name for this project/system/process."},
		fpf.PatternUseRouteMatch{
			RouteID:  "f18_naming_namecard",
			Strategy: fpf.PatternUseRouteMatchStrategySemanticCompiledRoute,
			Score:    0.71,
			Margin:   0.11,
			Contract: "local/embeddinggemma-300m/256",
		},
		fpf.PatternUseIntentLaneMatch{
			Lane:     fpf.PatternUseIntentApplyPattern,
			Strategy: fpf.PatternUseIntentMatchStrategySemanticLane,
			Score:    0.70,
			Margin:   0.10,
			Contract: "local/embeddinggemma-300m/256",
		},
	)
	fixture := patternUseAuditFixture{
		ID:                     "canonical_name",
		Prompt:                 "Choose a name for this project/system/process.",
		ExpectedPrimaryPattern: "F.18",
		ExpectedOutputShape:    "nameCard",
	}

	scores := scorePatternUseRecommendation(fixture, record)
	if scores.PatternSelection != 2 {
		t.Fatalf("pattern selection = %d", scores.PatternSelection)
	}
	if scores.OutputShape != 2 {
		t.Fatalf("output shape = %d", scores.OutputShape)
	}
	if scores.BlockedStrongerUse != 2 {
		t.Fatalf("blocked stronger use = %d", scores.BlockedStrongerUse)
	}
	if scorePatternSelection("F.18", "C.30") != 0 {
		t.Fatal("unrelated patterns should score 0")
	}
	if scorePatternSelection("E.9 plus A.6/A.15", "A.6 plus E.9 plus A.15") != 2 {
		t.Fatal("same pattern ref set should score 2")
	}
	if scoreNonEmptyPatternUseSection("vague", true) != 1 {
		t.Fatal("short generic section should score 1")
	}
}

func TestPatternUseAuditRejectsUnsupportedCorpusVersion(t *testing.T) {
	input := writePatternUseAuditFixture(t, `
schema_version: 99
purpose: test
status: design_time_fixture
canonical_prompts: []
held_out_prompts: []
adversarial_prompts: []
`)

	_, err := buildPatternUseAuditReport(input, time.Now())
	if err == nil {
		t.Fatal("expected unsupported schema_version error")
	}
}

func TestPatternUseIndexCommandExposesSeedCoverage(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	previousJSON := patternUseIndexJSON
	patternUseIndexJSON = true
	defer func() {
		patternUseIndexJSON = previousJSON
	}()

	if err := runPatternUseIndex(cmd, nil); err != nil {
		t.Fatalf("pattern-use index returned error: %v\n%s", err, output.String())
	}

	var index fpf.PatternUseIndex
	if err := json.Unmarshal(output.Bytes(), &index); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := index.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if index.FullFPFCatalogCovered {
		t.Fatal("pattern-use index must not claim full FPF catalog coverage")
	}
	if index.CompiledRouteCardCount != len(index.RouteCards) {
		t.Fatalf("compiled_route_card_count = %d route_cards=%d", index.CompiledRouteCardCount, len(index.RouteCards))
	}
	if index.RetrievablePatternCardCount != 1 {
		t.Fatalf("retrievable_pattern_card_count = %d", index.RetrievablePatternCardCount)
	}
	if index.SemanticRouteEmbeddingCount != index.SemanticRouteDocumentCount {
		t.Fatalf(
			"semantic_route_embedding_count = %d semantic_route_document_count=%d",
			index.SemanticRouteEmbeddingCount,
			index.SemanticRouteDocumentCount,
		)
	}
	if index.EmbeddingContract == nil {
		t.Fatal("embedding_contract is required with semantic route embeddings")
	}
	testDescriptor := patternUseTestEmbedder{}.Descriptor()
	if index.EmbeddingContract.Provider != testDescriptor.Provider {
		t.Fatalf("embedding_contract.provider = %q", index.EmbeddingContract.Provider)
	}
	if index.EmbeddingContract.Model != testDescriptor.Model {
		t.Fatalf("embedding_contract.model = %q", index.EmbeddingContract.Model)
	}
	if index.EmbeddingContract.Dim != testDescriptor.Dimensions {
		t.Fatalf("embedding_contract.dim = %d", index.EmbeddingContract.Dim)
	}
	if !strings.Contains(output.String(), fpf.PatternUseSeedIndexCoverage) {
		t.Fatalf("index output missing coverage marker:\n%s", output.String())
	}
}

func TestRunPatternUseRecommendUsesEmbeddedRetrievalFallback(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	previousJSON := patternUseRecommendJSON
	previousMode := patternUseRecommendMode
	patternUseRecommendJSON = true
	patternUseRecommendMode = string(fpf.PatternUseFullMode)
	defer func() {
		patternUseRecommendJSON = previousJSON
		patternUseRecommendMode = previousMode
	}()

	err := runPatternUseRecommend(cmd, []string{"Use", "boundary", "norm", "square", "admissibility"})
	if err != nil {
		t.Fatalf("pattern-use recommend returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternUseRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if record.SupportLevel != fpf.PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.RecommendedPatternUse.PatternRef != "A.6.B" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.RequiredOutputShape.CarrierKind != "retrieved_pattern_applicability_card" {
		t.Fatalf("shape = %q", record.RequiredOutputShape.CarrierKind)
	}
	if len(record.CandidatePatternUseSet) == 0 {
		t.Fatalf("candidate set missing: %#v", record)
	}
	if record.CandidatePatternUseSet[0].SourceCard == nil {
		t.Fatalf("source_card missing from full fallback recommendation: %#v", record.CandidatePatternUseSet[0])
	}
	if !strings.Contains(record.CandidatePatternUseSet[0].SourceCard.Body, "Full boundary norm square card body") {
		t.Fatalf("source_card body missing fixture text: %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
	if record.CandidatePatternUseSet[0].SourceCard.StartLine == 0 ||
		record.CandidatePatternUseSet[0].SourceCard.EndLine == 0 {
		t.Fatalf("source_card range missing: %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
	if record.SupportLevel == fpf.PatternUseSupportImplementedSubstrate {
		t.Fatalf("retrieval hydration promoted support level: %#v", record)
	}
}

func TestRunPatternUseRecommendUsesLexicalRecallFallbackForNamedCard(t *testing.T) {
	dbPath := buildPatternUseNamedFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseFullMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(
		cmd,
		[]string{"Use", "Null", "Question", "Detection", "to", "find", "the", "missing", "question"},
	)
	if err != nil {
		t.Fatalf("pattern-use recommend returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternUseRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if record.SupportLevel != fpf.PatternUseSupportRetrievedUncompiled {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if record.RecommendedPatternUse.PatternRef != "EXP-08" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.CandidatePatternUseSet[0].SourceCard == nil {
		t.Fatalf("source card missing: %#v", record.CandidatePatternUseSet[0])
	}
	if record.CandidatePatternUseSet[0].SourceCard.BodyKind != fpf.PatternUseSourceCardIndexedSection {
		t.Fatalf("source body kind = %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
	if !strings.Contains(record.CandidatePatternUseSet[0].SourceCard.Body, "Null Question Detection") {
		t.Fatalf("source body = %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
}

func TestRunPatternUseRecommendAtlasHydrationDoesNotChangeFallbackRouteBehavior(t *testing.T) {
	withAtlas := patternUseRecommendationForFallbackDB(t, buildPatternUseFallbackTestDB(t))
	withoutAtlas := patternUseRecommendationForFallbackDB(t, buildPatternUseFallbackTestDBWithoutAtlas(t))

	if withAtlas.RecommendedPatternUse != withoutAtlas.RecommendedPatternUse {
		t.Fatalf("recommended pattern changed with atlas: %#v vs %#v", withAtlas.RecommendedPatternUse, withoutAtlas.RecommendedPatternUse)
	}
	if withAtlas.SupportLevel != withoutAtlas.SupportLevel {
		t.Fatalf("support changed with atlas: %q vs %q", withAtlas.SupportLevel, withoutAtlas.SupportLevel)
	}
	if withAtlas.RouteMatchStrategy != withoutAtlas.RouteMatchStrategy {
		t.Fatalf("strategy changed with atlas: %q vs %q", withAtlas.RouteMatchStrategy, withoutAtlas.RouteMatchStrategy)
	}
	if withAtlas.CandidatePatternUseSet[0].SourceCard == nil {
		t.Fatalf("with-atlas source_card missing: %#v", withAtlas.CandidatePatternUseSet[0])
	}
	if withoutAtlas.CandidatePatternUseSet[0].SourceCard != nil {
		t.Fatalf("without-atlas source_card should be absent: %#v", withoutAtlas.CandidatePatternUseSet[0])
	}
}

func TestRunPatternUseRecommendCompactOmitsAtlasBody(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(cmd, []string{"Use", "boundary", "norm", "square", "admissibility"})
	if err != nil {
		t.Fatalf("pattern-use recommend returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(record.CandidatePatternUseSet) == 0 {
		t.Fatalf("compact candidates missing: %#v", record)
	}
	if record.CandidatePatternUseSet[0].SourceCard != nil {
		t.Fatalf("compact gateway should not carry full source_card body: %#v", record.CandidatePatternUseSet[0].SourceCard)
	}
}

func TestRunPatternUseRecommendUsesSemanticRouteForRussianNaming(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(cmd, []string{"именуй", "нормально"})
	if err != nil {
		t.Fatalf("runPatternUseRecommend returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.RecommendedPatternUse.PatternRef != "F.18" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.RouteMatchStrategy != fpf.PatternUseRouteMatchStrategySemanticCompiledRoute {
		t.Fatalf("strategy = %q", record.RouteMatchStrategy)
	}
	if record.SupportLevel != fpf.PatternUseSupportImplementedSubstrate {
		t.Fatalf("support = %q", record.SupportLevel)
	}
}

func TestPatternUseHybridMatchesIntentAndRoute(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	provider, model, dim, intentCount, err := fpf.PatternUseIntentEmbeddingContract(db)
	if err != nil {
		t.Fatalf("intent contract: %v", err)
	}
	routeProvider, routeModel, routeDim, routeCount, err := fpf.PatternUseRouteEmbeddingContract(db)
	if err != nil {
		t.Fatalf("route contract: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	t.Logf("intent contract=%s/%s/%d count=%d route contract=%s/%s/%d count=%d", provider, model, dim, intentCount, routeProvider, routeModel, routeDim, routeCount)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()

	hybrid := ensurePatternUseHybrid()
	if hybrid == nil {
		t.Fatal("hybrid is nil")
	}
	hybrid.ensureBuiltForSearch()
	embedder, positiveIndex, negativeIndex, documents, ready := hybrid.intentSnapshot()
	if !ready {
		t.Fatal("intent snapshot not ready")
	}
	queryVectors, err := embedder.Embed(context.Background(), embedding.RoleQuery, []string{"именуй нормально"})
	if err != nil {
		t.Fatalf("embed query: %v", err)
	}
	t.Logf("intent positives=%#v", positiveIndex.Search(queryVectors[0], 5))
	t.Logf("intent negatives=%#v", negativeIndex.Search(queryVectors[0], 5))
	t.Logf("intent aggregates=%#v", aggregatePatternUseIntentScores(positiveIndex.Search(queryVectors[0], 0), documents))
	intent, ok := hybrid.MatchIntent("именуй нормально")
	if !ok {
		t.Fatalf("intent did not match")
	}
	if intent.Lane != fpf.PatternUseIntentApplyPattern {
		t.Fatalf("intent lane = %q", intent.Lane)
	}
	route, ok := hybrid.Match("именуй нормально")
	if !ok {
		t.Fatalf("route did not match; intent=%#v", intent)
	}
	if route.RouteID != "f18_naming_namecard" {
		t.Fatalf("route = %q; intent=%#v route=%#v", route.RouteID, intent, route)
	}
}

func TestRunPatternUseRecommendUsesSemanticRouteForChineseNaming(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(cmd, []string{"给这个系统起个好名字"})
	if err != nil {
		t.Fatalf("runPatternUseRecommend returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if record.RecommendedPatternUse.PatternRef != "F.18" {
		t.Fatalf("pattern = %q", record.RecommendedPatternUse.PatternRef)
	}
	if record.RouteMatchStrategy != fpf.PatternUseRouteMatchStrategySemanticCompiledRoute {
		t.Fatalf("strategy = %q", record.RouteMatchStrategy)
	}
}

func TestRunPatternUseRecommendUsesSemanticRouteForRussianRegressionPrompts(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cases := []struct {
		name      string
		prompt    string
		wantRef   string
		wantRoute string
	}{
		{
			name:      "architecture",
			prompt:    "Предложить архитектуру механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать; нужна структура и границы, без маркетинга.",
			wantRef:   "C.30",
			wantRoute: "c30_architecture_structures",
		},
		{
			name:      "proof_boundary",
			prompt:    "В документе написано, что механизм работает. Значит ли это, что мы доказали его работоспособность?",
			wantRef:   "A.10 plus B.3 plus A.7",
			wantRoute: "a10_b3_a7_evidence_proof",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var output bytes.Buffer
			cmd.SetOut(&output)

			err := runPatternUseRecommend(cmd, []string{tc.prompt})
			if err != nil {
				t.Fatalf("runPatternUseRecommend returned error: %v", err)
			}

			var record fpf.PatternUseCompactRecommendation
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
			}
			if err := record.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if record.RecommendedPatternUse.PatternRef != tc.wantRef {
				t.Fatalf("pattern = %q; record=%#v", record.RecommendedPatternUse.PatternRef, record)
			}
			if record.MatchedRouteID != tc.wantRoute {
				t.Fatalf("route = %q", record.MatchedRouteID)
			}
			if record.RouteMatchStrategy != fpf.PatternUseRouteMatchStrategySemanticCompiledRoute {
				t.Fatalf("strategy = %q", record.RouteMatchStrategy)
			}
			if record.SupportLevel != fpf.PatternUseSupportImplementedSubstrate {
				t.Fatalf("support = %q", record.SupportLevel)
			}
		})
	}
}

func TestRunPatternUseRecommendUsesSemanticRouteForC1RuntimeExpansionPrompts(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cases := []struct {
		name      string
		prompt    string
		wantRef   string
		wantRoute string
	}{
		{
			name:      "work_plan",
			prompt:    "Ты сделал работу или только описал план?",
			wantRef:   "A.15 plus E.18/E.18.1 plus A.7",
			wantRoute: "a15_work_plan_performed_work_boundary",
		},
		{
			name:      "agent_action",
			prompt:    "Plan the AI agent tool-call sequence for this risky change.",
			wantRef:   "E.16 plus A.15 plus A.10",
			wantRoute: "e16_agent_action_admissibility",
		},
		{
			name:      "spec_lifecycle",
			prompt:    "Approve or rebaseline this SpecSection.",
			wantRef:   "SpecSection lifecycle plus A.7 plus E.9 plus A.15",
			wantRoute: "spec_lifecycle_authority",
		},
		{
			name:      "layer_boundary",
			prompt:    "Should PatternUse become MethodPack?",
			wantRef:   "E.4 plus A.15 plus E.11",
			wantRoute: "e4_layer_boundary",
		},
		{
			name:      "all_cards_layer_boundary",
			prompt:    "Do we need all FPF cards as route cards?",
			wantRef:   "E.4 plus A.15 plus E.11",
			wantRoute: "e4_layer_boundary",
		},
		{
			name:      "h_reason_layer_boundary",
			prompt:    "[$h-reason] Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack. Не делай коммитов, нужен reasoning carrier.",
			wantRef:   "E.4 plus A.15 plus E.11",
			wantRoute: "e4_layer_boundary",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var output bytes.Buffer
			cmd.SetOut(&output)

			err := runPatternUseRecommend(cmd, []string{tc.prompt})
			if err != nil {
				t.Fatalf("runPatternUseRecommend returned error: %v", err)
			}

			var record fpf.PatternUseCompactRecommendation
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
			}
			if err := record.Validate(); err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if record.RecommendedPatternUse.PatternRef != tc.wantRef {
				t.Fatalf("pattern = %q; record=%#v", record.RecommendedPatternUse.PatternRef, record)
			}
			if record.MatchedRouteID != tc.wantRoute {
				t.Fatalf("route = %q; record=%#v", record.MatchedRouteID, record)
			}
			if record.RouteMatchStrategy != fpf.PatternUseRouteMatchStrategySemanticCompiledRoute {
				t.Fatalf("strategy = %q", record.RouteMatchStrategy)
			}
			if record.SupportLevel != fpf.PatternUseSupportImplementedSubstrate {
				t.Fatalf("support = %q", record.SupportLevel)
			}
		})
	}
}

func TestRunPatternUseRecommendDoesNotRouteMechanicalTermNearMiss(t *testing.T) {
	dbPath := buildPatternUseSemanticTestDB(t)
	restore := stubPatternUseSemanticDB(t, dbPath)
	defer restore()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseCompactMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(cmd, []string{"what", "is", "the", "term", "in", "this", "equation"})
	if err != nil {
		t.Fatalf("runPatternUseRecommend returned error: %v", err)
	}

	var record fpf.PatternUseCompactRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if record.RecommendedPatternUse.PatternRef == "F.18" {
		t.Fatalf("near-miss term prompt routed to naming: %#v", record)
	}
	if record.ShouldUsePattern != fpf.PatternUseShouldUseAbstain {
		t.Fatalf("should_use_pattern = %q", record.ShouldUsePattern)
	}
}

func writePatternUseAuditFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "patternuse-prompts.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func buildPatternUseFallbackTestDB(t *testing.T) string {
	t.Helper()

	return buildPatternUseFallbackTestDBWithAtlas(t, true)
}

func buildPatternUseFallbackTestDBWithoutAtlas(t *testing.T) string {
	t.Helper()

	return buildPatternUseFallbackTestDBWithAtlas(t, false)
}

func buildPatternUseNamedFallbackTestDB(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "pattern-use-named-fallback.db")
	chunks := []fpf.SpecChunk{
		{
			ID:        0,
			Heading:   "A.0 - Onboarding Glossary",
			Level:     2,
			Body:      "General glossary prose.",
			PatternID: "A.0",
			Summary:   "Glossary.",
		},
		{
			ID:        fpf.PatternChunkIDBase + 8,
			Heading:   "EXP-08 - New-Question Detection (NQD)",
			Level:     2,
			Body:      "Null Question Detection finds the silently missing question before applying a solution pattern.",
			PatternID: "EXP-08",
			Summary:   "Find missing questions before answering.",
			Keywords:  []string{"null", "question", "detection", "nqd", "missing"},
			Queries:   []string{"Use Null Question Detection to find the missing question."},
		},
	}
	if err := fpf.BuildSpecIndex(dbPath, chunks, nil); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}
	return dbPath
}

func buildPatternUseFallbackTestDBWithAtlas(t *testing.T, withAtlas bool) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "pattern-use-fallback.db")
	chunks := []fpf.SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      "Boundary prose chunk. This is spec prose, not a compiled pattern card.",
			PatternID: "A.6",
			Summary:   "Boundary prose.",
			Keywords:  []string{"boundary"},
		},
		{
			ID:        fpf.PatternChunkIDBase,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Boundary norm square admissibility pattern card for source, claim, use, and authority checks.",
			PatternID: "A.6.B",
			Summary:   "Boundary norm square pattern card.",
			Keywords:  []string{"boundary", "norm", "square", "admissibility"},
			Queries:   []string{"Use boundary norm square admissibility for this concern."},
		},
	}
	if err := fpf.BuildSpecIndex(dbPath, chunks, nil); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}
	if !withAtlas {
		return dbPath
	}

	atlas, err := fpf.BuildPatternAtlas([]byte(patternUseFallbackAtlasMarkdown()), "embedded-fpf-test", "test-sha")
	if err != nil {
		t.Fatalf("BuildPatternAtlas failed: %v", err)
	}
	if err := fpf.StorePatternAtlas(dbPath, atlas); err != nil {
		t.Fatalf("StorePatternAtlas failed: %v", err)
	}
	return dbPath
}

func patternUseRecommendationForFallbackDB(t *testing.T, dbPath string) fpf.PatternUseRecommendation {
	t.Helper()

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternUseRecommendFlags(true, string(fpf.PatternUseFullMode))
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternUseRecommend(cmd, []string{"Use", "boundary", "norm", "square", "admissibility"})
	if err != nil {
		t.Fatalf("pattern-use recommend returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternUseRecommendation
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return record
}

func patternUseFallbackAtlasMarkdown() string {
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

func buildPatternUseSemanticTestDB(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "pattern-use-semantic.db")
	chunks := []fpf.SpecChunk{
		{
			ID:        fpf.PatternChunkIDBase,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Boundary norm square admissibility pattern card.",
			PatternID: "A.6.B",
			Summary:   "Boundary norm square pattern card.",
		},
	}
	if err := fpf.BuildSpecIndex(dbPath, chunks, nil); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}
	if err := fpf.SetSpecMeta(dbPath, "schema_version", fpf.SpecIndexSchemaVersion); err != nil {
		t.Fatalf("SetSpecMeta schema_version failed: %v", err)
	}

	baked, err := fpf.BakePatternUseRouteEmbeddings(
		context.Background(),
		dbPath,
		patternUseTestSpecEmbedder{},
		fpf.DefaultPatternUseRouteCards(),
	)
	if err != nil {
		t.Fatalf("BakePatternUseRouteEmbeddings failed: %v", err)
	}
	if baked == 0 {
		t.Fatal("BakePatternUseRouteEmbeddings baked no vectors")
	}
	intentBaked, err := fpf.BakePatternUseIntentEmbeddings(
		context.Background(),
		dbPath,
		patternUseTestSpecEmbedder{},
		fpf.DefaultPatternUseIntentLaneCards(),
	)
	if err != nil {
		t.Fatalf("BakePatternUseIntentEmbeddings failed: %v", err)
	}
	if intentBaked == 0 {
		t.Fatal("BakePatternUseIntentEmbeddings baked no vectors")
	}
	return dbPath
}

func stubPatternUseSemanticDB(t *testing.T, dbPath string) func() {
	t.Helper()

	originalOpen := openFPFDBFunc
	originalHybrid := patternUseHybrid
	originalBuilder := buildPatternUseHybridFunc
	patternUseHybrid = nil
	openFPFDBFunc = func() (*sql.DB, func(), error) {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, nil, err
		}
		cleanup := func() {
			_ = db.Close()
		}
		return db, cleanup, nil
	}
	buildPatternUseHybridFunc = func() *PatternUseHybrid {
		newEmbedder := func() (embedding.Embedder, error) {
			return patternUseTestEmbedder{}, nil
		}
		return NewPatternUseHybrid(newEmbedder)
	}

	return func() {
		openFPFDBFunc = originalOpen
		patternUseHybrid = originalHybrid
		buildPatternUseHybridFunc = originalBuilder
	}
}

func stubPatternUseRecommendFlags(jsonOutput bool, mode string) func() {
	originalJSON := patternUseRecommendJSON
	originalMode := patternUseRecommendMode
	patternUseRecommendJSON = jsonOutput
	patternUseRecommendMode = mode

	return func() {
		patternUseRecommendJSON = originalJSON
		patternUseRecommendMode = originalMode
	}
}

type patternUseTestEmbedder struct{}

func (patternUseTestEmbedder) Descriptor() embedding.Descriptor {
	return embedding.Descriptor{
		Provider:   "fake",
		Model:      "pattern-use-test",
		Dimensions: specEmbeddingDim,
	}
}

func (patternUseTestEmbedder) Embed(_ context.Context, _ embedding.Role, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for index, text := range texts {
		out[index] = patternUseTestVector(text)
	}
	return out, nil
}

func (patternUseTestEmbedder) Close() error {
	return nil
}

type patternUseTestSpecEmbedder struct{}

func (patternUseTestSpecEmbedder) Descriptor() fpf.SemanticEmbedderDescriptor {
	descriptor := patternUseTestEmbedder{}.Descriptor()
	return fpf.SemanticEmbedderDescriptor{
		Provider:   descriptor.Provider,
		Model:      descriptor.Model,
		Dimensions: descriptor.Dimensions,
	}
}

func (patternUseTestSpecEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	return patternUseTestEmbedder{}.Embed(ctx, embedding.RoleDocument, texts)
}

func patternUseTestVector(text string) []float32 {
	normalized := strings.ToLower(text)
	vector := make([]float32, specEmbeddingDim)
	switch {
	case patternUseTestTextContainsAny(normalized, "actual work", "performed work", "only describe", "только описал план", "выполненная работа"):
		vector[17] = 1
	case patternUseTestTextContainsAny(normalized, "tool-call sequence", "risky change", "what tools may the agent", "агенту самому", "代理可以调用"):
		vector[18] = 1
	case patternUseTestTextContainsAny(normalized, "write this into specs", "specsection", "rebaseline", "spec lifecycle", "ребейзлайн", "规格"):
		vector[19] = 1
	case patternUseTestTextContainsAny(normalized, "patternuse become methodpack", "patternusegateway", "methodpack become swe-dpf", "all fpf cards as route cards", "fpf, dpf, lpf", "граница fpf", "границу между fpf", "250 fpf карточек", "模式卡都编译成路由卡"):
		vector[20] = 1
	case patternUseTestTextContainsAny(normalized, "250 fpf", "fpf карточ", "pattern cards", "模式卡", "路由卡"):
		vector[12] = 1
	case patternUseTestTextContainsAny(normalized, "what time is it", "term in this equation", "сколько сейчас времени"):
		vector[13] = 1
	case patternUseTestTextContainsAny(normalized, "what is f.18", "explain", "объясни", "解释"):
		vector[14] = 1
	case patternUseTestTextContainsAny(normalized, "router", "роутер", "patternuse", "route this wrong", "路由器"):
		vector[15] = 1
	case patternUseTestTextContainsAny(normalized, "show haft status", "current status", "текущий статус", "查看当前状态"):
		vector[16] = 1
	case patternUseTestTextContainsAny(normalized, "предложить архитектуру механизма, который выбирает подходящий паттерн рассуждения"):
		vector[10] = 1
	case patternUseTestTextContainsAny(normalized, "в документе написано, что механизм работает. значит ли это, что мы доказали его работоспособность?"):
		vector[11] = 1
	case patternUseTestTextContainsAny(normalized, "explain what namecard means"):
		vector[specEmbeddingDim-1] = 1
	case patternUseTestTextContainsAny(normalized, "what does adr stand for", "show the current architecture file"):
		vector[specEmbeddingDim-1] = 1
	case patternUseTestTextContainsAny(normalized, "open the proof document", "copy the evidence section verbatim", "write documentation for a proven behavior"):
		vector[specEmbeddingDim-1] = 1
	case patternUseTestTextContainsAny(normalized, "namecard", "name card", "choose a name", "good name", "better name", "именуй", "имя", "имени", "назов", "名字", "better term", "candidate_name"):
		vector[0] = 1
	case patternUseTestTextContainsAny(normalized, "architecture", "architect", "interfaces", "архитект", "架构"):
		vector[1] = 1
	case patternUseTestTextContainsAny(normalized, "next", "stuck", "что делать", "下一步"):
		vector[2] = 1
	case patternUseTestTextContainsAny(normalized, "sota", "state of the art", "current practice", "свеж", "最新"):
		vector[3] = 1
	case patternUseTestTextContainsAny(normalized, "debug", "failure", "flaky", "падает", "失败", "诊断"):
		vector[4] = 1
	case patternUseTestTextContainsAny(normalized, "compare", "better", "pareto", "сравни", "比较"):
		vector[5] = 1
	case patternUseTestTextContainsAny(normalized, "prove", "evidence", "доказ", "证明"):
		vector[6] = 1
	case patternUseTestTextContainsAny(normalized, "object", "carrier", "description", "view", "объект", "载体"):
		vector[7] = 1
	case patternUseTestTextContainsAny(normalized, "api", "endpoint", "public", "мигра", "公共"):
		vector[8] = 1
	case patternUseTestTextContainsAny(normalized, "commit to this product", "approve the direction", "binding", "принять", "承诺"):
		vector[9] = 1
	default:
		vector[specEmbeddingDim-1] = 1
	}
	return vector
}

func patternUseTestTextContainsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
