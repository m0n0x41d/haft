package cli

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/tools"
	_ "modernc.org/sqlite"
)

func TestRunFPFSearch_ExplainTierAndLimit(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	restoreFlags := stubFPFSearchFlags(t, 1, false, true, fpf.SpecSearchTierRoute, "")
	defer restoreFlags()

	output, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"boundary"})
	})
	if err != nil {
		t.Fatalf("runFPFSearch returned error: %v", err)
	}
	if !strings.Contains(output, "tier: route · Boundary discipline and routing") {
		t.Fatalf("expected explain metadata in route output, got:\n%s", output)
	}
	if !strings.Contains(output, "summary: Boundary routing keeps claims on the right layer.") {
		t.Fatalf("expected explain output to include the section summary, got:\n%s", output)
	}
	if strings.Contains(output, "tier: fts") {
		t.Fatalf("expected tier filter to exclude fts results, got:\n%s", output)
	}
	if strings.Contains(output, "### 2.") {
		t.Fatalf("expected limit=1 to cap output, got:\n%s", output)
	}
}

func TestRunFPFSearch_FullFlagLoadsFullSectionBody(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	restoreFlags := stubFPFSearchFlags(t, 1, false, false, fpf.SpecSearchTierPattern, "")
	defer restoreFlags()

	compactOutput, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"A.6"})
	})
	if err != nil {
		t.Fatalf("compact runFPFSearch returned error: %v", err)
	}
	if strings.Contains(compactOutput, "TAIL-MARKER") {
		t.Fatalf("expected compact output to stay snippet-sized, got:\n%s", compactOutput)
	}

	fpfSearchFull = true
	fullOutput, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"A.6"})
	})
	if err != nil {
		t.Fatalf("full runFPFSearch returned error: %v", err)
	}
	if !strings.Contains(fullOutput, "TAIL-MARKER") {
		t.Fatalf("expected --full output to include the complete section body, got:\n%s", fullOutput)
	}
}

func TestRunFPFSemanticSearch_ExplainAndFull(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	artifactPath := filepath.Join(t.TempDir(), "semantic-cli.json.gz")
	embedder := newCLISemanticTestEmbedder()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if err := fpf.BuildSemanticArtifact(context.Background(), db, embedder, artifactPath); err != nil {
		_ = db.Close()
		t.Fatalf("BuildSemanticArtifact returned error: %v", err)
	}
	_ = db.Close()

	restoreFactory := stubFPFSemanticEmbedderFactory(t, func(model string, dimensions int) (fpf.SemanticEmbedder, error) {
		return embedder, nil
	})
	defer restoreFactory()

	restoreFlags := stubFPFSemanticSearchFlags(t, 1, true, true, artifactPath, "test-semantic-cli", 6)
	defer restoreFlags()

	output, err := captureStdout(t, func() error {
		return runFPFSemanticSearch(nil, []string{"boundary", "contract", "unpacking"})
	})
	if err != nil {
		t.Fatalf("runFPFSemanticSearch returned error: %v", err)
	}
	if !strings.Contains(output, "tier: semantic · semantic route seed Boundary discipline and routing") {
		t.Fatalf("expected semantic explain metadata, got:\n%s", output)
	}
	if !strings.Contains(output, "TAIL-MARKER") {
		t.Fatalf("expected semantic --full output to include the full section body, got:\n%s", output)
	}
}

func TestRunFPFSemanticIndex_BuildsArtifact(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	artifactPath := filepath.Join(t.TempDir(), "semantic-build.json.gz")
	embedder := newCLISemanticTestEmbedder()
	restoreFactory := stubFPFSemanticEmbedderFactory(t, func(model string, dimensions int) (fpf.SemanticEmbedder, error) {
		return embedder, nil
	})
	defer restoreFactory()

	restoreFlags := stubFPFSemanticSearchFlags(t, 0, false, false, artifactPath, "test-semantic-cli", 6)
	defer restoreFlags()

	output, err := captureStdout(t, func() error {
		return runFPFSemanticIndex(nil, nil)
	})
	if err != nil {
		t.Fatalf("runFPFSemanticIndex returned error: %v", err)
	}
	if !strings.Contains(output, artifactPath) {
		t.Fatalf("expected artifact path in output, got:\n%s", output)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("expected built artifact at %q: %v", artifactPath, err)
	}
}

