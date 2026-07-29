package recordcarrier

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var _ RecordMembershipSourceDeliveryV1 = (*trustedRecordMembershipSourceDeliveryV1)(nil)
var _ RecordMembershipSourceDeliveryV1 = (*untrustedRecordMembershipSourceDeliveryV1)(nil)
var _ RecordMembershipSourceDeliveryV1 = (*missingRecordMembershipSourceDeliveryV1)(nil)

func TestRecordMembershipEvaluatorGenericProjectRecordForEveryClosedVariant(t *testing.T) {
	t.Parallel()

	variants := recordMembershipTestVariants()
	for _, variant := range variants {
		variant := variant
		t.Run(variant.Token(), func(t *testing.T) {
			t.Parallel()

			fixture := newRecordMembershipEvaluatorFixture(
				t,
				variant,
				projectRecordKindV1,
			)
			judgement := fixture.evaluate(t)
			assertRecordMembershipJudgementKind(
				t,
				judgement,
				typedmemory.MemberJudgement,
			)
			assertRecordMembershipDefinedBasis(
				t,
				judgement,
				fixture.source.source.ObservableInput(),
			)
		})
	}
}

func TestRecordMembershipEvaluatorExactClosedSpecializations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		kind    string
		variant ProjectRecordCarrierVariantV1
	}{
		{kind: decisionRecordKindV1, variant: DecisionRecordVariantV1{}},
		{kind: specSectionRecordKindV1, variant: SpecSectionRecordVariantV1{}},
		{kind: evidenceRecordKindV1, variant: EvidenceRecordVariantV1{}},
		{kind: supportingEpistemeRecordKindV1, variant: SupportingEpistemeRecordVariantV1{}},
		{kind: workRecordKindV1, variant: WorkRecordVariantV1{}},
		{kind: workPlanRecordKindV1, variant: WorkPlanRecordVariantV1{}},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.variant.Token(), func(t *testing.T) {
			t.Parallel()

			fixture := newRecordMembershipEvaluatorFixture(
				t,
				testCase.variant,
				testCase.kind,
			)
			judgement := fixture.evaluate(t)
			assertRecordMembershipJudgementKind(
				t,
				judgement,
				typedmemory.MemberJudgement,
			)
			assertRecordMembershipDefinedBasis(
				t,
				judgement,
				fixture.source.source.ObservableInput(),
			)
		})
	}
}

func TestRecordMembershipObservableSupportsOnlyGenericAndCompatibleSpecialization(t *testing.T) {
	t.Parallel()

	queries := []struct {
		kind string
		want typedmemory.MemberOfJudgementKind
	}{
		{kind: projectRecordKindV1, want: typedmemory.MemberJudgement},
		{kind: decisionRecordKindV1, want: typedmemory.MemberJudgement},
		{kind: specSectionRecordKindV1, want: typedmemory.NotMemberJudgement},
		{kind: evidenceRecordKindV1, want: typedmemory.NotMemberJudgement},
		{kind: supportingEpistemeRecordKindV1, want: typedmemory.NotMemberJudgement},
		{kind: workRecordKindV1, want: typedmemory.NotMemberJudgement},
		{kind: workPlanRecordKindV1, want: typedmemory.NotMemberJudgement},
	}
	for _, query := range queries {
		query := query
		t.Run(query.kind, func(t *testing.T) {
			t.Parallel()

			fixture := newRecordMembershipEvaluatorFixture(
				t,
				DecisionRecordVariantV1{},
				query.kind,
			)
			judgement := fixture.evaluate(t)
			assertRecordMembershipJudgementKind(t, judgement, query.want)
			assertRecordMembershipDefinedBasis(
				t,
				judgement,
				fixture.source.source.ObservableInput(),
			)
		})
	}
}

func TestRecordMembershipEvaluatorUndefinedBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*testing.T, *recordMembershipEvaluatorFixture)
		repair    string
	}{
		{
			name: "missing source",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.delivery = mustMissingRecordMembershipSourceDelivery(
					t,
					fixture.source.source.ObservableInput().Reference(),
				)
			},
			repair: repairMissingSourceV1,
		},
		{
			name: "valid but untrusted source",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.delivery = mustUntrustedRecordMembershipSourceDelivery(
					t,
					fixture.source.source.ObservableInput(),
					fixture.source.source.CanonicalBytes(),
				)
			},
			repair: repairUntrustedSourceV1,
		},
		{
			name: "malformed untrusted source",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				canonical := append(fixture.source.source.CanonicalBytes(), []byte(`{}`)...)
				fixture.delivery = mustUntrustedRecordMembershipSourceDelivery(
					t,
					fixture.source.source.ObservableInput(),
					canonical,
				)
			},
			repair: repairInvalidSourceV1,
		},
		{
			name: "canonical byte substitution",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				substitute := testRecordSourceFixture(t, WorkRecordVariantV1{})
				fixture.delivery = mustTrustedRecordMembershipSourceDelivery(
					t,
					fixture.source.source.ObservableInput(),
					substitute.source.CanonicalBytes(),
				)
			},
			repair: repairInvalidSourceV1,
		},
		{
			name: "project mismatch",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.project = testProject(t, "qnt_cafebabe")
			},
			repair: repairProjectMismatchV1,
		},
		{
			name: "entity mismatch",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.queryEntity = testEntity(t, "entity:another-project-record")
			},
			repair: repairEntityMismatchV1,
		},
		{
			name: "bounded context mismatch",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				other := testContext(t, "context:another-project")
				fixture.queryContext = other
				fixture.definitionContext = other
			},
			repair: repairContextMismatchV1,
		},
		{
			name: "mapping manifest mismatch",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.expectedManifest = mustRecordMembershipMappingManifest(
					t,
					"Haft.AlternateRecordAdapter",
					"1.0.0",
					0x71,
				)
			},
			repair: repairMappingMismatchV1,
		},
		{
			name: "adapter version mismatch",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.expectedAdapter = mustRecordMembershipAdapterVersion(
					t,
					"artifact-adapter/1.0.1",
				)
			},
			repair: repairAdapterMismatchV1,
		},
		{
			name: "unsupported queried kind",
			configure: func(_ *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.queryKind = "U.Entity"
				fixture.definitionKind = "U.Entity"
			},
			repair: repairUnsupportedKindV1,
		},
		{
			name: "mismatched evaluator RuleRef",
			configure: func(t *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.signatureEvaluator = mustRecordMembershipRule(
					t,
					"haft.member-of.another-record-carrier/v1",
				)
			},
			repair: repairEvaluatorMismatchV1,
		},
		{
			name: "mismatched KindSignature basis",
			configure: func(_ *testing.T, fixture *recordMembershipEvaluatorFixture) {
				fixture.definitionKind = workRecordKindV1
			},
			repair: repairDefinitionMismatchV1,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newRecordMembershipEvaluatorFixture(
				t,
				DecisionRecordVariantV1{},
				decisionRecordKindV1,
			)
			testCase.configure(t, &fixture)
			judgement := fixture.evaluate(t)
			assertRecordMembershipUndefined(t, judgement, testCase.repair)
		})
	}
}

func TestRecordMembershipDeliveryCopiesBytesAndSealDoesNotConferTrust(t *testing.T) {
	t.Parallel()

	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	canonical := fixture.source.CanonicalBytes()
	delivery := mustUntrustedRecordMembershipSourceDelivery(
		t,
		fixture.source.ObservableInput(),
		canonical,
	)
	canonical[0] ^= 0xff
	untrusted := delivery.(*untrustedRecordMembershipSourceDeliveryV1)
	if !reflect.DeepEqual(untrusted.canonical, fixture.source.CanonicalBytes()) {
		t.Fatal("untrusted delivery retained caller-owned source bytes")
	}
	if _, trusted := any(fixture.source).(RecordMembershipSourceDeliveryV1); trusted {
		t.Fatal("a locally sealed RecordMembershipSource became a trusted delivery capability")
	}

	trustedBytes := fixture.source.CanonicalBytes()
	trusted := mustTrustedRecordMembershipSourceDelivery(
		t,
		fixture.source.ObservableInput(),
		trustedBytes,
	)
	trustedBytes[0] ^= 0xff
	evaluation := newRecordMembershipEvaluatorFixtureFromSource(
		t,
		fixture,
		decisionRecordKindV1,
		trusted,
	)
	assertRecordMembershipJudgementKind(
		t,
		evaluation.evaluate(t),
		typedmemory.MemberJudgement,
	)
}

