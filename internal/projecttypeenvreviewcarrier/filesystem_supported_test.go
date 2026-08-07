//go:build darwin || linux

package projecttypeenvreviewcarrier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestInstallCreatesRereadsAndExactlyReusesCarrier(t *testing.T) {
	root := newProjectRoot(t)
	proposed := mustCarrier(t, []byte("{\"review\":\"first\"}\n"))
	result, err := Install(root, proposed)
	if err != nil {
		t.Fatalf("install carrier: %v", err)
	}
	created, ok := result.(Created)
	if !ok {
		t.Fatalf("install result = %T, want Created", result)
	}
	assertCarrierBytes(t, created.Carrier, proposed.Bytes())
	installed, err := Read(root)
	if err != nil {
		t.Fatalf("read installed carrier: %v", err)
	}
	assertCarrierBytes(t, installed, proposed.Bytes())
	result, err = Install(root, proposed)
	if err != nil {
		t.Fatalf("reuse carrier: %v", err)
	}
	reused, ok := result.(Reused)
	if !ok {
		t.Fatalf("reuse result = %T, want Reused", result)
	}
	assertCarrierBytes(t, reused.Carrier, proposed.Bytes())
	info, err := os.Stat(filepath.Join(root, DirectoryName, FileName))
	if err != nil {
		t.Fatalf("stat installed carrier: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("installed permissions = %o, want 600", info.Mode().Perm())
	}
	stages, err := filepath.Glob(
		filepath.Join(root, DirectoryName, stagePrefix+"*"),
	)
	if err != nil {
		t.Fatalf("glob stages: %v", err)
	}
	if len(stages) != 0 {
		t.Fatalf("stages remain after installation: %v", stages)
	}
}

func TestInstallRetainsDifferentExistingCarrierAsConflict(t *testing.T) {
	root := newProjectRoot(t)
	current := mustCarrier(t, []byte("{\"review\":\"current\"}\n"))
	proposed := mustCarrier(t, []byte("{\"review\":\"proposed\"}\n"))
	if _, err := Install(root, current); err != nil {
		t.Fatalf("install current carrier: %v", err)
	}
	result, err := Install(root, proposed)
	if err != nil {
		t.Fatalf("install conflicting carrier: %v", err)
	}
	conflict, ok := result.(Conflict)
	if !ok {
		t.Fatalf("conflict result = %T, want Conflict", result)
	}
	if _, ok := conflict.Expectation.(MustBeAbsent); !ok {
		t.Fatalf(
			"conflict expectation = %T, want MustBeAbsent",
			conflict.Expectation,
		)
	}
	present, ok := conflict.Current.(Present)
	if !ok {
		t.Fatalf("conflict current = %T, want Present", conflict.Current)
	}
	assertCarrierBytes(t, present.Carrier, current.Bytes())
	assertInstalledBytes(t, root, current.Bytes())
}

func TestReplaceRequiresExpectedDigestAndRetainsMismatch(t *testing.T) {
	root := newProjectRoot(t)
	current := mustCarrier(t, []byte("{\"review\":\"current\"}\n"))
	unrelated := mustCarrier(t, []byte("{\"review\":\"unrelated\"}\n"))
	proposed := mustCarrier(t, []byte("{\"review\":\"proposed\"}\n"))
	if _, err := Install(root, current); err != nil {
		t.Fatalf("install current carrier: %v", err)
	}
	result, err := Replace(root, unrelated.Digest(), proposed)
	if err != nil {
		t.Fatalf("replace mismatched carrier: %v", err)
	}
	conflict, ok := result.(Conflict)
	if !ok {
		t.Fatalf("mismatch result = %T, want Conflict", result)
	}
	expected, ok := conflict.Expectation.(MustMatch)
	if !ok {
		t.Fatalf("mismatch expectation = %T, want MustMatch", conflict.Expectation)
	}
	if expected.Digest != unrelated.Digest() {
		t.Fatalf(
			"expected digest = %s, want %s",
			expected.Digest,
			unrelated.Digest(),
		)
	}
	assertInstalledBytes(t, root, current.Bytes())
	result, err = Replace(root, current.Digest(), proposed)
	if err != nil {
		t.Fatalf("replace matching carrier: %v", err)
	}
	created, ok := result.(Created)
	if !ok {
		t.Fatalf("matching result = %T, want Created", result)
	}
	assertCarrierBytes(t, created.Carrier, proposed.Bytes())
	assertInstalledBytes(t, root, proposed.Bytes())
}

func TestReplaceReusesAlreadyInstalledProposedBytes(t *testing.T) {
	root := newProjectRoot(t)
	old := mustCarrier(t, []byte("{\"review\":\"old\"}\n"))
	proposed := mustCarrier(t, []byte("{\"review\":\"proposed\"}\n"))
	if _, err := Install(root, proposed); err != nil {
		t.Fatalf("install proposed carrier: %v", err)
	}
	result, err := Replace(root, old.Digest(), proposed)
	if err != nil {
		t.Fatalf("retry replacement: %v", err)
	}
	if _, ok := result.(Reused); !ok {
		t.Fatalf("retry result = %T, want Reused", result)
	}
}

func TestExactReuseSynchronizesDirectoryBeforeReturning(t *testing.T) {
	root := newProjectRoot(t)
	proposed := mustCarrier(t, []byte("{\"review\":\"proposed\"}\n"))
	if _, err := Install(root, proposed); err != nil {
		t.Fatalf("install proposed carrier: %v", err)
	}
	directory := openLockedMutationDirectory(t, root)
	syncCalls := 0
	directory.postLinearization.syncDirectory = func(file *os.File) error {
		syncCalls++
		return file.Sync()
	}
	result, err := directory.installAbsent(proposed)
	directory.unlock()
	if closeErr := directory.Close(); closeErr != nil {
		t.Fatalf("close exact-reuse directory: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("reuse exact carrier: %v", err)
	}
	if _, ok := result.(Reused); !ok {
		t.Fatalf("reuse result = %T, want Reused", result)
	}
	if syncCalls != 1 {
		t.Fatalf("reuse directory sync calls = %d, want 1", syncCalls)
	}
}

func TestReplaceMissingCarrierReturnsTypedConflict(t *testing.T) {
	root := newProjectRoot(t)
	expected := mustCarrier(t, []byte("{\"review\":\"old\"}\n"))
	proposed := mustCarrier(t, []byte("{\"review\":\"proposed\"}\n"))
	result, err := Replace(root, expected.Digest(), proposed)
	if err != nil {
		t.Fatalf("replace missing carrier: %v", err)
	}
	conflict, ok := result.(Conflict)
	if !ok {
		t.Fatalf("missing result = %T, want Conflict", result)
	}
	if _, ok := conflict.Current.(Missing); !ok {
		t.Fatalf("missing current = %T, want Missing", conflict.Current)
	}
}

func TestInstallReturnsOutcomeUnknownAndSameProposalRetryConverges(
	t *testing.T,
) {
	tests := []struct {
		name            string
		inject          func(*heldDirectory)
		expectedCleanup CleanupDisposition
	}{
		{
			name: "directory fsync failure",
			inject: func(directory *heldDirectory) {
				directory.postLinearization.syncDirectory = func(*os.File) error {
					return errors.New("injected post-link fsync failure")
				}
			},
			expectedCleanup: PossibleOrphanStage{},
		},
		{
			name: "installed carrier reread failure",
			inject: func(directory *heldDirectory) {
				directory.postLinearization.rereadTarget = func(
					*heldDirectory,
				) (observedTarget, error) {
					return observedTarget{},
						errors.New("injected post-link reread failure")
				}
			},
			expectedCleanup: NoKnownCleanupDebt{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newProjectRoot(t)
			proposed := mustCarrier(
				t,
				[]byte("{\"review\":\"possibly-installed\"}\n"),
			)
			result, err := runFaultedInstall(
				t,
				root,
				proposed,
				test.inject,
			)
			if err != nil {
				t.Fatalf("faulted install returned bare error: %v", err)
			}
			assertOutcomeUnknown(
				t,
				result,
				proposed,
				ExactInstallRetry{Proposed: proposed.Digest()},
				test.expectedCleanup,
			)
			assertInstalledBytes(t, root, proposed.Bytes())
			retry, err := Install(root, proposed)
			if err != nil {
				t.Fatalf("retry exact proposal: %v", err)
			}
			if _, ok := retry.(Reused); !ok {
				t.Fatalf("retry result = %T, want Reused", retry)
			}
		})
	}
}

func TestReplaceReturnsOutcomeUnknownAndSameProposalRetryConverges(
	t *testing.T,
) {
	tests := []struct {
		name            string
		inject          func(*heldDirectory)
		expectedCleanup CleanupDisposition
	}{
		{
			name: "directory fsync failure",
			inject: func(directory *heldDirectory) {
				directory.postLinearization.syncDirectory = func(*os.File) error {
					return errors.New("injected post-rename fsync failure")
				}
			},
			expectedCleanup: PossibleOrphanStage{},
		},
		{
			name: "installed carrier reread failure",
			inject: func(directory *heldDirectory) {
				directory.postLinearization.rereadTarget = func(
					*heldDirectory,
				) (observedTarget, error) {
					return observedTarget{},
						errors.New("injected post-rename reread failure")
				}
			},
			expectedCleanup: NoKnownCleanupDebt{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newProjectRoot(t)
			current := mustCarrier(
				t,
				[]byte("{\"review\":\"current\"}\n"),
			)
			proposed := mustCarrier(
				t,
				[]byte("{\"review\":\"possibly-replaced\"}\n"),
			)
			if _, err := Install(root, current); err != nil {
				t.Fatalf("install current carrier: %v", err)
			}
			result, err := runFaultedReplace(
				t,
				root,
				current.Digest(),
				proposed,
				test.inject,
			)
			if err != nil {
				t.Fatalf("faulted replace returned bare error: %v", err)
			}
			assertOutcomeUnknown(
				t,
				result,
				proposed,
				ExactReplaceRetry{
					Expected: current.Digest(),
					Proposed: proposed.Digest(),
				},
				test.expectedCleanup,
			)
			assertInstalledBytes(t, root, proposed.Bytes())
			retry, err := Replace(root, current.Digest(), proposed)
			if err != nil {
				t.Fatalf("retry exact proposal: %v", err)
			}
			if _, ok := retry.(Reused); !ok {
				t.Fatalf("retry result = %T, want Reused", retry)
			}
		})
	}
}

func TestInstallUnlinkFailureReportsStageDebtAndRetryCleansIt(
	t *testing.T,
) {
	root := newProjectRoot(t)
	proposed := mustCarrier(
		t,
		[]byte("{\"review\":\"linked-with-stage-debt\"}\n"),
	)
	result, err := runFaultedInstall(
		t,
		root,
		proposed,
		func(directory *heldDirectory) {
			directory.postLinearization.unlinkStage = func(int, string) error {
				return errors.New("injected post-link stage cleanup failure")
			}
		},
	)
	if err != nil {
		t.Fatalf("faulted install returned bare error: %v", err)
	}
	assertOutcomeUnknown(
		t,
		result,
		proposed,
		ExactInstallRetry{Proposed: proposed.Digest()},
		PossibleOrphanStage{},
	)
	unknown, ok := result.(OutcomeUnknown)
	if !ok {
		t.Fatalf("faulted result = %T, want OutcomeUnknown", result)
	}
	debt, ok := unknown.Cleanup.(PossibleOrphanStage)
	if !ok {
		t.Fatalf("cleanup = %T, want PossibleOrphanStage", unknown.Cleanup)
	}
	stagePath := filepath.Join(root, DirectoryName, debt.Name)
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("expected non-canonical stage debt: %v", err)
	}
	retry, err := Install(root, proposed)
	if err != nil {
		t.Fatalf("retry exact proposal: %v", err)
	}
	if _, ok := retry.(Reused); !ok {
		t.Fatalf("retry result = %T, want Reused", retry)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("retry retained reconciled stage debt: %v", err)
	}
	assertInstalledBytes(t, root, proposed.Bytes())
}

func TestMutationReconcilesBoundedRegularStageDebt(t *testing.T) {
	root := newProjectRoot(t)
	stagePath := filepath.Join(
		root,
		DirectoryName,
		stagePrefix+"0123456789abcdef0123456789abcdef",
	)
	if err := os.WriteFile(stagePath, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("write stale stage: %v", err)
	}
	decoyPath := filepath.Join(
		root,
		DirectoryName,
		stagePrefix+"not-a-generated-stage",
	)
	if err := os.WriteFile(decoyPath, []byte("decoy\n"), 0o600); err != nil {
		t.Fatalf("write prefix decoy: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"created\"}\n"))
	result, err := Install(root, proposed)
	if err != nil {
		t.Fatalf("install after stale stage: %v", err)
	}
	if _, ok := result.(Created); !ok {
		t.Fatalf("install result = %T, want Created", result)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stale stage remains: %v", err)
	}
	decoy, err := os.ReadFile(decoyPath)
	if err != nil {
		t.Fatalf("read retained prefix decoy: %v", err)
	}
	if string(decoy) != "decoy\n" {
		t.Fatalf("prefix decoy changed to %q", decoy)
	}
}

func TestMutationRejectsExcessStageDebtWithoutTargetEffect(t *testing.T) {
	root := newProjectRoot(t)
	for index := 0; index <= maximumStaleStageDebt; index++ {
		name := fmt.Sprintf("%s%032x", stagePrefix, index)
		path := filepath.Join(root, DirectoryName, name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("write excess stage %d: %v", index, err)
		}
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-install\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation accepted excess stage debt")
	}
	target := filepath.Join(root, DirectoryName, FileName)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target changed despite excess stage debt: %v", err)
	}
	stages, err := filepath.Glob(
		filepath.Join(root, DirectoryName, stagePrefix+"*"),
	)
	if err != nil {
		t.Fatalf("glob retained stages: %v", err)
	}
	if len(stages) != maximumStaleStageDebt+1 {
		t.Fatalf(
			"retained stages = %d, want %d",
			len(stages),
			maximumStaleStageDebt+1,
		)
	}
}

