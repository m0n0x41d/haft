package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/spf13/cobra"
)

func TestMemoryTypeEnvGenesisCommandsExposeExplicitPrepareAndSelectBoundary(
	t *testing.T,
) {
	var typeEnvCommand *cobra.Command
	for _, command := range memoryCmd.Commands() {
		if command.Name() == memoryTypeEnvCmd.Name() {
			typeEnvCommand = command
			break
		}
	}
	if typeEnvCommand == nil {
		t.Fatal("memory typeenv command is not registered")
	}
	names := map[string]bool{}
	for _, command := range typeEnvCommand.Commands() {
		names[command.Name()] = true
	}
	for _, required := range []string{"prepare", "select"} {
		if !names[required] {
			t.Fatalf("memory typeenv omitted %q", required)
		}
	}
}

func TestMemoryTypeEnvPrepareAndSelectUsesOneHumanReadableReviewCarrier(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-genesis")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())

	prepareOutput := &bytes.Buffer{}
	prepareCommand := genesisTestCommand(prepareOutput)
	if err := runMemoryTypeEnvPrepare(prepareCommand, nil); err != nil {
		t.Fatalf("runMemoryTypeEnvPrepare(): %v", err)
	}
	prepared := genesisPrepareTestResponse{}
	if err := json.Unmarshal(prepareOutput.Bytes(), &prepared); err != nil {
		t.Fatalf("decode prepare response: %v", err)
	}
	if prepared.Result != "prepared_at_new_base" ||
		prepared.ProjectID != harness.ProjectID() ||
		prepared.Review.Title == "" ||
		prepared.Review.Choice == "" ||
		prepared.Candidate.GraphRevision != 0 ||
		prepared.Candidate.ProfilePosture != "compatible" ||
		prepared.ReviewCarrier.Path !=
			projectTypeEnvGenesisReviewRelativePath() {
		t.Fatalf("prepare response = %#v", prepared)
	}
	if prepared.ReviewCarrier.Digest == "" {
		t.Fatal("prepare response omitted review carrier digest")
	}
	carrierPath := projectTypeEnvGenesisReviewPath(harness.Root().String())
	info, err := os.Lstat(carrierPath)
	if err != nil {
		t.Fatalf("inspect review carrier: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("review carrier is not a regular non-symlink file")
	}
	observedReview, err := observeProjectTypeEnvGenesisReview(
		harness.Root().String(),
	)
	if err != nil {
		t.Fatalf("read human-readable review carrier: %v", err)
	}
	carrier := observedReview.carrier
	if observedReview.binding.Ref().String() !=
		projectTypeEnvGenesisReviewCarrierAuthorityRef ||
		observedReview.binding.Digest().String() !=
			prepared.ReviewCarrier.Digest {
		t.Fatalf(
			"observed review binding = %s / %s",
			observedReview.binding.Ref().String(),
			observedReview.binding.Digest().String(),
		)
	}
	if carrier.Review.Choice != prepared.Review.Choice ||
		carrier.Candidate != prepared.Candidate ||
		carrier.PreparationResult != prepared.Result {
		t.Fatalf("review carrier omitted or changed human review: %#v", carrier)
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"typed_memory_graph_heads",
		1,
	)

	selectOutput := &bytes.Buffer{}
	selectCommand := genesisTestCommand(selectOutput)
	if err := runMemoryTypeEnvSelect(selectCommand, nil); err != nil {
		t.Fatalf("runMemoryTypeEnvSelect(): %v", err)
	}
	selectedEnvelope := decodeGenesisSelectionEnvelope(
		t,
		selectOutput.Bytes(),
	)
	selected := projectTypeEnvGenesisFreshlyCommitted{}
	if err := json.Unmarshal(
		selectedEnvelope.Outcome,
		&selected,
	); err != nil {
		t.Fatalf("decode selection outcome: %v", err)
	}
	if selected.Kind != "freshly_committed" ||
		selectedEnvelope.ProjectID != harness.ProjectID() ||
		selectedEnvelope.AuthorityIngress != "explicit_h_decide" ||
		selected.CommittedClosure.HeadRevision != 1 ||
		selected.CommittedClosure.CommittedGraphRevision != 1 ||
		selected.CommittedClosure.SelectedCompositeRef !=
			prepared.Candidate.CompositeTypeEnvRef {
		t.Fatalf("selection response = %#v", selectedEnvelope)
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_heads",
		1,
	)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		1,
	)

	replayOutput := &bytes.Buffer{}
	replayCommand := genesisTestCommand(replayOutput)
	if err := runMemoryTypeEnvSelect(replayCommand, nil); err != nil {
		t.Fatalf("runMemoryTypeEnvSelect(replay): %v", err)
	}
	replayEnvelope := decodeGenesisSelectionEnvelope(
		t,
		replayOutput.Bytes(),
	)
	replayed := projectTypeEnvGenesisReplayedExisting{}
	if err := json.Unmarshal(
		replayEnvelope.Outcome,
		&replayed,
	); err != nil {
		t.Fatalf("decode replay outcome: %v", err)
	}
	if replayed.Kind != "replayed_existing" ||
		replayed.CommittedClosure.ReceiptRef !=
			selected.CommittedClosure.ReceiptRef ||
		replayed.CommittedClosure.ClosureRef !=
			selected.CommittedClosure.ClosureRef {
		t.Fatalf("replay response = %#v", replayEnvelope)
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		1,
	)

	carrier, err = readProjectTypeEnvGenesisReview(harness.Root().String())
	if err != nil {
		t.Fatalf("read exact review carrier: %v", err)
	}
	carrier.RequestDigest = "sha256:" +
		"0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := replaceProjectTypeEnvGenesisReview(
		harness.Root().String(),
		carrier,
	); err != nil {
		t.Fatalf("write tampered review carrier: %v", err)
	}
	tamperOutput := &bytes.Buffer{}
	tamperCommand := genesisTestCommand(tamperOutput)
	err = runMemoryTypeEnvSelect(tamperCommand, nil)
	if err == nil {
		t.Fatal("tampered review carrier was accepted")
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		1,
	)
}

