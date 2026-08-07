package typedmemory

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestEvaluateRelationConstraintsAcceptsPermutationInvariantSubsetAndPartition(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	entityB := relationConstraintEntityID(t, "entity:b")

	forward := fixture.prospectiveView(t, []relationConstraintTestSlot{
		fixture.referenceSlot(t, fixture.relation.whole, entityA, entityB),
		fixture.referenceSlot(t, fixture.relation.left, entityA),
		fixture.referenceSlot(t, fixture.relation.right, entityB),
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})
	reverse := fixture.prospectiveView(t, []relationConstraintTestSlot{
		fixture.valueSlot(t, fixture.relation.byValue, 1),
		fixture.referenceSlot(t, fixture.relation.right, entityB),
		fixture.referenceSlot(t, fixture.relation.left, entityA),
		fixture.referenceSlot(t, fixture.relation.whole, entityB, entityA),
	})

	forwardOutcome := EvaluateRelationConstraints(fixture.environment, forward)
	reverseOutcome := EvaluateRelationConstraints(fixture.environment, reverse)
	if forwardOutcome.Kind() != RelationConstraintsValid {
		t.Fatalf("forward outcome = %s, diagnostics = %v", forwardOutcome.Kind(), relationConstraintDiagnosticKeys(forwardOutcome))
	}
	if reverseOutcome.Kind() != RelationConstraintsValid {
		t.Fatalf("reverse outcome = %s, diagnostics = %v", reverseOutcome.Kind(), relationConstraintDiagnosticKeys(reverseOutcome))
	}
	if !reflect.DeepEqual(
		relationConstraintIDs(forwardOutcome.CheckedConstraints()),
		relationConstraintIDs(reverseOutcome.CheckedConstraints()),
	) {
		t.Fatal("slot or filler permutation changed the exact checked-constraint set")
	}
	if len(forwardOutcome.CheckedConstraints()) != 4 {
		t.Fatalf("checked constraints = %v; want exact four relation-local rules", relationConstraintIDs(forwardOutcome.CheckedConstraints()))
	}
}

func TestEvaluateRelationConstraintsTreatsAbsentReferenceBindingsAsKnownEmptySets(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	view := fixture.prospectiveView(t, []relationConstraintTestSlot{
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})

	outcome := EvaluateRelationConstraints(fixture.environment, view)
	if outcome.Kind() != RelationConstraintsValid {
		t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
	}
}

func TestEvaluateRelationConstraintsRejectsKnownSubsetAndPartitionContradictions(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	entityB := relationConstraintEntityID(t, "entity:b")
	entityC := relationConstraintEntityID(t, "entity:c")

	cases := []struct {
		name     string
		slots    []relationConstraintTestSlot
		wantCode DiagnosticCode
	}{
		{
			name: "subset member absent from superset",
			slots: []relationConstraintTestSlot{
				fixture.referenceSlot(t, fixture.relation.whole, entityA),
				fixture.referenceSlot(t, fixture.relation.left, entityB),
				fixture.referenceSlot(t, fixture.relation.right, entityA),
				fixture.valueSlot(t, fixture.relation.byValue, 1),
			},
			wantCode: DiagnosticReferenceSubsetMismatch,
		},
		{
			name: "part overlap",
			slots: []relationConstraintTestSlot{
				fixture.referenceSlot(t, fixture.relation.whole, entityA, entityB),
				fixture.referenceSlot(t, fixture.relation.left, entityA),
				fixture.referenceSlot(t, fixture.relation.right, entityA, entityB),
				fixture.valueSlot(t, fixture.relation.byValue, 1),
			},
			wantCode: DiagnosticReferencePartitionMismatch,
		},
		{
			name: "part union differs from whole",
			slots: []relationConstraintTestSlot{
				fixture.referenceSlot(t, fixture.relation.whole, entityA, entityB),
				fixture.referenceSlot(t, fixture.relation.left, entityA),
				fixture.referenceSlot(t, fixture.relation.right, entityC),
				fixture.valueSlot(t, fixture.relation.byValue, 1),
			},
			wantCode: DiagnosticReferencePartitionMismatch,
		},
		{
			name: "named cardinality mismatch",
			slots: []relationConstraintTestSlot{
				fixture.referenceSlot(t, fixture.relation.whole, entityA),
				fixture.referenceSlot(t, fixture.relation.left, entityA),
				fixture.valueSlot(t, fixture.relation.byValue, 2),
			},
			wantCode: DiagnosticCardinalityMismatch,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			view := fixture.prospectiveView(t, test.slots)
			outcome := EvaluateRelationConstraints(fixture.environment, view)
			if outcome.Kind() != RelationConstraintsInvalid {
				t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
			}
			if !relationConstraintHasDiagnostic(outcome, test.wantCode, DiagnosticInvalid) {
				t.Fatalf("diagnostics = %v; want %s", relationConstraintDiagnosticKeys(outcome), test.wantCode)
			}
		})
	}
}

