package typedmemory

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestTypeEnvRetainsExecutableMembershipDefinitionsSeparateFromContextKindAvailability(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	entitySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.ref,
		fixture.primaryContext.Ref(),
		"test:entity-set/primary/v1",
		fixture.provenance,
	)
	assumption := typeEnvTestKindAssumption(
		t,
		"standard:entity-registry",
		"v2.3",
		0xa1,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF4,
		[]KindAssumptionPin{assumption},
		"test:member-of/entity/v1",
		entitySet.Ref(),
		fixture.provenance,
	)

	environment, err := fixture.builderWithoutBridge().
		AddEntitySetDefinition(entitySet).
		AddKindSignatureDefinition(signature).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	retainedSet, exists := environment.EntitySetDefinition(entitySet.Ref())
	if !exists {
		t.Fatal("EntitySet definition was not retained")
	}
	if retainedSet.EnumerationRule().String() != "test:entity-set/primary/v1" {
		t.Fatalf("EntitySet rule = %q", retainedSet.EnumerationRule().String())
	}
	retainedSignature, exists := environment.KindSignatureDefinition(
		fixture.entityValueKind,
		fixture.primaryContext.Ref(),
	)
	if !exists {
		t.Fatal("KindSignature definition was not retained")
	}
	if retainedSignature.Formality() != SignatureF4 {
		t.Fatalf("KindSignature formality = %s; want F4", retainedSignature.Formality().String())
	}
	if retainedSignature.EntitySet() != entitySet.Ref() {
		t.Fatal("KindSignature lost its exact EntitySet definition")
	}
	if !bytes.Equal(retainedSignature.CanonicalBytes(), signature.CanonicalBytes()) {
		t.Fatal("KindSignature canonical bytes changed during TypeEnv build")
	}

	assumptions := retainedSignature.Assumptions()
	assumptions[0] = KindAssumptionPin{}
	retainedAgain, _ := environment.KindSignatureDefinition(
		fixture.entityValueKind,
		fixture.primaryContext.Ref(),
	)
	if len(retainedAgain.Assumptions()) != 1 || !retainedAgain.Assumptions()[0].valid() {
		t.Fatal("mutating a KindSignature accessor changed the immutable TypeEnv")
	}

	if !environment.HasKindInContext(fixture.primaryContext.Ref(), fixture.systemKind.ID()) {
		t.Fatal("fixture must admit U.System in the primary context")
	}
	systemValueKind := typeEnvTestValueKindRef(t, fixture.ref, fixture.systemKind.ID())
	if _, exists := environment.KindSignatureDefinition(
		systemValueKind,
		fixture.primaryContext.Ref(),
	); exists {
		t.Fatal("ContextKindAvailability was incorrectly promoted to executable MemberOf semantics")
	}
}

func TestTypeEnvRejectsBrokenOrDuplicateMembershipDefinitions(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	entitySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.ref,
		fixture.primaryContext.Ref(),
		"test:entity-set/primary/v1",
		fixture.provenance,
	)
	signature := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF4,
		nil,
		"test:member-of/entity/v1",
		entitySet.Ref(),
		fixture.provenance,
	)

	_, err := fixture.builderWithoutBridge().AddKindSignatureDefinition(signature).Build()
	if err == nil || !strings.Contains(err.Error(), "unknown EntitySet") {
		t.Fatalf("missing EntitySet error = %v; want unknown EntitySet", err)
	}

	secondSignature := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF5,
		nil,
		"test:member-of/entity/v2",
		entitySet.Ref(),
		fixture.provenance,
	)
	_, err = fixture.builderWithoutBridge().
		AddEntitySetDefinition(entitySet).
		AddKindSignatureDefinition(signature).
		AddKindSignatureDefinition(secondSignature).
		Build()
	if err == nil || !strings.Contains(err.Error(), "duplicate KindSignature") {
		t.Fatalf("duplicate KindSignature error = %v", err)
	}

	_, err = NewSignatureFormality(10)
	if err == nil {
		t.Fatal("formality outside F0..F9 was accepted")
	}
}

func TestKindSignatureResolutionIsContextLocal(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	valueKind := typeEnvTestValueKindRef(t, fixture.ref, fixture.systemKind.ID())
	primarySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.ref,
		fixture.primaryContext.Ref(),
		"test:entity-set/primary/v1",
		fixture.provenance,
	)
	secondarySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.ref,
		fixture.secondaryContext.Ref(),
		"test:entity-set/secondary/v1",
		fixture.provenance,
	)
	primarySignature := typeEnvTestKindSignatureDefinition(
		t,
		valueKind,
		SignatureF4,
		nil,
		"test:member-of/system/primary/v1",
		primarySet.Ref(),
		fixture.provenance,
	)
	secondarySignature := typeEnvTestKindSignatureDefinition(
		t,
		valueKind,
		SignatureF5,
		nil,
		"test:member-of/system/secondary/v1",
		secondarySet.Ref(),
		fixture.provenance,
	)
	environment, err := fixture.builderWithoutBridge().
		AddEntitySetDefinition(secondarySet).
		AddEntitySetDefinition(primarySet).
		AddKindSignatureDefinition(secondarySignature).
		AddKindSignatureDefinition(primarySignature).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	primary, primaryExists := environment.KindSignatureDefinition(
		valueKind,
		fixture.primaryContext.Ref(),
	)
	secondary, secondaryExists := environment.KindSignatureDefinition(
		valueKind,
		fixture.secondaryContext.Ref(),
	)
	if !primaryExists || !secondaryExists {
		t.Fatal("context-local KindSignatures were not both retained")
	}
	if primary.Ref() == secondary.Ref() || primary.Evaluator() == secondary.Evaluator() {
		t.Fatal("context-local KindSignatures collapsed to one definition")
	}

	unavailable := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF4,
		nil,
		"test:member-of/entity/secondary/v1",
		secondarySet.Ref(),
		fixture.provenance,
	)
	_, err = fixture.builderWithoutBridge().
		AddEntitySetDefinition(secondarySet).
		AddKindSignatureDefinition(unavailable).
		Build()
	if err == nil || !strings.Contains(err.Error(), "unavailable in context") {
		t.Fatalf("context-local signature with unavailable kind error = %v", err)
	}
}

