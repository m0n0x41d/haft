// Package initfs contains the filesystem effect boundary for initialization.
package initfs

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type ObservationFailureKind string

const (
	ObservationInvalidPlan     ObservationFailureKind = "invalid_plan"
	ObservationUnsafePath      ObservationFailureKind = "unsafe_path"
	ObservationUnsupportedFile ObservationFailureKind = "unsupported_file"
	ObservationFileTooLarge    ObservationFailureKind = "file_too_large"
	ObservationUnstable        ObservationFailureKind = "unstable_snapshot"
	ObservationReadFailure     ObservationFailureKind = "read_failure"
)

type ObservationFailure struct {
	kind  ObservationFailureKind
	path  string
	cause error
}

func (failure ObservationFailure) Error() string {
	if failure.cause == nil {
		return fmt.Sprintf("observe %s: %s", failure.path, failure.kind)
	}
	return fmt.Sprintf("observe %s: %s: %v", failure.path, failure.kind, failure.cause)
}

func (failure ObservationFailure) Unwrap() error {
	return failure.cause
}

func (failure ObservationFailure) Kind() ObservationFailureKind {
	return failure.kind
}

func (failure ObservationFailure) Path() string {
	return failure.path
}

type FileObserver struct {
	maxFileBytes int64
}

func NewFileObserver(maxFileBytes int64) (FileObserver, error) {
	if maxFileBytes <= 0 {
		return FileObserver{}, fmt.Errorf("filesystem observer byte limit must be positive")
	}
	return FileObserver{maxFileBytes: maxFileBytes}, nil
}

func (observer FileObserver) Observe(
	plan initplanning.InstallationObservationPlan,
) ([]initplanning.PathObservation, error) {
	roots := plan.ManagedRoots()
	targets := plan.Targets()
	if observer.maxFileBytes <= 0 || len(roots) == 0 || len(targets) == 0 {
		return nil, ObservationFailure{kind: ObservationInvalidPlan, path: "<installation>"}
	}
	observations := make([]initplanning.PathObservation, 0, len(targets))
	for _, target := range targets {
		observation, emitted, err := observer.observeTarget(roots, target)
		if err != nil {
			return nil, err
		}
		if emitted {
			observations = append(observations, observation)
		}
	}
	sort.Slice(observations, func(left int, right int) bool {
		return observations[left].Path() < observations[right].Path()
	})
	return observations, nil
}

func (observer FileObserver) ObserveManagedCarrier(
	plan initplanning.ManagedFragmentObservationPlan,
	managedRoots []string,
) (initplanning.ManagedCarrierInput, error) {
	path := plan.CarrierPath()
	if observer.maxFileBytes <= 0 ||
		path == "" ||
		len(managedRoots) == 0 {
		return initplanning.ManagedCarrierInput{}, ObservationFailure{
			kind: ObservationInvalidPlan,
			path: path,
		}
	}
	root, err := containingManagedRoot(path, managedRoots)
	if err != nil {
		return initplanning.ManagedCarrierInput{}, observationFailure(
			ObservationInvalidPlan,
			path,
			err,
		)
	}
	info, missing, err := lstatWithoutManagedRootSymlinks(root, path)
	if err != nil {
		return initplanning.ManagedCarrierInput{}, err
	}
	if missing {
		input, err := initplanning.NewMissingManagedCarrier(path)
		if err != nil {
			return initplanning.ManagedCarrierInput{}, observationFailure(
				ObservationReadFailure,
				path,
				err,
			)
		}
		return input, nil
	}
	content, mode, err := observer.readStableRegularFile(path, info)
	if err != nil {
		return initplanning.ManagedCarrierInput{}, err
	}
	input, err := initplanning.NewPresentManagedCarrier(
		path,
		content,
		mode.Perm(),
	)
	if err != nil {
		return initplanning.ManagedCarrierInput{}, observationFailure(
			ObservationReadFailure,
			path,
			err,
		)
	}
	return input, nil
}

func (observer FileObserver) observeTarget(
	roots []string,
	target initplanning.ObservationTarget,
) (initplanning.PathObservation, bool, error) {
	root, err := containingManagedRoot(target.Path(), roots)
	if err != nil {
		return initplanning.PathObservation{}, false, observationFailure(
			ObservationInvalidPlan,
			target.Path(),
			err,
		)
	}
	info, missing, err := lstatWithoutManagedRootSymlinks(root, target.Path())
	if err != nil {
		return initplanning.PathObservation{}, false, err
	}
	if missing && target.Requirement() == initplanning.ObservationIfPresent {
		return initplanning.PathObservation{}, false, nil
	}
	if missing {
		observation, err := initplanning.ObserveMissingPathForComponents(
			target.Path(),
			target.Components(),
		)
		return observation, true, err
	}
	digest, mode, err := observer.digestStableRegularFile(target.Path(), info)
	if err != nil {
		return initplanning.PathObservation{}, false, err
	}
	observation, err := initplanning.ObservePresentPathForComponents(
		target.Path(),
		target.Components(),
		digest,
		mode.Perm(),
	)
	if err != nil {
		return initplanning.PathObservation{}, false, observationFailure(
			ObservationReadFailure,
			target.Path(),
			err,
		)
	}
	return observation, true, nil
}

