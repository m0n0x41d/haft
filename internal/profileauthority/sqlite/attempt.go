package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
)

type AttemptKind string

const (
	AttemptNone AttemptKind = "none"
	// AttemptAwaitingCapture reports a foreign/partial integrity footprint.
	// Production phase one writes preparation+source atomically, so consumers
	// must not prompt for or complete this preparation.
	AttemptAwaitingCapture   AttemptKind = "awaiting_capture"
	AttemptPendingClosure    AttemptKind = "pending_closure"
	AttemptPendingResolution AttemptKind = "pending_resolution"
	AttemptResolutionReady   AttemptKind = "resolution_ready"
	AttemptHistoricalUse     AttemptKind = "historical_use"
	AttemptExpiredHistory    AttemptKind = "expired_history"
	AttemptAmbiguous         AttemptKind = "ambiguous"

	// AttemptClosed is retained as the semantic name used by the current
	// onboarding consumer for a closure that still needs pre-Work resolution.
	AttemptClosed AttemptKind = AttemptPendingResolution
)

type ProjectAttempt struct {
	kind       AttemptKind
	count      int
	prepared   profileauthority.PreparedAuthorization
	recorded   authority.RecordedSpeechActSource
	closure    profileauthority.Closure
	snapshot   ClosureSnapshot
	resolution profileauthority.AuthorityResolutionRecord
	use        profileauthority.AuthorityUseRecord
}

func (attempt ProjectAttempt) Kind() AttemptKind {
	return attempt.kind
}

func (attempt ProjectAttempt) CandidateCount() int {
	return attempt.count
}

func (attempt ProjectAttempt) Prepared() (
	profileauthority.PreparedAuthorization,
	bool,
) {
	usable := attempt.kind == AttemptAwaitingCapture
	usable = usable || attempt.kind == AttemptPendingClosure
	usable = usable || attempt.kind == AttemptPendingResolution
	usable = usable || attempt.kind == AttemptResolutionReady
	if !usable {
		return profileauthority.PreparedAuthorization{}, false
	}
	_, ok := attempt.prepared.Digest()
	return attempt.prepared, ok
}

func (attempt ProjectAttempt) RecordedSource() (
	authority.RecordedSpeechActSource,
	bool,
) {
	usable := attempt.kind == AttemptPendingClosure
	usable = usable || attempt.kind == AttemptPendingResolution
	usable = usable || attempt.kind == AttemptResolutionReady
	return attempt.recorded, usable && attempt.recorded.Valid()
}

func (attempt ProjectAttempt) Closure() (profileauthority.Closure, bool) {
	usable := attempt.kind == AttemptPendingResolution
	usable = usable || attempt.kind == AttemptResolutionReady
	if !usable {
		return profileauthority.Closure{}, false
	}
	_, ok := attempt.closure.Basis()
	return attempt.closure, ok
}

func (attempt ProjectAttempt) ClosureSnapshot() (ClosureSnapshot, bool) {
	usable := attempt.kind == AttemptPendingResolution
	usable = usable || attempt.kind == AttemptResolutionReady
	return attempt.snapshot, usable && attempt.snapshot.valid()
}

func (attempt ProjectAttempt) Resolution() (
	profileauthority.AuthorityResolutionRecord,
	bool,
) {
	if attempt.kind != AttemptResolutionReady {
		return profileauthority.AuthorityResolutionRecord{}, false
	}
	_, ok := attempt.resolution.Digest()
	return attempt.resolution, ok
}

func (attempt ProjectAttempt) HistoricalUse() (
	profileauthority.AuthorityUseRecord,
	bool,
) {
	if attempt.kind != AttemptHistoricalUse {
		return profileauthority.AuthorityUseRecord{}, false
	}
	_, ok := attempt.use.Digest()
	return attempt.use, ok
}

