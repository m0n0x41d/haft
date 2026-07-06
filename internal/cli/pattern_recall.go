package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	patternRecallAuditKind      = "pattern_recall_source_card_audit"
	patternRecallAuditAuthority = "read_only_pattern_recall_audit_not_enforcement_gate"
)

var patternRecallMutationBoundary = []string{
	"read_only_source_card_recall",
	"does_not_mutate_method_runs_decisions_evidence_or_carriers",
	"does_not_create_decision_workcommission_gate_evidence_claim_truth_global_truth_or_publication",
}

var (
	patternRecallJSON       bool
	patternRecallMode       string
	patternRecallLimit      int
	patternRecallAuditInput string
	patternRecallAuditJSON  bool
)

var patternCmd = &cobra.Command{
	Use:   "pattern",
	Short: "Recall and inspect FPF pattern source cards",
}

var patternRecallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Return read-only FPF source-card recall candidates",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runPatternRecall,
}

var patternRecallAuditCmd = &cobra.Command{
	Use:   "audit --input FILE",
	Short: "Audit PatternRecall source-card retrieval behavior over a prompt corpus",
	RunE:  runPatternRecallAudit,
}

func init() {
	patternRecallCmd.Flags().BoolVar(&patternRecallJSON, "json", false, "print the recall record as JSON")
	patternRecallCmd.Flags().StringVar(&patternRecallMode, "mode", fpf.PatternRecallCompactMode, "recall mode: compact or full")
	patternRecallCmd.Flags().IntVar(&patternRecallLimit, "limit", fpf.PatternRecallDefaultLimit, "maximum source-card candidates")
	patternRecallAuditCmd.Flags().StringVar(&patternRecallAuditInput, "input", "", "YAML prompt corpus")
	patternRecallAuditCmd.Flags().BoolVar(&patternRecallAuditJSON, "json", false, "print the full audit as JSON")
	patternRecallCmd.AddCommand(patternRecallAuditCmd)
	patternCmd.AddCommand(patternRecallCmd)
	rootCmd.AddCommand(patternCmd)
}

type patternRecallAuditCorpus struct {
	SchemaVersion    int                         `yaml:"schema_version"`
	Purpose          string                      `yaml:"purpose"`
	Status           string                      `yaml:"status"`
	CanonicalPrompts []patternRecallAuditFixture `yaml:"canonical_prompts"`
	HeldOutPrompts   []patternRecallAuditFixture `yaml:"held_out_prompts"`
	Adversarial      []patternRecallAuditFixture `yaml:"adversarial_prompts"`
}

type patternRecallAuditFixture struct {
	ID                   string `yaml:"id" json:"id"`
	Prompt               string `yaml:"prompt" json:"prompt"`
	Mode                 string `yaml:"mode,omitempty" json:"mode,omitempty"`
	ExpectedPatternID    string `yaml:"expected_pattern_id,omitempty" json:"expected_pattern_id,omitempty"`
	ForbiddenPatternID   string `yaml:"forbidden_pattern_id,omitempty" json:"forbidden_pattern_id,omitempty"`
	ExpectedSupportLevel string `yaml:"expected_support_level,omitempty" json:"expected_support_level,omitempty"`
	ExpectSourceBody     bool   `yaml:"expect_source_body,omitempty" json:"expect_source_body,omitempty"`
	ExpectNoCandidates   bool   `yaml:"expect_no_candidates,omitempty" json:"expect_no_candidates,omitempty"`
}

type patternRecallAuditReport struct {
	SchemaVersion    int                       `json:"schema_version"`
	AuditKind        string                    `json:"audit_kind"`
	Authority        string                    `json:"authority"`
	MutationBoundary []string                  `json:"mutation_boundary"`
	RunAt            string                    `json:"run_at"`
	CorpusRef        string                    `json:"corpus_ref"`
	Summary          patternRecallAuditSummary `json:"summary"`
	Rows             []patternRecallAuditRow   `json:"rows"`
}

