package typedmemory

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDisjointCounterUseKeepsDirectAndEntailedBasesDistinct(t *testing.T) {
	fixture := newMemberOfFixture(t)
	environment := fixture.typeEnv.build(t)
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	supporting := admissionTestMember(t, fixture)
	direct := admissionTestDisjointNotMember(t, fixture)
	entailed := disjointCounterTestEntailment(
		t,
		environment,
		constraint,
		supporting,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
	)

	if direct.Kind() != DirectNotMemberDisjointCounterUse ||
		entailed.Kind() != EntailedDisjointCounterUse {
		t.Fatal("disjoint counter variants lost their explicit discriminants")
	}
	if direct.CounterQuery().Digest() != entailed.CounterQuery().Digest() ||
		!bytes.Equal(direct.CounterQuery().CanonicalBytes(), entailed.CounterQuery().CanonicalBytes()) {
		t.Fatal("direct and entailed uses did not address the same exact counter query")
	}
	if bytes.Equal(direct.CanonicalBytes(), entailed.CanonicalBytes()) ||
		direct.Digest() == entailed.Digest() {
		t.Fatal("direct NotMember and deductive entailment collapsed to one canonical basis")
	}
	if !validDisjointCounterUse(direct) || !validDisjointCounterUse(entailed) {
		t.Fatal("constructor produced an invalid sealed disjoint counter use")
	}
	entailedInterface := reflect.TypeOf((*DisjointEntailmentUse)(nil)).Elem()
	if _, exposesJudgement := entailedInterface.MethodByName("Judgement"); exposesJudgement {
		t.Fatal("DisjointEntailmentUse exposed a fabricated MemberOfNotMember judgement")
	}

	legacy, err := NewDisjointNotMemberUse(constraint.ID(), direct.Judgement())
	if err != nil {
		t.Fatalf("NewDisjointNotMemberUse() error = %v", err)
	}
	precise, err := NewDirectNotMemberUse(constraint.ID(), direct.Judgement())
	if err != nil {
		t.Fatalf("NewDirectNotMemberUse() error = %v", err)
	}
	if legacy.Digest() != precise.Digest() ||
		!bytes.Equal(legacy.CanonicalBytes(), precise.CanonicalBytes()) {
		t.Fatal("compatibility constructor changed the direct-use canonical identity")
	}
}

func TestDisjointEntailmentUseSealsExactConstraintAndCoordinates(t *testing.T) {
	fixture := newMemberOfFixture(t)
	environment := fixture.typeEnv.build(t)
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	supporting := admissionTestMember(t, fixture)
	use := disjointCounterTestEntailment(
		t,
		environment,
		constraint,
		supporting,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
	)

	if use.Constraint() != constraint.ID() ||
		use.ConstraintDigest() != digestAdmissionBytes(constraint.CanonicalBytes()) ||
		use.MatchedOperand() != fixture.typeEnv.entityKind.ID() ||
		use.ExcludedOperand() != fixture.typeEnv.claimGraphKind.ID() {
		t.Fatal("entailment lost its exact constraint coordinates")
	}
	counter := use.CounterQuery()
	if counter.EntityID() != supporting.Query().EntityID() ||
		counter.ValueKind() != fixture.typeEnv.claimGraphValueKind ||
		!sameContextSlice(counter.ContextSlice(), supporting.Query().ContextSlice()) ||
		!sameMemberOfEvaluationView(use.EvaluationView(), supporting.EvaluationView()) {
		t.Fatal("entailment counter query drifted from the supporting membership coordinates")
	}
	if use.SupportingMembership().Digest() != supporting.Digest() ||
		!bytes.Equal(use.SupportingMembership().CanonicalBytes(), supporting.CanonicalBytes()) {
		t.Fatal("entailment lost the exact supporting positive judgement")
	}

	canonical := use.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, use.CanonicalBytes()) {
		t.Fatal("CanonicalBytes exposed mutable entailment storage")
	}
	constraintCopy := use.ConstraintRule()
	constraintCopy.kinds[0] = fixture.typeEnv.systemKind.ID()
	if use.ConstraintRule().Kinds()[0] == fixture.typeEnv.systemKind.ID() {
		t.Fatal("ConstraintRule exposed mutable constraint operand storage")
	}

	forged := use.(disjointEntailmentUse)
	forged.constraintDigest = digestAdmissionBytes([]byte("different constraint"))
	if validDisjointEntailmentUse(forged) {
		t.Fatal("tampered constraint digest remained a valid entailment use")
	}
}

