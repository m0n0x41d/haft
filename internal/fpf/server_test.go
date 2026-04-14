package fpf

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func mustListToolProperties(t *testing.T, toolName string) map[string]interface{} {
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

		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s properties missing or wrong type: %#v", toolName, inputSchema["properties"])
		}

		return properties
	}

	t.Fatalf("%s tool schema not found", toolName)
	return nil
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

	description, _ := selectedRef["description"].(string)
	if description != "(compare) Advisory recommendation variant ID; the human still chooses" {
		t.Fatalf("unexpected selected_ref description: %q", description)
	}
}

func TestHandleToolsList_ProblemSchemaIncludesProblemType(t *testing.T) {
	problemSchema := mustListToolProperties(t, "haft_problem")

	problemType, ok := problemSchema["problem_type"].(map[string]interface{})
	if !ok {
		t.Fatalf("problem_type schema missing or wrong type: %#v", problemSchema["problem_type"])
	}

	description, _ := problemType["description"].(string)
	if !strings.Contains(description, "optimization") {
		t.Fatalf("unexpected problem_type description: %q", description)
	}
}

func TestHandleToolsList_DecisionSchemaMarksValidUntilForEvidence(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	validUntil, ok := decisionSchema["valid_until"].(map[string]interface{})
	if !ok {
		t.Fatalf("valid_until schema missing or wrong type: %#v", decisionSchema["valid_until"])
	}

	description, _ := validUntil["description"].(string)
	if description != "(decide/evidence) Expiry date (RFC3339 or YYYY-MM-DD)" {
		t.Fatalf("unexpected valid_until description: %q", description)
	}

	for _, key := range []string{"predictions", "claim_refs", "claim_scope"} {
		if _, ok := decisionSchema[key]; !ok {
			t.Fatalf("expected decision schema to expose %q", key)
		}
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

func TestHandleToolsList_FPFQuerySchemaIncludesMode(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	mode, ok := querySchema["mode"].(map[string]interface{})
	if !ok {
		t.Fatalf("mode schema missing or wrong type: %#v", querySchema["mode"])
	}

	description, _ := mode["description"].(string)
	if description != "(fpf) Experimental retrieval mode; currently supports tree" {
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

func TestHandleToolsList_QuerySchemaIncludesProjectionView(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	view, ok := querySchema["view"].(map[string]interface{})
	if !ok {
		t.Fatalf("view schema missing or wrong type: %#v", querySchema["view"])
	}

	description, _ := view["description"].(string)
	if description != "(projection) engineer | manager | audit | compare | delegated-agent | change-rationale" {
		t.Fatalf("unexpected view description: %q", description)
	}
}
