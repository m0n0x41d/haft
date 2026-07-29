package typedmemorystore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// loadPersistedObservableInputTx recovers one exact already-committed input
// from the same transaction and revision used for revalidation. A prospective
// source is deliberately absent here and must come from the sealed request-
// scoped provider instead.
func loadPersistedObservableInputTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	input typedmemory.MemberOfObservableInput,
) (ObservableInputBlob, bool, error) {
	if transaction == nil {
		return ObservableInputBlob{}, false, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return ObservableInputBlob{}, false, err
	}
	sqliteRevision, exactRevision := sqliteIntegerFromUint64(revision.Value())
	if !exactRevision {
		return ObservableInputBlob{}, false, ErrRevisionOverflow
	}
	var encoded string
	err := transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(json_object(
			'event_revision', graph_revision,
			'event_ref', event_ref,
			'observable_input_ref', observable_input_ref,
			'observable_input_digest', observable_input_digest,
			'canonical_observable_input_hex', canonical_observable_input_hex
		)), '[]')
		FROM (
			SELECT event.graph_revision, observable.event_ref,
				observable.observable_input_ref,
				observable.observable_input_digest,
				hex(observable.canonical_observable_input_bytes)
					AS canonical_observable_input_hex
			FROM typed_memory_observable_input_blobs observable
			JOIN typed_memory_graph_events event
				ON event.project_id = observable.project_id
				AND event.event_ref = observable.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE observable.project_id = ?
				AND event.graph_revision <= ?
				AND observable.observable_input_ref = ?
				AND observable.observable_input_digest = ?
			ORDER BY event.graph_revision, observable.event_ref
		)`,
		[]any{
			project.String(),
			sqliteRevision,
			input.Reference().String(),
			input.Digest().String(),
		},
		[]any{&encoded},
	)
	if err != nil {
		return ObservableInputBlob{}, false, fmt.Errorf(
			"load persisted observable input: %w",
			err,
		)
	}
	rows := []storedObservableInputRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return ObservableInputBlob{}, false, storedAdmissionIntegrity(
			"decode persisted observable input",
			err,
		)
	}
	if len(rows) == 0 {
		return ObservableInputBlob{}, false, nil
	}
	blobs := make([]ObservableInputBlob, 0, len(rows))
	for _, row := range rows {
		blob, err := decodeStoredObservableInputRow(row)
		if err != nil {
			return ObservableInputBlob{}, false, err
		}
		if blob.Reference() != input.Reference() ||
			blob.Digest() != input.Digest() {
			return ObservableInputBlob{}, false, storedAdmissionIntegrity(
				"persisted observable input coordinate mismatch",
				ErrAdmissionEnvelopeMismatch,
			)
		}
		blobs = append(blobs, blob)
	}
	catalog, err := newImmutableObservableInputCatalog(blobs)
	if err != nil {
		return ObservableInputBlob{}, false, storedAdmissionIntegrity(
			"construct persisted observable input",
			err,
		)
	}
	exact := catalog.Blobs()
	if len(exact) != 1 {
		return ObservableInputBlob{}, false, storedAdmissionIntegrity(
			"persisted observable input is ambiguous",
			ErrAdmissionEnvelopeMismatch,
		)
	}
	return exact[0], true, nil
}
