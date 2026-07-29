package projecttypeenvselectionauthority

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
)

func TestProjectTypeEnvAuthorityCoreSealsExactOccurrenceAndStrictResolution(t *testing.T) {
	fixture := buildAuthorityFixture(t)

	if _, err := (ProjectTypeEnvHeadSelectionAction(0)).AuthorityActionKind(); err == nil {
		t.Fatal("zero head-selection action must fail closed")
	}
	if fixture.content.Action() != ProjectTypeEnvHeadSelectionTransition {
		t.Fatal("transition fixture projected the wrong closed action")
	}
	if !bytes.Contains(
		fixture.policy.CanonicalJSON(),
		[]byte(`"admitted_action":"transition"`),
	) || bytes.Contains(
		fixture.policy.CanonicalJSON(),
		[]byte(`"admitted_actions"`),
	) {
		t.Fatal("resolver policy does not canonically bind exactly one action")
	}
	if !bytes.Contains(
		fixture.content.CanonicalJSON(),
		[]byte(`"profile_posture":"underdetermined"`),
	) {
		t.Fatal("authorization content did not canonically bind the exact profile-fit posture")
	}
	if fixture.basis.Ref().Digest() != fixture.basis.Digest() {
		t.Fatal("authority-resolution basis ref does not bind its digest")
	}
	if fixture.resolution.Ref().Digest() != fixture.resolution.Digest() {
		t.Fatal("authority resolution ref does not bind its digest")
	}
	if fixture.basis.Digest() == fixture.resolution.Digest() ||
		fixture.record.Digest() == fixture.resolution.Digest() ||
		fixture.content.Digest() == fixture.resolution.Digest() {
		t.Fatal("description, record, basis, and resolution identities collapsed")
	}
	workWindow, workWindowOK := fixture.source.WorkWindow()
	if !workWindowOK || !fixture.record.PermissionRecord().EffectiveFrom().Equal(workWindow.Until()) {
		t.Fatal("Permission became effective before or after its instituting Work completed")
	}
	if !fixture.record.PermissionRecord().EffectiveFrom().After(
		fixture.content.ValidityWindow().From(),
	) {
		t.Fatal("fixture does not prove pre-SpeechAct Permission validity is excluded")
	}
	if _, err := SealProjectTypeEnvHeadSelectionPermissionRecord(
		fixture.content,
		authority.VerifiedSpeechActSourceV2{},
	); err == nil {
		t.Fatal("content plus planned identity minted a PermissionRecord without actual Work")
	}
	permission, err := DecodeProjectTypeEnvHeadSelectionPermissionRecord(
		fixture.content,
		fixture.source,
		fixture.record.PermissionRecord().CanonicalJSON(),
		fixture.record.PermissionRecord().Digest(),
	)
	if err != nil || permission.Digest() != fixture.record.PermissionRecord().Digest() {
		t.Fatalf("decode exact instituted Permission: %v", err)
	}

	content, err := DecodeProjectTypeEnvHeadSelectionAuthorizationContent(
		fixture.request,
		fixture.stage,
		fixture.content.CanonicalJSON(),
		fixture.content.Digest(),
	)
	if err != nil || content.Digest() != fixture.content.Digest() {
		t.Fatalf("decode exact authorization content: %v", err)
	}

	contract, err := DecodeProjectTypeEnvHeadSelectionSpeechActSourceContract(
		fixture.content.JudgementContext(),
		fixture.contract.CanonicalJSON(),
		fixture.contract.Digest(),
	)
	if err != nil || contract.Digest() != fixture.contract.Digest() {
		t.Fatalf("decode exact semantic source contract: %v", err)
	}

	adapter, err := DecodeProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		fixture.adapter.MethodDescription(),
		fixture.adapter.ExecutedWithin(),
		fixture.adapter.ContextPolicy(),
		fixture.adapter.CanonicalJSON(),
		fixture.adapter.Digest(),
	)
	if err != nil || adapter.Digest() != fixture.adapter.Digest() {
		t.Fatalf("decode exact source-adapter policy: %v", err)
	}

	bindingInput := ProjectAuthorityContextBindingInput{
		Project: fixture.binding.Project(),
		Root:    fixture.binding.Root(),
		Context: fixture.binding.Context(),
		Carrier: fixture.binding.Carrier(),
	}
	binding, err := DecodeProjectAuthorityContextBinding(
		bindingInput,
		fixture.binding.CanonicalJSON(),
		fixture.binding.Digest(),
	)
	if err != nil || binding.Digest() != fixture.binding.Digest() {
		t.Fatalf("decode exact project-context binding: %v", err)
	}

	recordInput := exactRecordInput(fixture, fixture.source, fixture.binding)
	record, err := DecodeProjectTypeEnvHeadSelectionSpeechActRecord(
		recordInput,
		fixture.record.CanonicalJSON(),
		fixture.record.Digest(),
	)
	if err != nil || record.Digest() != fixture.record.Digest() {
		t.Fatalf("decode exact SpeechAct record: %v", err)
	}

	policyInput := exactPolicyInput(fixture)
	policy, err := DecodeProjectTypeEnvHeadSelectionResolverPolicy(
		policyInput,
		fixture.policy.CanonicalJSON(),
		fixture.policy.Digest(),
	)
	if err != nil || policy.Digest() != fixture.policy.Digest() {
		t.Fatalf("decode exact resolver policy: %v", err)
	}

	basisInput := exactBasisInput(fixture)
	basis, err := DecodeProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
		basisInput,
		fixture.basis.CanonicalJSON(),
		fixture.basis.Digest(),
	)
	if err != nil || basis.Digest() != fixture.basis.Digest() {
		t.Fatalf("decode exact authority-resolution basis: %v", err)
	}

	resolution, err := DecodeStrictPermissionResolution(
		fixture.basis,
		fixture.resolution.CanonicalJSON(),
		fixture.resolution.Digest(),
	)
	if err != nil || resolution.Digest() != fixture.resolution.Digest() {
		t.Fatalf("decode exact authority resolution: %v", err)
	}
}

