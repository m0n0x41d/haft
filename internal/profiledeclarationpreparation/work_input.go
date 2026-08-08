package profiledeclarationpreparation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	profileOnboardingWorkInputSchema = "haft.profile-onboarding.work-input/v1"
	profileOnboardingWorkInputDomain = "haft.profile-onboarding.work-input/v1"
	profileOnboardingWorkInputPrefix = "profile-onboarding-work-input:"
	profileProposalSourceManual      = "manual_scope_proposal"
	profileProposalSourceRelation    = "profile_entity_relation_change"
	profileManualClassifierVersion   = "haft-profile-manual-scope/v1"
	profileManualPolicyVersion       = "haft-profile-manual-scope-policy/v1"
	profileRelationClassifierVersion = "haft-profile-entity-relation-change/v1"
	profileRelationPolicyVersion     = "haft-profile-entity-relation-change-policy/v1"
	maximumProfileWorkInputBytes     = 256 * 1024
	maximumManualProfileBasisBytes   = 4 * 1024
)

// ProfileOnboardingWorkInput is the one reliance-bearing input to profile
// classification Work. It binds an exact repository observation to stable
// project scope identities presented for explicit review. It is neither a
// detector suggestion, performed Work, authority, nor a canonical profile
// admission.
type ProfileOnboardingWorkInput struct {
	state *profileOnboardingWorkInputState
}

type profileOnboardingWorkInputState struct {
	projectRoot                projectprofile.ProjectRootV1
	suggestionRef              string
	detectorVersion            string
	policyVersion              string
	observationDetectorVersion string
	observationPolicyVersion   string
	observationDigest          string
	proposalSource             string
	manualBasis                string
	changeBasis                *ProfileChangeBasis
	scopeBindings              []profileScopeBinding
	payload                    projectprofile.ProfileDeclarationPayload
	ref                        projectprofile.WorkInputRef
	digest                     projectprofile.ContentDigest
	canonicalJSON              []byte
}

type profileScopeBinding struct {
	componentCandidateRef string
	scope                 projectprofile.RealizationScope
}

type profileOnboardingWorkInputJSON struct {
	Schema                     string                        `json:"schema"`
	ProjectRoot                string                        `json:"project_root"`
	SuggestionRef              string                        `json:"suggestion_ref"`
	DetectorVersion            string                        `json:"detector_version"`
	PolicyVersion              string                        `json:"policy_version"`
	ObservationDetectorVersion string                        `json:"observation_detector_version,omitempty"`
	ObservationPolicyVersion   string                        `json:"observation_policy_version,omitempty"`
	ObservationDigest          string                        `json:"observation_digest"`
	ProposalSource             string                        `json:"proposal_source,omitempty"`
	ManualBasis                string                        `json:"manual_basis,omitempty"`
	ChangeBasis                *profileChangeBasisJSON       `json:"change_basis,omitempty"`
	Scopes                     []profileScopeDeclarationJSON `json:"scopes"`
}

type profileChangeBasisJSON struct {
	AdmissionRecordRef    string `json:"admission_record_ref"`
	AdmissionRecordDigest string `json:"admission_record_digest"`
	PayloadDigest         string `json:"payload_digest"`
	LedgerRevision        uint64 `json:"ledger_revision"`
	ScopeID               string `json:"scope_id"`
	PreviousEntityRef     string `json:"previous_entity_ref,omitempty"`
	NextEntityRef         string `json:"next_entity_ref"`
}

// ProfileChangeBasis pins one bounded successor review to the exact current
// admission that was inspected. It is review evidence, not authority and not
// a mutation receipt.
type ProfileChangeBasis struct {
	admissionRecordRef    projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionRecordDigest projectprofile.ContentDigest
	payloadDigest         projectprofile.ContentDigest
	ledgerRevision        projectprofile.LedgerRevision
	scopeID               projectprofile.ScopeID
	previousEntityRef     string
	nextEntityRef         projectprofile.EntityRef
}

func NewProfileChangeBasis(
	admissionRecordRef projectprofile.ProfileDeclarationAdmissionRecordRef,
	admissionRecordDigest projectprofile.ContentDigest,
	payloadDigest projectprofile.ContentDigest,
	ledgerRevision projectprofile.LedgerRevision,
	scopeID projectprofile.ScopeID,
	previousEntityRef string,
	nextEntityRef projectprofile.EntityRef,
) (ProfileChangeBasis, error) {
	if admissionRecordRef.String() == "" ||
		admissionRecordDigest.String() == "" ||
		payloadDigest.String() == "" ||
		ledgerRevision.Value() == 0 ||
		scopeID.String() == "" ||
		nextEntityRef.String() == "" {
		return ProfileChangeBasis{}, fmt.Errorf(
			"profile change basis requires exact predecessor coordinates and one next entity_ref",
		)
	}
	if previousEntityRef != "" {
		if _, err := projectprofile.NewEntityRef(previousEntityRef); err != nil {
			return ProfileChangeBasis{}, err
		}
	}
	if previousEntityRef == nextEntityRef.String() {
		return ProfileChangeBasis{}, fmt.Errorf(
			"profile entity relation change must select a different entity_ref",
		)
	}
	if _, err := ledgerRevision.Next(); err != nil {
		return ProfileChangeBasis{}, err
	}
	return ProfileChangeBasis{
		admissionRecordRef:    admissionRecordRef,
		admissionRecordDigest: admissionRecordDigest,
		payloadDigest:         payloadDigest,
		ledgerRevision:        ledgerRevision,
		scopeID:               scopeID,
		previousEntityRef:     previousEntityRef,
		nextEntityRef:         nextEntityRef,
	}, nil
}

func (basis ProfileChangeBasis) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	return basis.admissionRecordRef
}

func (basis ProfileChangeBasis) AdmissionRecordDigest() projectprofile.ContentDigest {
	return basis.admissionRecordDigest
}

func (basis ProfileChangeBasis) PayloadDigest() projectprofile.ContentDigest {
	return basis.payloadDigest
}

func (basis ProfileChangeBasis) LedgerRevision() projectprofile.LedgerRevision {
	return basis.ledgerRevision
}

func (basis ProfileChangeBasis) ScopeID() projectprofile.ScopeID {
	return basis.scopeID
}

