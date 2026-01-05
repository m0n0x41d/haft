package fpf

import (
	"context"
	"fmt"
)

type PreconditionError struct {
	Tool       string
	Condition  string
	Suggestion string
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("Precondition failed for %s: %s. Suggestion: %s", e.Tool, e.Condition, e.Suggestion)
}

// CheckPreconditions is the unified entry point for all precondition checks.
// Validates tool-specific semantic requirements (holon existence, valid args).
//
// DESIGN: No phase gates. Semantic preconditions are sufficient.
// See roles.go for the design decision on removing phase gates.
func (t *Tools) CheckPreconditions(toolName string, args map[string]string) error {
	switch toolName {
	case "quint_internalize":
		return nil // No preconditions - can always be called
	case "quint_search":
		return t.checkSearchPreconditions(args)
	case "quint_propose":
		return t.checkProposePreconditions(args)
	case "quint_verify":
		return t.checkVerifyPreconditions(args)
	case "quint_test":
		return t.checkTestPreconditions(args)
	case "quint_audit":
		return t.checkAuditPreconditions(args)
	case "quint_decide":
		return t.checkDecidePreconditions(args)
	case "quint_calculate_r":
		return t.checkCalculateRPreconditions(args)
	case "quint_audit_tree":
		return t.checkAuditTreePreconditions(args)
	default:
		return nil
	}
}

func (t *Tools) getHolonLayer(holonID string) (string, error) {
	if t.DB == nil {
		return "", fmt.Errorf("database not initialized")
	}
	holon, err := t.DB.GetHolon(context.Background(), holonID)
	if err != nil {
		return "", fmt.Errorf("holon %s not found", holonID)
	}
	return holon.Layer, nil
}

func (t *Tools) checkSearchPreconditions(args map[string]string) error {
	if args["query"] == "" {
		return &PreconditionError{
			Tool:       "quint_search",
			Condition:  "query is required",
			Suggestion: "Provide search terms",
		}
	}
	return nil
}

func (t *Tools) checkProposePreconditions(args map[string]string) error {
	if args["title"] == "" {
		return &PreconditionError{
			Tool:       "quint_propose",
			Condition:  "title is required",
			Suggestion: "Provide a descriptive title for the hypothesis",
		}
	}
	if args["content"] == "" {
		return &PreconditionError{
			Tool:       "quint_propose",
			Condition:  "content is required",
			Suggestion: "Describe the hypothesis in detail",
		}
	}
	if args["kind"] != "system" && args["kind"] != "episteme" {
		return &PreconditionError{
			Tool:       "quint_propose",
			Condition:  "kind must be 'system' or 'episteme'",
			Suggestion: "Use 'system' for technical hypotheses, 'episteme' for knowledge claims",
		}
	}
	return nil
}

func (t *Tools) checkVerifyPreconditions(args map[string]string) error {
	hypoID := args["hypothesis_id"]
	if hypoID == "" {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  "hypothesis_id is required",
			Suggestion: "Specify which hypothesis to verify",
		}
	}

	if t.DB == nil {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  "database not initialized",
			Suggestion: "Run /q-internalize first",
		}
	}

	holon, err := t.DB.GetHolon(context.Background(), hypoID)
	if err != nil {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  fmt.Sprintf("hypothesis '%s' not found", hypoID),
			Suggestion: "Run /q1-hypothesize first to create a hypothesis",
		}
	}

	if holon.Layer != "L0" {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  fmt.Sprintf("hypothesis '%s' is in %s, not L0", hypoID, holon.Layer),
			Suggestion: "quint_verify only works on L0 hypotheses",
		}
	}

	verdict := args["verdict"]
	if verdict != "PASS" && verdict != "FAIL" && verdict != "REFINE" {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  "verdict must be PASS, FAIL, or REFINE",
			Suggestion: "Specify the verification outcome",
		}
	}

	return nil
}

func (t *Tools) checkTestPreconditions(args map[string]string) error {
	hypoID := args["hypothesis_id"]
	if hypoID == "" {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  "hypothesis_id is required",
			Suggestion: "Specify which hypothesis to test",
		}
	}

	if t.DB == nil {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  "database not initialized",
			Suggestion: "Run /q-internalize first",
		}
	}

	holon, err := t.DB.GetHolon(context.Background(), hypoID)
	if err != nil {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  fmt.Sprintf("hypothesis '%s' not found", hypoID),
			Suggestion: "Create and verify a hypothesis first",
		}
	}

	if holon.Layer == "L0" {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  fmt.Sprintf("hypothesis '%s' is still in L0", hypoID),
			Suggestion: "Run /q2-verify first to promote to L1 before testing",
		}
	}

	if holon.Layer != "L1" && holon.Layer != "L2" {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  fmt.Sprintf("hypothesis '%s' is in %s, expected L1 or L2", hypoID, holon.Layer),
			Suggestion: "quint_test works on L1 (new tests) or L2 (refresh evidence)",
		}
	}

	verdict := args["verdict"]
	if verdict != "PASS" && verdict != "FAIL" && verdict != "REFINE" {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  "verdict must be PASS, FAIL, or REFINE",
			Suggestion: "Specify the test outcome",
		}
	}

	return nil
}

