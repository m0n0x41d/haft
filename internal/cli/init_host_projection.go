package cli

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
	haftpi "github.com/m0n0x41d/haft/packages/haft-pi"
	"gopkg.in/yaml.v3"
)

type currentHostPublicationRuntime struct {
	haftVersion    string
	executablePath string
	userHomeRoot   string
}

type currentStandardSkillCandidate struct {
	platform   string
	host       initplanning.HostID
	scope      initplanning.InstallScope
	targetRoot string
	projection initplanning.SkillComponentProjection
}

func currentHostPublicationRuntimeFromProcess() (
	currentHostPublicationRuntime,
	error,
) {
	executablePath, err := os.Executable()
	if err != nil {
		return currentHostPublicationRuntime{}, fmt.Errorf(
			"resolve Haft executable: %w",
			err,
		)
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		return currentHostPublicationRuntime{}, fmt.Errorf(
			"resolve Haft executable symlinks: %w",
			err,
		)
	}
	executablePath = filepath.Clean(executablePath)
	userHomeRoot, err := os.UserHomeDir()
	if err != nil {
		return currentHostPublicationRuntime{}, fmt.Errorf(
			"resolve user home: %w",
			err,
		)
	}
	userHomeRoot, err = filepath.Abs(userHomeRoot)
	if err != nil {
		return currentHostPublicationRuntime{}, fmt.Errorf(
			"resolve absolute user home: %w",
			err,
		)
	}
	return currentHostPublicationRuntime{
		haftVersion:    Version,
		executablePath: executablePath,
		userHomeRoot:   filepath.Clean(userHomeRoot),
	}, nil
}

func currentHostPublicationIdentity(
	runtime currentHostPublicationRuntime,
	bundle initplanning.SkillSourceBundle,
) (initplanning.PublicationIdentity, error) {
	executableDigest, err := digestRegularFile(runtime.executablePath)
	if err != nil {
		return initplanning.PublicationIdentity{}, err
	}
	return initplanning.NewPublicationIdentity(
		initplanning.PublicationIdentityInput{
			HaftVersion:         runtime.haftVersion,
			ExecutablePath:      runtime.executablePath,
			ExecutableDigest:    executableDigest,
			SkillBundleDigest:   bundle.Digest(),
			KernelCatalogDigest: bundle.KernelCatalogDigest(),
		},
	)
}

func digestRegularFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open executable for digest: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		closeErr := file.Close()
		return "", fmt.Errorf(
			"inspect executable for digest: %w",
			errors.Join(statErr, closeErr),
		)
	}
	if !info.Mode().IsRegular() {
		closeErr := file.Close()
		kindErr := fmt.Errorf(
			"executable path is not a regular file: %s: %w",
			path,
			os.ErrInvalid,
		)
		return "", errors.Join(kindErr, closeErr)
	}
	digest := sha256.New()
	_, copyErr := io.Copy(digest, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", fmt.Errorf("digest executable: %w", err)
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}

func currentStandardSkillCandidates(
	projectRoot string,
	bundle initplanning.SkillSourceBundle,
	runtime currentHostPublicationRuntime,
) ([]currentStandardSkillCandidate, error) {
	adapters, err := buildCurrentSkillAdapterRegistry()
	if err != nil {
		return nil, err
	}
	candidates := make([]currentStandardSkillCandidate, 0, len(adapters)*2)
	for _, adapter := range adapters {
		roots, rootsErr := currentSkillProjectionRoots(
			adapter,
			projectRoot,
			runtime.userHomeRoot,
		)
		if rootsErr != nil {
			return nil, rootsErr
		}
		for _, root := range roots {
			projection, projectionErr := adapter.renderer.Render(
				bundle,
				root.targetRoot,
			)
			if projectionErr != nil {
				return nil, fmt.Errorf(
					"render %s %s skill projection: %w",
					adapter.definition.host,
					root.scope,
					projectionErr,
				)
			}
			candidates = append(candidates, currentStandardSkillCandidate{
				platform:   adapter.definition.platform,
				host:       adapter.definition.host,
				scope:      root.scope,
				targetRoot: root.targetRoot,
				projection: projection,
			})
		}
	}
	sort.Slice(candidates, func(left int, right int) bool {
		leftKey := string(candidates[left].host) + "\x00" +
			string(candidates[left].scope)
		rightKey := string(candidates[right].host) + "\x00" +
			string(candidates[right].scope)
		return leftKey < rightKey
	})
	return candidates, nil
}

