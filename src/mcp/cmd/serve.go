package cmd

import (
	"context"
	"encoding/json"
	"errors"
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

The server communicates via stdio and provides Quint Code v5 tools to AI
assistants like Claude Code, Cursor, Gemini CLI, and Codex CLI.

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

	server := fpf.NewServer()

	// Wire v5 artifact handler if DB exists
	if _, err := os.Stat(dbPath); err == nil {
		database, err := db.NewStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to open database")
		} else {
			artStore := artifact.NewStore(database.GetRawDB())
			server.SetV5Handler(makeV5Handler(artStore, quintDir))
		}
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
		case "quint_decision":
			return handleQuintDecision(ctx, store, quintDir, params.Arguments)
		case "quint_refresh":
			return handleQuintRefresh(ctx, store, quintDir, params.Arguments)
		case "quint_query":
			return handleQuintQuery(ctx, store, params.Arguments)
		default:
			return "", fmt.Errorf("unknown tool: %s", params.Name)
		}
	}
}

// --- Tool handlers ---

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

	validation := artifact.ValidateNote(ctx, store, input)
	navStrip := artifact.BuildNavStrip(ctx, store, input.Context)

	if !validation.OK {
		return artifact.FormatNoteRejection(validation, navStrip), nil
	}

	a, filePath, err := artifact.CreateNote(ctx, store, quintDir, input)
	if err != nil {
		// WriteWarning is non-fatal — surface warnings in response
		var ww *artifact.WriteWarning
		if errors.As(err, &ww) {
			for _, w := range ww.Warnings {
				validation.Warnings = append(validation.Warnings, w)
			}
		} else {
			return "", err
		}
	}
	return artifact.FormatNoteResponse(a, filePath, validation, navStrip), nil
}

