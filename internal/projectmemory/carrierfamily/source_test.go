package carrierfamily

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestFamilySpecificCarriersBindExactPayloadAndRule(t *testing.T) {
	payload := testPayload(t)
	entity := testEntity(t, "entity:carrier-family")
	contextRef := testContext(t)
	tests := []struct {
		name string
		seal func(typedmemory.EntityID, typedmemory.BoundedContextRef, SourcePayloadV1) (CarrierV1, error)
		rule typedmemory.RuleRef
	}{
		{name: "carrier edition", seal: SealCarrierEditionCarrierV1, rule: CarrierEditionEvaluatorRuleV1()},
		{name: "project claim", seal: SealProjectClaimCarrierV1, rule: ProjectClaimEvaluatorRuleV1()},
		{name: "performed work", seal: SealPerformedWorkOccurrenceCarrierV1, rule: PerformedWorkOccurrenceEvaluatorRuleV1()},
		{name: "code anchor", seal: SealCodeAnchorCarrierV1, rule: CodeAnchorEvaluatorRuleV1()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			carrier, err := test.seal(entity, contextRef, payload)
			if err != nil {
				t.Fatalf("seal carrier: %v", err)
			}
			decoded, err := DecodeCarrierV1(carrier.CanonicalBytes())
			if err != nil {
				t.Fatalf("DecodeCarrierV1: %v", err)
			}
			if decoded.EvaluatorRule() != test.rule ||
				decoded.Payload().Ref() != payload.Ref() ||
				decoded.Payload().Edition() != payload.Edition() ||
				decoded.Payload().Digest() != payload.Digest() ||
				!bytes.Equal(decoded.Payload().CanonicalBytes(), payload.CanonicalBytes()) {
				t.Fatal("decoded carrier lost its exact family or payload coordinate")
			}
		})
	}
}

func TestCarrierRejectsPayloadDigestSubstitution(t *testing.T) {
	payload := testPayload(t)
	mutated := payload.CanonicalBytes()
	mutated[0] ^= 0x01
	_, err := NewSourcePayloadV1(
		payload.Ref(),
		payload.Edition(),
		payload.Digest(),
		payload.SchemaVersion(),
		mutated,
	)
	if err == nil {
		t.Fatal("NewSourcePayloadV1 accepted bytes under another digest")
	}
}

func TestFamilyMappingManifestsAreExactAndDistinct(t *testing.T) {
	constructors := []func() (MappingManifestV1, error){
		CurrentCarrierEditionMappingManifestV1,
		CurrentProjectClaimMappingManifestV1,
		CurrentPerformedWorkOccurrenceMappingManifestV1,
		CurrentCodeAnchorMappingManifestV1,
	}
	seen := make(map[string]struct{}, len(constructors))
	for _, constructor := range constructors {
		manifest, err := constructor()
		if err != nil {
			t.Fatal(err)
		}
		if err := manifest.Verify(); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeMappingManifestV1(manifest.CanonicalBytes())
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Ref() != manifest.Ref() ||
			decoded.AdapterVersion() != manifest.AdapterVersion() ||
			decoded.EvaluatorRule() != manifest.EvaluatorRule() {
			t.Fatal("mapping manifest round trip changed exact coordinates")
		}
		if _, duplicate := seen[manifest.Ref().String()]; duplicate {
			t.Fatal("two family manifests share one content identity")
		}
		seen[manifest.Ref().String()] = struct{}{}
	}
}

func TestTrustedSourceRequiresExactFamilyPolicyAndMapping(t *testing.T) {
	fixture := testSourceFixture(t)
	policy := testPolicy(t, fixture.source.EvaluatorRule(), fixture.mapping, fixture.adapter)
	delivery, err := NewTrustedMembershipSourceDeliveryV1(
		policy,
		fixture.source.ObservableInput(),
		fixture.source.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("NewTrustedMembershipSourceDeliveryV1: %v", err)
	}
	if delivery.Source().EvaluatorRule() != ProjectClaimEvaluatorRuleV1() {
		t.Fatal("trusted delivery changed the exact family")
	}

	otherAdapter, err := recordmapping.NewAdapterVersion("claim-adapter/2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	wrongPolicy := testPolicy(t, fixture.source.EvaluatorRule(), fixture.mapping, otherAdapter)
	if _, err := NewTrustedMembershipSourceDeliveryV1(
		wrongPolicy,
		fixture.source.ObservableInput(),
		fixture.source.CanonicalBytes(),
	); err == nil {
		t.Fatal("trusted delivery accepted an unregistered adapter mapping")
	}
}

func TestSourceRejectsOuterFamilySubstitution(t *testing.T) {
	fixture := testSourceFixture(t)
	mutated := strings.Replace(
		string(fixture.source.CanonicalBytes()),
		`"family":"project_claim"`,
		`"family":"code_anchor"`,
		1,
	)
	if _, err := DecodeMembershipSourceV1([]byte(mutated)); err == nil {
		t.Fatal("DecodeMembershipSourceV1 accepted an outer family substitution")
	}
}

type sourceFixture struct {
	source  MembershipSourceV1
	mapping recordmapping.MappingManifestRef
	adapter recordmapping.AdapterVersion
}

func testSourceFixture(t *testing.T) sourceFixture {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	entity := testEntity(t, "entity:project-claim")
	contextRef := testContext(t)
	carrier, err := SealProjectClaimCarrierV1(entity, contextRef, testPayload(t))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := CurrentProjectClaimMappingManifestV1()
	if err != nil {
		t.Fatal(err)
	}
	mapping := manifest.Ref()
	adapter := manifest.AdapterVersion()
	binding, err := SealEntityCarrierBindingV1(project, carrier, mapping, adapter)
	if err != nil {
		t.Fatal(err)
	}
	source, err := SealMembershipSourceV1(
		project,
		entity,
		contextRef,
		carrier,
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	return sourceFixture{source: source, mapping: mapping, adapter: adapter}
}

func testPayload(t *testing.T) SourcePayloadV1 {
	t.Helper()
	canonical := []byte(`{"schema":"claim-graph/v1","value":"accepted"}`)
	digest, err := digestCanonical(canonical)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := typedmemory.NewCarrierRef("project-claim-payload:" + digest.String())
	if err != nil {
		t.Fatal(err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := NewSourcePayloadV1(
		ref,
		edition,
		digest,
		"haft.project-claim-payload/v1",
		canonical,
	)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func testPolicy(
	t *testing.T,
	rule typedmemory.RuleRef,
	mapping recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef("artifact:carrier-family-runtime")
	if err != nil {
		t.Fatal(err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	digest := testDigest(t, 'b')
	evaluator, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   digest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   digest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: mapping,
			Adapter:  adapter,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{accepted},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func testEntity(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func testContext(t *testing.T) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func testDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	value, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(
		string([]byte{fill}),
		64,
	))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
