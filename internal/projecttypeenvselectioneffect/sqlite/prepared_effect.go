package sqlite

import (
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionreadset"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type preparedGenesisEffect struct {
	request       projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	proof         projecttypeenvselectionreadset.NoPriorHeadProofRecord
	resolved      resolvedGenesisAuthority
	identity      projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTransactionIdentity
	referenceDAG  projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReferenceDAG
	delta         projecttypeenvselectioneffect.ProjectTypeEnvActivationDelta
	envelope      projecttypeenvselectioneffect.ProjectTypeEnvActivationAdmissionEnvelope
	basis         projecttypeenvselectioneffect.ProjectTypeEnvActivationAdmissionBasis
	manifest      projecttypeenvselectioneffect.ProjectTypeEnvActivationMaterializationManifest
	successorHead projecttypeenvselection.ProjectTypeEnvHeadState
	basisTypeEnv  typedmemory.TypeEnvRef
	graph         typedmemorystore.PreparedProjectTypeEnvActivationGraph
	workStartedAt time.Time
}

type sealedGenesisEffect struct {
	prepared     preparedGenesisEffect
	activation   projecttypeenvselectioneffect.CommittedProjectTypeEnvActivation
	result       projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionCommittedResult
	receipt      projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptV1
	authorityUse projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityUseRecord
	casWork      projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkRecord
	closure      projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1
	eventDigest  typedmemory.SHA256Digest
	recordedAt   time.Time
}

