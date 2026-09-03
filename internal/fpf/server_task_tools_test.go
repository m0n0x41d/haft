package fpf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestHandleToolsCallRoutesTaskLevelArgumentsBeforeV5(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		tool      string
		arguments json.RawMessage
		configure func(*Server, MemoryToolHandler)
	}{
		{
			name: "onboard",
			tool: "haft_onboard",
			arguments: json.RawMessage(
				`{"action":"status"}`,
			),
			configure: func(server *Server, handler MemoryToolHandler) {
				server.SetOnboardHandler(handler)
			},
		},
		{
			name: "entity",
			tool: "haft_entity",
			arguments: json.RawMessage(
				`{"action":"status","action":"duplicate-preserved"}`,
			),
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
			rawArguments := fixture.arguments
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

func TestHandleToolsCallRejectsRemovedOnboardMemoryChoiceBeforeHandler(
	t *testing.T,
) {
	for _, fixture := range []struct {
		name      string
		arguments string
		want      string
	}{
		{
			name:      "historical memory prepare",
			arguments: `{"action":"memory_prepare"}`,
			want: "Invalid haft_onboard request: " +
				"action must be status, profile_prepare, or profile_change_prepare",
		},
		{
			name:      "unknown",
			arguments: `{"action":"not_an_onboard_action"}`,
			want: "Invalid haft_onboard request: " +
				"action must be status, profile_prepare, or profile_change_prepare",
		},
		{
			name:      "duplicate action",
			arguments: `{"action":"status","action":"profile_prepare"}`,
			want: "Invalid haft_onboard request: " +
				"action field is duplicated",
		},
	} {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			server := NewServer("test")
			handlerCalls := 0
			server.SetOnboardHandler(func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				handlerCalls++
				return "unexpected-handler", nil
			})
			result := captureOnboardToolsCallThroughJSONRPC(
				t,
				server,
				"req-onboard-invalid",
				json.RawMessage(fixture.arguments),
			)
			if !result.IsError || len(result.Content) != 1 ||
				result.Content[0].Text != fixture.want {
				t.Fatalf("invalid onboard action result = %#v", result)
			}
			if handlerCalls != 0 {
				t.Fatalf(
					"invalid onboard action reached handler %d times",
					handlerCalls,
				)
			}
		})
	}
}

func TestHaftOnboardEveryAdvertisedActionCrossesJSONRPCDispatcher(
	t *testing.T,
) {
	schema := mustListToolInputSchema(t, "haft_onboard")
	properties := mustSchemaProperties(t, schema, "haft_onboard")
	actions := mustStringEnum(t, properties["action"], "haft_onboard.action")

	for action := range actions {
		action := action
		t.Run(action, func(t *testing.T) {
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
			arguments := json.RawMessage(`{"action":"` + action + `"}`)
			if action == haftOnboardProfileChangePrepareAction {
				arguments = json.RawMessage(
					`{"action":"profile_change_prepare","scope_id":"app","entity_ref":"entity:target"}`,
				)
			}
			handlerCalls := 0
			var received json.RawMessage
			server.SetOnboardHandler(func(
				_ context.Context,
				raw json.RawMessage,
			) (string, error) {
				handlerCalls++
				received = append(json.RawMessage(nil), raw...)
				return "onboard-ok", nil
			})

			result := captureOnboardToolsCallThroughJSONRPC(
				t,
				server,
				"req-onboard-"+action,
				arguments,
			)
			if result.IsError || len(result.Content) != 1 ||
				result.Content[0].Text != "onboard-ok" {
				t.Fatalf("advertised action result = %#v", result)
			}
			if handlerCalls != 1 || v5Calls != 0 {
				t.Fatalf(
					"handler calls = onboard:%d v5:%d",
					handlerCalls,
					v5Calls,
				)
			}
			if !bytes.Equal(received, arguments) {
				t.Fatalf(
					"advertised action arguments changed\n got: %s\nwant: %s",
					received,
					arguments,
				)
			}
		})
	}
}

func TestHaftOnboardProfileChangePrepareCrossesJSONRPCDispatcher(
	t *testing.T,
) {
	server := NewServer("test")
	arguments := json.RawMessage(
		`{"action":"profile_change_prepare","scope_id":"app","entity_ref":"entity:target"}`,
	)
	responses := []string{
		`{"action":"profile_change_prepare","result":"profile_change_review_prepared","effects":{"canonical_profile_changed":false,"authority_granted":false}}`,
		`{"action":"profile_change_prepare","result":"profile_change_review_reused","effects":{"canonical_profile_changed":false,"authority_granted":false}}`,
	}
	handlerCalls := 0
	server.SetOnboardHandler(func(
		_ context.Context,
		raw json.RawMessage,
	) (string, error) {
		if !bytes.Equal(raw, arguments) {
			t.Fatalf(
				"profile_change_prepare arguments changed\n got: %s\nwant: %s",
				raw,
				arguments,
			)
		}
		response := responses[handlerCalls]
		handlerCalls++
		return response, nil
	})

	for index, want := range responses {
		result := captureOnboardToolsCallThroughJSONRPC(
			t,
			server,
			"req-profile-change-prepare",
			arguments,
		)
		if result.IsError || len(result.Content) != 1 ||
			result.Content[0].Text != want {
			t.Fatalf("call %d result = %#v", index+1, result)
		}
	}
	if handlerCalls != len(responses) {
		t.Fatalf("profile_change_prepare handler calls = %d", handlerCalls)
	}
}

func captureOnboardToolsCallThroughJSONRPC(
	t *testing.T,
	server *Server,
	id string,
	arguments json.RawMessage,
) CallToolResult {
	t.Helper()

	params := make([]byte, 0, len(arguments)+64)
	params = append(params, `{"name":"haft_onboard","arguments":`...)
	params = append(params, arguments...)
	params = append(params, '}')
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      id,
		Params:  params,
	}

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
	server.handleRequest(request)
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
		t.Fatalf(
			"unmarshal haft_onboard tools/call response: %v\n%s",
			err,
			responseBytes,
		)
	}
	return response.Result
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
