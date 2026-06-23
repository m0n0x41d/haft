package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

func TestReconciliationPlanMaintenanceProposalsAreReadOnly(t *testing.T) {
	plan := artifact.DecisionReconciliationPlan{
		Groups: []artifact.DecisionReconciliationGroup{
			{
				GroupID:      "decision-reconcile-high",
				Category:     artifact.DecisionReconciliationMergeCandidate,
				DecisionRefs: []string{"dec-a", "dec-b", "dec-c"},
				Preview: artifact.DecisionReconciliationPreview{
					DownstreamImpact: &artifact.DecisionReconciliationDownstream{
						InternalEdges: 2,
						ExternalEdges: 2,
					},
				},
			},
			{
				GroupID:          "decision-reconcile-fallback",
				Category:         artifact.DecisionReconciliationKeep,
				DecisionRefs:     []string{"dec-fallback"},
				ScopeRepairHints: []string{"replace whole-file fallback with explicit governance target"},
			},
		},
	}

	proposals := reconciliationPlanMaintenanceProposals(plan)

	if len(proposals) != 2 {
		t.Fatalf("proposals = %#v", proposals)
	}
	for _, proposal := range proposals {
		if proposal.AuthorityBoundary != "read_only_reconciliation_proposal_not_binding_authority" {
			t.Fatalf("proposal authority = %q", proposal.AuthorityBoundary)
		}
		assertMaintenanceReconciliationProposalIsProposalOnly(t, proposal)
		if proposal.SuggestedCommand != maintenanceReconciliationInspectCommand {
			t.Fatalf("suggested command = %q", proposal.SuggestedCommand)
		}
	}
}

func TestGoverningSetMaintenanceProposalsAreReadOnly(t *testing.T) {
	report := artifact.CurrentGoverningSetReport{
		Sets: []artifact.CurrentGoverningSet{{
			SetID:                    "governing-set-fallback",
			TargetRef:                "unscoped:dec-fallback",
			WholeFileFallbackTargets: []string{"whole_file_fallback:internal/shared.go"},
			ScopeRepairHints:         []string{"replace whole-file fallback with explicit governance target"},
			CurrentDecisionRefs:      []string{"dec-fallback"},
			TerminalHistoryRefs:      []string{"dec-old"},
		}},
	}

	proposals := governingSetMaintenanceProposals(report)

	if len(proposals) != 1 {
		t.Fatalf("proposals = %#v", proposals)
	}
	if proposals[0].Kind != "fallback_governing_scope_review" {
		t.Fatalf("proposal kind = %q", proposals[0].Kind)
	}
	assertMaintenanceReconciliationProposalIsProposalOnly(t, proposals[0])
	if len(proposals[0].FallbackTargets) != 1 {
		t.Fatalf("fallback targets = %#v", proposals[0].FallbackTargets)
	}
	if proposals[0].SuggestedCommand != `haft decision governing-set --target-ref "unscoped:dec-fallback" --json --limit 5` {
		t.Fatalf("suggested command = %q", proposals[0].SuggestedCommand)
	}
}

func TestMaintenanceReconciliationReviewFromProposalsPreservesReadOnlyBoundary(t *testing.T) {
	review := maintenanceReconciliationReviewFromProposals([]overseer.MaintenanceReconciliationProposal{
		{
			ID:                "reconcile-a",
			Kind:              "fallback_scope_repair_review",
			GroupID:           "decision-reconcile-a",
			Reason:            "fallback targets need review",
			DecisionRefs:      []string{"dec-a"},
			Fanout:            7,
			SuggestedCommand:  maintenanceReconciliationInspectCommand,
			AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
		},
	})

	if review == nil || review.ProposalCount != 1 {
		t.Fatalf("review = %#v", review)
	}
	if review.Proposals[0].AuthorityBoundary != "read_only_reconciliation_proposal_not_binding_authority" {
		t.Fatalf("proposal authority = %q", review.Proposals[0].AuthorityBoundary)
	}
	if review.Proposals[0].SuggestedCommand != maintenanceReconciliationInspectCommand {
		t.Fatalf("suggested command = %q", review.Proposals[0].SuggestedCommand)
	}
	if strings.Contains(strings.ToLower(review.Proposals[0].SuggestedCommand), "apply") {
		t.Fatalf("judgment reconciliation command must be inspect-only: %#v", review.Proposals[0])
	}
}

func assertMaintenanceReconciliationProposalIsProposalOnly(t *testing.T, proposal overseer.MaintenanceReconciliationProposal) {
	t.Helper()

	text := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		proposal.SuggestedCommand,
		proposal.AuthorityBoundary,
		proposal.Reason,
		proposal.Kind,
	}, " ")))
	for _, forbidden := range []string{
		" decision reconcile apply ",
		"operator_approved_reconciliation_selection",
		"merge_through_successor",
		"retire_without_successor",
		"claim_lifecycle_update",
		"supersede",
		"retire",
		"superseded",
		"deprecated",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("proposal contains binding mutation cue %q: %s", forbidden, text)
		}
	}
}
