package typedmemorystore

import (
	"context"
	"testing"
)

func TestGenericReplayRejectsSealedAdmissionBasisCarrierMutation(t *testing.T) {
	tests := []struct {
		name       string
		assignment string
		argument   any
	}{
		{
			name:       "canonical_bytes",
			assignment: "canonical_admission_basis_bytes = ?",
			argument:   []byte("mutated but non-empty admission basis carrier"),
		},
		{
			name:       "digest",
			assignment: "admission_basis_digest = ?",
			argument:   mustDigest(t, []byte("mutated admission basis digest")).String(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExactBasisStoreFixture(t)
			request := fixture.request(t, "basis-carrier-mutation-"+test.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				request,
			)
			if err != nil {
				t.Fatalf("seed sealed admission basis: %v", err)
			}
			fixture.allowTestMutation(t, "typed_memory_event_admission_bases")
			result, err := fixture.base.database.Exec(
				"UPDATE typed_memory_event_admission_bases SET "+test.assignment+
					" WHERE project_id = ? AND event_ref = ?",
				test.argument,
				fixture.base.project.String(),
				receipt.EventRef(),
			)
			if err != nil {
				t.Fatalf("mutate sealed admission basis %s: %v", test.name, err)
			}
			assertExactBasisRowsAffected(t, result, 1, "sealed admission basis "+test.name)

			_, err = fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				request,
			)
			if err == nil {
				t.Fatalf("mutated admission basis %s was accepted", test.name)
			}
		})
	}
}
