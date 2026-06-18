package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

var (
	driftRouteJSON       bool
	driftRouteBearerRef  string
	driftRouteUseContext string
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

func init() {
	driftRouteCmd.Flags().BoolVar(&driftRouteJSON, "json", false, "print structured JSON output")
	driftRouteCmd.Flags().StringVar(&driftRouteBearerRef, "bearer-ref", "", "artifact/object carrying the drift")
	driftRouteCmd.Flags().StringVar(&driftRouteUseContext, "use-context", "", "use context to block until repair")
	driftCmd.AddCommand(driftRouteCmd)
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
