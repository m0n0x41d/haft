package fpf

import (
	"database/sql"
	"fmt"
	"sort"
)

// SpecIndexSchemaVersion identifies the source-native FPF Query index. The
// historical name is retained because the generated database is an embedded
// release artifact consumed by both the CLI and MCP surfaces.
const SpecIndexSchemaVersion = "11"

// SetSpecMetaEntries writes build provenance after the source-unit index has
// been created. The index builder owns creation of the meta table so this
// operation cannot silently turn an incomplete database into a valid index.
func SetSpecMetaEntries(dbPath string, entries map[string]string) error {
	if len(entries) == 0 {
		return nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open source index metadata: %w", err)
	}
	defer func() { _ = db.Close() }()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin source index metadata transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	statement, err := tx.Prepare(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source index metadata insert: %w", err)
	}
	defer func() { _ = statement.Close() }()

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := statement.Exec(key, entries[key]); err != nil {
			return fmt.Errorf("write source index metadata %q: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source index metadata: %w", err)
	}
	return nil
}

func GetSpecMeta(db *sql.DB, key string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("source index metadata database is required")
	}
	var value string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value); err != nil {
		return "", fmt.Errorf("read source index metadata %q: %w", key, err)
	}
	return value, nil
}

// LoadQuerySourceSnapshot reads the immutable publication coordinates used by
// public FPF Query replay. It does not infer them from one returned source unit:
// the complete index metadata is the snapshot authority.
func LoadQuerySourceSnapshot(db *sql.DB) (QuerySourceSnapshot, error) {
	keys := []string{
		"schema_version",
		"fpf_commit",
		"readme_document_digest",
		"spec_document_digest",
	}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		value, err := GetSpecMeta(db, key)
		if err != nil {
			return QuerySourceSnapshot{}, err
		}
		values = append(values, value)
	}
	snapshot, err := NewQuerySourceSnapshot(
		values[0],
		values[1],
		values[2],
		values[3],
	)
	if err != nil {
		return QuerySourceSnapshot{}, err
	}
	if err := verifyQuerySnapshotRevisionProjection(db, snapshot); err != nil {
		return QuerySourceSnapshot{}, err
	}
	return snapshot, nil
}

func verifyQuerySnapshotRevisionProjection(
	db *sql.DB,
	snapshot QuerySourceSnapshot,
) error {
	tables := []struct {
		name        string
		query       string
		requireRows bool
	}{
		{
			name:        "source_units",
			query:       `SELECT COUNT(*), COUNT(DISTINCT source_revision), COALESCE(MIN(source_revision), '') FROM source_units`,
			requireRows: true,
		},
		{
			name:        "source_unit_relations",
			query:       `SELECT COUNT(*), COUNT(DISTINCT source_revision), COALESCE(MIN(source_revision), '') FROM source_unit_relations`,
			requireRows: false,
		},
	}
	for _, table := range tables {
		var rowCount int
		var revisionCount int
		var revision string
		if err := db.QueryRow(table.query).Scan(&rowCount, &revisionCount, &revision); err != nil {
			return fmt.Errorf("verify FPF query snapshot %s revision: %w", table.name, err)
		}
		if rowCount == 0 && !table.requireRows {
			continue
		}
		if rowCount == 0 {
			return fmt.Errorf("FPF query snapshot %s is empty", table.name)
		}
		if revisionCount != 1 || revision != snapshot.Revision() {
			return fmt.Errorf(
				"FPF query snapshot %s revision projection differs from metadata %q",
				table.name,
				snapshot.Revision(),
			)
		}
	}
	return nil
}
