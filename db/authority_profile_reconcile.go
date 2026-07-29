package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"hash"
	"reflect"
	"slices"
	"strings"
)

const legacyV34AuthorityProfileFingerprint = "sha256:14a6152303cfe7593a8d386c1635efeb3eab19fb5b7d1ca9bcb4343b9829472b"

var legacyV34AuthorityProfileTables = []string{
	"authority_presentations",
	"authority_resolution_records",
	"authority_uses",
	"profile_onboarding_work_records",
	"project_profile_admissions",
	"project_profile_revisions",
	"project_profile_projection_debt",
}

var legacyV34AuthorityProfileDropOrder = []string{
	"project_profile_projection_debt",
	"project_profile_revisions",
	"authority_uses",
	"project_profile_admissions",
	"authority_resolution_records",
	"authority_presentations",
	"profile_onboarding_work_records",
}

var reconciledRequiredTables = []string{
	"authority_presentations",
	"authority_resolution_records",
	"authority_uses",
	"profile_onboarding_work_records",
	"project_profile_admissions",
	"project_profile_revisions",
	"project_profile_projection_debt",
	"migration_review_speech_acts",
	"migration_review_admissions",
}

var reconciledRequiredTriggers = []string{
	"authority_presentations_no_replace",
	"authority_presentations_exact_assignment_method",
	"authority_presentations_no_update",
	"authority_presentations_no_delete",
	"authority_resolution_records_no_replace",
	"authority_resolution_records_exact_presentation",
	"authority_resolution_records_no_update",
	"authority_resolution_records_no_delete",
	"authority_uses_no_replace",
	"authority_uses_exact_tuple",
	"authority_uses_no_update",
	"authority_uses_no_delete",
	"profile_onboarding_work_records_no_replace",
	"profile_onboarding_work_records_no_update",
	"profile_onboarding_work_records_no_delete",
	"project_profile_admissions_no_replace",
	"project_profile_admissions_revision_cas",
	"project_profile_admissions_exact_authority",
	"project_profile_admissions_no_update",
	"project_profile_admissions_no_delete",
	"project_profile_revisions_no_replace",
	"project_profile_revisions_exact_admission",
	"project_profile_revisions_no_update",
	"project_profile_revisions_no_delete",
	"project_profile_projection_debt_no_replace",
	"project_profile_projection_debt_no_update",
	"project_profile_projection_debt_no_delete",
}

type schemaQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type authorityProfileSchemaContract struct {
	fingerprint                string
	migrationReviewFingerprint string
	columns                    map[string][]string
}

func reconcileAuthorityProfileSchema(tx MigrationTransaction, migrations []Migration) error {
	contract, err := loadCanonicalAuthorityProfileContract(migrations)
	if err != nil {
		return err
	}
	if err := requireCanonicalMigrationReviewSchema(tx, contract); err != nil {
		return err
	}
	actualFingerprint, err := authorityProfileSchemaFingerprint(tx)
	if err != nil {
		return fmt.Errorf("fingerprint installed authority/profile schema: %w", err)
	}
	if actualFingerprint == contract.fingerprint {
		return verifyReconciledAuthorityProfileSchema(tx, contract)
	}
	if actualFingerprint != legacyV34AuthorityProfileFingerprint {
		return fmt.Errorf(
			"authority/profile schema reconciliation refused: fingerprint %s is neither canonical nor the exact known legacy-v34 schema; manual migration required",
			actualFingerprint,
		)
	}
	if err := requireEmptyLegacyV34AuthorityProfileTables(tx, 0); err != nil {
		return err
	}
	if err := replaceLegacyV34AuthorityProfileSchema(tx, migrations); err != nil {
		return err
	}
	return verifyReconciledAuthorityProfileSchema(tx, contract)
}

func loadCanonicalAuthorityProfileContract(migrations []Migration) (authorityProfileSchemaContract, error) {
	return buildCanonicalAuthorityProfileContract(migrations)
}

func buildCanonicalAuthorityProfileContract(migrations []Migration) (authorityProfileSchemaContract, error) {
	return buildAuthorityProfileSchemaContract(migrations, []int{34, 35})
}

