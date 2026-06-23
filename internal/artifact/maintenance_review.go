package artifact

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	JudgmentRecommendationReviewMaterialDrift = "review_material_drift"
	JudgmentRecommendationReviewUnprovenDrift = "review_unproven_drift"
	JudgmentRecommendationVerifyClaim         = "verify_claim"
	JudgmentRecommendationRefreshEvidence     = "refresh_evidence"
	JudgmentRecommendationReopenOrSupersede   = "reopen_or_supersede"
	JudgmentRecommendationTriageStaleDecision = "triage_stale_decision"
)

const (
	JudgmentConfidenceHigh   = "high"
	JudgmentConfidenceMedium = "medium"
	JudgmentConfidenceLow    = "low"
)

type MaintenanceJudgmentReview struct {
	GeneratedAt           string                           `json:"generated_at"`
	SourcePlanGeneratedAt string                           `json:"source_plan_generated_at,omitempty"`
	View                  string                           `json:"view,omitempty"`
	TotalTasks            int                              `json:"total_tasks"`
	JudgmentTasks         int                              `json:"judgment_tasks"`
	OmittedNonJudgment    int                              `json:"omitted_non_judgment"`
	OmittedJudgmentTasks  int                              `json:"omitted_judgment_tasks,omitempty"`
	FullAuditCommand      string                           `json:"full_audit_command,omitempty"`
	AuthorityBoundary     MaintenanceReviewAuthority       `json:"authority_boundary"`
	Counts                MaintenanceJudgmentCounts        `json:"counts"`
	Groups                []MaintenanceJudgmentGroup       `json:"groups"`
	Reconciliation        *MaintenanceReconciliationReview `json:"reconciliation,omitempty"`
}

type MaintenanceReviewAuthority struct {
	Mutation     string `json:"mutation"`
	Approval     string `json:"approval"`
	Evidence     string `json:"evidence"`
	AgentRole    string `json:"agent_role"`
	ApplySurface string `json:"apply_surface"`
}

type MaintenanceJudgmentCounts struct {
	ByRecommendation map[string]int `json:"by_recommendation"`
	ByConfidence     map[string]int `json:"by_confidence"`
	BySource         map[string]int `json:"by_source"`
}

type MaintenanceJudgmentGroup struct {
	Recommendation  string                          `json:"recommendation"`
	Confidence      string                          `json:"confidence"`
	Source          string                          `json:"source"`
	Category        string                          `json:"category"`
	TaskCount       int                             `json:"task_count"`
	OmittedTasks    int                             `json:"omitted_tasks,omitempty"`
	EvidenceNeed    string                          `json:"evidence_need"`
	SuggestedAction string                          `json:"suggested_action"`
	DrillDown       []string                        `json:"drill_down,omitempty"`
	Tasks           []MaintenanceJudgmentTaskReview `json:"tasks,omitempty"`
}

type MaintenanceJudgmentTaskReview struct {
	DecisionRef       string   `json:"decision_ref"`
	DecisionTitle     string   `json:"decision_title"`
	Source            string   `json:"source"`
	Category          string   `json:"category"`
	ClaimID           string   `json:"claim_id,omitempty"`
	Observable        string   `json:"observable,omitempty"`
	Threshold         string   `json:"threshold,omitempty"`
	Reason            string   `json:"reason"`
	Recommendation    string   `json:"recommendation"`
	Confidence        string   `json:"confidence"`
	EvidenceNeed      string   `json:"evidence_need"`
	SuggestedAction   string   `json:"suggested_action"`
	DrillDown         []string `json:"drill_down"`
	SuggestedCommands []string `json:"suggested_commands,omitempty"`
}

type MaintenanceReconciliationReview struct {
	AuthorityBoundary string                                    `json:"authority_boundary"`
	ProposalCount     int                                       `json:"proposal_count"`
	OmittedProposals  int                                       `json:"omitted_proposals,omitempty"`
	ByKind            map[string]int                            `json:"by_kind"`
	SuggestedCommands []string                                  `json:"suggested_commands,omitempty"`
	Proposals         []MaintenanceReconciliationReviewProposal `json:"proposals,omitempty"`
}

