package present

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestMaintenanceJudgmentReviewResponseIncludesReadOnlyReconciliationSummary(t *testing.T) {
	review := &artifact.MaintenanceJudgmentReview{
		JudgmentTasks:      1,
		OmittedNonJudgment: 0,
		AuthorityBoundary: artifact.MaintenanceReviewAuthority{
			Mutation:     "not_mutation",
			Approval:     "not_approval",
			Evidence:     "not_evidence",
			AgentRole:    "first_pass_judgment_review",
			ApplySurface: "operator_approval_required",
		},
		Counts: artifact.MaintenanceJudgmentCounts{
			ByRecommendation: map[string]int{},
			ByConfidence:     map[string]int{},
			BySource:         map[string]int{},
		},
		Reconciliation: artifact.BuildMaintenanceReconciliationReview([]artifact.MaintenanceReconciliationReviewProposal{
			{
				ID:               "reconcile-a",
				Kind:             "fallback_scope_repair_review",
				GroupID:          "decision-reconcile-a",
				Reason:           "fallback targets need review",
				Fanout:           9,
				SuggestedCommand: "haft decision reconcile --json",
			},
		}),
	}

	response := MaintenanceJudgmentReviewResponse(review, "")
	for _, want := range []string{
		"Reconciliation proposals (1 read-only)",
		"read_only_reconciliation_proposal_not_binding_authority_not_mutation_not_evidence_not_approval_not_gate_decision_not_claim_truth_not_global_truth_not_publication",
		"fallback_scope_repair_review=1",
		"haft decision reconcile --json",
		"reconcile-a",
	} {
		if !strings.Contains(response, want) {
			t.Fatalf("response missing %q:\n%s", want, response)
		}
	}
	if strings.Contains(strings.ToLower(response), "operator_approved_reconciliation_selection") ||
		strings.Contains(strings.ToLower(response), "decision reconcile apply") {
		t.Fatalf("response leaked binding reconciliation cue:\n%s", response)
	}
}
