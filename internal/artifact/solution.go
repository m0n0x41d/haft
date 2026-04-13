package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// ExploreInput is the input for creating a SolutionPortfolio with variants.
type ExploreInput struct {
	ProblemRef               string    `json:"problem_ref,omitempty"`
	Variants                 []Variant `json:"variants"`
	Context                  string    `json:"context,omitempty"`
	Mode                     string    `json:"mode,omitempty"`
	NoSteppingStoneRationale string    `json:"no_stepping_stone_rationale,omitempty"`
}

// CompareInput is the input for running a parity comparison.
type CompareInput struct {
	PortfolioRef string           `json:"portfolio_ref,omitempty"`
	Results      ComparisonResult `json:"results"`
}

// ExploreSolutions creates a SolutionPortfolio artifact with variants.
// ExploreContext holds pre-fetched data for pure portfolio construction.
type ExploreContext struct {
	ID           string
	Now          time.Time
	ProblemTitle string // from linked problem
	Context      string // inherited
	Mode         Mode   // inherited or default
	Links        []Link
}

type parityPlanSource string

const (
	parityPlanSourceNone     parityPlanSource = "none"
	parityPlanSourceLegacy   parityPlanSource = "legacy"
	parityPlanSourceExplicit parityPlanSource = "explicit"
)

// CompareValidationContext carries the pure inputs needed for compare-time validation.
type CompareValidationContext struct {
	Mode                    Mode
	PortfolioVariants       []string
	CharacterizedDimensions []charDim
	ParityPlan              *ParityPlan
	ParitySource            parityPlanSource
}

// CompareValidationResult carries the pure outputs of compare-time validation.
type CompareValidationResult struct {
	Warnings             []string
	EffectiveParity      *ParityPlan
	ComparedVariants     []string
	ComputedParetoFront  []string
	ConstraintEliminated map[string]string // variantID -> reason (nil when none)
}

// ValidateExploreInput checks variant constraints. Pure.
func ValidateExploreInput(input ExploreInput) error {
	if len(input.Variants) == 0 {
		return fmt.Errorf("no variants received — check that 'variants' is a JSON array of objects with 'title' and 'weakest_link' fields")
	}
	if len(input.Variants) < 2 {
		return fmt.Errorf("at least 2 variants required (got %d) — genuinely distinct options, not variations of one idea", len(input.Variants))
	}
	for i, v := range input.Variants {
		if strings.TrimSpace(v.Title) == "" {
			return fmt.Errorf("variant %d: title is required", i+1)
		}
		if strings.TrimSpace(v.WeakestLink) == "" {
			return fmt.Errorf("variant %d (%s): weakest_link is required — what bounds this option's quality?", i+1, v.Title)
		}
		if strings.TrimSpace(v.NoveltyMarker) == "" {
			return fmt.Errorf("variant %d (%s): novelty_marker is required — state how this differs from the other options", i+1, v.Title)
		}
		if v.SteppingStone && strings.TrimSpace(v.SteppingStoneBasis) == "" {
			return fmt.Errorf("variant %d (%s): stepping_stone_basis is required when stepping_stone=true", i+1, v.Title)
		}
	}
	if !hasSteppingStone(input.Variants) && strings.TrimSpace(input.NoSteppingStoneRationale) == "" {
		return fmt.Errorf("at least one variant must be a stepping stone or no_stepping_stone_rationale must explain why no stepping stones exist")
	}
	if err := ValidateVariantIdentitySet(materializeVariantIDs(input.Variants)); err != nil {
		return err
	}
	return nil
}

// CheckVariantDiversity warns on near-identical variants (Jaccard > 0.5). Pure.
func CheckVariantDiversity(variants []Variant) []string {
	var warnings []string
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			textI := strings.Join([]string{variants[i].Title, variants[i].Description, variants[i].NoveltyMarker}, " ")
			textJ := strings.Join([]string{variants[j].Title, variants[j].Description, variants[j].NoveltyMarker}, " ")
			sim := jaccardSimilarity(textI, textJ)
			if sim > 0.5 {
				warnings = append(warnings,
					fmt.Sprintf("Variants '%s' and '%s' look similar (%.0f%% word overlap) — do they differ in kind, not degree?",
						variants[i].Title, variants[j].Title, sim*100))
			}

			markerSim := jaccardSimilarity(variants[i].NoveltyMarker, variants[j].NoveltyMarker)
			if markerSim > 0.7 {
				warnings = append(warnings,
					fmt.Sprintf("Novelty markers for '%s' and '%s' overlap heavily (%.0f%%) — differentiate what new space each variant opens.",
						variants[i].Title, variants[j].Title, markerSim*100))
			}
		}
	}
	return warnings
}

