package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type AuthorityBasisWriteKind string

const (
	AuthorityBasisWriteStaged      AuthorityBasisWriteKind = "staged"
	AuthorityBasisWriteStored      AuthorityBasisWriteKind = "stored"
	AuthorityBasisWriteExactReplay AuthorityBasisWriteKind = "exact_replay"
	AuthorityBasisWriteRecovered   AuthorityBasisWriteKind = "recovered"
	AuthorityBasisWriteRejected    AuthorityBasisWriteKind = "rejected"
)

type AuthorityBasisWriteResult struct {
	kind     AuthorityBasisWriteKind
	recorded RecordedAuthorityBasis
	detail   string
}

func (result AuthorityBasisWriteResult) Kind() AuthorityBasisWriteKind { return result.kind }

func (result AuthorityBasisWriteResult) RecordedAuthorityBasis() (RecordedAuthorityBasis, bool) {
	success := result.kind == AuthorityBasisWriteStaged ||
		result.kind == AuthorityBasisWriteStored ||
		result.kind == AuthorityBasisWriteExactReplay ||
		result.kind == AuthorityBasisWriteRecovered
	return result.recorded, success && result.recorded.Valid()
}

func (result AuthorityBasisWriteResult) RejectionDetail() (string, bool) {
	return result.detail, result.kind == AuthorityBasisWriteRejected && result.detail != ""
}

// AuthorityBasisWriter persists the legacy profile-declaration authority DAG
// above the reusable SpeechActSourceWriter. Other domains must persist their
// own instituted effects and must not treat this writer as a generic gate.
type AuthorityBasisWriter struct {
	database     *sql.DB
	sourceWriter *SpeechActSourceWriter
	now          func() time.Time
}

func OpenAuthorityBasisWriter(database *sql.DB) (*AuthorityBasisWriter, error) {
	sourceWriter, err := OpenSpeechActSourceWriter(database)
	if err != nil {
		return nil, err
	}
	return &AuthorityBasisWriter{
		database:     database,
		sourceWriter: sourceWriter,
		now:          time.Now,
	}, nil
}

func (writer *AuthorityBasisWriter) Record(
	ctx context.Context,
	act VerifiedAuthorityAct,
) (AuthorityBasisWriteResult, error) {
	if err := validateAuthorityBasisWriter(writer, ctx); err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, writer.database)
	if err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	result, err := writer.RecordInTransaction(ctx, transaction, act)
	if err != nil || result.kind == AuthorityBasisWriteRejected || result.kind == AuthorityBasisWriteExactReplay {
		finish := transaction.Rollback(context.Background())
		return result, errors.Join(err, finish.Err())
	}
	finish := transaction.Commit(ctx)
	if finish.Err() != nil {
		return writer.classifyAuthorityBasisCommit(act, finish)
	}
	recorded, ok := result.RecordedAuthorityBasis()
	if !ok {
		return AuthorityBasisWriteResult{}, fmt.Errorf("committed authority basis omitted staged record")
	}
	return AuthorityBasisWriteResult{
		kind:     AuthorityBasisWriteStored,
		recorded: recorded,
	}, nil
}

