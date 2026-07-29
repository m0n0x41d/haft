package projectledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	_ "github.com/m0n0x41d/haft/internal/sqlitepolicy"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

const bindingSchemaV1 = "haft.project-ledger-binding/v1"

var ErrBindingMissing = errors.New("project ledger has no durable project identity binding; run haft init from the canonical project root")
var ErrBindingCommittedTopologyChanged = errors.New("project ledger binding committed to the anchored database, but project topology changed before post-commit verification")

type Access string

const (
	ReadOnly  Access = "read_only"
	ReadWrite Access = "read_write"
)

// ProjectID is an alias of the effect-free canonical project identity. The
// ledger establishes and verifies the binding; it does not own a second ID
// representation.
type ProjectID = projectidentity.ProjectID

func ParseProjectID(raw string) (ProjectID, error) {
	return projectidentity.ParseProjectID(raw)
}

type ProjectRoot struct {
	value string
}

func NewProjectRoot(raw string) (ProjectRoot, error) {
	if raw != strings.TrimSpace(raw) || !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return ProjectRoot{}, fmt.Errorf("project root must be a canonical absolute path")
	}
	info, err := os.Lstat(raw)
	if err != nil {
		return ProjectRoot{}, fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ProjectRoot{}, fmt.Errorf("project root must be a real directory")
	}
	physical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return ProjectRoot{}, fmt.Errorf("resolve physical project root: %w", err)
	}
	if filepath.Clean(physical) != raw {
		return ProjectRoot{}, fmt.Errorf("project root must use canonical physical form; symlink aliases are not admitted")
	}
	return ProjectRoot{value: raw}, nil
}

func (root ProjectRoot) String() string {
	return root.value
}

type Identity struct {
	root ProjectRoot
	id   ProjectID
}

func LoadIdentity(root string) (Identity, error) {
	identity, anchors, err := loadIdentityAnchored(root)
	_ = closeAnchors(anchors)
	return identity, err
}

func (identity Identity) ProjectRoot() ProjectRoot {
	return identity.root
}

func (identity Identity) ProjectID() ProjectID {
	return identity.id
}

type Handle struct {
	identity          Identity
	database          *sql.DB
	anchors           []topologyAnchor
	databaseAnchor    topologyAnchor
	sidecarGeneration *sqliteSidecarGeneration
	dbPath            string
}

func OpenExisting(
	ctx context.Context,
	root string,
	access Access,
) (*Handle, error) {
	identity, identityAnchors, err := loadIdentityAnchored(root)
	if err != nil {
		return nil, err
	}
	handle, err := openTopology(ctx, identity, identityAnchors, access)
	if err != nil {
		_ = closeAnchors(identityAnchors)
		return nil, err
	}
	if err := handle.Revalidate(ctx); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return handle, nil
}

// BindInitialized is called only by an explicit core initialization effect
// after all database migrations have completed. Ordinary runtime opens never
// create or repair this immutable identity binding.
func BindInitialized(
	ctx context.Context,
	root string,
	at time.Time,
) error {
	identity, identityAnchors, err := loadIdentityAnchored(root)
	if err != nil {
		return err
	}
	handle, err := openTopology(ctx, identity, identityAnchors, ReadWrite)
	if err != nil {
		_ = closeAnchors(identityAnchors)
		return err
	}
	defer handle.Close()
	if err := handle.bindInitialized(ctx, at); err != nil {
		return err
	}
	return handle.Revalidate(ctx)
}

func (handle *Handle) Database() *sql.DB {
	return handle.database
}

func (handle *Handle) ProjectID() ProjectID {
	return handle.identity.id
}

func (handle *Handle) ProjectRoot() ProjectRoot {
	return handle.identity.root
}

func (handle *Handle) DatabasePath() string {
	if handle == nil {
		return ""
	}
	return handle.dbPath
}

