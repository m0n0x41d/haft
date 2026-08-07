package projecttypeenvstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestTransactionReadersRereadExactCommittedArtifactClosure(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}

	baseRef, _ := fixture.base.TypeEnvRef()
	base, err := GetBaseTypeEnvArtifactTx(ctx, transaction, baseRef)
	if err != nil {
		t.Fatalf("GetBaseTypeEnvArtifactTx(): %v", err)
	}
	extension, err := GetProjectTypeEnvExtensionArtifactTx(
		ctx,
		transaction,
		fixture.extension.Ref(),
	)
	if err != nil {
		t.Fatalf("GetProjectTypeEnvExtensionArtifactTx(): %v", err)
	}
	runtime, err := GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		fixture.runtime.Ref(),
	)
	if err != nil {
		t.Fatalf("GetRuntimeEvaluationBasisArtifactTx(): %v", err)
	}
	composite, err := GetProjectTypeEnvCompositeArtifactTx(
		ctx,
		transaction,
		fixture.composite.Ref(),
	)
	if err != nil {
		t.Fatalf("GetProjectTypeEnvCompositeArtifactTx(): %v", err)
	}
	assertExactBytes(t, "transaction B", base.CanonicalBytes(), fixture.base.CanonicalBytes())
	assertExactBytes(
		t,
		"transaction E",
		extension.CanonicalBytes(),
		fixture.extension.CanonicalBytes(),
	)
	assertExactBytes(
		t,
		"transaction X",
		runtime.CanonicalBytes(),
		fixture.runtime.CanonicalBytes(),
	)
	assertExactBytes(
		t,
		"transaction C",
		composite.CanonicalBytes(),
		fixture.composite.CanonicalBytes(),
	)
	assertRollbackSucceeded(t, transaction.Rollback(ctx))
}

func TestTransactionRuntimeReaderResolvesExactMechanismClosure(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newNonEmptyRuntimeFixture(t)
	if err := store.PutRuntimeEvaluationBasisClosure(
		ctx,
		fixture.basis,
		[]runtimemechanism.RuntimeMechanismArtifactV1{fixture.mechanism},
	); err != nil {
		t.Fatalf("PutRuntimeEvaluationBasisClosure(): %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}

	basis, err := GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		fixture.basis.Ref(),
	)
	if err != nil {
		t.Fatalf("GetRuntimeEvaluationBasisArtifactTx(): %v", err)
	}
	if err := basis.VerifyResolvedClosure(); err != nil {
		t.Fatalf("transaction X VerifyResolvedClosure(): %v", err)
	}
	identity := fixture.mechanism.Identity()
	mechanism, err := GetRuntimeMechanismArtifactTx(
		ctx,
		transaction,
		identity.Artifact(),
		identity.Edition(),
		identity.Digest(),
	)
	if err != nil {
		t.Fatalf("GetRuntimeMechanismArtifactTx(): %v", err)
	}
	assertExactBytes(
		t,
		"transaction runtime mechanism",
		mechanism.CanonicalBytes(),
		fixture.mechanism.CanonicalBytes(),
	)

	wrongDigest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("0", 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	_, err = GetRuntimeMechanismArtifactTx(
		ctx,
		transaction,
		identity.Artifact(),
		identity.Edition(),
		wrongDigest,
	)
	if !errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("wrong mechanism digest error = %v, want ErrArtifactIntegrity", err)
	}
	assertRollbackSucceeded(t, transaction.Rollback(ctx))
}

func TestTransactionReaderRejectsWrongKindAndRolledBackCorruption(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}

	_, err = GetBaseTypeEnvArtifactTx(ctx, transaction, fixture.composite.Ref())
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("B reader at C ref error = %v, want ErrArtifactNotFound", err)
	}
	baseRef, _ := fixture.base.TypeEnvRef()
	_, err = transaction.Execute(
		ctx,
		`UPDATE project_typeenv_artifacts
		 SET canonical_bytes = ?
		 WHERE artifact_kind = ? AND artifact_ref = ?`,
		[]any{
			fixture.composite.CanonicalBytes(),
			string(ArtifactBaseTypeEnv),
			baseRef.String(),
		},
	)
	if err != nil {
		t.Fatalf("corrupt B inside transaction: %v", err)
	}
	_, err = GetBaseTypeEnvArtifactTx(ctx, transaction, baseRef)
	if !errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("corrupt B transaction read error = %v, want ErrArtifactIntegrity", err)
	}
	assertRollbackSucceeded(t, transaction.Rollback(ctx))

	base, err := store.GetBaseTypeEnvArtifact(ctx, baseRef)
	if err != nil {
		t.Fatalf("GetBaseTypeEnvArtifact() after rollback: %v", err)
	}
	assertExactBytes(t, "rolled-back B", base.CanonicalBytes(), fixture.base.CanonicalBytes())
}

func TestTransactionReaderSeesUncommittedInsertOnlyInsideOwningTransaction(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	record, _, err := prepareBaseArtifact(fixture.base)
	if err != nil {
		t.Fatalf("prepareBaseArtifact(): %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		[]any{
			string(record.kind),
			record.ref,
			record.digest,
			record.canonicalSchema,
			record.producerSchema,
			record.canonical,
		},
	)
	if err != nil {
		t.Fatalf("insert B inside transaction: %v", err)
	}
	baseRef, _ := fixture.base.TypeEnvRef()
	base, err := GetBaseTypeEnvArtifactTx(ctx, transaction, baseRef)
	if err != nil {
		t.Fatalf("GetBaseTypeEnvArtifactTx(uncommitted): %v", err)
	}
	if !bytes.Equal(base.CanonicalBytes(), fixture.base.CanonicalBytes()) {
		t.Fatal("transaction reader changed uncommitted B canonical bytes")
	}
	assertRollbackSucceeded(t, transaction.Rollback(ctx))

	_, err = store.GetBaseTypeEnvArtifact(ctx, baseRef)
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("outside read after rollback error = %v, want ErrArtifactNotFound", err)
	}
}

func TestTransactionReadersRejectInvalidAndFinishedCapabilities(t *testing.T) {
	ctx := context.Background()
	database, _ := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	baseRef, _ := fixture.base.TypeEnvRef()

	_, err := GetBaseTypeEnvArtifactTx(ctx, nil, baseRef)
	if !errors.Is(err, sqlitetransaction.ErrTransactionInvalid) {
		t.Fatalf("nil transaction error = %v, want ErrTransactionInvalid", err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	assertRollbackSucceeded(t, transaction.Rollback(ctx))
	_, err = GetBaseTypeEnvArtifactTx(ctx, transaction, baseRef)
	if !errors.Is(err, sqlitetransaction.ErrTransactionFinished) {
		t.Fatalf("finished transaction error = %v, want ErrTransactionFinished", err)
	}
	_, err = GetBaseTypeEnvArtifactTx(nil, transaction, baseRef)
	if !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil context error = %v, want ErrContextRequired", err)
	}
}

func assertRollbackSucceeded(
	t *testing.T,
	result sqlitetransaction.FinishResult,
) {
	t.Helper()
	if !result.Succeeded() {
		t.Fatalf("Rollback(): %v", result.Err())
	}
}
