package identityreconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// VerifiedClosure is the minimum exact closure needed by the generic current
// graph reader. It grants no identity mutation capability.
type VerifiedClosure struct {
	eventRef              string
	commitRef             string
	materializationDigest typedmemory.SHA256Digest
}

func (closure VerifiedClosure) EventRef() string { return closure.eventRef }

func (closure VerifiedClosure) CommitRef() string { return closure.commitRef }

func (closure VerifiedClosure) MaterializationDigest() typedmemory.SHA256Digest {
	return closure.materializationDigest
}

// VerifyCommittedClosure proves that eventRef is one complete immutable v52
// reviewed reconciliation. found=false means the event belongs to another
// writer generation; malformed v52 rows fail closed.
func VerifyCommittedClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	eventRef string,
) (VerifiedClosure, bool, error) {
	if ctx == nil {
		return VerifiedClosure{}, false, fmt.Errorf("verify identity reconciliation: context is required")
	}
	if transaction == nil {
		return VerifiedClosure{}, false, ErrDatabaseRequired
	}
	var operationText string
	var contextText string
	var primaryText string
	var basisText string
	var typeEnvText string
	var basisRevision int64
	var payloadDigestText string
	var provenanceText string
	var idempotencyText string
	err := transaction.ScanOne(
		ctx,
		`SELECT reconciliation.operation, reconciliation.bounded_context_ref,
			reconciliation.primary_entity_id, reconciliation.reconciliation_basis_ref,
			reconciliation.basis_type_env_ref, reconciliation.basis_graph_revision,
			reconciliation.review_payload_digest, reconciliation.review_provenance_ref,
			commit_record.idempotency_key
		FROM typed_memory_identity_reconciliations reconciliation
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = reconciliation.project_id
			AND commit_record.commit_ref = reconciliation.commit_ref
			AND commit_record.event_ref = reconciliation.event_ref
		WHERE reconciliation.project_id = ? AND reconciliation.event_ref = ?`,
		[]any{project.String(), eventRef},
		[]any{
			&operationText,
			&contextText,
			&primaryText,
			&basisText,
			&typeEnvText,
			&basisRevision,
			&payloadDigestText,
			&provenanceText,
			&idempotencyText,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return VerifiedClosure{}, false, nil
	}
	if err != nil {
		return VerifiedClosure{}, false, fmt.Errorf("load reviewed identity reconciliation: %w", err)
	}
	if basisRevision < 0 {
		return VerifiedClosure{}, true, fmt.Errorf("%w: negative reconciliation basis revision", ErrStoredIntegrity)
	}
	related, err := loadParticipantEntities(ctx, transaction, project, eventRef)
	if err != nil {
		return VerifiedClosure{}, true, err
	}
	contextRef, err := typedmemory.NewBoundedContextRef(contextText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("bounded context", err)
	}
	primary, err := typedmemory.NewEntityID(primaryText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("primary entity", err)
	}
	basisRef, err := typedmemory.NewReconciliationBasisRef(basisText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("basis reference", err)
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(typeEnvText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("TypeEnv", err)
	}
	payloadDigest, err := typedmemory.NewSHA256Digest(payloadDigestText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("review payload digest", err)
	}
	provenance, err := typedmemory.NewProvenanceRef(provenanceText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("review provenance", err)
	}
	operation := typedmemory.IdentityReconciliationOperation(operationText)
	resolvedBasis, err := typedmemory.NewResolvedReconciliationBasis(
		basisRef,
		operation,
		contextRef,
		primary,
		related,
		typedmemory.NewGraphRevision(uint64(basisRevision)),
		typeEnvRef,
		payloadDigest,
		provenance,
	)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("resolved basis", err)
	}
	change, err := rebuildIdentityChange(operation, primary, related, contextRef, basisRef)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("identity effect", err)
	}
	admission, err := typedmemory.NewReviewedIdentityReconciliationAdmission(change, resolvedBasis)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("reviewed admission", err)
	}
	key, err := NewIdempotencyKey(idempotencyText)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("idempotency key", err)
	}
	request, err := NewRequestBuilder().
		SetProject(project).
		SetIdempotencyKey(key).
		SetAdmission(admission).
		Build()
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("replay request", err)
	}
	prepared, err := prepareRequest(request)
	if err != nil {
		return VerifiedClosure{}, true, storedParseError("canonical replay", err)
	}
	if prepared.eventRef != eventRef {
		return VerifiedClosure{}, true, fmt.Errorf("%w: stored event reference is not canonical", ErrStoredIntegrity)
	}
	if err := verifyStoredReplayRows(ctx, transaction, prepared); err != nil {
		return VerifiedClosure{}, true, err
	}
	return VerifiedClosure{
		eventRef:              prepared.eventRef,
		commitRef:             prepared.commitRef,
		materializationDigest: prepared.materializationDigest,
	}, true, nil
}

func loadParticipantEntities(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	eventRef string,
) ([]typedmemory.EntityID, error) {
	var encoded string
	err := transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(entity_id), '[]')
		FROM (
			SELECT entity_id
			FROM typed_memory_identity_reconciliation_participants
			WHERE project_id = ? AND event_ref = ?
			ORDER BY participant_ordinal
		)`,
		[]any{project.String(), eventRef},
		[]any{&encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("load identity-reconciliation participants: %w", err)
	}
	values := []string{}
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return nil, storedParseError("participants", err)
	}
	entities := make([]typedmemory.EntityID, 0, len(values))
	for _, value := range values {
		entity, err := typedmemory.NewEntityID(value)
		if err != nil {
			return nil, storedParseError("participant entity", err)
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func rebuildIdentityChange(
	operation typedmemory.IdentityReconciliationOperation,
	primary typedmemory.EntityID,
	related []typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	basis typedmemory.ReconciliationBasisRef,
) (typedmemory.IdentityChange, error) {
	switch operation {
	case typedmemory.ReconciliationMergeEntities:
		return typedmemory.NewMergeEntities(primary, related, contextRef, basis)
	case typedmemory.ReconciliationSplitEntity:
		return typedmemory.NewSplitEntity(primary, related, contextRef, basis)
	default:
		return nil, fmt.Errorf("unknown reconciliation operation %q", operation)
	}
}

func storedParseError(field string, err error) error {
	return fmt.Errorf("%w: parse stored %s: %v", ErrStoredIntegrity, field, err)
}
