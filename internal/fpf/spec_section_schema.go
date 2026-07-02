package fpf

func haftSpecSectionTool() Tool {
	return Tool{
		Name:        "haft_spec_section",
		Description: "Spec lifecycle projection and binding mutations. Approval actions fail closed by default; host receipts require a registered kernel verifier.",
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
					"description": "lifecycle=typed projection. next_step=WorkflowIntent. approve/rebaseline/reopen return operator_confirmation_required in default MCP cli-only mode; host receipts require a registered kernel verifier.",
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