func (basis ProfileChangeBasis) PreviousEntityRef() string {
	return basis.previousEntityRef
}

func (basis ProfileChangeBasis) NextEntityRef() projectprofile.EntityRef {
	return basis.nextEntityRef
}

type profileScopeDeclarationJSON struct {
	ComponentCandidateRef string   `json:"component_candidate_ref"`
	ScopeID               string   `json:"scope_id"`
	Label                 string   `json:"label,omitempty"`
	RealizationKind       string   `json:"realization_kind"`
	EntityRef             string   `json:"entity_ref,omitempty"`
	AdmittedKindRef       string   `json:"admitted_kind_ref,omitempty"`
	GoverningPatternRefs  []string `json:"governing_pattern_refs,omitempty"`
	ContractRefs          []string `json:"contract_refs,omitempty"`
	EvidencePaths         []string `json:"evidence_paths,omitempty"`
}

// ManualProfileScopeInput is an operator-reviewable fallback scope used only
// when a complete detector observation cannot establish stable scope identity.
// The paths are observation references, not evidence-truth claims.
type ManualProfileScopeInput struct {
	ScopeID         string
	Label           string
	RealizationKind profiledetector.RealizationKind
	EvidencePaths   []string
}

// ManualProfileProposalInput supplies the readable rationale and exact scopes
// that the detector could not infer. It remains non-binding until the ordinary
// explicit profile declaration boundary consumes the reviewed carrier.
type ManualProfileProposalInput struct {
	Basis  string
	Scopes []ManualProfileScopeInput
}

// DecodeProfileOnboardingWorkInput parses one readable input-file document,
// rejects unknown fields, and seals it against the detector observation that
// exists at declaration time. Derived refs and digests are never typed by the
// operator.
func DecodeProfileOnboardingWorkInput(
	data []byte,
	suggestion profiledetector.Suggestion,
) (ProfileOnboardingWorkInput, error) {
	if len(data) == 0 || len(data) > maximumProfileWorkInputBytes {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work input must contain 1..%d bytes",
			maximumProfileWorkInputBytes,
		)
	}
	dto := profileOnboardingWorkInputJSON{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"decode profile onboarding Work input: %w",
			err,
		)
	}
	if err := requireProfileWorkInputEOF(decoder); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	return newProfileOnboardingWorkInput(dto, suggestion)
}

