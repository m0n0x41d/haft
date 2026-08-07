package embedding

import (
	"errors"
	"fmt"
	"strings"
)

// Provider identifiers selectable through Config.Provider.
const (
	ProviderLocal  = "local"  // local EmbeddingGemma sidecar (default)
	ProviderOpenAI = "openai" // OpenAI embeddings API
	ProviderNone   = "none"   // explicitly disabled
)

// ErrDisabled signals embeddings are turned off by configuration. Callers MUST
// degrade to FTS5+PPR recall on this error — it is not a failure.
var ErrDisabled = errors.New("embedding provider disabled")

// Config selects and parameterizes an embedding adapter. It is the single
// extension point: new providers (HTTP API backends, alternate local models)
// add a case to New and fields here, without touching the Embedder port or any
// consumer. Future fields (API base URL, key env var, batch size) slot in here.
type Config struct {
	// Provider: local (default) | openai | none.
	Provider string
	// Model id passed to the adapter (provider-specific; empty = adapter default).
	Model string
	// CacheDir for local model weights (local adapter only; empty = ~/.haft/models).
	CacheDir string
	// Dim requests an MRL-truncated dimension (0 = the model's native width).
	Dim int
}

// New builds the configured embedding adapter, or a sentinel error the caller
// degrades on:
//   - ErrDisabled            when Provider == none
//   - ErrSidecarUnavailable  when the local sidecar binary is not installed
//
// Either sentinel means "run FTS5+PPR only" — never a hard recall failure.
func New(cfg Config) (Embedder, error) {
	provider, err := normalizeProvider(cfg.Provider)
	if err != nil {
		return nil, err
	}

	switch provider {
	case ProviderLocal:
		return newSidecarAdapter(cfg)
	case ProviderOpenAI:
		return newOpenAIAdapter(cfg)
	case ProviderNone:
		return nil, ErrDisabled
	default:
		return nil, fmt.Errorf("embedding provider %q has no adapter", provider)
	}
}

// Degraded reports whether err is a benign "no embedder, use FTS5+PPR" signal
// rather than a real fault. Consumers wrap New with this to decide whether to
// silently fall back or surface the error.
func Degraded(err error) bool {
	return errors.Is(err, ErrDisabled) || errors.Is(err, ErrSidecarUnavailable)
}

// ValidateProvider checks the provider selector without constructing an
// adapter. It lets diagnostics validate configuration without requiring local
// model binaries or remote credentials.
func ValidateProvider(provider string) error {
	_, err := normalizeProvider(provider)
	return err
}

func normalizeProvider(provider string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	switch normalized {
	case "", ProviderLocal:
		return ProviderLocal, nil
	case ProviderOpenAI, ProviderNone:
		return normalized, nil
	default:
		return "", fmt.Errorf("unknown embedding provider %q", provider)
	}
}
