package problemcardadapter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const (
	problemCardSignatureID = "Haft.ProblemCardAtConcern"
	problemCardSlotID      = "Haft.ProblemCardAtConcern.ProblemCardSlot"
	entityOfConcernSlotID  = "Haft.ProblemCardAtConcern.EntityOfConcernSlot"
	claimGraphSlotID       = "Haft.ProblemCardAtConcern.ClaimGraphSlot"
	projectRecordKindID    = "Haft.ProjectRecord"
	projectRecordRefID     = "Haft.ProjectRecordRef"
	entityKindID           = "U.Entity"
	entityRefID            = "U.EntityRef"
	claimGraphKindID       = "U.ClaimGraph"
)

func TestAdaptBuildsGenericRecordAndProblemCardAtExactConcern(t *testing.T) {
	fixture := newAdapterFixture(t)
	result := Adapt(fixture.draft, fixture.runtime, fixture.concern)
	candidate, ok := result.(ValidCandidate)
	if !ok {
		t.Fatalf("Adapt() result = %T, want ValidCandidate", result)
	}
	changes := candidate.ChangeSet().Changes()
	if len(changes) != 2 {
		t.Fatalf("candidate changes = %d, want declaration plus relation", len(changes))
	}
	declaration, ok := changes[0].(typedmemory.DeclareEntity)
	if !ok {
		t.Fatalf("first candidate change = %T, want DeclareEntity", changes[0])
	}
	if declaration.Entity() != fixture.draft.RecordEntity() {
		t.Fatal("ProblemCard adapter declared a different project-record identity")
	}
	assertRelation, ok := changes[1].(typedmemory.AssertRelation)
	if !ok {
		t.Fatalf("second candidate change = %T, want AssertRelation", changes[1])
	}
	relation := assertRelation.Assertion()
	if relation.Modality().Kind() != typedmemory.AssertionModalityAffirmsObtaining {
		t.Fatalf("relation modality = %q, want affirms_obtaining", relation.Modality().Kind())
	}
	if relation.Signature().ID().String() != problemCardSignatureID {
		t.Fatalf("relation signature = %q, want %q", relation.Signature().ID(), problemCardSignatureID)
	}
	assertExactNoteBindings(t, relation, fixture)

	carrier := candidate.Carrier()
	if carrier.EntityID() != fixture.draft.RecordEntity() ||
		carrier.BoundedContext() != fixture.context {
		t.Fatal("GenericProjectRecord carrier lost record identity or context")
	}
	if carrier.Variant().Token() != (recordcarrier.GenericProjectRecordVariantV1{}).Token() {
		t.Fatalf("record carrier variant = %q, want generic project record", carrier.Variant().Token())
	}
	manifest := mustValueTest(CurrentMappingManifestV1())
	if candidate.MappingManifestRef() != manifest.Ref() ||
		candidate.AdapterVersion() != manifest.AdapterVersion() ||
		candidate.CarrierBinding().MappingManifestRef() != manifest.Ref() ||
		candidate.CarrierBinding().AdapterVersion() != manifest.AdapterVersion() {
		t.Fatal("record-carrier binding lost the exact ProblemCard mapping coordinate")
	}
	source := candidate.MembershipSource()
	verified, err := recordcarrier.VerifyRecordMembershipSourceV1(
		source.ObservableInput(),
		source.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyRecordMembershipSourceV1() error = %v", err)
	}
	if verified.EntityID() != fixture.draft.RecordEntity() ||
		verified.RecordVariant().Token() != carrier.Variant().Token() {
		t.Fatal("membership source round trip changed record identity or variant")
	}
}

