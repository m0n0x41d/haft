package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
	"github.com/spf13/cobra"
)

func TestSpecMigrationReviewAdmissionStopsBeforeTTYWhenProfileIsUnderdetermined(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, false)
	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(output)

	err := runSpecMigrateV2OperationWithReviewCapture(
		command,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		specmigrationv2.CaptureVerifiedMigrationReview,
	)
	if err == nil || migrationTestErrorCode(err) != specMigrationProfileUnderdeterminedCode {
		t.Fatalf("underdetermined admission error = %v", err)
	}
	if !strings.Contains(output.String(), "Specification migration: underdetermined") {
		t.Fatalf("underdetermined preflight output:\n%s", output.String())
	}
}

func TestSpecMigrationReviewAdmissionRejectsInvalidWitnessBeforeTTY(t *testing.T) {
	fixture := newCLISpecMigrationV2FixtureWithStaleSourceProvenance(t, true)
	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(output)

	err := runSpecMigrateV2OperationWithReviewCapture(
		command,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		specmigrationv2.CaptureVerifiedMigrationReview,
	)
	if err == nil || migrationTestErrorCode(err) != specMigrationInvalidCode {
		t.Fatalf("invalid-witness admission error = %v", err)
	}
	if !strings.Contains(output.String(), "Specification migration: invalid") {
		t.Fatalf("invalid-witness preflight output:\n%s", output.String())
	}
}

func TestOpenSpecMigrationReviewAdmissionServiceUsesExistingProjectLedger(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	service, closeDatabase, err := openSpecMigrationReviewAdmissionServiceForTest(
		context.Background(),
		fixture.root,
	)
	if err != nil {
		t.Fatalf("openSpecMigrationReviewAdmissionService: %v", err)
	}
	defer closeDatabase()

	observation, closeObservation, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeObservation()
	_, err = service.ResolveCurrentForAudit(
		context.Background(),
		observation.carrier,
		observation.partitionAudit,
	)
	if err == nil || !strings.Contains(err.Error(), "no admitted semantic review") {
		t.Fatalf("fresh admission ledger ResolveCurrent error = %v", err)
	}
}

func TestSpecMigrationReviewAdmissionRerunResumesDurableSourceWithoutSecondCapture(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	installCLIMigrationReviewFailureTrigger(t, fixture.root)

	captureCalls := 0
	firstCapture := cliMigrationReviewFixtureCapture(t, &captureCalls, nil)
	firstOutput := &bytes.Buffer{}
	firstCommand := &cobra.Command{}
	firstCommand.SetOut(firstOutput)
	firstErr := runSpecMigrateV2OperationWithReviewCapture(
		firstCommand,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		firstCapture,
	)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "injected CLI migration-review phase-two failure") {
		t.Fatalf("first admission error = %v", firstErr)
	}
	if captureCalls != 1 {
		t.Fatalf("first admission capture calls = %d, want 1", captureCalls)
	}
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 0)
	dropCLIMigrationReviewFailureTrigger(t, fixture.root)
	secondCapture := specMigrationV2ReviewCapture(func(
		context.Context,
		specmigrationv2.PreparedMigrationReviewAdmission,
	) (authority.VerifiedSpeechActSource, error) {
		captureCalls++
		return authority.VerifiedSpeechActSource{}, fmt.Errorf("second terminal capture is forbidden")
	})
	secondOutput := &bytes.Buffer{}
	secondCommand := &cobra.Command{}
	secondCommand.SetOut(secondOutput)
	secondErr := runSpecMigrateV2OperationWithReviewCapture(
		secondCommand,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		secondCapture,
	)
	if secondErr != nil {
		t.Fatalf("resume durable review source: %v", secondErr)
	}
	if captureCalls != 1 {
		t.Fatalf("rerun capture calls = %d, want no second capture", captureCalls)
	}
	assertSpecMigrationReviewHumanOutput(t, secondOutput.String())
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 1)

	thirdOutput := &bytes.Buffer{}
	thirdCommand := &cobra.Command{}
	thirdCommand.SetOut(thirdOutput)
	thirdErr := runSpecMigrateV2OperationWithReviewCapture(
		thirdCommand,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		secondCapture,
	)
	if thirdErr != nil {
		t.Fatalf("idempotent already-admitted rerun: %v", thirdErr)
	}
	if captureCalls != 1 {
		t.Fatalf("already-admitted capture calls = %d, want no second capture", captureCalls)
	}
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 1)
}

