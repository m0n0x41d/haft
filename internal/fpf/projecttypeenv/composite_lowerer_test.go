package projecttypeenv

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	compositeLowererEnumerationRule = "haft.rule.project-entities/v1"
	compositeLowererDefinednessRule = "haft.rule.project-concern-defined/v1"
	compositeLowererEvaluatorRule   = "haft.rule.project-concern-member/v1"
	compositeLowererMembershipRule  = "haft.member-of.project-record-carrier/v1"
)

var compositeLowererCodecContract = []string{
	"Equal conceptual values produce equal canonical bytes.",
	"Decode then encode preserves canonical bytes.",
}

type compositeLowererFixture struct {
	base            typeenv.BaseTypeEnvArtifact
	extension       ProjectTypeEnvExtensionArtifact
	linked          LinkedProjectTypeEnvCompositeIR
	runtime         RuntimeEvaluationBasisArtifact
	composite       ProjectTypeEnvCompositeArtifact
	environment     typedmemory.TypeEnv
	verification    ProjectTypeEnvCompositeVerification
	baseAtComposite typedmemory.TypeEnv
	codec           CodecSpecificationV1
}

func TestPrepareProjectTypeEnvCompositeLowersEverySourceFamilyAtC(t *testing.T) {
	fixture := newCompositeLowererFixture(t)
	environment := fixture.environment
	base := fixture.baseAtComposite
	availabilityPlan := acceptedContextKindAvailabilityPlanForTest(
		t,
		DeriveContextKindAvailabilityPlan(fixture.linked),
	)

	assertCompositeLowererCount(t, "bounded contexts", len(environment.BoundedContexts()), len(base.BoundedContexts())+1)
	assertCompositeLowererCount(t, "kind definitions", len(environment.KindDefinitions()), len(base.KindDefinitions())+1)
	assertCompositeLowererCount(t, "entity sets", len(environment.EntitySetDefinitions()), len(base.EntitySetDefinitions())+1)
	assertCompositeLowererCount(t, "kind signatures", len(environment.KindSignatureDefinitions()), len(base.KindSignatureDefinitions())+1)
	assertCompositeLowererCount(t, "ref kinds", len(environment.RefKindDefinitions()), len(base.RefKindDefinitions())+1)
	assertCompositeLowererCount(t, "subkind relations", len(environment.SubkindRelations()), len(base.SubkindRelations())+1)
	assertCompositeLowererCount(t, "context bridges", len(environment.ContextBridges()), len(base.ContextBridges()))
	assertCompositeLowererCount(
		t,
		"typed relation declaration fragments",
		len(environment.TypedRelationDeclarationFragments()),
		len(base.TypedRelationDeclarationFragments())+1,
	)
	assertCompositeLowererCount(t, "value shapes", len(environment.ValueShapes()), len(base.ValueShapes())+1)
	assertCompositeLowererCount(t, "value bindings", len(environment.ValueBindings()), len(base.ValueBindings())+1)
	assertCompositeLowererCount(t, "constraints", len(environment.Constraints()), len(base.Constraints())+3)
	assertCompositeLowererCount(
		t,
		"context-kind availability",
		len(environment.ContextKindAvailabilities()),
		len(base.ContextKindAvailabilities())+len(availabilityPlan.Inputs()),
	)

	assertCompositeLowererAllScopedRefsAtC(t, environment, fixture.composite.Ref())
	assertCompositeLowererBaseAvailabilityPreserved(t, base, environment)
	projectObjects := compositeLowererProjectObjects(t, fixture)
	for label, provenance := range projectObjects.provenance {
		assertCompositeLowererProjectProvenance(
			t,
			label,
			provenance,
			fixture.linked.BaseTypeEnvRef(),
			"lowerer.signature",
			"haft-project",
		)
	}

	if !environment.IsSubkind(projectObjects.kind.ID(), mustAvailabilityKindID(t, "U.Entity")) {
		t.Fatal("lowered Haft.ProjectConcern is not a subkind of U.Entity")
	}
	sourceIR := fixture.extension.IR()
	sourceSubkind := declarationByKind(t, &sourceIR, localpractice.DeclarationSubkind)
	subkindProvenance := mustCompositeLowererProjectProvenance(
		t,
		"subkind",
		projectObjects.subkind.Provenance(),
	)
	if subkindProvenance.LineRange().Start() != sourceSubkind.Span().Start() ||
		subkindProvenance.LineRange().End() != sourceSubkind.Span().End() {
		t.Fatalf(
			"subkind source range = %d..%d, want exact declaration span %d..%d",
			subkindProvenance.LineRange().Start(),
			subkindProvenance.LineRange().End(),
			sourceSubkind.Span().Start(),
			sourceSubkind.Span().End(),
		)
	}

	if projectObjects.binding.Codec() != fixture.codec.Ref() {
		t.Fatalf(
			"lowered codec = %q, want exact source-derived %q",
			projectObjects.binding.Codec().String(),
			fixture.codec.Ref().String(),
		)
	}
	if projectObjects.binding.ValueShape() != fixture.codec.ValueShape() {
		t.Fatalf("lowered binding does not retain the exact source-derived shape")
	}
	assertCompositeLowererSlotCardinality(t, projectObjects.relation, "Haft.ConcernSlot", 1, 1)
	assertCompositeLowererSlotCardinality(t, projectObjects.relation, "Haft.EvidenceSlot", 0, 1)

	closure := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		fixture.composite,
		environment,
		fixture.linked,
		fixture.runtime,
	)
	if closure.Rejected() {
		t.Fatalf("prepared TypeEnv runtime closure rejected: %#v", closure.Issues())
	}
	if len(closure.RequiredSet().Requirements()) != len(fixture.runtime.Pins()) {
		t.Fatalf(
			"runtime closure/pin counts = %d/%d",
			len(closure.RequiredSet().Requirements()),
			len(fixture.runtime.Pins()),
		)
	}
	if err := fixture.verification.Verify(); err != nil {
		t.Fatalf("prepared verification witness is invalid: %v", err)
	}
	if fixture.verification.Digest() != fixture.verification.Ref().Digest() ||
		fixture.verification.BaseTypeEnvRef() != fixture.linked.BaseTypeEnvRef() ||
		fixture.verification.BaseArtifactDigest() != fixture.base.Digest() ||
		!projectTypeEnvExtensionRefsEqual(
			fixture.verification.ExtensionRefs(),
			projectTypeEnvCompositeExtensionRefs(fixture.linked.Extensions()),
		) ||
		fixture.verification.RuntimeEvaluationBasisRef() != fixture.runtime.Ref() ||
		fixture.verification.CompositeRef() != fixture.composite.Ref() ||
		fixture.verification.LoweredEnvironmentRef() != environment.Ref() ||
		fixture.verification.LowererSchemaVersion() != ProjectTypeEnvCompositeLowererSchemaV2 {
		t.Fatalf("prepared verification witness does not bind exact B/E/X/C/output: %#v", fixture.verification)
	}
}

