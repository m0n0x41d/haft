package fpf

func haftMethodTool() Tool {
	return Tool{
		Name:        "haft_method",
		Description: "MethodRun lifecycle. Do not pass task, thread, commission, or work IDs as scope_id unless a prior response returned scope_choice_required.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"pull", "close", "show", "detail", "status", "catalog"},
					"description": "method action.",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "task.",
				},
				"declared_task_kind": map[string]interface{}{
					"type":        "string",
					"description": "kind.",
				},
				"change_intent": map[string]interface{}{
					"type":        "string",
					"description": "intent.",
				},
				"intended_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "files.",
				},
				"risk_signals": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":       map[string]interface{}{},
							"source":   map[string]interface{}{},
							"evidence": map[string]interface{}{},
						},
					},
					"description": "risks.",
				},
				"user_scope_constraints": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "scope.",
				},
				"artifact_refs": map[string]interface{}{
					"type":        "object",
					"description": "refs.",
				},
				"ceremony_request": map[string]interface{}{
					"type":        "string",
					"description": "ceremony.",
				},
				"response_budget": map[string]interface{}{
					"type":        "object",
					"description": "budget.",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "context.",
				},
				"scope_id": map[string]interface{}{
					"type":        "string",
					"description": "Exact canonical project ScopeID only after scope_choice_required for a multi-scope profile. Never pass task, thread, commission, or work IDs as selectors.",
				},
				"carry_through": map[string]interface{}{
					"type":        "array",
					"items":       methodCarryThroughItemSchema(),
					"description": "accepted items.",
				},
				"pull_id": map[string]interface{}{
					"type":        "string",
					"description": "mpull id.",
				},
				"changed_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "changed files.",
				},
				"gate_results": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"gate_id":       map[string]interface{}{},
							"status":        map[string]interface{}{"enum": []interface{}{"satisfied", "waived"}},
							"evidence_refs": map[string]interface{}{},
							"waiver_reason": map[string]interface{}{},
						},
					},
					"description": "gate results.",
				},
				"verification": map[string]interface{}{
					"type":        "object",
					"properties":  map[string]interface{}{"commands": map[string]interface{}{}, "result": map[string]interface{}{}, "output_ref": map[string]interface{}{}},
					"description": "verification.",
				},
				"waivers": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"gate_id": map[string]interface{}{}, "reason": map[string]interface{}{}},
					},
					"description": "waivers.",
				},
				"method_ref": map[string]interface{}{
					"type":        "string",
					"description": "method id.",
				},
				"method_status": map[string]interface{}{
					"description": "lifecycle filter.",
				},
				"limit": map[string]interface{}{
					"description": "limit.",
				},
			},
			"required": []string{"action"},
		},
	}
}

func methodCarryThroughItemSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source_ref":            map[string]interface{}{},
			"source_item_ref":       map[string]interface{}{},
			"acceptance_ref":        map[string]interface{}{},
			"acceptance_ref_kind":   map[string]interface{}{},
			"acceptance_ref_status": map[string]interface{}{},
			"disposition":           map[string]interface{}{},
			"target_refs":           map[string]interface{}{},
			"evidence_refs":         map[string]interface{}{},
			"reason":                map[string]interface{}{},
		},
		"required": []string{"source_ref", "source_item_ref", "acceptance_ref"},
	}
}
