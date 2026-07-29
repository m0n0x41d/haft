package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"gopkg.in/yaml.v3"
)

const publicHermesManifestSchema = "haft.hermes-installation-manifest/v1"

type publicHermesPlan struct {
	home       string
	configPath string
	skillsRoot string
	effects    []publicExactFileEffect
	recovery   []string
}

func (plan publicHermesPlan) Effects() []publicExactFileEffect {
	return slices.Clone(plan.effects)
}

type publicHermesPreview struct {
	Home       string
	ConfigPath string
	SkillsRoot string
	Effects    []publicExactFileEffectPreview
}

func compilePublicHermesPlan(
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
) (publicHermesPlan, error) {
	if request.hermes.kind != publicHermesConfigure {
		return publicHermesPlan{}, fmt.Errorf(
			"public Hermes publication is not selected",
		)
	}
	home, err := resolvePublicHermesHome(
		request.hermes,
		runtime.userHomeRoot,
	)
	if err != nil {
		return publicHermesPlan{}, err
	}
	skillsRoot := publicHermesSkillsRoot(
		request.projectRoot,
		runtime.userHomeRoot,
	)
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		return publicHermesPlan{}, err
	}
	adapter, err := currentSkillAdapterForPlatform("hermes")
	if err != nil {
		return publicHermesPlan{}, err
	}
	projection, err := adapter.renderer.Render(bundle, skillsRoot)
	if err != nil {
		return publicHermesPlan{}, fmt.Errorf(
			"render independent Hermes skills: %w",
			err,
		)
	}
	effects := make(
		[]publicExactFileEffect,
		0,
		len(projection.Outputs())+2,
	)
	for _, output := range projection.Outputs() {
		effect, effectErr := planPublicExactFile(
			output.Path(),
			output.Content(),
			output.Mode(),
		)
		if effectErr != nil {
			return publicHermesPlan{}, effectErr
		}
		effects = append(effects, effect)
	}
	configPath := filepath.Join(home, "config.yaml")
	configBytes, err := renderPublicHermesConfig(
		configPath,
		request.projectRoot,
		request.projectID,
		skillsRoot,
	)
	if err != nil {
		return publicHermesPlan{}, err
	}
	configEffect, err := planPublicExactFile(
		configPath,
		configBytes,
		0o644,
	)
	if err != nil {
		return publicHermesPlan{}, err
	}
	effects = append(effects, configEffect)
	manifestBytes, err := buildPublicHermesManifest(
		request,
		home,
		configPath,
		skillsRoot,
		bundle,
		effects,
	)
	if err != nil {
		return publicHermesPlan{}, err
	}
	manifestEffect, err := planPublicExactFile(
		filepath.Join(
			request.projectRoot,
			".haft",
			"ancillary-installations",
			"hermes.json",
		),
		manifestBytes,
		0o644,
	)
	if err != nil {
		return publicHermesPlan{}, err
	}
	effects = append(effects, manifestEffect)
	return publicHermesPlan{
		home:       home,
		configPath: configPath,
		skillsRoot: skillsRoot,
		effects:    effects,
		recovery:   publicHermesRecovery(request.hermes),
	}, nil
}

func resolvePublicHermesHome(
	options publicHermesOptions,
	userHomeRoot string,
) (string, error) {
	profile, err := cleanHermesProfile(options.profileInput)
	if err != nil {
		return "", err
	}
	explicitHome := strings.TrimSpace(options.homeInput)
	rawHome := explicitHome
	if rawHome == "" {
		rawHome = strings.TrimSpace(os.Getenv("HERMES_HOME"))
	}
	if rawHome == "" {
		rawHome = filepath.Join(userHomeRoot, ".hermes")
	}
	home, err := expandHermesPath(rawHome)
	if err != nil {
		return "", err
	}
	if profile == "" || explicitHome != "" {
		return filepath.Clean(home), nil
	}
	return filepath.Join(home, "profiles", profile), nil
}

