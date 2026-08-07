package projectmemory

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestRecordMembershipAdmissionEngineAcceptsExactRegisteredSource(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	blob := recordMembershipEngineObservableBlob(t, fixture.source)

	delivery, manifest, adapter, err := fixture.engine.trustedDelivery(
		[]typedmemorystore.ObservableInputBlob{blob},
	)
	if err != nil {
		t.Fatalf("trustedDelivery() error = %v", err)
	}
	if delivery == nil {
		t.Fatal("trustedDelivery() returned a nil trusted capability")
	}
	if manifest != fixture.manifest {
		t.Fatalf("trusted manifest = %s, want %s", manifest.String(), fixture.manifest.String())
	}
	if adapter != fixture.adapter {
		t.Fatalf("trusted adapter = %s, want %s", adapter.String(), fixture.adapter.String())
	}

	request := recordMembershipEngineEvaluationRequest(
		t,
		fixture,
		delivery,
		manifest,
		adapter,
	)
	registration, err := fixture.engine.evaluatorRegistration(
		fixture.policy.Evaluator(),
	)
	if err != nil {
		t.Fatalf("evaluatorRegistration() error = %v", err)
	}
	judgement, err := registration.Evaluator().Evaluate(request)
	if err != nil {
		t.Fatalf("registered evaluator error = %v", err)
	}
	if judgement.Kind() != typedmemory.MemberJudgement {
		t.Fatalf("registered evaluator judgement = %s, want member", judgement.Kind().String())
	}
	defined, ok := judgement.(typedmemory.DefinedMemberOfJudgement)
	if !ok {
		t.Fatalf("registered evaluator judgement = %T, want defined judgement", judgement)
	}
	inputs := defined.Basis().ObservableInputs()
	if len(inputs) != 1 || inputs[0] != fixture.source.ObservableInput() {
		t.Fatalf("defined observable inputs = %#v, want exact source", inputs)
	}
}

func TestRecordMembershipAdmissionEngineExecutesExactC32PrerequisiteChain(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	semantic := recordMembershipEngineSemanticFixture(
		t,
		fixture,
		typedmemory.PersistedEntitiesOnly{},
		recordMembershipEnginePersistedView(t),
	)

	result, err := fixture.engine.evaluateExactKindPrerequisites(
		semantic.request,
		semantic.signature,
		semantic.entitySet,
		[]typedmemory.EntityID{fixture.entity},
		semantic.universeInput,
	)
	if err != nil {
		t.Fatalf("evaluateExactKindPrerequisites() error = %v", err)
	}
	satisfied, ok := result.(recordMembershipKindPrerequisitesSatisfied)
	if !ok {
		t.Fatalf("prerequisite result = %T, want satisfied", result)
	}
	if !satisfied.enumeration.Contains(fixture.entity) {
		t.Fatal("EntitySet enumeration lost the exact persisted entity")
	}
	if satisfied.definedness.SignatureRef() != semantic.signature.Ref() {
		t.Fatal("Kind definedness lost the exact KindSignature")
	}
	if satisfied.certificate.MemberOfRequestDigest() != semantic.request.Digest() {
		t.Fatal("C.3.2 certificate lost the exact MemberOf request")
	}
	if len(satisfied.observableInputs) != 1 ||
		satisfied.observableInputs[0] != semantic.universeInput {
		t.Fatal("C.3.2 prerequisite result lost the persisted universe input")
	}

	blob := recordMembershipEngineObservableBlob(t, fixture.source)
	delivery, manifest, adapter, err := fixture.engine.trustedDelivery(
		[]typedmemorystore.ObservableInputBlob{blob},
	)
	if err != nil {
		t.Fatalf("trustedDelivery() error = %v", err)
	}
	provenance := mustRecordMembershipEngineValue(
		recordMembershipEvaluationProvenance(fixture.policy.Evaluator()))
	evaluation := mustRecordMembershipEngineValue(
		recordcarrier.NewRecordMembershipEvaluationRequestV3(
			recordcarrier.RecordMembershipEvaluationInputV3{
				ProjectID:                    fixture.project,
				Query:                        semantic.request.Query(),
				EvaluationView:               semantic.request.View(),
				KindSignature:                semantic.signature,
				EntitySet:                    semantic.entitySet,
				EvaluationProvenance:         provenance,
				ExpectedMappingManifest:      manifest,
				ExpectedAdapterVersion:       adapter,
				SourceDelivery:               delivery,
				Prerequisites:                satisfied.certificate,
				PrerequisiteObservableInputs: satisfied.observableInputs,
			},
		))
	judgement, err := recordcarrier.NewRecordMembershipEvaluatorV1().Evaluate(
		evaluation.RegisteredRequest(),
	)
	if err != nil {
		t.Fatalf("V3 record membership evaluation error = %v", err)
	}
	defined, ok := judgement.(typedmemory.DefinedMemberOfJudgement)
	if !ok {
		t.Fatalf("V3 record membership result = %T, want defined", judgement)
	}
	posture, ok := defined.Basis().Posture().(typedmemory.C32PrerequisiteMemberOfBasisV3)
	if !ok || posture.Certificate().Digest() != satisfied.certificate.Digest() {
		t.Fatal("final MemberOf basis did not retain the exact C.3.2 certificate")
	}
	inputs := defined.Basis().ObservableInputs()
	if !recordMembershipInputsContain(inputs, semantic.universeInput) ||
		!recordMembershipInputsContain(inputs, fixture.source.ObservableInput()) ||
		len(inputs) != 2 {
		t.Fatalf(
			"final MemberOf observable basis = %#v, want universe plus record source",
			inputs,
		)
	}
}

