package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type fixedProjectMemoryFullExecutor struct {
	readyErr        error
	basisAvailable  bool
	basisErr        error
	bundledResult   []byte
	projectResult   []byte
	admissionResult []byte
	bundledErr      error
	projectErr      error
	admissionErr    error
	readyCalls      int
	bundledCalls    int
	projectCalls    int
	admissionCalls  int
	projectActions  []string
}

type unknownThenReplayProjectMemoryFullExecutor struct {
	projectID      projectidentity.ProjectID
	admissionCalls int
	durableEffects int
}

func (executor *fixedProjectMemoryFullExecutor) EnsureReady(
	context.Context,
) error {
	executor.readyCalls++
	return executor.readyErr
}

func (executor *fixedProjectMemoryFullExecutor) ProjectBasisAvailable(
	context.Context,
) (bool, error) {
	return executor.basisAvailable, executor.basisErr
}

func (executor *fixedProjectMemoryFullExecutor) ValidateBundled(
	_ context.Context,
	_ typedmemorywire.ValidateRequest,
) ([]byte, error) {
	executor.bundledCalls++
	return append([]byte(nil), executor.bundledResult...), executor.bundledErr
}

func (executor *fixedProjectMemoryFullExecutor) ExecuteProjectRead(
	_ context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	executor.projectCalls++
	executor.projectActions = append(
		executor.projectActions,
		request.Action(),
	)
	return append([]byte(nil), executor.projectResult...), executor.projectErr
}

func (executor *fixedProjectMemoryFullExecutor) Admit(
	_ context.Context,
	_ typedmemorywire.AdmitRequest,
) ([]byte, error) {
	executor.admissionCalls++
	return append([]byte(nil), executor.admissionResult...),
		executor.admissionErr
}

func (executor *unknownThenReplayProjectMemoryFullExecutor) EnsureReady(
	context.Context,
) error {
	return nil
}

func (executor *unknownThenReplayProjectMemoryFullExecutor) ProjectBasisAvailable(
	context.Context,
) (bool, error) {
	return true, nil
}

func (executor *unknownThenReplayProjectMemoryFullExecutor) ValidateBundled(
	context.Context,
	typedmemorywire.ValidateRequest,
) ([]byte, error) {
	return nil, errors.New("unexpected bundled validation")
}

func (executor *unknownThenReplayProjectMemoryFullExecutor) ExecuteProjectRead(
	context.Context,
	typedmemorywire.Request,
) ([]byte, error) {
	return nil, errors.New("unexpected project read")
}

func (executor *unknownThenReplayProjectMemoryFullExecutor) Admit(
	_ context.Context,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	executor.admissionCalls++
	if executor.admissionCalls == 1 {
		executor.durableEffects++
		cause := fmt.Errorf(
			"%w for idempotency key %q",
			typedmemorystore.ErrCommitOutcomeUnknown,
			request.IdempotencyKey(),
		)
		unknown, err := projectmemory.NewAdmissionCommitOutcomeUnknown(
			executor.projectID,
			request,
			typedmemorystore.CommitReceipt{},
			cause,
		)
		if err != nil {
			return nil, err
		}
		response, err := presentProjectMemoryAdmission(
			unknown,
			request.AuthorityClass(),
		)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return append(encoded, '\n'), nil
	}
	return []byte(`{
	  "contract_version":"haft.memory.v1",
	  "action":"admit",
	  "result":"committed",
	  "persistence_disposition":{
	    "mode":"transactional_project_memory_commit",
	    "disposition":"replay",
	    "authority_granted":false
	  },
	  "receipt":{
	    "event_ref":"event:unknown-then-replay",
	    "commit_ref":"commit:unknown-then-replay",
	    "graph_revision":18,
	    "result_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	  }
	}`), nil
}

type sequencedProjectMemoryRevalidator struct {
	results []error
	calls   int
}

func (revalidator *sequencedProjectMemoryRevalidator) Revalidate(
	context.Context,
) error {
	index := revalidator.calls
	revalidator.calls++
	if index >= len(revalidator.results) {
		return nil
	}
	return revalidator.results[index]
}

type fixedProjectMemoryFullSurface struct {
	readyErr           error
	closeErr           error
	memoryHandler      fpf.MemoryToolHandler
	queryHandler       fpf.MemoryToolHandler
	readyCalls         int
	closeCalls         int
	memoryHandlerCalls int
	queryHandlerCalls  int
}

func (surface *fixedProjectMemoryFullSurface) EnsureReady(
	context.Context,
) error {
	surface.readyCalls++
	return surface.readyErr
}

func (surface *fixedProjectMemoryFullSurface) FullMCPHandler() fpf.MemoryToolHandler {
	surface.memoryHandlerCalls++
	return surface.memoryHandler
}

func (surface *fixedProjectMemoryFullSurface) ReadOnlyQueryMCPHandler() fpf.MemoryToolHandler {
	surface.queryHandlerCalls++
	return surface.queryHandler
}

func (surface *fixedProjectMemoryFullSurface) Close() error {
	surface.closeCalls++
	return surface.closeErr
}

