package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillAirUsesProjectSkillsDir(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	displayPath, err := installSkill("air", false, projectRoot)
	if err != nil {
		t.Fatalf("installSkill returned error: %v", err)
	}

	wantDir := filepath.Join(projectRoot, "skills", "q-reason")
	if displayPath != wantDir {
		t.Fatalf("display path = %q, want %q", displayPath, wantDir)
	}

	skillPath := filepath.Join(wantDir, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read installed skill: %v", err)
	}

	if string(content) != string(embeddedQReasonSkill) {
		t.Fatalf("installed skill content mismatch")
	}
}
