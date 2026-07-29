package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicLegacyHaftSkillRecognition(
	t *testing.T,
) {
	root := t.TempDir()
	path := filepath.Join(
		root,
		"h-commission",
		"SKILL.md",
	)
	output, err := initplanning.NewRenderedOutput(
		path,
		initplanning.ComponentSkills,
		embeddedHCommissionSkill,
		0o644,
	)
	if err != nil {
		t.Fatalf("NewRenderedOutput: %v", err)
	}
	markerLegacy := []byte(`---
name: h-commission
description: Old Haft commission skill.
---

<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:1111111111111111111111111111111111111111111111111111111111111111 -->

# h-commission — Old Haft commission
`)
	signatureLegacy := []byte(`---
name: h-commission
description: Old Haft commission skill.
---

# h-commission — Old Haft commission

Use mcp__haft__haft_query for Haft project state.
`)
	foreign := []byte(`---
name: h-commission
description: Private commission helper.
---

# Private commission helper

This file belongs to the operator.
`)
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "contract marker",
			content:  markerLegacy,
			expected: true,
		},
		{
			name:     "pre-marker Haft signature",
			content:  signatureLegacy,
			expected: true,
		},
		{
			name:     "unrelated reserved-name file",
			content:  foreign,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := isPublicLegacyHaftSkill(
				output,
				test.content,
			)
			if actual != test.expected {
				t.Fatalf(
					"isPublicLegacyHaftSkill = %t, want %t",
					actual,
					test.expected,
				)
			}
		})
	}
}

func TestPublicInitReplacesLegacyHaftSkillForClaudeAndCodex(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	homeRoot, err := filepath.EvalSymlinks(homeRoot)
	if err != nil {
		t.Fatalf("resolve physical home root: %v", err)
	}
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initClaude = true
	initCodex = true
	t.Setenv("HOME", homeRoot)

	skillPath := filepath.Join(
		homeRoot,
		".claude",
		"skills",
		"h-commission",
		"SKILL.md",
	)
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create legacy skill directory: %v", err)
	}
	staleMarker := []byte(
		"<!-- haft-contract-source: kernel_interface_catalog source_digest=sha256:" +
			strings.Repeat("1", 64) +
			" -->",
	)
	legacy := publicHaftSkillContractSourcePattern.ReplaceAll(
		embeddedHCommissionSkill,
		staleMarker,
	)
	legacy = append(
		legacy,
		[]byte("\nLegacy-only Haft instruction.\n")...,
	)
	if bytes.Equal(legacy, embeddedHCommissionSkill) {
		t.Fatal("legacy skill fixture equals current bundled skill")
	}
	if err := os.WriteFile(skillPath, legacy, 0o644); err != nil {
		t.Fatalf("write legacy Haft skill: %v", err)
	}

	output := &bytes.Buffer{}
	cmd := newPublicInitTestCommand()
	cmd.SetOut(output)
	var claude bool
	var codex bool
	cmd.Flags().BoolVar(&claude, "claude", false, "")
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("claude", "true"); err != nil {
		t.Fatalf("set Claude flag: %v", err)
	}
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(cmd, nil); err != nil {
		t.Fatalf("replace legacy Haft skill: %v", err)
	}
	updated, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read updated Haft skill: %v", err)
	}
	if !bytes.Equal(updated, embeddedHCommissionSkill) {
		t.Fatalf(
			"legacy skill was not replaced by current bundle",
		)
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".codex", "config.toml"),
		filepath.Join(
			homeRoot,
			".haft",
			"host-installations",
			"claude.user.json",
		),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected applied path %s: %v", path, err)
		}
	}
	if !strings.Contains(
		output.String(),
		"Haft initialization complete",
	) {
		t.Fatalf("legacy replacement output = %q", output.String())
	}
}

