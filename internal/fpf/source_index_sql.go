package fpf

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const sourceCandidateProducerLimit = 200

const sourceUnitsFTSSchema = `CREATE VIRTUAL TABLE IF NOT EXISTS source_units_fts USING fts5(
	unit_id UNINDEXED,
	source_role UNINDEXED,
	title,
	body,
	authored_phrases,
	keywords,
	tokenize='unicode61'
)`

var sourceSearchTokenRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

var sourceQuerySchema = []string{
	`CREATE TABLE IF NOT EXISTS source_units (
		unit_id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL DEFAULT '',
		source_role TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		pattern_id TEXT NOT NULL DEFAULT '',
		parent_pattern_id TEXT NOT NULL DEFAULT '',
		publication_status TEXT NOT NULL DEFAULT '',
		direct_refs_json TEXT NOT NULL DEFAULT '[]',
		relation_count INTEGER NOT NULL DEFAULT 0 CHECK (relation_count >= 0),
		relations_digest TEXT NOT NULL,
		authored_phrases_json TEXT NOT NULL DEFAULT '[]',
		keywords_json TEXT NOT NULL DEFAULT '[]',
		use_cues_json TEXT NOT NULL DEFAULT '{}',
		source_path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		source_revision TEXT NOT NULL
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS source_units_source_id_unique
		ON source_units(source_id) WHERE source_id <> ''`,
	`CREATE INDEX IF NOT EXISTS source_units_role_lines
		ON source_units(source_role, start_line, unit_id)`,
	`CREATE INDEX IF NOT EXISTS source_units_pattern
		ON source_units(pattern_id, source_role)`,
	`CREATE TABLE IF NOT EXISTS source_unit_refs (
		unit_id TEXT NOT NULL,
		ref_id TEXT NOT NULL,
		PRIMARY KEY (unit_id, ref_id),
		FOREIGN KEY (unit_id) REFERENCES source_units(unit_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS source_unit_relations (
		unit_id TEXT NOT NULL,
		relation_ordinal INTEGER NOT NULL CHECK (relation_ordinal >= 0),
		relation_kind TEXT NOT NULL,
		target_pattern_id TEXT NOT NULL,
		target_class TEXT NOT NULL,
		origin TEXT NOT NULL,
		source_path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		source_revision TEXT NOT NULL,
		PRIMARY KEY (unit_id, relation_ordinal),
		UNIQUE (unit_id, relation_kind, target_pattern_id),
		FOREIGN KEY (unit_id) REFERENCES source_units(unit_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS source_unit_relations_target
		ON source_unit_relations(target_pattern_id, relation_kind, unit_id)`,
	`CREATE TABLE IF NOT EXISTS source_authored_phrases (
		unit_id TEXT NOT NULL,
		phrase TEXT NOT NULL,
		PRIMARY KEY (unit_id, phrase),
		FOREIGN KEY (unit_id) REFERENCES source_units(unit_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS source_keywords (
		unit_id TEXT NOT NULL,
		keyword TEXT NOT NULL,
		PRIMARY KEY (unit_id, keyword),
		FOREIGN KEY (unit_id) REFERENCES source_units(unit_id) ON DELETE CASCADE
	)`,
	sourceUnitsFTSSchema,
}

type SQLiteQueryIndex struct {
	db *sql.DB
}

func NewSQLiteQueryIndex(db *sql.DB) QueryIndex {
	return SQLiteQueryIndex{db: db}
}

func EnsureSourceQuerySchemaDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("source query database is required")
	}
	for _, statement := range sourceQuerySchema {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("create source query schema: %w", err)
		}
	}
	return nil
}

func StoreSourceUnits(dbPath string, units []SourceUnit) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open source query index: %w", err)
	}
	defer func() { _ = db.Close() }()
	return StoreSourceUnitsDB(db, units)
}

