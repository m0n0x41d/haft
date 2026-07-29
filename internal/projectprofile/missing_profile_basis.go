package projectprofile

// MissingProfileBasis is shared vocabulary used by the final-v1
// classification result. It is not a legacy onboarding authority or result
// constructor.
type MissingProfileBasis string

const (
	MissingObservedProjectBasis MissingProfileBasis = "observed_project_basis"
	MissingStableScopeIdentity  MissingProfileBasis = "stable_scope_identity"
	MissingClassificationBasis  MissingProfileBasis = "classification_basis"
)
