package sqlite

import (
	"context"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// SoftwareSystemSpecMigrationApplicabilityKind is the closed binding result
// for the SoftwareSystemSpec migration capability. It is derived from the
// current canonical profile admission, never from YAML or caller fields.
type SoftwareSystemSpecMigrationApplicabilityKind string

const (
	SoftwareSystemSpecMigrationApplicabilityRequired        SoftwareSystemSpecMigrationApplicabilityKind = "required"
	SoftwareSystemSpecMigrationApplicabilityNotApplicable   SoftwareSystemSpecMigrationApplicabilityKind = "not_applicable"
	SoftwareSystemSpecMigrationApplicabilityUnderdetermined SoftwareSystemSpecMigrationApplicabilityKind = "underdetermined"
)

// SoftwareSystemSpecMigrationMissingBasis is retained as the migration-facing
// name for the shared canonical-profile applicability basis.
type SoftwareSystemSpecMigrationMissingBasis = CapabilityApplicabilityMissingBasis

type softwareSystemSpecMigrationBinding = canonicalProfileApplicabilityBinding

type softwareSystemSpecMigrationRequiredState struct {
	binding          softwareSystemSpecMigrationBinding
	softwareScopeIDs []projectprofile.ScopeID
}

// SoftwareSystemSpecMigrationRequired is the exact effect-usable capability.
// Its zero value is invalid and no public constructor or decoder exists.
// Service mints it only after ResolveCurrent has reread the durable admission.
type SoftwareSystemSpecMigrationRequired struct {
	state *softwareSystemSpecMigrationRequiredState
}

func (required SoftwareSystemSpecMigrationRequired) Valid() bool {
	return validateSoftwareSystemSpecMigrationRequired(required) == nil
}

func (required SoftwareSystemSpecMigrationRequired) ProjectRoot() projectprofile.ProjectRootV1 {
	if !required.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return required.state.binding.projectRoot
}

func (required SoftwareSystemSpecMigrationRequired) SoftwareScopeIDs() []projectprofile.ScopeID {
	if !required.Valid() {
		return nil
	}
	return append([]projectprofile.ScopeID{}, required.state.softwareScopeIDs...)
}

func (required SoftwareSystemSpecMigrationRequired) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	if !required.Valid() {
		return projectprofile.ProfileDeclarationAdmissionRecordRef{}
	}
	return required.state.binding.admissionRecordRef
}

func (required SoftwareSystemSpecMigrationRequired) AdmissionRecordDigest() projectprofile.ContentDigest {
	if !required.Valid() {
		return projectprofile.ContentDigest{}
	}
	return required.state.binding.admissionRecordDigest
}

func (required SoftwareSystemSpecMigrationRequired) ProfilePayloadDigest() projectprofile.ContentDigest {
	if !required.Valid() {
		return projectprofile.ContentDigest{}
	}
	return required.state.binding.payloadDigest
}

func (required SoftwareSystemSpecMigrationRequired) LedgerRevision() projectprofile.LedgerRevision {
	if !required.Valid() {
		return projectprofile.LedgerRevision{}
	}
	return required.state.binding.ledgerRevision
}

// SameCurrentBinding compares the complete effect-bearing identity. A token
// is current only when a fresh request-free resolver produces the same value.
func (required SoftwareSystemSpecMigrationRequired) SameCurrentBinding(
	other SoftwareSystemSpecMigrationRequired,
) bool {
	if !required.Valid() || !other.Valid() {
		return false
	}
	left := required.state
	right := other.state
	bindingsMatch := left.binding == right.binding
	scopesMatch := slices.Equal(left.softwareScopeIDs, right.softwareScopeIDs)
	return bindingsMatch && scopesMatch
}

type softwareSystemSpecMigrationNotApplicableState struct {
	binding softwareSystemSpecMigrationBinding
}