func StoreSourceUnitsDB(db *sql.DB, units []SourceUnit) error {
	if err := ValidateSourceUnits(units); err != nil {
		return err
	}
	if err := EnsureSourceQuerySchemaDB(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin source query index transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range []string{
		`DELETE FROM source_units_fts`,
		`DELETE FROM source_unit_relations`,
		`DELETE FROM source_unit_refs`,
		`DELETE FROM source_authored_phrases`,
		`DELETE FROM source_keywords`,
		`DELETE FROM source_units`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("clear source query index: %w", err)
		}
	}

	unitInsert, err := tx.Prepare(`
		INSERT INTO source_units (
			unit_id, source_id, source_role, title, body, pattern_id,
			parent_pattern_id, publication_status, direct_refs_json,
			relation_count, relations_digest,
			authored_phrases_json, keywords_json, use_cues_json,
			source_path, start_line, end_line, content_hash, source_revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source unit insert: %w", err)
	}
	defer func() { _ = unitInsert.Close() }()

	ftsInsert, err := tx.Prepare(`
		INSERT INTO source_units_fts (
			unit_id, source_role, title, body, authored_phrases, keywords
		) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source FTS insert: %w", err)
	}
	defer func() { _ = ftsInsert.Close() }()

	refInsert, err := tx.Prepare(`INSERT INTO source_unit_refs (unit_id, ref_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source ref insert: %w", err)
	}
	defer func() { _ = refInsert.Close() }()

	relationInsert, err := tx.Prepare(`
		INSERT INTO source_unit_relations (
			unit_id, relation_ordinal, relation_kind, target_pattern_id,
			target_class, origin, source_path, start_line, end_line,
			content_hash, source_revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source relation insert: %w", err)
	}
	defer func() { _ = relationInsert.Close() }()

	phraseInsert, err := tx.Prepare(`INSERT INTO source_authored_phrases (unit_id, phrase) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare authored phrase insert: %w", err)
	}
	defer func() { _ = phraseInsert.Close() }()

	keywordInsert, err := tx.Prepare(`INSERT INTO source_keywords (unit_id, keyword) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare source keyword insert: %w", err)
	}
	defer func() { _ = keywordInsert.Close() }()

	for _, unit := range units {
		directRefsJSON, err := encodeSourceJSON(unit.DirectRefs)
		if err != nil {
			return fmt.Errorf("encode source refs for %s: %w", unit.UnitID, err)
		}
		authoredPhrasesJSON, err := encodeSourceJSON(unit.AuthoredPhrases)
		if err != nil {
			return fmt.Errorf("encode authored phrases for %s: %w", unit.UnitID, err)
		}
		keywordsJSON, err := encodeSourceJSON(unit.Keywords)
		if err != nil {
			return fmt.Errorf("encode source keywords for %s: %w", unit.UnitID, err)
		}
		useCuesJSON, err := encodeSourceJSON(unit.UseCues)
		if err != nil {
			return fmt.Errorf("encode source use cues for %s: %w", unit.UnitID, err)
		}
		relationsDigest, err := sourceRelationsDigest(unit.Relations)
		if err != nil {
			return fmt.Errorf("encode source relation integrity for %s: %w", unit.UnitID, err)
		}
		if _, err := unitInsert.Exec(
			unit.UnitID,
			unit.SourceID,
			unit.Role,
			unit.Title,
			unit.Body,
			unit.PatternID,
			unit.ParentPatternID,
			unit.PublicationStatus,
			directRefsJSON,
			len(unit.Relations),
			relationsDigest,
			authoredPhrasesJSON,
			keywordsJSON,
			useCuesJSON,
			unit.Provenance.SourcePath,
			unit.Provenance.StartLine,
			unit.Provenance.EndLine,
			unit.Provenance.ContentHash,
			unit.Provenance.SourceRevision,
		); err != nil {
			return fmt.Errorf("insert source unit %s: %w", unit.UnitID, err)
		}

		if _, err := ftsInsert.Exec(
			unit.UnitID,
			unit.Role,
			unit.Title,
			unit.Body,
			strings.Join(unit.AuthoredPhrases, "\n"),
			strings.Join(unit.Keywords, " "),
		); err != nil {
			return fmt.Errorf("insert source FTS unit %s: %w", unit.UnitID, err)
		}

		for _, ref := range unit.DirectRefs {
			if _, err := refInsert.Exec(unit.UnitID, ref); err != nil {
				return fmt.Errorf("insert source ref %s -> %s: %w", unit.UnitID, ref, err)
			}
		}
		for ordinal, relation := range unit.Relations {
			if _, err := relationInsert.Exec(
				unit.UnitID,
				ordinal,
				relation.Kind,
				relation.TargetPatternID,
				relation.TargetClass,
				relation.Origin,
				relation.Provenance.SourcePath,
				relation.Provenance.StartLine,
				relation.Provenance.EndLine,
				relation.Provenance.ContentHash,
				relation.Provenance.SourceRevision,
			); err != nil {
				return fmt.Errorf("insert source relation for %s: %w", unit.UnitID, err)
			}
		}
		for _, phrase := range unit.AuthoredPhrases {
			if _, err := phraseInsert.Exec(unit.UnitID, phrase); err != nil {
				return fmt.Errorf("insert authored phrase for %s: %w", unit.UnitID, err)
			}
		}
		for _, keyword := range unit.Keywords {
			if _, err := keywordInsert.Exec(unit.UnitID, keyword); err != nil {
				return fmt.Errorf("insert keyword for %s: %w", unit.UnitID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source query index: %w", err)
	}
	return VerifySourceQueryIndexDB(db)
}

func VerifySourceQueryIndexDB(db *sql.DB) error {
	if err := EnsureSourceQuerySchemaDB(db); err != nil {
		return err
	}
	return verifySourceQueryIndexReadOnly(db)
}

// VerifySourceQueryIndexReadOnlyDB verifies an existing source projection
// without creating or repairing schema objects. It is safe for release guards
// that must leave malformed and legacy database files byte-for-byte untouched.
func VerifySourceQueryIndexReadOnlyDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("source query database is required")
	}
	return verifySourceQueryIndexReadOnly(db)
}

func verifySourceQueryIndexReadOnly(db *sql.DB) error {
	if err := verifySQLiteIntegrityReadOnly(db); err != nil {
		return err
	}
	units, err := loadSourceUnits(db, "", nil)
	if err != nil {
		return err
	}
	if err := ValidateSourceUnits(units); err != nil {
		return fmt.Errorf("verify source query grammar: %w", err)
	}
	if err := verifyStoredTOCBodyCompleteness(units); err != nil {
		return err
	}
	if err := verifyStoredSourceRelations(db, units); err != nil {
		return err
	}
	if err := verifyStoredUnitStringProjection(
		db,
		"direct-reference",
		`SELECT unit_id, ref_id FROM source_unit_refs ORDER BY unit_id, ref_id`,
		units,
		func(unit SourceUnit) []string { return unit.DirectRefs },
	); err != nil {
		return err
	}
	if err := verifyStoredUnitStringProjection(
		db,
		"authored-phrase",
		`SELECT unit_id, phrase FROM source_authored_phrases ORDER BY unit_id, phrase`,
		units,
		func(unit SourceUnit) []string { return unit.AuthoredPhrases },
	); err != nil {
		return err
	}
	if err := verifyStoredUnitStringProjection(
		db,
		"keyword",
		`SELECT unit_id, keyword FROM source_keywords ORDER BY unit_id, keyword`,
		units,
		func(unit SourceUnit) []string { return unit.Keywords },
	); err != nil {
		return err
	}
	if err := verifyStoredFTSProjection(db, units); err != nil {
		return err
	}

	return verifyStoredDirectReferenceTargets(db)
}

type unitStringProjectionRow struct {
	unitID string
	value  string
}

func verifyStoredUnitStringProjection(
	db *sql.DB,
	projectionName string,
	query string,
	units []SourceUnit,
	values func(SourceUnit) []string,
) error {
	expected, err := expectedUnitStringProjection(units, values)
	if err != nil {
		return fmt.Errorf("derive canonical %s projection: %w", projectionName, err)
	}
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("query %s projection: %w", projectionName, err)
	}
	defer func() { _ = rows.Close() }()
	actual := make([]unitStringProjectionRow, 0, len(expected))
	for rows.Next() {
		row := unitStringProjectionRow{}
		if err := rows.Scan(&row.unitID, &row.value); err != nil {
			return fmt.Errorf("scan %s projection: %w", projectionName, err)
		}
		actual = append(actual, row)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s projection: %w", projectionName, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s projection: %w", projectionName, err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf(
			"source %s projection row mismatch: canonical %d, stored %d",
			projectionName,
			len(expected),
			len(actual),
		)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf(
				"source %s projection mismatch at row %d: canonical %q/%q, stored %q/%q",
				projectionName,
				index,
				expected[index].unitID,
				expected[index].value,
				actual[index].unitID,
				actual[index].value,
			)
		}
	}
	return nil
}

func expectedUnitStringProjection(
	units []SourceUnit,
	values func(SourceUnit) []string,
) ([]unitStringProjectionRow, error) {
	projection := make([]unitStringProjectionRow, 0)
	seen := make(map[unitStringProjectionRow]struct{})
	for _, unit := range units {
		for _, value := range values(unit) {
			row := unitStringProjectionRow{unitID: unit.UnitID, value: value}
			if _, exists := seen[row]; exists {
				return nil, fmt.Errorf("duplicate canonical row %q/%q", row.unitID, row.value)
			}
			seen[row] = struct{}{}
			projection = append(projection, row)
		}
	}
	sort.Slice(projection, func(left, right int) bool {
		if projection[left].unitID != projection[right].unitID {
			return projection[left].unitID < projection[right].unitID
		}
		return projection[left].value < projection[right].value
	})
	return projection, nil
}

type sourceFTSProjectionRow struct {
	unitID          string
	sourceRole      string
	title           string
	body            string
	authoredPhrases string
	keywords        string
}