func TestMemoryTypeEnvSelectStrictModeCapturesOnceThenReplaysWithoutPrompt(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-genesis-strict")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())

	configPath := filepath.Join(root, ".haft", "config.yaml")
	strictConfig := []byte(
		"schema_version: 1\n" +
			"authority:\n" +
			"  decision_binding_mode: explicit_h_decide\n" +
			"  project_typeenv_head_selection_mode: strict_cli_speech_act\n",
	)
	if err := os.WriteFile(configPath, strictConfig, 0o600); err != nil {
		t.Fatalf("write strict project config: %v", err)
	}
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err != nil {
		t.Fatalf("prepare strict Genesis review: %v", err)
	}

	capturer := &genesisCLISpeechActCapturer{t: t}
	selectedOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvSelectWithCapturer(
		genesisTestCommand(selectedOutput),
		capturer,
	); err != nil {
		t.Fatalf("select strict Genesis review: %v", err)
	}
	selectedEnvelope := decodeGenesisSelectionEnvelope(
		t,
		selectedOutput.Bytes(),
	)
	selected := projectTypeEnvGenesisFreshlyCommitted{}
	if err := json.Unmarshal(selectedEnvelope.Outcome, &selected); err != nil {
		t.Fatalf("decode strict selection outcome: %v", err)
	}
	if selected.Kind != "freshly_committed" ||
		selectedEnvelope.AuthorityIngress != "strict_speech_act_captured" ||
		capturer.calls != 1 {
		t.Fatalf(
			"strict selection = %#v, capture calls = %d",
			selectedEnvelope,
			capturer.calls,
		)
	}

	replayOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvSelectWithCapturer(
		genesisTestCommand(replayOutput),
		capturer,
	); err != nil {
		t.Fatalf("replay strict Genesis review: %v", err)
	}
	replayEnvelope := decodeGenesisSelectionEnvelope(t, replayOutput.Bytes())
	replayed := projectTypeEnvGenesisReplayedExisting{}
	if err := json.Unmarshal(replayEnvelope.Outcome, &replayed); err != nil {
		t.Fatalf("decode strict replay outcome: %v", err)
	}
	if replayed.Kind != "replayed_existing" ||
		replayEnvelope.AuthorityIngress != "strict_speech_act_replayed" ||
		capturer.calls != 1 {
		t.Fatalf(
			"strict replay = %#v, capture calls = %d",
			replayEnvelope,
			capturer.calls,
		)
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_heads",
		1,
	)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		1,
	)
}

