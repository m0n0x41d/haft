package typedmemorystore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type recordingSelectedProjectTypeEnvRuntimeResolver struct {
	environment     typedmemory.TypeEnv
	codecs          typedmemory.CodecRegistry
	err             error
	calls           int
	context         context.Context
	transaction     *sqlitetransaction.Transaction
	request         SelectedProjectTypeEnvRuntimeRequest
	memberOf        SelectedMemberOfRuntime
	mutateResult    func(SelectedProjectTypeEnvRuntime) SelectedProjectTypeEnvRuntime
	cancelAfterRead func()
}

func (resolver *recordingSelectedProjectTypeEnvRuntimeResolver) ResolveSelectedProjectTypeEnvRuntime(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request SelectedProjectTypeEnvRuntimeRequest,
) (SelectedProjectTypeEnvRuntime, error) {
	resolver.calls++
	resolver.context = ctx
	resolver.transaction = transaction
	resolver.request = request
	if resolver.cancelAfterRead != nil {
		resolver.cancelAfterRead()
	}
	if resolver.err != nil {
		return SelectedProjectTypeEnvRuntime{}, resolver.err
	}
	memberOf := resolver.memberOf
	if memberOf == nil {
		memberOf = NewMemberOfNotRequired()
	}
	builder := NewSelectedProjectTypeEnvRuntimeBuilder(request)
	runtime, err := builder.
		SetEnvironment(resolver.environment).
		SetCodecs(resolver.codecs).
		SetMemberOfRuntime(memberOf).
		Build()
	if err != nil {
		return SelectedProjectTypeEnvRuntime{}, err
	}
	if resolver.mutateResult != nil {
		runtime = resolver.mutateResult(runtime)
	}
	return runtime, nil
}

func TestResolveTypeEnvRuntimeTxPreservesLegacyGenericSnapshotBranch(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{}
	fixture.adapter.selectedProjectRuntime = resolver
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	resolved, err := fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(0),
		fixture.environment.Ref(),
	)
	if err != nil {
		t.Fatalf("resolveTypeEnvRuntimeTx: %v", err)
	}
	if resolved.environment.Ref() != fixture.environment.Ref() {
		t.Fatalf(
			"resolved generic TypeEnv = %q, want %q",
			resolved.environment.Ref().String(),
			fixture.environment.Ref().String(),
		)
	}
	if resolver.calls != 0 {
		t.Fatalf(
			"project runtime resolver calls = %d, want 0 for generic snapshot",
			resolver.calls,
		)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("generic resolution finished caller transaction: %v", err)
	}
}

func TestProjectExecutableAdapterNeedsNoProcessGlobalMemberOfEvaluator(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{}
	adapter, err := NewProjectExecutableGenericSQLiteAdapterBuilder(
		fixture.database,
	).
		SetTypeEnvLoader(loader).
		SetClock(fixture.clock).
		SetReferenceEngine(unexpectedReferenceEngine{}).
		SetObservableInputs(unexpectedObservableProvider{}).
		SetSelectedProjectRuntime(resolver).
		Build()
	if err != nil {
		t.Fatalf("project-executable adapter builder: %v", err)
	}
	if memberOfEvaluationEngineIsPresent(adapter.memberOfEngine) {
		t.Fatal("project-executable adapter retained a process-global MemberOf evaluator")
	}
	if adapter.selectedProjectRuntime != resolver {
		t.Fatal("project-executable adapter lost its selected-C runtime resolver")
	}

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	resolved, err := adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(0),
		fixture.environment.Ref(),
	)
	if err != nil {
		t.Fatalf("resolve generic snapshot for read: %v", err)
	}
	if memberOfEvaluationEngineIsPresent(resolved.memberOf) {
		t.Fatal("generic snapshot unexpectedly acquired a process-global MemberOf evaluator")
	}
	if resolver.calls != 0 {
		t.Fatalf(
			"generic-snapshot resolution called selected-C resolver %d times",
			resolver.calls,
		)
	}
}

func TestResolveTypeEnvRuntimeTxUsesProjectResolverInsideSameTransaction(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	selected := input.Request.Target().VerifiedComposite()
	environment := projectRuntimeTestEnvironment(
		t,
		selected,
		fixture.environment,
	)
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{
		environment: environment,
		codecs:      fixture.registry,
	}
	fixture.adapter.selectedProjectRuntime = resolver
	ctx := context.WithValue(
		context.Background(),
		projectRuntimeContextKey("same-transaction"),
		"preserved",
	)
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	revision := typedmemory.NewGraphRevision(1)

	resolved, err := fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		revision,
		selected,
	)
	if err != nil {
		t.Fatalf("resolveTypeEnvRuntimeTx: %v", err)
	}
	if resolved.environment.Ref() != selected {
		t.Fatalf(
			"resolved project TypeEnv = %q, want %q",
			resolved.environment.Ref().String(),
			selected.String(),
		)
	}
	if resolver.calls != 1 {
		t.Fatalf("project runtime resolver calls = %d, want 1", resolver.calls)
	}
	if resolver.transaction != transaction {
		t.Fatal("project runtime resolver received another transaction")
	}
	if resolver.request.Project() != fixture.project ||
		resolver.request.GraphRevision() != revision ||
		resolver.request.SelectedTypeEnv() != selected {
		t.Fatalf(
			"project runtime request = %#v, want exact project/revision/C",
			resolver.request,
		)
	}
	if resolver.context.Value(projectRuntimeContextKey("same-transaction")) != "preserved" {
		t.Fatal("project runtime resolver did not receive the caller context")
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("project resolution finished caller transaction: %v", err)
	}
}