func TestKindSignatureCanonicalizesExactAssumptionsAndRequiresExplicitFormality(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	entitySet := typeEnvTestEntitySetDefinition(
		t,
		fixture.ref,
		fixture.primaryContext.Ref(),
		"test:entity-set/primary/v1",
		fixture.provenance,
	)
	first := typeEnvTestKindAssumption(t, "standard:registry", "v2.3", 0xa1)
	second := typeEnvTestKindAssumption(t, "policy:vehicle-shape", "2026-07", 0xa2)
	forward := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF4,
		[]KindAssumptionPin{first, second, first},
		"test:member-of/entity/v1",
		entitySet.Ref(),
		fixture.provenance,
	)
	reverse := typeEnvTestKindSignatureDefinition(
		t,
		fixture.entityValueKind,
		SignatureF4,
		[]KindAssumptionPin{second, first},
		"test:member-of/entity/v1",
		entitySet.Ref(),
		fixture.provenance,
	)
	if forward.Ref() != reverse.Ref() ||
		!bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("assumption order or exact duplicate changed KindSignature identity")
	}
	if len(forward.Assumptions()) != 2 {
		t.Fatalf("assumption count = %d; want exact duplicate deduplication", len(forward.Assumptions()))
	}

	conflict := typeEnvTestKindAssumption(t, first.Reference().String(), "v2.4", 0xa3)
	_, err := NewKindSignatureDefinition(KindSignatureDefinitionInput{
		ValueKind:       fixture.entityValueKind,
		Formality:       SignatureF4,
		Assumptions:     []KindAssumptionPin{first, conflict},
		DefinednessRule: typeEnvTestRuleRef(t, "test:member-of/entity/v1/definedness"),
		Evaluator:       typeEnvTestRuleRef(t, "test:member-of/entity/v1"),
		EntitySet:       entitySet.Ref(),
		Provenance:      fixture.provenance,
	})
	if err == nil {
		t.Fatal("one assumption reference with conflicting exact pins was accepted")
	}

	if SignatureFormality(0).valid() {
		t.Fatal("zero-value SignatureFormality was accepted as an implicit F0 declaration")
	}
	f0, err := NewSignatureFormality(0)
	if err != nil || f0 != SignatureF0 || f0.String() != "F0" {
		t.Fatalf("explicit F0 = %v, %v; want SignatureF0", f0, err)
	}
}

func TestTypeEnvRetainsA65SlotKindValueKindAndRefKind(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	environment := fixture.build(t)

	fragment, exists := environment.TypedRelationDeclarationFragment(
		fixture.relationFragment.Ref(),
	)
	if !exists {
		t.Fatal("typed relation declaration fragment was not retained")
	}
	if fragment.Posture() != RelationDeclarationTypedFragment {
		t.Fatalf("fragment posture = %q", fragment.Posture())
	}
	entitySlot, exists := fragment.Slot(fixture.entitySlot)
	if !exists {
		t.Fatal("EntityOfConcernSlot was not retained")
	}
	referenceTarget, ok := entitySlot.Target().(ReferenceSlotTarget)
	if !ok {
		t.Fatalf("EntityOfConcernSlot target = %T; want ReferenceSlotTarget", entitySlot.Target())
	}
	if referenceTarget.ValueKind() != fixture.entityValueKind {
		t.Fatal("reference slot lost its ValueKind")
	}
	if referenceTarget.ReferenceKind() != fixture.entityRefKind {
		t.Fatal("reference slot lost its RefKind")
	}
	if entitySlot.RefMode() != SlotByReference {
		t.Fatalf("EntityOfConcernSlot mode = %s; want by_reference", entitySlot.RefMode().String())
	}
	if _, accidentallyKind := environment.KindDefinition(typeEnvTestKindID(t, fixture.entitySlot.String())); accidentallyKind {
		t.Fatal("EntityOfConcernSlot was fabricated as a U.Kind")
	}
	if !environment.HasKindInContext(fixture.primaryContext.Ref(), fixture.entityKind.ID()) {
		t.Fatal("context-local U.Entity availability was lost")
	}
	if !environment.IsSubkind(fixture.systemKind.ID(), fixture.entityKind.ID()) {
		t.Fatal("U.System should be an available compatible subkind of U.Entity")
	}
	if !environment.HasContextBridge(
		fixture.primaryContext.Ref(),
		fixture.secondaryContext.Ref(),
		fixture.systemKind.ID(),
		fixture.systemKind.ID(),
	) {
		t.Fatal("exact context bridge was not retained")
	}
	if _, exists := environment.ValueBinding(fixture.claimGraphValueKind); !exists {
		t.Fatal("ClaimGraph value binding was not retained")
	}
}

func TestTypeEnvBuildIsImmutableAndCanonicalizesDeclarationOrder(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	builder := fixture.builder()
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	contexts := environment.BoundedContexts()
	contexts[0] = BoundedContext{}
	if _, exists := environment.BoundedContext(fixture.primaryContext.Ref()); !exists {
		t.Fatal("mutating an accessor slice changed the immutable TypeEnv")
	}

	extraContext := typeEnvTestBoundedContext(t, "ctx:later", fixture.provenance)
	builder.AddBoundedContext(extraContext)
	if _, exists := environment.BoundedContext(extraContext.Ref()); exists {
		t.Fatal("mutating the builder after Build changed the immutable TypeEnv")
	}

	signatures := environment.RelationSignatures()
	slots := signatures[0].Slots()
	for index := 1; index < len(slots); index++ {
		if slots[index-1].SlotKind().String() > slots[index].SlotKind().String() {
			t.Fatal("SlotSpecs are not in canonical SlotKind order")
		}
	}
}

