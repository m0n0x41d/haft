package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

type decisionBindingOutcome struct {
	DecisionRef          string
	Title                string
	FilePath             string
	EffectInstituted     bool
	ExactReplay          bool
	Warnings             []string
	TaskMemoryProjection *taskMemoryProjectionReport
}

var bindDecisionFromHostRequest = bindHostRoutedOperatorRequestDecision

// bindHostRoutedOperatorRequestDecision is the dedicated CLI effect sink. The
// host has already classified one direct, unambiguous operator request before
// calling it. The kernel records that routing provenance honestly; it does not
// claim to have independently established a U.SpeechAct occurrence.
func bindHostRoutedOperatorRequestDecision(
	ctx context.Context,
	projectRoot string,
	input artifact.DecideInput,
) (decisionBindingOutcome, error) {
	request, err := decisionOperatorRequest(input)
	if err != nil {
		return decisionBindingOutcome{}, err
	}
	input.AuthorityProvenance = string(request.Provenance())

	ledger, err := projectledger.OpenExisting(
		ctx,
		projectRoot,
		projectledger.ReadWrite,
	)
	if err != nil {
		return decisionBindingOutcome{}, fmt.Errorf(
			"open checked project ledger for host-routed decision binding: %w",
			err,
		)
	}
	defer ledger.Close()
	if err := ledger.Revalidate(ctx); err != nil {
		return decisionBindingOutcome{}, fmt.Errorf(
			"revalidate checked project ledger before decision preflight: %w",
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
		return decisionBindingOutcome{}, err
	}
	if err := ledger.Revalidate(ctx); err != nil {
		return decisionBindingOutcome{}, fmt.Errorf(
			"revalidate checked project ledger before decision write: %w",
			err,
		)
	}
	created, filePath, decideErr := artifact.Decide(
		ctx,
		store,
		haftDir,
		prepared,
	)
	warnings, err := decisionWriteWarnings(created, decideErr)
	if err != nil {
		return decisionBindingOutcome{}, err
	}
	projection := projectExistingDecisionAfterBinding(
		ctx,
		ledger.ProjectID(),
		ledger.Database(),
		store,
		created.Meta.ID,
	)
	outcome := decisionBindingOutcome{
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

func decisionOperatorRequest(
	input artifact.DecideInput,
) (operatorrequest.Request, error) {
	subject := strings.TrimSpace(input.DecisionSubjectRef)
	if subject == "" {
		subject = "decision:" + strings.TrimSpace(input.SelectedTitle)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return operatorrequest.Request{}, fmt.Errorf(
			"encode host-routed decision request: %w",
			err,
		)
	}
	request, err := operatorrequest.New(
		operatorrequest.DecisionBinding,
		subject,
		payload,
	)
	if err != nil {
		return operatorrequest.Request{}, fmt.Errorf(
			"validate host-routed decision request: %w",
			err,
		)
	}
	return request, nil
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
	return nil, fmt.Errorf("decision binding returned no DecisionRecord")
}

func decisionBindingArtifactResult(
	result decisionBindingOutcome,
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
