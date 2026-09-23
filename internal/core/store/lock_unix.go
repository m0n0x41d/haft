//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package store

import (
	"os"
	"syscall"
)

func tryLock(f *os.File, write bool) (bool, error) {
	mode := syscall.LOCK_SH
	if write {
		mode = syscall.LOCK_EX
	}
	err := syscall.Flock(int(f.Fd()), mode|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
		return false, nil
	}
	return err == nil, err
}
func unlock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
