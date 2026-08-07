package codeintel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/codebase"
)

// EnsureIndex coordinates a request-time freshness attempt. Waiting for an
// existing owner is bounded independently of the outer request so a retained
// complete epoch can still be queried before the caller's own deadline.
func (service *Service) EnsureIndex(
	ctx context.Context,
	projectRoot string,
) (IndexCoordinationResult, error) {
	return service.ensureIndex(ctx, projectRoot, indexAttemptRequest)
}

// EnsureIndexForStartup performs the same freshness decision with a
// non-blocking follower policy. Ordinary lease contention is a deferred result,
// not a startup failure.
func (service *Service) EnsureIndexForStartup(
	ctx context.Context,
	projectRoot string,
) (IndexCoordinationResult, error) {
	return service.ensureIndex(ctx, projectRoot, indexAttemptStartup)
}

func (service *Service) indexCoordinator(
	projectRoot string,
) (*ProjectIndexCoordinator, error) {
	service.coordinatorMu.Lock()
	defer service.coordinatorMu.Unlock()
	if service.coordinator == nil {
		service.coordinator = newProcessOnlyIndexCoordinator(projectRoot)
	}
	if err := service.coordinator.validateProjectRoot(projectRoot); err != nil {
		return nil, err
	}
	return service.coordinator, nil
}

func (service *Service) acquireIndexRead(
	projectRoot string,
) (func(), error) {
	coordinator, err := service.indexCoordinator(projectRoot)
	if err != nil {
		return nil, err
	}
	coordinator.state.readMu.RLock()
	return coordinator.state.readMu.RUnlock, nil
}

func (service *Service) ensureIndex(
	ctx context.Context,
	projectRoot string,
	policy indexAttemptPolicy,
) (IndexCoordinationResult, error) {
	if ctx == nil {
		return IndexCoordinationResult{}, fmt.Errorf(
			"code-index freshness context is required",
		)
	}
	coordinator, err := service.indexCoordinator(projectRoot)
	if err != nil {
		return IndexCoordinationResult{}, err
	}
	started := time.Now()
	finish := func(result IndexCoordinationResult) IndexCoordinationResult {
		coordinator.emit(result)
		return result
	}

	waitContext := ctx
	cancelWait := func() {}
	if policy == indexAttemptRequest {
		waitContext, cancelWait = boundedIndexWaitContext(ctx)
	}
	defer cancelWait()

	gateAcquired, gateContended := coordinator.acquireProcessGate(
		waitContext,
		policy,
	)
	if !gateAcquired {
		if policy == indexAttemptStartup {
			result := service.deferredIndexResult(
				ctx,
				"code-index rebuild already owned for this project",
			)
			result.WaitDuration = time.Since(started)
			return finish(result), nil
		}
		result := service.failedIndexResult(
			ctx,
			waitContext.Err(),
			"code-index rebuild wait bound reached",
			"",
		)
		result.WaitDuration = time.Since(started)
		return finish(result), nil
	}

	lease, leaseAcquired, leaseContended, leaseErr := coordinator.acquireLease(
		waitContext,
		policy,
	)
	contended := gateContended || leaseContended
	if !leaseAcquired {
		coordinator.releaseProcessGate()
		if leaseErr == nil && policy == indexAttemptStartup {
			result := service.deferredIndexResult(
				ctx,
				"code-index rebuild already owned for this project",
			)
			result.WaitDuration = time.Since(started)
			return finish(result), nil
		}
		result := service.failedIndexResult(
			ctx,
			leaseErr,
			"code-index rebuild lease unavailable",
			"",
		)
		result.WaitDuration = time.Since(started)
		return finish(result), nil
	}
	waitDuration := time.Since(started)

	coordinator.state.readMu.Lock()
	result := service.ensureIndexWhileLeaseOwned(
		ctx,
		projectRoot,
		coordinator,
		contended,
	)
	coordinator.state.readMu.Unlock()
	releaseErr := lease.release()
	coordinator.releaseProcessGate()
	if releaseErr != nil {
		result = service.failedIndexResult(
			ctx,
			releaseErr,
			"release code-index rebuild lease",
			result.SourceFingerprint,
		)
	}
	result.WaitDuration = waitDuration
	return finish(result), nil
}

