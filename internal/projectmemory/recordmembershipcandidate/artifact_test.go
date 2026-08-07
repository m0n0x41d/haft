package recordmembershipcandidate

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// These DTOs are test-only. They let the compatibility facade construct
// deliberately noncanonical bytes without duplicating codec grammar in
// production.
type mechanismCoordinateCanonicalV1 struct {
	Role        string `json:"role"`
	RuleRef     string `json:"rule_ref"`
	ArtifactRef string `json:"artifact_ref"`
	Edition     string `json:"edition"`
	Digest      string `json:"digest"`
}

type acceptedMappingCanonicalV1 struct {
	MappingManifestRef string `json:"mapping_manifest_ref"`
	AdapterVersion     string `json:"adapter_version"`
}

type registrationCanonicalV1 struct {
	SchemaVersion  string                         `json:"schema_version"`
	Evaluator      mechanismCoordinateCanonicalV1 `json:"evaluator"`
	SourceDelivery mechanismCoordinateCanonicalV1 `json:"source_delivery_boundary"`
	Mappings       []acceptedMappingCanonicalV1   `json:"accepted_mappings"`
}

func TestRegistrationArtifactFixedCanonicalParity(t *testing.T) {
	const expectedCanonical = `{"schema_version":"haft.record-membership-evaluator-registration-candidate/v1",` +
		`"evaluator":{"role":"evaluator","rule_ref":"haft.member-of.project-record-carrier/v1",` +
		`"artifact_ref":"haft.runtime.record-membership-evaluator","edition":"1.0.0",` +
		`"digest":"sha256:a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1"},` +
		`"source_delivery_boundary":{"role":"source_delivery_boundary",` +
		`"rule_ref":"haft.deliver.project-record-membership/v1",` +
		`"artifact_ref":"haft.runtime.record-membership-delivery","edition":"1.0.0",` +
		`"digest":"sha256:a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2"},` +
		`"accepted_mappings":[{"mapping_manifest_ref":` +
		`"mapping-manifest:20:mapping.spec-section5:1.0.0sha256:b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2",` +
		`"adapter_version":"haft-spec-adapter/1.0.0"},{"mapping_manifest_ref":` +
		`"mapping-manifest:23:mapping.decision-record5:1.0.0sha256:b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1",` +
		`"adapter_version":"haft-decision-adapter/1.0.0"}]}`
	const expectedRef = "record-membership-evaluator-registration-candidate:" +
		"sha256:194a884f6871a4897fa24989ad49cd35a487ee43967b33dced9f892738828d7c"

	fixture := newRegistrationFixture(t)
	artifact := sealRegistrationFixture(t, fixture, fixture.mappings)
	if string(artifact.CanonicalBytes()) != expectedCanonical {
		t.Fatalf(
			"canonical registration bytes changed\n got: %s\nwant: %s",
			artifact.CanonicalBytes(),
			expectedCanonical,
		)
	}
	if artifact.Ref().String() != expectedRef {
		t.Fatalf("registration ref = %q, want %q", artifact.Ref().String(), expectedRef)
	}
}

func TestRegistrationArtifactPermutationInvariantCanonicalIdentity(t *testing.T) {
	fixture := newRegistrationFixture(t)
	forward := sealRegistrationFixture(t, fixture, fixture.mappings)
	reversedMappings := append([]AcceptedMapping(nil), fixture.mappings...)
	slices.Reverse(reversedMappings)
	reverse := sealRegistrationFixture(t, fixture, reversedMappings)

	if forward.Ref() != reverse.Ref() {
		t.Fatalf("permutation changed RegistrationRef: %q != %q", forward.Ref(), reverse.Ref())
	}
	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("permutation changed canonical registration bytes")
	}
	if err := forward.Verify(); err != nil {
		t.Fatalf("RegistrationArtifactV1.Verify(): %v", err)
	}
	decoded, err := DecodeRegistrationArtifactV1(forward.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRegistrationArtifactV1(): %v", err)
	}
	if decoded.Ref() != forward.Ref() {
		t.Fatalf("decoded ref = %q, want %q", decoded.Ref(), forward.Ref())
	}
	verified, err := VerifyRegistrationArtifactV1(forward.Ref(), forward.CanonicalBytes())
	if err != nil {
		t.Fatalf("VerifyRegistrationArtifactV1(): %v", err)
	}
	if !reflect.DeepEqual(verified.AcceptedMappings(), forward.AcceptedMappings()) {
		t.Fatal("verified accepted mapping policy changed")
	}
}

