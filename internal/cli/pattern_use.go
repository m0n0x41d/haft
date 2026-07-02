package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	patternUseAuditKind      = "pattern_use_routing_behavior_audit"
	patternUseAuditAuthority = "read_only_pattern_use_audit_not_enforcement_gate"
)

var patternUseAuditMutationBoundary = []string{
	"read_only_pattern_use_fixture_audit",
	"does_not_mutate_method_runs_decisions_evidence_or_carriers",
	"does_not_create_decision_workcommission_gate_evidence_claim_truth_global_truth_or_publication",
}

var (
	patternUseRecommendJSON bool
	patternUseRecommendMode string
	patternUseAuditInput    string
	patternUseAuditJSON     bool
	patternUseIndexJSON     bool
)

var patternUseCmd = &cobra.Command{
	Use:   "pattern-use",
	Short: "Recommend and audit concrete FPF pattern-use routing",
}

var patternUseRecommendCmd = &cobra.Command{
	Use:   "recommend <operator-task>",
	Short: "Return a read-only PatternUseRecommendation for one task",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runPatternUseRecommend,
}

var patternUseAuditCmd = &cobra.Command{
	Use:   "audit --input FILE",
	Short: "Audit PatternUse router behavior over a prompt corpus",
	RunE:  runPatternUseAudit,
}

var patternUseIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Show the read-only PatternUseIndex seed coverage",
	RunE:  runPatternUseIndex,
}

func init() {
	patternUseRecommendCmd.Flags().BoolVar(&patternUseRecommendJSON, "json", false, "print the recommendation as JSON")
	patternUseRecommendCmd.Flags().StringVar(&patternUseRecommendMode, "mode", string(fpf.PatternUseCompactMode), "recommendation mode: compact or full")
	patternUseAuditCmd.Flags().StringVar(&patternUseAuditInput, "input", "", "YAML prompt corpus")
	patternUseAuditCmd.Flags().BoolVar(&patternUseAuditJSON, "json", false, "print the full audit as JSON")
	patternUseIndexCmd.Flags().BoolVar(&patternUseIndexJSON, "json", false, "print the index as JSON")
	patternUseCmd.AddCommand(patternUseRecommendCmd)
	patternUseCmd.AddCommand(patternUseAuditCmd)
	patternUseCmd.AddCommand(patternUseIndexCmd)
	rootCmd.AddCommand(patternUseCmd)
}

type patternUseAuditCorpus struct {
	SchemaVersion    int                      `yaml:"schema_version"`
	Purpose          string                   `yaml:"purpose"`
	Status           string                   `yaml:"status"`
	CanonicalPrompts []patternUseAuditFixture `yaml:"canonical_prompts"`
	HeldOutPrompts   []patternUseAuditFixture `yaml:"held_out_prompts"`
	Adversarial      []patternUseAuditFixture `yaml:"adversarial_prompts"`
}

type patternUseAuditFixture struct {
	ID                         string `yaml:"id" json:"id"`
	Prompt                     string `yaml:"prompt" json:"prompt"`
	ExpectedPrimaryPattern     string `yaml:"expected_primary_pattern,omitempty" json:"expected_primary_pattern,omitempty"`
	ForbiddenPrimaryPattern    string `yaml:"forbidden_primary_pattern,omitempty" json:"forbidden_primary_pattern,omitempty"`
	ExpectedOutputShape        string `yaml:"expected_output_shape,omitempty" json:"expected_output_shape,omitempty"`
	ExpectedBlockedStrongerUse string `yaml:"expected_blocked_stronger_use,omitempty" json:"expected_blocked_stronger_use,omitempty"`
}

type patternUseAuditReport struct {
	SchemaVersion    int                    `json:"schema_version"`
	AuditKind        string                 `json:"audit_kind"`
	Authority        string                 `json:"authority"`
	MutationBoundary []string               `json:"mutation_boundary"`
	BaselineSurface  string                 `json:"baseline_surface"`
	RoutedSurface    string                 `json:"routed_surface"`
	RunAt            string                 `json:"run_at"`
	CorpusRef        string                 `json:"corpus_ref"`
	Summary          patternUseAuditSummary `json:"summary"`
	Rows             []patternUseAuditRow   `json:"rows"`
}

