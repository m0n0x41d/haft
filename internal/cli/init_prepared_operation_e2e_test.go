package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCurrentCodexProjectionRunsThroughExactPreviewAndIdempotentApply(
	t *testing.T,
) {
	projectRoot := filepath.Join(t.TempDir(), "downstream")
	userHomeRoot := t.TempDir()
	projectID := "qnt_e3149c17"
	t.Setenv("HOME", userHomeRoot)
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create downstream root: %v", err)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	runtime.userHomeRoot = userHomeRoot
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	projection, err := buildCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		initplanning.HostCodex,
		initplanning.ScopeProject,
		candidates,
		bundle,
		publication,
		runtime,
	)
	if err != nil {
		t.Fatalf("buildCurrentCoherentHostProjection: %v", err)
	}
	core := currentPreparedOperationCorePlan(t, projectRoot, projectID)
	catalog := currentPreparedOperationCatalog(t, projection)
	store := currentPreparedOperationManifestStore(
		t,
		projectRoot,
		projectID,
		userHomeRoot,
	)
	inspector, err := initfs.NewHostStatusInspector(1 << 20)
	if err != nil {
		t.Fatalf("NewHostStatusInspector: %v", err)
	}
	firstCurrentness, err := inspector.InspectCoherentCurrentness(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectCoherentCurrentness first: %v", err)
	}
	firstPlan := compileCurrentPreparedOperationPlan(
		t,
		core,
		catalog,
		firstCurrentness,
	)
	first := prepareAndConfirmCurrentOperation(
		t,
		firstPlan,
		userHomeRoot,
	)
	executor := currentPreparedOperationExecutor(t)
	firstOutcome, err := first.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if firstOutcome.Kind() != initexecution.InitExecutionApplied ||
		len(firstOutcome.HostReceipts()) != 1 {
		t.Fatalf("first outcome = %#v", firstOutcome)
	}

	inspection, err := inspector.InspectCoherentBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectCoherentBinding: %v", err)
	}
	if inspection.Status().Posture !=
		initplanning.HostInstallationCurrent {
		t.Fatalf("installed posture = %#v", inspection.Status())
	}

	secondCurrentness, err := inspector.InspectCoherentCurrentness(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectCoherentCurrentness second: %v", err)
	}
	secondPlan := compileCurrentPreparedOperationPlan(
		t,
		core,
		catalog,
		secondCurrentness,
	)
	second := prepareAndConfirmCurrentOperation(
		t,
		secondPlan,
		userHomeRoot,
	)
	snapshotPaths := []string{store.Path()}
	for _, fragment := range projection.ManagedFragments() {
		snapshotPaths = append(
			snapshotPaths,
			fragment.Coordinate().CarrierPath(),
		)
	}
	before := snapshotHostStatusFixture(
		t,
		snapshotPaths,
		projection.Outputs(),
	)
	secondOutcome, err := second.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	after := snapshotHostStatusFixture(
		t,
		snapshotPaths,
		projection.Outputs(),
	)
	if secondOutcome.Kind() !=
		initexecution.InitExecutionAlreadyCurrent {
		t.Fatalf("second outcome = %#v", secondOutcome)
	}
	if !slices.Equal(before, after) {
		t.Fatal("idempotent current apply changed downstream carriers")
	}
}

func currentPreparedOperationCorePlan(
	t *testing.T,
	projectRoot string,
	projectID string,
) initplanning.CoreProjectPlan {
	t.Helper()
	basis, err := initplanning.NewSelectedBasis(
		"project-typeenv-head:"+projectID,
		2,
		"typeenv:sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("NewSelectedBasis: %v", err)
	}
	plan, err := initplanning.NewCoreProjectPlanBuilder().
		ForProject(projectRoot, projectID).
		AtDatabase(filepath.Join(projectRoot, ".haft", "haft.db")).
		WithSchemaTransition(initplanning.CoreVerifyCurrent, 54, 54).
		WithBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("CoreProjectPlanBuilder.Build: %v", err)
	}
	return plan
}

func currentPreparedOperationCatalog(
	t *testing.T,
	projection initplanning.HostAdapterProjection,
) initplanning.AdapterCatalog {
	t.Helper()
	capability, err := initplanning.NewAdapterCapabilityBuilder(
		projection.Host(),
	).
		AtEdition(projection.Edition()).
		Allow(projection.Scope(), projection.Components()).
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
	return catalog
}

func compileCurrentPreparedOperationPlan(
	t *testing.T,
	core initplanning.CoreProjectPlan,
	catalog initplanning.AdapterCatalog,
	currentness initplanning.CoherentInstallationCurrentness,
) initplanning.InitPlan {
	t.Helper()
	hostPlan, err :=
		initplanning.CompileCoherentHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileCoherentHostAdapterReconciliation: %v", err)
	}
	componentValues := hostPlan.Components().Values()
	components := make([]string, len(componentValues))
	for index, component := range componentValues {
		components[index] = string(component)
	}
	intent, err := initplanning.ParseInitIntent(initplanning.WeakInitIntent{
		InvocationPolicy: string(initplanning.InvocationExplicit),
		ProjectRoot:      core.ProjectRoot(),
		ProjectID:        core.ProjectID().String(),
		Hosts: []initplanning.WeakHostSelection{{
			Host:       string(hostPlan.Host()),
			Scope:      string(hostPlan.Scope()),
			Components: components,
		}},
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
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
	return plan
}

func prepareAndConfirmCurrentOperation(
	t *testing.T,
	plan initplanning.InitPlan,
	userHomeRoot string,
) initexecution.ConfirmedInitOperation {
	t.Helper()
	prepared, err := initexecution.PrepareHostInitOperation(
		plan,
		userHomeRoot,
		1<<20,
	)
	if err != nil {
		t.Fatalf("PrepareHostInitOperation: %v", err)
	}
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	return confirmed
}

func currentPreparedOperationExecutor(
	t *testing.T,
) initexecution.Executor {
	t.Helper()
	publisher, err := initfs.NewHostPublisher(1 << 20)
	if err != nil {
		t.Fatalf("NewHostPublisher: %v", err)
	}
	executor, err := initexecution.NewExecutor(
		initexecution.CoreEffectFunc(func(
			_ context.Context,
			core initplanning.CoreProjectPlan,
		) (initexecution.CoreEffectReceipt, error) {
			return initexecution.NewCoreEffectReceipt(
				initexecution.CoreEffectAlreadyCurrent,
				core.Effect(),
				core.ProjectRoot(),
				core.ProjectID().String(),
				core.DatabasePath(),
				core.BeforeSchema(),
				core.AfterSchema(),
			)
		}),
		publisher,
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	return executor
}

func currentPreparedOperationManifestStore(
	t *testing.T,
	projectRoot string,
	projectID string,
	userHomeRoot string,
) initfs.ManifestStore {
	t.Helper()
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projectRoot,
			ProjectID:    projectID,
			UserHomeRoot: userHomeRoot,
		},
	)
	if err != nil {
		t.Fatalf("NewPublicationLayout: %v", err)
	}
	location, err := layout.ManifestLocation(
		initplanning.HostCodex,
		initplanning.ScopeProject,
	)
	if err != nil {
		t.Fatalf("ManifestLocation: %v", err)
	}
	store, err := initfs.NewManifestStore(
		location.Root(),
		location.Path(),
		1<<20,
	)
	if err != nil {
		t.Fatalf("NewManifestStore: %v", err)
	}
	return store
}
