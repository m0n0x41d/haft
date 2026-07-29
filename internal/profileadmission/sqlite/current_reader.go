package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// CurrentCanonicalProfileObservation is a closed exact-read result. Store
// absence is data; corruption and operational failure are errors.
type CurrentCanonicalProfileObservation interface {
	ProjectRoot() projectprofile.ProjectRootV1
	LedgerRevision() projectprofile.LedgerRevision
	currentCanonicalProfileObservationVariant()
}

type NoCurrentCanonicalProfile struct {
	root projectprofile.ProjectRootV1
}

func (NoCurrentCanonicalProfile) currentCanonicalProfileObservationVariant() {}

func (value NoCurrentCanonicalProfile) ProjectRoot() projectprofile.ProjectRootV1 {
	return value.root
}

func (NoCurrentCanonicalProfile) LedgerRevision() projectprofile.LedgerRevision {
	return projectprofile.NewLedgerRevision(0)
}

type DeclaredCurrentCanonicalProfile struct {
	admission CanonicalProfileAdmission
}

func (DeclaredCurrentCanonicalProfile) currentCanonicalProfileObservationVariant() {}

func (value DeclaredCurrentCanonicalProfile) ProjectRoot() projectprofile.ProjectRootV1 {
	return value.admission.ProjectRoot()
}

func (value DeclaredCurrentCanonicalProfile) LedgerRevision() projectprofile.LedgerRevision {
	return value.admission.LedgerRevision()
}

func (value DeclaredCurrentCanonicalProfile) Admission() CanonicalProfileAdmission {
	return value.admission
}

type CurrentProfileReadFailureKind uint8

const (
	CurrentProfileStoreOperationalFailure CurrentProfileReadFailureKind = iota + 1
	CurrentProfileStoreCorruption
)

func (kind CurrentProfileReadFailureKind) String() string {
	switch kind {
	case CurrentProfileStoreOperationalFailure:
		return "operational_failure"
	case CurrentProfileStoreCorruption:
		return "store_corruption"
	default:
		return ""
	}
}

// CurrentProfileReadError keeps retryable transaction/store failure separate
// from a positive but invalid canonical footprint.
type CurrentProfileReadError struct {
	kind  CurrentProfileReadFailureKind
	cause error
}

func (err CurrentProfileReadError) Error() string {
	return fmt.Sprintf("current project-profile %s: %v", err.kind.String(), err.cause)
}

func (err CurrentProfileReadError) Unwrap() error { return err.cause }

func (err CurrentProfileReadError) Kind() CurrentProfileReadFailureKind {
	return err.kind
}

// ResolveCurrentWithin reloads the exact current profile in the caller-owned
// SQLite snapshot. It never commits or rolls back the transaction.
func ResolveCurrentWithin(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
) (CurrentCanonicalProfileObservation, error) {
	if ctx == nil || transaction == nil {
		return nil, operationalCurrentProfileReadError(
			fmt.Errorf("current project-profile read requires context and transaction"),
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return nil, operationalCurrentProfileReadError(err)
	}
	root, err := projectprofile.NewProjectRootV1(projectRoot.String())
	if err != nil || root != projectRoot {
		return nil, operationalCurrentProfileReadError(
			fmt.Errorf("canonical project root is required"),
		)
	}
	head, err := loadExactLedgerHead(ctx, transaction, root)
	if err != nil {
		return nil, classifyCurrentProfileReadError(ctx, err)
	}
	if head.Value() == 0 {
		return NoCurrentCanonicalProfile{root: root}, nil
	}
	material, err := resolveCurrentCanonicalOnConnection(ctx, transaction, root)
	if err != nil {
		return nil, classifyCurrentProfileReadError(ctx, err)
	}
	if err := validateHistoricalAuthorityMaterialInTransaction(
		ctx,
		transaction,
		material,
	); err != nil {
		return nil, classifyCurrentProfileReadError(ctx, err)
	}
	admission, err := newCanonicalProfileAdmission(
		material,
		CanonicalAdmissionResolvedAfterRestart,
	)
	if err != nil {
		return nil, corruptCurrentProfileReadError(err)
	}
	if admission.LedgerRevision() != head {
		return nil, corruptCurrentProfileReadError(
			fmt.Errorf("resolved profile admission differs from exact ledger head"),
		)
	}
	return DeclaredCurrentCanonicalProfile{admission: admission}, nil
}

func validateHistoricalAuthorityMaterialInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material canonicalAdmissionMaterial,
) error {
	if material.storageGeneration == "v1" {
		return nil
	}
	if material.storageGeneration == "v3" {
		return validateV3HistoricalMaterialInTransaction(ctx, transaction, material)
	}
	if material.storageGeneration != "v2" {
		return fmt.Errorf("canonical admission has an unknown storage generation")
	}
	useRef, err := profileauthority.NewProfileDeclarationAuthorityUseRef(
		material.authorityUseRef,
	)
	if err != nil {
		return fmt.Errorf("parse historical v2 authority-use ref: %w", err)
	}
	useDigest, err := authority.NewDigest(material.authorityUseDigest.String())
	if err != nil {
		return fmt.Errorf("parse historical v2 authority-use digest: %w", err)
	}
	use, err := profileauthoritysqlite.LoadAuthorityUseRecordInTransaction(
		ctx,
		transaction,
		useRef,
		useDigest,
	)
	if err != nil {
		return fmt.Errorf("strict-load historical v2 authority use in transaction: %w", err)
	}
	return validateAuthorityUseAgainstMaterial(use, material)
}

func operationalCurrentProfileReadError(cause error) CurrentProfileReadError {
	return CurrentProfileReadError{
		kind:  CurrentProfileStoreOperationalFailure,
		cause: cause,
	}
}

func corruptCurrentProfileReadError(cause error) CurrentProfileReadError {
	return CurrentProfileReadError{
		kind:  CurrentProfileStoreCorruption,
		cause: cause,
	}
}

func classifyCurrentProfileReadError(
	ctx context.Context,
	cause error,
) CurrentProfileReadError {
	if errors.Is(cause, context.Canceled) ||
		errors.Is(cause, context.DeadlineExceeded) ||
		errors.Is(cause, sqlitetransaction.ErrContextRequired) ||
		errors.Is(cause, sqlitetransaction.ErrDatabaseRequired) ||
		errors.Is(cause, sqlitetransaction.ErrTransactionInvalid) ||
		errors.Is(cause, sqlitetransaction.ErrTransactionFinished) {
		return operationalCurrentProfileReadError(cause)
	}
	if ctx != nil && ctx.Err() != nil {
		return operationalCurrentProfileReadError(cause)
	}
	type sqliteCodedError interface{ Code() int }
	var sqliteError sqliteCodedError
	if errors.As(cause, &sqliteError) {
		return operationalCurrentProfileReadError(cause)
	}
	return corruptCurrentProfileReadError(cause)
}
