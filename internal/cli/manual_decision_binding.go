package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/decisionbinding"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

type manualDecisionBindingOutcome struct {
	DecisionRef          string
	Title                string
	FilePath             string
	EffectInstituted     bool
	ExactReplay          bool
	Warnings             []string
	TaskMemoryProjection *taskMemoryProjectionReport
}

type manualDecisionBindingSession interface {
	Bind(context.Context, artifact.DecideInput) (manualDecisionBindingOutcome, error)
	Resume(context.Context, string) (manualDecisionBindingOutcome, error)
	Close() error
}

type canonicalManualDecisionBindingSession struct {
	ledger  *projectledger.Handle
	service *decisionbinding.DecisionBindingService
	store   *artifact.Store
	haftDir string
}

var openManualDecisionBindingSession = openCanonicalManualDecisionBindingSession

func bindDecisionByProjectPolicy(
	ctx context.Context,
	projectRoot string,
	input artifact.DecideInput,
) (manualDecisionBindingOutcome, error) {
	config, err := project.LoadProjectConfig(projectRoot)
	if err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf(
			"load Haft project config: %w",
			err,
		)
	}
	mode := config.EffectiveDecisionBindingMode()
	if mode == project.DecisionBindingModeExplicitHDecide {
		return bindExplicitHDecideDecision(
			ctx,
			projectRoot,
			input,
		)
	}
	if mode == project.DecisionBindingModeStrictCLISpeechAct {
		return bindStrictCLISpeechActDecision(
			ctx,
			projectRoot,
			input,
		)
	}
	return manualDecisionBindingOutcome{}, fmt.Errorf(
		"unsupported decision binding mode %q in %s",
		mode,
		project.ProjectConfigPath(
			filepath.Join(projectRoot, ".haft"),
		),
	)
}

func bindExplicitHDecideDecision(
	ctx context.Context,
	projectRoot string,
	input artifact.DecideInput,
) (manualDecisionBindingOutcome, error) {
	ledger, err := projectledger.OpenExisting(
		ctx,
		projectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf(
			"open checked project ledger for explicit h-decide: %w",
			err,
		)
	}
	defer ledger.Close()
	if err := ledger.Revalidate(ctx); err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf(
			"revalidate checked project ledger before explicit h-decide preflight: %w",
			err,
		)
	}
	store := artifact.NewStore(ledger.Database())
	haftDir := filepath.Join(projectRoot, ".haft")
	prepared, err := applyDecisionSpecBindingPreflight(
		ctx,
		store,
		haftDir,
		input,
	)
	if err != nil {
		return manualDecisionBindingOutcome{}, err
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf(
			"revalidate checked project ledger before explicit h-decide write: %w",
			err,
		)
	}
	created, filePath, decideErr := artifact.Decide(
		ctx,
		store,
		haftDir,
		prepared,
	)
	warnings, err := decisionWriteWarnings(
		created,
		decideErr,
	)
	if err != nil {
		return manualDecisionBindingOutcome{}, err
	}
	projection := projectExistingDecisionAfterBinding(
		ctx,
		ledger.ProjectID(),
		ledger.Database(),
		store,
		created.Meta.ID,
	)
	outcome := manualDecisionBindingOutcome{
		DecisionRef:          created.Meta.ID,
		Title:                created.Meta.Title,
		FilePath:             filePath,
		EffectInstituted:     true,
		Warnings:             warnings,
		TaskMemoryProjection: &projection,
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return outcome, fmt.Errorf(
			"DecisionRecord %s — %q was created, but checked-ledger verification after the write failed: %w",
			created.Meta.ID,
			created.Meta.Title,
			err,
		)
	}
	return outcome, nil
}

func decisionWriteWarnings(
	created *artifact.Artifact,
	writeErr error,
) ([]string, error) {
	if writeErr == nil && created != nil {
		return nil, nil
	}
	warning := &artifact.WriteWarning{}
	if errors.As(writeErr, &warning) && created != nil {
		return append([]string(nil), warning.Warnings...), nil
	}
	if writeErr != nil {
		return nil, writeErr
	}
	return nil, fmt.Errorf(
		"decision binding returned no DecisionRecord",
	)
}

func bindStrictCLISpeechActDecision(
	ctx context.Context,
	projectRoot string,
	input artifact.DecideInput,
) (outcome manualDecisionBindingOutcome, resultErr error) {
	session, err := openManualDecisionBindingSession(
		ctx,
		projectRoot,
	)
	if err != nil {
		return manualDecisionBindingOutcome{}, err
	}
	defer func() {
		resultErr = errors.Join(
			resultErr,
			session.Close(),
		)
	}()
	outcome, err = session.Bind(ctx, input)
	if err != nil {
		return outcome, resumableDecisionBindingError(
			outcome,
			err,
		)
	}
	return outcome, nil
}

func openCanonicalManualDecisionBindingSession(
	ctx context.Context,
	projectRoot string,
) (manualDecisionBindingSession, error) {
	ledger, err := projectledger.OpenExisting(
		ctx,
		projectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return nil, fmt.Errorf("open checked project ledger for manual decision binding: %w", err)
	}
	database := ledger.Database()
	store := artifact.NewStore(database)
	haftDir := filepath.Join(projectRoot, ".haft")
	service, err := decisionbinding.OpenDecisionBindingService(
		database,
		store,
		haftDir,
		decisionbinding.PostSourcePreEffectGuard(ledger.Revalidate),
	)
	if err != nil {
		_ = ledger.Close()
		return nil, fmt.Errorf("open manual DecisionRecord binding service: %w", err)
	}
	return &canonicalManualDecisionBindingSession{
		ledger:  ledger,
		service: service,
		store:   store,
		haftDir: haftDir,
	}, nil
}

