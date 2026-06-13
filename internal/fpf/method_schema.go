package fpf

func haftMethodTool() Tool {
	return Tool{
		Name:        "haft_method",
		Description: "Pull compact task-local SWE method cards before non-trivial code work; close the same MethodRun with evidence or explicit waivers before claiming completion. No internal LLM classification.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"pull", "close", "show", "detail", "status"},
					"description": "pull=create an open MethodRun and return compact cards; close=validate hard gates by pull_id; show=read one run; detail=full catalog method; status=list open runs.",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Operator task in one sentence.",
				},
				"declared_task_kind": map[string]interface{}{
					"type":        "string",
					"description": "(pull) feature | bugfix | debug | refactor | external_integration | mechanical_edit | formatting_only | architecture.",
				},
				"change_intent": map[string]interface{}{
					"type":        "string",
					"description": "(pull) add_feature | fix_bug | refactor | change_behavior | mechanical_edit | formatting_only.",
				},
				"intended_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(pull) Files or path fragments the agent expects to touch.",
				},
				"risk_signals": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":       map[string]interface{}{"type": "string"},
							"source":   map[string]interface{}{"type": "string"},
							"evidence": map[string]interface{}{"type": "string"},
						},
						"required": []string{"id"},
					},
					"description": "(pull) Deterministic/agent-declared risk ids, e.g. external_io, domain_boundary, failing_test, governed_file.",
				},
				"user_scope_constraints": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(pull) Human limits such as allowed files, no public API change, no DB migration.",
				},
				"artifact_refs": map[string]interface{}{
					"type":        "object",
					"description": "(pull) Optional problem_ref, decision_ref, commission_ref links.",
				},
				"ceremony_request": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Optional none | low | medium | deep. Mechanical edits should use low/none.",
				},
				"response_budget": map[string]interface{}{
					"type":        "object",
					"description": "(pull) Optional max_methods<=3 and detail=compact.",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Optional context saved on the MethodRun artifact.",
				},
				"pull_id": map[string]interface{}{
					"type":        "string",
					"description": "(close/show) MethodRun id returned by pull, e.g. mpull-...",
				},
				"changed_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(close) Files actually changed.",
				},
				"gate_results": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"gate_id":       map[string]interface{}{"type": "string"},
							"status":        map[string]interface{}{"type": "string", "enum": []interface{}{"satisfied", "waived"}},
							"evidence_refs": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"waiver_reason": map[string]interface{}{"type": "string"},
						},
						"required": []string{"gate_id", "status"},
					},
					"description": "(close) Gate result objects with gate_id, status=satisfied, and evidence_refs when required.",
				},
				"verification": map[string]interface{}{
					"type":        "object",
					"properties":  map[string]interface{}{"commands": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}, "result": map[string]interface{}{"type": "string"}, "output_ref": map[string]interface{}{"type": "string"}},
					"description": "(close) commands/result/output_ref for verification evidence.",
				},
				"waivers": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"gate_id": map[string]interface{}{"type": "string"}, "reason": map[string]interface{}{"type": "string"}},
						"required":   []string{"gate_id", "reason"},
					},
					"description": "(close) Explicit waiver objects with gate_id and reason.",
				},
				"method_ref": map[string]interface{}{
					"type":        "string",
					"description": "(detail) Built-in method id.",
				},
				"method_id": map[string]interface{}{
					"type":        "string",
					"description": "(detail) Alias for method_ref.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "(status) Max open runs, default 10.",
				},
			},
			"required": []string{"action"},
		},
	}
}
