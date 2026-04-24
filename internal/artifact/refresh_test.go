package artifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestScanStale_FindsExpired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old decision", ValidUntil: past},
		Body: "expired",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "Fresh decision", ValidUntil: future},
		Body: "still valid",
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(items))
	}
	if items[0].ID != "dec-001" {
		t.Errorf("expected dec-001, got %s", items[0].ID)
	}
	if items[0].DaysStale < 1 {
		t.Errorf("expected >0 days stale, got %d", items[0].DaysStale)
	}
}

func TestScanStale_NoneStale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 stale, got %d", len(items))
	}
}

func TestScanStale_SuppressesTerminalWorkCommissions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	for _, payload := range []map[string]any{
		{
			"id":          "wc-cancelled",
			"state":       "cancelled",
			"valid_until": past,
			"fetched_at":  past,
		},
		{
			"id":          "wc-open-expired",
			"state":       "queued",
			"valid_until": past,
			"fetched_at":  past,
		},
	} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(ctx, &Artifact{
			Meta: Meta{
				ID:         payload["id"].(string),
				Kind:       KindWorkCommission,
				Status:     StatusActive,
				Title:      "WorkCommission " + payload["id"].(string),
				ValidUntil: past,
			},
			StructuredData: string(encoded),
		}); err != nil {
			t.Fatal(err)
		}
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("items = %#v, want one open expired WorkCommission", items)
	}
	if items[0].ID != "wc-open-expired" {
		t.Fatalf("item id = %s, want wc-open-expired", items[0].ID)
	}
}

func TestWaiveArtifact_Decision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Stale decision", ValidUntil: past, Status: StatusActive},
		Body: "# Decision\n\nOriginal content",
	})

	dec, err := WaiveArtifact(ctx, store, haftDir, "dec-001", "Load test still valid at current traffic", "", "Recent test at 1000 req/s passed")
	if err != nil {
		t.Fatal(err)
	}

	if dec.Meta.Status != StatusActive {
		t.Errorf("status = %q, want active", dec.Meta.Status)
	}
	if dec.Meta.ValidUntil == past {
		t.Error("valid_until should have been extended")
	}
	if !strings.Contains(dec.Body, "## Waiver") {
		t.Error("waiver section not appended to body")
	}
}

func TestReopenDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Decision: NATS JetStream", Status: StatusActive, Context: "events"},
		Body: "# Decision\n\nOriginal content",
	})

	dec, newProb, err := ReopenDecision(ctx, store, haftDir, "dec-001", "Throughput approaching design limit")
	if err != nil {
		t.Fatal(err)
	}

	if dec.Meta.Status != StatusRefreshDue {
		t.Errorf("old decision status = %q, want refresh_due", dec.Meta.Status)
	}
	if newProb == nil {
		t.Fatal("expected new ProblemCard")
	}
	if newProb.Meta.Kind != KindProblemCard {
		t.Errorf("new artifact kind = %q, want ProblemCard", newProb.Meta.Kind)
	}
	if !strings.Contains(newProb.Meta.Title, "Revisit") {
		t.Errorf("new problem title should contain 'Revisit', got %q", newProb.Meta.Title)
	}
	if newProb.Meta.Context != "events" {
		t.Errorf("new problem context = %q, want events (inherited)", newProb.Meta.Context)
	}

	// Check link
	links, _ := store.GetLinks(ctx, newProb.Meta.ID)
	found := false
	for _, l := range links {
		if l.Ref == "dec-001" && l.Type == "revisits" {
			found = true
		}
	}
	if !found {
		t.Error("new problem should link to old decision with 'revisits'")
	}
}

func TestSupersedeArtifact_Decision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old", Status: StatusActive},
		Body: "old",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "New", Status: StatusActive},
		Body: "new",
	})

	dec, err := SupersedeArtifact(ctx, store, haftDir, "dec-001", "dec-002", "Team doubled, need Kafka now")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Meta.Status != StatusSuperseded {
		t.Errorf("status = %q, want superseded", dec.Meta.Status)
	}
	if !strings.Contains(dec.Body, "Superseded") {
		t.Error("body should contain superseded section")
	}
}

