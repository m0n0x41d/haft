package fpf

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchSpecSemantically_ExactPatternQueryStaysExact(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	results, err := SearchSpecSemantically(db, "a.6:", SemanticSearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("SearchSpecSemantically returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected one exact-pattern result, got %d", len(results))
	}
	if results[0].PatternID != "A.6" {
		t.Fatalf("expected A.6 exact hit, got %#v", results[0])
	}
	if results[0].Tier != SpecSearchTierPattern {
		t.Fatalf("expected exact pattern tier, got %q", results[0].Tier)
	}
}

func TestSearchSpecSemantically_DropsZeroSimilarityTail(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	artifactPath := filepath.Join(t.TempDir(), "semantic-test.json.gz")
	embedder := newSemanticTestEmbedder()

	if err := BuildSemanticArtifact(context.Background(), db, embedder, artifactPath); err != nil {
		t.Fatalf("BuildSemanticArtifact returned error: %v", err)
	}

	results, err := SearchSpecSemantically(db, "decision record rollback rationale", SemanticSearchOptions{
		Limit:        5,
		ArtifactPath: artifactPath,
		Embedder:     embedder,
	})
	if err != nil {
		t.Fatalf("SearchSpecSemantically returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected only one scored decision result, got %s", formatGoldenResults(results))
	}
	if results[0].PatternID != "E.9" {
		t.Fatalf("expected E.9 only, got %#v", results[0])
	}
	if strings.Contains(formatGoldenResults(results), "A.6") {
		t.Fatalf("expected zero-similarity boundary tail to be dropped, got %s", formatGoldenResults(results))
	}
}

func TestSearchSpecSemantically_FullCorpusNoisyQueriesShowSemanticGain(t *testing.T) {
	routes := loadGoldenRoutes(t)
	chunks := loadGoldenSpecChunks(t)
	_, db, cleanup := buildIndexWithChunksAndRoutes(t, chunks, routes, false)
	defer cleanup()

	artifactPath := filepath.Join(t.TempDir(), "semantic-full.json.gz")
	embedder := newSemanticTestEmbedder()
	if err := BuildSemanticArtifact(context.Background(), db, embedder, artifactPath); err != nil {
		t.Fatalf("BuildSemanticArtifact returned error: %v", err)
	}

	cases := loadSemanticEvalCases(t)
	gainCases := 0

	for _, test := range cases {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			deterministic, err := SearchSpecWithOptions(db, test.Query, SpecSearchOptions{
				Limit: test.TopN,
			})
			if err != nil {
				t.Fatalf("SearchSpecWithOptions(%q) error: %v", test.Query, err)
			}

			semantic, err := SearchSpecSemantically(db, test.Query, SemanticSearchOptions{
				Limit:            test.TopN,
				ArtifactPath:     artifactPath,
				Embedder:         embedder,
				DisableRouteSeed: true,
			})
			if err != nil {
				t.Fatalf("SearchSpecSemantically(%q) error: %v", test.Query, err)
			}

			deterministicHits := topGoldenHits(truncateGoldenResults(deterministic, test.TopN), test.ExpectedPatternIDs)
			semanticHits := topGoldenHits(truncateGoldenResults(semantic, test.TopN), test.ExpectedPatternIDs)

			if len(semanticHits) == 0 {
				t.Fatalf(
					"semantic retrieval missed %v for %q; got %s",
					test.ExpectedPatternIDs,
					test.Query,
					formatGoldenResults(semantic),
				)
			}

			if !test.RequireSemanticGain {
				return
			}

			if len(semanticHits) <= len(deterministicHits) {
				t.Fatalf(
					"semantic retrieval failed keep-rule gain for %q: semantic=%d deterministic=%d\nsemantic: %s\ndeterministic: %s",
					test.Query,
					len(semanticHits),
					len(deterministicHits),
					formatGoldenResults(semantic),
					formatGoldenResults(deterministic),
				)
			}

			gainCases++
		})
	}

	if gainCases == 0 {
		t.Fatal("expected at least one noisy query with semantic gain over deterministic baseline")
	}
}

