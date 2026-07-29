package codebase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const CodeIndexSchemaVersion = 6

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
