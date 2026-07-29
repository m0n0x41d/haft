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

type DecisionRecordEffectWriteKind string

const (
	DecisionRecordEffectStaged      DecisionRecordEffectWriteKind = "staged"
	DecisionRecordEffectExactReplay DecisionRecordEffectWriteKind = "exact_replay"
)

// RecordedDecisionRecordInstitutedEffect is the exact row staged inside the
// caller-owned transaction. It is durable only after that caller commits.
type RecordedDecisionRecordInstitutedEffect struct {
	state *recordedDecisionRecordInstitutedEffectState
}

type recordedDecisionRecordInstitutedEffectState struct {
	effect     DecisionRecordInstitutedEffect
	recordedAt time.Time
}

func (recorded RecordedDecisionRecordInstitutedEffect) Valid() bool {
	if recorded.state == nil || !recorded.state.effect.valid() {
		return false
	}
	occurredAt, occurredAtOK := recorded.state.effect.OccurredAt()
	recordedAt := recorded.state.recordedAt
	return occurredAtOK &&
		!recordedAt.IsZero() &&
		recordedAt.Equal(canonicalDecisionBindingTime(recordedAt)) &&
		!recordedAt.Before(occurredAt)
}

func (recorded RecordedDecisionRecordInstitutedEffect) Effect() (
	DecisionRecordInstitutedEffect,
	bool,
) {
	if !recorded.Valid() {
		return DecisionRecordInstitutedEffect{}, false
	}
	return recorded.state.effect, true
}

func (recorded RecordedDecisionRecordInstitutedEffect) RecordedAt() (time.Time, bool) {
	if !recorded.Valid() {
		return time.Time{}, false
	}
	return recorded.state.recordedAt, true
}

type DecisionRecordEffectWriteResult struct {
	kind     DecisionRecordEffectWriteKind
	recorded RecordedDecisionRecordInstitutedEffect
}

func (result DecisionRecordEffectWriteResult) Kind() DecisionRecordEffectWriteKind {
	return result.kind
}

func (result DecisionRecordEffectWriteResult) RecordedEffect() (
	RecordedDecisionRecordInstitutedEffect,
	bool,
) {
	validKind := result.kind == DecisionRecordEffectStaged ||
		result.kind == DecisionRecordEffectExactReplay
	return result.recorded, validKind && result.recorded.Valid()
}

