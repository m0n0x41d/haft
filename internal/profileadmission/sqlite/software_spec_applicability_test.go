package sqlite

import (
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestSoftwareSystemSpecMigrationResolverMintsRequiredFromCurrentAdmission(
	t *testing.T,
) {
	fixture := newTransactionFixture(t, "applicability-required", "applicability-required.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	result := service.ResolveSoftwareSystemSpecMigration(context.Background(), fixture.root)
	if result.Kind() != SoftwareSystemSpecMigrationApplicabilityRequired {
		t.Fatalf("kind = %q, want required", result.Kind())
	}
	required, ok := result.Required()
	if !ok || !required.Valid() {
		t.Fatal("required result omitted its sealed capability")
	}
	scopes := required.SoftwareScopeIDs()
	if len(scopes) != 1 || scopes[0].String() != "software-applicability-required" {
		t.Fatalf("software scopes = %#v", scopes)
	}
	if required.ProjectRoot() != fixture.root {
		t.Fatal("required capability has another project root")
	}
	current := service.ValidateCurrentSoftwareSystemSpecMigrationRequired(
		context.Background(),
		required,
	)
	if current != SoftwareSystemSpecMigrationProofValid {
		t.Fatalf("current validation = %q, want valid", current)
	}
	historical := service.ValidateHistoricalSoftwareSystemSpecMigrationRequired(
		context.Background(),
		required,
	)
	if historical != SoftwareSystemSpecMigrationProofValid {
		t.Fatalf("historical validation = %q, want valid", historical)
	}
	staleState := *required.state
	staleState.softwareScopeIDs = append(
		[]projectprofile.ScopeID{},
		required.state.softwareScopeIDs...,
	)
	nextRevision, err := staleState.binding.ledgerRevision.Next()
	if err != nil {
		t.Fatalf("advance ledger revision: %v", err)
	}
	staleState.binding.ledgerRevision = nextRevision
	stale := SoftwareSystemSpecMigrationRequired{state: &staleState}
	if !stale.Valid() {
		t.Fatal("well-shaped stale fixture is invalid before durable comparison")
	}
	current = service.ValidateCurrentSoftwareSystemSpecMigrationRequired(
		context.Background(),
		stale,
	)
	if current != SoftwareSystemSpecMigrationProofNotCurrent {
		t.Fatalf("stale current validation = %q, want not_current", current)
	}
	historical = service.ValidateHistoricalSoftwareSystemSpecMigrationRequired(
		context.Background(),
		stale,
	)
	if historical != SoftwareSystemSpecMigrationProofInvalid {
		t.Fatalf("stale historical validation = %q, want invalid", historical)
	}
}

func TestRequiredFreshnessRejectsAdvancedHeadButHistoricalRecoveryRemainsValid(
	t *testing.T,
) {
	fixture := newTransactionFixture(t, "applicability-head-one", "applicability-head-one.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	firstResult := service.ResolveSoftwareSystemSpecMigration(context.Background(), fixture.root)
	first, ok := firstResult.Required()
	if !ok {
		t.Fatalf("first kind = %q, want required", firstResult.Kind())
	}
	secondPayload := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{
			mustNonSoftwareApplicabilityScope(t, "documents-after-software"),
		},
	)
	secondRequest := prepareV3AdmissionRequest(
		t,
		fixture.database,
		fixture.root,
		secondPayload,
		"applicability-head-two",
	)
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), secondRequest),
		CanonicalAdmissionFresh,
	)
	current := service.ValidateCurrentSoftwareSystemSpecMigrationRequired(
		context.Background(),
		first,
	)
	if current != SoftwareSystemSpecMigrationProofNotCurrent {
		t.Fatalf("advanced-head validation = %q, want not_current", current)
	}
	historical := service.ValidateHistoricalSoftwareSystemSpecMigrationRequired(
		context.Background(),
		first,
	)
	if historical != SoftwareSystemSpecMigrationProofValid {
		t.Fatalf("historical validation after head advance = %q, want valid", historical)
	}
}