// ProposeProfileOnboardingWorkInput produces the readable, non-binding review
// carrier for the current detector observation. Scope IDs are stable semantic
// orientation names (software, documents, models); the operator or agent may
// edit semantic fields before declaration. The resulting bytes contain no
// authority receipt and perform no mutation by themselves.
func ProposeProfileOnboardingWorkInput(
	suggestion profiledetector.Suggestion,
) ([]byte, error) {
	snapshot := suggestion.Snapshot()
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.StableScopeIdentity {
		return nil, fmt.Errorf(
			"profile detector basis is insufficient for stable scopes; no declaration proposal can be prepared",
		)
	}
	suggested := suggestion.SuggestedScopes()
	if len(suggested) == 0 {
		return nil, fmt.Errorf(
			"profile detector returned no scope candidates",
		)
	}
	scopes := make([]profileScopeDeclarationJSON, len(suggested))
	for index, scope := range suggested {
		scopes[index] = profileScopeDeclarationJSON{
			ComponentCandidateRef: scope.ComponentCandidateRef(),
			ScopeID:               scope.Orientation(),
			RealizationKind:       string(scope.RealizationKind()),
		}
	}
	dto := profileOnboardingWorkInputJSON{
		Schema:            profileOnboardingWorkInputSchema,
		ProjectRoot:       snapshot.ProjectRoot(),
		SuggestionRef:     suggestion.SuggestionRef(),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		ObservationDigest: snapshot.ObservationDigest(),
		Scopes:            scopes,
	}
	input, err := newProfileOnboardingWorkInput(dto, suggestion)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	if err := json.Indent(buffer, input.CanonicalJSON(), "", "  "); err != nil {
		return nil, fmt.Errorf("format profile declaration proposal: %w", err)
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}

// ProposeManualProfileOnboardingWorkInput produces a non-binding review
// carrier for a complete detector snapshot whose language, repository shape,
// or conflicting signals leave scope identity underdetermined. It never
// guesses a scope: the caller supplies the readable basis and exact
// realization kinds.
func ProposeManualProfileOnboardingWorkInput(
	suggestion profiledetector.Suggestion,
	proposal ManualProfileProposalInput,
) ([]byte, error) {
	snapshot := suggestion.Snapshot()
	if snapshot.Truncated() {
		return nil, fmt.Errorf(
			"manual profile fallback requires a complete repository observation",
		)
	}
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.ScopeIdentityNeedsReview {
		return nil, fmt.Errorf(
			"manual profile fallback is available only when detector scope identity needs review",
		)
	}
	basis, err := validateManualProfileBasis(proposal.Basis)
	if err != nil {
		return nil, err
	}
	scopes, err := manualProfileScopeDeclarations(
		proposal.Scopes,
		snapshot,
	)
	if err != nil {
		return nil, err
	}
	dto := profileOnboardingWorkInputJSON{
		Schema:                     profileOnboardingWorkInputSchema,
		ProjectRoot:                snapshot.ProjectRoot(),
		SuggestionRef:              suggestion.SuggestionRef(),
		DetectorVersion:            profileManualClassifierVersion,
		PolicyVersion:              profileManualPolicyVersion,
		ObservationDetectorVersion: suggestion.DetectorVersion(),
		ObservationPolicyVersion:   profiledetector.PolicyVersion,
		ObservationDigest:          snapshot.ObservationDigest(),
		ProposalSource:             profileProposalSourceManual,
		ManualBasis:                basis,
		Scopes:                     scopes,
	}
	input, err := newProfileOnboardingWorkInput(dto, suggestion)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	if err := json.Indent(
		buffer,
		input.CanonicalJSON(),
		"",
		"  ",
	); err != nil {
		return nil, fmt.Errorf(
			"format manual profile declaration proposal: %w",
			err,
		)
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}

// ProposeProfileEntityRelationChangeWorkInput prepares one non-binding,
// predecessor-pinned successor review. The only semantic delta it can express
// is changing entity_ref on one existing realization scope.
func ProposeProfileEntityRelationChangeWorkInput(
	suggestion profiledetector.Suggestion,
	current projectprofile.ProfileDeclarationPayload,
	basis ProfileChangeBasis,
) ([]byte, error) {
	snapshot := suggestion.Snapshot()
	if snapshot.Truncated() {
		return nil, fmt.Errorf(
			"profile relation change requires a complete repository observation",
		)
	}
	currentDigest, err := projectprofile.DigestProfileDeclarationPayload(current)
	if err != nil {
		return nil, err
	}
	if currentDigest != basis.PayloadDigest() {
		return nil, fmt.Errorf(
			"profile relation change basis does not match the current payload",
		)
	}
	scopes, previous, err := profileRelationChangeScopes(current, basis)
	if err != nil {
		return nil, err
	}
	if previous != basis.PreviousEntityRef() {
		return nil, fmt.Errorf(
			"profile relation change basis carries another previous entity_ref",
		)
	}
	dto := profileOnboardingWorkInputJSON{
		Schema:                     profileOnboardingWorkInputSchema,
		ProjectRoot:                snapshot.ProjectRoot(),
		SuggestionRef:              suggestion.SuggestionRef(),
		DetectorVersion:            profileRelationClassifierVersion,
		PolicyVersion:              profileRelationPolicyVersion,
		ObservationDetectorVersion: suggestion.DetectorVersion(),
		ObservationPolicyVersion:   profiledetector.PolicyVersion,
		ObservationDigest:          snapshot.ObservationDigest(),
		ProposalSource:             profileProposalSourceRelation,
		ChangeBasis:                profileChangeBasisToJSON(basis),
		Scopes:                     scopes,
	}
	input, err := newProfileOnboardingWorkInput(dto, suggestion)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	if err := json.Indent(buffer, input.CanonicalJSON(), "", "  "); err != nil {
		return nil, fmt.Errorf(
			"format profile relation change review: %w",
			err,
		)
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}

func profileRelationChangeScopes(
	current projectprofile.ProfileDeclarationPayload,
	basis ProfileChangeBasis,
) ([]profileScopeDeclarationJSON, string, error) {
	values := current.Scopes().Values()
	result := make([]profileScopeDeclarationJSON, len(values))
	previous := ""
	found := false
	for index, value := range values {
		scope, entityRef, err := profileRelationScopeJSON(value, basis)
		if err != nil {
			return nil, "", err
		}
		if value.ScopeID() == basis.ScopeID() {
			previous = entityRef
			scope.EntityRef = basis.NextEntityRef().String()
			found = true
		}
		result[index] = scope
	}
	if !found {
		return nil, "", fmt.Errorf(
			"profile relation change scope_id %q is absent from the current profile",
			basis.ScopeID().String(),
		)
	}
	return result, previous, nil
}

func profileRelationScopeJSON(
	value projectprofile.RealizationScope,
	basis ProfileChangeBasis,
) (profileScopeDeclarationJSON, string, error) {
	componentRef := profileChangeComponentCandidateRef(
		basis.PayloadDigest().String(),
		value.ScopeID().String(),
	)
	switch scope := value.(type) {
	case projectprofile.SoftwareRealization:
		entityRef, err := profileEntityReferenceText(scope.EntityReference())
		return profileScopeDeclarationJSON{
			ComponentCandidateRef: componentRef,
			ScopeID:               scope.ScopeID().String(),
			RealizationKind:       string(profiledetector.SoftwareRealization),
			EntityRef:             entityRef,
		}, entityRef, err
	case projectprofile.NonSoftwareRealization:
		entityRef, err := profileEntityReferenceText(scope.EntityReference())
		if err != nil {
			return profileScopeDeclarationJSON{}, "", err
		}
		kindRef, err := profileKindOrientationText(scope.KindOrientation())
		if err != nil {
			return profileScopeDeclarationJSON{}, "", err
		}
		patterns := make([]string, len(scope.GoverningPatternRefs()))
		for index, ref := range scope.GoverningPatternRefs() {
			patterns[index] = ref.String()
		}
		contracts := make([]string, len(scope.ContractRefs()))
		for index, ref := range scope.ContractRefs() {
			contracts[index] = ref.String()
		}
		return profileScopeDeclarationJSON{
			ComponentCandidateRef: componentRef,
			ScopeID:               scope.ScopeID().String(),
			RealizationKind:       string(profiledetector.NonSoftwareRealization),
			EntityRef:             entityRef,
			AdmittedKindRef:       kindRef,
			GoverningPatternRefs:  patterns,
			ContractRefs:          contracts,
		}, entityRef, nil
	default:
		return profileScopeDeclarationJSON{}, "", fmt.Errorf(
			"profile relation change found an unsupported realization scope",
		)
	}
}

func profileEntityReferenceText(
	reference projectprofile.EntityReference,
) (string, error) {
	switch value := reference.(type) {
	case projectprofile.NoEntityReference:
		return "", nil
	case projectprofile.ReferencedEntity:
		return value.Ref().String(), nil
	default:
		return "", fmt.Errorf("profile entity reference variant is unsupported")
	}
}

func profileKindOrientationText(
	orientation projectprofile.KindOrientation,
) (string, error) {
	switch value := orientation.(type) {
	case projectprofile.UnspecifiedKindOrientation:
		return "", nil
	case projectprofile.ReferencedKindOrientation:
		return value.Ref().String(), nil
	default:
		return "", fmt.Errorf("profile kind orientation variant is unsupported")
	}
}

func profileChangeComponentCandidateRef(
	payloadDigest string,
	scopeID string,
) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("haft.profile-change-component/v1"))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(payloadDigest))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(scopeID))
	return "profile-change-component:sha256:" +
		hex.EncodeToString(hash.Sum(nil))
}

func profileChangeBasisToJSON(
	basis ProfileChangeBasis,
) *profileChangeBasisJSON {
	return &profileChangeBasisJSON{
		AdmissionRecordRef:    basis.AdmissionRecordRef().String(),
		AdmissionRecordDigest: basis.AdmissionRecordDigest().String(),
		PayloadDigest:         basis.PayloadDigest().String(),
		LedgerRevision:        basis.LedgerRevision().Value(),
		ScopeID:               basis.ScopeID().String(),
		PreviousEntityRef:     basis.PreviousEntityRef(),
		NextEntityRef:         basis.NextEntityRef().String(),
	}
}

