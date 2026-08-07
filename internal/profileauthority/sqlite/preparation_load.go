package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectContentSQL = `SELECT
	authorization_content_ref, authorization_content_digest, project_root,
	action_kind, profile_author_role_assignment_ref,
	profile_author_role_assignment_digest, method_description_ref,
	method_description_digest, method_contract_ref, method_contract_digest,
	classifier_version, policy_version, session_ref, allowed_work_from,
	allowed_work_until, basis_observation_from, basis_observation_until,
	authorization_valid_from, authorization_valid_until, single_use_key,
	canonical_json, recorded_at
FROM profile_declaration_authorization_contents_v2`

const selectPreparationSQL = `SELECT
	prepared_authorization_digest, project_root, authorization_content_ref,
	authorization_content_digest, permission_ref, speech_act_ref,
	capture_carrier_ref, speech_act_session_ref, claim_scope_ref,
	enactability_predicate_ref, evidence_claim_ref, carrier_class_ref,
	verifier_identity, verifier_version, verification_policy_ref,
	verification_policy_digest, basis_ref, context_policy_ref,
	context_policy_digest, speech_act_intent_digest, canonical_json, recorded_at
FROM profile_declaration_authorization_preparations_v2`

func LoadAuthorizationContent(
	ctx context.Context,
	database *sql.DB,
	ref authority.AuthorizationContentRef,
	digest authority.Digest,
) (profileauthority.AuthorizationContent, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	content, err := loadContentExact(ctx, transaction, ref, digest)
	return finishReadValue(ctx, transaction, content, err)
}

func LoadPreparedAuthorization(
	ctx context.Context,
	database *sql.DB,
	digest authority.Digest,
) (profileauthority.PreparedAuthorization, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	prepared, err := loadPreparedExact(ctx, transaction, digest)
	return finishReadValue(ctx, transaction, prepared, err)
}

func ResolvePreparedAuthorizationForSpeechAct(
	ctx context.Context,
	database *sql.DB,
	ref authority.SpeechActRef,
) (profileauthority.PreparedAuthorization, bool, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, false, err
	}
	row, found, err := scanPreparationForSpeechAct(ctx, transaction, ref)
	if err != nil || !found {
		finishErr := finishRead(ctx, transaction, err)
		return profileauthority.PreparedAuthorization{}, false, finishErr
	}
	prepared, err := reconstructPrepared(ctx, transaction, row)
	finishErr := finishRead(ctx, transaction, err)
	if finishErr != nil {
		return profileauthority.PreparedAuthorization{}, false, finishErr
	}
	return prepared, true, nil
}

func loadContentExact(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref authority.AuthorizationContentRef,
	digest authority.Digest,
) (profileauthority.AuthorizationContent, error) {
	row, found, err := scanContentByRef(ctx, transaction, ref.String())
	if err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	if !found {
		return profileauthority.AuthorizationContent{}, sql.ErrNoRows
	}
	if row.digest != digest.String() {
		return profileauthority.AuthorizationContent{}, fmt.Errorf(
			"profile authorization content digest differs from requested identity",
		)
	}
	return reconstructContent(row)
}

func loadPreparedExact(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	digest authority.Digest,
) (profileauthority.PreparedAuthorization, error) {
	row, found, err := scanPreparationByDigest(ctx, transaction, digest.String())
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	if !found {
		return profileauthority.PreparedAuthorization{}, sql.ErrNoRows
	}
	return reconstructPrepared(ctx, transaction, row)
}

