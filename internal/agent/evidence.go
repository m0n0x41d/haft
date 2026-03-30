package agent

import (
	"math"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// L1: Evidence tracking — two categories:
//
// 1. Observations: auto-detected from agent tool usage (tests, lint, reads).
//    Shown in overseer/status bar. NOT counted in R_eff.
//    Purpose: UX feedback — user sees what the agent verified.
//
// 2. Evidence: explicitly recorded via haft_decision(measure) or haft_decision(evidence).
//    Counted in R_eff. This is the honest signal — only explicit measurement
//    produces trust. FPF: design-time claims ≠ run-time evidence.
//
// R_eff = min(evidence scores) — weakest link, never average.
// ---------------------------------------------------------------------------

// EvidenceType classifies how evidence was obtained.
type EvidenceType string

const (
	// Explicit evidence — counts in R_eff
	EvidenceMeasure  EvidenceType = "explicit_measure" // haft_decision(measure, verdict=accepted)
	EvidencePartial  EvidenceType = "partial_measure"  // haft_decision(measure, verdict=partial)
	EvidenceAttached EvidenceType = "attached"          // haft_decision(evidence, ...)

	// Observations — shown in status, NOT in R_eff
	ObservationTestPass   EvidenceType = "obs_test_pass"   // bash(test) passed
	ObservationLintPass   EvidenceType = "obs_lint_pass"   // bash(lint/vet) clean
	ObservationFileReview EvidenceType = "obs_file_review" // read affected file
	ObservationExternal   EvidenceType = "obs_external"    // fetch used for reference
	ObservationNoVerify   EvidenceType = "obs_no_verify"   // no tests, no lint
)

// IsExplicitEvidence returns true if this type counts in R_eff.
func (t EvidenceType) IsExplicitEvidence() bool {
	return t == EvidenceMeasure || t == EvidencePartial || t == EvidenceAttached
}

// EvidenceItem is one piece of evidence or observation.
type EvidenceItem struct {
	Type       EvidenceType `json:"type"`
	Detail     string       `json:"detail,omitempty"`
	BaseScore  float64      `json:"base_score"`
	CL         int          `json:"cl"` // congruence level 0-3
	CapturedAt time.Time    `json:"captured_at"`
}

// EvidenceChain collects evidence and observations for a cycle's active decision.
type EvidenceChain struct {
	Items    []EvidenceItem `json:"items"`
	DecRef   string         `json:"decision_ref,omitempty"`
	CycleRef string         `json:"cycle_ref,omitempty"`
}

// baseScores maps evidence types to their default scores.
var baseScores = map[EvidenceType]float64{
	EvidenceMeasure:  0.8, // explicit measurement — high trust
	EvidencePartial:  0.5, // partial measurement — moderate trust
	EvidenceAttached: 0.7, // attached evidence — depends on CL

	// Observations — scores only matter for display, not R_eff
	ObservationTestPass:   0.9,
	ObservationLintPass:   0.8,
	ObservationFileReview: 0.6,
	ObservationExternal:   0.5,
	ObservationNoVerify:   0.2,
}

// clPenalties maps CL levels to their R_eff penalties.
var clPenalties = map[int]float64{
	3: 0.0, // same project, internal test
	2: 0.1, // similar project
	1: 0.4, // external docs
	0: 0.9, // different domain
}

// NewEvidenceItem creates an evidence item with default scoring.
func NewEvidenceItem(typ EvidenceType, detail string, cl int) EvidenceItem {
	score, ok := baseScores[typ]
	if !ok {
		score = 0.2
	}
	return EvidenceItem{
		Type:       typ,
		Detail:     detail,
		BaseScore:  score,
		CL:         cl,
		CapturedAt: time.Now().UTC(),
	}
}

// ComputeREff calculates R_eff from EXPLICIT evidence only.
// Observations are ignored. R_eff = min(effective_score for each explicit item).
// If no explicit evidence exists: R_eff = 0.0 (no evidence = no trust).
func ComputeREff(chain *EvidenceChain) float64 {
	if chain == nil || len(chain.Items) == 0 {
		return 0.0
	}

	minScore := 1.0
	hasExplicit := false

	for _, item := range chain.Items {
		if !item.Type.IsExplicitEvidence() {
			continue // skip observations
		}
		hasExplicit = true
		penalty := clPenalties[item.CL]
		effective := math.Max(0, item.BaseScore-penalty)
		if effective < minScore {
			minScore = effective
		}
	}

	if !hasExplicit {
		return 0.0 // observations alone don't produce trust
	}
	return math.Round(minScore*100) / 100
}

// ObservationCount returns how many observations (non-evidence) are in the chain.
func ObservationCount(chain *EvidenceChain) int {
	if chain == nil {
		return 0
	}
	count := 0
	for _, item := range chain.Items {
		if !item.Type.IsExplicitEvidence() {
			count++
		}
	}
	return count
}

// DetectObservationFromTool determines if a tool call is an observation.
// Observations are shown in status/overseer but NOT counted in R_eff.
// Returns observation item if yes, nil if not relevant.
func DetectObservationFromTool(toolName, args, output string, isError bool) *EvidenceItem {
	switch toolName {
	case "bash":
		lowerArgs := strings.ToLower(args)
		isTest := strings.Contains(lowerArgs, "test") ||
			strings.Contains(lowerArgs, "pytest") ||
			strings.Contains(lowerArgs, "jest") ||
			strings.Contains(lowerArgs, "cargo test")
		if isTest && !isError {
			item := NewEvidenceItem(ObservationTestPass, truncateDetail(args, 100), 3)
			return &item
		}
		isLint := strings.Contains(lowerArgs, "lint") ||
			strings.Contains(lowerArgs, "vet") ||
			strings.Contains(lowerArgs, "eslint") ||
			strings.Contains(lowerArgs, "golangci")
		if isLint && !isError {
			item := NewEvidenceItem(ObservationLintPass, truncateDetail(args, 100), 3)
			return &item
		}

	case "read":
		item := NewEvidenceItem(ObservationFileReview, truncateDetail(args, 100), 3)
		return &item

	case "fetch":
		item := NewEvidenceItem(ObservationExternal, truncateDetail(args, 100), 1)
		return &item
	}

	return nil
}

func truncateDetail(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
