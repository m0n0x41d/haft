//go:build darwin || linux

package onboardingfs

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

	"github.com/m0n0x41d/haft/internal/onboarding"
	"golang.org/x/sys/unix"
)

const (
	lockFileName = ".onboarding-memory-deferral-lock"
	stagePrefix  = ".onboarding-memory-deferral-stage-"
)

type heldDirectory struct {
	root     *os.File
	haft     *os.File
	lock     *os.File
	haftStat unix.Stat_t
}

func Read(projectRoot string) (ReadResult, error) {
	directory, err := openHeldDirectory(projectRoot, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = directory.close()
	}()
	return directory.readAttached()
}

func Install(
	projectRoot string,
	proposed onboarding.MemoryDeferral,
) (InstallResult, error) {
	proposedBytes, err := proposed.CanonicalJSON()
	if err != nil {
		return nil, err
	}
	directory, err := openHeldDirectory(projectRoot, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = directory.close()
	}()
	if err := directory.lockExclusive(); err != nil {
		return nil, err
	}
	defer directory.unlock()
	current, err := directory.readAttached()
	if err != nil {
		return nil, err
	}
	switch value := current.(type) {
	case Present:
		currentBytes, encodeErr := value.Deferral.CanonicalJSON()
		if encodeErr != nil {
			return nil, encodeErr
		}
		if bytes.Equal(currentBytes, proposedBytes) {
			return Reused(value), nil
		}
		return Conflict{
			Current:  value.Deferral,
			Proposed: proposed,
		}, nil
	case Absent:
		return directory.installAbsent(proposed, proposedBytes)
	default:
		return nil, fmt.Errorf(
			"memory deferral read returned an unsupported state",
		)
	}
}

func Reopen(
	projectRoot string,
	expected onboarding.MemoryDeferral,
) (ReopenResult, error) {
	expectedBytes, err := expected.CanonicalJSON()
	if err != nil {
		return nil, err
	}
	directory, err := openHeldDirectory(projectRoot, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = directory.close()
	}()
	if err := directory.lockExclusive(); err != nil {
		return nil, err
	}
	defer directory.unlock()
	current, err := directory.readAttached()
	if err != nil {
		return nil, err
	}
	switch value := current.(type) {
	case Absent:
		return AlreadyOpen{}, nil
	case Present:
		currentBytes, encodeErr := value.Deferral.CanonicalJSON()
		if encodeErr != nil {
			return nil, encodeErr
		}
		if !bytes.Equal(currentBytes, expectedBytes) {
			return ReopenConflict{
				Current:  value.Deferral,
				Expected: expected,
			}, nil
		}
	default:
		return nil, fmt.Errorf(
			"memory deferral read returned an unsupported state",
		)
	}
	haftDescriptor, err := checkedFileDescriptor(
		directory.haft,
		"onboarding state directory",
	)
	if err != nil {
		return nil, err
	}
	if err := unix.Unlinkat(
		haftDescriptor,
		FileName,
		0,
	); err != nil {
		return nil, fmt.Errorf(
			"remove exact memory deferral carrier: %w",
			err,
		)
	}
	if err := directory.haft.Sync(); err != nil {
		return ReopenOutcomeUnknown{
			Expected: expected,
			Failure:  err.Error(),
		}, nil
	}
	observed, err := directory.readAttached()
	if errors.Is(err, os.ErrNotExist) {
		return Reopened{Deferral: expected}, nil
	}
	if err != nil {
		return ReopenOutcomeUnknown{
			Expected: expected,
			Failure:  err.Error(),
		}, nil
	}
	if _, absent := observed.(Absent); absent {
		return Reopened{Deferral: expected}, nil
	}
	return ReopenOutcomeUnknown{
		Expected: expected,
		Failure:  "carrier remained after exact reopen",
	}, nil
}

