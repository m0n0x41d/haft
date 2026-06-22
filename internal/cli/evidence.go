package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/reff"
)

var (
	evidencePathJSON                     bool
	evidencePathClaimRef                 string
	evidencePathAttemptedUse             string
	evidencePathRequiresCurrentFormality bool
	evidencePathProducerRef              string
	evidencePathMethodRef                string
	evidencePathWorkRef                  string
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Inspect read-only evidence reliance records",
}

var evidencePathCmd = &cobra.Command{
	Use:   "path ARTIFACT_REF EVIDENCE_REF",
	Short: "Build a read-only EvidencePath/RelianceDisposition record",
	Long: `Build a deterministic EvidencePath/RelianceDisposition record for one
existing evidence item and one declared attempted use.

The record is read-only. It does not create evidence, approve anything, pass a
gate, or promote the evidence item into global truth.`,
	Args: cobra.ExactArgs(2),
	RunE: runEvidencePath,
}

func init() {
	evidencePathCmd.Flags().BoolVar(&evidencePathJSON, "json", false, "print structured JSON output")
	evidencePathCmd.Flags().StringVar(&evidencePathClaimRef, "claim-ref", "", "claim id/ref the attempted use relies on")
	evidencePathCmd.Flags().StringVar(&evidencePathAttemptedUse, "attempted-use", "", "declared attempted use boundary")
	evidencePathCmd.Flags().BoolVar(&evidencePathRequiresCurrentFormality, "requires-current-formality", false, "block bounded reliance unless evidence uses current F0-F9 formality")
	evidencePathCmd.Flags().StringVar(&evidencePathProducerRef, "producer-ref", "", "producer trace ref for the evidence")
	evidencePathCmd.Flags().StringVar(&evidencePathMethodRef, "method-ref", "", "method trace ref for the evidence")
	evidencePathCmd.Flags().StringVar(&evidencePathWorkRef, "work-ref", "", "work trace ref for the evidence")
	evidenceCmd.AddCommand(evidencePathCmd)
	rootCmd.AddCommand(evidenceCmd)
}

func runEvidencePath(cmd *cobra.Command, args []string) error {
	_, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	record, err := buildEvidencePathRecord(
		cmd.Context(),
		store,
		artifact.EvidencePathInput{
			ArtifactRef:              args[0],
			EvidenceRef:              args[1],
			ClaimRef:                 evidencePathClaimRef,
			AttemptedUse:             evidencePathAttemptedUse,
			RequiresCurrentFormality: evidencePathRequiresCurrentFormality,
			ProducerRef:              evidencePathProducerRef,
			MethodRef:                evidencePathMethodRef,
			WorkRef:                  evidencePathWorkRef,
		},
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}

	if evidencePathJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}

	return writeEvidencePathSummary(cmd.OutOrStdout(), record)
}

func buildEvidencePathRecord(
	ctx context.Context,
	store *artifact.Store,
	input artifact.EvidencePathInput,
	now time.Time,
) (artifact.EvidencePathRecord, error) {
	if strings.TrimSpace(input.ArtifactRef) == "" {
		return artifact.EvidencePathRecord{}, fmt.Errorf("artifact_ref is required")
	}
	if strings.TrimSpace(input.EvidenceRef) == "" {
		return artifact.EvidencePathRecord{}, fmt.Errorf("evidence_ref is required")
	}

	items, err := store.GetEvidenceItems(ctx, input.ArtifactRef)
	if err != nil {
		return artifact.EvidencePathRecord{}, fmt.Errorf("load evidence items: %w", err)
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) != strings.TrimSpace(input.EvidenceRef) {
			continue
		}

		return artifact.BuildEvidencePathRecord(input, item, now), nil
	}

	return artifact.EvidencePathRecord{}, fmt.Errorf("evidence %q not found on artifact %q", strings.TrimSpace(input.EvidenceRef), strings.TrimSpace(input.ArtifactRef))
}

func writeEvidencePathSummary(w io.Writer, record artifact.EvidencePathRecord) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft evidence path: %s %s artifact=%s evidence=%s\n",
		record.RecordKind,
		record.Authority,
		record.ArtifactRef,
		record.Evidence.ID,
	))
	builder.WriteString(fmt.Sprintf(
		"reliance: %s reason=%s\n",
		record.RelianceDisposition.Disposition,
		record.RelianceDisposition.Reason,
	))
	builder.WriteString(fmt.Sprintf(
		"claim_binding: %s claim_ref=%s\n",
		record.ClaimBinding.Status,
		record.ClaimBinding.ClaimRef,
	))
	builder.WriteString(fmt.Sprintf(
		"trace_binding: %s producer=%s method=%s work=%s\n",
		record.TraceBinding.Status,
		record.TraceBinding.ProducerRef,
		record.TraceBinding.MethodRef,
		record.TraceBinding.WorkRef,
	))
	builder.WriteString(fmt.Sprintf(
		"currentness: %s valid_until=%s\n",
		record.CurrentnessWindow.Status,
		record.CurrentnessWindow.ValidUntil,
	))
	builder.WriteString(fmt.Sprintf(
		"attempted_use: current_formality_required=%t\n",
		record.AttemptedUse.RequiresCurrentFormality,
	))
	builder.WriteString(fmt.Sprintf(
		"formality: %s\n",
		evidencePathFormalitySummary(record.Evidence),
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: approval=%s gate_decision=%s global_truth=%s\n",
		record.AuthorityBoundary.Approval,
		record.AuthorityBoundary.GateDecision,
		record.AuthorityBoundary.GlobalTruth,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}

func evidencePathFormalitySummary(evidence artifact.EvidencePathEvidence) string {
	scale := reff.CurrentFormalityScale(evidence.FormalityLevel)
	if evidence.FormalityScale != nil {
		scale = reff.NormalizeFormalityScale(*evidence.FormalityScale)
	}
	if evidence.FormalityBridge == nil {
		return fmt.Sprintf(
			"level=F%d scale=%s bridge=none loss=%s",
			scale.Level,
			scale.ScaleID,
			reff.FormalityBridgeNoLoss,
		)
	}

	bridge := *evidence.FormalityBridge
	return fmt.Sprintf(
		"level=F%d scale=%s bridge=%s->%s source_level=F%d target_level=F%d loss=%s",
		scale.Level,
		scale.ScaleID,
		bridge.SourceScaleID,
		bridge.TargetScaleID,
		bridge.SourceLevel,
		bridge.TargetLevel,
		bridge.Loss,
	)
}
