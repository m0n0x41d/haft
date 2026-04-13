package fpf

import (
	"bytes"
	"fmt"
	"os"
)

// SpecIndexCorpus keeps the enriched parse result separate from the production
// indexed subset so callers can share one corpus-construction path.
type SpecIndexCorpus struct {
	Chunks  []SpecChunk
	Indexed []SpecChunk
}

// BuildSpecIndexCorpus applies the production catalog -> chunk -> enrich ->
// filter pipeline to one markdown payload.
func BuildSpecIndexCorpus(markdown []byte) (SpecIndexCorpus, error) {
	catalogReader := bytes.NewReader(markdown)
	catalog, err := ParseSpecCatalog(catalogReader)
	if err != nil {
		return SpecIndexCorpus{}, fmt.Errorf("parse spec catalog: %w", err)
	}

	chunkReader := bytes.NewReader(markdown)
	chunks, err := ChunkMarkdown(chunkReader)
	if err != nil {
		return SpecIndexCorpus{}, fmt.Errorf("chunk markdown: %w", err)
	}

	chunks = EnrichChunks(chunks, catalog)

	return SpecIndexCorpus{
		Chunks:  chunks,
		Indexed: FilterIndexChunks(chunks),
	}, nil
}

// LoadSpecIndexCorpus reads a spec file and applies the production index
// corpus pipeline used by the shipped indexer.
func LoadSpecIndexCorpus(path string) (SpecIndexCorpus, error) {
	markdown, err := os.ReadFile(path)
	if err != nil {
		return SpecIndexCorpus{}, fmt.Errorf("read spec markdown: %w", err)
	}

	return BuildSpecIndexCorpus(markdown)
}