func TestPrepareProjectTypeEnvCompositeRejectsMismatchedBEXCWithoutPartialEnvironment(
	t *testing.T,
) {
	fixture := newCompositeLowererFixture(t)
	otherBase := compositeLowererDifferentValidBase(t, fixture.base)
	otherExtension := compositeContextSubkindArtifact(
		t,
		fixture.base,
		"other.signature",
		"Haft",
		"haft-project",
		"Haft.ProjectConcern",
		"U.Entity",
	)
	otherLinkResolution := LinkProjectTypeEnvCompositeIR(
		fixture.base,
		[]ProjectTypeEnvExtensionArtifact{otherExtension},
	)
	otherLinked := acceptedCompositeIR(t, otherLinkResolution)
	otherRuntime := compositeLowererDifferentRuntimeBasis(t, fixture)
	otherComposite, err := SealProjectTypeEnvComposite(fixture.linked, otherRuntime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(other X): %v", err)
	}

	driftedLinked := fixture.linked
	driftedLinked.canonical = append([]byte(nil), driftedLinked.canonical...)
	driftedLinked.canonical[0] ^= 0xff
	driftedRuntime := fixture.runtime
	driftedRuntime.canonical = append([]byte(nil), driftedRuntime.canonical...)
	driftedRuntime.canonical[0] ^= 0xff
	driftedComposite := fixture.composite
	driftedComposite.canonicalBytes = append([]byte(nil), driftedComposite.canonicalBytes...)
	driftedComposite.canonicalBytes[0] ^= 0xff

	tests := []struct {
		name  string
		input ProjectTypeEnvCompositePreparationInput
		code  ProjectTypeEnvCompositeLoweringIssueCode
	}{
		{
			name: "B byte mismatch",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         otherBase,
				Linked:       fixture.linked,
				RuntimeBasis: fixture.runtime,
				Composite:    fixture.composite,
			},
			code: CompositeLoweringIssueBaseMismatch,
		},
		{
			name: "E recipe mismatch",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       otherLinked,
				RuntimeBasis: fixture.runtime,
				Composite:    fixture.composite,
			},
			code: CompositeLoweringIssueCompositeMismatch,
		},
		{
			name: "X recipe mismatch",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       fixture.linked,
				RuntimeBasis: otherRuntime,
				Composite:    fixture.composite,
			},
			code: CompositeLoweringIssueCompositeMismatch,
		},
		{
			name: "C recipe mismatch",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       fixture.linked,
				RuntimeBasis: fixture.runtime,
				Composite:    otherComposite,
			},
			code: CompositeLoweringIssueCompositeMismatch,
		},
		{
			name: "forged linked proof",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       driftedLinked,
				RuntimeBasis: fixture.runtime,
				Composite:    fixture.composite,
			},
			code: CompositeLoweringIssueLinkedInvalid,
		},
		{
			name: "forged runtime basis",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       fixture.linked,
				RuntimeBasis: driftedRuntime,
				Composite:    fixture.composite,
			},
			code: CompositeLoweringIssueRuntimeInvalid,
		},
		{
			name: "forged composite",
			input: ProjectTypeEnvCompositePreparationInput{
				Base:         fixture.base,
				Linked:       fixture.linked,
				RuntimeBasis: fixture.runtime,
				Composite:    driftedComposite,
			},
			code: CompositeLoweringIssueCompositeInvalid,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preparation := PrepareProjectTypeEnvComposite(test.input)
			assertCompositeLowererRejection(t, preparation, test.code)
		})
	}
}

