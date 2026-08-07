package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

func TestBoundProjectMemoryAdmissionOpenLeavesCurrentStoreUnchanged(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_6eadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)

	admission, err := openBoundProjectMemoryRuntime(context.Background())
	if err != nil {
		t.Fatalf("openBoundProjectMemoryRuntime() error = %v", err)
	}
	if admission.ledger.ProjectID().String() != fixture.binding.ProjectID {
		_ = admission.Close()
		t.Fatalf(
			"opened project identity = %q, want %q",
			admission.ledger.ProjectID().String(),
			fixture.binding.ProjectID,
		)
	}
	if err := admission.Close(); err != nil {
		t.Fatalf("close bound project-memory admission: %v", err)
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("current-schema admission open changed SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("current-schema admission open changed project-store files")
	}
}

func TestBoundProjectMemoryAdmissionRejectsSchemaDriftWithoutRepair(
	t *testing.T,
) {
	tests := []struct {
		name      string
		projectID string
		statement string
		wantError string
	}{
		{
			name:      "old kernel schema",
			projectID: "qnt_70adbeef",
			statement: "DELETE FROM schema_version WHERE version = 49",
			wantError: "kernel schema is not current",
		},
		{
			name:      "missing artifact-store schema",
			projectID: "qnt_71adbeef",
			statement: "DROP TABLE project_typeenv_artifact_store_schema",
			wantError: "read project TypeEnv artifact schema without migration",
		},
		{
			name:      "missing Stage-store schema",
			projectID: "qnt_72adbeef",
			statement: "DROP TABLE project_typeenv_stage_store_schema",
			wantError: "read project TypeEnv Stage schema without migration",
		},
		{
			name:      "missing head-store schema",
			projectID: "qnt_73adbeef",
			statement: "DROP TABLE project_typeenv_head_store_schema",
			wantError: "read project TypeEnv head schema without migration",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newReadOnlyProjectValidationFixture(t, test.projectID)
			configureBoundProjectMemoryAdmissionFixture(t, fixture)
			executeReadOnlyProjectValidationFixtureSQL(
				t,
				fixture.database,
				test.statement,
			)
			beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
			beforeFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)

			admission, err := openBoundProjectMemoryRuntime(context.Background())
			if admission != nil {
				_ = admission.Close()
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf(
					"openBoundProjectMemoryRuntime() error = %v, want %q",
					err,
					test.wantError,
				)
			}

			afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
			afterFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)
			if !reflect.DeepEqual(afterSchema, beforeSchema) {
				t.Fatal("failed admission open changed SQLite schema")
			}
			if !reflect.DeepEqual(afterFiles, beforeFiles) {
				t.Fatal("failed admission open changed project-store files")
			}
		})
	}
}

func TestBoundProjectMemoryAdmissionRejectsMissingDatabaseWithoutCreation(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_74adbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	databaseFamily := []string{
		fixture.database,
		fixture.database + "-wal",
		fixture.database + "-shm",
	}
	for _, path := range databaseFamily {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	admission, err := openBoundProjectMemoryRuntime(context.Background())
	if admission != nil {
		_ = admission.Close()
	}
	if err == nil {
		t.Fatal("missing project database was accepted")
	}
	for _, path := range databaseFamily {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("failed admission open created %s: %v", path, statErr)
		}
	}
}

func TestBoundProjectMemoryAdmissionDoesNotCreateMissingStoreDirectory(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_79adbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	if err := os.RemoveAll(fixture.databaseDirectory); err != nil {
		t.Fatal(err)
	}

	admission, err := openBoundProjectMemoryRuntime(context.Background())
	if admission != nil {
		_ = admission.Close()
	}
	if err == nil {
		t.Fatal("missing project-store directory was accepted")
	}
	if _, statErr := os.Lstat(fixture.databaseDirectory); !os.IsNotExist(statErr) {
		t.Fatalf(
			"failed admission open created project-store directory %s: %v",
			fixture.databaseDirectory,
			statErr,
		)
	}
}

