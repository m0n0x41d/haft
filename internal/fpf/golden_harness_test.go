package fpf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func loadGoldenRoutes(t *testing.T) []Route {
	t.Helper()

	path := filepath.Join(testRepoRoot(t), ".context", "fpf-routes.json")
	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes(%q) failed: %v", path, err)
	}

	return routes
}

func loadGoldenSpecChunks(t *testing.T) []SpecChunk {
	t.Helper()

	path := filepath.Join(testRepoRoot(t), ".context", "FPF-Spec.md")
	catalogFile := mustOpenFile(t, path)
	catalog, err := ParseSpecCatalog(catalogFile)
	_ = catalogFile.Close()
	if err != nil {
		t.Fatalf("ParseSpecCatalog(%q) failed: %v", path, err)
	}

	chunkFile := mustOpenFile(t, path)
	chunks, err := ChunkMarkdown(chunkFile)
	_ = chunkFile.Close()
	if err != nil {
		t.Fatalf("ChunkMarkdown(%q) failed: %v", path, err)
	}

	chunks = EnrichChunks(chunks, catalog)
	return chunks
}

func loadGoldenSpecChunksForPatternIDs(t *testing.T, patternIDs map[string]struct{}) []SpecChunk {
	t.Helper()

	chunks := loadGoldenSpecChunks(t)
	return filterGoldenSpecChunks(chunks, patternIDs)
}

func filterGoldenSpecChunks(chunks []SpecChunk, patternIDs map[string]struct{}) []SpecChunk {
	filtered := make([]SpecChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.PatternID == "" {
			continue
		}
		if _, ok := patternIDs[chunk.PatternID]; !ok {
			continue
		}
		if len(strings.TrimSpace(chunk.Body)) <= 20 {
			continue
		}
		filtered = append(filtered, chunk)
	}

	return reindexGoldenSpecChunks(filtered)
}

func reindexGoldenSpecChunks(chunks []SpecChunk) []SpecChunk {
	reindexed := make([]SpecChunk, 0, len(chunks))
	for index, chunk := range chunks {
		chunk.ID = index
		reindexed = append(reindexed, chunk)
	}

	return reindexed
}

func collectRoutePatternIDs(routes []Route) map[string]struct{} {
	patternIDs := make(map[string]struct{})
	for _, route := range routes {
		for _, patternID := range route.Core {
			patternIDs[patternID] = struct{}{}
		}
		for _, patternID := range route.Chain {
			patternIDs[patternID] = struct{}{}
		}
	}

	return patternIDs
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) failed: %v", path, err)
	}

	return data
}

func mustOpenFile(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) failed: %v", path, err)
	}

	return file
}

func testPackageDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Dir(filename)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	return filepath.Clean(filepath.Join(testPackageDir(t), "..", ".."))
}