func TestSealedProjectMemorySplitSurfaceImplementsEveryAdvertisedAction(
	t *testing.T,
) {
	tests := []struct {
		name               string
		payload            string
		wantBundledCalls   int
		wantProjectActions []string
		wantAdmissionCalls int
		querySurface       bool
	}{
		{
			name:             "bundled validate",
			payload:          string(bundledMemoryValidationFixture()),
			wantBundledCalls: 1,
		},
		{
			name:               "project validate",
			payload:            projectMemoryFullProjectValidationPayload(),
			wantProjectActions: []string{typedmemorywire.ActionValidate},
		},
		{
			name:               "resolve",
			payload:            projectMemoryFullResolveQueryPayload(),
			wantProjectActions: []string{typedmemorywire.ActionResolve},
			querySurface:       true,
		},
		{
			name:               "neighborhood",
			payload:            projectMemoryFullNeighborhoodPayload(),
			wantProjectActions: []string{typedmemorywire.ActionNeighborhood},
			querySurface:       true,
		},
		{
			name:               "recall",
			payload:            projectMemoryFullRecallPayload(),
			wantProjectActions: []string{typedmemorywire.ActionRecall},
			querySurface:       true,
		},
		{
			name:               "admit",
			payload:            projectMemoryAdmissionTestPayload,
			wantAdmissionCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revalidator := &sequencedProjectMemoryRevalidator{}
			executor := &fixedProjectMemoryFullExecutor{
				bundledResult: []byte(`{"action":"validate","result":"bundled"}`),
				projectResult: []byte(`{"result":"project-read"}`),
				admissionResult: []byte(
					`{"contract_version":"haft.memory.v1","action":"admit","result":"committed"}`,
				),
			}
			surface := &sealedProjectMemoryFullSurface{
				projectID: mustProjectMemoryRuntimeProjectID(
					t,
					"qnt_f011beef",
				),
				revalidate: revalidator.Revalidate,
				executor:   executor,
			}

			handler := surface.FullMCPHandler()
			if test.querySurface {
				handler = surface.ReadOnlyQueryMCPHandler()
			}
			result, err := handler(
				context.Background(),
				json.RawMessage(test.payload),
			)
			if err != nil {
				t.Fatalf("split-surface handler error = %v", err)
			}
			if result == "" {
				t.Fatal("split-surface handler returned an empty result")
			}
			if revalidator.calls != 2 {
				t.Fatalf(
					"ledger revalidations = %d, want pre and post",
					revalidator.calls,
				)
			}
			if executor.bundledCalls != test.wantBundledCalls {
				t.Fatalf(
					"bundled calls = %d, want %d",
					executor.bundledCalls,
					test.wantBundledCalls,
				)
			}
			if !reflect.DeepEqual(
				executor.projectActions,
				test.wantProjectActions,
			) {
				t.Fatalf(
					"project actions = %#v, want %#v",
					executor.projectActions,
					test.wantProjectActions,
				)
			}
			if executor.admissionCalls != test.wantAdmissionCalls {
				t.Fatalf(
					"admission calls = %d, want %d",
					executor.admissionCalls,
					test.wantAdmissionCalls,
				)
			}
		})
	}
}

func TestSealedProjectMemoryFullHandlerStrictlyDecodesOriginalBytes(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{}
	executor := &fixedProjectMemoryFullExecutor{}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  mustProjectMemoryRuntimeProjectID(t, "qnt_f012beef"),
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}
	payload := json.RawMessage(`{
  "contract_version":"haft.memory.v1",
  "action":"validate",
  "action":"admit",
  "basis":{"kind":"bundled_candidate_open_world"},
  "change_set":{"changes":[]}
}`)

	result, err := surface.FullMCPHandler()(
		context.Background(),
		payload,
	)
	if err == nil {
		t.Fatal("duplicate action field was accepted")
	}
	if result != "" {
		t.Fatalf("strict decoder returned result %q", result)
	}
	if revalidator.calls != 0 ||
		executor.bundledCalls != 0 ||
		executor.projectCalls != 0 ||
		executor.admissionCalls != 0 {
		t.Fatalf(
			"rejected request crossed runtime boundary: revalidations=%d executor=%#v",
			revalidator.calls,
			executor,
		)
	}
}

func TestSealedProjectMemoryQueryHandlerStrictlyDecodesOriginalBytes(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{}
	executor := &fixedProjectMemoryFullExecutor{}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  mustProjectMemoryRuntimeProjectID(t, "qnt_f012ceef"),
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}
	payload := json.RawMessage(`{
  "action":"memory",
  "memory_request":{
    "contract_version":"haft.memory.v1",
    "mode":"resolve",
    "mode":"recall",
    "basis":{"kind":"project_current"},
    "query":"authorization service",
    "max_candidates":8
  }
}`)

	result, err := surface.ReadOnlyQueryMCPHandler()(
		context.Background(),
		payload,
	)
	if err == nil {
		t.Fatal("duplicate query mode field was accepted")
	}
	if result != "" {
		t.Fatalf("strict query decoder returned result %q", result)
	}
	if revalidator.calls != 0 ||
		executor.bundledCalls != 0 ||
		executor.projectCalls != 0 ||
		executor.admissionCalls != 0 {
		t.Fatalf(
			"rejected query crossed runtime boundary: revalidations=%d executor=%#v",
			revalidator.calls,
			executor,
		)
	}
}

func TestSealedProjectMemorySurfaceRequiresRestartAfterEnablement(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{}
	executor := &fixedProjectMemoryFullExecutor{
		basisAvailable: true,
		projectResult:  []byte(`{"must_not":"run"}`),
	}
	surface := &sealedProjectMemoryFullSurface{
		projectID:       mustProjectMemoryRuntimeProjectID(t, "qnt_f112ceef"),
		revalidate:      revalidator.Revalidate,
		executor:        executor,
		readyAtStartup:  false,
		startupObserved: true,
	}

	result, err := surface.ReadOnlyQueryMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryFullResolveQueryPayload()),
	)
	if err != nil {
		t.Fatalf("restart recovery error = %v", err)
	}
	for _, expected := range []string{
		`"result_kind":"restart_required"`,
		`"performed":false`,
		`"tool":"haft_onboard"`,
		`"action":"status"`,
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("restart recovery lacks %s: %s", expected, result)
		}
	}
	if executor.projectCalls != 0 ||
		executor.admissionCalls != 0 ||
		executor.bundledCalls != 0 {
		t.Fatalf(
			"stale process crossed memory effect boundary: %#v",
			executor,
		)
	}
}

