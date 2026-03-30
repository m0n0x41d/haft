package tui

import (
	"net/http"
	"time"
)

// httpClient returns a shared HTTP client with timeout for TUI verifications.
func httpClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// newHTTPRequest wraps http.NewRequest, returning nil on error.
func newHTTPRequest(method, url string) (*http.Request, error) {
	return http.NewRequest(method, url, nil)
}
