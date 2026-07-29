// Package projecttypeenvprofilebasis defines the immutable project-profile
// observation consumed by project TypeEnv profile-fit assessment.
package projecttypeenvprofilebasis

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	basisSchemaV1       = "haft.project-typeenv.current-project-profile-basis/v1"
	basisDigestDomain   = "haft.project-typeenv.current-project-profile-basis.v1"
	ledgerDigestDomain  = "haft.project-typeenv.project-profile-ledger-head.v1"
	supportDigestDomain = "haft.project-typeenv.project-profile-support-dag.v1"
)

// CurrentProjectProfileBasis is a closed observation sum. Absence is an exact
// zero-ledger fact, not a nil or a failed profile-store read.
type CurrentProjectProfileBasis interface {
	ProjectRoot() projectprofile.ProjectRootV1
	LedgerRevision() projectprofile.LedgerRevision
	ProfileBasisRef() ProjectProfileBasisRef
	Digest() typedmemory.SHA256Digest
	ProfileLedgerDigest() typedmemory.SHA256Digest
	CanonicalBytes() []byte
	Verify() error
	currentProjectProfileBasisVariant()
}

type basisState struct {
	root         projectprofile.ProjectRootV1
	revision     projectprofile.LedgerRevision
	ref          ProjectProfileBasisRef
	digest       typedmemory.SHA256Digest
	ledgerDigest typedmemory.SHA256Digest
	canonical    []byte
}

// ProjectProfileBasisRef is the content address of one exact basis. Stage
// integration may parse its String form into its own coordinate type without
// making this core depend on Stage construction.
type ProjectProfileBasisRef struct{ digest typedmemory.SHA256Digest }

func ParseProjectProfileBasisRef(raw string) (ProjectProfileBasisRef, error) {
	const prefix = "project-profile-basis:"
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, prefix) {
		return ProjectProfileBasisRef{}, fmt.Errorf(
			"project profile basis ref must start with %q",
			prefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return ProjectProfileBasisRef{}, err
	}
	return ProjectProfileBasisRef{digest: digest}, nil
}

func (ref ProjectProfileBasisRef) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref ProjectProfileBasisRef) String() string {
	return "project-profile-basis:" + ref.digest.String()
}

// NoCanonicalProjectProfile proves that the canonical profile ledger is
// exactly empty for one project root.
type NoCanonicalProjectProfile struct{ state basisState }

func (NoCanonicalProjectProfile) currentProjectProfileBasisVariant() {}

func NewNoCanonicalProjectProfile(
	root projectprofile.ProjectRootV1,
) (NoCanonicalProjectProfile, error) {
	parsedRoot, err := projectprofile.NewProjectRootV1(root.String())
	if err != nil || parsedRoot != root {
		return NoCanonicalProjectProfile{}, fmt.Errorf("canonical project root is required")
	}
	dto := basisJSON{
		Schema:         basisSchemaV1,
		Variant:        "no_canonical_project_profile",
		ProjectRoot:    root.String(),
		LedgerRevision: 0,
	}
	state, err := mintBasisState(root, projectprofile.NewLedgerRevision(0), dto)
	if err != nil {
		return NoCanonicalProjectProfile{}, err
	}
	result := NoCanonicalProjectProfile{state: state}
	if err := result.Verify(); err != nil {
		return NoCanonicalProjectProfile{}, err
	}
	return result, nil
}

func (basis NoCanonicalProjectProfile) ProjectRoot() projectprofile.ProjectRootV1 {
	return basis.state.root
}

func (basis NoCanonicalProjectProfile) LedgerRevision() projectprofile.LedgerRevision {
	return basis.state.revision
}

func (basis NoCanonicalProjectProfile) ProfileBasisRef() ProjectProfileBasisRef {
	return basis.state.ref
}

func (basis NoCanonicalProjectProfile) Digest() typedmemory.SHA256Digest {
	return basis.state.digest
}

func (basis NoCanonicalProjectProfile) ProfileLedgerDigest() typedmemory.SHA256Digest {
	return basis.state.ledgerDigest
}

func (basis NoCanonicalProjectProfile) CanonicalBytes() []byte {
	return append([]byte(nil), basis.state.canonical...)
}

func (basis NoCanonicalProjectProfile) Verify() error {
	expected, err := NewNoCanonicalProjectProfileUnchecked(basis.state.root)
	if err != nil {
		return err
	}
	return verifyBasisState(basis.state, expected.state)
}