func TestRunFPFSearch_TreeModeIsExplicitAndReachable(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	restoreFlags := stubFPFSearchFlags(t, 3, false, true, "", fpf.SpecSearchModeTree)
	defer restoreFlags()

	output, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"boundary", "deontics"})
	})
	if err != nil {
		t.Fatalf("runFPFSearch(tree mode) returned error: %v", err)
	}
	if !strings.Contains(output, "tier: drilldown · tree drill-down leaf A.6.B") {
		t.Fatalf("expected explicit drill-down output, got:\n%s", output)
	}
	if !strings.Contains(output, "### 2. A.6 - Signature Stack & Boundary Discipline") {
		t.Fatalf("expected ancestor path output, got:\n%s", output)
	}
}

func TestRetrieveEmbeddedFPF_ReturnsStructuredResults(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	retrieval, err := retrieveEmbeddedFPF(fpf.SpecRetrievalRequest{
		Query: "A.6",
		Limit: 1,
		Full:  true,
	})
	if err != nil {
		t.Fatalf("retrieveEmbeddedFPF returned error: %v", err)
	}

	if retrieval.Query != "A.6" {
		t.Fatalf("expected query to round-trip, got %q", retrieval.Query)
	}
	if len(retrieval.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(retrieval.Results))
	}
	if retrieval.Results[0].Tier != fpf.SpecSearchTierPattern {
		t.Fatalf("expected pattern tier, got %#v", retrieval.Results[0])
	}
	if !strings.Contains(retrieval.Results[0].Content, "TAIL-MARKER") {
		t.Fatalf("expected full retrieval helper to hydrate the section body, got %#v", retrieval.Results[0])
	}
}

func TestBuildFPFSearchFunc_UsesSharedRetriever(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	search := buildFPFSearchFunc()
	output, err := search(tools.FPFSearchRequest{
		Query:   "A.6",
		Limit:   1,
		Full:    true,
		Explain: true,
	})
	if err != nil {
		t.Fatalf("buildFPFSearchFunc search returned error: %v", err)
	}

	if !strings.Contains(output, "### A.6 - Signature Stack & Boundary Discipline") {
		t.Fatalf("expected agent output to include the shared retrieval heading, got:\n%s", output)
	}
	if !strings.Contains(output, "tier: pattern · exact pattern id") {
		t.Fatalf("expected agent output to keep explain metadata, got:\n%s", output)
	}
	if !strings.Contains(output, "TAIL-MARKER") {
		t.Fatalf("expected agent output to use the shared full-content retrieval, got:\n%s", output)
	}
}

func TestBuildFPFSearchFunc_PassesExperimentalModeThrough(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	search := buildFPFSearchFunc()
	output, err := search(tools.FPFSearchRequest{
		Query:   "boundary deontics",
		Limit:   3,
		Explain: true,
		Mode:    fpf.SpecSearchModeTree,
	})
	if err != nil {
		t.Fatalf("buildFPFSearchFunc tree search returned error: %v", err)
	}

	if !strings.Contains(output, "tier: drilldown · tree drill-down leaf A.6.B") {
		t.Fatalf("expected agent search to expose drilldown mode, got:\n%s", output)
	}
}

func TestRunFPFSearch_InvalidTier(t *testing.T) {
	restoreFlags := stubFPFSearchFlags(t, 10, false, false, "bogus", "")
	defer restoreFlags()

	_, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"boundary"})
	})
	if err == nil {
		t.Fatal("expected invalid tier error")
	}
	if !strings.Contains(err.Error(), "invalid --tier") {
		t.Fatalf("unexpected invalid tier error: %v", err)
	}
}

func TestRunFPFSearch_InvalidMode(t *testing.T) {
	restoreFlags := stubFPFSearchFlags(t, 10, false, false, "", "bogus")
	defer restoreFlags()

	_, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"boundary"})
	})
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "invalid --mode") {
		t.Fatalf("unexpected invalid mode error: %v", err)
	}
}