func buildAuthorityProfileSchemaContract(
	migrations []Migration,
	versions []int,
) (authorityProfileSchemaContract, error) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return authorityProfileSchemaContract{}, fmt.Errorf("open canonical authority/profile schema fixture: %w", err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	statements, err := canonicalMigrationStatements(migrations, versions, 0, nil)
	if err != nil {
		return authorityProfileSchemaContract{}, err
	}
	if err := executeStatements(database, statements, 0); err != nil {
		return authorityProfileSchemaContract{}, fmt.Errorf("build canonical authority/profile schema fixture: %w", err)
	}
	fingerprint, err := authorityProfileSchemaFingerprint(database)
	if err != nil {
		return authorityProfileSchemaContract{}, fmt.Errorf("fingerprint canonical authority/profile schema fixture: %w", err)
	}
	migrationReviewFingerprint, err := migrationReviewSchemaFingerprint(database)
	if err != nil {
		return authorityProfileSchemaContract{}, fmt.Errorf("fingerprint canonical migration-review schema fixture: %w", err)
	}
	columns, err := loadRequiredColumns(database, reconciledRequiredTables, 0, map[string][]string{})
	if err != nil {
		return authorityProfileSchemaContract{}, fmt.Errorf("load canonical authority/profile columns: %w", err)
	}
	return authorityProfileSchemaContract{
		fingerprint:                fingerprint,
		migrationReviewFingerprint: migrationReviewFingerprint,
		columns:                    columns,
	}, nil
}

func canonicalMigrationStatements(
	migrations []Migration,
	versions []int,
	index int,
	accumulator []string,
) ([]string, error) {
	if index >= len(versions) {
		return accumulator, nil
	}
	migration, ok := findMigration(migrations, versions[index], 0)
	if !ok {
		return nil, fmt.Errorf("canonical migration %d is unavailable", versions[index])
	}
	if migration.Apply != nil {
		return nil, fmt.Errorf("canonical migration %d cannot be replayed as statements", migration.Version)
	}
	next := slices.Clone(accumulator)
	next = append(next, migration.Statements...)
	return canonicalMigrationStatements(migrations, versions, index+1, next)
}

func findMigration(migrations []Migration, version int, index int) (Migration, bool) {
	if index >= len(migrations) {
		return Migration{}, false
	}
	if migrations[index].Version == version {
		return migrations[index], true
	}
	return findMigration(migrations, version, index+1)
}

type statementExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func executeStatements(executor statementExecutor, statements []string, index int) error {
	if index >= len(statements) {
		return nil
	}
	statement := strings.TrimSpace(statements[index])
	if statement == "" {
		return executeStatements(executor, statements, index+1)
	}
	if _, err := executor.Exec(statement); err != nil {
		return fmt.Errorf("execute canonical statement %d: %w", index+1, err)
	}
	return executeStatements(executor, statements, index+1)
}

