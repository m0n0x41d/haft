package codebase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

// ErrDatabaseIntegrity marks structural ledger damage. It is distinct from
// ordinary lock, cancellation, and transport failures: callers must not start
// expensive parser work or attempt schema repair after observing it.
var ErrDatabaseIntegrity = errors.New("project ledger database integrity failure")

// IsDatabaseIntegrityFailure reports failures that prove SQLite cannot safely
// traverse the current ledger structure. Extended result codes are normalized
// to their primary code before classification.
func IsDatabaseIntegrityFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDatabaseIntegrity) {
		return true
	}
	var sqliteErr *sqlitedriver.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() & 0xff {
	case sqlitelib.SQLITE_CORRUPT,
		sqlitelib.SQLITE_FORMAT,
		sqlitelib.SQLITE_NOTADB:
		return true
	default:
		return false
	}
}

// RequireDatabaseIntegrityForIndexRefresh performs a bounded, read-only
// structural check immediately before an expensive code-index rebuild. It is
// intentionally not part of the ordinary fresh-index read path.
func (s *Scanner) RequireDatabaseIntegrityForIndexRefresh(
	ctx context.Context,
) error {
	if ctx == nil {
		return fmt.Errorf("code-index integrity preflight context is required")
	}
	var result string
	err := s.db.QueryRowContext(ctx, `PRAGMA main.quick_check(1)`).Scan(&result)
	if err != nil {
		if IsDatabaseIntegrityFailure(err) {
			return fmt.Errorf(
				"%w: run read-only SQLite quick_check: %w",
				ErrDatabaseIntegrity,
				err,
			)
		}
		return fmt.Errorf("run read-only SQLite quick_check: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(result), "ok") {
		return nil
	}
	detail := strings.Join(strings.Fields(result), " ")
	const maxDetailBytes = 512
	if len(detail) > maxDetailBytes {
		detail = detail[:maxDetailBytes] + "..."
	}
	return fmt.Errorf(
		"%w: SQLite quick_check reported %q",
		ErrDatabaseIntegrity,
		detail,
	)
}