func TestProjectTypeEnvProfileFitPosturePreservesUnavailable(t *testing.T) {
	root, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-typeenv-head-selection-unavailable-profile",
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1: %v", err)
	}
	basis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile: %v", err)
	}
	edition, err := projecttypeenvprofilefit.NewRuleEdition(
		"haft.project-typeenv.profile-fit-rules/future",
	)
	if err != nil {
		t.Fatalf("NewRuleEdition: %v", err)
	}
	target := mustAuthorityStageTarget(t)
	assessment, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFitWithEdition(
		basis,
		target.snapshot,
		edition,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFitWithEdition: %v", err)
	}
	posture, err := projectProfileFitPosture(assessment)
	if err != nil {
		t.Fatalf("projectProfileFitPosture: %v", err)
	}
	if posture != "unavailable" {
		t.Fatalf("profile-fit posture = %q, want unavailable", posture)
	}
}

func TestProjectTypeEnvAuthorityCanonicalDecodersRejectTamper(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	recordInput := exactRecordInput(fixture, fixture.source, fixture.binding)
	basisInput := exactBasisInput(fixture)
	bindingInput := ProjectAuthorityContextBindingInput{
		Project: fixture.binding.Project(),
		Root:    fixture.binding.Root(),
		Context: fixture.binding.Context(),
		Carrier: fixture.binding.Carrier(),
	}
	policyInput := exactPolicyInput(fixture)

	tests := []struct {
		name      string
		canonical []byte
		decode    func([]byte) error
	}{
		{
			name:      "content",
			canonical: fixture.content.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionAuthorizationContent(
					fixture.request, fixture.stage, raw, fixture.content.Digest(),
				)
				return err
			},
		},
		{
			name:      "semantic source contract",
			canonical: fixture.contract.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionSpeechActSourceContract(
					fixture.contract.Context(),
					raw,
					fixture.contract.Digest(),
				)
				return err
			},
		},
		{
			name:      "source adapter policy",
			canonical: fixture.adapter.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionSourceAdapterPolicy(
					fixture.adapter.MethodDescription(),
					fixture.adapter.ExecutedWithin(),
					fixture.adapter.ContextPolicy(),
					raw,
					fixture.adapter.Digest(),
				)
				return err
			},
		},
		{
			name:      "project context binding",
			canonical: fixture.binding.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectAuthorityContextBinding(
					bindingInput,
					raw,
					fixture.binding.Digest(),
				)
				return err
			},
		},
		{
			name:      "resolver policy",
			canonical: fixture.policy.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionResolverPolicy(
					policyInput,
					raw,
					fixture.policy.Digest(),
				)
				return err
			},
		},
		{
			name:      "permission",
			canonical: fixture.record.PermissionRecord().CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionPermissionRecord(
					fixture.content,
					fixture.source,
					raw,
					fixture.record.PermissionRecord().Digest(),
				)
				return err
			},
		},
		{
			name:      "record",
			canonical: fixture.record.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionSpeechActRecord(
					recordInput, raw, fixture.record.Digest(),
				)
				return err
			},
		},
		{
			name:      "basis",
			canonical: fixture.basis.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
					basisInput, raw, fixture.basis.Digest(),
				)
				return err
			},
		},
		{
			name:      "resolution",
			canonical: fixture.resolution.CanonicalJSON(),
			decode: func(raw []byte) error {
				_, err := DecodeStrictPermissionResolution(
					fixture.basis, raw, fixture.resolution.Digest(),
				)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name+"/trailing", func(t *testing.T) {
			raw := append(append([]byte(nil), test.canonical...), byte(' '))
			if err := test.decode(raw); err == nil {
				t.Fatal("decoder accepted trailing material")
			}
		})
		t.Run(test.name+"/unknown-field", func(t *testing.T) {
			raw := bytes.Replace(
				test.canonical,
				[]byte(`{"schema":`),
				[]byte(`{"unknown":"tamper","schema":`),
				1,
			)
			if err := test.decode(raw); err == nil {
				t.Fatal("decoder accepted unknown field")
			}
		})
		t.Run(test.name+"/byte-change", func(t *testing.T) {
			raw := append([]byte(nil), test.canonical...)
			raw[len(raw)-2] ^= 1
			if err := test.decode(raw); err == nil {
				t.Fatal("decoder accepted changed canonical bytes")
			}
		})
	}
	if _, err := DecodeStrictPermissionResolution(
		fixture.basis,
		fixture.resolution.CanonicalJSON(),
		mustAuthorityDigest(t, "e"),
	); err == nil {
		t.Fatal("resolution decoder accepted a substituted digest")
	}
}