func publicHermesSkillsRoot(
	projectRoot string,
	userHomeRoot string,
) string {
	if isHaftSourceRoot(projectRoot) {
		return filepath.Join(
			projectRoot,
			filepath.FromSlash(hermesSkillsRelDir),
		)
	}
	return filepath.Join(
		userHomeRoot,
		".haft",
		"hermes",
		"skills",
	)
}

func renderPublicHermesConfig(
	configPath string,
	projectRoot string,
	projectID string,
	skillsRoot string,
) ([]byte, error) {
	settings, err := readHermesConfig(configPath)
	if err != nil {
		return nil, err
	}
	withPublicHermesMCP(settings, projectRoot, projectID)
	withHermesExternalDir(settings, skillsRoot)
	encoded, err := yaml.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf(
			"encode planned Hermes config %s: %w",
			configPath,
			err,
		)
	}
	return encoded, nil
}

func withPublicHermesMCP(
	settings map[string]any,
	projectRoot string,
	projectID string,
) {
	mcpServers := hermesMapField(settings, "mcp_servers")
	mcpServers["haft"] = map[string]any{
		"command": resolveHermesCommand("haft"),
		"args":    []string{"serve"},
		"env": currentBoundProjectEnvironment(
			projectRoot,
			projectID,
		),
		"enabled": true,
	}
	settings["mcp_servers"] = mcpServers
}

type publicHermesManifestOutput struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

type publicHermesManifestWire struct {
	Schema              string                       `json:"schema"`
	ProjectRoot         string                       `json:"project_root"`
	ProjectID           string                       `json:"project_id"`
	Home                string                       `json:"home"`
	ConfigPath          string                       `json:"config_path"`
	SkillsRoot          string                       `json:"skills_root"`
	SkillBundleDigest   string                       `json:"skill_bundle_digest"`
	KernelCatalogDigest string                       `json:"kernel_catalog_digest"`
	Outputs             []publicHermesManifestOutput `json:"outputs"`
}

func buildPublicHermesManifest(
	request publicInitRequest,
	home string,
	configPath string,
	skillsRoot string,
	bundle initplanning.SkillSourceBundle,
	effects []publicExactFileEffect,
) ([]byte, error) {
	outputs := make(
		[]publicHermesManifestOutput,
		len(effects),
	)
	for index, effect := range effects {
		outputs[index] = publicHermesManifestOutput{
			Path:   effect.path,
			Digest: effect.renderedDigest,
			Mode:   uint32(effect.mode.Perm()),
		}
	}
	wire := publicHermesManifestWire{
		Schema:              publicHermesManifestSchema,
		ProjectRoot:         request.projectRoot,
		ProjectID:           request.projectID,
		Home:                home,
		ConfigPath:          configPath,
		SkillsRoot:          skillsRoot,
		SkillBundleDigest:   bundle.Digest(),
		KernelCatalogDigest: bundle.KernelCatalogDigest(),
		Outputs:             outputs,
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf(
			"encode Hermes installation manifest: %w",
			err,
		)
	}
	return encoded, nil
}

func publicHermesRecovery(
	options publicHermesOptions,
) []string {
	recovery := []string{"haft", "init", "--hermes"}
	if options.homeInput != "" {
		recovery = append(
			recovery,
			"--hermes-home",
			options.homeInput,
		)
	}
	if options.profileInput != "" {
		recovery = append(
			recovery,
			"--profile",
			options.profileInput,
		)
	}
	return recovery
}

func applyPublicHermesPlan(
	ctx context.Context,
	plan publicHermesPlan,
) (publicExactFileReceipt, error) {
	if plan.home == "" ||
		plan.configPath == "" ||
		plan.skillsRoot == "" ||
		len(plan.effects) == 0 {
		return publicExactFileReceipt{},
			fmt.Errorf("public Hermes plan is invalid")
	}
	return applyPublicExactFileEffects(
		ctx,
		plan.effects,
		plan.recovery,
	)
}
