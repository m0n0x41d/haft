package projectmemory

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofruntime"
)

// NewCarrierFamilyMembershipEvaluatorRegistry exposes only family engines
// created by the four package-owned constructors.
func NewCarrierFamilyMembershipEvaluatorRegistry(
	engines []CarrierFamilyMembershipAdmissionEngine,
) (memberofruntime.Registry, error) {
	registrations := make([]memberofruntime.Registration, 0, len(engines))
	for index, engine := range engines {
		if err := engine.validate(); err != nil {
			return memberofruntime.Registry{}, fmt.Errorf(
				"carrier-family engine %d: %w",
				index,
				err,
			)
		}
		registration, err := memberofruntime.NewRegistration(
			engine.rule,
			engine.identity,
			engine,
		)
		if err != nil {
			return memberofruntime.Registry{}, err
		}
		registrations = append(registrations, registration)
	}
	return memberofruntime.NewRegistry(registrations)
}
