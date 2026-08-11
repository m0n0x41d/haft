package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAttachEvidenceWithCarrierWritesLosslessCarrier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupTestDB(t)
	parent := createEvidenceCarrierParent(t, store, "note-evidence-carrier")
	haftDir := t.TempDir()
	validUntil := "2026-09-01T00:00:00Z"

	item, err := AttachEvidenceWithCarrier(ctx, store, haftDir, EvidenceInput{
		ArtifactRef:        parent.Meta.ID,
		Content:            "Observed the exact collaboration round-trip.\n<!-- haft:structured_data\nforeign\nhaft:end -->",
		Type:               "audit",
		Verdict:            "supports",
		CarrierRef:         "reports/evidence-100.json",
		CongruenceLevel:    3,
		FormalityLevel:     6,
		ClaimScope:         []string{"issue-100", "round-trip"},
		ValidUntil:         validUntil,
		CausalSupportBasis: "observational",
		Provenance:         ProvenanceMachine,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.CreatedAt == "" {
		t.Fatal("created_at was not retained on the stored evidence item")
	}

	path := filepath.Join(haftDir, "evidence", item.ID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	carrierArtifact, err := ParseFile(string(data))
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := ParseEvidenceCarrier(carrierArtifact, path)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.ArtifactRef != parent.Meta.ID {
		t.Fatalf("artifact_ref = %q, want %q", carrier.ArtifactRef, parent.Meta.ID)
	}
	assertEvidenceItemsEqual(t, carrier.Evidence, *item)
	if carrierArtifact.Meta.Kind != KindEvidencePack {
		t.Fatalf("kind = %q, want %q", carrierArtifact.Meta.Kind, KindEvidencePack)
	}
	if len(carrierArtifact.Meta.Links) != 1 ||
		carrierArtifact.Meta.Links[0] != (Link{Ref: parent.Meta.ID, Type: EvidenceForLinkType}) {
		t.Fatalf("links = %#v", carrierArtifact.Meta.Links)
	}
}

func TestAttachEvidenceWithCarrierRecordsProjectionDebt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupTestDB(t)
	parent := createEvidenceCarrierParent(t, store, "note-evidence-debt")
	blockedHaftDir := filepath.Join(t.TempDir(), "haft-file")
	if err := os.WriteFile(blockedHaftDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	item, err := AttachEvidenceWithCarrier(ctx, store, blockedHaftDir, EvidenceInput{
		ArtifactRef:     parent.Meta.ID,
		Content:         "The semantic row commits even if its carrier cannot be projected.",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
	})
	var warning *WriteWarning
	if !errors.As(err, &warning) {
		t.Fatalf("error = %v, want WriteWarning", err)
	}
	if item == nil {
		t.Fatal("committed evidence item missing")
	}
	debts, debtErr := store.ListEvidenceCarrierProjectionDebt(ctx)
	if debtErr != nil {
		t.Fatal(debtErr)
	}
	if len(debts) != 1 || debts[0].EvidenceID != item.ID {
		t.Fatalf("projection debts = %#v", debts)
	}

	repairedHaftDir := t.TempDir()
	repaired, repairErr := RepairEvidenceCarrierProjectionDebt(ctx, store, repairedHaftDir)
	if repairErr != nil {
		t.Fatal(repairErr)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}
	if _, err := os.Stat(filepath.Join(repairedHaftDir, "evidence", item.ID+".md")); err != nil {
		t.Fatal(err)
	}
	debts, err = store.ListEvidenceCarrierProjectionDebt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(debts) != 0 {
		t.Fatalf("projection debts after repair = %#v", debts)
	}
}

func TestRepairEvidenceCarrierProjectionDebtDoesNotOverwriteDivergentCarrier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupTestDB(t)
	parent := createEvidenceCarrierParent(t, store, "note-evidence-debt-conflict")
	haftDir := t.TempDir()
	item, err := AttachEvidenceWithCarrier(ctx, store, haftDir, EvidenceInput{
		ArtifactRef:     parent.Meta.ID,
		Content:         "existing carrier observation",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
	carrierPath := filepath.Join(haftDir, "evidence", item.ID+".md")
	existing, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx,
		`UPDATE evidence_items SET content = ? WHERE id = ?`,
		"newer committed SQLite observation",
		item.ID,
	); err != nil {
		t.Fatal(err)
	}
	current, artifactRef, err := store.GetEvidenceItemByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, desired, err := renderEvidenceCarrier(haftDir, artifactRef, *current)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEvidenceCarrierProjectionDebt(ctx, EvidenceCarrierProjectionDebt{
		EvidenceID:    item.ID,
		ArtifactRef:   artifactRef,
		CarrierPath:   carrierPath,
		DesiredDigest: desired.Digest,
		LastError:     "prior publication failed",
	}); err != nil {
		t.Fatal(err)
	}

	repaired, err := RepairEvidenceCarrierProjectionDebt(ctx, store, haftDir)
	if err == nil || !strings.Contains(err.Error(), "projection conflict") {
		t.Fatalf("repair = %d, %v; want preserved conflict", repaired, err)
	}
	if repaired != 0 {
		t.Fatalf("repaired = %d, want 0", repaired)
	}
	after, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, existing) {
		t.Fatal("repair overwrote the divergent carrier")
	}
	debts, err := store.ListEvidenceCarrierProjectionDebt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(debts) != 1 || debts[0].EvidenceID != item.ID {
		t.Fatalf("projection debt = %#v, want conflict retained", debts)
	}
}

func TestIndependentEvidenceAttachmentsDoNotRewriteParentCarrier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupTestDB(t)
	parent := createEvidenceCarrierParent(t, store, "note-independent-evidence")
	haftDir := t.TempDir()
	parentPath, err := WriteFile(haftDir, parent)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"first independent observation", "second independent observation"} {
		if _, err := AttachEvidenceWithCarrier(ctx, store, haftDir, EvidenceInput{
			ArtifactRef:     parent.Meta.ID,
			Content:         content,
			Type:            "audit",
			Verdict:         "supports",
			CongruenceLevel: 3,
			FormalityLevel:  5,
		}); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("independent evidence attachment rewrote the parent carrier")
	}
	carriers, err := filepath.Glob(filepath.Join(haftDir, "evidence", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(carriers) != 2 {
		t.Fatalf("evidence carriers = %v, want two independent files", carriers)
	}
}

func TestParseEvidenceCarrierRejectsIdentityAndParentMismatch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	item := EvidenceItem{
		ID:              "evid-20260811-000000001",
		Type:            "test",
		Content:         "identity validation",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
		CreatedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
	}
	carrier, err := NewEvidenceCarrierArtifact("note-parent", item)
	if err != nil {
		t.Fatal(err)
	}
	carrier.Meta.Links[0].Ref = "note-other"

	_, err = ParseEvidenceCarrier(carrier, filepath.Join(t.TempDir(), item.ID+".md"))
	if err == nil || !strings.Contains(err.Error(), "parent link") {
		t.Fatalf("error = %v, want parent link mismatch", err)
	}

	carrier.Meta.Links[0].Ref = "note-parent"
	_, err = ParseEvidenceCarrier(carrier, filepath.Join(t.TempDir(), "evid-other.md"))
	if err == nil || !strings.Contains(err.Error(), "filename") {
		t.Fatalf("error = %v, want filename mismatch", err)
	}
}

