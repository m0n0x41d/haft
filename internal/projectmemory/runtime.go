package projectmemory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

var (
	ErrProjectIdentityMissing         = errors.New("project-memory project identity is missing")
	ErrProjectBasisSourceMissing      = errors.New("project-memory basis source is missing")
	ErrProjectBasisRequestInvalid     = errors.New("project-memory validation request is invalid")
	ErrProjectBasisUnsupported        = errors.New("project-memory basis selector is unsupported")
	ErrProjectBasisResolutionEmpty    = errors.New("project-memory basis resolution is missing")
	ErrProjectBasisUncorrelated       = errors.New("project-memory basis resolution is not correlated with the request")
	ErrProjectAdmissionStoreMissing   = errors.New("project-memory admission store is missing")
	ErrProjectAdmissionNotValid       = errors.New("project-memory admission requires a valid validation outcome")
	ErrProjectAdmissionRequestInvalid = errors.New(
		"project-memory admission request is invalid",
	)
	ErrProjectAdmissionReplayStoreMissing = errors.New(
		"project-memory admission replay store is missing",
	)
	ErrProjectAdmissionIdempotencyReplayStoreMissing = errors.New(
		"project-memory idempotency replay store is missing",
	)
)

// ProjectBasisSource resolves project state under the caller's context and a
// trusted project-ledger identity. It is read-only: admission and activation
// belong to later, separately composed runtimes.
type ProjectBasisSource interface {
	ResolveProjectBasis(
		context.Context,
		projectledger.ProjectID,
		typedmemorywire.BasisSelector,
	) (typedmemoryvalidation.BasisResolution, error)
}

// ValidationRuntime is the sealed project-bound orchestration for read-only
// typed-memory validation. It deliberately has no commit or activation port.
type ValidationRuntime struct {
	projectID projectledger.ProjectID
	source    ProjectBasisSource
}

func NewValidationRuntime(
	projectID projectledger.ProjectID,
	source ProjectBasisSource,
) (ValidationRuntime, error) {
	if projectID.String() == "" {
		return ValidationRuntime{}, ErrProjectIdentityMissing
	}
	if !projectBasisSourcePresent(source) {
		return ValidationRuntime{}, ErrProjectBasisSourceMissing
	}
	return ValidationRuntime{
		projectID: projectID,
		source:    source,
	}, nil
}

func (runtime ValidationRuntime) Validate(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) (typedmemoryvalidation.Response, error) {
	outcome, err := runtime.Evaluate(ctx, request)
	if err != nil {
		return nil, err
	}
	response := typedmemoryvalidation.PresentOutcome(outcome)
	if response == nil {
		return nil, ErrProjectBasisResolutionEmpty
	}
	return response, nil
}

func (runtime ValidationRuntime) Evaluate(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) (typedmemoryvalidation.Outcome, error) {
	if !typedmemorywire.IsDecodedValidateRequest(request) {
		return nil, ErrProjectBasisRequestInvalid
	}
	selector := request.Basis()
	service, err := runtime.resolveValidationService(ctx, selector)
	if err != nil {
		return nil, err
	}
	outcome := service.Evaluate(request)
	if outcome == nil {
		return nil, ErrProjectBasisResolutionEmpty
	}
	return outcome, nil
}

// EvaluateCandidate is the trusted internal bridge for a pure adapter that
// already produced a typed MemoryChangeSet. It resolves the same project basis
// and invokes the same semantic validator as the strict wire path without
// serializing the candidate through JSON.
func (runtime ValidationRuntime) EvaluateCandidate(
	ctx context.Context,
	selector typedmemorywire.BasisSelector,
	candidate typedmemory.MemoryChangeSet,
) (typedmemoryvalidation.Outcome, error) {
	if _, err := candidate.Digest(); err != nil {
		return nil, ErrProjectBasisRequestInvalid
	}
	service, err := runtime.resolveValidationService(ctx, selector)
	if err != nil {
		return nil, err
	}
	outcome := service.EvaluateCandidate(selector, candidate)
	if outcome == nil {
		return nil, ErrProjectBasisResolutionEmpty
	}
	return outcome, nil
}

