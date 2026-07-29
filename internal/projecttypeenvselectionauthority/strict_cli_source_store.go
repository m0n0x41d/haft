package projecttypeenvselectionauthority

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type StrictCLISpeechActCapturer interface {
	Capture(
		context.Context,
		authority.PreparedManualSpeechAct,
	) (authority.VerifiedSpeechActSource, error)
}

// ControllingTerminalStrictCLISpeechActCapturer is the production adapter. It
// delegates terminal observation and the generic SpeechAct mint to authority;
// this package owns only the TypeEnv-specific composition and durable effect.
type ControllingTerminalStrictCLISpeechActCapturer struct{}

func (ControllingTerminalStrictCLISpeechActCapturer) Capture(
	ctx context.Context,
	prepared authority.PreparedManualSpeechAct,
) (authority.VerifiedSpeechActSource, error) {
	return authority.CaptureVerifiedSpeechAct(ctx, prepared)
}

type strictCLIDurableSourceResult interface {
	strictCLIDurableSourceResultVariant()
}

type StrictCLISpeechActCaptured struct {
	record ProjectTypeEnvHeadSelectionSpeechActRecord
}

func (StrictCLISpeechActCaptured) strictCLIDurableSourceResultVariant() {}

func (result StrictCLISpeechActCaptured) Record() ProjectTypeEnvHeadSelectionSpeechActRecord {
	return result.record
}

type StrictCLISpeechActReplayed struct {
	record ProjectTypeEnvHeadSelectionSpeechActRecord
}

func (StrictCLISpeechActReplayed) strictCLIDurableSourceResultVariant() {}

func (result StrictCLISpeechActReplayed) Record() ProjectTypeEnvHeadSelectionSpeechActRecord {
	return result.record
}

// StrictCLIDurableSourceResult is a closed by-value result. It distinguishes a
// newly captured durable human act from an exact replay recovered without
// opening the terminal.
type StrictCLIDurableSourceResult struct {
	variant strictCLIDurableSourceResult
}

func NewStrictCLIDurableSourceResult(
	variant strictCLIDurableSourceResult,
) (StrictCLIDurableSourceResult, error) {
	switch value := variant.(type) {
	case StrictCLISpeechActCaptured:
		if err := value.record.Verify(value.record.Content().Request()); err != nil {
			return StrictCLIDurableSourceResult{}, err
		}
	case StrictCLISpeechActReplayed:
		if err := value.record.Verify(value.record.Content().Request()); err != nil {
			return StrictCLIDurableSourceResult{}, err
		}
	default:
		return StrictCLIDurableSourceResult{}, fmt.Errorf(
			"strict CLI durable source result variant is invalid",
		)
	}
	return StrictCLIDurableSourceResult{variant: variant}, nil
}

func (result StrictCLIDurableSourceResult) Captured() (
	StrictCLISpeechActCaptured,
	bool,
) {
	value, ok := result.variant.(StrictCLISpeechActCaptured)
	return value, ok
}

func (result StrictCLIDurableSourceResult) Replayed() (
	StrictCLISpeechActReplayed,
	bool,
) {
	value, ok := result.variant.(StrictCLISpeechActReplayed)
	return value, ok
}

func (result StrictCLIDurableSourceResult) Record() (
	ProjectTypeEnvHeadSelectionSpeechActRecord,
	bool,
) {
	switch value := result.variant.(type) {
	case StrictCLISpeechActCaptured:
		return value.record, true
	case StrictCLISpeechActReplayed:
		return value.record, true
	default:
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false
	}
}

type StrictCLISpeechActSourceStore struct {
	database *sql.DB
	capturer StrictCLISpeechActCapturer
	writer   *authority.SpeechActSourceWriter
	now      func() time.Time
}

func OpenStrictCLISpeechActSourceStore(
	database *sql.DB,
	capturer StrictCLISpeechActCapturer,
) (*StrictCLISpeechActSourceStore, error) {
	if database == nil || capturer == nil {
		return nil, fmt.Errorf(
			"strict CLI SpeechAct source store requires a database and capturer",
		)
	}
	writer, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		return nil, err
	}
	var count int
	err = database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 48",
	).Scan(&count)
	if err != nil || count != 1 {
		return nil, errors.Join(
			fmt.Errorf("ProjectTypeEnv strict source migration 48 is unavailable"),
			err,
		)
	}
	return &StrictCLISpeechActSourceStore{
		database: database,
		capturer: capturer,
		writer:   writer,
		now:      time.Now,
	}, nil
}