func (directory *heldDirectory) installAbsent(
	proposed onboarding.MemoryDeferral,
	proposedBytes []byte,
) (InstallResult, error) {
	haftDescriptor, err := checkedFileDescriptor(
		directory.haft,
		"onboarding state directory",
	)
	if err != nil {
		return nil, err
	}
	stage, stageName, err := directory.writeStage(
		haftDescriptor,
		proposedBytes,
	)
	if err != nil {
		return nil, err
	}
	stagePresent := true
	defer func() {
		if stagePresent {
			_ = unix.Unlinkat(
				haftDescriptor,
				stageName,
				0,
			)
		}
	}()
	if err := stage.Close(); err != nil {
		return nil, fmt.Errorf(
			"close memory deferral stage: %w",
			err,
		)
	}
	if err := directory.requireAttached(); err != nil {
		return nil, err
	}
	err = unix.Linkat(
		haftDescriptor,
		stageName,
		haftDescriptor,
		FileName,
		0,
	)
	if errors.Is(err, os.ErrExist) {
		current, readErr := directory.readAttached()
		if readErr != nil {
			return nil, readErr
		}
		present, ok := current.(Present)
		if !ok {
			return nil, fmt.Errorf(
				"memory deferral target appeared without readable bytes",
			)
		}
		currentBytes, encodeErr := present.Deferral.CanonicalJSON()
		if encodeErr != nil {
			return nil, encodeErr
		}
		if bytes.Equal(currentBytes, proposedBytes) {
			return Reused(present), nil
		}
		return Conflict{
			Current:  present.Deferral,
			Proposed: proposed,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf(
			"atomically install memory deferral carrier: %w",
			err,
		)
	}
	if err := unix.Unlinkat(
		haftDescriptor,
		stageName,
		0,
	); err != nil {
		stagePresent = false
		return OutcomeUnknown{
			Proposed: proposed,
			Failure:  err.Error(),
		}, nil
	}
	stagePresent = false
	if err := directory.haft.Sync(); err != nil {
		return OutcomeUnknown{
			Proposed: proposed,
			Failure:  err.Error(),
		}, nil
	}
	observed, err := directory.readAttached()
	if err != nil {
		return OutcomeUnknown{
			Proposed: proposed,
			Failure:  err.Error(),
		}, nil
	}
	present, ok := observed.(Present)
	if !ok {
		return OutcomeUnknown{
			Proposed: proposed,
			Failure:  "installed carrier was absent on exact reread",
		}, nil
	}
	observedBytes, err := present.Deferral.CanonicalJSON()
	if err != nil || !bytes.Equal(observedBytes, proposedBytes) {
		return OutcomeUnknown{
			Proposed: proposed,
			Failure:  "installed carrier differed on exact reread",
		}, nil
	}
	return Created(present), nil
}

func (directory *heldDirectory) writeStage(
	haftDescriptor int,
	content []byte,
) (*os.File, string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, "", fmt.Errorf(
			"create memory deferral stage name: %w",
			err,
		)
	}
	name := stagePrefix + hex.EncodeToString(random)
	fd, err := unix.Openat(
		haftDescriptor,
		name,
		unix.O_WRONLY|
			unix.O_CREAT|
			unix.O_EXCL|
			unix.O_CLOEXEC|
			unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, "", fmt.Errorf(
			"create memory deferral stage: %w",
			err,
		)
	}
	stage, err := newFileFromUnixDescriptor(fd, name)
	if err != nil {
		_ = unix.Unlinkat(haftDescriptor, name, 0)
		return nil, "", fmt.Errorf(
			"adopt memory deferral stage: %w",
			err,
		)
	}
	written, err := stage.Write(content)
	if err != nil {
		_ = stage.Close()
		_ = unix.Unlinkat(haftDescriptor, name, 0)
		return nil, "", fmt.Errorf(
			"write memory deferral stage: %w",
			err,
		)
	}
	if written != len(content) {
		_ = stage.Close()
		_ = unix.Unlinkat(haftDescriptor, name, 0)
		return nil, "", io.ErrShortWrite
	}
	if err := stage.Sync(); err != nil {
		_ = stage.Close()
		_ = unix.Unlinkat(haftDescriptor, name, 0)
		return nil, "", fmt.Errorf(
			"sync memory deferral stage: %w",
			err,
		)
	}
	return stage, name, nil
}

func openHeldDirectory(
	projectRoot string,
	withLock bool,
) (*heldDirectory, error) {
	absolute, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf(
			"resolve onboarding project root: %w",
			err,
		)
	}
	rootFD, err := unix.Open(
		absolute,
		unix.O_RDONLY|
			unix.O_DIRECTORY|
			unix.O_CLOEXEC|
			unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open onboarding project root: %w",
			err,
		)
	}
	root, err := newFileFromUnixDescriptor(rootFD, absolute)
	if err != nil {
		return nil, fmt.Errorf(
			"adopt onboarding project root: %w",
			err,
		)
	}
	haftFD, err := unix.Openat(
		rootFD,
		DirectoryName,
		unix.O_RDONLY|
			unix.O_DIRECTORY|
			unix.O_CLOEXEC|
			unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf(
			"open onboarding state directory: %w",
			err,
		)
	}
	haft, err := newFileFromUnixDescriptor(haftFD, DirectoryName)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf(
			"adopt onboarding state directory: %w",
			err,
		)
	}
	stat := unix.Stat_t{}
	if err := unix.Fstat(haftFD, &stat); err != nil {
		_ = haft.Close()
		_ = root.Close()
		return nil, fmt.Errorf(
			"inspect onboarding state directory: %w",
			err,
		)
	}
	directory := &heldDirectory{
		root:     root,
		haft:     haft,
		haftStat: stat,
	}
	if !withLock {
		return directory, nil
	}
	lockFD, err := unix.Openat(
		haftFD,
		lockFileName,
		unix.O_RDWR|
			unix.O_CREAT|
			unix.O_CLOEXEC|
			unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		_ = directory.close()
		return nil, fmt.Errorf(
			"open memory deferral lock: %w",
			err,
		)
	}
	lock, err := newFileFromUnixDescriptor(lockFD, lockFileName)
	if err != nil {
		_ = directory.close()
		return nil, fmt.Errorf(
			"adopt memory deferral lock: %w",
			err,
		)
	}
	directory.lock = lock
	return directory, nil
}