func TestTypeEnvRejectsMissingBasisAndSubkindCycle(t *testing.T) {
	fixture := newTypeEnvFixture(t)

	unknownKind := typeEnvTestKindID(t, "U.Unknown")
	badAvailability := typeEnvTestKindAvailability(
		fixture.primaryContext.Ref(),
		unknownKind,
		fixture.provenance,
	)
	_, err := fixture.builder().AddContextKindAvailability(badAvailability).Build()
	if err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("missing kind basis error = %v; want unknown kind", err)
	}

	reverse, err := NewSubkindRelation(
		fixture.entityKind.ID(),
		fixture.systemKind.ID(),
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation() error = %v", err)
	}
	_, err = fixture.builder().AddSubkindRelation(reverse).Build()
	if err == nil || !strings.Contains(err.Error(), "subkind cycle") {
		t.Fatalf("subkind-cycle error = %v; want cycle", err)
	}
}

func TestTypeEnvContextBridgeRequiresBothEndpointKindSignatures(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	tests := []struct {
		name  string
		build func() *TypeEnvBuilder
		want  string
	}{
		{
			name: "missing source signature",
			build: func() *TypeEnvBuilder {
				return fixture.builderWithoutBridge().
					AddEntitySetDefinition(fixture.bridgeTargetSet).
					AddKindSignatureDefinition(fixture.bridgeTargetSig).
					AddContextBridge(fixture.bridge)
			},
			want: "requires source KindSignature",
		},
		{
			name: "missing target signature",
			build: func() *TypeEnvBuilder {
				return fixture.builderWithoutBridge().
					AddEntitySetDefinition(fixture.bridgeSourceSet).
					AddKindSignatureDefinition(fixture.bridgeSourceSig).
					AddContextBridge(fixture.bridge)
			},
			want: "requires target KindSignature",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.build().Build()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v; want %q", err, test.want)
			}
		})
	}

	if _, err := fixture.builder().Build(); err != nil {
		t.Fatalf("bridge with both exact endpoint KindSignatures rejected: %v", err)
	}
}

func TestSubkindIdentityIsInjectiveAcrossDelimiterCharacters(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	leftSubkind := typeEnvTestKindDefinition(t, "test.a<b", fixture.provenance)
	leftSuperkind := typeEnvTestKindDefinition(t, "test.c", fixture.provenance)
	rightSubkind := typeEnvTestKindDefinition(t, "test.a", fixture.provenance)
	rightSuperkind := typeEnvTestKindDefinition(t, "test.b<c", fixture.provenance)
	left, err := NewSubkindRelation(
		leftSubkind.ID(),
		leftSuperkind.ID(),
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation(left): %v", err)
	}
	right, err := NewSubkindRelation(
		rightSubkind.ID(),
		rightSuperkind.ID(),
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewSubkindRelation(right): %v", err)
	}
	if left.key() == right.key() {
		t.Fatal("distinct subkind pairs have one composite identity")
	}
	_, err = fixture.builder().
		AddKindDefinition(leftSubkind).
		AddKindDefinition(leftSuperkind).
		AddKindDefinition(rightSubkind).
		AddKindDefinition(rightSuperkind).
		AddSubkindRelation(left).
		AddSubkindRelation(right).
		Build()
	if err != nil {
		t.Fatalf("TypeEnv rejected distinct subkind pairs: %v", err)
	}
}

func TestTypeEnvRejectsNilConstraintBeforeCanonicalSorting(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	if _, err := fixture.builder().AddConstraint(nil).Build(); err == nil {
		t.Fatal("TypeEnv accepted nil constraint")
	}
}

func TestNewValueShapeDeclarationRejectsUnrelatedDigest(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	shape := mustValueShapeIdentityScalar(t, ScalarText)
	unrelated := typeEnvTestShapeRef(t, "test.UnrelatedShape", 0x91)
	_, err := NewValueShapeDeclaration(unrelated, shape, fixture.provenance)
	if err == nil {
		t.Fatal("NewValueShapeDeclaration accepted an unrelated caller digest")
	}
	if !strings.Contains(err.Error(), "does not match canonical shape identity") {
		t.Fatalf("NewValueShapeDeclaration error = %v", err)
	}
}

func TestTypeEnvRejectsMissingChildShapeDeterministically(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	missingA := typeEnvTestShapeRef(t, "test.MissingA", 0xa1)
	missingZ := typeEnvTestShapeRef(t, "test.MissingZ", 0xa2)
	parentA := typeEnvTestSequenceShapeDeclaration(
		t,
		"test.ParentA",
		missingA,
		fixture.provenance,
	)
	parentZ := typeEnvTestSequenceShapeDeclaration(
		t,
		"test.ParentZ",
		missingZ,
		fixture.provenance,
	)

	_, forwardErr := fixture.builderWithoutBridge().
		AddValueShape(parentZ).
		AddValueShape(parentA).
		Build()
	_, reverseErr := fixture.builderWithoutBridge().
		AddValueShape(parentA).
		AddValueShape(parentZ).
		Build()
	if forwardErr == nil || reverseErr == nil {
		t.Fatal("TypeEnv accepted a missing child ValueShapeRef")
	}
	if forwardErr.Error() != reverseErr.Error() {
		t.Fatalf(
			"missing-child diagnostic depends on insertion order: %q != %q",
			forwardErr.Error(),
			reverseErr.Error(),
		)
	}
	expected := fmt.Sprintf(
		"value-shape declaration %q references missing child shape %q",
		parentA.Ref().String(),
		missingA.String(),
	)
	if forwardErr.Error() != expected {
		t.Fatalf("missing-child diagnostic = %q, want %q", forwardErr.Error(), expected)
	}
}