func verifyStoredFTSProjection(db *sql.DB, units []SourceUnit) error {
	if err := verifyStoredFTSSchema(db); err != nil {
		return err
	}
	expected := make([]sourceFTSProjectionRow, 0, len(units))
	for _, unit := range units {
		expected = append(expected, sourceFTSProjectionRow{
			unitID:          unit.UnitID,
			sourceRole:      string(unit.Role),
			title:           unit.Title,
			body:            unit.Body,
			authoredPhrases: strings.Join(unit.AuthoredPhrases, "\n"),
			keywords:        strings.Join(unit.Keywords, " "),
		})
	}
	sort.Slice(expected, func(left, right int) bool {
		return expected[left].unitID < expected[right].unitID
	})
	rows, err := db.Query(`
		SELECT unit_id, source_role, title, body, authored_phrases, keywords
		FROM source_units_fts
		ORDER BY unit_id`)
	if err != nil {
		return fmt.Errorf("query source FTS projection: %w", err)
	}
	defer func() { _ = rows.Close() }()
	actual := make([]sourceFTSProjectionRow, 0, len(expected))
	for rows.Next() {
		row := sourceFTSProjectionRow{}
		if err := rows.Scan(
			&row.unitID,
			&row.sourceRole,
			&row.title,
			&row.body,
			&row.authoredPhrases,
			&row.keywords,
		); err != nil {
			return fmt.Errorf("scan source FTS projection: %w", err)
		}
		actual = append(actual, row)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate source FTS projection: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close source FTS projection: %w", err)
	}
	if len(actual) != len(expected) {
		return fmt.Errorf(
			"source FTS projection row mismatch: canonical %d, stored %d",
			len(expected),
			len(actual),
		)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf(
				"source FTS projection mismatch for canonical unit %q at row %d",
				expected[index].unitID,
				index,
			)
		}
	}
	return verifyStoredFTSRuntime(db, units)
}

func verifySQLiteIntegrityReadOnly(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("run SQLite integrity check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	results := make([]string, 0, 1)
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("scan SQLite integrity check: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate SQLite integrity check: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close SQLite integrity check: %w", err)
	}
	if len(results) != 1 || results[0] != "ok" {
		return fmt.Errorf("SQLite integrity check failed: expected one ok result, got %q", results)
	}
	return nil
}

func verifyStoredFTSSchema(db *sql.DB) error {
	var objectType string
	var storedSchema string
	err := db.QueryRow(`
		SELECT type, sql
		FROM sqlite_master
		WHERE name = 'source_units_fts'`).Scan(&objectType, &storedSchema)
	if err != nil {
		return fmt.Errorf("inspect source FTS schema: %w", err)
	}
	if objectType != "table" {
		return fmt.Errorf("source FTS schema object type = %q, want table", objectType)
	}
	expected := normalizeSQLiteSchema(sourceUnitsFTSSchema)
	actual := normalizeSQLiteSchema(storedSchema)
	if actual != expected {
		return fmt.Errorf("source FTS schema mismatch: expected %q, stored %q", expected, actual)
	}
	return nil
}

func normalizeSQLiteSchema(statement string) string {
	normalized := strings.Join(strings.Fields(statement), " ")
	normalized = strings.Replace(normalized, " IF NOT EXISTS ", " ", 1)
	return normalized
}

func verifyStoredFTSRuntime(db *sql.DB, units []SourceUnit) error {
	unitID, expression, err := sourceFTSRuntimeProbe(units)
	if err != nil {
		return err
	}
	var matchedUnitID string
	var rank float64
	err = db.QueryRow(`
		SELECT unit_id, bm25(source_units_fts)
		FROM source_units_fts
		WHERE source_units_fts MATCH ? AND unit_id = ?
		ORDER BY bm25(source_units_fts), unit_id
		LIMIT 1`, expression, unitID).Scan(&matchedUnitID, &rank)
	if err != nil {
		return fmt.Errorf("run source FTS MATCH/bm25 probe: %w", err)
	}
	if matchedUnitID != unitID {
		return fmt.Errorf("source FTS MATCH/bm25 probe returned %q, want %q", matchedUnitID, unitID)
	}
	return nil
}

func sourceFTSRuntimeProbe(units []SourceUnit) (string, string, error) {
	for _, unit := range units {
		for _, value := range []string{unit.Title, unit.Body, strings.Join(unit.AuthoredPhrases, " "), strings.Join(unit.Keywords, " ")} {
			tokens := sourceSearchTokens(value)
			if len(tokens) == 0 {
				continue
			}
			expression := sourceFTSExpression(tokens[0])
			if expression != "" {
				return unit.UnitID, expression, nil
			}
		}
	}
	return "", "", fmt.Errorf("source FTS projection has no searchable runtime probe")
}

func verifyStoredDirectReferenceTargets(db *sql.DB) error {
	var missingUnit string
	var missingRef string
	err := db.QueryRow(`
		SELECT refs.unit_id, refs.ref_id
		FROM source_unit_refs refs
		WHERE NOT EXISTS (
			SELECT 1 FROM source_units source_target
			WHERE source_target.source_id = refs.ref_id
				AND source_target.source_id <> ''
		)
		AND NOT EXISTS (
			SELECT 1 FROM source_units pattern_target
			WHERE pattern_target.pattern_id = refs.ref_id
		)
		AND NOT EXISTS (
			SELECT 1
			FROM source_units ref_owner
			JOIN source_units relation_owner
				ON relation_owner.pattern_id = ref_owner.pattern_id
			JOIN source_unit_relations relation
				ON relation.unit_id = relation_owner.unit_id
			WHERE ref_owner.unit_id = refs.unit_id
				AND relation.target_pattern_id = refs.ref_id
				AND relation.target_class = ?
		)
		LIMIT 1`, SourceRelationTargetClassUnresolvedAuthored).Scan(&missingUnit, &missingRef)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("verify source direct references: %w", err)
	}
	if err == nil {
		return fmt.Errorf("source direct reference %s -> %s has no addressable target", missingUnit, missingRef)
	}
	return nil
}

// VerifyPublicationSnapshotDB proves that the stored Query projection was
// derived from the exact immutable publication snapshot shared with other
// source compilers. Internal index consistency alone cannot prove this join.
func VerifyPublicationSnapshotDB(db *sql.DB, snapshot PublicationSnapshot) error {
	if err := VerifySourceQueryIndexDB(db); err != nil {
		return err
	}
	return verifyPublicationSnapshotProjection(db, snapshot)
}

// VerifyPublicationSnapshotReadOnlyDB proves the exact snapshot join without
// any schema DDL. Callers should additionally open the SQLite database with
// mode=ro so this property is enforced below the verifier implementation.
func VerifyPublicationSnapshotReadOnlyDB(
	db *sql.DB,
	snapshot PublicationSnapshot,
) error {
	if err := VerifySourceQueryIndexReadOnlyDB(db); err != nil {
		return err
	}
	return verifyPublicationSnapshotProjection(db, snapshot)
}

