package cli

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/projectmemory"
)

type entityBasisAvailabilityProbe func(context.Context) (bool, error)

type startupAwareEntityEstablishmentPort struct {
	inner          projectmemory.EntityEstablishmentPort
	probe          entityBasisAvailabilityProbe
	readyAtStartup bool
}

func newStartupAwareEntityEstablishmentPort(
	inner projectmemory.EntityEstablishmentPort,
	probe entityBasisAvailabilityProbe,
	readyAtStartup bool,
) (*startupAwareEntityEstablishmentPort, error) {
	if inner == nil || probe == nil {
		return nil, fmt.Errorf(
			"startup-aware entity establishment requires runtime and basis probe",
		)
	}
	return &startupAwareEntityEstablishmentPort{
		inner:          inner,
		probe:          probe,
		readyAtStartup: readyAtStartup,
	}, nil
}

func (port *startupAwareEntityEstablishmentPort) Establish(
	ctx context.Context,
	request projectmemory.EntityEstablishmentRequest,
) (projectmemory.EntityEstablishmentResult, error) {
	if port == nil || port.inner == nil || port.probe == nil {
		return nil, fmt.Errorf(
			"startup-aware entity establishment port is incomplete",
		)
	}
	readyNow, err := port.probe(ctx)
	if err != nil {
		return nil, err
	}
	if !port.readyAtStartup && readyNow {
		return projectmemory.NewEntityRestartRequired(
			"Typed project memory became ready after this MCP server started; the stale process made no entity change.",
		)
	}
	return port.inner.Establish(ctx, request)
}

type fixedEntityEstablishmentPort struct {
	result projectmemory.EntityEstablishmentResult
}

func (port fixedEntityEstablishmentPort) Establish(
	context.Context,
	projectmemory.EntityEstablishmentRequest,
) (projectmemory.EntityEstablishmentResult, error) {
	if port.result == nil {
		return nil, fmt.Errorf("fixed entity establishment result is missing")
	}
	return port.result, nil
}

func newEntityOnboardingRequiredMCPHandler(
	detail string,
) (fpf.MemoryToolHandler, error) {
	result, err := projectmemory.NewEntityOnboardingRequired(detail)
	if err != nil {
		return nil, err
	}
	return newEntityMCPHandler(
		fixedEntityEstablishmentPort{result: result}.Establish,
	), nil
}
