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
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var timeNow = time.Now

const driftEventsSummaryEventLimit = 20
const driftBindingsDryRunJSONLimit = 5
const driftBindingsCompactCandidatePreviewLimit = 5
const driftBindingsCompactDiagnosticPreviewLimit = 2
const driftBindingsCompactReviewGroupLimit = 5
const driftBindingsCompactGroupCandidatePreviewLimit = 3

var (
	driftRouteJSON              bool
	driftRouteBearerRef         string
	driftRouteUseContext        string
	driftBindingsJSON           bool
	driftBindingsDryRun         bool
	driftBindingsOutputLimit    int
	driftBindingsCandidateLimit int
	driftBindingsAll            bool
	driftBindingsApply          bool
	driftBindingsSelect         string
	driftEventsJSON             bool
	driftEventsLedger           string
	driftEventsStatus           string
	driftEventsReason           string
	driftEventsExpiresAt        string
	driftEventsEvidence         []string
	driftEventsRecordedBy       string
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Inspect read-only drift and repair routing projections",
}

var driftRouteCmd = &cobra.Command{
	Use:   "route DRIFT_KIND",
	Short: "Build a semantic drift repair route",
	Long: `Build a deterministic read-only semantic drift route.

The route classifies the drift layer and lists candidate repair actions. It
does not mutate code, carriers, evidence, decisions, baselines, or gates, and
does not create approval, claim truth, global truth, or publication.`,
	Args: cobra.ExactArgs(1),
	RunE: runDriftRoute,
}

var driftBindingsCmd = &cobra.Command{
	Use:   "bindings",
	Short: "Review and repair legacy decision binding proposals",
	Long: `Inspect active DecisionRecords with affected_files and report whether their
binding targets are missing, broad, already precise, or carrier-only.

Default mode is read-only. --apply-high-confidence mutates only local
binding_targets and affected_symbols for resolver-proven high-confidence cases;
--apply-selection mutates only the explicit DecisionRecord binding_targets
provided in a JSON selection document. Neither mode changes file hashes,
evidence, approvals, or markdown carriers.`,
	RunE: runDriftBindings,
}

var driftEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Group per-decision drift into read-only DriftEvents",
	Long: `Group existing per-decision drift reports into deterministic DriftEvents.

This is a read-only projection: it does not mutate decisions, baselines,
evidence, lineage, or carrier files. One changed target can fan out to multiple
impacted decisions without becoming multiple independent debt events.
root_cause, resolution_status, and suggested_next_command are review posture,
not evidence, approval, or gate passage.`,
	RunE: runDriftEvents,
}

var driftEventsResolveCmd = &cobra.Command{
	Use:   "resolve EVENT_ID",
	Short: "Record non-binding DriftEvent resolution metadata",
	Long: `Record scoped DriftEvent resolution metadata in a local ledger.

This does not mutate decisions, baselines, evidence, lineage, gates, or carrier
files. The ledger is an overlay for DriftEvent reports: resolved and unexpired
waived_until records change the event resolution_status only in the report.`,
	Args: cobra.ExactArgs(1),
	RunE: runDriftEventsResolve,
}

