package artifact

import "testing"

func TestIsArtifactID(t *testing.T) {
	tests := map[string]bool{
		"dec-20260711-exact":       true,
		"  prob-20260711-a1b2  ":   true,
		"note-202607-short":        true,
		"decision search terms":    false,
		"dec-short":                false,
		"symbol.Name":              false,
		"dec-20260711-exact extra": false,
	}

	for value, want := range tests {
		if got := IsArtifactID(value); got != want {
			t.Errorf("IsArtifactID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsCanonicalArtifactID(t *testing.T) {
	tests := map[string]bool{
		"dec-20260711-exact":       true,
		"  prob-20260711-a1b2  ":   true,
		"note-202607-short":        true,
		"sol-20260711-a1b2":        true,
		"wc-20260711-a1b2":         true,
		"mpull-20260711-a1b2":      true,
		"evid-20260711-a1b2":       true,
		"ref-20260711-a1b2":        true,
		"cache-20260711-key":       false,
		"decision search terms":    false,
		"dec-short":                false,
		"dec-20260711-exact extra": false,
	}

	for value, want := range tests {
		if got := IsCanonicalArtifactID(value); got != want {
			t.Errorf("IsCanonicalArtifactID(%q) = %v, want %v", value, got, want)
		}
	}
}