func TestEvaluateRelationConstraintsRejectsDuplicateResolvedEntitiesExplicitly(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	whole := fixture.referenceSlotWithPairs(t, fixture.relation.whole, []relationConstraintEntityReference{
		{entity: entityA, referenceSuffix: "a-first"},
		{entity: entityA, referenceSuffix: "a-second"},
	})
	view := fixture.prospectiveView(t, []relationConstraintTestSlot{
		whole,
		fixture.referenceSlot(t, fixture.relation.left, entityA),
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})

	outcome := EvaluateRelationConstraints(fixture.environment, view)
	if outcome.Kind() != RelationConstraintsInvalid {
		t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
	}
	if !relationConstraintHasDiagnostic(outcome, DiagnosticReferenceEntityDuplicate, DiagnosticInvalid) {
		t.Fatalf("diagnostics = %v; want explicit duplicate EntityID", relationConstraintDiagnosticKeys(outcome))
	}
}

func TestEvaluateRelationConstraintsKeepsUnresolvedAndMissingReferenceResultsUnderdetermined(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	resolved := fixture.resolvedReference(t, entityA, "resolved-a")
	unresolved := fixture.unresolvedReference(t, "unresolved-b")
	missingBridge := fixture.missingBridgeReference(t, "missing-bridge-c")

	cases := []struct {
		name        string
		fillerCount uint64
		resolutions []StrongReferenceResolution
		wantCode    DiagnosticCode
	}{
		{
			name:        "explicit unresolved result",
			fillerCount: 2,
			resolutions: []StrongReferenceResolution{resolved, unresolved},
			wantCode:    DiagnosticReferenceUnresolved,
		},
		{
			name:        "missing resolution result",
			fillerCount: 2,
			resolutions: []StrongReferenceResolution{resolved},
			wantCode:    DiagnosticReferenceUnresolved,
		},
		{
			name:        "missing context bridge",
			fillerCount: 2,
			resolutions: []StrongReferenceResolution{resolved, missingBridge},
			wantCode:    DiagnosticContextBridgeMissing,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			whole, err := NewProspectiveRelationConstraintReferenceSlotView(
				fixture.relation.whole,
				test.fillerCount,
				test.resolutions,
			)
			if err != nil {
				t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView() error = %v", err)
			}
			view := fixture.prospectiveView(t, []relationConstraintTestSlot{
				{view: whole},
				fixture.referenceSlot(t, fixture.relation.left, entityA),
				fixture.valueSlot(t, fixture.relation.byValue, 1),
			})
			outcome := EvaluateRelationConstraints(fixture.environment, view)
			if outcome.Kind() != RelationConstraintsUnderdetermined {
				t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
			}
			if !relationConstraintHasDiagnostic(outcome, test.wantCode, DiagnosticUnderdetermined) {
				t.Fatalf("diagnostics = %v; want %s", relationConstraintDiagnosticKeys(outcome), test.wantCode)
			}
			for _, diagnostic := range outcome.Diagnostics() {
				if diagnostic.Posture() != DiagnosticUnderdetermined {
					t.Fatalf("underdetermined outcome contains posture %s", diagnostic.Posture())
				}
				if repair, found := diagnostic.Repair(); !found || repair.String() == "" {
					t.Fatal("underdetermined diagnostic lost its deterministic repair pointer")
				}
			}
		})
	}
}

