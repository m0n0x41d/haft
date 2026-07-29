package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionreadset"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type replayProbeVariant interface {
	replayProbeVariant()
}

// ReplayAbsent proves only that this transaction observed no consumed
// project/key pair. The original effect branch must still perform every
// currentness and authority check.
type ReplayAbsent struct{}

func (ReplayAbsent) replayProbeVariant() {}

// ExactReplay carries the already-committed byte-exact closure. Reaching this
// branch requires no current config, Stage, head, profile, or authority read.
type ExactReplay struct {
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1
}

func (ExactReplay) replayProbeVariant() {}

func (result ExactReplay) Closure() projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1 {
	return result.closure
}

// ConflictingReplay is a closed domain conflict, not a storage failure.
type ConflictingReplay struct {
	conflict projecttypeenvselectioneffect.ReplayConflict
}

func (ConflictingReplay) replayProbeVariant() {}

func (result ConflictingReplay) Conflict() projecttypeenvselectioneffect.ReplayConflict {
	return result.conflict
}

// ReplayProbe is a closed sum over absent, exact, and conflicting replay.
type ReplayProbe struct {
	variant replayProbeVariant
}

func (probe ReplayProbe) Absent() (ReplayAbsent, bool) {
	value, ok := probe.variant.(ReplayAbsent)
	return value, ok
}

func (probe ReplayProbe) Exact() (ExactReplay, bool) {
	value, ok := probe.variant.(ExactReplay)
	return value, ok
}

func (probe ReplayProbe) Conflict() (ConflictingReplay, bool) {
	value, ok := probe.variant.(ConflictingReplay)
	return value, ok
}

type ReplayProbeInput struct {
	Project        projectidentity.ProjectID
	IdempotencyKey projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	RequestDigest  typedmemory.SHA256Digest
	ContentDigest  authority.Digest
}

