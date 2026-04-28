package fpf

func haftSpecSectionTool() Tool {
	return Tool{
		Name: "haft_spec_section",
		Description: "Drive the Haft v7 spec onboarding method one step at a time. " +
			"Returns a typed WorkflowIntent: which onboarding phase is next, what " +
			"the human should decide, what context the host agent needs to draft " +
			"the section, which YAML fields the section must carry, and which " +
			"structural Checks the resulting section must satisfy. Surfaces " +
			"(MCP plugin, Desktop wizard, CLI) all consume the same intent shape.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"next_step"},
					"description": "next_step=return the next typed WorkflowIntent for the current project. Approve/rebaseline/reopen/rollback transitions arrive with the SpecSectionBaseline slice.",
				},
				"project_root": map[string]string{
					"type":        "string",
					"description": "Project root containing .haft/specs/*. Optional; defaults to the server-bound project.",
				},
			},
			"required": []string{"action"},
		},
	}
}