func init() {
	driftRouteCmd.Flags().BoolVar(&driftRouteJSON, "json", false, "print structured JSON output")
	driftRouteCmd.Flags().StringVar(&driftRouteBearerRef, "bearer-ref", "", "artifact/object carrying the drift")
	driftRouteCmd.Flags().StringVar(&driftRouteUseContext, "use-context", "", "use context to block until repair")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsJSON, "json", false, "print structured JSON output")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsDryRun, "dry-run", false, "preview binding repair posture without applying changes (default behavior)")
	driftBindingsCmd.Flags().IntVar(&driftBindingsOutputLimit, "limit", -1, "maximum binding review items to emit in JSON output; -1 keeps the full audit report")
	driftBindingsCmd.Flags().IntVar(&driftBindingsCandidateLimit, "candidate-limit", 20, "maximum candidate symbols per decision")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsAll, "all", false, "include already-clean/no-action decisions")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsApply, "apply-high-confidence", false, "apply resolver-proven high-confidence binding target repairs")
	driftBindingsCmd.Flags().StringVar(&driftBindingsSelect, "apply-selection", "", "apply explicit binding target selections from a JSON file")
	driftEventsCmd.Flags().BoolVar(&driftEventsJSON, "json", false, "print structured JSON output")
	driftEventsCmd.Flags().StringVar(&driftEventsLedger, "resolution-ledger", "", "path to DriftEvent resolution ledger JSON")
	driftEventsResolveCmd.Flags().BoolVar(&driftEventsJSON, "json", false, "print structured JSON output")
	driftEventsResolveCmd.Flags().StringVar(&driftEventsLedger, "resolution-ledger", "", "path to DriftEvent resolution ledger JSON")
	driftEventsResolveCmd.Flags().StringVar(&driftEventsStatus, "status", "", "resolution status: resolved or waived_until")
	driftEventsResolveCmd.Flags().StringVar(&driftEventsReason, "reason", "", "operator-readable resolution reason")
	driftEventsResolveCmd.Flags().StringVar(&driftEventsExpiresAt, "waiver-expires-at", "", "expiry for status=waived_until, RFC3339 or YYYY-MM-DD")
	driftEventsResolveCmd.Flags().StringArrayVar(&driftEventsEvidence, "evidence-ref", nil, "evidence reference supporting the resolution metadata")
	driftEventsResolveCmd.Flags().StringVar(&driftEventsRecordedBy, "recorded-by", "", "actor or workflow recording the metadata")
	driftEventsCmd.AddCommand(driftEventsResolveCmd)
	driftCmd.AddCommand(driftRouteCmd)
	driftCmd.AddCommand(driftBindingsCmd)
	driftCmd.AddCommand(driftEventsCmd)
	rootCmd.AddCommand(driftCmd)
}

func runDriftRoute(cmd *cobra.Command, args []string) error {
	record := artifact.BuildSemanticDriftRoute(artifact.DriftRouteInput{
		DriftKind:  args[0],
		BearerRef:  driftRouteBearerRef,
		UseContext: driftRouteUseContext,
	})

	if driftRouteJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}

	return writeDriftRouteSummary(cmd.OutOrStdout(), record)
}

