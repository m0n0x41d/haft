package fpfrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireGitSourceReadsPinnedCommitWithoutTouchingDirtyCheckout(t *testing.T) {
	repositoryPath := newGitSourceTestRepository(t)
	writeGitSourceTestPublications(t, repositoryPath, "readme-a\n", "spec-a\n")
	commitA := commitGitSourceTestChanges(t, repositoryPath, "candidate A")
	runGitSourceTestCommand(t, repositoryPath, "branch", "candidate", commitA)

	writeGitSourceTestPublications(t, repositoryPath, "readme-b\n", "spec-b\n")
	_ = commitGitSourceTestChanges(t, repositoryPath, "working checkout B")
	dirtyReadme := []byte("dirty working-tree readme\n")
	writeGitSourceTestFile(t, repositoryPath, gitSourceReadmePath, dirtyReadme)
	writeGitSourceTestFile(t, repositoryPath, "untracked.txt", []byte("untracked\n"))

	headBefore := runGitSourceTestCommand(t, repositoryPath, "rev-parse", "HEAD")
	statusBefore := runGitSourceTestCommand(t, repositoryPath, "status", "--porcelain=v1")
	specBefore := readGitSourceTestFile(t, repositoryPath, gitSourceSpecPath)

	snapshot, err := AcquireGitSource(context.Background(), GitSourceRequest{
		RepositoryPath: repositoryPath,
		CandidateRef:   "refs/heads/candidate",
	})
	if err != nil {
		t.Fatalf("AcquireGitSource() error = %v", err)
	}
	if snapshot.CandidateRef() != "refs/heads/candidate" {
		t.Fatalf("CandidateRef() = %q", snapshot.CandidateRef())
	}
	if snapshot.CommitSHA() != commitA {
		t.Fatalf("CommitSHA() = %q, want %q", snapshot.CommitSHA(), commitA)
	}
	if got := string(snapshot.ReadmeBytes()); got != "readme-a\n" {
		t.Fatalf("ReadmeBytes() = %q, want exact candidate A bytes", got)
	}
	if got := string(snapshot.SpecificationBytes()); got != "spec-a\n" {
		t.Fatalf("SpecificationBytes() = %q, want exact candidate A bytes", got)
	}

	returnedReadme := snapshot.ReadmeBytes()
	returnedReadme[0] = 'X'
	if got := string(snapshot.ReadmeBytes()); got != "readme-a\n" {
		t.Fatalf("caller mutation changed snapshot bytes: %q", got)
	}
	if got := runGitSourceTestCommand(t, repositoryPath, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("HEAD changed from %q to %q", headBefore, got)
	}
	if got := runGitSourceTestCommand(t, repositoryPath, "status", "--porcelain=v1"); got != statusBefore {
		t.Fatalf("worktree status changed:\nbefore:\n%s\nafter:\n%s", statusBefore, got)
	}
	if got := readGitSourceTestFile(t, repositoryPath, gitSourceReadmePath); !bytes.Equal(got, dirtyReadme) {
		t.Fatalf("dirty Readme.md changed to %q", got)
	}
	if got := readGitSourceTestFile(t, repositoryPath, gitSourceSpecPath); !bytes.Equal(got, specBefore) {
		t.Fatalf("working-tree FPF-Spec.md changed to %q", got)
	}
}