func recordMembershipInputsContain(
	inputs []typedmemory.MemberOfObservableInput,
	expected typedmemory.MemberOfObservableInput,
) bool {
	for _, input := range inputs {
		if input == expected {
			return true
		}
	}
	return false
}

func TestRecordMembershipAdmissionEngineKeepsEntityOutsideSetUndefined(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	semantic := recordMembershipEngineSemanticFixture(
		t,
		fixture,
		typedmemory.PersistedEntitiesOnly{},
		recordMembershipEnginePersistedView(t),
	)

	result, err := fixture.engine.evaluateExactKindPrerequisites(
		semantic.request,
		semantic.signature,
		semantic.entitySet,
		nil,
		semantic.universeInput,
	)
	if err != nil {
		t.Fatalf("evaluateExactKindPrerequisites() error = %v", err)
	}
	undefined, ok := result.(recordMembershipKindPrerequisitesUndefined)
	if !ok {
		t.Fatalf("prerequisite result = %T, want undefined", result)
	}
	if undefined.judgement.Kind() != typedmemory.UndefinedMemberJudgement {
		t.Fatalf(
			"prerequisite judgement = %s, want undefined",
			undefined.judgement.Kind().String(),
		)
	}
	missing := undefined.judgement.MissingBasis()
	if len(missing) != 1 || missing[0].Kind() != typedmemory.MissingMemberOfKindSignature {
		t.Fatalf("undefined missing basis = %#v, want KindSignature", missing)
	}
}

func TestRecordMembershipAdmissionEngineEvaluatesProspectiveCandidateVisibility(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	policy := mustRecordMembershipEngineValue(
		typedmemory.NewPriorBatchDeclarationsVisible(fixture.visibility))
	view := recordMembershipEngineProspectiveView(t, fixture)
	semantic := recordMembershipEngineSemanticFixture(t, fixture, policy, view)

	result, err := fixture.engine.evaluateExactKindPrerequisites(
		semantic.request,
		semantic.signature,
		semantic.entitySet,
		nil,
		semantic.universeInput,
	)
	if err != nil {
		t.Fatalf("evaluateExactKindPrerequisites() error = %v", err)
	}
	satisfied, ok := result.(recordMembershipKindPrerequisitesSatisfied)
	if !ok {
		t.Fatalf("prospective prerequisite result = %T, want satisfied", result)
	}
	if !satisfied.enumeration.Contains(fixture.entity) {
		t.Fatal("prospective EntitySet did not include the visible prior declaration")
	}
}

func TestRecordMembershipAdmissionEngineDoesNotRunUnpinnedKindRule(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	semantic := recordMembershipEngineSemanticFixture(
		t,
		fixture,
		typedmemory.PersistedEntitiesOnly{},
		recordMembershipEnginePersistedView(t),
	)
	otherRule := mustRecordMembershipEngineValue(
		typedmemory.NewRuleRef("haft.kind.project-record/unpinned-definedness/v1"))
	otherSignature := mustRecordMembershipEngineValue(
		typedmemory.NewKindSignatureDefinition(
			typedmemory.KindSignatureDefinitionInput{
				ValueKind:       semantic.signature.ValueKind(),
				Formality:       typedmemory.SignatureF4,
				DefinednessRule: otherRule,
				Evaluator:       recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
				EntitySet:       semantic.entitySet.Ref(),
				Provenance:      recordMembershipEngineDeclarationProvenance(t),
			},
		))

	result, err := fixture.engine.evaluateExactKindPrerequisites(
		semantic.request,
		otherSignature,
		semantic.entitySet,
		[]typedmemory.EntityID{fixture.entity},
		semantic.universeInput,
	)
	if err == nil || result != nil {
		t.Fatalf(
			"unpinned Kind rule result = %T, %v; want fail-closed runtime error",
			result,
			err,
		)
	}
}