func TestPrepareProjectTypeEnvCompositeCanonicalizesExtensionCallerPermutation(t *testing.T) {
	base := loadBaseArtifact(t)
	alpha := compositeContextSubkindArtifact(
		t,
		base,
		"alpha.lowerer",
		"Alpha",
		"alpha-project",
		"Alpha.ProjectConcern",
		"U.Entity",
	)
	beta := compositeContextSubkindArtifact(
		t,
		base,
		"beta.lowerer",
		"Beta",
		"beta-project",
		"Beta.ProjectConcern",
		"U.Entity",
	)
	leftResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{beta, alpha},
	)
	leftLinked := acceptedCompositeIR(t, leftResolution)
	rightResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{alpha, beta},
	)
	rightLinked := acceptedCompositeIR(t, rightResolution)
	if !bytes.Equal(leftLinked.CanonicalBytes(), rightLinked.CanonicalBytes()) {
		t.Fatal("E caller permutation changed linked canonical bytes")
	}

	alphaCodec := compositeLowererCodecSpecification(t, "Alpha")
	betaCodec := compositeLowererCodecSpecification(t, "Beta")
	runtime := compositeLowererRuntimeBasis(t, base, []CodecSpecificationV1{betaCodec, alphaCodec})
	leftComposite, err := SealProjectTypeEnvComposite(leftLinked, runtime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(left): %v", err)
	}
	rightComposite, err := SealProjectTypeEnvComposite(rightLinked, runtime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(right): %v", err)
	}
	if leftComposite.Ref() != rightComposite.Ref() ||
		!bytes.Equal(leftComposite.CanonicalBytes(), rightComposite.CanonicalBytes()) {
		t.Fatal("E caller permutation changed composite C")
	}

	leftPreparation := PrepareProjectTypeEnvComposite(ProjectTypeEnvCompositePreparationInput{
		Base:         base,
		Linked:       leftLinked,
		RuntimeBasis: runtime,
		Composite:    leftComposite,
	})
	leftEnvironment, leftVerification := acceptedCompositeLowererResult(t, leftPreparation)
	rightPreparation := PrepareProjectTypeEnvComposite(ProjectTypeEnvCompositePreparationInput{
		Base:         base,
		Linked:       rightLinked,
		RuntimeBasis: runtime,
		Composite:    rightComposite,
	})
	rightEnvironment, rightVerification := acceptedCompositeLowererResult(t, rightPreparation)
	if !reflect.DeepEqual(leftEnvironment, rightEnvironment) {
		t.Fatal("E caller permutation changed the prepared TypeEnv")
	}
	if !bytes.Equal(leftVerification.CanonicalBytes(), rightVerification.CanonicalBytes()) {
		t.Fatal("caller permutation changed the final-lowering verification witness")
	}
	leftSnapshot := acceptedProjectTypeEnvExecutableSnapshot(t, leftPreparation)
	rightSnapshot := acceptedProjectTypeEnvExecutableSnapshot(t, rightPreparation)
	if !bytes.Equal(
		leftSnapshot.Record().CanonicalBytes(),
		rightSnapshot.Record().CanonicalBytes(),
	) {
		t.Fatal("E caller permutation changed the executable TypeEnv snapshot")
	}
}

