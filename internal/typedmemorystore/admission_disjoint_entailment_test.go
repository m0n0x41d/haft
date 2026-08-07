package typedmemorystore

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestRebuildDisjointEntailmentUsesCurrentConstraintAndRequiredMembership(
	t *testing.T,
) {
	fixture := newStoreDisjointEntailmentFixture(t)

	rebuilt, err := rebuildDisjointEntailment(
		fixture.environment,
		fixture.supporting,
		fixture.entailment,
	)
	if err != nil {
		t.Fatalf("rebuildDisjointEntailment(): %v", err)
	}
	if rebuilt.Digest() != fixture.entailment.Digest() ||
		!bytes.Equal(rebuilt.CanonicalBytes(), fixture.entailment.CanonicalBytes()) {
		t.Fatal("revalidated entailment differs from the exact original proof")
	}

	_, err = rebuildDisjointEntailment(
		fixture.environment,
		fixture.alternateSupporting,
		fixture.entailment,
	)
	if !errors.Is(err, ErrAdmissionEnvelopeMismatch) {
		t.Fatalf("altered support error = %v; want ErrAdmissionEnvelopeMismatch", err)
	}

	_, err = rebuildDisjointEntailment(
		fixture.alteredEnvironment,
		fixture.supporting,
		fixture.entailment,
	)
	if !errors.Is(err, ErrAdmissionEnvelopeMismatch) {
		t.Fatalf("altered constraint error = %v; want ErrAdmissionEnvelopeMismatch", err)
	}
}

