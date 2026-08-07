package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type publicAgentSkillsEffectKind string

const (
	publicAgentSkillsCreate   publicAgentSkillsEffectKind = "create"
	publicAgentSkillsPreserve publicAgentSkillsEffectKind = "preserve"
	publicAgentSkillsReplace  publicAgentSkillsEffectKind = "replace_legacy"
)

type publicAgentSkillsEffect struct {
	kind           publicAgentSkillsEffectKind
	output         initplanning.RenderedOutput
	expectedDigest string
	expectedMode   fs.FileMode
}

func (effect publicAgentSkillsEffect) Kind() publicAgentSkillsEffectKind {
	return effect.kind
}

func (effect publicAgentSkillsEffect) Path() string {
	return effect.output.Path()
}

func (effect publicAgentSkillsEffect) RenderedDigest() string {
	return effect.output.Digest()
}

type publicAgentSkillsPlan struct {
	scope                initplanning.InstallScope
	root                 string
	effects              []publicAgentSkillsEffect
	manifestPath         string
	manifestBytes        []byte
	manifestDigest       string
	manifestPrecondition publicAgentManifestPrecondition
	recovery             []string
}

func (plan publicAgentSkillsPlan) Scope() initplanning.InstallScope {
	return plan.scope
}

func (plan publicAgentSkillsPlan) Root() string {
	return plan.root
}

func (plan publicAgentSkillsPlan) Effects() []publicAgentSkillsEffect {
	return slices.Clone(plan.effects)
}

func compilePublicAgentSkillsPlan(
	request publicInitRequest,
	userHomeRoot string,
	bundle initplanning.SkillSourceBundle,
) (publicAgentSkillsPlan, error) {
	scope, root, err := publicAgentSkillsLocation(
		request,
		userHomeRoot,
	)
	if err != nil {
		return publicAgentSkillsPlan{}, err
	}
	adapter, err := currentSkillAdapterForPlatform("codex")
	if err != nil {
		return publicAgentSkillsPlan{}, err
	}
	projection, err := adapter.renderer.Render(bundle, root)
	if err != nil {
		return publicAgentSkillsPlan{}, fmt.Errorf(
			"render independent agent skill publication: %w",
			err,
		)
	}
	outputs := projection.Outputs()
	effects := make(
		[]publicAgentSkillsEffect,
		0,
		len(outputs),
	)
	for _, output := range outputs {
		if !publicPathInsideRoot(output.Path(), root) {
			return publicAgentSkillsPlan{}, fmt.Errorf(
				"agent skill output %s is outside exact target root %s",
				output.Path(),
				root,
			)
		}
		effect, effectErr := observePublicAgentSkillsEffect(output)
		if effectErr != nil {
			return publicAgentSkillsPlan{}, effectErr
		}
		effects = append(effects, effect)
	}
	manifestPath := publicAgentSkillsManifestPath(
		scope,
		request.projectRoot,
		userHomeRoot,
	)
	manifestBytes, manifestDigest, err :=
		buildPublicAgentSkillsManifest(
			request,
			scope,
			root,
			projection.Edition(),
			bundle,
			outputs,
		)
	if err != nil {
		return publicAgentSkillsPlan{}, err
	}
	manifestPrecondition, err :=
		observePublicAgentManifest(manifestPath)
	if err != nil {
		return publicAgentSkillsPlan{}, err
	}
	return publicAgentSkillsPlan{
		scope:                scope,
		root:                 root,
		effects:              effects,
		manifestPath:         manifestPath,
		manifestBytes:        manifestBytes,
		manifestDigest:       manifestDigest,
		manifestPrecondition: manifestPrecondition,
		recovery:             publicAgentSkillsRecovery(request),
	}, nil
}

func publicAgentSkillsLocation(
	request publicInitRequest,
	userHomeRoot string,
) (
	initplanning.InstallScope,
	string,
	error,
) {
	switch request.agentSkills {
	case publicAgentSkillsProject:
		return initplanning.ScopeProject,
			filepath.Join(
				request.projectRoot,
				".agents",
				"skills",
			),
			nil
	case publicAgentSkillsUser:
		return initplanning.ScopeUser,
			filepath.Join(
				userHomeRoot,
				".agents",
				"skills",
			),
			nil
	default:
		return "", "", fmt.Errorf(
			"independent agent skill publication is not selected",
		)
	}
}

func publicPathInsideRoot(path string, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." ||
		(relative != ".." &&
			!strings.HasPrefix(
				relative,
				".."+string(filepath.Separator),
			))
}

