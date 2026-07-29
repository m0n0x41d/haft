package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type genesisStrictCapturer struct {
	t         testing.TB
	startedAt time.Time
	err       error
	calls     int
}

func (capturer *genesisStrictCapturer) Capture(
	_ context.Context,
	prepared authority.PreparedManualSpeechAct,
) (authority.VerifiedSpeechActSource, error) {
	capturer.calls++
	if capturer.err != nil {
		return authority.VerifiedSpeechActSource{}, capturer.err
	}
	return authority.CaptureVerifiedSpeechActForTestFixture(
		capturer.t,
		prepared,
		capturer.startedAt,
		capturer.startedAt.Add(time.Second),
		capturer.startedAt.Add(2*time.Second),
	)
}

func TestGenesisServiceConsumesFreshPredurableStrictSource(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	preparation, store, capturer := newGenesisStrictSourceFixture(t, fixture)
	record := captureGenesisStrictSource(t, store, preparation)
	result := selectGenesisWithStrictSource(t, fixture, preparation, record)
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("strict SelectGenesis = %T, want FreshlyCommitted", result)
	}
	if capturer.calls != 1 {
		t.Fatalf("strict terminal capture calls = %d, want 1", capturer.calls)
	}
	assertGenesisE2ECommittedFootprint(t, fixture, fresh.Closure())
	assertGenesisStrictSourceExact(t, fixture, preparation, record)
}

func TestGenesisServiceStrictSourceSurvivesFailureAndResumesWithoutPrompt(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	preparation, store, capturer := newGenesisStrictSourceFixture(t, fixture)
	record := captureGenesisStrictSource(t, store, preparation)
	installGenesisReceiptAbortTrigger(t, fixture.database)
	_, err := selectGenesisWithStrictSourceResult(
		fixture,
		preparation,
		record,
	)
	if err == nil {
		t.Fatal("strict SelectGenesis unexpectedly survived injected abort")
	}
	if _, dropErr := fixture.database.Exec(
		"DROP TRIGGER genesis_test_abort_before_receipt",
	); dropErr != nil {
		t.Fatalf("drop injected strict retry trigger: %v", dropErr)
	}
	capturer.err = errors.New("terminal must not reopen")
	replayedResult, err := store.ResolveOrCapture(
		context.Background(),
		preparation,
	)
	if err != nil {
		t.Fatalf("ResolveOrCapture(strict retry): %v", err)
	}
	replayed, ok := replayedResult.Replayed()
	if !ok {
		t.Fatalf("strict retry source = %#v, want replayed", replayedResult)
	}
	if replayed.Record().Digest() != record.Digest() {
		t.Fatal("strict retry recovered another SpeechAct record")
	}
	result := selectGenesisWithStrictSource(
		t,
		fixture,
		preparation,
		replayed.Record(),
	)
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("strict retry SelectGenesis = %T, want FreshlyCommitted", result)
	}
	if capturer.calls != 1 {
		t.Fatalf("strict retry terminal capture calls = %d, want 1", capturer.calls)
	}
	assertGenesisE2ECommittedFootprint(t, fixture, fresh.Closure())
	assertGenesisStrictSourceExact(t, fixture, preparation, replayed.Record())
}

