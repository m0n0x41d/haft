package artifact

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
)

func TestDriftSymbolCorpusResolvesExactEditedAndFuzzyMovesWithoutSourceScan(t *testing.T) {
	exactTarget := BindingTarget{
		Kind:       BindingTargetSymbol,
		SymbolName: "Exact",
		SymbolKind: "func",
		BodyHash:   "hash-exact",
	}
	editedTarget := BindingTarget{
		Kind:       BindingTargetSymbol,
		SymbolName: "Edited",
		SymbolKind: "func",
		BodyHash:   "hash-old",
	}
	fuzzyTarget := BindingTarget{
		Kind:       BindingTargetSymbol,
		SymbolName: "Fuzzy",
		SymbolKind: "method",
		Receiver:   "OldReceiver",
		BodyHash:   "hash-fuzzy-old",
	}
	corpus := NewCompleteDriftSymbolCorpus(7, "basis-7", []codebase.SymbolSnapshot{
		{FilePath: "new/exact.go", SymbolName: "Exact", SymbolKind: "func", Hash: "hash-exact"},
		{FilePath: "new/edited.go", SymbolName: "Edited", SymbolKind: "func", Hash: "hash-new"},
		{FilePath: "new/fuzzy.go", SymbolName: "Fuzzy", SymbolKind: "func", Hash: "hash-fuzzy-new"},
	})

	if got, ok := findMovedSymbolTarget(corpus, "old/exact.go", []BindingTarget{exactTarget}); !ok || got.FilePath != "new/exact.go" {
		t.Fatalf("exact move = (%+v, %v)", got, ok)
	}
	if got, ok := findEditedMovedSymbolTarget(corpus, "old/edited.go", []BindingTarget{editedTarget}); !ok || got.FilePath != "new/edited.go" {
		t.Fatalf("edited move = (%+v, %v)", got, ok)
	}
	if got, ok := findFuzzyEditedMovedSymbolTarget(corpus, "old/fuzzy.go", []BindingTarget{fuzzyTarget}); !ok || got.FilePath != "new/fuzzy.go" {
		t.Fatalf("fuzzy move = (%+v, %v)", got, ok)
	}
	projection := corpus.Projection()
	if projection.State != DriftProjectionComplete || projection.SourceIndexEpoch != 7 || projection.BasisDigest != "basis-7" {
		t.Fatalf("projection = %+v", projection)
	}
}

func TestDriftSymbolCorpusRejectsAmbiguousEditedMove(t *testing.T) {
	target := BindingTarget{
		Kind:       BindingTargetSymbol,
		SymbolName: "Run",
		SymbolKind: "func",
		BodyHash:   "old",
	}
	corpus := NewCompleteDriftSymbolCorpus(1, "basis", []codebase.SymbolSnapshot{
		{FilePath: "a.go", SymbolName: "Run", SymbolKind: "func", Hash: "a"},
		{FilePath: "b.go", SymbolName: "Run", SymbolKind: "func", Hash: "b"},
	})
	if got, ok := findEditedMovedSymbolTarget(corpus, "old.go", []BindingTarget{target}); ok {
		t.Fatalf("ambiguous move selected %+v", got)
	}
}

func TestBuildDriftSymbolCorpusHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := buildDriftSymbolCorpusFromSource(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