func TestTypeEnvRejectsValueShapeCycleDeterministically(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	leftRef := typeEnvTestShapeRef(t, "test.CycleA", 0xb1)
	rightRef := typeEnvTestShapeRef(t, "test.CycleB", 0xb2)
	leftShape, err := NewOrderedSequenceShape(rightRef)
	if err != nil {
		t.Fatalf("NewOrderedSequenceShape(left): %v", err)
	}
	rightShape, err := NewOrderedSequenceShape(leftRef)
	if err != nil {
		t.Fatalf("NewOrderedSequenceShape(right): %v", err)
	}
	left := ValueShapeDeclaration{
		ref:        leftRef,
		shape:      leftShape,
		provenance: fixture.provenance,
	}
	right := ValueShapeDeclaration{
		ref:        rightRef,
		shape:      rightShape,
		provenance: fixture.provenance,
	}

	_, forwardErr := fixture.builderWithoutBridge().
		AddValueShape(right).
		AddValueShape(left).
		Build()
	_, reverseErr := fixture.builderWithoutBridge().
		AddValueShape(left).
		AddValueShape(right).
		Build()
	if forwardErr == nil || reverseErr == nil {
		t.Fatal("TypeEnv accepted a ValueShape dependency cycle")
	}
	if forwardErr.Error() != reverseErr.Error() {
		t.Fatalf(
			"cycle diagnostic depends on insertion order: %q != %q",
			forwardErr.Error(),
			reverseErr.Error(),
		)
	}
	expected := fmt.Sprintf(
		"value-shape dependency cycle: %s -> %s -> %s",
		leftRef.String(),
		rightRef.String(),
		leftRef.String(),
	)
	if forwardErr.Error() != expected {
		t.Fatalf("cycle diagnostic = %q, want %q", forwardErr.Error(), expected)
	}
}

func TestCoverageManifestDoesNotPromoteSourceOnlyTextToRule(t *testing.T) {
	location := typeEnvTestSourceLocation(t, 0x21)
	compiledSubject, err := SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage() error = %v", err)
	}
	compiled, err := NewCompiledCoverageEntry(compiledSubject, location)
	if err != nil {
		t.Fatalf("NewCompiledCoverageEntry() error = %v", err)
	}

	unsupportedUnit := typeEnvTestSourceUnitID(t, "spec:pattern_body:a-14")
	unsupportedLocation, err := NewPatternedSourceLocation(
		unsupportedUnit,
		location.Revision(),
		typeEnvTestDigest(t, 0x22),
		typeEnvTestLineRange(t, 200, 240),
		typeEnvTestPatternID(t, "A.14"),
	)
	if err != nil {
		t.Fatalf("NewPatternedSourceLocation() error = %v", err)
	}
	unsupportedSubject, _ := SourceUnitCoverage(unsupportedUnit)
	unsupported, err := NewUnsupportedCoverageEntry(
		unsupportedSubject,
		unsupportedLocation,
		"heterogeneous prose has no supported normalized declaration",
	)
	if err != nil {
		t.Fatalf("NewUnsupportedCoverageEntry() error = %v", err)
	}
	manifest, err := NewCoverageManifest([]CoverageEntry{unsupported, compiled})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}

	entries := manifest.Entries()
	if len(entries) != 2 {
		t.Fatalf("coverage entries = %d; want 2", len(entries))
	}
	postures := map[CoveragePosture]bool{}
	for _, entry := range entries {
		postures[entry.Posture()] = true
	}
	if !postures[CoverageCompiled] || !postures[CoverageUnsupported] {
		t.Fatalf("coverage postures = %v; want compiled and unsupported", postures)
	}
}

type typeEnvFixture struct {
	ref                 TypeEnvRef
	revision            SourceRevision
	compiler            CompilerSchemaVersion
	coverage            CoverageManifest
	provenance          DeclarationProvenance
	primaryContext      BoundedContext
	secondaryContext    BoundedContext
	entityKind          KindDefinition
	systemKind          KindDefinition
	claimGraphKind      KindDefinition
	entityValueKind     ValueKindRef
	claimGraphValueKind ValueKindRef
	entityRefKind       RefKindRef
	refKindDefinition   RefKindDefinition
	subkind             SubkindRelation
	bridge              ContextBridge
	bridgeSourceSet     EntitySetDefinition
	bridgeTargetSet     EntitySetDefinition
	bridgeSourceSig     KindSignatureDefinition
	bridgeTargetSig     KindSignatureDefinition
	entitySlot          SlotKindID
	claimGraphSlot      SlotKindID
	shapeDeclaration    ValueShapeDeclaration
	binding             ValueBinding
	relationFragment    TypedRelationDeclarationFragment
	// signature preserves the fixture spelling used by edition-compatibility
	// tests. It is the same structurally limited fragment.
	signature  RelationSignature
	constraint ConstraintRule
}

