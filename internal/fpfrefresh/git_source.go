package fpfrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const (
	gitSourceReadmePath = "Readme.md"
	gitSourceSpecPath   = "FPF-Spec.md"
)

var (
	// ErrGitSourceMissing identifies a candidate commit that does not contain a
	// required FPF publication.
	ErrGitSourceMissing = errors.New("candidate Git source missing")

	// ErrGitSourceMalformed identifies a required publication that is not a
	// non-empty Git blob. Markdown grammar validation belongs to internal/fpf.
	ErrGitSourceMalformed = errors.New("candidate Git source malformed")

	fullGitCommitSHAPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// GitFetchRequest makes network/ref mutation an explicit caller choice. A nil
// Fetch on GitSourceRequest performs no fetch.
type GitFetchRequest struct {
	Remote   string
	RefSpecs []string
}

// GitSourceRequest identifies one candidate ref in one Git repository. The ref
// is resolved once after the optional fetch and is never consulted while the
// candidate publications are read.
type GitSourceRequest struct {
	RepositoryPath string
	CandidateRef   string
	Fetch          *GitFetchRequest
}

// GitSourceSnapshot owns the exact publication bytes read from one immutable
// commit. Accessors return copies so later ref movement or caller mutation
// cannot change the acquired source basis.
type GitSourceSnapshot struct {
	candidateRef       string
	commitSHA          string
	readmeBytes        []byte
	specificationBytes []byte
}

func (snapshot GitSourceSnapshot) CandidateRef() string {
	return snapshot.candidateRef
}

func (snapshot GitSourceSnapshot) CommitSHA() string {
	return snapshot.commitSHA
}

func (snapshot GitSourceSnapshot) ReadmeBytes() []byte {
	return bytes.Clone(snapshot.readmeBytes)
}

func (snapshot GitSourceSnapshot) SpecificationBytes() []byte {
	return bytes.Clone(snapshot.specificationBytes)
}

// CandidateRefObservation reports whether a separately observed ref still
// names an already pinned commit. It does not replace or mutate that pin.
type CandidateRefObservation struct {
	CandidateRef      string
	PinnedCommitSHA   string
	ObservedCommitSHA string
	Moved             bool
}

// AcquireGitSource optionally fetches only when request.Fetch is non-nil,
// resolves request.CandidateRef once to a full commit SHA, and reads both FPF
// publications directly from that commit's object graph. It never checks out,
// resets, stashes, or reads working-tree publication bytes.
func AcquireGitSource(
	ctx context.Context,
	request GitSourceRequest,
) (GitSourceSnapshot, error) {
	return acquireGitSource(ctx, commandGitSourceRunner{}, request)
}

// ObserveCandidateRef is a separate drift observation. An optional fetch is
// again explicit through request.Fetch. The returned observation never repins
// a previously acquired GitSourceSnapshot.
func ObserveCandidateRef(
	ctx context.Context,
	request GitSourceRequest,
	pinnedCommitSHA string,
) (CandidateRefObservation, error) {
	return observeCandidateRef(ctx, commandGitSourceRunner{}, request, pinnedCommitSHA)
}

type gitSourceRunner interface {
	Run(ctx context.Context, repositoryPath string, args ...string) ([]byte, error)
}

type commandGitSourceRunner struct{}

func (commandGitSourceRunner) Run(
	ctx context.Context,
	repositoryPath string,
	args ...string,
) ([]byte, error) {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", repositoryPath)
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return nil, fmt.Errorf("run Git command: %w", err)
	}
	return nil, fmt.Errorf("run Git command: %w: %s", err, detail)
}

func acquireGitSource(
	ctx context.Context,
	runner gitSourceRunner,
	request GitSourceRequest,
) (GitSourceSnapshot, error) {
	normalized, err := validateGitSourceRequest(request)
	if err != nil {
		return GitSourceSnapshot{}, err
	}
	if err := fetchGitSource(ctx, runner, normalized.RepositoryPath, normalized.Fetch); err != nil {
		return GitSourceSnapshot{}, err
	}
	commitSHA, err := resolveCandidateCommit(
		ctx,
		runner,
		normalized.RepositoryPath,
		normalized.CandidateRef,
	)
	if err != nil {
		return GitSourceSnapshot{}, err
	}
	snapshot := GitSourceSnapshot{
		candidateRef: normalized.CandidateRef,
		commitSHA:    commitSHA,
	}
	readme, err := readCandidatePublication(
		ctx,
		runner,
		normalized.RepositoryPath,
		commitSHA,
		gitSourceReadmePath,
	)
	if err != nil {
		return snapshot, err
	}
	snapshot.readmeBytes = bytes.Clone(readme)
	specification, err := readCandidatePublication(
		ctx,
		runner,
		normalized.RepositoryPath,
		commitSHA,
		gitSourceSpecPath,
	)
	if err != nil {
		return snapshot, err
	}
	snapshot.specificationBytes = bytes.Clone(specification)
	return snapshot, nil
}

