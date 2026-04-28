package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

// handleHaftSpecSection dispatches haft_spec_section MCP tool calls. The
// server-bound project root (parent of haftDir) is the default; callers
// may override via "project_root" arg.
func handleHaftSpecSection(_ context.Context, _ *artifact.Store, haftDir string, args map[string]any) (string, error) {
	action := strings.TrimSpace(stringArg(args, "action"))
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	switch action {
	case "next_step":
		projectRoot := strings.TrimSpace(stringArg(args, "project_root"))
		if projectRoot == "" {
			projectRoot = filepath.Dir(haftDir)
		}

		specSet, err := project.LoadProjectSpecificationSet(projectRoot)
		if err != nil {
			return "", err
		}

		intent := specflow.NextStep(specflow.DeriveState(specSet))

		payload, err := json.Marshal(intent)
		if err != nil {
			return "", fmt.Errorf("marshal intent: %w", err)
		}

		return string(payload), nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}
