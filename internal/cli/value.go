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
		"measurement_context: window=%s method_ref=%s evidence_refs=%d evidence_missing_characteristics=%d\n",
		valueSummaryField(space.Characteristics, func(characteristic artifact.EngineeringValueCharacteristic) string {
			return characteristic.Window
		}),
		valueSummaryMethodRef(space.Characteristics),
		valueSummaryEvidenceRefCount(space.Characteristics),
		valueSummaryMissingEvidenceCount(space.Characteristics),
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
	builder.WriteString("review_triggers:\n")
	for _, criterion := range space.SimplifyKillCriteria {
		builder.WriteString(fmt.Sprintf(
			"  - %s action=%s trigger=%s\n",
			criterion.ID,
			criterion.ReviewAction,
			criterion.Trigger,
		))
	}
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: score=%s evidence=%s approval=%s gate_decision=%s claim_truth=%s global_truth=%s publication=%s\n",
		space.AuthorityBoundary.Score,
		space.AuthorityBoundary.Evidence,
		space.AuthorityBoundary.Approval,
		space.AuthorityBoundary.GateDecision,
		space.AuthorityBoundary.ClaimTruth,
		space.AuthorityBoundary.GlobalTruth,
		space.AuthorityBoundary.Publication,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}

func valueSummaryField(
	characteristics []artifact.EngineeringValueCharacteristic,
	extract func(artifact.EngineeringValueCharacteristic) string,
) string {
	for _, characteristic := range characteristics {
		value := strings.TrimSpace(extract(characteristic))
		if value != "" {
			return value
		}
	}

	return "unspecified"
}

func valueSummaryMethodRef(characteristics []artifact.EngineeringValueCharacteristic) string {
	value := valueSummaryField(characteristics, func(characteristic artifact.EngineeringValueCharacteristic) string {
		return characteristic.Method
	})
	index := strings.Index(value, ":")
	if index > 0 {
		return value[:index]
	}

	return value
}

func valueSummaryEvidenceRefCount(characteristics []artifact.EngineeringValueCharacteristic) int {
	seen := map[string]struct{}{}

	for _, characteristic := range characteristics {
		for _, ref := range characteristic.EvidenceRefs {
			trimmed := strings.TrimSpace(ref)
			if trimmed == "" {
				continue
			}
			seen[trimmed] = struct{}{}
		}
	}

	return len(seen)
}

func valueSummaryMissingEvidenceCount(characteristics []artifact.EngineeringValueCharacteristic) int {
	count := 0

	for _, characteristic := range characteristics {
		if strings.TrimSpace(characteristic.Missingness) == "evidence_refs_missing_value_claim_blocked" {
			count++
		}
	}

	return count
}
