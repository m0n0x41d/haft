package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/m0n0x41d/haft/internal/initialprofilebootstrap"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func compilePublicInitialProfileBootstrapPlan(
	ctx context.Context,
	request publicInitRequest,
	databasePresent bool,
	carrierRoot string,
) (initplanning.InitialProfileBootstrapPlan, error) {
	suggestion, err := inspectPublicProfileSuggestion(request.projectRoot)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	currentProfileExists, err := observeAnyPublicProfileRevision(
		ctx,
		request.projectRoot,
		databasePresent,
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	reviewPath := filepath.Join(carrierRoot, profileDeclarationReviewFileName)
	reviewBytes, reviewPresent, err := readOptionalRegularProfileReview(reviewPath)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	reviewDisposition := initialprofilebootstrap.ReviewAbsent
	generatedReviewDigest := ""
	if reviewPresent {
		_, generated := profiledeclarationpreparation.
			InspectGeneratedProfileReview(reviewBytes)
		if generated {
			reviewDisposition = initialprofilebootstrap.ReviewGeneratedUnedited
			generatedReviewDigest, err = digestRegularFile(reviewPath)
			if err != nil {
				return initplanning.InitialProfileBootstrapPlan{}, err
			}
		} else {
			reviewDisposition = initialprofilebootstrap.ReviewHumanOrForeign
		}
	}
	decision, err := initialprofilebootstrap.Decide(
		currentProfileExists,
		reviewDisposition,
		suggestion,
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	observedFiles, err := publicInitialProfileObservedFiles(
		suggestion.Snapshot().ObservedFiles(),
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	input := initplanning.InitialProfileBootstrapInput{
		Kind:              initplanning.InitialProfileBootstrapKind(decision.Kind()),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		SuggestionRef:     suggestion.SuggestionRef(),
		ObservationDigest: suggestion.Snapshot().ObservationDigest(),
		Classification:    string(suggestion.Classification()),
		Confidence:        string(suggestion.ConfidencePosture()),
		ObservedFiles:     observedFiles,
		ScannedFileCount:  suggestion.Snapshot().ScannedFileCount(),
		Truncated:         suggestion.Snapshot().Truncated(),
	}
	if reason, ok := decision.ReviewReason(); ok {
		input.Reason = string(reason)
	}
	if generatedReviewDigest != "" {
		input.GeneratedReviewPath = reviewPath
		input.GeneratedReviewDigest = generatedReviewDigest
	}
	if decision.Kind() != initialprofilebootstrap.ApplySupportedSingleton {
		return initplanning.NewInitialProfileBootstrapPlan(input)
	}
	workInputJSON, err := profileonboarding.ProposeProfileOnboardingWorkInput(
		suggestion,
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	workInput, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		workInputJSON,
		suggestion,
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	scopes := workInput.Payload().Scopes().Values()
	if len(scopes) != 1 {
		return initplanning.InitialProfileBootstrapPlan{}, fmt.Errorf(
			"automatic profile WorkInput is not singleton",
		)
	}
	carrierInputs, err := publicProfileCoreFileInputsForPayload(
		carrierRoot,
		workInput.Payload(),
		time.Now().UTC(),
	)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	carrierEffects, err := publicCoreFileEffectsFromInputs(carrierInputs)
	if err != nil {
		return initplanning.InitialProfileBootstrapPlan{}, err
	}
	input.ScopeID = scopes[0].ScopeID().String()
	input.WorkInputRef = workInput.Ref().String()
	input.WorkInputDigest = workInput.Digest().String()
	input.WorkInputJSON = workInput.CanonicalJSON()
	input.PayloadDigest = workInput.PayloadDigest().String()
	input.PayloadJSON = workInput.PayloadCanonicalJSON()
	input.ContingentFileEffects = carrierEffects
	return initplanning.NewInitialProfileBootstrapPlan(input)
}

func inspectPublicProfileSuggestion(
	projectRoot string,
) (profiledetector.Suggestion, error) {
	_, err := os.Stat(projectRoot)
	if err == nil {
		return profiledetector.Inspect(projectRoot)
	}
	if !os.IsNotExist(err) {
		return profiledetector.Suggestion{}, err
	}
	if !filepath.IsAbs(projectRoot) || filepath.Clean(projectRoot) != projectRoot {
		return profiledetector.Suggestion{}, fmt.Errorf(
			"profile detector requires a canonical absolute project root",
		)
	}
	snapshot, err := profiledetector.NewObservedSnapshot(
		projectRoot,
		nil,
		0,
		false,
	)
	if err != nil {
		return profiledetector.Suggestion{}, err
	}
	return profiledetector.Detect(snapshot), nil
}

func publicInitialProfileObservedFiles(
	files []profiledetector.ObservedFile,
) ([]initplanning.InitialProfileObservedFile, error) {
	result := make([]initplanning.InitialProfileObservedFile, len(files))
	for index, file := range files {
		observed, err := initplanning.NewInitialProfileObservedFile(
			file.Path(),
			file.Size(),
		)
		if err != nil {
			return nil, err
		}
		result[index] = observed
	}
	return result, nil
}

func observeAnyPublicProfileRevision(
	ctx context.Context,
	projectRoot string,
	databasePresent bool,
) (bool, error) {
	if !databasePresent {
		return false, nil
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		projectRoot,
		projectledger.ReadOnly,
	)
	if err != nil {
		return false, fmt.Errorf("inspect existing profile before init: %w", err)
	}
	tables := []string{
		"project_profile_revisions",
		"project_profile_revisions_v2",
		"project_profile_revisions_v3",
		"project_profile_revisions_v4",
	}
	present := false
	for _, table := range tables {
		var exists int
		err = handle.Database().QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&exists)
		if err != nil {
			break
		}
		if exists == 0 {
			continue
		}
		var count int
		err = handle.Database().QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table,
		).Scan(&count)
		if err != nil {
			break
		}
		present = present || count > 0
	}
	closeErr := handle.Close()
	if err != nil || closeErr != nil {
		return false, fmt.Errorf(
			"inspect existing profile revisions: %w",
			errors.Join(err, closeErr),
		)
	}
	return present, nil
}

func publicProfileCoreFileInputsForPayload(
	carrierRoot string,
	payload projectprofile.ProfileDeclarationPayload,
	materializedAt time.Time,
) ([]publicCoreFileInput, error) {
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(payload)
	if err != nil {
		return nil, err
	}
	scopeIDs := matrix.ScopeIDs()
	if len(scopeIDs) != 1 {
		return nil, fmt.Errorf("initial profile carrier plan requires one scope")
	}
	applicability, err := project.DeriveProjectSpecificationSetApplicability(
		matrix,
		scopeIDs[0],
	)
	if err != nil {
		return nil, err
	}
	inputs, err := publicRequiredSpecCoreFileInputs(carrierRoot, applicability)
	if err != nil {
		return nil, err
	}
	methodApplicability, err := applicability.ScopedCapabilityApplicability(
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		return nil, err
	}
	if methodApplicability.Kind() != projectprofile.CapabilityRequired {
		return inputs, nil
	}
	methodInputs, err := publicMethodCoreFileInputs(carrierRoot, materializedAt)
	if err != nil {
		return nil, err
	}
	return append(inputs, methodInputs...), nil
}

func publicCoreFileEffectsFromInputs(
	inputs []publicCoreFileInput,
) ([]initplanning.CoreFileEffect, error) {
	effects := make([]initplanning.CoreFileEffect, len(inputs))
	for index, input := range inputs {
		exact, err := planPublicCoreFile(input)
		if err != nil {
			return nil, err
		}
		effect, err := initplanning.NewCoreFileEffect(
			initplanning.CoreFileEffectKind(exact.kind),
			exact.path,
			exact.content,
			exact.mode,
			exact.renderedDigest,
			exact.expectedDigest,
			exact.expectedMode,
		)
		if err != nil {
			return nil, err
		}
		effects[index] = effect
	}
	return effects, nil
}