func TestDeprecateArtifact_Decision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Old", Status: StatusActive},
		Body: "old",
	})

	dec, err := DeprecateArtifact(ctx, store, haftDir, "dec-001", "Feature removed entirely")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Meta.Status != StatusDeprecated {
		t.Errorf("status = %q, want deprecated", dec.Meta.Status)
	}
}

func TestCreateRefreshReport(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "D", Status: StatusActive},
		Body: "d",
	})

	report, err := CreateRefreshReport(ctx, store, haftDir, "dec-001", "waive", "Still valid", "Extended 90 days")
	if err != nil {
		t.Fatal(err)
	}
	if report.Meta.Kind != KindRefreshReport {
		t.Errorf("kind = %q", report.Meta.Kind)
	}
	if !strings.Contains(report.Body, "waive") {
		t.Error("report should mention action")
	}
}

// --- New tests for generalized lifecycle ---

func TestDeprecateArtifact_Note(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-001", Kind: KindNote, Title: "Old note", Status: StatusActive},
		Body: "# Old note\n\nUsing sync.Mutex",
	})

	note, err := DeprecateArtifact(ctx, store, haftDir, "note-001", "Refactored to use channels")
	if err != nil {
		t.Fatal(err)
	}
	if note.Meta.Status != StatusDeprecated {
		t.Errorf("status = %q, want deprecated", note.Meta.Status)
	}
	if !strings.Contains(note.Body, "Deprecated") {
		t.Error("body should contain deprecated section")
	}
}

func TestSupersedeArtifact_NoteByDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-001", Kind: KindNote, Title: "Quick mutex choice", Status: StatusActive},
		Body: "# Mutex\n\nUsing sync.Mutex for cache",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Cache strategy decision", Status: StatusActive},
		Body: "# Decision\n\nFull evaluation of cache options",
	})

	note, err := SupersedeArtifact(ctx, store, haftDir, "note-001", "dec-001", "Promoted to full decision")
	if err != nil {
		t.Fatal(err)
	}
	if note.Meta.Status != StatusSuperseded {
		t.Errorf("status = %q, want superseded", note.Meta.Status)
	}

	// Verify link: decision supersedes note
	links, _ := store.GetLinks(ctx, "dec-001")
	found := false
	for _, l := range links {
		if l.Ref == "note-001" && l.Type == "supersedes" {
			found = true
		}
	}
	if !found {
		t.Error("decision should link to note with 'supersedes'")
	}
}

func TestWaiveArtifact_Note(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-001", Kind: KindNote, Title: "Still valid note", ValidUntil: past, Status: StatusActive},
		Body: "# Note\n\nContent",
	})

	note, err := WaiveArtifact(ctx, store, haftDir, "note-001", "Still applies to current code", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if note.Meta.Status != StatusActive {
		t.Errorf("status = %q, want active", note.Meta.Status)
	}
	if note.Meta.ValidUntil == past {
		t.Error("valid_until should have been extended")
	}
}

func TestScanStale_FindsExpiredNotes(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-001", Kind: KindNote, Title: "Old note", ValidUntil: past, Status: StatusActive},
		Body: "expired note",
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(items))
	}
	if items[0].ID != "note-001" {
		t.Errorf("expected note-001, got %s", items[0].ID)
	}
	if items[0].Kind != string(KindNote) {
		t.Errorf("kind = %q, want Note", items[0].Kind)
	}
}

