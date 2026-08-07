package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionExactWireAdmissionAppliesAndReplays(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	if _, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf("production TypeEnv selection = %T, want FreshlyCommitted", selection)
	}

	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	loader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, loader)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC),
	}
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		productionNoteUnavailableObservableProvider{},
	)
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	current, err := loader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	mustProductionNoteNoError(t, err)
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "admit",
  "basis": {
    "kind": "exact_project",
    "type_env_digest": %q,
    "graph_revision": %d
  },
  "authority_class": %q,
  "idempotency_key": "production-exact-wire-admission",
  "request_provenance_ref": "provenance:production-exact-wire-admission",
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:production-exact-wire-admission",
      "local_ref": "local:production-exact-wire-admission",
      "context": "haft-project",
      "label": "Production exact wire admission",
      "provenance": "provenance:production-exact-wire-admission-change"
    }]
  }
}`,
		typedmemorywire.ContractVersionV2,
		current.Environment().Ref().Digest().String(),
		current.Snapshot().GraphRevision().Value(),
		typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
	)
	request, err := typedmemorywire.DecodeAdmitRequest([]byte(payload))
	mustProductionNoteNoError(t, err)

	result, err := runtime.Admit(ctx, request)
	mustProductionNoteNoError(t, err)
	committed, ok := result.(projectmemory.AdmissionCommitted)
	if !ok {
		t.Fatalf("wire admission = %T, want AdmissionCommitted", result)
	}
	receipt := committed.Receipt()
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf("wire admission disposition = %q, want applied", receipt.Disposition())
	}

	replay, err := runtime.Admit(ctx, request)
	mustProductionNoteNoError(t, err)
	replayed, ok := replay.(projectmemory.AdmissionCommitted)
	if !ok {
		t.Fatalf("wire replay = %T, want AdmissionCommitted", replay)
	}
	replayReceipt := replayed.Receipt()
	if replayReceipt.Disposition() != typedmemorystore.CommitReplay ||
		replayReceipt.EventRef() != receipt.EventRef() ||
		replayReceipt.CommitRef() != receipt.CommitRef() ||
		replayReceipt.GraphRevision() != receipt.GraphRevision() ||
		replayReceipt.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("wire replay = %#v, want exact replay of %#v", replayReceipt, receipt)
	}

	conflictingPayload := bytes.Replace(
		[]byte(payload),
		[]byte(`"label": "Production exact wire admission"`),
		[]byte(`"label": "Conflicting production exact wire admission"`),
		1,
	)
	conflictingRequest, err := typedmemorywire.DecodeAdmitRequest(
		conflictingPayload,
	)
	mustProductionNoteNoError(t, err)
	if _, err := runtime.Admit(ctx, conflictingRequest); !errors.Is(
		err,
		typedmemorystore.ErrIdempotencyConflict,
	) {
		t.Fatalf("conflicting wire replay error = %v, want ErrIdempotencyConflict", err)
	}

	stalePayload := bytes.Replace(
		[]byte(payload),
		[]byte(`"production-exact-wire-admission"`),
		[]byte(`"production-exact-wire-admission-stale"`),
		1,
	)
	staleRequest, err := typedmemorywire.DecodeAdmitRequest(stalePayload)
	mustProductionNoteNoError(t, err)
	staleResult, err := runtime.Admit(ctx, staleRequest)
	mustProductionNoteNoError(t, err)
	if _, ok := staleResult.(projectmemory.AdmissionNotAdmitted); !ok {
		t.Fatalf("stale exact wire admission = %T, want AdmissionNotAdmitted", staleResult)
	}

	entity, err := typedmemory.NewEntityID(
		"entity:production-exact-wire-admission",
	)
	mustProductionNoteNoError(t, err)
	stored, err := adapter.LoadEntity(ctx, fixture.project, entity)
	mustProductionNoteNoError(t, err)
	if stored.Revision() != receipt.GraphRevision() ||
		stored.Provenance().String() !=
			"provenance:production-exact-wire-admission-change" {
		t.Fatalf("stored wire entity = %#v", stored)
	}
}
