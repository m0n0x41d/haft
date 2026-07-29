package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const durableRereadTimeout = 5 * time.Second

type idempotencyBasis struct {
	found             bool
	changeSetDigest   typedmemory.SHA256Digest
	eventRef          string
	commitRef         string
	expectedRevision  typedmemory.GraphRevision
	graphRevision     typedmemory.GraphRevision
	basisTypeEnv      typedmemory.TypeEnvRef
	resultEventDigest typedmemory.SHA256Digest
}

func loadIdempotencyBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	key IdempotencyKey,
) (idempotencyBasis, error) {
	var changeSetDigestText string
	var eventRef string
	var commitRef string
	var expectedRevisionValue int64
	var graphRevisionValue int64
	var basisTypeEnvText string
	var idempotencyResultDigestText string
	var eventDigestText string
	err := transaction.ScanOne(
		ctx,
		`SELECT idempotency.change_set_digest, idempotency.event_ref,
			commit_record.commit_ref, event.expected_revision, event.graph_revision,
			event.basis_type_env_ref, idempotency.result_digest, event.event_digest
		FROM typed_memory_idempotency_history idempotency
		JOIN typed_memory_graph_events event
			ON event.project_id = idempotency.project_id
			AND event.event_ref = idempotency.event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE idempotency.project_id = ? AND idempotency.idempotency_key = ?`,
		[]any{project.String(), key.String()},
		[]any{
			&changeSetDigestText,
			&eventRef,
			&commitRef,
			&expectedRevisionValue,
			&graphRevisionValue,
			&basisTypeEnvText,
			&idempotencyResultDigestText,
			&eventDigestText,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return idempotencyBasis{}, nil
	}
	if err != nil {
		return idempotencyBasis{}, fmt.Errorf("load typed-memory idempotency basis: %w", err)
	}
	changeSetDigest, err := typedmemory.NewSHA256Digest(changeSetDigestText)
	if err != nil {
		return idempotencyBasis{}, err
	}
	resultDigest, err := typedmemory.NewSHA256Digest(idempotencyResultDigestText)
	if err != nil {
		return idempotencyBasis{}, err
	}
	eventDigest, err := typedmemory.NewSHA256Digest(eventDigestText)
	if err != nil {
		return idempotencyBasis{}, err
	}
	if resultDigest != eventDigest {
		return idempotencyBasis{}, fmt.Errorf("typed-memory idempotency result does not match its event digest")
	}
	expectedRevision, err := graphRevisionFromSQLite(expectedRevisionValue)
	if err != nil {
		return idempotencyBasis{}, err
	}
	graphRevision, err := graphRevisionFromSQLite(graphRevisionValue)
	if err != nil {
		return idempotencyBasis{}, err
	}
	if graphRevision.Value() != expectedRevision.Value()+1 {
		return idempotencyBasis{}, fmt.Errorf("typed-memory idempotency event revision is not contiguous")
	}
	basisTypeEnv, err := parseTypeEnvRef(basisTypeEnvText)
	if err != nil {
		return idempotencyBasis{}, err
	}
	return idempotencyBasis{
		found:             true,
		changeSetDigest:   changeSetDigest,
		eventRef:          eventRef,
		commitRef:         commitRef,
		expectedRevision:  expectedRevision,
		graphRevision:     graphRevision,
		basisTypeEnv:      basisTypeEnv,
		resultEventDigest: resultDigest,
	}, nil
}