func validateManualProfileBasis(raw string) (string, error) {
	basis := strings.TrimSpace(raw)
	if basis == "" || basis != raw ||
		len([]byte(basis)) > maximumManualProfileBasisBytes {
		return "", fmt.Errorf(
			"manual profile basis must be an exact non-empty string of at most %d bytes",
			maximumManualProfileBasisBytes,
		)
	}
	return basis, nil
}

func manualProfileScopeDeclarations(
	inputs []ManualProfileScopeInput,
	snapshot profiledetector.Snapshot,
) ([]profileScopeDeclarationJSON, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf(
			"manual profile fallback requires at least one explicit scope",
		)
	}
	observedPaths := make(
		map[string]struct{},
		len(snapshot.RelativeFiles()),
	)
	for _, path := range snapshot.RelativeFiles() {
		observedPaths[path] = struct{}{}
	}
	result := make(
		[]profileScopeDeclarationJSON,
		len(inputs),
	)
	for index, input := range inputs {
		scopeID, err := projectprofile.NewScopeID(input.ScopeID)
		if err != nil {
			return nil, fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		if input.RealizationKind !=
			profiledetector.SoftwareRealization &&
			input.RealizationKind !=
				profiledetector.NonSoftwareRealization {
			return nil, fmt.Errorf(
				"manual profile scope %d realization_kind must be %q or %q",
				index,
				profiledetector.SoftwareRealization,
				profiledetector.NonSoftwareRealization,
			)
		}
		label, err := validateManualProfileLabel(input.Label)
		if err != nil {
			return nil, fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		evidence, err := canonicalManualEvidencePaths(
			input.EvidencePaths,
			observedPaths,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		componentRef := manualComponentCandidateRef(
			snapshot.ObservationDigest(),
			scopeID.String(),
			label,
			input.RealizationKind,
			evidence,
		)
		result[index] = profileScopeDeclarationJSON{
			ComponentCandidateRef: componentRef,
			ScopeID:               scopeID.String(),
			Label:                 label,
			RealizationKind:       string(input.RealizationKind),
			EvidencePaths:         evidence,
		}
	}
	return result, nil
}

func validateManualProfileLabel(raw string) (string, error) {
	label := strings.TrimSpace(raw)
	if label == "" || label != raw ||
		len([]byte(label)) > 200 {
		return "", fmt.Errorf(
			"label must be an exact non-empty string of at most 200 bytes",
		)
	}
	return label, nil
}

func canonicalManualEvidencePaths(
	inputs []string,
	observed map[string]struct{},
) ([]string, error) {
	if len(observed) > 0 && len(inputs) == 0 {
		return nil, fmt.Errorf(
			"manual evidence paths are required for a non-empty repository observation",
		)
	}
	result := append([]string{}, inputs...)
	for _, path := range result {
		if path == "" || path != strings.TrimSpace(path) {
			return nil, fmt.Errorf(
				"manual evidence paths must be exact non-empty repository-relative paths",
			)
		}
		if _, exists := observed[path]; !exists {
			return nil, fmt.Errorf(
				"manual evidence path %q is absent from the current repository observation",
				path,
			)
		}
	}
	slices.Sort(result)
	result = slices.Compact(result)
	if len(result) != len(inputs) {
		return nil, fmt.Errorf(
			"manual evidence paths must not repeat",
		)
	}
	return result, nil
}

func manualComponentCandidateRef(
	observationDigest string,
	scopeID string,
	label string,
	kind profiledetector.RealizationKind,
	evidence []string,
) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("haft.manual-profile-component/v1"))
	for _, value := range append(
		[]string{
			observationDigest,
			scopeID,
			label,
			string(kind),
		},
		evidence...,
	) {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(value))
	}
	return "manual-component-candidate:sha256:" +
		hex.EncodeToString(hash.Sum(nil))
}

func requireProfileWorkInputEOF(decoder *json.Decoder) error {
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing profile onboarding input: %w", err)
	}
	return fmt.Errorf("profile onboarding Work input contains multiple JSON values")
}

func newProfileOnboardingWorkInput(
	dto profileOnboardingWorkInputJSON,
	suggestion profiledetector.Suggestion,
) (ProfileOnboardingWorkInput, error) {
	if err := validateProfileSuggestionBinding(dto, suggestion); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	input, err := sealProfileOnboardingWorkInput(dto)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	switch dto.ProposalSource {
	case profileProposalSourceManual:
		if err := validateManualProfileScopeCoverage(
			dto,
			suggestion,
		); err != nil {
			return ProfileOnboardingWorkInput{}, err
		}
		return input, nil
	case profileProposalSourceRelation:
		if err := validateProfileRelationChangeCoverage(dto); err != nil {
			return ProfileOnboardingWorkInput{}, err
		}
		return input, nil
	}
	if err := validateProfileScopeCoverage(input.state.scopeBindings, suggestion); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	return input, nil
}

func sealProfileOnboardingWorkInput(
	dto profileOnboardingWorkInputJSON,
) (ProfileOnboardingWorkInput, error) {
	if dto.Schema != profileOnboardingWorkInputSchema {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"unsupported profile onboarding Work input schema %q",
			dto.Schema,
		)
	}
	root, err := projectprofile.NewProjectRootV1(dto.ProjectRoot)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	if strings.TrimSpace(dto.SuggestionRef) == "" ||
		dto.SuggestionRef != strings.TrimSpace(dto.SuggestionRef) {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work input suggestion_ref is invalid",
		)
	}
	if strings.TrimSpace(dto.DetectorVersion) == "" ||
		strings.TrimSpace(dto.PolicyVersion) == "" ||
		strings.TrimSpace(dto.ObservationDigest) == "" {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work input detector coordinates are incomplete",
		)
	}
	if err := validateProfileProposalSource(dto); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	bindings, err := profileScopeBindings(dto.Scopes)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	payload, err := profilePayloadFromBindings(bindings)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	canonicalDTO := canonicalProfileWorkInputDTO(dto, bindings)
	canonical, err := json.Marshal(canonicalDTO)
	if err != nil {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"encode canonical profile onboarding Work input: %w",
			err,
		)
	}
	digest, ref, err := profileWorkInputIdentity(canonical)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	state := &profileOnboardingWorkInputState{
		projectRoot:                root,
		suggestionRef:              dto.SuggestionRef,
		detectorVersion:            dto.DetectorVersion,
		policyVersion:              dto.PolicyVersion,
		observationDetectorVersion: profileObservationDetectorVersion(dto),
		observationPolicyVersion:   profileObservationPolicyVersion(dto),
		observationDigest:          dto.ObservationDigest,
		proposalSource:             dto.ProposalSource,
		manualBasis:                dto.ManualBasis,
		changeBasis:                profileChangeBasisFromDTO(dto.ChangeBasis),
		scopeBindings:              append([]profileScopeBinding{}, bindings...),
		payload:                    payload,
		ref:                        ref,
		digest:                     digest,
		canonicalJSON:              append([]byte(nil), canonical...),
	}
	return ProfileOnboardingWorkInput{state: state}, nil
}

