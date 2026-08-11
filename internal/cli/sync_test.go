package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestSyncOneFile_RestoresProblemSemanticEnvelopeFromCarrierBlock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()

	problem, filePath, err := artifact.FrameProblem(ctx, sourceStore, haftDir, artifact.ProblemFrameInput{
		Title:      "Semantic sync",
		Signal:     "Markdown carrier must restore structured_data into SQLite.",
		Acceptance: "Imported ProblemCard keeps exact semantic envelope.",
	})
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	result, err := syncOneFile(ctx, importStore, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "created" {
		t.Fatalf("sync result = %q, want created", result)
	}

	imported, err := importStore.Get(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := imported.UnmarshalProblemFields()
	if fields.Semantic == nil {
		t.Fatal("imported semantic envelope missing")
	}
	if fields.Semantic.Status != artifact.SemanticStatusExact {
		t.Fatalf("semantic status = %q, want exact", fields.Semantic.Status)
	}
	if fields.Signal != "Markdown carrier must restore structured_data into SQLite." {
		t.Fatalf("signal = %q", fields.Signal)
	}
	if fields.Semantic.PublicationUnit.SourceEditionPin.Hash != fields.Semantic.SemanticEdition.Hash {
		t.Fatalf("source edition pin = %q, want %q", fields.Semantic.PublicationUnit.SourceEditionPin.Hash, fields.Semantic.SemanticEdition.Hash)
	}
	if fields.Semantic.PublicationUnit.PublicationHash == "" || fields.Semantic.PublicationUnit.CarrierHash == "" {
		t.Fatalf("publication unit missing hashes: %+v", fields.Semantic.PublicationUnit)
	}
}

func TestSyncOneFile_RoundTripsRichProblemSemanticIdentityFromCarrier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()

	problem, filePath, err := artifact.FrameProblem(ctx, sourceStore, haftDir, artifact.ProblemFrameInput{
		Title:                 "Rich semantic identity",
		ProblemType:           string(artifact.ProblemTypeSynthesis),
		ProblemProfile:        artifact.ProblemProfileDeep,
		SourceKind:            artifact.ProblemSourceObserved,
		Signal:                "Markdown carrier import must preserve typed semantic identity fields.",
		WhyNow:                "Semantic-spine slices now rely on the carrier as a recoverable projection.",
		Scope:                 "ProblemCard structured_data round-trip only.",
		AcceptanceProbe:       "Empty-store import reconstructs the same typed problem profile and semantic envelope.",
		FreshnessDisposition:  "Reopen if carrier import drops profile, source, or publication-unit fields.",
		Constraints:           []string{"do not promote markdown prose to authority"},
		OptimizationTargets:   []string{"semantic identity loss rate"},
		ObservationIndicators: []string{"legacy carrier warning count"},
		Acceptance:            "Imported structured_data keeps profile and publication source pin.",
		BlastRadius:           "ProblemCard sync only.",
		Reversibility:         "Test-only guard.",
	})
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	result, err := syncOneFile(ctx, importStore, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "created" {
		t.Fatalf("sync result = %q, want created", result)
	}

	imported, err := importStore.Get(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := imported.UnmarshalProblemFields()
	if fields.ProblemType != artifact.ProblemTypeSynthesis {
		t.Fatalf("problem_type = %q, want synthesis", fields.ProblemType)
	}
	if fields.Profile == nil {
		t.Fatal("profile missing after carrier import")
	}
	if fields.Profile.Level != artifact.ProblemProfileDeep {
		t.Fatalf("profile.level = %q, want deep", fields.Profile.Level)
	}
	if fields.Profile.Readiness != artifact.ProblemReadinessReady {
		t.Fatalf("profile.readiness = %q, want ready", fields.Profile.Readiness)
	}
	if fields.Profile.BoundaryStatus != artifact.ProblemBoundaryExplicit {
		t.Fatalf("profile.boundary_status = %q, want explicit", fields.Profile.BoundaryStatus)
	}
	if fields.Profile.WhyNow != "Semantic-spine slices now rely on the carrier as a recoverable projection." {
		t.Fatalf("profile.why_now = %q", fields.Profile.WhyNow)
	}
	if !slices.Equal(fields.Constraints, []string{"do not promote markdown prose to authority"}) {
		t.Fatalf("constraints = %#v", fields.Constraints)
	}
	if !slices.Equal(fields.OptimizationTargets, []string{"semantic identity loss rate"}) {
		t.Fatalf("optimization_targets = %#v", fields.OptimizationTargets)
	}
	if !slices.Equal(fields.ObservationIndicators, []string{"legacy carrier warning count"}) {
		t.Fatalf("observation_indicators = %#v", fields.ObservationIndicators)
	}
	if fields.Semantic == nil {
		t.Fatal("semantic envelope missing after carrier import")
	}
	if fields.Semantic.Status != artifact.SemanticStatusExact {
		t.Fatalf("semantic.status = %q, want exact", fields.Semantic.Status)
	}
	if fields.Semantic.CarrierBinding.SourceOfTruth != "sqlite" {
		t.Fatalf("source_of_truth = %q, want sqlite", fields.Semantic.CarrierBinding.SourceOfTruth)
	}
	if fields.Semantic.PublicationUnit.SourceEditionPin.Hash == "" {
		t.Fatal("source edition pin hash missing")
	}
	if fields.Semantic.PublicationUnit.SourceEditionPin.Hash != fields.Semantic.SemanticEdition.Hash {
		t.Fatalf("source edition pin = %q, want semantic edition hash %q",
			fields.Semantic.PublicationUnit.SourceEditionPin.Hash,
			fields.Semantic.SemanticEdition.Hash)
	}
	if fields.Semantic.PublicationUnit.Recoverability.Status != "exact" {
		t.Fatalf("recoverability.status = %q, want exact", fields.Semantic.PublicationUnit.Recoverability.Status)
	}
}

func TestSyncOneFile_ImportsLegacyProblemAsLegacyDegraded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	filePath := filepath.Join(t.TempDir(), "prob-legacy.md")
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	content := `---
id: prob-legacy
kind: ProblemCard
version: 1
status: active
title: Legacy problem
created_at: ` + now + `
updated_at: ` + now + `
---

# Legacy problem

## Signal

Old markdown has no structured data carrier block.
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := syncOneFile(ctx, store, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "created" {
		t.Fatalf("sync result = %q, want created", result)
	}

	imported, err := store.Get(ctx, "prob-legacy")
	if err != nil {
		t.Fatal(err)
	}
	fields := imported.UnmarshalProblemFields()
	if fields.Signal != "Old markdown has no structured data carrier block." {
		t.Fatalf("signal = %q", fields.Signal)
	}
	if fields.Semantic == nil {
		t.Fatal("legacy semantic envelope missing")
	}
	if fields.Semantic.Status != artifact.SemanticStatusLegacy {
		t.Fatalf("semantic status = %q, want legacy", fields.Semantic.Status)
	}
	if len(fields.Semantic.Warnings) == 0 {
		t.Fatal("legacy import should carry audit warning")
	}
	if len(fields.Semantic.PublicationUnit.Losses) == 0 {
		t.Fatalf("legacy import should expose publication loss: %+v", fields.Semantic.PublicationUnit)
	}
}

func TestSyncOneFile_AddsCarrierLinksEvenWhenArtifactRowIsUnchanged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	target := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "prob-sync-link-target",
			Kind:   artifact.KindProblemCard,
			Status: artifact.StatusActive,
			Title:  "Sync link target",
		},
		Body: "# Sync link target",
	}
	if err := store.Create(ctx, target); err != nil {
		t.Fatal(err)
	}

	source := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "sol-sync-link-source",
			Kind:   artifact.KindSolutionPortfolio,
			Status: artifact.StatusActive,
			Title:  "Sync link source",
		},
		Body: "# Sync link source",
	}
	if err := store.Create(ctx, source); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(t.TempDir(), "sol-sync-link-source.md")
	stale := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	content := `---
id: sol-sync-link-source
kind: SolutionPortfolio
version: 1
status: active
title: Sync link source
created_at: ` + stale + `
updated_at: ` + stale + `
links:
  - ref: prob-sync-link-target
    type: based_on
---

# Sync link source
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := syncOneFile(ctx, store, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if result != "updated" {
		t.Fatalf("sync result = %q, want updated", result)
	}

	backlinks, err := store.GetBacklinks(ctx, target.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(backlinks, func(link artifact.Link) bool {
		return link.Ref == source.Meta.ID && link.Type == "based_on"
	}) {
		t.Fatalf("missing synced backlink: %#v", backlinks)
	}
}

func TestSyncOneFile_RejectsUnknownProblemSemanticSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	filePath := filepath.Join(t.TempDir(), "prob-unknown-schema.md")
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	content := `---
id: prob-unknown-schema
kind: ProblemCard
version: 1
status: active
title: Unknown schema
created_at: ` + now + `
updated_at: ` + now + `
---

# Unknown schema

## Signal

Do not import future semantic envelopes as exact.
` + artifact.RenderStructuredDataBlock(`{
  "signal": "Do not import future semantic envelopes as exact.",
  "semantic": {
    "schema_version": 999,
    "status": "exact"
  }
}`)
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := syncOneFile(ctx, store, filePath)
	if err == nil {
		t.Fatal("expected unknown schema sync to fail closed")
	}
	if !strings.Contains(err.Error(), "unsupported problem semantic schema_version 999") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncOneFile_RoundTripsEvidenceCarrierWithoutDerivedDrift(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()
	predictionProbability := 0.9
	decision, parentPath, err := artifact.Decide(ctx, sourceStore, haftDir, artifact.DecideInput{
		SelectedTitle:    "Preserve evidence across collaborator ledgers",
		ProblemStatement: "Evidence disappears when only the local SQLite row exists.",
		WhySelected:      "A fresh ledger must reconstruct the exact evidence record and derived posture.",
		SelectionPolicy:  "Require exact round-trip and stable derived evidence state.",
		CounterArgument:  "A per-record carrier adds files to the repository.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Keep evidence only in SQLite",
			Reason:  "A collaborator cannot reconstruct it from git.",
		}},
		WeakestLink: "The parent carrier must exist before evidence import.",
		Predictions: []artifact.PredictionInput{{
			Claim:       "Evidence survives a clean ledger rebuild.",
			Observable:  "SQLite to Markdown to SQLite round-trip",
			Threshold:   "Exact equality for stored and derived evidence state",
			Probability: &predictionProbability,
			VerifyAfter: "2026-08-25",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Evidence round-trip changes a stored or derived value"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	item, err := artifact.AttachEvidenceWithCarrier(ctx, sourceStore, haftDir, artifact.EvidenceInput{
		ArtifactRef:        decision.Meta.ID,
		Content:            "Two-clone replay preserved the observation.",
		Type:               "measurement",
		Verdict:            "accepted",
		CarrierRef:         "reports/issue-100.json",
		CongruenceLevel:    2,
		FormalityLevel:     6,
		ClaimRefs:          []string{"claim-001"},
		ClaimScope:         []string{"Evidence survives a clean ledger rebuild."},
		ValidUntil:         "2026-09-01T00:00:00Z",
		CausalSupportBasis: "observational",
		Provenance:         artifact.ProvenanceMachine,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := artifact.ComputeWLNKSummary(ctx, sourceStore, decision.Meta.ID)
	beforeHealth := artifact.DeriveDecisionHealth(ctx, sourceStore, decision.Meta.ID)
	freshnessAt := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	beforeFreshness, err := artifact.BuildEvidenceFreshnessInventory(ctx, sourceStore, freshnessAt)
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	if result, err := syncOneFile(ctx, importStore, parentPath); err != nil || result != "created" {
		t.Fatalf("parent sync = %q, %v", result, err)
	}
	evidencePath := filepath.Join(haftDir, "evidence", item.ID+".md")
	if result, err := syncOneFile(ctx, importStore, evidencePath); err != nil || result != "created" {
		t.Fatalf("evidence sync = %q, %v", result, err)
	}
	if result, err := syncOneFile(ctx, importStore, evidencePath); err != nil || result != "unchanged" {
		t.Fatalf("idempotent evidence sync = %q, %v", result, err)
	}

	items, err := importStore.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("imported evidence count = %d, want 1", len(items))
	}
	if items[0].ID != item.ID || items[0].Content != item.Content ||
		items[0].Verdict != item.Verdict || items[0].CarrierRef != item.CarrierRef ||
		items[0].CongruenceLevel != item.CongruenceLevel ||
		items[0].FormalityLevel != item.FormalityLevel ||
		items[0].ValidUntil != item.ValidUntil ||
		items[0].CausalSupportBasis != item.CausalSupportBasis ||
		items[0].Provenance != item.Provenance || items[0].CreatedAt != item.CreatedAt ||
		items[0].UpdatedAt != item.UpdatedAt ||
		!slices.Equal(items[0].ClaimRefs, item.ClaimRefs) ||
		!slices.Equal(items[0].ClaimScope, item.ClaimScope) ||
		!reflect.DeepEqual(items[0].FormalityScale, item.FormalityScale) ||
		!reflect.DeepEqual(items[0].FormalityBridge, item.FormalityBridge) {
		t.Fatalf("round-trip mismatch:\n got: %#v\nwant: %#v", items[0], *item)
	}
	after := artifact.ComputeWLNKSummary(ctx, importStore, decision.Meta.ID)
	if before.REff != after.REff || before.FEff != after.FEff ||
		!slices.Equal(before.GEff, after.GEff) || before.MinFreshness != after.MinFreshness {
		t.Fatalf("derived posture drifted:\n before=%#v\n after=%#v", before, after)
	}
	afterHealth := artifact.DeriveDecisionHealth(ctx, importStore, decision.Meta.ID)
	if beforeHealth != afterHealth {
		t.Fatalf("decision health drifted: before=%#v after=%#v", beforeHealth, afterHealth)
	}
	afterFreshness, err := artifact.BuildEvidenceFreshnessInventory(ctx, importStore, freshnessAt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeFreshness, afterFreshness) {
		t.Fatalf("freshness inventory drifted: before=%#v after=%#v", beforeFreshness, afterFreshness)
	}
}

func TestSyncOneFile_EvidenceCarrierFailsClosedForMissingAuthorityParent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()
	parent := &artifact.Artifact{
		Meta: artifact.Meta{
			ID: "wc-existing-only-at-source", Kind: artifact.KindWorkCommission,
			Status: artifact.StatusActive, Title: "Authority parent",
		},
		Body: "Existing authority-bearing parent.",
	}
	if err := sourceStore.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	item, err := artifact.AttachEvidenceWithCarrier(ctx, sourceStore, haftDir, artifact.EvidenceInput{
		ArtifactRef: parent.Meta.ID, Content: "Execution observation.", Type: "audit",
		Verdict: "supports", CongruenceLevel: 3, FormalityLevel: 6,
	})
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	evidencePath := filepath.Join(haftDir, "evidence", item.ID+".md")
	_, err = syncOneFile(ctx, importStore, evidencePath)
	if err == nil || !strings.Contains(err.Error(), "parent") {
		t.Fatalf("error = %v, want missing-parent failure", err)
	}
	if _, getErr := importStore.Get(ctx, parent.Meta.ID); getErr == nil {
		t.Fatal("sync created the absent authority parent")
	}
	if _, getErr := importStore.Get(ctx, item.ID); getErr == nil {
		t.Fatal("sync imported evidence carrier as a generic artifact")
	}
	items, getErr := importStore.GetEvidenceItems(ctx, parent.Meta.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if len(items) != 0 {
		t.Fatalf("evidence rows = %#v, want none", items)
	}
}

func TestSyncOneFile_EvidenceCarrierPreservesExactProjectionConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	parent := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "note-evidence-projection-conflict",
			Kind:   artifact.KindNote,
			Status: artifact.StatusActive,
			Title:  "Projection conflict parent",
		},
		Body: "Parent already present in the receiving ledger.",
	}
	if err := store.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	stored, err := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     parent.Meta.ID,
		Content:         "committed SQLite observation",
		Type:            "audit",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
	pulled := *stored
	pulled.Content = "different pulled carrier observation"
	carrier, err := artifact.NewEvidenceCarrierArtifact(parent.Meta.ID, pulled)
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(t.TempDir(), stored.ID+".md")
	content := []byte(artifact.RenderArtifactFile(carrier))
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEvidenceCarrierProjectionDebt(ctx, artifact.EvidenceCarrierProjectionDebt{
		EvidenceID:    stored.ID,
		ArtifactRef:   parent.Meta.ID,
		CarrierPath:   filePath,
		DesiredDigest: "sha256:" + strings.Repeat("0", 64),
		LastError:     "prior publication failed",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = syncOneFile(ctx, store, filePath)
	if err == nil || !strings.Contains(err.Error(), "projection conflict") {
		t.Fatalf("sync error = %v, want projection conflict", err)
	}
	current, _, err := store.GetEvidenceItemByID(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Content != stored.Content {
		t.Fatalf("SQLite content = %q, want preserved %q", current.Content, stored.Content)
	}
	after, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, content) {
		t.Fatal("sync overwrote the conflicting carrier")
	}
	debts, err := store.ListEvidenceCarrierProjectionDebt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(debts) != 1 || debts[0].EvidenceID != stored.ID {
		t.Fatalf("projection debt = %#v, want conflict retained", debts)
	}
}

func TestSyncOneFile_RepairsLegacyGenericEvidencePackGhost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sourceStore := setupCLIArtifactStore(t)
	haftDir := t.TempDir()
	problem, parentPath, err := artifact.FrameProblem(ctx, sourceStore, haftDir, artifact.ProblemFrameInput{
		Title:      "Legacy evidence ghost",
		Signal:     "A 9.0.x sync can import a 9.1 evidence carrier as a generic artifact.",
		Acceptance: "A 9.1 sync removes only the exact matching ghost and imports the EvidenceRecord.",
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := artifact.AttachEvidenceWithCarrier(ctx, sourceStore, haftDir, artifact.EvidenceInput{
		ArtifactRef: problem.Meta.ID, Content: "Ghost repair fixture.", Type: "test",
		Verdict: "supports", CongruenceLevel: 3, FormalityLevel: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(haftDir, "evidence", item.ID+".md")
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	ghost, err := artifact.ParseFile(string(data))
	if err != nil {
		t.Fatal(err)
	}

	importStore := setupCLIArtifactStore(t)
	if _, err := syncOneFile(ctx, importStore, parentPath); err != nil {
		t.Fatal(err)
	}
	if err := importStore.Create(ctx, ghost); err != nil {
		t.Fatalf("create legacy generic ghost: %v", err)
	}
	if result, err := syncOneFile(ctx, importStore, evidencePath); err != nil || result != "created" {
		t.Fatalf("repair sync = %q, %v", result, err)
	}
	if _, err := importStore.Get(ctx, item.ID); err == nil {
		t.Fatal("legacy generic EvidencePack artifact survived repair")
	}
	items, err := importStore.GetEvidenceItems(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("imported evidence = %#v", items)
	}
}

func TestRunSyncImportsPulledEvidenceBeforeRepairingProjectionDebt(t *testing.T) {
	ctx := context.Background()
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_7fac7fb0")
	ledger, err := openCurrentProjectLedger(
		ctx,
		fixture.binding.ProjectRoot,
		projectledger.ReadWrite,
		"sync ordering test setup",
	)
	if err != nil {
		t.Fatal(err)
	}
	store := artifact.NewStore(ledger.Database())
	parent := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "note-sync-debt-parent",
			Kind:   artifact.KindNote,
			Status: artifact.StatusActive,
			Title:  "Sync debt parent",
		},
		Body: "Existing parent in the local projection.",
	}
	if err := store.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	item, err := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     parent.Meta.ID,
		Content:         "stale local projection",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
	})
	if err != nil {
		t.Fatal(err)
	}
	carrierPath := filepath.Join(
		fixture.binding.ProjectRoot,
		".haft",
		"evidence",
		item.ID+".md",
	)
	if err := store.RecordEvidenceCarrierProjectionDebt(ctx, artifact.EvidenceCarrierProjectionDebt{
		EvidenceID:  item.ID,
		ArtifactRef: parent.Meta.ID,
		CarrierPath: carrierPath,
		LastError:   "legacy backfill required",
	}); err != nil {
		t.Fatal(err)
	}
	pulled := *item
	pulled.Content = "new observation pulled from a collaborator"
	pulled.UpdatedAt = time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	carrier, err := artifact.NewEvidenceCarrierArtifact(parent.Meta.ID, pulled)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(carrierPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carrierPath, []byte(artifact.RenderArtifactFile(carrier)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	restore := enterTestProjectRoot(t, fixture.binding.ProjectRoot)
	runContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := &cobra.Command{}
	command.SetContext(runContext)
	err = runSync(command, nil)
	restore()
	if err != nil {
		t.Fatal(err)
	}

	ledger, err = openCurrentProjectLedger(
		ctx,
		fixture.binding.ProjectRoot,
		projectledger.ReadWrite,
		"sync ordering test verification",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	store = artifact.NewStore(ledger.Database())
	imported, _, err := store.GetEvidenceItemByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if imported.Content != pulled.Content || imported.UpdatedAt != pulled.UpdatedAt {
		t.Fatalf("imported evidence = %#v, want pulled carrier %#v", imported, pulled)
	}
	debts, err := store.ListEvidenceCarrierProjectionDebt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(debts) != 0 {
		t.Fatalf("projection debt after sync = %#v, want none", debts)
	}
	data, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), pulled.Content) {
		t.Fatalf("carrier was overwritten with stale DB content:\n%s", data)
	}
}

func TestRunSyncImportsOrdinaryParentBeforeEvidenceCarrier(t *testing.T) {
	ctx := context.Background()
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_7fac7fb1")
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	parent := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "ref-sync-parent-first",
			Kind:      artifact.KindRefreshReport,
			Version:   1,
			Status:    artifact.StatusActive,
			Title:     "Refresh parent imported first",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "An ordinary parent carrier in the same pull.",
		StructuredData: `{}`,
	}
	item := artifact.EvidenceItem{
		ID:              "evid-20260811-000000777",
		Type:            "audit",
		Content:         "Evidence imported after its refresh parent.",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  5,
		CreatedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
	}
	evidence, err := artifact.NewEvidenceCarrierArtifact(parent.Meta.ID, item)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(fixture.binding.ProjectRoot, ".haft", "refresh", parent.Meta.ID+".md"): artifact.RenderArtifactFile(parent),
		filepath.Join(fixture.binding.ProjectRoot, ".haft", "evidence", item.ID+".md"):       artifact.RenderArtifactFile(evidence),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	restore := enterTestProjectRoot(t, fixture.binding.ProjectRoot)
	runContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := &cobra.Command{}
	command.SetContext(runContext)
	err = runSync(command, nil)
	restore()
	if err != nil {
		t.Fatal(err)
	}

	ledger, err := openCurrentProjectLedger(
		ctx,
		fixture.binding.ProjectRoot,
		projectledger.ReadWrite,
		"parent-first sync test verification",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	store := artifact.NewStore(ledger.Database())
	items, err := store.GetEvidenceItems(ctx, parent.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("imported evidence = %#v, want %s", items, item.ID)
	}
}
