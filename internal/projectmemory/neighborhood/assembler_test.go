package neighborhood_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestAssemblerProducesDeterministicBoundedExactNeighborhood(t *testing.T) {
	fixture := newAssemblerFixture(t, false)
	left, err := neighborhood.Assemble(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	right, err := neighborhood.Assemble(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) ||
		left.Digest() != right.Digest() {
		t.Fatal("fixed pinned inputs did not produce byte-stable output")
	}
	var canonical map[string]any
	if err := json.Unmarshal(left.CanonicalBytes(), &canonical); err != nil {
		t.Fatal(err)
	}
	interpretation, ok := canonical["interpretation_contract"].(map[string]any)
	if !ok ||
		interpretation["relational_records"] !=
			string(neighborhood.RelationalRecordsLegacyUnqualifiedAssertions) {
		t.Fatalf("relational-record interpretation = %#v", interpretation)
	}
	if _, ambiguous := interpretation["relations"]; ambiguous {
		t.Fatalf("ambiguous relations field survived: %#v", interpretation)
	}
	canonicalFacets, ok := canonical["facets"].([]any)
	if !ok || len(canonicalFacets) == 0 {
		t.Fatalf("canonical facets = %#v", canonical["facets"])
	}
	problemFacet, ok := canonicalFacets[0].(map[string]any)
	if !ok {
		t.Fatalf("canonical problem facet = %#v", canonicalFacets[0])
	}
	items, ok := problemFacet["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("canonical problem items = %#v", problemFacet["items"])
	}
	firstItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("canonical problem item = %#v", items[0])
	}
	witnesses, ok := firstItem["why_included"].([]any)
	if !ok || len(witnesses) == 0 {
		t.Fatalf("canonical relation witnesses = %#v", firstItem["why_included"])
	}
	witness, ok := witnesses[0].(map[string]any)
	if !ok || witness["relational_record_posture"] !=
		string(neighborhood.RelationalRecordItemLegacyUnqualifiedAssertion) {
		t.Fatalf("canonical relation witness = %#v", witnesses[0])
	}
	if witness["relation_declaration_posture"] !=
		string(typedmemory.RelationDeclarationTypedFragment) {
		t.Fatalf("relation declaration posture = %#v", witnesses[0])
	}
	if left.Kind() != neighborhood.ResultExactNeighborhood ||
		left.ViewContext().Entity() != fixture.rootRef ||
		left.SnapshotBasis().GraphRevision().Value() != 11 {
		t.Fatal("assembler failed to preserve exact EntityOfConcern snapshot")
	}
	facets := left.Facets()
	if len(facets) != 2 ||
		facets[0].Kind() != neighborhood.FacetProblems ||
		facets[1].Kind() != neighborhood.FacetEvidence {
		t.Fatalf("profile facet order changed: %#v", facets)
	}
	partial, ok := facets[0].Coverage().(neighborhood.PartialCoverage)
	if !ok ||
		partial.Included() != 2 ||
		partial.OmittedAtLeast() != 1 ||
		!partial.Cursor().Valid() {
		t.Fatal("item budget did not become typed Partial coverage")
	}
	complete, ok := facets[1].Coverage().(neighborhood.CompleteCoverage)
	if !ok || complete.Included() != 0 {
		t.Fatal("exact known-empty facet was not Complete(0)")
	}
	if len(left.ProjectionBasis().ItemBases()) != 3 {
		t.Fatal("projection basis is not total over root plus emitted items")
	}
	budget := left.AppliedBudget()
	if budget.EmittedRelationPathCount() != 2 ||
		budget.OmittedRelationPathCount() != 2 ||
		budget.BoundedContentUTF8Bytes() == 0 ||
		len(budget.ContinuationCursors()) != 1 {
		t.Fatalf("applied budget lost exact counts: %#v", budget)
	}
	if !containsAffordance(
		left.ReadAffordances(),
		neighborhood.AffordanceInspectEntity,
	) ||
		!containsAffordance(
			left.ReadAffordances(),
			neighborhood.AffordanceExpandFacet,
		) {
		t.Fatal("exact deterministic read affordances are incomplete")
	}
	for _, affordance := range left.ReadAffordances() {
		if affordance.Kind() == neighborhood.ReadAffordanceKind("work") ||
			affordance.Kind() == neighborhood.ReadAffordanceKind("next_action") {
			t.Fatal("assembler emitted a capability or Work recommendation")
		}
	}
}