func TestTransactionAdmissionSnapshotResolvesExactDisjointEntailment(t *testing.T) {
	fixture := newStoreDisjointEntailmentFixture(t)
	candidate := fixture.base.declaration(t, "snapshot-proof", "Snapshot proof")
	batch := sealGenericDeclaration(t, fixture.base, candidate, 0)
	basis := batch.Basis()
	snapshot, err := newTransactionAdmissionSnapshotWithClassifications(
		basis,
		basis.SnapshotObservations(),
		nil,
		nil,
		nil,
		[]typedmemory.DisjointEntailmentUse{fixture.entailment},
	)
	if err != nil {
		t.Fatalf("newTransactionAdmissionSnapshotWithEntailments(): %v", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(
		fixture.entailment.CounterQuery(),
		fixture.entailment.EvaluationView(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	resolved, found := snapshot.ResolveDisjointEntailment(
		request,
		fixture.constraint.ID(),
		fixture.supporting,
	)
	if !found {
		t.Fatal("exact revalidated disjoint entailment was not resolved")
	}
	if resolved.Digest() != fixture.entailment.Digest() ||
		!bytes.Equal(resolved.CanonicalBytes(), fixture.entailment.CanonicalBytes()) {
		t.Fatal("transaction snapshot returned another disjoint entailment")
	}
	otherConstraint, err := typedmemory.NewConstraintID("constraint:test:other-disjoint")
	if err != nil {
		t.Fatalf("NewConstraintID(other): %v", err)
	}
	if _, found := snapshot.ResolveDisjointEntailment(
		request,
		otherConstraint,
		fixture.supporting,
	); found {
		t.Fatal("transaction snapshot broadened an exact disjoint-entailment lookup")
	}
	if _, found := snapshot.ResolveDisjointEntailment(
		request,
		fixture.constraint.ID(),
		fixture.alternateSupporting,
	); found {
		t.Fatal("transaction snapshot resolved an entailment for another positive support")
	}
}

func TestTransactionAdmissionSnapshotKeysEntailmentsByExactPositiveSupport(
	t *testing.T,
) {
	fixture := newStoreDisjointEntailmentFixture(t)
	alternate, err := typedmemory.NewDisjointEntailmentUse(
		typedmemory.DisjointEntailmentUseInput{
			TypeEnv:              fixture.environment,
			Constraint:           fixture.constraint,
			SupportingMembership: fixture.alternateSupporting,
			MatchedOperand:       fixture.entailment.MatchedOperand(),
			ExcludedOperand:      fixture.entailment.ExcludedOperand(),
		},
	)
	if err != nil {
		t.Fatalf("NewDisjointEntailmentUse(alternate support): %v", err)
	}
	candidate := fixture.base.declaration(
		t,
		"snapshot-two-proofs",
		"Snapshot with two proofs",
	)
	batch := sealGenericDeclaration(t, fixture.base, candidate, 0)
	basis := batch.Basis()
	snapshot, err := newTransactionAdmissionSnapshotWithClassifications(
		basis,
		basis.SnapshotObservations(),
		nil,
		nil,
		nil,
		[]typedmemory.DisjointEntailmentUse{fixture.entailment, alternate},
	)
	if err != nil {
		t.Fatalf("newTransactionAdmissionSnapshotWithEntailments(two supports): %v", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(
		fixture.entailment.CounterQuery(),
		fixture.entailment.EvaluationView(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}

	first, found := snapshot.ResolveDisjointEntailment(
		request,
		fixture.constraint.ID(),
		fixture.supporting,
	)
	if !found || first.Digest() != fixture.entailment.Digest() {
		t.Fatal("snapshot did not resolve the entailment for the first exact support")
	}
	second, found := snapshot.ResolveDisjointEntailment(
		request,
		fixture.constraint.ID(),
		fixture.alternateSupporting,
	)
	if !found || second.Digest() != alternate.Digest() {
		t.Fatal("snapshot did not resolve the entailment for the second exact support")
	}
}

type storeDisjointEntailmentFixture struct {
	base                sqliteStoreFixture
	environment         typedmemory.TypeEnv
	alteredEnvironment  typedmemory.TypeEnv
	constraint          typedmemory.KindDisjointConstraint
	supporting          typedmemory.MemberOfMember
	alternateSupporting typedmemory.MemberOfMember
	entailment          typedmemory.DisjointEntailmentUse
}

func newStoreDisjointEntailmentFixture(t *testing.T) storeDisjointEntailmentFixture {
	t.Helper()
	base := newSQLiteStoreFixture(t)
	provenance := mustFPFProvenance(t, base.snapshot.SourceRevision())
	boundedContext, exists := base.environment.BoundedContext(base.context)
	if !exists {
		t.Fatal("base fixture bounded context is missing")
	}
	entityKindID := mustGenericKindID(t, "U.Entity")
	claimKindID := mustGenericKindID(t, "U.ClaimGraph")
	otherKindID := mustGenericKindID(t, "U.Other")
	entityKind := storeDisjointKindDefinition(t, entityKindID, provenance)
	claimKind := storeDisjointKindDefinition(t, claimKindID, provenance)
	otherKind := storeDisjointKindDefinition(t, otherKindID, provenance)
	entityValueKind, err := typedmemory.NewValueKindRef(base.environment.Ref(), entityKindID)
	if err != nil {
		t.Fatalf("NewValueKindRef(entity): %v", err)
	}
	entityAvailability := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		entityKindID,
		provenance,
		"disjoint-entailment.entity-availability",
	)
	claimAvailability := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		claimKindID,
		provenance,
		"disjoint-entailment.claim-availability",
	)
	otherAvailability := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		otherKindID,
		provenance,
		"disjoint-entailment.other-availability",
	)
	entitySet, err := typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
		TypeEnv:         base.environment.Ref(),
		Context:         base.context,
		EnumerationRule: exactBasisRuleRef(t, "test:disjoint-entailment/entity-set/v1"),
		CandidatePolicy: typedmemory.PersistedEntitiesOnly{},
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewEntitySetDefinition(): %v", err)
	}
	kindSignature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       entityValueKind,
			Formality:       typedmemory.SignatureF4,
			DefinednessRule: exactBasisRuleRef(t, "test:disjoint-entailment/definedness/v1"),
			Evaluator:       exactBasisRuleRef(t, "test:disjoint-entailment/evaluator/v1"),
			EntitySet:       entitySet.Ref(),
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition(): %v", err)
	}
	constraintID, err := typedmemory.NewConstraintID("constraint:test:entity-claim-disjoint")
	if err != nil {
		t.Fatalf("NewConstraintID(): %v", err)
	}
	constraint, err := typedmemory.NewKindDisjointConstraint(
		constraintID,
		[]typedmemory.KindID{entityKindID, claimKindID},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint(): %v", err)
	}
	environment := storeDisjointTypeEnv(
		t,
		base,
		boundedContext,
		[]typedmemory.KindDefinition{entityKind, claimKind},
		[]typedmemory.ContextKindAvailability{entityAvailability, claimAvailability},
		entitySet,
		kindSignature,
		constraint,
	)
	alteredConstraint, err := typedmemory.NewKindDisjointConstraint(
		constraintID,
		[]typedmemory.KindID{entityKindID, otherKindID},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint(altered): %v", err)
	}
	alteredEnvironment := storeDisjointTypeEnv(
		t,
		base,
		boundedContext,
		[]typedmemory.KindDefinition{entityKind, otherKind},
		[]typedmemory.ContextKindAvailability{entityAvailability, otherAvailability},
		entitySet,
		kindSignature,
		alteredConstraint,
	)
	contextSlice := storeDisjointContextSlice(t, base.context)
	query, err := typedmemory.NewMemberOfQuery(
		mustGenericEntityID(t, "entity:disjoint-entailment"),
		entityValueKind,
		contextSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(): %v", err)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		base.environment.Ref(),
		typedmemory.NewGraphRevision(0),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView(): %v", err)
	}
	supporting := storeDisjointSupportingMembership(
		t,
		query,
		view,
		entitySet,
		kindSignature,
		"primary",
	)
	alternateSupporting := storeDisjointSupportingMembership(
		t,
		query,
		view,
		entitySet,
		kindSignature,
		"alternate",
	)
	entailment, err := typedmemory.NewDisjointEntailmentUse(
		typedmemory.DisjointEntailmentUseInput{
			TypeEnv:              environment,
			Constraint:           constraint,
			SupportingMembership: supporting,
			MatchedOperand:       entityKindID,
			ExcludedOperand:      claimKindID,
		},
	)
	if err != nil {
		t.Fatalf("NewDisjointEntailmentUse(): %v", err)
	}
	return storeDisjointEntailmentFixture{
		base:                base,
		environment:         environment,
		alteredEnvironment:  alteredEnvironment,
		constraint:          constraint,
		supporting:          supporting,
		alternateSupporting: alternateSupporting,
		entailment:          entailment,
	}
}

func storeDisjointTypeEnv(
	t *testing.T,
	base sqliteStoreFixture,
	boundedContext typedmemory.BoundedContext,
	kinds []typedmemory.KindDefinition,
	availabilities []typedmemory.ContextKindAvailability,
	entitySet typedmemory.EntitySetDefinition,
	kindSignature typedmemory.KindSignatureDefinition,
	constraint typedmemory.KindDisjointConstraint,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(base.environment.Ref()).
		SetSourceRevision(base.snapshot.SourceRevision()).
		SetCompilerSchemaVersion(base.snapshot.CompilerSchemaVersion()).
		SetCoverageManifest(base.environment.CoverageManifest()).
		AddBoundedContext(boundedContext)
	for _, kind := range kinds {
		builder = builder.AddKindDefinition(kind)
	}
	for _, availability := range availabilities {
		builder = builder.AddContextKindAvailability(availability)
	}
	environment, err := builder.
		AddEntitySetDefinition(entitySet).
		AddKindSignatureDefinition(kindSignature).
		AddConstraint(constraint).
		Build()
	if err != nil {
		t.Fatalf("build disjoint-entailment TypeEnv: %v", err)
	}
	return environment
}

func storeDisjointKindDefinition(
	t *testing.T,
	kind typedmemory.KindID,
	provenance typedmemory.DeclarationProvenance,
) typedmemory.KindDefinition {
	t.Helper()
	definition, err := typedmemory.NewKindDefinition(kind, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition(%s): %v", kind.String(), err)
	}
	return definition
}

func storeDisjointContextSlice(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	gamma, err := typedmemory.NewGammaPoint(time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewGammaPoint(): %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice(): %v", err)
	}
	return contextSlice
}

func storeDisjointSupportingMembership(
	t *testing.T,
	query typedmemory.MemberOfQuery,
	view typedmemory.MemberOfEvaluationView,
	entitySet typedmemory.EntitySetDefinition,
	kindSignature typedmemory.KindSignatureDefinition,
	suffix string,
) typedmemory.MemberOfMember {
	t.Helper()
	evaluationProvenance, err := typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         mustGenericProvenanceRef(t, "memory:test:disjoint-entailment/"+suffix),
			EvaluatorArtifact: exactBasisCarrierRef(t, "binary:disjoint-entailment-"+suffix),
			EvaluatorEdition:  exactBasisCarrierEdition(t, "build-20260717."+suffix),
			EvaluatorDigest:   mustDigest(t, []byte("disjoint-entailment-"+suffix)),
		},
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationProvenance(%s): %v", suffix, err)
	}
	inputRef, err := typedmemory.NewObservableInputRef("observable:disjoint-entailment/" + suffix)
	if err != nil {
		t.Fatalf("NewObservableInputRef(%s): %v", suffix, err)
	}
	input, err := typedmemory.NewMemberOfObservableInput(
		inputRef,
		mustDigest(t, []byte("observable-disjoint-entailment-"+suffix)),
	)
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput(%s): %v", suffix, err)
	}
	basis, err := typedmemory.NewMemberOfBasis(typedmemory.MemberOfBasisInput{
		Query:                query,
		EvaluationView:       view,
		KindSignature:        kindSignature,
		EntitySet:            entitySet,
		ObservableInputs:     []typedmemory.MemberOfObservableInput{input},
		EvaluationProvenance: evaluationProvenance,
	})
	if err != nil {
		t.Fatalf("NewMemberOfBasis(%s): %v", suffix, err)
	}
	judgement, err := typedmemory.NewMemberOfMember(query, basis)
	if err != nil {
		t.Fatalf("NewMemberOfMember(%s): %v", suffix, err)
	}
	return judgement
}