func observeCandidateRef(
	ctx context.Context,
	runner gitSourceRunner,
	request GitSourceRequest,
	pinnedCommitSHA string,
) (CandidateRefObservation, error) {
	normalized, err := validateGitSourceRequest(request)
	if err != nil {
		return CandidateRefObservation{}, err
	}
	if !fullGitCommitSHAPattern.MatchString(pinnedCommitSHA) {
		return CandidateRefObservation{}, fmt.Errorf(
			"observe candidate ref: pinned commit SHA must be 40 or 64 lowercase hexadecimal characters",
		)
	}
	if err := fetchGitSource(ctx, runner, normalized.RepositoryPath, normalized.Fetch); err != nil {
		return CandidateRefObservation{}, err
	}
	observedCommitSHA, err := resolveCandidateCommit(
		ctx,
		runner,
		normalized.RepositoryPath,
		normalized.CandidateRef,
	)
	if err != nil {
		return CandidateRefObservation{}, err
	}
	return CandidateRefObservation{
		CandidateRef:      normalized.CandidateRef,
		PinnedCommitSHA:   pinnedCommitSHA,
		ObservedCommitSHA: observedCommitSHA,
		Moved:             observedCommitSHA != pinnedCommitSHA,
	}, nil
}

func validateGitSourceRequest(request GitSourceRequest) (GitSourceRequest, error) {
	if err := validateGitSourceText("repository path", request.RepositoryPath); err != nil {
		return GitSourceRequest{}, fmt.Errorf("invalid Git source request: %w", err)
	}
	if err := validateGitSourceText("candidate ref", request.CandidateRef); err != nil {
		return GitSourceRequest{}, fmt.Errorf("invalid Git source request: %w", err)
	}
	normalized := GitSourceRequest{
		RepositoryPath: request.RepositoryPath,
		CandidateRef:   request.CandidateRef,
	}
	if request.Fetch == nil {
		return normalized, nil
	}
	if err := validateGitSourceText("fetch remote", request.Fetch.Remote); err != nil {
		return GitSourceRequest{}, fmt.Errorf("invalid Git source request: %w", err)
	}
	if strings.HasPrefix(request.Fetch.Remote, "-") {
		return GitSourceRequest{}, fmt.Errorf(
			"invalid Git source request: fetch remote must not begin with '-'",
		)
	}
	refSpecs := make([]string, len(request.Fetch.RefSpecs))
	for index, refSpec := range request.Fetch.RefSpecs {
		if err := validateGitSourceText("fetch refspec", refSpec); err != nil {
			return GitSourceRequest{}, fmt.Errorf("invalid Git source request: %w", err)
		}
		if strings.HasPrefix(refSpec, "-") {
			return GitSourceRequest{}, fmt.Errorf(
				"invalid Git source request: fetch refspec must not begin with '-'",
			)
		}
		refSpecs[index] = refSpec
	}
	normalized.Fetch = &GitFetchRequest{
		Remote:   request.Fetch.Remote,
		RefSpecs: refSpecs,
	}
	return normalized, nil
}

func validateGitSourceText(field string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not contain surrounding whitespace", field)
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("%s must be one line without NUL bytes", field)
	}
	return nil
}

func fetchGitSource(
	ctx context.Context,
	runner gitSourceRunner,
	repositoryPath string,
	request *GitFetchRequest,
) error {
	if request == nil {
		return nil
	}
	args := []string{
		"fetch",
		"--no-tags",
		"--no-recurse-submodules",
		"--no-write-fetch-head",
		request.Remote,
	}
	args = append(args, request.RefSpecs...)
	if _, err := runner.Run(ctx, repositoryPath, args...); err != nil {
		return fmt.Errorf("fetch candidate Git source: %w", err)
	}
	return nil
}

func resolveCandidateCommit(
	ctx context.Context,
	runner gitSourceRunner,
	repositoryPath string,
	candidateRef string,
) (string, error) {
	output, err := runner.Run(
		ctx,
		repositoryPath,
		"rev-parse",
		"--verify",
		"--end-of-options",
		candidateRef+"^{commit}",
	)
	if err != nil {
		return "", fmt.Errorf("resolve candidate ref %q: %w", candidateRef, err)
	}
	commitSHA := strings.TrimSpace(string(output))
	if !fullGitCommitSHAPattern.MatchString(commitSHA) {
		return "", fmt.Errorf(
			"resolve candidate ref %q: Git returned malformed full commit SHA %q",
			candidateRef,
			commitSHA,
		)
	}
	return commitSHA, nil
}

func readCandidatePublication(
	ctx context.Context,
	runner gitSourceRunner,
	repositoryPath string,
	commitSHA string,
	sourcePath string,
) ([]byte, error) {
	objectName := commitSHA + ":" + sourcePath
	typeOutput, err := runner.Run(ctx, repositoryPath, "cat-file", "-t", objectName)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %s at commit %s: %v",
			ErrGitSourceMissing,
			sourcePath,
			commitSHA,
			err,
		)
	}
	objectType := strings.TrimSpace(string(typeOutput))
	if objectType != "blob" {
		return nil, fmt.Errorf(
			"%w: %s at commit %s is Git object type %q, want blob",
			ErrGitSourceMalformed,
			sourcePath,
			commitSHA,
			objectType,
		)
	}
	content, err := runner.Run(ctx, repositoryPath, "cat-file", "blob", objectName)
	if err != nil {
		return nil, fmt.Errorf(
			"read candidate Git source %s at commit %s: %w",
			sourcePath,
			commitSHA,
			err,
		)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf(
			"%w: %s at commit %s is empty",
			ErrGitSourceMalformed,
			sourcePath,
			commitSHA,
		)
	}
	return content, nil
}