type patternUseAuditSummary struct {
	Prompts             int    `json:"prompts"`
	CanonicalPrompts    int    `json:"canonical_prompts"`
	HeldOutPrompts      int    `json:"held_out_prompts"`
	AdversarialPrompts  int    `json:"adversarial_prompts"`
	BaselineScore       int    `json:"baseline_score"`
	RoutedScore         int    `json:"routed_score"`
	ScoreDelta          int    `json:"score_delta"`
	MaxScore            int    `json:"max_score"`
	RowsPassing         int    `json:"rows_passing"`
	RowsFailing         int    `json:"rows_failing"`
	AuthorityViolations int    `json:"authority_violations"`
	Pass                bool   `json:"pass"`
	BaselineMeasurement string `json:"baseline_measurement"`
}

type patternUseAuditRow struct {
	PromptGroup                       string                                  `json:"prompt_group"`
	PromptID                          string                                  `json:"prompt_id"`
	Prompt                            string                                  `json:"prompt"`
	ExpectedPrimaryPattern            string                                  `json:"expected_primary_pattern,omitempty"`
	ForbiddenPrimaryPattern           string                                  `json:"forbidden_primary_pattern,omitempty"`
	ExpectedOutputShape               string                                  `json:"expected_output_shape,omitempty"`
	ExpectedBlockedStrongerUse        string                                  `json:"expected_blocked_stronger_use,omitempty"`
	ObservedPrimaryPattern            string                                  `json:"observed_primary_pattern"`
	SupportLevel                      fpf.PatternUseSupportLevel              `json:"support_level"`
	Scores                            patternUseAuditScores                   `json:"scores"`
	BaselineScores                    patternUseAuditScores                   `json:"baseline_scores"`
	ScoreDelta                        int                                     `json:"score_delta"`
	AuthorityViolation                bool                                    `json:"authority_violation"`
	FailureReason                     string                                  `json:"failure_reason"`
	RouteCardEditNeeded               string                                  `json:"route_card_edit_needed"`
	RecommendedPatternUse             fpf.PatternUseRef                       `json:"recommended_pattern_use"`
	RequiredOutputShape               fpf.RequiredOutputShape                 `json:"required_output_shape"`
	WrongPatternBoundary              []fpf.WrongPatternBoundary              `json:"wrong_pattern_boundary"`
	BlockedStrongerUse                []fpf.BlockedStrongerUse                `json:"blocked_stronger_use"`
	CloseoutOrVerificationExpectation []fpf.CloseoutOrVerificationExpectation `json:"closeout_or_verification_expectation"`
}

type patternUseAuditScores struct {
	PatternSelection   int `json:"pattern_selection"`
	ConcernRecovery    int `json:"concern_recovery"`
	WrongBoundary      int `json:"wrong_boundary"`
	OutputShape        int `json:"output_shape"`
	EvidenceOrSoTA     int `json:"evidence_or_sota"`
	BlockedStrongerUse int `json:"blocked_stronger_use"`
	ActualBehavior     int `json:"actual_behavior"`
}

func runPatternUseRecommend(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	mode, err := fpf.NormalizePatternUseMode(patternUseRecommendMode)
	if err != nil {
		return err
	}

	if mode == fpf.PatternUseFullMode {
		record, err := recommendPatternUseWithEmbeddedFallback(fpf.PatternUseRequest{Query: query})
		if err != nil {
			return err
		}
		if err := record.Validate(); err != nil {
			return err
		}
		if patternUseRecommendJSON {
			return writeJSON(cmd.OutOrStdout(), record)
		}
		return writePatternUseRecommendationText(cmd.OutOrStdout(), record)
	}

	record, err := recommendPatternUseCompactWithEmbeddedFallback(fpf.PatternUseRequest{Query: query})
	if err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	if patternUseRecommendJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}
	return writePatternUseCompactRecommendationText(cmd.OutOrStdout(), record)
}