// BuildPortfolioArtifact constructs a SolutionPortfolio from input. Pure — no side effects.
func BuildPortfolioArtifact(ectx ExploreContext, input ExploreInput, diversityWarnings []string, recall string) *Artifact {
	title := "Solution Portfolio"
	if ectx.ProblemTitle != "" {
		title = fmt.Sprintf("Solutions for: %s", ectx.ProblemTitle)
	}

	materializedVariants := materializeVariantIDs(input.Variants)

	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", title))

	if input.ProblemRef != "" {
		body.WriteString(fmt.Sprintf("Problem: %s\n\n", input.ProblemRef))
	}

	body.WriteString(fmt.Sprintf("## Variants (%d)\n\n", len(materializedVariants)))

	for _, v := range materializedVariants {
		vid := v.ID
		body.WriteString(fmt.Sprintf("### %s. %s\n\n", vid, v.Title))

		if v.Description != "" {
			body.WriteString(fmt.Sprintf("%s\n\n", v.Description))
		}

		if len(v.Strengths) > 0 {
			body.WriteString("**Strengths:**\n")
			for _, s := range v.Strengths {
				body.WriteString(fmt.Sprintf("- %s\n", s))
			}
			body.WriteString("\n")
		}

		body.WriteString(fmt.Sprintf("**Novelty:** %s\n\n", v.NoveltyMarker))
		body.WriteString(fmt.Sprintf("**Weakest link:** %s\n\n", v.WeakestLink))

		if len(v.Risks) > 0 {
			body.WriteString("**Risks:**\n")
			for _, r := range v.Risks {
				body.WriteString(fmt.Sprintf("- %s\n", r))
			}
			body.WriteString("\n")
		}

		if v.SteppingStone {
			body.WriteString("**Stepping stone:** yes — opens future possibilities\n\n")
			body.WriteString(fmt.Sprintf("**Stepping-stone basis:** %s\n\n", v.SteppingStoneBasis))
		}

		if v.DiversityRole != "" {
			body.WriteString(fmt.Sprintf("**Diversity role:** %s\n\n", v.DiversityRole))
		}

		if v.RollbackNotes != "" {
			body.WriteString(fmt.Sprintf("**Rollback:** %s\n\n", v.RollbackNotes))
		}
	}

	// Summary table
	body.WriteString("## Summary\n\n")
	body.WriteString("| Variant | Novelty | Diversity Role | Weakest Link | Stepping Stone |\n")
	body.WriteString("|---------|---------|----------------|-------------|----------------|\n")
	for _, v := range materializedVariants {
		vid := v.ID
		ss := "no"
		if v.SteppingStone {
			ss = "yes"
		}
		role := v.DiversityRole
		if role == "" {
			role = "-"
		}
		body.WriteString(fmt.Sprintf("| %s. %s | %s | %s | %s | %s |\n", vid, v.Title, v.NoveltyMarker, role, v.WeakestLink, ss))
	}
	body.WriteString("\n")

	if input.NoSteppingStoneRationale != "" {
		body.WriteString(fmt.Sprintf("**Stepping-stone assessment:** none identified yet — %s\n\n", input.NoSteppingStoneRationale))
	}

	a := &Artifact{
		Meta: Meta{
			ID:        ectx.ID,
			Kind:      KindSolutionPortfolio,
			Version:   1,
			Status:    StatusActive,
			Context:   ectx.Context,
			Mode:      ectx.Mode,
			Title:     title,
			CreatedAt: ectx.Now,
			UpdatedAt: ectx.Now,
			Links:     ectx.Links,
		},
		Body: body.String(),
	}

	sd, _ := json.Marshal(PortfolioFields{
		ProblemRef:               input.ProblemRef,
		Variants:                 cloneVariants(materializedVariants),
		NoSteppingStoneRationale: input.NoSteppingStoneRationale,
	})
	a.StructuredData = string(sd)

	if len(diversityWarnings) > 0 {
		a.Body += "\n## Diversity Warnings\n\n"
		for _, w := range diversityWarnings {
			a.Body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	if recall != "" {
		a.Body += recall
	}

	if ectx.Mode == ModeStandard || ectx.Mode == ModeDeep {
		a.Body += "\n## SoTA Survey Reminder\n\n"
		a.Body += "Before deciding, have you surveyed existing solutions?\n"
		a.Body += "- **Web search** — industry patterns, blog posts, case studies\n"
		a.Body += "- **Library docs** — check current API/usage patterns for relevant libraries\n"
		a.Body += "- **FPF spec search** — `haft_query(action=\"fpf\", query=\"<topic>\")` for methodology patterns\n"
		a.Body += "\n## Evidence Collection\n\n"
		a.Body += "Research each variant before comparing. Run tests, check benchmarks, validate claims.\n"
		a.Body += fmt.Sprintf("Attach findings: `haft_decision(action=\"evidence\", artifact_ref=\"%s\", evidence_content=\"...\", evidence_type=\"research\", evidence_verdict=\"supports\")`\n", a.Meta.ID)
	}

	return a
}

// ExploreSolutions creates a SolutionPortfolio. Orchestrates effects around BuildPortfolioArtifact.
func ExploreSolutions(ctx context.Context, store ArtifactStore, haftDir string, input ExploreInput) (*Artifact, string, error) {
	if err := ValidateExploreInput(input); err != nil {
		return nil, "", err
	}

	diversityWarnings := CheckVariantDiversity(input.Variants)

	// Effects: resolve problem reference
	var problemTitle string
	var links []Link
	resolvedContext := input.Context
	resolvedMode := input.Mode
	if input.ProblemRef != "" {
		prob, err := store.Get(ctx, input.ProblemRef)
		if err != nil {
			return nil, "", fmt.Errorf("problem %s not found: %w", input.ProblemRef, err)
		}
		if prob.Meta.Kind != KindProblemCard {
			return nil, "", fmt.Errorf("%s is %s, not ProblemCard", input.ProblemRef, prob.Meta.Kind)
		}
		problemTitle = prob.Meta.Title
		links = append(links, Link{Ref: input.ProblemRef, Type: "based_on"})
		if resolvedContext == "" {
			resolvedContext = prob.Meta.Context
		}
		if resolvedMode == "" {
			resolvedMode = string(prob.Meta.Mode)
		}
	}

	seq, err := store.NextSequence(ctx, KindSolutionPortfolio)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindSolutionPortfolio, seq)
	var mode Mode
	if resolvedMode == "" {
		mode = ModeStandard
	} else {
		var err error
		mode, err = ParseMode(resolvedMode)
		if err != nil {
			mode = ModeStandard // fallback: inherited mode may be from older schema
		}
	}

	recall := recallRelated(ctx, store, problemTitle)

	// Pure construction
	a := BuildPortfolioArtifact(ExploreContext{
		ID:           id,
		Now:          time.Now().UTC(),
		ProblemTitle: problemTitle,
		Context:      resolvedContext,
		Mode:         mode,
		Links:        links,
	}, input, diversityWarnings, recall)

	// Effects: persist
	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store portfolio: %w", err)
	}

	filePath, err := WriteFile(haftDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// CompareSolutions adds comparison results to an existing SolutionPortfolio.
func CompareSolutions(ctx context.Context, store ArtifactStore, haftDir string, input CompareInput) (*Artifact, string, error) {
	if input.PortfolioRef == "" {
		return nil, "", fmt.Errorf("portfolio_ref is required")
	}

	a, err := store.Get(ctx, input.PortfolioRef)
	if err != nil {
		return nil, "", fmt.Errorf("portfolio %s not found: %w", input.PortfolioRef, err)
	}
	if a.Meta.Kind != KindSolutionPortfolio {
		return nil, "", fmt.Errorf("%s is %s, not SolutionPortfolio", input.PortfolioRef, a.Meta.Kind)
	}

	mode := a.Meta.Mode
	if mode == "" {
		mode = ModeStandard
	}

	identities := portfolioVariantIdentities(a)
	if len(identities) == 0 {
		return nil, "", fmt.Errorf("portfolio %s declares no recoverable variants — repair or re-explore it before comparing", input.PortfolioRef)
	}
	if err := validatePortfolioVariantIdentities(identities); err != nil {
		return nil, "", fmt.Errorf("portfolio %s has ambiguous variant identities: %w", input.PortfolioRef, err)
	}
	normalizedResults, err := normalizeComparisonVariantReferences(input.Results, identities)
	if err != nil {
		return nil, "", err
	}

	validationContext := CompareValidationContext{
		Mode:              mode,
		PortfolioVariants: portfolioVariantKeys(identities),
		ParitySource:      parityPlanSourceNone,
	}

	if normalizedResults.ParityPlan != nil {
		validationContext.ParityPlan = cloneParityPlan(normalizedResults.ParityPlan)
		validationContext.ParitySource = parityPlanSourceExplicit
	}

	links, _ := store.GetLinks(ctx, input.PortfolioRef)
	for _, link := range links {
		if link.Type != "based_on" {
			continue
		}
		prob, err := store.Get(ctx, link.Ref)
		if err != nil || prob.Meta.Kind != KindProblemCard {
			continue
		}

		validationContext.CharacterizedDimensions = characterizedDimensionsForProblem(prob)
		if validationContext.ParityPlan == nil {
			validationContext.ParityPlan, validationContext.ParitySource = resolveParityPlan(prob)
			validationContext.ParityPlan = normalizeParityPlanVariantReferences(validationContext.ParityPlan, identities)
		}
		break
	}

	input.Results = normalizedResults

	validation, err := ValidateCompareInput(input, validationContext)
	if err != nil {
		return nil, "", err
	}

	validatedResults := normalizeComparisonResult(input.Results, validation.ComparedVariants)
	validatedResults.NonDominatedSet = append([]string(nil), validation.ComputedParetoFront...)
	validatedResults.ParityPlan = cloneParityPlan(validation.EffectiveParity)

	// Merge constraint-eliminated variants into the dominated_variants list so the UI
	// can display them with a clear reason even though they never entered dominance comparison.
	if len(validation.ConstraintEliminated) > 0 {
		existingDominated := make(map[string]bool, len(validatedResults.DominatedVariants))
		for _, note := range validatedResults.DominatedVariants {
			existingDominated[note.Variant] = true
		}
		for variantID, reason := range validation.ConstraintEliminated {
			if existingDominated[variantID] {
				continue
			}
			validatedResults.DominatedVariants = append(validatedResults.DominatedVariants, DominatedVariantExplanation{
				Variant: variantID,
				Summary: reason,
			})
		}
	}

	// Pure: build comparison section + apply to body
	a.Body = BuildComparisonBody(a.Body, validatedResults, validation.ComparedVariants, validation.Warnings)

	fields := a.UnmarshalPortfolioFields()
	fields.Comparison = cloneComparisonResult(validatedResults)
	sd, _ := json.Marshal(fields)
	a.StructuredData = string(sd)

	// Effects: persist
	if err := store.Update(ctx, a); err != nil {
		return nil, "", fmt.Errorf("update portfolio: %w", err)
	}

	filePath, err := WriteFile(haftDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// ValidateParityPlan checks that a structured parity plan is complete enough for strict enforcement.
func ValidateParityPlan(plan ParityPlan) error {
	normalized := normalizeParityPlan(&plan)
	if normalized == nil {
		return fmt.Errorf("parity plan is required")
	}
	if len(normalized.BaselineSet) == 0 {
		return fmt.Errorf("parity plan: baseline_set is required")
	}
	if normalized.Window == "" {
		return fmt.Errorf("parity plan: window is required")
	}
	if normalized.Budget == "" {
		return fmt.Errorf("parity plan: budget is required")
	}
	switch normalized.MissingDataPolicy {
	case MissingDataPolicyExplicitAbstain, MissingDataPolicyZero, MissingDataPolicyExclude:
	default:
		return fmt.Errorf("parity plan: missing_data_policy must be one of %q, %q, or %q",
			MissingDataPolicyExplicitAbstain, MissingDataPolicyZero, MissingDataPolicyExclude)
	}
	return nil
}

func parityPlanWarning(mode Mode, source parityPlanSource) string {
	if mode != ModeStandard && mode != ModeDeep {
		return ""
	}

	modeLabel := string(mode)

	if source == parityPlanSourceNone {
		return fmt.Sprintf(
			"%s mode comparison proceeds without a parity_plan — declare baseline_set, window, budget, and missing_data_policy",
			modeLabel,
		)
	}

	if source == parityPlanSourceExplicit {
		return fmt.Sprintf(
			"%s mode comparison received an unstructured parity_plan — fill baseline_set, window, budget, and missing_data_policy",
			modeLabel,
		)
	}

	return fmt.Sprintf(
		"%s mode comparison is using legacy parity notes only — add a structured parity_plan with baseline_set, window, budget, and missing_data_policy",
		modeLabel,
	)
}

// ValidateCompareInput applies FPF compare-time validation without side effects.
func ValidateCompareInput(input CompareInput, ctx CompareValidationContext) (CompareValidationResult, error) {
	result := CompareValidationResult{}
	if len(input.Results.Dimensions) == 0 {
		return result, fmt.Errorf("at least one comparison dimension is required")
	}

	rawScoredVariants := comparedVariantsFromScores(input.Results.Scores)
	if len(rawScoredVariants) == 0 {
		return result, fmt.Errorf("scores must include at least one compared variant")
	}

	var warnings []string
	effectiveParity := cloneParityPlan(ctx.ParityPlan)
	comparedVariants := dedupeTrimmedStrings(ctx.PortfolioVariants)
	switch {
	case effectiveParity != nil && effectiveParity.IsStructured():
		if err := ValidateParityPlan(*effectiveParity); err != nil {
			return result, err
		}
		result.EffectiveParity = effectiveParity
	case effectiveParity != nil:
		result.EffectiveParity = effectiveParity
		warning := parityPlanWarning(ctx.Mode, ctx.ParitySource)
		if warning != "" {
			warnings = append(warnings, warning)
		}
	default:
		warning := parityPlanWarning(ctx.Mode, ctx.ParitySource)
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}

	if result.EffectiveParity != nil && result.EffectiveParity.IsStructured() {
		if len(comparedVariants) > 0 {
			for _, variantID := range result.EffectiveParity.BaselineSet {
				if !containsString(comparedVariants, variantID) {
					return result, fmt.Errorf("parity plan baseline variant %q is not declared in the portfolio", variantID)
				}
			}
		}
		for _, variantID := range result.EffectiveParity.BaselineSet {
			if _, ok := input.Results.Scores[variantID]; !ok {
				return result, fmt.Errorf("parity plan baseline variant %q has no score entry", variantID)
			}
		}
		comparedVariants = append([]string(nil), result.EffectiveParity.BaselineSet...)
	}
	if len(comparedVariants) == 0 {
		comparedVariants = append([]string(nil), rawScoredVariants...)
	}
	result.ComparedVariants = append([]string(nil), comparedVariants...)

	for _, variantID := range rawScoredVariants {
		if containsString(comparedVariants, variantID) {
			continue
		}
		return result, fmt.Errorf("scored variant %q is outside the declared compare set", variantID)
	}

	for _, variantID := range input.Results.NonDominatedSet {
		if !containsString(comparedVariants, variantID) {
			return result, fmt.Errorf("non_dominated_set variant %q is outside the declared compare set", variantID)
		}
		if _, ok := input.Results.Scores[variantID]; !ok {
			return result, fmt.Errorf("non_dominated_set variant %q has no score entry", variantID)
		}
	}
	if input.Results.SelectedRef != "" {
		if !containsString(comparedVariants, input.Results.SelectedRef) {
			return result, fmt.Errorf("selected_ref %q is outside the declared compare set", input.Results.SelectedRef)
		}
		if _, ok := input.Results.Scores[input.Results.SelectedRef]; !ok {
			return result, fmt.Errorf("selected_ref %q has no score entry", input.Results.SelectedRef)
		}
	}
	for _, note := range input.Results.DominatedVariants {
		variantID := strings.TrimSpace(note.Variant)
		if variantID == "" {
			return result, fmt.Errorf("dominated_variants entry is missing variant")
		}
		if !containsString(comparedVariants, variantID) {
			return result, fmt.Errorf("dominated_variants variant %q is outside the declared compare set", variantID)
		}
		if strings.TrimSpace(note.Summary) == "" {
			return result, fmt.Errorf("dominated_variants variant %q is missing summary", variantID)
		}
		for _, dominator := range note.DominatedBy {
			if !containsString(comparedVariants, dominator) {
				return result, fmt.Errorf("dominated_by variant %q is outside the declared compare set", dominator)
			}
		}
	}
	for _, note := range input.Results.ParetoTradeoffs {
		variantID := strings.TrimSpace(note.Variant)
		if variantID == "" {
			return result, fmt.Errorf("pareto_tradeoffs entry is missing variant")
		}
		if !containsString(comparedVariants, variantID) {
			return result, fmt.Errorf("pareto_tradeoffs variant %q is outside the declared compare set", variantID)
		}
		if strings.TrimSpace(note.Summary) == "" {
			return result, fmt.Errorf("pareto_tradeoffs variant %q is missing summary", variantID)
		}
	}
	for _, pair := range input.Results.Incomparable {
		for _, variantID := range pair {
			if !containsString(comparedVariants, variantID) {
				return result, fmt.Errorf("incomparable variant %q is outside the declared compare set", variantID)
			}
		}
	}

	charByName := make(map[string]charDim)
	compareDims := make(map[string]string)
	for _, dimension := range input.Results.Dimensions {
		compareDims[normalizeArtifactKey(dimension)] = dimension
	}

	now := time.Now().UTC()
	for _, dimension := range ctx.CharacterizedDimensions {
		normalizedName := normalizeArtifactKey(dimension.Name)
		charByName[normalizedName] = dimension

		expiry, ok := parseComparisonExpiry(dimension.ValidUntil)
		if ok && expiry.Before(now) {
			return result, fmt.Errorf("dimension '%s' expired on %s — re-characterize before comparing (FPF B.3.4)",
				dimension.Name, expiry.Format("2006-01-02"))
		}

		_, present := compareDims[normalizedName]
		switch dimension.Role {
		case "constraint":
			if !present {
				return result, fmt.Errorf("comparison missing constraint dimension '%s'", dimension.Name)
			}
		case "target", "":
			if !present {
				return result, fmt.Errorf("comparison missing target dimension '%s'", dimension.Name)
			}
		case "observation":
			if !present {
				warnings = append(warnings,
					fmt.Sprintf("Observation dimension '%s' is not in comparison — omission is allowed but now explicit", dimension.Name))
			}
		}
	}

	missingDataPolicy := comparisonMissingDataPolicy(result.EffectiveParity)
	for _, dimension := range input.Results.Dimensions {
		role := "target"
		if characterized, ok := charByName[normalizeArtifactKey(dimension)]; ok && characterized.Role != "" {
			role = characterized.Role
		}
		if role == "observation" {
			continue
		}

		missingVariants := missingScoresForDimension(comparedVariants, input.Results.Scores, dimension)
		if len(missingVariants) == 0 {
			continue
		}
		if missingDataPolicy == MissingDataPolicyExclude || missingDataPolicy == MissingDataPolicyZero {
			warnings = append(warnings,
				fmt.Sprintf("dimension '%s' has missing scores for variants %s; continuing under missing_data_policy=%s",
					dimension, strings.Join(missingVariants, ", "), missingDataPolicy))
			continue
		}
		if role == "constraint" {
			return result, fmt.Errorf("constraint dimension '%s' missing values for variants: %s", dimension, strings.Join(missingVariants, ", "))
		}
		return result, fmt.Errorf("target dimension '%s' missing scores for variants: %s", dimension, strings.Join(missingVariants, ", "))
	}

	paretoResult := computeParetoFront(
		input.Results,
		comparedVariants,
		ctx.CharacterizedDimensions,
		missingDataPolicy,
	)
	computedFront := paretoResult.front
	result.ComputedParetoFront = computedFront
	result.ConstraintEliminated = paretoResult.constraintEliminated
	warnings = append(warnings, paretoResult.warnings...)
	if len(input.Results.NonDominatedSet) > 0 && !sameTrimmedSet(input.Results.NonDominatedSet, computedFront) {
		warnings = append(warnings,
			fmt.Sprintf("provided non_dominated_set disagrees with the computed Pareto front; storing computed front: %s",
				strings.Join(computedFront, ", ")))
	}
	if input.Results.SelectedRef != "" && !containsString(computedFront, input.Results.SelectedRef) {
		warnings = append(warnings,
			fmt.Sprintf("selected_ref %q is outside the computed Pareto front; verify that the compared dimensions capture the real selection policy",
				input.Results.SelectedRef))
	}
	warnings = append(warnings, parityChecklistWarnings(ctx.CharacterizedDimensions)...)

	// Constraint-eliminated variants don't require user-provided explanations —
	// they were removed by the system, not by manual dominance reasoning.
	explanationVariants := comparedVariants
	if len(paretoResult.constraintEliminated) > 0 {
		explanationVariants = make([]string, 0, len(comparedVariants))
		for _, v := range comparedVariants {
			if _, eliminated := paretoResult.constraintEliminated[v]; !eliminated {
				explanationVariants = append(explanationVariants, v)
			}
		}
	}

	if err := validateComparisonExplanationCoverage(
		explanationVariants,
		computedFront,
		input.Results.DominatedVariants,
		input.Results.ParetoTradeoffs,
	); err != nil {
		return result, err
	}
	result.Warnings = warnings
	return result, nil
}

// BuildComparisonBody appends comparison results to an existing portfolio body. Pure.
func BuildComparisonBody(existingBody string, results ComparisonResult, comparedVariants []string, warnings []string) string {
	var section strings.Builder
	displayLabels := portfolioVariantDisplayLabels(existingBody)
	section.WriteString("\n## Comparison\n\n")

	header := "| Variant |"
	sep := "|---------|"
	for _, d := range results.Dimensions {
		header += fmt.Sprintf(" %s |", d)
		sep += "------|"
	}
	section.WriteString(header + "\n")
	section.WriteString(sep + "\n")

	for _, variantID := range comparedVariants {
		scores := results.Scores[variantID]
		row := fmt.Sprintf("| %s |", displayVariantLabel(variantID, displayLabels))
		for _, d := range results.Dimensions {
			val := scoreForDimension(scores, d)
			if isMissingScore(val) {
				val = "-"
			}
			row += fmt.Sprintf(" %s |", val)
		}
		section.WriteString(row + "\n")
	}
	section.WriteString("\n")

	if results.ParityPlan != nil {
		section.WriteString("**Parity plan:**\n")
		if len(results.ParityPlan.BaselineSet) > 0 {
			section.WriteString(fmt.Sprintf("- Baseline set: %s\n",
				strings.Join(displayVariantLabels(results.ParityPlan.BaselineSet, displayLabels), ", ")))
		}
		if results.ParityPlan.Window != "" {
			section.WriteString(fmt.Sprintf("- Window: %s\n", results.ParityPlan.Window))
		}
		if results.ParityPlan.Budget != "" {
			section.WriteString(fmt.Sprintf("- Budget: %s\n", results.ParityPlan.Budget))
		}
		if results.ParityPlan.MissingDataPolicy != "" {
			section.WriteString(fmt.Sprintf("- Missing data policy: %s\n", results.ParityPlan.MissingDataPolicy))
		}
		for _, condition := range results.ParityPlan.PinnedConditions {
			section.WriteString(fmt.Sprintf("- Pinned condition: %s\n", condition))
		}
		section.WriteString("\n")
	}

	section.WriteString(fmt.Sprintf("## Non-Dominated Set\n\n**Computed Pareto front:** %s\n\n",
		strings.Join(displayVariantLabels(results.NonDominatedSet, displayLabels), ", ")))

	if len(results.DominatedVariants) > 0 {
		section.WriteString("## Dominated Variant Elimination\n\n")
		for _, note := range results.DominatedVariants {
			variantLabel := displayVariantLabel(note.Variant, displayLabels)
			summary := strings.TrimSpace(note.Summary)
			dominatedBy := strings.Join(displayVariantLabels(note.DominatedBy, displayLabels), ", ")
			switch {
			case dominatedBy != "":
				section.WriteString(fmt.Sprintf("- %s — dominated by %s. %s\n", variantLabel, dominatedBy, summary))
			default:
				section.WriteString(fmt.Sprintf("- %s — %s\n", variantLabel, summary))
			}
		}
		section.WriteString("\n")
	}

	if len(results.ParetoTradeoffs) > 0 {
		section.WriteString("## Pareto Front Trade-Offs\n\n")
		for _, note := range results.ParetoTradeoffs {
			variantLabel := displayVariantLabel(note.Variant, displayLabels)
			summary := strings.TrimSpace(note.Summary)
			section.WriteString(fmt.Sprintf("- %s — %s\n", variantLabel, summary))
		}
		section.WriteString("\n")
	}

	if len(results.Incomparable) > 0 {
		section.WriteString("**Incomparable pairs:**\n")
		for _, pair := range results.Incomparable {
			labels := displayVariantLabels(pair, displayLabels)
			if len(labels) == 2 {
				section.WriteString(fmt.Sprintf("- %s vs %s\n", labels[0], labels[1]))
				continue
			}
			section.WriteString(fmt.Sprintf("- %s\n", strings.Join(labels, " vs ")))
		}
		section.WriteString("\n")
	}

	if results.PolicyApplied != "" {
		section.WriteString(fmt.Sprintf("**Selection policy:** %s\n\n", results.PolicyApplied))
	}

	if results.SelectedRef != "" {
		section.WriteString(fmt.Sprintf("**Recommendation (advisory):** %s\n\n", displayVariantLabel(results.SelectedRef, displayLabels)))
	}

	if strings.TrimSpace(results.RecommendationRationale) != "" {
		section.WriteString(fmt.Sprintf("**Recommendation rationale:** %s\n\n", strings.TrimSpace(results.RecommendationRationale)))
	}

	// Strip existing comparison if present
	body := existingBody
	if idx := strings.Index(body, "\n## Comparison"); idx != -1 {
		body = body[:idx]
	}
	body += section.String()

	if len(warnings) > 0 {
		body += "\n## Comparison Warnings\n\n"
		for _, w := range warnings {
			body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	return body
}

// ExtractComparisonWarnings returns the rendered warning lines from a comparison body.
func ExtractComparisonWarnings(body string) []string {
	section := extractMarkdownSection(body, "## Comparison Warnings")
	if section == "" {
		return nil
	}

	lines := strings.Split(section, "\n")
	warnings := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Comparison Warnings" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "⚠"))
		if trimmed == "" {
			continue
		}
		warnings = append(warnings, trimmed)
	}

	if len(warnings) == 0 {
		return nil
	}

	return warnings
}

// charDim holds a parsed dimension with its indicator role and freshness.
type charDim struct {
	Name       string
	Role       string // constraint, target, observation
	Polarity   string // higher_better, lower_better
	ValidUntil string // measurement freshness (RFC3339 or empty)
}

type pairwiseDominance int

const (
	pairwiseTie pairwiseDominance = iota
	pairwiseLeftDominates
	pairwiseRightDominates
	pairwiseIncomparable
)

type dimensionComparisonStatus int

const (
	dimensionComparisonComparable dimensionComparisonStatus = iota
	dimensionComparisonExcluded
	dimensionComparisonUnresolved
)

type parsedScoreKind int

const (
	parsedScoreUnknown parsedScoreKind = iota
	parsedScoreNumeric
	parsedScoreOrdinal
)

type numericScore struct {
	Value float64
	Unit  string
}

type ordinalScore struct {
	Rank int
}

type parsedScore struct {
	Kind    parsedScoreKind
	Numeric numericScore
	Ordinal ordinalScore
}

var numericScorePattern = regexp.MustCompile(`^\s*([^\d+\-]*)([+\-]?\d[\d,]*(?:\.\d+)?)([kKmMbB]?)(.*)$`)

type portfolioVariantIdentity struct {
	Key     string
	Label   string
	Aliases []string
}

func hasSteppingStone(variants []Variant) bool {
	for _, variant := range variants {
		if variant.SteppingStone {
			return true
		}
	}
	return false
}

func normalizeArtifactKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func comparedVariantsFromScores(scores map[string]map[string]string) []string {
	var variants []string
	for variantID := range scores {
		variants = append(variants, variantID)
	}
	sort.Strings(variants)
	return variants
}

func materializeVariantIDs(variants []Variant) []Variant {
	materialized := cloneVariants(variants)
	for i := range materialized {
		if strings.TrimSpace(materialized[i].ID) != "" {
			continue
		}
		materialized[i].ID = fmt.Sprintf("V%d", i+1)
	}
	return materialized
}

func ValidateVariantIdentitySet(variants []Variant) error {
	return validatePortfolioVariantIdentities(variantIdentitiesFromVariants(variants))
}

func validatePortfolioVariantIdentities(identities []portfolioVariantIdentity) error {
	seenKeys := make(map[string]string)
	seenAliases := make(map[string]string)
	for _, identity := range identities {
		key := strings.TrimSpace(identity.Key)
		if key == "" {
			continue
		}

		label := strings.TrimSpace(identity.Label)
		if label == "" {
			label = key
		}

		if priorLabel, exists := seenKeys[key]; exists {
			return fmt.Errorf("variant identity %q is duplicated between %q and %q", key, priorLabel, label)
		}
		seenKeys[key] = label

		aliases := append([]string{key}, identity.Aliases...)
		for _, alias := range dedupeTrimmedStrings(aliases) {
			if alias == "" {
				continue
			}
			if priorKey, exists := seenAliases[alias]; exists && priorKey != key {
				return fmt.Errorf("variant alias %q is ambiguous between %q and %q", alias, priorKey, key)
			}
			seenAliases[alias] = key
		}
	}

	return nil
}

func variantIdentitiesFromVariants(variants []Variant) []portfolioVariantIdentity {
	var identities []portfolioVariantIdentity
	for _, variant := range variants {
		key := strings.TrimSpace(variant.ID)
		if key == "" {
			key = strings.TrimSpace(variant.Title)
		}
		if key == "" {
			continue
		}

		identities = append(identities, portfolioVariantIdentity{
			Key:   key,
			Label: strings.TrimSpace(variant.Title),
			Aliases: dedupeTrimmedStrings([]string{
				key,
				strings.TrimSpace(variant.Title),
			}),
		})
	}
	return identities
}

func portfolioVariantIdentities(portfolio *Artifact) []portfolioVariantIdentity {
	if portfolio == nil {
		return nil
	}

	fields := portfolio.UnmarshalPortfolioFields()
	bodyRefs := extractPortfolioVariantRefs(portfolio.Body)
	if len(fields.Variants) == 0 {
		return portfolioVariantIdentitiesFromRefs(bodyRefs)
	}

	var identities []portfolioVariantIdentity
	for index, variant := range fields.Variants {
		key := strings.TrimSpace(variant.ID)
		if key == "" && index < len(bodyRefs) {
			key = strings.TrimSpace(bodyRefs[index].ID)
		}
		if key == "" {
			key = strings.TrimSpace(variant.Title)
		}
		if key == "" {
			continue
		}

		label := strings.TrimSpace(variant.Title)
		if label == "" && index < len(bodyRefs) {
			label = strings.TrimSpace(bodyRefs[index].Title)
		}
		if label == "" {
			label = key
		}

		aliases := []string{key}
		aliases = append(aliases, strings.TrimSpace(variant.Title))
		if index < len(bodyRefs) {
			aliases = append(aliases, strings.TrimSpace(bodyRefs[index].ID))
			aliases = append(aliases, strings.TrimSpace(bodyRefs[index].Title))
		}

		identities = append(identities, portfolioVariantIdentity{
			Key:     key,
			Label:   label,
			Aliases: dedupeTrimmedStrings(aliases),
		})
	}

	return identities
}

func portfolioVariantIdentitiesFromRefs(refs []portfolioVariantRef) []portfolioVariantIdentity {
	var identities []portfolioVariantIdentity
	for _, ref := range refs {
		key := strings.TrimSpace(ref.ID)
		if key == "" {
			key = strings.TrimSpace(ref.Title)
		}
		if key == "" {
			continue
		}

		label := strings.TrimSpace(ref.Title)
		if label == "" {
			label = key
		}

		identities = append(identities, portfolioVariantIdentity{
			Key:   key,
			Label: label,
			Aliases: dedupeTrimmedStrings([]string{
				key,
				ref.ID,
				ref.Title,
			}),
		})
	}
	return identities
}

func portfolioVariantKeys(identities []portfolioVariantIdentity) []string {
	var keys []string
	for _, identity := range identities {
		keys = append(keys, identity.Key)
	}
	return dedupeTrimmedStrings(keys)
}

type portfolioVariantRef struct {
	ID    string
	Title string
}

func extractPortfolioVariantRefs(body string) []portfolioVariantRef {
	var refs []portfolioVariantRef
	lines := strings.Split(body, "\n")
	inVariants := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## Variants"):
			inVariants = true
			continue
		case inVariants && strings.HasPrefix(trimmed, "## "):
			return refs
		case inVariants && strings.HasPrefix(trimmed, "### "):
			header := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			parts := strings.SplitN(header, ". ", 2)
			ref := portfolioVariantRef{}
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				ref.ID = strings.TrimSpace(parts[0])
				ref.Title = strings.TrimSpace(parts[1])
				refs = append(refs, ref)
				continue
			}
			ref.Title = header
			refs = append(refs, ref)
		}
	}
	return refs
}

func portfolioVariantDisplayLabels(body string) map[string]string {
	labels := make(map[string]string)
	for _, ref := range extractPortfolioVariantRefs(body) {
		key := strings.TrimSpace(ref.ID)
		if key == "" {
			key = strings.TrimSpace(ref.Title)
		}
		if key == "" {
			continue
		}

		label := strings.TrimSpace(ref.Title)
		if label == "" {
			label = key
		}

		labels[key] = label
		if strings.TrimSpace(ref.Title) != "" {
			labels[strings.TrimSpace(ref.Title)] = label
		}
	}
	return labels
}

func displayVariantLabels(values []string, labels map[string]string) []string {
	var displayed []string
	for _, value := range values {
		displayed = append(displayed, displayVariantLabel(value, labels))
	}
	return displayed
}

func displayVariantLabel(value string, labels map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	if label, ok := labels[trimmed]; ok {
		return label
	}
	return trimmed
}

func containsString(values []string, target string) bool {
	normalizedTarget := strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == normalizedTarget {
			return true
		}
	}
	return false
}

func validateComparisonExplanationCoverage(
	comparedVariants []string,
	nonDominatedSet []string,
	dominatedVariants []DominatedVariantExplanation,
	paretoTradeoffs []ParetoTradeoffNote,
) error {
	expectedDominated := comparisonExplanationExpectations(comparedVariants, nonDominatedSet)
	if err := validateDominatedVariantCoverage(expectedDominated, dominatedVariants); err != nil {
		return err
	}

	expectedPareto := dedupeTrimmedStrings(nonDominatedSet)
	if err := validateParetoTradeoffCoverage(expectedPareto, paretoTradeoffs); err != nil {
		return err
	}

	return nil
}

func comparisonExplanationExpectations(comparedVariants []string, nonDominatedSet []string) []string {
	var expected []string
	for _, variantID := range comparedVariants {
		if containsString(nonDominatedSet, variantID) {
			continue
		}
		expected = append(expected, strings.TrimSpace(variantID))
	}
	return dedupeTrimmedStrings(expected)
}

func validateDominatedVariantCoverage(expected []string, notes []DominatedVariantExplanation) error {
	seen := make(map[string]int, len(notes))
	for _, note := range notes {
		variantID := strings.TrimSpace(note.Variant)
		if variantID == "" {
			continue
		}
		seen[variantID]++
	}

	if duplicates := duplicateComparisonExplanations(seen); len(duplicates) > 0 {
		return fmt.Errorf("dominated_variants must explain each dominated variant exactly once; duplicate entries for: %s",
			strings.Join(duplicates, ", "))
	}
	if missing := missingComparisonExplanations(expected, seen); len(missing) > 0 {
		return fmt.Errorf("dominated_variants must explain every compared variant outside the Pareto front; missing: %s",
			strings.Join(missing, ", "))
	}

	return nil
}

func validateParetoTradeoffCoverage(expected []string, notes []ParetoTradeoffNote) error {
	seen := make(map[string]int, len(notes))
	for _, note := range notes {
		variantID := strings.TrimSpace(note.Variant)
		if variantID == "" {
			continue
		}
		seen[variantID]++
	}

	if duplicates := duplicateComparisonExplanations(seen); len(duplicates) > 0 {
		return fmt.Errorf("pareto_tradeoffs must explain each Pareto-front variant exactly once; duplicate entries for: %s",
			strings.Join(duplicates, ", "))
	}
	if missing := missingComparisonExplanations(expected, seen); len(missing) > 0 {
		return fmt.Errorf("pareto_tradeoffs must explain every Pareto-front variant; missing: %s",
			strings.Join(missing, ", "))
	}

	return nil
}

func duplicateComparisonExplanations(seen map[string]int) []string {
	var duplicates []string
	for variantID, count := range seen {
		if count <= 1 {
			continue
		}
		duplicates = append(duplicates, variantID)
	}
	sort.Strings(duplicates)
	return duplicates
}

func missingComparisonExplanations(expected []string, seen map[string]int) []string {
	var missing []string
	for _, variantID := range expected {
		if seen[variantID] == 0 {
			missing = append(missing, variantID)
		}
	}
	sort.Strings(missing)
	return missing
}

func portfolioVariantAliasMap(identities []portfolioVariantIdentity) map[string]string {
	aliasMap := make(map[string]string)
	for _, identity := range identities {
		for _, alias := range identity.Aliases {
			trimmed := strings.TrimSpace(alias)
			if trimmed == "" {
				continue
			}
			aliasMap[trimmed] = identity.Key
		}
	}
	return aliasMap
}

func isMissingScore(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == "-"
}

func scoreForDimension(scores map[string]string, dimension string) string {
	if scores == nil {
		return ""
	}
	if value, ok := scores[dimension]; ok {
		return value
	}

	target := normalizeArtifactKey(dimension)
	for key, value := range scores {
		if normalizeArtifactKey(key) == target {
			return value
		}
	}
	return ""
}

func missingScoresForDimension(variants []string, scores map[string]map[string]string, dimension string) []string {
	var missing []string
	for _, variantID := range variants {
		if isMissingScore(scoreForDimension(scores[variantID], dimension)) {
			missing = append(missing, variantID)
		}
	}
	return missing
}

func parityChecklistWarnings(dims []charDim) []string {
	if len(dims) == 0 {
		return nil
	}

	warnings := []string{"Parity checklist (per characterized dimension):"}
	for _, dimension := range dims {
		switch dimension.Role {
		case "constraint":
			warnings = append(warnings,
				fmt.Sprintf("  - CONSTRAINT '%s': is it satisfied by all variants?", dimension.Name))
		case "observation":
			warnings = append(warnings,
				fmt.Sprintf("  - OBSERVE '%s': monitored under same conditions? (excluded from Pareto dominance)", dimension.Name))
		default:
			warnings = append(warnings,
				fmt.Sprintf("  - TARGET '%s': measured under same conditions for all variants?", dimension.Name))
		}
	}
	return warnings
}

func parseComparisonExpiry(raw string) (time.Time, bool) {
	return reff.ParseValidUntil(raw)
}

func characterizedDimensionsForProblem(problem *Artifact) []charDim {
	snapshot := latestStructuredCharacterization(problem)
	if snapshot != nil {
		return structuredCharacterizedDimensions(*snapshot)
	}
	return extractCharacterizedDimensionsWithRoles(problem.Body)
}

func latestStructuredCharacterization(problem *Artifact) *CharacterizationSnapshot {
	if problem == nil {
		return nil
	}

	fields := problem.UnmarshalProblemFields()
	if len(fields.Characterizations) == 0 {
		return nil
	}

	snapshot := fields.Characterizations[len(fields.Characterizations)-1]
	return &snapshot
}

func structuredCharacterizedDimensions(snapshot CharacterizationSnapshot) []charDim {
	var dims []charDim
	for _, dimension := range snapshot.Dimensions {
		role := strings.TrimSpace(dimension.Role)
		if role == "" {
			role = "target"
		}
		polarity := strings.TrimSpace(dimension.Polarity)
		dims = append(dims, charDim{
			Name:       strings.TrimSpace(dimension.Name),
			Role:       role,
			Polarity:   polarity,
			ValidUntil: strings.TrimSpace(dimension.ValidUntil),
		})
	}
	return dims
}

func resolveParityPlan(problem *Artifact) (*ParityPlan, parityPlanSource) {
	if problem == nil {
		return nil, parityPlanSourceNone
	}

	snapshot := latestStructuredCharacterization(problem)
	if snapshot != nil && snapshot.ParityPlan != nil {
		plan := cloneParityPlan(snapshot.ParityPlan)
		if plan != nil && plan.IsStructured() {
			return plan, parityPlanSourceExplicit
		}
		return plan, parityPlanSourceLegacy
	}

	if legacyRules := extractLegacyParityRules(problem.Body); legacyRules != "" {
		return &ParityPlan{PinnedConditions: []string{legacyRules}}, parityPlanSourceLegacy
	}

	return nil, parityPlanSourceNone
}

func extractLegacyParityRules(body string) string {
	section := latestCharacterizationSection(body)
	if section == "" {
		return ""
	}

	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "**Parity rules:**") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "**Parity rules:**"))
		}
	}
	return ""
}