func observePublicAgentSkillsEffect(
	output initplanning.RenderedOutput,
) (publicAgentSkillsEffect, error) {
	info, err := os.Lstat(output.Path())
	if os.IsNotExist(err) {
		return publicAgentSkillsEffect{
			kind:   publicAgentSkillsCreate,
			output: output,
		}, nil
	}
	if err != nil {
		return publicAgentSkillsEffect{}, fmt.Errorf(
			"inspect agent skill path %s: %w",
			output.Path(),
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return publicAgentSkillsEffect{}, fmt.Errorf(
			"agent skill path %s is not a regular file",
			output.Path(),
		)
	}
	content, err := os.ReadFile(output.Path())
	if err != nil {
		return publicAgentSkillsEffect{}, fmt.Errorf(
			"read agent skill path %s: %w",
			output.Path(),
			err,
		)
	}
	digest := fmt.Sprintf(
		"sha256:%x",
		sha256.Sum256(content),
	)
	kind := publicAgentSkillsReplace
	if digest == output.Digest() &&
		info.Mode().Perm() == output.Mode().Perm() {
		kind = publicAgentSkillsPreserve
	}
	return publicAgentSkillsEffect{
		kind:           kind,
		output:         output,
		expectedDigest: digest,
		expectedMode:   info.Mode().Perm(),
	}, nil
}

type publicAgentSkillsReceipt struct {
	changedPaths int
	manifestPath string
	completed    []string
	failed       string
	untouched    []string
	retry        []string
	recovery     []string
}

func (receipt publicAgentSkillsReceipt) ChangedPaths() int {
	return receipt.changedPaths
}

func (receipt publicAgentSkillsReceipt) ManifestPath() string {
	return receipt.manifestPath
}

func (receipt publicAgentSkillsReceipt) Completed() []string {
	return slices.Clone(receipt.completed)
}

func (receipt publicAgentSkillsReceipt) Failed() string {
	return receipt.failed
}

func (receipt publicAgentSkillsReceipt) Untouched() []string {
	return slices.Clone(receipt.untouched)
}

func (receipt publicAgentSkillsReceipt) Retry() []string {
	return slices.Clone(receipt.retry)
}

func (receipt publicAgentSkillsReceipt) Recovery() []string {
	return slices.Clone(receipt.recovery)
}

func applyPublicAgentSkillsPlan(
	ctx context.Context,
	plan publicAgentSkillsPlan,
) (publicAgentSkillsReceipt, error) {
	if ctx == nil {
		return publicAgentSkillsReceipt{},
			fmt.Errorf("agent skill publication context is required")
	}
	if plan.root == "" || len(plan.effects) == 0 {
		return publicAgentSkillsReceipt{},
			fmt.Errorf("agent skill publication plan is invalid")
	}
	for _, effect := range plan.effects {
		if err := verifyPublicAgentSkillsPrecondition(effect); err != nil {
			return publicAgentSkillsReceipt{
				manifestPath: plan.manifestPath,
				untouched:    publicAgentSkillsPlanPaths(plan),
				retry:        publicAgentSkillsPlanPaths(plan),
				recovery:     slices.Clone(plan.recovery),
			}, err
		}
	}
	if err := verifyPublicAgentManifestPrecondition(
		plan.manifestPath,
		plan.manifestPrecondition,
	); err != nil {
		return publicAgentSkillsReceipt{
			manifestPath: plan.manifestPath,
			untouched:    publicAgentSkillsPlanPaths(plan),
			retry:        publicAgentSkillsPlanPaths(plan),
			recovery:     slices.Clone(plan.recovery),
		}, err
	}
	changed := 0
	completed := make([]string, 0, len(plan.effects)+1)
	for index, effect := range plan.effects {
		if effect.kind == publicAgentSkillsPreserve {
			completed = append(completed, effect.Path())
			continue
		}
		if err := writePublicAgentSkillOutput(effect.output); err != nil {
			pending := publicAgentSkillsEffectPaths(
				plan.effects[index:],
			)
			pending = append(pending, plan.manifestPath)
			return publicAgentSkillsReceipt{
				changedPaths: changed,
				manifestPath: plan.manifestPath,
				completed:    slices.Clone(completed),
				failed:       effect.Path(),
				untouched: append(
					publicAgentSkillsEffectPaths(
						plan.effects[index+1:],
					),
					plan.manifestPath,
				),
				retry:    pending,
				recovery: slices.Clone(plan.recovery),
			}, err
		}
		changed++
		completed = append(completed, effect.Path())
	}
	if plan.manifestPrecondition.digest != plan.manifestDigest {
		if err := writePublicAgentManifest(plan); err != nil {
			return publicAgentSkillsReceipt{
				changedPaths: changed,
				manifestPath: plan.manifestPath,
				completed:    slices.Clone(completed),
				failed:       plan.manifestPath,
				retry:        []string{plan.manifestPath},
				recovery:     slices.Clone(plan.recovery),
			}, err
		}
	}
	completed = append(completed, plan.manifestPath)
	return publicAgentSkillsReceipt{
		changedPaths: changed,
		manifestPath: plan.manifestPath,
		completed:    completed,
		recovery:     slices.Clone(plan.recovery),
	}, nil
}

func publicAgentSkillsEffectPaths(
	effects []publicAgentSkillsEffect,
) []string {
	paths := make([]string, len(effects))
	for index, effect := range effects {
		paths[index] = effect.Path()
	}
	return paths
}

func publicAgentSkillsPlanPaths(
	plan publicAgentSkillsPlan,
) []string {
	paths := publicAgentSkillsEffectPaths(plan.effects)
	return append(paths, plan.manifestPath)
}

func publicAgentSkillsRecovery(
	request publicInitRequest,
) []string {
	recovery := []string{"haft", "init", "--agents"}
	if request.agentSkills == publicAgentSkillsProject {
		recovery = append(recovery, "--local")
	}
	return recovery
}

func verifyPublicAgentSkillsPrecondition(
	effect publicAgentSkillsEffect,
) error {
	observed, err := observePublicAgentSkillsEffect(effect.output)
	if err != nil {
		return err
	}
	exact := observed.kind == effect.kind
	exact = exact &&
		observed.expectedDigest == effect.expectedDigest
	exact = exact &&
		observed.expectedMode == effect.expectedMode
	if !exact {
		return fmt.Errorf(
			"agent skill path %s changed after preview; no agent skill files were written",
			effect.output.Path(),
		)
	}
	return nil
}

func writePublicAgentSkillOutput(
	output initplanning.RenderedOutput,
) error {
	parent := filepath.Dir(output.Path())
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf(
			"create agent skill parent %s: %w",
			parent,
			err,
		)
	}
	stage, err := os.CreateTemp(
		parent,
		".haft-agent-skill-*",
	)
	if err != nil {
		return fmt.Errorf(
			"stage agent skill %s: %w",
			output.Path(),
			err,
		)
	}
	stagePath := stage.Name()
	writeErr := stage.Chmod(output.Mode())
	if writeErr == nil {
		_, writeErr = stage.Write(output.Content())
	}
	if writeErr == nil {
		writeErr = stage.Sync()
	}
	closeErr := stage.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(stagePath)
		if writeErr != nil {
			return fmt.Errorf(
				"write staged agent skill %s: %w",
				output.Path(),
				writeErr,
			)
		}
		return fmt.Errorf(
			"close staged agent skill %s: %w",
			output.Path(),
			closeErr,
		)
	}
	if err := os.Rename(stagePath, output.Path()); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"publish agent skill %s: %w",
			output.Path(),
			err,
		)
	}
	return nil
}

