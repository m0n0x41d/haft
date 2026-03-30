// Package reff provides R_eff (effective reliability) scoring functions
// shared between artifact and codebase packages.
// Pure functions, zero dependencies, no DB, no state.
package reff

import (
	"math"
	"time"
)

// ScoreEvidence computes the effective reliability score for a single evidence item.
// FPF B.3: R_eff = max(0, base_score - Φ(CL)), with decay override for expired evidence.
func ScoreEvidence(verdict string, cl int, validUntil string, now time.Time) float64 {
	if validUntil != "" {
		if t, err := time.Parse(time.RFC3339, validUntil); err == nil && t.Before(now) {
			return 0.1 // expired evidence is weak regardless of verdict (FPF B.3.4)
		}
	}

	base := VerdictToScore(verdict)
	penalty := CLPenalty(cl)
	return math.Max(0, base-penalty)
}

// VerdictToScore maps evidence verdict to base reliability score.
func VerdictToScore(verdict string) float64 {
	switch verdict {
	case "supports", "accepted":
		return 1.0
	case "weakens", "partial":
		return 0.5
	case "refutes", "failed":
		return 0.0
	default:
		return 0.5 // unknown verdict treated as weakening
	}
}

// CLPenalty returns the congruence level penalty per FPF B.3.
// CL3 (same context) = no penalty, CL0 (opposed) = near-total penalty.
func CLPenalty(cl int) float64 {
	switch cl {
	case 3:
		return 0.0
	case 2:
		return 0.1
	case 1:
		return 0.4
	default: // CL=0 or invalid
		return 0.9
	}
}
