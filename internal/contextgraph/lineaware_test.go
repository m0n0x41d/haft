package contextgraph

import (
	"context"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// TestFetchCodeContext_LineAwareOverloadSeparation is the P0 keystone gate:
// two same-name methods in one file, each governed by a DIFFERENT decision at
// the legacy symbol-projection level, must NOT bleed context across each other.
// Neither row is upgraded to an exact binding.
func TestFetchCodeContext_LineAwareOverloadSeparation(t *testing.T) {
	store, g, db := setupContextDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mk := func(id, title string) {
		a := &artifact.Artifact{Meta: artifact.Meta{ID: id, Kind: artifact.KindDecisionRecord, Status: artifact.StatusActive, Title: title, CreatedAt: now, UpdatedAt: now}, Body: "x"}
		if err := store.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	mk("dec-a", "A governs the (*Foo) Do overload")
	mk("dec-b", "B governs the (*Bar) Do overload")

	file := "internal/x/do.go"
	ins := func(artID string, startLine, endLine int) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO affected_symbols (artifact_id, file_path, symbol_name, symbol_kind, symbol_line, symbol_end_line) VALUES (?,?,?,?,?,?)`,
			artID, file, "Do", "method", startLine, endLine); err != nil {
			t.Fatal(err)
		}
	}
	ins("dec-a", 10, 20) // (*Foo) Do
	ins("dec-b", 30, 40) // (*Bar) Do

	decIDs := func(cc CodeContext) []string {
		out := []string{}
		for _, d := range cc.AffectedPathContextDecisions {
			out = append(out, d.Meta.ID)
		}
		return out
	}

	// Line inside A's body → dec-a only, line-precise.
	ccA, err := FetchCodeContext(ctx, store, g, Target{File: file, Symbol: "Do", Line: 15})
	if err != nil {
		t.Fatal(err)
	}
	if ids := decIDs(ccA); len(ids) != 1 || ids[0] != "dec-a" {
		t.Fatalf("line 15 must resolve to dec-a only, got %v", ids)
	}
	if len(ccA.ExactBindingDecisions) != 0 || len(ccA.Decisions) != 0 {
		t.Fatalf("legacy affected_symbols became an exact binding: %+v", ccA)
	}
	if ccA.SymbolGranularity != "legacy-line-context (not an exact binding)" {
		t.Fatalf("expected honest legacy granularity, got %q", ccA.SymbolGranularity)
	}

	// Line inside B's body → dec-b only. THE wrong-attribution guard.
	ccB, err := FetchCodeContext(ctx, store, g, Target{File: file, Symbol: "Do", Line: 35})
	if err != nil {
		t.Fatal(err)
	}
	if ids := decIDs(ccB); len(ids) != 1 || ids[0] != "dec-b" {
		t.Fatalf("line 35 must resolve to dec-b only (no bleed from dec-a), got %v", ids)
	}

	// No line → cannot disambiguate; both surface, honestly labeled.
	ccBoth, err := FetchCodeContext(ctx, store, g, Target{File: file, Symbol: "Do"})
	if err != nil {
		t.Fatal(err)
	}
	if len(decIDs(ccBoth)) != 2 {
		t.Fatalf("line-blind query should surface both overloads, got %v", decIDs(ccBoth))
	}
	if ccBoth.SymbolGranularity != "legacy file+name context (not an exact binding)" {
		t.Fatalf("line-blind must be labeled, got %q", ccBoth.SymbolGranularity)
	}

	// Line covered by no symbol row → fall back + label, never silent false precision.
	ccFallback, err := FetchCodeContext(ctx, store, g, Target{File: file, Symbol: "Do", Line: 999})
	if err != nil {
		t.Fatal(err)
	}
	if ccFallback.SymbolGranularity != "legacy file+name context (not an exact binding)" {
		t.Fatalf("uncovered line must fall back + label, got %q", ccFallback.SymbolGranularity)
	}
	if len(decIDs(ccFallback)) != 2 {
		t.Fatalf("fallback should surface both, got %v", decIDs(ccFallback))
	}
}
