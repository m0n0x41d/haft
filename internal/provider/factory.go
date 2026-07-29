package provider

import "fmt"

// ProviderOptions configures a provider adapter without changing the legacy
// NewProvider call shape.
type ProviderOptions struct {
	APIType          string
	Region           string
	OpenAIBaseURL    string
	AnthropicBaseURL string
}

// NewProvider creates an LLM provider based on provider ID.
// Routes to the appropriate implementation:
//   - "openai": OpenAI Responses API (also handles Codex/ChatGPT auth)
//   - "anthropic": Anthropic Messages API
//   - "minimax": OpenAI-compatible by default, or Anthropic-compatible with options
//
// For OpenAI, apiKey can be empty — it resolves from env/config/codex.
// For Anthropic, apiKey is required (from env or config).
func NewProvider(providerID, model, apiKey string) (LLMProvider, error) {
	return NewProviderWithOptions(providerID, model, apiKey, ProviderOptions{})
}

// NewProviderWithOptions creates a provider with protocol and endpoint
// selection for compatible providers.
func NewProviderWithOptions(providerID, model, apiKey string, options ProviderOptions) (LLMProvider, error) {
	switch providerID {
	case "openai":
		return NewOpenAI(model)
	case "anthropic":
		return NewAnthropic(model, apiKey)
	case "minimax":
		endpoint, ok := MiniMaxEndpoint(options.Region)
		if !ok {
			return nil, fmt.Errorf("unknown MiniMax region %q", options.Region)
		}
		if options.APIType == "anthropic" {
			baseURL := options.AnthropicBaseURL
			if baseURL == "" {
				baseURL = endpoint.AnthropicBaseURL
			}
			return NewAnthropicWithBaseURL(model, apiKey, baseURL)
		}
		if options.APIType != "" && options.APIType != "openai" {
			return nil, fmt.Errorf("unsupported MiniMax API type %q", options.APIType)
		}
		baseURL := options.OpenAIBaseURL
		if baseURL == "" {
			baseURL = endpoint.OpenAIBaseURL
		}
		return NewOpenAICompatible(model, apiKey, baseURL)
	default:
		return nil, fmt.Errorf("provider %q not yet supported — use openai or anthropic", providerID)
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
	prefixes := map[string]string{
		"gpt-":      "openai",
		"o1":        "openai",
		"o3":        "openai",
		"o4":        "openai",
		"claude-":   "anthropic",
		"gemini-":   "google",
		"deepseek-": "deepseek",
		"llama-":    "groq",
		"MiniMax-":  "minimax",
	}
	for prefix, provider := range prefixes {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return provider
		}
	}
	return "openai" // default
}