func latestCharacterizationSection(body string) string {
	for i := 100; i >= 1; i-- {
		marker := fmt.Sprintf("## Characterization v%d", i)
		section := extractMarkdownSection(body, marker)
		if section != "" {
			return section
		}
	}

	return ""
}

func extractMarkdownSection(body string, heading string) string {
	start := strings.Index(body, heading)
	if start == -1 {
		return ""
	}

	section := body[start:]
	nextHeading := strings.Index(section[len(heading):], "\n## ")
	if nextHeading == -1 {
		return section
	}

	return section[:len(heading)+nextHeading]
}

func mergeLegacyParityRules(parityPlan *ParityPlan, parityRules string) *ParityPlan {
	trimmedRules := strings.TrimSpace(parityRules)
	if parityPlan == nil && trimmedRules == "" {
		return nil
	}

	merged := normalizeParityPlan(parityPlan)
	if merged == nil {
		merged = &ParityPlan{}
	}
	if trimmedRules != "" {
		merged.PinnedConditions = append(merged.PinnedConditions, trimmedRules)
		merged.PinnedConditions = dedupeTrimmedStrings(merged.PinnedConditions)
	}
	return merged
}

func normalizeParityPlan(plan *ParityPlan) *ParityPlan {
	if plan == nil {
		return nil
	}

	normalized := &ParityPlan{
		BaselineSet:       dedupeTrimmedStrings(plan.BaselineSet),
		Window:            strings.TrimSpace(plan.Window),
		Budget:            strings.TrimSpace(plan.Budget),
		MissingDataPolicy: strings.TrimSpace(plan.MissingDataPolicy),
		PinnedConditions:  dedupeTrimmedStrings(plan.PinnedConditions),
	}

	for _, rule := range plan.Normalization {
		dimension := strings.TrimSpace(rule.Dimension)
		method := strings.TrimSpace(rule.Method)
		if dimension == "" && method == "" {
			continue
		}
		normalized.Normalization = append(normalized.Normalization, NormRule{
			Dimension: dimension,
			Method:    method,
		})
	}

	return normalized
}

