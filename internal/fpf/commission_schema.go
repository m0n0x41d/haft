package fpf

func haftCommissionTool() Tool {
	return Tool{
		Name:        "haft_commission",
		Description: "Create, list, claim, and update WorkCommissions. WorkCommissions are bounded execution authorizations between DecisionRecords and RuntimeRuns; external trackers are optional projections, not work authority.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"create", "create_from_decision", "create_batch_from_decisions", "create_from_plan", "list_runnable", "claim_for_preflight", "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block"},
					"description": "create=persist a WorkCommission, create_from_decision=derive one from an active DecisionRecord plus explicit repo/scope inputs, create_batch_from_decisions=derive one WorkCommission per active DecisionRecord, create_from_plan=derive WorkCommissions from an ImplementationPlan-lite object, list_runnable=return queued/ready non-expired dependency-satisfied commissions, claim_for_preflight=atomically move one runnable commission to preflighting, other actions append lifecycle facts.",
				},
				"commission": map[string]interface{}{
					"type":        "object",
					"description": "(create) Canonical WorkCommission payload including decision_ref, problem_card_ref, scope, evidence_requirements, projection_policy, state, valid_until, fetched_at.",
				},
				"plan": map[string]interface{}{
					"type":        "object",
					"description": "(create_from_plan) ImplementationPlan-lite object with id, revision, optional defaults, and decisions. Decision entries may declare depends_on using other same-plan DecisionRecord ids; Haft maps these to WorkCommission dependencies and enforces them at list/claim time.",
				},
				"commission_id": map[string]string{
					"type":        "string",
					"description": "(claim/update) WorkCommission id. Optional for claim_for_preflight; omitted means claim the first runnable commission matching filters.",
				},
				"decision_ref": map[string]string{
					"type":        "string",
					"description": "(create_from_decision) Active DecisionRecord id to commission.",
				},
				"decision_refs": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create_batch_from_decisions) Active DecisionRecord ids to commission as a batch.",
				},
				"repo_ref": map[string]string{
					"type":        "string",
					"description": "(create_from_decision) Repository authority ref recorded in Scope, e.g. local:haft.",
				},
				"base_sha": map[string]string{
					"type":        "string",
					"description": "(create_from_decision) Base git SHA frozen into Scope.",
				},
				"target_branch": map[string]string{
					"type":        "string",
					"description": "(create_from_decision) Target branch policy/suggestion for runner work.",
				},
				"allowed_paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create_from_decision) Paths the runner may mutate. Defaults to decision affected_files when omitted.",
				},
				"forbidden_paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create_from_decision) Paths the runner must not mutate.",
				},
				"allowed_actions": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create_from_decision) Allowed runner action names. Defaults to edit_files and run_tests.",
				},
				"lockset": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create_from_decision) Lockset paths/patterns. Defaults to affected_files.",
				},
				"selector": map[string]string{
					"type":        "string",
					"description": "(list_runnable) Selector name. MVP-1R supports runnable.",
				},
				"plan_ref": map[string]string{
					"type":        "string",
					"description": "(list_runnable/claim_for_preflight) Optional ImplementationPlan id filter.",
				},
				"queue": map[string]string{
					"type":        "string",
					"description": "(create_from_plan/list_runnable/claim_for_preflight) Optional queue label for operator-controlled batches.",
				},
				"runner_id": map[string]string{
					"type":        "string",
					"description": "Runner identity claiming or updating the commission.",
				},
				"event": map[string]string{
					"type":        "string",
					"description": "(record_run_event) Runtime event name.",
				},
				"verdict": map[string]string{
					"type":        "string",
					"description": "(record_preflight/complete_or_block) pass, fail, blocked, completed, failed.",
				},
				"reason": map[string]string{
					"type":        "string",
					"description": "(record_preflight/complete_or_block) Deterministic reason for block/failure.",
				},
				"payload": map[string]interface{}{
					"type":        "object",
					"description": "Additional lifecycle payload. It is recorded as data, not authority expansion.",
				},
			},
			"required": []string{"action"},
		},
	}
}
