package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

var (
	valueSpaceJSON         bool
	valueSpaceWindow       string
	valueSpaceMethodRef    string
	valueSpaceEvidenceRefs []string
)

var valueCmd = &cobra.Command{
	Use:   "value",
	Short: "Inspect read-only engineering value projections",
}

var valueSpaceCmd = &cobra.Command{
	Use:   "space BEARER_REF",
	Short: "Build the Haft engineering-value characteristic space",
	Long: `Build a deterministic read-only engineering-value characteristic space.

The projection contains no total Haft or FPF score. Each characteristic names
its bearer, method, window, denominator, evidence refs, protected trade-offs,
and reopen condition.`,
	Args: cobra.ExactArgs(1),
	RunE: runValueSpace,
}

func init() {
	valueSpaceCmd.Flags().BoolVar(&valueSpaceJSON, "json", false, "print structured JSON output")
	valueSpaceCmd.Flags().StringVar(&valueSpaceWindow, "window", "", "measurement window")
	valueSpaceCmd.Flags().StringVar(&valueSpaceMethodRef, "method-ref", "", "measurement method ref")
	valueSpaceCmd.Flags().StringArrayVar(&valueSpaceEvidenceRefs, "evidence-ref", nil, "evidence ref; repeatable")
	valueCmd.AddCommand(valueSpaceCmd)
	rootCmd.AddCommand(valueCmd)
}

func runValueSpace(cmd *cobra.Command, args []string) error {
	space := artifact.BuildEngineeringValueSpace(artifact.EngineeringValueSpaceInput{
		BearerRef:    args[0],
		Window:       valueSpaceWindow,
		MethodRef:    valueSpaceMethodRef,
		EvidenceRefs: valueSpaceEvidenceRefs,
	})

	if valueSpaceJSON {
		return writeJSON(cmd.OutOrStdout(), space)
	}

	return writeEngineeringValueSpaceSummary(cmd.OutOrStdout(), space)
}

func writeEngineeringValueSpaceSummary(w io.Writer, space artifact.EngineeringValueSpace) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft value space: %s %s bearer=%s single_score=%s characteristics=%d\n",
		space.RecordKind,
		space.Authority,
		space.EvaluatedObject.BearerRef,
		space.ScorePolicy.SingleScore,
		len(space.Characteristics),
	))
	builder.WriteString(fmt.Sprintf(
		"interpretation: healthy_reopening=%s feature_without_value_movement=%s\n",
		space.InterpretationRules.HealthyReopening,
		space.InterpretationRules.FeatureWithoutValueMovement,
	))
	builder.WriteString(fmt.Sprintf(
		"protected_trade_offs: %s\n",
		strings.Join(space.ProtectedTradeOffs, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"simplify_kill_criteria: %d authority=%s\n",
		len(space.SimplifyKillCriteria),
		artifact.EngineeringValueSimplifyKillAuthority,
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: score=%s evidence=%s approval=%s gate_decision=%s global_truth=%s\n",
		space.AuthorityBoundary.Score,
		space.AuthorityBoundary.Evidence,
		space.AuthorityBoundary.Approval,
		space.AuthorityBoundary.GateDecision,
		space.AuthorityBoundary.GlobalTruth,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}