func runPatternUseAudit(cmd *cobra.Command, _ []string) error {
	if strings.TrimSpace(patternUseAuditInput) == "" {
		return fmt.Errorf("--input is required")
	}

	report, err := buildPatternUseAuditReport(patternUseAuditInput, timeNow())
	if err != nil {
		return err
	}
	if patternUseAuditJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writePatternUseAuditText(cmd.OutOrStdout(), report)
}

func runPatternUseIndex(cmd *cobra.Command, _ []string) error {
	index, err := patternUseIndexWithEmbeddedSummary()
	if err != nil {
		return err
	}
	if err := index.Validate(); err != nil {
		return err
	}
	if patternUseIndexJSON {
		return writeJSON(cmd.OutOrStdout(), index)
	}
	return writePatternUseIndexText(cmd.OutOrStdout(), index)
}

func buildPatternUseAuditReport(inputPath string, now time.Time) (patternUseAuditReport, error) {
	corpus, err := readPatternUseAuditCorpus(inputPath)
	if err != nil {
		return patternUseAuditReport{}, err
	}

	report := patternUseAuditReport{
		SchemaVersion:    1,
		AuditKind:        patternUseAuditKind,
		Authority:        patternUseAuditAuthority,
		MutationBoundary: append([]string(nil), patternUseAuditMutationBoundary...),
		BaselineSurface:  "h-reason_carrier_pre_router_static_proxy",
		RoutedSurface:    "pattern_use_router",
		RunAt:            now.UTC().Format(time.RFC3339),
		CorpusRef:        inputPath,
		Summary: patternUseAuditSummary{
			BaselineMeasurement: "static_proxy_from_current_h_reason_phase_routing_no_llm_judge",
		},
	}

	rows, err := buildPatternUseAuditRows("canonical", corpus.CanonicalPrompts)
	if err != nil {
		return patternUseAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	rows, err = buildPatternUseAuditRows("held_out", corpus.HeldOutPrompts)
	if err != nil {
		return patternUseAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	rows, err = buildPatternUseAuditRows("adversarial", corpus.Adversarial)
	if err != nil {
		return patternUseAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	report.Summary = summarizePatternUseAudit(report.Rows)
	return report, nil
}

func readPatternUseAuditCorpus(inputPath string) (patternUseAuditCorpus, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return patternUseAuditCorpus{}, err
	}

	var corpus patternUseAuditCorpus
	if err := yaml.Unmarshal(data, &corpus); err != nil {
		return patternUseAuditCorpus{}, err
	}
	if corpus.SchemaVersion != 1 {
		return patternUseAuditCorpus{}, fmt.Errorf("unsupported pattern-use audit corpus schema_version %d", corpus.SchemaVersion)
	}
	return corpus, nil
}

func buildPatternUseAuditRows(group string, fixtures []patternUseAuditFixture) ([]patternUseAuditRow, error) {
	rows := make([]patternUseAuditRow, 0, len(fixtures))
	for _, fixture := range fixtures {
		record, err := recommendPatternUseWithEmbeddedFallback(fpf.PatternUseRequest{Query: fixture.Prompt})
		if err != nil {
			return nil, err
		}
		row := buildPatternUseAuditRow(group, fixture, record)
		rows = append(rows, row)
	}
	return rows, nil
}

func recommendPatternUseWithEmbeddedFallback(
	request fpf.PatternUseRequest,
) (fpf.PatternUseRecommendation, error) {
	seed := fpf.RecommendPatternUseWithContext(request)
	if strings.TrimSpace(request.Query) == "" {
		return seed, nil
	}

	var intentMatch fpf.PatternUseIntentLaneMatch
	intentMatched := false
	hybrid := ensurePatternUseHybrid()
	if hybrid != nil {
		intentMatch, intentMatched = hybrid.MatchIntent(request.Query)
		if intentMatched {
			seed = fpf.WithPatternUseIntentMatch(seed, intentMatch)
		}
	}

	if intentMatched && fpf.PatternUseIntentLaneAllowsCompiledRoute(intentMatch.Lane) && hybrid != nil {
		if routeMatch, ok := hybrid.Match(request.Query); ok {
			record := fpf.RecommendPatternUseWithSemanticRouteAndIntentMatch(request, routeMatch, intentMatch)
			return record, nil
		}
	}

	if !shouldAttemptPatternUseRetrievalAfterIntent(request.Query, intentMatch, intentMatched) {
		return seed, nil
	}

	retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
		Query: request.Query,
		Limit: fpf.PatternUseRetrievedCandidateLimit * 3,
	})
	if err != nil {
		return fpf.PatternUseRecommendation{}, fmt.Errorf("retrieve FPF pattern-use candidates: %w", err)
	}
	candidates := fpf.PatternUseRetrievedCandidatesFromSpecResults(
		retrieval.Results,
		fpf.PatternUseRetrievedCandidateLimit,
	)
	candidates = hydratePatternUseRetrievedCandidatesWithEmbeddedAtlas(candidates)
	record := fpf.RecommendPatternUseWithRetrievedCandidates(request, candidates)
	if intentMatched {
		record = fpf.WithPatternUseIntentMatch(record, intentMatch)
	}
	return record, nil
}

