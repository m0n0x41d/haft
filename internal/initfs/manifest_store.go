package initfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type ManifestReadKind string

const (
	ManifestReadMissing ManifestReadKind = "missing"
	ManifestReadPresent ManifestReadKind = "present"
)

type ManifestReadResult struct {
	kind     ManifestReadKind
	path     string
	manifest initplanning.InstallationManifest
}

func (result ManifestReadResult) Kind() ManifestReadKind {
	return result.kind
}

func (result ManifestReadResult) Path() string {
	return result.path
}

func (result ManifestReadResult) Manifest() initplanning.InstallationManifest {
	return result.manifest
}

type ManifestPreconditionKind string

const (
	ManifestExpectedMissing ManifestPreconditionKind = "expected_missing"
	ManifestExpectedDigest  ManifestPreconditionKind = "expected_digest"
)

type ManifestPrecondition struct {
	kind   ManifestPreconditionKind
	digest string
}

func ExpectManifestMissing() ManifestPrecondition {
	return ManifestPrecondition{kind: ManifestExpectedMissing}
}

func ExpectManifestWithDigest(digest string) (ManifestPrecondition, error) {
	if !validSHA256Digest(digest) {
		return ManifestPrecondition{}, fmt.Errorf("manifest precondition digest is invalid")
	}
	return ManifestPrecondition{kind: ManifestExpectedDigest, digest: digest}, nil
}

type ManifestPersistKind string

const (
	ManifestPersisted          ManifestPersistKind = "persisted"
	ManifestAlreadyCurrent     ManifestPersistKind = "already_current"
	ManifestPreconditionFailed ManifestPersistKind = "precondition_failed"
	ManifestLockHeld           ManifestPersistKind = "lock_held"
)

type ManifestPersistOutcome struct {
	kind           ManifestPersistKind
	path           string
	expectedDigest string
	observedDigest string
	desiredDigest  string
}

func (outcome ManifestPersistOutcome) Kind() ManifestPersistKind {
	return outcome.kind
}

func (outcome ManifestPersistOutcome) Path() string {
	return outcome.path
}

func (outcome ManifestPersistOutcome) ExpectedDigest() string {
	return outcome.expectedDigest
}

func (outcome ManifestPersistOutcome) ObservedDigest() string {
	return outcome.observedDigest
}

func (outcome ManifestPersistOutcome) DesiredDigest() string {
	return outcome.desiredDigest
}

type ManifestStore struct {
	root     string
	path     string
	lockPath string
	observer FileObserver
}

func (store ManifestStore) Root() string {
	return store.root
}

func (store ManifestStore) Path() string {
	return store.path
}

func (store ManifestStore) LockPath() string {
	return store.lockPath
}

func (store ManifestStore) JournalPath() string {
	return store.path + ".pending"
}

func NewManifestStore(
	root string,
	path string,
	maxManifestBytes int64,
) (ManifestStore, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return ManifestStore{}, fmt.Errorf("manifest store root is not canonical")
	}
	selectedRoot, err := containingManagedRoot(path, []string{root})
	if err != nil || selectedRoot != root || path == root {
		return ManifestStore{}, fmt.Errorf("manifest store path is outside its root")
	}
	observer, err := NewFileObserver(maxManifestBytes)
	if err != nil {
		return ManifestStore{}, err
	}
	return ManifestStore{
		root:     root,
		path:     path,
		lockPath: path + ".lock",
		observer: observer,
	}, nil
}

func (store ManifestStore) Read() (ManifestReadResult, error) {
	info, missing, err := lstatWithoutManagedRootSymlinks(store.root, store.path)
	if err != nil {
		return ManifestReadResult{}, err
	}
	if missing {
		return ManifestReadResult{kind: ManifestReadMissing, path: store.path}, nil
	}
	content, _, err := store.observer.readStableRegularFile(store.path, info)
	if err != nil {
		return ManifestReadResult{}, err
	}
	manifest, err := initplanning.ParseInstallationManifest(content)
	if err != nil {
		return ManifestReadResult{}, fmt.Errorf("parse installation manifest %s: %w", store.path, err)
	}
	return ManifestReadResult{
		kind:     ManifestReadPresent,
		path:     store.path,
		manifest: manifest,
	}, nil
}