func newTypeEnvFixture(t *testing.T) typeEnvFixture {
	t.Helper()
	ref := typeEnvTestTypeEnvRef(t, 0x31)
	provenance := typeEnvTestFPFProvenance(t, "prov:fpf:typeenv", 0x32)
	primary := typeEnvTestBoundedContext(t, "ctx:haft", provenance)
	secondary := typeEnvTestBoundedContext(t, "ctx:release", provenance)
	entityKind := typeEnvTestKindDefinition(t, "U.Entity", provenance)
	systemKind := typeEnvTestKindDefinition(t, "U.System", provenance)
	claimGraphKind := typeEnvTestKindDefinition(t, "U.ClaimGraph", provenance)
	entityValueKind := typeEnvTestValueKindRef(t, ref, entityKind.ID())
	claimGraphValueKind := typeEnvTestValueKindRef(t, ref, claimGraphKind.ID())
	entityRefID := typeEnvTestRefKindID(t, "U.EntityRef")
	entityRef := typeEnvTestRefKindRef(t, ref, entityRefID)
	refDefinition, err := NewRefKindDefinition(entityRef, entityValueKind, provenance)
	if err != nil {
		t.Fatalf("NewRefKindDefinition() error = %v", err)
	}
	subkind, err := NewSubkindRelation(systemKind.ID(), entityKind.ID(), provenance)
	if err != nil {
		t.Fatalf("NewSubkindRelation() error = %v", err)
	}
	bridgeID := typeEnvTestBridgeID(t, "bridge:haft-release")
	bridge := typeEnvTestContextBridge(
		t,
		bridgeID,
		primary.Ref(),
		secondary.Ref(),
		systemKind.ID(),
		systemKind.ID(),
		TwoWayBridge,
		provenance,
	)
	systemValueKind := typeEnvTestValueKindRef(t, ref, systemKind.ID())
	bridgeSourceSet := typeEnvTestEntitySetDefinition(
		t,
		ref,
		primary.Ref(),
		"test:entity-set/bridge-source/v1",
		provenance,
	)
	bridgeTargetSet := typeEnvTestEntitySetDefinition(
		t,
		ref,
		secondary.Ref(),
		"test:entity-set/bridge-target/v1",
		provenance,
	)
	bridgeSourceSig := typeEnvTestKindSignatureDefinition(
		t,
		systemValueKind,
		SignatureF4,
		nil,
		"test:member-of/bridge-source/v1",
		bridgeSourceSet.Ref(),
		provenance,
	)
	bridgeTargetSig := typeEnvTestKindSignatureDefinition(
		t,
		systemValueKind,
		SignatureF4,
		nil,
		"test:member-of/bridge-target/v1",
		bridgeTargetSet.Ref(),
		provenance,
	)
	shape := NewClaimGraphShape()
	shapeRef := typeEnvTestDerivedShapeRef(t, "ClaimGraphV1", shape)
	shapeDeclaration, err := NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration() error = %v", err)
	}
	codecRef := typeEnvTestCodecRef(t, "ClaimGraphCodecV1", 0x34)
	binding, err := NewValueBinding(claimGraphValueKind, shapeRef, codecRef, provenance)
	if err != nil {
		t.Fatalf("NewValueBinding() error = %v", err)
	}
	entitySlot := typeEnvTestSlotKindID(t, "EntityOfConcernSlot")
	claimGraphSlot := typeEnvTestSlotKindID(t, "ClaimGraphSlot")
	entityTarget, _ := NewReferenceSlotTarget(entityValueKind, entityRef)
	claimTarget, _ := NewValueSlotTarget(claimGraphValueKind)
	entitySpec, _ := NewSlotSpec(entitySlot, entityTarget, ExactlyOneCardinality(), provenance)
	claimSpec, _ := NewSlotSpec(claimGraphSlot, claimTarget, ExactlyOneCardinality(), provenance)
	fragmentRef := typeEnvTestSignatureRef(t, ref, "C.2.1.EpistemeSlotRelation")
	fragment, err := NewTypedRelationDeclarationFragment(
		fragmentRef,
		[]BoundedContextRef{secondary.Ref(), primary.Ref()},
		[]SlotSpec{claimSpec, entitySpec},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewTypedRelationDeclarationFragment() error = %v", err)
	}
	constraintID := typeEnvTestConstraintID(t, "constraint:entity-claim-disjoint")
	constraint, err := NewKindDisjointConstraint(
		constraintID,
		[]KindID{entityKind.ID(), claimGraphKind.ID()},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint() error = %v", err)
	}
	location := provenance.Location()
	subject, _ := SourceUnitCoverage(location.UnitID())
	entry, _ := NewCompiledCoverageEntry(subject, location)
	coverage, _ := NewCoverageManifest([]CoverageEntry{entry})
	revision := typeEnvTestSourceRevision(t, "44dd88188a07646ef23aca32627a3f670525853f")
	compiler := typeEnvTestCompilerVersion(t, "cov-2.v1")

	return typeEnvFixture{
		ref:                 ref,
		revision:            revision,
		compiler:            compiler,
		coverage:            coverage,
		provenance:          provenance,
		primaryContext:      primary,
		secondaryContext:    secondary,
		entityKind:          entityKind,
		systemKind:          systemKind,
		claimGraphKind:      claimGraphKind,
		entityValueKind:     entityValueKind,
		claimGraphValueKind: claimGraphValueKind,
		entityRefKind:       entityRef,
		refKindDefinition:   refDefinition,
		subkind:             subkind,
		bridge:              bridge,
		bridgeSourceSet:     bridgeSourceSet,
		bridgeTargetSet:     bridgeTargetSet,
		bridgeSourceSig:     bridgeSourceSig,
		bridgeTargetSig:     bridgeTargetSig,
		entitySlot:          entitySlot,
		claimGraphSlot:      claimGraphSlot,
		shapeDeclaration:    shapeDeclaration,
		binding:             binding,
		relationFragment:    fragment,
		signature:           fragment,
		constraint:          constraint,
	}
}

func (fixture typeEnvFixture) builder() *TypeEnvBuilder {
	return fixture.builderWithoutBridge().
		AddEntitySetDefinition(fixture.bridgeSourceSet).
		AddEntitySetDefinition(fixture.bridgeTargetSet).
		AddKindSignatureDefinition(fixture.bridgeSourceSig).
		AddKindSignatureDefinition(fixture.bridgeTargetSig).
		AddContextBridge(fixture.bridge)
}