func TestRecordMembershipAdmissionEngineRejectsUnregisteredMappingOrAdapter(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	otherManifest := recordMembershipEngineManifest(t, "Haft.OtherRecordAdapter", 0x71)
	otherAdapter := mustRecordMembershipEngineValue(
		recordmapping.NewAdapterVersion("other-record-adapter/1.0.0"))

	testCases := []struct {
		name     string
		manifest recordmapping.MappingManifestRef
		adapter  recordmapping.AdapterVersion
	}{
		{
			name:     "mapping manifest",
			manifest: otherManifest,
			adapter:  fixture.adapter,
		},
		{
			name:     "adapter version",
			manifest: fixture.manifest,
			adapter:  otherAdapter,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			source := recordMembershipEngineSource(
				t,
				fixture.project,
				fixture.entity,
				fixture.context,
				testCase.manifest,
				testCase.adapter,
			)
			blob := recordMembershipEngineObservableBlob(t, source)
			delivery, _, _, err := fixture.engine.trustedDelivery(
				[]typedmemorystore.ObservableInputBlob{blob},
			)
			if err == nil || delivery != nil {
				t.Fatalf("trustedDelivery() = %T, %v; want fail-closed rejection", delivery, err)
			}
		})
	}
}

func TestRecordMembershipAdmissionEngineRejectsSubstitutedObservable(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	otherEntity := mustRecordMembershipEngineValue(
		typedmemory.NewEntityID("entity:other-project-record"))

	otherSource := recordMembershipEngineSource(
		t,
		fixture.project,
		otherEntity,
		fixture.context,
		fixture.manifest,
		fixture.adapter,
	)
	substituted := mustRecordMembershipEngineValue(
		typedmemorystore.NewObservableInputBlob(
			fixture.source.ObservableInput().Reference(),
			otherSource.Digest(),
			otherSource.CanonicalBytes(),
		))

	delivery, _, _, err := fixture.engine.trustedDelivery(
		[]typedmemorystore.ObservableInputBlob{substituted},
	)
	if err == nil || delivery != nil {
		t.Fatalf("substituted source produced trusted delivery %T, %v", delivery, err)
	}

	_, err = typedmemorystore.NewObservableInputBlob(
		fixture.source.ObservableInput().Reference(),
		otherSource.Digest(),
		fixture.source.CanonicalBytes(),
	)
	if err == nil {
		t.Fatal("substituted digest was accepted for the original source bytes")
	}
}

func TestRecordMembershipAdmissionEngineRequiresExactRegistrationPolicy(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)
	runtime := recordMembershipEngineRuntimeWithoutPolicy(t, fixture)
	if _, ok := exactRecordMembershipPolicy(runtime); ok {
		t.Fatal("runtime without an X-selected policy exposed an exact policy")
	}
	engine, err := NewRecordMembershipAdmissionEngine(runtime)
	if err == nil || engine.recordMembership.Len() != 0 {
		t.Fatalf("NewRecordMembershipAdmissionEngine() = %#v, %v; want missing-policy rejection", engine, err)
	}
}

func TestSelectedRecordMembershipRuntimeBindsExactXAndRegistryCoordinates(
	t *testing.T,
) {
	t.Parallel()
	fixture := newRecordMembershipAdmissionEngineFixture(t)

	selected, err := selectedMemberOfRuntime(
		fixture.runtimeBasis,
		fixture.runtime,
	)
	if err != nil {
		t.Fatalf("selectedMemberOfRuntime() error = %v", err)
	}
	exact, ok := selected.(typedmemorystore.ExactMemberOfRuntime)
	if !ok {
		t.Fatalf("selected membership posture = %T, want ExactMemberOfRuntime", selected)
	}
	if exact.RuntimeBasisDigest().Digest() != fixture.runtimeBasis.Digest() {
		t.Fatal("selected membership runtime lost the exact X digest")
	}
	registryDigest, ok := fixture.runtime.CoordinateDigest()
	if !ok {
		t.Fatal("fixture runtime did not expose its coordinate digest")
	}
	if exact.RegistryCoordinateDigest().Digest() != registryDigest {
		t.Fatal("selected membership runtime lost the exact target-registry digest")
	}
}

type recordMembershipAdmissionEngineFixture struct {
	project      projectidentity.ProjectID
	entity       typedmemory.EntityID
	context      typedmemory.BoundedContextRef
	manifest     recordmapping.MappingManifestRef
	adapter      recordmapping.AdapterVersion
	enumeration  typedmemory.RuleRef
	visibility   typedmemory.RuleRef
	definedness  typedmemory.RuleRef
	source       recordcarrier.RecordMembershipSourceV1
	catalog      runtimemechanism.RuntimeMechanismArtifactV1
	policy       recordmembershipregistration.RegistrationArtifactV1
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact
	runtime      projecttypeenvruntime.ExactTargetRuntimeRegistry
	engine       RecordMembershipAdmissionEngine
}

