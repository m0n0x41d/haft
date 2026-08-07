package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/overseer"
)

func TestConfigureOverseerWithConfigWritesConfigAndHook(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	git(t, root, "init")

	_, result, err := configureOverseerWithConfig(
		root,
		"haft",
		func() (overseer.Config, error) {
			return overseer.DefaultConfig(), nil
		},
	)
	if err != nil {
		t.Fatalf("configureOverseerWithConfig returned error: %v", err)
	}
	if !result.Installed || result.Skipped {
		t.Fatalf("hook result = %+v, want installed", result)
	}

	configPath := filepath.Join(root, ".haft", "overseer", "config.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read overseer config: %v", err)
	}
	if !strings.Contains(string(config), "llm_review: \"off\"") &&
		!strings.Contains(string(config), "llm_review: off") {
		t.Fatalf("config should keep LLM review off by default:\n%s", config)
	}

	hookPath := filepath.Join(root, ".git", "hooks", "post-commit")
	hook, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read post-commit hook: %v", err)
	}
	if !strings.Contains(
		string(hook),
		"haft overseer hook --commit HEAD --async || true",
	) {
		t.Fatalf("hook missing soft overseer e2e command:\n%s", hook)
	}
}

func TestConfigureOverseerWithConfigUsesCodexReviewerPreset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	git(t, root, "init")

	_, result, err := configureOverseerWithConfig(
		root,
		"haft",
		func() (overseer.Config, error) {
			return overseer.ConfigForReviewer(
				overseer.ReviewerCodex,
				"",
				true,
				45,
			)
		},
	)
	if err != nil {
		t.Fatalf("configureOverseerWithConfig returned error: %v", err)
	}
	if !result.Installed || result.Skipped {
		t.Fatalf("hook result = %+v, want installed", result)
	}

	configPath := filepath.Join(root, ".haft", "overseer", "config.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read overseer config: %v", err)
	}
	for _, want := range []string{
		"llm_review: \"on\"",
		"reviewer_agent: codex",
		"review_on_hook: true",
		"review_timeout_seconds: 45",
		"codex exec",
		"HAFT_OVERSEER_SCHEMA_FILE",
	} {
		if !strings.Contains(string(config), want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
}

func TestBuildOverseerConfigForProjectAutoUsesCodexHost(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	config, err := buildOverseerConfigForProject(
		root,
		overseerSetupOptions{
			reviewer: overseer.ReviewerAuto,
			hosts:    initHostOptions{codex: true},
			timeout:  45,
		},
	)
	if err != nil {
		t.Fatalf("buildOverseerConfigForProject returned error: %v", err)
	}
	if config.ReviewerAgent != overseer.ReviewerCodex {
		t.Fatalf("reviewer_agent = %q, want codex", config.ReviewerAgent)
	}
	if config.LLMReview != "on" || !config.ReviewOnHook {
		t.Fatalf("codex auto should enable hook review: %+v", config)
	}
}

func TestBuildOverseerConfigForProjectAutoDetectsCodexCarrier(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		configPath,
		[]byte("[mcp_servers.haft]\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	config, err := buildOverseerConfigForProject(
		root,
		overseerSetupOptions{
			reviewer: overseer.ReviewerAuto,
			timeout:  180,
		},
	)
	if err != nil {
		t.Fatalf("buildOverseerConfigForProject returned error: %v", err)
	}
	if config.ReviewerAgent != overseer.ReviewerCodex {
		t.Fatalf("reviewer_agent = %q, want codex", config.ReviewerAgent)
	}
}