func (fixture typeEnvFixture) builderWithoutBridge() *TypeEnvBuilder {
	return NewTypeEnvBuilder(fixture.ref).
		SetSourceRevision(fixture.revision).
		SetCompilerSchemaVersion(fixture.compiler).
		SetCoverageManifest(fixture.coverage).
		AddBoundedContext(fixture.secondaryContext).
		AddBoundedContext(fixture.primaryContext).
		AddKindDefinition(fixture.systemKind).
		AddKindDefinition(fixture.entityKind).
		AddKindDefinition(fixture.claimGraphKind).
		AddRefKindDefinition(fixture.refKindDefinition).
		AddContextKindAvailability(typeEnvTestKindAvailability(fixture.primaryContext.Ref(), fixture.entityKind.ID(), fixture.provenance)).
		AddContextKindAvailability(typeEnvTestKindAvailability(fixture.primaryContext.Ref(), fixture.systemKind.ID(), fixture.provenance)).
		AddContextKindAvailability(typeEnvTestKindAvailability(fixture.primaryContext.Ref(), fixture.claimGraphKind.ID(), fixture.provenance)).
		AddContextKindAvailability(typeEnvTestKindAvailability(fixture.secondaryContext.Ref(), fixture.systemKind.ID(), fixture.provenance)).
		AddSubkindRelation(fixture.subkind).
		AddTypedRelationDeclarationFragment(fixture.relationFragment).
		AddValueShape(fixture.shapeDeclaration).
		AddValueBinding(fixture.binding).
		AddConstraint(fixture.constraint)
}

func (fixture typeEnvFixture) build(t *testing.T) TypeEnv {
	t.Helper()
	environment, err := fixture.builder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return environment
}

func typeEnvTestFPFProvenance(t *testing.T, raw string, fill byte) FPFSourceProvenance {
	t.Helper()
	reference := typeEnvTestProvenanceRef(t, raw)
	location := typeEnvTestSourceLocation(t, fill)
	ruleID := typeEnvTestCompilerRuleID(t, "cov2.a6_5.slot.v1")
	provenance, err := NewFPFSourceProvenance(reference, location, ruleID)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance() error = %v", err)
	}
	return provenance
}

func typeEnvTestSourceLocation(t *testing.T, fill byte) SourceLocation {
	t.Helper()
	location, err := NewPatternedSourceLocation(
		typeEnvTestSourceUnitID(t, fmt.Sprintf("spec:pattern_body:a-6-5:%02x", fill)),
		typeEnvTestSourceRevision(t, "44dd88188a07646ef23aca32627a3f670525853f"),
		typeEnvTestDigest(t, fill),
		typeEnvTestLineRange(t, 16540, 16610),
		typeEnvTestPatternID(t, "A.6.5"),
	)
	if err != nil {
		t.Fatalf("NewPatternedSourceLocation() error = %v", err)
	}
	return location
}

func typeEnvTestDigest(t *testing.T, fill byte) SHA256Digest {
	t.Helper()
	digest, err := NewSHA256Digest("sha256:" + strings.Repeat(fmt.Sprintf("%02x", fill), 32))
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}

func typeEnvTestTypeEnvRef(t *testing.T, fill byte) TypeEnvRef {
	t.Helper()
	ref, err := NewTypeEnvRef(typeEnvTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	return ref
}

func typeEnvTestSourceUnitID(t *testing.T, raw string) SourceUnitID {
	t.Helper()
	value, err := NewSourceUnitID(raw)
	if err != nil {
		t.Fatalf("NewSourceUnitID() error = %v", err)
	}
	return value
}

func typeEnvTestSourceRevision(t *testing.T, raw string) SourceRevision {
	t.Helper()
	value, err := NewSourceRevision(raw)
	if err != nil {
		t.Fatalf("NewSourceRevision() error = %v", err)
	}
	return value
}

func typeEnvTestLineRange(t *testing.T, start, end uint64) SourceLineRange {
	t.Helper()
	value, err := NewSourceLineRange(start, end)
	if err != nil {
		t.Fatalf("NewSourceLineRange() error = %v", err)
	}
	return value
}

func typeEnvTestPatternID(t *testing.T, raw string) PatternID {
	t.Helper()
	value, err := NewPatternID(raw)
	if err != nil {
		t.Fatalf("NewPatternID() error = %v", err)
	}
	return value
}

func typeEnvTestCompilerRuleID(t *testing.T, raw string) CompilerRuleID {
	t.Helper()
	value, err := NewCompilerRuleID(raw)
	if err != nil {
		t.Fatalf("NewCompilerRuleID() error = %v", err)
	}
	return value
}

func typeEnvTestProvenanceRef(t *testing.T, raw string) ProvenanceRef {
	t.Helper()
	value, err := NewProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewProvenanceRef() error = %v", err)
	}
	return value
}

func typeEnvTestContextRef(t *testing.T, raw string) BoundedContextRef {
	t.Helper()
	value, err := NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	return value
}

func typeEnvTestBoundedContext(
	t *testing.T,
	raw string,
	provenance DeclarationProvenance,
) BoundedContext {
	t.Helper()
	value, err := NewBoundedContext(typeEnvTestContextRef(t, raw), provenance)
	if err != nil {
		t.Fatalf("NewBoundedContext() error = %v", err)
	}
	return value
}

func typeEnvTestKindID(t *testing.T, raw string) KindID {
	t.Helper()
	value, err := NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	return value
}

func typeEnvTestKindDefinition(
	t *testing.T,
	raw string,
	provenance DeclarationProvenance,
) KindDefinition {
	t.Helper()
	value, err := NewKindDefinition(typeEnvTestKindID(t, raw), provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition() error = %v", err)
	}
	return value
}

