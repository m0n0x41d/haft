package projecttypeenv

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestRuntimeEvaluationBasisRegistrationPolicyIsFirstClassAndFailClosed(
	t *testing.T,
) {
	policy := registrationPolicyArtifactFixture(t, defaultRegistrationPolicySpec())
	policyPin, err := NewRegistrationPolicyPin(policy)
	if err != nil {
		t.Fatalf("NewRegistrationPolicyPin(): %v", err)
	}
	mechanismPin := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.registration-policy-fixture/v1",
		"artifact:registration-policy-fixture",
		"1.0.0",
		0x51,
	)
	forward, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{policyPin, mechanismPin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(forward): %v", err)
	}
	reverse, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{mechanismPin, policyPin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(reverse): %v", err)
	}
	if forward.Ref() != reverse.Ref() ||
		!bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("X identity depends on mixed pin caller order")
	}
	if err := forward.VerifyResolvedClosure(); err != nil {
		t.Fatalf("VerifyResolvedClosure(mixed X): %v", err)
	}
	if got := forward.RegistrationPolicyPins(); len(got) != 1 ||
		got[0].Registration() != policy.Ref() {
		t.Fatalf("registration-policy pins = %#v, want exact %q", got, policy.Ref())
	}

	policyOnly, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{policyPin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(policy only): %v", err)
	}
	unresolved, err := DecodeRuntimeEvaluationBasisArtifact(policyOnly.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRuntimeEvaluationBasisArtifact(): %v", err)
	}
	if err := unresolved.VerifyResolvedClosure(); err == nil ||
		!strings.Contains(err.Error(), "is not resolved") {
		t.Fatalf("unresolved registration-policy closure error = %v", err)
	}
	resolved, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		policy,
	)
	if err != nil {
		t.Fatalf("ResolveRuntimeEvaluationBasisRegistrationPolicies(): %v", err)
	}
	if err := resolved.VerifyResolvedClosure(); err != nil {
		t.Fatalf("VerifyResolvedClosure(resolved policy): %v", err)
	}
	if _, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		policy,
		policy,
	); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate registration-policy closure error = %v", err)
	}
	other := defaultRegistrationPolicySpec()
	other.adapter = "haft-record-adapter/2.0.0"
	otherPolicy := registrationPolicyArtifactFixture(t, other)
	if _, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		otherPolicy,
	); err == nil || !strings.Contains(err.Error(), "is not referenced by X") {
		t.Fatalf("mismatched registration-policy closure error = %v", err)
	}
	if _, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		policy,
		otherPolicy,
	); err == nil || !strings.Contains(err.Error(), "is not referenced by X") {
		t.Fatalf("extra registration-policy closure error = %v", err)
	}

	tampered := policy.CanonicalBytes()
	tampered[len(tampered)-1] ^= 0x01
	if _, err := VerifyRegistrationPolicyArtifact(
		policy.Ref(),
		tampered,
	); err == nil {
		t.Fatal("tampered registration policy retained its asserted identity")
	}

	payload, err := decodeRuntimeEvaluationBasisEnvelope(policyOnly.CanonicalBytes())
	if err != nil {
		t.Fatalf("decodeRuntimeEvaluationBasisEnvelope(): %v", err)
	}
	encoded := runtimeEvaluationBasisCanonicalV1{}
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("json.Unmarshal(X): %v", err)
	}
	encoded.Pins[0].Role = string(RuntimeMechanismRoleEvaluator)
	forgedPayload, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal(forged X): %v", err)
	}
	if _, err := DecodeRuntimeEvaluationBasisArtifact(
		runtimeEvaluationBasisEnvelopeFixture(forgedPayload),
	); err == nil || !strings.Contains(err.Error(), "requires only") {
		t.Fatalf("forged registration-policy pin error = %v", err)
	}
}

