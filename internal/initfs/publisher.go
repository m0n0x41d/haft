package initfs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type HostPublicationOutcomeKind string

const (
	HostPublicationApplied             HostPublicationOutcomeKind = "applied"
	HostPublicationAlreadyCurrent      HostPublicationOutcomeKind = "already_current"
	HostPublicationBusy                HostPublicationOutcomeKind = "busy"
	HostPublicationPreconditionChanged HostPublicationOutcomeKind = "precondition_changed"
	HostPublicationIncomplete          HostPublicationOutcomeKind = "core_applied_host_incomplete"
)

type HostPublicationFailureKind string

const (
	HostPublicationPreflightFailure HostPublicationFailureKind = "preflight_failed"
	HostPublicationJournalFailure   HostPublicationFailureKind = "journal_failed"
	HostPublicationStageFailure     HostPublicationFailureKind = "stage_failed"
	HostPublicationPathFailure      HostPublicationFailureKind = "path_effect_failed"
	HostPublicationRecoveryConflict HostPublicationFailureKind = "recovery_conflict"
	HostPublicationManifestFailure  HostPublicationFailureKind = "manifest_failed"
	HostPublicationCleanupFailure   HostPublicationFailureKind = "cleanup_failed"
	HostPublicationLockFailure      HostPublicationFailureKind = "lock_failed"
)

type HostPublicationFailure struct {
	kind   HostPublicationFailureKind
	path   string
	detail string
}

func (failure HostPublicationFailure) Kind() HostPublicationFailureKind {
	return failure.kind
}

func (failure HostPublicationFailure) Path() string {
	return failure.path
}

func (failure HostPublicationFailure) Detail() string {
	return failure.detail
}

type HostPathReceipt struct {
	path              string
	components        initplanning.ComponentSet
	step              initplanning.HostPublicationStepKind
	predecessorDigest string
	predecessorMode   fs.FileMode
	finalDigest       string
	finalMode         fs.FileMode
	recovered         bool
}

func (receipt HostPathReceipt) Path() string {
	return receipt.path
}

func (receipt HostPathReceipt) Component() initplanning.Component {
	values := receipt.components.Values()
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

func (receipt HostPathReceipt) Components() initplanning.ComponentSet {
	return receipt.components
}

func (receipt HostPathReceipt) Step() initplanning.HostPublicationStepKind {
	return receipt.step
}

func (receipt HostPathReceipt) PredecessorDigest() string {
	return receipt.predecessorDigest
}

func (receipt HostPathReceipt) PredecessorMode() fs.FileMode {
	return receipt.predecessorMode
}

func (receipt HostPathReceipt) FinalDigest() string {
	return receipt.finalDigest
}

func (receipt HostPathReceipt) FinalMode() fs.FileMode {
	return receipt.finalMode
}

func (receipt HostPathReceipt) Recovered() bool {
	return receipt.recovered
}

type HostPublicationOutcome struct {
	kind                   HostPublicationOutcomeKind
	receipts               []HostPathReceipt
	pendingPaths           []string
	preconditionChanges    []initplanning.PathPreconditionChange
	expectedManifestDigest string
	observedManifestDigest string
	desiredManifestDigest  string
	manifestPath           string
	journalPath            string
	journalDigest          string
	recoveryArgv           []string
	failure                HostPublicationFailure
	hasFailure             bool
}

func (outcome HostPublicationOutcome) Kind() HostPublicationOutcomeKind {
	return outcome.kind
}

func (outcome HostPublicationOutcome) Receipts() []HostPathReceipt {
	return slices.Clone(outcome.receipts)
}

func (outcome HostPublicationOutcome) PendingPaths() []string {
	return slices.Clone(outcome.pendingPaths)
}

func (outcome HostPublicationOutcome) PreconditionChanges() []initplanning.PathPreconditionChange {
	return slices.Clone(outcome.preconditionChanges)
}

func (outcome HostPublicationOutcome) ExpectedManifestDigest() string {
	return outcome.expectedManifestDigest
}

func (outcome HostPublicationOutcome) ObservedManifestDigest() string {
	return outcome.observedManifestDigest
}

func (outcome HostPublicationOutcome) DesiredManifestDigest() string {
	return outcome.desiredManifestDigest
}

func (outcome HostPublicationOutcome) ManifestPath() string {
	return outcome.manifestPath
}

func (outcome HostPublicationOutcome) JournalPath() string {
	return outcome.journalPath
}

func (outcome HostPublicationOutcome) JournalDigest() string {
	return outcome.journalDigest
}

func (outcome HostPublicationOutcome) RecoveryArgv() []string {
	return slices.Clone(outcome.recoveryArgv)
}

func (outcome HostPublicationOutcome) Failure() (HostPublicationFailure, bool) {
	return outcome.failure, outcome.hasFailure
}

func (outcome HostPublicationOutcome) PartialEffectBoundary() string {
	if outcome.kind != HostPublicationIncomplete {
		return ""
	}
	return string(HostPublicationIncomplete)
}

type hostPublisherHooks struct {
	beforeStage    func(initplanning.HostPublicationStep) error
	afterStages    func() error
	beforePath     func(initplanning.HostPublicationStep) error
	afterPath      func(initplanning.HostPublicationStep) error
	beforeManifest func() error
	afterManifest  func() error
}

func productionHostPublisherHooks() hostPublisherHooks {
	return hostPublisherHooks{
		beforeStage: func(initplanning.HostPublicationStep) error {
			return nil
		},
		afterStages: func() error {
			return nil
		},
		beforePath: func(initplanning.HostPublicationStep) error {
			return nil
		},
		afterPath: func(initplanning.HostPublicationStep) error {
			return nil
		},
		beforeManifest: func() error {
			return nil
		},
		afterManifest: func() error {
			return nil
		},
	}
}

type HostPublisher struct {
	observer FileObserver
	hooks    hostPublisherHooks
}

func NewHostPublisher(maxFileBytes int64) (HostPublisher, error) {
	observer, err := NewFileObserver(maxFileBytes)
	if err != nil {
		return HostPublisher{}, err
	}
	return HostPublisher{
		observer: observer,
		hooks:    productionHostPublisherHooks(),
	}, nil
}

func (publisher HostPublisher) Publish(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
) (HostPublicationOutcome, error) {
	observationPlan, err := validatePublicationRequest(batch, manifestStore)
	if err != nil {
		return HostPublicationOutcome{}, err
	}
	journalStore, err := newPublicationJournalStore(manifestStore)
	if err != nil {
		return HostPublicationOutcome{}, err
	}
	lease, acquired, err := manifestStore.TryAcquire()
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			PublicationJournal{},
			nil,
			newHostPublicationFailure(
				HostPublicationLockFailure,
				manifestStore.Path(),
				err,
			),
		), nil
	}
	if !acquired {
		return publicationBusyOutcome(batch, manifestStore, journalStore), nil
	}
	outcome, publishErr := publisher.publishWithLease(
		batch,
		observationPlan,
		manifestStore,
		journalStore,
		lease,
	)
	releaseErr := lease.Release()
	if publishErr != nil {
		return HostPublicationOutcome{}, errors.Join(publishErr, releaseErr)
	}
	if releaseErr != nil {
		return outcome.withFailure(
			batch,
			newHostPublicationFailure(
				HostPublicationLockFailure,
				manifestStore.Path(),
				releaseErr,
			),
		), nil
	}
	return outcome, nil
}

