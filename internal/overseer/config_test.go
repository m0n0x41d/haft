package overseer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadConfigRoundTrip(t *testing.T) {
	root := t.TempDir()
	config := DefaultConfig()

	if err := SaveConfig(root, config); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	loaded, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if loaded.SchemaVersion != ConfigSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", loaded.SchemaVersion, ConfigSchemaVersion)
	}
	if !loaded.Enabled {
		t.Fatalf("enabled = false, want true")
	}
	if loaded.Trigger != "post_commit" {
		t.Fatalf("trigger = %q, want post_commit", loaded.Trigger)
	}
	if loaded.Mode != "soft" {
		t.Fatalf("mode = %q, want soft", loaded.Mode)
	}
	if loaded.LLMReview != "off" {
		t.Fatalf("llm_review = %q, want off", loaded.LLMReview)
	}
	if loaded.ReviewerAgent != "manual" {
		t.Fatalf("reviewer_agent = %q, want manual", loaded.ReviewerAgent)
	}
	if loaded.ReviewOnHook {
		t.Fatalf("review_on_hook = true, want false by default")
	}
	if loaded.ReviewTimeoutSeconds != 180 {
		t.Fatalf("review_timeout_seconds = %d, want 180", loaded.ReviewTimeoutSeconds)
	}

	if _, err := os.Stat(filepath.Join(root, ".haft", "overseer", "config.yaml")); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

func TestReviewConfigPredicates(t *testing.T) {
	config := DefaultConfig()
	if ReviewEnabled(config) {
		t.Fatalf("default config should not enable review")
	}

	config.LLMReview = "on"
	config.ReviewOnHook = true
	if !ReviewHookEnabled(config) {
		t.Fatalf("review hook should be enabled")
	}
	if !ShouldReviewPacket(config, Packet{Risk: Risk{LLMReview: "eligible"}}) {
		t.Fatalf("eligible packet should trigger hook review")
	}
	if ShouldReviewPacket(config, Packet{Risk: Risk{LLMReview: "off"}}) {
		t.Fatalf("low-risk packet should not trigger hook review")
	}
}

func TestConfigForReviewerCodexPresetEnablesHookReview(t *testing.T) {
	config, err := ConfigForReviewer(ReviewerCodex, "", true, 45)
	if err != nil {
		t.Fatalf("ConfigForReviewer returned error: %v", err)
	}
	if config.LLMReview != "on" {
		t.Fatalf("llm_review = %q, want on", config.LLMReview)
	}
	if config.ReviewerAgent != ReviewerCodex {
		t.Fatalf("reviewer_agent = %q, want codex", config.ReviewerAgent)
	}
	if !config.ReviewOnHook {
		t.Fatalf("review_on_hook = false, want true")
	}
	if config.ReviewTimeoutSeconds != 45 {
		t.Fatalf("timeout = %d, want 45", config.ReviewTimeoutSeconds)
	}
	for _, want := range []string{"codex exec", "model_reasoning_effort", "HAFT_OVERSEER_SCHEMA_FILE", "HAFT_OVERSEER_RESULT_FILE"} {
		if !strings.Contains(config.ReviewerCommand, want) {
			t.Fatalf("codex preset missing %q:\n%s", want, config.ReviewerCommand)
		}
	}
	if strings.Contains(config.ReviewerCommand, "--ask-for-approval") {
		t.Fatalf("codex preset uses an unsupported codex-cli flag:\n%s", config.ReviewerCommand)
	}
}

func TestConfigForReviewerCommandRequiresCommand(t *testing.T) {
	_, err := ConfigForReviewer(ReviewerCommand, "", true, 0)
	if err == nil {
		t.Fatalf("command reviewer without command should fail")
	}
}