func TestAdaptCanonicalizesClaimGraphPermutation(t *testing.T) {
	fixture := newAdapterFixture(t)
	forward := validCandidate(t, Adapt(fixture.draft, fixture.runtime, fixture.concern))
	reversedDraft := draftWithClaimGraph(
		t,
		fixture.draft,
		mustValueTest(NewExactClaimGraph(reversedClaimGraph(t, fixture))),
	)
	reverse := validCandidate(t, Adapt(reversedDraft, fixture.runtime, fixture.concern))
	forwardDigest, err := forward.ChangeSet().Digest()
	if err != nil {
		t.Fatalf("forward ChangeSet.Digest() error = %v", err)
	}
	reverseDigest, err := reverse.ChangeSet().Digest()
	if err != nil {
		t.Fatalf("reverse ChangeSet.Digest() error = %v", err)
	}
	if forwardDigest != reverseDigest {
		t.Fatal("ClaimGraph node permutation changed ProblemCard candidate identity")
	}
}

func TestAdaptKeepsMissingConcernAndMappingBasisUnderdetermined(t *testing.T) {
	fixture := newAdapterFixture(t)
	missingConcern := mustMissingBasisTest(
		t,
		"entity_of_concern_resolution",
		"repair:resolve-entity-of-concern",
	)
	unsettled, err := NewUnsettledConcernBinding([]MissingBasis{missingConcern})
	if err != nil {
		t.Fatalf("NewUnsettledConcernBinding() error = %v", err)
	}
	assertUnderdetermined(t, Adapt(fixture.draft, fixture.runtime, unsettled), "entity_of_concern_resolution")

	missingCodecRuntime := rebuildExactRuntimeBasis(
		t,
		fixture.runtime,
		fixture.environment,
		typedmemory.NewCodecRegistry(),
	)
	assertUnderdetermined(t, Adapt(fixture.draft, missingCodecRuntime, fixture.concern), "claim_graph_codec")

	missingRelationEnvironment := environmentWithoutProblemCardRelation(t, fixture.environment)
	missingRelationRuntime := rebuildExactRuntimeBasis(
		t,
		fixture.runtime,
		missingRelationEnvironment,
		fixture.runtime.Codecs(),
	)
	assertUnderdetermined(
		t,
		Adapt(fixture.draft, missingRelationRuntime, fixture.concern),
		"problem_card_typed_relation_declaration_fragment",
	)
}

func TestAdaptKeepsMissingClaimGraphUnderdetermined(t *testing.T) {
	fixture := newAdapterFixture(t)
	missing := mustMissingBasisTest(
		t,
		"claim_graph",
		"repair:provide-note-claim-graph",
	)
	missingGraph := mustValueTest(NewMissingClaimGraph([]MissingBasis{missing}))
	draft := draftWithClaimGraph(t, fixture.draft, missingGraph)
	assertUnderdetermined(
		t,
		Adapt(draft, fixture.runtime, fixture.concern),
		"claim_graph",
	)
}

func TestAdaptRejectsRuntimeFromAnotherProject(t *testing.T) {
	fixture := newAdapterFixture(t)
	otherProject := mustValueTest(projectidentity.ParseProjectID("qnt_b7f3b2c1"))
	runtime := mustValueTest(
		NewExactRuntimeBasisBuilder(otherProject).
			SetGraphRevision(fixture.runtime.GraphRevision()).
			SetEnvironment(fixture.runtime.Environment()).
			SetCodecs(fixture.runtime.Codecs()).
			SetSelectedRuntimeCoordinates(
				fixture.runtime.SelectedRuntimeBasis(),
				fixture.runtime.RegistryCoordinate(),
			).
			SetRegistrationPolicy(fixture.runtime.RegistrationPolicy()).
			Build(),
	)

	assertInvalid(
		t,
		Adapt(fixture.draft, runtime, fixture.concern),
		"runtime_project_mismatch",
	)
}

func TestAdaptKeepsUnacceptedMappingUnderdetermined(t *testing.T) {
	fixture := newAdapterFixture(t)
	manifest := mustValueTest(CurrentMappingManifestV1())
	wrongAdapter := mustValueTest(recordmapping.NewAdapterVersion("9.9.9"))
	policy := testRegistrationPolicy(t, manifest.Ref(), wrongAdapter)
	runtime := mustValueTest(
		NewExactRuntimeBasisBuilder(fixture.runtime.ProjectID()).
			SetGraphRevision(fixture.runtime.GraphRevision()).
			SetEnvironment(fixture.runtime.Environment()).
			SetCodecs(fixture.runtime.Codecs()).
			SetSelectedRuntimeCoordinates(
				fixture.runtime.SelectedRuntimeBasis(),
				fixture.runtime.RegistryCoordinate(),
			).
			SetRegistrationPolicy(policy).
			Build(),
	)

	assertUnderdetermined(
		t,
		Adapt(fixture.draft, runtime, fixture.concern),
		"problem_card_mapping_registration",
	)
}

