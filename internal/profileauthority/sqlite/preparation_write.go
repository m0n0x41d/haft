package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

type mutationKind uint8

const (
	mutationStaged mutationKind = iota + 1
	mutationExact
	mutationRejected
)

const insertContentSQL = `INSERT INTO profile_declaration_authorization_contents_v2 (
	authorization_content_ref, authorization_content_digest, project_root,
	action_kind, profile_author_role_assignment_ref,
	profile_author_role_assignment_digest, method_description_ref,
	method_description_digest, method_contract_ref, method_contract_digest,
	classifier_version, policy_version, session_ref, allowed_work_from,
	allowed_work_until, basis_observation_from, basis_observation_until,
	authorization_valid_from, authorization_valid_until, single_use_key,
	canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertPreparationSQL = `INSERT INTO profile_declaration_authorization_preparations_v2 (
	prepared_authorization_digest, project_root, authorization_content_ref,
	authorization_content_digest, permission_ref, speech_act_ref,
	capture_carrier_ref, speech_act_session_ref, claim_scope_ref,
	enactability_predicate_ref, evidence_claim_ref, carrier_class_ref,
	verifier_identity, verifier_version, verification_policy_ref,
	verification_policy_digest, basis_ref, context_policy_ref,
	context_policy_digest, speech_act_intent_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func persistPreparation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared profileauthority.PreparedAuthorization,
	recordedAt time.Time,
) (mutationKind, error) {
	contentKind, err := persistContent(ctx, transaction, prepared, recordedAt)
	if err != nil || contentKind == mutationRejected {
		return contentKind, err
	}
	preparedKind, err := persistPreparedRow(ctx, transaction, prepared, recordedAt)
	if err != nil || preparedKind == mutationRejected {
		return preparedKind, err
	}
	if contentKind == mutationExact && preparedKind == mutationExact {
		return mutationExact, nil
	}
	return mutationStaged, nil
}

func persistContent(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared profileauthority.PreparedAuthorization,
	recordedAt time.Time,
) (mutationKind, error) {
	content, ok := prepared.Content()
	if !ok {
		return 0, fmt.Errorf("prepared authorization omitted canonical content")
	}
	ref, _ := content.Ref()
	digest, _ := content.Digest()
	canonical, _ := content.CanonicalBytes()
	existingRow, found, err := scanContentByRef(ctx, transaction, ref.String())
	if err != nil {
		return 0, err
	}
	if found {
		existing, rebuildErr := reconstructContent(existingRow)
		if rebuildErr != nil {
			return 0, rebuildErr
		}
		existingDigest, _ := existing.Digest()
		existingCanonical, _ := existing.CanonicalBytes()
		exact := existingDigest.String() == digest.String()
		exact = exact && slices.Equal(existingCanonical, canonical)
		if exact {
			return mutationExact, nil
		}
		return mutationRejected, nil
	}
	dto := authorizationContentJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return 0, fmt.Errorf("decode canonical authorization content: %w", err)
	}
	arguments := []any{
		dto.Ref,
		digest.String(),
		dto.ProjectRoot,
		dto.ActionKind,
		dto.ProfileAuthorRef,
		dto.ProfileAuthorDigest,
		dto.MethodDescriptionRef,
		dto.MethodDescriptionDigest,
		dto.MethodContractRef,
		dto.MethodContractDigest,
		dto.ClassifierVersion,
		dto.PolicyVersion,
		dto.SessionRef,
		dto.AllowedWorkFrom,
		dto.AllowedWorkUntil,
		dto.BasisObservationFrom,
		dto.BasisObservationUntil,
		dto.AuthorizationValidFrom,
		dto.AuthorizationValidUntil,
		dto.SingleUseKey,
		string(canonical),
		formatTime(recordedAt),
	}
	_, err = transaction.Execute(ctx, insertContentSQL, arguments)
	if isConstraint(err) {
		return mutationRejected, nil
	}
	if err != nil {
		return 0, fmt.Errorf("insert profile authorization content: %w", err)
	}
	loaded, err := loadContentExact(ctx, transaction, ref, digest)
	if err != nil {
		return 0, fmt.Errorf("reread staged profile authorization content: %w", err)
	}
	loadedCanonical, _ := loaded.CanonicalBytes()
	if !slices.Equal(loadedCanonical, canonical) {
		return 0, fmt.Errorf("staged profile authorization content changed during persistence")
	}
	return mutationStaged, nil
}

func persistPreparedRow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared profileauthority.PreparedAuthorization,
	recordedAt time.Time,
) (mutationKind, error) {
	digest, ok := prepared.Digest()
	if !ok {
		return 0, fmt.Errorf("prepared authorization omitted canonical digest")
	}
	canonical, _ := prepared.CanonicalBytes()
	existingRow, found, err := scanPreparationByDigest(ctx, transaction, digest.String())
	if err != nil {
		return 0, err
	}
	if found {
		existing, rebuildErr := reconstructPrepared(ctx, transaction, existingRow)
		if rebuildErr != nil {
			return 0, rebuildErr
		}
		existingCanonical, _ := existing.CanonicalBytes()
		if slices.Equal(existingCanonical, canonical) {
			return mutationExact, nil
		}
		return mutationRejected, nil
	}
	dto := preparedAuthorizationJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return 0, fmt.Errorf("decode canonical prepared authorization: %w", err)
	}
	content, _ := prepared.Content()
	root, _ := content.ProjectRoot()
	arguments := []any{
		digest.String(),
		root.String(),
		dto.ContentRef,
		dto.ContentDigest,
		dto.PermissionRef,
		dto.SpeechActRef,
		dto.CaptureRef,
		dto.SpeechActSession,
		dto.ClaimScopeRef,
		dto.EnactabilityPredicateRef,
		dto.EvidenceClaimRef,
		dto.CarrierClassRef,
		dto.VerifierIdentity,
		dto.VerifierVersion,
		dto.VerificationPolicyRef,
		dto.VerificationPolicyDigest,
		dto.BasisRef,
		dto.ContextPolicyRef,
		dto.ContextPolicyDigest,
		dto.SpeechActIntentDigest,
		string(canonical),
		formatTime(recordedAt),
	}
	_, err = transaction.Execute(ctx, insertPreparationSQL, arguments)
	if isConstraint(err) {
		return mutationRejected, nil
	}
	if err != nil {
		return 0, fmt.Errorf("insert prepared profile authorization: %w", err)
	}
	loaded, err := loadPreparedExact(ctx, transaction, digest)
	if err != nil {
		return 0, fmt.Errorf("reread staged prepared authorization: %w", err)
	}
	loadedCanonical, _ := loaded.CanonicalBytes()
	if !slices.Equal(loadedCanonical, canonical) {
		return 0, fmt.Errorf("staged prepared authorization changed during persistence")
	}
	return mutationStaged, nil
}

func isConstraint(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlitedriver.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code()&0xff == sqlitelib.SQLITE_CONSTRAINT
}
