package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectTypeEnvActivationWriterPersistsGenesisAndHistoricalReplay(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	ctx := context.Background()
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	prepared := prepareActivationIntegrationGraph(t, input)
	effect := newActivationIntegrationEffect(
		t,
		input,
		canonicalTime(fixture.clock.Now()),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	stages, err := projecttypeenvstage.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New: %v", err)
	}
	adapter, err := NewProjectTypeEnvActivationAdapter(fixture.clock, stages)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvActivationAdapter: %v", err)
	}
	// The integration keeps the live graph-head and registry-owner checks.
	// Stage canonical decoding has its own store suite; this private seam avoids
	// reproducing the full B/E/X/C lowerer fixture in typedmemorystore tests.
	adapter.verifyStage = func(
		context.Context,
		*sqlitetransaction.Transaction,
		PreparedProjectTypeEnvActivationGraph,
	) error {
		return nil
	}
	heads, err := projecttypeenvheadstore.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New: %v", err)
	}
	successor := activationIntegrationSuccessor(t, input)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	insertActivationIntegrationSources(t, ctx, transaction, input, effect)
	err = heads.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("CompareAndSwapGenesisProjectTypeEnvHeadTx: %v", err)
	}
	observation, err := adapter.WritePreparedProjectTypeEnvActivationGraphTx(
		ctx,
		transaction,
		prepared,
		func(
			callbackContext context.Context,
			callbackTransaction *sqlitetransaction.Transaction,
			write ProjectTypeEnvActivationWriteContext,
		) error {
			if err := requireNoActivationCommit(
				callbackContext,
				callbackTransaction,
				input.Request.Project().String(),
				write.CommitRef().String(),
			); err != nil {
				return err
			}
			return insertActivationIntegrationClosure(
				callbackContext,
				callbackTransaction,
				input,
				effect,
				successor,
				write,
			)
		},
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("WritePreparedProjectTypeEnvActivationGraphTx: %v", err)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("activation writer finished caller transaction: %v", err)
	}
	if observation.ActiveTypeEnv() != input.Request.Target().VerifiedComposite() {
		t.Fatalf("activation observation did not select target C")
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit activation transaction: %v", finish.Err())
	}
	read, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	defer func() { _ = read.Rollback(ctx) }()
	history, err := VerifyCommittedProjectTypeEnvActivationHistoryTx(
		ctx,
		read,
		ProjectTypeEnvActivationHistoryCoordinate{
			Project:               input.Request.Project(),
			StorageIdempotencyKey: input.StorageIdempotencyKey,
			Delta:                 input.Delta,
			Event:                 prepared.EventRef(),
			EventDigest:           prepared.EventDigest(),
			Commit:                prepared.CommitRef(),
			GraphRevision:         prepared.GraphRevision(),
			MaterializationDigest: prepared.MaterializationDigest(),
		},
	)
	if err != nil {
		t.Fatalf("VerifyCommittedProjectTypeEnvActivationHistoryTx: %v", err)
	}
	if history.ReceiptRef() != effect.receiptRef ||
		history.SelectionClosureRef() != effect.closureRef ||
		!bytes.Equal(history.ReceiptCanonicalBytes(), effect.receiptBytes) ||
		!bytes.Equal(
			history.SelectionClosureCanonicalBytes(),
			effect.closureBytes,
		) {
		t.Fatalf("historical activation replay returned different effect carriers")
	}
}

