package projecttypeenvselectionauthority

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/authority"
)

func TestPrepareStrictCLISpeechActSealsLiteralPhraseAndExactRecord(
	t *testing.T,
) {
	fixture := buildAuthorityFixture(t)
	preparation := strictCLIPreparationFixture(t, fixture)

	if got := StrictCLITypeEnvSelectionPhrase(); got !=
		"AUTHORIZE THIS REVIEWED TYPEENV SELECTION" {
		t.Fatalf("strict CLI phrase = %q", got)
	}
	if err := preparation.Verify(); err != nil {
		t.Fatalf("Verify preparation: %v", err)
	}
	startedAt := fixture.content.ValidityWindow().From().Add(time.Minute)
	basis, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		preparation.PreparedSpeechAct(),
		startedAt,
		startedAt.Add(time.Second),
		startedAt.Add(2*time.Second),
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	record, err := preparation.SealCaptured(basis)
	if err != nil {
		t.Fatalf("SealCaptured: %v", err)
	}
	if err := record.Verify(fixture.request); err != nil {
		t.Fatalf("Verify record: %v", err)
	}
	if record.Content().Digest() != fixture.content.Digest() {
		t.Fatal("strict record changed reviewed content")
	}
	if record.PermissionRecord().Modality().String() != "MAY" {
		t.Fatal("strict record did not institute a bounded MAY Permission")
	}
}

func strictCLIPreparationFixture(
	t *testing.T,
	fixture authorityFixture,
) StrictCLISpeechActPreparation {
	t.Helper()
	configRef, err := authority.NewCarrierRef("carrier:.haft/config.yaml")
	if err != nil {
		t.Fatalf("NewCarrierRef(config): %v", err)
	}
	configCarrier, err := authority.NewObservableCarrierBinding(
		configRef,
		mustAuthorityDigest(t, "e"),
	)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding(config): %v", err)
	}
	reviewRef, err := authority.NewCarrierRef(
		"carrier:.haft/project-typeenv-genesis-review.json",
	)
	if err != nil {
		t.Fatalf("NewCarrierRef(review): %v", err)
	}
	reviewCarrier, err := authority.NewObservableCarrierBinding(
		reviewRef,
		mustAuthorityDigest(t, "f"),
	)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding(review): %v", err)
	}
	preparation, err := PrepareStrictCLISpeechAct(
		StrictCLISpeechActPreparationInput{
			Request:        fixture.request,
			Content:        fixture.content,
			Stage:          fixture.stage,
			ProjectBinding: fixture.binding,
			ConfigCarrier:  configCarrier,
			ReviewCarrier:  reviewCarrier,
		},
	)
	if err != nil {
		t.Fatalf("PrepareStrictCLISpeechAct: %v", err)
	}
	return preparation
}

type strictCLICapturerFixture struct {
	t           testing.TB
	preparation StrictCLISpeechActPreparation
	startedAt   time.Time
	err         error
	calls       int
}

func (capturer *strictCLICapturerFixture) Capture(
	_ context.Context,
	prepared authority.PreparedManualSpeechAct,
) (authority.VerifiedSpeechActSource, error) {
	capturer.calls++
	if capturer.err != nil {
		return authority.VerifiedSpeechActSource{}, capturer.err
	}
	gotDigest, gotOK := prepared.ReviewDigest()
	wantDigest, wantOK := capturer.preparation.PreparedSpeechAct().ReviewDigest()
	if !gotOK || !wantOK || gotDigest != wantDigest {
		return authority.VerifiedSpeechActSource{},
			errors.New("capturer received another prepared review")
	}
	return authority.CaptureVerifiedSpeechActForTestFixture(
		capturer.t,
		prepared,
		capturer.startedAt,
		capturer.startedAt.Add(time.Second),
		capturer.startedAt.Add(2*time.Second),
	)
}

