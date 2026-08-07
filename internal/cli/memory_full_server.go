package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const projectMemoryFullDeliveryContract = "haft.memory.delivery.v1"

// projectMemoryFullSurface is the sealed capability required before the MCP
// server may advertise haft_memory(validate|admit) and the three
// haft_query(action="memory") read modes.
type projectMemoryFullSurface interface {
	EnsureReady(context.Context) error
	FullMCPHandler() fpf.MemoryToolHandler
	ReadOnlyQueryMCPHandler() fpf.MemoryToolHandler
}

type serveProjectMemoryFullSurface interface {
	projectMemoryFullSurface
	Close() error
}

type serveProjectMemoryFullSurfaceOpener func(
	context.Context,
	ProjectBinding,
) (serveProjectMemoryFullSurface, error)

var openServeProjectMemoryFullSurface serveProjectMemoryFullSurfaceOpener = func(
	ctx context.Context,
	binding ProjectBinding,
) (serveProjectMemoryFullSurface, error) {
	return openSealedProjectMemoryFullSurface(ctx, binding)
}

type projectMemoryFullExecutor interface {
	EnsureReady(context.Context) error
	ProjectBasisAvailable(context.Context) (bool, error)
	ValidateBundled(
		context.Context,
		typedmemorywire.ValidateRequest,
	) ([]byte, error)
	ExecuteProjectRead(
		context.Context,
		typedmemorywire.Request,
	) ([]byte, error)
	Admit(
		context.Context,
		typedmemorywire.AdmitRequest,
	) ([]byte, error)
}

// sealedProjectMemoryFullSurface owns one already-open, anchored project
// ledger session. Opening and composing this surface never creates, migrates,
// repairs, or selects project state. The caller remains responsible for
// closing it.
type sealedProjectMemoryFullSurface struct {
	projectID       projectidentity.ProjectID
	revalidate      projectMemoryIdentityRevalidator
	executor        projectMemoryFullExecutor
	readyAtStartup  bool
	startupObserved bool
	close           func() error
	mu              sync.Mutex
}

type composedProjectMemoryFullExecutor struct {
	bundled readOnlyMemoryValidation
	project projectMemoryRuntime
}

type projectMemoryFullAdmissionDelivery struct {
	ContractVersion              string                                  `json:"contract_version"`
	Action                       string                                  `json:"action"`
	Result                       string                                  `json:"result"`
	AdmissionResult              json.RawMessage                         `json:"admission_result"`
	AdmissionResultDigest        string                                  `json:"admission_result_canonical_digest"`
	AdmissionOperation           projectMemoryFullAdmissionOperation     `json:"admission_operation"`
	PostEffectLedgerRevalidation projectMemoryFullLedgerRevalidation     `json:"post_effect_ledger_revalidation"`
	Interpretation               projectMemoryFullDeliveryInterpretation `json:"interpretation"`
}

type projectMemoryFullAdmissionOperation struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type projectMemoryFullLedgerRevalidation struct {
	Kind   string `json:"kind"`
	Code   string `json:"code,omitempty"`
	Detail string `json:"detail,omitempty"`
	Repair string `json:"repair,omitempty"`
}

type projectMemoryFullDeliveryInterpretation struct {
	Establishes      []string `json:"establishes"`
	DoesNotEstablish []string `json:"does_not_establish"`
	DoesNotAuthorize []string `json:"does_not_authorize"`
}

