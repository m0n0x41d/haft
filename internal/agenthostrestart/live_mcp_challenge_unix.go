//go:build darwin || linux

package agenthostrestart

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	liveMCPChallengeSuffix    = ".live-mcp-challenge.json"
	liveMCPReceiptSuffix      = ".live-mcp-receipt.json"
	fallbackReceiptSuffix     = ".resume-fallback-receipt.json"
	restartReceiptStagePrefix = ".restart-receipt-stage-"
	restartReceiptMaxBytes    = 16 << 10
)

type liveMCPChallengeState string

const (
	liveMCPChallengePrepared liveMCPChallengeState = "prepared"
	liveMCPChallengeBound    liveMCPChallengeState = "bound"
)

type liveMCPChallenge struct {
	RestartID             string                `json:"restart_id"`
	CheckpointBasisDigest string                `json:"checkpoint_basis_digest"`
	Nonce                 string                `json:"nonce"`
	State                 liveMCPChallengeState `json:"state"`
	ExpectedPID           int                   `json:"expected_pid,omitempty"`
	ExpectedExecutable    string                `json:"expected_executable,omitempty"`
	ExpectedDigest        string                `json:"expected_digest,omitempty"`
	ExpectedProjectRoot   string                `json:"expected_project_root,omitempty"`
	ExpectedStartedAt     string                `json:"expected_started_at,omitempty"`
	CreatedAt             string                `json:"created_at"`
	BoundAt               string                `json:"bound_at,omitempty"`
}

type liveMCPReceiptWire struct {
	RestartID             string `json:"restart_id"`
	CheckpointBasisDigest string `json:"checkpoint_basis_digest"`
	Nonce                 string `json:"nonce"`
	PID                   int    `json:"pid"`
	ExecutablePath        string `json:"executable_path"`
	ExecutableDigest      string `json:"executable_digest"`
	ProjectRoot           string `json:"project_root"`
	ProcessStartedAt      string `json:"process_started_at"`
	FulfilledAt           string `json:"fulfilled_at"`
}

type fallbackReceiptWire struct {
	RestartID             string `json:"restart_id"`
	CheckpointBasisDigest string `json:"checkpoint_basis_digest"`
	Nonce                 string `json:"nonce"`
	WakeCount             uint8  `json:"wake_count"`
	ClearedAt             string `json:"cleared_at"`
}

type mcpProcessObservation struct {
	PID              int
	ExecutablePath   string
	ExecutableDigest string
	ProjectRoot      string
	StartedAt        time.Time
}

// LiveMCPStatusPosture is the closed public-status disposition of the private
// restart challenge hook. A predecessor checkpoint is visible attention, not
// current restart authority and not a reason to fail the whole project status.
type LiveMCPStatusPosture uint8

const (
	LiveMCPStatusNoChallenge LiveMCPStatusPosture = iota + 1
	LiveMCPStatusCurrentChallenge
	LiveMCPStatusPredecessorIgnored
)

// InstallLiveMCPChallenge persists the private nonce before the host boundary.
// It is called only after install, init, basis, and carrier validation.
func (store Store) InstallLiveMCPChallenge(checkpoint Checkpoint) error {
	if checkpoint.state != StateSubmitted {
		return fmt.Errorf("%w: live MCP challenge requires submitted state", ErrInvalidTransition)
	}
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		return err
	}
	challenge := liveMCPChallenge{
		RestartID:             checkpoint.restartID,
		CheckpointBasisDigest: basisDigest,
		Nonce:                 checkpoint.liveMCPChallengeNonce,
		State:                 liveMCPChallengePrepared,
		CreatedAt:             checkpoint.createdAt.Format(time.RFC3339Nano),
	}
	content, err := canonicalRestartJSON(challenge)
	if err != nil {
		return err
	}
	return store.withExclusiveLock(func() error {
		current, loadErr := store.loadUnlocked()
		if loadErr != nil {
			return loadErr
		}
		if !checkpointBasisEqual(current, checkpoint) || current.state != StateSubmitted {
			return ErrConcurrentUpdate
		}
		path := store.restartReceiptPath(checkpoint.restartID, liveMCPChallengeSuffix)
		return createPrivateRestartFile(path, content)
	})
}