func TestSpeechActRecordRejectsSemanticAndProjectCrossBinding(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	startedAt := fixture.evaluated.Add(-10 * time.Minute)

	tests := []struct {
		name   string
		change func(speechActSourceVariant) speechActSourceVariant
	}{
		{
			name: "wrong act type",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.actType = "speech-act-type:authorize-unrelated-object"
				return value
			},
		},
		{
			name: "wrong state plane",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.statePlane = "state-plane:project-typeenv-head-storage"
				return value
			},
		},
		{
			name: "wrong delta",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.delta = "delta:compare-and-swap-project-typeenv-head"
				return value
			},
		},
		{
			name: "future CAS instituted object",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.institutedObject = "project-typeenv-head-CAS-effect:" + fixture.request.Ref().String()
				return value
			},
		},
		{
			name: "instituted value is not U Commitment",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.institutedKind = "Haft.ProjectTypeEnvHeadSelectionPermission"
				return value
			},
		},
		{
			name: "instituted commitment is not MAY",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.modality = "MUST"
				return value
			},
		},
		{
			name: "instituted commitment scopes another action",
			change: func(value speechActSourceVariant) speechActSourceVariant {
				value.scopedAction = "project-typeenv.head.unrelated"
				return value
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			variant := test.change(defaultSpeechActSourceVariant(t, fixture.content, fixture.request))
			source := buildSpeechActSourceVariant(
				t, fixture.content, fixture.request, startedAt, variant,
			)
			binding := bindingForSource(t, fixture, source)
			_, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
				exactRecordInput(fixture, source, binding),
			)
			if err == nil {
				t.Fatal("semantic cross-binding was admitted")
			}
		})
	}

	t.Run("missing exact affected ref", func(t *testing.T) {
		source := buildSpeechActSource(
			t, fixture.content, fixture.request, startedAt, true,
		)
		binding := bindingForSource(t, fixture, source)
		_, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
			exactRecordInput(fixture, source, binding),
		)
		if err == nil || !strings.Contains(err.Error(), "missing required affected referent") {
			t.Fatalf("missing strong affected ref was not rejected: %v", err)
		}
	})

	t.Run("different project root", func(t *testing.T) {
		variant := defaultSpeechActSourceVariant(t, fixture.content, fixture.request)
		variant.projectRoot = "/tmp/different-project-root"
		source := buildSpeechActSourceVariant(
			t, fixture.content, fixture.request, startedAt, variant,
		)
		_, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
			exactRecordInput(fixture, source, fixture.binding),
		)
		if err == nil || !strings.Contains(err.Error(), "ProjectRoot/ProjectID") {
			t.Fatalf("cross-root source was not rejected: %v", err)
		}
	})
}