func TestMergeCompositeContextKindAvailabilitiesDeduplicatesAndRejectsConflicts(
	t *testing.T,
) {
	fixture := newCompositeLowererFixture(t)
	derived, err := lowerCompositeContextKindAvailabilities(fixture.linked)
	if err != nil {
		t.Fatalf("lowerCompositeContextKindAvailabilities(): %v", err)
	}
	availability := compositeLowererAvailabilityFor(
		t,
		derived,
		"haft-project",
		"Haft.ProjectConcern",
	)
	if len(availability.Grounds()) < 2 {
		t.Fatalf("availability grounds = %d, want a strict subset fixture", len(availability.Grounds()))
	}

	deduplicated, err := mergeCompositeContextKindAvailabilities(
		[]typedmemory.ContextKindAvailability{availability},
		[]typedmemory.ContextKindAvailability{availability},
	)
	if err != nil {
		t.Fatalf("merge identical availability: %v", err)
	}
	if len(deduplicated) != 1 ||
		!bytes.Equal(deduplicated[0].CanonicalBytes(), availability.CanonicalBytes()) {
		t.Fatal("identical availability was not canonically deduplicated")
	}

	subsetGrounds, err := typedmemory.NewContextKindAvailabilityGroundSet(
		[]typedmemory.ContextKindAvailabilityGround{availability.Grounds()[0]},
	)
	if err != nil {
		t.Fatalf("NewContextKindAvailabilityGroundSet(subset): %v", err)
	}
	conflicting, err := typedmemory.NewContextKindAvailability(
		availability.Context(),
		availability.KindID(),
		subsetGrounds,
	)
	if err != nil {
		t.Fatalf("NewContextKindAvailability(subset): %v", err)
	}
	merged, err := mergeCompositeContextKindAvailabilities(
		[]typedmemory.ContextKindAvailability{availability},
		[]typedmemory.ContextKindAvailability{conflicting},
	)
	if err == nil || !strings.Contains(err.Error(), "grounds conflict") {
		t.Fatalf("conflicting availability error = %v", err)
	}
	if merged != nil {
		t.Fatalf("conflicting availability returned partial union: %#v", merged)
	}
}

type compositeLowererProjectObjectSet struct {
	kind       typedmemory.KindDefinition
	subkind    typedmemory.SubkindRelation
	relation   typedmemory.TypedRelationDeclarationFragment
	binding    typedmemory.ValueBinding
	provenance map[string]typedmemory.DeclarationProvenance
}

func newCompositeLowererFixture(t *testing.T) compositeLowererFixture {
	t.Helper()
	base := loadBaseArtifact(t)
	extension := compositeContextSubkindArtifact(
		t,
		base,
		"lowerer.signature",
		"Haft",
		"haft-project",
		"Haft.ProjectConcern",
		"U.Entity",
	)
	linkResolution := LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{extension},
	)
	linked := acceptedCompositeIR(t, linkResolution)
	codec := compositeLowererCodecSpecification(t, "Haft")
	runtime := compositeLowererRuntimeBasis(t, base, []CodecSpecificationV1{codec})
	composite, err := SealProjectTypeEnvComposite(linked, runtime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	preparation := PrepareProjectTypeEnvComposite(ProjectTypeEnvCompositePreparationInput{
		Base:         base,
		Linked:       linked,
		RuntimeBasis: runtime,
		Composite:    composite,
	})
	environment, verification := acceptedCompositeLowererResult(t, preparation)
	baseAtComposite, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		composite.Ref(),
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(C): %v", err)
	}
	return compositeLowererFixture{
		base:            base,
		extension:       extension,
		linked:          linked,
		runtime:         runtime,
		composite:       composite,
		environment:     environment,
		verification:    verification,
		baseAtComposite: baseAtComposite,
		codec:           codec,
	}
}

