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
		"Codex task runtime capture requires macOS",
	)
}