// openSealedProjectMemoryFullSurface composes the complete split
// surface from the ProjectBinding already resolved by the serve shell. The
// opener does not rediscover a root through cwd or environment variables. The
// anchored ledger verifies the exact project identity and topology; runtime
// composition verifies the current schema without create, migrate, repair, or head
// selection.
func openSealedProjectMemoryFullSurface(
	ctx context.Context,
	binding ProjectBinding,
) (*sealedProjectMemoryFullSurface, error) {
	bound, err := openBoundProjectMemoryRuntimeFromBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	bundled, err := newReadOnlyMemoryValidation(ctx)
	if err != nil {
		closeErr := bound.Close()
		return nil, errors.Join(err, closeErr)
	}
	executor := composedProjectMemoryFullExecutor{
		bundled: bundled,
		project: bound.runtime,
	}
	readyAtStartup, err := executor.ProjectBasisAvailable(ctx)
	if err != nil {
		closeErr := bound.Close()
		return nil, errors.Join(
			fmt.Errorf(
				"probe startup project-memory availability: %w",
				err,
			),
			closeErr,
		)
	}
	revalidate := func(revalidationContext context.Context) error {
		if err := bound.ledger.Revalidate(revalidationContext); err != nil {
			return err
		}
		database := bound.ledger.Database()
		if err := db.RequireCurrentSchemaReadOnly(
			revalidationContext,
			database,
		); err != nil {
			return err
		}
		if _, err := projecttypeenvstage.OpenReadOnly(
			revalidationContext,
			database,
		); err != nil {
			return err
		}
		if _, err := projecttypeenvheadstore.OpenReadOnly(
			revalidationContext,
			database,
		); err != nil {
			return err
		}
		return nil
	}
	return &sealedProjectMemoryFullSurface{
		projectID:       bound.ledger.ProjectID(),
		revalidate:      revalidate,
		executor:        executor,
		readyAtStartup:  readyAtStartup,
		startupObserved: true,
		close:           bound.Close,
	}, nil
}

func (executor composedProjectMemoryFullExecutor) EnsureReady(
	ctx context.Context,
) error {
	return executor.project.EnsureReady(ctx)
}

func (executor composedProjectMemoryFullExecutor) ProjectBasisAvailable(
	ctx context.Context,
) (bool, error) {
	return executor.project.ProjectBasisAvailable(ctx)
}

func (executor composedProjectMemoryFullExecutor) ValidateBundled(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch request.Basis().(type) {
	case typedmemorywire.BundledCandidateOpenWorldSelector:
		return executor.bundled.ValidateDecoded(request)
	default:
		return nil, fmt.Errorf(
			"bundled validation requires the bundled_candidate_open_world basis",
		)
	}
}

func (executor composedProjectMemoryFullExecutor) ExecuteProjectRead(
	ctx context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	return executor.project.ExecuteReadOnlyRequest(ctx, request)
}

func (executor composedProjectMemoryFullExecutor) Admit(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	return executor.project.Admit(ctx, request)
}

func (surface *sealedProjectMemoryFullSurface) EnsureReady(
	ctx context.Context,
) error {
	if surface == nil ||
		surface.projectID.String() == "" ||
		surface.revalidate == nil ||
		surface.executor == nil {
		return fmt.Errorf(
			"prepare full project-memory surface: dependencies are incomplete",
		)
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()

	if err := surface.revalidate(ctx); err != nil {
		return fmt.Errorf(
			"revalidate checked Haft project ledger before full-surface readiness: %w",
			err,
		)
	}
	readyErr := surface.executor.EnsureReady(ctx)
	postErr := surface.revalidate(ctx)
	if postErr != nil {
		postErr = fmt.Errorf(
			"revalidate checked Haft project ledger after full-surface readiness: %w",
			postErr,
		)
	}
	return errors.Join(readyErr, postErr)
}

func (surface *sealedProjectMemoryFullSurface) FullMCPHandler() fpf.MemoryToolHandler {
	if surface == nil ||
		surface.projectID.String() == "" ||
		surface.revalidate == nil ||
		surface.executor == nil {
		return nil
	}
	return func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		return surface.executeMemory(ctx, arguments)
	}
}

func (surface *sealedProjectMemoryFullSurface) ReadOnlyQueryMCPHandler() fpf.MemoryToolHandler {
	if surface == nil ||
		surface.projectID.String() == "" ||
		surface.revalidate == nil ||
		surface.executor == nil {
		return nil
	}
	return func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		return surface.executeQuery(ctx, arguments)
	}
}

