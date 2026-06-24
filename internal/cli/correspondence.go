package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

var correspondenceGraphJSON bool

var correspondenceCmd = &cobra.Command{
	Use:   "correspondence",
	Short: "Inspect read-only correspondence projections",
}

var correspondenceGraphCmd = &cobra.Command{
	Use:   "graph DECISION_REF",
	Short: "Build a qualified correspondence graph projection",
	Long: `Build a deterministic expected-vs-observed correspondence graph for one
DecisionRecord.

The projection is read-only. Graph paths are candidate correspondence paths,
not proof, evidence, approval, gate passage, claim truth, global truth, or
publication.`,
	Args: cobra.ExactArgs(1),
	RunE: runCorrespondenceGraph,
}

func init() {
	correspondenceGraphCmd.Flags().BoolVar(&correspondenceGraphJSON, "json", false, "print structured JSON output")
	correspondenceCmd.AddCommand(correspondenceGraphCmd)
	rootCmd.AddCommand(correspondenceCmd)
}

func runCorrespondenceGraph(cmd *cobra.Command, args []string) error {
	_, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	record, err := buildQualifiedCorrespondenceGraph(
		cmd.Context(),
		store,
		artifact.CorrespondenceGraphInput{DecisionRef: args[0]},
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}

	if correspondenceGraphJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}

	return writeCorrespondenceGraphSummary(cmd.OutOrStdout(), record)
}

func buildQualifiedCorrespondenceGraph(
	ctx context.Context,
	store *artifact.Store,
	input artifact.CorrespondenceGraphInput,
	now time.Time,
) (artifact.QualifiedCorrespondenceGraph, error) {
	decisionRef := strings.TrimSpace(input.DecisionRef)
	if decisionRef == "" {
		return artifact.QualifiedCorrespondenceGraph{}, fmt.Errorf("decision_ref is required")
	}

	decision, err := store.Get(ctx, decisionRef)
	if err != nil {
		return artifact.QualifiedCorrespondenceGraph{}, fmt.Errorf("load decision: %w", err)
	}
	affectedFiles, err := store.GetAffectedFiles(ctx, decisionRef)
	if err != nil {
		return artifact.QualifiedCorrespondenceGraph{}, fmt.Errorf("load affected files: %w", err)
	}
	evidence, err := store.GetEvidenceItems(ctx, decisionRef)
	if err != nil {
		return artifact.QualifiedCorrespondenceGraph{}, fmt.Errorf("load evidence items: %w", err)
	}

	return artifact.BuildQualifiedCorrespondenceGraph(input, decision, affectedFiles, evidence, now)
}

func writeCorrespondenceGraphSummary(
	w io.Writer,
	record artifact.QualifiedCorrespondenceGraph,
) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft correspondence graph: %s %s graph=%s decision=%s\n",
		record.RecordKind,
		record.Authority,
		record.GraphRef,
		record.DecisionRef,
	))
	builder.WriteString(fmt.Sprintf(
		"path_status: %s\n",
		record.PathStatus,
	))
	builder.WriteString(fmt.Sprintf(
		"expected_nodes: %d observed_nodes: %d edges: %d gaps: %d\n",
		len(record.ExpectedRealization),
		len(record.ObservedRealization),
		len(record.Edges),
		len(record.Gaps),
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: proof=%s evidence=%s approval=%s gate_decision=%s claim_truth=%s global_truth=%s publication=%s\n",
		record.AuthorityBoundary.Proof,
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
