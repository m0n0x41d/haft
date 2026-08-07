package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
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
