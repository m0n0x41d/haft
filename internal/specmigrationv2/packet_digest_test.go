package specmigrationv2_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestPacketDigestIsPermutationInvariantForTopLevelDispositionSet(t *testing.T) {
	fixture := newFixture(t)
	reordered := []specmigrationv2.SourceDisposition{
		fixture.dispositions[3],
		fixture.dispositions[1],
		fixture.dispositions[0],
		fixture.dispositions[2],
	}
	packet := fixture.packetWith(t, reordered, fixture.registry)

	first := mustPacketDigest(t, fixture.packet)
	second := mustPacketDigest(t, packet)
	if !first.Equal(second) {
		t.Fatalf("permuted packet digest = %s, want %s", second.String(), first.String())
	}
}

func TestPacketDigestDistinguishesSameIDAndCountWithDifferentMappings(t *testing.T) {
	fixture := newFixture(t)
	claim := mustTargetClaimID(t, "SS.alpha.001.D9")
	mapping := mustMapOne(t, []specmigrationv2.TargetAtomicClaimID{claim})
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	dispositions[0] = mustSourceDisposition(t, fixture.sections[0].ID(), mapping)
	packet := fixture.packetWith(t, dispositions, fixture.registry)

	first := mustPacketDigest(t, fixture.packet)
	second := mustPacketDigest(t, packet)
	if first.Equal(second) {
		t.Fatalf("different mappings share packet digest %s", first.String())
	}
}

func TestPacketDigestBindsExactDesignatedSourceProvenance(t *testing.T) {
	fixture := newFixture(t)
	provenance := mustRepositoryProvenance(
		t,
		"project-root:haft",
		fixture.sourceCarrier,
		specmigrationv2.SourceDigestOf(fixture.sourceBytes),
		"review:different-source-designation",
	)
	source := mustSourceManifestWithProvenance(
		t,
		fixture.sourceCarrier,
		fixture.archiveCarrier,
		fixture.sourceBytes,
		fixture.sections,
		provenance,
	)
	packet := mustPacket(t, source, fixture.target, fixture.registry, fixture.dispositions)

	first := mustPacketDigest(t, fixture.packet)
	second := mustPacketDigest(t, packet)
	if first.Equal(second) {
		t.Fatalf("different provenance records share packet digest %s", first.String())
	}
}

func TestLineageDigestIsPermutationInvariantAndMappingSensitive(t *testing.T) {
	fixture := newFixture(t)
	reordered := []specmigrationv2.SourceDisposition{
		fixture.dispositions[2],
		fixture.dispositions[0],
		fixture.dispositions[3],
		fixture.dispositions[1],
	}
	reorderedPacket := fixture.packetWith(t, reordered, fixture.registry)
	first := mustLineageDigest(t, fixture.packet.LineagePolicy())
	second := mustLineageDigest(t, reorderedPacket.LineagePolicy())
	if first.String() != second.String() {
		t.Fatalf("permuted lineage digest = %s, want %s", second.String(), first.String())
	}

	claim := mustTargetClaimID(t, "SS.alpha.001.D9")
	mapping := mustMapOne(t, []specmigrationv2.TargetAtomicClaimID{claim})
	dispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	dispositions[0] = mustSourceDisposition(t, fixture.sections[0].ID(), mapping)
	changedPacket := fixture.packetWith(t, dispositions, fixture.registry)
	changed := mustLineageDigest(t, changedPacket.LineagePolicy())
	if first.String() == changed.String() {
		t.Fatalf("different mappings share lineage digest %s", first.String())
	}
}

