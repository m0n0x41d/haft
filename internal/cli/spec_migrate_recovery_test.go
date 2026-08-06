package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestSpecMigrationPublicSurfaceIsStateDrivenAndJSONOnly(t *testing.T) {
	t.Parallel()

	flags := make([]string, 0, 1)
	specMigrateCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flags = append(flags, flag.Name)
	})
	if !slices.Equal(flags, []string{"json"}) {
		t.Fatalf("public `haft spec migrate` flags = %v, want only [json]", flags)
	}
	for _, removed := range []string{"to", "packet", "apply", "admit-review", "recover"} {
		if specMigrateCmd.Flags().Lookup(removed) != nil {
			t.Fatalf("public `haft spec migrate` still exposes internal switch --%s", removed)
		}
	}
	if err := specMigrateCmd.Args(specMigrateCmd, nil); err != nil {
		t.Fatalf("no-argument invocation rejected: %v", err)
	}
	if err := specMigrateCmd.Args(specMigrateCmd, []string{"opaque-packet-ref"}); err == nil {
		t.Fatal("public `haft spec migrate` accepts an unexpected positional argument")
	}
	jsonFlag := specMigrateCmd.Flags().Lookup("json")
	if jsonFlag == nil || !strings.Contains(jsonFlag.Usage, "without review or mutation") {
		t.Fatalf("--json is not documented as read-only: %+v", jsonFlag)
	}
	if !strings.Contains(specMigrateCmd.Long, "same command is state-driven") {
		t.Fatalf("migration help does not explain implicit state dispatch:\n%s", specMigrateCmd.Long)
	}
}

func TestSpecMigrationRecoveryWithoutJournalRejectsWithoutEffect(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	result, err := executeSpecMigrationV2RecoveryForTest(t, fixture)
	if err == nil || migrationTestErrorCode(err) != specMigrationRecoveryRejectedCode {
		t.Fatalf("recovery without journal error = %v", err)
	}
	if result.State != "recovery_rejected" || result.Applied {
		t.Fatalf("result = %+v, want non-applied recovery_rejected", result)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func TestSpecMigrationRecoveryReplaysCompletedJournalFromArchive(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	captureCalls := 0
	admissionOutput := &bytes.Buffer{}
	admissionCommand := &cobra.Command{}
	admissionCommand.SetOut(admissionOutput)
	err := runSpecMigrateV2OperationWithReviewCapture(
		admissionCommand,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		cliMigrationReviewFixtureCapture(t, &captureCalls, nil),
	)
	if err != nil {
		t.Fatalf("admit exact migration review: %v", err)
	}
	if captureCalls != 1 {
		t.Fatalf("review capture calls = %d, want 1", captureCalls)
	}

	applied, err := executeSpecMigrationV2ForTest(t, fixture, true)
	if err != nil {
		t.Fatalf("apply reviewed migration: %v", err)
	}
	if applied.State != "applied" || !applied.Applied {
		t.Fatalf("apply result = %+v", applied)
	}
	assertSpecMigrationV2ReceiptCarrier(t, fixture.root, applied)
	if _, err := os.Stat(fixture.sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source still exists after apply: %v", err)
	}
	archivePath := filepath.Join(fixture.root, fixture.archiveCarrier)
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, archivePath), fixture.sourceBytes) {
		t.Fatal("archive does not preserve exact source bytes")
	}

	replayed, err := executeSpecMigrationV2RecoveryForTest(t, fixture)
	if err != nil {
		t.Fatalf("replay completed migration from archive: %v", err)
	}
	if replayed.State != "replayed" || !replayed.Applied || !replayed.RecoveryRequested {
		t.Fatalf("recovery result = %+v, want replayed", replayed)
	}
	assertSpecMigrationV2ReceiptCarrier(t, fixture.root, replayed)
	if replayed.ReceiptCarrier != applied.ReceiptCarrier ||
		replayed.ReceiptCarrierDigest != applied.ReceiptCarrierDigest {
		t.Fatalf("replayed receipt carrier = %s/%s, want %s/%s",
			replayed.ReceiptCarrier,
			replayed.ReceiptCarrierDigest,
			applied.ReceiptCarrier,
			applied.ReceiptCarrierDigest,
		)
	}
	if !bytes.Equal(
		mustReadCLISpecMigrationFile(t, fixture.reviewSoftwarePath),
		fixture.targetBytes,
	) {
		t.Fatal("recovery changed the distinct review carrier")
	}
}