// NewNoCanonicalProjectProfileUnchecked is kept private to break recursive
// verification while preserving one canonical constructor pipeline.
func NewNoCanonicalProjectProfileUnchecked(
	root projectprofile.ProjectRootV1,
) (NoCanonicalProjectProfile, error) {
	parsedRoot, err := projectprofile.NewProjectRootV1(root.String())
	if err != nil || parsedRoot != root {
		return NoCanonicalProjectProfile{}, fmt.Errorf("canonical project root is required")
	}
	dto := basisJSON{
		Schema:         basisSchemaV1,
		Variant:        "no_canonical_project_profile",
		ProjectRoot:    root.String(),
		LedgerRevision: 0,
	}
	state, err := mintBasisState(root, projectprofile.NewLedgerRevision(0), dto)
	return NoCanonicalProjectProfile{state: state}, err
}

// DeclaredProjectProfileBasisInput is exact material from a strictly verified
// CanonicalProfileAdmission. This pure package binds it; it does not perform
// storage or authority verification itself.
type DeclaredProjectProfileBasisInput struct {
	ProjectRoot                       projectprofile.ProjectRootV1
	LedgerRevision                    projectprofile.LedgerRevision
	Payload                           projectprofile.ProfileDeclarationPayload
	AdmissionRecordRef                projectprofile.ProfileDeclarationAdmissionRecordRef
	AdmissionRecordDigest             projectprofile.ContentDigest
	AdmissionRecordCanonicalJSON      []byte
	ReceiptDigest                     projectprofile.ContentDigest
	ReceiptCanonicalJSON              []byte
	CandidateProvenanceDigest         projectprofile.ContentDigest
	WorkRecordRef                     projectprofile.ProfileOnboardingWorkRecordRef
	WorkRecordDigest                  projectprofile.ContentDigest
	AuthorityBasisRef                 projectprofile.ProfileDeclarationAuthorityBasisRef
	AuthorityBasisDigest              projectprofile.ContentDigest
	AuthorityResolutionRef            projectprofile.AuthorityResolutionRecordRef
	AuthorityResolutionDigest         projectprofile.ContentDigest
	ProfileAuthorRoleAssignmentRef    projectprofile.RoleAssignmentRef
	ProfileAuthorRoleAssignmentDigest projectprofile.ContentDigest
	ObservedProjectBasisRef           projectprofile.ObservedProjectBasisRefV1
	ObservedProjectBasisDigest        projectprofile.ContentDigest
	OutcomeAssessmentRef              projectprofile.ProfileOnboardingOutcomeAssessmentRefV1
	OutcomeAssessmentDigest           projectprofile.ContentDigest
}

// DeclaredCanonicalProjectProfile is the exact declared/admitted profile at a
// positive canonical ledger revision.
type DeclaredCanonicalProjectProfile struct {
	state              basisState
	payload            projectprofile.ProfileDeclarationPayload
	payloadCanonical   []byte
	payloadDigest      projectprofile.ContentDigest
	admissionRef       projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionDigest    projectprofile.ContentDigest
	admissionCanonical []byte
	receiptDigest      projectprofile.ContentDigest
	receiptCanonical   []byte
	supportDigest      typedmemory.SHA256Digest
}

func (DeclaredCanonicalProjectProfile) currentProjectProfileBasisVariant() {}

func NewDeclaredCanonicalProjectProfile(
	input DeclaredProjectProfileBasisInput,
) (DeclaredCanonicalProjectProfile, error) {
	canonical, err := canonicalDeclaredInput(input)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	return mintDeclared(canonical)
}

func (basis DeclaredCanonicalProjectProfile) ProjectRoot() projectprofile.ProjectRootV1 {
	return basis.state.root
}

func (basis DeclaredCanonicalProjectProfile) LedgerRevision() projectprofile.LedgerRevision {
	return basis.state.revision
}

func (basis DeclaredCanonicalProjectProfile) ProfileBasisRef() ProjectProfileBasisRef {
	return basis.state.ref
}

func (basis DeclaredCanonicalProjectProfile) Digest() typedmemory.SHA256Digest {
	return basis.state.digest
}

func (basis DeclaredCanonicalProjectProfile) ProfileLedgerDigest() typedmemory.SHA256Digest {
	return basis.state.ledgerDigest
}

func (basis DeclaredCanonicalProjectProfile) CanonicalBytes() []byte {
	return append([]byte(nil), basis.state.canonical...)
}