func TestSealedProjectMemoryDedicatedHandlerRejectsReadActions(
	t *testing.T,
) {
	tests := []struct {
		name    string
		payload string
		action  string
	}{
		{
			name:    "resolve",
			payload: memoryResolveReadPayload(),
			action:  typedmemorywire.ActionResolve,
		},
		{
			name:    "recall",
			payload: projectMemoryFullDirectRecallPayload(),
			action:  typedmemorywire.ActionRecall,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revalidator := &sequencedProjectMemoryRevalidator{}
			executor := &fixedProjectMemoryFullExecutor{}
			surface := &sealedProjectMemoryFullSurface{
				projectID: mustProjectMemoryRuntimeProjectID(
					t,
					"qnt_f012deef",
				),
				revalidate: revalidator.Revalidate,
				executor:   executor,
			}

			result, err := surface.FullMCPHandler()(
				context.Background(),
				json.RawMessage(test.payload),
			)
			if err == nil ||
				!strings.Contains(
					err.Error(),
					"unavailable on the validate/admit boundary",
				) {
				t.Fatalf(
					"dedicated handler %s error = %v",
					test.action,
					err,
				)
			}
			if result != "" {
				t.Fatalf(
					"dedicated handler %s returned %q",
					test.action,
					result,
				)
			}
			if revalidator.calls != 0 ||
				executor.projectCalls != 0 ||
				executor.admissionCalls != 0 {
				t.Fatalf(
					"rejected %s crossed runtime boundary: revalidations=%d executor=%#v",
					test.action,
					revalidator.calls,
					executor,
				)
			}
		})
	}
}

func TestSealedProjectMemoryFullHandlerPreEffectIdentityFailureWritesNothing(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{
		results: []error{errors.New("injected root-to-ledger drift")},
	}
	executor := &fixedProjectMemoryFullExecutor{}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  mustProjectMemoryRuntimeProjectID(t, "qnt_f013beef"),
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}

	result, err := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if err == nil ||
		!strings.Contains(err.Error(), "before haft_memory(admit)") {
		t.Fatalf("pre-effect identity error = %v", err)
	}
	if result != "" {
		t.Fatalf("pre-effect identity failure returned %q", result)
	}
	if executor.admissionCalls != 0 {
		t.Fatalf(
			"pre-effect identity failure admitted %d time(s)",
			executor.admissionCalls,
		)
	}
}

func TestSealedProjectMemoryFullHandlerDiscardsReadAfterIdentityFailure(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{
		results: []error{
			nil,
			errors.New("injected post-read root-to-ledger drift"),
		},
	}
	executor := &fixedProjectMemoryFullExecutor{
		projectResult: []byte(`{"result":"must-not-escape"}`),
	}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  mustProjectMemoryRuntimeProjectID(t, "qnt_f014beef"),
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}

	result, err := surface.ReadOnlyQueryMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryFullResolveQueryPayload()),
	)
	if err == nil ||
		!strings.Contains(err.Error(), "discard untrusted read result") {
		t.Fatalf("post-read identity error = %v", err)
	}
	if result != "" {
		t.Fatalf("untrusted read result escaped as %q", result)
	}
}

func TestSealedProjectMemoryFullHandlerPreservesAdmissionOnPostEffectAmbiguity(
	t *testing.T,
) {
	revalidator := &sequencedProjectMemoryRevalidator{
		results: []error{
			nil,
			errors.New("injected post-admission root-to-ledger drift"),
		},
	}
	admissionResult := []byte(
		`{"contract_version":"haft.memory.v1","action":"admit","result":"committed","receipt":{"commit_ref":"commit:test"}}` + "\n",
	)
	executor := &fixedProjectMemoryFullExecutor{
		admissionResult: admissionResult,
	}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  mustProjectMemoryRuntimeProjectID(t, "qnt_f015beef"),
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}

	result, err := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if err != nil {
		t.Fatalf(
			"qualified admission result was collapsed into MCP error: %v",
			err,
		)
	}
	response := struct {
		ContractVersion       string          `json:"contract_version"`
		Action                string          `json:"action"`
		Result                string          `json:"result"`
		AdmissionResult       json.RawMessage `json:"admission_result"`
		AdmissionResultDigest string          `json:"admission_result_canonical_digest"`
		Revalidation          struct {
			Kind   string `json:"kind"`
			Code   string `json:"code"`
			Repair string `json:"repair"`
		} `json:"post_effect_ledger_revalidation"`
		Interpretation struct {
			DoesNotEstablish []string `json:"does_not_establish"`
			DoesNotAuthorize []string `json:"does_not_authorize"`
		} `json:"interpretation"`
	}{}
	if decodeErr := json.Unmarshal([]byte(result), &response); decodeErr != nil {
		t.Fatalf("decode qualified admission result: %v", decodeErr)
	}
	if response.ContractVersion != projectMemoryFullDeliveryContract ||
		response.Action != typedmemorywire.ActionAdmit ||
		response.Result !=
			"admission_result_with_delivery_qualification" {
		t.Fatalf("qualified response header = %#v", response)
	}
	if !bytes.Equal(
		bytes.TrimSpace(response.AdmissionResult),
		bytes.TrimSpace(admissionResult),
	) {
		t.Fatalf(
			"nested admission result = %s, want %s",
			response.AdmissionResult,
			admissionResult,
		)
	}
	wantResultDigest, digestErr :=
		canonicalProjectMemoryFullJSONDigest(admissionResult)
	if digestErr != nil {
		t.Fatalf("digest admission result fixture: %v", digestErr)
	}
	if response.AdmissionResultDigest != wantResultDigest {
		t.Fatalf(
			"admission result digest = %q, want %q",
			response.AdmissionResultDigest,
			wantResultDigest,
		)
	}
	if response.Revalidation.Kind != "failed_after_effect" ||
		response.Revalidation.Code !=
			"project_ledger_revalidation_failed" ||
		response.Revalidation.Repair == "" {
		t.Fatalf(
			"post-effect revalidation = %#v",
			response.Revalidation,
		)
	}
	if len(response.Interpretation.DoesNotEstablish) == 0 ||
		len(response.Interpretation.DoesNotAuthorize) == 0 {
		t.Fatalf(
			"qualified interpretation = %#v",
			response.Interpretation,
		)
	}
}

