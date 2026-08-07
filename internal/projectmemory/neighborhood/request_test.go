package neighborhood_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestNeighborhoodViewSpecCanonicalizesOnlyByProfilePresentationOrder(
	t *testing.T,
) {
	profile := mustProfile(t, "agent_orientation.v1")
	spec, err := neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{
			neighborhood.FacetEvidence,
			neighborhood.FacetProblems,
			neighborhood.FacetDecisions,
		},
		neighborhood.DetailStandard,
		false,
	)
	if err != nil {
		t.Fatalf("NewNeighborhoodViewSpec: %v", err)
	}
	want := []neighborhood.FacetKind{
		neighborhood.FacetProblems,
		neighborhood.FacetDecisions,
		neighborhood.FacetEvidence,
	}
	if !slicesEqual(spec.RequestedFacets(), want) {
		t.Fatalf("requested facets = %#v, want %#v", spec.RequestedFacets(), want)
	}
	if !spec.Valid() {
		t.Fatal("view spec is invalid")
	}
}

func TestNeighborhoodViewSpecRejectsDuplicateUnsupportedAndUnknownInputs(
	t *testing.T,
) {
	profile := mustProfile(t, "decision_rationale.v1")
	_, err := neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{
			neighborhood.FacetDecisions,
			neighborhood.FacetDecisions,
		},
		neighborhood.DetailStandard,
		false,
	)
	if err == nil {
		t.Fatal("duplicate facet was accepted")
	}
	_, err = neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{neighborhood.FacetImplementation},
		neighborhood.DetailStandard,
		false,
	)
	if err == nil {
		t.Fatal("profile-excluded facet was accepted")
	}
	evidence := mustProfile(t, "evidence_currentness.v1")
	_, err = neighborhood.NewNeighborhoodViewSpec(
		evidence.Ref(),
		[]neighborhood.FacetKind{neighborhood.FacetEvidence},
		neighborhood.DetailOverview,
		false,
	)
	if err == nil {
		t.Fatal("profile-excluded detail level was accepted")
	}
}

func TestNeighborhoodRequestRequiresExactConsistentSnapshot(t *testing.T) {
	typeEnv := testTypeEnvRef(t, "a")
	entity := testPersistedRef(t, typeEnv, "entity:auth-service")
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatal(err)
	}
	profile := mustProfile(t, "agent_orientation.v1")
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{
			neighborhood.FacetProblems,
			neighborhood.FacetDecisions,
		},
		neighborhood.DetailStandard,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	budget := testReadBudget(t, 2)

	builder := neighborhood.NewNeighborhoodRequestBuilder()
	builder.SetEntity(entity)
	builder.SetContext(contextRef)
	builder.SetTypeEnv(typeEnv)
	builder.SetGraphRevision(typedmemory.NewGraphRevision(9))
	builder.SetView(view)
	builder.SetBudget(budget)
	request, err := builder.Build()
	if err != nil {
		t.Fatalf("build exact request: %v", err)
	}
	if !request.Valid() ||
		request.Entity().ReferenceID().String() != "entity:auth-service" ||
		request.GraphRevision().Value() != 9 {
		t.Fatal("request lost its exact snapshot coordinate")
	}

	otherTypeEnv := testTypeEnvRef(t, "b")
	builder.SetTypeEnv(otherTypeEnv)
	if _, err := builder.Build(); err == nil {
		t.Fatal("request accepted an EntityRef from another TypeEnv")
	}
	builder.SetTypeEnv(typeEnv)
	builder.SetGraphRevision(typedmemory.NewGraphRevision(0))
	if _, err := builder.Build(); err == nil {
		t.Fatal("request accepted an implicit revision-zero snapshot")
	}
}

func TestReadBudgetHasNoImplicitDefaults(t *testing.T) {
	builder := neighborhood.NewReadBudgetBuilder()
	builder.SetMaxFacets(3)
	builder.SetMaxItemsPerFacet(8)
	builder.SetMaxRelationPathsPerItem(4)
	builder.SetMaxCarrierExcerptCharacters(1200)
	if _, err := builder.Build(); err == nil {
		t.Fatal("budget accepted a missing provenance-depth limit")
	}
	builder.SetMaxProvenanceDepth(3)
	budget, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !budget.Valid() || budget.MaxItemsPerFacet() != 8 {
		t.Fatal("complete dimensioned budget is invalid")
	}
}

func testReadBudget(
	t *testing.T,
	maxFacets uint32,
) neighborhood.DimensionedReadBudget {
	t.Helper()
	builder := neighborhood.NewReadBudgetBuilder()
	builder.SetMaxFacets(maxFacets)
	builder.SetMaxItemsPerFacet(10)
	builder.SetMaxRelationPathsPerItem(5)
	builder.SetMaxCarrierExcerptCharacters(1000)
	builder.SetMaxProvenanceDepth(3)
	budget, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func testTypeEnvRef(
	t *testing.T,
	character string,
) typedmemory.TypeEnvRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(character, 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func testPersistedRef(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	rawID string,
) typedmemory.PersistedRef {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID(rawID)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	return reference
}

func slicesEqual[T comparable](left []T, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
