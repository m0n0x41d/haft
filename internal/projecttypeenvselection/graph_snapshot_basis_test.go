package projecttypeenvselection

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectGraphSnapshotBasisSealsExactCommittedClosure(t *testing.T) {
	basis := testCommittedSnapshotBasis(t, 17, "a")
	if err := basis.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	decoded, err := VerifyProjectGraphSnapshotBasis(basis.Ref(), basis.CanonicalBytes())
	if err != nil {
		t.Fatalf("VerifyProjectGraphSnapshotBasis(): %v", err)
	}
	if decoded.Project() != basis.Project() ||
		decoded.GraphRevision() != basis.GraphRevision() {
		t.Fatalf("decoded basis lost exact snapshot coordinates")
	}
	closure, ok := decoded.Closure().(CommittedProjectGraphClosure)
	if !ok {
		t.Fatalf("decoded closure = %T; want committed", decoded.Closure())
	}
	if closure.MaterializationSchema() != ProjectGraphMaterializationSchemaV1 {
		t.Fatalf("materialization schema = %q", closure.MaterializationSchema())
	}

	canonical := basis.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, basis.CanonicalBytes()) {
		t.Fatalf("CanonicalBytes exposed mutable storage")
	}
}

func TestProjectGraphSnapshotBasisSeparatesEmptyAndCommittedClosure(t *testing.T) {
	project := testProjectID(t)
	empty, err := SealProjectGraphSnapshotBasis(ProjectGraphSnapshotBasisInput{
		Project:       project,
		GraphRevision: typedmemory.NewGraphRevision(0),
		Closure:       EmptyProjectGraphClosure{},
	})
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(empty): %v", err)
	}
	if _, ok := empty.Closure().(EmptyProjectGraphClosure); !ok {
		t.Fatalf("empty closure = %T", empty.Closure())
	}

	committed := testCommittedClosure(t, "c")
	if _, err := SealProjectGraphSnapshotBasis(ProjectGraphSnapshotBasisInput{
		Project:       project,
		GraphRevision: typedmemory.NewGraphRevision(0),
		Closure:       committed,
	}); err == nil {
		t.Fatalf("zero revision accepted committed closure")
	}
	if _, err := SealProjectGraphSnapshotBasis(ProjectGraphSnapshotBasisInput{
		Project:       project,
		GraphRevision: typedmemory.NewGraphRevision(1),
		Closure:       EmptyProjectGraphClosure{},
	}); err == nil {
		t.Fatalf("non-zero revision accepted empty closure")
	}
}

func TestProjectGraphSnapshotBasisIdentityTracksEveryCoordinate(t *testing.T) {
	base := testCommittedSnapshotBasis(t, 7, "d")
	changedRevision := testCommittedSnapshotBasis(t, 8, "d")
	changedClosure := testCommittedSnapshotBasis(t, 7, "e")
	if base.Ref() == changedRevision.Ref() {
		t.Fatalf("graph revision did not affect snapshot basis identity")
	}
	if base.Ref() == changedClosure.Ref() {
		t.Fatalf("graph closure did not affect snapshot basis identity")
	}
}

func TestProjectGraphSnapshotBasisRejectsForgeryTrailingAndMalformedRefs(t *testing.T) {
	basis := testCommittedSnapshotBasis(t, 3, "f")
	other := testCommittedSnapshotBasis(t, 4, "f")
	if _, err := VerifyProjectGraphSnapshotBasis(other.Ref(), basis.CanonicalBytes()); err == nil {
		t.Fatalf("Verify accepted unrelated expected identity")
	}
	trailing := append(basis.CanonicalBytes(), 0x00)
	if _, err := DecodeProjectGraphSnapshotBasis(trailing); err == nil {
		t.Fatalf("Decode accepted trailing bytes")
	}
	if _, err := ParseGraphEventRef("typed-memory-commit:" + strings.Repeat("a", 64)); err == nil {
		t.Fatalf("event parser accepted commit ref")
	}
	if _, err := ParseGraphCommitRef("typed-memory-event:" + strings.Repeat("a", 64)); err == nil {
		t.Fatalf("commit parser accepted event ref")
	}
}

func testCommittedSnapshotBasis(
	t *testing.T,
	revision uint64,
	digit string,
) ProjectGraphSnapshotBasis {
	t.Helper()
	basis, err := SealProjectGraphSnapshotBasis(ProjectGraphSnapshotBasisInput{
		Project:       testProjectID(t),
		GraphRevision: typedmemory.NewGraphRevision(revision),
		Closure:       testCommittedClosure(t, digit),
	})
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(): %v", err)
	}
	return basis
}

func testCommittedClosure(t *testing.T, digit string) CommittedProjectGraphClosure {
	t.Helper()
	event, err := ParseGraphEventRef("typed-memory-event:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("ParseGraphEventRef(): %v", err)
	}
	commit, err := ParseGraphCommitRef("typed-memory-commit:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("ParseGraphCommitRef(): %v", err)
	}
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	closure, err := NewCommittedProjectGraphClosure(CommittedProjectGraphClosureInput{
		Event:                 event,
		Commit:                commit,
		MaterializationDigest: digest,
	})
	if err != nil {
		t.Fatalf("NewCommittedProjectGraphClosure(): %v", err)
	}
	return closure
}

func testProjectID(t *testing.T) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_0123abcd")
	if err != nil {
		t.Fatalf("ParseProjectID(): %v", err)
	}
	return project
}

func testTypeEnvRef(t *testing.T, digit string) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef("typeenv:sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(): %v", err)
	}
	return ref
}
