package sqlite

import (
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestCapabilityApplicabilityMatrixResolverBindsCurrentCanonicalAdmission(
	t *testing.T,
) {
	fixture := newTransactionFixture(
		t,
		"capability-matrix-current",
		"capability-matrix-current.nonce",
	)
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	result := service.ResolveCapabilityApplicabilityMatrix(
		context.Background(),
		fixture.root,
	)
	if result.Kind() != CapabilityApplicabilityMatrixResolved {
		t.Fatalf("kind = %q, want resolved", result.Kind())
	}
	resolved, ok := result.Resolved()
	if !ok || !resolved.Valid() {
		t.Fatal("resolved result omitted its sealed matrix")
	}
	if resolved.ProjectRoot() != fixture.root {
		t.Fatal("resolved matrix has another project root")
	}
	if resolved.LedgerRevision().Value() != 1 {
		t.Fatalf("ledger revision = %d, want 1", resolved.LedgerRevision().Value())
	}
	if resolved.ProfilePayloadDigest() != resolved.Matrix().ProfilePayloadDigest() {
		t.Fatal("resolved matrix lost its canonical profile-payload binding")
	}
	scopeID := mustScopeIDForCapabilityMatrix(t, "software-capability-matrix-current")
	entry, found := resolved.Matrix().Entry(
		scopeID,
		projectprofile.SoftwareSystemSpecCapability,
	)
	if !found || entry.Kind() != projectprofile.CapabilityRequired {
		t.Fatalf("software-system applicability = %#v", entry)
	}
}

func TestCapabilityApplicabilityMatrixResolverPreservesMixedScopes(t *testing.T) {
	payload := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareApplicabilityScope(t, "software"),
			mustNonSoftwareApplicabilityScope(t, "documents"),
		},
	)
	fixture := newTransactionFixtureWithPayload(
		t,
		"capability-matrix-mixed",
		"capability-matrix-mixed.nonce",
		payload,
	)
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	result := service.ResolveCapabilityApplicabilityMatrix(
		context.Background(),
		fixture.root,
	)
	resolved, ok := result.Resolved()
	if !ok {
		t.Fatalf("kind = %q, want resolved", result.Kind())
	}
	matrix := resolved.Matrix()
	assertResolvedCapabilityKind(
		t,
		matrix,
		"software",
		projectprofile.SoftwareSystemSpecCapability,
		projectprofile.CapabilityRequired,
	)
	assertResolvedCapabilityKind(
		t,
		matrix,
		"documents",
		projectprofile.SoftwareSystemSpecCapability,
		projectprofile.CapabilityNotApplicable,
	)
	assertResolvedCapabilityKind(
		t,
		matrix,
		"documents",
		projectprofile.TypedProjectMemoryCapability,
		projectprofile.CapabilityRequired,
	)
}

func TestCapabilityApplicabilityMatrixResolverIsUnderdeterminedWithoutAdmission(
	t *testing.T,
) {
	fixture := newTransactionFixture(
		t,
		"capability-matrix-absent",
		"capability-matrix-absent.nonce",
	)
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	result := service.ResolveCapabilityApplicabilityMatrix(
		context.Background(),
		fixture.root,
	)
	if result.Kind() != CapabilityApplicabilityMatrixUnderdetermined {
		t.Fatalf("kind = %q, want underdetermined", result.Kind())
	}
	value, ok := result.Underdetermined()
	if !ok || value.MissingBasis() != MissingCurrentCanonicalProfileAdmission {
		t.Fatalf("underdetermined result = %#v", value)
	}
	if _, ok := result.Resolved(); ok {
		t.Fatal("absent canonical admission produced a resolved matrix")
	}
}

func TestZeroCapabilityApplicabilityMatrixResultsAreInvalid(t *testing.T) {
	if (CapabilityApplicabilityMatrixResult{}).Valid() {
		t.Fatal("zero matrix result is valid")
	}
	if (ResolvedCapabilityApplicabilityMatrix{}).Valid() {
		t.Fatal("zero resolved matrix is valid")
	}
	if (UnderdeterminedCapabilityApplicabilityMatrix{}).Valid() {
		t.Fatal("zero underdetermined matrix is valid")
	}
}

func mustScopeIDForCapabilityMatrix(
	t *testing.T,
	raw string,
) projectprofile.ScopeID {
	t.Helper()
	value, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	return value
}

func assertResolvedCapabilityKind(
	t *testing.T,
	matrix projectprofile.CapabilityApplicabilityMatrix,
	rawScopeID string,
	capability projectprofile.Capability,
	want projectprofile.CapabilityApplicabilityKind,
) {
	t.Helper()
	scopeID := mustScopeIDForCapabilityMatrix(t, rawScopeID)
	entry, found := matrix.Entry(scopeID, capability)
	if !found {
		t.Fatalf("entry %s/%s is absent", rawScopeID, capability)
	}
	if entry.Kind() != want {
		t.Fatalf("entry %s/%s = %q, want %q", rawScopeID, capability, entry.Kind(), want)
	}
}
