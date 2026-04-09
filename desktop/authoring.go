package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

type ProblemCreateInput struct {
	Title                 string   `json:"title"`
	Signal                string   `json:"signal"`
	Acceptance            string   `json:"acceptance"`
	BlastRadius           string   `json:"blast_radius"`
	Reversibility         string   `json:"reversibility"`
	Context               string   `json:"context"`
	Mode                  string   `json:"mode"`
	Constraints           []string `json:"constraints"`
	OptimizationTargets   []string `json:"optimization_targets"`
	ObservationIndicators []string `json:"observation_indicators"`
}

type ComparisonDimensionInput struct {
	Name         string `json:"name"`
	ScaleType    string `json:"scale_type"`
	Unit         string `json:"unit"`
	Polarity     string `json:"polarity"`
	Role         string `json:"role"`
	HowToMeasure string `json:"how_to_measure"`
	ValidUntil   string `json:"valid_until"`
}

type NormRuleInput struct {
	Dimension string `json:"dimension"`
	Method    string `json:"method"`
}

type ParityPlanInput struct {
	BaselineSet       []string        `json:"baseline_set"`
	Window            string          `json:"window"`
	Budget            string          `json:"budget"`
	Normalization     []NormRuleInput `json:"normalization"`
	MissingDataPolicy string          `json:"missing_data_policy"`
	PinnedConditions  []string        `json:"pinned_conditions"`
}

type ProblemCharacterizationInput struct {
	ProblemRef  string                     `json:"problem_ref"`
	Dimensions  []ComparisonDimensionInput `json:"dimensions"`
	ParityRules string                     `json:"parity_rules"`
	ParityPlan  *ParityPlanInput           `json:"parity_plan"`
}

type PortfolioVariantInput struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Strengths          []string `json:"strengths"`
	WeakestLink        string   `json:"weakest_link"`
	NoveltyMarker      string   `json:"novelty_marker"`
	Risks              []string `json:"risks"`
	SteppingStone      bool     `json:"stepping_stone"`
	SteppingStoneBasis string   `json:"stepping_stone_basis"`
	DiversityRole      string   `json:"diversity_role"`
	AssumptionNotes    string   `json:"assumption_notes"`
	RollbackNotes      string   `json:"rollback_notes"`
	EvidenceRefs       []string `json:"evidence_refs"`
}

type PortfolioCreateInput struct {
	ProblemRef               string                  `json:"problem_ref"`
	Context                  string                  `json:"context"`
	Mode                     string                  `json:"mode"`
	NoSteppingStoneRationale string                  `json:"no_stepping_stone_rationale"`
	Variants                 []PortfolioVariantInput `json:"variants"`
}

type DominatedNoteInput struct {
	Variant     string   `json:"variant"`
	DominatedBy []string `json:"dominated_by"`
	Summary     string   `json:"summary"`
}

type TradeoffNoteInput struct {
	Variant string `json:"variant"`
	Summary string `json:"summary"`
}

type PortfolioCompareInput struct {
	PortfolioRef    string                       `json:"portfolio_ref"`
	Dimensions      []string                     `json:"dimensions"`
	Scores          map[string]map[string]string `json:"scores"`
	NonDominatedSet []string                     `json:"non_dominated_set"`
	Incomparable    [][]string                   `json:"incomparable"`
	DominatedNotes  []DominatedNoteInput         `json:"dominated_notes"`
	ParetoTradeoffs []TradeoffNoteInput          `json:"pareto_tradeoffs"`
	PolicyApplied   string                       `json:"policy_applied"`
	SelectedRef     string                       `json:"selected_ref"`
	Recommendation  string                       `json:"recommendation"`
	ParityPlan      *ParityPlanInput             `json:"parity_plan"`
}

type DecisionRejectionInput struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

type DecisionPredictionInput struct {
	Claim       string `json:"claim"`
	Observable  string `json:"observable"`
	Threshold   string `json:"threshold"`
	VerifyAfter string `json:"verify_after"`
}

type DecisionRollbackInput struct {
	Triggers    []string `json:"triggers"`
	Steps       []string `json:"steps"`
	BlastRadius string   `json:"blast_radius"`
}

