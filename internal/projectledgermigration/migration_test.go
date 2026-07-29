package projectledgermigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestApplyIsIdempotentAndHasNoHostCarrierEffects(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	hostPaths := writeHostSentinels(t, fixture.root)
	before := readFiles(t, hostPaths)
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	ledgerBefore := discoverHistoricalLedgerSnapshot(t, databasePath)

	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	first, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply first: %v", err)
	}
	second, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply second: %v", err)
	}

	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if first.Outcome != OutcomeAlreadyCurrent || second.Outcome != OutcomeAlreadyCurrent {
		t.Fatalf("outcomes = %s / %s, want already_current", first.Outcome, second.Outcome)
	}
	if first.BeforeSchema != current || first.AfterSchema != current {
		t.Fatalf("first schema = %d -> %d, want %d", first.BeforeSchema, first.AfterSchema, current)
	}
	if first.DatabasePath != databasePath {
		t.Fatalf("first database path = %s, want %s", first.DatabasePath, databasePath)
	}
	if second.BeforeSchema != current || second.AfterSchema != current {
		t.Fatalf("second schema = %d -> %d, want %d", second.BeforeSchema, second.AfterSchema, current)
	}
	if second.DatabasePath != databasePath {
		t.Fatalf("second database path = %s, want %s", second.DatabasePath, databasePath)
	}
	ledgerAfter := repeatHistoricalLedgerSnapshot(t, databasePath, ledgerBefore)
	assertHistoricalLedgerPreserved(t, ledgerBefore, ledgerAfter)
	assertFilesEqual(t, hostPaths, before)
}

func TestObserveReportsExactSchemaWithoutMutation(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	before := digestFileForTest(t, databasePath)

	observation, err := Observe(context.Background(), request)
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if observation.ProjectRoot != fixture.root ||
		observation.ProjectID != fixture.config.ID ||
		observation.DatabasePath != databasePath ||
		observation.ObservedSchema != current ||
		observation.CompiledSchema != current {
		t.Fatalf("observation = %#v", observation)
	}
	after := digestFileForTest(t, databasePath)
	if after != before {
		t.Fatal("read-only schema observation changed the project ledger")
	}
}

func TestApplyRejectsWrongProjectIdentityBeforeMigration(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, "qnt_ffffffff")
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply accepted a different expected project identity")
	}
}

func TestNewRequestRejectsWeakIdentityAndMissingRoot(t *testing.T) {
	if _, err := NewRequest("", "qnt_a7f3b2c1"); err == nil {
		t.Fatal("NewRequest accepted a missing project root")
	}
	if _, err := NewRequest(t.TempDir(), "project"); err == nil {
		t.Fatal("NewRequest accepted a weak project identity")
	}
}

