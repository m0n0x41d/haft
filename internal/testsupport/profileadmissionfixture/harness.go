package profileadmissionfixture

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profileadmission"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

// Harness owns one migrated database and one physical project root.
type Harness struct {
	store        *kerneldb.Store
	database     *sql.DB
	databasePath string
	projectID    string
	root         projectprofile.ProjectRootV1
	closeOnce    sync.Once
	closeFn      func() error
	closeErr     error
}

// New creates a test database and binds the harness to projectRoot.
func New(t testing.TB, projectRoot string) *Harness {
	t.Helper()
	err := os.MkdirAll(projectRoot, 0o755)
	if err != nil {
		t.Fatalf("create fixture project root: %v", err)
	}
	physicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("resolve fixture project root: %v", err)
	}
	root := mustValue(t, physicalRoot, projectprofile.NewProjectRootV1)
	projectID := "qnt_f17e0001"
	projectConfigDir := filepath.Join(physicalRoot, ".haft")
	if err := os.MkdirAll(projectConfigDir, 0o755); err != nil {
		t.Fatalf("create fixture .haft directory: %v", err)
	}
	projectConfig := []byte("id: " + projectID + "\nname: profile-admission-fixture\n")
	if err := os.WriteFile(
		filepath.Join(projectConfigDir, "project.yaml"),
		projectConfig,
		0o644,
	); err != nil {
		t.Fatalf("write fixture project identity: %v", err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	databaseDir := filepath.Join(home, ".haft", "projects", projectID)
	if err := os.MkdirAll(databaseDir, 0o755); err != nil {
		t.Fatalf("create fixture project-ledger directory: %v", err)
	}
	databasePath := filepath.Join(databaseDir, "haft.db")
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatalf("create fixture database: %v", err)
	}
	if err := projectledger.BindInitialized(
		context.Background(),
		physicalRoot,
		time.Now().UTC(),
	); err != nil {
		_ = store.Close()
		t.Fatalf("bind fixture project ledger: %v", err)
	}
	harness := &Harness{
		store:        store,
		database:     store.GetRawDB(),
		databasePath: databasePath,
		projectID:    projectID,
		root:         root,
		closeFn:      store.Close,
	}
	t.Cleanup(func() {
		if err := harness.Close(); err != nil {
			t.Errorf("close fixture database: %v", err)
		}
	})
	return harness
}

// OpenExisting attaches the fixture authority/admission builders to a project
// ledger that was created through the real `haft init` boundary. It does not
// create, migrate, bind, or repair the ledger. The returned harness can only
// admit fixture profiles while `go test` owns the checked ledger handle.
func OpenExisting(t testing.TB, projectRoot string) *Harness {
	t.Helper()
	physicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("resolve existing fixture project root: %v", err)
	}
	root := mustValue(t, physicalRoot, projectprofile.NewProjectRootV1)
	handle, err := projectledger.OpenExisting(
		context.Background(),
		physicalRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		t.Fatalf("open existing fixture project ledger: %v", err)
	}
	harness := &Harness{
		database:  handle.Database(),
		projectID: handle.ProjectID().String(),
		root:      root,
		closeFn:   handle.Close,
	}
	t.Cleanup(func() {
		if err := harness.Close(); err != nil {
			t.Errorf("close existing fixture project ledger: %v", err)
		}
	})
	return harness
}

// Close releases the checked fixture ledger before another process opens the
// same SQLite WAL database. Cleanup remains idempotent for ordinary tests.
func (harness *Harness) Close() error {
	if harness == nil {
		return nil
	}
	harness.closeOnce.Do(func() {
		if harness.closeFn != nil {
			harness.closeErr = harness.closeFn()
		}
		harness.database = nil
		harness.store = nil
	})
	return harness.closeErr
}

// Database exposes the migrated handle for outer-boundary integration tests.
func (harness *Harness) Database() *sql.DB {
	return harness.database
}

// DatabasePath exposes the owned database path for restart tests.
func (harness *Harness) DatabasePath() string {
	return harness.databasePath
}

// ProjectID returns the canonical project-ledger identity owned by the fixture.
func (harness *Harness) ProjectID() string {
	return harness.projectID
}

// Root returns the strong physical project-root value used by admissions.
func (harness *Harness) Root() projectprofile.ProjectRootV1 {
	return harness.root
}

