//go:build !(darwin || linux || freebsd || netbsd || openbsd || dragonfly)

package store

import (
	"fmt"
	"os"
)

func tryLock(_ *os.File, _ bool) (bool, error) {
	return false, fmt.Errorf("writer_lock_unsupported: no qualified OS lock implementation on this platform")
}
func unlock(_ *os.File) error { return nil }
