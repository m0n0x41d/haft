package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectmemory/kindclassificationengine"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionreadset"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type currentGenesisFrameDependencies struct {
	stages           *projecttypeenvstage.Store
	heads            *projecttypeenvheadstore.Store
	installedRuntime projecttypeenvruntime.InstalledRuntimeRegistryInput
	observedAt       time.Time
}

// currentGenesisFrame owns every non-serializable same-transaction
// capability needed by the original effect branch. It is never accepted from
// a caller and must not escape the transaction that minted it.
type currentGenesisFrame struct {
	readyStage     projecttypeenvstage.SelectionReadyStage
	currentStage   projecttypeenvstagerevalidation.CurrentSelectionStage
	headReadSet    projecttypeenvselectionreadset.GenesisHeadSelectionReadSet
	currentGraph   typedmemorystore.CurrentProjectGraphObservation
	currentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis
	targetRuntime  projecttypeenvruntime.ExactTargetRuntimeRegistry
	currentHead    projecttypeenvstagerevalidation.CurrentProjectTypeEnvHeadObservation
	projectRoot    projectprofile.ProjectRootV1
}

// loadCurrentGenesisFrameTx is reachable only after ProbeReplayTx returned
// ReplayAbsent. All mutable basis reads and capability minting stay inside the
// caller-owned BEGIN IMMEDIATE transaction.
func loadCurrentGenesisFrameTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	dependencies currentGenesisFrameDependencies,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) (currentGenesisFrameResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf(
			"load current Genesis frame: context is required",
		)
	}
	if transaction == nil {
		return nil, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	if dependencies.stages == nil || dependencies.heads == nil {
		return nil, fmt.Errorf(
			"load current Genesis frame: Stage and head stores are required",
		)
	}
	if err := request.Verify(); err != nil {
		return nil, fmt.Errorf(
			"load current Genesis frame request: %w",
			err,
		)
	}
	ready, err := dependencies.stages.LoadSelectionReadyTx(
		ctx,
		transaction,
		request.Target().Stage(),
	)
	if err != nil {
		if reason, rejected := selectionReadyLoadRejection(err); rejected {
			return rejectCurrentGenesisFrame(reason), nil
		}
		return nil, fmt.Errorf(
			"reload selection-ready Stage: %w",
			err,
		)
	}
	runtimeBasis, err := projecttypeenvstore.GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		ready.Stage().RuntimeBasis(),
	)
	if err != nil {
		if reason, rejected := selectionReadyLoadRejection(err); rejected {
			return rejectCurrentGenesisFrame(reason), nil
		}
		return nil, fmt.Errorf(
			"reload current runtime basis X: %w",
			err,
		)
	}
	runtimeResolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtimeBasis,
			Installed:    dependencies.installedRuntime,
		},
	)
	var targetRuntime projecttypeenvruntime.ExactTargetRuntimeRegistry
	switch current := runtimeResolution.(type) {
	case projecttypeenvruntime.Invalid:
		return rejectCurrentGenesisFrame(
			projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(),
		), nil
	case projecttypeenvruntime.Unavailable,
		projecttypeenvruntime.Drifted:
		return rejectCurrentGenesisFrame(
			projecttypeenvselectioneffect.NotSelectedStageDrift(),
		), nil
	case projecttypeenvruntime.Matched:
		targetRuntime, err = exactMatchedTargetRuntime(current)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf(
			"target runtime X returned an invalid result variant: %T",
			runtimeResolution,
		)
	}
	currentGraph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		request.Project(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"reload current typed-memory graph: %w",
			err,
		)
	}
	classificationEngine, err := kindclassificationengine.ForExactTargetRuntime(
		targetRuntime,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct target-C classification engine: %w",
			err,
		)
	}
	referenceKindFacts, err := typedmemorystore.LoadExactTargetReferenceKindFactViewTx(
		ctx,
		transaction,
		request.Project(),
		currentGraph.GraphSnapshotBasis(),
		ready.ExecutableSnapshot().Environment(),
		targetRuntime,
		classificationEngine,
	)
	if err != nil {
		return nil, fmt.Errorf("reload target-C reference-kind facts: %w", err)
	}
	projectRoot, err := loadBoundProjectRootTx(
		ctx,
		transaction,
		request.Project().String(),
	)
	if err != nil {
		return nil, err
	}
	currentProfile, err := loadCurrentProjectProfileBasisTx(
		ctx,
		transaction,
		projectRoot,
	)
	if err != nil {
		return nil, err
	}
	currentHead, err := dependencies.heads.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		request.Project(),
	)
	if err != nil {
		if errors.Is(err, projecttypeenvheadstore.ErrStoredHeadIntegrity) {
			return rejectCurrentGenesisFrame(
				projecttypeenvselectioneffect.NotSelectedCorruptHeadSlot(),
			), nil
		}
		return nil, fmt.Errorf(
			"reload current dedicated TypeEnv head: %w",
			err,
		)
	}
	stageResult := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 ready.Stage(),
			FinalVerification:     ready.FinalLowererVerification(),
			ExecutableTarget:      ready.ExecutableSnapshot(),
			TargetRuntimeRegistry: targetRuntime,
			CurrentGraph:          currentGraph,
			ReferenceKindFacts:    referenceKindFacts,
			CurrentProfile:        currentProfile,
			CurrentHead:           currentHead,
		},
	)
	currentStage, ok :=
		stageResult.(projecttypeenvstagerevalidation.CurrentSelectionStage)
	if !ok || !currentStage.Valid() {
		if reason, rejected := stageRevalidationRejection(stageResult); rejected {
			return rejectCurrentGenesisFrame(reason), nil
		}
		return nil, fmt.Errorf(
			"current Stage revalidation did not mint CurrentSelectionStage: %T",
			stageResult,
		)
	}
	headObservation, err := projecttypeenvselectionreadset.ObserveGenesisHeadTx(
		ctx,
		dependencies.heads,
		transaction,
		projecttypeenvselectionreadset.GenesisHeadObservationInput{
			Request:      request,
			Stage:        ready.Stage(),
			CurrentGraph: currentGraph.GraphSnapshotBasis(),
			ObservedAt:   dependencies.observedAt,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"observe Genesis head read set: %w",
			err,
		)
	}
	headReadSet, ok :=
		headObservation.(projecttypeenvselectionreadset.GenesisHeadSelectionReadSet)
	if !ok {
		switch headObservation.(type) {
		case projecttypeenvselectionreadset.GenesisPredecessorConflict:
			return rejectCurrentGenesisFrame(
				projecttypeenvselectioneffect.NotSelectedPriorHeadExists(),
			), nil
		case projecttypeenvselectionreadset.HeadStorageCorrupt:
			return rejectCurrentGenesisFrame(
				projecttypeenvselectioneffect.NotSelectedCorruptHeadSlot(),
			), nil
		}
		return nil, fmt.Errorf(
			"genesis head observation did not mint a read set: %T",
			headObservation,
		)
	}
	if err := headReadSet.VerifyForTransaction(transaction); err != nil {
		return nil, fmt.Errorf(
			"verify transaction-private Genesis head read set: %w",
			err,
		)
	}
	frame := currentGenesisFrame{
		readyStage:     ready,
		currentStage:   currentStage,
		headReadSet:    headReadSet,
		currentGraph:   currentGraph,
		currentProfile: currentProfile,
		targetRuntime:  targetRuntime,
		currentHead:    currentHead,
		projectRoot:    projectRoot,
	}
	return currentGenesisFrameReady{frame: frame}, nil
}