func handleQuintProblem(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "frame":
		input := artifact.ProblemFrameInput{Context: contextName}
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
		for _, key := range []string{"constraints", "optimization_targets", "observation_indicators"} {
			if items, ok := args[key].([]interface{}); ok {
				var strs []string
				for _, item := range items {
					if s, ok := item.(string); ok {
						strs = append(strs, s)
					}
				}
				switch key {
				case "constraints":
					input.Constraints = strs
				case "optimization_targets":
					input.OptimizationTargets = strs
				case "observation_indicators":
					input.ObservationIndicators = strs
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
		if input.ProblemRef == "" {
			prob, err := artifact.FindActiveProblem(ctx, store, contextName)
			if err != nil || prob == nil {
				return "No active ProblemCard found.\nUse /q-frame to create one first.\n" +
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
		problems, err := artifact.SelectProblems(ctx, store, contextName, 20)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatProblemsListResponse(problems, store, ctx, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'frame', 'characterize', or 'select'", action)
	}
}

func handleQuintSolution(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "explore":
		input := artifact.ExploreInput{Context: contextName}
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
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			} else {
				return "No active SolutionPortfolio found.\nUse /q-explore to create variants first.\n" +
					artifact.BuildNavStrip(ctx, store, contextName), nil
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

func handleQuintDecision(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	parseStringArray := func(key string) []string {
		var result []string
		if items, ok := args[key].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
		}
		return result
	}

	switch action {
	case "decide":
		input := artifact.DecideInput{Context: contextName}
		if v, ok := args["selected_title"].(string); ok {
			input.SelectedTitle = v
		}
		if v, ok := args["why_selected"].(string); ok {
			input.WhySelected = v
		}
		if v, ok := args["weakest_link"].(string); ok {
			input.WeakestLink = v
		}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		if v, ok := args["valid_until"].(string); ok {
			input.ValidUntil = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		input.Invariants = parseStringArray("invariants")
		input.PreConditions = parseStringArray("pre_conditions")
		input.PostConditions = parseStringArray("post_conditions")
		input.Admissibility = parseStringArray("admissibility")
		input.EvidenceReqs = parseStringArray("evidence_requirements")
		input.RefreshTriggers = parseStringArray("refresh_triggers")
		input.AffectedFiles = parseStringArray("affected_files")

		if rb, ok := args["rollback"].(map[string]interface{}); ok {
			rollback := &artifact.RollbackSpec{}
			if items, ok := rb["triggers"].([]interface{}); ok {
				for _, item := range items {
					if s, ok := item.(string); ok {
						rollback.Triggers = append(rollback.Triggers, s)
					}
				}
			}
			if items, ok := rb["steps"].([]interface{}); ok {
				for _, item := range items {
					if s, ok := item.(string); ok {
						rollback.Steps = append(rollback.Steps, s)
					}
				}
			}
			if v, ok := rb["blast_radius"].(string); ok {
				rollback.BlastRadius = v
			}
			input.Rollback = rollback
		}
		if items, ok := args["why_not_others"].([]interface{}); ok {
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					rr := artifact.RejectionReason{}
					if v, ok := m["variant"].(string); ok {
						rr.Variant = v
					}
					if v, ok := m["reason"].(string); ok {
						rr.Reason = v
					}
					input.WhyNotOthers = append(input.WhyNotOthers, rr)
				}
			}
		}
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			}
		}
		if input.ProblemRef == "" {
			p, _ := artifact.FindActiveProblem(ctx, store, contextName)
			if p != nil {
				input.ProblemRef = p.Meta.ID
			}
		}

		a, filePath, err := artifact.Decide(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatDecisionResponse("decide", a, filePath, "", navStrip), nil

	case "apply":
		decisionRef, _ := args["decision_ref"].(string)
		if decisionRef == "" {
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				decisionRef = decisions[0].Meta.ID
			} else {
				return "No DecisionRecord found.\nUse /q-decide to finalize a decision first.\n" +
					artifact.BuildNavStrip(ctx, store, contextName), nil
			}
		}

		brief, err := artifact.Apply(ctx, store, decisionRef)
		if err != nil {
			return "", err
		}
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatDecisionResponse("apply", nil, "", brief, navStrip), nil

	case "measure":
		input := artifact.MeasureInput{}
		if v, ok := args["decision_ref"].(string); ok {
			input.DecisionRef = v
		}
		if v, ok := args["findings"].(string); ok {
			input.Findings = v
		}
		if v, ok := args["verdict"].(string); ok {
			input.Verdict = v
		}
		if items, ok := args["criteria_met"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.CriteriaMet = append(input.CriteriaMet, s)
				}
			}
		}
		if items, ok := args["criteria_not_met"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.CriteriaNotMet = append(input.CriteriaNotMet, s)
				}
			}
		}
		if items, ok := args["measurements"].([]interface{}); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					input.Measurements = append(input.Measurements, s)
				}
			}
		}
		// Auto-detect decision
		if input.DecisionRef == "" {
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				input.DecisionRef = decisions[0].Meta.ID
			} else {
				return "No DecisionRecord found.\n" + artifact.BuildNavStrip(ctx, store, contextName), nil
			}
		}

		a, err := artifact.Measure(ctx, store, quintDir, input)
		if err != nil {
			return "", err
		}
		// Show WLNK summary after measurement
		wlnk := artifact.ComputeWLNKSummary(ctx, store, a.Meta.ID)
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatDecisionResponse("measure", a, "", fmt.Sprintf("WLNK: %s\n", wlnk.Summary), navStrip), nil

	case "evidence":
		input := artifact.EvidenceInput{}
		if v, ok := args["artifact_ref"].(string); ok {
			input.ArtifactRef = v
		}
		if v, ok := args["evidence_content"].(string); ok {
			input.Content = v
		}
		if v, ok := args["evidence_type"].(string); ok {
			input.Type = v
		}
		if v, ok := args["evidence_verdict"].(string); ok {
			input.Verdict = v
		}
		if v, ok := args["carrier_ref"].(string); ok {
			input.CarrierRef = v
		}
		if cl, ok := args["congruence_level"].(float64); ok {
			input.CongruenceLevel = int(cl)
		}

		item, err := artifact.AttachEvidence(ctx, store, input)
		if err != nil {
			return "", err
		}

		wlnk := artifact.ComputeWLNKSummary(ctx, store, input.ArtifactRef)
		navStrip := artifact.BuildNavStrip(ctx, store, contextName)
		return artifact.FormatDecisionResponse("evidence", nil, "",
			fmt.Sprintf("Evidence attached: %s [%s]\nVerdict: %s\nWLNK: %s\n", item.ID, item.Type, item.Verdict, wlnk.Summary),
			navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'decide', 'apply', 'measure', or 'evidence'", action)
	}
}