func validatePublicationRequest(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
) (initplanning.InstallationObservationPlan, error) {
	if err := manifestStore.valid(); err != nil {
		return initplanning.InstallationObservationPlan{}, err
	}
	plan, err := initplanning.BuildHostPublicationObservationPlan(batch)
	if err != nil {
		return initplanning.InstallationObservationPlan{}, err
	}
	manifest := batch.Manifest()
	if manifest.Digest() == "" ||
		manifest.ProjectRoot() != batch.ProjectRoot() ||
		manifest.ProjectID() != batch.ProjectID() ||
		manifest.Host() != batch.Host() ||
		manifest.AdapterEdition() != batch.Edition() ||
		manifest.Scope() != batch.Scope() {
		return initplanning.InstallationObservationPlan{}, fmt.Errorf(
			"publication batch manifest binding is invalid",
		)
	}
	for _, step := range batch.Steps() {
		if step.Path() == manifestStore.Path() ||
			step.Path() == manifestStore.JournalPath() {
			return initplanning.InstallationObservationPlan{}, fmt.Errorf(
				"publication path collides with its ownership carriers",
			)
		}
	}
	return plan, nil
}

func (publisher HostPublisher) publishWithLease(
	batch initplanning.HostPublicationBatch,
	observationPlan initplanning.InstallationObservationPlan,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
) (HostPublicationOutcome, error) {
	journalRead, err := journalStore.read()
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			PublicationJournal{},
			nil,
			newHostPublicationFailure(
				HostPublicationJournalFailure,
				journalStore.path,
				err,
			),
		), nil
	}
	if journalRead.kind == publicationJournalPresent {
		return publisher.resumePublication(
			batch,
			manifestStore,
			journalStore,
			lease,
			journalRead.journal,
		)
	}
	return publisher.startPublication(
		batch,
		observationPlan,
		manifestStore,
		journalStore,
		lease,
	)
}

func (publisher HostPublisher) startPublication(
	batch initplanning.HostPublicationBatch,
	observationPlan initplanning.InstallationObservationPlan,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
) (HostPublicationOutcome, error) {
	manifestRead, err := lease.Read()
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			PublicationJournal{},
			nil,
			newHostPublicationFailure(
				HostPublicationPreflightFailure,
				manifestStore.Path(),
				err,
			),
		), nil
	}
	if !manifestPredecessorMatches(batch.ManifestPredecessor(), manifestRead) {
		return manifestPreconditionChangedOutcome(
			batch,
			manifestStore,
			journalStore,
			manifestRead,
		), nil
	}
	observations, err := publisher.observer.Observe(observationPlan)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			PublicationJournal{},
			nil,
			newHostPublicationFailure(
				HostPublicationPreflightFailure,
				"<host-publication>",
				err,
			),
		), nil
	}
	admission, err := initplanning.ValidateHostPublicationPreconditions(
		batch,
		observations,
	)
	if err != nil {
		return HostPublicationOutcome{}, err
	}
	if admission.Kind() == initplanning.HostPublicationPreconditionsChanged {
		return pathPreconditionChangedOutcome(
			batch,
			manifestStore,
			journalStore,
			admission.Changes(),
		), nil
	}
	journal, err := NewPublicationJournal(batch, manifestStore.Path())
	if err != nil {
		return HostPublicationOutcome{}, err
	}
	if err := journalStore.create(journal); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			nil,
			newHostPublicationFailure(
				HostPublicationJournalFailure,
				journalStore.path,
				err,
			),
		), nil
	}
	return publisher.continuePublication(
		batch,
		manifestStore,
		journalStore,
		lease,
		journal,
		false,
	)
}