func (runtime ValidationRuntime) resolveValidationService(
	ctx context.Context,
	selector typedmemorywire.BasisSelector,
) (typedmemoryvalidation.Service, error) {
	if runtime.projectID.String() == "" {
		return typedmemoryvalidation.Service{}, ErrProjectIdentityMissing
	}
	if !projectBasisSourcePresent(runtime.source) {
		return typedmemoryvalidation.Service{}, ErrProjectBasisSourceMissing
	}
	if err := validationContextError(ctx); err != nil {
		return typedmemoryvalidation.Service{}, err
	}
	if err := requireProjectSelector(selector); err != nil {
		return typedmemoryvalidation.Service{}, err
	}
	resolution, err := runtime.source.ResolveProjectBasis(
		ctx,
		runtime.projectID,
		selector,
	)
	if err != nil {
		return typedmemoryvalidation.Service{}, fmt.Errorf(
			"resolve project-memory validation basis: %w",
			err,
		)
	}
	if err := validationContextError(ctx); err != nil {
		return typedmemoryvalidation.Service{}, err
	}
	if err := requireCorrelatedResolution(selector, resolution); err != nil {
		return typedmemoryvalidation.Service{}, err
	}
	resolver := &oneShotBasisResolver{
		selector:   selector,
		resolution: resolution,
	}
	service, err := typedmemoryvalidation.NewService(resolver)
	if err != nil {
		return typedmemoryvalidation.Service{}, fmt.Errorf(
			"construct project-memory validation service: %w",
			err,
		)
	}
	return service, nil
}

// AdmissionRuntime composes project-bound read-only validation with the
// separately supplied typed-memory commit port. It does not select a project
// TypeEnv, expose a public admit command, or infer human authority; callers must
// supply trusted idempotency/provenance coordinates explicitly.
type AdmissionRuntime struct {
	validation ValidationRuntime
	store      typedmemorystore.CommitPort
}

// ProjectID exposes only the immutable trusted project coordinate captured by
// the runtime constructor. Effect composers use it to reject cross-project
// pairing before any semantic commit; it grants no validation or write
// capability.
func (runtime AdmissionRuntime) ProjectID() projectledger.ProjectID {
	return runtime.validation.projectID
}

type AdmissionResultKind string

const (
	AdmissionNotAdmittedResult          AdmissionResultKind = "not_admitted"
	AdmissionCommittedResult            AdmissionResultKind = "committed"
	AdmissionCommitOutcomeUnknownResult AdmissionResultKind = "commit_outcome_unknown"
)

// AdmissionResult is the closed semantic/effect result. A non-valid semantic
// verdict remains a successful no-write result rather than being collapsed
// into an operational error.
type AdmissionResult interface {
	Kind() AdmissionResultKind
	ContractVersion() string
	admissionResultVariant()
}

type AdmissionNotAdmitted struct {
	response typedmemoryvalidation.Response
}

func (AdmissionNotAdmitted) Kind() AdmissionResultKind {
	return AdmissionNotAdmittedResult
}

func (AdmissionNotAdmitted) admissionResultVariant() {}

func (result AdmissionNotAdmitted) ContractVersion() string {
	if result.response == nil {
		return ""
	}
	return result.response.ContractVersion()
}

func (result AdmissionNotAdmitted) ValidationResponse() typedmemoryvalidation.Response {
	return result.response
}

type AdmissionCommitted struct {
	contractVersion string
	receipt         typedmemorystore.CommitReceipt
}

func (AdmissionCommitted) Kind() AdmissionResultKind {
	return AdmissionCommittedResult
}

func (AdmissionCommitted) admissionResultVariant() {}

func (result AdmissionCommitted) ContractVersion() string {
	return result.contractVersion
}

func (result AdmissionCommitted) Receipt() typedmemorystore.CommitReceipt {
	return result.receipt
}

// AdmissionRetryCoordinates identify one exact admission request independently
// of JSON presentation. They authorize only a same-project, same-key replay of
// the unchanged request so the durable store can resolve a prior unknown
// commit outcome without duplicating the semantic effect.
type AdmissionRetryCoordinates struct {
	contractVersion       typedmemorystore.AdmissionContractVersion
	projectID             projectledger.ProjectID
	idempotencyKey        typedmemorystore.IdempotencyKey
	basisKind             typedmemorywire.BasisKind
	typeEnv               typedmemory.TypeEnvRef
	graphRevision         typedmemory.GraphRevision
	requestProvenance     typedmemory.ProvenanceRef
	candidateDigest       typedmemory.SHA256Digest
	requestIdentityDigest typedmemory.SHA256Digest
}

