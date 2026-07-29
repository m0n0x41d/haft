package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestCurrentCoherentHostProjectionsOwnOnlyExactSharedFragments(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create user home: %v", err)
	}
	t.Setenv("HOME", home)
	executablePath := filepath.Join(t.TempDir(), "haft")
	if err := os.WriteFile(executablePath, []byte("haft fixture"), 0o755); err != nil {
		t.Fatalf("write executable fixture: %v", err)
	}
	runtime := currentHostPublicationRuntime{
		haftVersion:    "v9.dev",
		executablePath: executablePath,
		userHomeRoot:   home,
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(root, bundle, runtime)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}

	tests := []struct {
		host               initplanning.HostID
		scope              initplanning.InstallScope
		carrierPath        string
		components         []initplanning.Component
		wantSkillOutput    bool
		wantPackage        bool
		wantFragments      int
		fragmentComponents map[string]initplanning.Component
		fragmentPaths      map[string]string
		recoveryArgv       []string
	}{
		{
			host:        initplanning.HostClaude,
			scope:       initplanning.ScopeProject,
			carrierPath: filepath.Join(root, ".mcp.json"),
			components: []initplanning.Component{
				initplanning.ComponentInstructions,
				initplanning.ComponentMCP,
				initplanning.ComponentSkills,
			},
			wantSkillOutput: true,
			wantFragments:   2,
			fragmentComponents: map[string]initplanning.Component{
				"/mcpServers/haft": initplanning.ComponentMCP,
				"haft":             initplanning.ComponentInstructions,
			},
			fragmentPaths: map[string]string{
				"/mcpServers/haft": filepath.Join(root, ".mcp.json"),
				"haft":             filepath.Join(root, "CLAUDE.md"),
			},
			recoveryArgv: []string{"haft", "init", "--claude", "--local"},
		},
		{
			host:            initplanning.HostCursor,
			scope:           initplanning.ScopeProject,
			carrierPath:     filepath.Join(root, ".cursor", "mcp.json"),
			components:      []initplanning.Component{initplanning.ComponentMCP, initplanning.ComponentSkills},
			wantSkillOutput: true,
			wantFragments:   1,
			recoveryArgv:    []string{"haft", "init", "--cursor", "--local"},
		},
		{
			host:        initplanning.HostCodex,
			scope:       initplanning.ScopeProject,
			carrierPath: filepath.Join(root, ".codex", "config.toml"),
			components: []initplanning.Component{
				initplanning.ComponentInstructions,
				initplanning.ComponentMCP,
				initplanning.ComponentSkills,
			},
			wantSkillOutput: true,
			wantFragments:   2,
			fragmentComponents: map[string]initplanning.Component{
				"mcp_servers.haft": initplanning.ComponentMCP,
				"haft":             initplanning.ComponentInstructions,
			},
			fragmentPaths: map[string]string{
				"mcp_servers.haft": filepath.Join(root, ".codex", "config.toml"),
				"haft":             filepath.Join(root, "AGENTS.md"),
			},
			recoveryArgv: []string{"haft", "init", "--codex", "--local"},
		},
		{
			host:            initplanning.HostOpenCode,
			scope:           initplanning.ScopeProject,
			carrierPath:     filepath.Join(root, "opencode.json"),
			components:      []initplanning.Component{initplanning.ComponentMCP, initplanning.ComponentSkills},
			wantSkillOutput: true,
			wantFragments:   1,
			recoveryArgv:    []string{"haft", "init", "--opencode", "--local"},
		},
		{
			host:        initplanning.HostGrok,
			scope:       initplanning.ScopeProject,
			carrierPath: filepath.Join(root, ".grok", "config.toml"),
			components: []initplanning.Component{
				initplanning.ComponentInstructions,
				initplanning.ComponentMCP,
				initplanning.ComponentSkills,
			},
			wantSkillOutput: true,
			wantFragments:   2,
			fragmentComponents: map[string]initplanning.Component{
				"mcp_servers.haft": initplanning.ComponentMCP,
				"haft":             initplanning.ComponentInstructions,
			},
			fragmentPaths: map[string]string{
				"mcp_servers.haft": filepath.Join(root, ".grok", "config.toml"),
				"haft":             filepath.Join(root, "CLAUDE.md"),
			},
			recoveryArgv: []string{"haft", "init", "--grok", "--local"},
		},
		{
			host:            initplanning.HostAir,
			scope:           initplanning.ScopeProject,
			carrierPath:     filepath.Join(root, ".codex", "config.toml"),
			components:      []initplanning.Component{initplanning.ComponentMCP, initplanning.ComponentSkills},
			wantSkillOutput: true,
			wantFragments:   1,
			recoveryArgv:    []string{"haft", "init", "--air"},
		},
		{
			host:            initplanning.HostAntigravity,
			scope:           initplanning.ScopeUser,
			carrierPath:     filepath.Join(home, ".gemini", "config", "mcp_config.json"),
			components:      []initplanning.Component{initplanning.ComponentMCP, initplanning.ComponentSkills},
			wantSkillOutput: true,
			wantFragments:   1,
			recoveryArgv:    []string{"haft", "init", "--agy"},
		},
		{
			host:          initplanning.HostGemini,
			scope:         initplanning.ScopeUser,
			carrierPath:   filepath.Join(home, ".gemini", "settings.json"),
			components:    []initplanning.Component{initplanning.ComponentMCP},
			wantFragments: 1,
			recoveryArgv: []string{
				"haft",
				"init",
				"--gemini",
			},
		},
		{
			host:          initplanning.HostZed,
			scope:         initplanning.ScopeUser,
			carrierPath:   filepath.Join(home, ".config", "zed", "settings.json"),
			components:    []initplanning.Component{initplanning.ComponentMCP},
			wantFragments: 1,
			recoveryArgv: []string{
				"haft",
				"init",
				"--zed",
			},
		},
		{
			host:          initplanning.HostPi,
			scope:         initplanning.ScopeProject,
			carrierPath:   filepath.Join(root, ".pi", "settings.json"),
			components:    []initplanning.Component{initplanning.ComponentPackage},
			wantPackage:   true,
			wantFragments: 1,
			recoveryArgv: []string{
				"haft",
				"init",
				"--pi",
			},
		},
		{
			host:            initplanning.HostHermes,
			scope:           initplanning.ScopeUser,
			carrierPath:     filepath.Join(home, ".hermes", "config.yaml"),
			components:      []initplanning.Component{initplanning.ComponentMCP, initplanning.ComponentSkills},
			wantSkillOutput: true,
			wantFragments:   2,
			fragmentComponents: map[string]initplanning.Component{
				"/mcp_servers/haft":                          initplanning.ComponentMCP,
				"/skills/external_dirs#haft-standard-skills": initplanning.ComponentSkills,
			},
			recoveryArgv: []string{
				"haft",
				"init",
				"--hermes",
			},
		},
	}
	for _, test := range tests {
		t.Run(string(test.host), func(t *testing.T) {
			projection, err := buildCurrentCoherentHostProjection(
				root,
				"qnt_e3149c17",
				test.host,
				test.scope,
				candidates,
				bundle,
				publication,
				runtime,
			)
			if err != nil {
				t.Fatalf("buildCurrentCoherentHostProjection: %v", err)
			}
			if !slices.Equal(projection.Components().Values(), test.components) {
				t.Fatalf(
					"components = %v, want %v",
					projection.Components().Values(),
					test.components,
				)
			}
			if !slices.Equal(
				projection.Recovery().Argv(),
				test.recoveryArgv,
			) {
				t.Fatalf(
					"recovery argv = %v, want %v",
					projection.Recovery().Argv(),
					test.recoveryArgv,
				)
			}
			fragments := projection.ManagedFragments()
			if len(fragments) != test.wantFragments {
				t.Fatalf("managed fragments = %+v", fragments)
			}
			allowedCarrierPaths := map[string]struct{}{
				test.carrierPath: {},
			}
			for _, carrierPath := range test.fragmentPaths {
				allowedCarrierPaths[carrierPath] = struct{}{}
			}
			fragmentComponents := make(map[string]initplanning.Component)
			fragmentPaths := make(map[string]string)
			for _, fragment := range fragments {
				carrierPath := fragment.Coordinate().CarrierPath()
				if _, allowed := allowedCarrierPaths[carrierPath]; !allowed {
					t.Fatalf("managed fragment uses another carrier: %+v", fragment)
				}
				coordinate := fragment.Coordinate().Selector()
				if fragment.Coordinate().MemberID() != "" {
					coordinate += "#" + fragment.Coordinate().MemberID()
				}
				fragmentComponents[coordinate] = fragment.Component()
				fragmentPaths[coordinate] = carrierPath
			}
			for coordinate, component := range test.fragmentComponents {
				if fragmentComponents[coordinate] != component {
					t.Fatalf(
						"fragment component %s = %s, want %s; all=%v",
						coordinate,
						fragmentComponents[coordinate],
						component,
						fragmentComponents,
					)
				}
			}
			for coordinate, carrierPath := range test.fragmentPaths {
				if fragmentPaths[coordinate] != carrierPath {
					t.Fatalf(
						"fragment carrier %s = %s, want %s; all=%v",
						coordinate,
						fragmentPaths[coordinate],
						carrierPath,
						fragmentPaths,
					)
				}
			}
			for _, output := range projection.Outputs() {
				if _, shared := allowedCarrierPaths[output.Path()]; shared {
					t.Fatalf("projection claims shared carrier as whole output: %s", output.Path())
				}
			}
			if test.wantSkillOutput && !hasProjectionComponent(
				projection.Outputs(),
				initplanning.ComponentSkills,
			) {
				t.Fatal("coherent projection omitted standard skills")
			}
			if test.wantPackage && !hasProjectionComponent(
				projection.Outputs(),
				initplanning.ComponentPackage,
			) {
				t.Fatal("Pi coherent projection omitted package files")
			}
			content := make([]byte, 0)
			for _, fragment := range fragments {
				content = append(content, fragment.Content()...)
			}
			projectPortable := test.scope == initplanning.ScopeProject &&
				test.host != initplanning.HostPi
			if projectPortable {
				if bytes.Contains(content, []byte(executablePath)) ||
					bytes.Contains(content, []byte(root)) {
					t.Fatalf(
						"project fragment embeds machine-local paths: %s",
						content,
					)
				}
				if !bytes.Contains(content, []byte(currentPortableExecutable)) ||
					!bytes.Contains(content, []byte("qnt_e3149c17")) {
					t.Fatalf(
						"project fragment lacks portable command/project guard: %s",
						content,
					)
				}
			}
			if test.scope == initplanning.ScopeUser {
				if !bytes.Contains(content, []byte(executablePath)) ||
					!bytes.Contains(content, []byte(root)) ||
					!bytes.Contains(content, []byte("qnt_e3149c17")) {
					t.Fatalf(
						"user fragment lacks exact executable/project binding: %s",
						content,
					)
				}
			}
			manifest, err := initplanning.BuildProjectionInstallationManifest(
				projection,
			)
			if err != nil {
				t.Fatalf("BuildProjectionInstallationManifest: %v", err)
			}
			if manifest.Schema() != "haft.host-installation-manifest/v2" ||
				len(manifest.ManagedFragments()) != test.wantFragments {
				t.Fatalf("coherent manifest = %s", manifest.CanonicalBytes())
			}
			if !slices.Equal(manifest.Components(), test.components) {
				t.Fatalf(
					"manifest components = %v, want %v",
					manifest.Components(),
					test.components,
				)
			}
			manifestFragmentComponents := make(map[string]initplanning.Component)
			for _, fragment := range manifest.ManagedFragments() {
				coordinate := fragment.Selector
				if fragment.MemberID != "" {
					coordinate += "#" + fragment.MemberID
				}
				manifestFragmentComponents[coordinate] = fragment.Component
			}
			for coordinate, component := range test.fragmentComponents {
				if manifestFragmentComponents[coordinate] != component {
					t.Fatalf(
						"manifest fragment component %s = %s, want %s; all=%v",
						coordinate,
						manifestFragmentComponents[coordinate],
						component,
						manifestFragmentComponents,
					)
				}
			}
			canonical := string(manifest.CanonicalBytes())
			if strings.Contains(canonical, `"carrier_digest"`) ||
				strings.Contains(canonical, `"carrier_mode"`) {
				t.Fatalf("manifest claims whole shared carrier: %s", canonical)
			}
			if !strings.Contains(canonical, executablePath) {
				t.Fatalf(
					"manifest lost exact publication executable identity: %s",
					canonical,
				)
			}
			parsed, err := initplanning.ParseInstallationManifest(
				manifest.CanonicalBytes(),
			)
			if err != nil {
				t.Fatalf("ParseInstallationManifest: %v", err)
			}
			if parsed.Digest() != manifest.Digest() {
				t.Fatalf(
					"parsed manifest digest = %s, want %s",
					parsed.Digest(),
					manifest.Digest(),
				)
			}
		})
	}
}