func TestLineageDigestBindsHistoryReasonOutsideMeaningAndResolvedRegistry(t *testing.T) {
	fixture := newFixture(t)
	baseline := mustLineageDigest(t, fixture.packet.LineagePolicy())

	reasonDispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	reasonDispositions[2] = mustSourceDisposition(
		t,
		fixture.sections[2].ID(),
		mustRetire(t, "different historical retention reason"),
	)
	reasonPacket := fixture.packetWith(t, reasonDispositions, fixture.registry)
	assertDifferentLineageDigest(t, baseline, reasonPacket, "RetireHistory reason")

	meaningDispositions := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	meaningDispositions[3] = mustSourceDisposition(
		t,
		fixture.sections[3].ID(),
		mustOutside(
			t,
			"different outside-PSS meaning",
			[]specmigrationv2.OutsideCarrierID{fixture.outsideID},
		),
	)
	meaningPacket := fixture.packetWith(t, meaningDispositions, fixture.registry)
	assertDifferentLineageDigest(t, baseline, meaningPacket, "OutsidePSS meaning")

	differentPath := mustSourceCarrierID(t, "docs/host-discipline.md")
	pathRegistration := mustOutsideRegistration(t, fixture.outsideID, differentPath, fixture.outsideBytes)
	pathRegistry, err := specmigrationv2.NewOutsideCarrierRegistry(
		[]specmigrationv2.OutsideCarrierRegistration{pathRegistration},
	)
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	pathPacket := fixture.packetWith(t, fixture.dispositions, pathRegistry)
	assertDifferentLineageDigest(t, baseline, pathPacket, "resolved outside path")

	originalPath := mustSourceCarrierID(t, "AGENTS.md")
	digestRegistration := mustOutsideRegistration(
		t,
		fixture.outsideID,
		originalPath,
		[]byte("different exact outside carrier"),
	)
	digestRegistry, err := specmigrationv2.NewOutsideCarrierRegistry(
		[]specmigrationv2.OutsideCarrierRegistration{digestRegistration},
	)
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	digestPacket := fixture.packetWith(t, fixture.dispositions, digestRegistry)
	assertDifferentLineageDigest(t, baseline, digestPacket, "resolved outside digest")
}

func TestPacketDigestIsPermutationInvariantEvenForDuplicateSourceKeys(t *testing.T) {
	fixture := newFixture(t)
	duplicate := mustSourceDisposition(t, fixture.sections[0].ID(), fixture.mapA)
	firstOrder := append([]specmigrationv2.SourceDisposition{}, fixture.dispositions...)
	firstOrder = append(firstOrder, duplicate)
	secondOrder := []specmigrationv2.SourceDisposition{
		duplicate,
		fixture.dispositions[3],
		fixture.dispositions[0],
		fixture.dispositions[2],
		fixture.dispositions[1],
	}
	firstPacket := fixture.packetWith(t, firstOrder, fixture.registry)
	secondPacket := fixture.packetWith(t, secondOrder, fixture.registry)

	first := mustPacketDigest(t, firstPacket)
	second := mustPacketDigest(t, secondPacket)
	if !first.Equal(second) {
		t.Fatalf("duplicate-key permutation digest = %s, want %s", second.String(), first.String())
	}
}

func mustPacketDigest(t *testing.T, packet specmigrationv2.Packet) specmigrationv2.PacketDigest {
	t.Helper()
	digest, err := specmigrationv2.PacketDigestOf(packet)
	if err != nil {
		t.Fatalf("PacketDigestOf: %v", err)
	}
	return digest
}

func mustLineageDigest(
	t *testing.T,
	policy specmigrationv2.LineagePolicy,
) specmigrationv2.LineagePolicyDigest {
	t.Helper()
	digest, err := specmigrationv2.LineagePolicyDigestOf(policy)
	if err != nil {
		t.Fatalf("LineagePolicyDigestOf: %v", err)
	}
	return digest
}

func assertDifferentLineageDigest(
	t *testing.T,
	baseline specmigrationv2.LineagePolicyDigest,
	packet specmigrationv2.Packet,
	field string,
) {
	t.Helper()
	observed := mustLineageDigest(t, packet.LineagePolicy())
	if baseline.String() == observed.String() {
		t.Fatalf("different %s shares lineage digest %s", field, baseline.String())
	}
}
