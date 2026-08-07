package project

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type specificationMemberPolicy struct {
	documentKind SpecDocumentKind
	capability   projectprofile.Capability
}

var specificationMemberPolicies = []specificationMemberPolicy{
	{
		documentKind: SpecDocumentKindTargetSystem,
		capability:   projectprofile.TargetSystemSpecCapability,
	},
	{
		documentKind: SpecDocumentKindSoftwareSystem,
		capability:   projectprofile.SoftwareSystemSpecCapability,
	},
	{
		documentKind: SpecDocumentKindTermMap,
		capability:   projectprofile.TermMapCapability,
	},
}

// ProjectSpecificationMemberApplicability retains the exact profile entry
// used to include or exclude one built-in specification carrier.
type ProjectSpecificationMemberApplicability struct {
	documentKind SpecDocumentKind
	entry        projectprofile.CapabilityApplicabilityEntry
}

func (member ProjectSpecificationMemberApplicability) Valid() bool {
	policy, found := specificationMemberPolicyForDocumentKind(member.documentKind)
	if !found || !member.entry.Valid() {
		return false
	}
	return member.entry.Capability() == policy.capability
}

func (member ProjectSpecificationMemberApplicability) DocumentKind() SpecDocumentKind {
	if !member.Valid() {
		return ""
	}
	return member.documentKind
}

func (member ProjectSpecificationMemberApplicability) ScopeID() projectprofile.ScopeID {
	if !member.Valid() {
		return projectprofile.ScopeID{}
	}
	return member.entry.ScopeID()
}

func (member ProjectSpecificationMemberApplicability) Capability() projectprofile.Capability {
	if !member.Valid() {
		return ""
	}
	return member.entry.Capability()
}

func (member ProjectSpecificationMemberApplicability) Kind() projectprofile.CapabilityApplicabilityKind {
	if !member.Valid() {
		return ""
	}
	return member.entry.Kind()
}

func (member ProjectSpecificationMemberApplicability) MissingBasis() (
	projectprofile.CapabilityApplicabilityMissingBasis,
	bool,
) {
	if !member.Valid() {
		return "", false
	}
	return member.entry.MissingBasis()
}

// ProjectSpecificationSetApplicability is one immutable scope-local view over
// the central matrix. It is a read projection, not a profile admission or
// specification lifecycle act.
type ProjectSpecificationSetApplicability struct {
	matrix  projectprofile.CapabilityApplicabilityMatrix
	scopeID projectprofile.ScopeID
}

func (applicability ProjectSpecificationSetApplicability) Valid() bool {
	return validateProjectSpecificationSetApplicability(applicability) == nil
}

func (applicability ProjectSpecificationSetApplicability) ScopeID() projectprofile.ScopeID {
	if !applicability.Valid() {
		return projectprofile.ScopeID{}
	}
	return applicability.scopeID
}

func (applicability ProjectSpecificationSetApplicability) ProfilePayloadDigest() projectprofile.ContentDigest {
	if !applicability.Valid() {
		return projectprofile.ContentDigest{}
	}
	return applicability.matrix.ProfilePayloadDigest()
}

func (applicability ProjectSpecificationSetApplicability) Members() []ProjectSpecificationMemberApplicability {
	if !applicability.Valid() {
		return nil
	}
	members, err := projectSpecificationMembers(
		applicability.matrix,
		applicability.scopeID,
	)
	if err != nil {
		return nil
	}
	return members
}

func (applicability ProjectSpecificationSetApplicability) Member(
	documentKind SpecDocumentKind,
) (ProjectSpecificationMemberApplicability, bool) {
	if !applicability.Valid() {
		return ProjectSpecificationMemberApplicability{}, false
	}
	policy, found := specificationMemberPolicyForDocumentKind(documentKind)
	if !found {
		return ProjectSpecificationMemberApplicability{}, false
	}
	entry, found := applicability.matrix.Entry(applicability.scopeID, policy.capability)
	if !found {
		return ProjectSpecificationMemberApplicability{}, false
	}
	member := ProjectSpecificationMemberApplicability{
		documentKind: documentKind,
		entry:        entry,
	}
	return member, member.Valid()
}

// ScopedCapabilityApplicability exposes the canonical pure projection for one
// non-carrier capability at this exact selected ScopeID.
func (applicability ProjectSpecificationSetApplicability) ScopedCapabilityApplicability(
	capability projectprofile.Capability,
) (projectprofile.ScopedCapabilityApplicability, error) {
	if !applicability.Valid() {
		return projectprofile.ScopedCapabilityApplicability{}, fmt.Errorf(
			"project specification applicability is invalid",
		)
	}
	return projectprofile.ResolveScopedCapabilityApplicability(
		applicability.matrix,
		applicability.scopeID,
		capability,
	)
}

func (applicability ProjectSpecificationSetApplicability) ApplicableDocumentKinds() []SpecDocumentKind {
	return applicability.documentKindsWith(projectprofile.CapabilityRequired)
}

func (applicability ProjectSpecificationSetApplicability) ExcludedDocumentKinds() []SpecDocumentKind {
	return applicability.documentKindsWith(projectprofile.CapabilityNotApplicable)
}

