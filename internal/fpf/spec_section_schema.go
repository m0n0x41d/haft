package fpf

func haftSpecSectionTool() Tool {
	return Tool{
		Name: "haft_spec_section",
		Description: "Drive the Haft v7 spec onboarding method one step at a time. " +
			"`next_step` returns a typed WorkflowIntent (which onboarding phase is " +
			"next, what the human should decide, what context the host agent needs " +
			"to draft the section, which YAML fields the section must carry, " +
			"which structural Checks the resulting section must satisfy). " +
			"`approve` records a SpecSectionBaseline so drift detection becomes " +
			"meaningful; `rebaseline` overwrites a baseline after the operator " +
			"confirms drift is intentional evolution; `reopen` deletes a baseline " +
			"so the section returns to the onboarding loop. Surfaces (MCP plugin, " +
			"Desktop wizard, CLI) all consume the same intent shape.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{
						"next_step",
						"approve",
						"rebaseline",
						"reopen",
					},
					"description": "next_step=return the next typed WorkflowIntent. approve=record a baseline for an active section that has none. rebaseline=overwrite an existing baseline after confirming the carrier change is intentional. reopen=delete a baseline so the section re-enters the onboarding loop.",
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
					"description": "(approve/rebaseline) Identifier of who approved the baseline. Defaults to 'human' when omitted; use 'agent' when the host agent is acting on explicit operator authority.",
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
