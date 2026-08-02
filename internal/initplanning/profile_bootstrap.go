package initplanning

import (
	"fmt"
	"path/filepath"
	"slices"
)

type InitialProfileBootstrapKind string

const (
	InitialProfileNotPlanned          InitialProfileBootstrapKind = "not_planned"
	InitialProfileKeepExisting        InitialProfileBootstrapKind = "keep_existing"
	InitialProfileApplySingleton      InitialProfileBootstrapKind = "apply_supported_singleton"
	InitialProfileHumanReviewRequired InitialProfileBootstrapKind = "human_review_required"
)

type InitialProfileObservedFile struct {
	path string
	size int64
}

func NewInitialProfileObservedFile(
	path string,
	size int64,
) (InitialProfileObservedFile, error) {
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path ||
		path == "." || size < 0 {
		return InitialProfileObservedFile{}, fmt.Errorf("initial profile observed file is invalid")
	}
	return InitialProfileObservedFile{path: path, size: size}, nil
}

func (file InitialProfileObservedFile) Path() string { return file.path }
func (file InitialProfileObservedFile) Size() int64  { return file.size }

type InitialProfileBootstrapInput struct {
	Kind                  InitialProfileBootstrapKind
	Reason                string
	DetectorVersion       string
	PolicyVersion         string
	SuggestionRef         string
	ObservationDigest     string
	Classification        string
	Confidence            string
	ObservedFiles         []InitialProfileObservedFile
	ScannedFileCount      int
	Truncated             bool
	ScopeID               string
	WorkInputRef          string
	WorkInputDigest       string
	WorkInputJSON         []byte
	PayloadDigest         string
	PayloadJSON           []byte
	ContingentFileEffects []CoreFileEffect
	GeneratedReviewPath   string
	GeneratedReviewDigest string
}

type InitialProfileBootstrapPlan struct {
	kind                  InitialProfileBootstrapKind
	reason                string
	detectorVersion       string
	policyVersion         string
	suggestionRef         string
	observationDigest     string
	classification        string
	confidence            string
	observedFiles         []InitialProfileObservedFile
	scannedFileCount      int
	truncated             bool
	scopeID               string
	workInputRef          string
	workInputDigest       string
	workInputJSON         []byte
	payloadDigest         string
	payloadJSON           []byte
	contingentFileEffects []CoreFileEffect
	generatedReviewPath   string
	generatedReviewDigest string
}

func NewInitialProfileBootstrapPlan(
	input InitialProfileBootstrapInput,
) (InitialProfileBootstrapPlan, error) {
	plan := InitialProfileBootstrapPlan{
		kind: input.Kind, reason: input.Reason,
		detectorVersion: input.DetectorVersion, policyVersion: input.PolicyVersion,
		suggestionRef: input.SuggestionRef, observationDigest: input.ObservationDigest,
		classification: input.Classification, confidence: input.Confidence,
		observedFiles:    slices.Clone(input.ObservedFiles),
		scannedFileCount: input.ScannedFileCount, truncated: input.Truncated,
		scopeID: input.ScopeID, workInputRef: input.WorkInputRef,
		workInputDigest:       input.WorkInputDigest,
		workInputJSON:         append([]byte{}, input.WorkInputJSON...),
		payloadDigest:         input.PayloadDigest,
		payloadJSON:           append([]byte{}, input.PayloadJSON...),
		contingentFileEffects: slices.Clone(input.ContingentFileEffects),
		generatedReviewPath:   input.GeneratedReviewPath,
		generatedReviewDigest: input.GeneratedReviewDigest,
	}
	if err := validateInitialProfileBootstrapPlan(plan); err != nil {
		return InitialProfileBootstrapPlan{}, err
	}
	return plan, nil
}

