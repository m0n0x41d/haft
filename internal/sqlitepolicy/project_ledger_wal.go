// Package sqlitepolicy installs process-wide SQLite connection policies that
// are narrower than the generic database driver defaults.
package sqlitepolicy

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	sqlitedriver "modernc.org/sqlite"
)

func init() {
	sqlitedriver.RegisterConnectionHook(applyProjectLedgerConnectionPolicy)
}

func applyProjectLedgerConnectionPolicy(
	connection sqlitedriver.ExecQuerierContext,
	dsn string,
) error {
	if !isWritableProjectLedgerDSN(dsn) {
		return nil
	}
	fileControl, ok := connection.(sqlitedriver.FileControl)
	if !ok {
		return fmt.Errorf(
			"project ledger SQLite connection does not support persistent WAL control",
		)
	}
	mode, err := fileControl.FileControlPersistWAL("main", 1)
	if err != nil {
		return fmt.Errorf(
			"enable persistent project ledger WAL generation: %w",
			err,
		)
	}
	if mode != 1 {
		return fmt.Errorf(
			"enable persistent project ledger WAL generation: mode = %d, want 1",
			mode,
		)
	}
	return nil
}

func isWritableProjectLedgerDSN(dsn string) bool {
	databasePath, query, ok := sqliteDSNParts(dsn)
	if !ok || strings.EqualFold(query.Get("mode"), "ro") {
		return false
	}
	cleanedPath := filepath.Clean(databasePath)
	projectDirectory := filepath.Dir(cleanedPath)
	projectsDirectory := filepath.Dir(projectDirectory)
	haftDirectory := filepath.Dir(projectsDirectory)
	return filepath.Base(cleanedPath) == "haft.db" &&
		filepath.Base(projectsDirectory) == "projects" &&
		filepath.Base(haftDirectory) == ".haft"
}

func sqliteDSNParts(dsn string) (string, url.Values, bool) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", nil, false
	}
	if parsed.Scheme != "" && parsed.Scheme != "file" {
		return "", nil, false
	}
	databasePath := parsed.Path
	if parsed.Scheme == "" {
		databasePath = strings.SplitN(dsn, "?", 2)[0]
	}
	if databasePath == "" || databasePath == ":memory:" {
		return "", nil, false
	}
	return databasePath, parsed.Query(), true
}
