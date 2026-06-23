package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var timeNow = time.Now

const driftEventsSummaryEventLimit = 20

var (
	driftRouteJSON        bool
	driftRouteBearerRef   string
	driftRouteUseContext  string
	driftBindingsJSON     bool
	driftBindingsLimit    int
	driftBindingsAll      bool
	driftBindingsApply    bool
	driftBindingsSelect   string
	driftEventsJSON       bool
	driftEventsLedger     string
	driftEventsStatus     string
	driftEventsReason     string
	driftEventsExpiresAt  string
	driftEventsEvidence   []string
	driftEventsRecordedBy string
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
does not mutate code, carriers, evidence, decisions, baselines, or gates.`,
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
impacted decisions without becoming multiple independent debt events.`,
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
	driftBindingsCmd.Flags().IntVar(&driftBindingsLimit, "candidate-limit", 20, "maximum candidate symbols per decision")
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
	if driftBindingsApply && strings.TrimSpace(driftBindingsSelect) != "" {
		return fmt.Errorf("--apply-high-confidence and --apply-selection are mutually exclusive")
	}
	if strings.TrimSpace(driftBindingsSelect) != "" {
		selection, err := readLegacyBindingSelectionDocument(driftBindingsSelect)
		if err != nil {
			return err
		}
		report, err = artifact.ApplyLegacyBindingSelections(context.Background(), store, selection)
	} else if driftBindingsApply {
		report, err = artifact.ApplyHighConfidenceLegacyBindingRepairs(context.Background(), store, projectRoot, artifact.LegacyBindingApplyOptions{
			CandidateLimit: driftBindingsLimit,
		})
	} else {
		report, err = artifact.BuildLegacyBindingReport(context.Background(), store, projectRoot, artifact.LegacyBindingOptions{
			CandidateLimit: driftBindingsLimit,
			IncludeClean:   driftBindingsAll,
		})
	}
	if err != nil {
		return err
	}

	if driftBindingsJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}

	return writeDriftBindingsSummary(cmd.OutOrStdout(), report)
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
		"authority_boundary: mutation=%s evidence=%s approval=%s gate_decision=%s global_truth=%s\n",
		record.AuthorityBoundary.Mutation,
		record.AuthorityBoundary.Evidence,
		record.AuthorityBoundary.Approval,
		record.AuthorityBoundary.GateDecision,
		record.AuthorityBoundary.GlobalTruth,
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