const publicAgentSkillsManifestSchema = "haft.agent-skills-installation-manifest/v1"

type publicAgentSkillsManifestEntry struct {
	Path      string                 `json:"path"`
	Digest    string                 `json:"digest"`
	Mode      uint32                 `json:"mode"`
	Component initplanning.Component `json:"component"`
}

type publicAgentSkillsManifestWire struct {
	Schema              string                           `json:"schema"`
	ProjectRoot         string                           `json:"project_root"`
	ProjectID           string                           `json:"project_id"`
	Scope               initplanning.InstallScope        `json:"scope"`
	Root                string                           `json:"root"`
	AdapterEdition      string                           `json:"adapter_edition"`
	SkillBundleDigest   string                           `json:"skill_bundle_digest"`
	KernelCatalogDigest string                           `json:"kernel_catalog_digest"`
	RenderedPaths       []publicAgentSkillsManifestEntry `json:"rendered_paths"`
}

func buildPublicAgentSkillsManifest(
	request publicInitRequest,
	scope initplanning.InstallScope,
	root string,
	adapterEdition string,
	bundle initplanning.SkillSourceBundle,
	outputs []initplanning.RenderedOutput,
) ([]byte, string, error) {
	paths := make(
		[]publicAgentSkillsManifestEntry,
		len(outputs),
	)
	for index, output := range outputs {
		paths[index] = publicAgentSkillsManifestEntry{
			Path:      output.Path(),
			Digest:    output.Digest(),
			Mode:      uint32(output.Mode().Perm()),
			Component: output.Component(),
		}
	}
	wire := publicAgentSkillsManifestWire{
		Schema:              publicAgentSkillsManifestSchema,
		ProjectRoot:         request.projectRoot,
		ProjectID:           request.projectID,
		Scope:               scope,
		Root:                root,
		AdapterEdition:      adapterEdition,
		SkillBundleDigest:   bundle.Digest(),
		KernelCatalogDigest: bundle.KernelCatalogDigest(),
		RenderedPaths:       paths,
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return nil, "", fmt.Errorf(
			"encode agent skill installation manifest: %w",
			err,
		)
	}
	digest := fmt.Sprintf(
		"sha256:%x",
		sha256.Sum256(canonical),
	)
	return canonical, digest, nil
}