func TestMemoryTypeEnvSelectRejectsReviewFromAnotherProject(
	t *testing.T,
) {
	sourceRoot := filepath.Join(t.TempDir(), "source")
	source := profileadmissionfixture.New(t, sourceRoot)
	source.AdmitSoftwareRevision(t, "memory-typeenv-source")
	t.Setenv(envProjectRoot, source.Root().String())
	t.Setenv(envExpectedProjectID, source.ProjectID())

	prepareOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(prepareOutput),
		nil,
	); err != nil {
		t.Fatalf("prepare source review: %v", err)
	}
	carrier, err := readProjectTypeEnvGenesisReview(source.Root().String())
	if err != nil {
		t.Fatalf("read source review carrier: %v", err)
	}

	targetRoot := filepath.Join(t.TempDir(), "target")
	target := profileadmissionfixture.New(t, targetRoot)
	target.AdmitSoftwareRevision(t, "memory-typeenv-target")
	if _, err := writeProjectTypeEnvGenesisReview(
		target.Root().String(),
		carrier,
	); err != nil {
		t.Fatalf("copy source review carrier to target: %v", err)
	}
	t.Setenv(envProjectRoot, target.Root().String())
	t.Setenv(envExpectedProjectID, target.ProjectID())

	err = runMemoryTypeEnvSelect(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	)
	if err == nil {
		t.Fatal("Genesis review from another project was accepted")
	}
	assertGenesisSelectionTableCount(
		t,
		target,
		"project_typeenv_heads",
		0,
	)
	assertGenesisSelectionTableCount(
		t,
		target,
		"project_typeenv_head_selection_receipts",
		0,
	)
}

func TestMemoryTypeEnvSelectRejectsExpiredReviewWithoutReceipt(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-expired")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())

	ledger, _, err := openProjectTypeEnvGenesisLedger(
		context.Background(),
		projectledger.ReadWrite,
	)
	if err != nil {
		t.Fatalf("open Genesis ledger: %v", err)
	}
	runtime, err := loadEmbeddedMemoryRuntime(context.Background())
	if err != nil {
		_ = ledger.Close()
		t.Fatalf("load embedded memory runtime: %v", err)
	}
	prepared, err := prepareProjectTypeEnvGenesisReview(
		context.Background(),
		ledger,
		runtime.Artifact(),
		genesisTestClock{value: time.Now().Add(-48 * time.Hour)},
	)
	if err != nil {
		_ = ledger.Close()
		t.Fatalf("prepare expired Genesis review: %v", err)
	}
	if _, err := writeProjectTypeEnvGenesisReview(
		harness.Root().String(),
		prepared.carrier,
	); err != nil {
		_ = ledger.Close()
		t.Fatalf("write expired Genesis review: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close Genesis ledger: %v", err)
	}

	output := &bytes.Buffer{}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(output),
		nil,
	); err != nil {
		t.Fatalf("select expired Genesis review: %v", err)
	}
	envelope := decodeGenesisSelectionEnvelope(t, output.Bytes())
	result := projectTypeEnvGenesisNotSelected{}
	if err := json.Unmarshal(envelope.Outcome, &result); err != nil {
		t.Fatalf("decode expired selection outcome: %v", err)
	}
	if result.Kind != "not_selected" ||
		result.Reason != "review_expired" ||
		result.Repair !=
			"prepare a fresh exact review carrier; another h-decide cannot revive expired authorization content" {
		t.Fatalf("expired selection response = %#v", envelope)
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		0,
	)
}

func TestMemoryTypeEnvPrepareBlocksManualSelectionWithoutCompatibleProfile(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())

	output := &bytes.Buffer{}
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(output),
		nil,
	); err != nil {
		t.Fatalf("prepare profile-blocked Genesis review: %v", err)
	}
	prepared := genesisPrepareTestResponse{}
	if err := json.Unmarshal(output.Bytes(), &prepared); err != nil {
		t.Fatalf("decode profile-blocked prepare response: %v", err)
	}
	if prepared.Review.Readiness.Posture != "blocked" ||
		len(prepared.Review.Readiness.Reasons) == 0 ||
		prepared.Interpretation.NextHumanGate != "" ||
		prepared.Review.Validity.From == "" ||
		prepared.Review.Validity.Until == "" {
		t.Fatalf("profile-blocked prepare response = %#v", prepared)
	}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err == nil {
		t.Fatal("profile-blocked Genesis review reached selection")
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_heads",
		0,
	)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		0,
	)
}

