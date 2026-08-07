package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

const maintenanceHighFanoutThreshold = 5
const maintenanceReconciliationInspectCommand = "haft decision reconcile --json --limit 5"

func buildMaintenanceReconciliationProposals(
	ctx context.Context,
	store *artifact.Store,
) []overseer.MaintenanceReconciliationProposal {
	proposals := []overseer.MaintenanceReconciliationProposal{}
	if plan, err := artifact.BuildDecisionReconciliationPlan(ctx, store); err == nil {
		proposals = append(proposals, reconciliationPlanMaintenanceProposals(plan)...)
	}
	if governing, err := artifact.BuildCurrentGoverningSetReport(ctx, store); err == nil {
		proposals = append(proposals, governingSetMaintenanceProposals(governing)...)
	}
	sort.SliceStable(proposals, func(i, j int) bool {
		if proposals[i].Kind == proposals[j].Kind {
			return proposals[i].ID < proposals[j].ID
		}
		return proposals[i].Kind < proposals[j].Kind
	})
	return proposals
}

func buildMaintenanceJudgmentReconciliationReview(
	ctx context.Context,
	store *artifact.Store,
) *artifact.MaintenanceReconciliationReview {
	return maintenanceReconciliationReviewFromProposals(buildMaintenanceReconciliationProposals(ctx, store))
}

func maintenanceReconciliationReviewFromProposals(
	proposals []overseer.MaintenanceReconciliationProposal,
) *artifact.MaintenanceReconciliationReview {
	items := make([]artifact.MaintenanceReconciliationReviewProposal, 0, len(proposals))
	for _, proposal := range proposals {
		items = append(items, artifact.MaintenanceReconciliationReviewProposal{
			ID:                proposal.ID,
			Kind:              proposal.Kind,
			GroupID:           proposal.GroupID,
			Category:          proposal.Category,
			Reason:            proposal.Reason,
			DecisionRefs:      proposal.DecisionRefs,
			Fanout:            proposal.Fanout,
			FallbackTargets:   proposal.FallbackTargets,
			ScopeRepairHints:  proposal.ScopeRepairHints,
			SuggestedCommand:  proposal.SuggestedCommand,
			AuthorityBoundary: proposal.AuthorityBoundary,
		})
	}
	return artifact.BuildMaintenanceReconciliationReview(items)
}

func reconciliationPlanMaintenanceProposals(
	plan artifact.DecisionReconciliationPlan,
) []overseer.MaintenanceReconciliationProposal {
	proposals := []overseer.MaintenanceReconciliationProposal{}
	for _, group := range plan.Groups {
		fanout := decisionReconciliationGroupFanout(group)
		if fanout >= maintenanceHighFanoutThreshold {
			proposals = append(proposals, overseer.MaintenanceReconciliationProposal{
				ID:                "reconcile-high-fanout-" + group.GroupID,
				Kind:              "high_fanout_reconciliation_review",
				GroupID:           group.GroupID,
				Category:          group.Category,
				Reason:            fmt.Sprintf("group fanout %d meets threshold %d", fanout, maintenanceHighFanoutThreshold),
				DecisionRefs:      group.DecisionRefs,
				Fanout:            fanout,
				ScopeRepairHints:  group.ScopeRepairHints,
				SuggestedCommand:  maintenanceReconciliationInspectCommand,
				AuthorityBoundary: artifact.MaintenanceReconciliationProposalAuthorityBoundary,
			})
		}
		if len(group.ScopeRepairHints) > 0 {
			proposals = append(proposals, overseer.MaintenanceReconciliationProposal{
				ID:                "reconcile-scope-repair-" + group.GroupID,
				Kind:              "fallback_scope_repair_review",
				GroupID:           group.GroupID,
				Category:          group.Category,
				Reason:            "group contains scope repair hints; fallback targets need operator-approved enrichment before stronger use",
				DecisionRefs:      group.DecisionRefs,
				Fanout:            fanout,
				ScopeRepairHints:  group.ScopeRepairHints,
				SuggestedCommand:  maintenanceReconciliationInspectCommand,
				AuthorityBoundary: artifact.MaintenanceReconciliationProposalAuthorityBoundary,
			})
		}
	}
	return proposals
}

func governingSetMaintenanceProposals(
	report artifact.CurrentGoverningSetReport,
) []overseer.MaintenanceReconciliationProposal {
	proposals := []overseer.MaintenanceReconciliationProposal{}
	for _, set := range report.Sets {
		if len(set.WholeFileFallbackTargets) == 0 {
			continue
		}
		proposals = append(proposals, overseer.MaintenanceReconciliationProposal{
			ID:                "governing-fallback-" + set.SetID,
			Kind:              "fallback_governing_scope_review",
			GroupID:           set.SetID,
			Reason:            "current governing set uses whole-file fallback targets",
			DecisionRefs:      set.CurrentDecisionRefs,
			Fanout:            len(set.CurrentDecisionRefs) + len(set.TerminalHistoryRefs),
			FallbackTargets:   set.WholeFileFallbackTargets,
			ScopeRepairHints:  set.ScopeRepairHints,
			SuggestedCommand:  fmt.Sprintf("haft decision governing-set --target-ref %q --json --limit 5", set.TargetRef),
			AuthorityBoundary: artifact.MaintenanceReconciliationProposalAuthorityBoundary,
		})
	}
	return proposals
}

func decisionReconciliationGroupFanout(group artifact.DecisionReconciliationGroup) int {
	impact := group.Preview.DownstreamImpact
	if impact == nil {
		return len(group.DecisionRefs)
	}
	return impact.InternalEdges + impact.ExternalEdges + len(group.DecisionRefs)
}
