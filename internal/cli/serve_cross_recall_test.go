package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestApplyCrossProjectRecallAppendsParaphrasedFrameHit(t *testing.T) {
	ctx := context.Background()
	t.Setenv("HOME", t.TempDir())

	indexStore, err := project.OpenIndex()
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { _ = indexStore.Close() })

	if err := indexStore.WriteDecision(ctx, project.IndexEntry{
		ProjectID:     "other-project",
		ProjectName:   "Other Project",
		DecisionID:    "dec-cross-embed",
		Title:         "Rust fastembed gemma sidecar",
		SelectedTitle: "local embeddings",
		WhySelected:   "augment keyword recall with local vectors",
		WeakestLink:   "model availability",
		PrimaryLang:   "go",
		CreatedAt:     "2026-06-05",
	}); err != nil {
		t.Fatalf("seed cross-project decision: %v", err)
	}

	database, err := db.NewStore(filepath.Join(t.TempDir(), "haft.db"))
	if err != nil {
		t.Fatalf("open artifact db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := artifact.NewStore(database.GetRawDB())

	result := applyCrossProjectRecall(
		ctx,
		"Problem framed.",
		"haft_problem",
		"frame",
		map[string]any{
			"title":  "semantic memory",
			"signal": "local gemma recall should surface related prior decision",
		},
		store,
		&project.Config{ID: "current-project", Name: "Current Project"},
		indexStore,
		nil,
	)

	for _, want := range []string{
		"## Cross-Project History",
		"dec-cross-embed",
		"Rust fastembed gemma sidecar",
		"augment keyword recall with local vectors",
		"CL1 (different context)",
		"Other Project",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("cross-project recall output missing %q:\n%s", want, result)
		}
	}
}