func compositeLowererCodecSpecification(
	t *testing.T,
	prefix string,
) CodecSpecificationV1 {
	t.Helper()
	shape, err := typedmemory.NewScalarShape(typedmemory.ScalarText)
	if err != nil {
		t.Fatalf("NewScalarShape(text): %v", err)
	}
	shapeID, err := typedmemory.NewShapeID(prefix + ".Shape.Text")
	if err != nil {
		t.Fatalf("NewShapeID(): %v", err)
	}
	shapeRef, err := typedmemory.DeriveValueShapeRef(shapeID, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef(): %v", err)
	}
	codecID, err := typedmemory.NewCodecID(prefix + ".Codec.Text")
	if err != nil {
		t.Fatalf("NewCodecID(): %v", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion("v1")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(): %v", err)
	}
	specification, err := DeriveCodecSpecificationV1(
		codecID,
		version,
		shapeRef,
		compositeLowererCodecContract,
	)
	if err != nil {
		t.Fatalf("DeriveCodecSpecificationV1(): %v", err)
	}
	return specification
}

func compositeLowererRuntimeBasis(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	codecs []CodecSpecificationV1,
) RuntimeEvaluationBasisArtifact {
	t.Helper()
	return compositeLowererRuntimeBasisWithSourceCodecSeed(t, base, codecs, 0x81)
}

func compositeLowererRuntimeBasisWithSourceCodecSeed(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	codecs []CodecSpecificationV1,
	seed byte,
) RuntimeEvaluationBasisArtifact {
	t.Helper()
	pins := make([]RuntimeEvaluationMechanismPin, 0)
	for index, codec := range codecs {
		pin := runtimeCodecMechanismPinForRef(
			t,
			codec.Ref(),
			fmt.Sprintf("artifact:lowerer-source-codec-%d", index),
			"1.0.0",
			seed+byte(index),
		)
		pins = append(pins, pin)
	}
	baseEnvironment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(base)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecs(B): %v", err)
	}
	for index, binding := range baseEnvironment.ValueBindings() {
		pin := runtimeCodecMechanismPinForRef(
			t,
			binding.Codec(),
			fmt.Sprintf("artifact:lowerer-base-codec-%d", index),
			"1.0.0",
			byte(0x91+index),
		)
		pins = append(pins, pin)
	}
	pins = append(pins, runtimeEvaluatorMechanismPinWithContract(
		t,
		compositeLowererEnumerationRule,
		RuntimeMechanismContractEntitySetEnumeration,
		"artifact:lowerer-enumeration",
		"1.0.0",
		0xa1,
	))
	pins = append(pins, runtimeEvaluatorMechanismPinWithContract(
		t,
		compositeLowererDefinednessRule,
		RuntimeMechanismContractKindDefinedness,
		"artifact:lowerer-definedness",
		"1.0.0",
		0xa2,
	))
	pins = append(pins, runtimeEvaluatorMechanismPinWithContract(
		t,
		compositeLowererEvaluatorRule,
		RuntimeMechanismContractMemberOf,
		"artifact:lowerer-member-of",
		"1.0.0",
		0xa3,
	))
	pins = append(pins, runtimeCarrierMembershipMechanismPin(
		t,
		compositeLowererMembershipRule,
		"artifact:lowerer-carrier-membership",
		"1.0.0",
		0xa4,
	))
	basisPins := make([]RuntimeEvaluationBasisPin, 0, len(pins)+1)
	for _, pin := range pins {
		basisPins = append(basisPins, pin)
	}
	policy := registrationPolicyArtifactFixture(t, defaultRegistrationPolicySpec())
	policyPin, err := NewRegistrationPolicyPin(policy)
	if err != nil {
		t.Fatalf("NewRegistrationPolicyPin(): %v", err)
	}
	basisPins = append(basisPins, policyPin)
	basis, err := SealRuntimeEvaluationBasisWithPins(basisPins, nil, nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(): %v", err)
	}
	return basis
}

func compositeLowererDifferentRuntimeBasis(
	t *testing.T,
	fixture compositeLowererFixture,
) RuntimeEvaluationBasisArtifact {
	t.Helper()
	return compositeLowererRuntimeBasisWithSourceCodecSeed(
		t,
		fixture.base,
		[]CodecSpecificationV1{fixture.codec},
		0xb1,
	)
}

func compositeLowererDifferentValidBase(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	compiler, err := typedmemory.NewCompilerSchemaVersion(
		base.CompilerSchemaVersion().String() + "-lowerer-mismatch",
	)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion(other B): %v", err)
	}
	ir, err := typeenv.NewCompiledLinkedTypeEnvIR(
		base.SourceRevision(),
		compiler,
		base.CoverageManifest(),
		base.Declarations(),
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR(other B): %v", err)
	}
	other, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(other B): %v", err)
	}
	if bytes.Equal(other.CanonicalBytes(), base.CanonicalBytes()) {
		t.Fatal("other valid B did not change canonical bytes")
	}
	return other
}