func handleQuintRefresh(ctx context.Context, store *artifact.Store, quintDir string, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	decisionRef, _ := args["decision_ref"].(string)
	reason, _ := args["reason"].(string)
	navStrip := artifact.BuildNavStrip(ctx, store, contextName)

	switch artifact.RefreshAction(action) {
	case artifact.RefreshScan:
		items, err := artifact.ScanStale(ctx, store)
		if err != nil {
			return "", err
		}
		return artifact.FormatScanResponse(items, navStrip), nil

	case artifact.RefreshWaive:
		if decisionRef == "" {
			return "decision_ref is required for waive.\n" + navStrip, nil
		}
		newValidUntil, _ := args["new_valid_until"].(string)
		evidence, _ := args["evidence"].(string)
		dec, err := artifact.WaiveDecision(ctx, store, quintDir, decisionRef, reason, newValidUntil, evidence)
		if err != nil {
			return "", err
		}
		artifact.CreateRefreshReport(ctx, store, quintDir, decisionRef, "waive", reason, fmt.Sprintf("Extended to %s", dec.Meta.ValidUntil))
		return artifact.FormatRefreshActionResponse(artifact.RefreshWaive, dec, nil, navStrip), nil

	case artifact.RefreshReopen:
		if decisionRef == "" {
			return "decision_ref is required for reopen.\n" + navStrip, nil
		}
		dec, newProb, err := artifact.ReopenDecision(ctx, store, quintDir, decisionRef, reason)
		if err != nil {
			return "", err
		}
		artifact.CreateRefreshReport(ctx, store, quintDir, decisionRef, "reopen", reason, fmt.Sprintf("New problem: %s", newProb.Meta.ID))
		return artifact.FormatRefreshActionResponse(artifact.RefreshReopen, dec, newProb, navStrip), nil

	case artifact.RefreshSupersede:
		if decisionRef == "" {
			return "decision_ref is required for supersede.\n" + navStrip, nil
		}
		newDecRef, _ := args["new_decision_ref"].(string)
		dec, err := artifact.SupersedeDecision(ctx, store, quintDir, decisionRef, newDecRef, reason)
		if err != nil {
			return "", err
		}
		artifact.CreateRefreshReport(ctx, store, quintDir, decisionRef, "supersede", reason, fmt.Sprintf("Replaced by %s", newDecRef))
		return artifact.FormatRefreshActionResponse(artifact.RefreshSupersede, dec, nil, navStrip), nil

	case artifact.RefreshDeprecate:
		if decisionRef == "" {
			return "decision_ref is required for deprecate.\n" + navStrip, nil
		}
		dec, err := artifact.DeprecateDecision(ctx, store, quintDir, decisionRef, reason)
		if err != nil {
			return "", err
		}
		artifact.CreateRefreshReport(ctx, store, quintDir, decisionRef, "deprecate", reason, "Decision deprecated")
		return artifact.FormatRefreshActionResponse(artifact.RefreshDeprecate, dec, nil, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'scan', 'waive', 'reopen', 'supersede', or 'deprecate'", action)
	}
}

func handleQuintQuery(ctx context.Context, store *artifact.Store, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	navStrip := artifact.BuildNavStrip(ctx, store, contextName)

	switch action {
	case "search":
		query, _ := args["query"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		result, err := artifact.QuerySearch(ctx, store, query, limit)
		if err != nil {
			return "", err
		}
		return result + navStrip, nil

	case "status":
		result, err := artifact.QueryStatus(ctx, store, contextName)
		if err != nil {
			return "", err
		}
		return result + navStrip, nil

	case "related":
		file, _ := args["file"].(string)
		result, err := artifact.QueryRelated(ctx, store, file)
		if err != nil {
			return "", err
		}
		return result + navStrip, nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'search', 'status', or 'related'", action)
	}
}