type DecisionCreateInput struct {
	ProblemRef           string                    `json:"problem_ref"`
	ProblemRefs          []string                  `json:"problem_refs"`
	PortfolioRef         string                    `json:"portfolio_ref"`
	SelectedRef          string                    `json:"selected_ref"`
	SelectedTitle        string                    `json:"selected_title"`
	WhySelected          string                    `json:"why_selected"`
	SelectionPolicy      string                    `json:"selection_policy"`
	CounterArgument      string                    `json:"counterargument"`
	WhyNotOthers         []DecisionRejectionInput  `json:"why_not_others"`
	Invariants           []string                  `json:"invariants"`
	PreConditions        []string                  `json:"pre_conditions"`
	PostConditions       []string                  `json:"post_conditions"`
	Admissibility        []string                  `json:"admissibility"`
	EvidenceRequirements []string                  `json:"evidence_requirements"`
	Rollback             *DecisionRollbackInput    `json:"rollback"`
	RefreshTriggers      []string                  `json:"refresh_triggers"`
	WeakestLink          string                    `json:"weakest_link"`
	ValidUntil           string                    `json:"valid_until"`
	Context              string                    `json:"context"`
	Mode                 string                    `json:"mode"`
	AffectedFiles        []string                  `json:"affected_files"`
	Predictions          []DecisionPredictionInput `json:"predictions"`
	SearchKeywords       string                    `json:"search_keywords"`
	FirstModuleCoverage  bool                      `json:"first_module_coverage"`
}

func (a *App) CreateProblem(input ProblemCreateInput) (*ProblemDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	created, _, err := artifact.FrameProblem(a.ctx, a.store, a.haftDir(), artifact.ProblemFrameInput{
		Title:                 strings.TrimSpace(input.Title),
		Signal:                strings.TrimSpace(input.Signal),
		Constraints:           compactStrings(input.Constraints),
		OptimizationTargets:   compactStrings(input.OptimizationTargets),
		ObservationIndicators: compactStrings(input.ObservationIndicators),
		Acceptance:            strings.TrimSpace(input.Acceptance),
		BlastRadius:           strings.TrimSpace(input.BlastRadius),
		Reversibility:         strings.TrimSpace(input.Reversibility),
		Context:               strings.TrimSpace(input.Context),
		Mode:                  strings.TrimSpace(input.Mode),
	})
	if err != nil {
		return nil, err
	}

	view := toProblemDetail(a.ctx, created, a.store)
	return &view, nil
}

func (a *App) CharacterizeProblem(input ProblemCharacterizationInput) (*ProblemDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	updated, _, err := artifact.CharacterizeProblem(a.ctx, a.store, a.haftDir(), artifact.CharacterizeInput{
		ProblemRef:  strings.TrimSpace(input.ProblemRef),
		Dimensions:  toArtifactDimensions(input.Dimensions),
		ParityRules: strings.TrimSpace(input.ParityRules),
		ParityPlan:  toArtifactParityPlan(input.ParityPlan),
	})
	if err != nil {
		return nil, err
	}

	view := toProblemDetail(a.ctx, updated, a.store)
	return &view, nil
}

func (a *App) CreatePortfolio(input PortfolioCreateInput) (*PortfolioDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	created, _, err := artifact.ExploreSolutions(a.ctx, a.store, a.haftDir(), artifact.ExploreInput{
		ProblemRef:               strings.TrimSpace(input.ProblemRef),
		Context:                  strings.TrimSpace(input.Context),
		Mode:                     strings.TrimSpace(input.Mode),
		NoSteppingStoneRationale: strings.TrimSpace(input.NoSteppingStoneRationale),
		Variants:                 toArtifactVariants(input.Variants),
	})
	if err != nil {
		return nil, err
	}

	view := toPortfolioDetail(created)
	return &view, nil
}

func (a *App) ComparePortfolio(input PortfolioCompareInput) (*PortfolioDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	updated, _, err := artifact.CompareSolutions(a.ctx, a.store, a.haftDir(), artifact.CompareInput{
		PortfolioRef: strings.TrimSpace(input.PortfolioRef),
		Results: artifact.ComparisonResult{
			Dimensions:              compactStrings(input.Dimensions),
			Scores:                  normalizeScoreMatrix(input.Scores),
			NonDominatedSet:         compactStrings(input.NonDominatedSet),
			Incomparable:            normalizePairs(input.Incomparable),
			DominatedVariants:       toArtifactDominatedNotes(input.DominatedNotes),
			ParetoTradeoffs:         toArtifactTradeoffNotes(input.ParetoTradeoffs),
			PolicyApplied:           strings.TrimSpace(input.PolicyApplied),
			SelectedRef:             strings.TrimSpace(input.SelectedRef),
			RecommendationRationale: strings.TrimSpace(input.Recommendation),
			ParityPlan:              toArtifactParityPlan(input.ParityPlan),
		},
	})
	if err != nil {
		return nil, err
	}

	view := toPortfolioDetail(updated)
	return &view, nil
}