func TestSealedProjectMemoryFullHandlerReturnsClosedUnknownAndSameKeyReplay(
	t *testing.T,
) {
	postLedgerErr := errors.New(
		"injected post-admission root-to-ledger drift",
	)
	revalidator := &sequencedProjectMemoryRevalidator{
		results: []error{
			nil,
			postLedgerErr,
			nil,
			nil,
		},
	}
	projectID := mustProjectMemoryRuntimeProjectID(t, "qnt_f016beef")
	executor := &unknownThenReplayProjectMemoryFullExecutor{
		projectID: projectID,
	}
	surface := &sealedProjectMemoryFullSurface{
		projectID:  projectID,
		revalidate: revalidator.Revalidate,
		executor:   executor,
	}
	request, decodeErr := typedmemorywire.DecodeAdmitRequest(
		[]byte(projectMemoryAdmissionTestPayload),
	)
	if decodeErr != nil {
		t.Fatalf("decode admission fixture: %v", decodeErr)
	}
	unknownResult, retryErr :=
		projectmemory.NewAdmissionCommitOutcomeUnknown(
			projectID,
			request,
			typedmemorystore.CommitReceipt{},
			fmt.Errorf("%w: fixture", typedmemorystore.ErrCommitOutcomeUnknown),
		)
	if retryErr != nil {
		t.Fatalf("construct expected unknown result: %v", retryErr)
	}
	wantRetry := presentProjectMemoryAdmissionRetry(
		unknownResult.RetryCoordinates(),
	)

	first, firstErr := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if firstErr != nil {
		t.Fatalf("closed unknown outcome became MCP error: %v", firstErr)
	}
	unknown := struct {
		ContractVersion string                                  `json:"contract_version"`
		Action          string                                  `json:"action"`
		Result          string                                  `json:"result"`
		AdmissionResult projectMemoryAdmissionResponse          `json:"admission_result"`
		Operation       projectMemoryFullAdmissionOperation     `json:"admission_operation"`
		Revalidation    projectMemoryFullLedgerRevalidation     `json:"post_effect_ledger_revalidation"`
		Interpretation  projectMemoryFullDeliveryInterpretation `json:"interpretation"`
	}{}
	if err := json.Unmarshal([]byte(first), &unknown); err != nil {
		t.Fatalf("decode closed unknown outcome: %v", err)
	}
	if unknown.ContractVersion != projectMemoryFullDeliveryContract ||
		unknown.Action != typedmemorywire.ActionAdmit ||
		unknown.Result != "admission_result_with_delivery_qualification" ||
		unknown.Operation.Kind != "result_returned" ||
		unknown.AdmissionResult.Result !=
			projectmemory.AdmissionCommitOutcomeUnknownResult {
		t.Fatalf("unknown outcome header = %#v", unknown)
	}
	if unknown.AdmissionResult.Retry == nil ||
		!reflect.DeepEqual(*unknown.AdmissionResult.Retry, wantRetry) {
		t.Fatalf(
			"unknown retry coordinates = %#v, want %#v",
			unknown.AdmissionResult.Retry,
			wantRetry,
		)
	}
	if unknown.Revalidation.Kind != "failed_after_effect" ||
		unknown.Revalidation.Code != "project_ledger_revalidation_failed" ||
		unknown.Revalidation.Repair == "" {
		t.Fatalf(
			"unknown post-effect revalidation = %#v",
			unknown.Revalidation,
		)
	}
	if len(unknown.AdmissionResult.Interpretation.Establishes) == 0 ||
		len(unknown.AdmissionResult.Interpretation.Omits) == 0 ||
		len(unknown.AdmissionResult.Interpretation.DoesNotAuthorize) == 0 ||
		len(unknown.Interpretation.Establishes) == 0 ||
		len(unknown.Interpretation.DoesNotEstablish) == 0 ||
		len(unknown.Interpretation.DoesNotAuthorize) == 0 {
		t.Fatalf(
			"unknown interpretation = %#v",
			unknown,
		)
	}

	second, secondErr := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryAdmissionTestPayload),
	)
	if secondErr != nil {
		t.Fatalf("same-key replay error = %v", secondErr)
	}
	replayed := struct {
		Result      string `json:"result"`
		Persistence struct {
			Disposition string `json:"disposition"`
		} `json:"persistence_disposition"`
		Receipt struct {
			EventRef  string `json:"event_ref"`
			CommitRef string `json:"commit_ref"`
		} `json:"receipt"`
	}{}
	if err := json.Unmarshal([]byte(second), &replayed); err != nil {
		t.Fatalf("decode same-key replay: %v", err)
	}
	if replayed.Result != "committed" ||
		replayed.Persistence.Disposition != "replay" ||
		replayed.Receipt.EventRef == "" ||
		replayed.Receipt.CommitRef == "" {
		t.Fatalf("same-key replay result = %#v", replayed)
	}
	if executor.admissionCalls != 2 || executor.durableEffects != 1 {
		t.Fatalf(
			"same-key retry calls/effects = %d/%d, want 2/1",
			executor.admissionCalls,
			executor.durableEffects,
		)
	}
	if revalidator.calls != 4 {
		t.Fatalf(
			"same-key retry ledger revalidations = %d, want 4",
			revalidator.calls,
		)
	}
}

