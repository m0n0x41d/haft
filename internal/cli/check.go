package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

type checkReport struct {
	Stale        []checkStaleFinding       `json:"stale"`
	Drifted      []checkDriftFinding       `json:"drifted"`
	Unassessed   []checkDecisionFinding    `json:"unassessed"`
	CoverageGaps []checkCoverageGapFinding `json:"coverage_gaps"`
	Summary      checkSummary              `json:"summary"`
}

type checkSummary struct {
	TotalFindings int `json:"total_findings"`
}

type checkStaleFinding struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Kind       string  `json:"kind"`
	Category   string  `json:"category"`
	Reason     string  `json:"reason"`
	ValidUntil string  `json:"valid_until,omitempty"`
	DaysStale  int     `json:"days_stale,omitempty"`
	REff       float64 `json:"r_eff,omitempty"`
}

type checkDriftFinding struct {
	DecisionID        string               `json:"decision_id"`
	DecisionTitle     string               `json:"decision_title"`
	HasBaseline       bool                 `json:"has_baseline"`
	LikelyImplemented bool                 `json:"likely_implemented,omitempty"`
	Summary           string               `json:"summary"`
	Files             []artifact.DriftItem `json:"files,omitempty"`
}

type checkDecisionFinding struct {
	DecisionID string `json:"decision_id"`
	Title      string `json:"title"`
	Context    string `json:"context,omitempty"`
}

type checkCoverageGapFinding struct {
	DecisionID string   `json:"decision_id"`
	Title      string   `json:"title"`
	Gaps       []string `json:"gaps"`
}

