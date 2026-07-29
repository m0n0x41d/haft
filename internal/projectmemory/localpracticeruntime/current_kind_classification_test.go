package localpracticeruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestCurrentClassificationAdaptsSealedHistoricalRecordDeliveryWithoutUsingMembershipJudgement(
	t *testing.T,
) {
	t.Parallel()
	target := buildCurrentKindClassificationCandidate(t)
	environment, present := target.Preparation().Environment()
	if !present {
		t.Fatal("current candidate preparation has no executable TypeEnv")
	}
	runtime, present := target.ExactRuntimeRegistry()
	if !present {
		t.Fatal("current candidate has no exact runtime registry")
	}
	evaluators, present := runtime.KindClassificationRegistry()
	if !present {
		t.Fatal("current candidate has no KindClassification registry")
	}
	engine := mustCurrentClassificationValue(
		projectmemory.NewProjectKindClassificationAdmissionEngine(evaluators),
	)
	project := mustCurrentClassificationValue(
		projectidentity.ParseProjectID("qnt_deadbeef"),
	)
	entity := mustCurrentClassificationValue(
		typedmemory.NewEntityID("record:sealed-historical-decision"),
	)
	contextRef := mustCurrentClassificationValue(
		typedmemory.NewBoundedContextRef("haft-project"),
	)
	carrier := mustCurrentClassificationValue(
		recordcarrier.SealProjectRecordCarrierV1(
			entity,
			contextRef,
			recordcarrier.DecisionRecordVariantV1{},
		),
	)
	manifest := mustCurrentClassificationValue(
		decisionrecordadapter.CurrentMappingManifestV1(),
	)
	binding := mustCurrentClassificationValue(
		recordcarrier.SealEntityRecordCarrierBindingV1(
			project,
			carrier,
			manifest.Ref(),
			manifest.AdapterVersion(),
		),
	)
	historical := mustCurrentClassificationValue(
		recordcarrier.SealRecordMembershipSourceV1(
			project,
			entity,
			contextRef,
			carrier,
			binding,
		),
	)
	historicalBytes := historical.CanonicalBytes()
	observable := mustCurrentClassificationValue(
		typedmemorystore.NewObservableInputBlob(
			historical.ObservableInput().Reference(),
			historical.ObservableInput().Digest(),
			historicalBytes,
		),
	)
	adapted := mustCurrentClassificationValue(
		engine.AdaptSealedHistoricalKindClassificationSources(
			project,
			[]typedmemorystore.ObservableInputBlob{observable},
		),
	)
	if len(adapted) != 1 {
		t.Fatalf("adapted source count = %d, want 1", len(adapted))
	}
	expected := mustCurrentClassificationValue(
		recordcarrier.SealRecordClassificationSourceV1(
			project,
			entity,
			contextRef,
			carrier,
			binding,
		),
	)
	if adapted[0].Reference() != expected.Ref() ||
		adapted[0].Digest() != expected.Digest() ||
		!bytes.Equal(adapted[0].Bytes(), expected.CanonicalBytes()) {
		t.Fatal("adapted source differs from the exact current delivery carrier")
	}
	if !bytes.Equal(historical.CanonicalBytes(), historicalBytes) {
		t.Fatal("historical delivery bytes changed during adaptation")
	}
	visibility := mustCurrentClassificationValue(
		typedmemorystore.NewSnapshotKindClassificationVisibility(
			typedmemory.NewGraphRevision(7),
			entity,
			contextRef,
			mustCurrentClassificationValue(
				typedmemory.NewResolutionBasisRef("basis:sealed-historical-adapter-test"),
			),
		),
	)
	input := currentClassificationAdmissionInput(
		t,
		project,
		environment,
		target.InstalledRuntime().Codecs,
		entity,
		contextRef,
		"Haft.DecisionRecord",
		visibility,
		adapted,
	)
	judgement := mustCurrentClassificationValue(
		engine.EvaluateKindClassification(context.Background(), input),
	)
	if judgement.Kind() != typedmemory.KindClassificationTrue {
		t.Fatalf("adapted classification = %q, want true", judgement.Kind())
	}
	otherProject := mustCurrentClassificationValue(
		projectidentity.ParseProjectID("qnt_feedface"),
	)
	if _, err := engine.AdaptSealedHistoricalKindClassificationSources(
		otherProject,
		[]typedmemorystore.ObservableInputBlob{observable},
	); err == nil {
		t.Fatal("historical delivery adaptation accepted another project")
	}
}

