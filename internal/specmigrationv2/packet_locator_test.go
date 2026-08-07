package specmigrationv2

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLocateFinalCandidatePacketsReturnsTypedZeroAndOne(t *testing.T) {
	root := mustPacketLocatorRoot(t)

	empty, err := LocateFinalCandidatePackets(root)
	if err != nil {
		t.Fatalf("locate absent packet store: %v", err)
	}
	if _, ok := empty.(NoFinalCandidatePackets); !ok {
		t.Fatalf("absent store result = %T, want NoFinalCandidatePackets", empty)
	}

	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	carrierRef := writePacketLocatorCandidate(t, root, fixture.candidate)
	found, err := LocateFinalCandidatePackets(root)
	if err != nil {
		t.Fatalf("locate one packet: %v", err)
	}
	one, ok := found.(OneFinalCandidatePacket)
	if !ok {
		t.Fatalf("one-packet result = %T, want OneFinalCandidatePacket", found)
	}
	candidate := one.Candidate()
	if candidate.CarrierRef() != carrierRef {
		t.Fatalf("carrier ref = %q, want %q", candidate.CarrierRef(), carrierRef)
	}
	if candidate.PacketID().String() != fixture.packet.ID().String() {
		t.Fatalf("packet ID = %q, want %q", candidate.PacketID().String(), fixture.packet.ID().String())
	}
	if !candidate.PacketDigest().Equal(fixture.candidate.PacketDigest()) {
		t.Fatal("located packet digest does not match exact candidate")
	}
	if !candidate.CarrierDigest().Equal(fixture.candidate.CarrierDigest()) {
		t.Fatal("located carrier digest does not match exact candidate")
	}
}

func TestLocateFinalCandidatePacketsReturnsDeterministicManyWithoutSelection(t *testing.T) {
	root := mustPacketLocatorRoot(t)
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	zeta := packetLocatorCandidateWithID(t, fixture, "zeta-migration")
	alpha := packetLocatorCandidateWithID(t, fixture, "alpha-migration")
	writePacketLocatorCandidate(t, root, zeta)
	writePacketLocatorCandidate(t, root, alpha)

	result, err := LocateFinalCandidatePackets(root)
	if err != nil {
		t.Fatalf("locate multiple packets: %v", err)
	}
	many, ok := result.(ManyFinalCandidatePackets)
	if !ok {
		t.Fatalf("many-packet result = %T, want ManyFinalCandidatePackets", result)
	}
	candidates := many.Candidates()
	if len(candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(candidates))
	}
	ids := []string{
		candidates[0].PacketID().String(),
		candidates[1].PacketID().String(),
	}
	want := []string{"alpha-migration", "zeta-migration"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("candidate order = %v, want %v", ids, want)
	}

	second, err := LocateFinalCandidatePackets(root)
	if err != nil {
		t.Fatalf("repeat multiple-packet discovery: %v", err)
	}
	secondMany, ok := second.(ManyFinalCandidatePackets)
	if !ok {
		t.Fatalf("repeat result = %T, want ManyFinalCandidatePackets", second)
	}
	secondIDs := []string{
		secondMany.Candidates()[0].PacketID().String(),
		secondMany.Candidates()[1].PacketID().String(),
	}
	if !reflect.DeepEqual(secondIDs, want) {
		t.Fatalf("repeat candidate order = %v, want %v", secondIDs, want)
	}
}

func TestLocateFinalCandidatePacketsRejectsUnsafeAndInvalidEntriesDeterministically(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("packet locator no-follow filesystem support is unavailable on Windows")
	}
	root := mustPacketLocatorRoot(t)
	storePath := packetLocatorStorePath(root)
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("create packet store: %v", err)
	}
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})
	validBytes := fixture.candidate.CanonicalBytes()

	symlinkName := strings.Repeat("a", 64) + ".json"
	symlinkPath := filepath.Join(storePath, symlinkName)
	symlinkTarget := filepath.Join(root.String(), "outside-packet.json")
	if err := os.WriteFile(symlinkTarget, validBytes, 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatalf("create packet symlink: %v", err)
	}

	directoryName := strings.Repeat("b", 64) + ".json"
	if err := os.Mkdir(filepath.Join(storePath, directoryName), 0o755); err != nil {
		t.Fatalf("create non-regular packet entry: %v", err)
	}

	wrongName := strings.Repeat("c", 64) + ".json"
	if err := os.WriteFile(filepath.Join(storePath, wrongName), validBytes, 0o600); err != nil {
		t.Fatalf("write digest-name mismatch: %v", err)
	}

	malformedName := strings.Repeat("d", 64) + ".json"
	if err := os.WriteFile(filepath.Join(storePath, malformedName), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write malformed packet: %v", err)
	}

	invalidName := "packet.json"
	if err := os.WriteFile(filepath.Join(storePath, invalidName), validBytes, 0o600); err != nil {
		t.Fatalf("write invalid-name packet: %v", err)
	}

	first := locatePacketStoreFailure(t, root)
	second := locatePacketStoreFailure(t, root)
	if first.Error() != second.Error() {
		t.Fatalf("diagnostic rendering is nondeterministic:\nfirst: %s\nsecond: %s", first, second)
	}
	if !reflect.DeepEqual(first.Diagnostics(), second.Diagnostics()) {
		t.Fatalf("diagnostic order is nondeterministic: first=%v second=%v", first.Diagnostics(), second.Diagnostics())
	}

	wantCodes := map[string]PacketLocatorDiagnosticCode{
		symlinkName:   PacketLocatorSymlink,
		directoryName: PacketLocatorNotRegular,
		wrongName:     PacketLocatorDigestNameMismatch,
		malformedName: PacketLocatorDecodeFailed,
		invalidName:   PacketLocatorInvalidFilename,
	}
	diagnostics := first.Diagnostics()
	if len(diagnostics) != len(wantCodes) {
		t.Fatalf("diagnostic count = %d, want %d: %v", len(diagnostics), len(wantCodes), diagnostics)
	}
	for _, diagnostic := range diagnostics {
		name := filepath.Base(filepath.FromSlash(diagnostic.CarrierRef()))
		want, found := wantCodes[name]
		if !found {
			t.Fatalf("unexpected diagnostic carrier %q", diagnostic.CarrierRef())
		}
		if diagnostic.Code() != want {
			t.Fatalf("diagnostic %s code = %q, want %q", name, diagnostic.Code(), want)
		}
	}
}

