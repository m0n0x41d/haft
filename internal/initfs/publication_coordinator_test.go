package initfs

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	publicationCoordinatorHelperEnv      = "HAFT_INITFS_COORDINATOR_HELPER"
	publicationCoordinatorRootEnv        = "HAFT_INITFS_COORDINATOR_ROOT"
	publicationCoordinatorLockEnv        = "HAFT_INITFS_COORDINATOR_LOCK"
	publicationCoordinatorProjectRootEnv = "HAFT_INITFS_COORDINATOR_PROJECT_ROOT"
	publicationCoordinatorSharedRootEnv  = "HAFT_INITFS_COORDINATOR_SHARED_ROOT"
)

func TestPublicationCoordinatorSerializesSharedRootAcrossProcesses(
	t *testing.T,
) {
	if os.Getenv(publicationCoordinatorHelperEnv) == "1" {
		runPublicationCoordinatorHelper(t)
		return
	}

	coordinationRoot := t.TempDir()
	lockPath := filepath.Join(
		coordinationRoot,
		"host-publication",
		"publication.lock",
	)
	leftProjectRoot := filepath.Join(t.TempDir(), "left-project")
	rightProjectRoot := filepath.Join(t.TempDir(), "right-project")
	sharedUserRoot := filepath.Join(t.TempDir(), "user-skills")

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestPublicationCoordinatorSerializesSharedRootAcrossProcesses$",
	)
	command.Env = append(
		os.Environ(),
		publicationCoordinatorHelperEnv+"=1",
		publicationCoordinatorRootEnv+"="+coordinationRoot,
		publicationCoordinatorLockEnv+"="+lockPath,
		publicationCoordinatorProjectRootEnv+"="+leftProjectRoot,
		publicationCoordinatorSharedRootEnv+"="+sharedUserRoot,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("open helper stdin: %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("open helper stdout: %v", err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start coordination helper: %v", err)
	}
	waited := false
	defer func() {
		_ = stdin.Close()
		if waited {
			return
		}
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()

	childDigest, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read helper acquisition receipt: %v", err)
	}
	childDigest = strings.TrimSpace(childDigest)
	if !validSHA256Digest(childDigest) {
		t.Fatalf("helper resource digest = %q", childDigest)
	}

	coordinator, err := NewPublicationCoordinator(
		coordinationRoot,
		lockPath,
	)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator parent: %v", err)
	}
	parentResources := []string{rightProjectRoot, sharedUserRoot}
	contended, err := coordinator.TryAcquire(parentResources)
	if err != nil {
		t.Fatalf("TryAcquire cross-process contention: %v", err)
	}
	if contended.Kind() != PublicationCoordinationBusy {
		t.Fatalf(
			"cross-process coordination kind = %s",
			contended.Kind(),
		)
	}
	if contended.ResourceDigest() == childDigest {
		t.Fatal("different project resource sets share one digest")
	}

	if _, err := stdin.Write([]byte{'\n'}); err != nil {
		t.Fatalf("release helper lease: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close helper stdin: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("coordination helper: %v", err)
	}
	waited = true

	retry, err := coordinator.TryAcquire(parentResources)
	if err != nil {
		t.Fatalf("TryAcquire after helper release: %v", err)
	}
	lease, acquired := retry.Lease()
	if !acquired {
		t.Fatal("parent did not acquire released cross-process coordinator")
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("release parent coordination lease: %v", err)
	}
}

func runPublicationCoordinatorHelper(t *testing.T) {
	t.Helper()
	coordinator, err := NewPublicationCoordinator(
		os.Getenv(publicationCoordinatorRootEnv),
		os.Getenv(publicationCoordinatorLockEnv),
	)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator helper: %v", err)
	}
	resources := []string{
		os.Getenv(publicationCoordinatorProjectRootEnv),
		os.Getenv(publicationCoordinatorSharedRootEnv),
	}
	attempt, err := coordinator.TryAcquire(resources)
	if err != nil {
		t.Fatalf("TryAcquire helper: %v", err)
	}
	lease, acquired := attempt.Lease()
	if !acquired {
		t.Fatal("helper did not acquire publication coordinator")
	}
	if _, err := fmt.Fprintln(os.Stdout, attempt.ResourceDigest()); err != nil {
		t.Fatalf("publish helper acquisition receipt: %v", err)
	}
	if _, err := bufio.NewReader(os.Stdin).ReadByte(); err != nil {
		t.Fatalf("wait for helper release: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("release helper coordination lease: %v", err)
	}
}

