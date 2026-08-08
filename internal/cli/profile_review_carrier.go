package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
)

const (
	profileDeclarationReviewFileName = "profile-declaration-review.json"
	profileChangeReviewFileName      = "profile-change-review.json"
)

type profileReviewCandidate struct {
	Path            string   `json:"path"`
	State           string   `json:"state"`
	WorkInputRef    string   `json:"work_input_ref"`
	WorkInputDigest string   `json:"work_input_digest"`
	EditableFields  []string `json:"editable_fields"`
}

func prepareProfileReviewCandidate(
	projectRoot string,
	suggestion profiledetector.Suggestion,
) (profileReviewCandidate, error) {
	content, err := profileonboarding.ProposeProfileOnboardingWorkInput(
		suggestion,
	)
	if err != nil {
		return profileReviewCandidate{}, fmt.Errorf(
			"prepare project-profile review candidate: %w",
			err,
		)
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		content,
		suggestion,
	)
	if err != nil {
		return profileReviewCandidate{}, fmt.Errorf(
			"verify project-profile review candidate: %w",
			err,
		)
	}
	state, err := installProfileReviewCandidate(projectRoot, content)
	if err != nil {
		return profileReviewCandidate{}, err
	}
	return profileReviewCandidate{
		Path:            profileDeclarationReviewRelativePath(),
		State:           state,
		WorkInputRef:    input.Ref().String(),
		WorkInputDigest: input.Digest().String(),
		EditableFields: []string{
			"scopes[].scope_id",
			"scopes[].entity_ref",
			"scopes[].admitted_kind_ref",
			"scopes[].governing_pattern_refs",
			"scopes[].contract_refs",
		},
	}, nil
}

func installProfileReviewCandidate(
	projectRoot string,
	content []byte,
) (string, error) {
	target := profileDeclarationReviewPath(projectRoot)
	displayPath := profileDeclarationReviewRelativePath()
	stagePrefix := ".profile-declaration-review-stage-"
	current, present, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return "", err
	}
	if !present || bytes.Equal(current, content) {
		return installProfileReviewCandidateAt(
			target,
			displayPath,
			stagePrefix,
			content,
		)
	}
	_, generated := profiledeclarationpreparation.
		InspectGeneratedProfileReview(current)
	if !generated {
		return "", fmt.Errorf(
			"%s already contains a different review candidate; declare or deliberately remove that readable file before preparing another one",
			displayPath,
		)
	}
	return replaceGeneratedProfileReviewCandidateAt(
		target,
		displayPath,
		stagePrefix,
		current,
		content,
	)
}

func installProfileChangeReviewCandidate(
	projectRoot string,
	content []byte,
) (string, error) {
	return installProfileReviewCandidateAt(
		profileChangeReviewPath(projectRoot),
		profileChangeReviewRelativePath(),
		".profile-change-review-stage-",
		content,
	)
}

func consumeProfileChangeReviewCandidate(
	projectRoot string,
	expected []byte,
) error {
	target := profileChangeReviewPath(projectRoot)
	current, present, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf(
			"profile-change review changed while its exact effect was being applied; the newer carrier was retained",
		)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("consume applied profile-change review: %w", err)
	}
	if err := syncProfileReviewDirectory(filepath.Dir(target)); err != nil {
		return fmt.Errorf(
			"profile-change review was consumed but directory synchronization failed: %w",
			err,
		)
	}
	_, stillPresent, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return err
	}
	if stillPresent {
		return fmt.Errorf("applied profile-change review remains present")
	}
	return nil
}