func TestRecordMembershipDeliveryUnionHasOneCanonicalPointerFormPerVariant(t *testing.T) {
	t.Parallel()

	deliveryContract := reflect.TypeOf((*RecordMembershipSourceDeliveryV1)(nil)).Elem()
	variants := []reflect.Type{
		reflect.TypeOf(trustedRecordMembershipSourceDeliveryV1{}),
		reflect.TypeOf(untrustedRecordMembershipSourceDeliveryV1{}),
		reflect.TypeOf(missingRecordMembershipSourceDeliveryV1{}),
	}
	for _, variant := range variants {
		if variant.Implements(deliveryContract) {
			t.Fatalf("delivery value form %s unexpectedly implements the closed union", variant)
		}
		if !reflect.PointerTo(variant).Implements(deliveryContract) {
			t.Fatalf("delivery pointer form *%s does not implement the closed union", variant)
		}
	}
}

func TestRecordMembershipContractHasNoApprovalOrAuthoritySurface(t *testing.T) {
	t.Parallel()

	types := []reflect.Type{
		reflect.TypeOf(RecordMembershipEvaluatorV1{}),
		reflect.TypeOf(RecordMembershipEvaluationInputV1{}),
		reflect.TypeOf(RecordMembershipEvaluationRequestV1{}),
		reflect.TypeOf(trustedRecordMembershipSourceDeliveryV1{}),
		reflect.TypeOf(untrustedRecordMembershipSourceDeliveryV1{}),
		reflect.TypeOf(missingRecordMembershipSourceDeliveryV1{}),
		reflect.TypeOf(typedmemory.MemberOfMember{}),
		reflect.TypeOf(typedmemory.MemberOfNotMember{}),
		reflect.TypeOf(typedmemory.MemberOfUndefined{}),
	}
	for _, contractType := range types {
		assertNoRecordMembershipAuthorityNames(t, contractType)
	}

	fixture := newRecordMembershipEvaluatorFixture(
		t,
		DecisionRecordVariantV1{},
		decisionRecordKindV1,
	)
	judgement := fixture.evaluate(t)
	assertRecordMembershipJudgementKind(
		t,
		judgement,
		typedmemory.MemberJudgement,
	)
	assertNoRecordMembershipAuthorityNames(t, reflect.TypeOf(judgement))
}

func TestRecordMembershipEvaluatorRejectsInvalidMechanismOrRequest(t *testing.T) {
	t.Parallel()

	fixture := newRecordMembershipEvaluatorFixture(
		t,
		DecisionRecordVariantV1{},
		decisionRecordKindV1,
	)
	if judgement, err := (RecordMembershipEvaluatorV1{}).Evaluate(
		fixture.request(t),
	); err == nil || judgement != nil {
		t.Fatalf("zero evaluator result = %#v, %v; want explicit mechanism error", judgement, err)
	}
	if judgement, err := fixture.evaluator.Evaluate(
		RecordMembershipEvaluationRequestV1{},
	); err == nil || judgement != nil {
		t.Fatalf("zero request result = %#v, %v; want explicit request error", judgement, err)
	}
}

type recordMembershipEvaluatorFixture struct {
	source             recordSourceFixture
	evaluator          RecordMembershipEvaluatorV1
	typeEnv            typedmemory.TypeEnvRef
	project            projectidentity.ProjectID
	queryEntity        typedmemory.EntityID
	queryContext       typedmemory.BoundedContextRef
	definitionContext  typedmemory.BoundedContextRef
	queryKind          string
	definitionKind     string
	signatureEvaluator typedmemory.RuleRef
	expectedManifest   MappingManifestRef
	expectedAdapter    AdapterVersion
	delivery           RecordMembershipSourceDeliveryV1
}

func newRecordMembershipEvaluatorFixture(
	t *testing.T,
	variant ProjectRecordCarrierVariantV1,
	queryKind string,
) recordMembershipEvaluatorFixture {
	t.Helper()
	source := testRecordSourceFixture(t, variant)
	delivery := mustTrustedRecordMembershipSourceDelivery(
		t,
		source.source.ObservableInput(),
		source.source.CanonicalBytes(),
	)
	return newRecordMembershipEvaluatorFixtureFromSource(
		t,
		source,
		queryKind,
		delivery,
	)
}

