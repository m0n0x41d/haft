package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/logger"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server for AI tool integration.

The server communicates via stdio and provides Haft v5 tools to AI
assistants like Cursor, Gemini CLI, and Codex CLI.

The project root is determined by:
  1. HAFT_PROJECT_ROOT environment variable (if set)
  2. Current working directory (default)`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Ensure global ~/.haft/ exists (migrates from ~/.quint-code/ if needed)
	_ = project.EnsureDir()

	// Also check legacy env var
	cwd := os.Getenv("HAFT_PROJECT_ROOT")
	if cwd == "" {
		cwd = os.Getenv("QUINT_PROJECT_ROOT") // fallback for old configs
	}
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

	// HAFT_SERVER_ORIGIN: "local" (default) or URL for future remote server
	serverOrigin := os.Getenv("HAFT_SERVER_ORIGIN")
	if serverOrigin != "" && serverOrigin != "local" {
		logger.Info().Str("origin", serverOrigin).Msg("HAFT_SERVER_ORIGIN set to remote — not implemented yet, using local storage")
	}

	haftDir := filepath.Join(cwd, ".haft")

	server := fpf.NewServer()

	// Load project identity
	projCfg, err := project.Load(haftDir)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to load project config")
	}

	if projCfg != nil {
		// Unified storage: DB in ~/.haft/projects/{id}/
		dbPath, err := projCfg.DBPath()
		if err != nil {
			logger.Warn().Err(err).Msg("failed to determine DB path")
		} else if _, err := os.Stat(dbPath); err == nil {
			database, err := db.NewStore(dbPath)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to open database")
			} else {
				artStore := artifact.NewStore(database.GetRawDB())

				// Open cross-project index
				indexStore, indexErr := project.OpenIndex()
				if indexErr != nil {
					logger.Warn().Err(indexErr).Msg("failed to open cross-project index")
				}

				// Populate context_facts on startup
				_ = project.PopulateContextFacts(context.Background(), database.GetRawDB(), projCfg.Name)

				server.SetV5Handler(makeV5Handler(artStore, haftDir, projCfg, indexStore))
			}
		}
	} else {
		// Legacy: check for old .haft/haft.db (pre-migration)
		oldDBPath := filepath.Join(haftDir, "haft.db")
		if _, err := os.Stat(oldDBPath); err == nil {
			// Serve guard: block MCP and ask to re-run init
			server.SetV5Handler(func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
				return "", fmt.Errorf("haft storage has been upgraded, please run `haft init` in your project directory to migrate to unified storage, your data will be preserved")
			})
		}
	}

	server.Start()
	return nil
}

func makeV5Handler(store *artifact.Store, haftDir string, projCfg *project.Config, indexStore *project.IndexStore) fpf.V5ToolHandler {
	return func(ctx context.Context, toolName string, rawParams json.RawMessage) (string, error) {
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		action, _ := params.Arguments["action"].(string)
		logToolEntry(params.Name, action, params.Arguments)
		start := time.Now()

		// Dispatch
		result, toolErr := dispatchTool(ctx, store, haftDir, params.Name, params.Arguments)

		// Post-dispatch hooks
		logger.ToolResult(params.Name, action, time.Since(start).Milliseconds(), toolErr)

		if toolErr == nil {
			result = applyCrossProjectRecall(ctx, result, params.Name, action, params.Arguments, store, projCfg, indexStore)
			applyCrossProjectIndex(ctx, params.Name, action, params.Arguments, store, projCfg, indexStore)
		}

		logAudit(ctx, store.DB(), params.Name, action, params.Arguments, toolErr)

		if toolErr == nil {
			result = applyRefreshReminder(ctx, result, params.Name, store)
		}

		return result, toolErr
	}
}

// dispatchTool routes a tool call to its handler. Pure dispatch, no hooks.
func dispatchTool(ctx context.Context, store *artifact.Store, haftDir string, name string, args map[string]any) (string, error) {
	switch name {
	case "haft_note":
		return handleQuintNote(ctx, store, haftDir, args)
	case "haft_problem":
		return handleQuintProblem(ctx, store, haftDir, args)
	case "haft_solution":
		return handleQuintSolution(ctx, store, haftDir, args)
	case "haft_decision":
		return handleQuintDecision(ctx, store, haftDir, args)
	case "haft_refresh":
		return handleQuintRefresh(ctx, store, haftDir, args)
	case "haft_query":
		return handleQuintQuery(ctx, store, haftDir, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// logToolEntry logs the tool call entry with extracted refs.
func logToolEntry(name, action string, args map[string]any) {
	logParams := map[string]string{}
	if action != "" {
		logParams["action"] = action
	}
	for _, key := range []string{"decision_ref", "artifact_ref", "problem_ref"} {
		if ref, ok := args[key].(string); ok {
			logParams[key] = ref
		}
	}
	logger.ToolCall(name, action, logParams)
}

// applyCrossProjectRecall appends cross-project history to frame results.
func applyCrossProjectRecall(ctx context.Context, result, name, action string, args map[string]any, store *artifact.Store, projCfg *project.Config, indexStore *project.IndexStore) string {
	if name != "haft_problem" || action != "frame" || indexStore == nil || projCfg == nil {
		return result
	}
	signal, _ := args["signal"].(string)
	title, _ := args["title"].(string)
	query := title + " " + signal
	primaryLang := project.DetectPrimaryLanguage(store.DB())
	recalls, err := indexStore.Search(ctx, query, projCfg.ID, primaryLang, 3)
	if err != nil || len(recalls) == 0 {
		return result
	}
	result += "\n## Cross-Project History\n\n"
	for _, r := range recalls {
		clLabel := fmt.Sprintf("CL%d", r.CL)
		if r.CL == 2 {
			clLabel += " (similar context)"
		} else {
			clLabel += " (different context)"
		}
		result += fmt.Sprintf("- [%s] **%s** — %s (%s, from %s)\n",
			r.DecisionID, r.Title, truncateStr(r.WhySelected, 120), clLabel, r.ProjectName)
	}
	return result + "\n"
}

// applyCrossProjectIndex writes decision summaries to the global index on decide.
func applyCrossProjectIndex(ctx context.Context, name, action string, args map[string]any, store *artifact.Store, projCfg *project.Config, indexStore *project.IndexStore) {
	if name != "haft_decision" || action != "decide" || indexStore == nil || projCfg == nil {
		return
	}
	selectedTitle, _ := args["selected_title"].(string)
	if selectedTitle == "" {
		return
	}
	whySelected, _ := args["why_selected"].(string)
	weakestLink, _ := args["weakest_link"].(string)
	primaryLang := project.DetectPrimaryLanguage(store.DB())
	_ = indexStore.WriteDecision(ctx, project.IndexEntry{
		ProjectID:     projCfg.ID,
		ProjectName:   projCfg.Name,
		DecisionID:    selectedTitle,
		Title:         selectedTitle,
		SelectedTitle: selectedTitle,
		WhySelected:   whySelected,
		WeakestLink:   weakestLink,
		PrimaryLang:   primaryLang,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	logger.Debug().Str("project", projCfg.ID).Str("decision", selectedTitle).Msg("index.write")
}

// applyRefreshReminder appends a reminder if >5 days since last stale scan.
func applyRefreshReminder(ctx context.Context, result, name string, store *artifact.Store) string {
	if name == "haft_refresh" {
		return result
	}
	lastScan := store.LastRefreshScan(ctx)
	if lastScan.IsZero() {
		return result
	}
	daysSince := int(time.Since(lastScan).Hours() / 24)
	if daysSince >= 5 {
		result += fmt.Sprintf("\n\n--- Refresh reminder: %d days since last stale scan. Run haft_refresh(action=\"scan\") to check for stale decisions and evidence decay. ---\n", daysSince)
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// logAudit writes an audit_log row for every tool call. Errors are logged, never propagated.
func logAudit(ctx context.Context, rawDB *sql.DB, toolName, action string, args map[string]any, toolErr error) {
	operation := toolName
	if action != "" {
		operation = toolName + ":" + action
	}

	resultStr := "ok"
	if toolErr != nil {
		resultStr = "error: " + toolErr.Error()
	}

	// Extract target ID from common arg patterns
	targetID := ""
	for _, key := range []string{"artifact_ref", "decision_ref", "problem_ref", "portfolio_ref"} {
		if v, ok := args[key].(string); ok && v != "" {
			targetID = v
			break
		}
	}

	contextID := ""
	if v, ok := args["context"].(string); ok {
		contextID = v
	}

	id := fmt.Sprintf("audit-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)

	_, err := rawDB.ExecContext(ctx,
		`INSERT INTO audit_log (id, tool_name, operation, actor, target_id, result, context_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, toolName, operation, "agent", targetID, resultStr, contextID,
	)
	if err != nil {
		logger.Warn().Err(err).Str("tool", toolName).Msg("audit log write failed")
	}
}