// ProbeReplayTx is intentionally the first database operation in the effect
// transaction. It observes only the stable replay key and committed closure.
func ProbeReplayTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input ReplayProbeInput,
) (ReplayProbe, error) {
	if ctx == nil {
		return ReplayProbe{}, sqlitetransaction.ErrContextRequired
	}
	if transaction == nil {
		return ReplayProbe{}, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireImmediate(); err != nil {
		return ReplayProbe{}, err
	}
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return ReplayProbe{}, fmt.Errorf("replay project identity is required")
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		input.IdempotencyKey.String(),
	)
	if err != nil || key != input.IdempotencyKey {
		return ReplayProbe{}, fmt.Errorf("replay idempotency key is required")
	}
	requestDigest, err := typedmemory.NewSHA256Digest(input.RequestDigest.String())
	if err != nil || requestDigest != input.RequestDigest {
		return ReplayProbe{}, fmt.Errorf("replay request digest is required")
	}
	contentDigest, err := authority.NewDigest(input.ContentDigest.String())
	if err != nil || contentDigest != input.ContentDigest {
		return ReplayProbe{}, fmt.Errorf("replay content digest is required")
	}
	row := replayRow{}
	err = transaction.ScanOne(
		ctx,
		replayProbeSQL,
		[]any{project.String(), key.String()},
		[]any{
			&row.requestDigest,
			&row.contentDigest,
			&row.closurePresent,
			&row.closureRef,
			&row.closureDigest,
			&row.closureCanonical,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ReplayProbe{variant: ReplayAbsent{}}, nil
	}
	if err != nil {
		return ReplayProbe{}, fmt.Errorf("probe TypeEnv head-selection replay: %w", err)
	}
	if !row.closurePresent.Valid || row.closurePresent.Int64 != 1 ||
		!row.closureRef.Valid ||
		!row.closureDigest.Valid ||
		row.closureCanonical == nil {
		return ReplayProbe{}, corruptReplay(
			"closure footprint",
			fmt.Errorf("idempotency-key owner exists without one exact closure"),
		)
	}
	existingRequestDigest, err := typedmemory.NewSHA256Digest(row.requestDigest)
	if err != nil {
		return ReplayProbe{}, corruptReplay("request digest", err)
	}
	existingContentDigest, err := authority.NewDigest(row.contentDigest)
	if err != nil {
		return ReplayProbe{}, corruptReplay("content digest", err)
	}
	exact := existingRequestDigest == requestDigest &&
		existingContentDigest == contentDigest
	if !exact {
		conflict, conflictErr := projecttypeenvselectioneffect.NewReplayConflict(
			projecttypeenvselectioneffect.ReplayConflictInput{
				Key:                    key,
				ExistingRequestDigest:  existingRequestDigest,
				PresentedRequestDigest: requestDigest,
				ExistingContentDigest:  existingContentDigest,
				PresentedContentDigest: contentDigest,
			},
		)
		if conflictErr != nil {
			return ReplayProbe{}, corruptReplay("conflict row", conflictErr)
		}
		return ReplayProbe{
			variant: ConflictingReplay{conflict: conflict},
		}, nil
	}
	closure, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadSelectionClosureV1(
			row.closureCanonical,
		)
	if err != nil {
		return ReplayProbe{}, corruptReplay("closure canonical bytes", err)
	}
	if err := verifyReplayClosure(
		closure,
		project,
		key,
		requestDigest,
		contentDigest,
		row.closureRef.String,
		row.closureDigest.String,
	); err != nil {
		return ReplayProbe{}, err
	}
	if err := verifyStoredReplayDAGTx(
		ctx,
		transaction,
		project,
		key,
		closure,
	); err != nil {
		return ReplayProbe{}, err
	}
	return ReplayProbe{variant: ExactReplay{closure: closure}}, nil
}

type replayRow struct {
	requestDigest    string
	contentDigest    string
	closurePresent   sql.NullInt64
	closureRef       sql.NullString
	closureDigest    sql.NullString
	closureCanonical []byte
}

const replayProbeSQL = `
SELECT
	authority_use.request_digest,
	authority_use.content_digest,
	CASE
		WHEN closure.closure_ref IS NULL THEN 0
		ELSE 1
	END,
	closure.closure_ref,
	closure.closure_digest,
	closure.canonical_bytes
FROM project_typeenv_head_selection_authority_uses authority_use
LEFT JOIN project_typeenv_head_selection_closures closure
	ON closure.authority_use_ref = authority_use.authority_use_ref
	AND closure.authority_use_digest = authority_use.authority_use_digest
WHERE authority_use.project_id = ?
	AND authority_use.original_idempotency_key = ?
`

type replayDAGRow struct {
	requestCanonical         []byte
	proofRefRaw              sql.NullString
	proofDigestRaw           sql.NullString
	proofObservationSchema   sql.NullString
	proofCanonical           []byte
	authorityUseCanonical    []byte
	workCanonical            []byte
	receiptCanonical         []byte
	deltaCanonical           []byte
	envelopeCanonical        []byte
	basisCanonical           []byte
	manifestCanonical        []byte
	materializationDigestRaw string
	eventDigestRaw           string
}

func verifyStoredReplayDAGTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectidentity.ProjectID,
	key projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	row := replayDAGRow{}
	err := transaction.ScanOne(
		ctx,
		replayDAGSQL,
		[]any{
			project.String(),
			key.String(),
			closure.Ref().String(),
			closure.Digest().String(),
		},
		[]any{
			&row.requestCanonical,
			&row.proofRefRaw,
			&row.proofDigestRaw,
			&row.proofObservationSchema,
			&row.proofCanonical,
			&row.authorityUseCanonical,
			&row.workCanonical,
			&row.receiptCanonical,
			&row.deltaCanonical,
			&row.envelopeCanonical,
			&row.basisCanonical,
			&row.manifestCanonical,
			&row.materializationDigestRaw,
			&row.eventDigestRaw,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return corruptReplay(
			"reference DAG",
			fmt.Errorf("committed closure members are missing or disagree"),
		)
	}
	if err != nil {
		return corruptReplay("reference DAG query", err)
	}
	if err := verifyReplayDAGCanonicals(row, closure); err != nil {
		return err
	}
	return verifyReplayActivationHistoryTx(
		ctx,
		transaction,
		row,
		closure,
	)
}

const replayDAGSQL = `
SELECT
	request.canonical_bytes,
	proof.proof_ref,
	proof.proof_digest,
	proof.observation_schema,
	proof.canonical_bytes,
	authority_use.canonical_bytes,
	work_record.canonical_bytes,
	receipt.canonical_bytes,
	activation.canonical_activation_bytes,
	admission.canonical_admission_envelope_bytes,
	admission.canonical_admission_basis_bytes,
	admission.canonical_materialization_manifest_bytes,
	materialization.materialization_digest,
	event.event_digest
FROM project_typeenv_head_selection_authority_uses authority_use
JOIN project_typeenv_head_selection_closures closure
	ON closure.authority_use_ref = authority_use.authority_use_ref
	AND closure.authority_use_digest = authority_use.authority_use_digest
JOIN project_typeenv_head_cas_work_records work_record
	ON work_record.cas_work_record_ref = closure.cas_work_record_ref
	AND work_record.cas_work_record_digest = closure.cas_work_record_digest
	AND work_record.authority_use_ref = authority_use.authority_use_ref
	AND work_record.authority_use_digest = authority_use.authority_use_digest
JOIN project_typeenv_head_selection_receipts receipt
	ON receipt.receipt_ref = closure.receipt_ref
	AND receipt.receipt_digest = closure.receipt_digest
	AND receipt.authority_use_ref = authority_use.authority_use_ref
	AND receipt.authority_use_digest = authority_use.authority_use_digest
	AND receipt.cas_work_record_ref = work_record.cas_work_record_ref
	AND receipt.cas_work_record_digest = work_record.cas_work_record_digest
JOIN typed_memory_type_env_activations activation
	ON activation.project_id = closure.project_id
	AND activation.activation_ref = closure.activation_ref
	AND activation.activation_digest = closure.activation_digest
	AND activation.authority_use_ref = authority_use.authority_use_ref
	AND activation.authority_use_digest = authority_use.authority_use_digest
	AND activation.work_ref = work_record.work_ref
JOIN project_typeenv_head_selection_authority_resolutions resolution
	ON resolution.authority_resolution_ref = closure.authority_resolution_ref
	AND resolution.authority_resolution_digest = closure.authority_resolution_digest
	AND resolution.authority_resolution_ref = authority_use.authority_resolution_ref
	AND resolution.authority_resolution_digest = authority_use.authority_resolution_digest
JOIN project_typeenv_head_selection_authorization_contents content
	ON content.content_ref = closure.content_ref
	AND content.content_digest = closure.content_digest
	AND content.content_ref = authority_use.content_ref
	AND content.content_digest = authority_use.content_digest
JOIN project_typeenv_head_selection_requests request
	ON request.request_ref = closure.request_ref
	AND request.request_digest = closure.request_digest
	AND request.request_ref = authority_use.request_ref
	AND request.request_digest = authority_use.request_digest
LEFT JOIN project_typeenv_no_prior_head_proofs proof
	ON proof.proof_ref = COALESCE(
		work_record.no_prior_head_proof_ref,
		request.no_prior_head_proof_ref
	)
	AND proof.proof_digest = COALESCE(
		work_record.no_prior_head_proof_digest,
		request.no_prior_head_proof_digest
	)
	AND proof.project_id = closure.project_id
	AND proof.head_ref = closure.head_ref
JOIN project_typeenv_head_history history
	ON history.project_id = closure.project_id
	AND history.head_ref = closure.head_ref
	AND history.head_revision = closure.head_revision
	AND history.head_state_digest = closure.head_state_digest
	AND history.graph_revision = closure.graph_revision
	AND history.graph_event_ref = closure.graph_event_ref
	AND history.graph_commit_ref = closure.graph_commit_ref
	AND history.activation_ref = closure.activation_ref
	AND history.authority_use_ref = authority_use.authority_use_ref
	AND history.authority_use_digest = authority_use.authority_use_digest
	AND history.work_ref = work_record.work_ref
	AND history.receipt_ref = receipt.receipt_ref
JOIN project_typeenv_head_states head_state
	ON head_state.project_id = history.project_id
	AND head_state.head_ref = history.head_ref
	AND head_state.head_revision = history.head_revision
	AND head_state.selected_composite_ref = history.selected_composite_ref
	AND head_state.state_digest = history.head_state_digest
	AND head_state.canonical_bytes = history.canonical_head_state_bytes
JOIN typed_memory_graph_events event
	ON event.project_id = closure.project_id
	AND event.event_ref = closure.graph_event_ref
	AND event.event_digest = closure.graph_event_digest
	AND event.commit_ref = closure.graph_commit_ref
	AND event.graph_revision = closure.graph_revision
	AND event.change_set_digest = closure.activation_digest
	AND event.canonical_change_set_bytes = activation.canonical_activation_bytes
JOIN typed_memory_graph_commits commit_record
	ON commit_record.project_id = closure.project_id
	AND commit_record.commit_ref = closure.graph_commit_ref
	AND commit_record.event_ref = closure.graph_event_ref
	AND commit_record.event_digest = closure.graph_event_digest
	AND commit_record.graph_revision = closure.graph_revision
	AND commit_record.change_set_digest = closure.activation_digest
JOIN typed_memory_event_admission_bases admission
	ON admission.project_id = closure.project_id
	AND admission.event_ref = closure.graph_event_ref
	AND admission.event_digest = closure.graph_event_digest
	AND admission.request_digest = closure.request_digest
	AND admission.semantic_digest = closure.activation_digest
JOIN typed_memory_commit_materialization_closures materialization
	ON materialization.project_id = closure.project_id
	AND materialization.event_ref = closure.graph_event_ref
	AND materialization.commit_ref = closure.graph_commit_ref
	AND materialization.event_digest = closure.graph_event_digest
	AND materialization.request_digest = closure.request_digest
	AND materialization.semantic_digest = closure.activation_digest
	AND materialization.admission_envelope_digest =
		admission.admission_envelope_digest
	AND materialization.admission_basis_digest =
		admission.admission_basis_digest
	AND materialization.materialization_manifest_digest =
		admission.materialization_manifest_digest
WHERE authority_use.project_id = ?
	AND authority_use.original_idempotency_key = ?
	AND closure.closure_ref = ?
	AND closure.closure_digest = ?
	AND closure.project_id = authority_use.project_id
	AND closure.request_ref = request.request_ref
	AND closure.request_digest = request.request_digest
	AND closure.content_ref = content.content_ref
	AND closure.content_digest = content.content_digest
	AND closure.authority_resolution_ref = resolution.authority_resolution_ref
	AND closure.authority_resolution_digest = resolution.authority_resolution_digest
	AND closure.activation_ref = receipt.activation_ref
	AND closure.activation_digest = receipt.activation_digest
	AND closure.graph_revision = receipt.graph_revision
	AND closure.graph_event_ref = receipt.graph_event_ref
	AND closure.graph_commit_ref = receipt.graph_commit_ref
	AND authority_use.work_ref = work_record.work_ref
	AND authority_use.receipt_ref = receipt.receipt_ref
	AND authority_use.committed_head_revision = history.head_revision
	AND authority_use.committed_graph_revision = history.graph_revision
	AND authority_use.selected_composite_ref = history.selected_composite_ref
	AND activation.request_ref = request.request_ref
	AND activation.request_digest = request.request_digest
	AND activation.content_ref = content.content_ref
	AND activation.content_digest = content.content_digest
	AND activation.committed_head_revision = history.head_revision
	AND activation.committed_graph_revision = history.graph_revision
	AND activation.result_type_env_ref = history.selected_composite_ref
	AND (
		(
			request.request_schema =
				'haft.project-typeenv.head-selection-request.v2'
			AND request.predecessor_kind = 'genesis'
			AND request.no_prior_head_proof_ref IS NULL
			AND request.no_prior_head_proof_digest IS NULL
			AND work_record.no_prior_head_proof_ref = proof.proof_ref
			AND work_record.no_prior_head_proof_digest = proof.proof_digest
			AND activation.no_prior_head_proof_ref = proof.proof_ref
			AND activation.no_prior_head_proof_digest = proof.proof_digest
			AND history.no_prior_head_proof_ref = proof.proof_ref
			AND history.no_prior_head_proof_digest = proof.proof_digest
			AND receipt.no_prior_head_proof_ref = proof.proof_ref
			AND receipt.no_prior_head_proof_digest = proof.proof_digest
			AND closure.no_prior_head_proof_ref = proof.proof_ref
			AND closure.no_prior_head_proof_digest = proof.proof_digest
		)
		OR
		(
			request.request_schema =
				'haft.project-typeenv.head-selection-request.v1'
			AND request.predecessor_kind = 'genesis'
			AND request.no_prior_head_proof_ref = proof.proof_ref
			AND request.no_prior_head_proof_digest = proof.proof_digest
			AND work_record.no_prior_head_proof_ref IS NULL
			AND work_record.no_prior_head_proof_digest IS NULL
			AND activation.no_prior_head_proof_ref IS NULL
			AND activation.no_prior_head_proof_digest IS NULL
			AND history.no_prior_head_proof_ref IS NULL
			AND history.no_prior_head_proof_digest IS NULL
			AND receipt.no_prior_head_proof_ref IS NULL
			AND receipt.no_prior_head_proof_digest IS NULL
			AND closure.no_prior_head_proof_ref IS NULL
			AND closure.no_prior_head_proof_digest IS NULL
		)
		OR
		(
			request.request_schema =
				'haft.project-typeenv.head-selection-request.v2'
			AND request.predecessor_kind = 'transition'
			AND request.no_prior_head_proof_ref IS NULL
			AND request.no_prior_head_proof_digest IS NULL
			AND request.prior_head_ref = authority_use.predecessor_head_ref
			AND request.prior_head_revision = authority_use.predecessor_head_revision
			AND request.prior_selected_composite_ref =
				authority_use.predecessor_selected_composite_ref
			AND work_record.no_prior_head_proof_ref IS NULL
			AND work_record.no_prior_head_proof_digest IS NULL
			AND activation.no_prior_head_proof_ref IS NULL
			AND activation.no_prior_head_proof_digest IS NULL
			AND history.no_prior_head_proof_ref IS NULL
			AND history.no_prior_head_proof_digest IS NULL
			AND receipt.no_prior_head_proof_ref IS NULL
			AND receipt.no_prior_head_proof_digest IS NULL
			AND closure.no_prior_head_proof_ref IS NULL
			AND closure.no_prior_head_proof_digest IS NULL
			AND proof.proof_ref IS NULL
		)
	)
	AND admission.admission_basis_kind = 'snapshot_only'
	AND materialization.admission_basis_kind = 'snapshot_only'
	AND materialization.type_env_activation_count = 1
`

func verifyReplayClosure(
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	project projectidentity.ProjectID,
	key projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey,
	requestDigest typedmemory.SHA256Digest,
	contentDigest authority.Digest,
	storedRef string,
	storedDigest string,
) error {
	digest, err := typedmemory.NewSHA256Digest(storedDigest)
	if err != nil {
		return corruptReplay("closure digest", err)
	}
	matches := closure.Ref().String() == storedRef &&
		closure.Digest() == digest &&
		closure.Project() == project &&
		closure.IdempotencyKey() == key &&
		closure.RequestDigest() == requestDigest &&
		closure.AuthorityCoordinates().ContentDigest() == contentDigest
	if !matches {
		return corruptReplay(
			"closure coordinates",
			fmt.Errorf("stored replay row differs from decoded closure"),
		)
	}
	return nil
}

func verifyReplayDAGCanonicals(
	row replayDAGRow,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	request, err :=
		projecttypeenvselection.DecodeProjectTypeEnvHeadSelectionRequest(
			row.requestCanonical,
		)
	if err != nil {
		return corruptReplay("request canonical bytes", err)
	}
	requestTarget, err :=
		projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTargetFromRequest(
			request,
		)
	if err != nil {
		return corruptReplay("request target", err)
	}
	if !replayRequestMatchesClosure(request, requestTarget, closure) {
		return corruptReplay(
			"request coordinates",
			fmt.Errorf("request differs from closure"),
		)
	}
	authorityUse, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadSelectionAuthorityUseRecord(
			row.authorityUseCanonical,
		)
	if err != nil {
		return corruptReplay("authority-use canonical bytes", err)
	}
	if !replayAuthorityUseMatchesClosure(authorityUse, closure) {
		return corruptReplay(
			"authority-use coordinates",
			fmt.Errorf("authority use differs from closure"),
		)
	}
	work, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadCASWorkRecord(
			row.workCanonical,
		)
	if err != nil {
		return corruptReplay("Work-record canonical bytes", err)
	}
	if !replayWorkMatchesClosure(work, closure) {
		return corruptReplay(
			"Work-record coordinates",
			fmt.Errorf("work record differs from closure"),
		)
	}
	if err := verifyReplayPredecessor(row, request, work, closure); err != nil {
		return err
	}
	receipt, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvHeadSelectionReceiptV1(
			row.receiptCanonical,
		)
	if err != nil {
		return corruptReplay("receipt canonical bytes", err)
	}
	if !replayReceiptMatchesClosure(receipt, closure) {
		return corruptReplay(
			"receipt coordinates",
			fmt.Errorf("receipt differs from closure"),
		)
	}
	delta, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvActivationDelta(
			row.deltaCanonical,
		)
	if err != nil {
		return corruptReplay("activation-delta canonical bytes", err)
	}
	if !replayDeltaMatchesClosure(delta, closure) {
		return corruptReplay(
			"activation-delta coordinates",
			fmt.Errorf("activation delta differs from closure"),
		)
	}
	envelope, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvActivationAdmissionEnvelope(
			row.envelopeCanonical,
		)
	if err != nil {
		return corruptReplay("activation-envelope canonical bytes", err)
	}
	basis, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvActivationAdmissionBasis(
			row.basisCanonical,
		)
	if err != nil {
		return corruptReplay("activation-basis canonical bytes", err)
	}
	manifest, err :=
		projecttypeenvselectioneffect.DecodeProjectTypeEnvActivationMaterializationManifest(
			row.manifestCanonical,
		)
	if err != nil {
		return corruptReplay("activation-manifest canonical bytes", err)
	}
	materializationDigest, err := typedmemory.NewSHA256Digest(
		row.materializationDigestRaw,
	)
	if err != nil {
		return corruptReplay("materialization digest", err)
	}
	if !replayActivationMembersMatchClosure(
		envelope,
		basis,
		manifest,
		materializationDigest,
		closure,
	) {
		return corruptReplay(
			"activation admission coordinates",
			fmt.Errorf("activation admission DAG differs from closure"),
		)
	}
	return nil
}

func verifyReplayNoPriorHeadProof(
	row replayDAGRow,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) (projecttypeenvselection.NoPriorHeadProofRef, error) {
	if !row.proofRefRaw.Valid || !row.proofDigestRaw.Valid ||
		!row.proofObservationSchema.Valid || row.proofCanonical == nil {
		return projecttypeenvselection.NoPriorHeadProofRef{},
			corruptReplay(
				"absence-proof footprint",
				fmt.Errorf("genesis replay omitted its exact absence proof"),
			)
	}
	ref, err := projecttypeenvselection.ParseNoPriorHeadProofRef(row.proofRefRaw.String)
	if err != nil {
		return projecttypeenvselection.NoPriorHeadProofRef{},
			corruptReplay("absence-proof ref", err)
	}
	digest, err := typedmemory.NewSHA256Digest(row.proofDigestRaw.String)
	if err != nil || digest != ref.Digest() {
		return projecttypeenvselection.NoPriorHeadProofRef{},
			corruptReplay(
				"absence-proof digest",
				fmt.Errorf("absence-proof ref/digest mismatch"),
			)
	}
	switch row.proofObservationSchema.String {
	case "effect_owned_head_absence_v1":
		proof, verifyErr := projecttypeenvselectionreadset.VerifyNoPriorHeadProof(
			ref,
			row.proofCanonical,
		)
		if verifyErr != nil {
			return projecttypeenvselection.NoPriorHeadProofRef{},
				corruptReplay("absence-proof canonical bytes", verifyErr)
		}
		matches := proof.Project() == closure.Project() &&
			proof.Head() == closure.SuccessorHead().Ref() &&
			proof.GraphRevision() == closure.ExpectedGraphRevision()
		if !matches {
			return projecttypeenvselection.NoPriorHeadProofRef{},
				corruptReplay(
					"absence-proof coordinates",
					fmt.Errorf("absence proof differs from committed closure"),
				)
		}
	case "legacy_request_owned_v47":
		proof, verifyErr := projecttypeenvselection.VerifyNoPriorHeadProof(
			ref,
			row.proofCanonical,
		)
		if verifyErr != nil {
			return projecttypeenvselection.NoPriorHeadProofRef{},
				corruptReplay("legacy absence-proof canonical bytes", verifyErr)
		}
		matches := proof.Project() == closure.Project() &&
			proof.Head() == closure.SuccessorHead().Ref() &&
			proof.ExpectedGraphRevision() == closure.ExpectedGraphRevision()
		if !matches {
			return projecttypeenvselection.NoPriorHeadProofRef{},
				corruptReplay(
					"legacy absence-proof coordinates",
					fmt.Errorf("legacy absence proof differs from committed closure"),
				)
		}
	default:
		return projecttypeenvselection.NoPriorHeadProofRef{},
			corruptReplay(
				"absence-proof observation schema",
				fmt.Errorf(
					"unsupported absence-proof schema %q",
					row.proofObservationSchema.String,
				),
			)
	}
	return ref, nil
}

func verifyReplayPredecessor(
	row replayDAGRow,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	work projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkRecord,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	switch predecessor := request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		proof, err := verifyReplayNoPriorHeadProof(row, closure)
		if err != nil {
			return err
		}
		comparison, ok := work.PredecessorComparison().(projecttypeenvselectioneffect.GenesisHeadAbsenceMatched)
		if !ok || comparison.Proof() != proof {
			return corruptReplay(
				"Work-record absence proof",
				fmt.Errorf("work record differs from effect-owned absence proof"),
			)
		}
		return nil
	case projecttypeenvselection.TransitionStagePredecessor:
		proofAbsent := !row.proofRefRaw.Valid &&
			!row.proofDigestRaw.Valid &&
			!row.proofObservationSchema.Valid &&
			row.proofCanonical == nil
		if !proofAbsent {
			return corruptReplay(
				"Transition absence-proof footprint",
				fmt.Errorf("transition replay unexpectedly contains a Genesis proof"),
			)
		}
		comparison, ok := work.PredecessorComparison().(projecttypeenvselectioneffect.TransitionHeadMatched)
		if !ok || !predecessorsEqual(comparison.Prior(), predecessor) {
			return corruptReplay(
				"Work-record Transition predecessor",
				fmt.Errorf("work record differs from exact prior head"),
			)
		}
		return nil
	default:
		return corruptReplay(
			"request predecessor",
			fmt.Errorf("request predecessor variant is unsupported"),
		)
	}
}

func verifyReplayActivationHistoryTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	row replayDAGRow,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) error {
	delta, err := projecttypeenvactivation.DecodeDelta(row.deltaCanonical)
	if err != nil {
		return corruptReplay("neutral activation delta", err)
	}
	eventDigest, err := typedmemory.NewSHA256Digest(row.eventDigestRaw)
	if err != nil {
		return corruptReplay("activation event digest", err)
	}
	identity, err :=
		projecttypeenvselectioneffect.SealProjectTypeEnvHeadSelectionTransactionIdentity(
			projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTransactionIdentityInput{
				Project:                closure.Project(),
				IdempotencyKey:         closure.IdempotencyKey(),
				RequestRef:             closure.RequestRef(),
				RequestDigest:          closure.RequestDigest(),
				ContentDigest:          closure.AuthorityCoordinates().ContentDigest(),
				SuccessorHeadRevision:  closure.SuccessorHead().Revision(),
				CommittedGraphRevision: closure.CommittedGraphRevision(),
			},
		)
	if err != nil {
		return corruptReplay("transaction identity", err)
	}
	if identity.Ref() != closure.TransactionRef() ||
		identity.Digest() != closure.TransactionDigest() {
		return corruptReplay(
			"transaction identity",
			fmt.Errorf("closure transaction identity differs from recomputed identity"),
		)
	}
	referenceDAG, err :=
		projecttypeenvselectioneffect.DeriveProjectTypeEnvHeadSelectionReferenceDAG(
			identity,
		)
	if err != nil {
		return corruptReplay("reference DAG", err)
	}
	history, err := typedmemorystore.VerifyCommittedProjectTypeEnvActivationHistoryTx(
		ctx,
		transaction,
		typedmemorystore.ProjectTypeEnvActivationHistoryCoordinate{
			Project:               closure.Project(),
			StorageIdempotencyKey: referenceDAG.GraphIdempotencyKey().String(),
			Delta:                 delta,
			Event:                 closure.EventRef(),
			EventDigest:           eventDigest,
			Commit:                closure.CommitRef(),
			GraphRevision:         closure.CommittedGraphRevision(),
			MaterializationDigest: closure.MaterializationDigest(),
		},
	)
	if err != nil {
		return corruptReplay("exact activation history", err)
	}
	matches := history.Project() == closure.Project() &&
		history.EventRef() == closure.EventRef() &&
		history.EventDigest() == eventDigest &&
		history.CommitRef() == closure.CommitRef() &&
		history.GraphRevision() == closure.CommittedGraphRevision() &&
		history.MaterializationDigest() == closure.MaterializationDigest() &&
		bytes.Equal(history.Delta().CanonicalBytes(), row.deltaCanonical) &&
		history.ReceiptRef() == closure.ReceiptRef().String() &&
		history.ReceiptDigest() == closure.ReceiptDigest() &&
		bytes.Equal(history.ReceiptCanonicalBytes(), row.receiptCanonical) &&
		history.SelectionClosureRef() == closure.Ref().String() &&
		history.SelectionClosureDigest() == closure.Digest() &&
		bytes.Equal(
			history.SelectionClosureCanonicalBytes(),
			closure.CanonicalBytes(),
		)
	if !matches {
		return corruptReplay(
			"exact activation history",
			fmt.Errorf("verified storage history differs from effect closure"),
		)
	}
	return nil
}