// ResolveOrCapture first recovers an exact already-durable source. Only an
// absent source opens the controlling terminal. After a successful utterance,
// the generic SpeechAct source, TypeEnv-specific source record, and instituted
// Permission are committed atomically in a transaction that precedes and is
// independent of the later ProjectTypeEnvHead CAS transaction.
func (store *StrictCLISpeechActSourceStore) ResolveOrCapture(
	ctx context.Context,
	preparation StrictCLISpeechActPreparation,
) (StrictCLIDurableSourceResult, error) {
	if ctx == nil {
		return StrictCLIDurableSourceResult{},
			sqlitetransaction.ErrContextRequired
	}
	if store == nil || store.database == nil || store.capturer == nil ||
		store.writer == nil ||
		store.now == nil {
		return StrictCLIDurableSourceResult{}, fmt.Errorf(
			"strict CLI SpeechAct source store is unavailable",
		)
	}
	if err := preparation.Verify(); err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	recovered, found, err := store.loadExact(ctx, preparation)
	if err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	if found {
		return NewStrictCLIDurableSourceResult(
			StrictCLISpeechActReplayed{record: recovered},
		)
	}
	if err := store.rejectOrphanedGenericSource(ctx, preparation); err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	captured, err := store.capturer.Capture(
		ctx,
		preparation.PreparedSpeechAct(),
	)
	if err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	record, err := preparation.SealCaptured(captured)
	if err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.database)
	if err != nil {
		return StrictCLIDurableSourceResult{}, err
	}
	existing, found, err := loadStrictCLISpeechActSourceTx(
		ctx,
		transaction,
		preparation,
	)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(err, finish.Err())
	}
	if found {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(
			fmt.Errorf(
				"a concurrent durable TypeEnv SpeechAct capture won after terminal observation; retry the unchanged review to replay %s without another prompt",
				existing.Ref().String(),
			),
			finish.Err(),
		)
	}
	sourceResult, err := store.writer.RecordInTransaction(
		ctx,
		transaction,
		captured,
	)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(err, finish.Err())
	}
	if sourceResult.Kind() != authority.SpeechActSourceWriteStaged {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(
			fmt.Errorf(
				"new strict TypeEnv SpeechAct did not stage one new generic source: %s",
				sourceResult.Kind(),
			),
			finish.Err(),
		)
	}
	recordedAt := store.now().Round(0).UTC()
	if recordedAt.IsZero() {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(
			fmt.Errorf("strict TypeEnv source recording time is unavailable"),
			finish.Err(),
		)
	}
	if err := writeStrictCLIPreEffectAuthorityTx(
		ctx,
		transaction,
		preparation,
		record,
		recordedAt,
	); err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(err, finish.Err())
	}
	durable, found, err := loadStrictCLISpeechActSourceTx(
		ctx,
		transaction,
		preparation,
	)
	if err != nil || !found {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return StrictCLIDurableSourceResult{}, errors.Join(
			fmt.Errorf("strict TypeEnv source failed exact pre-commit reread"),
			err,
			finish.Err(),
		)
	}
	finish := transaction.Commit(ctx)
	if finish.Succeeded() {
		return NewStrictCLIDurableSourceResult(
			StrictCLISpeechActCaptured{record: durable},
		)
	}
	recovered, found, recoveryErr := store.loadExact(
		context.Background(),
		preparation,
	)
	if recoveryErr == nil && found {
		return NewStrictCLIDurableSourceResult(
			StrictCLISpeechActCaptured{record: recovered},
		)
	}
	return StrictCLIDurableSourceResult{}, errors.Join(
		fmt.Errorf(
			"strict TypeEnv source commit outcome is unknown; retry the unchanged review before performing another SpeechAct",
		),
		finish.Err(),
		recoveryErr,
	)
}

func (store *StrictCLISpeechActSourceStore) loadExact(
	ctx context.Context,
	preparation StrictCLISpeechActPreparation,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, bool, error) {
	transaction, err := sqlitetransaction.BeginRead(ctx, store.database)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	record, found, err := loadStrictCLISpeechActSourceTx(
		ctx,
		transaction,
		preparation,
	)
	if err != nil {
		finish := transaction.Rollback(context.WithoutCancel(ctx))
		return ProjectTypeEnvHeadSelectionSpeechActRecord{},
			false,
			errors.Join(err, finish.Err())
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{},
			false,
			finish.Err()
	}
	return record, found, nil
}