type currentSkillProjectionRoot struct {
	scope      initplanning.InstallScope
	targetRoot string
}

func currentSkillProjectionRoots(
	adapter currentSkillAdapter,
	projectRoot string,
	userHomeRoot string,
) ([]currentSkillProjectionRoot, error) {
	platform := adapter.definition.platform
	switch platform {
	case "air":
		root, supported := skillsRoot(platform, true, projectRoot)
		if !supported {
			return nil, fmt.Errorf("air skill root is unavailable")
		}
		return []currentSkillProjectionRoot{{
			scope:      initplanning.ScopeProject,
			targetRoot: root,
		}}, nil
	case "hermes":
		userRoot := currentHermesUserSkillsRoot(userHomeRoot)
		roots := []currentSkillProjectionRoot{{
			scope:      initplanning.ScopeUser,
			targetRoot: userRoot,
		}}
		if isHaftSourceRoot(projectRoot) {
			projectRoot := filepath.Join(
				projectRoot,
				filepath.FromSlash(hermesSkillsRelDir),
			)
			roots = append(roots, currentSkillProjectionRoot{
				scope:      initplanning.ScopeProject,
				targetRoot: projectRoot,
			})
		}
		return roots, nil
	default:
		projectSkillRoot, projectSupported := skillsRoot(
			platform,
			true,
			projectRoot,
		)
		userSkillRoot, userSupported := skillsRoot(
			platform,
			false,
			projectRoot,
		)
		if !projectSupported || !userSupported {
			return nil, fmt.Errorf(
				"%s standard skill roots are incomplete",
				adapter.definition.host,
			)
		}
		userRelative, userRelativeErr := filepath.Rel(
			userHomeRoot,
			userSkillRoot,
		)
		userOutsidePrefix := ".." + string(filepath.Separator)
		userInsideHome := userRelativeErr == nil
		userInsideHome = userInsideHome && userRelative != ".."
		userInsideHome = userInsideHome && !filepath.IsAbs(userRelative)
		userInsideHome = userInsideHome && !strings.HasPrefix(
			userRelative,
			userOutsidePrefix,
		)
		if !userInsideHome {
			return nil, fmt.Errorf(
				"%s user skill root is outside the supplied user home",
				adapter.definition.host,
			)
		}
		if filepath.Clean(userSkillRoot) == filepath.Clean(projectSkillRoot) {
			return []currentSkillProjectionRoot{{
				scope:      initplanning.ScopeProject,
				targetRoot: projectSkillRoot,
			}}, nil
		}
		return []currentSkillProjectionRoot{
			{
				scope:      initplanning.ScopeProject,
				targetRoot: projectSkillRoot,
			},
			{
				scope:      initplanning.ScopeUser,
				targetRoot: userSkillRoot,
			},
		}, nil
	}
}

func buildCurrentStandardSkillHostProjection(
	projectRoot string,
	projectID string,
	candidate currentStandardSkillCandidate,
	publication initplanning.PublicationIdentity,
) (initplanning.HostAdapterProjection, error) {
	components, err := initplanning.ParseComponentSet([]string{
		string(initplanning.ComponentSkills),
	})
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	recovery, err := currentLegacySkillRecovery(candidate)
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	builder := initplanning.NewHostAdapterProjectionBuilder(candidate.host)
	builder = builder.AtEdition(candidate.projection.Edition())
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(projectRoot, projectID)
	builder = builder.WithSelection(candidate.scope, components)
	builder = builder.AddTargetRoot(candidate.targetRoot)
	for _, output := range candidate.projection.Outputs() {
		builder = builder.AddOutput(output)
	}
	builder = builder.RecoverWith(recovery)
	return builder.Build()
}