func replayRequestMatchesClosure(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	target projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTarget,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return request.Ref() == closure.RequestRef() &&
		request.Ref().Digest() == closure.RequestDigest() &&
		request.Project() == closure.Project() &&
		request.IdempotencyKey() == closure.IdempotencyKey() &&
		predecessorsEqual(request.Predecessor(), closure.Predecessor()) &&
		targetsEqual(target, closure.Target()) &&
		request.ExpectedGraphRevision() == closure.ExpectedGraphRevision()
}

func replayAuthorityUseMatchesClosure(
	record projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionAuthorityUseRecord,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return record.Ref() == closure.AuthorityUseRecordRef() &&
		record.Digest() == closure.AuthorityUseRecordDigest() &&
		record.TransactionRef() == closure.TransactionRef() &&
		record.TransactionDigest() == closure.TransactionDigest() &&
		record.Project() == closure.Project() &&
		record.IdempotencyKey() == closure.IdempotencyKey() &&
		record.RequestRef() == closure.RequestRef() &&
		record.RequestDigest() == closure.RequestDigest() &&
		record.AuthorityCoordinates().ExactEqual(closure.AuthorityCoordinates()) &&
		record.WorkRef() == closure.WorkRef() &&
		record.ReceiptRef() == closure.ReceiptRef() &&
		record.ReceiptDigest() == closure.ReceiptDigest() &&
		predecessorsEqual(record.Predecessor(), closure.Predecessor()) &&
		targetsEqual(record.Target(), closure.Target()) &&
		record.ExpectedGraphRevision() == closure.ExpectedGraphRevision() &&
		record.CommittedHeadRevision() == closure.SuccessorHead().Revision() &&
		record.CommittedGraphRevision() == closure.CommittedGraphRevision() &&
		record.CommittedResultRef() == closure.CommittedResultRef() &&
		record.CommittedResultDigest() == closure.CommittedResultDigest()
}