func TestAssemblerDoesNotTreatHistoryFilteringAsBudgetOmission(t *testing.T) {
	fixture := newAssemblerFixture(t, true)
	result, err := neighborhood.Assemble(fixture.input)
	if err != nil {
		t.Fatal(err)
	}
	facet := result.Facets()[0]
	if facet.Coverage().Kind() != neighborhood.CoverageComplete ||
		facet.Coverage().Included() != 2 {
		t.Fatal("profile history filtering was mislabeled as truncation")
	}
	report := result.AppliedBudget().PerFacet()[0]
	if report.ProfileFilteredItems() != 1 ||
		report.OmittedItems() != 0 {
		t.Fatal("history loss and budget omission were collapsed")
	}
}

func TestPinnedInputRequiresEveryRequestedFacetAndRejectsDuplicates(t *testing.T) {
	fixture := newAssemblerFixture(t, false)
	builder := fixture.baseBuilder()
	builder.AddFacet(fixture.problemFacet)
	if _, err := builder.Build(); err == nil {
		t.Fatal("pinned input accepted a missing requested facet")
	}
	builder = fixture.baseBuilder()
	builder.AddFacet(fixture.problemFacet)
	builder.AddFacet(fixture.problemFacet)
	builder.AddFacet(fixture.evidenceFacet)
	if _, err := builder.Build(); err == nil {
		t.Fatal("pinned input accepted a duplicate facet source")
	}
}

type assemblerFixture struct {
	input         neighborhood.PinnedNeighborhoodInput
	request       neighborhood.NeighborhoodRequest
	snapshot      neighborhood.SnapshotBasis
	root          neighborhood.RootProjectionSource
	rootRef       typedmemory.PersistedRef
	canonical     neighborhood.CanonicalInputCoordinate
	problemFacet  neighborhood.ExactFacetInput
	evidenceFacet neighborhood.ExactFacetInput
}

func (fixture assemblerFixture) baseBuilder() *neighborhood.PinnedNeighborhoodInputBuilder {
	return neighborhood.NewPinnedNeighborhoodInputBuilder().
		SetRequest(fixture.request).
		SetSnapshot(fixture.snapshot).
		SetRoot(fixture.root).
		AddCanonicalInput(fixture.canonical)
}