func (coordinates AdmissionRetryCoordinates) ContractVersion() typedmemorystore.AdmissionContractVersion {
	return coordinates.contractVersion
}

func (coordinates AdmissionRetryCoordinates) ProjectID() projectledger.ProjectID {
	return coordinates.projectID
}

func (coordinates AdmissionRetryCoordinates) IdempotencyKey() typedmemorystore.IdempotencyKey {
	return coordinates.idempotencyKey
}

func (coordinates AdmissionRetryCoordinates) BasisKind() typedmemorywire.BasisKind {
	return coordinates.basisKind
}

func (coordinates AdmissionRetryCoordinates) TypeEnv() typedmemory.TypeEnvRef {
	return coordinates.typeEnv
}

func (coordinates AdmissionRetryCoordinates) GraphRevision() typedmemory.GraphRevision {
	return coordinates.graphRevision
}

func (coordinates AdmissionRetryCoordinates) RequestProvenance() typedmemory.ProvenanceRef {
	return coordinates.requestProvenance
}

func (coordinates AdmissionRetryCoordinates) CandidateDigest() typedmemory.SHA256Digest {
	return coordinates.candidateDigest
}

func (coordinates AdmissionRetryCoordinates) RequestIdentityDigest() typedmemory.SHA256Digest {
	return coordinates.requestIdentityDigest
}

// AdmissionCommitOutcomeUnknown is a closed effect result rather than an
// operational error. The store may have committed the exact transaction, so
// neither "committed" nor "rolled back" is established until the unchanged
// request is replayed with these coordinates.
type AdmissionCommitOutcomeUnknown struct {
	retry           AdmissionRetryCoordinates
	possibleReceipt typedmemorystore.CommitReceipt
	receiptPresent  bool
	detail          string
}

func (AdmissionCommitOutcomeUnknown) Kind() AdmissionResultKind {
	return AdmissionCommitOutcomeUnknownResult
}

func (AdmissionCommitOutcomeUnknown) admissionResultVariant() {}

func (result AdmissionCommitOutcomeUnknown) ContractVersion() string {
	return result.retry.ContractVersion().String()
}

func (result AdmissionCommitOutcomeUnknown) RetryCoordinates() AdmissionRetryCoordinates {
	return result.retry
}

func (result AdmissionCommitOutcomeUnknown) PossibleReceipt() (
	typedmemorystore.CommitReceipt,
	bool,
) {
	return result.possibleReceipt, result.receiptPresent
}

func (result AdmissionCommitOutcomeUnknown) OperationalDetail() string {
	return result.detail
}

type admissionRequestIdentityV1 struct {
	ContractVersion   string `json:"contract_version"`
	Action            string `json:"action"`
	ProjectID         string `json:"project_id"`
	AuthorityClass    string `json:"authority_class"`
	IdempotencyKey    string `json:"idempotency_key"`
	BasisKind         string `json:"basis_kind"`
	TypeEnvDigest     string `json:"type_env_digest"`
	GraphRevision     uint64 `json:"graph_revision"`
	RequestProvenance string `json:"request_provenance_ref"`
	CandidateDigest   string `json:"candidate_digest"`
}

// NewAdmissionCommitOutcomeUnknown constructs the only retryable unknown
// admission result. It is exported so effect shells and contract tests can
// preserve the same core result without inventing protocol-local recovery
// coordinates.
func NewAdmissionCommitOutcomeUnknown(
	projectID projectledger.ProjectID,
	request typedmemorywire.AdmitRequest,
	possibleReceipt typedmemorystore.CommitReceipt,
	cause error,
) (AdmissionCommitOutcomeUnknown, error) {
	if !errors.Is(cause, typedmemorystore.ErrCommitOutcomeUnknown) {
		return AdmissionCommitOutcomeUnknown{}, fmt.Errorf(
			"construct unknown project-memory admission outcome: %w",
			typedmemorystore.ErrCommitOutcomeUnknown,
		)
	}
	coordinates, err := newAdmissionRetryCoordinates(projectID, request)
	if err != nil {
		return AdmissionCommitOutcomeUnknown{}, err
	}
	return AdmissionCommitOutcomeUnknown{
		retry:           coordinates,
		possibleReceipt: possibleReceipt,
		receiptPresent: possibleReceipt !=
			(typedmemorystore.CommitReceipt{}),
		detail: cause.Error(),
	}, nil
}