func authorityProfileSchemaFingerprint(queryer schemaQueryer) (string, error) {
	rows, err := queryer.Query(`
		SELECT type, name, tbl_name, sql
		FROM sqlite_master
		WHERE sql IS NOT NULL
		AND (
			name = 'current_project_profiles'
			OR tbl_name = 'current_project_profiles'
			OR name = 'observed_project_bases'
			OR tbl_name = 'observed_project_bases'
			OR name GLOB 'authority_*'
			OR tbl_name GLOB 'authority_*'
			OR name GLOB 'profile_*'
			OR tbl_name GLOB 'profile_*'
			OR name GLOB 'project_profile_*'
			OR tbl_name GLOB 'project_profile_*'
		)
		AND name NOT GLOB 'authority_basis_*'
		AND tbl_name NOT GLOB 'authority_basis_*'
		AND name NOT GLOB 'profile_declaration_*'
		AND tbl_name NOT GLOB 'profile_declaration_*'
		AND name NOT IN (
			'authority_presentations_require_v38_basis',
			'authority_resolution_records_require_v38_basis'
		)
		ORDER BY type, name, tbl_name`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	digest := sha256.New()
	if err := appendSchemaFingerprintRows(rows, digest); err != nil {
		return "", err
	}
	sum := digest.Sum(nil)
	encoded := hex.EncodeToString(sum)
	return "sha256:" + encoded, nil
}

func migrationReviewSchemaFingerprint(queryer schemaQueryer) (string, error) {
	rows, err := queryer.Query(`
		SELECT type, name, tbl_name, sql
		FROM sqlite_master
		WHERE sql IS NOT NULL
		AND type IN ('table', 'index', 'trigger')
		AND (
			name GLOB 'migration_review_*'
			OR tbl_name GLOB 'migration_review_*'
		)
		AND name NOT GLOB 'migration_review_acceptance_*'
		AND tbl_name NOT GLOB 'migration_review_acceptance_*'
		AND name NOT GLOB 'migration_review_admissions_v2*'
		AND tbl_name NOT GLOB 'migration_review_admissions_v2*'
		AND name NOT GLOB 'migration_review_instituted_*'
		AND tbl_name NOT GLOB 'migration_review_instituted_*'
		ORDER BY type, name, tbl_name`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	digest := sha256.New()
	if err := appendExactSchemaFingerprintRows(rows, digest); err != nil {
		return "", err
	}
	sum := digest.Sum(nil)
	encoded := hex.EncodeToString(sum)
	return "sha256:" + encoded, nil
}

func appendExactSchemaFingerprintRows(rows *sql.Rows, digest hash.Hash) error {
	if !rows.Next() {
		return rows.Err()
	}
	var objectType string
	var name string
	var tableName string
	var statement string
	if err := rows.Scan(&objectType, &name, &tableName, &statement); err != nil {
		return err
	}
	_, _ = digest.Write([]byte(objectType))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(name))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(tableName))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(statement))
	_, _ = digest.Write([]byte{0})
	return appendExactSchemaFingerprintRows(rows, digest)
}

func requireCanonicalMigrationReviewSchema(
	queryer schemaQueryer,
	contract authorityProfileSchemaContract,
) error {
	actual, err := migrationReviewSchemaFingerprint(queryer)
	if err != nil {
		return fmt.Errorf("fingerprint installed migration-review schema: %w", err)
	}
	if actual != contract.migrationReviewFingerprint {
		return fmt.Errorf(
			"migration-review schema reconciliation refused: fingerprint %s does not match canonical migration 35 fingerprint %s; manual migration required",
			actual,
			contract.migrationReviewFingerprint,
		)
	}
	return nil
}

func appendSchemaFingerprintRows(rows *sql.Rows, digest hash.Hash) error {
	if !rows.Next() {
		return rows.Err()
	}
	var objectType string
	var name string
	var tableName string
	var statement string
	if err := rows.Scan(&objectType, &name, &tableName, &statement); err != nil {
		return err
	}
	canonicalStatement := strings.Join(strings.Fields(statement), " ")
	_, _ = digest.Write([]byte(objectType))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(name))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(tableName))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(canonicalStatement))
	_, _ = digest.Write([]byte{0})
	return appendSchemaFingerprintRows(rows, digest)
}

func requireEmptyLegacyV34AuthorityProfileTables(tx MigrationTransaction, index int) error {
	if index >= len(legacyV34AuthorityProfileTables) {
		return nil
	}
	table := legacyV34AuthorityProfileTables[index]
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)
	var count int64
	if err := tx.QueryRow(query).Scan(&count); err != nil {
		return fmt.Errorf("inspect known legacy-v34 table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"authority/profile schema reconciliation refused: known legacy-v34 table %s contains %d row(s); automatic replacement is allowed only for an entirely empty subsystem; manual migration required",
			table,
			count,
		)
	}
	return requireEmptyLegacyV34AuthorityProfileTables(tx, index+1)
}