func currentLegacySkillRecovery(
	candidate currentStandardSkillCandidate,
) (initplanning.RecoveryOperation, error) {
	argv := []string{"haft", "init", "--" + candidate.platform}
	if candidate.scope == initplanning.ScopeProject &&
		candidate.platform != "air" &&
		candidate.platform != "hermes" {
		argv = append(argv, "--local")
	}
	return initplanning.NewRecoveryOperation(argv)
}

func findCurrentStandardSkillCandidate(
	candidates []currentStandardSkillCandidate,
	host initplanning.HostID,
	scope initplanning.InstallScope,
) (currentStandardSkillCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.host == host && candidate.scope == scope {
			return candidate, true
		}
	}
	return currentStandardSkillCandidate{}, false
}

const (
	currentJSONFragmentMergeEdition = "json.semantic-merge.v1"
	currentTOMLFragmentMergeEdition = "toml.table-family-merge.v1"
	currentYAMLFragmentMergeEdition = "yaml.semantic-merge.v1"
	currentTextFragmentMergeEdition = "html-comment-section-merge.v1"
	currentPortableExecutable       = "haft"
)

type currentCoherentHostKey struct {
	host  initplanning.HostID
	scope initplanning.InstallScope
}

type currentCoherentHostContext struct {
	projectRoot string
	projectID   string
	runtime     currentHostPublicationRuntime
	bundle      initplanning.SkillSourceBundle
}

type currentCoherentHostFace struct {
	edition     string
	platform    string
	components  []initplanning.Component
	targetRoots []string
	outputs     []initplanning.RenderedOutput
	fragments   []initplanning.ManagedFragment
	coarsening  initplanning.ControlledCoarseningDeclaration
	recovery    []string
}

type currentCoherentHostFaceFactory func(
	currentCoherentHostContext,
) (currentCoherentHostFace, error)

func buildSelectedCurrentCoherentHostProjection(
	projectRoot string,
	projectID string,
	host initplanning.HostID,
	scope initplanning.InstallScope,
	components initplanning.ComponentSet,
	candidates []currentStandardSkillCandidate,
	bundle initplanning.SkillSourceBundle,
	publication initplanning.PublicationIdentity,
	runtime currentHostPublicationRuntime,
) (initplanning.HostAdapterProjection, error) {
	key := currentCoherentHostKey{host: host, scope: scope}
	factory, available := currentCoherentHostFaceRegistry()[key]
	if !available {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s has no coherent %s projection",
			host,
			scope,
		)
	}
	applicabilityRegistry, err :=
		currentCoherentHostApplicabilityRegistry()
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	applicability, available := applicabilityRegistry[key]
	if !available {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s has no coherent %s projection component applicability",
			host,
			scope,
		)
	}
	if publication.SkillBundleDigest() != bundle.Digest() ||
		publication.KernelCatalogDigest() !=
			bundle.KernelCatalogDigest() {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s coherent projection uses another source bundle",
			host,
		)
	}
	context := currentCoherentHostContext{
		projectRoot: projectRoot,
		projectID:   projectID,
		runtime:     runtime,
		bundle:      bundle,
	}
	face, err := factory(context)
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	selected := currentComponentLookup(components)
	skillsSelected := selected[initplanning.ComponentSkills]
	if face.platform != "" && skillsSelected {
		candidate, found := findCurrentStandardSkillCandidate(
			candidates,
			host,
			scope,
		)
		if !found {
			return initplanning.HostAdapterProjection{}, fmt.Errorf(
				"host %s coherent projection lacks its standard skill face",
				host,
			)
		}
		face.components = append(
			slicesCloneComponents(face.components),
			initplanning.ComponentSkills,
		)
		face.targetRoots = append(
			slicesCloneStrings(face.targetRoots),
			candidate.targetRoot,
		)
		face.outputs = append(
			cloneCurrentRenderedOutputs(face.outputs),
			candidate.projection.Outputs()...,
		)
	}
	if err := applicability.ValidateSupportedSelection(components); err != nil {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s coherent component applicability: %w",
			host,
			err,
		)
	}
	requiresCoarsening :=
		applicability.RequiresControlledCoarsening()
	hasCoarsening := face.coarsening.Valid()
	if requiresCoarsening != hasCoarsening {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s coherent projection controlled-coarsening declaration mismatch",
			host,
		)
	}
	if hasCoarsening &&
		(face.coarsening.SourceBearingSideRef() != bundle.Ref() ||
			face.coarsening.SourceBearingDigest() != bundle.Digest()) {
		return initplanning.HostAdapterProjection{}, fmt.Errorf(
			"host %s coherent projection coarsening uses another source bundle",
			host,
		)
	}
	outputs := selectedCurrentRenderedOutputs(
		face.outputs,
		selected,
	)
	fragments := selectedCurrentManagedFragments(
		face.fragments,
		selected,
	)
	targetRoots := uniqueSortedCurrentPaths(face.targetRoots)
	recoveryArgv := currentSelectedHostRecovery(
		face.recovery,
		host,
		scope,
		selected,
	)
	recovery, err := initplanning.NewRecoveryOperation(recoveryArgv)
	if err != nil {
		return initplanning.HostAdapterProjection{}, err
	}
	builder := initplanning.NewHostAdapterProjectionBuilder(host)
	builder = builder.AtEdition(face.edition)
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(projectRoot, projectID)
	builder = builder.WithSelection(scope, components)
	for _, targetRoot := range targetRoots {
		builder = builder.AddTargetRoot(targetRoot)
	}
	for _, output := range outputs {
		builder = builder.AddOutput(output)
	}
	for _, fragment := range fragments {
		builder = builder.AddManagedFragment(fragment)
	}
	builder = builder.RecoverWith(recovery)
	return builder.Build()
}

