package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

var (
	artifactCreateInputFile string
	artifactCreateJSON      bool
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Create Haft artifacts through their dedicated effect sinks",
}

var artifactCreateCmd = &cobra.Command{
	Use:   "create CAPABILITY",
	Short: "Create or update one Haft artifact from a JSON input file",
	Long: `Create or update one Haft artifact from a JSON input file.

Use ` + "`haft interface <capability> --json`" + ` to retrieve the compact
contract before writing the input file. Supported capabilities:
problem.frame, solution.explore, solution.compare, decision.decide, note.record.
For decision.decide, a supported host routes one direct and unambiguous
operator request to this internal effect sink. The command records
host_routed_operator_request provenance and never treats a skill token,
recommendation, quotation, or generated tool payload as authority.`,
	Args: cobra.ExactArgs(1),
	RunE: runArtifactCreate,
}

type artifactCreateResult struct {
	Capability           string                      `json:"capability"`
	ID                   string                      `json:"id"`
	Kind                 string                      `json:"kind"`
	Title                string                      `json:"title,omitempty"`
	File                 string                      `json:"file,omitempty"`
	ExactReplay          bool                        `json:"exact_replay,omitempty"`
	Warnings             []string                    `json:"warnings,omitempty"`
	TaskMemoryProjection *taskMemoryProjectionReport `json:"task_memory_projection,omitempty"`
}

type artifactCompareFileInput struct {
	PortfolioRef            string                                 `json:"portfolio_ref,omitempty"`
	Results                 artifact.ComparisonResult              `json:"results,omitempty"`
	Dimensions              []string                               `json:"dimensions,omitempty"`
	Scores                  map[string]map[string]string           `json:"scores,omitempty"`
	NonDominatedSet         []string                               `json:"non_dominated_set,omitempty"`
	Incomparable            [][]string                             `json:"incomparable,omitempty"`
	DominatedVariants       []artifact.DominatedVariantExplanation `json:"dominated_variants,omitempty"`
	ParetoTradeoffs         []artifact.ParetoTradeoffNote          `json:"pareto_tradeoffs,omitempty"`
	PolicyApplied           string                                 `json:"policy_applied,omitempty"`
	LegacyRecommendationRef string                                 `json:"legacy_recommendation_ref,omitempty"`
	SelectedRef             string                                 `json:"selected_ref,omitempty"`
	RecommendationRationale string                                 `json:"recommendation_rationale,omitempty"`
	ParityPlan              *artifact.ParityPlan                   `json:"parity_plan,omitempty"`
}

func init() {
	artifactCreateCmd.Flags().StringVar(&artifactCreateInputFile, "input-file", "", "JSON input file matching the capability contract")
	artifactCreateCmd.Flags().BoolVar(&artifactCreateJSON, "json", false, "print structured JSON output")
	artifactCmd.AddCommand(artifactCreateCmd)
	rootCmd.AddCommand(artifactCmd)
}

func runArtifactCreate(cmd *cobra.Command, args []string) error {
	capability := args[0]
	if artifactCreateInputFile == "" {
		return fmt.Errorf("--input-file is required; run `haft interface %s --json` for the input contract", capability)
	}

	inputBytes, err := os.ReadFile(artifactCreateInputFile)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}
	if capability == "decision.decide" {
		return runDecisionArtifactCreate(cmd, inputBytes)
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ledger, err := openCurrentProjectLedger(
		ctx,
		projectRoot,
		projectledger.ReadWrite,
		"artifact creation",
	)
	if err != nil {
		return err
	}
	defer ledger.Close()
	projectRoot = ledger.ProjectRoot().String()
	if err := ledger.Revalidate(ctx); err != nil {
		return fmt.Errorf(
			"revalidate checked project ledger before artifact creation: %w",
			err,
		)
	}
	store := artifact.NewStore(ledger.Database())
	haftDir := filepath.Join(projectRoot, ".haft")
	result, err := createArtifactFromInput(
		ctx,
		store,
		haftDir,
		capability,
		inputBytes,
	)
	if err != nil {
		return err
	}
	rawArguments := map[string]any{}
	if err := json.Unmarshal(inputBytes, &rawArguments); err != nil {
		return fmt.Errorf(
			"decode task-memory projection arguments: %w",
			err,
		)
	}
	projector, projectionErr := newTaskMemoryProjectionRuntime(
		ctx,
		ledger.ProjectID(),
		ledger.Database(),
		store,
	)
	var taskProjector taskMemoryProjector = projector
	if projectionErr != nil {
		taskProjector = newUnavailableTaskMemoryProjector(
			projectionErr,
		)
	}
	projectionRequest := artifactTaskMemoryProjectionRequest(
		capability,
		result.ID,
		rawArguments,
	)
	projection, applicable := taskProjector.Project(
		ctx,
		projectionRequest,
	)
	if applicable {
		result.TaskMemoryProjection = &projection
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return fmt.Errorf(
			"artifact %s was created, but checked-ledger verification after task-memory projection failed: %w",
			result.ID,
			err,
		)
	}

	if artifactCreateJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}

	return writeArtifactCreateText(cmd.OutOrStdout(), result)
}