func cloneParityPlan(plan *ParityPlan) *ParityPlan {
	return normalizeParityPlan(plan)
}

func cloneComparisonResult(result ComparisonResult) *ComparisonResult {
	cloned := &ComparisonResult{
		Dimensions:              append([]string(nil), result.Dimensions...),
		NonDominatedSet:         append([]string(nil), result.NonDominatedSet...),
		DominatedVariants:       cloneDominatedVariantExplanations(result.DominatedVariants),
		ParetoTradeoffs:         cloneParetoTradeoffNotes(result.ParetoTradeoffs),
		PolicyApplied:           result.PolicyApplied,
		SelectedRef:             result.SelectedRef,
		RecommendationRationale: result.RecommendationRationale,
		ParityPlan:              cloneParityPlan(result.ParityPlan),
	}

	for _, pair := range result.Incomparable {
		cloned.Incomparable = append(cloned.Incomparable, append([]string(nil), pair...))
	}

	if len(result.Scores) > 0 {
		cloned.Scores = make(map[string]map[string]string, len(result.Scores))
		for variantID, scores := range result.Scores {
			cloned.Scores[variantID] = make(map[string]string, len(scores))
			for dimension, value := range scores {
				cloned.Scores[variantID][dimension] = value
			}
		}
	}

	return cloned
}