func (basis DeclaredCanonicalProjectProfile) Payload() projectprofile.ProfileDeclarationPayload {
	return basis.payload
}

func (basis DeclaredCanonicalProjectProfile) PayloadDigest() projectprofile.ContentDigest {
	return basis.payloadDigest
}

func (basis DeclaredCanonicalProjectProfile) PayloadCanonicalJSON() []byte {
	return append([]byte(nil), basis.payloadCanonical...)
}

func (basis DeclaredCanonicalProjectProfile) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	return basis.admissionRef
}

func (basis DeclaredCanonicalProjectProfile) AdmissionRecordDigest() projectprofile.ContentDigest {
	return basis.admissionDigest
}

func (basis DeclaredCanonicalProjectProfile) AdmissionRecordCanonicalJSON() []byte {
	return append([]byte(nil), basis.admissionCanonical...)
}

func (basis DeclaredCanonicalProjectProfile) ReceiptDigest() projectprofile.ContentDigest {
	return basis.receiptDigest
}

func (basis DeclaredCanonicalProjectProfile) ReceiptCanonicalJSON() []byte {
	return append([]byte(nil), basis.receiptCanonical...)
}

func (basis DeclaredCanonicalProjectProfile) SupportDAGDigest() typedmemory.SHA256Digest {
	return basis.supportDigest
}

func (basis DeclaredCanonicalProjectProfile) Verify() error {
	dto := basisJSON{}
	if err := json.Unmarshal(basis.state.canonical, &dto); err != nil {
		return fmt.Errorf("decode canonical declared project-profile basis: %w", err)
	}
	input, err := declaredInputFromJSON(dto)
	if err != nil {
		return err
	}
	expected, err := mintDeclared(input)
	if err != nil {
		return err
	}
	if err := verifyBasisState(basis.state, expected.state); err != nil {
		return err
	}
	if basis.payloadDigest != expected.payloadDigest ||
		basis.admissionRef != expected.admissionRef ||
		basis.admissionDigest != expected.admissionDigest ||
		basis.receiptDigest != expected.receiptDigest ||
		basis.supportDigest != expected.supportDigest ||
		!bytes.Equal(basis.payloadCanonical, expected.payloadCanonical) ||
		!bytes.Equal(basis.admissionCanonical, expected.admissionCanonical) ||
		!bytes.Equal(basis.receiptCanonical, expected.receiptCanonical) {
		return fmt.Errorf("declared project-profile basis fields differ from canonical bytes")
	}
	return nil
}

type supportMemberJSON struct {
	Role   string `json:"role"`
	Ref    string `json:"ref,omitempty"`
	Digest string `json:"digest"`
}

type basisJSON struct {
	Schema                string              `json:"schema"`
	Variant               string              `json:"variant"`
	ProjectRoot           string              `json:"project_root"`
	LedgerRevision        uint64              `json:"ledger_revision"`
	Payload               json.RawMessage     `json:"payload,omitempty"`
	PayloadDigest         string              `json:"payload_digest,omitempty"`
	AdmissionRecordRef    string              `json:"admission_record_ref,omitempty"`
	AdmissionRecordDigest string              `json:"admission_record_digest,omitempty"`
	AdmissionRecord       json.RawMessage     `json:"admission_record,omitempty"`
	ReceiptDigest         string              `json:"receipt_digest,omitempty"`
	Receipt               json.RawMessage     `json:"receipt,omitempty"`
	SupportDAGDigest      string              `json:"support_dag_digest,omitempty"`
	SupportMembers        []supportMemberJSON `json:"support_members,omitempty"`
}