func TestGenesisServiceStrictExactReplayDoesNotMutateOrPrompt(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	preparation, store, capturer := newGenesisStrictSourceFixture(t, fixture)
	record := captureGenesisStrictSource(t, store, preparation)
	first := selectGenesisWithStrictSource(t, fixture, preparation, record)
	fresh, ok := first.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("first strict SelectGenesis = %T, want FreshlyCommitted", first)
	}
	before := observeGenesisFaultSnapshot(t, fixture)
	capturer.err = errors.New("terminal must not reopen")
	replayedSourceResult, err := store.ResolveOrCapture(
		context.Background(),
		preparation,
	)
	if err != nil {
		t.Fatalf("ResolveOrCapture(exact replay): %v", err)
	}
	replayedSource, ok := replayedSourceResult.Replayed()
	if !ok {
		t.Fatalf(
			"exact strict source result = %#v, want replayed",
			replayedSourceResult,
		)
	}
	second := selectGenesisWithStrictSource(
		t,
		fixture,
		preparation,
		replayedSource.Record(),
	)
	replayed, ok := second.(projecttypeenvselectioneffect.ReplayedExisting)
	if !ok {
		t.Fatalf("second strict SelectGenesis = %T, want ReplayedExisting", second)
	}
	if replayed.Closure().Ref() != fresh.Closure().Ref() {
		t.Fatal("strict exact replay returned another closure")
	}
	after := observeGenesisFaultSnapshot(t, fixture)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("strict exact replay mutated effect rows: before=%v after=%v", before, after)
	}
	if capturer.calls != 1 {
		t.Fatalf("strict replay terminal capture calls = %d, want 1", capturer.calls)
	}
	assertGenesisStrictSourceExact(
		t,
		fixture,
		preparation,
		replayedSource.Record(),
	)
}

func TestGenesisServiceResolveCurrentCLIIngressUsesClosedConfiguredMode(
	t *testing.T,
) {
	t.Run("explicit_h_decide_never_opens_terminal", func(t *testing.T) {
		fixture := newGenesisE2EFixture(t)
		resolution, err := fixture.service.ResolveCurrentCLIIngress(
			context.Background(),
			fixture.request,
			fixture.content,
			fixture.stage,
			authority.ObservableCarrierBinding{},
			nil,
		)
		if err != nil {
			t.Fatalf("ResolveCurrentCLIIngress(explicit): %v", err)
		}
		if !resolution.ExplicitHDecide() ||
			resolution.StrictCaptured() ||
			resolution.StrictReplayed() {
			t.Fatal("explicit CLI ingress returned another posture")
		}
		result, err := fixture.service.SelectGenesis(
			context.Background(),
			GenesisSelectionInput{
				Request:   fixture.request,
				Content:   fixture.content,
				Authority: resolution.Ingress(),
			},
		)
		if err != nil {
			t.Fatalf("SelectGenesis(explicit resolved ingress): %v", err)
		}
		if _, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
			t.Fatalf("explicit resolved ingress result = %T", result)
		}
	})

	t.Run("strict_captures_then_replays_after_commit", func(t *testing.T) {
		fixture := newGenesisE2EFixture(t)
		writeGenesisStrictConfig(t, fixture)
		capturer := &genesisStrictCapturer{
			t:         t,
			startedAt: fixture.content.ValidityWindow().From().Add(time.Minute),
		}
		reviewCarrier := genesisStrictReviewCarrier(t, fixture)
		first, err := fixture.service.ResolveCurrentCLIIngress(
			context.Background(),
			fixture.request,
			fixture.content,
			fixture.stage,
			reviewCarrier,
			capturer,
		)
		if err != nil {
			t.Fatalf("ResolveCurrentCLIIngress(strict fresh): %v", err)
		}
		if !first.StrictCaptured() ||
			first.ExplicitHDecide() ||
			first.StrictReplayed() {
			t.Fatal("fresh strict CLI ingress returned another posture")
		}
		result, err := fixture.service.SelectGenesis(
			context.Background(),
			GenesisSelectionInput{
				Request:   fixture.request,
				Content:   fixture.content,
				Authority: first.Ingress(),
			},
		)
		if err != nil {
			t.Fatalf("SelectGenesis(strict resolved ingress): %v", err)
		}
		if _, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
			t.Fatalf("strict resolved ingress result = %T", result)
		}
		capturer.err = errors.New("terminal must not reopen")
		second, err := fixture.service.ResolveCurrentCLIIngress(
			context.Background(),
			fixture.request,
			fixture.content,
			fixture.stage,
			reviewCarrier,
			capturer,
		)
		if err != nil {
			t.Fatalf("ResolveCurrentCLIIngress(strict replay): %v", err)
		}
		if !second.StrictReplayed() ||
			second.ExplicitHDecide() ||
			second.StrictCaptured() {
			t.Fatal("replayed strict CLI ingress returned another posture")
		}
		replayed, err := fixture.service.SelectGenesis(
			context.Background(),
			GenesisSelectionInput{
				Request:   fixture.request,
				Content:   fixture.content,
				Authority: second.Ingress(),
			},
		)
		if err != nil {
			t.Fatalf("SelectGenesis(strict replayed ingress): %v", err)
		}
		if _, ok := replayed.(projecttypeenvselectioneffect.ReplayedExisting); !ok {
			t.Fatalf("strict replayed ingress result = %T", replayed)
		}
		if capturer.calls != 1 {
			t.Fatalf("configured CLI ingress capture calls = %d, want 1", capturer.calls)
		}
	})
}

