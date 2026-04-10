package main

import (
	"context"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// View models — clean DTOs for the React frontend.
// All fields have json tags (required for Wails TS binding generation).
//
// IMPORTANT: Go nil slices serialize as JSON null, not [].
// Use safeStrings/safeSlice helpers to ensure all slice fields
// are empty slices, never nil. Frontend code calls .map()/.length
// on these and will crash on null.

// safeStrings returns an empty slice if the input is nil.
func safeStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func safeArtifactViews(s []ArtifactView) []ArtifactView {
	if s == nil {
		return []ArtifactView{}
	}
	return s
}

func safeRejections(s []RejectionView) []RejectionView {
	if s == nil {
		return []RejectionView{}
	}
	return s
}

func safeClaims(s []ClaimView) []ClaimView {
	if s == nil {
		return []ClaimView{}
	}
	return s
}

func safeCoverageModules(s []CoverageModuleView) []CoverageModuleView {
	if s == nil {
		return []CoverageModuleView{}
	}
	return s
}

func safeVariants(s []VariantView) []VariantView {
	if s == nil {
		return []VariantView{}
	}
	return s
}

func safeDominatedNotes(s []DominatedNote) []DominatedNote {
	if s == nil {
		return []DominatedNote{}
	}
	return s
}

func safeTradeoffNotes(s []TradeoffNote) []TradeoffNote {
	if s == nil {
		return []TradeoffNote{}
	}
	return s
}

func safeCharacterizations(s []CharacterizationView) []CharacterizationView {
	if s == nil {
		return []CharacterizationView{}
	}
	return s
}

func safeDimensions(s []DimensionView) []DimensionView {
	if s == nil {
		return []DimensionView{}
	}
	return s
}
// No domain logic here — just data projection.

// DashboardView is the landing page data.
type DashboardView struct {
	ProjectName     string         `json:"project_name"`
	ProblemCount    int            `json:"problem_count"`
	DecisionCount   int            `json:"decision_count"`
	PortfolioCount  int            `json:"portfolio_count"`
	NoteCount       int            `json:"note_count"`
	StaleCount      int            `json:"stale_count"`
	RecentProblems  []ProblemView  `json:"recent_problems"`
	RecentDecisions []DecisionView `json:"recent_decisions"`
	StaleItems      []ArtifactView `json:"stale_items"`
}

// ArtifactView is the minimal representation for lists and search results.
type ArtifactView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Mode      string `json:"mode"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProblemView is a summary for problem lists.
type ProblemView struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Mode          string   `json:"mode"`
	Signal        string   `json:"signal"`
	Reversibility string   `json:"reversibility"`
	Constraints   []string `json:"constraints"`
	CreatedAt     string   `json:"created_at"`
}

// ProblemDetailView is the full problem card.
type ProblemDetailView struct {
	ID                     string                 `json:"id"`
	Title                  string                 `json:"title"`
	Status                 string                 `json:"status"`
	Mode                   string                 `json:"mode"`
	Signal                 string                 `json:"signal"`
	Constraints            []string               `json:"constraints"`
	OptimizationTargets    []string               `json:"optimization_targets"`
	ObservationIndicators  []string               `json:"observation_indicators"`
	Acceptance             string                 `json:"acceptance"`
	BlastRadius            string                 `json:"blast_radius"`
	Reversibility          string                 `json:"reversibility"`
	Characterizations      []CharacterizationView `json:"characterizations"`
	LatestCharacterization *CharacterizationView  `json:"latest_characterization,omitempty"`
	LinkedPortfolios       []ArtifactView         `json:"linked_portfolios"`
	LinkedDecisions        []ArtifactView         `json:"linked_decisions"`
	Body                   string                 `json:"body"`
	CreatedAt              string                 `json:"created_at"`
	UpdatedAt              string                 `json:"updated_at"`
}

type PortfolioSummaryView struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Mode          string `json:"mode"`
	ProblemRef    string `json:"problem_ref"`
	HasComparison bool   `json:"has_comparison"`
	CreatedAt     string `json:"created_at"`
}

// DecisionView is a summary for decision lists.
type DecisionView struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Mode          string `json:"mode"`
	SelectedTitle string `json:"selected_title"`
	WeakestLink   string `json:"weakest_link"`
	ValidUntil    string `json:"valid_until"`
	CreatedAt     string `json:"created_at"`
}

// DecisionDetailView is the full decision record.
type DecisionDetailView struct {
	ID                   string               `json:"id"`
	Title                string               `json:"title"`
	Status               string               `json:"status"`
	Mode                 string               `json:"mode"`
	ProblemRefs          []string             `json:"problem_refs"`
	SelectedTitle        string               `json:"selected_title"`
	WhySelected          string               `json:"why_selected"`
	SelectionPolicy      string               `json:"selection_policy"`
	CounterArgument      string               `json:"counterargument"`
	WeakestLink          string               `json:"weakest_link"`
	WhyNotOthers         []RejectionView      `json:"why_not_others"`
	Invariants           []string             `json:"invariants"`
	PreConditions        []string             `json:"pre_conditions"`
	PostConditions       []string             `json:"post_conditions"`
	Admissibility        []string             `json:"admissibility"`
	EvidenceRequirements []string             `json:"evidence_requirements"`
	RefreshTriggers      []string             `json:"refresh_triggers"`
	Claims               []ClaimView          `json:"claims"`
	FirstModuleCoverage  bool                 `json:"first_module_coverage"`
	AffectedFiles        []string             `json:"affected_files"`
	CoverageModules      []CoverageModuleView `json:"coverage_modules"`
	CoverageWarnings     []string             `json:"coverage_warnings"`
	RollbackTriggers     []string             `json:"rollback_triggers"`
	RollbackSteps        []string             `json:"rollback_steps"`
	RollbackBlastRadius  string               `json:"rollback_blast_radius"`
	ValidUntil           string               `json:"valid_until"`
	Body                 string               `json:"body"`
	CreatedAt            string               `json:"created_at"`
	UpdatedAt            string               `json:"updated_at"`
}

type RejectionView struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

type ClaimView struct {
	ID          string `json:"id"`
	Claim       string `json:"claim"`
	Observable  string `json:"observable"`
	Threshold   string `json:"threshold"`
	Status      string `json:"status"`
	VerifyAfter string `json:"verify_after"`
}

// PortfolioDetailView is the full solution portfolio with variants and comparison.
type PortfolioDetailView struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Status     string          `json:"status"`
	ProblemRef string          `json:"problem_ref"`
	Variants   []VariantView   `json:"variants"`
	Comparison *ComparisonView `json:"comparison"`
	Body       string          `json:"body"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

type VariantView struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	WeakestLink   string   `json:"weakest_link"`
	NoveltyMarker string   `json:"novelty_marker"`
	SteppingStone bool     `json:"stepping_stone"`
	Strengths     []string `json:"strengths"`
	Risks         []string `json:"risks"`
}