func (handle *Handle) Revalidate(ctx context.Context) error {
	if handle == nil || handle.database == nil || len(handle.anchors) == 0 || handle.databaseAnchor.file == nil {
		return fmt.Errorf("project ledger handle is closed")
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return err
	}
	connection, err := handle.database.Conn(ctx)
	if err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return fmt.Errorf("reserve checked project ledger connection: %w", err)
	}
	defer connection.Close()
	if err := connection.PingContext(ctx); err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return fmt.Errorf("ping checked project ledger: %w", err)
	}
	if err := verifySQLiteMainDatabaseIdentity(ctx, connection, handle.databaseAnchor); err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return err
	}
	if err := requireAttachedIdentity(ctx, connection, handle.identity); err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return fmt.Errorf("revalidate project ledger topology after identity read: %w", err)
	}
	return nil
}

func (handle *Handle) RequireAttachedIdentity(ctx context.Context) error {
	if handle == nil || handle.database == nil || len(handle.anchors) == 0 || handle.databaseAnchor.file == nil {
		return fmt.Errorf("project ledger handle is closed")
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return err
	}
	connection, err := handle.database.Conn(ctx)
	if err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return fmt.Errorf("reserve checked project ledger identity connection: %w", err)
	}
	defer connection.Close()
	if err := verifySQLiteMainDatabaseIdentity(ctx, connection, handle.databaseAnchor); err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return err
	}
	if err := requireAttachedIdentity(ctx, connection, handle.identity); err != nil {
		if generationErr := handle.sidecarGeneration.Revalidate(); generationErr != nil {
			return generationErr
		}
		return err
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		return err
	}
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return fmt.Errorf("revalidate project ledger topology after identity read: %w", err)
	}
	return nil
}

func (handle *Handle) Close() error {
	if handle == nil {
		return nil
	}
	var databaseErr error
	if handle.database != nil {
		databaseErr = handle.database.Close()
		handle.database = nil
	}
	anchorErr := closeAnchors(handle.anchors)
	handle.anchors = nil
	return errors.Join(databaseErr, anchorErr)
}

func openTopology(
	ctx context.Context,
	identity Identity,
	identityAnchors []topologyAnchor,
	access Access,
) (*Handle, error) {
	return openTopologyWithHook(ctx, identity, identityAnchors, access, nil)
}

type sqliteOpenStage string

const (
	sqliteOpenStageBeforeConnect                   sqliteOpenStage = "before_connect"
	sqliteOpenStageAfterConnectBeforeIdentityCheck sqliteOpenStage = "after_connect_before_identity_check"
)

type sqliteOpenHook func(sqliteOpenStage) error

