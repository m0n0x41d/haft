package codebase

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestConcernQueryBoundaryPreservesRawAndNormalizesTerms(t *testing.T) {
	query, err := NewConcernQuery(
		"Где обновляется граф через RefreshIncremental path:internal/codebase",
	)
	if err != nil {
		t.Fatal(err)
	}
	if query.Raw() !=
		"Где обновляется граф через RefreshIncremental path:internal/codebase" {
		t.Fatalf("raw query changed: %q", query.Raw())
	}
	if !reflect.DeepEqual(
		query.Terms(),
		[]string{"refreshincremental", "refresh", "incremental"},
	) {
		t.Fatalf("normalized terms = %#v", query.Terms())
	}
	if !reflect.DeepEqual(
		query.PathFilters(),
		[]string{"internal/codebase"},
	) {
		t.Fatalf("path filters = %#v", query.PathFilters())
	}

	for _, raw := range []string{"", " \n\t ", "the and where"} {
		if _, err := NewConcernQuery(raw); err == nil {
			t.Fatalf("query %q should fail", raw)
		}
	}
	if _, err := NewConcernQuery(
		strings.Repeat("x", MaxConcernQueryBytes+1),
	); err == nil {
		t.Fatal("oversized concern query should fail")
	}
}

func TestDiscoverSymbolsUsesDeterministicEvidenceBearingTiers(t *testing.T) {
	store, _ := newSymbolStore(t)
	ctx := context.Background()
	symbols := []CodeSymbol{
		discoveryFixtureSymbol(
			"internal/codebase/incremental.go",
			"publishIndexEpoch",
			"Scanner.publishIndexEpoch",
			"Scanner",
			"method",
			"go",
			10,
		),
		discoveryFixtureSymbol(
			"internal/codebase/incremental_test.go",
			"TestPublishIndexEpoch",
			"TestPublishIndexEpoch",
			"",
			"func",
			"go",
			20,
		),
		discoveryFixtureSymbol(
			"internal/codebase/generated_mock.go",
			"PublishIndexEpoch",
			"Generated.PublishIndexEpoch",
			"Generated",
			"method",
			"go",
			30,
		),
		discoveryFixtureSymbol(
			"internal/codebase/incremental.go",
			"RefreshIncremental",
			"Scanner.RefreshIncremental",
			"Scanner",
			"method",
			"go",
			40,
		),
		discoveryFixtureSymbol(
			"internal/auth/service.go",
			"Authenticate",
			"Service.Authenticate",
			"Service",
			"method",
			"go",
			50,
		),
	}
	if err := store.ReplaceFileSymbols(
		ctx,
		"internal/codebase/incremental.go",
		[]CodeSymbol{symbols[0], symbols[3]},
	); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range symbols[1:] {
		if symbol.FilePath == "internal/codebase/incremental.go" {
			continue
		}
		if err := store.ReplaceFileSymbols(
			ctx,
			symbol.FilePath,
			[]CodeSymbol{symbol},
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetSymbolSearchEpoch(ctx, 7); err != nil {
		t.Fatal(err)
	}
	budget, err := NewDiscoveryBudget(12)
	if err != nil {
		t.Fatal(err)
	}

	exact := discoverFixtureBatch(
		t,
		store,
		ctx,
		"publishIndexEpoch",
		budget,
		7,
	)
	if exact.Kind().String() != DiscoveryCandidates ||
		len(exact.Candidates()) != 1 ||
		exact.Candidates()[0].Tier().String() != LexicalTierExactName {
		t.Fatalf("exact batch = %#v", exact)
	}

	stableID := discoverFixtureBatch(
		t,
		store,
		ctx,
		exact.Candidates()[0].Symbol().AnchorID,
		budget,
		7,
	)
	if len(stableID.Candidates()) != 1 ||
		stableID.Candidates()[0].Tier().String() !=
			LexicalTierExactStableID {
		t.Fatalf("stable ID batch = %#v", stableID)
	}

	qualified := discoverFixtureBatch(
		t,
		store,
		ctx,
		"Scanner.publishIndexEpoch",
		budget,
		7,
	)
	if len(qualified.Candidates()) != 1 ||
		qualified.Candidates()[0].Tier().String() !=
			LexicalTierExactQualifiedName {
		t.Fatalf("qualified batch = %#v", qualified)
	}

	lexical := discoverFixtureBatch(
		t,
		store,
		ctx,
		"Where is the code index epoch published atomically?",
		budget,
		7,
	)
	if len(lexical.Candidates()) < 3 {
		t.Fatalf("lexical candidates = %#v", lexical.Candidates())
	}
	lanes := map[string]bool{}
	for _, candidate := range lexical.Candidates() {
		lanes[candidate.Lane().String()] = true
		if candidate.Epoch() != 7 ||
			candidate.Symbol().AnchorID == "" ||
			candidate.Coverage().Covered() == 0 ||
			len(candidate.Matches()) == 0 {
			t.Fatalf("candidate lacks evidence: %#v", candidate)
		}
	}
	for _, lane := range []string{
		SymbolLaneProduction,
		SymbolLaneTest,
		SymbolLaneGenerated,
	} {
		if !lanes[lane] {
			t.Fatalf("lane %q missing from %#v", lane, lanes)
		}
	}

	russian := discoverFixtureBatch(
		t,
		store,
		ctx,
		"Где происходит RefreshIncremental?",
		budget,
		7,
	)
	if len(russian.Candidates()) == 0 ||
		russian.Candidates()[0].Symbol().Name != "RefreshIncremental" {
		t.Fatalf("mixed-language query = %#v", russian.Candidates())
	}

	typo := discoverFixtureBatch(
		t,
		store,
		ctx,
		"Autenticate",
		budget,
		7,
	)
	if len(typo.Candidates()) != 1 ||
		typo.Candidates()[0].Symbol().Name != "Authenticate" ||
		typo.Candidates()[0].Tier().String() != LexicalTierEditDistance {
		t.Fatalf("typo batch = %#v", typo)
	}

	repeated := discoverFixtureBatch(
		t,
		store,
		ctx,
		"Where is the code index epoch published atomically?",
		budget,
		7,
	)
	if !reflect.DeepEqual(
		discoveryCandidateKeys(lexical.Candidates()),
		discoveryCandidateKeys(repeated.Candidates()),
	) {
		t.Fatalf(
			"discovery order changed: %#v != %#v",
			discoveryCandidateKeys(lexical.Candidates()),
			discoveryCandidateKeys(repeated.Candidates()),
		)
	}
}

func TestDiscoverSymbolsRanksTermCoverageBeforeFieldCompleteness(t *testing.T) {
	store, _ := newSymbolStore(t)
	ctx := context.Background()
	directOwner := discoveryFixtureSymbol(
		"internal/p13acceptance/manifest_test.go",
		"validatePlannedGraphContracts",
		"validatePlannedGraphContracts",
		"",
		"func",
		"go",
		10,
	)
	terseHelper := discoveryFixtureSymbol(
		"internal/p13acceptance/manifest_test.go",
		"testAnchor",
		"testAnchor",
		"",
		"type",
		"go",
		20,
	)
	if err := store.ReplaceFileSymbols(
		ctx,
		"internal/p13acceptance/manifest_test.go",
		[]CodeSymbol{directOwner, terseHelper},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSymbolSearchEpoch(ctx, 1); err != nil {
		t.Fatal(err)
	}
	query, err := NewConcernQuery(
		"planned graph contract test anchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewDiscoveryBudget(12)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.DiscoverSymbols(ctx, query, budget, 1)
	if err != nil {
		t.Fatal(err)
	}
	candidates := result.Candidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	if candidates[0].Symbol().Name != directOwner.Name {
		t.Fatalf(
			"term-rich direct owner ranked below terse helper: %+v",
			discoveryCandidateKeys(candidates),
		)
	}
}

func TestSymbolSearchProjectionSurvivesReopenAndCanonicalRebuild(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "search.db")
	ctx := context.Background()
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSymbolStore(database)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	symbol := discoveryFixtureSymbol(
		"internal/codebase/incremental.go",
		"publishIndexEpoch",
		"Scanner.publishIndexEpoch",
		"Scanner",
		"method",
		"go",
		10,
	)
	if err := store.ReplaceFileSymbols(
		ctx,
		symbol.FilePath,
		[]CodeSymbol{symbol},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSymbolSearchEpoch(ctx, 9); err != nil {
		t.Fatal(err)
	}
	budget, err := NewDiscoveryBudget(5)
	if err != nil {
		t.Fatal(err)
	}
	before := discoverFixtureBatch(
		t,
		store,
		ctx,
		"publish index epoch",
		budget,
		9,
	)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	reopenedStore := NewSymbolStore(reopened)
	if err := reopenedStore.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	after := discoverFixtureBatch(
		t,
		reopenedStore,
		ctx,
		"publish index epoch",
		budget,
		9,
	)
	if !reflect.DeepEqual(
		discoveryCandidateKeys(before.Candidates()),
		discoveryCandidateKeys(after.Candidates()),
	) {
		t.Fatalf(
			"reopen changed candidates: %#v != %#v",
			before.Candidates(),
			after.Candidates(),
		)
	}
	if _, err := reopened.ExecContext(
		ctx,
		`DELETE FROM code_symbol_search`,
	); err != nil {
		t.Fatal(err)
	}
	if err := reopenedStore.RebuildSymbolSearchProjection(
		ctx,
		9,
	); err != nil {
		t.Fatal(err)
	}
	rebuilt := discoverFixtureBatch(
		t,
		reopenedStore,
		ctx,
		"publish index epoch",
		budget,
		9,
	)
	if !reflect.DeepEqual(
		discoveryCandidateKeys(before.Candidates()),
		discoveryCandidateKeys(rebuilt.Candidates()),
	) {
		t.Fatalf(
			"canonical rebuild changed candidates: %#v != %#v",
			before.Candidates(),
			rebuilt.Candidates(),
		)
	}
}

func TestDiscoverSymbolsRejectsSearchEpochMismatch(t *testing.T) {
	store, _ := newSymbolStore(t)
	ctx := context.Background()
	symbol := discoveryFixtureSymbol(
		"internal/codebase/incremental.go",
		"RefreshIncremental",
		"Scanner.RefreshIncremental",
		"Scanner",
		"method",
		"go",
		10,
	)
	if err := store.ReplaceFileSymbols(
		ctx,
		symbol.FilePath,
		[]CodeSymbol{symbol},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSymbolSearchEpoch(ctx, 3); err != nil {
		t.Fatal(err)
	}
	query, err := NewConcernQuery("refresh incremental")
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewDiscoveryBudget(5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DiscoverSymbols(
		ctx,
		query,
		budget,
		4,
	); err == nil || !strings.Contains(err.Error(), "search_epoch_mismatch") {
		t.Fatalf("epoch mismatch error = %v", err)
	}
}

func discoveryFixtureSymbol(
	path string,
	name string,
	qualifiedName string,
	receiver string,
	kind string,
	language string,
	line int,
) CodeSymbol {
	return CodeSymbol{
		FilePath:      path,
		Name:          name,
		QualifiedName: qualifiedName,
		Receiver:      receiver,
		Kind:          kind,
		Lang:          language,
		StartLine:     line,
		EndLine:       line + 1,
		StartByte:     line * 10,
		EndByte:       line*10 + 10,
		Hash:          strings.Repeat("a", 64),
	}
}

func discoverFixtureBatch(
	t *testing.T,
	store *SymbolStore,
	ctx context.Context,
	raw string,
	budget DiscoveryBudget,
	epoch int64,
) SymbolDiscoveryBatch {
	t.Helper()
	query, err := NewConcernQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.DiscoverSymbols(
		ctx,
		query,
		budget,
		epoch,
	)
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func discoveryCandidateKeys(
	candidates []SymbolDiscoveryCandidate,
) []string {
	keys := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		keys = append(
			keys,
			candidate.Tier().String()+"\x00"+
				candidate.Symbol().AnchorID,
		)
	}
	return keys
}
