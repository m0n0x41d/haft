//go:build !darwin && !linux

package codeintel

import "fmt"

type codeIndexLease interface {
	release() error
}

func tryAcquireCodeIndexLease(
	directory string,
	name string,
) (codeIndexLease, bool, error) {
	return nil, false, fmt.Errorf(
		"code-index rebuild lease is unsupported for %s",
		directory,
	)
}
