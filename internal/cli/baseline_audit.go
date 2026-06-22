package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	baselineAuditKind      = "haft_baseline_term_audit"
	baselineAuditAuthority = "read_only_term_audit_not_baseline_mutation"

	baselineAuditSpecApproval     = "spec_section_approval_baseline"
	baselineAuditPreWorkReference = "pre_work_reference_snapshot"
	baselineAuditVerifiedState    = "verified_state_snapshot"
	baselineAuditComparison       = "comparison_or_benchmark_baseline"
	baselineAuditOrdinary         = "ordinary_language_baseline"
	baselineAuditLegacyAmbiguous  = "legacy_ambiguous_baseline"
)

var baselineAuditJSON bool

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Inspect baseline terminology and snapshot posture",
}

var baselineAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Classify repository baseline terminology",
	Long: `Classify repository uses of the overloaded word "baseline".

The audit is read-only. It distinguishes spec approval baselines, pre-work
reference snapshots, verified-state snapshots, comparison baselines, ordinary
language, and legacy ambiguous baseline wording. It skips Open-Sleigh, ignored
planning carriers, node_modules, vendor, and build output.`,
	RunE: runBaselineAudit,
}

func init() {
	baselineAuditCmd.Flags().BoolVar(&baselineAuditJSON, "json", false, "print the full audit as JSON")
	baselineCmd.AddCommand(baselineAuditCmd)
	rootCmd.AddCommand(baselineCmd)
}

type baselineTermAuditReport struct {
	Kind          string                      `json:"kind"`
	SchemaVersion int                         `json:"schema_version"`
	Authority     string                      `json:"authority"`
	ScanPolicy    baselineTermAuditScanPolicy `json:"scan_policy"`
	Summary       baselineTermAuditSummary    `json:"summary"`
	Findings      []baselineTermAuditFinding  `json:"findings,omitempty"`
}

type baselineTermAuditScanPolicy struct {
	Root              string   `json:"root"`
	IncludedClasses   []string `json:"included_classes"`
	ExcludedPathHints []string `json:"excluded_path_hints"`
}

type baselineTermAuditSummary struct {
	FilesScanned                int `json:"files_scanned"`
	MatchedLines                int `json:"matched_lines"`
	SpecSectionApprovalBaseline int `json:"spec_section_approval_baseline"`
	PreWorkReferenceSnapshot    int `json:"pre_work_reference_snapshot"`
	VerifiedStateSnapshot       int `json:"verified_state_snapshot"`
	ComparisonOrBenchmark       int `json:"comparison_or_benchmark_baseline"`
	OrdinaryLanguageBaseline    int `json:"ordinary_language_baseline"`
	LegacyAmbiguousBaseline     int `json:"legacy_ambiguous_baseline"`
}

type baselineTermAuditFinding struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Category  string `json:"category"`
	Snippet   string `json:"snippet"`
	Rationale string `json:"rationale"`
}

func runBaselineAudit(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	report, err := buildBaselineTermAuditReport(root)
	if err != nil {
		return err
	}
	if baselineAuditJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}

	return writeBaselineAuditText(cmd.OutOrStdout(), report)
}

func buildBaselineTermAuditReport(root string) (baselineTermAuditReport, error) {
	normalizedRoot := filepath.Clean(root)
	paths, err := baselineAuditPaths(normalizedRoot)
	if err != nil {
		return baselineTermAuditReport{}, err
	}

	report := baselineTermAuditReport{
		Kind:          baselineAuditKind,
		SchemaVersion: 1,
		Authority:     baselineAuditAuthority,
		ScanPolicy: baselineTermAuditScanPolicy{
			Root: normalizedRoot,
			IncludedClasses: []string{
				"code",
				"tests",
				"docs",
				"skills",
				"templates",
				"generated_schema_text",
				".haft_carriers",
			},
			ExcludedPathHints: baselineAuditExcludedPathHints(),
		},
	}

	for _, path := range paths {
		findings, err := scanBaselineAuditFile(normalizedRoot, path)
		if err != nil {
			return baselineTermAuditReport{}, err
		}
		report.Summary.FilesScanned++
		report.Findings = append(report.Findings, findings...)
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		left := report.Findings[i]
		right := report.Findings[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Line < right.Line
	})
	report.Summary.MatchedLines = len(report.Findings)
	for _, finding := range report.Findings {
		report.Summary.add(finding.Category)
	}

	return report, nil
}

func baselineAuditPaths(root string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if baselineAuditSkipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !baselineAuditScannableFile(rel) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func scanBaselineAuditFile(root string, path string) ([]baselineTermAuditFinding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)

	findings := []baselineTermAuditFinding{}
	reader := bufio.NewReader(file)
	lineNumber := 0
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		if readErr == io.EOF && line == "" {
			break
		}
		lineNumber++
		if !strings.Contains(strings.ToLower(line), "baseline") {
			if readErr == io.EOF {
				break
			}
			continue
		}
		category, rationale := classifyBaselineTerm(rel, line)
		findings = append(findings, baselineTermAuditFinding{
			Path:      rel,
			Line:      lineNumber,
			Category:  category,
			Snippet:   compactBaselineAuditSnippet(line),
			Rationale: rationale,
		})
		if readErr == io.EOF {
			break
		}
	}

	return findings, nil
}

