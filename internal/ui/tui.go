// Interactive terminal dashboard for haft board.
// Lightweight alt-screen TUI with tab switching, no external dependencies.
package ui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// ViewRenderer returns formatted ANSI text for a named view at the given terminal width.
type ViewRenderer func(viewIndex int, width int) string

var ViewNames = [5]string{"Overview", "Decisions", "Problems", "Coverage", "Evidence"}

// RunInteractive launches the full-screen interactive dashboard.
// Blocks until the user presses q or Ctrl+C.
// renderView(0..4) returns the ANSI content for that view.
// refresh() reloads data (called on 'r' and auto-timer).
func RunInteractive(renderView ViewRenderer, refresh func() error) error {
	// Enter alt-screen + hide cursor
	os.Stderr.WriteString("\x1b[?1049h\x1b[?25l")
	defer func() {
		os.Stderr.WriteString("\x1b[?25h\x1b[?1049l")
	}()

	// Open /dev/tty for direct keyboard input (works even when stdin is piped)
	ttyFile, err := os.Open("/dev/tty")
	if err != nil {
		// Fallback to stdin if /dev/tty is not available
		ttyFile = os.Stdin
	} else {
		defer ttyFile.Close()
	}

	// Set tty to raw mode
	oldState, err := makeRaw(int(ttyFile.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer restore(int(ttyFile.Fd()), oldState)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	current := 0

	render := func() {
		w, h := termSize()
		content := renderView(current, w)

		var frame strings.Builder
		frame.WriteString("\x1b[H\x1b[2J")

		// Tab bar
		for i, name := range ViewNames {
			if i == current {
				frame.WriteString(fmt.Sprintf(" \x1b[1m\x1b[36m[%d %s]\x1b[0m", i+1, name))
			} else {
				frame.WriteString(fmt.Sprintf(" \x1b[2m %d %s \x1b[0m", i+1, name))
			}
		}
		frame.WriteString("\n")
		frame.WriteString("\x1b[2m" + strings.Repeat("─", min(w, 80)) + "\x1b[0m\n")

		frame.WriteString(content)

		footer := " \x1b[2m1-5 switch view · r refresh · q quit\x1b[0m"
		frame.WriteString(fmt.Sprintf("\x1b[%d;1H%s", h, footer))

		os.Stderr.WriteString(frame.String())
	}

	render()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	inputCh := make(chan byte, 16)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := ttyFile.Read(buf)
			if n > 0 {
				inputCh <- buf[0]
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case b := <-inputCh:
			switch b {
			case 'q', 'Q', 3, 27: // q, Ctrl+C, or Escape
				return nil
			case '1':
				current = 0
				render()
			case '2':
				current = 1
				render()
			case '3':
				current = 2
				render()
			case '4':
				current = 3
				render()
			case '5':
				current = 4
				render()
			case 'r', 'R':
				_ = refresh()
				render()
			}
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGWINCH:
				render()
			case syscall.SIGINT, syscall.SIGTERM:
				return nil
			}
		case <-ticker.C:
			_ = refresh()
			render()
		}
	}
}

// --- Raw terminal helpers ---

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