func replayWorkMatchesClosure(
	record projecttypeenvselectioneffect.ProjectTypeEnvHeadCASWorkRecord,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return record.Ref() == closure.CASWorkRecordRef() &&
		record.Digest() == closure.CASWorkRecordDigest() &&
		record.TransactionRef() == closure.TransactionRef() &&
		record.TransactionDigest() == closure.TransactionDigest() &&
		record.Project() == closure.Project() &&
		record.WorkRef() == closure.WorkRef() &&
		record.RequestRef() == closure.RequestRef() &&
		record.RequestDigest() == closure.RequestDigest() &&
		record.AuthorityCoordinates().ExactEqual(closure.AuthorityCoordinates()) &&
		record.AuthorityUseRecordRef() == closure.AuthorityUseRecordRef() &&
		record.AuthorityUseRecordDigest() == closure.AuthorityUseRecordDigest() &&
		record.ReceiptRef() == closure.ReceiptRef() &&
		record.ReceiptDigest() == closure.ReceiptDigest() &&
		predecessorsEqual(record.Predecessor(), closure.Predecessor()) &&
		targetsEqual(record.Target(), closure.Target()) &&
		record.ExpectedGraphRevision() == closure.ExpectedGraphRevision() &&
		record.CommittedGraphRevision() == closure.CommittedGraphRevision() &&
		headsEqual(record.SuccessorHead(), closure.SuccessorHead()) &&
		record.SuccessorHeadDigest() == closure.SuccessorHeadDigest() &&
		record.EventRef() == closure.EventRef() &&
		record.CommitRef() == closure.CommitRef() &&
		record.CommittedResultRef() == closure.CommittedResultRef() &&
		record.CommittedResultDigest() == closure.CommittedResultDigest()
}

