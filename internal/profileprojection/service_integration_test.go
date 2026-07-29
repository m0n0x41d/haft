package profileprojection_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	kerneldb "github.com/m0n0x41d/haft/db"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	. "github.com/m0n0x41d/haft/internal/profileprojection"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestCanonicalAdmissionProjectsAndRebuildsFromLedger(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, rootPath)
	createProjectionDirectory(t, rootPath)
	admission := harness.AdmitSoftwareRevision(t, "project-rebuild-v1")
	service := mustProjectionService(t, harness.Database())

	projected, err := service.Project(context.Background(), admission)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	requireSynchronizedResult(t, projected)
	projectionPath := projected.ProjectionPath()
	assertProjectedRevision(t, projectionPath, admission.LedgerRevision().Value())
	projectedBytes := readProjection(t, projectionPath)

	if err := os.Remove(projectionPath); err != nil {
		t.Fatalf("remove projection before Rebuild: %v", err)
	}
	rebuilt, err := service.Rebuild(context.Background(), harness.Root())
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	requireSynchronizedResult(t, rebuilt)
	assertProjectedRevision(t, projectionPath, admission.LedgerRevision().Value())
	rebuiltBytes := readProjection(t, projectionPath)
	if !bytes.Equal(rebuiltBytes, projectedBytes) || rebuilt.ExpectedDigest() != projected.ExpectedDigest() {
		t.Fatal("Rebuild did not reproduce exact deterministic projection bytes")
	}
}

func TestMissingAndDriftedProjectionDebtIsExactDurableAndRetryable(t *testing.T) {
	cases := []struct {
		name               string
		prepareProjection  func(*testing.T, Service, profileadmissionsqlite.CanonicalProfileAdmission)
		wantReason         string
		wantObservedDigest bool
	}{
		{
			name: "missing",
			prepareProjection: func(
				*testing.T,
				Service,
				profileadmissionsqlite.CanonicalProfileAdmission,
			) {
			},
			wantReason:         "projection_missing",
			wantObservedDigest: false,
		},
		{
			name: "drifted",
			prepareProjection: func(
				t *testing.T,
				_ Service,
				admission profileadmissionsqlite.CanonicalProfileAdmission,
			) {
				path, err := ProjectionPath(admission.ProjectRoot())
				if err != nil {
					t.Fatalf("ProjectionPath: %v", err)
				}
				if err := os.WriteFile(path, []byte("drift: true\n"), 0o644); err != nil {
					t.Fatalf("write drifted projection: %v", err)
				}
			},
			wantReason:         "projection_drift",
			wantObservedDigest: true,
		},
	}
	runProjectionDebtCases(t, cases, 0)
}

func TestStaleAdmissionCannotOverwriteCurrentRevision(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, rootPath)
	createProjectionDirectory(t, rootPath)
	service := mustProjectionService(t, harness.Database())
	revisionOne := harness.AdmitSoftwareRevision(t, "stale-v1")
	first, err := service.Project(context.Background(), revisionOne)
	if err != nil {
		t.Fatalf("Project revision one: %v", err)
	}
	requireSynchronizedResult(t, first)
	beforeStale := readProjection(t, first.ProjectionPath())
	revisionTwo := harness.AdmitSoftwareRevision(t, "stale-v2")
	if revisionTwo.LedgerRevision().Value() <= revisionOne.LedgerRevision().Value() {
		t.Fatalf(
			"revision two = %d, want newer than %d",
			revisionTwo.LedgerRevision().Value(),
			revisionOne.LedgerRevision().Value(),
		)
	}

	staleResult, staleErr := service.Project(context.Background(), revisionOne)
	if staleErr == nil {
		t.Fatalf("stale Project result = %#v, want fail-closed error", staleResult)
	}
	if !strings.Contains(staleErr.Error(), "stale canonical admission") {
		t.Fatalf("stale Project error = %v", staleErr)
	}
	assertProjectedRevision(t, first.ProjectionPath(), revisionOne.LedgerRevision().Value())
	afterStale := readProjection(t, first.ProjectionPath())
	if !bytes.Equal(afterStale, beforeStale) {
		t.Fatal("stale admission changed projection bytes")
	}

	rebuilt, err := service.Rebuild(context.Background(), harness.Root())
	if err != nil {
		t.Fatalf("Rebuild current revision: %v", err)
	}
	requireSynchronizedResult(t, rebuilt)
	assertProjectedRevision(t, rebuilt.ProjectionPath(), revisionTwo.LedgerRevision().Value())
}