func (publisher HostPublisher) resumePublication(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
	journal PublicationJournal,
) (HostPublicationOutcome, error) {
	if err := journal.ValidateAgainst(batch, manifestStore.Path()); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			nil,
			newHostPublicationFailure(
				HostPublicationRecoveryConflict,
				journalStore.path,
				err,
			),
		), nil
	}
	return publisher.continuePublication(
		batch,
		manifestStore,
		journalStore,
		lease,
		journal,
		true,
	)
}

func (publisher HostPublisher) continuePublication(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
	journal PublicationJournal,
	recovering bool,
) (HostPublicationOutcome, error) {
	receipts := make(map[string]HostPathReceipt)
	reconciled, err := publisher.reconcileJournalState(
		batch,
		manifestStore,
		journalStore,
		lease,
		journal,
		receipts,
	)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationRecoveryConflict,
				recoveryFailurePath(err, journalStore.path),
				err,
			),
		), nil
	}
	journal = reconciled
	if err := publisher.cleanupCompletedPublicationStages(batch, journal); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				recoveryFailurePath(err, "<host-stage>"),
				err,
			),
		), nil
	}
	if journal.Phase() == PublicationJournalManifest {
		return publisher.finishManifestPublication(
			batch,
			manifestStore,
			journalStore,
			lease,
			journal,
			receipts,
			recovering,
		)
	}
	stages, err := publisher.stagePendingOutputs(batch, journal)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationStageFailure,
				recoveryFailurePath(err, "<host-stage>"),
				err,
			),
		), nil
	}
	cleanupStages := true
	defer func() {
		if cleanupStages {
			_ = cleanupStagedCarriers(stages)
		}
	}()
	if err := publisher.hooks.afterStages(); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationStageFailure,
				"<host-stage>",
				err,
			),
		), nil
	}
	reconciled, err = publisher.reconcileJournalState(
		batch,
		manifestStore,
		journalStore,
		lease,
		journal,
		receipts,
	)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationRecoveryConflict,
				recoveryFailurePath(err, "<host-publication>"),
				err,
			),
		), nil
	}
	journal = reconciled
	if err := publisher.cleanupCompletedPublicationStages(batch, journal); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				recoveryFailurePath(err, "<host-stage>"),
				err,
			),
		), nil
	}
	journal, err = publisher.applyMutationSteps(
		batch,
		journalStore,
		journal,
		stages,
		receipts,
	)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationPathFailure,
				recoveryFailurePath(err, "<host-publication>"),
				err,
			),
		), nil
	}
	if err := publisher.verifyAllTerminal(batch, receipts); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationRecoveryConflict,
				recoveryFailurePath(err, "<host-publication>"),
				err,
			),
		), nil
	}
	if err := cleanupStagedCarriers(stages); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				recoveryFailurePath(err, "<host-stage>"),
				err,
			),
		), nil
	}
	cleanupStages = false
	return publisher.finishManifestPublication(
		batch,
		manifestStore,
		journalStore,
		lease,
		journal,
		receipts,
		recovering,
	)
}

