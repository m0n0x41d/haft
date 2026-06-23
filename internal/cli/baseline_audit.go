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
	baselineAuditHistoricalGov    = "historical_governance_carrier_baseline"
	baselineAuditSupportArchive   = "support_archive_carrier_baseline"
	baselineAuditSourceSpec       = "source_spec_reference_baseline"
	baselineAuditTypedModel       = "typed_baseline_model"
	baselineAuditLifecycleAuth    = "baseline_lifecycle_authority"
	baselineAuditReleaseNotes     = "release_notes_carrier_baseline"
	baselineAuditToolSurface      = "baseline_audit_tool_surface"
	baselineAuditTestFixture      = "baseline_test_fixture_surface"
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
	Kind          string                        `json:"kind"`
	SchemaVersion int                           `json:"schema_version"`
	Authority     string                        `json:"authority"`
	ScanPolicy    baselineTermAuditScanPolicy   `json:"scan_policy"`
	Summary       baselineTermAuditSummary      `json:"summary"`
	Diagnostics   []baselineTermAuditDiagnostic `json:"diagnostics,omitempty"`
	Findings      []baselineTermAuditFinding    `json:"findings,omitempty"`
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
	HistoricalGovernanceCarrier int `json:"historical_governance_carrier_baseline"`
	HistoricalGovernanceFiles   int `json:"historical_governance_carrier_files"`
	SupportArchiveCarrier       int `json:"support_archive_carrier_baseline"`
	SupportArchiveFiles         int `json:"support_archive_carrier_files"`
	SourceSpecReference         int `json:"source_spec_reference_baseline"`
	SourceSpecFiles             int `json:"source_spec_reference_files"`
	TypedBaselineModel          int `json:"typed_baseline_model"`
	TypedBaselineModelFiles     int `json:"typed_baseline_model_files"`
	LifecycleAuthority          int `json:"baseline_lifecycle_authority"`
	LifecycleAuthorityFiles     int `json:"baseline_lifecycle_authority_files"`
	ReleaseNotesCarrier         int `json:"release_notes_carrier_baseline"`
	ReleaseNotesFiles           int `json:"release_notes_carrier_files"`
	AuditToolSurface            int `json:"baseline_audit_tool_surface"`
	AuditToolSurfaceFiles       int `json:"baseline_audit_tool_surface_files"`
	TestFixtureSurface          int `json:"baseline_test_fixture_surface"`
	TestFixtureSurfaceFiles     int `json:"baseline_test_fixture_surface_files"`
	LegacyAmbiguousBaseline     int `json:"legacy_ambiguous_baseline"`
	LegacyAmbiguousFiles        int `json:"legacy_ambiguous_files"`
}

type baselineTermAuditDiagnostic struct {
	Level      string   `json:"level"`
	Code       string   `json:"code"`
	Category   string   `json:"category"`
	Count      int      `json:"count"`
	Files      int      `json:"files"`
	Message    string   `json:"message"`
	NextAction string   `json:"next_action"`
	Examples   []string `json:"examples,omitempty"`
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
	report.Summary.HistoricalGovernanceFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditHistoricalGov)
	report.Summary.SupportArchiveFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditSupportArchive)
	report.Summary.SourceSpecFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditSourceSpec)
	report.Summary.TypedBaselineModelFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditTypedModel)
	report.Summary.LifecycleAuthorityFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditLifecycleAuth)
	report.Summary.ReleaseNotesFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditReleaseNotes)
	report.Summary.AuditToolSurfaceFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditToolSurface)
	report.Summary.TestFixtureSurfaceFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditTestFixture)
	report.Summary.LegacyAmbiguousFiles = baselineAuditCategoryFiles(report.Findings, baselineAuditLegacyAmbiguous)
	report.Diagnostics = baselineAuditDiagnostics(report.Findings, report.Summary)

	return report, nil
}

func baselineAuditDiagnostics(
	findings []baselineTermAuditFinding,
	summary baselineTermAuditSummary,
) []baselineTermAuditDiagnostic {
	if summary.LegacyAmbiguousBaseline == 0 {
		return nil
	}
	return []baselineTermAuditDiagnostic{{
		Level:    "warn",
		Code:     "legacy_ambiguous_baseline_terms",
		Category: baselineAuditLegacyAmbiguous,
		Count:    summary.LegacyAmbiguousBaseline,
		Files:    summary.LegacyAmbiguousFiles,
		Message:  "baseline wording remains overloaded and is not typed as spec approval, pre-work reference, verified-state snapshot, comparison, or ordinary fixture wording",
		NextAction: strings.Join([]string{
			"rename the usage to a typed baseline concept when it writes or reads state",
			"or add local wording that classifies it as comparison/fixture/ordinary language",
			"do not treat this audit finding as permission to rewrite baselines",
		}, "; "),
		Examples: baselineAuditDiagnosticExamples(findings, baselineAuditLegacyAmbiguous, 5),
	}}
}

