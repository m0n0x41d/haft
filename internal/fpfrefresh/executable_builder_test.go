package fpfrefresh

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBoundedIndexerOutputRetainsOnlyTail(t *testing.T) {
	t.Parallel()

	var output boundedIndexerOutput
	prefix := strings.Repeat("p", maxIndexerOutputBytes)
	const suffix = "candidate indexer failure detail"

	written, err := output.Write([]byte(prefix + suffix))
	if err != nil {
		t.Fatal(err)
	}
	if written != len(prefix)+len(suffix) {
		t.Fatalf("Write() = %d, want %d", written, len(prefix)+len(suffix))
	}
	detail := output.Detail()
	if !strings.HasPrefix(detail, "[output truncated]\n") {
		t.Fatalf("Detail() omitted truncation marker: %q", detail)
	}
	if !strings.HasSuffix(detail, suffix) {
		t.Fatalf("Detail() omitted final failure detail: %q", detail)
	}
	if len(detail) > maxIndexerOutputBytes+len("[output truncated]\n") {
		t.Fatalf("Detail() retained %d bytes, want at most %d", len(detail), maxIndexerOutputBytes+len("[output truncated]\n"))
	}
}

func TestExecutableIndexBuilderBoundsFailedProcessOutput(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("executable script fixture requires a POSIX shell")
	}

	repositoryPath := newGitSourceTestRepository(t)
	writeGitSourceTestFile(t, repositoryPath, "source.txt", []byte("source"))
	revision := commitGitSourceTestChanges(t, repositoryPath, "source")

	executablePath := filepath.Join(t.TempDir(), "failing-indexer")
	const finalDetail = "candidate indexer failure detail"
	script := "#!/bin/sh\n" +
		"printf '%s\\n' '" + strings.Repeat("x", maxIndexerOutputBytes+1024) + "'\n" +
		"printf '%s\\n' '" + finalDetail + "' >&2\n" +
		"exit 23\n"
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	workingDirectory := t.TempDir()
	err := (ExecutableIndexBuilder{
		ExecutablePath:       executablePath,
		SourceRepositoryPath: repositoryPath,
	}).BuildIndex(context.Background(), IndexBuildInput{
		WorkingDirectory:  workingDirectory,
		SpecificationPath: filepath.Join(workingDirectory, "spec.md"),
		DatabasePath:      filepath.Join(workingDirectory, "candidate.db"),
		SourceRevision:    revision,
	})
	if err == nil {
		t.Fatal("BuildIndex() error = nil")
	}
	detail := err.Error()
	if !strings.Contains(detail, "[output truncated]") ||
		!strings.HasSuffix(detail, finalDetail) {
		t.Fatalf("BuildIndex() error omitted bounded final detail: %q", detail)
	}
	if len(detail) > maxIndexerOutputBytes+256 {
		t.Fatalf("BuildIndex() error retained %d bytes, want a bounded diagnostic", len(detail))
	}
}