func TestAdaptRejectsConcernContextAndKindMismatch(t *testing.T) {
	fixture := newAdapterFixture(t)
	otherContext, err := typedmemory.NewBoundedContextRef("context:other")
	if err != nil {
		t.Fatalf("NewBoundedContextRef(other) error = %v", err)
	}
	otherContextConcern := exactConcernFor(
		t,
		fixture.entityRefKind,
		fixture.concern.Entity(),
		otherContext,
	)
	assertInvalid(t, Adapt(fixture.draft, fixture.runtime, otherContextConcern), "concern_context_mismatch")

	wrongKindConcern := exactConcernFor(
		t,
		fixture.projectRecordRefKind,
		fixture.concern.Entity(),
		fixture.context,
	)
	assertInvalid(t, Adapt(fixture.draft, fixture.runtime, wrongKindConcern), "concern_reference_kind_mismatch")

}

func assertExactNoteBindings(
	t *testing.T,
	relation typedmemory.RelationalAssertionCandidate,
	fixture adapterFixture,
) {
	t.Helper()
	bindings := relation.Bindings()
	if len(bindings) != 3 {
		t.Fatalf("ProblemCardAtConcern bindings = %d, want 3", len(bindings))
	}
	byName := make(map[string]typedmemory.CandidateSlotBinding, len(bindings))
	for _, binding := range bindings {
		byName[binding.Name().String()] = binding
	}
	note := byName[problemCardSlotID].Fillers()[0].(typedmemory.ByReferenceCandidate)
	local, ok := note.Reference().(typedmemory.LocalRef)
	if !ok || local.BatchLocalRef() != fixture.draft.RecordLocalRef() {
		t.Fatal("ProblemCardSlot did not point to the same-batch project record")
	}
	concern := byName[entityOfConcernSlotID].Fillers()[0].(typedmemory.ByReferenceCandidate)
	persisted, ok := concern.Reference().(typedmemory.PersistedRef)
	if !ok || persisted != fixture.concern.Reference() {
		t.Fatal("EntityOfConcernSlot did not preserve the pre-resolved concern reference")
	}
	claim := byName[claimGraphSlotID].Fillers()[0].(typedmemory.ByValueCandidate)
	if claim.Value().ValueKind().ID().String() != claimGraphKindID {
		t.Fatal("ClaimGraphSlot did not carry the exact U.ClaimGraph ValueKind")
	}
	if bytes.HasPrefix(claim.Value().InputBytes(), []byte("{")) {
		t.Fatal("ClaimGraphSlot fell back to arbitrary JSON")
	}
}

type adapterFixture struct {
	environment          typedmemory.TypeEnv
	runtime              ExactRuntimeBasis
	draft                Draft
	concern              ExactConcernBinding
	context              typedmemory.BoundedContextRef
	projectRecordRefKind typedmemory.RefKindRef
	entityRefKind        typedmemory.RefKindRef
	textKind             typedmemory.ValueKindRef
}

