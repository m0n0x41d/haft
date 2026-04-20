package fpf

import "context"

// Defaults exposed for surface layers (CLI flags) and provider-bound
// implementations to reference the canonical semantic-embedding contract.
// Concrete provider-bound implementations live in internal/embedding to keep
// the FPF Core layer free of provider/agent imports.
const (
	DefaultSemanticEmbeddingProvider   = "openai"
	DefaultSemanticEmbeddingModel      = "text-embedding-3-small"
	DefaultSemanticEmbeddingDimensions = 256
)

// SemanticEmbedderDescriptor identifies the provider/model contract behind an
// experimental embedding artifact.
type SemanticEmbedderDescriptor struct {
	Provider   string
	Model      string
	Dimensions int
}

// SemanticEmbedder produces embedding vectors for semantic artifact build and
// query-time scoring. Concrete implementations live in higher layers
// (e.g. internal/embedding for provider-bound implementations).
type SemanticEmbedder interface {
	Descriptor() SemanticEmbedderDescriptor
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}
