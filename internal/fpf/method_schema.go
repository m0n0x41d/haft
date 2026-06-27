package fpf

func haftMethodTool() Tool {
	return Tool{
		Name:        "haft_method",
		Description: "MethodRun pull/close/read/catalog.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"pull", "close", "show", "detail", "status", "catalog"},
					"description": "pull, close, show, detail, status, catalog.",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Task.",
				},
				"declared_task_kind": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Task kind.",
				},
				"change_intent": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Change intent.",
				},
				"intended_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(pull) Expected files.",
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
					"description": "(pull) Risk ids.",
				},
				"user_scope_constraints": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(pull) Scope limits.",
				},
				"artifact_refs": map[string]interface{}{
					"type":        "object",
					"description": "(pull) Artifact refs.",
				},
				"ceremony_request": map[string]interface{}{
					"type":        "string",
					"description": "(pull) none|low|medium|deep.",
				},
				"response_budget": map[string]interface{}{
					"type":        "object",
					"description": "(pull) Response budget.",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "(pull) Context.",
				},
				"carry_through": map[string]interface{}{
					"type":        "array",
					"items":       methodCarryThroughItemSchema(),
					"description": "(pull/close) Accepted items to dispose.",
				},
				"pull_id": map[string]interface{}{
					"type":        "string",
					"description": "(close/show) mpull id.",
				},
				"changed_files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "(close) Changed files.",
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
					"description": "(close) Gate results.",
				},
				"verification": map[string]interface{}{
					"type":        "object",
					"properties":  map[string]interface{}{"commands": map[string]interface{}{}, "result": map[string]interface{}{}, "output_ref": map[string]interface{}{}},
					"description": "(close) Verification.",
				},
				"waivers": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"gate_id": map[string]interface{}{}, "reason": map[string]interface{}{}},
					},
					"description": "(close) Waivers.",
				},
				"method_ref": map[string]interface{}{
					"type":        "string",
					"description": "(detail) Method id.",
				},
				"method_status": map[string]interface{}{
					"description": "(catalog) current | experimental | superseded | deprecated | all.",
				},
				"limit": map[string]interface{}{
					"description": "(status) Max open runs, default 10.",
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
			"source_ref":      map[string]interface{}{},
			"source_item_ref": map[string]interface{}{},
			"acceptance_ref":  map[string]interface{}{},
			"disposition":     map[string]interface{}{"enum": []interface{}{"pending", "applied", "rejected", "deferred", "superseded"}},
			"target_refs":     map[string]interface{}{},
			"evidence_refs":   map[string]interface{}{},
			"reason":          map[string]interface{}{},
		},
		"required": []string{"source_ref", "source_item_ref", "acceptance_ref"},
	}
}