func installProfileReviewCandidateAt(
	target string,
	displayPath string,
	stagePrefix string,
	content []byte,
) (string, error) {
	current, present, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return "", err
	}
	if present && bytes.Equal(current, content) {
		return "reused", nil
	}
	if present {
		return "", fmt.Errorf(
			"%s already contains a different review candidate; declare or deliberately remove that readable file before preparing another one",
			displayPath,
		)
	}
	directory := filepath.Dir(target)
	stage, err := os.CreateTemp(directory, stagePrefix)
	if err != nil {
		return "", fmt.Errorf("create project-profile review stage: %w", err)
	}
	stagePath := stage.Name()
	stageOpen := true
	defer func() {
		if stageOpen {
			_ = stage.Close()
		}
		_ = os.Remove(stagePath)
	}()
	if err := writeAndSyncProfileReviewStage(stage, content); err != nil {
		return "", err
	}
	stageOpen = false
	if err := os.Link(stagePath, target); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf(
				"atomically install project-profile review candidate: %w",
				err,
			)
		}
		observed, observedPresent, readErr := readOptionalRegularProfileReview(
			target,
		)
		if readErr != nil {
			return "", readErr
		}
		if observedPresent && bytes.Equal(observed, content) {
			return "reused", nil
		}
		return "", fmt.Errorf(
			"%s was concurrently populated with a different review candidate",
			displayPath,
		)
	}
	if err := syncProfileReviewDirectory(directory); err != nil {
		return "", fmt.Errorf(
			"project-profile review candidate may already exist, but directory synchronization failed: %w",
			err,
		)
	}
	installed, installedPresent, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return "", err
	}
	if !installedPresent || !bytes.Equal(installed, content) {
		return "", fmt.Errorf(
			"project-profile review candidate installation could not be verified",
		)
	}
	return "created", nil
}

func replaceGeneratedProfileReviewCandidateAt(
	target string,
	displayPath string,
	stagePrefix string,
	expected []byte,
	content []byte,
) (string, error) {
	directory := filepath.Dir(target)
	stage, err := os.CreateTemp(directory, stagePrefix)
	if err != nil {
		return "", fmt.Errorf("create project-profile review replacement stage: %w", err)
	}
	stagePath := stage.Name()
	stageOpen := true
	defer func() {
		if stageOpen {
			_ = stage.Close()
		}
		_ = os.Remove(stagePath)
	}()
	if err := writeAndSyncProfileReviewStage(stage, content); err != nil {
		return "", err
	}
	stageOpen = false
	current, present, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return "", err
	}
	if !present || !bytes.Equal(current, expected) {
		return "", fmt.Errorf(
			"%s changed while its generated candidate was being refreshed; the current readable file was retained",
			displayPath,
		)
	}
	_, generated := profiledeclarationpreparation.
		InspectGeneratedProfileReview(current)
	if !generated {
		return "", fmt.Errorf(
			"%s is no longer an unchanged Haft-generated review; the current readable file was retained",
			displayPath,
		)
	}
	if err := os.Rename(stagePath, target); err != nil {
		return "", fmt.Errorf("atomically refresh generated project-profile review candidate: %w", err)
	}
	if err := syncProfileReviewDirectory(directory); err != nil {
		return "", fmt.Errorf(
			"generated project-profile review may already be refreshed, but directory synchronization failed: %w",
			err,
		)
	}
	installed, installedPresent, err := readOptionalRegularProfileReview(target)
	if err != nil {
		return "", err
	}
	if !installedPresent || !bytes.Equal(installed, content) {
		return "", fmt.Errorf(
			"generated project-profile review refresh could not be verified",
		)
	}
	return "created", nil
}

func writeAndSyncProfileReviewStage(file *os.File, content []byte) error {
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write project-profile review stage: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync project-profile review stage: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close project-profile review stage: %w", err)
	}
	return nil
}

func syncProfileReviewDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func readOptionalRegularProfileReview(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect project-profile review candidate: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf(
			"project-profile review candidate must be a regular file, not %s",
			info.Mode().String(),
		)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read project-profile review candidate: %w", err)
	}
	return content, true, nil
}

func profileDeclarationReviewPath(projectRoot string) string {
	return filepath.Join(
		projectRoot,
		".haft",
		profileDeclarationReviewFileName,
	)
}

func profileDeclarationReviewRelativePath() string {
	return filepath.ToSlash(filepath.Join(".haft", profileDeclarationReviewFileName))
}

func profileChangeReviewPath(projectRoot string) string {
	return filepath.Join(
		projectRoot,
		".haft",
		profileChangeReviewFileName,
	)
}

func profileChangeReviewRelativePath() string {
	return filepath.ToSlash(filepath.Join(".haft", profileChangeReviewFileName))
}