func prepareOriginalGenesisEffect(
	transaction *sqlitetransaction.Transaction,
	frame currentGenesisFrame,
	input GenesisSelectionInput,
	authorityUse *admittedGenesisAuthorityUse,
	workStartedAt time.Time,
) (preparedGenesisEffect, error) {
	if err := frame.headReadSet.VerifyForTransaction(transaction); err != nil {
		return preparedGenesisEffect{}, err
	}
	resolved, err := authorityUse.consume(transaction)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	successor, successorOK := frame.headReadSet.SuccessorHead()
	proof, proofOK := frame.headReadSet.Proof()
	committedGraphRevision, graphRevisionOK :=
		frame.headReadSet.CommittedGraphRevision()
	if !successorOK || !proofOK || !graphRevisionOK {
		return preparedGenesisEffect{},
			fmt.Errorf("genesis read set omitted proof or successor coordinates")
	}
	target, err :=
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTargetFromRequest(
			input.Request,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	return prepareOriginalHeadSelectionEffect(
		input.Request,
		proof,
		resolved,
		successor,
		committedGraphRevision,
		target.Base(),
		workStartedAt,
	)
}

func prepareOriginalTransitionEffect(
	transaction *sqlitetransaction.Transaction,
	frame currentTransitionFrame,
	input TransitionSelectionInput,
	authorityUse *admittedGenesisAuthorityUse,
	workStartedAt time.Time,
) (preparedGenesisEffect, error) {
	if err := frame.headReadSet.VerifyForTransaction(transaction); err != nil {
		return preparedGenesisEffect{}, err
	}
	resolved, err := authorityUse.consume(transaction)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	prior, priorOK := frame.headReadSet.PriorHead()
	successor, successorOK := frame.headReadSet.SuccessorHead()
	committedGraphRevision, graphRevisionOK :=
		frame.headReadSet.CommittedGraphRevision()
	if !priorOK || !successorOK || !graphRevisionOK {
		return preparedGenesisEffect{},
			fmt.Errorf("transition read set omitted predecessor or successor coordinates")
	}
	return prepareOriginalHeadSelectionEffect(
		input.Request,
		projecttypeenvselectionreadset.NoPriorHeadProofRecord{},
		resolved,
		successor,
		committedGraphRevision,
		prior.SelectedComposite(),
		workStartedAt,
	)
}

func prepareOriginalHeadSelectionEffect(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	proof projecttypeenvselectionreadset.NoPriorHeadProofRecord,
	resolved resolvedGenesisAuthority,
	successor projecttypeenvselection.ProjectTypeEnvHeadState,
	committedGraphRevision typedmemory.GraphRevision,
	basisTypeEnv typedmemory.TypeEnvRef,
	workStartedAt time.Time,
) (preparedGenesisEffect, error) {
	identity, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionTransactionIdentity(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTransactionIdentityInput{
				Project:                request.Project(),
				IdempotencyKey:         request.IdempotencyKey(),
				RequestRef:             request.Ref(),
				RequestDigest:          request.Ref().Digest(),
				ContentDigest:          resolved.coordinates.ContentDigest(),
				SuccessorHeadRevision:  successor.Revision(),
				CommittedGraphRevision: committedGraphRevision,
			},
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	referenceDAG, err :=
		projecttypeenvselectioneffect.DeriveProjectTypeEnvHeadSelectionReferenceDAG(
			identity,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	target, err :=
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTargetFromRequest(
			request,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	head, err := request.Head()
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	delta, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvActivationDelta(
			projecttypeenvselectioneffect.ProjectTypeEnvActivationDeltaInput{
				Identity:              identity,
				ReferenceDAG:          referenceDAG,
				Head:                  head,
				Predecessor:           request.Predecessor(),
				Target:                target,
				ExpectedGraphRevision: request.ExpectedGraphRevision(),
				AuthorityClass:        resolved.coordinates.Kind().String(),
			},
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	envelope, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvActivationAdmissionEnvelope(
			delta,
			referenceDAG,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	basis, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvActivationAdmissionBasis(
			delta,
			envelope,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	neutralDelta, err := projecttypeenvactivation.DecodeDelta(
		delta.CanonicalBytes(),
	)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	graphIdentity, err := typedmemorystore.PrepareProjectTypeEnvActivationGraph(
		typedmemorystore.ProjectTypeEnvActivationGraphInput{
			Request:               request,
			BasisTypeEnv:          basisTypeEnv,
			StorageIdempotencyKey: referenceDAG.GraphIdempotencyKey().String(),
			Delta:                 neutralDelta,
		},
	)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	graphCoordinates, err :=
		projecttypeenvselectioneffect.NewProjectTypeEnvActivationGraphCoordinates(
			projecttypeenvselectioneffect.ProjectTypeEnvActivationGraphCoordinatesInput{
				Event:  graphIdentity.EventRef(),
				Commit: graphIdentity.CommitRef(),
			},
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	manifest, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvActivationMaterializationManifest(
			delta,
			envelope,
			basis,
			graphCoordinates,
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	neutralEnvelope, err := projecttypeenvactivation.DecodeAdmissionEnvelope(
		envelope.CanonicalBytes(),
	)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	neutralBasis, err := projecttypeenvactivation.DecodeAdmissionBasis(
		basis.CanonicalBytes(),
	)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	neutralManifest, err :=
		projecttypeenvactivation.DecodeMaterializationManifest(
			manifest.CanonicalBytes(),
		)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	graph, err := typedmemorystore.SealPreparedProjectTypeEnvActivationGraph(
		graphIdentity,
		neutralEnvelope,
		neutralBasis,
		neutralManifest,
	)
	if err != nil {
		return preparedGenesisEffect{}, err
	}
	startedAt := workStartedAt.Round(0).UTC()
	if startedAt.IsZero() {
		return preparedGenesisEffect{}, fmt.Errorf("head-selection Work start time is required")
	}
	return preparedGenesisEffect{
		request:       request,
		proof:         proof,
		resolved:      resolved,
		identity:      identity,
		referenceDAG:  referenceDAG,
		delta:         delta,
		envelope:      envelope,
		basis:         basis,
		manifest:      manifest,
		successorHead: successor,
		basisTypeEnv:  basisTypeEnv,
		graph:         graph,
		workStartedAt: startedAt,
	}, nil
}

func (prepared preparedGenesisEffect) seal(
	writeContext typedmemorystore.ProjectTypeEnvActivationWriteContext,
) (sealedGenesisEffect, error) {
	recordedAt, err := time.Parse(time.RFC3339Nano, writeContext.RecordedAt())
	if err != nil {
		return sealedGenesisEffect{}, fmt.Errorf(
			"parse activation effect timestamp: %w",
			err,
		)
	}
	activation, err :=
		projecttypeenvselectioneffect.SealCommittedProjectTypeEnvActivation(
			projecttypeenvselectioneffect.CommittedProjectTypeEnvActivationInput{
				Identity:              prepared.identity,
				ReferenceDAG:          prepared.referenceDAG,
				Delta:                 prepared.delta,
				Envelope:              prepared.envelope,
				Basis:                 prepared.basis,
				Manifest:              prepared.manifest,
				SuccessorHead:         prepared.successorHead,
				MaterializationDigest: writeContext.MaterializationDigest(),
			},
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	result, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionCommittedResult(
			activation,
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	receipt, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionReceiptV1(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptInput{
				Identity:     prepared.identity,
				ReferenceDAG: prepared.referenceDAG,
				Authority:    prepared.resolved.coordinates,
				Activation:   activation,
				Result:       result,
			},
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	verifier, verifierEdition, err := currentHeadSelectionVerifier(
		prepared.request.Predecessor(),
	)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	authorityUse, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionAuthorityUseRecord(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityUseRecordInput{
				Identity:        prepared.identity,
				ReferenceDAG:    prepared.referenceDAG,
				Receipt:         receipt,
				Result:          result,
				Verifier:        verifier,
				VerifierEdition: verifierEdition,
			},
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	method, err := currentGenesisHeadSelectionMethodContract()
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	coordinates, err := method.workCoordinates(
		prepared.resolved.coordinates,
		prepared.workStartedAt,
		recordedAt,
	)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	casWork, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadCASWorkRecord(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkRecordInput{
				Identity:     prepared.identity,
				ReferenceDAG: prepared.referenceDAG,
				Receipt:      receipt,
				AuthorityUse: authorityUse,
				Result:       result,
				Coordinates:  coordinates,
				GenesisProof: prepared.proof.Ref(),
			},
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	closure, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionClosureV1(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureInput{
				Identity:     prepared.identity,
				ReferenceDAG: prepared.referenceDAG,
				Activation:   activation,
				Result:       result,
				Receipt:      receipt,
				AuthorityUse: authorityUse,
				CASWork:      casWork,
			},
		)
	if err != nil {
		return sealedGenesisEffect{}, err
	}
	return sealedGenesisEffect{
		prepared:     prepared,
		activation:   activation,
		result:       result,
		receipt:      receipt,
		authorityUse: authorityUse,
		casWork:      casWork,
		closure:      closure,
		eventDigest:  writeContext.EventDigest(),
		recordedAt:   recordedAt,
	}, nil
}