func (store ManifestStore) Persist(
	manifest initplanning.InstallationManifest,
	precondition ManifestPrecondition,
) (ManifestPersistOutcome, error) {
	if err := validateManifestPersistInput(manifest, precondition); err != nil {
		return ManifestPersistOutcome{}, err
	}
	initial, err := store.Read()
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	if outcome, terminal := manifestPreconditionOutcome(
		store.path,
		manifest.Digest(),
		precondition,
		initial,
	); terminal {
		return outcome, nil
	}
	lease, acquired, err := store.TryAcquire()
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	if !acquired {
		return ManifestPersistOutcome{
			kind:           ManifestLockHeld,
			path:           store.path,
			expectedDigest: precondition.digest,
			desiredDigest:  manifest.Digest(),
		}, nil
	}
	outcome, persistErr := lease.Persist(manifest, precondition)
	releaseErr := lease.Release()
	if err := errors.Join(persistErr, releaseErr); err != nil {
		return ManifestPersistOutcome{}, err
	}
	return outcome, nil
}

type ManifestLease struct {
	mu       sync.Mutex
	store    ManifestStore
	lock     *manifestLock
	released bool
}

func (store ManifestStore) TryAcquire() (*ManifestLease, bool, error) {
	if err := store.valid(); err != nil {
		return nil, false, err
	}
	parent := filepath.Dir(store.path)
	if err := ensureDirectoryTreeWithoutSymlinks(store.root, parent); err != nil {
		return nil, false, err
	}
	lock, acquired, err := tryAcquireManifestLock(store.lockPath)
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	if err := reconcileCanonicalCarrierStages(store); err != nil {
		_ = lock.release()
		return nil, false, err
	}
	return &ManifestLease{
		store: store,
		lock:  lock,
	}, true, nil
}

func (lease *ManifestLease) Read() (ManifestReadResult, error) {
	if lease == nil {
		return ManifestReadResult{}, fmt.Errorf("manifest lease is nil")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return ManifestReadResult{}, fmt.Errorf("manifest lease is released")
	}
	return lease.store.Read()
}

func (lease *ManifestLease) Persist(
	manifest initplanning.InstallationManifest,
	precondition ManifestPrecondition,
) (ManifestPersistOutcome, error) {
	if lease == nil {
		return ManifestPersistOutcome{}, fmt.Errorf("manifest lease is nil")
	}
	if err := validateManifestPersistInput(manifest, precondition); err != nil {
		return ManifestPersistOutcome{}, err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return ManifestPersistOutcome{}, fmt.Errorf("manifest lease is released")
	}
	return lease.store.persistLocked(manifest, precondition)
}

func (lease *ManifestLease) Release() error {
	if lease == nil {
		return fmt.Errorf("manifest lease is nil")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return nil
	}
	if lease.lock == nil {
		return fmt.Errorf("manifest lease lock is absent")
	}
	if err := lease.lock.release(); err != nil {
		return fmt.Errorf("release manifest lock %s: %w", lease.store.lockPath, err)
	}
	lease.lock = nil
	lease.released = true
	return nil
}

func validateManifestPersistInput(
	manifest initplanning.InstallationManifest,
	precondition ManifestPrecondition,
) error {
	canonical := manifest.CanonicalBytes()
	parsed, err := initplanning.ParseInstallationManifest(canonical)
	if err != nil || parsed.Digest() != manifest.Digest() {
		return fmt.Errorf("manifest store received an invalid canonical manifest")
	}
	return validateManifestPrecondition(precondition)
}

func (store ManifestStore) valid() error {
	if store.root == "" ||
		store.path == "" ||
		store.lockPath != store.path+".lock" ||
		store.observer.maxFileBytes <= 0 {
		return fmt.Errorf("manifest store is invalid")
	}
	return nil
}