func TestSelectedCurrentCoherentHostProjectionPublishesOnlySelectedComponents(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create user home: %v", err)
	}
	t.Setenv("HOME", home)
	executablePath := filepath.Join(t.TempDir(), "haft")
	if err := os.WriteFile(executablePath, []byte("haft fixture"), 0o755); err != nil {
		t.Fatalf("write executable fixture: %v", err)
	}
	runtime := currentHostPublicationRuntime{
		haftVersion:    "v9.dev",
		executablePath: executablePath,
		userHomeRoot:   home,
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(root, bundle, runtime)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}

	tests := []struct {
		name               string
		host               initplanning.HostID
		components         []string
		wantOutputContains string
		wantOutputAbsent   string
		wantSelectors      []string
	}{
		{
			name: "codex mcp does not publish skills",
			host: initplanning.HostCodex,
			components: []string{
				string(initplanning.ComponentMCP),
			},
			wantOutputAbsent: filepath.Join(root, ".codex", "skills"),
			wantSelectors:    []string{"mcp_servers.haft"},
		},
		{
			name: "codex full local integration",
			host: initplanning.HostCodex,
			components: []string{
				string(initplanning.ComponentInstructions),
				string(initplanning.ComponentMCP),
				string(initplanning.ComponentSkills),
			},
			wantOutputContains: filepath.Join(root, ".agents", "skills"),
			wantSelectors:      []string{"mcp_servers.haft", "haft"},
		},
		{
			name: "claude project mcp and instructions omit skills",
			host: initplanning.HostClaude,
			components: []string{
				string(initplanning.ComponentMCP),
				string(initplanning.ComponentInstructions),
			},
			wantOutputAbsent: filepath.Join(root, ".claude", "skills"),
			wantSelectors:    []string{"/mcpServers/haft", "haft"},
		},
		{
			name: "claude local skills and mcp omit instructions",
			host: initplanning.HostClaude,
			components: []string{
				string(initplanning.ComponentMCP),
				string(initplanning.ComponentSkills),
			},
			wantOutputContains: filepath.Join(root, ".claude", "skills"),
			wantSelectors:      []string{"/mcpServers/haft"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			components, err := initplanning.ParseComponentSet(test.components)
			if err != nil {
				t.Fatalf("ParseComponentSet: %v", err)
			}
			projection, err := buildSelectedCurrentCoherentHostProjection(
				root,
				"qnt_e3149c17",
				test.host,
				initplanning.ScopeProject,
				components,
				candidates,
				bundle,
				publication,
				runtime,
			)
			if err != nil {
				t.Fatalf(
					"buildSelectedCurrentCoherentHostProjection: %v",
					err,
				)
			}
			if !slices.Equal(
				projection.Components().Values(),
				components.Values(),
			) {
				t.Fatalf(
					"components = %v, want %v",
					projection.Components().Values(),
					components.Values(),
				)
			}
			outputPaths := make([]string, 0, len(projection.Outputs()))
			for _, output := range projection.Outputs() {
				outputPaths = append(outputPaths, output.Path())
			}
			if test.wantOutputContains != "" &&
				!containsPathPrefix(outputPaths, test.wantOutputContains) {
				t.Fatalf(
					"outputs %v do not contain prefix %s",
					outputPaths,
					test.wantOutputContains,
				)
			}
			if test.wantOutputAbsent != "" &&
				containsPathPrefix(outputPaths, test.wantOutputAbsent) {
				t.Fatalf(
					"outputs %v unexpectedly contain prefix %s",
					outputPaths,
					test.wantOutputAbsent,
				)
			}
			selectors := make(
				[]string,
				0,
				len(projection.ManagedFragments()),
			)
			for _, fragment := range projection.ManagedFragments() {
				selectors = append(
					selectors,
					fragment.Coordinate().Selector(),
				)
			}
			if !slices.Equal(selectors, test.wantSelectors) {
				t.Fatalf(
					"managed selectors = %v, want %v",
					selectors,
					test.wantSelectors,
				)
			}
		})
	}
}

