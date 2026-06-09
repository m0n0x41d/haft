package artifact

import (
	"strings"
	"testing"
)

func TestKindMethodRunMetadata(t *testing.T) {
	if !KindMethodRun.IsValid() {
		t.Fatal("KindMethodRun should be a valid artifact kind")
	}
	if got := KindMethodRun.IDPrefix(); got != "mpull" {
		t.Fatalf("IDPrefix = %q, want mpull", got)
	}
	if got := KindMethodRun.Dir(); got != "method-runs" {
		t.Fatalf("Dir = %q, want method-runs", got)
	}
	if got := KindMethodRun.UserFacingLabel(); got != "method run" {
		t.Fatalf("UserFacingLabel = %q, want method run", got)
	}
	if !strings.Contains(strings.Join(ValidKindNames(), " "), string(KindMethodRun)) {
		t.Fatalf("ValidKindNames missing %s", KindMethodRun)
	}
}