func publicAgentSkillsManifestPath(
	scope initplanning.InstallScope,
	projectRoot string,
	userHomeRoot string,
) string {
	root := projectRoot
	if scope == initplanning.ScopeUser {
		root = userHomeRoot
	}
	return filepath.Join(
		root,
		".haft",
		"agent-skill-installations",
		"agent-skills."+string(scope)+".json",
	)
}

type publicAgentManifestPreconditionKind string

const (
	publicAgentManifestMissing publicAgentManifestPreconditionKind = "missing"
	publicAgentManifestDigest  publicAgentManifestPreconditionKind = "digest"
)

type publicAgentManifestPrecondition struct {
	kind   publicAgentManifestPreconditionKind
	digest string
}

func observePublicAgentManifest(
	path string,
) (publicAgentManifestPrecondition, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return publicAgentManifestPrecondition{
			kind: publicAgentManifestMissing,
		}, nil
	}
	if err != nil {
		return publicAgentManifestPrecondition{}, fmt.Errorf(
			"inspect agent skill installation manifest: %w",
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return publicAgentManifestPrecondition{}, fmt.Errorf(
			"agent skill installation manifest is not a regular file",
		)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return publicAgentManifestPrecondition{}, fmt.Errorf(
			"read agent skill installation manifest: %w",
			err,
		)
	}
	var wire publicAgentSkillsManifestWire
	if err := json.Unmarshal(content, &wire); err != nil {
		return publicAgentManifestPrecondition{}, fmt.Errorf(
			"parse agent skill installation manifest: %w",
			err,
		)
	}
	if wire.Schema != publicAgentSkillsManifestSchema {
		return publicAgentManifestPrecondition{}, fmt.Errorf(
			"agent skill installation manifest schema is not current",
		)
	}
	return publicAgentManifestPrecondition{
		kind: publicAgentManifestDigest,
		digest: fmt.Sprintf(
			"sha256:%x",
			sha256.Sum256(content),
		),
	}, nil
}

func verifyPublicAgentManifestPrecondition(
	path string,
	expected publicAgentManifestPrecondition,
) error {
	observed, err := observePublicAgentManifest(path)
	if err != nil {
		return err
	}
	if observed == expected {
		return nil
	}
	return fmt.Errorf(
		"agent skill installation manifest changed after preview; no agent skill files were written",
	)
}

func writePublicAgentManifest(
	plan publicAgentSkillsPlan,
) error {
	parent := filepath.Dir(plan.manifestPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf(
			"create agent skill manifest parent: %w",
			err,
		)
	}
	stage, err := os.CreateTemp(
		parent,
		".haft-agent-skill-manifest-*",
	)
	if err != nil {
		return fmt.Errorf(
			"stage agent skill installation manifest: %w",
			err,
		)
	}
	stagePath := stage.Name()
	writeErr := stage.Chmod(0o644)
	if writeErr == nil {
		_, writeErr = stage.Write(plan.manifestBytes)
	}
	if writeErr == nil {
		writeErr = stage.Sync()
	}
	closeErr := stage.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(stagePath)
		if writeErr != nil {
			return fmt.Errorf(
				"write staged agent skill installation manifest: %w",
				writeErr,
			)
		}
		return fmt.Errorf(
			"close staged agent skill installation manifest: %w",
			closeErr,
		)
	}
	if err := os.Rename(stagePath, plan.manifestPath); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf(
			"publish agent skill installation manifest: %w",
			err,
		)
	}
	return nil
}
