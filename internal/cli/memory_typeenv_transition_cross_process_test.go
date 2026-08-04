package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	transitionCrossProcessHelperEnv  = "HAFT_TEST_P12E_STALE_ADMISSION_HELPER"
	transitionCrossProcessReadyEnv   = "HAFT_TEST_P12E_READY_PATH"
	transitionCrossProcessReleaseEnv = "HAFT_TEST_P12E_RELEASE_PATH"
	transitionCrossProcessEntity     = "entity:p12e-stale-cross-process"
)

func TestMemoryTypeEnvTransitionRejectsStaleCrossProcessAdmission(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-transition-cross-process")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())
	seedPriorProjectTypeEnvHead(t, harness)

	coordination := t.TempDir()
	readyPath := filepath.Join(coordination, "predecessor-request-ready")
	releasePath := filepath.Join(coordination, "successor-selected")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestMemoryTypeEnvTransitionStaleAdmissionHelper$",
		"-test.count=1",
	)
	command.Env = append(
		os.Environ(),
		transitionCrossProcessHelperEnv+"=1",
		envProjectRoot+"="+harness.Root().String(),
		envExpectedProjectID+"="+harness.ProjectID(),
		transitionCrossProcessReadyEnv+"="+readyPath,
		transitionCrossProcessReleaseEnv+"="+releasePath,
	)
	output := &bytes.Buffer{}
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		t.Fatalf("start stale-admission helper process: %v", err)
	}
	waited := false
	defer func() {
		if waited {
			return
		}
		cancel()
		_ = command.Wait()
	}()
	waitForTransitionCrossProcessMarker(t, readyPath, 30*time.Second)

	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err != nil {
		t.Fatalf("automatically select successor while helper holds predecessor request: %v", err)
	}
	if err := os.WriteFile(releasePath, []byte("successor-selected\n"), 0o600); err != nil {
		t.Fatalf("release stale-admission helper: %v", err)
	}
	if err := command.Wait(); err != nil {
		waited = true
		t.Fatalf("stale-admission helper failed: %v\n%s", err, output.String())
	}
	waited = true
}

func TestMemoryTypeEnvTransitionStaleAdmissionHelper(t *testing.T) {
	if os.Getenv(transitionCrossProcessHelperEnv) != "1" {
		t.Skip("subprocess helper")
	}
	ctx := context.Background()
	root := os.Getenv(envProjectRoot)
	projectID := mustProjectMemoryRuntimeProjectID(
		t,
		os.Getenv(envExpectedProjectID),
	)
	ledger, err := projectledger.OpenExisting(ctx, root, projectledger.ReadWrite)
	if err != nil {
		t.Fatalf("open helper project ledger: %v", err)
	}
	defer func() {
		if err := ledger.Close(); err != nil {
			t.Fatalf("close helper project ledger: %v", err)
		}
	}()
	basis, err := buildProjectMemoryRuntimeBasisAtSources(
		ctx,
		projectID,
		ledger.Database(),
		[][]byte{transitionPriorSource(t)},
	)
	if err != nil {
		t.Fatalf("build helper predecessor runtime: %v", err)
	}
	snapshot, err := basis.snapshotLoader.LoadCurrentProjectSnapshot(ctx, projectID)
	if err != nil {
		t.Fatalf("load helper predecessor snapshot: %v", err)
	}
	adapter, err := typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
		ledger.Database(),
	).
		SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
		SetClock(typedmemorystore.SystemClock{}).
		SetReferenceEngine(typedmemorystore.NewExactPersistedStrongReferenceEngine()).
		SetObservableInputs(unavailableProjectMemoryObservableInputProvider{}).
		SetSelectedProjectRuntime(basis.selectedRuntime).
		Build()
	if err != nil {
		t.Fatalf("construct helper admission adapter: %v", err)
	}
	admission, err := projectmemory.NewAdmissionRuntime(
		projectID,
		basis.source,
		adapter,
	)
	if err != nil {
		t.Fatalf("construct helper admission runtime: %v", err)
	}
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "admit",
  "basis": {
    "kind": "exact_project",
    "type_env_digest": %q,
    "graph_revision": %d
  },
  "authority_class": %q,
  "idempotency_key": "p12e-stale-cross-process",
  "request_provenance_ref": "provenance:p12e-stale-cross-process",
  "change_set": {"changes": [{
    "kind": "declare_entity",
    "entity_id": %q,
    "local_ref": "local:p12e-stale-cross-process",
    "context": "haft-project",
    "label": "P12E stale cross-process admission",
    "provenance": "provenance:p12e-stale-cross-process-change"
  }]}
}`,
		typedmemorywire.ContractVersionV2,
		snapshot.Environment().Ref().Digest().String(),
		snapshot.Snapshot().GraphRevision().Value(),
		typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		transitionCrossProcessEntity,
	)
	request, err := typedmemorywire.DecodeAdmitRequest([]byte(payload))
	if err != nil {
		t.Fatalf("decode helper predecessor admission request: %v", err)
	}
	readyPath := os.Getenv(transitionCrossProcessReadyEnv)
	if err := os.WriteFile(readyPath, []byte("predecessor-request-ready\n"), 0o600); err != nil {
		t.Fatalf("publish helper readiness: %v", err)
	}
	releasePath := os.Getenv(transitionCrossProcessReleaseEnv)
	waitForTransitionCrossProcessMarker(t, releasePath, 90*time.Second)

	if err := ledger.Revalidate(ctx); err != nil {
		t.Fatalf(
			"helper project ledger rejected the normal successor transaction: %v",
			err,
		)
	}
	result, err := admission.Admit(ctx, request)
	if err != nil {
		t.Fatalf("stale helper admission returned operational error: %v", err)
	}
	if _, ok := result.(projectmemory.AdmissionNotAdmitted); !ok {
		t.Fatalf("stale helper admission = %T, want AdmissionNotAdmitted", result)
	}
	var durableRows int
	err = ledger.Database().QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM typed_memory_entities
		 WHERE project_id = ? AND entity_id = ?`,
		projectID.String(),
		transitionCrossProcessEntity,
	).Scan(&durableRows)
	if err != nil {
		t.Fatalf("count stale helper entity rows: %v", err)
	}
	if durableRows != 0 {
		t.Fatalf("stale helper admission wrote %d entity row(s)", durableRows)
	}
}

func waitForTransitionCrossProcessMarker(
	t *testing.T,
	path string,
	timeout time.Duration,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		if !os.IsNotExist(err) {
			t.Fatalf("observe cross-process marker %q: %v", path, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("cross-process marker %q was not produced within %s", path, timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
