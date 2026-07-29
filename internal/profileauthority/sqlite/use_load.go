package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectAuthorityUseSQL = `SELECT
	use_ref, use_digest, project_root, action_kind, project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	permission_ref, permission_digest,
	authorization_content_ref, authorization_content_digest,
	single_use_key, admission_request_digest,
	committed_admission_ref, committed_admission_digest,
	canonical_json, consumed_at, recorded_at
FROM profile_declaration_authority_uses_v2`

type authorityUseRow struct {
	ref                      string
	digest                   string
	projectRoot              string
	actionKind               string
	projectBindingDigest     string
	resolutionRef            string
	resolutionDigest         string
	basisRef                 string
	basisDigest              string
	permissionRef            string
	permissionDigest         string
	contentRef               string
	contentDigest            string
	singleUseKey             string
	admissionRequestDigest   string
	committedAdmissionRef    string
	committedAdmissionDigest string
	canonical                string
	consumedAt               string
	recordedAt               string
}

func (row *authorityUseRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.projectRoot, &row.actionKind,
		&row.projectBindingDigest, &row.resolutionRef, &row.resolutionDigest,
		&row.basisRef, &row.basisDigest, &row.permissionRef,
		&row.permissionDigest, &row.contentRef, &row.contentDigest,
		&row.singleUseKey, &row.admissionRequestDigest,
		&row.committedAdmissionRef, &row.committedAdmissionDigest,
		&row.canonical, &row.consumedAt, &row.recordedAt,
	}
}

type authorityUseCanonicalJSON struct {
	Schema                   string `json:"schema"`
	Ref                      string `json:"use_ref"`
	ProjectRoot              string `json:"project_root"`
	ActionKind               string `json:"action_kind"`
	ProjectBindingDigest     string `json:"project_binding_digest"`
	ResolutionRef            string `json:"authority_resolution_ref"`
	ResolutionDigest         string `json:"authority_resolution_digest"`
	BasisRef                 string `json:"authority_basis_ref"`
	BasisDigest              string `json:"authority_basis_digest"`
	PermissionRef            string `json:"permission_ref"`
	PermissionDigest         string `json:"permission_digest"`
	ContentRef               string `json:"authorization_content_ref"`
	ContentDigest            string `json:"authorization_content_digest"`
	SingleUseKey             string `json:"single_use_key"`
	AdmissionRequestDigest   string `json:"admission_request_digest"`
	CommittedAdmissionRef    string `json:"committed_admission_ref"`
	CommittedAdmissionDigest string `json:"committed_admission_digest"`
	ConsumedAt               string `json:"consumed_at"`
}

func scanAuthorityUseByRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (authorityUseRow, bool, error) {
	return scanAuthorityUse(
		ctx,
		transaction,
		selectAuthorityUseSQL+" WHERE use_ref = ?",
		ref,
	)
}

func scanAuthorityUseByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (authorityUseRow, bool, error) {
	return scanAuthorityUse(
		ctx,
		transaction,
		selectAuthorityUseSQL+" WHERE authority_basis_ref = ?",
		ref,
	)
}

func scanAuthorityUse(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	statement string,
	value string,
) (authorityUseRow, bool, error) {
	row := authorityUseRow{}
	err := transaction.ScanOne(ctx, statement, []any{value}, row.scanTargets())
	if err == sql.ErrNoRows {
		return authorityUseRow{}, false, nil
	}
	if err != nil {
		return authorityUseRow{}, false, fmt.Errorf("scan profile authority use: %w", err)
	}
	return row, true, nil
}

func LoadAuthorityUseRecord(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.ProfileDeclarationAuthorityUseRef,
	digest authority.Digest,
) (profileauthority.AuthorityUseRecord, error) {
	if err := requireV44(database); err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	row, found, scanErr := scanAuthorityUseByRef(ctx, transaction, ref.String())
	if scanErr == nil && !found {
		scanErr = sql.ErrNoRows
	}
	if scanErr == nil && row.digest != digest.String() {
		scanErr = fmt.Errorf("profile authority use digest differs from requested identity")
	}
	row, err = finishReadValue(ctx, transaction, row, scanErr)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	resolutionRef, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(row.resolutionRef)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse use resolution ref: %w", err)
	}
	resolutionDigest, err := authority.NewDigest(row.resolutionDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse use resolution digest: %w", err)
	}
	resolution, err := LoadAuthorityResolution(
		ctx,
		database,
		resolutionRef,
		resolutionDigest,
	)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	basisRef, basisDigest, ok := resolution.Basis()
	if !ok {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("strict resolution omitted basis")
	}
	closure, err := LoadClosure(ctx, database, basisRef, basisDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	return reconstructAuthorityUse(row, resolution, closure)
}

