//go:build darwin || linux

package projecttypeenvreviewcarrier

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	lockFileName          = ".project-typeenv-genesis-review-lock"
	stagePrefix           = ".project-typeenv-genesis-review-stage-"
	maximumStaleStageDebt = 64
)

type heldDirectory struct {
	rootPath          string
	root              *os.File
	rootDescriptor    int
	haft              *os.File
	haftDescriptor    int
	lock              *os.File
	lockDescriptor    int
	postLinearization postLinearizationEffects
}

type observedTarget struct {
	carrier Carrier
	stat    unix.Stat_t
}

type postLinearizationEffects struct {
	unlinkStage   func(int, string) error
	syncDirectory func(*os.File) error
	rereadTarget  func(*heldDirectory) (observedTarget, error)
}

func productionPostLinearizationEffects() postLinearizationEffects {
	return postLinearizationEffects{
		unlinkStage: func(directoryFD int, name string) error {
			return unix.Unlinkat(directoryFD, name, 0)
		},
		syncDirectory: func(directory *os.File) error {
			return directory.Sync()
		},
		rereadTarget: func(directory *heldDirectory) (observedTarget, error) {
			return directory.readTarget()
		},
	}
}

// Install creates the canonical carrier without replacing different bytes.
func Install(projectRoot string, proposed Carrier) (InstallationResult, error) {
	if err := proposed.valid(); err != nil {
		return nil, err
	}
	directory, err := openHeldDirectoryForMutation(projectRoot)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if err := directory.lockExclusive(); err != nil {
		return nil, err
	}
	defer directory.unlock()
	if err := directory.reconcileStaleStages(); err != nil {
		return nil, err
	}
	return directory.installAbsent(proposed)
}

// Replace installs proposed only when the current exact-byte digest matches
// expected. An already installed proposed carrier is an idempotent reuse.
func Replace(
	projectRoot string,
	expected Digest,
	proposed Carrier,
) (InstallationResult, error) {
	if !expected.valid() {
		return nil, fmt.Errorf("expected Genesis review digest is invalid")
	}
	if err := proposed.valid(); err != nil {
		return nil, err
	}
	directory, err := openHeldDirectoryForMutation(projectRoot)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if err := directory.lockExclusive(); err != nil {
		return nil, err
	}
	defer directory.unlock()
	if err := directory.reconcileStaleStages(); err != nil {
		return nil, err
	}
	return directory.replaceMatching(expected, proposed)
}

// Read observes the canonical carrier without creating filesystem entries.
func Read(projectRoot string) (Carrier, error) {
	directory, err := openHeldDirectoryForRead(projectRoot)
	if err != nil {
		return Carrier{}, err
	}
	defer directory.Close()
	if err := directory.lockShared(); err != nil {
		return Carrier{}, err
	}
	defer directory.unlock()
	observed, err := directory.readTarget()
	if err != nil {
		return Carrier{}, err
	}
	return observed.carrier, nil
}

func openHeldDirectoryForMutation(projectRoot string) (*heldDirectory, error) {
	directory, err := openHeldDirectory(projectRoot, openOrCreateLockFileAt)
	if err != nil {
		return nil, err
	}
	if err := directory.haft.Sync(); err != nil {
		_ = directory.Close()
		return nil, fmt.Errorf("sync Genesis review .haft directory: %w", err)
	}
	return directory, nil
}

func openHeldDirectoryForRead(projectRoot string) (*heldDirectory, error) {
	return openHeldDirectory(projectRoot, openExistingLockFileAt)
}

func openHeldDirectory(
	projectRoot string,
	openLock func(int) (*os.File, error),
) (*heldDirectory, error) {
	absoluteRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve Genesis review project root: %w", err)
	}
	root, err := openDirectoryNoFollow(absoluteRoot)
	if err != nil {
		return nil, err
	}
	rootDescriptor, err := checkedFileDescriptor(root, "Genesis review project root")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	haft, err := openDirectoryAt(rootDescriptor, DirectoryName)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("open held Genesis review .haft directory: %w", err)
	}
	haftDescriptor, err := checkedFileDescriptor(haft, "Genesis review .haft directory")
	if err != nil {
		_ = haft.Close()
		_ = root.Close()
		return nil, err
	}
	lock, err := openLock(haftDescriptor)
	if err != nil {
		_ = haft.Close()
		_ = root.Close()
		return nil, err
	}
	lockDescriptor := -1
	if lock != nil {
		lockDescriptor, err = checkedFileDescriptor(lock, "Genesis review lock file")
		if err != nil {
			_ = lock.Close()
			_ = haft.Close()
			_ = root.Close()
			return nil, err
		}
	}
	directory := &heldDirectory{
		rootPath:          absoluteRoot,
		root:              root,
		rootDescriptor:    rootDescriptor,
		haft:              haft,
		haftDescriptor:    haftDescriptor,
		lock:              lock,
		lockDescriptor:    lockDescriptor,
		postLinearization: productionPostLinearizationEffects(),
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		_ = directory.Close()
		return nil, err
	}
	return directory, nil
}

