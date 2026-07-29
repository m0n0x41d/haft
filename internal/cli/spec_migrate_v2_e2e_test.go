package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/spf13/cobra"
)

type cliMigrationV2ReceiptProbe struct {
	Schema                  string `json:"schema"`
	MigrationID             string `json:"migration_id"`
	PacketDigest            string `json:"packet_digest"`
	SourceDigest            string `json:"source_digest"`
	TargetDigest            string `json:"target_digest"`
	LineagePolicyDigest     string `json:"lineage_policy_digest"`
	ProfileAdmissionRef     string `json:"profile_admission_ref"`
	ProfileAdmissionDigest  string `json:"profile_admission_digest"`
	ProfileLedgerRevision   uint64 `json:"profile_ledger_revision"`
	SemanticReviewRef       string `json:"semantic_review_ref"`
	SemanticAdmissionDigest string `json:"semantic_review_admission_digest"`
	SemanticReviewDigest    string `json:"semantic_review_digest"`
}

type cliMigrationV2LineageProbe struct {
	Schema              string                       `json:"schema"`
	MigrationID         string                       `json:"migration_id"`
	PacketDigest        string                       `json:"packet_digest"`
	LineagePolicyDigest string                       `json:"lineage_policy_digest"`
	Entries             []cliMigrationV2LineageEntry `json:"entries"`
}

type cliMigrationV2LineageEntry struct {
	SubjectKind    string   `json:"subject_kind"`
	SourceSection  string   `json:"source_section"`
	OutcomeKind    string   `json:"outcome_kind"`
	TargetClaims   []string `json:"target_claims"`
	FragmentDigest string   `json:"fragment_digest"`
}

