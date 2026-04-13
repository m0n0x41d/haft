package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillAirUsesProjectSkillsDir(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	displayPath, err := installSkill("air", false, projectRoot)
	if err != nil {
		t.Fatalf("installSkill returned error: %v", err)
	}

	wantDir := filepath.Join(projectRoot, "skills", "h-reason")
	if displayPath != wantDir {
		t.Fatalf("display path = %q, want %q", displayPath, wantDir)
	}

	skillPath := filepath.Join(wantDir, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read installed skill: %v", err)
	}

	if string(content) != string(embeddedHReasonSkill) {
		t.Fatalf("installed skill content mismatch")
	}
}

func TestEmbeddedHReasonSkill_Path5RequiresAutonomousMode(t *testing.T) {
	content := string(embeddedHReasonSkill)

	required := []string{
		`ONLY when autonomous mode is already enabled for the session`,
		`If autonomous mode is OFF, phrases like "figure out the best approach and do it" or "fix everything" are NOT enough`,
	}

	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Fatalf("embedded skill missing %q", want)
		}
	}
}
