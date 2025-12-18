package assurance

import (
	"context"
	"database/sql"
	"math"
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

// CalculateReliability calculates R for a holon
func (c *Calculator) CalculateReliability(ctx context.Context, holonID string) (*AssuranceReport, error) {
	report := &AssuranceReport{HolonID: holonID}

	// 1. Calculate Self Score (based on Evidence)
	// B.3.4: Check for expired evidence
	rows, err := c.DB.QueryContext(ctx, "SELECT verdict, valid_until FROM evidence WHERE holon_id = ?", holonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalScore, count float64
	for rows.Next() {
		var verdict string
		var validUntil *time.Time
		if err := rows.Scan(&verdict, &validUntil); err != nil {
			continue
		}

		score := 0.0
		switch verdict {
		case "pass":
			score = 1.0
		case "degrade":
			score = 0.5
		case "fail":
			score = 0.0
		}

		// Evidence Decay Logic
		if validUntil != nil && time.Now().After(*validUntil) {
			report.Factors = append(report.Factors, "Evidence expired (Decay applied)")
			score = 0.1 // Penalty for expiration, not zero but close
			report.DecayPenalty += 0.9 // Track how much was lost
		}
		totalScore += score
		count++
	}

	if count > 0 {
		report.SelfScore = totalScore / count // Or other aggregation logic
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
		if d.id != holonID { // Simple cycle check
			deps = append(deps, d)
		}
	}
	depRows.Close()

	minDepScore := 1.0
	for _, d := range deps {
		// Recursive call for dependency
		depReport, err := c.CalculateReliability(ctx, d.id)
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

	// Update cache
	// We ignore error here as it's a side effect
	_, _ = c.DB.ExecContext(ctx, "UPDATE holons SET cached_r_score = ? WHERE id = ?", report.FinalScore, holonID)

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
