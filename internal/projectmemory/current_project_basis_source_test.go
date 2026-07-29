package projectmemory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type recordingCurrentProjectSnapshotLoader struct {
	current   typedmemorystore.CurrentProjectSnapshot
	err       error
	calls     int
	projectID projectledger.ProjectID
	context   context.Context
	afterCall func()
}

type sqliteBasisContextKey string

func (loader *recordingCurrentProjectSnapshotLoader) LoadCurrentProjectSnapshot(
	ctx context.Context,
	projectID projectledger.ProjectID,
) (typedmemorystore.CurrentProjectSnapshot, error) {
	loader.calls++
	loader.projectID = projectID
	loader.context = ctx
	if loader.afterCall != nil {
		loader.afterCall()
	}
	return loader.current, loader.err
}

type sqliteBasisSnapshot struct {
	revision typedmemory.GraphRevision
	typeEnv  typedmemory.TypeEnvRef
}

func (snapshot *sqliteBasisSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot *sqliteBasisSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (*sqliteBasisSnapshot) ResolveEntity(
	typedmemory.EntityID,
	typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return nil
}

func (*sqliteBasisSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (*sqliteBasisSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (*sqliteBasisSnapshot) AssertionState(
	typedmemory.AssertionID,
) typedmemory.AssertionState {
	return nil
}

func (*sqliteBasisSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (*sqliteBasisSnapshot) ResolveReconciliationBasis(
	typedmemory.ReconciliationBasisRef,
	typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	return nil
}

func TestCurrentProjectBasisSourceResolvesCurrentFromOneLoadedSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 7)
	loader := &recordingCurrentProjectSnapshotLoader{current: fixture.current}
	source := newCurrentProjectBasisSource(t, loader)
	key := sqliteBasisContextKey("sqlite-basis-source")
	ctx := context.WithValue(context.Background(), key, "preserved")
	selector := typedmemorywire.ProjectCurrentSelector{}

	resolution, err := source.ResolveProjectBasis(ctx, fixture.projectID, selector)
	if err != nil {
		t.Fatalf("ResolveProjectBasis() error = %v", err)
	}
	resolved, ok := resolution.(*typedmemoryvalidation.ResolvedProjectBasis)
	if !ok {
		t.Fatalf("ResolveProjectBasis() = %T, want *ResolvedProjectBasis", resolution)
	}
	if resolved.Environment().Ref() != fixture.environment.Ref() {
		t.Fatalf("resolved TypeEnv = %q, want %q", resolved.Environment().Ref().String(), fixture.environment.Ref().String())
	}
	if resolved.Snapshot() != fixture.snapshot {
		t.Fatal("resolved basis did not preserve the immutable loaded snapshot")
	}
	if loader.calls != 1 {
		t.Fatalf("loader calls = %d, want 1", loader.calls)
	}
	if loader.projectID != fixture.projectID {
		t.Fatalf("loader project = %q, want %q", loader.projectID.String(), fixture.projectID.String())
	}
	if loader.context.Value(key) != "preserved" {
		t.Fatal("loader did not receive the caller context")
	}
}

func TestCurrentProjectBasisSourceResolvesExactMatch(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 11)
	loader := &recordingCurrentProjectSnapshotLoader{current: fixture.current}
	source := newCurrentProjectBasisSource(t, loader)
	digest := fixture.environment.Ref().Digest().String()
	basis := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":%q,"graph_revision":11}`,
		digest,
	)
	selector := sqliteBasisSelector(t, basis)

	resolution, err := source.ResolveProjectBasis(
		context.Background(),
		fixture.projectID,
		selector,
	)
	if err != nil {
		t.Fatalf("ResolveProjectBasis() error = %v", err)
	}
	resolved, ok := resolution.(*typedmemoryvalidation.ResolvedProjectBasis)
	if !ok {
		t.Fatalf("ResolveProjectBasis() = %T, want *ResolvedProjectBasis", resolution)
	}
	if resolved.Snapshot().GraphRevision() != typedmemory.NewGraphRevision(11) {
		t.Fatalf("resolved revision = %d, want 11", resolved.Snapshot().GraphRevision().Value())
	}
}