func verifyPublicationSnapshotProjection(
	db *sql.DB,
	snapshot PublicationSnapshot,
) error {
	stored, err := loadSourceUnits(db, "", nil)
	if err != nil {
		return err
	}
	expected := snapshot.SourceUnits()
	if len(stored) != len(expected) {
		return fmt.Errorf(
			"publication snapshot has %d source units but database has %d",
			len(expected),
			len(stored),
		)
	}
	expectedByID := make(map[string]SourceUnit, len(expected))
	for _, unit := range expected {
		expectedByID[unit.UnitID] = unit
	}
	for _, unit := range stored {
		want, exists := expectedByID[unit.UnitID]
		if !exists {
			return fmt.Errorf("stored source unit %q is outside publication snapshot", unit.UnitID)
		}
		storedJSON, marshalErr := json.Marshal(unit)
		if marshalErr != nil {
			return fmt.Errorf("encode stored source unit %q: %w", unit.UnitID, marshalErr)
		}
		expectedJSON, marshalErr := json.Marshal(want)
		if marshalErr != nil {
			return fmt.Errorf("encode snapshot source unit %q: %w", unit.UnitID, marshalErr)
		}
		if !bytes.Equal(storedJSON, expectedJSON) {
			return fmt.Errorf("stored source unit %q differs from publication snapshot", unit.UnitID)
		}
	}
	metadata := []struct {
		key  string
		want string
	}{
		{key: "fpf_commit", want: snapshot.Revision()},
		{key: "readme_document_digest", want: snapshot.ReadmeDigest().String()},
		{key: "spec_document_digest", want: snapshot.SpecDigest().String()},
	}
	for _, entry := range metadata {
		got, readErr := GetSpecMeta(db, entry.key)
		if readErr != nil {
			return readErr
		}
		if got != entry.want {
			return fmt.Errorf("source metadata %q=%q, want %q", entry.key, got, entry.want)
		}
	}
	return nil
}