func replaceLegacyV34AuthorityProfileSchema(tx MigrationTransaction, migrations []Migration) error {
	if _, err := tx.Exec("DROP VIEW current_project_profiles"); err != nil {
		return fmt.Errorf("drop known legacy-v34 current-profile view: %w", err)
	}
	if err := dropLegacyV34AuthorityProfileTables(tx, 0); err != nil {
		return err
	}
	statements, err := canonicalMigrationStatements(migrations, []int{34}, 0, nil)
	if err != nil {
		return err
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install canonical authority/profile schema: %w", err)
	}
	return nil
}

func dropLegacyV34AuthorityProfileTables(tx MigrationTransaction, index int) error {
	if index >= len(legacyV34AuthorityProfileDropOrder) {
		return nil
	}
	table := legacyV34AuthorityProfileDropOrder[index]
	statement := "DROP TABLE " + quoteSQLiteIdentifier(table)
	if _, err := tx.Exec(statement); err != nil {
		return fmt.Errorf("drop known empty legacy-v34 table %s: %w", table, err)
	}
	return dropLegacyV34AuthorityProfileTables(tx, index+1)
}

func verifyReconciledAuthorityProfileSchema(
	tx MigrationTransaction,
	contract authorityProfileSchemaContract,
) error {
	fingerprint, err := authorityProfileSchemaFingerprint(tx)
	if err != nil {
		return fmt.Errorf("verify authority/profile schema fingerprint: %w", err)
	}
	if fingerprint != contract.fingerprint {
		return fmt.Errorf(
			"verify authority/profile schema fingerprint: got %s, want %s",
			fingerprint,
			contract.fingerprint,
		)
	}
	if err := verifyRequiredTableCount(tx); err != nil {
		return err
	}
	if err := verifyRequiredColumns(tx, contract.columns, reconciledRequiredTables, 0); err != nil {
		return err
	}
	if err := verifyRequiredTriggerCount(tx); err != nil {
		return err
	}
	return verifyForeignKeys(tx)
}

func verifyRequiredTableCount(tx MigrationTransaction) error {
	query, args := sqliteMasterCountQuery("table", reconciledRequiredTables)
	var count int
	if err := tx.QueryRow(query, args...).Scan(&count); err != nil {
		return fmt.Errorf("verify reconciled authority/profile tables: %w", err)
	}
	if count != 9 {
		return fmt.Errorf("verify reconciled authority/profile tables: found %d of 9 required tables", count)
	}
	return nil
}

func verifyRequiredTriggerCount(tx MigrationTransaction) error {
	query, args := sqliteMasterCountQuery("trigger", reconciledRequiredTriggers)
	var count int
	if err := tx.QueryRow(query, args...).Scan(&count); err != nil {
		return fmt.Errorf("verify reconciled authority/profile triggers: %w", err)
	}
	if count != 27 {
		return fmt.Errorf("verify reconciled authority/profile triggers: found %d of 27 required triggers", count)
	}
	return nil
}

func sqliteMasterCountQuery(objectType string, names []string) (string, []any) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name IN (" + placeholders + ")"
	args := appendStringArguments([]any{objectType}, names, 0)
	return query, args
}

func appendStringArguments(arguments []any, values []string, index int) []any {
	if index >= len(values) {
		return arguments
	}
	next := slices.Clone(arguments)
	next = append(next, values[index])
	return appendStringArguments(next, values, index+1)
}

func loadRequiredColumns(
	queryer schemaQueryer,
	tables []string,
	index int,
	accumulator map[string][]string,
) (map[string][]string, error) {
	if index >= len(tables) {
		return accumulator, nil
	}
	table := tables[index]
	columns, err := loadTableColumns(queryer, table)
	if err != nil {
		return nil, err
	}
	accumulator[table] = columns
	return loadRequiredColumns(queryer, tables, index+1, accumulator)
}

func loadTableColumns(queryer schemaQueryer, table string) ([]string, error) {
	query := "PRAGMA table_xinfo(" + quoteSQLiteIdentifier(table) + ")"
	rows, err := queryer.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return appendTableColumns(rows, nil)
}

