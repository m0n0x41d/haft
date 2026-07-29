package typeenv

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCompareBaseTypeEnvArtifactsIsOutsideArtifactIdentity(t *testing.T) {
	fixture := newArtifactFixture(t)
	previous := sealCompatibilityFixture(t, fixture, []LinkedDeclaration{
		fixture.declaration(t, "U.Alpha", fixture.location(t, "unit:alpha-before", 1, 1), nil),
		fixture.declaration(t, "U.Removed", fixture.location(t, "unit:removed", 2, 2), nil),
	})
	current := sealCompatibilityFixture(t, fixture, []LinkedDeclaration{
		fixture.declaration(t, "U.Alpha", fixture.location(t, "unit:alpha-after", 3, 3), nil),
		fixture.declaration(t, "U.Added", fixture.location(t, "unit:added", 4, 4), nil),
	})
	currentDigest := current.Digest()

	assessment, err := CompareBaseTypeEnvArtifacts(previous, current)
	if err != nil {
		t.Fatalf("CompareBaseTypeEnvArtifacts() error = %v", err)
	}
	compared, ok := assessment.(ComparedCompatibilityAssessment)
	if !ok {
		t.Fatalf("assessment = %T, want ComparedCompatibilityAssessment", assessment)
	}
	changes := compared.Diff().Changes()
	if len(changes) != 3 {
		t.Fatalf("compatibility changes = %d, want 3", len(changes))
	}
	assertCompatibilityChange(t, changes, "kind:U.Added", typedmemory.CompatibilityAdded)
	assertCompatibilityChange(t, changes, "kind:U.Alpha", typedmemory.CompatibilityChanged)
	assertCompatibilityChange(t, changes, "kind:U.Removed", typedmemory.CompatibilityRemoved)
	if current.Digest() != currentDigest {
		t.Fatal("compatibility comparison changed current artifact identity")
	}
}

func TestCompareBaseTypeEnvArtifactsUsesInitialAssessmentWithoutPreviousRef(t *testing.T) {
	fixture := newArtifactFixture(t)
	previous := coverageOnlyCompatibilityFixture(t, fixture)
	current := sealCompatibilityFixture(t, fixture, []LinkedDeclaration{
		fixture.declaration(t, "U.Alpha", fixture.location(t, "unit:alpha", 1, 1), nil),
	})

	assessment, err := CompareBaseTypeEnvArtifacts(previous, current)
	if err != nil {
		t.Fatalf("CompareBaseTypeEnvArtifacts() error = %v", err)
	}
	if _, ok := assessment.(InitialCompatibilityAssessment); !ok {
		t.Fatalf("assessment = %T, want InitialCompatibilityAssessment", assessment)
	}
}

func sealCompatibilityFixture(
	t *testing.T,
	fixture artifactFixture,
	declarations []LinkedDeclaration,
) BaseTypeEnvArtifact {
	t.Helper()
	artifact, err := SealBaseTypeEnv(fixture.compiledIR(t, declarations))
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	return artifact
}

func coverageOnlyCompatibilityFixture(
	t *testing.T,
	fixture artifactFixture,
) BaseTypeEnvArtifact {
	t.Helper()
	location := fixture.location(t, "unit:coverage-only", 1, 1)
	unitID, err := typedmemory.NewSourceUnitID("unit:coverage-only")
	if err != nil {
		t.Fatalf("NewSourceUnitID() error = %v", err)
	}
	subject, err := typedmemory.SourceUnitCoverage(unitID)
	if err != nil {
		t.Fatalf("SourceUnitCoverage() error = %v", err)
	}
	entry, err := typedmemory.NewSourceOnlyCoverageEntry(subject, location, "source_only_fixture")
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry() error = %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := NewCoverageOnlyLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		"source_only_fixture",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR() error = %v", err)
	}
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	return artifact
}

func assertCompatibilityChange(
	t *testing.T,
	changes []typedmemory.CompatibilityChange,
	symbol string,
	kind typedmemory.CompatibilityChangeKind,
) {
	t.Helper()
	for _, change := range changes {
		if change.Symbol().String() == symbol && change.Kind() == kind {
			return
		}
	}
	t.Fatalf("compatibility change %s/%s not found", symbol, kind.String())
}
