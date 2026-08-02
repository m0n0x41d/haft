package fpfrefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

var ErrRecoveryRequired = errors.New("FPF refresh recovery is required")

// RecoveryStatus is the read-only apply/re-entry state. Check mode uses this
// before fetching or building a new candidate so an interrupted apply cannot
// be silently bypassed.
type RecoveryStatus struct {
	Required   bool
	Receipt    ApplyReceipt
	Directions ReceiptRecoveryDirections
}

// InspectRecovery reports active receipt state. Terminal receipts are not
// recovery blockers; they remain available for explicit archival.
func InspectRecovery(path string) (RecoveryStatus, error) {
	receipt, err := LoadReceipt(path)
	if errors.Is(err, ErrReceiptNotFound) {
		return RecoveryStatus{}, nil
	}
	if err != nil {
		return RecoveryStatus{}, err
	}
	directions, err := receipt.RecoveryDirections()
	if err != nil {
		return RecoveryStatus{}, err
	}
	return RecoveryStatus{
		Required:   directions.Required,
		Receipt:    receipt,
		Directions: directions,
	}, nil
}

// ArchiveTerminalReceipt moves a complete/restored receipt out of the active
// recovery slot. An existing identical archive is reused; different bytes are
// never overwritten.
func ArchiveTerminalReceipt(
	receiptPath string,
	archiveDirectory string,
) (string, error) {
	release, err := acquireOperationLock(receiptPath)
	if err != nil {
		return "", err
	}
	defer release()
	return archiveTerminalReceiptLocked(receiptPath, archiveDirectory)
}