func classifyBaselineTerm(path string, line string) (string, string) {
	value := strings.ToLower(path + "\n" + line)
	switch {
	case containsAnyBaselineTerm(value,
		"specsectionbaseline",
		"specsectionapprovalbaseline",
		"spec_section_approval_baseline",
		"baselinkindspecsectionapproval",
		"spec section approval baseline",
		"spec approval baseline",
	):
		return baselineAuditSpecApproval, "names a SpecSection approval baseline or its typed profile"
	case containsAnyBaselineTerm(value,
		"pre_work_reference_snapshot",
		"baselinekindpreworkreference",
		"pre-work reference",
		"pre work reference",
	):
		return baselineAuditPreWorkReference, "names a pre-work reference snapshot"
	case containsAnyBaselineTerm(value,
		"verified_state_snapshot",
		"baselinekindverifiedstate",
		"baselinekindverifiedstatesnapshot",
		"verified-state snapshot",
		"verified state snapshot",
		"drift_detection_snapshot",
	):
		return baselineAuditVerifiedState, "names a verified-state snapshot or drift-detection baseline profile"
	case containsAnyBaselineTerm(value,
		"baseline_set",
		"baseline set",
		"comparison baseline",
		"benchmark baseline",
		"deterministic comparison harness",
		"beat baseline",
		"against baseline",
		"simpler baseline",
	):
		return baselineAuditComparison, "uses baseline as a comparison or benchmark reference"
	case containsAnyBaselineTerm(value,
		"unknown_legacy_baseline",
		"baselinekindunknownlegacy",
		"legacy ambiguous baseline",
		"legacy/unknown",
	):
		return baselineAuditLegacyAmbiguous, "explicitly names legacy ambiguous baseline posture"
	case containsAnyBaselineTerm(value,
		"normalization baseline",
		"baseline db",
		"baseline test",
		"baseline fixture",
	):
		return baselineAuditOrdinary, "uses baseline as ordinary test or fixture wording"
	default:
		return baselineAuditLegacyAmbiguous, "overloaded baseline term needs explicit classification"
	}
}

func containsAnyBaselineTerm(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func baselineAuditSkipDir(rel string) bool {
	rel = filepath.ToSlash(rel)
	switch rel {
	case ".", "":
		return false
	}
	if strings.HasPrefix(rel, ".") && !baselineAuditAllowedHiddenDir(rel) {
		return true
	}
	for _, hint := range baselineAuditExcludedPathHints() {
		if rel == hint || strings.HasPrefix(rel, hint+"/") {
			return true
		}
	}
	return false
}

func baselineAuditAllowedHiddenDir(rel string) bool {
	return rel == ".agents" ||
		strings.HasPrefix(rel, ".agents/") ||
		rel == ".haft" ||
		strings.HasPrefix(rel, ".haft/")
}

func baselineAuditExcludedPathHints() []string {
	return []string{
		".git",
		".context",
		"hidden directories except .agents and .haft",
		"desktop/frontend/node_modules",
		"node_modules",
		"open-sleigh",
		"vendor",
		"dist",
		"build",
		"tmp",
	}
}

func baselineAuditScannableFile(rel string) bool {
	if strings.Contains(rel, "/.") && !strings.HasPrefix(rel, ".agents/") && !strings.HasPrefix(rel, ".haft/") {
		return false
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".md", ".yaml", ".yml", ".json", ".toml", ".txt", ".tmpl", ".tpl", ".sh":
		return true
	default:
		return rel == "AGENTS.md" || rel == "CHANGELOG.md"
	}
}

func compactBaselineAuditSnippet(line string) string {
	snippet := strings.Join(strings.Fields(line), " ")
	if len(snippet) <= 180 {
		return snippet
	}
	return snippet[:177] + "..."
}

func (summary *baselineTermAuditSummary) add(category string) {
	switch category {
	case baselineAuditSpecApproval:
		summary.SpecSectionApprovalBaseline++
	case baselineAuditPreWorkReference:
		summary.PreWorkReferenceSnapshot++
	case baselineAuditVerifiedState:
		summary.VerifiedStateSnapshot++
	case baselineAuditComparison:
		summary.ComparisonOrBenchmark++
	case baselineAuditOrdinary:
		summary.OrdinaryLanguageBaseline++
	case baselineAuditLegacyAmbiguous:
		summary.LegacyAmbiguousBaseline++
	}
}

func writeBaselineAuditText(w io.Writer, report baselineTermAuditReport) error {
	if _, err := fmt.Fprintf(w, "Haft baseline term audit v%d\n", report.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "authority: %s\n", report.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"summary: files=%d matched=%d spec_approval=%d pre_work=%d verified_state=%d comparison=%d ordinary=%d legacy_ambiguous=%d\n",
		report.Summary.FilesScanned,
		report.Summary.MatchedLines,
		report.Summary.SpecSectionApprovalBaseline,
		report.Summary.PreWorkReferenceSnapshot,
		report.Summary.VerifiedStateSnapshot,
		report.Summary.ComparisonOrBenchmark,
		report.Summary.OrdinaryLanguageBaseline,
		report.Summary.LegacyAmbiguousBaseline,
	); err != nil {
		return err
	}

	if report.Summary.LegacyAmbiguousBaseline == 0 {
		_, err := fmt.Fprintln(w, "legacy_ambiguous: none")
		return err
	}

	if _, err := fmt.Fprintln(w, "legacy_ambiguous:"); err != nil {
		return err
	}
	written := 0
	for _, finding := range report.Findings {
		if finding.Category != baselineAuditLegacyAmbiguous {
			continue
		}
		if _, err := fmt.Fprintf(w, "- %s:%d %s\n", finding.Path, finding.Line, finding.Snippet); err != nil {
			return err
		}
		written++
		if written >= 20 {
			break
		}
	}
	if report.Summary.LegacyAmbiguousBaseline > written {
		_, err := fmt.Fprintf(w, "... and %d more; rerun with --json\n", report.Summary.LegacyAmbiguousBaseline-written)
		return err
	}

	return nil
}
