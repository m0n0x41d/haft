package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/m0n0x41d/haft/internal/project"
)

type publicExactFileEffectKind string

const (
	publicExactFileCreate   publicExactFileEffectKind = "create"
	publicExactFilePreserve publicExactFileEffectKind = "preserve"
	publicExactFileReplace  publicExactFileEffectKind = "replace"
	publicExactFileRemove   publicExactFileEffectKind = "remove"
)

type publicExactFileEffect struct {
	kind           publicExactFileEffectKind
	path           string
	content        []byte
	mode           fs.FileMode
	renderedDigest string
	expectedDigest string
	expectedMode   fs.FileMode
}

type publicExactFileReceipt struct {
	completed []string
	failed    string
	untouched []string
	retry     []string
	recovery  []string
}

func (receipt publicExactFileReceipt) Completed() []string {
	return slices.Clone(receipt.completed)
}

func (receipt publicExactFileReceipt) Failed() string {
	return receipt.failed
}

func (receipt publicExactFileReceipt) Untouched() []string {
	return slices.Clone(receipt.untouched)
}

func (receipt publicExactFileReceipt) Retry() []string {
	return slices.Clone(receipt.retry)
}

func (receipt publicExactFileReceipt) Recovery() []string {
	return slices.Clone(receipt.recovery)
}

func planPublicExactFile(
	path string,
	content []byte,
	mode fs.FileMode,
) (publicExactFileEffect, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return publicExactFileEffect{}, fmt.Errorf(
			"public exact file path must be canonical and absolute",
		)
	}
	if mode == 0 || mode.Perm() != mode {
		return publicExactFileEffect{}, fmt.Errorf(
			"public exact file mode is invalid",
		)
	}
	renderedDigest := publicContentDigest(content)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return publicExactFileEffect{
			kind:           publicExactFileCreate,
			path:           path,
			content:        slices.Clone(content),
			mode:           mode,
			renderedDigest: renderedDigest,
		}, nil
	}
	if err != nil {
		return publicExactFileEffect{}, fmt.Errorf(
			"inspect public exact file %s: %w",
			path,
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return publicExactFileEffect{}, fmt.Errorf(
			"public exact file %s is not a regular file",
			path,
		)
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return publicExactFileEffect{}, fmt.Errorf(
			"read public exact file %s: %w",
			path,
			err,
		)
	}
	expectedDigest := publicContentDigest(existing)
	kind := publicExactFileReplace
	if expectedDigest == renderedDigest &&
		info.Mode().Perm() == mode.Perm() {
		kind = publicExactFilePreserve
	}
	return publicExactFileEffect{
		kind:           kind,
		path:           path,
		content:        slices.Clone(content),
		mode:           mode,
		renderedDigest: renderedDigest,
		expectedDigest: expectedDigest,
		expectedMode:   info.Mode().Perm(),
	}, nil
}

func verifyPublicExactFile(effect publicExactFileEffect) error {
	if effect.kind == publicExactFileRemove {
		return verifyPublicExactFileRemoval(effect)
	}
	observed, err := planPublicExactFile(
		effect.path,
		effect.content,
		effect.mode,
	)
	if err != nil {
		return err
	}
	exact := observed.kind == effect.kind
	exact = exact &&
		observed.expectedDigest == effect.expectedDigest
	exact = exact &&
		observed.expectedMode == effect.expectedMode
	if !exact {
		return fmt.Errorf(
			"public exact file %s changed after preview; no ancillary files were written",
			effect.path,
		)
	}
	return nil
}

