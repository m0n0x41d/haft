package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

// CopyToClipboard copies text via both OSC 52 (terminal escape, works over
// SSH/tmux) and native clipboard for maximum compatibility.
func CopyToClipboard(text string) tea.Cmd {
	return tea.Batch(
		tea.SetClipboard(text),
		func() tea.Msg {
			_ = clipboard.WriteAll(text)
			return nil
		},
	)
}
