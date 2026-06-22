package fpf

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func mustToolsListResponseBytes(t *testing.T) []byte {
	t.Helper()

	server := NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      "req-schema-budget",
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
	}()

	os.Stdout = writer
	server.handleToolsList(request)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	return responseBytes
}

func mustListToolProperties(t *testing.T, toolName string) map[string]interface{} {
	t.Helper()

	inputSchema := mustListToolInputSchema(t, toolName)
	properties, ok := inputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties missing or wrong type: %#v", toolName, inputSchema["properties"])
	}

	return properties
}

func mustSchemaProperties(t *testing.T, raw interface{}, label string) map[string]interface{} {
	t.Helper()

	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema missing or wrong type: %#v", label, raw)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties missing or wrong type: %#v", label, schema["properties"])
	}
	return properties
}

func mustArrayItemProperties(t *testing.T, raw interface{}, label string) map[string]interface{} {
	t.Helper()

	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema missing or wrong type: %#v", label, raw)
	}
	items, ok := schema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s items missing or wrong type: %#v", label, schema["items"])
	}
	properties, ok := items["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s item properties missing or wrong type: %#v", label, items["properties"])
	}
	return properties
}

func mustListToolInputSchema(t *testing.T, toolName string) map[string]interface{} {
	t.Helper()

	server := NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      "req-schema",
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
	}()

	os.Stdout = writer
	server.handleToolsList(request)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	response := map[string]interface{}{}
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		t.Fatalf("unmarshal tools/list response: %v\n%s", err, string(responseBytes))
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("tools missing or wrong type: %#v", result["tools"])
	}

	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]interface{})
		if !ok {
			t.Fatalf("tool entry has wrong type: %#v", rawTool)
		}
		if tool["name"] != toolName {
			continue
		}

		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s inputSchema missing or wrong type: %#v", toolName, tool["inputSchema"])
		}

		return inputSchema
	}

	t.Fatalf("%s tool schema not found", toolName)
	return nil
}

func TestHandleToolsList_StaysUnderContextBudget(t *testing.T) {
	responseBytes := mustToolsListResponseBytes(t)

	const maxToolsListBytes = 15000
	if len(responseBytes) > maxToolsListBytes {
		t.Fatalf("tools/list response = %d bytes, want <= %d", len(responseBytes), maxToolsListBytes)
	}
}

func TestHandleToolsList_MethodSchemaExposesPullAndClose(t *testing.T) {
	methodSchema := mustListToolProperties(t, "haft_method")

	action, ok := methodSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_method action schema missing: %#v", methodSchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_method action enum missing: %#v", action["enum"])
	}
	for _, want := range []string{"pull", "close", "show", "detail", "status"} {
		if !schemaEnumContains(enum, want) {
			t.Fatalf("haft_method action enum = %#v, missing %q", enum, want)
		}
	}
	for _, key := range []string{"task", "declared_task_kind", "change_intent", "intended_files", "risk_signals", "pull_id", "gate_results", "verification", "waivers"} {
		if _, ok := methodSchema[key]; !ok {
			t.Fatalf("haft_method schema missing %q", key)
		}
	}

	gateResults, ok := methodSchema["gate_results"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results schema missing or wrong type: %#v", methodSchema["gate_results"])
	}
	gateItems, ok := gateResults["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results.items missing or wrong type: %#v", gateResults["items"])
	}
	gateProperties, ok := gateItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results.items.properties missing or wrong type: %#v", gateItems["properties"])
	}
	for _, key := range []string{"gate_id", "status", "evidence_refs", "waiver_reason"} {
		if _, ok := gateProperties[key]; !ok {
			t.Fatalf("gate_results item schema missing %q", key)
		}
	}

	waivers, ok := methodSchema["waivers"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers schema missing or wrong type: %#v", methodSchema["waivers"])
	}
	waiverItems, ok := waivers["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers.items missing or wrong type: %#v", waivers["items"])
	}
	waiverProperties, ok := waiverItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers.items.properties missing or wrong type: %#v", waiverItems["properties"])
	}
	for _, key := range []string{"gate_id", "reason"} {
		if _, ok := waiverProperties[key]; !ok {
			t.Fatalf("waiver item schema missing %q", key)
		}
	}
}

