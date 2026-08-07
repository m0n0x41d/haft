package specmigrationv2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestPacketCarrierCanonicalRoundTripPreservesDomainAndBasis(t *testing.T) {
	packet := packetCarrierFixturePacket(t, false)
	basis := packetCarrierFixtureBasis(t, "semantic-zero-pass-a")

	carrier, err := specmigrationv2.FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	decoded, err := specmigrationv2.DecodePacketCarrier(carrier.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodePacketCarrier: %v", err)
	}

	computedPacketDigest, err := specmigrationv2.PacketDigestOf(decoded.Packet())
	if err != nil {
		t.Fatalf("PacketDigestOf: %v", err)
	}
	if !computedPacketDigest.Equal(decoded.PacketDigest()) {
		t.Fatalf("decoded packet digest = %q, recomputed = %q", decoded.PacketDigest().String(), computedPacketDigest.String())
	}
	if !carrier.CarrierDigest().Equal(decoded.CarrierDigest()) {
		t.Fatalf("carrier digest changed across canonical round trip")
	}
	if !bytes.Equal(carrier.CanonicalBytes(), decoded.CanonicalBytes()) {
		t.Fatalf("canonical bytes changed across round trip")
	}
	if len(decoded.ReviewBasis().CarrierDigests().Values()) != 3 {
		t.Fatalf("review carrier digests = %d, want 3", len(decoded.ReviewBasis().CarrierDigests().Values()))
	}
	if decoded.ReviewBasis().SemanticZeroPass().Carrier().String() != ".context/review/semantic-zero-pass.md" {
		t.Fatalf("semantic zero-pass carrier = %q", decoded.ReviewBasis().SemanticZeroPass().Carrier().String())
	}
}

func TestPacketCarrierCanonicalizesPermutationSets(t *testing.T) {
	packet := packetCarrierFixturePacket(t, false)
	firstBasis := packetCarrierFixtureBasisWithOrder(t, false)
	secondBasis := packetCarrierFixtureBasisWithOrder(t, true)

	first, err := specmigrationv2.FinalizePacketCarrier(packet, firstBasis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier first: %v", err)
	}
	second, err := specmigrationv2.FinalizePacketCarrier(packet, secondBasis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier second: %v", err)
	}

	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatalf("canonical carrier changed when review basis set order changed")
	}
	if !first.CarrierDigest().Equal(second.CarrierDigest()) {
		t.Fatalf("carrier digest changed when review basis set order changed")
	}
}

func TestPacketCarrierDigestCoversReviewBasisSeparatelyFromPacketDigest(t *testing.T) {
	packet := packetCarrierFixturePacket(t, false)
	firstBasis := packetCarrierFixtureBasis(t, "semantic-zero-pass-a")
	secondBasis := packetCarrierFixtureBasis(t, "semantic-zero-pass-b")

	first, err := specmigrationv2.FinalizePacketCarrier(packet, firstBasis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier first: %v", err)
	}
	second, err := specmigrationv2.FinalizePacketCarrier(packet, secondBasis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier second: %v", err)
	}

	if !first.PacketDigest().Equal(second.PacketDigest()) {
		t.Fatalf("same domain packet produced different packet digests")
	}
	if first.CarrierDigest().Equal(second.CarrierDigest()) {
		t.Fatalf("different review bases produced equal carrier digests")
	}
}

func TestPacketCarrierRoundTripsWorkingTreeProvenance(t *testing.T) {
	packet := packetCarrierFixturePacket(t, true)
	basis := packetCarrierFixtureBasis(t, "semantic-zero-pass")

	carrier, err := specmigrationv2.FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	decoded, err := specmigrationv2.DecodePacketCarrier(carrier.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodePacketCarrier: %v", err)
	}
	origin, ok := decoded.Packet().Source().Provenance().Origin().(specmigrationv2.WorkingTreeEdition)
	if !ok {
		t.Fatalf("decoded provenance origin = %T, want WorkingTreeEdition", decoded.Packet().Source().Provenance().Origin())
	}
	if origin.Delta().Format() != specmigrationv2.WorktreeDeltaGitBinaryV1 {
		t.Fatalf("working-tree delta format = %q", origin.Delta().Format())
	}
}

