package cli

import "testing"

// TestParseDimensions_PreservesRoleAndValidUntil locks the fix for the live MCP
// characterize path. The server dispatches haft_problem ->
// handleQuintProblemWithCreatedRef -> parseDimensions (NOT
// tools.HaftProblemTool.Execute), and parseDimensions
// previously mapped name/scale_type/unit/polarity/how_to_measure but DROPPED role
// and valid_until — so every characterized dimension persisted as role="target"
// with no valid_until, which made /h-compare ignore constraint/observation roles.
func TestParseDimensions_PreservesRoleAndValidUntil(t *testing.T) {
	raw := []any{
		map[string]any{
			"name":        "must_pass",
			"role":        "constraint",
			"polarity":    "true_better",
			"scale_type":  "binary",
			"valid_until": "2026-09-03",
		},
		map[string]any{
			"name":     "watch_only",
			"role":     "observation",
			"polarity": "lower_better",
		},
	}

	dims := parseDimensions(raw)
	if len(dims) != 2 {
		t.Fatalf("parsed %d dimensions, want 2", len(dims))
	}
	if got := dims[0].Role; got != "constraint" {
		t.Errorf("constraint role dropped by parseDimensions: got %q, want constraint", got)
	}
	if got := dims[0].ValidUntil; got != "2026-09-03" {
		t.Errorf("valid_until dropped by parseDimensions: got %q, want 2026-09-03", got)
	}
	if got := dims[1].Role; got != "observation" {
		t.Errorf("observation role dropped by parseDimensions: got %q, want observation", got)
	}
}