func TestMutationRejectsNonRegularStageDebtWithoutTargetEffect(t *testing.T) {
	root := newProjectRoot(t)
	outside := filepath.Join(t.TempDir(), "outside-stage")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatalf("write outside stage: %v", err)
	}
	stagePath := filepath.Join(
		root,
		DirectoryName,
		stagePrefix+"abcdef0123456789abcdef0123456789",
	)
	if err := os.Symlink(outside, stagePath); err != nil {
		t.Fatalf("symlink stale stage: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-install\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation accepted non-regular stage debt")
	}
	target := filepath.Join(root, DirectoryName, FileName)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target changed despite non-regular stage debt: %v", err)
	}
}

func TestBoundaryRejectsSymlinkedHaftDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, DirectoryName)); err != nil {
		t.Fatalf("symlink .haft: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-escape\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation followed a symlinked .haft directory")
	}
	if _, err := os.Stat(filepath.Join(outside, FileName)); !os.IsNotExist(err) {
		t.Fatalf("writer escaped through symlinked .haft: %v", err)
	}
}

func TestBoundaryRejectsSymlinkedCarrier(t *testing.T) {
	root := newProjectRoot(t)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	target := filepath.Join(root, DirectoryName, FileName)
	if err := os.Symlink(outside, target); err != nil {
		t.Fatalf("symlink carrier: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-follow\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation accepted a symlinked carrier")
	}
	outsideBytes, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(outsideBytes) != "outside\n" {
		t.Fatalf("outside file changed to %q", outsideBytes)
	}
}