func TestTwoConnectionsSerializeProjectorsForOneCanonicalRevision(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, rootPath)
	createProjectionDirectory(t, rootPath)
	admission := harness.AdmitSoftwareRevision(t, "two-connection-v1")
	secondStore, err := kerneldb.NewStore(harness.DatabasePath())
	if err != nil {
		t.Fatalf("open second store: %v", err)
	}
	t.Cleanup(func() {
		if err := secondStore.Close(); err != nil {
			t.Errorf("close second store: %v", err)
		}
	})
	services := []Service{
		mustProjectionService(t, harness.Database()),
		mustProjectionService(t, secondStore.GetRawDB()),
	}
	results := make([]projectionCall, len(services))
	barrier := make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(len(services))
	launchProjector(&wait, barrier, services, admission, results, 0)
	launchProjector(&wait, barrier, services, admission, results, 1)
	close(barrier)
	wait.Wait()
	requireProjectionCallsSynchronized(t, results, 0)

	events := readDebtEvents(t, harness.Database(), admission)
	if len(events) != 2 {
		t.Fatalf("serialized projector debt events = %d, want one opened/resolved pair", len(events))
	}
	assertResolvedDebtPair(t, events, "projection_missing", false)
}

func runProjectionDebtCases(
	t *testing.T,
	cases []struct {
		name               string
		prepareProjection  func(*testing.T, Service, profileadmissionsqlite.CanonicalProfileAdmission)
		wantReason         string
		wantObservedDigest bool
	},
	index int,
) {
	t.Helper()
	if index >= len(cases) {
		return
	}
	test := cases[index]
	t.Run(test.name, func(t *testing.T) {
		rootPath := filepath.Join(t.TempDir(), "project")
		harness := profileadmissionfixture.New(t, rootPath)
		createProjectionDirectory(t, rootPath)
		admission := harness.AdmitSoftwareRevision(t, "debt-"+test.name+"-v1")
		service := mustProjectionService(t, harness.Database())
		test.prepareProjection(t, service, admission)
		SetIdentifierSourceForTest(&service, func(string) (string, error) {
			return "", errors.New("injected stage identifier failure")
		})

		failed, err := service.Project(context.Background(), admission)
		if err == nil {
			t.Fatal("projection failure returned no error")
		}
		if failed.Kind() != ResultProjectionDebt || failed.DebtID() == "" {
			t.Fatalf("failed result = %#v, want durable projection debt", failed)
		}
		opened := readDebtEvents(t, harness.Database(), admission)
		if len(opened) != 1 {
			t.Fatalf("opened debt events = %d, want one", len(opened))
		}
		assertOpenedDebt(t, opened[0], admission, failed, test.wantReason, test.wantObservedDigest)

		SetIdentifierSourceForTest(&service, RandomIdentifierForTest)
		retried, err := service.Rebuild(context.Background(), harness.Root())
		if err != nil {
			t.Fatalf("retry Rebuild: %v", err)
		}
		requireSynchronizedResult(t, retried)
		resolved := readDebtEvents(t, harness.Database(), admission)
		if len(resolved) != 2 {
			t.Fatalf("resolved debt events = %d, want opened/resolved pair", len(resolved))
		}
		assertResolvedDebtPair(t, resolved, test.wantReason, test.wantObservedDigest)
	})
	runProjectionDebtCases(t, cases, index+1)
}

type projectionCall struct {
	result Result
	err    error
}

func launchProjector(
	wait *sync.WaitGroup,
	barrier <-chan struct{},
	services []Service,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	results []projectionCall,
	index int,
) {
	go func() {
		defer wait.Done()
		<-barrier
		result, err := services[index].Project(context.Background(), admission)
		results[index] = projectionCall{result: result, err: err}
	}()
}

func requireProjectionCallsSynchronized(
	t *testing.T,
	results []projectionCall,
	index int,
) {
	t.Helper()
	if index >= len(results) {
		return
	}
	call := results[index]
	if call.err != nil {
		t.Fatalf("projector %d: %v", index, call.err)
	}
	requireSynchronizedResult(t, call.result)
	requireProjectionCallsSynchronized(t, results, index+1)
}

