// Package reff provides R_eff (effective reliability) scoring functions
// shared between artifact and codebase packages.
// Pure functions, zero dependencies, no DB, no state.
package reff

import (
	"math"
	"strings"
	"time"
)

type EDItem struct {
	ValidUntil time.Time
	Now        time.Time
	K          float64
}

type EDBudgetAlert struct {
	TotalED float64
	Budget  float64
	Excess  float64
}

// ScoreEvidence computes the effective reliability score for a single evidence item.
// FPF B.3: R_eff = max(0, base_score - Φ(CL)), with decay override for expired evidence.
func ScoreEvidence(verdict string, cl int, validUntil string, now time.Time) float64 {
	return ScoreTypedEvidence("", verdict, cl, validUntil, now)
}

// ScoreTypedEvidence computes the effective reliability score while preserving
// evidence-type-specific base scores where the model defines them explicitly.
func ScoreTypedEvidence(evidenceType string, verdict string, cl int, validUntil string, now time.Time) float64 {
	if expiry, ok := ParseValidUntil(validUntil); ok && expiry.Before(now) {
		return 0.1 // expired evidence is weak regardless of verdict (FPF B.3.4)
	}

	base := TypedVerdictToScore(evidenceType, verdict)
	penalty := CLPenalty(cl)
	score := math.Max(0, base-penalty)
	return math.Round(score*100) / 100
}

// VerdictToScore maps evidence verdict to base reliability score.
func VerdictToScore(verdict string) float64 {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "supports", "accepted", "pass":
		return 1.0
	case "weakens", "partial", "degrade":
		return 0.5
	case "refutes", "failed", "fail":
		return 0.0
	default:
		return 0.5 // unknown verdict treated as weakening
	}
}

// TypedVerdictToScore maps evidence type + verdict to the base reliability score.
// Attached evidence keeps its dedicated 0.7 base from the in-memory agent model.
func TypedVerdictToScore(evidenceType string, verdict string) float64 {
	switch strings.ToLower(strings.TrimSpace(evidenceType)) {
	case "explicit_measure":
		return 0.8
	case "partial_measure":
		return 0.5
	case "attached":
		return attachedVerdictScore(verdict)
	default:
		return VerdictToScore(verdict)
	}
}

func attachedVerdictScore(verdict string) float64 {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "refutes", "failed", "fail":
		return 0.0
	default:
		return 0.7
	}
}

// ParseValidUntil accepts RFC3339 timestamps and YYYY-MM-DD values.
// Date-only inputs are interpreted as valid through the end of that UTC day.
func ParseValidUntil(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), true
		}
	}

	dateOnly, err := time.Parse("2006-01-02", trimmed)
	if err == nil {
		year, month, day := dateOnly.Date()
		endOfDay := time.Date(year, month, day, 23, 59, 59, 0, time.UTC)
		return endOfDay, true
	}

	if monotonic := strings.Index(trimmed, " m="); monotonic != -1 {
		return ParseValidUntil(trimmed[:monotonic])
	}

	return time.Time{}, false
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

// ComputeED returns the epistemic debt for evidence that has expired.
// k defaults to 1.0 debt unit per day.
func ComputeED(validUntil time.Time, now time.Time, k float64) float64 {
	if validUntil.IsZero() {
		return 0
	}
	if now.Before(validUntil) || now.Equal(validUntil) {
		return 0
	}
	if k <= 0 {
		k = 1.0
	}

	daysExpired := now.Sub(validUntil).Hours() / 24
	return k * math.Max(0, daysExpired)
}

// AggregateED returns the total epistemic debt across all items.
func AggregateED(items []EDItem) float64 {
	total := 0.0

	for _, item := range items {
		total += ComputeED(item.ValidUntil, item.Now, item.K)
	}

	return total
}

// CheckEDBudget reports when total epistemic debt exceeds the configured budget.
func CheckEDBudget(totalED, budget float64) *EDBudgetAlert {
	if budget < 0 {
		budget = 0
	}
	if totalED <= budget {
		return nil
	}

	alert := &EDBudgetAlert{
		TotalED: totalED,
		Budget:  budget,
		Excess:  totalED - budget,
	}
	return alert
}
