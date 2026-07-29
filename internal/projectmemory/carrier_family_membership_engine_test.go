package projectmemory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestCarrierFamilyBuildersExposeFourExactRealRegistrations(t *testing.T) {
	identity := carrierFamilyTestIdentity(t)
	enumeration := carrierFamilyTestEnumeration(t, identity)
	visibility := carrierFamilyTestVisibility(t, identity)
	definedness := carrierFamilyTestDefinedness(t, identity)
	builders := []struct {
		rule    typedmemory.RuleRef
		builder CarrierFamilyMembershipAdmissionEngineBuilder
	}{
		{rule: carrierfamily.CarrierEditionEvaluatorRuleV1(), builder: NewCarrierEditionMembershipAdmissionEngineBuilder()},
		{rule: carrierfamily.ProjectClaimEvaluatorRuleV1(), builder: NewProjectClaimMembershipAdmissionEngineBuilder()},
		{rule: carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1(), builder: NewPerformedWorkOccurrenceMembershipAdmissionEngineBuilder()},
		{rule: carrierfamily.CodeAnchorEvaluatorRuleV1(), builder: NewCodeAnchorMembershipAdmissionEngineBuilder()},
	}
	engines := make([]CarrierFamilyMembershipAdmissionEngine, 0, len(builders))
	for _, fixture := range builders {
		policy := carrierFamilyTestPolicy(t, fixture.rule, identity)
		engine, err := fixture.builder.
			SetEntitySetEnumeration(enumeration).
			SetCandidateVisibility(visibility).
			SetKindDefinedness(definedness).
			SetMechanismIdentity(identity).
			SetRegistrationPolicy(policy).
			Build()
		if err != nil {
			t.Fatalf("Build(%s): %v", fixture.rule.String(), err)
		}
		engines = append(engines, engine)
	}
	registry, err := NewCarrierFamilyMembershipEvaluatorRegistry(engines)
	if err != nil {
		t.Fatalf("NewCarrierFamilyMembershipEvaluatorRegistry: %v", err)
	}
	if registry.Len() != 4 {
		t.Fatalf("registry.Len() = %d, want 4", registry.Len())
	}
	for _, fixture := range builders {
		lookup, err := registry.Lookup(fixture.rule, identity)
		if err != nil {
			t.Fatal(err)
		}
		if lookup.Kind().String() != "found" {
			t.Fatalf("Lookup(%s) = %s, want found", fixture.rule.String(), lookup.Kind().String())
		}
	}
}

func TestCarrierFamilyBuilderRejectsPolicyForAnotherFamily(t *testing.T) {
	identity := carrierFamilyTestIdentity(t)
	engine := NewProjectClaimMembershipAdmissionEngineBuilder().
		SetEntitySetEnumeration(carrierFamilyTestEnumeration(t, identity)).
		SetCandidateVisibility(carrierFamilyTestVisibility(t, identity)).
		SetKindDefinedness(carrierFamilyTestDefinedness(t, identity)).
		SetMechanismIdentity(identity).
		SetRegistrationPolicy(carrierFamilyTestPolicy(
			t,
			carrierfamily.CodeAnchorEvaluatorRuleV1(),
			identity,
		))
	if _, err := engine.Build(); err == nil {
		t.Fatal("project-claim builder accepted a code-anchor registration policy")
	}
}

