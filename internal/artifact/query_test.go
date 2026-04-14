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

	hasDecisionHealth := len(data.PendingDecisions) > 0 ||
		len(data.HealthyDecisions) > 0 ||
		len(data.UnassessedDecisions) > 0
	if !hasDecisionHealth {
		t.Error("missing decision health buckets")
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
		len(data.HealthyDecisions) > 0 ||
		len(data.UnassessedDecisions) > 0 ||
		len(data.StaleItems) > 0 ||
		len(data.InProgressProblems) > 0 ||
		len(data.BacklogProblems) > 0 ||
		len(data.AddressedProblems) > 0 ||
		len(data.RecentNotes) > 0
	if hasAny {
		t.Error("expected empty status data for empty DB")
	}
}

func TestFetchStatusData_DerivesDecisionHealthBuckets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mustCreateDecision := func(id string, title string) {
		t.Helper()

		err := store.Create(ctx, &Artifact{
			Meta: Meta{
				ID:        id,
				Kind:      KindDecisionRecord,
				Title:     title,
				Status:    StatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Body: title,
		})
		if err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	mustAddEvidence := func(decisionID string, item EvidenceItem) {
		t.Helper()

		err := store.AddEvidenceItem(ctx, &item, decisionID)
		if err != nil {
			t.Fatalf("add evidence to %s: %v", decisionID, err)
		}
	}

	mustCreateDecision("dec-healthy", "Healthy decision")
	mustAddEvidence("dec-healthy", EvidenceItem{
		ID:              "evid-healthy",
		Type:            "measurement",
		Content:         "latency meets target",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})

	mustCreateDecision("dec-pending", "Pending decision")
	mustAddEvidence("dec-pending", EvidenceItem{
		ID:              "evid-pending",
		Type:            "research",
		Content:         "design review completed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})

	mustCreateDecision("dec-unassessed", "Unassessed decision")

	mustCreateDecision("dec-stale", "Stale decision")
	mustAddEvidence("dec-stale", EvidenceItem{
		ID:              "evid-stale-measure",
		Type:            "measurement",
		Content:         "rollout met initial threshold",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})
	mustAddEvidence("dec-stale", EvidenceItem{
		ID:              "evid-stale-research",
		Type:            "research",
		Content:         "follow-up field evidence weakens the result",
		Verdict:         "weakens",
		CongruenceLevel: 2,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})

	data, err := FetchStatusData(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(data.HealthyDecisions) != 1 || data.HealthyDecisions[0].Meta.ID != "dec-healthy" {
		t.Fatalf("healthy decisions = %#v, want dec-healthy only", decisionIDs(data.HealthyDecisions))
	}

	if len(data.PendingDecisions) != 1 || data.PendingDecisions[0].Meta.ID != "dec-pending" {
		t.Fatalf("pending decisions = %#v, want dec-pending only", decisionIDs(data.PendingDecisions))
	}

	if len(data.UnassessedDecisions) != 1 || data.UnassessedDecisions[0].Meta.ID != "dec-unassessed" {
		t.Fatalf("unassessed decisions = %#v, want dec-unassessed only", decisionIDs(data.UnassessedDecisions))
	}

	staleHealth := data.DecisionHealth["dec-stale"]
	if got := staleHealth.Label(); got != "Shipped / Stale" {
		t.Fatalf("stale decision label = %q, want %q", got, "Shipped / Stale")
	}

	foundStale := false
	for _, item := range data.StaleItems {
		if item.ID == "dec-stale" {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatal("expected stale decision in refresh queue")
	}
}

func decisionIDs(items []*Artifact) []string {
	ids := make([]string, 0, len(items))

	for _, item := range items {
		ids = append(ids, item.Meta.ID)
	}

	return ids
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
