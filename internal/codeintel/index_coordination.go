package codeintel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/logger"
)

const (
	defaultIndexFollowerWait = 2 * time.Second
	indexFollowerReadReserve = 50 * time.Millisecond
	indexLeasePollInterval   = 10 * time.Millisecond
)

// IndexCoordinationOutcome is the closed public result of one index-freshness
// attempt. It deliberately has no generic "success" value: callers can retain
// the exact difference between freshness, publication, deferral, and absence.
type IndexCoordinationOutcome string

const (
	IndexAlreadyFresh         IndexCoordinationOutcome = "already_fresh"
	IndexRebuiltPublished     IndexCoordinationOutcome = "rebuilt_published"
	IndexFreshAfterWait       IndexCoordinationOutcome = "fresh_after_wait"
	IndexDeferredBusy         IndexCoordinationOutcome = "deferred_busy"
	IndexRetainedAfterFailure IndexCoordinationOutcome = "retained_after_failure"
	IndexNoCompleteEpoch      IndexCoordinationOutcome = "no_complete_epoch"
)

// IndexCoordinationResult describes the exact result without exposing lock
// carrier details. SourceFingerprint is empty only when a contender was not
// permitted to observe freshness because another owner held the lease.
type IndexCoordinationResult struct {
	Outcome           IndexCoordinationOutcome
	WaitDuration      time.Duration
	SourceFingerprint string
	PublishedEpoch    int64
	Reason            string
}

func (result IndexCoordinationResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Outcome           IndexCoordinationOutcome `json:"outcome"`
		WaitMilliseconds  int64                    `json:"wait_ms"`
		SourceFingerprint string                   `json:"source_fingerprint,omitempty"`
		PublishedEpoch    int64                    `json:"published_epoch"`
		Reason            string                   `json:"reason,omitempty"`
	}{
		Outcome:           result.Outcome,
		WaitMilliseconds:  result.WaitDuration.Milliseconds(),
		SourceFingerprint: result.SourceFingerprint,
		PublishedEpoch:    result.PublishedEpoch,
		Reason:            result.Reason,
	})
}

// Rebuilt reports whether this exact caller published a code-index epoch.
func (result IndexCoordinationResult) Rebuilt() bool {
	return result.Outcome == IndexRebuiltPublished
}

// EffectiveIndexState applies a bounded coordination failure to the public
// state without mutating the shared ledger. A healthy concurrent leader must
// not have its publication marked degraded by a follower that timed out.
func (result IndexCoordinationResult) EffectiveIndexState(
	state codebase.IndexState,
) codebase.IndexState {
	switch result.Outcome {
	case IndexDeferredBusy,
		IndexRetainedAfterFailure,
		IndexNoCompleteEpoch:
		state.Degraded = true
		state.DegradedReason = strings.TrimSpace(result.Reason)
		if state.DegradedReason == "" {
			state.DegradedReason = "code-index freshness was not established"
		}
	}
	return state
}

// ProjectIndexCoordinates are resolved by the checked project-ledger binding,
// never from model-supplied query arguments.
type ProjectIndexCoordinates struct {
	ProjectID   string
	ProjectRoot string
	LedgerPath  string
}

type projectIndexProcessState struct {
	gate          chan struct{}
	readMu        sync.RWMutex
	resultMu      sync.RWMutex
	lastResult    IndexCoordinationResult
	hasLastResult bool
}

var projectIndexProcessStates sync.Map

type indexCoordinatorHooks struct {
	leaseBusy     func()
	afterLease    func(context.Context) error
	beforeRefresh func(context.Context) error
	afterRefresh  func(context.Context, codebase.IndexRefreshResult, error) error
}

// ProjectIndexCoordinator owns the process-local gate and the coordinates of
// the cross-process advisory lease. Coordinators with identical canonical
// coordinates share the same process state even when independently created.
type ProjectIndexCoordinator struct {
	projectID   string
	projectRoot string
	ledgerPath  string
	lockDir     string
	lockName    string
	key         string
	state       *projectIndexProcessState
	bound       bool
	hooks       indexCoordinatorHooks
}

