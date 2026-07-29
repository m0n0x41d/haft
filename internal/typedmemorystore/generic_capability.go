package typedmemorystore

import (
	"context"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func requireGenericStorageCapability(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) error {
	availability, err := loadGenericStorageAvailability(ctx, transaction)
	if err != nil {
		return err
	}
	if availability != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	return nil
}

func requireAdmissionBasisStorageCapability(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisKind typedmemory.AdmissionBasisKind,
) error {
	if basisKind != typedmemory.ContextSliceClassificationAdmissionBasis {
		return nil
	}
	availability, err := loadKindClassificationStorageAvailability(
		ctx,
		transaction,
	)
	if err != nil {
		return err
	}
	if availability != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	return nil
}

func requireGenericAdmissionStorageCapability(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	version AdmissionContractVersion,
) error {
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return err
	}
	if version.IsV1() {
		return nil
	}
	if !version.IsV2() {
		return ErrStorageGenerationUnavailable
	}
	availability, err := loadRelationalAssertionStorageAvailability(
		ctx,
		transaction,
	)
	if err != nil {
		return err
	}
	if availability != genericStorageExact {
		return ErrStorageGenerationUnavailable
	}
	return nil
}
