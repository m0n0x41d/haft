package typedmemorystore

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// LoadExactTargetMemberOfFactViewTx builds a read-only target-C membership
// fact view from the current graph/entity universe and durable observable
// catalog inside the caller-owned transaction. It does not load or reinterpret
// historical MemberOf judgements produced under the graph head's active C.
func LoadExactTargetMemberOfFactViewTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	observedBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	target typedmemory.TypeEnv,
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (projectgraphobservation.ExactTargetMemberOfFactView, error) {
	if err := observedBasis.Verify(); err != nil {
		return nil, fmt.Errorf("load exact target MemberOf facts: graph basis: %w", err)
	}
	if observedBasis.Project() != project {
		return nil, fmt.Errorf("load exact target MemberOf facts: project mismatch")
	}
	targetRef, err := typedmemory.ParseTypeEnvRef(target.Ref().String())
	if err != nil || targetRef != target.Ref() {
		return nil, fmt.Errorf("load exact target MemberOf facts: target C is required")
	}
	memberOf, available := runtime.MemberOfRegistry()
	if !runtime.Valid() || !available {
		return nil, fmt.Errorf("load exact target MemberOf facts: exact target runtime is required")
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
	observables, err := loadCurrentObservableInputCatalog(ctx, transaction, head)
	if err != nil {
		return nil, err
	}
	repair, err := typedmemory.NewRepairPointer(memberOfSnapshotRepair)
	if err != nil {
		return nil, err
	}
	snapshot := &currentMemorySnapshot{
		project:         project,
		revision:        currentBasis.GraphRevision(),
		typeEnv:         target.Ref(),
		environment:     target,
		memberOfEngine:  memberOf,
		memberOfSources: observables,
		memberOfRepair:  repair,
		entityContexts:  entities,
	}
	return exactTargetMemberOfFactView{
		target:   target.Ref(),
		basis:    currentBasis,
		snapshot: snapshot,
	}, nil
}

type exactTargetMemberOfFactView struct {
	target   typedmemory.TypeEnvRef
	basis    projecttypeenvselection.ProjectGraphSnapshotBasis
	snapshot *currentMemorySnapshot
}

var _ projectgraphobservation.ExactTargetMemberOfFactView = exactTargetMemberOfFactView{}

func (view exactTargetMemberOfFactView) TargetTypeEnv() typedmemory.TypeEnvRef {
	return view.target
}

func (view exactTargetMemberOfFactView) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return view.basis
}

func (view exactTargetMemberOfFactView) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return view.snapshot.EvaluateMemberOf(request)
}