func canonicalDeclaredInput(
	input DeclaredProjectProfileBasisInput,
) (DeclaredProjectProfileBasisInput, error) {
	root, err := projectprofile.NewProjectRootV1(input.ProjectRoot.String())
	if err != nil || root != input.ProjectRoot {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("canonical project root is required")
	}
	if input.LedgerRevision.Value() == 0 {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("declared project profile requires a positive ledger revision")
	}
	if _, err := input.LedgerRevision.Next(); err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(input.Payload.Scopes())
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	if len(input.AdmissionRecordCanonicalJSON) == 0 || !json.Valid(input.AdmissionRecordCanonicalJSON) {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("canonical admission-record JSON is required")
	}
	if !bytes.Equal(compactJSON(input.AdmissionRecordCanonicalJSON), input.AdmissionRecordCanonicalJSON) {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("admission-record JSON is not canonical compact JSON")
	}
	if len(input.ReceiptCanonicalJSON) == 0 || !json.Valid(input.ReceiptCanonicalJSON) {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("canonical receipt JSON is required")
	}
	if !bytes.Equal(compactJSON(input.ReceiptCanonicalJSON), input.ReceiptCanonicalJSON) {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("receipt JSON is not canonical compact JSON")
	}
	if err := validateDeclaredCoordinates(input); err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	result := input
	result.ProjectRoot = root
	result.Payload = payload
	result.AdmissionRecordCanonicalJSON = append([]byte(nil), input.AdmissionRecordCanonicalJSON...)
	result.ReceiptCanonicalJSON = append([]byte(nil), input.ReceiptCanonicalJSON...)
	return result, nil
}

func mintDeclared(
	input DeclaredProjectProfileBasisInput,
) (DeclaredCanonicalProjectProfile, error) {
	payloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(input.Payload)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(input.Payload)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	members := supportMembers(input)
	supportCanonical, err := json.Marshal(members)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	supportDigest, err := digestBytes(supportDigestDomain, supportCanonical)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	dto := basisJSON{
		Schema:                basisSchemaV1,
		Variant:               "declared_canonical_project_profile",
		ProjectRoot:           input.ProjectRoot.String(),
		LedgerRevision:        input.LedgerRevision.Value(),
		Payload:               append(json.RawMessage(nil), payloadJSON...),
		PayloadDigest:         payloadDigest.String(),
		AdmissionRecordRef:    input.AdmissionRecordRef.String(),
		AdmissionRecordDigest: input.AdmissionRecordDigest.String(),
		AdmissionRecord:       append(json.RawMessage(nil), input.AdmissionRecordCanonicalJSON...),
		ReceiptDigest:         input.ReceiptDigest.String(),
		Receipt:               append(json.RawMessage(nil), input.ReceiptCanonicalJSON...),
		SupportDAGDigest:      supportDigest.String(),
		SupportMembers:        members,
	}
	state, err := mintBasisState(input.ProjectRoot, input.LedgerRevision, dto)
	if err != nil {
		return DeclaredCanonicalProjectProfile{}, err
	}
	return DeclaredCanonicalProjectProfile{
		state:              state,
		payload:            input.Payload,
		payloadCanonical:   append([]byte(nil), payloadJSON...),
		payloadDigest:      payloadDigest,
		admissionRef:       input.AdmissionRecordRef,
		admissionDigest:    input.AdmissionRecordDigest,
		admissionCanonical: append([]byte(nil), input.AdmissionRecordCanonicalJSON...),
		receiptDigest:      input.ReceiptDigest,
		receiptCanonical:   append([]byte(nil), input.ReceiptCanonicalJSON...),
		supportDigest:      supportDigest,
	}, nil
}

func mintBasisState(
	root projectprofile.ProjectRootV1,
	revision projectprofile.LedgerRevision,
	dto basisJSON,
) (basisState, error) {
	canonical, err := json.Marshal(dto)
	if err != nil {
		return basisState{}, fmt.Errorf("encode current project-profile basis: %w", err)
	}
	digest, err := digestBytes(basisDigestDomain, canonical)
	if err != nil {
		return basisState{}, err
	}
	ref, err := ParseProjectProfileBasisRef(
		"project-profile-basis:" + digest.String(),
	)
	if err != nil {
		return basisState{}, err
	}
	ledgerCanonical, err := canonicalLedgerHead(dto)
	if err != nil {
		return basisState{}, err
	}
	ledgerDigest, err := digestBytes(ledgerDigestDomain, ledgerCanonical)
	if err != nil {
		return basisState{}, err
	}
	return basisState{
		root:         root,
		revision:     revision,
		ref:          ref,
		digest:       digest,
		ledgerDigest: ledgerDigest,
		canonical:    canonical,
	}, nil
}

func canonicalLedgerHead(dto basisJSON) ([]byte, error) {
	head := struct {
		Schema          string `json:"schema"`
		ProjectRoot     string `json:"project_root"`
		LedgerRevision  uint64 `json:"ledger_revision"`
		Variant         string `json:"variant"`
		AdmissionRef    string `json:"admission_record_ref,omitempty"`
		AdmissionDigest string `json:"admission_record_digest,omitempty"`
	}{
		Schema:          "haft.project-typeenv.project-profile-ledger-head/v1",
		ProjectRoot:     dto.ProjectRoot,
		LedgerRevision:  dto.LedgerRevision,
		Variant:         dto.Variant,
		AdmissionRef:    dto.AdmissionRecordRef,
		AdmissionDigest: dto.AdmissionRecordDigest,
	}
	return json.Marshal(head)
}

