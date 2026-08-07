package specmigrationv2

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

type applyE2EFixture struct {
	root           ApplyProjectRoot
	structural     StructuralRequest
	packetDigest   PacketDigest
	sourceCarrier  SourceCarrierID
	targetCarrier  TargetCarrierID
	archiveCarrier ArchiveCarrierID
	sourceBytes    []byte
	targetBytes    []byte
}

func TestNewApplyRequestToApplyUsesSQLiteRequiredAndSeparateReviewDraft(t *testing.T) {
	ctx := context.Background()
	rootPath, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	fixture := newApplyE2EFixture(t, rootPath)

	profileHarness := profileadmissionfixture.New(t, rootPath)
	profileHarness.AdmitSoftwareRevision(t, "spec-migration-e2e")
	profileService, err := profileadmissionsqlite.NewService(profileHarness.Database())
	if err != nil {
		t.Fatalf("profile admission service: %v", err)
	}
	applicability := profileService.ResolveSoftwareSystemSpecMigration(ctx, profileHarness.Root())
	required, ok := applicability.Required()
	if !ok {
		t.Fatalf("migration applicability = %q, want required", applicability.Kind())
	}

	review := effectReviewFixture(
		t,
		fixture.root,
		fixture.packetDigest,
		SourceDigestOf(fixture.sourceBytes),
		fixture.targetBytes,
		true,
	)
	softwareReview, found := softwareReviewBinding(review)
	if !found {
		t.Fatal("admitted review has no SoftwareSystemSpec source binding")
	}
	if softwareReview.carrier.String() == fixture.targetCarrier.String() {
		t.Fatal("review source carrier must remain distinct from the final packet target")
	}
	finalTargetPath := filepath.Join(
		rootPath,
		filepath.FromSlash(fixture.targetCarrier.String()),
	)
	reviewDraftPath := filepath.Join(rootPath, ".context", "review-software-system.md")
	if _, statErr := os.Lstat(finalTargetPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("final target must be absent before apply: %v", statErr)
	}
	assertFileBytes(t, reviewDraftPath, fixture.targetBytes)

	request, err := NewApplyRequest(ctx, profileService, ApplyRequestInput{
		ProjectRoot:          fixture.root,
		Structural:           fixture.structural,
		ProfileApplicability: required,
		Review:               review,
		RequestedAt:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewApplyRequest: %v", err)
	}
	result := ApplyMigration(ctx, profileService, request)
	applied, ok := result.(Applied)
	if !ok {
		if recovery, recoveryRequired := result.(RecoveryRequired); recoveryRequired {
			t.Fatalf("ApplyMigration = RecoveryRequired(%s): %s", recovery.Phase(), recovery.Reason())
		}
		t.Fatalf("ApplyMigration = %T, want Applied", result)
	}

	archivePath := filepath.Join(
		rootPath,
		filepath.FromSlash(fixture.archiveCarrier.String()),
	)
	sourcePath := filepath.Join(
		rootPath,
		filepath.FromSlash(fixture.sourceCarrier.String()),
	)
	assertFileBytes(t, finalTargetPath, fixture.targetBytes)
	assertFileBytes(t, reviewDraftPath, fixture.targetBytes)
	assertFileBytes(t, archivePath, fixture.sourceBytes)
	if _, statErr := os.Lstat(sourcePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("source must be archived after apply: %v", statErr)
	}

	receipt := applied.Receipt()
	if receipt.OpaqueProfileBindingRef() != required.AdmissionRecordRef().String() {
		t.Fatal("receipt does not bind the exact SQLite admission ref")
	}
	if receipt.OpaqueProfileBindingDigest() != required.AdmissionRecordDigest().String() {
		t.Fatal("receipt does not bind the exact SQLite admission digest")
	}
	if receipt.OpaqueProfileLedgerRevision() != required.LedgerRevision().Value() {
		t.Fatal("receipt does not bind the exact SQLite ledger revision")
	}
}

func TestRecoverCompletedMigrationExposesExactDurableReceiptCarrier(t *testing.T) {
	ctx := context.Background()
	fixture := newReviewAdmissionFixture(t)
	profileHarness := profileadmissionfixture.New(t, fixture.root.String())
	profileHarness.AdmitSoftwareRevision(t, "completed-recovery-receipt-carrier")
	profileService, err := profileadmissionsqlite.NewService(profileHarness.Database())
	if err != nil {
		t.Fatalf("profile admission service: %v", err)
	}
	reviewService, err := NewReviewAdmissionService(profileHarness.Database())
	if err != nil {
		t.Fatalf("review admission service: %v", err)
	}
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}
	startedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	observedAt := startedAt.Add(time.Nanosecond)
	endedAt := observedAt.Add(time.Nanosecond)
	source, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared.state.manualSource,
		startedAt,
		observedAt,
		endedAt,
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	review, err := reviewService.Admit(ctx, prepared, source)
	if err != nil {
		t.Fatalf("admit migration review: %v", err)
	}
	applicability := profileService.ResolveSoftwareSystemSpecMigration(
		ctx,
		profileHarness.Root(),
	)
	required, ok := applicability.Required()
	if !ok {
		t.Fatalf("migration applicability = %q, want required", applicability.Kind())
	}
	request, err := NewApplyRequest(ctx, profileService, ApplyRequestInput{
		ProjectRoot:          fixture.root,
		Structural:           fixture.structural,
		ProfileApplicability: required,
		Review:               review,
		RequestedAt:          endedAt.Add(time.Nanosecond),
	})
	if err != nil {
		t.Fatalf("NewApplyRequest: %v", err)
	}
	appliedResult, ok := ApplyMigration(ctx, profileService, request).(Applied)
	if !ok {
		t.Fatal("ApplyMigration did not complete")
	}
	recoveryRequest, err := NewRecoveryRequest(RecoveryRequestInput{
		ProjectRoot: fixture.root,
		Structural:  fixture.structural,
	})
	if err != nil {
		t.Fatalf("NewRecoveryRequest: %v", err)
	}
	replayedResult, ok := RecoverMigration(
		ctx,
		profileService,
		reviewService,
		recoveryRequest,
	).(Replayed)
	if !ok {
		t.Fatal("RecoverMigration did not replay the completed effect")
	}
	appliedCarrier := appliedResult.ReceiptCarrier()
	replayedCarrier := replayedResult.ReceiptCarrier()
	if appliedCarrier.Ref().String() != replayedCarrier.Ref().String() {
		t.Fatalf(
			"recovered receipt carrier ref = %q, want %q",
			replayedCarrier.Ref().String(),
			appliedCarrier.Ref().String(),
		)
	}
	if !appliedCarrier.Digest().Equal(replayedCarrier.Digest()) {
		t.Fatalf(
			"recovered receipt carrier digest = %q, want %q",
			replayedCarrier.Digest().String(),
			appliedCarrier.Digest().String(),
		)
	}
	if filepath.IsAbs(replayedCarrier.Ref().String()) {
		t.Fatalf("receipt carrier ref is absolute: %q", replayedCarrier.Ref().String())
	}
	if !strings.HasPrefix(replayedCarrier.Ref().String(), ".haft/spec-migration-v2.") {
		t.Fatalf("receipt carrier ref is outside the migration namespace: %q", replayedCarrier.Ref().String())
	}
	if !strings.HasSuffix(replayedCarrier.Ref().String(), ".receipt.json") {
		t.Fatalf("receipt carrier ref does not name a receipt: %q", replayedCarrier.Ref().String())
	}
}

