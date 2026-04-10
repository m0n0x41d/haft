package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ReadinessReport assesses whether a portfolio is ready for fair comparison.
type ReadinessReport struct {
	PortfolioID       string   `json:"portfolio_id"`
	VariantCount      int      `json:"variant_count"`
	DimensionCount    int      `json:"dimension_count"`
	ScoreCoverage     float64  `json:"score_coverage"`     // 0-1: fraction of cells filled
	ConstraintCount   int      `json:"constraint_count"`   // dimensions with role=constraint
	MissingScores     []string `json:"missing_scores"`     // "Variant X / Dimension Y"
	HasParity         bool     `json:"has_parity"`
	Recommendation    string   `json:"recommendation"`     // commit, probe, widen, reroute
	RecommendationWhy string   `json:"recommendation_why"`
	Warnings          []string `json:"warnings"`
}

// AssessReadiness evaluates whether a portfolio comparison will be meaningful.
// Returns a recommendation: commit (ready), probe (need specific data),
// widen (need more variants), or reroute (wrong problem framing).
func AssessReadiness(ctx context.Context, db *sql.DB, portfolioID string) (*ReadinessReport, error) {
	report := &ReadinessReport{
		PortfolioID: portfolioID,
	}

	// Load portfolio
	var structuredData string
	var problemRef string
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(structured_data, '{}')
		FROM artifacts WHERE id = ? AND kind = 'SolutionPortfolio'
	`, portfolioID).Scan(&structuredData)
	if err != nil {
		return nil, fmt.Errorf("portfolio not found: %w", err)
	}

	var portfolio struct {
		ProblemRef string `json:"problem_ref"`
		Variants   []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"variants"`
		Comparison *struct {
			Dimensions []string                    `json:"dimensions"`
			Scores     map[string]map[string]string `json:"scores"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal([]byte(structuredData), &portfolio); err != nil {
		return nil, fmt.Errorf("parse portfolio: %w", err)
	}

	problemRef = portfolio.ProblemRef
	report.VariantCount = len(portfolio.Variants)

	// Load characterization dimensions from the problem
	var dimensions []struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if problemRef != "" {
		var problemSD string
		err := db.QueryRowContext(ctx, `
			SELECT COALESCE(structured_data, '{}')
			FROM artifacts WHERE id = ?
		`, problemRef).Scan(&problemSD)
		if err == nil {
			var problem struct {
				Characterizations []struct {
					Dimensions []struct {
						Name string `json:"name"`
						Role string `json:"role"`
					} `json:"dimensions"`
					ParityPlan *struct{} `json:"parity_plan"`
				} `json:"characterizations"`
			}
			if json.Unmarshal([]byte(problemSD), &problem) == nil && len(problem.Characterizations) > 0 {
				latest := problem.Characterizations[len(problem.Characterizations)-1]
				dimensions = latest.Dimensions
				report.HasParity = latest.ParityPlan != nil
			}
		}
	}

	report.DimensionCount = len(dimensions)

	for _, d := range dimensions {
		if d.Role == "constraint" {
			report.ConstraintCount++
		}
	}

	// Check score coverage
	totalCells := report.VariantCount * report.DimensionCount
	filledCells := 0
	if portfolio.Comparison != nil && totalCells > 0 {
		for _, variant := range portfolio.Variants {
			scores := portfolio.Comparison.Scores[variant.ID]
			for _, dim := range dimensions {
				if val, ok := scores[dim.Name]; ok && val != "" {
					filledCells++
				} else {
					report.MissingScores = append(report.MissingScores,
						fmt.Sprintf("%s / %s", variant.Title, dim.Name))
				}
			}
		}
		report.ScoreCoverage = float64(filledCells) / float64(totalCells)
	}

	// Generate warnings
	if report.VariantCount < 2 {
		report.Warnings = append(report.Warnings, "Fewer than 2 variants — comparison is meaningless")
	}
	if report.VariantCount == 2 {
		report.Warnings = append(report.Warnings, "Only 2 variants — consider a third for diversity")
	}
	if report.DimensionCount == 0 {
		report.Warnings = append(report.Warnings, "No characterized dimensions — comparison has no criteria")
	}
	if report.ConstraintCount == 0 && report.DimensionCount > 0 {
		report.Warnings = append(report.Warnings, "No constraint dimensions — all dimensions are soft targets")
	}
	if !report.HasParity && report.DimensionCount > 0 {
		report.Warnings = append(report.Warnings, "No parity plan — comparison conditions may not be fair")
	}
	if len(report.MissingScores) > 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("%d score cells missing — comparison will have gaps", len(report.MissingScores)))
	}

	// Recommendation
	switch {
	case report.VariantCount < 2:
		report.Recommendation = "widen"
		report.RecommendationWhy = "Need at least 2 genuinely distinct variants before comparing."
	case report.DimensionCount == 0:
		report.Recommendation = "reroute"
		report.RecommendationWhy = "No comparison dimensions defined. Characterize the problem first."
	case report.ScoreCoverage < 0.5 && totalCells > 0:
		report.Recommendation = "probe"
		report.RecommendationWhy = fmt.Sprintf("Only %.0f%% of scores filled. Gather data for missing cells before comparing.", report.ScoreCoverage*100)
	case report.ScoreCoverage < 0.8 && totalCells > 0:
		report.Recommendation = "probe"
		report.RecommendationWhy = fmt.Sprintf("%.0f%% of scores filled. Consider gathering remaining data for a stronger comparison.", report.ScoreCoverage*100)
	default:
		report.Recommendation = "commit"
		report.RecommendationWhy = "Portfolio has sufficient variants, dimensions, and score coverage for a meaningful comparison."
	}

	return report, nil
}