func TestEvaluateRelationConstraintsRetainsKnownInvalidAlongsideMissingBasis(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	resolutions := []StrongReferenceResolution{
		fixture.resolvedReference(t, entityA, "a-first"),
		fixture.resolvedReference(t, entityA, "a-second"),
		fixture.unresolvedReference(t, "unresolved"),
	}
	whole, err := NewProspectiveRelationConstraintReferenceSlotView(
		fixture.relation.whole,
		uint64(len(resolutions)),
		resolutions,
	)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView() error = %v", err)
	}
	view := fixture.prospectiveView(t, []relationConstraintTestSlot{
		{view: whole},
		fixture.referenceSlot(t, fixture.relation.left, entityA),
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})

	outcome := EvaluateRelationConstraints(fixture.environment, view)
	if outcome.Kind() != RelationConstraintsInvalid {
		t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
	}
	if !relationConstraintHasDiagnostic(outcome, DiagnosticReferenceEntityDuplicate, DiagnosticInvalid) {
		t.Fatalf("diagnostics = %v; want retained known contradiction", relationConstraintDiagnosticKeys(outcome))
	}
	if !relationConstraintHasDiagnostic(outcome, DiagnosticReferenceUnresolved, DiagnosticUnderdetermined) {
		t.Fatalf("diagnostics = %v; want retained missing basis", relationConstraintDiagnosticKeys(outcome))
	}

	entityB := relationConstraintEntityID(t, "entity:b")
	partialSubset, err := NewProspectiveRelationConstraintReferenceSlotView(
		fixture.relation.left,
		2,
		[]StrongReferenceResolution{
			fixture.resolvedReference(t, entityB, "subset-b"),
			fixture.unresolvedReference(t, "subset-unresolved"),
		},
	)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView(subset) error = %v", err)
	}
	partialSubsetView := fixture.prospectiveView(t, []relationConstraintTestSlot{
		fixture.referenceSlot(t, fixture.relation.whole, entityA),
		{view: partialSubset},
		fixture.referenceSlot(t, fixture.relation.right, entityA),
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})
	partialSubsetOutcome := EvaluateRelationConstraints(fixture.environment, partialSubsetView)
	if partialSubsetOutcome.Kind() != RelationConstraintsInvalid ||
		!relationConstraintHasDiagnostic(partialSubsetOutcome, DiagnosticReferenceSubsetMismatch, DiagnosticInvalid) ||
		!relationConstraintHasDiagnostic(partialSubsetOutcome, DiagnosticReferenceUnresolved, DiagnosticUnderdetermined) {
		t.Fatalf("partial-subset diagnostics = %v; want known subset Invalid plus missing basis", relationConstraintDiagnosticKeys(partialSubsetOutcome))
	}

	partialWhole, err := NewProspectiveRelationConstraintReferenceSlotView(
		fixture.relation.whole,
		2,
		[]StrongReferenceResolution{
			fixture.resolvedReference(t, entityA, "whole-a"),
			fixture.unresolvedReference(t, "whole-unresolved"),
		},
	)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView(whole) error = %v", err)
	}
	partialPartitionView := fixture.prospectiveView(t, []relationConstraintTestSlot{
		{view: partialWhole},
		fixture.referenceSlot(t, fixture.relation.left, entityA),
		fixture.referenceSlot(t, fixture.relation.right, entityA),
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})
	partialPartitionOutcome := EvaluateRelationConstraints(fixture.environment, partialPartitionView)
	if partialPartitionOutcome.Kind() != RelationConstraintsInvalid ||
		!relationConstraintHasDiagnostic(partialPartitionOutcome, DiagnosticReferencePartitionMismatch, DiagnosticInvalid) ||
		!relationConstraintHasDiagnostic(partialPartitionOutcome, DiagnosticReferenceUnresolved, DiagnosticUnderdetermined) {
		t.Fatalf("partial-partition diagnostics = %v; want known overlap Invalid plus missing basis", relationConstraintDiagnosticKeys(partialPartitionOutcome))
	}
}

