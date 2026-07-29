package profileauthority

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
)

// Closure is the pure phase-two result. Persistence is intentionally absent
// until schema v43 can store the A.2.8-correct Permission and exact four-ref
// basis without a legacy projection.
type Closure struct {
	permission Permission
	effect     InstitutedEffect
	basis      FourRefBasis
}

func NewClosure(
	prepared PreparedAuthorization,
	source authority.RecordedSpeechActSource,
) (Closure, error) {
	permission, err := NewPermission(prepared, source)
	if err != nil {
		return Closure{}, err
	}
	effect, err := NewInstitutedEffect(permission)
	if err != nil {
		return Closure{}, err
	}
	basis, err := NewFourRefBasis(prepared, permission)
	if err != nil {
		return Closure{}, err
	}
	closure := Closure{
		permission: permission,
		effect:     effect,
		basis:      basis,
	}
	if !closure.valid() {
		return Closure{}, fmt.Errorf("profile authority closure is inconsistent")
	}
	return closure, nil
}

func (closure Closure) valid() bool {
	return closure.permission.valid() && closure.effect.valid() && closure.basis.valid()
}

func (closure Closure) Permission() (Permission, bool) {
	return closure.permission, closure.valid()
}

func (closure Closure) Effect() (InstitutedEffect, bool) {
	return closure.effect, closure.valid()
}

func (closure Closure) Basis() (FourRefBasis, bool) {
	return closure.basis, closure.valid()
}
