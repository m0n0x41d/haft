//go:build darwin || linux

package profileprojection

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	projectionDirectoryName = ".haft"
	projectionFileName      = "project-profile.yaml"
	projectionStagePrefix   = ".project-profile.yaml.project-profile-projection-stage-"
	projectionStageSuffix   = ".tmp"
)

// projectionDirectory keeps both directory identities open for the complete
// observation/write/reread operation. All child effects are *at syscalls and
// never resolve .haft by pathname after this capability is minted.
type projectionDirectory struct {
	rootPath string
	root     *os.File
	haft     *os.File
}

func openProjectionDirectory(projectRoot string) (*projectionDirectory, error) {
	root, err := openDirectoryNoFollow(projectRoot)
	if err != nil {
		return nil, err
	}
	haft, err := openHaftDirectory(root)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	directory := &projectionDirectory{rootPath: projectRoot, root: root, haft: haft}
	if err := directory.requireAttachedIdentity(); err != nil {
		_ = directory.Close()
		return nil, err
	}
	return directory, nil
}

func (directory *projectionDirectory) Close() error {
	if directory == nil {
		return nil
	}
	var haftErr error
	if directory.haft != nil {
		haftErr = directory.haft.Close()
		directory.haft = nil
	}
	var rootErr error
	if directory.root != nil {
		rootErr = directory.root.Close()
		directory.root = nil
	}
	return errors.Join(haftErr, rootErr)
}

func (directory *projectionDirectory) observe(expected []byte) projectionObservation {
	if err := directory.requireAttachedIdentity(); err != nil {
		return unreadableProjectionObservation(err)
	}
	content, err := directory.readTarget()
	if errors.Is(err, os.ErrNotExist) {
		return projectionObservation{
			kind:   observationMissing,
			detail: "canonical profile exists but its YAML projection is missing",
		}
	}
	if err != nil {
		return projectionObservation{
			kind:   observationUnreadable,
			detail: fmt.Sprintf("profile projection cannot be read safely: %v", err),
		}
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return unreadableProjectionObservation(err)
	}
	digest, err := digestProjection(content)
	if err != nil {
		return projectionObservation{
			kind:   observationUnreadable,
			detail: fmt.Sprintf("profile projection cannot be digested: %v", err),
		}
	}
	if bytes.Equal(content, expected) {
		return projectionObservation{
			kind:   observationMatched,
			digest: digest,
			detail: "profile projection exactly matches the canonical ledger revision",
		}
	}
	return projectionObservation{
		kind:   observationDrifted,
		digest: digest,
		detail: "profile projection bytes drift from the canonical ledger revision",
	}
}

func (directory *projectionDirectory) reconcileStages() error {
	if err := directory.valid(); err != nil {
		return err
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	entries, err := directory.haft.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("enumerate profile projection stages: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, projectionStagePrefix) {
			continue
		}
		if !strings.HasSuffix(name, projectionStageSuffix) {
			continue
		}
		var stat unix.Stat_t
		err := unix.Fstatat(directory.haftFD(), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return fmt.Errorf("inspect stale profile projection stage: %w", err)
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("profile projection stage %s is not a regular file", name)
		}
		if err := unix.Unlinkat(directory.haftFD(), name, 0); err != nil {
			return fmt.Errorf("remove stale profile projection stage: %w", err)
		}
	}
	if err := syncDirectoryFile(directory.haft); err != nil {
		return err
	}
	return directory.requireAttachedIdentity()
}