func TestProjectMemoryFullAdmissionIdentityIgnoresJSONPresentation(
	t *testing.T,
) {
	projectID := mustProjectMemoryRuntimeProjectID(t, "qnt_f017beef")
	v1 := projectMemoryAdmissionRetryCoordinatesForPresentation(
		t,
		projectID,
		typedmemorywire.ContractVersionV1,
	)
	v2 := projectMemoryAdmissionRetryCoordinatesForPresentation(
		t,
		projectID,
		typedmemorywire.ContractVersionV2,
	)
	if v1.ContractVersion() != typedmemorystore.AdmissionContractV1() {
		t.Fatalf("v1 retry contract version = %q", v1.ContractVersion().String())
	}
	if v2.ContractVersion() != typedmemorystore.AdmissionContractV2() {
		t.Fatalf("v2 retry contract version = %q", v2.ContractVersion().String())
	}
	if v1.CandidateDigest() != v2.CandidateDigest() {
		t.Fatalf(
			"version-only request change altered semantic candidate identity:\nv1=%q\nv2=%q",
			v1.CandidateDigest().String(),
			v2.CandidateDigest().String(),
		)
	}
	if v1.RequestIdentityDigest() == v2.RequestIdentityDigest() {
		t.Fatalf(
			"v1 and v2 requests collapsed to one retry identity %q",
			v1.RequestIdentityDigest().String(),
		)
	}
}

func projectMemoryAdmissionRetryCoordinatesForPresentation(
	t *testing.T,
	projectID projectidentity.ProjectID,
	contractVersion string,
) projectmemory.AdmissionRetryCoordinates {
	t.Helper()
	orderedPayload := strings.Replace(
		projectMemoryAdmissionTestPayload,
		typedmemorywire.ContractVersionV2,
		contractVersion,
		1,
	)
	reorderedPayload := fmt.Sprintf(`{
	  "change_set":{"changes":[{
	    "provenance":"provenance:cli-admission-shell-fixture-change",
	    "label":"CLI admission shell fixture",
	    "context":"haft-project",
	    "local_ref":"local:cli-admission-shell-fixture",
	    "entity_id":"entity:cli-admission-shell-fixture",
	    "kind":"declare_entity"
	  }]},
	  "request_provenance_ref":"provenance:cli-admission-shell-fixture",
	  "idempotency_key":"cli-admission-shell-fixture",
	  "authority_class":"non_binding_semantic_assertion",
	  "basis":{
	    "graph_revision":17,
	    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	    "kind":"exact_project"
	  },
	  "action":"admit",
	  "contract_version":%q
	}`, contractVersion)
	firstRequest, err := typedmemorywire.DecodeAdmitRequest(
		[]byte(orderedPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest, err := typedmemorywire.DecodeAdmitRequest(
		[]byte(reorderedPayload),
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := projectmemory.NewAdmissionCommitOutcomeUnknown(
		projectID,
		firstRequest,
		typedmemorystore.CommitReceipt{},
		fmt.Errorf("%w: first", typedmemorystore.ErrCommitOutcomeUnknown),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := projectmemory.NewAdmissionCommitOutcomeUnknown(
		projectID,
		secondRequest,
		typedmemorystore.CommitReceipt{},
		fmt.Errorf("%w: second", typedmemorystore.ErrCommitOutcomeUnknown),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		first.RetryCoordinates(),
		second.RetryCoordinates(),
	) {
		t.Fatalf(
			"presentation-equivalent requests changed retry identity:\nfirst=%#v\nsecond=%#v",
			first,
			second,
		)
	}
	return first.RetryCoordinates()
}

func TestProjectMemoryFullSurfaceIsInstalledOnlyAfterExactReadiness(
	t *testing.T,
) {
	t.Run("ready", func(t *testing.T) {
		server := fpf.NewServer("test")
		surface := &fixedProjectMemoryFullSurface{
			memoryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "memory-fixture", nil
			},
			queryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "query-fixture", nil
			},
		}

		err := installProjectMemoryFullSurface(
			context.Background(),
			server,
			surface,
		)
		if err != nil {
			t.Fatal(err)
		}
		if surface.readyCalls != 1 ||
			surface.memoryHandlerCalls != 1 ||
			surface.queryHandlerCalls != 1 {
			t.Fatalf("surface calls = %#v", surface)
		}
		assertMemoryFullSchemaInstalled(t, server)
	})

	t.Run("not ready", func(t *testing.T) {
		server := fpf.NewServer("test")
		server.SetMemoryHandler(
			func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "validation", nil
			},
		)
		surface := &fixedProjectMemoryFullSurface{
			readyErr: errors.New("selected TypeEnv head is unavailable"),
			memoryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "must-not-install", nil
			},
			queryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "must-not-install", nil
			},
		}

		err := installProjectMemoryFullSurface(
			context.Background(),
			server,
			surface,
		)
		if err == nil {
			t.Fatal("unready full project-memory surface was installed")
		}
		if surface.readyCalls != 1 ||
			surface.memoryHandlerCalls != 0 ||
			surface.queryHandlerCalls != 0 {
			t.Fatalf("surface calls = %#v", surface)
		}
		assertMemoryValidationSchemaRetained(t, server)
	})

	t.Run("missing query handler installs neither half", func(t *testing.T) {
		server := fpf.NewServer("test")
		server.SetMemoryHandler(
			func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "validation", nil
			},
		)
		surface := &fixedProjectMemoryFullSurface{
			memoryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "must-not-install", nil
			},
		}

		err := installProjectMemoryFullSurface(
			context.Background(),
			server,
			surface,
		)
		if err == nil ||
			!strings.Contains(err.Error(), "query-read handler is required") {
			t.Fatalf("missing query handler error = %v", err)
		}
		assertMemoryValidationSchemaRetained(t, server)
		assertMemoryQueryRecoveryAdvertised(t, server)
	})

	t.Run("missing memory handler installs neither half", func(t *testing.T) {
		server := fpf.NewServer("test")
		surface := &fixedProjectMemoryFullSurface{
			queryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "must-not-install", nil
			},
		}

		err := installProjectMemoryFullSurface(
			context.Background(),
			server,
			surface,
		)
		if err == nil ||
			!strings.Contains(
				err.Error(),
				"validate/admit handler is required",
			) {
			t.Fatalf("missing memory handler error = %v", err)
		}
		assertMemoryQueryRecoveryAdvertised(t, server)
	})
}