func validateInitialProfileBootstrapPlan(plan InitialProfileBootstrapPlan) error {
	if plan.kind == "" {
		return nil
	}
	if plan.kind == InitialProfileNotPlanned {
		if plan.detectorVersion == "" && len(plan.observedFiles) == 0 {
			return nil
		}
		return fmt.Errorf("unplanned initial profile bootstrap carries material")
	}
	if plan.kind != InitialProfileKeepExisting &&
		plan.kind != InitialProfileApplySingleton &&
		plan.kind != InitialProfileHumanReviewRequired {
		return fmt.Errorf("initial profile bootstrap kind is invalid")
	}
	if plan.detectorVersion == "" || plan.policyVersion == "" ||
		plan.suggestionRef == "" || plan.observationDigest == "" ||
		plan.classification == "" || plan.confidence == "" ||
		!sha256DigestPattern.MatchString(plan.observationDigest) ||
		plan.scannedFileCount < len(plan.observedFiles) {
		return fmt.Errorf("initial profile detector snapshot is invalid")
	}
	seen := map[string]struct{}{}
	for _, file := range plan.observedFiles {
		if file.path == "" || file.size < 0 {
			return fmt.Errorf("initial profile observed file is invalid")
		}
		if _, duplicate := seen[file.path]; duplicate {
			return fmt.Errorf("initial profile observed file is duplicated")
		}
		seen[file.path] = struct{}{}
	}
	if plan.kind == InitialProfileApplySingleton {
		if plan.truncated || plan.scopeID == "" || plan.workInputRef == "" ||
			!sha256DigestPattern.MatchString(plan.workInputDigest) ||
			len(plan.workInputJSON) == 0 ||
			!sha256DigestPattern.MatchString(plan.payloadDigest) ||
			len(plan.payloadJSON) == 0 || plan.reason != "" {
			return fmt.Errorf("automatic singleton profile bootstrap payload is invalid")
		}
		for _, effect := range plan.contingentFileEffects {
			if !effect.valid() {
				return fmt.Errorf("automatic singleton carrier effect is invalid")
			}
		}
	} else if plan.reason == "" || plan.scopeID != "" ||
		plan.workInputRef != "" || plan.workInputDigest != "" ||
		len(plan.workInputJSON) != 0 || plan.payloadDigest != "" ||
		len(plan.payloadJSON) != 0 || len(plan.contingentFileEffects) != 0 {
		return fmt.Errorf("non-applying initial profile bootstrap plan is invalid")
	}
	if (plan.generatedReviewPath == "") != (plan.generatedReviewDigest == "") {
		return fmt.Errorf("generated profile review precondition is incomplete")
	}
	if plan.generatedReviewPath != "" &&
		(!filepath.IsAbs(plan.generatedReviewPath) ||
			filepath.Clean(plan.generatedReviewPath) != plan.generatedReviewPath ||
			!sha256DigestPattern.MatchString(plan.generatedReviewDigest)) {
		return fmt.Errorf("generated profile review precondition is invalid")
	}
	return nil
}

func (plan InitialProfileBootstrapPlan) Kind() InitialProfileBootstrapKind {
	return plan.kind
}
func (plan InitialProfileBootstrapPlan) Reason() string { return plan.reason }
func (plan InitialProfileBootstrapPlan) DetectorVersion() string {
	return plan.detectorVersion
}
func (plan InitialProfileBootstrapPlan) PolicyVersion() string { return plan.policyVersion }
func (plan InitialProfileBootstrapPlan) SuggestionRef() string { return plan.suggestionRef }
func (plan InitialProfileBootstrapPlan) ObservationDigest() string {
	return plan.observationDigest
}
func (plan InitialProfileBootstrapPlan) Classification() string { return plan.classification }
func (plan InitialProfileBootstrapPlan) Confidence() string     { return plan.confidence }
func (plan InitialProfileBootstrapPlan) ObservedFiles() []InitialProfileObservedFile {
	return slices.Clone(plan.observedFiles)
}
func (plan InitialProfileBootstrapPlan) ScannedFileCount() int { return plan.scannedFileCount }
func (plan InitialProfileBootstrapPlan) Truncated() bool       { return plan.truncated }
func (plan InitialProfileBootstrapPlan) ScopeID() string       { return plan.scopeID }
func (plan InitialProfileBootstrapPlan) WorkInputRef() string  { return plan.workInputRef }
func (plan InitialProfileBootstrapPlan) WorkInputDigest() string {
	return plan.workInputDigest
}
func (plan InitialProfileBootstrapPlan) WorkInputJSON() []byte {
	return append([]byte{}, plan.workInputJSON...)
}
func (plan InitialProfileBootstrapPlan) PayloadDigest() string { return plan.payloadDigest }
func (plan InitialProfileBootstrapPlan) PayloadJSON() []byte {
	return append([]byte{}, plan.payloadJSON...)
}
func (plan InitialProfileBootstrapPlan) ContingentFileEffects() []CoreFileEffect {
	return slices.Clone(plan.contingentFileEffects)
}
func (plan InitialProfileBootstrapPlan) GeneratedReview() (string, string, bool) {
	if plan.generatedReviewPath == "" {
		return "", "", false
	}
	return plan.generatedReviewPath, plan.generatedReviewDigest, true
}