func replayReceiptMatchesClosure(
	receipt projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptV1,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return receipt.Ref() == closure.ReceiptRef() &&
		receipt.Digest() == closure.ReceiptDigest() &&
		receipt.TransactionRef() == closure.TransactionRef() &&
		receipt.TransactionDigest() == closure.TransactionDigest() &&
		receipt.Project() == closure.Project() &&
		receipt.IdempotencyKey() == closure.IdempotencyKey() &&
		receipt.RequestRef() == closure.RequestRef() &&
		receipt.RequestDigest() == closure.RequestDigest() &&
		receipt.AuthorityCoordinates().ExactEqual(closure.AuthorityCoordinates()) &&
		receipt.AuthorityUseRecordRef() == closure.AuthorityUseRecordRef() &&
		receipt.WorkRef() == closure.WorkRef() &&
		receipt.CASWorkRecordRef() == closure.CASWorkRecordRef() &&
		predecessorsEqual(receipt.Predecessor(), closure.Predecessor()) &&
		targetsEqual(receipt.Target(), closure.Target()) &&
		receipt.ExpectedGraphRevision() == closure.ExpectedGraphRevision() &&
		receipt.CommittedGraphRevision() == closure.CommittedGraphRevision() &&
		headsEqual(receipt.SuccessorHead(), closure.SuccessorHead()) &&
		receipt.SuccessorHeadDigest() == closure.SuccessorHeadDigest() &&
		receipt.EventRef() == closure.EventRef() &&
		receipt.CommitRef() == closure.CommitRef() &&
		receipt.MaterializationDigest() == closure.MaterializationDigest() &&
		receipt.CommittedResultRef() == closure.CommittedResultRef() &&
		receipt.CommittedResultDigest() == closure.CommittedResultDigest()
}

