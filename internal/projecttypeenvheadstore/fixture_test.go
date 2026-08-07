package projecttypeenvheadstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

type headStoreFixture struct {
	database *sql.DB
	store    *Store
	project  projectidentity.ProjectID
}

func newHeadStoreFixture(t *testing.T) headStoreFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "project-typeenv-head.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("open project TypeEnv head database: %v", err)
	}
	database.SetMaxOpenConns(1)
	store, err := New(context.Background(), database)
	if err != nil {
		_ = database.Close()
		t.Fatalf("New(): %v", err)
	}
	project, err := projectidentity.ParseProjectID("qnt_0123abcd")
	if err != nil {
		_ = database.Close()
		t.Fatalf("ParseProjectID(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return headStoreFixture{
		database: database,
		store:    store,
		project:  project,
	}
}

func headStateFixture(
	t *testing.T,
	project projectidentity.ProjectID,
	revision uint64,
	digit string,
) projecttypeenvselection.ProjectTypeEnvHeadState {
	t.Helper()
	composite, err := typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(): %v", err)
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(revision)
	if err != nil {
		t.Fatalf("NewHeadRevision(): %v", err)
	}
	state, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           project,
			SelectedComposite: composite,
			Revision:          headRevision,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState(): %v", err)
	}
	return state
}

func insertHistoryRowDirect(
	t *testing.T,
	database *sql.DB,
	state projecttypeenvselection.ProjectTypeEnvHeadState,
) {
	t.Helper()
	row, err := prepareStoredHeadRow(state)
	if err != nil {
		t.Fatalf("prepareStoredHeadRow(): %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_head_states (
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		row.arguments()...,
	)
	if err != nil {
		t.Fatalf("insert direct history row: %v", err)
	}
}

func countHeadRows(
	t *testing.T,
	database *sql.DB,
) (int, int) {
	t.Helper()
	var currentCount int
	err := database.QueryRow(
		`SELECT COUNT(*) FROM project_typeenv_heads`,
	).Scan(&currentCount)
	if err != nil {
		t.Fatalf("count current heads: %v", err)
	}
	var historyCount int
	err = database.QueryRow(
		`SELECT COUNT(*) FROM project_typeenv_head_states`,
	).Scan(&historyCount)
	if err != nil {
		t.Fatalf("count immutable head states: %v", err)
	}
	return currentCount, historyCount
}
