package authority

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestResolveRecordedSpeechActSourceMissing(t *testing.T) {
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "speech-act-source.sqlite"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ref, err := NewSpeechActRef("speech-act:missing")
	if err != nil {
		t.Fatalf("NewSpeechActRef: %v", err)
	}

	recorded, found, err := ResolveRecordedSpeechActSource(
		context.Background(),
		store.GetRawDB(),
		ref,
	)
	if err != nil {
		t.Fatalf("ResolveRecordedSpeechActSource: %v", err)
	}
	if found || recorded.Valid() {
		t.Fatal("missing SpeechAct ref resolved a durable source")
	}
}

func TestResolveRecordedSpeechActSourceFound(t *testing.T) {
	database, verified, recorded := recordSpeechActSourceResolverFixture(t)
	ref, _ := recorded.SpeechActRef()
	wantDigest, _ := recorded.SpeechActDigest()

	resolved, found, err := ResolveRecordedSpeechActSource(
		context.Background(),
		database,
		ref,
	)
	if err != nil {
		t.Fatalf("ResolveRecordedSpeechActSource: %v", err)
	}
	if !found {
		t.Fatal("durable SpeechAct source was not found by canonical ref")
	}
	gotDigest, digestOK := resolved.SpeechActDigest()
	if !digestOK || gotDigest != wantDigest {
		t.Fatal("resolved SpeechAct source changed its canonical digest")
	}
	assertRecordedSpeechActSourceBindings(t, verified, resolved)
}

func TestLoadSpeechActContextPolicyRecomputesExactIndependentBasisMember(t *testing.T) {
	database, _, recorded := recordSpeechActSourceResolverFixture(t)
	ref, refOK := recorded.ContextPolicyRef()
	digest, digestOK := recorded.ContextPolicyDigest()
	if !refOK || !digestOK {
		t.Fatal("recorded SpeechAct source has no context-policy identity")
	}

	policy, err := LoadSpeechActContextPolicy(
		context.Background(),
		database,
		ref,
		digest,
	)
	if err != nil {
		t.Fatalf("LoadSpeechActContextPolicy: %v", err)
	}
	loadedRef, loadedRefOK := policy.Ref()
	loadedDigest, loadedDigestOK := policy.Digest()
	if !loadedRefOK || !loadedDigestOK || loadedRef != ref || loadedDigest != digest {
		t.Fatal("independently loaded context policy changed its exact identity")
	}

	wrongDigest, err := NewDigest("sha256:" + strings.Repeat("f", 64))
	if err != nil {
		t.Fatalf("NewDigest: %v", err)
	}
	_, err = LoadSpeechActContextPolicy(
		context.Background(),
		database,
		ref,
		wrongDigest,
	)
	if err == nil {
		t.Fatal("context-policy loader accepted a foreign digest")
	}
}

func TestResolveRecordedSpeechActSourceRejectsTamperedStoredDigest(t *testing.T) {
	tests := []struct {
		name   string
		digest string
	}{
		{name: "malformed", digest: "sha256:" + strings.Repeat("z", 64)},
		{name: "canonical collision", digest: "sha256:" + strings.Repeat("f", 64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database, _, recorded := recordSpeechActSourceResolverFixture(t)
			ref, _ := recorded.SpeechActRef()
			_, err := database.Exec("DROP TRIGGER speech_acts_no_update")
			if err != nil {
				t.Fatalf("drop immutable-row guard for tamper fixture: %v", err)
			}
			_, err = database.Exec(
				"UPDATE speech_acts SET speech_act_digest = ? WHERE speech_act_ref = ?",
				test.digest,
				ref.String(),
			)
			if err != nil {
				t.Fatalf("tamper stored SpeechAct digest: %v", err)
			}

			resolved, found, err := ResolveRecordedSpeechActSource(
				context.Background(),
				database,
				ref,
			)
			if err == nil || found || resolved.Valid() {
				t.Fatal("resolver accepted tampered or colliding SpeechAct material")
			}
		})
	}
}

