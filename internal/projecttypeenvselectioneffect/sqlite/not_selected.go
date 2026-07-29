package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

// currentGenesisFrameResult is a private closed outcome. Expected
// current-world non-selection is data, never Go error control flow.
type currentGenesisFrameResult interface {
	currentGenesisFrameResultVariant()
}

type currentGenesisFrameReady struct {
	frame currentGenesisFrame
}

func (currentGenesisFrameReady) currentGenesisFrameResultVariant() {}

type currentGenesisFrameRejected struct {
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
}

func (currentGenesisFrameRejected) currentGenesisFrameResultVariant() {}

// currentGenesisAuthorityResult applies the same distinction at the authority
// boundary: an absent current permission is an expected result, while malformed
// immutable input, transport failure, or an impossible internal state remains
// an error.
type currentGenesisAuthorityResult interface {
	currentGenesisAuthorityResultVariant()
}

type currentGenesisAuthorityReady struct {
	use *admittedGenesisAuthorityUse
}

func (currentGenesisAuthorityReady) currentGenesisAuthorityResultVariant() {}

type currentGenesisAuthorityRejected struct {
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
}

func (currentGenesisAuthorityRejected) currentGenesisAuthorityResultVariant() {}

func rejectCurrentGenesisFrame(
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) currentGenesisFrameResult {
	return currentGenesisFrameRejected{reason: reason}
}

func rejectCurrentGenesisAuthority() currentGenesisAuthorityResult {
	return rejectCurrentGenesisAuthorityFor(
		projecttypeenvselectioneffect.NotSelectedCurrentAuthorityRejection(),
	)
}

func rejectCurrentGenesisAuthorityFor(
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) currentGenesisAuthorityResult {
	return currentGenesisAuthorityRejected{
		reason: reason,
	}
}

func genesisNotSelectedOutcome(
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) (genesisTransactionOutcome, error) {
	result, err := projecttypeenvselectioneffect.NewNotSelected(reason)
	if err != nil {
		return genesisTransactionOutcome{exactReplayRuledOut: true}, err
	}
	return genesisTransactionOutcome{
		result:              result,
		exactReplayRuledOut: true,
	}, nil
}

// preCommitNotSelectedReason classifies only failures that can safely be
// retried as an expected current-world non-selection. SQLite integrity,
// constraint, schema, misuse, format, and corruption codes deliberately remain
// errors. Callers must additionally prove either that no transaction started or
// that exact replay was ruled out and rollback succeeded.
func preCommitNotSelectedReason(
	cause error,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
	bool,
) {
	if cause == nil {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{},
			false
	}
	if errors.Is(cause, context.Canceled) ||
		errors.Is(cause, context.DeadlineExceeded) {
		return projecttypeenvselectioneffect.NotSelectedCancellation(), true
	}
	if errors.Is(cause, sql.ErrConnDone) ||
		errors.Is(cause, driver.ErrBadConn) {
		return projecttypeenvselectioneffect.NotSelectedStorageFailure(), true
	}
	var sqliteError *sqlitedriver.Error
	if !errors.As(cause, &sqliteError) {
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{},
			false
	}
	switch sqliteError.Code() & 0xff {
	case sqlitelib.SQLITE_BUSY,
		sqlitelib.SQLITE_LOCKED,
		sqlitelib.SQLITE_READONLY,
		sqlitelib.SQLITE_IOERR,
		sqlitelib.SQLITE_FULL,
		sqlitelib.SQLITE_CANTOPEN:
		return projecttypeenvselectioneffect.NotSelectedStorageFailure(), true
	default:
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{},
			false
	}
}