func (directory *heldDirectory) Close() error {
	if directory == nil {
		return nil
	}
	var lockErr error
	if directory.lock != nil {
		lockErr = directory.lock.Close()
		directory.lock = nil
		directory.lockDescriptor = -1
	}
	var haftErr error
	if directory.haft != nil {
		haftErr = directory.haft.Close()
		directory.haft = nil
		directory.haftDescriptor = -1
	}
	var rootErr error
	if directory.root != nil {
		rootErr = directory.root.Close()
		directory.root = nil
		directory.rootDescriptor = -1
	}
	return errors.Join(lockErr, haftErr, rootErr)
}

func (directory *heldDirectory) installAbsent(
	proposed Carrier,
) (InstallationResult, error) {
	if err := directory.requireAttachedIdentity(); err != nil {
		return nil, err
	}
	current, err := directory.readTarget()
	if err == nil {
		return directory.reuseOrConflict(
			current.carrier,
			proposed,
			MustBeAbsent{},
		)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	stage, stageName, err := directory.writeStage(proposed)
	if err != nil {
		return nil, err
	}
	stagePresent := true
	defer func() {
		if stagePresent {
			_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		}
	}()
	if err := stage.Close(); err != nil {
		return nil, fmt.Errorf("close Genesis review stage: %w", err)
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return nil, err
	}
	err = unix.Linkat(
		directory.haftFD(),
		stageName,
		directory.haftFD(),
		FileName,
		0,
	)
	if errors.Is(err, os.ErrExist) {
		current, readErr := directory.readTarget()
		if readErr != nil {
			return nil, readErr
		}
		return directory.reuseOrConflict(
			current.carrier,
			proposed,
			MustBeAbsent{},
		)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"atomically install Genesis review carrier without replacement: %w",
			err,
		)
	}
	err = directory.postLinearization.unlinkStage(
		directory.haftFD(),
		stageName,
	)
	if err != nil {
		stagePresent = false
		return outcomeUnknown(
			proposed,
			fmt.Sprintf(
				"remove installed Genesis review stage link: %v",
				err,
			),
			exactInstallRetry(proposed),
			PossibleOrphanStage{Name: stageName},
		), nil
	}
	stagePresent = false
	return directory.finishInstallation(
		proposed,
		exactInstallRetry(proposed),
		PossibleOrphanStage{Name: stageName},
		NoKnownCleanupDebt{},
	), nil
}