func shouldAttemptPatternUseRetrievalAfterIntent(
	query string,
	intentMatch fpf.PatternUseIntentLaneMatch,
	intentMatched bool,
) bool {
	if intentMatched {
		return fpf.PatternUseIntentLanePermitsRetrieval(intentMatch.Lane)
	}
	return fpf.ShouldAttemptPatternUseRetrieval(query)
}

func hydratePatternUseRetrievedCandidatesWithEmbeddedAtlas(
	candidates []fpf.PatternUseRetrievedCandidate,
) []fpf.PatternUseRetrievedCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return candidates
	}
	defer cleanup()

	return fpf.HydratePatternUseRetrievedCandidatesWithAtlas(db, candidates)
}

func recommendPatternUseCompactWithEmbeddedFallback(
	request fpf.PatternUseRequest,
) (fpf.PatternUseCompactRecommendation, error) {
	record, err := recommendPatternUseWithEmbeddedFallback(request)
	if err != nil {
		return fpf.PatternUseCompactRecommendation{}, err
	}
	return fpf.CompactPatternUseRecommendation(record), nil
}

func recordToRetrievedCandidates(record fpf.PatternUseRecommendation) []fpf.PatternUseRetrievedCandidate {
	if record.SupportLevel != fpf.PatternUseSupportRetrievedUncompiled {
		return nil
	}
	candidates := make([]fpf.PatternUseRetrievedCandidate, 0, len(record.CandidatePatternUseSet))
	for _, candidate := range record.CandidatePatternUseSet {
		candidates = append(candidates, fpf.PatternUseRetrievedCandidate{
			PatternRef:   candidate.PatternRef,
			Title:        candidate.Title,
			Summary:      candidate.Summary,
			Snippet:      candidate.Snippet,
			SourceTier:   candidate.SourceTier,
			SourceReason: candidate.SourceReason,
			SourceCard:   candidate.SourceCard,
		})
	}
	return candidates
}