func digestBytes(domain string, canonical []byte) (typedmemory.SHA256Digest, error) {
	hasher := sha256.New()
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(domain)))
	_, _ = hasher.Write(size[:])
	_, _ = hasher.Write([]byte(domain))
	binary.BigEndian.PutUint64(size[:], uint64(len(canonical)))
	_, _ = hasher.Write(size[:])
	_, _ = hasher.Write(canonical)
	raw := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return typedmemory.NewSHA256Digest(raw)
}

func verifyBasisState(actual basisState, expected basisState) error {
	if actual.root != expected.root ||
		actual.revision != expected.revision ||
		actual.ref != expected.ref ||
		actual.digest != expected.digest ||
		actual.ledgerDigest != expected.ledgerDigest ||
		!bytes.Equal(actual.canonical, expected.canonical) {
		return fmt.Errorf("current project-profile basis fields differ from canonical bytes")
	}
	return nil
}

func supportMembers(input DeclaredProjectProfileBasisInput) []supportMemberJSON {
	return []supportMemberJSON{
		{Role: "candidate_provenance", Digest: input.CandidateProvenanceDigest.String()},
		{Role: "work_record", Ref: input.WorkRecordRef.String(), Digest: input.WorkRecordDigest.String()},
		{Role: "authority_basis", Ref: input.AuthorityBasisRef.String(), Digest: input.AuthorityBasisDigest.String()},
		{Role: "authority_resolution", Ref: input.AuthorityResolutionRef.String(), Digest: input.AuthorityResolutionDigest.String()},
		{Role: "profile_author_role_assignment", Ref: input.ProfileAuthorRoleAssignmentRef.String(), Digest: input.ProfileAuthorRoleAssignmentDigest.String()},
		{Role: "observed_project_basis", Ref: input.ObservedProjectBasisRef.String(), Digest: input.ObservedProjectBasisDigest.String()},
		{Role: "outcome_assessment", Ref: input.OutcomeAssessmentRef.String(), Digest: input.OutcomeAssessmentDigest.String()},
	}
}

