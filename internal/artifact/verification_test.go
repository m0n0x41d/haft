package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRecordVerificationPassBaselinesFilesAndAttachesCL3Evidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "internal/artifact/verification.go", "package artifact\n")
	writeTestFile(t, projectRoot, "desktop/agents.go", "package main\n")

	decision := createTestDecision(t, store, "dec-verify-pass", "Verification pass")
	decision.Meta.ValidUntil = time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	if err := store.Update(ctx, decision); err != nil {
		t.Fatalf("Update decision: %v", err)
	}

	files := []AffectedFile{
		{Path: "internal/artifact/verification.go"},
		{Path: "desktop/agents.go"},
	}
	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, files); err != nil {
		t.Fatalf("SetAffectedFiles: %v", err)
	}

	result, err := RecordVerificationPass(ctx, store, projectRoot, VerificationPassInput{
		DecisionRef: decision.Meta.ID,
		CarrierRef:  "desktop-task:task-42",
		Summary:     "Task task-42 completed on branch feat/verification-pass.",
	})
	if err != nil {
		t.Fatalf("RecordVerificationPass: %v", err)
	}

	if len(result.Baseline) != 2 {
		t.Fatalf("baseline files = %d, want 2", len(result.Baseline))
	}

	for _, file := range result.Baseline {
		if file.Hash == "" {
			t.Fatalf("baseline hash missing for %s", file.Path)
		}
	}

	if result.Evidence == nil {
		t.Fatal("expected evidence item")
	}
	if result.Evidence.Type != "audit" {
		t.Fatalf("evidence type = %q, want audit", result.Evidence.Type)
	}
	if result.Evidence.Verdict != "supports" {
		t.Fatalf("evidence verdict = %q, want supports", result.Evidence.Verdict)
	}
	if result.Evidence.CongruenceLevel != 3 {
		t.Fatalf("evidence congruence = %d, want 3", result.Evidence.CongruenceLevel)
	}
	if result.Evidence.FormalityLevel != 2 {
		t.Fatalf("evidence formality = %d, want 2", result.Evidence.FormalityLevel)
	}
	if result.Evidence.CarrierRef != "desktop-task:task-42" {
		t.Fatalf("evidence carrier_ref = %q, want desktop-task:task-42", result.Evidence.CarrierRef)
	}
	if result.Evidence.ValidUntil != decision.Meta.ValidUntil {
		t.Fatalf("evidence valid_until = %q, want %q", result.Evidence.ValidUntil, decision.Meta.ValidUntil)
	}
	if !strings.Contains(result.Evidence.Content, "Baselined files (2):") {
		t.Fatalf("evidence content missing baseline summary: %q", result.Evidence.Content)
	}
	if !strings.Contains(result.Evidence.Content, "internal/artifact/verification.go") {
		t.Fatalf("evidence content missing verification.go path: %q", result.Evidence.Content)
	}
	if !strings.Contains(result.Evidence.Content, "desktop/agents.go") {
		t.Fatalf("evidence content missing agents.go path: %q", result.Evidence.Content)
	}
	if !strings.Contains(result.Evidence.Content, "Task task-42 completed on branch feat/verification-pass.") {
		t.Fatalf("evidence content missing task summary: %q", result.Evidence.Content)
	}

	storedFiles, err := store.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatalf("GetAffectedFiles: %v", err)
	}
	for _, file := range storedFiles {
		if file.Hash == "" {
			t.Fatalf("stored baseline hash missing for %s", file.Path)
		}
	}

	items, err := store.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatalf("GetEvidenceItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("evidence items = %d, want 1", len(items))
	}
	if items[0].ID != result.Evidence.ID {
		t.Fatalf("stored evidence id = %q, want %q", items[0].ID, result.Evidence.ID)
	}
}

func TestRecordVerificationPassStopsBeforeEvidenceWhenBaselineFails(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	decision := createTestDecision(t, store, "dec-verify-missing", "Missing file")
	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, []AffectedFile{{Path: "missing.go"}}); err != nil {
		t.Fatalf("SetAffectedFiles: %v", err)
	}

	// Missing files are now skipped gracefully in Baseline.
	// RecordVerificationPass should succeed (baseline returns 0 files, evidence still recorded).
	_, err := RecordVerificationPass(ctx, store, projectRoot, VerificationPassInput{
		DecisionRef: decision.Meta.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v (missing files should be skipped)", err)
	}
}
