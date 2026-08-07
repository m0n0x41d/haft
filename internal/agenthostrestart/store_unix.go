//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"golang.org/x/sys/unix"
)

const (
	RestartDirectoryName  = "restart"
	CheckpointFileName    = "checkpoint.json"
	ignoreFileName        = ".gitignore"
	lockFileName          = ".checkpoint.lock"
	supervisorLockName    = ".supervisor.lock"
	stagePrefix           = ".checkpoint-stage-"
	attemptsDirectoryName = "attempts"
	attemptMarkerPrefix   = "sha256-"
	attemptMarkerSuffix   = ".attempt"
	privateDirectoryMode  = 0o700
	privateFileMode       = 0o600
	maximumMarkerBytes    = 80
)

var restartIgnoreBytes = []byte("*\n!.gitignore\n")

// Store persists one current checkpoint under the project-local,
// gitignored .haft/restart directory.
type Store struct {
	projectRoot           string
	directoryPath         string
	attemptsDirectoryPath string
	checkpointPath        string
}

func NewStore(projectRoot string) (Store, error) {
	absoluteRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return Store{}, fmt.Errorf("resolve restart project root: %w", err)
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	if err := requireDirectoryNoSymlink(absoluteRoot, "project root"); err != nil {
		return Store{}, err
	}
	haftPath := filepath.Join(absoluteRoot, ".haft")
	if err := requireDirectoryNoSymlink(haftPath, ".haft directory"); err != nil {
		return Store{}, err
	}
	directoryPath := filepath.Join(haftPath, RestartDirectoryName)
	return Store{
		projectRoot:           absoluteRoot,
		directoryPath:         directoryPath,
		attemptsDirectoryPath: filepath.Join(directoryPath, attemptsDirectoryName),
		checkpointPath:        filepath.Join(directoryPath, CheckpointFileName),
	}, nil
}

func (store Store) CheckpointPath() string { return store.checkpointPath }

func (store Store) SupervisorLogPath(restartID string) (string, error) {
	if !exactToken.MatchString(restartID) {
		return "", fmt.Errorf("restart_id is invalid")
	}
	return filepath.Join(store.directoryPath, restartID+".log"), nil
}

// Prepare writes a new attempt only when no active handoff exists and the
// desired digest has never been consumed by this store.
func (store Store) Prepare(checkpoint Checkpoint) error {
	if err := checkpoint.validate(); err != nil {
		return err
	}
	if checkpoint.state != StatePrepared || checkpoint.attempt != checkpointAttempt {
		return fmt.Errorf("%w: only a prepared attempt=1 checkpoint can be installed", ErrInvalidTransition)
	}
	if checkpoint.repositoryRoot != store.projectRoot {
		return fmt.Errorf("restart checkpoint belongs to another repository")
	}
	expectedLogPath, err := store.SupervisorLogPath(checkpoint.restartID)
	if err != nil {
		return err
	}
	if checkpoint.supervisorLogPath != expectedLogPath {
		return fmt.Errorf("restart checkpoint supervisor log path is not store-owned")
	}
	return store.withExclusiveLock(func() error {
		current, loadErr := store.loadUnlocked()
		if loadErr != nil && !errors.Is(loadErr, ErrCheckpointNotFound) {
			return loadErr
		}
		if loadErr == nil {
			if err := admitPreparation(current, checkpoint); err != nil {
				return err
			}
		}
		attempts, err := store.openAttemptsDirectory()
		if err != nil {
			return err
		}
		defer attempts.Close()
		if loadErr == nil {
			if err := ensureAttemptMarker(attempts, current.desiredHaftBinaryDigest); err != nil {
				return err
			}
		}
		if err := reserveAttemptMarker(attempts, checkpoint.desiredHaftBinaryDigest); err != nil {
			return err
		}
		return store.writeUnlocked(checkpoint)
	})
}

