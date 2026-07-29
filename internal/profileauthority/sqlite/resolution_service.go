package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

var (
	ErrAuthorityResolutionRequired = errors.New("pre-Work profile authority resolution is required")
	ErrAuthorityAlreadyConsumed    = errors.New("profile authority single-use key is already consumed")
)

type ResolutionWriteResult struct {
	kind   WriteKind
	record profileauthority.AuthorityResolutionRecord
	detail string
}

func (result ResolutionWriteResult) Kind() WriteKind {
	return result.kind
}

func (result ResolutionWriteResult) Record() (
	profileauthority.AuthorityResolutionRecord,
	bool,
) {
	usable := result.kind == WriteStaged
	usable = usable || result.kind == WriteExactReplay
	usable = usable || result.kind == WriteRecovered
	if !usable {
		return profileauthority.AuthorityResolutionRecord{}, false
	}
	_, ok := result.record.Digest()
	return result.record, ok
}

func (result ResolutionWriteResult) RejectionDetail() (string, bool) {
	return result.detail, result.kind == WriteRejected && result.detail != ""
}

// InstituteResolution performs the pre-Work enactability judgement. It owns
// its BEGIN IMMEDIATE, persists only the immutable resolution record, and never
// returns an AdmittedUse. Final use is minted only by post-Work exact replay.
func (store *Store) InstituteResolution(
	ctx context.Context,
	snapshot ClosureSnapshot,
) (ResolutionWriteResult, error) {
	if err := store.validateResolutionStore(ctx); err != nil {
		return ResolutionWriteResult{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.database)
	if err != nil {
		return ResolutionWriteResult{}, fmt.Errorf("begin profile authority resolution: %w", err)
	}
	result, err := store.instituteResolutionInTransaction(ctx, transaction, snapshot)
	if err != nil {
		finish := transaction.Rollback(context.Background())
		return ResolutionWriteResult{}, errors.Join(err, finish.Err())
	}
	if result.kind == WriteRejected || result.kind == WriteExactReplay {
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return ResolutionWriteResult{}, finish.Err()
		}
		return result, nil
	}
	ref, _ := result.record.Ref()
	digest, _ := result.record.Digest()
	finish := transaction.Commit(ctx)
	kind := WriteStaged
	if !finish.Succeeded() {
		kind = WriteRecovered
	}
	durable, loadErr := LoadAuthorityResolution(
		context.Background(),
		store.database,
		ref,
		digest,
	)
	if loadErr == nil {
		return ResolutionWriteResult{kind: kind, record: durable}, nil
	}
	return ResolutionWriteResult{}, errors.Join(
		fmt.Errorf("profile authority resolution commit outcome is unknown"),
		finish.Err(),
		loadErr,
	)
}

func (store *Store) instituteResolutionInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot ClosureSnapshot,
) (ResolutionWriteResult, error) {
	if err := store.validateMutation(ctx, transaction); err != nil {
		return ResolutionWriteResult{}, err
	}
	closure, err := store.ValidateClosureSnapshotInTransaction(ctx, transaction, snapshot)
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	basisRef, basisDigest, _ := snapshot.Basis()
	existing, found, err := scanResolutionByBasis(ctx, transaction, basisRef.String())
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	if found {
		record, reconstructErr := reconstructResolution(existing, closure)
		if reconstructErr != nil {
			return ResolutionWriteResult{}, reconstructErr
		}
		consumed, consumedErr := resolutionConsumedInTransaction(ctx, transaction, existing.ref)
		if consumedErr != nil {
			return ResolutionWriteResult{}, consumedErr
		}
		if consumed {
			return ResolutionWriteResult{
				kind:   WriteRejected,
				detail: ErrAuthorityAlreadyConsumed.Error(),
			}, nil
		}
		validity, validityOK := record.PermissionValidity()
		if !validityOK || !validity.Contains(canonicalTime(store.now())) {
			return ResolutionWriteResult{
				kind:   WriteRejected,
				detail: "stored profile authority resolution is no longer current",
			}, nil
		}
		return ResolutionWriteResult{kind: WriteExactReplay, record: record}, nil
	}
	ref, err := deriveResolutionRef(basisDigest)
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	evaluated := profileauthority.EvaluateNewResolution(
		ref,
		closure,
		canonicalTime(store.now()),
	)
	if evaluated.Kind() == profileauthority.ResolutionDenied {
		denied, _ := evaluated.Denied()
		return ResolutionWriteResult{
			kind:   WriteRejected,
			detail: formatResolutionDenials(denied.Reasons()),
		}, nil
	}
	created, ok := evaluated.New()
	if !ok {
		return ResolutionWriteResult{}, fmt.Errorf("profile authority resolution gate returned no new record")
	}
	record, ok := created.Record()
	if !ok {
		return ResolutionWriteResult{}, fmt.Errorf("new profile authority resolution is unavailable")
	}
	row, err := buildResolutionRow(record)
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	_, err = transaction.Execute(ctx, insertResolutionSQL, row.args())
	if err != nil {
		if isConstraint(err) {
			return ResolutionWriteResult{kind: WriteRejected, detail: err.Error()}, nil
		}
		return ResolutionWriteResult{}, fmt.Errorf("insert profile authority resolution: %w", err)
	}
	durableRow, found, err := scanResolutionByRef(ctx, transaction, row.ref)
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	if !found {
		return ResolutionWriteResult{}, fmt.Errorf("inserted profile authority resolution disappeared before commit")
	}
	durable, err := reconstructResolution(durableRow, closure)
	if err != nil {
		return ResolutionWriteResult{}, err
	}
	return ResolutionWriteResult{kind: WriteStaged, record: durable}, nil
}