func verifyPublicExactFileRemoval(effect publicExactFileEffect) error {
	info, err := os.Lstat(effect.path)
	if err != nil {
		return fmt.Errorf(
			"inspect obsolete project file %s: %w",
			effect.path,
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf(
			"obsolete project file %s is no longer regular",
			effect.path,
		)
	}
	content, err := os.ReadFile(effect.path)
	if err != nil {
		return fmt.Errorf(
			"read obsolete project file %s: %w",
			effect.path,
			err,
		)
	}
	observedDigest := publicContentDigest(content)
	exact := observedDigest == effect.expectedDigest
	exact = exact && info.Mode().Perm() == effect.expectedMode.Perm()
	if !exact {
		return fmt.Errorf(
			"obsolete project file %s changed after preview; it was preserved",
			effect.path,
		)
	}
	if !project.IsKnownGeneratedLegacyProjectConfigDigest(observedDigest) {
		return fmt.Errorf(
			"obsolete project file %s is not an exact Haft-generated carrier; it was preserved",
			effect.path,
		)
	}
	return nil
}

func writePublicExactFile(effect publicExactFileEffect) error {
	if effect.kind == publicExactFilePreserve {
		return nil
	}
	if effect.kind == publicExactFileRemove {
		if err := os.Remove(effect.path); err != nil {
			return fmt.Errorf(
				"remove obsolete project file %s: %w",
				effect.path,
				err,
			)
		}
		return syncPublicExactFileDirectory(filepath.Dir(effect.path))
	}
	parent := filepath.Dir(effect.path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf(
			"create public exact file parent %s: %w",
			parent,
			err,
		)
	}
	stage, err := os.CreateTemp(parent, ".haft-init-stage-*")
	if err != nil {
		return fmt.Errorf(
			"stage public exact file %s: %w",
			effect.path,
			err,
		)
	}
	stagePath := stage.Name()
	writeErr := stage.Chmod(effect.mode)
	if writeErr == nil {
		_, writeErr = stage.Write(effect.content)
	}
	if writeErr == nil {
		writeErr = stage.Sync()
	}
	closeErr := stage.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(stagePath)
		if writeErr != nil {
			return fmt.Errorf(
				"write staged public exact file %s: %w",
				effect.path,
				writeErr,
			)
		}
		return fmt.Errorf(
			"close staged public exact file %s: %w",
			effect.path,
			closeErr,
		)
	}
	if err := os.Rename(stagePath, effect.path); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"publish public exact file %s: %w",
			effect.path,
			err,
		)
	}
	return nil
}

func syncPublicExactFileDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open public exact file directory %s: %w", path, err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync public exact file directory %s: %w", path, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close public exact file directory %s: %w", path, closeErr)
	}
	return nil
}

func applyPublicExactFileEffects(
	ctx context.Context,
	effects []publicExactFileEffect,
	recovery []string,
) (publicExactFileReceipt, error) {
	if ctx == nil {
		return publicExactFileReceipt{},
			fmt.Errorf("public exact file context is required")
	}
	if len(effects) == 0 {
		return publicExactFileReceipt{
			recovery: slices.Clone(recovery),
		}, nil
	}
	for _, effect := range effects {
		if err := verifyPublicExactFile(effect); err != nil {
			return publicExactFileReceipt{
				untouched: publicExactFilePaths(effects),
				retry:     publicExactFilePaths(effects),
				recovery:  slices.Clone(recovery),
			}, err
		}
	}
	completed := make([]string, 0, len(effects))
	for index, effect := range effects {
		if err := writePublicExactFile(effect); err != nil {
			pending := publicExactFilePaths(effects[index:])
			return publicExactFileReceipt{
				completed: slices.Clone(completed),
				failed:    effect.path,
				untouched: publicExactFilePaths(effects[index+1:]),
				retry:     pending,
				recovery:  slices.Clone(recovery),
			}, err
		}
		completed = append(completed, effect.path)
	}
	return publicExactFileReceipt{
		completed: completed,
		recovery:  slices.Clone(recovery),
	}, nil
}

func publicExactFilePaths(
	effects []publicExactFileEffect,
) []string {
	paths := make([]string, len(effects))
	for index, effect := range effects {
		paths[index] = effect.path
	}
	return paths
}

func publicContentDigest(content []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}