// SoftwareSystemSpecMigrationNotApplicableValue is a successful binding
// result for an integrity-valid Declared profile with no software scope.
type SoftwareSystemSpecMigrationNotApplicableValue struct {
	state *softwareSystemSpecMigrationNotApplicableState
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) Valid() bool {
	return value.state != nil && validateSoftwareSystemSpecMigrationBinding(value.state.binding) == nil
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) ProjectRoot() projectprofile.ProjectRootV1 {
	if !value.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return value.state.binding.projectRoot
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) AdmissionRecordRef() projectprofile.ProfileDeclarationAdmissionRecordRef {
	if !value.Valid() {
		return projectprofile.ProfileDeclarationAdmissionRecordRef{}
	}
	return value.state.binding.admissionRecordRef
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) AdmissionRecordDigest() projectprofile.ContentDigest {
	if !value.Valid() {
		return projectprofile.ContentDigest{}
	}
	return value.state.binding.admissionRecordDigest
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) ProfilePayloadDigest() projectprofile.ContentDigest {
	if !value.Valid() {
		return projectprofile.ContentDigest{}
	}
	return value.state.binding.payloadDigest
}

func (value SoftwareSystemSpecMigrationNotApplicableValue) LedgerRevision() projectprofile.LedgerRevision {
	if !value.Valid() {
		return projectprofile.LedgerRevision{}
	}
	return value.state.binding.ledgerRevision
}

type softwareSystemSpecMigrationUnderdeterminedState struct {
	projectRoot  projectprofile.ProjectRootV1
	missingBasis SoftwareSystemSpecMigrationMissingBasis
}

// SoftwareSystemSpecMigrationUnderdeterminedValue is fail-closed orientation;
// it is never an effect capability.
type SoftwareSystemSpecMigrationUnderdeterminedValue struct {
	state *softwareSystemSpecMigrationUnderdeterminedState
}

func (value SoftwareSystemSpecMigrationUnderdeterminedValue) Valid() bool {
	if value.state == nil {
		return false
	}
	rootPresent := value.state.projectRoot.String() != ""
	return rootPresent && validSoftwareSystemSpecMigrationMissingBasis(value.state.missingBasis)
}

func (value SoftwareSystemSpecMigrationUnderdeterminedValue) ProjectRoot() projectprofile.ProjectRootV1 {
	if !value.Valid() {
		return projectprofile.ProjectRootV1{}
	}
	return value.state.projectRoot
}

func (value SoftwareSystemSpecMigrationUnderdeterminedValue) MissingBasis() SoftwareSystemSpecMigrationMissingBasis {
	if !value.Valid() {
		return ""
	}
	return value.state.missingBasis
}

// SoftwareSystemSpecMigrationApplicability is a concrete opaque sum. Exactly
// one accessor succeeds, so callers cannot combine incompatible variants.
type SoftwareSystemSpecMigrationApplicability struct {
	kind            SoftwareSystemSpecMigrationApplicabilityKind
	required        SoftwareSystemSpecMigrationRequired
	notApplicable   SoftwareSystemSpecMigrationNotApplicableValue
	underdetermined SoftwareSystemSpecMigrationUnderdeterminedValue
}

// SoftwareSystemSpecMigrationProofValidationKind is the closed result of a
// service-owned current-head or historical-exact reread.
type SoftwareSystemSpecMigrationProofValidationKind string

const (
	SoftwareSystemSpecMigrationProofValid           SoftwareSystemSpecMigrationProofValidationKind = "valid"
	SoftwareSystemSpecMigrationProofInvalid         SoftwareSystemSpecMigrationProofValidationKind = "invalid"
	SoftwareSystemSpecMigrationProofNotCurrent      SoftwareSystemSpecMigrationProofValidationKind = "not_current"
	SoftwareSystemSpecMigrationProofNotApplicable   SoftwareSystemSpecMigrationProofValidationKind = "not_applicable"
	SoftwareSystemSpecMigrationProofUnderdetermined SoftwareSystemSpecMigrationProofValidationKind = "underdetermined"
)