func currentSelectedHostRecovery(
	base []string,
	host initplanning.HostID,
	scope initplanning.InstallScope,
	selected map[initplanning.Component]bool,
) []string {
	result := slicesCloneStrings(base)
	skillsLocal := scope == initplanning.ScopeProject &&
		selected[initplanning.ComponentSkills] &&
		currentHostSupportsVariableSkillScope(host)
	if skillsLocal && !currentStringSliceContains(result, "--local") {
		result = append(result, "--local")
	}
	mcpOnly := len(selected) == 1 &&
		selected[initplanning.ComponentMCP] &&
		(host == initplanning.HostClaude ||
			host == initplanning.HostCodex)
	if mcpOnly && !currentStringSliceContains(result, "--mcp-only") {
		result = append(result, "--mcp-only")
	}
	return result
}

func currentHostSupportsVariableSkillScope(
	host initplanning.HostID,
) bool {
	switch host {
	case initplanning.HostClaude,
		initplanning.HostCodex,
		initplanning.HostCursor,
		initplanning.HostOpenCode,
		initplanning.HostGrok:
		return true
	default:
		return false
	}
}

func currentStringSliceContains(
	values []string,
	target string,
) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func currentComponentLookup(
	components initplanning.ComponentSet,
) map[initplanning.Component]bool {
	selected := make(
		map[initplanning.Component]bool,
		len(components.Values()),
	)
	for _, component := range components.Values() {
		selected[component] = true
	}
	return selected
}

func selectedCurrentRenderedOutputs(
	outputs []initplanning.RenderedOutput,
	selected map[initplanning.Component]bool,
) []initplanning.RenderedOutput {
	result := make([]initplanning.RenderedOutput, 0, len(outputs))
	for _, output := range outputs {
		if !selected[output.Component()] {
			continue
		}
		result = append(result, output)
	}
	return result
}

func selectedCurrentManagedFragments(
	fragments []initplanning.ManagedFragment,
	selected map[initplanning.Component]bool,
) []initplanning.ManagedFragment {
	result := make([]initplanning.ManagedFragment, 0, len(fragments))
	for _, fragment := range fragments {
		if !selected[fragment.Component()] {
			continue
		}
		result = append(result, fragment)
	}
	return result
}