func (publisher HostPublisher) reconcileJournalState(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
	journal PublicationJournal,
	receipts map[string]HostPathReceipt,
) (PublicationJournal, error) {
	if err := journal.ValidateAgainst(batch, manifestStore.Path()); err != nil {
		return PublicationJournal{}, err
	}
	manifestRead, err := lease.Read()
	if err != nil {
		return PublicationJournal{}, err
	}
	if journal.Phase() == PublicationJournalManifest {
		desired := manifestRead.Kind() == ManifestReadPresent &&
			manifestRead.Manifest().Digest() == batch.Manifest().Digest()
		predecessor := manifestPredecessorMatches(
			batch.ManifestPredecessor(),
			manifestRead,
		)
		if !desired && !predecessor {
			return PublicationJournal{}, fmt.Errorf(
				"manifest path %s changed during publication recovery",
				manifestStore.Path(),
			)
		}
		if err := publisher.verifyAllTerminal(batch, receipts); err != nil {
			return PublicationJournal{}, err
		}
		return journal, nil
	}
	if !manifestPredecessorMatches(batch.ManifestPredecessor(), manifestRead) {
		return PublicationJournal{}, fmt.Errorf(
			"manifest path %s changed before publication completed",
			manifestStore.Path(),
		)
	}
	if journal.Phase() == PublicationJournalApplying {
		step, exists := publicationStepByPath(batch, journal.ActivePath())
		if !exists {
			return PublicationJournal{}, fmt.Errorf("active publication path is absent from its batch")
		}
		observation, err := publisher.observeStep(batch, step)
		if err != nil {
			return PublicationJournal{}, err
		}
		if stepTerminalMatches(step, observation) {
			completed, err := CompletePublicationStep(journal, batch, step.Path())
			if err != nil {
				return PublicationJournal{}, err
			}
			if err := journalStore.replace(completed, journal.Digest()); err != nil {
				return PublicationJournal{}, err
			}
			journal = completed
			receipts[step.Path()] = receiptForStep(step, true)
		}
		if journal.Phase() == PublicationJournalApplying &&
			!step.Expectation().MatchesObservation(observation) {
			return PublicationJournal{}, fmt.Errorf(
				"path %s matches neither active predecessor nor desired result",
				step.Path(),
			)
		}
	}
	completed := stringSet(journal.CompletedPaths())
	for _, step := range batch.Steps() {
		_, isCompleted := completed[step.Path()]
		if isCompleted {
			observation, err := publisher.observeStep(batch, step)
			if err != nil {
				return PublicationJournal{}, err
			}
			if !stepTerminalMatches(step, observation) {
				return PublicationJournal{}, fmt.Errorf(
					"completed publication path %s no longer matches its result",
					step.Path(),
				)
			}
			receipts[step.Path()] = receiptForStep(step, true)
			continue
		}
		if journal.ActivePath() == step.Path() {
			continue
		}
		observation, err := publisher.observeStep(batch, step)
		if err != nil {
			return PublicationJournal{}, err
		}
		if !step.Expectation().MatchesObservation(observation) {
			return PublicationJournal{}, fmt.Errorf(
				"pending publication path %s changed from its predecessor",
				step.Path(),
			)
		}
	}
	return journal, nil
}

func (publisher HostPublisher) stagePendingOutputs(
	batch initplanning.HostPublicationBatch,
	journal PublicationJournal,
) (map[string]stagedCarrier, error) {
	completed := stringSet(journal.CompletedPaths())
	stages := make(map[string]stagedCarrier)
	for _, step := range batch.Steps() {
		if step.Kind() != initplanning.PublicationCreate &&
			step.Kind() != initplanning.PublicationReplace {
			continue
		}
		if _, exists := completed[step.Path()]; exists {
			continue
		}
		if err := publisher.hooks.beforeStage(step); err != nil {
			return stages, pathEffectError{path: step.Path(), cause: err}
		}
		stage, err := publisher.stageOutput(batch, step)
		if err != nil {
			return stages, pathEffectError{path: step.Path(), cause: err}
		}
		stages[step.Path()] = stage
	}
	return stages, nil
}

func (publisher HostPublisher) applyMutationSteps(
	batch initplanning.HostPublicationBatch,
	journalStore publicationJournalStore,
	journal PublicationJournal,
	stages map[string]stagedCarrier,
	receipts map[string]HostPathReceipt,
) (PublicationJournal, error) {
	completed := stringSet(journal.CompletedPaths())
	for _, step := range batch.Steps() {
		if !mutatingPublicationStep(step.Kind()) {
			if _, exists := receipts[step.Path()]; !exists {
				receipts[step.Path()] = receiptForStep(step, false)
			}
			continue
		}
		if _, exists := completed[step.Path()]; exists {
			continue
		}
		var err error
		if journal.Phase() != PublicationJournalApplying {
			next, beginErr := BeginPublicationStep(journal, batch, step.Path())
			if beginErr != nil {
				return journal, pathEffectError{path: step.Path(), cause: beginErr}
			}
			if beginErr := journalStore.replace(next, journal.Digest()); beginErr != nil {
				return journal, pathEffectError{path: step.Path(), cause: beginErr}
			}
			journal = next
		}
		if journal.ActivePath() != step.Path() {
			return journal, pathEffectError{
				path:  step.Path(),
				cause: fmt.Errorf("another publication path is active"),
			}
		}
		if err = publisher.hooks.beforePath(step); err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		observation, err := publisher.observeStep(batch, step)
		if err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		recovered := stepTerminalMatches(step, observation)
		if !recovered && !step.Expectation().MatchesObservation(observation) {
			return journal, pathEffectError{
				path:  step.Path(),
				cause: fmt.Errorf("path changed after publication journal activation"),
			}
		}
		if !recovered {
			err = publisher.applyPathEffect(step, stages)
			if err != nil {
				return journal, pathEffectError{path: step.Path(), cause: err}
			}
		}
		observation, err = publisher.observeStep(batch, step)
		if err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		if !stepTerminalMatches(step, observation) {
			return journal, pathEffectError{
				path:  step.Path(),
				cause: fmt.Errorf("published path failed exact terminal reread"),
			}
		}
		if err := publisher.hooks.afterPath(step); err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		next, err := CompletePublicationStep(journal, batch, step.Path())
		if err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		if err := journalStore.replace(next, journal.Digest()); err != nil {
			return journal, pathEffectError{path: step.Path(), cause: err}
		}
		journal = next
		completed[step.Path()] = struct{}{}
		receipts[step.Path()] = receiptForStep(step, recovered)
		if stage, exists := stages[step.Path()]; exists {
			if err := cleanupStagedCarrier(stage); err != nil {
				return journal, pathEffectError{path: step.Path(), cause: err}
			}
			delete(stages, step.Path())
		}
	}
	return journal, nil
}

