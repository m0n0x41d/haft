package projectprofile

import (
	"cmp"
	"fmt"
	"math"
	"path/filepath"
	"slices"
)

// ProjectRootV1 is the canonical absolute project-root identity used by the
// final-v1 profile algebra. It is a lexical identity; filesystem resolution
// and symlink admission remain effect-boundary responsibilities.
type ProjectRootV1 struct {
	value string
}

func NewProjectRootV1(raw string) (ProjectRootV1, error) {
	value, err := requireText("project root", raw)
	if err != nil {
		return ProjectRootV1{}, err
	}
	if !filepath.IsAbs(value) {
		return ProjectRootV1{}, fmt.Errorf("project root must be absolute")
	}
	canonical := filepath.Clean(value)
	if canonical != value {
		return ProjectRootV1{}, fmt.Errorf("project root must use canonical lexical form")
	}
	return ProjectRootV1{value: value}, nil
}

func (root ProjectRootV1) String() string {
	return root.value
}

func (root ProjectRootV1) valid() bool {
	_, err := NewProjectRootV1(root.value)
	return err == nil
}

type v1Reference struct {
	value string
}

func newV1Reference(name string, raw string) (v1Reference, error) {
	value, err := parseReference(name, raw)
	if err != nil {
		return v1Reference{}, err
	}
	return v1Reference{value: value}, nil
}

func (ref v1Reference) String() string {
	return ref.value
}

func (ref v1Reference) valid() bool {
	return validReference(ref.value)
}

type ProfileDeclarationAuthorityBasisRef struct{ v1Reference }
type ProfileOnboardingWorkRecordRef struct{ v1Reference }
type WorkRef struct{ v1Reference }
type MethodRef struct{ v1Reference }
type MethodDescriptionRef struct{ v1Reference }
type RoleRef struct{ v1Reference }
type RoleAssignmentRef struct{ v1Reference }
type SystemRef struct{ v1Reference }
type BoundedContextRef struct{ v1Reference }
type WorkInputRef struct{ v1Reference }
type WorkOutputRef struct{ v1Reference }
type WorkResourceRef struct{ v1Reference }
type AffectedReferentRef struct{ v1Reference }
type StatePlaneRef struct{ v1Reference }
type StateRef struct{ v1Reference }
type DeltaPredicateRef struct{ v1Reference }
type AuthorityResolutionRecordRef struct{ v1Reference }
type ProfileDeclarationAdmissionRecordRef struct{ v1Reference }
type SingleUseKey struct{ v1Reference }
type SessionRef struct{ v1Reference }

func NewProfileDeclarationAuthorityBasisRef(raw string) (ProfileDeclarationAuthorityBasisRef, error) {
	ref, err := newV1Reference("profile declaration authority-basis ref", raw)
	return ProfileDeclarationAuthorityBasisRef{v1Reference: ref}, err
}

func NewProfileOnboardingWorkRecordRef(raw string) (ProfileOnboardingWorkRecordRef, error) {
	ref, err := newV1Reference("profile onboarding Work-record ref", raw)
	return ProfileOnboardingWorkRecordRef{v1Reference: ref}, err
}

func NewWorkRef(raw string) (WorkRef, error) {
	ref, err := newV1Reference("Work ref", raw)
	return WorkRef{v1Reference: ref}, err
}

func NewMethodRef(raw string) (MethodRef, error) {
	ref, err := newV1Reference("Method ref", raw)
	return MethodRef{v1Reference: ref}, err
}

func NewMethodDescriptionRef(raw string) (MethodDescriptionRef, error) {
	ref, err := newV1Reference("MethodDescription ref", raw)
	return MethodDescriptionRef{v1Reference: ref}, err
}

func NewRoleRef(raw string) (RoleRef, error) {
	ref, err := newV1Reference("Role ref", raw)
	return RoleRef{v1Reference: ref}, err
}

func NewRoleAssignmentRef(raw string) (RoleAssignmentRef, error) {
	ref, err := newV1Reference("RoleAssignment ref", raw)
	return RoleAssignmentRef{v1Reference: ref}, err
}

func NewSystemRef(raw string) (SystemRef, error) {
	ref, err := newV1Reference("System ref", raw)
	return SystemRef{v1Reference: ref}, err
}

func NewBoundedContextRef(raw string) (BoundedContextRef, error) {
	ref, err := newV1Reference("BoundedContext ref", raw)
	return BoundedContextRef{v1Reference: ref}, err
}

func NewWorkInputRef(raw string) (WorkInputRef, error) {
	ref, err := newV1Reference("Work input ref", raw)
	return WorkInputRef{v1Reference: ref}, err
}

func NewWorkOutputRef(raw string) (WorkOutputRef, error) {
	ref, err := newV1Reference("Work output ref", raw)
	return WorkOutputRef{v1Reference: ref}, err
}

func NewWorkResourceRef(raw string) (WorkResourceRef, error) {
	ref, err := newV1Reference("Work resource ref", raw)
	return WorkResourceRef{v1Reference: ref}, err
}

