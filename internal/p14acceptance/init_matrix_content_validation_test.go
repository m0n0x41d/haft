package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
	"gopkg.in/yaml.v3"
)

const (
	p14CanonicalSkillBundleEdition = "haft-public-skills.v2"
	p14AgentSkillManifestSchema    = "haft.agent-skills-installation-manifest/v1"
	p14InitJSONMergeEdition        = "json.semantic-merge.v1"
	p14InitTOMLMergeEdition        = "toml.table-family-merge.v1"
	p14InitTextMergeEdition        = "html-comment-section-merge.v1"
)

var p14CanonicalSkillNames = []string{
	"h-commission",
	"h-compare",
	"h-decide",
	"h-diagnose",
	"h-explore",
	"h-frame",
	"h-note",
	"h-onboard",
	"h-reason",
	"h-spec",
	"h-status",
	"h-verify",
}

type p14AgentSkillsManifest struct {
	Schema              string                      `json:"schema"`
	ProjectRoot         string                      `json:"project_root"`
	ProjectID           string                      `json:"project_id"`
	Scope               initplanning.InstallScope   `json:"scope"`
	Root                string                      `json:"root"`
	AdapterEdition      string                      `json:"adapter_edition"`
	SkillBundleDigest   string                      `json:"skill_bundle_digest"`
	KernelCatalogDigest string                      `json:"kernel_catalog_digest"`
	RenderedPaths       []initplanning.ManifestPath `json:"rendered_paths"`
}

type p14SkillFrontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

type p14ExpectedManagedFragment struct {
	Fragment   initplanning.ManagedFragment
	TOMLTables []string
}

func TestP14InitLegacyCommandFixturesExerciseExistingSixCaseMatrix(
	t *testing.T,
) {
	policies := p14InitMatrixPolicies()
	if len(policies) != 6 {
		t.Fatalf("P14 init policy count = %d, want 6", len(policies))
	}
	for _, policy := range policies {
		t.Run(policy.ID, func(t *testing.T) {
			projectRoot := t.TempDir()
			homeRoot := t.TempDir()
			if err := materializeP14InitLegacyCommandFixtures(
				projectRoot,
				homeRoot,
			); err != nil {
				t.Fatalf("materialize legacy-command fixtures: %v", err)
			}
			removed := make(
				map[string]struct{},
				len(policy.RemovedLegacyCommands),
			)
			for _, host := range policy.RemovedLegacyCommands {
				removed[host] = struct{}{}
			}
			for _, fixture := range p14InitLegacyCommandFixtures(
				projectRoot,
				homeRoot,
			) {
				if _, selected := removed[fixture.Host]; !selected ||
					!fixture.Removable {
					continue
				}
				if err := os.Remove(fixture.Path); err != nil {
					t.Fatalf("remove %s fixture: %v", fixture.Host, err)
				}
			}
			if err := validateP14InitLegacyCommandTakeover(
				projectRoot,
				homeRoot,
				policy.RemovedLegacyCommands,
			); err != nil {
				t.Fatalf("validate legacy-command takeover: %v", err)
			}
		})
	}
	t.Run("foreign fixture drift is rejected", func(t *testing.T) {
		projectRoot := t.TempDir()
		homeRoot := t.TempDir()
		if err := materializeP14InitLegacyCommandFixtures(
			projectRoot,
			homeRoot,
		); err != nil {
			t.Fatalf("materialize legacy-command fixtures: %v", err)
		}
		foreignPath := filepath.Join(
			homeRoot,
			".codex",
			"prompts",
			"custom.md",
		)
		if err := os.WriteFile(
			foreignPath,
			[]byte("changed\n"),
			0o644,
		); err != nil {
			t.Fatalf("change foreign fixture: %v", err)
		}
		if err := validateP14InitLegacyCommandTakeover(
			projectRoot,
			homeRoot,
			nil,
		); err == nil {
			t.Fatal("foreign legacy-command fixture drift was accepted")
		}
	})
}