func (directory *heldDirectory) read() (ReadResult, error) {
	haftDescriptor, err := checkedFileDescriptor(
		directory.haft,
		"onboarding state directory",
	)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat(
		haftDescriptor,
		FileName,
		unix.O_RDONLY|
			unix.O_CLOEXEC|
			unix.O_NOFOLLOW,
		0,
	)
	if errors.Is(err, os.ErrNotExist) {
		return Absent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf(
			"open memory deferral carrier: %w",
			err,
		)
	}
	file, err := newFileFromUnixDescriptor(fd, FileName)
	if err != nil {
		return nil, fmt.Errorf(
			"adopt memory deferral carrier: %w",
			err,
		)
	}
	defer func() {
		_ = file.Close()
	}()
	stat := unix.Stat_t{}
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, fmt.Errorf(
			"inspect memory deferral carrier: %w",
			err,
		)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf(
			"memory deferral carrier is not a regular file",
		)
	}
	if stat.Size > MaximumBytes {
		return nil, fmt.Errorf(
			"memory deferral carrier exceeds %d bytes",
			MaximumBytes,
		)
	}
	content, err := io.ReadAll(
		io.LimitReader(file, MaximumBytes+1),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"read memory deferral carrier: %w",
			err,
		)
	}
	if len(content) > MaximumBytes {
		return nil, fmt.Errorf(
			"memory deferral carrier exceeds %d bytes",
			MaximumBytes,
		)
	}
	deferral, err := onboarding.DecodeMemoryDeferral(content)
	if err != nil {
		return nil, err
	}
	return Present{Deferral: deferral}, nil
}

func (directory *heldDirectory) readAttached() (ReadResult, error) {
	if err := directory.requireAttached(); err != nil {
		return nil, err
	}
	result, err := directory.read()
	if err != nil {
		return nil, err
	}
	if err := directory.requireAttached(); err != nil {
		return nil, err
	}
	return result, nil
}

func (directory *heldDirectory) requireAttached() error {
	rootDescriptor, err := checkedFileDescriptor(
		directory.root,
		"onboarding project root",
	)
	if err != nil {
		return err
	}
	current := unix.Stat_t{}
	if err := unix.Fstatat(
		rootDescriptor,
		DirectoryName,
		&current,
		unix.AT_SYMLINK_NOFOLLOW,
	); err != nil {
		return fmt.Errorf(
			"reinspect onboarding state directory: %w",
			err,
		)
	}
	if current.Mode&unix.S_IFMT != unix.S_IFDIR ||
		current.Dev != directory.haftStat.Dev ||
		current.Ino != directory.haftStat.Ino {
		return fmt.Errorf(
			"onboarding state directory identity changed",
		)
	}
	return nil
}

func (directory *heldDirectory) lockExclusive() error {
	if directory.lock == nil {
		return fmt.Errorf("memory deferral lock is unavailable")
	}
	lockDescriptor, err := checkedFileDescriptor(
		directory.lock,
		"memory deferral lock",
	)
	if err != nil {
		return err
	}
	if err := unix.Flock(
		lockDescriptor,
		unix.LOCK_EX,
	); err != nil {
		return fmt.Errorf("lock memory deferral carrier: %w", err)
	}
	return nil
}

func (directory *heldDirectory) unlock() {
	if directory.lock == nil {
		return
	}
	lockDescriptor, err := checkedFileDescriptor(
		directory.lock,
		"memory deferral lock",
	)
	if err != nil {
		return
	}
	_ = unix.Flock(
		lockDescriptor,
		unix.LOCK_UN,
	)
}

func (directory *heldDirectory) close() error {
	if directory == nil {
		return nil
	}
	var lockErr error
	if directory.lock != nil {
		lockErr = directory.lock.Close()
	}
	haftErr := directory.haft.Close()
	rootErr := directory.root.Close()
	return errors.Join(lockErr, haftErr, rootErr)
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
		return 0, fmt.Errorf(
			"%s descriptor exceeds the platform int range",
			subject,
		)
	}
	return int(descriptor), nil
}

func newFileFromUnixDescriptor(
	descriptor int,
	name string,
) (*os.File, error) {
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