func (directory *heldDirectory) replaceMatching(
	expected Digest,
	proposed Carrier,
) (InstallationResult, error) {
	if err := directory.requireAttachedIdentity(); err != nil {
		return nil, err
	}
	current, err := directory.readTarget()
	if errors.Is(err, os.ErrNotExist) {
		return Conflict{
			Current:     Missing{},
			Proposed:    proposed.Digest(),
			Expectation: MustMatch{Digest: expected},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if bytes.Equal(current.carrier.content, proposed.content) {
		return directory.finishReuse(
			proposed,
			exactReplaceRetry(expected, proposed),
		), nil
	}
	if current.carrier.Digest() != expected {
		return Conflict{
			Current:     Present{Carrier: current.carrier},
			Proposed:    proposed.Digest(),
			Expectation: MustMatch{Digest: expected},
		}, nil
	}
	stage, stageName, err := directory.writeStage(proposed)
	if err != nil {
		return nil, err
	}
	stagePresent := true
	defer func() {
		if stagePresent {
			_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		}
	}()
	if err := stage.Close(); err != nil {
		return nil, fmt.Errorf("close Genesis review stage: %w", err)
	}
	if err := directory.requireCurrentTarget(current); err != nil {
		latest, readErr := directory.readTarget()
		if errors.Is(readErr, os.ErrNotExist) {
			return Conflict{
				Current:     Missing{},
				Proposed:    proposed.Digest(),
				Expectation: MustMatch{Digest: expected},
			}, nil
		}
		if readErr != nil {
			return nil, errors.Join(err, readErr)
		}
		return Conflict{
			Current:     Present{Carrier: latest.carrier},
			Proposed:    proposed.Digest(),
			Expectation: MustMatch{Digest: expected},
		}, nil
	}
	if err := unix.Renameat(
		directory.haftFD(),
		stageName,
		directory.haftFD(),
		FileName,
	); err != nil {
		return nil, fmt.Errorf("atomically replace Genesis review carrier: %w", err)
	}
	stagePresent = false
	return directory.finishInstallation(
		proposed,
		exactReplaceRetry(expected, proposed),
		PossibleOrphanStage{Name: stageName},
		NoKnownCleanupDebt{},
	), nil
}

func (directory *heldDirectory) writeStage(
	carrier Carrier,
) (*os.File, string, error) {
	stageName, err := randomStageName()
	if err != nil {
		return nil, "", err
	}
	stage, err := openExclusiveFileAt(directory.haftFD(), stageName)
	if err != nil {
		return nil, "", err
	}
	written, err := stage.Write(carrier.content)
	if err != nil {
		_ = stage.Close()
		_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		return nil, "", fmt.Errorf("write Genesis review stage: %w", err)
	}
	if written != len(carrier.content) {
		_ = stage.Close()
		_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		return nil, "", io.ErrShortWrite
	}
	if err := stage.Sync(); err != nil {
		_ = stage.Close()
		_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		return nil, "", fmt.Errorf("sync Genesis review stage: %w", err)
	}
	return stage, stageName, nil
}

func (directory *heldDirectory) reconcileStaleStages() error {
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	entries, err := directory.haft.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("enumerate Genesis review stage debt: %w", err)
	}
	stageNames := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !validStageName(name) {
			continue
		}
		var stat unix.Stat_t
		err := unix.Fstatat(
			directory.haftFD(),
			name,
			&stat,
			unix.AT_SYMLINK_NOFOLLOW,
		)
		if err != nil {
			return fmt.Errorf("inspect Genesis review stage debt: %w", err)
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf(
				"genesis review stage debt %s is not a regular file",
				name,
			)
		}
		stageNames = append(stageNames, name)
	}
	if len(stageNames) > maximumStaleStageDebt {
		return fmt.Errorf(
			"genesis review stage debt has %d entries; bounded cleanup permits %d",
			len(stageNames),
			maximumStaleStageDebt,
		)
	}
	for _, name := range stageNames {
		if err := unix.Unlinkat(directory.haftFD(), name, 0); err != nil {
			return fmt.Errorf("remove Genesis review stage debt: %w", err)
		}
	}
	if len(stageNames) == 0 {
		return nil
	}
	if err := directory.haft.Sync(); err != nil {
		return fmt.Errorf("sync Genesis review stage-debt cleanup: %w", err)
	}
	return directory.requireAttachedIdentity()
}

func (directory *heldDirectory) finishInstallation(
	proposed Carrier,
	retry ExactSameProposalRetry,
	cleanupBeforeSync CleanupDisposition,
	cleanupAfterSync CleanupDisposition,
) InstallationResult {
	if err := directory.postLinearization.syncDirectory(directory.haft); err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf("sync Genesis review .haft directory: %v", err),
			retry,
			cleanupBeforeSync,
		)
	}
	installed, err := directory.postLinearization.rereadTarget(directory)
	if err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf("reread Genesis review carrier: %v", err),
			retry,
			cleanupAfterSync,
		)
	}
	if !bytes.Equal(installed.carrier.content, proposed.content) {
		return outcomeUnknown(
			proposed,
			"reread Genesis review carrier differs from installed bytes",
			retry,
			cleanupAfterSync,
		)
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf(
				"revalidate Genesis review directory after installation: %v",
				err,
			),
			retry,
			cleanupAfterSync,
		)
	}
	return Created{Carrier: installed.carrier}
}