func (writer *AuthorityBasisWriter) RecordInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	act VerifiedAuthorityAct,
) (AuthorityBasisWriteResult, error) {
	if err := validateAuthorityBasisWriter(writer, ctx); err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	if transaction == nil {
		return AuthorityBasisWriteResult{}, fmt.Errorf("authority basis write requires a transaction")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	if !act.valid() {
		return rejectedAuthorityBasis("authority act is not package-verified canonical material"), nil
	}
	existing, found, err := loadAuthorityBasisByReservedID(ctx, transaction, act)
	if err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	if found {
		if recordedAuthorityBasisMatchesAct(existing, act) {
			return AuthorityBasisWriteResult{
				kind:     AuthorityBasisWriteExactReplay,
				recorded: existing,
			}, nil
		}
		return rejectedAuthorityBasis("authority resolution identity already binds different canonical material"), nil
	}
	writer.sourceWriter.now = writer.now
	sourceResult, err := writer.sourceWriter.RecordInTransaction(ctx, transaction, act.state.source)
	if err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	recordedSource, ok := sourceResult.RecordedSource()
	if !ok {
		detail, _ := sourceResult.RejectionDetail()
		return rejectedAuthorityBasis(detail), nil
	}
	durableSource := VerifiedSpeechActSource(recordedSource)
	durableAct, err := completeVerifiedAuthorityAct(act.state.intent, durableSource)
	if err != nil {
		return AuthorityBasisWriteResult{}, fmt.Errorf("recompose authority act from durable source: %w", err)
	}
	checkedAt := canonicalAuthorityTime(writer.now())
	checked, err := newCheckedAuthorityBasis(durableAct, checkedAt)
	if err != nil {
		return rejectedAuthorityBasis(err.Error()), nil
	}
	recordedAt := canonicalAuthorityTime(writer.now())
	if recordedAt.Before(checkedAt) {
		return rejectedAuthorityBasis("authority recording clock precedes checkedAt"), nil
	}
	if err := insertCheckedAuthorityBasis(ctx, transaction, checked, recordedAt); err != nil {
		return AuthorityBasisWriteResult{}, err
	}
	recorded, found, err := loadRecordedAuthorityBasisInTransaction(
		ctx,
		transaction,
		checked.resolution.id,
		checked.resolution.digest,
	)
	if err != nil || !found {
		return AuthorityBasisWriteResult{}, errors.Join(
			fmt.Errorf("staged authority basis failed strict exact reread"),
			err,
		)
	}
	if !recordedAuthorityBasisMatchesChecked(recorded, checked) {
		return AuthorityBasisWriteResult{}, fmt.Errorf("staged authority basis differs from checked canonical graph")
	}
	return AuthorityBasisWriteResult{
		kind:     AuthorityBasisWriteStaged,
		recorded: recorded,
	}, nil
}

func validateAuthorityBasisWriter(writer *AuthorityBasisWriter, ctx context.Context) error {
	if writer == nil || writer.database == nil || writer.sourceWriter == nil || writer.now == nil {
		return fmt.Errorf("authority basis writer is not open")
	}
	if ctx == nil {
		return fmt.Errorf("authority basis writer requires a context")
	}
	return ctx.Err()
}

func rejectedAuthorityBasis(detail string) AuthorityBasisWriteResult {
	if detail == "" {
		detail = "authority basis source was rejected"
	}
	return AuthorityBasisWriteResult{kind: AuthorityBasisWriteRejected, detail: detail}
}

func (writer *AuthorityBasisWriter) classifyAuthorityBasisCommit(
	act VerifiedAuthorityAct,
	finish sqlitetransaction.FinishResult,
) (AuthorityBasisWriteResult, error) {
	id := act.state.intent.state.authorityResolutionID
	digest, found, err := loadAuthorityBasisDigestByID(context.Background(), writer.database, id)
	if err == nil && found {
		recorded, loadErr := LoadRecordedAuthorityBasisByResolution(
			context.Background(),
			writer.database,
			id,
			digest,
		)
		if loadErr == nil && recordedAuthorityBasisMatchesAct(recorded, act) {
			return AuthorityBasisWriteResult{
				kind:     AuthorityBasisWriteRecovered,
				recorded: recorded,
			}, nil
		}
	}
	return AuthorityBasisWriteResult{}, errors.Join(
		fmt.Errorf("authority basis commit outcome is unknown"),
		finish.Err(),
		err,
	)
}

func loadAuthorityBasisByReservedID(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	act VerifiedAuthorityAct,
) (RecordedAuthorityBasis, bool, error) {
	id := act.state.intent.state.authorityResolutionID
	var rawDigest string
	err := transaction.ScanOne(
		ctx,
		"SELECT authority_resolution_digest FROM authority_basis_resolutions WHERE authority_resolution_id = ?",
		[]any{id.String()},
		[]any{&rawDigest},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedAuthorityBasis{}, false, nil
	}
	if err != nil {
		return RecordedAuthorityBasis{}, false, err
	}
	digest, err := NewDigest(rawDigest)
	if err != nil {
		return RecordedAuthorityBasis{}, true, err
	}
	return loadRecordedAuthorityBasisInTransaction(ctx, transaction, id, digest)
}

func loadAuthorityBasisDigestByID(
	ctx context.Context,
	database *sql.DB,
	id AuthorityResolutionID,
) (Digest, bool, error) {
	var raw string
	err := database.QueryRowContext(
		ctx,
		"SELECT authority_resolution_digest FROM authority_basis_resolutions WHERE authority_resolution_id = ?",
		id.String(),
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Digest{}, false, nil
	}
	if err != nil {
		return Digest{}, false, err
	}
	digest, err := NewDigest(raw)
	return digest, true, err
}

func insertCheckedAuthorityBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	checked checkedAuthorityBasis,
	recordedAt time.Time,
) error {
	if err := insertProfileAuthorizationContent(ctx, transaction, checked.act, recordedAt); err != nil {
		return err
	}
	if err := insertProfileDeclarationPermission(ctx, transaction, checked.act, recordedAt); err != nil {
		return err
	}
	if err := insertSpeechActInstitutedEffect(ctx, transaction, checked.act, recordedAt); err != nil {
		return err
	}
	if err := insertAuthorityBasisPresentation(ctx, transaction, checked.presentation, recordedAt); err != nil {
		return err
	}
	if err := insertAuthorityBasisResolution(ctx, transaction, checked.resolution, recordedAt); err != nil {
		return err
	}
	if err := insertCanonicalPresentationWithPermissionWindow(
		ctx,
		transaction,
		checked.legacyPresentation,
		checked.act.state.permission.state.validityWindow,
		recordedAt,
	); err != nil {
		return err
	}
	return insertCanonicalAuthorityResolution(ctx, transaction, checked.legacyResolution, recordedAt)
}