func (result SoftwareSystemSpecMigrationApplicability) Kind() SoftwareSystemSpecMigrationApplicabilityKind {
	return result.kind
}

func (result SoftwareSystemSpecMigrationApplicability) Valid() bool {
	switch result.kind {
	case SoftwareSystemSpecMigrationApplicabilityRequired:
		return result.required.Valid() && !result.notApplicable.Valid() && !result.underdetermined.Valid()
	case SoftwareSystemSpecMigrationApplicabilityNotApplicable:
		return !result.required.Valid() && result.notApplicable.Valid() && !result.underdetermined.Valid()
	case SoftwareSystemSpecMigrationApplicabilityUnderdetermined:
		return !result.required.Valid() && !result.notApplicable.Valid() && result.underdetermined.Valid()
	default:
		return false
	}
}

func (result SoftwareSystemSpecMigrationApplicability) Required() (SoftwareSystemSpecMigrationRequired, bool) {
	if result.kind != SoftwareSystemSpecMigrationApplicabilityRequired || !result.Valid() {
		return SoftwareSystemSpecMigrationRequired{}, false
	}
	return result.required, true
}

func (result SoftwareSystemSpecMigrationApplicability) NotApplicable() (SoftwareSystemSpecMigrationNotApplicableValue, bool) {
	if result.kind != SoftwareSystemSpecMigrationApplicabilityNotApplicable || !result.Valid() {
		return SoftwareSystemSpecMigrationNotApplicableValue{}, false
	}
	return result.notApplicable, true
}

func (result SoftwareSystemSpecMigrationApplicability) Underdetermined() (SoftwareSystemSpecMigrationUnderdeterminedValue, bool) {
	if result.kind != SoftwareSystemSpecMigrationApplicabilityUnderdetermined || !result.Valid() {
		return SoftwareSystemSpecMigrationUnderdeterminedValue{}, false
	}
	return result.underdetermined, true
}

// ResolveSoftwareSystemSpecMigration derives one binding result from the
// current canonical ledger head. It performs no writes and never consults the
// human-readable projection.
func (service Service) ResolveSoftwareSystemSpecMigration(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) SoftwareSystemSpecMigrationApplicability {
	resolved := service.ResolveCurrent(ctx, projectRoot)
	admission, admitted := resolved.Admission()
	if admitted {
		return applicabilityFromCanonicalAdmission(admission)
	}
	if hasDenialCode(resolved, "profile_not_declared") {
		return softwareSystemSpecMigrationUnderdetermined(
			projectRoot,
			MissingCurrentCanonicalProfileAdmission,
		)
	}
	return softwareSystemSpecMigrationUnderdetermined(
		projectRoot,
		MissingIntegrityValidProfileBasis,
	)
}

// ResolveHistoricalSoftwareSystemSpecMigration reconstructs applicability
// from one exact durable admission identity. It exists for restart recovery;
// caller-supplied refs are lookup coordinates, never proof by themselves.
func (service Service) ResolveHistoricalSoftwareSystemSpecMigration(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	admissionRef projectprofile.ProfileDeclarationAdmissionRecordRef,
	admissionDigest projectprofile.ContentDigest,
) SoftwareSystemSpecMigrationApplicability {
	if ctx == nil || service.adapter.database == nil {
		return softwareSystemSpecMigrationUnderdetermined(
			projectRoot,
			MissingIntegrityValidProfileBasis,
		)
	}
	material, err := service.adapter.resolveCanonicalByReference(
		ctx,
		projectRoot,
		admissionRef,
		admissionDigest,
	)
	if err != nil {
		return softwareSystemSpecMigrationUnderdetermined(
			projectRoot,
			MissingIntegrityValidProfileBasis,
		)
	}
	admission, err := newCanonicalProfileAdmission(
		material,
		CanonicalAdmissionResolvedAfterRestart,
	)
	if err != nil {
		return softwareSystemSpecMigrationUnderdetermined(
			projectRoot,
			MissingIntegrityValidProfileBasis,
		)
	}
	return applicabilityFromCanonicalAdmission(admission)
}