func (directory *heldDirectory) finishReuse(
	proposed Carrier,
	retry ExactSameProposalRetry,
) InstallationResult {
	cleanup := NoKnownCleanupDebt{}
	if err := directory.postLinearization.syncDirectory(directory.haft); err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf(
				"sync exact Genesis review reuse directory: %v",
				err,
			),
			retry,
			cleanup,
		)
	}
	installed, err := directory.postLinearization.rereadTarget(directory)
	if err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf("reread exact Genesis review reuse: %v", err),
			retry,
			cleanup,
		)
	}
	if !bytes.Equal(installed.carrier.content, proposed.content) {
		return outcomeUnknown(
			proposed,
			"reread exact Genesis review reuse differs from proposed bytes",
			retry,
			cleanup,
		)
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return outcomeUnknown(
			proposed,
			fmt.Sprintf(
				"revalidate exact Genesis review reuse: %v",
				err,
			),
			retry,
			cleanup,
		)
	}
	return Reused{Carrier: installed.carrier}
}

func outcomeUnknown(
	proposed Carrier,
	failure string,
	retry ExactSameProposalRetry,
	cleanup CleanupDisposition,
) OutcomeUnknown {
	digest := proposed.Digest()
	return OutcomeUnknown{
		Proposed: digest,
		Failure:  failure,
		Retry:    retry,
		Cleanup:  cleanup,
	}
}

func exactInstallRetry(proposed Carrier) ExactInstallRetry {
	return ExactInstallRetry{Proposed: proposed.Digest()}
}

func exactReplaceRetry(
	expected Digest,
	proposed Carrier,
) ExactReplaceRetry {
	return ExactReplaceRetry{
		Expected: expected,
		Proposed: proposed.Digest(),
	}
}

func (directory *heldDirectory) readTarget() (observedTarget, error) {
	if err := directory.requireAttachedIdentity(); err != nil {
		return observedTarget{}, err
	}
	file, stat, err := openReadOnlyRegularFileAt(directory.haftFD(), FileName)
	if err != nil {
		return observedTarget{}, err
	}
	defer file.Close()
	content, err := readBounded(file, stat)
	if err != nil {
		return observedTarget{}, err
	}
	if err := directory.requireTargetIdentity(file, stat); err != nil {
		return observedTarget{}, err
	}
	carrier, err := NewCarrier(content)
	if err != nil {
		return observedTarget{}, err
	}
	return observedTarget{
		carrier: carrier,
		stat:    stat,
	}, nil
}

func (directory *heldDirectory) requireCurrentTarget(
	current observedTarget,
) error {
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	var attached unix.Stat_t
	err := unix.Fstatat(
		directory.haftFD(),
		FileName,
		&attached,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return err
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("genesis review carrier is not a regular file")
	}
	if !sameIdentity(current.stat, attached) {
		return fmt.Errorf("genesis review carrier changed during CAS replacement")
	}
	return nil
}

func (directory *heldDirectory) requireTargetIdentity(
	file *os.File,
	opened unix.Stat_t,
) error {
	fileDescriptor, err := checkedFileDescriptor(file, "Genesis review carrier")
	if err != nil {
		return err
	}
	var held unix.Stat_t
	if err := unix.Fstat(fileDescriptor, &held); err != nil {
		return fmt.Errorf("inspect held Genesis review carrier: %w", err)
	}
	var attached unix.Stat_t
	err = unix.Fstatat(
		directory.haftFD(),
		FileName,
		&attached,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return fmt.Errorf("inspect attached Genesis review carrier: %w", err)
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("genesis review carrier is not a regular file")
	}
	if !sameIdentity(opened, held) || !sameIdentity(held, attached) {
		return fmt.Errorf("genesis review carrier identity changed while reading")
	}
	return nil
}

func (directory *heldDirectory) requireAttachedIdentity() error {
	if err := directory.valid(); err != nil {
		return err
	}
	var heldRoot unix.Stat_t
	if err := unix.Fstat(directory.rootDescriptor, &heldRoot); err != nil {
		return fmt.Errorf("inspect held Genesis review project root: %w", err)
	}
	var pathRoot unix.Stat_t
	if err := unix.Lstat(directory.rootPath, &pathRoot); err != nil {
		return fmt.Errorf("inspect Genesis review project-root path: %w", err)
	}
	if pathRoot.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("genesis review project root is not a real directory")
	}
	if !sameIdentity(heldRoot, pathRoot) {
		return fmt.Errorf("genesis review project-root path identity changed")
	}
	var heldHaft unix.Stat_t
	if err := unix.Fstat(directory.haftFD(), &heldHaft); err != nil {
		return fmt.Errorf("inspect held Genesis review .haft directory: %w", err)
	}
	var attachedHaft unix.Stat_t
	err := unix.Fstatat(
		directory.rootDescriptor,
		DirectoryName,
		&attachedHaft,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return fmt.Errorf("inspect attached Genesis review .haft directory: %w", err)
	}
	if attachedHaft.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("genesis review .haft entry is not a real directory")
	}
	if !sameIdentity(heldHaft, attachedHaft) {
		return fmt.Errorf("genesis review .haft directory identity changed")
	}
	if err := directory.requireAttachedLockIdentity(); err != nil {
		return err
	}
	return nil
}

