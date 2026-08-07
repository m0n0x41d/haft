package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/spf13/cobra"
)

func TestSpecMigrationPublicStateMachineReviewsAppliesAndReplays(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	installCurrentSpecMigrationCandidate(t, fixture)
	t.Setenv(envProjectRoot, fixture.root)
	setSpecMigrationJSONForTest(t, false)

	captureCalls := 0
	capture := cliMigrationReviewFixtureCapture(t, &captureCalls, nil)
	first := runPublicSpecMigrationForTest(t, capture)
	if !strings.Contains(first, "Semantic review recorded.") ||
		!strings.Contains(first, "No specification files were changed.") {
		t.Fatalf("first invocation did not stop at semantic review:\n%s", first)
	}
	if captureCalls != 1 {
		t.Fatalf("review capture calls after first invocation = %d, want 1", captureCalls)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)

	second := runPublicSpecMigrationForTest(t, capture)
	if !strings.Contains(second, "Specification migration: applied") {
		t.Fatalf("second invocation did not perform the reviewed migration:\n%s", second)
	}
	if captureCalls != 1 {
		t.Fatalf("second invocation repeated semantic review; capture calls = %d", captureCalls)
	}
	assertAppliedPublicSpecMigration(t, fixture)
	receiptBefore := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")

	setSpecMigrationJSONForTest(t, true)
	inspectionOutput := runPublicSpecMigrationForTest(t, capture)
	inspection := specMigrationV2Result{}
	if err := json.Unmarshal([]byte(inspectionOutput), &inspection); err != nil {
		t.Fatalf("decode completed migration inspection: %v\n%s", err, inspectionOutput)
	}
	if inspection.State != "replayed" || !inspection.Applied || inspection.RecoveryRequested {
		t.Fatalf("completed migration inspection = %+v, want terminal replayed state", inspection)
	}
	if inspection.ReceiptCarrier == "" || inspection.ReceiptCarrierDigest == "" {
		t.Fatalf("completed migration inspection omits receipt provenance: %+v", inspection)
	}
	if strings.Contains(inspection.NextAction, "run haft spec migrate") {
		t.Fatalf("completed migration inspection requests a rerun: %+v", inspection)
	}
	receiptAfterInspection := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	if !bytes.Equal(receiptBefore, receiptAfterInspection) {
		t.Fatal("read-only completed migration inspection changed the durable receipt bytes")
	}

	setSpecMigrationJSONForTest(t, false)
	third := runPublicSpecMigrationForTest(t, capture)
	if !strings.Contains(third, "Specification migration: replayed") ||
		!strings.Contains(third, "durable receipt: recorded") {
		t.Fatalf("third invocation did not replay the durable receipt:\n%s", third)
	}
	if captureCalls != 1 {
		t.Fatalf("receipt replay repeated semantic review; capture calls = %d", captureCalls)
	}
	receiptAfter := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	if !bytes.Equal(receiptBefore, receiptAfter) {
		t.Fatal("receipt replay changed the durable receipt bytes")
	}
}

func TestSpecMigrationPublicJSONKeepsHistoricalCompletionAfterTargetEvolution(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	installCurrentSpecMigrationCandidate(t, fixture)
	t.Setenv(envProjectRoot, fixture.root)
	setSpecMigrationJSONForTest(t, false)

	captureCalls := 0
	capture := cliMigrationReviewFixtureCapture(t, &captureCalls, nil)
	runPublicSpecMigrationForTest(t, capture)
	runPublicSpecMigrationForTest(t, capture)
	journalBefore := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "journal")
	receiptBefore := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	targetPath := filepath.Join(fixture.root, fixture.finalTargetCarrier)
	evolvedTarget := []byte("# Software System Spec\n\nLater legitimate lifecycle edition.\n")
	mustWriteCLISpecMigrationFile(t, targetPath, evolvedTarget)

	setSpecMigrationJSONForTest(t, true)
	output := runPublicSpecMigrationForTest(t, capture)
	result := specMigrationV2Result{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode evolved-target migration inspection: %v\n%s", err, output)
	}
	if result.State != "replayed" || !result.Applied || result.RecoveryRequested {
		t.Fatalf("evolved-target migration inspection = %+v, want historical completion", result)
	}
	if result.ReceiptCarrier == "" || result.ReceiptCarrierDigest == "" {
		t.Fatalf("evolved-target migration inspection omits receipt provenance: %+v", result)
	}
	if !strings.Contains(result.NextAction, "later lifecycle edition") ||
		strings.Contains(result.NextAction, "run haft spec migrate") {
		t.Fatalf("evolved-target next action does not distinguish lifecycle from migration: %q", result.NextAction)
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, targetPath), evolvedTarget) {
		t.Fatal("read-only migration inspection changed the evolved target")
	}
	journalAfter := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "journal")
	receiptAfter := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	if !bytes.Equal(journalBefore, journalAfter) || !bytes.Equal(receiptBefore, receiptAfter) {
		t.Fatal("read-only migration inspection changed historical completion carriers")
	}
	if captureCalls != 1 {
		t.Fatalf("completed migration inspection repeated semantic review; capture calls = %d", captureCalls)
	}
}