func TestDisjointEntailmentUseRejectsNonExactOrMisalignedInputs(t *testing.T) {
	fixture := newMemberOfFixture(t)
	environment := fixture.typeEnv.build(t)
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	supporting := admissionTestMember(t, fixture)
	base := DisjointEntailmentUseInput{
		TypeEnv:              environment,
		Constraint:           constraint,
		SupportingMembership: supporting,
		MatchedOperand:       fixture.typeEnv.entityKind.ID(),
		ExcludedOperand:      fixture.typeEnv.claimGraphKind.ID(),
	}

	wrongMatched := base
	wrongMatched.MatchedOperand = fixture.typeEnv.claimGraphKind.ID()
	if _, err := NewDisjointEntailmentUse(wrongMatched); err == nil {
		t.Fatal("entailment accepted a matched operand not supported by MemberOf")
	}

	sameOperand := base
	sameOperand.ExcludedOperand = fixture.typeEnv.entityKind.ID()
	if _, err := NewDisjointEntailmentUse(sameOperand); err == nil {
		t.Fatal("entailment accepted the matched operand as its own counter")
	}

	nonOperand := base
	nonOperand.ExcludedOperand = fixture.typeEnv.systemKind.ID()
	if _, err := NewDisjointEntailmentUse(nonOperand); err == nil {
		t.Fatal("entailment accepted a counter outside the exact constraint operands")
	}

	otherProvenance := typeEnvTestFPFProvenance(t, "prov:fpf:other-disjoint", 0xee)
	alteredConstraint, err := NewKindDisjointConstraint(
		constraint.ID(),
		constraint.Kinds(),
		otherProvenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint(altered) error = %v", err)
	}
	altered := base
	altered.Constraint = alteredConstraint
	if _, err := NewDisjointEntailmentUse(altered); err == nil {
		t.Fatal("entailment accepted a same-ID constraint with different canonical provenance")
	}

	zeroSupport := base
	zeroSupport.SupportingMembership = MemberOfMember{}
	if _, err := NewDisjointEntailmentUse(zeroSupport); err == nil {
		t.Fatal("entailment accepted an absent supporting MemberOf judgement")
	}
}

func TestDisjointCounterUseNormalizationAndCoveragePreserveEveryPosition(t *testing.T) {
	fixture := newMemberOfFixture(t)
	thirdKind := typeEnvTestKindDefinition(t, "U.Episteme", fixture.typeEnv.provenance)
	constraint := fixture.typeEnv.constraint.(KindDisjointConstraint)
	naryConstraint, err := NewKindDisjointConstraint(
		typeEnvTestConstraintID(t, "constraint:entity-claim-episteme-disjoint"),
		[]KindID{
			fixture.typeEnv.entityKind.ID(),
			fixture.typeEnv.claimGraphKind.ID(),
			thirdKind.ID(),
		},
		fixture.typeEnv.provenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint(n-ary) error = %v", err)
	}
	environment, err := fixture.typeEnv.builder().
		AddKindDefinition(thirdKind).
		AddContextKindAvailability(typeEnvTestKindAvailability(
			fixture.typeEnv.primaryContext.Ref(),
			thirdKind.ID(),
			fixture.typeEnv.provenance,
		)).
		AddConstraint(naryConstraint).
		Build()
	if err != nil {
		t.Fatalf("Build(n-ary TypeEnv) error = %v", err)
	}
	supporting := admissionTestMember(t, fixture)
	binary := disjointCounterTestEntailment(
		t,
		environment,
		constraint,
		supporting,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
	)
	naryClaim := disjointCounterTestEntailment(
		t,
		environment,
		naryConstraint,
		supporting,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
	)
	naryThird := disjointCounterTestEntailment(
		t,
		environment,
		naryConstraint,
		supporting,
		fixture.typeEnv.entityKind.ID(),
		thirdKind.ID(),
	)

	normalized, err := normalizeDisjointCounterUses([]DisjointCounterUse{
		naryThird,
		binary,
		naryClaim,
		naryThird,
	})
	if err != nil || len(normalized) != 3 {
		t.Fatalf("normalizeDisjointCounterUses() = (%d, %v); want three exact positions", len(normalized), err)
	}
	complete := []DisjointCounterUse{naryThird, binary, naryClaim}
	if err := validateExactDisjointCounterUseSet(environment, supporting, complete); err != nil {
		t.Fatalf("complete disjoint counter coverage rejected: %v", err)
	}
	if err := validateExactDisjointCounterUseSet(
		environment,
		supporting,
		[]DisjointCounterUse{binary, naryClaim},
	); err == nil {
		t.Fatal("coverage accepted a missing n-ary counter position")
	}

	direct := admissionTestDisjointNotMember(t, fixture)
	if _, err := normalizeDisjointCounterUses([]DisjointCounterUse{direct, binary}); err == nil {
		t.Fatal("normalizer collapsed conflicting direct and entailed bases for one counter position")
	}

	otherBasis := fixture.basis(t, []MemberOfObservableInput{
		memberOfTestObservableInput(t, "observable:registry/vehicle-7-other", 0xef),
	})
	otherSupporting, err := NewMemberOfMember(fixture.query, otherBasis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(other support) error = %v", err)
	}
	otherBinary := disjointCounterTestEntailment(
		t,
		environment,
		constraint,
		otherSupporting,
		fixture.typeEnv.entityKind.ID(),
		fixture.typeEnv.claimGraphKind.ID(),
	)
	if err := validateExactDisjointCounterUseSet(
		environment,
		supporting,
		[]DisjointCounterUse{otherBinary, naryClaim, naryThird},
	); err == nil {
		t.Fatal("coverage accepted an entailment backed by another positive MemberOf basis")
	}
}

func disjointCounterTestEntailment(
	t *testing.T,
	environment TypeEnv,
	constraint KindDisjointConstraint,
	supporting MemberOfMember,
	matched KindID,
	excluded KindID,
) DisjointEntailmentUse {
	t.Helper()
	use, err := NewDisjointEntailmentUse(DisjointEntailmentUseInput{
		TypeEnv:              environment,
		Constraint:           constraint,
		SupportingMembership: supporting,
		MatchedOperand:       matched,
		ExcludedOperand:      excluded,
	})
	if err != nil {
		t.Fatalf("NewDisjointEntailmentUse() error = %v", err)
	}
	return use
}