func NewAffectedReferentRef(raw string) (AffectedReferentRef, error) {
	ref, err := newV1Reference("affected referent ref", raw)
	return AffectedReferentRef{v1Reference: ref}, err
}

func NewStatePlaneRef(raw string) (StatePlaneRef, error) {
	ref, err := newV1Reference("StatePlane ref", raw)
	return StatePlaneRef{v1Reference: ref}, err
}

func NewStateRef(raw string) (StateRef, error) {
	ref, err := newV1Reference("state ref", raw)
	return StateRef{v1Reference: ref}, err
}

func NewDeltaPredicateRef(raw string) (DeltaPredicateRef, error) {
	ref, err := newV1Reference("delta-predicate ref", raw)
	return DeltaPredicateRef{v1Reference: ref}, err
}

func NewAuthorityResolutionRecordRef(raw string) (AuthorityResolutionRecordRef, error) {
	ref, err := newV1Reference("authority-resolution record ref", raw)
	return AuthorityResolutionRecordRef{v1Reference: ref}, err
}

func NewProfileDeclarationAdmissionRecordRef(raw string) (ProfileDeclarationAdmissionRecordRef, error) {
	ref, err := newV1Reference("profile-declaration admission-record ref", raw)
	return ProfileDeclarationAdmissionRecordRef{v1Reference: ref}, err
}

func NewSingleUseKey(raw string) (SingleUseKey, error) {
	ref, err := newV1Reference("single-use key", raw)
	return SingleUseKey{v1Reference: ref}, err
}

func NewSessionRef(raw string) (SessionRef, error) {
	ref, err := newV1Reference("session ref", raw)
	return SessionRef{v1Reference: ref}, err
}

type LedgerRevision struct {
	value uint64
}

func NewLedgerRevision(value uint64) LedgerRevision {
	return LedgerRevision{value: value}
}

func (revision LedgerRevision) Value() uint64 {
	return revision.value
}

func (revision LedgerRevision) Next() (LedgerRevision, error) {
	if revision.value >= math.MaxInt64 {
		return LedgerRevision{}, fmt.Errorf("ledger revision is exhausted")
	}
	return LedgerRevision{value: revision.value + 1}, nil
}

type MethodParameterBinding struct {
	name  string
	value string
}

func NewMethodParameterBinding(name string, value string) (MethodParameterBinding, error) {
	parsedName, err := requireText("method parameter name", name)
	if err != nil {
		return MethodParameterBinding{}, err
	}
	parsedValue, err := requireText("method parameter value", value)
	if err != nil {
		return MethodParameterBinding{}, err
	}
	return MethodParameterBinding{name: parsedName, value: parsedValue}, nil
}

func (binding MethodParameterBinding) Name() string {
	return binding.name
}

func (binding MethodParameterBinding) Value() string {
	return binding.value
}

func (binding MethodParameterBinding) valid() bool {
	_, nameErr := requireText("method parameter name", binding.name)
	_, valueErr := requireText("method parameter value", binding.value)
	return nameErr == nil && valueErr == nil
}

type MethodParameterBindings struct {
	values []MethodParameterBinding
}

func NewMethodParameterBindings(values []MethodParameterBinding) (MethodParameterBindings, error) {
	if len(values) == 0 {
		return MethodParameterBindings{}, fmt.Errorf("concrete method parameter bindings must not be empty")
	}
	canonical := append([]MethodParameterBinding{}, values...)
	slices.SortFunc(canonical, compareMethodParameterBindingV1)
	err := visitSliceV1(canonical, func(index int, binding MethodParameterBinding) error {
		if !binding.valid() {
			return fmt.Errorf("method parameter binding %d is invalid", index)
		}
		return nil
	})
	if err != nil {
		return MethodParameterBindings{}, err
	}
	err = visitAdjacentV1(canonical, func(previous MethodParameterBinding, current MethodParameterBinding) error {
		if previous.name == current.name {
			return fmt.Errorf("duplicate method parameter %q", current.name)
		}
		return nil
	})
	if err != nil {
		return MethodParameterBindings{}, err
	}
	return MethodParameterBindings{values: canonical}, nil
}

func (bindings MethodParameterBindings) Values() []MethodParameterBinding {
	return append([]MethodParameterBinding{}, bindings.values...)
}

func (bindings MethodParameterBindings) valid() bool {
	_, err := NewMethodParameterBindings(bindings.values)
	return err == nil
}

func (bindings MethodParameterBindings) ValueFor(name string) (string, bool) {
	index, found := slices.BinarySearchFunc(
		bindings.values,
		name,
		compareMethodParameterBindingNameV1,
	)
	if !found {
		return "", false
	}
	binding := bindings.values[index]
	return binding.value, true
}

func compareMethodParameterBindingV1(left MethodParameterBinding, right MethodParameterBinding) int {
	return cmp.Compare(left.name, right.name)
}

func compareMethodParameterBindingNameV1(left MethodParameterBinding, right string) int {
	return cmp.Compare(left.name, right)
}