func (surface *sealedProjectMemoryFullSurface) executeMemory(
	ctx context.Context,
	arguments json.RawMessage,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	request, err := typedmemorywire.DecodeRequest(arguments)
	if err != nil {
		return "", err
	}
	switch exact := request.(type) {
	case typedmemorywire.ValidateRequest:
		return surface.executeCheckedValidation(ctx, exact)
	case typedmemorywire.AdmitRequest:
		return surface.executeCheckedAdmission(ctx, exact)
	default:
		return "", fmt.Errorf(
			"haft_memory action %q is unavailable on the validate/admit boundary",
			request.Action(),
		)
	}
}

func (surface *sealedProjectMemoryFullSurface) executeQuery(
	ctx context.Context,
	arguments json.RawMessage,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	request, err := typedmemorywire.DecodeQueryReadRequest(arguments)
	if err != nil {
		return "", err
	}
	return surface.executeCheckedQueryRead(ctx, request)
}

func (surface *sealedProjectMemoryFullSurface) executeCheckedValidation(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) (string, error) {
	surface.mu.Lock()
	defer surface.mu.Unlock()

	if err := surface.revalidate(ctx); err != nil {
		return "", fmt.Errorf(
			"revalidate checked Haft project ledger before haft_memory(%s): %w",
			typedmemorywire.ActionValidate,
			err,
		)
	}
	if _, bundled := request.Basis().(typedmemorywire.BundledCandidateOpenWorldSelector); !bundled {
		recovery, restart, restartErr := surface.restartRecovery(
			ctx,
			request.ContractVersion(),
			request.Action(),
			"",
		)
		if restartErr != nil || restart {
			return string(recovery), restartErr
		}
	}
	result, operationErr := surface.executeDecodedValidation(ctx, request)
	postErr := surface.revalidate(ctx)
	if postErr != nil {
		return "", errors.Join(
			operationErr,
			fmt.Errorf(
				"revalidate checked Haft project ledger after haft_memory(%s): discard untrusted read result: %w",
				typedmemorywire.ActionValidate,
				postErr,
			),
		)
	}
	if operationErr != nil {
		return "", operationErr
	}
	return string(result), nil
}

func (surface *sealedProjectMemoryFullSurface) executeCheckedQueryRead(
	ctx context.Context,
	request typedmemorywire.Request,
) (string, error) {
	surface.mu.Lock()
	defer surface.mu.Unlock()

	if err := surface.revalidate(ctx); err != nil {
		return "", fmt.Errorf(
			"revalidate checked Haft project ledger before haft_query(memory/%s): %w",
			request.Action(),
			err,
		)
	}
	recovery, restart, restartErr := surface.restartRecovery(
		ctx,
		request.ContractVersion(),
		typedmemorywire.QueryActionMemory,
		request.Action(),
	)
	if restartErr != nil || restart {
		return string(recovery), restartErr
	}
	result, operationErr := surface.executeDecodedQueryRead(ctx, request)
	postErr := surface.revalidate(ctx)
	if postErr != nil {
		return "", errors.Join(
			operationErr,
			fmt.Errorf(
				"revalidate checked Haft project ledger after haft_query(memory/%s): discard untrusted read result: %w",
				request.Action(),
				postErr,
			),
		)
	}
	if operationErr != nil {
		return "", operationErr
	}
	return string(result), nil
}

func (surface *sealedProjectMemoryFullSurface) executeCheckedAdmission(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) (string, error) {
	surface.mu.Lock()
	defer surface.mu.Unlock()

	if err := surface.revalidate(ctx); err != nil {
		return "", fmt.Errorf(
			"revalidate checked Haft project ledger before haft_memory(%s): %w",
			typedmemorywire.ActionAdmit,
			err,
		)
	}
	recovery, restart, restartErr := surface.restartRecovery(
		ctx,
		request.ContractVersion(),
		request.Action(),
		"",
	)
	if restartErr != nil || restart {
		return string(recovery), restartErr
	}
	result, operationErr := surface.executeDecodedAdmission(ctx, request)
	postErr := surface.revalidate(ctx)
	return deliverProjectMemoryFullAdmission(
		result,
		operationErr,
		postErr,
	)
}