func TestStrictCLISourceStoreCapturesAtomicallyThenReplaysWithoutPrompt(
	t *testing.T,
) {
	fixture := buildAuthorityFixture(t)
	preparation := strictCLIPreparationFixture(t, fixture)
	database := strictCLISourceDatabase(t)
	capturer := &strictCLICapturerFixture{
		t:           t,
		preparation: preparation,
		startedAt:   fixture.content.ValidityWindow().From().Add(time.Minute),
	}
	store, err := OpenStrictCLISpeechActSourceStore(database, capturer)
	if err != nil {
		t.Fatalf("OpenStrictCLISpeechActSourceStore: %v", err)
	}

	first, err := store.ResolveOrCapture(context.Background(), preparation)
	if err != nil {
		t.Fatalf("ResolveOrCapture(first): %v", err)
	}
	captured, capturedOK := first.Captured()
	if !capturedOK {
		t.Fatalf("first result = %#v, want captured", first)
	}
	if err := captured.Record().Verify(fixture.request); err != nil {
		t.Fatalf("captured record: %v", err)
	}
	if capturer.calls != 1 {
		t.Fatalf("capture calls after first = %d, want 1", capturer.calls)
	}

	capturer.err = errors.New("terminal must not reopen")
	second, err := store.ResolveOrCapture(context.Background(), preparation)
	if err != nil {
		t.Fatalf("ResolveOrCapture(replay): %v", err)
	}
	replayed, replayedOK := second.Replayed()
	if !replayedOK {
		t.Fatalf("second result = %#v, want replayed", second)
	}
	if replayed.Record().Digest() != captured.Record().Digest() {
		t.Fatal("durable replay changed exact TypeEnv SpeechAct record")
	}
	if capturer.calls != 1 {
		t.Fatalf("capture calls after replay = %d, want 1", capturer.calls)
	}
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_selection_speech_act_records",
		1,
	)
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_selection_permissions_v3",
		1,
	)
	assertStrictCLISourceCount(t, database, "speech_acts", 1)
	assertStrictCLISourceCount(t, database, "project_typeenv_heads", 0)
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_selection_authority_uses",
		0,
	)
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_cas_work_records",
		0,
	)
}

func TestStrictCLISourceStoreCaptureFailuresWriteNothing(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "wrong phrase", err: errors.New("SpeechAct rejected: wrong phrase")},
		{name: "EOF", err: io.EOF},
		{name: "cancel", err: context.Canceled},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := buildAuthorityFixture(t)
			preparation := strictCLIPreparationFixture(t, fixture)
			database := strictCLISourceDatabase(t)
			capturer := &strictCLICapturerFixture{
				t:           t,
				preparation: preparation,
				startedAt:   fixture.content.ValidityWindow().From().Add(time.Minute),
				err:         test.err,
			}
			store, err := OpenStrictCLISpeechActSourceStore(
				database,
				capturer,
			)
			if err != nil {
				t.Fatalf("OpenStrictCLISpeechActSourceStore: %v", err)
			}
			_, err = store.ResolveOrCapture(
				context.Background(),
				preparation,
			)
			if !errors.Is(err, test.err) &&
				(err == nil || !errors.Is(test.err, io.EOF)) {
				t.Fatalf("ResolveOrCapture error = %v, want %v", err, test.err)
			}
			assertStrictCLISourceCount(t, database, "speech_acts", 0)
			assertStrictCLISourceCount(
				t,
				database,
				"project_typeenv_head_selection_speech_act_records",
				0,
			)
			assertStrictCLISourceCount(
				t,
				database,
				"project_typeenv_head_selection_permissions_v3",
				0,
			)
		})
	}
}

func TestStrictCLISourceStoreRollsBackGenericAndTypedRowsTogether(
	t *testing.T,
) {
	fixture := buildAuthorityFixture(t)
	preparation := strictCLIPreparationFixture(t, fixture)
	database := strictCLISourceDatabase(t)
	_, err := database.Exec(`
		CREATE TRIGGER strict_cli_source_test_abort
		BEFORE INSERT ON project_typeenv_head_selection_permissions_v3
		BEGIN
			SELECT RAISE(ABORT, 'simulated typed Permission failure');
		END`)
	if err != nil {
		t.Fatalf("install abort trigger: %v", err)
	}
	capturer := &strictCLICapturerFixture{
		t:           t,
		preparation: preparation,
		startedAt:   fixture.content.ValidityWindow().From().Add(time.Minute),
	}
	store, err := OpenStrictCLISpeechActSourceStore(database, capturer)
	if err != nil {
		t.Fatalf("OpenStrictCLISpeechActSourceStore: %v", err)
	}
	_, err = store.ResolveOrCapture(context.Background(), preparation)
	if err == nil {
		t.Fatal("ResolveOrCapture unexpectedly survived Permission failure")
	}
	assertStrictCLISourceCount(t, database, "speech_acts", 0)
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_selection_speech_act_records",
		0,
	)
	assertStrictCLISourceCount(
		t,
		database,
		"project_typeenv_head_selection_permissions_v3",
		0,
	)
}

func strictCLISourceDatabase(t *testing.T) *sql.DB {
	t.Helper()
	store, err := kerneldb.NewStore(
		filepath.Join(t.TempDir(), "strict-cli-source.sqlite"),
	)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	_, err = database.Exec(`
		PRAGMA foreign_keys = OFF;
		DROP TRIGGER project_typeenv_head_selection_requests_v48_exact_predecessor;
		DROP TRIGGER project_typeenv_head_selection_permissions_v3_v48_exact_source`)
	if err != nil {
		t.Fatalf("relax unrelated fixture dependencies: %v", err)
	}
	return database
}

func assertStrictCLISourceCount(
	t *testing.T,
	database *sql.DB,
	table string,
	want int,
) {
	t.Helper()
	var got int
	err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got)
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}