func (store ManifestStore) persistLocked(
	manifest initplanning.InstallationManifest,
	precondition ManifestPrecondition,
) (ManifestPersistOutcome, error) {
	current, err := store.Read()
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	if outcome, terminal := manifestPreconditionOutcome(
		store.path,
		manifest.Digest(),
		precondition,
		current,
	); terminal {
		return outcome, nil
	}
	stagePath, err := stageCanonicalCarrier(store.path, manifest.CanonicalBytes())
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.Remove(stagePath)
		}
	}()
	current, err = store.Read()
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	if outcome, terminal := manifestPreconditionOutcome(
		store.path,
		manifest.Digest(),
		precondition,
		current,
	); terminal {
		return outcome, nil
	}
	if precondition.kind == ManifestExpectedMissing {
		if err := os.Link(stagePath, store.path); err != nil {
			if os.IsExist(err) {
				observed, readErr := store.Read()
				if readErr != nil {
					return ManifestPersistOutcome{}, readErr
				}
				outcome, _ := manifestPreconditionOutcome(
					store.path,
					manifest.Digest(),
					precondition,
					observed,
				)
				return outcome, nil
			}
			return ManifestPersistOutcome{}, fmt.Errorf("publish new installation manifest: %w", err)
		}
	}
	if precondition.kind == ManifestExpectedDigest {
		if err := os.Rename(stagePath, store.path); err != nil {
			return ManifestPersistOutcome{}, fmt.Errorf("replace installation manifest: %w", err)
		}
		removeStage = false
	}
	if precondition.kind == ManifestExpectedMissing {
		if err := os.Remove(stagePath); err != nil {
			return ManifestPersistOutcome{}, fmt.Errorf("remove linked manifest stage: %w", err)
		}
		removeStage = false
	}
	if err := syncDirectory(filepath.Dir(store.path)); err != nil {
		return ManifestPersistOutcome{}, err
	}
	verified, err := store.Read()
	if err != nil {
		return ManifestPersistOutcome{}, err
	}
	if verified.kind != ManifestReadPresent || verified.manifest.Digest() != manifest.Digest() {
		return ManifestPersistOutcome{}, fmt.Errorf("persisted installation manifest failed exact reread")
	}
	return ManifestPersistOutcome{
		kind:           ManifestPersisted,
		path:           store.path,
		expectedDigest: precondition.digest,
		observedDigest: currentManifestDigest(current),
		desiredDigest:  manifest.Digest(),
	}, nil
}

func manifestPreconditionOutcome(
	path string,
	desiredDigest string,
	precondition ManifestPrecondition,
	current ManifestReadResult,
) (ManifestPersistOutcome, bool) {
	observedDigest := currentManifestDigest(current)
	base := ManifestPersistOutcome{
		path:           path,
		expectedDigest: precondition.digest,
		observedDigest: observedDigest,
		desiredDigest:  desiredDigest,
	}
	if current.kind == ManifestReadPresent && observedDigest == desiredDigest {
		base.kind = ManifestAlreadyCurrent
		return base, true
	}
	switch precondition.kind {
	case ManifestExpectedMissing:
		if current.kind == ManifestReadMissing {
			return base, false
		}
		base.kind = ManifestPreconditionFailed
		return base, true
	case ManifestExpectedDigest:
		if current.kind != ManifestReadPresent || observedDigest != precondition.digest {
			base.kind = ManifestPreconditionFailed
			return base, true
		}
		return base, false
	default:
		base.kind = ManifestPreconditionFailed
		return base, true
	}
}

func currentManifestDigest(current ManifestReadResult) string {
	if current.kind != ManifestReadPresent {
		return ""
	}
	return current.manifest.Digest()
}

func validateManifestPrecondition(precondition ManifestPrecondition) error {
	switch precondition.kind {
	case ManifestExpectedMissing:
		if precondition.digest != "" {
			return fmt.Errorf("missing-manifest precondition carries a digest")
		}
		return nil
	case ManifestExpectedDigest:
		if !validSHA256Digest(precondition.digest) {
			return fmt.Errorf("manifest digest precondition is invalid")
		}
		return nil
	default:
		return fmt.Errorf("manifest precondition kind is invalid")
	}
}

func validSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	raw := strings.TrimPrefix(value, "sha256:")
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func ensureDirectoryTreeWithoutSymlinks(root string, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("manifest parent is outside store root")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return observationFailure(ObservationReadFailure, root, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return observationFailure(ObservationUnsafePath, root, fmt.Errorf("manifest store root is not a real directory"))
	}
	current := root
	for _, segment := range pathSegments(relative) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return fmt.Errorf("create manifest directory %s: %w", current, err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return observationFailure(ObservationReadFailure, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return observationFailure(ObservationUnsafePath, current, fmt.Errorf("manifest parent is not a real directory"))
		}
	}
	return nil
}

func stageCanonicalCarrier(path string, canonical []byte) (string, error) {
	parent := filepath.Dir(path)
	stageName, err := newCanonicalCarrierStageName(filepath.Base(path))
	if err != nil {
		return "", err
	}
	stagePath := filepath.Join(parent, stageName)
	stage, err := os.OpenFile(
		stagePath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return "", fmt.Errorf("create manifest stage: %w", err)
	}
	cleanup := true
	defer func() {
		_ = stage.Close()
		if cleanup {
			_ = os.Remove(stagePath)
		}
	}()
	if _, err := stage.Write(canonical); err != nil {
		return "", fmt.Errorf("write manifest stage: %w", err)
	}
	if err := stage.Chmod(0o644); err != nil {
		return "", fmt.Errorf("chmod manifest stage: %w", err)
	}
	if err := stage.Sync(); err != nil {
		return "", fmt.Errorf("sync manifest stage: %w", err)
	}
	if err := stage.Close(); err != nil {
		return "", fmt.Errorf("close manifest stage: %w", err)
	}
	cleanup = false
	return stagePath, nil
}

const maximumCanonicalCarrierStageDebt = 64

func newCanonicalCarrierStageName(base string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate canonical carrier stage name: %w", err)
	}
	return canonicalCarrierStagePrefix(base) + hex.EncodeToString(random[:]), nil
}

func canonicalCarrierStagePrefix(base string) string {
	return "." + base + ".stage-"
}

func validCanonicalCarrierStageName(
	name string,
	base string,
) bool {
	suffix, found := strings.CutPrefix(name, canonicalCarrierStagePrefix(base))
	if !found || len(suffix) != 32 {
		return false
	}
	_, err := hex.DecodeString(suffix)
	return err == nil && suffix == strings.ToLower(suffix)
}

func reconcileCanonicalCarrierStages(store ManifestStore) error {
	parent := filepath.Dir(store.path)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return fmt.Errorf("enumerate canonical carrier stages: %w", err)
	}
	manifestBase := filepath.Base(store.path)
	journalBase := filepath.Base(store.JournalPath())
	candidates := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		owned := validCanonicalCarrierStageName(name, manifestBase)
		owned = owned || validCanonicalCarrierStageName(name, journalBase)
		if !owned {
			continue
		}
		candidates = append(candidates, filepath.Join(parent, name))
	}
	if len(candidates) > maximumCanonicalCarrierStageDebt {
		return fmt.Errorf(
			"canonical carrier stage debt %d exceeds limit %d",
			len(candidates),
			maximumCanonicalCarrierStageDebt,
		)
	}
	for _, path := range candidates {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect canonical carrier stage %s: %w", path, err)
		}
		regular := info.Mode().IsRegular()
		safeMode := info.Mode().Perm() == 0o600 || info.Mode().Perm() == 0o644
		safeMode = safeMode &&
			info.Mode()&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0
		if !regular || !safeMode {
			return fmt.Errorf("canonical carrier stage %s is not a safe regular file", path)
		}
	}
	sort.Strings(candidates)
	for _, path := range candidates {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale canonical carrier stage %s: %w", path, err)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return syncDirectory(parent)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open manifest directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync manifest directory: %w", err)
	}
	return nil
}