func (a *App) CreateDecision(input DecisionCreateInput) (*DecisionDetailView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database connection")
	}

	resolved := normalizeDecisionCreateInput(input)
	resolved, err := a.enrichDecisionFromPortfolio(resolved)
	if err != nil {
		return nil, err
	}

	created, _, err := artifact.Decide(a.ctx, a.store, a.haftDir(), artifact.DecideInput{
		ProblemRef:          resolved.ProblemRef,
		ProblemRefs:         compactStrings(resolved.ProblemRefs),
		PortfolioRef:        resolved.PortfolioRef,
		SelectedTitle:       resolved.SelectedTitle,
		WhySelected:         resolved.WhySelected,
		SelectionPolicy:     resolved.SelectionPolicy,
		CounterArgument:     resolved.CounterArgument,
		WhyNotOthers:        toArtifactRejections(resolved.WhyNotOthers),
		Invariants:          compactStrings(resolved.Invariants),
		PreConditions:       compactStrings(resolved.PreConditions),
		PostConditions:      compactStrings(resolved.PostConditions),
		Admissibility:       compactStrings(resolved.Admissibility),
		EvidenceReqs:        compactStrings(resolved.EvidenceRequirements),
		Rollback:            toArtifactRollback(resolved.Rollback),
		RefreshTriggers:     compactStrings(resolved.RefreshTriggers),
		WeakestLink:         strings.TrimSpace(resolved.WeakestLink),
		ValidUntil:          strings.TrimSpace(resolved.ValidUntil),
		Context:             strings.TrimSpace(resolved.Context),
		Mode:                strings.TrimSpace(resolved.Mode),
		AffectedFiles:       compactStrings(resolved.AffectedFiles),
		Predictions:         toArtifactPredictions(resolved.Predictions),
		SearchKeywords:      strings.TrimSpace(resolved.SearchKeywords),
		FirstModuleCoverage: resolved.FirstModuleCoverage,
	})
	if err != nil {
		return nil, err
	}

	view := toDecisionDetail(created)
	return &view, nil
}

func (a *App) haftDir() string {
	return filepath.Join(a.projectRoot, ".haft")
}

func normalizeDecisionCreateInput(input DecisionCreateInput) DecisionCreateInput {
	input.ProblemRef = strings.TrimSpace(input.ProblemRef)
	input.ProblemRefs = compactStrings(input.ProblemRefs)
	input.PortfolioRef = strings.TrimSpace(input.PortfolioRef)
	input.SelectedRef = strings.TrimSpace(input.SelectedRef)
	input.SelectedTitle = strings.TrimSpace(input.SelectedTitle)
	input.WhySelected = strings.TrimSpace(input.WhySelected)
	input.SelectionPolicy = strings.TrimSpace(input.SelectionPolicy)
	input.CounterArgument = strings.TrimSpace(input.CounterArgument)
	input.WeakestLink = strings.TrimSpace(input.WeakestLink)
	input.ValidUntil = strings.TrimSpace(input.ValidUntil)
	input.Context = strings.TrimSpace(input.Context)
	input.Mode = strings.TrimSpace(input.Mode)
	input.SearchKeywords = strings.TrimSpace(input.SearchKeywords)
	input.Invariants = compactStrings(input.Invariants)
	input.PreConditions = compactStrings(input.PreConditions)
	input.PostConditions = compactStrings(input.PostConditions)
	input.Admissibility = compactStrings(input.Admissibility)
	input.EvidenceRequirements = compactStrings(input.EvidenceRequirements)
	input.RefreshTriggers = compactStrings(input.RefreshTriggers)
	input.AffectedFiles = compactStrings(input.AffectedFiles)

	return input
}