func runDriftBindings(cmd *cobra.Command, _ []string) error {
	if err := validateDriftBindingsMode(); err != nil {
		return err
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	var report artifact.LegacyBindingReport
	if strings.TrimSpace(driftBindingsSelect) != "" {
		selection, err := readLegacyBindingSelectionDocument(driftBindingsSelect)
		if err != nil {
			return err
		}
		report, err = artifact.ApplyLegacyBindingSelections(context.Background(), store, selection)
	} else if driftBindingsApply {
		report, err = artifact.ApplyHighConfidenceLegacyBindingRepairs(context.Background(), store, projectRoot, artifact.LegacyBindingApplyOptions{
			CandidateLimit: driftBindingsCandidateLimit,
		})
	} else {
		report, err = artifact.BuildLegacyBindingReport(context.Background(), store, projectRoot, artifact.LegacyBindingOptions{
			CandidateLimit: driftBindingsCandidateLimit,
			IncludeClean:   driftBindingsAll,
		})
	}
	if err != nil {
		return err
	}

	if driftBindingsJSON {
		payload := driftBindingsJSONPayload(report, driftBindingsDryRun, driftBindingsOutputLimit)
		return writeJSON(cmd.OutOrStdout(), payload)
	}

	return writeDriftBindingsSummary(cmd.OutOrStdout(), report)
}

type driftBindingsProjectedReport struct {
	SchemaVersion    string                        `json:"schema_version"`
	Authority        string                        `json:"authority"`
	View             string                        `json:"view"`
	Summary          artifact.LegacyBindingSummary `json:"summary"`
	Items            []driftBindingsProjectedItem  `json:"items"`
	Applied          []artifact.LegacyBindingApply `json:"applied,omitempty"`
	OmittedItems     int                           `json:"omitted_items"`
	FullAuditCommand string                        `json:"full_audit_command"`
}

type driftBindingsProjectedItem struct {
	DecisionID                string                              `json:"decision_id"`
	DecisionTitle             string                              `json:"decision_title"`
	AffectedFiles             []string                            `json:"affected_files"`
	StoredSymbolCount         int                                 `json:"stored_symbol_count"`
	CandidateSymbolCount      int                                 `json:"candidate_symbol_count"`
	Posture                   string                              `json:"posture"`
	RecommendedAction         string                              `json:"recommended_action"`
	HighConfidence            bool                                `json:"high_confidence,omitempty"`
	Reason                    string                              `json:"reason"`
	RankingPolicy             string                              `json:"ranking_policy,omitempty"`
	CandidateSymbolPreview    []driftBindingsRankedCandidate      `json:"candidate_symbol_preview,omitempty"`
	CandidateSymbolsOmitted   int                                 `json:"candidate_symbols_omitted,omitempty"`
	CandidateReviewGroups     []driftBindingsCandidateReviewGroup `json:"candidate_review_groups,omitempty"`
	CandidateReviewGroupsOmit int                                 `json:"candidate_review_groups_omitted,omitempty"`
	BindingTargets            []artifact.BindingTarget            `json:"binding_targets,omitempty"`
	DiagnosticPreview         []driftBindingsProjectedDiagnostic  `json:"diagnostic_preview,omitempty"`
	DiagnosticsOmitted        int                                 `json:"diagnostics_omitted,omitempty"`
	FullCandidateAuditCommand string                              `json:"full_candidate_audit_command,omitempty"`
}

type driftBindingsRankedCandidate struct {
	FilePath       string   `json:"file_path"`
	SymbolName     string   `json:"symbol_name"`
	SymbolKind     string   `json:"symbol_kind"`
	Line           int      `json:"line"`
	EndLine        int      `json:"end_line"`
	ReviewRank     int      `json:"review_rank"`
	ReviewScore    int      `json:"review_score"`
	MatchedTerms   []string `json:"matched_terms,omitempty"`
	RankingSignals []string `json:"ranking_signals,omitempty"`
}

type driftBindingsCandidateReviewGroup struct {
	FilePath                string                         `json:"file_path"`
	CandidateCount          int                            `json:"candidate_count"`
	CandidateSymbolPreview  []driftBindingsRankedCandidate `json:"candidate_symbol_preview,omitempty"`
	CandidateSymbolsOmitted int                            `json:"candidate_symbols_omitted,omitempty"`
	BestReviewScore         int                            `json:"best_review_score"`
	MatchedTerms            []string                       `json:"matched_terms,omitempty"`
	RankingSignals          []string                       `json:"ranking_signals,omitempty"`
}

type driftBindingsProjectedDiagnostic struct {
	FilePath string `json:"file_path"`
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
}

func driftBindingsJSONPayload(report artifact.LegacyBindingReport, dryRun bool, limit int) any {
	effectiveLimit := limit
	if dryRun && effectiveLimit < 0 {
		effectiveLimit = driftBindingsDryRunJSONLimit
	}
	if effectiveLimit < 0 {
		return report
	}

	items := report.Items
	omitted := 0
	if effectiveLimit < len(items) {
		items = items[:effectiveLimit]
		omitted = len(report.Items) - effectiveLimit
	}

	return driftBindingsProjectedReport{
		SchemaVersion:    report.SchemaVersion,
		Authority:        report.Authority,
		View:             "compact",
		Summary:          report.Summary,
		Items:            driftBindingsProjectedItems(items),
		Applied:          report.Applied,
		OmittedItems:     omitted,
		FullAuditCommand: "haft drift bindings --json",
	}
}

func driftBindingsProjectedItems(items []artifact.LegacyBindingItem) []driftBindingsProjectedItem {
	out := make([]driftBindingsProjectedItem, 0, len(items))
	for _, item := range items {
		out = append(out, driftBindingsProjectedItemFromLegacy(item))
	}
	return out
}

func driftBindingsProjectedItemFromLegacy(item artifact.LegacyBindingItem) driftBindingsProjectedItem {
	rankedCandidates := driftBindingsRankedCandidates(item)
	candidates := rankedCandidates
	candidateOmitted := 0
	if len(candidates) > driftBindingsCompactCandidatePreviewLimit {
		candidates = candidates[:driftBindingsCompactCandidatePreviewLimit]
		candidateOmitted = len(rankedCandidates) - driftBindingsCompactCandidatePreviewLimit
	}

	groups := driftBindingsCandidateReviewGroups(rankedCandidates)
	groupsOmitted := 0
	if len(groups) > driftBindingsCompactReviewGroupLimit {
		groupsOmitted = len(groups) - driftBindingsCompactReviewGroupLimit
		groups = groups[:driftBindingsCompactReviewGroupLimit]
	}

	diagnostics := driftBindingsProjectedDiagnostics(item.Diagnostics)
	diagnosticsOmitted := 0
	if len(diagnostics) > driftBindingsCompactDiagnosticPreviewLimit {
		diagnostics = diagnostics[:driftBindingsCompactDiagnosticPreviewLimit]
		diagnosticsOmitted = len(item.Diagnostics) - driftBindingsCompactDiagnosticPreviewLimit
	}

	return driftBindingsProjectedItem{
		DecisionID:                item.DecisionID,
		DecisionTitle:             item.DecisionTitle,
		AffectedFiles:             item.AffectedFiles,
		StoredSymbolCount:         item.StoredSymbolCount,
		CandidateSymbolCount:      item.CandidateSymbolCount,
		Posture:                   item.Posture,
		RecommendedAction:         item.RecommendedAction,
		HighConfidence:            item.HighConfidence,
		Reason:                    item.Reason,
		RankingPolicy:             driftBindingsRankingPolicy(item),
		CandidateSymbolPreview:    candidates,
		CandidateSymbolsOmitted:   candidateOmitted,
		CandidateReviewGroups:     groups,
		CandidateReviewGroupsOmit: groupsOmitted,
		BindingTargets:            item.BindingTargets,
		DiagnosticPreview:         diagnostics,
		DiagnosticsOmitted:        diagnosticsOmitted,
		FullCandidateAuditCommand: driftBindingsFullCandidateAuditCommand(item),
	}
}

func driftBindingsRankingPolicy(item artifact.LegacyBindingItem) string {
	if len(item.CandidateSymbols) == 0 {
		return ""
	}
	return "review_only_title_file_kind_rank_not_binding_authority"
}

func driftBindingsRankedCandidates(item artifact.LegacyBindingItem) []driftBindingsRankedCandidate {
	titleTokens := driftBindingsReviewTokenSet(item.DecisionTitle)
	ranked := make([]driftBindingsRankedCandidate, 0, len(item.CandidateSymbols))
	for _, candidate := range item.CandidateSymbols {
		ranked = append(ranked, driftBindingsRankCandidate(candidate, titleTokens))
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		left := ranked[i]
		right := ranked[j]
		if left.ReviewScore != right.ReviewScore {
			return left.ReviewScore > right.ReviewScore
		}
		if left.FilePath != right.FilePath {
			return left.FilePath < right.FilePath
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.SymbolName < right.SymbolName
	})

	for index := range ranked {
		ranked[index].ReviewRank = index + 1
	}
	return ranked
}

func driftBindingsRankCandidate(
	candidate artifact.LegacyBindingCandidate,
	titleTokens map[string]struct{},
) driftBindingsRankedCandidate {
	symbolTokens := driftBindingsReviewTokenSet(candidate.SymbolName, candidate.SymbolKind)
	fileTokens := driftBindingsReviewTokenSet(candidate.FilePath, filepath.Base(candidate.FilePath))
	symbolMatches := driftBindingsIntersectSorted(titleTokens, symbolTokens)
	fileMatches := driftBindingsIntersectSorted(titleTokens, fileTokens)
	matchedTerms := driftBindingsMergeSortedTerms(symbolMatches, fileMatches)

	score := 0
	signals := []string{}
	if len(symbolMatches) > 0 {
		score += len(symbolMatches) * 10
		signals = append(signals, "symbol_title_match")
	}
	if len(fileMatches) > 0 {
		score += len(fileMatches) * 4
		signals = append(signals, "file_title_match")
	}
	if driftBindingsSourceFile(candidate.FilePath) {
		score += 2
		signals = append(signals, "source_file")
	}
	if driftBindingsExportedSymbol(candidate.SymbolName) {
		score++
		signals = append(signals, "exported_symbol")
	}
	switch candidate.SymbolKind {
	case "type", "func", "method":
		score++
		signals = append(signals, "governed_symbol_kind")
	}
	if len(signals) == 0 {
		signals = append(signals, "source_order_tiebreak")
	}

	return driftBindingsRankedCandidate{
		FilePath:       candidate.FilePath,
		SymbolName:     candidate.SymbolName,
		SymbolKind:     candidate.SymbolKind,
		Line:           candidate.Line,
		EndLine:        candidate.EndLine,
		ReviewScore:    score,
		MatchedTerms:   matchedTerms,
		RankingSignals: signals,
	}
}

func driftBindingsCandidateReviewGroups(
	ranked []driftBindingsRankedCandidate,
) []driftBindingsCandidateReviewGroup {
	byFile := map[string][]driftBindingsRankedCandidate{}
	for _, candidate := range ranked {
		byFile[candidate.FilePath] = append(byFile[candidate.FilePath], candidate)
	}

	groups := make([]driftBindingsCandidateReviewGroup, 0, len(byFile))
	for filePath, candidates := range byFile {
		preview := candidates
		omitted := 0
		if len(preview) > driftBindingsCompactGroupCandidatePreviewLimit {
			omitted = len(candidates) - driftBindingsCompactGroupCandidatePreviewLimit
			preview = preview[:driftBindingsCompactGroupCandidatePreviewLimit]
		}

		groups = append(groups, driftBindingsCandidateReviewGroup{
			FilePath:                filePath,
			CandidateCount:          len(candidates),
			CandidateSymbolPreview:  preview,
			CandidateSymbolsOmitted: omitted,
			BestReviewScore:         candidates[0].ReviewScore,
			MatchedTerms:            driftBindingsGroupMatchedTerms(candidates),
			RankingSignals:          driftBindingsGroupSignals(candidates),
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		left := groups[i]
		right := groups[j]
		if left.BestReviewScore != right.BestReviewScore {
			return left.BestReviewScore > right.BestReviewScore
		}
		if left.CandidateCount != right.CandidateCount {
			return left.CandidateCount > right.CandidateCount
		}
		return left.FilePath < right.FilePath
	})
	return groups
}

func driftBindingsGroupMatchedTerms(candidates []driftBindingsRankedCandidate) []string {
	terms := map[string]struct{}{}
	for _, candidate := range candidates {
		for _, term := range candidate.MatchedTerms {
			terms[term] = struct{}{}
		}
	}
	return driftBindingsSortedKeys(terms)
}

func driftBindingsGroupSignals(candidates []driftBindingsRankedCandidate) []string {
	signals := map[string]struct{}{}
	for _, candidate := range candidates {
		for _, signal := range candidate.RankingSignals {
			signals[signal] = struct{}{}
		}
	}
	return driftBindingsSortedKeys(signals)
}

func driftBindingsReviewTokenSet(values ...string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		expanded := driftBindingsExpandIdentifier(value)
		for _, token := range strings.FieldsFunc(expanded, driftBindingsTokenSeparator) {
			token = strings.ToLower(strings.TrimSpace(token))
			if len(token) < 3 || driftBindingsReviewStopWord(token) {
				continue
			}
			out[token] = struct{}{}
		}
	}
	return out
}

func driftBindingsExpandIdentifier(value string) string {
	var builder strings.Builder
	var previous rune
	for _, current := range value {
		if previous != 0 && unicode.IsLower(previous) && unicode.IsUpper(current) {
			builder.WriteRune(' ')
		}
		builder.WriteRune(current)
		previous = current
	}
	return builder.String()
}

func driftBindingsTokenSeparator(value rune) bool {
	return !unicode.IsLetter(value) && !unicode.IsDigit(value)
}

func driftBindingsReviewStopWord(token string) bool {
	switch token {
	case "add", "and", "for", "from", "into", "the", "this", "that", "with":
		return true
	default:
		return false
	}
}

func driftBindingsIntersectSorted(
	left map[string]struct{},
	right map[string]struct{},
) []string {
	out := map[string]struct{}{}
	for token := range left {
		if _, ok := right[token]; ok {
			out[token] = struct{}{}
		}
	}
	return driftBindingsSortedKeys(out)
}

func driftBindingsMergeSortedTerms(sets ...[]string) []string {
	out := map[string]struct{}{}
	for _, set := range sets {
		for _, value := range set {
			out[value] = struct{}{}
		}
	}
	return driftBindingsSortedKeys(out)
}

func driftBindingsSortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func driftBindingsSourceFile(path string) bool {
	return !strings.HasSuffix(path, "_test.go") && !strings.Contains(path, "_test.")
}

func driftBindingsExportedSymbol(symbolName string) bool {
	for _, char := range symbolName {
		return unicode.IsUpper(char)
	}
	return false
}

func driftBindingsProjectedDiagnostics(
	diagnostics []artifact.BindingDiagnostic,
) []driftBindingsProjectedDiagnostic {
	out := make([]driftBindingsProjectedDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, driftBindingsProjectedDiagnostic{
			FilePath: diagnostic.FilePath,
			Kind:     diagnostic.Kind,
			Severity: diagnostic.Severity,
		})
	}
	return out
}