func TestCarrierFamilySourceSelectionKeepsAbsenceDistinctFromSourceFailure(
	t *testing.T,
) {
	project := mustCarrierFamilySelectionValue(
		projectidentity.ParseProjectID("qnt_deadbeef"),
	)
	entity := mustCarrierFamilySelectionValue(
		typedmemory.NewEntityID("entity:source-selection"),
	)
	contextRef := mustCarrierFamilySelectionValue(
		typedmemory.NewBoundedContextRef("haft-project"),
	)
	manifest := mustCarrierFamilySelectionValue(
		carrierfamily.CurrentProjectClaimMappingManifestV1(),
	)
	firstSource := carrierFamilySelectionSource(
		t,
		project,
		entity,
		contextRef,
		"first",
		manifest.AdapterVersion(),
	)
	secondSource := carrierFamilySelectionSource(
		t,
		project,
		entity,
		contextRef,
		"second",
		manifest.AdapterVersion(),
	)
	untrustedAdapter := mustCarrierFamilySelectionValue(
		recordmapping.NewAdapterVersion("claim-adapter/9.0.0"),
	)
	untrustedSource := carrierFamilySelectionSource(
		t,
		project,
		entity,
		contextRef,
		"untrusted",
		untrustedAdapter,
	)
	otherEntity := mustCarrierFamilySelectionValue(
		typedmemory.NewEntityID("entity:other-source-selection"),
	)
	unrelatedSource := carrierFamilySelectionSource(
		t,
		project,
		otherEntity,
		contextRef,
		"unrelated",
		manifest.AdapterVersion(),
	)
	firstBlob := carrierFamilySelectionBlob(t, firstSource)
	secondBlob := carrierFamilySelectionBlob(t, secondSource)
	untrustedBlob := carrierFamilySelectionBlob(t, untrustedSource)
	unrelatedBlob := carrierFamilySelectionBlob(t, unrelatedSource)
	malformedBlob := carrierFamilyMalformedSelectionBlob(t, firstSource)
	input := carrierFamilySelectionInput(
		t,
		project,
		entity,
		contextRef,
		[]memberofevaluation.ObservableInputBlob{
			firstBlob,
			secondBlob,
			untrustedBlob,
			malformedBlob,
		},
	)
	identity := carrierFamilyTestIdentity(t)
	engine := CarrierFamilyMembershipAdmissionEngine{
		rule: carrierfamily.ProjectClaimEvaluatorRuleV1(),
		policy: carrierFamilyTestPolicy(
			t,
			carrierfamily.ProjectClaimEvaluatorRuleV1(),
			identity,
		),
	}
	snapshotEngine, err := NewProjectClaimMembershipAdmissionEngineBuilder().
		SetEntitySetEnumeration(carrierFamilyTestEnumeration(t, identity)).
		SetCandidateVisibility(carrierFamilyTestVisibility(t, identity)).
		SetKindDefinedness(carrierFamilyTestDefinedness(t, identity)).
		SetMechanismIdentity(identity).
		SetRegistrationPolicy(carrierFamilyTestPolicy(
			t,
			carrierfamily.ProjectClaimEvaluatorRuleV1(),
			identity,
		)).
		Build()
	if err != nil {
		t.Fatalf("build snapshot source-selection engine: %v", err)
	}
	noSourceInput := carrierFamilySelectionInput(
		t,
		project,
		entity,
		contextRef,
		nil,
	)
	noSourceSelection := snapshotEngine.SelectSnapshotObservableInputs(noSourceInput)
	if _, ok := noSourceSelection.(memberofevaluation.SnapshotObservableInputsNotApplicable); !ok {
		t.Fatalf("clean zero-source snapshot selection = %T; want NotApplicable", noSourceSelection)
	}
	unrelatedInput := carrierFamilySelectionInput(
		t,
		project,
		entity,
		contextRef,
		[]memberofevaluation.ObservableInputBlob{unrelatedBlob},
	)
	unrelatedSelection := snapshotEngine.SelectSnapshotObservableInputs(unrelatedInput)
	if _, ok := unrelatedSelection.(memberofevaluation.SnapshotObservableInputsNotApplicable); !ok {
		t.Fatalf("unrelated snapshot selection = %T; want NotApplicable", unrelatedSelection)
	}
	for name, blobs := range map[string][]memberofevaluation.ObservableInputBlob{
		"malformed": {malformedBlob},
		"untrusted": {untrustedBlob},
		"ambiguous": {firstBlob, secondBlob},
	} {
		problemInput := carrierFamilySelectionInput(
			t,
			project,
			entity,
			contextRef,
			blobs,
		)
		problemSelection := snapshotEngine.SelectSnapshotObservableInputs(problemInput)
		if _, ok := problemSelection.(memberofevaluation.SnapshotObservableInputsUnavailable); !ok {
			t.Fatalf("%s snapshot selection = %T; want Unavailable", name, problemSelection)
		}
	}

	noSource := engine.selectTrustedSource(input, nil)
	if _, ok := noSource.(carrierFamilySourceNotApplicable); !ok {
		t.Fatalf("no-source selection = %T; want not applicable", noSource)
	}
	unrelated := engine.selectTrustedSource(
		input,
		[]memberofevaluation.ObservableInputBlob{unrelatedBlob},
	)
	if _, ok := unrelated.(carrierFamilySourceNotApplicable); !ok {
		t.Fatalf("unrelated-source selection = %T; want not applicable", unrelated)
	}
	noSourceJudgement, err := carrierFamilyUndefinedForNoApplicableSource(
		input.Request(),
	)
	if err != nil {
		t.Fatalf("carrierFamilyUndefinedForNoApplicableSource() error = %v", err)
	}
	noSourceUndefined, ok := noSourceJudgement.(typedmemory.MemberOfUndefined)
	if !ok || !noSourceUndefined.IsNoApplicableObservableSource() {
		t.Fatal("no-source selection did not produce exact no-applicable Undefined")
	}

	tests := []struct {
		name    string
		blobs   []memberofevaluation.ObservableInputBlob
		problem carrierFamilySourceProblemKind
	}{
		{
			name:    "malformed",
			blobs:   []memberofevaluation.ObservableInputBlob{malformedBlob},
			problem: carrierFamilySourceMalformed,
		},
		{
			name:    "untrusted",
			blobs:   []memberofevaluation.ObservableInputBlob{untrustedBlob},
			problem: carrierFamilySourceUntrusted,
		},
		{
			name: "ambiguous",
			blobs: []memberofevaluation.ObservableInputBlob{
				firstBlob,
				secondBlob,
			},
			problem: carrierFamilySourceAmbiguous,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := engine.selectTrustedSource(input, test.blobs)
			underdetermined, ok := selection.(carrierFamilySourceUnderdetermined)
			if !ok {
				t.Fatalf("selection = %T; want underdetermined", selection)
			}
			if underdetermined.problem != test.problem {
				t.Fatalf(
					"selection problem = %d; want %d",
					underdetermined.problem,
					test.problem,
				)
			}
			judgement, err := carrierFamilyUndefinedForUnusableSource(
				input.Request(),
			)
			if err != nil {
				t.Fatalf("carrierFamilyUndefinedForUnusableSource() error = %v", err)
			}
			undefined, ok := judgement.(typedmemory.MemberOfUndefined)
			if !ok {
				t.Fatalf("unusable-source judgement = %T; want Undefined", judgement)
			}
			if undefined.IsNoApplicableObservableSource() {
				t.Fatal("source failure became no-applicable-source")
			}
			missing := undefined.MissingBasis()
			if len(missing) != 1 ||
				missing[0].Kind() != typedmemory.MissingMemberOfUniqueTrustedObservableSource {
				t.Fatalf("unusable-source missing basis = %#v", missing)
			}
		})
	}

	selected := engine.selectTrustedSource(
		input,
		[]memberofevaluation.ObservableInputBlob{firstBlob, firstBlob},
	)
	if _, ok := selected.(carrierFamilySourceSelected); !ok {
		t.Fatalf("duplicate exact source selection = %T; want selected", selected)
	}
}

