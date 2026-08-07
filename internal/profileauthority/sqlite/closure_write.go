package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const insertPermissionSQL = `INSERT INTO profile_declaration_permissions_v2 (
	permission_ref, permission_digest, prepared_authorization_digest,
	project_root, permission_kind, subject_ref, subject_digest, modality,
	action_kind, claim_scope_ref, bounded_context_ref, valid_from, valid_until,
	referents_json, authorization_content_ref, authorization_content_digest,
	method_description_ref, method_description_digest, enactability_predicate_ref,
	adjudication_evidence_claim_refs_json,
	adjudication_carrier_class_refs_json, adjudication_verifier_identity,
	adjudication_verifier_version, adjudication_evaluation_policy_ref,
	adjudication_evaluation_policy_digest, source_speech_act_ref,
	source_speech_act_digest, context_policy_ref, context_policy_digest,
	capture_carrier_ref, capture_carrier_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertEffectSQL = `INSERT INTO profile_declaration_instituted_effects_v2 (
	effect_digest, project_root, speech_act_ref, speech_act_digest,
	permission_ref, permission_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

const insertBasisSQL = `INSERT INTO profile_declaration_authority_bases_v2 (
	basis_ref, basis_digest, project_root, speech_act_ref, speech_act_digest,
	authorization_content_ref, authorization_content_digest, permission_ref,
	permission_digest, context_policy_ref, context_policy_digest,
	instituted_effect_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

type ClosureWriteResult struct {
	kind    WriteKind
	closure profileauthority.Closure
	detail  string
}

func (result ClosureWriteResult) Kind() WriteKind {
	return result.kind
}

func (result ClosureWriteResult) Closure() (profileauthority.Closure, bool) {
	usable := result.kind == WriteStaged ||
		result.kind == WriteExactReplay ||
		result.kind == WriteRecovered
	if !usable {
		return profileauthority.Closure{}, false
	}
	_, ok := result.closure.Basis()
	return result.closure, ok
}

func (result ClosureWriteResult) Basis() (profileauthority.FourRefBasis, bool) {
	closure, ok := result.Closure()
	if !ok {
		return profileauthority.FourRefBasis{}, false
	}
	return closure.Basis()
}

func (result ClosureWriteResult) RejectionDetail() (string, bool) {
	return result.detail, result.kind == WriteRejected && result.detail != ""
}

// InstituteClosure starts a transaction separate from source capture. It
// re-resolves the immutable generic source and context-policy member, constructs
// the pure Permission/effect/four-ref closure, persists all three atomically,
// and performs exact rereads before and after commit.
func (store *Store) InstituteClosure(
	ctx context.Context,
	preparedDigest authority.Digest,
) (ClosureWriteResult, error) {
	if store == nil || store.database == nil || store.now == nil {
		return ClosureWriteResult{}, fmt.Errorf("profile authority SQLite store is not open")
	}
	if ctx == nil {
		return ClosureWriteResult{}, fmt.Errorf("profile closure institution requires a context")
	}
	prepared, err := LoadPreparedAuthorization(ctx, store.database, preparedDigest)
	if err != nil {
		return ClosureWriteResult{}, fmt.Errorf("load prepared profile authorization: %w", err)
	}
	speechActRef, ok := prepared.SpeechActRef()
	if !ok {
		return ClosureWriteResult{}, fmt.Errorf("prepared profile authorization omitted SpeechAct ref")
	}
	recorded, found, err := authority.ResolveRecordedSpeechActSource(
		ctx,
		store.database,
		speechActRef,
	)
	if err != nil {
		return ClosureWriteResult{}, fmt.Errorf("resolve durable generic SpeechAct source: %w", err)
	}
	if !found {
		return ClosureWriteResult{}, fmt.Errorf("durable generic SpeechAct source is unavailable")
	}
	if err := profileauthority.ValidateRecordedSource(prepared, recorded); err != nil {
		return ClosureWriteResult{}, err
	}
	policyRef, policyRefOK := recorded.ContextPolicyRef()
	policyDigest, policyDigestOK := recorded.ContextPolicyDigest()
	if !policyRefOK || !policyDigestOK {
		return ClosureWriteResult{}, fmt.Errorf("durable SpeechAct source omitted context policy")
	}
	policy, err := authority.LoadSpeechActContextPolicy(
		ctx,
		store.database,
		policyRef,
		policyDigest,
	)
	if err != nil {
		return ClosureWriteResult{}, err
	}
	if err := validatePreparedPolicy(prepared, policy); err != nil {
		return ClosureWriteResult{}, err
	}
	closure, err := profileauthority.NewClosure(prepared, recorded)
	if err != nil {
		return ClosureWriteResult{}, err
	}
	sources := exactClosureSources{
		prepared: prepared,
		source:   recorded,
		policy:   policy,
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.database)
	if err != nil {
		return ClosureWriteResult{}, fmt.Errorf("begin profile closure institution: %w", err)
	}
	result, err := store.instituteClosureInTransaction(
		ctx,
		transaction,
		preparedDigest,
		sources,
		closure,
	)
	if err != nil {
		finish := transaction.Rollback(context.Background())
		return ClosureWriteResult{}, errors.Join(err, finish.Err())
	}
	if result.kind == WriteRejected || result.kind == WriteExactReplay {
		finish := transaction.Rollback(ctx)
		if !finish.Succeeded() {
			return ClosureWriteResult{}, finish.Err()
		}
		return result, nil
	}
	basis, _ := closure.Basis()
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	finish := transaction.Commit(ctx)
	kind := WriteStaged
	if !finish.Succeeded() {
		kind = WriteRecovered
	}
	durable, loadErr := LoadClosure(
		context.Background(),
		store.database,
		basisRef,
		basisDigest,
	)
	if loadErr == nil {
		return ClosureWriteResult{kind: kind, closure: durable}, nil
	}
	return ClosureWriteResult{}, errors.Join(
		fmt.Errorf("profile closure commit outcome is unknown"),
		finish.Err(),
		loadErr,
	)
}

func (store *Store) instituteClosureInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	preparedDigest authority.Digest,
	sources exactClosureSources,
	closure profileauthority.Closure,
) (ClosureWriteResult, error) {
	if err := store.validateMutation(ctx, transaction); err != nil {
		return ClosureWriteResult{}, err
	}
	reloadedPrepared, err := loadPreparedExact(ctx, transaction, preparedDigest)
	if err != nil {
		return ClosureWriteResult{}, fmt.Errorf("reread prepared authorization in closure transaction: %w", err)
	}
	if !sameCanonicalPrepared(reloadedPrepared, sources.prepared) {
		return ClosureWriteResult{}, fmt.Errorf("prepared authorization changed before closure institution")
	}
	if err := assertSourceAddressInTransaction(ctx, transaction, sources.source); err != nil {
		return ClosureWriteResult{}, err
	}
	basis, _ := closure.Basis()
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	existingBasis, found, err := scanBasisByRef(ctx, transaction, basisRef.String())
	if err != nil {
		return ClosureWriteResult{}, err
	}
	if found {
		if existingBasis.digest != basisDigest.String() {
			return ClosureWriteResult{
				kind:   WriteRejected,
				detail: "four-ref basis identity collides with another digest",
			}, nil
		}
		durable, loadErr := loadClosureInTransaction(
			ctx,
			transaction,
			sources,
			basisRef,
			basisDigest,
		)
		if loadErr != nil {
			return ClosureWriteResult{}, loadErr
		}
		return ClosureWriteResult{kind: WriteExactReplay, closure: durable}, nil
	}
	partial, err := closurePartialFootprint(ctx, transaction, closure)
	if err != nil {
		return ClosureWriteResult{}, err
	}
	if partial {
		return ClosureWriteResult{
			kind:   WriteRejected,
			detail: "profile closure has a partial or colliding durable footprint",
		}, nil
	}
	if err := insertClosureRows(
		ctx,
		transaction,
		preparedDigest,
		closure,
		canonicalTime(store.now()),
	); err != nil {
		if isConstraint(err) {
			return ClosureWriteResult{
				kind:   WriteRejected,
				detail: err.Error(),
			}, nil
		}
		return ClosureWriteResult{}, err
	}
	durable, err := loadClosureInTransaction(
		ctx,
		transaction,
		sources,
		basisRef,
		basisDigest,
	)
	if err != nil {
		return ClosureWriteResult{}, fmt.Errorf("reread staged profile closure: %w", err)
	}
	if err := compareClosureMembersFromClosures(durable, closure); err != nil {
		return ClosureWriteResult{}, err
	}
	return ClosureWriteResult{kind: WriteStaged, closure: durable}, nil
}

func insertClosureRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	preparedDigest authority.Digest,
	closure profileauthority.Closure,
	recordedAt time.Time,
) error {
	permission, _ := closure.Permission()
	effect, _ := closure.Effect()
	basis, _ := closure.Basis()
	if err := insertPermission(ctx, transaction, preparedDigest, permission, recordedAt); err != nil {
		return err
	}
	if err := insertEffect(ctx, transaction, effect, recordedAt); err != nil {
		return err
	}
	return insertBasis(ctx, transaction, basis, effect, recordedAt)
}

func insertPermission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	preparedDigest authority.Digest,
	permission profileauthority.Permission,
	recordedAt time.Time,
) error {
	digest, _ := permission.Digest()
	canonical, _ := permission.CanonicalBytes()
	dto := permissionJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return fmt.Errorf("decode canonical profile Permission: %w", err)
	}
	referents, _ := json.Marshal(dto.Referents)
	evidence, _ := json.Marshal(dto.EvidenceClaimRefs)
	carriers, _ := json.Marshal(dto.CarrierClassRefs)
	arguments := []any{
		dto.PermissionRef,
		digest.String(),
		preparedDigest.String(),
		dto.ProjectRoot,
		"U.Commitment",
		dto.SubjectRef,
		dto.SubjectDigest,
		dto.Modality,
		dto.ActionKind,
		dto.ClaimScopeRef,
		dto.BoundedContextRef,
		dto.ValidFrom,
		dto.ValidUntil,
		string(referents),
		dto.AuthorizationContentRef,
		dto.AuthorizationContentDig,
		dto.MethodDescriptionRef,
		dto.MethodDescriptionDigest,
		dto.EnactabilityPredicateRef,
		string(evidence),
		string(carriers),
		dto.VerifierIdentity,
		dto.VerifierVersion,
		dto.EvaluationPolicyRef,
		dto.EvaluationPolicyDigest,
		dto.SourceSpeechActRef,
		dto.SourceSpeechActDigest,
		dto.ContextPolicyRef,
		dto.ContextPolicyDigest,
		dto.CaptureCarrierRef,
		dto.CaptureCarrierDigest,
		string(canonical),
		formatTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, insertPermissionSQL, arguments)
	return err
}

func insertEffect(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect profileauthority.InstitutedEffect,
	recordedAt time.Time,
) error {
	digest, _ := effect.Digest()
	canonical, _ := effect.CanonicalBytes()
	dto := effectJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return fmt.Errorf("decode canonical instituted effect: %w", err)
	}
	arguments := []any{
		digest.String(),
		dto.ProjectRoot,
		dto.SpeechActRef,
		dto.SpeechActDigest,
		dto.PermissionRef,
		dto.PermissionDigest,
		string(canonical),
		formatTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, insertEffectSQL, arguments)
	return err
}

func insertBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis profileauthority.FourRefBasis,
	effect profileauthority.InstitutedEffect,
	recordedAt time.Time,
) error {
	digest, _ := basis.Digest()
	canonical, _ := basis.CanonicalBytes()
	effectDigest, _ := effect.Digest()
	dto := basisJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return fmt.Errorf("decode canonical four-ref basis: %w", err)
	}
	arguments := []any{
		dto.BasisRef,
		digest.String(),
		dto.ProjectRoot,
		dto.SpeechActRef,
		dto.SpeechActDigest,
		dto.ContentRef,
		dto.ContentDigest,
		dto.PermissionRef,
		dto.PermissionDigest,
		dto.ContextPolicyRef,
		dto.ContextPolicyDigest,
		effectDigest.String(),
		string(canonical),
		formatTime(recordedAt),
	}
	_, err := transaction.Execute(ctx, insertBasisSQL, arguments)
	return err
}

func loadClosureInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	sources exactClosureSources,
	basisRef profileauthority.BasisRef,
	basisDigest authority.Digest,
) (profileauthority.Closure, error) {
	basisRow, found, err := scanBasisByRef(ctx, transaction, basisRef.String())
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if !found || basisRow.digest != basisDigest.String() {
		return profileauthority.Closure{}, fmt.Errorf("exact profile four-ref basis is unavailable")
	}
	permissionRow, found, err := scanPermissionByRef(ctx, transaction, basisRow.permissionRef)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if !found || permissionRow.digest != basisRow.permissionDigest {
		return profileauthority.Closure{}, fmt.Errorf("exact profile Permission is unavailable")
	}
	permission, err := reconstructPermission(permissionRow, sources)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	effectRow, found, err := scanEffectByDigest(
		ctx,
		transaction,
		basisRow.institutedEffectDigest,
	)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if !found {
		return profileauthority.Closure{}, fmt.Errorf("exact instituted profile effect is unavailable")
	}
	effect, err := reconstructEffect(effectRow, permission)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	basis, err := reconstructBasis(basisRow, sources, permission, effect)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	closure, err := profileauthority.NewClosure(sources.prepared, sources.source)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	if err := compareClosureMembers(closure, permission, effect, basis); err != nil {
		return profileauthority.Closure{}, err
	}
	return closure, nil
}

func closurePartialFootprint(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	closure profileauthority.Closure,
) (bool, error) {
	permission, _ := closure.Permission()
	permissionRef, _ := permission.Ref()
	_, permissionFound, err := scanPermissionByRef(ctx, transaction, permissionRef.String())
	if err != nil {
		return false, err
	}
	effect, _ := closure.Effect()
	effectDigest, _ := effect.Digest()
	_, effectFound, err := scanEffectByDigest(ctx, transaction, effectDigest.String())
	if err != nil {
		return false, err
	}
	return permissionFound || effectFound, nil
}

func assertSourceAddressInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	source authority.RecordedSpeechActSource,
) error {
	ref, refOK := source.SpeechActRef()
	digest, digestOK := source.SpeechActDigest()
	policyRef, policyRefOK := source.ContextPolicyRef()
	policyDigest, policyDigestOK := source.ContextPolicyDigest()
	if !refOK || !digestOK || !policyRefOK || !policyDigestOK {
		return fmt.Errorf("recorded SpeechAct source omitted exact address")
	}
	var count int
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM speech_acts act
		 JOIN speech_act_context_policies policy
		 ON policy.context_policy_ref = act.context_policy_ref
		 AND policy.context_policy_digest = act.context_policy_digest
		 WHERE act.speech_act_ref = ? AND act.speech_act_digest = ?
		 AND policy.context_policy_ref = ? AND policy.context_policy_digest = ?`,
		[]any{ref.String(), digest.String(), policyRef.String(), policyDigest.String()},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("reread source address in closure transaction: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("exact source and independent context policy are unavailable")
	}
	return nil
}