func validateP14InstalledCLIInitSemanticEffects(
	repositoryRoot string,
	execution p14InstalledCLIExecutionContext,
	projectRoot string,
	homeRoot string,
	semantic p14InitMatrixSemanticCase,
) error {
	config, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		return fmt.Errorf("parse project identity: %w", err)
	}
	if config == nil {
		return fmt.Errorf("project identity is absent")
	}
	if err := validateP14InitHostManifestInventory(
		repositoryRoot,
		execution,
		projectRoot,
		homeRoot,
		config.ID,
		semantic.HostManifests,
	); err != nil {
		return err
	}
	if err := validateP14InitAgentSkillsManifestInventory(
		repositoryRoot,
		projectRoot,
		homeRoot,
		config.ID,
		semantic.IndependentAgentSkills,
	); err != nil {
		return err
	}
	if err := validateP14InitLegacyCommandTakeover(
		projectRoot,
		homeRoot,
		semantic.RemovedLegacyCommands,
	); err != nil {
		return err
	}
	if semantic.ForbidExperimentalHosts {
		if err := validateP14InitExperimentalHostsAbsent(
			projectRoot,
			homeRoot,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateP14InitLegacyCommandTakeover(
	projectRoot string,
	homeRoot string,
	removedHosts []string,
) error {
	if !slices.IsSorted(removedHosts) {
		return fmt.Errorf("removed legacy-command hosts are not canonical")
	}
	removed := make(map[string]struct{}, len(removedHosts))
	for _, host := range removedHosts {
		if host != "claude" && host != "codex" {
			return fmt.Errorf(
				"removed legacy-command host %q is unsupported",
				host,
			)
		}
		if _, duplicate := removed[host]; duplicate {
			return fmt.Errorf(
				"removed legacy-command host %q is duplicated",
				host,
			)
		}
		removed[host] = struct{}{}
	}
	for _, fixture := range p14InitLegacyCommandFixtures(
		projectRoot,
		homeRoot,
	) {
		_, shouldRemove := removed[fixture.Host]
		shouldRemove = shouldRemove && fixture.Removable
		if shouldRemove {
			if _, err := os.Lstat(fixture.Path); os.IsNotExist(err) {
				continue
			} else if err != nil {
				return fmt.Errorf(
					"inspect removed legacy-command fixture %s: %w",
					fixture.Path,
					err,
				)
			}
			return fmt.Errorf(
				"legacy-command fixture %s was not removed",
				fixture.Path,
			)
		}
		raw, mode, err := readP14InitRegularFile(fixture.Path)
		if err != nil {
			return fmt.Errorf(
				"read preserved legacy-command fixture %s: %w",
				fixture.Path,
				err,
			)
		}
		if mode.Perm() != 0o644 ||
			!bytes.Equal(raw, []byte(fixture.Content)) {
			return fmt.Errorf(
				"preserved legacy-command fixture %s changed",
				fixture.Path,
			)
		}
	}
	return nil
}

func validateP14InitHostManifestInventory(
	repositoryRoot string,
	execution p14InstalledCLIExecutionContext,
	projectRoot string,
	homeRoot string,
	projectID string,
	expected []p14InitMatrixHostManifest,
) error {
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projectRoot,
			ProjectID:    projectID,
			UserHomeRoot: homeRoot,
		},
	)
	if err != nil {
		return err
	}
	expectedPaths := make([]string, 0, len(expected))
	for _, binding := range expected {
		host, err := initplanning.ParseHostID(binding.Host)
		if err != nil {
			return fmt.Errorf("parse expected host %q: %w", binding.Host, err)
		}
		scope, err := parseP14InitScope(binding.Scope)
		if err != nil {
			return err
		}
		location, err := layout.ManifestLocation(host, scope)
		if err != nil {
			return err
		}
		expectedPaths = append(expectedPaths, location.Path())
		raw, mode, err := readP14InitRegularFile(location.Path())
		if err != nil {
			return fmt.Errorf(
				"read %s/%s host manifest: %w",
				binding.Host,
				binding.Scope,
				err,
			)
		}
		if mode.Perm() != 0o644 {
			return fmt.Errorf(
				"%s/%s host manifest mode differs",
				binding.Host,
				binding.Scope,
			)
		}
		manifest, err := initplanning.ParseInstallationManifest(raw)
		if err != nil {
			return fmt.Errorf(
				"parse %s/%s host manifest: %w",
				binding.Host,
				binding.Scope,
				err,
			)
		}
		if err := validateP14InitHostManifest(
			repositoryRoot,
			execution,
			projectRoot,
			homeRoot,
			projectID,
			binding,
			manifest,
		); err != nil {
			return err
		}
	}
	slices.Sort(expectedPaths)
	observedPaths, err := collectP14InitManifestPaths(
		projectRoot,
		homeRoot,
		"host-installations",
	)
	if err != nil {
		return err
	}
	if !slices.Equal(observedPaths, expectedPaths) {
		return fmt.Errorf(
			"host manifest inventory = %v, want %v",
			observedPaths,
			expectedPaths,
		)
	}
	return nil
}