func TestEvaluateRelationConstraintsRequiresExactSignatureAndUnambiguousSlotCoordinates(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	whole := fixture.referenceSlot(t, fixture.relation.whole, entityA)

	_, err := NewProspectiveRelationConstraintEvaluationView(
		fixture.relation.signature.Ref(),
		[]RelationConstraintSlotView{whole.view, whole.view},
	)
	if err == nil {
		t.Fatal("prospective view accepted duplicate slot coordinates")
	}

	otherTypeEnv := typeEnvTestTypeEnvRef(t, 0x91)
	otherSignature := typeEnvTestSignatureRef(t, otherTypeEnv, fixture.relation.signature.Ref().ID().String())
	view, err := NewProspectiveRelationConstraintEvaluationView(
		otherSignature,
		[]RelationConstraintSlotView{whole.view},
	)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintEvaluationView() error = %v", err)
	}
	outcome := EvaluateRelationConstraints(fixture.environment, view)
	if outcome.Kind() != RelationConstraintsUnderdetermined ||
		!relationConstraintHasDiagnostic(outcome, DiagnosticTypeRuleUnavailable, DiagnosticUnderdetermined) {
		t.Fatalf("mismatched TypeEnv outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
	}

	unknownSlot := typeEnvTestSlotKindID(t, "Haft.UnknownSlot")
	unknown := fixture.referenceSlot(t, unknownSlot, entityA)
	unknownView := fixture.prospectiveView(t, []relationConstraintTestSlot{unknown})
	unknownOutcome := EvaluateRelationConstraints(fixture.environment, unknownView)
	if unknownOutcome.Kind() != RelationConstraintsInvalid ||
		!relationConstraintHasDiagnostic(unknownOutcome, DiagnosticUnknownSlot, DiagnosticInvalid) {
		t.Fatalf("unknown slot outcome = %s, diagnostics = %v", unknownOutcome.Kind(), relationConstraintDiagnosticKeys(unknownOutcome))
	}

	wrongMode := fixture.valueSlot(t, fixture.relation.whole, 1)
	wrongModeView := fixture.prospectiveView(t, []relationConstraintTestSlot{wrongMode})
	wrongModeOutcome := EvaluateRelationConstraints(fixture.environment, wrongModeView)
	if wrongModeOutcome.Kind() != RelationConstraintsInvalid ||
		!relationConstraintHasDiagnostic(wrongModeOutcome, DiagnosticReferenceModeMismatch, DiagnosticInvalid) {
		t.Fatalf("wrong-mode outcome = %s, diagnostics = %v", wrongModeOutcome.Kind(), relationConstraintDiagnosticKeys(wrongModeOutcome))
	}

	wrongRefKindID, err := NewReferenceID("reference:wrong-ref-kind")
	if err != nil {
		t.Fatalf("NewReferenceID() error = %v", err)
	}
	wrongRefKind, err := NewPersistedRef(
		fixture.relation.otherReferenceDefinition.Ref(),
		wrongRefKindID,
	)
	if err != nil {
		t.Fatalf("NewPersistedRef() error = %v", err)
	}
	basis, err := NewResolutionBasisRef("basis:wrong-ref-kind")
	if err != nil {
		t.Fatalf("NewResolutionBasisRef() error = %v", err)
	}
	wrongResolution, err := NewResolvedStrongReference(
		wrongRefKind,
		entityA,
		fixture.base.primaryContext.Ref(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference() error = %v", err)
	}
	wrongRefKindSlot, err := NewProspectiveRelationConstraintReferenceSlotView(
		fixture.relation.whole,
		1,
		[]StrongReferenceResolution{wrongResolution},
	)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView() error = %v", err)
	}
	wrongRefKindView := fixture.prospectiveView(t, []relationConstraintTestSlot{
		{view: wrongRefKindSlot},
		fixture.valueSlot(t, fixture.relation.byValue, 1),
	})
	wrongRefKindOutcome := EvaluateRelationConstraints(fixture.environment, wrongRefKindView)
	if wrongRefKindOutcome.Kind() != RelationConstraintsInvalid ||
		!relationConstraintHasDiagnostic(wrongRefKindOutcome, DiagnosticReferenceKindMismatch, DiagnosticInvalid) {
		t.Fatalf("wrong-RefKind outcome = %s, diagnostics = %v", wrongRefKindOutcome.Kind(), relationConstraintDiagnosticKeys(wrongRefKindOutcome))
	}
}

