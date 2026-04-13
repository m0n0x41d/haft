//go:build linux

package cli

import (
	"golang.org/x/sys/unix"
)

func disableEcho(fd int) (restore func(), err error) {
	oldTermios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	modified := *oldTermios
	modified.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &modified); err != nil {
		return nil, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, oldTermios)
	}, nil
}
