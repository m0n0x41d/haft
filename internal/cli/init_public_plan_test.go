package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestCompilePublicHostInitPlanUsesExactRequestBindings(
	t *testing.T,
) {
	projectRoot := filepath.Join(t.TempDir(), "project")
	userHomeRoot := t.TempDir()
	projectID := "qnt_e3149c17"
	t.Setenv("HOME", userHomeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   projectID,
			hosts:       initHostOptions{all: true},
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	runtime.userHomeRoot = userHomeRoot
	core := currentPreparedOperationCorePlan(
		t,
		projectRoot,
		projectID,
	)

	plan, err := compilePublicHostInitPlan(
		request,
		core,
		runtime,
		1<<20,
	)
	if err != nil {
		t.Fatalf("compilePublicHostInitPlan: %v", err)
	}
	got := make([]string, 0, len(plan.Hosts()))
	for _, host := range plan.Hosts() {
		names := make(
			[]string,
			0,
			len(host.Components().Values()),
		)
		for _, component := range host.Components().Values() {
			names = append(names, string(component))
		}
		got = append(
			got,
			host.BindingID().String()+":"+strings.Join(names, ","),
		)
	}
	want := []string{
		"claude/project:instructions,mcp",
		"claude/user:skills",
		"codex/project:instructions,mcp",
		"codex/user:skills",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("planned bindings = %v, want %v", got, want)
	}

	preview := plan.Preview()
	if !slices.Equal(preview.ApplyOrder, []string{
		"core_project",
		"host_adapter_fanout",
	}) {
		t.Fatalf("apply order = %v", preview.ApplyOrder)
	}
	for _, host := range preview.Hosts {
		for _, effect := range host.Effects {
			if filepath.Clean(effect.Path) ==
				filepath.Join(userHomeRoot, ".agents", "skills") {
				t.Fatalf(
					"--all planned independent .agents publication: %#v",
					effect,
				)
			}
		}
	}
}

func TestCompilePublicCorePlanIsPureForGreenfieldProject(
	t *testing.T,
) {
	projectRoot := filepath.Join(t.TempDir(), "greenfield")
	userHomeRoot := t.TempDir()
	projectID := "qnt_e3149c17"
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   projectID,
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}

	core, err := compilePublicCorePlan(
		context.Background(),
		request,
		userHomeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if core.Effect() != initplanning.CoreInitialize ||
		core.BeforeSchema() != 0 ||
		core.AfterSchema() <= 0 ||
		core.Basis().Kind() != initplanning.BasisUnavailable {
		t.Fatalf("core = %#v", core)
	}
	if len(core.FileEffects()) != 10 {
		t.Fatalf(
			"greenfield core file effects = %d, want 10",
			len(core.FileEffects()),
		)
	}
	wantDB := filepath.Join(
		userHomeRoot,
		".haft",
		"projects",
		projectID,
		"haft.db",
	)
	if core.DatabasePath() != wantDB {
		t.Fatalf(
			"database path = %q, want %q",
			core.DatabasePath(),
			wantDB,
		)
	}
	if _, err := os.Stat(projectRoot); !os.IsNotExist(err) {
		t.Fatalf("core planning changed project root: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(wantDB)); !os.IsNotExist(err) {
		t.Fatalf("core planning created database storage: %v", err)
	}
}

func TestCompilePublicCorePlanSelectsMigrationForPreBindingLedger(
	t *testing.T,
) {
	userHomeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve user home: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", userHomeRoot)
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	config, err := project.Create(haftDir, projectRoot)
	if err != nil {
		t.Fatalf("project.Create: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		t.Fatalf("create project ledger directory: %v", err)
	}
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open project ledger: %v", err)
	}
	_, createErr := database.Exec(
		`CREATE TABLE schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		WITH RECURSIVE versions(version) AS (
			SELECT 1
			UNION ALL
			SELECT version + 1 FROM versions WHERE version < 35
		)
		INSERT INTO schema_version(version)
		SELECT version FROM versions`,
	)
	closeErr := database.Close()
	if createErr != nil || closeErr != nil {
		t.Fatalf(
			"create schema-35 project ledger: create=%v close=%v",
			createErr,
			closeErr,
		)
	}
	beforeDigest, err := digestRegularFile(databasePath)
	if err != nil {
		t.Fatalf("digest schema-35 ledger: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   config.ID,
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}

	core, err := compilePublicCorePlan(
		context.Background(),
		request,
		userHomeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if core.Effect() != initplanning.CoreMigrate ||
		core.BeforeSchema() != 35 ||
		core.AfterSchema() != current ||
		core.Basis().Kind() != initplanning.BasisUnavailable {
		t.Fatalf("core = %#v", core)
	}
	afterDigest, err := digestRegularFile(databasePath)
	if err != nil {
		t.Fatalf("redigest schema-35 ledger: %v", err)
	}
	if afterDigest != beforeDigest {
		t.Fatal("pre-binding core planning changed the project ledger")
	}
}