// BindLiveMCPChallenge fixes the only process generation allowed to satisfy
// the challenge. It performs no MCP call itself.
func (store Store) BindLiveMCPChallenge(
	ctx context.Context,
	checkpoint Checkpoint,
	pid int,
	now time.Time,
) (mcpProcessObservation, error) {
	if checkpoint.state != StateResumed {
		return mcpProcessObservation{}, fmt.Errorf("%w: live MCP binding requires resumed state", ErrInvalidTransition)
	}
	observed, err := observeMCPProcess(ctx, pid, checkpoint.repositoryRoot)
	if err != nil {
		return mcpProcessObservation{}, err
	}
	if observed.ExecutablePath != checkpoint.expectedHaftBinaryPath ||
		observed.ExecutableDigest != checkpoint.desiredHaftBinaryDigest {
		return mcpProcessObservation{}, fmt.Errorf("live MCP process does not use the expected Haft bytes")
	}
	if !observed.StartedAt.After(checkpoint.createdAt) {
		return mcpProcessObservation{}, fmt.Errorf("live MCP process predates the restart checkpoint")
	}
	err = store.withExclusiveLock(func() error {
		current, loadErr := store.loadUnlocked()
		if loadErr != nil {
			return loadErr
		}
		if !checkpointBasisEqual(current, checkpoint) || current.state != StateResumed {
			return ErrConcurrentUpdate
		}
		return store.bindLiveMCPChallengeUnlocked(checkpoint, observed, now)
	})
	if err != nil {
		return mcpProcessObservation{}, err
	}
	return observed, nil
}

func (store Store) bindLiveMCPChallengeUnlocked(
	checkpoint Checkpoint,
	observed mcpProcessObservation,
	now time.Time,
) error {
	challenge, content, err := store.loadLiveMCPChallengeUnlocked(checkpoint)
	if err != nil {
		return err
	}
	if challenge.State != liveMCPChallengePrepared {
		return fmt.Errorf("live MCP challenge is already %s", challenge.State)
	}
	challenge.State = liveMCPChallengeBound
	challenge.ExpectedPID = observed.PID
	challenge.ExpectedExecutable = observed.ExecutablePath
	challenge.ExpectedDigest = observed.ExecutableDigest
	challenge.ExpectedProjectRoot = observed.ProjectRoot
	challenge.ExpectedStartedAt = observed.StartedAt.Format(time.RFC3339Nano)
	challenge.BoundAt = now.UTC().Format(time.RFC3339Nano)
	proposed, err := canonicalRestartJSON(challenge)
	if err != nil {
		return err
	}
	path := store.restartReceiptPath(checkpoint.restartID, liveMCPChallengeSuffix)
	return replacePrivateRestartFileCAS(path, content, proposed)
}

// FulfillLiveMCPChallenge is the minimal hook called by the status handler.
// With no exact bound private challenge it is a read-only no-op.
func FulfillLiveMCPChallenge(projectRoot string) error {
	_, err := FulfillLiveMCPChallengeForStatus(projectRoot)
	return err
}