func CountSourceUnits(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM source_units`).Scan(&count)
	return count, err
}

// RebuildSourceIndexAtomic keeps the previous derived file intact until the
// complete callback output passes its caller-supplied final verification.
// The verifier runs against the exact temporary database that will be renamed.
func RebuildSourceIndexAtomic(
	targetPath string,
	build func(tempPath string) error,
	verify func(db *sql.DB) error,
) error {
	if build == nil {
		return fmt.Errorf("source query index build callback is required")
	}
	if verify == nil {
		return fmt.Errorf("source index final verifier is required")
	}

	targetPath = filepath.Clean(targetPath)
	directory := filepath.Dir(targetPath)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(targetPath)+".build-*")
	if err != nil {
		return fmt.Errorf("create temporary source query index: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary source query index: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("prepare temporary source query index: %w", err)
	}
	defer removeSQLiteBuildFiles(temporaryPath)

	if err := build(temporaryPath); err != nil {
		return fmt.Errorf("build source query index: %w", err)
	}

	db, err := sql.Open("sqlite", temporaryPath)
	if err != nil {
		return fmt.Errorf("open rebuilt source query index: %w", err)
	}
	verifyErr := verify(db)
	closeErr := db.Close()
	if verifyErr != nil {
		return verifyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close rebuilt source query index: %w", closeErr)
	}

	file, err := os.OpenFile(temporaryPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open rebuilt source query index for sync: %w", err)
	}
	syncErr := file.Sync()
	closeErr = file.Close()
	if syncErr != nil {
		return fmt.Errorf("sync rebuilt source query index: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close synced source query index: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("atomically replace source query index: %w", err)
	}
	return syncDirectory(directory)
}

// RebuildSourceQueryIndexAtomic retains the narrow query-only public contract.
// Combined source-derived artifacts use RebuildSourceIndexAtomic so one final
// verifier can prove their shared publication snapshot before replacement.
func RebuildSourceQueryIndexAtomic(targetPath string, build func(tempPath string) error) error {
	return RebuildSourceIndexAtomic(targetPath, build, VerifySourceQueryIndexDB)
}

func (index SQLiteQueryIndex) LookupExact(identifier string, roles []SourceUnitRole) (SourceUnit, bool, error) {
	return index.lookupExact(identifier, roles)
}

func (index SQLiteQueryIndex) InspectExact(identifier string, roles []SourceUnitRole) (SourceUnit, bool, error) {
	return index.lookupExact(identifier, roles)
}

func (index SQLiteQueryIndex) lookupExact(identifier string, roles []SourceUnitRole) (SourceUnit, bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return SourceUnit{}, false, nil
	}

	if unit, found, err := index.uniqueExactMatch("unit_id", identifier, roles); found || err != nil {
		return unit, found, err
	}
	if unit, found, err := index.uniqueExactMatch("source_id", identifier, roles); found || err != nil {
		return unit, found, err
	}
	if len(roles) == 1 {
		if unit, found, err := index.uniqueExactMatch("pattern_id", identifier, roles); found || err != nil {
			return unit, found, err
		}
	}
	return index.exactTitleMatch(identifier, roles)
}

func (index SQLiteQueryIndex) exactTitleMatch(title string, roles []SourceUnitRole) (SourceUnit, bool, error) {
	where, args := sourceRoleWhere("source_role", roles)
	query := sourceUnitSelect + ` WHERE lower(title) = lower(?) AND ` + where + ` ORDER BY start_line, unit_id`
	args = append([]any{title}, args...)
	units, err := loadSourceUnits(index.db, query, args)
	if err != nil {
		return SourceUnit{}, false, err
	}
	if len(units) == 1 {
		return units[0], true, nil
	}
	if len(units) < 2 {
		return SourceUnit{}, false, nil
	}

	patternID := units[0].PatternID
	if patternID == "" {
		return SourceUnit{}, false, nil
	}
	var body SourceUnit
	bodyCount := 0
	for _, unit := range units {
		if !strings.EqualFold(unit.PatternID, patternID) {
			return SourceUnit{}, false, nil
		}
		if unit.Role == SourceUnitRolePatternBody {
			body = unit
			bodyCount++
		}
	}
	if bodyCount != 1 {
		return SourceUnit{}, false, nil
	}
	return body, true, nil
}

func (index SQLiteQueryIndex) uniqueExactMatch(column, value string, roles []SourceUnitRole) (SourceUnit, bool, error) {
	if !isExactSourceColumn(column) {
		return SourceUnit{}, false, fmt.Errorf("unsupported exact source column %q", column)
	}
	where, args := sourceRoleWhere("source_role", roles)
	query := sourceUnitSelect + ` WHERE lower(` + column + `) = lower(?) AND ` + where + ` ORDER BY start_line, unit_id LIMIT 2`
	args = append([]any{value}, args...)
	units, err := loadSourceUnits(index.db, query, args)
	if err != nil {
		return SourceUnit{}, false, err
	}
	if len(units) != 1 {
		return SourceUnit{}, false, nil
	}
	return units[0], true, nil
}

func (index SQLiteQueryIndex) SearchSourceProbePhrases(phrases []SourceProbePhrase, roles []SourceUnitRole) (CandidateBatch, error) {
	candidates := make([]RetrievedCandidate, 0)
	for _, phrase := range phrases {
		for _, role := range roles {
			units, err := index.searchSourceProbePhrase(phrase, role)
			if err != nil {
				return CandidateBatch{}, err
			}
			for _, unit := range units {
				candidate := newRetrievedCandidate(
					unit,
					RetrievalTierRoleLocalFTS,
					phrase.ProbeField,
					sourceFieldDerivedPhrase,
					phrase.Value,
				)
				candidate.MatchGrounds[0].PhraseKind = phrase.Kind
				candidates = appendOrMergeRetrieved(candidates, candidate)
			}
		}
	}
	return boundCandidateBatch(candidates, "derived_source_phrase_producer_limit"), nil
}

func (index SQLiteQueryIndex) searchSourceProbePhrase(phrase SourceProbePhrase, role SourceUnitRole) ([]SourceUnit, error) {
	if phrase.Kind == SourcePhraseKindExactProbeSpan {
		query := sourceUnitSelect + `
			WHERE source_role = ? AND (
				instr(lower(title), lower(?)) > 0 OR
				instr(lower(body), lower(?)) > 0
			)
			ORDER BY start_line, unit_id
			LIMIT ?`
		args := []any{role, phrase.Value, phrase.Value, sourceCandidateProducerLimit + 1}
		return loadSourceUnits(index.db, query, args)
	}
	if phrase.Kind != SourcePhraseKindScaffoldCompressed {
		return nil, fmt.Errorf("unsupported source phrase kind %q", phrase.Kind)
	}
	expression := sourceRoleLocalExactPhraseExpression(phrase.Value)
	if expression == "" {
		return nil, nil
	}
	query := sourceUnitSelectAliased("u") + `
		FROM source_units_fts
		JOIN source_units u ON u.unit_id = source_units_fts.unit_id
		WHERE source_units_fts MATCH ? AND u.source_role = ?
		ORDER BY bm25(source_units_fts), u.start_line, u.unit_id
		LIMIT ?`
	args := []any{expression, role, sourceCandidateProducerLimit + 1}
	return loadSourceUnits(index.db, query, args)
}

func (index SQLiteQueryIndex) SearchAuthoredPhrases(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error) {
	candidates := make([]RetrievedCandidate, 0)
	for _, input := range sourceProbeInputs(probe) {
		where, roleArgs := sourceRoleWhere("u.source_role", roles)
		query := sourceUnitSelectAliased("u") + `, phrases.phrase
			FROM source_units u
			JOIN source_authored_phrases phrases ON phrases.unit_id = u.unit_id
			WHERE ` + where + ` AND (
				lower(phrases.phrase) = lower(?) OR
				instr(lower(phrases.phrase), lower(?)) > 0 OR
				instr(lower(?), lower(phrases.phrase)) > 0
			)
			ORDER BY u.start_line, u.unit_id
			LIMIT ?`
		args := slices.Clone(roleArgs)
		args = append(args, input.Value, input.Value, input.Value, sourceCandidateProducerLimit+1)
		matched, err := loadMatchedSourceUnits(index.db, query, args)
		if err != nil {
			return CandidateBatch{}, err
		}
		for _, row := range matched {
			candidates = appendOrMergeRetrieved(candidates, RetrievedCandidate{
				Unit: row.Unit,
				MatchGrounds: []MatchGround{{
					Tier:         RetrievalTierAuthoredPhrase,
					ProbeField:   input.Field,
					SourceField:  "authored_phrases",
					MatchedValue: row.Matched,
				}},
			})
		}
	}
	return boundCandidateBatch(candidates, "authored_phrase_producer_limit"), nil
}

func (index SQLiteQueryIndex) SearchHeadingsAndKeywords(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error) {
	candidates := make([]RetrievedCandidate, 0)
	for _, input := range sourceProbeInputs(probe) {
		for _, token := range sourceGroundingSearchTokens(input.Value) {
			sourceIDMatches, err := index.searchSourceFieldContains("source_id", token, roles)
			if err != nil {
				return CandidateBatch{}, err
			}
			for _, unit := range sourceIDMatches {
				candidates = appendOrMergeRetrieved(candidates, newRetrievedCandidate(
					unit,
					RetrievalTierHeadingKeyword,
					input.Field,
					"source_id",
					token,
				))
			}

			titleMatches, err := index.searchSourceFieldContains("title", token, roles)
			if err != nil {
				return CandidateBatch{}, err
			}
			for _, unit := range titleMatches {
				candidates = appendOrMergeRetrieved(candidates, newRetrievedCandidate(
					unit,
					RetrievalTierHeadingKeyword,
					input.Field,
					"heading",
					token,
				))
			}

			keywordMatches, err := index.searchKeywordContains(token, roles)
			if err != nil {
				return CandidateBatch{}, err
			}
			for _, row := range keywordMatches {
				candidates = appendOrMergeRetrieved(candidates, newRetrievedCandidate(
					row.Unit,
					RetrievalTierHeadingKeyword,
					input.Field,
					"keywords",
					row.Matched,
				))
			}
		}
	}
	return boundCandidateBatch(candidates, "heading_keyword_producer_limit"), nil
}

func (index SQLiteQueryIndex) SearchRoleLocalFTS(probe CandidateProbe, roles []SourceUnitRole) (CandidateBatch, error) {
	candidates := make([]RetrievedCandidate, 0)
	inputs := sourceProbeInputs(probe)
	for _, input := range inputs {
		for _, token := range sourceGroundingSearchTokens(input.Value) {
			expression := sourceRoleLocalFTSExpression(token)
			for _, role := range roles {
				query := sourceUnitSelectAliased("u") + `
					FROM source_units_fts
					JOIN source_units u ON u.unit_id = source_units_fts.unit_id
					WHERE source_units_fts MATCH ? AND u.source_role = ?
					ORDER BY bm25(source_units_fts), u.start_line, u.unit_id
					LIMIT ?`
				args := []any{expression, role, sourceCandidateProducerLimit + 1}
				units, err := loadSourceUnits(index.db, query, args)
				if err != nil {
					return CandidateBatch{}, err
				}
				for _, unit := range units {
					candidates = appendOrMergeRetrieved(candidates, newRetrievedCandidate(
						unit,
						RetrievalTierRoleLocalFTS,
						input.Field,
						sourceFieldTitleBodyToken,
						token,
					))
				}
			}
		}
	}
	bounded := boundCandidateBatch(candidates, "role_local_fts_producer_limit")
	omitted, omittedBasis, err := index.countOmittedRoleLocalFTSCandidates(inputs, roles, bounded.Candidates)
	if err != nil {
		return CandidateBatch{}, err
	}
	bounded.Truncated = bounded.Truncated || omitted > 0
	bounded.OmittedAtLeast = max(bounded.OmittedAtLeast, omitted)
	bounded.OmittedBasis = dedupeStrings(append(bounded.OmittedBasis, omittedBasis...))
	return bounded, nil
}

func (index SQLiteQueryIndex) NavigationCandidatesForPatterns(patternIDs []string) (CandidateBatch, error) {
	patternIDs = normalizeNonEmptyStrings(patternIDs)
	if len(patternIDs) == 0 {
		return CandidateBatch{}, nil
	}
	placeholders := make([]string, 0, len(patternIDs))
	patternArgs := make([]any, 0, len(patternIDs))
	for _, patternID := range patternIDs {
		placeholders = append(placeholders, "?")
		patternArgs = append(patternArgs, patternID)
	}
	inClause := strings.Join(placeholders, ",")
	query := sourceUnitSelectAliased("u") + `
		FROM source_units u
		WHERE (
			u.source_role = ? AND u.pattern_id IN (` + inClause + `)
		) OR (
			u.source_role = ? AND EXISTS (
				SELECT 1 FROM source_unit_refs refs
				WHERE refs.unit_id = u.unit_id AND refs.ref_id IN (` + inClause + `)
			)
		)
		ORDER BY CASE u.source_role WHEN ? THEN 0 ELSE 1 END, u.start_line, u.unit_id`
	args := []any{SourceUnitRoleTOCRow}
	args = append(args, patternArgs...)
	args = append(args, SourceUnitRolePracticalUseCard)
	args = append(args, patternArgs...)
	args = append(args, SourceUnitRolePracticalUseCard)
	units, err := loadSourceUnits(index.db, query, args)
	if err != nil {
		return CandidateBatch{}, err
	}
	candidates := make([]RetrievedCandidate, 0, len(units))
	for _, unit := range units {
		candidates = append(candidates, RetrievedCandidate{Unit: unit})
	}
	return CandidateBatch{Candidates: candidates}, nil
}

func (index SQLiteQueryIndex) RelationProjectionsForPatterns(patternIDs []string) (map[string]CandidateRelationProjection, error) {
	patternIDs = normalizeNonEmptyStrings(patternIDs)
	projections := make(map[string]CandidateRelationProjection)
	if len(patternIDs) == 0 {
		return projections, nil
	}

	placeholders := make([]string, 0, len(patternIDs))
	args := make([]any, 0, len(patternIDs))
	for _, patternID := range patternIDs {
		placeholders = append(placeholders, "?")
		args = append(args, patternID)
	}
	query := sourceUnitSelect + `
		WHERE pattern_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY pattern_id, source_role, unit_id`
	units, err := loadSourceUnits(index.db, query, args)
	if err != nil {
		return nil, err
	}
	for _, unit := range units {
		if len(unit.Relations) == 0 {
			continue
		}
		if _, exists := projections[unit.PatternID]; exists {
			return nil, fmt.Errorf("pattern %s has multiple canonical relation owners", unit.PatternID)
		}
		projections[unit.PatternID] = CandidateRelationProjection{
			SubjectPatternID: unit.PatternID,
			CanonicalUnitID:  unit.UnitID,
			Relations:        cloneSourceRelations(unit.Relations),
		}
	}
	return projections, nil
}

func (index SQLiteQueryIndex) SourceLexemeFrequencies(lexemes []string, roles []SourceUnitRole) (SourceLexemeFrequencies, error) {
	lexemes = normalizeNonEmptyStrings(lexemes)
	where, roleArgs := sourceRoleWhere("source_role", roles)
	var total int
	if err := index.db.QueryRow(`SELECT COUNT(*) FROM source_units WHERE `+where, roleArgs...).Scan(&total); err != nil {
		return SourceLexemeFrequencies{}, fmt.Errorf("count source lexeme corpus: %w", err)
	}

	frequencies := SourceLexemeFrequencies{
		TotalSourceUnits:  total,
		DocumentFrequency: make(map[string]int, len(lexemes)),
	}
	for _, lexeme := range lexemes {
		expression := sourceFTSExpression(lexeme)
		if expression == "" {
			continue
		}
		query := `SELECT COUNT(*) FROM source_units_fts WHERE ` + where + `
			AND source_units_fts MATCH ?`
		args := append(append([]any(nil), roleArgs...), expression)
		var count int
		if err := index.db.QueryRow(query, args...).Scan(&count); err != nil {
			return SourceLexemeFrequencies{}, fmt.Errorf("count source lexeme %q documents: %w", lexeme, err)
		}
		frequencies.DocumentFrequency[lexeme] = count
	}
	return frequencies, nil
}

// countOmittedRoleLocalFTSCandidates counts the union of source units matched
// by all probe fields within each role. Counting after candidate deduplication
// avoids reporting the same omitted unit once per overlapping probe field.
func (index SQLiteQueryIndex) countOmittedRoleLocalFTSCandidates(
	inputs []sourceProbeInput,
	roles []SourceUnitRole,
	candidates []RetrievedCandidate,
) (int, []string, error) {
	expression := combinedSourceFTSExpression(inputs)
	if expression == "" {
		return 0, nil, nil
	}

	includedByRole := make(map[SourceUnitRole]int)
	for _, candidate := range candidates {
		includedByRole[candidate.Unit.Role]++
	}

	omitted := 0
	basis := make([]string, 0)
	for _, role := range roles {
		var matched int
		err := index.db.QueryRow(`
			SELECT COUNT(*)
			FROM source_units_fts
			WHERE source_units_fts MATCH ? AND source_role = ?`,
			expression,
			role,
		).Scan(&matched)
		if err != nil {
			return 0, nil, fmt.Errorf("count role-local FTS candidates for %s: %w", role, err)
		}
		roleOmitted := max(0, matched-includedByRole[role])
		if roleOmitted == 0 {
			continue
		}
		omitted += roleOmitted
		basis = append(basis, "role_local_fts:"+string(role))
	}
	return omitted, basis, nil
}

func (index SQLiteQueryIndex) searchSourceFieldContains(column, value string, roles []SourceUnitRole) ([]SourceUnit, error) {
	if column != "title" && column != "source_id" {
		return nil, fmt.Errorf("unsupported source search column %q", column)
	}
	where, args := sourceRoleWhere("source_role", roles)
	query := sourceUnitSelect + ` WHERE ` + where + ` AND instr(lower(` + column + `), lower(?)) > 0 ORDER BY start_line, unit_id LIMIT ?`
	args = append(args, value, sourceCandidateProducerLimit+1)
	return loadSourceUnits(index.db, query, args)
}

func (index SQLiteQueryIndex) searchKeywordContains(value string, roles []SourceUnitRole) ([]matchedSourceUnit, error) {
	where, roleArgs := sourceRoleWhere("u.source_role", roles)
	query := sourceUnitSelectAliased("u") + `, keywords.keyword
		FROM source_units u
		JOIN source_keywords keywords ON keywords.unit_id = u.unit_id
		WHERE ` + where + ` AND (
			lower(keywords.keyword) = lower(?) OR
			instr(lower(keywords.keyword), lower(?)) > 0 OR
			instr(lower(?), lower(keywords.keyword)) > 0
		)
		ORDER BY u.start_line, u.unit_id
		LIMIT ?`
	args := slices.Clone(roleArgs)
	args = append(args, value, value, value, sourceCandidateProducerLimit+1)
	return loadMatchedSourceUnits(index.db, query, args)
}

type sourceProbeInput struct {
	Field string
	Value string
}

func sourceProbeInputs(probe CandidateProbe) []sourceProbeInput {
	inputs := []sourceProbeInput{
		{Field: "text", Value: probe.Text},
		{Field: "entity_of_concern", Value: probe.EntityOfConcern},
	}
	for _, context := range probe.KnownContext {
		inputs = append(inputs, sourceProbeInput{Field: "known_context", Value: context})
	}
	inputs = append(inputs, sourceProbeInput{Field: "intended_use", Value: probe.IntendedUse})

	filtered := make([]sourceProbeInput, 0, len(inputs))
	seen := make(map[string]struct{})
	for _, input := range inputs {
		input.Value = strings.TrimSpace(input.Value)
		key := input.Field + "\x00" + strings.ToLower(input.Value)
		if input.Value == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, input)
	}
	return filtered
}

func sourceSearchTokens(value string) []string {
	matches := sourceSearchTokenRE.FindAllString(strings.ToLower(value), -1)
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		if len([]rune(match)) >= 2 {
			tokens = append(tokens, match)
		}
	}
	return dedupeStrings(tokens)
}

func sourceGroundingSearchTokens(value string) []string {
	tokens := make([]string, 0)
	for _, token := range sourceSearchTokens(value) {
		if _, scaffold := genericQuestionScaffoldLexemes[token]; scaffold {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func sourceFTSExpression(value string) string {
	tokens := sourceSearchTokens(value)
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func sourceRoleLocalFTSExpression(value string) string {
	tokens := sourceGroundingSearchTokens(value)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	expression := strings.Join(quoted, " OR ")
	return `{title body} : (` + expression + `)`
}

func sourceRoleLocalExactPhraseExpression(value string) string {
	tokens := sourceGroundingSearchTokens(value)
	if len(dedupeStrings(tokens)) < minimumDistinctSourceGroundedLexemes {
		return ""
	}
	phrase := strings.Join(tokens, " ")
	phrase = strings.ReplaceAll(phrase, `"`, `""`)
	return `{title body} : "` + phrase + `"`
}

