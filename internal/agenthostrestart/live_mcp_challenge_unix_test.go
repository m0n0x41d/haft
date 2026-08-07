//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFulfillLiveMCPChallengeNoOpsWithoutHaftDirectory(t *testing.T) {
	root := t.TempDir()
	haftPath := filepath.Join(root, ".haft")

	if err := FulfillLiveMCPChallenge(root); err != nil {
		t.Fatalf("FulfillLiveMCPChallenge without .haft: %v", err)
	}
	if _, err := os.Lstat(haftPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no-op created or altered .haft: %v", err)
	}
}

func TestFulfillLiveMCPChallengeRejectsSymlinkedHaftDirectory(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	haftPath := filepath.Join(root, ".haft")
	if err := os.Symlink(target, haftPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := FulfillLiveMCPChallenge(root); err == nil {
		t.Fatal("FulfillLiveMCPChallenge accepted a symlinked .haft directory")
	}
}

func TestFulfillLiveMCPChallengeForStatusIgnoresPredecessorWithoutMutation(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	checkpoint := newPreparedFixture(
		t,
		root,
		"restart-predecessor-status",
		digestOf('a'),
	)
	content, err := checkpoint.CanonicalBytes()
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	predecessor := bytes.Replace(
		content,
		[]byte(`"resume_intent_digest"`),
		[]byte(`"goal_objective_digest"`),
		1,
	)
	if err := store.Prepare(checkpoint); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := os.WriteFile(store.checkpointPath, predecessor, 0o600); err != nil {
		t.Fatalf("write predecessor checkpoint: %v", err)
	}

	posture, err := FulfillLiveMCPChallengeForStatus(root)
	if err != nil {
		t.Fatalf("FulfillLiveMCPChallengeForStatus: %v", err)
	}
	if posture != LiveMCPStatusPredecessorIgnored {
		t.Fatalf("posture = %d, want predecessor ignored", posture)
	}
	after, err := os.ReadFile(store.checkpointPath)
	if err != nil {
		t.Fatalf("read predecessor checkpoint: %v", err)
	}
	if !bytes.Equal(after, predecessor) {
		t.Fatal("status hook changed predecessor checkpoint bytes")
	}
}

func TestLiveMCPChallengeIsPrivateBoundAndIdempotentlyReceipted(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, "restart-live-mcp", digestOf('d'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	submitted := applyStoreState(t, store, prepared, StateSubmitted, "")
	challengePath := store.restartReceiptPath(submitted.restartID, liveMCPChallengeSuffix)
	if _, err := os.Stat(challengePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("challenge existed before verified install boundary: %v", err)
	}
	if err := store.InstallLiveMCPChallenge(submitted); err != nil {
		t.Fatalf("InstallLiveMCPChallenge: %v", err)
	}
	assertPrivateRegularMode(t, challengePath)
	if err := store.InstallLiveMCPChallenge(submitted); !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate challenge install error = %v", err)
	}

	opened := applyStoreState(t, store, submitted, StateAppOpened, "")
	resumed := applyStoreState(t, store, opened, StateResumed, "")
	observed := mcpProcessObservation{
		PID:              42,
		ExecutablePath:   resumed.expectedHaftBinaryPath,
		ExecutableDigest: resumed.desiredHaftBinaryDigest,
		ProjectRoot:      resumed.repositoryRoot,
		StartedAt:        resumed.createdAt.Add(time.Minute),
	}
	if err := store.withExclusiveLock(func() error {
		return store.bindLiveMCPChallengeUnlocked(
			resumed,
			observed,
			resumed.createdAt.Add(2*time.Minute),
		)
	}); err != nil {
		t.Fatalf("bindLiveMCPChallengeUnlocked: %v", err)
	}
	challenge, _, err := store.loadLiveMCPChallengeUnlocked(resumed)
	if err != nil {
		t.Fatalf("load bound challenge: %v", err)
	}
	if challenge.State != liveMCPChallengeBound {
		t.Fatalf("challenge state = %s", challenge.State)
	}

	firstFulfilledAt := resumed.createdAt.Add(3 * time.Minute)
	if err := store.withExclusiveLock(func() error {
		return store.fulfillLiveMCPChallengeUnlocked(
			resumed,
			challenge,
			observed,
			firstFulfilledAt,
		)
	}); err != nil {
		t.Fatalf("first fulfillment: %v", err)
	}
	receiptPath := store.restartReceiptPath(resumed.restartID, liveMCPReceiptSuffix)
	assertPrivateRegularMode(t, receiptPath)
	firstBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read first receipt: %v", err)
	}
	if err := store.withExclusiveLock(func() error {
		return store.fulfillLiveMCPChallengeUnlocked(
			resumed,
			challenge,
			observed,
			firstFulfilledAt.Add(time.Minute),
		)
	}); err != nil {
		t.Fatalf("idempotent fulfillment: %v", err)
	}
	secondBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read second receipt: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("repeated live status fulfillment rewrote immutable receipt")
	}
	receipt, err := store.LoadLiveMCPReceipt(resumed)
	if err != nil {
		t.Fatalf("LoadLiveMCPReceipt: %v", err)
	}
	if receipt.PID != observed.PID || !receipt.FulfilledAt.Equal(firstFulfilledAt) {
		t.Fatalf("receipt identity = %#v", receipt)
	}
}

