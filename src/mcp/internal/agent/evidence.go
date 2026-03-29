package agent

import (
	"math"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// L1: Evidence tracking — auto-captured from agent behavior.
//
// Coordinator observes what the agent does after decide (tests, lints,
// file reads, measure) and records each as an evidence item. R_eff is
// computed as min(scores) — weakest link, never average.
// ---------------------------------------------------------------------------

// EvidenceType classifies how evidence was obtained.
type EvidenceType string

const (
	EvidenceTestPass   EvidenceType = "test_pass"        // bash(test) passed
	EvidenceTestFix    EvidenceType = "test_fix_pass"    // test failed then passed after fix
	EvidenceLintPass   EvidenceType = "lint_pass"        // bash(lint/vet) clean
	EvidenceFileReview EvidenceType = "file_review"      // read affected file, no issues raised
	EvidenceMeasure    EvidenceType = "explicit_measure" // quint_decision(measure, accepted)
	EvidencePartial    EvidenceType = "partial_measure"  // quint_decision(measure, partial)
	EvidenceExternal   EvidenceType = "external_ref"     // fetch used for reference
	EvidenceNoVerify   EvidenceType = "no_verification"  // no tests, no lint, just "done"
)

// EvidenceItem is one piece of evidence auto-captured from agent actions.
type EvidenceItem struct {
	Type       EvidenceType `json:"type"`
	Detail     string       `json:"detail,omitempty"` // e.g., "go test ./internal/tools/..."
	BaseScore  float64      `json:"base_score"`       // 0.0-1.0
	CL         int          `json:"cl"`               // congruence level 0-3
	CapturedAt time.Time    `json:"captured_at"`
}

// EvidenceChain collects evidence for a cycle's active decision.
type EvidenceChain struct {
	Items    []EvidenceItem `json:"items"`
	DecRef   string         `json:"decision_ref,omitempty"`
	CycleRef string         `json:"cycle_ref,omitempty"`
}

// baseScores maps evidence types to their default scores.
var baseScores = map[EvidenceType]float64{
	EvidenceTestPass:   0.9,
	EvidenceTestFix:    0.8,
	EvidenceLintPass:   0.8,
	EvidenceFileReview: 0.6,
	EvidenceMeasure:    0.7,
	EvidencePartial:    0.4,
	EvidenceExternal:   0.5,
	EvidenceNoVerify:   0.2,
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

// ComputeREff calculates R_eff from an evidence chain.
// R_eff = min(effective_score for each item)
// effective_score = max(0, base_score - cl_penalty)
// If chain is empty: R_eff = 0.0 (no evidence = no trust)
func ComputeREff(chain *EvidenceChain) float64 {
	if chain == nil || len(chain.Items) == 0 {
		return 0.0
	}

	minScore := 1.0
	for _, item := range chain.Items {
		penalty := clPenalties[item.CL]
		effective := math.Max(0, item.BaseScore-penalty)
		if effective < minScore {
			minScore = effective
		}
	}
	return math.Round(minScore*100) / 100 // round to 2 decimal places
}

// DetectEvidenceFromTool determines if a tool call constitutes evidence.
// Returns evidence item if yes, nil if not evidence.
func DetectEvidenceFromTool(toolName, args, output string, isError bool) *EvidenceItem {
	switch toolName {
	case "bash":
		lowerArgs := strings.ToLower(args)
		// Test commands
		isTest := strings.Contains(lowerArgs, "test") ||
			strings.Contains(lowerArgs, "pytest") ||
			strings.Contains(lowerArgs, "jest") ||
			strings.Contains(lowerArgs, "cargo test")
		if isTest {
			if isError {
				return nil // test failed — not positive evidence (yet)
			}
			item := NewEvidenceItem(EvidenceTestPass, truncateDetail(args, 100), 3)
			return &item
		}

		// Lint/vet commands
		isLint := strings.Contains(lowerArgs, "lint") ||
			strings.Contains(lowerArgs, "vet") ||
			strings.Contains(lowerArgs, "eslint") ||
			strings.Contains(lowerArgs, "golangci")
		if isLint && !isError {
			item := NewEvidenceItem(EvidenceLintPass, truncateDetail(args, 100), 3)
			return &item
		}

	case "read":
		// Reading affected files = basic review evidence
		item := NewEvidenceItem(EvidenceFileReview, truncateDetail(args, 100), 3)
		return &item

	case "fetch":
		// External reference
		item := NewEvidenceItem(EvidenceExternal, truncateDetail(args, 100), 1)
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