// Apply is an exact-byte compare-and-swap for one closed state transition.
// Replays are idempotent except for the resumed-turn single-writer claim.
func (store Store) Apply(change Change) error {
	if !change.valid() {
		return fmt.Errorf("%w: proposed durable transition is not legal", ErrInvalidTransition)
	}
	expected := change.before
	proposed := change.after
	if err := expected.validate(); err != nil {
		return err
	}
	if err := proposed.validate(); err != nil {
		return err
	}
	return store.withExclusiveLock(func() error {
		current, err := store.loadUnlocked()
		if err != nil {
			return err
		}
		currentBytes, err := current.CanonicalBytes()
		if err != nil {
			return err
		}
		proposedBytes, err := proposed.CanonicalBytes()
		if err != nil {
			return err
		}
		if bytes.Equal(currentBytes, proposedBytes) {
			if expected.state == StateAppOpened && proposed.state == StateResumed {
				return ErrDuplicateResumeWriter
			}
			return nil
		}
		expectedBytes, err := expected.CanonicalBytes()
		if err != nil {
			return err
		}
		if !bytes.Equal(currentBytes, expectedBytes) {
			return ErrConcurrentUpdate
		}
		return store.writeUnlocked(proposed)
	})
}

func (store Store) Load() (Checkpoint, error) {
	if err := store.requireExistingDirectory(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Checkpoint{}, ErrCheckpointNotFound
		}
		return Checkpoint{}, err
	}
	lock, err := store.openExistingLock()
	if err != nil {
		return Checkpoint{}, err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_SH); err != nil { // #nosec G115 -- lock is an open file returned by the restart store.
		return Checkpoint{}, fmt.Errorf("lock restart checkpoint for read: %w", err)
	}
	defer func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) // #nosec G115 -- lock remains open until this deferred unlock runs.
	}()
	return store.loadUnlocked()
}

// OpenSupervisorLog opens only the store-owned log path from the exact
// checkpoint. The log remains inside the gitignored restart directory.
func (store Store) OpenSupervisorLog(checkpoint Checkpoint) (*os.File, error) {
	if err := store.requireExistingDirectory(); err != nil {
		return nil, err
	}
	if checkpoint.repositoryRoot != store.projectRoot {
		return nil, fmt.Errorf("restart checkpoint belongs to another repository")
	}
	expectedPath, err := store.SupervisorLogPath(checkpoint.restartID)
	if err != nil {
		return nil, err
	}
	if checkpoint.supervisorLogPath != expectedPath {
		return nil, fmt.Errorf("restart checkpoint supervisor log path is not store-owned")
	}
	flags := unix.O_WRONLY | unix.O_APPEND | unix.O_CREAT | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(expectedPath, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open restart supervisor log: %w", err)
	}
	file := os.NewFile(uintptr(fd), expectedPath) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if err := requirePrivateRegularFile(file, "restart supervisor log"); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (store Store) withSupervisorLease(
	effect func() (Checkpoint, error),
) (Checkpoint, error) {
	if err := store.requireExistingDirectory(); err != nil {
		return Checkpoint{}, err
	}
	path := filepath.Join(store.directoryPath, supervisorLockName)
	flags := unix.O_RDWR | unix.O_CREAT | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0o600)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("open restart supervisor lease: %w", err)
	}
	lease := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	defer lease.Close()
	if err := requirePrivateRegularFile(lease, "restart supervisor lease"); err != nil {
		return Checkpoint{}, err
	}
	if err := unix.Flock(int(lease.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil { // #nosec G115 -- lease wraps the descriptor returned by unix.Open.
		if errors.Is(err, unix.EWOULDBLOCK) {
			return Checkpoint{}, ErrDuplicateSupervisor
		}
		return Checkpoint{}, fmt.Errorf("acquire restart supervisor lease: %w", err)
	}
	defer func() {
		_ = unix.Flock(int(lease.Fd()), unix.LOCK_UN) // #nosec G115 -- lease remains open until this deferred unlock runs.
	}()
	return effect()
}

func admitPreparation(current Checkpoint, proposed Checkpoint) error {
	switch current.state {
	case StatePrepared, StateSubmitted, StateAppOpened, StateResumed:
		return fmt.Errorf("%w: restart %s remains %s", ErrLoopGuard, current.restartID, current.state.String())
	case StateInstallFailed:
		if current.desiredHaftBinaryDigest == proposed.desiredHaftBinaryDigest {
			return fmt.Errorf("%w: desired digest already consumed attempt=1", ErrLoopGuard)
		}
		return nil
	case StateVerified:
		if current.desiredHaftBinaryDigest == proposed.desiredHaftBinaryDigest {
			return ErrAlreadyVerified
		}
		return nil
	default:
		return fmt.Errorf("%w: current checkpoint state is invalid", ErrLoopGuard)
	}
}

func (store Store) withExclusiveLock(effect func() error) error {
	if err := store.ensureDirectory(); err != nil {
		return err
	}
	lock, err := store.openLock()
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil { // #nosec G115 -- lock is an open file returned by the restart store.
		return fmt.Errorf("lock restart checkpoint for write: %w", err)
	}
	defer func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN) // #nosec G115 -- lock remains open until this deferred unlock runs.
	}()
	return effect()
}