func TestPublicInitReplacesEveryRecognizedCodexSkillFromExactCurrentBundle(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	physicalHomeRoot, err := filepath.EvalSymlinks(homeRoot)
	if err != nil {
		t.Fatalf("resolve physical home root: %v", err)
	}
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initCodex = true
	t.Setenv("HOME", physicalHomeRoot)

	codexAdapter, err := currentSkillAdapterForPlatform("codex")
	if err != nil {
		t.Fatalf("build Codex skill adapter: %v", err)
	}
	expectedSkills := make(map[string][]byte, len(allSkills))
	for _, skill := range allSkills {
		rendered, renderErr := codexAdapter.rewrite.Apply(skill.Content)
		if renderErr != nil {
			t.Fatalf("render Codex skill %s: %v", skill.Name, renderErr)
		}
		expectedSkills[skill.Name] = rendered
		skillPath := filepath.Join(
			physicalHomeRoot,
			".agents",
			"skills",
			skill.Name,
			"SKILL.md",
		)
		if mkdirErr := os.MkdirAll(filepath.Dir(skillPath), 0o755); mkdirErr != nil {
			t.Fatalf("create stale Codex skill %s: %v", skill.Name, mkdirErr)
		}
		stale := append([]byte{}, rendered...)
		stale = append(stale, []byte("\nLegacy-only Haft instruction.\n")...)
		if writeErr := os.WriteFile(skillPath, stale, 0o644); writeErr != nil {
			t.Fatalf("write stale Codex skill %s: %v", skill.Name, writeErr)
		}
	}

	command := newPublicInitTestCommand()
	command.SetOut(&bytes.Buffer{})
	var codex bool
	command.Flags().BoolVar(&codex, "codex", false, "")
	if err := command.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	if err := runPublicInit(command, nil); err != nil {
		t.Fatalf("replace recognized Codex skills: %v", err)
	}

	for _, skill := range allSkills {
		skillRoot := filepath.Join(
			physicalHomeRoot,
			".agents",
			"skills",
			skill.Name,
		)
		actual, readErr := os.ReadFile(filepath.Join(skillRoot, "SKILL.md"))
		if readErr != nil {
			t.Fatalf("read replaced Codex skill %s: %v", skill.Name, readErr)
		}
		if !bytes.Equal(actual, expectedSkills[skill.Name]) {
			t.Errorf(
				"recognized Codex skill %s was not replaced byte-for-byte",
				skill.Name,
			)
		}
		policyValue := strconv.FormatBool(skill.AllowImplicit)
		wantPolicyText := "policy:\n  allow_implicit_invocation: " +
			policyValue +
			"\n"
		wantPolicy := []byte(wantPolicyText)
		actualPolicy, policyErr := os.ReadFile(
			filepath.Join(skillRoot, "agents", "openai.yaml"),
		)
		if policyErr != nil {
			t.Fatalf("read Codex policy for %s: %v", skill.Name, policyErr)
		}
		if !bytes.Equal(actualPolicy, wantPolicy) {
			t.Errorf(
				"Codex policy for %s = %q, want %q",
				skill.Name,
				actualPolicy,
				wantPolicy,
			)
		}
	}
}

func TestPublicInitPreservesUnrelatedReservedNameSkill(
	t *testing.T,
) {
	projectRoot := t.TempDir()
	homeRoot := t.TempDir()
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initClaude = true
	initCodex = true
	t.Setenv("HOME", homeRoot)

	skillPath := filepath.Join(
		homeRoot,
		".claude",
		"skills",
		"h-commission",
		"SKILL.md",
	)
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("create foreign skill directory: %v", err)
	}
	foreign := []byte(`---
name: h-commission
description: Private commission helper.
---

# Private commission helper

This file belongs to the operator.
`)
	if err := os.WriteFile(skillPath, foreign, 0o644); err != nil {
		t.Fatalf("write foreign skill: %v", err)
	}

	cmd := newPublicInitTestCommand()
	var claude bool
	var codex bool
	cmd.Flags().BoolVar(&claude, "claude", false, "")
	cmd.Flags().BoolVar(&codex, "codex", false, "")
	if err := cmd.Flags().Set("claude", "true"); err != nil {
		t.Fatalf("set Claude flag: %v", err)
	}
	if err := cmd.Flags().Set("codex", "true"); err != nil {
		t.Fatalf("set Codex flag: %v", err)
	}
	err := runPublicInit(cmd, nil)
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"unowned path collides",
		) {
		t.Fatalf("foreign skill error = %v", err)
	}
	preserved, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read preserved foreign skill: %v", err)
	}
	if !bytes.Equal(preserved, foreign) {
		t.Fatal("foreign reserved-name skill was changed")
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".haft"),
		filepath.Join(projectRoot, ".codex"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf(
				"blocked init wrote %s: %v",
				path,
				err,
			)
		}
	}
}
