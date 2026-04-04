// Package tools — haft kernel tool executors for the haft agent.
// These wrap the same L4 kernel functions as the MCP server.
// One kernel, two transports: MCP (for external agents) and direct (for haft agent).
package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/present"
)

// ---------------------------------------------------------------------------
// HaftProblemTool — left cycle: frame problems, characterize, select
// ---------------------------------------------------------------------------

type HaftProblemTool struct {
	store   artifact.ArtifactStore
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
							"valid_until":    map[string]any{"type": "string", "description": "Dimension freshness deadline (RFC3339 or YYYY-MM-DD)"},
						},
					},
				},
				"parity_plan": parityPlanSchema("Structured parity plan for fair comparison (characterize)"),
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
		comparedPortfolioRef := ""
		decisionRef := ""
		related, _ := artifact.FetchSearchResults(ctx, t.store, ref, 20)
		for _, r := range related {
			switch r.Meta.Kind {
			case artifact.KindSolutionPortfolio:
				if portfolioRef == "" {
					portfolioRef = r.Meta.ID
					comparedPortfolioRef = resolveComparedPortfolioRef(ctx, t.store, r.Meta.ID)
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
				Kind:                 "problem",
				ArtifactRef:          a.Meta.ID,
				Operation:            "adopt",
				AdoptPortfolioRef:    portfolioRef,
				ComparedPortfolioRef: comparedPortfolioRef,
				AdoptDecisionRef:     decisionRef,
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

		_ = jsonDecodeArg(args, "dimensions", &input.Dimensions)
		if parityPlan, ok := jsonParityPlan(args, "parity_plan"); ok {
			input.ParityPlan = parityPlan
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
	haftDir  string
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
					"description": "Solution variants (explore). Each needs title, weakest_link, and novelty_marker. If stepping_stone=true, stepping_stone_basis is required.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":                   map[string]any{"type": "string"},
							"title":                map[string]any{"type": "string"},
							"description":          map[string]any{"type": "string"},
							"weakest_link":         map[string]any{"type": "string"},
							"novelty_marker":       map[string]any{"type": "string"},
							"strengths":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"risks":                map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"stepping_stone":       map[string]any{"type": "boolean"},
							"stepping_stone_basis": map[string]any{"type": "string"},
							"diversity_role":       map[string]any{"type": "string"},
							"assumption_notes":     map[string]any{"type": "string"},
							"rollback_notes":       map[string]any{"type": "string"},
							"evidence_refs":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						},
					},
				},
				"no_stepping_stone_rationale": map[string]any{"type": "string", "description": "Required when no variant is a stepping stone"},
				"dimensions":                  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Comparison dimensions (compare)"},
				"scores":                      map[string]any{"type": "object", "description": "Scores per variant per dimension (compare)"},
				"non_dominated_set":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Advisory Pareto-front claim (compare). Runtime computes and stores the front from scores."},
				"incomparable":                map[string]any{"type": "array", "items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "description": "Pairs that are intentionally incomparable (compare)"},
				"dominated_variants": map[string]any{
					"type":        "array",
					"description": "Persisted elimination reasoning for dominated variants (compare)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"variant":      map[string]any{"type": "string"},
							"dominated_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"summary":      map[string]any{"type": "string"},
						},
					},
				},
				"pareto_tradeoffs": map[string]any{
					"type":        "array",
					"description": "Persisted trade-off notes for Pareto-front variants (compare)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"variant": map[string]any{"type": "string"},
							"summary": map[string]any{"type": "string"},
						},
					},
				},
				"selected_ref":             map[string]any{"type": "string", "description": "Advisory recommendation variant (compare); the human still chooses in delegated mode"},
				"recommendation_rationale": map[string]any{"type": "string", "description": "Why the recommendation is advised, separated from the human choice (compare)"},
				"policy_applied":           map[string]any{"type": "string", "description": "Selection rule used (compare)"},
				"parity_plan":              parityPlanSchema("Structured parity plan for compare-time enforcement"),
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
		ProblemRef:               jsonStr(args, "problem_ref"),
		Context:                  jsonStr(args, "context"),
		Mode:                     jsonStr(args, "mode"),
		NoSteppingStoneRationale: jsonStr(args, "no_stepping_stone_rationale"),
	}

	_ = jsonDecodeArg(args, "variants", &input.Variants)

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
	input.Results.RecommendationRationale = jsonStr(args, "recommendation_rationale")
	input.Results.PolicyApplied = jsonStr(args, "policy_applied")

	_ = jsonDecodeArg(args, "scores", &input.Results.Scores)
	_ = jsonDecodeArg(args, "incomparable", &input.Results.Incomparable)
	_ = jsonDecodeArg(args, "dominated_variants", &input.Results.DominatedVariants)
	_ = jsonDecodeArg(args, "pareto_tradeoffs", &input.Results.ParetoTradeoffs)
	if parityPlan, ok := jsonParityPlan(args, "parity_plan"); ok {
		input.Results.ParityPlan = parityPlan
	}

	a, filePath, err := artifact.CompareSolutions(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	var display strings.Builder
	display.WriteString("Comparison recorded.\n")
	display.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
	if filePath != "" {
		display.WriteString(fmt.Sprintf("File: %s\n", filePath))
	}
	display.WriteString(present.ComparisonSummary(a))
	return agent.ToolResult{
		DisplayText: display.String(),
		Meta: &agent.ArtifactMeta{
			Kind:                 "solution",
			ArtifactRef:          a.Meta.ID,
			Operation:            "compare",
			ComparedPortfolioRef: a.Meta.ID,
		},
	}, nil
}