func containsPathPrefix(paths []string, prefix string) bool {
	for _, path := range paths {
		if path == prefix || strings.HasPrefix(
			path,
			prefix+string(filepath.Separator),
		) {
			return true
		}
	}
	return false
}

func TestCurrentCoherentHostProjectionRejectsUnboundHermesProjectScope(
	t *testing.T,
) {
	runtime := currentHostPublicationRuntime{
		haftVersion:    "v9.dev",
		executablePath: filepath.Join(t.TempDir(), "haft"),
		userHomeRoot:   t.TempDir(),
	}
	_, err := buildCurrentCoherentHostProjection(
		t.TempDir(),
		"qnt_e3149c17",
		initplanning.HostHermes,
		initplanning.ScopeProject,
		nil,
		initplanning.SkillSourceBundle{},
		initplanning.PublicationIdentity{},
		runtime,
	)
	if err == nil || !strings.Contains(err.Error(), "no coherent project projection") {
		t.Fatalf("Hermes project projection error = %v", err)
	}
}

func TestCurrentHaftInstructionFragmentMatchesLegacyClaudeSection(
	t *testing.T,
) {
	root := t.TempDir()
	fragment, err := currentHaftInstructionFragment(root)
	if err != nil {
		t.Fatalf("currentHaftInstructionFragment: %v", err)
	}
	wantSection := "<!-- haft:start -->\n" +
		strings.TrimSpace(embeddedClaudeMDTemplate) +
		"\n<!-- haft:end -->"
	if !bytes.Equal(fragment.Content(), []byte(wantSection)) {
		t.Fatal("coherent instruction fragment differs from the embedded Haft section")
	}
	coordinate := fragment.Coordinate()
	if coordinate.Kind() != initplanning.ManagedHTMLCommentSection ||
		coordinate.Selector() != "haft" ||
		fragment.Component() != initplanning.ComponentInstructions {
		t.Fatalf(
			"coherent instruction fragment coordinate=%+v component=%s",
			coordinate,
			fragment.Component(),
		)
	}
}

func hasProjectionComponent(
	outputs []initplanning.RenderedOutput,
	component initplanning.Component,
) bool {
	for _, output := range outputs {
		if output.Component() == component {
			return true
		}
	}
	return false
}