func TestAcquireGitSourceFetchOccursOnlyWhenExplicitlyRequested(t *testing.T) {
	fixtureRoot := t.TempDir()
	seedPath := filepath.Join(fixtureRoot, "seed")
	initializeGitSourceTestRepository(t, seedPath)
	writeGitSourceTestPublications(t, seedPath, "readme-a\n", "spec-a\n")
	commitA := commitGitSourceTestChanges(t, seedPath, "remote A")

	remotePath := filepath.Join(fixtureRoot, "remote.git")
	runGitSourceTestProgram(t, "init", "--quiet", "--bare", "--initial-branch=main", remotePath)
	runGitSourceTestCommand(t, seedPath, "remote", "add", "origin", remotePath)
	runGitSourceTestCommand(t, seedPath, "push", "--quiet", "origin", "main")

	clonePath := filepath.Join(fixtureRoot, "clone")
	runGitSourceTestProgram(t, "clone", "--quiet", remotePath, clonePath)

	writeGitSourceTestPublications(t, seedPath, "readme-b\n", "spec-b\n")
	commitB := commitGitSourceTestChanges(t, seedPath, "remote B")
	runGitSourceTestCommand(t, seedPath, "push", "--quiet", "origin", "main")

	dirtyReadme := []byte("dirty local readme\n")
	writeGitSourceTestFile(t, clonePath, gitSourceReadmePath, dirtyReadme)
	headBefore := runGitSourceTestCommand(t, clonePath, "rev-parse", "HEAD")
	if headBefore != commitA {
		t.Fatalf("clone HEAD = %q, want predecessor %q", headBefore, commitA)
	}

	withoutFetch, err := AcquireGitSource(context.Background(), GitSourceRequest{
		RepositoryPath: clonePath,
		CandidateRef:   "refs/remotes/origin/main",
	})
	if err != nil {
		t.Fatalf("AcquireGitSource() without fetch error = %v", err)
	}
	if withoutFetch.CommitSHA() != commitA {
		t.Fatalf("without-fetch commit = %q, want %q", withoutFetch.CommitSHA(), commitA)
	}

	withFetch, err := AcquireGitSource(context.Background(), GitSourceRequest{
		RepositoryPath: clonePath,
		CandidateRef:   "refs/remotes/origin/main",
		Fetch: &GitFetchRequest{
			Remote: "origin",
		},
	})
	if err != nil {
		t.Fatalf("AcquireGitSource() with explicit fetch error = %v", err)
	}
	if withFetch.CommitSHA() != commitB {
		t.Fatalf("with-fetch commit = %q, want %q", withFetch.CommitSHA(), commitB)
	}
	if got := string(withFetch.ReadmeBytes()); got != "readme-b\n" {
		t.Fatalf("with-fetch ReadmeBytes() = %q", got)
	}
	if got := runGitSourceTestCommand(t, clonePath, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("explicit fetch changed checkout HEAD from %q to %q", headBefore, got)
	}
	if got := readGitSourceTestFile(t, clonePath, gitSourceReadmePath); !bytes.Equal(got, dirtyReadme) {
		t.Fatalf("explicit fetch changed dirty worktree bytes to %q", got)
	}
}

func TestAcquireGitSourcePinsOnceAndObservesMovingRefSeparately(t *testing.T) {
	repositoryPath := newGitSourceTestRepository(t)
	writeGitSourceTestPublications(t, repositoryPath, "readme-a\n", "spec-a\n")
	commitA := commitGitSourceTestChanges(t, repositoryPath, "candidate A")
	runGitSourceTestCommand(t, repositoryPath, "branch", "candidate", commitA)

	writeGitSourceTestPublications(t, repositoryPath, "readme-b\n", "spec-b\n")
	commitB := commitGitSourceTestChanges(t, repositoryPath, "candidate B")

	runner := &moveCandidateAfterResolutionRunner{
		base:           commandGitSourceRunner{},
		candidateRef:   "refs/heads/candidate",
		replacementSHA: commitB,
	}
	snapshot, err := acquireGitSource(context.Background(), runner, GitSourceRequest{
		RepositoryPath: repositoryPath,
		CandidateRef:   runner.candidateRef,
	})
	if err != nil {
		t.Fatalf("acquireGitSource() error = %v", err)
	}
	if runner.resolveCalls != 1 {
		t.Fatalf("candidate ref resolution calls = %d, want exactly 1", runner.resolveCalls)
	}
	if snapshot.CommitSHA() != commitA {
		t.Fatalf("snapshot commit = %q, want pinned %q", snapshot.CommitSHA(), commitA)
	}
	if got := string(snapshot.ReadmeBytes()); got != "readme-a\n" {
		t.Fatalf("snapshot followed moving ref: ReadmeBytes() = %q", got)
	}

	observation, err := ObserveCandidateRef(
		context.Background(),
		GitSourceRequest{
			RepositoryPath: repositoryPath,
			CandidateRef:   runner.candidateRef,
		},
		snapshot.CommitSHA(),
	)
	if err != nil {
		t.Fatalf("ObserveCandidateRef() error = %v", err)
	}
	if !observation.Moved ||
		observation.PinnedCommitSHA != commitA ||
		observation.ObservedCommitSHA != commitB {
		t.Fatalf("observation = %#v, want %s -> %s movement", observation, commitA, commitB)
	}
	if snapshot.CommitSHA() != commitA || string(snapshot.ReadmeBytes()) != "readme-a\n" {
		t.Fatal("separate observation mutated or repinned the acquired snapshot")
	}
}