// openTopologyWithHook establishes two observable identity boundaries around
// the driver's lazy SQLite connect: every anchored pathname is checked before
// the connect, and the main database pathname reported by that exact
// connection is checked against the anchored database inode immediately after
// it. This process-local check deliberately does not claim to detect an
// adversarial swap-and-restore completed wholly between unobservable syscalls;
// eliminating that gap would require a descriptor-aware SQLite VFS rather than
// pathname-based database/sql opening.
func openTopologyWithHook(
	ctx context.Context,
	identity Identity,
	identityAnchors []topologyAnchor,
	access Access,
	hook sqliteOpenHook,
) (*Handle, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	mode, err := sqliteAccessMode(access)
	if err != nil {
		return nil, err
	}
	home, err := canonicalPhysicalHome()
	if err != nil {
		return nil, err
	}
	homeAnchor, err := openAbsoluteDirectoryAnchor(home, "home directory")
	if err != nil {
		return nil, err
	}
	ledgerAnchors := []topologyAnchor{homeAnchor}
	closeOnFailure := func() {
		_ = closeAnchors(ledgerAnchors)
	}
	haftHome := filepath.Join(home, ".haft")
	haftHomeAnchor, err := openChildDirectoryAnchor(homeAnchor, ".haft", haftHome, "Haft home")
	if err != nil {
		closeOnFailure()
		return nil, err
	}
	ledgerAnchors = append(ledgerAnchors, haftHomeAnchor)
	projectsHome := filepath.Join(haftHome, "projects")
	projectsAnchor, err := openChildDirectoryAnchor(haftHomeAnchor, "projects", projectsHome, "Haft projects directory")
	if err != nil {
		closeOnFailure()
		return nil, err
	}
	ledgerAnchors = append(ledgerAnchors, projectsAnchor)
	projectDirectory := filepath.Join(projectsHome, identity.id.String())
	projectAnchor, err := openChildDirectoryAnchor(projectsAnchor, identity.id.String(), projectDirectory, "project ledger directory")
	if err != nil {
		closeOnFailure()
		return nil, err
	}
	ledgerAnchors = append(ledgerAnchors, projectAnchor)
	databasePath := filepath.Join(projectDirectory, "haft.db")
	databaseAnchor, err := openChildRegularFileAnchor(projectAnchor, "haft.db", databasePath, "project ledger database")
	if err != nil {
		closeOnFailure()
		return nil, err
	}
	ledgerAnchors = append(ledgerAnchors, databaseAnchor)
	allAnchors := append([]topologyAnchor{}, identityAnchors...)
	allAnchors = append(allAnchors, ledgerAnchors...)
	if err := verifyAnchoredTopology(allAnchors); err != nil {
		closeOnFailure()
		return nil, fmt.Errorf("verify project ledger topology before SQLite connect: %w", err)
	}
	query := url.Values{}
	query.Set("mode", mode)
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	dsn := url.URL{Scheme: "file", Path: databasePath, RawQuery: query.Encode()}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		closeOnFailure()
		return nil, fmt.Errorf("open project ledger SQLite handle: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	database.SetConnMaxLifetime(0)
	if hook != nil {
		if err := hook(sqliteOpenStageBeforeConnect); err != nil {
			_ = database.Close()
			closeOnFailure()
			return nil, fmt.Errorf("run pre-connect project ledger identity hook: %w", err)
		}
	}
	connection, err := database.Conn(ctx)
	if err != nil {
		_ = database.Close()
		closeOnFailure()
		return nil, fmt.Errorf("reserve project ledger SQLite connection: %w", err)
	}
	closeConnectionOnFailure := func() {
		_ = connection.Close()
		_ = database.Close()
		closeOnFailure()
	}
	if err := connection.PingContext(ctx); err != nil {
		closeConnectionOnFailure()
		return nil, fmt.Errorf("ping project ledger SQLite handle: %w", err)
	}
	if hook != nil {
		if err := hook(sqliteOpenStageAfterConnectBeforeIdentityCheck); err != nil {
			closeConnectionOnFailure()
			return nil, fmt.Errorf("run post-connect project ledger identity hook: %w", err)
		}
	}
	if err := verifySQLiteMainDatabaseIdentity(ctx, connection, databaseAnchor); err != nil {
		closeConnectionOnFailure()
		return nil, err
	}
	if err := verifyAnchoredTopology(allAnchors); err != nil {
		closeConnectionOnFailure()
		return nil, err
	}
	if err := connection.Close(); err != nil {
		_ = database.Close()
		closeOnFailure()
		return nil, fmt.Errorf("release checked project ledger SQLite connection: %w", err)
	}
	handle := &Handle{
		identity:          identity,
		database:          database,
		anchors:           allAnchors,
		databaseAnchor:    databaseAnchor,
		sidecarGeneration: newSQLiteSidecarGeneration(databasePath),
		dbPath:            databasePath,
	}
	if err := handle.sidecarGeneration.Revalidate(); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return handle, nil
}

func (handle *Handle) bindInitialized(ctx context.Context, at time.Time) error {
	return handle.bindInitializedWithHook(ctx, at, nil)
}

type bindingStage string

const (
	bindingStageBeforeCommit           bindingStage = "before_commit"
	bindingStageAfterCommitBeforeCheck bindingStage = "after_commit_before_check"
)

type bindingHook func(bindingStage) error

func (handle *Handle) bindInitializedWithHook(
	ctx context.Context,
	at time.Time,
	hook bindingHook,
) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if at.IsZero() {
		return fmt.Errorf("project ledger binding time is required")
	}
	record, err := newLedgerBindingRecord(handle.identity, at)
	if err != nil {
		return err
	}
	connection, err := handle.database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve project ledger binding connection: %w", err)
	}
	defer connection.Close()
	if err := verifyAnchoredTopology(handle.anchors); err != nil {
		return fmt.Errorf("revalidate project ledger topology before binding: %w", err)
	}
	if err := verifySQLiteMainDatabaseIdentity(ctx, connection, handle.databaseAnchor); err != nil {
		return fmt.Errorf("revalidate project ledger database identity before binding: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin atomic project ledger binding: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = connection.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	commitAndVerify := func() error {
		if hook != nil {
			if err := hook(bindingStageBeforeCommit); err != nil {
				return fmt.Errorf("run project ledger binding transition hook: %w", err)
			}
		}
		if err := verifyAnchoredTopology(handle.anchors); err != nil {
			return fmt.Errorf("revalidate project ledger topology before binding commit: %w", err)
		}
		if err := verifySQLiteMainDatabaseIdentity(ctx, connection, handle.databaseAnchor); err != nil {
			return fmt.Errorf("revalidate project ledger database identity before binding commit: %w", err)
		}
		if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
			return fmt.Errorf("commit project ledger binding: %w", err)
		}
		committed = true
		if hook != nil {
			if err := hook(bindingStageAfterCommitBeforeCheck); err != nil {
				return errors.Join(ErrBindingCommittedTopologyChanged, err)
			}
		}
		if err := verifyAnchoredTopology(handle.anchors); err != nil {
			return errors.Join(ErrBindingCommittedTopologyChanged, err)
		}
		if err := verifySQLiteMainDatabaseIdentity(ctx, connection, handle.databaseAnchor); err != nil {
			return errors.Join(ErrBindingCommittedTopologyChanged, err)
		}
		return nil
	}
	existingErr := requireAttachedIdentity(ctx, connection, handle.identity)
	if existingErr == nil {
		return commitAndVerify()
	}
	if !errors.Is(existingErr, ErrBindingMissing) {
		return existingErr
	}
	if err := rejectConflictingDurableRoots(ctx, connection, handle.identity.root); err != nil {
		return err
	}
	_, err = connection.ExecContext(
		ctx,
		`INSERT INTO project_ledger_binding (
			binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
		 ) VALUES (1, ?, ?, ?, ?, ?)`,
		record.dto.ProjectID,
		record.dto.ProjectRoot,
		record.digest,
		string(record.canonical),
		record.dto.BoundAt,
	)
	if err != nil {
		return fmt.Errorf("bind initialized project ledger: %w", err)
	}
	if err := requireAttachedIdentity(ctx, connection, handle.identity); err != nil {
		return fmt.Errorf("recheck initialized project ledger binding: %w", err)
	}
	return commitAndVerify()
}