func cloneDominatedVariantExplanations(notes []DominatedVariantExplanation) []DominatedVariantExplanation {
	cloned := make([]DominatedVariantExplanation, 0, len(notes))
	for _, note := range notes {
		cloned = append(cloned, DominatedVariantExplanation{
			Variant:     note.Variant,
			DominatedBy: append([]string(nil), note.DominatedBy...),
			Summary:     note.Summary,
		})
	}
	return cloned
}

func cloneParetoTradeoffNotes(notes []ParetoTradeoffNote) []ParetoTradeoffNote {
	cloned := make([]ParetoTradeoffNote, 0, len(notes))
	for _, note := range notes {
		cloned = append(cloned, ParetoTradeoffNote{
			Variant: note.Variant,
			Summary: note.Summary,
		})
	}
	return cloned
}

func normalizeDominatedVariantExplanations(notes []DominatedVariantExplanation, aliasMap map[string]string) []DominatedVariantExplanation {
	normalized := make([]DominatedVariantExplanation, 0, len(notes))
	for _, note := range notes {
		normalized = append(normalized, DominatedVariantExplanation{
			Variant:     normalizeVariantReference(note.Variant, aliasMap),
			DominatedBy: dedupeTrimmedStrings(normalizeVariantReferences(note.DominatedBy, aliasMap)),
			Summary:     strings.TrimSpace(note.Summary),
		})
	}
	return normalized
}