func patternUseIndexWithEmbeddedSummary() (fpf.PatternUseIndex, error) {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return fpf.PatternUseIndex{}, fmt.Errorf("open fpf db: %w", err)
	}
	defer cleanup()

	count, err := fpf.CountPatternCardSections(db)
	if err != nil {
		return fpf.PatternUseIndex{}, fmt.Errorf("count FPF pattern cards: %w", err)
	}
	provider, model, dim, routeCount, err := fpf.PatternUseRouteEmbeddingContract(db)
	if err != nil {
		return fpf.PatternUseIndex{}, fmt.Errorf("read PatternUse route embedding contract: %w", err)
	}
	intentProvider, intentModel, intentDim, intentCount, err := fpf.PatternUseIntentEmbeddingContract(db)
	if err != nil {
		return fpf.PatternUseIndex{}, fmt.Errorf("read PatternUse intent embedding contract: %w", err)
	}
	contract := fpf.PatternUseEmbeddingContract{}
	if routeCount > 0 {
		contract = fpf.PatternUseEmbeddingContractFor(
			provider,
			model,
			dim,
		)
	}
	if routeCount == 0 && intentCount > 0 {
		contract = fpf.PatternUseEmbeddingContractFor(
			intentProvider,
			intentModel,
			intentDim,
		)
	}
	return fpf.PatternUseIndexWithRetrievalRouteAndIntentSemanticContract(count, routeCount, intentCount, contract), nil
}

func buildPatternUseAuditRow(
	group string,
	fixture patternUseAuditFixture,
	record fpf.PatternUseRecommendation,
) patternUseAuditRow {
	scores := scorePatternUseRecommendation(fixture, record)
	baselineScores := scorePatternUseBaseline(fixture)
	row := patternUseAuditRow{
		PromptGroup:                       group,
		PromptID:                          fixture.ID,
		Prompt:                            fixture.Prompt,
		ExpectedPrimaryPattern:            fixture.ExpectedPrimaryPattern,
		ForbiddenPrimaryPattern:           fixture.ForbiddenPrimaryPattern,
		ExpectedOutputShape:               fixture.ExpectedOutputShape,
		ExpectedBlockedStrongerUse:        fixture.ExpectedBlockedStrongerUse,
		ObservedPrimaryPattern:            record.RecommendedPatternUse.PatternRef,
		SupportLevel:                      record.SupportLevel,
		Scores:                            scores,
		BaselineScores:                    baselineScores,
		ScoreDelta:                        scores.total() - baselineScores.total(),
		AuthorityViolation:                record.HasAuthorityViolation(),
		RecommendedPatternUse:             record.RecommendedPatternUse,
		RequiredOutputShape:               record.RequiredOutputShape,
		WrongPatternBoundary:              record.WrongPatternBoundary,
		BlockedStrongerUse:                record.BlockedStrongerUse,
		CloseoutOrVerificationExpectation: record.CloseoutOrVerificationExpectation,
	}
	row.FailureReason = patternUseAuditFailureReason(row)
	row.RouteCardEditNeeded = patternUseRouteCardEditNeeded(row)
	return row
}

func scorePatternUseRecommendation(
	fixture patternUseAuditFixture,
	record fpf.PatternUseRecommendation,
) patternUseAuditScores {
	return patternUseAuditScores{
		PatternSelection:   scorePatternSelection(fixture.ExpectedPrimaryPattern, record.RecommendedPatternUse.PatternRef),
		ConcernRecovery:    scorePatternUseConcernRecovery(record),
		WrongBoundary:      scoreNonEmptyPatternUseSection(patternUseWrongBoundaryText(record), true),
		OutputShape:        scorePatternUseOutputShape(fixture.ExpectedOutputShape, record),
		EvidenceOrSoTA:     scoreNonEmptyPatternUseSection(patternUseEvidenceText(record), true),
		BlockedStrongerUse: scorePatternUseBlockedStrongerUse(fixture.ExpectedBlockedStrongerUse, record),
		ActualBehavior:     scorePatternUseActualBehavior(record),
	}
}

func scorePatternUseBaseline(fixture patternUseAuditFixture) patternUseAuditScores {
	query := normalizeAuditText(fixture.Prompt)
	score := patternUseAuditScores{}

	if strings.Contains(query, "compare") || strings.Contains(query, "which of these") {
		score.PatternSelection = 1
		score.ConcernRecovery = 1
		score.ActualBehavior = 1
	}
	if strings.Contains(query, "debug") || strings.Contains(query, "flaky") || strings.Contains(query, "failure") {
		score.PatternSelection = 1
		score.ConcernRecovery = 1
		score.ActualBehavior = 1
	}
	if strings.Contains(query, "prove") || strings.Contains(query, "evidence") || strings.Contains(query, "spec says") {
		score.EvidenceOrSoTA = 1
	}
	if strings.Contains(query, "sota") || strings.Contains(query, "current practice") {
		score.EvidenceOrSoTA = 1
	}
	return score
}