func TestNoteAutoValidUntil(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	note, _, err := CreateNote(ctx, store, haftDir, NoteInput{
		Title:     "Auto-expiry test",
		Rationale: "Testing that notes get automatic valid_until",
	})
	if err != nil {
		t.Fatal(err)
	}

	if note.Meta.ValidUntil == "" {
		t.Fatal("note should have auto-set valid_until")
	}

	vu, err := time.Parse(time.RFC3339, note.Meta.ValidUntil)
	if err != nil {
		t.Fatalf("invalid valid_until format: %v", err)
	}

	// Should be ~90 days from now
	days := int(time.Until(vu).Hours() / 24)
	if days < 88 || days > 92 {
		t.Errorf("auto valid_until should be ~90 days out, got %d days", days)
	}
}

func TestNoteExplicitValidUntil(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	explicit := "2027-06-01T00:00:00Z"
	note, _, err := CreateNote(ctx, store, haftDir, NoteInput{
		Title:      "Explicit expiry test",
		Rationale:  "User-provided valid_until should be preserved",
		ValidUntil: explicit,
	})
	if err != nil {
		t.Fatal(err)
	}

	if note.Meta.ValidUntil != explicit {
		t.Errorf("valid_until = %q, want %q", note.Meta.ValidUntil, explicit)
	}
}

func TestScanStale_REffDegradedDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create decision with refuting evidence → R_eff = 0.0
	dec, _, _ := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Test decision",
		WhySelected:   "For testing",
		ValidUntil:    "2027-01-01T00:00:00Z", // not expired by valid_until
	}))

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "System crashed under load",
		Verdict:         "refutes",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, item := range items {
		if item.ID == dec.Meta.ID {
			found = true
			if !strings.Contains(item.Reason, "evidence degraded") {
				t.Errorf("reason should mention evidence degraded: %q", item.Reason)
			}
			if !strings.Contains(item.Reason, "R_eff: 0.00") {
				t.Errorf("reason should show R_eff: %q", item.Reason)
			}
		}
	}
	if !found {
		t.Error("decision with R_eff=0.0 should appear in stale scan")
	}
}

func TestScanStale_HealthyEvidenceNotStale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create decision with supporting evidence → R_eff = 1.0
	dec, _, _ := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Healthy decision",
		WhySelected:   "For testing",
		ValidUntil:    "2027-01-01T00:00:00Z",
	}))

	AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "All tests pass",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2027-01-01T00:00:00Z",
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if item.ID == dec.Meta.ID {
			t.Error("decision with R_eff=1.0 should NOT appear in stale scan")
		}
	}
}

