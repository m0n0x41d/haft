package fpfrefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseLocalPracticeCandidateMakesCurrentCarrierExactAndIdempotent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := ResolveRepositoryLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	lockPayload, err := os.ReadFile(layout.IntegrationLock)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(layout.LatestLocalPracticeCandidate)
	if err != nil {
		t.Fatal(err)
	}
	current, pinCount, err := rebaseLocalPracticeCandidateBytes(
		original,
		lock.Coordinates.BaseTypeEnvRef,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if pinCount == 0 {
		t.Fatal("current carrier has no source pins")
	}
	stale := strings.Replace(
		string(current),
		lock.Coordinates.BaseTypeEnvRef,
		"typeenv:sha256:"+strings.Repeat("a", 64),
		1,
	)
	stale = strings.ReplaceAll(
		stale,
		lock.Coordinates.SourceRevision,
		strings.Repeat("b", len(lock.Coordinates.SourceRevision)),
	)
	stale = strings.ReplaceAll(
		stale,
		lock.Coordinates.SpecDocumentDigest,
		"sha256:"+strings.Repeat("c", 64),
	)
	candidatePath := filepath.Join(t.TempDir(), "candidate.yaml")
	if err := os.WriteFile(candidatePath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := RebaseLocalPracticeCandidate(
		candidatePath,
		layout.Database,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.SourcePinCount != pinCount {
		t.Fatalf("first rebase = %#v, want changed with %d pins", result, pinCount)
	}
	if err := VerifyLocalPracticeCandidateExact(
		candidatePath,
		layout.Database,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	); err != nil {
		t.Fatal(err)
	}
	result, err = RebaseLocalPracticeCandidate(
		candidatePath,
		layout.Database,
		lock.Coordinates.SourceRevision,
		lock.Coordinates.SpecDocumentDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("second rebase rewrote an already-current carrier")
	}
}