func (directory *projectionDirectory) writeAtomic(
	content []byte,
	temporaryID string,
) error {
	if err := directory.valid(); err != nil {
		return err
	}
	if !validTemporaryIdentifier(temporaryID) {
		return fmt.Errorf("profile projection temporary identifier is invalid")
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	stageName := ".project-profile.yaml." + temporaryID + projectionStageSuffix
	stage, err := openExclusiveFileAt(directory.haftFD(), stageName, 0o644)
	if err != nil {
		return err
	}
	stagePresent := true
	defer func() {
		if stagePresent {
			_ = unix.Unlinkat(directory.haftFD(), stageName, 0)
		}
	}()
	writeErr := writeAndSync(stage, content)
	closeErr := stage.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := directory.requireAttachedIdentity(); err != nil {
		return err
	}
	err = unix.Renameat(
		directory.haftFD(),
		stageName,
		directory.haftFD(),
		projectionFileName,
	)
	if err != nil {
		return fmt.Errorf("atomically install profile projection: %w", err)
	}
	stagePresent = false
	if err := syncDirectoryFile(directory.haft); err != nil {
		return err
	}
	observed, err := directory.readTarget()
	if err != nil {
		return err
	}
	if !bytes.Equal(observed, content) {
		return fmt.Errorf("profile projection reread differs from written bytes")
	}
	return directory.requireAttachedIdentity()
}

func (directory *projectionDirectory) readTarget() ([]byte, error) {
	if err := directory.valid(); err != nil {
		return nil, err
	}
	file, err := openReadOnlyFileAt(directory.haftFD(), projectionFileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (directory *projectionDirectory) valid() error {
	if directory == nil || directory.rootPath == "" || directory.root == nil || directory.haft == nil {
		return fmt.Errorf("profile projection directory capability is invalid")
	}
	return nil
}

func (directory *projectionDirectory) requireAttachedIdentity() error {
	if err := directory.valid(); err != nil {
		return err
	}
	var heldRoot unix.Stat_t
	if err := unix.Fstat(int(directory.root.Fd()), &heldRoot); err != nil { // #nosec G115 -- root wraps a descriptor returned by unix.Open.
		return fmt.Errorf("inspect held project root: %w", err)
	}
	var pathRoot unix.Stat_t
	if err := unix.Lstat(directory.rootPath, &pathRoot); err != nil {
		return fmt.Errorf("inspect project-root path identity: %w", err)
	}
	if heldRoot.Dev != pathRoot.Dev || heldRoot.Ino != pathRoot.Ino {
		return fmt.Errorf("project-root path identity changed during profile projection")
	}
	var heldHaft unix.Stat_t
	if err := unix.Fstat(directory.haftFD(), &heldHaft); err != nil {
		return fmt.Errorf("inspect held .haft directory: %w", err)
	}
	var attachedHaft unix.Stat_t
	err := unix.Fstatat(
		int(directory.root.Fd()), // #nosec G115 -- root wraps the descriptor returned by unix.Open.
		projectionDirectoryName,
		&attachedHaft,
		unix.AT_SYMLINK_NOFOLLOW,
	)
	if err != nil {
		return fmt.Errorf("inspect attached .haft identity: %w", err)
	}
	if heldHaft.Dev != attachedHaft.Dev || heldHaft.Ino != attachedHaft.Ino {
		return fmt.Errorf(".haft directory identity changed during profile projection")
	}
	return nil
}

func (directory *projectionDirectory) haftFD() int {
	return int(directory.haft.Fd()) // #nosec G115 -- haft wraps the descriptor returned by unix.Openat.
}

func openHaftDirectory(root *os.File) (*os.File, error) {
	rootFD := int(root.Fd()) // #nosec G115 -- root wraps the descriptor returned by unix.Open.
	haft, err := openDirectoryAt(rootFD, projectionDirectoryName)
	if err == nil {
		return haft, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	err = unix.Mkdirat(rootFD, projectionDirectoryName, 0o755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create profile projection directory: %w", err)
	}
	if err := syncDirectoryFile(root); err != nil {
		return nil, err
	}
	return openDirectoryAt(rootFD, projectionDirectoryName)
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open profile projection root without following symlinks: %w", err)
	}
	return os.NewFile(uintptr(fd), path), nil // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
}

func openDirectoryAt(parentFD int, name string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
}

func openExclusiveFileAt(parentFD int, name string, mode os.FileMode) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(parentFD, name, flags, uint32(mode.Perm()))
	if err != nil {
		return nil, fmt.Errorf("create profile projection stage: %w", err)
	}
	return os.NewFile(uintptr(fd), name), nil // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
}

func openReadOnlyFileAt(parentFD int, name string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name) // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
	var stat unix.Stat_t
	err = unix.Fstat(fd, &stat)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = file.Close()
		return nil, fmt.Errorf("profile projection is not a regular file")
	}
	return file, nil
}

func syncDirectoryFile(directory *os.File) error {
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync profile projection directory: %w", err)
	}
	return nil
}

func validTemporaryIdentifier(value string) bool {
	return value != "" &&
		!strings.ContainsAny(value, `/\\`) &&
		!strings.Contains(value, "..")
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