func TestHandleToolsList_RefreshSchemaIncludesReview(t *testing.T) {
	refreshSchema := mustListToolProperties(t, "haft_refresh")

	action, ok := refreshSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_refresh action schema missing: %#v", refreshSchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_refresh action enum missing: %#v", action["enum"])
	}
	for _, want := range []string{"scan", "plan", "review", "drain", "waive", "reopen", "supersede", "deprecate", "reconcile"} {
		if !schemaEnumContains(enum, want) {
			t.Fatalf("haft_refresh action enum = %#v, missing %q", enum, want)
		}
	}
	if _, ok := refreshSchema["dry_run"].(map[string]interface{}); !ok {
		t.Fatalf("haft_refresh schema missing dry_run field: %#v", refreshSchema)
	}
}

func TestHandleToolsList_QuerySchemaIncludesOperationalGate(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")
	properties := mustSchemaProperties(t, querySchema["operational_gate"], "operational_gate")
	for _, key := range []string{"schema_version", "gate_ref", "bearer_ref", "use_context", "rule", "evidence_refs", "expires_at", "reopen_condition"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("operational_gate.%s schema missing or wrong type: %#v", key, properties[key])
		}
	}
}

func TestHandleToolsList_QuerySchemaIncludesContractAudit(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	action, ok := querySchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_query action schema missing: %#v", querySchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_query action enum missing: %#v", action["enum"])
	}
	if !schemaEnumContains(enum, "contract_audit") {
		t.Fatalf("haft_query action enum missing contract_audit: %#v", enum)
	}
	if !schemaEnumContains(enum, "contract_generation") {
		t.Fatalf("haft_query action enum missing contract_generation: %#v", enum)
	}
}

func TestHandleToolsList_AdvertisesNativePiTools(t *testing.T) {
	for _, name := range []string{"haft_method", "haft_commission", "haft_spec_section"} {
		t.Run(name, func(t *testing.T) {
			if schema := mustListToolInputSchema(t, name); schema["type"] != "object" {
				t.Fatalf("%s input schema type = %#v, want object", name, schema["type"])
			}
		})
	}
}

func TestHandleToolsList_CompareSchemaIncludesNarrativeFields(t *testing.T) {
	compareSchema := mustListToolProperties(t, "haft_solution")

	for _, key := range []string{"dominated_variants", "pareto_tradeoffs", "recommendation_rationale"} {
		if _, ok := compareSchema[key]; !ok {
			t.Fatalf("expected compare schema to expose %q", key)
		}
	}

	dominatedVariants, ok := compareSchema["dominated_variants"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants schema missing or wrong type: %#v", compareSchema["dominated_variants"])
	}

	dominatedItems, ok := dominatedVariants["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants items missing or wrong type: %#v", dominatedVariants["items"])
	}

	dominatedProperties, ok := dominatedItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants properties missing or wrong type: %#v", dominatedItems["properties"])
	}

	for _, key := range []string{"variant", "dominated_by", "summary"} {
		if _, ok := dominatedProperties[key]; !ok {
			t.Fatalf("expected dominated_variants item schema to expose %q", key)
		}
	}

	paretoTradeoffs, ok := compareSchema["pareto_tradeoffs"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs schema missing or wrong type: %#v", compareSchema["pareto_tradeoffs"])
	}

	paretoItems, ok := paretoTradeoffs["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs items missing or wrong type: %#v", paretoTradeoffs["items"])
	}

	paretoProperties, ok := paretoItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs properties missing or wrong type: %#v", paretoItems["properties"])
	}

	for _, key := range []string{"variant", "summary"} {
		if _, ok := paretoProperties[key]; !ok {
			t.Fatalf("expected pareto_tradeoffs item schema to expose %q", key)
		}
	}

	selectedRef, ok := compareSchema["selected_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf("selected_ref schema missing or wrong type: %#v", compareSchema["selected_ref"])
	}
	if selectedRef["type"] != "string" {
		t.Fatalf("selected_ref type = %#v, want string", selectedRef["type"])
	}

	legacyRef, ok := compareSchema["legacy_recommendation_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf("legacy_recommendation_ref schema missing or wrong type: %#v", compareSchema["legacy_recommendation_ref"])
	}

	legacyDescription, _ := legacyRef["description"].(string)
	if !strings.Contains(legacyDescription, "Advisory recommendation only") {
		t.Fatalf("unexpected legacy_recommendation_ref description: %q", legacyDescription)
	}
}