func parseP14InitScope(raw string) (initplanning.InstallScope, error) {
	scope := initplanning.InstallScope(raw)
	if scope != initplanning.ScopeProject &&
		scope != initplanning.ScopeUser {
		return "", fmt.Errorf("init manifest scope %q is invalid", raw)
	}
	return scope, nil
}

func validateP14InitHostManifest(
	repositoryRoot string,
	execution p14InstalledCLIExecutionContext,
	projectRoot string,
	homeRoot string,
	projectID string,
	expected p14InitMatrixHostManifest,
	manifest initplanning.InstallationManifest,
) error {
	components := make([]string, len(manifest.Components()))
	for index, component := range manifest.Components() {
		components[index] = string(component)
	}
	executablePath, err := filepath.EvalSymlinks(execution.ExecutablePath)
	if err != nil {
		return fmt.Errorf("resolve installed executable: %w", err)
	}
	executablePath = filepath.Clean(executablePath)
	if string(manifest.Host()) != expected.Host ||
		string(manifest.Scope()) != expected.Scope ||
		manifest.AdapterEdition() != expected.AdapterEdition ||
		manifest.ProjectRoot() != projectRoot ||
		manifest.ProjectID() != projectID ||
		manifest.ExecutablePath() != executablePath ||
		manifest.ExecutableDigest() != execution.ExecutableDigest ||
		!slices.Equal(components, expected.Components) {
		return fmt.Errorf(
			"%s/%s host manifest identity differs",
			expected.Host,
			expected.Scope,
		)
	}
	bundle, err := buildP14CanonicalSkillBundle(
		repositoryRoot,
		manifest.KernelCatalogDigest(),
	)
	if err != nil {
		return err
	}
	if manifest.SkillBundleDigest() != bundle.Digest() {
		return fmt.Errorf(
			"%s/%s host manifest uses another canonical skill bundle",
			expected.Host,
			expected.Scope,
		)
	}
	targetRoots, err := expectedP14InitTargetRoots(
		projectRoot,
		homeRoot,
		expected,
	)
	if err != nil {
		return err
	}
	if !slices.Equal(manifest.TargetRoots(), targetRoots) {
		return fmt.Errorf(
			"%s/%s target roots = %v, want %v",
			expected.Host,
			expected.Scope,
			manifest.TargetRoots(),
			targetRoots,
		)
	}
	if slices.Contains(expected.Components, "skills") {
		root := targetRoots[0]
		projection, err := buildP14ExpectedSkillProjection(
			expected.Host,
			root,
			bundle,
		)
		if err != nil {
			return err
		}
		if err := validateP14RenderedSkillPaths(
			manifest.RenderedPaths(),
			projection.Outputs(),
		); err != nil {
			return fmt.Errorf(
				"%s/%s rendered skill bundle: %w",
				expected.Host,
				expected.Scope,
				err,
			)
		}
	}
	if !slices.Contains(expected.Components, "skills") &&
		len(manifest.RenderedPaths()) != 0 {
		return fmt.Errorf(
			"%s/%s host manifest has undeclared rendered paths",
			expected.Host,
			expected.Scope,
		)
	}
	managed, err := expectedP14InitManagedFragments(
		repositoryRoot,
		projectRoot,
		projectID,
		expected,
	)
	if err != nil {
		return err
	}
	if err := validateP14InitManagedFragments(
		manifest.ManagedFragments(),
		managed,
	); err != nil {
		return fmt.Errorf(
			"%s/%s managed carriers: %w",
			expected.Host,
			expected.Scope,
			err,
		)
	}
	return nil
}