// NewProjectIndexCoordinator creates a coordinator from an already resolved
// ledger binding. It rejects aliases and non-regular ledger carriers before a
// lock path is derived.
func NewProjectIndexCoordinator(
	coordinates ProjectIndexCoordinates,
) (*ProjectIndexCoordinator, error) {
	projectID, err := projectidentity.ParseProjectID(coordinates.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("code-index project coordinate: %w", err)
	}
	root, err := requireCanonicalPhysicalDirectory(
		"code-index project root",
		coordinates.ProjectRoot,
	)
	if err != nil {
		return nil, err
	}
	ledgerPath, err := requireCanonicalRegularFile(
		"code-index ledger",
		coordinates.LedgerPath,
	)
	if err != nil {
		return nil, err
	}
	ledgerDir, err := requireCanonicalPhysicalDirectory(
		"code-index ledger directory",
		filepath.Dir(ledgerPath),
	)
	if err != nil {
		return nil, err
	}
	key := projectIndexCoordinateKey(projectID.String(), root, ledgerPath)
	stateValue, _ := projectIndexProcessStates.LoadOrStore(
		key,
		newProjectIndexProcessState(),
	)
	return &ProjectIndexCoordinator{
		projectID:   projectID.String(),
		projectRoot: root,
		ledgerPath:  ledgerPath,
		lockDir:     ledgerDir,
		lockName:    "code-index-rebuild.lock",
		key:         key,
		state:       stateValue.(*projectIndexProcessState),
		bound:       true,
	}, nil
}

func newProjectIndexProcessState() *projectIndexProcessState {
	state := &projectIndexProcessState{gate: make(chan struct{}, 1)}
	state.gate <- struct{}{}
	return state
}

func newProcessOnlyIndexCoordinator(projectRoot string) *ProjectIndexCoordinator {
	root := filepath.Clean(projectRoot)
	key := projectIndexCoordinateKey("process_only", root, "")
	stateValue, _ := projectIndexProcessStates.LoadOrStore(
		key,
		newProjectIndexProcessState(),
	)
	return &ProjectIndexCoordinator{
		projectID:   "process_only",
		projectRoot: root,
		key:         key,
		state:       stateValue.(*projectIndexProcessState),
	}
}

func projectIndexCoordinateKey(projectID, projectRoot, ledgerPath string) string {
	sum := sha256.Sum256([]byte(
		projectID + "\x00" + projectRoot + "\x00" + ledgerPath,
	))
	return hex.EncodeToString(sum[:])
}

func requireCanonicalPhysicalDirectory(label, raw string) (string, error) {
	if raw != strings.TrimSpace(raw) || !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return "", fmt.Errorf("%s must be a canonical absolute path", label)
	}
	info, err := os.Lstat(raw)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s must be a real directory", label)
	}
	physical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", fmt.Errorf("resolve physical %s: %w", label, err)
	}
	if filepath.Clean(physical) != raw {
		return "", fmt.Errorf("%s must use canonical physical form", label)
	}
	return raw, nil
}

func requireCanonicalRegularFile(label, raw string) (string, error) {
	if raw != strings.TrimSpace(raw) || !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return "", fmt.Errorf("%s must be a canonical absolute path", label)
	}
	info, err := os.Lstat(raw)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s must be a real regular file", label)
	}
	physical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", fmt.Errorf("resolve physical %s: %w", label, err)
	}
	if filepath.Clean(physical) != raw {
		return "", fmt.Errorf("%s must use canonical physical form", label)
	}
	return raw, nil
}

type indexAttemptPolicy int

const (
	indexAttemptRequest indexAttemptPolicy = iota
	indexAttemptStartup
)

type indexCoordinationPolicyInput struct {
	Fresh            bool
	LeaseAcquired    bool
	Contended        bool
	StartupDeferred  bool
	Failure          string
	RefreshComplete  bool
	RefreshPublished bool
	Epoch            int64
}

type indexCoordinationDecision struct {
	Outcome      IndexCoordinationOutcome
	EnterRebuild bool
	Reason       string
}

// decideIndexCoordination is the minimal computational core. In particular,
// EnterRebuild is impossible unless this caller owns the lease; followers can
// only return a closed non-parsing outcome.
func decideIndexCoordination(
	input indexCoordinationPolicyInput,
) indexCoordinationDecision {
	if input.StartupDeferred {
		return indexCoordinationDecision{Outcome: IndexDeferredBusy}
	}
	if strings.TrimSpace(input.Failure) != "" {
		outcome := IndexNoCompleteEpoch
		if input.Epoch > 0 {
			outcome = IndexRetainedAfterFailure
		}
		return indexCoordinationDecision{
			Outcome: outcome,
			Reason:  strings.TrimSpace(input.Failure),
		}
	}
	if input.Fresh {
		outcome := IndexAlreadyFresh
		if input.Contended {
			outcome = IndexFreshAfterWait
		}
		return indexCoordinationDecision{Outcome: outcome}
	}
	if !input.LeaseAcquired {
		outcome := IndexNoCompleteEpoch
		if input.Epoch > 0 {
			outcome = IndexRetainedAfterFailure
		}
		return indexCoordinationDecision{
			Outcome: outcome,
			Reason:  "code-index rebuild lease was not acquired",
		}
	}
	if !input.RefreshComplete {
		return indexCoordinationDecision{EnterRebuild: true}
	}
	if input.RefreshPublished {
		return indexCoordinationDecision{Outcome: IndexRebuiltPublished}
	}
	// A successful no-publication refresh means the corpus was semantically
	// unchanged. The shell normalizes the cheap fingerprint before reaching
	// this branch, so it is an already-fresh result rather than a rebuild.
	outcome := IndexAlreadyFresh
	if input.Contended {
		outcome = IndexFreshAfterWait
	}
	return indexCoordinationDecision{Outcome: outcome}
}