func TestProjectTypeEnvActivationWriterLeavesCallbackFailureForCallerRollback(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	ctx := context.Background()
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	prepared := prepareActivationIntegrationGraph(t, input)
	effect := newActivationIntegrationEffect(
		t,
		input,
		canonicalTime(fixture.clock.Now()),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	stages, err := projecttypeenvstage.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New: %v", err)
	}
	adapter, err := NewProjectTypeEnvActivationAdapter(fixture.clock, stages)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvActivationAdapter: %v", err)
	}
	// Keep the same boundary as the success case: only the independently tested
	// Stage decoder is replaced, while graph and registry coordinates stay live.
	adapter.verifyStage = func(
		context.Context,
		*sqlitetransaction.Transaction,
		PreparedProjectTypeEnvActivationGraph,
	) error {
		return nil
	}
	heads, err := projecttypeenvheadstore.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New: %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	insertActivationIntegrationSources(t, ctx, transaction, input, effect)
	successor := activationIntegrationSuccessor(t, input)
	err = heads.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("CompareAndSwapGenesisProjectTypeEnvHeadTx: %v", err)
	}
	callbackFailure := errors.New("activation integration callback failure")
	_, err = adapter.WritePreparedProjectTypeEnvActivationGraphTx(
		ctx,
		transaction,
		prepared,
		func(
			context.Context,
			*sqlitetransaction.Transaction,
			ProjectTypeEnvActivationWriteContext,
		) error {
			return callbackFailure
		},
	)
	if !errors.Is(err, callbackFailure) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("callback failure = %v; want %v", err, callbackFailure)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("callback failure finished caller transaction: %v", err)
	}
	if count := activationIntegrationTxRowCount(
		t,
		ctx,
		transaction,
		"typed_memory_graph_events",
		input.Request.Project().String(),
	); count != 1 {
		t.Fatalf("callback failure prelude event count = %d; want 1", count)
	}
	if count := activationIntegrationTxRowCount(
		t,
		ctx,
		transaction,
		"typed_memory_graph_commits",
		input.Request.Project().String(),
	); count != 0 {
		t.Fatalf("callback failure graph commit count = %d; want 0", count)
	}
	finish := transaction.Rollback(ctx)
	if !finish.Succeeded() {
		t.Fatalf("rollback activation transaction: %v", finish.Err())
	}
	for _, table := range []string{
		"project_typeenv_heads",
		"project_typeenv_head_states",
		"typed_memory_graph_events",
		"project_typeenv_head_selection_requests",
	} {
		var count int64
		err := fixture.database.QueryRow(
			"SELECT COUNT(*) FROM "+table+" WHERE project_id = ?",
			input.Request.Project().String(),
		).Scan(&count)
		if err != nil {
			t.Fatalf("count rolled-back %s rows: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("rolled-back %s row count = %d; want 0", table, count)
		}
	}
}

func activationIntegrationTxRowCount(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	table string,
	project string,
) int64 {
	t.Helper()
	var count int64
	err := transaction.ScanOne(
		ctx,
		"SELECT COUNT(*) FROM "+table+" WHERE project_id = ?",
		[]any{project},
		[]any{&count},
	)
	if err != nil {
		t.Fatalf("count activation integration %s rows: %v", table, err)
	}
	return count
}

func prepareActivationIntegrationGraph(
	t *testing.T,
	input ProjectTypeEnvActivationGraphInput,
) PreparedProjectTypeEnvActivationGraph {
	t.Helper()
	graph, err := PrepareProjectTypeEnvActivationGraph(input)
	if err != nil {
		t.Fatalf("PrepareProjectTypeEnvActivationGraph: %v", err)
	}
	envelope, err := projecttypeenvactivation.NewAdmissionEnvelope(
		input.Delta,
		input.StorageIdempotencyKey,
	)
	if err != nil {
		t.Fatalf("NewAdmissionEnvelope: %v", err)
	}
	basis, err := projecttypeenvactivation.NewAdmissionBasis(input.Delta, envelope)
	if err != nil {
		t.Fatalf("NewAdmissionBasis: %v", err)
	}
	manifest, err := projecttypeenvactivation.NewMaterializationManifest(
		input.Delta,
		envelope,
		basis,
		graph.EventRef(),
		graph.CommitRef(),
	)
	if err != nil {
		t.Fatalf("NewMaterializationManifest: %v", err)
	}
	prepared, err := SealPreparedProjectTypeEnvActivationGraph(
		graph,
		envelope,
		basis,
		manifest,
	)
	if err != nil {
		t.Fatalf("SealPreparedProjectTypeEnvActivationGraph: %v", err)
	}
	return prepared
}

func activationIntegrationSuccessor(
	t *testing.T,
	input ProjectTypeEnvActivationGraphInput,
) projecttypeenvselection.ProjectTypeEnvHeadState {
	t.Helper()
	state, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           input.Request.Project(),
			SelectedComposite: input.Request.Target().VerifiedComposite(),
			Revision:          input.Delta.SuccessorHeadRevision(),
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState: %v", err)
	}
	return state
}