func TestConfigureServeProjectMemoryFullSurfaceOwnsReadinessAndCleanup(
	t *testing.T,
) {
	original := openServeProjectMemoryFullSurface
	t.Cleanup(func() {
		openServeProjectMemoryFullSurface = original
	})

	t.Run("ready surface remains owned by serve", func(t *testing.T) {
		server := fpf.NewServer("test")
		surface := &fixedProjectMemoryFullSurface{
			memoryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "memory", nil
			},
			queryHandler: func(
				context.Context,
				json.RawMessage,
			) (string, error) {
				return "query", nil
			},
		}
		openServeProjectMemoryFullSurface = func(
			context.Context,
			ProjectBinding,
		) (serveProjectMemoryFullSurface, error) {
			return surface, nil
		}

		configured, err := configureServeProjectMemoryFullSurface(
			context.Background(),
			server,
			ProjectBinding{},
		)
		if err != nil {
			t.Fatal(err)
		}
		if configured != surface || surface.closeCalls != 0 {
			t.Fatalf(
				"configured surface = %#v, fixture = %#v",
				configured,
				surface,
			)
		}
		assertMemoryFullSchemaInstalled(t, server)
		if err := configured.Close(); err != nil {
			t.Fatal(err)
		}
		if surface.closeCalls != 1 {
			t.Fatalf("close calls = %d, want 1", surface.closeCalls)
		}
	})

	t.Run("unready surface is closed and not advertised", func(t *testing.T) {
		server := fpf.NewServer("test")
		server.SetMemoryHandler(func(
			context.Context,
			json.RawMessage,
		) (string, error) {
			return "validation", nil
		})
		surface := &fixedProjectMemoryFullSurface{
			readyErr: errors.New("selected TypeEnv head is unavailable"),
		}
		openServeProjectMemoryFullSurface = func(
			context.Context,
			ProjectBinding,
		) (serveProjectMemoryFullSurface, error) {
			return surface, nil
		}

		configured, err := configureServeProjectMemoryFullSurface(
			context.Background(),
			server,
			ProjectBinding{},
		)
		if err == nil || configured != nil {
			t.Fatalf(
				"unready surface = %#v, error = %v",
				configured,
				err,
			)
		}
		if surface.closeCalls != 1 {
			t.Fatalf("close calls = %d, want 1", surface.closeCalls)
		}
		assertMemoryValidationSchemaRetained(t, server)
		assertMemoryQueryRecoveryAdvertised(t, server)
	})
}

func TestOpenSealedProjectMemoryFullSurfaceDoesNotMutateCurrentLedger(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_9eadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)

	surface, err := openSealedProjectMemoryFullSurface(
		context.Background(),
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("openSealedProjectMemoryFullSurface() error = %v", err)
	}
	if err := surface.EnsureReady(context.Background()); err != nil {
		_ = surface.Close()
		t.Fatalf(
			"headless project did not expose typed recovery surface: %v",
			err,
		)
	}
	recovery, err := surface.ReadOnlyQueryMCPHandler()(
		context.Background(),
		json.RawMessage(projectMemoryFullResolveQueryPayload()),
	)
	if err != nil {
		_ = surface.Close()
		t.Fatalf("headless project recovery query: %v", err)
	}
	for _, expected := range []string{
		`"result_kind":"project_basis_unavailable"`,
		`"performed":false`,
		`"tool":"haft_onboard"`,
		`"action":"status"`,
	} {
		if !strings.Contains(recovery, expected) {
			_ = surface.Close()
			t.Fatalf(
				"headless recovery lacks %s: %s",
				expected,
				recovery,
			)
		}
	}
	if err := surface.Close(); err != nil {
		t.Fatalf("close full surface: %v", err)
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("full-surface open changed SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("full-surface open changed project-store files")
	}
}

