package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// CapabilityApplicabilityMissingBasis names why no canonical per-scope matrix
// can be returned. It does not infer a profile from repository shape or YAML.
type CapabilityApplicabilityMissingBasis string

const (
	MissingCurrentCanonicalProfileAdmission CapabilityApplicabilityMissingBasis = "current_canonical_profile_admission"
	MissingIntegrityValidProfileBasis       CapabilityApplicabilityMissingBasis = "integrity_valid_profile_basis"
)

type canonicalProfileApplicabilityBinding struct {
	projectRoot           projectprofile.ProjectRootV1
	origin                projectprofile.ProfileAdmissionOrigin
	admissionRecordRef    projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionRecordDigest projectprofile.ContentDigest
	payloadDigest         projectprofile.ContentDigest
	ledgerRevision        projectprofile.LedgerRevision
}

type capabilityApplicabilityMatrixResolvedState struct {
	binding canonicalProfileApplicabilityBinding
	matrix  projectprofile.CapabilityApplicabilityMatrix
}

// ResolvedCapabilityApplicabilityMatrix binds one pure matrix to the exact
// current canonical profile admission from which it was derived.
type ResolvedCapabilityApplicabilityMatrix struct {
	state *capabilityApplicabilityMatrixResolvedState
}

func (resolved ResolvedCapabilityApplicabilityMatrix) Valid() bool {
	return validateResolvedCapabilityApplicabilityMatrix(resolved) == nil
}

func (resolved ResolvedCapabilityApplicabilityMatrix) ProjectRoot() projectprofile.ProjectRootV1 {
	if !resolved.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return resolved.state.binding.projectRoot
}

func (resolved ResolvedCapabilityApplicabilityMatrix) Origin() projectprofile.ProfileAdmissionOrigin {
	if !resolved.Valid() {
		return ""
	}
	return resolved.state.binding.origin
}

func (resolved ResolvedCapabilityApplicabilityMatrix) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	if !resolved.Valid() {
		return projectprofile.ProfileDeclarationAdmissionRecordRef{}
	}
	return resolved.state.binding.admissionRecordRef
}

func (resolved ResolvedCapabilityApplicabilityMatrix) AdmissionRecordDigest() projectprofile.ContentDigest {
	if !resolved.Valid() {
		return projectprofile.ContentDigest{}
	}
	return resolved.state.binding.admissionRecordDigest
}

func (resolved ResolvedCapabilityApplicabilityMatrix) ProfilePayloadDigest() projectprofile.ContentDigest {
	if !resolved.Valid() {
		return projectprofile.ContentDigest{}
	}
	return resolved.state.binding.payloadDigest
}

func (resolved ResolvedCapabilityApplicabilityMatrix) LedgerRevision() projectprofile.LedgerRevision {
	if !resolved.Valid() {
		return projectprofile.LedgerRevision{}
	}
	return resolved.state.binding.ledgerRevision
}

func (resolved ResolvedCapabilityApplicabilityMatrix) Matrix() projectprofile.CapabilityApplicabilityMatrix {
	if !resolved.Valid() {
		return projectprofile.CapabilityApplicabilityMatrix{}
	}
	return resolved.state.matrix
}

type capabilityApplicabilityMatrixUnderdeterminedState struct {
	projectRoot  projectprofile.ProjectRootV1
	missingBasis CapabilityApplicabilityMissingBasis
}

// UnderdeterminedCapabilityApplicabilityMatrix is orientation-only. It cannot
// be used as Required or NotApplicable for any binding consumer.
type UnderdeterminedCapabilityApplicabilityMatrix struct {
	state *capabilityApplicabilityMatrixUnderdeterminedState
}

func (value UnderdeterminedCapabilityApplicabilityMatrix) Valid() bool {
	if value.state == nil {
		return false
	}
	rootPresent := value.state.projectRoot.String() != ""
	return rootPresent && validCapabilityApplicabilityMissingBasis(value.state.missingBasis)
}

func (value UnderdeterminedCapabilityApplicabilityMatrix) ProjectRoot() projectprofile.ProjectRootV1 {
	if !value.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return value.state.projectRoot
}

func (value UnderdeterminedCapabilityApplicabilityMatrix) MissingBasis() CapabilityApplicabilityMissingBasis {
	if !value.Valid() {
		return ""
	}
	return value.state.missingBasis
}

type CapabilityApplicabilityMatrixResultKind string

const (
	CapabilityApplicabilityMatrixResolved        CapabilityApplicabilityMatrixResultKind = "resolved"
	CapabilityApplicabilityMatrixUnderdetermined CapabilityApplicabilityMatrixResultKind = "underdetermined"
)

// CapabilityApplicabilityMatrixResult is a concrete opaque sum. Exactly one
// accessor succeeds.
type CapabilityApplicabilityMatrixResult struct {
	kind            CapabilityApplicabilityMatrixResultKind
	resolved        ResolvedCapabilityApplicabilityMatrix
	underdetermined UnderdeterminedCapabilityApplicabilityMatrix
}

func (result CapabilityApplicabilityMatrixResult) Kind() CapabilityApplicabilityMatrixResultKind {
	return result.kind
}

func (result CapabilityApplicabilityMatrixResult) Valid() bool {
	switch result.kind {
	case CapabilityApplicabilityMatrixResolved:
		return result.resolved.Valid() && !result.underdetermined.Valid()
	case CapabilityApplicabilityMatrixUnderdetermined:
		return !result.resolved.Valid() && result.underdetermined.Valid()
	default:
		return false
	}
}