func reconstructContent(
	row contentRow,
) (profileauthority.AuthorizationContent, error) {
	ref, err := authority.NewAuthorizationContentRef(row.ref)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse content ref: %w", err)
	}
	root, err := authority.NewProjectRoot(row.projectRoot)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse content project root: %w", err)
	}
	authorRef, err := authority.NewRoleAssignmentRef(row.profileAuthorRef)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse ProfileAuthor ref: %w", err)
	}
	authorDigest, err := authority.NewDigest(row.profileAuthorDigest)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse ProfileAuthor digest: %w", err)
	}
	methodRef, err := authority.NewMethodDescriptionRef(row.methodDescriptionRef)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse MethodDescription ref: %w", err)
	}
	methodDigest, err := authority.NewDigest(row.methodDescriptionDigest)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse MethodDescription digest: %w", err)
	}
	contractRef, err := authority.NewMethodContractRef(row.methodContractRef)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse MethodContract ref: %w", err)
	}
	contractDigest, err := authority.NewDigest(row.methodContractDigest)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse MethodContract digest: %w", err)
	}
	classifier, err := authority.NewClassifierVersion(row.classifierVersion)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse classifier version: %w", err)
	}
	policy, err := authority.NewPolicyVersion(row.policyVersion)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse policy version: %w", err)
	}
	session, err := authority.NewSessionRef(row.sessionRef)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse future Work session: %w", err)
	}
	workWindow, err := parseWindow(row.allowedWorkFrom, row.allowedWorkUntil)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse allowed Work window: %w", err)
	}
	basisWindow, err := parseWindow(row.basisObservationFrom, row.basisObservationUntil)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse basis observation window: %w", err)
	}
	validity, err := parseWindow(row.authorizationValidFrom, row.authorizationValidUntil)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse authorization validity: %w", err)
	}
	singleUse, err := authority.NewSingleUseKey(row.singleUseKey)
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("parse single-use key: %w", err)
	}
	content, err := profileauthority.NewAuthorizationContentBuilder(ref, root).
		ForProfileAuthor(authorRef, authorDigest).
		ForMethod(methodRef, methodDigest, contractRef, contractDigest).
		WithVersions(classifier, policy).
		InSession(session).
		AllowWorkWithin(workWindow).
		AllowBasisObservationWithin(basisWindow).
		ValidWithin(validity).
		SingleUse(singleUse).
		Build()
	if err != nil {
		return profileauthority.AuthorizationContent{}, fmt.Errorf("rebuild authorization content: %w", err)
	}
	if err := validateContentRow(row, content); err != nil {
		return profileauthority.AuthorizationContent{}, err
	}
	return content, nil
}

func reconstructPrepared(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row preparationRow,
) (profileauthority.PreparedAuthorization, error) {
	contentRef, err := authority.NewAuthorizationContentRef(row.contentRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse preparation content ref: %w", err)
	}
	contentDigest, err := authority.NewDigest(row.contentDigest)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse preparation content digest: %w", err)
	}
	content, err := loadContentExact(ctx, transaction, contentRef, contentDigest)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("load preparation content: %w", err)
	}
	permissionRef, err := authority.NewPermissionRef(row.permissionRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse preparation Permission ref: %w", err)
	}
	speechActRef, err := authority.NewSpeechActRef(row.speechActRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse preparation SpeechAct ref: %w", err)
	}
	captureRef, err := authority.NewCarrierRef(row.captureRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse preparation capture ref: %w", err)
	}
	sessionRef, err := authority.NewSessionRef(row.speechActSession)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse SpeechAct session: %w", err)
	}
	claimScope, err := authority.NewClaimScopeRef(row.claimScopeRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse claim scope: %w", err)
	}
	predicate, err := profileauthority.NewEnactabilityPredicateRef(row.enactabilityPredicateRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse admission predicate: %w", err)
	}
	evidence, err := profileauthority.NewEvidenceClaimRef(row.evidenceClaimRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse evidence claim: %w", err)
	}
	carrierClass, err := profileauthority.NewCarrierClassRef(row.carrierClassRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse carrier class: %w", err)
	}
	verifier, err := authority.NewVerifierIdentity(row.verifierIdentity)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse verifier identity: %w", err)
	}
	verifierVersion, err := authority.NewVerifierVersion(row.verifierVersion)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse verifier version: %w", err)
	}
	verificationPolicy, err := authority.NewVerificationPolicyRef(row.verificationPolicyRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse verification policy: %w", err)
	}
	verificationPolicyDigest, err := authority.NewDigest(row.verificationPolicyDigest)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse verification policy digest: %w", err)
	}
	basisRef, err := profileauthority.NewBasisRef(row.basisRef)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("parse four-ref basis ref: %w", err)
	}
	prepared, err := profileauthority.NewPreparedAuthorizationBuilder(
		content,
		permissionRef,
		speechActRef,
		captureRef,
	).
		InSpeechActSession(sessionRef).
		WithinClaimScope(claimScope).
		UnderEnactabilityPredicate(predicate).
		WithAdjudication(evidence, carrierClass).
		VerifiedBy(
			verifier,
			verifierVersion,
			verificationPolicy,
			verificationPolicyDigest,
		).
		AsBasis(basisRef).
		Build()
	if err != nil {
		return profileauthority.PreparedAuthorization{}, fmt.Errorf("rebuild prepared authorization: %w", err)
	}
	if err := validatePreparationRow(row, prepared); err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	return prepared, nil
}