func TestLocateFinalCandidatePacketsRejectsSymlinkStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("packet locator no-follow filesystem support is unavailable on Windows")
	}
	root := mustPacketLocatorRoot(t)
	storePath := packetLocatorStorePath(root)
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatalf("create packet store parent: %v", err)
	}
	target := filepath.Join(root.String(), "packet-store-target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create packet store symlink target: %v", err)
	}
	if err := os.Symlink(target, storePath); err != nil {
		t.Fatalf("create packet store symlink: %v", err)
	}

	failure := locatePacketStoreFailure(t, root)
	diagnostics := failure.Diagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Code() != PacketLocatorSymlink {
		t.Fatalf("symlink store diagnostics = %v", diagnostics)
	}
}

func TestLocateFinalCandidatePacketsRejectsSymlinkStoreAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("packet locator no-follow filesystem support is unavailable on Windows")
	}
	root := mustPacketLocatorRoot(t)
	external := t.TempDir()
	externalStore := filepath.Join(external, "spec-migration-v2", "packets")
	if err := os.MkdirAll(externalStore, 0o755); err != nil {
		t.Fatalf("create external packet store: %v", err)
	}
	if err := os.Symlink(external, filepath.Join(root.String(), ".haft")); err != nil {
		t.Fatalf("create packet store ancestor symlink: %v", err)
	}

	failure := locatePacketStoreFailure(t, root)
	diagnostics := failure.Diagnostics()
	if len(diagnostics) != 1 || diagnostics[0].Code() != PacketLocatorUnsafeTopology {
		t.Fatalf("symlink ancestor diagnostics = %v", diagnostics)
	}
}

func TestEffectJournalCarrierRefUsesCanonicalEffectPathDerivation(t *testing.T) {
	root := mustPacketLocatorRoot(t)
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})

	carrierRef, err := EffectJournalCarrierRef(root, fixture.packet)
	if err != nil {
		t.Fatalf("derive effect journal carrier ref: %v", err)
	}
	packetID := fixture.packet.ID().String()
	key := sha256.Sum256([]byte(packetID))
	want := fmt.Sprintf(".haft/spec-migration-v2.%x.journal.json", key)
	if carrierRef != want {
		t.Fatalf("journal carrier ref = %q, want %q", carrierRef, want)
	}
}

func mustPacketLocatorRoot(t *testing.T) ApplyProjectRoot {
	t.Helper()
	root, err := NewApplyProjectRoot(filepath.Clean(t.TempDir()))
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	return root
}

func packetLocatorStorePath(root ApplyProjectRoot) string {
	return filepath.Join(root.String(), filepath.FromSlash(FinalCandidatePacketStoreRef))
}

func writePacketLocatorCandidate(
	t *testing.T,
	root ApplyProjectRoot,
	carrier FinalCandidatePacketCarrier,
) string {
	t.Helper()
	storePath := packetLocatorStorePath(root)
	if err := os.MkdirAll(storePath, 0o755); err != nil {
		t.Fatalf("create packet store: %v", err)
	}
	name := finalCandidatePacketFilename(carrier)
	path := filepath.Join(storePath, name)
	if err := os.WriteFile(path, carrier.CanonicalBytes(), 0o600); err != nil {
		t.Fatalf("write packet candidate: %v", err)
	}
	return filepath.ToSlash(filepath.Join(FinalCandidatePacketStoreRef, name))
}

func packetLocatorCandidateWithID(
	t *testing.T,
	fixture packetPartitionAuditFixture,
	rawID string,
) FinalCandidatePacketCarrier {
	t.Helper()
	id, err := NewMigrationPacketID(rawID)
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	packet, err := NewPacket(PacketInput{
		ID:                 id,
		SchemaVersion:      fixture.packet.SchemaVersion(),
		Source:             fixture.packet.Source(),
		Target:             fixture.packet.Target(),
		OutsideRegistry:    fixture.packet.OutsideRegistry(),
		SourceDispositions: fixture.packet.SourceDispositions(),
	})
	if err != nil {
		t.Fatalf("NewPacket: %v", err)
	}
	carrier, err := FinalizePacketCarrier(packet, fixture.candidate.ReviewBasis())
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}
	return carrier
}

func locatePacketStoreFailure(
	t *testing.T,
	root ApplyProjectRoot,
) *InvalidFinalCandidatePacketStoreError {
	t.Helper()
	result, err := LocateFinalCandidatePackets(root)
	if result != nil {
		t.Fatalf("invalid packet store returned result %T", result)
	}
	var failure *InvalidFinalCandidatePacketStoreError
	if !errors.As(err, &failure) {
		t.Fatalf("packet store error = %v, want InvalidFinalCandidatePacketStoreError", err)
	}
	return failure
}
