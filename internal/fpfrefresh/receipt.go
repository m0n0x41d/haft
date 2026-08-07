// Package fpfrefresh contains the bounded preparation and recovery primitives
// used to refresh Haft's pinned FPF source and its derived database.
package fpfrefresh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	// ApplyReceiptSchema identifies the only receipt representation accepted by
	// this package.
	ApplyReceiptSchema = "haft.fpf-refresh.apply-receipt/v1"

	// MaximumApplyReceiptBytes bounds receipt reads before JSON decoding.
	MaximumApplyReceiptBytes = 64 << 10
)

var (
	ErrReceiptInvalid    = errors.New("FPF refresh receipt is invalid")
	ErrReceiptCorrupt    = errors.New("FPF refresh receipt is corrupt")
	ErrReceiptNotFound   = errors.New("FPF refresh receipt was not found")
	ErrReceiptExists     = errors.New("FPF refresh receipt already exists")
	ErrReceiptBusy       = errors.New("FPF refresh receipt is busy")
	ErrReceiptStale      = errors.New("FPF refresh receipt has a stale basis")
	ErrReceiptTransition = errors.New("FPF refresh receipt transition is invalid")
)

var (
	exactReceiptGitSHA       = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	exactReceiptSHA256Digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ReceiptState is the closed durable state of one FPF refresh apply.
type ReceiptState string

const (
	ReceiptStatePrepared      ReceiptState = "prepared"
	ReceiptStateSourceApplied ReceiptState = "source-applied"
	ReceiptStateDBApplied     ReceiptState = "db-applied"
	ReceiptStateLockApplied   ReceiptState = "lock-applied"
	ReceiptStateVerified      ReceiptState = "verified"
	ReceiptStateComplete      ReceiptState = "complete"
	ReceiptStateRestored      ReceiptState = "restored"
)

// ReceiptCoordinates bind one exact FPF source revision to one exact derived
// database. The database digest always includes the sha256: prefix.
type ReceiptCoordinates struct {
	SourceSHA      string `json:"source_sha"`
	DatabaseDigest string `json:"database_digest"`
}

// ReceiptTargets are the only repository objects an apply or restore operation
// may mutate. Every path is absolute, clean, and distinct.
type ReceiptTargets struct {
	SourcePath           string `json:"source_path"`
	DatabasePath         string `json:"database_path"`
	LockPath             string `json:"lock_path"`
	TokenGateFixturePath string `json:"token_gate_fixture_path,omitempty"`
}

// ReceiptLockPresence closes the predecessor-lock alternatives. A first-ever
// generated lock is missing; an existing lock is present and has exact backup
// bytes.
type ReceiptLockPresence string

const (
	ReceiptLockMissing ReceiptLockPresence = "missing"
	ReceiptLockPresent ReceiptLockPresence = "present"
)

// ReceiptPredecessorLock records whether the target lock existed before apply.
// Present locks carry the only backup path and digest that restore may use.
// Missing locks carry neither and restore by removing the candidate lock.
type ReceiptPredecessorLock struct {
	Presence   ReceiptLockPresence `json:"presence"`
	BackupPath string              `json:"backup_path,omitempty"`
	Digest     string              `json:"digest,omitempty"`
}

// ReceiptArtifacts identify the already-prepared byte sources used by resume
// and restore. The candidate database, candidate lock, and predecessor
// database backup are always exact files. The predecessor lock explicitly
// distinguishes present backup bytes from a first-ever missing lock.
type ReceiptArtifacts struct {
	CandidateDatabasePath               string                 `json:"candidate_database_path"`
	CandidateLockPath                   string                 `json:"candidate_lock_path"`
	CandidateLockDigest                 string                 `json:"candidate_lock_digest"`
	PredecessorDatabaseBackupPath       string                 `json:"predecessor_database_backup_path"`
	PredecessorLock                     ReceiptPredecessorLock `json:"predecessor_lock"`
	CandidateTokenGateFixturePath       string                 `json:"candidate_token_gate_fixture_path,omitempty"`
	CandidateTokenGateFixtureDigest     string                 `json:"candidate_token_gate_fixture_digest,omitempty"`
	PredecessorTokenGateFixturePresence ReceiptLockPresence    `json:"predecessor_token_gate_fixture_presence,omitempty"`
	PredecessorTokenGateFixturePath     string                 `json:"predecessor_token_gate_fixture_path,omitempty"`
	PredecessorTokenGateFixtureDigest   string                 `json:"predecessor_token_gate_fixture_digest,omitempty"`
}

// ReceiptBasis is the immutable identity of one refresh attempt.
// InitialSourceSHA is the observed submodule worktree HEAD at preparation. It
// may already be the candidate while the parent gitlink and database remain at
// the predecessor, but no unrelated third revision is accepted.
type ReceiptBasis struct {
	Predecessor      ReceiptCoordinates `json:"predecessor"`
	Candidate        ReceiptCoordinates `json:"candidate"`
	InitialSourceSHA string             `json:"initial_source_sha"`
	Targets          ReceiptTargets     `json:"targets"`
	Artifacts        ReceiptArtifacts   `json:"artifacts"`
}

// ApplyReceipt records one recoverable apply. State is the only field that may
// change after creation.
type ApplyReceipt struct {
	Schema           string             `json:"schema"`
	State            ReceiptState       `json:"state"`
	Predecessor      ReceiptCoordinates `json:"predecessor"`
	Candidate        ReceiptCoordinates `json:"candidate"`
	InitialSourceSHA string             `json:"initial_source_sha"`
	Targets          ReceiptTargets     `json:"targets"`
	Artifacts        ReceiptArtifacts   `json:"artifacts"`
}

// ReceiptRecoveryStepKind is one closed, effect-free recovery instruction.
// This package describes these steps but never performs their Git, database,
// generated-lock, or verification effects.
type ReceiptRecoveryStepKind string

const (
	RecoveryApplyCandidateSource        ReceiptRecoveryStepKind = "apply-candidate-source"
	RecoveryApplyCandidateDatabase      ReceiptRecoveryStepKind = "apply-candidate-database"
	RecoveryApplyCandidateTokenGate     ReceiptRecoveryStepKind = "apply-candidate-token-gate-fixture"
	RecoveryMaterializeCandidateLock    ReceiptRecoveryStepKind = "materialize-candidate-lock"
	RecoveryVerifyCandidatePair         ReceiptRecoveryStepKind = "verify-candidate-pair"
	RecoveryMarkReceiptComplete         ReceiptRecoveryStepKind = "mark-receipt-complete"
	RecoveryMarkReceiptRestored         ReceiptRecoveryStepKind = "mark-receipt-restored"
	RecoveryRestorePredecessorLock      ReceiptRecoveryStepKind = "restore-predecessor-lock"
	RecoveryRemoveCandidateLock         ReceiptRecoveryStepKind = "remove-candidate-lock"
	RecoveryRestorePredecessorDatabase  ReceiptRecoveryStepKind = "restore-predecessor-database"
	RecoveryRestorePredecessorTokenGate ReceiptRecoveryStepKind = "restore-predecessor-token-gate-fixture" // #nosec G101 -- domain recovery-step label, not a credential.
	RecoveryRestorePredecessorSource    ReceiptRecoveryStepKind = "restore-predecessor-source"
	RecoveryVerifyPredecessorPair       ReceiptRecoveryStepKind = "verify-predecessor-pair"
)

// ReceiptRecoveryStep names one exact next effect. TargetPaths never contains a
// path outside the receipt. SourceSHA and DatabaseDigest are populated only
// when the step needs that identity. ArtifactPath and ArtifactDigest identify
// exact prepared bytes; LockPresence makes an absent predecessor lock explicit.
// ExpectedSourceSHA, ExpectedDatabaseDigest, ExpectedArtifactDigest, and
// ExpectedLockPresence make effects idempotent: an executor accepts either the
// expected or desired identity, performs work only for the expected identity,
// and verifies the desired identity before advancing. ResultState is populated
// only for legal forward receipt transitions.
type ReceiptRecoveryStep struct {
	Kind                   ReceiptRecoveryStepKind
	TargetPaths            []string
	ExpectedSourceSHA      string
	SourceSHA              string
	ExpectedDatabaseDigest string
	DatabaseDigest         string
	ArtifactPath           string
	ExpectedArtifactDigest string
	ArtifactDigest         string
	ExpectedLockPresence   ReceiptLockPresence
	LockPresence           ReceiptLockPresence
	ResultState            ReceiptState
}

// ReceiptRecoveryDirections provide both deterministic continuations from one
// observed state. Required is false only after a terminal complete or restored
// receipt.
type ReceiptRecoveryDirections struct {
	CurrentState ReceiptState
	Required     bool
	Resume       []ReceiptRecoveryStep
	Restore      []ReceiptRecoveryStep
}

// NewApplyReceipt validates an exact basis and creates its prepared receipt.
func NewApplyReceipt(basis ReceiptBasis) (ApplyReceipt, error) {
	if err := basis.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	receipt := ApplyReceipt{
		Schema:           ApplyReceiptSchema,
		State:            ReceiptStatePrepared,
		Predecessor:      basis.Predecessor,
		Candidate:        basis.Candidate,
		InitialSourceSHA: basis.InitialSourceSHA,
		Targets:          basis.Targets,
		Artifacts:        basis.Artifacts,
	}
	if err := receipt.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	return receipt, nil
}

// Validate checks the immutable receipt basis and its closed state.
func (receipt ApplyReceipt) Validate() error {
	if receipt.Schema != ApplyReceiptSchema {
		return invalidReceiptError("schema %q is not supported", receipt.Schema)
	}
	if !validReceiptState(receipt.State) {
		return invalidReceiptError("state %q is not defined", receipt.State)
	}
	return receipt.Basis().Validate()
}

// Validate checks one immutable refresh identity.
func (basis ReceiptBasis) Validate() error {
	if err := validateReceiptCoordinates("predecessor", basis.Predecessor); err != nil {
		return err
	}
	if err := validateReceiptCoordinates("candidate", basis.Candidate); err != nil {
		return err
	}
	if !exactReceiptGitSHA.MatchString(basis.InitialSourceSHA) {
		return invalidReceiptError("initial source SHA is not exact")
	}
	if basis.InitialSourceSHA != basis.Predecessor.SourceSHA &&
		basis.InitialSourceSHA != basis.Candidate.SourceSHA {
		return invalidReceiptError(
			"initial source SHA is neither predecessor nor candidate",
		)
	}
	if err := validateReceiptTargets(basis.Targets); err != nil {
		return err
	}
	if err := validateReceiptArtifacts(basis.Artifacts); err != nil {
		return err
	}
	tokenGateBound := basis.Artifacts.CandidateTokenGateFixturePath != ""
	if tokenGateBound != (basis.Targets.TokenGateFixturePath != "") {
		return invalidReceiptError(
			"token-gate fixture target and prepared artifacts must be present together",
		)
	}
	return validateReceiptBasisSeparation(basis)
}

// Basis returns the immutable part of a receipt.
func (receipt ApplyReceipt) Basis() ReceiptBasis {
	return ReceiptBasis{
		Predecessor:      receipt.Predecessor,
		Candidate:        receipt.Candidate,
		InitialSourceSHA: receipt.InitialSourceSHA,
		Targets:          receipt.Targets,
		Artifacts:        receipt.Artifacts,
	}
}

// CanonicalJSON returns deterministic JSON with one terminal newline.
func (receipt ApplyReceipt) CanonicalJSON() ([]byte, error) {
	if err := receipt.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return nil, invalidReceiptError("encode canonical JSON: %v", err)
	}
	return append(encoded, '\n'), nil
}