func newRecordMembershipAdmissionEngineFixture(
	t *testing.T,
) recordMembershipAdmissionEngineFixture {
	t.Helper()
	project := mustRecordMembershipEngineValue(
		projectidentity.ParseProjectID("qnt_deadbeef"))

	entity := mustRecordMembershipEngineValue(
		typedmemory.NewEntityID("entity:project-record-1"))

	contextRef := mustRecordMembershipEngineValue(
		typedmemory.NewBoundedContextRef("context:haft-project"))

	manifest := recordMembershipEngineManifest(t, "Haft.ProjectRecordAdapter", 0x51)
	adapter := mustRecordMembershipEngineValue(
		recordmapping.NewAdapterVersion("project-record-adapter/1.0.0"))

	source := recordMembershipEngineSource(
		t,
		project,
		entity,
		contextRef,
		manifest,
		adapter,
	)
	rule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	enumerationRule := mustRecordMembershipEngineValue(
		typedmemory.NewRuleRef("haft.entity-set.project-records/v1"))
	visibilityRule := mustRecordMembershipEngineValue(
		typedmemory.NewRuleRef("haft.entity-set.project-records/prior-declarations/v1"))
	definednessRule := mustRecordMembershipEngineValue(
		typedmemory.NewRuleRef("haft.kind.project-record/definedness/v1"))
	catalog := recordMembershipEngineCatalog(
		t,
		rule,
		enumerationRule,
		visibilityRule,
		definednessRule,
	)
	policy := recordMembershipEnginePolicy(t, catalog, rule, manifest, adapter)
	basis := recordMembershipEngineRuntimeBasis(
		t,
		catalog,
		policy,
		rule,
		enumerationRule,
		visibilityRule,
		definednessRule,
	)
	identity := recordMembershipEngineMechanismIdentity(t, catalog)
	recordCarrierEvaluators := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewRecordMembershipRegistry(identity))
	enumerators := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewEntitySetEnumerationRegistry(
			enumerationRule,
			identity,
		))
	visibilityEvaluators := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewCandidateVisibilityRegistry(
			visibilityRule,
			identity,
		))
	definednessEvaluators := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewKindDefinednessRegistry(
			definednessRule,
			identity,
		))
	installedEngine := mustRecordMembershipEngineValue(
		NewRecordMembershipAdmissionEngineBuilder().
			SetEntitySetEnumeration(enumerators).
			SetCandidateVisibility(visibilityEvaluators).
			SetKindDefinedness(definednessEvaluators).
			SetRecordCarrierMembership(recordCarrierEvaluators).
			SetRegistrationPolicy(policy).
			Build())
	memberOfEvaluators := mustRecordMembershipEngineValue(
		NewRecordMembershipEvaluatorRegistry(installedEngine))

	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:                         typedmemory.NewCodecRegistry(),
				EntitySetEnumerationEvaluators: enumerators,
				CandidateVisibilityEvaluators:  visibilityEvaluators,
				KindDefinednessEvaluators:      definednessEvaluators,
				MemberOfEvaluators:             memberOfEvaluators,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					policy,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("ObserveCurrentTargetRuntime() = %s, want matched", resolution.Kind().String())
	}
	runtime, ok := matched.Registry()
	if !ok {
		t.Fatal("matched runtime did not expose its exact registry")
	}
	engine, err := NewRecordMembershipAdmissionEngine(runtime)
	if err != nil {
		t.Fatalf("NewRecordMembershipAdmissionEngine() error = %v", err)
	}
	return recordMembershipAdmissionEngineFixture{
		project:      project,
		entity:       entity,
		context:      contextRef,
		manifest:     manifest,
		adapter:      adapter,
		enumeration:  enumerationRule,
		visibility:   visibilityRule,
		definedness:  definednessRule,
		source:       source,
		catalog:      catalog,
		policy:       policy,
		runtimeBasis: basis,
		runtime:      runtime,
		engine:       engine,
	}
}

type recordMembershipEngineSemanticBasis struct {
	request       typedmemory.MemberOfEvaluationRequest
	signature     typedmemory.KindSignatureDefinition
	entitySet     typedmemory.EntitySetDefinition
	universeInput typedmemory.MemberOfObservableInput
}