func TestCurrentProjectBasisSourceRejectsSnapshotFromAnotherProject(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 11)
	foreignProject, err := projectledger.ParseProjectID("qnt_deadbeef")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	foreign, err := typedmemorystore.NewCurrentProjectSnapshot(
		foreignProject,
		fixture.environment,
		fixture.current.Codecs(),
		fixture.snapshot,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectSnapshot() error = %v", err)
	}
	loader := &recordingCurrentProjectSnapshotLoader{current: foreign}
	source := newCurrentProjectBasisSource(t, loader)

	resolution, err := source.ResolveProjectBasis(
		context.Background(),
		fixture.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if !errors.Is(err, ErrProjectBasisUncorrelated) {
		t.Fatalf("ResolveProjectBasis() error = %v, want ErrProjectBasisUncorrelated", err)
	}
	if resolution != nil {
		t.Fatalf("ResolveProjectBasis() = %T, want nil", resolution)
	}
}

func TestNewCurrentProjectSnapshotRejectsMissingProject(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 11)
	var missingProject projectledger.ProjectID
	_, err := typedmemorystore.NewCurrentProjectSnapshot(
		missingProject,
		fixture.environment,
		fixture.current.Codecs(),
		fixture.snapshot,
	)
	if err == nil || !strings.Contains(err.Error(), "exact project identity") {
		t.Fatalf("NewCurrentProjectSnapshot() error = %v; want exact project identity rejection", err)
	}
}

func TestCurrentProjectBasisSourceReturnsExactMismatchWithoutFallback(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 13)
	loader := &recordingCurrentProjectSnapshotLoader{current: fixture.current}
	source := newCurrentProjectBasisSource(t, loader)
	basis := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":%q,"graph_revision":12}`,
		fixture.environment.Ref().Digest().String(),
	)
	selector := sqliteBasisSelector(t, basis)

	resolution, err := source.ResolveProjectBasis(
		context.Background(),
		fixture.projectID,
		selector,
	)
	if err != nil {
		t.Fatalf("ResolveProjectBasis() error = %v", err)
	}
	mismatch, ok := resolution.(*typedmemoryvalidation.ExactProjectBasisMismatch)
	if !ok {
		t.Fatalf("ResolveProjectBasis() = %T, want *ExactProjectBasisMismatch", resolution)
	}
	if mismatch.ObservedTypeEnvRef() != fixture.environment.Ref() {
		t.Fatalf("observed TypeEnv = %q, want %q", mismatch.ObservedTypeEnvRef().String(), fixture.environment.Ref().String())
	}
	if mismatch.ObservedGraphRevision() != typedmemory.NewGraphRevision(13) {
		t.Fatalf("observed revision = %d, want 13", mismatch.ObservedGraphRevision().Value())
	}
}

func TestCurrentProjectBasisSourceMapsOnlyMissingHeadToUnavailable(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 0)
	loader := &recordingCurrentProjectSnapshotLoader{
		err: typedmemorystore.ErrProjectNotInitialized,
	}
	source := newCurrentProjectBasisSource(t, loader)

	resolution, err := source.ResolveProjectBasis(
		context.Background(),
		fixture.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if err != nil {
		t.Fatalf("ResolveProjectBasis() error = %v", err)
	}
	if _, ok := resolution.(*typedmemoryvalidation.ProjectBasisUnavailable); !ok {
		t.Fatalf("ResolveProjectBasis() = %T, want *ProjectBasisUnavailable", resolution)
	}
}

func TestCurrentProjectBasisSourcePropagatesStoredStateCorruption(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 0)
	corruption := fmt.Errorf("fixture partial v46 state: %w", typedmemorystore.ErrStoredAdmissionIntegrity)
	loader := &recordingCurrentProjectSnapshotLoader{err: corruption}
	source := newCurrentProjectBasisSource(t, loader)

	resolution, err := source.ResolveProjectBasis(
		context.Background(),
		fixture.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if !errors.Is(err, typedmemorystore.ErrStoredAdmissionIntegrity) {
		t.Fatalf("ResolveProjectBasis() error = %v, want stored integrity error", err)
	}
	if resolution != nil {
		t.Fatalf("ResolveProjectBasis() = %T, want nil on corruption", resolution)
	}
}