func TestRelationConstraintEvaluationViewFromFinalInstanceUsesSamePureEvaluator(t *testing.T) {
	fixture := newRelationConstraintEvaluationFixture(t)
	entityA := relationConstraintEntityID(t, "entity:a")
	entityB := relationConstraintEntityID(t, "entity:b")
	instance := fixture.finalInstance(t, []relationConstraintEntityReference{
		{entity: entityA, referenceSuffix: "whole-a"},
		{entity: entityB, referenceSuffix: "whole-b"},
	})

	view, err := NewRelationConstraintEvaluationViewFromInstance(instance)
	if err != nil {
		t.Fatalf("NewRelationConstraintEvaluationViewFromInstance() error = %v", err)
	}
	outcome := EvaluateRelationConstraints(fixture.environment, view)
	if outcome.Kind() != RelationConstraintsInvalid {
		// The final fixture intentionally contains Whole only. Partition then
		// proves a known mismatch rather than silently treating revalidation as
		// a different evaluator path.
		t.Fatalf("outcome = %s, diagnostics = %v", outcome.Kind(), relationConstraintDiagnosticKeys(outcome))
	}
	if !relationConstraintHasDiagnostic(outcome, DiagnosticReferencePartitionMismatch, DiagnosticInvalid) {
		t.Fatalf("diagnostics = %v; want partition mismatch", relationConstraintDiagnosticKeys(outcome))
	}
}

type relationConstraintEvaluationFixture struct {
	base        typeEnvFixture
	relation    referenceConstraintRelationFixture
	environment TypeEnv
}

func newRelationConstraintEvaluationFixture(t *testing.T) relationConstraintEvaluationFixture {
	t.Helper()
	base := newTypeEnvFixture(t)
	relation := newReferenceConstraintRelation(t, base)
	cardinality := mustTypedMemoryValue(NewSlotCardinalityConstraint(
		typeEnvTestConstraintID(t, "constraint:value-cardinality"),
		relation.signature.Ref(),
		relation.byValue,
		ExactlyOneCardinality(),
		base.provenance,
	))
	subset := mustTypedMemoryValue(NewReferenceSlotSubsetConstraint(
		typeEnvTestConstraintID(t, "constraint:left-subset-whole"),
		relation.signature.Ref(),
		relation.left,
		relation.whole,
		base.provenance,
	))
	partition := mustTypedMemoryValue(NewReferenceSlotPartitionConstraint(
		typeEnvTestConstraintID(t, "constraint:whole-partition"),
		relation.signature.Ref(),
		relation.whole,
		[]SlotKindID{relation.left, relation.right},
		base.provenance,
	))
	wholeCardinality := mustTypedMemoryValue(NewSlotCardinalityConstraint(
		typeEnvTestConstraintID(t, "constraint:whole-cardinality"),
		relation.signature.Ref(),
		relation.whole,
		NewUnboundedCardinality(0),
		base.provenance,
	))
	environment, err := base.builder().
		AddRefKindDefinition(relation.otherReferenceDefinition).
		AddRelationSignature(relation.signature).
		AddConstraint(cardinality).
		AddConstraint(subset).
		AddConstraint(partition).
		AddConstraint(wholeCardinality).
		Build()
	if err != nil {
		t.Fatalf("TypeEnv Build() error = %v", err)
	}
	return relationConstraintEvaluationFixture{
		base:        base,
		relation:    relation,
		environment: environment,
	}
}