func combinedSourceFTSExpression(inputs []sourceProbeInput) string {
	expressions := make([]string, 0, len(inputs))
	seen := make(map[string]struct{})
	for _, input := range inputs {
		expression := sourceRoleLocalFTSExpression(input.Value)
		if expression == "" {
			continue
		}
		if _, exists := seen[expression]; exists {
			continue
		}
		seen[expression] = struct{}{}
		expressions = append(expressions, "("+expression+")")
	}
	return strings.Join(expressions, " OR ")
}

func sourceRoleWhere(column string, roles []SourceUnitRole) (string, []any) {
	placeholders := make([]string, 0, len(roles))
	args := make([]any, 0, len(roles))
	for _, role := range roles {
		placeholders = append(placeholders, "?")
		args = append(args, role)
	}
	return column + ` IN (` + strings.Join(placeholders, ",") + `)`, args
}

func newRetrievedCandidate(unit SourceUnit, tier RetrievalTier, probeField, sourceField, matchedValue string) RetrievedCandidate {
	return RetrievedCandidate{
		Unit: unit,
		MatchGrounds: []MatchGround{{
			Tier:         tier,
			ProbeField:   probeField,
			SourceField:  sourceField,
			MatchedValue: matchedValue,
		}},
	}
}

func appendOrMergeRetrieved(candidates []RetrievedCandidate, candidate RetrievedCandidate) []RetrievedCandidate {
	for index := range candidates {
		if candidates[index].Unit.UnitID != candidate.Unit.UnitID {
			continue
		}
		candidates[index].MatchGrounds = mergeMatchGrounds(
			candidates[index].MatchGrounds,
			candidate.MatchGrounds,
		)
		return candidates
	}
	return append(candidates, candidate)
}