type semanticEvalCase struct {
	Name                string   `json:"name"`
	Query               string   `json:"query"`
	TopN                int      `json:"top_n"`
	ExpectedPatternIDs  []string `json:"expected_pattern_ids"`
	RequireSemanticGain bool     `json:"require_semantic_gain"`
}

func loadSemanticEvalCases(t *testing.T) []semanticEvalCase {
	t.Helper()

	path := filepath.Join(testPackageDir(t), "testdata", "semantic_eval_queries.json")
	data := mustReadFile(t, path)

	cases := []semanticEvalCase{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v", path, err)
	}

	return cases
}

type semanticTestEmbedder struct {
	descriptor SemanticEmbedderDescriptor
}

func newSemanticTestEmbedder() semanticTestEmbedder {
	return semanticTestEmbedder{
		descriptor: SemanticEmbedderDescriptor{
			Provider:   "test",
			Model:      "semantic-eval-stub",
			Dimensions: 7,
		},
	}
}

func (embedder semanticTestEmbedder) Descriptor() SemanticEmbedderDescriptor {
	return embedder.descriptor
}

func (embedder semanticTestEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	_ = ctx

	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vectors = append(vectors, embedder.embedText(text))
	}
	return vectors, nil
}

func (embedder semanticTestEmbedder) embedText(text string) []float32 {
	lower := strings.ToLower(cleanMarkdownText(text))
	concepts := []float32{
		semanticAxisScore(lower, "boundary", "interface", "contract", "promise", "guarantee", "duty", "obligation", "gate", "admission", "admissibility", "deontic", "evidence", "proof"),
		semanticAxisScore(lower, "language", "cue", "route", "routing", "routed", "stabilize", "stabilized", "notice", "observe", "articulation", "hunch", "signal", "handoff", "half formed", "half-formed"),
		semanticAxisScore(lower, "compare", "comparison", "trade-off", "tradeoff", "trade space", "pareto", "non-dominated", "dimension", "normalization", "selection", "choose", "scalar", "score", "number"),
		semanticAxisScore(lower, "creative", "novel", "novelty", "diverse", "diversity", "generator", "portfolio", "explore", "exploration", "converging", "converge", "idea", "wide", "variety"),
		semanticAxisScore(lower, "decision", "decision record", "design rationale", "architecture rationale", "rationale", "record", "adr", "decision"),
		semanticAxisScore(lower, "rollback", "counterargument", "rejected", "alternative", "option", "rollback condition", "rollback trigger"),
		semanticAxisScore(lower, "evidence", "assurance", "reliability", "warrant", "evidence", "congruence"),
	}

	return concepts
}

func semanticAxisScore(text string, terms ...string) float32 {
	score := float32(0)
	for _, term := range terms {
		if strings.Contains(text, term) {
			score++
		}
	}
	return score
}

func TestBuildSemanticArtifactData_RoundTripsMetadata(t *testing.T) {
	_, db, cleanup := buildTestIndex(t)
	defer cleanup()

	artifact, err := BuildSemanticArtifactData(context.Background(), db, newSemanticTestEmbedder())
	if err != nil {
		t.Fatalf("BuildSemanticArtifactData returned error: %v", err)
	}

	if artifact.Provider != "test" || artifact.Model != "semantic-eval-stub" || artifact.Dimensions != 7 {
		t.Fatalf("unexpected semantic descriptor in artifact: %#v", artifact)
	}
	if len(artifact.Documents) == 0 {
		t.Fatal("expected semantic artifact documents")
	}
	if len(artifact.Routes) == 0 {
		t.Fatal("expected semantic artifact routes")
	}
	if artifact.Version != SemanticArtifactVersion {
		t.Fatalf("unexpected semantic artifact version: %q", artifact.Version)
	}
}
