package projecttypeenvstage

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// LoadSelectionReady reloads the exact immutable closure inside one
// caller-owned read transaction. It reruns final lowering on a cache miss and
// reuses a bounded per-Store result only after all reconstruction bytes match.
func (store *Store) LoadSelectionReady(
	ctx context.Context,
	ref projecttypeenvselection.ProjectTypeEnvStageRef,
) (SelectionReadyStage, error) {
	if ctx == nil {
		return SelectionReadyStage{}, ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return SelectionReadyStage{}, ErrStoreRequired
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, store.database)
	if err != nil {
		return SelectionReadyStage{}, fmt.Errorf(
			"begin project TypeEnv selection-ready transaction: %w",
			err,
		)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	ready, err := store.LoadSelectionReadyTx(ctx, transaction, ref)
	if err != nil {
		return SelectionReadyStage{}, err
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return SelectionReadyStage{}, fmt.Errorf(
			"commit project TypeEnv selection-ready transaction: %w",
			finish.Err(),
		)
	}
	return ready, nil
}

// LoadSelectionReadyTx keeps Stage, verification, executable snapshot, exact
// B/E/X/C, and X catalog reload under one opaque caller-owned SQLite
// transaction. Cache reuse does not skip those reads or their integrity checks.
// The operation performs no head, authority, CAS, or mutation effect.
func (store *Store) LoadSelectionReadyTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref projecttypeenvselection.ProjectTypeEnvStageRef,
) (SelectionReadyStage, error) {
	if ctx == nil {
		return SelectionReadyStage{}, ErrContextRequired
	}
	if store == nil || store.database == nil || store.artifacts == nil {
		return SelectionReadyStage{}, ErrStoreRequired
	}
	if transaction == nil {
		return SelectionReadyStage{}, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return SelectionReadyStage{}, err
	}
	stageRow, err := loadStageRecordTx(ctx, transaction, ref.String())
	if err != nil {
		return SelectionReadyStage{}, err
	}
	verificationRow, err := loadVerificationRecordTx(
		ctx,
		transaction,
		stageRow.verificationRef,
	)
	if err != nil {
		return SelectionReadyStage{}, err
	}
	snapshotRow, err := loadExecutableSnapshotRecordTx(
		ctx,
		transaction,
		stageRow.executableRef,
	)
	if err != nil {
		return SelectionReadyStage{}, err
	}
	rowsKey := newSelectionReadyStageRowsCacheKey(
		stageRow,
		verificationRow,
		snapshotRow,
	)
	inputs := selectionReadyStageReloadInputs{}
	inputsLoaded := false
	if cachedKey, cached, found := store.reloadCache.lookupRows(rowsKey); found {
		inputs, err = loadSelectionReadyStageReloadInputs(
			ctx,
			transaction,
			cached.Stage(),
			cached.VerificationRecord(),
		)
		if err != nil {
			return SelectionReadyStage{}, err
		}
		inputsLoaded = true
		if newSelectionReadyStageCacheKey(
			stageRow,
			verificationRow,
			snapshotRow,
			inputs,
		) == cachedKey {
			return cached, nil
		}
	}
	persisted, err := decodePersistedStage(stageRow, verificationRow, snapshotRow)
	if err != nil {
		return SelectionReadyStage{}, err
	}
	if !inputsLoaded {
		inputs, err = loadSelectionReadyStageReloadInputs(
			ctx,
			transaction,
			persisted.Stage(),
			persisted.VerificationRecord(),
		)
		if err != nil {
			return SelectionReadyStage{}, err
		}
	}
	key := newSelectionReadyStageCacheKey(
		stageRow,
		verificationRow,
		snapshotRow,
		inputs,
	)
	return store.reloadCache.load(rowsKey, key, func() (SelectionReadyStage, error) {
		return rebuildSelectionReadyStage(persisted, inputs)
	})
}