func (result CapabilityApplicabilityMatrixResult) Resolved() (ResolvedCapabilityApplicabilityMatrix, bool) {
	if result.kind != CapabilityApplicabilityMatrixResolved || !result.Valid() {
		return ResolvedCapabilityApplicabilityMatrix{}, false
	}
	return result.resolved, true
}

func (result CapabilityApplicabilityMatrixResult) Underdetermined() (UnderdeterminedCapabilityApplicabilityMatrix, bool) {
	if result.kind != CapabilityApplicabilityMatrixUnderdetermined || !result.Valid() {
		return UnderdeterminedCapabilityApplicabilityMatrix{}, false
	}
	return result.underdetermined, true
}

// ResolveCapabilityApplicabilityMatrix performs one request-free canonical
// profile resolution and then invokes the pure projectprofile matrix. It
// performs no writes and never reads the YAML projection.
func (service Service) ResolveCapabilityApplicabilityMatrix(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) CapabilityApplicabilityMatrixResult {
	current := service.ResolveCurrent(ctx, projectRoot)
	admission, admitted := current.Admission()
	if admitted {
		return capabilityApplicabilityMatrixFromCanonicalAdmission(admission)
	}
	if hasDenialCode(current, "profile_not_declared") {
		return underdeterminedCapabilityApplicabilityMatrix(
			projectRoot,
			MissingCurrentCanonicalProfileAdmission,
		)
	}
	return underdeterminedCapabilityApplicabilityMatrix(
		projectRoot,
		MissingIntegrityValidProfileBasis,
	)
}

func capabilityApplicabilityMatrixFromCanonicalAdmission(
	admission CanonicalProfileAdmission,
) CapabilityApplicabilityMatrixResult {
	if !admission.Valid() {
		return underdeterminedCapabilityApplicabilityMatrix(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	binding, err := canonicalProfileApplicabilityBindingFrom(admission)
	if err != nil {
		return underdeterminedCapabilityApplicabilityMatrix(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	matrix, err := projectprofile.ResolveCapabilityApplicabilityMatrix(admission.Payload())
	if err != nil || matrix.ProfilePayloadDigest() != binding.payloadDigest {
		return underdeterminedCapabilityApplicabilityMatrix(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	resolved := ResolvedCapabilityApplicabilityMatrix{
		state: &capabilityApplicabilityMatrixResolvedState{
			binding: binding,
			matrix:  matrix,
		},
	}
	return CapabilityApplicabilityMatrixResult{
		kind:     CapabilityApplicabilityMatrixResolved,
		resolved: resolved,
	}
}

func canonicalProfileApplicabilityBindingFrom(
	admission CanonicalProfileAdmission,
) (canonicalProfileApplicabilityBinding, error) {
	binding := canonicalProfileApplicabilityBinding{
		projectRoot:           admission.ProjectRoot(),
		origin:                admission.Origin(),
		admissionRecordRef:    admission.AdmissionRecordRef(),
		admissionRecordDigest: admission.AdmissionRecordDigest(),
		payloadDigest:         admission.PayloadDigest(),
		ledgerRevision:        admission.LedgerRevision(),
	}
	if err := validateCanonicalProfileApplicabilityBinding(binding); err != nil {
		return canonicalProfileApplicabilityBinding{}, err
	}
	return binding, nil
}

func validateResolvedCapabilityApplicabilityMatrix(
	resolved ResolvedCapabilityApplicabilityMatrix,
) error {
	if resolved.state == nil {
		return fmt.Errorf("resolved capability applicability matrix is absent")
	}
	if err := validateCanonicalProfileApplicabilityBinding(resolved.state.binding); err != nil {
		return err
	}
	if !resolved.state.matrix.Valid() {
		return fmt.Errorf("resolved capability applicability matrix is invalid")
	}
	if resolved.state.matrix.ProfilePayloadDigest() != resolved.state.binding.payloadDigest {
		return fmt.Errorf("resolved capability applicability matrix has another profile payload")
	}
	return nil
}

func validateCanonicalProfileApplicabilityBinding(
	binding canonicalProfileApplicabilityBinding,
) error {
	if binding.projectRoot.String() == "" {
		return fmt.Errorf("canonical profile applicability project root is absent")
	}
	if _, ok := projectprofile.ParseProfileAdmissionOrigin(string(binding.origin)); !ok {
		return fmt.Errorf("canonical profile applicability origin is invalid")
	}
	if binding.admissionRecordRef.String() == "" || binding.admissionRecordDigest.String() == "" {
		return fmt.Errorf("canonical profile applicability admission binding is absent")
	}
	if binding.payloadDigest.String() == "" || binding.ledgerRevision.Value() == 0 {
		return fmt.Errorf("canonical profile applicability revision binding is absent")
	}
	return nil
}

func underdeterminedCapabilityApplicabilityMatrix(
	projectRoot projectprofile.ProjectRootV1,
	missingBasis CapabilityApplicabilityMissingBasis,
) CapabilityApplicabilityMatrixResult {
	value := UnderdeterminedCapabilityApplicabilityMatrix{
		state: &capabilityApplicabilityMatrixUnderdeterminedState{
			projectRoot:  projectRoot,
			missingBasis: missingBasis,
		},
	}
	return CapabilityApplicabilityMatrixResult{
		kind:            CapabilityApplicabilityMatrixUnderdetermined,
		underdetermined: value,
	}
}

func validCapabilityApplicabilityMissingBasis(
	value CapabilityApplicabilityMissingBasis,
) bool {
	return value == MissingCurrentCanonicalProfileAdmission ||
		value == MissingIntegrityValidProfileBasis
}