func (publisher HostPublisher) finishManifestPublication(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	lease *ManifestLease,
	journal PublicationJournal,
	receipts map[string]HostPathReceipt,
	recovering bool,
) (HostPublicationOutcome, error) {
	if err := publisher.cleanupCompletedPublicationStages(batch, journal); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				recoveryFailurePath(err, "<host-stage>"),
				err,
			),
		), nil
	}
	if err := publisher.verifyAllTerminal(batch, receipts); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationRecoveryConflict,
				recoveryFailurePath(err, "<host-publication>"),
				err,
			),
		), nil
	}
	if journal.Phase() != PublicationJournalManifest {
		next, err := BeginManifestPublication(journal, batch)
		if err != nil {
			return HostPublicationOutcome{}, err
		}
		if err := journalStore.replace(next, journal.Digest()); err != nil {
			return incompletePublicationOutcome(
				batch,
				manifestStore,
				journalStore,
				journal,
				receiptValues(receipts),
				newHostPublicationFailure(
					HostPublicationJournalFailure,
					journalStore.path,
					err,
				),
			), nil
		}
		journal = next
	}
	if err := publisher.hooks.beforeManifest(); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationManifestFailure,
				manifestStore.Path(),
				err,
			),
		), nil
	}
	precondition, err := manifestPersistPrecondition(batch.ManifestPredecessor())
	if err != nil {
		return HostPublicationOutcome{}, err
	}
	persisted, err := lease.Persist(batch.Manifest(), precondition)
	if err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationManifestFailure,
				manifestStore.Path(),
				err,
			),
		), nil
	}
	if persisted.Kind() != ManifestPersisted &&
		persisted.Kind() != ManifestAlreadyCurrent {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationManifestFailure,
				manifestStore.Path(),
				fmt.Errorf("manifest precondition changed at publication"),
			),
		), nil
	}
	if err := publisher.hooks.afterManifest(); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				journalStore.path,
				err,
			),
		), nil
	}
	if err := journalStore.remove(journal.Digest()); err != nil {
		return incompletePublicationOutcome(
			batch,
			manifestStore,
			journalStore,
			journal,
			receiptValues(receipts),
			newHostPublicationFailure(
				HostPublicationCleanupFailure,
				journalStore.path,
				err,
			),
		), nil
	}
	kind := HostPublicationApplied
	noMutation := len(mutationStepSetForPublisher(batch)) == 0
	if noMutation &&
		persisted.Kind() == ManifestAlreadyCurrent &&
		!recovering {
		kind = HostPublicationAlreadyCurrent
	}
	return successfulPublicationOutcome(
		kind,
		batch,
		manifestStore,
		journalStore,
		receiptValues(receipts),
	), nil
}

func (publisher HostPublisher) verifyAllTerminal(
	batch initplanning.HostPublicationBatch,
	receipts map[string]HostPathReceipt,
) error {
	for _, step := range batch.Steps() {
		observation, err := publisher.observeStep(batch, step)
		if err != nil {
			return pathEffectError{path: step.Path(), cause: err}
		}
		if !stepTerminalMatches(step, observation) {
			return pathEffectError{
				path:  step.Path(),
				cause: fmt.Errorf("path does not match the desired terminal projection"),
			}
		}
		if _, exists := receipts[step.Path()]; !exists {
			receipts[step.Path()] = receiptForStep(step, true)
		}
	}
	return nil
}

func (publisher HostPublisher) observeStep(
	batch initplanning.HostPublicationBatch,
	step initplanning.HostPublicationStep,
) (initplanning.PathObservation, error) {
	root, err := containingManagedRoot(step.Path(), batch.TargetRoots())
	if err != nil {
		return initplanning.PathObservation{}, err
	}
	info, missing, err := lstatWithoutManagedRootSymlinks(root, step.Path())
	if err != nil {
		return initplanning.PathObservation{}, err
	}
	if missing {
		return initplanning.ObserveMissingPathForComponents(
			step.Path(),
			step.Components(),
		)
	}
	digest, mode, err := publisher.observer.digestStableRegularFile(step.Path(), info)
	if err != nil {
		return initplanning.PathObservation{}, err
	}
	return initplanning.ObservePresentPathForComponents(
		step.Path(),
		step.Components(),
		digest,
		mode.Perm(),
	)
}

type stagedCarrier struct {
	root      string
	path      string
	output    initplanning.RenderedOutput
	published bool
}