func archiveTerminalReceiptLocked(
	receiptPath string,
	archiveDirectory string,
) (string, error) {
	receipt, err := LoadReceipt(receiptPath)
	if errors.Is(err, ErrReceiptNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	directions, err := receipt.RecoveryDirections()
	if err != nil {
		return "", err
	}
	if directions.Required {
		return "", fmt.Errorf(
			"%w: active receipt state is %s",
			ErrRecoveryRequired,
			receipt.State,
		)
	}
	if !filepath.IsAbs(archiveDirectory) ||
		filepath.Clean(archiveDirectory) != archiveDirectory {
		return "", fmt.Errorf("receipt archive directory must be absolute and canonical")
	}
	if err := os.MkdirAll(archiveDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create receipt archive directory: %w", err)
	}
	payload, err := receipt.CanonicalJSON()
	if err != nil {
		return "", err
	}
	receiptDigest := strings.TrimPrefix(digestBytesSHA256(payload), "sha256:")
	archivePath := filepath.Join(
		archiveDirectory,
		receipt.Candidate.SourceSHA+"-"+string(receipt.State)+"-"+
			receiptDigest+".json",
	)
	existing, readErr := os.ReadFile(archivePath)
	switch {
	case readErr == nil:
		if string(existing) != string(payload) {
			return "", fmt.Errorf(
				"terminal receipt archive %s exists with different bytes",
				archivePath,
			)
		}
	case errors.Is(readErr, os.ErrNotExist):
		if err := writeExclusiveFile(archivePath, payload, 0o600); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("read terminal receipt archive: %w", readErr)
	}
	activePayload, err := os.ReadFile(receiptPath)
	if err != nil {
		return "", fmt.Errorf("reread active terminal receipt: %w", err)
	}
	if string(activePayload) != string(payload) {
		return "", fmt.Errorf("%w: active receipt changed before archival", ErrReceiptBusy)
	}
	if err := os.Remove(receiptPath); err != nil {
		return "", fmt.Errorf("remove archived active receipt: %w", err)
	}
	if err := syncDirectory(filepath.Dir(receiptPath)); err != nil {
		return "", err
	}
	return archivePath, nil
}

// ExecuteReceiptResume performs only the typed forward steps in one active
// receipt. Every byte source and target comes from that receipt.
func ExecuteReceiptResume(ctx context.Context, receiptPath string) (ApplyReceipt, error) {
	return executeReceiptRecovery(ctx, receiptPath, false)
}

// ExecuteReceiptRestore restores only the exact predecessor artifacts named by
// one active receipt and closes it in the restored terminal state.
func ExecuteReceiptRestore(ctx context.Context, receiptPath string) (ApplyReceipt, error) {
	return executeReceiptRecovery(ctx, receiptPath, true)
}

func executeReceiptRecovery(
	ctx context.Context,
	receiptPath string,
	restore bool,
) (ApplyReceipt, error) {
	release, err := acquireOperationLock(receiptPath)
	if err != nil {
		return ApplyReceipt{}, err
	}
	defer release()
	return executeReceiptRecoveryLocked(ctx, receiptPath, restore)
}

func executeReceiptRecoveryLocked(
	ctx context.Context,
	receiptPath string,
	restore bool,
) (ApplyReceipt, error) {
	receipt, err := LoadReceipt(receiptPath)
	if err != nil {
		return ApplyReceipt{}, err
	}
	directions, err := receipt.RecoveryDirections()
	if err != nil {
		return ApplyReceipt{}, err
	}
	if !directions.Required {
		return receipt, nil
	}
	steps := directions.Resume
	if restore {
		steps = directions.Restore
	}
	if len(steps) == 0 {
		return ApplyReceipt{}, fmt.Errorf(
			"%w: receipt state %s has no %s continuation",
			ErrReceiptTransition,
			receipt.State,
			map[bool]string{false: "resume", true: "restore"}[restore],
		)
	}

	basis := receipt.Basis()
	if err := VerifySourceCheckoutClean(
		ctx,
		RepositoryLayout{SourceRepository: basis.Targets.SourcePath},
	); err != nil {
		return ApplyReceipt{}, fmt.Errorf(
			"%w: state=%s preflight-source-cleanliness: %v",
			ErrRecoveryRequired,
			receipt.State,
			err,
		)
	}
	for _, step := range steps {
		if err := executeReceiptStep(ctx, receiptPath, basis, step); err != nil {
			return ApplyReceipt{}, fmt.Errorf(
				"%w: state=%s step=%s: %v",
				ErrRecoveryRequired,
				receipt.State,
				step.Kind,
				err,
			)
		}
		if step.ResultState != "" {
			receipt, err = AdvanceReceipt(receiptPath, basis, step.ResultState)
			if err != nil {
				return ApplyReceipt{}, err
			}
		}
	}
	return LoadReceiptFor(receiptPath, basis)
}

func executeReceiptStep(
	ctx context.Context,
	receiptPath string,
	basis ReceiptBasis,
	step ReceiptRecoveryStep,
) error {
	switch step.Kind {
	case RecoveryApplyCandidateSource, RecoveryRestorePredecessorSource:
		return ensureExactSourceRevision(
			ctx,
			basis.Targets.SourcePath,
			step.ExpectedSourceSHA,
			step.SourceSHA,
		)
	case RecoveryApplyCandidateDatabase:
		return ensureExactInstalledArtifact(
			step.ArtifactPath,
			basis.Targets.DatabasePath,
			step.ArtifactDigest,
			[]string{
				basis.Predecessor.DatabaseDigest,
				basis.Candidate.DatabaseDigest,
			},
			false,
		)
	case RecoveryRestorePredecessorDatabase:
		return ensureExactInstalledArtifact(
			step.ArtifactPath,
			basis.Targets.DatabasePath,
			step.ArtifactDigest,
			[]string{
				basis.Candidate.DatabaseDigest,
				basis.Predecessor.DatabaseDigest,
			},
			false,
		)
	case RecoveryApplyCandidateTokenGate, RecoveryRestorePredecessorTokenGate:
		return ensureExactInstalledArtifact(
			step.ArtifactPath,
			basis.Targets.TokenGateFixturePath,
			step.ArtifactDigest,
			allowedTokenGateFixtureDigests(basis),
			false,
		)
	case RecoveryMaterializeCandidateLock:
		return ensureExactInstalledArtifact(
			step.ArtifactPath,
			basis.Targets.LockPath,
			step.ArtifactDigest,
			allowedLockDigests(basis),
			basis.Artifacts.PredecessorLock.Presence == ReceiptLockMissing,
		)
	case RecoveryRestorePredecessorLock:
		return ensureExactInstalledArtifact(
			step.ArtifactPath,
			basis.Targets.LockPath,
			step.ArtifactDigest,
			allowedLockDigests(basis),
			false,
		)
	case RecoveryRemoveCandidateLock:
		return ensureExactLockAbsent(basis)
	case RecoveryVerifyCandidatePair:
		return verifyReceiptPair(ctx, basis, true)
	case RecoveryVerifyPredecessorPair:
		return verifyReceiptPair(ctx, basis, false)
	case RecoveryMarkReceiptComplete:
		if err := verifyReceiptPair(ctx, basis, true); err != nil {
			return err
		}
		return nil
	case RecoveryMarkReceiptRestored:
		if err := verifyReceiptPair(ctx, basis, false); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("receipt %s contains unsupported recovery step %q", receiptPath, step.Kind)
	}
}

func ensureExactSourceRevision(
	ctx context.Context,
	repositoryPath string,
	expected string,
	desired string,
) error {
	current, err := exactGitRevisionAt(ctx, repositoryPath)
	if err != nil {
		return err
	}
	if current == desired {
		return VerifySourceCheckoutClean(
			ctx,
			RepositoryLayout{SourceRepository: repositoryPath},
		)
	}
	if current != expected {
		return fmt.Errorf(
			"%w: source revision is %s, expected %s or already-applied %s",
			ErrReceiptStale,
			current,
			expected,
			desired,
		)
	}
	status, err := runRepositoryGit(
		ctx,
		repositoryPath,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return fmt.Errorf("inspect source checkout before recovery: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return fmt.Errorf(
			"%w: source checkout has unrelated dirt:\n%s",
			ErrReceiptStale,
			strings.TrimSpace(string(status)),
		)
	}
	if _, err := runRepositoryGit(
		ctx,
		repositoryPath,
		"checkout",
		"--detach",
		desired,
	); err != nil {
		return fmt.Errorf("checkout source revision %s: %w", desired, err)
	}
	observed, err := exactGitRevisionAt(ctx, repositoryPath)
	if err != nil {
		return err
	}
	if observed != desired {
		return fmt.Errorf("source checkout is %s after requesting %s", observed, desired)
	}
	return VerifySourceCheckoutClean(
		ctx,
		RepositoryLayout{SourceRepository: repositoryPath},
	)
}

func exactGitRevisionAt(ctx context.Context, repositoryPath string) (string, error) {
	output, err := runRepositoryGit(ctx, repositoryPath, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	revision := strings.TrimSpace(string(output))
	if !fullGitCommitSHAPattern.MatchString(revision) {
		return "", fmt.Errorf("source revision %q is not an exact commit SHA", revision)
	}
	return revision, nil
}

func ensureExactInstalledArtifact(
	artifactPath string,
	targetPath string,
	expectedDigest string,
	allowedCurrentDigests []string,
	allowMissingCurrent bool,
) error {
	if err := verifyRegularFileDigest(artifactPath, expectedDigest); err != nil {
		return fmt.Errorf("verify prepared artifact: %w", err)
	}
	currentDigest, exists, err := optionalRegularFileDigest(targetPath)
	if err != nil {
		return err
	}
	if exists && currentDigest == expectedDigest {
		return nil
	}
	if !exists && !allowMissingCurrent {
		return fmt.Errorf(
			"%w: target %s is absent",
			ErrReceiptStale,
			targetPath,
		)
	}
	if exists && !containsExactString(allowedCurrentDigests, currentDigest) {
		return fmt.Errorf(
			"%w: target %s has unrecognized digest %s",
			ErrReceiptStale,
			targetPath,
			currentDigest,
		)
	}
	if err := installFileAtomic(artifactPath, targetPath); err != nil {
		return err
	}
	return verifyRegularFileDigest(targetPath, expectedDigest)
}

func ensureExactLockAbsent(basis ReceiptBasis) error {
	target := basis.Targets.LockPath
	digest, exists, err := optionalRegularFileDigest(target)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if digest != basis.Artifacts.CandidateLockDigest {
		return fmt.Errorf(
			"%w: lock target has unrecognized digest %s",
			ErrReceiptStale,
			digest,
		)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove candidate integration lock: %w", err)
	}
	return syncDirectory(filepath.Dir(target))
}

func verifyReceiptPair(
	ctx context.Context,
	basis ReceiptBasis,
	candidate bool,
) error {
	coordinates := basis.Predecessor
	if candidate {
		coordinates = basis.Candidate
	}
	revision, err := exactGitRevisionAt(ctx, basis.Targets.SourcePath)
	if err != nil {
		return err
	}
	if revision != coordinates.SourceSHA {
		return fmt.Errorf(
			"%w: source revision %s, want %s",
			ErrReceiptStale,
			revision,
			coordinates.SourceSHA,
		)
	}
	if err := VerifySourceCheckoutClean(
		ctx,
		RepositoryLayout{SourceRepository: basis.Targets.SourcePath},
	); err != nil {
		return fmt.Errorf("%w: %v", ErrReceiptStale, err)
	}
	if err := verifyRegularFileDigest(
		basis.Targets.DatabasePath,
		coordinates.DatabaseDigest,
	); err != nil {
		return err
	}
	observed, err := ReadIntegrationCoordinates(IntegrationCoordinateInput{
		SourceRevision: coordinates.SourceSHA,
		ReadmePath: filepath.Join(
			basis.Targets.SourcePath,
			gitSourceReadmePath,
		),
		SpecPath: filepath.Join(
			basis.Targets.SourcePath,
			gitSourceSpecPath,
		),
		DatabasePath: basis.Targets.DatabasePath,
	})
	if err != nil {
		return fmt.Errorf(
			"%w: source/database pair is incoherent: %v",
			ErrReceiptStale,
			err,
		)
	}
	if observed.SourceRevision != coordinates.SourceSHA ||
		observed.DatabaseDigest != coordinates.DatabaseDigest {
		return fmt.Errorf(
			"%w: source/database pair differs from receipt coordinates",
			ErrReceiptStale,
		)
	}

	if candidate {
		if err := verifyRegularFileDigest(
			basis.Targets.LockPath,
			basis.Artifacts.CandidateLockDigest,
		); err != nil {
			return err
		}
		if err := verifyReceiptIntegrationLock(basis, coordinates); err != nil {
			return err
		}
		_, err := VerifyCandidateQueryContract(basis.Targets.DatabasePath)
		return err
	}
	switch basis.Artifacts.PredecessorLock.Presence {
	case ReceiptLockMissing:
		_, exists, err := optionalRegularFileDigest(basis.Targets.LockPath)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w: predecessor integration lock should be absent", ErrReceiptStale)
		}
	case ReceiptLockPresent:
		if err := verifyRegularFileDigest(
			basis.Targets.LockPath,
			basis.Artifacts.PredecessorLock.Digest,
		); err != nil {
			return err
		}
		if err := verifyReceiptIntegrationLock(basis, coordinates); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported predecessor lock presence %q", basis.Artifacts.PredecessorLock.Presence)
	}
	return VerifySourceQueryRuntime(basis.Targets.DatabasePath)
}

func verifyReceiptIntegrationLock(
	basis ReceiptBasis,
	coordinates ReceiptCoordinates,
) error {
	payload, err := os.ReadFile(basis.Targets.LockPath)
	if err != nil {
		return err
	}
	lock, err := ParseIntegrationLock(payload)
	if err != nil {
		return err
	}
	if lock.Coordinates.SourceRevision != coordinates.SourceSHA ||
		lock.Coordinates.DatabaseDigest != coordinates.DatabaseDigest {
		return fmt.Errorf(
			"%w: integration lock does not name the receipt source/database pair",
			ErrReceiptStale,
		)
	}
	fixturePath := basis.Targets.TokenGateFixturePath
	if fixturePath == "" {
		root := filepath.Dir(filepath.Dir(basis.Targets.SourcePath))
		fixturePath = filepath.Join(root, DefaultTokenGateFixtureRelativePath)
	}
	if err := verifyRepositoryTokenGateFixture(fixturePath, lock.TokenGate); err != nil {
		return fmt.Errorf("%w: %v", ErrReceiptStale, err)
	}
	input := IntegrationCoordinateInput{
		SourceRevision: coordinates.SourceSHA,
		ReadmePath:     filepath.Join(basis.Targets.SourcePath, gitSourceReadmePath),
		SpecPath:       filepath.Join(basis.Targets.SourcePath, gitSourceSpecPath),
		DatabasePath:   basis.Targets.DatabasePath,
		GeneratedBy:    lock.GeneratedBy,
		TokenGate:      lock.TokenGate,
	}
	if err := VerifyIntegrationLock(lock, input); err != nil {
		return err
	}
	return nil
}

func verifyRepositoryTokenGateFixture(
	fixturePath string,
	expected *TokenGateCoordinates,
) error {
	if expected == nil {
		return nil
	}
	observed, err := ReadTokenGateCoordinates(fixturePath)
	if err != nil {
		return err
	}
	if observed != *expected {
		return fmt.Errorf(
			"token-gate fixture coordinates %#v differ from integration lock %#v",
			observed,
			*expected,
		)
	}
	return nil
}

func allowedLockDigests(basis ReceiptBasis) []string {
	values := []string{basis.Artifacts.CandidateLockDigest}
	if basis.Artifacts.PredecessorLock.Presence == ReceiptLockPresent {
		values = append(values, basis.Artifacts.PredecessorLock.Digest)
	}
	return values
}

func allowedTokenGateFixtureDigests(basis ReceiptBasis) []string {
	values := []string{basis.Artifacts.CandidateTokenGateFixtureDigest}
	if basis.Artifacts.PredecessorTokenGateFixturePresence == ReceiptLockPresent {
		values = append(
			values,
			basis.Artifacts.PredecessorTokenGateFixtureDigest,
		)
	}
	return values
}

func verifyRegularFileDigest(path string, expected string) error {
	actual, exists, err := optionalRegularFileDigest(path)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: required file %s is absent", ErrReceiptStale, path)
	}
	if actual != expected {
		return fmt.Errorf(
			"%w: %s digest %s, want %s",
			ErrReceiptStale,
			path,
			actual,
			expected,
		)
	}
	return nil
}