func validateDeclaredCoordinates(input DeclaredProjectProfileBasisInput) error {
	checks := []error{}
	_, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(input.AdmissionRecordRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.AdmissionRecordDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.ReceiptDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.CandidateProvenanceDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewProfileOnboardingWorkRecordRef(input.WorkRecordRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.WorkRecordDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewProfileDeclarationAuthorityBasisRef(input.AuthorityBasisRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.AuthorityBasisDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewAuthorityResolutionRecordRef(input.AuthorityResolutionRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.AuthorityResolutionDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewRoleAssignmentRef(input.ProfileAuthorRoleAssignmentRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.ProfileAuthorRoleAssignmentDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewObservedProjectBasisRefV1(input.ObservedProjectBasisRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.ObservedProjectBasisDigest.String())
	checks = append(checks, err)
	_, err = projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(input.OutcomeAssessmentRef.String())
	checks = append(checks, err)
	_, err = projectprofile.NewContentDigest(input.OutcomeAssessmentDigest.String())
	checks = append(checks, err)
	for _, check := range checks {
		if check != nil {
			return fmt.Errorf("declared project-profile basis coordinate: %w", check)
		}
	}
	return nil
}

func declaredInputFromJSON(dto basisJSON) (DeclaredProjectProfileBasisInput, error) {
	if dto.Schema != basisSchemaV1 || dto.Variant != "declared_canonical_project_profile" {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("unsupported declared project-profile basis schema or variant")
	}
	root, err := projectprofile.NewProjectRootV1(dto.ProjectRoot)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	payload, err := projectprofile.DecodeProfileDeclarationPayloadCanonicalJSON(dto.Payload)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(dto.AdmissionRecordRef)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	admissionDigest, err := projectprofile.NewContentDigest(dto.AdmissionRecordDigest)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	receiptDigest, err := projectprofile.NewContentDigest(dto.ReceiptDigest)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	member := func(role string) (supportMemberJSON, error) {
		for _, candidate := range dto.SupportMembers {
			if candidate.Role == role {
				return candidate, nil
			}
		}
		return supportMemberJSON{}, fmt.Errorf("declared project-profile basis omitted %s support", role)
	}
	provenance, err := member("candidate_provenance")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	work, err := member("work_record")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	authorityBasis, err := member("authority_basis")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	authorityResolution, err := member("authority_resolution")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	assignment, err := member("profile_author_role_assignment")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	observed, err := member("observed_project_basis")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	outcome, err := member("outcome_assessment")
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	input := DeclaredProjectProfileBasisInput{
		ProjectRoot: root, LedgerRevision: projectprofile.NewLedgerRevision(dto.LedgerRevision),
		Payload: payload, AdmissionRecordRef: admissionRef,
		AdmissionRecordDigest: admissionDigest, AdmissionRecordCanonicalJSON: dto.AdmissionRecord,
		ReceiptDigest: receiptDigest, ReceiptCanonicalJSON: dto.Receipt,
	}
	if err := decodeSupportMembers(&input, provenance, work, authorityBasis, authorityResolution, assignment, observed, outcome); err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	canonical, err := canonicalDeclaredInput(input)
	if err != nil {
		return DeclaredProjectProfileBasisInput{}, err
	}
	if canonicalPayloadDigest, _ := projectprofile.DigestProfileDeclarationPayload(canonical.Payload); canonicalPayloadDigest.String() != dto.PayloadDigest {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("declared project-profile basis payload digest mismatch")
	}
	supportJSON, _ := json.Marshal(dto.SupportMembers)
	supportDigest, err := digestBytes(supportDigestDomain, supportJSON)
	if err != nil || supportDigest.String() != dto.SupportDAGDigest {
		return DeclaredProjectProfileBasisInput{}, fmt.Errorf("declared project-profile basis support-DAG digest mismatch")
	}
	return canonical, nil
}

func decodeSupportMembers(
	input *DeclaredProjectProfileBasisInput,
	provenance supportMemberJSON,
	work supportMemberJSON,
	authorityBasis supportMemberJSON,
	authorityResolution supportMemberJSON,
	assignment supportMemberJSON,
	observed supportMemberJSON,
	outcome supportMemberJSON,
) error {
	var err error
	input.CandidateProvenanceDigest, err = projectprofile.NewContentDigest(provenance.Digest)
	if err != nil {
		return err
	}
	input.WorkRecordRef, err = projectprofile.NewProfileOnboardingWorkRecordRef(work.Ref)
	if err != nil {
		return err
	}
	input.WorkRecordDigest, err = projectprofile.NewContentDigest(work.Digest)
	if err != nil {
		return err
	}
	input.AuthorityBasisRef, err = projectprofile.NewProfileDeclarationAuthorityBasisRef(authorityBasis.Ref)
	if err != nil {
		return err
	}
	input.AuthorityBasisDigest, err = projectprofile.NewContentDigest(authorityBasis.Digest)
	if err != nil {
		return err
	}
	input.AuthorityResolutionRef, err = projectprofile.NewAuthorityResolutionRecordRef(authorityResolution.Ref)
	if err != nil {
		return err
	}
	input.AuthorityResolutionDigest, err = projectprofile.NewContentDigest(authorityResolution.Digest)
	if err != nil {
		return err
	}
	input.ProfileAuthorRoleAssignmentRef, err = projectprofile.NewRoleAssignmentRef(assignment.Ref)
	if err != nil {
		return err
	}
	input.ProfileAuthorRoleAssignmentDigest, err = projectprofile.NewContentDigest(assignment.Digest)
	if err != nil {
		return err
	}
	input.ObservedProjectBasisRef, err = projectprofile.NewObservedProjectBasisRefV1(observed.Ref)
	if err != nil {
		return err
	}
	input.ObservedProjectBasisDigest, err = projectprofile.NewContentDigest(observed.Digest)
	if err != nil {
		return err
	}
	input.OutcomeAssessmentRef, err = projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(outcome.Ref)
	if err != nil {
		return err
	}
	input.OutcomeAssessmentDigest, err = projectprofile.NewContentDigest(outcome.Digest)
	return err
}

func compactJSON(value []byte) []byte {
	buffer := bytes.Buffer{}
	if err := json.Compact(&buffer, value); err != nil {
		return append([]byte(nil), value...)
	}
	return buffer.Bytes()
}
