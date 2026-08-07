package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/overseer"
)

const publicOverseerManifestSchema = "haft.overseer-installation-manifest/v1"

type publicOverseerPlan struct {
	effects           []publicExactFileEffect
	hookSkippedReason string
	recovery          []string
}

func (plan publicOverseerPlan) Effects() []publicExactFileEffect {
	return slices.Clone(plan.effects)
}

func (plan publicOverseerPlan) HookSkippedReason() string {
	return plan.hookSkippedReason
}

type publicOverseerPreview struct {
	Effects           []publicExactFileEffectPreview
	HookSkippedReason string
}

type publicExactFileEffectPreview struct {
	Path           string
	Kind           publicExactFileEffectKind
	ExpectedDigest string
	ExpectedMode   uint32
	RenderedDigest string
	RenderedMode   uint32
}

func previewPublicExactFileEffects(
	effects []publicExactFileEffect,
) []publicExactFileEffectPreview {
	preview := make(
		[]publicExactFileEffectPreview,
		len(effects),
	)
	for index, effect := range effects {
		preview[index] = publicExactFileEffectPreview{
			Path:           effect.path,
			Kind:           effect.kind,
			ExpectedDigest: effect.expectedDigest,
			ExpectedMode:   uint32(effect.expectedMode.Perm()),
			RenderedDigest: effect.renderedDigest,
			RenderedMode:   uint32(effect.mode.Perm()),
		}
	}
	return preview
}

func compilePublicOverseerPlan(
	request publicInitRequest,
) (publicOverseerPlan, error) {
	if request.overseer.kind != publicOverseerConfigure {
		return publicOverseerPlan{}, fmt.Errorf(
			"public overseer publication is not selected",
		)
	}
	config, err := buildOverseerConfigForProject(
		request.projectRoot,
		overseerSetupOptions{
			reviewer: request.overseer.reviewer,
			command:  request.overseer.command,
			reviewOnHook: request.overseer.hook ==
				publicOverseerHookEnabled,
			timeout: request.overseer.timeout,
			hosts:   publicOverseerHostOptions(request.hostBindings),
		},
	)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	configBytes, err := overseer.RenderConfig(config)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	configEffect, err := planPublicExactFile(
		overseer.ConfigPath(request.projectRoot),
		configBytes,
		0o644,
	)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	effects := []publicExactFileEffect{configEffect}
	hookPath, supported, err := overseer.PostCommitHookPath(
		request.projectRoot,
	)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	hookSkippedReason := ""
	if supported {
		hookEffect, hookErr := planPublicOverseerHook(hookPath)
		if hookErr != nil {
			return publicOverseerPlan{}, hookErr
		}
		effects = append(effects, hookEffect)
	} else {
		hookSkippedReason = "no .git directory"
	}
	manifestBytes, err := buildPublicOverseerManifest(
		request,
		effects,
		hookSkippedReason,
	)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	manifestPath := filepath.Join(
		request.projectRoot,
		".haft",
		"ancillary-installations",
		"overseer.json",
	)
	manifestEffect, err := planPublicExactFile(
		manifestPath,
		manifestBytes,
		0o644,
	)
	if err != nil {
		return publicOverseerPlan{}, err
	}
	effects = append(effects, manifestEffect)
	return publicOverseerPlan{
		effects:           effects,
		hookSkippedReason: hookSkippedReason,
		recovery:          publicOverseerRecovery(request.overseer),
	}, nil
}

func publicOverseerHostOptions(
	bindings []publicHostBinding,
) initHostOptions {
	options := initHostOptions{}
	for _, binding := range bindings {
		switch binding.host {
		case initplanning.HostClaude:
			options.claude = true
		case initplanning.HostCodex:
			options.codex = true
		case initplanning.HostAir:
			options.air = true
		}
	}
	return options
}

func planPublicOverseerHook(
	path string,
) (publicExactFileEffect, error) {
	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		existing = nil
	} else if err != nil {
		return publicExactFileEffect{}, fmt.Errorf(
			"read planned overseer hook %s: %w",
			path,
			err,
		)
	}
	rendered := overseer.RenderPostCommitHook(
		string(existing),
		"haft",
	)
	return planPublicExactFile(path, []byte(rendered), 0o755)
}

type publicOverseerManifestOutput struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

type publicOverseerManifestWire struct {
	Schema            string                         `json:"schema"`
	ProjectRoot       string                         `json:"project_root"`
	ProjectID         string                         `json:"project_id"`
	Reviewer          string                         `json:"reviewer"`
	ReviewOnHook      bool                           `json:"review_on_hook"`
	HookSkippedReason string                         `json:"hook_skipped_reason,omitempty"`
	Outputs           []publicOverseerManifestOutput `json:"outputs"`
}

func buildPublicOverseerManifest(
	request publicInitRequest,
	effects []publicExactFileEffect,
	hookSkippedReason string,
) ([]byte, error) {
	outputs := make(
		[]publicOverseerManifestOutput,
		len(effects),
	)
	for index, effect := range effects {
		outputs[index] = publicOverseerManifestOutput{
			Path:   effect.path,
			Digest: effect.renderedDigest,
			Mode:   uint32(effect.mode.Perm()),
		}
	}
	wire := publicOverseerManifestWire{
		Schema:            publicOverseerManifestSchema,
		ProjectRoot:       request.projectRoot,
		ProjectID:         request.projectID,
		Reviewer:          request.overseer.reviewer,
		ReviewOnHook:      request.overseer.hook == publicOverseerHookEnabled,
		HookSkippedReason: hookSkippedReason,
		Outputs:           outputs,
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf(
			"encode overseer installation manifest: %w",
			err,
		)
	}
	return encoded, nil
}

func publicOverseerRecovery(
	selection publicOverseerSelection,
) []string {
	recovery := []string{
		"haft",
		"init",
		"--overseer",
		"--overseer-reviewer",
		selection.reviewer,
		"--overseer-review-timeout",
		strconv.Itoa(selection.timeout),
	}
	if selection.command != "" {
		recovery = append(
			recovery,
			"--overseer-reviewer-command",
			selection.command,
		)
	}
	if selection.hook == publicOverseerHookEnabled {
		recovery = append(
			recovery,
			"--overseer-review-on-hook",
		)
	}
	return recovery
}

func applyPublicOverseerPlan(
	ctx context.Context,
	plan publicOverseerPlan,
) (publicExactFileReceipt, error) {
	if len(plan.effects) == 0 {
		return publicExactFileReceipt{},
			fmt.Errorf("public overseer plan is invalid")
	}
	return applyPublicExactFileEffects(
		ctx,
		plan.effects,
		plan.recovery,
	)
}