func decodeCanonicalProfileOnboardingWorkInput(
	data []byte,
) (ProfileOnboardingWorkInput, error) {
	if len(data) == 0 || len(data) > maximumProfileWorkInputBytes {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"canonical profile onboarding Work input has invalid size",
		)
	}
	dto := profileOnboardingWorkInputJSON{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	if err := requireProfileWorkInputEOF(decoder); err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	input, err := sealProfileOnboardingWorkInput(dto)
	if err != nil {
		return ProfileOnboardingWorkInput{}, err
	}
	if !bytes.Equal(input.CanonicalJSON(), data) {
		return ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work input JSON is not canonical",
		)
	}
	return input, nil
}

// DecodeCanonicalProfileOnboardingWorkInput is the strict durable-reload
// boundary. It accepts only bytes produced by the canonical reviewed-input
// codec; it is not a second weak input parser.
func DecodeCanonicalProfileOnboardingWorkInput(
	data []byte,
) (ProfileOnboardingWorkInput, error) {
	return decodeCanonicalProfileOnboardingWorkInput(data)
}

func validateProfileSuggestionBinding(
	dto profileOnboardingWorkInputJSON,
	suggestion profiledetector.Suggestion,
) error {
	snapshot := suggestion.Snapshot()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: dto.ProjectRoot == snapshot.ProjectRoot(), name: "project_root"},
		{matches: dto.SuggestionRef == suggestion.SuggestionRef(), name: "suggestion_ref"},
		{
			matches: profileObservationDetectorVersion(dto) ==
				suggestion.DetectorVersion(),
			name: "observation_detector_version",
		},
		{
			matches: profileObservationPolicyVersion(dto) ==
				profiledetector.PolicyVersion,
			name: "observation_policy_version",
		},
		{matches: dto.ObservationDigest == snapshot.ObservationDigest(), name: "observation_digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"profile onboarding Work input %s differs from the current detector observation",
				check.name,
			)
		}
	}
	if snapshot.Truncated() {
		return fmt.Errorf(
			"profile detector basis is insufficient; declaration requires an explicit complete observation",
		)
	}
	if dto.ProposalSource == profileProposalSourceManual {
		if suggestion.ScopeIdentityPosture() !=
			profiledetector.ScopeIdentityNeedsReview {
			return fmt.Errorf(
				"manual profile fallback requires detector scope identity that needs review",
			)
		}
		return nil
	}
	if dto.ProposalSource == profileProposalSourceRelation {
		return nil
	}
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.StableScopeIdentity {
		return fmt.Errorf(
			"profile detector basis is insufficient for stable scopes; declaration requires an explicit reviewed manual scope",
		)
	}
	return nil
}

func validateProfileProposalSource(
	dto profileOnboardingWorkInputJSON,
) error {
	if dto.ProposalSource == "" {
		if dto.ChangeBasis != nil {
			return fmt.Errorf(
				"detector profile proposal cannot carry change_basis",
			)
		}
		if dto.ObservationDetectorVersion != "" ||
			dto.ObservationPolicyVersion != "" {
			return fmt.Errorf(
				"detector profile proposal cannot carry separate observation coordinates",
			)
		}
		if dto.ManualBasis != "" {
			return fmt.Errorf(
				"detector profile proposal cannot carry manual_basis",
			)
		}
		for _, scope := range dto.Scopes {
			if scope.Label != "" ||
				len(scope.EvidencePaths) != 0 {
				return fmt.Errorf(
					"detector profile proposal cannot carry a manual label or evidence paths",
				)
			}
		}
		return nil
	}
	if dto.ProposalSource == profileProposalSourceRelation {
		if dto.ManualBasis != "" {
			return fmt.Errorf(
				"profile relation change cannot carry manual_basis",
			)
		}
		if dto.DetectorVersion != profileRelationClassifierVersion ||
			dto.PolicyVersion != profileRelationPolicyVersion {
			return fmt.Errorf(
				"profile relation change classifier coordinates are invalid",
			)
		}
		if strings.TrimSpace(dto.ObservationDetectorVersion) == "" ||
			strings.TrimSpace(dto.ObservationPolicyVersion) == "" {
			return fmt.Errorf(
				"profile relation change requires exact observation detector coordinates",
			)
		}
		_, err := profileChangeBasisFromJSON(dto.ChangeBasis)
		return err
	}
	if dto.ProposalSource != profileProposalSourceManual {
		return fmt.Errorf(
			"profile onboarding Work input proposal_source is invalid",
		)
	}
	if dto.ChangeBasis != nil {
		return fmt.Errorf(
			"manual profile fallback cannot carry change_basis",
		)
	}
	if _, err := validateManualProfileBasis(dto.ManualBasis); err != nil {
		return err
	}
	if dto.DetectorVersion != profileManualClassifierVersion ||
		dto.PolicyVersion != profileManualPolicyVersion {
		return fmt.Errorf(
			"manual profile fallback classifier coordinates are invalid",
		)
	}
	if strings.TrimSpace(dto.ObservationDetectorVersion) == "" ||
		strings.TrimSpace(dto.ObservationPolicyVersion) == "" {
		return fmt.Errorf(
			"manual profile proposal requires exact observation detector coordinates",
		)
	}
	return validateCanonicalManualProfileScopes(
		dto.Scopes,
		dto.ObservationDigest,
	)
}

