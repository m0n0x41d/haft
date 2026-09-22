package fpfrefresh

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

func TestSourceAccessRetainsCompilerDiagnosticFields(t *testing.T) {
	diagnostic, err := typeenv.NewCompilerDiagnostic("unsupported_contract", "spec:unit", "exact missing semantic cue")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := marshalSourceCompilerDiagnostics([]typeenv.CompilerDiagnostic{diagnostic})
	if err != nil {
		t.Fatal(err)
	}
	var views []map[string]string
	if err := json.Unmarshal(payload, &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0]["code"] != diagnostic.Code() || views[0]["unit_id"] != diagnostic.UnitID() || views[0]["message"] != diagnostic.Message() {
		t.Fatalf("compiler diagnostic lost fields: %s", payload)
	}
}

func TestSourceVerificationUsesPinAndRejectsDifferentExplicitRevision(t *testing.T) {
	repository := newGitSourceTestRepository(t)
	writeGitSourceTestPublications(t, repository, "readme-a\n", "spec-a\n")
	pinned := commitGitSourceTestChanges(t, repository, "pinned")
	writeGitSourceTestPublications(t, repository, "readme-b\n", "spec-b\n")
	moved := commitGitSourceTestChanges(t, repository, "moving ref")
	request := GitSourceRequest{RepositoryPath: repository}
	actual, err := pinSourceVerification(context.Background(), request, pinned)
	if err != nil {
		t.Fatal(err)
	}
	if actual.CandidateRef != pinned || actual.Fetch != nil {
		t.Fatalf("verification followed moving ref instead of pin: %+v", actual)
	}
	request.CandidateRef = moved
	_, err = pinSourceVerification(context.Background(), request, pinned)
	if err == nil || !strings.Contains(err.Error(), "differs from explicit expected candidate") {
		t.Fatalf("expected revision mismatch, got %v", err)
	}
}

func TestSourceVerificationRejectsFetchAndUninitializedSourceDirectory(t *testing.T) {
	request := SourceAccessRequest{Source: GitSourceRequest{Fetch: &GitFetchRequest{Remote: "origin"}}}
	_, err := VerifySourceAccess(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("verification admitted fetch: %v", err)
	}
	repository := newGitSourceTestRepository(t)
	subdirectory := filepath.Join(repository, "data", "FPF")
	if err := os.MkdirAll(subdirectory, 0755); err != nil {
		t.Fatal(err)
	}
	err = validateSourceObjectRepository(context.Background(), subdirectory)
	if err == nil || !strings.Contains(err.Error(), "git submodule update --init") {
		t.Fatalf("uninitialized source could reach parent Git repository: %v", err)
	}
}

func TestSourceAccessPreservesArchiveWhenMemoryBasisChanged(t *testing.T) {
	directory := t.TempDir()
	memoryPath := filepath.Join(directory, "memory.db")
	archivePath := filepath.Join(directory, "fpf-source.db.gz")
	previous := []byte("previous coherent source archive")
	if err := os.WriteFile(memoryPath, []byte("changed memory basis"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, previous, 0600); err != nil {
		t.Fatal(err)
	}
	candidate := []byte("candidate source archive")
	report := SourceAccessReport{ArchiveDigest: digestBytesSHA256(candidate), Basis: fpf.SourceAccessBasis{MemoryDatabaseDigest: digestBytesSHA256([]byte("previous memory basis"))}}
	request := SourceAccessRequest{MemoryDatabasePath: memoryPath, ArchivePath: archivePath}
	if err := ApplySourceAccess(request, report, candidate); err == nil {
		t.Fatal("stale memory basis accepted")
	}
	actual, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, previous) {
		t.Fatal("rejected candidate replaced the previous archive")
	}
}
