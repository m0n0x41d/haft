package fpf

import (
	"strings"
	"testing"
)

// TestPhaseHintBuildsFromEmbeddedPatterns verifies that PhaseHint returns
// content derived from embedded pattern files, including newly added Core
// patterns. This fails fast if renaming a pattern ID in markdown breaks the
// hint wiring.
func TestPhaseHintBuildsFromEmbeddedPatterns(t *testing.T) {
	cases := []struct {
		phase  string
		mustID []string
	}{
		{"frame", []string{"FRAME-01", "FRAME-02", "FRAME-03", "FRAME-05", "FRAME-08", "CHR-01"}},
		{"characterize", []string{"CHR-01", "CHR-02", "CHR-04", "CHR-09", "CHR-10"}},
		{"explore", []string{"EXP-01", "EXP-02", "EXP-04", "EXP-05", "EXP-07"}},
		{"compare", []string{"CMP-01", "CMP-02", "CMP-03", "CMP-04", "CMP-06"}},
		{"decide", []string{"DEC-01", "DEC-04", "DEC-05", "DEC-06", "DEC-08"}},
		{"verify", []string{"VER-01", "VER-02", "VER-03", "VER-07", "X-WLNK"}},
	}

	for _, c := range cases {
		hint := PhaseHint(c.phase)
		if hint == "" {
			t.Errorf("phase %q: empty hint", c.phase)
			continue
		}
		for _, id := range c.mustID {
			if !strings.Contains(hint, id) {
				t.Errorf("phase %q: hint missing pattern %q\nhint:\n%s", c.phase, id, hint)
			}
		}
		if !strings.Contains(hint, "haft_query") {
			t.Errorf("phase %q: hint missing retrieval guidance", c.phase)
		}
	}
}

func TestPhaseHintUnknownPhaseReturnsEmpty(t *testing.T) {
	if PhaseHint("nonsense") != "" {
		t.Error("expected empty hint for unknown phase")
	}
}