func TestHandleToolsList_CompareScoresSchemaExposesNestedMapShape(t *testing.T) {
	compareSchema := mustListToolProperties(t, "haft_solution")

	scores, ok := compareSchema["scores"].(map[string]interface{})
	if !ok {
		t.Fatalf("scores schema missing or wrong type: %#v", compareSchema["scores"])
	}

	variantScores, ok := scores["additionalProperties"].(map[string]interface{})
	if !ok {
		t.Fatalf("scores.additionalProperties missing or wrong type: %#v", scores["additionalProperties"])
	}
	if variantScores["type"] != "object" {
		t.Fatalf("scores additionalProperties type = %#v, want object", variantScores["type"])
	}

	dimensionScore, ok := variantScores["additionalProperties"].(map[string]interface{})
	if !ok {
		t.Fatalf("variant score additionalProperties missing or wrong type: %#v", variantScores["additionalProperties"])
	}
	if dimensionScore["type"] != "string" {
		t.Fatalf("dimension score type = %#v, want string", dimensionScore["type"])
	}
}

func schemaEnumContains(values []interface{}, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestHandleToolsList_ProblemSchemaIncludesProfileFields(t *testing.T) {
	problemSchema := mustListToolProperties(t, "haft_problem")

	for _, key := range []string{
		"problem_type",
		"problem_profile",
		"source_kind",
		"why_now",
		"scope",
		"acceptance_probe",
		"freshness_disposition",
	} {
		if _, ok := problemSchema[key].(map[string]interface{}); !ok {
			t.Fatalf("%s schema missing or wrong type: %#v", key, problemSchema[key])
		}
	}

	problemType := problemSchema["problem_type"].(map[string]interface{})
	description, _ := problemType["description"].(string)
	if !strings.Contains(description, "optimization") {
		t.Fatalf("unexpected problem_type description: %q", description)
	}

	problemProfile := problemSchema["problem_profile"].(map[string]interface{})
	profileEnum, ok := problemProfile["enum"].([]interface{})
	if !ok {
		t.Fatalf("problem_profile enum missing or wrong type: %#v", problemProfile["enum"])
	}
	for _, want := range []string{"cue", "thin", "deep"} {
		if !schemaEnumContains(profileEnum, want) {
			t.Fatalf("problem_profile enum = %#v, missing %q", profileEnum, want)
		}
	}

	sourceKind := problemSchema["source_kind"].(map[string]interface{})
	sourceEnum, ok := sourceKind["enum"].([]interface{})
	if !ok {
		t.Fatalf("source_kind enum missing or wrong type: %#v", sourceKind["enum"])
	}
	for _, want := range []string{"observed_problem", "wish", "ticket", "chosen_method"} {
		if !schemaEnumContains(sourceEnum, want) {
			t.Fatalf("source_kind enum = %#v, missing %q", sourceEnum, want)
		}
	}
}

func TestHandleToolsList_DecisionSchemaMarksValidUntilForEvidence(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	validUntil, ok := decisionSchema["valid_until"].(map[string]interface{})
	if !ok {
		t.Fatalf("valid_until schema missing or wrong type: %#v", decisionSchema["valid_until"])
	}

	description, _ := validUntil["description"].(string)
	if description != "Expiry date" {
		t.Fatalf("unexpected valid_until description: %q", description)
	}

	for _, key := range []string{"predictions", "claim_refs", "claim_scope"} {
		if _, ok := decisionSchema[key]; !ok {
			t.Fatalf("expected decision schema to expose %q", key)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesChoiceResult(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	choiceResult, ok := decisionSchema["choice_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result schema missing or wrong type: %#v", decisionSchema["choice_result"])
	}

	properties, ok := choiceResult["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result properties missing or wrong type: %#v", choiceResult["properties"])
	}

	nextMove, ok := properties["next_move"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result.next_move missing or wrong type: %#v", properties["next_move"])
	}
	for _, key := range []string{"subject_ref", "option_set", "comparison_basis", "choice_rule", "variant_ref", "problem_refs", "portfolio_ref", "reason", "reversibility", "reopen_condition"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("choice_result.%s missing or wrong type: %#v", key, properties[key])
		}
	}

	encoded, err := json.Marshal(nextMove["enum"])
	if err != nil {
		t.Fatalf("marshal next_move enum: %v", err)
	}
	for _, want := range []string{"choose_now", "reject_current_set", "probe_again", "reroute"} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("choice_result.next_move enum missing %q: %s", want, encoded)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesTransformationRecord(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	transformationRecord, ok := decisionSchema["transformation_record"].(map[string]interface{})
	if !ok {
		t.Fatalf("transformation_record schema missing or wrong type: %#v", decisionSchema["transformation_record"])
	}

	properties, ok := transformationRecord["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("transformation_record properties missing or wrong type: %#v", transformationRecord["properties"])
	}

	for _, key := range []string{"transformed_entity", "initial_state", "post_state", "relation", "context", "window", "method_refs", "work_refs", "evidence_refs", "publication_refs"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("transformation_record.%s schema missing or wrong type: %#v", key, properties[key])
		}
	}

}

func TestHandleToolsList_DecisionSchemaExposesScopeNestedShapes(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	implementationFootprint := mustSchemaProperties(t, decisionSchema["implementation_footprint"], "implementation_footprint")
	for _, key := range []string{"files", "commits", "work_refs"} {
		if _, ok := implementationFootprint[key].(map[string]interface{}); !ok {
			t.Fatalf("implementation_footprint.%s schema missing or wrong type: %#v", key, implementationFootprint[key])
		}
	}

	governanceTarget := mustArrayItemProperties(t, decisionSchema["governance_targets"], "governance_targets")
	for _, key := range []string{"kind", "ref", "binding_target"} {
		if _, ok := governanceTarget[key].(map[string]interface{}); !ok {
			t.Fatalf("governance_targets.%s schema missing or wrong type: %#v", key, governanceTarget[key])
		}
	}

	driftWatchTarget := mustArrayItemProperties(t, decisionSchema["drift_watch_targets"], "drift_watch_targets")
	for _, key := range []string{"target_ref", "trigger", "binding_target"} {
		if _, ok := driftWatchTarget[key].(map[string]interface{}); !ok {
			t.Fatalf("drift_watch_targets.%s schema missing or wrong type: %#v", key, driftWatchTarget[key])
		}
	}

	claim := mustArrayItemProperties(t, decisionSchema["claims"], "claims")
	for _, key := range []string{"id", "claim", "observable", "threshold", "lifecycle_status", "successor_ref", "retired_reason", "governance_target_refs"} {
		if _, ok := claim[key].(map[string]interface{}); !ok {
			t.Fatalf("claims.%s schema missing or wrong type: %#v", key, claim[key])
		}
	}
}

func TestHaftDecisionSchemaExposesTaskContext(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	taskContext, ok := decisionSchema["task_context"].(map[string]interface{})
	if !ok {
		t.Fatalf("task_context schema missing or wrong type: %#v", decisionSchema["task_context"])
	}

	description, _ := taskContext["description"].(string)
	if !strings.Contains(description, "DecisionRecord ID filename") {
		t.Fatalf("unexpected task_context description: %q", description)
	}
}

func TestHandleToolsList_DecisionSchemaExposesTacticalSkipFields(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	for _, key := range []string{"_skips", "_skip"} {
		field, ok := decisionSchema[key].(map[string]interface{})
		if !ok {
			t.Fatalf("decision schema missing %q skip field", key)
		}
		if field["type"] != "array" {
			t.Fatalf("%s type = %v, want array", key, field["type"])
		}
		items, ok := field["items"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s items = %T, want map[string]interface{}", key, field["items"])
		}
		if items["type"] != "string" {
			t.Fatalf("%s items.type = %v, want string", key, items["type"])
		}
		description, _ := field["description"].(string)
		if !strings.Contains(description, "_skip_reason") {
			t.Fatalf("%s description should mention _skip_reason: %q", key, description)
		}
	}

	reason, ok := decisionSchema["_skip_reason"].(map[string]interface{})
	if !ok {
		t.Fatalf("decision schema missing _skip_reason field")
	}
	if reason["type"] != "string" {
		t.Fatalf("_skip_reason type = %v, want string", reason["type"])
	}
}

func TestHandleToolsList_DecisionSchemaRequiresCompletePredictions(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	predictions, ok := decisionSchema["predictions"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions schema missing or wrong type: %#v", decisionSchema["predictions"])
	}

	items, ok := predictions["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("prediction items schema missing or wrong type: %#v", predictions["items"])
	}

	required, ok := items["required"].([]interface{})
	if !ok {
		t.Fatalf("prediction required fields missing or wrong type: %#v", items["required"])
	}

	got := make([]string, 0, len(required))
	for _, item := range required {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("prediction required item has wrong type: %#v", item)
		}
		got = append(got, value)
	}

	want := []string{"claim", "observable", "threshold"}
	if len(got) != len(want) {
		t.Fatalf("prediction required fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prediction required fields = %v, want %v", got, want)
		}
	}
}