func acceptedCompositeLowererResult(
	t *testing.T,
	preparation ProjectTypeEnvCompositePreparation,
) (typedmemory.TypeEnv, ProjectTypeEnvCompositeVerification) {
	t.Helper()
	if preparation.Rejected() {
		t.Fatalf("PrepareProjectTypeEnvComposite() rejected: %#v", preparation.Issues())
	}
	environment, exists := preparation.Environment()
	if !exists {
		t.Fatal("accepted composite preparation has no TypeEnv")
	}
	verification, exists := preparation.Verification()
	if !exists {
		t.Fatal("accepted composite preparation has no verification witness")
	}
	return environment, verification
}

func assertCompositeLowererRejection(
	t *testing.T,
	preparation ProjectTypeEnvCompositePreparation,
	code ProjectTypeEnvCompositeLoweringIssueCode,
) {
	t.Helper()
	if !preparation.Rejected() {
		t.Fatalf("PrepareProjectTypeEnvComposite() accepted, want %q", code)
	}
	for _, issue := range preparation.Issues() {
		if issue.Code() == code {
			if strings.TrimSpace(issue.Detail()) == "" || strings.TrimSpace(issue.Repair()) == "" {
				t.Fatalf("issue %q lacks detail or repair: %#v", code, issue)
			}
			if _, exists := preparation.Environment(); exists {
				t.Fatalf("rejection %q exposed a partial TypeEnv", code)
			}
			if _, exists := preparation.Verification(); exists {
				t.Fatalf("rejection %q exposed a verification witness", code)
			}
			return
		}
	}
	t.Fatalf("issues = %#v, want %q", preparation.Issues(), code)
}

func assertCompositeLowererCount(
	t *testing.T,
	label string,
	got int,
	want int,
) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %d, want %d", label, got, want)
	}
}

func compositeLowererProjectObjects(
	t *testing.T,
	fixture compositeLowererFixture,
) compositeLowererProjectObjectSet {
	t.Helper()
	environment := fixture.environment
	contextRef := mustAvailabilityContextRef(t, "haft-project")
	context, exists := environment.BoundedContext(contextRef)
	if !exists {
		t.Fatal("lowered bounded context is absent")
	}
	kindID := mustAvailabilityKindID(t, "Haft.ProjectConcern")
	kind, exists := environment.KindDefinition(kindID)
	if !exists {
		t.Fatal("lowered ValueKind is absent")
	}
	entitySet, exists := environment.EntitySetForContext(contextRef)
	if !exists {
		t.Fatal("lowered EntitySet is absent")
	}
	valueKind, err := typedmemory.NewValueKindRef(fixture.composite.Ref(), kindID)
	if err != nil {
		t.Fatalf("NewValueKindRef(): %v", err)
	}
	kindSignature, exists := environment.KindSignatureDefinition(valueKind, contextRef)
	if !exists {
		t.Fatal("lowered KindSignature is absent")
	}
	refKindID, err := typedmemory.NewRefKindID("Haft.ProjectConcernRef")
	if err != nil {
		t.Fatalf("NewRefKindID(): %v", err)
	}
	refKindRef, err := typedmemory.NewRefKindRef(fixture.composite.Ref(), refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef(): %v", err)
	}
	refKind, exists := environment.RefKindDefinition(refKindRef)
	if !exists {
		t.Fatal("lowered RefKind is absent")
	}
	subkind := compositeLowererSubkindFor(t, environment, "Haft.ProjectConcern", "U.Entity")
	relationID, err := typedmemory.NewSignatureID("Haft.ConcernMemory")
	if err != nil {
		t.Fatalf("NewSignatureID(): %v", err)
	}
	relationRef, err := typedmemory.NewTypedRelationDeclarationFragmentRef(
		fixture.composite.Ref(),
		relationID,
	)
	if err != nil {
		t.Fatalf("NewTypedRelationDeclarationFragmentRef(): %v", err)
	}
	relation, exists := environment.TypedRelationDeclarationFragment(relationRef)
	if !exists {
		t.Fatal("lowered typed relation declaration fragment is absent")
	}
	if relation.Posture() != typedmemory.RelationDeclarationTypedFragment {
		t.Fatalf("lowered relation declaration posture = %q", relation.Posture())
	}
	shape := compositeLowererShapeFor(t, environment, "Haft.Shape.Text")
	binding, exists := environment.ValueBinding(valueKind)
	if !exists {
		t.Fatal("lowered ValueBinding is absent")
	}
	constraints := compositeLowererConstraintsFor(t, environment, []string{
		"Haft.Constraint.RequiredConcern",
		"Haft.Constraint.OptionalEvidence",
		"Haft.Constraint.ConcernOrEvidence",
	})
	provenance := map[string]typedmemory.DeclarationProvenance{
		"bounded context": context.Provenance(),
		"value kind":      kind.Provenance(),
		"entity set":      entitySet.Provenance(),
		"kind signature":  kindSignature.Provenance(),
		"ref kind":        refKind.Provenance(),
		"subkind":         subkind.Provenance(),
		"relation":        relation.Provenance(),
		"value shape":     shape.Provenance(),
		"value binding":   binding.Provenance(),
	}
	for _, constraint := range constraints {
		provenance["constraint "+constraint.ID().String()] = constraint.Provenance()
	}
	return compositeLowererProjectObjectSet{
		kind:       kind,
		subkind:    subkind,
		relation:   relation,
		binding:    binding,
		provenance: provenance,
	}
}