func (store Store) ensureDirectory() error {
	if err := requireDirectoryNoSymlink(store.projectRoot, "project root"); err != nil {
		return err
	}
	haftPath := filepath.Join(store.projectRoot, ".haft")
	if err := requireDirectoryNoSymlink(haftPath, ".haft directory"); err != nil {
		return err
	}
	if err := os.Mkdir(store.directoryPath, privateDirectoryMode); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create restart checkpoint directory: %w", err)
	}
	if err := requirePrivateDirectoryNoSymlink(store.directoryPath, "restart checkpoint directory"); err != nil {
		return err
	}
	return store.ensureIgnoreFile()
}

func (store Store) requireExistingDirectory() error {
	if err := requireDirectoryNoSymlink(store.projectRoot, "project root"); err != nil {
		return err
	}
	haftPath := filepath.Join(store.projectRoot, ".haft")
	if err := requireDirectoryNoSymlink(haftPath, ".haft directory"); err != nil {
		return err
	}
	if err := requirePrivateDirectoryNoSymlink(store.directoryPath, "restart checkpoint directory"); err != nil {
		return err
	}
	content, err := readRegularNoFollow(
		filepath.Join(store.directoryPath, ignoreFileName),
		MaximumCheckpointBytes,
	)
	if err != nil {
		return err
	}
	if !ignoreProtectsRestartFiles(content) {
		return fmt.Errorf("restart .gitignore does not ignore checkpoint and log files")
	}
	return nil
}

