//go:build darwin || linux

package codeintel

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestSafeCodeIndexLeaseMetadata(t *testing.T) {
	const owner = uint32(501)
	regularPrivate := uint32(unix.S_IFREG | 0o600)
	if !safeCodeIndexLeaseMetadata(regularPrivate, 1, owner, owner) {
		t.Fatal("private operator-owned regular file was rejected")
	}
	for name, candidate := range map[string]struct {
		mode     uint32
		links    uint64
		owner    uint32
		expected uint32
	}{
		"unsafe mode":   {regularPrivate | 0o044, 1, owner, owner},
		"foreign owner": {regularPrivate, 1, owner + 1, owner},
		"hard link":     {regularPrivate, 2, owner, owner},
		"special bits":  {regularPrivate | unix.S_ISUID, 1, owner, owner},
		"not regular":   {uint32(unix.S_IFDIR | 0o600), 1, owner, owner},
	} {
		t.Run(name, func(t *testing.T) {
			if safeCodeIndexLeaseMetadata(
				candidate.mode,
				candidate.links,
				candidate.owner,
				candidate.expected,
			) {
				t.Fatalf("unsafe metadata accepted: %#v", candidate)
			}
		})
	}
}
