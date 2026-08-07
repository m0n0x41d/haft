package decisionbinding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type decisionSpeechActCapture func(
	context.Context,
	authority.PreparedManualSpeechAct,
) (authority.VerifiedSpeechActSource, error)

// PostSourcePreEffectGuard is a caller-owned check that must succeed after the
// exact SpeechAct source is durably recorded or reloaded and before the
// DecisionRecord effect transaction begins. The production caller supplies
// its checked project-ledger revalidation; this package deliberately does not
// know the concrete ledger implementation.
type PostSourcePreEffectGuard func(context.Context) error

// DecisionBindingService is the only package service that composes inert
// decision content, actual manual SpeechAct Work, and the atomic
// DecisionRecord institutional effect. It does not create a WorkCommission or
// any deontic U.Commitment.
type DecisionBindingService struct {
	database                 *sql.DB
	store                    *artifact.Store
	haftDir                  string
	projectRoot              string
	contentWriter            *DecisionBindingContentWriter
	sourceWriter             *authority.SpeechActSourceWriter
	postSourcePreEffectGuard PostSourcePreEffectGuard
	capture                  decisionSpeechActCapture
	now                      func() time.Time
}

// BindingResult is useful even when Bind returns an error: DecisionRef names
// the inert durable review subject that can be resumed after a cancelled act or
// a failed effect transaction. Hashes and nonces are deliberately absent from
// the ordinary result surface.
type BindingResult struct {
	decisionRef string
	title       string
	filePath    string
	speechAct   string
	effect      string
	exactReplay bool
	warnings    []string
}

func (result BindingResult) DecisionRef() string { return result.decisionRef }

func (result BindingResult) Title() string { return result.title }

func (result BindingResult) FilePath() string { return result.filePath }

func (result BindingResult) SpeechActRef() string { return result.speechAct }

func (result BindingResult) EffectDigest() string { return result.effect }

func (result BindingResult) ExactReplay() bool { return result.exactReplay }

func (result BindingResult) Warnings() []string { return slices.Clone(result.warnings) }

// OpenDecisionBindingService seals the production service to the generic
// authority package's controlling-terminal capture. Tests in this package may
// replace the private runtime function; callers cannot inject a model-minted
// approval source.
func OpenDecisionBindingService(
	database *sql.DB,
	store *artifact.Store,
	haftDir string,
	guard PostSourcePreEffectGuard,
) (*DecisionBindingService, error) {
	if database == nil || store == nil || store.DB() != database {
		return nil, fmt.Errorf("decision-binding service requires one shared artifact database")
	}
	if guard == nil {
		return nil, fmt.Errorf("decision-binding service requires a checked post-source/pre-effect guard")
	}
	canonicalHaftDir := filepath.Clean(haftDir)
	canonical := filepath.IsAbs(haftDir) && canonicalHaftDir == haftDir
	canonical = canonical && filepath.Base(canonicalHaftDir) == ".haft"
	if !canonical {
		return nil, fmt.Errorf("decision-binding service requires an absolute canonical .haft directory")
	}
	contentWriter, err := OpenDecisionBindingContentWriter(database)
	if err != nil {
		return nil, err
	}
	sourceWriter, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		return nil, err
	}
	return &DecisionBindingService{
		database:                 database,
		store:                    store,
		haftDir:                  canonicalHaftDir,
		projectRoot:              filepath.Dir(canonicalHaftDir),
		contentWriter:            contentWriter,
		sourceWriter:             sourceWriter,
		postSourcePreEffectGuard: guard,
		capture:                  authority.CaptureVerifiedSpeechAct,
		now:                      time.Now,
	}, nil
}

// Bind prepares one fresh DecisionRecord identity, records inert review
// content, captures the actual manual SpeechAct, and atomically institutes the
// exact artifact and effect.
func (service *DecisionBindingService) Bind(
	ctx context.Context,
	input artifact.DecideInput,
) (BindingResult, error) {
	if err := service.validate(ctx); err != nil {
		return BindingResult{}, err
	}
	reservation, err := artifact.ReserveDecisionIdentity(input.TaskContext)
	if err != nil {
		return BindingResult{}, err
	}
	result := BindingResult{decisionRef: reservation.String()}
	prepared, err := artifact.PrepareDecision(
		ctx,
		service.store,
		service.haftDir,
		reservation,
		input,
	)
	if err != nil {
		return result, fmt.Errorf("prepare decision %s: %w", reservation.String(), err)
	}
	return service.BindPrepared(ctx, prepared)
}

