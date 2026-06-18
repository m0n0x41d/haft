package artifact

import "strings"

const (
	BlockedUseAttentionSchemaVersion = 1
	BlockedUseAttentionRecordKind    = "blocked_use_attention_item"
	BlockedUseAttentionAuthority     = "read_only_attention_projection"

	BlockedUseSourceReturnDeclared = "source_return_declared"
	BlockedUseExactRecordNeeded    = "exact_record_needed"
	BlockedUseMissingSourceReturn  = "missing_source_return"

	BlockedUseBoundaryNotWorkPlan     = "not_work_plan"
	BlockedUseBoundaryNotApproval     = "not_approval"
	BlockedUseBoundaryNotEvidence     = "not_evidence"
	BlockedUseBoundaryNotGateDecision = "not_gate_decision"
	BlockedUseBoundaryNotGlobalTruth  = "not_global_truth"
)

type BlockedUseAttentionInput struct {
	BearerRef                 string
	EntityOrSubjectLabel      string
	FindingKind               string
	BlockedUse                string
	SourceRefs                []string
	ExactRecordNeeded         string
	NextAdmissibleActions     []string
	RequiredRoleAssignmentRef string
	ValidUntil                string
}

type BlockedUseAttentionItem struct {
	SchemaVersion             int                         `json:"schema_version"`
	RecordKind                string                      `json:"record_kind"`
	Authority                 string                      `json:"authority"`
	Object                    BlockedUseAttentionObject   `json:"object"`
	FindingKind               string                      `json:"finding_kind"`
	BlockedUse                string                      `json:"blocked_use"`
	SourceReturn              BlockedUseSourceReturn      `json:"source_return"`
	NextAdmissibleActions     []string                    `json:"next_admissible_actions"`
	RequiredRoleAssignmentRef string                      `json:"required_role_assignment_ref,omitempty"`
	ValidUntil                string                      `json:"valid_until,omitempty"`
	AuthorityBoundary         BlockedUseAttentionBoundary `json:"authority_boundary"`
}

type BlockedUseAttentionObject struct {
	BearerRef            string `json:"bearer_ref"`
	EntityOrSubjectLabel string `json:"entity_or_subject_label,omitempty"`
}

type BlockedUseSourceReturn struct {
	Status            string   `json:"status"`
	SourceRefs        []string `json:"source_refs,omitempty"`
	ExactRecordNeeded string   `json:"exact_record_needed,omitempty"`
}

type BlockedUseAttentionBoundary struct {
	WorkPlan     string `json:"work_plan"`
	Evidence     string `json:"evidence"`
	Approval     string `json:"approval"`
	GateDecision string `json:"gate_decision"`
	GlobalTruth  string `json:"global_truth"`
}

func BuildBlockedUseAttentionItem(input BlockedUseAttentionInput) BlockedUseAttentionItem {
	normalized := normalizeBlockedUseAttentionInput(input)

	return BlockedUseAttentionItem{
		SchemaVersion: BlockedUseAttentionSchemaVersion,
		RecordKind:    BlockedUseAttentionRecordKind,
		Authority:     BlockedUseAttentionAuthority,
		Object: BlockedUseAttentionObject{
			BearerRef:            normalized.BearerRef,
			EntityOrSubjectLabel: normalized.EntityOrSubjectLabel,
		},
		FindingKind:               normalized.FindingKind,
		BlockedUse:                normalized.BlockedUse,
		SourceReturn:              blockedUseSourceReturn(normalized),
		NextAdmissibleActions:     blockedUseNextActions(normalized),
		RequiredRoleAssignmentRef: normalized.RequiredRoleAssignmentRef,
		ValidUntil:                normalized.ValidUntil,
		AuthorityBoundary: BlockedUseAttentionBoundary{
			WorkPlan:     BlockedUseBoundaryNotWorkPlan,
			Evidence:     BlockedUseBoundaryNotEvidence,
			Approval:     BlockedUseBoundaryNotApproval,
			GateDecision: BlockedUseBoundaryNotGateDecision,
			GlobalTruth:  BlockedUseBoundaryNotGlobalTruth,
		},
	}
}

func normalizeBlockedUseAttentionInput(input BlockedUseAttentionInput) BlockedUseAttentionInput {
	return BlockedUseAttentionInput{
		BearerRef:                 strings.TrimSpace(input.BearerRef),
		EntityOrSubjectLabel:      strings.TrimSpace(input.EntityOrSubjectLabel),
		FindingKind:               strings.TrimSpace(input.FindingKind),
		BlockedUse:                strings.TrimSpace(input.BlockedUse),
		SourceRefs:                compactStrings(input.SourceRefs),
		ExactRecordNeeded:         strings.TrimSpace(input.ExactRecordNeeded),
		NextAdmissibleActions:     compactStrings(input.NextAdmissibleActions),
		RequiredRoleAssignmentRef: strings.TrimSpace(input.RequiredRoleAssignmentRef),
		ValidUntil:                strings.TrimSpace(input.ValidUntil),
	}
}

func blockedUseSourceReturn(input BlockedUseAttentionInput) BlockedUseSourceReturn {
	status := BlockedUseSourceReturnDeclared
	if len(input.SourceRefs) == 0 {
		status = BlockedUseMissingSourceReturn
	}
	if input.ExactRecordNeeded != "" {
		status = BlockedUseExactRecordNeeded
	}

	return BlockedUseSourceReturn{
		Status:            status,
		SourceRefs:        append([]string(nil), input.SourceRefs...),
		ExactRecordNeeded: input.ExactRecordNeeded,
	}
}

func blockedUseNextActions(input BlockedUseAttentionInput) []string {
	if len(input.NextAdmissibleActions) > 0 {
		return append([]string(nil), input.NextAdmissibleActions...)
	}

	actions := []string{}
	if input.ExactRecordNeeded != "" || len(input.SourceRefs) == 0 {
		actions = append(actions, "recover_exact_source_record")
	}
	if input.FindingKind != "" {
		actions = append(actions, "review_finding_kind")
	}
	if input.RequiredRoleAssignmentRef != "" {
		actions = append(actions, "assign_required_role")
	}
	if len(actions) == 0 {
		actions = append(actions, "review_blocked_use")
	}

	return actions
}
