//go:build !darwin && !linux

package initfs

import "fmt"

type manifestLock struct{}

func tryAcquireManifestLock(
	path string,
) (*manifestLock, bool, error) {
	return nil, false, fmt.Errorf(
		"manifest publication lock is unsupported for %s",
		path,
	)
}

func (lock *manifestLock) release() error {
	return nil
}