func typeEnvTestValueKindRef(t *testing.T, environment TypeEnvRef, id KindID) ValueKindRef {
	t.Helper()
	value, err := NewValueKindRef(environment, id)
	if err != nil {
		t.Fatalf("NewValueKindRef() error = %v", err)
	}
	return value
}

func typeEnvTestRefKindID(t *testing.T, raw string) RefKindID {
	t.Helper()
	value, err := NewRefKindID(raw)
	if err != nil {
		t.Fatalf("NewRefKindID() error = %v", err)
	}
	return value
}

func typeEnvTestRefKindRef(t *testing.T, environment TypeEnvRef, id RefKindID) RefKindRef {
	t.Helper()
	value, err := NewRefKindRef(environment, id)
	if err != nil {
		t.Fatalf("NewRefKindRef() error = %v", err)
	}
	return value
}

func typeEnvTestBridgeID(t *testing.T, raw string) ContextBridgeID {
	t.Helper()
	value, err := NewContextBridgeID(raw)
	if err != nil {
		t.Fatalf("NewContextBridgeID() error = %v", err)
	}
	return value
}

func typeEnvTestContextBridge(
	t *testing.T,
	id ContextBridgeID,
	sourceContext BoundedContextRef,
	targetContext BoundedContextRef,
	sourceKind KindID,
	targetKind KindID,
	direction BridgeDirection,
	provenance DeclarationProvenance,
) ContextBridge {
	t.Helper()
	source := mustTypedMemoryValue(NewContextBridgeEndpoint(
		sourceContext,
		mustTypedMemoryValue(NewContextEdition("1.0.0-source")),
	))
	target := mustTypedMemoryValue(NewContextBridgeEndpoint(
		targetContext,
		mustTypedMemoryValue(NewContextEdition("1.0.0-target")),
	))
	mapping := mustTypedMemoryValue(NewNamedTargetKindMapping(sourceKind, targetKind))
	congruence := mustTypedMemoryValue(NewKindCongruenceLevel(2))
	lossNotes := mustTypedMemoryValue(NewKindBridgeLossNotes([]string{
		"No source SubkindOf links are covered by this v1 bridge.",
	}))
	definedness := mustTypedMemoryValue(NewKindBridgeDefinednessArea([]string{
		"The exact pinned context editions are active.",
	}))
	return mustTypedMemoryValue(NewContextBridge(ContextBridgeInput{
		ID:              id,
		Source:          source,
		Target:          target,
		Mapping:         mapping,
		Direction:       direction,
		OrderCoverage:   NoOrderLinksCovered,
		KindCongruence:  congruence,
		LossNotes:       lossNotes,
		DefinednessArea: definedness,
		Provenance:      provenance,
	}))
}

func typeEnvTestShapeRef(t *testing.T, raw string, fill byte) ValueShapeRef {
	t.Helper()
	shapeID, err := NewShapeID(raw)
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	value, err := NewValueShapeRef(shapeID, typeEnvTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewValueShapeRef() error = %v", err)
	}
	return value
}

func typeEnvTestDerivedShapeRef(
	t *testing.T,
	raw string,
	shape ValueShape,
) ValueShapeRef {
	t.Helper()
	shapeID, err := NewShapeID(raw)
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	value, err := DeriveValueShapeRef(shapeID, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef() error = %v", err)
	}
	return value
}

func typeEnvTestSequenceShapeDeclaration(
	t *testing.T,
	idRaw string,
	child ValueShapeRef,
	provenance DeclarationProvenance,
) ValueShapeDeclaration {
	t.Helper()
	shape, err := NewOrderedSequenceShape(child)
	if err != nil {
		t.Fatalf("NewOrderedSequenceShape(): %v", err)
	}
	ref := typeEnvTestDerivedShapeRef(t, idRaw, shape)
	declaration, err := NewValueShapeDeclaration(ref, shape, provenance)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration(): %v", err)
	}
	return declaration
}

func typeEnvTestCodecRef(t *testing.T, raw string, fill byte) CodecRef {
	t.Helper()
	codecID, err := NewCodecID(raw)
	if err != nil {
		t.Fatalf("NewCodecID() error = %v", err)
	}
	version, err := NewCanonicalizationVersion("v1")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion() error = %v", err)
	}
	value, err := NewCodecRef(codecID, version, typeEnvTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewCodecRef() error = %v", err)
	}
	return value
}

func typeEnvTestSlotKindID(t *testing.T, raw string) SlotKindID {
	t.Helper()
	value, err := NewSlotKindID(raw)
	if err != nil {
		t.Fatalf("NewSlotKindID() error = %v", err)
	}
	return value
}

func typeEnvTestSignatureRef(t *testing.T, environment TypeEnvRef, raw string) RelationSignatureRef {
	t.Helper()
	id, err := NewSignatureID(raw)
	if err != nil {
		t.Fatalf("NewSignatureID() error = %v", err)
	}
	value, err := NewRelationSignatureRef(environment, id)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef() error = %v", err)
	}
	return value
}

func typeEnvTestConstraintID(t *testing.T, raw string) ConstraintID {
	t.Helper()
	value, err := NewConstraintID(raw)
	if err != nil {
		t.Fatalf("NewConstraintID() error = %v", err)
	}
	return value
}

