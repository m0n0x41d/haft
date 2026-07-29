package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPublicationSnapshotOwnsExactPublicationAndSourceUnits(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	readme := mustReadSourceFixture(t, readmePath)
	spec := mustReadSourceFixture(t, specPath)
	originalReadme := append([]byte(nil), readme...)
	originalSpec := append([]byte(nil), spec...)
	bundle := SourceBundle{
		Readme: SourceDocument{Path: readmePath, SourceRevision: "snapshot-revision", Markdown: readme},
		Spec:   SourceDocument{Path: specPath, SourceRevision: "snapshot-revision", Markdown: spec},
	}

	snapshot, err := BuildPublicationSnapshot(bundle)
	if err != nil {
		t.Fatalf("BuildPublicationSnapshot() error: %v", err)
	}
	if snapshot.Revision() != "snapshot-revision" {
		t.Fatalf("revision = %q, want snapshot-revision", snapshot.Revision())
	}
	if snapshot.ReadmeDigest().String() != exactDocumentDigest(originalReadme) {
		t.Fatalf("README digest = %q, want exact document digest", snapshot.ReadmeDigest().String())
	}
	if snapshot.SpecDigest().String() != exactDocumentDigest(originalSpec) {
		t.Fatalf("spec digest = %q, want exact document digest", snapshot.SpecDigest().String())
	}

	bundle.Readme.Markdown[0] ^= 0xff
	bundle.Spec.Markdown[0] ^= 0xff
	if string(snapshot.Readme().Markdown) != string(originalReadme) {
		t.Fatal("snapshot README changed after caller mutated the source bundle")
	}
	if string(snapshot.Spec().Markdown) != string(originalSpec) {
		t.Fatal("snapshot spec changed after caller mutated the source bundle")
	}

	readmeCopy := snapshot.Readme()
	readmeCopy.Markdown[0] ^= 0xff
	if string(snapshot.Readme().Markdown) != string(originalReadme) {
		t.Fatal("snapshot README changed after caller mutated a returned document")
	}
	bundleCopy := snapshot.SourceBundle()
	bundleCopy.Spec.Markdown[0] ^= 0xff
	if string(snapshot.Spec().Markdown) != string(originalSpec) {
		t.Fatal("snapshot spec changed after caller mutated a returned bundle")
	}

	assertPublicationSourceUnitCopiesAreIndependent(t, snapshot)
}

func TestLoadPublicationSnapshotReadsOneExactPublicationBasis(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	readme := mustReadSourceFixture(t, readmePath)
	spec := mustReadSourceFixture(t, specPath)

	snapshot, err := LoadPublicationSnapshot(readmePath, specPath, "")
	if err != nil {
		t.Fatalf("LoadPublicationSnapshot() error: %v", err)
	}
	wantRevision := "sha256:" + sourceContentHash(string(spec))
	if snapshot.Revision() != wantRevision {
		t.Fatalf("resolved revision = %q, want %q", snapshot.Revision(), wantRevision)
	}
	if snapshot.Readme().Path != filepath.Clean(readmePath) ||
		snapshot.Spec().Path != filepath.Clean(specPath) {
		t.Fatalf(
			"snapshot paths = %q, %q; want cleaned input paths",
			snapshot.Readme().Path,
			snapshot.Spec().Path,
		)
	}
	if string(snapshot.Readme().Markdown) != string(readme) ||
		string(snapshot.Spec().Markdown) != string(spec) {
		t.Fatal("loaded snapshot did not retain the exact source bytes")
	}
	if snapshot.ReadmeDigest().String() != exactDocumentDigest(readme) ||
		snapshot.SpecDigest().String() != exactDocumentDigest(spec) {
		t.Fatal("loaded snapshot digest does not identify its exact source bytes")
	}
}