func TestApplyExactRejectsStaleSchemaPlanBeforeMigration(t *testing.T) {
	fixture := newCurrentProjectFixture(t)
	request, err := NewRequest(fixture.root, fixture.config.ID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	transition, err := NewExactTransition(current-1, current)
	if err != nil {
		t.Fatalf("NewExactTransition: %v", err)
	}
	if _, err := ApplyExact(
		context.Background(),
		request,
		transition,
	); err == nil || !strings.Contains(err.Error(), "no migration was attempted") {
		t.Fatalf("ApplyExact stale-plan error = %v", err)
	}
	databasePath, err := fixture.config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if observed := readSchemaFrontierForTest(t, databasePath); observed != current {
		t.Fatalf("schema after stale exact plan = %d, want %d", observed, current)
	}
	if _, err := NewExactTransition(current, current); err == nil {
		t.Fatal("NewExactTransition accepted a no-op transition")
	}
}

func TestApplyMigratesConfiguredDetachedLedgerCopyWithoutHostEffects(t *testing.T) {
	sourceDB := strings.TrimSpace(os.Getenv("HAFT_I0_COPY_SOURCE_DB"))
	projectRoot := strings.TrimSpace(os.Getenv("HAFT_I0_PROJECT_ROOT"))
	projectID := strings.TrimSpace(os.Getenv("HAFT_I0_PROJECT_ID"))
	if sourceDB == "" || projectRoot == "" || projectID == "" {
		t.Skip("detached ledger copy input is not configured")
	}
	wal := sourceDB + "-wal"
	walInfo, err := os.Stat(wal)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("inspect source WAL: %v", err)
	}
	if err == nil && walInfo.Size() != 0 {
		t.Fatalf("source WAL contains %d bytes; checkpoint before copy rehearsal", walInfo.Size())
	}
	beforeLiveSchema := readSchemaFrontierForTest(t, sourceDB)
	if beforeLiveSchema != 53 {
		t.Fatalf("source schema = %d, want exact rehearsal predecessor 53", beforeLiveSchema)
	}
	sourceDigestBefore := digestFileForTest(t, sourceDB)
	hostBefore := snapshotHostCarriers(t, projectRoot)

	temporaryHome := canonicalTempDir(t)
	t.Setenv("HOME", temporaryHome)
	destinationDB := filepath.Join(
		temporaryHome,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	copyFileForTest(t, sourceDB, destinationDB)
	destinationDigestBefore := digestFileForTest(t, destinationDB)
	if destinationDigestBefore != sourceDigestBefore {
		t.Fatalf(
			"detached ledger digest = %s, want exact source digest %s",
			destinationDigestBefore,
			sourceDigestBefore,
		)
	}
	ledgerBefore := discoverHistoricalLedgerSnapshot(t, destinationDB)
	assertRequiredI0Frontiers(t, ledgerBefore)
	request, err := NewRequest(projectRoot, projectID)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	result, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply detached copy: %v", err)
	}
	if result.Outcome != OutcomeApplied ||
		result.BeforeSchema != 53 ||
		result.AfterSchema != 54 ||
		result.ProjectRoot != projectRoot ||
		result.ProjectID != projectID ||
		result.DatabasePath != destinationDB {
		t.Fatalf("detached result = %+v, want applied 53 -> 54", result)
	}
	ledgerAfterFirst := repeatHistoricalLedgerSnapshot(t, destinationDB, ledgerBefore)
	assertHistoricalLedgerPreserved(t, ledgerBefore, ledgerAfterFirst)
	currentBeforeRetry := discoverHistoricalLedgerSnapshot(t, destinationDB)
	second, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("Apply detached copy retry: %v", err)
	}
	if second.Outcome != OutcomeAlreadyCurrent ||
		second.BeforeSchema != 54 ||
		second.AfterSchema != 54 ||
		second.ProjectRoot != projectRoot ||
		second.ProjectID != projectID ||
		second.DatabasePath != destinationDB {
		t.Fatalf("detached retry result = %+v, want already_current 54 -> 54", second)
	}
	currentAfterRetry := repeatHistoricalLedgerSnapshot(
		t,
		destinationDB,
		currentBeforeRetry,
	)
	assertHistoricalLedgerPreserved(t, currentBeforeRetry, currentAfterRetry)
	if liveSchema := readSchemaFrontierForTest(t, sourceDB); liveSchema != beforeLiveSchema {
		t.Fatalf("live source schema changed from %d to %d", beforeLiveSchema, liveSchema)
	}
	sourceDigestAfter := digestFileForTest(t, sourceDB)
	if sourceDigestAfter != sourceDigestBefore {
		t.Fatalf(
			"live source digest changed from %s to %s",
			sourceDigestBefore,
			sourceDigestAfter,
		)
	}
	hostAfter := snapshotHostCarriers(t, projectRoot)
	if !reflect.DeepEqual(hostAfter, hostBefore) {
		t.Fatalf("project host carriers changed\nbefore: %v\nafter:  %v", hostBefore, hostAfter)
	}
	t.Logf(
		"preserved source %s, %d schema-53 tables, %d schema-54 tables, and %d foreign-key witnesses",
		sourceDigestBefore,
		len(ledgerBefore.tables),
		len(currentBeforeRetry.tables),
		len(ledgerBefore.foreignKeyWitnesses),
	)
}