func driftBindingsFullCandidateAuditCommand(item artifact.LegacyBindingItem) string {
	if strings.TrimSpace(item.DecisionID) == "" {
		return ""
	}
	return "haft drift bindings --json"
}

func validateDriftBindingsMode() error {
	selectionPath := strings.TrimSpace(driftBindingsSelect)
	if driftBindingsApply && selectionPath != "" {
		return fmt.Errorf("--apply-high-confidence and --apply-selection are mutually exclusive")
	}
	if driftBindingsDryRun && (driftBindingsApply || selectionPath != "") {
		return fmt.Errorf("--dry-run is read-only and cannot be combined with --apply-high-confidence or --apply-selection")
	}
	return nil
}

func runDriftEvents(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	reports, err := artifact.CheckDrift(context.Background(), store, projectRoot)
	if err != nil {
		return fmt.Errorf("scan drift: %w", err)
	}
	ledger, err := readDriftEventResolutionLedger(driftEventResolutionLedgerPath(projectRoot, driftEventsLedger))
	if err != nil {
		return err
	}
	eventReport := buildDriftEventReportWithResolutionLedger(reports, ledger, timeNow())

	if driftEventsJSON {
		return writeJSON(cmd.OutOrStdout(), eventReport)
	}

	return writeDriftEventsSummary(cmd.OutOrStdout(), eventReport)
}