// AdmitSoftwareRevision records one fresh canonical profile revision whose
// payload contains one SoftwareRealization scope. A distinct suffix produces
// a distinct support DAG and can therefore be used to admit the next revision.
func (harness *Harness) AdmitSoftwareRevision(
	t testing.TB,
	suffix string,
) profileadmissionsqlite.CanonicalProfileAdmission {
	t.Helper()
	payload := newIntegrationPayload(t, suffix)
	return harness.admitPayload(t, suffix, payload)
}

// AdmitNonSoftwareRevision records one fresh canonical profile revision whose
// payload contains one NonSoftwareRealization scope.
func (harness *Harness) AdmitNonSoftwareRevision(
	t testing.TB,
	suffix string,
) profileadmissionsqlite.CanonicalProfileAdmission {
	t.Helper()
	payload := newNonSoftwareIntegrationPayload(t, suffix)
	return harness.admitPayload(t, suffix, payload)
}

// AdmitMixedRevision records one canonical profile revision with one software
// and one non-software scope. The software scope has the stable ID "software"
// so installed init fixtures can exercise exact scope selection.
func (harness *Harness) AdmitMixedRevision(
	t testing.TB,
	suffix string,
) profileadmissionsqlite.CanonicalProfileAdmission {
	t.Helper()
	payload := newMixedIntegrationPayload(t)
	return harness.admitPayload(t, suffix, payload)
}

// AdmitSoftwareRevisionWithTargetEntity records one software scope whose
// admitted payload directly references the current target-system entity.
func (harness *Harness) AdmitSoftwareRevisionWithTargetEntity(
	t testing.TB,
	suffix string,
	entityRef string,
) profileadmissionsqlite.CanonicalProfileAdmission {
	t.Helper()
	entity := mustValue(t, entityRef, projectprofile.NewEntityRef)
	payload := newIntegrationPayloadWithEntity(
		t,
		suffix,
		projectprofile.NewReferencedEntity(entity),
	)
	return harness.admitPayload(t, suffix, payload)
}

func (harness *Harness) admitPayload(
	t testing.TB,
	suffix string,
	payload projectprofile.ProfileDeclarationPayload,
) profileadmissionsqlite.CanonicalProfileAdmission {
	t.Helper()
	request := harness.prepareV3AdmissionRequest(t, suffix, payload)
	service, err := profileadmissionsqlite.NewService(harness.database)
	if err != nil {
		t.Fatalf("create profile-admission service: %v", err)
	}
	result := service.Admit(context.Background(), request)
	if result.Kind() != profileadmissionsqlite.AdmissionResultAdmitted {
		denials, _ := result.Denials()
		failure, _ := result.Failure()
		t.Fatalf("profile admission = %q, denials = %#v, failure = %#v", result.Kind(), denials, failure)
	}
	admission, ok := result.Admission()
	if !ok || !admission.Valid() {
		t.Fatal("admitted result omitted a valid canonical admission")
	}
	if admission.Delivery() != profileadmissionsqlite.CanonicalAdmissionFresh {
		t.Fatalf("profile admission delivery = %q, want fresh", admission.Delivery())
	}
	return admission
}

func (harness *Harness) prepareV3AdmissionRequest(
	t testing.TB,
	suffix string,
	payload projectprofile.ProfileDeclarationPayload,
) profileadmission.ProfileDeclarationAdmissionRequest {
	t.Helper()
	input := newFixtureProfileWorkInput(t, harness.root, payload)
	operatorRequest, err := operatorrequest.New(
		operatorrequest.ProfileDeclaration,
		"profile-fixture:"+suffix,
		input.CanonicalJSON(),
	)
	if err != nil {
		t.Fatalf("build fixture operator request: %v", err)
	}
	policy, err := profileonboarding.NewProfileDeclarationPolicy(operatorRequest)
	if err != nil {
		t.Fatalf("build fixture profile-declaration policy: %v", err)
	}
	clock := func() time.Time { return time.Now().UTC().Round(0) }
	fixture, err := profileonboarding.PrepareProfileDeclarationAdmissionForTestFixture(
		t,
		context.Background(),
		harness.database,
		harness.root.String(),
		input,
		policy,
		clock,
	)
	if err != nil {
		t.Fatalf("prepare v3 profile admission fixture: %v", err)
	}
	request, ok := fixture.AdmissionRequest()
	if !ok {
		t.Fatal("v3 profile admission fixture omitted an exact request")
	}
	return request
}