func TestResolveTypeEnvRuntimeTxUsesExactSelectedMemberOfRuntimeWithoutStaticFallback(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	selected := input.Request.Target().VerifiedComposite()
	environment := projectRuntimeTestEnvironment(
		t,
		selected,
		fixture.environment,
	)
	runtimeBasisDigest, registryCoordinateDigest := selectedRuntimeTestCoordinates(
		t,
		selected.Digest(),
		fixture.environment.Ref().Digest(),
	)
	exact, err := NewExactMemberOfRuntime(
		ExactMemberOfRuntimeInput{
			Engine:                   selectedProjectMemberOfEngine{},
			RuntimeBasisDigest:       runtimeBasisDigest,
			RegistryCoordinateDigest: registryCoordinateDigest,
		},
	)
	if err != nil {
		t.Fatalf("NewExactMemberOfRuntime: %v", err)
	}
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{
		environment: environment,
		codecs:      fixture.registry,
		memberOf:    exact,
	}
	fixture.adapter.selectedProjectRuntime = resolver
	fixture.adapter.memberOfEngine = unexpectedMemberOfEngine{}
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	resolved, err := fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(1),
		selected,
	)
	if err != nil {
		t.Fatalf("resolveTypeEnvRuntimeTx: %v", err)
	}
	if _, ok := resolved.memberOf.(selectedProjectMemberOfEngine); !ok {
		t.Fatalf(
			"resolved MemberOf engine = %T, want selected project engine",
			resolved.memberOf,
		)
	}
}

func TestSelectedProjectTypeEnvRuntimeCarriesClosedMemberOfPosture(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	request := SelectedProjectTypeEnvRuntimeRequest{
		project:       fixture.project,
		graphRevision: typedmemory.NewGraphRevision(1),
		selected:      fixture.environment.Ref(),
	}
	runtimeBasisDigest, registryCoordinateDigest := selectedRuntimeTestCoordinates(
		t,
		fixture.environment.Ref().Digest(),
		fixture.snapshot.Ref().Digest(),
	)
	exact, err := NewExactMemberOfRuntime(
		ExactMemberOfRuntimeInput{
			Engine:                   selectedProjectMemberOfEngine{},
			RuntimeBasisDigest:       runtimeBasisDigest,
			RegistryCoordinateDigest: registryCoordinateDigest,
		},
	)
	if err != nil {
		t.Fatalf("NewExactMemberOfRuntime: %v", err)
	}
	builder := NewSelectedProjectTypeEnvRuntimeBuilder(request)
	runtime, err := builder.
		SetEnvironment(fixture.environment).
		SetCodecs(fixture.registry).
		SetMemberOfRuntime(exact).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got, ok := runtime.MemberOfRuntime().(ExactMemberOfRuntime)
	if !ok {
		t.Fatalf(
			"MemberOfRuntime = %T, want ExactMemberOfRuntime",
			runtime.MemberOfRuntime(),
		)
	}
	if got.RuntimeBasisDigest() != exact.RuntimeBasisDigest() ||
		got.RegistryCoordinateDigest() != exact.RegistryCoordinateDigest() {
		t.Fatal("exact MemberOf runtime lost its X/registry correlation")
	}
}

func TestExactMemberOfRuntimeRejectsIncompleteCorrelation(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	validRuntimeBasis, validRegistryCoordinate := selectedRuntimeTestCoordinates(
		t,
		fixture.environment.Ref().Digest(),
		fixture.snapshot.Ref().Digest(),
	)
	tests := []struct {
		name  string
		input ExactMemberOfRuntimeInput
	}{
		{
			name: "missing evaluator",
			input: ExactMemberOfRuntimeInput{
				RuntimeBasisDigest:       validRuntimeBasis,
				RegistryCoordinateDigest: validRegistryCoordinate,
			},
		},
		{
			name: "missing runtime basis",
			input: ExactMemberOfRuntimeInput{
				Engine:                   selectedProjectMemberOfEngine{},
				RegistryCoordinateDigest: validRegistryCoordinate,
			},
		},
		{
			name: "missing registry coordinate",
			input: ExactMemberOfRuntimeInput{
				Engine:             selectedProjectMemberOfEngine{},
				RuntimeBasisDigest: validRuntimeBasis,
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewExactMemberOfRuntime(test.input); err == nil {
				t.Fatal("NewExactMemberOfRuntime accepted incomplete correlation")
			}
		})
	}
}