func TestRuntimeEvaluationBasisRegistrationPolicyClosureSupportsMultipleExactRules(
	t *testing.T,
) {
	first := registrationPolicyArtifactFixture(t, defaultRegistrationPolicySpec())
	secondSpec := defaultRegistrationPolicySpec()
	secondSpec.evaluatorRule = "haft.member-of.code-anchor-carrier/v1"
	secondSpec.evaluatorArtifact = "haft.runtime.code-anchor-membership-evaluator"
	secondSpec.deliveryRule = "haft.deliver.code-anchor-membership/v1"
	secondSpec.deliveryArtifact = "haft.runtime.code-anchor-membership-delivery"
	secondSpec.evaluatorDigest = 0xc1
	secondSpec.deliveryDigest = 0xc2
	secondSpec.manifestID = "mapping.code-anchor"
	secondSpec.manifestDigest = 0xc3
	secondSpec.adapter = "haft-code-anchor-adapter/2.0.0"
	second := registrationPolicyArtifactFixture(t, secondSpec)
	firstPin, err := NewRegistrationPolicyPin(first)
	if err != nil {
		t.Fatalf("NewRegistrationPolicyPin(first): %v", err)
	}
	secondPin, err := NewRegistrationPolicyPin(second)
	if err != nil {
		t.Fatalf("NewRegistrationPolicyPin(second): %v", err)
	}

	forward, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{firstPin, secondPin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(forward): %v", err)
	}
	reverse, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{secondPin, firstPin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(reverse): %v", err)
	}
	if forward.Ref() != reverse.Ref() ||
		!bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("multi-policy X identity depends on caller order")
	}
	if err := forward.VerifyResolvedClosure(); err != nil {
		t.Fatalf("VerifyResolvedClosure(multi-policy X): %v", err)
	}
	if pins := forward.RegistrationPolicyPins(); len(pins) != 2 {
		t.Fatalf("registration-policy pins = %d, want 2", len(pins))
	}

	unresolved, err := DecodeRuntimeEvaluationBasisArtifact(forward.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRuntimeEvaluationBasisArtifact(): %v", err)
	}
	if _, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		first,
	); err == nil || !strings.Contains(err.Error(), second.Ref().String()) {
		t.Fatalf("missing second registration-policy error = %v", err)
	}
	resolved, err := ResolveRuntimeEvaluationBasisRegistrationPolicies(
		unresolved,
		second,
		first,
	)
	if err != nil {
		t.Fatalf("ResolveRuntimeEvaluationBasisRegistrationPolicies(): %v", err)
	}
	if err := resolved.VerifyResolvedClosure(); err != nil {
		t.Fatalf("VerifyResolvedClosure(resolved multi-policy X): %v", err)
	}
	if _, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{firstPin, firstPin},
		nil,
		nil,
	); err == nil || !strings.Contains(err.Error(), "repeats registration-policy coordinate") {
		t.Fatalf("duplicate registration-policy pin error = %v", err)
	}
}

func TestRegistrationPolicyCoordinatesChangeRegistrationXAndComposite(
	t *testing.T,
) {
	baseSpec := defaultRegistrationPolicySpec()
	basePolicy := registrationPolicyArtifactFixture(t, baseSpec)
	baseBasis := registrationPolicyRuntimeBasisFixture(t, basePolicy)
	base, extensions := projectTypeEnvCompositeArtifactFixture(t)
	linked := acceptedCompositeIR(
		t,
		LinkProjectTypeEnvCompositeIR(base, extensions),
	)
	baseComposite, err := SealProjectTypeEnvComposite(linked, baseBasis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(base): %v", err)
	}

	tests := []struct {
		name   string
		change func(registrationPolicyTestSpec) registrationPolicyTestSpec
	}{
		{
			name: "evaluator identity",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.evaluatorDigest = 0xa3
				return spec
			},
		},
		{
			name: "delivery identity",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.deliveryDigest = 0xa4
				return spec
			},
		},
		{
			name: "mapping manifest",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.manifestID = "mapping.project-record-v2"
				return spec
			},
		},
		{
			name: "mapping manifest version",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.manifestVersion = "1.0.1"
				return spec
			},
		},
		{
			name: "mapping manifest content",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.manifestDigest = 0xb2
				return spec
			},
		},
		{
			name: "adapter version",
			change: func(spec registrationPolicyTestSpec) registrationPolicyTestSpec {
				spec.adapter = "haft-record-adapter/1.0.1"
				return spec
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changedPolicy := registrationPolicyArtifactFixture(
				t,
				test.change(baseSpec),
			)
			if changedPolicy.Ref() == basePolicy.Ref() {
				t.Fatal("registration identity did not change")
			}
			changedBasis := registrationPolicyRuntimeBasisFixture(t, changedPolicy)
			if changedBasis.Ref() == baseBasis.Ref() {
				t.Fatal("X identity did not change")
			}
			changedComposite, err := SealProjectTypeEnvComposite(
				linked,
				changedBasis,
			)
			if err != nil {
				t.Fatalf("SealProjectTypeEnvComposite(changed): %v", err)
			}
			if changedComposite.Ref() == baseComposite.Ref() {
				t.Fatal("C identity did not change transitively through X")
			}
		})
	}
}