func newAdapterFixture(t *testing.T) adapterFixture {
	t.Helper()
	ref := mustTypeEnvRefTest(t, '1')
	context := mustValueTest(typedmemory.NewBoundedContextRef("context:haft-v9"))
	provenance := testDeclarationProvenance(t)
	projectRecordKind := mustKindDefinitionTest(t, projectRecordKindID, provenance)
	entityKind := mustKindDefinitionTest(t, entityKindID, provenance)
	claimGraphKind := mustKindDefinitionTest(t, claimGraphKindID, provenance)
	textKind := mustKindDefinitionTest(t, "U.Text", provenance)
	projectRecordValueKind := mustValueKindRefTest(t, ref, projectRecordKind.ID())
	entityValueKind := mustValueKindRefTest(t, ref, entityKind.ID())
	claimGraphValueKind := mustValueKindRefTest(t, ref, claimGraphKind.ID())
	textValueKind := mustValueKindRefTest(t, ref, textKind.ID())
	projectRecordRefKind := mustRefKindRefTest(t, ref, projectRecordRefID)
	entityRefKind := mustRefKindRefTest(t, ref, entityRefID)
	projectRecordRefDefinition := mustValueTest(
		typedmemory.NewRefKindDefinition(projectRecordRefKind, projectRecordValueKind, provenance),
	)
	entityRefDefinition := mustValueTest(
		typedmemory.NewRefKindDefinition(entityRefKind, entityValueKind, provenance),
	)
	shape := typedmemory.NewClaimGraphShape()
	shapeID := mustValueTest(typedmemory.NewShapeID("Haft.Shape.ClaimGraphV1"))
	shapeRef := mustValueTest(typedmemory.DeriveValueShapeRef(shapeID, shape))
	shapeDeclaration := mustValueTest(
		typedmemory.NewValueShapeDeclaration(shapeRef, shape, provenance),
	)
	codecRef := testCodecRef(t)
	valueBinding := mustValueTest(
		typedmemory.NewValueBinding(claimGraphValueKind, shapeRef, codecRef, provenance),
	)
	relation := testProblemCardRelationSignature(
		t,
		ref,
		context,
		projectRecordValueKind,
		projectRecordRefKind,
		entityValueKind,
		entityRefKind,
		claimGraphValueKind,
		provenance,
	)
	contextDeclaration := mustValueTest(typedmemory.NewBoundedContext(context, provenance))
	revision := mustValueTest(typedmemory.NewSourceRevision("fixture-revision-1"))
	compiler := mustValueTest(typedmemory.NewCompilerSchemaVersion("fixture.compiler/v1"))
	coverage := testCoverageManifest(t)
	environment, err := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(revision).
		SetCompilerSchemaVersion(compiler).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextDeclaration).
		AddKindDefinition(projectRecordKind).
		AddKindDefinition(entityKind).
		AddKindDefinition(claimGraphKind).
		AddKindDefinition(textKind).
		AddRefKindDefinition(projectRecordRefDefinition).
		AddRefKindDefinition(entityRefDefinition).
		AddRelationSignature(relation).
		AddValueShape(shapeDeclaration).
		AddValueBinding(valueBinding).
		Build()
	if err != nil {
		t.Fatalf("build ProblemCard adapter TypeEnv fixture: %v", err)
	}
	codec := mustValueTest(typedmemory.NewClaimGraphCodecV1(shapeRef))
	registry, err := typedmemory.NewCodecRegistry().Register(codecRef, codec)
	if err != nil {
		t.Fatalf("register ClaimGraph codec: %v", err)
	}
	project := mustValueTest(projectidentity.ParseProjectID("qnt_a7f3b2c1"))
	manifest := mustValueTest(CurrentMappingManifestV1())
	registration := testRegistrationPolicy(t, manifest.Ref(), manifest.AdapterVersion())
	runtimeBasis := mustValueTest(typedmemorystore.NewSelectedRuntimeBasisDigest(
		mustDigestTest(t, '8'),
	))
	registryCoordinate := mustValueTest(
		typedmemorystore.NewExactTargetRegistryCoordinateDigest(
			mustDigestTest(t, '9'),
		),
	)
	runtime := mustValueTest(
		NewExactRuntimeBasisBuilder(project).
			SetGraphRevision(typedmemory.NewGraphRevision(17)).
			SetEnvironment(environment).
			SetCodecs(registry).
			SetSelectedRuntimeCoordinates(runtimeBasis, registryCoordinate).
			SetRegistrationPolicy(registration).
			Build(),
	)
	contextSlice := testContextSlice(t, context)
	recordEntity := mustValueTest(typedmemory.NewEntityID("record:problem-card-1"))
	draft := mustValueTest(NewDraft(DraftInput{
		ProjectID:      project,
		RecordEntity:   recordEntity,
		RecordLocalRef: mustValueTest(typedmemory.NewBatchLocalRef("record:problem-card-1")),
		RecordLabel:    mustValueTest(typedmemory.NewEntityLabel("Problem: stale CAS description")),
		AssertionID:    mustValueTest(typedmemory.NewAssertionID("assertion:problem-card-1-at-concern")),
		ContextSlice:   contextSlice,
		ClaimGraph: mustValueTest(
			NewExactClaimGraph(testClaimGraph(t, textValueKind, false)),
		),
		Provenance: mustValueTest(typedmemory.NewProvenanceRef("memory:problem-card-adapter-test")),
	}))
	concernEntity := mustValueTest(typedmemory.NewEntityID("entity:authorization-service"))
	concern := exactConcernFor(t, entityRefKind, concernEntity, context)
	return adapterFixture{
		environment:          environment,
		runtime:              runtime,
		draft:                draft,
		concern:              concern,
		context:              context,
		projectRecordRefKind: projectRecordRefKind,
		entityRefKind:        entityRefKind,
		textKind:             textValueKind,
	}
}