func normalizeParetoTradeoffNotes(notes []ParetoTradeoffNote, aliasMap map[string]string) []ParetoTradeoffNote {
	normalized := make([]ParetoTradeoffNote, 0, len(notes))
	for _, note := range notes {
		normalized = append(normalized, ParetoTradeoffNote{
			Variant: normalizeVariantReference(note.Variant, aliasMap),
			Summary: strings.TrimSpace(note.Summary),
		})
	}
	return normalized
}

func normalizeComparisonVariantReferences(result ComparisonResult, identities []portfolioVariantIdentity) (ComparisonResult, error) {
	aliasMap := portfolioVariantAliasMap(identities)
	normalized := ComparisonResult{
		Dimensions:              append([]string(nil), result.Dimensions...),
		Incomparable:            make([][]string, 0, len(result.Incomparable)),
		DominatedVariants:       normalizeDominatedVariantExplanations(result.DominatedVariants, aliasMap),
		ParetoTradeoffs:         normalizeParetoTradeoffNotes(result.ParetoTradeoffs, aliasMap),
		PolicyApplied:           result.PolicyApplied,
		SelectedRef:             normalizeVariantReference(result.SelectedRef, aliasMap),
		RecommendationRationale: strings.TrimSpace(result.RecommendationRationale),
		ParityPlan:              cloneParityPlan(result.ParityPlan),
	}

	normalized.NonDominatedSet = normalizeVariantReferences(result.NonDominatedSet, aliasMap)
	normalized.NonDominatedSet = dedupeTrimmedStrings(normalized.NonDominatedSet)

	if normalized.ParityPlan != nil {
		normalized.ParityPlan.BaselineSet = normalizeVariantReferences(normalized.ParityPlan.BaselineSet, aliasMap)
		normalized.ParityPlan.BaselineSet = dedupeTrimmedStrings(normalized.ParityPlan.BaselineSet)
	}

	if len(result.Scores) > 0 {
		normalized.Scores = make(map[string]map[string]string, len(result.Scores))
		for rawKey, scores := range result.Scores {
			canonicalKey := normalizeVariantReference(rawKey, aliasMap)
			if canonicalKey == "" {
				continue
			}
			if _, exists := normalized.Scores[canonicalKey]; exists {
				return ComparisonResult{}, fmt.Errorf("comparison includes duplicate score entries for variant %q", canonicalKey)
			}

			normalized.Scores[canonicalKey] = make(map[string]string, len(scores))
			for dimension, value := range scores {
				normalized.Scores[canonicalKey][dimension] = value
			}
		}
	}

	for _, pair := range result.Incomparable {
		normalizedPair := normalizeVariantReferences(pair, aliasMap)
		if len(normalizedPair) == 0 {
			continue
		}
		normalized.Incomparable = append(normalized.Incomparable, normalizedPair)
	}

	return normalized, nil
}

func normalizeParityPlanVariantReferences(plan *ParityPlan, identities []portfolioVariantIdentity) *ParityPlan {
	normalized := cloneParityPlan(plan)
	if normalized == nil {
		return nil
	}

	aliasMap := portfolioVariantAliasMap(identities)
	normalized.BaselineSet = normalizeVariantReferences(normalized.BaselineSet, aliasMap)
	normalized.BaselineSet = dedupeTrimmedStrings(normalized.BaselineSet)
	return normalized
}

func normalizeVariantReferences(values []string, aliasMap map[string]string) []string {
	var normalized []string
	for _, value := range values {
		normalizedValue := normalizeVariantReference(value, aliasMap)
		if normalizedValue == "" {
			continue
		}
		normalized = append(normalized, normalizedValue)
	}
	return normalized
}

func normalizeVariantReference(value string, aliasMap map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if canonical, ok := aliasMap[trimmed]; ok {
		return canonical
	}
	return trimmed
}

func normalizeComparisonResult(result ComparisonResult, comparedVariants []string) ComparisonResult {
	normalized := ComparisonResult{
		Dimensions:              append([]string(nil), result.Dimensions...),
		NonDominatedSet:         append([]string(nil), result.NonDominatedSet...),
		DominatedVariants:       cloneDominatedVariantExplanations(result.DominatedVariants),
		ParetoTradeoffs:         cloneParetoTradeoffNotes(result.ParetoTradeoffs),
		PolicyApplied:           result.PolicyApplied,
		SelectedRef:             result.SelectedRef,
		RecommendationRationale: result.RecommendationRationale,
		ParityPlan:              cloneParityPlan(result.ParityPlan),
	}

	for _, pair := range result.Incomparable {
		normalized.Incomparable = append(normalized.Incomparable, append([]string(nil), pair...))
	}

	if len(result.Scores) == 0 {
		return normalized
	}

	normalized.Scores = make(map[string]map[string]string, len(comparedVariants))
	for _, variantID := range comparedVariants {
		scores := result.Scores[variantID]
		if scores == nil {
			continue
		}
		normalized.Scores[variantID] = make(map[string]string, len(scores))
		for dimension, value := range scores {
			normalized.Scores[variantID][dimension] = value
		}
	}

	return normalized
}

func cloneVariants(variants []Variant) []Variant {
	cloned := make([]Variant, 0, len(variants))
	for _, variant := range variants {
		cloned = append(cloned, Variant{
			ID:                 variant.ID,
			Title:              variant.Title,
			Description:        variant.Description,
			Strengths:          append([]string(nil), variant.Strengths...),
			WeakestLink:        variant.WeakestLink,
			NoveltyMarker:      variant.NoveltyMarker,
			Risks:              append([]string(nil), variant.Risks...),
			SteppingStone:      variant.SteppingStone,
			SteppingStoneBasis: variant.SteppingStoneBasis,
			DiversityRole:      variant.DiversityRole,
			AssumptionNotes:    variant.AssumptionNotes,
			RollbackNotes:      variant.RollbackNotes,
			EvidenceRefs:       append([]string(nil), variant.EvidenceRefs...),
		})
	}
	return cloned
}

func cloneDimensions(dimensions []ComparisonDimension) []ComparisonDimension {
	cloned := make([]ComparisonDimension, 0, len(dimensions))
	cloned = append(cloned, dimensions...)
	return cloned
}

func dedupeTrimmedStrings(values []string) []string {
	seen := make(map[string]bool)
	var deduped []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func sameTrimmedSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	leftSet := dedupeTrimmedStrings(left)
	rightSet := dedupeTrimmedStrings(right)
	if len(leftSet) != len(rightSet) {
		return false
	}

	sort.Strings(leftSet)
	sort.Strings(rightSet)

	for index := range leftSet {
		if leftSet[index] != rightSet[index] {
			return false
		}
	}

	return true
}

// extractCharacterizedDimensions parses dimension names and roles from the latest
// Characterization table in a ProblemCard body. Returns nil if no characterization found.
func extractCharacterizedDimensions(body string) []string {
	dims := extractCharacterizedDimensionsWithRoles(body)
	if dims == nil {
		return nil
	}
	var names []string
	for _, d := range dims {
		names = append(names, d.Name)
	}
	return names
}

