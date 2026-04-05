package fpf

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/m0n0x41d/haft/internal/provider"
)

const (
	DefaultSemanticEmbeddingProvider   = "openai"
	DefaultSemanticEmbeddingModel      = string(openai.EmbeddingModelTextEmbedding3Small)
	DefaultSemanticEmbeddingDimensions = 256
	defaultSemanticEmbeddingBatchSize  = 64
)

// SemanticEmbedderDescriptor identifies the provider/model contract behind an
// experimental embedding artifact.
type SemanticEmbedderDescriptor struct {
	Provider   string
	Model      string
	Dimensions int
}

// SemanticEmbedder produces embedding vectors for semantic artifact build and
// query-time scoring.
type SemanticEmbedder interface {
	Descriptor() SemanticEmbedderDescriptor
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}

type openAIEmbeddingClient interface {
	CreateEmbeddings(ctx context.Context, model string, dimensions int, texts []string) ([][]float32, error)
}

type openAISemanticEmbedder struct {
	client     openAIEmbeddingClient
	descriptor SemanticEmbedderDescriptor
	batchSize  int
}

type openAIEmbeddingClientAdapter struct {
	client openai.Client
}

// NewOpenAISemanticEmbedder creates the explicit embedding model used by the
// optional semantic FPF prototype.
func NewOpenAISemanticEmbedder(model string, dimensions int) (SemanticEmbedder, error) {
	model = firstNonEmpty(strings.TrimSpace(model), DefaultSemanticEmbeddingModel)
	if dimensions <= 0 {
		dimensions = DefaultSemanticEmbeddingDimensions
	}

	apiKey, err := provider.ResolveOpenAIAPIKey()
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))
	return newOpenAISemanticEmbedderWithClient(openAIEmbeddingClientAdapter{client: client}, model, dimensions), nil
}

func newOpenAISemanticEmbedderWithClient(
	client openAIEmbeddingClient,
	model string,
	dimensions int,
) SemanticEmbedder {
	return openAISemanticEmbedder{
		client: client,
		descriptor: SemanticEmbedderDescriptor{
			Provider:   DefaultSemanticEmbeddingProvider,
			Model:      model,
			Dimensions: dimensions,
		},
		batchSize: defaultSemanticEmbeddingBatchSize,
	}
}

func (embedder openAISemanticEmbedder) Descriptor() SemanticEmbedderDescriptor {
	return embedder.descriptor
}

func (embedder openAISemanticEmbedder) EmbedTexts(
	ctx context.Context,
	texts []string,
) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	batchSize := embedder.batchSize
	if batchSize <= 0 {
		batchSize = defaultSemanticEmbeddingBatchSize
	}

	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchVectors, err := embedder.client.CreateEmbeddings(
			ctx,
			embedder.descriptor.Model,
			embedder.descriptor.Dimensions,
			texts[start:end],
		)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batchVectors...)
	}

	return vectors, nil
}

func (client openAIEmbeddingClientAdapter) CreateEmbeddings(
	ctx context.Context,
	model string,
	dimensions int,
	texts []string,
) ([][]float32, error) {
	response, err := client.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model:          openai.EmbeddingModel(model),
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