func TestSpecMigrationPublicJSONInspectionNeverReviewsOrMutates(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	installCurrentSpecMigrationCandidate(t, fixture)
	t.Setenv(envProjectRoot, fixture.root)
	setSpecMigrationJSONForTest(t, true)

	capture := specMigrationV2ReviewCapture(func(
		context.Context,
		specmigrationv2.PreparedMigrationReviewAdmission,
	) (authority.VerifiedSpeechActSource, error) {
		return authority.VerifiedSpeechActSource{}, fmt.Errorf("read-only JSON inspection attempted terminal capture")
	})
	output := runPublicSpecMigrationForTest(t, capture)
	result := specMigrationV2Result{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode read-only migration result: %v\n%s", err, output)
	}
	if result.State != "pending_review" || result.ApplyRequested || result.Applied {
		t.Fatalf("read-only migration result = %+v, want pending non-effect", result)
	}
	assertCLISpecMigrationV2NoEffect(t, fixture)
}

func runPublicSpecMigrationForTest(
	t *testing.T,
	capture specMigrationV2ReviewCapture,
) string {
	t.Helper()
	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(output)
	if err := runSpecMigrateWithReviewCapture(command, capture); err != nil {
		t.Fatalf("run public specification migration: %v\n%s", err, output.String())
	}
	return output.String()
}

func installCurrentSpecMigrationCandidate(
	t *testing.T,
	fixture cliSpecMigrationV2Fixture,
) string {
	t.Helper()
	carrier, err := specmigrationv2.DecodePacketCarrier(fixture.packetBytes)
	if err != nil {
		t.Fatalf("decode fixture final candidate: %v", err)
	}
	digest := carrier.CarrierDigest().String()
	name := strings.TrimPrefix(digest, "sha256:") + ".json"
	ref := filepath.ToSlash(filepath.Join(specmigrationv2.FinalCandidatePacketStoreRef, name))
	path := filepath.Join(fixture.root, filepath.FromSlash(ref))
	mustWriteCLISpecMigrationFile(t, path, fixture.packetBytes)
	return ref
}

func setSpecMigrationJSONForTest(t *testing.T, value bool) {
	t.Helper()
	previous := specMigrateJSON
	specMigrateJSON = value
	t.Cleanup(func() {
		specMigrateJSON = previous
	})
}

func assertAppliedPublicSpecMigration(
	t *testing.T,
	fixture cliSpecMigrationV2Fixture,
) {
	t.Helper()
	if _, err := os.Stat(fixture.sourcePath); !os.IsNotExist(err) {
		t.Fatalf("applied migration retained source carrier: %v", err)
	}
	archivePath := filepath.Join(fixture.root, fixture.archiveCarrier)
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, archivePath), fixture.sourceBytes) {
		t.Fatal("applied migration did not archive exact source bytes")
	}
	targetPath := filepath.Join(fixture.root, fixture.finalTargetCarrier)
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, targetPath), fixture.targetBytes) {
		t.Fatal("applied migration did not install exact reviewed target bytes")
	}
}