func TestRegistrationArtifactRejectsDuplicateAndConflictingManifestPolicy(t *testing.T) {
	fixture := newRegistrationFixture(t)
	duplicate := []AcceptedMapping{fixture.mappings[0], fixture.mappings[0]}
	_, err := SealRegistrationArtifactV1(RegistrationArtifactInputV1{
		Evaluator:      fixture.evaluator,
		SourceDelivery: fixture.delivery,
		Mappings:       duplicate,
	})
	assertMappingConflict(t, err, DuplicateAcceptedMapping)

	otherAdapter := mustAdapter(t, "haft-record-adapter/2.0.0")
	conflicting := mustAcceptedMapping(t, fixture.mappings[0].Manifest(), otherAdapter)
	forward := []AcceptedMapping{conflicting, fixture.mappings[0]}
	_, err = SealRegistrationArtifactV1(RegistrationArtifactInputV1{
		Evaluator:      fixture.evaluator,
		SourceDelivery: fixture.delivery,
		Mappings:       forward,
	})
	first := assertMappingConflict(t, err, ConflictingAdapterForManifest)
	slices.Reverse(forward)
	_, err = SealRegistrationArtifactV1(RegistrationArtifactInputV1{
		Evaluator:      fixture.evaluator,
		SourceDelivery: fixture.delivery,
		Mappings:       forward,
	})
	second := assertMappingConflict(t, err, ConflictingAdapterForManifest)
	if !reflect.DeepEqual(first.Adapters(), second.Adapters()) {
		t.Fatal("conflict diagnostics depend on caller input order")
	}
}

func TestRegistrationArtifactPolicyEvaluationIsExactAndNonMembership(t *testing.T) {
	fixture := newRegistrationFixture(t)
	artifact := sealRegistrationFixture(t, fixture, fixture.mappings)
	accepted := fixture.mappings[0]

	decision, err := artifact.EvaluateMappingPolicy(
		accepted.Manifest(),
		accepted.Adapter(),
	)
	if err != nil {
		t.Fatalf("EvaluateMappingPolicy(accepted): %v", err)
	}
	if decision.Kind() != MappingAccepted {
		t.Fatalf("accepted policy result = %q", decision.Kind())
	}
	if _, exists := decision.ExpectedAdapter(); exists {
		t.Fatal("accepted policy result exposed a mismatch adapter")
	}

	wrongAdapter := mustAdapter(t, "haft-record-adapter/9.9.9")
	decision, err = artifact.EvaluateMappingPolicy(accepted.Manifest(), wrongAdapter)
	if err != nil {
		t.Fatalf("EvaluateMappingPolicy(mismatch): %v", err)
	}
	if decision.Kind() != MappingAdapterMismatch {
		t.Fatalf("mismatch policy result = %q", decision.Kind())
	}
	expected, exists := decision.ExpectedAdapter()
	if !exists || expected != accepted.Adapter() {
		t.Fatalf("mismatch expected adapter = %q, %v", expected.String(), exists)
	}

	unknown := mustManifest(t, "mapping.unknown", "1.0.0", 0xc1)
	decision, err = artifact.EvaluateMappingPolicy(unknown, wrongAdapter)
	if err != nil {
		t.Fatalf("EvaluateMappingPolicy(unknown): %v", err)
	}
	if decision.Kind() != MappingManifestNotAccepted {
		t.Fatalf("unknown policy result = %q", decision.Kind())
	}

	typeName := reflect.TypeOf(decision).String()
	for _, forbidden := range []string{"MemberOf", "Trusted", "Authority", "Approval"} {
		if strings.Contains(typeName, forbidden) {
			t.Fatalf("policy decision type %q contains forbidden semantic %q", typeName, forbidden)
		}
	}
}

func TestRegistrationArtifactRejectsTamperUnknownTrailingAndNoncanonicalOrder(t *testing.T) {
	fixture := newRegistrationFixture(t)
	artifact := sealRegistrationFixture(t, fixture, fixture.mappings)
	canonical := artifact.CanonicalBytes()

	tampered := append([]byte(nil), canonical...)
	index := bytes.Index(tampered, []byte(fixture.evaluator.Rule().String()))
	if index < 0 {
		t.Fatal("canonical fixture is missing evaluator RuleRef")
	}
	tampered[index] = 'x'
	if _, err := VerifyRegistrationArtifactV1(artifact.Ref(), tampered); err == nil {
		t.Fatal("tampered registration retained the asserted identity")
	}

	var raw map[string]any
	if err := json.Unmarshal(canonical, &raw); err != nil {
		t.Fatalf("json.Unmarshal(): %v", err)
	}
	raw["unknown"] = true
	unknown, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("json.Marshal(unknown): %v", err)
	}
	if _, err := DecodeRegistrationArtifactV1(unknown); err == nil {
		t.Fatal("registration with unknown field decoded")
	}

	trailing := append(append([]byte(nil), canonical...), []byte("{}")...)
	if _, err := DecodeRegistrationArtifactV1(trailing); err == nil {
		t.Fatal("registration with trailing content decoded")
	}

	var encoded registrationCanonicalV1
	if err := json.Unmarshal(canonical, &encoded); err != nil {
		t.Fatalf("json.Unmarshal(canonical DTO): %v", err)
	}
	slices.Reverse(encoded.Mappings)
	noncanonical, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal(noncanonical): %v", err)
	}
	if _, err := DecodeRegistrationArtifactV1(noncanonical); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("DecodeRegistrationArtifactV1(noncanonical) error = %v", err)
	}
}