func truncateMeasure(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// --- Tool handlers ---

func handleQuintNote(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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
	input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
	if v, ok := args["search_keywords"].(string); ok {
		input.SearchKeywords = v
	}

	validation := artifact.ValidateNote(ctx, store, input)
	navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, input.Context))

	if !validation.OK {
		return present.NoteRejection(validation, navStrip), nil
	}

	a, filePath, err := artifact.CreateNote(ctx, store, haftDir, input)
	if err != nil {
		// WriteWarning is non-fatal — surface warnings in response
		var ww *artifact.WriteWarning
		if errors.As(err, &ww) {
			validation.Warnings = append(validation.Warnings, ww.Warnings...)
		} else {
			return "", err
		}
	}
	return present.NoteResponse(a, filePath, validation, navStrip), nil
}

func handleQuintProblem(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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
		input.Constraints = parseStringArrayFromArgs(args, "constraints")
		input.OptimizationTargets = parseStringArrayFromArgs(args, "optimization_targets")
		input.ObservationIndicators = parseStringArrayFromArgs(args, "observation_indicators")

		a, filePath, err := artifact.FrameProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.ProblemResponse("frame", a, filePath, navStrip), nil

	case "characterize":
		input := artifact.CharacterizeInput{}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		if v, ok := args["parity_rules"].(string); ok {
			input.ParityRules = v
		}
		// Log all args keys and types for debugging
		for k, v := range args {
			logger.Debug().Str("key", k).Str("type", fmt.Sprintf("%T", v)).Msg("characterize arg")
		}
		input.Dimensions = parseDimensions(args["dimensions"])
		if input.ProblemRef == "" {
			prob, err := artifact.FindActiveProblem(ctx, store, contextName)
			if err != nil || prob == nil {
				return present.ApplyFPFAnswerHygiene(
					"No active ProblemCard found.\nUse /q-frame to create one first.\n" +
						present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)),
				), nil
			}
			input.ProblemRef = prob.Meta.ID
		}

		a, filePath, err := artifact.CharacterizeProblem(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.ProblemResponse("characterize", a, filePath, navStrip), nil

	case "select":
		problems, err := artifact.SelectProblems(ctx, store, contextName, 20)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		items := artifact.EnrichProblemsForList(ctx, store, problems)
		return present.ProblemsListResponse(items, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'frame', 'characterize', or 'select'", action)
	}
}