func recordSpeechActSourceResolverFixture(
	t *testing.T,
) (*sql.DB, VerifiedSpeechActSource, RecordedSpeechActSource) {
	t.Helper()
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "speech-act-source.sqlite"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	verified := testVerifiedAuthorityAct(t).state.source
	writer, err := OpenSpeechActSourceWriter(database)
	if err != nil {
		t.Fatalf("OpenSpeechActSourceWriter: %v", err)
	}
	recorded, err := writer.Record(context.Background(), verified)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	return database, verified, recorded
}

func TestSpeechActSourceWriterRecordSurvivesLaterEffectFailureAndReplaysExactly(
	t *testing.T,
) {
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "speech-act-source.sqlite"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	verified := testVerifiedAuthorityAct(t).state.source
	writer, err := OpenSpeechActSourceWriter(database)
	if err != nil {
		t.Fatalf("OpenSpeechActSourceWriter: %v", err)
	}

	recorded, err := writer.Record(context.Background(), verified)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	assertRecordedSpeechActSourceBindings(t, verified, recorded)
	ref, _ := recorded.SpeechActRef()
	digest, _ := recorded.SpeechActDigest()

	_, err = database.Exec(`CREATE TABLE simulated_speech_act_domain_effects (
		effect_ref TEXT PRIMARY KEY
	)`)
	if err != nil {
		t.Fatalf("create simulated domain-effect table: %v", err)
	}
	simulatedFailure := errors.New("simulated later domain effect failure")
	err = simulateFailedSpeechActDomainEffect(database, simulatedFailure)
	if !errors.Is(err, simulatedFailure) {
		t.Fatalf("simulated domain effect: %v", err)
	}

	durable, err := LoadRecordedSpeechActSource(
		context.Background(),
		database,
		ref,
		digest,
	)
	if err != nil {
		t.Fatalf("LoadRecordedSpeechActSource after effect rollback: %v", err)
	}
	assertRecordedSpeechActSourceBindings(t, verified, durable)
	assertTableRowCount(t, database, "simulated_speech_act_domain_effects", 0)
	assertTableRowCount(t, database, "terminal_capture_records", 1)
	assertTableRowCount(t, database, "speech_acts", 1)

	writer.now = func() time.Time {
		panic("exact SpeechAct source replay attempted to restage canonical rows")
	}
	replayed, err := writer.Record(context.Background(), verified)
	if err != nil {
		t.Fatalf("Record exact retry: %v", err)
	}
	assertRecordedSpeechActSourceBindings(t, verified, replayed)
	assertTableRowCount(t, database, "terminal_capture_records", 1)
	assertTableRowCount(t, database, "speech_acts", 1)
}

func simulateFailedSpeechActDomainEffect(
	database *sql.DB,
	simulatedFailure error,
) error {
	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		return err
	}
	_, err = transaction.Execute(
		context.Background(),
		"INSERT INTO simulated_speech_act_domain_effects (effect_ref) VALUES (?)",
		[]any{"effect:simulated"},
	)
	if err != nil {
		finish := transaction.Rollback(context.Background())
		return errors.Join(err, finish.Err())
	}
	finish := transaction.Rollback(context.Background())
	return errors.Join(simulatedFailure, finish.Err())
}