func TestSoftwareSystemSpecMigrationResolverReturnsUnderdeterminedWithoutAdmission(
	t *testing.T,
) {
	fixture := newTransactionFixture(t, "applicability-absent", "applicability-absent.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	result := service.ResolveSoftwareSystemSpecMigration(context.Background(), fixture.root)
	if result.Kind() != SoftwareSystemSpecMigrationApplicabilityUnderdetermined {
		t.Fatalf("kind = %q, want underdetermined", result.Kind())
	}
	value, ok := result.Underdetermined()
	if !ok || value.MissingBasis() != MissingCurrentCanonicalProfileAdmission {
		t.Fatalf("underdetermined result = %#v", value)
	}
	if _, ok := result.Required(); ok {
		t.Fatal("absent canonical admission minted Required")
	}
}

func TestSoftwareSystemSpecMigrationResolverReturnsNotApplicableForNonSoftwareProfile(
	t *testing.T,
) {
	payload := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{
			mustNonSoftwareApplicabilityScope(t, "documents"),
		},
	)
	fixture := newTransactionFixtureWithPayload(
		t,
		"applicability-nonsoftware",
		"applicability-nonsoftware.nonce",
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
	result := service.ResolveSoftwareSystemSpecMigration(context.Background(), fixture.root)
	if result.Kind() != SoftwareSystemSpecMigrationApplicabilityNotApplicable {
		t.Fatalf("kind = %q, want not_applicable", result.Kind())
	}
	value, ok := result.NotApplicable()
	if !ok || !value.Valid() {
		t.Fatal("NotApplicable result omitted its canonical provenance")
	}
	if value.ProjectRoot() != fixture.root || value.LedgerRevision().Value() != 1 {
		t.Fatal("NotApplicable result has another canonical profile binding")
	}
	if _, ok := result.Required(); ok {
		t.Fatal("non-software admission minted Required")
	}
}

func TestSoftwareSystemSpecMigrationResolverKeepsOnlySoftwareScopesFromMixedProfile(
	t *testing.T,
) {
	payload := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{
			mustSoftwareApplicabilityScope(t, "software"),
			mustNonSoftwareApplicabilityScope(t, "documents"),
		},
	)
	fixture := newTransactionFixtureWithPayload(
		t,
		"applicability-mixed",
		"applicability-mixed.nonce",
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
	result := service.ResolveSoftwareSystemSpecMigration(context.Background(), fixture.root)
	required, ok := result.Required()
	if !ok {
		t.Fatalf("kind = %q, want required", result.Kind())
	}
	ids := required.SoftwareScopeIDs()
	if len(ids) != 1 || ids[0].String() != "software" {
		t.Fatalf("mixed Required scope IDs = %#v", ids)
	}
}

func TestSoftwareScopeIDsClassifiesMixedAndNonSoftwarePayloads(t *testing.T) {
	softwareB := mustSoftwareApplicabilityScope(t, "software-b")
	softwareA := mustSoftwareApplicabilityScope(t, "software-a")
	nonSoftware := mustNonSoftwareApplicabilityScope(t, "documents")
	mixed := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{softwareB, nonSoftware, softwareA},
	)
	ids, err := softwareScopeIDs(mixed)
	if err != nil {
		t.Fatalf("softwareScopeIDs(mixed): %v", err)
	}
	if len(ids) != 2 || ids[0].String() != "software-a" || ids[1].String() != "software-b" {
		t.Fatalf("mixed software scope IDs = %#v", ids)
	}
	nonSoftwarePayload := mustApplicabilityPayload(
		t,
		[]projectprofile.RealizationScope{nonSoftware},
	)
	ids, err = softwareScopeIDs(nonSoftwarePayload)
	if err != nil {
		t.Fatalf("softwareScopeIDs(non-software): %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("non-software scope IDs = %#v, want none", ids)
	}
}

func TestZeroApplicabilityValuesAreInvalid(t *testing.T) {
	if (SoftwareSystemSpecMigrationApplicability{}).Valid() {
		t.Fatal("zero applicability result is valid")
	}
	if (SoftwareSystemSpecMigrationRequired{}).Valid() {
		t.Fatal("zero Required capability is valid")
	}
	if (SoftwareSystemSpecMigrationNotApplicableValue{}).Valid() {
		t.Fatal("zero NotApplicable value is valid")
	}
	if (SoftwareSystemSpecMigrationUnderdeterminedValue{}).Valid() {
		t.Fatal("zero Underdetermined value is valid")
	}
}

func mustSoftwareApplicabilityScope(
	t *testing.T,
	raw string,
) projectprofile.SoftwareRealization {
	t.Helper()
	id, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewSoftwareRealization(id, projectprofile.NoEntityReference{})
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return scope
}

func mustNonSoftwareApplicabilityScope(
	t *testing.T,
	raw string,
) projectprofile.NonSoftwareRealization {
	t.Helper()
	id, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		id,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	return scope
}

func mustApplicabilityPayload(
	t *testing.T,
	values []projectprofile.RealizationScope,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopes, err := projectprofile.NewScopeSet(values)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}
