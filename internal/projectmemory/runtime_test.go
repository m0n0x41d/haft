package projectmemory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type runtimeContextKey string

type recordingBasisSource struct {
	resolution typedmemoryvalidation.BasisResolution
	err        error
	calls      int
	projectID  projectledger.ProjectID
	selector   typedmemorywire.BasisSelector
	context    context.Context
	afterCall  func()
}

type nilableBasisSource map[string]typedmemoryvalidation.BasisResolution

func (source nilableBasisSource) ResolveProjectBasis(
	context.Context,
	projectledger.ProjectID,
	typedmemorywire.BasisSelector,
) (typedmemoryvalidation.BasisResolution, error) {
	return source["resolution"], nil
}

func (source *recordingBasisSource) ResolveProjectBasis(
	ctx context.Context,
	projectID projectledger.ProjectID,
	selector typedmemorywire.BasisSelector,
) (typedmemoryvalidation.BasisResolution, error) {
	source.calls++
	source.projectID = projectID
	source.selector = selector
	source.context = ctx
	if source.afterCall != nil {
		source.afterCall()
	}
	return source.resolution, source.err
}

func TestValidationRuntimeBindsTrustedProjectAndCallerContext(t *testing.T) {
	t.Parallel()

	projectID := runtimeProjectID(t)
	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
	}
	runtime := runtimeValidationRuntime(t, projectID, source)
	request := runtimeRequest(t, `{"kind":"project_current"}`)
	key := runtimeContextKey("request")
	ctx := context.WithValue(context.Background(), key, "kept")

	response, err := runtime.Validate(ctx, request)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if source.calls != 1 {
		t.Fatalf("basis source calls = %d, want 1", source.calls)
	}
	if source.projectID != projectID {
		t.Fatalf("basis source project = %q, want %q", source.projectID.String(), projectID.String())
	}
	if source.selector.Kind() != typedmemorywire.BasisProjectCurrent {
		t.Fatalf("basis source selector = %q", source.selector.Kind())
	}
	if source.context.Value(key) != "kept" {
		t.Fatal("basis source did not receive the caller context")
	}
	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("response verdict = %q, want underdetermined", response.Verdict())
	}
	if response.PersistenceDisposition().RowsWritten() != 0 {
		t.Fatal("read-only validation reported a write")
	}
	if response.PersistenceDisposition().AuthorityGranted() {
		t.Fatal("read-only validation reported admission authority")
	}
}

func TestValidationRuntimeFailsOperationalErrorsWithoutSemanticFallback(t *testing.T) {
	t.Parallel()

	operational := errors.New("fixture database read failed")
	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
		err:        operational,
	}
	runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
	request := runtimeRequest(t, `{"kind":"project_current"}`)

	response, err := runtime.Validate(context.Background(), request)
	if !errors.Is(err, operational) {
		t.Fatalf("Validate() error = %v, want operational error", err)
	}
	if response != nil {
		t.Fatalf("Validate() response = %T, want nil", response)
	}
}

func TestValidationRuntimeRejectsMissingAndTypedNilResolution(t *testing.T) {
	t.Parallel()

	var typedNil *typedmemoryvalidation.ProjectBasisUnavailable
	tests := []struct {
		name       string
		resolution typedmemoryvalidation.BasisResolution
	}{
		{name: "nil interface", resolution: nil},
		{name: "typed nil", resolution: typedNil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := &recordingBasisSource{resolution: test.resolution}
			runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
			request := runtimeRequest(t, `{"kind":"project_current"}`)

			response, err := runtime.Validate(context.Background(), request)
			if !errors.Is(err, ErrProjectBasisResolutionEmpty) {
				t.Fatalf("Validate() error = %v, want ErrProjectBasisResolutionEmpty", err)
			}
			if response != nil {
				t.Fatalf("Validate() response = %T, want nil", response)
			}
		})
	}
}

func TestValidationRuntimeRejectsStructurallyInvalidResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		basis      string
		resolution typedmemoryvalidation.BasisResolution
	}{
		{
			name:       "zero resolved project basis",
			basis:      `{"kind":"project_current"}`,
			resolution: &typedmemoryvalidation.ResolvedProjectBasis{},
		},
		{
			name: "zero exact mismatch",
			basis: fmt.Sprintf(
				`{"kind":"exact_project","type_env_digest":"sha256:%s","graph_revision":5}`,
				strings.Repeat("f", 64),
			),
			resolution: &typedmemoryvalidation.ExactProjectBasisMismatch{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := &recordingBasisSource{resolution: test.resolution}
			runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
			request := runtimeRequest(t, test.basis)

			response, err := runtime.Validate(context.Background(), request)
			if !errors.Is(err, ErrProjectBasisUncorrelated) {
				t.Fatalf("Validate() error = %v, want ErrProjectBasisUncorrelated", err)
			}
			if response != nil {
				t.Fatalf("Validate() response = %T, want nil", response)
			}
		})
	}
}