func TestCurrentProjectBasisSourceHonorsCancellationBeforeAndDuringLoad(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteBasisFixture(t, 2)
	preCanceled, cancelBefore := context.WithCancel(context.Background())
	cancelBefore()
	unusedLoader := &recordingCurrentProjectSnapshotLoader{current: fixture.current}
	source := newCurrentProjectBasisSource(t, unusedLoader)

	resolution, err := source.ResolveProjectBasis(
		preCanceled,
		fixture.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled ResolveProjectBasis() error = %v, want context.Canceled", err)
	}
	if resolution != nil {
		t.Fatalf("pre-canceled ResolveProjectBasis() = %T, want nil", resolution)
	}
	if unusedLoader.calls != 0 {
		t.Fatalf("pre-canceled loader calls = %d, want 0", unusedLoader.calls)
	}

	duringLoad, cancelDuring := context.WithCancel(context.Background())
	loader := &recordingCurrentProjectSnapshotLoader{
		current:   fixture.current,
		afterCall: cancelDuring,
	}
	source = newCurrentProjectBasisSource(t, loader)
	resolution, err = source.ResolveProjectBasis(
		duringLoad,
		fixture.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("mid-load ResolveProjectBasis() error = %v, want context.Canceled", err)
	}
	if resolution != nil {
		t.Fatalf("mid-load ResolveProjectBasis() = %T, want nil", resolution)
	}
}

func TestNewCurrentProjectBasisSourceRejectsNilAndTypedNilLoader(t *testing.T) {
	t.Parallel()

	var typedNil *recordingCurrentProjectSnapshotLoader
	tests := []struct {
		name   string
		loader typedmemorystore.CurrentProjectSnapshotLoader
	}{
		{name: "nil interface", loader: nil},
		{name: "typed nil", loader: typedNil},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source, err := NewCurrentProjectBasisSource(test.loader)
			if !errors.Is(err, ErrCurrentProjectSnapshotLoaderMissing) {
				t.Fatalf("NewCurrentProjectBasisSource() error = %v, want missing loader", err)
			}
			if source != nil {
				t.Fatalf("NewCurrentProjectBasisSource() = %#v, want nil", source)
			}
		})
	}
}

type sqliteBasisFixture struct {
	projectID   projectledger.ProjectID
	environment typedmemory.TypeEnv
	snapshot    *sqliteBasisSnapshot
	current     typedmemorystore.CurrentProjectSnapshot
}

func newSQLiteBasisFixture(
	t *testing.T,
	revision uint64,
) sqliteBasisFixture {
	t.Helper()
	projectID := sqliteBasisProjectID(t)
	artifact := loaderTestBundledArtifact(t)
	environment, codecs, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecs() error = %v", err)
	}
	snapshot := &sqliteBasisSnapshot{
		revision: typedmemory.NewGraphRevision(revision),
		typeEnv:  environment.Ref(),
	}
	current, err := typedmemorystore.NewCurrentProjectSnapshot(
		projectID,
		environment,
		codecs,
		snapshot,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectSnapshot() error = %v", err)
	}
	return sqliteBasisFixture{
		projectID:   projectID,
		environment: environment,
		snapshot:    snapshot,
		current:     current,
	}
}

func sqliteBasisProjectID(t *testing.T) projectledger.ProjectID {
	t.Helper()
	projectID, err := projectledger.ParseProjectID("qnt_5a1e5ba5")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	return projectID
}

func newCurrentProjectBasisSource(
	t *testing.T,
	loader typedmemorystore.CurrentProjectSnapshotLoader,
) *CurrentProjectBasisSource {
	t.Helper()
	source, err := NewCurrentProjectBasisSource(loader)
	if err != nil {
		t.Fatalf("NewCurrentProjectBasisSource() error = %v", err)
	}
	return source
}

func sqliteBasisSelector(
	t *testing.T,
	basis string,
) typedmemorywire.BasisSelector {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": %s,
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:sqlite-basis-fixture",
      "local_ref": "local:sqlite-basis-fixture",
      "context": "context:sqlite-basis-fixture",
      "label": "SQLite basis fixture",
      "provenance": "provenance:sqlite-basis-fixture"
    }]
  }
}`, typedmemorywire.ContractVersion, basis)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request.Basis()
}