func rebuildExactRuntimeBasis(
	t *testing.T,
	source ExactRuntimeBasis,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
) ExactRuntimeBasis {
	t.Helper()
	return mustValueTest(
		NewExactRuntimeBasisBuilder(source.ProjectID()).
			SetGraphRevision(source.GraphRevision()).
			SetEnvironment(environment).
			SetCodecs(codecs).
			SetSelectedRuntimeCoordinates(
				source.SelectedRuntimeBasis(),
				source.RegistryCoordinate(),
			).
			SetRegistrationPolicy(source.RegistrationPolicy()).
			Build(),
	)
}

func draftWithClaimGraph(
	t *testing.T,
	source Draft,
	graph ClaimGraphBasis,
) Draft {
	t.Helper()
	return mustValueTest(NewDraft(DraftInput{
		ProjectID:      source.ProjectID(),
		RecordEntity:   source.RecordEntity(),
		RecordLocalRef: source.RecordLocalRef(),
		RecordLabel:    source.RecordLabel(),
		AssertionID:    source.AssertionID(),
		ContextSlice:   source.ContextSlice(),
		ClaimGraph:     graph,
		Provenance:     source.Provenance(),
	}))
}

func testRegistrationPolicy(
	t *testing.T,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	rule := mustValueTest(typedmemory.NewRuleRef(
		"rule:haft-project-record-membership/v1",
	))
	artifact := mustValueTest(typedmemory.NewCarrierRef(
		"artifact:problem-card-adapter-membership-runtime/v1",
	))
	edition := mustValueTest(typedmemory.NewCarrierEdition("build-20260717.1"))
	digest := mustDigestTest(t, '7')
	evaluator := mustValueTest(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   digest,
		},
	))
	delivery := mustValueTest(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   digest,
		},
	))
	mapping := mustValueTest(recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	))
	return mustValueTest(recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
		},
	))
}

func testProblemCardRelationSignature(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	projectRecordKind typedmemory.ValueKindRef,
	projectRecordRef typedmemory.RefKindRef,
	entityKind typedmemory.ValueKindRef,
	entityRef typedmemory.RefKindRef,
	claimGraphKind typedmemory.ValueKindRef,
	provenance typedmemory.DeclarationProvenance,
) typedmemory.RelationSignature {
	t.Helper()
	signatureID := mustValueTest(typedmemory.NewSignatureID(problemCardSignatureID))
	signatureRef := mustValueTest(typedmemory.NewRelationSignatureRef(typeEnv, signatureID))
	noteTarget := mustValueTest(typedmemory.NewReferenceSlotTarget(projectRecordKind, projectRecordRef))
	concernTarget := mustValueTest(typedmemory.NewReferenceSlotTarget(entityKind, entityRef))
	claimTarget := mustValueTest(typedmemory.NewValueSlotTarget(claimGraphKind))
	slots := []typedmemory.SlotSpec{
		mustValueTest(typedmemory.NewSlotSpec(
			mustValueTest(typedmemory.NewSlotKindID(problemCardSlotID)),
			noteTarget,
			typedmemory.ExactlyOneCardinality(),
			provenance,
		)),
		mustValueTest(typedmemory.NewSlotSpec(
			mustValueTest(typedmemory.NewSlotKindID(entityOfConcernSlotID)),
			concernTarget,
			typedmemory.ExactlyOneCardinality(),
			provenance,
		)),
		mustValueTest(typedmemory.NewSlotSpec(
			mustValueTest(typedmemory.NewSlotKindID(claimGraphSlotID)),
			claimTarget,
			typedmemory.ExactlyOneCardinality(),
			provenance,
		)),
	}
	return mustValueTest(
		typedmemory.NewRelationSignature(
			signatureRef,
			[]typedmemory.BoundedContextRef{context},
			slots,
			provenance,
		),
	)
}