func buildDriftEventReportWithResolutionLedger(
	reports []artifact.DriftReport,
	ledger artifact.DriftEventResolutionLedger,
	now time.Time,
) artifact.DriftEventReport {
	eventReport := artifact.BuildDriftEventReport(reports)
	return artifact.ApplyDriftEventResolutionLedger(eventReport, ledger, now)
}

func applyDefaultDriftEventResolutionLedgerToStatusData(
	ctx context.Context,
	store artifact.ArtifactStore,
	projectRoot string,
	data artifact.StatusData,
) artifact.StatusData {
	if strings.TrimSpace(projectRoot) == "" || data.DriftEvents.SchemaVersion == 0 {
		return data
	}
	ledger, err := readDriftEventResolutionLedger(driftEventResolutionLedgerPath(projectRoot, ""))
	if err != nil {
		return data
	}
	data.DriftEvents = artifact.ApplyDriftEventResolutionLedger(data.DriftEvents, ledger, timeNow())
	data.ReconciliationCues = artifact.BuildStatusReconciliationCueReport(ctx, store, data.DriftEvents)
	return data
}

func runDriftEventsResolve(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	reports, err := artifact.CheckDrift(context.Background(), store, projectRoot)
	if err != nil {
		return fmt.Errorf("scan drift: %w", err)
	}
	eventReport := artifact.BuildDriftEventReport(reports)
	currentEvent, ok := driftEventReportEvent(eventReport, args[0])
	if !ok {
		return fmt.Errorf("drift event %q not found in current scan", args[0])
	}

	ledgerPath := driftEventResolutionLedgerPath(projectRoot, driftEventsLedger)
	ledger, err := readDriftEventResolutionLedger(ledgerPath)
	if err != nil {
		return err
	}
	now := timeNow()
	record := artifact.DriftEventResolution{
		EventID:         strings.TrimSpace(args[0]),
		Status:          strings.TrimSpace(driftEventsStatus),
		Reason:          strings.TrimSpace(driftEventsReason),
		EvidenceRefs:    append([]string(nil), driftEventsEvidence...),
		WaiverExpiresAt: strings.TrimSpace(driftEventsExpiresAt),
		RecordedAt:      now.Format(time.RFC3339),
		RecordedBy:      strings.TrimSpace(driftEventsRecordedBy),
	}
	record = artifact.BindDriftEventResolutionToEvent(record, currentEvent)
	updated, err := artifact.UpsertDriftEventResolution(ledger, record, now)
	if err != nil {
		return err
	}
	if err := writeDriftEventResolutionLedger(ledgerPath, updated); err != nil {
		return err
	}

	result := artifact.ApplyDriftEventResolutionLedger(eventReport, updated, now)
	if driftEventsJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	return writeDriftEventsResolutionSummary(cmd.OutOrStdout(), ledgerPath, record)
}