func TestCurrentKindClassificationCandidateBuildsNoMemberOfRuntime(t *testing.T) {
	t.Parallel()
	base := loadCurrentBaseArtifact(t)
	target, err := Build(base, typedmemorycandidates.SourceV1_3())
	if err != nil {
		t.Fatalf("Build(current KindClassification candidate) error = %v", err)
	}
	classificationRequirements := 0
	for _, requirement := range target.Requirements().Requirements() {
		switch requirement.InvocationContract() {
		case projecttypeenv.RuntimeMechanismContractKindClassification:
			classificationRequirements++
		case projecttypeenv.RuntimeMechanismContractMemberOf,
			projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
			projecttypeenv.RuntimeMechanismContractCandidateVisibility,
			projecttypeenv.RuntimeMechanismContractKindDefinedness,
			projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:
			t.Fatalf(
				"current candidate leaked historical runtime contract %q",
				requirement.InvocationContract(),
			)
		}
	}
	if classificationRequirements != 12 {
		t.Fatalf(
			"KindClassification requirements = %d, want 12",
			classificationRequirements,
		)
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		t.Fatal("current candidate has no exact target runtime registry")
	}
	classification, present := registry.KindClassificationRegistry()
	if !present || classification.Len() != 12 {
		t.Fatalf(
			"classification registry presence/size = %v/%d, want true/12",
			present,
			classification.Len(),
		)
	}
	memberOf, present := registry.MemberOfRegistry()
	if !present || memberOf.Len() != 0 {
		t.Fatalf(
			"historical MemberOf registry presence/size = %v/%d, want true/0",
			present,
			memberOf.Len(),
		)
	}
	if len(target.RegistrationPolicies()) != 0 {
		t.Fatalf(
			"current candidate emitted %d historical registration policies",
			len(target.RegistrationPolicies()),
		)
	}
}

func TestCurrentKindClassificationRuntimePreservesTrueFalseAndUnknown(
	t *testing.T,
) {
	t.Parallel()
	target := buildCurrentKindClassificationCandidate(t)
	environment, present := target.Preparation().Environment()
	if !present {
		t.Fatal("current candidate preparation has no executable TypeEnv")
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		t.Fatal("current candidate has no exact runtime registry")
	}
	evaluators, present := registry.KindClassificationRegistry()
	if !present {
		t.Fatal("current candidate has no KindClassification registry")
	}
	engine := mustCurrentClassificationValue(
		projectmemory.NewRecordKindClassificationAdmissionEngine(evaluators),
	)
	project := mustCurrentClassificationValue(
		projectidentity.ParseProjectID("qnt_deadbeef"),
	)
	entity := mustCurrentClassificationValue(
		typedmemory.NewEntityID("entity:decision-record-1"),
	)
	contextRef := mustCurrentClassificationValue(
		typedmemory.NewBoundedContextRef("haft-project"),
	)
	carrier := mustCurrentClassificationValue(
		recordcarrier.SealProjectRecordCarrierV1(
			entity,
			contextRef,
			recordcarrier.DecisionRecordVariantV1{},
		),
	)
	manifest := mustCurrentClassificationValue(
		decisionrecordadapter.CurrentMappingManifestV1(),
	)
	binding := mustCurrentClassificationValue(
		recordcarrier.SealEntityRecordCarrierBindingV1(
			project,
			carrier,
			manifest.Ref(),
			manifest.AdapterVersion(),
		),
	)
	source := mustCurrentClassificationValue(
		recordcarrier.SealRecordClassificationSourceV1(
			project,
			entity,
			contextRef,
			carrier,
			binding,
		),
	)
	blob := mustCurrentClassificationValue(
		typedmemorystore.NewKindClassificationSourceBlob(
			source.Ref(),
			source.Digest(),
			source.CanonicalBytes(),
		),
	)
	visibility := mustCurrentClassificationValue(
		typedmemorystore.NewSnapshotKindClassificationVisibility(
			typedmemory.NewGraphRevision(7),
			entity,
			contextRef,
			mustCurrentClassificationValue(
				typedmemory.NewResolutionBasisRef("basis:current-classification-test"),
			),
		),
	)
	codecs := target.InstalledRuntime().Codecs

	trueInput := currentClassificationAdmissionInput(
		t,
		project,
		environment,
		codecs,
		entity,
		contextRef,
		"Haft.DecisionRecord",
		visibility,
		[]typedmemorystore.KindClassificationSourceBlob{blob},
	)
	trueJudgement := mustCurrentClassificationValue(
		engine.EvaluateKindClassification(context.Background(), trueInput),
	)
	if trueJudgement.Kind() != typedmemory.KindClassificationTrue {
		t.Fatalf("DecisionRecord judgement = %q, want true", trueJudgement.Kind())
	}

	falseInput := currentClassificationAdmissionInput(
		t,
		project,
		environment,
		codecs,
		entity,
		contextRef,
		"Haft.ProjectClaim",
		visibility,
		[]typedmemorystore.KindClassificationSourceBlob{blob},
	)
	falseJudgement := mustCurrentClassificationValue(
		engine.EvaluateKindClassification(context.Background(), falseInput),
	)
	if falseJudgement.Kind() != typedmemory.KindClassificationFalse {
		t.Fatalf("ProjectClaim judgement = %q, want false", falseJudgement.Kind())
	}

	unknownInput := currentClassificationAdmissionInput(
		t,
		project,
		environment,
		codecs,
		entity,
		contextRef,
		"Haft.DecisionRecord",
		visibility,
		nil,
	)
	unknownJudgement := mustCurrentClassificationValue(
		engine.EvaluateKindClassification(context.Background(), unknownInput),
	)
	if unknownJudgement.Kind() != typedmemory.KindClassificationUnknown {
		t.Fatalf("missing-source judgement = %q, want unknown", unknownJudgement.Kind())
	}

	entityInput := currentClassificationAdmissionInput(
		t,
		project,
		environment,
		codecs,
		entity,
		contextRef,
		"U.Entity",
		visibility,
		nil,
	)
	entityJudgement := mustCurrentClassificationValue(
		engine.EvaluateKindClassification(context.Background(), entityInput),
	)
	if entityJudgement.Kind() != typedmemory.KindClassificationTrue {
		t.Fatalf("visible U.Entity judgement = %q, want true", entityJudgement.Kind())
	}
}