func newAdmissionRetryCoordinates(
	projectID projectledger.ProjectID,
	request typedmemorywire.AdmitRequest,
) (AdmissionRetryCoordinates, error) {
	if projectID.String() == "" ||
		!typedmemorywire.IsDecodedAdmitRequest(request) {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"construct project-memory admission retry coordinates: exact project and decoded request are required",
		)
	}
	key, err := typedmemorystore.NewIdempotencyKey(request.IdempotencyKey())
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"construct project-memory admission retry key: %w",
			err,
		)
	}
	contractVersion, err := typedmemorystore.ParseAdmissionContractVersion(
		request.ContractVersion(),
	)
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"construct project-memory admission retry contract version: %w",
			err,
		)
	}
	basis := request.Basis()
	typeEnv, err := typedmemory.NewTypeEnvRef(
		basis.RequestedTypeEnvDigest(),
	)
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"construct project-memory admission retry TypeEnv: %w",
			err,
		)
	}
	candidate, err := request.ValidationRequest().BindChangeSet(typeEnv)
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"construct project-memory admission retry candidate: %w",
			err,
		)
	}
	candidateDigest, err := candidate.Digest()
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"digest project-memory admission retry candidate: %w",
			err,
		)
	}
	identity := admissionRequestIdentityV1{
		ContractVersion:   request.ContractVersion(),
		Action:            request.Action(),
		ProjectID:         projectID.String(),
		AuthorityClass:    request.AuthorityClass(),
		IdempotencyKey:    key.String(),
		BasisKind:         string(basis.Kind()),
		TypeEnvDigest:     typeEnv.String(),
		GraphRevision:     basis.RequestedGraphRevision().Value(),
		RequestProvenance: request.RequestProvenance().String(),
		CandidateDigest:   candidateDigest.String(),
	}
	canonicalIdentity, err := json.Marshal(identity)
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"encode project-memory admission retry identity: %w",
			err,
		)
	}
	identitySum := sha256.Sum256(canonicalIdentity)
	identityDigest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(identitySum[:]),
	)
	if err != nil {
		return AdmissionRetryCoordinates{}, fmt.Errorf(
			"digest project-memory admission retry identity: %w",
			err,
		)
	}
	return AdmissionRetryCoordinates{
		contractVersion:       contractVersion,
		projectID:             projectID,
		idempotencyKey:        key,
		basisKind:             basis.Kind(),
		typeEnv:               typeEnv,
		graphRevision:         basis.RequestedGraphRevision(),
		requestProvenance:     request.RequestProvenance(),
		candidateDigest:       candidateDigest,
		requestIdentityDigest: identityDigest,
	}, nil
}

func NewAdmissionRuntime(
	projectID projectledger.ProjectID,
	source ProjectBasisSource,
	store typedmemorystore.CommitPort,
) (AdmissionRuntime, error) {
	validation, err := NewValidationRuntime(projectID, source)
	if err != nil {
		return AdmissionRuntime{}, err
	}
	if !commitPortPresent(store) {
		return AdmissionRuntime{}, ErrProjectAdmissionStoreMissing
	}
	return AdmissionRuntime{
		validation: validation,
		store:      store,
	}, nil
}