func resolveComparedPortfolioRef(ctx context.Context, store artifact.ArtifactStore, portfolioRef string) string {
	return artifact.ResolveComparedPortfolioRef(ctx, store, portfolioRef)
}

func repairComparedPortfolioRef(ctx context.Context, cycle *agent.Cycle, store artifact.ArtifactStore, registry *Registry) (*agent.Cycle, error) {
	if cycle == nil || store == nil || registry == nil {
		return cycle, nil
	}
	if strings.TrimSpace(cycle.PortfolioRef) == "" || cycle.ComparedPortfolioRef != "" {
		return cycle, nil
	}

	comparedPortfolioRef := artifact.ResolveComparedPortfolioRef(ctx, store, cycle.PortfolioRef)
	if comparedPortfolioRef == "" {
		return cycle, nil
	}

	repaired := *cycle
	repaired.ComparedPortfolioRef = comparedPortfolioRef
	repaired.Phase = agent.DerivePhaseFromCycle(&repaired)
	if err := registry.UpdateCycle(ctx, &repaired); err != nil {
		return cycle, fmt.Errorf("persist compared portfolio repair: %w", err)
	}

	return &repaired, nil
}

func validateDecisionSelection(ctx context.Context, store artifact.ArtifactStore, cycle *agent.Cycle, selectedTitle string) error {
	if !agent.HasDecisionSelection(cycle) || store == nil {
		return nil
	}

	portfolio, err := store.Get(ctx, cycle.ComparedPortfolioRef)
	if err != nil {
		return fmt.Errorf("FPF guardrail: unable to load compared portfolio %q for selection verification: %w", cycle.ComparedPortfolioRef, err)
	}

	variant, ok, err := selectedVariant(portfolio, cycle.SelectedVariantRef)
	if err != nil {
		return fmt.Errorf("FPF guardrail: unable to validate the human-selected variant in compared portfolio %q: %w",
			cycle.ComparedPortfolioRef, err)
	}
	if !ok {
		return fmt.Errorf("FPF guardrail: stored user selection %q is not present in compared portfolio %q. Re-run compare and restate the human choice.",
			cycle.SelectedVariantRef, cycle.ComparedPortfolioRef)
	}

	if decisionMatchesSelectedVariant(selectedTitle, variant) {
		return nil
	}

	return fmt.Errorf("FPF guardrail: decide selected_title %q does not match the human-selected variant %q (%s). Record the variant the human chose or ask them to restate their choice.",
		selectedTitle, variant.Key, variant.Label)
}

