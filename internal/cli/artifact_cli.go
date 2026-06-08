package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	artifactCreateInputFile string
	artifactCreateJSON      bool
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Create Haft artifacts from structured input files",
}

var artifactCreateCmd = &cobra.Command{
	Use:   "create CAPABILITY",
	Short: "Create or update one Haft artifact from a JSON input file",
	Long: `Create or update one Haft artifact from a JSON input file.

Use ` + "`haft interface <capability> --json`" + ` to retrieve the compact
contract before writing the input file. Supported capabilities:
problem.frame, solution.explore, solution.compare, decision.decide, note.record.`,
	Args: cobra.ExactArgs(1),
	RunE: runArtifactCreate,
}

type artifactCreateResult struct {
	Capability string   `json:"capability"`
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	File       string   `json:"file,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
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

	projectRoot, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	haftDir := filepath.Join(projectRoot, ".haft")
	inputBytes, err := os.ReadFile(artifactCreateInputFile)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}

	result, err := createArtifactFromInput(context.Background(), store, haftDir, capability, inputBytes)
	if err != nil {
		return err
	}

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

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("open DB: %w", err)
	}

	closeStore := func() {
		_ = sqlDB.Close()
	}

	return projectRoot, artifact.NewStore(sqlDB), closeStore, nil
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
		created, filePath, err := artifact.FrameProblem(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
	case "solution.explore":
		var input artifact.ExploreInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		created, filePath, err := artifact.ExploreSolutions(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
	case "solution.compare":
		var input artifactCompareFileInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		created, filePath, err := artifact.CompareSolutions(ctx, store, haftDir, input.toCompareInput())
		return artifactResult(capability, created, filePath), err
	case "decision.decide":
		var input artifact.DecideInput
		if err := decodeArtifactInput(inputBytes, &input); err != nil {
			return artifactCreateResult{}, err
		}
		created, filePath, err := artifact.Decide(ctx, store, haftDir, input)
		return artifactResult(capability, created, filePath), err
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
		File:       filePath,
	}
}

func writeArtifactCreateText(output io.Writer, result artifactCreateResult) error {
	_, err := fmt.Fprintf(output, "%s created %s `%s`\n", result.Capability, result.Kind, result.ID)
	return err
}