func assertCompositeLowererProjectProvenance(
	t *testing.T,
	label string,
	provenance typedmemory.DeclarationProvenance,
	base typedmemory.TypeEnvRef,
	carrier string,
	context string,
) {
	t.Helper()
	project := mustCompositeLowererProjectProvenance(t, label, provenance)
	if project.Carrier().String() != carrier ||
		project.BaseTypeEnv() != base ||
		project.BoundedContext().String() != context ||
		project.ManifestBasis().Manifest().ID() != carrier ||
		project.LineRange().Start() == 0 ||
		project.LineRange().End() < project.LineRange().Start() ||
		project.CompilerRuleID().String() == "" {
		t.Fatalf("%s provenance = %#v", label, project)
	}
}

func mustCompositeLowererProjectProvenance(
	t *testing.T,
	label string,
	provenance typedmemory.DeclarationProvenance,
) typedmemory.ProjectSourceProvenance {
	t.Helper()
	project, ok := provenance.(typedmemory.ProjectSourceProvenance)
	if !ok {
		t.Fatalf("%s provenance type = %T, want ProjectSourceProvenance", label, provenance)
	}
	return project
}

func assertCompositeLowererAllScopedRefsAtC(
	t *testing.T,
	environment typedmemory.TypeEnv,
	composite typedmemory.TypeEnvRef,
) {
	t.Helper()
	for _, definition := range environment.EntitySetDefinitions() {
		if definition.Ref().TypeEnv() != composite {
			t.Fatalf("EntitySet ref %q is not C-bound", definition.Ref().String())
		}
	}
	for _, definition := range environment.KindSignatureDefinitions() {
		if definition.Ref().TypeEnv() != composite ||
			definition.ValueKind().TypeEnv() != composite ||
			definition.EntitySet().TypeEnv() != composite {
			t.Fatalf("KindSignature ref %q is not wholly C-bound", definition.Ref().String())
		}
	}
	for _, definition := range environment.RefKindDefinitions() {
		if definition.Ref().TypeEnv() != composite || definition.ValueKind().TypeEnv() != composite {
			t.Fatalf("RefKind ref %q is not wholly C-bound", definition.Ref().String())
		}
	}
	for _, fragment := range environment.TypedRelationDeclarationFragments() {
		if fragment.Ref().TypeEnv() != composite {
			t.Fatalf("relation-fragment ref %q is not C-bound", fragment.Ref().String())
		}
		if fragment.Posture() != typedmemory.RelationDeclarationTypedFragment {
			t.Fatalf("relation-fragment posture = %q", fragment.Posture())
		}
		for _, slot := range fragment.Slots() {
			switch target := slot.Target().(type) {
			case typedmemory.ValueSlotTarget:
				if target.ValueKind().TypeEnv() != composite {
					t.Fatalf("slot %q ValueKind is not C-bound", slot.SlotKind().String())
				}
			case typedmemory.ReferenceSlotTarget:
				if target.ValueKind().TypeEnv() != composite ||
					target.ReferenceKind().TypeEnv() != composite {
					t.Fatalf("slot %q reference target is not wholly C-bound", slot.SlotKind().String())
				}
			default:
				t.Fatalf("slot %q has unsupported target %T", slot.SlotKind().String(), slot.Target())
			}
		}
	}
	for _, binding := range environment.ValueBindings() {
		if binding.ValueKind().TypeEnv() != composite {
			t.Fatalf("ValueBinding %q is not C-bound", binding.ValueKind().String())
		}
	}
	for _, constraint := range environment.Constraints() {
		switch value := constraint.(type) {
		case typedmemory.SlotCardinalityConstraint:
			if value.Signature().TypeEnv() != composite {
				t.Fatalf("constraint %q is not C-bound", value.ID().String())
			}
		case typedmemory.SlotGroupConstraint:
			if value.Signature().TypeEnv() != composite {
				t.Fatalf("constraint %q is not C-bound", value.ID().String())
			}
		case typedmemory.ReferenceSlotSubsetConstraint:
			if value.Signature().TypeEnv() != composite {
				t.Fatalf("constraint %q is not C-bound", value.ID().String())
			}
		case typedmemory.ReferenceSlotPartitionConstraint:
			if value.Signature().TypeEnv() != composite {
				t.Fatalf("constraint %q is not C-bound", value.ID().String())
			}
		case typedmemory.KindDisjointConstraint:
		default:
			t.Fatalf("constraint %q has unsupported variant %T", constraint.ID().String(), constraint)
		}
	}
}