func TestRunFPFSearch_TreeModeRejectsRouteTier(t *testing.T) {
	restoreFlags := stubFPFSearchFlags(t, 10, false, false, fpf.SpecSearchTierRoute, fpf.SpecSearchModeTree)
	defer restoreFlags()

	_, err := captureStdout(t, func() error {
		return runFPFSearch(nil, []string{"boundary"})
	})
	if err == nil {
		t.Fatal("expected incompatible mode+tier error")
	}
	if !strings.Contains(err.Error(), "does not support tier") {
		t.Fatalf("unexpected incompatible mode+tier error: %v", err)
	}
}

func TestRunFPFSection_LooksUpHeadingAndPatternID(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	tests := []struct {
		name        string
		args        []string
		wantHeading string
		wantBody    string
	}{
		{
			name:        "pattern id",
			args:        []string{"A.6"},
			wantHeading: "## A.6",
			wantBody:    "TAIL-MARKER",
		},
		{
			name:        "heading",
			args:        []string{"A.6 - Signature Stack & Boundary Discipline"},
			wantHeading: "## A.6 - Signature Stack & Boundary Discipline",
			wantBody:    "TAIL-MARKER",
		},
	}

	for _, tt := range tests {
		output, err := captureStdout(t, func() error {
			return runFPFSection(nil, tt.args)
		})
		if err != nil {
			t.Fatalf("%s lookup returned error: %v", tt.name, err)
		}
		if !strings.Contains(output, tt.wantHeading) {
			t.Fatalf("%s lookup output missing heading %q:\n%s", tt.name, tt.wantHeading, output)
		}
		if !strings.Contains(output, tt.wantBody) {
			t.Fatalf("%s lookup output missing body marker %q:\n%s", tt.name, tt.wantBody, output)
		}
	}
}

func TestRunFPFSection_NotFoundMentionsHeadingAndPatternID(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	err := runFPFSection(nil, []string{"missing section"})
	if err == nil {
		t.Fatal("expected missing section error")
	}
	if !strings.Contains(err.Error(), "heading or pattern id") {
		t.Fatalf("expected error to mention heading or pattern id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "\"missing section\"") {
		t.Fatalf("expected error to include the lookup text, got: %v", err)
	}
}

func TestRunFPFSection_UnexpectedLookupErrorKeepsContext(t *testing.T) {
	original := openFPFDBFunc
	openFPFDBFunc = func() (*sql.DB, func(), error) {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			return nil, nil, err
		}

		cleanup := func() {
			_ = db.Close()
		}
		return db, cleanup, nil
	}
	defer func() {
		openFPFDBFunc = original
	}()

	err := runFPFSection(nil, []string{"A.6"})
	if err == nil {
		t.Fatal("expected unexpected lookup error")
	}
	if !strings.Contains(err.Error(), "get FPF section:") {
		t.Fatalf("expected wrapped section lookup error, got: %v", err)
	}
	if strings.Contains(err.Error(), "section not found by heading or pattern id") {
		t.Fatalf("expected unexpected error to avoid not-found rewrite, got: %v", err)
	}
}

func buildFPFSearchTestDB(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fpf-test.db")
	body := "Boundary routing keeps claims on the right layer. " + strings.Repeat("Boundary routing body ", 20) + "TAIL-MARKER"

	chunks := []fpf.SpecChunk{
		{
			ID:        0,
			Heading:   "A.6 - Signature Stack & Boundary Discipline",
			Level:     2,
			Body:      body,
			PatternID: "A.6",
			Keywords:  []string{"boundary", "routing"},
			Queries:   []string{"How do I route boundary statements?"},
		},
		{
			ID:              1,
			Heading:         "A.6.B - Boundary Norm Square",
			Level:           2,
			Body:            "Norm square body",
			PatternID:       "A.6.B",
			ParentPatternID: "A.6",
			Keywords:        []string{"boundary", "deontics"},
			Queries:         []string{"What is the Boundary Norm Square?"},
		},
	}
	routes := []fpf.Route{{
		ID:          "boundary-discipline",
		Title:       "Boundary discipline and routing",
		Description: "Boundary statements and routing",
		Matchers:    []string{"boundary", "routing"},
		Core:        []string{"A.6", "A.6.B"},
		Chain:       []string{"A.6", "A.6.B"},
	}}

	if err := fpf.BuildSpecIndex(dbPath, chunks, routes); err != nil {
		t.Fatalf("BuildSpecIndex failed: %v", err)
	}

	return dbPath
}

