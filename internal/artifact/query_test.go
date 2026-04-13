package artifact

import (
	"context"
	"testing"
	"time"
)

func TestFetchSearchResults_FindsResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS JetStream for events"},
		Body: "Selected NATS over Kafka for domain event infrastructure",
	})

	results, err := FetchSearchResults(ctx, store, "NATS events", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, a := range results {
		if a.Meta.Title == "NATS JetStream for events" {
			found = true
		}
	}
	if !found {
		t.Error("expected NATS JetStream artifact in results")
	}
}

func TestFetchSearchResults_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	results, err := FetchSearchResults(ctx, store, "nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchSearchResults_EmptyQuery(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := FetchSearchResults(ctx, store, "", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestFetchStatusData_Dashboard(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create some artifacts
	FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Rate limiting", Signal: "Scraper traffic", Context: "api",
	})
	Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "x/time/rate",
		WhySelected:   "Zero deps",
		ValidUntil:    time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}))
	CreateNote(ctx, store, haftDir, NoteInput{
		Title:     "Using RWMutex",
		Rationale: "Low contention verified by load test",
	})

	data, err := FetchStatusData(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}

	hasPendingOrShipped := len(data.PendingDecisions) > 0 || len(data.ShippedDecisions) > 0
	if !hasPendingOrShipped {
		t.Error("missing pending or shipped decisions")
	}
	if len(data.BacklogProblems) == 0 {
		t.Error("missing backlog problems")
	}
	if len(data.RecentNotes) == 0 {
		t.Error("missing recent notes")
	}
}

func TestFetchStatusData_Empty(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	data, err := FetchStatusData(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}
	hasAny := len(data.PendingDecisions) > 0 ||
		len(data.ShippedDecisions) > 0 ||
		len(data.StaleItems) > 0 ||
		len(data.InProgressProblems) > 0 ||
		len(data.BacklogProblems) > 0 ||
		len(data.AddressedProblems) > 0 ||
		len(data.RecentNotes) > 0
	if hasAny {
		t.Error("expected empty status data for empty DB")
	}
}

func TestFetchRelatedArtifacts_FindsByFile(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS decision"},
		Body: "d",
	})
	store.SetAffectedFiles(ctx, "dec-001", []AffectedFile{{Path: "internal/events/producer.go"}})

	results, err := FetchRelatedArtifacts(ctx, store, "internal/events/producer.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, a := range results {
		if a.Meta.Title == "NATS decision" {
			found = true
		}
	}
	if !found {
		t.Error("expected NATS decision in related results")
	}
}

func TestFetchRelatedArtifacts_NoResults(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	results, err := FetchRelatedArtifacts(ctx, store, "nonexistent.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestFetchRelatedArtifacts_EmptyPath(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := FetchRelatedArtifacts(ctx, store, "")
	if err == nil {
		t.Error("expected error for empty file path")
	}
}

func TestResolveProblemAdoptionRefs_FindsLinkedDecisionAndComparedPortfolio(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("REST", "chatty payloads", "Keep JSON request-response semantics"),
			testVariant("gRPC", "tooling overhead", "Adopt binary RPC with generated clients"),
		},
		NoSteppingStoneRationale: "Both transports are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "42ms"},
				"V2": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"V2"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "V1",
					DominatedBy: []string{"V2"},
					Summary:     "Higher latency with no compensating advantage.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "V2", Summary: "Best latency in the compared set."},
			},
			SelectedRef:   "V2",
			PolicyApplied: "Minimize latency within budget.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "gRPC",
		WhySelected:   "Lower latency is worth the tooling overhead for the current scope.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	refs := ResolveProblemAdoptionRefs(ctx, store, problem.Meta.ID)
	if refs.PortfolioRef != portfolio.Meta.ID {
		t.Fatalf("PortfolioRef = %q, want %q", refs.PortfolioRef, portfolio.Meta.ID)
	}
	if refs.ComparedPortfolioRef != portfolio.Meta.ID {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", refs.ComparedPortfolioRef, portfolio.Meta.ID)
	}
	if refs.DecisionRef != decision.Meta.ID {
		t.Fatalf("DecisionRef = %q, want %q", refs.DecisionRef, decision.Meta.ID)
	}
}

func TestResolveProblemAdoptionRefs_KeepsDecisionOnSelectedPortfolioChain(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	comparedPortfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("REST", "chatty payloads", "Keep JSON request-response semantics"),
			testVariant("gRPC", "tooling overhead", "Adopt binary RPC with generated clients"),
		},
		NoSteppingStoneRationale: "Both transports are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: comparedPortfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "42ms"},
				"V2": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"V2"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "V1",
					DominatedBy: []string{"V2"},
					Summary:     "Higher latency with no compensating advantage.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "V2", Summary: "Best latency in the compared set."},
			},
			SelectedRef:   "V2",
			PolicyApplied: "Minimize latency within budget.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	comparedDecision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  comparedPortfolio.Meta.ID,
		SelectedTitle: "gRPC",
		WhySelected:   "The compared portfolio remains the active decision path.",
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			testVariant("WebSocket", "connection lifecycle complexity", "Keep duplex sessions alive"),
			testVariant("SSE", "server-to-client only", "Use unidirectional event streams"),
		},
		NoSteppingStoneRationale: "Both transports are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.DB().ExecContext(ctx, `
		UPDATE artifacts
		SET created_at = ?, updated_at = ?
		WHERE id = ?`,
		"2026-01-02T00:00:00Z",
		"2026-01-02T00:00:00Z",
		comparedDecision.Meta.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	refs := ResolveProblemAdoptionRefs(ctx, store, problem.Meta.ID)
	if refs.PortfolioRef != comparedPortfolio.Meta.ID {
		t.Fatalf("PortfolioRef = %q, want %q", refs.PortfolioRef, comparedPortfolio.Meta.ID)
	}
	if refs.ComparedPortfolioRef != comparedPortfolio.Meta.ID {
		t.Fatalf("ComparedPortfolioRef = %q, want %q", refs.ComparedPortfolioRef, comparedPortfolio.Meta.ID)
	}
	if refs.DecisionRef != comparedDecision.Meta.ID {
		t.Fatalf("DecisionRef = %q, want %q", refs.DecisionRef, comparedDecision.Meta.ID)
	}
}