type relationConstraintTestSlot struct {
	view RelationConstraintSlotView
}

type relationConstraintEntityReference struct {
	entity          EntityID
	referenceSuffix string
}

func (fixture relationConstraintEvaluationFixture) prospectiveView(
	t *testing.T,
	slots []relationConstraintTestSlot,
) RelationConstraintEvaluationView {
	t.Helper()
	views := make([]RelationConstraintSlotView, 0, len(slots))
	for _, slot := range slots {
		views = append(views, slot.view)
	}
	view, err := NewProspectiveRelationConstraintEvaluationView(fixture.relation.signature.Ref(), views)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintEvaluationView() error = %v", err)
	}
	return view
}

func (fixture relationConstraintEvaluationFixture) valueSlot(
	t *testing.T,
	slot SlotKindID,
	count uint64,
) relationConstraintTestSlot {
	t.Helper()
	view, err := NewRelationConstraintValueSlotView(slot, count)
	if err != nil {
		t.Fatalf("NewRelationConstraintValueSlotView() error = %v", err)
	}
	return relationConstraintTestSlot{view: view}
}

func (fixture relationConstraintEvaluationFixture) referenceSlot(
	t *testing.T,
	slot SlotKindID,
	entities ...EntityID,
) relationConstraintTestSlot {
	t.Helper()
	pairs := make([]relationConstraintEntityReference, 0, len(entities))
	for index, entity := range entities {
		pairs = append(pairs, relationConstraintEntityReference{
			entity:          entity,
			referenceSuffix: fmt.Sprintf("%s-%d", entity.String(), index),
		})
	}
	return fixture.referenceSlotWithPairs(t, slot, pairs)
}

func (fixture relationConstraintEvaluationFixture) referenceSlotWithPairs(
	t *testing.T,
	slot SlotKindID,
	pairs []relationConstraintEntityReference,
) relationConstraintTestSlot {
	t.Helper()
	resolutions := make([]StrongReferenceResolution, 0, len(pairs))
	for _, pair := range pairs {
		resolutions = append(resolutions, fixture.resolvedReference(t, pair.entity, pair.referenceSuffix))
	}
	view, err := NewProspectiveRelationConstraintReferenceSlotView(slot, uint64(len(pairs)), resolutions)
	if err != nil {
		t.Fatalf("NewProspectiveRelationConstraintReferenceSlotView() error = %v", err)
	}
	return relationConstraintTestSlot{view: view}
}

