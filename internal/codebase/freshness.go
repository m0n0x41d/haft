package codebase

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const CodeIndexSchemaVersion = 6

// IndexFreshnessObservation is the read-only input to code-index coordination.
// It keeps filesystem and stored publication identity separate so the caller
// can make the rebuild decision in a pure policy after it owns the project
// rebuild lease. Missing legacy columns are represented by their zero values
// and therefore cannot be mistaken for a current publication.
type IndexFreshnessObservation struct {
	SourceFingerprint       string
	StoredSourceFingerprint string
	ConfigFingerprint       string
	StoredConfigFingerprint string
	CurrentSchemaVersion    int
	StoredSchemaVersion     int
	PublishedEpoch          int64
	Degraded                bool
	DegradedReason          string
}

// SourceFingerprint computes a cheap fingerprint of the indexable source tree:
// a sha256 over sorted "relPath\x00size\x00mtime" lines for every file that
// ScanSymbols would index (same ignore + language filter). Stat-only — no file
// reads — so it is fast enough to check on each query. Any added/removed file,
// or a size/mtime change, flips the fingerprint; an unchanged tree reproduces
// it exactly. Shell (walk + stat); the hash is deterministic given the metadata.
func (s *Scanner) SourceFingerprint(projectRoot string) (string, error) {
	lines := []string{fmt.Sprintf("code-index-schema\x00%d", CodeIndexSchemaVersion)}
	err := walkProjectFiles(projectRoot, func(
		path string,
		relPath string,
		entry os.DirEntry,
	) error {
		if !s.registry.SupportsSymbols(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		lines = append(
			lines,
			fmt.Sprintf(
				"%s\x00%d\x00%d",
				relPath,
				info.Size(),
				info.ModTime().UnixNano(),
			),
		)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// ObserveIndexFreshness gathers the filesystem and stored identity used by the
// index coordinator. It performs no schema repair and no publication. Callers
// must repeat this observation only after acquiring the project-scoped rebuild
// lease before relying on it to skip parsing.
func (s *Scanner) ObserveIndexFreshness(
	ctx context.Context,
	projectRoot string,
) (IndexFreshnessObservation, error) {
	observation := IndexFreshnessObservation{
		CurrentSchemaVersion: CodeIndexSchemaVersion,
	}
	var err error
	observation.SourceFingerprint, err = s.SourceFingerprint(projectRoot)
	if err != nil {
		return IndexFreshnessObservation{}, fmt.Errorf(
			"observe current code source fingerprint: %w",
			err,
		)
	}
	observation.ConfigFingerprint, err = projectConfigHash(projectRoot)
	if err != nil {
		return IndexFreshnessObservation{}, fmt.Errorf(
			"observe current code configuration fingerprint: %w",
			err,
		)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return IndexFreshnessObservation{}, fmt.Errorf(
			"begin code-index freshness observation: %w",
			err,
		)
	}
	defer func() { _ = tx.Rollback() }()
	columns, err := codeIndexMetaColumns(ctx, tx)
	if err != nil {
		return IndexFreshnessObservation{}, err
	}
	if len(columns) == 0 {
		if err := tx.Commit(); err != nil {
			return IndexFreshnessObservation{}, fmt.Errorf(
				"commit absent code-index freshness observation: %w",
				err,
			)
		}
		return observation, nil
	}

	expression := func(column, fallback string) string {
		if columns[column] {
			return column
		}
		return fallback
	}
	row := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, %s, %s, %s, %s
		FROM code_index_meta WHERE id = 1`,
		expression("fingerprint", "''"),
		expression("config_hash", "''"),
		expression("schema_version", "0"),
		expression("current_epoch", "0"),
		expression("degraded", "0"),
		expression("degraded_reason", "''"),
	))
	var degraded int
	err = row.Scan(
		&observation.StoredSourceFingerprint,
		&observation.StoredConfigFingerprint,
		&observation.StoredSchemaVersion,
		&observation.PublishedEpoch,
		&degraded,
		&observation.DegradedReason,
	)
	if err != nil && err != sql.ErrNoRows {
		return IndexFreshnessObservation{}, fmt.Errorf(
			"read stored code-index freshness: %w",
			err,
		)
	}
	observation.Degraded = degraded != 0
	if err := tx.Commit(); err != nil {
		return IndexFreshnessObservation{}, fmt.Errorf(
			"commit code-index freshness observation: %w",
			err,
		)
	}
	return observation, nil
}

type codeIndexMetaColumnReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func codeIndexMetaColumns(
	ctx context.Context,
	reader codeIndexMetaColumnReader,
) (map[string]bool, error) {
	rows, err := reader.QueryContext(ctx, `PRAGMA table_info(code_index_meta)`)
	if err != nil {
		return nil, fmt.Errorf("inspect code-index metadata columns: %w", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var (
			position     int
			name         string
			columnType   string
			notNull      int
			defaultValue any
			primaryKey   int
		)
		if err := rows.Scan(
			&position,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			return nil, fmt.Errorf(
				"read code-index metadata column: %w",
				err,
			)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate code-index metadata columns: %w", err)
	}
	return columns, nil
}

const codeIndexMetaSchema = `
CREATE TABLE IF NOT EXISTS code_index_meta (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  fingerprint TEXT NOT NULL
);`

// EnsureIndexMetaSchema creates the single-row index-meta table (idempotent).
func (s *Scanner) EnsureIndexMetaSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, codeIndexMetaSchema); err != nil {
		return fmt.Errorf("ensure code_index_meta schema: %w", err)
	}
	return nil
}

// StoredFingerprint returns the fingerprint captured at the last index build, or
// "" if the index has never recorded one (treated as stale by the caller).
func (s *Scanner) StoredFingerprint(ctx context.Context) (string, error) {
	var fp string
	err := s.db.QueryRowContext(ctx, `SELECT fingerprint FROM code_index_meta WHERE id = 1`).Scan(&fp)
	if err != nil {
		return "", nil // no row yet — not an error, just "unknown"
	}
	return fp, nil
}

// SetFingerprint records the fingerprint of the tree the current index was built
// from, so a later query can tell whether the source has changed since.
func (s *Scanner) SetFingerprint(ctx context.Context, fp string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO code_index_meta (id, fingerprint) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET fingerprint = excluded.fingerprint`, fp)
	return err
}
