package typedmemorystore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// loadPersistedKindClassificationSourceTx recovers one exact already-
// committed source from the same transaction and graph revision used for
// revalidation. Prospective source bytes remain request-scoped and must come
// from the configured provider.
func loadPersistedKindClassificationSourceTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	reference typedmemory.CarrierRef,
	digest typedmemory.SHA256Digest,
) (KindClassificationSourceBlob, bool, error) {
	if transaction == nil {
		return KindClassificationSourceBlob{}, false, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return KindClassificationSourceBlob{}, false, err
	}
	sqliteRevision, exactRevision := sqliteIntegerFromUint64(revision.Value())
	if !exactRevision {
		return KindClassificationSourceBlob{}, false, ErrRevisionOverflow
	}
	var encoded string
	err := transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(json_object(
			'event_revision', graph_revision,
			'event_ref', event_ref,
			'source_ref', source_ref,
			'source_digest', source_digest,
			'canonical_source_hex', canonical_source_hex
		)), '[]')
		FROM (
			SELECT event.graph_revision, source.event_ref,
				source.source_ref, source.source_digest,
				hex(source.canonical_source_bytes) AS canonical_source_hex
			FROM typed_memory_kind_classification_source_blobs_v54 source
			JOIN typed_memory_graph_events event
				ON event.project_id = source.project_id
				AND event.event_ref = source.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE source.project_id = ?
				AND event.graph_revision <= ?
				AND source.source_ref = ?
				AND source.source_digest = ?
			ORDER BY event.graph_revision, source.event_ref
		)`,
		[]any{
			project.String(),
			sqliteRevision,
			reference.String(),
			digest.String(),
		},
		[]any{&encoded},
	)
	if err != nil {
		return KindClassificationSourceBlob{}, false, fmt.Errorf(
			"load persisted kind-classification source: %w",
			err,
		)
	}
	rows := []storedKindClassificationSourceRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return KindClassificationSourceBlob{}, false, storedAdmissionIntegrity(
			"decode persisted kind-classification source",
			err,
		)
	}
	if len(rows) == 0 {
		return KindClassificationSourceBlob{}, false, nil
	}
	blobs := make([]KindClassificationSourceBlob, 0, len(rows))
	for _, row := range rows {
		blob, decodeErr := decodeStoredKindClassificationSourceRow(row)
		if decodeErr != nil {
			return KindClassificationSourceBlob{}, false, decodeErr
		}
		if blob.Reference() != reference || blob.Digest() != digest {
			return KindClassificationSourceBlob{}, false, storedAdmissionIntegrity(
				"persisted kind-classification source coordinate mismatch",
				ErrAdmissionEnvelopeMismatch,
			)
		}
		blobs = append(blobs, blob)
	}
	exact, err := coalesceKindClassificationSourceBlobs(blobs)
	if err != nil {
		return KindClassificationSourceBlob{}, false, storedAdmissionIntegrity(
			"construct persisted kind-classification source",
			err,
		)
	}
	if len(exact) != 1 {
		return KindClassificationSourceBlob{}, false, storedAdmissionIntegrity(
			"persisted kind-classification source is ambiguous",
			ErrAdmissionEnvelopeMismatch,
		)
	}
	return exact[0], true, nil
}