func expectedP14InitTargetRoots(
	projectRoot string,
	homeRoot string,
	binding p14InitMatrixHostManifest,
) ([]string, error) {
	if binding.Scope == "user" && binding.Host == "claude" {
		return []string{filepath.Join(homeRoot, ".claude", "skills")}, nil
	}
	if binding.Scope == "user" && binding.Host == "codex" {
		return []string{filepath.Join(homeRoot, ".agents", "skills")}, nil
	}
	if binding.Scope == "project" && binding.Host == "claude" {
		return []string{projectRoot}, nil
	}
	if binding.Scope == "project" && binding.Host == "codex" {
		return []string{
			projectRoot,
			filepath.Join(projectRoot, ".codex"),
		}, nil
	}
	return nil, fmt.Errorf(
		"unsupported P14 init binding %s/%s",
		binding.Host,
		binding.Scope,
	)
}

func expectedP14InitManagedFragments(
	repositoryRoot string,
	projectRoot string,
	projectID string,
	binding p14InitMatrixHostManifest,
) ([]p14ExpectedManagedFragment, error) {
	if binding.Scope != "project" {
		return []p14ExpectedManagedFragment{}, nil
	}
	result := make([]p14ExpectedManagedFragment, 0, 2)
	if slices.Contains(binding.Components, "mcp") {
		fragment, tables, err := expectedP14InitMCPFragment(
			projectRoot,
			projectID,
			binding.Host,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, p14ExpectedManagedFragment{
			Fragment:   fragment,
			TOMLTables: tables,
		})
	}
	if slices.Contains(binding.Components, "instructions") {
		templatePath := filepath.Join(
			repositoryRoot,
			"internal",
			"cli",
			"claude_md_template.md",
		)
		template, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("read canonical instruction template: %w", err)
		}
		fileName := "CLAUDE.md"
		if binding.Host == "codex" {
			fileName = "AGENTS.md"
		}
		fragment, err := initplanning.NewHTMLCommentSectionFragment(
			filepath.Join(projectRoot, fileName),
			initplanning.ComponentInstructions,
			"haft",
			template,
			0o644,
			p14InitTextMergeEdition,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, p14ExpectedManagedFragment{
			Fragment: fragment,
		})
	}
	return result, nil
}

func expectedP14InitMCPFragment(
	projectRoot string,
	projectID string,
	host string,
) (initplanning.ManagedFragment, []string, error) {
	if host == "claude" {
		value := map[string]any{
			"command": "haft",
			"args":    []string{"serve"},
			"env": map[string]string{
				"HAFT_PROJECT_ROOT":        "${PWD:-.}",
				"HAFT_EXPECTED_PROJECT_ID": projectID,
			},
		}
		content, err := json.Marshal(value)
		if err != nil {
			return initplanning.ManagedFragment{}, nil, err
		}
		fragment, err := initplanning.NewJSONObjectEntryFragment(
			filepath.Join(projectRoot, ".mcp.json"),
			initplanning.ComponentMCP,
			[]string{"mcpServers", "haft"},
			content,
			0o644,
			p14InitJSONMergeEdition,
		)
		return fragment, nil, err
	}
	if host == "codex" {
		projectIDJSON, err := json.Marshal(projectID)
		if err != nil {
			return initplanning.ManagedFragment{}, nil, err
		}
		content := fmt.Sprintf(`[mcp_servers.haft]
command = "haft"
args = ["serve"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.haft.env]
HAFT_PROJECT_ROOT = "."
HAFT_EXPECTED_PROJECT_ID = %s
`, projectIDJSON)
		tables := []string{
			"mcp_servers.haft",
			"mcp_servers.haft.env",
		}
		fragment, err := initplanning.NewTOMLTableSetFragment(
			filepath.Join(projectRoot, ".codex", "config.toml"),
			initplanning.ComponentMCP,
			"mcp_servers.haft",
			tables,
			[]byte(content),
			0o644,
			p14InitTOMLMergeEdition,
		)
		return fragment, tables, err
	}
	return initplanning.ManagedFragment{}, nil, fmt.Errorf(
		"unsupported P14 MCP host %q",
		host,
	)
}

