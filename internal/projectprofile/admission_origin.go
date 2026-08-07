package projectprofile

// ProfileAdmissionOrigin is the closed provenance class for the authority
// path that produced the current canonical project profile.
type ProfileAdmissionOrigin string

const (
	ProfileAdmissionOriginDetectorDefault           ProfileAdmissionOrigin = "detector_default"
	ProfileAdmissionOriginHostRoutedOperatorRequest ProfileAdmissionOrigin = "host_routed_operator_request"
	ProfileAdmissionOriginExplicitOperator          ProfileAdmissionOrigin = "explicit_operator"
	ProfileAdmissionOriginLegacyUnknown             ProfileAdmissionOrigin = "legacy_unknown"
)

func ParseProfileAdmissionOrigin(raw string) (ProfileAdmissionOrigin, bool) {
	origin := ProfileAdmissionOrigin(raw)
	valid := origin == ProfileAdmissionOriginDetectorDefault ||
		origin == ProfileAdmissionOriginHostRoutedOperatorRequest ||
		origin == ProfileAdmissionOriginExplicitOperator ||
		origin == ProfileAdmissionOriginLegacyUnknown
	return origin, valid
}