func (a *App) enrichDecisionFromPortfolio(input DecisionCreateInput) (DecisionCreateInput, error) {
	if input.PortfolioRef == "" {
		return input, nil
	}

	portfolio, err := a.store.Get(a.ctx, input.PortfolioRef)
	if err != nil {
		return input, fmt.Errorf("portfolio %s not found: %w", input.PortfolioRef, err)
	}

	fields := portfolio.UnmarshalPortfolioFields()
	variantByID := make(map[string]artifact.Variant)
	variantByTitle := make(map[string]artifact.Variant)

	for _, variant := range fields.Variants {
		variantByID[variant.ID] = variant
		variantByTitle[strings.TrimSpace(variant.Title)] = variant
	}

	selectedRef := input.SelectedRef
	if input.SelectedTitle == "" && selectedRef != "" {
		if variant, ok := variantByID[selectedRef]; ok {
			input.SelectedTitle = strings.TrimSpace(variant.Title)
		}
	}

	if input.SelectedTitle == "" {
		input.SelectedTitle = selectedRef
	}

	selectedVariant, hasSelectedVariant := variantByID[selectedRef]
	if !hasSelectedVariant {
		selectedVariant, hasSelectedVariant = variantByTitle[input.SelectedTitle]
	}

	if input.WeakestLink == "" && hasSelectedVariant {
		input.WeakestLink = strings.TrimSpace(selectedVariant.WeakestLink)
	}

	if input.ProblemRef == "" && len(input.ProblemRefs) == 0 && strings.TrimSpace(fields.ProblemRef) != "" {
		input.ProblemRef = strings.TrimSpace(fields.ProblemRef)
	}

	if input.Context == "" && portfolio.Meta.Context != "" {
		input.Context = string(portfolio.Meta.Context)
	}

	if input.Mode == "" && portfolio.Meta.Mode != "" {
		input.Mode = string(portfolio.Meta.Mode)
	}

	if len(input.WhyNotOthers) == 0 {
		input.WhyNotOthers = defaultDecisionRejections(fields, input.SelectedTitle)
		return input, nil
	}

	normalized := make([]DecisionRejectionInput, 0, len(input.WhyNotOthers))
	for _, rejection := range input.WhyNotOthers {
		variant := strings.TrimSpace(rejection.Variant)
		reason := strings.TrimSpace(rejection.Reason)
		if mapped, ok := variantByID[variant]; ok {
			variant = strings.TrimSpace(mapped.Title)
		}
		normalized = append(normalized, DecisionRejectionInput{Variant: variant, Reason: reason})
	}
	input.WhyNotOthers = normalized

	return input, nil
}

func defaultDecisionRejections(fields artifact.PortfolioFields, selectedTitle string) []DecisionRejectionInput {
	rejections := make([]DecisionRejectionInput, 0, len(fields.Variants))

	for _, variant := range fields.Variants {
		title := strings.TrimSpace(variant.Title)
		if title == "" || strings.EqualFold(title, selectedTitle) {
			continue
		}

		rejections = append(rejections, DecisionRejectionInput{
			Variant: title,
			Reason:  fmt.Sprintf("Did not beat %s under the active comparison policy.", selectedTitle),
		})
	}

	return rejections
}

func toArtifactDimensions(inputs []ComparisonDimensionInput) []artifact.ComparisonDimension {
	dimensions := make([]artifact.ComparisonDimension, 0, len(inputs))

	for _, input := range inputs {
		dimensions = append(dimensions, artifact.ComparisonDimension{
			Name:         strings.TrimSpace(input.Name),
			ScaleType:    strings.TrimSpace(input.ScaleType),
			Unit:         strings.TrimSpace(input.Unit),
			Polarity:     strings.TrimSpace(input.Polarity),
			Role:         strings.TrimSpace(input.Role),
			HowToMeasure: strings.TrimSpace(input.HowToMeasure),
			ValidUntil:   strings.TrimSpace(input.ValidUntil),
		})
	}

	return dimensions
}

func toArtifactParityPlan(input *ParityPlanInput) *artifact.ParityPlan {
	if input == nil {
		return nil
	}

	plan := &artifact.ParityPlan{
		BaselineSet:       compactStrings(input.BaselineSet),
		Window:            strings.TrimSpace(input.Window),
		Budget:            strings.TrimSpace(input.Budget),
		MissingDataPolicy: strings.TrimSpace(input.MissingDataPolicy),
		PinnedConditions:  compactStrings(input.PinnedConditions),
	}

	for _, rule := range input.Normalization {
		plan.Normalization = append(plan.Normalization, artifact.NormRule{
			Dimension: strings.TrimSpace(rule.Dimension),
			Method:    strings.TrimSpace(rule.Method),
		})
	}

	return plan
}

