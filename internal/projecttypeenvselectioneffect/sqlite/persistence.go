package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func (service *GenesisService) writeGenesisEffectTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	extensions, err := sealOrderedExtensionCoordinates(
		effect.prepared.request.Target().OrderedExtensions(),
	)
	if err != nil {
		return err
	}
	if err := writeGenesisConfigBasisTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisModePolicyTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisProofTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisRequestTx(
		ctx,
		transaction,
		effect,
		extensions,
	); err != nil {
		return err
	}
	if err := writeGenesisAuthorizationContentTx(
		ctx,
		transaction,
		effect,
	); err != nil {
		return err
	}
	if err := writeGenesisAuthoritySourceTx(
		ctx,
		transaction,
		effect,
	); err != nil {
		return err
	}
	if err := writeGenesisAuthorityResolutionTx(
		ctx,
		transaction,
		effect,
	); err != nil {
		return err
	}
	if err := writeGenesisAuthorityUseTx(
		ctx,
		transaction,
		effect,
		extensions,
	); err != nil {
		return err
	}
	if err := writeGenesisCASWorkTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := service.heads.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		effect.activation.SuccessorHead(),
	); err != nil {
		return fmt.Errorf("compare-and-swap Genesis ProjectTypeEnv head: %w", err)
	}
	if err := writeGenesisActivationTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisHeadHistoryTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisReceiptTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisClosureTx(ctx, transaction, effect); err != nil {
		return err
	}
	return nil
}

func (service *TransitionService) writeTransitionEffectTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	if service == nil || service.core == nil {
		return fmt.Errorf("transition persistence service is invalid")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	extensions, err := sealOrderedExtensionCoordinates(
		effect.prepared.request.Target().OrderedExtensions(),
	)
	if err != nil {
		return err
	}
	if err := writeGenesisConfigBasisTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisModePolicyTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisRequestTx(ctx, transaction, effect, extensions); err != nil {
		return err
	}
	if err := writeGenesisAuthorizationContentTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisAuthoritySourceTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisAuthorityResolutionTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisAuthorityUseTx(ctx, transaction, effect, extensions); err != nil {
		return err
	}
	if err := writeGenesisCASWorkTx(ctx, transaction, effect); err != nil {
		return err
	}
	predecessor, ok := effect.prepared.request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return fmt.Errorf("transition persistence received a non-transition request")
	}
	prior, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           predecessor.Project(),
			SelectedComposite: predecessor.SelectedComposite(),
			Revision:          predecessor.HeadRevision(),
		},
	)
	if err != nil {
		return err
	}
	if err := service.core.heads.CompareAndSwapTransitionProjectTypeEnvHeadTx(
		ctx,
		transaction,
		prior,
		effect.activation.SuccessorHead(),
	); err != nil {
		return fmt.Errorf("compare-and-swap Transition ProjectTypeEnv head: %w", err)
	}
	if err := writeGenesisActivationTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisHeadHistoryTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisReceiptTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisClosureTx(ctx, transaction, effect); err != nil {
		return err
	}
	return nil
}

type orderedExtensionCoordinates struct {
	digest    typedmemory.SHA256Digest
	canonical []byte
}

type storedHeadSelectionPredecessor struct {
	kind              string
	headRef           any
	headRevision      any
	selectedComposite any
}

func storedPredecessorCoordinates(
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) (storedHeadSelectionPredecessor, error) {
	switch exact := predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		return storedHeadSelectionPredecessor{kind: "genesis"}, nil
	case projecttypeenvselection.TransitionStagePredecessor:
		headRevision, err := exactSQLiteInteger(
			"predecessor head revision",
			exact.HeadRevision().Value(),
		)
		if err != nil {
			return storedHeadSelectionPredecessor{}, err
		}
		return storedHeadSelectionPredecessor{
			kind:              "transition",
			headRef:           exact.Head().String(),
			headRevision:      headRevision,
			selectedComposite: exact.SelectedComposite().String(),
		}, nil
	default:
		return storedHeadSelectionPredecessor{},
			fmt.Errorf("head-selection predecessor variant is invalid")
	}
}

func storedGenesisProofCoordinates(
	effect sealedGenesisEffect,
) (any, any, error) {
	switch effect.prepared.request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		if err := effect.prepared.proof.Verify(); err != nil {
			return nil, nil, err
		}
		return effect.prepared.proof.Ref().String(),
			effect.prepared.proof.Digest().String(),
			nil
	case projecttypeenvselection.TransitionStagePredecessor:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("head-selection predecessor variant is invalid")
	}
}