func bindDecisionPortfolioRef(args map[string]any, cycle *agent.Cycle) error {
	if cycle == nil {
		return nil
	}

	activePortfolioRef := strings.TrimSpace(cycle.ComparedPortfolioRef)
	if activePortfolioRef == "" {
		activePortfolioRef = strings.TrimSpace(cycle.PortfolioRef)
	}
	if activePortfolioRef == "" {
		return nil
	}

	requestedPortfolioRef := strings.TrimSpace(jsonStr(args, "portfolio_ref"))
	if requestedPortfolioRef == "" {
		args["portfolio_ref"] = activePortfolioRef
		return nil
	}

	if requestedPortfolioRef == activePortfolioRef {
		return nil
	}

	return &agent.GuardrailError{
		Tool:    "haft_decision(decide)",
		Missing: "portfolio_ref aligned with the active compared portfolio",
		Guidance: fmt.Sprintf(
			"Decide against the active compared portfolio %q. Omit portfolio_ref to use it automatically, or pass that exact value.",
			activePortfolioRef,
		),
	}
}

func selectedVariant(portfolio *artifact.Artifact, selectedRef string) (artifact.PortfolioVariantIdentity, bool, error) {
	return artifact.ResolvePortfolioVariantIdentity(portfolio, selectedRef)
}

func decisionMatchesSelectedVariant(selectedTitle string, variant artifact.PortfolioVariantIdentity) bool {
	normalizedTitle := normalizeDecisionSelectionValue(selectedTitle)
	if normalizedTitle == "" {
		return false
	}

	aliases := decisionSelectionAliases(variant)

	for _, alias := range aliases {
		if normalizeDecisionSelectionValue(alias) == normalizedTitle {
			return true
		}
	}

	return false
}

func decisionSelectionAliases(variant artifact.PortfolioVariantIdentity) []string {
	aliases := append([]string(nil), variant.Aliases...)
	aliases = append(aliases,
		variant.Key+" "+variant.Label,
		"variant "+variant.Key,
		"option "+variant.Key,
		"variant "+variant.Label,
		"option "+variant.Label,
	)

	return aliases
}

func normalizeDecisionSelectionValue(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return r
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, lowered)

	return strings.Join(strings.Fields(cleaned), " ")
}

// ---------------------------------------------------------------------------
// HaftDecisionTool — decide with rationale, attach evidence, baseline, measure
// ---------------------------------------------------------------------------

type HaftDecisionTool struct {
	store       artifact.ArtifactStore
	haftDir     string
	projectRoot string
	registry    *Registry
}

type sqlArtifactStore interface {
	DB() *sql.DB
}

func NewHaftDecisionTool(store artifact.ArtifactStore, haftDir, projectRoot string, registry *Registry) *HaftDecisionTool {
	return &HaftDecisionTool{store: store, haftDir: haftDir, projectRoot: projectRoot, registry: registry}
}

func (t *HaftDecisionTool) Name() string { return "haft_decision" }

