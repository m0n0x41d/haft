package specmigrationv2_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestGitSourceProvenanceWitnessVerifiesRepositoryAndWorkingTreeEditions(t *testing.T) {
	root, carrier, recordBinding, parent, original := newGitWitnessRepository(t)
	repositoryProvenance, err := specmigrationv2.NewDesignatedSourceProvenance(parent, recordBinding)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance(repository): %v", err)
	}
	repositoryWitness, err := specmigrationv2.VerifyGitSourceProvenance(
		context.Background(),
		root,
		repositoryProvenance,
	)
	if err != nil {
		t.Fatalf("VerifyGitSourceProvenance(repository): %v", err)
	}
	if repositoryWitness.DesignatedDigest().String() != specmigrationv2.SourceDigestOf(original).String() {
		t.Fatalf("repository witness digest = %s", repositoryWitness.DesignatedDigest().String())
	}

	working := []byte("# selected working-tree source\n\nchanged\n")
	writeTestFile(t, filepath.Join(root.String(), filepath.FromSlash(carrier.String())), working)
	delta := runTestGit(t, root.String(), "diff", "--binary", "--no-ext-diff", "HEAD", "--", carrier.String())
	deltaBinding, err := specmigrationv2.NewWorktreeDeltaBinding(
		specmigrationv2.WorktreeDeltaGitBinaryV1,
		specmigrationv2.WorktreeDeltaDigestOf(delta),
	)
	if err != nil {
		t.Fatalf("NewWorktreeDeltaBinding: %v", err)
	}
	workingEdition, err := specmigrationv2.NewWorkingTreeEdition(
		parent,
		specmigrationv2.SourceDigestOf(working),
		deltaBinding,
	)
	if err != nil {
		t.Fatalf("NewWorkingTreeEdition: %v", err)
	}
	workingProvenance, err := specmigrationv2.NewDesignatedSourceProvenance(
		workingEdition,
		recordBinding,
	)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance(working): %v", err)
	}
	workingWitness, err := specmigrationv2.VerifyGitSourceProvenance(
		context.Background(),
		root,
		workingProvenance,
	)
	if err != nil {
		t.Fatalf("VerifyGitSourceProvenance(working): %v", err)
	}
	if workingWitness.Digest().String() == repositoryWitness.Digest().String() {
		t.Fatal("repository and working-tree editions share one witness digest")
	}
}

func TestGitSourceProvenanceWitnessRejectsWrongDeltaAndResolutionRecord(t *testing.T) {
	root, carrier, recordBinding, parent, _ := newGitWitnessRepository(t)
	working := []byte("changed source\n")
	writeTestFile(t, filepath.Join(root.String(), filepath.FromSlash(carrier.String())), working)
	wrongDelta, err := specmigrationv2.NewWorktreeDeltaBinding(
		specmigrationv2.WorktreeDeltaGitBinaryV1,
		specmigrationv2.WorktreeDeltaDigestOf([]byte("not the Git diff")),
	)
	if err != nil {
		t.Fatalf("NewWorktreeDeltaBinding: %v", err)
	}
	edition, err := specmigrationv2.NewWorkingTreeEdition(
		parent,
		specmigrationv2.SourceDigestOf(working),
		wrongDelta,
	)
	if err != nil {
		t.Fatalf("NewWorkingTreeEdition: %v", err)
	}
	provenance, err := specmigrationv2.NewDesignatedSourceProvenance(edition, recordBinding)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance: %v", err)
	}
	if _, err := specmigrationv2.VerifyGitSourceProvenance(context.Background(), root, provenance); err == nil {
		t.Fatal("Git witness accepted a wrong binary-diff digest")
	}

	badRecordDigest := specmigrationv2.ProvenanceRecordDigestOf([]byte("different record"))
	badRecord, err := specmigrationv2.NewProvenanceRecordBinding(
		recordBinding.Ref(),
		badRecordDigest,
	)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	goodDelta := runTestGit(t, root.String(), "diff", "--binary", "--no-ext-diff", "HEAD", "--", carrier.String())
	goodDeltaBinding, err := specmigrationv2.NewWorktreeDeltaBinding(
		specmigrationv2.WorktreeDeltaGitBinaryV1,
		specmigrationv2.WorktreeDeltaDigestOf(goodDelta),
	)
	if err != nil {
		t.Fatalf("NewWorktreeDeltaBinding(good): %v", err)
	}
	goodEdition, err := specmigrationv2.NewWorkingTreeEdition(
		parent,
		specmigrationv2.SourceDigestOf(working),
		goodDeltaBinding,
	)
	if err != nil {
		t.Fatalf("NewWorkingTreeEdition(good): %v", err)
	}
	badRecordProvenance, err := specmigrationv2.NewDesignatedSourceProvenance(goodEdition, badRecord)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance(bad record): %v", err)
	}
	if _, err := specmigrationv2.VerifyGitSourceProvenance(context.Background(), root, badRecordProvenance); err == nil {
		t.Fatal("Git witness accepted a wrong source-designation record digest")
	}

	goodProvenance, err := specmigrationv2.NewDesignatedSourceProvenance(goodEdition, recordBinding)
	if err != nil {
		t.Fatalf("NewDesignatedSourceProvenance(good record): %v", err)
	}
	recordPath := filepath.Join(root.String(), filepath.FromSlash(recordBinding.Ref().String()))
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("ReadFile(record): %v", err)
	}
	outsideRecord := filepath.Join(t.TempDir(), "source-designation.md")
	writeTestFile(t, outsideRecord, recordBytes)
	if err := os.Remove(recordPath); err != nil {
		t.Fatalf("Remove(record): %v", err)
	}
	if err := os.Symlink(outsideRecord, recordPath); err != nil {
		t.Fatalf("Symlink(record): %v", err)
	}
	if _, err := specmigrationv2.VerifyGitSourceProvenance(context.Background(), root, goodProvenance); err == nil {
		t.Fatal("Git witness followed a symlinked source-designation record")
	}
}