func validateP14InitManagedFragments(
	observed []initplanning.ManifestFragment,
	expected []p14ExpectedManagedFragment,
) error {
	if len(observed) != len(expected) {
		return fmt.Errorf(
			"managed fragment count = %d, want %d",
			len(observed),
			len(expected),
		)
	}
	remaining := slices.Clone(observed)
	for _, expectation := range expected {
		coordinate := expectation.Fragment.Coordinate()
		index := slices.IndexFunc(
			remaining,
			func(candidate initplanning.ManifestFragment) bool {
				return candidate.CarrierPath == coordinate.CarrierPath() &&
					candidate.Kind == coordinate.Kind() &&
					candidate.Selector == coordinate.Selector() &&
					candidate.MemberID == coordinate.MemberID() &&
					candidate.MergeEdition == coordinate.MergeEdition()
			},
		)
		if index < 0 {
			return fmt.Errorf(
				"managed fragment %s %s is absent",
				coordinate.CarrierPath(),
				coordinate.Selector(),
			)
		}
		candidate := remaining[index]
		if candidate.Component != expectation.Fragment.Component() ||
			candidate.Digest != expectation.Fragment.Digest() ||
			!slices.Equal(candidate.TOMLTables, expectation.TOMLTables) {
			return fmt.Errorf(
				"managed fragment %s %s digest or shape differs",
				coordinate.CarrierPath(),
				coordinate.Selector(),
			)
		}
		if err := observeP14ExpectedManagedFragment(
			expectation.Fragment,
		); err != nil {
			return err
		}
		remaining = slices.Delete(remaining, index, index+1)
	}
	if len(remaining) != 0 {
		return fmt.Errorf("unexpected managed fragments remain")
	}
	return nil
}

func observeP14ExpectedManagedFragment(
	fragment initplanning.ManagedFragment,
) error {
	path := fragment.Coordinate().CarrierPath()
	raw, mode, err := readP14InitRegularFile(path)
	if err != nil {
		return fmt.Errorf("read managed carrier %s: %w", path, err)
	}
	if mode.Perm() != 0o644 {
		return fmt.Errorf("managed carrier %s mode differs", path)
	}
	input, err := initplanning.NewPresentManagedCarrier(
		path,
		raw,
		mode.Perm(),
	)
	if err != nil {
		return err
	}
	plan, err := initplanning.BuildManagedFragmentObservationPlan(
		[]initplanning.ManagedFragment{fragment},
		initplanning.NoPriorManagedFragmentBaseline(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		return err
	}
	observation, err := initplanning.ObserveManagedCarrier(plan, input)
	if err != nil {
		return err
	}
	fragments := observation.Fragments()
	if len(fragments) != 1 ||
		fragments[0].Kind() != initplanning.ManagedFragmentObservedPresent ||
		fragments[0].Digest() != fragment.Digest() {
		return fmt.Errorf("managed carrier %s semantic digest differs", path)
	}
	return nil
}

func buildP14CanonicalSkillBundle(
	repositoryRoot string,
	kernelCatalogDigest string,
) (initplanning.SkillSourceBundle, error) {
	skillRoot := filepath.Join(repositoryRoot, "internal", "cli", "skill")
	entries, err := os.ReadDir(skillRoot)
	if err != nil {
		return initplanning.SkillSourceBundle{}, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return initplanning.SkillSourceBundle{}, fmt.Errorf(
				"canonical skill root contains non-directory %s",
				entry.Name(),
			)
		}
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, p14CanonicalSkillNames) {
		return initplanning.SkillSourceBundle{}, fmt.Errorf(
			"canonical skill inventory = %v, want %v",
			names,
			p14CanonicalSkillNames,
		)
	}
	inputs := make([]initplanning.SkillSourceInput, 0, len(names))
	for _, name := range names {
		content, err := os.ReadFile(
			filepath.Join(skillRoot, name, "SKILL.md"),
		)
		if err != nil {
			return initplanning.SkillSourceBundle{}, err
		}
		frontmatter, err := decodeP14SkillFrontmatter(content)
		if err != nil {
			return initplanning.SkillSourceBundle{}, fmt.Errorf(
				"canonical skill %s: %w",
				name,
				err,
			)
		}
		if frontmatter.Name != name {
			return initplanning.SkillSourceBundle{}, fmt.Errorf(
				"canonical skill directory %s carries name %s",
				name,
				frontmatter.Name,
			)
		}
		policy := initplanning.SkillInvocationImplicitAllowed
		if frontmatter.DisableModelInvocation {
			policy = initplanning.SkillInvocationManualOnly
		}
		inputs = append(inputs, initplanning.SkillSourceInput{
			Name:             name,
			Description:      strings.TrimSpace(frontmatter.Description),
			InvocationPolicy: policy,
			Content:          content,
		})
	}
	return initplanning.BuildSkillSourceBundle(
		p14CanonicalSkillBundleEdition,
		kernelCatalogDigest,
		inputs,
	)
}