func (directory *heldDirectory) requireAttachedLockIdentity() error {
	if directory.lock == nil {
		return nil
	}
	var heldLock unix.Stat_t
	if err := unix.Fstat(directory.lockDescriptor, &heldLock); err != nil {
		return fmt.Errorf("inspect held Genesis review lock file: %w", err)
	}
	if heldLock.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("held Genesis review lock is not a regular file")
	}
	var attachedLock unix.Stat_t
	err := unix.Fstatat(
		directory.haftFD(),
		lockFileName,
		&attachedLock,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return fmt.Errorf("inspect attached Genesis review lock file: %w", err)
	}
	if attachedLock.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("attached Genesis review lock is not a regular file")
	}
	if !sameIdentity(heldLock, attachedLock) {
		return fmt.Errorf("genesis review lock-file identity changed")
	}
	return nil
}

func (directory *heldDirectory) valid() error {
	if directory == nil ||
		directory.rootPath == "" ||
		directory.root == nil ||
		directory.rootDescriptor < 0 ||
		directory.haft == nil ||
		directory.haftDescriptor < 0 ||
		(directory.lock != nil && directory.lockDescriptor < 0) {
		return fmt.Errorf("genesis review directory capability is invalid")
	}
	return nil
}

func (directory *heldDirectory) haftFD() int {
	return directory.haftDescriptor
}

func (directory *heldDirectory) lockExclusive() error {
	if directory.lock == nil {
		return fmt.Errorf("genesis review mutation lock is unavailable")
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	if err := unix.Flock(directory.lockDescriptor, unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock Genesis review carrier exclusively: %w", err)
	}
	return directory.requireAttachedIdentity()
}

func (directory *heldDirectory) lockShared() error {
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	if directory.lock == nil {
		return nil
	}
	if err := unix.Flock(directory.lockDescriptor, unix.LOCK_SH); err != nil {
		return fmt.Errorf("lock Genesis review carrier for reading: %w", err)
	}
	return directory.requireAttachedIdentity()
}

func (directory *heldDirectory) unlock() {
	if directory == nil || directory.lock == nil {
		return
	}
	_ = unix.Flock(directory.lockDescriptor, unix.LOCK_UN)
}

func (directory *heldDirectory) reuseOrConflict(
	current Carrier,
	proposed Carrier,
	expectation Expectation,
) (InstallationResult, error) {
	if bytes.Equal(current.content, proposed.content) {
		return directory.finishReuse(
			proposed,
			exactInstallRetry(proposed),
		), nil
	}
	return Conflict{
		Current:     Present{Carrier: current},
		Proposed:    proposed.Digest(),
		Expectation: expectation,
	}, nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf(
			"open Genesis review project root without following symlinks: %w",
			err,
		)
	}
	return newFileFromUnixDescriptor(fd, path)
}

func openDirectoryAt(parentFD int, name string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return nil, err
	}
	return newFileFromUnixDescriptor(fd, name)
}

func openOrCreateLockFileAt(parentFD int) (*os.File, error) {
	file, err := openExistingRegularLockAt(parentFD, unix.O_RDWR)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	createFlags := unix.O_RDWR |
		unix.O_CREAT |
		unix.O_EXCL |
		unix.O_CLOEXEC |
		unix.O_NOFOLLOW |
		unix.O_NONBLOCK
	fd, err := unix.Openat(parentFD, lockFileName, createFlags, 0o600)
	if errors.Is(err, os.ErrExist) {
		return openExistingRegularLockAt(parentFD, unix.O_RDWR)
	}
	if err != nil {
		return nil, fmt.Errorf("create Genesis review lock file: %w", err)
	}
	created, err := newFileFromUnixDescriptor(fd, lockFileName)
	if err != nil {
		return nil, err
	}
	if err := requireOpenedRegularLock(parentFD, created); err != nil {
		_ = created.Close()
		return nil, err
	}
	return created, nil
}

