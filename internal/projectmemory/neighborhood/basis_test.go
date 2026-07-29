package neighborhood_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectionBasisRequiresOneExactBasisPerOutput(t *testing.T) {
	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "f")
	rootRef := testPersistedRef(t, typeEnv, "root")
	itemRef := testPersistedRef(t, typeEnv, "problem")
	input := testProjectionInput(t, "canonical:event:1", "1")
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(rootRef)
	if err != nil {
		t.Fatal(err)
	}
	itemCoordinate, err := neighborhood.NewFacetOutputCoordinate(
		neighborhood.FacetProblems,
		itemRef,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{input},
		neighborhood.TransformIdentity,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	itemBasis, err := neighborhood.NewDirectProjectionItemBasis(
		itemCoordinate,
		[]neighborhood.ProjectionInputCoordinate{input},
		neighborhood.TransformFieldSelection,
		[]neighborhood.IntentionalLossKind{
			neighborhood.LossNoGeneratedSummary,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := neighborhood.NewCanonicalInputCoordinate(
		input.Ref(),
		input.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(canonical).
		AddItemBasis(itemBasis).
		AddItemBasis(rootBasis).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if !basis.Valid() {
		t.Fatal("projection basis is invalid")
	}
	found, ok := basis.ItemBasisFor(itemCoordinate)
	if !ok || found.Kind() != neighborhood.ItemBasisDirect {
		t.Fatal("total item basis lost facet coordinate")
	}
}

func TestRelationalAssertionPathWitnessPreservesExactModalityWithoutOccurrence(
	t *testing.T,
) {
	typeEnv := testTypeEnvRef(t, "e")
	target := testPersistedRef(t, typeEnv, "asserted-target")
	assertion, err := typedmemory.NewAssertionID("assertion:exact-v3")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := typedmemory.NewSignatureID("Haft.RecordAtConcern")
	if err != nil {
		t.Fatal(err)
	}
	context, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	slot, err := typedmemory.NewSlotKindID("Haft.RecordSlot")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("event:exact-v3")
	if err != nil {
		t.Fatal(err)
	}
	witness, err := neighborhood.NewRelationalAssertionPathWitness(
		assertion,
		signature,
		context,
		slot,
		target,
		provenance,
		"admission:event:exact-v3",
		typedmemory.AssertionModalityObtainingUnknown,
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionPathWitness: %v", err)
	}
	posture := witness.RelationalRecordPosture()
	if posture.Kind() != neighborhood.RelationalRecordItemAssertionExact {
		t.Fatalf("v3 witness posture = %q; want assertion_exact", posture.Kind())
	}
	modality, explicit := posture.ExplicitModality()
	if !explicit || modality != typedmemory.AssertionModalityObtainingUnknown {
		t.Fatalf(
			"v3 witness modality = (%q, %t); want explicit obtaining_unknown",
			modality,
			explicit,
		)
	}
	if posture.Kind() == neighborhood.RelationalRecordItemOccurrenceExact {
		t.Fatal("v3 assertion witness was upgraded to an occurrence")
	}
	if _, err := neighborhood.NewRelationalAssertionPathWitness(
		assertion,
		signature,
		context,
		slot,
		target,
		provenance,
		"admission:event:exact-v3",
		"",
	); err == nil {
		t.Fatal("v3 assertion witness accepted a missing explicit modality")
	}
}

func TestProjectionBasisRejectsUndeclaredInputAndDuplicateOutput(t *testing.T) {
	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "1")
	rootRef := testPersistedRef(t, typeEnv, "root")
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(rootRef)
	if err != nil {
		t.Fatal(err)
	}
	declared := testProjectionInput(t, "canonical:event:1", "2")
	undeclared := testProjectionInput(t, "canonical:event:2", "3")
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{undeclared},
		neighborhood.TransformIdentity,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := neighborhood.NewCanonicalInputCoordinate(
		declared.Ref(),
		declared.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(canonical).
		AddItemBasis(rootBasis).
		Build(); err == nil {
		t.Fatal("projection basis accepted an undeclared item input")
	}

	declaredBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{declared},
		neighborhood.TransformIdentity,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(canonical).
		AddItemBasis(declaredBasis).
		AddItemBasis(declaredBasis).
		Build(); err == nil {
		t.Fatal("projection basis accepted duplicate output coordinates")
	}
}

func TestCorrespondenceBasisCannotExistWithoutExactManifest(t *testing.T) {
	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "2")
	itemRef := testPersistedRef(t, typeEnv, "problem")
	output, err := neighborhood.NewFacetOutputCoordinate(
		neighborhood.FacetProblems,
		itemRef,
	)
	if err != nil {
		t.Fatal(err)
	}
	first := testProjectionInput(t, "canonical:event:1", "4")
	second := testProjectionInput(t, "canonical:carrier:1", "5")
	witness := testRelationWitness(t, typeEnv, itemRef)
	manifestRef, err := neighborhood.NewProjectionCorrespondenceManifestRef(
		"projection-correspondence:problem:1",
	)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := neighborhood.NewProjectionCorrespondenceManifest(
		manifestRef,
		[]neighborhood.ProjectionInputCoordinate{first, second},
		output,
		[]neighborhood.RelationPathWitness{witness},
		neighborhood.TransformFieldSelection,
		[]neighborhood.IntentionalLossKind{
			neighborhood.LossNoGeneratedSummary,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	itemBasis, err := neighborhood.NewCorrespondenceProjectionItemBasis(
		manifest,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstCanonical, err := neighborhood.NewCanonicalInputCoordinate(
		first.Ref(),
		first.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := neighborhood.NewCanonicalInputCoordinate(
		second.Ref(),
		second.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	builder := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(firstCanonical).
		AddCanonicalInput(secondCanonical).
		AddItemBasis(itemBasis)
	if _, err := builder.Build(); err == nil {
		t.Fatal("correspondence item survived without its exact manifest")
	}
	basis, err := builder.
		AddCorrespondenceManifest(manifest).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if !basis.Valid() ||
		len(basis.CorrespondenceManifests()) != 1 ||
		basis.Digest().String() == "" {
		t.Fatal("exact correspondence closure was not preserved")
	}
}

func TestProjectionBasisDigestIsIndependentOfBuilderInsertionOrder(t *testing.T) {
	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "3")
	rootRef := testPersistedRef(t, typeEnv, "root")
	itemRef := testPersistedRef(t, typeEnv, "problem")
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(rootRef)
	if err != nil {
		t.Fatal(err)
	}
	itemCoordinate, err := neighborhood.NewFacetOutputCoordinate(
		neighborhood.FacetProblems,
		itemRef,
	)
	if err != nil {
		t.Fatal(err)
	}
	first := testProjectionInput(t, "canonical:event:1", "6")
	second := testProjectionInput(t, "canonical:event:2", "7")
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{first},
		neighborhood.TransformIdentity,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	itemBasis, err := neighborhood.NewDirectProjectionItemBasis(
		itemCoordinate,
		[]neighborhood.ProjectionInputCoordinate{second},
		neighborhood.TransformFieldSelection,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	firstCanonical, err := neighborhood.NewCanonicalInputCoordinate(
		first.Ref(),
		first.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := neighborhood.NewCanonicalInputCoordinate(
		second.Ref(),
		second.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	left, err := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(firstCanonical).
		AddCanonicalInput(secondCanonical).
		AddItemBasis(rootBasis).
		AddItemBasis(itemBasis).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	right, err := neighborhood.NewProjectionBasisBuilder(profile).
		AddCanonicalInput(secondCanonical).
		AddCanonicalInput(firstCanonical).
		AddItemBasis(itemBasis).
		AddItemBasis(rootBasis).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if left.Digest() != right.Digest() {
		t.Fatal("builder insertion order changed projection-basis identity")
	}
}

func testProjectionInput(
	t *testing.T,
	rawRef string,
	fill string,
) neighborhood.ProjectionInputCoordinate {
	t.Helper()
	ref, err := neighborhood.NewProjectionInputRef(rawRef)
	if err != nil {
		t.Fatal(err)
	}
	digest := testSHA256Digest(t, fill)
	input, err := neighborhood.NewProjectionInputCoordinate(ref, digest)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func testRelationWitness(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	target typedmemory.PersistedRef,
) neighborhood.RelationPathWitness {
	t.Helper()
	assertion, err := typedmemory.NewAssertionID("assertion:problem")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := typedmemory.NewSignatureID("Haft.RecordAtConcern")
	if err != nil {
		t.Fatal(err)
	}
	context, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	slot, err := typedmemory.NewSlotKindID("Haft.RecordSlot")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("event:problem")
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
		"admission:event:problem",
	)
	if err != nil {
		t.Fatal(err)
	}
	if witness.Target().RefKind().TypeEnv() != typeEnv {
		t.Fatal("test witness TypeEnv drifted")
	}
	if witness.RelationalRecordPosture().Kind() !=
		neighborhood.RelationalRecordItemLegacyUnqualifiedAssertion {
		t.Fatal("v2 relation witness was upgraded beyond its legacy basis")
	}
	return witness
}

func testSHA256Digest(t *testing.T, fill string) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + repeatDigestFill(fill)
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func repeatDigestFill(fill string) string {
	result := ""
	for len(result) < 64 {
		result += fill
	}
	return result[:64]
}