func handleQuintSolution(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
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
		input.Variants = parseVariants(args)
		if input.ProblemRef == "" {
			prob, _ := artifact.FindActiveProblem(ctx, store, contextName)
			if prob != nil {
				input.ProblemRef = prob.Meta.ID
			}
		}

		a, filePath, err := artifact.ExploreSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.SolutionResponse("explore", a, filePath, navStrip), nil

	case "compare":
		input := artifact.CompareInput{}
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		input.Results.Dimensions = parseStringArrayFromArgs(args, "dimensions")
		if scores := parseNestedStringMapFromArgs(args, "scores"); scores != nil {
			input.Results.Scores = scores
		}
		input.Results.NonDominatedSet = parseStringArrayFromArgs(args, "non_dominated_set")
		if v, ok := args["policy_applied"].(string); ok {
			input.Results.PolicyApplied = v
		}
		if v, ok := args["selected_ref"].(string); ok {
			input.Results.SelectedRef = v
		}
		if v, ok := args["recommendation_rationale"].(string); ok {
			input.Results.RecommendationRationale = v
		}
		_ = parseJSONArg(args, "dominated_variants", &input.Results.DominatedVariants)
		_ = parseJSONArg(args, "pareto_tradeoffs", &input.Results.ParetoTradeoffs)
		if input.PortfolioRef == "" {
			p, _ := artifact.FindActivePortfolio(ctx, store, contextName)
			if p != nil {
				input.PortfolioRef = p.Meta.ID
			} else {
				return present.ApplyFPFAnswerHygiene(
					"No active SolutionPortfolio found.\nUse /q-explore to create variants first.\n" +
						present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)),
				), nil
			}
		}

		a, filePath, err := artifact.CompareSolutions(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.SolutionResponse("compare", a, filePath, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'explore' or 'compare'", action)
	}
}