func (store Store) ensureIgnoreFile() error {
	path := filepath.Join(store.directoryPath, ignoreFileName)
	content, err := readRegularNoFollow(path, MaximumCheckpointBytes)
	if err == nil {
		if !ignoreProtectsRestartFiles(content) {
			return fmt.Errorf("restart .gitignore does not ignore checkpoint and log files")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, openErr := unix.Open(path, flags, 0o644)
	if openErr != nil {
		if errors.Is(openErr, unix.EEXIST) {
			return store.ensureIgnoreFile()
		}
		return fmt.Errorf("create restart .gitignore: %w", openErr)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	writeErr := writeAndSync(file, restartIgnoreBytes)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return syncDirectory(store.directoryPath)
}

func (store Store) openAttemptsDirectory() (*os.File, error) {
	err := os.Mkdir(store.attemptsDirectoryPath, privateDirectoryMode)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create restart attempts directory: %w", err)
	}
	if err := syncDirectory(store.directoryPath); err != nil {
		return nil, fmt.Errorf("sync restart directory after attempts directory: %w", err)
	}
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(store.attemptsDirectoryPath, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open restart attempts directory without following symlinks: %w", err)
	}
	directory := os.NewFile(uintptr(fd), store.attemptsDirectoryPath) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if err := requirePrivateDirectory(directory, "restart attempts directory"); err != nil {
		_ = directory.Close()
		return nil, err
	}
	return directory, nil
}

func ensureAttemptMarker(directory *os.File, desiredDigest string) error {
	err := createAttemptMarker(directory, desiredDigest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	return validateAttemptMarker(directory, desiredDigest)
}

func reserveAttemptMarker(directory *os.File, desiredDigest string) error {
	err := createAttemptMarker(directory, desiredDigest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := validateAttemptMarker(directory, desiredDigest); err != nil {
		return err
	}
	return fmt.Errorf("%w: desired digest already consumed attempt=1", ErrLoopGuard)
}

func createAttemptMarker(directory *os.File, desiredDigest string) error {
	name, content, err := attemptMarker(desiredDigest)
	if err != nil {
		return err
	}
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(int(directory.Fd()), name, flags, privateFileMode) // #nosec G115 -- directory wraps the descriptor returned by unix.Open.
	if err != nil {
		if errors.Is(err, unix.EEXIST) {
			return os.ErrExist
		}
		return fmt.Errorf("reserve restart attempt marker: %w", err)
	}
	path := filepath.Join(directory.Name(), name)
	marker := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
	if err := marker.Chmod(privateFileMode); err != nil {
		_ = marker.Close()
		return fmt.Errorf("set restart attempt marker mode: %w", err)
	}
	if err := requirePrivateRegularFile(marker, "restart attempt marker"); err != nil {
		_ = marker.Close()
		return err
	}
	writeErr := writeAndSync(marker, content)
	closeErr := marker.Close()
	if writeErr != nil {
		return fmt.Errorf("write restart attempt marker: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close restart attempt marker: %w", closeErr)
	}
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync restart attempts directory: %w", err)
	}
	return nil
}

func validateAttemptMarker(directory *os.File, desiredDigest string) error {
	name, expected, err := attemptMarker(desiredDigest)
	if err != nil {
		return err
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(int(directory.Fd()), name, flags, 0) // #nosec G115 -- directory wraps the descriptor returned by unix.Open.
	if err != nil {
		return fmt.Errorf("open existing restart attempt marker without following symlinks: %w", err)
	}
	path := filepath.Join(directory.Name(), name)
	marker := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
	defer marker.Close()
	if err := requirePrivateRegularFile(marker, "restart attempt marker"); err != nil {
		return err
	}
	reader := io.LimitReader(marker, maximumMarkerBytes+1)
	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read restart attempt marker: %w", err)
	}
	if len(content) > maximumMarkerBytes {
		return fmt.Errorf("restart attempt marker exceeds %d bytes", maximumMarkerBytes)
	}
	if !bytes.Equal(content, expected) {
		return fmt.Errorf("restart attempt marker does not match desired digest")
	}
	return nil
}

func attemptMarker(desiredDigest string) (string, []byte, error) {
	if !exactSHA256Digest.MatchString(desiredDigest) {
		return "", nil, fmt.Errorf("restart attempt marker digest is invalid")
	}
	hexDigest := strings.TrimPrefix(desiredDigest, "sha256:")
	name := attemptMarkerPrefix + hexDigest + attemptMarkerSuffix
	content := []byte(desiredDigest + "\n")
	return name, content, nil
}

func ignoreProtectsRestartFiles(content []byte) bool {
	lines := strings.Split(string(content), "\n")
	matcher := ignore.CompileIgnoreLines(lines...)
	return matcher.MatchesPath(CheckpointFileName) && matcher.MatchesPath("restart.log")
}

func (store Store) openLock() (*os.File, error) {
	path := filepath.Join(store.directoryPath, lockFileName)
	flags := unix.O_RDWR | unix.O_CREAT | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open restart checkpoint lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if err := requirePrivateRegularFile(file, "restart checkpoint lock"); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (store Store) openExistingLock() (*os.File, error) {
	path := filepath.Join(store.directoryPath, lockFileName)
	flags := unix.O_RDWR | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open existing restart checkpoint lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if err := requirePrivateRegularFile(file, "restart checkpoint lock"); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (store Store) loadUnlocked() (Checkpoint, error) {
	content, err := readPrivateRegularNoFollow(
		store.checkpointPath,
		MaximumCheckpointBytes,
		"restart checkpoint",
	)
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	if err != nil {
		return Checkpoint{}, err
	}
	checkpoint, err := DecodeCheckpoint(content)
	if err != nil {
		return Checkpoint{}, err
	}
	if checkpoint.repositoryRoot != store.projectRoot {
		return Checkpoint{}, fmt.Errorf("stored restart checkpoint belongs to another repository")
	}
	return checkpoint, nil
}

func (store Store) writeUnlocked(checkpoint Checkpoint) error {
	content, err := checkpoint.CanonicalBytes()
	if err != nil {
		return err
	}
	stage, err := os.CreateTemp(store.directoryPath, stagePrefix)
	if err != nil {
		return fmt.Errorf("create restart checkpoint stage: %w", err)
	}
	stagePath := stage.Name()
	stagePresent := true
	defer func() {
		if stagePresent {
			_ = os.Remove(stagePath)
		}
	}()
	if err := stage.Chmod(privateFileMode); err != nil {
		_ = stage.Close()
		return err
	}
	writeErr := writeAndSync(stage, content)
	closeErr := stage.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(stagePath, store.checkpointPath); err != nil {
		return fmt.Errorf("atomically install restart checkpoint: %w", err)
	}
	stagePresent = false
	if err := syncDirectory(store.directoryPath); err != nil {
		return err
	}
	reread, err := readPrivateRegularNoFollow(
		store.checkpointPath,
		MaximumCheckpointBytes,
		"restart checkpoint",
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(reread, content) {
		return fmt.Errorf("restart checkpoint reread differs from written bytes")
	}
	return nil
}

func requireDirectoryNoSymlink(path string, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a non-symlink directory", label)
	}
	return nil
}

func requirePrivateDirectoryNoSymlink(path string, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a non-symlink directory", label)
	}
	if info.Mode().Perm() != privateDirectoryMode {
		return fmt.Errorf("%s mode is %04o, want %04o", label, info.Mode().Perm(), privateDirectoryMode)
	}
	return nil
}

func requirePrivateDirectory(directory *os.File, label string) error {
	info, err := directory.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", label)
	}
	if info.Mode().Perm() != privateDirectoryMode {
		return fmt.Errorf("%s mode is %04o, want %04o", label, info.Mode().Perm(), privateDirectoryMode)
	}
	return nil
}

func readRegularNoFollow(path string, maximum int) ([]byte, error) {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("open restart file without following symlinks: %w", err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	defer file.Close()
	if err := requireRegularFile(file, "restart file"); err != nil {
		return nil, err
	}
	reader := io.LimitReader(file, int64(maximum+1))
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(content) > maximum {
		return nil, fmt.Errorf("restart file exceeds %d bytes", maximum)
	}
	return content, nil
}

func readPrivateRegularNoFollow(
	path string,
	maximum int,
	label string,
) ([]byte, error) {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("open %s without following symlinks: %w", label, err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	defer file.Close()
	if err := requirePrivateRegularFile(file, label); err != nil {
		return nil, err
	}
	reader := io.LimitReader(file, int64(maximum+1))
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(content) > maximum {
		return nil, fmt.Errorf("%s exceeds %d bytes", label, maximum)
	}
	return content, nil
}

func requireRegularFile(file *os.File, label string) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", label)
	}
	return nil
}

func requirePrivateRegularFile(file *os.File, label string) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", label)
	}
	if info.Mode().Perm() != privateFileMode {
		return fmt.Errorf("%s mode is %04o, want %04o", label, info.Mode().Perm(), privateFileMode)
	}
	return nil
}

func writeAndSync(file *os.File, content []byte) error {
	written, err := file.Write(content)
	if err != nil {
		return err
	}
	if written != len(content) {
		return io.ErrShortWrite
	}
	return file.Sync()
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