func TestBoundProjectMemoryAdmissionDoesNotRenameLegacyDatabase(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_7aadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	legacyDatabase := filepath.Join(fixture.databaseDirectory, "quint.db")
	if err := os.Rename(fixture.database, legacyDatabase); err != nil {
		t.Fatal(err)
	}

	admission, err := openBoundProjectMemoryRuntime(context.Background())
	if admission != nil {
		_ = admission.Close()
	}
	if err == nil {
		t.Fatal("legacy project database was silently migrated")
	}
	if _, statErr := os.Lstat(fixture.database); !os.IsNotExist(statErr) {
		t.Fatalf("failed admission open created or renamed %s: %v", fixture.database, statErr)
	}
	info, statErr := os.Lstat(legacyDatabase)
	if statErr != nil {
		t.Fatalf("failed admission open removed legacy database: %v", statErr)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("legacy database is no longer a regular file: %s", legacyDatabase)
	}
}

func TestBoundProjectMemoryAdmissionRejectsIdentityAndTopologyDrift(
	t *testing.T,
) {
	t.Run("expected project identity", func(t *testing.T) {
		fixture := newReadOnlyProjectValidationFixture(t, "qnt_75adbeef")
		configureBoundProjectMemoryAdmissionFixture(t, fixture)
		t.Setenv(envExpectedProjectID, "qnt_76adbeef")

		admission, err := openBoundProjectMemoryRuntime(context.Background())
		if admission != nil {
			_ = admission.Close()
		}
		if err == nil || !errors.Is(err, errExpectedProjectIDMiss) {
			t.Fatalf("identity mismatch error = %v", err)
		}
	})

	t.Run("symlinked project database", func(t *testing.T) {
		fixture := newReadOnlyProjectValidationFixture(t, "qnt_77adbeef")
		configureBoundProjectMemoryAdmissionFixture(t, fixture)
		target := fixture.database + ".target"
		if err := os.Rename(fixture.database, target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, fixture.database); err != nil {
			t.Fatal(err)
		}

		admission, err := openBoundProjectMemoryRuntime(context.Background())
		if admission != nil {
			_ = admission.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "not a symlink") {
			t.Fatalf("symlinked database error = %v", err)
		}
		info, statErr := os.Lstat(fixture.database)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("failed admission open replaced the symlinked database")
		}
	})
}

func TestBoundProjectMemoryAdmissionPostCheckPreservesCommittedAmbiguity(
	t *testing.T,
) {
	t.Parallel()

	revalidations := 0
	revalidate := func(context.Context) error {
		revalidations++
		if revalidations == 2 {
			return errors.New("injected post-admission identity drift")
		}
		return nil
	}
	want := []byte(`{"result":"committed","receipt":{"event_ref":"event:test"}}`)
	admit := func(
		context.Context,
		typedmemorywire.AdmitRequest,
	) ([]byte, error) {
		return append([]byte(nil), want...), nil
	}

	got, err := admitBoundProjectMemory(
		context.Background(),
		revalidate,
		admit,
		typedmemorywire.AdmitRequest{},
	)
	if !bytes.Equal(got, want) {
		t.Fatalf("guarded admission result = %q, want %q", got, want)
	}
	if err == nil ||
		!strings.Contains(err.Error(), "may already be durable") ||
		!strings.Contains(err.Error(), "must not be reported as no-write") {
		t.Fatalf("post-admission identity error = %v", err)
	}
}

