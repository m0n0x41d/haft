package fpf

import "testing"

func TestToolsListPreservesPrimitiveAndContainerTypeConstraints(t *testing.T) {
	note := mustListToolProperties(t, "haft_note")
	assertArrayItemType(t, note["anchors"], "haft_note.anchors", "object")

	problem := mustListToolProperties(t, "haft_problem")
	assertSchemaType(t, problem["action"], "haft_problem.action", "string")
	assertArrayItemType(t, problem["dimensions"], "haft_problem.dimensions", "object")

	solution := mustListToolProperties(t, "haft_solution")
	assertSchemaType(t, solution["action"], "haft_solution.action", "string")
	assertArrayItemType(t, solution["variants"], "haft_solution.variants", "object")
	assertSchemaType(t, solution["scores"], "haft_solution.scores", "object")
	assertArrayItemType(t, solution["dominated_variants"], "haft_solution.dominated_variants", "object")
	assertArrayItemType(t, solution["pareto_tradeoffs"], "haft_solution.pareto_tradeoffs", "object")

	decision := mustListToolProperties(t, "haft_decision")
	assertSchemaType(t, decision["action"], "haft_decision.action", "string")
	choiceResult := mustSchemaProperties(t, decision["choice_result"], "haft_decision.choice_result")
	assertSchemaType(t, choiceResult["next_move"], "haft_decision.choice_result.next_move", "string")
	assertArrayItemType(t, decision["why_not_others"], "haft_decision.why_not_others", "object")
	assertSchemaType(t, decision["rollback"], "haft_decision.rollback", "object")
	assertArrayItemType(t, decision["predictions"], "haft_decision.predictions", "object")

	refresh := mustListToolProperties(t, "haft_refresh")
	assertSchemaType(t, refresh["action"], "haft_refresh.action", "string")

	query := mustListToolProperties(t, "haft_query")
	assertSchemaType(t, query["action"], "haft_query.action", "string")
	assertSchemaType(t, query["probe"], "haft_query.probe", "object")
	assertSchemaType(t, query["variants"], "haft_query.variants", "array")
	assertSchemaType(t, query["policy"], "haft_query.policy", "string")
	assertSchemaType(t, query["operational_gate"], "haft_query.operational_gate", "object")
	assertSchemaType(t, query["lane"], "haft_query.lane", "string")
	assertSchemaType(t, query["view"], "haft_query.view", "string")
	memoryRequest, ok := query["memory_request"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"haft_query.memory_request = %#v, want closed nested variants",
			query["memory_request"],
		)
	}
	if _, variants := memoryRequest["oneOf"].([]interface{}); !variants {
		t.Fatalf(
			"haft_query.memory_request.oneOf = %#v",
			memoryRequest["oneOf"],
		)
	}

	method := mustListToolProperties(t, "haft_method")
	assertSchemaType(t, method["task"], "haft_method.task", "string")
	assertArrayItemType(t, method["intended_files"], "haft_method.intended_files", "string")

	commission := mustListToolProperties(t, "haft_commission")
	assertSchemaType(t, commission["commission"], "haft_commission.commission", "object")
	assertArrayItemType(t, commission["allowed_paths"], "haft_commission.allowed_paths", "string")

	spec := mustListToolProperties(t, "haft_spec_section")
	assertSchemaType(t, spec["section_id"], "haft_spec_section.section_id", "string")
	assertSchemaType(t, spec["scope_id"], "haft_spec_section.scope_id", "string")
}

func assertArrayItemType(t *testing.T, raw interface{}, label, itemType string) {
	t.Helper()
	schema := assertSchemaType(t, raw, label, "array")
	assertSchemaType(t, schema["items"], label+".items", itemType)
}

func assertSchemaType(
	t *testing.T,
	raw interface{},
	label string,
	want string,
) map[string]interface{} {
	t.Helper()
	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema = %#v, want object", label, raw)
	}
	if schema["type"] != want {
		t.Fatalf("%s type = %#v, want %q", label, schema["type"], want)
	}
	return schema
}