func TestRegistrationIdentityChangesWithEverySemanticCoordinate(t *testing.T) {
	fixture := newRegistrationFixture(t)
	base := sealRegistrationFixture(t, fixture, fixture.mappings)

	changedEvaluator := fixture
	changedEvaluator.evaluator = mustMechanism(
		t,
		EvaluatorMechanism,
		"haft.member-of.project-record-carrier/v2",
		"haft.runtime.record-membership-evaluator",
		"2.0.0",
		0xd1,
	)
	assertRegistrationIdentityChanges(t, base, sealRegistrationFixture(t, changedEvaluator, changedEvaluator.mappings))

	changedDelivery := fixture
	changedDelivery.delivery = mustMechanism(
		t,
		SourceDeliveryBoundaryMechanism,
		"haft.deliver.project-record-membership/v2",
		"haft.runtime.record-membership-delivery",
		"2.0.0",
		0xd2,
	)
	assertRegistrationIdentityChanges(t, base, sealRegistrationFixture(t, changedDelivery, changedDelivery.mappings))

	changedPolicy := append([]AcceptedMapping(nil), fixture.mappings...)
	changedPolicy[0] = mustAcceptedMapping(
		t,
		changedPolicy[0].Manifest(),
		mustAdapter(t, "haft-record-adapter/1.0.1"),
	)
	assertRegistrationIdentityChanges(t, base, sealRegistrationFixture(t, fixture, changedPolicy))
}

func TestRegistrationArtifactDoesNotExposeActivationOrTrustedDeliverySurface(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(RegistrationArtifactV1{}),
		reflect.TypeOf(RegistrationArtifactInputV1{}),
		reflect.TypeOf(MappingPolicyDecision{}),
	}
	for _, current := range types {
		for index := 0; index < current.NumMethod(); index++ {
			method := current.Method(index)
			for _, forbidden := range []string{"Activate", "Select", "Admit", "Trusted", "MemberOf"} {
				if strings.Contains(method.Name, forbidden) {
					t.Fatalf("%s exposes forbidden method %q", current, method.Name)
				}
			}
		}
	}
}

func TestMechanismCoordinateRequiresExactEditionAndClosedRole(t *testing.T) {
	rule, err := typedmemory.NewRuleRef("haft.member-of.project-record-carrier/v1")
	if err != nil {
		t.Fatalf("typedmemory.NewRuleRef(): %v", err)
	}
	artifact, err := typedmemory.NewCarrierRef("haft.runtime.record-membership-evaluator")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef(): %v", err)
	}
	for _, editionRaw := range []string{"latest", "1", "1.0", "*"} {
		edition, editionErr := typedmemory.NewCarrierEdition(editionRaw)
		if editionErr != nil {
			t.Fatalf("typedmemory.NewCarrierEdition(%q): %v", editionRaw, editionErr)
		}
		_, coordinateErr := NewMechanismCoordinate(MechanismCoordinateInput{
			Role:     EvaluatorMechanism,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   mustDigest(t, 0xe1),
		})
		if coordinateErr == nil {
			t.Fatalf("NewMechanismCoordinate() accepted floating edition %q", editionRaw)
		}
	}
	exact, err := typedmemory.NewCarrierEdition("build-20260717.1")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition(exact build): %v", err)
	}
	_, err = NewMechanismCoordinate(MechanismCoordinateInput{
		Role:     EvaluatorMechanism,
		Rule:     rule,
		Artifact: artifact,
		Edition:  exact,
		Digest:   mustDigest(t, 0xe2),
	})
	if err != nil {
		t.Fatalf("NewMechanismCoordinate(exact build): %v", err)
	}
}

type registrationFixture struct {
	evaluator MechanismCoordinate
	delivery  MechanismCoordinate
	mappings  []AcceptedMapping
}