func (surface *sealedProjectMemoryFullSurface) restartRecovery(
	ctx context.Context,
	contractVersion string,
	action string,
	mode string,
) ([]byte, bool, error) {
	readyNow, err := surface.executor.ProjectBasisAvailable(ctx)
	if err != nil {
		return nil, false, err
	}
	if !surface.startupObserved ||
		surface.readyAtStartup ||
		!readyNow {
		return nil, false, nil
	}
	response, err := projectMemoryRestartRequiredResponse(
		contractVersion,
		action,
		mode,
	)
	return response, true, err
}

func (surface *sealedProjectMemoryFullSurface) executeDecodedValidation(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) ([]byte, error) {
	switch request.Basis().(type) {
	case typedmemorywire.BundledCandidateOpenWorldSelector:
		return surface.executor.ValidateBundled(ctx, request)
	default:
		return surface.executor.ExecuteProjectRead(ctx, request)
	}
}

func (surface *sealedProjectMemoryFullSurface) executeDecodedQueryRead(
	ctx context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	switch exact := request.(type) {
	case typedmemorywire.ResolveReadRequest:
		return surface.executor.ExecuteProjectRead(ctx, exact)
	case typedmemorywire.NeighborhoodReadRequest:
		return surface.executor.ExecuteProjectRead(ctx, exact)
	case typedmemorywire.RecallReadRequest:
		return surface.executor.ExecuteProjectRead(ctx, exact)
	default:
		return nil, fmt.Errorf(
			"decoded haft_query memory request variant %T is unsupported",
			request,
		)
	}
}

func (surface *sealedProjectMemoryFullSurface) executeDecodedAdmission(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) ([]byte, error) {
	if request.AuthorityClass() !=
		typedmemorywire.AuthorityClassNonBindingSemanticAssertion {
		return nil, fmt.Errorf(
			"haft_memory(admit) requires non-binding semantic assertion authority",
		)
	}
	return surface.executor.Admit(ctx, request)
}

func deliverProjectMemoryFullAdmission(
	result []byte,
	operationErr error,
	postErr error,
) (string, error) {
	trimmed := bytes.TrimSpace(result)
	if len(trimmed) == 0 {
		return "", errors.Join(operationErr, postErr)
	}
	if !json.Valid(trimmed) {
		return "", errors.Join(
			operationErr,
			postErr,
			fmt.Errorf(
				"project-memory admission returned invalid JSON; exact result cannot be delivered safely",
			),
		)
	}
	resultDigest, err := canonicalProjectMemoryFullJSONDigest(trimmed)
	if err != nil {
		return "", errors.Join(operationErr, postErr, err)
	}
	if operationErr == nil && postErr == nil {
		return string(result), nil
	}
	response := projectMemoryFullAdmissionDelivery{
		ContractVersion:       projectMemoryFullDeliveryContract,
		Action:                typedmemorywire.ActionAdmit,
		Result:                "admission_result_with_delivery_qualification",
		AdmissionResult:       append(json.RawMessage(nil), trimmed...),
		AdmissionResultDigest: resultDigest,
		AdmissionOperation: projectMemoryFullAdmissionOperation{
			Kind: "result_returned",
		},
		PostEffectLedgerRevalidation: projectMemoryFullLedgerRevalidation{
			Kind: "verified_after_effect",
		},
		Interpretation: projectMemoryFullDeliveryInterpretation{
			Establishes: []string{
				"the admission runtime returned the semantically identical nested JSON result shown here",
				"the canonical digest identifies that nested semantic JSON independently of whitespace and object-field order",
			},
			DoesNotEstablish: []string{
				"truth, evidence quality, applicability outside the exact ContextSlice, completion, or recommendation",
			},
			DoesNotAuthorize: []string{
				"decision, commission, specification lifecycle, schema, ProjectTypeEnvHead, or binding-authority mutation",
			},
		},
	}
	if operationErr != nil {
		response.AdmissionOperation = projectMemoryFullAdmissionOperation{
			Kind:   "result_returned_with_operational_error",
			Detail: operationErr.Error(),
		}
		response.Interpretation.DoesNotEstablish = append(
			response.Interpretation.DoesNotEstablish,
			"that the admission operation completed without an operational error",
		)
	}
	if postErr != nil {
		response.PostEffectLedgerRevalidation =
			projectMemoryFullLedgerRevalidation{
				Kind:   "failed_after_effect",
				Code:   "project_ledger_revalidation_failed",
				Detail: postErr.Error(),
				Repair: "restore the canonical project root-to-ledger attachment, then replay the unchanged exact admission request and idempotency key",
			}
		response.Interpretation.DoesNotEstablish = append(
			response.Interpretation.DoesNotEstablish,
			"the current project root-to-ledger attachment after the admission effect",
			"that no semantic rows were committed",
		)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return "", errors.Join(
			operationErr,
			postErr,
			fmt.Errorf(
				"encode qualified project-memory admission result: %w",
				err,
			),
		)
	}
	return string(append(encoded, '\n')), nil
}

