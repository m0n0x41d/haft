package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	openaioption "github.com/openai/openai-go/v3/option"
)

func TestMiniMaxProviderFactory(t *testing.T) {
	openAIProvider, err := NewProviderWithOptions(
		"minimax",
		"MiniMax-M3",
		"test-key",
		ProviderOptions{Region: "global_en"},
	)
	if err != nil {
		t.Fatalf("create MiniMax OpenAI-compatible provider: %v", err)
	}
	if _, ok := openAIProvider.(*OpenAIProvider); !ok {
		t.Fatalf("MiniMax default provider type: got %T, want *OpenAIProvider", openAIProvider)
	}
	if openAIProvider.ModelID() != "MiniMax-M3" {
		t.Fatalf("MiniMax OpenAI-compatible model: got %q", openAIProvider.ModelID())
	}

	anthropicProvider, err := NewProviderWithOptions(
		"minimax",
		"MiniMax-M2.7",
		"test-key",
		ProviderOptions{APIType: "anthropic", Region: "cn_zh"},
	)
	if err != nil {
		t.Fatalf("create MiniMax Anthropic-compatible provider: %v", err)
	}
	if _, ok := anthropicProvider.(*AnthropicProvider); !ok {
		t.Fatalf("MiniMax Anthropic-compatible provider type: got %T, want *AnthropicProvider", anthropicProvider)
	}
	if anthropicProvider.ModelID() != "MiniMax-M2.7" {
		t.Fatalf("MiniMax Anthropic-compatible model: got %q", anthropicProvider.ModelID())
	}
}

func TestMiniMaxOpenAIEndpointPath(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","created_at":1,"status":"completed","model":"MiniMax-M3","output":[]}`))
	}))
	defer server.Close()

	provider, err := newOpenAIProvider(
		"MiniMax-M3",
		"test-key",
		"api_key",
		"",
		server.URL+"/v1",
		openaioption.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("create OpenAI-compatible provider: %v", err)
	}
	_, _ = provider.client.Responses.New(context.Background(), buildResponseParams("MiniMax-M3", "", "api_key", nil, nil))
	if path != "/v1/responses" {
		t.Fatalf("OpenAI-compatible request path: got %q, want /v1/responses", path)
	}
}

func TestMiniMaxAnthropicEndpointPath(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider, err := newAnthropicProvider(
		"MiniMax-M2.7",
		"test-key",
		server.URL+"/anthropic",
		option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("create Anthropic-compatible provider: %v", err)
	}
	_, _ = provider.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "MiniMax-M2.7",
		MaxTokens: 1,
	})
	if path != "/anthropic/v1/messages" {
		t.Fatalf("Anthropic-compatible request path: got %q, want /anthropic/v1/messages", path)
	}
}