func currentCoherentHostFaceRegistry() map[currentCoherentHostKey]currentCoherentHostFaceFactory {
	return map[currentCoherentHostKey]currentCoherentHostFaceFactory{
		{
			host:  initplanning.HostClaude,
			scope: initplanning.ScopeProject,
		}: currentClaudeCoherentFace,
		{
			host:  initplanning.HostCursor,
			scope: initplanning.ScopeProject,
		}: currentCursorCoherentFace,
		{
			host:  initplanning.HostCodex,
			scope: initplanning.ScopeProject,
		}: func(
			context currentCoherentHostContext,
		) (currentCoherentHostFace, error) {
			return currentCodexCoherentFace(
				context,
				initplanning.HostCodex,
				"codex",
			)
		},
		{
			host:  initplanning.HostAir,
			scope: initplanning.ScopeProject,
		}: func(
			context currentCoherentHostContext,
		) (currentCoherentHostFace, error) {
			return currentCodexCoherentFace(
				context,
				initplanning.HostAir,
				"air",
			)
		},
		{
			host:  initplanning.HostOpenCode,
			scope: initplanning.ScopeProject,
		}: currentOpenCodeCoherentFace,
		{
			host:  initplanning.HostGrok,
			scope: initplanning.ScopeProject,
		}: currentGrokCoherentFace,
		{
			host:  initplanning.HostAntigravity,
			scope: initplanning.ScopeUser,
		}: currentAntigravityCoherentFace,
		{
			host:  initplanning.HostGemini,
			scope: initplanning.ScopeUser,
		}: currentGeminiCoherentFace,
		{
			host:  initplanning.HostZed,
			scope: initplanning.ScopeUser,
		}: currentZedCoherentFace,
		{
			host:  initplanning.HostPi,
			scope: initplanning.ScopeProject,
		}: currentPiCoherentFace,
		{
			host:  initplanning.HostHermes,
			scope: initplanning.ScopeUser,
		}: currentHermesCoherentFace,
	}
}

func currentClaudeCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(context.projectRoot, ".mcp.json")
	server := currentProjectMCPServerValue(
		context,
		claudeProjectRootEnv,
	)
	mcpFragment, err := currentJSONObjectEntryFragment(
		path,
		[]string{"mcpServers", "haft"},
		server,
		initplanning.ComponentMCP,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	instructionFragment, err := currentHaftInstructionFragment(
		context.projectRoot,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:  "claude.coherent.v2",
		platform: "claude",
		components: []initplanning.Component{
			initplanning.ComponentMCP,
			initplanning.ComponentInstructions,
		},
		targetRoots: []string{context.projectRoot},
		fragments: []initplanning.ManagedFragment{
			mcpFragment,
			instructionFragment,
		},
		recovery: []string{"haft", "init", "--claude"},
	}, nil
}

func currentCursorCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(context.projectRoot, ".cursor", "mcp.json")
	server := currentProjectMCPServerValue(
		context,
		cursorProjectRootEnv,
	)
	fragment, err := currentJSONObjectEntryFragment(
		path,
		[]string{"mcpServers", "haft"},
		server,
		initplanning.ComponentMCP,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "cursor.coherent.v1",
		platform:    "cursor",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(path)},
		fragments:   []initplanning.ManagedFragment{fragment},
		recovery:    []string{"haft", "init", "--cursor"},
	}, nil
}

