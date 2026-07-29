package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/projectmemory"
)

type serveProjectEntitySurface interface {
	Handler() fpf.MemoryToolHandler
	Close() error
}

type serveProjectEntitySurfaceOpener func(
	context.Context,
	ProjectBinding,
) (serveProjectEntitySurface, error)

var openServeProjectEntitySurface serveProjectEntitySurfaceOpener = func(
	ctx context.Context,
	binding ProjectBinding,
) (serveProjectEntitySurface, error) {
	return openSealedProjectEntitySurface(ctx, binding)
}

type sealedProjectEntitySurface struct {
	executor projectmemory.EntityEstablishmentPort
	close    func() error
	mu       sync.Mutex
}

func openSealedProjectEntitySurface(
	ctx context.Context,
	binding ProjectBinding,
) (*sealedProjectEntitySurface, error) {
	bound, err := openBoundProjectMemoryRuntimeFromBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	closeWithError := func(openErr error) (
		*sealedProjectEntitySurface,
		error,
	) {
		return nil, errors.Join(openErr, bound.Close())
	}
	if bound.runtime.entity == nil {
		return closeWithError(
			fmt.Errorf("project entity establishment runtime is unavailable"),
		)
	}
	ready, err := bound.runtime.entity.ProjectBasisAvailable(ctx)
	if err != nil {
		return closeWithError(err)
	}
	executor, err := newStartupAwareEntityEstablishmentPort(
		bound.runtime.entity,
		bound.runtime.entity.ProjectBasisAvailable,
		ready,
	)
	if err != nil {
		return closeWithError(err)
	}
	return &sealedProjectEntitySurface{
		executor: executor,
		close:    bound.Close,
	}, nil
}

func (surface *sealedProjectEntitySurface) Handler() fpf.MemoryToolHandler {
	if surface == nil || surface.executor == nil {
		return nil
	}
	return newEntityMCPHandler(surface.execute)
}

func (surface *sealedProjectEntitySurface) execute(
	ctx context.Context,
	request projectmemory.EntityEstablishmentRequest,
) (projectmemory.EntityEstablishmentResult, error) {
	if surface == nil || surface.executor == nil {
		return nil, fmt.Errorf("project entity surface is closed")
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()
	return surface.executor.Establish(ctx, request)
}

func (surface *sealedProjectEntitySurface) Close() error {
	if surface == nil || surface.close == nil {
		return nil
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()
	closeEffect := surface.close
	surface.close = nil
	surface.executor = nil
	return closeEffect()
}

type entityEstablishmentOperation func(
	context.Context,
	projectmemory.EntityEstablishmentRequest,
) (projectmemory.EntityEstablishmentResult, error)

func newEntityMCPHandler(
	operation entityEstablishmentOperation,
) fpf.MemoryToolHandler {
	if operation == nil {
		return nil
	}
	return func(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error) {
		request, err :=
			projectmemory.DecodeEntityEstablishmentRequest(arguments)
		if err != nil {
			return "", err
		}
		result, err := operation(ctx, request)
		if err != nil {
			return "", err
		}
		payload, err :=
			projectmemory.MarshalEntityEstablishmentResult(result)
		if err != nil {
			return "", err
		}
		return string(payload), nil
	}
}
