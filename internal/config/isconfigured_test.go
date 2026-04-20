package config

import "testing"

func TestIsConfiguredClaudeCodeNeedsNoCredentials(t *testing.T) {
	cfg := &Config{Model: "claude-code"}
	if !cfg.IsConfigured() {
		t.Fatalf("claude-code model should be enough to be configured")
	}
}

func TestIsConfiguredClaudeCodeSubModel(t *testing.T) {
	cfg := &Config{Model: "claude-code:sonnet"}
	if !cfg.IsConfigured() {
		t.Fatalf("claude-code:sonnet should be configured without creds")
	}
}

func TestIsConfiguredAnthropicStillRequiresKey(t *testing.T) {
	cfg := &Config{Model: "claude-opus-4-20250514"}
	if cfg.IsConfigured() {
		t.Fatalf("anthropic model without creds should NOT be configured")
	}
	cfg.SetAuth("anthropic", ProviderAuth{APIKey: "sk-test"})
	if !cfg.IsConfigured() {
		t.Fatalf("anthropic with api key should be configured")
	}
}

func TestIsConfiguredEmptyModelStillFails(t *testing.T) {
	cfg := &Config{}
	if cfg.IsConfigured() {
		t.Fatalf("empty model should never be configured")
	}
}