func indexObservationIsFresh(
	observation codebase.IndexFreshnessObservation,
) bool {
	return observation.PublishedEpoch > 0 &&
		!observation.Degraded &&
		observation.SourceFingerprint != "" &&
		observation.SourceFingerprint == observation.StoredSourceFingerprint &&
		observation.ConfigFingerprint == observation.StoredConfigFingerprint &&
		observation.CurrentSchemaVersion == observation.StoredSchemaVersion
}

func (coordinator *ProjectIndexCoordinator) validateProjectRoot(
	projectRoot string,
) error {
	if coordinator == nil || coordinator.state == nil {
		return fmt.Errorf("code-index coordinator is unavailable")
	}
	if filepath.Clean(projectRoot) != coordinator.projectRoot {
		return fmt.Errorf(
			"code-index project root %q does not match coordinator root %q",
			projectRoot,
			coordinator.projectRoot,
		)
	}
	return nil
}

func (coordinator *ProjectIndexCoordinator) emit(
	result IndexCoordinationResult,
) {
	if coordinator == nil || !coordinator.bound {
		return
	}
	coordinator.state.resultMu.Lock()
	coordinator.state.lastResult = result
	coordinator.state.hasLastResult = true
	coordinator.state.resultMu.Unlock()
	event := logger.Info().
		Str("component", "code_index").
		Str("project_id", coordinator.projectID).
		Str("outcome", string(result.Outcome)).
		Dur("wait", result.WaitDuration).
		Str("source_fingerprint", result.SourceFingerprint).
		Int64("published_epoch", result.PublishedEpoch)
	if strings.TrimSpace(result.Reason) != "" {
		event = event.Str("reason", result.Reason)
	}
	event.Msg("code-index coordination")
}

func (coordinator *ProjectIndexCoordinator) latestResult() (
	IndexCoordinationResult,
	bool,
) {
	if coordinator == nil {
		return IndexCoordinationResult{}, false
	}
	coordinator.state.resultMu.RLock()
	defer coordinator.state.resultMu.RUnlock()
	return coordinator.state.lastResult, coordinator.state.hasLastResult
}

func boundedIndexWaitContext(
	parent context.Context,
) (context.Context, context.CancelFunc) {
	bound := defaultIndexFollowerWait
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline) - indexFollowerReadReserve
		if remaining < bound {
			bound = remaining
		}
	}
	if bound <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}
	return context.WithTimeout(parent, bound)
}

func (coordinator *ProjectIndexCoordinator) acquireProcessGate(
	ctx context.Context,
	policy indexAttemptPolicy,
) (bool, bool) {
	if policy == indexAttemptStartup {
		select {
		case <-coordinator.state.gate:
			return true, false
		default:
			return false, true
		}
	}
	select {
	case <-coordinator.state.gate:
		return true, false
	default:
	}
	select {
	case <-coordinator.state.gate:
		return true, true
	case <-ctx.Done():
		return false, true
	}
}

func (coordinator *ProjectIndexCoordinator) releaseProcessGate() {
	coordinator.state.gate <- struct{}{}
}

func (coordinator *ProjectIndexCoordinator) acquireLease(
	ctx context.Context,
	policy indexAttemptPolicy,
) (codeIndexLease, bool, bool, error) {
	if !coordinator.bound {
		return processOnlyCodeIndexLease{}, true, false, nil
	}
	lease, acquired, err := tryAcquireCodeIndexLease(
		coordinator.lockDir,
		coordinator.lockName,
	)
	if err != nil || acquired {
		return lease, acquired, false, err
	}
	if hook := coordinator.hooks.leaseBusy; hook != nil {
		hook()
	}
	if policy == indexAttemptStartup {
		return nil, false, true, nil
	}
	ticker := time.NewTicker(indexLeasePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, false, true, ctx.Err()
		case <-ticker.C:
			lease, acquired, err = tryAcquireCodeIndexLease(
				coordinator.lockDir,
				coordinator.lockName,
			)
			if err != nil || acquired {
				return lease, acquired, true, err
			}
		}
	}
}

type processOnlyCodeIndexLease struct{}

func (processOnlyCodeIndexLease) release() error { return nil }

func coordinationFailureReason(primary error, release error) string {
	joined := errors.Join(primary, release)
	if joined == nil {
		return ""
	}
	return joined.Error()
}