func TestAcquireGitSourceClassifiesMissingAndMalformedPublications(t *testing.T) {
	t.Run("missing specification", func(t *testing.T) {
		repositoryPath := newGitSourceTestRepository(t)
		writeGitSourceTestFile(t, repositoryPath, gitSourceReadmePath, []byte("readme\n"))
		commit := commitGitSourceTestChanges(t, repositoryPath, "missing specification")

		snapshot, err := AcquireGitSource(context.Background(), GitSourceRequest{
			RepositoryPath: repositoryPath,
			CandidateRef:   "HEAD",
		})
		if !errors.Is(err, ErrGitSourceMissing) ||
			!strings.Contains(err.Error(), gitSourceSpecPath) {
			t.Fatalf("error = %v, want missing %s classification", err, gitSourceSpecPath)
		}
		if snapshot.CommitSHA() != commit ||
			string(snapshot.ReadmeBytes()) != "readme\n" ||
			len(snapshot.SpecificationBytes()) != 0 {
			t.Fatalf("partial source observation = %#v", snapshot)
		}
	})

	t.Run("publication is a tree", func(t *testing.T) {
		repositoryPath := newGitSourceTestRepository(t)
		writeGitSourceTestFile(
			t,
			repositoryPath,
			filepath.Join(gitSourceReadmePath, "nested.txt"),
			[]byte("not a blob publication\n"),
		)
		writeGitSourceTestFile(t, repositoryPath, gitSourceSpecPath, []byte("spec\n"))
		_ = commitGitSourceTestChanges(t, repositoryPath, "tree publication")

		_, err := AcquireGitSource(context.Background(), GitSourceRequest{
			RepositoryPath: repositoryPath,
			CandidateRef:   "HEAD",
		})
		if !errors.Is(err, ErrGitSourceMalformed) ||
			!strings.Contains(err.Error(), gitSourceReadmePath) ||
			!strings.Contains(err.Error(), `object type "tree"`) {
			t.Fatalf("error = %v, want malformed tree publication classification", err)
		}
	})

	t.Run("publication is empty", func(t *testing.T) {
		repositoryPath := newGitSourceTestRepository(t)
		writeGitSourceTestPublications(t, repositoryPath, "", "spec\n")
		_ = commitGitSourceTestChanges(t, repositoryPath, "empty readme")

		_, err := AcquireGitSource(context.Background(), GitSourceRequest{
			RepositoryPath: repositoryPath,
			CandidateRef:   "HEAD",
		})
		if !errors.Is(err, ErrGitSourceMalformed) ||
			!strings.Contains(err.Error(), gitSourceReadmePath) ||
			!strings.Contains(err.Error(), "is empty") {
			t.Fatalf("error = %v, want malformed empty publication classification", err)
		}
	})
}