// ResolveProjectAttempt returns the highest live recovery frontier for one
// project/action. It never chooses by timestamp or row order. An unused current
// resolution outranks a current recorded source. Completed and expired history
// never contributes to live ambiguity.
func (store *Store) ResolveProjectAttempt(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
) (ProjectAttempt, error) {
	if err := store.validateAttemptRead(ctx, action); err != nil {
		return ProjectAttempt{}, err
	}
	if err := store.validateUsedAdmissionIntegrity(ctx, root, action); err != nil {
		return ProjectAttempt{}, err
	}
	now := formatTime(store.now())
	resolutionCount, err := store.countLiveResolutions(ctx, root, action, now)
	if err != nil {
		return ProjectAttempt{}, err
	}
	if resolutionCount > 1 {
		return ProjectAttempt{kind: AttemptAmbiguous, count: resolutionCount}, nil
	}
	if resolutionCount == 1 {
		return store.loadUniqueLiveResolutionAttempt(ctx, root, action, now)
	}
	sourceCount, err := store.countLiveRecordedSources(ctx, root, action, now)
	if err != nil {
		return ProjectAttempt{}, err
	}
	if sourceCount > 1 {
		return ProjectAttempt{kind: AttemptAmbiguous, count: sourceCount}, nil
	}
	if sourceCount == 1 {
		return store.loadUniqueLiveSourceAttempt(ctx, root, action, now)
	}
	inertCount, err := store.countLiveInertPreparations(ctx, root, action, now)
	if err != nil {
		return ProjectAttempt{}, err
	}
	if inertCount == 1 {
		return store.loadUniqueInertAttempt(ctx, root, action, now)
	}
	if inertCount > 1 {
		return ProjectAttempt{kind: AttemptAmbiguous, count: inertCount}, nil
	}
	historicalCount, err := store.countCommittedUses(ctx, root, action)
	if err != nil {
		return ProjectAttempt{}, err
	}
	if historicalCount == 1 {
		return store.loadUniqueHistoricalUse(ctx, root, action)
	}
	if historicalCount > 1 {
		return ProjectAttempt{kind: AttemptHistoricalUse, count: historicalCount}, nil
	}
	historyCount, err := store.countAuthorityHistory(ctx, root, action)
	if err != nil {
		return ProjectAttempt{}, err
	}
	if historyCount > 0 {
		return ProjectAttempt{kind: AttemptExpiredHistory, count: historyCount}, nil
	}
	return ProjectAttempt{kind: AttemptNone}, nil
}

