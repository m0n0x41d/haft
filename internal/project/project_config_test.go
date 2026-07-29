package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfigMissingFileUsesExplicitHDecide(t *testing.T) {
	config, err := LoadProjectConfig(t.TempDir())
	if err != nil {
		t.Fatalf("LoadProjectConfig: %v", err)
	}
	if config.SchemaVersion != currentProjectConfigSchema {
		t.Fatalf("schema_version = %d, want %d", config.SchemaVersion, currentProjectConfigSchema)
	}
	if config.EffectiveDecisionBindingMode() != DecisionBindingModeExplicitHDecide {
		t.Fatalf("decision binding mode = %q", config.EffectiveDecisionBindingMode())
	}
	if config.EffectiveProjectTypeEnvHeadSelectionMode() != ProjectTypeEnvHeadSelectionModeExplicitHDecide {
		t.Fatalf(
			"project TypeEnv head-selection mode = %q",
			config.EffectiveProjectTypeEnvHeadSelectionMode(),
		)
	}
	if config.EffectiveProfileDeclarationMode() != ProfileDeclarationModeExplicitHOnboard {
		t.Fatalf("profile declaration mode = %q", config.EffectiveProfileDeclarationMode())
	}
}

func TestParseProjectConfigMissingFieldsUsesV1Defaults(t *testing.T) {
	for _, body := range []string{"", "authority: {}\n"} {
		config, err := ParseProjectConfig([]byte(body))
		if err != nil {
			t.Fatalf("ParseProjectConfig(%q): %v", body, err)
		}
		if config.SchemaVersion != currentProjectConfigSchema {
			t.Fatalf("schema_version = %d, want %d", config.SchemaVersion, currentProjectConfigSchema)
		}
		if config.Authority.DecisionBindingMode != DecisionBindingModeExplicitHDecide {
			t.Fatalf("decision_binding_mode = %q", config.Authority.DecisionBindingMode)
		}
		if config.Authority.ProjectTypeEnvHeadSelectionMode != ProjectTypeEnvHeadSelectionModeExplicitHDecide {
			t.Fatalf(
				"project_typeenv_head_selection_mode = %q",
				config.Authority.ProjectTypeEnvHeadSelectionMode,
			)
		}
		if config.Authority.ProfileDeclarationMode != ProfileDeclarationModeExplicitHOnboard {
			t.Fatalf("profile_declaration_mode = %q", config.Authority.ProfileDeclarationMode)
		}
	}
}

func TestParseProjectConfigAcceptsStrictCLISpeechActOptIn(t *testing.T) {
	config, err := ParseProjectConfig([]byte(strings.Join([]string{
		"schema_version: 1",
		"authority:",
		"  decision_binding_mode: strict_cli_speech_act",
		"  project_typeenv_head_selection_mode: strict_cli_speech_act",
		"  profile_declaration_mode: strict_cli_speech_act",
		"",
	}, "\n")))
	if err != nil {
		t.Fatalf("ParseProjectConfig: %v", err)
	}
	if config.EffectiveDecisionBindingMode() != DecisionBindingModeStrictCLISpeechAct {
		t.Fatalf("decision binding mode = %q", config.EffectiveDecisionBindingMode())
	}
	if config.EffectiveProjectTypeEnvHeadSelectionMode() != ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct {
		t.Fatalf(
			"project TypeEnv head-selection mode = %q",
			config.EffectiveProjectTypeEnvHeadSelectionMode(),
		)
	}
	if config.EffectiveProfileDeclarationMode() != ProfileDeclarationModeStrictCLISpeechAct {
		t.Fatalf("profile declaration mode = %q", config.EffectiveProfileDeclarationMode())
	}
}