func newRecordMembershipEvaluatorFixtureFromSource(
	t *testing.T,
	source recordSourceFixture,
	queryKind string,
	delivery RecordMembershipSourceDeliveryV1,
) recordMembershipEvaluatorFixture {
	t.Helper()
	evaluator := NewRecordMembershipEvaluatorV1()
	return recordMembershipEvaluatorFixture{
		source:             source,
		evaluator:          evaluator,
		typeEnv:            mustRecordMembershipTypeEnvRef(t, 0x81),
		project:            source.project,
		queryEntity:        source.entity,
		queryContext:       source.context,
		definitionContext:  source.context,
		queryKind:          queryKind,
		definitionKind:     queryKind,
		signatureEvaluator: evaluator.RuleRef(),
		expectedManifest:   source.manifest,
		expectedAdapter:    source.adapter,
		delivery:           delivery,
	}
}

func (fixture recordMembershipEvaluatorFixture) evaluate(
	t *testing.T,
) typedmemory.MemberOfJudgement {
	t.Helper()
	request := fixture.request(t)
	judgement, err := fixture.evaluator.Evaluate(request)
	if err != nil {
		t.Fatalf("RecordMembershipEvaluatorV1.Evaluate() error = %v", err)
	}
	return judgement
}

func (fixture recordMembershipEvaluatorFixture) request(
	t *testing.T,
) RecordMembershipEvaluationRequestV1 {
	t.Helper()
	queryKind := mustRecordMembershipValueKind(t, fixture.typeEnv, fixture.queryKind)
	query, err := typedmemory.NewMemberOfQuery(
		fixture.queryEntity,
		queryKind,
		mustRecordMembershipContextSlice(t, fixture.queryContext),
	)
	if err != nil {
		t.Fatalf("typedmemory.NewMemberOfQuery() error = %v", err)
	}
	declarationProvenance := mustRecordMembershipDeclarationProvenance(t)
	entitySet, err := typedmemory.NewEntitySetDefinition(
		typedmemory.EntitySetDefinitionInput{
			TypeEnv:         fixture.typeEnv,
			Context:         fixture.definitionContext,
			EnumerationRule: mustRecordMembershipRule(t, "haft.entity-set.project-records/v1"),
			CandidatePolicy: typedmemory.PersistedEntitiesOnly{},
			Provenance:      declarationProvenance,
		},
	)
	if err != nil {
		t.Fatalf("typedmemory.NewEntitySetDefinition() error = %v", err)
	}
	definitionKind := mustRecordMembershipValueKind(
		t,
		fixture.typeEnv,
		fixture.definitionKind,
	)
	signature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       definitionKind,
			Formality:       typedmemory.SignatureF4,
			DefinednessRule: mustRecordMembershipRule(t, fixture.signatureEvaluator.String()+"/definedness"),
			Evaluator:       fixture.signatureEvaluator,
			EntitySet:       entitySet.Ref(),
			Provenance:      declarationProvenance,
		},
	)
	if err != nil {
		t.Fatalf("typedmemory.NewKindSignatureDefinition() error = %v", err)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		fixture.typeEnv,
		typedmemory.NewGraphRevision(17),
	)
	if err != nil {
		t.Fatalf("typedmemory.NewPersistedSnapshotView() error = %v", err)
	}
	request, err := NewRecordMembershipEvaluationRequestV1(
		RecordMembershipEvaluationInputV1{
			ProjectID:               fixture.project,
			Query:                   query,
			EvaluationView:          view,
			KindSignature:           signature,
			EntitySet:               entitySet,
			EvaluationProvenance:    mustRecordMembershipEvaluationProvenance(t),
			ExpectedMappingManifest: fixture.expectedManifest,
			ExpectedAdapterVersion:  fixture.expectedAdapter,
			SourceDelivery:          fixture.delivery,
		},
	)
	if err != nil {
		t.Fatalf("NewRecordMembershipEvaluationRequestV1() error = %v", err)
	}
	return request
}

