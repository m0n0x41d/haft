package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	projectConfigFile          = "config.yaml"
	currentProjectConfigSchema = 1
)

// DecisionBindingMode selects the project-local authority boundary used when
// an explicitly invoked h-decide records a DecisionRecord.
type DecisionBindingMode string

const (
	// DecisionBindingModeExplicitHDecide trusts the host's structural
	// explicit-only skill boundary. This is the compatibility and fresh-init
	// default: a person's explicit h-decide invocation is sufficient.
	DecisionBindingModeExplicitHDecide DecisionBindingMode = "explicit_h_decide"

	// DecisionBindingModeStrictCLISpeechAct adds the controlling-terminal
	// review and exact literal SpeechAct before a DecisionRecord is instituted.
	DecisionBindingModeStrictCLISpeechAct DecisionBindingMode = "strict_cli_speech_act"
)

// ProjectTypeEnvHeadSelectionMode selects the project-local human-gate
// sufficiency policy for the dedicated TypeEnv-head effect. It is deliberately
// distinct from DecisionBindingMode: one effect's policy must not silently
// govern another effect.
type ProjectTypeEnvHeadSelectionMode string

const (
	// ProjectTypeEnvHeadSelectionModeExplicitHDecide trusts the host's
	// manual-only h-decide invocation for the exact selection request. The
	// kernel does not claim that it independently observed a U.SpeechAct.
	ProjectTypeEnvHeadSelectionModeExplicitHDecide ProjectTypeEnvHeadSelectionMode = "explicit_h_decide"

	// ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct requires the separate
	// durable controlling-terminal SpeechAct adapter before selection.
	ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct ProjectTypeEnvHeadSelectionMode = "strict_cli_speech_act"
)

// ProfileDeclarationMode selects the project-local human-gate sufficiency
// policy for an initial project-profile declaration. It is deliberately
// separate from DecisionBindingMode and ProjectTypeEnvHeadSelectionMode:
// authority sufficiency for one institutional effect cannot authorize another.
type ProfileDeclarationMode string

const (
	// ProfileDeclarationModeExplicitHOnboard trusts an explicit operator request
	// to run h-onboard or declare this exact reviewed profile. The CLI performs
	// the typed Work/admission path without asking for a second acknowledgement.
	// Read-only inspect/propose calls never satisfy this boundary.
	ProfileDeclarationModeExplicitHOnboard ProfileDeclarationMode = "explicit_h_onboard"

	// ProfileDeclarationModeStrictCLISpeechAct reserves the stronger durable
	// terminal source policy. Until a native v3 issuer is installed, initial
	// profile declaration fails closed before any authority or Work write.
	ProfileDeclarationModeStrictCLISpeechAct ProfileDeclarationMode = "strict_cli_speech_act"
)

// ProjectConfig is the project-local Haft behavior configuration stored in
// .haft/config.yaml. Stable project identity remains in .haft/project.yaml.
type ProjectConfig struct {
	SchemaVersion int                    `yaml:"schema_version" json:"schema_version"`
	Authority     ProjectAuthorityConfig `yaml:"authority" json:"authority"`
}

// ProjectAuthorityConfig controls optional authority-strengthening policies.
type ProjectAuthorityConfig struct {
	DecisionBindingMode             DecisionBindingMode             `yaml:"decision_binding_mode,omitempty" json:"decision_binding_mode"`
	ProjectTypeEnvHeadSelectionMode ProjectTypeEnvHeadSelectionMode `yaml:"project_typeenv_head_selection_mode,omitempty" json:"project_typeenv_head_selection_mode"`
	ProfileDeclarationMode          ProfileDeclarationMode          `yaml:"profile_declaration_mode,omitempty" json:"profile_declaration_mode"`
}

// DefaultProjectConfig returns the effective policy for a fresh project and
// for older projects that have no .haft/config.yaml carrier.
func DefaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		SchemaVersion: currentProjectConfigSchema,
		Authority: ProjectAuthorityConfig{
			DecisionBindingMode:             DecisionBindingModeExplicitHDecide,
			ProjectTypeEnvHeadSelectionMode: ProjectTypeEnvHeadSelectionModeExplicitHDecide,
			ProfileDeclarationMode:          ProfileDeclarationModeExplicitHOnboard,
		},
	}
}

