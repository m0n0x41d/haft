package cli

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestPublicProjectCoreEffectInitializesExactPlannedIdentity(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project parent: %v", err)
	}
	projectRoot := filepath.Join(parent, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)

	receipt, err := effect.ApplyCore(
		context.Background(),
		plan,
	)
	if err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	if receipt.Outcome() != initexecution.CoreEffectApplied ||
		receipt.Effect() != initplanning.CoreInitialize {
		t.Fatalf("receipt = %#v", receipt)
	}
	config, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		t.Fatalf("Load project config: %v", err)
	}
	if config == nil || config.ID != "qnt_e3149c17" {
		t.Fatalf("project config = %#v", config)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	if receipt.BeforeSchema() != 0 ||
		receipt.AfterSchema() != current {
		t.Fatalf(
			"schema receipt = %d -> %d, want 0 -> %d",
			receipt.BeforeSchema(),
			receipt.AfterSchema(),
			current,
		)
	}
}

func TestPublicProjectCoreEffectMigratesExactLegacyQuintDatabaseSeed(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	legacyPath := filepath.Join(
		projectRoot,
		".quint",
		"quint.db",
	)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("create legacy root: %v", err)
	}
	legacyWorkflowPath := filepath.Join(
		projectRoot,
		".quint",
		"workflow.md",
	)
	legacyWorkflow := []byte("# Legacy workflow\n\nKeep these bytes.\n")
	if err := os.WriteFile(
		legacyWorkflowPath,
		legacyWorkflow,
		0o640,
	); err != nil {
		t.Fatalf("write legacy workflow: %v", err)
	}
	if err := initializeDatabase(legacyPath); err != nil {
		t.Fatalf("initialize legacy database: %v", err)
	}
	legacyDatabase, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	_, createErr := legacyDatabase.Exec(
		`CREATE TABLE legacy_probe (value TEXT NOT NULL);
		 INSERT INTO legacy_probe(value) VALUES ('preserved')`,
	)
	closeErr := legacyDatabase.Close()
	if createErr != nil || closeErr != nil {
		t.Fatalf(
			"seed legacy database: create=%v close=%v",
			createErr,
			closeErr,
		)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	seed := plan.DatabaseSeed()
	if seed.Kind() != initplanning.CoreDatabaseSeedLegacyCopy ||
		seed.ObservationPath() != legacyPath ||
		seed.SourcePath() != filepath.Join(
			projectRoot,
			".haft",
			"quint.db",
		) ||
		seed.Digest() == "" {
		t.Fatalf("legacy seed = %#v", seed)
	}
	if len(plan.FileEffects()) != 10 {
		t.Fatalf(
			"legacy carrier effects = %d, want 10",
			len(plan.FileEffects()),
		)
	}
	workflowPlanned := false
	for _, file := range plan.FileEffects() {
		if file.Path() != legacyWorkflowPath {
			continue
		}
		workflowPlanned = file.Kind() ==
			initplanning.CoreFilePreserve
	}
	if !workflowPlanned {
		t.Fatalf(
			"legacy workflow was not an exact preserve effect: %#v",
			plan.FileEffects(),
		)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(
		context.Background(),
		plan,
	); err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	canonical, err := sql.Open("sqlite", plan.DatabasePath())
	if err != nil {
		t.Fatalf("open canonical database: %v", err)
	}
	var value string
	queryErr := canonical.QueryRow(
		"SELECT value FROM legacy_probe",
	).Scan(&value)
	closeErr = canonical.Close()
	if queryErr != nil || closeErr != nil || value != "preserved" {
		t.Fatalf(
			"legacy probe value=%q query=%v close=%v",
			value,
			queryErr,
			closeErr,
		)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".quint"),
	); !os.IsNotExist(err) {
		t.Fatalf("legacy project root was not migrated: %v", err)
	}
	migratedWorkflowPath := filepath.Join(
		projectRoot,
		".haft",
		"workflow.md",
	)
	migratedWorkflow, err := os.ReadFile(migratedWorkflowPath)
	if err != nil {
		t.Fatalf("read migrated workflow: %v", err)
	}
	migratedWorkflowInfo, err := os.Stat(migratedWorkflowPath)
	if err != nil {
		t.Fatalf("stat migrated workflow: %v", err)
	}
	if string(migratedWorkflow) != string(legacyWorkflow) ||
		migratedWorkflowInfo.Mode().Perm() != 0o640 {
		t.Fatalf(
			"migrated workflow bytes=%q mode=%o",
			migratedWorkflow,
			migratedWorkflowInfo.Mode().Perm(),
		)
	}
}

