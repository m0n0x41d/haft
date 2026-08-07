package recordcarrier

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestTrustedRecordMembershipDeliveryRequiresAcceptedMappingPolicy(
	t *testing.T,
) {
	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	policy := trustedDeliveryPolicy(t, fixture.manifest, fixture.adapter)

	delivery, err := NewTrustedRecordMembershipSourceDeliveryV1(
		policy,
		fixture.source.ObservableInput(),
		fixture.source.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("NewTrustedRecordMembershipSourceDeliveryV1() error = %v", err)
	}
	if _, ok := delivery.(*trustedRecordMembershipSourceDeliveryV1); !ok {
		t.Fatalf("trusted delivery = %T, want trusted delivery variant", delivery)
	}

	otherManifest := mustRecordMembershipMappingManifest(
		t,
		"Haft.NoteRecordAdapter",
		"1.0.0",
		0x64,
	)
	rejectingPolicy := trustedDeliveryPolicy(t, otherManifest, fixture.adapter)
	delivery, err = NewTrustedRecordMembershipSourceDeliveryV1(
		rejectingPolicy,
		fixture.source.ObservableInput(),
		fixture.source.CanonicalBytes(),
	)
	if err == nil {
		t.Fatalf("unaccepted mapping produced trusted delivery %T", delivery)
	}
}

func TestTrustedRecordMembershipDeliveryRejectsSubstitutedSourceBytes(
	t *testing.T,
) {
	fixture := testRecordSourceFixture(t, DecisionRecordVariantV1{})
	other := testRecordSourceFixture(t, SpecSectionRecordVariantV1{})
	policy := trustedDeliveryPolicy(t, fixture.manifest, fixture.adapter)

	delivery, err := NewTrustedRecordMembershipSourceDeliveryV1(
		policy,
		fixture.source.ObservableInput(),
		other.source.CanonicalBytes(),
	)
	if err == nil {
		t.Fatalf("substituted source bytes produced trusted delivery %T", delivery)
	}
}

func trustedDeliveryPolicy(
	t *testing.T,
	manifest MappingManifestRef,
	adapter AdapterVersion,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	evaluator := trustedDeliveryMechanism(
		t,
		recordmembershipregistration.EvaluatorMechanism,
		NewRecordMembershipEvaluatorV1().RuleRef(),
		"artifact:record-membership-evaluator/v1",
		0x91,
	)
	delivery := trustedDeliveryMechanism(
		t,
		recordmembershipregistration.SourceDeliveryBoundaryMechanism,
		mustRecordMembershipRule(t, "haft.record-membership-source-delivery/v1"),
		"artifact:record-membership-source-delivery/v1",
		0x92,
	)
	mapping, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	)
	if err != nil {
		t.Fatalf("NewAcceptedMapping() error = %v", err)
	}
	artifact, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
		},
	)
	if err != nil {
		t.Fatalf("SealRegistrationArtifactV1() error = %v", err)
	}
	return artifact
}

func trustedDeliveryMechanism(
	t *testing.T,
	role recordmembershipregistration.MechanismRole,
	rule typedmemory.RuleRef,
	artifactRaw string,
	fill byte,
) recordmembershipregistration.MechanismCoordinate {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("NewCarrierRef() error = %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("build-20260717.1")
	if err != nil {
		t.Fatalf("NewCarrierEdition() error = %v", err)
	}
	coordinate, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     role,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   testDigest(t, fill),
		},
	)
	if err != nil {
		t.Fatalf("NewMechanismCoordinate() error = %v", err)
	}
	return coordinate
}