type currentProjectFixture struct {
	root   string
	config *project.Config
}

func newCurrentProjectFixture(t *testing.T) currentProjectFixture {
	t.Helper()
	home := canonicalTempDir(t)
	root := canonicalTempDir(t)
	t.Setenv("HOME", home)
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	config, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close current store: %v", err)
	}
	boundAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	if err := projectledger.BindInitialized(context.Background(), root, boundAt); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	return currentProjectFixture{
		root:   root,
		config: config,
	}
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	return filepath.Clean(physical)
}

func writeHostSentinels(t *testing.T, root string) []string {
	t.Helper()
	paths := []string{
		filepath.Join(root, ".agents", "skills", "keep.txt"),
		filepath.Join(root, ".codex", "config.toml"),
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, "CLAUDE.md"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create host sentinel parent: %v", err)
		}
		if err := os.WriteFile(path, []byte("foreign-user-byte\n"), 0o644); err != nil {
			t.Fatalf("write host sentinel: %v", err)
		}
	}
	return paths
}

func readFiles(t *testing.T, paths []string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		result[path] = string(content)
	}
	return result
}

func assertFilesEqual(t *testing.T, paths []string, expected map[string]string) {
	t.Helper()
	observed := readFiles(t, paths)
	for _, path := range paths {
		if observed[path] != expected[path] {
			t.Fatalf("host carrier changed at %s", path)
		}
	}
}

func copyFileForTest(t *testing.T, source string, destination string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("create detached ledger directory: %v", err)
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatalf("open source ledger: %v", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create detached ledger: %v", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatalf("copy detached ledger: %v", err)
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		t.Fatalf("sync detached ledger: %v", err)
	}
	if err := output.Close(); err != nil {
		t.Fatalf("close detached ledger: %v", err)
	}
}

func readSchemaFrontierForTest(t *testing.T, path string) int {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open ledger read-only: %v", err)
	}
	defer database.Close()
	frontier := 0
	if err := database.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&frontier); err != nil {
		t.Fatalf("read schema frontier: %v", err)
	}
	return frontier
}

type logicalTableSnapshot struct {
	columns    []string
	rowDigests []string
}

type historicalLedgerSnapshot struct {
	tables              map[string]logicalTableSnapshot
	foreignKeyWitnesses []string
	integrity           string
}

func discoverHistoricalLedgerSnapshot(
	t *testing.T,
	path string,
) historicalLedgerSnapshot {
	t.Helper()
	database := openLedgerReadOnlyForTest(t, path)
	defer database.Close()
	tableNames := loadHistoricalTableNames(t, database)
	tables := make(map[string]logicalTableSnapshot, len(tableNames))
	for _, tableName := range tableNames {
		columns := loadVisibleTableColumns(t, database, tableName)
		tables[tableName] = snapshotLogicalTable(t, database, tableName, columns)
	}
	return historicalLedgerSnapshot{
		tables:              tables,
		foreignKeyWitnesses: loadForeignKeyWitnesses(t, database),
		integrity:           loadIntegrityResult(t, database),
	}
}

func repeatHistoricalLedgerSnapshot(
	t *testing.T,
	path string,
	basis historicalLedgerSnapshot,
) historicalLedgerSnapshot {
	t.Helper()
	database := openLedgerReadOnlyForTest(t, path)
	defer database.Close()
	tableNames := make([]string, 0, len(basis.tables))
	for tableName := range basis.tables {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)
	tables := make(map[string]logicalTableSnapshot, len(tableNames))
	for _, tableName := range tableNames {
		columns := basis.tables[tableName].columns
		tables[tableName] = snapshotLogicalTable(t, database, tableName, columns)
	}
	return historicalLedgerSnapshot{
		tables:              tables,
		foreignKeyWitnesses: loadForeignKeyWitnesses(t, database),
		integrity:           loadIntegrityResult(t, database),
	}
}

func openLedgerReadOnlyForTest(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open ledger read-only: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping ledger read-only: %v", err)
	}
	return database
}