func newGenesisStrictSourceFixture(
	t *testing.T,
	fixture genesisE2EFixture,
) (
	projecttypeenvselectionauthority.StrictCLISpeechActPreparation,
	*projecttypeenvselectionauthority.StrictCLISpeechActSourceStore,
	*genesisStrictCapturer,
) {
	t.Helper()
	writeGenesisStrictConfig(t, fixture)
	_, configCarrier, err := loadCurrentProjectConfigAuthorityCarrier(
		fixture.service.projectRoot,
	)
	if err != nil {
		t.Fatalf("load strict config carrier: %v", err)
	}
	projectBinding := genesisCurrentProjectAuthorityBinding(t, fixture)
	reviewCarrier := genesisStrictReviewCarrier(t, fixture)
	preparation, err :=
		projecttypeenvselectionauthority.PrepareStrictCLISpeechAct(
			projecttypeenvselectionauthority.StrictCLISpeechActPreparationInput{
				Request:        fixture.request,
				Content:        fixture.content,
				Stage:          fixture.stage,
				ProjectBinding: projectBinding,
				ConfigCarrier:  configCarrier,
				ReviewCarrier:  reviewCarrier,
			},
		)
	if err != nil {
		t.Fatalf("PrepareStrictCLISpeechAct: %v", err)
	}
	capturer := &genesisStrictCapturer{
		t:         t,
		startedAt: fixture.content.ValidityWindow().From().Add(time.Minute),
	}
	store, err :=
		projecttypeenvselectionauthority.OpenStrictCLISpeechActSourceStore(
			fixture.database,
			capturer,
		)
	if err != nil {
		t.Fatalf("OpenStrictCLISpeechActSourceStore: %v", err)
	}
	return preparation, store, capturer
}

func genesisStrictReviewCarrier(
	t *testing.T,
	fixture genesisE2EFixture,
) authority.ObservableCarrierBinding {
	t.Helper()
	reviewRef, err := authority.NewCarrierRef(
		"carrier:.haft/project-typeenv-genesis-review.json",
	)
	if err != nil {
		t.Fatalf("NewCarrierRef(strict review): %v", err)
	}
	reviewDigest, err := authority.NewDigest(fixture.content.Digest().String())
	if err != nil {
		t.Fatalf("NewDigest(strict review): %v", err)
	}
	reviewCarrier, err := authority.NewObservableCarrierBinding(
		reviewRef,
		reviewDigest,
	)
	if err != nil {
		t.Fatalf("NewObservableCarrierBinding(strict review): %v", err)
	}
	return reviewCarrier
}

func writeGenesisStrictConfig(t *testing.T, fixture genesisE2EFixture) {
	t.Helper()
	configDirectory := filepath.Join(fixture.service.projectRoot.String(), ".haft")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatalf("create strict config directory: %v", err)
	}
	config := []byte(
		"schema_version: 1\n" +
			"authority:\n" +
			"  decision_binding_mode: explicit_h_decide\n" +
			"  project_typeenv_head_selection_mode: strict_cli_speech_act\n",
	)
	if err := os.WriteFile(
		filepath.Join(configDirectory, "config.yaml"),
		config,
		0o600,
	); err != nil {
		t.Fatalf("write strict config: %v", err)
	}
}