func (service *Service) ensureIndexWhileLeaseOwned(
	ctx context.Context,
	projectRoot string,
	coordinator *ProjectIndexCoordinator,
	contended bool,
) IndexCoordinationResult {
	if hook := coordinator.hooks.afterLease; hook != nil {
		if err := hook(ctx); err != nil {
			return service.failedIndexResult(
				ctx,
				err,
				"code-index post-lease checkpoint failed",
				"",
			)
		}
	}
	observation, err := service.scanner.ObserveIndexFreshness(
		ctx,
		projectRoot,
	)
	if err != nil {
		if codebase.IsDatabaseIntegrityFailure(err) {
			return service.failedIndexResult(
				ctx,
				err,
				"observe code-index freshness after lease acquisition",
				"",
			)
		}
		// A never-initialized ledger still needs the zero-epoch metadata shape
		// so the request can return a structured unavailable result. This
		// schema preparation occurs only while the project lease is owned and
		// cannot start parser workers. Structural damage is returned above and
		// must never be followed by a schema write.
		err = errors.Join(err, service.scanner.EnsureIncrementalSchema(ctx))
		return service.failedIndexResult(
			ctx,
			err,
			"observe code-index freshness after lease acquisition",
			"",
		)
	}
	decision := decideIndexCoordination(indexCoordinationPolicyInput{
		Fresh:         indexObservationIsFresh(observation),
		LeaseAcquired: true,
		Contended:     contended,
		Epoch:         observation.PublishedEpoch,
	})
	if !decision.EnterRebuild {
		return IndexCoordinationResult{
			Outcome:           decision.Outcome,
			SourceFingerprint: observation.SourceFingerprint,
			PublishedEpoch:    observation.PublishedEpoch,
			Reason:            decision.Reason,
		}
	}

	if err := service.scanner.RequireDatabaseIntegrityForIndexRefresh(ctx); err != nil {
		return service.failedIndexResult(
			ctx,
			err,
			"verify project ledger integrity before code-index rebuild",
			observation.SourceFingerprint,
		)
	}

	if hook := coordinator.hooks.beforeRefresh; hook != nil {
		if err := hook(ctx); err != nil {
			return service.failedIndexResult(
				ctx,
				err,
				"code-index pre-refresh checkpoint failed",
				observation.SourceFingerprint,
			)
		}
	}
	refresh, refreshErr := service.scanner.RefreshIncremental(ctx, projectRoot)
	if refreshErr == nil && !refresh.Degraded {
		refreshErr = service.finishIndexRefresh(
			ctx,
			projectRoot,
			observation.SourceFingerprint,
			refresh,
		)
	}
	if hook := coordinator.hooks.afterRefresh; hook != nil {
		refreshErr = errors.Join(refreshErr, hook(ctx, refresh, refreshErr))
	}
	if refresh.Degraded && refreshErr == nil {
		refreshErr = errors.New(strings.TrimSpace(refresh.Reason))
		if refreshErr.Error() == "" {
			refreshErr = errors.New("code-index refresh degraded")
		}
	}
	if refreshErr != nil {
		return service.failedIndexResult(
			ctx,
			refreshErr,
			"refresh code index while owning project lease",
			observation.SourceFingerprint,
		)
	}
	state, stateErr := service.scanner.CurrentIndexState(ctx)
	if stateErr != nil {
		return service.failedIndexResult(
			ctx,
			stateErr,
			"inspect code-index state after refresh",
			observation.SourceFingerprint,
		)
	}
	decision = decideIndexCoordination(indexCoordinationPolicyInput{
		LeaseAcquired:    true,
		Contended:        contended,
		RefreshComplete:  true,
		RefreshPublished: refresh.Published,
		Epoch:            state.Epoch,
	})
	return IndexCoordinationResult{
		Outcome:           decision.Outcome,
		SourceFingerprint: observation.SourceFingerprint,
		PublishedEpoch:    state.Epoch,
		Reason:            decision.Reason,
	}
}

func (service *Service) finishIndexRefresh(
	ctx context.Context,
	projectRoot string,
	observedFingerprint string,
	refresh codebase.IndexRefreshResult,
) error {
	modules, err := service.scanner.GetModules(ctx)
	if err != nil {
		return fmt.Errorf("inspect code modules: %w", err)
	}
	if refresh.Published || len(modules) == 0 {
		if _, err := service.scanner.ScanModules(ctx, projectRoot); err != nil {
			return fmt.Errorf("refresh code modules: %w", err)
		}
	}
	if !refresh.Published {
		// RefreshIncremental proved that content/config/schema were unchanged.
		// Normalize a metadata-only source change so every follower does not
		// repeat the same corpus scan.
		if err := service.scanner.SetFingerprint(
			ctx,
			observedFingerprint,
		); err != nil {
			return fmt.Errorf(
				"record unchanged code source fingerprint: %w",
				err,
			)
		}
	}
	return nil
}

func (service *Service) deferredIndexResult(
	ctx context.Context,
	reason string,
) IndexCoordinationResult {
	state, stateErr := service.indexStateForCoordination(ctx)
	if stateErr != nil {
		reason = coordinationFailureReason(errors.New(reason), stateErr)
	}
	return IndexCoordinationResult{
		Outcome:        IndexDeferredBusy,
		PublishedEpoch: state.Epoch,
		Reason:         reason,
	}
}

func (service *Service) failedIndexResult(
	ctx context.Context,
	cause error,
	boundary string,
	sourceFingerprint string,
) IndexCoordinationResult {
	state, stateErr := service.indexStateForCoordination(ctx)
	reason := strings.TrimSpace(boundary)
	if cause != nil {
		reason = coordinationFailureReason(errors.New(reason), cause)
	}
	if stateErr != nil {
		reason = coordinationFailureReason(errors.New(reason), stateErr)
	}
	decision := decideIndexCoordination(indexCoordinationPolicyInput{
		Failure: reason,
		Epoch:   state.Epoch,
	})
	return IndexCoordinationResult{
		Outcome:           decision.Outcome,
		SourceFingerprint: sourceFingerprint,
		PublishedEpoch:    state.Epoch,
		Reason:            decision.Reason,
	}
}

func (service *Service) indexStateForCoordination(
	ctx context.Context,
) (codebase.IndexState, error) {
	readContext := ctx
	cancel := func() {}
	if ctx == nil || ctx.Err() != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		readContext, cancel = context.WithTimeout(
			context.WithoutCancel(ctx),
			250*time.Millisecond,
		)
	}
	defer cancel()
	return service.scanner.CurrentIndexState(readContext)
}
