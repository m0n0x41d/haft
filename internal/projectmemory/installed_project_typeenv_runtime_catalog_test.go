package projectmemory_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	_ "modernc.org/sqlite"
)

func TestInstalledProjectTypeEnvRuntimeCatalogDispatchesOnlyByExactX(
	t *testing.T,
) {
	t.Parallel()
	target := currentLocalPracticeTarget(t)
	entry, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		target.RuntimeBasis(),
		target.InstalledRuntime(),
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeEntry() error = %v", err)
	}
	catalog, err := projectmemory.NewInstalledProjectTypeEnvRuntimeCatalog(
		[]projectmemory.InstalledProjectTypeEnvRuntimeEntry{entry},
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeCatalog() error = %v", err)
	}

	installed, present := catalog.Lookup(target.RuntimeBasis().Ref())
	if !present || len(installed.MechanismCatalogs) == 0 {
		t.Fatal("exact X lookup returned no installed callable surface")
	}
	unknown, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(empty) error = %v", err)
	}
	_, present = catalog.Lookup(unknown.Ref())
	if present {
		t.Fatal("unknown X lookup used an implicit fallback")
	}
}

func TestInstalledProjectTypeEnvRuntimeCatalogRejectsDriftAndDuplicateX(
	t *testing.T,
) {
	t.Parallel()
	target := currentLocalPracticeTarget(t)
	drifted := target.InstalledRuntime()
	drifted.MechanismCatalogs = nil
	_, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		target.RuntimeBasis(),
		drifted,
	)
	if !errors.Is(err, projectmemory.ErrInstalledProjectTypeEnvRuntimeEntryInvalid) {
		t.Fatalf("drifted entry error = %v, want entry-invalid", err)
	}

	entry, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		target.RuntimeBasis(),
		target.InstalledRuntime(),
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeEntry() error = %v", err)
	}
	_, err = projectmemory.NewInstalledProjectTypeEnvRuntimeCatalog(
		[]projectmemory.InstalledProjectTypeEnvRuntimeEntry{entry, entry},
	)
	if !errors.Is(err, projectmemory.ErrInstalledProjectTypeEnvRuntimeCatalogInvalid) {
		t.Fatalf("duplicate-X catalog error = %v, want catalog-invalid", err)
	}
}

func TestInstalledProjectTypeEnvRuntimeCatalogOwnsRuntimeSnapshots(
	t *testing.T,
) {
	t.Parallel()
	target := currentLocalPracticeTarget(t)
	installed := target.InstalledRuntime()
	wantCatalogs := len(installed.MechanismCatalogs)
	entry, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		target.RuntimeBasis(),
		installed,
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeEntry() error = %v", err)
	}
	installed.MechanismCatalogs = nil

	catalog, err := projectmemory.NewInstalledProjectTypeEnvRuntimeCatalog(
		[]projectmemory.InstalledProjectTypeEnvRuntimeEntry{entry},
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeCatalog() error = %v", err)
	}
	first, present := catalog.Lookup(target.RuntimeBasis().Ref())
	if !present {
		t.Fatal("exact X lookup is absent")
	}
	first.MechanismCatalogs = nil
	second, present := catalog.Lookup(target.RuntimeBasis().Ref())
	if !present || len(second.MechanismCatalogs) != wantCatalogs {
		t.Fatalf(
			"second exact X lookup catalogs = %d, want immutable %d",
			len(second.MechanismCatalogs),
			wantCatalogs,
		)
	}
}

func currentLocalPracticeTarget(t *testing.T) localpracticeruntime.Target {
	t.Helper()
	path := filepath.Join("..", "cli", "fpf.db")
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatalf("open embedded FPF database read-only: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	base, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadArtifactReadOnlyDB() error = %v", err)
	}
	target, err := localpracticeruntime.Build(base, typedmemorycandidates.SourceV1_6())
	if err != nil {
		t.Fatalf("Build(current Local-Practice target) error = %v", err)
	}
	return target
}
