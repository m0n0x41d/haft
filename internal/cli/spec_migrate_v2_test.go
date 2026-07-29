package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestSpecMigrationV2UsesReviewBytesUnderFinalTargetIdentity(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, false)

	observation, closeDatabase, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	if err != nil {
		t.Fatalf("observeSpecMigrationV2: %v", err)
	}
	defer closeDatabase()

	if got := observation.structural.target.Carrier().String(); got != fixture.finalTargetCarrier {
		t.Fatalf("target identity = %q, want final install path %q", got, fixture.finalTargetCarrier)
	}
	if got := observation.reviewSoftwareCarrier; got != fixture.reviewSoftwareCarrier {
		t.Fatalf("review software carrier = %q, want %q", got, fixture.reviewSoftwareCarrier)
	}
	if !bytes.Equal(observation.structural.target.Bytes(), fixture.targetBytes) {
		t.Fatal("target snapshot does not contain exact review-basis SoftwareSystemSpec bytes")
	}
	if observation.gitWitness == nil {
		t.Fatal("read-only observation omitted the canonical Git source witness")
	}
	if got := observation.gitWitness.HeadCommit().String(); got != fixture.sourceCommit {
		t.Fatalf("Git witness HEAD = %q, want %q", got, fixture.sourceCommit)
	}
	if _, err := os.Stat(filepath.Join(fixture.root, fixture.finalTargetCarrier)); !os.IsNotExist(err) {
		t.Fatalf("read-only observation wrote final target: %v", err)
	}
}