func TestReadProjectTypeEnvGenesisReviewRejectsOversizedCarrier(
	t *testing.T,
) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".haft"), 0o700); err != nil {
		t.Fatalf("create .haft directory: %v", err)
	}
	seed := projectTypeEnvGenesisReviewCarrier{
		Schema:    projectTypeEnvGenesisReviewSchema,
		ProjectID: "project:seed",
	}
	if _, err := writeProjectTypeEnvGenesisReview(root, seed); err != nil {
		t.Fatalf("seed review carrier: %v", err)
	}
	path := projectTypeEnvGenesisReviewPath(root)
	content := bytes.Repeat(
		[]byte{'x'},
		maximumProjectTypeEnvGenesisReviewBytes+1,
	)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write oversized review carrier: %v", err)
	}

	_, err := readProjectTypeEnvGenesisReview(root)
	if err == nil {
		t.Fatal("oversized Genesis review carrier was accepted")
	}
}

func TestWriteProjectTypeEnvGenesisReviewDoesNotSilentlyReplaceReview(
	t *testing.T,
) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".haft"), 0o700); err != nil {
		t.Fatalf("create .haft directory: %v", err)
	}
	path := projectTypeEnvGenesisReviewPath(root)
	first := projectTypeEnvGenesisReviewCarrier{
		Schema:    projectTypeEnvGenesisReviewSchema,
		ProjectID: "project:first",
	}
	second := projectTypeEnvGenesisReviewCarrier{
		Schema:    projectTypeEnvGenesisReviewSchema,
		ProjectID: "project:second",
	}
	if _, err := writeProjectTypeEnvGenesisReview(root, first); err != nil {
		t.Fatalf("write first Genesis review: %v", err)
	}
	firstBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first Genesis review: %v", err)
	}
	if _, err := writeProjectTypeEnvGenesisReview(root, second); err == nil {
		t.Fatal("different Genesis review silently replaced the reviewed carrier")
	} else if strings.Contains(err.Error(), "sha256:") {
		t.Fatalf("no-clobber error asked the operator to interpret a digest: %v", err)
	}
	retainedBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained Genesis review: %v", err)
	}
	if !bytes.Equal(retainedBytes, firstBytes) {
		t.Fatal("failed no-clobber write changed the reviewed carrier")
	}
	if _, err := replaceProjectTypeEnvGenesisReview(
		root,
		second,
	); err != nil {
		t.Fatalf("explicitly replace Genesis review: %v", err)
	}
	replaced, err := readProjectTypeEnvGenesisReview(root)
	if err != nil {
		t.Fatalf("read explicitly replaced Genesis review: %v", err)
	}
	if replaced.ProjectID != second.ProjectID {
		t.Fatalf("replaced Genesis review = %#v", replaced)
	}
}

func TestProjectTypeEnvGenesisResultResponsePreservesConflictAndUnknownCoordinates(
	t *testing.T,
) {
	key, err :=
		projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
			"genesis:test-result-contract",
		)
	if err != nil {
		t.Fatalf("new Genesis result key: %v", err)
	}
	existingRequest := genesisTestTypedDigest(t, '1')
	presentedRequest := genesisTestTypedDigest(t, '2')
	existingContent := genesisTestAuthorityDigest(t, '3')
	presentedContent := genesisTestAuthorityDigest(t, '4')
	conflict, err := projecttypeenvselectioneffect.NewReplayConflict(
		projecttypeenvselectioneffect.ReplayConflictInput{
			Key:                    key,
			ExistingRequestDigest:  existingRequest,
			PresentedRequestDigest: presentedRequest,
			ExistingContentDigest:  existingContent,
			PresentedContentDigest: presentedContent,
		},
	)
	if err != nil {
		t.Fatalf("new Genesis replay conflict: %v", err)
	}
	conflictResponse, err := projectTypeEnvGenesisResultResponse(
		"project:test",
		conflict,
	)
	if err != nil {
		t.Fatalf("project Genesis replay conflict: %v", err)
	}
	conflictWire, ok :=
		conflictResponse.Outcome.(projectTypeEnvGenesisReplayConflict)
	if !ok ||
		conflictWire.IdempotencyKey != key.String() ||
		conflictWire.ExistingRequestDigest != existingRequest.String() ||
		conflictWire.PresentedRequestDigest != presentedRequest.String() ||
		conflictWire.ExistingContentDigest != existingContent.String() ||
		conflictWire.PresentedContentDigest != presentedContent.String() {
		t.Fatalf("replay-conflict projection = %#v", conflictResponse.Outcome)
	}

	unknown, err := projecttypeenvselectioneffect.NewCommitOutcomeUnknown(
		projecttypeenvselectioneffect.CommitOutcomeUnknownInput{
			RetryKey:      key,
			RequestDigest: presentedRequest,
			ContentDigest: presentedContent,
		},
	)
	if err != nil {
		t.Fatalf("new Genesis unknown outcome: %v", err)
	}
	unknownResponse, err := projectTypeEnvGenesisResultResponse(
		"project:test",
		unknown,
	)
	if err != nil {
		t.Fatalf("project Genesis unknown outcome: %v", err)
	}
	unknownWire, ok :=
		unknownResponse.Outcome.(projectTypeEnvGenesisCommitOutcomeUnknown)
	if !ok ||
		unknownWire.RetryKey != key.String() ||
		unknownWire.RequestDigest != presentedRequest.String() ||
		unknownWire.ContentDigest != presentedContent.String() ||
		len(unknownResponse.Interpretation.DoesNotEstablish) != 2 {
		t.Fatalf("unknown-outcome projection = %#v", unknownResponse)
	}
}

