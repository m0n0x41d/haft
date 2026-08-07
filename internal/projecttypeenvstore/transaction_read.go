package projecttypeenvstore

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// GetBaseTypeEnvArtifactTx rereads one exact immutable B from the caller-owned
// SQLite transaction. It neither selects a project head nor changes storage.
func GetBaseTypeEnvArtifactTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) (typeenv.BaseTypeEnvArtifact, error) {
	if err := requireReadTransaction(ctx, transaction); err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	if err := validateTypeEnvRef(ref); err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	value, err := readArtifactWithScanner(
		ctx,
		transaction,
		ArtifactBaseTypeEnv,
		ref.String(),
	)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	artifact, ok := value.(typeenv.BaseTypeEnvArtifact)
	if !ok {
		return typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

// GetProjectTypeEnvExtensionArtifactTx rereads one exact immutable E from the
// caller-owned SQLite transaction.
func GetProjectTypeEnvExtensionArtifactTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvExtensionRef,
) (projecttypeenv.ProjectTypeEnvExtensionArtifact, error) {
	if err := requireReadTransaction(ctx, transaction); err != nil {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, err
	}
	if err := validateExtensionRef(ref); err != nil {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, err
	}
	value, err := readArtifactWithScanner(
		ctx,
		transaction,
		ArtifactExtensionTypeEnv,
		ref.String(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, err
	}
	artifact, ok := value.(projecttypeenv.ProjectTypeEnvExtensionArtifact)
	if !ok {
		return projecttypeenv.ProjectTypeEnvExtensionArtifact{}, integrityError(
			ArtifactExtensionTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

// GetRuntimeEvaluationBasisArtifactTx rereads one exact immutable X and every
// exact runtime-mechanism artifact it pins from the same caller-owned SQLite
// transaction. The returned X has passed resolved-closure verification.
func GetRuntimeEvaluationBasisArtifactTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref projecttypeenv.RuntimeEvaluationBasisRef,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	if err := requireReadTransaction(ctx, transaction); err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	if err := validateRuntimeBasisRef(ref); err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	value, err := readArtifactWithScanner(
		ctx,
		transaction,
		ArtifactRuntimeBasis,
		ref.String(),
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	artifact, ok := value.(projecttypeenv.RuntimeEvaluationBasisArtifact)
	if !ok {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return resolveStoredRuntimeBasis(ctx, transaction, artifact)
}

// GetProjectTypeEnvCompositeArtifactTx rereads one exact immutable C recipe
// from the caller-owned SQLite transaction. A valid C is not by itself proof
// that final lowering into an executable TypeEnv succeeded.
func GetProjectTypeEnvCompositeArtifactTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) (projecttypeenv.ProjectTypeEnvCompositeArtifact, error) {
	if err := requireReadTransaction(ctx, transaction); err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, err
	}
	if err := validateTypeEnvRef(ref); err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, err
	}
	value, err := readArtifactWithScanner(
		ctx,
		transaction,
		ArtifactCompositeTypeEnv,
		ref.String(),
	)
	if err != nil {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, err
	}
	artifact, ok := value.(projecttypeenv.ProjectTypeEnvCompositeArtifact)
	if !ok {
		return projecttypeenv.ProjectTypeEnvCompositeArtifact{}, integrityError(
			ArtifactCompositeTypeEnv,
			ref.String(),
			fmt.Errorf("decoded artifact has type %T", value),
		)
	}
	return artifact, nil
}

// GetRuntimeMechanismArtifactTx rereads one exact runtime-mechanism artifact
// from the caller-owned SQLite transaction and checks all three identity
// coordinates against its canonical bytes.
func GetRuntimeMechanismArtifactTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	artifactRef typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	digest typedmemory.SHA256Digest,
) (runtimemechanism.RuntimeMechanismArtifactV1, error) {
	if err := requireReadTransaction(ctx, transaction); err != nil {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, err
	}
	pin, err := projecttypeenv.NewRuntimeMechanismArtifactPin(
		projecttypeenv.RuntimeMechanismArtifactPinInput{
			Artifact: artifactRef,
			Edition:  edition,
			Digest:   digest,
		},
	)
	if err != nil {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, err
	}
	record, err := loadRuntimeMechanismRecord(
		ctx,
		transaction,
		pin.Artifact().String(),
		pin.Edition().String(),
	)
	if err != nil {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, err
	}
	if record.digest != pin.Digest().String() {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, integrityError(
			ArtifactRuntimeBasis,
			record.coordinate(),
			fmt.Errorf(
				"stored digest is %q; exact reader requested %q",
				record.digest,
				pin.Digest().String(),
			),
		)
	}
	artifact, err := decodeRuntimeMechanismRecord(record)
	if err != nil {
		return runtimemechanism.RuntimeMechanismArtifactV1{}, integrityError(
			ArtifactRuntimeBasis,
			record.coordinate(),
			err,
		)
	}
	return artifact, nil
}

func requireReadTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if transaction == nil {
		return sqlitetransaction.ErrTransactionInvalid
	}
	return transaction.RequireActive()
}