func exactMatchedTargetRuntime(
	matched projecttypeenvruntime.Matched,
) (projecttypeenvruntime.ExactTargetRuntimeRegistry, error) {
	registry, ok := matched.Registry()
	if !ok || !registry.Valid() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{}, fmt.Errorf(
			"matched target runtime did not expose an exact registry",
		)
	}
	return registry, nil
}

func selectionReadyLoadRejection(
	err error,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
	bool,
) {
	switch {
	case errors.Is(err, projecttypeenvstage.ErrStageNotFound),
		errors.Is(err, projecttypeenvstore.ErrArtifactNotFound):
		return projecttypeenvselectioneffect.NotSelectedTargetSnapshotMissing(), true
	case errors.Is(err, projecttypeenvstage.ErrStageConflict),
		errors.Is(err, projecttypeenvstore.ErrArtifactConflict):
		return projecttypeenvselectioneffect.NotSelectedTargetSnapshotConflict(), true
	case errors.Is(err, projecttypeenvstage.ErrStageIntegrity),
		errors.Is(err, projecttypeenvstore.ErrArtifactIntegrity),
		errors.Is(err, projecttypeenvstore.ErrClosureInconsistent),
		errors.Is(err, projecttypeenvstore.ErrBaseNotExecutable),
		errors.Is(err, projecttypeenvstore.ErrRuntimeClosureRequired),
		errors.Is(err, projecttypeenvstore.ErrRuntimeBasisRebuildRequired):
		return projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(), true
	default:
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
	}
}