func sealOrderedExtensionCoordinates(
	extensions []typedmemory.TypeEnvExtensionRef,
) (orderedExtensionCoordinates, error) {
	values := make([]string, len(extensions))
	for index := range extensions {
		values[index] = extensions[index].String()
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return orderedExtensionCoordinates{}, fmt.Errorf(
			"encode ordered TypeEnv extensions: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return orderedExtensionCoordinates{}, err
	}
	return orderedExtensionCoordinates{
		digest:    digest,
		canonical: canonical,
	}, nil
}

func writeGenesisConfigBasisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	basis := effect.prepared.resolved.policy.ConfigBasis()
	exact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_config_authority_bases",
		"config_authority_basis_ref",
		"config_authority_basis_digest",
		"canonical_bytes",
		basis.Ref().String(),
		basis.Digest().String(),
		basis.CanonicalJSON(),
	)
	if err != nil || exact {
		return err
	}
	carrier := basis.ConfigCarrier()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		basis.Ref().String(),
		basis.Digest().String(),
		basis.Project().String(),
		basis.Mode().String(),
		carrier.Ref().String(),
		carrier.Digest().String(),
		basis.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisModePolicyTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	policy := effect.prepared.resolved.policy
	exact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_mode_policies",
		"mode_policy_ref",
		"mode_policy_digest",
		"canonical_bytes",
		policy.Ref().String(),
		policy.Digest().String(),
		policy.CanonicalJSON(),
	)
	if err != nil || exact {
		return err
	}
	strict, strictOK := policy.StrictCLISpeechAct()
	if strictOK {
		resolver := strict.ResolverPolicy()
		return executeGenesisStatement(
			ctx,
			transaction,
			`INSERT OR IGNORE INTO project_typeenv_head_selection_mode_policies (
				mode_policy_ref,
				mode_policy_digest,
				project_id,
				authority_mode,
				config_authority_basis_ref,
				config_authority_basis_digest,
				resolver_policy_ref,
				resolver_policy_edition,
				resolver_policy_digest,
				canonical_bytes,
				recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			policy.Ref().String(),
			policy.Digest().String(),
			policy.Project().String(),
			policy.Mode().String(),
			policy.ConfigBasis().Ref().String(),
			policy.ConfigBasis().Digest().String(),
			resolver.Ref().String(),
			resolver.Edition().String(),
			resolver.Digest().String(),
			policy.CanonicalJSON(),
			canonicalGenesisTime(effect.recordedAt),
		)
	}
	if _, explicitOK := policy.ExplicitHDecide(); !explicitOK {
		return fmt.Errorf("genesis mode policy variant is invalid")
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_mode_policies (
			mode_policy_ref,
			mode_policy_digest,
			project_id,
			authority_mode,
			config_authority_basis_ref,
			config_authority_basis_digest,
			resolver_policy_ref,
			resolver_policy_edition,
			resolver_policy_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?)`,
		policy.Ref().String(),
		policy.Digest().String(),
		policy.Project().String(),
		policy.Mode().String(),
		policy.ConfigBasis().Ref().String(),
		policy.ConfigBasis().Digest().String(),
		policy.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisProofTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	proof := effect.prepared.proof
	graphRevision, err := exactSQLiteInteger(
		"no-prior-head proof graph revision",
		proof.GraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_no_prior_head_proofs (
			proof_ref,
			proof_digest,
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision,
			canonical_bytes,
			observation_schema,
			observed_at,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proof.Ref().String(),
		proof.Ref().Digest().String(),
		proof.Project().String(),
		proof.Head().String(),
		proof.GraphSnapshotBasis().String(),
		proof.GraphSnapshotBasisDigest().String(),
		graphRevision,
		proof.CanonicalBytes(),
		"effect_owned_head_absence_v1",
		canonicalGenesisTime(proof.ObservedAt()),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisRequestTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	extensions orderedExtensionCoordinates,
) error {
	request := effect.prepared.request
	exact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_requests",
		"request_ref",
		"request_digest",
		"canonical_bytes",
		request.Ref().String(),
		request.Ref().Digest().String(),
		request.CanonicalBytes(),
	)
	if err != nil || exact {
		return err
	}
	target := request.Target()
	predecessor, err := storedPredecessorCoordinates(request.Predecessor())
	if err != nil {
		return err
	}
	head, err := request.Head()
	if err != nil {
		return err
	}
	expectedGraphRevision, err := exactSQLiteInteger(
		"head-selection request expected graph revision",
		request.ExpectedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_requests (
			request_ref,
			request_digest,
			project_id,
			head_ref,
			predecessor_kind,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			prior_head_ref,
			prior_head_revision,
			prior_selected_composite_ref,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			original_idempotency_key,
			request_schema,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, ?, NULL, NULL,
			?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		request.Ref().String(),
		request.Ref().Digest().String(),
		request.Project().String(),
		head.String(),
		predecessor.kind,
		predecessor.headRef,
		predecessor.headRevision,
		predecessor.selectedComposite,
		target.Base().String(),
		extensions.digest.String(),
		extensions.canonical,
		target.RuntimeBasis().String(),
		target.VerifiedComposite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		expectedGraphRevision,
		request.IdempotencyKey().String(),
		"haft.project-typeenv.head-selection-request.v2",
		request.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisAuthorizationContentTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	content := effect.prepared.resolved.coordinates.ContentRef()
	reviewed := effect.prepared.resolvedContent()
	exact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_authorization_contents",
		"content_ref",
		"content_digest",
		"canonical_bytes",
		content.String(),
		reviewed.Digest().String(),
		reviewed.CanonicalJSON(),
	)
	if err != nil || exact {
		return err
	}
	window := reviewed.ValidityWindow()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authorization_contents (
			content_ref,
			content_ref_kind,
			content_digest,
			project_id,
			request_ref,
			request_digest,
			judgement_context_ref,
			action_kind,
			valid_from,
			valid_until,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		content.String(),
		string(content.Kind()),
		reviewed.Digest().String(),
		reviewed.Project().String(),
		reviewed.Request().Ref().String(),
		reviewed.Request().Ref().Digest().String(),
		reviewed.JudgementContext().String(),
		reviewed.Action().String(),
		canonicalGenesisTime(window.From()),
		canonicalGenesisTime(window.Until()),
		reviewed.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisAuthoritySourceTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	source := effect.prepared.resolved.source
	if explicit, ok := source.TrustedDedicatedCLIInvocation(); ok {
		return writeGenesisTrustedCLISourceTx(
			ctx,
			transaction,
			effect,
			explicit,
		)
	}
	if strict, ok := source.VerifiedSpeechAct(); ok {
		return writeGenesisStrictSpeechActSourceTx(
			ctx,
			transaction,
			effect,
			strict,
		)
	}
	return fmt.Errorf("genesis authority source variant is invalid")
}

func writeGenesisTrustedCLISourceTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	source projecttypeenvselectionauthority.TrustedDedicatedCLIInvocationSourceRecord,
) error {
	policy := source.Policy()
	content := source.Content()
	request := source.Request()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_trusted_cli_sources (
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			project_id,
			mode_policy_ref,
			mode_policy_digest,
			config_authority_basis_ref,
			config_authority_basis_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.Ref().String(),
		source.Digest().String(),
		request.Project().String(),
		policy.Ref().String(),
		policy.Digest().String(),
		policy.ConfigBasis().Ref().String(),
		policy.ConfigBasis().Digest().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		source.CanonicalJSON(),
		canonicalGenesisTime(source.RecordedAt()),
	)
}

func writeGenesisStrictSpeechActSourceTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	source projecttypeenvselectionauthority.VerifiedSpeechActAuthoritySourceRecord,
) error {
	record := source.Record()
	permission := record.PermissionRecord()
	recordExact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_speech_act_records",
		"speech_act_record_ref",
		"speech_act_record_digest",
		"canonical_bytes",
		record.Ref().String(),
		record.Digest().String(),
		record.CanonicalJSON(),
	)
	if err != nil {
		return err
	}
	permissionExact, err := exactGenesisCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_permissions_v3",
		"permission_ref",
		"permission_digest",
		"canonical_bytes",
		permission.Ref().String(),
		permission.Digest().String(),
		permission.CanonicalJSON(),
	)
	if err != nil {
		return err
	}
	if recordExact != permissionExact {
		return fmt.Errorf(
			"durable strict SpeechAct source is partial: record exact=%t permission exact=%t",
			recordExact,
			permissionExact,
		)
	}
	if recordExact {
		return nil
	}
	sourceCoordinates, err := strictSpeechActSourceCoordinates(record.Source())
	if err != nil {
		return err
	}
	content := record.Content()
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_speech_act_records (
			speech_act_record_ref,
			speech_act_record_digest,
			project_id,
			speech_act_ref,
			human_work_ref,
			source_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			permission_ref,
			permission_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Ref().String(),
		record.Digest().String(),
		content.Project().String(),
		sourceCoordinates.speechAct.String(),
		sourceCoordinates.work.String(),
		sourceCoordinates.digest.String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		content.Request().Ref().String(),
		content.Request().Ref().Digest().String(),
		permission.Ref().String(),
		permission.Digest().String(),
		record.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	); err != nil {
		return err
	}
	return writeGenesisPermissionTx(
		ctx,
		transaction,
		record,
		permission,
	)
}

