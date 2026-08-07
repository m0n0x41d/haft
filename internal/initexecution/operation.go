package initexecution

import (
	"context"
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

// PreparedInitOperation binds one immutable compiled plan to its effect
// resources without performing IO. The exact preview must be confirmed before
// the operation can be applied.
type PreparedInitOperation struct {
	plan        initplanning.InitPlan
	registry    HostManifestRegistry
	coordinator initfs.PublicationCoordinator
	prepared    bool
}

func PrepareCoreOnlyInitOperation(
	plan initplanning.InitPlan,
) (PreparedInitOperation, error) {
	if len(plan.Hosts()) != 0 {
		return PreparedInitOperation{}, fmt.Errorf(
			"core-only init preparation cannot bind host publications",
		)
	}
	registry, err := NewHostManifestRegistry(nil)
	if err != nil {
		return PreparedInitOperation{}, err
	}
	return PreparedInitOperation{
		plan:     plan,
		registry: registry,
		prepared: true,
	}, nil
}

func PrepareHostInitOperation(
	plan initplanning.InitPlan,
	userHomeRoot string,
	maxManifestBytes int64,
) (PreparedInitOperation, error) {
	if len(plan.Hosts()) == 0 {
		return PreparedInitOperation{}, fmt.Errorf(
			"host init preparation requires at least one host publication",
		)
	}
	registry, coordinator, err := BindCanonicalPublicationResources(
		plan,
		userHomeRoot,
		maxManifestBytes,
	)
	if err != nil {
		return PreparedInitOperation{}, err
	}
	return PreparedInitOperation{
		plan:        plan,
		registry:    registry,
		coordinator: coordinator,
		prepared:    true,
	}, nil
}

func (operation PreparedInitOperation) Preview() (
	initplanning.InitPlanPreview,
	error,
) {
	if !operation.prepared {
		return initplanning.InitPlanPreview{}, fmt.Errorf(
			"init operation is not prepared",
		)
	}
	return operation.plan.Preview(), nil
}

func (operation PreparedInitOperation) PublicationResources() (
	[]string,
	error,
) {
	if !operation.prepared {
		return nil, fmt.Errorf("init operation is not prepared")
	}
	return executionResources(
		operation.plan,
		operation.registry,
	), nil
}

func (operation PreparedInitOperation) ConfirmPreview(
	reviewed initplanning.InitPlanPreview,
) (ConfirmedInitOperation, error) {
	exact, err := operation.Preview()
	if err != nil {
		return ConfirmedInitOperation{}, err
	}
	if exact.Readiness != initplanning.PlanReady {
		return ConfirmedInitOperation{}, fmt.Errorf(
			"blocked init plan cannot be confirmed",
		)
	}
	if !reflect.DeepEqual(exact, reviewed) {
		return ConfirmedInitOperation{}, fmt.Errorf(
			"reviewed init preview differs from the prepared plan",
		)
	}
	return ConfirmedInitOperation{
		operation: operation,
		confirmed: true,
	}, nil
}

// ConfirmedInitOperation is the only prepared-operation state that exposes
// Apply. Public shells remain responsible for creating this state only after
// their explicit review event.
type ConfirmedInitOperation struct {
	operation PreparedInitOperation
	confirmed bool
}

func (operation ConfirmedInitOperation) Apply(
	ctx context.Context,
	executor Executor,
) (InitExecutionOutcome, error) {
	if !operation.confirmed || !operation.operation.prepared {
		return InitExecutionOutcome{}, fmt.Errorf(
			"init operation is not confirmed",
		)
	}
	return executor.Execute(
		ctx,
		operation.operation.plan,
		operation.operation.registry,
		operation.operation.coordinator,
	)
}

func (operation ConfirmedInitOperation) ApplyUnderPublicationLease(
	ctx context.Context,
	executor Executor,
	lease *initfs.PublicationCoordinationLease,
) (InitExecutionOutcome, error) {
	if !operation.confirmed || !operation.operation.prepared {
		return InitExecutionOutcome{}, fmt.Errorf(
			"init operation is not confirmed",
		)
	}
	resources := executionResources(
		operation.operation.plan,
		operation.operation.registry,
	)
	if err := lease.RequireCoverage(resources); err != nil {
		return InitExecutionOutcome{}, err
	}
	return executor.ExecuteUnderPublicationLease(
		ctx,
		operation.operation.plan,
		operation.operation.registry,
	)
}