// FulfillLiveMCPChallengeForStatus executes the same exact current-schema
// challenge hook while classifying the one supported predecessor schema.
// Predecessor bytes are never decoded into a current Checkpoint and remain
// untouched for explicit diagnostic or archival handling.
func FulfillLiveMCPChallengeForStatus(
	projectRoot string,
) (LiveMCPStatusPosture, error) {
	store, err := NewStore(projectRoot)
	if errors.Is(err, os.ErrNotExist) {
		return LiveMCPStatusNoChallenge, nil
	}
	if err != nil {
		return 0, err
	}
	checkpoint, err := store.Load()
	if errors.Is(err, ErrCheckpointNotFound) {
		return LiveMCPStatusNoChallenge, nil
	}
	if errors.Is(err, ErrPredecessorCheckpoint) {
		return LiveMCPStatusPredecessorIgnored, nil
	}
	if err != nil {
		return 0, err
	}
	if checkpoint.state != StateResumed && checkpoint.state != StateVerified {
		return LiveMCPStatusNoChallenge, nil
	}
	err = store.withExclusiveLock(func() error {
		current, loadErr := store.loadUnlocked()
		if loadErr != nil {
			return loadErr
		}
		challenge, _, loadErr := store.loadLiveMCPChallengeUnlocked(current)
		if errors.Is(loadErr, os.ErrNotExist) {
			return nil
		}
		if loadErr != nil {
			return loadErr
		}
		if challenge.State != liveMCPChallengeBound {
			return nil
		}
		observed, observeErr := observeMCPProcess(
			context.Background(),
			os.Getpid(),
			current.repositoryRoot,
		)
		if observeErr != nil {
			return observeErr
		}
		return store.fulfillLiveMCPChallengeUnlocked(
			current,
			challenge,
			observed,
			time.Now().UTC(),
		)
	})
	if err != nil {
		return 0, err
	}
	return LiveMCPStatusCurrentChallenge, nil
}