func TestHandleToolsList_CommissionSchemaExposesRunnableClaimActions(t *testing.T) {
	commissionInputSchema := mustListToolInputSchema(t, "haft_commission")
	commissionSchema, ok := commissionInputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("commission properties missing or wrong type: %#v", commissionInputSchema["properties"])
	}

	action, ok := commissionSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("action schema missing or wrong type: %#v", commissionSchema["action"])
	}

	values, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("action enum missing or wrong type: %#v", action["enum"])
	}

	got := map[string]bool{}
	for _, value := range values {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("action enum value has wrong type: %#v", value)
		}
		got[name] = true
	}

	for _, want := range []string{"create", "list", "list_runnable", "show", "claim_for_preflight", "requeue", "cancel"} {
		if !got[want] {
			t.Fatalf("expected haft_commission action %q in schema enum %#v", want, values)
		}
	}
	// Per-action conditional requirements (commission_id required for
	// show/requeue/cancel; reason required for requeue/cancel) are
	// enforced at the handler boundary in internal/cli/serve_commission.go,
	// not in the MCP-advertised schema. Anthropic API rejects top-level
	// allOf/oneOf/anyOf in input_schema (validation error
	// "tools.N.custom.input_schema does not support oneOf, allOf, or
	// anyOf at the top level"), so the schema only declares "action" as
	// universally required and the handler returns an error when a
	// conditional field is missing for the given action.
	assertCommissionSchemaHasNoTopLevelCompositors(t, commissionInputSchema)

	override, ok := commissionSchema["spec_readiness_override"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec_readiness_override schema missing or wrong type: %#v", commissionSchema["spec_readiness_override"])
	}
	if override["type"] != "object" {
		t.Fatalf("spec_readiness_override type = %#v, want object", override["type"])
	}
}

