package cli

import (
	"strings"
	"testing"
)

func TestSpecLifecycleHelpNamesAuthorityBoundaries(t *testing.T) {
	cases := map[string]string{
		"spec sync":            specSyncCmd.Long,
		"spec export":          specExportCmd.Long,
		"spec apply-change":    specApplyChangeCmd.Long,
		"spec classify-change": specClassifyChangeCmd.Long,
	}

	for name, help := range cases {
		normalized := strings.ToLower(strings.Join(strings.Fields(help), " "))
		for _, want := range []string{"evidence", "gate", "claim truth", "global truth", "prose authority"} {
			if !strings.Contains(normalized, want) {
				t.Fatalf("%s help missing %q:\n%s", name, want, help)
			}
		}
	}
}
