package authority

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func loadRecordedAuthorityInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	presentationID PresentationID,
	authorityResolutionID AuthorityResolutionID,
) (recordedAuthority, bool, error) {
	record := authorityRecordRow{}
	err := transaction.ScanOne(
		ctx,
		loadAuthorityRecordSQL,
		[]any{presentationID.String(), authorityResolutionID.String()},
		record.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedAuthority{}, false, nil
	}
	if err != nil {
		return recordedAuthority{}, false, fmt.Errorf("load authority pair in transaction: %w", err)
	}
	presentation, resolution, err := parseAuthorityRecord(record)
	if err != nil {
		return recordedAuthority{}, true, fmt.Errorf("validate authority pair in transaction: %w", err)
	}
	recorded, err := newRecordedAuthority(presentation, resolution)
	if err != nil {
		return recordedAuthority{}, true, err
	}
	return recorded, true, nil
}

const insertCanonicalPresentationSQL = `
INSERT INTO authority_presentations (
	presentation_id, speech_act_ref, speech_act_digest,
	authorization_content_ref, authorization_content_digest,
	permission_ref, permission_digest, permission_modality,
	permission_source_speech_act_ref,
	permission_subject_role_assignment_ref,
	permission_authorization_content_ref,
	permission_action_kind, permission_project_root,
	permission_method_description_ref,
	permission_valid_from, permission_valid_until,
	permission_single_use_key, permission_profile_admission_predicate_ref,
	permission_context_policy_ref,
	context_policy_ref,
	context_policy_digest, action_kind, project_root,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, session_ref,
	allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until,
	valid_from, valid_until, single_use_key, project_binding_digest, envelope_digest,
	presentation_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func insertCanonicalPresentationWithPermissionWindow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	presentation canonicalPresentation,
	permissionWindow TimeWindow,
	recordedAt time.Time,
) error {
	envelope := presentation.envelope
	basis := presentation.basis
	envelopeDigest, err := envelope.Digest()
	if err != nil {
		return err
	}
	projectBindingDigest, err := ProjectBindingDigest(envelope.actionKind, envelope.projectRoot)
	if err != nil {
		return err
	}
	arguments := []any{
		presentation.id.String(),
		basis.speechActRef.String(),
		basis.speechActDigest.String(),
		basis.authorizationContentRef.String(),
		basis.authorizationContentDigest.String(),
		basis.permissionRef.String(),
		basis.permissionDigest.String(),
		permissionModalityMay,
		basis.speechActRef.String(),
		envelope.profileAuthor.String(),
		basis.authorizationContentRef.String(),
		envelope.actionKind.String(),
		envelope.projectRoot.String(),
		envelope.methodDescription.String(),
		formatAuthorityTime(permissionWindow.from),
		formatAuthorityTime(permissionWindow.until),
		envelope.singleUseKey.String(),
		basis.permissionPredicateRef.String(),
		basis.contextPolicyRef.String(),
		basis.contextPolicyRef.String(),
		basis.contextPolicyDigest.String(),
		envelope.actionKind.String(),
		envelope.projectRoot.String(),
		envelope.profileAuthor.String(),
		envelope.profileAuthorDigest.String(),
		envelope.methodDescription.String(),
		envelope.methodDescriptionDigest.String(),
		envelope.methodContract.String(),
		envelope.methodContractDigest.String(),
		envelope.classifierVersion.String(),
		envelope.policyVersion.String(),
		envelope.sessionRef.String(),
		formatAuthorityTime(envelope.allowedWorkWindow.from),
		formatAuthorityTime(envelope.allowedWorkWindow.until),
		formatAuthorityTime(envelope.allowedBasisObservation.from),
		formatAuthorityTime(envelope.allowedBasisObservation.until),
		formatAuthorityTime(envelope.authorizationValidityWindow.from),
		formatAuthorityTime(envelope.authorizationValidityWindow.until),
		envelope.singleUseKey.String(),
		projectBindingDigest.String(),
		envelopeDigest.String(),
		presentation.digest.String(),
		formatAuthorityTime(recordedAt),
	}
	_, err = transaction.Execute(ctx, insertCanonicalPresentationSQL, arguments)
	if err != nil {
		return fmt.Errorf("insert canonical authority presentation: %w", err)
	}
	return nil
}

const insertCanonicalAuthorityResolutionSQL = `
INSERT INTO authority_resolution_records (
	authority_resolution_id, presentation_id, presentation_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	resolved_at, valid_until, authority_resolution_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func insertCanonicalAuthorityResolution(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	resolution canonicalAuthorityResolution,
	recordedAt time.Time,
) error {
	arguments := []any{
		resolution.id.String(),
		resolution.presentationID.String(),
		resolution.presentationDigest.String(),
		resolution.profileAuthorRef.String(),
		resolution.profileAuthorDigest.String(),
		resolution.methodDescriptionRef.String(),
		resolution.methodDescriptionDigest.String(),
		resolution.methodContractRef.String(),
		resolution.methodContractDigest.String(),
		resolution.verifierIdentity.String(),
		resolution.verifierVersion.String(),
		resolution.verificationPolicyRef.String(),
		resolution.verificationPolicyDigest.String(),
		formatAuthorityTime(resolution.resolvedAt),
		formatAuthorityTime(resolution.validUntil),
		resolution.digest.String(),
		formatAuthorityTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, insertCanonicalAuthorityResolutionSQL, arguments)
	if err != nil {
		return fmt.Errorf("insert canonical authority resolution: %w", err)
	}
	return nil
}