func (store *Store) validateAttemptRead(
	ctx context.Context,
	action authority.ActionKind,
) error {
	if store == nil || store.database == nil || store.now == nil {
		return fmt.Errorf("profile authority SQLite store is not open")
	}
	if ctx == nil {
		return fmt.Errorf("profile authority recovery requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	expectedAction, err := profileauthority.ActionKind()
	if err != nil {
		return err
	}
	if action.String() != expectedAction.String() {
		return fmt.Errorf("unsupported profile authority action %q", action.String())
	}
	return requireV44(store.database)
}

func (store *Store) validateUsedAdmissionIntegrity(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
) error {
	count, err := queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authority_uses_v2 authority_use
		 LEFT JOIN project_profile_admissions_v2 admission
		 ON admission.admission_id = authority_use.committed_admission_ref
		 AND admission.admission_digest = authority_use.committed_admission_digest
		 AND admission.authority_basis_ref = authority_use.authority_basis_ref
		 AND admission.authority_basis_digest = authority_use.authority_basis_digest
		 AND admission.authority_resolution_ref = authority_use.authority_resolution_ref
		 AND admission.authority_resolution_digest = authority_use.authority_resolution_digest
		 WHERE authority_use.project_root = ?
		 AND authority_use.action_kind = ?
		 AND admission.admission_id IS NULL`,
		root.String(),
		action.String(),
	)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("durable profile authority use has no exact committed v2 admission")
	}
	return nil
}

func (store *Store) countLiveResolutions(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (int, error) {
	return queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authority_resolutions_v2 resolution
		 WHERE resolution.project_root = ? AND resolution.action_kind = ?
		 AND resolution.checked_at <= ?
		 AND resolution.permission_valid_from <= ? AND ? < resolution.permission_valid_until
		 AND NOT EXISTS (
			SELECT 1 FROM profile_declaration_authority_uses_v2 authority_use
			WHERE authority_use.authority_resolution_ref = resolution.authority_resolution_ref
		 )`,
		root.String(),
		action.String(),
		now,
		now,
		now,
	)
}

func (store *Store) countLiveRecordedSources(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (int, error) {
	return queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authorization_preparations_v2 prepared
		 JOIN profile_declaration_authorization_contents_v2 content
		 ON content.authorization_content_ref = prepared.authorization_content_ref
		 AND content.authorization_content_digest = prepared.authorization_content_digest
		 JOIN speech_acts act ON act.speech_act_ref = prepared.speech_act_ref
		 WHERE content.project_root = ? AND content.action_kind = ?
		 AND content.authorization_valid_from <= ? AND ? < content.authorization_valid_until
		 AND NOT EXISTS (
			SELECT 1 FROM profile_declaration_authority_resolutions_v2 resolution
			WHERE resolution.authority_basis_ref = prepared.basis_ref
		 )`,
		root.String(),
		action.String(),
		now,
		now,
	)
}

func (store *Store) countLiveInertPreparations(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (int, error) {
	return queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authorization_preparations_v2 prepared
		 JOIN profile_declaration_authorization_contents_v2 content
		 ON content.authorization_content_ref = prepared.authorization_content_ref
		 AND content.authorization_content_digest = prepared.authorization_content_digest
		 LEFT JOIN speech_acts act ON act.speech_act_ref = prepared.speech_act_ref
		 WHERE content.project_root = ? AND content.action_kind = ?
		 AND content.authorization_valid_from <= ? AND ? < content.authorization_valid_until
		 AND act.speech_act_ref IS NULL`,
		root.String(),
		action.String(),
		now,
		now,
	)
}

func (store *Store) countCommittedUses(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
) (int, error) {
	return queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authority_uses_v2 authority_use
		 JOIN project_profile_admissions_v2 admission
		 ON admission.admission_id = authority_use.committed_admission_ref
		 AND admission.admission_digest = authority_use.committed_admission_digest
		 WHERE authority_use.project_root = ? AND authority_use.action_kind = ?`,
		root.String(),
		action.String(),
	)
}

func (store *Store) countAuthorityHistory(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
) (int, error) {
	return queryCount(
		ctx,
		store.database,
		`SELECT COUNT(*)
		 FROM profile_declaration_authorization_preparations_v2 prepared
		 JOIN profile_declaration_authorization_contents_v2 content
		 ON content.authorization_content_ref = prepared.authorization_content_ref
		 AND content.authorization_content_digest = prepared.authorization_content_digest
		 WHERE content.project_root = ? AND content.action_kind = ?`,
		root.String(),
		action.String(),
	)
}

func (store *Store) loadUniqueLiveResolutionAttempt(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (ProjectAttempt, error) {
	row := resolutionRow{}
	err := store.database.QueryRowContext(
		ctx,
		selectResolutionSQL+` WHERE project_root = ? AND action_kind = ?
		 AND checked_at <= ?
		 AND permission_valid_from <= ? AND ? < permission_valid_until
		 AND NOT EXISTS (
			SELECT 1 FROM profile_declaration_authority_uses_v2 authority_use
			WHERE authority_use.authority_resolution_ref = profile_declaration_authority_resolutions_v2.authority_resolution_ref
		 )`,
		root.String(),
		action.String(),
		now,
		now,
		now,
	).Scan(row.scanTargets()...)
	if err != nil {
		return ProjectAttempt{}, fmt.Errorf("load unique live authority resolution: %w", err)
	}
	basisRef, basisDigest, err := resolutionBasis(row)
	if err != nil {
		return ProjectAttempt{}, err
	}
	snapshot, err := store.PrepareClosureSnapshot(ctx, basisRef, basisDigest)
	if err != nil {
		return ProjectAttempt{}, err
	}
	record, err := reconstructResolution(row, snapshot.closure)
	if err != nil {
		return ProjectAttempt{}, err
	}
	return ProjectAttempt{
		kind:       AttemptResolutionReady,
		count:      1,
		prepared:   snapshot.sources.prepared,
		recorded:   snapshot.sources.source,
		closure:    snapshot.closure,
		snapshot:   snapshot,
		resolution: record,
	}, nil
}

func (store *Store) loadUniqueLiveSourceAttempt(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (ProjectAttempt, error) {
	digestRaw := ""
	err := store.database.QueryRowContext(
		ctx,
		`SELECT prepared.prepared_authorization_digest
		 FROM profile_declaration_authorization_preparations_v2 prepared
		 JOIN profile_declaration_authorization_contents_v2 content
		 ON content.authorization_content_ref = prepared.authorization_content_ref
		 AND content.authorization_content_digest = prepared.authorization_content_digest
		 JOIN speech_acts act ON act.speech_act_ref = prepared.speech_act_ref
		 WHERE content.project_root = ? AND content.action_kind = ?
		 AND content.authorization_valid_from <= ? AND ? < content.authorization_valid_until
		 AND NOT EXISTS (
			SELECT 1 FROM profile_declaration_authority_resolutions_v2 resolution
			WHERE resolution.authority_basis_ref = prepared.basis_ref
		 )`,
		root.String(),
		action.String(),
		now,
		now,
	).Scan(&digestRaw)
	if err != nil {
		return ProjectAttempt{}, fmt.Errorf("load unique live prepared authorization: %w", err)
	}
	digest, err := authority.NewDigest(digestRaw)
	if err != nil {
		return ProjectAttempt{}, err
	}
	prepared, err := LoadPreparedAuthorization(ctx, store.database, digest)
	if err != nil {
		return ProjectAttempt{}, err
	}
	recorded, err := store.loadRecordedSourceForPrepared(ctx, prepared)
	if err != nil {
		return ProjectAttempt{}, err
	}
	basisRef, ok := prepared.BasisRef()
	if !ok {
		return ProjectAttempt{}, fmt.Errorf("live preparation omitted basis ref")
	}
	snapshot, snapshotErr := store.PrepareClosureSnapshotForBasis(ctx, basisRef)
	if snapshotErr == sql.ErrNoRows {
		return ProjectAttempt{
			kind:     AttemptPendingClosure,
			count:    1,
			prepared: prepared,
			recorded: recorded,
		}, nil
	}
	if snapshotErr != nil {
		return ProjectAttempt{}, snapshotErr
	}
	return ProjectAttempt{
		kind:     AttemptPendingResolution,
		count:    1,
		prepared: prepared,
		recorded: recorded,
		closure:  snapshot.closure,
		snapshot: snapshot,
	}, nil
}

func (store *Store) loadUniqueInertAttempt(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
	now string,
) (ProjectAttempt, error) {
	digestRaw := ""
	err := store.database.QueryRowContext(
		ctx,
		`SELECT prepared.prepared_authorization_digest
		 FROM profile_declaration_authorization_preparations_v2 prepared
		 JOIN profile_declaration_authorization_contents_v2 content
		 ON content.authorization_content_ref = prepared.authorization_content_ref
		 AND content.authorization_content_digest = prepared.authorization_content_digest
		 LEFT JOIN speech_acts act ON act.speech_act_ref = prepared.speech_act_ref
		 WHERE content.project_root = ? AND content.action_kind = ?
		 AND content.authorization_valid_from <= ? AND ? < content.authorization_valid_until
		 AND act.speech_act_ref IS NULL`,
		root.String(),
		action.String(),
		now,
		now,
	).Scan(&digestRaw)
	if err != nil {
		return ProjectAttempt{}, fmt.Errorf("load unique inert preparation: %w", err)
	}
	digest, err := authority.NewDigest(digestRaw)
	if err != nil {
		return ProjectAttempt{}, err
	}
	prepared, err := LoadPreparedAuthorization(ctx, store.database, digest)
	if err != nil {
		return ProjectAttempt{}, err
	}
	return ProjectAttempt{
		kind:     AttemptAwaitingCapture,
		count:    1,
		prepared: prepared,
	}, nil
}

func (store *Store) loadUniqueHistoricalUse(
	ctx context.Context,
	root authority.ProjectRoot,
	action authority.ActionKind,
) (ProjectAttempt, error) {
	refRaw := ""
	digestRaw := ""
	err := store.database.QueryRowContext(
		ctx,
		`SELECT authority_use.use_ref, authority_use.use_digest
		 FROM profile_declaration_authority_uses_v2 authority_use
		 JOIN project_profile_admissions_v2 admission
		 ON admission.admission_id = authority_use.committed_admission_ref
		 AND admission.admission_digest = authority_use.committed_admission_digest
		 WHERE authority_use.project_root = ? AND authority_use.action_kind = ?`,
		root.String(),
		action.String(),
	).Scan(&refRaw, &digestRaw)
	if err != nil {
		return ProjectAttempt{}, fmt.Errorf("load unique historical authority use: %w", err)
	}
	ref, err := profileauthority.NewProfileDeclarationAuthorityUseRef(refRaw)
	if err != nil {
		return ProjectAttempt{}, err
	}
	digest, err := authority.NewDigest(digestRaw)
	if err != nil {
		return ProjectAttempt{}, err
	}
	use, err := LoadAuthorityUseRecord(ctx, store.database, ref, digest)
	if err != nil {
		return ProjectAttempt{}, err
	}
	return ProjectAttempt{kind: AttemptHistoricalUse, count: 1, use: use}, nil
}

func (store *Store) loadRecordedSourceForPrepared(
	ctx context.Context,
	prepared profileauthority.PreparedAuthorization,
) (authority.RecordedSpeechActSource, error) {
	ref, ok := prepared.SpeechActRef()
	if !ok {
		return authority.RecordedSpeechActSource{}, fmt.Errorf("prepared authorization omitted SpeechAct ref")
	}
	recorded, found, err := authority.ResolveRecordedSpeechActSource(ctx, store.database, ref)
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	if !found {
		return authority.RecordedSpeechActSource{}, fmt.Errorf("live recorded-source frontier lost its SpeechAct source")
	}
	if err := profileauthority.ValidateRecordedSource(prepared, recorded); err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	return recorded, nil
}

func queryCount(
	ctx context.Context,
	database *sql.DB,
	statement string,
	arguments ...any,
) (int, error) {
	count := 0
	err := database.QueryRowContext(ctx, statement, arguments...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count profile authority recovery frontier: %w", err)
	}
	return count, nil
}