func TestBoundaryRejectsNonRegularCarrier(t *testing.T) {
	root := newProjectRoot(t)
	target := filepath.Join(root, DirectoryName, FileName)
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("create directory at carrier path: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-replace-dir\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation accepted a non-regular carrier")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat retained carrier directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("carrier directory was replaced: mode=%v", info.Mode())
	}
}

func TestReadRejectsFIFOWithoutBlocking(t *testing.T) {
	const childMode = "HAFT_GENESIS_REVIEW_FIFO_CHILD"
	const rootVariable = "HAFT_GENESIS_REVIEW_FIFO_ROOT"
	if os.Getenv(childMode) == "1" {
		root := os.Getenv(rootVariable)
		if _, err := Read(root); err == nil {
			t.Fatal("read accepted FIFO carrier")
		}
		return
	}
	root := newProjectRoot(t)
	target := filepath.Join(root, DirectoryName, FileName)
	if err := unix.Mkfifo(target, 0o600); err != nil {
		t.Fatalf("create carrier FIFO: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestReadRejectsFIFOWithoutBlocking$",
	)
	command.Env = append(
		os.Environ(),
		childMode+"=1",
		rootVariable+"="+root,
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("FIFO read blocked until timeout: %v; output=%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("FIFO rejection subprocess failed: %v; output=%s", err, output)
	}
}

func TestReadRejectsFIFOLockWithoutBlocking(t *testing.T) {
	const childMode = "HAFT_GENESIS_REVIEW_FIFO_LOCK_CHILD"
	const rootVariable = "HAFT_GENESIS_REVIEW_FIFO_LOCK_ROOT"
	if os.Getenv(childMode) == "1" {
		root := os.Getenv(rootVariable)
		if _, err := Read(root); err == nil {
			t.Fatal("read accepted FIFO lock")
		}
		return
	}
	root := newProjectRoot(t)
	carrier := mustCarrier(t, []byte("{\"review\":\"current\"}\n"))
	target := filepath.Join(root, DirectoryName, FileName)
	if err := os.WriteFile(target, carrier.Bytes(), 0o600); err != nil {
		t.Fatalf("write carrier: %v", err)
	}
	lockPath := filepath.Join(root, DirectoryName, lockFileName)
	if err := unix.Mkfifo(lockPath, 0o600); err != nil {
		t.Fatalf("create lock FIFO: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestReadRejectsFIFOLockWithoutBlocking$",
	)
	command.Env = append(
		os.Environ(),
		childMode+"=1",
		rootVariable+"="+root,
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf(
			"FIFO lock read blocked until timeout: %v; output=%s",
			ctx.Err(),
			output,
		)
	}
	if err != nil {
		t.Fatalf(
			"FIFO lock rejection subprocess failed: %v; output=%s",
			err,
			output,
		)
	}
}

func TestHeldDirectoryRejectsAttachedPathSwap(t *testing.T) {
	root := newProjectRoot(t)
	directory, err := openHeldDirectoryForMutation(root)
	if err != nil {
		t.Fatalf("open held directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	heldPath := filepath.Join(root, ".haft-held")
	if err := os.Rename(filepath.Join(root, DirectoryName), heldPath); err != nil {
		t.Fatalf("move held .haft: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, DirectoryName)); err != nil {
		t.Fatalf("swap .haft path: %v", err)
	}
	if err := directory.lockExclusive(); err == nil {
		t.Fatal("held directory accepted a swapped .haft path")
	}
	if _, err := os.Stat(filepath.Join(outside, FileName)); !os.IsNotExist(err) {
		t.Fatalf("writer escaped through swapped .haft path: %v", err)
	}
}

func TestCASCheckRejectsCarrierPathSwap(t *testing.T) {
	root := newProjectRoot(t)
	current := mustCarrier(t, []byte("{\"review\":\"current\"}\n"))
	if _, err := Install(root, current); err != nil {
		t.Fatalf("install current carrier: %v", err)
	}
	directory, err := openHeldDirectoryForMutation(root)
	if err != nil {
		t.Fatalf("open held directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	if err := directory.lockExclusive(); err != nil {
		t.Fatalf("lock held directory: %v", err)
	}
	t.Cleanup(directory.unlock)
	observed, err := directory.readTarget()
	if err != nil {
		t.Fatalf("read CAS basis: %v", err)
	}
	target := filepath.Join(root, DirectoryName, FileName)
	displaced := filepath.Join(root, DirectoryName, "displaced-review.json")
	if err := os.Rename(target, displaced); err != nil {
		t.Fatalf("displace reviewed carrier: %v", err)
	}
	if err := os.WriteFile(target, current.Bytes(), 0o600); err != nil {
		t.Fatalf("install same-byte replacement inode: %v", err)
	}
	if err := directory.requireCurrentTarget(observed); err == nil {
		t.Fatal("CAS basis accepted a swapped carrier inode with identical bytes")
	}
}

func TestHeldLockRejectsLockPathSwap(t *testing.T) {
	root := newProjectRoot(t)
	directory, err := openHeldDirectoryForMutation(root)
	if err != nil {
		t.Fatalf("open held directory: %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	if err := directory.lockExclusive(); err != nil {
		t.Fatalf("lock held directory: %v", err)
	}
	t.Cleanup(directory.unlock)
	lockPath := filepath.Join(root, DirectoryName, lockFileName)
	displaced := filepath.Join(root, DirectoryName, "displaced-review.lock")
	if err := os.Rename(lockPath, displaced); err != nil {
		t.Fatalf("displace held lock: %v", err)
	}
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("install replacement lock inode: %v", err)
	}
	if err := directory.requireAttachedIdentity(); err == nil {
		t.Fatal("held directory accepted a swapped lock-file inode")
	}
}

func TestConcurrentInstallNeverSilentlyOverwritesCarrier(t *testing.T) {
	root := newProjectRoot(t)
	left := mustCarrier(t, []byte("{\"review\":\"left\"}\n"))
	right := mustCarrier(t, []byte("{\"review\":\"right\"}\n"))
	type installation struct {
		result InstallationResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan installation, 2)
	var workers sync.WaitGroup
	for _, carrier := range []Carrier{left, right} {
		workers.Add(1)
		go func(candidate Carrier) {
			defer workers.Done()
			<-start
			result, err := Install(root, candidate)
			results <- installation{result: result, err: err}
		}(carrier)
	}
	close(start)
	workers.Wait()
	close(results)
	created := 0
	conflicts := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent install: %v", result.err)
		}
		switch result.result.(type) {
		case Created:
			created++
		case Conflict:
			conflicts++
		default:
			t.Fatalf("concurrent result = %T", result.result)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("created=%d conflicts=%d, want 1 and 1", created, conflicts)
	}
	installed, err := Read(root)
	if err != nil {
		t.Fatalf("read concurrent winner: %v", err)
	}
	if !bytes.Equal(installed.Bytes(), left.Bytes()) &&
		!bytes.Equal(installed.Bytes(), right.Bytes()) {
		t.Fatalf("installed unexpected bytes %q", installed.Bytes())
	}
}

func TestConcurrentReplaceAllowsOnlyOneMatchingCAS(t *testing.T) {
	root := newProjectRoot(t)
	current := mustCarrier(t, []byte("{\"review\":\"current\"}\n"))
	left := mustCarrier(t, []byte("{\"review\":\"left\"}\n"))
	right := mustCarrier(t, []byte("{\"review\":\"right\"}\n"))
	if _, err := Install(root, current); err != nil {
		t.Fatalf("install current carrier: %v", err)
	}
	type installation struct {
		result InstallationResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan installation, 2)
	var workers sync.WaitGroup
	for _, carrier := range []Carrier{left, right} {
		workers.Add(1)
		go func(candidate Carrier) {
			defer workers.Done()
			<-start
			result, err := Replace(root, current.Digest(), candidate)
			results <- installation{result: result, err: err}
		}(carrier)
	}
	close(start)
	workers.Wait()
	close(results)
	created := 0
	conflicts := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent replace: %v", result.err)
		}
		switch result.result.(type) {
		case Created:
			created++
		case Conflict:
			conflicts++
		default:
			t.Fatalf("concurrent replacement result = %T", result.result)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("created=%d conflicts=%d, want 1 and 1", created, conflicts)
	}
}

func TestReadRejectsOversizedCarrier(t *testing.T) {
	root := newProjectRoot(t)
	path := filepath.Join(root, DirectoryName, FileName)
	if err := os.WriteFile(path, make([]byte, MaximumBytes+1), 0o600); err != nil {
		t.Fatalf("write oversized carrier: %v", err)
	}
	if _, err := Read(root); err == nil {
		t.Fatal("oversized carrier was read")
	}
}

func TestReadDoesNotCreateMutationLock(t *testing.T) {
	root := newProjectRoot(t)
	carrier := mustCarrier(t, []byte("{\"review\":\"legacy\"}\n"))
	target := filepath.Join(root, DirectoryName, FileName)
	if err := os.WriteFile(target, carrier.Bytes(), 0o600); err != nil {
		t.Fatalf("write legacy carrier: %v", err)
	}
	observed, err := Read(root)
	if err != nil {
		t.Fatalf("read legacy carrier: %v", err)
	}
	assertCarrierBytes(t, observed, carrier.Bytes())
	lockPath := filepath.Join(root, DirectoryName, lockFileName)
	if _, err := os.Lstat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("read created a mutation lock: %v", err)
	}
}

func TestBoundaryRejectsSymlinkedLockFile(t *testing.T) {
	root := newProjectRoot(t)
	outside := filepath.Join(t.TempDir(), "outside.lock")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatalf("write outside lock: %v", err)
	}
	lockPath := filepath.Join(root, DirectoryName, lockFileName)
	if err := os.Symlink(outside, lockPath); err != nil {
		t.Fatalf("symlink lock: %v", err)
	}
	proposed := mustCarrier(t, []byte("{\"review\":\"must-not-lock\"}\n"))
	if _, err := Install(root, proposed); err == nil {
		t.Fatal("installation accepted a symlinked lock file")
	}
}

func newProjectRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, DirectoryName), 0o755); err != nil {
		t.Fatalf("mkdir .haft: %v", err)
	}
	return root
}