func boundCandidateBatch(candidates []RetrievedCandidate, basis string) CandidateBatch {
	if len(candidates) <= sourceCandidateProducerLimit {
		return CandidateBatch{Candidates: candidates}
	}
	return CandidateBatch{
		Candidates:     append([]RetrievedCandidate(nil), candidates[:sourceCandidateProducerLimit]...),
		Truncated:      true,
		OmittedAtLeast: len(candidates) - sourceCandidateProducerLimit,
		OmittedBasis:   []string{basis},
	}
}

const sourceUnitColumns = `
	unit_id, source_id, source_role, title, body, pattern_id,
	parent_pattern_id, publication_status, direct_refs_json,
	relation_count, relations_digest,
	authored_phrases_json, keywords_json, use_cues_json,
	source_path, start_line, end_line, content_hash, source_revision`

const sourceUnitSelect = `SELECT ` + sourceUnitColumns + ` FROM source_units`

func sourceUnitSelectAliased(alias string) string {
	columns := strings.Split(strings.TrimSpace(sourceUnitColumns), ",")
	qualified := make([]string, 0, len(columns))
	for _, column := range columns {
		qualified = append(qualified, alias+"."+strings.TrimSpace(column))
	}
	return `SELECT ` + strings.Join(qualified, ", ")
}

type sourceUnitRow struct {
	unit            SourceUnit
	directRefsJSON  string
	relationCount   int
	relationsDigest string
	phrasesJSON     string
	keywordsJSON    string
	useCuesJSON     string
}

func (row *sourceUnitRow) scanTargets() []any {
	return []any{
		&row.unit.UnitID,
		&row.unit.SourceID,
		&row.unit.Role,
		&row.unit.Title,
		&row.unit.Body,
		&row.unit.PatternID,
		&row.unit.ParentPatternID,
		&row.unit.PublicationStatus,
		&row.directRefsJSON,
		&row.relationCount,
		&row.relationsDigest,
		&row.phrasesJSON,
		&row.keywordsJSON,
		&row.useCuesJSON,
		&row.unit.Provenance.SourcePath,
		&row.unit.Provenance.StartLine,
		&row.unit.Provenance.EndLine,
		&row.unit.Provenance.ContentHash,
		&row.unit.Provenance.SourceRevision,
	}
}

func (row *sourceUnitRow) decode() (SourceUnit, error) {
	if err := decodeSourceJSON(row.directRefsJSON, &row.unit.DirectRefs); err != nil {
		return SourceUnit{}, err
	}
	if err := decodeSourceJSON(row.phrasesJSON, &row.unit.AuthoredPhrases); err != nil {
		return SourceUnit{}, err
	}
	if err := decodeSourceJSON(row.keywordsJSON, &row.unit.Keywords); err != nil {
		return SourceUnit{}, err
	}
	if err := decodeSourceJSON(row.useCuesJSON, &row.unit.UseCues); err != nil {
		return SourceUnit{}, err
	}
	return row.unit, nil
}