func TestSpecMigrationReviewAdmissionPostTTYDriftKeepsSourceWithoutDomainEffect(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	captureCalls := 0
	drift := func() {
		drifted := append([]byte{}, fixture.targetBytes...)
		drifted = append(drifted, []byte("\n# drift after terminal SpeechAct\n")...)
		mustWriteCLISpecMigrationFile(t, fixture.reviewSoftwarePath, drifted)
	}
	capture := cliMigrationReviewFixtureCapture(t, &captureCalls, drift)
	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetOut(output)

	err := runSpecMigrateV2OperationWithReviewCapture(
		command,
		fixture.root,
		fixture.packetPath,
		specMigrationV2AdmitReviewOperation,
		capture,
	)
	if err == nil || !strings.Contains(err.Error(), "repeat complete migration observation after terminal capture") {
		t.Fatalf("post-TTY drift error = %v", err)
	}
	if captureCalls != 1 {
		t.Fatalf("post-TTY drift capture calls = %d, want 1", captureCalls)
	}
	assertCLIMigrationReviewCounts(t, fixture.root, 1, 0)
	if _, statErr := os.Stat(filepath.Join(fixture.root, fixture.finalTargetCarrier)); !os.IsNotExist(statErr) {
		t.Fatalf("post-TTY drift wrote final target: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, fixture.archiveCarrier)); !os.IsNotExist(statErr) {
		t.Fatalf("post-TTY drift wrote archive: %v", statErr)
	}
}

func TestWriteSpecMigrationV2ReviewAdmissionResultHidesOpaqueReferences(t *testing.T) {
	result := specMigrationV2ReviewAdmissionResult{
		reviewRef:             "review-admission:abc",
		reviewAdmissionDigest: "sha256:review",
		speechActRef:          "speech-act:manual:abc",
		speechActDigest:       "sha256:speech",
		projectRoot:           "/tmp/project",
		packetDigest:          "sha256:packet",
		packetCarrierDigest:   "sha256:carrier",
		partitionAuditStatus:  "verified",
		partitionAuditDigest:  "sha256:audit",
	}
	output := &bytes.Buffer{}
	if err := writeSpecMigrationV2ReviewAdmissionResult(output, result); err != nil {
		t.Fatal(err)
	}
	assertSpecMigrationReviewHumanOutput(t, output.String())
}

func assertSpecMigrationReviewHumanOutput(t *testing.T, output string) {
	t.Helper()
	for _, want := range []string{
		"Semantic review recorded.",
		"No specification files were changed.",
		"Run `haft spec migrate` again",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("semantic-review output omitted %q:\n%s", want, output)
		}
	}
	for _, opaque := range []string{
		"review-admission:abc",
		"speech-act:manual:abc",
		"sha256:",
		"/tmp/project",
		"review_ref",
		"speech_act_ref",
		"packet_carrier_digest",
		"partition_audit_digest",
	} {
		if strings.Contains(output, opaque) {
			t.Fatalf("semantic-review output exposes opaque internal value %q:\n%s", opaque, output)
		}
	}
}