func readLegacyBindingSelectionDocument(path string) (artifact.LegacyBindingSelectionDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifact.LegacyBindingSelectionDocument{}, fmt.Errorf("read binding selection %s: %w", path, err)
	}

	var document artifact.LegacyBindingSelectionDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return artifact.LegacyBindingSelectionDocument{}, fmt.Errorf("parse binding selection %s: %w", path, err)
	}
	if len(document.Items) == 0 {
		return artifact.LegacyBindingSelectionDocument{}, fmt.Errorf("binding selection %s has no items", path)
	}

	return document, nil
}

func driftEventResolutionLedgerPath(projectRoot string, explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	return filepath.Join(projectRoot, ".haft", "drift-event-resolutions.json")
}

func readDriftEventResolutionLedger(path string) (artifact.DriftEventResolutionLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return artifact.NewDriftEventResolutionLedger(nil), nil
		}
		return artifact.DriftEventResolutionLedger{}, fmt.Errorf("read drift event resolution ledger %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return artifact.NewDriftEventResolutionLedger(nil), nil
	}

	var ledger artifact.DriftEventResolutionLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return artifact.DriftEventResolutionLedger{}, fmt.Errorf("parse drift event resolution ledger %s: %w", path, err)
	}
	if ledger.SchemaVersion == 0 {
		ledger.SchemaVersion = 1
	}
	if strings.TrimSpace(ledger.Authority) == "" {
		ledger.Authority = artifact.DriftEventResolutionLedgerAuthority
	}
	return ledger, nil
}

