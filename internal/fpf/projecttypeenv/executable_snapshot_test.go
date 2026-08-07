package projecttypeenv

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectTypeEnvExecutableSnapshotRoundTripsExactExecutableEnvironment(
	t *testing.T,
) {
	fixture := newCompositeLowererFixture(t)
	input := projectTypeEnvExecutableSnapshotInput(fixture)
	preparation := PrepareProjectTypeEnvComposite(input)
	snapshot := acceptedProjectTypeEnvExecutableSnapshot(t, preparation)
	record := snapshot.Record()

	if snapshot.TypeEnvRef() != fixture.composite.Ref() {
		t.Fatalf(
			"snapshot TypeEnvRef = %q, want exact C %q",
			snapshot.TypeEnvRef().String(),
			fixture.composite.Ref().String(),
		)
	}
	if snapshot.LoweredEnvironmentDigest() != fixture.verification.LoweredEnvironmentDigest() {
		t.Fatal("snapshot lowered digest differs from final-lowerer verification")
	}
	if record.SourceRevision() != fixture.environment.SourceRevision() ||
		record.CompilerSchemaVersion() != fixture.environment.CompilerSchemaVersion() ||
		record.LowererSchemaVersion() != fixture.composite.LowererSchemaVersion() {
		t.Fatal("snapshot does not retain exact source, compiler, and lowerer editions")
	}
	if len(record.runtimeMechanismCanonicals) == 0 {
		t.Fatal("snapshot omitted resolved X runtime-mechanism closure")
	}
	if !bytes.Contains(record.CanonicalBytes(), fixture.extension.CanonicalBytes()) {
		t.Fatal("snapshot canonical bytes omit the exact E payload")
	}

	decoded, err := DecodeProjectTypeEnvExecutableSnapshotRecord(record.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvExecutableSnapshotRecord(): %v", err)
	}
	restored, verification, err := RestoreProjectTypeEnvExecutableSnapshot(decoded, input)
	if err != nil {
		t.Fatalf("RestoreProjectTypeEnvExecutableSnapshot(): %v", err)
	}
	if !bytes.Equal(restored.Record().CanonicalBytes(), record.CanonicalBytes()) {
		t.Fatal("restored snapshot does not byte-match persisted snapshot")
	}
	if !bytes.Equal(verification.CanonicalBytes(), fixture.verification.CanonicalBytes()) {
		t.Fatal("restored final-lowerer verification differs from original")
	}
	assertProjectTypeEnvExecutablePublicValuesEqual(
		t,
		restored.Environment(),
		fixture.environment,
	)
}

func TestHistoricalV1ExecutableSnapshotReplaysWithoutCurrentClassificationField(
	t *testing.T,
) {
	fixture := newCompositeLowererFixture(t)
	historicalComposite, err := sealProjectTypeEnvCompositeAtSchema(
		fixture.linked,
		fixture.runtime,
		ProjectTypeEnvCompositeLowererSchemaV1,
	)
	if err != nil {
		t.Fatalf("seal historical v1 composite: %v", err)
	}
	input := ProjectTypeEnvCompositePreparationInput{
		Base:         fixture.base,
		Linked:       fixture.linked,
		RuntimeBasis: fixture.runtime,
		Composite:    historicalComposite,
	}
	preparation := PrepareProjectTypeEnvComposite(input)
	snapshot := acceptedProjectTypeEnvExecutableSnapshot(t, preparation)
	record := snapshot.Record()
	if record.LowererSchemaVersion() != ProjectTypeEnvCompositeLowererSchemaV1 {
		t.Fatalf("historical lowerer = %q", record.LowererSchemaVersion())
	}

	restored, verification, err := RestoreProjectTypeEnvExecutableSnapshot(
		record,
		input,
	)
	if err != nil {
		t.Fatalf("restore historical v1 executable snapshot: %v", err)
	}
	if !bytes.Equal(restored.Record().CanonicalBytes(), record.CanonicalBytes()) {
		t.Fatal("historical v1 executable snapshot changed during replay")
	}
	if verification.LowererSchemaVersion() != ProjectTypeEnvCompositeLowererSchemaV1 {
		t.Fatal("historical replay was relabeled as the current lowerer")
	}
}

