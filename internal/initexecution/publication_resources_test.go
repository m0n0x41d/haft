package initexecution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCanonicalPublicationResourcesDriveExactExecutorLocations(
	t *testing.T,
) {
	fixture := newExecutionFixture(t, false)
	userHomeRoot := t.TempDir()
	registry, coordinator, err := BindCanonicalPublicationResources(
		fixture.plan,
		userHomeRoot,
		1<<20,
	)
	if err != nil {
		t.Fatalf("BindCanonicalPublicationResources: %v", err)
	}

	projectRoot := fixture.plan.Core().ProjectRoot()
	manifestPath := filepath.Join(
		projectRoot,
		".haft",
		"host-installations",
		"codex.project.json",
	)
	binding := fixture.plan.Hosts()[0].BindingID()
	store := registry.stores[binding]
	if store.Root() != projectRoot ||
		store.Path() != manifestPath ||
		store.LockPath() != manifestPath+".lock" ||
		store.JournalPath() != manifestPath+".pending" {
		t.Fatalf("canonical store = %#v", store)
	}
	if coordinator.Root() != userHomeRoot ||
		coordinator.LockPath() != filepath.Join(
			userHomeRoot,
			".haft",
			"host-installations",
			"publication.lock",
		) {
		t.Fatalf("canonical coordinator = %#v", coordinator)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("binding factory changed manifest path: %v", err)
	}
	if _, err := os.Lstat(coordinator.LockPath()); !os.IsNotExist(err) {
		t.Fatalf("binding factory changed coordination path: %v", err)
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
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		registry,
		coordinator,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if outcome.Kind() != InitExecutionApplied ||
		outcome.CoordinationPath() != coordinator.LockPath() {
		t.Fatalf("execution outcome = %#v", outcome)
	}
	read, err := store.Read()
	if err != nil {
		t.Fatalf("read canonical manifest: %v", err)
	}
	if read.Kind() != initfs.ManifestReadPresent ||
		read.Manifest().ProjectID() != fixture.plan.Core().ProjectID().String() ||
		read.Manifest().Host() != initplanning.HostCodex ||
		read.Manifest().Scope() != initplanning.ScopeProject {
		t.Fatalf("canonical manifest = %#v", read)
	}
	if _, err := os.Lstat(store.JournalPath()); !os.IsNotExist(err) {
		t.Fatalf("completed execution retained journal: %v", err)
	}
}

func TestCanonicalPublicationResourcesRejectWeakUserHaftRoot(t *testing.T) {
	fixture := newExecutionFixture(t, false)
	if _, _, err := BindCanonicalPublicationResources(
		fixture.plan,
		".haft",
		1<<20,
	); err == nil {
		t.Fatal("relative user Haft root was accepted")
	}
}

func TestCanonicalPublicationResourcesUseOneUserScopedConflictCarrier(
	t *testing.T,
) {
	userHomeRoot := t.TempDir()
	fixture := newExecutionFixtureAtScope(
		t,
		false,
		initplanning.ScopeUser,
		userHomeRoot,
	)
	registry, coordinator, err := BindCanonicalPublicationResources(
		fixture.plan,
		userHomeRoot,
		1<<20,
	)
	if err != nil {
		t.Fatalf("BindCanonicalPublicationResources: %v", err)
	}
	binding := fixture.plan.Hosts()[0].BindingID()
	store := registry.stores[binding]
	wantManifest := filepath.Join(
		userHomeRoot,
		".haft",
		"host-installations",
		"codex.user.json",
	)
	if store.Root() != userHomeRoot ||
		store.Path() != wantManifest ||
		coordinator.Root() != userHomeRoot {
		t.Fatalf(
			"user resources = store %#v coordinator %#v",
			store,
			coordinator,
		)
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
	outcome, err := executor.Execute(
		context.Background(),
		fixture.plan,
		registry,
		coordinator,
	)
	if err != nil {
		t.Fatalf("Execute user scope: %v", err)
	}
	if outcome.Kind() != InitExecutionApplied {
		t.Fatalf("user-scoped execution = %#v", outcome)
	}
	read, err := store.Read()
	if err != nil {
		t.Fatalf("read user manifest: %v", err)
	}
	if read.Kind() != initfs.ManifestReadPresent ||
		read.Manifest().Scope() != initplanning.ScopeUser {
		t.Fatalf("user manifest = %#v", read)
	}
}

func TestCanonicalPublicationResourcesRejectUserTargetOutsideSuppliedHome(
	t *testing.T,
) {
	targetHome := t.TempDir()
	fixture := newExecutionFixtureAtScope(
		t,
		false,
		initplanning.ScopeUser,
		targetHome,
	)
	anotherHome := t.TempDir()
	if _, _, err := BindCanonicalPublicationResources(
		fixture.plan,
		anotherHome,
		1<<20,
	); err == nil {
		t.Fatal("user target outside supplied home was accepted")
	}
}
