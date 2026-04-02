// Package setup provides the interactive first-run setup.
// Stub — the BubbleTea setup TUI was removed. This provides
// a minimal implementation that reads environment variables.
package setup

import (
	"fmt"
	"os"

	"github.com/m0n0x41d/haft/internal/config"
)

// Result is returned after setup completes.
type Result struct {
	Config *config.Config
}

// Run executes the setup flow.
// Minimal stub: reads OPENAI_API_KEY from env and saves config.
func Run() (*Result, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set — set it in your environment")
	}

	cfg := &config.Config{
		Model: "gpt-4.1",
	}
	cfg.SetAuth("openai", config.ProviderAuth{APIKey: apiKey})

	if err := config.Save(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &Result{Config: cfg}, nil
}
