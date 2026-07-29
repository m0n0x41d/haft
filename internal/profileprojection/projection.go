package profileprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"gopkg.in/yaml.v3"
)

const (
	ProjectionSchemaV1 = "haft.project-profile-projection/v1"
	projectionHeader   = "# Generated from the canonical SQLite profile-admission ledger.\n" +
		"# Human-readable projection only: not authority and not admission proof.\n"
)

type projectionDocumentV1 struct {
	Schema               string               `yaml:"schema"`
	SemanticRole         string               `yaml:"semantic_role"`
	CanonicalSource      string               `yaml:"canonical_source"`
	ProjectRoot          string               `yaml:"project_root"`
	ProfileKind          string               `yaml:"profile_kind"`
	LedgerRevision       uint64               `yaml:"ledger_revision"`
	LedgerRecordedAt     string               `yaml:"ledger_recorded_at"`
	PayloadDigest        string               `yaml:"profile_payload_digest"`
	AdmissionRecordRef   string               `yaml:"admission_record_ref"`
	AdmissionDigest      string               `yaml:"admission_record_digest"`
	ReceiptDigest        string               `yaml:"receipt_digest"`
	ReceiptCanonicalJSON string               `yaml:"receipt_canonical_json"`
	Provenance           projectionProvenance `yaml:"provenance"`
	Scopes               []projectionScope    `yaml:"scopes"`
}

type projectionProvenance struct {
	CandidateProvenanceDigest     string `yaml:"candidate_provenance_digest"`
	WorkRecordRef                 string `yaml:"work_record_ref"`
	WorkRecordDigest              string `yaml:"work_record_digest"`
	AuthorityBasisRef             string `yaml:"authority_basis_ref"`
	AuthorityBasisDigest          string `yaml:"authority_basis_digest"`
	AuthorityResolutionRef        string `yaml:"authority_resolution_ref"`
	AuthorityResolutionDigest     string `yaml:"authority_resolution_digest"`
	ProfileAuthorAssignmentRef    string `yaml:"profile_author_assignment_ref"`
	ProfileAuthorAssignmentDigest string `yaml:"profile_author_assignment_digest"`
	ObservedProjectBasisRef       string `yaml:"observed_project_basis_ref"`
	ObservedProjectBasisDigest    string `yaml:"observed_project_basis_digest"`
	OutcomeAssessmentRef          string `yaml:"outcome_assessment_ref"`
	OutcomeAssessmentDigest       string `yaml:"outcome_assessment_digest"`
}

type projectionScope struct {
	Kind                 string                     `yaml:"kind"`
	ScopeID              string                     `yaml:"scope_id"`
	EntityReference      projectionEntityReference  `yaml:"entity_reference"`
	KindOrientation      *projectionKindOrientation `yaml:"kind_admission,omitempty"`
	GoverningPatternRefs []string                   `yaml:"governing_pattern_refs,omitempty"`
	ContractRefs         []string                   `yaml:"contract_refs,omitempty"`
}

type projectionEntityReference struct {
	Kind string `yaml:"kind"`
	Ref  string `yaml:"ref,omitempty"`
}

// projectionKindOrientation retains the v1 kind_admission projection spelling.
type projectionKindOrientation struct {
	Kind string `yaml:"kind"`
	Ref  string `yaml:"ref,omitempty"`
}

type projectionMaterial struct {
	projectRoot                       projectprofile.ProjectRootV1
	payload                           projectprofile.ProfileDeclarationPayload
	payloadDigest                     projectprofile.ContentDigest
	admissionRecordRef                projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionRecordDigest             projectprofile.ContentDigest
	receiptCanonicalJSON              []byte
	receiptDigest                     projectprofile.ContentDigest
	candidateProvenanceDigest         projectprofile.ContentDigest
	workRecordRef                     projectprofile.ProfileOnboardingWorkRecordRef
	workRecordDigest                  projectprofile.ContentDigest
	authorityBasisRef                 projectprofile.ProfileDeclarationAuthorityBasisRef
	authorityBasisDigest              projectprofile.ContentDigest
	authorityResolutionRef            projectprofile.AuthorityResolutionRecordRef
	authorityResolutionDigest         projectprofile.ContentDigest
	profileAuthorRoleAssignmentRef    projectprofile.RoleAssignmentRef
	profileAuthorRoleAssignmentDigest projectprofile.ContentDigest
	observedProjectBasisRef           projectprofile.ObservedProjectBasisRefV1
	observedProjectBasisDigest        projectprofile.ContentDigest
	outcomeAssessmentRef              projectprofile.ProfileOnboardingOutcomeAssessmentRefV1
	outcomeAssessmentDigest           projectprofile.ContentDigest
	ledgerRevision                    projectprofile.LedgerRevision
	recordedAt                        time.Time
}

