package fpf

func haftSpecSectionTool() Tool {
	return Tool{
		Name:        "haft_spec_section",
		Description: "Project/scope-level specification workflow projection, profile-independent draft contract, exact-section projection, and binding mutations over project SQL editions. Project workflow readiness is not exact SpecSection lifecycle or stronger-use admission. The draft contract does not establish applicability, activation, approval, or evidence. This tool does not establish compatibility with a newer FPF source. Approval actions fail closed by default; host receipts require a registered kernel verifier.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{
						"lifecycle",
						"next_step",
						"draft_contract",
						"project",
						"approve",
						"rebaseline",
						"reopen",
					},
					"description": "lifecycle=project/scope-level ProjectSpecificationSet workflow projection. next_step=its WorkflowIntent. Both reject section_id; exact-section reads use haft_query action spec_trace or spec_use. draft_contract=canonical phases, fields, values, checks, and exact haft_query action=spec_validate continuation without resolving applicability. project=non-binding SpecSectionAtConcern projection from one exact current SQL edition. These actions do not compare section meaning with a newer FPF source. approve/rebaseline/reopen return operator_confirmation_required in default MCP cli-only mode; host receipts require a registered kernel verifier.",
				},
				"project_root": map[string]string{
					"type":        "string",
					"description": "Project root containing .haft/specs/*. Optional; defaults to the server-bound project.",
				},
				"scope_id": map[string]string{
					"type":        "string",
					"description": "(lifecycle/next_step) Exact canonical project-profile ScopeID. Optional for a singleton profile; required when several scopes exist. Rejected by draft_contract because that action is explicitly profile-independent. This is read-only selection, not profile authority.",
				},
				"section_id": map[string]string{
					"type":        "string",
					"description": "(project/approve/rebaseline/reopen) SpecSection id (e.g. 'TS.environment-change.001'). Rejected for lifecycle/next_step because those actions are project/scope-level; use haft_query action spec_trace or spec_use for an exact section. project requires an exact current SQL edition; approve/rebaseline require an active section.",
				},
				"entity_ref": memoryEntityReferenceSchema(),
				"bounded_context_ref": map[string]interface{}{
					"type":        "string",
					"description": "(project) Exact typed-memory bounded context paired with entity_ref.",
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
