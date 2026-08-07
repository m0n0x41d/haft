package codebase

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

type hg5CorpusManifest struct {
	Concerns []hg5CorpusConcern `json:"concerns"`
}

type hg5CorpusConcern struct {
	ID                  string            `json:"id"`
	Class               string            `json:"class"`
	OperatorQuery       string            `json:"operator_query"`
	Filters             map[string]string `json:"filters"`
	AcceptableSymbolIDs []string          `json:"acceptable_symbol_ids"`
}

func TestConcernDiscoveryMeetsFrozenCodeNativeCorpus(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest := readHG5CorpusManifest(t, projectRoot)
	store := buildHG5CorpusSymbolStore(t, projectRoot)
	budget, err := NewDiscoveryBudget(12)
	if err != nil {
		t.Fatal(err)
	}
	topFive := 0
	topThree := 0
	for _, concern := range manifest.Concerns {
		if concern.Class != "code_native" {
			continue
		}
		query, err := NewConcernQuery(
			hg5ConcernQueryWithFilters(concern),
		)
		if err != nil {
			t.Fatalf("%s query: %v", concern.ID, err)
		}
		result, err := store.DiscoverSymbols(
			context.Background(),
			query,
			budget,
			1,
		)
		if err != nil {
			t.Fatalf("%s discovery: %v", concern.ID, err)
		}
		rank := hg5AcceptableRank(
			result,
			concern.AcceptableSymbolIDs,
		)
		t.Logf(
			"%s acceptable rank=%d candidates=%s",
			concern.ID,
			rank,
			hg5CandidateSummary(result),
		)
		if rank > 0 && rank <= 5 {
			topFive++
		}
		if rank > 0 && rank <= 3 {
			topThree++
		}
	}
	if topFive != 6 {
		t.Fatalf("code-native top-5 threshold = %d/6, want 6/6", topFive)
	}
	if topThree < 5 {
		t.Fatalf("code-native top-3 threshold = %d/6, want at least 5/6", topThree)
	}
}

func TestConcernDiscoveryNeverSelectsFrozenNegativeCorpus(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest := readHG5CorpusManifest(t, projectRoot)
	store := buildHG5CorpusSymbolStore(t, projectRoot)
	budget, err := NewDiscoveryBudget(12)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, concern := range manifest.Concerns {
		if concern.Class != "ambiguous_negative" {
			continue
		}
		query, err := NewConcernQuery(
			hg5ConcernQueryWithFilters(concern),
		)
		if err != nil {
			t.Fatalf("%s query: %v", concern.ID, err)
		}
		result, err := store.DiscoverSymbols(
			context.Background(),
			query,
			budget,
			1,
		)
		if err != nil {
			t.Fatalf("%s discovery: %v", concern.ID, err)
		}
		wire, err := result.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			`"selected"`,
			`"winner"`,
			`"resolved_symbol"`,
		} {
			if strings.Contains(string(wire), forbidden) {
				t.Fatalf(
					"%s selected a symbol through %s: %s",
					concern.ID,
					forbidden,
					wire,
				)
			}
		}
		count++
	}
	if count != 3 {
		t.Fatalf("negative corpus cases = %d, want 3", count)
	}
}

func BenchmarkConcernDiscoveryFrozenScopedCorpus(b *testing.B) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	manifest := readHG5CorpusManifest(b, projectRoot)
	store := buildHG5CorpusSymbolStore(b, projectRoot)
	budget, err := NewDiscoveryBudget(12)
	if err != nil {
		b.Fatal(err)
	}
	var concern hg5CorpusConcern
	for _, candidate := range manifest.Concerns {
		if candidate.ID == "CN1" {
			concern = candidate
			break
		}
	}
	query, err := NewConcernQuery(
		hg5ConcernQueryWithFilters(concern),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		result, err := store.DiscoverSymbols(
			context.Background(),
			query,
			budget,
			1,
		)
		if err != nil || len(result.Candidates()) == 0 {
			b.Fatalf(
				"concern discovery candidates=%d err=%v",
				len(result.Candidates()),
				err,
			)
		}
	}
}