// EffectiveProfileDeclarationMode resolves the backward-compatible default
// for an omitted authority.profile_declaration_mode field.
func (config ProjectConfig) EffectiveProfileDeclarationMode() ProfileDeclarationMode {
	rawMode := string(config.Authority.ProfileDeclarationMode)
	trimmedMode := strings.TrimSpace(rawMode)
	mode := ProfileDeclarationMode(trimmedMode)
	if mode == "" {
		return ProfileDeclarationModeExplicitHOnboard
	}
	return mode
}

// EffectiveDecisionBindingMode resolves the backward-compatible default for
// an omitted authority.decision_binding_mode field.
func (config ProjectConfig) EffectiveDecisionBindingMode() DecisionBindingMode {
	rawMode := string(config.Authority.DecisionBindingMode)
	trimmedMode := strings.TrimSpace(rawMode)
	mode := DecisionBindingMode(trimmedMode)
	if mode == "" {
		return DecisionBindingModeExplicitHDecide
	}
	return mode
}

// EffectiveProjectTypeEnvHeadSelectionMode resolves the backward-compatible
// default for an omitted authority.project_typeenv_head_selection_mode field.
func (config ProjectConfig) EffectiveProjectTypeEnvHeadSelectionMode() ProjectTypeEnvHeadSelectionMode {
	rawMode := string(config.Authority.ProjectTypeEnvHeadSelectionMode)
	trimmedMode := strings.TrimSpace(rawMode)
	mode := ProjectTypeEnvHeadSelectionMode(trimmedMode)
	if mode == "" {
		return ProjectTypeEnvHeadSelectionModeExplicitHDecide
	}
	return mode
}

// ProjectConfigPath returns the project-local behavior-config carrier path.
func ProjectConfigPath(haftDir string) string {
	return filepath.Join(haftDir, projectConfigFile)
}

// LoadProjectConfig reads .haft/config.yaml from projectRoot. Missing files are
// old-project compatibility and resolve to DefaultProjectConfig.
func LoadProjectConfig(projectRoot string) (ProjectConfig, error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	path := ProjectConfigPath(haftDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultProjectConfig(), nil
		}
		return ProjectConfig{}, fmt.Errorf("read project config %s: %w", path, err)
	}

	config, err := ParseProjectConfig(data)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("parse project config %s: %w", path, err)
	}
	return config, nil
}

// ParseProjectConfig parses and validates one exact project-config document.
// Missing fields use v1 defaults; unknown fields, schemas, and enum values are
// rejected so a typo cannot silently change the authority boundary.
func ParseProjectConfig(data []byte) (ProjectConfig, error) {
	presence, err := inspectProjectConfigPresence(data)
	if err != nil {
		return ProjectConfig{}, err
	}

	config := ProjectConfig{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	decodeErr := decoder.Decode(&config)
	if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		return ProjectConfig{}, fmt.Errorf("decode YAML (allowed top-level fields: schema_version, authority): %w", decodeErr)
	}

	if decodeErr == nil {
		extra := any(nil)
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err != nil {
				return ProjectConfig{}, fmt.Errorf("decode trailing YAML document: %w", err)
			}
			return ProjectConfig{}, fmt.Errorf("exactly one YAML document is allowed")
		}
	}

	if !presence.schemaVersion {
		config.SchemaVersion = currentProjectConfigSchema
	}
	if config.SchemaVersion != currentProjectConfigSchema {
		return ProjectConfig{}, fmt.Errorf(
			"schema_version %d is unsupported; expected %d",
			config.SchemaVersion,
			currentProjectConfigSchema,
		)
	}

	mode := config.Authority.DecisionBindingMode
	if !presence.decisionBindingMode {
		mode = DecisionBindingModeExplicitHDecide
	}
	if err := validateDecisionBindingMode(mode); err != nil {
		return ProjectConfig{}, err
	}
	config.Authority.DecisionBindingMode = mode
	headMode := config.Authority.ProjectTypeEnvHeadSelectionMode
	if !presence.projectTypeEnvHeadSelectionMode {
		headMode = ProjectTypeEnvHeadSelectionModeExplicitHDecide
	}
	if err := validateProjectTypeEnvHeadSelectionMode(headMode); err != nil {
		return ProjectConfig{}, err
	}
	config.Authority.ProjectTypeEnvHeadSelectionMode = headMode
	profileMode := config.Authority.ProfileDeclarationMode
	if !presence.profileDeclarationMode {
		profileMode = ProfileDeclarationModeExplicitHOnboard
	}
	if err := validateProfileDeclarationMode(profileMode); err != nil {
		return ProjectConfig{}, err
	}
	config.Authority.ProfileDeclarationMode = profileMode
	return config, nil
}

