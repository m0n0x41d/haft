package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/fpf"
	"github.com/m0n0x41d/quint-code/logger"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI tool integration.

The server communicates via stdio and provides FPF tools to AI assistants
like Claude Code, Cursor, Gemini CLI, and Codex CLI.

The project root is determined by:
  1. QUINT_PROJECT_ROOT environment variable (if set)
  2. Current working directory (default)`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cwd := os.Getenv("QUINT_PROJECT_ROOT")
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	if err := logger.Init(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
	}
	defer logger.Close()

	quintDir := filepath.Join(cwd, ".quint")
	dbPath := filepath.Join(quintDir, "quint.db")

	var database *db.Store
	if _, err := os.Stat(dbPath); err == nil {
		database, err = db.NewStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to open database")
		}
	}

	var rawDB *sql.DB
	if database != nil {
		rawDB = database.GetRawDB()
	}

	fsm, err := fpf.LoadState("default", rawDB)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	tools := fpf.NewTools(fsm, cwd, database)
	server := fpf.NewServer(tools)

	// Wire v5 artifact handler
	if rawDB != nil {
		artStore := artifact.NewStore(rawDB)
		server.SetV5Handler(makeV5Handler(artStore, quintDir))
	}

	server.Start()

	return nil
}

func makeV5Handler(store *artifact.Store, quintDir string) fpf.V5ToolHandler {
	return func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		switch params.Name {
		case "quint_note":
			return handleQuintNote(ctx, store, quintDir, params.Arguments)
		case "quint_problem":
			return handleQuintProblem(ctx, store, quintDir, params.Arguments)
		case "quint_solution":
			return handleQuintSolution(ctx, store, quintDir, params.Arguments)
		default:
			return "", fmt.Errorf("unknown tool: %s", params.Name)
		}
	}
}

func handleQuintSolution(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "explore":
		input := artifact.ExploreInput{
			Context: contextName,
		}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}

		if variants, ok := args["variants"].([]interface{}); ok {
			for _, vRaw := range variants {
				vm, ok := vRaw.(map[string]interface{})
				if !ok {
					continue
				}
				v := artifact.Variant{}
				if s, ok := vm["title"].(string); ok {
					v.Title = s
				}
				if s, ok := vm["description"].(string); ok {
					v.Description = s
				}
				if s, ok := vm["weakest_link"].(string); ok {
					v.WeakestLink = s
				}
				if s, ok := vm["rollback_notes"].(string); ok {
					v.RollbackNotes = s
				}
				if b, ok := vm["stepping_stone"].(bool); ok {
					v.SteppingStone = b
				}
				if items, ok := vm["strengths"].([]interface{}); ok {
					for _, item := range items {
						if s, ok := item.(string); ok {
							v.Strengths = append(v.Strengths, s)
						}
					}
				}
				if items, ok := vm["risks"].([]interface{}); ok {
					for _, item := range items {
						if s, ok := item.(string); ok {
							v.Risks = append(v.Risks, s)
						}
					}
				}
				input.Variants = append(input.Variants, v)
			}
		}

		// Auto-detect problem if not specified
		if input.ProblemRef == "" {
			prob, _ := artifact.FindActiveProblem(ctx, store, contextName)
			if prob != nil {
				input.ProblemRef = prob.Meta.ID
			}
		}

		a, filePath, err := artifact.ExploreSolutions(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatSolutionResponse("explore", a, filePath, navStrip), nil

	case "compare":
		input := artifact.CompareInput{}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}

		// Parse comparison results
		if dims, ok := args["dimensions"].([]interface{}); ok {
			for _, d := range dims {
				if s, ok := d.(string); ok {
					input.Results.Dimensions = append(input.Results.Dimensions, s)
				}
			}
		}
		if scores, ok := args["scores"].(map[string]interface{}); ok {
			input.Results.Scores = make(map[string]map[string]string)
			for variantID, dimScores := range scores {
				if ds, ok := dimScores.(map[string]interface{}); ok {
					input.Results.Scores[variantID] = make(map[string]string)
					for dim, val := range ds {
						if s, ok := val.(string); ok {
							input.Results.Scores[variantID][dim] = s
						}
					}
				}
			}
		}
		if nds, ok := args["non_dominated_set"].([]interface{}); ok {
			for _, n := range nds {
				if s, ok := n.(string); ok {
					input.Results.NonDominatedSet = append(input.Results.NonDominatedSet, s)
				}
			}
		}
		if v, ok := args["policy_applied"].(string); ok {
			input.Results.PolicyApplied = v
		}
		if v, ok := args["selected_ref"].(string); ok {
			input.Results.SelectedRef = v
		}

		// Auto-detect portfolio if not specified
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			} else {
				navStrip := artifact.BuildNavStrip(ctx, store, contextName)
				return "No active SolutionPortfolio found.\nUse /q-explore to create variants first.\n" + navStrip, nil
			}
		}

		a, filePath, err := artifact.CompareSolutions(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatSolutionResponse("compare", a, filePath, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'explore' or 'compare'", action)
	}
}

func handleQuintProblem(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "frame":
		input := artifact.ProblemFrameInput{
			Context: contextName,
		}
		if v, ok := args["title"].(string); ok {
			input.Title = v
		}
		if v, ok := args["signal"].(string); ok {
			input.Signal = v
		}
		if v, ok := args["acceptance"].(string); ok {
			input.Acceptance = v
		}
		if v, ok := args["blast_radius"].(string); ok {
			input.BlastRadius = v
		}
		if v, ok := args["reversibility"].(string); ok {
			input.Reversibility = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		if items, ok := args["constraints"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.Constraints = append(input.Constraints, s)
				}
			}
		}
		if items, ok := args["optimization_targets"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.OptimizationTargets = append(input.OptimizationTargets, s)
				}
			}
		}
		if items, ok := args["observation_indicators"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.ObservationIndicators = append(input.ObservationIndicators, s)
				}
			}
		}

		a, filePath, err := artifact.FrameProblem(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatProblemResponse("frame", a, filePath, navStrip), nil

	case "characterize":
		input := artifact.CharacterizeInput{}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["parity_rules"].(string); ok {
			input.ParityRules = v
		}
		if dims, ok := args["dimensions"].([]interface{}); ok {
			for _, d := range dims {
				if dm, ok := d.(map[string]interface{}); ok {
					dim := artifact.ComparisonDimension{}
					if v, ok := dm["name"].(string); ok {
						dim.Name = v
					}
					if v, ok := dm["scale_type"].(string); ok {
						dim.ScaleType = v
					}
					if v, ok := dm["unit"].(string); ok {
						dim.Unit = v
					}
					if v, ok := dm["polarity"].(string); ok {
						dim.Polarity = v
					}
					if v, ok := dm["how_to_measure"].(string); ok {
						dim.HowToMeasure = v
					}
					input.Dimensions = append(input.Dimensions, dim)
				}
			}
		}

		// If no problem_ref, find the most recent active problem
		if input.ProblemRef == "" {
			prob, err := artifact.FindActiveProblem(ctx, store, contextName)
			if err != nil || prob == nil {
				return "No active ProblemCard found.\nUse /q-frame to create one first, then /q-char to add comparison dimensions.\n" +
					artifact.BuildNavStrip(ctx, store, contextName), nil
			}
			input.ProblemRef = prob.Meta.ID
		}

		a, filePath, err := artifact.CharacterizeProblem(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatProblemResponse("characterize", a, filePath, navStrip), nil

	case "select":
		limit := 20
		problems, err := artifact.SelectProblems(ctx, store, contextName, limit)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatProblemsListResponse(problems, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'frame', 'characterize', or 'select'", action)
	}
}

func handleQuintNote(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	input := artifact.NoteInput{}

	if v, ok := args["title"].(string); ok {
		input.Title = v
	}
	if v, ok := args["rationale"].(string); ok {
		input.Rationale = v
	}
	if v, ok := args["evidence"].(string); ok {
		input.Evidence = v
	}
	if v, ok := args["context"].(string); ok {
		input.Context = v
	}
	if files, ok := args["affected_files"].([]interface{}); ok {
		for _, f := range files {
			if s, ok := f.(string); ok {
				input.AffectedFiles = append(input.AffectedFiles, s)
			}
		}
	}

	// Validate
	validation := artifact.ValidateNote(ctx, store, input)

	navStrip := artifact.BuildNavStrip(ctx, store, input.Context)

	if !validation.OK {
		return artifact.FormatNoteRejection(validation, navStrip), nil
	}

	// Create
	a, filePath, err := artifact.CreateNote(ctx, store, quintDir, input)
	if err != nil {
		return "", err
	}

	return artifact.FormatNoteResponse(a, filePath, validation, navStrip), nil
}