// DecodeApplyReceipt accepts only strict, canonical receipt JSON.
func DecodeApplyReceipt(content []byte) (ApplyReceipt, error) {
	if len(content) == 0 {
		return ApplyReceipt{}, corruptReceiptError("receipt is empty")
	}
	if len(content) > MaximumApplyReceiptBytes {
		return ApplyReceipt{}, corruptReceiptError(
			"receipt exceeds %d bytes",
			MaximumApplyReceiptBytes,
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var receipt ApplyReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return ApplyReceipt{}, corruptReceiptError("decode JSON: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ApplyReceipt{}, corruptReceiptError("receipt has trailing JSON")
	}
	if err := receipt.Validate(); err != nil {
		return ApplyReceipt{}, fmt.Errorf("%w: %v", ErrReceiptCorrupt, err)
	}
	canonical, err := receipt.CanonicalJSON()
	if err != nil {
		return ApplyReceipt{}, fmt.Errorf("%w: %v", ErrReceiptCorrupt, err)
	}
	if !bytes.Equal(content, canonical) {
		return ApplyReceipt{}, corruptReceiptError("receipt is not canonical JSON")
	}
	return receipt, nil
}

// TransitionApplyReceipt applies one legal transition. Repeating the current
// state is idempotent. Candidate application follows the linear forward chain;
// after externally verified predecessor restoration, any non-terminal state
// may close as restored. Terminal receipts cannot be reopened or crossed.
func TransitionApplyReceipt(
	receipt ApplyReceipt,
	next ReceiptState,
) (ApplyReceipt, error) {
	if err := receipt.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	if receipt.State == next {
		return receipt, nil
	}
	if next == ReceiptStateRestored &&
		receipt.State != ReceiptStateComplete &&
		receipt.State != ReceiptStateRestored {
		receipt.State = next
		if err := receipt.Validate(); err != nil {
			return ApplyReceipt{}, err
		}
		return receipt, nil
	}
	expected, ok := nextReceiptState(receipt.State)
	if !ok || next != expected {
		return ApplyReceipt{}, fmt.Errorf(
			"%w: cannot move from %q to %q",
			ErrReceiptTransition,
			receipt.State,
			next,
		)
	}
	receipt.State = next
	if err := receipt.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	return receipt, nil
}

// RecoveryDirections returns the exact remaining forward steps and the exact
// reverse-to-predecessor steps for the receipt's current state. Restore includes
// the next mutation that might have completed before its receipt transition,
// so interruption between an effect and its atomic state write is recoverable.
func (receipt ApplyReceipt) RecoveryDirections() (ReceiptRecoveryDirections, error) {
	if err := receipt.Validate(); err != nil {
		return ReceiptRecoveryDirections{}, err
	}

	resume := receiptResumeSteps(receipt)
	restore := receiptRestoreSteps(receipt)
	return ReceiptRecoveryDirections{
		CurrentState: receipt.State,
		Required: receipt.State != ReceiptStateComplete &&
			receipt.State != ReceiptStateRestored,
		Resume:  resume,
		Restore: restore,
	}, nil
}

// CreateReceipt exclusively publishes one prepared receipt. A concurrent or
// existing receipt is never overwritten, including when it has the same basis.
func CreateReceipt(path string, receipt ApplyReceipt) error {
	if err := validateReceiptStoragePath(path); err != nil {
		return err
	}
	if err := receipt.Validate(); err != nil {
		return err
	}
	if receipt.State != ReceiptStatePrepared {
		return invalidReceiptError(
			"new receipt state is %q, want %q",
			receipt.State,
			ReceiptStatePrepared,
		)
	}
	if err := validateReceiptStorageSeparation(path, receipt.Basis()); err != nil {
		return err
	}
	canonical, err := receipt.CanonicalJSON()
	if err != nil {
		return err
	}

	return withReceiptExclusiveLock(path, func() error {
		_, err := os.Lstat(path)
		switch {
		case err == nil:
			return fmt.Errorf("%w: %s", ErrReceiptExists, path)
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("inspect FPF refresh receipt %s: %w", path, err)
		}
		return writeReceiptAtomic(path, canonical)
	})
}

// LoadReceipt reads, strictly decodes, and validates one receipt.
func LoadReceipt(path string) (ApplyReceipt, error) {
	if err := validateReceiptStoragePath(path); err != nil {
		return ApplyReceipt{}, err
	}
	content, err := readReceiptBytes(path)
	if err != nil {
		return ApplyReceipt{}, err
	}
	receipt, err := DecodeApplyReceipt(content)
	if err != nil {
		return ApplyReceipt{}, fmt.Errorf("load FPF refresh receipt %s: %w", path, err)
	}
	if err := validateReceiptStorageSeparation(path, receipt.Basis()); err != nil {
		return ApplyReceipt{}, fmt.Errorf("%w: %v", ErrReceiptCorrupt, err)
	}
	return receipt, nil
}

// LoadReceiptFor loads a receipt and rejects a valid receipt that belongs to a
// different exact refresh basis.
func LoadReceiptFor(path string, expected ReceiptBasis) (ApplyReceipt, error) {
	if err := expected.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	receipt, err := LoadReceipt(path)
	if err != nil {
		return ApplyReceipt{}, err
	}
	if receipt.Basis() != expected {
		return ApplyReceipt{}, fmt.Errorf(
			"%w: receipt %s belongs to another exact refresh",
			ErrReceiptStale,
			path,
		)
	}
	return receipt, nil
}

// AdvanceReceipt atomically persists one legal transition after checking that
// the live receipt still belongs to the caller's exact basis. Replaying an
// already-persisted target state is a successful no-op.
func AdvanceReceipt(
	path string,
	expected ReceiptBasis,
	next ReceiptState,
) (ApplyReceipt, error) {
	if err := validateReceiptStoragePath(path); err != nil {
		return ApplyReceipt{}, err
	}
	if err := expected.Validate(); err != nil {
		return ApplyReceipt{}, err
	}
	if err := validateReceiptStorageSeparation(path, expected); err != nil {
		return ApplyReceipt{}, err
	}

	var advanced ApplyReceipt
	err := withReceiptExclusiveLock(path, func() error {
		current, err := LoadReceipt(path)
		if err != nil {
			return err
		}
		if current.Basis() != expected {
			return fmt.Errorf(
				"%w: receipt %s belongs to another exact refresh",
				ErrReceiptStale,
				path,
			)
		}
		advanced, err = TransitionApplyReceipt(current, next)
		if err != nil {
			return err
		}
		if advanced.State == current.State {
			return nil
		}
		canonical, err := advanced.CanonicalJSON()
		if err != nil {
			return err
		}
		return writeReceiptAtomic(path, canonical)
	})
	if err != nil {
		return ApplyReceipt{}, err
	}
	return advanced, nil
}

func validReceiptState(state ReceiptState) bool {
	switch state {
	case ReceiptStatePrepared,
		ReceiptStateSourceApplied,
		ReceiptStateDBApplied,
		ReceiptStateLockApplied,
		ReceiptStateVerified,
		ReceiptStateComplete,
		ReceiptStateRestored:
		return true
	default:
		return false
	}
}

func nextReceiptState(state ReceiptState) (ReceiptState, bool) {
	switch state {
	case ReceiptStatePrepared:
		return ReceiptStateSourceApplied, true
	case ReceiptStateSourceApplied:
		return ReceiptStateDBApplied, true
	case ReceiptStateDBApplied:
		return ReceiptStateLockApplied, true
	case ReceiptStateLockApplied:
		return ReceiptStateVerified, true
	case ReceiptStateVerified:
		return ReceiptStateComplete, true
	default:
		return "", false
	}
}

func validateReceiptCoordinates(label string, coordinates ReceiptCoordinates) error {
	if !exactReceiptGitSHA.MatchString(coordinates.SourceSHA) {
		return invalidReceiptError("%s source SHA is not exact", label)
	}
	if !exactReceiptSHA256Digest.MatchString(coordinates.DatabaseDigest) {
		return invalidReceiptError("%s database digest is not exact", label)
	}
	return nil
}

func validateReceiptTargets(targets ReceiptTargets) error {
	entries := []struct {
		label string
		path  string
	}{
		{label: "source", path: targets.SourcePath},
		{label: "database", path: targets.DatabasePath},
		{label: "lock", path: targets.LockPath},
	}
	if targets.TokenGateFixturePath != "" {
		entries = append(entries, struct {
			label string
			path  string
		}{
			label: "token-gate fixture",
			path:  targets.TokenGateFixturePath,
		})
	}
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if err := validateExactReceiptPath(entry.label+" target", entry.path); err != nil {
			return err
		}
		if previous, exists := seen[entry.path]; exists {
			return invalidReceiptError(
				"%s and %s targets are the same path",
				previous,
				entry.label,
			)
		}
		seen[entry.path] = entry.label
	}
	if receiptPathWithin(targets.DatabasePath, targets.SourcePath) ||
		receiptPathWithin(targets.LockPath, targets.SourcePath) ||
		(targets.TokenGateFixturePath != "" &&
			receiptPathWithin(targets.TokenGateFixturePath, targets.SourcePath)) {
		return invalidReceiptError(
			"database, lock, and token-gate targets must be outside the source target",
		)
	}
	return nil
}

func validateReceiptArtifacts(artifacts ReceiptArtifacts) error {
	if err := validateExactReceiptPath(
		"candidate database artifact",
		artifacts.CandidateDatabasePath,
	); err != nil {
		return err
	}
	if err := validateExactReceiptPath(
		"candidate lock artifact",
		artifacts.CandidateLockPath,
	); err != nil {
		return err
	}
	if !exactReceiptSHA256Digest.MatchString(artifacts.CandidateLockDigest) {
		return invalidReceiptError("candidate lock artifact digest is not exact")
	}
	if err := validateExactReceiptPath(
		"predecessor database backup",
		artifacts.PredecessorDatabaseBackupPath,
	); err != nil {
		return err
	}
	artifactPaths := []string{
		artifacts.CandidateDatabasePath,
		artifacts.CandidateLockPath,
		artifacts.PredecessorDatabaseBackupPath,
	}
	tokenGateBound := artifacts.CandidateTokenGateFixturePath != "" ||
		artifacts.CandidateTokenGateFixtureDigest != "" ||
		artifacts.PredecessorTokenGateFixturePresence != "" ||
		artifacts.PredecessorTokenGateFixturePath != "" ||
		artifacts.PredecessorTokenGateFixtureDigest != ""
	if tokenGateBound {
		if err := validateExactReceiptPath(
			"candidate token-gate fixture artifact",
			artifacts.CandidateTokenGateFixturePath,
		); err != nil {
			return err
		}
		if !exactReceiptSHA256Digest.MatchString(
			artifacts.CandidateTokenGateFixtureDigest,
		) {
			return invalidReceiptError(
				"candidate token-gate fixture digest is not exact",
			)
		}
		artifactPaths = append(
			artifactPaths,
			artifacts.CandidateTokenGateFixturePath,
		)
		switch artifacts.PredecessorTokenGateFixturePresence {
		case ReceiptLockMissing:
			if artifacts.PredecessorTokenGateFixturePath != "" ||
				artifacts.PredecessorTokenGateFixtureDigest != "" {
				return invalidReceiptError(
					"missing predecessor token-gate fixture carries backup identity",
				)
			}
		case ReceiptLockPresent:
			if err := validateExactReceiptPath(
				"predecessor token-gate fixture backup",
				artifacts.PredecessorTokenGateFixturePath,
			); err != nil {
				return err
			}
			if !exactReceiptSHA256Digest.MatchString(
				artifacts.PredecessorTokenGateFixtureDigest,
			) {
				return invalidReceiptError(
					"predecessor token-gate fixture digest is not exact",
				)
			}
			artifactPaths = append(
				artifactPaths,
				artifacts.PredecessorTokenGateFixturePath,
			)
		default:
			return invalidReceiptError(
				"predecessor token-gate fixture presence %q is not defined",
				artifacts.PredecessorTokenGateFixturePresence,
			)
		}
	}
	if artifacts.PredecessorLock.Presence == ReceiptLockMissing &&
		artifacts.PredecessorTokenGateFixturePresence == ReceiptLockPresent {
		return invalidReceiptError(
			"missing predecessor lock cannot bind a predecessor token-gate fixture",
		)
	}
	for left := range artifactPaths {
		for right := left + 1; right < len(artifactPaths); right++ {
			if artifactPaths[left] == artifactPaths[right] {
				return invalidReceiptError(
					"prepared artifacts are the same path %q",
					artifactPaths[left],
				)
			}
		}
	}

	switch artifacts.PredecessorLock.Presence {
	case ReceiptLockMissing:
		if artifacts.PredecessorLock.BackupPath != "" ||
			artifacts.PredecessorLock.Digest != "" {
			return invalidReceiptError(
				"missing predecessor lock carries backup identity",
			)
		}
	case ReceiptLockPresent:
		if err := validateExactReceiptPath(
			"predecessor lock backup",
			artifacts.PredecessorLock.BackupPath,
		); err != nil {
			return err
		}
		if !exactReceiptSHA256Digest.MatchString(artifacts.PredecessorLock.Digest) {
			return invalidReceiptError(
				"predecessor lock backup digest is not exact",
			)
		}
		for _, artifactPath := range artifactPaths {
			if artifacts.PredecessorLock.BackupPath == artifactPath {
				return invalidReceiptError(
					"predecessor lock backup collides with prepared artifact %q",
					artifactPath,
				)
			}
		}
	default:
		return invalidReceiptError(
			"predecessor lock presence %q is not defined",
			artifacts.PredecessorLock.Presence,
		)
	}
	return nil
}

func validateReceiptBasisSeparation(basis ReceiptBasis) error {
	targets := []string{
		basis.Targets.SourcePath,
		basis.Targets.DatabasePath,
		basis.Targets.LockPath,
	}
	if basis.Targets.TokenGateFixturePath != "" {
		targets = append(targets, basis.Targets.TokenGateFixturePath)
	}
	artifacts := []string{
		basis.Artifacts.CandidateDatabasePath,
		basis.Artifacts.CandidateLockPath,
		basis.Artifacts.PredecessorDatabaseBackupPath,
	}
	if basis.Artifacts.CandidateTokenGateFixturePath != "" {
		artifacts = append(
			artifacts,
			basis.Artifacts.CandidateTokenGateFixturePath,
		)
	}
	if basis.Artifacts.PredecessorTokenGateFixturePresence == ReceiptLockPresent {
		artifacts = append(
			artifacts,
			basis.Artifacts.PredecessorTokenGateFixturePath,
		)
	}
	if basis.Artifacts.PredecessorLock.Presence == ReceiptLockPresent {
		artifacts = append(artifacts, basis.Artifacts.PredecessorLock.BackupPath)
	}
	for _, artifact := range artifacts {
		for _, target := range targets {
			if artifact == target {
				return invalidReceiptError(
					"prepared artifact %q collides with a receipt target",
					artifact,
				)
			}
		}
		if receiptPathWithin(artifact, basis.Targets.SourcePath) {
			return invalidReceiptError(
				"prepared artifact %q is inside the source target",
				artifact,
			)
		}
	}
	return nil
}

func validateReceiptStoragePath(path string) error {
	if err := validateExactReceiptPath("storage", path); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("inspect FPF refresh receipt directory %s: %w", parent, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return invalidReceiptError("storage directory %q is not a real directory", parent)
	}
	return nil
}

func validateExactReceiptPath(label string, path string) error {
	if path == "" ||
		!filepath.IsAbs(path) ||
		filepath.Clean(path) != path ||
		filepath.Dir(path) == path {
		return invalidReceiptError("%s path %q is not absolute and canonical", label, path)
	}
	return nil
}

func validateReceiptStorageSeparation(path string, basis ReceiptBasis) error {
	lockPath := path + ".lock"
	for _, target := range []string{
		basis.Targets.SourcePath,
		basis.Targets.DatabasePath,
		basis.Targets.LockPath,
		basis.Targets.TokenGateFixturePath,
	} {
		if target != "" && (path == target || lockPath == target) {
			return invalidReceiptError(
				"receipt storage path collides with target %q",
				target,
			)
		}
	}
	for _, artifact := range []string{
		basis.Artifacts.CandidateDatabasePath,
		basis.Artifacts.CandidateLockPath,
		basis.Artifacts.PredecessorDatabaseBackupPath,
		basis.Artifacts.PredecessorLock.BackupPath,
		basis.Artifacts.CandidateTokenGateFixturePath,
		basis.Artifacts.PredecessorTokenGateFixturePath,
	} {
		if artifact != "" && (path == artifact || lockPath == artifact) {
			return invalidReceiptError(
				"receipt storage path collides with prepared artifact %q",
				artifact,
			)
		}
	}
	if receiptPathWithin(path, basis.Targets.SourcePath) ||
		receiptPathWithin(lockPath, basis.Targets.SourcePath) {
		return invalidReceiptError(
			"receipt storage must be outside the source target",
		)
	}
	return nil
}

func receiptPathWithin(path string, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	if err != nil {
		return false
	}
	return relative == "." ||
		(relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func invalidReceiptError(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrReceiptInvalid, fmt.Sprintf(format, arguments...))
}

func corruptReceiptError(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrReceiptCorrupt, fmt.Sprintf(format, arguments...))
}

func receiptResumeSteps(receipt ApplyReceipt) []ReceiptRecoveryStep {
	targets := receipt.Targets
	candidate := receipt.Candidate
	artifacts := receipt.Artifacts
	all := []ReceiptRecoveryStep{
		{
			Kind:              RecoveryApplyCandidateSource,
			TargetPaths:       []string{targets.SourcePath},
			ExpectedSourceSHA: receipt.InitialSourceSHA,
			SourceSHA:         candidate.SourceSHA,
			ResultState:       ReceiptStateSourceApplied,
		},
		{
			Kind:                   RecoveryApplyCandidateDatabase,
			TargetPaths:            []string{targets.DatabasePath},
			ExpectedDatabaseDigest: receipt.Predecessor.DatabaseDigest,
			DatabaseDigest:         candidate.DatabaseDigest,
			ArtifactPath:           artifacts.CandidateDatabasePath,
			ArtifactDigest:         candidate.DatabaseDigest,
			ResultState:            ReceiptStateDBApplied,
		},
	}
	if artifacts.CandidateTokenGateFixturePath != "" {
		all = append(all, ReceiptRecoveryStep{
			Kind:                   RecoveryApplyCandidateTokenGate,
			TargetPaths:            []string{targets.TokenGateFixturePath},
			ArtifactPath:           artifacts.CandidateTokenGateFixturePath,
			ExpectedArtifactDigest: artifacts.PredecessorTokenGateFixtureDigest,
			ArtifactDigest:         artifacts.CandidateTokenGateFixtureDigest,
			ExpectedLockPresence:   artifacts.PredecessorTokenGateFixturePresence,
			LockPresence:           ReceiptLockPresent,
		})
	}
	lockStepIndex := len(all)
	all = append(all,
		ReceiptRecoveryStep{
			Kind:                   RecoveryMaterializeCandidateLock,
			TargetPaths:            []string{targets.LockPath},
			SourceSHA:              candidate.SourceSHA,
			DatabaseDigest:         candidate.DatabaseDigest,
			ArtifactPath:           artifacts.CandidateLockPath,
			ExpectedArtifactDigest: artifacts.PredecessorLock.Digest,
			ArtifactDigest:         artifacts.CandidateLockDigest,
			ExpectedLockPresence:   artifacts.PredecessorLock.Presence,
			LockPresence:           ReceiptLockPresent,
			ResultState:            ReceiptStateLockApplied,
		},
		ReceiptRecoveryStep{
			Kind: RecoveryVerifyCandidatePair,
			TargetPaths: []string{
				targets.SourcePath,
				targets.DatabasePath,
				targets.LockPath,
			},
			SourceSHA:      candidate.SourceSHA,
			DatabaseDigest: candidate.DatabaseDigest,
			ArtifactDigest: artifacts.CandidateLockDigest,
			LockPresence:   ReceiptLockPresent,
			ResultState:    ReceiptStateVerified,
		},
		ReceiptRecoveryStep{
			Kind:           RecoveryMarkReceiptComplete,
			TargetPaths:    []string{},
			SourceSHA:      candidate.SourceSHA,
			DatabaseDigest: candidate.DatabaseDigest,
			ArtifactDigest: artifacts.CandidateLockDigest,
			LockPresence:   ReceiptLockPresent,
			ResultState:    ReceiptStateComplete,
		},
	)
	if targets.TokenGateFixturePath != "" {
		all[lockStepIndex+1].TargetPaths = append(
			all[lockStepIndex+1].TargetPaths,
			targets.TokenGateFixturePath,
		)
	}

	start := len(all)
	switch receipt.State {
	case ReceiptStatePrepared:
		start = 0
	case ReceiptStateSourceApplied:
		start = 1
	case ReceiptStateDBApplied:
		start = 2
	case ReceiptStateLockApplied:
		start = lockStepIndex + 1
	case ReceiptStateVerified:
		start = len(all) - 1
	case ReceiptStateComplete, ReceiptStateRestored:
		start = len(all)
	}
	return cloneReceiptRecoverySteps(all[start:])
}

func receiptRestoreSteps(receipt ApplyReceipt) []ReceiptRecoveryStep {
	targets := receipt.Targets
	predecessor := receipt.Predecessor
	artifacts := receipt.Artifacts
	verify := ReceiptRecoveryStep{
		Kind: RecoveryVerifyPredecessorPair,
		TargetPaths: []string{
			targets.SourcePath,
			targets.DatabasePath,
			targets.LockPath,
		},
		SourceSHA:      predecessor.SourceSHA,
		DatabaseDigest: predecessor.DatabaseDigest,
		ArtifactDigest: artifacts.PredecessorLock.Digest,
		LockPresence:   artifacts.PredecessorLock.Presence,
	}
	if targets.TokenGateFixturePath != "" {
		verify.TargetPaths = append(verify.TargetPaths, targets.TokenGateFixturePath)
	}
	restoreSource := ReceiptRecoveryStep{
		Kind:              RecoveryRestorePredecessorSource,
		TargetPaths:       []string{targets.SourcePath},
		ExpectedSourceSHA: receipt.Candidate.SourceSHA,
		SourceSHA:         predecessor.SourceSHA,
	}
	restoreDatabase := ReceiptRecoveryStep{
		Kind:                   RecoveryRestorePredecessorDatabase,
		TargetPaths:            []string{targets.DatabasePath},
		ExpectedDatabaseDigest: receipt.Candidate.DatabaseDigest,
		DatabaseDigest:         predecessor.DatabaseDigest,
		ArtifactPath:           artifacts.PredecessorDatabaseBackupPath,
		ArtifactDigest:         predecessor.DatabaseDigest,
	}
	restoreLock := ReceiptRecoveryStep{
		TargetPaths:            []string{targets.LockPath},
		ExpectedArtifactDigest: artifacts.CandidateLockDigest,
		ExpectedLockPresence:   ReceiptLockPresent,
		LockPresence:           artifacts.PredecessorLock.Presence,
	}
	var restoreTokenGate []ReceiptRecoveryStep
	if artifacts.PredecessorTokenGateFixturePresence == ReceiptLockPresent {
		restoreTokenGate = []ReceiptRecoveryStep{{
			Kind:                   RecoveryRestorePredecessorTokenGate,
			TargetPaths:            []string{targets.TokenGateFixturePath},
			ArtifactPath:           artifacts.PredecessorTokenGateFixturePath,
			ExpectedArtifactDigest: artifacts.CandidateTokenGateFixtureDigest,
			ArtifactDigest:         artifacts.PredecessorTokenGateFixtureDigest,
			ExpectedLockPresence:   ReceiptLockPresent,
			LockPresence:           ReceiptLockPresent,
		}}
	}
	markRestored := ReceiptRecoveryStep{
		Kind:           RecoveryMarkReceiptRestored,
		TargetPaths:    []string{},
		SourceSHA:      predecessor.SourceSHA,
		DatabaseDigest: predecessor.DatabaseDigest,
		ArtifactDigest: artifacts.PredecessorLock.Digest,
		LockPresence:   artifacts.PredecessorLock.Presence,
		ResultState:    ReceiptStateRestored,
	}
	switch artifacts.PredecessorLock.Presence {
	case ReceiptLockMissing:
		restoreLock.Kind = RecoveryRemoveCandidateLock
	case ReceiptLockPresent:
		restoreLock.Kind = RecoveryRestorePredecessorLock
		restoreLock.SourceSHA = predecessor.SourceSHA
		restoreLock.DatabaseDigest = predecessor.DatabaseDigest
		restoreLock.ArtifactPath = artifacts.PredecessorLock.BackupPath
		restoreLock.ArtifactDigest = artifacts.PredecessorLock.Digest
	}

	switch receipt.State {
	case ReceiptStatePrepared:
		steps := append([]ReceiptRecoveryStep{}, restoreTokenGate...)
		steps = append(steps, restoreSource, verify, markRestored)
		return cloneReceiptRecoverySteps(steps)
	case ReceiptStateSourceApplied:
		steps := append([]ReceiptRecoveryStep{}, restoreTokenGate...)
		steps = append(steps, restoreDatabase, restoreSource, verify, markRestored)
		return cloneReceiptRecoverySteps(steps)
	case ReceiptStateDBApplied:
		steps := []ReceiptRecoveryStep{restoreLock}
		steps = append(steps, restoreTokenGate...)
		steps = append(steps, restoreDatabase, restoreSource, verify, markRestored)
		return cloneReceiptRecoverySteps(steps)
	case ReceiptStateLockApplied, ReceiptStateVerified:
		steps := []ReceiptRecoveryStep{restoreLock}
		steps = append(steps, restoreTokenGate...)
		steps = append(steps, restoreDatabase, restoreSource, verify, markRestored)
		return cloneReceiptRecoverySteps(steps)
	case ReceiptStateComplete, ReceiptStateRestored:
		return []ReceiptRecoveryStep{}
	default:
		return nil
	}
}

func cloneReceiptRecoverySteps(steps []ReceiptRecoveryStep) []ReceiptRecoveryStep {
	cloned := make([]ReceiptRecoveryStep, len(steps))
	for index, step := range steps {
		cloned[index] = step
		cloned[index].TargetPaths = append([]string(nil), step.TargetPaths...)
		if len(step.TargetPaths) == 0 {
			cloned[index].TargetPaths = []string{}
		}
	}
	return cloned
}

func withReceiptExclusiveLock(path string, effect func() error) (result error) {
	lockPath := path + ".lock"
	fd, err := unix.Open(
		lockPath,
		unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return fmt.Errorf("open FPF refresh receipt lock %s: %w", lockPath, err)
	}
	lock := os.NewFile(uintptr(fd), lockPath) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if lock == nil {
		_ = unix.Close(fd)
		return fmt.Errorf("adopt FPF refresh receipt lock %s", lockPath)
	}
	info, err := lock.Stat()
	if err != nil {
		_ = lock.Close()
		return fmt.Errorf("stat FPF refresh receipt lock %s: %w", lockPath, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		_ = lock.Close()
		return invalidReceiptError(
			"lock carrier %q is not a private regular file",
			lockPath,
		)
	}
	if err := unix.Flock(
		int(lock.Fd()), // #nosec G115 -- lock is an open file descriptor.
		unix.LOCK_EX|unix.LOCK_NB,
	); err != nil {
		_ = lock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("%w: %s", ErrReceiptBusy, path)
		}
		return fmt.Errorf("lock FPF refresh receipt %s: %w", lockPath, err)
	}
	defer func() {
		unlockErr := unix.Flock(
			int(lock.Fd()), // #nosec G115 -- lock remains open through the effect.
			unix.LOCK_UN,
		)
		closeErr := lock.Close()
		if unlockErr != nil {
			result = errors.Join(
				result,
				fmt.Errorf("unlock FPF refresh receipt %s: %w", lockPath, unlockErr),
			)
		}
		if closeErr != nil {
			result = errors.Join(
				result,
				fmt.Errorf("close FPF refresh receipt lock %s: %w", lockPath, closeErr),
			)
		}
	}()
	if err := lock.Sync(); err != nil {
		return fmt.Errorf("sync FPF refresh receipt lock %s: %w", lockPath, err)
	}
	if err := syncReceiptDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return effect()
}

func writeReceiptAtomic(path string, content []byte) error {
	parent := filepath.Dir(path)
	stage, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create FPF refresh receipt stage: %w", err)
	}
	stagePath := stage.Name()
	stagePresent := true
	defer func() {
		_ = stage.Close()
		if stagePresent {
			_ = os.Remove(stagePath)
		}
	}()

	if err := stage.Chmod(0o600); err != nil {
		return fmt.Errorf("set FPF refresh receipt stage mode: %w", err)
	}
	written, err := stage.Write(content)
	if err != nil {
		return fmt.Errorf("write FPF refresh receipt stage: %w", err)
	}
	if written != len(content) {
		return fmt.Errorf("write FPF refresh receipt stage: %w", io.ErrShortWrite)
	}
	if err := stage.Sync(); err != nil {
		return fmt.Errorf("sync FPF refresh receipt stage: %w", err)
	}
	if err := stage.Close(); err != nil {
		return fmt.Errorf("close FPF refresh receipt stage: %w", err)
	}
	if err := os.Rename(stagePath, path); err != nil {
		return fmt.Errorf("atomically install FPF refresh receipt: %w", err)
	}
	stagePresent = false
	if err := syncReceiptDirectory(parent); err != nil {
		return err
	}
	reread, err := readReceiptBytes(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(reread, content) {
		return fmt.Errorf("FPF refresh receipt reread differs from written bytes")
	}
	return nil
}

func readReceiptBytes(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrReceiptNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("inspect FPF refresh receipt %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, corruptReceiptError("receipt %s is not a regular file", path)
	}
	if info.Size() > MaximumApplyReceiptBytes {
		return nil, corruptReceiptError(
			"receipt %s exceeds %d bytes",
			path,
			MaximumApplyReceiptBytes,
		)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open FPF refresh receipt %s: %w", path, err)
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat opened FPF refresh receipt %s: %w", path, statErr)
	}
	if !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("%w: receipt %s changed while opening", ErrReceiptBusy, path)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, MaximumApplyReceiptBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read FPF refresh receipt %s: %w", path, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close FPF refresh receipt %s: %w", path, closeErr)
	}
	if len(content) > MaximumApplyReceiptBytes {
		return nil, corruptReceiptError(
			"receipt %s exceeds %d bytes",
			path,
			MaximumApplyReceiptBytes,
		)
	}
	return content, nil
}

func syncReceiptDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open FPF refresh receipt directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync FPF refresh receipt directory: %w", err)
	}
	return nil
}