func handleQuintDecision(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)

	switch action {
	case "decide":
		input := artifact.DecideInput{Context: contextName}
		if v, ok := args["selected_title"].(string); ok {
			input.SelectedTitle = v
		}
		if v, ok := args["why_selected"].(string); ok {
			input.WhySelected = v
		}
		if v, ok := args["selection_policy"].(string); ok {
			input.SelectionPolicy = v
		}
		if v, ok := args["counterargument"].(string); ok {
			input.CounterArgument = v
		}
		if v, ok := args["weakest_link"].(string); ok {
			input.WeakestLink = v
		}
		if v, ok := args["problem_ref"].(string); ok {
			input.ProblemRef = v
		}
		input.ProblemRefs = parseStringArrayFromArgs(args, "problem_refs")
		if v, ok := args["portfolio_ref"].(string); ok {
			input.PortfolioRef = v
		}
		if v, ok := args["valid_until"].(string); ok {
			input.ValidUntil = v
		}
		if v, ok := args["mode"].(string); ok {
			input.Mode = v
		}
		input.Invariants = parseStringArrayFromArgs(args, "invariants")
		input.PreConditions = parseStringArrayFromArgs(args, "pre_conditions")
		input.PostConditions = parseStringArrayFromArgs(args, "post_conditions")
		input.Admissibility = parseStringArrayFromArgs(args, "admissibility")
		input.EvidenceReqs = parseStringArrayFromArgs(args, "evidence_requirements")
		input.RefreshTriggers = parseStringArrayFromArgs(args, "refresh_triggers")
		input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")
		if v, ok := args["search_keywords"].(string); ok {
			input.SearchKeywords = v
		}

		if rb, ok := args["rollback"].(map[string]any); ok {
			rollback := &artifact.RollbackSpec{}
			if items, ok := rb["triggers"].([]any); ok {
				for _, item := range items {
					if s, ok := item.(string); ok {
						rollback.Triggers = append(rollback.Triggers, s)
					}
				}
			}
			if items, ok := rb["steps"].([]any); ok {
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
		if items, ok := args["why_not_others"].([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
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

		a, filePath, err := artifact.Decide(ctx, store, haftDir, input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("decide", a, filePath, "", navStrip), nil

	case "apply":
		decisionRef, _ := args["decision_ref"].(string)
		if decisionRef == "" {
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				decisionRef = decisions[0].Meta.ID
			} else {
				return present.ApplyFPFAnswerHygiene(
					"No DecisionRecord found.\nUse /q-decide to finalize a decision first.\n" +
						present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)),
				), nil
			}
		}

		brief, err := artifact.Apply(ctx, store, decisionRef)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("apply", nil, "", brief, navStrip), nil

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
		input.CriteriaMet = parseStringArrayFromArgs(args, "criteria_met")
		input.CriteriaNotMet = parseStringArrayFromArgs(args, "criteria_not_met")
		input.Measurements = parseStringArrayFromArgs(args, "measurements")
		// Auto-detect decision
		if input.DecisionRef == "" {
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				input.DecisionRef = decisions[0].Meta.ID
			} else {
				return present.ApplyFPFAnswerHygiene(
					"No DecisionRecord found.\n" + present.NavStrip(artifact.ComputeNavState(ctx, store, contextName)),
				), nil
			}
		}

		a, err := artifact.Measure(ctx, store, haftDir, input)
		// Surface baseline gate warnings (not errors — measurement still recorded)
		var measureWarning string
		if ww, ok := err.(*artifact.WriteWarning); ok {
			for _, w := range ww.Warnings {
				measureWarning += w + "\n"
			}
			err = nil // warnings, not errors
		}
		if err != nil {
			return "", err
		}
		// Show WLNK summary after measurement
		wlnk := artifact.ComputeWLNKSummary(ctx, store, a.Meta.ID)
		extra := ""
		if measureWarning != "" {
			extra += measureWarning + "\n"
		}
		extra += fmt.Sprintf("WLNK: %s\n", wlnk.Summary)

		// Lemniscate feedback: failed/partial measurement → suggest reopen
		if input.Verdict == "failed" || input.Verdict == "partial" {
			extra += fmt.Sprintf("\nThis decision's measurement %s. Consider re-evaluating:\n", input.Verdict)
			extra += fmt.Sprintf("  haft_refresh(action=\"reopen\", artifact_ref=\"%s\", reason=\"measurement %s: %s\")\n",
				input.DecisionRef, input.Verdict, truncateMeasure(input.Findings, 80))
		}

		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("measure", a, "", extra, navStrip), nil

	case "evidence":
		input := artifact.EvidenceInput{
			CongruenceLevel: -1, // sentinel: "not provided", will default to 3
			FormalityLevel:  -1, // sentinel: "not provided", will default to 5
		}
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
		if v, ok := args["valid_until"].(string); ok {
			input.ValidUntil = v
		}
		if cl, ok := args["congruence_level"].(float64); ok {
			input.CongruenceLevel = int(cl)
		}
		if fl, ok := args["formality_level"].(float64); ok {
			input.FormalityLevel = int(fl)
		}

		item, err := artifact.AttachEvidence(ctx, store, input)
		if err != nil {
			return "", err
		}

		wlnk := artifact.ComputeWLNKSummary(ctx, store, input.ArtifactRef)
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		return present.DecisionResponse("evidence", nil, "",
			fmt.Sprintf("Evidence attached: %s [%s]\nVerdict: %s\nWLNK: %s\n", item.ID, item.Type, item.Verdict, wlnk.Summary),
			navStrip), nil

	case "baseline":
		input := artifact.BaselineInput{}
		if v, ok := args["decision_ref"].(string); ok {
			input.DecisionRef = v
		}
		if input.DecisionRef == "" {
			// Auto-detect: use the most recent decision
			decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 1)
			if len(decisions) > 0 {
				input.DecisionRef = decisions[0].Meta.ID
			}
		}
		input.AffectedFiles = parseStringArrayFromArgs(args, "affected_files")

		var baselineWarnings []string
		if len(input.AffectedFiles) > 0 {
			baselineWarnings = artifact.WarnSharedFiles(input.AffectedFiles)
		}

		files, err := artifact.Baseline(ctx, store, filepath.Dir(haftDir), input)
		if err != nil {
			return "", err
		}
		navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))
		result := present.BaselineResponse(input.DecisionRef, files, navStrip)
		for _, w := range baselineWarnings {
			result = "⚠ " + w + "\n" + result
		}
		return result, nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'decide', 'apply', 'measure', 'evidence', or 'baseline'", action)
	}
}