func typeEnvTestKindAvailability(
	context BoundedContextRef,
	kind KindID,
	provenance DeclarationProvenance,
) ContextKindAvailability {
	writer := newCanonicalWriter("test.context-kind-availability-extension.v1")
	writer.addString(context.String())
	writer.addString(kind.String())
	writer.addBytes(provenance.CanonicalBytes())
	digest := writer.digest()
	symbol := mustTypedMemoryValue(KindSymbolRef(kind))
	manifest := mustTypedMemoryValue(NewSignatureManifestRef(
		"test.context-kind-availability",
		"1.0.0",
	))
	basis := mustTypedMemoryValue(NewManifestSymbolBasis(
		manifest,
		ManifestProvide,
		symbol,
	))
	projectProvenance := mustTypedMemoryValue(
		NewProjectSourceProvenanceBuilder(
			mustTypedMemoryValue(NewProvenanceRef("prov:test:context-kind-availability")),
			mustTypedMemoryValue(NewCarrierRef("carrier:test:context-kind-availability")),
			mustTypedMemoryValue(NewCarrierEdition("1.0.0")),
			digest,
		).
			SetDeclarationRange(mustTypedMemoryValue(NewSourceLineRange(1, 1))).
			SetCompilerRule(mustTypedMemoryValue(NewCompilerRuleID("test.context-kind-availability.v1"))).
			SetBoundedContext(context).
			SetBaseTypeEnv(mustTypedMemoryValue(NewTypeEnvRef(digest))).
			SetSignatureBlockRow(VocabularyRow).
			SetManifestBasis(basis).
			Build(),
	)
	contextSource := mustTypedMemoryValue(
		NewContextKindAvailabilitySource(context.String(), projectProvenance),
	)
	declarationSource := mustTypedMemoryValue(
		NewContextKindAvailabilitySource(kind.String(), projectProvenance),
	)
	extensionID := mustTypedMemoryValue(NewExtensionID(manifest.ID()))
	extensionRef := mustTypedMemoryValue(newTypeEnvExtensionRef(extensionID, digest))
	provider := mustTypedMemoryValue(NewExtensionKindAvailabilityProvider(
		ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      extensionRef,
			Context:           context,
			ContextSource:     contextSource,
			Symbol:            symbol,
			DeclarationSource: declarationSource,
		},
	))
	ground := mustTypedMemoryValue(NewLocalContextKindAvailabilityGround(
		LocalContextKindAvailabilityGroundInput{
			Context:             context,
			KindID:              kind,
			ContextSource:       contextSource,
			ApplicabilitySource: contextSource,
			Provider:            provider,
		},
	))
	groundSet := mustTypedMemoryValue(NewContextKindAvailabilityGroundSet(
		[]ContextKindAvailabilityGround{ground},
	))
	return mustTypedMemoryValue(NewContextKindAvailability(context, kind, groundSet))
}

func typeEnvTestCompilerVersion(t *testing.T, raw string) CompilerSchemaVersion {
	t.Helper()
	value, err := NewCompilerSchemaVersion(raw)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion() error = %v", err)
	}
	return value
}

func typeEnvTestCarrierRef(t *testing.T, raw string) CarrierRef {
	t.Helper()
	value, err := NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef() error = %v", err)
	}
	return value
}

func typeEnvTestCarrierEdition(t *testing.T, raw string) CarrierEdition {
	t.Helper()
	value, err := NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition() error = %v", err)
	}
	return value
}

func typeEnvTestRuleRef(t *testing.T, raw string) RuleRef {
	t.Helper()
	value, err := NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef() error = %v", err)
	}
	return value
}

func typeEnvTestEntitySetDefinition(
	t *testing.T,
	typeEnv TypeEnvRef,
	context BoundedContextRef,
	rule string,
	provenance DeclarationProvenance,
) EntitySetDefinition {
	return typeEnvTestEntitySetDefinitionWithPolicy(
		t,
		typeEnv,
		context,
		rule,
		PersistedEntitiesOnly{},
		provenance,
	)
}

func typeEnvTestEntitySetDefinitionWithPolicy(
	t *testing.T,
	typeEnv TypeEnvRef,
	context BoundedContextRef,
	rule string,
	policy EntitySetCandidatePolicy,
	provenance DeclarationProvenance,
) EntitySetDefinition {
	t.Helper()
	value, err := NewEntitySetDefinition(EntitySetDefinitionInput{
		TypeEnv:         typeEnv,
		Context:         context,
		EnumerationRule: typeEnvTestRuleRef(t, rule),
		CandidatePolicy: policy,
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewEntitySetDefinition() error = %v", err)
	}
	return value
}

func typeEnvTestKindAssumption(
	t *testing.T,
	reference string,
	edition string,
	fill byte,
) KindAssumptionPin {
	t.Helper()
	value, err := NewKindAssumptionPin(
		typeEnvTestCarrierRef(t, reference),
		typeEnvTestCarrierEdition(t, edition),
		typeEnvTestDigest(t, fill),
	)
	if err != nil {
		t.Fatalf("NewKindAssumptionPin() error = %v", err)
	}
	return value
}

func typeEnvTestKindSignatureDefinition(
	t *testing.T,
	kind ValueKindRef,
	formality SignatureFormality,
	assumptions []KindAssumptionPin,
	evaluator string,
	entitySet EntitySetDefinitionRef,
	provenance DeclarationProvenance,
) KindSignatureDefinition {
	t.Helper()
	value, err := NewKindSignatureDefinition(KindSignatureDefinitionInput{
		ValueKind:       kind,
		Formality:       formality,
		Assumptions:     assumptions,
		DefinednessRule: typeEnvTestRuleRef(t, evaluator+"/definedness"),
		Evaluator:       typeEnvTestRuleRef(t, evaluator),
		EntitySet:       entitySet,
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition() error = %v", err)
	}
	return value
}

func typeEnvTestKindSymbol(t *testing.T, raw string) SchemaSymbolRef {
	t.Helper()
	value, err := KindSymbolRef(typeEnvTestKindID(t, raw))
	if err != nil {
		t.Fatalf("KindSymbolRef() error = %v", err)
	}
	return value
}

func typeEnvTestManifestRef(t *testing.T) SignatureManifestRef {
	t.Helper()
	value, err := NewSignatureManifestRef("haft.typed-memory", "v1")
	if err != nil {
		t.Fatalf("NewSignatureManifestRef() error = %v", err)
	}
	return value
}
