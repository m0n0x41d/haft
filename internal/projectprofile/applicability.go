package projectprofile

// Capability names a profile-gated Haft capability. The legacy project-profile
// carrier can orient a caller, but it cannot establish binding applicability.
type Capability string

const SoftwareSystemSpecMigration Capability = "software_system_spec_migration"

type MissingBasis string

const (
	// MissingAuthoritativeProfile is returned for Auto until a canonical profile
	// resolver backed by the durable admission ledger is wired in.
	MissingAuthoritativeProfile MissingBasis = "missing_authoritative_profile"
	// MissingCanonicalDurableProfileAdmission is returned for every valid legacy
	// Declared carrier. A receipt-shaped YAML record is not the final-v1 durable
	// admission record and cannot grant binding applicability.
	MissingCanonicalDurableProfileAdmission MissingBasis = "missing_canonical_durable_profile_admission"
	MissingValidConfiguredProfile           MissingBasis = "missing_valid_configured_profile"
)

type MissingBasisSet struct {
	values []MissingBasis
}

func newMissingBasisSet(first MissingBasis) MissingBasisSet {
	return MissingBasisSet{values: []MissingBasis{first}}
}

func (set MissingBasisSet) Values() []MissingBasis {
	return append([]MissingBasis{}, set.values...)
}

type Applicability interface {
	applicabilityVariant()
	Capability() Capability
}

type Underdetermined interface {
	Applicability
	MissingBasis() MissingBasisSet
	underdeterminedVariant()
}

type underdetermined struct {
	capability   Capability
	missingBasis MissingBasisSet
}

func (underdetermined) applicabilityVariant()   {}
func (underdetermined) underdeterminedVariant() {}

func (result underdetermined) Capability() Capability {
	return result.capability
}

func (result underdetermined) MissingBasis() MissingBasisSet {
	return result.missingBasis
}

// ResolveSoftwareSystemSpecMigration is deliberately orientation-only. The
// legacy Auto/Declared algebra and YAML receipt records predate the final-v1
// durable admission contract, so neither variant can produce Required or
// NotApplicable. A later canonical resolver may consume ConfiguredProjectProfileV1
// after P0PA lands; this function must not anticipate that authority boundary.
func ResolveSoftwareSystemSpecMigration(profile ConfiguredProjectProfile) Applicability {
	switch value := profile.(type) {
	case Auto:
		return unresolvedSoftwareSystemSpecMigration(MissingAuthoritativeProfile)
	case Declared:
		if _, err := NewDeclared(value.scopes, value.declaration); err != nil {
			return unresolvedSoftwareSystemSpecMigration(MissingValidConfiguredProfile)
		}
		return unresolvedSoftwareSystemSpecMigration(MissingCanonicalDurableProfileAdmission)
	default:
		return unresolvedSoftwareSystemSpecMigration(MissingValidConfiguredProfile)
	}
}

func unresolvedSoftwareSystemSpecMigration(missing MissingBasis) Underdetermined {
	return underdetermined{
		capability:   SoftwareSystemSpecMigration,
		missingBasis: newMissingBasisSet(missing),
	}
}