func TestAuthorityBasisRejectsWrongAdapterAndOutOfWindowPolicy(t *testing.T) {
	fixture := buildAuthorityFixture(t)
	startedAt := fixture.evaluated.Add(-10 * time.Minute)

	t.Run("valid semantic source but unadmitted method adapter", func(t *testing.T) {
		variant := defaultSpeechActSourceVariant(t, fixture.content, fixture.request)
		variant.methodRef = "method:different-reviewed-source-adapter"
		variant.methodDescriptionRef = "method-description:different-reviewed-source-adapter:v1"
		variant.procedureRef = "procedure:different-reviewed-source-adapter:v1"
		source := buildSpeechActSourceVariant(
			t, fixture.content, fixture.request, startedAt, variant,
		)
		binding := bindingForSource(t, fixture, source)
		record, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
			exactRecordInput(fixture, source, binding),
		)
		if err != nil {
			t.Fatalf("semantic record should not hardcode one adapter: %v", err)
		}
		_, err = SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
			ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
				Policy:      fixture.policy,
				Record:      record,
				Content:     fixture.content,
				Request:     fixture.request,
				Stage:       fixture.stage,
				EvaluatedAt: fixture.evaluated,
			},
		)
		if err == nil || !strings.Contains(err.Error(), string(AuthorityRejectedAdapterMismatch)) {
			t.Fatalf("unadmitted adapter was not rejected by policy application: %v", err)
		}
	})

	t.Run("out-of-window resolver policy", func(t *testing.T) {
		window, err := authority.NewTimeWindow(
			fixture.evaluated.Add(-2*time.Hour),
			fixture.evaluated.Add(-time.Hour),
		)
		if err != nil {
			t.Fatalf("NewTimeWindow: %v", err)
		}
		policy, err := SealProjectTypeEnvHeadSelectionResolverPolicy(
			ProjectTypeEnvHeadSelectionResolverPolicyInput{
				Ref:             mustResolverPolicyRef(t, "policy:typeenv-head-selection:stale"),
				Edition:         mustResolverPolicyEdition(t, "resolver-policy-edition:stale"),
				EffectiveWindow: window,
				Action:          fixture.content.Action(),
				SourceContract:  fixture.contract,
				SourceAdapter:   fixture.adapter,
				ProjectBinding:  fixture.binding,
			},
		)
		if err != nil {
			t.Fatalf("Seal stale policy snapshot: %v", err)
		}
		input := exactBasisInput(fixture)
		input.Policy = policy
		_, err = SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(input)
		if err == nil || !strings.Contains(err.Error(), string(AuthorityRejectedPolicyMismatch)) {
			t.Fatalf("stale resolver-policy snapshot was not rejected: %v", err)
		}
	})
}