// Admit evaluates the exact-project request and commits only a sealed Valid
// result. Invalid and Underdetermined outcomes are returned explicitly with
// zero writes. The store independently revalidates the exact basis inside its
// transaction before committing.
func (runtime AdmissionRuntime) Admit(
	ctx context.Context,
	request typedmemorywire.AdmitRequest,
) (AdmissionResult, error) {
	if !typedmemorywire.IsDecodedAdmitRequest(request) ||
		request.AuthorityClass() !=
			typedmemorywire.AuthorityClassNonBindingSemanticAssertion {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	key, err := typedmemorystore.NewIdempotencyKey(request.IdempotencyKey())
	if err != nil {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(
		request.Basis().RequestedTypeEnvDigest(),
	)
	if err != nil {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	candidate, err := request.ValidationRequest().BindChangeSet(typeEnv)
	if err != nil {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	contractVersion, err := typedmemorystore.ParseAdmissionContractVersion(
		request.ContractVersion(),
	)
	if err != nil {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	replayRequest, err := typedmemorystore.NewReplayRequestBuilder().
		SetContractVersion(contractVersion).
		SetProject(runtime.validation.projectID).
		SetExpectedRevision(request.Basis().RequestedGraphRevision()).
		SetExpectedTypeEnv(typeEnv).
		SetIdempotencyKey(key).
		SetRequestProvenance(request.RequestProvenance()).
		SetCandidate(candidate).
		Build()
	if err != nil {
		return nil, ErrProjectAdmissionRequestInvalid
	}
	replayStore, present := runtime.store.(typedmemorystore.ReplayPort)
	if !present || !interfaceValuePresent(replayStore) {
		return nil, ErrProjectAdmissionReplayStoreMissing
	}
	replay, found, err := replayStore.ReplayMemoryChangeSet(
		ctx,
		replayRequest,
	)
	if err != nil {
		if errors.Is(err, typedmemorystore.ErrCommitOutcomeUnknown) {
			unknown, unknownErr := NewAdmissionCommitOutcomeUnknown(
				runtime.validation.projectID,
				request,
				replay,
				err,
			)
			if unknownErr != nil {
				return nil, errors.Join(err, unknownErr)
			}
			return unknown, nil
		}
		return nil, err
	}
	if found {
		return AdmissionCommitted{
			contractVersion: request.ContractVersion(),
			receipt:         replay,
		}, nil
	}
	outcome, err := runtime.validation.Evaluate(
		ctx,
		request.ValidationRequest(),
	)
	if err != nil {
		return nil, err
	}
	valid, accepted := outcome.(typedmemoryvalidation.ValidOutcome)
	if !accepted {
		response := typedmemoryvalidation.PresentOutcome(outcome)
		if response == nil {
			return nil, ErrProjectBasisResolutionEmpty
		}
		return AdmissionNotAdmitted{response: response}, nil
	}
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		request.RequestProvenance(),
	)
	if err != nil {
		if errors.Is(err, typedmemorystore.ErrCommitOutcomeUnknown) {
			unknown, unknownErr := NewAdmissionCommitOutcomeUnknown(
				runtime.validation.projectID,
				request,
				receipt,
				err,
			)
			if unknownErr != nil {
				return nil, errors.Join(err, unknownErr)
			}
			return unknown, nil
		}
		return nil, err
	}
	return AdmissionCommitted{
		contractVersion: valid.ContractVersion(),
		receipt:         receipt,
	}, nil
}

func (runtime AdmissionRuntime) PrepareAdmission(
	ctx context.Context,
	request typedmemorywire.ValidateRequest,
) (typedmemoryvalidation.ValidOutcome, error) {
	outcome, err := runtime.validation.Evaluate(ctx, request)
	if err != nil {
		return nil, err
	}
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		return nil, ErrProjectAdmissionNotValid
	}
	return valid, nil
}

// ReplayCandidateByIdempotencyKey recovers an exact prior v2 admission without
// requiring a high-level caller to retain or expose its internal snapshot
// basis. It has no write capability.
func (runtime AdmissionRuntime) ReplayCandidateByIdempotencyKey(
	ctx context.Context,
	candidate typedmemory.MemoryChangeSet,
	idempotencyKey typedmemorystore.IdempotencyKey,
	provenance typedmemory.ProvenanceRef,
) (typedmemorystore.CommitReceipt, bool, error) {
	if _, err := candidate.Digest(); err != nil ||
		idempotencyKey.String() == "" ||
		provenance.String() == "" {
		return typedmemorystore.CommitReceipt{},
			false,
			ErrProjectAdmissionRequestInvalid
	}
	store, present := runtime.store.(typedmemorystore.IdempotencyReplayPort)
	if !present || !interfaceValuePresent(store) {
		return typedmemorystore.CommitReceipt{},
			false,
			ErrProjectAdmissionIdempotencyReplayStoreMissing
	}
	request, err := typedmemorystore.NewIdempotencyReplayRequestBuilder().
		SetContractVersion(typedmemorystore.AdmissionContractV2()).
		SetProject(runtime.validation.projectID).
		SetIdempotencyKey(idempotencyKey).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build()
	if err != nil {
		return typedmemorystore.CommitReceipt{},
			false,
			ErrProjectAdmissionRequestInvalid
	}
	return store.ReplayMemoryChangeSetByIdempotencyKey(ctx, request)
}

// PrepareCandidate validates an already-typed internal candidate against the
// requested project basis and exposes admission authority only for a sealed
// ValidOutcome. It is the no-shadow-JSON counterpart of PrepareAdmission.
func (runtime AdmissionRuntime) PrepareCandidate(
	ctx context.Context,
	selector typedmemorywire.BasisSelector,
	candidate typedmemory.MemoryChangeSet,
) (typedmemoryvalidation.ValidOutcome, error) {
	outcome, err := runtime.validation.EvaluateCandidate(
		ctx,
		selector,
		candidate,
	)
	if err != nil {
		return nil, err
	}
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		return nil, ErrProjectAdmissionNotValid
	}
	return valid, nil
}

func (runtime AdmissionRuntime) AdmitValidated(
	ctx context.Context,
	valid typedmemoryvalidation.ValidOutcome,
	idempotencyKey typedmemorystore.IdempotencyKey,
	provenance typedmemory.ProvenanceRef,
) (typedmemorystore.CommitReceipt, error) {
	if !commitPortPresent(runtime.store) {
		return typedmemorystore.CommitReceipt{}, ErrProjectAdmissionStoreMissing
	}
	if valid == nil {
		return typedmemorystore.CommitReceipt{}, ErrProjectAdmissionNotValid
	}
	basis := valid.AdmissionBasis()
	contractVersion, err := typedmemorystore.ParseAdmissionContractVersion(
		valid.ContractVersion(),
	)
	if err != nil {
		return typedmemorystore.CommitReceipt{}, err
	}
	commitRequest, err := typedmemorystore.NewCommitRequestBuilder().
		SetContractVersion(contractVersion).
		SetProject(runtime.validation.projectID).
		SetExpectedRevision(basis.GraphRevision()).
		SetExpectedTypeEnv(basis.TypeEnv()).
		SetIdempotencyKey(idempotencyKey).
		SetRequestProvenance(provenance).
		SetCandidate(valid.Candidate()).
		SetAdmissionBatch(valid.AdmissionBatch()).
		Build()
	if err != nil {
		return typedmemorystore.CommitReceipt{}, err
	}
	receipt, err := runtime.store.CommitMemoryChangeSet(ctx, commitRequest)
	if err != nil {
		return typedmemorystore.CommitReceipt{}, err
	}
	return receipt, nil
}

type oneShotBasisResolver struct {
	selector   typedmemorywire.BasisSelector
	resolution typedmemoryvalidation.BasisResolution
	used       bool
}

func (resolver *oneShotBasisResolver) Resolve(
	selector typedmemorywire.BasisSelector,
) typedmemoryvalidation.BasisResolution {
	if resolver == nil || resolver.used {
		return nil
	}
	resolver.used = true
	if !sameBasisSelector(resolver.selector, selector) {
		return nil
	}
	return resolver.resolution
}

func requireProjectSelector(selector typedmemorywire.BasisSelector) error {
	if selector == nil {
		return ErrProjectBasisRequestInvalid
	}
	switch selector.(type) {
	case typedmemorywire.ProjectCurrentSelector:
		return nil
	case typedmemorywire.ExactProjectSelector:
		return nil
	default:
		return fmt.Errorf(
			"%w: %q",
			ErrProjectBasisUnsupported,
			selector.Kind(),
		)
	}
}

func requireCorrelatedResolution(
	selector typedmemorywire.BasisSelector,
	resolution typedmemoryvalidation.BasisResolution,
) error {
	if !basisResolutionPresent(resolution) {
		return ErrProjectBasisResolutionEmpty
	}
	switch requested := selector.(type) {
	case typedmemorywire.ProjectCurrentSelector:
		return requireCurrentResolution(resolution)
	case typedmemorywire.ExactProjectSelector:
		return requireExactResolution(requested, resolution)
	default:
		return ErrProjectBasisUnsupported
	}
}

func requireCurrentResolution(
	resolution typedmemoryvalidation.BasisResolution,
) error {
	switch observed := resolution.(type) {
	case *typedmemoryvalidation.ProjectBasisUnavailable:
		return nil
	case *typedmemoryvalidation.ResolvedProjectBasis:
		return requireResolvedProjectBasis(observed)
	default:
		return fmt.Errorf(
			"%w: current selector received %q",
			ErrProjectBasisUncorrelated,
			resolution.Kind(),
		)
	}
}

func requireExactResolution(
	selector typedmemorywire.ExactProjectSelector,
	resolution typedmemoryvalidation.BasisResolution,
) error {
	switch observed := resolution.(type) {
	case *typedmemoryvalidation.ProjectBasisUnavailable:
		return nil
	case *typedmemoryvalidation.ResolvedProjectBasis:
		return requireResolvedProjectBasis(observed)
	case *typedmemoryvalidation.ExactProjectBasisMismatch:
		return requireActualMismatch(selector, observed)
	default:
		return fmt.Errorf(
			"%w: exact selector received %q",
			ErrProjectBasisUncorrelated,
			resolution.Kind(),
		)
	}
}

func requireActualMismatch(
	selector typedmemorywire.ExactProjectSelector,
	observed *typedmemoryvalidation.ExactProjectBasisMismatch,
) error {
	requestedDigest := selector.RequestedTypeEnvDigest()
	observedDigest := observed.ObservedTypeEnvRef().Digest()
	if observedDigest.String() == "" {
		return fmt.Errorf(
			"%w: exact mismatch has no observed TypeEnvRef",
			ErrProjectBasisUncorrelated,
		)
	}
	requestedRevision := selector.RequestedGraphRevision()
	observedRevision := observed.ObservedGraphRevision()
	if requestedDigest != observedDigest || requestedRevision != observedRevision {
		return nil
	}
	return fmt.Errorf(
		"%w: mismatch result repeats the requested exact basis",
		ErrProjectBasisUncorrelated,
	)
}

func requireResolvedProjectBasis(
	basis *typedmemoryvalidation.ResolvedProjectBasis,
) error {
	if basis == nil {
		return ErrProjectBasisResolutionEmpty
	}
	environment := basis.Environment()
	environmentRef := environment.Ref()
	if environmentRef.Digest().String() == "" {
		return fmt.Errorf(
			"%w: resolved project basis has no TypeEnv",
			ErrProjectBasisUncorrelated,
		)
	}
	snapshot := basis.Snapshot()
	if !interfaceValuePresent(snapshot) {
		return fmt.Errorf(
			"%w: resolved project basis has no MemorySnapshot",
			ErrProjectBasisUncorrelated,
		)
	}
	if snapshot.TypeEnvRef() != environmentRef {
		return fmt.Errorf(
			"%w: resolved project basis snapshot has a different TypeEnvRef",
			ErrProjectBasisUncorrelated,
		)
	}
	return nil
}

func sameBasisSelector(
	left typedmemorywire.BasisSelector,
	right typedmemorywire.BasisSelector,
) bool {
	if left == nil || right == nil || left.Kind() != right.Kind() {
		return false
	}
	switch expected := left.(type) {
	case typedmemorywire.ProjectCurrentSelector:
		_, matches := right.(typedmemorywire.ProjectCurrentSelector)
		return matches
	case typedmemorywire.ExactProjectSelector:
		actual, matches := right.(typedmemorywire.ExactProjectSelector)
		if !matches {
			return false
		}
		return expected.RequestedTypeEnvDigest() == actual.RequestedTypeEnvDigest() &&
			expected.RequestedGraphRevision() == actual.RequestedGraphRevision()
	default:
		return false
	}
}

func validationContextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("project-memory validation context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("project-memory validation context: %w", err)
	}
	return nil
}

func projectBasisSourcePresent(source ProjectBasisSource) bool {
	return interfaceValuePresent(source)
}

func commitPortPresent(store typedmemorystore.CommitPort) bool {
	return interfaceValuePresent(store)
}

func basisResolutionPresent(
	resolution typedmemoryvalidation.BasisResolution,
) bool {
	return interfaceValuePresent(resolution)
}

func interfaceValuePresent(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !reflected.IsNil()
	default:
		return true
	}
}
