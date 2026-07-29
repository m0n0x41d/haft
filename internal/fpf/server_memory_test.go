package fpf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestHandleToolsCallRoutesRawMemoryArgumentsBeforeV5(t *testing.T) {
	server := NewServer("test")
	v5Calls := 0
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		v5Calls++
		return "unexpected-v5", nil
	})

	rawArguments := json.RawMessage(`{
  "contract_version": "haft.memory.v1",
  "contract_version": "duplicate-must-survive",
  "action": "validate",
  "basis": { "kind": "project_current" },
  "change_set": { "changes": [] }
}`)
	var received json.RawMessage
	server.SetMemoryHandler(func(_ context.Context, arguments json.RawMessage) (string, error) {
		received = append(json.RawMessage(nil), arguments...)
		return "memory-ok", nil
	})

	params := make([]byte, 0, len(rawArguments)+80)
	params = append(
		params,
		`{"name":"haft_memory","arguments":{"request":`...,
	)
	params = append(params, rawArguments...)
	params = append(params, '}', '}')
	result := captureToolsCallResult(t, server, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "req-memory-raw",
		Params:  params,
	})

	if !bytes.Equal(received, rawArguments) {
		t.Fatalf("memory arguments changed\n got: %s\nwant: %s", received, rawArguments)
	}
	if v5Calls != 0 {
		t.Fatalf("v5 handler called %d times for haft_memory", v5Calls)
	}
	if result.IsError {
		t.Fatalf("haft_memory returned error: %#v", result)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "memory-ok" {
		t.Fatalf("haft_memory result = %#v", result)
	}
}

func TestHandleToolsCallMemoryWithoutHandlerFailsClosedBeforeV5(t *testing.T) {
	server := NewServer("test")
	v5Calls := 0
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		v5Calls++
		return "unexpected-v5", nil
	})

	result := captureToolsCallResult(t, server, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "req-memory-unavailable",
		Params:  json.RawMessage(`{"name":"haft_memory","arguments":{}}`),
	})

	if v5Calls != 0 {
		t.Fatalf("v5 handler called %d times for unavailable haft_memory", v5Calls)
	}
	if !result.IsError {
		t.Fatalf("unavailable haft_memory result = %#v, want error", result)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "Haft memory validation is unavailable" {
		t.Fatalf("unavailable haft_memory result = %#v", result)
	}
}

func TestHandleToolsCallRejectsMemoryActionOutsideConfiguredSurface(
	t *testing.T,
) {
	tests := []struct {
		name      string
		configure func(*Server, MemoryToolHandler)
		action    string
	}{
		{
			name: "validate surface rejects resolve",
			configure: func(server *Server, handler MemoryToolHandler) {
				server.SetMemoryHandler(handler)
			},
			action: typedmemorywire.ActionResolve,
		},
		{
			name: "validate-admit surface rejects resolve",
			configure: func(server *Server, handler MemoryToolHandler) {
				server.SetMemoryFullHandler(handler)
			},
			action: typedmemorywire.ActionResolve,
		},
		{
			name: "invalid internal surface rejects every action",
			configure: func(server *Server, handler MemoryToolHandler) {
				server.memoryHandler = handler
				server.memorySurface = memorySurfaceUnavailable
			},
			action: typedmemorywire.ActionValidate,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer("test")
			handlerCalls := 0
			handler := func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				handlerCalls++
				return "must-not-run", nil
			}
			test.configure(server, handler)

			result := captureToolsCallResult(
				t,
				server,
				memoryToolCallRequest(t, test.action),
			)
			if !result.IsError ||
				len(result.Content) != 1 ||
				!strings.Contains(
					result.Content[0].Text,
					"unavailable on the configured surface",
				) {
				t.Fatalf("unadvertised action result = %#v", result)
			}
			if handlerCalls != 0 {
				t.Fatalf("unadvertised action reached handler %d time(s)", handlerCalls)
			}
		})
	}
}

func TestHandleToolsCallFullSurfaceAllowsExactlyValidateAndAdmit(
	t *testing.T,
) {
	server := NewServer("test")
	received := make([]string, 0, 2)
	server.SetMemoryFullHandler(func(
		_ context.Context,
		arguments json.RawMessage,
	) (string, error) {
		action, err := decodeMemoryToolAction(arguments)
		if err != nil {
			return "", err
		}
		received = append(received, action)
		return "full-ok", nil
	})

	for _, action := range memorySurfaceActions(memorySurfaceFull) {
		result := captureToolsCallResult(
			t,
			server,
			memoryToolCallRequest(t, action),
		)
		if result.IsError ||
			len(result.Content) != 1 ||
			result.Content[0].Text != "full-ok" {
			t.Fatalf("full action %q result = %#v", action, result)
		}
	}
	if len(received) != 2 {
		t.Fatalf("full handler received actions = %#v", received)
	}
}