func TestSpecMigrationV2DurableReviewAdmissionToFreshCLIApply(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	observation, closeObservation, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	if err != nil {
		t.Fatalf("observe exact pre-admission profile binding: %v", err)
	}
	required, ok := observation.profileApplicability.Required()
	closeObservation()
	if !ok {
		t.Fatalf("profile applicability = %q, want required", observation.profileApplicability.Kind())
	}

	previousJSON := specMigrateJSON
	specMigrateJSON = false
	t.Cleanup(func() { specMigrateJSON = previousJSON })
	captureCalls := 0
	capture := cliMigrationReviewFixtureCapture(t, &captureCalls, nil)
	admissionOutput := &bytes.Buffer{}
	admissionCommand := &cobra.Command{}
	admissionCommand.SetOut(admissionOutput)
	err = runSpecMigrateV2OperationWithReviewCapture(
		admissionCommand,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		capture,
	)
	if err != nil {
		t.Fatalf("admit exact semantic review through CLI: %v", err)
	}
	if captureCalls != 1 {
		t.Fatalf("terminal capture calls = %d, want 1", captureCalls)
	}
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 1)
	if !bytes.Contains(admissionOutput.Bytes(), []byte("Semantic review recorded.")) {
		t.Fatalf("admission output omitted the human result:\n%s", admissionOutput.String())
	}
	reviewService, closeReviewService, err := openSpecMigrationReviewAdmissionServiceForTest(
		context.Background(),
		fixture.root,
	)
	if err != nil {
		t.Fatalf("reopen admitted semantic review: %v", err)
	}
	admission, err := reviewService.ResolveCurrentForAudit(
		context.Background(),
		observation.carrier,
		observation.partitionAudit,
	)
	closeReviewService()
	if err != nil {
		t.Fatalf("resolve admitted semantic review: %v", err)
	}

	result, err := executeSpecMigrationV2ForTest(t, fixture, true)
	if err != nil {
		t.Fatalf("apply admitted migration through fresh CLI invocation: %v", err)
	}
	if result.State != "applied" || !result.Applied || !result.ApplyRequested {
		t.Fatalf("apply result = %+v, want requested Applied", result)
	}
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 1)

	finalTargetPath := filepath.Join(fixture.root, fixture.finalTargetCarrier)
	archivePath := filepath.Join(fixture.root, fixture.archiveCarrier)
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, finalTargetPath), fixture.targetBytes) {
		t.Fatal("fresh CLI apply did not install the exact reviewed target bytes")
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, archivePath), fixture.sourceBytes) {
		t.Fatal("fresh CLI apply did not archive the exact source bytes")
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, fixture.reviewSoftwarePath), fixture.targetBytes) {
		t.Fatal("fresh CLI apply changed the review-basis carrier")
	}
	if !bytes.Equal(mustReadCLISpecMigrationFile(t, fixture.packetPath), fixture.packetBytes) {
		t.Fatal("fresh CLI apply changed the admitted packet carrier")
	}
	if _, statErr := os.Stat(fixture.sourcePath); !os.IsNotExist(statErr) {
		t.Fatalf("fresh CLI apply retained the designated source: %v", statErr)
	}

	carrier, err := specmigrationv2.DecodePacketCarrier(fixture.packetBytes)
	if err != nil {
		t.Fatalf("decode admitted packet carrier: %v", err)
	}
	migrationID := carrier.Packet().ID().String()
	receiptBytes := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "receipt")
	receipt := cliMigrationV2ReceiptProbe{}
	if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
		t.Fatalf("decode durable migration receipt: %v", err)
	}
	if receipt.Schema != "haft.spec-migration-v2.receipt/v2" {
		t.Fatalf("receipt schema = %q", receipt.Schema)
	}
	if receipt.MigrationID != migrationID || receipt.PacketDigest != result.PacketDigest {
		t.Fatalf("receipt migration binding = %q/%q, want %q/%q", receipt.MigrationID, receipt.PacketDigest, migrationID, result.PacketDigest)
	}
	if receipt.SourceDigest != result.SourceDigest || receipt.TargetDigest != result.TargetDigest {
		t.Fatalf("receipt carrier digests = %q/%q, want %q/%q", receipt.SourceDigest, receipt.TargetDigest, result.SourceDigest, result.TargetDigest)
	}
	if receipt.ProfileAdmissionRef != required.AdmissionRecordRef().String() ||
		receipt.ProfileAdmissionDigest != required.AdmissionRecordDigest().String() ||
		receipt.ProfileLedgerRevision != required.LedgerRevision().Value() {
		t.Fatalf("receipt profile binding does not match the exact pre-apply Required capability: %+v", receipt)
	}
	if receipt.SemanticReviewRef != admission.ReviewRef().String() ||
		receipt.SemanticAdmissionDigest != admission.ReviewAdmissionDigest().String() {
		t.Fatalf("receipt semantic-review binding = %q/%q, want %q/%q", receipt.SemanticReviewRef, receipt.SemanticAdmissionDigest, admission.ReviewRef().String(), admission.ReviewAdmissionDigest().String())
	}
	if receipt.SemanticReviewDigest == "" || receipt.LineagePolicyDigest == "" {
		t.Fatalf("receipt omitted semantic-review or lineage binding: %+v", receipt)
	}

	lineageBytes := mustSingleCLIMigrationV2EffectCarrier(t, fixture.root, "lineage")
	lineage := cliMigrationV2LineageProbe{}
	if err := json.Unmarshal(lineageBytes, &lineage); err != nil {
		t.Fatalf("decode durable migration lineage: %v", err)
	}
	if lineage.Schema != "haft.spec-migration-v2.lineage-record/v1" {
		t.Fatalf("lineage schema = %q", lineage.Schema)
	}
	if lineage.MigrationID != migrationID || lineage.PacketDigest != result.PacketDigest {
		t.Fatalf("lineage migration binding = %q/%q, want %q/%q", lineage.MigrationID, lineage.PacketDigest, migrationID, result.PacketDigest)
	}
	if lineage.LineagePolicyDigest != receipt.LineagePolicyDigest {
		t.Fatalf("lineage policy digest = %q, receipt binds %q", lineage.LineagePolicyDigest, receipt.LineagePolicyDigest)
	}
	if len(lineage.Entries) != 1 {
		t.Fatalf("lineage entries = %d, want 1", len(lineage.Entries))
	}
	entry := lineage.Entries[0]
	if entry.SubjectKind != "whole_source_section" ||
		entry.SourceSection != "ES.fixture.001" ||
		entry.OutcomeKind != "meaning_mapped_to_target_claims" ||
		len(entry.TargetClaims) != 1 ||
		entry.TargetClaims[0] != "SS.alpha.001.L1" ||
		entry.FragmentDigest == "" {
		t.Fatalf("lineage entry does not preserve the exact map-one binding: %+v", entry)
	}
}

func mustSingleCLIMigrationV2EffectCarrier(t *testing.T, root string, kind string) []byte {
	t.Helper()
	pattern := filepath.Join(root, ".haft", "spec-migration-v2.*."+kind+".json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %s carrier: %v", kind, err)
	}
	if len(paths) != 1 {
		t.Fatalf("%s carriers = %v, want exactly one", kind, paths)
	}
	return mustReadCLISpecMigrationFile(t, paths[0])
}