func cliMigrationReviewFixtureCapture(
	t *testing.T,
	calls *int,
	afterCapture func(),
) specMigrationV2ReviewCapture {
	t.Helper()
	return func(
		_ context.Context,
		prepared specmigrationv2.PreparedMigrationReviewAdmission,
	) (authority.VerifiedSpeechActSource, error) {
		(*calls)++
		startedAt := time.Now().UTC().Add(-time.Second)
		observedAt := startedAt.Add(time.Nanosecond)
		endedAt := observedAt.Add(time.Nanosecond)
		source, err := specmigrationv2.CaptureVerifiedMigrationReviewForTestFixture(
			t,
			prepared,
			startedAt,
			observedAt,
			endedAt,
		)
		if err != nil {
			return authority.VerifiedSpeechActSource{}, err
		}
		if afterCapture != nil {
			afterCapture()
		}
		return source, nil
	}
}

func installCLIMigrationReviewFailureTrigger(t *testing.T, root string) {
	t.Helper()
	ledger := openCLIMigrationReviewLedgerForTest(t, root, projectledger.ReadWrite)
	defer ledger.Close()
	_, err := ledger.handle.Database().Exec(`CREATE TRIGGER inject_cli_migration_review_phase_two_failure
		BEFORE INSERT ON migration_review_acceptance_contents
		BEGIN
			SELECT RAISE(ABORT, 'injected CLI migration-review phase-two failure');
		END`)
	if err != nil {
		t.Fatalf("create CLI migration-review failure trigger: %v", err)
	}
}

func dropCLIMigrationReviewFailureTrigger(t *testing.T, root string) {
	t.Helper()
	ledger := openCLIMigrationReviewLedgerForTest(t, root, projectledger.ReadWrite)
	defer ledger.Close()
	_, err := ledger.handle.Database().Exec("DROP TRIGGER inject_cli_migration_review_phase_two_failure")
	if err != nil {
		t.Fatalf("drop CLI migration-review failure trigger: %v", err)
	}
}

func assertCLIMigrationReviewCounts(
	t *testing.T,
	root string,
	wantGeneric int,
	wantDomain int,
) {
	t.Helper()
	ledger := openCLIMigrationReviewLedgerForTest(t, root, projectledger.ReadOnly)
	defer ledger.Close()
	wants := map[string]struct {
		query string
		count int
	}{
		"migration review method": {
			query: "SELECT COUNT(*) FROM speech_act_method_descriptions WHERE method_ref = 'method:migration-review-acceptance'",
			count: wantGeneric,
		},
		"migration review policy": {
			query: "SELECT COUNT(*) FROM speech_act_context_policies WHERE context_policy_ref = 'context-policy:migration-review-acceptance:v2'",
			count: wantGeneric,
		},
		"migration review capture": {
			query: "SELECT COUNT(*) FROM terminal_capture_records WHERE capture_carrier_ref LIKE 'carrier:terminal-capture:migration-review:%'",
			count: wantGeneric,
		},
		"migration review assignment": {
			query: "SELECT COUNT(*) FROM speech_act_role_assignments WHERE bounded_context_ref = 'bounded-context:haft-spec-migration-v2'",
			count: wantGeneric,
		},
		"migration review SpeechAct": {
			query: "SELECT COUNT(*) FROM speech_acts WHERE speech_act_ref LIKE 'speech-act:migration-review:%'",
			count: wantGeneric,
		},
		"migration review content": {
			query: "SELECT COUNT(*) FROM migration_review_acceptance_contents",
			count: wantDomain,
		},
		"migration review admission": {
			query: "SELECT COUNT(*) FROM migration_review_admissions_v2",
			count: wantDomain,
		},
		"migration review effect": {
			query: "SELECT COUNT(*) FROM migration_review_instituted_effects",
			count: wantDomain,
		},
	}
	for name, want := range wants {
		var got int
		if err := ledger.handle.Database().QueryRow(want.query).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if got != want.count {
			t.Fatalf("%s count = %d, want %d", name, got, want.count)
		}
	}
}

func openCLIMigrationReviewLedgerForTest(
	t *testing.T,
	root string,
	access projectledger.Access,
) *specMigrationV2Ledger {
	t.Helper()
	ledger, err := openSpecMigrationV2Ledger(context.Background(), root, access)
	if err != nil {
		t.Fatalf("open CLI migration-review ledger: %v", err)
	}
	return ledger
}