func handleQuintRefresh(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	reason, _ := args["reason"].(string)
	navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))

	// Support both artifact_ref (new) and decision_ref (backward compat)
	artifactRef, _ := args["artifact_ref"].(string)
	if artifactRef == "" {
		artifactRef, _ = args["decision_ref"].(string)
	}

	switch artifact.RefreshAction(action) {
	case artifact.RefreshScan:
		projectRoot := filepath.Dir(haftDir)

		// Rescan modules before drift detection — keeps dependency graph fresh
		scanner := codebase.NewScanner(store.DB())
		if _, err := scanner.ScanModules(ctx, projectRoot); err != nil {
			logger.Warn().Err(err).Msg("refresh: module rescan failed (non-fatal)")
		}
		if _, err := scanner.ScanDependencies(ctx, projectRoot); err != nil {
			_ = err // non-fatal
		}

		items, err := artifact.ScanStale(ctx, store, projectRoot)
		if err != nil {
			return "", err
		}
		result := present.ScanResponse(items, "")

		// Level C: enrich drift reports with dependency impact
		driftReports, _ := artifact.CheckDrift(ctx, store, projectRoot)
		for i, r := range driftReports {
			if !r.HasBaseline {
				continue
			}
			hasDrift := false
			var driftedFiles []string
			for _, f := range r.Files {
				if f.Status == artifact.DriftModified || f.Status == artifact.DriftAdded || f.Status == artifact.DriftMissing {
					hasDrift = true
					driftedFiles = append(driftedFiles, f.Path)
				}
			}
			if hasDrift && len(driftedFiles) > 0 {
				impacts, _ := codebase.EnrichDriftWithImpact(ctx, store.DB(), driftedFiles)
				if len(impacts) > 0 {
					for _, imp := range impacts {
						driftReports[i].ImpactedModules = append(driftReports[i].ImpactedModules, artifact.ModuleImpact{
							ModuleID:    imp.ModuleID,
							ModulePath:  imp.ModulePath,
							DecisionIDs: imp.DecisionIDs,
							IsBlind:     imp.IsBlind,
						})
					}
				}
			}
		}
		// If any drift has impact propagation, append the detailed report
		hasImpact := false
		for _, r := range driftReports {
			if len(r.ImpactedModules) > 0 {
				hasImpact = true
				break
			}
		}
		if hasImpact {
			result += "\n" + present.DriftResponse(driftReports, "")
		}

		return result + navStrip, nil

	case artifact.RefreshWaive:
		if artifactRef == "" {
			return "artifact_ref is required for waive.\n" + navStrip, nil
		}
		newValidUntil, _ := args["new_valid_until"].(string)
		evidence, _ := args["evidence"].(string)
		a, err := artifact.WaiveArtifact(ctx, store, haftDir, artifactRef, reason, newValidUntil, evidence)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReport(ctx, store, haftDir, artifactRef, "waive", reason, fmt.Sprintf("Extended to %s", a.Meta.ValidUntil))
		return present.RefreshActionResponse(artifact.RefreshWaive, a, nil, navStrip), nil

	case artifact.RefreshReopen:
		if artifactRef == "" {
			return present.ApplyFPFAnswerHygiene(
				"artifact_ref is required for reopen. Note: reopen only works on DecisionRecords.\n" + navStrip,
			), nil
		}
		dec, newProb, err := artifact.ReopenDecision(ctx, store, haftDir, artifactRef, reason)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReport(ctx, store, haftDir, artifactRef, "reopen", reason, fmt.Sprintf("New problem: %s", newProb.Meta.ID))
		return present.RefreshActionResponse(artifact.RefreshReopen, dec, newProb, navStrip), nil

	case artifact.RefreshSupersede:
		if artifactRef == "" {
			return "artifact_ref is required for supersede.\n" + navStrip, nil
		}
		newRef, _ := args["new_decision_ref"].(string)
		if newRef == "" {
			newRef, _ = args["new_artifact_ref"].(string)
		}
		a, err := artifact.SupersedeArtifact(ctx, store, haftDir, artifactRef, newRef, reason)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReport(ctx, store, haftDir, artifactRef, "supersede", reason, fmt.Sprintf("Replaced by %s", newRef))
		return present.RefreshActionResponse(artifact.RefreshSupersede, a, nil, navStrip), nil

	case artifact.RefreshDeprecate:
		if artifactRef == "" {
			return "artifact_ref is required for deprecate.\n" + navStrip, nil
		}
		a, err := artifact.DeprecateArtifact(ctx, store, haftDir, artifactRef, reason)
		if err != nil {
			return "", err
		}
		_, _ = artifact.CreateRefreshReport(ctx, store, haftDir, artifactRef, "deprecate", reason, "Artifact deprecated")
		return present.RefreshActionResponse(artifact.RefreshDeprecate, a, nil, navStrip), nil

	case artifact.RefreshReconcile:
		overlaps, err := artifact.Reconcile(ctx, store)
		if err != nil {
			return "", err
		}
		return present.ReconcileResponse(overlaps, navStrip), nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'scan', 'waive', 'reopen', 'supersede', 'deprecate', or 'reconcile'", action)
	}
}