// BindPrepared is the exact semantic entry point used when a caller has
// already reserved and prepared a review snapshot. The PreparedDecision is
// revalidated again inside the atomic effect transaction.
func (service *DecisionBindingService) BindPrepared(
	ctx context.Context,
	prepared artifact.PreparedDecision,
) (BindingResult, error) {
	if err := service.validate(ctx); err != nil {
		return BindingResult{}, err
	}
	content, err := NewDecisionBindingContent(prepared)
	if err != nil {
		return BindingResult{}, err
	}
	root, rootOK := content.ProjectRoot()
	if !rootOK || root.String() != service.projectRoot {
		return BindingResult{}, fmt.Errorf("prepared decision belongs to another project root")
	}
	return service.bindContent(ctx, content)
}

// Resume reconstructs the exact durable review subject by its human-facing
// DecisionRecord ID. It reuses an already-recorded SpeechAct source and never
// asks the person to perform the same act twice after an effect failure.
func (service *DecisionBindingService) Resume(
	ctx context.Context,
	decisionRef string,
) (BindingResult, error) {
	if err := service.validate(ctx); err != nil {
		return BindingResult{}, err
	}
	completed, found, err := service.resumeCompletedDecision(ctx, decisionRef)
	if err != nil {
		return BindingResult{decisionRef: decisionRef}, err
	}
	if found {
		return completed, nil
	}
	content, err := service.reconstructContent(ctx, decisionRef)
	if err != nil {
		return BindingResult{decisionRef: decisionRef}, err
	}
	return service.bindContent(ctx, content)
}

func (service *DecisionBindingService) validate(ctx context.Context) error {
	complete := service != nil && service.database != nil && service.store != nil
	complete = complete && service.contentWriter != nil && service.sourceWriter != nil
	complete = complete && service.postSourcePreEffectGuard != nil
	complete = complete && service.capture != nil && service.now != nil
	if !complete || ctx == nil {
		return fmt.Errorf("decision-binding service is not open")
	}
	return ctx.Err()
}

func (service *DecisionBindingService) bindContent(
	ctx context.Context,
	content DecisionBindingContent,
) (BindingResult, error) {
	result, err := baseBindingResult(content)
	if err != nil {
		return BindingResult{}, err
	}
	_, err = service.contentWriter.Record(ctx, content)
	if err != nil {
		return result, fmt.Errorf("record inert decision content %s: %w", result.decisionRef, err)
	}
	source, found, err := service.resolveDecisionSpeechActSource(ctx, content)
	if err != nil {
		return result, err
	}
	if !found {
		source, err = service.captureAndRecordDecisionSpeechAct(ctx, content)
		if err != nil {
			return result, fmt.Errorf(
				"decision %s remains inert and can be resumed: %w",
				result.decisionRef,
				err,
			)
		}
	}
	speechActRef, speechActRefOK := source.SpeechActRef()
	if !speechActRefOK {
		return result, fmt.Errorf("durable decision SpeechAct has no canonical ref")
	}
	result.speechAct = speechActRef.String()
	// Both source paths return only after their transaction or query row is
	// closed. The caller-owned checked-ledger guard therefore runs in the exact
	// post-source/pre-BEGIN-IMMEDIATE gap and releases its own resources before
	// instituteDecisionRecord starts the effect transaction.
	if err := service.postSourcePreEffectGuard(ctx); err != nil {
		return result, fmt.Errorf(
			"decision %s SpeechAct is durable but its effect was not instituted: checked post-source/pre-effect guard: %w",
			result.decisionRef,
			err,
		)
	}
	recorded, exactReplay, err := service.instituteDecisionRecord(ctx, content, source)
	if err != nil {
		return result, fmt.Errorf(
			"decision %s SpeechAct is durable but its effect was not instituted: %w",
			result.decisionRef,
			err,
		)
	}
	effect, effectOK := recorded.Effect()
	effectDigest, digestOK := effect.Digest()
	if !effectOK || !digestOK {
		return result, fmt.Errorf("instituted decision effect has no canonical identity")
	}
	result.effect = effectDigest.String()
	result.exactReplay = exactReplay
	return service.projectDecision(ctx, result)
}

