package fpf

import (
	"context"
	"encoding/json"
	"io"
	"os"
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