func openExistingLockFileAt(parentFD int) (*os.File, error) {
	file, err := openExistingRegularLockAt(parentFD, unix.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return file, err
}

func openExistingRegularLockAt(
	parentFD int,
	accessFlags int,
) (*os.File, error) {
	var entry unix.Stat_t
	err := unix.Fstatat(
		parentFD,
		lockFileName,
		&entry,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return nil, err
	}
	if entry.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("genesis review lock entry is not a regular file")
	}
	flags := accessFlags | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	fd, err := unix.Openat(parentFD, lockFileName, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open existing Genesis review lock file: %w", err)
	}
	file, err := newFileFromUnixDescriptor(fd, lockFileName)
	if err != nil {
		return nil, err
	}
	if err := requireOpenedRegularLock(parentFD, file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func requireOpenedRegularLock(parentFD int, file *os.File) error {
	fileDescriptor, err := checkedFileDescriptor(file, "Genesis review lock file")
	if err != nil {
		return err
	}
	var held unix.Stat_t
	if err := unix.Fstat(fileDescriptor, &held); err != nil {
		return fmt.Errorf("inspect held Genesis review lock file: %w", err)
	}
	if held.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("held Genesis review lock is not a regular file")
	}
	var attached unix.Stat_t
	err = unix.Fstatat(
		parentFD,
		lockFileName,
		&attached,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return fmt.Errorf("inspect attached Genesis review lock file: %w", err)
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("attached Genesis review lock is not a regular file")
	}
	if !sameIdentity(held, attached) {
		return fmt.Errorf("genesis review lock-file identity changed while opening")
	}
	return nil
}

func openExclusiveFileAt(parentFD int, name string) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(parentFD, name, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create exclusive Genesis review stage: %w", err)
	}
	return newFileFromUnixDescriptor(fd, name)
}

func openReadOnlyRegularFileAt(
	parentFD int,
	name string,
) (*os.File, unix.Stat_t, error) {
	var entry unix.Stat_t
	err := unix.Fstatat(parentFD, name, &entry, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return nil, unix.Stat_t{}, err
	}
	if entry.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, unix.Stat_t{}, fmt.Errorf(
			"genesis review carrier is not a regular file",
		)
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return nil, unix.Stat_t{}, err
	}
	file, err := newFileFromUnixDescriptor(fd, name)
	if err != nil {
		return nil, unix.Stat_t{}, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = file.Close()
		return nil, unix.Stat_t{}, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = file.Close()
		return nil, unix.Stat_t{}, fmt.Errorf(
			"genesis review carrier is not a regular file",
		)
	}
	return file, stat, nil
}

func checkedFileDescriptor(file *os.File, subject string) (int, error) {
	if file == nil {
		return 0, fmt.Errorf("%s descriptor is unavailable", subject)
	}
	descriptor := file.Fd()
	if descriptor == ^uintptr(0) {
		return 0, fmt.Errorf("%s descriptor is closed", subject)
	}
	if descriptor > uintptr(math.MaxInt) {
		return 0, fmt.Errorf("%s descriptor exceeds the platform int range", subject)
	}
	return int(descriptor), nil
}

func newFileFromUnixDescriptor(descriptor int, name string) (*os.File, error) {
	if descriptor < 0 {
		return nil, fmt.Errorf("unix descriptor for %s is negative", name)
	}
	file := os.NewFile(uintptr(descriptor), name)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("adopt unix descriptor for %s", name)
	}
	return file, nil
}

func readBounded(file *os.File, stat unix.Stat_t) ([]byte, error) {
	if stat.Size > MaximumBytes {
		return nil, fmt.Errorf(
			"genesis review carrier exceeds %d bytes",
			MaximumBytes,
		)
	}
	content, err := io.ReadAll(io.LimitReader(file, MaximumBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Genesis review carrier: %w", err)
	}
	if len(content) > MaximumBytes {
		return nil, fmt.Errorf(
			"genesis review carrier exceeds %d bytes",
			MaximumBytes,
		)
	}
	return content, nil
}

func randomStageName() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate Genesis review stage name: %w", err)
	}
	return stagePrefix + hex.EncodeToString(random[:]), nil
}

func validStageName(name string) bool {
	suffix, found := strings.CutPrefix(name, stagePrefix)
	if !found || len(suffix) != 32 {
		return false
	}
	for _, character := range []byte(suffix) {
		decimal := character >= '0' && character <= '9'
		lowerHex := character >= 'a' && character <= 'f'
		if !decimal && !lowerHex {
			return false
		}
	}
	return true
}

func sameIdentity(left unix.Stat_t, right unix.Stat_t) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino
}