func (store *StrictCLISpeechActSourceStore) rejectOrphanedGenericSource(
	ctx context.Context,
	preparation StrictCLISpeechActPreparation,
) error {
	var count int
	err := store.database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM speech_acts WHERE speech_act_ref = ?",
		preparation.SpeechActRef().String(),
	).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf(
			"generic SpeechAct %s exists without its atomic TypeEnv source and Permission; repair or remove the inconsistent development ledger before retry",
			preparation.SpeechActRef().String(),
		)
	}
	return nil
}

func loadStrictCLISpeechActSourceTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	preparation StrictCLISpeechActPreparation,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, bool, error) {
	var recordDigestRaw string
	var sourceDigestRaw string
	var canonical []byte
	err := transaction.ScanOne(
		ctx,
		`SELECT speech_act_record_digest, source_digest, canonical_bytes
		FROM project_typeenv_head_selection_speech_act_records
		WHERE speech_act_ref = ?`,
		[]any{preparation.SpeechActRef().String()},
		[]any{&recordDigestRaw, &sourceDigestRaw, &canonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, nil
	}
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	recordDigest, err := authority.NewDigest(recordDigestRaw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	var speechActDigestRaw string
	err = transaction.ScanOne(
		ctx,
		`SELECT speech_act_digest FROM speech_acts WHERE speech_act_ref = ?`,
		[]any{preparation.SpeechActRef().String()},
		[]any{&speechActDigestRaw},
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{},
			false,
			fmt.Errorf("load generic strict SpeechAct identity: %w", err)
	}
	speechActDigest, err := authority.NewDigest(speechActDigestRaw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	recorded, err := authority.LoadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		preparation.SpeechActRef(),
		speechActDigest,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	record, err := preparation.DecodeRecorded(
		recorded,
		canonical,
		recordDigest,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	sourceDigest, digestOK := record.Source().Digest()
	if !digestOK || sourceDigest.String() != sourceDigestRaw {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{},
			false,
			fmt.Errorf("durable strict TypeEnv source digest differs from record")
	}
	permission := record.PermissionRecord()
	if err := requireExactCanonicalRowTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_permissions_v3",
		"permission_ref",
		"permission_digest",
		"canonical_bytes",
		permission.Ref().String(),
		permission.Digest().String(),
		permission.CanonicalJSON(),
	); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, false, err
	}
	return record, true, nil
}

func writeStrictCLIPreEffectAuthorityTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	preparation StrictCLISpeechActPreparation,
	record ProjectTypeEnvHeadSelectionSpeechActRecord,
	recordedAt time.Time,
) error {
	if err := writeStrictCLIConfigBasisTx(
		ctx,
		transaction,
		preparation.ConfigBasis(),
		recordedAt,
	); err != nil {
		return err
	}
	if err := writeStrictCLIModePolicyTx(
		ctx,
		transaction,
		preparation.ModePolicy(),
		recordedAt,
	); err != nil {
		return err
	}
	if err := writeStrictCLIRequestTx(
		ctx,
		transaction,
		preparation.Request(),
		recordedAt,
	); err != nil {
		return err
	}
	if err := writeStrictCLIContentTx(
		ctx,
		transaction,
		preparation.Content(),
		recordedAt,
	); err != nil {
		return err
	}
	if err := writeStrictCLISpeechActRecordTx(
		ctx,
		transaction,
		record,
		recordedAt,
	); err != nil {
		return err
	}
	return writeStrictCLIPermissionTx(ctx, transaction, record)
}

func writeStrictCLIConfigBasisTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	recordedAt time.Time,
) error {
	exact, err := exactCanonicalRowExistsTx(
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
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref, config_authority_basis_digest,
			project_id, authority_mode, config_carrier_ref,
			config_carrier_digest, canonical_bytes, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			basis.Ref().String(),
			basis.Digest().String(),
			basis.Project().String(),
			basis.Mode().String(),
			carrier.Ref().String(),
			carrier.Digest().String(),
			basis.CanonicalJSON(),
			formatTime(recordedAt),
		},
	)
	return err
}

func writeStrictCLIModePolicyTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	policy StrictCLISpeechActAuthorityPolicy,
	recordedAt time.Time,
) error {
	exact, err := exactCanonicalRowExistsTx(
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
	resolver := policy.ResolverPolicy()
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_mode_policies (
			mode_policy_ref, mode_policy_digest, project_id, authority_mode,
			config_authority_basis_ref, config_authority_basis_digest,
			resolver_policy_ref, resolver_policy_edition,
			resolver_policy_digest, canonical_bytes, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
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
			formatTime(recordedAt),
		},
	)
	return err
}

func writeStrictCLIRequestTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	recordedAt time.Time,
) error {
	exact, err := exactCanonicalRowExistsTx(
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
	extensions, extensionsDigest, err := strictCLIOrderedExtensions(
		target.OrderedExtensions(),
	)
	if err != nil {
		return err
	}
	head, err := request.Head()
	if err != nil {
		return err
	}
	expectedGraphRevision, err := strictCLIStoredRevision(
		"expected graph revision",
		request.ExpectedGraphRevision().Value(),
	)
	if err != nil {
		return err
	}
	predecessorKind := ""
	var priorHead any
	var priorRevision any
	var priorComposite any
	switch predecessor := request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		predecessorKind = "genesis"
	case projecttypeenvselection.TransitionStagePredecessor:
		predecessorKind = "transition"
		priorHead = predecessor.Head().String()
		priorRevisionValue, revisionErr := strictCLIStoredRevision(
			"prior head revision",
			predecessor.HeadRevision().Value(),
		)
		if revisionErr != nil {
			return revisionErr
		}
		priorRevision = priorRevisionValue
		priorComposite = predecessor.SelectedComposite().String()
	default:
		return fmt.Errorf("strict CLI request predecessor is invalid")
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_requests (
			request_ref, request_digest, request_schema, project_id, head_ref,
			predecessor_kind, no_prior_head_proof_ref,
			no_prior_head_proof_digest, prior_head_ref, prior_head_revision,
			prior_selected_composite_ref, base_type_env_ref,
			ordered_extension_refs_digest, canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref, selected_composite_ref, stage_ref,
			stage_digest, expected_graph_revision, original_idempotency_key,
			canonical_bytes, recorded_at
		) VALUES (
			?, ?, 'haft.project-typeenv.head-selection-request.v2', ?, ?, ?,
			NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		[]any{
			request.Ref().String(),
			request.Ref().Digest().String(),
			request.Project().String(),
			head.String(),
			predecessorKind,
			priorHead,
			priorRevision,
			priorComposite,
			target.Base().String(),
			extensionsDigest,
			extensions,
			target.RuntimeBasis().String(),
			target.VerifiedComposite().String(),
			target.Stage().String(),
			target.Stage().Digest().String(),
			expectedGraphRevision,
			request.IdempotencyKey().String(),
			request.CanonicalBytes(),
			formatTime(recordedAt),
		},
	)
	return err
}

func strictCLIStoredRevision(label string, value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds SQLite integer range", label)
	}
	return int64(value), nil // #nosec G115 -- value is bounded by math.MaxInt64 above.
}

func writeStrictCLIContentTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	recordedAt time.Time,
) error {
	exact, err := exactCanonicalRowExistsTx(
		ctx,
		transaction,
		"project_typeenv_head_selection_authorization_contents",
		"content_ref",
		"content_digest",
		"canonical_bytes",
		content.DescriptionRef().String(),
		content.Digest().String(),
		content.CanonicalJSON(),
	)
	if err != nil || exact {
		return err
	}
	window := content.ValidityWindow()
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_authorization_contents (
			content_ref, content_ref_kind, content_digest, project_id,
			request_ref, request_digest, judgement_context_ref, action_kind,
			valid_from, valid_until, canonical_bytes, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			content.DescriptionRef().String(),
			string(content.DescriptionRef().Kind()),
			content.Digest().String(),
			content.Project().String(),
			content.Request().Ref().String(),
			content.Request().Ref().Digest().String(),
			content.JudgementContext().String(),
			content.Action().String(),
			formatTime(window.From()),
			formatTime(window.Until()),
			content.CanonicalJSON(),
			formatTime(recordedAt),
		},
	)
	return err
}

func writeStrictCLISpeechActRecordTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record ProjectTypeEnvHeadSelectionSpeechActRecord,
	recordedAt time.Time,
) error {
	exact, err := exactCanonicalRowExistsTx(
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
	if err != nil || exact {
		return err
	}
	source := record.Source()
	speechActRef, speechActOK := source.SpeechActRef()
	workRef, workOK := source.WorkRef()
	sourceDigest, digestOK := source.Digest()
	if !speechActOK || !workOK || !digestOK {
		return fmt.Errorf("strict CLI source coordinates are unavailable")
	}
	content := record.Content()
	permission := record.PermissionRecord()
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_speech_act_records (
			speech_act_record_ref, speech_act_record_digest, project_id,
			speech_act_ref, human_work_ref, source_digest, content_ref,
			content_digest, request_ref, request_digest, permission_ref,
			permission_digest, canonical_bytes, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		[]any{
			record.Ref().String(),
			record.Digest().String(),
			content.Project().String(),
			speechActRef.String(),
			workRef.String(),
			sourceDigest.String(),
			content.DescriptionRef().String(),
			content.Digest().String(),
			content.Request().Ref().String(),
			content.Request().Ref().Digest().String(),
			permission.Ref().String(),
			permission.Digest().String(),
			record.CanonicalJSON(),
			formatTime(recordedAt),
		},
	)
	return err
}

func writeStrictCLIPermissionTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record ProjectTypeEnvHeadSelectionSpeechActRecord,
) error {
	permission := record.PermissionRecord()
	exact, err := exactCanonicalRowExistsTx(
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
	if err != nil || exact {
		return err
	}
	subject := permission.Subject()
	subjectPolicy := subject.AssignmentPolicy()
	subjectWindow := subject.AssignmentWindow()
	scope := permission.Scope()
	referents, err := strictCLIPermissionReferents(permission.Referents())
	if err != nil {
		return err
	}
	content := record.Content()
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO project_typeenv_head_selection_permissions_v3 (
			permission_ref, permission_digest, project_id,
			subject_role_assignment_ref, subject_role_assignment_digest,
			subject_schema, subject_holder_system_ref, subject_holder_kind,
			subject_role_ref, subject_context_ref, subject_assignment_from,
			subject_assignment_until, subject_assignment_policy_ref,
			subject_assignment_policy_digest,
			subject_assignment_policy_edition_ref,
			subject_assignment_policy_selection, subject_system_admission_ref,
			subject_system_admission_digest, subject_role_admission_ref,
			subject_role_admission_digest,
			subject_assignment_justification_ref,
			subject_assignment_justification_digest,
			subject_assignment_provenance_ref,
			subject_assignment_provenance_digest,
			subject_authorization_description_kind,
			subject_authorization_description_ref,
			subject_authorization_content_digest, subject_canonical_bytes,
			modality, claim_scope_ref, claim_scope_digest, context_policy_ref,
			context_policy_digest, referents_canonical_bytes, effective_from,
			validity_until, speech_act_record_ref, speech_act_record_digest,
			content_ref, content_digest, request_ref, request_digest,
			canonical_bytes
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		[]any{
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
			formatTime(subjectWindow.From()),
			formatTime(subjectWindow.Until()),
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
			formatTime(permission.EffectiveFrom()),
			formatTime(permission.ValidityUntil()),
			record.Ref().String(),
			record.Digest().String(),
			content.DescriptionRef().String(),
			content.Digest().String(),
			content.Request().Ref().String(),
			content.Request().Ref().Digest().String(),
			permission.CanonicalJSON(),
		},
	)
	return err
}

func strictCLIOrderedExtensions(
	extensions []typedmemory.TypeEnvExtensionRef,
) ([]byte, string, error) {
	values := make([]string, len(extensions))
	for index := range extensions {
		values[index] = extensions[index].String()
	}
	canonical, err := json.Marshal(values)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	return canonical, "sha256:" + hex.EncodeToString(sum[:]), nil
}

func strictCLIPermissionReferents(
	referents []ProjectTypeEnvHeadSelectionPermissionReferent,
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
	return json.Marshal(values)
}

func exactCanonicalRowExistsTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	table string,
	refColumn string,
	digestColumn string,
	canonicalColumn string,
	ref string,
	digest string,
	canonical []byte,
) (bool, error) {
	statement := "SELECT " + refColumn + ", " + digestColumn + ", " +
		canonicalColumn +
		" FROM " + table + " WHERE " + refColumn + " = ? OR " +
		digestColumn + " = ?"
	var storedRef string
	var storedDigest string
	var storedCanonical []byte
	err := transaction.ScanOne(
		ctx,
		statement,
		[]any{ref, digest},
		[]any{&storedRef, &storedDigest, &storedCanonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if storedRef != ref ||
		storedDigest != digest ||
		!bytes.Equal(storedCanonical, canonical) {
		return false, fmt.Errorf(
			"%s identity already binds different canonical material",
			table,
		)
	}
	return true, nil
}

func requireExactCanonicalRowTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	table string,
	refColumn string,
	digestColumn string,
	canonicalColumn string,
	ref string,
	digest string,
	canonical []byte,
) error {
	exact, err := exactCanonicalRowExistsTx(
		ctx,
		transaction,
		table,
		refColumn,
		digestColumn,
		canonicalColumn,
		ref,
		digest,
		canonical,
	)
	if err != nil {
		return err
	}
	if !exact {
		return fmt.Errorf("%s exact canonical row is missing", table)
	}
	return nil
}
