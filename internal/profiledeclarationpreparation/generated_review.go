package profiledeclarationpreparation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

var generatedReviewDetectorVersions = []string{
	"haft.project-profile-detector/file-paths-v1",
	"haft.project-profile-detector/file-metadata-v2",
}

var generatedReviewPolicyVersions = []string{
	"haft.project-profile-detector-policy/file-paths-v1",
	"haft.project-profile-detector-policy/supported-singleton-v2",
}

// GeneratedProfileReview identifies an exact, unedited review carrier emitted
// by Haft. It is not a current detector result or an admission authority.
type GeneratedProfileReview struct {
	suggestionRef     string
	detectorVersion   string
	policyVersion     string
	observationDigest string
}

func (review GeneratedProfileReview) SuggestionRef() string {
	return review.suggestionRef
}

func (review GeneratedProfileReview) DetectorVersion() string {
	return review.detectorVersion
}

func (review GeneratedProfileReview) PolicyVersion() string {
	return review.policyVersion
}

func (review GeneratedProfileReview) ObservationDigest() string {
	return review.observationDigest
}

// InspectGeneratedProfileReview accepts only the exact pretty-printed shape
// produced by Haft with every operator-editable semantic field still empty.
// A false result is ordinary data: manual, enriched, or foreign reviews must
// remain visible and route init to human review.
func InspectGeneratedProfileReview(data []byte) (GeneratedProfileReview, bool) {
	dto := profileOnboardingWorkInputJSON{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return GeneratedProfileReview{}, false
	}
	if err := requireProfileWorkInputEOF(decoder); err != nil {
		return GeneratedProfileReview{}, false
	}
	if !generatedReviewDTO(dto) {
		return GeneratedProfileReview{}, false
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return GeneratedProfileReview{}, false
	}
	formatted := &bytes.Buffer{}
	if err := json.Indent(formatted, canonical, "", "  "); err != nil {
		return GeneratedProfileReview{}, false
	}
	formatted.WriteByte('\n')
	if !bytes.Equal(formatted.Bytes(), data) {
		return GeneratedProfileReview{}, false
	}
	return GeneratedProfileReview{
		suggestionRef:     dto.SuggestionRef,
		detectorVersion:   dto.DetectorVersion,
		policyVersion:     dto.PolicyVersion,
		observationDigest: dto.ObservationDigest,
	}, true
}

func generatedReviewDTO(dto profileOnboardingWorkInputJSON) bool {
	if dto.Schema != profileOnboardingWorkInputSchema ||
		strings.TrimSpace(dto.ProjectRoot) == "" ||
		strings.TrimSpace(dto.SuggestionRef) == "" ||
		strings.TrimSpace(dto.ObservationDigest) == "" ||
		!slices.Contains(generatedReviewDetectorVersions, dto.DetectorVersion) ||
		!slices.Contains(generatedReviewPolicyVersions, dto.PolicyVersion) ||
		dto.ObservationDetectorVersion != "" ||
		dto.ObservationPolicyVersion != "" ||
		dto.ProposalSource != "" ||
		dto.ManualBasis != "" ||
		len(dto.Scopes) == 0 {
		return false
	}
	for _, scope := range dto.Scopes {
		if !generatedReviewScope(scope) {
			return false
		}
	}
	return true
}

func generatedReviewScope(scope profileScopeDeclarationJSON) bool {
	if !strings.HasPrefix(
		scope.ComponentCandidateRef,
		"profile-component-suggestion:sha256:",
	) {
		return false
	}
	expectedKind := map[string]string{
		"software":  "software",
		"documents": "non_software",
		"models":    "non_software",
	}[scope.ScopeID]
	return expectedKind != "" &&
		scope.RealizationKind == expectedKind &&
		scope.Label == "" &&
		scope.EntityRef == "" &&
		scope.AdmittedKindRef == "" &&
		len(scope.GoverningPatternRefs) == 0 &&
		len(scope.ContractRefs) == 0 &&
		len(scope.EvidencePaths) == 0
}

func formatGeneratedProfileReviewForTest(dto profileOnboardingWorkInputJSON) ([]byte, error) {
	canonical, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	formatted := &bytes.Buffer{}
	if err := json.Indent(formatted, canonical, "", "  "); err != nil {
		return nil, fmt.Errorf("format generated review: %w", err)
	}
	formatted.WriteByte('\n')
	return formatted.Bytes(), nil
}
