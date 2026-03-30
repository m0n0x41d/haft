// Package tools — haft kernel tool executors for the haft agent.
// These wrap the same L4 kernel functions as the MCP server.
// One kernel, two transports: MCP (for Claude Code) and direct (for haft).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

// ---------------------------------------------------------------------------
// HaftProblemTool — left cycle: frame problems, characterize, select
// ---------------------------------------------------------------------------

type HaftProblemTool struct {
	store    artifact.ArtifactStore
	haftDir string
}

func NewHaftProblemTool(store artifact.ArtifactStore, haftDir string) *HaftProblemTool {
	return &HaftProblemTool{store: store, haftDir: haftDir}
}

func (t *HaftProblemTool) Name() string { return "haft_problem" }

func (t *HaftProblemTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_problem",
		Description: `Frame engineering problems before solving them.

Actions:
- frame: Create a ProblemCard with signal (what's broken), constraints, acceptance criteria.
  This is the LEFT CYCLE of the lemniscate — understand before implementing.
- adopt: Continue work on an existing problem from a previous session.
  Pass the problem ref (e.g. "prob-20260329-008") to pick up where it left off.
  The cycle will start from the phase that matches existing artifacts.
- select: List active problems with readiness signals (Goldilocks assessment).
- characterize: Add comparison dimensions to a problem (what to measure, how).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":                 map[string]any{"type": "string", "enum": []string{"frame", "adopt", "select", "characterize"}, "description": "frame | adopt | select | characterize"},
				"ref":                    map[string]any{"type": "string", "description": "Existing problem ID to adopt (adopt)"},
				"title":                  map[string]any{"type": "string", "description": "Problem title (frame)"},
				"signal":                 map[string]any{"type": "string", "description": "What's anomalous or broken (frame)"},
				"constraints":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Hard limits (frame)"},
				"optimization_targets":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What to improve, 1-3 max (frame)"},
				"observation_indicators": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Watch but don't optimize (frame)"},
				"acceptance":             map[string]any{"type": "string", "description": "How we'll know it's solved (frame)"},
				"blast_radius":           map[string]any{"type": "string", "description": "What's affected (frame)"},
				"reversibility":          map[string]any{"type": "string", "description": "How easy to undo (frame)"},
				"mode":                   map[string]any{"type": "string", "description": "tactical | standard | deep (frame)"},
				"problem_ref":            map[string]any{"type": "string", "description": "Problem ID to characterize; if omitted, use the current active problem"},
				"parity_rules":           map[string]any{"type": "string", "description": "What must be equal across variants for fair comparison (characterize)"},
				"dimensions": map[string]any{
					"type":        "array",
					"description": "Comparison dimensions to persist on the problem (characterize)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":           map[string]any{"type": "string"},
							"scale_type":     map[string]any{"type": "string"},
							"unit":           map[string]any{"type": "string"},
							"polarity":       map[string]any{"type": "string"},
							"role":           map[string]any{"type": "string"},
							"how_to_measure": map[string]any{"type": "string"},
							"valid_until":    map[string]any{"type": "string"},
						},
					},
				},
			},
			"required": []any{"action"},
		},
	}
}

func (t *HaftProblemTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "frame":
		input := artifact.ProblemFrameInput{
			Title:                 jsonStr(args, "title"),
			Signal:                jsonStr(args, "signal"),
			Acceptance:            jsonStr(args, "acceptance"),
			BlastRadius:           jsonStr(args, "blast_radius"),
			Reversibility:         jsonStr(args, "reversibility"),
			Mode:                  jsonStr(args, "mode"),
			Constraints:           jsonStrArray(args, "constraints"),
			OptimizationTargets:   jsonStrArray(args, "optimization_targets"),
			ObservationIndicators: jsonStrArray(args, "observation_indicators"),
		}
		a, filePath, err := artifact.FrameProblem(ctx, t.store, t.haftDir, input)
		if err != nil {
			return agent.ToolResult{}, err
		}
		display := fmt.Sprintf("Problem framed: %s\nID: %s\nFile: %s", a.Meta.Title, a.Meta.ID, filePath)
		return agent.ToolResult{
			DisplayText: display,
			Meta: &agent.ArtifactMeta{
				Kind:        "problem",
				ArtifactRef: a.Meta.ID,
				Operation:   "frame",
			},
		}, nil

	case "adopt":
		ref := jsonStr(args, "ref")
		if ref == "" {
			return agent.ToolResult{}, fmt.Errorf("ref required: pass an existing problem ID (e.g. prob-20260329-008)")
		}
		// Verify the problem exists
		a, err := t.store.Get(ctx, ref)
		if err != nil {
			return agent.PlainResult(fmt.Sprintf("Problem '%s' not found. Use haft_problem(select) to list active problems.", ref)), nil
		}
		if a.Meta.Kind != artifact.KindProblemCard {
			return agent.PlainResult(fmt.Sprintf("'%s' is a %s, not a ProblemCard.", ref, a.Meta.Kind)), nil
		}

		// Find related solution and decision artifacts
		portfolioRef := ""
		decisionRef := ""
		related, _ := artifact.FetchSearchResults(ctx, t.store, ref, 20)
		for _, r := range related {
			switch r.Meta.Kind {
			case artifact.KindSolutionPortfolio:
				if portfolioRef == "" {
					portfolioRef = r.Meta.ID
				}
			case artifact.KindDecisionRecord:
				if decisionRef == "" {
					decisionRef = r.Meta.ID
				}
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Adopted problem: %s\nID: %s\n", a.Meta.Title, a.Meta.ID)
		if portfolioRef != "" {
			fmt.Fprintf(&b, "Found portfolio: %s\n", portfolioRef)
		}
		if decisionRef != "" {
			fmt.Fprintf(&b, "Found decision: %s\n", decisionRef)
		}

		return agent.ToolResult{
			DisplayText: b.String(),
			Meta: &agent.ArtifactMeta{
				Kind:              "problem",
				ArtifactRef:       a.Meta.ID,
				Operation:         "adopt",
				AdoptPortfolioRef: portfolioRef,
				AdoptDecisionRef:  decisionRef,
			},
		}, nil

	case "select":
		problems, err := artifact.SelectProblems(ctx, t.store, "", 10)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if len(problems) == 0 {
			return agent.PlainResult("No active problems found."), nil
		}
		var b strings.Builder
		items := artifact.EnrichProblemsForList(ctx, t.store, problems)
		for _, item := range items {
			fmt.Fprintf(&b, "- [%s] %s (%s) %s\n", item.Problem.Meta.ID, item.Problem.Meta.Title, item.Problem.Meta.Status, item.Signals)
		}
		return agent.PlainResult(b.String()), nil

	case "characterize":
		problemRef := jsonStr(args, "problem_ref")
		if problemRef == "" {
			// Try to get from active cycle
			if t.store != nil {
				problems, _ := artifact.SelectProblems(ctx, t.store, "", 1)
				if len(problems) > 0 {
					problemRef = problems[0].Meta.ID
				}
			}
		}
		if problemRef == "" {
			return agent.PlainResult("problem_ref required. Frame a problem first, then characterize."), nil
		}

		input := artifact.CharacterizeInput{
			ProblemRef:  problemRef,
			ParityRules: jsonStr(args, "parity_rules"),
		}

		// Parse dimensions
		if dims, ok := args["dimensions"]; ok {
			data, _ := json.Marshal(dims)
			var parsed []artifact.ComparisonDimension
			if json.Unmarshal(data, &parsed) == nil {
				input.Dimensions = parsed
			}
		}

		if len(input.Dimensions) == 0 {
			return agent.PlainResult("At least one dimension required for characterization."), nil
		}

		a, filePath, err := artifact.CharacterizeProblem(ctx, t.store, t.haftDir, input)
		if err != nil {
			return agent.ToolResult{}, err
		}

		var dimNames []string
		for _, d := range input.Dimensions {
			label := d.Name
			if d.Role != "" {
				label += " (" + d.Role + ")"
			}
			dimNames = append(dimNames, label)
		}
		display := fmt.Sprintf("Problem characterized: %s\nDimensions: %s\nFile: %s",
			a.Meta.ID, strings.Join(dimNames, ", "), filePath)
		return agent.PlainResult(display), nil

	default:
		return agent.ToolResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

// ---------------------------------------------------------------------------
// HaftSolutionTool — right cycle: explore variants, compare
// ---------------------------------------------------------------------------

type HaftSolutionTool struct {
	store    artifact.ArtifactStore
	haftDir string
	registry *Registry // for cycle access (guardrails)
}

func NewHaftSolutionTool(store artifact.ArtifactStore, haftDir string, registry *Registry) *HaftSolutionTool {
	return &HaftSolutionTool{store: store, haftDir: haftDir, registry: registry}
}

func (t *HaftSolutionTool) Name() string { return "haft_solution" }

func (t *HaftSolutionTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_solution",
		Description: `Explore and compare solution variants.

Actions:
- explore: Generate 2+ genuinely distinct approaches with strengths, weakest link, and risks.
  Each variant must differ in KIND, not degree. This is creative abduction.
- compare: Fair comparison of variants on explicit dimensions. Identify the Pareto front.
- similar: Search past solution portfolios for patterns matching a query. Reuse proven approaches.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":        map[string]any{"type": "string", "enum": []string{"explore", "compare", "similar"}, "description": "explore | compare | similar"},
				"query":         map[string]any{"type": "string", "description": "Search query for similar past solutions (similar)"},
				"problem_ref":   map[string]any{"type": "string", "description": "Problem ID to solve (explore)"},
				"portfolio_ref": map[string]any{"type": "string", "description": "Portfolio ID to compare (compare)"},
				"variants": map[string]any{
					"type":        "array",
					"description": "Solution variants (explore). Each needs: title, description, weakest_link, strengths[], risks[]",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":          map[string]any{"type": "string"},
							"description":    map[string]any{"type": "string"},
							"weakest_link":   map[string]any{"type": "string"},
							"strengths":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"risks":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"stepping_stone": map[string]any{"type": "boolean"},
						},
					},
				},
				"dimensions":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Comparison dimensions (compare)"},
				"scores":            map[string]any{"type": "object", "description": "Scores per variant per dimension (compare)"},
				"non_dominated_set": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Pareto front variants (compare)"},
				"selected_ref":      map[string]any{"type": "string", "description": "Selected variant (compare)"},
				"policy_applied":    map[string]any{"type": "string", "description": "Selection rule used (compare)"},
			},
			"required": []any{"action"},
		},
	}
}

