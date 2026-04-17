package fpf

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func loadGoldenRoutes(t *testing.T) []Route {
	t.Helper()

	path := filepath.Join(testRepoRoot(t), "internal", "fpf", "fpf-routes.json")
	routes, err := LoadRoutes(path)
	if err != nil {
		t.Fatalf("LoadRoutes(%q) failed: %v", path, err)
	}

	return routes
}

func loadGoldenSpecChunks(t *testing.T) []SpecChunk {
	t.Helper()

	path := filepath.Join(testRepoRoot(t), "data", "FPF", "FPF-Spec.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("FPF submodule not initialized — run 'git submodule update --init' (%s)", path)
	}
	corpus, err := LoadSpecIndexCorpus(path)
	if err != nil {
		t.Fatalf("LoadSpecIndexCorpus(%q) failed: %v", path, err)
	}

	// Also load compiled pattern files
	patternsDir := filepath.Join(testRepoRoot(t), "internal", "fpf", "patterns")
	patternChunks, err := LoadPatternChunks(patternsDir)
	if err != nil {
		t.Logf("warning: could not load pattern chunks from %s: %v", patternsDir, err)
	}

	allChunks := append(corpus.Indexed, patternChunks...)
	return reindexGoldenSpecChunks(allChunks)
}

func loadGoldenSpecChunksForPatternIDs(t *testing.T, patternIDs map[string]struct{}) []SpecChunk {
	t.Helper()

	chunks := loadGoldenSpecChunks(t)
	return filterGoldenSpecChunks(chunks, patternIDs)
}

func filterGoldenSpecChunks(chunks []SpecChunk, patternIDs map[string]struct{}) []SpecChunk {
	selected := make([]SpecChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk.PatternID == "" {
			continue
		}
		if _, ok := patternIDs[chunk.PatternID]; !ok {
			continue
		}
		selected = append(selected, chunk)
	}

	filtered := FilterIndexChunks(selected)
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
