package provider

import "fmt"

// NewProvider creates an LLM provider based on provider ID.
// Routes to the appropriate implementation:
//   - "openai": OpenAI Responses API (also handles Codex/ChatGPT auth)
//   - "anthropic": Anthropic Messages API
//   - "claudecode": Claude Code CLI (uses Max/Pro subscription, no API key)
//   - Others: treated as OpenAI-compatible (DeepSeek, Groq, Mistral, etc.)
//
// For OpenAI, apiKey can be empty — it resolves from env/config/codex.
// For Anthropic, apiKey is required (from env or config).
// For claudecode, apiKey is ignored — auth is owned by the `claude` CLI.
func NewProvider(providerID, model, apiKey string) (LLMProvider, error) {
	switch providerID {
	case "openai":
		return NewOpenAI(model)
	case "anthropic":
		return NewAnthropic(model, apiKey)
	case "claudecode":
		return NewClaudeCode(model)
	default:
		// OpenAI-compatible providers (DeepSeek, Groq, etc.)
		// For now, route through OpenAI — they use the same API format.
		// TODO: support custom base URLs for non-OpenAI providers.
		return nil, fmt.Errorf("provider %q not yet supported — use openai, anthropic, or claudecode", providerID)
	}
}

// ProviderIDForModel guesses the provider from a model name.
// Used when --model flag is set without explicit provider.
func ProviderIDForModel(model string) string {
	// Check registry first
	reg := DefaultRegistry()
	if m, ok := reg.Lookup(model); ok {
		// Find which provider this model belongs to
		for _, p := range reg.Providers() {
			for _, pm := range p.Models {
				if pm.ID == m.ID {
					return p.ID
				}
			}
		}
	}

	// Fallback: prefix heuristic
	return guessProviderFromPrefix(model)
}

func guessProviderFromPrefix(model string) string {
	// Ordered list so longer prefixes win (e.g. "claude-code" beats "claude-").
	type entry struct{ prefix, provider string }
	prefixes := []entry{
		{"claude-code", "claudecode"},
		{"claude-", "anthropic"},
		{"gpt-", "openai"},
		{"o1", "openai"},
		{"o3", "openai"},
		{"o4", "openai"},
		{"gemini-", "google"},
		{"deepseek-", "deepseek"},
		{"llama-", "groq"},
	}
	for _, e := range prefixes {
		if len(model) >= len(e.prefix) && model[:len(e.prefix)] == e.prefix {
			return e.provider
		}
	}
	return "openai" // default
}