func decodeP14SkillFrontmatter(
	content []byte,
) (p14SkillFrontmatter, error) {
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return p14SkillFrontmatter{}, fmt.Errorf("frontmatter start is absent")
	}
	remainder := content[4:]
	end := bytes.Index(remainder, []byte("\n---"))
	if end < 0 {
		return p14SkillFrontmatter{}, fmt.Errorf("frontmatter end is absent")
	}
	frontmatter := p14SkillFrontmatter{}
	if err := yaml.Unmarshal(remainder[:end], &frontmatter); err != nil {
		return p14SkillFrontmatter{}, err
	}
	if strings.TrimSpace(frontmatter.Name) == "" ||
		strings.TrimSpace(frontmatter.Description) == "" {
		return p14SkillFrontmatter{}, fmt.Errorf(
			"frontmatter name or description is empty",
		)
	}
	return frontmatter, nil
}

func buildP14ExpectedSkillProjection(
	host string,
	root string,
	bundle initplanning.SkillSourceBundle,
) (initplanning.SkillComponentProjection, error) {
	if host == "claude" {
		rewrite, err := initplanning.NewSkillRewriteSet(
			"claude.exact-source.v1",
			[]initplanning.SkillRewriteInput{},
		)
		if err != nil {
			return initplanning.SkillComponentProjection{}, err
		}
		renderer, err := initplanning.NewSkillComponentRenderer(
			initplanning.HostClaude,
			"claude.skills.v1",
			[]string{},
			initplanning.SkillPolicyInSourceFrontmatter,
			rewrite,
		)
		if err != nil {
			return initplanning.SkillComponentProjection{}, err
		}
		return renderer.Render(bundle, root)
	}
	if host == "codex" {
		rewrite, err := initplanning.NewSkillRewriteSet(
			"codex.skill-syntax.v1",
			[]initplanning.SkillRewriteInput{
				{From: "/h-", To: "$h-"},
				{
					From: "Slash commands",
					To:   "Explicit skill invocations",
				},
				{
					From: "slash commands",
					To:   "explicit skill invocations",
				},
				{From: "Slash command", To: "Explicit skill"},
				{From: "slash command", To: "explicit skill"},
				{From: "Quint", To: "Haft"},
				{From: "quint", To: "haft"},
			},
		)
		if err != nil {
			return initplanning.SkillComponentProjection{}, err
		}
		renderer, err := initplanning.NewSkillComponentRenderer(
			initplanning.HostCodex,
			"codex.skills.v1",
			[]string{},
			initplanning.SkillPolicyCodexOpenAIYAML,
			rewrite,
		)
		if err != nil {
			return initplanning.SkillComponentProjection{}, err
		}
		return renderer.Render(bundle, root)
	}
	return initplanning.SkillComponentProjection{}, fmt.Errorf(
		"unsupported P14 skill host %q",
		host,
	)
}