// ValidateCurrentSoftwareSystemSpecMigrationRequired closes the freshness
// gate immediately before a new effect. It compares against a new durable
// current-head reread, not against cached or projected state.
func (service Service) ValidateCurrentSoftwareSystemSpecMigrationRequired(
	ctx context.Context,
	required SoftwareSystemSpecMigrationRequired,
) SoftwareSystemSpecMigrationProofValidationKind {
	if ctx == nil || !required.Valid() {
		return SoftwareSystemSpecMigrationProofInvalid
	}
	current := service.ResolveSoftwareSystemSpecMigration(ctx, required.ProjectRoot())
	currentRequired, ok := current.Required()
	if ok && required.SameCurrentBinding(currentRequired) {
		return SoftwareSystemSpecMigrationProofValid
	}
	if ok {
		return SoftwareSystemSpecMigrationProofNotCurrent
	}
	if notApplicable, ok := current.NotApplicable(); ok {
		if required.state.binding == notApplicable.state.binding {
			return SoftwareSystemSpecMigrationProofInvalid
		}
		return SoftwareSystemSpecMigrationProofNotCurrent
	}
	return SoftwareSystemSpecMigrationProofUnderdetermined
}

// ValidateHistoricalSoftwareSystemSpecMigrationRequired rereads the exact
// admission named by an already-started effect. Recovery must remain possible
// after the current profile head advances, while still failing closed if the
// journal-bound historical admission or its support closure is corrupt.
func (service Service) ValidateHistoricalSoftwareSystemSpecMigrationRequired(
	ctx context.Context,
	required SoftwareSystemSpecMigrationRequired,
) SoftwareSystemSpecMigrationProofValidationKind {
	if ctx == nil || !required.Valid() || service.adapter.database == nil {
		return SoftwareSystemSpecMigrationProofInvalid
	}
	resolved := service.ResolveHistoricalSoftwareSystemSpecMigration(
		ctx,
		required.ProjectRoot(),
		required.AdmissionRecordRef(),
		required.AdmissionRecordDigest(),
	)
	historicalRequired, ok := resolved.Required()
	if ok && required.SameCurrentBinding(historicalRequired) {
		return SoftwareSystemSpecMigrationProofValid
	}
	if ok {
		return SoftwareSystemSpecMigrationProofInvalid
	}
	if resolved.Kind() == SoftwareSystemSpecMigrationApplicabilityNotApplicable {
		return SoftwareSystemSpecMigrationProofNotApplicable
	}
	return SoftwareSystemSpecMigrationProofUnderdetermined
}

