//go:build linux

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

type termState struct {
	termios syscall.Termios
}

func makeRaw(fd uintptr) (*termState, error) {
	var oldState termState
	//nolint:gosec // ioctl requires passing the termios pointer to the kernel
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&oldState.termios)),
		0, 0, 0); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= syscall.ICRNL | syscall.IXON | syscall.IXOFF
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	//nolint:gosec // ioctl requires passing the termios pointer to the kernel
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&newState)),
		0, 0, 0); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

func restore(fd uintptr, state *termState) {
	//nolint:gosec // ioctl requires passing the termios pointer to the kernel
	syscall.Syscall6(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&state.termios)),
		0, 0, 0)
}

func termSize() (int, int) {
	type winsize struct {
		Row, Col       uint16
		Xpixel, Ypixel uint16
	}
	var ws winsize
	//nolint:gosec // ioctl requires passing the winsize pointer to the kernel
	syscall.Syscall(syscall.SYS_IOCTL, os.Stderr.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	w, h := int(ws.Col), int(ws.Row)
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	return w, h
}