func TestCurrentKindClassificationRuntimeCoversAllCarrierFamilies(
	t *testing.T,
) {
	t.Parallel()
	target := buildCurrentKindClassificationCandidate(t)
	environment, present := target.Preparation().Environment()
	if !present {
		t.Fatal("current candidate preparation has no executable TypeEnv")
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		t.Fatal("current candidate has no exact runtime registry")
	}
	evaluators, present := registry.KindClassificationRegistry()
	if !present {
		t.Fatal("current candidate has no KindClassification registry")
	}
	engine := mustCurrentClassificationValue(
		projectmemory.NewProjectKindClassificationAdmissionEngine(evaluators),
	)
	project := mustCurrentClassificationValue(
		projectidentity.ParseProjectID("qnt_deadbeef"),
	)
	contextRef := mustCurrentClassificationValue(
		typedmemory.NewBoundedContextRef("haft-project"),
	)
	visibilityBasis := mustCurrentClassificationValue(
		typedmemory.NewResolutionBasisRef("basis:carrier-family-classification-test"),
	)
	codecs := target.InstalledRuntime().Codecs
	tests := []struct {
		name      string
		localKind string
		seal      func(
			typedmemory.EntityID,
			typedmemory.BoundedContextRef,
			carrierfamily.SourcePayloadV1,
		) (carrierfamily.CarrierV1, error)
		manifest func() (carrierfamily.MappingManifestV1, error)
	}{
		{
			name:      "carrier edition",
			localKind: "Haft.CarrierEdition",
			seal:      carrierfamily.SealCarrierEditionCarrierV1,
			manifest:  carrierfamily.CurrentCarrierEditionMappingManifestV1,
		},
		{
			name:      "project claim",
			localKind: "Haft.ProjectClaim",
			seal:      carrierfamily.SealProjectClaimCarrierV1,
			manifest:  carrierfamily.CurrentProjectClaimMappingManifestV1,
		},
		{
			name:      "performed work occurrence",
			localKind: "Haft.PerformedWorkOccurrence",
			seal:      carrierfamily.SealPerformedWorkOccurrenceCarrierV1,
			manifest:  carrierfamily.CurrentPerformedWorkOccurrenceMappingManifestV1,
		},
		{
			name:      "code anchor",
			localKind: "Haft.CodeAnchor",
			seal:      carrierfamily.SealCodeAnchorCarrierV1,
			manifest:  carrierfamily.CurrentCodeAnchorMappingManifestV1,
		},
	}
	for index, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			entity := mustCurrentClassificationValue(
				typedmemory.NewEntityID(fmt.Sprintf("entity:carrier-family-%d", index)),
			)
			payload := currentCarrierFamilyClassificationPayload(t, index)
			carrier := mustCurrentClassificationValue(
				testCase.seal(entity, contextRef, payload),
			)
			manifest := mustCurrentClassificationValue(testCase.manifest())
			binding := mustCurrentClassificationValue(
				carrierfamily.SealEntityCarrierBindingV1(
					project,
					carrier,
					manifest.Ref(),
					manifest.AdapterVersion(),
				),
			)
			source := mustCurrentClassificationValue(
				carrierfamily.SealClassificationSourceV1(
					project,
					entity,
					contextRef,
					carrier,
					binding,
				),
			)
			blob := mustCurrentClassificationValue(
				typedmemorystore.NewKindClassificationSourceBlob(
					source.Ref(),
					source.Digest(),
					source.CanonicalBytes(),
				),
			)
			visibility := mustCurrentClassificationValue(
				typedmemorystore.NewSnapshotKindClassificationVisibility(
					typedmemory.NewGraphRevision(7),
					entity,
					contextRef,
					visibilityBasis,
				),
			)
			input := currentClassificationAdmissionInput(
				t,
				project,
				environment,
				codecs,
				entity,
				contextRef,
				testCase.localKind,
				visibility,
				[]typedmemorystore.KindClassificationSourceBlob{blob},
			)
			sources := input.Sources()
			if len(sources) != 1 {
				t.Fatalf(
					"classification source count = %d, want 1",
					len(sources),
				)
			}
			if !carrierfamily.IsClassificationSourceReference(
				sources[0].Reference(),
			) || source.EntityID() != entity {
				t.Fatalf(
					"classification source correlation = ref %q source entity %q candidate %q",
					sources[0].Reference().String(),
					source.EntityID().String(),
					entity.String(),
				)
			}
			judgement := mustCurrentClassificationValue(
				engine.EvaluateKindClassification(context.Background(), input),
			)
			if judgement.Kind() != typedmemory.KindClassificationTrue {
				unknown, _ := judgement.(typedmemory.UnknownKindClassification)
				t.Fatalf(
					"%s judgement = %q (%v), want true",
					testCase.localKind,
					judgement.Kind(),
					unknown.Reasons(),
				)
			}
		})
	}
}