func baselineAuditCategoryFiles(findings []baselineTermAuditFinding, category string) int {
	files := map[string]struct{}{}
	for _, finding := range findings {
		if finding.Category != category {
			continue
		}
		files[finding.Path] = struct{}{}
	}
	return len(files)
}

func baselineAuditDiagnosticExamples(
	findings []baselineTermAuditFinding,
	category string,
	limit int,
) []string {
	examples := []string{}
	for _, finding := range findings {
		if finding.Category != category {
			continue
		}
		examples = append(examples, fmt.Sprintf("%s:%d", finding.Path, finding.Line))
		if len(examples) >= limit {
			break
		}
	}
	return examples
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
	case baselineAuditHistoricalGovernanceCarrier(path):
		return baselineAuditHistoricalGov, "mentions baseline inside a historical governance carrier; audit-visible but not current terminology debt"
	case baselineAuditSupportArchiveCarrier(path):
		return baselineAuditSupportArchive, "mentions baseline inside a support or archive carrier; audit-visible but not current terminology debt"
	case baselineAuditSourceSpecCarrier(path):
		return baselineAuditSourceSpec, "mentions baseline inside an upstream source specification carrier; audit-visible but not current Haft terminology debt"
	case baselineAuditReleaseNotesCarrier(path):
		return baselineAuditReleaseNotes, "mentions baseline inside release notes; audit-visible provenance, not current terminology debt"
	case baselineAuditToolSurfaceCarrier(path):
		return baselineAuditToolSurface, "mentions baseline inside the baseline audit tool implementation; audit-visible self surface, not terminology debt"
	case baselineAuditTestFixtureSurface(path, value):
		return baselineAuditTestFixture, "mentions baseline inside test helper or fixture vocabulary; audit-visible test surface, not product terminology debt"
	case containsAnyBaselineTerm(value,
		"specsectionbaseline",
		"specsectionapprovalbaseline",
		"spec_section_approval_baseline",
		"baselinkindspecsectionapproval",
		"spec section approval baseline",
		"spec approval baseline",
		"spec_lifecycle_approval_baseline",
		"putspecsectionapproval",
		"getspecsectionapproval",
		"listspecsectionapproval",
		"projectbaseline",
		"derivestatewithbaselines",
		"put approval baseline",
		"get approval baseline",
		"approval baseline",
		"baseline recorded",
		"baseline already current",
		"baseline overwritten with current carrier hash",
		"baseline removed; section re-enters",
		"active but has no baseline; the operator has not yet approved",
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
		"baselineinput",
		"hasbaseline",
		"driftnobaseline",
		"no baseline",
		"snapshotting file hashes",
		"store baseline hashes",
		"stored baseline hashes",
		"file-hash baseline",
		"file hash baseline",
		"file-level verification",
		"symbol-level baseline",
		"stored symbol baseline",
		"baselined files",
		"baselined symbols",
		"baseline exists",
		"baseline for drift detection",
		"compares current file state against stored baseline",
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
		"unknownlegacybaseline",
		"baselinekindunknownlegacy",
		"legacy ambiguous baseline",
		"legacy baseline-like record",
		"legacy/unknown",
	):
		return baselineAuditLegacyAmbiguous, "explicitly names legacy ambiguous baseline posture"
	case containsAnyBaselineTerm(value,
		"approve/rebaseline",
		"approve or rebaseline",
		"approval or rebaseline",
		"approve sections, rebaseline",
		"approve|rebaseline",
		"approve`, `rebaseline",
		"approve`, `reopen",
		"rebaseline/reopen",
		"haft_spec_section(action=\"approve\"",
		"haft_spec_section(approve",
		"haft spec approve",
		"haft spec rebaseline",
		"spec lifecycle commands surface human gates before baseline",
		"human gates before baseline",
		"does not approve specs, decisions, commissions, or baseline",
		"does not approve, rebaseline",
		"without creating approval, rebaseline",
		"not_approval_rebaseline",
		"operator reviews the active specsection and records a baseline",
		"operator chooses rebaseline",
		"human principal approves binding choices and baseline",
		"human review is required for value choices, authority gates, scope expansion, public interface changes, and baseline",
	):
		return baselineAuditLifecycleAuth, "names baseline lifecycle or operator-authority boundary, not a baseline object"
	case containsAnyBaselineTerm(value,
		"baselinekind",
		"sectionbaseline",
		"section baseline",
		"specsection baseline",
		"specsection baselines",
		"spec_section_baselines",
		"baselinestore",
		"baseline store",
		"baselinekindprofile",
		"baseline kind profile",
		"baseline-shaped",
		"baseline shaped",
		"typed baseline",
		"baseline type details",
	):
		return baselineAuditTypedModel, "names the typed baseline model or compatibility projection"
	case baselineAuditSpecflowBaselineStoreSurface(path):
		return baselineAuditSpecApproval, "mentions baseline inside the SpecSection approval baseline store implementation"
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

func baselineAuditHistoricalGovernanceCarrier(path string) bool {
	path = filepath.ToSlash(path)
	for _, prefix := range []string{
		".haft/decisions/",
		".haft/problems/",
		".haft/solutions/",
		".haft/notes/",
		".haft/refresh/",
		".haft/method-runs/",
		".haft/overseer/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func baselineAuditSupportArchiveCarrier(path string) bool {
	path = filepath.ToSlash(path)
	for _, prefix := range []string{
		".haft/methods/",
		".haft/night-runs/",
		".haft/plans/",
		".haft/pi/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func baselineAuditSourceSpecCarrier(path string) bool {
	path = filepath.ToSlash(path)
	for _, prefix := range []string{
		"data/FPF/",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func baselineAuditReleaseNotesCarrier(path string) bool {
	return filepath.ToSlash(path) == "CHANGELOG.md"
}

func baselineAuditToolSurfaceCarrier(path string) bool {
	switch filepath.ToSlash(path) {
	case "internal/cli/baseline_audit.go", "internal/cli/baseline_audit_test.go":
		return true
	default:
		return false
	}
}

func baselineAuditTestFixtureSurface(path string, value string) bool {
	if !strings.HasSuffix(filepath.ToSlash(path), "_test.go") {
		return false
	}
	return containsAnyBaselineTerm(value,
		"baselinetest",
		"testbaseline",
		"newtestbaselinedb",
		"newbaselinetestproject",
		"writebaselinetestsection",
		"baseline fixture",
		"baseline test",
		"decode baseline result",
		"t.Fatalf(\"baseline",
		"t.Fatal(\"baseline",
	)
}

func baselineAuditSpecflowBaselineStoreSurface(path string) bool {
	return filepath.ToSlash(path) == "internal/project/specflow/baseline.go"
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
	if baselineAuditHasPathSegment(rel, "node_modules") {
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
		"*/node_modules",
		"open-sleigh",
		"vendor",
		"dist",
		"build",
		"tmp",
	}
}

func baselineAuditHasPathSegment(rel string, segment string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == segment {
			return true
		}
	}
	return false
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
	case baselineAuditHistoricalGov:
		summary.HistoricalGovernanceCarrier++
	case baselineAuditSupportArchive:
		summary.SupportArchiveCarrier++
	case baselineAuditSourceSpec:
		summary.SourceSpecReference++
	case baselineAuditTypedModel:
		summary.TypedBaselineModel++
	case baselineAuditLifecycleAuth:
		summary.LifecycleAuthority++
	case baselineAuditReleaseNotes:
		summary.ReleaseNotesCarrier++
	case baselineAuditToolSurface:
		summary.AuditToolSurface++
	case baselineAuditTestFixture:
		summary.TestFixtureSurface++
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
		"summary: files=%d matched=%d spec_approval=%d pre_work=%d verified_state=%d comparison=%d ordinary=%d historical_governance=%d support_archive=%d source_spec=%d typed_model=%d lifecycle_authority=%d release_notes=%d audit_tool=%d test_fixture=%d legacy_ambiguous=%d\n",
		report.Summary.FilesScanned,
		report.Summary.MatchedLines,
		report.Summary.SpecSectionApprovalBaseline,
		report.Summary.PreWorkReferenceSnapshot,
		report.Summary.VerifiedStateSnapshot,
		report.Summary.ComparisonOrBenchmark,
		report.Summary.OrdinaryLanguageBaseline,
		report.Summary.HistoricalGovernanceCarrier,
		report.Summary.SupportArchiveCarrier,
		report.Summary.SourceSpecReference,
		report.Summary.TypedBaselineModel,
		report.Summary.LifecycleAuthority,
		report.Summary.ReleaseNotesCarrier,
		report.Summary.AuditToolSurface,
		report.Summary.TestFixtureSurface,
		report.Summary.LegacyAmbiguousBaseline,
	); err != nil {
		return err
	}

	if report.Summary.LegacyAmbiguousBaseline == 0 {
		_, err := fmt.Fprintln(w, "legacy_ambiguous: none")
		return err
	}

	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Category != baselineAuditLegacyAmbiguous {
			continue
		}
		if _, err := fmt.Fprintf(
			w,
			"diagnostic: [%s/%s] %d legacy ambiguous baseline line(s) across %d file(s); next_action: %s\n",
			diagnostic.Level,
			diagnostic.Code,
			diagnostic.Count,
			diagnostic.Files,
			diagnostic.NextAction,
		); err != nil {
			return err
		}
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