func TestResolverPolicyIsActionSpecificAndRejectsUnsatisfiableAdapters(t *testing.T) {
	fixture := buildAuthorityFixture(t)

	t.Run("one policy cannot claim genesis through a transition adapter", func(t *testing.T) {
		input := exactPolicyInput(fixture)
		input.Action = ProjectTypeEnvHeadSelectionGenesis
		_, err := SealProjectTypeEnvHeadSelectionResolverPolicy(input)
		if err == nil || !strings.Contains(err.Error(), "scoped action differs") {
			t.Fatalf("unsatisfiable cross-action policy was not rejected: %v", err)
		}
	})

	t.Run("adapter context must match semantic contract", func(t *testing.T) {
		otherContext := mustAuthorityValue(
			t,
			authority.NewBoundedContextRef,
			"bounded-context:unrelated-authority",
		)
		adapter := buildResolverSourceAdapter(
			t,
			fixture,
			otherContext,
			fixture.contract.ActType(),
			ProjectTypeEnvHeadSelectionTransition,
			"wrong-context",
		)
		input := exactPolicyInput(fixture)
		input.SourceAdapter = adapter
		_, err := SealProjectTypeEnvHeadSelectionResolverPolicy(input)
		if err == nil || !strings.Contains(err.Error(), "context differs") {
			t.Fatalf("context-incompatible adapter policy was not rejected: %v", err)
		}
	})

	t.Run("adapter recognized act type must match semantic contract", func(t *testing.T) {
		otherActType := mustAuthorityValue(
			t,
			authority.NewSpeechActTypeRef,
			"speech-act-type:authorize-unrelated-object",
		)
		adapter := buildResolverSourceAdapter(
			t,
			fixture,
			fixture.contract.Context(),
			otherActType,
			ProjectTypeEnvHeadSelectionTransition,
			"wrong-act-type",
		)
		input := exactPolicyInput(fixture)
		input.SourceAdapter = adapter
		_, err := SealProjectTypeEnvHeadSelectionResolverPolicy(input)
		if err == nil || !strings.Contains(err.Error(), "act type differs") {
			t.Fatalf("act-type-incompatible adapter policy was not rejected: %v", err)
		}
	})

	t.Run("genesis policy cannot admit transition content", func(t *testing.T) {
		genesisAdapter := buildResolverSourceAdapter(
			t,
			fixture,
			fixture.contract.Context(),
			fixture.contract.ActType(),
			ProjectTypeEnvHeadSelectionGenesis,
			"genesis",
		)
		genesisPolicy, err := SealProjectTypeEnvHeadSelectionResolverPolicy(
			ProjectTypeEnvHeadSelectionResolverPolicyInput{
				Ref: mustResolverPolicyRef(
					t,
					"policy:typeenv-head-selection:genesis",
				),
				Edition: mustResolverPolicyEdition(
					t,
					"resolver-policy-edition:genesis-v1",
				),
				EffectiveWindow: fixture.policy.EffectiveWindow(),
				Action:          ProjectTypeEnvHeadSelectionGenesis,
				SourceContract:  fixture.contract,
				SourceAdapter:   genesisAdapter,
				ProjectBinding:  fixture.binding,
			},
		)
		if err != nil {
			t.Fatalf("seal internally consistent genesis policy: %v", err)
		}
		if genesisPolicy.Action() != ProjectTypeEnvHeadSelectionGenesis ||
			fixture.policy.Action() != ProjectTypeEnvHeadSelectionTransition ||
			genesisPolicy.Digest() == fixture.policy.Digest() {
			t.Fatal("genesis and transition resolver policies are not distinct")
		}
		input := exactBasisInput(fixture)
		input.Policy = genesisPolicy
		_, err = SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(input)
		if err == nil ||
			!strings.Contains(err.Error(), string(AuthorityRejectedPolicyMismatch)) ||
			!strings.Contains(err.Error(), "admitted action differs") {
			t.Fatalf("genesis policy admitted transition occurrence: %v", err)
		}
	})
}

func TestResolverPolicyReusesAcrossOccurrencesAndResolutionsRemainDistinct(t *testing.T) {
	first := buildAuthorityFixture(t)
	stage, request := buildStageAndRequest(t, "selection-key-b")
	validity := first.content.ValidityWindow()
	content, err := SealProjectTypeEnvHeadSelectionAuthorizationContent(
		ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   mustDescriptionRef(t, "claim:project-typeenv-head-selection:b"),
			Request:          request,
			Stage:            stage,
			JudgementContext: first.content.JudgementContext(),
			ValidityWindow:   validity,
		},
	)
	if err != nil {
		t.Fatalf("Seal second authorization content: %v", err)
	}
	variant := defaultSpeechActSourceVariant(t, content, request)
	variant.speechActRef = "speech-act:typeenv-head-selection:test-b"
	speechAct := mustAuthorityValue(t, authority.NewSpeechActRef, variant.speechActRef)
	permissionRef, err := DeriveProjectTypeEnvHeadSelectionPermissionRef(content, speechAct)
	if err != nil {
		t.Fatalf("derive second PermissionRef: %v", err)
	}
	variant.institutedObject = permissionRef.String()
	source := buildSpeechActSourceVariant(
		t,
		content,
		request,
		first.evaluated.Add(-10*time.Minute),
		variant,
	)
	binding := bindingForSource(t, first, source)
	record, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
		ProjectTypeEnvHeadSelectionSpeechActRecordInput{
			Source:         source,
			SourceContract: first.contract,
			ProjectBinding: binding,
			Content:        content,
			Request:        request,
			Stage:          stage,
		},
	)
	if err != nil {
		t.Fatalf("Seal second SpeechAct record under reusable policy: %v", err)
	}
	basis, err := SealProjectTypeEnvHeadSelectionAuthorityResolutionBasis(
		ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
			Policy:      first.policy,
			Record:      record,
			Content:     content,
			Request:     request,
			Stage:       stage,
			EvaluatedAt: first.evaluated,
		},
	)
	if err != nil {
		t.Fatalf("reuse exact resolver policy for second occurrence: %v", err)
	}
	if basis.Digest() == first.basis.Digest() {
		t.Fatal("two occurrence-specific authority bases collapsed")
	}
	secondResolution, err := SealStrictPermissionResolution(basis)
	if err != nil {
		t.Fatalf("resolve second occurrence under reusable policy: %v", err)
	}
	if secondResolution.Digest() == first.resolution.Digest() {
		t.Fatal("two occurrence-specific resolutions collapsed")
	}
}

