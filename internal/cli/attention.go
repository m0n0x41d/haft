package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

var (
	attentionBlockedJSON                 bool
	attentionBlockedEntityOrSubjectLabel string
	attentionBlockedFindingKind          string
	attentionBlockedUse                  string
	attentionBlockedSourceRefs           []string
	attentionBlockedExactRecordNeeded    string
	attentionBlockedNextActions          []string
	attentionBlockedRoleRef              string
	attentionBlockedValidUntil           string
)

var attentionCmd = &cobra.Command{
	Use:   "attention",
	Short: "Inspect read-only attention projections",
}

var attentionBlockedCmd = &cobra.Command{
	Use:   "blocked BEARER_REF",
	Short: "Build an object-first blocked-use attention item",
	Long: `Build a deterministic read-only attention item for a blocked use.

The item names the bearer first, returns exact source refs, and lists
admissible next actions. It is not a WorkPlan, approval, evidence,
GateDecision, or global truth.`,
	Args: cobra.ExactArgs(1),
	RunE: runAttentionBlocked,
}

func init() {
	attentionBlockedCmd.Flags().BoolVar(&attentionBlockedJSON, "json", false, "print structured JSON output")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedEntityOrSubjectLabel, "label", "", "human-readable bearer label")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedFindingKind, "finding-kind", "", "attention finding kind")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedUse, "blocked-use", "", "use blocked until exact source return or repair")
	attentionBlockedCmd.Flags().StringArrayVar(&attentionBlockedSourceRefs, "source-ref", nil, "exact source ref; repeatable")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedExactRecordNeeded, "exact-record-needed", "", "exact record needed before stronger use")
	attentionBlockedCmd.Flags().StringArrayVar(&attentionBlockedNextActions, "next-action", nil, "admissible next action; repeatable")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedRoleRef, "required-role-assignment-ref", "", "role assignment needed before action")
	attentionBlockedCmd.Flags().StringVar(&attentionBlockedValidUntil, "valid-until", "", "currentness bound for this attention item")
	attentionCmd.AddCommand(attentionBlockedCmd)
	rootCmd.AddCommand(attentionCmd)
}

func runAttentionBlocked(cmd *cobra.Command, args []string) error {
	item := artifact.BuildBlockedUseAttentionItem(artifact.BlockedUseAttentionInput{
		BearerRef:                 args[0],
		EntityOrSubjectLabel:      attentionBlockedEntityOrSubjectLabel,
		FindingKind:               attentionBlockedFindingKind,
		BlockedUse:                attentionBlockedUse,
		SourceRefs:                attentionBlockedSourceRefs,
		ExactRecordNeeded:         attentionBlockedExactRecordNeeded,
		NextAdmissibleActions:     attentionBlockedNextActions,
		RequiredRoleAssignmentRef: attentionBlockedRoleRef,
		ValidUntil:                attentionBlockedValidUntil,
	})

	if attentionBlockedJSON {
		return writeJSON(cmd.OutOrStdout(), item)
	}

	return writeBlockedUseAttentionSummary(cmd.OutOrStdout(), item)
}

func writeBlockedUseAttentionSummary(w io.Writer, item artifact.BlockedUseAttentionItem) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft attention blocked: %s %s bearer=%s finding=%s blocked_use=%s\n",
		item.RecordKind,
		item.Authority,
		item.Object.BearerRef,
		item.FindingKind,
		item.BlockedUse,
	))
	builder.WriteString(fmt.Sprintf(
		"source_return: status=%s refs=%s exact_record_needed=%s\n",
		item.SourceReturn.Status,
		strings.Join(item.SourceReturn.SourceRefs, ","),
		item.SourceReturn.ExactRecordNeeded,
	))
	builder.WriteString(fmt.Sprintf(
		"next_admissible_actions: %s\n",
		strings.Join(item.NextAdmissibleActions, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: work_plan=%s evidence=%s approval=%s gate_decision=%s global_truth=%s\n",
		item.AuthorityBoundary.WorkPlan,
		item.AuthorityBoundary.Evidence,
		item.AuthorityBoundary.Approval,
		item.AuthorityBoundary.GateDecision,
		item.AuthorityBoundary.GlobalTruth,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}
