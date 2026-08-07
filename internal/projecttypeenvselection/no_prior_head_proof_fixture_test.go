package projecttypeenvselection

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type noPriorHeadProofInput struct {
	Project               projectidentity.ProjectID
	Head                  ProjectTypeEnvHeadRef
	GraphSnapshot         ProjectGraphSnapshotBasis
	ExpectedGraphRevision typedmemory.GraphRevision
}

func sealNoPriorHeadProof(input noPriorHeadProofInput) (NoPriorHeadProofRecord, error) {
	if err := input.GraphSnapshot.Verify(); err != nil {
		return NoPriorHeadProofRecord{}, fmt.Errorf("verify no-prior-head graph snapshot: %w", err)
	}
	state, err := normalizeNoPriorHeadProofState(noPriorHeadProofState{
		project:               input.Project,
		head:                  input.Head,
		graphSnapshot:         input.GraphSnapshot.Ref(),
		graphSnapshotDigest:   input.GraphSnapshot.Ref().Digest(),
		expectedGraphRevision: input.ExpectedGraphRevision,
	})
	if err != nil {
		return NoPriorHeadProofRecord{}, err
	}
	if input.GraphSnapshot.Project() != state.project {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head graph snapshot project mismatch")
	}
	if input.GraphSnapshot.GraphRevision() != state.expectedGraphRevision {
		return NoPriorHeadProofRecord{}, fmt.Errorf("no-prior-head graph snapshot revision mismatch")
	}
	canonical := encodeNoPriorHeadProofState(state)
	return DecodeNoPriorHeadProof(canonical)
}