func TestPacketCarrierRejectsNonCanonicalAndStructurallyAmbiguousJSON(t *testing.T) {
	packet := packetCarrierFixturePacket(t, false)
	basis := packetCarrierFixtureBasis(t, "semantic-zero-pass")
	carrier, err := specmigrationv2.FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	canonical := carrier.CanonicalBytes()

	duplicate := bytes.Replace(
		canonical,
		[]byte(`"kind":"non_binding_final_candidate"`),
		[]byte(`"kind":"non_binding_final_candidate","kind":"non_binding_final_candidate"`),
		1,
	)
	unknown := bytes.Replace(canonical, []byte(`{"schema":`), []byte(`{"unknown":true,"schema":`), 1)
	nonCanonicalWhitespace := append([]byte("\n"), canonical...)
	multipleRoots := append(append([]byte{}, canonical...), []byte(`{}`)...)
	unionFields := bytes.Replace(
		canonical,
		[]byte(`"kind":"map_one","target_claim_ids"`),
		[]byte(`"kind":"map_one","reason":"not allowed","target_claim_ids"`),
		1,
	)

	cases := map[string][]byte{
		"duplicate key":       duplicate,
		"unknown field":       unknown,
		"noncanonical bytes":  nonCanonicalWhitespace,
		"multiple roots":      multipleRoots,
		"mixed union variant": unionFields,
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := specmigrationv2.DecodePacketCarrier(value); err == nil {
				t.Fatalf("DecodePacketCarrier accepted %s", name)
			}
		})
	}
}

func TestPacketCarrierRejectsDeclaredPacketDigestMismatch(t *testing.T) {
	packet := packetCarrierFixturePacket(t, false)
	basis := packetCarrierFixtureBasis(t, "semantic-zero-pass")
	carrier, err := specmigrationv2.FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	canonical := carrier.CanonicalBytes()
	replacement := "sha256:" + strings.Repeat("0", 64)
	tampered := bytes.Replace(canonical, []byte(carrier.PacketDigest().String()), []byte(replacement), 1)

	if _, err := specmigrationv2.DecodePacketCarrier(tampered); err == nil {
		t.Fatalf("DecodePacketCarrier accepted mismatched declared packet digest")
	}
}

func TestPacketCarrierRequiresAbsoluteProvenanceRoot(t *testing.T) {
	fixture := newFixture(t)
	basis := packetCarrierFixtureBasis(t, "semantic-zero-pass")

	if _, err := specmigrationv2.FinalizePacketCarrier(fixture.packet, basis); err == nil {
		t.Fatalf("FinalizePacketCarrier accepted non-absolute provenance project root")
	}
}

func TestFinalCandidateReviewBasisRejectsInvalidExactSets(t *testing.T) {
	valid := packetCarrierFixtureBasisInput(t, "semantic-zero-pass")

	missingRole := valid
	missingRole.CarrierDigests = missingRole.CarrierDigests[:2]
	if _, err := specmigrationv2.NewFinalCandidateReviewBasis(missingRole); err == nil {
		t.Fatalf("NewFinalCandidateReviewBasis accepted fewer than three carrier roles")
	}

	duplicateLifecycle := valid
	duplicateLifecycle.LifecycleIntent = append(
		duplicateLifecycle.LifecycleIntent,
		specmigrationv2.LifecycleIntentInput{
			SectionRef: "SS.alpha.001",
			Operation:  specmigrationv2.LifecycleRebaseline,
		},
	)
	if _, err := specmigrationv2.NewFinalCandidateReviewBasis(duplicateLifecycle); err == nil {
		t.Fatalf("NewFinalCandidateReviewBasis accepted two operations for one section")
	}

	reopen := valid
	reopen.LifecycleIntent[0].Operation = specmigrationv2.LifecycleReopen
	if _, err := specmigrationv2.NewFinalCandidateReviewBasis(reopen); err == nil {
		t.Fatalf("NewFinalCandidateReviewBasis accepted reopen operation")
	}
}

func packetCarrierFixturePacket(t *testing.T, workingTree bool) specmigrationv2.Packet {
	t.Helper()
	fixture := newFixture(t)
	digest := specmigrationv2.SourceDigestOf(fixture.sourceBytes)
	provenance := mustRepositoryProvenance(
		t,
		t.TempDir(),
		fixture.sourceCarrier,
		digest,
		"review:packet-carrier-fixture",
	)
	if workingTree {
		provenance = packetCarrierWorkingTreeProvenance(t, fixture.sourceCarrier, digest)
	}
	source := mustSourceManifestWithProvenance(
		t,
		fixture.sourceCarrier,
		fixture.archiveCarrier,
		fixture.sourceBytes,
		fixture.sections,
		provenance,
	)
	return mustPacket(t, source, fixture.target, fixture.registry, fixture.dispositions)
}