type strictSpeechActSourceCoordinateSet struct {
	speechAct authority.SpeechActRef
	work      authority.WorkRef
	digest    authority.Digest
}

func strictSpeechActSourceCoordinates(
	source authority.VerifiedSpeechActSourceV2,
) (strictSpeechActSourceCoordinateSet, error) {
	speechAct, speechActOK := source.SpeechActRef()
	work, workOK := source.WorkRef()
	digest, digestOK := source.Digest()
	if !speechActOK || !workOK || !digestOK {
		return strictSpeechActSourceCoordinateSet{},
			fmt.Errorf("strict SpeechAct source coordinates are unavailable")
	}
	return strictSpeechActSourceCoordinateSet{
		speechAct: speechAct,
		work:      work,
		digest:    digest,
	}, nil
}

func writeGenesisPermissionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecord,
	permission projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionRecord,
) error {
	subject := permission.Subject()
	subjectPolicy := subject.AssignmentPolicy()
	subjectWindow := subject.AssignmentWindow()
	scope := permission.Scope()
	referents, err := canonicalPermissionReferents(permission.Referents())
	if err != nil {
		return err
	}
	content := record.Content()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_permissions_v3 (
			permission_ref,
			permission_digest,
			project_id,
			subject_role_assignment_ref,
			subject_role_assignment_digest,
			subject_schema,
			subject_holder_system_ref,
			subject_holder_kind,
			subject_role_ref,
			subject_context_ref,
			subject_assignment_from,
			subject_assignment_until,
			subject_assignment_policy_ref,
			subject_assignment_policy_digest,
			subject_assignment_policy_edition_ref,
			subject_assignment_policy_selection,
			subject_system_admission_ref,
			subject_system_admission_digest,
			subject_role_admission_ref,
			subject_role_admission_digest,
			subject_assignment_justification_ref,
			subject_assignment_justification_digest,
			subject_assignment_provenance_ref,
			subject_assignment_provenance_digest,
			subject_authorization_description_kind,
			subject_authorization_description_ref,
			subject_authorization_content_digest,
			subject_canonical_bytes,
			modality,
			claim_scope_ref,
			claim_scope_digest,
			context_policy_ref,
			context_policy_digest,
			referents_canonical_bytes,
			effective_from,
			validity_until,
			speech_act_record_ref,
			speech_act_record_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			canonical_bytes
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?
		)`,
		permission.Ref().String(),
		permission.Digest().String(),
		content.Project().String(),
		subject.Ref().String(),
		subject.Digest().String(),
		"haft.project-typeenv.head-selection-permission-subject-role-assignment/v1",
		subject.HolderSystemRef().String(),
		"U.System",
		subject.RoleRef().String(),
		subject.BoundedContext().String(),
		canonicalGenesisTime(subjectWindow.From()),
		canonicalGenesisTime(subjectWindow.Until()),
		subjectPolicy.Ref().String(),
		subjectPolicy.Digest().String(),
		subjectPolicy.Edition().String(),
		"current_for_new_write_at_seal",
		subject.SystemAdmissionRef().String(),
		subject.SystemAdmissionDigest().String(),
		subject.RoleAdmissionRef().String(),
		subject.RoleAdmissionDigest().String(),
		subject.AssignmentJustificationRef().String(),
		subject.AssignmentJustificationDigest().String(),
		subject.AssignmentProvenanceRef().String(),
		subject.AssignmentProvenanceDigest().String(),
		string(subject.AuthorizationDescriptionRef().Kind()),
		subject.AuthorizationDescriptionRef().String(),
		subject.AuthorizationContentDigest().String(),
		subject.CanonicalJSON(),
		permission.Modality().String(),
		scope.Ref().String(),
		scope.Digest().String(),
		scope.ContextPolicyRef().String(),
		scope.ContextPolicyDigest().String(),
		referents,
		canonicalGenesisTime(permission.EffectiveFrom()),
		canonicalGenesisTime(permission.ValidityUntil()),
		record.Ref().String(),
		record.Digest().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		content.Request().Ref().String(),
		content.Request().Ref().Digest().String(),
		permission.CanonicalJSON(),
	)
}

func canonicalPermissionReferents(
	referents []projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionReferent,
) ([]byte, error) {
	type projection struct {
		Kind   string `json:"kind"`
		Ref    string `json:"ref"`
		Digest string `json:"digest"`
	}
	values := make([]projection, len(referents))
	for index := range referents {
		values[index] = projection{
			Kind:   referents[index].Kind().String(),
			Ref:    referents[index].Ref(),
			Digest: referents[index].Digest().String(),
		}
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode Permission referents: %w", err)
	}
	return canonical, nil
}

func writeGenesisAuthorityResolutionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	resolution := effect.prepared.resolved.resolution
	if explicit, ok := resolution.ExplicitPolicyAcceptance(); ok {
		return writeGenesisExplicitResolutionTx(
			ctx,
			transaction,
			effect,
			explicit,
		)
	}
	if strict, ok := resolution.StrictPermission(); ok {
		return writeGenesisStrictResolutionTx(
			ctx,
			transaction,
			effect,
			strict,
		)
	}
	return fmt.Errorf("genesis authority resolution variant is invalid")
}

func writeGenesisExplicitResolutionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	resolution projecttypeenvselectionauthority.ExplicitPolicyAcceptanceResolution,
) error {
	source := resolution.Source()
	content := source.Content()
	request := source.Request()
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			project_id,
			authority_resolution_kind,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			strict_basis_ref,
			strict_basis_digest,
			explicit_resolution_ref,
			explicit_resolution_digest,
			strict_resolution_ref,
			strict_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, 'explicit_policy_acceptance',
			?, ?, ?, ?, ?, ?,
			NULL, NULL, ?, ?, NULL, NULL, ?, ?, ?
		)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		request.Project().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		source.Ref().String(),
		source.Digest().String(),
		resolution.Ref().String(),
		resolution.Digest().String(),
		canonicalGenesisTime(resolution.EvaluatedAt()),
		resolution.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	); err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_explicit_policy_acceptance_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest
		) VALUES (?, ?, ?, ?)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		source.Ref().String(),
		source.Digest().String(),
	)
}

func writeGenesisStrictResolutionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	resolution projecttypeenvselectionauthority.StrictPermissionResolution,
) error {
	basis := resolution.Basis()
	record := basis.Record()
	content := basis.Content()
	request := basis.Request()
	resolver := basis.Policy()
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_resolution_bases (
			basis_ref,
			basis_digest,
			project_id,
			resolver_policy_ref,
			resolver_policy_edition,
			resolver_policy_digest,
			speech_act_record_ref,
			speech_act_record_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			stage_ref,
			stage_digest,
			evaluated_at,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		basis.Ref().String(),
		basis.Digest().String(),
		request.Project().String(),
		resolver.Ref().String(),
		resolver.Edition().String(),
		resolver.Digest().String(),
		record.Ref().String(),
		record.Digest().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		basis.Stage().Ref().String(),
		basis.Stage().Ref().Digest().String(),
		canonicalGenesisTime(basis.EvaluatedAt()),
		basis.CanonicalJSON(),
	); err != nil {
		return err
	}
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			project_id,
			authority_resolution_kind,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			strict_basis_ref,
			strict_basis_digest,
			explicit_resolution_ref,
			explicit_resolution_digest,
			strict_resolution_ref,
			strict_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, 'strict_permission',
			?, ?, ?, ?, NULL, NULL, ?, ?,
			NULL, NULL, ?, ?, ?, ?, ?
		)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		request.Project().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		basis.Ref().String(),
		basis.Digest().String(),
		resolution.Ref().String(),
		resolution.Digest().String(),
		canonicalGenesisTime(resolution.EvaluatedAt()),
		resolution.CanonicalJSON(),
		canonicalGenesisTime(effect.recordedAt),
	); err != nil {
		return err
	}
	permission := resolution.Permission()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_strict_permission_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			basis_ref,
			basis_digest,
			speech_act_record_ref,
			speech_act_record_digest,
			permission_ref,
			permission_digest
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		basis.Ref().String(),
		basis.Digest().String(),
		record.Ref().String(),
		record.Digest().String(),
		permission.Ref().String(),
		permission.Digest().String(),
	)
}

type storedAuthorityResolutionCoordinates struct {
	kind   string
	ref    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	digest authority.Digest
}

func storedAuthorityResolution(
	coordinates projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) (storedAuthorityResolutionCoordinates, error) {
	if explicit, ok := coordinates.TrustedDedicatedCLIInvocation(); ok {
		return storedAuthorityResolutionCoordinates{
			kind:   "explicit_policy_acceptance",
			ref:    explicit.AuthorityResolutionRef(),
			digest: explicit.AuthorityResolutionDigest(),
		}, nil
	}
	if strict, ok := coordinates.VerifiedSpeechAct(); ok {
		return storedAuthorityResolutionCoordinates{
			kind:   "strict_permission",
			ref:    strict.AuthorityResolutionRef(),
			digest: strict.AuthorityResolutionDigest(),
		}, nil
	}
	return storedAuthorityResolutionCoordinates{},
		fmt.Errorf("stored authority coordinates variant is invalid")
}

func writeGenesisAuthorityUseTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	extensions orderedExtensionCoordinates,
) error {
	use := effect.authorityUse
	target := use.Target()
	predecessor, err := storedPredecessorCoordinates(use.Predecessor())
	if err != nil {
		return err
	}
	resolution, err := storedAuthorityResolution(use.AuthorityCoordinates())
	if err != nil {
		return err
	}
	expectedGraphRevision, err := exactSQLiteInteger(
		"authority use expected graph revision",
		use.ExpectedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	committedHeadRevision, err := exactSQLiteInteger(
		"authority use committed head revision",
		use.CommittedHeadRevision().Value(),
	)
	if err != nil {
		return err
	}
	committedGraphRevision, err := exactSQLiteInteger(
		"authority use committed graph revision",
		use.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	verifierEdition, err := exactSQLiteInteger(
		"authority use verifier edition",
		use.VerifierEdition().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref,
			authority_use_digest,
			project_id,
			original_idempotency_key,
			authority_resolution_kind,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			work_ref,
			receipt_ref,
			predecessor_kind,
			predecessor_head_ref,
			predecessor_head_revision,
			predecessor_selected_composite_ref,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			committed_head_revision,
			committed_graph_revision,
			verifier_ref,
			verifier_edition,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		use.Ref().String(),
		use.Digest().String(),
		use.Project().String(),
		use.IdempotencyKey().String(),
		resolution.kind,
		resolution.ref.String(),
		resolution.digest.String(),
		use.AuthorityCoordinates().ContentRef().String(),
		use.AuthorityCoordinates().ContentDigest().String(),
		use.RequestRef().String(),
		use.RequestDigest().String(),
		use.WorkRef().String(),
		use.ReceiptRef().String(),
		predecessor.kind,
		predecessor.headRef,
		predecessor.headRevision,
		predecessor.selectedComposite,
		target.Base().String(),
		extensions.digest.String(),
		extensions.canonical,
		target.RuntimeBasis().String(),
		target.Composite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		expectedGraphRevision,
		committedHeadRevision,
		committedGraphRevision,
		use.Verifier().String(),
		verifierEdition,
		use.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisCASWorkTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	work := effect.casWork
	coordinates := work.Coordinates()
	window := coordinates.WorkInterval()
	activation := effect.activation.Delta()
	proofRef, proofDigest, err := storedGenesisProofCoordinates(effect)
	if err != nil {
		return err
	}
	committedHeadRevision, err := exactSQLiteInteger(
		"CAS work committed head revision",
		work.SuccessorHead().Revision().Value(),
	)
	if err != nil {
		return err
	}
	committedGraphRevision, err := exactSQLiteInteger(
		"CAS work committed graph revision",
		work.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_cas_work_records (
			cas_work_record_ref,
			cas_work_record_digest,
			work_ref,
			project_id,
			authority_use_ref,
			authority_use_digest,
			receipt_ref,
			activation_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			method_description_ref,
			executor_system_ref,
			executor_role_ref,
			bounded_context_ref,
			work_started_at,
			effect_sealed_at,
			committed_head_revision,
			committed_graph_revision,
			selected_composite_ref,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		work.Ref().String(),
		work.Digest().String(),
		work.WorkRef().String(),
		work.Project().String(),
		work.AuthorityUseRecordRef().String(),
		work.AuthorityUseRecordDigest().String(),
		work.ReceiptRef().String(),
		activation.Ref().String(),
		proofRef,
		proofDigest,
		coordinates.MethodDescription().String(),
		coordinates.ExecutedWithin().String(),
		coordinates.PerformedBy().String(),
		coordinates.BoundedContext().String(),
		canonicalGenesisTime(window.From()),
		canonicalGenesisTime(window.Until()),
		committedHeadRevision,
		committedGraphRevision,
		work.Target().Composite().String(),
		work.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisActivationTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	activation := effect.activation
	delta := activation.Delta()
	identity := activation.Identity()
	target := delta.Target()
	head := delta.Head()
	proofRef, proofDigest, err := storedGenesisProofCoordinates(effect)
	if err != nil {
		return err
	}
	expectedGraphRevision, err := exactSQLiteInteger(
		"activation expected graph revision",
		delta.ExpectedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	committedGraphRevision, err := exactSQLiteInteger(
		"activation committed graph revision",
		delta.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	committedHeadRevision, err := exactSQLiteInteger(
		"activation committed head revision",
		identity.SuccessorHeadRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO typed_memory_type_env_activations (
			project_id,
			event_ref,
			change_ordinal,
			activation_ref,
			activation_digest,
			canonical_activation_bytes,
			request_ref,
			request_digest,
			content_ref,
			content_digest,
			authority_use_ref,
			authority_use_digest,
			work_ref,
			basis_type_env_ref,
			result_type_env_ref,
			stage_ref,
			stage_digest,
			head_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			expected_graph_revision,
			committed_graph_revision,
			committed_head_revision,
			recorded_at
		) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delta.Project().String(),
		activation.EventRef().String(),
		delta.Ref().String(),
		delta.Digest().String(),
		delta.CanonicalBytes(),
		delta.RequestRef().String(),
		delta.RequestDigest().String(),
		effect.prepared.resolved.coordinates.ContentRef().String(),
		effect.prepared.resolved.coordinates.ContentDigest().String(),
		effect.authorityUse.Ref().String(),
		effect.authorityUse.Digest().String(),
		delta.WorkRef().String(),
		effect.prepared.basisTypeEnv.String(),
		target.Composite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		head.String(),
		proofRef,
		proofDigest,
		expectedGraphRevision,
		committedGraphRevision,
		committedHeadRevision,
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisHeadHistoryTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	activation := effect.activation
	delta := activation.Delta()
	successor := activation.SuccessorHead()
	proofRef, proofDigest, err := storedGenesisProofCoordinates(effect)
	if err != nil {
		return err
	}
	headRevision, err := exactSQLiteInteger(
		"head-history head revision",
		successor.Revision().Value(),
	)
	if err != nil {
		return err
	}
	graphRevision, err := exactSQLiteInteger(
		"head-history graph revision",
		delta.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_history (
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			graph_revision,
			graph_event_ref,
			graph_commit_ref,
			activation_ref,
			activation_digest,
			request_ref,
			request_digest,
			authority_use_ref,
			authority_use_digest,
			work_ref,
			receipt_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			head_state_digest,
			canonical_head_state_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		successor.Project().String(),
		successor.Ref().String(),
		headRevision,
		successor.SelectedComposite().String(),
		graphRevision,
		activation.EventRef().String(),
		activation.CommitRef().String(),
		delta.Ref().String(),
		delta.Digest().String(),
		delta.RequestRef().String(),
		delta.RequestDigest().String(),
		effect.authorityUse.Ref().String(),
		effect.authorityUse.Digest().String(),
		delta.WorkRef().String(),
		effect.receipt.Ref().String(),
		proofRef,
		proofDigest,
		activation.SuccessorHeadDigest().String(),
		successor.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisReceiptTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	receipt := effect.receipt
	activation := effect.activation.Delta()
	resolution, err := storedAuthorityResolution(receipt.AuthorityCoordinates())
	if err != nil {
		return err
	}
	successor := receipt.SuccessorHead()
	proofRef, proofDigest, err := storedGenesisProofCoordinates(effect)
	if err != nil {
		return err
	}
	headRevision, err := exactSQLiteInteger(
		"receipt head revision",
		successor.Revision().Value(),
	)
	if err != nil {
		return err
	}
	graphRevision, err := exactSQLiteInteger(
		"receipt graph revision",
		receipt.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_receipts (
			receipt_ref,
			receipt_digest,
			project_id,
			authority_use_ref,
			authority_use_digest,
			cas_work_record_ref,
			cas_work_record_digest,
			work_ref,
			activation_ref,
			activation_digest,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			head_ref,
			head_revision,
			selected_composite_ref,
			graph_revision,
			graph_event_ref,
			graph_commit_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		receipt.Ref().String(),
		receipt.Digest().String(),
		receipt.Project().String(),
		receipt.AuthorityUseRecordRef().String(),
		effect.authorityUse.Digest().String(),
		receipt.CASWorkRecordRef().String(),
		effect.casWork.Digest().String(),
		receipt.WorkRef().String(),
		activation.Ref().String(),
		activation.Digest().String(),
		resolution.ref.String(),
		resolution.digest.String(),
		receipt.AuthorityCoordinates().ContentRef().String(),
		receipt.AuthorityCoordinates().ContentDigest().String(),
		receipt.RequestRef().String(),
		receipt.RequestDigest().String(),
		successor.Ref().String(),
		headRevision,
		successor.SelectedComposite().String(),
		graphRevision,
		receipt.EventRef().String(),
		receipt.CommitRef().String(),
		proofRef,
		proofDigest,
		receipt.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func writeGenesisClosureTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	closure := effect.closure
	activation := effect.activation.Delta()
	resolution, err := storedAuthorityResolution(closure.AuthorityCoordinates())
	if err != nil {
		return err
	}
	successor := closure.SuccessorHead()
	proofRef, proofDigest, err := storedGenesisProofCoordinates(effect)
	if err != nil {
		return err
	}
	headRevision, err := exactSQLiteInteger(
		"closure head revision",
		successor.Revision().Value(),
	)
	if err != nil {
		return err
	}
	graphRevision, err := exactSQLiteInteger(
		"closure graph revision",
		closure.CommittedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_closures (
			closure_ref,
			closure_digest,
			project_id,
			authority_use_ref,
			authority_use_digest,
			cas_work_record_ref,
			cas_work_record_digest,
			receipt_ref,
			receipt_digest,
			activation_ref,
			activation_digest,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			head_ref,
			head_revision,
			head_state_digest,
			graph_revision,
			graph_event_ref,
			graph_event_digest,
			graph_commit_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		closure.Ref().String(),
		closure.Digest().String(),
		closure.Project().String(),
		closure.AuthorityUseRecordRef().String(),
		closure.AuthorityUseRecordDigest().String(),
		closure.CASWorkRecordRef().String(),
		closure.CASWorkRecordDigest().String(),
		closure.ReceiptRef().String(),
		closure.ReceiptDigest().String(),
		activation.Ref().String(),
		activation.Digest().String(),
		resolution.ref.String(),
		resolution.digest.String(),
		closure.AuthorityCoordinates().ContentRef().String(),
		closure.AuthorityCoordinates().ContentDigest().String(),
		closure.RequestRef().String(),
		closure.RequestDigest().String(),
		successor.Ref().String(),
		headRevision,
		closure.SuccessorHeadDigest().String(),
		graphRevision,
		closure.EventRef().String(),
		effect.eventDigest.String(),
		closure.CommitRef().String(),
		proofRef,
		proofDigest,
		closure.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}

func (resolved resolvedGenesisAuthority) resolvedContent() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent {
	if source, ok := resolved.source.TrustedDedicatedCLIInvocation(); ok {
		return source.Content()
	}
	if source, ok := resolved.source.VerifiedSpeechAct(); ok {
		return source.Record().Content()
	}
	return projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent{}
}

func (prepared preparedGenesisEffect) resolvedContent() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent {
	return prepared.resolved.resolvedContent()
}

func executeGenesisStatement(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	query string,
	arguments ...any,
) error {
	_, err := transaction.Execute(ctx, query, arguments)
	if err != nil {
		return fmt.Errorf("persist Genesis selection effect: %w", err)
	}
	return nil
}

func canonicalGenesisTime(value time.Time) string {
	return value.Round(0).UTC().Format(time.RFC3339Nano)
}