func profileChangeBasisFromJSON(
	dto *profileChangeBasisJSON,
) (ProfileChangeBasis, error) {
	if dto == nil {
		return ProfileChangeBasis{}, fmt.Errorf(
			"profile relation change requires change_basis",
		)
	}
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		dto.AdmissionRecordRef,
	)
	if err != nil {
		return ProfileChangeBasis{}, err
	}
	admissionDigest, err := projectprofile.NewContentDigest(
		dto.AdmissionRecordDigest,
	)
	if err != nil {
		return ProfileChangeBasis{}, err
	}
	payloadDigest, err := projectprofile.NewContentDigest(dto.PayloadDigest)
	if err != nil {
		return ProfileChangeBasis{}, err
	}
	scopeID, err := projectprofile.NewScopeID(dto.ScopeID)
	if err != nil {
		return ProfileChangeBasis{}, err
	}
	nextEntityRef, err := projectprofile.NewEntityRef(dto.NextEntityRef)
	if err != nil {
		return ProfileChangeBasis{}, err
	}
	return NewProfileChangeBasis(
		admissionRef,
		admissionDigest,
		payloadDigest,
		projectprofile.NewLedgerRevision(dto.LedgerRevision),
		scopeID,
		dto.PreviousEntityRef,
		nextEntityRef,
	)
}

func profileChangeBasisFromDTO(
	dto *profileChangeBasisJSON,
) *ProfileChangeBasis {
	if dto == nil {
		return nil
	}
	basis, err := profileChangeBasisFromJSON(dto)
	if err != nil {
		return nil
	}
	return &basis
}