func TestLiveMCPAndFallbackReceiptsRejectTamperAndDuplicateWake(t *testing.T) {
	root := newRestartProject(t)
	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	prepared := newPreparedFixture(t, root, "restart-receipt-tamper", digestOf('e'))
	if err := store.Prepare(prepared); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	submitted := applyStoreState(t, store, prepared, StateSubmitted, "")
	if err := store.InstallLiveMCPChallenge(submitted); err != nil {
		t.Fatalf("InstallLiveMCPChallenge: %v", err)
	}
	opened := applyStoreState(t, store, submitted, StateAppOpened, "")
	resumed := applyStoreState(t, store, opened, StateResumed, "")
	verification := validRuntimeVerification(resumed)
	if err := store.withExclusiveLock(func() error {
		return store.writeLiveMCPReceiptUnlocked(resumed, verification.LiveMCPReceipt)
	}); err != nil {
		t.Fatalf("write live receipt: %v", err)
	}
	livePath := store.restartReceiptPath(resumed.restartID, liveMCPReceiptSuffix)
	content, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live receipt: %v", err)
	}
	content = bytes.Replace(
		content,
		[]byte(resumed.liveMCPChallengeNonce),
		[]byte(digestOf('0')),
		1,
	)
	if err := os.WriteFile(livePath, content, 0o600); err != nil {
		t.Fatalf("tamper live receipt: %v", err)
	}
	if _, err := store.LoadLiveMCPReceipt(resumed); err == nil {
		t.Fatal("tampered live MCP receipt was accepted")
	}

	if _, err := store.RecordResumeFallbackCleared(
		resumed,
		2,
		resumed.createdAt.Add(time.Minute),
	); err == nil {
		t.Fatal("fallback accepted more than one wake")
	}
	if _, err := store.RecordResumeFallbackCleared(
		resumed,
		1,
		resumed.createdAt.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("record valid fallback receipt: %v", err)
	}
	fallbackPath := store.restartReceiptPath(resumed.restartID, fallbackReceiptSuffix)
	if err := os.Chmod(fallbackPath, 0o644); err != nil {
		t.Fatalf("weaken fallback mode: %v", err)
	}
	if _, err := store.LoadResumeFallbackReceipt(resumed); err == nil {
		t.Fatal("world-readable fallback receipt was accepted")
	}
}

func assertPrivateRegularMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect %s: %v", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("%s mode = %s", filepath.Base(path), info.Mode())
	}
}
