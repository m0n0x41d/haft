package codeanchoradapter

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
)

func TestDraftNormalizesSemanticLinkOrderAndRequiresExplicitLink(t *testing.T) {
	missingClaim := testUnsettledBinding(t, "project_claim")
	missingWork := testUnsettledBinding(t, "performed_work")
	claimAssertion := testAssertion(t, "assertion:code-realizes-claim")
	workAssertion := testAssertion(t, "assertion:code-changed-by-work")
	claim, err := NewClaimLink(claimAssertion, missingClaim)
	if err != nil {
		t.Fatal(err)
	}
	work, err := NewWorkLink(workAssertion, missingWork)
	if err != nil {
		t.Fatal(err)
	}
	left := testDraftInput(t)
	left.Links = []SemanticLink{work, claim}
	right := testDraftInput(t)
	right.Links = []SemanticLink{claim, work}
	leftDraft, err := NewDraft(left)
	if err != nil {
		t.Fatal(err)
	}
	rightDraft, err := NewDraft(right)
	if err != nil {
		t.Fatal(err)
	}
	leftLinks := leftDraft.Links()
	rightLinks := rightDraft.Links()
	if len(leftLinks) != 2 || len(rightLinks) != 2 {
		t.Fatal("CodeAnchor draft lost an explicit semantic link")
	}
	for index := range leftLinks {
		if leftLinks[index].sortKey() != rightLinks[index].sortKey() {
			t.Fatal("CodeAnchor link permutation changed normalized order")
		}
	}

	noLink := testDraftInput(t)
	if _, err := NewDraft(noLink); err == nil {
		t.Fatal("CodeAnchor draft accepted a locator with no semantic link")
	}
}

func TestAdaptReturnsUnderdeterminedAndNoCandidateForUnresolvedTarget(
	t *testing.T,
) {
	input := testDraftInput(t)
	link, err := NewClaimLink(
		testAssertion(t, "assertion:code-realizes-unresolved-claim"),
		testUnsettledBinding(t, "project_claim"),
	)
	if err != nil {
		t.Fatal(err)
	}
	input.Links = []SemanticLink{link}
	draft, err := NewDraft(input)
	if err != nil {
		t.Fatal(err)
	}
	result := Adapt(draft, nil)
	missing, ok := result.(Underdetermined)
	if !ok {
		t.Fatalf("Adapt = %T, want Underdetermined", result)
	}
	if len(missing.MissingBasis()) != 1 ||
		missing.MissingBasis()[0].Name() != "project_claim" {
		t.Fatalf(
			"missing basis = %#v, want exact project_claim basis",
			missing.MissingBasis(),
		)
	}
}

func testDraftInput(t *testing.T) DraftInput {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	entity, err := typedmemory.NewEntityID("entity:code-anchor")
	if err != nil {
		t.Fatal(err)
	}
	local, err := typedmemory.NewBatchLocalRef("local:code-anchor")
	if err != nil {
		t.Fatal(err)
	}
	label, err := typedmemory.NewEntityLabel("Exact code anchor")
	if err != nil {
		t.Fatal(err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatal(err)
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 18, 14, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	contextSlice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	target, err := typedmemorycandidatecodec.NewSymbolCodeAnchorTarget(
		"internal/projectmemory/codeanchoradapter/adapter.go",
		"Adapt",
	)
	if err != nil {
		t.Fatal(err)
	}
	locator, err := typedmemorycandidatecodec.NewCodeAnchorLocator(
		"github.com/m0n0x41d/haft",
		"0123456789abcdef",
		target,
	)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:code-anchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	return DraftInput{
		ProjectID:             project,
		AnchorEntity:          entity,
		AnchorLocalRef:        local,
		AnchorLabel:           label,
		DefinitionAssertionID: testAssertion(t, "assertion:code-anchor-definition"),
		ContextSlice:          contextSlice,
		Locator:               NewExactLocator(locator),
		Provenance:            provenance,
	}
}

func testUnsettledBinding(
	t *testing.T,
	name string,
) UnsettledReferenceBinding {
	t.Helper()
	repair, err := typedmemory.NewRepairPointer(
		"repair:resolve-" + name,
	)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := NewMissingBasis(name, repair)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := NewUnsettledReferenceBinding([]MissingBasis{missing})
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func testAssertion(
	t *testing.T,
	raw string,
) typedmemory.AssertionID {
	t.Helper()
	value, err := typedmemory.NewAssertionID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
