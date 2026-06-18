package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestSyncOneFile_RestoresProblemSemanticEnvelopeFromCarrierBlock(t *testing.T) {
	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()

	problem, filePath, err := artifact.FrameProblem(ctx, sourceStore, haftDir, artifact.ProblemFrameInput{
		Title:      "Semantic sync",
		Signal:     "Markdown carrier must restore structured_data into SQLite.",
		Acceptance: "Imported ProblemCard keeps exact semantic envelope.",
	})
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	result, err := syncOneFile(ctx, importStore, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "created" {
		t.Fatalf("sync result = %q, want created", result)
	}

	imported, err := importStore.Get(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := imported.UnmarshalProblemFields()
	if fields.Semantic == nil {
		t.Fatal("imported semantic envelope missing")
	}
	if fields.Semantic.Status != artifact.SemanticStatusExact {
		t.Fatalf("semantic status = %q, want exact", fields.Semantic.Status)
	}
	if fields.Signal != "Markdown carrier must restore structured_data into SQLite." {
		t.Fatalf("signal = %q", fields.Signal)
	}
}

func TestSyncOneFile_ImportsLegacyProblemAsLegacyDegraded(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	filePath := filepath.Join(t.TempDir(), "prob-legacy.md")
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	content := `---
id: prob-legacy
kind: ProblemCard
version: 1
status: active
title: Legacy problem
created_at: ` + now + `
updated_at: ` + now + `
---

# Legacy problem

## Signal

Old markdown has no structured data carrier block.
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := syncOneFile(ctx, store, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "created" {
		t.Fatalf("sync result = %q, want created", result)
	}

	imported, err := store.Get(ctx, "prob-legacy")
	if err != nil {
		t.Fatal(err)
	}
	fields := imported.UnmarshalProblemFields()
	if fields.Signal != "Old markdown has no structured data carrier block." {
		t.Fatalf("signal = %q", fields.Signal)
	}
	if fields.Semantic == nil {
		t.Fatal("legacy semantic envelope missing")
	}
	if fields.Semantic.Status != artifact.SemanticStatusLegacy {
		t.Fatalf("semantic status = %q, want legacy", fields.Semantic.Status)
	}
	if len(fields.Semantic.Warnings) == 0 {
		t.Fatal("legacy import should carry audit warning")
	}
}
