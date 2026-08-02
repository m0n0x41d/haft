package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestPublicProjectCoreEffectConsumesOnlyUneditedGeneratedProfileReview(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	writePublicBootstrapFixture(t, projectRoot, "go.mod", "module example.test/app\n")
	writePublicBootstrapFixture(t, projectRoot, "internal/kernel.go", "package internal\n")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	suggestion, err := inspectPublicProfileSuggestion(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileReviewCandidate(projectRoot, suggestion); err != nil {
		t.Fatal(err)
	}
	reviewPath := profileDeclarationReviewPath(projectRoot)
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_4c8e2a6d")
	plan, err := compilePublicCorePlan(context.Background(), request, homeRoot)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if plan.InitialProfileBootstrap().Kind() !=
		initplanning.InitialProfileApplySingleton {
		t.Fatalf("generated review bootstrap = %s", plan.InitialProfileBootstrap().Kind())
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(context.Background(), plan); err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	if _, err := os.Stat(reviewPath); !os.IsNotExist(err) {
		t.Fatalf("admitted generated review was not removed: %v", err)
	}
}

func TestPublicProfileBootstrapRequiresReviewForEnrichedGeneratedCarrier(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	writePublicBootstrapFixture(t, projectRoot, "go.mod", "module example.test/app\n")
	writePublicBootstrapFixture(t, projectRoot, "internal/kernel.go", "package internal\n")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	suggestion, err := inspectPublicProfileSuggestion(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileReviewCandidate(projectRoot, suggestion); err != nil {
		t.Fatal(err)
	}
	reviewPath := profileDeclarationReviewPath(projectRoot)
	reviewBytes, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatal(err)
	}
	review := map[string]any{}
	if err := json.Unmarshal(reviewBytes, &review); err != nil {
		t.Fatal(err)
	}
	review["operator_note"] = "Keep this human assessment."
	enriched, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	enriched = append(enriched, '\n')
	if err := os.WriteFile(reviewPath, enriched, 0o644); err != nil {
		t.Fatal(err)
	}
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_6a2e4c8d")
	plan, err := compilePublicCorePlan(context.Background(), request, homeRoot)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	bootstrap := plan.InitialProfileBootstrap()
	if bootstrap.Kind() != initplanning.InitialProfileHumanReviewRequired ||
		bootstrap.Reason() != "human_or_foreign_review" {
		t.Fatalf("enriched review bootstrap = %s reason=%s", bootstrap.Kind(), bootstrap.Reason())
	}
	if _, err := os.Stat(plan.DatabasePath()); !os.IsNotExist(err) {
		t.Fatalf("planning enriched review mutated ledger: %v", err)
	}
}

func TestPublicProjectCoreEffectAutomaticallyBootstrapsSoftwareProfile(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	writePublicBootstrapFixture(t, projectRoot, "go.mod", "module example.test/app\n")
	writePublicBootstrapFixture(
		t,
		projectRoot,
		"internal/kernel.go",
		"package internal\n\nfunc Ready() bool { return true }\n",
	)
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_e3149c17")
	plan, err := compilePublicCorePlan(context.Background(), request, homeRoot)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if plan.InitialProfileBootstrap().Kind() !=
		initplanning.InitialProfileApplySingleton {
		t.Fatalf("profile bootstrap = %s", plan.InitialProfileBootstrap().Kind())
	}
	output := &bytes.Buffer{}
	effect := newPublicProjectCoreEffect(request, output)
	if _, err := effect.ApplyCore(context.Background(), plan); err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	inspection, err := executeProfileInspection(
		context.Background(),
		projectRoot,
		false,
	)
	if err != nil {
		t.Fatalf("executeProfileInspection: %v", err)
	}
	if inspection.CanonicalProfile.Origin != "detector_default" ||
		len(inspection.CanonicalProfile.Scopes) != 1 ||
		inspection.CanonicalProfile.Scopes[0].RealizationKind != "software" {
		t.Fatalf("canonical profile = %#v", inspection.CanonicalProfile)
	}
	methodCarrierFound := false
	methodCarrierPath := ""
	for _, carrier := range plan.InitialProfileBootstrap().ContingentFileEffects() {
		if _, err := os.Stat(carrier.Path()); err != nil {
			t.Fatalf("profile carrier %s: %v", carrier.Path(), err)
		}
		isMethodCarrier := strings.Contains(
			filepath.ToSlash(carrier.Path()),
			"/.haft/methods/",
		)
		methodCarrierFound = methodCarrierFound || isMethodCarrier
		if isMethodCarrier {
			methodCarrierPath = carrier.Path()
		}
	}
	if !methodCarrierFound {
		t.Fatal("software bootstrap installed no MethodPack carrier")
	}
	if !strings.Contains(output.String(), "origin=detector_default") {
		t.Fatalf("init output = %q", output.String())
	}
	if err := os.Remove(methodCarrierPath); err != nil {
		t.Fatalf("remove MethodPack carrier for recovery fixture: %v", err)
	}
	repeatPlan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatalf("compile repeat init: %v", err)
	}
	if repeatPlan.InitialProfileBootstrap().Kind() !=
		initplanning.InitialProfileKeepExisting {
		t.Fatalf("repeat bootstrap = %s", repeatPlan.InitialProfileBootstrap().Kind())
	}
	repeatOutput := &bytes.Buffer{}
	repeatEffect := newPublicProjectCoreEffect(request, repeatOutput)
	if _, err := repeatEffect.ApplyCore(
		context.Background(),
		repeatPlan,
	); err != nil {
		t.Fatalf("repeat ApplyCore: %v", err)
	}
	if !strings.Contains(repeatOutput.String(), "origin=detector_default") {
		t.Fatalf("repeat init output = %q", repeatOutput.String())
	}
	if _, err := os.Stat(methodCarrierPath); err != nil {
		t.Fatalf("repeat init did not recover MethodPack carrier: %v", err)
	}
	binding := mustOnboardProjectBinding(t, projectRoot)
	surface, err := openSealedProjectOnboardSurface(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer surface.Close()
	statusOutput := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"status"}`,
	)
	status := decodeOnboardResponse(t, statusOutput)
	if status.ProfileOrigin != "detector_default" ||
		!status.ProfileOverrideEligible {
		t.Fatalf("detector-default onboarding status = %#v", status)
	}
	prepareOutput := callOnboardHandler(
		t,
		surface.Handler(),
		`{"action":"profile_prepare"}`,
	)
	prepared := decodeOnboardResponse(t, prepareOutput)
	if prepared.Result != "profile_review_prepared" ||
		prepared.ProfileOrigin != "detector_default" ||
		!prepared.ProfileOverrideEligible ||
		prepared.Effects.CanonicalProfileChanged {
		t.Fatalf("detector-default explicit review = %#v", prepared)
	}
}

func TestPublicProjectCoreEffectAutomaticallyBootstrapsDocumentsProfile(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	writePublicBootstrapFixture(t, projectRoot, "notes/one.md", "# One\n")
	writePublicBootstrapFixture(t, projectRoot, "proposal.mdx", "# Two\n")
	writePublicBootstrapFixture(t, projectRoot, "papers/three.rst", "Three\n")
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_2b6c8d4e")
	plan, err := compilePublicCorePlan(context.Background(), request, homeRoot)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	if plan.InitialProfileBootstrap().Kind() !=
		initplanning.InitialProfileApplySingleton {
		t.Fatalf("profile bootstrap = %s", plan.InitialProfileBootstrap().Kind())
	}
	for _, carrier := range plan.InitialProfileBootstrap().ContingentFileEffects() {
		if strings.Contains(filepath.ToSlash(carrier.Path()), "/.haft/methods/") {
			t.Fatalf("docs-only bootstrap planned MethodPack carrier %s", carrier.Path())
		}
	}
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(context.Background(), plan); err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	inspection, err := executeProfileInspection(
		context.Background(),
		projectRoot,
		false,
	)
	if err != nil {
		t.Fatalf("executeProfileInspection: %v", err)
	}
	if inspection.CanonicalProfile.Origin != "detector_default" ||
		len(inspection.CanonicalProfile.Scopes) != 1 ||
		inspection.CanonicalProfile.Scopes[0].RealizationKind != "non_software" {
		t.Fatalf("canonical profile = %#v", inspection.CanonicalProfile)
	}
}

func TestPublicProjectCoreEffectRejectsProfileDetectorTOCTOUBeforeWrites(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	writePublicBootstrapFixture(t, projectRoot, "go.mod", "module example.test/app\n")
	writePublicBootstrapFixture(t, projectRoot, "internal/kernel.go", "package internal\n")
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_8e4d2b6c")
	plan, err := compilePublicCorePlan(context.Background(), request, homeRoot)
	if err != nil {
		t.Fatalf("compilePublicCorePlan: %v", err)
	}
	writePublicBootstrapFixture(
		t,
		projectRoot,
		"internal/kernel.go",
		"package internal\n\nvar Changed = true\n",
	)
	effect := newPublicProjectCoreEffect(request, io.Discard)
	if _, err := effect.ApplyCore(context.Background(), plan); err == nil {
		t.Fatal("changed detector observation was admitted")
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".haft")); !os.IsNotExist(err) {
		t.Fatalf("TOCTOU rejection wrote .haft: %v", err)
	}
}

func mustResolvedTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func mustPublicCoreOnlyRequest(
	t *testing.T,
	projectRoot string,
	projectID string,
) publicInitRequest {
	t.Helper()
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
		t.Fatal(err)
	}
	return request
}

func writePublicBootstrapFixture(
	t *testing.T,
	projectRoot string,
	relativePath string,
	content string,
) {
	t.Helper()
	path := filepath.Join(projectRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

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
	if len(plan.FileEffects()) != 9 {
		t.Fatalf(
			"legacy carrier effects = %d, want 9",
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
	configBytes := []byte("operator_owned: true\n")
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
	output := &bytes.Buffer{}
	effect := newPublicProjectCoreEffect(request, output)
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
	if !strings.Contains(
		output.String(),
		"preserved byte-for-byte and ignored",
	) {
		t.Fatalf("legacy warning = %q", output.String())
	}
}

func TestPublicProjectCoreEffectRemovesExactGeneratedLegacyConfig(
	t *testing.T,
) {
	homeRoot := mustResolvedTempDir(t)
	projectRoot := mustResolvedTempDir(t)
	t.Setenv("HOME", homeRoot)
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	encoded := "IyBIYWZ0IHByb2plY3QgYmVoYXZpb3IgY29uZmlndXJhdGlvbgpzY2hlbWFfdmVyc2lvbjogMQphdXRob3JpdHk6CiAgIyBEZWNpc2lvblJlY29yZDogZXhwbGljaXRfaF9kZWNpZGUgKGRlZmF1bHQpIHwgc3RyaWN0X2NsaV9zcGVlY2hfYWN0IChvcHQtaW4pCiAgZGVjaXNpb25fYmluZGluZ19tb2RlOiBleHBsaWNpdF9oX2RlY2lkZQogICMgUHJvamVjdCBwcm9maWxlOiBleHBsaWNpdF9oX29uYm9hcmQgKGRlZmF1bHQpIHwgc3RyaWN0X2NsaV9zcGVlY2hfYWN0IChyZXNlcnZlZDsgZmFpbHMgY2xvc2VkIHdpdGhvdXQgbmF0aXZlIHYzIHN0cmljdCBhdXRob3JpdHkpCiAgcHJvZmlsZV9kZWNsYXJhdGlvbl9tb2RlOiBleHBsaWNpdF9oX29uYm9hcmQK"
	legacy, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	configPath := project.ProjectConfigPath(haftDir)
	if err := os.WriteFile(configPath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	request := mustPublicCoreOnlyRequest(t, projectRoot, "qnt_e3149c17")
	plan, err := compilePublicCorePlan(
		context.Background(),
		request,
		homeRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	effect := newPublicProjectCoreEffect(request, output)
	if _, err := effect.ApplyCore(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("generated legacy config still exists: %v", err)
	}
	if !strings.Contains(output.String(), "Legacy project config removed") {
		t.Fatalf("removal report = %q", output.String())
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