func (store Store) fulfillLiveMCPChallengeUnlocked(
	checkpoint Checkpoint,
	challenge liveMCPChallenge,
	observed mcpProcessObservation,
	fulfilledAt time.Time,
) error {
	if err := challenge.matches(observed); err != nil {
		return err
	}
	existing, err := store.loadLiveMCPReceiptUnlocked(checkpoint)
	if err == nil {
		return existing.matchesChallengeAndProcess(challenge, observed)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	receipt := LiveMCPReceipt{
		RestartID:             checkpoint.restartID,
		CheckpointBasisDigest: challenge.CheckpointBasisDigest,
		Nonce:                 challenge.Nonce,
		PID:                   observed.PID,
		ExecutablePath:        observed.ExecutablePath,
		ExecutableDigest:      observed.ExecutableDigest,
		ProjectRoot:           observed.ProjectRoot,
		ProcessStartedAt:      observed.StartedAt,
		FulfilledAt:           fulfilledAt.UTC(),
	}
	return store.writeLiveMCPReceiptUnlocked(checkpoint, receipt)
}

func (store Store) LoadLiveMCPReceipt(checkpoint Checkpoint) (LiveMCPReceipt, error) {
	return store.loadLiveMCPReceiptUnlocked(checkpoint)
}

func (store Store) loadLiveMCPReceiptUnlocked(checkpoint Checkpoint) (LiveMCPReceipt, error) {
	path := store.restartReceiptPath(checkpoint.restartID, liveMCPReceiptSuffix)
	content, err := readPrivateRestartJSON(path)
	if err != nil {
		return LiveMCPReceipt{}, err
	}
	var wire liveMCPReceiptWire
	if err := decodeStrictRestartJSON(content, &wire); err != nil {
		return LiveMCPReceipt{}, err
	}
	startedAt, err := time.Parse(time.RFC3339Nano, wire.ProcessStartedAt)
	if err != nil {
		return LiveMCPReceipt{}, err
	}
	fulfilledAt, err := time.Parse(time.RFC3339Nano, wire.FulfilledAt)
	if err != nil {
		return LiveMCPReceipt{}, err
	}
	receipt := LiveMCPReceipt{
		RestartID:             wire.RestartID,
		CheckpointBasisDigest: wire.CheckpointBasisDigest,
		Nonce:                 wire.Nonce,
		PID:                   wire.PID,
		ExecutablePath:        wire.ExecutablePath,
		ExecutableDigest:      wire.ExecutableDigest,
		ProjectRoot:           wire.ProjectRoot,
		ProcessStartedAt:      startedAt.UTC(),
		FulfilledAt:           fulfilledAt.UTC(),
	}
	if err := validateLiveMCPReceipt(checkpoint, receipt); err != nil {
		return LiveMCPReceipt{}, err
	}
	return receipt, nil
}

func (store Store) writeLiveMCPReceiptUnlocked(
	checkpoint Checkpoint,
	receipt LiveMCPReceipt,
) error {
	if err := validateLiveMCPReceipt(checkpoint, receipt); err != nil {
		return err
	}
	wire := liveMCPReceiptWire{
		RestartID:             receipt.RestartID,
		CheckpointBasisDigest: receipt.CheckpointBasisDigest,
		Nonce:                 receipt.Nonce,
		PID:                   receipt.PID,
		ExecutablePath:        receipt.ExecutablePath,
		ExecutableDigest:      receipt.ExecutableDigest,
		ProjectRoot:           receipt.ProjectRoot,
		ProcessStartedAt:      receipt.ProcessStartedAt.UTC().Format(time.RFC3339Nano),
		FulfilledAt:           receipt.FulfilledAt.UTC().Format(time.RFC3339Nano),
	}
	content, err := canonicalRestartJSON(wire)
	if err != nil {
		return err
	}
	path := store.restartReceiptPath(checkpoint.restartID, liveMCPReceiptSuffix)
	existing, readErr := readPrivateRestartJSON(path)
	if readErr == nil {
		if bytes.Equal(existing, content) {
			return nil
		}
		return fmt.Errorf("live MCP receipt already exists with different bytes")
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	return createPrivateRestartFile(path, content)
}

func (store Store) RecordResumeFallbackCleared(
	checkpoint Checkpoint,
	wakeCount uint8,
	clearedAt time.Time,
) (ResumeFallbackReceipt, error) {
	if checkpoint.state != StateResumed {
		return ResumeFallbackReceipt{}, fmt.Errorf("fallback cleanup requires resumed checkpoint")
	}
	if wakeCount > 1 {
		return ResumeFallbackReceipt{}, fmt.Errorf("fallback wake count exceeds one-shot bound")
	}
	if !clearedAt.UTC().After(checkpoint.createdAt) {
		return ResumeFallbackReceipt{}, fmt.Errorf("fallback cleanup predates restart checkpoint")
	}
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	receipt := ResumeFallbackReceipt{
		RestartID:             checkpoint.restartID,
		CheckpointBasisDigest: basisDigest,
		Nonce:                 checkpoint.resumeFallbackNonce,
		WakeCount:             wakeCount,
		ClearedAt:             clearedAt.UTC(),
	}
	wire := fallbackReceiptWire{
		RestartID:             receipt.RestartID,
		CheckpointBasisDigest: receipt.CheckpointBasisDigest,
		Nonce:                 receipt.Nonce,
		WakeCount:             receipt.WakeCount,
		ClearedAt:             receipt.ClearedAt.Format(time.RFC3339Nano),
	}
	content, err := canonicalRestartJSON(wire)
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	err = store.withExclusiveLock(func() error {
		current, loadErr := store.loadUnlocked()
		if loadErr != nil {
			return loadErr
		}
		if !checkpointBasisEqual(current, checkpoint) || current.state != StateResumed {
			return ErrConcurrentUpdate
		}
		path := store.restartReceiptPath(checkpoint.restartID, fallbackReceiptSuffix)
		return createPrivateRestartFile(path, content)
	})
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	return receipt, nil
}

func (store Store) LoadResumeFallbackReceipt(
	checkpoint Checkpoint,
) (ResumeFallbackReceipt, error) {
	path := store.restartReceiptPath(checkpoint.restartID, fallbackReceiptSuffix)
	content, err := readPrivateRestartJSON(path)
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	var wire fallbackReceiptWire
	if err := decodeStrictRestartJSON(content, &wire); err != nil {
		return ResumeFallbackReceipt{}, err
	}
	clearedAt, err := time.Parse(time.RFC3339Nano, wire.ClearedAt)
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	receipt := ResumeFallbackReceipt{
		RestartID:             wire.RestartID,
		CheckpointBasisDigest: wire.CheckpointBasisDigest,
		Nonce:                 wire.Nonce,
		WakeCount:             wire.WakeCount,
		ClearedAt:             clearedAt.UTC(),
	}
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		return ResumeFallbackReceipt{}, err
	}
	if receipt.RestartID != checkpoint.restartID ||
		receipt.CheckpointBasisDigest != basisDigest ||
		receipt.Nonce != checkpoint.resumeFallbackNonce ||
		receipt.WakeCount > 1 ||
		!receipt.ClearedAt.After(checkpoint.createdAt) {
		return ResumeFallbackReceipt{}, fmt.Errorf("resume fallback receipt does not match checkpoint")
	}
	return receipt, nil
}

func (store Store) loadLiveMCPChallengeUnlocked(
	checkpoint Checkpoint,
) (liveMCPChallenge, []byte, error) {
	path := store.restartReceiptPath(checkpoint.restartID, liveMCPChallengeSuffix)
	content, err := readPrivateRestartJSON(path)
	if err != nil {
		return liveMCPChallenge{}, nil, err
	}
	var challenge liveMCPChallenge
	if err := decodeStrictRestartJSON(content, &challenge); err != nil {
		return liveMCPChallenge{}, nil, err
	}
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		return liveMCPChallenge{}, nil, err
	}
	if challenge.RestartID != checkpoint.restartID ||
		challenge.CheckpointBasisDigest != basisDigest ||
		challenge.Nonce != checkpoint.liveMCPChallengeNonce {
		return liveMCPChallenge{}, nil, fmt.Errorf("live MCP challenge does not match checkpoint")
	}
	if challenge.State != liveMCPChallengePrepared && challenge.State != liveMCPChallengeBound {
		return liveMCPChallenge{}, nil, fmt.Errorf("live MCP challenge state is invalid")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, challenge.CreatedAt)
	if err != nil || !createdAt.UTC().Equal(checkpoint.createdAt) {
		return liveMCPChallenge{}, nil, fmt.Errorf("live MCP challenge creation time is invalid")
	}
	if err := challenge.validateState(); err != nil {
		return liveMCPChallenge{}, nil, err
	}
	return challenge, content, nil
}