func recordMembershipEngineSemanticFixture(
	t *testing.T,
	fixture recordMembershipAdmissionEngineFixture,
	policy typedmemory.EntitySetCandidatePolicy,
	view typedmemory.MemberOfEvaluationView,
) recordMembershipEngineSemanticBasis {
	t.Helper()
	typeEnv := recordMembershipEngineTypeEnvRef(t)
	kindID := mustRecordMembershipEngineValue(
		typedmemory.NewKindID("Haft.ProjectRecord"))
	valueKind := mustRecordMembershipEngineValue(
		typedmemory.NewValueKindRef(typeEnv, kindID))
	gamma := mustRecordMembershipEngineValue(
		typedmemory.NewGammaPoint(
			time.Date(2026, time.July, 17, 9, 0, 0, 0, time.UTC),
		))
	contextSlice := mustRecordMembershipEngineValue(
		typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
			Context:   fixture.context,
			GammaTime: gamma,
		}))
	query := mustRecordMembershipEngineValue(
		typedmemory.NewMemberOfQuery(fixture.entity, valueKind, contextSlice))
	provenance := recordMembershipEngineDeclarationProvenance(t)
	entitySet := mustRecordMembershipEngineValue(
		typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
			TypeEnv:         typeEnv,
			Context:         fixture.context,
			EnumerationRule: fixture.enumeration,
			CandidatePolicy: policy,
			Provenance:      provenance,
		}))
	signature := mustRecordMembershipEngineValue(
		typedmemory.NewKindSignatureDefinition(
			typedmemory.KindSignatureDefinitionInput{
				ValueKind:       valueKind,
				Formality:       typedmemory.SignatureF4,
				DefinednessRule: fixture.definedness,
				Evaluator:       recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
				EntitySet:       entitySet.Ref(),
				Provenance:      provenance,
			},
		))
	request := mustRecordMembershipEngineValue(
		typedmemory.NewMemberOfEvaluationRequest(query, view))
	observableRef := mustRecordMembershipEngineValue(
		typedmemory.NewObservableInputRef(
			"persisted-entity-universe:record-membership-engine-test"))
	universeInput := mustRecordMembershipEngineValue(
		typedmemory.NewMemberOfObservableInput(
			observableRef,
			recordMembershipEngineDigest(t, 0xb1),
		))
	return recordMembershipEngineSemanticBasis{
		request:       request,
		signature:     signature,
		entitySet:     entitySet,
		universeInput: universeInput,
	}
}

func recordMembershipEngineTypeEnvRef(
	t *testing.T,
) typedmemory.TypeEnvRef {
	t.Helper()
	return mustRecordMembershipEngineValue(
		typedmemory.NewTypeEnvRef(recordMembershipEngineDigest(t, 0x81)))
}

func recordMembershipEnginePersistedView(
	t *testing.T,
) typedmemory.PersistedSnapshotView {
	t.Helper()
	return mustRecordMembershipEngineValue(
		typedmemory.NewPersistedSnapshotView(
			recordMembershipEngineTypeEnvRef(t),
			typedmemory.NewGraphRevision(17),
		))
}

func recordMembershipEngineProspectiveView(
	t *testing.T,
	fixture recordMembershipAdmissionEngineFixture,
) typedmemory.ProspectiveBatchView {
	t.Helper()
	typeEnv := recordMembershipEngineTypeEnvRef(t)
	local := mustRecordMembershipEngineValue(
		typedmemory.NewBatchLocalRef("local:prospective-project-record"))
	label := mustRecordMembershipEngineValue(
		typedmemory.NewEntityLabel("Prospective project record"))
	provenance := mustRecordMembershipEngineValue(
		typedmemory.NewProvenanceRef(
			"prov:prospective-project-record"))
	declaration := mustRecordMembershipEngineValue(
		typedmemory.NewDeclareEntity(
			fixture.entity,
			local,
			fixture.context,
			label,
			provenance,
		))
	changeSet := mustRecordMembershipEngineValue(
		typedmemory.NewMemoryChangeSet(
			[]typedmemory.MemoryChange{declaration},
		))
	prefix := mustRecordMembershipEngineValue(
		typedmemory.ComputeOrderedCandidatePrefix(changeSet, 1))
	refKindID := mustRecordMembershipEngineValue(
		typedmemory.NewRefKindID("Haft.ProjectRecordRef"))
	refKind := mustRecordMembershipEngineValue(
		typedmemory.NewRefKindRef(typeEnv, refKindID))
	localReference := mustRecordMembershipEngineValue(
		typedmemory.NewLocalRef(refKind, local))
	referenceID := mustRecordMembershipEngineValue(
		typedmemory.NewReferenceID(fixture.entity.String()))
	persistedReference := mustRecordMembershipEngineValue(
		typedmemory.NewPersistedRef(refKind, referenceID))
	return mustRecordMembershipEngineValue(
		typedmemory.NewProspectiveBatchView(
			typedmemory.ProspectiveBatchViewInput{
				TypeEnv:                  typeEnv,
				PreStateGraphRevision:    typedmemory.NewGraphRevision(17),
				EvaluationChangeOrdinal:  1,
				DeclarationChangeOrdinal: 0,
				Declaration:              declaration,
				LocalReference:           localReference,
				PersistedReference:       persistedReference,
				OrderedCandidatePrefix:   prefix,
			},
		))
}