func packetCarrierWorkingTreeProvenance(
	t *testing.T,
	carrier specmigrationv2.SourceCarrierID,
	designatedDigest specmigrationv2.SourceDigest,
) specmigrationv2.DesignatedSourceProvenance {
	t.Helper()
	projectRoot, err := specmigrationv2.NewProjectRootRef(t.TempDir())
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	commit, err := specmigrationv2.NewGitCommitOID("sha1:" + strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	parentDigest := specmigrationv2.SourceDigestOf([]byte("parent source bytes"))
	parent, err := specmigrationv2.NewRepositoryEdition(projectRoot, commit, carrier, parentDigest)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	deltaDigest := specmigrationv2.WorktreeDeltaDigestOf([]byte("git binary diff"))
	delta, err := specmigrationv2.NewWorktreeDeltaBinding(
		specmigrationv2.WorktreeDeltaGitBinaryV1,
		deltaDigest,
	)
	if err != nil {
		t.Fatalf("NewWorktreeDeltaBinding: %v", err)
	}
	edition, err := specmigrationv2.NewWorkingTreeEdition(parent, designatedDigest, delta)
	if err != nil {
		t.Fatalf("NewWorkingTreeEdition: %v", err)
	}
	recordRef, err := specmigrationv2.NewProvenanceRecordRef("review:working-tree-packet-carrier-fixture")
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	recordDigest := specmigrationv2.ProvenanceRecordDigestOf([]byte("working-tree designation record"))
	record, err := specmigrationv2.NewProvenanceRecordBinding(recordRef, recordDigest)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	provenance, err := specmigrationv2.NewDesignatedSourceProvenance(edition, record)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}
	return provenance
}

func packetCarrierFixtureBasis(
	t *testing.T,
	semanticMaterial string,
) specmigrationv2.FinalCandidateReviewBasis {
	t.Helper()
	input := packetCarrierFixtureBasisInput(t, semanticMaterial)
	basis, err := specmigrationv2.NewFinalCandidateReviewBasis(input)
	if err != nil {
		t.Fatalf("NewFinalCandidateReviewBasis: %v", err)
	}
	return basis
}

func packetCarrierFixtureBasisWithOrder(
	t *testing.T,
	reversed bool,
) specmigrationv2.FinalCandidateReviewBasis {
	t.Helper()
	input := packetCarrierFixtureBasisInput(t, "semantic-zero-pass")
	if reversed {
		input.CarrierDigests[0], input.CarrierDigests[2] = input.CarrierDigests[2], input.CarrierDigests[0]
		input.LifecycleIntent[0], input.LifecycleIntent[2] = input.LifecycleIntent[2], input.LifecycleIntent[0]
	}
	basis, err := specmigrationv2.NewFinalCandidateReviewBasis(input)
	if err != nil {
		t.Fatalf("NewFinalCandidateReviewBasis: %v", err)
	}
	return basis
}

func packetCarrierFixtureBasisInput(
	t *testing.T,
	semanticMaterial string,
) specmigrationv2.FinalCandidateReviewBasisInput {
	t.Helper()
	targetSystemCarrier := mustTargetCarrierID(t, ".context/review/target-system.md")
	softwareSystemCarrier := mustTargetCarrierID(t, ".context/review/software-system.md")
	termMapCarrier := mustTargetCarrierID(t, ".context/review/term-map.md")
	semanticCarrier := mustTargetCarrierID(t, ".context/review/semantic-zero-pass.md")
	return specmigrationv2.FinalCandidateReviewBasisInput{
		CarrierDigests: []specmigrationv2.ReviewCarrierDigestInput{
			{
				Role:    specmigrationv2.ReviewTermMapCarrier,
				Carrier: termMapCarrier,
				Digest:  specmigrationv2.DigestBytes([]byte("term map")),
			},
			{
				Role:    specmigrationv2.ReviewTargetSystemCarrier,
				Carrier: targetSystemCarrier,
				Digest:  specmigrationv2.DigestBytes([]byte("target system")),
			},
			{
				Role:    specmigrationv2.ReviewSoftwareSystemCarrier,
				Carrier: softwareSystemCarrier,
				Digest:  specmigrationv2.DigestBytes([]byte("software system")),
			},
		},
		FPFRevision: strings.Repeat("a", 40),
		SemanticZeroPass: specmigrationv2.SemanticZeroPassInput{
			Carrier: semanticCarrier,
			Digest:  specmigrationv2.DigestBytes([]byte(semanticMaterial)),
		},
		LifecycleIntent: []specmigrationv2.LifecycleIntentInput{
			{SectionRef: "SS.alpha.001", Operation: specmigrationv2.LifecycleActivate},
			{SectionRef: "TS.environment.001", Operation: specmigrationv2.LifecycleRebaseline},
			{SectionRef: "TS.role.001", Operation: specmigrationv2.LifecycleActivate},
		},
	}
}