type patternRecallAuditSummary struct {
	Prompts             int  `json:"prompts"`
	CanonicalPrompts    int  `json:"canonical_prompts"`
	HeldOutPrompts      int  `json:"held_out_prompts"`
	AdversarialPrompts  int  `json:"adversarial_prompts"`
	RowsPassing         int  `json:"rows_passing"`
	RowsFailing         int  `json:"rows_failing"`
	AuthorityViolations int  `json:"authority_violations"`
	Pass                bool `json:"pass"`
}

type patternRecallAuditRow struct {
	PromptGroup          string   `json:"prompt_group"`
	PromptID             string   `json:"prompt_id"`
	Prompt               string   `json:"prompt"`
	Mode                 string   `json:"mode"`
	ExpectedPatternID    string   `json:"expected_pattern_id,omitempty"`
	ForbiddenPatternID   string   `json:"forbidden_pattern_id,omitempty"`
	ExpectedSupportLevel string   `json:"expected_support_level,omitempty"`
	ExpectSourceBody     bool     `json:"expect_source_body,omitempty"`
	ExpectNoCandidates   bool     `json:"expect_no_candidates,omitempty"`
	ObservedPatternIDs   []string `json:"observed_pattern_ids"`
	ObservedSupportLevel string   `json:"observed_support_level"`
	ObservedSourceBodies int      `json:"observed_source_bodies"`
	AuthorityViolation   bool     `json:"authority_violation"`
	Pass                 bool     `json:"pass"`
	FailureReason        string   `json:"failure_reason,omitempty"`
	FullRecallCommand    string   `json:"full_recall_command,omitempty"`
	OneLineBoundary      string   `json:"one_line_boundary"`
}

func runPatternRecall(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	record, err := patternRecallWithEmbeddedSourceCards(fpf.PatternRecallRequest{
		Query: query,
		Mode:  patternRecallMode,
		Limit: patternRecallLimit,
	})
	if err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	if patternRecallJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}
	return writePatternRecallText(cmd.OutOrStdout(), record)
}

func runPatternRecallAudit(cmd *cobra.Command, _ []string) error {
	if strings.TrimSpace(patternRecallAuditInput) == "" {
		return fmt.Errorf("--input is required")
	}

	report, err := buildPatternRecallAuditReport(patternRecallAuditInput, timeNow())
	if err != nil {
		return err
	}
	if patternRecallAuditJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writePatternRecallAuditText(cmd.OutOrStdout(), report)
}

func patternRecallWithEmbeddedSourceCards(
	request fpf.PatternRecallRequest,
) (fpf.PatternRecallResult, error) {
	mode, err := fpf.NormalizePatternRecallMode(request.Mode)
	if err != nil {
		return fpf.PatternRecallResult{}, err
	}
	request.Mode = mode

	if !fpf.ShouldAttemptPatternRecall(request.Query) {
		return fpf.PatternRecallFromRetrievedCandidates(request, nil), nil
	}

	limit := request.Limit
	if limit <= 0 {
		limit = fpf.PatternRecallDefaultLimit
	}
	exactCandidates := patternRecallExactCandidatesWithEmbeddedAtlas(request.Query, limit)
	retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
		Query: request.Query,
		Limit: limit * 3,
	})
	if err != nil {
		return fpf.PatternRecallResult{}, fmt.Errorf("retrieve FPF pattern source cards: %w", err)
	}

	candidates := fpf.PatternUseRetrievedCandidatesFromSpecResults(retrieval.Results, limit)
	candidates = hydratePatternUseRetrievedCandidatesWithEmbeddedAtlas(candidates)
	candidates = append(exactCandidates, candidates...)
	return fpf.PatternRecallFromRetrievedCandidates(request, candidates), nil
}

func patternRecallExactCandidatesWithEmbeddedAtlas(
	query string,
	limit int,
) []fpf.PatternUseRetrievedCandidate {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return nil
	}
	defer cleanup()

	return fpf.PatternRecallExactCandidatesFromAtlas(db, query, limit)
}