func LoadAuthorityResolution(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.ProfileDeclarationAuthorityResolutionRef,
	digest authority.Digest,
) (profileauthority.AuthorityResolutionRecord, error) {
	if err := requireV44(database); err != nil {
		return profileauthority.AuthorityResolutionRecord{}, err
	}
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, err
	}
	row, found, scanErr := scanResolutionByRef(ctx, transaction, ref.String())
	if scanErr == nil && !found {
		scanErr = sql.ErrNoRows
	}
	if scanErr == nil && row.digest != digest.String() {
		scanErr = fmt.Errorf("profile authority resolution digest differs from requested identity")
	}
	row, err = finishReadValue(ctx, transaction, row, scanErr)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, err
	}
	basisRef, basisDigest, err := resolutionBasis(row)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("parse resolution basis: %w", err)
	}
	closure, err := LoadClosure(ctx, database, basisRef, basisDigest)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("load resolution source closure: %w", err)
	}
	return reconstructResolution(row, closure)
}

// ResolveForAdmission is the post-Work transaction-local authority gate. The
// caller prepares the opaque snapshot before BEGIN IMMEDIATE. This method then
// exact-rereads every closure and resolution member, rejects consumed authority,
// samples judgement time, and only then returns a sealed AdmittedUse.
func (store *Store) ResolveForAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot ClosureSnapshot,
) (
	profileauthority.AdmittedUse,
	[]profileauthority.ResolutionDenial,
	error,
) {
	if err := store.validateMutation(ctx, transaction); err != nil {
		return profileauthority.AdmittedUse{}, nil, err
	}
	closure, err := store.ValidateClosureSnapshotInTransaction(ctx, transaction, snapshot)
	if err != nil {
		return profileauthority.AdmittedUse{}, nil, err
	}
	basisRef, _, _ := snapshot.Basis()
	row, found, err := scanResolutionByBasis(ctx, transaction, basisRef.String())
	if err != nil {
		return profileauthority.AdmittedUse{}, nil, err
	}
	if !found {
		return profileauthority.AdmittedUse{}, nil, ErrAuthorityResolutionRequired
	}
	consumed, err := resolutionConsumedInTransaction(ctx, transaction, row.ref)
	if err != nil {
		return profileauthority.AdmittedUse{}, nil, err
	}
	if consumed {
		return profileauthority.AdmittedUse{}, nil, ErrAuthorityAlreadyConsumed
	}
	record, err := reconstructResolution(row, closure)
	if err != nil {
		return profileauthority.AdmittedUse{}, nil, err
	}
	evaluated := profileauthority.EvaluateReplayResolution(
		record,
		closure,
		canonicalTime(store.now()),
	)
	if evaluated.Kind() == profileauthority.ResolutionDenied {
		denied, _ := evaluated.Denied()
		return profileauthority.AdmittedUse{}, denied.Reasons(), nil
	}
	replay, ok := evaluated.Replay()
	if !ok {
		return profileauthority.AdmittedUse{}, nil, fmt.Errorf("profile authority replay returned no admitted branch")
	}
	use, ok := replay.AdmittedUse()
	if !ok {
		return profileauthority.AdmittedUse{}, nil, fmt.Errorf("profile authority replay returned no sealed use")
	}
	return use, nil, nil
}

func resolutionConsumedInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	resolutionRef string,
) (bool, error) {
	count := 0
	err := transaction.ScanOne(
		ctx,
		"SELECT COUNT(*) FROM profile_declaration_authority_uses_v2 WHERE authority_resolution_ref = ?",
		[]any{resolutionRef},
		[]any{&count},
	)
	if err != nil {
		return false, fmt.Errorf("inspect profile authority consumption: %w", err)
	}
	if count > 1 {
		return false, fmt.Errorf("profile authority resolution has multiple durable uses")
	}
	return count == 1, nil
}

func (store *Store) validateResolutionStore(ctx context.Context) error {
	if store == nil || store.database == nil || store.now == nil {
		return fmt.Errorf("profile authority SQLite store is not open")
	}
	if ctx == nil {
		return fmt.Errorf("profile authority resolution requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return requireV44(store.database)
}

func requireV44(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("profile authority schema v44 requires a database")
	}
	versionCount := 0
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 44",
	).Scan(&versionCount)
	if err != nil || versionCount != 1 {
		return errors.Join(fmt.Errorf("profile authority schema v44 is unavailable"), err)
	}
	tableCount := 0
	err = database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master
		 WHERE type = 'table' AND name IN (?, ?)`,
		resolutionTable,
		"profile_declaration_authority_uses_v2",
	).Scan(&tableCount)
	if err != nil || tableCount != 2 {
		return errors.Join(fmt.Errorf("profile authority schema v44 tables are incomplete"), err)
	}
	return nil
}

func formatResolutionDenials(reasons []profileauthority.ResolutionDenial) string {
	if len(reasons) == 0 {
		return "profile authority resolution was denied"
	}
	details := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		details = append(details, string(reason.Code())+": "+reason.Detail())
	}
	return fmt.Sprint(details)
}
