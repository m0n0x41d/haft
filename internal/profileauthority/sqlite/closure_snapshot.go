package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type genericSourceRows struct {
	methodDigest     string
	methodCanonical  string
	policyDigest     string
	policyCanonical  string
	captureDigest    string
	captureCanonical string
	authorizerDigest string
	authorizerJSON   string
	speechActDigest  string
	speechActJSON    string
}

func (rows *genericSourceRows) scanTargets() []any {
	return []any{
		&rows.methodDigest,
		&rows.methodCanonical,
		&rows.policyDigest,
		&rows.policyCanonical,
		&rows.captureDigest,
		&rows.captureCanonical,
		&rows.authorizerDigest,
		&rows.authorizerJSON,
		&rows.speechActDigest,
		&rows.speechActJSON,
	}
}

const selectGenericSourceRowsSQL = `SELECT
	method.method_description_digest, method.canonical_json,
	policy.context_policy_digest, policy.canonical_json,
	capture.capture_carrier_digest, capture.canonical_json,
	authorizer.role_assignment_digest, authorizer.canonical_json,
	act.speech_act_digest, act.canonical_json
FROM speech_acts act
JOIN speech_act_method_descriptions method
	ON method.method_description_ref = act.method_description_ref
	AND method.method_description_digest = act.method_description_digest
JOIN speech_act_context_policies policy
	ON policy.context_policy_ref = act.context_policy_ref
	AND policy.context_policy_digest = act.context_policy_digest
JOIN terminal_capture_records capture
	ON capture.capture_carrier_ref = act.capture_carrier_ref
	AND capture.capture_carrier_digest = act.capture_carrier_digest
JOIN speech_act_role_assignments authorizer
	ON authorizer.role_assignment_ref = act.performed_by_ref
	AND authorizer.role_assignment_digest = act.performed_by_digest
	AND authorizer.provenance_carrier_ref = capture.capture_carrier_ref
	AND authorizer.provenance_carrier_digest = capture.capture_carrier_digest
WHERE act.speech_act_ref = ? AND act.speech_act_digest = ?`

// ClosureSnapshot is an opaque, pre-resolved immutable v43 closure and its
// exact generic-source rows. It carries no authority use. A caller-owned
// transaction must still call ValidateClosureSnapshotInTransaction before
// sampling a gate judgement.
type ClosureSnapshot struct {
	basisRef    profileauthority.BasisRef
	basisDigest authority.Digest
	sources     exactClosureSources
	closure     profileauthority.Closure
	genericRows genericSourceRows
}

func (snapshot ClosureSnapshot) Basis() (
	profileauthority.BasisRef,
	authority.Digest,
	bool,
) {
	if !snapshot.valid() {
		return profileauthority.BasisRef{}, authority.Digest{}, false
	}
	return snapshot.basisRef, snapshot.basisDigest, true
}

func (snapshot ClosureSnapshot) Closure() (profileauthority.Closure, bool) {
	if !snapshot.valid() {
		return profileauthority.Closure{}, false
	}
	return snapshot.closure, true
}

func (snapshot ClosureSnapshot) valid() bool {
	if !snapshot.sources.source.Valid() || snapshot.genericRows.speechActJSON == "" {
		return false
	}
	basis, ok := snapshot.closure.Basis()
	if !ok {
		return false
	}
	ref, refOK := basis.Ref()
	digest, digestOK := basis.Digest()
	return refOK && digestOK &&
		ref.String() == snapshot.basisRef.String() &&
		digest.String() == snapshot.basisDigest.String()
}

func (store *Store) PrepareClosureSnapshot(
	ctx context.Context,
	ref profileauthority.BasisRef,
	digest authority.Digest,
) (ClosureSnapshot, error) {
	if store == nil || store.database == nil {
		return ClosureSnapshot{}, fmt.Errorf("profile authority SQLite store is not open")
	}
	row, err := readBasisRow(ctx, store.database, ref, digest)
	if err != nil {
		return ClosureSnapshot{}, err
	}
	bundle, err := loadClosureBundle(ctx, store.database, row)
	if err != nil {
		return ClosureSnapshot{}, err
	}
	speechActRef, speechActDigest, ok := bundle.basis.SpeechAct()
	if !ok {
		return ClosureSnapshot{}, fmt.Errorf("strict four-ref basis omitted SpeechAct pair")
	}
	rows, err := loadGenericSourceRows(
		ctx,
		store.database,
		speechActRef,
		speechActDigest,
	)
	if err != nil {
		return ClosureSnapshot{}, err
	}
	snapshot := ClosureSnapshot{
		basisRef:    ref,
		basisDigest: digest,
		sources:     bundle.sources,
		closure:     bundle.closure,
		genericRows: rows,
	}
	if !snapshot.valid() {
		return ClosureSnapshot{}, fmt.Errorf("prepared closure snapshot is inconsistent")
	}
	return snapshot, nil
}

