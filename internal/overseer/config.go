package overseer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigSchemaVersion = "overseer.config.v1"
	configFileName      = "config.yaml"
)

const (
	ReviewerAuto    = "auto"
	ReviewerManual  = "manual"
	ReviewerCommand = "command"
	ReviewerCodex   = "codex"
	ReviewerClaude  = "claude"
)

type Config struct {
	SchemaVersion        string `json:"schema_version" yaml:"schema_version"`
	Enabled              bool   `json:"enabled" yaml:"enabled"`
	Trigger              string `json:"trigger" yaml:"trigger"`
	Mode                 string `json:"mode" yaml:"mode"`
	LLMReview            string `json:"llm_review" yaml:"llm_review"`
	ReviewerAgent        string `json:"reviewer_agent" yaml:"reviewer_agent"`
	ReviewerCommand      string `json:"reviewer_command,omitempty" yaml:"reviewer_command,omitempty"`
	ReviewOnHook         bool   `json:"review_on_hook" yaml:"review_on_hook"`
	ReviewTimeoutSeconds int    `json:"review_timeout_seconds" yaml:"review_timeout_seconds"`
	AgentReminder        bool   `json:"agent_reminder" yaml:"agent_reminder"`
}

func DefaultConfig() Config {
	return Config{
		SchemaVersion:        ConfigSchemaVersion,
		Enabled:              true,
		Trigger:              "post_commit",
		Mode:                 "soft",
		LLMReview:            "off",
		ReviewerAgent:        "manual",
		ReviewerCommand:      "",
		ReviewOnHook:         false,
		ReviewTimeoutSeconds: 180,
		AgentReminder:        true,
	}
}

func ConfigForReviewer(agent string, command string, reviewOnHook bool, timeoutSeconds int) (Config, error) {
	config := DefaultConfig()
	agent = normalizeReviewerAgent(agent)
	command = strings.TrimSpace(command)

	if agent == "" {
		agent = ReviewerManual
	}
	if !reviewerAgentAllowed(agent) {
		return Config{}, fmt.Errorf("unknown reviewer agent %q", agent)
	}
	if agent == ReviewerCommand && command == "" {
		return Config{}, fmt.Errorf("reviewer_command is required for reviewer_agent=command")
	}

	config.ReviewerAgent = agent
	config.ReviewerCommand = command
	if config.ReviewerCommand == "" {
		config.ReviewerCommand = PresetReviewerCommand(agent)
	}
	if timeoutSeconds > 0 {
		config.ReviewTimeoutSeconds = timeoutSeconds
	}

	if agent != ReviewerManual || reviewOnHook {
		config.LLMReview = "on"
		config.ReviewOnHook = true
	}
	if reviewOnHook {
		config.ReviewOnHook = true
	}
	if config.ReviewOnHook && config.ReviewerCommand == "" {
		return Config{}, fmt.Errorf("reviewer_command is required when review_on_hook is enabled")
	}

	return normalizeConfig(config), nil
}

func PresetReviewerCommand(agent string) string {
	switch normalizeReviewerAgent(agent) {
	case ReviewerCodex:
		return "codex exec -c 'model_reasoning_effort=\"low\"' --sandbox read-only --ephemeral --output-schema \"$HAFT_OVERSEER_SCHEMA_FILE\" --output-last-message \"$HAFT_OVERSEER_RESULT_FILE\" - < \"$HAFT_OVERSEER_PROMPT_FILE\""
	case ReviewerClaude:
		return "claude -p --permission-mode dontAsk --json-schema \"$(cat \"$HAFT_OVERSEER_SCHEMA_FILE\")\" \"$(cat \"$HAFT_OVERSEER_PROMPT_FILE\")\" > \"$HAFT_OVERSEER_RESULT_FILE\""
	default:
		return ""
	}
}

func OverseerDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".haft", "overseer")
}

func ConfigPath(projectRoot string) string {
	return filepath.Join(OverseerDir(projectRoot), configFileName)
}

func SaveConfig(projectRoot string, config Config) error {
	config = normalizeConfig(config)
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal overseer config: %w", err)
	}

	path := ConfigPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create overseer config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write overseer config: %w", err)
	}
	return nil
}

func LoadConfig(projectRoot string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("read overseer config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse overseer config: %w", err)
	}
	return normalizeConfig(config), nil
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()
	if strings.TrimSpace(config.SchemaVersion) == "" {
		config.SchemaVersion = ConfigSchemaVersion
	}
	if strings.TrimSpace(config.Trigger) == "" {
		config.Trigger = "post_commit"
	}
	if strings.TrimSpace(config.Mode) == "" {
		config.Mode = "soft"
	}
	if strings.TrimSpace(config.LLMReview) == "" {
		config.LLMReview = "off"
	}
	if strings.TrimSpace(config.ReviewerAgent) == "" {
		config.ReviewerAgent = defaults.ReviewerAgent
	}
	config.ReviewerAgent = normalizeReviewerAgent(config.ReviewerAgent)
	if config.ReviewTimeoutSeconds <= 0 {
		config.ReviewTimeoutSeconds = defaults.ReviewTimeoutSeconds
	}
	return config
}

func normalizeReviewerAgent(agent string) string {
	return strings.ToLower(strings.TrimSpace(agent))
}

func reviewerAgentAllowed(agent string) bool {
	switch normalizeReviewerAgent(agent) {
	case ReviewerManual, ReviewerCommand, ReviewerCodex, ReviewerClaude:
		return true
	default:
		return false
	}
}

func ReviewEnabled(config Config) bool {
	mode := strings.ToLower(strings.TrimSpace(config.LLMReview))
	return config.Enabled && mode != "" && mode != "off" && mode != "false" && mode != "disabled"
}

func ReviewHookEnabled(config Config) bool {
	return ReviewEnabled(config) && config.ReviewOnHook
}

func ShouldReviewPacket(config Config, packet Packet) bool {
	return ReviewHookEnabled(config) && packet.Risk.LLMReview == "eligible"
}