func TestBuildPublicationSnapshotCallsBuilderExactlyOnce(t *testing.T) {
	bundle := validSnapshotTestBundle()
	calls := 0
	builder := func(received SourceBundle) ([]SourceUnit, error) {
		calls++
		received.Readme.Markdown[0] = 'X'
		return []SourceUnit{{UnitID: "unit-1", DirectRefs: []string{"A.1"}}}, nil
	}

	snapshot, err := buildPublicationSnapshot(bundle, builder)
	if err != nil {
		t.Fatalf("buildPublicationSnapshot() error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("builder calls = %d, want exactly 1", calls)
	}
	if string(snapshot.Readme().Markdown) != "readme" {
		t.Fatal("snapshot source bytes alias the builder input")
	}
	resolved, ok := snapshot.ResolveSourceUnit("unit-1")
	if !ok || resolved.UnitID != "unit-1" {
		t.Fatalf("ResolveSourceUnit(unit-1) = %#v, %v", resolved, ok)
	}
	if _, ok := snapshot.ResolveSourceUnit(" unit-1 "); ok {
		t.Fatal("source-unit resolution must not trim or broaden an exact UnitID")
	}
}

func TestBuildPublicationSnapshotRejectsRevisionMismatchBeforeBuild(t *testing.T) {
	bundle := validSnapshotTestBundle()
	bundle.Spec.SourceRevision = "other-revision"
	calls := 0
	builder := func(SourceBundle) ([]SourceUnit, error) {
		calls++
		return nil, nil
	}

	_, err := buildPublicationSnapshot(bundle, builder)
	if err == nil || !strings.Contains(err.Error(), "revisions differ") {
		t.Fatalf("revision mismatch error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("builder calls = %d, want 0 for an invalid source bundle", calls)
	}
}

func TestBuildPublicationSnapshotPreservesBuilderFailure(t *testing.T) {
	want := errors.New("publication grammar changed")
	calls := 0
	builder := func(SourceBundle) ([]SourceUnit, error) {
		calls++
		return nil, want
	}

	_, err := buildPublicationSnapshot(validSnapshotTestBundle(), builder)
	if !errors.Is(err, want) {
		t.Fatalf("build error = %v, want wrapped builder error", err)
	}
	if calls != 1 {
		t.Fatalf("builder calls = %d, want exactly 1", calls)
	}
}

func TestBuildPublicationSnapshotRejectsDuplicateUnitIDDefensively(t *testing.T) {
	builder := func(SourceBundle) ([]SourceUnit, error) {
		return []SourceUnit{
			{UnitID: "duplicate"},
			{UnitID: "duplicate"},
		}, nil
	}

	_, err := buildPublicationSnapshot(validSnapshotTestBundle(), builder)
	if err == nil || !strings.Contains(err.Error(), `duplicate source unit id "duplicate"`) {
		t.Fatalf("duplicate UnitID error = %v", err)
	}
}

func assertPublicationSourceUnitCopiesAreIndependent(t *testing.T, snapshot PublicationSnapshot) {
	t.Helper()
	units := snapshot.SourceUnits()
	if len(units) == 0 {
		t.Fatal("snapshot has no source units")
	}
	sawDirectRefs := false
	sawRelations := false
	sawAuthoredPhrases := false
	sawKeywords := false

	for index := range units {
		unit := units[index]
		unitID := unit.UnitID
		units[index].Title = "mutated title"
		if len(unit.DirectRefs) > 0 {
			sawDirectRefs = true
			units[index].DirectRefs[0] = "mutated direct ref"
		}
		if len(unit.Relations) > 0 {
			sawRelations = true
			units[index].Relations[0].TargetPatternID = "mutated relation"
		}
		if len(unit.AuthoredPhrases) > 0 {
			sawAuthoredPhrases = true
			units[index].AuthoredPhrases[0] = "mutated authored phrase"
		}
		if len(unit.Keywords) > 0 {
			sawKeywords = true
			units[index].Keywords[0] = "mutated keyword"
		}

		resolved, ok := snapshot.ResolveSourceUnit(unitID)
		if !ok {
			t.Fatalf("ResolveSourceUnit(%q) did not find a snapshotted unit", unitID)
		}
		if resolved.Title == "mutated title" ||
			containsSourceString(resolved.DirectRefs, "mutated direct ref") ||
			containsSourceRelationTarget(resolved.Relations, "mutated relation") ||
			containsSourceString(resolved.AuthoredPhrases, "mutated authored phrase") ||
			containsSourceString(resolved.Keywords, "mutated keyword") {
			t.Fatalf("source unit %q aliases a returned nested slice", unitID)
		}
	}
	if !sawDirectRefs || !sawRelations || !sawAuthoredPhrases || !sawKeywords {
		t.Fatalf(
			"production snapshot did not exercise every nested SourceUnit slice: refs=%v relations=%v phrases=%v keywords=%v",
			sawDirectRefs,
			sawRelations,
			sawAuthoredPhrases,
			sawKeywords,
		)
	}

	resolved, ok := snapshot.ResolveSourceUnit(units[0].UnitID)
	if !ok {
		t.Fatalf("ResolveSourceUnit(%q) did not find a snapshotted unit", units[0].UnitID)
	}
	resolved.Title = "resolved mutation"
	again, ok := snapshot.ResolveSourceUnit(units[0].UnitID)
	if !ok || again.Title == "resolved mutation" {
		t.Fatal("resolved SourceUnit aliases snapshot storage")
	}
}

func containsSourceRelationTarget(relations []SourceRelation, target string) bool {
	for _, relation := range relations {
		if relation.TargetPatternID == target {
			return true
		}
	}
	return false
}

func validSnapshotTestBundle() SourceBundle {
	return SourceBundle{
		Readme: SourceDocument{Path: "Readme.md", SourceRevision: "revision", Markdown: []byte("readme")},
		Spec:   SourceDocument{Path: "FPF-Spec.md", SourceRevision: "revision", Markdown: []byte("spec")},
	}
}

func exactDocumentDigest(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