func writeDriftEventResolutionLedger(path string, ledger artifact.DriftEventResolutionLedger) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create drift event resolution ledger directory %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("encode drift event resolution ledger: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write drift event resolution ledger %s: %w", path, err)
	}
	return nil
}

func driftEventReportHasEvent(report artifact.DriftEventReport, eventID string) bool {
	_, ok := driftEventReportEvent(report, eventID)
	return ok
}

func driftEventReportEvent(report artifact.DriftEventReport, eventID string) (artifact.DriftEvent, bool) {
	eventID = strings.TrimSpace(eventID)
	for _, event := range report.Events {
		if event.EventID == eventID {
			return event, true
		}
	}
	return artifact.DriftEvent{}, false
}

func openArtifactStore(projectRoot string) (*artifact.Store, func(), error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return nil, nil, fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return nil, nil, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return nil, nil, fmt.Errorf("get DB path: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open DB: %w", err)
	}

	closeFn := func() {
		_ = sqlDB.Close()
	}
	return artifact.NewStore(sqlDB), closeFn, nil
}

func writeDriftBindingsSummary(w io.Writer, report artifact.LegacyBindingReport) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf(
		"haft drift bindings: %s total=%d high_confidence=%d needs_operator=%d missing_bindings=%d ambiguous=%d already_precise=%d applied=%d\n",
		report.Authority,
		report.Summary.TotalDecisions,
		report.Summary.HighConfidenceProposals,
		report.Summary.NeedsOperatorSelection,
		report.Summary.MissingBindingTargets,
		report.Summary.AmbiguousFileScope,
		report.Summary.AlreadyPrecise,
		len(report.Applied),
	))
	for _, item := range report.Items {
		builder.WriteString(fmt.Sprintf(
			"- %s `%s` posture=%s action=%s candidates=%d\n",
			item.DecisionTitle,
			item.DecisionID,
			item.Posture,
			item.RecommendedAction,
			item.CandidateSymbolCount,
		))
	}
	_, err := io.WriteString(w, builder.String())
	return err
}

