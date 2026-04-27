package fpf

func haftCommissionTool() Tool {
	return Tool{
		Name:        "haft_commission",
		Description: "Create, list, show, claim, requeue, cancel, and update WorkCommissions. WorkCommissions are bounded execution authorizations between DecisionRecords and RuntimeRuns; external trackers are optional projections, not work authority.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"create", "create_from_decision", "create_batch_from_decisions", "create_from_plan", "list", "list_runnable", "show", "claim_for_preflight", "requeue", "cancel", "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block"},
					"description": "create=persist a WorkCommission, create_from_decision=derive one from an active DecisionRecord plus explicit repo/scope inputs, create_batch_from_decisions=derive one WorkCommission per active DecisionRecord, create_from_plan=derive WorkCommissions from an ImplementationPlan-lite object, list=return commissions by selector open/stale/terminal/runnable/all with operator hints, list_runnable=return queued/ready non-expired dependency-satisfied commissions, show=return one WorkCommission, claim_for_preflight=atomically move one runnable commission to preflighting, requeue=operator/runtime recovery that clears an active lease and returns allowed states to queued, cancel=operator cancellation that keeps the audit record, other actions append lifecycle facts.",
				},
				"commission": map[string]interface{}{
					"type":        "object",
					"description": "(create) Canonical WorkCommission payload including decision_ref, problem_card_ref, spec_section_refs or spec_readiness_override, scope, evidence_requirements, projection_policy, delivery_policy, state, valid_until, fetched_at.",
				},
				"plan": map[string]interface{}{
					"type":        "object",
					"description": "(create_from_plan) ImplementationPlan-lite object with id, revision, optional defaults, and decisions. Decision entries may declare depends_on using other same-plan DecisionRecord ids; Haft maps these to WorkCommission dependencies and enforces them at list/claim time.",
				},
				"autonomy_envelope_snapshot": map[string]interface{}{
					"type":        "object",
					"description": "(create/create_from_decision/create_from_plan) Optional human-approved AutonomyEnvelope snapshot. It may only further restrict runnable commissions; it cannot skip freshness, scope, evidence, lease, lockset, or one-way-door gates.",
				},
				"autonomy_envelope_ref": map[string]string{
					"type":        "string",
					"description": "(create/create_from_decision/create_from_plan) Optional AutonomyEnvelope id carried in the CommissionSnapshot equality set.",
				},
				"autonomy_envelope_revision": map[string]string{
					"type":        "string",
					"description": "(create/create_from_decision/create_from_plan/list_runnable/claim_for_preflight) Optional AutonomyEnvelope revision/hash carried in the CommissionSnapshot equality set.",
				},
				"commission_id": map[string]string{
					"type":        "string",
					"description": "(show/claim/requeue/cancel/update) WorkCommission id. Optional for claim_for_preflight; omitted means claim the first runnable commission matching filters.",
				},
				"decision_ref": map[string]string{
					"type":        "string",
					"description": "(create_from_decision) Active DecisionRecord id to commission.",
				},
				"project_root": map[string]string{
					"type":        "string",
					"description": "(create_from_decision/create_from_plan) Project root used to snapshot SpecSection revision hashes when available.",
				},
				"spec_section_refs": map[string]interface{}{
					"type":        "array",
					"items":       map[string]string{"type": "string"},
					"description": "(create) SpecSection ids carried by a raw WorkCommission payload. Missing refs require spec_readiness_override with out_of_spec tactical reason.",
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
					"description": "(list/list_runnable) Selector name. list supports open, stale, terminal, runnable, all. list_runnable is equivalent to runnable.",
				},
				"state": map[string]string{
					"type":        "string",
					"description": "(list) Optional exact WorkCommission state filter.",
				},
				"older_than": map[string]string{
					"type":        "string",
					"description": "(list/show operator hints) Open commission age threshold as a Go duration, e.g. 24h.",
				},
				"plan_ref": map[string]string{
					"type":        "string",
					"description": "(list_runnable/claim_for_preflight) Optional ImplementationPlan id filter.",
				},
				"plan_revision": map[string]string{
					"type":        "string",
					"description": "(list_runnable/claim_for_preflight) Optional ImplementationPlan revision filter used with plan_ref.",
				},
				"queue": map[string]string{
					"type":        "string",
					"description": "(create_from_plan/list_runnable/claim_for_preflight) Optional queue label for operator-controlled batches.",
				},
				"delivery_policy": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"workspace_patch_manual", "workspace_patch_auto_on_pass"},
					"description": "(create/create_from_decision/create_from_plan) How a completed workspace diff is adopted. MVP default is workspace_patch_manual.",
				},
				"projection_policy": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"local_only", "external_optional", "external_required"},
					"description": "(create/create_from_decision/create_from_plan) External projection policy. local_only never requires tracker publication; external_required records ProjectionDebt when local evidence passes but external publication is missing or failed.",
				},
				"spec_readiness_override": map[string]interface{}{
					"type":        "object",
					"description": "(create_from_decision/create_from_plan) Explicit tactical override record for out-of-spec work, including kind=tactical, out_of_spec=true, project_readiness, and reason.",
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
					"description": "(record_preflight/complete_or_block/requeue/cancel) Deterministic reason for block/failure/recovery/operator closure.",
				},
				"payload": map[string]interface{}{
					"type":        "object",
					"description": "Additional lifecycle payload. It is recorded as data, not authority expansion. For complete_or_block/pass, payload.external_publication may report carrier/target/state/last_error/retry_policy; local RuntimeRun evidence remains separate from external carrier sync.",
				},
			},
			"required": []string{"action"},
			"allOf": []interface{}{
				commissionActionRequires("show", []string{"commission_id"}),
				commissionActionRequires("requeue", []string{"commission_id", "reason"}),
				commissionActionRequires("cancel", []string{"commission_id", "reason"}),
			},
		},
	}
}

func commissionActionRequires(action string, required []string) map[string]interface{} {
	return map[string]interface{}{
		"if": map[string]interface{}{
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"const": action,
				},
			},
		},
		"then": map[string]interface{}{
			"required": required,
		},
	}
}