func TestScanStale_EpistemicDebtBudgetExceeded(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	now := time.Now().UTC()

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO fpf_state (context_id, active_role, epistemic_debt_budget, updated_at)
		VALUES (?, ?, ?, ?)`,
		"default", "decide", 5.0, now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert fpf_state: %v", err)
	}

	createDecisionWithDebt := func(title string, expiredDays int) string {
		decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
			SelectedTitle: title,
			WhySelected:   "For testing",
			ValidUntil:    now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		}))
		if err != nil {
			t.Fatalf("Decide(%s): %v", title, err)
		}

		_, evidenceErr := AttachEvidence(ctx, store, EvidenceInput{
			ArtifactRef:     decision.Meta.ID,
			Content:         fmt.Sprintf("expired %d days", expiredDays),
			Verdict:         "supports",
			CongruenceLevel: 3,
			ValidUntil:      now.Add(time.Duration(-expiredDays) * 24 * time.Hour).Format(time.RFC3339),
		})
		if evidenceErr != nil {
			t.Fatalf("AttachEvidence(%s): %v", title, evidenceErr)
		}

		return decision.Meta.ID
	}

	firstID := createDecisionWithDebt("First debt", 2)
	secondID := createDecisionWithDebt("Second debt", 3)
	thirdID := createDecisionWithDebt("Third debt", 4)

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	var debtItem *StaleItem
	for index := range items {
		if items[index].Category == StaleCategoryEpistemicDebtExceeded {
			debtItem = &items[index]
			break
		}
	}
	if debtItem == nil {
		t.Fatal("expected epistemic debt budget alert")
	}

	if math.Abs(debtItem.TotalED-9.0) > 0.05 {
		t.Fatalf("total ED = %.6f, want approx 9.0", debtItem.TotalED)
	}
	if debtItem.DebtBudget != 5.0 {
		t.Fatalf("budget = %.1f, want 5.0", debtItem.DebtBudget)
	}
	if math.Abs(debtItem.DebtExcess-4.0) > 0.05 {
		t.Fatalf("excess = %.6f, want approx 4.0", debtItem.DebtExcess)
	}
	if len(debtItem.DecisionDebt) != 3 {
		t.Fatalf("decision debt count = %d, want 3", len(debtItem.DecisionDebt))
	}
	if debtItem.DecisionDebt[0].DecisionID != thirdID {
		t.Fatalf("highest debt decision = %q, want %q", debtItem.DecisionDebt[0].DecisionID, thirdID)
	}
	if debtItem.DecisionDebt[1].DecisionID != secondID {
		t.Fatalf("second debt decision = %q, want %q", debtItem.DecisionDebt[1].DecisionID, secondID)
	}
	if debtItem.DecisionDebt[2].DecisionID != firstID {
		t.Fatalf("third debt decision = %q, want %q", debtItem.DecisionDebt[2].DecisionID, firstID)
	}
	if !strings.Contains(debtItem.Reason, thirdID+" 4.0") {
		t.Fatalf("reason should include decision breakdown, got %q", debtItem.Reason)
	}
}

func TestScanStale_EpistemicDebtBudgetExceeded_DateOnlyEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO fpf_state (context_id, active_role, epistemic_debt_budget, updated_at)
		VALUES (?, ?, ?, ?)`,
		"default", "decide", 1.0, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert fpf_state: %v", err)
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Date-only evidence",
		WhySelected:   "For testing",
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "evidence with date-only expiry",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      "2020-01-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if item.Category != StaleCategoryEpistemicDebtExceeded {
			continue
		}
		if item.TotalED <= 0 {
			t.Fatalf("expected positive debt for date-only expiry, got %+v", item)
		}
		return
	}

	t.Fatal("expected epistemic debt alert from date-only evidence")
}

