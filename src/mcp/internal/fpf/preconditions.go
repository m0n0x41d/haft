package fpf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type PreconditionError struct {
	Tool       string
	Condition  string
	Suggestion string
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("Precondition failed for %s: %s. Suggestion: %s", e.Tool, e.Condition, e.Suggestion)
}

func (t *Tools) CheckPreconditions(toolName string, args map[string]string) error {
	switch toolName {
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

	l0Path := filepath.Join(t.GetFPFDir(), "knowledge", "L0", hypoID+".md")
	if _, err := os.Stat(l0Path); os.IsNotExist(err) {
		return &PreconditionError{
			Tool:       "quint_verify",
			Condition:  fmt.Sprintf("hypothesis '%s' not found in L0", hypoID),
			Suggestion: "Run /q1-hypothesize first to create a hypothesis, or check the hypothesis ID",
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

	l0Path := filepath.Join(t.GetFPFDir(), "knowledge", "L0", hypoID+".md")
	if _, err := os.Stat(l0Path); err == nil {
		return &PreconditionError{
			Tool:       "quint_test",
			Condition:  fmt.Sprintf("hypothesis '%s' is still in L0", hypoID),
			Suggestion: "Run /q2-verify first to promote the hypothesis to L1 before testing",
		}
	}

	l1Path := filepath.Join(t.GetFPFDir(), "knowledge", "L1", hypoID+".md")
	l2Path := filepath.Join(t.GetFPFDir(), "knowledge", "L2", hypoID+".md")
	l1Exists := false
	l2Exists := false

	if _, err := os.Stat(l1Path); err == nil {
		l1Exists = true
	}
	if _, err := os.Stat(l2Path); err == nil {
		l2Exists = true
	}

	if !l1Exists && !l2Exists {
		if t.DB != nil {
			ctx := context.Background()
			holon, err := t.DB.GetHolon(ctx, hypoID)
			if err != nil || (holon.Layer != "L1" && holon.Layer != "L2") {
				return &PreconditionError{
					Tool:       "quint_test",
					Condition:  fmt.Sprintf("hypothesis '%s' not found in L1 or L2", hypoID),
					Suggestion: "Ensure hypothesis exists and has been verified (L0 -> L1) first. L2 hypotheses can also be tested to refresh evidence.",
				}
			}
		} else {
			return &PreconditionError{
				Tool:       "quint_test",
				Condition:  fmt.Sprintf("hypothesis '%s' not found in L1 or L2", hypoID),
				Suggestion: "Ensure hypothesis exists and has been verified (L0 -> L1) first. L2 hypotheses can also be tested to refresh evidence.",
			}
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
	}

	return nil
}

func (t *Tools) checkCalculateRPreconditions(args map[string]string) error {
	if t.DB == nil {
		return &PreconditionError{
			Tool:       "quint_calculate_r",
			Condition:  "database not initialized",
			Suggestion: "Run /q0-init to initialize the project first",
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
			Suggestion: "Run /q0-init to initialize the project first",
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