func (t *HaftSolutionTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "explore":
		// FPF guardrail: requires problem frame
		if t.registry != nil {
			if err := agent.CanExplore(t.registry.ActiveCycle(ctx)); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
		}
		return t.explore(ctx, args)
	case "compare":
		// FPF guardrail: requires solution portfolio
		if t.registry != nil {
			if err := agent.CanCompare(t.registry.ActiveCycle(ctx)); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
		}
		return t.compare(ctx, args)
	case "similar":
		query := jsonStr(args, "query")
		if query == "" {
			return agent.ToolResult{}, fmt.Errorf("query required for similar search")
		}
		results, err := artifact.FetchSearchResults(ctx, t.store, query, 10)
		if err != nil {
			return agent.ToolResult{}, err
		}
		var matches []string
		for _, r := range results {
			if r.Meta.Kind == artifact.KindSolutionPortfolio {
				matches = append(matches, fmt.Sprintf("- [%s] %s (problem: %s)",
					r.Meta.ID, r.Meta.Title, r.Meta.Context))
			}
		}
		if len(matches) == 0 {
			return agent.PlainResult("No similar past solutions found. This is a novel problem."), nil
		}
		return agent.PlainResult(fmt.Sprintf("Past solution portfolios matching \"%s\":\n%s\n\nUse haft_query(search) for details on any portfolio.",
			query, strings.Join(matches, "\n"))), nil
	default:
		return agent.ToolResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *HaftSolutionTool) explore(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	input := artifact.ExploreInput{
		ProblemRef: jsonStr(args, "problem_ref"),
		Context:    jsonStr(args, "context"),
		Mode:       jsonStr(args, "mode"),
	}

	if variants, ok := args["variants"]; ok {
		data, _ := json.Marshal(variants)
		var parsed []artifact.Variant
		if json.Unmarshal(data, &parsed) == nil {
			input.Variants = parsed
		}
	}

	if len(input.Variants) < 2 {
		return agent.ToolResult{}, fmt.Errorf("at least 2 variants required for exploration")
	}

	a, filePath, err := artifact.ExploreSolutions(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Solution portfolio created: %s\nID: %s\nFile: %s\n\n", a.Meta.Title, a.Meta.ID, filePath)
	fmt.Fprintf(&b, "Variants:\n")
	for _, v := range input.Variants {
		fmt.Fprintf(&b, "- %s (WLNK: %s)\n", v.Title, v.WeakestLink)
	}
	return agent.ToolResult{
		DisplayText: b.String(),
		Meta: &agent.ArtifactMeta{
			Kind:        "solution",
			ArtifactRef: a.Meta.ID,
			Operation:   "explore",
		},
	}, nil
}

func (t *HaftSolutionTool) compare(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	input := artifact.CompareInput{
		PortfolioRef: jsonStr(args, "portfolio_ref"),
	}
	input.Results.Dimensions = jsonStrArray(args, "dimensions")
	input.Results.NonDominatedSet = jsonStrArray(args, "non_dominated_set")
	input.Results.SelectedRef = jsonStr(args, "selected_ref")
	input.Results.PolicyApplied = jsonStr(args, "policy_applied")

	if scores, ok := args["scores"]; ok {
		data, _ := json.Marshal(scores)
		var parsed map[string]map[string]string
		if json.Unmarshal(data, &parsed) == nil {
			input.Results.Scores = parsed
		}
	}

	a, filePath, err := artifact.CompareSolutions(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	display := fmt.Sprintf("Comparison recorded.\nID: %s\nFile: %s\nSelected: %s", a.Meta.ID, filePath, input.Results.SelectedRef)
	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:        "solution",
			ArtifactRef: a.Meta.ID,
			Operation:   "compare",
		},
	}, nil
}