type ComparisonView struct {
	Dimensions      []string                     `json:"dimensions"`
	Scores          map[string]map[string]string `json:"scores"`
	NonDominatedSet []string                     `json:"non_dominated_set"`
	DominatedNotes  []DominatedNote              `json:"dominated_notes"`
	ParetoTradeoffs []TradeoffNote               `json:"pareto_tradeoffs"`
	PolicyApplied   string                       `json:"policy_applied"`
	SelectedRef     string                       `json:"selected_ref"`
	Recommendation  string                       `json:"recommendation"`
}

type DominatedNote struct {
	Variant     string   `json:"variant"`
	DominatedBy []string `json:"dominated_by"`
	Summary     string   `json:"summary"`
}

type TradeoffNote struct {
	Variant string `json:"variant"`
	Summary string `json:"summary"`
}

type CharacterizationView struct {
	Version    int             `json:"version"`
	Dimensions []DimensionView `json:"dimensions"`
	ParityPlan *ParityPlanView `json:"parity_plan,omitempty"`
}

type DimensionView struct {
	Name         string `json:"name"`
	ScaleType    string `json:"scale_type"`
	Unit         string `json:"unit"`
	Polarity     string `json:"polarity"`
	Role         string `json:"role"`
	HowToMeasure string `json:"how_to_measure"`
	ValidUntil   string `json:"valid_until"`
}

type ParityPlanView struct {
	BaselineSet       []string       `json:"baseline_set"`
	Window            string         `json:"window"`
	Budget            string         `json:"budget"`
	Normalization     []NormRuleView `json:"normalization"`
	MissingDataPolicy string         `json:"missing_data_policy"`
	PinnedConditions  []string       `json:"pinned_conditions"`
}

type NormRuleView struct {
	Dimension string `json:"dimension"`
	Method    string `json:"method"`
}

// --- Projection functions: domain types → view models ---

func toArtifactView(a *artifact.Artifact) ArtifactView {
	return ArtifactView{
		ID:        a.Meta.ID,
		Kind:      string(a.Meta.Kind),
		Title:     a.Meta.Title,
		Status:    string(a.Meta.Status),
		Mode:      string(a.Meta.Mode),
		CreatedAt: a.Meta.CreatedAt.Format("2006-01-02"),
		UpdatedAt: a.Meta.UpdatedAt.Format("2006-01-02"),
	}
}

func toProblemView(a *artifact.Artifact) ProblemView {
	pf := a.UnmarshalProblemFields()
	return ProblemView{
		ID:            a.Meta.ID,
		Title:         a.Meta.Title,
		Status:        string(a.Meta.Status),
		Mode:          string(a.Meta.Mode),
		Signal:        pf.Signal,
		Reversibility: pf.Reversibility,
		Constraints:   pf.Constraints,
		CreatedAt:     a.Meta.CreatedAt.Format("2006-01-02"),
	}
}