func containingManagedRoot(path string, roots []string) (string, error) {
	candidates := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return "", fmt.Errorf("managed root is not canonical")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || filepath.IsAbs(relative) || relative == ".." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		candidates = append(candidates, root)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("target is outside managed roots")
	}
	sort.Slice(candidates, func(left int, right int) bool {
		return len(candidates[left]) > len(candidates[right])
	})
	return candidates[0], nil
}

func lstatWithoutManagedRootSymlinks(
	root string,
	path string,
) (os.FileInfo, bool, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, false, observationFailure(ObservationInvalidPlan, path, err)
	}
	rootInfo, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, observationFailure(ObservationReadFailure, root, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, false, observationFailure(ObservationUnsafePath, root, fmt.Errorf("managed root is a symbolic link"))
	}
	segments := pathSegments(relative)
	if len(segments) == 0 {
		return rootInfo, false, nil
	}
	if !rootInfo.IsDir() {
		return nil, false, observationFailure(ObservationUnsupportedFile, root, fmt.Errorf("managed root is not a directory"))
	}
	parent := root
	for _, segment := range segments[:len(segments)-1] {
		parent = filepath.Join(parent, segment)
		info, missing, err := lstatRequiredDirectory(parent)
		if err != nil {
			return nil, false, err
		}
		if missing {
			return nil, true, nil
		}
		if !info.IsDir() {
			return nil, false, observationFailure(ObservationUnsupportedFile, parent, fmt.Errorf("path ancestor is not a directory"))
		}
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, observationFailure(ObservationReadFailure, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, observationFailure(ObservationUnsafePath, path, fmt.Errorf("target is a symbolic link"))
	}
	return info, false, nil
}

func pathSegments(relative string) []string {
	if relative == "." || relative == "" {
		return nil
	}
	return strings.Split(relative, string(filepath.Separator))
}

func lstatRequiredDirectory(path string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, observationFailure(ObservationReadFailure, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, observationFailure(ObservationUnsafePath, path, fmt.Errorf("path ancestor is a symbolic link"))
	}
	return info, false, nil
}

func (observer FileObserver) digestStableRegularFile(
	path string,
	pathInfo os.FileInfo,
) (string, os.FileMode, error) {
	content, mode, err := observer.readStableRegularFile(path, pathInfo)
	if err != nil {
		return "", 0, err
	}
	digest := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", digest), mode, nil
}

func (observer FileObserver) readStableRegularFile(
	path string,
	pathInfo os.FileInfo,
) ([]byte, os.FileMode, error) {
	if !pathInfo.Mode().IsRegular() {
		return nil, 0, observationFailure(ObservationUnsupportedFile, path, fmt.Errorf("target is not a regular file"))
	}
	if pathInfo.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return nil, 0, observationFailure(ObservationUnsupportedFile, path, fmt.Errorf("target has special permission bits"))
	}
	if pathInfo.Size() > observer.maxFileBytes {
		return nil, 0, observationFailure(ObservationFileTooLarge, path, fmt.Errorf("file exceeds %d bytes", observer.maxFileBytes))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, observationFailure(ObservationReadFailure, path, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, 0, observationFailure(ObservationReadFailure, path, err)
	}
	if !os.SameFile(pathInfo, openedInfo) {
		return nil, 0, observationFailure(ObservationUnstable, path, fmt.Errorf("path changed before read"))
	}
	content := bytes.NewBuffer(make([]byte, 0, openedInfo.Size()))
	limited := &io.LimitedReader{R: file, N: observer.maxFileBytes + 1}
	readBytes, err := io.Copy(content, limited)
	if err != nil {
		return nil, 0, observationFailure(ObservationReadFailure, path, err)
	}
	if readBytes > observer.maxFileBytes {
		return nil, 0, observationFailure(ObservationFileTooLarge, path, fmt.Errorf("file grew beyond %d bytes", observer.maxFileBytes))
	}
	openedAfter, err := file.Stat()
	if err != nil {
		return nil, 0, observationFailure(ObservationReadFailure, path, err)
	}
	pathAfter, err := os.Lstat(path)
	if err != nil {
		return nil, 0, observationFailure(ObservationUnstable, path, err)
	}
	if !stableFileSnapshot(pathInfo, openedInfo, openedAfter, pathAfter) {
		return nil, 0, observationFailure(ObservationUnstable, path, fmt.Errorf("file changed while observed"))
	}
	return content.Bytes(), openedAfter.Mode(), nil
}

func stableFileSnapshot(
	pathBefore os.FileInfo,
	openedBefore os.FileInfo,
	openedAfter os.FileInfo,
	pathAfter os.FileInfo,
) bool {
	return os.SameFile(pathBefore, openedBefore) &&
		os.SameFile(openedBefore, openedAfter) &&
		os.SameFile(openedAfter, pathAfter) &&
		openedBefore.Size() == openedAfter.Size() &&
		openedBefore.Mode() == openedAfter.Mode() &&
		openedBefore.ModTime().Equal(openedAfter.ModTime())
}

func observationFailure(
	kind ObservationFailureKind,
	path string,
	cause error,
) ObservationFailure {
	return ObservationFailure{kind: kind, path: path, cause: cause}
}
