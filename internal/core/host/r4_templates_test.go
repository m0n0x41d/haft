package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledSpecSkillExplainsInlineClaimExtensions(t *testing.T) {
	c := config(t)
	if _, err := Init(c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(c.Root, ".agents/skills/h-spec/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, instruction := range []string{
		`"text":"Bounded meaning","x-bound":5`,
		"Do not introduce an extra wrapper.",
		"A literal field named extra is ordinary",
		"Re-recall claims saved from an older API's extra envelope",
		"not other record\nor source-snapshot JSON envelopes",
	} {
		if !strings.Contains(string(raw), instruction) {
			t.Errorf("installed h-spec omits round-trip instruction: %q", instruction)
		}
	}
}
