package projecttypeenv

import (
	"bytes"
	"testing"
)

func TestCompositeRuntimeRequirementDiscoveryReadsSourceBeforeXAndC(
	t *testing.T,
) {
	t.Parallel()

	base := loadBaseArtifact(t)
	extension := sealMembershipBasisFixture(
		t,
		base,
		carrierFirstMembershipBasisFixture,
	)
	linked := acceptedCompositeIR(
		t,
		LinkProjectTypeEnvCompositeIR(
			base,
			[]ProjectTypeEnvExtensionArtifact{extension},
		),
	)

	discovery := DiscoverProjectTypeEnvCompositeRuntimeRequirements(base, linked)
	if discovery.Rejected() {
		t.Fatalf("source-owned discovery rejected: %#v", discovery.Issues())
	}
	required, exists := discovery.RequiredSet()
	if !exists {
		t.Fatal("accepted source-owned discovery has no requirement set")
	}
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractEntitySetEnumeration,
		"haft.rule.project-entities/v1",
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractKindDefinedness,
		"haft.rule.project-concern-defined/v1",
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleEvaluator,
		RuntimeMechanismContractMemberOf,
		"haft.rule.project-concern-member/v1",
	)
	assertCompositeRuntimeRequirement(
		t,
		required,
		RuntimeMechanismRoleCarrierMembership,
		RuntimeMechanismContractCarrierMembershipDelivery,
		compositeRuntimeAdapterRule,
	)

	first := required.CanonicalBytes()
	second, secondExists := discovery.RequiredSet()
	if !secondExists || !bytes.Equal(first, second.CanonicalBytes()) {
		t.Fatal("source-owned requirement discovery is not deterministic")
	}
	first[0] ^= 0xff
	if bytes.Equal(first, second.CanonicalBytes()) {
		t.Fatal("source-owned discovery returned shared canonical storage")
	}
}

func TestCompositeRuntimeRequirementDiscoveryRejectsUnverifiedSourceClosure(
	t *testing.T,
) {
	t.Parallel()

	base := loadBaseArtifact(t)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(base, nil))
	linked.canonical = append([]byte(nil), linked.canonical...)
	linked.canonical[0] ^= 0xff

	discovery := DiscoverProjectTypeEnvCompositeRuntimeRequirements(base, linked)
	if !discovery.Rejected() {
		t.Fatal("forged linked B/E source produced runtime requirements")
	}
	if len(discovery.Issues()) != 1 ||
		discovery.Issues()[0].Code() != CompositeLoweringIssueLinkedInvalid {
		t.Fatalf("forged source issues = %#v", discovery.Issues())
	}
}