func stubOpenFPFDB(t *testing.T, dbPath string) func() {
	t.Helper()

	original := openFPFDBFunc
	openFPFDBFunc = func() (*sql.DB, func(), error) {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, nil, err
		}
		cleanup := func() {
			_ = db.Close()
		}
		return db, cleanup, nil
	}

	return func() {
		openFPFDBFunc = original
	}
}

func stubFPFSearchFlags(t *testing.T, limit int, full bool, explain bool, tier string, mode string) func() {
	t.Helper()

	originalLimit := fpfSearchLimit
	originalFull := fpfSearchFull
	originalExplain := fpfSearchExplain
	originalTier := fpfSearchTier
	originalMode := fpfSearchMode

	fpfSearchLimit = limit
	fpfSearchFull = full
	fpfSearchExplain = explain
	fpfSearchTier = tier
	fpfSearchMode = mode

	return func() {
		fpfSearchLimit = originalLimit
		fpfSearchFull = originalFull
		fpfSearchExplain = originalExplain
		fpfSearchTier = originalTier
		fpfSearchMode = originalMode
	}
}

func stubFPFSemanticSearchFlags(
	t *testing.T,
	limit int,
	full bool,
	explain bool,
	artifactPath string,
	model string,
	dimensions int,
) func() {
	t.Helper()

	originalLimit := fpfSemanticSearchLimit
	originalFull := fpfSemanticSearchFull
	originalExplain := fpfSemanticSearchExplain
	originalArtifactPath := fpfSemanticArtifactPath
	originalModel := fpfSemanticModel
	originalDimensions := fpfSemanticDimensions

	fpfSemanticSearchLimit = limit
	fpfSemanticSearchFull = full
	fpfSemanticSearchExplain = explain
	fpfSemanticArtifactPath = artifactPath
	fpfSemanticModel = model
	fpfSemanticDimensions = dimensions

	return func() {
		fpfSemanticSearchLimit = originalLimit
		fpfSemanticSearchFull = originalFull
		fpfSemanticSearchExplain = originalExplain
		fpfSemanticArtifactPath = originalArtifactPath
		fpfSemanticModel = originalModel
		fpfSemanticDimensions = originalDimensions
	}
}

func stubFPFSemanticEmbedderFactory(
	t *testing.T,
	factory func(model string, dimensions int) (fpf.SemanticEmbedder, error),
) func() {
	t.Helper()

	original := newFPFSemanticEmbedder
	newFPFSemanticEmbedder = factory

	return func() {
		newFPFSemanticEmbedder = original
	}
}

type cliSemanticTestEmbedder struct{}

func newCLISemanticTestEmbedder() cliSemanticTestEmbedder {
	return cliSemanticTestEmbedder{}
}

func (cliSemanticTestEmbedder) Descriptor() fpf.SemanticEmbedderDescriptor {
	return fpf.SemanticEmbedderDescriptor{
		Provider:   "test",
		Model:      "test-semantic-cli",
		Dimensions: 6,
	}
}

func (cliSemanticTestEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	_ = ctx

	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		lower := strings.ToLower(text)
		vectors = append(vectors, []float32{
			axisScore(lower, "boundary", "contract", "routing"),
			axisScore(lower, "norm", "square", "deontic"),
			axisScore(lower, "decision", "record", "rationale"),
			0,
			0,
			0,
		})
	}
	return vectors, nil
}

func axisScore(text string, terms ...string) float32 {
	score := float32(0)
	for _, term := range terms {
		if strings.Contains(text, term) {
			score++
		}
	}
	return score
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}

	os.Stdout = writer
	runErr := fn()
	_ = writer.Close()
	os.Stdout = originalStdout

	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatalf("io.ReadAll failed: %v", readErr)
	}

	return string(data), runErr
}
