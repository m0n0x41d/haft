//go:build !darwin && !linux

package agenthostrestart

import (
	"fmt"
	"os"
)

const (
	RestartDirectoryName = "restart"
	CheckpointFileName   = "checkpoint.json"
)

// Store is unavailable because the self-restart acceptance mechanism is
// intentionally limited to the macOS/Linux filesystem and locking contract.
type Store struct{}

func NewStore(string) (Store, error) {
	return Store{}, fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) CheckpointPath() string { return "" }

func (Store) SupervisorLogPath(string) (string, error) {
	return "", fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) Prepare(Checkpoint) error {
	return fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) Apply(Change) error {
	return fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) Load() (Checkpoint, error) {
	return Checkpoint{}, fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) OpenSupervisorLog(Checkpoint) (*os.File, error) {
	return nil, fmt.Errorf("agent-host restart store is unsupported on this platform")
}

func (Store) withSupervisorLease(
	func() (Checkpoint, error),
) (Checkpoint, error) {
	return Checkpoint{}, fmt.Errorf("agent-host restart store is unsupported on this platform")
}