func TestHandleToolsCallRoutesRawMemoryQueryBeforeV5(t *testing.T) {
	server := NewServer("test")
	v5Calls := 0
	server.SetV5Handler(func(
		context.Context,
		string,
		json.RawMessage,
	) (string, error) {
		v5Calls++
		return "unexpected-v5", nil
	})

	rawArguments := json.RawMessage(`{
  "action":"memory",
  "memory_request":{
    "contract_version":"haft.memory.v1",
    "mode":"resolve",
    "mode":"duplicate-must-survive"
  }
}`)
	var received json.RawMessage
	server.SetMemoryReadHandler(func(
		_ context.Context,
		arguments json.RawMessage,
	) (string, error) {
		received = append(json.RawMessage(nil), arguments...)
		return "query-memory-ok", nil
	})

	params := make([]byte, 0, len(rawArguments)+64)
	params = append(params, `{"name":"haft_query","arguments":`...)
	params = append(params, rawArguments...)
	params = append(params, '}')
	result := captureToolsCallResult(t, server, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "req-memory-query-raw",
		Params:  params,
	})

	if !bytes.Equal(received, rawArguments) {
		t.Fatalf(
			"memory query arguments changed\n got: %s\nwant: %s",
			received,
			rawArguments,
		)
	}
	if v5Calls != 0 {
		t.Fatalf("v5 handler called %d times for memory query", v5Calls)
	}
	if result.IsError ||
		len(result.Content) != 1 ||
		result.Content[0].Text != "query-memory-ok" {
		t.Fatalf("memory query result = %#v", result)
	}
}

func TestHandleToolsCallRejectsAmbiguousMemoryActionBeforeHandler(
	t *testing.T,
) {
	server := NewServer("test")
	handlerCalls := 0
	server.SetMemoryFullHandler(func(
		context.Context,
		json.RawMessage,
	) (string, error) {
		handlerCalls++
		return "must-not-run", nil
	})
	result := captureToolsCallResult(t, server, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "req-memory-duplicate-action",
		Params: json.RawMessage(
			`{"name":"haft_memory","arguments":{"request":{` +
				`"action":"validate","action":"admit"}}}`,
		),
	})
	if !result.IsError ||
		len(result.Content) != 1 ||
		!strings.Contains(result.Content[0].Text, "action field is duplicated") {
		t.Fatalf("ambiguous action result = %#v", result)
	}
	if handlerCalls != 0 {
		t.Fatalf("ambiguous action reached handler %d time(s)", handlerCalls)
	}
}

func TestHandleToolsCallRejectsInvalidMemoryEnvelopeBeforeHandler(
	t *testing.T,
) {
	tests := []struct {
		name      string
		arguments string
		wantError string
	}{
		{
			name:      "legacy flat arguments",
			arguments: `{"action":"validate"}`,
			wantError: `arguments field "action" is not allowed`,
		},
		{
			name:      "missing request",
			arguments: `{}`,
			wantError: "request is required",
		},
		{
			name: "unknown sibling",
			arguments: `{"request":{"action":"validate"},` +
				`"extra":true}`,
			wantError: `arguments field "extra" is not allowed`,
		},
		{
			name: "duplicate request",
			arguments: `{"request":{"action":"validate"},` +
				`"request":{"action":"admit"}}`,
			wantError: "request field is duplicated",
		},
		{
			name:      "request is not object",
			arguments: `{"request":"validate"}`,
			wantError: "arguments must be a JSON object",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer("test")
			handlerCalls := 0
			server.SetMemoryFullHandler(func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				handlerCalls++
				return "must-not-run", nil
			})
			params := `{"name":"haft_memory","arguments":` +
				test.arguments +
				`}`
			result := captureToolsCallResult(t, server, JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/call",
				ID:      "req-invalid-memory-envelope",
				Params:  json.RawMessage(params),
			})
			if !result.IsError ||
				len(result.Content) != 1 ||
				!strings.Contains(result.Content[0].Text, test.wantError) {
				t.Fatalf(
					"invalid envelope result = %#v, want %q",
					result,
					test.wantError,
				)
			}
			if handlerCalls != 0 {
				t.Fatalf(
					"invalid envelope reached handler %d time(s)",
					handlerCalls,
				)
			}
		})
	}
}

func memoryToolCallRequest(
	t *testing.T,
	action string,
) JSONRPCRequest {
	t.Helper()
	arguments, err := json.Marshal(map[string]string{"action": action})
	if err != nil {
		t.Fatal(err)
	}
	params := make([]byte, 0, len(arguments)+64)
	params = append(
		params,
		`{"name":"haft_memory","arguments":{"request":`...,
	)
	params = append(params, arguments...)
	params = append(params, '}', '}')
	return JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      "req-memory-action-" + action,
		Params:  params,
	}
}

func captureToolsCallResult(
	t *testing.T,
	server *Server,
	request JSONRPCRequest,
) CallToolResult {
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
	server.handleToolsCall(request)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	response := struct {
		Result CallToolResult `json:"result"`
	}{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal tools/call response: %v\n%s", err, responseBytes)
	}
	return response.Result
}