func applicabilityFromCanonicalAdmission(
	admission CanonicalProfileAdmission,
) SoftwareSystemSpecMigrationApplicability {
	if !admission.Valid() {
		return softwareSystemSpecMigrationUnderdetermined(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	binding, err := softwareSystemSpecMigrationBindingFrom(admission)
	if err != nil {
		return softwareSystemSpecMigrationUnderdetermined(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	softwareScopeIDs, err := softwareScopeIDs(admission.Payload())
	if err != nil {
		return softwareSystemSpecMigrationUnderdetermined(
			admission.ProjectRoot(),
			MissingIntegrityValidProfileBasis,
		)
	}
	if len(softwareScopeIDs) == 0 {
		value := SoftwareSystemSpecMigrationNotApplicableValue{
			state: &softwareSystemSpecMigrationNotApplicableState{binding: binding},
		}
		return SoftwareSystemSpecMigrationApplicability{
			kind:          SoftwareSystemSpecMigrationApplicabilityNotApplicable,
			notApplicable: value,
		}
	}
	required := SoftwareSystemSpecMigrationRequired{
		state: &softwareSystemSpecMigrationRequiredState{
			binding:          binding,
			softwareScopeIDs: softwareScopeIDs,
		},
	}
	return SoftwareSystemSpecMigrationApplicability{
		kind:     SoftwareSystemSpecMigrationApplicabilityRequired,
		required: required,
	}
}

func softwareSystemSpecMigrationBindingFrom(
	admission CanonicalProfileAdmission,
) (softwareSystemSpecMigrationBinding, error) {
	binding, err := canonicalProfileApplicabilityBindingFrom(admission)
	if err != nil {
		return softwareSystemSpecMigrationBinding{}, err
	}
	if err := validateSoftwareSystemSpecMigrationBinding(binding); err != nil {
		return softwareSystemSpecMigrationBinding{}, err
	}
	return binding, nil
}

func softwareScopeIDs(
	payload projectprofile.ProfileDeclarationPayload,
) ([]projectprofile.ScopeID, error) {
	if err := projectprofile.ValidateProfileDeclarationPayloadStructuralConsistencyV1(payload); err != nil {
		return nil, err
	}
	values := payload.Scopes().Values()
	result := make([]projectprofile.ScopeID, 0, len(values))
	for _, scope := range values {
		if _, software := scope.(projectprofile.SoftwareRealization); software {
			result = append(result, scope.ScopeID())
		}
	}
	slices.SortFunc(result, func(left projectprofile.ScopeID, right projectprofile.ScopeID) int {
		if left.String() < right.String() {
			return -1
		}
		if left.String() > right.String() {
			return 1
		}
		return 0
	})
	return result, nil
}

func validateSoftwareSystemSpecMigrationRequired(
	required SoftwareSystemSpecMigrationRequired,
) error {
	if required.state == nil {
		return fmt.Errorf("SoftwareSystemSpec migration applicability is absent")
	}
	if err := validateSoftwareSystemSpecMigrationBinding(required.state.binding); err != nil {
		return err
	}
	if len(required.state.softwareScopeIDs) == 0 {
		return fmt.Errorf("SoftwareSystemSpec migration requires at least one software scope")
	}
	previous := ""
	for _, scopeID := range required.state.softwareScopeIDs {
		current := scopeID.String()
		if current == "" || current <= previous {
			return fmt.Errorf("SoftwareSystemSpec migration scope IDs are not canonical")
		}
		previous = current
	}
	return nil
}

func validateSoftwareSystemSpecMigrationBinding(
	binding softwareSystemSpecMigrationBinding,
) error {
	if err := validateCanonicalProfileApplicabilityBinding(binding); err != nil {
		return fmt.Errorf("SoftwareSystemSpec migration applicability: %w", err)
	}
	return nil
}

func softwareSystemSpecMigrationUnderdetermined(
	projectRoot projectprofile.ProjectRootV1,
	missingBasis SoftwareSystemSpecMigrationMissingBasis,
) SoftwareSystemSpecMigrationApplicability {
	value := SoftwareSystemSpecMigrationUnderdeterminedValue{
		state: &softwareSystemSpecMigrationUnderdeterminedState{
			projectRoot:  projectRoot,
			missingBasis: missingBasis,
		},
	}
	return SoftwareSystemSpecMigrationApplicability{
		kind:            SoftwareSystemSpecMigrationApplicabilityUnderdetermined,
		underdetermined: value,
	}
}

func hasDenialCode(result AdmissionResult, code string) bool {
	denials, ok := result.Denials()
	if !ok {
		return false
	}
	return slices.ContainsFunc(denials, func(denial AdmissionDenial) bool {
		return denial.Code() == code
	})
}

func validSoftwareSystemSpecMigrationMissingBasis(
	value SoftwareSystemSpecMigrationMissingBasis,
) bool {
	return value == MissingCurrentCanonicalProfileAdmission ||
		value == MissingIntegrityValidProfileBasis
}