// assertCommissionSchemaHasNoTopLevelCompositors guards the regression
// from 2026-04-28: a top-level allOf with conditional if/then blocks
// passed Go-side schema construction and Haft tests, but Anthropic API
// rejected it as input_schema validation error, taking the entire MCP
// server offline (`/mcp` reported `1 MCP server failed`). The handler
// at internal/cli/serve_commission.go enforces per-action requirements;
// the schema must remain free of top-level allOf / oneOf / anyOf.
func assertCommissionSchemaHasNoTopLevelCompositors(t *testing.T, schema map[string]interface{}) {
	t.Helper()
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if _, present := schema[key]; present {
			t.Fatalf("commission schema must not declare top-level %q (Anthropic API rejects it)", key)
		}
	}
}

// TestHandleToolsList_NoToolDeclaresTopLevelCompositors is the broader
// version of the same guard: iterates every advertised tool and asserts
// none of them declares top-level allOf / oneOf / anyOf in
// inputSchema. Anthropic API rejects all three at the top level
// regardless of which tool ships them, so ALL tools must comply.
func TestHandleToolsList_NoToolDeclaresTopLevelCompositors(t *testing.T) {
	server := NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      "req-no-compositors",
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdout = stdout }()
	os.Stdout = writer
	server.handleToolsList(request)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	response := map[string]any{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal tools/list: %v\n%s", err, string(responseBytes))
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %#v", response["result"])
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools missing: %#v", result["tools"])
	}
	if len(tools) == 0 {
		t.Fatalf("no tools advertised")
	}

	banned := []string{"allOf", "oneOf", "anyOf"}
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool entry has wrong type: %#v", raw)
		}
		name, _ := tool["name"].(string)
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("tool %q inputSchema missing", name)
		}
		for _, key := range banned {
			if _, present := schema[key]; present {
				t.Fatalf("tool %q declares top-level %q in inputSchema; Anthropic API rejects this and takes the whole MCP server offline", name, key)
			}
		}
	}
}