func toArtifactVariants(inputs []PortfolioVariantInput) []artifact.Variant {
	variants := make([]artifact.Variant, 0, len(inputs))

	for _, input := range inputs {
		variants = append(variants, artifact.Variant{
			ID:                 strings.TrimSpace(input.ID),
			Title:              strings.TrimSpace(input.Title),
			Description:        strings.TrimSpace(input.Description),
			Strengths:          compactStrings(input.Strengths),
			WeakestLink:        strings.TrimSpace(input.WeakestLink),
			NoveltyMarker:      strings.TrimSpace(input.NoveltyMarker),
			Risks:              compactStrings(input.Risks),
			SteppingStone:      input.SteppingStone,
			SteppingStoneBasis: strings.TrimSpace(input.SteppingStoneBasis),
			DiversityRole:      strings.TrimSpace(input.DiversityRole),
			AssumptionNotes:    strings.TrimSpace(input.AssumptionNotes),
			RollbackNotes:      strings.TrimSpace(input.RollbackNotes),
			EvidenceRefs:       compactStrings(input.EvidenceRefs),
		})
	}

	return variants
}

func toArtifactDominatedNotes(inputs []DominatedNoteInput) []artifact.DominatedVariantExplanation {
	notes := make([]artifact.DominatedVariantExplanation, 0, len(inputs))

	for _, input := range inputs {
		notes = append(notes, artifact.DominatedVariantExplanation{
			Variant:     strings.TrimSpace(input.Variant),
			DominatedBy: compactStrings(input.DominatedBy),
			Summary:     strings.TrimSpace(input.Summary),
		})
	}

	return notes
}

func toArtifactTradeoffNotes(inputs []TradeoffNoteInput) []artifact.ParetoTradeoffNote {
	notes := make([]artifact.ParetoTradeoffNote, 0, len(inputs))

	for _, input := range inputs {
		notes = append(notes, artifact.ParetoTradeoffNote{
			Variant: strings.TrimSpace(input.Variant),
			Summary: strings.TrimSpace(input.Summary),
		})
	}

	return notes
}

func toArtifactRejections(inputs []DecisionRejectionInput) []artifact.RejectionReason {
	rejections := make([]artifact.RejectionReason, 0, len(inputs))

	for _, input := range inputs {
		rejections = append(rejections, artifact.RejectionReason{
			Variant: strings.TrimSpace(input.Variant),
			Reason:  strings.TrimSpace(input.Reason),
		})
	}

	return rejections
}

func toArtifactRollback(input *DecisionRollbackInput) *artifact.RollbackSpec {
	if input == nil {
		return nil
	}

	return &artifact.RollbackSpec{
		Triggers:    compactStrings(input.Triggers),
		Steps:       compactStrings(input.Steps),
		BlastRadius: strings.TrimSpace(input.BlastRadius),
	}
}

func toArtifactPredictions(inputs []DecisionPredictionInput) []artifact.PredictionInput {
	predictions := make([]artifact.PredictionInput, 0, len(inputs))

	for _, input := range inputs {
		predictions = append(predictions, artifact.PredictionInput{
			Claim:       strings.TrimSpace(input.Claim),
			Observable:  strings.TrimSpace(input.Observable),
			Threshold:   strings.TrimSpace(input.Threshold),
			VerifyAfter: strings.TrimSpace(input.VerifyAfter),
		})
	}

	return predictions
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}

func normalizeScoreMatrix(scores map[string]map[string]string) map[string]map[string]string {
	if len(scores) == 0 {
		return map[string]map[string]string{}
	}

	normalized := make(map[string]map[string]string, len(scores))

	for variant, dimensionScores := range scores {
		variantID := strings.TrimSpace(variant)
		if variantID == "" {
			continue
		}

		row := make(map[string]string, len(dimensionScores))
		for dimension, value := range dimensionScores {
			dimensionName := strings.TrimSpace(dimension)
			if dimensionName == "" {
				continue
			}

			row[dimensionName] = strings.TrimSpace(value)
		}

		normalized[variantID] = row
	}

	return normalized
}

func normalizePairs(pairs [][]string) [][]string {
	normalized := make([][]string, 0, len(pairs))

	for _, pair := range pairs {
		current := compactStrings(pair)
		if len(current) == 0 {
			continue
		}

		normalized = append(normalized, current)
	}

	return normalized
}
