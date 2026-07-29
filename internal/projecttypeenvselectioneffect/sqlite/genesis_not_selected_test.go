package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestGenesisServiceMapsCurrentFrameRejectionsWithoutWrites(
	t *testing.T,
) {
	tests := []struct {
		name   string
		mutate func(*genesisE2EFixture)
		reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
	}{
		{
			name: "invalid-installed-runtime-is-target-integrity",
			mutate: func(fixture *genesisE2EFixture) {
				installed := fixture.service.installedRuntime
				installed.MechanismCatalogs = append(
					installed.MechanismCatalogs,
					runtimemechanism.RuntimeMechanismArtifactV1{},
				)
				fixture.service.installedRuntime = installed
			},
			reason: projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(),
		},
		{
			name: "unavailable-installed-runtime-is-stage-drift",
			mutate: func(fixture *genesisE2EFixture) {
				fixture.service.installedRuntime =
					projecttypeenvruntime.InstalledRuntimeRegistryInput{}
			},
			reason: projecttypeenvselectioneffect.NotSelectedStageDrift(),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newGenesisE2EFixture(t)
			testCase.mutate(&fixture)
			before := genesisE2EEffectCounts(t, fixture.database)
			result, err := fixture.service.SelectGenesis(
				context.Background(),
				genesisSelectionInput(fixture),
			)
			if err != nil {
				t.Fatalf("SelectGenesis(): %v", err)
			}
			assertGenesisNotSelected(t, result, testCase.reason)
			after := genesisE2EEffectCounts(t, fixture.database)
			if after != before {
				t.Fatalf(
					"current-frame rejection wrote effect rows: before=%+v after=%+v",
					before,
					after,
				)
			}
		})
	}
}

func TestGenesisServiceMapsCancellationBeforeTransactionWithoutWrites(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	before := genesisE2EEffectCounts(t, fixture.database)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(canceled): %v", err)
	}
	assertGenesisNotSelected(
		t,
		result,
		projecttypeenvselectioneffect.NotSelectedCancellation(),
	)
	after := genesisE2EEffectCounts(t, fixture.database)
	if after != before {
		t.Fatalf(
			"canceled selection wrote effect rows: before=%+v after=%+v",
			before,
			after,
		)
	}
}

func TestGenesisServiceMapsBusyBeginAsStorageFailureWithoutWrites(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	before := genesisE2EEffectCounts(t, fixture.database)
	if _, err := fixture.database.Exec("PRAGMA busy_timeout = 0"); err != nil {
		t.Fatalf("disable service busy timeout: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir(): %v", err)
	}
	databasePath := filepath.Join(
		home,
		".haft",
		"projects",
		fixture.project.String(),
		"haft.db",
	)
	locker, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open competing SQLite handle: %v", err)
	}
	t.Cleanup(func() { _ = locker.Close() })
	lock, err := sqlitetransaction.BeginImmediate(
		context.Background(),
		locker,
	)
	if err != nil {
		t.Fatalf("hold competing immediate transaction: %v", err)
	}
	defer func() {
		_ = lock.Rollback(context.Background())
	}()
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(busy): %v", err)
	}
	assertGenesisNotSelected(
		t,
		result,
		projecttypeenvselectioneffect.NotSelectedStorageFailure(),
	)
	if finish := lock.Rollback(context.Background()); !finish.Succeeded() {
		t.Fatalf("release competing immediate transaction: %v", finish.Err())
	}
	after := genesisE2EEffectCounts(t, fixture.database)
	if after != before {
		t.Fatalf(
			"busy selection wrote effect rows: before=%+v after=%+v",
			before,
			after,
		)
	}
}

func TestPreCommitNotSelectedReasonUsesTypedOperationalFailuresOnly(
	t *testing.T,
) {
	tests := []struct {
		name   string
		err    error
		reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
		ok     bool
	}{
		{
			name:   "cancellation",
			err:    context.Canceled,
			reason: projecttypeenvselectioneffect.NotSelectedCancellation(),
			ok:     true,
		},
		{
			name:   "bad-connection",
			err:    driver.ErrBadConn,
			reason: projecttypeenvselectioneffect.NotSelectedStorageFailure(),
			ok:     true,
		},
		{
			name: "untyped-error-remains-error",
			err:  errors.New("programmer or replay failure"),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			reason, ok := preCommitNotSelectedReason(testCase.err)
			if ok != testCase.ok {
				t.Fatalf(
					"preCommitNotSelectedReason() ok = %t, want %t",
					ok,
					testCase.ok,
				)
			}
			if ok && reason != testCase.reason {
				t.Fatalf(
					"preCommitNotSelectedReason() = %q, want %q",
					reason.String(),
					testCase.reason.String(),
				)
			}
		})
	}
}