func recordedAuthorityBasisMatchesAct(
	recorded RecordedAuthorityBasis,
	act VerifiedAuthorityAct,
) bool {
	if !recorded.Valid() || !act.valid() {
		return false
	}
	state := act.state
	return recorded.state.projectRoot == state.intent.state.content.state.envelope.projectRoot &&
		recorded.state.presentationID == state.intent.state.presentationID &&
		recorded.state.authorityResolutionID == state.intent.state.authorityResolutionID &&
		recorded.state.contentRef == state.intent.state.content.state.ref &&
		recorded.state.contentDigest == state.intent.state.content.state.digest &&
		recorded.state.permissionRef == state.permission.state.ref &&
		recorded.state.permissionDigest == state.permission.state.digest &&
		recorded.state.institutedEffectDigest == state.effect.state.digest &&
		recorded.state.source.state.speechAct.state.digest == state.speechAct.state.digest
}

func recordedAuthorityBasisMatchesChecked(
	recorded RecordedAuthorityBasis,
	checked checkedAuthorityBasis,
) bool {
	return recordedAuthorityBasisMatchesAct(recorded, checked.act) &&
		recorded.state.presentationDigest == checked.presentation.digest &&
		recorded.state.authorityResolutionDig == checked.resolution.digest
}

func insertProfileAuthorizationContent(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	act VerifiedAuthorityAct,
	recordedAt time.Time,
) error {
	content := act.state.intent.state.content.state
	envelope := content.envelope
	statement := `INSERT INTO profile_declaration_authorization_contents (
		authorization_content_ref, authorization_content_digest, project_root,
		action_kind, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref,
		allowed_work_from, allowed_work_until, basis_observation_from,
		basis_observation_until, authorization_valid_from,
		authorization_valid_until, single_use_key, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := []any{
		content.ref.String(), content.digest.String(), envelope.projectRoot.String(),
		envelope.actionKind.String(), envelope.profileAuthor.String(),
		envelope.profileAuthorDigest.String(), envelope.methodDescription.String(),
		envelope.methodDescriptionDigest.String(), envelope.methodContract.String(),
		envelope.methodContractDigest.String(), envelope.classifierVersion.String(),
		envelope.policyVersion.String(), envelope.sessionRef.String(),
		formatAuthorityTime(envelope.allowedWorkWindow.from),
		formatAuthorityTime(envelope.allowedWorkWindow.until),
		formatAuthorityTime(envelope.allowedBasisObservation.from),
		formatAuthorityTime(envelope.allowedBasisObservation.until),
		formatAuthorityTime(envelope.authorizationValidityWindow.from),
		formatAuthorityTime(envelope.authorizationValidityWindow.until),
		envelope.singleUseKey.String(), string(content.canonicalJSON),
		formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, statement, arguments)
	return err
}

func insertProfileDeclarationPermission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	act VerifiedAuthorityAct,
	recordedAt time.Time,
) error {
	permission := act.state.permission.state
	referents, _ := json.Marshal(permission.referents)
	evidenceClaims, _ := json.Marshal(permission.evidenceClaimRefs)
	carrierRefs, _ := json.Marshal(permission.carrierRefs)
	statement := `INSERT INTO profile_declaration_permissions (
		permission_ref, permission_digest, project_root, subject_ref,
		subject_digest, modality, claim_scope_ref, action_kind,
		bounded_context_ref, valid_from, valid_until,
		authorization_content_ref, authorization_content_digest,
		method_description_ref, method_description_digest,
		admission_predicate_ref, referents_json, source_speech_act_ref,
		context_policy_ref, context_policy_digest,
		adjudication_verifier_identity, adjudication_verifier_version,
		adjudication_evidence_claim_refs_json,
		adjudication_carrier_refs_json,
		adjudication_evaluation_policy_ref,
		adjudication_evaluation_policy_digest,
		capture_carrier_ref, capture_carrier_digest, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := []any{
		permission.ref.String(), permission.digest.String(), permission.projectRoot.String(),
		permission.subjectRef.String(), permission.subjectDigest.String(), permission.modality,
		permission.claimScopeRef.String(), permission.actionKind.String(),
		permission.boundedContextRef.String(), formatAuthorityTime(permission.validityWindow.from),
		formatAuthorityTime(permission.validityWindow.until),
		permission.authorizationContentRef.String(), permission.authorizationContentDig.String(),
		permission.methodDescriptionRef.String(), permission.methodDescriptionDigest.String(),
		permission.predicateRef.String(), string(referents), permission.sourceSpeechActRef.String(),
		permission.contextPolicyRef.String(), permission.contextPolicyDigest.String(),
		permission.verifierIdentity.String(), permission.verifierVersion.String(),
		string(evidenceClaims), string(carrierRefs),
		permission.verificationPolicyRef.String(), permission.verificationPolicyDig.String(),
		permission.captureCarrierRef.String(), permission.captureCarrierDigest.String(),
		string(permission.canonicalJSON), formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, statement, arguments)
	return err
}

func insertSpeechActInstitutedEffect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	act VerifiedAuthorityAct,
	recordedAt time.Time,
) error {
	effect := act.state.effect.state
	statement := `INSERT INTO speech_act_instituted_effects (
		instituted_effect_digest, project_root, speech_act_ref,
		speech_act_digest, permission_ref, permission_digest,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := []any{
		effect.digest.String(), effect.projectRoot.String(), effect.speechActRef.String(),
		effect.speechActDigest.String(), effect.permissionRef.String(),
		effect.permissionDigest.String(), string(effect.canonicalJSON),
		formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, statement, arguments)
	return err
}

func insertAuthorityBasisPresentation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	presentation canonicalAuthorityBasisPresentation,
	recordedAt time.Time,
) error {
	statement := `INSERT INTO authority_basis_presentations (
		presentation_id, presentation_digest, project_root,
		context_policy_ref, context_policy_digest,
		authorization_content_ref, authorization_content_digest,
		capture_carrier_ref, capture_carrier_digest,
		authorizer_ref, authorizer_digest, speech_act_ref, speech_act_digest,
		permission_ref, permission_digest, instituted_effect_digest,
		legacy_projection_digest, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := []any{
		presentation.id.String(), presentation.digest.String(), presentation.projectRoot.String(),
		presentation.contextPolicyRef.String(), presentation.contextPolicyDigest.String(),
		presentation.contentRef.String(), presentation.contentDigest.String(),
		presentation.captureRef.String(), presentation.captureDigest.String(),
		presentation.authorizerRef.String(), presentation.authorizerDigest.String(),
		presentation.speechActRef.String(), presentation.speechActDigest.String(),
		presentation.permissionRef.String(), presentation.permissionDigest.String(),
		presentation.institutedEffectDigest.String(), presentation.legacyProjectionDigest.String(),
		string(presentation.canonicalJSON), formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, statement, arguments)
	return err
}

func insertAuthorityBasisResolution(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	resolution canonicalAuthorityBasisResolution,
	recordedAt time.Time,
) error {
	statement := `INSERT INTO authority_basis_resolutions (
		authority_resolution_id, authority_resolution_digest, project_root,
		presentation_id, presentation_digest, verifier_identity, verifier_version,
		verification_policy_ref, verification_policy_digest,
		resolved_at, authority_valid_from, valid_until,
		legacy_projection_digest, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := []any{
		resolution.id.String(), resolution.digest.String(), resolution.projectRoot.String(),
		resolution.presentationID.String(), resolution.presentationDigest.String(),
		resolution.verifierIdentity.String(), resolution.verifierVersion.String(),
		resolution.verificationPolicyRef.String(), resolution.verificationPolicyDigest.String(),
		formatAuthorityTime(resolution.resolvedAt), formatAuthorityTime(resolution.authorityValidFrom),
		formatAuthorityTime(resolution.validUntil),
		resolution.legacyProjectionDigest.String(), string(resolution.canonicalJSON),
		formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, statement, arguments)
	return err
}