func environmentWithoutProblemCardRelation(
	t *testing.T,
	source typedmemory.TypeEnv,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(source.Ref()).
		SetSourceRevision(source.SourceRevision()).
		SetCompilerSchemaVersion(source.CompilerSchemaVersion()).
		SetCoverageManifest(source.CoverageManifest())
	for _, context := range source.BoundedContexts() {
		builder.AddBoundedContext(context)
	}
	for _, kind := range source.KindDefinitions() {
		builder.AddKindDefinition(kind)
	}
	for _, refKind := range source.RefKindDefinitions() {
		builder.AddRefKindDefinition(refKind)
	}
	for _, shape := range source.ValueShapes() {
		builder.AddValueShape(shape)
	}
	for _, binding := range source.ValueBindings() {
		builder.AddValueBinding(binding)
	}
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("build mapping-drift TypeEnv: %v", err)
	}
	return environment
}

func testClaimGraph(
	t *testing.T,
	textKind typedmemory.ValueKindRef,
	reversed bool,
) typedmemory.ClaimGraphValue {
	t.Helper()
	first := mustValueTest(typedmemory.NewClaimNode(
		mustValueTest(typedmemory.NewClaimNodeID("claim:cas-current")),
		textKind,
		typedmemory.NewTextValue("CAS protects concurrent record updates"),
	))
	second := mustValueTest(typedmemory.NewClaimNode(
		mustValueTest(typedmemory.NewClaimNodeID("claim:cleanup-current")),
		textKind,
		typedmemory.NewTextValue("orphan cleanup is independently retryable"),
	))
	nodes := []typedmemory.ClaimNode{first, second}
	if reversed {
		nodes = []typedmemory.ClaimNode{second, first}
	}
	return mustValueTest(typedmemory.NewClaimGraphValue(nodes, nil))
}

func reversedClaimGraph(t *testing.T, fixture adapterFixture) typedmemory.ClaimGraphValue {
	t.Helper()
	return testClaimGraph(t, fixture.textKind, true)
}

func exactConcernFor(
	t *testing.T,
	refKind typedmemory.RefKindRef,
	entity typedmemory.EntityID,
	context typedmemory.BoundedContextRef,
) ExactConcernBinding {
	t.Helper()
	referenceID := mustValueTest(typedmemory.NewReferenceID(entity.String()))
	reference := mustValueTest(typedmemory.NewPersistedRef(refKind, referenceID))
	basis := mustValueTest(typedmemory.NewResolutionBasisRef("snapshot:fixture"))
	resolution := mustValueTest(
		typedmemory.NewResolvedStrongReference(reference, entity, context, basis),
	)
	return mustValueTest(NewExactConcernBinding(resolution))
}

func testContextSlice(
	t *testing.T,
	context typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	point := mustValueTest(typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC),
	))
	return mustValueTest(typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   context,
		GammaTime: point,
	}))
}

func testDeclarationProvenance(t *testing.T) typedmemory.FPFSourceProvenance {
	t.Helper()
	location := testSourceLocation(t)
	reference := mustValueTest(typedmemory.NewProvenanceRef("provenance:problem-card-adapter-fixture"))
	rule := mustValueTest(typedmemory.NewCompilerRuleID("fixture.problem-card-adapter.v1"))
	return mustValueTest(typedmemory.NewFPFSourceProvenance(reference, location, rule))
}