func newAssemblerFixture(
	t *testing.T,
	thirdItemHistorical bool,
) assemblerFixture {
	t.Helper()
	profile := mustProfile(t, "decision_rationale.v1")
	typeEnv := testTypeEnvRef(t, "6")
	context, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	rootRef := testPersistedRef(t, typeEnv, "service:auth")
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{
			neighborhood.FacetEvidence,
			neighborhood.FacetProblems,
		},
		neighborhood.DetailStandard,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	budget := assemblerBudget(t, 2, 1)
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(rootRef).
		SetContext(context).
		SetTypeEnv(typeEnv).
		SetGraphRevision(typedmemory.NewGraphRevision(11)).
		SetView(view).
		SetBudget(budget).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshotBasis(t, 11, typeEnv)
	canonicalRef, err := neighborhood.NewProjectionInputRef(
		"canonical:golden-bundle:fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	canonicalDigest := testSHA256Digest(t, "8")
	canonical, err := neighborhood.NewCanonicalInputCoordinate(
		canonicalRef,
		canonicalDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	itemInput, err := neighborhood.NewProjectionInputCoordinate(
		canonicalRef,
		canonicalDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	current := currentItemPostures(t)
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(rootRef)
	if err != nil {
		t.Fatal(err)
	}
	rootText, err := neighborhood.NewReadableItemText("Authentication service")
	if err != nil {
		t.Fatal(err)
	}
	rootProvenance, err := typedmemory.NewProvenanceRef("event:root")
	if err != nil {
		t.Fatal(err)
	}
	root, err := neighborhood.NewProjectedRoot(
		rootCoordinate,
		rootText,
		current,
		rootProvenance,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{itemInput},
		neighborhood.TransformFieldSelection,
		[]neighborhood.IntentionalLossKind{
			neighborhood.LossNoGeneratedSummary,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	rootSource, err := neighborhood.NewRootProjectionSource(root, rootBasis)
	if err != nil {
		t.Fatal(err)
	}
	problems := make([]neighborhood.ItemProjectionSource, 0, 3)
	for index := 1; index <= 3; index++ {
		postures := current
		if thirdItemHistorical && index == 3 {
			postures = historicalItemPostures(t)
		}
		source := assemblerProblemSource(
			t,
			typeEnv,
			context,
			itemInput,
			index,
			postures,
		)
		problems = append(problems, source)
	}
	problemFacet, err := neighborhood.NewExactFacetInput(
		neighborhood.FacetProblems,
		itemInput,
		problems,
	)
	if err != nil {
		t.Fatal(err)
	}
	evidenceFacet, err := neighborhood.NewExactFacetInput(
		neighborhood.FacetEvidence,
		itemInput,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture := assemblerFixture{
		request:       request,
		snapshot:      snapshot,
		root:          rootSource,
		rootRef:       rootRef,
		canonical:     canonical,
		problemFacet:  problemFacet,
		evidenceFacet: evidenceFacet,
	}
	input, err := fixture.baseBuilder().
		AddFacet(evidenceFacet).
		AddFacet(problemFacet).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	fixture.input = input
	return fixture
}

func assemblerProblemSource(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	input neighborhood.ProjectionInputCoordinate,
	index int,
	postures neighborhood.ItemPostures,
) neighborhood.ItemProjectionSource {
	t.Helper()
	reference := testPersistedRef(
		t,
		typeEnv,
		fmt.Sprintf("problem:%d", index),
	)
	coordinate, err := neighborhood.NewFacetOutputCoordinate(
		neighborhood.FacetProblems,
		reference,
	)
	if err != nil {
		t.Fatal(err)
	}
	text, err := neighborhood.NewReadableItemText(
		fmt.Sprintf("Problem %d", index),
	)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		fmt.Sprintf("event:problem:%d", index),
	)
	if err != nil {
		t.Fatal(err)
	}
	witnesses := []neighborhood.RelationPathWitness{
		assemblerWitness(t, context, reference, index, 1),
		assemblerWitness(t, context, reference, index, 2),
	}
	item, err := neighborhood.NewNeighborhoodItem(
		coordinate,
		neighborhood.ItemProblemCard,
		text,
		postures,
		provenance,
		witnesses,
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewDirectProjectionItemBasis(
		coordinate,
		[]neighborhood.ProjectionInputCoordinate{input},
		neighborhood.TransformFieldSelection,
		[]neighborhood.IntentionalLossKind{
			neighborhood.LossNoGeneratedSummary,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := neighborhood.NewItemProjectionSource(item, basis)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func assemblerWitness(
	t *testing.T,
	context typedmemory.BoundedContextRef,
	target typedmemory.PersistedRef,
	itemIndex int,
	pathIndex int,
) neighborhood.RelationPathWitness {
	t.Helper()
	assertion, err := typedmemory.NewAssertionID(
		fmt.Sprintf("assertion:%d:%d", itemIndex, pathIndex),
	)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := typedmemory.NewSignatureID("Haft.RecordAtConcern")
	if err != nil {
		t.Fatal(err)
	}
	slot, err := typedmemory.NewSlotKindID("Haft.RecordSlot")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		fmt.Sprintf("event:path:%d:%d", itemIndex, pathIndex),
	)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := neighborhood.NewRelationPathWitness(
		assertion,
		signature,
		context,
		slot,
		target,
		provenance,
		fmt.Sprintf("admission:path:%d:%d", itemIndex, pathIndex),
	)
	if err != nil {
		t.Fatal(err)
	}
	return witness
}

func assemblerBudget(
	t *testing.T,
	maxItems uint32,
	maxPaths uint32,
) neighborhood.DimensionedReadBudget {
	t.Helper()
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(2).
		SetMaxItemsPerFacet(maxItems).
		SetMaxRelationPathsPerItem(maxPaths).
		SetMaxCarrierExcerptCharacters(1000).
		SetMaxProvenanceDepth(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func currentItemPostures(t *testing.T) neighborhood.ItemPostures {
	t.Helper()
	postures, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedActive,
		neighborhood.LifecycleActive,
		neighborhood.EvidenceCurrent,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		t.Fatal("current item postures are invalid")
	}
	return postures
}

func historicalItemPostures(t *testing.T) neighborhood.ItemPostures {
	t.Helper()
	postures, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedHistorical,
		neighborhood.LifecycleHistorical,
		neighborhood.EvidenceUnknown,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		t.Fatal("historical item postures are invalid")
	}
	return postures
}

func containsAffordance(
	values []neighborhood.ReadAffordance,
	kind neighborhood.ReadAffordanceKind,
) bool {
	for _, value := range values {
		if value.Kind() == kind {
			return true
		}
	}
	return false
}