func handleQuintQuery(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
	action, _ := args["action"].(string)
	contextName, _ := args["context"].(string)
	navStrip := present.NavStrip(artifact.ComputeNavState(ctx, store, contextName))

	switch action {
	case "search":
		query, _ := args["query"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		results, err := artifact.FetchSearchResults(ctx, store, query, limit)
		if err != nil {
			return "", err
		}
		return present.SearchResponse(results, query) + navStrip, nil

	case "status":
		data, err := artifact.FetchStatusData(ctx, store, contextName)
		if err != nil {
			return "", err
		}
		result := present.StatusResponse(data)
		// Append module coverage if modules are scanned
		scanner := codebase.NewScanner(store.DB())
		if !scanner.ModulesLastScanned(ctx).IsZero() {
			if report, err := codebase.ComputeCoverage(ctx, store.DB()); err == nil && report.TotalModules > 0 {
				result += "\n" + codebase.FormatCoverageResponse(report)
			}
		}
		return result + navStrip, nil

	case "related":
		file, _ := args["file"].(string)
		results, err := artifact.FetchRelatedArtifacts(ctx, store, file)
		if err != nil {
			return "", err
		}
		return present.RelatedResponse(results, file) + navStrip, nil

	case "projection":
		viewName, _ := args["view"].(string)
		view, err := artifact.ParseProjectionView(viewName)
		if err != nil {
			return "", err
		}
		graph, err := artifact.FetchProjectionGraph(ctx, store, contextName)
		if err != nil {
			return "", err
		}
		return present.ProjectionResponse(graph, view) + navStrip, nil

	case "list":
		kind, _ := args["kind"].(string)
		limit := 50
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		data, err := artifact.FetchListData(ctx, store, kind, limit)
		if err != nil {
			return "", err
		}
		return present.ListResponse(data) + navStrip, nil

	case "coverage":
		projectRoot := filepath.Dir(haftDir)
		scanner := codebase.NewScanner(store.DB())

		// Always rescan — module detection is fast (<100ms)
		if _, err := scanner.ScanModules(ctx, projectRoot); err != nil {
			return "", fmt.Errorf("module scan: %w", err)
		}
		if _, err := scanner.ScanDependencies(ctx, projectRoot); err != nil {
			_ = err // non-fatal
		}

		report, err := codebase.ComputeCoverage(ctx, store.DB())
		if err != nil {
			return "", fmt.Errorf("compute coverage: %w", err)
		}
		return codebase.FormatCoverageResponse(report) + navStrip, nil

	case "fpf":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query is required for fpf search")
		}
		limit := fpf.DefaultSpecSearchLimit
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		full, _ := args["full"].(bool)
		explain, _ := args["explain"].(bool)
		retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
			Query: query,
			Limit: limit,
			Full:  full,
		})
		if err != nil {
			return "", fmt.Errorf("fpf search: %w", err)
		}
		if len(retrieval.Results) == 0 {
			return formatMCPFPFSearchWithExplain(nil, explain) + navStrip, nil
		}

		return formatMCPFPFSearchWithExplain(presentFPFRetrieval(retrieval.Results), explain) + navStrip, nil

	default:
		return "", fmt.Errorf("unknown action %q — use 'search', 'status', 'related', 'list', 'coverage', or 'fpf'", action)
	}
}