func TestGenesisServiceMapsCurrentAuthorityRejectionWithoutWrites(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	configPath := filepath.Join(
		fixture.service.projectRoot.String(),
		".haft",
		"config.yaml",
	)
	config := []byte(
		"schema_version: 1\n" +
			"authority:\n" +
			"  decision_binding_mode: explicit_h_decide\n" +
			"  project_typeenv_head_selection_mode: strict_cli_speech_act\n",
	)
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write strict current authority config: %v", err)
	}
	before := genesisE2EEffectCounts(t, fixture.database)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	assertGenesisNotSelected(
		t,
		result,
		projecttypeenvselectioneffect.NotSelectedCurrentAuthorityRejection(),
	)
	after := genesisE2EEffectCounts(t, fixture.database)
	if after != before {
		t.Fatalf(
			"current-authority rejection wrote effect rows: before=%+v after=%+v",
			before,
			after,
		)
	}
}

func TestGenesisServiceMapsPriorHeadBeforeConcurrentGraphDrift(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	first, err := fixture.service.SelectGenesis(
		context.Background(),
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(seed): %v", err)
	}
	if _, ok := first.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf("SelectGenesis(seed) = %T, want FreshlyCommitted", first)
	}
	request, content := anotherGenesisRequest(t, fixture)
	before := genesisE2EEffectCounts(t, fixture.database)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		GenesisSelectionInput{
			Request:   request,
			Content:   content,
			Authority: NewDedicatedCLIInvocation(),
		},
	)
	if err != nil {
		t.Fatalf("SelectGenesis(second key): %v", err)
	}
	assertGenesisNotSelected(
		t,
		result,
		projecttypeenvselectioneffect.NotSelectedPriorHeadExists(),
	)
	after := genesisE2EEffectCounts(t, fixture.database)
	if after != before {
		t.Fatalf(
			"prior-head rejection wrote effect rows: before=%+v after=%+v",
			before,
			after,
		)
	}
}

func TestGenesisServiceDoesNotMapInvalidIngressToNotSelected(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	before := genesisE2EEffectCounts(t, fixture.database)
	result, err := fixture.service.SelectGenesis(
		context.Background(),
		GenesisSelectionInput{
			Request: fixture.request,
			Content: fixture.content,
		},
	)
	if err == nil {
		t.Fatalf("SelectGenesis(invalid ingress) = %T, want error", result)
	}
	if result != nil {
		t.Fatalf("SelectGenesis(invalid ingress) result = %T, want nil", result)
	}
	after := genesisE2EEffectCounts(t, fixture.database)
	if after != before {
		t.Fatalf(
			"invalid ingress wrote effect rows: before=%+v after=%+v",
			before,
			after,
		)
	}
}

func TestSelectionReadyLoadRejectionUsesOnlyTypedStoreFailures(
	t *testing.T,
) {
	tests := []struct {
		name   string
		err    error
		reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
		ok     bool
	}{
		{
			name:   "missing",
			err:    projecttypeenvstage.ErrStageNotFound,
			reason: projecttypeenvselectioneffect.NotSelectedTargetSnapshotMissing(),
			ok:     true,
		},
		{
			name:   "conflict",
			err:    projecttypeenvstore.ErrArtifactConflict,
			reason: projecttypeenvselectioneffect.NotSelectedTargetSnapshotConflict(),
			ok:     true,
		},
		{
			name:   "integrity",
			err:    projecttypeenvstage.ErrStageIntegrity,
			reason: projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(),
			ok:     true,
		},
		{
			name: "transport-is-not-mapped",
			err:  errors.New("transport failure"),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			reason, ok := selectionReadyLoadRejection(testCase.err)
			if ok != testCase.ok {
				t.Fatalf("selectionReadyLoadRejection() ok = %t, want %t", ok, testCase.ok)
			}
			if ok && reason != testCase.reason {
				t.Fatalf(
					"selectionReadyLoadRejection() = %q, want %q",
					reason.String(),
					testCase.reason.String(),
				)
			}
		})
	}
}

func assertGenesisNotSelected(
	t *testing.T,
	result projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionResult,
	want projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) {
	t.Helper()
	notSelected, ok := result.(projecttypeenvselectioneffect.NotSelected)
	if !ok {
		t.Fatalf("SelectGenesis() = %T, want NotSelected", result)
	}
	if notSelected.Reason() != want {
		t.Fatalf(
			"NotSelected reason = %q, want %q",
			notSelected.Reason().String(),
			want.String(),
		)
	}
}

func anotherGenesisRequest(
	t *testing.T,
	fixture genesisE2EFixture,
) (
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) {
	t.Helper()
	key, err :=
		projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
			"genesis-not-selected-prior-head",
		)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	request, err :=
		projecttypeenvselection.SealGenesisProjectTypeEnvHeadSelectionRequest(
			projecttypeenvselection.GenesisProjectTypeEnvHeadSelectionRequestInput{
				Project:               fixture.project,
				Stage:                 fixture.stage,
				ExpectedGraphRevision: fixture.request.ExpectedGraphRevision(),
				IdempotencyKey:        key,
			},
		)
	if err != nil {
		t.Fatalf("SealGenesisProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-head-selection:prior-head",
	)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef(): %v", err)
	}
	content, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
				DescriptionRef:   description,
				Request:          request,
				Stage:            fixture.stage,
				JudgementContext: fixture.content.JudgementContext(),
				ValidityWindow:   fixture.content.ValidityWindow(),
			},
		)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent(): %v", err)
	}
	return request, content
}
