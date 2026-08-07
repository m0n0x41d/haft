//go:build !darwin && !linux

package profileprojection

import "fmt"

type projectionDirectory struct{}

func openProjectionDirectory(projectRoot string) (*projectionDirectory, error) {
	return nil, fmt.Errorf(
		"identity-held no-follow profile projection is unsupported on this platform for %s",
		projectRoot,
	)
}

func (directory *projectionDirectory) Close() error {
	return nil
}

func (directory *projectionDirectory) observe(expected []byte) projectionObservation {
	return unreadableProjectionObservation(
		fmt.Errorf("identity-held no-follow profile projection is unsupported on this platform"),
	)
}

func (directory *projectionDirectory) reconcileStages() error {
	return fmt.Errorf("profile projection stage reconciliation is unsupported on this platform")
}

func (directory *projectionDirectory) writeAtomic(content []byte, temporaryID string) error {
	return fmt.Errorf("atomic no-follow profile projection write is unsupported on this platform")
}
