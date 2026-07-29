//go:build !darwin

package agenthostrestart

import "fmt"

func NewCommandEffects(
	string,
	applicationTerminationPolicy,
) (Effects, error) {
	return nil, fmt.Errorf("agent-host restart command effects require macOS")
}