func toDecisionView(a *artifact.Artifact) DecisionView {
	df := a.UnmarshalDecisionFields()
	return DecisionView{
		ID:            a.Meta.ID,
		Title:         a.Meta.Title,
		Status:        string(a.Meta.Status),
		Mode:          string(a.Meta.Mode),
		SelectedTitle: df.SelectedTitle,
		WeakestLink:   df.WeakestLink,
		ValidUntil:    a.Meta.ValidUntil,
		CreatedAt:     a.Meta.CreatedAt.Format("2006-01-02"),
	}
}

func toProblemDetail(ctx context.Context, a *artifact.Artifact, store *artifact.Store) ProblemDetailView {
	pf := a.UnmarshalProblemFields()

	var linkedPortfolios, linkedDecisions []ArtifactView
	if store != nil {
		links, _ := store.GetBacklinks(ctx, a.Meta.ID)
		for _, link := range links {
			linked, err := store.Get(ctx, link.Ref)
			if err != nil {
				continue
			}
			v := toArtifactView(linked)
			switch linked.Meta.Kind {
			case artifact.KindSolutionPortfolio:
				linkedPortfolios = append(linkedPortfolios, v)
			case artifact.KindDecisionRecord:
				linkedDecisions = append(linkedDecisions, v)
			}
		}
	}

	return ProblemDetailView{
		ID:                     a.Meta.ID,
		Title:                  a.Meta.Title,
		Status:                 string(a.Meta.Status),
		Mode:                   string(a.Meta.Mode),
		Signal:                 pf.Signal,
		Constraints:            safeStrings(pf.Constraints),
		OptimizationTargets:    safeStrings(pf.OptimizationTargets),
		ObservationIndicators:  safeStrings(pf.ObservationIndicators),
		Acceptance:             pf.Acceptance,
		BlastRadius:            pf.BlastRadius,
		Reversibility:          pf.Reversibility,
		Characterizations:      safeCharacterizations(toCharacterizationViews(pf.Characterizations)),
		LatestCharacterization: latestCharacterizationView(pf.Characterizations),
		LinkedPortfolios:       safeArtifactViews(linkedPortfolios),
		LinkedDecisions:        safeArtifactViews(linkedDecisions),
		Body:                   a.Body,
		CreatedAt:              a.Meta.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:              a.Meta.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toPortfolioSummary(a *artifact.Artifact) PortfolioSummaryView {
	fields := a.UnmarshalPortfolioFields()

	return PortfolioSummaryView{
		ID:            a.Meta.ID,
		Title:         a.Meta.Title,
		Status:        string(a.Meta.Status),
		Mode:          string(a.Meta.Mode),
		ProblemRef:    fields.ProblemRef,
		HasComparison: fields.Comparison != nil,
		CreatedAt:     a.Meta.CreatedAt.Format("2006-01-02"),
	}
}

func toDecisionDetail(
	a *artifact.Artifact,
	affectedFiles []string,
	coverageModules []CoverageModuleView,
	coverageWarnings []string,
) DecisionDetailView {
	df := a.UnmarshalDecisionFields()

	rejections := make([]RejectionView, 0, len(df.WhyNotOthers))
	for _, r := range df.WhyNotOthers {
		rejections = append(rejections, RejectionView{Variant: r.Variant, Reason: r.Reason})
	}

	claims := make([]ClaimView, 0, len(df.Claims))
	for _, c := range df.Claims {
		claims = append(claims, ClaimView{
			ID:          c.ID,
			Claim:       c.Claim,
			Observable:  c.Observable,
			Threshold:   c.Threshold,
			Status:      string(c.Status),
			VerifyAfter: c.VerifyAfter,
		})
	}

	return DecisionDetailView{
		ID:                   a.Meta.ID,
		Title:                a.Meta.Title,
		Status:               string(a.Meta.Status),
		Mode:                 string(a.Meta.Mode),
		ProblemRefs:          safeStrings(df.ProblemRefs),
		SelectedTitle:        df.SelectedTitle,
		WhySelected:          df.WhySelected,
		SelectionPolicy:      df.SelectionPolicy,
		CounterArgument:      df.CounterArgument,
		WeakestLink:          df.WeakestLink,
		WhyNotOthers:         safeRejections(rejections),
		Invariants:           safeStrings(df.Invariants),
		PreConditions:        safeStrings(df.PreConditions),
		PostConditions:       safeStrings(df.PostConds),
		Admissibility:        safeStrings(df.Admissibility),
		EvidenceRequirements: safeStrings(df.EvidenceRequirements),
		RefreshTriggers:      safeStrings(df.RefreshTriggers),
		Claims:               safeClaims(claims),
		FirstModuleCoverage:  df.FirstModuleCoverage,
		AffectedFiles:        safeStrings(affectedFiles),
		CoverageModules:      safeCoverageModules(coverageModules),
		CoverageWarnings:     safeStrings(coverageWarnings),
		RollbackTriggers:     safeStrings(df.RollbackTriggers),
		RollbackSteps:        safeStrings(df.RollbackSteps),
		RollbackBlastRadius:  df.RollbackBlastRadius,
		ValidUntil:           a.Meta.ValidUntil,
		Body:                 a.Body,
		CreatedAt:            a.Meta.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:            a.Meta.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toPortfolioDetail(a *artifact.Artifact) PortfolioDetailView {
	pf := a.UnmarshalPortfolioFields()

	variants := make([]VariantView, 0, len(pf.Variants))
	for _, v := range pf.Variants {
		variants = append(variants, VariantView{
			ID:            v.ID,
			Title:         v.Title,
			Description:   v.Description,
			WeakestLink:   v.WeakestLink,
			NoveltyMarker: v.NoveltyMarker,
			SteppingStone: v.SteppingStone,
			Strengths:     safeStrings(v.Strengths),
			Risks:         safeStrings(v.Risks),
		})
	}

	var comparison *ComparisonView
	if pf.Comparison != nil {
		c := pf.Comparison
		dominated := make([]DominatedNote, 0, len(c.DominatedVariants))
		for _, d := range c.DominatedVariants {
			dominated = append(dominated, DominatedNote{
				Variant: d.Variant, DominatedBy: d.DominatedBy, Summary: d.Summary,
			})
		}
		tradeoffs := make([]TradeoffNote, 0, len(c.ParetoTradeoffs))
		for _, t := range c.ParetoTradeoffs {
			tradeoffs = append(tradeoffs, TradeoffNote{Variant: t.Variant, Summary: t.Summary})
		}
		comparison = &ComparisonView{
			Dimensions:      safeStrings(c.Dimensions),
			Scores:          c.Scores,
			NonDominatedSet: safeStrings(c.NonDominatedSet),
			DominatedNotes:  safeDominatedNotes(dominated),
			ParetoTradeoffs: safeTradeoffNotes(tradeoffs),
			PolicyApplied:   c.PolicyApplied,
			SelectedRef:     c.SelectedRef,
			Recommendation:  c.RecommendationRationale,
		}
	}

	return PortfolioDetailView{
		ID:         a.Meta.ID,
		Title:      a.Meta.Title,
		Status:     string(a.Meta.Status),
		ProblemRef: pf.ProblemRef,
		Variants:   safeVariants(variants),
		Comparison: comparison,
		Body:       a.Body,
		CreatedAt:  a.Meta.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  a.Meta.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func toCharacterizationViews(values []artifact.CharacterizationSnapshot) []CharacterizationView {
	views := make([]CharacterizationView, 0, len(values))

	for _, value := range values {
		views = append(views, CharacterizationView{
			Version:    value.Version,
			Dimensions: safeDimensions(toDimensionViews(value.Dimensions)),
			ParityPlan: toParityPlanView(value.ParityPlan),
		})
	}

	return views
}

func latestCharacterizationView(values []artifact.CharacterizationSnapshot) *CharacterizationView {
	if len(values) == 0 {
		return nil
	}

	latest := values[len(values)-1]
	view := CharacterizationView{
		Version:    latest.Version,
		Dimensions: toDimensionViews(latest.Dimensions),
		ParityPlan: toParityPlanView(latest.ParityPlan),
	}

	return &view
}

func toDimensionViews(values []artifact.ComparisonDimension) []DimensionView {
	dimensions := make([]DimensionView, 0, len(values))

	for _, value := range values {
		dimensions = append(dimensions, DimensionView{
			Name:         value.Name,
			ScaleType:    value.ScaleType,
			Unit:         value.Unit,
			Polarity:     value.Polarity,
			Role:         value.Role,
			HowToMeasure: value.HowToMeasure,
			ValidUntil:   value.ValidUntil,
		})
	}

	return dimensions
}

func toParityPlanView(value *artifact.ParityPlan) *ParityPlanView {
	if value == nil {
		return nil
	}

	view := &ParityPlanView{
		BaselineSet:       safeStrings(value.BaselineSet),
		Window:            value.Window,
		Budget:            value.Budget,
		MissingDataPolicy: value.MissingDataPolicy,
		PinnedConditions:  safeStrings(value.PinnedConditions),
	}

	for _, rule := range value.Normalization {
		view.Normalization = append(view.Normalization, NormRuleView{
			Dimension: rule.Dimension,
			Method:    rule.Method,
		})
	}

	return view
}
