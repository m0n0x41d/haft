//go:build !darwin && !linux

package codeintel

import (
	"strings"
	"testing"
)

func TestCodeIndexLeaseUnsupportedPlatformFailsClosed(t *testing.T) {
	lease, acquired, err := tryAcquireCodeIndexLease("runtime", "lease")
	if lease != nil || acquired || err == nil ||
		!strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("lease=%#v acquired=%v err=%v", lease, acquired, err)
	}
}