func testSourceLocation(t *testing.T) typedmemory.SourceLocation {
	t.Helper()
	unit := mustValueTest(typedmemory.NewSourceUnitID("source:problem-card-adapter-fixture"))
	revision := mustValueTest(typedmemory.NewSourceRevision("fixture-revision-1"))
	digest := mustDigestTest(t, 0x31)
	lines := mustValueTest(typedmemory.NewSourceLineRange(1, 1))
	return mustValueTest(typedmemory.NewUnpatternedSourceLocation(unit, revision, digest, lines))
}

func testCoverageManifest(t *testing.T) typedmemory.CoverageManifest {
	t.Helper()
	location := testSourceLocation(t)
	subject := mustValueTest(typedmemory.SourceUnitCoverage(location.UnitID()))
	entry := mustValueTest(typedmemory.NewCompiledCoverageEntry(subject, location))
	return mustValueTest(typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry}))
}

func testCodecRef(t *testing.T) typedmemory.CodecRef {
	t.Helper()
	id := mustValueTest(typedmemory.NewCodecID("Haft.Codec.ClaimGraphV1"))
	version := mustValueTest(typedmemory.NewCanonicalizationVersion("v1"))
	return mustValueTest(typedmemory.NewCodecRef(id, version, mustDigestTest(t, '4')))
}

func mustKindDefinitionTest(
	t *testing.T,
	raw string,
	provenance typedmemory.DeclarationProvenance,
) typedmemory.KindDefinition {
	t.Helper()
	id := mustValueTest(typedmemory.NewKindID(raw))
	return mustValueTest(typedmemory.NewKindDefinition(id, provenance))
}

func mustValueKindRefTest(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	kind typedmemory.KindID,
) typedmemory.ValueKindRef {
	t.Helper()
	return mustValueTest(typedmemory.NewValueKindRef(typeEnv, kind))
}

func mustRefKindRefTest(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	raw string,
) typedmemory.RefKindRef {
	t.Helper()
	id := mustValueTest(typedmemory.NewRefKindID(raw))
	return mustValueTest(typedmemory.NewRefKindRef(typeEnv, id))
}

func mustTypeEnvRefTest(t *testing.T, fill byte) typedmemory.TypeEnvRef {
	t.Helper()
	return mustValueTest(typedmemory.NewTypeEnvRef(mustDigestTest(t, fill)))
}

func mustDigestTest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	return mustValueTest(typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(string([]byte{fill}), 64),
	))
}

func mustMissingBasisTest(
	t *testing.T,
	name string,
	repair string,
) MissingBasis {
	t.Helper()
	pointer := mustValueTest(typedmemory.NewRepairPointer(repair))
	return mustValueTest(NewMissingBasis(name, pointer))
}

func validCandidate(t *testing.T, result Result) ValidCandidate {
	t.Helper()
	candidate, ok := result.(ValidCandidate)
	if !ok {
		t.Fatalf("adapter result = %T, want ValidCandidate", result)
	}
	return candidate
}

func assertUnderdetermined(t *testing.T, result Result, want string) {
	t.Helper()
	value, ok := result.(Underdetermined)
	if !ok {
		t.Fatalf("adapter result = %T, want Underdetermined", result)
	}
	missing := value.MissingBasis()
	if len(missing) != 1 || missing[0].Name() != want {
		t.Fatalf("missing basis = %#v, want %q", missing, want)
	}
}

func assertInvalid(t *testing.T, result Result, want string) {
	t.Helper()
	value, ok := result.(Invalid)
	if !ok {
		t.Fatalf("adapter result = %T, want Invalid", result)
	}
	violations := value.Violations()
	if len(violations) != 1 || violations[0].Code() != want {
		t.Fatalf("violations = %#v, want %q", violations, want)
	}
}

func mustValueTest[T any](value T, err error) T {
	if err != nil {
		panic("ProblemCard adapter fixture construction failed: " + err.Error())
	}
	return value
}