func (challenge liveMCPChallenge) validateState() error {
	boundFieldsEmpty := challenge.ExpectedPID == 0 &&
		challenge.ExpectedExecutable == "" &&
		challenge.ExpectedDigest == "" &&
		challenge.ExpectedProjectRoot == "" &&
		challenge.ExpectedStartedAt == "" &&
		challenge.BoundAt == ""
	if challenge.State == liveMCPChallengePrepared {
		if !boundFieldsEmpty {
			return fmt.Errorf("prepared live MCP challenge contains bound process fields")
		}
		return nil
	}
	if challenge.ExpectedPID <= 0 ||
		challenge.ExpectedExecutable == "" ||
		!exactSHA256Digest.MatchString(challenge.ExpectedDigest) ||
		challenge.ExpectedProjectRoot == "" {
		return fmt.Errorf("bound live MCP challenge process identity is incomplete")
	}
	startedAt, startedErr := time.Parse(time.RFC3339Nano, challenge.ExpectedStartedAt)
	boundAt, boundErr := time.Parse(time.RFC3339Nano, challenge.BoundAt)
	if startedErr != nil || boundErr != nil || boundAt.Before(startedAt) {
		return fmt.Errorf("bound live MCP challenge timestamps are invalid")
	}
	return nil
}

func (challenge liveMCPChallenge) matches(observed mcpProcessObservation) error {
	startedAt, err := time.Parse(time.RFC3339Nano, challenge.ExpectedStartedAt)
	if err != nil {
		return fmt.Errorf("live MCP challenge start time is invalid")
	}
	checks := []bool{
		challenge.ExpectedPID == observed.PID,
		challenge.ExpectedExecutable == observed.ExecutablePath,
		challenge.ExpectedDigest == observed.ExecutableDigest,
		challenge.ExpectedProjectRoot == observed.ProjectRoot,
		startedAt.UTC().Equal(observed.StartedAt.UTC()),
	}
	for _, matches := range checks {
		if !matches {
			return fmt.Errorf("live MCP process does not match the bound challenge")
		}
	}
	return nil
}