func validateContentRow(
	row contentRow,
	content profileauthority.AuthorizationContent,
) error {
	digest, digestOK := content.Digest()
	canonical, canonicalOK := content.CanonicalBytes()
	action, actionErr := profileauthority.ActionKind()
	_, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && actionErr == nil && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	valid = valid && action.String() == row.actionKind
	if !valid {
		return fmt.Errorf("stored profile authorization content failed canonical rehash")
	}
	return nil
}

func validatePreparationRow(
	row preparationRow,
	prepared profileauthority.PreparedAuthorization,
) error {
	digest, digestOK := prepared.Digest()
	canonical, canonicalOK := prepared.CanonicalBytes()
	content, contentOK := prepared.Content()
	root, rootOK := content.ProjectRoot()
	policy, policyOK := prepared.ContextPolicy()
	policyRef, policyRefOK := policy.Ref()
	policyDigest, policyDigestOK := policy.Digest()
	intent, intentOK := prepared.SpeechActIntent()
	intentDigest, intentDigestOK := intent.Digest()
	_, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && contentOK && rootOK
	valid = valid && policyOK && policyRefOK && policyDigestOK
	valid = valid && intentOK && intentDigestOK && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	valid = valid && root.String() == row.projectRoot
	valid = valid && policyRef.String() == row.contextPolicyRef
	valid = valid && policyDigest.String() == row.contextPolicyDigest
	valid = valid && intentDigest.String() == row.speechActIntentDigest
	if !valid {
		return fmt.Errorf("stored prepared profile authorization failed canonical rehash")
	}
	return nil
}

func scanContentByRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (contentRow, bool, error) {
	row := contentRow{}
	err := transaction.ScanOne(
		ctx,
		selectContentSQL+" WHERE authorization_content_ref = ?",
		[]any{ref},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return contentRow{}, false, nil
	}
	if err != nil {
		return contentRow{}, false, fmt.Errorf("load profile authorization content: %w", err)
	}
	return row, true, nil
}

func scanPreparationByDigest(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	digest string,
) (preparationRow, bool, error) {
	row := preparationRow{}
	err := transaction.ScanOne(
		ctx,
		selectPreparationSQL+" WHERE prepared_authorization_digest = ?",
		[]any{digest},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return preparationRow{}, false, nil
	}
	if err != nil {
		return preparationRow{}, false, fmt.Errorf("load prepared profile authorization: %w", err)
	}
	return row, true, nil
}

func scanPreparationForSpeechAct(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref authority.SpeechActRef,
) (preparationRow, bool, error) {
	row := preparationRow{}
	err := transaction.ScanOne(
		ctx,
		selectPreparationSQL+" WHERE speech_act_ref = ?",
		[]any{ref.String()},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return preparationRow{}, false, nil
	}
	if err != nil {
		return preparationRow{}, false, fmt.Errorf("resolve preparation for SpeechAct: %w", err)
	}
	return row, true, nil
}

func beginRead(
	ctx context.Context,
	database *sql.DB,
) (*sqlitetransaction.Transaction, error) {
	if ctx == nil || database == nil {
		return nil, fmt.Errorf("profile authority read requires a context and database")
	}
	if err := requireV43(database); err != nil {
		return nil, err
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("begin profile authority read: %w", err)
	}
	return transaction, nil
}

func finishRead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	cause error,
) error {
	if cause != nil {
		finish := transaction.Rollback(context.Background())
		return errors.Join(cause, finish.Err())
	}
	finish := transaction.Commit(ctx)
	return finish.Err()
}

func finishReadValue[T any](
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	value T,
	cause error,
) (T, error) {
	err := finishRead(ctx, transaction, cause)
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

func parseWindow(from string, until string) (authority.TimeWindow, error) {
	fromTime, err := parseCanonicalTime(from)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	untilTime, err := parseCanonicalTime(until)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	return authority.NewTimeWindow(fromTime, untilTime)
}

func parseCanonicalTime(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, err
	}
	canonical := formatTime(value)
	if raw != canonical {
		return time.Time{}, fmt.Errorf("time is not canonical UTC RFC3339Nano")
	}
	return value, nil
}
