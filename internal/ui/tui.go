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
		frame.WriteString("\x1b[2m" + strings.Repeat("─", w) + "\x1b[0m\n")

		// Trim content to fit between tab bar (2 rows) and footer (1 row)
		maxContentLines := h - 3
		if maxContentLines > 0 {
			lines := strings.Split(content, "\n")
			if len(lines) > maxContentLines {
				lines = lines[:maxContentLines]
			}
			frame.WriteString(strings.Join(lines, "\n"))
		}

		footer := " \x1b[2m1-5 switch · \u2190\u2192 navigate · r refresh · q/Esc close\x1b[0m"
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

// Platform-specific terminal helpers are in term_darwin.go and term_linux.go.