func TestHandleToolsList_FPFQuerySchemaIncludesMode(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	mode, ok := querySchema["mode"].(map[string]interface{})
	if !ok {
		t.Fatalf("mode schema missing or wrong type: %#v", querySchema["mode"])
	}

	description, _ := mode["description"].(string)
	if description != "tree mode" {
		t.Fatalf("unexpected mode description: %q", description)
	}
}

func TestHandleInitialize_IncludesWorkflowInstructionsWhenConfigured(t *testing.T) {
	server := NewServer()
	server.SetInstructions("## Project Workflow\nDefaults:\n- mode: standard")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      "req-init",
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
	}()

	os.Stdout = writer
	server.handleInitialize(request)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	response := map[string]interface{}{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal initialize response: %v\n%s", err, string(responseBytes))
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", response["result"])
	}

	instructions, _ := result["instructions"].(string)
	if !strings.Contains(instructions, "Project Workflow") {
		t.Fatalf("expected workflow instructions, got %#v", result["instructions"])
	}
}

// TestHandleToolsList_SolutionExposesParityPlan is the regression test for
// GitHub issue #62 — deep-mode haft_solution(action="compare") requires a
// structured parity plan, but the MCP-side schema in handleToolsList did not
// expose any parameter that accepts it. Without parity_plan in the schema,
// MCP clients (Claude Code, Cursor, Gemini CLI, Codex) cannot reach deep mode.
//
// The schema must expose parity_plan as an object with at minimum the four
// fields the deep-mode validator requires per FPF G.9:4.2.
func TestHandleToolsList_SolutionExposesParityPlan(t *testing.T) {
	solutionSchema := mustListToolProperties(t, "haft_solution")

	parityPlan, ok := solutionSchema["parity_plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan missing from haft_solution schema (issue #62); got: %#v", solutionSchema["parity_plan"])
	}
	if pType, _ := parityPlan["type"].(string); pType != "object" {
		t.Fatalf("parity_plan should be an object schema, got type=%q", pType)
	}
	props, ok := parityPlan["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan.properties missing or wrong type: %#v", parityPlan["properties"])
	}
	for _, key := range []string{"baseline_set", "window", "budget", "missing_data_policy"} {
		if _, ok := props[key]; !ok {
			t.Errorf("parity_plan must expose %q (required for deep-mode comparison per FPF G.9:4.2)", key)
		}
	}
}