func TestOpenSealedProjectMemoryFullSurfaceUsesResolvedServeBinding(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_9ebdbeef")
	decoyRoot := t.TempDir()
	t.Setenv(envProjectRoot, decoyRoot)
	t.Setenv(envLegacyProjectRoot, decoyRoot)
	t.Chdir(decoyRoot)

	rootInput, err := projectRootInputFromExplicitOrEnv(
		fixture.binding.ProjectRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rootInput.Source != projectRootSourceFlag {
		t.Fatalf(
			"explicit serve root source = %q, want %q",
			rootInput.Source,
			projectRootSourceFlag,
		)
	}
	binding, err := resolveProjectBindingFromInput(
		rootInput,
		fixture.binding.ProjectID,
	)
	if err != nil {
		t.Fatalf("resolve explicit serve binding: %v", err)
	}

	// If the dormant opener rediscovered process state, either the decoy root
	// or this conflicting environment guard would win. It must consume only
	// the already-resolved binding.
	t.Setenv(envExpectedProjectID, "qnt_9ecdbeef")
	surface, err := openSealedProjectMemoryFullSurface(
		context.Background(),
		binding,
	)
	if err != nil {
		t.Fatalf("open full surface from explicit serve binding: %v", err)
	}
	if surface.projectID.String() != fixture.binding.ProjectID {
		_ = surface.Close()
		t.Fatalf(
			"full surface project = %q, want %q",
			surface.projectID.String(),
			fixture.binding.ProjectID,
		)
	}
	if err := surface.EnsureReady(context.Background()); err != nil {
		_ = surface.Close()
		t.Fatalf(
			"headless explicit project did not expose typed recovery surface: %v",
			err,
		)
	}
	if err := surface.Close(); err != nil {
		t.Fatalf("close full surface: %v", err)
	}
}

func TestOpenSealedProjectMemoryFullSurfaceRejectsExpectedProjectIDMismatch(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_9fbdbeef")
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	binding := fixture.binding
	binding.ExpectedProjectID = "qnt_9fcdbabe"

	surface, err := openSealedProjectMemoryFullSurface(
		context.Background(),
		binding,
	)
	if surface != nil {
		_ = surface.Close()
		t.Fatal("identity-mismatched full surface was returned")
	}
	if !errors.Is(err, errExpectedProjectIDMiss) {
		t.Fatalf(
			"identity mismatch error = %v, want errExpectedProjectIDMiss",
			err,
		)
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("identity-mismatched full-surface open changed SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("identity-mismatched full-surface open changed project-store files")
	}
}

func TestSealedProjectMemoryFullHandlerRejectsSchemaDriftWithoutRepair(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_9fadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	surface, err := openSealedProjectMemoryFullSurface(
		context.Background(),
		fixture.binding,
	)
	if err != nil {
		t.Fatalf("openSealedProjectMemoryFullSurface() error = %v", err)
	}
	t.Cleanup(func() { _ = surface.Close() })
	executeReadOnlyProjectValidationFixtureSQL(
		t,
		fixture.database,
		"DELETE FROM schema_version WHERE version = 49",
	)

	result, err := surface.FullMCPHandler()(
		context.Background(),
		json.RawMessage(bundledMemoryValidationFixture()),
	)
	if err == nil ||
		!strings.Contains(err.Error(), "kernel schema is not current") {
		t.Fatalf("schema-drift handler error = %v", err)
	}
	if result != "" {
		t.Fatalf("schema-drift handler returned %q", result)
	}
	if projectMemoryFullKernelVersionPresent(
		t,
		fixture.database,
		49,
	) {
		t.Fatal("full handler silently repaired missing schema version 49")
	}
}

func projectMemoryFullKernelVersionPresent(
	t *testing.T,
	databasePath string,
	version int,
) bool {
	t.Helper()
	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{
		Scheme:   "file",
		Path:     databasePath,
		RawQuery: query.Encode(),
	}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	count := 0
	err = database.
		QueryRow(
			`SELECT COUNT(*)
			 FROM schema_version
			 WHERE version = ?`,
			version,
		).
		Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count == 1
}

func assertMemoryFullSchemaInstalled(
	t *testing.T,
	server *fpf.Server,
) {
	t.Helper()
	queryFound := false
	memoryFound := false
	for _, tool := range server.ToolCatalog() {
		if tool.Name == "haft_query" {
			queryFound = true
			assertMemoryQueryModes(t, tool)
		}
		if tool.Name != "haft_memory" {
			continue
		}
		memoryFound = true
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			t.Fatalf("haft_memory full schema = %T", tool.InputSchema)
		}
		got := memoryFullSchemaActionSet(t, schema)
		for _, want := range []string{
			typedmemorywire.ActionValidate,
			typedmemorywire.ActionAdmit,
		} {
			if !got[want] {
				t.Fatalf(
					"haft_memory full action enum = %#v, missing %q",
					got,
					want,
				)
			}
		}
		if len(got) != 2 {
			t.Fatalf("haft_memory full action enum = %#v", got)
		}
	}
	if !memoryFound {
		t.Fatal("haft_memory full schema is absent")
	}
	if !queryFound {
		t.Fatal("haft_query memory schema is absent")
	}
}

func memoryFullSchemaActionSet(
	t *testing.T,
	schema map[string]interface{},
) map[string]bool {
	t.Helper()
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_memory full properties = %#v", schema)
	}
	request, ok := properties["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_memory full request = %#v", properties["request"])
	}
	rawVariants, ok := request["oneOf"].([]interface{})
	if !ok {
		t.Fatalf("haft_memory full request.oneOf = %#v", request)
	}
	got := map[string]bool{}
	for _, rawVariant := range rawVariants {
		variant, variantOK := rawVariant.(map[string]interface{})
		if !variantOK {
			t.Fatalf("haft_memory request variant = %#v", rawVariant)
		}
		variantProperties, propertiesOK :=
			variant["properties"].(map[string]interface{})
		if !propertiesOK {
			t.Fatalf("haft_memory request variant properties = %#v", variant)
		}
		action, actionOK :=
			variantProperties["action"].(map[string]interface{})
		if !actionOK {
			t.Fatalf(
				"haft_memory request variant action = %#v",
				variantProperties["action"],
			)
		}
		rawActions, enumOK := action["enum"].([]interface{})
		if !enumOK || len(rawActions) != 1 {
			t.Fatalf("haft_memory request variant action enum = %#v", action)
		}
		value, stringValue := rawActions[0].(string)
		if !stringValue {
			t.Fatalf("haft_memory action value = %#v", rawActions[0])
		}
		got[value] = true
	}
	return got
}