func buildResolverSourceAdapter(
	t *testing.T,
	fixture authorityFixture,
	contextRef authority.BoundedContextRef,
	actType authority.SpeechActTypeRef,
	action ProjectTypeEnvHeadSelectionAction,
	suffix string,
) ProjectTypeEnvHeadSelectionSourceAdapterPolicy {
	t.Helper()
	actionKind, err := action.AuthorityActionKind()
	if err != nil {
		t.Fatalf("AuthorityActionKind: %v", err)
	}
	effectRule, err := authority.NewInstitutionalEffectRule(
		mustAuthorityValue(
			t,
			authority.NewInstitutionalEffectRuleRef,
			"institution-rule:typeenv-head-selection:"+suffix,
		),
		mustAuthorityValue(t, authority.NewInstitutedObjectKind, "U.Commitment"),
		mustAuthorityValue(t, authority.NewInstitutionalModality, "MAY"),
		actionKind,
		authority.AuthorizeReviewedIntentUtteranceRule(),
		mustAuthorityValue(
			t,
			authority.NewUtteranceRef,
			"utterance:authorize-project-typeenv-head-selection:"+suffix,
		),
	)
	if err != nil {
		t.Fatalf("NewInstitutionalEffectRule: %v", err)
	}
	contextPolicy, err := authority.NewSpeechActContextPolicy(
		mustAuthorityValue(
			t,
			authority.NewContextPolicyRef,
			"policy:typeenv-head-selection:"+suffix,
		),
		contextRef,
		actType,
		effectRule,
	)
	if err != nil {
		t.Fatalf("NewSpeechActContextPolicy: %v", err)
	}
	adapter, err := SealProjectTypeEnvHeadSelectionSourceAdapterPolicy(
		fixture.adapter.MethodDescription(),
		fixture.adapter.ExecutedWithin(),
		contextPolicy,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionSourceAdapterPolicy: %v", err)
	}
	return adapter
}

func exactRecordInput(
	fixture authorityFixture,
	source authority.VerifiedSpeechActSourceV2,
	binding ProjectAuthorityContextBinding,
) ProjectTypeEnvHeadSelectionSpeechActRecordInput {
	return ProjectTypeEnvHeadSelectionSpeechActRecordInput{
		Source:         source,
		SourceContract: fixture.contract,
		ProjectBinding: binding,
		Content:        fixture.content,
		Request:        fixture.request,
		Stage:          fixture.stage,
	}
}

func exactPolicyInput(
	fixture authorityFixture,
) ProjectTypeEnvHeadSelectionResolverPolicyInput {
	return ProjectTypeEnvHeadSelectionResolverPolicyInput{
		Ref:             fixture.policy.Ref(),
		Edition:         fixture.policy.Edition(),
		EffectiveWindow: fixture.policy.EffectiveWindow(),
		Action:          fixture.policy.Action(),
		SourceContract:  fixture.contract,
		SourceAdapter:   fixture.adapter,
		ProjectBinding:  fixture.binding,
	}
}

func exactBasisInput(
	fixture authorityFixture,
) ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput {
	return ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
		Policy:      fixture.policy,
		Record:      fixture.record,
		Content:     fixture.content,
		Request:     fixture.request,
		Stage:       fixture.stage,
		EvaluatedAt: fixture.evaluated,
	}
}

func bindingForSource(
	t *testing.T,
	fixture authorityFixture,
	source authority.VerifiedSpeechActSourceV2,
) ProjectAuthorityContextBinding {
	t.Helper()
	root, rootOK := source.ProjectRoot()
	carriers, carriersOK := source.DescriptionCarriers()
	if !rootOK || !carriersOK || len(carriers) == 0 {
		t.Fatal("source project/carrier coordinates are unavailable")
	}
	binding, err := SealProjectAuthorityContextBinding(ProjectAuthorityContextBindingInput{
		Project: fixture.request.Project(),
		Root:    root,
		Context: fixture.content.JudgementContext(),
		Carrier: carriers[0],
	})
	if err != nil {
		t.Fatalf("SealProjectAuthorityContextBinding: %v", err)
	}
	return binding
}
