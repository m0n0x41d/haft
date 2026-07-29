//go:build darwin || linux

package onboardingfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/onboarding"
)

func TestMemoryDeferralInstallReplayConflictAndExactReopen(
	t *testing.T,
) {
	root := t.TempDir()
	if err := os.Mkdir(
		filepath.Join(root, DirectoryName),
		0o755,
	); err != nil {
		t.Fatal(err)
	}
	first := memoryDeferralFixture(
		t,
		"qnt_a11ce001",
		"a",
		time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC),
	)
	second := memoryDeferralFixture(
		t,
		"qnt_a11ce001",
		"b",
		time.Date(2026, time.July, 28, 12, 1, 0, 0, time.UTC),
	)
	created, err := Install(root, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := created.(Created); !ok {
		t.Fatalf("first install = %T, want Created", created)
	}
	reused, err := Install(root, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reused.(Reused); !ok {
		t.Fatalf("exact replay = %T, want Reused", reused)
	}
	conflict, err := Install(root, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := conflict.(Conflict); !ok {
		t.Fatalf("different install = %T, want Conflict", conflict)
	}
	present, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := present.(Present)
	if !ok ||
		current.Deferral.ReviewDigest() != first.ReviewDigest() {
		t.Fatalf("conflict changed carrier: %#v", present)
	}
	reopenConflict, err := Reopen(root, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopenConflict.(ReopenConflict); !ok {
		t.Fatalf(
			"different reopen = %T, want ReopenConflict",
			reopenConflict,
		)
	}
	reopened, err := Reopen(root, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.(Reopened); !ok {
		t.Fatalf("exact reopen = %T, want Reopened", reopened)
	}
	replay, err := Reopen(root, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := replay.(AlreadyOpen); !ok {
		t.Fatalf("reopen replay = %T, want AlreadyOpen", replay)
	}
	absent, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := absent.(Absent); !ok {
		t.Fatalf("post-reopen read = %T, want Absent", absent)
	}
}

func TestMemoryDeferralReadRejectsSymlinkAndTamperedCarrier(
	t *testing.T,
) {
	root := t.TempDir()
	directory := filepath.Join(root, DirectoryName)
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, FileName)
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); err == nil {
		t.Fatal("symlink carrier was accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); err == nil {
		t.Fatal("tampered carrier was accepted")
	}
}

func TestMemoryDeferralHeldDirectoryRejectsDetachedNamespace(
	t *testing.T,
) {
	root := t.TempDir()
	directoryPath := filepath.Join(root, DirectoryName)
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := openHeldDirectory(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.close()
	detachedPath := filepath.Join(root, ".haft-detached")
	if err := os.Rename(directoryPath, detachedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.readAttached(); err == nil {
		t.Fatal("detached onboarding namespace was accepted")
	}
}

func TestUnixDescriptorBoundaryRejectsUnavailableDescriptors(
	t *testing.T,
) {
	if _, err := checkedFileDescriptor(nil, "test file"); err == nil {
		t.Fatal("nil file descriptor was accepted")
	}
	file, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := checkedFileDescriptor(file, "test file"); err == nil {
		t.Fatal("closed file descriptor was accepted")
	}
	if _, err := newFileFromUnixDescriptor(-1, "test file"); err == nil {
		t.Fatal("negative Unix descriptor was accepted")
	}
}

func memoryDeferralFixture(
	t *testing.T,
	projectID string,
	digestCharacter string,
	recordedAt time.Time,
) onboarding.MemoryDeferral {
	t.Helper()
	deferral, err := onboarding.NewMemoryDeferral(
		onboarding.MemoryDeferralInput{
			ProjectID:    projectID,
			ReviewRef:    onboarding.MemoryReviewRef,
			ReviewDigest: "sha256:" + strings.Repeat(digestCharacter, 64),
			Choice:       onboarding.DeferStructuredMemoryChoice,
			RecordedAt:   recordedAt,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return deferral
}