func currentCodexCoherentFace(
	context currentCoherentHostContext,
	host initplanning.HostID,
	platform string,
) (currentCoherentHostFace, error) {
	path := filepath.Join(context.projectRoot, ".codex", "config.toml")
	content, err := currentCodexTOMLFragmentContent(context)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	// Retain the v1 merge edition so an existing broad table-family receipt
	// can migrate only after the table-set observer proves these exact tables.
	fragment, err := initplanning.NewTOMLTableSetFragment(
		path,
		initplanning.ComponentMCP,
		"mcp_servers.haft",
		[]string{
			"mcp_servers.haft",
			"mcp_servers.haft.env",
		},
		content,
		0o644,
		currentTOMLFragmentMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	recovery := []string{"haft", "init", "--" + platform}
	edition := string(host) + ".coherent.v1"
	components := []initplanning.Component{
		initplanning.ComponentMCP,
	}
	targetRoots := []string{filepath.Dir(path)}
	fragments := []initplanning.ManagedFragment{fragment}
	if host == initplanning.HostCodex {
		instructionFragment, instructionErr :=
			currentHaftCodexInstructionFragment(
				context.projectRoot,
			)
		if instructionErr != nil {
			return currentCoherentHostFace{}, instructionErr
		}
		edition = "codex.coherent.v2"
		components = append(
			components,
			initplanning.ComponentInstructions,
		)
		targetRoots = append(targetRoots, context.projectRoot)
		fragments = append(fragments, instructionFragment)
	}
	return currentCoherentHostFace{
		edition:     edition,
		platform:    platform,
		components:  components,
		targetRoots: targetRoots,
		fragments:   fragments,
		recovery:    recovery,
	}, nil
}

func currentOpenCodeCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(context.projectRoot, "opencode.json")
	value := map[string]any{
		"type":    "local",
		"command": []string{currentPortableExecutable, "serve"},
		"environment": currentBoundProjectEnvironment(
			codexProjectRootEnv,
			context.projectID,
		),
		"enabled": true,
	}
	fragment, err := currentJSONObjectEntryFragment(
		path,
		[]string{"mcp", "haft"},
		value,
		initplanning.ComponentMCP,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "opencode.coherent.v1",
		platform:    "opencode",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(path)},
		fragments:   []initplanning.ManagedFragment{fragment},
		recovery:    []string{"haft", "init", "--opencode"},
	}, nil
}

func currentGrokCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(context.projectRoot, ".grok", "config.toml")
	content, err := currentGrokTOMLFragmentContent(context)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	mcpFragment, err := initplanning.NewTOMLTableFamilyFragment(
		path,
		initplanning.ComponentMCP,
		"mcp_servers.haft",
		content,
		0o644,
		currentTOMLFragmentMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	instructionFragment, err := currentHaftInstructionFragment(
		context.projectRoot,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:  "grok.coherent.v2",
		platform: "grok",
		components: []initplanning.Component{
			initplanning.ComponentMCP,
			initplanning.ComponentInstructions,
		},
		targetRoots: []string{context.projectRoot},
		fragments: []initplanning.ManagedFragment{
			mcpFragment,
			instructionFragment,
		},
		recovery: []string{"haft", "init", "--grok"},
	}, nil
}

func currentHaftInstructionFragment(
	projectRoot string,
) (initplanning.ManagedFragment, error) {
	return currentHaftInstructionFragmentAt(
		projectRoot,
		"CLAUDE.md",
	)
}

func currentHaftCodexInstructionFragment(
	projectRoot string,
) (initplanning.ManagedFragment, error) {
	return currentHaftInstructionFragmentAt(
		projectRoot,
		"AGENTS.md",
	)
}

func currentHaftInstructionFragmentAt(
	projectRoot string,
	fileName string,
) (initplanning.ManagedFragment, error) {
	path := filepath.Join(projectRoot, fileName)
	return initplanning.NewHTMLCommentSectionFragment(
		path,
		initplanning.ComponentInstructions,
		"haft",
		[]byte(embeddedClaudeMDTemplate),
		0o644,
		currentTextFragmentMergeEdition,
	)
}

func currentAntigravityCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(
		context.runtime.userHomeRoot,
		".gemini",
		"config",
		"mcp_config.json",
	)
	value := map[string]any{
		"command": context.runtime.executablePath,
		"args": []string{
			"serve",
			"--project-root",
			context.projectRoot,
			"--expected-project-id",
			context.projectID,
		},
	}
	fragment, err := currentJSONObjectEntryFragment(
		path,
		[]string{"mcpServers", "haft"},
		value,
		initplanning.ComponentMCP,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "antigravity.coherent.v1",
		platform:    "agy",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(path)},
		fragments:   []initplanning.ManagedFragment{fragment},
		recovery:    []string{"haft", "init", "--agy"},
	}, nil
}

func currentGeminiCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(
		context.runtime.userHomeRoot,
		".gemini",
		"settings.json",
	)
	value := currentUserMCPServerValue(context)
	value["cwd"] = context.projectRoot
	value["timeout"] = 30000
	fragment, err := currentJSONObjectEntryFragment(
		path,
		[]string{"mcpServers", "haft"},
		value,
		initplanning.ComponentMCP,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "gemini.coherent.v1",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(path)},
		fragments:   []initplanning.ManagedFragment{fragment},
		recovery:    []string{"haft", "init", "--gemini"},
	}, nil
}

func currentZedCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	path := filepath.Join(
		context.runtime.userHomeRoot,
		".config",
		"zed",
		"settings.json",
	)
	value := currentUserMCPServerValue(context)
	fragment, err := currentJSONObjectEntryFragmentAtEdition(
		path,
		[]string{"context_servers", zedContextServerName},
		value,
		initplanning.ComponentMCP,
		initplanning.ManagedJSONCRewriteMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "zed.coherent.v1",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(path)},
		fragments:   []initplanning.ManagedFragment{fragment},
		recovery:    []string{"haft", "init", "--zed"},
	}, nil
}

func currentPiCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	settingsPath := filepath.Join(
		context.projectRoot,
		".pi",
		"settings.json",
	)
	value, err := json.Marshal(piSettingsEntry)
	if err != nil {
		return currentCoherentHostFace{}, fmt.Errorf(
			"encode Pi package setting: %w",
			err,
		)
	}
	fragment, err := initplanning.NewJSONArrayMemberFragment(
		settingsPath,
		initplanning.ComponentPackage,
		[]string{"packages"},
		"haft-pi-package",
		value,
		0o644,
		initplanning.ManagedJSONArraySourceMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	packageRoot := filepath.Join(
		context.projectRoot,
		filepath.FromSlash(piPackageRelDir),
	)
	outputs, err := currentPiPackageOutputs(packageRoot)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	parity, err := currentPiCondensedParity(
		context.bundle,
		packageRoot,
		outputs,
	)
	if err != nil {
		return currentCoherentHostFace{}, fmt.Errorf(
			"verify Pi controlled coarsening: %w",
			err,
		)
	}
	return currentCoherentHostFace{
		edition:    "pi.coherent.v2",
		components: []initplanning.Component{initplanning.ComponentPackage},
		targetRoots: []string{
			filepath.Dir(settingsPath),
			packageRoot,
		},
		outputs:    outputs,
		fragments:  []initplanning.ManagedFragment{fragment},
		coarsening: parity.Declaration,
		recovery:   []string{"haft", "init", "--pi"},
	}, nil
}

func currentHermesCoherentFace(
	context currentCoherentHostContext,
) (currentCoherentHostFace, error) {
	configPath := filepath.Join(
		context.runtime.userHomeRoot,
		".hermes",
		"config.yaml",
	)
	server := currentUserMCPServerValue(context)
	server["enabled"] = true
	serverContent, err := yaml.Marshal(server)
	if err != nil {
		return currentCoherentHostFace{}, fmt.Errorf(
			"encode coherent Hermes MCP fragment: %w",
			err,
		)
	}
	serverFragment, err := initplanning.NewYAMLMappingEntryFragment(
		configPath,
		initplanning.ComponentMCP,
		[]string{"mcp_servers", "haft"},
		serverContent,
		0o644,
		currentYAMLFragmentMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	skillsRoot := currentHermesUserSkillsRoot(
		context.runtime.userHomeRoot,
	)
	skillsContent, err := yaml.Marshal(skillsRoot)
	if err != nil {
		return currentCoherentHostFace{}, fmt.Errorf(
			"encode coherent Hermes skills fragment: %w",
			err,
		)
	}
	skillsFragment, err := initplanning.NewYAMLSequenceMemberFragment(
		configPath,
		initplanning.ComponentSkills,
		[]string{"skills", "external_dirs"},
		"haft-standard-skills",
		skillsContent,
		0o644,
		currentYAMLFragmentMergeEdition,
	)
	if err != nil {
		return currentCoherentHostFace{}, err
	}
	return currentCoherentHostFace{
		edition:     "hermes.coherent.v1",
		platform:    "hermes",
		components:  []initplanning.Component{initplanning.ComponentMCP},
		targetRoots: []string{filepath.Dir(configPath)},
		fragments: []initplanning.ManagedFragment{
			serverFragment,
			skillsFragment,
		},
		recovery: []string{"haft", "init", "--hermes"},
	}, nil
}

func currentHermesUserSkillsRoot(
	userHomeRoot string,
) string {
	return filepath.Join(
		userHomeRoot,
		".haft",
		"hermes",
		"skills",
	)
}

func currentProjectMCPServerValue(
	context currentCoherentHostContext,
	projectRootValue string,
) map[string]any {
	return map[string]any{
		"command": currentPortableExecutable,
		"args":    []string{"serve"},
		"env": currentBoundProjectEnvironment(
			projectRootValue,
			context.projectID,
		),
	}
}

func currentUserMCPServerValue(
	context currentCoherentHostContext,
) map[string]any {
	return map[string]any{
		"command": context.runtime.executablePath,
		"args":    []string{"serve"},
		"env": currentBoundProjectEnvironment(
			context.projectRoot,
			context.projectID,
		),
	}
}

func currentBoundProjectEnvironment(
	projectRootValue string,
	projectID string,
) map[string]string {
	return map[string]string{
		envProjectRoot:       projectRootValue,
		envExpectedProjectID: projectID,
	}
}

func currentJSONObjectEntryFragment(
	path string,
	selector []string,
	value any,
	component initplanning.Component,
) (initplanning.ManagedFragment, error) {
	return currentJSONObjectEntryFragmentAtEdition(
		path,
		selector,
		value,
		component,
		currentJSONFragmentMergeEdition,
	)
}

func currentJSONObjectEntryFragmentAtEdition(
	path string,
	selector []string,
	value any,
	component initplanning.Component,
	mergeEdition string,
) (initplanning.ManagedFragment, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return initplanning.ManagedFragment{}, fmt.Errorf(
			"encode managed JSON fragment: %w",
			err,
		)
	}
	return initplanning.NewJSONObjectEntryFragment(
		path,
		component,
		selector,
		content,
		0o644,
		mergeEdition,
	)
}

