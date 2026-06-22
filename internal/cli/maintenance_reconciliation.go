package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

const maintenanceHighFanoutThreshold = 5

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
				SuggestedCommand:  "haft decision reconcile --json",
				AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
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
				SuggestedCommand:  "haft decision reconcile --json",
				AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
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
			SuggestedCommand:  fmt.Sprintf("haft decision governing-set --target-ref %q --json", set.TargetRef),
			AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
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