func baseBindingResult(content DecisionBindingContent) (BindingResult, error) {
	decisionRef, decisionRefOK := content.DecisionRef()
	review, reviewOK := content.ReviewSnapshot()
	title, titleOK := review.Title()
	if !decisionRefOK || !reviewOK || !titleOK {
		return BindingResult{}, fmt.Errorf("decision-binding content has no readable identity")
	}
	return BindingResult{decisionRef: decisionRef, title: title}, nil
}

func (service *DecisionBindingService) resolveDecisionSpeechActSource(
	ctx context.Context,
	content DecisionBindingContent,
) (authority.RecordedSpeechActSource, bool, error) {
	digest, ok := content.Digest()
	if !ok {
		return authority.RecordedSpeechActSource{}, false, fmt.Errorf(
			"decision content has no canonical digest",
		)
	}
	identity := strings.TrimPrefix(digest.String(), "sha256:")
	ref, err := authority.NewSpeechActRef("speech-act:decision-binding:" + identity)
	if err != nil {
		return authority.RecordedSpeechActSource{}, false, err
	}
	source, found, err := authority.ResolveRecordedSpeechActSource(
		ctx,
		service.database,
		ref,
	)
	if err != nil {
		return authority.RecordedSpeechActSource{}, false, fmt.Errorf(
			"resolve durable decision SpeechAct: %w",
			err,
		)
	}
	if !found {
		return authority.RecordedSpeechActSource{}, false, nil
	}
	if _, err := NewDecisionRecordInstitutedEffect(content, source); err != nil {
		return authority.RecordedSpeechActSource{}, false, fmt.Errorf(
			"durable SpeechAct source does not bind this exact decision: %w",
			err,
		)
	}
	return source, true, nil
}

func (service *DecisionBindingService) captureAndRecordDecisionSpeechAct(
	ctx context.Context,
	content DecisionBindingContent,
) (authority.RecordedSpeechActSource, error) {
	intent, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	card, err := content.ReviewCard()
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	reviewText, reviewTextOK := card.Text()
	if !reviewTextOK {
		return authority.RecordedSpeechActSource{}, fmt.Errorf("decision review card has no canonical text")
	}
	prepared, err := authority.PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	verified, err := service.capture(ctx, prepared)
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	recorded, err := service.sourceWriter.Record(ctx, verified)
	if err != nil {
		return authority.RecordedSpeechActSource{}, err
	}
	return recorded, nil
}

func (service *DecisionBindingService) instituteDecisionRecord(
	ctx context.Context,
	content DecisionBindingContent,
	source authority.RecordedSpeechActSource,
) (RecordedDecisionRecordInstitutedEffect, bool, error) {
	effect, err := NewDecisionRecordInstitutedEffect(content, source)
	if err != nil {
		return RecordedDecisionRecordInstitutedEffect{}, false, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return RecordedDecisionRecordInstitutedEffect{}, false, fmt.Errorf(
			"begin DecisionRecord effect transaction: %w",
			err,
		)
	}
	existing, found, err := loadDecisionRecordEffectInTransaction(
		ctx,
		transaction,
		effect,
	)
	if err != nil {
		return rollbackDecisionEffect(transaction, err)
	}
	if found {
		finish := transaction.Commit(ctx)
		if !finish.Succeeded() {
			return recoverDecisionEffectCommit(service.database, effect, finish.Err())
		}
		return existing, true, nil
	}
	prepared, preparedOK := content.PreparedDecision()
	if !preparedOK {
		return rollbackDecisionEffect(
			transaction,
			fmt.Errorf("decision content has no exact PreparedDecision"),
		)
	}
	transactionStore, err := newDecisionTransactionReadStore(
		service.store,
		transaction,
	)
	if err != nil {
		return rollbackDecisionEffect(transaction, err)
	}
	err = artifact.RevalidatePreparedDecision(
		ctx,
		transactionStore,
		service.haftDir,
		prepared,
	)
	if err != nil {
		return rollbackDecisionEffect(transaction, err)
	}
	occurredAt, occurredAtOK := source.OccurredAt()
	if !occurredAtOK {
		return rollbackDecisionEffect(
			transaction,
			fmt.Errorf("durable decision SpeechAct has no occurrence time"),
		)
	}
	if err := stagePreparedDecisionArtifactInTransaction(
		ctx,
		transaction,
		content,
		occurredAt,
	); err != nil {
		return rollbackDecisionEffect(transaction, err)
	}
	recordedAt := canonicalDecisionBindingTime(service.now())
	writeResult, err := RecordEffectInTransaction(
		ctx,
		transaction,
		effect,
		recordedAt,
	)
	if err != nil {
		return rollbackDecisionEffect(transaction, err)
	}
	_, recordedOK := writeResult.RecordedEffect()
	if !recordedOK {
		return rollbackDecisionEffect(
			transaction,
			fmt.Errorf("DecisionRecord effect writer returned no exact staged effect"),
		)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		return recoverDecisionEffectCommit(service.database, effect, finish.Err())
	}
	durable, found, err := loadDecisionRecordEffect(service.database, effect)
	if err != nil || !found {
		return RecordedDecisionRecordInstitutedEffect{}, false, errors.Join(
			fmt.Errorf("committed DecisionRecord effect failed strict durable reread"),
			err,
		)
	}
	return durable, false, nil
}

