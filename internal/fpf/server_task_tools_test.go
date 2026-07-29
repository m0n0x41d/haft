package fpf

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestHandleToolsCallRoutesTaskLevelArgumentsBeforeV5(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		tool      string
		configure func(*Server, MemoryToolHandler)
	}{
		{
			name: "onboard",
			tool: "haft_onboard",
			configure: func(server *Server, handler MemoryToolHandler) {
				server.SetOnboardHandler(handler)
			},
		},
		{
			name: "entity",
			tool: "haft_entity",
			configure: func(server *Server, handler MemoryToolHandler) {
				server.SetEntityHandler(handler)
			},
		},
	} {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
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
			rawArguments := json.RawMessage(
				`{"action":"status","action":"duplicate-preserved"}`,
			)
			var received json.RawMessage
			fixture.configure(server, func(
				_ context.Context,
				arguments json.RawMessage,
			) (string, error) {
				received = append(json.RawMessage(nil), arguments...)
				return "task-ok", nil
			})
			params := make([]byte, 0, len(rawArguments)+64)
			params = append(
				params,
				`{"name":"`+fixture.tool+`","arguments":`...,
			)
			params = append(params, rawArguments...)
			params = append(params, '}')

			result := captureToolsCallResult(t, server, JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/call",
				ID:      "req-" + fixture.name,
				Params:  params,
			})
			if !bytes.Equal(received, rawArguments) {
				t.Fatalf(
					"%s arguments changed\n got: %s\nwant: %s",
					fixture.tool,
					received,
					rawArguments,
				)
			}
			if v5Calls != 0 {
				t.Fatalf(
					"v5 handler called %d times for %s",
					v5Calls,
					fixture.tool,
				)
			}
			if result.IsError ||
				len(result.Content) != 1 ||
				result.Content[0].Text != "task-ok" {
				t.Fatalf("%s result = %#v", fixture.tool, result)
			}
		})
	}
}

func TestHandleToolsCallTaskLevelSurfaceWithoutHandlerFailsClosed(
	t *testing.T,
) {
	for _, tool := range []string{"haft_onboard", "haft_entity"} {
		tool := tool
		t.Run(tool, func(t *testing.T) {
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
			result := captureToolsCallResult(t, server, JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/call",
				ID:      "req-" + tool + "-missing",
				Params: json.RawMessage(
					`{"name":"` + tool + `","arguments":{"action":"status"}}`,
				),
			})
			if !result.IsError || len(result.Content) != 1 {
				t.Fatalf("%s missing-handler result = %#v", tool, result)
			}
			if v5Calls != 0 {
				t.Fatalf(
					"v5 handler called %d times for missing %s",
					v5Calls,
					tool,
				)
			}
		})
	}
}