func TestScanStale_EpistemicDebtBudgetUsesRawAggregate(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO fpf_state (context_id, active_role, epistemic_debt_budget, updated_at)
		VALUES (?, ?, ?, ?)`,
		"default", "decide", 0.1, now.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert fpf_state: %v", err)
	}

	for index := 0; index < 3; index++ {
		decisionID := fmt.Sprintf("dec-raw-%03d", index)
		createActiveDecisionForScan(t, store, decisionID, fmt.Sprintf("raw-%03d", index), now.Add(time.Duration(index)*time.Minute))
		addEvidenceForScan(t, store, decisionID, fmt.Sprintf("ev-raw-%03d", index), "supports", now.Add(-58*time.Minute))
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	debtItem := findStaleItemByCategory(items, StaleCategoryEpistemicDebtExceeded)
	if debtItem == nil {
		t.Fatal("expected epistemic debt alert from sub-tenth debts")
	}
	if debtItem.TotalED <= debtItem.DebtBudget {
		t.Fatalf("total ED %.3f should exceed budget %.3f", debtItem.TotalED, debtItem.DebtBudget)
	}
}

func TestScanStale_REffDegradedIncludesOldActiveDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	baseTime := time.Now().UTC().Add(-48 * time.Hour)

	createActiveDecisionForScan(t, store, "dec-old-risk", "old risk", baseTime)
	addEvidenceForScan(t, store, "dec-old-risk", "ev-old-risk", "refutes", baseTime.Add(24*time.Hour))

	for index := 0; index < 100; index++ {
		decisionID := fmt.Sprintf("dec-new-%03d", index)
		title := fmt.Sprintf("new-%03d", index)
		createdAt := baseTime.Add(time.Duration(index+1) * time.Minute)
		createActiveDecisionForScan(t, store, decisionID, title, createdAt)
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if item.ID != "dec-old-risk" {
			continue
		}
		if item.Category != StaleCategoryREffDegraded {
			t.Fatalf("stale category = %q, want %q", item.Category, StaleCategoryREffDegraded)
		}
		return
	}

	t.Fatal("expected degraded old decision to be included")
}

func TestScanStale_EpistemicDebtIncludesOldActiveDecisionBeyondFiveHundred(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	baseTime := time.Now().UTC().Add(-72 * time.Hour)

	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO fpf_state (context_id, active_role, epistemic_debt_budget, updated_at)
		VALUES (?, ?, ?, ?)`,
		"default", "decide", 0.5, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert fpf_state: %v", err)
	}

	createActiveDecisionForScan(t, store, "dec-old-debt", "old debt", baseTime)
	addEvidenceForScan(t, store, "dec-old-debt", "ev-old-debt", "supports", baseTime.Add(-24*time.Hour))

	for index := 0; index < 500; index++ {
		decisionID := fmt.Sprintf("dec-buffer-%03d", index)
		title := fmt.Sprintf("buffer-%03d", index)
		createdAt := baseTime.Add(time.Duration(index+1) * time.Minute)
		createActiveDecisionForScan(t, store, decisionID, title, createdAt)
	}

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	debtItem := findStaleItemByCategory(items, StaleCategoryEpistemicDebtExceeded)
	if debtItem == nil {
		t.Fatal("expected debt alert beyond 500 newer decisions")
	}
	if len(debtItem.DecisionDebt) == 0 || debtItem.DecisionDebt[0].DecisionID != "dec-old-debt" {
		t.Fatalf("unexpected debt breakdown: %+v", debtItem.DecisionDebt)
	}
}

func TestScanStale_EmitsScanFailureItem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	wrapped := failingArtifactStore{
		ArtifactStore:      store,
		findStaleArtifacts: errors.New("stale artifacts unavailable"),
	}

	items, err := ScanStale(ctx, wrapped)
	if err != nil {
		t.Fatal(err)
	}

	failureItem := findStaleItemByCategory(items, StaleCategoryScanFailed)
	if failureItem == nil {
		t.Fatal("expected scan failure item")
	}
	if !strings.Contains(failureItem.Reason, "stale artifact scan failed") {
		t.Fatalf("unexpected failure reason: %q", failureItem.Reason)
	}
}

func createActiveDecisionForScan(t *testing.T, store *Store, decisionID, title string, createdAt time.Time) {
	t.Helper()

	err := store.Create(context.Background(), &Artifact{
		Meta: Meta{
			ID:        decisionID,
			Kind:      KindDecisionRecord,
			Status:    StatusActive,
			Title:     title,
			CreatedAt: createdAt,
		},
		Body: "# Decision\n\nscan fixture",
	})
	if err != nil {
		t.Fatalf("create decision %s: %v", decisionID, err)
	}
}

func addEvidenceForScan(t *testing.T, store *Store, decisionID, evidenceID, verdict string, validUntil time.Time) {
	t.Helper()

	err := store.AddEvidenceItem(context.Background(), &EvidenceItem{
		ID:              evidenceID,
		Type:            "measure",
		Content:         evidenceID,
		Verdict:         verdict,
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      validUntil.Format(time.RFC3339),
	}, decisionID)
	if err != nil {
		t.Fatalf("add evidence %s: %v", evidenceID, err)
	}
}

func findStaleItemByCategory(items []StaleItem, category StaleCategory) *StaleItem {
	for index := range items {
		if items[index].Category == category {
			return &items[index]
		}
	}

	return nil
}

type failingArtifactStore struct {
	ArtifactStore
	findStaleArtifacts error
	getEvidenceItems   error
}