type genesisTestClock struct {
	value time.Time
}

func (clock genesisTestClock) Now() time.Time {
	return clock.value
}

type genesisCLISpeechActCapturer struct {
	t     testing.TB
	calls int
}

func (capturer *genesisCLISpeechActCapturer) Capture(
	_ context.Context,
	prepared authority.PreparedManualSpeechAct,
) (authority.VerifiedSpeechActSource, error) {
	capturer.calls++
	startedAt := time.Now().Add(-2 * time.Millisecond).Round(0).UTC()
	return authority.CaptureVerifiedSpeechActForTestFixture(
		capturer.t,
		prepared,
		startedAt,
		startedAt.Add(time.Millisecond),
		startedAt.Add(2*time.Millisecond),
	)
}

func genesisTestTypedDigest(
	t *testing.T,
	fill byte,
) typedmemory.SHA256Digest {
	t.Helper()

	value, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(string(fill), 64),
	)
	if err != nil {
		t.Fatalf("new typed-memory digest: %v", err)
	}
	return value
}

func genesisTestAuthorityDigest(
	t *testing.T,
	fill byte,
) authority.Digest {
	t.Helper()

	value, err := authority.NewDigest(
		"sha256:" + strings.Repeat(string(fill), 64),
	)
	if err != nil {
		t.Fatalf("new authority digest: %v", err)
	}
	return value
}

type genesisSelectionTestEnvelope struct {
	ContractVersion              string                                       `json:"contract_version"`
	Action                       string                                       `json:"action"`
	ProjectID                    string                                       `json:"project_id"`
	AuthorityIngress             string                                       `json:"authority_ingress"`
	Outcome                      json.RawMessage                              `json:"outcome"`
	PostEffectLedgerRevalidation json.RawMessage                              `json:"post_effect_ledger_revalidation"`
	Interpretation               projectTypeEnvGenesisSelectionInterpretation `json:"interpretation"`
}

type genesisPrepareTestResponse struct {
	ContractVersion               string                                    `json:"contract_version"`
	Action                        string                                    `json:"action"`
	Result                        string                                    `json:"result"`
	ProjectID                     string                                    `json:"project_id"`
	Review                        projectTypeEnvGenesisHumanReview          `json:"review"`
	Candidate                     projectTypeEnvGenesisCandidateResponse    `json:"candidate"`
	ReviewCarrier                 projectTypeEnvGenesisReviewCarrierRef     `json:"review_carrier"`
	PostPrepareLedgerRevalidation json.RawMessage                           `json:"post_prepare_ledger_revalidation"`
	Interpretation                projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
}

func decodeGenesisSelectionEnvelope(
	t *testing.T,
	raw []byte,
) genesisSelectionTestEnvelope {
	t.Helper()

	envelope := genesisSelectionTestEnvelope{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode Genesis selection envelope: %v", err)
	}
	if len(envelope.Outcome) == 0 ||
		len(envelope.PostEffectLedgerRevalidation) == 0 {
		t.Fatalf("Genesis selection envelope is incomplete: %#v", envelope)
	}
	return envelope
}

func genesisTestCommand(output *bytes.Buffer) *cobra.Command {
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(output)
	command.SetErr(output)
	return command
}

func assertGenesisSelectionTableCount(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
	table string,
	want int64,
) {
	t.Helper()

	query := "SELECT COUNT(*) FROM " + table
	count := int64(0)
	if err := harness.Database().QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