func TestParseProjectConfigRejectsUnknownSchemaAndMode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "schema",
			body: "schema_version: 2\n",
			want: []string{"schema_version 2", "expected 1"},
		},
		{
			name: "zero schema",
			body: "schema_version: 0\n",
			want: []string{"schema_version 0", "expected 1"},
		},
		{
			name: "null schema",
			body: "schema_version: null\n",
			want: []string{"schema_version", "null is not allowed"},
		},
		{
			name: "mode",
			body: "authority:\n  decision_binding_mode: magic\n",
			want: []string{
				"authority.decision_binding_mode",
				"explicit_h_decide",
				"strict_cli_speech_act",
			},
		},
		{
			name: "empty mode",
			body: "authority:\n  decision_binding_mode: \"\"\n",
			want: []string{"authority.decision_binding_mode", "empty value is not allowed"},
		},
		{
			name: "null mode",
			body: "authority:\n  decision_binding_mode: null\n",
			want: []string{"authority.decision_binding_mode", "null is not allowed"},
		},
		{
			name: "head selection mode",
			body: "authority:\n  project_typeenv_head_selection_mode: magic\n",
			want: []string{
				"authority.project_typeenv_head_selection_mode",
				"explicit_h_decide",
				"strict_cli_speech_act",
			},
		},
		{
			name: "empty head selection mode",
			body: "authority:\n  project_typeenv_head_selection_mode: \"\"\n",
			want: []string{
				"authority.project_typeenv_head_selection_mode",
				"empty value is not allowed",
			},
		},
		{
			name: "null head selection mode",
			body: "authority:\n  project_typeenv_head_selection_mode: null\n",
			want: []string{
				"authority.project_typeenv_head_selection_mode",
				"null is not allowed",
			},
		},
		{
			name: "profile declaration mode",
			body: "authority:\n  profile_declaration_mode: magic\n",
			want: []string{
				"authority.profile_declaration_mode",
				"explicit_h_onboard",
				"strict_cli_speech_act",
			},
		},
		{
			name: "empty profile declaration mode",
			body: "authority:\n  profile_declaration_mode: \"\"\n",
			want: []string{"authority.profile_declaration_mode", "empty value is not allowed"},
		},
		{
			name: "null profile declaration mode",
			body: "authority:\n  profile_declaration_mode: null\n",
			want: []string{"authority.profile_declaration_mode", "null is not allowed"},
		},
		{
			name: "null authority",
			body: "authority: null\n",
			want: []string{"authority", "null is not allowed"},
		},
		{
			name: "field",
			body: "authority:\n  decision_binding_mod: explicit_h_decide\n",
			want: []string{"decision_binding_mod", "not found"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseProjectConfig([]byte(test.body))
			if err == nil {
				t.Fatal("ParseProjectConfig accepted invalid config")
			}
			for _, want := range test.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q omitted %q", err, want)
				}
			}
		})
	}
}

func TestEnsureProjectConfigCreatesDefaultAndPreservesExistingBytes(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("mkdir .haft: %v", err)
	}

	if err := EnsureProjectConfig(haftDir); err != nil {
		t.Fatalf("EnsureProjectConfig fresh: %v", err)
	}
	path := ProjectConfigPath(haftDir)
	created, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fresh config: %v", err)
	}
	config, err := ParseProjectConfig(created)
	if err != nil {
		t.Fatalf("parse fresh config: %v", err)
	}
	if config.EffectiveDecisionBindingMode() != DecisionBindingModeExplicitHDecide {
		t.Fatalf("fresh mode = %q", config.EffectiveDecisionBindingMode())
	}
	if config.EffectiveProjectTypeEnvHeadSelectionMode() != ProjectTypeEnvHeadSelectionModeExplicitHDecide {
		t.Fatalf(
			"fresh project TypeEnv head-selection mode = %q",
			config.EffectiveProjectTypeEnvHeadSelectionMode(),
		)
	}
	if config.EffectiveProfileDeclarationMode() != ProfileDeclarationModeExplicitHOnboard {
		t.Fatalf("fresh profile declaration mode = %q", config.EffectiveProfileDeclarationMode())
	}

	strict := []byte(strings.Join([]string{
		"schema_version: 1",
		"authority:",
		"  decision_binding_mode: strict_cli_speech_act",
		"  project_typeenv_head_selection_mode: strict_cli_speech_act",
		"  profile_declaration_mode: strict_cli_speech_act",
		"",
	}, "\n"))
	if err := os.WriteFile(path, strict, 0o644); err != nil {
		t.Fatalf("write strict config: %v", err)
	}
	if err := EnsureProjectConfig(haftDir); err != nil {
		t.Fatalf("EnsureProjectConfig rerun: %v", err)
	}
	preserved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preserved config: %v", err)
	}
	if string(preserved) != string(strict) {
		t.Fatalf("rerun rewrote config:\n%s", preserved)
	}
}