func optionalRegularFileDigest(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("%w: %s is not a regular file", ErrReceiptStale, path)
	}
	digest, err := digestFile(path)
	if err != nil {
		return "", false, fmt.Errorf("digest %s: %w", path, err)
	}
	return digest, true, nil
}

func installFileAtomic(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open prepared artifact: %w", err)
	}
	defer func() { _ = source.Close() }()
	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat prepared artifact: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("prepared artifact %s is not a regular file", sourcePath)
	}
	parent := filepath.Dir(targetPath)
	stage, err := os.CreateTemp(parent, "."+filepath.Base(targetPath)+".refresh-*")
	if err != nil {
		return fmt.Errorf("create target stage: %w", err)
	}
	stagePath := stage.Name()
	stagePresent := true
	defer func() {
		_ = stage.Close()
		if stagePresent {
			_ = os.Remove(stagePath)
		}
	}()
	if err := stage.Chmod(sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("set target stage mode: %w", err)
	}
	if _, err := io.Copy(stage, source); err != nil {
		return fmt.Errorf("copy prepared artifact: %w", err)
	}
	if err := stage.Sync(); err != nil {
		return fmt.Errorf("sync target stage: %w", err)
	}
	if err := stage.Close(); err != nil {
		return fmt.Errorf("close target stage: %w", err)
	}
	if err := os.Rename(stagePath, targetPath); err != nil {
		return fmt.Errorf("atomically install target artifact: %w", err)
	}
	stagePresent = false
	return syncDirectory(parent)
}