func TestSpecMigrationPublicJSONKeepsIncompleteJournalRecoveryPending(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	installCurrentSpecMigrationCandidate(t, fixture)
	t.Setenv(envProjectRoot, fixture.root)
	setSpecMigrationJSONForTest(t, false)

	captureCalls := 0
	capture := cliMigrationReviewFixtureCapture(t, &captureCalls, nil)
	runPublicSpecMigrationForTest(t, capture)
	runPublicSpecMigrationForTest(t, capture)
	carrier, err := specmigrationv2.DecodePacketCarrier(fixture.packetBytes)
	if err != nil {
		t.Fatalf("decode fixture packet carrier: %v", err)
	}
	root, err := specmigrationv2.NewApplyProjectRoot(fixture.root)
	if err != nil {
		t.Fatalf("construct apply root: %v", err)
	}
	journalRef, err := specmigrationv2.EffectJournalCarrierRef(root, carrier.Packet())
	if err != nil {
		t.Fatalf("derive journal carrier: %v", err)
	}
	journalPath := filepath.Join(fixture.root, filepath.FromSlash(journalRef))
	completedJournal := mustReadCLISpecMigrationFile(t, journalPath)
	incompleteJournal := bytes.Replace(
		completedJournal,
		[]byte(`"phase":"completed"`),
		[]byte(`"phase":"receipt_written"`),
		1,
	)
	if bytes.Equal(completedJournal, incompleteJournal) {
		t.Fatal("fixture journal did not contain the completed phase")
	}
	mustWriteCLISpecMigrationFile(t, journalPath, incompleteJournal)
	receiptBefore := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	targetPath := filepath.Join(fixture.root, fixture.finalTargetCarrier)
	targetBefore := mustReadCLISpecMigrationFile(t, targetPath)

	setSpecMigrationJSONForTest(t, true)
	output := runPublicSpecMigrationForTest(t, capture)
	result := specMigrationV2Result{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode incomplete migration inspection: %v\n%s", err, output)
	}
	if result.State != "recovery_pending" || result.Applied || result.RecoveryRequested {
		t.Fatalf("incomplete migration inspection = %+v, want recovery_pending", result)
	}
	if result.RecoveryPhase != "receipt_written" ||
		!strings.Contains(result.NextAction, "run haft spec migrate") {
		t.Fatalf("incomplete migration recovery guidance = %+v", result)
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, journalPath), incompleteJournal) ||
		!bytes.Equal(mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt"), receiptBefore) ||
		!bytes.Equal(mustReadCLISpecMigrationFile(t, targetPath), targetBefore) {
		t.Fatal("read-only incomplete migration inspection mutated saga carriers")
	}

	setSpecMigrationJSONForTest(t, false)
	recoveryOutput := runPublicSpecMigrationForTest(t, capture)
	if !strings.Contains(recoveryOutput, "Specification migration: recovered") {
		t.Fatalf("plain incomplete-journal recovery behavior changed:\n%s", recoveryOutput)
	}
	if captureCalls != 1 {
		t.Fatalf("journal recovery repeated semantic review; capture calls = %d", captureCalls)
	}
}

func assertSpecMigrationV2ReceiptCarrier(
	t *testing.T,
	root string,
	result specMigrationV2Result,
) {
	t.Helper()
	if result.ReceiptCarrier == "" || result.ReceiptCarrierDigest == "" {
		t.Fatalf("result omits durable receipt carrier binding: %+v", result)
	}
	carrierPath := filepath.Join(root, filepath.FromSlash(result.ReceiptCarrier))
	carrierBytes := mustReadCLISpecMigrationFile(t, carrierPath)
	observed := specmigrationv2.DigestBytes(carrierBytes).String()
	if observed != result.ReceiptCarrierDigest {
		t.Fatalf("receipt carrier digest = %s, output reports %s", observed, result.ReceiptCarrierDigest)
	}
}

func executeSpecMigrationV2RecoveryForTest(
	t *testing.T,
	fixture cliSpecMigrationV2Fixture,
) (specMigrationV2Result, error) {
	t.Helper()
	previousJSON := specMigrateJSON
	specMigrateJSON = true
	t.Cleanup(func() { specMigrateJSON = previousJSON })

	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(output)
	err := runSpecMigrateV2OperationWithReviewCapture(
		command,
		fixture.root,
		fixture.packetPath,
		specMigrationV2RecoverOperation,
		specmigrationv2.CaptureVerifiedMigrationReview,
	)
	var result specMigrationV2Result
	if output.Len() == 0 {
		return result, err
	}
	if decodeErr := json.Unmarshal(output.Bytes(), &result); decodeErr != nil {
		t.Fatalf("decode recovery result: %v\n%s", decodeErr, output.String())
	}
	return result, err
}