func (s failingArtifactStore) FindStaleArtifacts(ctx context.Context) ([]*Artifact, error) {
	if s.findStaleArtifacts != nil {
		return nil, s.findStaleArtifacts
	}
	return s.ArtifactStore.FindStaleArtifacts(ctx)
}

func (s failingArtifactStore) GetEvidenceItems(ctx context.Context, artifactRef string) ([]EvidenceItem, error) {
	if s.getEvidenceItems != nil {
		return nil, s.getEvidenceItems
	}
	return s.ArtifactStore.GetEvidenceItems(ctx, artifactRef)
}

func TestScanStale_SurfacesPendingVerifyAfterClaims(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	pastDate := time.Now().Add(-48 * time.Hour).UTC().Format("2006-01-02")
	futureDate := time.Now().Add(7 * 24 * time.Hour).UTC().Format("2006-01-02")

	// Create a decision directly with structured_data containing claims with verify_after.
	claims := []DecisionClaim{
		{
			ID:          "claim-001",
			Claim:       "p99 drops to < 10ms",
			Observable:  "wrk benchmark",
			Threshold:   "p99 < 10ms",
			Status:      ClaimStatusUnverified,
			VerifyAfter: pastDate,
		},
		{
			ID:          "claim-002",
			Claim:       "error rate stable",
			Observable:  "grafana dashboard",
			Threshold:   "< 2%",
			Status:      ClaimStatusUnverified,
			VerifyAfter: futureDate,
		},
		{
			ID:         "claim-003",
			Claim:      "no regression on throughput",
			Observable: "load test",
			Threshold:  "> 1000 rps",
			Status:     ClaimStatusUnverified,
		},
	}

	structuredJSON, err := json.Marshal(DecisionFields{Claims: claims})
	if err != nil {
		t.Fatal(err)
	}

	validFuture := time.Now().Add(90 * 24 * time.Hour).UTC().Format(time.RFC3339)
	store.Create(ctx, &Artifact{
		Meta: Meta{
			ID:         "dec-test-va",
			Kind:       KindDecisionRecord,
			Title:      "Test verify_after",
			ValidUntil: validFuture,
		},
		Body:           "test decision",
		StructuredData: string(structuredJSON),
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	var pendingItems []StaleItem
	for _, item := range items {
		if item.ID == "dec-test-va" && item.Category == StaleCategoryPendingVerification {
			pendingItems = append(pendingItems, item)
		}
	}

	if len(pendingItems) != 1 {
		t.Fatalf("expected 1 pending verification item (past date only), got %d. All items: %+v", len(pendingItems), items)
	}

	if !strings.Contains(pendingItems[0].Reason, "claim-001") {
		t.Errorf("expected pending item to reference claim-001, got reason: %s", pendingItems[0].Reason)
	}
	if !strings.Contains(pendingItems[0].Reason, "wrk benchmark") {
		t.Errorf("expected pending item to include observable, got reason: %s", pendingItems[0].Reason)
	}
}

func TestScanStale_SurfacesPendingVerifyAfterClaimsForRefreshDueDecision(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	verifyAfter := time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339)
	structuredJSON, err := json.Marshal(DecisionFields{
		Claims: []DecisionClaim{
			{
				ID:          "claim-001",
				Claim:       "desktop verification remains possible",
				Observable:  "verification task output",
				Threshold:   "measurement recorded",
				Status:      ClaimStatusUnverified,
				VerifyAfter: verifyAfter,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	store.Create(ctx, &Artifact{
		Meta: Meta{
			ID:         "dec-refresh-due",
			Kind:       KindDecisionRecord,
			Status:     StatusRefreshDue,
			Title:      "Refresh due verification candidate",
			ValidUntil: time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339),
		},
		Body:           "test decision",
		StructuredData: string(structuredJSON),
	})

	items, err := ScanStale(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, item := range items {
		if item.ID == "dec-refresh-due" && item.Category == StaleCategoryPendingVerification {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected refresh_due decision to surface pending verification finding, got %+v", items)
	}
}