func validateP14RenderedSkillPaths(
	observed []initplanning.ManifestPath,
	expected []initplanning.RenderedOutput,
) error {
	if len(observed) != len(expected) {
		return fmt.Errorf(
			"rendered path count = %d, want %d",
			len(observed),
			len(expected),
		)
	}
	for index, output := range expected {
		path := observed[index]
		if path.Path != output.Path() ||
			path.Component != output.Component() ||
			path.Digest != output.Digest() ||
			path.Mode != uint32(output.Mode().Perm()) {
			return fmt.Errorf("rendered path %d identity differs", index)
		}
		content, mode, err := readP14InitRegularFile(path.Path)
		if err != nil {
			return fmt.Errorf("read rendered skill %s: %w", path.Path, err)
		}
		if uint32(mode.Perm()) != path.Mode ||
			!bytes.Equal(content, output.Content()) ||
			p14Digest(content) != path.Digest {
			return fmt.Errorf("rendered skill %s bytes differ", path.Path)
		}
	}
	return nil
}

func validateP14InitAgentSkillsManifestInventory(
	repositoryRoot string,
	projectRoot string,
	homeRoot string,
	projectID string,
	expected bool,
) error {
	paths, err := collectP14InitManifestPaths(
		projectRoot,
		homeRoot,
		"agent-skill-installations",
	)
	if err != nil {
		return err
	}
	if !expected {
		if len(paths) != 0 {
			return fmt.Errorf(
				"unexpected independent agent-skill manifests: %v",
				paths,
			)
		}
		return nil
	}
	expectedPath := filepath.Join(
		homeRoot,
		".haft",
		"agent-skill-installations",
		"agent-skills.user.json",
	)
	if !slices.Equal(paths, []string{expectedPath}) {
		return fmt.Errorf(
			"agent-skill manifest inventory = %v, want %s",
			paths,
			expectedPath,
		)
	}
	raw, mode, err := readP14InitRegularFile(expectedPath)
	if err != nil {
		return err
	}
	if mode.Perm() != 0o644 {
		return fmt.Errorf("agent-skill installation manifest mode differs")
	}
	manifest := p14AgentSkillsManifest{}
	if err := decodeP14InitJSON(
		raw,
		&manifest,
		"agent-skill installation manifest",
	); err != nil {
		return err
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonical) {
		return fmt.Errorf("agent-skill installation manifest is not canonical")
	}
	root := filepath.Join(homeRoot, ".agents", "skills")
	if manifest.Schema != p14AgentSkillManifestSchema ||
		manifest.ProjectRoot != projectRoot ||
		manifest.ProjectID != projectID ||
		manifest.Scope != initplanning.ScopeUser ||
		manifest.Root != root ||
		manifest.AdapterEdition != "codex.skills.v1" {
		return fmt.Errorf("agent-skill installation manifest identity differs")
	}
	bundle, err := buildP14CanonicalSkillBundle(
		repositoryRoot,
		manifest.KernelCatalogDigest,
	)
	if err != nil {
		return err
	}
	if manifest.SkillBundleDigest != bundle.Digest() {
		return fmt.Errorf(
			"agent-skill installation manifest uses another canonical bundle",
		)
	}
	projection, err := buildP14ExpectedSkillProjection(
		"codex",
		root,
		bundle,
	)
	if err != nil {
		return err
	}
	return validateP14RenderedSkillPaths(
		manifest.RenderedPaths,
		projection.Outputs(),
	)
}

func collectP14InitManifestPaths(
	projectRoot string,
	homeRoot string,
	directory string,
) ([]string, error) {
	result := make([]string, 0)
	for _, root := range []string{projectRoot, homeRoot} {
		path := filepath.Join(root, ".haft", directory)
		entries, err := os.ReadDir(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			result = append(result, filepath.Join(path, entry.Name()))
		}
	}
	slices.Sort(result)
	return result, nil
}

func readP14InitRegularFile(path string) ([]byte, fs.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return content, info.Mode().Perm(), nil
}