func TestAcquireGitSourceRejectsMalformedInputs(t *testing.T) {
	repositoryPath := newGitSourceTestRepository(t)
	writeGitSourceTestPublications(t, repositoryPath, "readme\n", "spec\n")
	_ = commitGitSourceTestChanges(t, repositoryPath, "fixture")

	testCases := []struct {
		name    string
		request GitSourceRequest
		want    string
	}{
		{
			name: "missing ref",
			request: GitSourceRequest{
				RepositoryPath: repositoryPath,
			},
			want: "candidate ref is required",
		},
		{
			name: "invalid ref",
			request: GitSourceRequest{
				RepositoryPath: repositoryPath,
				CandidateRef:   "not a valid ref",
			},
			want: "resolve candidate ref",
		},
		{
			name: "implicit fetch remote forbidden",
			request: GitSourceRequest{
				RepositoryPath: repositoryPath,
				CandidateRef:   "HEAD",
				Fetch:          &GitFetchRequest{},
			},
			want: "fetch remote is required",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := AcquireGitSource(context.Background(), testCase.request)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want text %q", err, testCase.want)
			}
		})
	}
}

type moveCandidateAfterResolutionRunner struct {
	base           gitSourceRunner
	candidateRef   string
	replacementSHA string
	resolveCalls   int
	moved          bool
}

func (runner *moveCandidateAfterResolutionRunner) Run(
	ctx context.Context,
	repositoryPath string,
	args ...string,
) ([]byte, error) {
	output, err := runner.base.Run(ctx, repositoryPath, args...)
	if err != nil || len(args) == 0 || args[0] != "rev-parse" {
		return output, err
	}
	runner.resolveCalls++
	if runner.moved {
		return output, nil
	}
	runner.moved = true
	if _, updateErr := runner.base.Run(
		ctx,
		repositoryPath,
		"update-ref",
		runner.candidateRef,
		runner.replacementSHA,
	); updateErr != nil {
		return nil, fmt.Errorf("move candidate ref after resolution: %w", updateErr)
	}
	return output, nil
}

func newGitSourceTestRepository(t *testing.T) string {
	t.Helper()
	repositoryPath := filepath.Join(t.TempDir(), "repository")
	initializeGitSourceTestRepository(t, repositoryPath)
	return repositoryPath
}

func initializeGitSourceTestRepository(t *testing.T, repositoryPath string) {
	t.Helper()
	if err := os.MkdirAll(repositoryPath, 0o755); err != nil {
		t.Fatalf("create Git fixture repository: %v", err)
	}
	runGitSourceTestCommand(t, repositoryPath, "init", "--quiet", "--initial-branch=main")
	runGitSourceTestCommand(t, repositoryPath, "config", "user.name", "Haft Test")
	runGitSourceTestCommand(t, repositoryPath, "config", "user.email", "haft-test@example.invalid")
}

func writeGitSourceTestPublications(
	t *testing.T,
	repositoryPath string,
	readme string,
	specification string,
) {
	t.Helper()
	writeGitSourceTestFile(t, repositoryPath, gitSourceReadmePath, []byte(readme))
	writeGitSourceTestFile(t, repositoryPath, gitSourceSpecPath, []byte(specification))
}

func writeGitSourceTestFile(
	t *testing.T,
	repositoryPath string,
	relativePath string,
	content []byte,
) {
	t.Helper()
	path := filepath.Join(repositoryPath, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory for %s: %v", relativePath, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", relativePath, err)
	}
}

func readGitSourceTestFile(
	t *testing.T,
	repositoryPath string,
	relativePath string,
) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryPath, relativePath))
	if err != nil {
		t.Fatalf("read fixture %s: %v", relativePath, err)
	}
	return content
}

func commitGitSourceTestChanges(
	t *testing.T,
	repositoryPath string,
	message string,
) string {
	t.Helper()
	runGitSourceTestCommand(t, repositoryPath, "add", "--all")
	runGitSourceTestCommand(t, repositoryPath, "commit", "--quiet", "-m", message)
	return runGitSourceTestCommand(t, repositoryPath, "rev-parse", "HEAD")
}

func runGitSourceTestCommand(
	t *testing.T,
	repositoryPath string,
	args ...string,
) string {
	t.Helper()
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", repositoryPath)
	commandArgs = append(commandArgs, args...)
	return runGitSourceTestProgram(t, commandArgs...)
}

func runGitSourceTestProgram(
	t *testing.T,
	args ...string,
) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
