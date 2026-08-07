package decisionbinding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const decisionBindingContentColumnsV41 = "decision_content_ref,decision_content_digest,prepared_decision_digest,project_root,decision_ref,canonical_json,recorded_at"

type DecisionBindingContentWriteKind string

const (
	DecisionBindingContentStored      DecisionBindingContentWriteKind = "stored"
	DecisionBindingContentExactReplay DecisionBindingContentWriteKind = "exact_replay"
)

// RecordedDecisionBindingContent proves only that the exact review subject is
// durable. It carries no SpeechAct and implies neither a human decision nor an
// instituted DecisionRecord.
type RecordedDecisionBindingContent struct {
	state *recordedDecisionBindingContentState
}

type recordedDecisionBindingContentState struct {
	content    DecisionBindingContent
	recordedAt time.Time
}

func (recorded RecordedDecisionBindingContent) Valid() bool {
	return recorded.state != nil &&
		recorded.state.content.valid() &&
		!recorded.state.recordedAt.IsZero() &&
		recorded.state.recordedAt.Equal(
			canonicalDecisionBindingTime(recorded.state.recordedAt),
		)
}

func (recorded RecordedDecisionBindingContent) Content() (
	DecisionBindingContent,
	bool,
) {
	if !recorded.Valid() {
		return DecisionBindingContent{}, false
	}
	return recorded.state.content, true
}

func (recorded RecordedDecisionBindingContent) RecordedAt() (time.Time, bool) {
	if !recorded.Valid() {
		return time.Time{}, false
	}
	return recorded.state.recordedAt, true
}

type DecisionBindingContentWriteResult struct {
	kind     DecisionBindingContentWriteKind
	recorded RecordedDecisionBindingContent
}

func (result DecisionBindingContentWriteResult) Kind() DecisionBindingContentWriteKind {
	return result.kind
}

func (result DecisionBindingContentWriteResult) RecordedContent() (
	RecordedDecisionBindingContent,
	bool,
) {
	validKind := result.kind == DecisionBindingContentStored ||
		result.kind == DecisionBindingContentExactReplay
	return result.recorded, validKind && result.recorded.Valid()
}

type DecisionBindingContentWriter struct {
	database *sql.DB
	now      func() time.Time
}

func OpenDecisionBindingContentWriter(
	database *sql.DB,
) (*DecisionBindingContentWriter, error) {
	if database == nil {
		return nil, fmt.Errorf("decision-binding content writer requires a database")
	}
	if err := requireDecisionBindingContentSchemaV41(database); err != nil {
		return nil, err
	}
	return &DecisionBindingContentWriter{
		database: database,
		now:      time.Now,
	}, nil
}

func requireDecisionBindingContentSchemaV41(database *sql.DB) error {
	var versionCount int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 41",
	).Scan(&versionCount)
	if err != nil || versionCount != 1 {
		return errors.Join(
			fmt.Errorf("decision-binding content migration 41 is unavailable"),
			err,
		)
	}
	var columns string
	err = database.QueryRow(`SELECT COALESCE(group_concat(name, ','), '')
		FROM (SELECT name FROM pragma_table_info('decision_binding_contents') ORDER BY cid)`).
		Scan(&columns)
	if err != nil || columns != decisionBindingContentColumnsV41 {
		return errors.Join(
			fmt.Errorf("decision-binding content schema is stale or incompatible"),
			err,
		)
	}
	return nil
}

