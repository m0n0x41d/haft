package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestReadOnlyProjectMemoryValidationLeavesProjectStoreUnchanged(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_4eadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(t, fixture.databaseDirectory)

	session, err := openBoundProjectMemoryReadRuntime(context.Background())
	if err != nil {
		t.Fatalf("openBoundProjectMemoryReadRuntime() error = %v", err)
	}
	payload := bytes.Replace(
		bundledMemoryValidationFixture(),
		[]byte(`{"kind":"bundled_candidate_open_world"}`),
		[]byte(`{"kind":"project_current"}`),
		1,
	)
	request, err := typedmemorywire.DecodeValidateRequest(payload)
	if err != nil {
		_ = session.Close()
		t.Fatalf("DecodeValidateRequest(project_current) error = %v", err)
	}
	result, err := session.Execute(context.Background(), request)
	if err != nil {
		_ = session.Close()
		t.Fatalf("Execute(project_current validation) error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close read-only project-memory validation: %v", err)
	}

	response := struct {
		Verdict string `json:"verdict"`
		Basis   struct {
			RequestedKind  string `json:"requested_kind"`
			ResolutionKind string `json:"resolution_kind"`
		} `json:"basis"`
		Persistence struct {
			Mode             string `json:"mode"`
			RowsWritten      uint64 `json:"rows_written"`
			AuthorityGranted bool   `json:"authority_granted"`
		} `json:"persistence_disposition"`
	}{}
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("decode read-only project-memory response: %v", err)
	}
	if response.Verdict != "underdetermined" ||
		response.Basis.RequestedKind != "project_current" ||
		response.Basis.ResolutionKind != "project_basis_unavailable" {
		t.Fatalf("read-only project-memory response = %#v", response)
	}
	if response.Persistence.Mode != "validation_only_no_write" ||
		response.Persistence.RowsWritten != 0 ||
		response.Persistence.AuthorityGranted {
		t.Fatalf("persistence disposition = %#v", response.Persistence)
	}
	if _, found := reflect.TypeOf(session.runtime).MethodByName("Admit"); found {
		t.Fatal("read-only project-memory validation exposes admission")
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(t, fixture.databaseDirectory)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatalf(
			"read-only validation changed SQLite schema\nbefore: %v\nafter:  %v",
			beforeSchema,
			afterSchema,
		)
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatalf(
			"read-only validation changed project-store files\nbefore: %v\nafter:  %v",
			beforeFiles,
			afterFiles,
		)
	}
}

func TestReadOnlyProjectMemoryValidationFailsClosedOnMissingOrOldSchema(
	t *testing.T,
) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, string)
		wantError string
	}{
		{
			name: "missing project TypeEnv head schema",
			mutate: func(t *testing.T, databasePath string) {
				executeReadOnlyProjectValidationFixtureSQL(
					t,
					databasePath,
					"DROP TABLE project_typeenv_head_store_schema",
				)
			},
			wantError: "read project TypeEnv head schema without migration",
		},
		{
			name: "old kernel schema",
			mutate: func(t *testing.T, databasePath string) {
				executeReadOnlyProjectValidationFixtureSQL(
					t,
					databasePath,
					"DELETE FROM schema_version WHERE version = 49",
				)
			},
			wantError: "kernel schema is not current",
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectID := fmt.Sprintf("qnt_5eadbee%d", index)
			fixture := newReadOnlyProjectValidationFixture(t, projectID)
			configureBoundProjectMemoryAdmissionFixture(t, fixture)
			test.mutate(t, fixture.database)
			beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
			beforeFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)

			session, err := openBoundProjectMemoryReadRuntime(
				context.Background(),
			)
			if err == nil {
				_ = session.Close()
				t.Fatal("read-only project-memory validation accepted incompatible schema")
			}
			if !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf(
					"openBoundProjectMemoryReadRuntime() error = %v, want %q",
					err,
					test.wantError,
				)
			}

			afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
			afterFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)
			if !reflect.DeepEqual(afterSchema, beforeSchema) {
				t.Fatalf("failed read-only open changed SQLite schema")
			}
			if !reflect.DeepEqual(afterFiles, beforeFiles) {
				t.Fatalf("failed read-only open changed project-store files")
			}
		})
	}
}

type readOnlyProjectValidationFixture struct {
	binding           ProjectBinding
	database          string
	databaseDirectory string
}

func newReadOnlyProjectValidationFixture(
	t *testing.T,
	projectID string,
) readOnlyProjectValidationFixture {
	t.Helper()
	home := canonicalReadOnlyProjectValidationDirectory(t, t.TempDir())
	root := canonicalReadOnlyProjectValidationDirectory(t, t.TempDir())
	t.Setenv("HOME", home)
	haftDirectory := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	projectCarrier := []byte("id: " + projectID + "\nname: read-only-memory\n")
	if err := os.WriteFile(
		filepath.Join(haftDirectory, "project.yaml"),
		projectCarrier,
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	databaseDirectory := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(databaseDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(databaseDirectory, "haft.db")
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Date(2026, time.July, 18, 8, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("projectledger.BindInitialized() error = %v", err)
	}
	return readOnlyProjectValidationFixture{
		binding: ProjectBinding{
			ProjectRoot: root,
			ProjectID:   projectID,
			DBPath:      databasePath,
		},
		database:          databasePath,
		databaseDirectory: databaseDirectory,
	}
}

type readOnlyProjectValidationFile struct {
	Path    string
	Mode    os.FileMode
	Content []byte
}

func readOnlyProjectValidationFiles(
	t *testing.T,
	root string,
) []readOnlyProjectValidationFile {
	t.Helper()
	files := []readOnlyProjectValidationFile{}
	err := filepath.WalkDir(
		root,
		func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, readOnlyProjectValidationFile{
				Path:    relative,
				Mode:    info.Mode(),
				Content: content,
			})
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(files, func(left int, right int) bool {
		return files[left].Path < files[right].Path
	})
	return files
}

func readOnlyProjectValidationSchema(
	t *testing.T,
	databasePath string,
) []string {
	t.Helper()
	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{Scheme: "file", Path: databasePath, RawQuery: query.Encode()}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(
		`SELECT type || ':' || name || ':' || COALESCE(sql, '')
		 FROM sqlite_schema
		 ORDER BY type, name`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	schema := []string{}
	for rows.Next() {
		item := ""
		if err := rows.Scan(&item); err != nil {
			t.Fatal(err)
		}
		schema = append(schema, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return schema
}

func executeReadOnlyProjectValidationFixtureSQL(
	t *testing.T,
	databasePath string,
	statement string,
) {
	t.Helper()
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(statement); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func canonicalReadOnlyProjectValidationDirectory(
	t *testing.T,
	path string,
) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(canonical)
}
