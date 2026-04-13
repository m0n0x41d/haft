package assurance

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

type AssuranceReport struct {
	HolonID        string
	FinalScore     float64
	SelfScore      float64 // Score based on own evidence
	FormalityScore int     // F_eff = min(F_i) after normalizing legacy data to F0-F3
	WeakestLink    string  // ID of the dependency pulling the score down
	DecayPenalty   float64
	Factors        []string // Textual explanations for AI
}

type Calculator struct {
	DB *sql.DB
}

func New(db *sql.DB) *Calculator {
	return &Calculator{DB: db}
}

func (c *Calculator) CalculateReliability(ctx context.Context, holonID string) (*AssuranceReport, error) {
	visited := make(map[string]bool)
	return c.calculateReliabilityWithVisited(ctx, holonID, visited)
}

func (c *Calculator) calculateReliabilityWithVisited(ctx context.Context, holonID string, visited map[string]bool) (*AssuranceReport, error) {
	// Cycle detection: return neutral (1.0) to break cycle without penalizing
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
	now := time.Now().UTC()

	// 1. Calculate Self Score (based on Evidence)
	// B.3.4: Check for expired evidence + congruence penalty
	// C.2.3: F_eff = min(F_i) for all evidence (Formality level)
	rows, err := c.DB.QueryContext(ctx,
		`SELECT id, type, verdict, COALESCE(valid_until, ''), COALESCE(formality_level, 5), COALESCE(congruence_level, -1)
		 FROM evidence WHERE holon_id = ?`, holonID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	minScore := 1.0
	minFormality := 3
	var hasEvidence bool
	for rows.Next() {
		var evidenceID, evidenceType, verdict string
		var validUntil string
		var formalityLevel int
		var congruenceLevel int
		if err := rows.Scan(&evidenceID, &evidenceType, &verdict, &validUntil, &formalityLevel, &congruenceLevel); err != nil {
			continue
		}
		hasEvidence = true
		_ = evidenceID // Used for potential future logging

		effectiveCL := resolveEvidenceCongruenceLevel(evidenceType, congruenceLevel)
		score := reff.ScoreTypedEvidence(evidenceType, verdict, effectiveCL, validUntil, now)

		if effectiveCL < 3 {
			report.Factors = append(report.Factors, congruencePenaltyFactor(evidenceType, congruenceLevel, effectiveCL))
		}

		// Evidence Decay Logic (B.3.4: time-based expiration)
		if expiry, ok := reff.ParseValidUntil(validUntil); ok && now.After(expiry) {
			report.Factors = append(report.Factors, "Evidence expired (Decay applied)")
			score = 0.1                // Penalty for expiration, not zero but close
			report.DecayPenalty += 0.9 // Track how much was lost
		}

		// WLNK: weakest evidence determines self score
		if score < minScore {
			minScore = score
		}

		normalizedFormality := normalizeFormalityLevel(formalityLevel)
		if normalizedFormality < minFormality {
			minFormality = normalizedFormality
		}
	}

	if hasEvidence {
		report.SelfScore = minScore
		report.FormalityScore = minFormality
	} else {
		report.SelfScore = 0.0
		report.FormalityScore = 0
		report.Factors = append(report.Factors, "No evidence found (L0)")
	}

	// 2. Calculate Dependencies Score (Weakest Link + CL Penalty)
	// B.3: R_eff = max(0, min(R_dep) - Penalty(CL))
	// Relation directionality:
	//   - componentOf: Part → Whole (source is part OF target)
	//   - dependsOn / dependsOnProjected: source DEPENDS ON target
	// When calculating reliability for holonID:
	//   - componentOf: find rows where target_id = holonID, dependency is source_id
	//   - dependsOn*:  find rows where source_id = holonID, dependency is target_id
	depRows, err := c.DB.QueryContext(ctx, `
		SELECT source_id AS dep_id, congruence_level FROM relations
		WHERE target_id = ? AND relation_type = 'componentOf'
		UNION
		SELECT target_id AS dep_id, congruence_level FROM relations
		WHERE source_id = ? AND relation_type IN ('dependsOn', 'dependsOnProjected')`, holonID, holonID)

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
func evidenceTypeToCongruenceLevel(evidenceType string) int {
	switch strings.ToLower(evidenceType) {
	case "internal", "audit_report":
		return 3
	case "external":
		return 2
	case "research":
		return 1
	default:
		return 3
	}
}

func resolveEvidenceCongruenceLevel(evidenceType string, stored int) int {
	if stored >= 0 && stored <= 3 {
		return stored
	}
	return evidenceTypeToCongruenceLevel(evidenceType)
}

func congruencePenaltyFactor(evidenceType string, stored int, effective int) string {
	if stored < 0 {
		switch strings.ToLower(evidenceType) {
		case "external":
			return "External evidence CL2 penalty applied"
		case "research":
			return "Research evidence CL1 penalty applied"
		}
	}
	return "Evidence congruence penalty applied"
}

func normalizeFormalityLevel(level int) int {
	switch {
	case level < 0:
		return 0
	case level <= 3:
		return level
	case level <= 5:
		return 1
	case level <= 8:
		return 2
	default:
		return 3
	}
}