func (writer *DecisionBindingContentWriter) Record(
	ctx context.Context,
	content DecisionBindingContent,
) (DecisionBindingContentWriteResult, error) {
	if writer == nil || writer.database == nil || writer.now == nil || ctx == nil {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"decision-binding content write requires an open writer and context",
		)
	}
	if err := ctx.Err(); err != nil {
		return DecisionBindingContentWriteResult{}, err
	}
	if !content.valid() {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"decision-binding content write requires exact canonical content",
		)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, writer.database)
	if err != nil {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"begin decision-binding content transaction: %w",
			err,
		)
	}
	result, err := writer.recordInTransaction(ctx, transaction, content)
	if err != nil {
		finish := transaction.Rollback(context.Background())
		return DecisionBindingContentWriteResult{}, errors.Join(err, finish.Err())
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		recovered, recoverErr := loadExactDecisionBindingContent(
			context.Background(),
			writer.database,
			content,
		)
		if recoverErr == nil && recovered.Valid() {
			return DecisionBindingContentWriteResult{
				kind:     result.kind,
				recorded: recovered,
			}, nil
		}
		return DecisionBindingContentWriteResult{}, errors.Join(
			fmt.Errorf("decision-binding content commit outcome is unknown"),
			finish.Err(),
			recoverErr,
		)
	}
	durable, err := loadExactDecisionBindingContent(
		context.Background(),
		writer.database,
		content,
	)
	if err != nil {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"committed decision-binding content failed strict exact reread: %w",
			err,
		)
	}
	return DecisionBindingContentWriteResult{
		kind:     result.kind,
		recorded: durable,
	}, nil
}

func (writer *DecisionBindingContentWriter) recordInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	content DecisionBindingContent,
) (DecisionBindingContentWriteResult, error) {
	if err := transaction.RequireImmediate(); err != nil {
		return DecisionBindingContentWriteResult{}, err
	}
	existing, found, err := loadDecisionBindingContentRowInTransaction(
		ctx,
		transaction,
		content,
	)
	if err != nil {
		return DecisionBindingContentWriteResult{}, err
	}
	if found {
		return DecisionBindingContentWriteResult{
			kind:     DecisionBindingContentExactReplay,
			recorded: existing,
		}, nil
	}
	recordedAt := canonicalDecisionBindingTime(writer.now())
	if recordedAt.IsZero() {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"decision-binding content recording time is required",
		)
	}
	arguments, err := decisionBindingContentInsertArguments(content, recordedAt)
	if err != nil {
		return DecisionBindingContentWriteResult{}, err
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO decision_binding_contents (
			decision_content_ref, decision_content_digest,
			prepared_decision_digest, project_root,
			decision_ref, canonical_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		arguments,
	)
	if err != nil {
		return DecisionBindingContentWriteResult{}, fmt.Errorf(
			"record decision-binding content: %w",
			err,
		)
	}
	recorded, found, err := loadDecisionBindingContentRowInTransaction(
		ctx,
		transaction,
		content,
	)
	if err != nil || !found || !recorded.Valid() {
		return DecisionBindingContentWriteResult{}, errors.Join(
			fmt.Errorf("staged decision-binding content failed strict exact reread"),
			err,
		)
	}
	return DecisionBindingContentWriteResult{
		kind:     DecisionBindingContentStored,
		recorded: recorded,
	}, nil
}

func decisionBindingContentInsertArguments(
	content DecisionBindingContent,
	recordedAt time.Time,
) ([]any, error) {
	ref, refOK := content.ContentRef()
	digest, digestOK := content.Digest()
	root, rootOK := content.ProjectRoot()
	decisionRef, decisionRefOK := content.DecisionRef()
	prepared, preparedOK := content.PreparedDecision()
	canonical, canonicalOK := content.CanonicalBytes()
	preparedDigest, preparedDigestOK := prepared.Digest()
	complete := refOK && digestOK && rootOK && decisionRefOK && preparedOK
	complete = complete && canonicalOK && preparedDigestOK && !recordedAt.IsZero()
	if !complete {
		return nil, fmt.Errorf("decision-binding content is incomplete")
	}
	return []any{
		ref.String(),
		digest.String(),
		preparedDigest.String(),
		root.String(),
		decisionRef,
		string(canonical),
		formatDecisionBindingTime(recordedAt),
	}, nil
}

type decisionBindingContentRow struct {
	contentRef     string
	contentDigest  string
	preparedDigest string
	projectRoot    string
	decisionRef    string
	canonicalJSON  string
	recordedAt     string
}

func loadDecisionBindingContentRowInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	content DecisionBindingContent,
) (RecordedDecisionBindingContent, bool, error) {
	ref, ok := content.ContentRef()
	if !ok {
		return RecordedDecisionBindingContent{}, false, fmt.Errorf(
			"decision-binding content has no canonical ref",
		)
	}
	row := decisionBindingContentRow{}
	err := transaction.ScanOne(
		ctx,
		`SELECT decision_content_ref, decision_content_digest,
			prepared_decision_digest, project_root, decision_ref,
			canonical_json, recorded_at
		 FROM decision_binding_contents WHERE decision_content_ref = ?`,
		[]any{ref.String()},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedDecisionBindingContent{}, false, nil
	}
	if err != nil {
		return RecordedDecisionBindingContent{}, false, fmt.Errorf(
			"read decision-binding content: %w",
			err,
		)
	}
	recorded, err := exactRecordedDecisionBindingContent(content, row)
	return recorded, true, err
}

func loadExactDecisionBindingContent(
	ctx context.Context,
	database *sql.DB,
	content DecisionBindingContent,
) (RecordedDecisionBindingContent, error) {
	ref, ok := content.ContentRef()
	if !ok {
		return RecordedDecisionBindingContent{}, fmt.Errorf(
			"decision-binding content has no canonical ref",
		)
	}
	row := decisionBindingContentRow{}
	err := database.QueryRowContext(
		ctx,
		`SELECT decision_content_ref, decision_content_digest,
			prepared_decision_digest, project_root, decision_ref,
			canonical_json, recorded_at
		 FROM decision_binding_contents WHERE decision_content_ref = ?`,
		ref.String(),
	).Scan(row.scanTargets()...)
	if err != nil {
		return RecordedDecisionBindingContent{}, err
	}
	return exactRecordedDecisionBindingContent(content, row)
}

func (row *decisionBindingContentRow) scanTargets() []any {
	return []any{
		&row.contentRef,
		&row.contentDigest,
		&row.preparedDigest,
		&row.projectRoot,
		&row.decisionRef,
		&row.canonicalJSON,
		&row.recordedAt,
	}
}

func exactRecordedDecisionBindingContent(
	content DecisionBindingContent,
	row decisionBindingContentRow,
) (RecordedDecisionBindingContent, error) {
	arguments, err := decisionBindingContentInsertArguments(content, time.Unix(1, 0))
	if err != nil {
		return RecordedDecisionBindingContent{}, err
	}
	want := []string{
		arguments[0].(string),
		arguments[1].(string),
		arguments[2].(string),
		arguments[3].(string),
		arguments[4].(string),
		arguments[5].(string),
	}
	got := []string{
		row.contentRef,
		row.contentDigest,
		row.preparedDigest,
		row.projectRoot,
		row.decisionRef,
		row.canonicalJSON,
	}
	if !slices.Equal(want, got) {
		return RecordedDecisionBindingContent{}, fmt.Errorf(
			"durable decision-binding content differs from exact canonical material",
		)
	}
	recordedAt, err := parseDecisionBindingTime(row.recordedAt)
	if err != nil {
		return RecordedDecisionBindingContent{}, err
	}
	state := recordedDecisionBindingContentState{
		content:    content,
		recordedAt: recordedAt,
	}
	recorded := RecordedDecisionBindingContent{state: &state}
	if !recorded.Valid() {
		return RecordedDecisionBindingContent{}, fmt.Errorf(
			"durable decision-binding content is invalid",
		)
	}
	return recorded, nil
}

func canonicalDecisionBindingTime(value time.Time) time.Time {
	return value.Round(0).UTC()
}

func formatDecisionBindingTime(value time.Time) string {
	return canonicalDecisionBindingTime(value).Format(time.RFC3339Nano)
}

func parseDecisionBindingTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse decision-binding time: %w", err)
	}
	canonical := canonicalDecisionBindingTime(parsed)
	if value != formatDecisionBindingTime(canonical) {
		return time.Time{}, fmt.Errorf(
			"decision-binding time must use canonical UTC RFC3339Nano form",
		)
	}
	return canonical, nil
}