func newGitWitnessRepository(
	t *testing.T,
) (
	specmigrationv2.ApplyProjectRoot,
	specmigrationv2.SourceCarrierID,
	specmigrationv2.ProvenanceRecordBinding,
	specmigrationv2.RepositoryEdition,
	[]byte,
) {
	t.Helper()
	rootPath, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	runTestGit(t, rootPath, "init")
	runTestGit(t, rootPath, "config", "user.email", "test@example.com")
	runTestGit(t, rootPath, "config", "user.name", "Haft Test")
	carrier, err := specmigrationv2.NewSourceCarrierID(".haft/specs/enabling-system.md")
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	recordCarrier := ".context/source-designation.md"
	original := []byte("# committed source edition\n")
	record := []byte("operator designated the exact source edition\n")
	writeTestFile(t, filepath.Join(rootPath, filepath.FromSlash(carrier.String())), original)
	writeTestFile(t, filepath.Join(rootPath, filepath.FromSlash(recordCarrier)), record)
	runTestGit(t, rootPath, "add", "-f", "--", carrier.String(), recordCarrier)
	runTestGit(t, rootPath, "commit", "-m", "source edition")
	head := strings.TrimSpace(string(runTestGit(t, rootPath, "rev-parse", "HEAD")))
	objectFormat := strings.TrimSpace(string(runTestGit(t, rootPath, "rev-parse", "--show-object-format")))
	commit, err := specmigrationv2.NewGitCommitOID(objectFormat + ":" + head)
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	root, err := specmigrationv2.NewApplyProjectRoot(rootPath)
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	projectRef, err := specmigrationv2.NewProjectRootRef(rootPath)
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	parent, err := specmigrationv2.NewRepositoryEdition(
		projectRef,
		commit,
		carrier,
		specmigrationv2.SourceDigestOf(original),
	)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	recordRef, err := specmigrationv2.NewProvenanceRecordRef(recordCarrier)
	if err != nil {
		t.Fatalf("NewProvenanceRecordRef: %v", err)
	}
	recordBinding, err := specmigrationv2.NewProvenanceRecordBinding(
		recordRef,
		specmigrationv2.ProvenanceRecordDigestOf(record),
	)
	if err != nil {
		t.Fatalf("NewProvenanceRecordBinding: %v", err)
	}
	return root, carrier, recordBinding, parent, original
}

func runTestGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	commandArgs := []string{"-C", root}
	commandArgs = append(commandArgs, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return output
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