func currentCodexTOMLFragmentContent(
	context currentCoherentHostContext,
) ([]byte, error) {
	command, err := currentTOMLString(currentPortableExecutable)
	if err != nil {
		return nil, err
	}
	projectID, err := currentTOMLString(context.projectID)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf(`[mcp_servers.haft]
command = %s
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
HAFT_EXPECTED_PROJECT_ID = %s
`, command, projectID)
	return []byte(content), nil
}

func currentGrokTOMLFragmentContent(
	context currentCoherentHostContext,
) ([]byte, error) {
	command, err := currentTOMLString(currentPortableExecutable)
	if err != nil {
		return nil, err
	}
	projectID, err := currentTOMLString(context.projectID)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf(`[mcp_servers.haft]
command = %s
args = ["serve"]
enabled = true
startup_timeout_sec = 15
tool_timeout_sec = 60
env = { HAFT_PROJECT_ROOT = ".", HAFT_EXPECTED_PROJECT_ID = %s }
`, command, projectID)
	return []byte(content), nil
}

func currentTOMLString(value string) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode TOML basic string: %w", err)
	}
	return string(encoded), nil
}

func currentPiPackageOutputs(
	packageRoot string,
) ([]initplanning.RenderedOutput, error) {
	return piPackageOutputsFromFS(haftpi.Assets, packageRoot)
}

func parseCurrentCoherentComponents(
	components []initplanning.Component,
) (initplanning.ComponentSet, error) {
	raw := make([]string, len(components))
	for index, component := range components {
		raw[index] = string(component)
	}
	return initplanning.ParseComponentSet(raw)
}

func uniqueSortedCurrentPaths(source []string) []string {
	seen := make(map[string]struct{}, len(source))
	result := make([]string, 0, len(source))
	for _, path := range source {
		canonical := filepath.Clean(path)
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result
}

func slicesCloneComponents(
	source []initplanning.Component,
) []initplanning.Component {
	return append([]initplanning.Component(nil), source...)
}

func slicesCloneStrings(source []string) []string {
	return append([]string(nil), source...)
}

func cloneCurrentRenderedOutputs(
	source []initplanning.RenderedOutput,
) []initplanning.RenderedOutput {
	return append([]initplanning.RenderedOutput(nil), source...)
}