func (t *Tools) checkAuditPreconditions(args map[string]string) error {
	hypoID := args["hypothesis_id"]
	if hypoID == "" {
		return &PreconditionError{
			Tool:       "quint_audit",
			Condition:  "hypothesis_id is required",
			Suggestion: "Specify which hypothesis to audit",
		}
	}

	if t.DB != nil {
		ctx := context.Background()
		holon, err := t.DB.GetHolon(ctx, hypoID)
		if err != nil {
			return &PreconditionError{
				Tool:       "quint_audit",
				Condition:  fmt.Sprintf("hypothesis '%s' not found", hypoID),
				Suggestion: "Ensure hypothesis exists in the database",
			}
		}
		if holon.Layer != "L2" {
			return &PreconditionError{
				Tool:       "quint_audit",
				Condition:  fmt.Sprintf("hypothesis '%s' is in %s, not L2", hypoID, holon.Layer),
				Suggestion: "Only L2 (validated) hypotheses can be audited for final decision",
			}
		}
	}

	return nil
}

func (t *Tools) checkDecidePreconditions(args map[string]string) error {
	winnerID := args["winner_id"]
	if winnerID == "" {
		return &PreconditionError{
			Tool:       "quint_decide",
			Condition:  "winner_id is required",
			Suggestion: "Specify the winning hypothesis ID",
		}
	}

	if args["title"] == "" {
		return &PreconditionError{
			Tool:       "quint_decide",
			Condition:  "title is required",
			Suggestion: "Provide a title for the decision record",
		}
	}

	if t.DB != nil {
		ctx := context.Background()
		counts, _ := t.DB.CountHolonsByLayer(ctx, "default")

		l2Count := int64(0)
		for _, c := range counts {
			if c.Layer == "L2" {
				l2Count = c.Count
				break
			}
		}

		if l2Count == 0 {
			return &PreconditionError{
				Tool:       "quint_decide",
				Condition:  "no L2 hypotheses found",
				Suggestion: "Complete the ADI cycle: propose (L0) -> verify (L1) -> test (L2) before deciding",
			}
		}

		winner, err := t.DB.GetHolon(ctx, winnerID)
		if err != nil {
			return &PreconditionError{
				Tool:       "quint_decide",
				Condition:  fmt.Sprintf("winner '%s' not found", winnerID),
				Suggestion: "Specify an existing holon ID as winner",
			}
		}
		if winner.Layer != "L2" {
			return &PreconditionError{
				Tool:       "quint_decide",
				Condition:  fmt.Sprintf("winner '%s' is in %s, not L2", winnerID, winner.Layer),
				Suggestion: "Complete the ADI cycle: verify (L1) -> test (L2) before deciding",
			}
		}
	}

	return nil
}

func (t *Tools) checkCalculateRPreconditions(args map[string]string) error {
	if t.DB == nil {
		return &PreconditionError{
			Tool:       "quint_calculate_r",
			Condition:  "database not initialized",
			Suggestion: "Run /q-internalize to initialize the project first",
		}
	}

	holonID := args["holon_id"]
	if holonID == "" {
		return &PreconditionError{
			Tool:       "quint_calculate_r",
			Condition:  "holon_id is required",
			Suggestion: "Specify which holon to calculate R for",
		}
	}

	ctx := context.Background()
	_, err := t.DB.GetHolon(ctx, holonID)
	if err != nil {
		return &PreconditionError{
			Tool:       "quint_calculate_r",
			Condition:  fmt.Sprintf("holon '%s' not found", holonID),
			Suggestion: "Ensure the holon exists in the database",
		}
	}

	return nil
}

func (t *Tools) checkAuditTreePreconditions(args map[string]string) error {
	if t.DB == nil {
		return &PreconditionError{
			Tool:       "quint_audit_tree",
			Condition:  "database not initialized",
			Suggestion: "Run /q-internalize to initialize the project first",
		}
	}

	holonID := args["holon_id"]
	if holonID == "" {
		return &PreconditionError{
			Tool:       "quint_audit_tree",
			Condition:  "holon_id is required",
			Suggestion: "Specify which holon to visualize the audit tree for",
		}
	}

	return nil
}
