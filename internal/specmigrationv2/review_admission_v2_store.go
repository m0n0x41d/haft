package specmigrationv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const insertMigrationReviewContentV2SQL = `INSERT INTO migration_review_acceptance_contents (
	review_content_ref, review_content_digest, project_root,
	packet_digest, packet_carrier_digest,
	partition_audit_schema, partition_audit_status, partition_audit_digest,
	source_carrier, source_digest, target_carrier_digests_json,
	fpf_revision, semantic_zero_pass_carrier, semantic_zero_pass_digest,
	lifecycle_intent_json, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertMigrationReviewAdmissionV2SQL = `INSERT INTO migration_review_admissions_v2 (
	admission_ref, admission_digest, project_root, packet_carrier_digest,
	review_content_ref, review_content_digest, review_text, review_digest,
	capture_carrier_ref, capture_carrier_digest,
	speech_act_ref, speech_act_digest,
	context_policy_ref, context_policy_digest, act_type_ref,
	method_ref, method_description_ref, method_description_digest,
	bounded_context_ref, institutional_effect_rule_ref,
	admission_json, admitted_at, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertMigrationReviewEffectV1SQL = `INSERT INTO migration_review_instituted_effects (
	effect_digest, project_root, speech_act_ref, speech_act_digest,
	admission_ref, admission_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

const selectMigrationReviewAdmissionV2Columns = `
SELECT
	content.review_content_ref, content.review_content_digest, content.project_root,
	content.packet_digest, content.packet_carrier_digest,
	content.partition_audit_schema, content.partition_audit_status, content.partition_audit_digest,
	content.source_carrier, content.source_digest, content.target_carrier_digests_json,
	content.fpf_revision, content.semantic_zero_pass_carrier, content.semantic_zero_pass_digest,
	content.lifecycle_intent_json, content.canonical_json, content.recorded_at,
		admission.admission_ref, admission.admission_digest, admission.project_root,
		admission.packet_carrier_digest, admission.review_content_ref, admission.review_content_digest,
		admission.review_text, admission.review_digest,
		admission.capture_carrier_ref, admission.capture_carrier_digest,
		admission.speech_act_ref, admission.speech_act_digest,
		admission.context_policy_ref, admission.context_policy_digest, admission.act_type_ref,
		admission.method_ref, admission.method_description_ref, admission.method_description_digest,
		admission.bounded_context_ref, admission.institutional_effect_rule_ref,
		admission.admission_json, admission.admitted_at, admission.recorded_at,
		capture.review_text,
		act.context_policy_ref, act.context_policy_digest, act.act_type_ref,
		act.method_ref, act.method_description_ref, act.method_description_digest,
		act.bounded_context_ref,
		policy.context_policy_ref, policy.context_policy_digest,
		policy.bounded_context_ref, policy.recognized_act_type_ref,
		policy.institutional_effect_rule_ref,
		method.method_ref, method.method_description_ref, method.method_description_digest,
		effect.effect_digest, effect.project_root, effect.speech_act_ref, effect.speech_act_digest,
	effect.admission_ref, effect.admission_digest, effect.canonical_json, effect.recorded_at
FROM migration_review_admissions_v2 admission
JOIN migration_review_acceptance_contents content
	ON content.review_content_ref = admission.review_content_ref
	AND content.review_content_digest = admission.review_content_digest
	JOIN migration_review_instituted_effects effect
		ON effect.admission_ref = admission.admission_ref
		AND effect.admission_digest = admission.admission_digest
	JOIN terminal_capture_records capture
		ON capture.capture_carrier_ref = admission.capture_carrier_ref
		AND capture.capture_carrier_digest = admission.capture_carrier_digest
	JOIN speech_acts act
		ON act.speech_act_ref = admission.speech_act_ref
		AND act.speech_act_digest = admission.speech_act_digest
	JOIN speech_act_context_policies policy
		ON policy.context_policy_ref = act.context_policy_ref
		AND policy.context_policy_digest = act.context_policy_digest
	JOIN speech_act_method_descriptions method
		ON method.method_description_ref = act.method_description_ref
		AND method.method_description_digest = act.method_description_digest`

const selectMigrationReviewAdmissionV2ByCurrent = selectMigrationReviewAdmissionV2Columns + `
WHERE admission.project_root = ?
AND admission.packet_carrier_digest = ?
AND content.partition_audit_digest = ?`

const selectMigrationReviewAdmissionV2ByRef = selectMigrationReviewAdmissionV2Columns + `
WHERE admission.admission_ref = ?
AND admission.admission_digest = ?`

const selectMigrationReviewAdmissionV2ByStableRef = selectMigrationReviewAdmissionV2Columns + `
WHERE admission.admission_ref = ?`

type migrationReviewAdmissionStore struct {
	database *sql.DB
}

func openMigrationReviewAdmissionStore(
	database *sql.DB,
) (migrationReviewAdmissionStore, error) {
	if database == nil {
		return migrationReviewAdmissionStore{}, fmt.Errorf("migration-review admission database is required")
	}
	if err := verifyMigrationReviewProtocolV2Schema(database); err != nil {
		return migrationReviewAdmissionStore{}, err
	}
	return migrationReviewAdmissionStore{database: database}, nil
}

func (store migrationReviewAdmissionStore) insert(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	record migrationReviewAdmissionRecordV2,
	recordedAt time.Time,
) error {
	if err := validateMigrationReviewAdmissionRecordV2(record); err != nil {
		return err
	}
	carrierJSON, err := marshalMigrationReviewFragment(
		canonicalReviewCarrierDTOs(record.targetCarrierDigests),
	)
	if err != nil {
		return err
	}
	lifecycleJSON, err := marshalMigrationReviewFragment(
		canonicalLifecycleIntentDTOs(record.lifecycleIntent),
	)
	if err != nil {
		return err
	}
	contentArguments := migrationReviewContentInsertArguments(
		record,
		carrierJSON,
		lifecycleJSON,
		recordedAt,
	)
	_, err = transaction.Execute(ctx, insertMigrationReviewContentV2SQL, contentArguments)
	if err != nil {
		return fmt.Errorf("record migration-review acceptance content: %w", err)
	}
	admissionArguments := migrationReviewAdmissionInsertArguments(record, recordedAt)
	_, err = transaction.Execute(ctx, insertMigrationReviewAdmissionV2SQL, admissionArguments)
	if err != nil {
		return fmt.Errorf("record migration-review admission: %w", err)
	}
	effectArguments := migrationReviewEffectInsertArguments(record, recordedAt)
	_, err = transaction.Execute(ctx, insertMigrationReviewEffectV1SQL, effectArguments)
	if err != nil {
		return fmt.Errorf("record migration-review instituted effect: %w", err)
	}
	return nil
}

func migrationReviewContentInsertArguments(
	record migrationReviewAdmissionRecordV2,
	carrierJSON []byte,
	lifecycleJSON []byte,
	recordedAt time.Time,
) []any {
	content := record.content
	packet := content.carrier.Packet()
	basis := content.carrier.ReviewBasis()
	zeroPass := basis.SemanticZeroPass()
	return []any{
		content.ref,
		content.digest.String(),
		content.root.String(),
		content.carrier.PacketDigest().String(),
		content.carrier.CarrierDigest().String(),
		content.audit.Schema(),
		string(content.audit.Status()),
		content.audit.Digest().String(),
		packet.Source().Carrier().String(),
		packet.Source().Digest().String(),
		string(carrierJSON),
		basis.FPFRevision().String(),
		zeroPass.Carrier().String(),
		zeroPass.Digest().String(),
		string(lifecycleJSON),
		string(content.canonical),
		formatReviewTime(recordedAt),
	}
}

func migrationReviewAdmissionInsertArguments(
	record migrationReviewAdmissionRecordV2,
	recordedAt time.Time,
) []any {
	return []any{
		record.reviewRef.String(),
		record.admissionDigest.String(),
		record.projectRoot.String(),
		record.packetCarrierDigest.String(),
		record.content.ref,
		record.content.digest.String(),
		record.reviewText,
		record.reviewDigest.String(),
		record.captureRef,
		record.captureDigest.String(),
		record.speechActRef.String(),
		record.speechActDigest.String(),
		record.protocol.contextPolicyRef,
		record.protocol.contextPolicyDigest.String(),
		record.protocol.actTypeRef,
		record.protocol.methodRef,
		record.protocol.methodDescriptionRef,
		record.protocol.methodDescriptionDigest.String(),
		record.protocol.boundedContextRef,
		record.protocol.effectRuleRef,
		string(record.canonical),
		formatReviewTime(record.admittedAt),
		formatReviewTime(recordedAt),
	}
}

func migrationReviewEffectInsertArguments(
	record migrationReviewAdmissionRecordV2,
	recordedAt time.Time,
) []any {
	return []any{
		record.effectDigest.String(),
		record.projectRoot.String(),
		record.speechActRef.String(),
		record.speechActDigest.String(),
		record.reviewRef.String(),
		record.admissionDigest.String(),
		string(record.effectCanonical),
		formatReviewTime(recordedAt),
	}
}

func (store migrationReviewAdmissionStore) loadCurrent(
	ctx context.Context,
	root ApplyProjectRoot,
	carrier PacketCarrierDigest,
	audit PacketPartitionAuditBinding,
) (migrationReviewAdmissionRecordV2, bool, error) {
	arguments := []any{root.String(), carrier.String(), audit.Digest().String()}
	return store.load(ctx, selectMigrationReviewAdmissionV2ByCurrent, arguments)
}

func (store migrationReviewAdmissionStore) loadByRef(
	ctx context.Context,
	ref ReviewRef,
	digest SHA256,
) (migrationReviewAdmissionRecordV2, bool, error) {
	arguments := []any{ref.String(), digest.String()}
	return store.load(ctx, selectMigrationReviewAdmissionV2ByRef, arguments)
}

func (store migrationReviewAdmissionStore) loadInTransactionByStableRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref ReviewRef,
) (migrationReviewAdmissionRecordV2, bool, error) {
	row := migrationReviewAdmissionV2Row{}
	arguments := []any{ref.String()}
	err := transaction.ScanOne(
		ctx,
		selectMigrationReviewAdmissionV2ByStableRef,
		arguments,
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return migrationReviewAdmissionRecordV2{}, false, nil
	}
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, false, fmt.Errorf("load migration-review admission in transaction: %w", err)
	}
	record, err := migrationReviewAdmissionRecordV2FromRow(row)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, false, err
	}
	return record, true, nil
}

func (store migrationReviewAdmissionStore) load(
	ctx context.Context,
	statement string,
	arguments []any,
) (migrationReviewAdmissionRecordV2, bool, error) {
	transaction, err := sqlitetransaction.BeginRead(ctx, store.database)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, false, fmt.Errorf("begin migration-review admission read: %w", err)
	}
	row := migrationReviewAdmissionV2Row{}
	err = transaction.ScanOne(ctx, statement, arguments, row.scanTargets())
	finish := transaction.Rollback(context.Background())
	if finish.Err() != nil {
		return migrationReviewAdmissionRecordV2{}, false, fmt.Errorf("close migration-review admission read: %w", finish.Err())
	}
	if errors.Is(err, sql.ErrNoRows) {
		return migrationReviewAdmissionRecordV2{}, false, nil
	}
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, false, fmt.Errorf("load migration-review admission: %w", err)
	}
	record, err := migrationReviewAdmissionRecordV2FromRow(row)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, false, err
	}
	if err := verifyRecordedSourceForAdmission(ctx, store.database, record); err != nil {
		return migrationReviewAdmissionRecordV2{}, false, err
	}
	return record, true, nil
}

type migrationReviewAdmissionV2Row struct {
	contentRef                 string
	contentDigest              string
	contentRoot                string
	packetDigest               string
	contentPacketCarrierDigest string
	auditSchema                string
	auditStatus                string
	auditDigest                string
	sourceCarrier              string
	sourceDigest               string
	targetCarrierDigestsJSON   string
	fpfRevision                string
	semanticZeroPassCarrier    string
	semanticZeroPassDigest     string
	lifecycleIntentJSON        string
	contentJSON                string
	contentRecordedAt          string
	admissionRef               string
	admissionDigest            string
	admissionRoot              string
	admissionPacketCarrier     string
	admissionContentRef        string
	admissionContentDigest     string
	admissionReviewText        string
	reviewDigest               string
	captureRef                 string
	captureDigest              string
	speechActRef               string
	speechActDigest            string
	admissionContextPolicyRef  string
	admissionContextPolicyHash string
	admissionActTypeRef        string
	admissionMethodRef         string
	admissionMethodDescRef     string
	admissionMethodDescHash    string
	admissionBoundedContextRef string
	admissionEffectRuleRef     string
	admissionJSON              string
	admittedAt                 string
	admissionRecordedAt        string
	sourceReviewText           string
	sourceContextPolicyRef     string
	sourceContextPolicyHash    string
	sourceActTypeRef           string
	sourceMethodRef            string
	sourceMethodDescRef        string
	sourceMethodDescHash       string
	sourceBoundedContextRef    string
	policyRef                  string
	policyDigest               string
	policyBoundedContextRef    string
	policyActTypeRef           string
	policyEffectRuleRef        string
	methodRef                  string
	methodDescriptionRef       string
	methodDescriptionDigest    string
	effectDigest               string
	effectRoot                 string
	effectSpeechActRef         string
	effectSpeechActDigest      string
	effectAdmissionRef         string
	effectAdmissionDigest      string
	effectJSON                 string
	effectRecordedAt           string
}

func (row *migrationReviewAdmissionV2Row) scanTargets() []any {
	return []any{
		&row.contentRef,
		&row.contentDigest,
		&row.contentRoot,
		&row.packetDigest,
		&row.contentPacketCarrierDigest,
		&row.auditSchema,
		&row.auditStatus,
		&row.auditDigest,
		&row.sourceCarrier,
		&row.sourceDigest,
		&row.targetCarrierDigestsJSON,
		&row.fpfRevision,
		&row.semanticZeroPassCarrier,
		&row.semanticZeroPassDigest,
		&row.lifecycleIntentJSON,
		&row.contentJSON,
		&row.contentRecordedAt,
		&row.admissionRef,
		&row.admissionDigest,
		&row.admissionRoot,
		&row.admissionPacketCarrier,
		&row.admissionContentRef,
		&row.admissionContentDigest,
		&row.admissionReviewText,
		&row.reviewDigest,
		&row.captureRef,
		&row.captureDigest,
		&row.speechActRef,
		&row.speechActDigest,
		&row.admissionContextPolicyRef,
		&row.admissionContextPolicyHash,
		&row.admissionActTypeRef,
		&row.admissionMethodRef,
		&row.admissionMethodDescRef,
		&row.admissionMethodDescHash,
		&row.admissionBoundedContextRef,
		&row.admissionEffectRuleRef,
		&row.admissionJSON,
		&row.admittedAt,
		&row.admissionRecordedAt,
		&row.sourceReviewText,
		&row.sourceContextPolicyRef,
		&row.sourceContextPolicyHash,
		&row.sourceActTypeRef,
		&row.sourceMethodRef,
		&row.sourceMethodDescRef,
		&row.sourceMethodDescHash,
		&row.sourceBoundedContextRef,
		&row.policyRef,
		&row.policyDigest,
		&row.policyBoundedContextRef,
		&row.policyActTypeRef,
		&row.policyEffectRuleRef,
		&row.methodRef,
		&row.methodDescriptionRef,
		&row.methodDescriptionDigest,
		&row.effectDigest,
		&row.effectRoot,
		&row.effectSpeechActRef,
		&row.effectSpeechActDigest,
		&row.effectAdmissionRef,
		&row.effectAdmissionDigest,
		&row.effectJSON,
		&row.effectRecordedAt,
	}
}

func migrationReviewAdmissionRecordV2FromRow(
	row migrationReviewAdmissionV2Row,
) (migrationReviewAdmissionRecordV2, error) {
	contentDTO, err := decodeMigrationReviewAcceptanceContent([]byte(row.contentJSON))
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, fmt.Errorf("decode migration-review acceptance content: %w", err)
	}
	if contentDTO.Schema != migrationReviewAcceptanceContentSchemaV2 {
		return migrationReviewAdmissionRecordV2{}, fmt.Errorf("unsupported migration-review acceptance content schema %q", contentDTO.Schema)
	}
	contentDigest, err := NewSHA256(row.contentDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	if !DigestBytes([]byte(row.contentJSON)).Equal(contentDigest) {
		return migrationReviewAdmissionRecordV2{}, fmt.Errorf("migration-review acceptance content digest does not bind canonical JSON")
	}
	if err := validateMigrationReviewContentRow(contentDTO, row); err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	review, err := admittedMigrationReviewFromV2Row(contentDTO, row)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	reviewDigest, err := NewSHA256(row.reviewDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	captureDigest, err := NewSHA256(row.captureDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	admittedAt, err := parseReviewTime(row.admittedAt)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	protocol, err := migrationReviewProtocolPinsFromRow(row)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	record := migrationReviewAdmissionRecordV2{
		admittedMigrationReview: review,
		content: migrationReviewAcceptanceContent{
			ref:       row.contentRef,
			digest:    contentDigest,
			root:      review.projectRoot,
			audit:     review.partitionAudit,
			canonical: []byte(row.contentJSON),
		},
		reviewText:    row.admissionReviewText,
		reviewDigest:  reviewDigest,
		captureRef:    row.captureRef,
		captureDigest: captureDigest,
		protocol:      protocol,
		admittedAt:    admittedAt,
		canonical:     []byte(row.admissionJSON),
	}
	effectDigest, err := NewSHA256(row.effectDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	record.effectDigest = effectDigest
	record.effectCanonical = []byte(row.effectJSON)
	if err := validateMigrationReviewAdmissionRow(record, row); err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	return record, nil
}

func migrationReviewProtocolPinsFromRow(
	row migrationReviewAdmissionV2Row,
) (migrationReviewProtocolPins, error) {
	policyDigest, err := NewSHA256(row.admissionContextPolicyHash)
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	methodDigest, err := NewSHA256(row.admissionMethodDescHash)
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	return migrationReviewProtocolPins{
		contextPolicyRef:        row.admissionContextPolicyRef,
		contextPolicyDigest:     policyDigest,
		actTypeRef:              row.admissionActTypeRef,
		methodRef:               row.admissionMethodRef,
		methodDescriptionRef:    row.admissionMethodDescRef,
		methodDescriptionDigest: methodDigest,
		boundedContextRef:       row.admissionBoundedContextRef,
		effectRuleRef:           row.admissionEffectRuleRef,
	}, nil
}

func admittedMigrationReviewFromV2Row(
	content migrationReviewAcceptanceContentJSONV2,
	row migrationReviewAdmissionV2Row,
) (admittedMigrationReview, error) {
	ref, err := newReviewRef(row.admissionRef)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	admissionDigest, err := NewSHA256(row.admissionDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	speechActRef, err := NewSemanticReviewSpeechActRef(row.speechActRef)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	speechActDigest, err := NewSHA256(row.speechActDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	root, err := NewApplyProjectRoot(content.ProjectRoot)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	packetDigest, err := NewPacketDigest(content.PacketDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	carrierDigest, err := NewSHA256(content.PacketCarrierDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	audit, err := packetPartitionAuditBindingFromRow(
		content.PartitionAuditSchema,
		content.PartitionAuditStatus,
		content.PartitionAuditDigest,
	)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	sourceCarrier, err := NewSourceCarrierID(content.SourceCarrier)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	sourceDigest, err := NewSourceDigest(content.SourceDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	carriers, err := decodeReviewCarrierDigests(content.TargetCarrierDigests)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	fpfRevision, err := newFPFRevision(content.FPFRevision)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	zeroPassCarrier, err := NewTargetCarrierID(content.SemanticZeroPassCarrier)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	zeroPassDigest, err := NewSHA256(content.SemanticZeroPassDigest)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	lifecycle, err := decodeLifecycleIntent(content.LifecycleIntent)
	if err != nil {
		return admittedMigrationReview{}, err
	}
	review := admittedMigrationReview{
		reviewRef:            ref,
		admissionDigest:      admissionDigest,
		speechActRef:         speechActRef,
		speechActDigest:      speechActDigest,
		projectRoot:          root,
		packetDigest:         packetDigest,
		packetCarrierDigest:  PacketCarrierDigest{value: carrierDigest},
		partitionAudit:       audit,
		sourceCarrier:        sourceCarrier,
		sourceDigest:         sourceDigest,
		targetCarrierDigests: carriers,
		fpfRevision:          fpfRevision,
		semanticZeroPass: SemanticZeroPassBinding{
			carrier: zeroPassCarrier,
			digest:  zeroPassDigest,
		},
		lifecycleIntent: lifecycle,
	}
	return review, validateAdmittedMigrationReview(review)
}

func validateMigrationReviewContentRow(
	dto migrationReviewAcceptanceContentJSONV2,
	row migrationReviewAdmissionV2Row,
) error {
	carrierJSON, err := marshalMigrationReviewFragment(dto.TargetCarrierDigests)
	if err != nil {
		return err
	}
	lifecycleJSON, err := marshalMigrationReviewFragment(dto.LifecycleIntent)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: dto.ReviewContentRef == row.contentRef, name: "content ref"},
		{matches: dto.ProjectRoot == row.contentRoot, name: "content root"},
		{matches: dto.PacketDigest == row.packetDigest, name: "packet digest"},
		{matches: dto.PacketCarrierDigest == row.contentPacketCarrierDigest, name: "packet-carrier digest"},
		{matches: dto.PartitionAuditSchema == row.auditSchema, name: "audit schema"},
		{matches: dto.PartitionAuditStatus == row.auditStatus, name: "audit status"},
		{matches: dto.PartitionAuditDigest == row.auditDigest, name: "audit digest"},
		{matches: dto.SourceCarrier == row.sourceCarrier, name: "source carrier"},
		{matches: dto.SourceDigest == row.sourceDigest, name: "source digest"},
		{matches: string(carrierJSON) == row.targetCarrierDigestsJSON, name: "review carriers"},
		{matches: dto.FPFRevision == row.fpfRevision, name: "FPF revision"},
		{matches: dto.SemanticZeroPassCarrier == row.semanticZeroPassCarrier, name: "semantic zero-pass carrier"},
		{matches: dto.SemanticZeroPassDigest == row.semanticZeroPassDigest, name: "semantic zero-pass digest"},
		{matches: string(lifecycleJSON) == row.lifecycleIntentJSON, name: "lifecycle intent"},
		{matches: row.admissionReviewText == migrationReviewTextFromDTO(dto), name: "canonical review text"},
		{matches: row.sourceReviewText == row.admissionReviewText, name: "source review text"},
	}
	return firstMigrationReviewMismatch(checks, "acceptance content")
}

func validateMigrationReviewAdmissionRow(
	record migrationReviewAdmissionRecordV2,
	row migrationReviewAdmissionV2Row,
) error {
	admissionDTO := migrationReviewAdmissionJSONV2{}
	if err := decodeCanonicalMigrationReviewJSON([]byte(row.admissionJSON), &admissionDTO); err != nil {
		return fmt.Errorf("decode migration-review admission: %w", err)
	}
	if admissionDTO.Schema != migrationReviewAdmissionSchemaV2 {
		return fmt.Errorf("unsupported migration-review admission schema %q", admissionDTO.Schema)
	}
	expectedAdmission := migrationReviewAdmissionDTO(record)
	if admissionDTO != expectedAdmission {
		return fmt.Errorf("migration-review admission JSON does not bind exact row values")
	}
	if err := validateMigrationReviewProtocolRow(record, row); err != nil {
		return err
	}
	if !DigestBytes([]byte(row.admissionJSON)).Equal(record.admissionDigest) {
		return fmt.Errorf("migration-review admission digest does not bind canonical JSON")
	}
	effectDTO := migrationReviewEffectJSONV1{}
	if err := decodeCanonicalMigrationReviewJSON([]byte(row.effectJSON), &effectDTO); err != nil {
		return fmt.Errorf("decode migration-review instituted effect: %w", err)
	}
	expectedEffectDigest, expectedEffectJSON, err := encodeMigrationReviewEffectV1(record)
	if err != nil {
		return err
	}
	if !expectedEffectDigest.Equal(record.effectDigest) || !slices.Equal(expectedEffectJSON, []byte(row.effectJSON)) {
		return fmt.Errorf("migration-review instituted effect does not bind exact admission")
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: row.contentRoot == row.admissionRoot && row.admissionRoot == row.effectRoot, name: "project root"},
		{matches: row.contentPacketCarrierDigest == row.admissionPacketCarrier, name: "packet-carrier digest"},
		{matches: row.contentRef == row.admissionContentRef, name: "content ref"},
		{matches: row.contentDigest == row.admissionContentDigest, name: "content digest"},
		{matches: row.speechActRef == row.effectSpeechActRef, name: "SpeechAct ref"},
		{matches: row.speechActDigest == row.effectSpeechActDigest, name: "SpeechAct digest"},
		{matches: row.admissionRef == row.effectAdmissionRef, name: "effect admission ref"},
		{matches: row.admissionDigest == row.effectAdmissionDigest, name: "effect admission digest"},
		{matches: effectDTO.EffectDigest == row.effectDigest, name: "effect digest"},
	}
	if err := firstMigrationReviewMismatch(checks, "admission closure"); err != nil {
		return err
	}
	return validateMigrationReviewRecordTimes(record, row)
}

func validateMigrationReviewProtocolRow(
	record migrationReviewAdmissionRecordV2,
	row migrationReviewAdmissionV2Row,
) error {
	expected, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		return err
	}
	if !sameMigrationReviewProtocolPins(record.protocol, expected) {
		return fmt.Errorf("migration-review admission row does not bind the sealed protocol")
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: row.sourceContextPolicyRef == expected.contextPolicyRef, name: "source context policy ref"},
		{matches: row.sourceContextPolicyHash == expected.contextPolicyDigest.String(), name: "source context policy digest"},
		{matches: row.sourceActTypeRef == expected.actTypeRef, name: "source act type"},
		{matches: row.sourceMethodRef == expected.methodRef, name: "source method"},
		{matches: row.sourceMethodDescRef == expected.methodDescriptionRef, name: "source MethodDescription ref"},
		{matches: row.sourceMethodDescHash == expected.methodDescriptionDigest.String(), name: "source MethodDescription digest"},
		{matches: row.sourceBoundedContextRef == expected.boundedContextRef, name: "source bounded context"},
		{matches: row.policyRef == expected.contextPolicyRef, name: "policy ref"},
		{matches: row.policyDigest == expected.contextPolicyDigest.String(), name: "policy digest"},
		{matches: row.policyBoundedContextRef == expected.boundedContextRef, name: "policy bounded context"},
		{matches: row.policyActTypeRef == expected.actTypeRef, name: "policy act type"},
		{matches: row.policyEffectRuleRef == expected.effectRuleRef, name: "policy effect rule"},
		{matches: row.methodRef == expected.methodRef, name: "method ref"},
		{matches: row.methodDescriptionRef == expected.methodDescriptionRef, name: "method description ref"},
		{matches: row.methodDescriptionDigest == expected.methodDescriptionDigest.String(), name: "method description digest"},
	}
	return firstMigrationReviewMismatch(checks, "sealed protocol")
}

func validateMigrationReviewRecordTimes(
	record migrationReviewAdmissionRecordV2,
	row migrationReviewAdmissionV2Row,
) error {
	contentRecordedAt, err := parseReviewTime(row.contentRecordedAt)
	if err != nil {
		return err
	}
	admissionRecordedAt, err := parseReviewTime(row.admissionRecordedAt)
	if err != nil {
		return err
	}
	effectRecordedAt, err := parseReviewTime(row.effectRecordedAt)
	if err != nil {
		return err
	}
	if contentRecordedAt.Before(record.admittedAt) ||
		!contentRecordedAt.Equal(admissionRecordedAt) ||
		!contentRecordedAt.Equal(effectRecordedAt) {
		return fmt.Errorf("migration-review durable record times are inconsistent")
	}
	return nil
}

func verifyRecordedSourceForAdmission(
	ctx context.Context,
	database *sql.DB,
	record migrationReviewAdmissionRecordV2,
) error {
	ref, err := authority.NewSpeechActRef(record.speechActRef.String())
	if err != nil {
		return err
	}
	digest, err := authority.NewDigest(record.speechActDigest.String())
	if err != nil {
		return err
	}
	source, err := authority.LoadRecordedSpeechActSource(ctx, database, ref, digest)
	if err != nil {
		return fmt.Errorf("load migration-review SpeechAct source: %w", err)
	}
	bindings, err := recordedMigrationReviewSourceValues(source)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: bindings.projectRoot == record.projectRoot.String(), name: "source project root"},
		{matches: bindings.speechActRef == record.speechActRef.String(), name: "source SpeechAct ref"},
		{matches: bindings.speechActDigest == record.speechActDigest.String(), name: "source SpeechAct digest"},
		{matches: bindings.captureRef == record.captureRef, name: "source capture ref"},
		{matches: bindings.captureDigest == record.captureDigest.String(), name: "source capture digest"},
		{matches: bindings.reviewDigest == record.reviewDigest.String(), name: "source review digest"},
		{matches: bindings.reviewText == record.reviewText, name: "source review text"},
		{matches: bindings.subjectRef == record.content.ref, name: "source review subject ref"},
		{matches: bindings.subjectDigest == record.content.digest.String(), name: "source review subject digest"},
		{matches: canonicalReviewTime(bindings.occurredAt).Equal(record.admittedAt), name: "source occurrence"},
	}
	return firstMigrationReviewMismatch(checks, "durable SpeechAct")
}

func recordedMigrationReviewSourceValues(
	source authority.RecordedSpeechActSource,
) (recordedMigrationReviewSourceBindings, error) {
	if !source.Valid() {
		return recordedMigrationReviewSourceBindings{}, fmt.Errorf("recorded migration-review SpeechAct source is invalid")
	}
	projectRoot, rootOK := source.ProjectRoot()
	speechActRef, refOK := source.SpeechActRef()
	speechActDigest, digestOK := source.SpeechActDigest()
	captureRef, captureRefOK := source.TerminalCaptureRef()
	captureDigest, captureDigestOK := source.TerminalCaptureDigest()
	reviewDigest, reviewDigestOK := source.ReviewDigest()
	reviewText, reviewTextOK := source.ReviewText()
	subjectRef, subjectRefOK := source.ReviewSubjectRef()
	subjectDigest, subjectDigestOK := source.ReviewSubjectDigest()
	occurredAt, occurredAtOK := source.OccurredAt()
	complete := rootOK && refOK && digestOK && captureRefOK && captureDigestOK
	complete = complete && reviewDigestOK && reviewTextOK && subjectRefOK && subjectDigestOK && occurredAtOK
	if !complete {
		return recordedMigrationReviewSourceBindings{}, fmt.Errorf("recorded migration-review SpeechAct source is incomplete")
	}
	return recordedMigrationReviewSourceBindings{
		projectRoot:     projectRoot.String(),
		speechActRef:    speechActRef.String(),
		speechActDigest: speechActDigest.String(),
		captureRef:      captureRef.String(),
		captureDigest:   captureDigest.String(),
		reviewDigest:    reviewDigest.String(),
		reviewText:      reviewText,
		subjectRef:      subjectRef.String(),
		subjectDigest:   subjectDigest.String(),
		occurredAt:      occurredAt,
	}, nil
}

func firstMigrationReviewMismatch(
	checks []struct {
		matches bool
		name    string
	},
	scope string,
) error {
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("migration-review %s %s is inconsistent", scope, check.name)
		}
	}
	return nil
}

func marshalMigrationReviewFragment(value any) ([]byte, error) {
	return json.Marshal(value)
}

func decodeCanonicalMigrationReviewJSON(content []byte, target any) error {
	if err := unmarshalCanonicalJSON(content, target); err != nil {
		return err
	}
	reencoded, err := marshalCanonicalJSON(target)
	if err != nil {
		return err
	}
	if !slices.Equal(reencoded, content) {
		return fmt.Errorf("migration-review JSON is not canonical")
	}
	return nil
}

func verifyMigrationReviewProtocolV2Schema(database *sql.DB) error {
	if err := requireMigrationReviewSchemaVersion(database, 39); err != nil {
		return err
	}
	if err := requireMigrationReviewSchemaVersion(database, 42); err != nil {
		return err
	}
	requiredTables := map[string][]string{
		"migration_review_acceptance_contents": {
			"review_content_ref", "review_content_digest", "project_root",
			"packet_digest", "packet_carrier_digest", "partition_audit_digest",
			"canonical_json", "recorded_at",
		},
		"migration_review_admissions_v2": {
			"admission_ref", "admission_digest", "project_root",
			"review_content_ref", "review_content_digest", "review_text", "review_digest",
			"capture_carrier_ref", "capture_carrier_digest",
			"speech_act_ref", "speech_act_digest",
			"context_policy_ref", "context_policy_digest", "act_type_ref",
			"method_ref", "method_description_ref", "method_description_digest",
			"bounded_context_ref", "institutional_effect_rule_ref", "admission_json",
		},
		"migration_review_instituted_effects": {
			"effect_digest", "project_root", "speech_act_ref", "speech_act_digest",
			"admission_ref", "admission_digest", "canonical_json",
		},
	}
	for table, columns := range requiredTables {
		if err := verifyMigrationReviewV2Columns(database, table, columns); err != nil {
			return err
		}
	}
	if err := verifyMigrationReviewV2Triggers(database); err != nil {
		return err
	}
	if err := verifyMigrationReviewSemanticLiteralTriggerV42(database); err != nil {
		return err
	}
	return verifyLegacyMigrationReviewTablesEmpty(database)
}

func requireMigrationReviewSchemaVersion(database *sql.DB, version int) error {
	var versionCount int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		version,
	).Scan(&versionCount)
	if err != nil || versionCount != 1 {
		failure := fmt.Errorf("migration-review protocol schema version %d is unavailable", version)
		return errors.Join(failure, err)
	}
	return nil
}

func verifyMigrationReviewV2Columns(
	database *sql.DB,
	table string,
	required []string,
) error {
	rows, err := database.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return fmt.Errorf("inspect migration-review table %s: %w", table, err)
	}
	defer rows.Close()
	observed := map[string]struct{}{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan migration-review table %s: %w", table, err)
		}
		observed[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, column := range required {
		if _, found := observed[column]; !found {
			return fmt.Errorf("migration-review table %s lacks required column %s", table, column)
		}
	}
	return nil
}

func verifyMigrationReviewV2Triggers(database *sql.DB) error {
	required := []string{
		"migration_review_admissions_v2_exact_sources",
		"migration_review_instituted_effects_exact_sources",
		"migration_review_acceptance_contents_no_replace",
		"migration_review_acceptance_contents_no_update",
		"migration_review_acceptance_contents_no_delete",
		"migration_review_admissions_v2_no_replace",
		"migration_review_admissions_v2_no_update",
		"migration_review_admissions_v2_no_delete",
		"migration_review_instituted_effects_no_replace",
		"migration_review_instituted_effects_no_update",
		"migration_review_instituted_effects_no_delete",
		"migration_review_acceptance_contents_project_ledger_root",
		"migration_review_admissions_v2_project_ledger_root",
		"migration_review_instituted_effects_project_ledger_root",
	}
	for _, name := range required {
		var statement string
		err := database.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
			name,
		).Scan(&statement)
		if err != nil || strings.TrimSpace(statement) == "" {
			return errors.Join(fmt.Errorf("migration-review trigger %s is unavailable", name), err)
		}
	}
	return nil
}

func verifyMigrationReviewSemanticLiteralTriggerV42(database *sql.DB) error {
	var statement string
	err := database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		"migration_review_admissions_v2_exact_sources",
	).Scan(&statement)
	if err != nil {
		return errors.Join(
			fmt.Errorf("migration-review semantic-literal protocol v42 trigger is unavailable"),
			err,
		)
	}
	required := []string{
		"capture.canonical_utterance = '" + migrationReviewUtteranceVerb + " " + migrationReviewUtteranceLiteral + "'",
		"NEW.context_policy_ref = '" + migrationReviewContextPolicyValue + "'",
		"NEW.institutional_effect_rule_ref = '" + migrationReviewEffectRuleValue + "'",
		"policy.utterance_description_ref = '" + migrationReviewUtteranceValue + "'",
		"policy.utterance_binding = 'literal'",
		"policy.utterance_literal = '" + migrationReviewUtteranceLiteral + "'",
	}
	for _, fragment := range required {
		if !strings.Contains(statement, fragment) {
			return fmt.Errorf(
				"migration-review exact-source trigger does not seal semantic-literal protocol v42: missing %s",
				fragment,
			)
		}
	}
	return nil
}

func verifyLegacyMigrationReviewTablesEmpty(database *sql.DB) error {
	for _, table := range []string{"migration_review_speech_acts", "migration_review_admissions"} {
		var count int
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&count); err != nil {
			return fmt.Errorf("inspect historical migration-review table %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf("historical migration-review table %s is non-empty; v35 records are read-only and not v2 authority", table)
		}
	}
	return nil
}