func rollbackDecisionEffect(
	transaction *sqlitetransaction.Transaction,
	cause error,
) (RecordedDecisionRecordInstitutedEffect, bool, error) {
	finish := transaction.Rollback(context.Background())
	return RecordedDecisionRecordInstitutedEffect{}, false, errors.Join(cause, finish.Err())
}

func recoverDecisionEffectCommit(
	database *sql.DB,
	effect DecisionRecordInstitutedEffect,
	commitErr error,
) (RecordedDecisionRecordInstitutedEffect, bool, error) {
	durable, found, loadErr := loadDecisionRecordEffect(database, effect)
	if loadErr == nil && found {
		return durable, true, nil
	}
	return RecordedDecisionRecordInstitutedEffect{}, false, errors.Join(
		fmt.Errorf("DecisionRecord effect commit outcome is unknown"),
		commitErr,
		loadErr,
	)
}

func loadDecisionRecordEffect(
	database *sql.DB,
	effect DecisionRecordInstitutedEffect,
) (RecordedDecisionRecordInstitutedEffect, bool, error) {
	if database == nil {
		return RecordedDecisionRecordInstitutedEffect{}, false, fmt.Errorf(
			"DecisionRecord effect load requires a database",
		)
	}
	digest, digestOK := effect.Digest()
	if !digestOK {
		return RecordedDecisionRecordInstitutedEffect{}, false, fmt.Errorf(
			"DecisionRecord effect has no canonical digest",
		)
	}
	row := decisionRecordEffectRow{}
	err := database.QueryRow(
		`SELECT effect_digest, project_root, decision_ref,
			decision_content_ref, decision_content_digest,
			speech_act_ref, speech_act_digest,
			context_policy_ref, context_policy_digest,
			institutional_effect_rule_ref, canonical_json, recorded_at
		 FROM decision_record_instituted_effects WHERE effect_digest = ?`,
		digest.String(),
	).Scan(row.scanTargets()...)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedDecisionRecordInstitutedEffect{}, false, nil
	}
	if err != nil {
		return RecordedDecisionRecordInstitutedEffect{}, false, err
	}
	recorded, err := exactRecordedDecisionRecordEffect(effect, row)
	return recorded, err == nil, err
}

