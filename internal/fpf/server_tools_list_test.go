package fpf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestToolsListReturnsEveryToolExactlyOnce(t *testing.T) {
	server := mustCatalogServer()
	pages := mustToolsListResponsePagesForServer(t, server)
	if len(pages) != 1 {
		t.Fatalf("tools/list returned %d pages, want one atomic catalog", len(pages))
	}

	gotNames := make([]string, 0, len(server.ToolCatalog()))
	seen := make(map[string]struct{})
	for _, tool := range pages[0].tools {
		name, _ := tool["name"].(string)
		if _, duplicate := seen[name]; duplicate {
			t.Fatalf("tool %q repeated in atomic catalog", name)
		}
		seen[name] = struct{}{}
		gotNames = append(gotNames, name)
	}

	wantNames := catalogToolNames(server)
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("tools/list tools = %#v, want catalog order %#v", gotNames, wantNames)
	}
	if pages[0].nextCursor != "" {
		t.Fatalf("tools/list returned nextCursor %q, want none", pages[0].nextCursor)
	}
}

func TestToolsListAtomicResponseIsByteDeterministic(t *testing.T) {
	server := mustCatalogServer()
	first := mustToolsListResponsePagesForServer(t, server)
	second := mustToolsListResponsePagesForServer(t, server)
	if !bytes.Equal(first[0].responseBytes, second[0].responseBytes) {
		t.Fatal("atomic tools/list response changed between identical calls")
	}
}

// Grok and other MCP hosts send tools/list params beyond an empty object.
// None of those params may hide tools: every request receives the same full
// catalog, without requiring the host to understand or follow nextCursor.
func TestToolsListIgnoresClientCompatParamsAndReturnsFullCatalog(t *testing.T) {
	server := mustCatalogServer()
	wantToolCount := len(server.ToolCatalog())
	for _, params := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`null`),
		json.RawMessage(`{"cursor":null}`),
		json.RawMessage(`{"cursor":""}`),
		json.RawMessage(`{"cursor":"stale-client-cursor"}`),
		json.RawMessage(`{"cursor":7}`),
		json.RawMessage(`{"cursor":"first","cursor":"second"}`),
		json.RawMessage(`{"_meta":{"progressToken":1}}`),
		json.RawMessage(`{"unknown":"value"}`),
	} {
		responseBytes := captureToolsListResponse(t, server, JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "tools/list",
			ID:      "req-compat-full-catalog",
			Params:  params,
		})
		response := struct {
			Error  *RPCError `json:"error"`
			Result struct {
				Tools      []map[string]interface{} `json:"tools"`
				NextCursor string                   `json:"nextCursor"`
			} `json:"result"`
		}{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("params %s: unmarshal: %v\n%s", params, err, responseBytes)
		}
		if response.Error != nil {
			t.Fatalf("params %s: error %v, want full-catalog success", params, response.Error)
		}
		if len(response.Result.Tools) != wantToolCount {
			t.Fatalf(
				"params %s: got %d tools, want complete catalog of %d",
				params,
				len(response.Result.Tools),
				wantToolCount,
			)
		}
		if response.Result.NextCursor != "" {
			t.Fatalf("params %s: unexpected nextCursor %q", params, response.Result.NextCursor)
		}
	}
}

func TestToolsListAlwaysAdvertisesOnboardingAndMemoryRecoverySurfaces(
	t *testing.T,
) {
	withoutMemory := NewServer("test")
	withoutMemory.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	withoutNames := catalogToolNames(withoutMemory)
	for _, name := range []string{
		"haft_query",
		"haft_onboard",
		"haft_entity",
		"haft_memory",
	} {
		if !slices.Contains(withoutNames, name) {
			t.Fatalf(
				"%s disappeared before enablement: %#v",
				name,
				withoutNames,
			)
		}
	}

	withNames := catalogToolNames(mustCatalogServer())
	if !slices.Equal(withoutNames, withNames) {
		t.Fatalf(
			"handler availability changed tools/list\nwithout=%#v\nwith=%#v",
			withoutNames,
			withNames,
		)
	}
}

func TestToolsListMemoryEnvelopeKeepsExactBranchesAndTypedArrays(
	t *testing.T,
) {
	schema := mustListToolInputSchema(t, "haft_memory")
	variants := memoryRequestVariantsByAction(
		t,
		schema,
		"tools/list haft_memory",
	)
	wantRequired := map[string][]string{
		typedmemorywire.ActionValidate: {
			"action",
			"basis",
			"change_set",
			"contract_version",
		},
		typedmemorywire.ActionAdmit: {
			"action",
			"authority_class",
			"basis",
			"change_set",
			"contract_version",
			"idempotency_key",
			"request_provenance_ref",
		},
	}
	for action, want := range wantRequired {
		variant, present := variants[action]
		if !present {
			t.Fatalf("tools/list haft_memory omitted %s branch", action)
		}
		got := schemaStringSet(
			t,
			variant["required"],
			"tools/list haft_memory."+action+".required",
		)
		if !slices.Equal(got, want) {
			t.Fatalf(
				"tools/list haft_memory %s required = %#v, want %#v",
				action,
				got,
				want,
			)
		}
		assertNoUntypedArraySchemas(
			t,
			variant,
			"tools/list haft_memory."+action,
		)
	}
}

func TestToolsListNeverAdvertisesUntypedArrays(t *testing.T) {
	server := mustCatalogServer()
	pages := mustToolsListResponsePagesForServer(t, server)
	if len(pages) != 1 {
		t.Fatalf(
			"tools/list returned %d pages, want one atomic catalog",
			len(pages),
		)
	}
	for _, tool := range pages[0].tools {
		name, _ := tool["name"].(string)
		assertNoAnyArraySchemas(
			t,
			tool["inputSchema"],
			"tools/list "+name,
		)
	}
}

func assertNoAnyArraySchemas(
	t *testing.T,
	raw interface{},
	path string,
) {
	t.Helper()
	switch value := raw.(type) {
	case map[string]interface{}:
		if value["type"] == "array" {
			items, ok := value["items"].(map[string]interface{})
			if !ok || len(items) == 0 {
				t.Fatalf(
					"%s degraded to an unconstrained any[]: %#v",
					path,
					value,
				)
			}
		}
		for name, child := range value {
			assertNoAnyArraySchemas(t, child, path+"."+name)
		}
	case []interface{}:
		for index, child := range value {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			assertNoAnyArraySchemas(t, child, childPath)
		}
	}
}

func assertNoUntypedArraySchemas(
	t *testing.T,
	raw interface{},
	path string,
) {
	t.Helper()
	switch value := raw.(type) {
	case map[string]interface{}:
		if value["type"] == "array" {
			items, ok := value["items"].(map[string]interface{})
			if !ok || len(items) == 0 {
				t.Fatalf("%s degraded to an unconstrained any[]: %#v", path, value)
			}
			_, typedItems := items["type"]
			_, variantItems := items["oneOf"]
			if !typedItems && !variantItems {
				t.Fatalf(
					"%s array items lost type/variant constraints: %#v",
					path,
					items,
				)
			}
		}
		for name, child := range value {
			assertNoUntypedArraySchemas(t, child, path+"."+name)
		}
	case []interface{}:
		for index, child := range value {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			assertNoUntypedArraySchemas(t, child, childPath)
		}
	}
}

func catalogToolNames(server *Server) []string {
	tools := server.ToolCatalog()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func captureToolsListResponse(
	t *testing.T,
	server *Server,
	request JSONRPCRequest,
) []byte {
	t.Helper()

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
		_ = reader.Close()
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