// ---------------------------------------------------------------------------
// HaftDecisionTool — decide with rationale, measure with evidence
// ---------------------------------------------------------------------------

type HaftDecisionTool struct {
	store       artifact.ArtifactStore
	haftDir     string
	projectRoot string
	registry    *Registry
}

func NewHaftDecisionTool(store artifact.ArtifactStore, haftDir, projectRoot string, registry *Registry) *HaftDecisionTool {
	return &HaftDecisionTool{store: store, haftDir: haftDir, projectRoot: projectRoot, registry: registry}
}

func (t *HaftDecisionTool) Name() string { return "haft_decision" }

func (t *HaftDecisionTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_decision",
		Description: `Record decisions and measurements.

Actions:
- decide: Record a formal decision with rationale (FPF E.9 DRR).
  Includes: selected variant, why selected, invariants, rollback plan, weakest link.
- baseline: Snapshot affected files after implementation and before measurement.
- measure: Record measurement results against acceptance criteria.
  Closes the lemniscate cycle with inductive evidence.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":          map[string]any{"type": "string", "enum": []string{"decide", "baseline", "measure"}, "description": "decide | baseline | measure"},
				"problem_ref":     map[string]any{"type": "string", "description": "Problem ID (decide)"},
				"portfolio_ref":   map[string]any{"type": "string", "description": "Portfolio ID (decide)"},
				"selected_title":  map[string]any{"type": "string", "description": "Chosen variant title (decide)"},
				"why_selected":    map[string]any{"type": "string", "description": "Rationale for selection (decide)"},
				"invariants":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What MUST hold (decide)"},
				"post_conditions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Implementation checklist (decide)"},
				"admissibility":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What is NOT acceptable (decide)"},
				"weakest_link":    map[string]any{"type": "string", "description": "What bounds quality (decide)"},
				"predictions": map[string]any{
					"type":        "array",
					"description": "Testable predictions — measure will check each one (decide)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"claim":      map[string]any{"type": "string", "description": "What should be true after implementation"},
							"observable": map[string]any{"type": "string", "description": "How to verify (test name, command, file check)"},
							"threshold":  map[string]any{"type": "string", "description": "What counts as passing"},
						},
					},
				},
				"affected_files":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Files affected (decide/baseline)"},
				"valid_until":      map[string]any{"type": "string", "description": "Expiry date YYYY-MM-DD (decide)"},
				"decision_ref":     map[string]any{"type": "string", "description": "Decision ID (measure)"},
				"findings":         map[string]any{"type": "string", "description": "What was observed (measure)"},
				"criteria_met":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Passing criteria (measure)"},
				"criteria_not_met": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Failing criteria (measure)"},
				"verdict":          map[string]any{"type": "string", "enum": []string{"accepted", "partial", "failed"}, "description": "Verdict (measure)"},
				"mode":             map[string]any{"type": "string", "description": "tactical | standard | deep"},
			},
			"required": []any{"action"},
		},
	}
}

func (t *HaftDecisionTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "decide":
		// FPF guardrails: requires explored variants + user consent (Transformer Mandate)
		if t.registry != nil {
			if err := agent.CanDecide(t.registry.ActiveCycle(ctx), t.registry.UserConsented(ctx)); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
		}
		return t.decide(ctx, args)
	case "baseline":
		return t.baseline(ctx, args)
	case "measure":
		// FPF guardrail: requires decision
		if t.registry != nil {
			if err := agent.CanMeasure(t.registry.ActiveCycle(ctx)); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
		}
		return t.measure(ctx, args)
	default:
		return agent.ToolResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

func (t *HaftDecisionTool) decide(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	input := artifact.DecideInput{
		ProblemRef:     jsonStr(args, "problem_ref"),
		PortfolioRef:   jsonStr(args, "portfolio_ref"),
		SelectedTitle:  jsonStr(args, "selected_title"),
		WhySelected:    jsonStr(args, "why_selected"),
		WeakestLink:    jsonStr(args, "weakest_link"),
		ValidUntil:     jsonStr(args, "valid_until"),
		Context:        jsonStr(args, "context"),
		Mode:           jsonStr(args, "mode"),
		Invariants:     jsonStrArray(args, "invariants"),
		PostConditions: jsonStrArray(args, "post_conditions"),
		Admissibility:  jsonStrArray(args, "admissibility"),
		AffectedFiles:  jsonStrArray(args, "affected_files"),
	}

	// Parse predictions
	if preds, ok := args["predictions"]; ok {
		data, _ := json.Marshal(preds)
		var parsed []artifact.PredictionInput
		if json.Unmarshal(data, &parsed) == nil {
			input.Predictions = parsed
		}
	}

	a, filePath, err := artifact.Decide(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	display := fmt.Sprintf("Decision recorded: %s\nID: %s\nFile: %s\nSelected: %s", a.Meta.Title, a.Meta.ID, filePath, input.SelectedTitle)
	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:        "decision",
			ArtifactRef: a.Meta.ID,
			Operation:   "decide",
		},
	}, nil
}

func (t *HaftDecisionTool) baseline(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	decisionRef := jsonStr(args, "decision_ref")
	files, err := artifact.Baseline(ctx, t.store, t.projectRoot, artifact.BaselineInput{
		DecisionRef:   decisionRef,
		AffectedFiles: jsonStrArray(args, "affected_files"),
	})
	if err != nil {
		return agent.ToolResult{}, err
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	display := fmt.Sprintf("Baseline recorded: %s\nFiles: %s", decisionRef, strings.Join(paths, ", "))
	return agent.ToolResult{DisplayText: display}, nil
}

func (t *HaftDecisionTool) measure(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	decisionRef := jsonStr(args, "decision_ref")

	if decisionRef == "" {
		return agent.PlainResult("No decision_ref provided. In tactical mode (no formal decision recorded), " +
			"report your findings as text in your response instead of calling this tool. " +
			"Only use haft_decision(measure) after haft_decision(decide) has been called."), nil
	}

	a, err := t.store.Get(ctx, decisionRef)
	if err != nil {
		return agent.PlainResult(fmt.Sprintf("Decision '%s' not found. If you're in tactical mode, report findings as text instead.", decisionRef)), nil
	}
	if a.Meta.Kind != artifact.KindDecisionRecord {
		return agent.PlainResult(fmt.Sprintf("'%s' is a %s, not a DecisionRecord. You likely passed a problem ID. "+
			"In tactical mode, report your findings as text instead of calling this tool.", decisionRef, a.Meta.Kind)), nil
	}

	files, err := t.store.GetAffectedFiles(ctx, decisionRef)
	if err == nil && len(files) > 0 {
		hasBaseline := false
		for _, f := range files {
			if f.Hash != "" {
				hasBaseline = true
				break
			}
		}
		if !hasBaseline {
			return agent.PlainResult(fmt.Sprintf(
				"Decision '%s' has affected_files but no baseline yet. Run haft_decision(action=\"baseline\", decision_ref=%q) after implementation, then verify, then measure.",
				decisionRef, decisionRef,
			)), nil
		}
	}

	input := artifact.MeasureInput{
		DecisionRef:    decisionRef,
		Findings:       jsonStr(args, "findings"),
		Verdict:        jsonStr(args, "verdict"),
		CriteriaMet:    jsonStrArray(args, "criteria_met"),
		CriteriaNotMet: jsonStrArray(args, "criteria_not_met"),
	}

	result, err := artifact.Measure(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	display := fmt.Sprintf("Measurement recorded: verdict=%s\nArtifact: %s", input.Verdict, result.Meta.ID)
	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:           "decision",
			ArtifactRef:    result.Meta.ID,
			Operation:      "measure",
			MeasureVerdict: input.Verdict,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// HaftQueryTool — search, status, related decisions
// ---------------------------------------------------------------------------

// FPFSearchFunc is a callback that searches the FPF specification.
// Returns formatted results or error. Injected by cmd/agent.go to avoid
// importing the embedded DB from internal packages.
type FPFSearchFunc func(query string, limit int) (string, error)

type HaftQueryTool struct {
	store     artifact.ArtifactStore
	fpfSearch FPFSearchFunc
}

func NewHaftQueryTool(store artifact.ArtifactStore, fpfSearch FPFSearchFunc) *HaftQueryTool {
	return &HaftQueryTool{store: store, fpfSearch: fpfSearch}
}

func (t *HaftQueryTool) Name() string { return "haft_query" }

func (t *HaftQueryTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_query",
		Description: `Search past decisions, check project status, find related artifacts, look up FPF spec.

Actions:
- search: FTS5 keyword search across all artifacts.
- status: Compact dashboard — shipped/pending decisions, stale items, coverage.
- related: Find decisions affecting a specific file.
- fpf: Search the FPF specification for formal definitions, aggregation rules, and patterns.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"search", "status", "related", "fpf"}, "description": "search | status | related | fpf"},
				"query":  map[string]any{"type": "string", "description": "Search terms (search, fpf)"},
				"file":   map[string]any{"type": "string", "description": "File path (related)"},
			},
			"required": []any{"action"},
		},
	}
}