func TestValidationRuntimeRejectsUncorrelatedResolution(t *testing.T) {
	t.Parallel()

	mismatch := runtimeMismatch(t, strings.Repeat("a", 64), 7)
	source := &recordingBasisSource{resolution: mismatch}
	runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
	request := runtimeRequest(t, `{"kind":"project_current"}`)

	response, err := runtime.Validate(context.Background(), request)
	if !errors.Is(err, ErrProjectBasisUncorrelated) {
		t.Fatalf("Validate() error = %v, want ErrProjectBasisUncorrelated", err)
	}
	if response != nil {
		t.Fatalf("Validate() response = %T, want nil", response)
	}
}

func TestValidationRuntimeRejectsMismatchThatRepeatsExactRequest(t *testing.T) {
	t.Parallel()

	digest := strings.Repeat("b", 64)
	mismatch := runtimeMismatch(t, digest, 11)
	source := &recordingBasisSource{resolution: mismatch}
	runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
	basis := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":"sha256:%s","graph_revision":11}`,
		digest,
	)
	request := runtimeRequest(t, basis)

	response, err := runtime.Validate(context.Background(), request)
	if !errors.Is(err, ErrProjectBasisUncorrelated) {
		t.Fatalf("Validate() error = %v, want ErrProjectBasisUncorrelated", err)
	}
	if response != nil {
		t.Fatalf("Validate() response = %T, want nil", response)
	}
}

func TestValidationRuntimePreservesExactMismatchWithoutFallback(t *testing.T) {
	t.Parallel()

	mismatch := runtimeMismatch(t, strings.Repeat("c", 64), 13)
	source := &recordingBasisSource{resolution: mismatch}
	runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
	basis := fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":"sha256:%s","graph_revision":17}`,
		strings.Repeat("d", 64),
	)
	request := runtimeRequest(t, basis)

	response, err := runtime.Validate(context.Background(), request)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if response.Verdict() != typedmemory.ValidationUnderdetermined {
		t.Fatalf("response verdict = %q, want underdetermined", response.Verdict())
	}
	if response.Basis().ResolutionKind() != typedmemoryvalidation.BasisResolutionExactMismatch {
		t.Fatalf("resolution kind = %q", response.Basis().ResolutionKind())
	}
	if response.PersistenceDisposition().RowsWritten() != 0 {
		t.Fatal("exact mismatch path reported a write")
	}
}

func TestValidationRuntimePropagatesCancellationBeforeAndDuringResolution(t *testing.T) {
	t.Parallel()

	t.Run("before resolution", func(t *testing.T) {
		t.Parallel()

		source := &recordingBasisSource{
			resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
		}
		runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
		request := runtimeRequest(t, `{"kind":"project_current"}`)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		response, err := runtime.Validate(ctx, request)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Validate() error = %v, want context.Canceled", err)
		}
		if response != nil {
			t.Fatalf("Validate() response = %T, want nil", response)
		}
		if source.calls != 0 {
			t.Fatalf("basis source calls = %d, want 0", source.calls)
		}
	})

	t.Run("during resolution", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		source := &recordingBasisSource{
			resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
			afterCall:  cancel,
		}
		runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)
		request := runtimeRequest(t, `{"kind":"project_current"}`)

		response, err := runtime.Validate(ctx, request)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Validate() error = %v, want context.Canceled", err)
		}
		if response != nil {
			t.Fatalf("Validate() response = %T, want nil", response)
		}
	})
}

func TestValidationRuntimeRejectsInvalidAndBundledRequestsBeforeSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request typedmemorywire.ValidateRequest
		target  error
	}{
		{
			name:    "zero request",
			request: typedmemorywire.ValidateRequest{},
			target:  ErrProjectBasisRequestInvalid,
		},
		{
			name:    "bundled selector",
			request: runtimeRequest(t, `{"kind":"bundled_candidate_open_world"}`),
			target:  ErrProjectBasisUnsupported,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := &recordingBasisSource{
				resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
			}
			runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)

			response, err := runtime.Validate(context.Background(), test.request)
			if !errors.Is(err, test.target) {
				t.Fatalf("Validate() error = %v, want %v", err, test.target)
			}
			if response != nil {
				t.Fatalf("Validate() response = %T, want nil", response)
			}
			if source.calls != 0 {
				t.Fatalf("basis source calls = %d, want 0", source.calls)
			}
		})
	}
}

func TestNewValidationRuntimeRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	projectID := runtimeProjectID(t)
	validSource := &recordingBasisSource{}
	var typedNil *recordingBasisSource
	var nilMap nilableBasisSource
	tests := []struct {
		name      string
		projectID projectledger.ProjectID
		source    ProjectBasisSource
		target    error
	}{
		{
			name:      "missing project identity",
			projectID: projectledger.ProjectID{},
			source:    validSource,
			target:    ErrProjectIdentityMissing,
		},
		{
			name:      "nil source",
			projectID: projectID,
			source:    nil,
			target:    ErrProjectBasisSourceMissing,
		},
		{
			name:      "typed nil source",
			projectID: projectID,
			source:    typedNil,
			target:    ErrProjectBasisSourceMissing,
		},
		{
			name:      "nil map source",
			projectID: projectID,
			source:    nilMap,
			target:    ErrProjectBasisSourceMissing,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewValidationRuntime(test.projectID, test.source)
			if !errors.Is(err, test.target) {
				t.Fatalf("NewValidationRuntime() error = %v, want %v", err, test.target)
			}
		})
	}
}

func TestZeroValidationRuntimeFailsClosed(t *testing.T) {
	t.Parallel()

	request := runtimeRequest(t, `{"kind":"project_current"}`)
	tests := []struct {
		name    string
		runtime ValidationRuntime
		target  error
	}{
		{
			name:    "zero runtime",
			runtime: ValidationRuntime{},
			target:  ErrProjectIdentityMissing,
		},
		{
			name: "missing source",
			runtime: ValidationRuntime{
				projectID: runtimeProjectID(t),
			},
			target: ErrProjectBasisSourceMissing,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response, err := test.runtime.Validate(context.Background(), request)
			if !errors.Is(err, test.target) {
				t.Fatalf("Validate() error = %v, want %v", err, test.target)
			}
			if response != nil {
				t.Fatalf("Validate() response = %T, want nil", response)
			}
		})
	}
}

func TestOneShotBasisResolverCannotReplayOrSubstituteSelector(t *testing.T) {
	t.Parallel()

	resolution := typedmemoryvalidation.NewProjectBasisUnavailable()
	resolver := &oneShotBasisResolver{
		selector:   typedmemorywire.ProjectCurrentSelector{},
		resolution: resolution,
	}
	exactRequest := runtimeRequest(
		t,
		fmt.Sprintf(
			`{"kind":"exact_project","type_env_digest":"sha256:%s","graph_revision":1}`,
			strings.Repeat("e", 64),
		),
	)

	if resolved := resolver.Resolve(exactRequest.Basis()); resolved != nil {
		t.Fatalf("selector substitution resolution = %T, want nil", resolved)
	}
	if resolved := resolver.Resolve(typedmemorywire.ProjectCurrentSelector{}); resolved != nil {
		t.Fatalf("replayed resolution = %T, want nil", resolved)
	}
}

func runtimeValidationRuntime(
	t *testing.T,
	projectID projectledger.ProjectID,
	source ProjectBasisSource,
) ValidationRuntime {
	t.Helper()
	runtime, err := NewValidationRuntime(projectID, source)
	if err != nil {
		t.Fatalf("NewValidationRuntime() error = %v", err)
	}
	return runtime
}

func runtimeProjectID(t *testing.T) projectledger.ProjectID {
	t.Helper()
	projectID, err := projectledger.ParseProjectID("qnt_1234abcd")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	return projectID
}

func runtimeMismatch(
	t *testing.T,
	digestHex string,
	revision uint64,
) *typedmemoryvalidation.ExactProjectBasisMismatch {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + digestHex)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	typeEnvRef, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	graphRevision := typedmemory.NewGraphRevision(revision)
	mismatch, err := typedmemoryvalidation.NewExactProjectBasisMismatch(
		typeEnvRef,
		graphRevision,
	)
	if err != nil {
		t.Fatalf("NewExactProjectBasisMismatch() error = %v", err)
	}
	return mismatch
}

func runtimeRequest(
	t *testing.T,
	basis string,
) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": %s,
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:runtime-fixture",
      "local_ref": "local:runtime-fixture",
      "context": "context:runtime-fixture",
      "label": "Runtime fixture",
      "provenance": "provenance:runtime-fixture"
    }]
  }
}`, typedmemorywire.ContractVersion, basis)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}