type projectConfigPresence struct {
	schemaVersion                   bool
	decisionBindingMode             bool
	projectTypeEnvHeadSelectionMode bool
	profileDeclarationMode          bool
}

func inspectProjectConfigPresence(data []byte) (projectConfigPresence, error) {
	document := yaml.Node{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&document)
	if errors.Is(err, io.EOF) {
		return projectConfigPresence{}, nil
	}
	if err != nil {
		return projectConfigPresence{}, fmt.Errorf("decode YAML structure: %w", err)
	}
	if len(document.Content) != 1 {
		return projectConfigPresence{}, fmt.Errorf("project config must contain one YAML mapping")
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return projectConfigPresence{}, fmt.Errorf("project config must be a YAML mapping")
	}

	schemaNode, schemaPresent := yamlMappingValue(root, "schema_version")
	if schemaPresent && yamlNodeIsNull(schemaNode) {
		return projectConfigPresence{}, fmt.Errorf("schema_version must be integer %d; null is not allowed", currentProjectConfigSchema)
	}

	authorityNode, authorityPresent := yamlMappingValue(root, "authority")
	if authorityPresent && yamlNodeIsNull(authorityNode) {
		return projectConfigPresence{}, fmt.Errorf("authority must be a mapping; null is not allowed")
	}
	if authorityPresent && authorityNode.Kind != yaml.MappingNode {
		return projectConfigPresence{}, fmt.Errorf("authority must be a YAML mapping")
	}

	modeNode, modePresent := yamlMappingValue(authorityNode, "decision_binding_mode")
	if modePresent && yamlNodeIsNull(modeNode) {
		return projectConfigPresence{}, fmt.Errorf("authority.decision_binding_mode must be %q or %q; null is not allowed", DecisionBindingModeExplicitHDecide, DecisionBindingModeStrictCLISpeechAct)
	}
	if modePresent && strings.TrimSpace(modeNode.Value) == "" {
		return projectConfigPresence{}, fmt.Errorf("authority.decision_binding_mode must be %q or %q; empty value is not allowed", DecisionBindingModeExplicitHDecide, DecisionBindingModeStrictCLISpeechAct)
	}

	headModeNode, headModePresent := yamlMappingValue(
		authorityNode,
		"project_typeenv_head_selection_mode",
	)
	if headModePresent && yamlNodeIsNull(headModeNode) {
		return projectConfigPresence{}, fmt.Errorf(
			"authority.project_typeenv_head_selection_mode must be %q or %q; null is not allowed",
			ProjectTypeEnvHeadSelectionModeExplicitHDecide,
			ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct,
		)
	}
	if headModePresent && strings.TrimSpace(headModeNode.Value) == "" {
		return projectConfigPresence{}, fmt.Errorf(
			"authority.project_typeenv_head_selection_mode must be %q or %q; empty value is not allowed",
			ProjectTypeEnvHeadSelectionModeExplicitHDecide,
			ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct,
		)
	}
	profileModeNode, profileModePresent := yamlMappingValue(
		authorityNode,
		"profile_declaration_mode",
	)
	if profileModePresent && yamlNodeIsNull(profileModeNode) {
		return projectConfigPresence{}, fmt.Errorf(
			"authority.profile_declaration_mode must be %q or %q; null is not allowed",
			ProfileDeclarationModeExplicitHOnboard,
			ProfileDeclarationModeStrictCLISpeechAct,
		)
	}
	if profileModePresent && strings.TrimSpace(profileModeNode.Value) == "" {
		return projectConfigPresence{}, fmt.Errorf(
			"authority.profile_declaration_mode must be %q or %q; empty value is not allowed",
			ProfileDeclarationModeExplicitHOnboard,
			ProfileDeclarationModeStrictCLISpeechAct,
		)
	}

	return projectConfigPresence{
		schemaVersion:                   schemaPresent,
		decisionBindingMode:             modePresent,
		projectTypeEnvHeadSelectionMode: headModePresent,
		profileDeclarationMode:          profileModePresent,
	}, nil
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], true
		}
	}
	return nil, false
}