func (t *HaftQueryTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "search":
		query := jsonStr(args, "query")
		if query == "" {
			return agent.ToolResult{}, fmt.Errorf("query required")
		}
		results, err := artifact.FetchSearchResults(ctx, t.store, query, 10)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if len(results) == 0 {
			return agent.PlainResult("No results found."), nil
		}
		var b strings.Builder
		for _, r := range results {
			fmt.Fprintf(&b, "- [%s] %s (%s)\n", r.Meta.ID, r.Meta.Title, r.Meta.Kind)
		}
		return agent.PlainResult(b.String()), nil

	case "status":
		data, err := artifact.FetchStatusData(ctx, t.store, "")
		if err != nil {
			return agent.ToolResult{}, err
		}
		return agent.PlainResult(present.StatusResponse(data)), nil

	case "related":
		file := jsonStr(args, "file")
		if file == "" {
			return agent.ToolResult{}, fmt.Errorf("file required")
		}
		results, err := artifact.FetchRelatedArtifacts(ctx, t.store, file)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if len(results) == 0 {
			return agent.PlainResult("No decisions found for this file."), nil
		}
		var b strings.Builder
		for _, r := range results {
			fmt.Fprintf(&b, "- [%s] %s\n", r.Meta.ID, r.Meta.Title)
		}
		return agent.PlainResult(b.String()), nil

	case "fpf":
		query := jsonStr(args, "query")
		if query == "" {
			return agent.ToolResult{}, fmt.Errorf("query required for fpf search")
		}
		if t.fpfSearch == nil {
			return agent.ToolResult{}, fmt.Errorf("FPF spec search not available")
		}
		result, err := t.fpfSearch(query, 5)
		if err != nil {
			return agent.ToolResult{}, fmt.Errorf("fpf search: %w", err)
		}
		return agent.PlainResult(result), nil

	default:
		return agent.ToolResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

// ---------------------------------------------------------------------------
// HaftRefreshTool — artifact lifecycle management
// ---------------------------------------------------------------------------

type HaftRefreshTool struct {
	store       artifact.ArtifactStore
	haftDir    string
	projectRoot string
}

func NewHaftRefreshTool(store artifact.ArtifactStore, haftDir, projectRoot string) *HaftRefreshTool {
	return &HaftRefreshTool{store: store, haftDir: haftDir, projectRoot: projectRoot}
}

func (t *HaftRefreshTool) Name() string { return "haft_refresh" }

func (t *HaftRefreshTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_refresh",
		Description: `Manage artifact lifecycle — detect stale decisions, check drift.

Actions:
- scan: Find stale artifacts (expired valid_until, evidence decay).
- drift: Check if files under decisions have changed since baseline.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"scan", "drift"}, "description": "scan | drift"},
			},
			"required": []any{"action"},
		},
	}
}

func (t *HaftRefreshTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}

	action, _ := args["action"].(string)

	switch action {
	case "scan":
		items, err := artifact.ScanStale(ctx, t.store, t.projectRoot)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if len(items) == 0 {
			return agent.PlainResult("No stale artifacts found."), nil
		}
		return agent.PlainResult(present.ScanResponse(items, "")), nil

	case "drift":
		reports, err := artifact.CheckDrift(ctx, t.store, t.projectRoot)
		if err != nil {
			return agent.ToolResult{}, err
		}
		if len(reports) == 0 {
			return agent.PlainResult("No drift detected."), nil
		}
		return agent.PlainResult(present.DriftResponse(reports, "")), nil

	default:
		return agent.ToolResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// HaftNoteTool — micro-decisions during coding
// ---------------------------------------------------------------------------

type HaftNoteTool struct {
	store    artifact.ArtifactStore
	haftDir string
}

func NewHaftNoteTool(store artifact.ArtifactStore, haftDir string) *HaftNoteTool {
	return &HaftNoteTool{store: store, haftDir: haftDir}
}

func (t *HaftNoteTool) Name() string { return "haft_note" }

func (t *HaftNoteTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name:        "haft_note",
		Description: "Record a micro-decision with rationale. Use when you make a choice during coding — library selection, config approach, naming convention. Quick and lightweight.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":          map[string]any{"type": "string", "description": "What was decided"},
				"rationale":      map[string]any{"type": "string", "description": "Why this choice (required — no rationale = rejected)"},
				"affected_files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Files affected"},
			},
			"required": []any{"title", "rationale"},
		},
	}
}

func (t *HaftNoteTool) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return agent.ToolResult{}, fmt.Errorf("parse args: %w", err)
	}
	input := artifact.NoteInput{
		Title:         jsonStr(args, "title"),
		Rationale:     jsonStr(args, "rationale"),
		AffectedFiles: jsonStrArray(args, "affected_files"),
	}
	validation := artifact.ValidateNote(ctx, t.store, input)
	if !validation.OK {
		return agent.PlainResult(fmt.Sprintf("Note rejected: %s", strings.Join(validation.Warnings, "; "))), nil
	}
	a, filePath, err := artifact.CreateNote(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}
	display := fmt.Sprintf("Note recorded: %s\nID: %s\nFile: %s", a.Meta.Title, a.Meta.ID, filePath)
	if len(validation.Warnings) > 0 {
		display += "\nWarnings: " + strings.Join(validation.Warnings, "; ")
	}
	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:        "note",
			ArtifactRef: a.Meta.ID,
			Operation:   "note",
		},
	}, nil
}

// JSON helpers
// ---------------------------------------------------------------------------

func jsonStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func jsonStrArray(args map[string]any, key string) []string {
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
		var parsed []string
		if json.Unmarshal([]byte(s), &parsed) == nil {
			return parsed
		}
	}
	return nil
}