func validateLiveMCPReceipt(checkpoint Checkpoint, receipt LiveMCPReceipt) error {
	basisDigest, err := checkpoint.BasisDigest()
	if err != nil {
		return err
	}
	if receipt.RestartID != checkpoint.restartID ||
		receipt.CheckpointBasisDigest != basisDigest ||
		receipt.Nonce != checkpoint.liveMCPChallengeNonce {
		return fmt.Errorf("live MCP receipt does not match checkpoint")
	}
	if receipt.PID <= 0 || receipt.ExecutablePath != checkpoint.expectedHaftBinaryPath ||
		receipt.ExecutableDigest != checkpoint.desiredHaftBinaryDigest ||
		receipt.ProjectRoot != checkpoint.repositoryRoot ||
		!receipt.ProcessStartedAt.After(checkpoint.createdAt) ||
		receipt.FulfilledAt.Before(receipt.ProcessStartedAt) {
		return fmt.Errorf("live MCP receipt process identity is invalid")
	}
	return nil
}

func (receipt LiveMCPReceipt) matchesChallengeAndProcess(
	challenge liveMCPChallenge,
	observed mcpProcessObservation,
) error {
	checks := []bool{
		receipt.RestartID == challenge.RestartID,
		receipt.CheckpointBasisDigest == challenge.CheckpointBasisDigest,
		receipt.Nonce == challenge.Nonce,
		receipt.PID == observed.PID,
		receipt.ExecutablePath == observed.ExecutablePath,
		receipt.ExecutableDigest == observed.ExecutableDigest,
		receipt.ProjectRoot == observed.ProjectRoot,
		receipt.ProcessStartedAt.Equal(observed.StartedAt),
		!receipt.FulfilledAt.Before(observed.StartedAt),
	}
	for _, matches := range checks {
		if !matches {
			return fmt.Errorf("existing live MCP receipt differs from bound process")
		}
	}
	return nil
}

func (store Store) restartReceiptPath(restartID string, suffix string) string {
	return filepath.Join(store.directoryPath, restartID+suffix)
}

func createPrivateRestartFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(privateFileMode); err != nil {
		_ = file.Close()
		return err
	}
	if err := requirePrivateRegularFile(file, "restart receipt"); err != nil {
		_ = file.Close()
		return err
	}
	writeErr := writeAndSync(file, content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return syncDirectory(filepath.Dir(path))
}

func replacePrivateRestartFileCAS(path string, expected []byte, proposed []byte) error {
	current, err := readPrivateRestartJSON(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return ErrConcurrentUpdate
	}
	stage, err := os.CreateTemp(
		filepath.Dir(path),
		restartReceiptStagePrefix,
	)
	if err != nil {
		return err
	}
	stagePath := stage.Name()
	defer func() {
		_ = os.Remove(stagePath)
	}()
	if err := stage.Chmod(0o600); err != nil {
		_ = stage.Close()
		return err
	}
	writeErr := writeAndSync(stage, proposed)
	closeErr := stage.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(stagePath, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func readPrivateRestartJSON(path string) ([]byte, error) {
	return readPrivateRegularNoFollow(
		path,
		restartReceiptMaxBytes,
		"restart receipt",
	)
}

func canonicalRestartJSON(value any) ([]byte, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func decodeStrictRestartJSON(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("restart receipt has trailing JSON")
	}
	return nil
}

func randomRestartNonce() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