func assertMemoryQueryRecoveryAdvertised(
	t *testing.T,
	server *fpf.Server,
) {
	t.Helper()
	for _, tool := range server.ToolCatalog() {
		if tool.Name != "haft_query" {
			continue
		}
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			t.Fatalf("haft_query schema = %T", tool.InputSchema)
		}
		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("haft_query properties = %#v", schema)
		}
		action, ok := properties["action"].(map[string]interface{})
		if !ok {
			t.Fatalf("haft_query action = %#v", properties["action"])
		}
		rawActions, ok := action["enum"].([]interface{})
		if !ok {
			t.Fatalf("haft_query action enum = %#v", action)
		}
		memoryFound := false
		for _, raw := range rawActions {
			value, stringValue := raw.(string)
			if !stringValue {
				t.Fatalf("haft_query action value = %#v", raw)
			}
			if value == "memory" {
				memoryFound = true
			}
		}
		if !memoryFound {
			t.Fatal("haft_query memory recovery disappeared after failed install")
		}
		return
	}
	t.Fatal("haft_query disappeared after failed install")
}

func projectMemoryFullProjectValidationPayload() string {
	return `{
  "contract_version":"haft.memory.v1",
  "action":"validate",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":1
  },
  "change_set":{"changes":[{
    "kind":"declare_entity",
    "entity_id":"entity:full-handler-project-validation",
    "local_ref":"local:full-handler-project-validation",
    "context":"haft-project",
    "label":"Full handler project validation",
    "provenance":"provenance:full-handler-project-validation"
  }]}
}`
}

func projectMemoryFullResolveQueryPayload() string {
	return `{
  "action":"memory",
  "memory_request":{
    "contract_version":"haft.memory.v1",
    "mode":"resolve",
    "basis":{
      "kind":"exact_project",
      "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "graph_revision":1
    },
    "query":"authorization service",
    "max_candidates":8
  }
}`
}

func projectMemoryFullNeighborhoodPayload() string {
	return `{
  "action":"memory",
  "memory_request":{
    "contract_version":"haft.memory.v1",
    "mode":"neighborhood",
    "basis":{"kind":"project_current"},
    "entity_ref":{
      "ref_kind_id":"refkind:project-entity",
      "reference_id":"entity:authorization-service"
    },
    "bounded_context_ref":"haft-project",
    "view":{
      "projection_profile_ref":"agent_orientation.v1",
      "requested_facets":["problems","decisions"],
      "detail":"standard",
      "include_history":false
    },
    "read_budget":{
      "max_facets":2,
      "max_items_per_facet":8,
      "max_relation_paths_per_item":4,
      "max_carrier_excerpt_characters":2048,
      "max_provenance_depth":3
    }
  }
}`
}

func projectMemoryFullRecallPayload() string {
	return `{
  "action":"memory",
  "memory_request":{
    "contract_version":"haft.memory.v1",
    "mode":"recall",
    "basis":{"kind":"project_current"},
    "entity_ref":{
      "ref_kind_id":"refkind:project-entity",
      "reference_id":"entity:authorization-service"
    },
    "bounded_context_ref":"haft-project",
    "view":{
      "projection_profile_ref":"agent_orientation.v1",
      "requested_facets":["problems","decisions"],
      "detail":"standard",
      "include_history":false
    },
    "read_budget":{
      "max_facets":2,
      "max_items_per_facet":8,
      "max_relation_paths_per_item":4,
      "max_carrier_excerpt_characters":2048,
      "max_provenance_depth":3
    },
    "query":"authorization decisions",
    "candidate_budget":{"max_candidates":8}
  }
}`
}

func projectMemoryFullDirectRecallPayload() string {
	return `{
  "contract_version":"haft.memory.v1",
  "action":"recall",
  "basis":{"kind":"project_current"},
  "entity_ref":{
    "ref_kind_id":"refkind:project-entity",
    "reference_id":"entity:authorization-service"
  },
  "bounded_context_ref":"haft-project",
  "view":{
    "projection_profile_ref":"agent_orientation.v1",
    "requested_facets":["problems","decisions"],
    "detail":"standard",
    "include_history":false
  },
  "read_budget":{
    "max_facets":2,
    "max_items_per_facet":8,
    "max_relation_paths_per_item":4,
    "max_carrier_excerpt_characters":2048,
    "max_provenance_depth":3
  },
  "query":"authorization decisions",
  "candidate_budget":{"max_candidates":8}
}`
}
