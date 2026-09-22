package fpfrefresh

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func readSourceAccessArchive(path string) ([]byte, []byte, error) {
	archive, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	buffer := bytes.NewReader(archive)
	reader, err := gzip.NewReader(buffer)
	if err != nil {
		return nil, nil, err
	}
	payload, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	return archive, payload, nil
}

// SourceAccessRevision reads the archive's own pin without consulting a moving
// Git ref. It only supplies an acquisition coordinate, not integrity evidence.
func SourceAccessRevision(path string) (string, error) {
	_, payload, err := readSourceAccessArchive(path)
	if err != nil {
		return "", err
	}
	return sourceAccessRevision(payload)
}

func sourceAccessRevision(payload []byte) (string, error) {
	workspace, err := os.MkdirTemp("", "haft-source-pin-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	path := filepath.Join(workspace, "source.db")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return "", err
	}
	database, err := openIntegrationDatabaseReadOnly(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = database.Close() }()
	revision, err := fpf.GetSpecMeta(database, "fpf_commit")
	if err != nil {
		return "", err
	}
	if !fullGitCommitSHAPattern.MatchString(revision) {
		return "", fmt.Errorf("source archive has no exact Git revision")
	}
	return revision, nil
}

func pinSourceVerification(ctx context.Context, source GitSourceRequest, revision string) (GitSourceRequest, error) {
	if err := validateSourceObjectRepository(ctx, source.RepositoryPath); err != nil {
		return GitSourceRequest{}, err
	}
	runner := commandGitSourceRunner{}
	_, err := resolveCandidateCommit(ctx, runner, source.RepositoryPath, revision)
	if err != nil {
		path := strings.ReplaceAll(source.RepositoryPath, "'", "'\"'\"'")
		return GitSourceRequest{}, fmt.Errorf("pinned source objects unavailable for %s: %w; fetch outside the read-only verifier: git -C '%s' fetch --no-tags origin %s (or fpf-refresh source-fetch --repo <Haft-root> --source-repo '%s')", revision, err, path, revision, path)
	}
	if source.CandidateRef == "" {
		return GitSourceRequest{RepositoryPath: source.RepositoryPath, CandidateRef: revision}, nil
	}
	expected, err := resolveCandidateCommit(ctx, runner, source.RepositoryPath, source.CandidateRef)
	if err != nil {
		return GitSourceRequest{}, err
	}
	if expected != revision {
		return GitSourceRequest{}, fmt.Errorf("source archive revision %s differs from explicit expected candidate %s", revision, expected)
	}
	return GitSourceRequest{RepositoryPath: source.RepositoryPath, CandidateRef: revision}, nil
}

// FetchSourceAccessObjects is the explicit network boundary used before CI's
// read-only verifier. It never checks out or changes the archive's source pin.
func FetchSourceAccessObjects(ctx context.Context, archivePath, repositoryPath, remote string) (string, error) {
	if err := validateSourceObjectRepository(ctx, repositoryPath); err != nil {
		return "", err
	}
	revision, err := SourceAccessRevision(archivePath)
	if err != nil {
		return "", err
	}
	request := GitSourceRequest{
		RepositoryPath: repositoryPath,
		CandidateRef:   revision,
		Fetch:          &GitFetchRequest{Remote: remote, RefSpecs: []string{revision}},
	}
	normalized, err := validateGitSourceRequest(request)
	if err != nil {
		return "", err
	}
	runner := commandGitSourceRunner{}
	if err := fetchGitSource(ctx, runner, repositoryPath, normalized.Fetch); err != nil {
		return "", err
	}
	return resolveCandidateCommit(ctx, runner, repositoryPath, revision)
}

func validateSourceObjectRepository(ctx context.Context, path string) error {
	runner := commandGitSourceRunner{}
	prefix, err := runner.Run(ctx, path, "rev-parse", "--show-prefix")
	if err != nil {
		return fmt.Errorf("source Git repository %q is unavailable: %w; initialize data/FPF with git submodule update --init -- data/FPF or supply --source-repo <initialized-FPF-repository>", path, err)
	}
	if strings.TrimSpace(string(prefix)) != "" {
		return fmt.Errorf("source path %q is inside another Git repository; initialize data/FPF with git submodule update --init -- data/FPF or supply --source-repo <initialized-FPF-repository>", path)
	}
	return nil
}