func loadSelectionReadyStageReloadInputs(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	stage projecttypeenvselection.ProjectTypeEnvStage,
	record projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
) (selectionReadyStageReloadInputs, error) {
	base, err := projecttypeenvstore.GetBaseTypeEnvArtifactTx(
		ctx,
		transaction,
		stage.Base(),
	)
	if err != nil {
		return selectionReadyStageReloadInputs{}, fmt.Errorf("reload exact Stage B: %w", err)
	}
	if base.Digest() != record.BaseArtifactDigest() {
		return selectionReadyStageReloadInputs{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("stored B digest differs from final-lowerer record"),
		)
	}
	extensionRefs := stage.OrderedExtensions()
	extensions := make([]projecttypeenv.ProjectTypeEnvExtensionArtifact, 0, len(extensionRefs))
	for index, extensionRef := range extensionRefs {
		extension, loadErr := projecttypeenvstore.GetProjectTypeEnvExtensionArtifactTx(
			ctx,
			transaction,
			extensionRef,
		)
		if loadErr != nil {
			return selectionReadyStageReloadInputs{}, fmt.Errorf(
				"reload exact Stage E[%d] %q: %w",
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
		stage.RuntimeBasis(),
	)
	if err != nil {
		return selectionReadyStageReloadInputs{}, fmt.Errorf("reload exact Stage X: %w", err)
	}
	runtimeMechanisms, registrationPolicies, err :=
		runtimeBasis.ResolvedClosureCanonicalBytes()
	if err != nil {
		return selectionReadyStageReloadInputs{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("verify reloaded X runtime catalogs: %w", err),
		)
	}
	composite, err := projecttypeenvstore.GetProjectTypeEnvCompositeArtifactTx(
		ctx,
		transaction,
		stage.VerifiedComposite(),
	)
	if err != nil {
		return selectionReadyStageReloadInputs{}, fmt.Errorf("reload exact Stage C: %w", err)
	}
	if err := verifyReloadedCompositeCoordinates(stage, composite); err != nil {
		return selectionReadyStageReloadInputs{}, integrityError(stage.Ref().String(), err)
	}
	return selectionReadyStageReloadInputs{
		base:                         base,
		extensions:                   extensions,
		runtimeBasis:                 runtimeBasis,
		runtimeMechanismCanonicals:   runtimeMechanisms,
		registrationPolicyCanonicals: registrationPolicies,
		composite:                    composite,
	}, nil
}

func rebuildSelectionReadyStage(
	persisted PersistedStage,
	inputs selectionReadyStageReloadInputs,
) (SelectionReadyStage, error) {
	stage := persisted.Stage()
	record := persisted.VerificationRecord()
	snapshotRecord := persisted.ExecutableSnapshotRecord()
	base := inputs.base
	extensions := inputs.extensions
	runtimeBasis := inputs.runtimeBasis
	composite := inputs.composite
	extensionRefs := stage.OrderedExtensions()
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, extensions)
	if resolution.Rejected() {
		return SelectionReadyStage{}, integrityError(
			stage.Ref().String(),
			linkRejectionError(resolution.Issues()),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return SelectionReadyStage{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("accepted B/E link produced no linked IR"),
		)
	}
	if err := verifyLinkedExtensionOrder(extensionRefs, linked); err != nil {
		return SelectionReadyStage{}, integrityError(stage.Ref().String(), err)
	}
	snapshot, restored, err := projecttypeenv.RestoreProjectTypeEnvExecutableSnapshot(
		snapshotRecord,
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtimeBasis,
			Composite:    composite,
		},
	)
	if err != nil {
		return SelectionReadyStage{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("restore executable TypeEnv snapshot: %w", err),
		)
	}
	if restored.Ref() != stage.CompositeVerificationRef() ||
		restored.Digest() != stage.CompositeVerificationDigest() ||
		!bytes.Equal(restored.CanonicalBytes(), record.CanonicalBytes()) {
		return SelectionReadyStage{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("restored verification does not byte-match Stage identity"),
		)
	}
	ready := SelectionReadyStage{
		persisted:    persisted,
		verification: restored,
		snapshot:     snapshot,
		capability:   &selectionReadyStageCapability{},
	}
	if err := ready.Verify(); err != nil {
		return SelectionReadyStage{}, err
	}
	return ready, nil
}

func loadStageRecordTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (stageRecord, error) {
	record := stageRecord{}
	executableRef := sql.NullString{}
	err := transaction.ScanOne(
		ctx,
		`SELECT
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			executable_type_env_ref,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_stages
		 WHERE stage_ref = ?`,
		[]any{ref},
		[]any{
			&record.ref,
			&record.digest,
			&record.project,
			&record.verificationRef,
			&executableRef,
			&record.canonicalSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return stageRecord{}, fmt.Errorf("%w: %q", ErrStageNotFound, ref)
	}
	if err != nil {
		return stageRecord{}, fmt.Errorf("load project TypeEnv Stage %q: %w", ref, err)
	}
	record.executableRef = executableRef.String
	if record.ref != ref {
		return stageRecord{}, integrityError(
			ref,
			fmt.Errorf("selected Stage row coordinate changed to %q", record.ref),
		)
	}
	if !executableRef.Valid || record.executableRef == "" {
		return stageRecord{}, integrityError(
			ref,
			fmt.Errorf("stored Stage has no executable snapshot coordinate"),
		)
	}
	return record.clone(), nil
}

func loadVerificationRecordTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (verificationRecord, error) {
	record := verificationRecord{}
	err := transaction.ScanOne(
		ctx,
		`SELECT
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_composite_verifications
		 WHERE verification_ref = ?`,
		[]any{ref},
		[]any{
			&record.ref,
			&record.digest,
			&record.lowererSchema,
			&record.canonicalSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return verificationRecord{}, fmt.Errorf(
			"%w: final-lowerer verification %q",
			ErrStageNotFound,
			ref,
		)
	}
	if err != nil {
		return verificationRecord{}, fmt.Errorf(
			"load final-lowerer verification %q: %w",
			ref,
			err,
		)
	}
	if record.ref != ref {
		return verificationRecord{}, integrityError(
			ref,
			fmt.Errorf("selected verification row coordinate changed to %q", record.ref),
		)
	}
	return record.clone(), nil
}

func loadExecutableSnapshotRecordTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	typeEnvRef string,
) (executableSnapshotRecord, error) {
	record := executableSnapshotRecord{}
	err := transaction.ScanOne(
		ctx,
		`SELECT
			type_env_ref,
			snapshot_digest,
			lowered_environment_digest,
			source_revision,
			compiler_schema_version,
			lowerer_schema_version,
			verification_ref,
			canonical_schema_version,
			canonical_bytes
		 FROM project_typeenv_executable_snapshots
		 WHERE type_env_ref = ?`,
		[]any{typeEnvRef},
		[]any{
			&record.typeEnvRef,
			&record.snapshotDigest,
			&record.loweredDigest,
			&record.sourceRevision,
			&record.compilerSchema,
			&record.lowererSchema,
			&record.verificationRef,
			&record.canonicalSchema,
			&record.canonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return executableSnapshotRecord{}, fmt.Errorf(
			"%w: executable snapshot %q",
			ErrStageNotFound,
			typeEnvRef,
		)
	}
	if err != nil {
		return executableSnapshotRecord{}, fmt.Errorf(
			"load project TypeEnv executable snapshot %q: %w",
			typeEnvRef,
			err,
		)
	}
	if record.typeEnvRef != typeEnvRef {
		return executableSnapshotRecord{}, integrityError(
			typeEnvRef,
			fmt.Errorf(
				"selected executable snapshot coordinate changed to %q",
				record.typeEnvRef,
			),
		)
	}
	return record.clone(), nil
}

func verifyReloadedCompositeCoordinates(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact,
) error {
	if composite.Ref() != stage.VerifiedComposite() {
		return fmt.Errorf("reloaded C reference differs from Stage C")
	}
	if composite.BaseTypeEnvRef() != stage.Base() {
		return fmt.Errorf("reloaded C binds a different B")
	}
	if !orderedExtensionRefsEqual(composite.ExtensionRefs(), stage.OrderedExtensions()) {
		return fmt.Errorf("reloaded C binds a different ordered E DAG")
	}
	if composite.RuntimeEvaluationBasisRef() != stage.RuntimeBasis() {
		return fmt.Errorf("reloaded C binds a different X")
	}
	return nil
}

func verifyLinkedExtensionOrder(
	expected []typedmemory.TypeEnvExtensionRef,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) error {
	actual := linked.Extensions()
	if len(expected) != len(actual) {
		return fmt.Errorf("linked E DAG contains %d entries; Stage binds %d", len(actual), len(expected))
	}
	for index, extension := range actual {
		if expected[index].String() != extension.Artifact().Ref().String() {
			return fmt.Errorf("linked E[%d] differs from Stage ordered E DAG", index)
		}
	}
	return nil
}

func linkRejectionError(issues []projecttypeenv.LinkIssue) error {
	if len(issues) == 0 {
		return fmt.Errorf("B/E relink rejected")
	}
	issue := issues[0]
	return fmt.Errorf(
		"B/E relink rejected with %s at %s: %s",
		issue.Code(),
		issue.Location().String(),
		issue.Detail(),
	)
}
