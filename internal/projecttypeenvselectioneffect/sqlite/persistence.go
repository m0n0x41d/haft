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
	if err := writeGenesisHostRequestTx(
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
	if err := writeGenesisRequestTx(ctx, transaction, effect, extensions); err != nil {
		return err
	}
	if err := writeGenesisAuthorizationContentTx(ctx, transaction, effect); err != nil {
		return err
	}
	if err := writeGenesisHostRequestTx(ctx, transaction, effect); err != nil {
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

func writeGenesisHostRequestTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	resolved := effect.prepared.resolved
	if resolved.isCompatibleSuccessor() {
		return nil
	}
	request := resolved.request
	resolution := resolved.hostResolution
	binding := resolution.ProjectBinding()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_host_requests_v1 (
			request_ref,
			request_digest,
			project_id,
			project_root,
			effect_kind,
			subject_ref,
			payload_digest,
			provenance,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		request.Ref(),
		request.Digest(),
		resolved.content.Project().String(),
		binding.Root().String(),
		string(request.Effect()),
		request.SubjectRef(),
		request.PayloadDigest(),
		string(request.Provenance()),
		canonicalGenesisTime(resolution.EvaluatedAt()),
	)
}

func writeGenesisAuthorityResolutionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	resolved := effect.prepared.resolved
	if resolved.isCompatibleSuccessor() {
		return writeCompatibleSuccessorAuthorityResolutionTx(
			ctx,
			transaction,
			effect,
		)
	}
	resolution := resolved.hostResolution
	request := resolution.SelectionRequest()
	content := resolution.Content()
	binding := resolution.ProjectBinding()
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_host_resolutions_v1 (
			resolution_ref,
			resolution_digest,
			request_ref,
			request_digest,
			project_id,
			project_root,
			project_binding_digest,
			selection_request_ref,
			selection_request_digest,
			content_ref,
			content_digest,
			resolution_kind,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		resolved.request.Ref(),
		resolved.request.Digest(),
		request.Project().String(),
		binding.Root().String(),
		binding.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		"host_routed_request_acceptance",
		resolution.CanonicalJSON(),
		canonicalGenesisTime(resolution.EvaluatedAt()),
	); err != nil {
		return err
	}
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			authority_generation,
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
			host_resolution_ref,
			host_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, 'host_routed_operator_request', ?,
			'host_routed_request_acceptance', ?, ?, ?, ?,
			NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL,
			?, ?, ?, ?, ?
		)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		request.Project().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		resolution.Ref().String(),
		resolution.Digest().String(),
		canonicalGenesisTime(resolution.EvaluatedAt()),
		resolution.CanonicalJSON(),
		canonicalGenesisTime(resolution.EvaluatedAt()),
	)
}

func writeGenesisAuthorityUseTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
	extensions orderedExtensionCoordinates,
) error {
	if effect.prepared.resolved.isCompatibleSuccessor() {
		return writeCompatibleSuccessorAuthorityUseTx(
			ctx,
			transaction,
			effect,
			extensions,
		)
	}
	use := effect.authorityUse
	target := use.Target()
	predecessor, err := storedPredecessorCoordinates(use.Predecessor())
	if err != nil {
		return err
	}
	host, ok := use.AuthorityCoordinates().HostRoutedOperatorRequest()
	if !ok {
		return fmt.Errorf("current authority use is not host-routed")
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
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref,
			authority_use_digest,
			authority_generation,
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
			?, ?, 'host_routed_operator_request', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		use.Ref().String(),
		use.Digest().String(),
		use.Project().String(),
		use.IdempotencyKey().String(),
		"host_routed_request_acceptance",
		host.AuthorityResolutionRef().String(),
		host.AuthorityResolutionDigest().String(),
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
	); err != nil {
		return err
	}
	request := host.OperatorRequest()
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_host_uses_v1 (
			use_ref,
			use_digest,
			resolution_ref,
			resolution_digest,
			request_ref,
			request_digest,
			project_id,
			project_root,
			selected_composite_ref,
			head_revision,
			canonical_bytes,
			consumed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		use.Ref().String(),
		use.Digest().String(),
		host.AuthorityResolutionRef().String(),
		host.AuthorityResolutionDigest().String(),
		request.Ref(),
		request.Digest(),
		use.Project().String(),
		effect.prepared.resolved.hostResolution.ProjectBinding().Root().String(),
		target.Composite().String(),
		committedHeadRevision,
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
		coordinates.ActualPerformerSystem().String(),
		coordinates.CoveringRoleAssignment().String(),
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
	resolutionRef, resolutionDigest, err := currentAuthorityResolutionCoordinates(
		receipt.AuthorityCoordinates(),
	)
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
		resolutionRef.String(),
		resolutionDigest.String(),
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
	resolutionRef, resolutionDigest, err := currentAuthorityResolutionCoordinates(
		closure.AuthorityCoordinates(),
	)
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
		resolutionRef.String(),
		resolutionDigest.String(),
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

func currentAuthorityResolutionCoordinates(
	coordinates projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) (
	projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef,
	authority.Digest,
	error,
) {
	if host, ok := coordinates.HostRoutedOperatorRequest(); ok {
		return host.AuthorityResolutionRef(), host.AuthorityResolutionDigest(), nil
	}
	if automatic, ok := coordinates.CompatibleSuccessorPolicy(); ok {
		return automatic.AuthorityResolutionRef(), automatic.AuthorityResolutionDigest(), nil
	}
	return projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef{},
		authority.Digest{},
		fmt.Errorf("current receipt or closure authority provenance is unsupported")
}

func (prepared preparedGenesisEffect) resolvedContent() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent {
	return prepared.resolved.content
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