type projection struct {
	content []byte
	digest  projectprofile.ContentDigest
}

func projectionFromAdmission(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (projectionMaterial, error) {
	if !admission.Valid() {
		return projectionMaterial{}, fmt.Errorf("canonical profile admission is required")
	}
	material := projectionMaterial{
		projectRoot:                       admission.ProjectRoot(),
		payload:                           admission.Payload(),
		payloadDigest:                     admission.PayloadDigest(),
		admissionRecordRef:                admission.AdmissionRecordRef(),
		admissionRecordDigest:             admission.AdmissionRecordDigest(),
		receiptCanonicalJSON:              admission.ReceiptCanonicalJSON(),
		receiptDigest:                     admission.ReceiptDigest(),
		candidateProvenanceDigest:         admission.CandidateProvenanceDigest(),
		workRecordRef:                     admission.WorkRecordRef(),
		workRecordDigest:                  admission.WorkRecordDigest(),
		authorityBasisRef:                 admission.AuthorityBasisRef(),
		authorityBasisDigest:              admission.AuthorityBasisDigest(),
		authorityResolutionRef:            admission.AuthorityResolutionRef(),
		authorityResolutionDigest:         admission.AuthorityResolutionDigest(),
		profileAuthorRoleAssignmentRef:    admission.ProfileAuthorRoleAssignmentRef(),
		profileAuthorRoleAssignmentDigest: admission.ProfileAuthorRoleAssignmentDigest(),
		observedProjectBasisRef:           admission.ObservedProjectBasisRef(),
		observedProjectBasisDigest:        admission.ObservedProjectBasisDigest(),
		outcomeAssessmentRef:              admission.OutcomeAssessmentRef(),
		outcomeAssessmentDigest:           admission.OutcomeAssessmentDigest(),
		ledgerRevision:                    admission.LedgerRevision(),
		recordedAt:                        admission.RecordedAt(),
	}
	return material, nil
}

func buildProjection(material projectionMaterial) (projection, error) {
	document, err := buildProjectionDocument(material)
	if err != nil {
		return projection{}, err
	}
	content, err := encodeProjectionDocument(document)
	if err != nil {
		return projection{}, err
	}
	digest, err := digestProjection(content)
	if err != nil {
		return projection{}, err
	}
	return projection{content: content, digest: digest}, nil
}

// VerifyExactProjectionBytes reconstructs the human-readable projection from
// one resolver-minted canonical admission and compares every byte with the
// observed carrier. It is a pure read boundary: it performs no filesystem or
// ledger mutation and returns the canonical projection digest only after an
// exact match.
func VerifyExactProjectionBytes(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	observed []byte,
) (projectprofile.ContentDigest, error) {
	material, err := projectionFromAdmission(admission)
	if err != nil {
		return projectprofile.ContentDigest{}, err
	}
	expected, err := buildProjection(material)
	if err != nil {
		return projectprofile.ContentDigest{}, err
	}
	if err := verifyExactProjectionBytes(expected, observed); err != nil {
		return projectprofile.ContentDigest{}, err
	}
	return expected.digest, nil
}

func verifyExactProjectionBytes(expected projection, observed []byte) error {
	if !bytes.Equal(expected.content, observed) {
		return fmt.Errorf("profile projection differs from exact canonical bytes")
	}
	observedDigest, err := digestProjection(observed)
	if err != nil {
		return err
	}
	if observedDigest != expected.digest {
		return fmt.Errorf("profile projection digest differs from exact canonical digest")
	}
	return nil
}

func buildProjectionDocument(material projectionMaterial) (projectionDocumentV1, error) {
	if err := validateProjectionMaterial(material); err != nil {
		return projectionDocumentV1{}, err
	}
	scopes, err := projectScopes(material.payload.Scopes().Values())
	if err != nil {
		return projectionDocumentV1{}, err
	}
	document := projectionDocumentV1{
		Schema:               ProjectionSchemaV1,
		SemanticRole:         "human_readable_projection",
		CanonicalSource:      "sqlite_profile_admission_ledger",
		ProjectRoot:          material.projectRoot.String(),
		ProfileKind:          "Declared",
		LedgerRevision:       material.ledgerRevision.Value(),
		LedgerRecordedAt:     material.recordedAt.UTC().Format(time.RFC3339Nano),
		PayloadDigest:        material.payloadDigest.String(),
		AdmissionRecordRef:   material.admissionRecordRef.String(),
		AdmissionDigest:      material.admissionRecordDigest.String(),
		ReceiptDigest:        material.receiptDigest.String(),
		ReceiptCanonicalJSON: string(material.receiptCanonicalJSON),
		Provenance: projectionProvenance{
			CandidateProvenanceDigest:     material.candidateProvenanceDigest.String(),
			WorkRecordRef:                 material.workRecordRef.String(),
			WorkRecordDigest:              material.workRecordDigest.String(),
			AuthorityBasisRef:             material.authorityBasisRef.String(),
			AuthorityBasisDigest:          material.authorityBasisDigest.String(),
			AuthorityResolutionRef:        material.authorityResolutionRef.String(),
			AuthorityResolutionDigest:     material.authorityResolutionDigest.String(),
			ProfileAuthorAssignmentRef:    material.profileAuthorRoleAssignmentRef.String(),
			ProfileAuthorAssignmentDigest: material.profileAuthorRoleAssignmentDigest.String(),
			ObservedProjectBasisRef:       material.observedProjectBasisRef.String(),
			ObservedProjectBasisDigest:    material.observedProjectBasisDigest.String(),
			OutcomeAssessmentRef:          material.outcomeAssessmentRef.String(),
			OutcomeAssessmentDigest:       material.outcomeAssessmentDigest.String(),
		},
		Scopes: scopes,
	}
	return document, nil
}

func validateProjectionMaterial(material projectionMaterial) error {
	root, err := projectprofile.NewProjectRootV1(material.projectRoot.String())
	if err != nil || root != material.projectRoot {
		return fmt.Errorf("projection project root is invalid")
	}
	if err := projectprofile.ValidateProfileDeclarationPayloadStructuralConsistencyV1(material.payload); err != nil {
		return err
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(material.payload)
	if err != nil {
		return err
	}
	if payloadDigest != material.payloadDigest {
		return fmt.Errorf("projection payload digest does not match canonical payload")
	}
	if material.ledgerRevision.Value() == 0 {
		return fmt.Errorf("projection ledger revision is invalid")
	}
	if material.recordedAt.IsZero() {
		return fmt.Errorf("projection ledger recorded time is absent")
	}
	if !bytes.Equal(bytes.TrimSpace(material.receiptCanonicalJSON), material.receiptCanonicalJSON) {
		return fmt.Errorf("projection receipt JSON is not canonical text")
	}
	if len(material.receiptCanonicalJSON) == 0 {
		return fmt.Errorf("projection receipt JSON is absent")
	}
	return validateProjectionReferences(material)
}

func validateProjectionReferences(material projectionMaterial) error {
	values := []string{
		material.admissionRecordRef.String(),
		material.admissionRecordDigest.String(),
		material.receiptDigest.String(),
		material.candidateProvenanceDigest.String(),
		material.workRecordRef.String(),
		material.workRecordDigest.String(),
		material.authorityBasisRef.String(),
		material.authorityBasisDigest.String(),
		material.authorityResolutionRef.String(),
		material.authorityResolutionDigest.String(),
		material.profileAuthorRoleAssignmentRef.String(),
		material.profileAuthorRoleAssignmentDigest.String(),
		material.observedProjectBasisRef.String(),
		material.observedProjectBasisDigest.String(),
		material.outcomeAssessmentRef.String(),
		material.outcomeAssessmentDigest.String(),
	}
	for index, value := range values {
		if value == "" {
			return fmt.Errorf("projection reference or digest %d is absent", index)
		}
	}
	return nil
}

func projectScopes(values []projectprofile.RealizationScope) ([]projectionScope, error) {
	result := make([]projectionScope, 0, len(values))
	for _, value := range values {
		projected, err := projectScope(value)
		if err != nil {
			return nil, err
		}
		result = append(result, projected)
	}
	slices.SortFunc(result, func(left projectionScope, right projectionScope) int {
		if left.ScopeID < right.ScopeID {
			return -1
		}
		if left.ScopeID > right.ScopeID {
			return 1
		}
		return 0
	})
	return result, nil
}

func projectScope(value projectprofile.RealizationScope) (projectionScope, error) {
	switch scope := value.(type) {
	case projectprofile.SoftwareRealization:
		entity, err := projectEntityReference(scope.EntityReference())
		if err != nil {
			return projectionScope{}, err
		}
		return projectionScope{
			Kind:            "software",
			ScopeID:         scope.ScopeID().String(),
			EntityReference: entity,
		}, nil
	case projectprofile.NonSoftwareRealization:
		entity, err := projectEntityReference(scope.EntityReference())
		if err != nil {
			return projectionScope{}, err
		}
		kind, err := projectKindOrientation(scope.KindOrientation())
		if err != nil {
			return projectionScope{}, err
		}
		return projectionScope{
			Kind:                 "non_software",
			ScopeID:              scope.ScopeID().String(),
			EntityReference:      entity,
			KindOrientation:      &kind,
			GoverningPatternRefs: sourceUnitRefStrings(scope.GoverningPatternRefs()),
			ContractRefs:         specSectionRefStrings(scope.ContractRefs()),
		}, nil
	default:
		return projectionScope{}, fmt.Errorf("projection cannot encode unknown realization scope")
	}
}

func projectEntityReference(
	value projectprofile.EntityReference,
) (projectionEntityReference, error) {
	switch reference := value.(type) {
	case projectprofile.NoEntityReference:
		return projectionEntityReference{Kind: "none"}, nil
	case projectprofile.ReferencedEntity:
		return projectionEntityReference{Kind: "referenced", Ref: reference.Ref().String()}, nil
	default:
		return projectionEntityReference{}, fmt.Errorf("projection cannot encode unknown entity reference")
	}
}

func projectKindOrientation(
	value projectprofile.KindOrientation,
) (projectionKindOrientation, error) {
	switch orientation := value.(type) {
	case projectprofile.UnspecifiedKindOrientation:
		return projectionKindOrientation{Kind: "none"}, nil
	case projectprofile.ReferencedKindOrientation:
		return projectionKindOrientation{Kind: "admitted", Ref: orientation.Ref().String()}, nil
	default:
		return projectionKindOrientation{}, fmt.Errorf("projection cannot encode unknown kind orientation")
	}
}

func sourceUnitRefStrings(values []projectprofile.SourceUnitRef) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func specSectionRefStrings(values []projectprofile.SpecSectionRef) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.String())
	}
	return result
}

func encodeProjectionDocument(document projectionDocumentV1) ([]byte, error) {
	var body bytes.Buffer
	encoder := yaml.NewEncoder(&body)
	encoder.SetIndent(2)
	err := encoder.Encode(document)
	closeErr := encoder.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	content := append([]byte(projectionHeader), body.Bytes()...)
	return content, nil
}

func digestProjection(content []byte) (projectprofile.ContentDigest, error) {
	sum := sha256.Sum256(content)
	encoded := hex.EncodeToString(sum[:])
	return projectprofile.NewContentDigest("sha256:" + encoded)
}