func recordMembershipEngineSource(
	t *testing.T,
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) recordcarrier.RecordMembershipSourceV1 {
	t.Helper()
	carrier := mustRecordMembershipEngineValue(
		recordcarrier.SealProjectRecordCarrierV1(
			entity,
			contextRef,
			recordcarrier.GenericProjectRecordVariantV1{},
		))

	binding := mustRecordMembershipEngineValue(
		recordcarrier.SealEntityRecordCarrierBindingV1(
			project,
			carrier,
			manifest,
			adapter,
		))

	return mustRecordMembershipEngineValue(
		recordcarrier.SealRecordMembershipSourceV1(
			project,
			entity,
			contextRef,
			carrier,
			binding,
		))
}

func recordMembershipEngineObservableBlob(
	t *testing.T,
	source recordcarrier.RecordMembershipSourceV1,
) typedmemorystore.ObservableInputBlob {
	t.Helper()
	observable := source.ObservableInput()
	return mustRecordMembershipEngineValue(
		typedmemorystore.NewObservableInputBlob(
			observable.Reference(),
			observable.Digest(),
			source.CanonicalBytes(),
		))
}

func recordMembershipEngineEvaluationRequest(
	t *testing.T,
	fixture recordMembershipAdmissionEngineFixture,
	delivery recordcarrier.RecordMembershipSourceDeliveryV1,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) recordcarrier.RecordMembershipEvaluationRequestV1 {
	t.Helper()
	typeEnv := mustRecordMembershipEngineValue(
		typedmemory.NewTypeEnvRef(recordMembershipEngineDigest(t, 0x81)))

	kindID := mustRecordMembershipEngineValue(
		typedmemory.NewKindID("Haft.ProjectRecord"))

	valueKind := mustRecordMembershipEngineValue(
		typedmemory.NewValueKindRef(typeEnv, kindID))

	gamma := mustRecordMembershipEngineValue(
		typedmemory.NewGammaPoint(
			time.Date(2026, time.July, 17, 9, 0, 0, 0, time.UTC),
		))

	contextSlice := mustRecordMembershipEngineValue(
		typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
			Context:   fixture.context,
			GammaTime: gamma,
		}))

	query := mustRecordMembershipEngineValue(
		typedmemory.NewMemberOfQuery(fixture.entity, valueKind, contextSlice))

	declarationProvenance := recordMembershipEngineDeclarationProvenance(t)
	entitySet := mustRecordMembershipEngineValue(
		typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
			TypeEnv:         typeEnv,
			Context:         fixture.context,
			EnumerationRule: fixture.enumeration,

			CandidatePolicy: typedmemory.PersistedEntitiesOnly{},
			Provenance:      declarationProvenance,
		}))

	evaluatorRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	signature := mustRecordMembershipEngineValue(
		typedmemory.NewKindSignatureDefinition(
			typedmemory.KindSignatureDefinitionInput{
				ValueKind:       valueKind,
				Formality:       typedmemory.SignatureF4,
				DefinednessRule: fixture.definedness,

				Evaluator:  evaluatorRule,
				EntitySet:  entitySet.Ref(),
				Provenance: declarationProvenance,
			},
		))

	view := mustRecordMembershipEngineValue(
		typedmemory.NewPersistedSnapshotView(typeEnv, typedmemory.NewGraphRevision(17)))

	provenance := mustRecordMembershipEngineValue(
		recordMembershipEvaluationProvenance(fixture.policy.Evaluator()))

	return mustRecordMembershipEngineValue(
		recordcarrier.NewRecordMembershipEvaluationRequestV1(
			recordcarrier.RecordMembershipEvaluationInputV1{
				ProjectID:               fixture.project,
				Query:                   query,
				EvaluationView:          view,
				KindSignature:           signature,
				EntitySet:               entitySet,
				EvaluationProvenance:    provenance,
				ExpectedMappingManifest: manifest,
				ExpectedAdapterVersion:  adapter,
				SourceDelivery:          delivery,
			},
		))
}