// parseStringArrayFromArgs handles MCP client serialization differences.
// Some clients send JSON arrays as parsed []any, others as raw JSON strings.
func parseStringArrayFromArgs(args map[string]any, key string) []string {
	if items, ok := args[key].([]any); ok {
		var result []string
		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if s, ok := args[key].(string); ok && len(s) > 0 && s[0] == '[' {
		logger.Debug().Str("key", key).Str("raw_type", "string").Msg("parseStringArrayFromArgs: JSON string fallback")
		var parsed []string
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}

// parseDimensions handles MCP client serialization of comparison dimensions.
// Some MCP clients may send the array as:
//   - []any (parsed JSON array) — standard case
//   - string (JSON-encoded string) — when the client serializes nested arrays as strings
func parseDimensions(raw any) []artifact.ComparisonDimension {
	var items []map[string]any

	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				items = append(items, m)
			}
		}
	case string:
		// JSON string fallback: "[{...}, {...}]"
		if len(v) > 0 && v[0] == '[' {
			var parsed []map[string]any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				items = parsed
			}
		}
	case nil:
		return nil
	default:
		// Try JSON marshal/unmarshal roundtrip as last resort
		data, err := json.Marshal(raw)
		if err == nil {
			var parsed []map[string]any
			if json.Unmarshal(data, &parsed) == nil {
				items = parsed
			}
		}
	}

	var dims []artifact.ComparisonDimension
	for _, dm := range items {
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
		if dim.Name != "" {
			dims = append(dims, dim)
		}
	}
	return dims
}

