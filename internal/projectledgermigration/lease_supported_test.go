//go:build darwin || linux

package projectledgermigration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSafeMigrationLeaseMetadata(t *testing.T) {
	const owner = uint32(501)
	regularPrivate := uint32(unix.S_IFREG | 0o600)
	if !safeMigrationLeaseMetadata(regularPrivate, 1, owner, owner) {
		t.Fatal("private operator-owned regular lease was rejected")
	}
	for name, candidate := range map[string]struct {
		mode     uint32
		links    uint64
		owner    uint32
		expected uint32
	}{
		"group readable": {regularPrivate | 0o040, 1, owner, owner},
		"hard linked":    {regularPrivate, 2, owner, owner},
		"foreign owner":  {regularPrivate, 1, owner + 1, owner},
		"special bits":   {regularPrivate | unix.S_ISUID, 1, owner, owner},
		"not regular":    {uint32(unix.S_IFDIR | 0o600), 1, owner, owner},
	} {
		t.Run(name, func(t *testing.T) {
			if safeMigrationLeaseMetadata(
				candidate.mode,
				candidate.links,
				candidate.owner,
				candidate.expected,
			) {
				t.Fatal("unsafe migration lease metadata was accepted")
			}
		})
	}
}

func TestEnsureCurrentForServeRejectsUnsafeLeaseCarrier(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
	}{
		{
			name: "symlink",
			prepare: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "lease-target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unsafe mode",
			prepare: func(t *testing.T, path string) {
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, databasePath := newSchema57ServeFixture(t, false)
			leasePath := filepath.Join(
				filepath.Dir(databasePath),
				migrationLeaseFilename,
			)
			test.prepare(t, leasePath)
			result, err := EnsureCurrentForServe(
				context.Background(),
				serveFixtureRequest(t, fixture),
				serveMigrationTestTime,
			)
			if err == nil || result.Blocker != ServeBlockerLeaseUnavailable {
				t.Fatalf("unsafe lease activation = %#v, %v", result, err)
			}
			if frontier := readSchemaFrontierForTest(
				t,
				databasePath,
			); frontier != 57 {
				t.Fatalf("schema with unsafe lease = %d, want 57", frontier)
			}
		})
	}
}
