package typedmemorystore

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestPrepareAndSealProjectTypeEnvActivationGraphIsDeterministic(t *testing.T) {
	input := activationAdapterTestInput(t)
	left, err := PrepareProjectTypeEnvActivationGraph(input)
	if err != nil {
		t.Fatalf("PrepareProjectTypeEnvActivationGraph(left): %v", err)
	}
	right, err := PrepareProjectTypeEnvActivationGraph(input)
	if err != nil {
		t.Fatalf("PrepareProjectTypeEnvActivationGraph(right): %v", err)
	}
	if left.EventRef() != right.EventRef() ||
		left.EventDigest() != right.EventDigest() ||
		left.CommitRef() != right.CommitRef() ||
		left.ProjectionJobRef() != right.ProjectionJobRef() ||
		left.GraphRevision() != right.GraphRevision() {
		t.Fatalf("same activation inputs produced different graph identity")
	}
	envelope, err := projecttypeenvactivation.NewAdmissionEnvelope(
		input.Delta,
		input.StorageIdempotencyKey,
	)
	if err != nil {
		t.Fatalf("NewAdmissionEnvelope: %v", err)
	}
	basis, err := projecttypeenvactivation.NewAdmissionBasis(input.Delta, envelope)
	if err != nil {
		t.Fatalf("NewAdmissionBasis: %v", err)
	}
	manifest, err := projecttypeenvactivation.NewMaterializationManifest(
		input.Delta,
		envelope,
		basis,
		left.EventRef(),
		left.CommitRef(),
	)
	if err != nil {
		t.Fatalf("NewMaterializationManifest: %v", err)
	}
	prepared, err := SealPreparedProjectTypeEnvActivationGraph(
		left,
		envelope,
		basis,
		manifest,
	)
	if err != nil {
		t.Fatalf("SealPreparedProjectTypeEnvActivationGraph: %v", err)
	}
	if prepared.EventRef() != left.EventRef() ||
		prepared.EventDigest() != left.EventDigest() ||
		prepared.CommitRef() != left.CommitRef() ||
		prepared.GraphRevision() != typedmemory.NewGraphRevision(1) ||
		prepared.MaterializationDigest().String() == "" {
		t.Fatalf("prepared activation graph lost exact storage coordinates")
	}
}

func TestPrepareProjectTypeEnvActivationGraphRejectsNoOpBasis(t *testing.T) {
	input := activationAdapterTestInput(t)
	input.BasisTypeEnv = input.Request.Target().VerifiedComposite()
	if _, err := PrepareProjectTypeEnvActivationGraph(input); err == nil {
		t.Fatalf("activation with equal current basis and result C was accepted")
	}
}

func TestPrepareGenesisActivationRequiresTargetBaseAsBasis(t *testing.T) {
	input := activationAdapterTestInput(t)
	input.BasisTypeEnv = activationAdapterTestTypeEnv(t, 'c')
	if _, err := PrepareProjectTypeEnvActivationGraph(input); err == nil {
		t.Fatalf("Genesis activation with a non-B current basis was accepted")
	}
}

func TestActivationTxWriterRequiresCallerOwnedImmediateTransaction(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	transaction, err := sqlitetransaction.BeginRead(
		context.Background(),
		fixture.database,
	)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = transaction.Rollback(context.Background()) }()
	adapter, err := NewProjectTypeEnvActivationAdapter(
		fixture.clock,
		&projecttypeenvstage.Store{},
	)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvActivationAdapter: %v", err)
	}
	_, err = adapter.WritePreparedProjectTypeEnvActivationGraphTx(
		context.Background(),
		transaction,
		PreparedProjectTypeEnvActivationGraph{},
		func(
			context.Context,
			*sqlitetransaction.Transaction,
			ProjectTypeEnvActivationWriteContext,
		) error {
			return nil
		},
	)
	if !errors.Is(err, sqlitetransaction.ErrImmediateRequired) {
		t.Fatalf("read transaction error = %v; want ErrImmediateRequired", err)
	}
}

func activationAdapterTestInput(t *testing.T) ProjectTypeEnvActivationGraphInput {
	t.Helper()
	base := activationAdapterTestTypeEnv(t, '1')
	return activationAdapterTestInputForProject(
		t,
		"qnt_1234abcd",
		base,
	)
}