func (publisher HostPublisher) stageOutput(
	batch initplanning.HostPublicationBatch,
	step initplanning.HostPublicationStep,
) (stagedCarrier, error) {
	output, exists := step.Output()
	if !exists {
		return stagedCarrier{}, fmt.Errorf("publication output is absent")
	}
	root, err := containingManagedRoot(step.Path(), batch.TargetRoots())
	if err != nil {
		return stagedCarrier{}, err
	}
	if err := ensureManagedRoot(root); err != nil {
		return stagedCarrier{}, err
	}
	parent := filepath.Dir(step.Path())
	if err := ensureDirectoryTreeWithoutSymlinks(root, parent); err != nil {
		return stagedCarrier{}, err
	}
	stagePath := filepath.Join(
		parent,
		publicationStageName(batch.Digest(), step.Path(), output.Digest()),
	)
	info, missing, err := lstatWithoutManagedRootSymlinks(root, stagePath)
	if err != nil {
		return stagedCarrier{}, err
	}
	if !missing {
		digest, mode, err := publisher.observer.digestStableRegularFile(stagePath, info)
		if err != nil {
			return stagedCarrier{}, err
		}
		if digest != output.Digest() || mode.Perm() != output.Mode().Perm() {
			return stagedCarrier{}, fmt.Errorf("publication stage exists with different bytes or mode")
		}
		return stagedCarrier{
			root:   root,
			path:   stagePath,
			output: output,
		}, nil
	}
	file, err := os.OpenFile(
		stagePath,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return stagedCarrier{}, fmt.Errorf("create publication stage: %w", err)
	}
	writeErr := writeStagedOutput(file, output)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(stagePath)
		return stagedCarrier{}, err
	}
	if err := syncDirectory(parent); err != nil {
		return stagedCarrier{}, err
	}
	info, missing, err = lstatWithoutManagedRootSymlinks(root, stagePath)
	if err != nil {
		return stagedCarrier{}, err
	}
	if missing {
		return stagedCarrier{}, fmt.Errorf("publication stage disappeared")
	}
	digest, mode, err := publisher.observer.digestStableRegularFile(stagePath, info)
	if err != nil {
		return stagedCarrier{}, err
	}
	if digest != output.Digest() || mode.Perm() != output.Mode().Perm() {
		return stagedCarrier{}, fmt.Errorf("publication stage failed exact reread")
	}
	return stagedCarrier{
		root:   root,
		path:   stagePath,
		output: output,
	}, nil
}

func writeStagedOutput(
	file *os.File,
	output initplanning.RenderedOutput,
) error {
	content := output.Content()
	written, err := file.Write(content)
	if err != nil {
		return fmt.Errorf("write publication stage: %w", err)
	}
	if written != len(content) {
		return fmt.Errorf("write publication stage: %w", io.ErrShortWrite)
	}
	if err := file.Chmod(output.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod publication stage: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync publication stage: %w", err)
	}
	return nil
}

func (publisher HostPublisher) applyPathEffect(
	step initplanning.HostPublicationStep,
	stages map[string]stagedCarrier,
) error {
	switch step.Kind() {
	case initplanning.PublicationCreate:
		stage, exists := stages[step.Path()]
		if !exists {
			return fmt.Errorf("create publication stage is absent")
		}
		if err := os.Link(stage.path, step.Path()); err != nil {
			return fmt.Errorf("publish new host carrier without replacement: %w", err)
		}
		if err := syncDirectory(filepath.Dir(step.Path())); err != nil {
			return err
		}
		return nil
	case initplanning.PublicationReplace:
		stage, exists := stages[step.Path()]
		if !exists {
			return fmt.Errorf("replacement publication stage is absent")
		}
		if err := os.Rename(stage.path, step.Path()); err != nil {
			return fmt.Errorf("replace owned host carrier atomically: %w", err)
		}
		stage.published = true
		stages[step.Path()] = stage
		if err := syncDirectory(filepath.Dir(step.Path())); err != nil {
			return err
		}
		return nil
	case initplanning.PublicationRemove:
		if err := os.Remove(step.Path()); err != nil {
			return fmt.Errorf("remove exact owned host carrier: %w", err)
		}
		if err := syncDirectory(filepath.Dir(step.Path())); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("publication step is not a filesystem mutation")
	}
}

