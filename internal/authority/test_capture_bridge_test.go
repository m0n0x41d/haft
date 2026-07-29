package authority

import (
	"strings"
	"testing"
)

func TestAuthorityFixtureCaptureRefusesProductionRuntime(t *testing.T) {
	err := requireAuthorityTestFixtureRuntime(false)
	if err == nil || !strings.Contains(err.Error(), "outside go test") {
		t.Fatalf("production fixture bridge error = %v", err)
	}
}