func TestProjectTypeEnvExecutableSnapshotRejectsTamperMissingClosureAndSubstitution(
	t *testing.T,
) {
	fixture := newCompositeLowererFixture(t)
	input := projectTypeEnvExecutableSnapshotInput(fixture)
	preparation := PrepareProjectTypeEnvComposite(input)
	snapshot := acceptedProjectTypeEnvExecutableSnapshot(t, preparation)
	record := snapshot.Record()
	canonical := record.CanonicalBytes()

	tampered := append([]byte(nil), canonical...)
	tampered[len(tampered)-1] ^= 0xff
	truncated := append([]byte(nil), canonical[:len(canonical)-1]...)
	trailing := append(append([]byte(nil), canonical...), 0)
	for name, candidate := range map[string][]byte{
		"tampered":  tampered,
		"truncated": truncated,
		"trailing":  trailing,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeProjectTypeEnvExecutableSnapshotRecord(candidate); err == nil {
				t.Fatalf("%s snapshot was accepted", name)
			}
		})
	}

	material, err := decodeProjectTypeEnvExecutableSnapshot(canonical)
	if err != nil {
		t.Fatalf("decodeProjectTypeEnvExecutableSnapshot(): %v", err)
	}
	material.runtimeMechanismCanonicals = nil
	missingClosure, err := encodeProjectTypeEnvExecutableSnapshot(material)
	if err != nil {
		t.Fatalf("encode snapshot without X closure: %v", err)
	}
	_, err = DecodeProjectTypeEnvExecutableSnapshotRecord(missingClosure)
	if err == nil || !strings.Contains(err.Error(), "not resolved") {
		t.Fatalf("missing X closure error = %v", err)
	}

	otherRuntime := compositeLowererDifferentRuntimeBasis(t, fixture)
	otherComposite, err := SealProjectTypeEnvComposite(fixture.linked, otherRuntime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(other X): %v", err)
	}
	otherInput := ProjectTypeEnvCompositePreparationInput{
		Base:         fixture.base,
		Linked:       fixture.linked,
		RuntimeBasis: otherRuntime,
		Composite:    otherComposite,
	}
	_, _, err = RestoreProjectTypeEnvExecutableSnapshot(record, otherInput)
	if err == nil {
		t.Fatal("active TypeEnv substitution was accepted")
	}
}

func TestProjectTypeEnvExecutableSnapshotOwnsCanonicalBytes(t *testing.T) {
	fixture := newCompositeLowererFixture(t)
	preparation := PrepareProjectTypeEnvComposite(
		projectTypeEnvExecutableSnapshotInput(fixture),
	)
	snapshot := acceptedProjectTypeEnvExecutableSnapshot(t, preparation)
	record := snapshot.Record()
	first := record.CanonicalBytes()
	first[0] ^= 0xff
	if bytes.Equal(first, record.CanonicalBytes()) {
		t.Fatal("snapshot CanonicalBytes leaked mutable storage")
	}

	copyRecord := snapshot.Record()
	copyRecord.runtimeMechanismCanonicals[0][0] ^= 0xff
	if bytes.Equal(
		copyRecord.runtimeMechanismCanonicals[0],
		snapshot.Record().runtimeMechanismCanonicals[0],
	) {
		t.Fatal("snapshot Record leaked mutable runtime-mechanism storage")
	}
}

func projectTypeEnvExecutableSnapshotInput(
	fixture compositeLowererFixture,
) ProjectTypeEnvCompositePreparationInput {
	return ProjectTypeEnvCompositePreparationInput{
		Base:         fixture.base,
		Linked:       fixture.linked,
		RuntimeBasis: fixture.runtime,
		Composite:    fixture.composite,
	}
}

func acceptedProjectTypeEnvExecutableSnapshot(
	t *testing.T,
	preparation ProjectTypeEnvCompositePreparation,
) ProjectTypeEnvExecutableSnapshot {
	t.Helper()
	if preparation.Rejected() {
		t.Fatalf("PrepareProjectTypeEnvComposite() rejected: %#v", preparation.Issues())
	}
	snapshot, exists := preparation.ExecutableSnapshot()
	if !exists {
		t.Fatal("accepted composite preparation has no executable snapshot")
	}
	if err := snapshot.Verify(); err != nil {
		t.Fatalf("executable snapshot Verify(): %v", err)
	}
	return snapshot
}

func assertProjectTypeEnvExecutablePublicValuesEqual(
	t *testing.T,
	got typedmemory.TypeEnv,
	want typedmemory.TypeEnv,
) {
	t.Helper()
	if got.Ref() != want.Ref() ||
		got.SourceRevision() != want.SourceRevision() ||
		got.CompilerSchemaVersion() != want.CompilerSchemaVersion() {
		t.Fatal("restored TypeEnv identity or edition differs")
	}
	values := []struct {
		label string
		got   any
		want  any
	}{
		{"coverage", got.CoverageManifest(), want.CoverageManifest()},
		{"bounded contexts", got.BoundedContexts(), want.BoundedContexts()},
		{"kind definitions", got.KindDefinitions(), want.KindDefinitions()},
		{"entity sets", got.EntitySetDefinitions(), want.EntitySetDefinitions()},
		{"kind signatures", got.KindSignatureDefinitions(), want.KindSignatureDefinitions()},
		{"ref kinds", got.RefKindDefinitions(), want.RefKindDefinitions()},
		{"kind availabilities", got.ContextKindAvailabilities(), want.ContextKindAvailabilities()},
		{"subkind relations", got.SubkindRelations(), want.SubkindRelations()},
		{"context bridges", got.ContextBridges(), want.ContextBridges()},
		{
			"typed relation declaration fragments",
			got.TypedRelationDeclarationFragments(),
			want.TypedRelationDeclarationFragments(),
		},
		{"value-shape payloads", got.ValueShapes(), want.ValueShapes()},
		{"value bindings", got.ValueBindings(), want.ValueBindings()},
		{"constraint payloads", got.Constraints(), want.Constraints()},
	}
	for _, value := range values {
		if !reflect.DeepEqual(value.got, value.want) {
			t.Fatalf("restored %s differ:\ngot  = %#v\nwant = %#v", value.label, value.got, value.want)
		}
	}
}
