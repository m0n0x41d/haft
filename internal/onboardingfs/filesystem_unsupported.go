//go:build !darwin && !linux

package onboardingfs

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/onboarding"
)

func Read(projectRoot string) (ReadResult, error) {
	return nil, unsupportedPlatformError(projectRoot)
}

func Install(
	projectRoot string,
	proposed onboarding.MemoryDeferral,
) (InstallResult, error) {
	return nil, unsupportedPlatformError(projectRoot)
}

func Reopen(
	projectRoot string,
	expected onboarding.MemoryDeferral,
) (ReopenResult, error) {
	return nil, unsupportedPlatformError(projectRoot)
}

func unsupportedPlatformError(projectRoot string) error {
	return fmt.Errorf(
		"identity-held memory deferral effects are unsupported on this platform for %s",
		projectRoot,
	)
}
