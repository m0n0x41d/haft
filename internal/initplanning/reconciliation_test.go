package initplanning

import (
	"reflect"
	"testing"
)

func TestCompileHostAdapterReconciliationPreservesConflictsAndExactEvidence(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	currentness, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	plan, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileHostAdapterReconciliation: %v", err)
	}
	preview := previewHostPlan(plan)
	effects := make(map[string]FileEffectPreview, len(preview.Effects))
	for _, effect := range preview.Effects {
		effects[effect.Path] = effect
	}
	want := map[string]FileEffectKind{
		fixture.paths.current:  FilePreserve,
		fixture.paths.outdated: FileUpdate,
		fixture.paths.modified: FileConflict,
		fixture.paths.legacy:   FileUpdateLegacy,
		fixture.paths.foreign:  FileConflict,
		fixture.paths.orphaned: FileRemoveOwnedOrphan,
		fixture.paths.missing:  FileCreate,
		fixture.paths.vacant:   FileCreate,
	}
	for path, kind := range want {
		effect, exists := effects[path]
		if !exists {
			t.Fatalf("path %s has no reconciliation effect", path)
		}
		if effect.Effect != kind {
			t.Fatalf("path %s effect = %s, want %s", path, effect.Effect, kind)
		}
	}
	modified := effects[fixture.paths.modified]
	if modified.PredecessorKind != PredecessorLocallyModifiedOwned ||
		modified.PredecessorDigest == "" ||
		modified.ManifestPathDigest == "" ||
		modified.OwnershipKind != OwnershipManifestReceipt ||
		modified.RenderedDigest == "" {
		t.Fatalf("locally-modified preview lost evidence: %+v", modified)
	}
	foreign := effects[fixture.paths.foreign]
	if foreign.PredecessorKind != PredecessorForeign ||
		foreign.OwnershipKind != "" ||
		foreign.RenderedDigest == "" {
		t.Fatalf("foreign preview fabricated ownership or lost target: %+v", foreign)
	}
	missing := effects[fixture.paths.missing]
	if missing.PredecessorKind != PredecessorMissingOwned ||
		missing.ManifestPathDigest == "" ||
		missing.OwnershipKind != OwnershipManifestReceipt {
		t.Fatalf("missing-owned preview lost receipt evidence: %+v", missing)
	}
	if len(plan.Conflicts()) != 2 {
		t.Fatalf("conflict count = %d, want 2", len(plan.Conflicts()))
	}
	if _, err := BuildInstallationManifest(plan); err == nil {
		t.Fatal("blocked reconciliation produced a success manifest")
	}
}

func TestCompileHostAdapterReconciliationAdoptsOnlyExactLegacyAndIsDeterministic(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	observations := removeObservation(fixture.observations, fixture.paths.foreign)
	outputs := make([]RenderedOutput, 0, len(fixture.projection.outputs)-2)
	for _, output := range fixture.projection.outputs {
		if output.path != fixture.paths.modified && output.path != fixture.paths.foreign {
			outputs = append(outputs, output)
		}
	}
	projection := buildCurrentProjection(t, root, outputs)
	currentness, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		projection,
		observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	left, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileHostAdapterReconciliation: %v", err)
	}
	right, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileHostAdapterReconciliation repeat: %v", err)
	}
	if len(left.Conflicts()) != 1 {
		// The removed locally-modified manifest path remains a preserving conflict.
		t.Fatalf("cleaned fixture conflict count = %d, want 1", len(left.Conflicts()))
	}
	if !hostPlansEqualForTest(left, right) {
		t.Fatal("same currentness evidence produced different reconciliation plans")
	}
}

func hostPlansEqualForTest(left HostAdapterInstallPlan, right HostAdapterInstallPlan) bool {
	leftPreview := previewHostPlan(left)
	rightPreview := previewHostPlan(right)
	if leftPreview.Host != rightPreview.Host ||
		leftPreview.Edition != rightPreview.Edition ||
		leftPreview.Scope != rightPreview.Scope {
		return false
	}
	if len(leftPreview.Effects) != len(rightPreview.Effects) {
		return false
	}
	for index, effect := range leftPreview.Effects {
		if !reflect.DeepEqual(effect, rightPreview.Effects[index]) {
			return false
		}
	}
	return true
}