func recordMembershipTestVariants() []ProjectRecordCarrierVariantV1 {
	return []ProjectRecordCarrierVariantV1{
		GenericProjectRecordVariantV1{},
		DecisionRecordVariantV1{},
		SpecSectionRecordVariantV1{},
		EvidenceRecordVariantV1{},
		SupportingEpistemeRecordVariantV1{},
		WorkRecordVariantV1{},
		WorkPlanRecordVariantV1{},
	}
}

func mustTrustedRecordMembershipSourceDelivery(
	t *testing.T,
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) *trustedRecordMembershipSourceDeliveryV1 {
	t.Helper()
	delivery, err := newTrustedRecordMembershipSourceDeliveryV1(expected, canonical)
	if err != nil {
		t.Fatalf("newTrustedRecordMembershipSourceDeliveryV1() error = %v", err)
	}
	return delivery
}

func mustUntrustedRecordMembershipSourceDelivery(
	t *testing.T,
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) RecordMembershipSourceDeliveryV1 {
	t.Helper()
	delivery, err := NewUntrustedRecordMembershipSourceDeliveryV1(expected, canonical)
	if err != nil {
		t.Fatalf("NewUntrustedRecordMembershipSourceDeliveryV1() error = %v", err)
	}
	return delivery
}

func mustMissingRecordMembershipSourceDelivery(
	t *testing.T,
	expected typedmemory.ObservableInputRef,
) RecordMembershipSourceDeliveryV1 {
	t.Helper()
	delivery, err := NewMissingRecordMembershipSourceDeliveryV1(expected)
	if err != nil {
		t.Fatalf("NewMissingRecordMembershipSourceDeliveryV1() error = %v", err)
	}
	return delivery
}

func mustRecordMembershipTypeEnvRef(
	t *testing.T,
	fill byte,
) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.NewTypeEnvRef(testDigest(t, fill))
	if err != nil {
		t.Fatalf("typedmemory.NewTypeEnvRef() error = %v", err)
	}
	return ref
}

func mustRecordMembershipValueKind(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	raw string,
) typedmemory.ValueKindRef {
	t.Helper()
	kind, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatalf("typedmemory.NewKindID(%q) error = %v", raw, err)
	}
	ref, err := typedmemory.NewValueKindRef(typeEnv, kind)
	if err != nil {
		t.Fatalf("typedmemory.NewValueKindRef(%q) error = %v", raw, err)
	}
	return ref
}

func mustRecordMembershipContextSlice(
	t *testing.T,
	context typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, time.July, 17, 9, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("typedmemory.NewGammaPoint() error = %v", err)
	}
	slice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   context,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("typedmemory.NewContextSlice() error = %v", err)
	}
	return slice
}

func mustRecordMembershipDeclarationProvenance(
	t *testing.T,
) typedmemory.DeclarationProvenance {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID("local-practice:haft-record-membership")
	if err != nil {
		t.Fatalf("typedmemory.NewSourceUnitID() error = %v", err)
	}
	revision, err := typedmemory.NewSourceRevision("haft-local-practice-v1")
	if err != nil {
		t.Fatalf("typedmemory.NewSourceRevision() error = %v", err)
	}
	lineRange, err := typedmemory.NewSourceLineRange(1, 4)
	if err != nil {
		t.Fatalf("typedmemory.NewSourceLineRange() error = %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		testDigest(t, 0x82),
		lineRange,
	)
	if err != nil {
		t.Fatalf("typedmemory.NewUnpatternedSourceLocation() error = %v", err)
	}
	reference, err := typedmemory.NewProvenanceRef("prov:haft-record-membership/v1")
	if err != nil {
		t.Fatalf("typedmemory.NewProvenanceRef() error = %v", err)
	}
	rule, err := typedmemory.NewCompilerRuleID("haft-record-membership-v1")
	if err != nil {
		t.Fatalf("typedmemory.NewCompilerRuleID() error = %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(reference, location, rule)
	if err != nil {
		t.Fatalf("typedmemory.NewFPFSourceProvenance() error = %v", err)
	}
	return provenance
}

func mustRecordMembershipEvaluationProvenance(
	t *testing.T,
) typedmemory.MemberOfEvaluationProvenance {
	t.Helper()
	reference, err := typedmemory.NewProvenanceRef("prov:record-membership-evaluation/v1")
	if err != nil {
		t.Fatalf("typedmemory.NewProvenanceRef() error = %v", err)
	}
	artifact, err := typedmemory.NewCarrierRef("artifact:record-membership-evaluator/v1")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef() error = %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("build-20260717.1")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition() error = %v", err)
	}
	provenance, err := typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         reference,
			EvaluatorArtifact: artifact,
			EvaluatorEdition:  edition,
			EvaluatorDigest:   testDigest(t, 0x83),
		},
	)
	if err != nil {
		t.Fatalf("typedmemory.NewMemberOfEvaluationProvenance() error = %v", err)
	}
	return provenance
}

