package cli

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

func TestRunFPFSearch_ExplainTierAndLimit(t *testing.T) {
	dbPath := buildFPFSearchTestDB(t)

	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	restoreFlags := stubFPFSearchFlags(t, 1, false, true, fpf.SpecSearchTierRoute)
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

	restoreFlags := stubFPFSearchFlags(t, 1, false, false, fpf.SpecSearchTierPattern)
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

func TestRunFPFSearch_InvalidTier(t *testing.T) {
	restoreFlags := stubFPFSearchFlags(t, 10, false, false, "bogus")
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

func buildFPFSearchTestDB(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fpf-test.db")
	body := strings.Repeat("Boundary routing body ", 20) + "TAIL-MARKER"

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
			ID:        1,
			Heading:   "A.6.B - Boundary Norm Square",
			Level:     2,
			Body:      "Norm square body",
			PatternID: "A.6.B",
			Keywords:  []string{"boundary", "deontics"},
			Queries:   []string{"What is the Boundary Norm Square?"},
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

func stubFPFSearchFlags(t *testing.T, limit int, full bool, explain bool, tier string) func() {
	t.Helper()

	originalLimit := fpfSearchLimit
	originalFull := fpfSearchFull
	originalExplain := fpfSearchExplain
	originalTier := fpfSearchTier

	fpfSearchLimit = limit
	fpfSearchFull = full
	fpfSearchExplain = explain
	fpfSearchTier = tier

	return func() {
		fpfSearchLimit = originalLimit
		fpfSearchFull = originalFull
		fpfSearchExplain = originalExplain
		fpfSearchTier = originalTier
	}
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