func artifactTaskMemoryProjectionRequest(
	capability string,
	artifactRef string,
	arguments map[string]any,
) taskMemoryProjectionRequest {
	routes := map[string]taskMemoryProjectionRequest{
		"problem.frame": {
			ToolName: "haft_problem",
			Action:   "frame",
		},
		"solution.explore": {
			ToolName: "haft_solution",
			Action:   "explore",
		},
		"solution.compare": {
			ToolName: "haft_solution",
			Action:   "compare",
		},
		"note.record": {
			ToolName: "haft_note",
			Action:   "record",
		},
	}
	request := routes[capability]
	request.ArtifactRef = artifactRef
	request.Arguments = arguments
	request.Mode = taskMemoryProjectionApply
	return request
}

func runDecisionArtifactCreate(cmd *cobra.Command, inputBytes []byte) error {
	input := artifact.DecideInput{}
	if err := decodeArtifactInput(inputBytes, &input); err != nil {
		return err
	}
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	bound, err := bindDecisionFromHostRequest(
		ctx,
		projectRoot,
		input,
	)
	if err != nil {
		return err
	}
	result := decisionBindingArtifactResult(bound)
	if artifactCreateJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	return writeArtifactCreateText(cmd.OutOrStdout(), result)
}

func openArtifactCLIStore() (string, *artifact.Store, func(), error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("load project config: %w", err)
	}
	if cfg == nil {
		return "", nil, func() {}, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("get DB path: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("open DB: %w", err)
	}

	closeStore := func() {
		_ = database.Close()
	}

	return projectRoot, artifact.NewStore(database.GetRawDB()), closeStore, nil
}

func createArtifactFromInput(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	capability string,
	inputBytes []byte,
) (artifactCreateResult, error) {
	switch capability {
	case "problem.frame":
		var input artifact.ProblemFrameInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		input = applyProblemSpecFit(ctx, store, haftDir, input)
		created, filePath, err := artifact.FrameProblem(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
	case "solution.explore":
		var input artifact.ExploreInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		input = applyExploreSpecFit(ctx, store, haftDir, input)
		created, filePath, err := artifact.ExploreSolutions(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
	case "solution.compare":
		var input artifactCompareFileInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		compareInput := applyCompareSpecFit(ctx, store, haftDir, input.toCompareInput())
		created, filePath, err := artifact.CompareSolutions(ctx, store, haftDir, compareInput)
		return artifactResult(capability, created, filePath), err
	case "decision.decide":
		return artifactCreateResult{}, errDecisionBindingUnavailable
	case "note.record":
		var input artifact.NoteInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		created, filePath, err := artifact.CreateNote(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
	default:
		return artifactCreateResult{}, fmt.Errorf("unknown artifact capability %q — run `haft interface --json`", capability)
	}
}

func decodeArtifactInput(inputBytes []byte, output any) error {
	if err := json.Unmarshal(inputBytes, output); err != nil {
		return fmt.Errorf("decode input JSON: %w", err)
	}

	return nil
}

func (input artifactCompareFileInput) toCompareInput() artifact.CompareInput {
	results := input.Results
	if len(results.Dimensions) == 0 {
		results.Dimensions = input.Dimensions
	}
	if len(results.Scores) == 0 {
		results.Scores = input.Scores
	}
	if len(results.NonDominatedSet) == 0 {
		results.NonDominatedSet = input.NonDominatedSet
	}
	if len(results.Incomparable) == 0 {
		results.Incomparable = input.Incomparable
	}
	if len(results.DominatedVariants) == 0 {
		results.DominatedVariants = input.DominatedVariants
	}
	if len(results.ParetoTradeoffs) == 0 {
		results.ParetoTradeoffs = input.ParetoTradeoffs
	}
	if results.PolicyApplied == "" {
		results.PolicyApplied = input.PolicyApplied
	}
	if results.LegacyRecommendationRef == "" {
		results.LegacyRecommendationRef = input.LegacyRecommendationRef
	}
	if results.SelectedRef == "" {
		results.SelectedRef = input.SelectedRef
	}
	if results.RecommendationRationale == "" {
		results.RecommendationRationale = input.RecommendationRationale
	}
	if results.ParityPlan == nil {
		results.ParityPlan = input.ParityPlan
	}

	return artifact.CompareInput{
		PortfolioRef: input.PortfolioRef,
		Results:      results,
	}
}

func artifactResult(capability string, created *artifact.Artifact, filePath string) artifactCreateResult {
	if created == nil {
		return artifactCreateResult{Capability: capability, File: filePath}
	}

	return artifactCreateResult{
		Capability: capability,
		ID:         created.Meta.ID,
		Kind:       string(created.Meta.Kind),
		Title:      created.Meta.Title,
		File:       filePath,
	}
}

func writeArtifactCreateText(output io.Writer, result artifactCreateResult) error {
	verb := "created"
	if result.ExactReplay {
		verb = "confirmed"
	}
	builder := strings.Builder{}
	_, _ = fmt.Fprintf(
		&builder,
		"%s %s %s `%s` — %s\n",
		result.Capability,
		verb,
		result.Kind,
		result.ID,
		result.Title,
	)
	for _, warning := range result.Warnings {
		_, _ = fmt.Fprintf(&builder, "warning: %s\n", warning)
	}
	if result.TaskMemoryProjection != nil {
		_, _ = fmt.Fprintf(
			&builder,
			"typed memory: %s",
			result.TaskMemoryProjection.AdmissionResult,
		)
		if result.TaskMemoryProjection.RelationDeclarationFragmentID != "" {
			_, _ = fmt.Fprintf(
				&builder,
				" (%s)",
				result.TaskMemoryProjection.RelationDeclarationFragmentID,
			)
		}
		_, _ = fmt.Fprintln(&builder)
	}
	_, err := io.WriteString(output, builder.String())
	return err
}
