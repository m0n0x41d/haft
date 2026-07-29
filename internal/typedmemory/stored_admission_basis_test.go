package typedmemory

import (
	"testing"
)

func TestVerifyStoredAdmissionBasisCoordinatesAcceptsCanonicalVariants(t *testing.T) {
	snapshot, membership := storedAdmissionBasisFixtures(t)
	for _, basis := range []AdmissionBasis{snapshot, membership} {
		if err := VerifyStoredAdmissionBasisCoordinates(
			basis.Kind(),
			basis.TypeEnv(),
			basis.GraphRevision(),
			basis.CanonicalBytes(),
		); err != nil {
			t.Fatalf("VerifyStoredAdmissionBasisCoordinates(%s) error = %v", basis.Kind().String(), err)
		}
	}
}

func TestVerifyStoredAdmissionBasisCoordinatesRejectsCoordinateDrift(t *testing.T) {
	snapshot, _ := storedAdmissionBasisFixtures(t)
	otherTypeEnv := typeEnvTestTypeEnvRef(t, 0xd1)
	tests := []struct {
		name     string
		typeEnv  TypeEnvRef
		revision GraphRevision
	}{
		{
			name:     "same kind different TypeEnv",
			typeEnv:  otherTypeEnv,
			revision: snapshot.GraphRevision(),
		},
		{
			name:     "same kind different graph revision",
			typeEnv:  snapshot.TypeEnv(),
			revision: NewGraphRevision(snapshot.GraphRevision().Value() + 1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyStoredAdmissionBasisCoordinates(
				snapshot.Kind(),
				test.typeEnv,
				test.revision,
				snapshot.CanonicalBytes(),
			)
			if err == nil {
				t.Fatal("coordinate drift was accepted")
			}
		})
	}
}

func TestVerifyStoredAdmissionBasisCoordinatesRejectsWrongStrongKind(t *testing.T) {
	snapshot, membership := storedAdmissionBasisFixtures(t)
	tests := []struct {
		name  string
		kind  AdmissionBasisKind
		basis AdmissionBasis
	}{
		{
			name:  "snapshot bytes as membership",
			kind:  ContextSliceMembershipAdmissionBasis,
			basis: snapshot,
		},
		{
			name:  "membership bytes as snapshot",
			kind:  SnapshotOnlyAdmissionBasis,
			basis: membership,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyStoredAdmissionBasisCoordinates(
				test.kind,
				test.basis.TypeEnv(),
				test.basis.GraphRevision(),
				test.basis.CanonicalBytes(),
			)
			if err == nil {
				t.Fatal("canonical admission basis was accepted under the wrong strong kind")
			}
		})
	}
}

func TestVerifyStoredAdmissionBasisCoordinatesRejectsMalformedOrTrailingBytes(t *testing.T) {
	snapshot, membership := storedAdmissionBasisFixtures(t)
	snapshotCountMismatch := malformedStoredSnapshotOnlyBasis(
		t,
		snapshot.TypeEnv(),
		snapshot.GraphRevision(),
	)
	membershipCountMismatch := malformedStoredMembershipBasis(
		t,
		membership.TypeEnv(),
		membership.GraphRevision(),
	)
	trailing := append(snapshot.CanonicalBytes(), 0xff)
	tests := []struct {
		name      string
		kind      AdmissionBasisKind
		typeEnv   TypeEnvRef
		revision  GraphRevision
		canonical []byte
	}{
		{
			name:      "truncated outer field",
			kind:      snapshot.Kind(),
			typeEnv:   snapshot.TypeEnv(),
			revision:  snapshot.GraphRevision(),
			canonical: snapshot.CanonicalBytes()[:len(snapshot.CanonicalBytes())-1],
		},
		{
			name:      "snapshot observation count exceeds fields",
			kind:      snapshot.Kind(),
			typeEnv:   snapshot.TypeEnv(),
			revision:  snapshot.GraphRevision(),
			canonical: snapshotCountMismatch,
		},
		{
			name:      "membership use count exceeds fields",
			kind:      membership.Kind(),
			typeEnv:   membership.TypeEnv(),
			revision:  membership.GraphRevision(),
			canonical: membershipCountMismatch,
		},
		{
			name:      "trailing bytes",
			kind:      snapshot.Kind(),
			typeEnv:   snapshot.TypeEnv(),
			revision:  snapshot.GraphRevision(),
			canonical: trailing,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyStoredAdmissionBasisCoordinates(
				test.kind,
				test.typeEnv,
				test.revision,
				test.canonical,
			)
			if err == nil {
				t.Fatal("malformed or trailing canonical bytes were accepted")
			}
		})
	}
}

func storedAdmissionBasisFixtures(
	t *testing.T,
) (SnapshotOnlyBasis, ContextSliceMembershipBasis) {
	t.Helper()
	fixture := newMemberOfFixture(t)
	assertion := admissionTestAssertionID(t, "assertion:stored-basis-coordinate-verifier")
	coordinate, reference := admissionTestCoordinate(t, fixture, assertion, 7, 0)
	resolution := admissionTestSnapshotReferenceResolution(
		t,
		reference,
		fixture.query.EntityID(),
		fixture.query.ContextSlice().Context(),
		"snapshot:stored-basis-coordinate-verifier",
	)
	use := admissionTestReferenceUse(t, fixture, coordinate, resolution, admissionTestMember(t, fixture))
	observation := admissionTestAssertionAbsentObservation(t, 7, assertion)
	revision := NewGraphRevision(42)
	snapshot := admissionTestSnapshotOnlyBasis(
		t,
		fixture.typeEnv.ref,
		revision,
		[]AdmissionSnapshotObservation{observation},
	)
	membership, err := NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      fixture.typeEnv.ref,
		GraphRevision:                revision,
		Observations:                 []AdmissionSnapshotObservation{observation},
		ReferenceFillerAdmissionUses: []ReferenceFillerAdmissionUse{use},
	})
	if err != nil {
		t.Fatalf("NewContextSliceMembershipBasis() error = %v", err)
	}
	return snapshot, membership
}

func malformedStoredSnapshotOnlyBasis(
	t *testing.T,
	typeEnv TypeEnvRef,
	revision GraphRevision,
) []byte {
	t.Helper()
	snapshot := newCanonicalWriter(admissionSnapshotBasisDomain)
	snapshot.addString(typeEnv.String())
	snapshot.addUint64(revision.Value())
	snapshot.addUint64(2)
	snapshot.addBytes([]byte("one observation only"))
	basis := newCanonicalWriter(snapshotOnlyAdmissionBasisDomain)
	basis.addBytes(snapshot.bytes())
	return basis.bytes()
}

func malformedStoredMembershipBasis(
	t *testing.T,
	typeEnv TypeEnvRef,
	revision GraphRevision,
) []byte {
	t.Helper()
	snapshot := newCanonicalWriter(admissionSnapshotBasisDomain)
	snapshot.addString(typeEnv.String())
	snapshot.addUint64(revision.Value())
	snapshot.addUint64(1)
	snapshot.addBytes([]byte("one snapshot observation"))
	basis := newCanonicalWriter(contextSliceMembershipAdmissionBasisDomain)
	basis.addBytes(snapshot.bytes())
	basis.addUint64(2)
	basis.addBytes([]byte("one admission use only"))
	return basis.bytes()
}