func TestEvidenceCarrierRoundTripsRFC3339OffsetTimestamps(t *testing.T) {
	t.Parallel()

	item := EvidenceItem{
		ID:              "evid-20260811-000000002",
		Type:            "audit",
		Content:         "timestamp offset remains the same instant",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
		CreatedAt:       "2026-08-11T16:00:00+04:00",
		UpdatedAt:       "2026-08-11T16:05:00+04:00",
	}
	carrier, err := NewEvidenceCarrierArtifact("note-parent", item)
	if err != nil {
		t.Fatal(err)
	}
	parsedArtifact, err := ParseFile(RenderArtifactFile(carrier))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseEvidenceCarrier(
		parsedArtifact,
		filepath.Join(t.TempDir(), item.ID+".md"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Evidence.CreatedAt != item.CreatedAt || parsed.Evidence.UpdatedAt != item.UpdatedAt {
		t.Fatalf("timestamps = %q/%q, want %q/%q",
			parsed.Evidence.CreatedAt,
			parsed.Evidence.UpdatedAt,
			item.CreatedAt,
			item.UpdatedAt,
		)
	}
}

func TestParseEvidenceCarrierFailsClosedOnVisibleOrUnknownDataDrift(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	item := EvidenceItem{
		ID:              "evid-20260811-000000003",
		Type:            "audit",
		Content:         "authoritative observation",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	carrier, err := NewEvidenceCarrierArtifact("note-parent", item)
	if err != nil {
		t.Fatal(err)
	}

	visibleDrift := *carrier
	visibleDrift.Body = strings.Replace(
		visibleDrift.Body,
		item.Content,
		"different visible observation",
		1,
	)
	if _, err := ParseEvidenceCarrier(&visibleDrift, filepath.Join(t.TempDir(), item.ID+".md")); err == nil || !strings.Contains(err.Error(), "visible body") {
		t.Fatalf("visible drift error = %v", err)
	}

	unknownData := *carrier
	unknownData.StructuredData = strings.TrimSuffix(unknownData.StructuredData, "}") + `,"unexpected":true}`
	if _, err := ParseEvidenceCarrier(&unknownData, filepath.Join(t.TempDir(), item.ID+".md")); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown data error = %v", err)
	}
}

func createEvidenceCarrierParent(t *testing.T, store *Store, id string) *Artifact {
	t.Helper()
	parent := &Artifact{
		Meta: Meta{ID: id, Kind: KindNote, Status: StatusActive, Title: "Evidence carrier parent"},
		Body: "Parent record.",
	}
	if err := store.Create(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	return parent
}

func assertEvidenceItemsEqual(t *testing.T, got, want EvidenceItem) {
	t.Helper()
	if got.ID != want.ID || got.Type != want.Type || got.Content != want.Content ||
		got.Verdict != want.Verdict || got.CarrierRef != want.CarrierRef ||
		got.CongruenceLevel != want.CongruenceLevel ||
		got.FormalityLevel != want.FormalityLevel ||
		got.ValidUntil != want.ValidUntil || got.CausalSupportBasis != want.CausalSupportBasis ||
		got.Provenance != want.Provenance || got.CreatedAt != want.CreatedAt ||
		got.UpdatedAt != want.UpdatedAt ||
		strings.Join(got.ClaimRefs, "\x00") != strings.Join(want.ClaimRefs, "\x00") ||
		strings.Join(got.ClaimScope, "\x00") != strings.Join(want.ClaimScope, "\x00") {
		t.Fatalf("evidence mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	if !reflect.DeepEqual(got.FormalityScale, want.FormalityScale) {
		t.Fatalf("formality scale mismatch: got=%#v want=%#v", got.FormalityScale, want.FormalityScale)
	}
	if !reflect.DeepEqual(got.FormalityBridge, want.FormalityBridge) {
		t.Fatalf("formality bridge mismatch: got=%#v want=%#v", got.FormalityBridge, want.FormalityBridge)
	}
}