// parseNestedStringMapFromArgs handles MCP client serialization of map[string]map[string]string.
// Some clients send JSON objects as parsed map[string]any, others as raw JSON strings.
func parseNestedStringMapFromArgs(args map[string]any, key string) map[string]map[string]string {
	var raw map[string]any
	if m, ok := args[key].(map[string]any); ok {
		raw = m
	} else if s, ok := args[key].(string); ok && len(s) > 0 && s[0] == '{' {
		logger.Debug().Str("key", key).Str("raw_type", "string").Msg("parseNestedStringMapFromArgs: JSON string fallback")
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return nil
		}
	}
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]map[string]string, len(raw))
	for outerKey, innerVal := range raw {
		if inner, ok := innerVal.(map[string]any); ok {
			result[outerKey] = make(map[string]string, len(inner))
			for k, v := range inner {
				if s, ok := v.(string); ok {
					result[outerKey][k] = s
				}
			}
		}
	}
	return result
}

func parseJSONArg(args map[string]any, key string, target any) bool {
	value, ok := args[key]
	if !ok {
		return false
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false
	}

	return true
}

// parseVariants handles MCP client serialization of the variants array.
// Accepts both parsed []any and raw JSON string formats.
func parseVariants(args map[string]any) []artifact.Variant {
	var raw []any

	if items, ok := args["variants"].([]any); ok {
		raw = items
	} else if s, ok := args["variants"].(string); ok && len(s) > 0 && s[0] == '[' {
		logger.Debug().Str("key", "variants").Str("raw_type", "string").Msg("parseVariants: JSON string fallback")
		// Try direct unmarshal into []Variant first
		var parsed []artifact.Variant
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
		// Fall back to generic unmarshal
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			logger.Warn().Str("key", "variants").Err(err).Msg("parseVariants: failed to parse JSON string")
			return nil
		}
	}

	if len(raw) == 0 {
		return nil
	}

	var variants []artifact.Variant
	for _, vRaw := range raw {
		vm, ok := vRaw.(map[string]any)
		if !ok {
			continue
		}
		v := artifact.Variant{}
		if s, ok := vm["title"].(string); ok {
			v.Title = s
		}
		if s, ok := vm["id"].(string); ok {
			v.ID = s
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
		if items, ok := vm["strengths"].([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					v.Strengths = append(v.Strengths, s)
				}
			}
		}
		if items, ok := vm["risks"].([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					v.Risks = append(v.Risks, s)
				}
			}
		}
		variants = append(variants, v)
	}
	return variants
}