func replayDeltaMatchesClosure(
	delta projecttypeenvselectioneffect.ProjectTypeEnvActivationDelta,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return delta.Ref() == closure.ActivationDeltaRef() &&
		delta.Digest() == closure.ActivationDeltaDigest() &&
		delta.TransactionRef() == closure.TransactionRef() &&
		delta.TransactionDigest() == closure.TransactionDigest() &&
		delta.Project() == closure.Project() &&
		delta.Head() == closure.SuccessorHead().Ref() &&
		delta.RequestRef() == closure.RequestRef() &&
		delta.RequestDigest() == closure.RequestDigest() &&
		delta.ContentDigest() == closure.AuthorityCoordinates().ContentDigest() &&
		delta.AuthorityUseRecordRef() == closure.AuthorityUseRecordRef() &&
		delta.WorkRef() == closure.WorkRef() &&
		delta.CASWorkRecordRef() == closure.CASWorkRecordRef() &&
		predecessorsEqual(delta.Predecessor(), closure.Predecessor()) &&
		targetsEqual(delta.Target(), closure.Target()) &&
		delta.ExpectedGraphRevision() == closure.ExpectedGraphRevision() &&
		delta.CommittedGraphRevision() == closure.CommittedGraphRevision() &&
		delta.SuccessorHeadRevision() == closure.SuccessorHead().Revision()
}