type activationIntegrationEffect struct {
	configBasisRef          string
	configBasisDigest       typedmemory.SHA256Digest
	configCarrierDigest     typedmemory.SHA256Digest
	modePolicyRef           string
	modePolicyDigest        typedmemory.SHA256Digest
	proofRef                string
	proofDigest             typedmemory.SHA256Digest
	contentRef              string
	trustedCLIRef           string
	trustedCLIDigest        typedmemory.SHA256Digest
	resolutionRef           string
	resolutionDigest        typedmemory.SHA256Digest
	authorityUseDigest      typedmemory.SHA256Digest
	workRecordDigest        typedmemory.SHA256Digest
	receiptRef              string
	receiptDigest           typedmemory.SHA256Digest
	receiptBytes            []byte
	closureRef              string
	closureDigest           typedmemory.SHA256Digest
	closureBytes            []byte
	orderedExtensions       []byte
	orderedExtensionsDigest typedmemory.SHA256Digest
	verifierRef             string
	recordedAt              string
}

func newActivationIntegrationEffect(
	t *testing.T,
	input ProjectTypeEnvActivationGraphInput,
	recordedAt string,
) activationIntegrationEffect {
	t.Helper()
	receiptBytes := []byte("canonical-project-typeenv-selection-receipt:activation-integration")
	closureBytes := []byte("canonical-project-typeenv-selection-closure:activation-integration")
	configBasisDigest := activationIntegrationDigest(
		t,
		[]byte("canonical-config-authority-basis:activation-integration"),
	)
	modePolicyDigest := activationIntegrationDigest(
		t,
		[]byte("canonical-mode-policy:activation-integration"),
	)
	trustedCLIDigest := activationIntegrationDigest(
		t,
		[]byte("canonical-trusted-cli-source:activation-integration"),
	)
	resolutionDigest := activationIntegrationDigest(
		t,
		[]byte("canonical-authority-resolution:activation-integration"),
	)
	orderedExtensions := []byte("[]")
	proofDigest := activationAdapterTestDigest('e')
	return activationIntegrationEffect{
		configBasisRef: "project-typeenv-config-basis:" +
			configBasisDigest.String(),
		configBasisDigest: configBasisDigest,
		configCarrierDigest: activationIntegrationDigest(
			t,
			[]byte("canonical-config-carrier:activation-integration"),
		),
		modePolicyRef: "project-typeenv-mode-policy:" +
			modePolicyDigest.String(),
		modePolicyDigest: modePolicyDigest,
		proofRef: "project-typeenv-no-prior-head-proof:" +
			proofDigest.String(),
		proofDigest: proofDigest,
		contentRef:  "claim:" + input.Delta.ContentDigest().String(),
		trustedCLIRef: "project-typeenv-trusted-cli-source:" +
			trustedCLIDigest.String(),
		trustedCLIDigest: trustedCLIDigest,
		resolutionRef: "project-typeenv-authority-resolution:" +
			resolutionDigest.String(),
		resolutionDigest:   resolutionDigest,
		authorityUseDigest: activationAdapterTestDigest('9'),
		workRecordDigest: activationIntegrationDigest(
			t,
			[]byte("canonical-project-typeenv-cas-work-record:activation-integration"),
		),
		receiptRef: "project-typeenv-selection-receipt:" +
			activationIntegrationDigest(t, receiptBytes).String(),
		receiptDigest: activationIntegrationDigest(t, receiptBytes),
		receiptBytes:  receiptBytes,
		closureRef: "project-typeenv-selection-closure:" +
			activationIntegrationDigest(t, closureBytes).String(),
		closureDigest:     activationIntegrationDigest(t, closureBytes),
		closureBytes:      closureBytes,
		orderedExtensions: orderedExtensions,
		orderedExtensionsDigest: activationIntegrationDigest(
			t,
			orderedExtensions,
		),
		verifierRef: "project-typeenv-head-selection-verifier:" +
			activationAdapterTestDigest('f').String(),
		recordedAt: recordedAt,
	}
}

func activationIntegrationDigest(
	t *testing.T,
	canonical []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := digestBytes(canonical)
	if err != nil {
		t.Fatalf("digestBytes: %v", err)
	}
	return digest
}

