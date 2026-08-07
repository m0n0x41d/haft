package typedmemorystore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCommitDeclareEntityRecoversAfterPhysicalCommitReportsFailure(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization:ambiguous-commit",
		candidate,
	)
	requestContext, cancel := context.WithCancel(context.Background())
	fixture.adapter.finisher = cancelAfterCommitFailureFinisher{
		cancel:  cancel,
		failure: errors.New("synthetic transport failure after physical COMMIT"),
	}

	receipt, err := fixture.adapter.commitDeclareEntity(requestContext, request)
	if err != nil {
		t.Fatalf("CommitDeclareEntity recovered outcome: %v", err)
	}
	if receipt.Disposition() != CommitRecovered {
		t.Fatalf(
			"commit disposition = %q; want %q",
			receipt.Disposition(),
			CommitRecovered,
		)
	}
	if !errors.Is(requestContext.Err(), context.Canceled) {
		t.Fatalf("request context error = %v; want cancellation after physical COMMIT", requestContext.Err())
	}
	assertCommittedDeclaration(t, fixture.adapter, fixture, candidate, receipt)
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func TestCommitDeclareEntityRejectsExecutableTypeEnvMetadataDrift(t *testing.T) {
	tests := []struct {
		name           string
		sourceRevision func(*testing.T, sqliteStoreFixture) typedmemory.SourceRevision
		compiler       func(*testing.T, sqliteStoreFixture) typedmemory.CompilerSchemaVersion
	}{
		{
			name: "source revision",
			sourceRevision: func(t *testing.T, fixture sqliteStoreFixture) typedmemory.SourceRevision {
				return mustSourceRevision(t, fixture.snapshot.SourceRevision().String()+"-drift")
			},
			compiler: func(_ *testing.T, fixture sqliteStoreFixture) typedmemory.CompilerSchemaVersion {
				return fixture.snapshot.CompilerSchemaVersion()
			},
		},
		{
			name: "compiler schema version",
			sourceRevision: func(_ *testing.T, fixture sqliteStoreFixture) typedmemory.SourceRevision {
				return fixture.snapshot.SourceRevision()
			},
			compiler: func(t *testing.T, fixture sqliteStoreFixture) typedmemory.CompilerSchemaVersion {
				return mustCompilerVersion(t, fixture.snapshot.CompilerSchemaVersion().String()+"-drift")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSQLiteStoreFixture(t)
			drifted := buildDriftedTypeEnv(
				t,
				fixture.environment,
				test.sourceRevision(t, fixture),
				test.compiler(t, fixture),
			)
			fixture.adapter.loader = staticTypeEnvLoader{
				reference:   fixture.environment.Ref(),
				environment: drifted,
				registry:    fixture.registry,
			}
			candidate := fixture.declaration(t, "authorization-service", "Authorization service")
			request := fixture.request(
				t,
				0,
				fixture.environment.Ref(),
				"declare:authorization:typeenv-drift",
				candidate,
			)

			receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) ||
				!strings.Contains(err.Error(), "loaded generic TypeEnv differs from immutable snapshot metadata") {
				t.Fatalf("CommitDeclareEntity drift error = %v; want immutable-metadata mismatch", err)
			}
			if receipt != (CommitReceipt{}) {
				t.Fatalf("TypeEnv drift returned non-zero receipt: %+v", receipt)
			}
			assertTypedMemoryRowCounts(t, fixture.database, emptyDeclarationCounts())
		})
	}
}

type syntheticFinishFailure struct {
	failure error
}

func (result syntheticFinishFailure) Succeeded() bool { return false }

func (result syntheticFinishFailure) Err() error { return result.failure }

type cancelAfterCommitFailureFinisher struct {
	cancel  context.CancelFunc
	failure error
}

func (finisher cancelAfterCommitFailureFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	physical := transaction.Commit(ctx)
	if !physical.Succeeded() {
		return physical
	}
	finisher.cancel()
	return syntheticFinishFailure{failure: finisher.failure}
}

func buildDriftedTypeEnv(
	t *testing.T,
	original typedmemory.TypeEnv,
	sourceRevision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(original.Ref()).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compiler).
		SetCoverageManifest(original.CoverageManifest())
	for _, boundedContext := range original.BoundedContexts() {
		builder.AddBoundedContext(boundedContext)
	}
	drifted, err := builder.Build()
	if err != nil {
		t.Fatalf("build drifted TypeEnv: %v", err)
	}
	return drifted
}