// LoadAuthorityUseRecordInTransaction strictly reconstructs one historical
// authority use and its complete source-native closure inside the caller's
// active SQLite snapshot. It does not own commit or rollback.
func LoadAuthorityUseRecordInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref profileauthority.ProfileDeclarationAuthorityUseRef,
	digest authority.Digest,
) (profileauthority.AuthorityUseRecord, error) {
	if ctx == nil || transaction == nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf(
			"transactional profile authority-use load requires context and transaction",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	parsedRef, err := profileauthority.NewProfileDeclarationAuthorityUseRef(ref.String())
	if err != nil || parsedRef != ref {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf(
			"transactional profile authority-use ref is invalid",
		)
	}
	parsedDigest, err := authority.NewDigest(digest.String())
	if err != nil || parsedDigest != digest {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf(
			"transactional profile authority-use digest is invalid",
		)
	}
	row, found, err := scanAuthorityUseByRef(ctx, transaction, ref.String())
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	if !found {
		return profileauthority.AuthorityUseRecord{}, sql.ErrNoRows
	}
	if row.digest != digest.String() {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf(
			"profile authority use digest differs from requested identity",
		)
	}
	resolutionRef, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(
		row.resolutionRef,
	)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	resolutionDigest, err := authority.NewDigest(row.resolutionDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	resolutionRow, found, err := scanResolutionByRef(
		ctx,
		transaction,
		resolutionRef.String(),
	)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	if !found || resolutionRow.digest != resolutionDigest.String() {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf(
			"exact profile authority resolution is unavailable",
		)
	}
	basisRef, err := profileauthority.NewBasisRef(resolutionRow.basisRef)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	basisDigest, err := authority.NewDigest(resolutionRow.basisDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	closure, err := loadHistoricalClosureInTransaction(
		ctx,
		transaction,
		basisRef,
		basisDigest,
	)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	resolution, err := reconstructResolution(resolutionRow, closure)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	return reconstructAuthorityUse(row, resolution, closure)
}

func loadHistoricalClosureInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef profileauthority.BasisRef,
	basisDigest authority.Digest,
) (profileauthority.Closure, error) {
	row, found, err := scanBasisByRef(ctx, transaction, basisRef.String())
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if !found || row.digest != basisDigest.String() {
		return profileauthority.Closure{}, fmt.Errorf("exact profile four-ref basis is unavailable")
	}
	preparation := preparationRow{}
	err = transaction.ScanOne(
		ctx,
		selectPreparationSQL+" WHERE authorization_content_ref = ? AND authorization_content_digest = ?",
		[]any{row.contentRef, row.contentDigest},
		preparation.scanTargets(),
	)
	if err != nil {
		return profileauthority.Closure{}, fmt.Errorf(
			"load exact prepared authorization in transaction: %w",
			err,
		)
	}
	prepared, err := reconstructPrepared(ctx, transaction, preparation)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	speechActRef, err := authority.NewSpeechActRef(row.speechActRef)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	speechActDigest, err := authority.NewDigest(row.speechActDigest)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	source, err := authority.LoadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		speechActRef,
		speechActDigest,
	)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if err := profileauthority.ValidateRecordedSource(prepared, source); err != nil {
		return profileauthority.Closure{}, err
	}
	policyRef, err := authority.NewContextPolicyRef(row.contextPolicyRef)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	policyDigest, err := authority.NewDigest(row.contextPolicyDigest)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	policy, err := authority.LoadSpeechActContextPolicyInTransaction(
		ctx,
		transaction,
		policyRef,
		policyDigest,
	)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if err := validatePreparedPolicy(prepared, policy); err != nil {
		return profileauthority.Closure{}, err
	}
	sources := exactClosureSources{
		prepared: prepared,
		source:   source,
		policy:   policy,
	}
	return loadClosureInTransaction(
		ctx,
		transaction,
		sources,
		basisRef,
		basisDigest,
	)
}

// ResolveAuthorityUseForBasisInTransaction strictly reconstructs historical
// use from the same opaque snapshot used by the admission gate. No DB-level
// loader is called while the caller owns BEGIN IMMEDIATE.
func (store *Store) ResolveAuthorityUseForBasisInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot ClosureSnapshot,
) (profileauthority.AuthorityUseRecord, bool, error) {
	if err := store.validateMutation(ctx, transaction); err != nil {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	closure, err := store.ValidateClosureSnapshotInTransaction(ctx, transaction, snapshot)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	basisRef, _, _ := snapshot.Basis()
	useRow, found, err := scanAuthorityUseByBasis(ctx, transaction, basisRef.String())
	if err != nil || !found {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	resolutionRow, resolutionFound, err := scanResolutionByBasis(
		ctx,
		transaction,
		basisRef.String(),
	)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	if !resolutionFound {
		return profileauthority.AuthorityUseRecord{}, false, fmt.Errorf(
			"durable authority use has no exact resolution",
		)
	}
	resolution, err := reconstructResolution(resolutionRow, closure)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	use, err := reconstructAuthorityUse(useRow, resolution, closure)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, false, err
	}
	return use, true, nil
}

func reconstructAuthorityUse(
	row authorityUseRow,
	resolution profileauthority.AuthorityResolutionRecord,
	closure profileauthority.Closure,
) (profileauthority.AuthorityUseRecord, error) {
	consumedAt, err := parseCanonicalTime(row.consumedAt)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse authority use consumed_at: %w", err)
	}
	replayed := profileauthority.EvaluateReplayResolution(resolution, closure, consumedAt)
	if replayed.Kind() != profileauthority.ResolutionReplay {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use failed resolution replay")
	}
	replay, ok := replayed.Replay()
	if !ok {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use replay omitted result")
	}
	admitted, ok := replay.AdmittedUse()
	if !ok {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use replay omitted sealed use")
	}
	ref, err := profileauthority.NewProfileDeclarationAuthorityUseRef(row.ref)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse authority use ref: %w", err)
	}
	requestDigest, err := authority.NewDigest(row.admissionRequestDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse authority use request digest: %w", err)
	}
	committedRef, err := profileauthority.NewCommittedProfileAdmissionRef(row.committedAdmissionRef)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse committed admission ref: %w", err)
	}
	committedDigest, err := authority.NewDigest(row.committedAdmissionDigest)
	if err != nil {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("parse committed admission digest: %w", err)
	}
	evaluated := profileauthority.EvaluateNewAuthorityUse(
		ref,
		admitted,
		requestDigest,
		committedRef,
		committedDigest,
		consumedAt,
	)
	if evaluated.Kind() != profileauthority.AuthorityUseNew {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use failed pure reconstruction")
	}
	created, ok := evaluated.New()
	if !ok {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use reconstruction omitted record")
	}
	record, ok := created.Record()
	if !ok {
		return profileauthority.AuthorityUseRecord{}, fmt.Errorf("stored authority use record is unavailable")
	}
	if err := validateAuthorityUseRow(row, record); err != nil {
		return profileauthority.AuthorityUseRecord{}, err
	}
	return record, nil
}