var (
	checkJSON bool
	checkExit = os.Exit
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check governance debt for CI",
	Long: `Run governance checks for stale artifacts, drift, unassessed decisions, and coverage gaps.

Exit code 0 means clean.
Exit code 1 means findings were detected.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "print structured JSON output")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}

	store := artifact.NewStore(sqlDB)
	report, err := buildCheckReport(context.Background(), store, projectRoot)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}

	output := cmd.OutOrStdout()
	if checkJSON {
		err = writeCheckJSON(output, report)
	} else {
		err = writeCheckSummary(output, report)
	}
	if err != nil {
		_ = sqlDB.Close()
		return err
	}

	if closeErr := sqlDB.Close(); closeErr != nil {
		return fmt.Errorf("close DB: %w", closeErr)
	}

	if report.hasFindings() {
		checkExit(1)
	}

	return nil
}

func buildCheckReport(ctx context.Context, store *artifact.Store, projectRoot string) (checkReport, error) {
	report := checkReport{}

	staleItems, err := artifact.ScanStale(ctx, store)
	if err != nil {
		return report, fmt.Errorf("scan stale artifacts: %w", err)
	}

	driftReports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return report, fmt.Errorf("scan drift: %w", err)
	}

	decisions, err := store.ListByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return report, fmt.Errorf("list decisions: %w", err)
	}

	activeDecisions := filterCheckActiveDecisions(decisions)

	report.Stale = mapCheckStaleFindings(staleItems)
	report.Drifted = mapCheckDriftFindings(driftReports)
	report.Unassessed = collectUnassessedFindings(ctx, store, activeDecisions)
	report.CoverageGaps = collectCoverageGapFindings(ctx, store, activeDecisions)
	report.Summary = checkSummary{
		TotalFindings: len(report.Stale) + len(report.Drifted) + len(report.Unassessed) + len(report.CoverageGaps),
	}

	return report, nil
}

func filterCheckActiveDecisions(decisions []*artifact.Artifact) []*artifact.Artifact {
	active := make([]*artifact.Artifact, 0, len(decisions))

	for _, decision := range decisions {
		if decision.Meta.Status != artifact.StatusActive {
			continue
		}

		active = append(active, decision)
	}

	return active
}

func mapCheckStaleFindings(items []artifact.StaleItem) []checkStaleFinding {
	findings := make([]checkStaleFinding, 0, len(items))

	for _, item := range items {
		findings = append(findings, checkStaleFinding{
			ID:         item.ID,
			Title:      item.Title,
			Kind:       item.Kind,
			Category:   string(item.Category),
			Reason:     item.Reason,
			ValidUntil: item.ValidUntil,
			DaysStale:  item.DaysStale,
			REff:       item.REff,
		})
	}

	return findings
}

func mapCheckDriftFindings(reports []artifact.DriftReport) []checkDriftFinding {
	findings := make([]checkDriftFinding, 0, len(reports))

	for _, report := range reports {
		findings = append(findings, checkDriftFinding{
			DecisionID:        report.DecisionID,
			DecisionTitle:     report.DecisionTitle,
			HasBaseline:       report.HasBaseline,
			LikelyImplemented: report.LikelyImplemented,
			Summary:           summarizeCheckDrift(report),
			Files:             report.Files,
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DecisionID < findings[j].DecisionID
	})

	return findings
}

func collectUnassessedFindings(
	ctx context.Context,
	store *artifact.Store,
	decisions []*artifact.Artifact,
) []checkDecisionFinding {
	findings := make([]checkDecisionFinding, 0, len(decisions))

	for _, decision := range decisions {
		health := artifact.DeriveDecisionHealth(ctx, store, decision.Meta.ID)
		if health.Maturity != artifact.DecisionMaturityUnassessed {
			continue
		}

		findings = append(findings, checkDecisionFinding{
			DecisionID: decision.Meta.ID,
			Title:      decision.Meta.Title,
			Context:    decision.Meta.Context,
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DecisionID < findings[j].DecisionID
	})

	return findings
}

func collectCoverageGapFindings(
	ctx context.Context,
	store *artifact.Store,
	decisions []*artifact.Artifact,
) []checkCoverageGapFinding {
	findings := make([]checkCoverageGapFinding, 0, len(decisions))

	for _, decision := range decisions {
		wlnk := artifact.ComputeWLNKSummary(ctx, store, decision.Meta.ID)
		if len(wlnk.CoverageGaps) == 0 {
			continue
		}

		findings = append(findings, checkCoverageGapFinding{
			DecisionID: decision.Meta.ID,
			Title:      decision.Meta.Title,
			Gaps:       append([]string(nil), wlnk.CoverageGaps...),
		})
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DecisionID < findings[j].DecisionID
	})

	return findings
}

func summarizeCheckDrift(report artifact.DriftReport) string {
	if !report.HasBaseline {
		summary := fmt.Sprintf("no baseline — %d file(s) unmonitored", len(report.Files))
		if report.LikelyImplemented {
			summary += " — git activity detected after decision date"
		}
		return summary
	}

	modified := 0
	added := 0
	missing := 0
	unbaselined := 0

	for _, file := range report.Files {
		switch file.Status {
		case artifact.DriftModified:
			modified++
		case artifact.DriftAdded:
			added++
		case artifact.DriftMissing:
			missing++
		case artifact.DriftNoBaseline:
			unbaselined++
		}
	}

	parts := []string{
		fmt.Sprintf("%d modified", modified),
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if unbaselined > 0 {
		parts = append(parts, fmt.Sprintf("%d unbaselined", unbaselined))
	}

	return "code drift — " + strings.Join(parts, ", ")
}

func writeCheckJSON(w io.Writer, report checkReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func writeCheckSummary(w io.Writer, report checkReport) error {
	var sb strings.Builder

	if report.hasFindings() {
		sb.WriteString(fmt.Sprintf("haft check: governance debt found (%d finding(s))\n", report.Summary.TotalFindings))
	} else {
		sb.WriteString("haft check: clean\n")
	}

	sb.WriteString(fmt.Sprintf("stale: %d\n", len(report.Stale)))
	sb.WriteString(fmt.Sprintf("drifted: %d\n", len(report.Drifted)))
	sb.WriteString(fmt.Sprintf("unassessed: %d\n", len(report.Unassessed)))
	sb.WriteString(fmt.Sprintf("coverage gaps: %d\n", len(report.CoverageGaps)))

	appendStaleSection(&sb, report.Stale)
	appendDriftSection(&sb, report.Drifted)
	appendUnassessedSection(&sb, report.Unassessed)
	appendCoverageSection(&sb, report.CoverageGaps)

	_, err := io.WriteString(w, sb.String())
	return err
}

func appendStaleSection(sb *strings.Builder, findings []checkStaleFinding) {
	if len(findings) == 0 {
		return
	}

	sb.WriteString("\nStale\n")
	for _, finding := range findings {
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", finding.Title, finding.ID, finding.Reason))
	}
}

func appendDriftSection(sb *strings.Builder, findings []checkDriftFinding) {
	if len(findings) == 0 {
		return
	}

	sb.WriteString("\nDrift\n")
	for _, finding := range findings {
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", finding.DecisionTitle, finding.DecisionID, finding.Summary))
	}
}

func appendUnassessedSection(sb *strings.Builder, findings []checkDecisionFinding) {
	if len(findings) == 0 {
		return
	}

	sb.WriteString("\nUnassessed\n")
	for _, finding := range findings {
		sb.WriteString(fmt.Sprintf("- %s [%s]\n", finding.Title, finding.DecisionID))
	}
}

func appendCoverageSection(sb *strings.Builder, findings []checkCoverageGapFinding) {
	if len(findings) == 0 {
		return
	}

	sb.WriteString("\nCoverage Gaps\n")
	for _, finding := range findings {
		sb.WriteString(fmt.Sprintf("- %s [%s]: %s\n", finding.Title, finding.DecisionID, strings.Join(finding.Gaps, ", ")))
	}
}

func (report checkReport) hasFindings() bool {
	return report.Summary.TotalFindings > 0
}