func loadSourceUnits(db *sql.DB, query string, args []any) ([]SourceUnit, error) {
	if query == "" {
		query = sourceUnitSelect + ` ORDER BY source_role, start_line, unit_id`
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query source units: %w", err)
	}
	defer func() { _ = rows.Close() }()

	units := make([]SourceUnit, 0)
	for rows.Next() {
		var row sourceUnitRow
		if err := rows.Scan(row.scanTargets()...); err != nil {
			return nil, fmt.Errorf("scan source unit: %w", err)
		}
		unit, err := row.decode()
		if err != nil {
			return nil, fmt.Errorf("decode source unit %s: %w", row.unit.UnitID, err)
		}
		units = append(units, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close source unit rows: %w", err)
	}
	if err := hydrateSourceRelations(db, units); err != nil {
		return nil, err
	}
	return units, nil
}

const sourceRelationLoadChunkSize = 500

func hydrateSourceRelations(db *sql.DB, units []SourceUnit) error {
	if len(units) == 0 {
		return nil
	}

	unitIndexByID := make(map[string]int, len(units))
	for index := range units {
		units[index].Relations = nil
		unitIndexByID[units[index].UnitID] = index
	}

	for start := 0; start < len(units); start += sourceRelationLoadChunkSize {
		end := min(start+sourceRelationLoadChunkSize, len(units))
		placeholders := make([]string, 0, end-start)
		arguments := make([]any, 0, end-start)
		for _, unit := range units[start:end] {
			placeholders = append(placeholders, "?")
			arguments = append(arguments, unit.UnitID)
		}
		// #nosec G202 -- the only interpolated fragment is a count-derived list of SQL placeholders; unit IDs remain bound parameters.
		query := `
			SELECT unit_id, relation_ordinal, relation_kind, target_pattern_id,
				target_class, origin, source_path, start_line, end_line,
				content_hash, source_revision
			FROM source_unit_relations
			WHERE unit_id IN (` + strings.Join(placeholders, ",") + `)
			ORDER BY unit_id, relation_ordinal`
		rows, err := db.Query(query, arguments...)
		if err != nil {
			return fmt.Errorf("query source relations: %w", err)
		}
		expectedOrdinalByUnit := make(map[string]int)
		for rows.Next() {
			var unitID string
			var ordinal int
			var relation SourceRelation
			err := rows.Scan(
				&unitID,
				&ordinal,
				&relation.Kind,
				&relation.TargetPatternID,
				&relation.TargetClass,
				&relation.Origin,
				&relation.Provenance.SourcePath,
				&relation.Provenance.StartLine,
				&relation.Provenance.EndLine,
				&relation.Provenance.ContentHash,
				&relation.Provenance.SourceRevision,
			)
			if err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan source relation: %w", err)
			}
			expectedOrdinal := expectedOrdinalByUnit[unitID]
			if ordinal != expectedOrdinal {
				_ = rows.Close()
				return fmt.Errorf(
					"source relation ordinals for %s are not contiguous: got %d, want %d",
					unitID,
					ordinal,
					expectedOrdinal,
				)
			}
			unitIndex, exists := unitIndexByID[unitID]
			if !exists {
				_ = rows.Close()
				return fmt.Errorf("source relation references unloaded unit %s", unitID)
			}
			units[unitIndex].Relations = append(units[unitIndex].Relations, relation)
			expectedOrdinalByUnit[unitID] = expectedOrdinal + 1
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("iterate source relations: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close source relation rows: %w", err)
		}
	}
	return nil
}

func verifyStoredSourceRelations(db *sql.DB, units []SourceUnit) error {
	unitByID := make(map[string]SourceUnit, len(units))
	for _, unit := range units {
		unitByID[unit.UnitID] = unit
	}

	rows, err := db.Query(`
		SELECT unit_id, relation_count, relations_digest
		FROM source_units
		ORDER BY unit_id`)
	if err != nil {
		return fmt.Errorf("query source relation integrity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	metadataRows := 0
	expectedTotal := 0
	for rows.Next() {
		var unitID string
		var expectedCount int
		var expectedDigest string
		if err := rows.Scan(&unitID, &expectedCount, &expectedDigest); err != nil {
			return fmt.Errorf("scan source relation integrity: %w", err)
		}
		unit, exists := unitByID[unitID]
		if !exists {
			return fmt.Errorf("source relation integrity references unloaded unit %s", unitID)
		}
		actualCount := len(unit.Relations)
		if actualCount != expectedCount {
			return fmt.Errorf(
				"source relation count mismatch for %s: stored %d, loaded %d",
				unitID,
				expectedCount,
				actualCount,
			)
		}
		actualDigest, err := sourceRelationsDigest(unit.Relations)
		if err != nil {
			return fmt.Errorf("encode loaded source relations for %s: %w", unitID, err)
		}
		if actualDigest != expectedDigest {
			return fmt.Errorf("source relation integrity mismatch for %s", unitID)
		}
		metadataRows++
		expectedTotal += expectedCount
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate source relation integrity: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close source relation integrity rows: %w", err)
	}
	if metadataRows != len(units) {
		return fmt.Errorf(
			"source relation integrity row mismatch: %d source units but %d integrity rows",
			len(units),
			metadataRows,
		)
	}

	var actualTotal int
	if err := db.QueryRow(`SELECT COUNT(*) FROM source_unit_relations`).Scan(&actualTotal); err != nil {
		return fmt.Errorf("count source relation rows: %w", err)
	}
	if actualTotal != expectedTotal {
		return fmt.Errorf(
			"source relation row count mismatch: expected %d, stored %d",
			expectedTotal,
			actualTotal,
		)
	}

	var orphanUnitID string
	err = db.QueryRow(`
		SELECT relations.unit_id
		FROM source_unit_relations relations
		LEFT JOIN source_units units ON units.unit_id = relations.unit_id
		WHERE units.unit_id IS NULL
		LIMIT 1`).Scan(&orphanUnitID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("verify source relation foreign keys: %w", err)
	}
	if err == nil {
		return fmt.Errorf("source relation references missing unit %s", orphanUnitID)
	}
	return nil
}

func sourceRelationsDigest(relations []SourceRelation) (string, error) {
	normalized := append([]SourceRelation{}, relations...)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return sourceContentHash(string(encoded)), nil
}

type matchedSourceUnit struct {
	Unit    SourceUnit
	Matched string
}

func loadMatchedSourceUnits(db *sql.DB, query string, args []any) ([]matchedSourceUnit, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query matched source units: %w", err)
	}
	defer func() { _ = rows.Close() }()

	matched := make([]matchedSourceUnit, 0)
	for rows.Next() {
		var row sourceUnitRow
		var value string
		targets := append(row.scanTargets(), &value)
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("scan matched source unit: %w", err)
		}
		unit, err := row.decode()
		if err != nil {
			return nil, err
		}
		matched = append(matched, matchedSourceUnit{Unit: unit, Matched: value})
	}
	return matched, rows.Err()
}

func isExactSourceColumn(column string) bool {
	return column == "unit_id" || column == "source_id" || column == "pattern_id" || column == "title"
}

func verifyStoredTOCBodyCompleteness(units []SourceUnit) error {
	bodies := make(map[string]struct{})
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternBody {
			bodies[unit.PatternID] = struct{}{}
		}
	}
	for _, unit := range units {
		if unit.Role != SourceUnitRoleTOCRow || unit.PatternID == "" || strings.EqualFold(unit.PublicationStatus, "planned") {
			continue
		}
		if _, exists := bodies[unit.PatternID]; !exists {
			return fmt.Errorf("source query grammar: non-planned ToC pattern %s has no pattern_body", unit.PatternID)
		}
	}
	return nil
}

func encodeSourceJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeSourceJSON(encoded string, target any) error {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	return json.Unmarshal([]byte(encoded), target)
}

func removeSQLiteBuildFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open source query index directory: %w", err)
	}
	defer func() { _ = directory.Close() }()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync source query index directory: %w", err)
	}
	return nil
}