func TestRegistrationPolicyAndPinDoNotClaimExecutableAttestation(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(RegistrationPolicyPin{}),
		reflect.TypeOf(recordmembershipregistration.RegistrationArtifactV1{}),
	}
	for _, current := range types {
		for index := 0; index < current.NumMethod(); index++ {
			method := current.Method(index)
			for _, forbidden := range []string{
				"Activate",
				"Attest",
				"Execute",
				"Loaded",
				"Trust",
			} {
				if strings.Contains(method.Name, forbidden) {
					t.Fatalf("%s exposes executable claim method %q", current, method.Name)
				}
			}
		}
	}
}

type registrationPolicyTestSpec struct {
	evaluatorDigest   byte
	deliveryDigest    byte
	evaluatorRule     string
	evaluatorArtifact string
	deliveryRule      string
	deliveryArtifact  string
	manifestID        string
	manifestVersion   string
	manifestDigest    byte
	adapter           string
}

func defaultRegistrationPolicySpec() registrationPolicyTestSpec {
	return registrationPolicyTestSpec{
		evaluatorDigest:   0xa1,
		deliveryDigest:    0xa2,
		evaluatorRule:     "haft.rule.project-concern-member/v1",
		evaluatorArtifact: "haft.runtime.record-membership-evaluator",
		deliveryRule:      "haft.member-of.project-record-carrier/v1",
		deliveryArtifact:  "haft.runtime.record-membership-delivery",
		manifestID:        "mapping.project-record",
		manifestVersion:   "1.0.0",
		manifestDigest:    0xb1,
		adapter:           "haft-record-adapter/1.0.0",
	}
}

func registrationPolicyArtifactFixture(
	t *testing.T,
	spec registrationPolicyTestSpec,
) RegistrationPolicyArtifact {
	t.Helper()
	evaluator := registrationPolicyMechanismFixture(
		t,
		recordmembershipregistration.EvaluatorMechanism,
		spec.evaluatorRule,
		spec.evaluatorArtifact,
		spec.evaluatorDigest,
	)
	delivery := registrationPolicyMechanismFixture(
		t,
		recordmembershipregistration.SourceDeliveryBoundaryMechanism,
		spec.deliveryRule,
		spec.deliveryArtifact,
		spec.deliveryDigest,
	)
	manifest, err := recordmapping.NewMappingManifestRef(
		spec.manifestID,
		spec.manifestVersion,
		runtimeEvaluationBasisDigestFixture(t, spec.manifestDigest),
	)
	if err != nil {
		t.Fatalf("recordmapping.NewMappingManifestRef(): %v", err)
	}
	adapter, err := recordmapping.NewAdapterVersion(spec.adapter)
	if err != nil {
		t.Fatalf("recordmapping.NewAdapterVersion(): %v", err)
	}
	mapping, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	)
	if err != nil {
		t.Fatalf("recordmembershipregistration.NewAcceptedMapping(): %v", err)
	}
	artifact, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings: []recordmembershipregistration.AcceptedMapping{
				mapping,
			},
		},
	)
	if err != nil {
		t.Fatalf("SealRegistrationArtifactV1(): %v", err)
	}
	return artifact
}

func registrationPolicyMechanismFixture(
	t *testing.T,
	role recordmembershipregistration.MechanismRole,
	ruleRaw string,
	artifactRaw string,
	digest byte,
) recordmembershipregistration.MechanismCoordinate {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewRuleRef(): %v", err)
	}
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition(): %v", err)
	}
	coordinate, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     role,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   runtimeEvaluationBasisDigestFixture(t, digest),
		},
	)
	if err != nil {
		t.Fatalf("recordmembershipregistration.NewMechanismCoordinate(): %v", err)
	}
	return coordinate
}

func registrationPolicyRuntimeBasisFixture(
	t *testing.T,
	policy RegistrationPolicyArtifact,
) RuntimeEvaluationBasisArtifact {
	t.Helper()
	pin, err := NewRegistrationPolicyPin(policy)
	if err != nil {
		t.Fatalf("NewRegistrationPolicyPin(): %v", err)
	}
	basis, err := SealRuntimeEvaluationBasisWithPins(
		[]RuntimeEvaluationBasisPin{pin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasisWithPins(): %v", err)
	}
	if err := basis.VerifyResolvedClosure(); err != nil {
		t.Fatalf("VerifyResolvedClosure(): %v", err)
	}
	return basis
}
