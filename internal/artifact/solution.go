package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	Warnings         []string
	EffectiveParity  *ParityPlan
	ComparedVariants []string
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
	validatedResults.ParityPlan = cloneParityPlan(validation.EffectiveParity)

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

// ValidateCompareInput applies FPF compare-time validation without side effects.
func ValidateCompareInput(input CompareInput, ctx CompareValidationContext) (CompareValidationResult, error) {
	result := CompareValidationResult{}
	if len(input.Results.Dimensions) == 0 {
		return result, fmt.Errorf("at least one comparison dimension is required")
	}
	if len(input.Results.NonDominatedSet) == 0 {
		return result, fmt.Errorf("non_dominated_set is required — which variants are on the Pareto front?")
	}

	rawScoredVariants := comparedVariantsFromScores(input.Results.Scores)
	if len(rawScoredVariants) == 0 {
		return result, fmt.Errorf("scores must include at least one compared variant")
	}

	var warnings []string
	effectiveParity := cloneParityPlan(ctx.ParityPlan)
	comparedVariants := dedupeTrimmedStrings(ctx.PortfolioVariants)
	switch {
	case ctx.ParitySource == parityPlanSourceExplicit:
		if err := ValidateParityPlan(*effectiveParity); err != nil {
			return result, err
		}
		result.EffectiveParity = effectiveParity
	case effectiveParity != nil && effectiveParity.IsStructured():
		if err := ValidateParityPlan(*effectiveParity); err != nil {
			return result, err
		}
		result.EffectiveParity = effectiveParity
	case effectiveParity != nil:
		result.EffectiveParity = effectiveParity
		if ctx.Mode == ModeDeep {
			return result, fmt.Errorf("deep mode comparison requires a structured parity plan with baseline_set, window, budget, and missing_data_policy")
		}
		if ctx.Mode == ModeStandard {
			warnings = append(warnings,
				"standard mode comparison is using legacy parity notes only — add a structured parity plan with baseline_set, window, budget, and missing_data_policy")
		}
	default:
		if ctx.Mode == ModeDeep {
			return result, fmt.Errorf("deep mode comparison requires a parity plan")
		}
		if ctx.Mode == ModeStandard {
			warnings = append(warnings,
				"standard mode comparison proceeds without a parity plan — declare baseline_set, window, budget, and missing_data_policy")
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
		if role == "constraint" {
			return result, fmt.Errorf("constraint dimension '%s' missing values for variants: %s", dimension, strings.Join(missingVariants, ", "))
		}
		return result, fmt.Errorf("target dimension '%s' missing scores for variants: %s", dimension, strings.Join(missingVariants, ", "))
	}

	warnings = append(warnings, parityChecklistWarnings(ctx.CharacterizedDimensions)...)
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
			section.WriteString(fmt.Sprintf("- Baseline set: %s\n", strings.Join(results.ParityPlan.BaselineSet, ", ")))
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

	section.WriteString(fmt.Sprintf("## Non-Dominated Set\n\n**Pareto front:** %s\n\n",
		strings.Join(displayVariantLabels(results.NonDominatedSet, displayLabels), ", ")))

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
		section.WriteString(fmt.Sprintf("**Recommended:** %s\n\n", displayVariantLabel(results.SelectedRef, displayLabels)))
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

// charDim holds a parsed dimension with its indicator role and freshness.
type charDim struct {
	Name       string
	Role       string // constraint, target, observation
	ValidUntil string // measurement freshness (RFC3339 or empty)
}

type portfolioVariantIdentity struct {
	Key     string
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

		aliases := []string{key}
		aliases = append(aliases, strings.TrimSpace(variant.Title))
		if index < len(bodyRefs) {
			aliases = append(aliases, strings.TrimSpace(bodyRefs[index].ID))
			aliases = append(aliases, strings.TrimSpace(bodyRefs[index].Title))
		}

		identities = append(identities, portfolioVariantIdentity{
			Key:     key,
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

		identities = append(identities, portfolioVariantIdentity{
			Key: key,
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
				fmt.Sprintf("  - OBSERVE '%s': monitored under same conditions? (don't optimize)", dimension.Name))
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
		dims = append(dims, charDim{
			Name:       strings.TrimSpace(dimension.Name),
			Role:       role,
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
	lastIdx := -1
	for i := 100; i >= 1; i-- {
		marker := fmt.Sprintf("## Characterization v%d", i)
		if idx := strings.Index(body, marker); idx != -1 {
			lastIdx = idx
			break
		}
	}
	if lastIdx == -1 {
		return ""
	}

	section := body[lastIdx:]
	if endIdx := strings.Index(section[1:], "\n## "); endIdx != -1 {
		section = section[:endIdx+1]
	}
	return section
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
		Dimensions:      append([]string(nil), result.Dimensions...),
		NonDominatedSet: append([]string(nil), result.NonDominatedSet...),
		PolicyApplied:   result.PolicyApplied,
		SelectedRef:     result.SelectedRef,
		ParityPlan:      cloneParityPlan(result.ParityPlan),
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

func normalizeComparisonVariantReferences(result ComparisonResult, identities []portfolioVariantIdentity) (ComparisonResult, error) {
	aliasMap := portfolioVariantAliasMap(identities)
	normalized := ComparisonResult{
		Dimensions:    append([]string(nil), result.Dimensions...),
		Incomparable:  make([][]string, 0, len(result.Incomparable)),
		PolicyApplied: result.PolicyApplied,
		SelectedRef:   normalizeVariantReference(result.SelectedRef, aliasMap),
		ParityPlan:    cloneParityPlan(result.ParityPlan),
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
		Dimensions:      append([]string(nil), result.Dimensions...),
		NonDominatedSet: append([]string(nil), result.NonDominatedSet...),
		PolicyApplied:   result.PolicyApplied,
		SelectedRef:     result.SelectedRef,
		ParityPlan:      cloneParityPlan(result.ParityPlan),
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
	for _, dimension := range dimensions {
		cloned = append(cloned, dimension)
	}
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
		validUntil := ""
		if hasValidUntilColumn {
			// Valid Until is the last data column
			lastCol := strings.TrimSpace(parts[len(parts)-2]) // -2 because last is empty after trailing |
			if lastCol != "" && lastCol != "-" {
				validUntil = lastCol
			}
		}
		dims = append(dims, charDim{Name: name, Role: role, ValidUntil: validUntil})
	}
	return dims
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
		sb.WriteString(fmt.Sprintf("- [%s] **%s** `%s`\n", r.Meta.Kind, r.Meta.Title, r.Meta.ID))
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
