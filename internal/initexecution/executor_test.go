package initexecution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestExecutorPublishesHostsOnlyAfterExactCoreReceipt(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	coreApplied := false
	core := CoreEffectFunc(func(
		_ context.Context,
		plan initplanning.CoreProjectPlan,
	) (CoreEffectReceipt, error) {
		coreApplied = true
		return exactAppliedCoreReceipt(t, plan), nil
	})
	publisher := mustExecutionHostPublisher(t)
	host := HostPublicationFunc(func(
		batch initplanning.HostPublicationBatch,
		store initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error) {
		if !coreApplied {
			t.Fatal("host publication ran before the core receipt")
		}
		return publisher.Publish(batch, store)
	})
	executor, err := NewExecutor(core, host)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Kind() != InitExecutionApplied ||
		outcome.PartialEffectBoundary() != "" ||
		len(outcome.PendingHosts()) != 0 ||
		len(outcome.HostReceipts()) != 1 {
		t.Fatalf("execution outcome = %#v", outcome)
	}
	coreReceipt, exists := outcome.CoreReceipt()
	if !exists ||
		coreReceipt.ProjectRoot() != fixture.plan.Core().ProjectRoot() ||
		coreReceipt.DatabasePath() != fixture.plan.Core().DatabasePath() ||
		coreReceipt.BeforeSchema() != 53 ||
		coreReceipt.AfterSchema() != 54 {
		t.Fatalf("core receipt = %#v exists=%t", coreReceipt, exists)
	}
	hostReceipt := outcome.HostReceipts()[0]
	if hostReceipt.Host() != initplanning.HostCodex ||
		hostReceipt.BatchDigest() == "" ||
		hostReceipt.Outcome().Kind() != initfs.HostPublicationApplied {
		t.Fatalf("host receipt = %#v", hostReceipt)
	}
	if outcome.CoordinationPath() != fixture.coordinator.LockPath() ||
		outcome.ResourceDigest() == "" {
		t.Fatalf(
			"coordination receipt = %s %s",
			outcome.CoordinationPath(),
			outcome.ResourceDigest(),
		)
	}
	assertExecutionFile(t, fixture.carrierPath, fixture.desiredContent)
	manifest, err := fixture.store.Read()
	if err != nil {
		t.Fatalf("read published manifest: %v", err)
	}
	if manifest.Kind() != initfs.ManifestReadPresent {
		t.Fatalf("manifest read kind = %s", manifest.Kind())
	}
}