func TestSpecMigrationV2WithoutCanonicalProfileReturnsUnderdetermined(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, false)
	result, err := executeSpecMigrationV2ForTest(t, fixture, false)
	if err != nil {
		t.Fatalf("runSpecMigrateV2: %v", err)
	}
	if result.State != "underdetermined" {
		t.Fatalf("state = %q, want underdetermined", result.State)
	}
	if len(result.ProfileMissingBasis) != 1 || result.ProfileMissingBasis[0] == "" {
		t.Fatalf("profile missing basis = %#v", result.ProfileMissingBasis)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2WithCurrentSoftwareProfileReturnsPendingReview(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	result, err := executeSpecMigrationV2ForTest(t, fixture, false)
	if err != nil {
		t.Fatalf("runSpecMigrateV2: %v", err)
	}
	if result.State != "pending_review" || result.ProfileApplicability != "required" {
		t.Fatalf("result = %+v, want required/PendingReview", result)
	}
	want := string(specmigrationv2.MissingExactReviewBinding)
	if len(result.ReviewMissingBasis) != 1 || result.ReviewMissingBasis[0] != want {
		t.Fatalf("review missing basis = %#v, want [%s]", result.ReviewMissingBasis, want)
	}
	if result.PartitionAuditStatus != string(specmigrationv2.PacketPartitionAuditVerified) ||
		result.PartitionAuditDigest == "" ||
		result.PartitionAuditCounts.SourceSections != 1 ||
		result.PartitionAuditCounts.TopLevelDispositions != 1 {
		t.Fatalf("partition audit = %+v, want verified exact one-section packet", result)
	}
	if result.ReviewSoftwareCarrier == result.FinalTargetCarrier {
		t.Fatalf("review carrier %q collapsed into final target identity", result.ReviewSoftwareCarrier)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2ReportsOutsideCarrierDriftAsInvalid(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	carrier, err := specmigrationv2.DecodePacketCarrier(fixture.packetBytes)
	if err != nil {
		t.Fatal(err)
	}
	packet := carrier.Packet()
	outsideCarrierRaw := ".context/migration-v2/operator-policy.md"
	outsideBytes := []byte("operator policy at review time\n")
	outsidePath := filepath.Join(fixture.root, outsideCarrierRaw)
	mustWriteCLISpecMigrationFile(t, outsidePath, outsideBytes)
	outsideID, err := specmigrationv2.NewOutsideCarrierID("operator_policy")
	mustCLISpecMigrationNoError(t, err)
	outsideCarrier, err := specmigrationv2.NewSourceCarrierID(outsideCarrierRaw)
	mustCLISpecMigrationNoError(t, err)
	registration, err := specmigrationv2.NewOutsideCarrierRegistration(
		specmigrationv2.OutsideCarrierRegistrationInput{
			ID:      outsideID,
			Carrier: outsideCarrier,
			Digest:  specmigrationv2.OutsideCarrierDigestOf(outsideBytes),
		},
	)
	mustCLISpecMigrationNoError(t, err)
	registry, err := specmigrationv2.NewOutsideCarrierRegistry(
		[]specmigrationv2.OutsideCarrierRegistration{registration},
	)
	mustCLISpecMigrationNoError(t, err)
	outsideSet, err := specmigrationv2.NewOutsideCarrierSet(
		[]specmigrationv2.OutsideCarrierID{outsideID},
	)
	mustCLISpecMigrationNoError(t, err)
	outsideDisposition, err := specmigrationv2.NewOutsidePSS(
		"Operator policy remains outside the SoftwareSystemSpec.",
		outsideSet,
	)
	mustCLISpecMigrationNoError(t, err)
	section := packet.Source().Sections()[0]
	disposition, err := specmigrationv2.NewSourceDisposition(
		section.ID(),
		outsideDisposition,
	)
	mustCLISpecMigrationNoError(t, err)
	packet, err = specmigrationv2.NewPacket(specmigrationv2.PacketInput{
		ID:                 packet.ID(),
		SchemaVersion:      packet.SchemaVersion(),
		Source:             packet.Source(),
		Target:             packet.Target(),
		OutsideRegistry:    registry,
		SourceDispositions: []specmigrationv2.SourceDisposition{disposition},
	})
	mustCLISpecMigrationNoError(t, err)
	carrier, err = specmigrationv2.FinalizePacketCarrier(packet, carrier.ReviewBasis())
	mustCLISpecMigrationNoError(t, err)
	fixture.packetBytes = carrier.CanonicalBytes()
	mustWriteCLISpecMigrationFile(t, fixture.packetPath, fixture.packetBytes)

	observation, closeDatabase, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	if err != nil {
		closeDatabase()
		t.Fatalf("observe exact outside carrier: %v", err)
	}
	closeDatabase()
	if observation.partitionAudit.Status() != specmigrationv2.PacketPartitionAuditVerified {
		t.Fatal("exact outside carrier did not produce a verified partition audit")
	}

	mustWriteCLISpecMigrationFile(t, outsidePath, []byte("operator policy drifted\n"))
	result, err := executeSpecMigrationV2ForTest(t, fixture, false)
	if err != nil {
		t.Fatalf("run drifted migration-v2 dry-run: %v", err)
	}
	if result.State != "invalid" || result.PartitionAuditStatus != string(specmigrationv2.PacketPartitionAuditRejected) {
		t.Fatalf("result=%+v, want rejected-audit Invalid", result)
	}
	hasOutsideDigestDiagnostic := slices.ContainsFunc(
		result.Diagnostics,
		func(value specMigrationV2Diagnostic) bool {
			return value.Code == string(specmigrationv2.DiagnosticOutsideCarrierDigestMismatch)
		},
	)
	if !hasOutsideDigestDiagnostic {
		t.Fatalf("diagnostics=%+v, want outside carrier digest mismatch", result.Diagnostics)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2RejectsStaleGitSourceProvenanceBeforeAudit(t *testing.T) {
	fixture := newCLISpecMigrationV2FixtureWithStaleSourceProvenance(t, true)
	result, err := executeSpecMigrationV2ForTest(t, fixture, false)
	if err != nil {
		t.Fatalf("runSpecMigrateV2 dry-run: %v", err)
	}
	assertCLISpecMigrationV2GitProvenanceInvalid(t, result)
	assertCLISpecMigrationV2NoEffect(t, fixture)

	applyResult, applyErr := executeSpecMigrationV2ForTest(t, fixture, true)
	if applyErr == nil || migrationTestErrorCode(applyErr) != specMigrationInvalidCode {
		t.Fatalf("result=%+v err=%v, want migration_invalid precondition", applyResult, applyErr)
	}
	assertCLISpecMigrationV2GitProvenanceInvalid(t, applyResult)
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2RejectsDirtyFPFSourceBeforeAudit(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	driftPath := filepath.Join(fixture.fpfRoot, "Readme.md")
	mustWriteCLISpecMigrationFile(t, driftPath, []byte("# reviewed FPF fixture\n\ndrift\n"))

	result, err := executeSpecMigrationV2ForTest(t, fixture, false)
	if err != nil {
		t.Fatalf("runSpecMigrateV2: %v", err)
	}
	if result.State != "invalid" || len(result.Diagnostics) != 1 {
		t.Fatalf("result=%+v, want one typed FPF-source Invalid diagnostic", result)
	}
	if result.Diagnostics[0].Code != specMigrationFPFSourceInvalidCode {
		t.Fatalf("diagnostic=%+v, want %s", result.Diagnostics[0], specMigrationFPFSourceInvalidCode)
	}
	if result.PartitionAuditStatus != "not_run" || result.PartitionAuditDigest != "" {
		t.Fatalf("partition audit = %q/%q, want not_run with no digest", result.PartitionAuditStatus, result.PartitionAuditDigest)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2ApplyWithoutAdmittedReviewFailsClosed(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	result, err := executeSpecMigrationV2ForTest(t, fixture, true)
	if err == nil || migrationTestErrorCode(err) != specMigrationPendingReviewCode {
		t.Fatalf("result=%+v err=%v, want pending_review precondition", result, err)
	}
	if result.State != "pending_review" || result.Applied {
		t.Fatalf("result=%+v, want non-applied PendingReview", result)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationPublicCommandWithoutPreparedCandidateFailsClosed(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	t.Setenv(envProjectRoot, fixture.root)
	previousJSON := specMigrateJSON
	specMigrateJSON = false
	t.Cleanup(func() {
		specMigrateJSON = previousJSON
	})

	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(output)
	err := runSpecMigrate(command, nil)
	if err == nil || migrationTestErrorCode(err) != "migration_candidate_not_prepared" {
		t.Fatalf("error = %v, want migration_candidate_not_prepared", err)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestWriteSpecMigrationV2ResultHumanOutputHidesOpaqueProvenance(t *testing.T) {
	result := specMigrationV2Result{
		State:                "pending_review",
		PacketID:             "migration-packet:opaque",
		PacketDigest:         "sha256:packet",
		PacketCarrier:        ".haft/spec-migration-v2/packets/opaque.json",
		PacketCarrierDigest:  "sha256:packet-carrier",
		SourceCarrier:        ".haft/specs/enabling-system.md",
		SourceDigest:         "sha256:source",
		FinalTargetCarrier:   ".haft/specs/software-system.md",
		TargetDigest:         "sha256:target",
		PartitionAuditStatus: "verified",
		PartitionAuditDigest: "sha256:audit",
		ProfileApplicability: "required",
		ReceiptCarrier:       ".haft/spec-migration-v2/receipts/opaque.json",
		ReceiptCarrierDigest: "sha256:receipt",
		NextAction:           "run haft spec migrate interactively to review this migration",
		PartitionAuditCounts: specMigrationV2AuditCounts{
			SourceSections: 4,
			LineageEntries: 7,
		},
	}
	output := &bytes.Buffer{}
	if err := writeSpecMigrationV2Result(output, result, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Specification migration: pending_review",
		"profile applicability: required",
		"semantic partition audit: verified",
		"source sections: 4; target lineage entries: 7",
		"next: run haft spec migrate interactively",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("human migration output omitted %q:\n%s", want, output.String())
		}
	}
	for _, opaque := range []string{
		"migration-packet:opaque",
		"sha256:",
		"opaque.json",
		"PacketID",
		"packet_id",
		"receipt_carrier",
	} {
		if strings.Contains(output.String(), opaque) {
			t.Fatalf("human migration output exposes opaque internal value %q:\n%s", opaque, output.String())
		}
	}
}

func TestSpecMigrationV2RejectsReviewDraftJSONInsteadOfTreatingItAsAuthority(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	reviewDraftPath := filepath.Join(fixture.root, ".context", "review-draft.json")
	mustWriteCLISpecMigrationFile(t, reviewDraftPath, []byte(`{"kind":"PendingReview","receipt":{"kind":"Absent"}}`))

	_, closeDatabase, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		reviewDraftPath,
	)
	closeDatabase()
	if err == nil || !strings.Contains(err.Error(), "strict migration-v2 final candidate") {
		t.Fatalf("error = %v, want strict final-candidate rejection", err)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2KeepsReviewCarrierDistinctFromFinalInstallTarget(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, false)
	carrier, err := specmigrationv2.DecodePacketCarrier(fixture.packetBytes)
	if err != nil {
		t.Fatal(err)
	}
	err = validateCLISoftwareMigrationPacket(carrier.Packet(), specMigrationV2TargetCarrier)
	if err == nil || !strings.Contains(err.Error(), "must remain distinct") {
		t.Fatalf("error = %v, want review/final-target distinction rejection", err)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationV2RejectsDriftedReviewBasisBytes(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	drifted := append([]byte{}, fixture.targetBytes...)
	drifted = append(drifted, []byte("\n# drift\n")...)
	mustWriteCLISpecMigrationFile(t, fixture.reviewSoftwarePath, drifted)

	_, closeDatabase, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	closeDatabase()
	if err == nil || !strings.Contains(err.Error(), "review-basis carrier digest mismatch") {
		t.Fatalf("error = %v, want exact review-basis digest rejection", err)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, fixture.finalTargetCarrier)); !os.IsNotExist(statErr) {
		t.Fatalf("drift rejection wrote final target: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, fixture.archiveCarrier)); !os.IsNotExist(statErr) {
		t.Fatalf("drift rejection wrote archive: %v", statErr)
	}
}

type cliSpecMigrationV2Fixture struct {
	root                  string
	packetPath            string
	sourcePath            string
	reviewSoftwarePath    string
	finalTargetCarrier    string
	reviewSoftwareCarrier string
	archiveCarrier        string
	sourceBytes           []byte
	targetBytes           []byte
	packetBytes           []byte
	fpfRoot               string
	sourceCommit          string
}

func newCLISpecMigrationV2Fixture(
	t *testing.T,
	withSoftwareProfile bool,
) cliSpecMigrationV2Fixture {
	t.Helper()
	return newCLISpecMigrationV2FixtureWithProvenance(t, withSoftwareProfile, false)
}

func newCLISpecMigrationV2FixtureWithStaleSourceProvenance(
	t *testing.T,
	withSoftwareProfile bool,
) cliSpecMigrationV2Fixture {
	t.Helper()
	return newCLISpecMigrationV2FixtureWithProvenance(t, withSoftwareProfile, true)
}

func newCLISpecMigrationV2FixtureWithProvenance(
	t *testing.T,
	withSoftwareProfile bool,
	staleSourceProvenance bool,
) cliSpecMigrationV2Fixture {
	t.Helper()
	physicalSeed := t.TempDir()
	profileHarness := profileadmissionfixture.New(t, physicalSeed)
	root := profileHarness.Root().String()
	if withSoftwareProfile {
		profileHarness.AdmitSoftwareRevision(t, "cli-migration-v2")
	}

	projectID := profileHarness.ProjectID()
	home := t.TempDir()
	t.Setenv("HOME", home)
	mustWriteCLISpecMigrationFile(
		t,
		filepath.Join(root, ".haft", "project.yaml"),
		[]byte("id: "+projectID+"\nname: cli-migration-v2\n"),
	)
	databaseDir := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(databaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(databaseDir, "haft.db")
	escapedDatabasePath := strings.ReplaceAll(databasePath, "'", "''")
	if _, err := profileHarness.Database().Exec("VACUUM INTO '" + escapedDatabasePath + "'"); err != nil {
		t.Fatalf("snapshot canonical profile database: %v", err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("bind CLI fixture project ledger: %v", err)
	}

	sourceCarrier := ".haft/specs/enabling-system.md"
	finalTargetCarrier := ".haft/specs/software-system.md"
	archiveCarrier := ".haft/migration-archive/enabling-system.md"
	reviewSoftwareCarrier := ".context/migration-v2/software.review.md"
	targetSystemCarrier := ".context/migration-v2/target.review.md"
	termMapCarrier := ".context/migration-v2/term-map.review.md"
	semanticCarrier := ".context/migration-v2/semantic-review.md"
	provenanceCarrier := ".context/migration-v2/source-designation.md"
	sourceBytes := []byte("## ES.fixture.001 Fixture\n\n```yaml spec-section\nid: ES.fixture.001\nspec: enabling-system\nkind: enabling.architecture\nstatus: draft\n```\n")
	targetBytes := []byte("# Software System Spec\n\n## SS.alpha.001 Alpha\n\n```yaml spec-section\nid: SS.alpha.001\nspec: software-system\nkind: software.functional_behavior\nstatus: draft\nclaims:\n  - id: SS.alpha.001.L1\n    class: L\n    statement: Exact fixture law.\n    scope:\n      - fixture\n```\n")
	targetSystemBytes := []byte("# Target system review fixture\n")
	termMapBytes := []byte("# Term map review fixture\n")
	semanticBytes := []byte("# Semantic zero-pass fixture\n")
	provenanceBytes := []byte("operator designated the exact CLI migration source edition\n")
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, sourceCarrier), sourceBytes)
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, reviewSoftwareCarrier), targetBytes)
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, targetSystemCarrier), targetSystemBytes)
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, termMapCarrier), termMapBytes)
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, semanticCarrier), semanticBytes)
	mustWriteCLISpecMigrationFile(t, filepath.Join(root, provenanceCarrier), provenanceBytes)

	mustRunCLISpecMigrationGit(t, root, "init")
	mustRunCLISpecMigrationGit(t, root, "config", "user.email", "test@example.com")
	mustRunCLISpecMigrationGit(t, root, "config", "user.name", "Haft CLI Test")
	mustRunCLISpecMigrationGit(t, root, "add", "-f", "--", sourceCarrier, provenanceCarrier)
	mustRunCLISpecMigrationGit(t, root, "commit", "-m", "designated source edition")
	objectFormatBytes := mustRunCLISpecMigrationGit(t, root, "rev-parse", "--show-object-format")
	objectFormatText := string(objectFormatBytes)
	objectFormat := strings.TrimSpace(objectFormatText)
	headBytes := mustRunCLISpecMigrationGit(t, root, "rev-parse", "HEAD")
	headText := string(headBytes)
	head := strings.TrimSpace(headText)
	sourceCommit := objectFormat + ":" + head
	if staleSourceProvenance {
		sourceCommit = "sha1:" + strings.Repeat("b", 40)
	}

	fpfRoot := filepath.Join(root, "data", "FPF")
	mustWriteCLISpecMigrationFile(t, filepath.Join(fpfRoot, "FPF-Spec.md"), []byte("# reviewed FPF specification fixture\n"))
	mustWriteCLISpecMigrationFile(t, filepath.Join(fpfRoot, "Readme.md"), []byte("# reviewed FPF fixture\n"))
	mustRunCLISpecMigrationGit(t, fpfRoot, "init")
	mustRunCLISpecMigrationGit(t, fpfRoot, "config", "user.email", "test@example.com")
	mustRunCLISpecMigrationGit(t, fpfRoot, "config", "user.name", "Haft FPF Test")
	mustRunCLISpecMigrationGit(t, fpfRoot, "add", "--", "FPF-Spec.md", "Readme.md")
	mustRunCLISpecMigrationGit(t, fpfRoot, "commit", "-m", "reviewed FPF revision")
	fpfRevisionBytes := mustRunCLISpecMigrationGit(t, fpfRoot, "rev-parse", "HEAD")
	fpfRevisionText := string(fpfRevisionBytes)
	fpfRevision := strings.TrimSpace(fpfRevisionText)

	packet := newCLISpecMigrationPacket(
		t,
		root,
		sourceCarrier,
		finalTargetCarrier,
		archiveCarrier,
		sourceBytes,
		targetBytes,
		sourceCommit,
		provenanceCarrier,
		provenanceBytes,
	)
	targetSystemID := mustCLITargetCarrierID(t, targetSystemCarrier)
	reviewSoftwareID := mustCLITargetCarrierID(t, reviewSoftwareCarrier)
	termMapID := mustCLITargetCarrierID(t, termMapCarrier)
	semanticID := mustCLITargetCarrierID(t, semanticCarrier)
	basis, err := specmigrationv2.NewFinalCandidateReviewBasis(
		specmigrationv2.FinalCandidateReviewBasisInput{
			CarrierDigests: []specmigrationv2.ReviewCarrierDigestInput{
				{
					Role:    specmigrationv2.ReviewTargetSystemCarrier,
					Carrier: targetSystemID,
					Digest:  specmigrationv2.DigestBytes(targetSystemBytes),
				},
				{
					Role:    specmigrationv2.ReviewSoftwareSystemCarrier,
					Carrier: reviewSoftwareID,
					Digest:  specmigrationv2.DigestBytes(targetBytes),
				},
				{
					Role:    specmigrationv2.ReviewTermMapCarrier,
					Carrier: termMapID,
					Digest:  specmigrationv2.DigestBytes(termMapBytes),
				},
			},
			FPFRevision: fpfRevision,
			SemanticZeroPass: specmigrationv2.SemanticZeroPassInput{
				Carrier: semanticID,
				Digest:  specmigrationv2.DigestBytes(semanticBytes),
			},
			LifecycleIntent: []specmigrationv2.LifecycleIntentInput{
				{SectionRef: "TS.fixture.001", Operation: specmigrationv2.LifecycleRebaseline},
				{SectionRef: "SS.alpha.001", Operation: specmigrationv2.LifecycleActivate},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := specmigrationv2.FinalizePacketCarrier(packet, basis)
	if err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(root, ".context", "migration-v2", "packet.final.json")
	packetBytes := carrier.CanonicalBytes()
	mustWriteCLISpecMigrationFile(t, packetPath, packetBytes)

	return cliSpecMigrationV2Fixture{
		root:                  root,
		packetPath:            packetPath,
		sourcePath:            filepath.Join(root, sourceCarrier),
		reviewSoftwarePath:    filepath.Join(root, reviewSoftwareCarrier),
		finalTargetCarrier:    finalTargetCarrier,
		reviewSoftwareCarrier: reviewSoftwareCarrier,
		archiveCarrier:        archiveCarrier,
		sourceBytes:           sourceBytes,
		targetBytes:           targetBytes,
		packetBytes:           packetBytes,
		fpfRoot:               fpfRoot,
		sourceCommit:          sourceCommit,
	}
}

func newCLISpecMigrationPacket(
	t *testing.T,
	root string,
	sourceCarrierRaw string,
	targetCarrierRaw string,
	archiveCarrierRaw string,
	sourceBytes []byte,
	targetBytes []byte,
	sourceCommitRaw string,
	provenanceCarrierRaw string,
	provenanceBytes []byte,
) specmigrationv2.Packet {
	t.Helper()
	sourceCarrier, err := specmigrationv2.NewSourceCarrierID(sourceCarrierRaw)
	mustCLISpecMigrationNoError(t, err)
	targetCarrier, err := specmigrationv2.NewTargetCarrierID(targetCarrierRaw)
	mustCLISpecMigrationNoError(t, err)
	archiveCarrier, err := specmigrationv2.NewArchiveCarrierID(archiveCarrierRaw)
	mustCLISpecMigrationNoError(t, err)
	sectionID, err := specmigrationv2.NewSourceSectionID("ES.fixture.001")
	mustCLISpecMigrationNoError(t, err)
	length, err := specmigrationv2.NewByteLength(uint64(len(sourceBytes)))
	mustCLISpecMigrationNoError(t, err)
	span, err := specmigrationv2.NewExactByteSpan(
		0,
		length,
		specmigrationv2.FragmentDigestOf(sourceBytes),
	)
	mustCLISpecMigrationNoError(t, err)
	section, err := specmigrationv2.NewSourceSection(sectionID, span)
	mustCLISpecMigrationNoError(t, err)
	projectRoot, err := specmigrationv2.NewProjectRootRef(root)
	mustCLISpecMigrationNoError(t, err)
	commitOID, err := specmigrationv2.NewGitCommitOID(sourceCommitRaw)
	mustCLISpecMigrationNoError(t, err)
	sourceDigest := specmigrationv2.SourceDigestOf(sourceBytes)
	edition, err := specmigrationv2.NewRepositoryEdition(
		projectRoot,
		commitOID,
		sourceCarrier,
		sourceDigest,
	)
	mustCLISpecMigrationNoError(t, err)
	recordRef, err := specmigrationv2.NewProvenanceRecordRef(provenanceCarrierRaw)
	mustCLISpecMigrationNoError(t, err)
	recordBinding, err := specmigrationv2.NewProvenanceRecordBinding(
		recordRef,
		specmigrationv2.ProvenanceRecordDigestOf(provenanceBytes),
	)
	mustCLISpecMigrationNoError(t, err)
	provenance, err := specmigrationv2.NewDesignatedSourceProvenance(
		edition,
		recordBinding,
	)
	mustCLISpecMigrationNoError(t, err)
	archive, err := specmigrationv2.NewArchiveManifest(archiveCarrier, sourceDigest)
	mustCLISpecMigrationNoError(t, err)
	source, err := specmigrationv2.NewSourceManifest(
		specmigrationv2.SourceManifestInput{
			Carrier:    sourceCarrier,
			Digest:     sourceDigest,
			ByteLength: length,
			Archive:    archive,
			Provenance: provenance,
			Sections:   []specmigrationv2.SourceSection{section},
		},
	)
	mustCLISpecMigrationNoError(t, err)
	targetLength, err := specmigrationv2.NewByteLength(uint64(len(targetBytes)))
	mustCLISpecMigrationNoError(t, err)
	target, err := specmigrationv2.NewTargetManifest(
		specmigrationv2.TargetManifestInput{
			Carrier:    targetCarrier,
			Digest:     specmigrationv2.TargetDigestOf(targetBytes),
			ByteLength: targetLength,
		},
	)
	mustCLISpecMigrationNoError(t, err)
	claim, err := specmigrationv2.NewTargetAtomicClaimID("SS.alpha.001.L1")
	mustCLISpecMigrationNoError(t, err)
	claimSet, err := specmigrationv2.NewTargetClaimSet(
		[]specmigrationv2.TargetAtomicClaimID{claim},
	)
	mustCLISpecMigrationNoError(t, err)
	mapping, err := specmigrationv2.NewMapOne(claimSet)
	mustCLISpecMigrationNoError(t, err)
	disposition, err := specmigrationv2.NewSourceDisposition(sectionID, mapping)
	mustCLISpecMigrationNoError(t, err)
	registry, err := specmigrationv2.NewOutsideCarrierRegistry(nil)
	mustCLISpecMigrationNoError(t, err)
	packetID, err := specmigrationv2.NewMigrationPacketID("cli-migration-v2-fixture")
	mustCLISpecMigrationNoError(t, err)
	packet, err := specmigrationv2.NewPacket(specmigrationv2.PacketInput{
		ID:                 packetID,
		SchemaVersion:      specmigrationv2.SchemaVersionV2,
		Source:             source,
		Target:             target,
		OutsideRegistry:    registry,
		SourceDispositions: []specmigrationv2.SourceDisposition{disposition},
	})
	mustCLISpecMigrationNoError(t, err)
	return packet
}

func executeSpecMigrationV2ForTest(
	t *testing.T,
	fixture cliSpecMigrationV2Fixture,
	apply bool,
) (specMigrationV2Result, error) {
	t.Helper()
	previousJSON := specMigrateJSON
	specMigrateJSON = true
	t.Cleanup(func() { specMigrateJSON = previousJSON })

	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(output)
	operation := specMigrationV2InspectOperation
	if apply {
		operation = specMigrationV2ApplyOperation
	}
	err := runSpecMigrateV2OperationWithReviewCapture(
		command,
		fixture.root,
		fixture.packetPath,
		operation,
		specmigrationv2.CaptureVerifiedMigrationReview,
	)
	var result specMigrationV2Result
	if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil {
		t.Fatalf("decode migration-v2 result: %v\n%s", decodeErr, output.String())
	}
	return result, err
}

func observeSpecMigrationV2ForTest(
	ctx context.Context,
	root string,
	packetPath string,
) (specMigrationV2Observation, func(), error) {
	ledger, err := openSpecMigrationV2Ledger(ctx, root, projectledger.ReadOnly)
	if err != nil {
		return specMigrationV2Observation{}, noopClose, err
	}
	closeLedger := func() {
		_ = ledger.Close()
	}
	observation, err := observeSpecMigrationV2WithProfile(
		ctx,
		root,
		packetPath,
		ledger.profile,
	)
	if err != nil {
		closeLedger()
		return specMigrationV2Observation{}, noopClose, err
	}
	return observation, closeLedger, nil
}

func openSpecMigrationReviewAdmissionServiceForTest(
	ctx context.Context,
	root string,
) (specmigrationv2.ReviewAdmissionService, func(), error) {
	ledger, err := openSpecMigrationV2Ledger(ctx, root, projectledger.ReadWrite)
	if err != nil {
		return specmigrationv2.ReviewAdmissionService{}, noopClose, err
	}
	closeLedger := func() {
		_ = ledger.Close()
	}
	return ledger.review, closeLedger, nil
}

func migrationTestErrorCode(err error) string {
	type codedError interface {
		Code() string
	}
	var coded codedError
	if !errors.As(err, &coded) {
		return ""
	}
	return coded.Code()
}

func assertCLISpecMigrationV2NoEffect(
	t *testing.T,
	fixture cliSpecMigrationV2Fixture,
) {
	t.Helper()
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, fixture.sourcePath), fixture.sourceBytes) {
		t.Fatal("migration-v2 dry-run/apply rejection changed source bytes")
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, fixture.reviewSoftwarePath), fixture.targetBytes) {
		t.Fatal("migration-v2 dry-run/apply rejection changed review candidate bytes")
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, fixture.packetPath), fixture.packetBytes) {
		t.Fatal("migration-v2 dry-run/apply rejection changed packet bytes")
	}
	if _, err := os.Stat(filepath.Join(fixture.root, fixture.finalTargetCarrier)); !os.IsNotExist(err) {
		t.Fatalf("migration-v2 dry-run/apply rejection wrote final target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.root, fixture.archiveCarrier)); !os.IsNotExist(err) {
		t.Fatalf("migration-v2 dry-run/apply rejection wrote archive: %v", err)
	}
}

func assertCLISpecMigrationV2GitProvenanceInvalid(
	t *testing.T,
	result specMigrationV2Result,
) {
	t.Helper()
	if result.State != "invalid" || result.Applied {
		t.Fatalf("result=%+v, want non-applied Invalid", result)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%+v, want one Git provenance diagnostic", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Code != specMigrationGitProvenanceInvalidCode {
		t.Fatalf("diagnostic=%+v, want %s", diagnostic, specMigrationGitProvenanceInvalidCode)
	}
	if !strings.Contains(diagnostic.Detail, "does not match current HEAD") {
		t.Fatalf("diagnostic detail=%q, want exact verifier rejection", diagnostic.Detail)
	}
	if result.PartitionAuditStatus != "not_run" || result.PartitionAuditDigest != "" {
		t.Fatalf("partition audit=%q/%q, want not_run with no digest", result.PartitionAuditStatus, result.PartitionAuditDigest)
	}
	counts := result.PartitionAuditCounts
	if counts != (specMigrationV2AuditCounts{}) {
		t.Fatalf("partition audit counts=%+v, want zero because audit did not run", counts)
	}
	if result.ProfileApplicability != "not_observed" {
		t.Fatalf("profile applicability=%q, want not_observed", result.ProfileApplicability)
	}
}

func mustWriteCLISpecMigrationFile(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, value, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadCLISpecMigrationFile(t *testing.T, path string) []byte {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustRunCLISpecMigrationGit(
	t *testing.T,
	root string,
	args ...string,
) []byte {
	t.Helper()
	commandArgs := []string{"-C", root}
	commandArgs = append(commandArgs, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return output
}

func mustCLITargetCarrierID(
	t *testing.T,
	raw string,
) specmigrationv2.TargetCarrierID {
	t.Helper()
	value, err := specmigrationv2.NewTargetCarrierID(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustCLISpecMigrationNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
