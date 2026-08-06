//go:build !darwin

package agenthostrestart

import (
	"context"
	"fmt"
)

func captureCurrentCodexTaskRuntime(
	context.Context,
) (TaskRuntimeIdentity, error) {
	return TaskRuntimeIdentity{}, fmt.Errorf(
		"capturing the Codex task runtime requires macOS",
	)
}