func newApplyE2EFixture(t *testing.T, rootPath string) applyE2EFixture {
	t.Helper()
	sourceBytes := []byte("## ES.alpha.001 Alpha\n\n```yaml spec-section\nid: ES.alpha.001\nspec: enabling-system\nkind: enabling.alpha\nstatus: draft\n```\n")
	targetBytes := []byte("# Exact software-system target\n\n## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: The migrated behavior is preserved.\n    scope:\n      - migration\n```\n")
	recordCarrier := ".context/source-designation.md"
	recordBytes := []byte("operator designated the committed enabling-system source\n")

	runApplyE2EGit(t, rootPath, "init")
	runApplyE2EGit(t, rootPath, "config", "user.email", "test@example.com")
	runApplyE2EGit(t, rootPath, "config", "user.name", "Haft Test")

	sourceCarrier, err := NewSourceCarrierID(".haft/specs/enabling-system.md")
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	targetCarrier, err := NewTargetCarrierID(".haft/specs/software-system.md")
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	archiveCarrier, err := NewArchiveCarrierID(".haft/migration-archive/enabling-system.md")
	if err != nil {
		t.Fatalf("NewArchiveCarrierID: %v", err)
	}
	sourcePath := filepath.Join(rootPath, filepath.FromSlash(sourceCarrier.String()))
	recordPath := filepath.Join(rootPath, filepath.FromSlash(recordCarrier))
	writeFixtureFile(t, sourcePath, sourceBytes)
	writeFixtureFile(t, recordPath, recordBytes)
	runApplyE2EGit(t, rootPath, "add", "-f", "--", sourceCarrier.String(), recordCarrier)
	runApplyE2EGit(t, rootPath, "commit", "-m", "designate migration source")

	root, err := NewApplyProjectRoot(rootPath)
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	projectRef, err := NewProjectRootRef(rootPath)
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	head := strings.TrimSpace(string(runApplyE2EGit(t, rootPath, "rev-parse", "HEAD")))
	objectFormat := strings.TrimSpace(string(runApplyE2EGit(t, rootPath, "rev-parse", "--show-object-format")))
	commit, err := NewGitCommitOID(objectFormat + ":" + head)
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	edition, err := NewRepositoryEdition(
		projectRef,
		commit,
		sourceCarrier,
		SourceDigestOf(sourceBytes),
	)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	recordRef, err := NewProvenanceRecordRef(recordCarrier)
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	recordBinding, err := NewProvenanceRecordBinding(
		recordRef,
		ProvenanceRecordDigestOf(recordBytes),
	)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	provenance, err := NewDesignatedSourceProvenance(edition, recordBinding)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}

	sections, err := deriveSourceSections(sourceBytes)
	if err != nil {
		t.Fatalf("deriveSourceSections: %v", err)
	}
	sourceLength, err := NewByteLength(uint64(len(sourceBytes)))
	if err != nil {
		t.Fatalf("source NewByteLength: %v", err)
	}
	archive, err := NewArchiveManifest(archiveCarrier, SourceDigestOf(sourceBytes))
	if err != nil {
		t.Fatalf("NewArchiveManifest: %v", err)
	}
	source, err := NewSourceManifest(SourceManifestInput{
		Carrier:    sourceCarrier,
		Digest:     SourceDigestOf(sourceBytes),
		ByteLength: sourceLength,
		Archive:    archive,
		Provenance: provenance,
		Sections:   sections,
	})
	if err != nil {
		t.Fatalf("NewSourceManifest: %v", err)
	}

	targetLength, err := NewByteLength(uint64(len(targetBytes)))
	if err != nil {
		t.Fatalf("target NewByteLength: %v", err)
	}
	target, err := NewTargetManifest(TargetManifestInput{
		Carrier:    targetCarrier,
		Digest:     TargetDigestOf(targetBytes),
		ByteLength: targetLength,
	})
	if err != nil {
		t.Fatalf("NewTargetManifest: %v", err)
	}
	claim, err := NewTargetAtomicClaimID("SS.alpha.001.L1")
	if err != nil {
		t.Fatalf("NewTargetAtomicClaimID: %v", err)
	}
	claimSet, err := NewTargetClaimSet([]TargetAtomicClaimID{claim})
	if err != nil {
		t.Fatalf("NewTargetClaimSet: %v", err)
	}
	mapping, err := NewMapOne(claimSet)
	if err != nil {
		t.Fatalf("NewMapOne: %v", err)
	}
	disposition, err := NewSourceDisposition(sections[0].ID(), mapping)
	if err != nil {
		t.Fatalf("NewSourceDisposition: %v", err)
	}
	outsideRegistry, err := NewOutsideCarrierRegistry(nil)
	if err != nil {
		t.Fatalf("NewOutsideCarrierRegistry: %v", err)
	}
	packetID, err := NewMigrationPacketID("migration:sqlite-review-source-e2e")
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	packet, err := NewPacket(PacketInput{
		ID:                 packetID,
		SchemaVersion:      SchemaVersionV2,
		Source:             source,
		Target:             target,
		OutsideRegistry:    outsideRegistry,
		SourceDispositions: []SourceDisposition{disposition},
	})
	if err != nil {
		t.Fatalf("NewPacket: %v", err)
	}
	packetDigest, err := PacketDigestOf(packet)
	if err != nil {
		t.Fatalf("PacketDigestOf: %v", err)
	}

	sourceSnapshot, err := NewSourceSnapshot(SourceSnapshotInput{
		Carrier: sourceCarrier,
		Bytes:   sourceBytes,
	})
	if err != nil {
		t.Fatalf("NewSourceSnapshot: %v", err)
	}
	targetSnapshot, err := NewTargetSnapshot(TargetSnapshotInput{
		Carrier: targetCarrier,
		Bytes:   targetBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetSnapshot: %v", err)
	}
	targetClaims, err := NewTargetClaimCatalog(TargetClaimCatalogInput{
		Carrier: targetCarrier,
		Bytes:   targetBytes,
	})
	if err != nil {
		t.Fatalf("NewTargetClaimCatalog: %v", err)
	}
	outsideSnapshots, err := NewOutsideCarrierSnapshots(nil)
	if err != nil {
		t.Fatalf("NewOutsideCarrierSnapshots: %v", err)
	}
	structural, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           packet,
		ProjectRoot:      projectRef,
		Source:           sourceSnapshot,
		Target:           targetSnapshot,
		TargetClaims:     targetClaims,
		OutsideSnapshots: outsideSnapshots,
	})
	if err != nil {
		t.Fatalf("NewStructuralRequest: %v", err)
	}

	return applyE2EFixture{
		root:           root,
		structural:     structural,
		packetDigest:   packetDigest,
		sourceCarrier:  sourceCarrier,
		targetCarrier:  targetCarrier,
		archiveCarrier: archiveCarrier,
		sourceBytes:    sourceBytes,
		targetBytes:    targetBytes,
	}
}

func runApplyE2EGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return output
}
