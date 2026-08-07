package projectmemory

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const maximumEntityEstablishmentCASAttempts = 3

type EntityEstablishmentRuntime struct {
	projectID projectledger.ProjectID
	source    ProjectBasisSource
	admission AdmissionRuntime
	entities  typedmemorystore.EntityContextReadPort
}

var _ EntityEstablishmentPort = (*EntityEstablishmentRuntime)(nil)

func NewEntityEstablishmentRuntime(
	projectID projectledger.ProjectID,
	source ProjectBasisSource,
	admission AdmissionRuntime,
	entities typedmemorystore.EntityContextReadPort,
) (*EntityEstablishmentRuntime, error) {
	if projectID.String() == "" || admission.ProjectID() != projectID {
		return nil, fmt.Errorf(
			"entity establishment runtime requires one exact project identity",
		)
	}
	if !projectBasisSourcePresent(source) ||
		!interfaceValuePresent(entities) {
		return nil, fmt.Errorf(
			"entity establishment runtime requires basis, admission, and entity-read ports",
		)
	}
	return &EntityEstablishmentRuntime{
		projectID: projectID,
		source:    source,
		admission: admission,
		entities:  entities,
	}, nil
}

func (runtime *EntityEstablishmentRuntime) ProjectBasisAvailable(
	ctx context.Context,
) (bool, error) {
	if runtime == nil ||
		runtime.projectID.String() == "" ||
		!projectBasisSourcePresent(runtime.source) {
		return false, fmt.Errorf("entity establishment runtime is incomplete")
	}
	resolution, err := runtime.source.ResolveProjectBasis(
		ctx,
		runtime.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if err != nil {
		return false, err
	}
	switch resolution.(type) {
	case *typedmemoryvalidation.ProjectBasisUnavailable:
		return false, nil
	case *typedmemoryvalidation.ResolvedProjectBasis:
		return true, nil
	default:
		return false, fmt.Errorf(
			"entity establishment basis returned unsupported result %T",
			resolution,
		)
	}
}

func (runtime *EntityEstablishmentRuntime) Establish(
	ctx context.Context,
	request EntityEstablishmentRequest,
) (EntityEstablishmentResult, error) {
	if runtime == nil ||
		runtime.projectID.String() == "" ||
		!projectBasisSourcePresent(runtime.source) ||
		!interfaceValuePresent(runtime.entities) {
		return nil, fmt.Errorf("entity establishment runtime is incomplete")
	}
	if err := validationContextError(ctx); err != nil {
		return nil, err
	}
	candidate, err := request.Candidate()
	if err != nil {
		return nil, fmt.Errorf("construct entity establishment candidate: %w", err)
	}
	replayed, found, err :=
		runtime.admission.ReplayCandidateByIdempotencyKey(
			ctx,
			candidate,
			request.idempotencyKey,
			request.requestProvenance,
		)
	if err != nil {
		return runtime.entityReplayError(request, err)
	}
	if found {
		return NewCommittedEntityEstablished(
			request,
			EntityReplayedDelivery,
			replayed,
		)
	}

	for attempt := 0; attempt < maximumEntityEstablishmentCASAttempts; attempt++ {
		result, retry, establishErr := runtime.establishAtCurrentBasis(
			ctx,
			request,
			candidate,
		)
		if establishErr != nil {
			return nil, establishErr
		}
		if !retry {
			return result, nil
		}
	}
	return NewEntityEstablishmentRejected([]string{
		"Typed project memory changed concurrently during three compare-and-swap attempts.",
		"Retry the unchanged request with the same idempotency_key.",
	})
}

func (runtime *EntityEstablishmentRuntime) entityReplayError(
	request EntityEstablishmentRequest,
	err error,
) (EntityEstablishmentResult, error) {
	if errors.Is(err, typedmemorystore.ErrIdempotencyConflict) {
		return NewEntityIdempotencyConflict(
			"The idempotency_key is already attached to a different entity establishment request.",
		)
	}
	if errors.Is(err, typedmemorystore.ErrCommitOutcomeUnknown) {
		return NewEntityEstablishmentCommitOutcomeUnknown(
			request,
			err.Error(),
		)
	}
	return nil, err
}

func (runtime *EntityEstablishmentRuntime) establishAtCurrentBasis(
	ctx context.Context,
	request EntityEstablishmentRequest,
	candidate typedmemory.MemoryChangeSet,
) (EntityEstablishmentResult, bool, error) {
	resolution, err := runtime.source.ResolveProjectBasis(
		ctx,
		runtime.projectID,
		typedmemorywire.ProjectCurrentSelector{},
	)
	if err != nil {
		return nil, false, err
	}
	switch basis := resolution.(type) {
	case *typedmemoryvalidation.ProjectBasisUnavailable:
		result, resultErr := NewEntityOnboardingRequired(
			"Default project memory is incomplete; rerun haft init. No entity was written.",
		)
		return result, false, resultErr
	case *typedmemoryvalidation.ResolvedProjectBasis:
		return runtime.establishAtResolvedBasis(
			ctx,
			request,
			candidate,
			basis,
		)
	default:
		result, resultErr := NewEntityEstablishmentRejected([]string{
			fmt.Sprintf(
				"Current project-memory basis returned unsupported result %T.",
				resolution,
			),
		})
		return result, false, resultErr
	}
}

func (runtime *EntityEstablishmentRuntime) establishAtResolvedBasis(
	ctx context.Context,
	request EntityEstablishmentRequest,
	candidate typedmemory.MemoryChangeSet,
	basis *typedmemoryvalidation.ResolvedProjectBasis,
) (EntityEstablishmentResult, bool, error) {
	if basis == nil || !interfaceValuePresent(basis.Snapshot()) {
		return nil, false, ErrProjectBasisResolutionEmpty
	}
	preflight, err := runtime.preflightEntityEstablishment(
		ctx,
		request,
		basis.Snapshot(),
	)
	if err != nil || preflight != nil {
		return preflight, false, err
	}
	exact, err := typedmemorywire.NewExactProjectSelector(
		basis.Environment().Ref().Digest(),
		basis.Snapshot().GraphRevision(),
	)
	if err != nil {
		return nil, false, err
	}
	outcome, err := runtime.admission.validation.EvaluateCandidate(
		ctx,
		exact,
		candidate,
	)
	if err != nil {
		return nil, false, err
	}
	if outcome.BasisProjection().ResolutionKind() ==
		typedmemoryvalidation.BasisResolutionExactMismatch {
		return nil, true, nil
	}
	valid, accepted := outcome.(typedmemoryvalidation.ValidOutcome)
	if !accepted {
		result, resultErr := NewEntityEstablishmentRejected(
			entityValidationDetails(outcome),
		)
		return result, false, resultErr
	}
	receipt, err := runtime.admission.AdmitValidated(
		ctx,
		valid,
		request.idempotencyKey,
		request.requestProvenance,
	)
	if err != nil {
		return runtime.entityCommitError(request, err)
	}
	delivery := EntityFreshlyCommittedDelivery
	if receipt.Disposition() == typedmemorystore.CommitReplay {
		delivery = EntityReplayedDelivery
	}
	result, err := NewCommittedEntityEstablished(
		request,
		delivery,
		receipt,
	)
	return result, false, err
}

func (runtime *EntityEstablishmentRuntime) preflightEntityEstablishment(
	ctx context.Context,
	request EntityEstablishmentRequest,
	snapshot typedmemory.MemorySnapshot,
) (EntityEstablishmentResult, error) {
	resolution := snapshot.ResolveEntity(request.entityID, request.context)
	switch exact := resolution.(type) {
	case typedmemory.AbsentEntityResolution:
		if exact.Entity() != request.entityID ||
			exact.Context() != request.context {
			return NewEntityEstablishmentRejected([]string{
				"Entity absence evidence was not correlated with the requested EntityID and context.",
			})
		}
		return preflightAbsentEntityAliases(request, snapshot)
	case typedmemory.ExactEntityResolution:
		if exact.Entity() != request.entityID ||
			exact.Context() != request.context {
			return NewEntityEstablishmentRejected([]string{
				"Exact entity evidence was not correlated with the requested EntityID and context.",
			})
		}
		return runtime.preflightExistingEntity(ctx, request, snapshot)
	case typedmemory.CandidateEntityResolution:
		return NewEntityEstablishmentRejected([]string{
			"The EntityID has unresolved identity candidates; select an exact candidate before establishment.",
		})
	case typedmemory.UnknownEntityResolution,
		typedmemory.UnsettledEntityResolution:
		return NewEntityEstablishmentRejected([]string{
			"The EntityID absence basis is incomplete; establishment requires exact absence evidence.",
		})
	default:
		return NewEntityEstablishmentRejected([]string{
			"The EntityID lookup returned no recognized identity basis.",
		})
	}
}

func preflightAbsentEntityAliases(
	request EntityEstablishmentRequest,
	snapshot typedmemory.MemorySnapshot,
) (EntityEstablishmentResult, error) {
	for _, alias := range request.aliases {
		resolution := snapshot.ResolveAlias(alias, request.context)
		switch exact := resolution.(type) {
		case typedmemory.UnboundAliasResolution:
			if exact.Alias() != alias ||
				exact.Context() != request.context {
				return NewEntityEstablishmentRejected([]string{
					fmt.Sprintf(
						"Alias %q availability evidence was not correlated with the request.",
						alias.String(),
					),
				})
			}
		case typedmemory.BoundAliasResolution:
			if exact.Alias() != alias ||
				exact.Context() != request.context {
				return NewEntityEstablishmentRejected([]string{
					fmt.Sprintf(
						"Alias %q binding evidence was not correlated with the request.",
						alias.String(),
					),
				})
			}
			return NewEntityAliasConflict(
				alias,
				exact.Entity(),
				fmt.Sprintf(
					"Alias %q is already bound to another established entity in this context.",
					alias.String(),
				),
			)
		case typedmemory.CandidateAliasResolution,
			typedmemory.UnsettledAliasResolution:
			return NewEntityEstablishmentRejected([]string{
				fmt.Sprintf(
					"Alias %q has unresolved identity candidates or incomplete availability evidence.",
					alias.String(),
				),
			})
		default:
			return NewEntityEstablishmentRejected([]string{
				fmt.Sprintf(
					"Alias %q lookup returned no recognized availability basis.",
					alias.String(),
				),
			})
		}
	}
	return nil, nil
}

func (runtime *EntityEstablishmentRuntime) preflightExistingEntity(
	ctx context.Context,
	request EntityEstablishmentRequest,
	snapshot typedmemory.MemorySnapshot,
) (EntityEstablishmentResult, error) {
	stored, err := runtime.entities.LoadEntityContext(
		ctx,
		runtime.projectID,
		request.entityID,
		request.context,
	)
	if err != nil {
		return nil, err
	}
	if stored.Project() != runtime.projectID ||
		stored.Entity() != request.entityID ||
		stored.Context() != request.context {
		return nil, fmt.Errorf(
			"stored entity context is not correlated with the requested identity",
		)
	}
	if stored.Label() != request.label {
		return NewEntityIdentityConflict(
			request.entityID,
			fmt.Sprintf(
				"EntityID %q already exists in this context with label %q, not %q.",
				request.entityID.String(),
				stored.Label().String(),
				request.label.String(),
			),
		)
	}
	for _, alias := range request.aliases {
		resolution := snapshot.ResolveAlias(alias, request.context)
		switch exact := resolution.(type) {
		case typedmemory.BoundAliasResolution:
			if exact.Alias() != alias ||
				exact.Context() != request.context {
				return NewEntityEstablishmentRejected([]string{
					fmt.Sprintf(
						"Alias %q binding evidence was not correlated with the request.",
						alias.String(),
					),
				})
			}
			if exact.Entity() != request.entityID {
				return NewEntityAliasConflict(
					alias,
					exact.Entity(),
					fmt.Sprintf(
						"Alias %q is already bound to another established entity in this context.",
						alias.String(),
					),
				)
			}
		case typedmemory.UnboundAliasResolution:
			return NewEntityIdentityConflict(
				request.entityID,
				fmt.Sprintf(
					"EntityID %q already exists, but requested alias %q is not part of its exact identity; this atomic establishment call made no partial change.",
					request.entityID.String(),
					alias.String(),
				),
			)
		default:
			return NewEntityEstablishmentRejected([]string{
				fmt.Sprintf(
					"Alias %q does not have an exact binding basis.",
					alias.String(),
				),
			})
		}
	}
	return NewAlreadyExactEntityEstablished(request)
}

func (runtime *EntityEstablishmentRuntime) entityCommitError(
	request EntityEstablishmentRequest,
	err error,
) (EntityEstablishmentResult, bool, error) {
	switch {
	case errors.Is(err, typedmemorystore.ErrIdempotencyConflict):
		result, resultErr := NewEntityIdempotencyConflict(
			"The idempotency_key was consumed by a different entity establishment request.",
		)
		return result, false, resultErr
	case errors.Is(err, typedmemorystore.ErrCommitOutcomeUnknown):
		result, resultErr := NewEntityEstablishmentCommitOutcomeUnknown(
			request,
			err.Error(),
		)
		return result, false, resultErr
	case errors.Is(err, typedmemorystore.ErrStaleGraphRevision),
		errors.Is(err, typedmemorystore.ErrActiveTypeEnvMismatch),
		errors.Is(err, typedmemorystore.ErrRevalidationRejected),
		errors.Is(err, typedmemorystore.ErrAdmissionEnvelopeMismatch):
		return nil, true, nil
	default:
		return nil, false, err
	}
}

func entityValidationDetails(
	outcome typedmemoryvalidation.Outcome,
) []string {
	details := make([]string, 0, len(outcome.Diagnostics())+1)
	for _, diagnostic := range outcome.Diagnostics() {
		details = append(
			details,
			entityValidationDetail(diagnostic),
		)
	}
	if len(details) == 0 {
		details = append(
			details,
			fmt.Sprintf(
				"Entity establishment validation returned %s without a sealed admission.",
				outcome.Verdict(),
			),
		)
	}
	return details
}

func entityValidationDetail(
	diagnostic typedmemoryvalidation.DiagnosticProjection,
) string {
	switch diagnostic.Code() {
	case string(typedmemory.DiagnosticContextNotActive):
		return "The bounded_context_ref is not enabled for typed project memory."
	case string(typedmemory.DiagnosticEntityAlreadyExists):
		return "The EntityID became established concurrently; retry the unchanged request to recover its exact state."
	case string(typedmemory.DiagnosticAliasAlreadyBound),
		string(typedmemory.DiagnosticAliasAmbiguous):
		return "A requested alias became bound concurrently; retry the unchanged request to receive the exact conflict."
	case string(typedmemory.DiagnosticIdentityBasisMissing):
		return "Exact entity or alias identity evidence is unavailable for this request."
	default:
		return fmt.Sprintf(
			"Entity establishment was rejected at %s (%s).",
			diagnostic.Path(),
			diagnostic.Code(),
		)
	}
}