func recordMembershipEngineCatalog(
	t *testing.T,
	rule typedmemory.RuleRef,
	enumerationRule typedmemory.RuleRef,
	visibilityRule typedmemory.RuleRef,
	definednessRule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismArtifactV1 {
	t.Helper()
	entries := []runtimemechanism.RuntimeMechanismEntryV1{
		mustRecordMembershipEngineValue(
			runtimemechanism.NewEntitySetEnumerationEntry(enumerationRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewCandidateVisibilityEntry(visibilityRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewKindDefinednessEntry(definednessRule)),
		mustRecordMembershipEngineValue(runtimemechanism.NewMemberOfEntry(rule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)),
	}
	artifact := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierRef("artifact:record-membership-runtime/v1"))

	edition := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierEdition("build-20260717.1"))

	return mustRecordMembershipEngineValue(
		runtimemechanism.SealRuntimeMechanismArtifactV1(artifact, edition, entries))
}

func recordMembershipEnginePolicy(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	identity := catalog.Identity()
	evaluator := recordMembershipEngineCoordinate(
		t,
		recordmembershipregistration.EvaluatorMechanism,
		rule,
		identity,
	)
	delivery := recordMembershipEngineCoordinate(
		t,
		recordmembershipregistration.SourceDeliveryBoundaryMechanism,
		rule,
		identity,
	)
	mapping := mustRecordMembershipEngineValue(
		recordmembershipregistration.NewAcceptedMapping(
			recordmembershipregistration.AcceptedMappingInput{
				Manifest: manifest,
				Adapter:  adapter,
			},
		))

	return mustRecordMembershipEngineValue(
		recordmembershipregistration.SealRegistrationArtifactV1(
			recordmembershipregistration.RegistrationArtifactInputV1{
				Evaluator:      evaluator,
				SourceDelivery: delivery,
				Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
			},
		))
}

func recordMembershipEngineCoordinate(
	t *testing.T,
	role recordmembershipregistration.MechanismRole,
	rule typedmemory.RuleRef,
	identity runtimemechanism.RuntimeMechanismArtifactIdentityV1,
) recordmembershipregistration.MechanismCoordinate {
	t.Helper()
	return mustRecordMembershipEngineValue(
		recordmembershipregistration.NewMechanismCoordinate(
			recordmembershipregistration.MechanismCoordinateInput{
				Role:     role,
				Rule:     rule,
				Artifact: identity.Artifact(),
				Edition:  identity.Edition(),
				Digest:   identity.Digest(),
			},
		))
}

func recordMembershipEngineRuntimeBasis(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	policy recordmembershipregistration.RegistrationArtifactV1,
	rule typedmemory.RuleRef,
	enumerationRule typedmemory.RuleRef,
	visibilityRule typedmemory.RuleRef,
	definednessRule typedmemory.RuleRef,
) projecttypeenv.RuntimeEvaluationBasisArtifact {
	t.Helper()
	mechanism := mustRecordMembershipEngineValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))

	evaluator := mustRecordMembershipEngineValue(
		projecttypeenv.NewEvaluatorRuntimeMechanismPin(
			projecttypeenv.EvaluatorRuntimeMechanismPinInput{
				Rule:             rule,
				Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		))

	delivery := mustRecordMembershipEngineValue(
		projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		))
	enumeration := recordMembershipEngineEvaluatorPin(
		t,
		enumerationRule,
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
		mechanism,
		catalog,
	)
	visibility := recordMembershipEngineEvaluatorPin(
		t,
		visibilityRule,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility,
		mechanism,
		catalog,
	)
	definedness := recordMembershipEngineEvaluatorPin(
		t,
		definednessRule,
		projecttypeenv.RuntimeMechanismContractKindDefinedness,
		mechanism,
		catalog,
	)

	policyPin := mustRecordMembershipEngineValue(
		projecttypeenv.NewRegistrationPolicyPin(policy))

	return mustRecordMembershipEngineValue(
		projecttypeenv.SealRuntimeEvaluationBasisWithPins(
			[]projecttypeenv.RuntimeEvaluationBasisPin{
				enumeration,
				visibility,
				definedness,
				evaluator,
				delivery,
				policyPin,
			},
			[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
			nil,
		))
}

