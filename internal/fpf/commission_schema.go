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
					"enum":        []interface{}{"create", "list_runnable", "claim_for_preflight", "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block"},
					"description": "create=persist a WorkCommission, list_runnable=return queued/ready non-expired commissions, claim_for_preflight=atomically move one to preflighting, other actions append lifecycle facts.",
				},
				"commission": map[string]interface{}{
					"type":        "object",
					"description": "(create) Canonical WorkCommission payload including decision_ref, problem_card_ref, scope, evidence_requirements, projection_policy, state, valid_until, fetched_at.",
				},
				"commission_id": map[string]string{
					"type":        "string",
					"description": "(claim/update) WorkCommission id.",
				},
				"selector": map[string]string{
					"type":        "string",
					"description": "(list_runnable) Selector name. MVP-1R supports runnable.",
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