func (publisher HostPublisher) cleanupCompletedPublicationStages(
	batch initplanning.HostPublicationBatch,
	journal PublicationJournal,
) error {
	completed := stringSet(journal.CompletedPaths())
	if journal.Phase() == PublicationJournalManifest {
		completed = mutationStepSetForPublisher(batch)
	}
	for _, step := range batch.Steps() {
		if step.Kind() != initplanning.PublicationCreate &&
			step.Kind() != initplanning.PublicationReplace {
			continue
		}
		if _, exists := completed[step.Path()]; !exists {
			continue
		}
		output, exists := step.Output()
		if !exists {
			return pathEffectError{
				path:  step.Path(),
				cause: fmt.Errorf("completed publication output is absent"),
			}
		}
		root, err := containingManagedRoot(step.Path(), batch.TargetRoots())
		if err != nil {
			return pathEffectError{path: step.Path(), cause: err}
		}
		stage := stagedCarrier{
			root: root,
			path: filepath.Join(
				filepath.Dir(step.Path()),
				publicationStageName(batch.Digest(), step.Path(), output.Digest()),
			),
			output: output,
		}
		if err := cleanupStagedCarrier(stage); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStagedCarriers(stages map[string]stagedCarrier) error {
	paths := make([]string, 0, len(stages))
	for path := range stages {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var result error
	for _, path := range paths {
		err := cleanupStagedCarrier(stages[path])
		result = errors.Join(result, err)
	}
	return result
}

func cleanupStagedCarrier(stage stagedCarrier) error {
	if stage.published {
		return nil
	}
	info, err := os.Lstat(stage.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return pathEffectError{path: stage.path, cause: err}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return pathEffectError{
			path:  stage.path,
			cause: fmt.Errorf("publication stage is no longer a regular file"),
		}
	}
	observer, err := NewFileObserver(int64(len(stage.output.Content())) + 1)
	if err != nil {
		return pathEffectError{path: stage.path, cause: err}
	}
	digest, mode, err := observer.digestStableRegularFile(stage.path, info)
	if err != nil {
		return pathEffectError{path: stage.path, cause: err}
	}
	if digest != stage.output.Digest() ||
		mode.Perm() != stage.output.Mode().Perm() {
		return pathEffectError{
			path:  stage.path,
			cause: fmt.Errorf("publication stage changed; preserve it"),
		}
	}
	if err := os.Remove(stage.path); err != nil {
		return pathEffectError{path: stage.path, cause: err}
	}
	if err := syncDirectory(filepath.Dir(stage.path)); err != nil {
		return pathEffectError{path: stage.path, cause: err}
	}
	return nil
}

func ensureManagedRoot(root string) error {
	if root == "" ||
		!filepath.IsAbs(root) ||
		filepath.Clean(root) != root ||
		filepath.Dir(root) == root {
		return observationFailure(
			ObservationUnsafePath,
			root,
			fmt.Errorf("managed root is invalid or too broad"),
		)
	}
	missing := make([]string, 0)
	current := root
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return observationFailure(
					ObservationUnsafePath,
					current,
					fmt.Errorf("managed-root ancestor is not a real directory"),
				)
			}
			break
		}
		if !os.IsNotExist(err) {
			return observationFailure(ObservationReadFailure, current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return observationFailure(
				ObservationUnsafePath,
				root,
				fmt.Errorf("managed root has no existing directory ancestor"),
			)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
	for index := len(missing) - 1; index >= 0; index-- {
		current = filepath.Join(current, missing[index])
		if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
			return observationFailure(ObservationReadFailure, current, err)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return observationFailure(ObservationReadFailure, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return observationFailure(
				ObservationUnsafePath,
				current,
				fmt.Errorf("created managed-root segment is not a real directory"),
			)
		}
	}
	return nil
}

func publicationStageName(
	batchDigest string,
	path string,
	outputDigest string,
) string {
	identity := strings.Join([]string{batchDigest, path, outputDigest}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf(".haft-host-stage-%x", digest)
}

func stepTerminalMatches(
	step initplanning.HostPublicationStep,
	observation initplanning.PathObservation,
) bool {
	if step.Kind() == initplanning.PublicationRemove {
		return observation.Kind() == initplanning.PathObservedMissing
	}
	output, exists := step.Output()
	if !exists {
		return false
	}
	return observation.Kind() == initplanning.PathObservedPresent &&
		observation.Digest() == output.Digest() &&
		observation.Mode() == output.Mode().Perm()
}

func receiptForStep(
	step initplanning.HostPublicationStep,
	recovered bool,
) HostPathReceipt {
	expectation := step.Expectation()
	receipt := HostPathReceipt{
		path:              step.Path(),
		components:        step.Components(),
		step:              step.Kind(),
		predecessorDigest: expectation.Digest(),
		predecessorMode:   expectation.Mode().Perm(),
		recovered:         recovered,
	}
	output, exists := step.Output()
	if exists {
		receipt.finalDigest = output.Digest()
		receipt.finalMode = output.Mode().Perm()
	}
	return receipt
}

func manifestPredecessorMatches(
	predecessor initplanning.ManifestPredecessor,
	read ManifestReadResult,
) bool {
	switch predecessor.Kind() {
	case initplanning.ManifestPredecessorMissing:
		return read.Kind() == ManifestReadMissing
	case initplanning.ManifestPredecessorExact:
		return read.Kind() == ManifestReadPresent &&
			read.Manifest().Digest() == predecessor.Digest()
	default:
		return false
	}
}

func manifestPersistPrecondition(
	predecessor initplanning.ManifestPredecessor,
) (ManifestPrecondition, error) {
	switch predecessor.Kind() {
	case initplanning.ManifestPredecessorMissing:
		return ExpectManifestMissing(), nil
	case initplanning.ManifestPredecessorExact:
		return ExpectManifestWithDigest(predecessor.Digest())
	default:
		return ManifestPrecondition{}, fmt.Errorf("manifest predecessor is invalid")
	}
}

func mutatingPublicationStep(
	kind initplanning.HostPublicationStepKind,
) bool {
	return kind == initplanning.PublicationCreate ||
		kind == initplanning.PublicationReplace ||
		kind == initplanning.PublicationRemove
}

func mutationStepSetForPublisher(
	batch initplanning.HostPublicationBatch,
) map[string]struct{} {
	result := make(map[string]struct{})
	for _, step := range batch.Steps() {
		if mutatingPublicationStep(step.Kind()) {
			result[step.Path()] = struct{}{}
		}
	}
	return result
}

func publicationStepByPath(
	batch initplanning.HostPublicationBatch,
	path string,
) (initplanning.HostPublicationStep, bool) {
	for _, step := range batch.Steps() {
		if step.Path() == path {
			return step, true
		}
	}
	return initplanning.HostPublicationStep{}, false
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func receiptValues(
	byPath map[string]HostPathReceipt,
) []HostPathReceipt {
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]HostPathReceipt, len(paths))
	for index, path := range paths {
		result[index] = byPath[path]
	}
	return result
}

func pendingPublicationPaths(
	batch initplanning.HostPublicationBatch,
	receipts []HostPathReceipt,
) []string {
	completed := make(map[string]struct{}, len(receipts))
	for _, receipt := range receipts {
		completed[receipt.path] = struct{}{}
	}
	result := make([]string, 0)
	for _, step := range batch.Steps() {
		if _, exists := completed[step.Path()]; !exists {
			result = append(result, step.Path())
		}
	}
	sort.Strings(result)
	return result
}

func publicationBaseOutcome(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
) HostPublicationOutcome {
	predecessor := batch.ManifestPredecessor()
	return HostPublicationOutcome{
		expectedManifestDigest: predecessor.Digest(),
		desiredManifestDigest:  batch.Manifest().Digest(),
		manifestPath:           manifestStore.Path(),
		journalPath:            journalStore.path,
		recoveryArgv:           batch.Recovery().Argv(),
	}
}

func successfulPublicationOutcome(
	kind HostPublicationOutcomeKind,
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	receipts []HostPathReceipt,
) HostPublicationOutcome {
	outcome := publicationBaseOutcome(batch, manifestStore, journalStore)
	outcome.kind = kind
	outcome.receipts = slices.Clone(receipts)
	outcome.observedManifestDigest = batch.Manifest().Digest()
	return outcome
}

func publicationBusyOutcome(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
) HostPublicationOutcome {
	outcome := publicationBaseOutcome(batch, manifestStore, journalStore)
	outcome.kind = HostPublicationBusy
	outcome.pendingPaths = pendingPublicationPaths(batch, nil)
	return outcome
}

func manifestPreconditionChangedOutcome(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	read ManifestReadResult,
) HostPublicationOutcome {
	outcome := publicationBaseOutcome(batch, manifestStore, journalStore)
	outcome.kind = HostPublicationPreconditionChanged
	outcome.observedManifestDigest = currentManifestDigest(read)
	outcome.pendingPaths = pendingPublicationPaths(batch, nil)
	return outcome
}

func pathPreconditionChangedOutcome(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	changes []initplanning.PathPreconditionChange,
) HostPublicationOutcome {
	outcome := publicationBaseOutcome(batch, manifestStore, journalStore)
	outcome.kind = HostPublicationPreconditionChanged
	outcome.preconditionChanges = slices.Clone(changes)
	outcome.pendingPaths = pendingPublicationPaths(batch, nil)
	return outcome
}

func incompletePublicationOutcome(
	batch initplanning.HostPublicationBatch,
	manifestStore ManifestStore,
	journalStore publicationJournalStore,
	journal PublicationJournal,
	receipts []HostPathReceipt,
	failure HostPublicationFailure,
) HostPublicationOutcome {
	outcome := publicationBaseOutcome(batch, manifestStore, journalStore)
	outcome.kind = HostPublicationIncomplete
	outcome.receipts = slices.Clone(receipts)
	outcome.pendingPaths = pendingPublicationPaths(batch, receipts)
	outcome.failure = failure
	outcome.hasFailure = true
	if journal.Digest() != "" {
		outcome.journalDigest = journal.Digest()
	}
	return outcome
}

func (outcome HostPublicationOutcome) withFailure(
	batch initplanning.HostPublicationBatch,
	failure HostPublicationFailure,
) HostPublicationOutcome {
	outcome.kind = HostPublicationIncomplete
	outcome.failure = failure
	outcome.hasFailure = true
	outcome.pendingPaths = pendingPublicationPaths(batch, outcome.receipts)
	return outcome
}

func newHostPublicationFailure(
	kind HostPublicationFailureKind,
	path string,
	err error,
) HostPublicationFailure {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return HostPublicationFailure{
		kind:   kind,
		path:   path,
		detail: detail,
	}
}

type pathEffectError struct {
	path  string
	cause error
}

func (failure pathEffectError) Error() string {
	return fmt.Sprintf("%s: %v", failure.path, failure.cause)
}

func (failure pathEffectError) Unwrap() error {
	return failure.cause
}

func recoveryFailurePath(err error, fallback string) string {
	var failure pathEffectError
	if errors.As(err, &failure) {
		return failure.path
	}
	return fallback
}
