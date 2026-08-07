package projecttypeenvstage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// LoadExecutableSnapshotTx restores one exact immutable executable TypeEnv
// directly by C. It does not infer a Stage, select a head, or consult a mutable
// latest pointer. The returned capability exists only after replaying final
// lowering against the persisted B/E/X/C closure in the caller transaction.
func (store *Store) LoadExecutableSnapshotTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) (projecttypeenv.ProjectTypeEnvExecutableSnapshot, error) {
	if ctx == nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, ErrStoreRequired
	}
	if transaction == nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, err
	}
	parsed, err := typedmemory.ParseTypeEnvRef(ref.String())
	if err != nil || parsed != ref {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"executable TypeEnv reference is invalid",
		)
	}
	snapshotRow, err := loadExecutableSnapshotRecordTx(
		ctx,
		transaction,
		ref.String(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, err
	}
	snapshotRecord, err := projecttypeenv.DecodeProjectTypeEnvExecutableSnapshotRecord(
		snapshotRow.canonical,
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			ref.String(),
			err,
		)
	}
	if !executableSnapshotRowMatchesRecord(snapshotRow, snapshotRecord) {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			ref.String(),
			fmt.Errorf("stored executable snapshot metadata does not match canonical bytes"),
		)
	}
	verificationRow, err := loadVerificationRecordTx(
		ctx,
		transaction,
		snapshotRecord.VerificationRef().String(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, err
	}
	verificationRecord, err := projecttypeenv.VerifyProjectTypeEnvCompositeVerificationRecord(
		snapshotRecord.VerificationRef(),
		verificationRow.canonical,
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			ref.String(),
			err,
		)
	}
	if !verificationRowMatchesRecord(verificationRow, verificationRecord) {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			ref.String(),
			fmt.Errorf("stored verification metadata does not match canonical bytes"),
		)
	}
	if err := verifyExecutableRecordCoordinates(
		ref,
		verificationRecord,
		snapshotRecord,
	); err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			ref.String(),
			err,
		)
	}
	return store.restoreExecutableSnapshotTx(
		ctx,
		transaction,
		verificationRecord,
		snapshotRecord,
	)
}

func (store *Store) restoreExecutableSnapshotTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	verificationRecord projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
	snapshotRecord projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) (projecttypeenv.ProjectTypeEnvExecutableSnapshot, error) {
	base, err := projecttypeenvstore.GetBaseTypeEnvArtifactTx(
		ctx,
		transaction,
		snapshotRecord.BaseTypeEnvRef(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"reload exact executable B: %w",
			err,
		)
	}
	extensionRefs := snapshotRecord.ExtensionRefs()
	extensions := make([]projecttypeenv.ProjectTypeEnvExtensionArtifact, 0, len(extensionRefs))
	for index, extensionRef := range extensionRefs {
		extension, loadErr := projecttypeenvstore.GetProjectTypeEnvExtensionArtifactTx(
			ctx,
			transaction,
			extensionRef,
		)
		if loadErr != nil {
			return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
				"reload exact executable E[%d] %q: %w",
				index,
				extensionRef.String(),
				loadErr,
			)
		}
		extensions = append(extensions, extension)
	}
	runtimeBasis, err := projecttypeenvstore.GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		snapshotRecord.RuntimeEvaluationBasisRef(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"reload exact executable X: %w",
			err,
		)
	}
	if err := runtimeBasis.VerifyResolvedClosure(); err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			fmt.Errorf("verify reloaded X runtime catalogs: %w", err),
		)
	}
	composite, err := projecttypeenvstore.GetProjectTypeEnvCompositeArtifactTx(
		ctx,
		transaction,
		snapshotRecord.TypeEnvRef(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"reload exact executable C: %w",
			err,
		)
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, extensions)
	if resolution.Rejected() {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			linkRejectionError(resolution.Issues()),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			fmt.Errorf("accepted B/E link produced no linked IR"),
		)
	}
	if err := verifyLinkedExtensionOrder(extensionRefs, linked); err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			err,
		)
	}
	snapshot, restoredVerification, err := projecttypeenv.RestoreProjectTypeEnvExecutableSnapshot(
		snapshotRecord,
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtimeBasis,
			Composite:    composite,
		},
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			fmt.Errorf("restore executable TypeEnv snapshot: %w", err),
		)
	}
	if restoredVerification.Ref() != verificationRecord.Ref() ||
		restoredVerification.Digest() != verificationRecord.Digest() ||
		!bytes.Equal(
			restoredVerification.CanonicalBytes(),
			verificationRecord.CanonicalBytes(),
		) {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			fmt.Errorf("restored verification does not match stored record"),
		)
	}
	if snapshot.TypeEnvRef() != snapshotRecord.TypeEnvRef() ||
		snapshot.Digest() != snapshotRecord.Digest() ||
		!bytes.Equal(snapshot.Record().CanonicalBytes(), snapshotRecord.CanonicalBytes()) {
		return projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, integrityError(
			snapshotRecord.TypeEnvRef().String(),
			fmt.Errorf("restored executable snapshot does not match stored record"),
		)
	}
	return snapshot, nil
}

func executableSnapshotRowMatchesRecord(
	row executableSnapshotRecord,
	record projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) bool {
	expected := executableSnapshotRecord{
		typeEnvRef:      record.TypeEnvRef().String(),
		snapshotDigest:  record.Digest().String(),
		loweredDigest:   record.LoweredEnvironmentDigest().String(),
		sourceRevision:  record.SourceRevision().String(),
		compilerSchema:  record.CompilerSchemaVersion().String(),
		lowererSchema:   record.LowererSchemaVersion(),
		verificationRef: record.VerificationRef().String(),
		canonicalSchema: executableSnapshotCanonicalSchema,
		canonical:       record.CanonicalBytes(),
	}
	return expected.exactEqual(row)
}

func verificationRowMatchesRecord(
	row verificationRecord,
	record projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
) bool {
	expected := verificationRecord{
		ref:             record.Ref().String(),
		digest:          record.Digest().String(),
		lowererSchema:   record.LowererSchemaVersion(),
		canonicalSchema: verificationRecordCanonicalSchema,
		canonical:       record.CanonicalBytes(),
	}
	return expected.exactEqual(row)
}

func verifyExecutableRecordCoordinates(
	ref typedmemory.TypeEnvRef,
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) error {
	if snapshot.TypeEnvRef() != ref || verification.CompositeRef() != ref {
		return fmt.Errorf("executable snapshot C coordinate mismatch")
	}
	if snapshot.VerificationRef() != verification.Ref() {
		return fmt.Errorf("executable snapshot verification coordinate mismatch")
	}
	if snapshot.BaseTypeEnvRef() != verification.BaseTypeEnvRef() {
		return fmt.Errorf("executable snapshot B differs from verification")
	}
	if !orderedExtensionRefsEqual(snapshot.ExtensionRefs(), verification.ExtensionRefs()) {
		return fmt.Errorf("executable snapshot ordered E DAG differs from verification")
	}
	if snapshot.RuntimeEvaluationBasisRef() != verification.RuntimeEvaluationBasisRef() {
		return fmt.Errorf("executable snapshot X differs from verification")
	}
	if snapshot.LoweredEnvironmentDigest() != verification.LoweredEnvironmentDigest() {
		return fmt.Errorf("executable snapshot lowered digest differs from verification")
	}
	if snapshot.LowererSchemaVersion() != verification.LowererSchemaVersion() {
		return fmt.Errorf("executable snapshot lowerer edition differs from verification")
	}
	return nil
}
