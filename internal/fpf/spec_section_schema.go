package fpf

func haftSpecSectionTool() Tool {
	return Tool{
		Name: "haft_spec_section",
		Description: "Drive the Haft v7 spec lifecycle method one step at a time. " +
			"`lifecycle` returns the typed UX projection: the next admissible " +
			"operator action, carrier, section identity, findings, and the " +
			"underlying WorkflowIntent. " +
			"`next_step` returns a typed WorkflowIntent (which onboarding phase is " +
			"next, what the human should decide, what context the host agent needs " +
			"to draft the section, which YAML fields the section must carry, " +
			"which structural Checks the resulting section must satisfy). " +
			"`approve` records a SpecSectionBaseline so drift detection becomes " +
			"meaningful; `rebaseline` overwrites a baseline after the operator " +
			"confirms drift is intentional evolution; `reopen` deletes a baseline " +
			"so the section returns to the onboarding loop. Approval actions are " +
			"binding governance acts; MCP fails them closed by default with " +
			"operator_confirmation_required because model-supplied arguments are " +
			"not kernel-verifiable manual_cli authorization receipts. " +
			"Lifecycle and mutation JSON expose baseline_kind/profile so " +
			"SpecSectionApprovalBaseline stays distinct from other snapshots. " +
			"Surfaces (MCP plugin, host workflow, CLI) all consume the " +
			"same intent shape.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{
						"lifecycle",
						"next_step",
						"approve",
						"rebaseline",
						"reopen",
					},
					"description": "lifecycle=typed spec lifecycle projection with baseline profile. next_step=legacy WorkflowIntent. approve/rebaseline/reopen are binding mutations and return operator_confirmation_required in default MCP cli-only mode.",
				},
				"project_root": map[string]string{
					"type":        "string",
					"description": "Project root containing .haft/specs/*. Optional; defaults to the server-bound project.",
				},
				"section_id": map[string]string{
					"type":        "string",
					"description": "(approve/rebaseline/reopen) SpecSection id (e.g. 'TS.environment-change.001'). Must match an active section in the carriers for approve/rebaseline.",
				},
				"approved_by": map[string]string{
					"type":        "string",
					"description": "(approve/rebaseline) Identifier recorded by manual/authorized paths. In default MCP cli-only mode this is not accepted as proof of human authorship.",
				},
				"reason": map[string]string{
					"type":        "string",
					"description": "(rebaseline/reopen) Free-text rationale recorded in the response so the audit trail explains why the baseline changed.",
				},
			},
			"required": []string{"action"},
		},
	}
}