func yamlNodeIsNull(node *yaml.Node) bool {
	return node != nil && node.Tag == "!!null"
}

// EnsureProjectConfig creates the fresh-init default carrier exactly once.
// Existing bytes are never rewritten by haft init, including an operator's
// strict opt-in.
func EnsureProjectConfig(haftDir string) error {
	path := ProjectConfigPath(haftDir)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create project config %s: %w", path, err)
	}

	_, writeErr := file.WriteString(ExampleProjectConfigYAML())
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write project config %s: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close project config %s: %w", path, closeErr)
	}
	return nil
}

// ExampleProjectConfigYAML is the canonical fresh-init carrier. The default
// keeps the explicit h-decide interaction as the only required human action;
// strict CLI SpeechAct capture is an explicit project opt-in.
func ExampleProjectConfigYAML() string {
	return strings.Join([]string{
		"# Haft project behavior configuration",
		"schema_version: 1",
		"authority:",
		"  # DecisionRecord: explicit_h_decide (default) | strict_cli_speech_act (opt-in)",
		"  decision_binding_mode: explicit_h_decide",
		"  # Project TypeEnv head: explicit_h_decide (default) | strict_cli_speech_act (opt-in)",
		"  project_typeenv_head_selection_mode: explicit_h_decide",
		"  # Project profile: explicit_h_onboard (default) | strict_cli_speech_act (reserved; fails closed without native v3 strict authority)",
		"  profile_declaration_mode: explicit_h_onboard",
		"",
	}, "\n")
}

func validateProfileDeclarationMode(mode ProfileDeclarationMode) error {
	switch mode {
	case ProfileDeclarationModeExplicitHOnboard,
		ProfileDeclarationModeStrictCLISpeechAct:
		return nil
	default:
		return fmt.Errorf(
			"authority.profile_declaration_mode %q is unsupported; expected %q or %q",
			mode,
			ProfileDeclarationModeExplicitHOnboard,
			ProfileDeclarationModeStrictCLISpeechAct,
		)
	}
}

func validateDecisionBindingMode(mode DecisionBindingMode) error {
	switch mode {
	case DecisionBindingModeExplicitHDecide,
		DecisionBindingModeStrictCLISpeechAct:
		return nil
	default:
		return fmt.Errorf(
			"authority.decision_binding_mode %q is unsupported; expected %q or %q",
			mode,
			DecisionBindingModeExplicitHDecide,
			DecisionBindingModeStrictCLISpeechAct,
		)
	}
}

func validateProjectTypeEnvHeadSelectionMode(
	mode ProjectTypeEnvHeadSelectionMode,
) error {
	switch mode {
	case ProjectTypeEnvHeadSelectionModeExplicitHDecide,
		ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct:
		return nil
	default:
		return fmt.Errorf(
			"authority.project_typeenv_head_selection_mode %q is unsupported; expected %q or %q",
			mode,
			ProjectTypeEnvHeadSelectionModeExplicitHDecide,
			ProjectTypeEnvHeadSelectionModeStrictCLISpeechAct,
		)
	}
}