func TestPublicationCoordinatorSerializesDifferentResourceSets(
	t *testing.T,
) {
	root := t.TempDir()
	lockPath := filepath.Join(
		root,
		".haft",
		"host-installations",
		"publication.lock",
	)
	coordinator, err := NewPublicationCoordinator(root, lockPath)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator: %v", err)
	}
	leftResources := []string{
		filepath.Join(root, ".agents"),
		filepath.Join(root, ".haft", "haft.db"),
	}
	first, err := coordinator.TryAcquire(leftResources)
	if err != nil {
		t.Fatalf("TryAcquire first: %v", err)
	}
	lease, acquired := first.Lease()
	if first.Kind() != PublicationCoordinationAcquired ||
		!acquired ||
		lease == nil ||
		first.LockPath() != lockPath ||
		first.ResourceDigest() == "" {
		t.Fatalf("first coordination attempt = %#v", first)
	}
	if !slices.Equal(
		first.Resources(),
		[]string{leftResources[0], leftResources[1]},
	) {
		t.Fatalf("canonical resources = %v", first.Resources())
	}
	second, err := coordinator.TryAcquire(
		[]string{filepath.Join(root, ".codex")},
	)
	if err != nil {
		t.Fatalf("TryAcquire contended: %v", err)
	}
	if second.Kind() != PublicationCoordinationBusy {
		t.Fatalf("contended coordination kind = %s", second.Kind())
	}
	if _, acquired := second.Lease(); acquired {
		t.Fatal("contended coordination returned a lease")
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("release coordination lease: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("idempotent coordination release: %v", err)
	}
	third, err := coordinator.TryAcquire(
		[]string{filepath.Join(root, ".codex")},
	)
	if err != nil {
		t.Fatalf("TryAcquire after release: %v", err)
	}
	next, acquired := third.Lease()
	if !acquired {
		t.Fatal("coordination lock was not reacquired")
	}
	if err := next.Release(); err != nil {
		t.Fatalf("release reacquired coordination: %v", err)
	}
	info, err := os.Lstat(lockPath)
	if err != nil {
		t.Fatalf("inspect coordination lock: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("coordination lock mode = %s", info.Mode())
	}
}

func TestPublicationCoordinatorResourceIdentityIsOrderIndependent(
	t *testing.T,
) {
	root := t.TempDir()
	coordinator, err := NewPublicationCoordinator(
		root,
		filepath.Join(root, "locks", "publication.lock"),
	)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator: %v", err)
	}
	left := []string{
		filepath.Join(root, "z"),
		filepath.Join(root, "a"),
		filepath.Join(root, "z"),
	}
	right := []string{
		filepath.Join(root, "a"),
		filepath.Join(root, "z"),
	}
	first, err := coordinator.TryAcquire(left)
	if err != nil {
		t.Fatalf("TryAcquire left: %v", err)
	}
	firstLease, acquired := first.Lease()
	if !acquired {
		t.Fatal("left resource set did not acquire the coordinator")
	}
	if err := firstLease.Release(); err != nil {
		t.Fatalf("release left resource set: %v", err)
	}
	second, err := coordinator.TryAcquire(right)
	if err != nil {
		t.Fatalf("TryAcquire right: %v", err)
	}
	secondLease, acquired := second.Lease()
	if !acquired {
		t.Fatal("right resource set did not acquire the coordinator")
	}
	if first.ResourceDigest() != second.ResourceDigest() ||
		!slices.Equal(first.Resources(), second.Resources()) {
		t.Fatal("resource order or duplication changed coordination identity")
	}
	if err := secondLease.Release(); err != nil {
		t.Fatalf("release right resource set: %v", err)
	}
}

func TestPublicationCoordinatorRejectsUnsafeCarrierAndResources(
	t *testing.T,
) {
	root := t.TempDir()
	lockPath := filepath.Join(root, "locks", "publication.lock")
	coordinator, err := NewPublicationCoordinator(root, lockPath)
	if err != nil {
		t.Fatalf("NewPublicationCoordinator: %v", err)
	}
	if _, err := coordinator.TryAcquire(nil); err == nil {
		t.Fatal("empty publication resource set was accepted")
	}
	if _, err := coordinator.TryAcquire([]string{"relative"}); err == nil {
		t.Fatal("relative publication resource was accepted")
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("create coordination parent: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside lock target: %v", err)
	}
	if err := os.Symlink(outside, lockPath); err != nil {
		t.Fatalf("create coordination lock symlink: %v", err)
	}
	if _, err := coordinator.TryAcquire(
		[]string{filepath.Join(root, ".agents")},
	); err == nil {
		t.Fatal("symlinked coordination lock was accepted")
	}
	content, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside target: %v", err)
	}
	if string(content) != "outside" {
		t.Fatalf("outside target changed to %q", content)
	}
}