type projectionDebtEvent struct {
	storageGeneration         string
	profileRevisionGeneration string
	eventID                   string
	debtID                    string
	admissionID               string
	admissionDigest           string
	projectRoot               string
	ledgerRevision            uint64
	profilePayloadDigest      string
	projectionPath            string
	eventKind                 string
	reasonCode                string
	expectedProjectionDigest  string
	observedProjectionDigest  string
	supersedesEventGeneration string
	supersedesEventID         string
}

func readDebtEvents(
	t *testing.T,
	database *sql.DB,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) []projectionDebtEvent {
	t.Helper()
	rows, err := database.Query(
		`WITH debt_events AS (
			SELECT 'v1' AS storage_generation,
			       'v1' AS profile_revision_generation,
			       event_id, debt_id, admission_id, admission_digest,
			       project_root, ledger_revision, profile_payload_digest,
			       projection_path, event_kind, reason_code,
			       expected_projection_digest, observed_projection_digest,
			       CASE
			           WHEN COALESCE(supersedes_event_id, '') = '' THEN ''
			           ELSE 'v1'
			       END AS supersedes_event_generation,
			       COALESCE(supersedes_event_id, '') AS supersedes_event_id
			FROM project_profile_projection_debt
			UNION ALL
			SELECT 'v2', profile_revision_generation,
			       event_id, debt_id, admission_id, admission_digest,
			       project_root, ledger_revision, profile_payload_digest,
			       projection_path, event_kind, reason_code,
			       expected_projection_digest, observed_projection_digest,
			       COALESCE(supersedes_event_generation, ''),
			       COALESCE(supersedes_event_id, '')
			FROM project_profile_projection_debt_v2
			UNION ALL
			SELECT 'v3', profile_revision_generation,
			       event_id, debt_id, admission_id, admission_digest,
			       project_root, ledger_revision, profile_payload_digest,
			       projection_path, event_kind, reason_code,
			       expected_projection_digest, observed_projection_digest,
			       COALESCE(supersedes_event_generation, ''),
			       COALESCE(supersedes_event_id, '')
			FROM project_profile_projection_debt_v3
		)
		SELECT storage_generation, profile_revision_generation,
		       event_id, debt_id, admission_id, admission_digest,
                project_root, ledger_revision, profile_payload_digest,
                projection_path, event_kind, reason_code,
                expected_projection_digest, observed_projection_digest,
		       supersedes_event_generation, supersedes_event_id
		 FROM debt_events
         WHERE project_root = ?
           AND ledger_revision = ?
           AND admission_id = ?
           AND admission_digest = ?
           AND profile_payload_digest = ?
         ORDER BY CASE event_kind WHEN 'opened' THEN 0 ELSE 1 END, event_id`,
		admission.ProjectRoot().String(),
		admission.LedgerRevision().Value(),
		admission.AdmissionRecordRef().String(),
		admission.AdmissionRecordDigest().String(),
		admission.PayloadDigest().String(),
	)
	if err != nil {
		t.Fatalf("query projection debt: %v", err)
	}
	defer rows.Close()
	events := []projectionDebtEvent{}
	for rows.Next() {
		event := projectionDebtEvent{}
		err = rows.Scan(
			&event.storageGeneration,
			&event.profileRevisionGeneration,
			&event.eventID,
			&event.debtID,
			&event.admissionID,
			&event.admissionDigest,
			&event.projectRoot,
			&event.ledgerRevision,
			&event.profilePayloadDigest,
			&event.projectionPath,
			&event.eventKind,
			&event.reasonCode,
			&event.expectedProjectionDigest,
			&event.observedProjectionDigest,
			&event.supersedesEventGeneration,
			&event.supersedesEventID,
		)
		if err != nil {
			t.Fatalf("scan projection debt: %v", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate projection debt: %v", err)
	}
	return events
}

func assertOpenedDebt(
	t *testing.T,
	event projectionDebtEvent,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	result Result,
	wantReason string,
	wantObservedDigest bool,
) {
	t.Helper()
	if event.eventKind != "opened" || event.debtID != result.DebtID() {
		t.Fatalf("opened event = %#v, result = %#v", event, result)
	}
	if event.storageGeneration != "v3" || event.supersedesEventGeneration != "" {
		t.Fatalf("new opened debt was not stored in the v3 event sum: %#v", event)
	}
	if event.profileRevisionGeneration != "v3" {
		t.Fatalf("new projection debt did not retain its exact v3 admission generation: %#v", event)
	}
	if event.admissionID != admission.AdmissionRecordRef().String() ||
		event.admissionDigest != admission.AdmissionRecordDigest().String() ||
		event.projectRoot != admission.ProjectRoot().String() ||
		event.ledgerRevision != admission.LedgerRevision().Value() ||
		event.profilePayloadDigest != admission.PayloadDigest().String() {
		t.Fatalf("debt event is not bound to exact admission: %#v", event)
	}
	if event.projectionPath != result.ProjectionPath() ||
		event.expectedProjectionDigest != result.ExpectedDigest().String() ||
		event.reasonCode != wantReason ||
		event.supersedesEventID != "" {
		t.Fatalf("opened event has wrong projection relation: %#v", event)
	}
	hasObservedDigest := event.observedProjectionDigest != ""
	if hasObservedDigest != wantObservedDigest {
		t.Fatalf("opened observed digest = %q, want present = %t", event.observedProjectionDigest, wantObservedDigest)
	}
}

func assertResolvedDebtPair(
	t *testing.T,
	events []projectionDebtEvent,
	wantReason string,
	wantOpenedObservedDigest bool,
) {
	t.Helper()
	opened := events[0]
	resolved := events[1]
	if opened.eventKind != "opened" || opened.reasonCode != wantReason {
		t.Fatalf("opened debt = %#v", opened)
	}
	hasOpenedObservedDigest := opened.observedProjectionDigest != ""
	if hasOpenedObservedDigest != wantOpenedObservedDigest {
		t.Fatalf("opened observed digest = %q, want present = %t", opened.observedProjectionDigest, wantOpenedObservedDigest)
	}
	if resolved.eventKind != "resolved" || resolved.reasonCode != "projection_verified" {
		t.Fatalf("resolved debt = %#v", resolved)
	}
	if resolved.storageGeneration != "v3" ||
		resolved.profileRevisionGeneration != opened.profileRevisionGeneration ||
		resolved.supersedesEventGeneration != opened.storageGeneration {
		t.Fatalf("resolution generation lineage = %#v, opened = %#v", resolved, opened)
	}
	if resolved.debtID != opened.debtID || resolved.supersedesEventID != opened.eventID {
		t.Fatalf("resolution lineage = %#v, opened = %#v", resolved, opened)
	}
	if resolved.expectedProjectionDigest != opened.expectedProjectionDigest ||
		resolved.observedProjectionDigest != opened.expectedProjectionDigest {
		t.Fatalf("resolution digests = %#v, opened = %#v", resolved, opened)
	}
}

func createProjectionDirectory(t *testing.T, rootPath string) {
	t.Helper()
	path := filepath.Join(rootPath, ".haft")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create projection directory: %v", err)
	}
}

func mustProjectionService(t *testing.T, database *sql.DB) Service {
	t.Helper()
	service, err := NewService(database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

func requireSynchronizedResult(t *testing.T, result Result) {
	t.Helper()
	if result.Kind() != ResultSynchronized {
		t.Fatalf("result kind = %q, want synchronized: %#v", result.Kind(), result)
	}
	if result.ExpectedDigest().String() == "" || result.ObservedDigest() != result.ExpectedDigest() {
		t.Fatalf("synchronized result lacks exact digest evidence: %#v", result)
	}
}

func assertProjectedRevision(t *testing.T, path string, revision uint64) {
	t.Helper()
	content := readProjection(t, path)
	want := "ledger_revision: " + uint64Text(revision)
	if !strings.Contains(string(content), want) {
		t.Fatalf("projection does not contain %q:\n%s", want, content)
	}
}

func readProjection(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read projection: %v", err)
	}
	return content
}

func uint64Text(value uint64) string {
	const digits = "0123456789"
	if value < 10 {
		return string(digits[value])
	}
	return uint64Text(value/10) + string(digits[value%10])
}