// RecordEffectInTransaction inserts and strictly rereads an instituted effect
// after the caller has staged the exact PreparedDecision artifact rows. It
// requires an existing BEGIN IMMEDIATE transaction and never starts, commits,
// or rolls back that transaction.
func RecordEffectInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect DecisionRecordInstitutedEffect,
	recordedAt time.Time,
) (DecisionRecordEffectWriteResult, error) {
	if ctx == nil || transaction == nil {
		return DecisionRecordEffectWriteResult{}, fmt.Errorf(
			"decision-record effect write requires a context and transaction",
		)
	}
	if err := ctx.Err(); err != nil {
		return DecisionRecordEffectWriteResult{}, err
	}
	if err := transaction.RequireImmediate(); err != nil {
		return DecisionRecordEffectWriteResult{}, err
	}
	if !effect.valid() {
		return DecisionRecordEffectWriteResult{}, fmt.Errorf(
			"decision-record effect write requires an exact canonical effect",
		)
	}
	recordedAt = canonicalDecisionBindingTime(recordedAt)
	occurredAt, occurredAtOK := effect.OccurredAt()
	if recordedAt.IsZero() || !occurredAtOK || recordedAt.Before(occurredAt) {
		return DecisionRecordEffectWriteResult{}, fmt.Errorf(
			"decision-record effect recording time must be at or after the SpeechAct occurrence",
		)
	}
	existing, found, err := loadDecisionRecordEffectInTransaction(
		ctx,
		transaction,
		effect,
	)
	if err != nil {
		return DecisionRecordEffectWriteResult{}, err
	}
	if found {
		return DecisionRecordEffectWriteResult{
			kind:     DecisionRecordEffectExactReplay,
			recorded: existing,
		}, nil
	}
	arguments, err := decisionRecordEffectInsertArguments(effect, recordedAt)
	if err != nil {
		return DecisionRecordEffectWriteResult{}, err
	}
	_, err = transaction.Execute(
		ctx,
		`INSERT INTO decision_record_instituted_effects (
			effect_digest, project_root, decision_ref,
			decision_content_ref, decision_content_digest,
			speech_act_ref, speech_act_digest,
			context_policy_ref, context_policy_digest,
			institutional_effect_rule_ref, canonical_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		arguments,
	)
	if err != nil {
		return DecisionRecordEffectWriteResult{}, fmt.Errorf(
			"record decision-record instituted effect: %w",
			err,
		)
	}
	recorded, found, err := loadDecisionRecordEffectInTransaction(
		ctx,
		transaction,
		effect,
	)
	if err != nil || !found || !recorded.Valid() {
		return DecisionRecordEffectWriteResult{}, errors.Join(
			fmt.Errorf("staged decision-record effect failed strict exact reread"),
			err,
		)
	}
	return DecisionRecordEffectWriteResult{
		kind:     DecisionRecordEffectStaged,
		recorded: recorded,
	}, nil
}

func decisionRecordEffectInsertArguments(
	effect DecisionRecordInstitutedEffect,
	recordedAt time.Time,
) ([]any, error) {
	if !effect.valid() || recordedAt.IsZero() {
		return nil, fmt.Errorf("decision-record instituted effect is incomplete")
	}
	state := effect.state
	return []any{
		state.digest.String(),
		state.projectRoot.String(),
		state.decisionRef,
		state.contentRef.String(),
		state.contentDigest.String(),
		state.speechActRef.String(),
		state.speechActDigest.String(),
		state.contextPolicyRef.String(),
		state.contextPolicyDigest.String(),
		state.institutionalEffectRule.String(),
		string(state.canonicalBytes),
		formatDecisionBindingTime(recordedAt),
	}, nil
}

type decisionRecordEffectRow struct {
	effectDigest        string
	projectRoot         string
	decisionRef         string
	contentRef          string
	contentDigest       string
	speechActRef        string
	speechActDigest     string
	contextPolicyRef    string
	contextPolicyDigest string
	effectRuleRef       string
	canonicalJSON       string
	recordedAt          string
}

func (row *decisionRecordEffectRow) scanTargets() []any {
	return []any{
		&row.effectDigest,
		&row.projectRoot,
		&row.decisionRef,
		&row.contentRef,
		&row.contentDigest,
		&row.speechActRef,
		&row.speechActDigest,
		&row.contextPolicyRef,
		&row.contextPolicyDigest,
		&row.effectRuleRef,
		&row.canonicalJSON,
		&row.recordedAt,
	}
}

func loadDecisionRecordEffectInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	effect DecisionRecordInstitutedEffect,
) (RecordedDecisionRecordInstitutedEffect, bool, error) {
	digest, ok := effect.Digest()
	if !ok {
		return RecordedDecisionRecordInstitutedEffect{}, false, fmt.Errorf(
			"decision-record effect has no canonical digest",
		)
	}
	row := decisionRecordEffectRow{}
	err := transaction.ScanOne(
		ctx,
		`SELECT effect_digest, project_root, decision_ref,
			decision_content_ref, decision_content_digest,
			speech_act_ref, speech_act_digest,
			context_policy_ref, context_policy_digest,
			institutional_effect_rule_ref, canonical_json, recorded_at
		 FROM decision_record_instituted_effects WHERE effect_digest = ?`,
		[]any{digest.String()},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedDecisionRecordInstitutedEffect{}, false, nil
	}
	if err != nil {
		return RecordedDecisionRecordInstitutedEffect{}, false, fmt.Errorf(
			"read decision-record instituted effect: %w",
			err,
		)
	}
	recorded, err := exactRecordedDecisionRecordEffect(effect, row)
	return recorded, true, err
}

func exactRecordedDecisionRecordEffect(
	effect DecisionRecordInstitutedEffect,
	row decisionRecordEffectRow,
) (RecordedDecisionRecordInstitutedEffect, error) {
	if !effect.valid() {
		return RecordedDecisionRecordInstitutedEffect{}, fmt.Errorf(
			"decision-record instituted effect is invalid",
		)
	}
	state := effect.state
	want := []string{
		state.digest.String(),
		state.projectRoot.String(),
		state.decisionRef,
		state.contentRef.String(),
		state.contentDigest.String(),
		state.speechActRef.String(),
		state.speechActDigest.String(),
		state.contextPolicyRef.String(),
		state.contextPolicyDigest.String(),
		state.institutionalEffectRule.String(),
		string(state.canonicalBytes),
	}
	got := []string{
		row.effectDigest,
		row.projectRoot,
		row.decisionRef,
		row.contentRef,
		row.contentDigest,
		row.speechActRef,
		row.speechActDigest,
		row.contextPolicyRef,
		row.contextPolicyDigest,
		row.effectRuleRef,
		row.canonicalJSON,
	}
	if !slices.Equal(want, got) {
		return RecordedDecisionRecordInstitutedEffect{}, fmt.Errorf(
			"durable decision-record effect differs from exact canonical material",
		)
	}
	recordedAt, err := parseDecisionBindingTime(row.recordedAt)
	if err != nil {
		return RecordedDecisionRecordInstitutedEffect{}, err
	}
	stateValue := recordedDecisionRecordInstitutedEffectState{
		effect:     effect,
		recordedAt: recordedAt,
	}
	recorded := RecordedDecisionRecordInstitutedEffect{state: &stateValue}
	if !recorded.Valid() {
		return RecordedDecisionRecordInstitutedEffect{}, fmt.Errorf(
			"durable decision-record instituted effect is invalid",
		)
	}
	return recorded, nil
}
