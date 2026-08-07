package db

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

const legacyAffectedPathSchemaVersion58 = 58

var legacyAffectedPathMigration58 = Migration{
	Version: legacyAffectedPathSchemaVersion58,
	Description: "Drop pre-invariant affected_files rows that are not " +
		"project-relative",
	Apply: applyLegacyAffectedPathMigration58,
}

// unexpressibleAffectedRow58 is one stored artifact<->file row the
// project-relative path invariant cannot express.
type unexpressibleAffectedRow58 struct {
	artifactID string
	path       string
}

// applyLegacyAffectedPathMigration58 removes artifact<->file rows that the
// project-relative path invariant cannot express.
//
// Rows were admitted before canonicalAffectedFile existed, so a database that
// predates that invariant can hold absolute or traversing paths. Readers that
// enforce the invariant then exclude those rows on every pass; deleting them
// once makes the stored table agree with what every reader can express.
//
// The invariant itself belongs to projectpath. This migration asks that
// package rather than restating the rule in SQL, so the policy stays in one
// place and cannot drift away from the read path.
func applyLegacyAffectedPathMigration58(
	tx MigrationTransaction,
	_ []Migration,
) error {
	unexpressible, err := readUnexpressibleAffectedRows58(tx)
	if err != nil {
		return err
	}
	if len(unexpressible) == 0 {
		return nil
	}
	for _, row := range unexpressible {
		if _, err := tx.Exec(
			`DELETE FROM affected_files
			 WHERE artifact_id = ? AND file_path = ?`,
			row.artifactID,
			row.path,
		); err != nil {
			return fmt.Errorf(
				"drop non-project-relative affected file for %s: %w",
				row.artifactID,
				err,
			)
		}
	}
	if err := verifyNoUnexpressibleAffectedRows58(tx); err != nil {
		return err
	}
	reportDroppedAffectedRows58(unexpressible)
	return nil
}

func readUnexpressibleAffectedRows58(
	tx MigrationTransaction,
) ([]unexpressibleAffectedRow58, error) {
	rows, err := tx.Query(
		`SELECT artifact_id, file_path FROM affected_files
		 ORDER BY artifact_id, file_path`,
	)
	if err != nil {
		return nil, fmt.Errorf("read stored affected files: %w", err)
	}
	defer rows.Close()
	unexpressible := make([]unexpressibleAffectedRow58, 0)
	for rows.Next() {
		var row unexpressibleAffectedRow58
		if err := rows.Scan(&row.artifactID, &row.path); err != nil {
			return nil, fmt.Errorf("scan stored affected file: %w", err)
		}
		if _, err := projectpath.Parse(row.path); err == nil {
			continue
		}
		unexpressible = append(unexpressible, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("enumerate stored affected files: %w", err)
	}
	return unexpressible, nil
}

func verifyNoUnexpressibleAffectedRows58(tx MigrationTransaction) error {
	remaining, err := readUnexpressibleAffectedRows58(tx)
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf(
			"%d non-project-relative affected file(s) survived the migration",
			len(remaining),
		)
	}
	return nil
}

// reportDroppedAffectedRows58 names what the migration deleted. Deleting stored
// rows without saying so would leave the operator no way to tell a repaired
// database from one that never carried the rows.
func reportDroppedAffectedRows58(dropped []unexpressibleAffectedRow58) {
	artifactIDs := make([]string, 0, len(dropped))
	seen := make(map[string]bool, len(dropped))
	for _, row := range dropped {
		if seen[row.artifactID] {
			continue
		}
		seen[row.artifactID] = true
		artifactIDs = append(artifactIDs, row.artifactID)
	}
	sort.Strings(artifactIDs)
	log.Printf(
		"haft migration %d: dropped %d non-project-relative affected_files "+
			"row(s) across %d artifact(s): %s",
		legacyAffectedPathSchemaVersion58,
		len(dropped),
		len(artifactIDs),
		strings.Join(artifactIDs, ", "),
	)
}