func carrierFamilySelectionInput(
	t *testing.T,
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	blobs []memberofevaluation.ObservableInputBlob,
) memberofevaluation.MemberOfEvaluationInput {
	t.Helper()
	typeEnv := mustCarrierFamilySelectionValue(
		typedmemory.NewTypeEnvRef(carrierFamilySelectionDigest(t, []byte("typeenv"))),
	)
	kindID := mustCarrierFamilySelectionValue(
		typedmemory.NewKindID("Haft.ProjectClaim"),
	)
	valueKind := mustCarrierFamilySelectionValue(
		typedmemory.NewValueKindRef(typeEnv, kindID),
	)
	gamma := mustCarrierFamilySelectionValue(
		typedmemory.NewGammaPoint(
			time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC),
		),
	)
	contextSlice := mustCarrierFamilySelectionValue(
		typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		}),
	)
	query := mustCarrierFamilySelectionValue(
		typedmemory.NewMemberOfQuery(entity, valueKind, contextSlice),
	)
	view := mustCarrierFamilySelectionValue(
		typedmemory.NewPersistedSnapshotView(
			typeEnv,
			typedmemory.NewGraphRevision(17),
		),
	)
	request := mustCarrierFamilySelectionValue(
		typedmemory.NewMemberOfEvaluationRequest(query, view),
	)
	return mustCarrierFamilySelectionValue(
		memberofevaluation.NewMemberOfEvaluationInput(
			project,
			typedmemory.TypeEnv{},
			request,
			blobs,
			memberofevaluation.NewPersistedEntityUniverseUnavailable(),
		),
	)
}

func carrierFamilySelectionSource(
	t *testing.T,
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	value string,
	adapter recordmapping.AdapterVersion,
) carrierfamily.MembershipSourceV1 {
	t.Helper()
	canonical := []byte(`{"schema":"claim/v1","value":"` + value + `"}`)
	digest := carrierFamilySelectionDigest(t, canonical)
	payloadRef := mustCarrierFamilySelectionValue(
		typedmemory.NewCarrierRef("payload:project-claim:" + digest.String()),
	)
	edition := mustCarrierFamilySelectionValue(
		typedmemory.NewCarrierEdition("1.0.0"),
	)
	payload := mustCarrierFamilySelectionValue(
		carrierfamily.NewSourcePayloadV1(
			payloadRef,
			edition,
			digest,
			"haft.project-claim-payload/v1",
			canonical,
		),
	)
	carrier := mustCarrierFamilySelectionValue(
		carrierfamily.SealProjectClaimCarrierV1(entity, contextRef, payload),
	)
	manifest := mustCarrierFamilySelectionValue(
		carrierfamily.CurrentProjectClaimMappingManifestV1(),
	)
	binding := mustCarrierFamilySelectionValue(
		carrierfamily.SealEntityCarrierBindingV1(
			project,
			carrier,
			manifest.Ref(),
			adapter,
		),
	)
	return mustCarrierFamilySelectionValue(
		carrierfamily.SealMembershipSourceV1(
			project,
			entity,
			contextRef,
			carrier,
			binding,
		),
	)
}

