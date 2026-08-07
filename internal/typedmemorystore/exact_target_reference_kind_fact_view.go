package typedmemorystore

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type exactTargetReferenceKindRuntimePosture uint8

const (
	exactTargetReferenceKindRuntimeNone exactTargetReferenceKindRuntimePosture = iota + 1
	exactTargetReferenceKindRuntimeHistorical
	exactTargetReferenceKindRuntimeCurrent
	exactTargetReferenceKindRuntimeMixed
)

func exactTargetReferenceKindRuntimePostureForCounts(
	memberOfCount int,
	classificationCount int,
) exactTargetReferenceKindRuntimePosture {
	switch {
	case memberOfCount == 0 && classificationCount == 0:
		return exactTargetReferenceKindRuntimeNone
	case memberOfCount > 0 && classificationCount == 0:
		return exactTargetReferenceKindRuntimeHistorical
	case memberOfCount == 0 && classificationCount > 0:
		return exactTargetReferenceKindRuntimeCurrent
	default:
		return exactTargetReferenceKindRuntimeMixed
	}
}

// LoadExactTargetReferenceKindFactViewTx derives the exact semantic-mechanism
// posture declared by target X. Historical MemberOf, current direct
// KindClassification, and explicit absence are disjoint variants; a mixed
// runtime fails closed rather than selecting an implicit fallback.
func LoadExactTargetReferenceKindFactViewTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	observedBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	target typedmemory.TypeEnv,
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
	classificationEngine ExactKindClassificationAdmissionEngine,
) (projectgraphobservation.ExactTargetReferenceKindFactView, error) {
	if !runtime.Valid() {
		return nil, fmt.Errorf(
			"load exact target reference-kind facts: exact target runtime is required",
		)
	}
	memberOf, memberOfAvailable := runtime.MemberOfRegistry()
	classification, classificationAvailable := runtime.KindClassificationRegistry()
	if !memberOfAvailable || !classificationAvailable {
		return nil, fmt.Errorf(
			"load exact target reference-kind facts: target runtime registries are unavailable",
		)
	}
	posture := exactTargetReferenceKindRuntimePostureForCounts(
		memberOf.Len(),
		classification.Len(),
	)
	switch posture {
	case exactTargetReferenceKindRuntimeHistorical:
		view, err := LoadExactTargetMemberOfFactViewTx(
			ctx,
			transaction,
			project,
			observedBasis,
			target,
			runtime,
		)
		if err != nil {
			return nil, err
		}
		facts, err := projectgraphobservation.NewExactTargetMemberOfReferenceKindFacts(view)
		if err != nil {
			return nil, err
		}
		return facts, nil
	case exactTargetReferenceKindRuntimeCurrent:
		view, err := loadExactTargetKindClassificationFactViewTx(
			ctx,
			transaction,
			project,
			observedBasis,
			target,
			runtime,
			classificationEngine,
		)
		if err != nil {
			return nil, err
		}
		facts, err := projectgraphobservation.NewExactTargetKindClassificationReferenceKindFacts(view)
		if err != nil {
			return nil, err
		}
		return facts, nil
	case exactTargetReferenceKindRuntimeNone:
		facts, err := loadExactTargetNoReferenceKindFactsTx(
			ctx,
			transaction,
			project,
			observedBasis,
			target,
		)
		if err != nil {
			return nil, err
		}
		return facts, nil
	default:
		return nil, fmt.Errorf(
			"load exact target reference-kind facts: target X mixes historical MemberOf and current KindClassification",
		)
	}
}

func loadExactTargetNoReferenceKindFactsTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	observedBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	target typedmemory.TypeEnv,
) (projectgraphobservation.ExactTargetNoReferenceKindFacts, error) {
	if err := observedBasis.Verify(); err != nil {
		return projectgraphobservation.ExactTargetNoReferenceKindFacts{}, fmt.Errorf(
			"load exact target no-reference-kind facts: graph basis: %w",
			err,
		)
	}
	if observedBasis.Project() != project {
		return projectgraphobservation.ExactTargetNoReferenceKindFacts{}, fmt.Errorf(
			"load exact target no-reference-kind facts: project mismatch",
		)
	}
	currentHead, currentBasis, err := loadCurrentGraphSnapshotBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return projectgraphobservation.ExactTargetNoReferenceKindFacts{}, err
	}
	if currentHead.Project() != project || currentBasis.Ref() != observedBasis.Ref() {
		return projectgraphobservation.ExactTargetNoReferenceKindFacts{}, ErrStaleGraphRevision
	}
	facts, err := projectgraphobservation.NewExactTargetNoReferenceKindFacts(
		target.Ref(),
		currentBasis,
	)
	if err != nil {
		return projectgraphobservation.ExactTargetNoReferenceKindFacts{}, err
	}
	return facts, nil
}

func loadExactTargetKindClassificationFactViewTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	observedBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	target typedmemory.TypeEnv,
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
	engine ExactKindClassificationAdmissionEngine,
) (projectgraphobservation.ExactTargetKindClassificationFactView, error) {
	if err := observedBasis.Verify(); err != nil {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: graph basis: %w",
			err,
		)
	}
	if observedBasis.Project() != project {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: project mismatch",
		)
	}
	targetRef, err := typedmemory.ParseTypeEnvRef(target.Ref().String())
	if err != nil || targetRef != target.Ref() {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: target C is required",
		)
	}
	classification, available := runtime.KindClassificationRegistry()
	if !runtime.Valid() || !available || classification.Len() == 0 {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: exact target runtime is required",
		)
	}
	if !exactKindClassificationAdmissionEngineIsPresent(engine) {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: correlated admission engine is required",
		)
	}
	if !kindClassificationRegistryIdentitiesEqual(
		classification,
		engine.ExactKindClassificationRegistry(),
	) {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: admission engine differs from target X",
		)
	}
	codecs, available := runtime.CodecRegistry()
	if !available {
		return nil, fmt.Errorf(
			"load exact target KindClassification facts: target codec registry is unavailable",
		)
	}
	head, currentBasis, err := loadCurrentGraphSnapshotBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		return nil, err
	}
	if currentBasis.Ref() != observedBasis.Ref() {
		return nil, ErrStaleGraphRevision
	}
	entities, err := loadCurrentEntityContexts(ctx, transaction, head)
	if err != nil {
		return nil, err
	}
	sources, err := loadCurrentKindClassificationSourceCatalog(
		ctx,
		transaction,
		head,
	)
	if err != nil {
		return nil, err
	}
	historical, err := loadCurrentObservableInputCatalog(
		ctx,
		transaction,
		head,
	)
	if err != nil {
		return nil, err
	}
	sources, err = extendKindClassificationSourceCatalogWithSealedHistorical(
		project,
		engine,
		historical,
		sources,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"adapt sealed historical target-C classification delivery sources: %w",
			err,
		)
	}
	resolutionBasis, err := snapshotResolutionBasis(
		project,
		currentBasis.GraphRevision(),
	)
	if err != nil {
		return nil, err
	}
	snapshot := &currentMemorySnapshot{
		project:               project,
		revision:              currentBasis.GraphRevision(),
		typeEnv:               target.Ref(),
		environment:           target,
		codecs:                codecs,
		classificationEngine:  engine,
		classificationSources: sources,
		resolutionBasis:       resolutionBasis,
		entityContexts:        entities,
	}
	return exactTargetKindClassificationFactView{
		target:   target.Ref(),
		basis:    currentBasis,
		snapshot: snapshot,
	}, nil
}

type exactTargetKindClassificationFactView struct {
	target   typedmemory.TypeEnvRef
	basis    projecttypeenvselection.ProjectGraphSnapshotBasis
	snapshot *currentMemorySnapshot
}

var _ projectgraphobservation.ExactTargetKindClassificationFactView = exactTargetKindClassificationFactView{}

func (view exactTargetKindClassificationFactView) TargetTypeEnv() typedmemory.TypeEnvRef {
	return view.target
}

func (view exactTargetKindClassificationFactView) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return view.basis
}

func (view exactTargetKindClassificationFactView) EvaluateKindClassification(
	request typedmemory.KindClassificationRequest,
) typedmemory.KindClassificationJudgement {
	return view.snapshot.EvaluateKindClassification(request)
}

func exactKindClassificationAdmissionEngineIsPresent(
	engine ExactKindClassificationAdmissionEngine,
) bool {
	return interfaceCapabilityPresent(engine)
}

func kindClassificationRegistryIdentitiesEqual(
	left kindclassificationruntime.Registry,
	right kindclassificationruntime.Registry,
) bool {
	if left.Len() != right.Len() {
		return false
	}
	for _, registration := range left.Registrations() {
		other, found := right.Registration(registration.RuleRef())
		if !found || other.Identity() != registration.Identity() {
			return false
		}
	}
	return true
}
