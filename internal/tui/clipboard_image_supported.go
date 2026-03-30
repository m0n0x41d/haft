//go:build (linux || darwin || windows) && !arm && !386 && !ios && !android

package tui

import nativeclipboard "github.com/aymanbagabas/go-nativeclipboard"

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		return nativeclipboard.Text.Read()
	case clipboardFormatImage:
		return nativeclipboard.Image.Read()
	default:
		return nil, errClipboardUnknownFormat
	}
}