func replayActivationMembersMatchClosure(
	envelope projecttypeenvselectioneffect.ProjectTypeEnvActivationAdmissionEnvelope,
	basis projecttypeenvselectioneffect.ProjectTypeEnvActivationAdmissionBasis,
	manifest projecttypeenvselectioneffect.ProjectTypeEnvActivationMaterializationManifest,
	materializationDigest typedmemory.SHA256Digest,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) bool {
	return envelope.Ref() == closure.ActivationEnvelopeRef() &&
		envelope.Digest() == closure.ActivationEnvelopeDigest() &&
		envelope.DeltaRef() == closure.ActivationDeltaRef() &&
		envelope.DeltaDigest() == closure.ActivationDeltaDigest() &&
		envelope.RequestRef() == closure.RequestRef() &&
		envelope.RequestDigest() == closure.RequestDigest() &&
		envelope.TargetComposite() == closure.Target().Composite() &&
		envelope.Stage() == closure.Target().Stage() &&
		basis.Ref() == closure.ActivationBasisRef() &&
		basis.Digest() == closure.ActivationBasisDigest() &&
		basis.EnvelopeRef() == closure.ActivationEnvelopeRef() &&
		basis.EnvelopeDigest() == closure.ActivationEnvelopeDigest() &&
		basis.Project() == closure.Project() &&
		predecessorsEqual(basis.Predecessor(), closure.Predecessor()) &&
		basis.TargetComposite() == closure.Target().Composite() &&
		basis.Stage() == closure.Target().Stage() &&
		basis.ExpectedGraphRevision() == closure.ExpectedGraphRevision() &&
		manifest.Ref() == closure.ActivationManifestRef() &&
		manifest.Digest() == closure.ActivationManifestDigest() &&
		manifest.DeltaRef() == closure.ActivationDeltaRef() &&
		manifest.DeltaDigest() == closure.ActivationDeltaDigest() &&
		manifest.EnvelopeRef() == closure.ActivationEnvelopeRef() &&
		manifest.EnvelopeDigest() == closure.ActivationEnvelopeDigest() &&
		manifest.BasisRef() == closure.ActivationBasisRef() &&
		manifest.BasisDigest() == closure.ActivationBasisDigest() &&
		manifest.EventRef() == closure.EventRef() &&
		manifest.CommitRef() == closure.CommitRef() &&
		manifest.ActivationCount() == 1 &&
		manifest.TopLevelChangeCount() == 1 &&
		materializationDigest == closure.MaterializationDigest()
}

func targetsEqual(
	left projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTarget,
	right projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionTarget,
) bool {
	return left.Base() == right.Base() &&
		slices.Equal(left.OrderedExtensions(), right.OrderedExtensions()) &&
		left.RuntimeBasis() == right.RuntimeBasis() &&
		left.Composite() == right.Composite() &&
		left.Stage() == right.Stage()
}

func predecessorsEqual(
	left projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
	right projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) bool {
	switch typedLeft := left.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		typedRight, ok := right.(projecttypeenvselection.GenesisStagePredecessor)
		return ok && typedLeft == typedRight
	case projecttypeenvselection.TransitionStagePredecessor:
		typedRight, ok := right.(projecttypeenvselection.TransitionStagePredecessor)
		return ok && typedLeft == typedRight
	default:
		return false
	}
}

func headsEqual(
	left projecttypeenvselection.ProjectTypeEnvHeadState,
	right projecttypeenvselection.ProjectTypeEnvHeadState,
) bool {
	return left.Project() == right.Project() &&
		left.Ref() == right.Ref() &&
		left.SelectedComposite() == right.SelectedComposite() &&
		left.Revision() == right.Revision() &&
		slices.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func corruptReplay(label string, cause error) error {
	return fmt.Errorf("corrupt TypeEnv head-selection replay %s: %w", label, cause)
}