func (session *canonicalManualDecisionBindingSession) Bind(
	ctx context.Context,
	input artifact.DecideInput,
) (manualDecisionBindingOutcome, error) {
	if session == nil || session.ledger == nil || session.service == nil || session.store == nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf("manual decision-binding session is closed")
	}
	if err := session.ledger.Revalidate(ctx); err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf("revalidate checked project ledger before decision review: %w", err)
	}
	preparedInput, err := applyDecisionSpecBindingPreflight(
		ctx,
		session.store,
		session.haftDir,
		input,
	)
	if err != nil {
		return manualDecisionBindingOutcome{}, err
	}
	result, bindErr := session.service.Bind(ctx, preparedInput)
	resultView := manualDecisionBindingOutcomeFromService(result)
	resultView = session.projectInstitutedDecision(
		ctx,
		resultView,
	)
	revalidateErr := session.ledger.Revalidate(ctx)
	if revalidateErr != nil {
		revalidateErr = fmt.Errorf("revalidate checked project ledger after decision binding: %w", revalidateErr)
	}
	return resultView, errors.Join(bindErr, revalidateErr)
}

func (session *canonicalManualDecisionBindingSession) Resume(
	ctx context.Context,
	decisionRef string,
) (manualDecisionBindingOutcome, error) {
	if session == nil || session.ledger == nil || session.service == nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf("manual decision-binding session is closed")
	}
	if err := session.ledger.Revalidate(ctx); err != nil {
		return manualDecisionBindingOutcome{}, fmt.Errorf("revalidate checked project ledger before decision resume: %w", err)
	}
	result, resumeErr := session.service.Resume(ctx, decisionRef)
	resultView := manualDecisionBindingOutcomeFromService(result)
	resultView = session.projectInstitutedDecision(
		ctx,
		resultView,
	)
	revalidateErr := session.ledger.Revalidate(ctx)
	if revalidateErr != nil {
		revalidateErr = fmt.Errorf("revalidate checked project ledger after decision resume: %w", revalidateErr)
	}
	return resultView, errors.Join(resumeErr, revalidateErr)
}

func (session *canonicalManualDecisionBindingSession) projectInstitutedDecision(
	ctx context.Context,
	outcome manualDecisionBindingOutcome,
) manualDecisionBindingOutcome {
	if !outcome.EffectInstituted ||
		strings.TrimSpace(outcome.DecisionRef) == "" {
		return outcome
	}
	projection := projectExistingDecisionAfterBinding(
		ctx,
		session.ledger.ProjectID(),
		session.ledger.Database(),
		session.store,
		outcome.DecisionRef,
	)
	outcome.TaskMemoryProjection = &projection
	return outcome
}

func (session *canonicalManualDecisionBindingSession) Close() error {
	if session == nil || session.ledger == nil {
		return nil
	}
	err := session.ledger.Close()
	session.ledger = nil
	session.service = nil
	session.store = nil
	return err
}

func manualDecisionBindingOutcomeFromService(
	result decisionbinding.BindingResult,
) manualDecisionBindingOutcome {
	decisionRef := result.DecisionRef()
	title := result.Title()
	filePath := result.FilePath()
	effectInstituted := result.EffectDigest() != ""
	exactReplay := result.ExactReplay()
	warnings := result.Warnings()
	return manualDecisionBindingOutcome{
		DecisionRef:      decisionRef,
		Title:            title,
		FilePath:         filePath,
		EffectInstituted: effectInstituted,
		ExactReplay:      exactReplay,
		Warnings:         warnings,
	}
}

func resumableDecisionBindingError(
	result manualDecisionBindingOutcome,
	cause error,
) error {
	if cause == nil {
		return nil
	}
	decisionRef := strings.TrimSpace(result.DecisionRef)
	title := strings.TrimSpace(result.Title)
	if decisionRef == "" || title == "" {
		return cause
	}
	if result.EffectInstituted {
		return fmt.Errorf(
			"DecisionRecord %s — %q was instituted, but its checked-ledger verification did not complete: %w; inspect or safely replay with `haft artifact resume-decision %s`",
			decisionRef,
			title,
			cause,
			decisionRef,
		)
	}
	return fmt.Errorf(
		"DecisionRecord %s — %q was not instituted: %w; resume with `haft artifact resume-decision %s`",
		decisionRef,
		title,
		cause,
		decisionRef,
	)
}

func decisionBindingArtifactResult(
	result manualDecisionBindingOutcome,
) artifactCreateResult {
	return artifactCreateResult{
		Capability:  "decision.decide",
		ID:          result.DecisionRef,
		Kind:        string(artifact.KindDecisionRecord),
		Title:       result.Title,
		File:        result.FilePath,
		ExactReplay: result.ExactReplay,
		Warnings:    result.Warnings,
		TaskMemoryProjection: result.
			TaskMemoryProjection,
	}
}
