package assurance

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"
)

// AssuranceReport contains details of the reliability calculation for AI explanation
type AssuranceReport struct {
	HolonID      string
	FinalScore   float64
	SelfScore    float64 // Score based on own evidence
	WeakestLink  string  // ID of the dependency pulling the score down
	DecayPenalty float64
	Factors      []string // Textual explanations for AI
}

// Calculator handles assurance logic
type Calculator struct {
	DB *sql.DB
}

// New creates a new Calculator
func New(db *sql.DB) *Calculator {
	return &Calculator{DB: db}
}

// CalculateReliability calculates R for a holon (public API)
func (c *Calculator) CalculateReliability(ctx context.Context, holonID string) (*AssuranceReport, error) {
	visited := make(map[string]bool)
	return c.calculateReliabilityWithVisited(ctx, holonID, visited)
}

// calculateReliabilityWithVisited is the internal implementation with cycle detection
func (c *Calculator) calculateReliabilityWithVisited(ctx context.Context, holonID string, visited map[string]bool) (*AssuranceReport, error) {
	// Cycle detection: if already visited, return neutral score to break cycle
	if visited[holonID] {
		return &AssuranceReport{
			HolonID:    holonID,
			FinalScore: 1.0, // Neutral - don't penalize for cycle
			SelfScore:  1.0,
			Factors:    []string{"Cycle detected, skipping re-evaluation"},
		}, nil
	}
	visited[holonID] = true

	report := &AssuranceReport{HolonID: holonID}

	// 1. Calculate Self Score (based on Evidence)
	// B.3.4: Check for expired evidence + evidence source CL penalty
	rows, err := c.DB.QueryContext(ctx, "SELECT type, verdict, valid_until FROM evidence WHERE holon_id = ?", holonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var minScore float64 = 1.0 // WLNK: track weakest evidence
	var hasEvidence bool
	for rows.Next() {
		var evidenceType, verdict string
		var validUntil *time.Time
		if err := rows.Scan(&evidenceType, &verdict, &validUntil); err != nil {
			continue
		}
		hasEvidence = true

		score := 0.0
		switch strings.ToLower(verdict) {
		case "pass":
			score = 1.0
		case "degrade":
			score = 0.5
		case "fail":
			score = 0.0
		}

		// Evidence Source CL Penalty (B.3: external evidence has lower congruence)
		// internal/audit_report → CL3 (0%), external → CL2 (10%)
		clPenalty := evidenceTypeToCLPenalty(evidenceType)
		if clPenalty > 0 {
			score = math.Max(0, score-clPenalty)
			report.Factors = append(report.Factors, "External evidence CL2 penalty applied")
		}

		// Evidence Decay Logic
		if validUntil != nil && time.Now().After(*validUntil) {
			report.Factors = append(report.Factors, "Evidence expired (Decay applied)")
			score = 0.1                // Penalty for expiration, not zero but close
			report.DecayPenalty += 0.9 // Track how much was lost
		}

		// WLNK: weakest evidence determines self score
		if score < minScore {
			minScore = score
		}
	}

	if hasEvidence {
		report.SelfScore = minScore // WLNK: weakest evidence determines score
	} else {
		report.SelfScore = 0.0 // L0: Unsubstantiated
		report.Factors = append(report.Factors, "No evidence found (L0)")
	}

	// 2. Calculate Dependencies Score (Weakest Link + CL Penalty)
	// B.3: R_eff = max(0, min(R_dep) - Penalty(CL))
	// Relation directionality:
	//   - componentOf: Part → Whole (source is part OF target)
	//   - dependsOn:   Dependent → Dependency (source DEPENDS ON target)
	// When calculating reliability for holonID:
	//   - componentOf: find rows where target_id = holonID, dependency is source_id
	//   - dependsOn:   find rows where source_id = holonID, dependency is target_id
	depRows, err := c.DB.QueryContext(ctx, `
		SELECT source_id AS dep_id, congruence_level FROM relations
		WHERE target_id = ? AND relation_type = 'componentOf'
		UNION
		SELECT target_id AS dep_id, congruence_level FROM relations
		WHERE source_id = ? AND relation_type = 'dependsOn'`, holonID, holonID)

	if err != nil {
		return nil, err
	}

	// Collect deps first to avoid holding cursor during recursive calls
	type dep struct {
		id string
		cl int
	}
	var deps []dep
	for depRows.Next() {
		var d dep
		if err := depRows.Scan(&d.id, &d.cl); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	_ = depRows.Close()

	minDepScore := 1.0
	for _, d := range deps {
		// Recursive call for dependency with visited map for cycle detection
		depReport, err := c.calculateReliabilityWithVisited(ctx, d.id, visited)
		if err != nil {
			depReport = &AssuranceReport{FinalScore: 0.0}
		}

		// CL Penalty: CL=3 (0.0), CL=2 (0.1), CL=1 (0.4), CL=0 (0.9)
		penalty := calculateCLPenalty(d.cl)
		effectiveR := math.Max(0, depReport.FinalScore-penalty)

		if effectiveR < minDepScore {
			minDepScore = effectiveR
			report.WeakestLink = d.id
		}

		if penalty > 0 {
			report.Factors = append(report.Factors, "CL Penalty applied for "+d.id)
		}
	}

	hasDeps := len(deps) > 0

	// 3. Weakest Link Principle (WLNK)
	// The final rating cannot be higher than the weakest link (self or dependency)
	if hasDeps {
		report.FinalScore = math.Min(report.SelfScore, minDepScore)
	} else {
		report.FinalScore = report.SelfScore
	}

	// Update cache (non-critical, log warning on failure)
	if _, err := c.DB.ExecContext(ctx, "UPDATE holons SET cached_r_score = ? WHERE id = ?", report.FinalScore, holonID); err != nil {
		report.Factors = append(report.Factors, "Warning: cache update failed")
	}

	return report, nil
}

func calculateCLPenalty(cl int) float64 {
	switch cl {
	case 3:
		return 0.0
	case 2:
		return 0.1
	case 1:
		return 0.4
	default:
		return 0.9
	}
}

// evidenceTypeToCLPenalty maps evidence source type to congruence penalty.
// internal/audit_report = CL3 (same context, no penalty)
// external = CL2 (similar context, 10% penalty)
// research = CL1 (different context, 40% penalty)
func evidenceTypeToCLPenalty(evidenceType string) float64 {
	switch strings.ToLower(evidenceType) {
	case "internal", "audit_report":
		return 0.0 // CL3: same context
	case "external":
		return 0.1 // CL2: similar context
	case "research":
		return 0.4 // CL1: different context
	default:
		return 0.0 // Unknown type, no penalty
	}
}