func scorePatternSelection(expected string, observed string) int {
	expected = strings.TrimSpace(expected)
	observed = strings.TrimSpace(observed)
	if expected == "" {
		if observed == "" {
			return 0
		}
		return 2
	}
	if patternUsePatternRefsMatch(expected, observed) {
		return 2
	}
	if patternUseTextOverlaps(expected, observed) {
		return 1
	}
	return 0
}

func scorePatternUseConcernRecovery(record fpf.PatternUseRecommendation) int {
	concern := strings.TrimSpace(record.ProjectConcernRef)
	if concern == "" || concern == "missing_operator_concern" {
		return 0
	}
	return 2
}

func scorePatternUseOutputShape(expected string, record fpf.PatternUseRecommendation) int {
	text := strings.Join(append([]string{record.RequiredOutputShape.CarrierKind}, record.RequiredOutputShape.RequiredSections...), " ")
	if strings.TrimSpace(expected) == "" {
		return scoreNonEmptyPatternUseSection(text, true)
	}
	if patternUseTextContainsMeaning(text, expected) {
		return 2
	}
	if patternUseTextOverlaps(text, expected) {
		return 1
	}
	return 0
}

func scorePatternUseBlockedStrongerUse(expected string, record fpf.PatternUseRecommendation) int {
	text := patternUseBlockedText(record)
	if strings.TrimSpace(expected) == "" {
		return scoreNonEmptyPatternUseSection(text, true)
	}
	if patternUseTextContainsMeaning(text, expected) {
		return 2
	}
	if patternUseTextOverlaps(text, expected) {
		return 1
	}
	return 0
}

func scorePatternUseActualBehavior(record fpf.PatternUseRecommendation) int {
	if record.SupportLevel == fpf.PatternUseSupportImplementedSubstrate {
		return 2
	}
	if record.SupportLevel == fpf.PatternUseSupportPromptLevel {
		return 1
	}
	return 0
}

func scoreNonEmptyPatternUseSection(text string, requireSpecific bool) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	if !requireSpecific {
		return 1
	}
	if len(strings.Fields(trimmed)) < 6 {
		return 1
	}
	return 2
}

func summarizePatternUseAudit(rows []patternUseAuditRow) patternUseAuditSummary {
	summary := patternUseAuditSummary{
		Prompts:             len(rows),
		MaxScore:            len(rows) * 14,
		BaselineMeasurement: "static_proxy_from_current_h_reason_phase_routing_no_llm_judge",
	}
	for _, row := range rows {
		switch row.PromptGroup {
		case "canonical":
			summary.CanonicalPrompts++
		case "held_out":
			summary.HeldOutPrompts++
		case "adversarial":
			summary.AdversarialPrompts++
		}
		summary.BaselineScore += row.BaselineScores.total()
		summary.RoutedScore += row.Scores.total()
		if row.AuthorityViolation {
			summary.AuthorityViolations++
		}
		if row.FailureReason == "" {
			summary.RowsPassing++
		} else {
			summary.RowsFailing++
		}
	}
	summary.ScoreDelta = summary.RoutedScore - summary.BaselineScore
	summary.Pass = summary.RowsFailing == 0 && summary.AuthorityViolations == 0 && summary.ScoreDelta > 0
	return summary
}

