// Package embedding provides concrete embedding-model implementations used by
// the optional FPF semantic-search prototype. The abstract SemanticEmbedder
// interface lives in internal/fpf (Core layer); this package owns the
// provider-bound implementations so the Core layer stays free of provider /
// agent / flow imports.
package embedding

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/provider"
)

// Re-exported defaults so Surface layer (CLI flags) can stay decoupled from
// the openai SDK package while still referencing the canonical model name.
const (
	DefaultProvider   = fpf.DefaultSemanticEmbeddingProvider
	DefaultModel      = fpf.DefaultSemanticEmbeddingModel
	DefaultDimensions = fpf.DefaultSemanticEmbeddingDimensions
)

const defaultBatchSize = 64

type openAIEmbeddingClient interface {
	CreateEmbeddings(ctx context.Context, model string, dimensions int, texts []string) ([][]float32, error)
}

type openAISemanticEmbedder struct {
	client     openAIEmbeddingClient
	descriptor fpf.SemanticEmbedderDescriptor
	batchSize  int
}

type openAIEmbeddingClientAdapter struct {
	client openai.Client
}

// NewOpenAI creates the explicit embedding model used by the optional
// semantic FPF prototype. Resolves the OpenAI API key via the provider
// package; defaults model + dimensions when unset.
func NewOpenAI(model string, dimensions int) (fpf.SemanticEmbedder, error) {
	resolvedModel := strings.TrimSpace(model)
	if resolvedModel == "" {
		resolvedModel = DefaultModel
	}
	if dimensions <= 0 {
		dimensions = DefaultDimensions
	}

	apiKey, err := provider.ResolveOpenAIAPIKey()
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))
	return newOpenAIWithClient(openAIEmbeddingClientAdapter{client: client}, resolvedModel, dimensions), nil
}

func newOpenAIWithClient(
	client openAIEmbeddingClient,
	model string,
	dimensions int,
) fpf.SemanticEmbedder {
	return openAISemanticEmbedder{
		client: client,
		descriptor: fpf.SemanticEmbedderDescriptor{
			Provider:   DefaultProvider,
			Model:      model,
			Dimensions: dimensions,
		},
		batchSize: defaultBatchSize,
	}
}

func (e openAISemanticEmbedder) Descriptor() fpf.SemanticEmbedderDescriptor {
	return e.descriptor
}

func (e openAISemanticEmbedder) EmbedTexts(
	ctx context.Context,
	texts []string,
) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	batchSize := e.batchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := min(start+batchSize, len(texts))

		batchVectors, err := e.client.CreateEmbeddings(
			ctx,
			e.descriptor.Model,
			e.descriptor.Dimensions,
			texts[start:end],
		)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batchVectors...)
	}

	return vectors, nil
}

func (a openAIEmbeddingClientAdapter) CreateEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
	texts []string,
) ([][]float32, error) {
	response, err := a.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model:          model,
		Dimensions:     openai.Int(int64(dimensions)),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embeddings: %w", err)
	}

	indexed := make([][]float32, len(response.Data))
	for _, item := range response.Data {
		if item.Index < 0 || int(item.Index) >= len(indexed) {
			return nil, fmt.Errorf("openai embeddings returned out-of-range index %d", item.Index)
		}
		indexed[item.Index] = float64SliceToFloat32(item.Embedding)
	}

	for index, vector := range indexed {
		if len(vector) == 0 {
			return nil, fmt.Errorf("openai embeddings missing vector at index %d", index)
		}
	}

	return indexed, nil
}

func float64SliceToFloat32(values []float64) []float32 {
	converted := make([]float32, 0, len(values))
	for _, value := range values {
		converted = append(converted, float32(value))
	}
	return converted
}
