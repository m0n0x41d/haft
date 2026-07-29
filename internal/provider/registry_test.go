package provider

import "testing"

func TestRegistryLookup_Exact(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	m, ok := r.Lookup("gpt-5.4")
	if !ok {
		t.Fatal("gpt-5.4 not found in registry")
	}
	if m.ContextWindow != 1_050_000 {
		t.Errorf("gpt-5.4 context window: got %d, want 1050000", m.ContextWindow)
	}
	if !m.CanReason {
		t.Error("gpt-5.4 should support reasoning")
	}
}

func TestRegistryLookup_Prefix(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	// "gpt-5.4-turbo" should match "gpt-5.4" prefix
	m, ok := r.Lookup("gpt-5.4-turbo")
	if !ok {
		t.Fatal("gpt-5.4-turbo should prefix-match gpt-5.4")
	}
	if m.ID != "gpt-5.4" {
		t.Errorf("got %s, want gpt-5.4", m.ID)
	}
}

func TestRegistryLookup_Unknown(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	_, ok := r.Lookup("totally-unknown-model")
	if ok {
		t.Error("unknown model should not be found")
	}
}

func TestContextWindow_Known(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	cases := []struct {
		model string
		want  int
	}{
		{"gpt-5.4", 1_050_000},
		{"gpt-4o", 128_000},
		{"claude-opus-4-20250514", 200_000},
		{"gemini-2.5-pro", 1_000_000},
		{"o4-mini", 200_000},
	}
	for _, tc := range cases {
		if got := r.ContextWindow(tc.model); got != tc.want {
			t.Errorf("ContextWindow(%s): got %d, want %d", tc.model, got, tc.want)
		}
	}
}

func TestContextWindow_Unknown(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	if got := r.ContextWindow("unknown"); got != 128_000 {
		t.Errorf("unknown model should default to 128k, got %d", got)
	}
}

func TestMergeProviders(t *testing.T) {
	embedded := []ProviderInfo{
		{ID: "openai", Name: "OpenAI", Models: []ModelInfo{{ID: "gpt-4o", ContextWindow: 128_000}}},
		{ID: "local", Name: "Local", Models: []ModelInfo{{ID: "llama", ContextWindow: 8_000}}},
	}
	remote := []ProviderInfo{
		{ID: "openai", Name: "OpenAI (updated)", Models: []ModelInfo{{ID: "gpt-4o", ContextWindow: 256_000}}},
	}

	merged := MergeProviders(remote, embedded)
	r := NewRegistry(merged)

	// Remote should win for openai
	m, _ := r.Lookup("gpt-4o")
	if m.ContextWindow != 256_000 {
		t.Errorf("remote should override embedded: got %d, want 256000", m.ContextWindow)
	}

	// Embedded-only provider should survive
	_, ok := r.Lookup("llama")
	if !ok {
		t.Error("embedded-only provider should survive merge")
	}
}

func TestFormatModelList(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())
	out := r.FormatModelList("gpt-5.4")
	if out == "" {
		t.Error("FormatModelList should return non-empty for gpt-5.4 filter")
	}
}

func TestMiniMaxRegistry(t *testing.T) {
	r := NewRegistry(EmbeddedProviders())

	cases := []struct {
		id      string
		context int
		in      float64
		out     float64
		cacheR  float64
		cacheW  float64
		video   bool
	}{
		{id: "MiniMax-M3", context: 1_000_000, in: 0.6, out: 2.4, cacheR: 0.12, video: true},
		{id: "MiniMax-M2.7", context: 204_800, in: 0.3, out: 1.2, cacheR: 0.06, cacheW: 0.375},
	}
	for _, tc := range cases {
		m, ok := r.Lookup(tc.id)
		if !ok {
			t.Fatalf("%s not found in registry", tc.id)
		}
		if m.ContextWindow != tc.context {
			t.Errorf("%s context window: got %d, want %d", tc.id, m.ContextWindow, tc.context)
		}
		if m.CostPer1MIn != tc.in {
			t.Errorf("%s input price: got %v, want %v", tc.id, m.CostPer1MIn, tc.in)
		}
		if m.CostPer1MOut != tc.out {
			t.Errorf("%s output price: got %v, want %v", tc.id, m.CostPer1MOut, tc.out)
		}
		if m.CostPer1MCacheRead != tc.cacheR {
			t.Errorf("%s cache-read price: got %v, want %v", tc.id, m.CostPer1MCacheRead, tc.cacheR)
		}
		if m.CostPer1MCacheWrite != tc.cacheW {
			t.Errorf("%s cache-write price: got %v, want %v", tc.id, m.CostPer1MCacheWrite, tc.cacheW)
		}
		if m.CanReason == false {
			t.Errorf("%s should support reasoning", tc.id)
		}
		if m.SupportsVideo != tc.video {
			t.Errorf("%s video support: got %t, want %t", tc.id, m.SupportsVideo, tc.video)
		}
	}

	provider, ok := findProvider(EmbeddedProviders(), "minimax")
	if !ok {
		t.Fatal("MiniMax provider not found")
	}
	if provider.APIType != "openai" {
		t.Errorf("MiniMax default API type: got %q, want openai", provider.APIType)
	}
	if len(provider.Protocols) != 2 || provider.Protocols[0] != "openai" || provider.Protocols[1] != "anthropic" {
		t.Errorf("MiniMax protocols: got %#v", provider.Protocols)
	}
	if len(provider.Endpoints) != 2 {
		t.Fatalf("MiniMax endpoints: got %d, want 2", len(provider.Endpoints))
	}
	if endpoint, ok := MiniMaxEndpoint("cn_zh"); !ok || endpoint.OpenAIBaseURL != "https://api.minimaxi.com/v1" || endpoint.AnthropicBaseURL != "https://api.minimaxi.com/anthropic" {
		t.Errorf("China endpoint: got %#v, ok=%t", endpoint, ok)
	}

	merged := MergeProviders(
		[]ProviderInfo{{ID: "minimax", Models: []ModelInfo{{ID: "MiniMax-M3"}}}},
		EmbeddedProviders(),
	)
	mergedProvider, ok := findProvider(merged, "minimax")
	if !ok || len(mergedProvider.Endpoints) != 2 {
		t.Fatalf("merged MiniMax metadata: provider=%#v, ok=%t", mergedProvider, ok)
	}
	if _, ok := NewRegistry(merged).Lookup("MiniMax-M2.7"); !ok {
		t.Fatal("embedded MiniMax model should survive remote registry merge")
	}
}

func findProvider(providers []ProviderInfo, id string) (ProviderInfo, bool) {
	for _, provider := range providers {
		if provider.ID == id {
			return provider, true
		}
	}
	return ProviderInfo{}, false
}