func TestSelectedMemberOfRuntimeCannotBecomeAStoredCarrier(t *testing.T) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	runtimeBasisDigest, registryCoordinateDigest := selectedRuntimeTestCoordinates(
		t,
		fixture.environment.Ref().Digest(),
		fixture.snapshot.Ref().Digest(),
	)
	exact, err := NewExactMemberOfRuntime(
		ExactMemberOfRuntimeInput{
			Engine:                   selectedProjectMemberOfEngine{},
			RuntimeBasisDigest:       runtimeBasisDigest,
			RegistryCoordinateDigest: registryCoordinateDigest,
		},
	)
	if err != nil {
		t.Fatalf("NewExactMemberOfRuntime: %v", err)
	}
	postures := []SelectedMemberOfRuntime{
		NewMemberOfNotRequired(),
		exact,
	}
	for _, posture := range postures {
		if _, err := json.Marshal(posture); !errors.Is(
			err,
			ErrSelectedMemberOfRuntimeNotSerializable,
		) {
			t.Fatalf(
				"json.Marshal(%T) error = %v, want non-serializable runtime",
				posture,
				err,
			)
		}
	}
}

func TestResolveTypeEnvRuntimeTxProjectBranchNeverFallsBack(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	selected := input.Request.Target().VerifiedComposite()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	_, err = fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(1),
		selected,
	)
	if !errors.Is(err, ErrSelectedProjectTypeEnvRuntimeResolverRequired) {
		t.Fatalf(
			"resolveTypeEnvRuntimeTx error = %v, want missing project resolver",
			err,
		)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("failed project resolution finished caller transaction: %v", err)
	}
}

func TestResolveTypeEnvRuntimeTxRejectsUncorrelatedProjectResult(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	selected := input.Request.Target().VerifiedComposite()
	environment := projectRuntimeTestEnvironment(
		t,
		selected,
		fixture.environment,
	)
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{
		environment: environment,
		codecs:      fixture.registry,
		mutateResult: func(
			runtime SelectedProjectTypeEnvRuntime,
		) SelectedProjectTypeEnvRuntime {
			runtime.request.graphRevision = typedmemory.NewGraphRevision(2)
			return runtime
		},
	}
	fixture.adapter.selectedProjectRuntime = resolver
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	_, err = fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		fixture.project,
		typedmemory.NewGraphRevision(1),
		selected,
	)
	if err == nil {
		t.Fatal("resolveTypeEnvRuntimeTx accepted an uncorrelated project result")
	}
	if resolver.calls != 1 {
		t.Fatalf("project runtime resolver calls = %d, want 1", resolver.calls)
	}
}

func TestProjectAwareSnapshotLoaderRejectsMissingProjectResolver(
	t *testing.T,
) {
	t.Parallel()

	fixture := newSQLiteStoreFixture(t)
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	var typedNil *recordingSelectedProjectTypeEnvRuntimeResolver
	tests := []struct {
		name     string
		resolver SelectedProjectTypeEnvRuntimeResolver
	}{
		{name: "nil interface", resolver: nil},
		{name: "typed nil", resolver: typedNil},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			reader, err := NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
				fixture.database,
				loader,
				test.resolver,
			)
			if !errors.Is(
				err,
				ErrSelectedProjectTypeEnvRuntimeResolverRequired,
			) {
				t.Fatalf(
					"NewProjectAwareSQLiteCurrentProjectSnapshotLoader error = %v",
					err,
				)
			}
			if reader != nil {
				t.Fatalf(
					"NewProjectAwareSQLiteCurrentProjectSnapshotLoader = %#v, want nil",
					reader,
				)
			}
		})
	}
}

type projectRuntimeContextKey string

type selectedProjectMemberOfEngine struct{}

func (selectedProjectMemberOfEngine) EvaluateMemberOf(
	context.Context,
	MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, errors.New("selected project MemberOf test engine")
}

func selectedRuntimeTestCoordinates(
	t *testing.T,
	runtimeBasis typedmemory.SHA256Digest,
	registryCoordinate typedmemory.SHA256Digest,
) (SelectedRuntimeBasisDigest, ExactTargetRegistryCoordinateDigest) {
	t.Helper()
	x, err := NewSelectedRuntimeBasisDigest(runtimeBasis)
	if err != nil {
		t.Fatalf("NewSelectedRuntimeBasisDigest: %v", err)
	}
	registry, err := NewExactTargetRegistryCoordinateDigest(registryCoordinate)
	if err != nil {
		t.Fatalf("NewExactTargetRegistryCoordinateDigest: %v", err)
	}
	return x, registry
}

func projectRuntimeTestEnvironment(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	template typedmemory.TypeEnv,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(template.SourceRevision()).
		SetCompilerSchemaVersion(template.CompilerSchemaVersion()).
		SetCoverageManifest(template.CoverageManifest())
	for _, boundedContext := range template.BoundedContexts() {
		builder.AddBoundedContext(boundedContext)
	}
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("build project runtime test TypeEnv: %v", err)
	}
	return environment
}

var _ SelectedProjectTypeEnvRuntimeResolver = (*recordingSelectedProjectTypeEnvRuntimeResolver)(nil)