// extractCharacterizedDimensionsWithRoles parses dimension names and roles.
// Table format: | Dimension | Role | Scale | Unit | Polarity | Measurement |
// Old format (no Role column): | Dimension | Scale | Unit | Polarity | Measurement |
func extractCharacterizedDimensionsWithRoles(body string) []charDim {
	section := latestCharacterizationSection(body)
	if section == "" {
		return nil
	}

	var dims []charDim
	lines := strings.Split(section, "\n")
	inTable := false
	hasRoleColumn := false
	hasValidUntilColumn := false
	hasPolarityColumn := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		if strings.Contains(line, "Dimension") {
			hasRoleColumn = strings.Contains(line, "Role")
			hasValidUntilColumn = strings.Contains(line, "Valid Until")
			hasPolarityColumn = strings.Contains(line, "Polarity")
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name == "" || name == "-" {
			continue
		}
		role := "target"
		if hasRoleColumn && len(parts) >= 4 {
			r := strings.TrimSpace(parts[2])
			if r == "constraint" || r == "target" || r == "observation" {
				role = r
			}
		}
		polarity := ""
		if hasPolarityColumn {
			polarityIndex := 4
			if hasRoleColumn {
				polarityIndex = 5
			}
			if polarityIndex < len(parts) {
				value := strings.TrimSpace(parts[polarityIndex])
				if value != "" && value != "-" {
					polarity = value
				}
			}
		}
		validUntil := ""
		if hasValidUntilColumn {
			// Valid Until is the last data column
			lastCol := strings.TrimSpace(parts[len(parts)-2]) // -2 because last is empty after trailing |
			if lastCol != "" && lastCol != "-" {
				validUntil = lastCol
			}
		}
		dims = append(dims, charDim{Name: name, Role: role, Polarity: polarity, ValidUntil: validUntil})
	}
	return dims
}

// paretoFrontResult holds the Pareto front, warnings, and any constraint-eliminated variants.
type paretoFrontResult struct {
	front                []string
	warnings             []string
	constraintEliminated map[string]string // variantID -> reason (nil when none eliminated)
}

func computeParetoFront(
	results ComparisonResult,
	comparedVariants []string,
	characterized []charDim,
	missingDataPolicy string,
) paretoFrontResult {
	specs, warnings := effectiveDominanceDimensions(results.Dimensions, characterized)
	if len(specs) == 0 {
		warnings = append(warnings, "computed Pareto front is conservative because no dominance-relevant dimensions could be derived")
		return paretoFrontResult{
			front:    append([]string(nil), comparedVariants...),
			warnings: warnings,
		}
	}

	// Eliminate variants that violate constraint dimensions before dominance computation.
	// Constraints are hard limits — a variant that fails one is not just "worse", it's inadmissible.
	elimination := eliminateConstraintViolations(comparedVariants, results.Scores, specs, &warnings)
	surviving := elimination.surviving
	if len(surviving) == 0 {
		warnings = append(warnings, "all variants eliminated by constraint violations; returning original set as Pareto front")
		surviving = append([]string(nil), comparedVariants...)
	}

	dominatedBy := make(map[string]map[string]bool, len(surviving))
	for _, variantID := range surviving {
		dominatedBy[variantID] = make(map[string]bool)
	}

	for leftIndex := 0; leftIndex < len(surviving); leftIndex++ {
		leftVariant := surviving[leftIndex]
		for rightIndex := leftIndex + 1; rightIndex < len(surviving); rightIndex++ {
			rightVariant := surviving[rightIndex]
			relation := compareVariantPair(
				leftVariant,
				rightVariant,
				results.Scores,
				specs,
				missingDataPolicy,
			)
			switch relation {
			case pairwiseLeftDominates:
				dominatedBy[rightVariant][leftVariant] = true
			case pairwiseRightDominates:
				dominatedBy[leftVariant][rightVariant] = true
			}
		}
	}

	var nonDominated []string
	for _, variantID := range surviving {
		if len(dominatedBy[variantID]) != 0 {
			continue
		}
		nonDominated = append(nonDominated, variantID)
	}

	return paretoFrontResult{
		front:                nonDominated,
		warnings:             warnings,
		constraintEliminated: elimination.eliminated,
	}
}

// constraintEliminationResult pairs a surviving set with the eliminated variants and their reasons.
type constraintEliminationResult struct {
	surviving  []string
	eliminated map[string]string // variantID -> reason
}

// isExplicitConstraintViolation returns true when the score text is an explicit failure label
// (case-insensitive): "FAIL", "violated", "no", "false".
func isExplicitConstraintViolation(score string) bool {
	normalized := strings.ToLower(strings.TrimSpace(score))
	switch normalized {
	case "fail", "violated", "no", "false":
		return true
	}
	return false
}

// eliminateConstraintViolations removes variants that fail any constraint dimension.
// Two elimination passes run in order:
//  1. Explicit-label pass: if a variant's score is "FAIL", "violated", "no", or "false"
//     (case-insensitive), the variant is eliminated immediately — this is an absolute signal,
//     not a relative comparison.
//  2. Relative-worst pass (original logic): a variant that is strictly worse than ALL other
//     remaining variants on a constraint dimension gets eliminated.
//
// Missing data is treated conservatively: the variant is NOT eliminated.
func eliminateConstraintViolations(
	variants []string,
	scores map[string]map[string]string,
	specs []charDim,
	warnings *[]string,
) constraintEliminationResult {
	constraintSpecs := make([]charDim, 0)
	for _, spec := range specs {
		if spec.Role == "constraint" {
			constraintSpecs = append(constraintSpecs, spec)
		}
	}
	if len(constraintSpecs) == 0 {
		return constraintEliminationResult{
			surviving:  append([]string(nil), variants...),
			eliminated: nil,
		}
	}

	eliminated := make(map[string]string) // variantID → reason

	// Pass 1: explicit-label violations — absolute, not relative.
	for _, constraint := range constraintSpecs {
		for _, candidate := range variants {
			if _, alreadyOut := eliminated[candidate]; alreadyOut {
				continue
			}
			candidateScore := scoreForDimension(scores[candidate], constraint.Name)
			if isExplicitConstraintViolation(candidateScore) {
				eliminated[candidate] = fmt.Sprintf("Constraint violation: %s (score: %s)",
					constraint.Name, strings.TrimSpace(candidateScore))
			}
		}
	}

	// Pass 2: relative-worst — only among variants still surviving after pass 1.
	pass1Surviving := make([]string, 0, len(variants)-len(eliminated))
	for _, v := range variants {
		if _, out := eliminated[v]; !out {
			pass1Surviving = append(pass1Surviving, v)
		}
	}

	for _, constraint := range constraintSpecs {
		for _, candidate := range pass1Surviving {
			if _, alreadyOut := eliminated[candidate]; alreadyOut {
				continue
			}
			candidateScore := scoreForDimension(scores[candidate], constraint.Name)
			if strings.TrimSpace(candidateScore) == "" {
				continue // missing data — don't eliminate (conservative)
			}

			// Check if candidate is strictly worse than ALL other surviving variants on this constraint.
			worseCount := 0
			comparableCount := 0
			var bestOther string
			var bestOtherScore string
			for _, other := range pass1Surviving {
				if other == candidate {
					continue
				}
				if _, out := eliminated[other]; out {
					continue
				}
				otherScore := scoreForDimension(scores[other], constraint.Name)
				if strings.TrimSpace(otherScore) == "" {
					continue
				}
				comparison, status := compareDimensionValues(candidateScore, otherScore, constraint.Polarity, MissingDataPolicyExclude)
				if status != dimensionComparisonComparable {
					continue
				}
				comparableCount++
				if comparison == -1 { // candidate is worse
					worseCount++
					bestOther = other
					bestOtherScore = otherScore
				}
			}

			// Only eliminate if worse than ALL comparable others (unique worst).
			if comparableCount > 0 && worseCount == comparableCount {
				eliminated[candidate] = fmt.Sprintf("violates constraint '%s' (worst: %s, e.g. %s has %s)",
					constraint.Name, candidateScore, bestOther, bestOtherScore)
			}
		}
	}

	if len(eliminated) == 0 {
		return constraintEliminationResult{
			surviving:  append([]string(nil), variants...),
			eliminated: nil,
		}
	}

	surviving := make([]string, 0, len(variants)-len(eliminated))
	for _, v := range variants {
		if reason, out := eliminated[v]; out {
			*warnings = append(*warnings, fmt.Sprintf("variant '%s' eliminated: %s", v, reason))
			continue
		}
		surviving = append(surviving, v)
	}
	return constraintEliminationResult{
		surviving:  surviving,
		eliminated: eliminated,
	}
}

func effectiveDominanceDimensions(compareDimensions []string, characterized []charDim) ([]charDim, []string) {
	charByName := make(map[string]charDim, len(characterized))
	for _, dimension := range characterized {
		charByName[normalizeArtifactKey(dimension.Name)] = dimension
	}

	specs := make([]charDim, 0, len(compareDimensions))
	warnings := make([]string, 0, len(compareDimensions))
	for _, dimensionName := range compareDimensions {
		spec := charDim{
			Name:     dimensionName,
			Role:     "target",
			Polarity: inferDimensionPolarity(dimensionName),
		}
		if characterizedDimension, ok := charByName[normalizeArtifactKey(dimensionName)]; ok {
			spec.Role = characterizedDimension.Role
			spec.Polarity = characterizedDimension.Polarity
			if spec.Polarity == "" {
				spec.Polarity = inferDimensionPolarity(dimensionName)
			}
		}
		if spec.Role == "observation" {
			continue
		}
		if spec.Polarity == "" {
			warnings = append(warnings,
				fmt.Sprintf("dimension '%s' has no polarity; excluding it from Pareto dominance computation", dimensionName))
			continue
		}
		specs = append(specs, spec)
	}

	return specs, warnings
}