func activationAdapterTestInputForProject(
	t *testing.T,
	project string,
	base typedmemory.TypeEnvRef,
) ProjectTypeEnvActivationGraphInput {
	t.Helper()
	result := activationAdapterTestTypeEnv(t, '2')
	basis := base
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:" + activationAdapterTestDigest('4').String(),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef: %v", err)
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(
		"project-typeenv-stage:" + activationAdapterTestDigest('5').String(),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvStageRef: %v", err)
	}
	requestBytes := activationAdapterTestGenesisRequestBytes(
		project,
		base,
		result,
		runtimeBasis,
		stage,
	)
	request, err := projecttypeenvselection.DecodeProjectTypeEnvHeadSelectionRequest(
		requestBytes,
	)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvHeadSelectionRequest: %v", err)
	}
	head, err := request.Head()
	if err != nil {
		t.Fatalf("request.Head: %v", err)
	}
	target, err := projecttypeenvactivation.NewTarget(
		projecttypeenvactivation.TargetInput{
			Base:         base,
			RuntimeBasis: runtimeBasis,
			Composite:    result,
			Stage:        stage,
		},
	)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	transactionDigest := activationAdapterTestDigest('7')
	contentDigest, err := authority.NewDigest(
		activationAdapterTestDigest('8').String(),
	)
	if err != nil {
		t.Fatalf("NewDigest: %v", err)
	}
	work, err := authority.NewWorkRef("work:activation-adapter-test")
	if err != nil {
		t.Fatalf("NewWorkRef: %v", err)
	}
	successor, err := projecttypeenvselection.NewHeadRevision(1)
	if err != nil {
		t.Fatalf("NewHeadRevision: %v", err)
	}
	delta, err := projecttypeenvactivation.NewDelta(
		projecttypeenvactivation.DeltaInput{
			TransactionRef: "project-typeenv-head-selection-transaction:" +
				transactionDigest.String(),
			TransactionDigest: transactionDigest,
			Project:           request.Project(),
			Head:              head,
			RequestRef:        request.Ref(),
			RequestDigest:     request.Ref().Digest(),
			ContentDigest:     contentDigest,
			AuthorityUseRef: "project-typeenv-head-selection-authority-use:" +
				activationAdapterTestDigest('9').String(),
			WorkRef: work,
			WorkRecordRef: "project-typeenv-head-cas-work-record:" +
				activationAdapterTestDigest('a').String(),
			Predecessor:            request.Predecessor(),
			Target:                 target,
			ExpectedGraphRevision:  typedmemory.NewGraphRevision(0),
			CommittedGraphRevision: typedmemory.NewGraphRevision(1),
			SuccessorHeadRevision:  successor,
			AuthorityClass: projecttypeenvactivation.
				HostRoutedOperatorRequestAuthorityClass,
		},
	)
	if err != nil {
		t.Fatalf("NewDelta: %v", err)
	}
	return ProjectTypeEnvActivationGraphInput{
		Request:               request,
		BasisTypeEnv:          basis,
		StorageIdempotencyKey: "project-typeenv-head-activation:" + strings.Repeat("b", 64),
		Delta:                 delta,
	}
}

func activationAdapterTestGenesisRequestBytes(
	project string,
	base typedmemory.TypeEnvRef,
	result typedmemory.TypeEnvRef,
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef,
	stage projecttypeenvselection.ProjectTypeEnvStageRef,
) []byte {
	writer := activationAdapterTestWriter{}
	writer.addString("haft.project-typeenv.head-selection-request.v2")
	writer.addString(project)
	writer.addString("genesis")
	writer.addString(base.String())
	writer.addUint64(0)
	writer.addString(runtimeBasis.String())
	writer.addString(result.String())
	writer.addString(stage.String())
	writer.addUint64(0)
	writer.addString("activation-adapter-request")
	return writer.value
}

type activationAdapterTestWriter struct {
	value []byte
}

func (writer *activationAdapterTestWriter) addString(value string) {
	writer.addUint64(uint64(len(value)))
	writer.value = append(writer.value, value...)
}

func (writer *activationAdapterTestWriter) addUint64(value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.value = append(writer.value, encoded[:]...)
}

func activationAdapterTestDigest(fill byte) typedmemory.SHA256Digest {
	value, _ := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(string(fill), 64),
	)
	return value
}

func activationAdapterTestTypeEnv(t *testing.T, fill byte) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.NewTypeEnvRef(activationAdapterTestDigest(fill))
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}