func genesisCurrentProjectAuthorityBinding(
	t *testing.T,
	fixture genesisE2EFixture,
) projecttypeenvselectionauthority.ProjectAuthorityContextBinding {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("begin strict preparation observation: %v", err)
	}
	frameResult, err := loadCurrentGenesisFrameTx(
		ctx,
		transaction,
		currentGenesisFrameDependencies{
			stages:           fixture.service.stages,
			heads:            fixture.service.heads,
			installedRuntime: fixture.service.installedRuntime,
			observedAt:       fixture.service.clock.Now(),
		},
		fixture.request,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("load strict preparation frame: %v", err)
	}
	ready, ok := frameResult.(currentGenesisFrameReady)
	if !ok {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("strict preparation frame = %T, want ready", frameResult)
	}
	binding, err := currentProjectAuthorityContextBinding(
		ready.frame,
		GenesisSelectionInput{
			Request:   fixture.request,
			Content:   fixture.content,
			Authority: NewDedicatedCLIInvocation(),
		},
	)
	finish := transaction.Rollback(context.Background())
	if err != nil {
		t.Fatalf("seal strict project binding: %v", err)
	}
	if !finish.Succeeded() {
		t.Fatalf("rollback strict preparation observation: %v", finish.Err())
	}
	return binding
}

func captureGenesisStrictSource(
	t *testing.T,
	store *projecttypeenvselectionauthority.StrictCLISpeechActSourceStore,
	preparation projecttypeenvselectionauthority.StrictCLISpeechActPreparation,
) projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord {
	t.Helper()
	result, err := store.ResolveOrCapture(context.Background(), preparation)
	if err != nil {
		t.Fatalf("ResolveOrCapture(fresh strict source): %v", err)
	}
	captured, ok := result.Captured()
	if !ok {
		t.Fatalf("fresh strict source = %#v, want captured", result)
	}
	return captured.Record()
}

func selectGenesisWithStrictSource(
	t *testing.T,
	fixture genesisE2EFixture,
	preparation projecttypeenvselectionauthority.StrictCLISpeechActPreparation,
	record projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord,
) projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult {
	t.Helper()
	result, err := selectGenesisWithStrictSourceResult(
		fixture,
		preparation,
		record,
	)
	if err != nil {
		t.Fatalf("SelectGenesis(strict source): %v", err)
	}
	return result
}

func selectGenesisWithStrictSourceResult(
	fixture genesisE2EFixture,
	preparation projecttypeenvselectionauthority.StrictCLISpeechActPreparation,
	record projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord,
) (projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult, error) {
	ingress, err := NewVerifiedSpeechActIngress(
		preparation.ResolverPolicy(),
		record,
	)
	if err != nil {
		return nil, err
	}
	return fixture.service.SelectGenesis(
		context.Background(),
		GenesisSelectionInput{
			Request:   fixture.request,
			Content:   fixture.content,
			Authority: ingress,
		},
	)
}

func assertGenesisStrictSourceExact(
	t *testing.T,
	fixture genesisE2EFixture,
	preparation projecttypeenvselectionauthority.StrictCLISpeechActPreparation,
	record projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord,
) {
	t.Helper()
	assertGenesisExactRowCount(
		t,
		fixture,
		"speech_acts",
		"speech_act_ref",
		preparation.SpeechActRef().String(),
	)
	assertGenesisExactRowCount(
		t,
		fixture,
		"project_typeenv_head_selection_speech_act_records",
		"speech_act_record_ref",
		record.Ref().String(),
	)
	assertGenesisExactRowCount(
		t,
		fixture,
		"project_typeenv_head_selection_permissions_v3",
		"permission_ref",
		record.PermissionRecord().Ref().String(),
	)
}

func assertGenesisExactRowCount(
	t *testing.T,
	fixture genesisE2EFixture,
	table string,
	refColumn string,
	ref string,
) {
	t.Helper()
	var got int
	if err := fixture.database.QueryRow(
		"SELECT COUNT(*) FROM "+table+" WHERE "+refColumn+" = ?",
		ref,
	).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != 1 {
		t.Fatalf("%s exact row count = %d, want 1", table, got)
	}
}