func carrierFamilySelectionBlob(
	t *testing.T,
	source carrierfamily.MembershipSourceV1,
) memberofevaluation.ObservableInputBlob {
	t.Helper()
	input := source.ObservableInput()
	return mustCarrierFamilySelectionValue(
		memberofevaluation.NewObservableInputBlob(
			input.Reference(),
			input.Digest(),
			source.CanonicalBytes(),
		),
	)
}

func carrierFamilyMalformedSelectionBlob(
	t *testing.T,
	source carrierfamily.MembershipSourceV1,
) memberofevaluation.ObservableInputBlob {
	t.Helper()
	canonical := []byte(`{"schema_version":`)
	digest := carrierFamilySelectionDigest(t, canonical)
	return mustCarrierFamilySelectionValue(
		memberofevaluation.NewObservableInputBlob(
			source.ObservableInput().Reference(),
			digest,
			canonical,
		),
	)
}

func carrierFamilySelectionDigest(
	t *testing.T,
	canonical []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(canonical)
	encoded := hex.EncodeToString(sum[:])
	return mustCarrierFamilySelectionValue(
		typedmemory.NewSHA256Digest("sha256:" + encoded),
	)
}

func mustCarrierFamilySelectionValue[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic(err)
	}
	return value
}

func carrierFamilyTestIdentity(t *testing.T) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef("artifact:carrier-family-runtime-test")
	if err != nil {
		t.Fatal(err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := typedmemoryevaluation.NewMechanismIdentity(
		artifact,
		edition,
		digest,
		typedmemoryevaluation.EvaluatorRole,
	)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func carrierFamilyTestEnumeration(
	t *testing.T,
	identity typedmemoryevaluation.MechanismIdentity,
) typedmemoryevaluation.EntitySetEnumerationRegistry {
	t.Helper()
	rule, err := typedmemory.NewRuleRef("haft.entity-set.project-entities/v1")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := typedmemoryevaluation.NewEntitySetEnumerationRegistry(rule, identity)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func carrierFamilyTestVisibility(
	t *testing.T,
	identity typedmemoryevaluation.MechanismIdentity,
) typedmemoryevaluation.CandidateVisibilityRegistry {
	t.Helper()
	rule, err := typedmemory.NewRuleRef("haft.entity-set.project-entities.prior-batch-visible/v1")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := typedmemoryevaluation.NewCandidateVisibilityRegistry(rule, identity)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func carrierFamilyTestDefinedness(
	t *testing.T,
	identity typedmemoryevaluation.MechanismIdentity,
) typedmemoryevaluation.KindDefinednessRegistry {
	t.Helper()
	rules := []string{
		"haft.member-of.carrier-edition-carrier/v1/definedness",
		"haft.member-of.project-claim-carrier/v1/definedness",
		"haft.member-of.performed-work-occurrence-carrier/v1/definedness",
		"haft.member-of.code-anchor-carrier/v1/definedness",
	}
	registrations := make([]typedmemoryevaluation.Registration[
		struct{},
		struct{},
	], 0)
	_ = registrations
	result := typedmemoryevaluation.KindDefinednessRegistry{}
	for _, raw := range rules {
		rule, err := typedmemory.NewRuleRef(raw)
		if err != nil {
			t.Fatal(err)
		}
		one, err := typedmemoryevaluation.NewKindDefinednessRegistry(rule, identity)
		if err != nil {
			t.Fatal(err)
		}
		result, err = typedmemoryevaluation.NewRegistry(
			append(result.Registrations(), one.Registrations()...),
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func carrierFamilyTestPolicy(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	evaluator, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: identity.ArtifactRef(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: identity.ArtifactRef(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := carrierFamilyTestManifest(rule)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest.Ref(),
			Adapter:  manifest.AdapterVersion(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func carrierFamilyTestManifest(
	rule typedmemory.RuleRef,
) (carrierfamily.MappingManifestV1, error) {
	constructors := map[string]func() (carrierfamily.MappingManifestV1, error){
		carrierfamily.CarrierEditionEvaluatorRuleV1().String():          carrierfamily.CurrentCarrierEditionMappingManifestV1,
		carrierfamily.ProjectClaimEvaluatorRuleV1().String():            carrierfamily.CurrentProjectClaimMappingManifestV1,
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1().String(): carrierfamily.CurrentPerformedWorkOccurrenceMappingManifestV1,
		carrierfamily.CodeAnchorEvaluatorRuleV1().String():              carrierfamily.CurrentCodeAnchorMappingManifestV1,
	}
	constructor, found := constructors[rule.String()]
	if !found {
		return carrierfamily.MappingManifestV1{}, ErrCarrierFamilyMembershipRuntimeInvalid
	}
	return constructor()
}