func compareVariantPair(
	leftVariant string,
	rightVariant string,
	scores map[string]map[string]string,
	specs []charDim,
	missingDataPolicy string,
) pairwiseDominance {
	leftBetter := false
	rightBetter := false
	unresolved := false
	comparableDimensions := 0

	for _, spec := range specs {
		leftValue := scoreForDimension(scores[leftVariant], spec.Name)
		rightValue := scoreForDimension(scores[rightVariant], spec.Name)
		comparison, status := compareDimensionValues(leftValue, rightValue, spec.Polarity, missingDataPolicy)
		switch status {
		case dimensionComparisonExcluded:
			continue
		case dimensionComparisonUnresolved:
			unresolved = true
			continue
		}

		comparableDimensions++
		switch comparison {
		case 1:
			leftBetter = true
		case -1:
			rightBetter = true
		}
	}

	switch {
	case comparableDimensions == 0:
		return pairwiseIncomparable
	case unresolved:
		return pairwiseIncomparable
	case leftBetter && !rightBetter:
		return pairwiseLeftDominates
	case rightBetter && !leftBetter:
		return pairwiseRightDominates
	case !leftBetter && !rightBetter:
		return pairwiseTie
	default:
		return pairwiseIncomparable
	}
}

func compareDimensionValues(
	leftValue string,
	rightValue string,
	polarity string,
	missingDataPolicy string,
) (int, dimensionComparisonStatus) {
	leftTrimmed := strings.TrimSpace(leftValue)
	rightTrimmed := strings.TrimSpace(rightValue)
	if normalizeArtifactKey(leftTrimmed) == normalizeArtifactKey(rightTrimmed) {
		return 0, dimensionComparisonComparable
	}

	leftMissing := isMissingScore(leftTrimmed)
	rightMissing := isMissingScore(rightTrimmed)
	if leftMissing || rightMissing {
		switch missingDataPolicy {
		case MissingDataPolicyExclude:
			return 0, dimensionComparisonExcluded
		case MissingDataPolicyExplicitAbstain, "":
			return 0, dimensionComparisonUnresolved
		case MissingDataPolicyZero:
			filledLeft, filledRight, ok := zeroFillScoreValues(leftTrimmed, rightTrimmed)
			if !ok {
				return 0, dimensionComparisonUnresolved
			}
			leftTrimmed = filledLeft
			rightTrimmed = filledRight
		default:
			return 0, dimensionComparisonUnresolved
		}
	}

	leftScore, leftOK := parseComparableScore(leftTrimmed)
	rightScore, rightOK := parseComparableScore(rightTrimmed)
	if !leftOK || !rightOK {
		return 0, dimensionComparisonUnresolved
	}

	comparison, comparable := compareParsedScores(leftScore, rightScore, polarity)
	if !comparable {
		return 0, dimensionComparisonUnresolved
	}

	return comparison, dimensionComparisonComparable
}

func orientComparison(raw int, polarity string) int {
	switch strings.TrimSpace(polarity) {
	case "higher_better":
		return raw
	case "lower_better":
		return -raw
	default:
		return 0
	}
}

func compareOrderedValues(left float64, right float64) int {
	if math.Abs(left-right) < 1e-9 {
		return 0
	}
	if left > right {
		return 1
	}
	return -1
}

func parseNumericScore(value string) (numericScore, bool) {
	matches := numericScorePattern.FindStringSubmatch(value)
	if len(matches) != 5 {
		return numericScore{}, false
	}

	numberValue, err := strconv.ParseFloat(strings.ReplaceAll(matches[2], ",", ""), 64)
	if err != nil {
		return numericScore{}, false
	}

	multiplier := 1.0
	switch strings.ToLower(matches[3]) {
	case "k":
		multiplier = 1_000
	case "m":
		multiplier = 1_000_000
	case "b":
		multiplier = 1_000_000_000
	}

	unit := normalizeNumericUnit(matches[1] + matches[4])
	return numericScore{
		Value: numberValue * multiplier,
		Unit:  unit,
	}, true
}

func normalizeNumericUnit(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func parseOrdinalScore(value string) (ordinalScore, bool) {
	rank, ok := ordinalScoreRanks[normalizeArtifactKey(value)]
	if !ok {
		return ordinalScore{}, false
	}
	return ordinalScore{Rank: rank}, true
}

func parseComparableScore(value string) (parsedScore, bool) {
	if numeric, ok := parseNumericScore(value); ok {
		return parsedScore{
			Kind:    parsedScoreNumeric,
			Numeric: numeric,
		}, true
	}

	if ordinal, ok := parseOrdinalScore(value); ok {
		return parsedScore{
			Kind:    parsedScoreOrdinal,
			Ordinal: ordinal,
		}, true
	}

	return parsedScore{}, false
}

func compareParsedScores(left parsedScore, right parsedScore, polarity string) (int, bool) {
	if left.Kind != right.Kind {
		return 0, false
	}

	switch left.Kind {
	case parsedScoreNumeric:
		if left.Numeric.Unit != right.Numeric.Unit {
			return 0, false
		}
		comparison := compareOrderedValues(left.Numeric.Value, right.Numeric.Value)
		return orientComparison(comparison, polarity), true
	case parsedScoreOrdinal:
		comparison := compareOrderedValues(float64(left.Ordinal.Rank), float64(right.Ordinal.Rank))
		return orientComparison(comparison, polarity), true
	default:
		return 0, false
	}
}

func zeroFillScoreValues(leftValue string, rightValue string) (string, string, bool) {
	leftMissing := isMissingScore(leftValue)
	rightMissing := isMissingScore(rightValue)
	if leftMissing && rightMissing {
		return "", "", false
	}

	if leftMissing {
		rightScore, ok := parseComparableScore(rightValue)
		if !ok {
			return "", "", false
		}
		return renderZeroScore(rightScore), rightValue, true
	}

	if rightMissing {
		leftScore, ok := parseComparableScore(leftValue)
		if !ok {
			return "", "", false
		}
		return leftValue, renderZeroScore(leftScore), true
	}

	return leftValue, rightValue, true
}

func renderZeroScore(score parsedScore) string {
	switch score.Kind {
	case parsedScoreNumeric:
		return "0" + score.Numeric.Unit
	case parsedScoreOrdinal:
		return "none"
	default:
		return ""
	}
}

var ordinalScoreRanks = map[string]int{
	"none":      0,
	"very low":  1,
	"minimal":   1,
	"low":       2,
	"small":     2,
	"slow":      2,
	"limited":   2,
	"medium":    3,
	"moderate":  3,
	"ok":        3,
	"high":      4,
	"large":     4,
	"fast":      4,
	"good":      4,
	"very high": 5,
	"severe":    5,
	"best":      5,
}

func inferDimensionPolarity(name string) string {
	normalized := normalizeArtifactKey(name)
	for _, hint := range []string{
		"latency",
		"cost",
		"complexity",
		"overhead",
		"error",
		"risk",
		"time",
		"duration",
		"load",
		"cpu",
		"memory",
		"page",
		"downtime",
		"variance",
	} {
		if strings.Contains(normalized, hint) {
			return "lower_better"
		}
	}

	for _, hint := range []string{
		"throughput",
		"speed",
		"availability",
		"reliability",
		"accuracy",
		"coverage",
		"headroom",
		"novelty",
		"value",
		"quality",
	} {
		if strings.Contains(normalized, hint) {
			return "higher_better"
		}
	}

	return ""
}

func comparisonMissingDataPolicy(plan *ParityPlan) string {
	if plan == nil {
		return MissingDataPolicyExplicitAbstain
	}

	policy := strings.TrimSpace(plan.MissingDataPolicy)
	if policy == "" {
		return MissingDataPolicyExplicitAbstain
	}

	return policy
}

// jaccardSimilarity computes Jaccard index (intersection/union) of word sets from two texts.
func jaccardSimilarity(a, b string) float64 {
	wordsA := wordSet(a)
	wordsB := wordSet(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 0
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// containment computes what fraction of words in 'subset' appear in 'superset'.
// Answers: "is the topic of 'subset' already covered by 'superset'?"
func containment(subset, superset string) float64 {
	subWords := wordSet(subset)
	superWords := wordSet(superset)
	if len(subWords) == 0 {
		return 0
	}

	intersection := 0
	for w := range subWords {
		if superWords[w] {
			intersection++
		}
	}

	return float64(intersection) / float64(len(subWords))
}

// wordSet splits text into a set of lowercase words, stripping punctuation.
func wordSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		// Strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()-[]{}/*")
		if len(w) > 1 { // skip single chars
			set[w] = true
		}
	}
	return set
}

// recallRelated searches for existing active artifacts related to the given query.
// Pass title + signal for better recall. Returns a markdown section or empty string.
func recallRelated(ctx context.Context, store ArtifactStore, query string) string {
	if store == nil || query == "" {
		return ""
	}

	results, err := store.Search(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return ""
	}

	// Filter to active only, skip self-matches by checking creation time (just created = skip)
	var related []*Artifact
	for _, r := range results {
		if r.Meta.Status != StatusActive && r.Meta.Status != StatusRefreshDue {
			continue
		}
		related = append(related, r)
		if len(related) >= 3 {
			break
		}
	}

	if len(related) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Related History\n\n")
	for _, r := range related {
		kindLabel := r.Meta.Kind.UserFacingLabel()
		sb.WriteString(fmt.Sprintf("- [%s] **%s** `%s`\n", kindLabel, r.Meta.Title, r.Meta.ID))
	}
	return sb.String()
}

// FindActivePortfolio returns the most recent active SolutionPortfolio for a context.
func FindActivePortfolio(ctx context.Context, store ArtifactStore, contextName string) (*Artifact, error) {
	var portfolios []*Artifact
	var err error

	if contextName != "" {
		all, e := store.ListByContext(ctx, contextName)
		if e != nil {
			return nil, e
		}
		for _, a := range all {
			if a.Meta.Kind == KindSolutionPortfolio && a.Meta.Status == StatusActive {
				portfolios = append(portfolios, a)
			}
		}
	} else {
		portfolios, err = store.ListByKind(ctx, KindSolutionPortfolio, 1)
		if err != nil {
			return nil, err
		}
	}

	if len(portfolios) == 0 {
		return nil, nil
	}
	return portfolios[0], nil
}
