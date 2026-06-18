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

var (
	changeCaseJSON         bool
	changeCaseAttemptedUse string
	changeCaseProducerRef  string
	changeCaseMethodRef    string
	changeCaseWorkRef      string
)

var changeCmd = &cobra.Command{
	Use:   "change",
	Short: "Inspect read-only engineering change projections",
}

var changeCaseCmd = &cobra.Command{
	Use:   "case DECISION_REF",
	Short: "Build a read-only EngineeringChangeCase projection",
	Long: `Build a deterministic EngineeringChangeCase aggregate for one
DecisionRecord.

The projection derives problem, transformation, choice, and evidence references
from existing artifacts. It is not proof, approval, gate passage, work
occurrence, or global truth.`,
	Args: cobra.ExactArgs(1),
	RunE: runChangeCase,
}

func init() {
	changeCaseCmd.Flags().BoolVar(&changeCaseJSON, "json", false, "print structured JSON output")
	changeCaseCmd.Flags().StringVar(&changeCaseAttemptedUse, "attempted-use", "", "declared evidence reliance boundary")
	changeCaseCmd.Flags().StringVar(&changeCaseProducerRef, "producer-ref", "", "producer trace ref for derived evidence paths")
	changeCaseCmd.Flags().StringVar(&changeCaseMethodRef, "method-ref", "", "method trace ref for derived evidence paths")
	changeCaseCmd.Flags().StringVar(&changeCaseWorkRef, "work-ref", "", "work trace ref for derived evidence paths")
	changeCmd.AddCommand(changeCaseCmd)
	rootCmd.AddCommand(changeCmd)
}

func runChangeCase(cmd *cobra.Command, args []string) error {
	_, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	record, err := buildEngineeringChangeCase(
		cmd.Context(),
		store,
		artifact.EngineeringChangeCaseInput{
			DecisionRef:  args[0],
			AttemptedUse: changeCaseAttemptedUse,
			ProducerRef:  changeCaseProducerRef,
			MethodRef:    changeCaseMethodRef,
			WorkRef:      changeCaseWorkRef,
		},
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}

	if changeCaseJSON {
		return writeJSON(cmd.OutOrStdout(), record)
	}

	return writeEngineeringChangeCaseSummary(cmd.OutOrStdout(), record)
}

func buildEngineeringChangeCase(
	ctx context.Context,
	store *artifact.Store,
	input artifact.EngineeringChangeCaseInput,
	now time.Time,
) (artifact.EngineeringChangeCase, error) {
	if strings.TrimSpace(input.DecisionRef) == "" {
		return artifact.EngineeringChangeCase{}, fmt.Errorf("decision_ref is required")
	}

	decision, err := store.Get(ctx, strings.TrimSpace(input.DecisionRef))
	if err != nil {
		return artifact.EngineeringChangeCase{}, fmt.Errorf("load decision: %w", err)
	}

	evidence, err := store.GetEvidenceItems(ctx, strings.TrimSpace(input.DecisionRef))
	if err != nil {
		return artifact.EngineeringChangeCase{}, fmt.Errorf("load evidence items: %w", err)
	}

	skeleton, err := artifact.BuildEngineeringChangeCase(input, decision, nil, evidence, now)
	if err != nil {
		return artifact.EngineeringChangeCase{}, err
	}

	problems := loadEngineeringChangeCaseProblems(ctx, store, skeleton.ProblemCardRefs)

	return artifact.BuildEngineeringChangeCase(input, decision, problems, evidence, now)
}

func loadEngineeringChangeCaseProblems(
	ctx context.Context,
	store *artifact.Store,
	refs []string,
) []*artifact.Artifact {
	problems := make([]*artifact.Artifact, 0, len(refs))
	for _, ref := range refs {
		problem, err := store.Get(ctx, ref)
		if err != nil {
			continue
		}
		if problem.Meta.Kind != artifact.KindProblemCard {
			continue
		}
		problems = append(problems, problem)
	}

	return problems
}

func writeEngineeringChangeCaseSummary(w io.Writer, record artifact.EngineeringChangeCase) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft change case: %s %s case=%s decision=%s\n",
		record.RecordKind,
		record.Authority,
		record.CaseRef,
		record.DecisionSpeechActRef,
	))
	builder.WriteString(fmt.Sprintf(
		"problem_refs: %s\n",
		strings.Join(record.ProblemCardRefs, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"transformation_refs: %s\n",
		strings.Join(record.TransformationRefs, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"choice_result_ref: %s candidate_set_ref=%s comparison_result_ref=%s\n",
		record.ChoiceResultRef,
		record.CandidateSetRef,
		record.ComparisonResultRef,
	))
	builder.WriteString(fmt.Sprintf(
		"evidence_refs: %s evidence_path_refs: %s\n",
		strings.Join(record.EvidenceItemRefs, ","),
		strings.Join(record.EvidencePathRefs, ","),
	))
	builder.WriteString(fmt.Sprintf(
		"next_admissible_move: %s\n",
		record.NextAdmissibleMove,
	))
	builder.WriteString(fmt.Sprintf(
		"authority_boundary: proof=%s approval=%s gate_decision=%s work_occurrence=%s global_truth=%s\n",
		record.AuthorityBoundary.Proof,
		record.AuthorityBoundary.Approval,
		record.AuthorityBoundary.GateDecision,
		record.AuthorityBoundary.WorkOccurrence,
		record.AuthorityBoundary.GlobalTruth,
	))

	_, err := io.WriteString(w, builder.String())

	return err
}
