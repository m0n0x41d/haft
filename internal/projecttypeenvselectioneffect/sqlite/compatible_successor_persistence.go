package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func writeCompatibleSuccessorAuthorityResolutionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect sealedGenesisEffect,
) error {
	resolved := effect.prepared.resolved
	resolution := resolved.compatibleSuccessorResolution
	if err := resolution.Verify(); err != nil {
		return fmt.Errorf("verify compatible-successor resolution: %w", err)
	}
	request := resolution.Request()
	content := resolution.Content()
	stage := resolution.Stage()
	binding := resolution.ProjectBinding()
	if err := executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_compatible_resolutions_v1 (
			resolution_ref,
			resolution_digest,
			project_id,
			project_root,
			project_binding_digest,
			selection_request_ref,
			selection_request_digest,
			content_ref,
			content_digest,
			stage_ref,
			stage_digest,
			policy_edition,
			policy_digest,
			resolution_kind,
			predicate_result,
			canonical_bytes,
			evaluated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		request.Project().String(),
		binding.Root().String(),
		binding.Digest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		content.DescriptionRef().String(),
		content.Digest().String(),
		stage.Ref().String(),
		stage.Ref().Digest().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorPolicyEdition,
		resolution.PolicyDigest().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorResolutionKind,
		"satisfied",
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
			compatible_resolution_ref,
			compatible_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?,
			NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL,
			?, ?, ?, ?, ?
		)`,
		resolution.Ref().String(),
		resolution.Digest().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorAuthorityGeneration,
		request.Project().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorResolutionKind,
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

func writeCompatibleSuccessorAuthorityUseTx(
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
	automatic, ok := use.AuthorityCoordinates().CompatibleSuccessorPolicy()
	if !ok {
		return fmt.Errorf("current authority use is not a compatible-successor policy use")
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
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		use.Ref().String(),
		use.Digest().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorAuthorityGeneration,
		use.Project().String(),
		use.IdempotencyKey().String(),
		projecttypeenvselectionauthority.CompatibleSuccessorResolutionKind,
		automatic.AuthorityResolutionRef().String(),
		automatic.AuthorityResolutionDigest().String(),
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
	resolution := effect.prepared.resolved.compatibleSuccessorResolution
	return executeGenesisStatement(
		ctx,
		transaction,
		`INSERT OR IGNORE INTO project_typeenv_head_selection_compatible_uses_v1 (
			use_ref,
			use_digest,
			resolution_ref,
			resolution_digest,
			project_id,
			project_root,
			selected_composite_ref,
			head_revision,
			canonical_bytes,
			consumed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		use.Ref().String(),
		use.Digest().String(),
		automatic.AuthorityResolutionRef().String(),
		automatic.AuthorityResolutionDigest().String(),
		use.Project().String(),
		resolution.ProjectBinding().Root().String(),
		target.Composite().String(),
		committedHeadRevision,
		use.CanonicalBytes(),
		canonicalGenesisTime(effect.recordedAt),
	)
}