func newRegistrationFixture(t *testing.T) registrationFixture {
	t.Helper()
	evaluator := mustMechanism(
		t,
		EvaluatorMechanism,
		"haft.member-of.project-record-carrier/v1",
		"haft.runtime.record-membership-evaluator",
		"1.0.0",
		0xa1,
	)
	delivery := mustMechanism(
		t,
		SourceDeliveryBoundaryMechanism,
		"haft.deliver.project-record-membership/v1",
		"haft.runtime.record-membership-delivery",
		"1.0.0",
		0xa2,
	)
	firstManifest := mustManifest(t, "mapping.decision-record", "1.0.0", 0xb1)
	secondManifest := mustManifest(t, "mapping.spec-section", "1.0.0", 0xb2)
	return registrationFixture{
		evaluator: evaluator,
		delivery:  delivery,
		mappings: []AcceptedMapping{
			mustAcceptedMapping(t, secondManifest, mustAdapter(t, "haft-spec-adapter/1.0.0")),
			mustAcceptedMapping(t, firstManifest, mustAdapter(t, "haft-decision-adapter/1.0.0")),
		},
	}
}

func sealRegistrationFixture(
	t *testing.T,
	fixture registrationFixture,
	mappings []AcceptedMapping,
) RegistrationArtifactV1 {
	t.Helper()
	artifact, err := SealRegistrationArtifactV1(RegistrationArtifactInputV1{
		Evaluator:      fixture.evaluator,
		SourceDelivery: fixture.delivery,
		Mappings:       mappings,
	})
	if err != nil {
		t.Fatalf("SealRegistrationArtifactV1(): %v", err)
	}
	return artifact
}

func mustMechanism(
	t *testing.T,
	role MechanismRole,
	ruleRaw string,
	artifactRaw string,
	editionRaw string,
	digestFill byte,
) MechanismCoordinate {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewRuleRef(): %v", err)
	}
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition(): %v", err)
	}
	coordinate, err := NewMechanismCoordinate(MechanismCoordinateInput{
		Role:     role,
		Rule:     rule,
		Artifact: artifact,
		Edition:  edition,
		Digest:   mustDigest(t, digestFill),
	})
	if err != nil {
		t.Fatalf("NewMechanismCoordinate(): %v", err)
	}
	return coordinate
}

func mustAcceptedMapping(
	t *testing.T,
	manifest recordcarrier.MappingManifestRef,
	adapter recordcarrier.AdapterVersion,
) AcceptedMapping {
	t.Helper()
	mapping, err := NewAcceptedMapping(AcceptedMappingInput{
		Manifest: manifest,
		Adapter:  adapter,
	})
	if err != nil {
		t.Fatalf("NewAcceptedMapping(): %v", err)
	}
	return mapping
}

func mustManifest(
	t *testing.T,
	id string,
	version string,
	digestFill byte,
) recordcarrier.MappingManifestRef {
	t.Helper()
	manifest, err := recordcarrier.NewMappingManifestRef(
		id,
		version,
		mustDigest(t, digestFill),
	)
	if err != nil {
		t.Fatalf("recordcarrier.NewMappingManifestRef(): %v", err)
	}
	return manifest
}

func mustAdapter(t *testing.T, raw string) recordcarrier.AdapterVersion {
	t.Helper()
	adapter, err := recordcarrier.NewAdapterVersion(raw)
	if err != nil {
		t.Fatalf("recordcarrier.NewAdapterVersion(): %v", err)
	}
	return adapter
}

func mustDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	hexValue := strings.Repeat(string([]byte{hexDigit(fill >> 4), hexDigit(fill & 0x0f)}), 32)
	raw := "sha256:" + hexValue
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("typedmemory.NewSHA256Digest(): %v", err)
	}
	return digest
}

func hexDigit(value byte) byte {
	if value < 10 {
		return '0' + value
	}
	return 'a' + value - 10
}

func assertMappingConflict(
	t *testing.T,
	err error,
	want AcceptedMappingConflictKind,
) AcceptedMappingConflict {
	t.Helper()
	var conflict AcceptedMappingConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %T %v, want AcceptedMappingConflict", err, err)
	}
	if conflict.Kind() != want {
		t.Fatalf("conflict kind = %q, want %q", conflict.Kind(), want)
	}
	return conflict
}

func assertRegistrationIdentityChanges(
	t *testing.T,
	left RegistrationArtifactV1,
	right RegistrationArtifactV1,
) {
	t.Helper()
	if left.Ref() == right.Ref() {
		t.Fatalf("semantic change retained registration identity %q", left.Ref())
	}
	if bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("semantic change retained canonical registration bytes")
	}
}
