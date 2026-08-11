package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProjectMemoryRuntimeCatalogRetainsHistoricalEditions(t *testing.T) {
	t.Parallel()

	database, err := openCurrentKernelTestStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	basis, err := buildProjectMemoryRuntimeBasis(
		context.Background(),
		mustProjectMemoryRuntimeProjectID(t, "qnt_cafebabe"),
		database.GetRawDB(),
	)
	if err != nil {
		t.Fatalf("build project-memory runtime basis: %v", err)
	}
	tests := []struct {
		edition string
		baseRef string
		source  []byte
	}{
		{
			edition: "1.4.0",
			baseRef: basetypeenvartifacts.HistoricalV5Ref,
			source:  typedmemorycandidates.SourceV1_4(),
		},
		{
			edition: "1.5.0",
			baseRef: basetypeenvartifacts.HistoricalV6Ref,
			source:  typedmemorycandidates.SourceV1_5(),
		},
	}
	for _, test := range tests {
		t.Run(test.edition, func(t *testing.T) {
			historicalRef, err := typedmemory.ParseTypeEnvRef(test.baseRef)
			if err != nil {
				t.Fatalf("parse historical %s Base TypeEnv ref: %v", test.edition, err)
			}
			historicalBase, err := basetypeenvartifacts.LoadExact(historicalRef)
			if err != nil {
				t.Fatalf("load historical %s Base TypeEnv: %v", test.edition, err)
			}
			historicalTarget, err := localpracticeruntime.Build(
				historicalBase,
				test.source,
			)
			if err != nil {
				t.Fatalf("build historical %s Local-Practice target: %v", test.edition, err)
			}
			composite := historicalTarget.Composite().Ref().String()
			installed, present := basis.targetsByTypeEnv[composite]
			if !present {
				t.Fatalf("installed runtime catalog omitted historical %s composite %s", test.edition, composite)
			}
			if installed.RuntimeBasis().Ref() != historicalTarget.RuntimeBasis().Ref() {
				t.Fatalf("installed historical %s runtime uses another exact X basis", test.edition)
			}
		})
	}
}

func TestProjectMemoryRuntimeBuildsWithoutSelectingAProjectTypeEnvHead(
	t *testing.T,
) {
	t.Parallel()

	database, err := openCurrentKernelTestStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	projectID := mustProjectMemoryRuntimeProjectID(t, "qnt_deadbeef")
	runtime, err := newProjectMemoryRuntime(
		context.Background(),
		projectID,
		database.GetRawDB(),
	)
	if err != nil {
		t.Fatalf("newProjectMemoryRuntime() error = %v", err)
	}
	payload := bytes.Replace(
		bundledMemoryValidationFixture(),
		[]byte(`{"kind":"bundled_candidate_open_world"}`),
		[]byte(`{"kind":"project_current"}`),
		1,
	)
	result, err := runtime.Validate(context.Background(), payload)
	if err != nil {
		t.Fatalf("Validate(project_current) error = %v", err)
	}
	response := struct {
		Verdict string `json:"verdict"`
		Basis   struct {
			RequestedKind  string `json:"requested_kind"`
			ResolutionKind string `json:"resolution_kind"`
		} `json:"basis"`
	}{}
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("decode project-memory response: %v", err)
	}
	if response.Verdict != "underdetermined" {
		t.Fatalf("verdict = %q, want underdetermined", response.Verdict)
	}
	if response.Basis.RequestedKind != "project_current" ||
		response.Basis.ResolutionKind != "project_basis_unavailable" {
		t.Fatalf("basis = %#v", response.Basis)
	}

	admissionPayload := []byte(`{
  "contract_version":"haft.memory.v1",
  "action":"admit",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":0
  },
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"cli-project-memory-no-head",
  "request_provenance_ref":"provenance:cli-project-memory-no-head",
  "change_set":{
    "changes":[{
      "kind":"declare_entity",
      "entity_id":"entity:cli-project-memory-no-head",
      "local_ref":"local:cli-project-memory-no-head",
      "context":"haft-project",
      "label":"CLI project memory without a selected head",
      "provenance":"provenance:cli-project-memory-no-head-change"
    }]
  }
}`)
	admissionRequest, err := typedmemorywire.DecodeAdmitRequest(admissionPayload)
	if err != nil {
		t.Fatalf("DecodeAdmitRequest() error = %v", err)
	}
	admissionResult, err := runtime.Admit(context.Background(), admissionRequest)
	if err != nil {
		t.Fatalf("Admit(without head) error = %v", err)
	}
	admissionResponse := struct {
		Action      string `json:"action"`
		Result      string `json:"result"`
		Persistence struct {
			Mode             string  `json:"mode"`
			RowsWritten      *uint64 `json:"rows_written"`
			AuthorityGranted bool    `json:"authority_granted"`
		} `json:"persistence_disposition"`
		Interpretation struct {
			Establishes      []string `json:"establishes"`
			Omits            []string `json:"omits"`
			DoesNotAuthorize []string `json:"does_not_authorize"`
		} `json:"interpretation"`
	}{}
	if err := json.Unmarshal(admissionResult, &admissionResponse); err != nil {
		t.Fatalf("decode admission response: %v", err)
	}
	if admissionResponse.Action != "admit" ||
		admissionResponse.Result != "not_admitted" ||
		admissionResponse.Persistence.Mode != "not_admitted_no_write" ||
		admissionResponse.Persistence.RowsWritten == nil ||
		*admissionResponse.Persistence.RowsWritten != 0 ||
		admissionResponse.Persistence.AuthorityGranted {
		t.Fatalf("admission response = %#v", admissionResponse)
	}
	if len(admissionResponse.Interpretation.Establishes) == 0 ||
		len(admissionResponse.Interpretation.Omits) == 0 ||
		len(admissionResponse.Interpretation.DoesNotAuthorize) == 0 {
		t.Fatalf(
			"admission interpretation contract = %#v",
			admissionResponse.Interpretation,
		)
	}
}

