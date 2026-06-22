package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	driftRouteJSON       bool
	driftRouteBearerRef  string
	driftRouteUseContext string
	driftBindingsJSON    bool
	driftBindingsLimit   int
	driftBindingsAll     bool
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
	Short: "Dry-run legacy decision symbol-binding proposals",
	Long: `Inspect active DecisionRecords with affected_files and report whether their
symbol-level baselines are missing, broad, already precise, or carrier-only.

This is read-only. It does not mutate affected_symbols, baselines, evidence,
decisions, or markdown carriers.`,
	RunE: runDriftBindings,
}

func init() {
	driftRouteCmd.Flags().BoolVar(&driftRouteJSON, "json", false, "print structured JSON output")
	driftRouteCmd.Flags().StringVar(&driftRouteBearerRef, "bearer-ref", "", "artifact/object carrying the drift")
	driftRouteCmd.Flags().StringVar(&driftRouteUseContext, "use-context", "", "use context to block until repair")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsJSON, "json", false, "print structured JSON output")
	driftBindingsCmd.Flags().IntVar(&driftBindingsLimit, "candidate-limit", 20, "maximum candidate symbols per decision")
	driftBindingsCmd.Flags().BoolVar(&driftBindingsAll, "all", false, "include already-clean/no-action decisions")
	driftCmd.AddCommand(driftRouteCmd)
	driftCmd.AddCommand(driftBindingsCmd)
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

	report, err := artifact.BuildLegacyBindingReport(context.Background(), store, projectRoot, artifact.LegacyBindingOptions{
		CandidateLimit: driftBindingsLimit,
		IncludeClean:   driftBindingsAll,
	})
	if err != nil {
		return err
	}

	if driftBindingsJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}

	return writeDriftBindingsSummary(cmd.OutOrStdout(), report)
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
		"haft drift bindings: %s total=%d high_confidence=%d needs_operator=%d ambiguous=%d already_precise=%d\n",
		report.Authority,
		report.Summary.TotalDecisions,
		report.Summary.HighConfidenceProposals,
		report.Summary.NeedsOperatorSelection,
		report.Summary.AmbiguousFileScope,
		report.Summary.AlreadyPrecise,
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