func TestExistingProjectCoreEffectVerifiesExactCurrentPlan(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	projectRoot := filepath.Join(projectParent, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	haftDir := filepath.Join(projectRoot, ".haft")
	t.Setenv("HOME", homeRoot)
	config := project.Config{
		ID:   "qnt_e3149c17",
		Name: "project",
	}
	if _, err := project.PersistExactConfig(
		haftDir,
		projectRoot,
		config,
	); err != nil {
		t.Fatalf("PersistExactConfig: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		projectRoot,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("BindInitialized: %v", err)
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}
	basis, err := initplanning.NewUnavailableBasis(
		"project_basis_unavailable: no current project TypeEnv head",
	)
	if err != nil {
		t.Fatalf("NewUnavailableBasis: %v", err)
	}
	plan, err := initplanning.NewCoreProjectPlanBuilder().
		ForProject(projectRoot, config.ID).
		AtDatabase(databasePath).
		WithSchemaTransition(
			initplanning.CoreVerifyCurrent,
			current,
			current,
		).
		WithBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("CoreProjectPlanBuilder.Build: %v", err)
	}

	receipt, err := (ExistingProjectCoreEffect{}).
		ApplyCore(context.Background(), plan)
	if err != nil {
		t.Fatalf("ApplyCore: %v", err)
	}
	if receipt.Outcome() != CoreEffectAlreadyCurrent ||
		receipt.Effect() != initplanning.CoreVerifyCurrent ||
		receipt.BeforeSchema() != current ||
		receipt.AfterSchema() != current {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestExecutorCoreFailureLeavesEveryHostEffectUnperformed(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	hostCalls := 0
	coreFailure := errors.New("core migration outcome is unavailable")
	executor, err := NewExecutor(
		CoreEffectFunc(func(
			context.Context,
			initplanning.CoreProjectPlan,
		) (CoreEffectReceipt, error) {
			return CoreEffectReceipt{}, coreFailure
		}),
		HostPublicationFunc(func(
			initplanning.HostPublicationBatch,
			initfs.ManifestStore,
		) (initfs.HostPublicationOutcome, error) {
			hostCalls++
			return initfs.HostPublicationOutcome{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if !errors.Is(err, coreFailure) {
		t.Fatalf("Execute error = %v, want %v", err, coreFailure)
	}
	if outcome.Kind() != InitExecutionCoreUnconfirmed ||
		hostCalls != 0 ||
		len(outcome.PendingHosts()) != 1 ||
		outcome.PendingHosts()[0] != initplanning.HostCodex {
		t.Fatalf("core-failure outcome = %#v hostCalls=%d", outcome, hostCalls)
	}
	if _, exists := outcome.CoreReceipt(); exists {
		t.Fatal("failed core effect minted a success receipt")
	}
	assertExecutionPathMissing(t, fixture.carrierPath)
	assertExecutionManifestMissing(t, fixture.store)
}

func TestExecutorRejectsMismatchedCoreReceiptBeforeHostEffects(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	hostCalls := 0
	core := CoreEffectFunc(func(
		_ context.Context,
		plan initplanning.CoreProjectPlan,
	) (CoreEffectReceipt, error) {
		receipt, err := NewCoreEffectReceipt(
			CoreEffectApplied,
			plan.Effect(),
			plan.ProjectRoot(),
			plan.ProjectID().String(),
			filepath.Join(plan.ProjectRoot(), ".haft", "another.db"),
			plan.BeforeSchema(),
			plan.AfterSchema(),
		)
		if err != nil {
			t.Fatalf("NewCoreEffectReceipt: %v", err)
		}
		return receipt, nil
	})
	executor, err := NewExecutor(
		core,
		HostPublicationFunc(func(
			initplanning.HostPublicationBatch,
			initfs.ManifestStore,
		) (initfs.HostPublicationOutcome, error) {
			hostCalls++
			return initfs.HostPublicationOutcome{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if err == nil ||
		outcome.Kind() != InitExecutionCoreUnconfirmed ||
		hostCalls != 0 {
		t.Fatalf("mismatched receipt outcome = %#v err=%v hostCalls=%d", outcome, err, hostCalls)
	}
	if _, exists := outcome.CoreReceipt(); !exists {
		t.Fatal("mismatched but real core receipt was discarded")
	}
	assertExecutionPathMissing(t, fixture.carrierPath)
	assertExecutionManifestMissing(t, fixture.store)
}

func TestExecutorReturnsExactPartialBoundaryAfterCoreAndHostDrift(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	if err := os.MkdirAll(filepath.Dir(fixture.carrierPath), 0o755); err != nil {
		t.Fatalf("create drift parent: %v", err)
	}
	foreign := []byte("local foreign carrier")
	if err := os.WriteFile(fixture.carrierPath, foreign, 0o644); err != nil {
		t.Fatalf("write drift carrier: %v", err)
	}
	coreApplied := false
	core := CoreEffectFunc(func(
		_ context.Context,
		plan initplanning.CoreProjectPlan,
	) (CoreEffectReceipt, error) {
		coreApplied = true
		return exactAppliedCoreReceipt(t, plan), nil
	})
	publisher := mustExecutionHostPublisher(t)
	host := HostPublicationFunc(func(
		batch initplanning.HostPublicationBatch,
		store initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error) {
		if !coreApplied {
			t.Fatal("host drift check ran before core")
		}
		return publisher.Publish(batch, store)
	})
	executor, err := NewExecutor(core, host)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if err != nil {
		t.Fatalf("Execute drifted host: %v", err)
	}
	if outcome.Kind() != InitExecutionHostIncomplete ||
		outcome.PartialEffectBoundary() != "core_applied_host_incomplete" ||
		len(outcome.PendingHosts()) != 1 ||
		outcome.PendingHosts()[0] != initplanning.HostCodex ||
		len(outcome.HostReceipts()) != 1 ||
		outcome.HostReceipts()[0].Outcome().Kind() != initfs.HostPublicationPreconditionChanged {
		t.Fatalf("partial host outcome = %#v", outcome)
	}
	assertExecutionFile(t, fixture.carrierPath, foreign)
	assertExecutionManifestMissing(t, fixture.store)
}

func TestHostManifestRegistryKeysSameHostByIndependentScope(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	projectStore, err := initfs.NewManifestStore(
		projectRoot,
		filepath.Join(projectRoot, "claude.project.json"),
		1<<20,
	)
	if err != nil {
		t.Fatalf("project manifest store: %v", err)
	}
	userStore, err := initfs.NewManifestStore(
		userRoot,
		filepath.Join(userRoot, "claude.user.json"),
		1<<20,
	)
	if err != nil {
		t.Fatalf("user manifest store: %v", err)
	}
	projectBindingID, err := initplanning.NewHostBindingID(
		initplanning.HostClaude,
		initplanning.ScopeProject,
	)
	if err != nil {
		t.Fatalf("project binding ID: %v", err)
	}
	userBindingID, err := initplanning.NewHostBindingID(
		initplanning.HostClaude,
		initplanning.ScopeUser,
	)
	if err != nil {
		t.Fatalf("user binding ID: %v", err)
	}
	projectBinding, err := NewHostManifestBinding(
		projectBindingID,
		projectStore,
	)
	if err != nil {
		t.Fatalf("project binding: %v", err)
	}
	userBinding, err := NewHostManifestBinding(
		userBindingID,
		userStore,
	)
	if err != nil {
		t.Fatalf("user binding: %v", err)
	}
	registry, err := NewHostManifestRegistry(
		[]HostManifestBinding{
			userBinding,
			projectBinding,
		},
	)
	if err != nil {
		t.Fatalf("NewHostManifestRegistry: %v", err)
	}
	if len(registry.stores) != 2 ||
		registry.stores[projectBindingID].Path() != projectStore.Path() ||
		registry.stores[userBindingID].Path() != userStore.Path() {
		t.Fatalf("registry = %#v", registry)
	}
}

func TestExecutorBlockedPlanPerformsNeitherCoreNorHostEffect(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, true)
	called := false
	core := CoreEffectFunc(func(
		context.Context,
		initplanning.CoreProjectPlan,
	) (CoreEffectReceipt, error) {
		called = true
		return CoreEffectReceipt{}, nil
	})
	host := HostPublicationFunc(func(
		initplanning.HostPublicationBatch,
		initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error) {
		called = true
		return initfs.HostPublicationOutcome{}, nil
	})
	executor, err := NewExecutor(core, host)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if err != nil {
		t.Fatalf("Execute blocked plan: %v", err)
	}
	if outcome.Kind() != InitExecutionPlanBlocked ||
		outcome.Reason() != "compiled_init_plan_has_preserved_conflicts" ||
		called {
		t.Fatalf("blocked outcome = %#v called=%t", outcome, called)
	}
	assertExecutionPathMissing(t, fixture.carrierPath)
	assertExecutionManifestMissing(t, fixture.store)
}

func TestExecutorBusyCoordinationPerformsNeitherCoreNorHostEffect(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	held, err := fixture.coordinator.TryAcquire(
		[]string{fixture.plan.Core().DatabasePath()},
	)
	if err != nil {
		t.Fatalf("hold publication coordination: %v", err)
	}
	lease, acquired := held.Lease()
	if !acquired {
		t.Fatal("could not hold publication coordination fixture")
	}
	defer lease.Release()
	called := false
	executor, err := NewExecutor(
		CoreEffectFunc(func(
			context.Context,
			initplanning.CoreProjectPlan,
		) (CoreEffectReceipt, error) {
			called = true
			return CoreEffectReceipt{}, nil
		}),
		HostPublicationFunc(func(
			initplanning.HostPublicationBatch,
			initfs.ManifestStore,
		) (initfs.HostPublicationOutcome, error) {
			called = true
			return initfs.HostPublicationOutcome{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		fixture.registry,
		fixture.coordinator,
	)
	if err != nil {
		t.Fatalf("Execute busy coordination: %v", err)
	}
	if outcome.Kind() != InitExecutionBusy ||
		outcome.Reason() != "publication_coordination_busy" ||
		outcome.ResourceDigest() == "" ||
		called {
		t.Fatalf("busy outcome = %#v called=%t", outcome, called)
	}
	assertExecutionPathMissing(t, fixture.carrierPath)
	assertExecutionManifestMissing(t, fixture.store)
}

type executionFixture struct {
	plan           initplanning.InitPlan
	registry       HostManifestRegistry
	store          initfs.ManifestStore
	coordinator    initfs.PublicationCoordinator
	carrierPath    string
	desiredContent []byte
}

func newExecutionFixture(
	t *testing.T,
	blocked bool,
) executionFixture {
	t.Helper()
	return newExecutionFixtureAtScope(
		t,
		blocked,
		initplanning.ScopeProject,
		"",
	)
}

func newExecutionFixtureAtScope(
	t *testing.T,
	blocked bool,
	scope initplanning.InstallScope,
	userHomeRoot string,
) executionFixture {
	t.Helper()
	root := t.TempDir()
	scopeRoot := root
	if scope == initplanning.ScopeUser {
		if userHomeRoot == "" {
			t.Fatal("user-scoped execution fixture needs a user home")
		}
		scopeRoot = userHomeRoot
	}
	projectID := "qnt_e3149c17"
	components, err := initplanning.ParseComponentSet(
		[]string{string(initplanning.ComponentSkills)},
	)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	intent, err := initplanning.ParseInitIntent(initplanning.WeakInitIntent{
		InvocationPolicy: string(initplanning.InvocationExplicit),
		ProjectRoot:      root,
		ProjectID:        projectID,
		Hosts: []initplanning.WeakHostSelection{{
			Host:       string(initplanning.HostCodex),
			Scope:      string(scope),
			Components: []string{string(initplanning.ComponentSkills)},
		}},
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	basis, err := initplanning.NewUnavailableBasis(
		"project TypeEnv basis is not selected",
	)
	if err != nil {
		t.Fatalf("NewUnavailableBasis: %v", err)
	}
	databasePath := filepath.Join(root, ".haft", "haft.db")
	core, err := initplanning.NewCoreProjectPlanBuilder().
		ForProject(root, projectID).
		AtDatabase(databasePath).
		WithSchemaTransition(initplanning.CoreMigrate, 53, 54).
		WithBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("CoreProjectPlanBuilder.Build: %v", err)
	}
	publication, err := initplanning.NewPublicationIdentity(
		initplanning.PublicationIdentityInput{
			HaftVersion:         "v9-test",
			ExecutablePath:      filepath.Join(root, "bin", "haft"),
			ExecutableDigest:    "sha256:" + strings.Repeat("a", 64),
			SkillBundleDigest:   "sha256:" + strings.Repeat("b", 64),
			KernelCatalogDigest: "sha256:" + strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatalf("NewPublicationIdentity: %v", err)
	}
	carrierPath := filepath.Join(
		scopeRoot,
		".agents",
		"skills",
		"h-reason",
		"SKILL.md",
	)
	desired := []byte("desired h-reason carrier")
	output, err := initplanning.NewRenderedOutput(
		carrierPath,
		initplanning.ComponentSkills,
		desired,
		0o644,
	)
	if err != nil {
		t.Fatalf("NewRenderedOutput: %v", err)
	}
	expectation, err := initplanning.ExpectMissing(carrierPath)
	if err != nil {
		t.Fatalf("ExpectMissing: %v", err)
	}
	recovery, err := initplanning.NewRecoveryOperation(
		[]string{"haft", "init", "--check", "--host", "codex"},
	)
	if err != nil {
		t.Fatalf("NewRecoveryOperation: %v", err)
	}
	hostBuilder := initplanning.NewHostAdapterInstallPlanBuilder(
		initplanning.HostCodex,
	).
		AtEdition("codex.skills.v1").
		PublishedFrom(publication).
		ForProject(root, projectID).
		WithSelection(scope, components).
		AddTargetRoot(filepath.Join(scopeRoot, ".agents")).
		AddOutput(expectation, output).
		RecoverWith(recovery)
	if blocked {
		conflict, err := initplanning.NewForeignConflict(
			carrierPath,
			"preserve a foreign carrier",
		)
		if err != nil {
			t.Fatalf("NewForeignConflict: %v", err)
		}
		hostBuilder = hostBuilder.AddConflict(conflict)
	}
	hostPlan, err := hostBuilder.Build()
	if err != nil {
		t.Fatalf("HostAdapterInstallPlanBuilder.Build: %v", err)
	}
	capability, err := initplanning.NewAdapterCapabilityBuilder(
		initplanning.HostCodex,
	).
		AtEdition("codex.skills.v1").
		Allow(scope, components).
		Build()
	if err != nil {
		t.Fatalf("AdapterCapabilityBuilder.Build: %v", err)
	}
	catalog, err := initplanning.NewAdapterCatalog(
		[]initplanning.AdapterCapability{capability},
	)
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	plan, err := initplanning.CompileInitPlan(
		intent,
		core,
		[]initplanning.HostAdapterInstallPlan{hostPlan},
		catalog,
	)
	if err != nil {
		t.Fatalf("CompileInitPlan: %v", err)
	}
	manifestPath := filepath.Join(
		scopeRoot,
		".haft",
		"host-installations",
		"codex."+string(scope)+".json",
	)
	store, err := initfs.NewManifestStore(scopeRoot, manifestPath, 1<<20)
	if err != nil {
		t.Fatalf("NewManifestStore: %v", err)
	}
	binding, err := NewHostManifestBinding(
		hostPlan.BindingID(),
		store,
	)
	if err != nil {
		t.Fatalf("NewHostManifestBinding: %v", err)
	}
	registry, err := NewHostManifestRegistry(
		[]HostManifestBinding{binding},
	)
	if err != nil {
		t.Fatalf("NewHostManifestRegistry: %v", err)
	}
	coordinator, err := initfs.NewPublicationCoordinator(
		scopeRoot,
		filepath.Join(
			scopeRoot,
			".haft",
			"host-installations",
			"publication.lock",
		),
	)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator: %v", err)
	}
	return executionFixture{
		plan:           plan,
		registry:       registry,
		store:          store,
		coordinator:    coordinator,
		carrierPath:    carrierPath,
		desiredContent: desired,
	}
}

func exactAppliedCoreReceipt(
	t *testing.T,
	plan initplanning.CoreProjectPlan,
) CoreEffectReceipt {
	t.Helper()
	receipt, err := NewCoreEffectReceipt(
		CoreEffectApplied,
		plan.Effect(),
		plan.ProjectRoot(),
		plan.ProjectID().String(),
		plan.DatabasePath(),
		plan.BeforeSchema(),
		plan.AfterSchema(),
	)
	if err != nil {
		t.Fatalf("NewCoreEffectReceipt: %v", err)
	}
	return receipt
}

func mustExecutionHostPublisher(t *testing.T) initfs.HostPublisher {
	t.Helper()
	publisher, err := initfs.NewHostPublisher(1 << 20)
	if err != nil {
		t.Fatalf("NewHostPublisher: %v", err)
	}
	return publisher
}

func assertExecutionPathMissing(
	t *testing.T,
	path string,
) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("path %s exists or returned another error: %v", path, err)
	}
}

func assertExecutionManifestMissing(
	t *testing.T,
	store initfs.ManifestStore,
) {
	t.Helper()
	result, err := store.Read()
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if result.Kind() != initfs.ManifestReadMissing {
		t.Fatalf("manifest read kind = %s", result.Kind())
	}
}

func assertExecutionFile(
	t *testing.T,
	path string,
	want []byte,
) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("file %s = %q, want %q", path, got, want)
	}
}
