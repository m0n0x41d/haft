//go:build linux

package cli

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func disableEcho(fd uintptr) (restore func(), err error) {
	fdInt, err := fileDescriptorInt(fd)
	if err != nil {
		return nil, err
	}

	oldTermios, err := unix.IoctlGetTermios(fdInt, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	modified := *oldTermios
	modified.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	if err := unix.IoctlSetTermios(fdInt, unix.TCSETS, &modified); err != nil {
		return nil, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fdInt, unix.TCSETS, oldTermios)
	}, nil
}

func fileDescriptorInt(fd uintptr) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if fd > uintptr(maxInt) {
		return 0, fmt.Errorf("file descriptor %d exceeds int range", fd)
	}

	return int(fd), nil
}
