package initfs

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"sync"
)

const publicationResourceSetSchema = "haft.publication-resource-set/v1"

type PublicationCoordinationKind string

const (
	PublicationCoordinationAcquired PublicationCoordinationKind = "acquired"
	PublicationCoordinationBusy     PublicationCoordinationKind = "busy"
)

type publicationResourceSetWire struct {
	Schema    string   `json:"schema"`
	Resources []string `json:"resources"`
}

type PublicationCoordinator struct {
	root     string
	lockPath string
}

func NewPublicationCoordinator(
	root string,
	lockPath string,
) (PublicationCoordinator, error) {
	if root == "" ||
		!filepath.IsAbs(root) ||
		filepath.Clean(root) != root {
		return PublicationCoordinator{}, fmt.Errorf(
			"publication coordination root is not canonical",
		)
	}
	selectedRoot, err := containingManagedRoot(lockPath, []string{root})
	if err != nil || selectedRoot != root || lockPath == root {
		return PublicationCoordinator{}, fmt.Errorf(
			"publication coordination lock is outside its root",
		)
	}
	return PublicationCoordinator{
		root:     root,
		lockPath: lockPath,
	}, nil
}

func (coordinator PublicationCoordinator) Root() string {
	return coordinator.root
}

func (coordinator PublicationCoordinator) LockPath() string {
	return coordinator.lockPath
}

type PublicationCoordinationAttempt struct {
	kind           PublicationCoordinationKind
	lockPath       string
	resourceDigest string
	resources      []string
	lease          *PublicationCoordinationLease
}

func (coordinator PublicationCoordinator) TryAcquire(
	resources []string,
) (PublicationCoordinationAttempt, error) {
	resourceSet, digest, err := canonicalPublicationResourceSet(resources)
	if err != nil {
		return PublicationCoordinationAttempt{}, err
	}
	if err := coordinator.valid(); err != nil {
		return PublicationCoordinationAttempt{}, err
	}
	parent := filepath.Dir(coordinator.lockPath)
	if err := ensureDirectoryTreeWithoutSymlinks(
		coordinator.root,
		parent,
	); err != nil {
		return PublicationCoordinationAttempt{}, err
	}
	lock, acquired, err := tryAcquireManifestLock(coordinator.lockPath)
	if err != nil {
		return PublicationCoordinationAttempt{}, err
	}
	attempt := PublicationCoordinationAttempt{
		kind:           PublicationCoordinationBusy,
		lockPath:       coordinator.lockPath,
		resourceDigest: digest,
		resources:      resourceSet,
	}
	if !acquired {
		return attempt, nil
	}
	attempt.kind = PublicationCoordinationAcquired
	attempt.lease = &PublicationCoordinationLease{
		lockPath:       coordinator.lockPath,
		resourceDigest: digest,
		resources:      slices.Clone(resourceSet),
		lock:           lock,
	}
	return attempt, nil
}

func (coordinator PublicationCoordinator) valid() error {
	if coordinator.root == "" ||
		coordinator.lockPath == "" ||
		coordinator.root == coordinator.lockPath {
		return fmt.Errorf("publication coordinator is invalid")
	}
	selectedRoot, err := containingManagedRoot(
		coordinator.lockPath,
		[]string{coordinator.root},
	)
	if err != nil || selectedRoot != coordinator.root {
		return fmt.Errorf("publication coordinator binding is invalid")
	}
	return nil
}

func canonicalPublicationResourceSet(
	raw []string,
) ([]string, string, error) {
	if len(raw) == 0 {
		return nil, "", fmt.Errorf(
			"publication coordination resource set is empty",
		)
	}
	seen := make(map[string]struct{}, len(raw))
	resources := make([]string, 0, len(raw))
	for _, resource := range raw {
		if resource == "" ||
			!filepath.IsAbs(resource) ||
			filepath.Clean(resource) != resource {
			return nil, "", fmt.Errorf(
				"publication coordination resource is not canonical",
			)
		}
		volumeRoot := filepath.VolumeName(resource) +
			string(filepath.Separator)
		if resource == volumeRoot {
			return nil, "", fmt.Errorf(
				"filesystem root cannot be a publication resource",
			)
		}
		if _, duplicate := seen[resource]; duplicate {
			continue
		}
		seen[resource] = struct{}{}
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	canonical, err := json.Marshal(publicationResourceSetWire{
		Schema:    publicationResourceSetSchema,
		Resources: resources,
	})
	if err != nil {
		return nil, "", fmt.Errorf(
			"encode publication resource set: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	return resources, fmt.Sprintf("sha256:%x", sum), nil
}

func (attempt PublicationCoordinationAttempt) Kind() PublicationCoordinationKind {
	return attempt.kind
}

func (attempt PublicationCoordinationAttempt) LockPath() string {
	return attempt.lockPath
}

func (attempt PublicationCoordinationAttempt) ResourceDigest() string {
	return attempt.resourceDigest
}

func (attempt PublicationCoordinationAttempt) Resources() []string {
	return slices.Clone(attempt.resources)
}

func (attempt PublicationCoordinationAttempt) Lease() (
	*PublicationCoordinationLease,
	bool,
) {
	if attempt.kind != PublicationCoordinationAcquired ||
		attempt.lease == nil {
		return nil, false
	}
	return attempt.lease, true
}

type PublicationCoordinationLease struct {
	mu             sync.Mutex
	lockPath       string
	resourceDigest string
	resources      []string
	lock           *manifestLock
	released       bool
}

func (lease *PublicationCoordinationLease) LockPath() string {
	if lease == nil {
		return ""
	}
	return lease.lockPath
}

func (lease *PublicationCoordinationLease) ResourceDigest() string {
	if lease == nil {
		return ""
	}
	return lease.resourceDigest
}

func (lease *PublicationCoordinationLease) Resources() []string {
	if lease == nil {
		return nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return slices.Clone(lease.resources)
}

func (lease *PublicationCoordinationLease) RequireCoverage(
	resources []string,
) error {
	if lease == nil {
		return fmt.Errorf("publication coordination lease is nil")
	}
	required, _, err := canonicalPublicationResourceSet(resources)
	if err != nil {
		return err
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released ||
		lease.lock == nil ||
		lease.lockPath == "" ||
		!validSHA256Digest(lease.resourceDigest) ||
		len(lease.resources) == 0 {
		return fmt.Errorf("publication coordination lease is not active")
	}
	for _, resource := range required {
		if !slices.Contains(lease.resources, resource) {
			return fmt.Errorf(
				"publication coordination lease does not cover %s",
				resource,
			)
		}
	}
	return nil
}

func (lease *PublicationCoordinationLease) Release() error {
	if lease == nil {
		return fmt.Errorf("publication coordination lease is nil")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released {
		return nil
	}
	if lease.lock == nil ||
		lease.lockPath == "" ||
		!validSHA256Digest(lease.resourceDigest) ||
		len(lease.resources) == 0 {
		return fmt.Errorf("publication coordination lease is invalid")
	}
	if err := lease.lock.release(); err != nil {
		return fmt.Errorf(
			"release publication coordination lock %s: %w",
			lease.lockPath,
			err,
		)
	}
	lease.lock = nil
	lease.released = true
	return nil
}