func TestProjectMemoryValidationHandlerCannotReachAdmission(t *testing.T) {
	t.Parallel()

	database, err := openCurrentKernelTestStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	runtime, err := newProjectMemoryRuntime(
		context.Background(),
		mustProjectMemoryRuntimeProjectID(t, "qnt_deadbeef"),
		database.GetRawDB(),
	)
	if err != nil {
		t.Fatalf("newProjectMemoryRuntime() error = %v", err)
	}
	payload := json.RawMessage(`{
  "contract_version":"haft.memory.v1",
  "action":"admit",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":0
  },
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"validation-handler-must-not-admit",
  "request_provenance_ref":"provenance:validation-handler-must-not-admit",
  "change_set":{"changes":[{
    "kind":"declare_entity",
    "entity_id":"entity:validation-handler-must-not-admit",
    "local_ref":"local:validation-handler-must-not-admit",
    "context":"haft-project",
    "label":"must not admit",
    "provenance":"provenance:validation-handler-must-not-admit-change"
  }]}
}`)
	result, err := runtime.ValidationMCPHandler()(
		context.Background(),
		payload,
	)
	if err == nil {
		t.Fatal("ValidationMCPHandler accepted admission payload")
	}
	if result != "" {
		t.Fatalf("rejected admission produced result %q", result)
	}
}

func TestProjectMemoryReadRuntimeHasNoAdmissionCapability(t *testing.T) {
	t.Parallel()

	database, err := openCurrentKernelTestStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	runtime, err := newProjectMemoryReadRuntime(
		context.Background(),
		mustProjectMemoryRuntimeProjectID(t, "qnt_7eadbeef"),
		database.GetRawDB(),
	)
	if err != nil {
		t.Fatalf("newProjectMemoryReadRuntime() error = %v", err)
	}
	runtimeType := reflect.TypeOf(runtime)
	if _, found := runtimeType.MethodByName("Admit"); found {
		t.Fatal("projectMemoryReadRuntime exposes admission")
	}
	if err := runtime.EnsureReady(context.Background()); err != nil {
		t.Fatalf(
			"headless project-memory runtime did not expose recovery surface: %v",
			err,
		)
	}

	payload := json.RawMessage(`{
  "contract_version":"haft.memory.v1",
  "action":"admit",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":1
  },
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"read-handler-must-not-admit",
  "request_provenance_ref":"provenance:read-handler-must-not-admit",
  "change_set":{"changes":[{
    "kind":"declare_entity",
    "entity_id":"entity:read-handler-must-not-admit",
    "local_ref":"local:read-handler-must-not-admit",
    "context":"haft-project",
    "label":"must not admit",
    "provenance":"provenance:read-handler-must-not-admit-change"
  }]}
}`)
	result, err := runtime.ReadOnlyMCPHandler()(
		context.Background(),
		payload,
	)
	if err == nil {
		t.Fatal("ReadOnlyMCPHandler accepted admission payload")
	}
	if result != "" {
		t.Fatalf("rejected admission produced result %q", result)
	}
}

func TestProjectMemoryAdmissionPresenterRejectsMissingClosedResult(t *testing.T) {
	t.Parallel()

	response, err := presentProjectMemoryAdmission(
		nil,
		"non_binding_semantic_assertion",
	)
	if err == nil {
		t.Fatal("presentProjectMemoryAdmission(nil) succeeded")
	}
	if !strings.Contains(err.Error(), "unsupported result variant") {
		t.Fatalf("presentProjectMemoryAdmission(nil) error = %v", err)
	}
	if response.ContractVersion != "" ||
		response.Action != "" ||
		response.Result != "" ||
		response.Receipt != nil {
		t.Fatalf("failed presentation returned response %#v", response)
	}
}

func mustProjectMemoryRuntimeProjectID(
	t *testing.T,
	raw string,
) projectidentity.ProjectID {
	t.Helper()
	value, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID(%q) error = %v", raw, err)
	}
	return value
}
