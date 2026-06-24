package project

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/embedding"
)

type crossLiveQuery struct {
	text   string
	target string
}

var crossLiveQueries = []crossLiveQuery{
	{"how do we show which functions are exercised by tests in the graph", "ef966a11"},
	{"scoring whether our past predictions were overconfident", "c3c7fa88"},
	{"finding a symbol when I mistype its name", "1c108a55"},
	{"representing call relationships between functions across files", "5825abc6"},
	{"stop a risky database migration from being treated as a quick low-effort change", "0a6edafd"},
	{"showing when the code has diverged from what a recorded decision assumed", "9fdd33ed"},
	{"attach a remembered fact to a specific place in the code", "26be1e4b"},
	{"filter out low-confidence machine-written justifications", "732219b6"},
	{"order the ungoverned modules by how many things depend on them", "e4b86938"},
	{"the plan for adding meaning-based search to the tool later", "3aaad199"},
	{"only run the tests that a change actually touches", "ef966a11"},
	{"measure if the agent is systematically too sure of itself", "c3c7fa88"},
	{"connect a class to the parent it inherits from", "5825abc6"},
	{"warn me before I make an irreversible choice about authentication", "0a6edafd"},
	{"remember a project fact without a full decision record", "26be1e4b"},
	{"rank search hits so generated stub files sink below hand-written code", "ef966a11"},
}

// TestLiveCrossProjectRecallFloorEval measures the cross-project boundary with
// the live decision corpus and production EmbeddingGemma sidecar. It is skipped
// when the corpus or sidecar is absent, but when it runs, hybrid R@k must not
// regress below the FTS floor for the same corpus/query set.
func TestLiveCrossProjectRecallFloorEval(t *testing.T) {
	if testing.Short() {
		t.Skip("live cross-project recall eval skipped in -short")
	}

	decisionsDir := filepath.Join("..", "..", ".haft", "decisions")
	files, err := filepath.Glob(filepath.Join(decisionsDir, "*.md"))
	if err != nil || len(files) == 0 {
		t.Skipf("live decision corpus not found at %s — skipping eval", decisionsDir)
	}

	embedder, err := embedding.New(embedding.Config{Provider: embedding.ProviderLocal})
	if embedding.Degraded(err) {
		t.Skipf("sidecar unavailable, eval needs the production embedder: %v", err)
	}
	if err != nil {
		t.Fatalf("embedder: %v", err)
	}
	_ = embedder.Close()

	ctx := context.Background()
	store := newTempIndexStore(t)
	loadCrossDecisionCorpus(t, ctx, store, files)

	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) {
		return embedding.New(embedding.Config{Provider: embedding.ProviderLocal})
	})
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm hybrid corpus: %v", err)
	}

	ftsRanks := make([]int, 0, len(crossLiveQueries))
	hybridRanks := make([]int, 0, len(crossLiveQueries))
	for _, query := range crossLiveQueries {
		ftsRanks = append(ftsRanks, rankCrossTarget(ctx, t, store, query, false, nil))
		hybridRanks = append(hybridRanks, rankCrossTarget(ctx, t, store, query, true, hybrid))
	}

	fts := crossRecallReport(ftsRanks)
	hyb := crossRecallReport(hybridRanks)
	t.Logf("corpus=%d decisions  queries=%d", len(files), len(crossLiveQueries))
	t.Logf("%-8s %10s %10s %8s", "metric", "FTS5", "Hybrid", "delta")
	for _, k := range []int{1, 3, 5, 10} {
		t.Logf("R@%-6d %10.2f %10.2f %+8.2f", k, fts.at[k], hyb.at[k], hyb.at[k]-fts.at[k])
		if hyb.at[k] < fts.at[k] {
			t.Errorf("hybrid R@%d %.2f < FTS R@%d %.2f", k, hyb.at[k], k, fts.at[k])
		}
	}
	t.Logf("%-8s %10.3f %10.3f %+8.3f", "MRR", fts.mrr, hyb.mrr, hyb.mrr-fts.mrr)
}

func loadCrossDecisionCorpus(t *testing.T, ctx context.Context, store *IndexStore, files []string) {
	t.Helper()
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(path), ".md")
		title, body := splitCrossFrontmatterTitle(string(raw))
		if err := store.WriteDecision(ctx, IndexEntry{
			ProjectID:     "haft-live",
			ProjectName:   "haft-live",
			DecisionID:    id,
			Title:         title,
			SelectedTitle: title,
			WhySelected:   body,
			PrimaryLang:   "go",
			CreatedAt:     "2026-06-24",
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
}

var crossHeadingPattern = regexp.MustCompile(`(?m)^#\s+(.+)$`)

func splitCrossFrontmatterTitle(text string) (string, string) {
	if strings.HasPrefix(text, "---") {
		if parts := strings.SplitN(text, "---", 3); len(parts) == 3 {
			text = parts[2]
		}
	}
	title := ""
	if match := crossHeadingPattern.FindStringSubmatch(text); match != nil {
		title = strings.TrimSpace(match[1])
	}
	return title, strings.TrimSpace(text)
}

func rankCrossTarget(ctx context.Context, t *testing.T, store *IndexStore, query crossLiveQuery, useHybrid bool, hybrid *CrossHybrid) int {
	t.Helper()
	var results []IndexRecall
	var err error
	if useHybrid {
		results, err = hybrid.Search(ctx, query.text, "current-project", "go", 100)
	} else {
		results, err = store.Search(ctx, query.text, "current-project", "go", 100)
	}
	if err != nil {
		t.Fatalf("search %q: %v", query.text, err)
	}
	for rank, item := range results {
		if strings.Contains(item.DecisionID, query.target) {
			return rank
		}
	}
	return crossRankMiss
}

type crossRecallEvalReport struct {
	at  map[int]float64
	mrr float64
}

func crossRecallReport(ranks []int) crossRecallEvalReport {
	out := crossRecallEvalReport{at: map[int]float64{}}
	for _, k := range []int{1, 3, 5, 10} {
		hits := 0
		for _, rank := range ranks {
			if rank < k {
				hits++
			}
		}
		out.at[k] = float64(hits) / float64(len(ranks))
	}
	for _, rank := range ranks {
		if rank < crossRankMiss {
			out.mrr += 1.0 / float64(rank+1)
		}
	}
	out.mrr /= float64(len(ranks))
	return out
}