func mustCarrier(t *testing.T, content []byte) Carrier {
	t.Helper()
	carrier, err := NewCarrier(content)
	if err != nil {
		t.Fatalf("new carrier: %v", err)
	}
	return carrier
}

func assertCarrierBytes(t *testing.T, carrier Carrier, expected []byte) {
	t.Helper()
	if !bytes.Equal(carrier.Bytes(), expected) {
		t.Fatalf("carrier bytes = %q, want %q", carrier.Bytes(), expected)
	}
}

func assertInstalledBytes(t *testing.T, root string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(filepath.Join(root, DirectoryName, FileName))
	if err != nil {
		t.Fatalf("read installed bytes: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("installed bytes = %q, want %q", actual, expected)
	}
}

func runFaultedInstall(
	t *testing.T,
	root string,
	proposed Carrier,
	inject func(*heldDirectory),
) (InstallationResult, error) {
	t.Helper()
	directory := openLockedMutationDirectory(t, root)
	inject(directory)
	result, err := directory.installAbsent(proposed)
	directory.unlock()
	if closeErr := directory.Close(); closeErr != nil {
		t.Fatalf("close faulted-install directory: %v", closeErr)
	}
	return result, err
}

func runFaultedReplace(
	t *testing.T,
	root string,
	expected Digest,
	proposed Carrier,
	inject func(*heldDirectory),
) (InstallationResult, error) {
	t.Helper()
	directory := openLockedMutationDirectory(t, root)
	inject(directory)
	result, err := directory.replaceMatching(expected, proposed)
	directory.unlock()
	if closeErr := directory.Close(); closeErr != nil {
		t.Fatalf("close faulted-replace directory: %v", closeErr)
	}
	return result, err
}

func openLockedMutationDirectory(
	t *testing.T,
	root string,
) *heldDirectory {
	t.Helper()
	directory, err := openHeldDirectoryForMutation(root)
	if err != nil {
		t.Fatalf("open mutation directory: %v", err)
	}
	if err := directory.lockExclusive(); err != nil {
		_ = directory.Close()
		t.Fatalf("lock mutation directory: %v", err)
	}
	if err := directory.reconcileStaleStages(); err != nil {
		directory.unlock()
		_ = directory.Close()
		t.Fatalf("reconcile stage debt: %v", err)
	}
	return directory
}

func assertOutcomeUnknown(
	t *testing.T,
	result InstallationResult,
	proposed Carrier,
	expectedRetry ExactSameProposalRetry,
	expectedCleanup CleanupDisposition,
) {
	t.Helper()
	unknown, ok := result.(OutcomeUnknown)
	if !ok {
		t.Fatalf("faulted result = %T, want OutcomeUnknown", result)
	}
	if unknown.Proposed != proposed.Digest() {
		t.Fatalf(
			"unknown proposed digest = %s, want %s",
			unknown.Proposed,
			proposed.Digest(),
		)
	}
	if unknown.Retry.ProposedDigest() != proposed.Digest() {
		t.Fatalf(
			"retry proposed digest = %s, want %s",
			unknown.Retry.ProposedDigest(),
			proposed.Digest(),
		)
	}
	actualRetryType := reflect.TypeOf(unknown.Retry)
	expectedRetryType := reflect.TypeOf(expectedRetry)
	if actualRetryType != expectedRetryType {
		t.Fatalf(
			"unknown retry = %T, want %T",
			unknown.Retry,
			expectedRetry,
		)
	}
	expectedInstruction := expectedRetry.Instruction()
	if unknown.Retry.Instruction() != expectedInstruction {
		t.Fatalf(
			"retry instruction = %q, want %q",
			unknown.Retry.Instruction(),
			expectedInstruction,
		)
	}
	if unknown.Failure == "" {
		t.Fatal("unknown result omitted post-linearization failure")
	}
	actualCleanupType := reflect.TypeOf(unknown.Cleanup)
	expectedCleanupType := reflect.TypeOf(expectedCleanup)
	if actualCleanupType != expectedCleanupType {
		t.Fatalf(
			"unknown cleanup = %T, want %T",
			unknown.Cleanup,
			expectedCleanup,
		)
	}
	if debt, ok := unknown.Cleanup.(PossibleOrphanStage); ok {
		if !validStageName(debt.Name) {
			t.Fatalf(
				"unknown cleanup stage %q is not a generated stage name",
				debt.Name,
			)
		}
	}
}