func writeDriftRouteSummary(w io.Writer, record artifact.SemanticDriftRoute) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft drift route: %s %s drift=%s layer=%s recognized=%t\n",
		record.RecordKind,
		record.Authority,
		record.DriftKind,
		record.DriftLayer,
		record.Recognized,
	))
	builder.WriteString(fmt.Sprintf(
		"candidate_repair_actions: %s\n",
		strings.Join(record.CandidateRepairActions, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"language_state_moves: %s entity_mode=%s\n",
		strings.Join(record.LanguageStateMoveKinds, ","),
		record.EntityOfConcernChangeMode,
	))
	builder.WriteString(fmt.Sprintf(
		"next_admissible_move: %s\n",
		record.NextAdmissibleMove,
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: mutation=%s evidence=%s approval=%s gate_decision=%s claim_truth=%s global_truth=%s publication=%s\n",
		record.AuthorityBoundary.Mutation,
		record.AuthorityBoundary.Evidence,
		record.AuthorityBoundary.Approval,
		record.AuthorityBoundary.GateDecision,
		record.AuthorityBoundary.ClaimTruth,
		record.AuthorityBoundary.GlobalTruth,
		record.AuthorityBoundary.Publication,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}

func writeDriftEventsSummary(w io.Writer, report artifact.DriftEventReport) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf(
		"haft drift events: unique=%d impacted_decisions=%d material=%d audit_only=%d needs_binding=%d resolved=%d waived=%d max_fanout=%d\n",
		report.Summary.UniqueEvents,
		report.Summary.ImpactedDecisions,
		report.Summary.MaterialEvents,
		report.Summary.AuditOnlyEvents,
		report.Summary.NeedsBindingResolutionEvents,
		report.Summary.ResolvedByLedgerEvents,
		report.Summary.WaivedByLedgerEvents,
		report.Summary.MaxFanout,
	))
	visible := report.Events
	if len(visible) > driftEventsSummaryEventLimit {
		visible = visible[:driftEventsSummaryEventLimit]
	}
	for _, event := range visible {
		fallback := ""
		if event.FallbackKind != "" {
			fallback = fmt.Sprintf(" fallback=%s", event.FallbackKind)
		}
		builder.WriteString(fmt.Sprintf(
			"- %s target=%s fanout=%d materiality=%s%s root_cause=%s resolution=%s\n",
			event.EventID,
			event.ChangedTargetRef,
			event.Fanout,
			event.Materiality,
			fallback,
			event.RootCause,
			event.ResolutionStatus,
		))
	}
	if omitted := len(report.Events) - len(visible); omitted > 0 {
		builder.WriteString(fmt.Sprintf(
			"... and %d more DriftEvent(s); run `haft drift events --json` for full audit detail\n",
			omitted,
		))
	}
	_, err := io.WriteString(w, builder.String())
	return err
}

func writeDriftEventsResolutionSummary(
	w io.Writer,
	ledgerPath string,
	record artifact.DriftEventResolution,
) error {
	_, err := fmt.Fprintf(
		w,
		"haft drift events resolve: event=%s status=%s ledger=%s authority=%s\n",
		record.EventID,
		record.Status,
		ledgerPath,
		artifact.DriftEventResolutionLedgerAuthority,
	)
	return err
}