func writeExclusiveFile(path string, content []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create exclusive file %s: %w", path, err)
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write exclusive file %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync exclusive file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close exclusive file %s: %w", path, err)
	}
	keep = true
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close synced directory: %w", closeErr)
	}
	return nil
}

func acquireOperationLock(receiptPath string) (func(), error) {
	path := receiptPath + ".operation.lock"
	fd, err := unix.Open(
		path,
		unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("create refresh operation lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("adopt refresh operation lock %s", path)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat refresh operation lock %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		_ = file.Close()
		return nil, fmt.Errorf(
			"%w: operation lock carrier %q is not a private regular file",
			ErrReceiptStale,
			path,
		)
	}
	if err := unix.Flock(
		int(file.Fd()), // #nosec G115 -- file is an open lock descriptor.
		unix.LOCK_EX|unix.LOCK_NB,
	); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, fmt.Errorf(
				"%w: another refresh operation is active",
				ErrReceiptBusy,
			)
		}
		return nil, fmt.Errorf("lock refresh operation %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN) // #nosec G115
		_ = file.Close()
		return nil, fmt.Errorf("sync refresh operation lock %s: %w", path, err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN) // #nosec G115
		_ = file.Close()
		return nil, err
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		_ = unix.Flock(
			int(file.Fd()), // #nosec G115 -- file remains open until this release.
			unix.LOCK_UN,
		)
		_ = file.Close()
	}, nil
}

func containsExactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
