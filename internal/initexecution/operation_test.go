package initexecution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPreparedHostInitOperationAppliesOnlyItsExactReviewedPreview(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	userHomeRoot := t.TempDir()
	prepared, err := PrepareHostInitOperation(
		fixture.plan,
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
	changed := preview
	changed.ApplyOrder = []string{"host_adapter_fanout", "core_project"}
	if _, err := prepared.ConfirmPreview(changed); err == nil {
		t.Fatal("changed preview was confirmed")
	}
	assertExecutionPathMissing(t, fixture.carrierPath)
	assertExecutionPathMissing(
		t,
		filepath.Join(
			fixture.plan.Core().ProjectRoot(),
			".haft",
			"host-installations",
			"codex.project.json",
		),
	)
	assertExecutionPathMissing(
		t,
		filepath.Join(
			userHomeRoot,
			".haft",
			"host-installations",
			"publication.lock",
		),
	)

	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	executor, err := NewExecutor(
		CoreEffectFunc(func(
			_ context.Context,
			plan initplanning.CoreProjectPlan,
		) (CoreEffectReceipt, error) {
			return exactAppliedCoreReceipt(t, plan), nil
		}),
		mustExecutionHostPublisher(t),
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := confirmed.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.Kind() != InitExecutionApplied ||
		len(outcome.HostReceipts()) != 1 {
		t.Fatalf("execution outcome = %#v", outcome)
	}
	assertExecutionFile(t, fixture.carrierPath, fixture.desiredContent)
}

func TestPreparedCoreOnlyInitOperationCreatesNoHostPublicationCarrier(
	t *testing.T,
) {
	root := t.TempDir()
	plan := mustCoreOnlyExecutionPlan(t, root)
	prepared, err := PrepareCoreOnlyInitOperation(plan)
	if err != nil {
		t.Fatalf("PrepareCoreOnlyInitOperation: %v", err)
	}
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}

	hostCalls := 0
	executor, err := NewExecutor(
		CoreEffectFunc(func(
			_ context.Context,
			core initplanning.CoreProjectPlan,
		) (CoreEffectReceipt, error) {
			return NewCoreEffectReceipt(
				CoreEffectAlreadyCurrent,
				core.Effect(),
				core.ProjectRoot(),
				core.ProjectID().String(),
				core.DatabasePath(),
				core.BeforeSchema(),
				core.AfterSchema(),
			)
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
	outcome, err := confirmed.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.Kind() != InitExecutionAlreadyCurrent ||
		hostCalls != 0 ||
		len(outcome.HostReceipts()) != 0 ||
		outcome.CoordinationPath() != "" ||
		outcome.ResourceDigest() != "" {
		t.Fatalf(
			"core-only outcome = %#v hostCalls=%d",
			outcome,
			hostCalls,
		)
	}
	hostRoot := filepath.Join(root, ".haft", "host-installations")
	if _, err := os.Lstat(hostRoot); !os.IsNotExist(err) {
		t.Fatalf("core-only apply created host publication root: %v", err)
	}
}

func TestPreparedInitOperationRejectsWrongModeAndBlockedConfirmation(
	t *testing.T,
) {
	hostFixture := newExecutionFixture(t, false)
	if _, err := PrepareCoreOnlyInitOperation(hostFixture.plan); err == nil {
		t.Fatal("host plan entered core-only preparation")
	}

	corePlan := mustCoreOnlyExecutionPlan(t, t.TempDir())
	if _, err := PrepareHostInitOperation(
		corePlan,
		t.TempDir(),
		1<<20,
	); err == nil {
		t.Fatal("core-only plan entered host preparation")
	}

	blocked := newExecutionFixture(t, true)
	prepared, err := PrepareHostInitOperation(
		blocked.plan,
		t.TempDir(),
		1<<20,
	)
	if err != nil {
		t.Fatalf("PrepareHostInitOperation blocked: %v", err)
	}
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview blocked: %v", err)
	}
	if _, err := prepared.ConfirmPreview(preview); err == nil ||
		!strings.Contains(err.Error(), "blocked") {
		t.Fatalf("blocked preview confirmation error = %v", err)
	}
}

func mustCoreOnlyExecutionPlan(
	t *testing.T,
	root string,
) initplanning.InitPlan {
	t.Helper()
	projectID := "qnt_e3149c17"
	intent, err := initplanning.ParseInitIntent(initplanning.WeakInitIntent{
		InvocationPolicy: string(initplanning.InvocationExplicit),
		ProjectRoot:      root,
		ProjectID:        projectID,
	})
	if err != nil {
		t.Fatalf("ParseInitIntent: %v", err)
	}
	basis, err := initplanning.NewSelectedBasis(
		"project-typeenv-head:"+projectID,
		2,
		"typeenv:sha256:"+strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("NewSelectedBasis: %v", err)
	}
	core, err := initplanning.NewCoreProjectPlanBuilder().
		ForProject(root, projectID).
		AtDatabase(filepath.Join(root, ".haft", "haft.db")).
		WithSchemaTransition(initplanning.CoreVerifyCurrent, 54, 54).
		WithBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("CoreProjectPlanBuilder.Build: %v", err)
	}
	components, err := initplanning.ParseComponentSet(
		[]string{string(initplanning.ComponentMCP)},
	)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	capability, err := initplanning.NewAdapterCapabilityBuilder(
		initplanning.HostCodex,
	).
		AtEdition("codex.v1").
		Allow(initplanning.ScopeProject, components).
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
		nil,
		catalog,
	)
	if err != nil {
		t.Fatalf("CompileInitPlan: %v", err)
	}
	return plan
}