func patternUseAuditFailureReason(row patternUseAuditRow) string {
	reasons := []string{}
	if row.Scores.PatternSelection == 0 {
		reasons = append(reasons, "wrong_pattern_selection")
	}
	if strings.TrimSpace(row.ForbiddenPrimaryPattern) != "" &&
		patternUsePatternRefsMatch(row.ForbiddenPrimaryPattern, row.ObservedPrimaryPattern) {
		reasons = append(reasons, "forbidden_pattern_selected")
	}
	if row.Scores.OutputShape == 0 {
		reasons = append(reasons, "missing_required_output_shape")
	}
	if row.Scores.WrongBoundary == 0 {
		reasons = append(reasons, "missing_wrong_boundary")
	}
	if row.Scores.BlockedStrongerUse == 0 {
		reasons = append(reasons, "missing_blocked_stronger_use")
	}
	if row.AuthorityViolation {
		reasons = append(reasons, "authority_violation")
	}
	return strings.Join(reasons, ",")
}

func patternUseRouteCardEditNeeded(row patternUseAuditRow) string {
	if row.FailureReason == "" {
		return ""
	}
	return "edit_or_remove_route_card_for:" + row.FailureReason
}

func (scores patternUseAuditScores) total() int {
	return scores.PatternSelection +
		scores.ConcernRecovery +
		scores.WrongBoundary +
		scores.OutputShape +
		scores.EvidenceOrSoTA +
		scores.BlockedStrongerUse +
		scores.ActualBehavior
}

func writePatternUseRecommendationText(output interface{ Write([]byte) (int, error) }, record fpf.PatternUseRecommendation) error {
	lines := []string{
		"PatternUseRecommendation",
		fmt.Sprintf("authority: %s", record.Authority),
		fmt.Sprintf("pattern: %s - %s", record.RecommendedPatternUse.PatternRef, record.RecommendedPatternUse.Title),
		fmt.Sprintf("support: %s", record.SupportLevel),
		fmt.Sprintf("strategy: %s", record.RouteMatchStrategy),
		fmt.Sprintf("score: %.4f margin=%.4f contract=%s", record.RouteMatchScore, record.RouteMatchMargin, record.RouteMatchContract),
		fmt.Sprintf("shape: %s", record.RequiredOutputShape.CarrierKind),
		fmt.Sprintf("blocked: %s", patternUseBlockedText(record)),
	}
	_, err := fmt.Fprintln(output, strings.Join(lines, "\n"))
	return err
}

func writePatternUseCompactRecommendationText(output interface{ Write([]byte) (int, error) }, record fpf.PatternUseCompactRecommendation) error {
	lines := []string{
		"PatternUseGateway",
		fmt.Sprintf("authority: %s", record.Authority),
		fmt.Sprintf("should_use_pattern: %s", record.ShouldUsePattern),
		fmt.Sprintf("pattern: %s - %s", record.RecommendedPatternUse.PatternRef, record.RecommendedPatternUse.Title),
		fmt.Sprintf("surface: %s", record.SuggestedHaftSurface),
		fmt.Sprintf("support: %s", record.SupportLevel),
		fmt.Sprintf("strategy: %s", record.RouteMatchStrategy),
		fmt.Sprintf("boundary: %s", record.OneLineBoundary),
		fmt.Sprintf("full: %s", record.FullRecommendationCommand),
	}
	if len(record.SuggestedMethodRefs) > 0 {
		lines = append(lines, fmt.Sprintf("method_refs: %s", strings.Join(record.SuggestedMethodRefs, ", ")))
	}
	_, err := fmt.Fprintln(output, strings.Join(lines, "\n"))
	return err
}

func writePatternUseIndexText(output interface{ Write([]byte) (int, error) }, index fpf.PatternUseIndex) error {
	lines := []string{
		"PatternUseIndex",
		fmt.Sprintf("authority: %s", index.Authority),
		fmt.Sprintf("coverage: %s", index.Coverage),
		fmt.Sprintf("full_fpf_catalog_covered: %t", index.FullFPFCatalogCovered),
		fmt.Sprintf("compiled_route_cards: %d", index.CompiledRouteCardCount),
		fmt.Sprintf("semantic_route_documents: %d", index.SemanticRouteDocumentCount),
		fmt.Sprintf("semantic_route_embeddings: %d", index.SemanticRouteEmbeddingCount),
		fmt.Sprintf("semantic_route_embedding_model: %s", index.SemanticRouteEmbeddingModel),
		fmt.Sprintf("retrievable_pattern_cards: %d", index.RetrievablePatternCardCount),
	}
	_, err := fmt.Fprintln(output, strings.Join(lines, "\n"))
	return err
}