func validateP14InitExperimentalHostsAbsent(
	projectRoot string,
	homeRoot string,
) error {
	paths := []string{
		filepath.Join(projectRoot, ".cursor", "mcp.json"),
		filepath.Join(projectRoot, ".cursor", "skills"),
		filepath.Join(projectRoot, "opencode.json"),
		filepath.Join(projectRoot, ".opencode", "skills"),
		filepath.Join(projectRoot, ".grok", "config.toml"),
		filepath.Join(projectRoot, ".grok", "skills"),
		filepath.Join(projectRoot, ".pi", "settings.json"),
		filepath.Join(projectRoot, ".haft", "pi", "haft-pi"),
		filepath.Join(projectRoot, "skills"),
		filepath.Join(homeRoot, ".gemini", "config", "mcp_config.json"),
		filepath.Join(homeRoot, ".gemini", "settings.json"),
		filepath.Join(homeRoot, ".gemini", "skills"),
		filepath.Join(homeRoot, ".config", "zed", "settings.json"),
		filepath.Join(homeRoot, ".config", "opencode", "skills"),
		filepath.Join(homeRoot, ".cursor", "skills"),
		filepath.Join(homeRoot, ".grok", "skills"),
		filepath.Join(homeRoot, ".hermes", "config.yaml"),
		filepath.Join(homeRoot, ".haft", "hermes", "skills"),
	}
	for _, path := range paths {
		_, err := os.Lstat(path)
		if err == nil {
			return fmt.Errorf(
				"--all materialized experimental host path %s",
				path,
			)
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect experimental host path %s: %w", path, err)
		}
	}
	return nil
}

func TestP14InitCanonicalSkillProjectionCoversEntireBundle(
	t *testing.T,
) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	kernelDigest := p14TestDigest("init-canonical-kernel")
	bundle, err := buildP14CanonicalSkillBundle(
		repositoryRoot,
		kernelDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	claude, err := buildP14ExpectedSkillProjection(
		"claude",
		filepath.Join(t.TempDir(), ".claude", "skills"),
		bundle,
	)
	if err != nil {
		t.Fatal(err)
	}
	codex, err := buildP14ExpectedSkillProjection(
		"codex",
		filepath.Join(t.TempDir(), ".agents", "skills"),
		bundle,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Skills()) != len(p14CanonicalSkillNames) ||
		len(claude.Outputs()) != len(p14CanonicalSkillNames) ||
		len(codex.Outputs()) != len(p14CanonicalSkillNames)*2 {
		t.Fatalf(
			"skill coverage bundle=%d claude=%d codex=%d",
			len(bundle.Skills()),
			len(claude.Outputs()),
			len(codex.Outputs()),
		)
	}
	claudeManifestPaths := make(
		[]initplanning.ManifestPath,
		len(claude.Outputs()),
	)
	for index, output := range claude.Outputs() {
		if err := os.MkdirAll(filepath.Dir(output.Path()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			output.Path(),
			output.Content(),
			output.Mode(),
		); err != nil {
			t.Fatal(err)
		}
		claudeManifestPaths[index] = initplanning.ManifestPath{
			Path:      output.Path(),
			Component: output.Component(),
			Digest:    output.Digest(),
			Mode:      uint32(output.Mode().Perm()),
		}
	}
	if err := validateP14RenderedSkillPaths(
		claudeManifestPaths,
		claude.Outputs(),
	); err != nil {
		t.Fatal(err)
	}
	tamperedPath := claudeManifestPaths[0].Path
	if err := os.WriteFile(tamperedPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateP14RenderedSkillPaths(
		claudeManifestPaths,
		claude.Outputs(),
	); err == nil {
		t.Fatal("tampered rendered skill passed P14 init validation")
	}
}

func TestP14InitManagedContentRejectsSemanticDrift(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	projectID := "qnt_09afbeef"
	binding := p14InitMatrixHostManifest{
		Host:           "codex",
		Scope:          "project",
		AdapterEdition: "codex.coherent.v2",
		Components:     []string{"instructions", "mcp"},
	}
	expected, err := expectedP14InitManagedFragments(
		repositoryRoot,
		projectRoot,
		projectID,
		binding,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range expected {
		path := item.Fragment.Coordinate().CarrierPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, item.Fragment.Content(), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := observeP14ExpectedManagedFragment(item.Fragment); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(projectRoot, ".codex", "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.Replace(
		content,
		[]byte(`command = "haft"`),
		[]byte(`command = "foreign"`),
		1,
	)
	if err := os.WriteFile(configPath, content, fs.FileMode(0o644)); err != nil {
		t.Fatal(err)
	}
	if err := observeP14ExpectedManagedFragment(
		expected[0].Fragment,
	); err == nil {
		t.Fatal("semantic MCP drift passed the P14 init validator")
	}
}