func loadHistoricalTableNames(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`SELECT name
		FROM sqlite_schema
		WHERE type = 'table'
			AND name NOT LIKE 'sqlite_%'
			AND name != 'schema_version'
		ORDER BY name`)
	if err != nil {
		t.Fatalf("list historical ledger tables: %v", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("decode historical ledger table: %v", err)
		}
		result = append(result, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list historical ledger tables: %v", err)
	}
	return result
}

func loadVisibleTableColumns(
	t *testing.T,
	database *sql.DB,
	tableName string,
) []string {
	t.Helper()
	quotedTable := quoteSQLiteIdentifierForTest(tableName)
	query := "PRAGMA table_xinfo(" + quotedTable + ")"
	rows, err := database.Query(query)
	if err != nil {
		t.Fatalf("inspect historical table %s: %v", tableName, err)
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var columnID int
		var name string
		var declaredType string
		var notNull int
		var defaultValue any
		var primaryKey int
		var hidden int
		err := rows.Scan(
			&columnID,
			&name,
			&declaredType,
			&notNull,
			&defaultValue,
			&primaryKey,
			&hidden,
		)
		if err != nil {
			t.Fatalf("decode historical table %s column: %v", tableName, err)
		}
		if hidden == 0 {
			columns = append(columns, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("inspect historical table %s: %v", tableName, err)
	}
	if len(columns) == 0 {
		t.Fatalf("historical table %s has no visible columns", tableName)
	}
	return columns
}

func snapshotLogicalTable(
	t *testing.T,
	database *sql.DB,
	tableName string,
	columns []string,
) logicalTableSnapshot {
	t.Helper()
	quotedColumns := make([]string, len(columns))
	for index, column := range columns {
		quotedColumns[index] = quoteSQLiteIdentifierForTest(column)
	}
	quotedTable := quoteSQLiteIdentifierForTest(tableName)
	projection := strings.Join(quotedColumns, ", ")
	query := "SELECT " + projection + " FROM " + quotedTable
	rows, err := database.Query(query)
	if err != nil {
		t.Fatalf("snapshot historical table %s: %v", tableName, err)
	}
	defer rows.Close()
	rowDigests := make([]string, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatalf("decode historical table %s row: %v", tableName, err)
		}
		rowDigest := digestLogicalRow(t, tableName, values)
		rowDigests = append(rowDigests, rowDigest)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("snapshot historical table %s: %v", tableName, err)
	}
	sort.Strings(rowDigests)
	return logicalTableSnapshot{
		columns:    append([]string(nil), columns...),
		rowDigests: rowDigests,
	}
}

func digestLogicalRow(t *testing.T, tableName string, values []any) string {
	t.Helper()
	cells := make([]string, len(values))
	for index, value := range values {
		cell, err := canonicalSQLiteCell(value)
		if err != nil {
			t.Fatalf("canonicalize historical table %s row: %v", tableName, err)
		}
		cells[index] = cell
	}
	canonical := strings.Join(cells, "\x1f")
	digest := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("sha256:%x", digest)
}

func canonicalSQLiteCell(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "null", nil
	case int64:
		return fmt.Sprintf("integer:%d", typed), nil
	case float64:
		return fmt.Sprintf("real:%016x", math.Float64bits(typed)), nil
	case string:
		return digestSQLiteBytes("text", []byte(typed)), nil
	case []byte:
		return digestSQLiteBytes("blob", typed), nil
	case time.Time:
		canonical := typed.UTC().Format(time.RFC3339Nano)
		return digestSQLiteBytes("time", []byte(canonical)), nil
	default:
		return "", fmt.Errorf("unsupported SQLite value type %T", value)
	}
}

func digestSQLiteBytes(kind string, value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%s:%d:%x", kind, len(value), digest)
}