func appendTableColumns(rows *sql.Rows, accumulator []string) ([]string, error) {
	if !rows.Next() {
		return accumulator, rows.Err()
	}
	var cid int
	var name string
	var columnType string
	var notNull int
	var defaultValue any
	var primaryKey int
	var hidden int
	if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey, &hidden); err != nil {
		return nil, err
	}
	next := slices.Clone(accumulator)
	next = append(next, name)
	return appendTableColumns(rows, next)
}

func verifyRequiredColumns(
	queryer schemaQueryer,
	expected map[string][]string,
	tables []string,
	index int,
) error {
	if index >= len(tables) {
		return nil
	}
	table := tables[index]
	actual, err := loadTableColumns(queryer, table)
	if err != nil {
		return fmt.Errorf("verify reconciled table %s columns: %w", table, err)
	}
	if !reflect.DeepEqual(actual, expected[table]) {
		return fmt.Errorf(
			"verify reconciled table %s columns: got %v, want %v",
			table,
			actual,
			expected[table],
		)
	}
	return verifyRequiredColumns(queryer, expected, tables, index+1)
}

func verifyForeignKeys(tx MigrationTransaction) error {
	rows, err := tx.Query("PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("verify reconciled schema foreign keys: %w", err)
	}
	violations, err := loadForeignKeyViolations(rows, nil)
	closeErr := rows.Close()
	if err != nil {
		return fmt.Errorf("read reconciled schema foreign-key violation: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close reconciled schema foreign-key scan: %w", closeErr)
	}
	return verifyForeignKeyViolations(tx, violations, 0)
}

type foreignKeyViolation struct {
	table        string
	rowID        any
	parent       string
	foreignKeyID int
}

func loadForeignKeyViolations(
	rows *sql.Rows,
	accumulator []foreignKeyViolation,
) ([]foreignKeyViolation, error) {
	if !rows.Next() {
		return accumulator, rows.Err()
	}
	var violation foreignKeyViolation
	if err := rows.Scan(
		&violation.table,
		&violation.rowID,
		&violation.parent,
		&violation.foreignKeyID,
	); err != nil {
		return nil, err
	}
	return loadForeignKeyViolations(rows, append(accumulator, violation))
}

func verifyForeignKeyViolations(
	tx MigrationTransaction,
	violations []foreignKeyViolation,
	index int,
) error {
	if index >= len(violations) {
		return nil
	}
	violation := violations[index]
	admitted, err := isLegacyDecisionSpecSectionProjection(tx, violation)
	if err != nil {
		return fmt.Errorf("verify legacy DecisionRecord SpecSection projection: %w", err)
	}
	if admitted {
		return verifyForeignKeyViolations(tx, violations, index+1)
	}
	return fmt.Errorf(
		"verify reconciled schema foreign keys: table %s row %v violates parent %s foreign key %d",
		violation.table,
		violation.rowID,
		violation.parent,
		violation.foreignKeyID,
	)
}

// artifact_links predates typed cross-carrier graph relations. Its target
// foreign key therefore describes Artifact -> Artifact edges but not the
// historical DecisionRecord -> SpecSection `governs` relation. The canonical
// DecisionRecord section_refs field remains the typed relation when a section
// is retired or migrated out of the current ProjectSpecificationSet. Admit
// only the exact dual-carried legacy relation; every other violation remains
// fail-closed. New writes cannot create it while foreign keys are enabled.
func isLegacyDecisionSpecSectionProjection(
	tx MigrationTransaction,
	violation foreignKeyViolation,
) (bool, error) {
	if violation.table != "artifact_links" ||
		violation.parent != "artifacts" ||
		violation.foreignKeyID != 0 {
		return false, nil
	}
	var count int
	err := tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM artifact_links AS link
			JOIN artifacts AS source
				ON source.id = link.source_id
			JOIN json_each(json_extract(source.structured_data, '$.section_refs')) AS section_ref
				ON section_ref.value = link.target_id
				AND section_ref.type = 'text'
			WHERE link.rowid = ?
				AND link.link_type = 'governs'
				AND source.kind = 'DecisionRecord'
				AND json_valid(source.structured_data) = 1
				AND json_type(source.structured_data, '$.section_refs') = 'array'
		)`,
		violation.rowID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

func quoteSQLiteIdentifier(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}