func (service *DecisionBindingService) reconstructContent(
	ctx context.Context,
	decisionRef string,
) (DecisionBindingContent, error) {
	decisionRef = strings.TrimSpace(decisionRef)
	if decisionRef == "" {
		return DecisionBindingContent{}, fmt.Errorf("decision ref is required")
	}
	row := decisionBindingContentRow{}
	err := service.database.QueryRowContext(
		ctx,
		`SELECT decision_content_ref, decision_content_digest,
			prepared_decision_digest, project_root, decision_ref,
			canonical_json, recorded_at
		 FROM decision_binding_contents WHERE decision_ref = ?`,
		decisionRef,
	).Scan(row.scanTargets()...)
	if err != nil {
		return DecisionBindingContent{}, fmt.Errorf("load decision binding %s: %w", decisionRef, err)
	}
	envelope := decisionBindingContentJSONV1{}
	if err := json.Unmarshal([]byte(row.canonicalJSON), &envelope); err != nil {
		return DecisionBindingContent{}, fmt.Errorf("decode durable decision content: %w", err)
	}
	projection := struct {
		ProjectRoot   string          `json:"project_root"`
		DecisionRef   string          `json:"decision_ref"`
		ProposalInput json.RawMessage `json:"proposal_input"`
	}{}
	if err := json.Unmarshal(envelope.PreparedDecision, &projection); err != nil {
		return DecisionBindingContent{}, fmt.Errorf("decode durable PreparedDecision: %w", err)
	}
	if projection.ProjectRoot != service.projectRoot || projection.DecisionRef != decisionRef {
		return DecisionBindingContent{}, fmt.Errorf("durable decision belongs to another project or identity")
	}
	input, err := artifact.DecodeDecideInputCanonicalJSON(projection.ProposalInput)
	if err != nil {
		return DecisionBindingContent{}, err
	}
	reservation, err := artifact.NewDecisionReservation(decisionRef)
	if err != nil {
		return DecisionBindingContent{}, err
	}
	prepared, err := artifact.PrepareDecision(
		ctx,
		service.store,
		service.haftDir,
		reservation,
		input,
	)
	if err != nil {
		return DecisionBindingContent{}, err
	}
	content, err := NewDecisionBindingContent(prepared)
	if err != nil {
		return DecisionBindingContent{}, err
	}
	if _, err := exactRecordedDecisionBindingContent(content, row); err != nil {
		return DecisionBindingContent{}, fmt.Errorf(
			"durable decision content is stale against current project sources: %w",
			err,
		)
	}
	return content, nil
}

func (service *DecisionBindingService) resumeCompletedDecision(
	ctx context.Context,
	decisionRef string,
) (BindingResult, bool, error) {
	decisionRef = strings.TrimSpace(decisionRef)
	if decisionRef == "" {
		return BindingResult{}, false, fmt.Errorf("decision ref is required")
	}
	row := struct {
		title           string
		projectRoot     string
		speechActRef    string
		speechActDigest string
		effectDigest    string
	}{}
	err := service.database.QueryRowContext(
		ctx,
		`SELECT artifact.title, effect.project_root,
			effect.speech_act_ref, effect.speech_act_digest,
			effect.effect_digest
		 FROM decision_record_instituted_effects effect
		 JOIN decision_binding_contents content
			ON content.decision_content_ref = effect.decision_content_ref
			AND content.decision_content_digest = effect.decision_content_digest
			AND content.decision_ref = effect.decision_ref
		 JOIN artifacts artifact ON artifact.id = effect.decision_ref
		 WHERE effect.decision_ref = ?`,
		decisionRef,
	).Scan(
		&row.title,
		&row.projectRoot,
		&row.speechActRef,
		&row.speechActDigest,
		&row.effectDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return BindingResult{}, false, nil
	}
	if err != nil {
		return BindingResult{}, false, fmt.Errorf("resolve completed decision binding: %w", err)
	}
	if row.projectRoot != service.projectRoot {
		return BindingResult{}, false, fmt.Errorf("completed decision belongs to another project root")
	}
	ref, err := authority.NewSpeechActRef(row.speechActRef)
	if err != nil {
		return BindingResult{}, false, err
	}
	digest, err := authority.NewDigest(row.speechActDigest)
	if err != nil {
		return BindingResult{}, false, err
	}
	source, err := authority.LoadRecordedSpeechActSource(
		ctx,
		service.database,
		ref,
		digest,
	)
	if err != nil || !source.Valid() {
		return BindingResult{}, false, errors.Join(
			fmt.Errorf("completed decision has no strict durable SpeechAct source"),
			err,
		)
	}
	result := BindingResult{
		decisionRef: decisionRef,
		title:       row.title,
		speechAct:   row.speechActRef,
		effect:      row.effectDigest,
		exactReplay: true,
	}
	projected, err := service.projectDecision(ctx, result)
	return projected, true, err
}

func (service *DecisionBindingService) projectDecision(
	ctx context.Context,
	result BindingResult,
) (BindingResult, error) {
	value, err := service.store.Get(ctx, result.decisionRef)
	if err != nil {
		result.warnings = append(
			result.warnings,
			fmt.Sprintf("DecisionRecord is durable but its Markdown projection could not be loaded: %v", err),
		)
		return result, nil
	}
	path, err := artifact.WriteFile(service.haftDir, value)
	if err != nil {
		result.warnings = append(
			result.warnings,
			fmt.Sprintf("DecisionRecord is durable but its Markdown projection failed: %v", err),
		)
		return result, nil
	}
	result.filePath = path
	return result, nil
}