func (applicability ProjectSpecificationSetApplicability) UnderdeterminedDocumentKinds() []SpecDocumentKind {
	return applicability.documentKindsWith(projectprofile.CapabilityUnderdetermined)
}

func (applicability ProjectSpecificationSetApplicability) documentKindsWith(
	kind projectprofile.CapabilityApplicabilityKind,
) []SpecDocumentKind {
	members := applicability.Members()
	selected := slices.DeleteFunc(
		append([]ProjectSpecificationMemberApplicability{}, members...),
		func(member ProjectSpecificationMemberApplicability) bool {
			return member.Kind() != kind
		},
	)
	result := make([]SpecDocumentKind, len(selected))
	fillSpecificationDocumentKinds(selected, result, 0)
	return result
}

func DeriveProjectSpecificationSetApplicability(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	scopeID projectprofile.ScopeID,
) (ProjectSpecificationSetApplicability, error) {
	applicability := ProjectSpecificationSetApplicability{
		matrix:  matrix,
		scopeID: scopeID,
	}
	if err := validateProjectSpecificationSetApplicability(applicability); err != nil {
		return ProjectSpecificationSetApplicability{}, err
	}
	return applicability, nil
}

func projectSpecificationMembers(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	scopeID projectprofile.ScopeID,
) ([]ProjectSpecificationMemberApplicability, error) {
	result := make(
		[]ProjectSpecificationMemberApplicability,
		len(specificationMemberPolicies),
	)
	err := fillProjectSpecificationMembers(
		matrix,
		scopeID,
		result,
		0,
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func fillProjectSpecificationMembers(
	matrix projectprofile.CapabilityApplicabilityMatrix,
	scopeID projectprofile.ScopeID,
	result []ProjectSpecificationMemberApplicability,
	index int,
) error {
	if index == len(specificationMemberPolicies) {
		return nil
	}
	policy := specificationMemberPolicies[index]
	entry, found := matrix.Entry(scopeID, policy.capability)
	if !found {
		return fmt.Errorf(
			"capability matrix has no %q entry for scope %q",
			policy.capability,
			scopeID.String(),
		)
	}
	member := ProjectSpecificationMemberApplicability{
		documentKind: policy.documentKind,
		entry:        entry,
	}
	if !member.Valid() {
		return fmt.Errorf(
			"capability matrix has an invalid %q entry for scope %q",
			policy.capability,
			scopeID.String(),
		)
	}
	result[index] = member
	return fillProjectSpecificationMembers(
		matrix,
		scopeID,
		result,
		index+1,
	)
}

func fillSpecificationDocumentKinds(
	members []ProjectSpecificationMemberApplicability,
	result []SpecDocumentKind,
	index int,
) {
	if index == len(members) {
		return
	}
	result[index] = members[index].DocumentKind()
	fillSpecificationDocumentKinds(members, result, index+1)
}

func validateProjectSpecificationSetApplicability(
	applicability ProjectSpecificationSetApplicability,
) error {
	if !applicability.matrix.Valid() {
		return fmt.Errorf("project specification applicability matrix is invalid")
	}
	canonicalScopeID, err := projectprofile.NewScopeID(applicability.scopeID.String())
	if err != nil || canonicalScopeID != applicability.scopeID {
		return fmt.Errorf("project specification applicability ScopeID is invalid")
	}
	_, err = projectSpecificationMembers(
		applicability.matrix,
		applicability.scopeID,
	)
	return err
}

func specificationMemberPolicyForDocumentKind(
	documentKind SpecDocumentKind,
) (specificationMemberPolicy, bool) {
	switch documentKind {
	case SpecDocumentKindTargetSystem:
		return specificationMemberPolicies[0], true
	case SpecDocumentKindSoftwareSystem:
		return specificationMemberPolicies[1], true
	case SpecDocumentKindTermMap:
		return specificationMemberPolicies[2], true
	default:
		return specificationMemberPolicy{}, false
	}
}

func applicableSpecDocumentInputs(
	documents []SpecDocumentInput,
	applicability ProjectSpecificationSetApplicability,
) []SpecDocumentInput {
	copied := append([]SpecDocumentInput{}, documents...)
	return slices.DeleteFunc(copied, func(document SpecDocumentInput) bool {
		kind := SpecDocumentKind(document.Kind)
		if kind == SpecDocumentKindEnablingSystem {
			return true
		}
		member, found := applicability.Member(kind)
		return found && member.Kind() != projectprofile.CapabilityRequired
	})
}

func requiredSpecCheckCarriers(
	applicability ProjectSpecificationSetApplicability,
) ([]specCheckCarrier, error) {
	if !applicability.Valid() {
		return nil, fmt.Errorf(
			"project specification applicability is invalid",
		)
	}
	carriers := append([]specCheckCarrier{}, specCheckCarriers...)
	selected := slices.DeleteFunc(carriers, func(carrier specCheckCarrier) bool {
		member, found := applicability.Member(SpecDocumentKind(carrier.kind))
		return !found || member.Kind() != projectprofile.CapabilityRequired
	})
	return selected, nil
}