func validateAuthorityUseRow(
	row authorityUseRow,
	record profileauthority.AuthorityUseRecord,
) error {
	digest, digestOK := record.Digest()
	canonical, canonicalOK := record.CanonicalBytes()
	consumedAt, consumedAtOK := record.ConsumedAt()
	recordedAt, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && consumedAtOK && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	valid = valid && consumedAt.Equal(recordedAt)
	if !valid {
		return fmt.Errorf("stored profile authority use failed canonical rehash")
	}
	expected := authorityUseCanonicalJSON{}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return fmt.Errorf("decode canonical profile authority use: %w", err)
	}
	actual := authorityUseCanonicalJSON{
		Schema: "haft.profile-authority.authority-use/v2",
		Ref:    row.ref, ProjectRoot: row.projectRoot, ActionKind: row.actionKind,
		ProjectBindingDigest: row.projectBindingDigest,
		ResolutionRef:        row.resolutionRef, ResolutionDigest: row.resolutionDigest,
		BasisRef: row.basisRef, BasisDigest: row.basisDigest,
		PermissionRef: row.permissionRef, PermissionDigest: row.permissionDigest,
		ContentRef: row.contentRef, ContentDigest: row.contentDigest,
		SingleUseKey: row.singleUseKey, AdmissionRequestDigest: row.admissionRequestDigest,
		CommittedAdmissionRef:    row.committedAdmissionRef,
		CommittedAdmissionDigest: row.committedAdmissionDigest,
		ConsumedAt:               row.consumedAt,
	}
	if actual != expected {
		return fmt.Errorf("stored profile authority use columns differ from canonical content")
	}
	return nil
}
