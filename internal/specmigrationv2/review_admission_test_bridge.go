package specmigrationv2

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

// CaptureVerifiedMigrationReviewForTestFixture lets cross-package CLI tests
// traverse the real sealed capture core without exposing the prepared manual
// source. testing.TB and the authority runtime guard make the capability
// unavailable outside go test.
func CaptureVerifiedMigrationReviewForTestFixture(
	t testing.TB,
	prepared PreparedMigrationReviewAdmission,
	startedAt time.Time,
	exactUtteranceObservedAt time.Time,
	endedAt time.Time,
) (authority.VerifiedSpeechActSource, error) {
	t.Helper()
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return authority.VerifiedSpeechActSource{}, err
	}
	return authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared.state.manualSource,
		startedAt,
		exactUtteranceObservedAt,
		endedAt,
	)
}