func canonicalProjectMemoryFullJSONDigest(
	raw []byte,
) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value := interface{}(nil)
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf(
			"decode project-memory admission result for canonical identity: %w",
			err,
		)
	}
	trailing := interface{}(nil)
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", fmt.Errorf(
			"decode project-memory admission result for canonical identity: trailing JSON",
		)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf(
			"encode project-memory admission result canonical identity: %w",
			err,
		)
	}
	digest := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", digest), nil
}

func (surface *sealedProjectMemoryFullSurface) Close() error {
	if surface == nil {
		return nil
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()

	if surface.close == nil {
		return nil
	}
	close := surface.close
	surface.close = nil
	return close()
}

// installProjectMemoryFullSurface keeps readiness and both handler
// constructions behind one interface so the server cannot advertise either
// half of an incomplete split surface.
func installProjectMemoryFullSurface(
	ctx context.Context,
	server *fpf.Server,
	surface projectMemoryFullSurface,
) error {
	if ctx == nil {
		return fmt.Errorf(
			"install full project-memory surface: context is required",
		)
	}
	if server == nil {
		return fmt.Errorf(
			"install full project-memory surface: server is required",
		)
	}
	if surface == nil {
		return fmt.Errorf(
			"install full project-memory surface: runtime is required",
		)
	}
	if err := surface.EnsureReady(ctx); err != nil {
		return fmt.Errorf(
			"install full project-memory surface: %w",
			err,
		)
	}
	memoryHandler := surface.FullMCPHandler()
	if memoryHandler == nil {
		return fmt.Errorf(
			"install full project-memory surface: validate/admit handler is required",
		)
	}
	readHandler := surface.ReadOnlyQueryMCPHandler()
	if readHandler == nil {
		return fmt.Errorf(
			"install full project-memory surface: query-read handler is required",
		)
	}
	server.SetMemoryFullHandler(memoryHandler)
	server.SetMemoryReadHandler(readHandler)
	return nil
}

func configureServeProjectMemoryFullSurface(
	ctx context.Context,
	server *fpf.Server,
	binding ProjectBinding,
) (serveProjectMemoryFullSurface, error) {
	surface, err := openServeProjectMemoryFullSurface(ctx, binding)
	if err != nil {
		return nil, err
	}
	if err := installProjectMemoryFullSurface(ctx, server, surface); err != nil {
		closeErr := surface.Close()
		return nil, errors.Join(err, closeErr)
	}
	return surface, nil
}