func BenchmarkExactSymbolSearchFrozenScopedCorpus(b *testing.B) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	store := buildHG5CorpusSymbolStore(b, projectRoot)
	b.ResetTimer()
	for range b.N {
		symbols, err := store.SearchSymbols(
			context.Background(),
			"shortestPath",
			12,
		)
		if err != nil || len(symbols) == 0 {
			b.Fatalf(
				"exact search candidates=%d err=%v",
				len(symbols),
				err,
			)
		}
	}
}

func buildHG5CorpusSymbolStore(
	t testing.TB,
	projectRoot string,
) *SymbolStore {
	t.Helper()
	fixtureRoot := t.TempDir()
	for _, directory := range []string{
		"internal/codebase",
		"internal/codeintel",
		"internal/p13acceptance",
	} {
		sourceDirectory := filepath.Join(projectRoot, directory)
		entries, err := os.ReadDir(sourceDirectory)
		if err != nil {
			t.Fatal(err)
		}
		targetDirectory := filepath.Join(fixtureRoot, directory)
		if err := os.MkdirAll(targetDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() ||
				filepath.Ext(entry.Name()) != ".go" {
				continue
			}
			content, err := os.ReadFile(
				filepath.Join(sourceDirectory, entry.Name()),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(targetDirectory, entry.Name()),
				content,
				0o644,
			); err != nil {
				t.Fatal(err)
			}
		}
	}
	database, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "hg5-corpus.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := NewSymbolStore(database)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewScanner(database).ScanSymbols(
		context.Background(),
		fixtureRoot,
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSymbolSearchEpoch(
		context.Background(),
		1,
	); err != nil {
		t.Fatal(err)
	}
	return store
}

func readHG5CorpusManifest(
	t testing.TB,
	projectRoot string,
) hg5CorpusManifest {
	t.Helper()
	path := filepath.Join(
		projectRoot,
		".context",
		"v9-remaining-evidence",
		"r2-graph-baseline",
		"corpus-manifest.json",
	)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Корпус живёт под .context, который не отслеживается git, поэтому в
		// свежем чекауте его нет. Пропускаем, как это делает
		// internal/recall/liveeval_test.go со своим живым корпусом: без
		// носителя проверять нечего, и падение сообщало бы об отсутствии
		// файла, а не о регрессии.
		t.Skipf("frozen HG5 corpus not found at %s — skipping", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest hg5CorpusManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func hg5ConcernQueryWithFilters(concern hg5CorpusConcern) string {
	keys := make([]string, 0, len(concern.Filters))
	for key := range concern.Filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{concern.OperatorQuery}
	for _, key := range keys {
		value := concern.Filters[key]
		switch key {
		case "language":
			parts = append(parts, "lang:"+value)
		default:
			parts = append(parts, key+":"+value)
		}
	}
	return strings.Join(parts, " ")
}

func hg5AcceptableRank(
	result SymbolDiscoveryBatch,
	acceptable []string,
) int {
	wanted := make(map[string]bool, len(acceptable))
	for _, id := range acceptable {
		wanted[id] = true
	}
	for index, candidate := range result.Candidates() {
		if wanted[candidate.Symbol().AnchorID] {
			return index + 1
		}
	}
	return 0
}

func hg5CandidateSummary(
	result SymbolDiscoveryBatch,
) string {
	parts := make([]string, 0, len(result.Candidates()))
	for index, candidate := range result.Candidates() {
		parts = append(
			parts,
			candidate.Symbol().Name+
				"@"+
				candidate.Tier().String()+
				"#"+
				candidate.Symbol().AnchorID,
		)
		if index == 4 {
			break
		}
	}
	return strings.Join(parts, ",")
}
