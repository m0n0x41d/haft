package specmigrationv2_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestWorkingTreeEditionRequiresExactDistinctParentAndDelta(t *testing.T) {
	carrier := mustSourceCarrierID(t, ".haft/specs/enabling-system.md")
	projectRoot, err := specmigrationv2.NewProjectRootRef("project-root:haft")
	if err != nil {
		t.Fatalf("NewProjectRootRef: %v", err)
	}
	commitOID, err := specmigrationv2.NewGitCommitOID("sha1:" + strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("NewGitCommitOID: %v", err)
	}
	parentDigest := specmigrationv2.SourceDigestOf([]byte("parent edition"))
	parent, err := specmigrationv2.NewRepositoryEdition(
		projectRoot,
		commitOID,
		carrier,
		parentDigest,
	)
	if err != nil {
		t.Fatalf("NewRepositoryEdition: %v", err)
	}
	deltaDigest := specmigrationv2.WorktreeDeltaDigestOf([]byte("exact git binary diff"))
	delta, err := specmigrationv2.NewWorktreeDeltaBinding(
		specmigrationv2.WorktreeDeltaGitBinaryV1,
		deltaDigest,
	)
	if err != nil {
		t.Fatalf("NewWorktreeDeltaBinding: %v", err)
	}
	workingDigest := specmigrationv2.SourceDigestOf([]byte("working-tree edition"))
	edition, err := specmigrationv2.NewWorkingTreeEdition(parent, workingDigest, delta)
	if err != nil {
		t.Fatalf("NewWorkingTreeEdition: %v", err)
	}
	if !edition.DesignatedDigest().Equal(workingDigest) {
		t.Fatalf("designated digest = %s, want %s", edition.DesignatedDigest().String(), workingDigest.String())
	}
	if !edition.Parent().DesignatedDigest().Equal(parentDigest) {
		t.Fatalf("parent digest = %s, want %s", edition.Parent().DesignatedDigest().String(), parentDigest.String())
	}

	if _, err := specmigrationv2.NewWorkingTreeEdition(parent, parentDigest, delta); err == nil {
		t.Fatal("NewWorkingTreeEdition accepted a no-op edition that must canonicalize to RepositoryEdition")
	}
}

func TestGitCommitOIDRejectsUntaggedOrNoncanonicalValues(t *testing.T) {
	invalid := []string{
		strings.Repeat("a", 40),
		"sha1:" + strings.Repeat("A", 40),
		" sha1:" + strings.Repeat("a", 40),
		"sha256:" + strings.Repeat("a", 63),
	}
	for _, raw := range invalid {
		if _, err := specmigrationv2.NewGitCommitOID(raw); err == nil {
			t.Errorf("NewGitCommitOID(%q) succeeded", raw)
		}
	}
}