type queryRows interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type queryRow interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// PersistedBindingReader is the narrow read capability needed to verify the
// immutable project-ledger binding inside a caller-owned transaction.
type PersistedBindingReader interface {
	ScanOne(
		context.Context,
		string,
		[]any,
		[]any,
	) error
}

// RequireExactPersistedBinding verifies the canonical immutable binding row
// and requires it to name the exact requested project. It performs no topology
// discovery and grants no project mutation authority.
func RequireExactPersistedBinding(
	ctx context.Context,
	reader PersistedBindingReader,
	project ProjectID,
) error {
	if ctx == nil {
		return fmt.Errorf("verify persisted project ledger binding: context is required")
	}
	if reader == nil {
		return fmt.Errorf("verify persisted project ledger binding: reader is required")
	}
	canonicalProject, err := ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return fmt.Errorf(
			"verify persisted project ledger binding: project identity is required",
		)
	}
	row := ledgerBindingRow{}
	err = reader.ScanOne(
		ctx,
		`SELECT binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
		 FROM project_ledger_binding WHERE binding_slot = 1`,
		nil,
		[]any{
			&row.slot,
			&row.projectID,
			&row.projectRoot,
			&row.bindingDigest,
			&row.bindingJSON,
			&row.boundAt,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBindingMissing
	}
	if err != nil {
		return fmt.Errorf("read persisted project ledger binding: %w", err)
	}
	storedProject, err := ParseProjectID(row.projectID)
	if err != nil {
		return fmt.Errorf("parse persisted project ledger identity: %w", err)
	}
	storedRoot, err := NewProjectRoot(row.projectRoot)
	if err != nil {
		return fmt.Errorf("parse persisted project ledger root: %w", err)
	}
	storedIdentity := Identity{
		root: storedRoot,
		id:   storedProject,
	}
	if err := validateLedgerBindingRow(row, storedIdentity); err != nil {
		return fmt.Errorf("verify persisted project ledger binding: %w", err)
	}
	if storedProject != canonicalProject {
		return fmt.Errorf(
			"project ledger is durably bound to id %q, not requested id %q",
			storedProject.String(),
			canonicalProject.String(),
		)
	}
	return nil
}