func assertTableRowCount(
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
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func assertRecordedSpeechActSourceBindings(
	t *testing.T,
	verified VerifiedSpeechActSource,
	recorded RecordedSpeechActSource,
) {
	t.Helper()
	wantIntent, wantIntentOK := verified.PreparedIntentDigest()
	gotIntent, gotIntentOK := recorded.PreparedIntentDigest()
	wantWindow, wantWindowOK := verified.WorkWindow()
	gotWindow, gotWindowOK := recorded.WorkWindow()
	wantCompleted, wantCompletedOK := verified.CompletedAt()
	gotCompleted, gotCompletedOK := recorded.CompletedAt()
	wantPerformer, wantPerformerOK := verified.PerformedByRoleAssignmentRef()
	gotPerformer, gotPerformerOK := recorded.PerformedByRoleAssignmentRef()
	wantPerformerDigest, wantPerformerDigestOK := verified.PerformedByRoleAssignmentDigest()
	gotPerformerDigest, gotPerformerDigestOK := recorded.PerformedByRoleAssignmentDigest()
	complete := verified.Valid() && recorded.Valid() &&
		wantIntentOK && gotIntentOK &&
		wantWindowOK && gotWindowOK &&
		wantCompletedOK && gotCompletedOK &&
		wantPerformerOK && gotPerformerOK &&
		wantPerformerDigestOK && gotPerformerDigestOK
	if !complete {
		t.Fatal("durable SpeechAct source omitted generic domain-effect bindings")
	}
	if gotIntent != wantIntent || gotWindow != wantWindow ||
		gotCompleted != wantCompleted || gotPerformer != wantPerformer ||
		gotPerformerDigest != wantPerformerDigest {
		t.Fatal("durable SpeechAct source changed generic domain-effect bindings")
	}
}

func TestRecordedSpeechActSourceLoaderRejectsMalformedStoredReference(t *testing.T) {
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "speech-act-source.sqlite"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	verified := testVerifiedAuthorityAct(t).state.source
	writer, err := OpenSpeechActSourceWriter(database)
	if err != nil {
		t.Fatalf("OpenSpeechActSourceWriter: %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	result, err := writer.RecordInTransaction(context.Background(), transaction, verified)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("Record: %v", err)
	}
	if finish := transaction.Commit(context.Background()); finish.Err() != nil {
		t.Fatalf("commit SpeechAct source: %v", finish.Err())
	}
	recorded, ok := result.RecordedSource()
	if !ok {
		t.Fatal("stored SpeechAct source is unavailable")
	}
	assertRecordedSpeechActEffectIdentity(t, verified, recorded)
	ref, _ := recorded.SpeechActRef()
	digest, _ := recorded.SpeechActDigest()
	_, err = database.Exec("DROP TRIGGER speech_acts_no_update")
	if err != nil {
		t.Fatalf("drop immutable-row guard for corruption fixture: %v", err)
	}
	_, err = database.Exec(`UPDATE speech_acts
		SET executed_within_ref = 'invalid' || char(10) || 'system',
			canonical_json = json_set(
				canonical_json,
				'$.executed_within_system_ref',
				'invalid' || char(10) || 'system'
			)
		WHERE speech_act_ref = ?`, ref.String())
	if err != nil {
		t.Fatalf("inject malformed stored SpeechAct ref: %v", err)
	}

	_, err = LoadRecordedSpeechActSource(context.Background(), database, ref, digest)
	if err == nil {
		t.Fatal("loader accepted a malformed stored system reference")
	}
}

func assertRecordedSpeechActEffectIdentity(
	t *testing.T,
	verified VerifiedSpeechActSource,
	recorded RecordedSpeechActSource,
) {
	t.Helper()
	wantObject, wantObjectOK := verified.InstitutedObjectRef()
	gotObject, gotObjectOK := recorded.InstitutedObjectRef()
	wantPolicy, wantPolicyOK := verified.ContextPolicyRef()
	gotPolicy, gotPolicyOK := recorded.ContextPolicyRef()
	wantPolicyDigest, wantDigestOK := verified.ContextPolicyDigest()
	gotPolicyDigest, gotDigestOK := recorded.ContextPolicyDigest()
	complete := wantObjectOK && gotObjectOK && wantPolicyOK && gotPolicyOK &&
		wantDigestOK && gotDigestOK
	if !complete {
		t.Fatal("SpeechAct source omitted domain-effect identity")
	}
	if gotObject != wantObject || gotPolicy != wantPolicy || gotPolicyDigest != wantPolicyDigest {
		t.Fatal("recorded SpeechAct source changed its instituted object or context policy")
	}
}

func TestRecordedSpeechActSourceHasNoVerifiedCompletionCapability(t *testing.T) {
	typeOfSource := reflect.TypeOf(RecordedSpeechActSource{})
	forbidden := map[string]struct{}{
		"VerifiedSource":          {},
		"VerifiedAuthorityAct":    {},
		"CompleteAuthorityAct":    {},
		"PreparedSpeechActIntent": {},
	}
	for index := 0; index < typeOfSource.NumMethod(); index++ {
		name := typeOfSource.Method(index).Name
		if _, found := forbidden[name]; found {
			t.Fatalf("RecordedSpeechActSource exposes completion capability %q", name)
		}
	}
}
