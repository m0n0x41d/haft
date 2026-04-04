package fpf

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestHandleToolsList_CompareSchemaIncludesNarrativeFields(t *testing.T) {
	server := NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      "req-1",
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

	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
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

	var compareSchema map[string]interface{}
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]interface{})
		if !ok {
			t.Fatalf("tool entry has wrong type: %#v", rawTool)
		}
		if tool["name"] != "haft_solution" {
			continue
		}

		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Fatalf("haft_solution inputSchema missing or wrong type: %#v", tool["inputSchema"])
		}

		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("haft_solution properties missing or wrong type: %#v", inputSchema["properties"])
		}
		compareSchema = properties
		break
	}

	if compareSchema == nil {
		t.Fatal("haft_solution tool schema not found")
	}

	for _, key := range []string{"dominated_variants", "pareto_tradeoffs", "recommendation_rationale"} {
		if _, ok := compareSchema[key]; !ok {
			t.Fatalf("expected compare schema to expose %q", key)
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