func rejectConflictingDurableRoots(
	ctx context.Context,
	database queryRows,
	root ProjectRoot,
) error {
	tables, err := discoverDurableRootTables(ctx, database)
	if err != nil {
		return err
	}
	for _, table := range tables {
		query := "SELECT DISTINCT project_root FROM " + quoteSQLiteIdentifier(table) + " ORDER BY project_root"
		rows, err := database.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("inspect durable roots in %s before binding: %w", table, err)
		}
		for rows.Next() {
			value := ""
			if err := rows.Scan(&value); err != nil {
				_ = rows.Close()
				return fmt.Errorf("read durable root in %s before binding: %w", table, err)
			}
			if value != root.String() {
				_ = rows.Close()
				return fmt.Errorf(
					"project ledger contains %s row for root %q, not requested root %q",
					table,
					value,
					root.String(),
				)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read durable roots in %s before binding: %w", table, err)
		}
		_ = rows.Close()
	}
	return nil
}

func discoverDurableRootTables(
	ctx context.Context,
	database queryRows,
) ([]string, error) {
	rows, err := database.QueryContext(
		ctx,
		`SELECT DISTINCT schema_table.name
		 FROM sqlite_schema schema_table
		 JOIN pragma_table_info(schema_table.name) column_info
		 WHERE schema_table.type = 'table'
		 AND column_info.name = 'project_root'
		 AND schema_table.name != 'project_ledger_binding'
		 ORDER BY schema_table.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("discover durable project-root tables before binding: %w", err)
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		table := ""
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("read durable project-root table before binding: %w", err)
		}
		if table == "" {
			return nil, fmt.Errorf("durable project-root table has an empty name")
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read durable project-root tables before binding: %w", err)
	}
	return tables, nil
}

func quoteSQLiteIdentifier(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}

func requireAttachedIdentity(
	ctx context.Context,
	database queryRow,
	identity Identity,
) error {
	row := ledgerBindingRow{}
	err := database.QueryRowContext(
		ctx,
		`SELECT binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
		 FROM project_ledger_binding WHERE binding_slot = 1`,
	).Scan(
		&row.slot,
		&row.projectID,
		&row.projectRoot,
		&row.bindingDigest,
		&row.bindingJSON,
		&row.boundAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBindingMissing
	}
	if err != nil {
		return fmt.Errorf("read durable project ledger binding: %w", err)
	}
	return validateLedgerBindingRow(row, identity)
}

type ledgerBindingDTO struct {
	Schema      string `json:"schema"`
	ProjectID   string `json:"project_id"`
	ProjectRoot string `json:"project_root"`
	BoundAt     string `json:"bound_at"`
}

type ledgerBindingRecord struct {
	dto       ledgerBindingDTO
	canonical []byte
	digest    string
}

type ledgerBindingRow struct {
	slot          int
	projectID     string
	projectRoot   string
	bindingDigest string
	bindingJSON   string
	boundAt       string
}

func newLedgerBindingRecord(identity Identity, at time.Time) (ledgerBindingRecord, error) {
	canonicalTime := at.Round(0).UTC()
	dto := ledgerBindingDTO{
		Schema:      bindingSchemaV1,
		ProjectID:   identity.id.String(),
		ProjectRoot: identity.root.String(),
		BoundAt:     canonicalTime.Format(time.RFC3339Nano),
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return ledgerBindingRecord{}, err
	}
	digest := sha256.Sum256(canonical)
	return ledgerBindingRecord{
		dto:       dto,
		canonical: canonical,
		digest:    "sha256:" + hex.EncodeToString(digest[:]),
	}, nil
}

func validateLedgerBindingRow(row ledgerBindingRow, identity Identity) error {
	if row.slot != 1 {
		return fmt.Errorf("project ledger binding slot is invalid")
	}
	id, err := ParseProjectID(row.projectID)
	if err != nil {
		return err
	}
	root, err := NewProjectRoot(row.projectRoot)
	if err != nil {
		return err
	}
	boundAt, err := time.Parse(time.RFC3339Nano, row.boundAt)
	if err != nil || boundAt.Location() != time.UTC {
		return fmt.Errorf("project ledger binding time is not canonical UTC RFC3339Nano")
	}
	record, err := newLedgerBindingRecord(Identity{root: root, id: id}, boundAt)
	if err != nil {
		return err
	}
	if !bytes.Equal(record.canonical, []byte(row.bindingJSON)) || record.digest != row.bindingDigest {
		return fmt.Errorf("project ledger binding canonical bytes or digest are invalid")
	}
	if id.String() != identity.id.String() || root.String() != identity.root.String() {
		return fmt.Errorf(
			"project ledger is durably bound to id %q root %q, not id %q root %q",
			id.String(),
			root.String(),
			identity.id.String(),
			identity.root.String(),
		)
	}
	return nil
}

func canonicalPhysicalHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return "", fmt.Errorf("home directory must be a canonical absolute path")
	}
	physical, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", fmt.Errorf("resolve physical home directory: %w", err)
	}
	return filepath.Clean(physical), nil
}

type projectIdentityCarrier struct {
	ID string `yaml:"id"`
}

type topologyAnchor struct {
	file          *os.File
	info          os.FileInfo
	path          string
	name          string
	contentDigest string
}

func loadIdentityAnchored(root string) (Identity, []topologyAnchor, error) {
	projectRoot, err := NewProjectRoot(root)
	if err != nil {
		return Identity{}, nil, err
	}
	rootAnchor, err := openAbsoluteDirectoryAnchor(projectRoot.String(), "project root")
	if err != nil {
		return Identity{}, nil, err
	}
	anchors := []topologyAnchor{rootAnchor}
	closeOnFailure := func() {
		_ = closeAnchors(anchors)
	}
	haftPath := filepath.Join(projectRoot.String(), ".haft")
	haftAnchor, err := openChildDirectoryAnchor(rootAnchor, ".haft", haftPath, "project .haft directory")
	if err != nil {
		closeOnFailure()
		return Identity{}, nil, err
	}
	anchors = append(anchors, haftAnchor)
	configPath := filepath.Join(haftPath, "project.yaml")
	configAnchor, err := openChildRegularFileAnchor(haftAnchor, "project.yaml", configPath, "project identity carrier")
	if err != nil {
		closeOnFailure()
		return Identity{}, nil, err
	}
	configBytes, err := readAnchoredContent(configAnchor)
	if err != nil {
		_ = configAnchor.file.Close()
		closeOnFailure()
		return Identity{}, nil, err
	}
	configDigest := sha256.Sum256(configBytes)
	configAnchor.contentDigest = hex.EncodeToString(configDigest[:])
	anchors = append(anchors, configAnchor)
	config := projectIdentityCarrier{}
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		closeOnFailure()
		return Identity{}, nil, fmt.Errorf("parse project identity carrier: %w", err)
	}
	projectID, err := ParseProjectID(config.ID)
	if err != nil {
		closeOnFailure()
		return Identity{}, nil, err
	}
	return Identity{root: projectRoot, id: projectID}, anchors, nil
}

func openAbsoluteDirectoryAnchor(path string, name string) (topologyAnchor, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return topologyAnchor{}, fmt.Errorf("open no-follow %s: %w", name, err)
	}
	return anchorOpenedFile(fd, path, name, true)
}

func openChildDirectoryAnchor(
	parent topologyAnchor,
	entry string,
	path string,
	name string,
) (topologyAnchor, error) {
	fd, err := unix.Openat(int(parent.file.Fd()), entry, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0) // #nosec G115 -- parent is an open topology anchor.
	if err != nil {
		return topologyAnchor{}, fmt.Errorf("open anchored no-follow %s: %w", name, err)
	}
	return anchorOpenedFile(fd, path, name, true)
}

func openChildRegularFileAnchor(
	parent topologyAnchor,
	entry string,
	path string,
	name string,
) (topologyAnchor, error) {
	fd, err := unix.Openat(int(parent.file.Fd()), entry, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0) // #nosec G115 -- parent is an open topology anchor.
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return topologyAnchor{}, fmt.Errorf("anchored %s must be a real regular file, not a symlink", name)
		}
		return topologyAnchor{}, fmt.Errorf("open anchored no-follow %s: %w", name, err)
	}
	return anchorOpenedFile(fd, path, name, false)
}

func anchorOpenedFile(fd int, path string, name string, directory bool) (topologyAnchor, error) {
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open or unix.Openat returned a valid nonnegative descriptor.
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return topologyAnchor{}, fmt.Errorf("inspect anchored %s: %w", name, err)
	}
	valid := info.Mode().IsRegular()
	if directory {
		valid = info.IsDir()
	}
	if !valid {
		_ = file.Close()
		kind := "regular file"
		if directory {
			kind = "directory"
		}
		return topologyAnchor{}, fmt.Errorf("anchored %s must be a real %s, not a symlink", name, kind)
	}
	return topologyAnchor{file: file, info: info, path: path, name: name}, nil
}

func verifyAnchoredTopology(anchors []topologyAnchor) error {
	for _, anchor := range anchors {
		observed, err := os.Lstat(anchor.path)
		if err != nil {
			return fmt.Errorf("reinspect anchored %s: %w", anchor.name, err)
		}
		if observed.Mode()&os.ModeSymlink != 0 || !os.SameFile(anchor.info, observed) {
			return fmt.Errorf("anchored %s identity changed after checked open", anchor.name)
		}
		if anchor.contentDigest == "" {
			continue
		}
		content, err := readAnchoredContent(anchor)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != anchor.contentDigest {
			return fmt.Errorf("anchored %s content changed after checked open", anchor.name)
		}
	}
	return nil
}

// verifySQLiteMainDatabaseIdentity checks the pathname that SQLite reports for
// the exact reserved connection, then requires that pathname to still name the
// regular-file inode pinned by the no-follow anchor. Together with the
// immediately preceding topology check this brackets connection creation. It
// is an observed pathname/inode boundary, not proof against a swap-and-restore
// performed wholly between the underlying syscalls.
func verifySQLiteMainDatabaseIdentity(
	ctx context.Context,
	database queryRow,
	anchor topologyAnchor,
) error {
	if anchor.file == nil || anchor.path == "" {
		return fmt.Errorf("project ledger database anchor is unavailable")
	}
	reportedPath := ""
	err := database.QueryRowContext(
		ctx,
		"SELECT file FROM pragma_database_list WHERE name = 'main'",
	).Scan(&reportedPath)
	if err != nil {
		return fmt.Errorf("read SQLite main database pathname: %w", err)
	}
	if !filepath.IsAbs(reportedPath) || filepath.Clean(reportedPath) != reportedPath {
		return fmt.Errorf("SQLite main database pathname is not canonical absolute: %q", reportedPath)
	}
	if reportedPath != anchor.path {
		return fmt.Errorf(
			"SQLite main database pathname %q does not match anchored pathname %q",
			reportedPath,
			anchor.path,
		)
	}
	observed, err := os.Lstat(reportedPath)
	if err != nil {
		return fmt.Errorf("reinspect SQLite main database pathname: %w", err)
	}
	if observed.Mode()&os.ModeSymlink != 0 || !observed.Mode().IsRegular() {
		return fmt.Errorf("SQLite main database pathname must still name a real regular file")
	}
	if !os.SameFile(anchor.info, observed) {
		return fmt.Errorf("SQLite main database pathname no longer names the anchored database inode")
	}
	return nil
}

func readAnchoredContent(anchor topologyAnchor) ([]byte, error) {
	info, err := anchor.file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect anchored %s content: %w", anchor.name, err)
	}
	reader := io.NewSectionReader(anchor.file, 0, info.Size())
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read anchored %s content: %w", anchor.name, err)
	}
	return content, nil
}

func closeAnchors(anchors []topologyAnchor) error {
	errorsByAnchor := make([]error, 0, len(anchors))
	for index := len(anchors) - 1; index >= 0; index-- {
		if anchors[index].file == nil {
			continue
		}
		errorsByAnchor = append(errorsByAnchor, anchors[index].file.Close())
	}
	return errors.Join(errorsByAnchor...)
}

func sqliteAccessMode(access Access) (string, error) {
	switch access {
	case ReadOnly:
		return "ro", nil
	case ReadWrite:
		return "rw", nil
	default:
		return "", fmt.Errorf("unknown project ledger access %q", access)
	}
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("project ledger context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("project ledger context is inactive: %w", err)
	}
	return nil
}