func TestPublicProjectCoreEffectRejectsChangedLegacyCarrierBeforeWrites(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	legacyRoot := filepath.Join(projectRoot, ".quint")
	legacyPath := filepath.Join(legacyRoot, "quint.db")
	workflowPath := filepath.Join(legacyRoot, "workflow.md")
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("create legacy root: %v", err)
	}
	if err := initializeDatabase(legacyPath); err != nil {
		t.Fatalf("initialize legacy database: %v", err)
	}
	if err := os.WriteFile(
		workflowPath,
		[]byte("before\n"),
		0o644,
	); err != nil {
		t.Fatalf("write legacy workflow: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if err := os.WriteFile(
		workflowPath,
		[]byte("after\n"),
		0o644,
	); err != nil {
		t.Fatalf("mutate legacy workflow: %v", err)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(
		context.Background(),
		plan,
	); err == nil {
		t.Fatal("changed legacy carrier was applied")
	}
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("legacy workflow disappeared: %v", err)
	}
	if string(content) != "after\n" {
		t.Fatalf("legacy workflow = %q", content)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("changed carrier precondition wrote .haft: %v", err)
	}
	if _, err := os.Stat(plan.DatabasePath()); !os.IsNotExist(err) {
		t.Fatalf(
			"changed carrier precondition wrote canonical database: %v",
			err,
		)
	}
}

func TestPublicProjectCoreEffectRejectsChangedLegacySeedBeforeWrites(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	legacyPath := filepath.Join(
		projectRoot,
		".quint",
		"quint.db",
	)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("create legacy root: %v", err)
	}
	if err := initializeDatabase(legacyPath); err != nil {
		t.Fatalf("initialize legacy database: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	file, err := os.OpenFile(
		legacyPath,
		os.O_WRONLY|os.O_APPEND,
		0,
	)
	if err != nil {
		t.Fatalf("open legacy seed for change: %v", err)
	}
	_, writeErr := file.Write([]byte("changed"))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf(
			"change legacy seed: write=%v close=%v",
			writeErr,
			closeErr,
		)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(
		context.Background(),
		plan,
	); err == nil {
		t.Fatal("changed legacy seed was applied")
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("stale seed precondition wrote .haft: %v", err)
	}
	if _, err := os.Stat(plan.DatabasePath()); !os.IsNotExist(err) {
		t.Fatalf("stale seed precondition wrote canonical database: %v", err)
	}
}

func TestPublicProjectCoreEffectPreservesExistingCoreCarriers(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create Haft root: %v", err)
	}
	configBytes := []byte(
		"schema_version: 1\nauthority:\n" +
			"  decision_binding_mode: strict_cli_speech_act\n" +
			"  project_typeenv_head_selection_mode: explicit_h_decide\n" +
			"  profile_declaration_mode: explicit_h_onboard\n",
	)
	workflowBytes := []byte(
		"# Workflow\n\n## Intent\n\nKeep custom workflow bytes.\n\n" +
			"## Defaults\n\n```yaml\nmode: standard\n" +
			"require_decision: true\nrequire_verify: true\n" +
			"allow_autonomy: false\n```\n",
	)
	configPath := filepath.Join(haftDir, "config.yaml")
	workflowPath := filepath.Join(haftDir, "workflow.md")
	if err := os.WriteFile(configPath, configBytes, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(workflowPath, workflowBytes, 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(
		context.Background(),
		plan,
	); err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	gotWorkflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(gotConfig) != string(configBytes) ||
		string(gotWorkflow) != string(workflowBytes) {
		t.Fatalf(
			"core carriers changed:\nconfig=%q\nworkflow=%q",
			gotConfig,
			gotWorkflow,
		)
	}
}

func TestPublicProjectCoreEffectRejectsChangedCoreCarrierBeforeWrites(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create Haft root: %v", err)
	}
	workflowPath := filepath.Join(haftDir, "workflow.md")
	if err := os.WriteFile(
		workflowPath,
		[]byte(project.ExampleWorkflowMarkdown()),
		0o644,
	); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if err := os.WriteFile(
		workflowPath,
		[]byte("changed after preview\n"),
		0o644,
	); err != nil {
		t.Fatalf("change workflow: %v", err)
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(
		context.Background(),
		plan,
	); err == nil {
		t.Fatal("changed core carrier was applied")
	}
	if _, err := os.Stat(
		filepath.Join(haftDir, "project.yaml"),
	); !os.IsNotExist(err) {
		t.Fatalf("stale core carrier wrote project identity: %v", err)
	}
	if _, err := os.Stat(plan.DatabasePath()); !os.IsNotExist(err) {
		t.Fatalf("stale core carrier wrote database: %v", err)
	}
}
