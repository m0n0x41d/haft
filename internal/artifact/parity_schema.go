package artifact

// ParityPlanJSONSchema returns the JSON Schema fragment describing the
// structured parity plan parameter. Used by both the standalone tool surface
// (internal/tools/haft.go) and the MCP-advertised schema
// (internal/fpf/server.go) so the two transports stay in lock-step.
//
// Mirrors the ParityPlan struct + MissingDataPolicy* constants and the
// FPF G.9:4.2 ParityPlan contract. Required for deep-mode comparison;
// optional for standard/tactical modes.
//
// The description argument is per-call so each surface can phrase the
// usage hint in a way that matches its own action labels (compare vs
// characterize, REQUIRED vs accepted, etc.).
func ParityPlanJSONSchema(description string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": description,
		"properties": map[string]any{
			"baseline_set": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Variant IDs that share comparable baseline conditions (e.g., same cohort, same dataset version). Required for deep mode.",
			},
			"window": map[string]any{
				"type":        "string",
				"description": "Time / observation window across which scores are comparable (e.g., '2026-Q2 production', 'last 14 days'). Required for deep mode.",
			},
			"budget": map[string]any{
				"type":        "string",
				"description": "Resource budget assumed equal across variants (e.g., '4 GPU-hours', 'p95 latency target 200ms'). Required for deep mode.",
			},
			"missing_data_policy": map[string]any{
				"type": "string",
				"enum": []string{
					MissingDataPolicyExplicitAbstain,
					MissingDataPolicyZero,
					MissingDataPolicyExclude,
				},
				"description": "How to treat missing scores: explicit_abstain (preserve gap, flag in output), zero (treat absence as 0), exclude (drop the variant). Required for deep mode.",
			},
			"normalization": map[string]any{
				"type":        "array",
				"description": "Per-dimension normalization rules to compare across heterogeneous units.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dimension": map[string]any{"type": "string", "description": "Dimension name being normalized"},
						"method":    map[string]any{"type": "string", "description": "Normalization method (e.g., 'min-max', 'z-score', 'rank')"},
					},
				},
			},
			"pinned_conditions": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Conditions that must hold for the comparison to be valid (e.g., 'same load profile', 'identical hardware').",
			},
		},
	}
}