func recordMembershipEngineEvaluatorPin(
	t *testing.T,
	rule typedmemory.RuleRef,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
) projecttypeenv.EvaluatorRuntimeMechanismPin {
	t.Helper()
	return mustRecordMembershipEngineValue(
		projecttypeenv.NewEvaluatorRuntimeMechanismPin(
			projecttypeenv.EvaluatorRuntimeMechanismPinInput{
				Rule:             rule,
				Contract:         contract,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		))
}

func recordMembershipEngineRuntimeWithoutPolicy(
	t *testing.T,
	fixture recordMembershipAdmissionEngineFixture,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	codec := recordMembershipEngineCodecRef(t)
	entry := mustRecordMembershipEngineValue(
		runtimemechanism.NewCodecCanonicalizationEntry(codec))

	artifact := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierRef("artifact:codec-only-record-runtime/v1"))

	edition := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierEdition("1.0.0"))

	catalog := mustRecordMembershipEngineValue(
		runtimemechanism.SealRuntimeMechanismArtifactV1(
			artifact,
			edition,
			[]runtimemechanism.RuntimeMechanismEntryV1{entry},
		))

	mechanism := mustRecordMembershipEngineValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))

	pin := mustRecordMembershipEngineValue(
		projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec:            codec,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		))

	basis := mustRecordMembershipEngineValue(
		projecttypeenv.SealRuntimeEvaluationBasis(
			[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
			catalog,
		))

	codecs := mustRecordMembershipEngineValue(
		typedmemory.NewCodecRegistry().Register(codec, recordMembershipEngineInertCodec{}))

	evaluators, ok := fixture.runtime.MemberOfRegistry()
	if !ok {
		t.Fatal("fixture runtime did not expose its evaluator registry")
	}
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             codecs,
				MemberOfEvaluators: evaluators,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("codec-only ObserveCurrentTargetRuntime() = %s, want matched", resolution.Kind().String())
	}
	runtime, ok := matched.Registry()
	if !ok {
		t.Fatal("codec-only matched runtime did not expose its exact registry")
	}
	return runtime
}

type recordMembershipEngineInertCodec struct{}

func (recordMembershipEngineInertCodec) Canonicalize(
	typedmemory.ValueShapeRef,
	[]byte,
) typedmemory.CodecCanonicalization {
	return typedmemory.RejectedCodecValue{}
}

func recordMembershipEngineCodecRef(t *testing.T) typedmemory.CodecRef {
	t.Helper()
	id := mustRecordMembershipEngineValue(
		typedmemory.NewCodecID("Haft.Codec.RecordMembershipEngineTestV1"))

	version := mustRecordMembershipEngineValue(
		typedmemory.NewCanonicalizationVersion("v1"))

	return mustRecordMembershipEngineValue(
		typedmemory.NewCodecRef(id, version, recordMembershipEngineDigest(t, 0x22)))
}

func recordMembershipEngineMechanismIdentity(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	identity := catalog.Identity()
	return mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewMechanismIdentity(
			identity.Artifact(),
			identity.Edition(),
			identity.Digest(),
			typedmemoryevaluation.EvaluatorRole,
		))
}

func recordMembershipEngineManifest(
	t *testing.T,
	id string,
	fill byte,
) recordmapping.MappingManifestRef {
	t.Helper()
	return mustRecordMembershipEngineValue(
		recordmapping.NewMappingManifestRef(
			id,
			"1.0.0",
			recordMembershipEngineDigest(t, fill),
		))
}

func recordMembershipEngineDeclarationProvenance(
	t *testing.T,
) typedmemory.DeclarationProvenance {
	t.Helper()
	unit := mustRecordMembershipEngineValue(
		typedmemory.NewSourceUnitID("local-practice:record-membership-engine-test"))

	revision := mustRecordMembershipEngineValue(
		typedmemory.NewSourceRevision("record-membership-engine-test-v1"))

	lineRange := mustRecordMembershipEngineValue(
		typedmemory.NewSourceLineRange(1, 4))

	location := mustRecordMembershipEngineValue(
		typedmemory.NewUnpatternedSourceLocation(
			unit,
			revision,
			recordMembershipEngineDigest(t, 0x82),
			lineRange,
		))

	reference := mustRecordMembershipEngineValue(
		typedmemory.NewProvenanceRef("prov:record-membership-engine-test/v1"))

	rule := mustRecordMembershipEngineValue(
		typedmemory.NewCompilerRuleID("record-membership-engine-test-v1"))

	return mustRecordMembershipEngineValue(
		typedmemory.NewFPFSourceProvenance(reference, location, rule))
}

func recordMembershipEngineDigest(
	t *testing.T,
	fill byte,
) typedmemory.SHA256Digest {
	t.Helper()
	const digits = "0123456789abcdef"
	raw := make([]byte, len("sha256:")+64)
	copy(raw, "sha256:")
	for index := len("sha256:"); index < len(raw); index++ {
		raw[index] = digits[int(fill+byte(index))%len(digits)]
	}
	return mustRecordMembershipEngineValue(
		typedmemory.NewSHA256Digest(string(raw)))
}

func mustRecordMembershipEngineValue[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic(err)
	}
	return value
}