func sameCanonicalPrepared(
	left profileauthority.PreparedAuthorization,
	right profileauthority.PreparedAuthorization,
) bool {
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	leftBytes, leftBytesOK := left.CanonicalBytes()
	rightBytes, rightBytesOK := right.CanonicalBytes()
	return leftDigestOK && rightDigestOK && leftBytesOK && rightBytesOK &&
		leftDigest.String() == rightDigest.String() && slices.Equal(leftBytes, rightBytes)
}

func compareClosureMembersFromClosures(
	left profileauthority.Closure,
	right profileauthority.Closure,
) error {
	leftPermission, leftPermissionOK := left.Permission()
	rightPermission, rightPermissionOK := right.Permission()
	leftEffect, leftEffectOK := left.Effect()
	rightEffect, rightEffectOK := right.Effect()
	leftBasis, leftBasisOK := left.Basis()
	rightBasis, rightBasisOK := right.Basis()
	complete := leftPermissionOK && rightPermissionOK && leftEffectOK
	complete = complete && rightEffectOK && leftBasisOK && rightBasisOK
	exact := complete && sameCanonicalPermission(leftPermission, rightPermission)
	exact = exact && sameCanonicalEffect(leftEffect, rightEffect)
	exact = exact && sameCanonicalBasis(leftBasis, rightBasis)
	if !exact {
		return fmt.Errorf("reread profile closure differs from staged closure")
	}
	return nil
}
