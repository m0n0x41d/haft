//go:build darwin

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

type termState struct {
	termios syscall.Termios
}

func makeRaw(fd int) (*termState, error) {
	var oldState termState
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TIOCGETA), uintptr(unsafe.Pointer(&oldState.termios)),
		0, 0, 0); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= syscall.ICRNL | syscall.IXON | syscall.IXOFF
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TIOCSETA), uintptr(unsafe.Pointer(&newState)),
		0, 0, 0); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

func restore(fd int, state *termState) {
	syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TIOCSETA), uintptr(unsafe.Pointer(&state.termios)),
		0, 0, 0)
}

func termSize() (int, int) {
	type winsize struct {
		Row, Col       uint16
		Xpixel, Ypixel uint16
	}
	var ws winsize
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stderr.Fd()),
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
