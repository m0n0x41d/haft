package fpfrefresh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	fpfSourceCommitEpochEnvironment = "HAFT_FPF_SOURCE_COMMIT_EPOCH"
	maxIndexerOutputBytes           = 8 << 10
)

// ExecutableIndexBuilder adapts the repository's indexer executable to the
// candidate preparation port. SourceRepositoryPath is consulted only for the
// exact pinned commit timestamp; publications still come exclusively from the
// immutable GitSourceSnapshot materialized in the private workspace.
type ExecutableIndexBuilder struct {
	ExecutablePath       string
	SourceRepositoryPath string
}

func (builder ExecutableIndexBuilder) BuildIndex(
	ctx context.Context,
	input IndexBuildInput,
) error {
	if !filepath.IsAbs(builder.ExecutablePath) {
		return fmt.Errorf("indexer executable path must be absolute")
	}
	info, err := os.Stat(builder.ExecutablePath)
	if err != nil {
		return fmt.Errorf("inspect indexer executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("indexer executable is not an executable regular file")
	}
	if !filepath.IsAbs(builder.SourceRepositoryPath) {
		return fmt.Errorf("source repository path must be absolute")
	}
	commitEpoch, err := exactSourceCommitEpoch(
		ctx,
		builder.SourceRepositoryPath,
		input.SourceRevision,
	)
	if err != nil {
		return err
	}

	command := exec.CommandContext(
		ctx,
		builder.ExecutablePath,
		input.SpecificationPath,
		input.DatabasePath,
		input.SourceRevision,
	)
	command.Dir = input.WorkingDirectory
	command.Env = replaceProcessEnvironment(
		os.Environ(),
		fpfSourceCommitEpochEnvironment,
		commitEpoch,
	)
	var output boundedIndexerOutput
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		detail := output.Detail()
		if detail == "" {
			return fmt.Errorf("run candidate indexer: %w", err)
		}
		return fmt.Errorf(
			"run candidate indexer: %w: %s",
			err,
			detail,
		)
	}
	return nil
}

// boundedIndexerOutput retains only the final diagnostic bytes because the
// indexer's actionable failure is conventionally emitted at process exit.
// Write still reports the full payload length so os/exec does not treat the
// intentional truncation as a short write.
type boundedIndexerOutput struct {
	tail      []byte
	truncated bool
}

func (output *boundedIndexerOutput) Write(payload []byte) (int, error) {
	originalLength := len(payload)
	if originalLength == 0 {
		return 0, nil
	}
	if originalLength >= maxIndexerOutputBytes {
		output.truncated = output.truncated ||
			len(output.tail) > 0 ||
			originalLength > maxIndexerOutputBytes
		output.tail = append(
			output.tail[:0],
			payload[originalLength-maxIndexerOutputBytes:]...,
		)
		return originalLength, nil
	}

	overflow := len(output.tail) + originalLength - maxIndexerOutputBytes
	if overflow > 0 {
		copy(output.tail, output.tail[overflow:])
		output.tail = output.tail[:len(output.tail)-overflow]
		output.truncated = true
	}
	output.tail = append(output.tail, payload...)
	return originalLength, nil
}

func (output *boundedIndexerOutput) Detail() string {
	detail := strings.TrimSpace(string(output.tail))
	if !output.truncated {
		return detail
	}
	if detail == "" {
		return "[output truncated]"
	}
	return "[output truncated]\n" + detail
}

func exactSourceCommitEpoch(
	ctx context.Context,
	repositoryPath string,
	revision string,
) (string, error) {
	if !fullGitCommitSHAPattern.MatchString(revision) {
		return "", fmt.Errorf("candidate source revision is not an exact commit SHA")
	}
	output, err := runRepositoryGit(
		ctx,
		repositoryPath,
		"show",
		"-s",
		"--format=%ct",
		revision,
	)
	if err != nil {
		return "", fmt.Errorf("read candidate source commit epoch: %w", err)
	}
	value := strings.TrimSpace(string(output))
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 {
		return "", fmt.Errorf("candidate source commit epoch %q is invalid", value)
	}
	return strconv.FormatInt(seconds, 10), nil
}

func replaceProcessEnvironment(
	environment []string,
	key string,
	value string,
) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, prefix+value)
}