// PrepareClosureSnapshotForBasis resolves the immutable exact digest owned by
// one unique basis ref, then prepares the same sealed snapshot as the explicit
// ref+digest form. This keeps admission consumers out of v43 storage internals.
func (store *Store) PrepareClosureSnapshotForBasis(
	ctx context.Context,
	ref profileauthority.BasisRef,
) (ClosureSnapshot, error) {
	if store == nil || store.database == nil {
		return ClosureSnapshot{}, fmt.Errorf("profile authority SQLite store is not open")
	}
	digestRaw, found, err := resolveBasisDigest(ctx, store.database, ref)
	if err != nil {
		return ClosureSnapshot{}, err
	}
	if !found {
		return ClosureSnapshot{}, sql.ErrNoRows
	}
	digest, err := authority.NewDigest(digestRaw)
	if err != nil {
		return ClosureSnapshot{}, fmt.Errorf("parse exact basis digest: %w", err)
	}
	return store.PrepareClosureSnapshot(ctx, ref, digest)
}

func resolveBasisDigest(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.BasisRef,
) (string, bool, error) {
	if ctx == nil || database == nil {
		return "", false, fmt.Errorf("resolve basis digest requires a context and database")
	}
	digest := ""
	err := database.QueryRowContext(
		ctx,
		"SELECT basis_digest FROM profile_declaration_authority_bases_v2 WHERE basis_ref = ?",
		ref.String(),
	).Scan(&digest)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve exact basis digest: %w", err)
	}
	return digest, true, nil
}

func (store *Store) ValidateClosureSnapshotInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot ClosureSnapshot,
) (profileauthority.Closure, error) {
	if store == nil || store.database == nil || ctx == nil || !snapshot.valid() {
		return profileauthority.Closure{}, fmt.Errorf(
			"profile closure transaction validation requires an open store and sealed snapshot",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return profileauthority.Closure{}, err
	}
	preparedDigest, _ := snapshot.sources.prepared.Digest()
	prepared, err := loadPreparedExact(ctx, transaction, preparedDigest)
	if err != nil {
		return profileauthority.Closure{}, fmt.Errorf("transaction-reread prepared authorization: %w", err)
	}
	if !sameCanonicalPrepared(prepared, snapshot.sources.prepared) {
		return profileauthority.Closure{}, fmt.Errorf(
			"transaction-reread preparation differs from closure snapshot",
		)
	}
	speechActRef, speechActRefOK := snapshot.sources.source.SpeechActRef()
	speechActDigest, speechActDigestOK := snapshot.sources.source.SpeechActDigest()
	if !speechActRefOK || !speechActDigestOK {
		return profileauthority.Closure{}, fmt.Errorf(
			"closure snapshot generic source omitted SpeechAct address",
		)
	}
	actualRows, err := scanGenericSourceRows(
		ctx,
		transaction,
		speechActRef,
		speechActDigest,
	)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if actualRows != snapshot.genericRows {
		return profileauthority.Closure{}, fmt.Errorf(
			"transaction-reread generic SpeechAct source differs from strict snapshot",
		)
	}
	closure, err := loadClosureInTransaction(
		ctx,
		transaction,
		snapshot.sources,
		snapshot.basisRef,
		snapshot.basisDigest,
	)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if err := compareClosureMembersFromClosures(closure, snapshot.closure); err != nil {
		return profileauthority.Closure{}, err
	}
	return closure, nil
}

func loadGenericSourceRows(
	ctx context.Context,
	database *sql.DB,
	ref authority.SpeechActRef,
	digest authority.Digest,
) (genericSourceRows, error) {
	rows := genericSourceRows{}
	err := database.QueryRowContext(
		ctx,
		selectGenericSourceRowsSQL,
		ref.String(),
		digest.String(),
	).Scan(rows.scanTargets()...)
	if err != nil {
		return genericSourceRows{}, fmt.Errorf("load strict generic SpeechAct rows: %w", err)
	}
	return rows, nil
}

func scanGenericSourceRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref authority.SpeechActRef,
	digest authority.Digest,
) (genericSourceRows, error) {
	rows := genericSourceRows{}
	err := transaction.ScanOne(
		ctx,
		selectGenericSourceRowsSQL,
		[]any{ref.String(), digest.String()},
		rows.scanTargets(),
	)
	if err != nil {
		return genericSourceRows{}, fmt.Errorf(
			"transaction-reread strict generic SpeechAct rows: %w",
			err,
		)
	}
	return rows, nil
}