func mustRecordMembershipRule(
	t *testing.T,
	raw string,
) typedmemory.RuleRef {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("typedmemory.NewRuleRef(%q) error = %v", raw, err)
	}
	return rule
}

func mustRecordMembershipMappingManifest(
	t *testing.T,
	id string,
	version string,
	fill byte,
) MappingManifestRef {
	t.Helper()
	manifest, err := NewMappingManifestRef(id, version, testDigest(t, fill))
	if err != nil {
		t.Fatalf("NewMappingManifestRef() error = %v", err)
	}
	return manifest
}

func mustRecordMembershipAdapterVersion(
	t *testing.T,
	raw string,
) AdapterVersion {
	t.Helper()
	version, err := NewAdapterVersion(raw)
	if err != nil {
		t.Fatalf("NewAdapterVersion() error = %v", err)
	}
	return version
}

func assertRecordMembershipJudgementKind(
	t *testing.T,
	judgement typedmemory.MemberOfJudgement,
	want typedmemory.MemberOfJudgementKind,
) {
	t.Helper()
	if judgement == nil {
		t.Fatal("record membership evaluator returned a nil judgement")
	}
	if judgement.Kind() != want {
		t.Fatalf(
			"record membership judgement = %s; want %s",
			judgement.Kind().String(),
			want.String(),
		)
	}
}

func assertRecordMembershipDefinedBasis(
	t *testing.T,
	judgement typedmemory.MemberOfJudgement,
	wantObservable typedmemory.MemberOfObservableInput,
) {
	t.Helper()
	defined, ok := judgement.(typedmemory.DefinedMemberOfJudgement)
	if !ok {
		t.Fatalf("record membership judgement %T has no defined basis", judgement)
	}
	inputs := defined.Basis().ObservableInputs()
	if len(inputs) != 1 || inputs[0] != wantObservable {
		t.Fatalf("defined observable inputs = %#v; want exact membership source", inputs)
	}
}

func assertRecordMembershipUndefined(
	t *testing.T,
	judgement typedmemory.MemberOfJudgement,
	wantRepair string,
) {
	t.Helper()
	assertRecordMembershipJudgementKind(
		t,
		judgement,
		typedmemory.UndefinedMemberJudgement,
	)
	undefined, ok := judgement.(typedmemory.MemberOfUndefined)
	if !ok {
		t.Fatalf("undefined record membership judgement type = %T", judgement)
	}
	if undefined.Repair().String() != wantRepair {
		t.Fatalf(
			"undefined repair = %q; want %q",
			undefined.Repair().String(),
			wantRepair,
		)
	}
}

func assertNoRecordMembershipAuthorityNames(
	t *testing.T,
	contractType reflect.Type,
) {
	t.Helper()
	denied := []string{
		"admission",
		"activation",
		"approval",
		"authority",
		"authorization",
		"commitment",
		"permission",
		"speechact",
	}
	for index := 0; index < contractType.NumField(); index++ {
		name := strings.ToLower(contractType.Field(index).Name)
		for _, fragment := range denied {
			if strings.Contains(name, fragment) {
				t.Fatalf("%s exposes authority-like field %q", contractType, name)
			}
		}
	}
	for index := 0; index < contractType.NumMethod(); index++ {
		name := strings.ToLower(contractType.Method(index).Name)
		for _, fragment := range denied {
			if strings.Contains(name, fragment) {
				t.Fatalf("%s exposes authority-like method %q", contractType, name)
			}
		}
	}
}
