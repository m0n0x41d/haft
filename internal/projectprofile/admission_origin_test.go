package projectprofile

import "testing"

func TestProfileAdmissionOriginIsClosed(t *testing.T) {
	for _, raw := range []string{
		"detector_default",
		"host_routed_operator_request",
		"explicit_operator",
		"legacy_unknown",
	} {
		origin, ok := ParseProfileAdmissionOrigin(raw)
		if !ok || string(origin) != raw {
			t.Fatalf("origin %q = %q, %v", raw, origin, ok)
		}
	}
	if _, ok := ParseProfileAdmissionOrigin("operator_guess"); ok {
		t.Fatal("open origin value was accepted")
	}
}