func loadForeignKeyWitnesses(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("inspect historical foreign-key witnesses: %v", err)
	}
	defer rows.Close()
	witnesses := make([]string, 0)
	for rows.Next() {
		var tableName string
		var rowID any
		var parent string
		var foreignKeyID int
		err := rows.Scan(&tableName, &rowID, &parent, &foreignKeyID)
		if err != nil {
			t.Fatalf("decode historical foreign-key witness: %v", err)
		}
		canonicalRowID, err := canonicalSQLiteCell(rowID)
		if err != nil {
			t.Fatalf("canonicalize historical foreign-key witness: %v", err)
		}
		witness := fmt.Sprintf(
			"%s|%s|%s|%d",
			tableName,
			canonicalRowID,
			parent,
			foreignKeyID,
		)
		witnesses = append(witnesses, witness)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("inspect historical foreign-key witnesses: %v", err)
	}
	sort.Strings(witnesses)
	return witnesses
}

func loadIntegrityResult(t *testing.T, database *sql.DB) string {
	t.Helper()
	var result string
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		t.Fatalf("inspect historical ledger integrity: %v", err)
	}
	return result
}

func assertRequiredI0Frontiers(t *testing.T, snapshot historicalLedgerSnapshot) {
	t.Helper()
	required := []string{
		"project_ledger_binding",
		"project_typeenv_heads",
		"typed_memory_graph_heads",
	}
	for _, tableName := range required {
		table, ok := snapshot.tables[tableName]
		if !ok {
			t.Fatalf("I0 predecessor lacks required table %s", tableName)
		}
		if len(table.rowDigests) != 1 {
			t.Fatalf(
				"I0 predecessor has %d rows in %s, want exactly one",
				len(table.rowDigests),
				tableName,
			)
		}
	}
}

func assertHistoricalLedgerPreserved(
	t *testing.T,
	before historicalLedgerSnapshot,
	after historicalLedgerSnapshot,
) {
	t.Helper()
	if before.integrity != "ok" || after.integrity != "ok" {
		t.Fatalf(
			"ledger integrity changed or is not clean: before=%q after=%q",
			before.integrity,
			after.integrity,
		)
	}
	if !reflect.DeepEqual(after.foreignKeyWitnesses, before.foreignKeyWitnesses) {
		t.Fatalf(
			"historical foreign-key witness set changed\nbefore: %v\nafter:  %v",
			before.foreignKeyWitnesses,
			after.foreignKeyWitnesses,
		)
	}
	tableNames := make([]string, 0, len(before.tables))
	for tableName := range before.tables {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)
	for _, tableName := range tableNames {
		beforeTable := before.tables[tableName]
		afterTable, ok := after.tables[tableName]
		if !ok {
			t.Fatalf("historical table %s disappeared", tableName)
		}
		if !reflect.DeepEqual(afterTable, beforeTable) {
			t.Fatalf(
				"historical table %s changed: before=%d rows after=%d rows",
				tableName,
				len(beforeTable.rowDigests),
				len(afterTable.rowDigests),
			)
		}
	}
}

func quoteSQLiteIdentifierForTest(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}

func digestFileForTest(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file for digest %s: %v", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		t.Fatalf("digest file %s: %v", path, err)
	}
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil))
}

func snapshotHostCarriers(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	roots := []string{
		".agents",
		".claude",
		".codex",
		".cursor",
		".grok",
		".pi",
		".mcp.json",
		"AGENTS.md",
		"CLAUDE.md",
		"opencode.json",
	}
	sort.Strings(roots)
	for _, relativeRoot := range roots {
		absoluteRoot := filepath.Join(root, relativeRoot)
		if _, err := os.Lstat(absoluteRoot); os.IsNotExist(err) {
			result[relativeRoot] = "missing"
			continue
		}
		err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if entry.IsDir() {
				result[relative] = fmt.Sprintf("dir:%o", info.Mode().Perm())
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				result[relative] = "symlink:" + target
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(content)
			result[relative] = fmt.Sprintf("file:%o:%x", info.Mode().Perm(), digest)
			return nil
		})
		if err != nil {
			t.Fatalf("snapshot host carrier %s: %v", relativeRoot, err)
		}
	}
	return result
}
