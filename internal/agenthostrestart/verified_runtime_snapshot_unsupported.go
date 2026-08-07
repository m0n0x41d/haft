//go:build !darwin && !linux

package agenthostrestart

import "fmt"

func LoadVerifiedRuntimeSnapshot(
	string,
) (VerifiedRuntimeSnapshot, error) {
	return VerifiedRuntimeSnapshot{}, fmt.Errorf(
		"verified agent-host restart evidence is unsupported on this platform",
	)
}