func (t *HaftDecisionTool) Schema() agent.ToolSchema {
	return agent.ToolSchema{
		Name: "haft_decision",
		Description: `Record decisions, evidence, and measurements.

Actions:
- decide: Record a formal decision with rationale (FPF E.9 DRR).
  Includes: selected variant, explicit selection policy, strongest counterargument,
  rejected alternatives, rollback trigger, invariants, and selected-variant weakest link.
- evidence: Attach an explicit evidence item to any artifact.
- baseline: Snapshot affected files after implementation and before measurement.
- measure: Record measurement results against acceptance criteria.
  Closes the lemniscate cycle with inductive evidence.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":         map[string]any{"type": "string", "enum": []string{"decide", "evidence", "baseline", "measure"}, "description": "decide | evidence | baseline | measure"},
				"problem_ref":    map[string]any{"type": "string", "description": "Problem ID (decide)"},
				"portfolio_ref":  map[string]any{"type": "string", "description": "Portfolio ID (decide)"},
				"selected_title": map[string]any{"type": "string", "description": "Chosen variant title (decide)"},
				"why_selected":   map[string]any{"type": "string", "description": "Rationale for selection (decide)"},
				"selection_policy": map[string]any{
					"type":        "string",
					"description": "Explicit policy used to choose among the compared variants (decide)",
				},
				"counterargument": map[string]any{
					"type":        "string",
					"description": "Strongest argument against the chosen option (decide)",
				},
				"why_not_others": map[string]any{
					"type":        "array",
					"description": "At least one key rejected alternative and why it lost (decide)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"variant": map[string]any{"type": "string"},
							"reason":  map[string]any{"type": "string"},
						},
					},
				},
				"invariants":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What MUST hold (decide)"},
				"post_conditions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Implementation checklist (decide)"},
				"admissibility":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "What is NOT acceptable (decide)"},
				"weakest_link":    map[string]any{"type": "string", "description": "Selected variant weakest link — what most plausibly breaks this choice (decide)"},
				"rollback": map[string]any{
					"type":        "object",
					"description": "How and when to reverse the decision (decide). At least one trigger is required.",
					"properties": map[string]any{
						"triggers":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"steps":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"blast_radius": map[string]any{"type": "string"},
					},
				},
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
				"valid_until":      map[string]any{"type": "string", "description": "Expiry deadline (RFC3339 or YYYY-MM-DD) (decide/evidence)"},
				"decision_ref":     map[string]any{"type": "string", "description": "Decision ID (measure)"},
				"findings":         map[string]any{"type": "string", "description": "What was observed (measure)"},
				"criteria_met":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Passing criteria (measure)"},
				"criteria_not_met": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Failing criteria (measure)"},
				"verdict":          map[string]any{"type": "string", "enum": []string{"accepted", "partial", "failed"}, "description": "Verdict (measure)"},
				"artifact_ref":     map[string]any{"type": "string", "description": "Artifact ID to attach evidence to (evidence)"},
				"evidence_content": map[string]any{"type": "string", "description": "The evidence itself (evidence)"},
				"evidence_type":    map[string]any{"type": "string", "description": "measurement | test | research | benchmark | audit (evidence)"},
				"evidence_verdict": map[string]any{"type": "string", "enum": []string{"supports", "weakens", "refutes"}, "description": "How the evidence bears on the artifact (evidence)"},
				"carrier_ref":      map[string]any{"type": "string", "description": "File path or URL for the evidence source (evidence)"},
				"congruence_level": map[string]any{"type": "integer", "description": "CL 0-3: 3=same context, 2=similar, 1=different, 0=opposed (evidence)"},
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
		// FPF guardrails: requires compared variants + decision-boundary user selection
		if t.registry != nil {
			cycle := t.registry.ActiveCycle(ctx)
			var err error
			cycle, err = repairComparedPortfolioRef(ctx, cycle, t.store, t.registry)
			if err != nil {
				return agent.PlainResult(fmt.Sprintf("FPF guardrail: haft_decision(decide) could not persist the compared-portfolio repair required for this cycle. %s", err.Error())), nil
			}

			selectionSatisfied, err := t.registry.DecisionBoundarySatisfied(ctx, cycle)
			if err != nil {
				return agent.PlainResult(fmt.Sprintf("FPF guardrail: haft_decision(decide) could not verify the compare -> decide selection boundary. %s", err.Error())), nil
			}

			if err := agent.CanDecide(cycle, selectionSatisfied); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
			if err := bindDecisionPortfolioRef(args, cycle); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
			if err := validateDecisionSelection(ctx, t.store, cycle, jsonStr(args, "selected_title")); err != nil {
				return agent.PlainResult(err.Error()), nil
			}
		}
		return t.decide(ctx, args)
	case "evidence":
		return t.evidence(ctx, args)
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
		ProblemRef:      jsonStr(args, "problem_ref"),
		PortfolioRef:    jsonStr(args, "portfolio_ref"),
		SelectedTitle:   jsonStr(args, "selected_title"),
		WhySelected:     jsonStr(args, "why_selected"),
		SelectionPolicy: jsonStr(args, "selection_policy"),
		CounterArgument: jsonStr(args, "counterargument"),
		WeakestLink:     jsonStr(args, "weakest_link"),
		ValidUntil:      jsonStr(args, "valid_until"),
		Context:         jsonStr(args, "context"),
		Mode:            jsonStr(args, "mode"),
		Invariants:      jsonStrArray(args, "invariants"),
		PostConditions:  jsonStrArray(args, "post_conditions"),
		Admissibility:   jsonStrArray(args, "admissibility"),
		AffectedFiles:   jsonStrArray(args, "affected_files"),
	}

	if items, ok := args["why_not_others"].([]any); ok {
		for _, item := range items {
			value, ok := item.(map[string]any)
			if !ok {
				continue
			}

			input.WhyNotOthers = append(input.WhyNotOthers, artifact.RejectionReason{
				Variant: jsonStr(value, "variant"),
				Reason:  jsonStr(value, "reason"),
			})
		}
	}

	if rawRollback, ok := args["rollback"].(map[string]any); ok {
		rollback := &artifact.RollbackSpec{
			BlastRadius: jsonStr(rawRollback, "blast_radius"),
		}
		rollback.Triggers = jsonStrArray(rawRollback, "triggers")
		rollback.Steps = jsonStrArray(rawRollback, "steps")
		input.Rollback = rollback
	}

	// Parse predictions
	if preds, ok := args["predictions"]; ok {
		data, _ := json.Marshal(preds)
		var parsed []artifact.PredictionInput
		if json.Unmarshal(data, &parsed) == nil {
			input.Predictions = parsed
		}
	}

	gaps := t.coverageGaps(ctx, input.AffectedFiles)
	input.FirstModuleCoverage = len(gaps) > 0

	a, filePath, err := artifact.Decide(ctx, t.store, t.haftDir, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	display := fmt.Sprintf("Decision recorded: %s\nID: %s\nFile: %s\nSelected: %s", a.Meta.Title, a.Meta.ID, filePath, input.SelectedTitle)
	coverageWarnings := formatCoverageWarnings(gaps)
	if len(coverageWarnings) > 0 {
		display += "\n\n" + strings.Join(coverageWarnings, "\n")
	}

	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:        "decision",
			ArtifactRef: a.Meta.ID,
			Operation:   "decide",
		},
	}, nil
}

func (t *HaftDecisionTool) evidence(ctx context.Context, args map[string]any) (agent.ToolResult, error) {
	input := artifact.EvidenceInput{
		CongruenceLevel: -1,
		FormalityLevel:  -1,
		ArtifactRef:     jsonStr(args, "artifact_ref"),
		Content:         jsonStr(args, "evidence_content"),
		Type:            jsonStr(args, "evidence_type"),
		Verdict:         jsonStr(args, "evidence_verdict"),
		CarrierRef:      jsonStr(args, "carrier_ref"),
		ValidUntil:      jsonStr(args, "valid_until"),
	}

	if level, ok := args["congruence_level"].(float64); ok {
		input.CongruenceLevel = int(level)
	}

	item, err := artifact.AttachEvidence(ctx, t.store, input)
	if err != nil {
		return agent.ToolResult{}, err
	}

	wlnk := artifact.ComputeWLNKSummary(ctx, t.store, input.ArtifactRef)
	display := fmt.Sprintf(
		"Evidence attached: %s [%s]\nArtifact: %s\nVerdict: %s\nWLNK: %s",
		item.ID,
		item.Type,
		input.ArtifactRef,
		item.Verdict,
		wlnk.Summary,
	)

	return agent.ToolResult{
		DisplayText: display,
		Meta: &agent.ArtifactMeta{
			Kind:        "evidence",
			ArtifactRef: input.ArtifactRef,
			Operation:   "evidence",
		},
	}, nil
}

func (t *HaftDecisionTool) coverageGaps(ctx context.Context, affectedFiles []string) []codebase.ModuleGovernanceGap {
	if len(affectedFiles) == 0 {
		return nil
	}

	dbStore, ok := t.store.(sqlArtifactStore)
	if !ok {
		return nil
	}

	gaps, err := codebase.FindFirstDecisionModules(ctx, dbStore.DB(), affectedFiles)
	if err != nil {
		return nil
	}
	return gaps
}

func formatCoverageWarnings(gaps []codebase.ModuleGovernanceGap) []string {
	if len(gaps) == 0 {
		return nil
	}
	warnings := make([]string, 0, len(gaps)*2)

	for _, gap := range gaps {
		modulePath := gap.Module.Path
		if modulePath == "" {
			modulePath = "(root)"
		}

		warnings = append(warnings,
			fmt.Sprintf("⚠ First decision governing module '%s' — no prior architectural context exists.", modulePath),
		)
		warnings = append(warnings,
			"Consider: are there implicit conventions or patterns in this module that should be captured?",
		)
	}

	return warnings
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

// FPFSearchRequest captures deterministic FPF retrieval and presentation
// options for the haft_query tool.
type FPFSearchRequest struct {
	Query   string
	Limit   int
	Full    bool
	Explain bool
}

// FPFSearchFunc is a callback that searches the FPF specification.
// Returns formatted results or error. Injected by cmd/agent.go to avoid
// importing the embedded DB from internal packages.
type FPFSearchFunc func(request FPFSearchRequest) (string, error)

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
				"limit":  map[string]any{"type": "integer", "description": fmt.Sprintf("(fpf) Max FPF results, default %d", fpf.DefaultSpecSearchLimit)},
				"full":   map[string]any{"type": "boolean", "description": "(fpf) Show full section content instead of snippets"},
				"explain": map[string]any{
					"type":        "boolean",
					"description": "(fpf) Show why each FPF result matched",
				},
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
		request := FPFSearchRequest{
			Query:   query,
			Limit:   jsonIntDefault(args, "limit", fpf.DefaultSpecSearchLimit),
			Full:    jsonBool(args, "full"),
			Explain: jsonBool(args, "explain"),
		}
		result, err := t.fpfSearch(request)
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
	haftDir     string
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
	store   artifact.ArtifactStore
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

func jsonBool(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

func jsonIntDefault(args map[string]any, key string, defaultValue int) int {
	value, ok := args[key]
	if !ok {
		return defaultValue
	}

	switch typed := value.(type) {
	case float64:
		if int(typed) > 0 {
			return int(typed)
		}
	case int:
		if typed > 0 {
			return typed
		}
	}

	return defaultValue
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

func jsonDecodeArg(args map[string]any, key string, target any) bool {
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

func jsonParityPlan(args map[string]any, key string) (*artifact.ParityPlan, bool) {
	var plan artifact.ParityPlan
	if !jsonDecodeArg(args, key, &plan) {
		return nil, false
	}

	return &plan, true
}

func parityPlanSchema(description string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": description,
		"properties": map[string]any{
			"baseline_set": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"window":       map[string]any{"type": "string"},
			"budget":       map[string]any{"type": "string"},
			"missing_data_policy": map[string]any{
				"type": "string",
				"enum": []string{
					artifact.MissingDataPolicyExplicitAbstain,
					artifact.MissingDataPolicyZero,
					artifact.MissingDataPolicyExclude,
				},
			},
			"normalization": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dimension": map[string]any{"type": "string"},
						"method":    map[string]any{"type": "string"},
					},
				},
			},
			"pinned_conditions": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
}
