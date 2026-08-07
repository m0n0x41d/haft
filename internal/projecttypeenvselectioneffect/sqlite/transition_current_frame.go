package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectmemory/kindclassificationengine"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
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

type currentTransitionFrame struct {
	readyStage     projecttypeenvstage.SelectionReadyStage
	currentStage   projecttypeenvstagerevalidation.CurrentSelectionStage
	headReadSet    projecttypeenvselectionreadset.TransitionHeadSelectionReadSet
	prior          projecttypeenv.ProjectTypeEnvExecutableSnapshot
	currentGraph   typedmemorystore.CurrentProjectGraphObservation
	currentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis
	targetRuntime  projecttypeenvruntime.ExactTargetRuntimeRegistry
	projectRoot    projectprofile.ProjectRootV1
}

type currentTransitionFrameResult interface {
	currentTransitionFrameResultVariant()
}

type currentTransitionFrameReady struct {
	frame currentTransitionFrame
}

func (currentTransitionFrameReady) currentTransitionFrameResultVariant() {}

type currentTransitionFrameRejected struct {
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason
}

func (currentTransitionFrameRejected) currentTransitionFrameResultVariant() {}

func rejectCurrentTransitionFrame(
	reason projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
) currentTransitionFrameResult {
	return currentTransitionFrameRejected{reason: reason}
}

func loadCurrentTransitionFrameTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	dependencies currentGenesisFrameDependencies,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) (currentTransitionFrameResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("load current Transition frame: context is required")
	}
	if transaction == nil {
		return nil, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	if dependencies.stages == nil || dependencies.heads == nil {
		return nil, fmt.Errorf(
			"load current Transition frame: Stage and head stores are required",
		)
	}
	if err := request.Verify(); err != nil {
		return nil, fmt.Errorf("load current Transition frame request: %w", err)
	}
	if _, ok := request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); !ok {
		return nil, fmt.Errorf("load current Transition frame requires Transition predecessor")
	}
	ready, err := dependencies.stages.LoadSelectionReadyTx(
		ctx,
		transaction,
		request.Target().Stage(),
	)
	if err != nil {
		if reason, rejected := selectionReadyLoadRejection(err); rejected {
			return rejectCurrentTransitionFrame(reason), nil
		}
		return nil, fmt.Errorf("reload Transition selection-ready Stage: %w", err)
	}
	runtimeBasis, err := projecttypeenvstore.GetRuntimeEvaluationBasisArtifactTx(
		ctx,
		transaction,
		ready.Stage().RuntimeBasis(),
	)
	if err != nil {
		if reason, rejected := selectionReadyLoadRejection(err); rejected {
			return rejectCurrentTransitionFrame(reason), nil
		}
		return nil, fmt.Errorf("reload Transition target runtime basis X: %w", err)
	}
	runtimeResolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtimeBasis,
			Installed:    dependencies.installedRuntime,
		},
	)
	targetRuntime, reason, err := resolveTransitionTargetRuntime(runtimeResolution)
	if err != nil {
		return nil, err
	}
	if reason.String() != "" {
		return rejectCurrentTransitionFrame(reason), nil
	}
	currentGraph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		request.Project(),
	)
	if err != nil {
		return nil, fmt.Errorf("reload current typed-memory graph: %w", err)
	}
	classificationEngine, err := kindclassificationengine.ForExactTargetRuntime(
		targetRuntime,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct Transition target-C classification engine: %w",
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
	headObservation, err := projecttypeenvselectionreadset.ObserveTransitionHeadTx(
		ctx,
		dependencies.heads,
		transaction,
		projecttypeenvselectionreadset.TransitionHeadObservationInput{
			Request:      request,
			Stage:        ready.Stage(),
			CurrentGraph: currentGraph.GraphSnapshotBasis(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("observe Transition head read set: %w", err)
	}
	headReadSet, ok := headObservation.(projecttypeenvselectionreadset.TransitionHeadSelectionReadSet)
	if !ok {
		switch headObservation.(type) {
		case projecttypeenvselectionreadset.TransitionHeadAbsent:
			return rejectCurrentTransitionFrame(
				projecttypeenvselectioneffect.NotSelectedPriorHeadAbsent(),
			), nil
		case projecttypeenvselectionreadset.TransitionPredecessorConflict:
			return rejectCurrentTransitionFrame(
				projecttypeenvselectioneffect.NotSelectedStalePriorHead(),
			), nil
		case projecttypeenvselectionreadset.HeadStorageCorrupt:
			return rejectCurrentTransitionFrame(
				projecttypeenvselectioneffect.NotSelectedCorruptHeadSlot(),
			), nil
		default:
			return nil, fmt.Errorf(
				"transition head observation did not mint a read set: %T",
				headObservation,
			)
		}
	}
	if err := headReadSet.VerifyForTransaction(transaction); err != nil {
		return nil, fmt.Errorf(
			"verify transaction-private Transition head read set: %w",
			err,
		)
	}
	priorHead, ok := headReadSet.PriorHead()
	if !ok {
		return nil, fmt.Errorf("transition read set omitted exact prior head")
	}
	prior, err := dependencies.stages.LoadExecutableSnapshotTx(
		ctx,
		transaction,
		priorHead.SelectedComposite(),
	)
	if err != nil {
		if reason, rejected := selectionReadyLoadRejection(err); rejected {
			return rejectCurrentTransitionFrame(reason), nil
		}
		return nil, fmt.Errorf("reload prior selected executable C: %w", err)
	}
	currentHead, err := projecttypeenvstagerevalidation.NewObservedProjectTypeEnvHead(
		priorHead,
	)
	if err != nil {
		return nil, fmt.Errorf("restore current Transition head observation: %w", err)
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
			PriorExecutable:       prior,
		},
	)
	currentStage, ok := stageResult.(projecttypeenvstagerevalidation.CurrentSelectionStage)
	if !ok || !currentStage.Valid() {
		if reason, rejected := stageRevalidationRejection(stageResult); rejected {
			return rejectCurrentTransitionFrame(reason), nil
		}
		return nil, fmt.Errorf(
			"current Transition Stage revalidation did not mint CurrentSelectionStage: %T",
			stageResult,
		)
	}
	frame := currentTransitionFrame{
		readyStage:     ready,
		currentStage:   currentStage,
		headReadSet:    headReadSet,
		prior:          prior,
		currentGraph:   currentGraph,
		currentProfile: currentProfile,
		targetRuntime:  targetRuntime,
		projectRoot:    projectRoot,
	}
	return currentTransitionFrameReady{frame: frame}, nil
}

func resolveTransitionTargetRuntime(
	resolution projecttypeenvruntime.Resolution,
) (
	projecttypeenvruntime.ExactTargetRuntimeRegistry,
	projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason,
	error,
) {
	switch current := resolution.(type) {
	case projecttypeenvruntime.Invalid:
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			projecttypeenvselectioneffect.NotSelectedTargetIntegrityFailure(),
			nil
	case projecttypeenvruntime.Unavailable,
		projecttypeenvruntime.Drifted:
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			projecttypeenvselectioneffect.NotSelectedStageDrift(),
			nil
	case projecttypeenvruntime.Matched:
		registry, err := exactMatchedTargetRuntime(current)
		return registry,
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{},
			err
	default:
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{},
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionNotSelectedReason{},
			fmt.Errorf(
				"target runtime X returned an invalid result variant: %T",
				resolution,
			)
	}
}
