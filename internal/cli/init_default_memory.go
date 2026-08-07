package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/m0n0x41d/haft/internal/projectledger"
	transitionsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvtransitionpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// initializeDefaultProjectMemory completes the package-owned memory runtime
// as part of project initialization. The internal schema coordinates remain
// implementation details; successful public initialization means memory is
// ready without a later enable/defer choice.
func initializeDefaultProjectMemory(
	ctx context.Context,
	projectRoot string,
	projectID string,
) error {
	binding, err := defaultMemoryProjectBinding(
		projectRoot,
		projectID,
	)
	if err != nil {
		return err
	}
	headPresent, err := reconcileBundledDefaultProjectMemory(ctx, binding)
	if err != nil {
		return fmt.Errorf("reconcile default project memory: %w", err)
	}
	if headPresent {
		return removeConsumedDefaultMemoryReview(binding.ProjectRoot)
	}
	prepared, err := prepareDefaultProjectMemory(ctx, binding)
	if err != nil {
		return err
	}
	response, err := executeDefaultMemorySelectionAtBinding(
		ctx,
		binding,
		prepared,
	)
	if err != nil {
		return fmt.Errorf("activate default project memory: %w", err)
	}
	if !defaultMemorySelectionCommitted(response) {
		return fmt.Errorf(
			"activate default project memory: initialization produced no committed memory state",
		)
	}
	if err := removeConsumedDefaultMemoryReview(binding.ProjectRoot); err != nil {
		return err
	}
	ready, err := projectMemoryReadyReadOnly(ctx, binding)
	if err != nil {
		return fmt.Errorf("verify default project memory: %w", err)
	}
	if !ready {
		return fmt.Errorf(
			"verify default project memory: initialization completed without ready memory",
		)
	}
	return nil
}

// reconcileBundledDefaultProjectMemory advances an existing project head to
// the bundled successor whenever the transaction-current compatibility policy
// is satisfied. It never fabricates an operator request or waits on a human
// review. A non-compatible or stale candidate leaves the head unchanged and
// returns a typed reason as an initialization error.
func reconcileBundledDefaultProjectMemory(
	ctx context.Context,
	binding ProjectBinding,
) (
	headPresent bool,
	runErr error,
) {
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return false, fmt.Errorf("open project memory for successor reconciliation: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	if ledger.ProjectID().String() != binding.ProjectID {
		return false, fmt.Errorf("project identity changed during successor reconciliation")
	}
	runtime, err := loadEmbeddedMemoryRuntime(ctx)
	if err != nil {
		return false, fmt.Errorf("load bundled project memory runtime: %w", err)
	}
	service, err := transitionsqlite.NewService(ctx, ledger)
	if err != nil {
		return false, err
	}
	result, preparationErr := service.PrepareAtBase(ctx, runtime.Artifact())
	switch value := result.(type) {
	case transitionsqlite.NoPriorHead:
		return false, preparationErr
	case transitionsqlite.AlreadySelected:
		return true, preparationErr
	case transitionsqlite.Prepared:
		if preparationErr != nil {
			return true, fmt.Errorf(
				"bundled successor was prepared but its project basis changed: %w",
				preparationErr,
			)
		}
		response, selectionErr := executeAutomaticCompatibleTransition(
			ctx,
			ledger,
			value.Candidate(),
			typedmemorystore.SystemClock{},
		)
		if selectionErr != nil {
			return true, selectionErr
		}
		if defaultMemorySelectionCommitted(response) {
			return true, nil
		}
		if notSelected, ok := response.Outcome.(projectTypeEnvGenesisNotSelected); ok {
			return true, fmt.Errorf(
				"bundled successor was not automatically selected: reason=%s repair=%s",
				notSelected.Reason,
				notSelected.Repair,
			)
		}
		return true, fmt.Errorf(
			"bundled successor returned an unsupported selection outcome %T",
			response.Outcome,
		)
	case nil:
		return false, preparationErr
	default:
		return false, fmt.Errorf(
			"successor reconciliation returned an unsupported result %T",
			result,
		)
	}
}

func removeConsumedDefaultMemoryReview(projectRoot string) error {
	path := projectTypeEnvGenesisReviewPath(projectRoot)
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect consumed default memory review: %w", err)
	}
	schema, err := projectTypeEnvReviewSchema(projectRoot)
	if err != nil {
		return fmt.Errorf("inspect consumed default memory review schema: %w", err)
	}
	if schema != projectTypeEnvGenesisReviewSchema &&
		schema != projectTypeEnvTransitionReviewSchema {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove consumed default memory review: %w", err)
	}
	return nil
}

func defaultMemoryProjectBinding(
	projectRoot string,
	projectID string,
) (ProjectBinding, error) {
	input, err := absProjectRootInput(
		projectRoot,
		projectRootSourceFlag,
	)
	if err != nil {
		return ProjectBinding{}, err
	}
	binding, err := resolveProjectBindingFromInput(input, projectID)
	if err != nil {
		return ProjectBinding{}, err
	}
	return binding, nil
}

func prepareDefaultProjectMemory(
	ctx context.Context,
	binding ProjectBinding,
) (
	observed observedProjectTypeEnvGenesisReview,
	runErr error,
) {
	ledger, err := projectledger.OpenExisting(
		ctx,
		binding.ProjectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("open project memory during initialization: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	if ledger.ProjectID().String() != binding.ProjectID {
		return observedProjectTypeEnvGenesisReview{}, fmt.Errorf(
			"prepare default project memory: project identity changed",
		)
	}
	runtime, err := loadEmbeddedMemoryRuntime(ctx)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("load default project memory runtime: %w", err)
	}
	prepared, err := prepareProjectTypeEnvGenesisReview(
		ctx,
		ledger,
		runtime.Artifact(),
		typedmemorystore.SystemClock{},
	)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("prepare default project memory: %w", err)
	}
	if prepared.response.Review.Readiness.Posture != "selectable" {
		return observedProjectTypeEnvGenesisReview{}, fmt.Errorf(
			"prepare default project memory: bundled runtime is not selectable: posture=%s reasons=%v",
			prepared.response.Review.Readiness.Posture,
			prepared.response.Review.Readiness.Reasons,
		)
	}
	observed, err = observePreparedDefaultMemoryReview(prepared.carrier)
	if err != nil {
		return observedProjectTypeEnvGenesisReview{}, err
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return observedProjectTypeEnvGenesisReview{},
			fmt.Errorf("revalidate default project memory basis: %w", err)
	}
	return observed, nil
}

func executeDefaultMemorySelectionAtBinding(
	ctx context.Context,
	binding ProjectBinding,
	observed observedProjectTypeEnvGenesisReview,
) (
	response projectTypeEnvGenesisSelectionResponse,
	runErr error,
) {
	ledger, binding, err := openProjectTypeEnvGenesisLedgerAtBinding(
		ctx,
		projectledger.ReadWrite,
		binding,
	)
	if err != nil {
		return projectTypeEnvGenesisSelectionResponse{}, err
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	return executeObservedGenesisSelection(
		ctx,
		ledger,
		binding,
		observed,
	)
}

func defaultMemorySelectionCommitted(
	response projectTypeEnvGenesisSelectionResponse,
) bool {
	switch response.Outcome.(type) {
	case projectTypeEnvGenesisFreshlyCommitted:
		return true
	case projectTypeEnvGenesisReplayedExisting:
		return true
	default:
		return false
	}
}