func TestRunMemoryAdmitSurfacesReceiptWithPostWriteRevalidationError(
	t *testing.T,
) {
	want := []byte(
		`{"result":"committed","receipt":{"event_ref":"event:test"}}` + "\n",
	)
	session := &fixedProjectMemoryAdmissionSession{
		result: want,
		err: errors.New(
			"post-admission identity drift: result may already be durable and must not be reported as no-write",
		),
	}
	previousOpener := openProjectMemoryAdmissionSession
	openProjectMemoryAdmissionSession = func(
		context.Context,
	) (projectMemoryAdmissionSession, error) {
		return session, nil
	}
	t.Cleanup(func() {
		openProjectMemoryAdmissionSession = previousOpener
	})
	previousInputFile := memoryAdmitInputFile
	memoryAdmitInputFile = "-"
	t.Cleanup(func() {
		memoryAdmitInputFile = previousInputFile
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(strings.NewReader(projectMemoryAdmissionTestPayload))
	output := bytes.Buffer{}
	command.SetOut(&output)

	err := runMemoryAdmit(command, nil)
	if err == nil ||
		!strings.Contains(err.Error(), "may already be durable") ||
		!strings.Contains(err.Error(), "must not be reported as no-write") {
		t.Fatalf("runMemoryAdmit() error = %v", err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("runMemoryAdmit() output = %q, want %q", output.Bytes(), want)
	}
	if !session.closed {
		t.Fatal("runMemoryAdmit() did not close the admission session")
	}
}

func TestMemoryAdmitInputHelpSelectsCurrentV2Contract(t *testing.T) {
	t.Parallel()

	flag := memoryAdmitCmd.Flags().Lookup("input-file")
	if flag == nil {
		t.Fatal("memory admit input-file flag is missing")
	}
	if !strings.Contains(flag.Usage, typedmemorywire.ContractVersionV2) {
		t.Fatalf("memory admit input help = %q, want v2", flag.Usage)
	}
	if strings.Contains(flag.Usage, typedmemorywire.ContractVersionV1) {
		t.Fatalf("memory admit input help exposes v1 as a fresh contract: %q", flag.Usage)
	}
}

func TestProjectMemoryCommitUnknownHasCLIAndMCPDeliveryParity(
	t *testing.T,
) {
	request, err := typedmemorywire.DecodeAdmitRequest(
		[]byte(projectMemoryAdmissionTestPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	projectID := mustProjectMemoryRuntimeProjectID(t, "qnt_7badbeef")
	unknown, err := projectmemory.NewAdmissionCommitOutcomeUnknown(
		projectID,
		request,
		typedmemorystore.CommitReceipt{},
		fmt.Errorf(
			"%w: injected delivery-parity fixture",
			typedmemorystore.ErrCommitOutcomeUnknown,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := presentProjectMemoryAdmission(
		unknown,
		request.AuthorityClass(),
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')

	surface := &sealedProjectMemoryFullSurface{
		projectID: projectID,
		revalidate: func(context.Context) error {
			return nil
		},
		executor: &fixedProjectMemoryFullExecutor{
			admissionResult: encoded,
		},
	}
	mcpResult, err := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if err != nil {
		t.Fatalf("MCP unknown delivery error = %v", err)
	}

	session := &fixedProjectMemoryAdmissionSession{result: encoded}
	previousOpener := openProjectMemoryAdmissionSession
	openProjectMemoryAdmissionSession = func(
		context.Context,
	) (projectMemoryAdmissionSession, error) {
		return session, nil
	}
	t.Cleanup(func() {
		openProjectMemoryAdmissionSession = previousOpener
	})
	previousInputFile := memoryAdmitInputFile
	memoryAdmitInputFile = "-"
	t.Cleanup(func() {
		memoryAdmitInputFile = previousInputFile
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(strings.NewReader(projectMemoryAdmissionTestPayload))
	cliOutput := bytes.Buffer{}
	command.SetOut(&cliOutput)
	if err := runMemoryAdmit(command, nil); err != nil {
		t.Fatalf("CLI unknown delivery error = %v", err)
	}

	if !bytes.Equal([]byte(mcpResult), encoded) {
		t.Fatalf("MCP unknown delivery = %q, want %q", mcpResult, encoded)
	}
	if !bytes.Equal(cliOutput.Bytes(), encoded) {
		t.Fatalf("CLI unknown delivery = %q, want %q", cliOutput.Bytes(), encoded)
	}
	if !session.closed {
		t.Fatal("CLI unknown delivery did not close the admission session")
	}
}

func TestBoundProjectMemoryAdmissionPreCheckPreventsAdmission(
	t *testing.T,
) {
	t.Parallel()

	admitCalls := 0
	revalidate := func(context.Context) error {
		return errors.New("injected pre-admission identity drift")
	}
	admit := func(
		context.Context,
		typedmemorywire.AdmitRequest,
	) ([]byte, error) {
		admitCalls++
		return nil, nil
	}

	_, err := admitBoundProjectMemory(
		context.Background(),
		revalidate,
		admit,
		typedmemorywire.AdmitRequest{},
	)
	if err == nil || !strings.Contains(err.Error(), "before admission") {
		t.Fatalf("pre-admission identity error = %v", err)
	}
	if admitCalls != 0 {
		t.Fatalf("admission calls after failed identity check = %d, want 0", admitCalls)
	}
}

func configureBoundProjectMemoryAdmissionFixture(
	t *testing.T,
	fixture readOnlyProjectValidationFixture,
) {
	t.Helper()
	t.Setenv(envProjectRoot, fixture.binding.ProjectRoot)
	t.Setenv(envLegacyProjectRoot, "")
	t.Setenv(envExpectedProjectID, fixture.binding.ProjectID)
}

type fixedProjectMemoryAdmissionSession struct {
	result []byte
	err    error
	closed bool
}

func (session *fixedProjectMemoryAdmissionSession) Admit(
	context.Context,
	typedmemorywire.AdmitRequest,
) ([]byte, error) {
	return append([]byte(nil), session.result...), session.err
}

func (session *fixedProjectMemoryAdmissionSession) Close() error {
	session.closed = true
	return nil
}

func TestProjectMemoryAdmissionPresenterRetainsV2InEnvelopeAndRetry(t *testing.T) {
	t.Parallel()

	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "admit",
  "basis": {
    "kind": "exact_project",
    "type_env_digest": "sha256:%s",
    "graph_revision": 23
  },
  "authority_class": "non_binding_semantic_assertion",
  "idempotency_key": "present-v2-retry",
  "request_provenance_ref": "test:present-v2-retry",
  "change_set": {"changes": [{
    "kind": "declare_entity",
    "entity_id": "entity:present-v2-retry",
    "local_ref": "local:present-v2-retry",
    "context": "project",
    "label": "Present v2 retry",
    "provenance": "test:present-v2-retry"
  }]}
}`,
		typedmemorywire.ContractVersionV2,
		strings.Repeat("a", 64),
	)
	request, err := typedmemorywire.DecodeAdmitRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeAdmitRequest(v2): %v", err)
	}
	unknown, err := projectmemory.NewAdmissionCommitOutcomeUnknown(
		mustProjectMemoryRuntimeProjectID(t, "qnt_7cadbeef"),
		request,
		typedmemorystore.CommitReceipt{},
		fmt.Errorf("%w: v2 presenter fixture", typedmemorystore.ErrCommitOutcomeUnknown),
	)
	if err != nil {
		t.Fatalf("NewAdmissionCommitOutcomeUnknown(v2): %v", err)
	}
	response, err := presentProjectMemoryAdmission(
		unknown,
		request.AuthorityClass(),
	)
	if err != nil {
		t.Fatalf("presentProjectMemoryAdmission(v2): %v", err)
	}
	if response.ContractVersion != typedmemorywire.ContractVersionV2 {
		t.Fatalf("response contract version = %q; want v2", response.ContractVersion)
	}
	if response.Retry == nil ||
		response.Retry.ContractVersion != typedmemorywire.ContractVersionV2 {
		t.Fatalf("retry = %#v; want v2 contract version", response.Retry)
	}
}

const projectMemoryAdmissionTestPayload = `{
  "contract_version":"haft.memory.v2",
  "action":"admit",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":17
  },
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"cli-admission-shell-fixture",
  "request_provenance_ref":"provenance:cli-admission-shell-fixture",
  "change_set":{"changes":[{
    "kind":"declare_entity",
    "entity_id":"entity:cli-admission-shell-fixture",
    "local_ref":"local:cli-admission-shell-fixture",
    "context":"haft-project",
    "label":"CLI admission shell fixture",
    "provenance":"provenance:cli-admission-shell-fixture-change"
  }]}
}`
