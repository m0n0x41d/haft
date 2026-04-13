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