func stageRevalidationRejection(
	result projecttypeenvstagerevalidation.StageRevalidationResult,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
	bool,
) {
	switch current := result.(type) {
	case projecttypeenvstagerevalidation.InvalidSelectionStage:
		if len(current.Issues()) == 0 {
			return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
		}
		return projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(), true
	case projecttypeenvstagerevalidation.DriftedSelectionStage:
		if len(current.Issues()) == 0 {
			return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
		}
		reason := driftedStageNotSelectedReason(current.Issues())
		return reason, true
	case projecttypeenvstagerevalidation.RejectedSelectionStage:
		return rejectedStageNotSelectedReason(current.Issues())
	case projecttypeenvstagerevalidation.UnavailableSelectionStage:
		if len(current.Requirements()) == 0 {
			return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
		}
		return projecttypeenvselectioneffect.NotSelectedStageDrift(), true
	default:
		return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
	}
}

func driftedStageNotSelectedReason(
	issues []projecttypeenvstagerevalidation.StageRevalidationIssue,
) projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason {
	if stageIssuesContainCode(
		issues,
		projecttypeenvstagerevalidation.IssueHeadPresenceMismatch,
	) {
		return projecttypeenvselectioneffect.NotSelectedPriorHeadExists()
	}
	if stageIssuesContainKind(
		issues,
		projecttypeenvstagerevalidation.IssueKindCurrentGraph,
	) {
		return projecttypeenvselectioneffect.NotSelectedStaleGraph()
	}
	if stageIssuesContainKind(
		issues,
		projecttypeenvstagerevalidation.IssueKindProjectProfile,
	) {
		return projecttypeenvselectioneffect.NotSelectedProfileDrift()
	}
	if stageIssuesContainKind(
		issues,
		projecttypeenvstagerevalidation.IssueKindAssertionRevalidation,
	) {
		return projecttypeenvselectioneffect.NotSelectedAssertionRevalidationFailure()
	}
	return projecttypeenvselectioneffect.NotSelectedStageDrift()
}

func rejectedStageNotSelectedReason(
	issues []projecttypeenvstagerevalidation.StageRevalidationIssue,
) (
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
	bool,
) {
	if stageIssuesContainCode(
		issues,
		projecttypeenvstagerevalidation.IssueProfileIncompatible,
	) || stageIssuesContainCode(
		issues,
		projecttypeenvstagerevalidation.IssueProjectionProfileBlocked,
	) {
		return projecttypeenvselectioneffect.NotSelectedProfileIncompatible(), true
	}
	if stageIssuesContainCode(
		issues,
		projecttypeenvstagerevalidation.IssueProfileUnderdetermined,
	) || stageIssuesContainCode(
		issues,
		projecttypeenvstagerevalidation.IssueProfileUnavailable,
	) {
		return projecttypeenvselectioneffect.NotSelectedProfileUnderdetermined(), true
	}
	if stageIssuesContainKind(
		issues,
		projecttypeenvstagerevalidation.IssueKindAssertionRevalidation,
	) {
		return projecttypeenvselectioneffect.NotSelectedAssertionRevalidationFailure(), true
	}
	return projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{}, false
}

func stageIssuesContainCode(
	issues []projecttypeenvstagerevalidation.StageRevalidationIssue,
	code projecttypeenvstagerevalidation.StageRevalidationIssueCode,
) bool {
	for _, issue := range issues {
		if issue.Code() == code {
			return true
		}
	}
	return false
}

func stageIssuesContainKind(
	issues []projecttypeenvstagerevalidation.StageRevalidationIssue,
	kind projecttypeenvstagerevalidation.StageRevalidationIssueKind,
) bool {
	for _, issue := range issues {
		if issue.Kind() == kind {
			return true
		}
	}
	return false
}

func loadBoundProjectRootTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectID string,
) (projectprofile.ProjectRootV1, error) {
	var raw string
	err := transaction.ScanOne(
		ctx,
		`SELECT project_root
		 FROM project_ledger_binding
		 WHERE project_id = ?`,
		[]any{projectID},
		[]any{&raw},
	)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf(
			"load exact project-root binding: %w",
			err,
		)
	}
	root, err := projectprofile.NewProjectRootV1(raw)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf(
			"parse exact project-root binding: %w",
			err,
		)
	}
	return root, nil
}

func loadCurrentProjectProfileBasisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
) (projecttypeenvprofilebasis.CurrentProjectProfileBasis, error) {
	return profilebasissqlite.LoadCurrentWithin(ctx, transaction, root)
}
