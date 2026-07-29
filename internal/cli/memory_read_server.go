package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func newProjectMemoryUnavailableReadMCPHandler() fpf.MemoryToolHandler {
	return func(
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
		result, err := projectMemoryUnavailableReadResponse(
			request.Action(),
		)
		return string(result), err
	}
}

func (runtime projectMemoryReadRuntime) EnsureReady(
	ctx context.Context,
) error {
	_, err := runtime.ProjectBasisAvailable(ctx)
	if err != nil {
		return fmt.Errorf(
			"probe current project-memory basis: %w",
			err,
		)
	}
	return nil
}

func (runtime projectMemoryReadRuntime) ProjectBasisAvailable(
	ctx context.Context,
) (bool, error) {
	if runtime.projectID.String() == "" || runtime.basis == nil {
		return false, fmt.Errorf(
			"project-memory basis probe requires one exact project runtime",
		)
	}
	resolution, err := runtime.basis.ResolveProjectBasis(
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
			"project-memory basis probe returned unsupported result %T",
			resolution,
		)
	}
}