func writePatternUseAuditText(output interface{ Write([]byte) (int, error) }, report patternUseAuditReport) error {
	lines := []string{
		"PatternUse routing behavior audit v1",
		fmt.Sprintf("authority: %s", report.Authority),
		fmt.Sprintf("corpus: %s", report.CorpusRef),
		fmt.Sprintf("prompts: %d canonical=%d held_out=%d adversarial=%d", report.Summary.Prompts, report.Summary.CanonicalPrompts, report.Summary.HeldOutPrompts, report.Summary.AdversarialPrompts),
		fmt.Sprintf("score: routed=%d baseline=%d delta=%d max=%d", report.Summary.RoutedScore, report.Summary.BaselineScore, report.Summary.ScoreDelta, report.Summary.MaxScore),
		fmt.Sprintf("pass: %t authority_violations=%d failing_rows=%d", report.Summary.Pass, report.Summary.AuthorityViolations, report.Summary.RowsFailing),
	}
	_, err := fmt.Fprintln(output, strings.Join(lines, "\n"))
	return err
}

var patternUsePatternRefRE = regexp.MustCompile(`[A-Z]\.[0-9]+(?:\.[A-Z]+)?`)

func patternUsePatternRefsMatch(expected string, observed string) bool {
	expectedRefs := patternUsePatternRefs(expected)
	observedRefs := patternUsePatternRefs(observed)
	if len(expectedRefs) == 0 || len(observedRefs) == 0 {
		return normalizeAuditText(expected) == normalizeAuditText(observed)
	}
	for ref := range expectedRefs {
		if _, ok := observedRefs[ref]; ok {
			continue
		}
		return false
	}
	return true
}

func patternUsePatternRefs(text string) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, ref := range patternUsePatternRefRE.FindAllString(strings.ToUpper(text), -1) {
		refs[ref] = struct{}{}
	}
	return refs
}

func patternUseTextContainsMeaning(text string, expected string) bool {
	text = normalizeAuditText(text)
	expected = normalizeAuditText(expected)
	if text == "" || expected == "" {
		return false
	}
	return strings.Contains(text, expected)
}

func patternUseTextOverlaps(left string, right string) bool {
	leftTerms := significantPatternUseTerms(left)
	rightTerms := significantPatternUseTerms(right)
	matches := 0
	for term := range rightTerms {
		if _, ok := leftTerms[term]; ok {
			matches++
		}
	}
	return matches > 0
}

func significantPatternUseTerms(text string) map[string]struct{} {
	terms := map[string]struct{}{}
	for _, term := range strings.Fields(normalizeAuditText(text)) {
		if len(term) < 4 {
			continue
		}
		terms[term] = struct{}{}
	}
	return terms
}

func normalizeAuditText(text string) string {
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
	normalized := replacer.Replace(strings.ToLower(text))
	fields := strings.Fields(normalized)
	return strings.Join(fields, " ")
}

func patternUseWrongBoundaryText(record fpf.PatternUseRecommendation) string {
	parts := []string{}
	for _, boundary := range record.WrongPatternBoundary {
		parts = append(parts, boundary.TemptingPatternOrMove, boundary.WhyWrongNow)
	}
	return strings.Join(parts, " ")
}

func patternUseEvidenceText(record fpf.PatternUseRecommendation) string {
	parts := []string{}
	for _, evidence := range record.RequiredEvidenceOrSoTA {
		parts = append(parts, evidence.Requirement, evidence.FreshnessOrSourceRule)
	}
	return strings.Join(parts, " ")
}

func patternUseBlockedText(record fpf.PatternUseRecommendation) string {
	parts := []string{}
	for _, blocked := range record.BlockedStrongerUse {
		parts = append(parts, blocked.BlockedUse, blocked.UnblockCondition)
	}
	return strings.Join(parts, " ")
}