func buildPatternRecallAuditReport(
	inputPath string,
	now time.Time,
) (patternRecallAuditReport, error) {
	corpus, err := readPatternRecallAuditCorpus(inputPath)
	if err != nil {
		return patternRecallAuditReport{}, err
	}

	report := patternRecallAuditReport{
		SchemaVersion:    1,
		AuditKind:        patternRecallAuditKind,
		Authority:        patternRecallAuditAuthority,
		MutationBoundary: append([]string(nil), patternRecallMutationBoundary...),
		RunAt:            now.UTC().Format(time.RFC3339),
		CorpusRef:        inputPath,
	}

	rows, err := buildPatternRecallAuditRows("canonical", corpus.CanonicalPrompts)
	if err != nil {
		return patternRecallAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	rows, err = buildPatternRecallAuditRows("held_out", corpus.HeldOutPrompts)
	if err != nil {
		return patternRecallAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	rows, err = buildPatternRecallAuditRows("adversarial", corpus.Adversarial)
	if err != nil {
		return patternRecallAuditReport{}, err
	}
	report.Rows = append(report.Rows, rows...)
	report.Summary = summarizePatternRecallAudit(report.Rows)
	return report, nil
}

func readPatternRecallAuditCorpus(inputPath string) (patternRecallAuditCorpus, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return patternRecallAuditCorpus{}, err
	}

	var corpus patternRecallAuditCorpus
	if err := yaml.Unmarshal(data, &corpus); err != nil {
		return patternRecallAuditCorpus{}, err
	}
	if corpus.SchemaVersion != 1 {
		return patternRecallAuditCorpus{}, fmt.Errorf("unsupported pattern-recall audit corpus schema_version %d", corpus.SchemaVersion)
	}
	return corpus, nil
}

func buildPatternRecallAuditRows(
	group string,
	fixtures []patternRecallAuditFixture,
) ([]patternRecallAuditRow, error) {
	rows := make([]patternRecallAuditRow, 0, len(fixtures))
	for _, fixture := range fixtures {
		record, err := patternRecallWithEmbeddedSourceCards(fpf.PatternRecallRequest{
			Query: fixture.Prompt,
			Mode:  fixture.Mode,
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, buildPatternRecallAuditRow(group, fixture, record))
	}
	return rows, nil
}

func buildPatternRecallAuditRow(
	group string,
	fixture patternRecallAuditFixture,
	record fpf.PatternRecallResult,
) patternRecallAuditRow {
	row := patternRecallAuditRow{
		PromptGroup:          group,
		PromptID:             fixture.ID,
		Prompt:               fixture.Prompt,
		Mode:                 record.Mode,
		ExpectedPatternID:    fixture.ExpectedPatternID,
		ForbiddenPatternID:   fixture.ForbiddenPatternID,
		ExpectedSupportLevel: fixture.ExpectedSupportLevel,
		ExpectSourceBody:     fixture.ExpectSourceBody,
		ExpectNoCandidates:   fixture.ExpectNoCandidates,
		ObservedPatternIDs:   patternRecallObservedPatternIDs(record),
		ObservedSupportLevel: record.SupportLevel,
		ObservedSourceBodies: patternRecallObservedSourceBodies(record),
		AuthorityViolation:   record.HasAuthorityViolation(),
		FullRecallCommand:    record.FullRecallCommand,
		OneLineBoundary:      record.OneLineBoundary,
	}

	failures := patternRecallAuditFailures(fixture, record, row)
	row.Pass = len(failures) == 0
	row.FailureReason = strings.Join(failures, "; ")
	return row
}

func summarizePatternRecallAudit(rows []patternRecallAuditRow) patternRecallAuditSummary {
	summary := patternRecallAuditSummary{Prompts: len(rows)}
	for _, row := range rows {
		switch row.PromptGroup {
		case "canonical":
			summary.CanonicalPrompts++
		case "held_out":
			summary.HeldOutPrompts++
		case "adversarial":
			summary.AdversarialPrompts++
		}
		if row.Pass {
			summary.RowsPassing++
		} else {
			summary.RowsFailing++
		}
		if row.AuthorityViolation {
			summary.AuthorityViolations++
		}
	}
	summary.Pass = summary.RowsFailing == 0 && summary.AuthorityViolations == 0
	return summary
}

func patternRecallAuditFailures(
	fixture patternRecallAuditFixture,
	record fpf.PatternRecallResult,
	row patternRecallAuditRow,
) []string {
	var failures []string
	if err := record.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if fixture.ExpectedSupportLevel != "" && record.SupportLevel != fixture.ExpectedSupportLevel {
		failures = append(failures, fmt.Sprintf("support_level=%q want %q", record.SupportLevel, fixture.ExpectedSupportLevel))
	}
	if fixture.ExpectedPatternID != "" && !stringSliceContains(row.ObservedPatternIDs, fixture.ExpectedPatternID) {
		failures = append(failures, fmt.Sprintf("missing expected pattern_id %q", fixture.ExpectedPatternID))
	}
	if fixture.ForbiddenPatternID != "" && stringSliceContains(row.ObservedPatternIDs, fixture.ForbiddenPatternID) {
		failures = append(failures, fmt.Sprintf("forbidden pattern_id %q observed", fixture.ForbiddenPatternID))
	}
	if fixture.ExpectSourceBody && row.ObservedSourceBodies == 0 {
		failures = append(failures, "expected source_card body")
	}
	if fixture.ExpectNoCandidates && len(row.ObservedPatternIDs) > 0 {
		failures = append(failures, "expected no candidates")
	}
	if row.AuthorityViolation {
		failures = append(failures, "authority overclaim")
	}
	return failures
}

func patternRecallObservedPatternIDs(record fpf.PatternRecallResult) []string {
	out := make([]string, 0, len(record.CandidateSourceCards))
	for _, candidate := range record.CandidateSourceCards {
		out = append(out, candidate.PatternID)
	}
	return out
}

func patternRecallObservedSourceBodies(record fpf.PatternRecallResult) int {
	count := 0
	for _, candidate := range record.CandidateSourceCards {
		if candidate.SourceCard == nil {
			continue
		}
		if strings.TrimSpace(candidate.SourceCard.Body) == "" {
			continue
		}
		count++
	}
	return count
}

func writePatternRecallText(w io.Writer, record fpf.PatternRecallResult) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}
	if err := printf("PatternRecall: support=%s mode=%s\n", record.SupportLevel, record.Mode); err != nil {
		return err
	}
	if err := printf("Boundary: %s\n", record.OneLineBoundary); err != nil {
		return err
	}
	if record.FullRecallCommand != "" {
		if err := printf("Full: %s\n", record.FullRecallCommand); err != nil {
			return err
		}
	}
	for _, candidate := range record.CandidateSourceCards {
		if err := printf("- %s %s [%s]\n", candidate.PatternID, candidate.Title, candidate.SourceTier); err != nil {
			return err
		}
		if candidate.SourceRefShort != "" {
			if err := printf("  source: %s\n", candidate.SourceRefShort); err != nil {
				return err
			}
		}
		if candidate.SourceCard != nil && candidate.SourceCard.Body != "" {
			if err := printf("  body: %s\n", candidate.SourceCard.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

func writePatternRecallAuditText(w io.Writer, report patternRecallAuditReport) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}
	if err := printf("PatternRecall audit: pass=%t prompts=%d rows_passing=%d rows_failing=%d authority_violations=%d\n",
		report.Summary.Pass,
		report.Summary.Prompts,
		report.Summary.RowsPassing,
		report.Summary.RowsFailing,
		report.Summary.AuthorityViolations,
	); err != nil {
		return err
	}
	for _, row := range report.Rows {
		if row.Pass {
			continue
		}
		if err := printf("- %s/%s failed: %s\n", row.PromptGroup, row.PromptID, row.FailureReason); err != nil {
			return err
		}
	}
	return nil
}