func installActivationIntegrationCandidate(
	t *testing.T,
	database *sql.DB,
	input ProjectTypeEnvActivationGraphInput,
) {
	t.Helper()
	target := input.Request.Target()
	verificationRef := "project-typeenv-verification:" +
		activationAdapterTestDigest('c').String()
	_, err := database.Exec(
		`INSERT INTO project_typeenv_composite_verifications (
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?)`,
		verificationRef,
		activationAdapterTestDigest('c').String(),
		"lowerer-schema:activation-integration",
		"verification-schema:activation-integration",
		[]byte("canonical-composite-verification:activation-integration"),
	)
	if err != nil {
		t.Fatalf("insert composite verification: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_executable_snapshots (
			type_env_ref,
			snapshot_digest,
			lowered_environment_digest,
			source_revision,
			compiler_schema_version,
			lowerer_schema_version,
			verification_ref,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		target.VerifiedComposite().String(),
		activationAdapterTestDigest('d').String(),
		activationAdapterTestDigest('e').String(),
		"source-revision:activation-integration",
		"compiler-schema:activation-integration",
		"lowerer-schema:activation-integration",
		verificationRef,
		"executable-snapshot-schema:activation-integration",
		[]byte("canonical-executable-type-env:activation-integration"),
	)
	if err != nil {
		t.Fatalf("insert executable TypeEnv snapshot: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES ('base_type_env', ?, ?, ?, ?, ?)`,
		target.Base().String(),
		activationAdapterTestDigest('f').String(),
		"base-type-env-schema:activation-integration",
		"compiler-schema:activation-integration",
		[]byte("canonical-base-type-env-artifact:activation-integration"),
	)
	if err != nil {
		t.Fatalf("insert base TypeEnv artifact: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			canonical_schema_version,
			canonical_bytes,
			executable_type_env_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		target.Stage().String(),
		target.Stage().Digest().String(),
		input.Request.Project().String(),
		verificationRef,
		"stage-schema:activation-integration",
		[]byte("canonical-project-typeenv-stage:activation-integration"),
		target.VerifiedComposite().String(),
	)
	if err != nil {
		t.Fatalf("insert ProjectTypeEnv Stage: %v", err)
	}
}

func insertActivationIntegrationSources(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input ProjectTypeEnvActivationGraphInput,
	effect activationIntegrationEffect,
) {
	t.Helper()
	request := input.Request
	target := request.Target()
	head, err := request.Head()
	if err != nil {
		t.Fatalf("request.Head: %v", err)
	}
	_, ok := request.Predecessor().(projecttypeenvselection.GenesisStagePredecessor)
	if !ok {
		t.Fatalf("activation integration request is not Genesis")
	}
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, 'explicit_h_decide', ?, ?, ?, ?)`,
		effect.configBasisRef,
		effect.configBasisDigest.String(),
		request.Project().String(),
		".haft/config.yaml",
		effect.configCarrierDigest.String(),
		[]byte("canonical-config-authority-basis:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_mode_policies (
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
		) VALUES (?, ?, ?, 'explicit_h_decide', ?, ?, NULL, NULL, NULL, ?, ?)`,
		effect.modePolicyRef,
		effect.modePolicyDigest.String(),
		request.Project().String(),
		effect.configBasisRef,
		effect.configBasisDigest.String(),
		[]byte("canonical-mode-policy:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_no_prior_head_proofs (
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
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, 'effect_owned_head_absence_v1', ?, ?)`,
		effect.proofRef,
		effect.proofDigest.String(),
		request.Project().String(),
		head.String(),
		"project-graph-snapshot:"+activationAdapterTestDigest('f').String(),
		activationAdapterTestDigest('f').String(),
		[]byte("canonical-no-prior-head-proof:activation-integration"),
		effect.recordedAt,
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_requests (
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
			?, ?, ?, ?, 'genesis', NULL, NULL,
			NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, ?,
			'haft.project-typeenv.head-selection-request.v2', ?, ?
		)`,
		request.Ref().String(),
		request.Ref().Digest().String(),
		request.Project().String(),
		head.String(),
		target.Base().String(),
		effect.orderedExtensionsDigest.String(),
		effect.orderedExtensions,
		target.RuntimeBasis().String(),
		target.VerifiedComposite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		request.IdempotencyKey().String(),
		request.CanonicalBytes(),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authorization_contents (
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
		) VALUES (?, 'claim_id', ?, ?, ?, ?, ?, 'genesis', ?, ?, ?, ?)`,
		effect.contentRef,
		input.Delta.ContentDigest().String(),
		request.Project().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		"judgement-context:activation-integration",
		"2026-07-16T08:00:00Z",
		"2026-07-16T09:00:00Z",
		[]byte("canonical-authorization-content:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_trusted_cli_sources (
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
		effect.trustedCLIRef,
		effect.trustedCLIDigest.String(),
		request.Project().String(),
		effect.modePolicyRef,
		effect.modePolicyDigest.String(),
		effect.configBasisRef,
		effect.configBasisDigest.String(),
		effect.contentRef,
		input.Delta.ContentDigest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		[]byte("canonical-trusted-cli-source:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_resolutions (
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
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		request.Project().String(),
		effect.contentRef,
		input.Delta.ContentDigest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		effect.trustedCLIRef,
		effect.trustedCLIDigest.String(),
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		effect.recordedAt,
		[]byte("canonical-authority-resolution:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_explicit_policy_acceptance_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest
		) VALUES (?, ?, ?, ?)`,
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		effect.trustedCLIRef,
		effect.trustedCLIDigest.String(),
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_uses (
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
			?, ?, ?, ?, 'explicit_policy_acceptance',
			?, ?, ?, ?, ?, ?, ?, ?,
			'genesis', NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, 1, 1, ?, 1, ?, ?
		)`,
		input.Delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		request.Project().String(),
		request.IdempotencyKey().String(),
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		effect.contentRef,
		input.Delta.ContentDigest().String(),
		request.Ref().String(),
		request.Ref().Digest().String(),
		input.Delta.WorkRef().String(),
		effect.receiptRef,
		target.Base().String(),
		effect.orderedExtensionsDigest.String(),
		effect.orderedExtensions,
		target.RuntimeBasis().String(),
		target.VerifiedComposite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		effect.verifierRef,
		[]byte("canonical-authority-use:activation-integration"),
		effect.recordedAt,
	)
	activationIntegrationMustExecute(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_cas_work_records (
			cas_work_record_ref,
			cas_work_record_digest,
			work_ref,
			project_id,
			authority_use_ref,
			authority_use_digest,
			receipt_ref,
			activation_ref,
			method_description_ref,
			executor_system_ref,
			executor_role_ref,
			bounded_context_ref,
			work_started_at,
			effect_sealed_at,
			committed_head_revision,
			committed_graph_revision,
			selected_composite_ref,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, ?, ?)`,
		input.Delta.WorkRecordRef(),
		effect.workRecordDigest.String(),
		input.Delta.WorkRef().String(),
		request.Project().String(),
		input.Delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		effect.receiptRef,
		input.Delta.Ref().String(),
		"method-description:project-typeenv-head-cas:activation-integration",
		"system:haft-software-system",
		"role:project-typeenv-head-selector",
		"bounded-context:haft-project-memory",
		effect.recordedAt,
		effect.recordedAt,
		target.VerifiedComposite().String(),
		effect.proofRef,
		effect.proofDigest.String(),
		[]byte("canonical-project-typeenv-cas-work-record:activation-integration"),
		effect.recordedAt,
	)
}

func requireNoActivationCommit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project string,
	commit string,
) error {
	var count int64
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM typed_memory_graph_commits
		WHERE project_id = ? AND commit_ref = ?`,
		[]any{project, commit},
		[]any{&count},
	)
	if err != nil {
		return fmt.Errorf("inspect activation callback commit boundary: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("activation graph commit exists before effect callback")
	}
	return nil
}

func insertActivationIntegrationClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input ProjectTypeEnvActivationGraphInput,
	effect activationIntegrationEffect,
	successor projecttypeenvselection.ProjectTypeEnvHeadState,
	write ProjectTypeEnvActivationWriteContext,
) error {
	var headStateDigest string
	var headStateBytes []byte
	err := transaction.ScanOne(
		ctx,
		`SELECT state_digest, canonical_bytes
		FROM project_typeenv_heads
		WHERE project_id = ?`,
		[]any{input.Request.Project().String()},
		[]any{&headStateDigest, &headStateBytes},
	)
	if err != nil {
		return fmt.Errorf("load activation successor head: %w", err)
	}
	if !bytes.Equal(headStateBytes, successor.CanonicalBytes()) {
		return fmt.Errorf("stored activation successor differs from CAS candidate")
	}
	delta := input.Delta
	target := input.Request.Target()
	head, err := input.Request.Head()
	if err != nil {
		return err
	}
	if err := activationIntegrationExecute(
		ctx,
		transaction,
		`INSERT INTO typed_memory_type_env_activations (
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
			expected_graph_revision,
			committed_graph_revision,
			committed_head_revision,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			recorded_at
		) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Request.Project().String(),
		write.EventRef().String(),
		delta.Ref().String(),
		delta.Digest().String(),
		delta.CanonicalBytes(),
		input.Request.Ref().String(),
		input.Request.Ref().Digest().String(),
		effect.contentRef,
		delta.ContentDigest().String(),
		delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		delta.WorkRef().String(),
		input.BasisTypeEnv.String(),
		target.VerifiedComposite().String(),
		target.Stage().String(),
		target.Stage().Digest().String(),
		head.String(),
		int64(delta.ExpectedGraphRevision().Value()),
		int64(write.GraphRevision().Value()),
		int64(delta.SuccessorHeadRevision().Value()),
		effect.proofRef,
		effect.proofDigest.String(),
		write.RecordedAt(),
	); err != nil {
		return err
	}
	if err := activationIntegrationExecute(
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_history (
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
			head_state_digest,
			canonical_head_state_bytes,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Request.Project().String(),
		head.String(),
		int64(delta.SuccessorHeadRevision().Value()),
		target.VerifiedComposite().String(),
		int64(write.GraphRevision().Value()),
		write.EventRef().String(),
		write.CommitRef().String(),
		delta.Ref().String(),
		delta.Digest().String(),
		input.Request.Ref().String(),
		input.Request.Ref().Digest().String(),
		delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		delta.WorkRef().String(),
		effect.receiptRef,
		headStateDigest,
		headStateBytes,
		effect.proofRef,
		effect.proofDigest.String(),
		write.RecordedAt(),
	); err != nil {
		return err
	}
	if err := activationIntegrationExecute(
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_receipts (
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
		effect.receiptRef,
		effect.receiptDigest.String(),
		input.Request.Project().String(),
		delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		delta.WorkRecordRef(),
		effect.workRecordDigest.String(),
		delta.WorkRef().String(),
		delta.Ref().String(),
		delta.Digest().String(),
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		effect.contentRef,
		delta.ContentDigest().String(),
		input.Request.Ref().String(),
		input.Request.Ref().Digest().String(),
		head.String(),
		int64(delta.SuccessorHeadRevision().Value()),
		target.VerifiedComposite().String(),
		int64(write.GraphRevision().Value()),
		write.EventRef().String(),
		write.CommitRef().String(),
		effect.proofRef,
		effect.proofDigest.String(),
		effect.receiptBytes,
		write.RecordedAt(),
	); err != nil {
		return err
	}
	return activationIntegrationExecute(
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_closures (
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
		effect.closureRef,
		effect.closureDigest.String(),
		input.Request.Project().String(),
		delta.AuthorityUseRef(),
		effect.authorityUseDigest.String(),
		delta.WorkRecordRef(),
		effect.workRecordDigest.String(),
		effect.receiptRef,
		effect.receiptDigest.String(),
		delta.Ref().String(),
		delta.Digest().String(),
		effect.resolutionRef,
		effect.resolutionDigest.String(),
		effect.contentRef,
		delta.ContentDigest().String(),
		input.Request.Ref().String(),
		input.Request.Ref().Digest().String(),
		head.String(),
		int64(delta.SuccessorHeadRevision().Value()),
		headStateDigest,
		int64(write.GraphRevision().Value()),
		write.EventRef().String(),
		write.EventDigest().String(),
		write.CommitRef().String(),
		effect.proofRef,
		effect.proofDigest.String(),
		effect.closureBytes,
		write.RecordedAt(),
	)
}

func activationIntegrationMustExecute(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	query string,
	arguments ...any,
) {
	t.Helper()
	if err := activationIntegrationExecute(
		ctx,
		transaction,
		query,
		arguments...,
	); err != nil {
		t.Fatalf("insert activation integration effect source: %v", err)
	}
}

func activationIntegrationExecute(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	query string,
	arguments ...any,
) error {
	_, err := transaction.Execute(ctx, query, arguments)
	if err != nil {
		return fmt.Errorf("execute activation integration statement: %w", err)
	}
	return nil
}