func assertCompositeLowererBaseAvailabilityPreserved(
	t *testing.T,
	base typedmemory.TypeEnv,
	prepared typedmemory.TypeEnv,
) {
	t.Helper()
	for _, availability := range base.ContextKindAvailabilities() {
		retained, exists := prepared.ContextKindAvailability(
			availability.Context(),
			availability.KindID(),
		)
		if !exists || !bytes.Equal(retained.CanonicalBytes(), availability.CanonicalBytes()) {
			t.Fatalf(
				"base availability %s/%s was not preserved exactly",
				availability.Context().String(),
				availability.KindID().String(),
			)
		}
	}
}

func assertCompositeLowererSlotCardinality(
	t *testing.T,
	relation typedmemory.TypedRelationDeclarationFragment,
	slotRaw string,
	minimum uint64,
	maximum uint64,
) {
	t.Helper()
	slotID, err := typedmemory.NewSlotKindID(slotRaw)
	if err != nil {
		t.Fatalf("NewSlotKindID(%q): %v", slotRaw, err)
	}
	slot, exists := relation.Slot(slotID)
	if !exists {
		t.Fatalf("relation has no slot %q", slotRaw)
	}
	actualMaximum, bounded := slot.Cardinality().Maximum().BoundedValue()
	if slot.Cardinality().Minimum() != minimum || !bounded || actualMaximum != maximum {
		t.Fatalf(
			"slot %q cardinality = %d..%d bounded=%t, want %d..%d",
			slotRaw,
			slot.Cardinality().Minimum(),
			actualMaximum,
			bounded,
			minimum,
			maximum,
		)
	}
}

func compositeLowererSubkindFor(
	t *testing.T,
	environment typedmemory.TypeEnv,
	child string,
	superkind string,
) typedmemory.SubkindRelation {
	t.Helper()
	for _, relation := range environment.SubkindRelations() {
		if relation.Subkind().String() == child && relation.Superkind().String() == superkind {
			return relation
		}
	}
	t.Fatalf("subkind relation %s < %s is absent", child, superkind)
	return typedmemory.SubkindRelation{}
}

func compositeLowererShapeFor(
	t *testing.T,
	environment typedmemory.TypeEnv,
	id string,
) typedmemory.ValueShapeDeclaration {
	t.Helper()
	for _, shape := range environment.ValueShapes() {
		if shape.Ref().ID().String() == id {
			return shape
		}
	}
	t.Fatalf("value shape %q is absent", id)
	return typedmemory.ValueShapeDeclaration{}
}

func compositeLowererConstraintsFor(
	t *testing.T,
	environment typedmemory.TypeEnv,
	ids []string,
) []typedmemory.ConstraintRule {
	t.Helper()
	byID := make(map[string]typedmemory.ConstraintRule)
	for _, constraint := range environment.Constraints() {
		byID[constraint.ID().String()] = constraint
	}
	result := make([]typedmemory.ConstraintRule, 0, len(ids))
	for _, id := range ids {
		constraint, exists := byID[id]
		if !exists {
			t.Fatalf("constraint %q is absent", id)
		}
		result = append(result, constraint)
	}
	return result
}

func compositeLowererAvailabilityFor(
	t *testing.T,
	values []typedmemory.ContextKindAvailability,
	context string,
	kind string,
) typedmemory.ContextKindAvailability {
	t.Helper()
	for _, availability := range values {
		if availability.Context().String() == context && availability.KindID().String() == kind {
			return availability
		}
	}
	t.Fatalf("availability %s/%s is absent", context, kind)
	return typedmemory.ContextKindAvailability{}
}