func (fixture relationConstraintEvaluationFixture) resolvedReference(
	t *testing.T,
	entity EntityID,
	suffix string,
) ResolvedStrongReference {
	t.Helper()
	reference := fixture.persistedReference(t, suffix)
	basis, err := NewResolutionBasisRef("basis:" + suffix)
	if err != nil {
		t.Fatalf("NewResolutionBasisRef() error = %v", err)
	}
	resolution, err := NewResolvedStrongReference(
		reference,
		entity,
		fixture.base.primaryContext.Ref(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewResolvedStrongReference() error = %v", err)
	}
	return resolution
}

func (fixture relationConstraintEvaluationFixture) unresolvedReference(
	t *testing.T,
	suffix string,
) UnresolvedStrongReference {
	t.Helper()
	reference := fixture.persistedReference(t, suffix)
	repair, err := NewRepairPointer("resolve-reference:" + suffix)
	if err != nil {
		t.Fatalf("NewRepairPointer() error = %v", err)
	}
	resolution, err := NewUnresolvedStrongReference(
		reference,
		fixture.base.primaryContext.Ref(),
		repair,
	)
	if err != nil {
		t.Fatalf("NewUnresolvedStrongReference() error = %v", err)
	}
	return resolution
}

func (fixture relationConstraintEvaluationFixture) missingBridgeReference(
	t *testing.T,
	suffix string,
) MissingContextBridgeResolution {
	t.Helper()
	reference := fixture.persistedReference(t, suffix)
	repair, err := NewRepairPointer("activate-context-bridge:" + suffix)
	if err != nil {
		t.Fatalf("NewRepairPointer() error = %v", err)
	}
	resolution, err := NewMissingContextBridgeResolution(
		reference,
		fixture.base.primaryContext.Ref(),
		fixture.base.secondaryContext.Ref(),
		fixture.base.entityKind.ID(),
		fixture.base.entityKind.ID(),
		repair,
	)
	if err != nil {
		t.Fatalf("NewMissingContextBridgeResolution() error = %v", err)
	}
	return resolution
}

func (fixture relationConstraintEvaluationFixture) persistedReference(
	t *testing.T,
	suffix string,
) PersistedRef {
	t.Helper()
	referenceID, err := NewReferenceID("reference:" + suffix)
	if err != nil {
		t.Fatalf("NewReferenceID() error = %v", err)
	}
	reference, err := NewPersistedRef(fixture.base.entityRefKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef() error = %v", err)
	}
	return reference
}

func (fixture relationConstraintEvaluationFixture) finalInstance(
	t *testing.T,
	whole []relationConstraintEntityReference,
) RelationInstance {
	t.Helper()
	firstReference := fixture.persistedReference(t, "candidate")
	candidateFiller, err := NewByReferenceCandidate(firstReference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate() error = %v", err)
	}
	candidateBinding, err := NewCandidateSlotBinding(
		fixture.relation.whole,
		[]CandidateSlotFiller{candidateFiller},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding() error = %v", err)
	}
	assertion, err := NewAssertionID("assertion:constraint-final-instance")
	if err != nil {
		t.Fatalf("NewAssertionID() error = %v", err)
	}
	provenance, err := NewProvenanceRef("memory:constraint-final-instance")
	if err != nil {
		t.Fatalf("NewProvenanceRef() error = %v", err)
	}
	contextSlice := mustContextSliceBuild(t, ContextSliceInput{
		Context:   fixture.base.primaryContext.Ref(),
		GammaTime: mustContextSlicePoint(t, "2026-07-17T08:00:00Z"),
	})
	candidate, err := NewRelationInstantiation(
		assertion,
		fixture.relation.signature.Ref(),
		contextSlice,
		[]CandidateSlotBinding{candidateBinding},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation() error = %v", err)
	}

	fillers := make([]SlotFiller, 0, len(whole))
	for _, pair := range whole {
		fillers = append(fillers, newReferenceFiller(
			fixture.persistedReference(t, pair.referenceSuffix),
			pair.entity,
		))
	}
	instance := newRelationInstance(
		candidate,
		[]SlotBinding{newSlotBinding(fixture.relation.whole, fillers)},
	)
	if !instance.valid() {
		t.Fatal("final relation fixture is invalid")
	}
	return instance
}

func relationConstraintEntityID(t *testing.T, raw string) EntityID {
	t.Helper()
	entity, err := NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	return entity
}

func relationConstraintHasDiagnostic(
	outcome RelationConstraintEvaluation,
	code DiagnosticCode,
	posture DiagnosticPosture,
) bool {
	for _, diagnostic := range outcome.Diagnostics() {
		if diagnostic.Code() == code && diagnostic.Posture() == posture {
			return true
		}
	}
	return false
}

func relationConstraintDiagnosticKeys(outcome RelationConstraintEvaluation) []string {
	result := make([]string, 0, len(outcome.Diagnostics()))
	for _, diagnostic := range outcome.Diagnostics() {
		result = append(result, fmt.Sprintf(
			"%s:%s:%s",
			diagnostic.Posture(),
			diagnostic.Code(),
			diagnostic.Path().String(),
		))
	}
	sort.Strings(result)
	return result
}

func relationConstraintIDs(values []ConstraintID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}
