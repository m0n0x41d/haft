//go:build !((linux || darwin || windows) && !arm && !386 && !ios && !android)

package tui

func readClipboard(clipboardFormat) ([]byte, error) {
	return nil, errClipboardPlatformUnsupported
}