type MaintenanceReconciliationReviewProposal struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	GroupID           string   `json:"group_id,omitempty"`
	Category          string   `json:"category,omitempty"`
	Reason            string   `json:"reason"`
	DecisionRefs      []string `json:"decision_refs,omitempty"`
	Fanout            int      `json:"fanout,omitempty"`
	FallbackTargets   []string `json:"fallback_targets,omitempty"`
	ScopeRepairHints  []string `json:"scope_repair_hints,omitempty"`
	SuggestedCommand  string   `json:"suggested_command"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type maintenanceJudgmentClass struct {
	recommendation  string
	confidence      string
	evidenceNeed    string
	suggestedAction string
}

func BuildMaintenanceJudgmentReview(plan *MaintenancePlan) *MaintenanceJudgmentReview {
	review := &MaintenanceJudgmentReview{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		AuthorityBoundary: maintenanceReviewAuthority(),
		Counts:            newMaintenanceJudgmentCounts(),
		Groups:            []MaintenanceJudgmentGroup{},
	}
	if plan == nil {
		return review
	}

	review.SourcePlanGeneratedAt = plan.GeneratedAt
	review.TotalTasks = len(plan.Tasks)

	groups := make(map[string]*MaintenanceJudgmentGroup)
	for _, task := range plan.Tasks {
		if task.Rung != RungJudgment {
			review.OmittedNonJudgment++
			continue
		}

		item := maintenanceJudgmentTaskReview(task)
		review.JudgmentTasks++
		review.Counts.ByRecommendation[item.Recommendation]++
		review.Counts.ByConfidence[item.Confidence]++
		review.Counts.BySource[item.Source]++

		key := maintenanceJudgmentGroupKey(item)
		group := groups[key]
		if group == nil {
			group = &MaintenanceJudgmentGroup{
				Recommendation:  item.Recommendation,
				Confidence:      item.Confidence,
				Source:          item.Source,
				Category:        item.Category,
				EvidenceNeed:    item.EvidenceNeed,
				SuggestedAction: item.SuggestedAction,
				DrillDown:       append([]string(nil), item.DrillDown...),
				Tasks:           []MaintenanceJudgmentTaskReview{},
			}
			groups[key] = group
		}
		group.TaskCount++
		group.Tasks = append(group.Tasks, item)
	}

	for _, group := range groups {
		sortMaintenanceJudgmentTasks(group.Tasks)
		review.Groups = append(review.Groups, *group)
	}
	sortMaintenanceJudgmentGroups(review.Groups)
	return review
}

func CompactMaintenanceJudgmentReview(
	review *MaintenanceJudgmentReview,
	limit int,
) *MaintenanceJudgmentReview {
	if review == nil {
		return nil
	}
	if limit <= 0 {
		return review
	}

	compact := cloneMaintenanceJudgmentReview(review)
	compact.View = "compact"
	compact.FullAuditCommand = "haft overseer judgment --json"
	remaining := limit
	for index := range compact.Groups {
		group := &compact.Groups[index]
		if len(group.Tasks) <= remaining {
			remaining -= len(group.Tasks)
			continue
		}
		omitted := len(group.Tasks) - remaining
		if remaining > 0 {
			group.Tasks = append([]MaintenanceJudgmentTaskReview(nil), group.Tasks[:remaining]...)
		} else {
			group.Tasks = []MaintenanceJudgmentTaskReview{}
		}
		group.OmittedTasks = omitted
		compact.OmittedJudgmentTasks += omitted
		remaining = 0
	}
	if compact.Reconciliation != nil && len(compact.Reconciliation.Proposals) > limit {
		compact.Reconciliation.OmittedProposals = len(compact.Reconciliation.Proposals) - limit
		compact.Reconciliation.Proposals = append(
			[]MaintenanceReconciliationReviewProposal(nil),
			compact.Reconciliation.Proposals[:limit]...,
		)
	}
	return compact
}

func BuildMaintenanceReconciliationReview(
	proposals []MaintenanceReconciliationReviewProposal,
) *MaintenanceReconciliationReview {
	normalized := normalizeMaintenanceReconciliationReviewProposals(proposals)
	if len(normalized) == 0 {
		return nil
	}

	review := &MaintenanceReconciliationReview{
		AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
		ProposalCount:     len(normalized),
		ByKind:            map[string]int{},
		Proposals:         normalized,
	}
	commands := map[string]bool{}
	for _, proposal := range normalized {
		review.ByKind[proposal.Kind]++
		if proposal.SuggestedCommand != "" && !commands[proposal.SuggestedCommand] {
			commands[proposal.SuggestedCommand] = true
			review.SuggestedCommands = append(review.SuggestedCommands, proposal.SuggestedCommand)
		}
	}
	sort.Strings(review.SuggestedCommands)
	return review
}

func cloneMaintenanceJudgmentReview(
	review *MaintenanceJudgmentReview,
) *MaintenanceJudgmentReview {
	clone := *review
	clone.Counts = cloneMaintenanceJudgmentCounts(review.Counts)
	clone.Groups = make([]MaintenanceJudgmentGroup, len(review.Groups))
	for index, group := range review.Groups {
		clone.Groups[index] = cloneMaintenanceJudgmentGroup(group)
	}
	if review.Reconciliation != nil {
		clone.Reconciliation = cloneMaintenanceReconciliationReview(review.Reconciliation)
	}
	return &clone
}

func cloneMaintenanceJudgmentCounts(
	counts MaintenanceJudgmentCounts,
) MaintenanceJudgmentCounts {
	return MaintenanceJudgmentCounts{
		ByRecommendation: cloneStringIntMap(counts.ByRecommendation),
		ByConfidence:     cloneStringIntMap(counts.ByConfidence),
		BySource:         cloneStringIntMap(counts.BySource),
	}
}

func cloneStringIntMap(source map[string]int) map[string]int {
	if source == nil {
		return nil
	}
	clone := make(map[string]int, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneMaintenanceJudgmentGroup(
	group MaintenanceJudgmentGroup,
) MaintenanceJudgmentGroup {
	clone := group
	clone.DrillDown = append([]string(nil), group.DrillDown...)
	clone.Tasks = append([]MaintenanceJudgmentTaskReview(nil), group.Tasks...)
	for index := range clone.Tasks {
		clone.Tasks[index].DrillDown = append([]string(nil), clone.Tasks[index].DrillDown...)
		clone.Tasks[index].SuggestedCommands = append([]string(nil), clone.Tasks[index].SuggestedCommands...)
	}
	return clone
}

func cloneMaintenanceReconciliationReview(
	review *MaintenanceReconciliationReview,
) *MaintenanceReconciliationReview {
	clone := *review
	clone.ByKind = cloneStringIntMap(review.ByKind)
	clone.SuggestedCommands = append([]string(nil), review.SuggestedCommands...)
	clone.Proposals = append([]MaintenanceReconciliationReviewProposal(nil), review.Proposals...)
	for index := range clone.Proposals {
		clone.Proposals[index].DecisionRefs = append([]string(nil), clone.Proposals[index].DecisionRefs...)
		clone.Proposals[index].FallbackTargets = append([]string(nil), clone.Proposals[index].FallbackTargets...)
		clone.Proposals[index].ScopeRepairHints = append([]string(nil), clone.Proposals[index].ScopeRepairHints...)
	}
	return &clone
}

func maintenanceReviewAuthority() MaintenanceReviewAuthority {
	return MaintenanceReviewAuthority{
		Mutation:     "not_mutation",
		Approval:     "not_approval",
		Evidence:     "not_evidence",
		AgentRole:    "first_pass_judgment_review",
		ApplySurface: "operator_approval_required",
	}
}

func normalizeMaintenanceReconciliationReviewProposals(
	proposals []MaintenanceReconciliationReviewProposal,
) []MaintenanceReconciliationReviewProposal {
	out := make([]MaintenanceReconciliationReviewProposal, 0, len(proposals))
	for index, proposal := range proposals {
		proposal.ID = strings.TrimSpace(proposal.ID)
		if proposal.ID == "" {
			proposal.ID = fmt.Sprintf("reconcile-proposal-%03d", index+1)
		}
		proposal.Kind = strings.TrimSpace(proposal.Kind)
		proposal.GroupID = strings.TrimSpace(proposal.GroupID)
		proposal.Category = strings.TrimSpace(proposal.Category)
		proposal.Reason = strings.TrimSpace(proposal.Reason)
		proposal.DecisionRefs = compactStrings(proposal.DecisionRefs)
		proposal.FallbackTargets = compactStrings(proposal.FallbackTargets)
		proposal.ScopeRepairHints = compactStrings(proposal.ScopeRepairHints)
		proposal.SuggestedCommand = strings.TrimSpace(proposal.SuggestedCommand)
		proposal.AuthorityBoundary = strings.TrimSpace(proposal.AuthorityBoundary)
		if proposal.AuthorityBoundary == "" {
			proposal.AuthorityBoundary = "read_only_reconciliation_proposal_not_binding_authority"
		}
		if proposal.Kind == "" || proposal.Reason == "" {
			continue
		}
		out = append(out, proposal)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func newMaintenanceJudgmentCounts() MaintenanceJudgmentCounts {
	return MaintenanceJudgmentCounts{
		ByRecommendation: map[string]int{},
		ByConfidence:     map[string]int{},
		BySource:         map[string]int{},
	}
}

func maintenanceJudgmentTaskReview(task MaintenanceTask) MaintenanceJudgmentTaskReview {
	class := classifyMaintenanceJudgmentTask(task)
	return MaintenanceJudgmentTaskReview{
		DecisionRef:       task.DecisionRef,
		DecisionTitle:     task.DecisionTitle,
		Source:            task.Source,
		Category:          task.Category,
		ClaimID:           task.ClaimID,
		Observable:        task.Observable,
		Threshold:         task.Threshold,
		Reason:            task.Reason,
		Recommendation:    class.recommendation,
		Confidence:        class.confidence,
		EvidenceNeed:      class.evidenceNeed,
		SuggestedAction:   class.suggestedAction,
		DrillDown:         maintenanceJudgmentDrillDown(task),
		SuggestedCommands: maintenanceJudgmentSuggestedCommands(task, class),
	}
}

func classifyMaintenanceJudgmentTask(task MaintenanceTask) maintenanceJudgmentClass {
	switch task.Source {
	case "drift":
		return classifyDriftJudgmentTask(task)
	case "stale":
		return classifyStaleJudgmentTask(task)
	default:
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationTriageStaleDecision,
			confidence:      JudgmentConfidenceLow,
			evidenceNeed:    "inspect task source and recover the exact stale/drift record",
			suggestedAction: "route manually; the task source is not recognized by the judgment reviewer",
		}
	}
}

func classifyDriftJudgmentTask(task MaintenanceTask) maintenanceJudgmentClass {
	switch task.Category {
	case string(StageForConfirm):
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationReviewMaterialDrift,
			confidence:      JudgmentConfidenceHigh,
			evidenceNeed:    "read the governed-file diff and decide whether the symbol/body change preserves the decision",
			suggestedAction: "if benign, approve rebaseline; if material, reopen or supersede the decision",
		}
	case string(SurfaceForReview):
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationReviewUnprovenDrift,
			confidence:      JudgmentConfidenceMedium,
			evidenceNeed:    "inspect the diff and add focused test/evidence because benignity was not provable from symbol data",
			suggestedAction: "agent should prepare evidence; operator then approves rebaseline, reopen, or supersede",
		}
	default:
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationReviewUnprovenDrift,
			confidence:      JudgmentConfidenceLow,
			evidenceNeed:    "recover the drift category and inspect verbose drift output",
			suggestedAction: "do not apply; inspect verbose scan first",
		}
	}
}

func classifyStaleJudgmentTask(task MaintenanceTask) maintenanceJudgmentClass {
	switch task.Category {
	case string(StaleCategoryPendingVerification):
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationVerifyClaim,
			confidence:      JudgmentConfidenceMedium,
			evidenceNeed:    claimEvidenceNeed(task),
			suggestedAction: "agent should collect fresh evidence for the claim; attach evidence or report why it cannot be checked",
		}
	case string(StaleCategoryEvidenceExpired), string(StaleCategoryDecisionStale):
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationRefreshEvidence,
			confidence:      JudgmentConfidenceMedium,
			evidenceNeed:    claimEvidenceNeed(task),
			suggestedAction: "agent should refresh evidence; operator may waive only with explicit evidence and new valid_until",
		}
	case string(StaleCategoryREffDegraded):
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationReopenOrSupersede,
			confidence:      JudgmentConfidenceLow,
			evidenceNeed:    "inspect degraded evidence chain and identify the weakest failed claim",
			suggestedAction: "prefer reopen or supersede over waiving degraded trust without new evidence",
		}
	default:
		return maintenanceJudgmentClass{
			recommendation:  JudgmentRecommendationTriageStaleDecision,
			confidence:      JudgmentConfidenceLow,
			evidenceNeed:    "inspect stale reason and recover the governing claim/evidence record",
			suggestedAction: "manual triage required; no safe batch action yet",
		}
	}
}

func claimEvidenceNeed(task MaintenanceTask) string {
	if strings.TrimSpace(task.Observable) == "" && strings.TrimSpace(task.Threshold) == "" {
		return "recover claim observable and threshold before deciding"
	}
	if strings.TrimSpace(task.Threshold) == "" {
		return "check observable: " + task.Observable
	}
	if strings.TrimSpace(task.Observable) == "" {
		return "recover observable; threshold: " + task.Threshold
	}
	return fmt.Sprintf("check observable %q against threshold %q", task.Observable, task.Threshold)
}

func maintenanceJudgmentDrillDown(task MaintenanceTask) []string {
	out := []string{
		`haft_refresh(action="plan")`,
		`haft_refresh(action="scan", verbose=true)`,
	}
	if task.DecisionRef != "" {
		out = append(out, fmt.Sprintf(`haft_query(action="related", artifact_ref="%s")`, task.DecisionRef))
	}
	return out
}

func maintenanceJudgmentSuggestedCommands(task MaintenanceTask, class maintenanceJudgmentClass) []string {
	if task.DecisionRef == "" {
		return nil
	}
	switch class.recommendation {
	case JudgmentRecommendationReviewMaterialDrift, JudgmentRecommendationReviewUnprovenDrift:
		return []string{
			fmt.Sprintf(`haft_decision(action="baseline", decision_ref="%s") # only after operator approves drift as benign`, task.DecisionRef),
			fmt.Sprintf(`haft_refresh(action="reopen", artifact_ref="%s", reason="material drift after judgment review")`, task.DecisionRef),
			fmt.Sprintf(`haft_refresh(action="supersede", artifact_ref="%s", new_artifact_ref="dec-...", reason="decision replaced after judgment review")`, task.DecisionRef),
		}
	case JudgmentRecommendationVerifyClaim, JudgmentRecommendationRefreshEvidence:
		if task.ClaimID != "" {
			return []string{
				fmt.Sprintf(`haft_decision(action="evidence", artifact_ref="%s", claim_refs=["%s"], evidence_content="...", evidence_verdict="supports|weakens")`, task.DecisionRef, task.ClaimID),
				fmt.Sprintf(`haft_refresh(action="waive", artifact_ref="%s", evidence="fresh evidence refs ...", new_valid_until="YYYY-MM-DD") # operator approval only`, task.DecisionRef),
			}
		}
		return []string{
			fmt.Sprintf(`haft_decision(action="evidence", artifact_ref="%s", evidence_content="...", evidence_verdict="supports|weakens")`, task.DecisionRef),
			fmt.Sprintf(`haft_refresh(action="waive", artifact_ref="%s", evidence="fresh evidence refs ...", new_valid_until="YYYY-MM-DD") # operator approval only`, task.DecisionRef),
		}
	case JudgmentRecommendationReopenOrSupersede:
		return []string{
			fmt.Sprintf(`haft_refresh(action="reopen", artifact_ref="%s", reason="R_eff degraded after judgment review")`, task.DecisionRef),
			fmt.Sprintf(`haft_refresh(action="supersede", artifact_ref="%s", new_artifact_ref="dec-...", reason="stale/degraded decision replaced")`, task.DecisionRef),
		}
	default:
		return []string{fmt.Sprintf(`haft_refresh(action="scan", verbose=true) # inspect %s before choosing a mutation`, task.DecisionRef)}
	}
}

func maintenanceJudgmentGroupKey(item MaintenanceJudgmentTaskReview) string {
	return strings.Join([]string{item.Recommendation, item.Confidence, item.Source, item.Category}, "\x00")
}

func sortMaintenanceJudgmentTasks(tasks []MaintenanceJudgmentTaskReview) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].DecisionRef != tasks[j].DecisionRef {
			return tasks[i].DecisionRef < tasks[j].DecisionRef
		}
		if tasks[i].ClaimID != tasks[j].ClaimID {
			return tasks[i].ClaimID < tasks[j].ClaimID
		}
		return tasks[i].Reason < tasks[j].Reason
	})
}

func sortMaintenanceJudgmentGroups(groups []MaintenanceJudgmentGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		left := maintenanceJudgmentGroupSortKey(groups[i])
		right := maintenanceJudgmentGroupSortKey(groups[j])
		return left < right
	})
}

func maintenanceJudgmentGroupSortKey(group MaintenanceJudgmentGroup) string {
	return strings.Join([]string{
		group.Recommendation,
		group.Confidence,
		group.Source,
		group.Category,
	}, "\x00")
}
