//go:build !darwin && !linux

package projecttypeenvreviewcarrier

import "fmt"

// Install is unavailable when identity-held openat effects are unsupported.
func Install(projectRoot string, proposed Carrier) (InstallationResult, error) {
	return nil, unsupportedPlatformError(projectRoot)
}

// Replace is unavailable when identity-held openat effects are unsupported.
func Replace(
	projectRoot string,
	expected Digest,
	proposed Carrier,
) (InstallationResult, error) {
	return nil, unsupportedPlatformError(projectRoot)
}

// Read is unavailable when identity-held openat effects are unsupported.
func Read(projectRoot string) (Carrier, error) {
	return Carrier{}, unsupportedPlatformError(projectRoot)
}

func unsupportedPlatformError(projectRoot string) error {
	return fmt.Errorf(
		"identity-held Genesis review carrier effects are unsupported on this platform for %s",
		projectRoot,
	)
}