// TestHandleToolsList_ProblemExposesParityPlan ensures characterize can
// declare a structured parity plan early, not just prose parity_rules.
// Same MCP gap as the haft_solution case but on the characterize entry point.
func TestHandleToolsList_ProblemExposesParityPlan(t *testing.T) {
	problemSchema := mustListToolProperties(t, "haft_problem")

	parityPlan, ok := problemSchema["parity_plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan missing from haft_problem schema (issue #62); got: %#v", problemSchema["parity_plan"])
	}
	props, ok := parityPlan["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan.properties missing or wrong type: %#v", parityPlan["properties"])
	}
	for _, key := range []string{"baseline_set", "window", "budget", "missing_data_policy"} {
		if _, ok := props[key]; !ok {
			t.Errorf("parity_plan on haft_problem must expose %q", key)
		}
	}
}

func TestHandleToolsList_QuerySchemaIncludesProjectionView(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	view, ok := querySchema["view"].(map[string]interface{})
	if !ok {
		t.Fatalf("view schema missing or wrong type: %#v", querySchema["view"])
	}

	description, _ := view["description"].(string)
	if description != "projection views" {
		t.Fatalf("unexpected view description: %q", description)
	}
}

func TestHandleToolsList_QuerySchemaIncludesCodeContextLane(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	lane, ok := querySchema["lane"].(map[string]interface{})
	if !ok {
		t.Fatalf("lane schema missing or wrong type: %#v", querySchema["lane"])
	}

	description, _ := lane["description"].(string)
	for _, want := range []string{"Progressive disclosure lane", "Default index", "full=true audit"} {
		if !strings.Contains(description, want) {
			t.Fatalf("lane description missing %q: %q", want, description)
		}
	}

	enum, ok := lane["enum"].([]interface{})
	if !ok {
		t.Fatalf("lane enum missing or wrong type: %#v", lane["enum"])
	}
	got := make(map[string]bool)
	for _, value := range enum {
		text, _ := value.(string)
		got[text] = true
	}
	for _, want := range []string{"index", "symbols", "decisions", "invariants", "notes", "problems", "portfolios", "all"} {
		if !got[want] {
			t.Fatalf("lane enum missing %q: %#v", want, enum)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesCausalSupportBasis(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	basis, ok := decisionSchema["causal_support_basis"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_decision schema must expose causal_support_basis: %#v", decisionSchema["causal_support_basis"])
	}
	if basis["type"] != "string" {
		t.Fatalf("causal_support_basis type = %v, want string", basis["type"])
	}
	desc, _ := basis["description"].(string)
	if !strings.Contains(desc, "C.28") || !strings.Contains(desc, "simulation_only") {
		t.Fatalf("causal_support_basis description must cite C.28 and simulation_only: %q", desc)
	}

	predictions, ok := decisionSchema["predictions"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions schema missing: %#v", decisionSchema["predictions"])
	}
	items, ok := predictions["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions.items missing: %#v", predictions["items"])
	}
	props, ok := items["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions.items.properties missing: %#v", items["properties"])
	}
	if _, ok := props["realizability"]; !ok {
		t.Fatalf("predictions[].realizability missing from haft_decision schema")
	}
}