func validateProfileRelationChangeCoverage(
	dto profileOnboardingWorkInputJSON,
) error {
	basis, err := profileChangeBasisFromJSON(dto.ChangeBasis)
	if err != nil {
		return err
	}
	found := false
	for _, scope := range dto.Scopes {
		expected := profileChangeComponentCandidateRef(
			basis.PayloadDigest().String(),
			scope.ScopeID,
		)
		if scope.ComponentCandidateRef != expected {
			return fmt.Errorf(
				"profile relation change scope %q has another predecessor binding",
				scope.ScopeID,
			)
		}
		if scope.ScopeID == basis.ScopeID().String() {
			if scope.EntityRef != basis.NextEntityRef().String() {
				return fmt.Errorf(
					"profile relation change successor does not carry the selected entity_ref",
				)
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf(
			"profile relation change successor omits its selected scope",
		)
	}
	return nil
}

func validateCanonicalManualProfileScopes(
	scopes []profileScopeDeclarationJSON,
	observationDigest string,
) error {
	for index, scope := range scopes {
		label, err := validateManualProfileLabel(scope.Label)
		if err != nil {
			return fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		evidence := canonicalStringSet(scope.EvidencePaths)
		if !slices.Equal(evidence, scope.EvidencePaths) {
			return fmt.Errorf(
				"manual profile scope %d evidence_paths are not canonical",
				index,
			)
		}
		for _, path := range evidence {
			if path == "" || path != strings.TrimSpace(path) {
				return fmt.Errorf(
					"manual profile scope %d evidence_paths contain an invalid path",
					index,
				)
			}
		}
		expectedRef := manualComponentCandidateRef(
			observationDigest,
			scope.ScopeID,
			label,
			profiledetector.RealizationKind(
				scope.RealizationKind,
			),
			evidence,
		)
		if scope.ComponentCandidateRef != expectedRef {
			return fmt.Errorf(
				"manual profile scope %d component_candidate_ref differs from its scope declaration",
				index,
			)
		}
	}
	return nil
}

func profileObservationDetectorVersion(
	dto profileOnboardingWorkInputJSON,
) string {
	if dto.ObservationDetectorVersion != "" {
		return dto.ObservationDetectorVersion
	}
	return dto.DetectorVersion
}

func profileObservationPolicyVersion(
	dto profileOnboardingWorkInputJSON,
) string {
	if dto.ObservationPolicyVersion != "" {
		return dto.ObservationPolicyVersion
	}
	return dto.PolicyVersion
}

func profileScopeBindings(
	values []profileScopeDeclarationJSON,
) ([]profileScopeBinding, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("profile onboarding Work input requires at least one scope")
	}
	result := make([]profileScopeBinding, len(values))
	for index, value := range values {
		binding, err := profileScopeBindingFromJSON(value)
		if err != nil {
			return nil, fmt.Errorf("profile scope %d: %w", index, err)
		}
		result[index] = binding
	}
	slices.SortFunc(result, func(left profileScopeBinding, right profileScopeBinding) int {
		return strings.Compare(left.componentCandidateRef, right.componentCandidateRef)
	})
	for index := 1; index < len(result); index++ {
		if result[index-1].componentCandidateRef == result[index].componentCandidateRef {
			return nil, fmt.Errorf(
				"duplicate component_candidate_ref %q",
				result[index].componentCandidateRef,
			)
		}
	}
	return result, nil
}

func profileScopeBindingFromJSON(
	value profileScopeDeclarationJSON,
) (profileScopeBinding, error) {
	componentRef := strings.TrimSpace(value.ComponentCandidateRef)
	if componentRef == "" || componentRef != value.ComponentCandidateRef {
		return profileScopeBinding{}, fmt.Errorf(
			"component_candidate_ref must be a non-empty exact detector reference",
		)
	}
	scopeID, err := projectprofile.NewScopeID(value.ScopeID)
	if err != nil {
		return profileScopeBinding{}, err
	}
	entity, err := profileEntityReference(value.EntityRef)
	if err != nil {
		return profileScopeBinding{}, err
	}
	if value.RealizationKind == string(profiledetector.SoftwareRealization) {
		if value.AdmittedKindRef != "" ||
			len(value.GoverningPatternRefs) != 0 ||
			len(value.ContractRefs) != 0 {
			return profileScopeBinding{}, fmt.Errorf(
				"software scope cannot carry non-software kind, pattern, or contract fields",
			)
		}
		scope, buildErr := projectprofile.NewSoftwareRealization(scopeID, entity)
		return profileScopeBinding{
			componentCandidateRef: componentRef,
			scope:                 scope,
		}, buildErr
	}
	if value.RealizationKind != string(profiledetector.NonSoftwareRealization) {
		return profileScopeBinding{}, fmt.Errorf(
			"realization_kind must be %q or %q",
			profiledetector.SoftwareRealization,
			profiledetector.NonSoftwareRealization,
		)
	}
	kind, err := profileKindOrientation(value.AdmittedKindRef)
	if err != nil {
		return profileScopeBinding{}, err
	}
	patterns, err := parseProfileReferences(
		value.GoverningPatternRefs,
		projectprofile.NewSourceUnitRef,
	)
	if err != nil {
		return profileScopeBinding{}, fmt.Errorf("governing_pattern_refs: %w", err)
	}
	contracts, err := parseProfileReferences(
		value.ContractRefs,
		projectprofile.NewSpecSectionRef,
	)
	if err != nil {
		return profileScopeBinding{}, fmt.Errorf("contract_refs: %w", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		entity,
		kind,
		patterns,
		contracts,
	)
	return profileScopeBinding{
		componentCandidateRef: componentRef,
		scope:                 scope,
	}, err
}

func profileEntityReference(raw string) (projectprofile.EntityReference, error) {
	if raw == "" {
		return projectprofile.NoEntityReference{}, nil
	}
	ref, err := projectprofile.NewEntityRef(raw)
	if err != nil {
		return nil, err
	}
	return projectprofile.NewReferencedEntity(ref), nil
}

func profileKindOrientation(raw string) (projectprofile.KindOrientation, error) {
	if raw == "" {
		return projectprofile.UnspecifiedKindOrientation{}, nil
	}
	ref, err := projectprofile.NewKindRef(raw)
	if err != nil {
		return nil, err
	}
	return projectprofile.NewReferencedKindOrientation(ref), nil
}

func parseProfileReferences[T any](
	values []string,
	parser func(string) (T, error),
) ([]T, error) {
	result := make([]T, len(values))
	for index, raw := range values {
		value, err := parser(raw)
		if err != nil {
			return nil, fmt.Errorf("reference %d: %w", index, err)
		}
		result[index] = value
	}
	return result, nil
}

func validateProfileScopeCoverage(
	bindings []profileScopeBinding,
	suggestion profiledetector.Suggestion,
) error {
	suggested := suggestion.SuggestedScopes()
	if len(bindings) != len(suggested) {
		return fmt.Errorf(
			"profile declaration maps %d scope(s); current detector observation has %d component candidate(s)",
			len(bindings),
			len(suggested),
		)
	}
	want := map[string]profiledetector.RealizationKind{}
	for _, scope := range suggested {
		want[scope.ComponentCandidateRef()] = scope.RealizationKind()
	}
	for _, binding := range bindings {
		kind, exists := want[binding.componentCandidateRef]
		if !exists {
			return fmt.Errorf(
				"component_candidate_ref %q is not present in the current detector observation",
				binding.componentCandidateRef,
			)
		}
		if !profileScopeMatchesRealizationKind(binding.scope, kind) {
			return fmt.Errorf(
				"component_candidate_ref %q changes detector realization_kind %q",
				binding.componentCandidateRef,
				kind,
			)
		}
	}
	return nil
}

func validateManualProfileScopeCoverage(
	dto profileOnboardingWorkInputJSON,
	suggestion profiledetector.Suggestion,
) error {
	if suggestion.ScopeIdentityPosture() !=
		profiledetector.ScopeIdentityNeedsReview {
		return fmt.Errorf(
			"manual profile fallback cannot replace a stable detector scope identity",
		)
	}
	snapshot := suggestion.Snapshot()
	observedPaths := make(
		map[string]struct{},
		len(snapshot.RelativeFiles()),
	)
	for _, path := range snapshot.RelativeFiles() {
		observedPaths[path] = struct{}{}
	}
	for index, scope := range dto.Scopes {
		label, err := validateManualProfileLabel(scope.Label)
		if err != nil {
			return fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		evidence, err := canonicalManualEvidencePaths(
			scope.EvidencePaths,
			observedPaths,
		)
		if err != nil {
			return fmt.Errorf(
				"manual profile scope %d: %w",
				index,
				err,
			)
		}
		if !slices.Equal(evidence, scope.EvidencePaths) {
			return fmt.Errorf(
				"manual profile scope %d evidence_paths are not canonical",
				index,
			)
		}
		expectedRef := manualComponentCandidateRef(
			snapshot.ObservationDigest(),
			scope.ScopeID,
			label,
			profiledetector.RealizationKind(
				scope.RealizationKind,
			),
			evidence,
		)
		if scope.ComponentCandidateRef != expectedRef {
			return fmt.Errorf(
				"manual profile scope %d component_candidate_ref differs from its reviewed scope and current observation",
				index,
			)
		}
	}
	return nil
}

func profileScopeMatchesRealizationKind(
	scope projectprofile.RealizationScope,
	kind profiledetector.RealizationKind,
) bool {
	switch scope.(type) {
	case projectprofile.SoftwareRealization:
		return kind == profiledetector.SoftwareRealization
	case projectprofile.NonSoftwareRealization:
		return kind == profiledetector.NonSoftwareRealization
	default:
		return false
	}
}

func profilePayloadFromBindings(
	bindings []profileScopeBinding,
) (projectprofile.ProfileDeclarationPayload, error) {
	values := make([]projectprofile.RealizationScope, len(bindings))
	for index, binding := range bindings {
		values[index] = binding.scope
	}
	scopes, err := projectprofile.NewScopeSet(values)
	if err != nil {
		return projectprofile.ProfileDeclarationPayload{}, err
	}
	return projectprofile.NewProfileDeclarationPayload(scopes)
}

func canonicalProfileWorkInputDTO(
	dto profileOnboardingWorkInputJSON,
	bindings []profileScopeBinding,
) profileOnboardingWorkInputJSON {
	scopes := make([]profileScopeDeclarationJSON, len(bindings))
	byComponent := map[string]profileScopeDeclarationJSON{}
	for _, scope := range dto.Scopes {
		copyValue := scope
		copyValue.GoverningPatternRefs = canonicalStringSet(scope.GoverningPatternRefs)
		copyValue.ContractRefs = canonicalStringSet(scope.ContractRefs)
		copyValue.EvidencePaths = canonicalStringSet(
			scope.EvidencePaths,
		)
		byComponent[scope.ComponentCandidateRef] = copyValue
	}
	for index, binding := range bindings {
		scopes[index] = byComponent[binding.componentCandidateRef]
	}
	return profileOnboardingWorkInputJSON{
		Schema:                     profileOnboardingWorkInputSchema,
		ProjectRoot:                dto.ProjectRoot,
		SuggestionRef:              dto.SuggestionRef,
		DetectorVersion:            dto.DetectorVersion,
		PolicyVersion:              dto.PolicyVersion,
		ObservationDetectorVersion: dto.ObservationDetectorVersion,
		ObservationPolicyVersion:   dto.ObservationPolicyVersion,
		ObservationDigest:          dto.ObservationDigest,
		ProposalSource:             dto.ProposalSource,
		ManualBasis:                dto.ManualBasis,
		ChangeBasis:                cloneProfileChangeBasisJSON(dto.ChangeBasis),
		Scopes:                     scopes,
	}
}

func cloneProfileChangeBasisJSON(
	value *profileChangeBasisJSON,
) *profileChangeBasisJSON {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func canonicalStringSet(values []string) []string {
	result := append([]string{}, values...)
	slices.Sort(result)
	return slices.Compact(result)
}

func profileWorkInputIdentity(
	canonical []byte,
) (projectprofile.ContentDigest, projectprofile.WorkInputRef, error) {
	hash := sha256.New()
	_, _ = hash.Write([]byte(profileOnboardingWorkInputDomain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(canonical)
	hexDigest := hex.EncodeToString(hash.Sum(nil))
	digest, err := projectprofile.NewContentDigest("sha256:" + hexDigest)
	if err != nil {
		return projectprofile.ContentDigest{}, projectprofile.WorkInputRef{}, err
	}
	ref, err := projectprofile.NewWorkInputRef(profileOnboardingWorkInputPrefix + hexDigest)
	if err != nil {
		return projectprofile.ContentDigest{}, projectprofile.WorkInputRef{}, err
	}
	return digest, ref, nil
}

func (input ProfileOnboardingWorkInput) Valid() bool {
	return input.state != nil && input.state.ref.String() != "" &&
		input.state.digest.String() != "" && len(input.state.canonicalJSON) > 0
}

func (input ProfileOnboardingWorkInput) ProjectRoot() projectprofile.ProjectRootV1 {
	return input.state.projectRoot
}

func (input ProfileOnboardingWorkInput) Ref() projectprofile.WorkInputRef {
	return input.state.ref
}

func (input ProfileOnboardingWorkInput) Digest() projectprofile.ContentDigest {
	return input.state.digest
}

func (input ProfileOnboardingWorkInput) CanonicalJSON() []byte {
	return append([]byte(nil), input.state.canonicalJSON...)
}

func (input ProfileOnboardingWorkInput) Payload() projectprofile.ProfileDeclarationPayload {
	return input.state.payload
}

func (input ProfileOnboardingWorkInput) PayloadDigest() projectprofile.ContentDigest {
	digest, _ := projectprofile.DigestProfileDeclarationPayload(input.state.payload)
	return digest
}

func (input ProfileOnboardingWorkInput) PayloadCanonicalJSON() []byte {
	canonical, _ := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(
		input.state.payload,
	)
	return canonical
}

func (input ProfileOnboardingWorkInput) SuggestionRef() string {
	return input.state.suggestionRef
}

func (input ProfileOnboardingWorkInput) DetectorVersion() string {
	return input.state.detectorVersion
}

func (input ProfileOnboardingWorkInput) ClassifierVersion() string {
	return input.state.detectorVersion
}

func (input ProfileOnboardingWorkInput) PolicyVersion() string {
	return input.state.policyVersion
}

func (input ProfileOnboardingWorkInput) ClassificationPolicyVersion() string {
	return input.state.policyVersion
}

func (input ProfileOnboardingWorkInput) ObservationDetectorVersion() string {
	return input.state.observationDetectorVersion
}

func (input ProfileOnboardingWorkInput) ObservationPolicyVersion() string {
	return input.state.observationPolicyVersion
}

func (input ProfileOnboardingWorkInput) ObservationDigest() string {
	return input.state.observationDigest
}

func (input ProfileOnboardingWorkInput) UsesManualScopeBasis() bool {
	return input.state.proposalSource == profileProposalSourceManual
}

func (input ProfileOnboardingWorkInput) ManualBasis() string {
	return input.state.manualBasis
}

func (input ProfileOnboardingWorkInput) ProfileChangeBasis() (
	ProfileChangeBasis,
	bool,
) {
	if input.state == nil || input.state.changeBasis == nil {
		return ProfileChangeBasis{}, false
	}
	return *input.state.changeBasis, true
}

// ValidateProfileEntityRelationChange proves that the reviewed successor is
// exactly the predecessor payload with one scope entity_ref replaced.
func (input ProfileOnboardingWorkInput) ValidateProfileEntityRelationChange(
	current projectprofile.ProfileDeclarationPayload,
) error {
	basis, ok := input.ProfileChangeBasis()
	if !ok {
		return fmt.Errorf(
			"reviewed WorkInput is not a profile entity relation change",
		)
	}
	currentDigest, err := projectprofile.DigestProfileDeclarationPayload(current)
	if err != nil {
		return err
	}
	if currentDigest != basis.PayloadDigest() {
		return fmt.Errorf(
			"profile relation change predecessor payload is stale",
		)
	}
	scopes, previous, err := profileRelationChangeScopes(current, basis)
	if err != nil {
		return err
	}
	if previous != basis.PreviousEntityRef() {
		return fmt.Errorf(
			"profile relation change previous entity_ref is stale",
		)
	}
	bindings, err := profileScopeBindings(scopes)
	if err != nil {
		return err
	}
	expected, err := profilePayloadFromBindings(bindings)
	if err != nil {
		return err
	}
	expectedDigest, err := projectprofile.DigestProfileDeclarationPayload(expected)
	if err != nil {
		return err
	}
	if expectedDigest != input.PayloadDigest() {
		return fmt.Errorf(
			"profile relation change contains a semantic delta outside the selected entity_ref",
		)
	}
	return nil
}
