package provider

import (
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
)

func TestBuildResponseParamsMovesSystemPromptToInstructions(t *testing.T) {
	messages := []agent.Message{
		{
			Role:      agent.RoleSystem,
			SessionID: "ses-123",
			Parts:     []agent.Part{agent.TextPart{Text: "system prompt"}},
		},
		{
			Role:      agent.RoleUser,
			SessionID: "ses-123",
			Parts:     []agent.Part{agent.TextPart{Text: "hello"}},
		},
	}
	tools := []agent.ToolSchema{
		{
			Name:        "read_file",
			Description: "Read a file",
			Parameters: map[string]any{
				"type": "object",
			},
		},
	}

	params := buildResponseParams("gpt-5.4", "ses-123", "api_key", messages, tools)
	body, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload["instructions"] != "system prompt" {
		t.Fatalf("expected instructions to carry system prompt, got %#v", payload["instructions"])
	}
	if payload["prompt_cache_key"] != "ses-123" {
		t.Fatalf("expected prompt_cache_key to use session id, got %#v", payload["prompt_cache_key"])
	}
	if payload["parallel_tool_calls"] != true {
		t.Fatalf("expected parallel_tool_calls=true, got %#v", payload["parallel_tool_calls"])
	}
	if payload["store"] != false {
		t.Fatalf("expected store=false, got %#v", payload["store"])
	}
	if payload["tool_choice"] != "auto" {
		t.Fatalf("expected tool_choice=auto, got %#v", payload["tool_choice"])
	}
	// truncation parameter removed — not supported by Codex backend
	if _, hasTruncation := payload["truncation"]; hasTruncation {
		t.Fatalf("truncation should not be set (unsupported by Codex backend), got %#v", payload["truncation"])
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("expected reasoning object, got %#v", payload["reasoning"])
	}
	if reasoning["effort"] != "medium" {
		t.Fatalf("expected reasoning.effort=medium, got %#v", reasoning["effort"])
	}
	if reasoning["summary"] != "auto" {
		t.Fatalf("expected reasoning.summary=auto, got %#v", reasoning["summary"])
	}
	include, ok := payload["include"].([]any)
	if !ok || len(include) != 1 {
		t.Fatalf("expected one include entry, got %#v", payload["include"])
	}
	if include[0] != "reasoning.encrypted_content" {
		t.Fatalf("expected reasoning encrypted include, got %#v", include[0])
	}

	input, ok := payload["input"].([]any)
	if !ok {
		t.Fatalf("expected input array, got %#v", payload["input"])
	}
	if len(input) != 1 {
		t.Fatalf("expected one input item after removing system prompt, got %d", len(input))
	}

	item, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("expected input item object, got %#v", input[0])
	}
	if item["role"] != "user" {
		t.Fatalf("expected remaining input role=user, got %#v", item["role"])
	}
	content, ok := item["content"].([]any)
	if !ok {
		t.Fatalf("expected typed content array, got %#v", item["content"])
	}
	if len(content) != 1 {
		t.Fatalf("expected single content item, got %d", len(content))
	}
	contentItem, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected content object, got %#v", content[0])
	}
	if contentItem["type"] != "input_text" {
		t.Fatalf("expected input_text content item, got %#v", contentItem["type"])
	}
	if contentItem["text"] != "hello" {
		t.Fatalf("expected content text=hello, got %#v", contentItem["text"])
	}
}

func TestBuildResponseParamsIncludesImageParts(t *testing.T) {
	messages := []agent.Message{
		{
			Role:      agent.RoleUser,
			SessionID: "ses-img",
			Parts: []agent.Part{
				agent.TextPart{Text: "look"},
				agent.ImagePart{Filename: "paste.png", MIMEType: "image/png", Data: []byte("pngdata")},
			},
		},
	}

	params := buildResponseParams("gpt-4o", "ses-img", "api_key", messages, nil)
	body, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	input := payload["input"].([]any)
	item := input[0].(map[string]any)
	content := item["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected text + image content, got %d", len(content))
	}
	if content[1].(map[string]any)["type"] != "input_image" {
		t.Fatalf("expected second content item to be input_image, got %#v", content[1])
	}
}

func TestExtractSessionIDUsesFirstPersistedMessage(t *testing.T) {
	sessionID := extractSessionID([]agent.Message{
		{Role: agent.RoleSystem},
		{Role: agent.RoleUser, SessionID: "ses-abc"},
		{Role: agent.RoleAssistant, SessionID: "ses-def"},
	})

	if sessionID != "ses-abc" {
		t.Fatalf("expected first non-empty session id, got %q", sessionID)
	}
}

func TestDebugRequestPayloadRedactsAccountIDAndAddsStream(t *testing.T) {
	provider := &OpenAIProvider{
		model:     "gpt-5.4",
		accountID: "acct-123",
		authType:  "codex_cli",
	}
	params := buildResponseParams("gpt-5.4", "thread-123", "api_key", []agent.Message{
		{
			Role:      agent.RoleUser,
			SessionID: "thread-123",
			Parts:     []agent.Part{agent.TextPart{Text: "hello"}},
		},
	}, nil)

	payload := provider.debugRequestPayload("thread-123", params)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal debug payload: %v", err)
	}

	headers, ok := decoded["headers"].(map[string]any)
	if !ok {
		t.Fatalf("expected headers object, got %#v", decoded["headers"])
	}
	if headers["chatgpt-account-id"] != "[redacted]" {
		t.Fatalf("expected redacted account id, got %#v", headers["chatgpt-account-id"])
	}
	body, ok := decoded["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected body object, got %#v", decoded["body"])
	}
	if body["stream"] != true {
		t.Fatalf("expected debug payload to include stream=true, got %#v", body["stream"])
	}
}