func currentCarrierFamilyClassificationPayload(
	t *testing.T,
	ordinal int,
) carrierfamily.SourcePayloadV1 {
	t.Helper()
	canonical := []byte(fmt.Sprintf(`{"ordinal":%d,"value":"current"}`, ordinal))
	sum := sha256.Sum256(canonical)
	digest := mustCurrentClassificationValue(
		typedmemory.NewSHA256Digest(fmt.Sprintf("sha256:%x", sum)),
	)
	reference := mustCurrentClassificationValue(
		typedmemory.NewCarrierRef(
			fmt.Sprintf("carrier-family-payload:current-%d", ordinal),
		),
	)
	edition := mustCurrentClassificationValue(
		typedmemory.NewCarrierEdition("1.0.0"),
	)
	return mustCurrentClassificationValue(
		carrierfamily.NewSourcePayloadV1(
			reference,
			edition,
			digest,
			"haft.current-classification-payload/v1",
			canonical,
		),
	)
}

func buildCurrentKindClassificationCandidate(t *testing.T) Target {
	t.Helper()
	base := loadCurrentBaseArtifact(t)
	target, err := Build(base, typedmemorycandidates.SourceV1_3())
	if err != nil {
		t.Fatalf("Build(current KindClassification candidate) error = %v", err)
	}
	return target
}

func currentClassificationAdmissionInput(
	t *testing.T,
	project projectidentity.ProjectID,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	localKindRaw string,
	visibility typedmemorystore.KindClassificationVisibility,
	sources []typedmemorystore.KindClassificationSourceBlob,
) typedmemorystore.KindClassificationAdmissionInput {
	t.Helper()
	localKindID := mustCurrentClassificationValue(
		typedmemory.NewKindID(localKindRaw),
	)
	localValueKind := mustCurrentClassificationValue(
		typedmemory.NewValueKindRef(environment.Ref(), localKindID),
	)
	localKind := mustCurrentClassificationValue(
		typedmemory.NewLocalKindRef(localValueKind, contextRef),
	)
	signature, found := environment.KindClassificationSignatureDefinition(localKind)
	if !found {
		t.Fatalf("current TypeEnv has no KindSignature for %s", localKindRaw)
	}
	entityKind := mustCurrentClassificationValue(
		typedmemory.NewKindID("U.Entity"),
	)
	entityValueKind := mustCurrentClassificationValue(
		typedmemory.NewValueKindRef(environment.Ref(), entityKind),
	)
	candidate := mustCurrentClassificationValue(
		typedmemory.NewExactKindEntityCandidate(entity, entityValueKind),
	)
	gamma := mustCurrentClassificationValue(
		typedmemory.NewGammaPoint(
			time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
		),
	)
	slice := mustCurrentClassificationValue(
		typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		}),
	)
	request := mustCurrentClassificationValue(
		typedmemory.NewKindClassificationRequest(
			typedmemory.KindClassificationRequestInput{
				Candidate:        candidate,
				LocalKind:        localKind,
				SignatureEdition: signature.Ref(),
				ContextSlice:     slice,
			},
		),
	)
	return mustCurrentClassificationValue(
		typedmemorystore.NewKindClassificationAdmissionInput(
			project,
			environment,
			codecs,
			request,
			visibility,
			sources,
		),
	)
}

func mustCurrentClassificationValue[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic("construct current classification fixture: " + err.Error())
	}
	return value
}
