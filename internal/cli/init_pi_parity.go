package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/initplanning"
	haftpi "github.com/m0n0x41d/haft/packages/haft-pi"
)

const (
	currentPiAdmissibleUse = "pi_host_orientation_and_kernel_routed_invocation"
	currentPiRenderingName = "host-rendering:pi-package"
)

var currentPiToolNamePattern = regexp.MustCompile(
	`(?m)^\s*name: "(haft_[a-z_]+)",\s*$`,
)

type currentPiCondensedParityReport struct {
	BundleRef           string
	BundleDigest        string
	KernelCatalogDigest string
	PackageDigest       string
	Skills              []string
	ManualSkills        []string
	Tools               []string
	Declaration         initplanning.ControlledCoarseningDeclaration
}

type currentPiPackageDigestEntry struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

type currentPiPackageDigestWire struct {
	Schema string                        `json:"schema"`
	Files  []currentPiPackageDigestEntry `json:"files"`
}

type currentPiSemanticAssertion struct {
	skillMarkers  []string
	promptMarkers []string
}

func currentPiSemanticAssertions() map[string]currentPiSemanticAssertion {
	return map[string]currentPiSemanticAssertion{
		"h-reason": {
			skillMarkers: []string{
				"source-first",
				"persistence is conditional",
				"binding actions remain manual",
				"human gate brief",
			},
			promptMarkers: []string{
				"source-first",
				"inspect the selected pattern's full body",
				"human gate brief",
			},
		},
		"h-frame": {
			skillMarkers: []string{
				"under-articulated",
				"without assuming a solution",
				"explicit save intent",
			},
		},
		"h-diagnose": {
			skillMarkers: []string{
				"rival-hypothesis",
				"discriminating observations",
				"runtime evidence",
			},
		},
		"h-explore": {
			skillMarkers: []string{
				"genuinely distinct",
				"weakest link",
				"persistence is conditional",
			},
		},
		"h-compare": {
			skillMarkers: []string{
				"parity",
				"pareto",
				"direct unambiguous operator request",
			},
		},
		"h-decide": {
			skillMarkers: []string{
				"direct, unambiguous",
				"human gate brief",
				"host_routed_operator_request",
			},
			promptMarkers: []string{
				"direct, unambiguous operator request",
				"human gate brief",
				"host_routed_operator_request",
			},
		},
		"h-commission": {
			skillMarkers: []string{
				"manual-only",
				"human gate brief",
				"explicit operator grant",
			},
			promptMarkers: []string{
				"manual gate",
				"human gate brief",
				"generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts",
			},
		},
		"h-verify": {
			skillMarkers: []string{
				"baseline",
				"current code",
				"runtime evidence",
			},
		},
		"h-status": {
			skillMarkers: []string{
				"read-only",
				"human gate brief",
				"not evidence truth",
			},
			promptMarkers: []string{
				"read-only",
				"human gate brief",
			},
		},
		"h-spec": {
			skillMarkers: []string{
				"typed spec lifecycle",
				"explicit human gates",
				"source compatibility",
				"human gate brief",
			},
		},
		"h-onboard": {
			skillMarkers: []string{
				"detector proposals are read-only",
				"profile and spec gates remain human",
				"not a scopeid",
			},
		},
		"h-note": {
			skillMarkers: []string{
				"explicitly requested non-binding fact",
				"do not auto-persist",
				"not a choice",
			},
		},
	}
}

func currentPiCondensedParity(
	bundle initplanning.SkillSourceBundle,
	packageRoot string,
	outputs []initplanning.RenderedOutput,
) (currentPiCondensedParityReport, error) {
	return verifyPiCondensedParity(
		bundle,
		haftpi.Assets,
		packageRoot,
		outputs,
		currentKernelMCPToolNames(),
	)
}

func verifyPiCondensedParity(
	bundle initplanning.SkillSourceBundle,
	assets fs.FS,
	packageRoot string,
	outputs []initplanning.RenderedOutput,
	expectedTools []string,
) (currentPiCondensedParityReport, error) {
	skills, manuals, err := verifyPiSkillAndPromptCoverage(
		bundle,
		assets,
	)
	if err != nil {
		return currentPiCondensedParityReport{}, err
	}
	tools, err := verifyPiToolCoverage(
		assets,
		expectedTools,
		bundle.KernelCatalogDigest(),
	)
	if err != nil {
		return currentPiCondensedParityReport{}, err
	}
	if err := verifyPiKernelSourcePins(
		assets,
		bundle.KernelCatalogDigest(),
	); err != nil {
		return currentPiCondensedParityReport{}, err
	}
	packageDigest, err := currentPiPackageDigest(
		packageRoot,
		outputs,
	)
	if err != nil {
		return currentPiCondensedParityReport{}, err
	}
	declaration, err := initplanning.NewControlledCoarseningDeclaration(
		initplanning.ControlledCoarseningInput{
			SourceRef:             bundle.Ref(),
			SourceDigest:          bundle.Digest(),
			RenderingRef:          currentPiRenderingName,
			RenderingDigest:       packageDigest,
			NarrowerAdmissibleUse: currentPiAdmissibleUse,
			SourceLossModes: []initplanning.SourceLossMode{
				initplanning.SourceLossOmittedDetail,
				initplanning.SourceLossRecoverability,
				initplanning.SourceLossRepresentationFactor,
			},
			NonAdmissibleUses: []string{
				"binding_authorization",
				"canonical_skill_source",
				"semantic_authority",
				"standalone_release_or_parity_evidence",
			},
			ReopenTriggers: []string{
				"dispute_or_external_reliance",
				"kernel_contract_or_public_skill_set_change",
				"skill_semantics_or_invocation_policy_review",
			},
		},
	)
	if err != nil {
		return currentPiCondensedParityReport{}, err
	}
	return currentPiCondensedParityReport{
		BundleRef:           bundle.Ref(),
		BundleDigest:        bundle.Digest(),
		KernelCatalogDigest: bundle.KernelCatalogDigest(),
		PackageDigest:       packageDigest,
		Skills:              skills,
		ManualSkills:        manuals,
		Tools:               tools,
		Declaration:         declaration,
	}, nil
}

func verifyPiSkillAndPromptCoverage(
	bundle initplanning.SkillSourceBundle,
	assets fs.FS,
) ([]string, []string, error) {
	skillPaths, err := fs.Glob(assets, "skills/*/SKILL.md")
	if err != nil {
		return nil, nil, fmt.Errorf("list Pi skill carriers: %w", err)
	}
	promptPaths, err := fs.Glob(assets, "prompts/h-*.md")
	if err != nil {
		return nil, nil, fmt.Errorf("list Pi prompt carriers: %w", err)
	}
	expectedSkills := make([]string, 0, len(bundle.Skills()))
	manualSkills := make([]string, 0)
	expectedSkillPaths := make([]string, 0, len(bundle.Skills()))
	expectedPromptPaths := make([]string, 0, len(bundle.Skills()))
	assertions := currentPiSemanticAssertions()
	for _, source := range bundle.Skills() {
		name := source.Name()
		expectedSkills = append(expectedSkills, name)
		expectedSkillPaths = append(
			expectedSkillPaths,
			filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")),
		)
		expectedPromptPaths = append(
			expectedPromptPaths,
			filepath.ToSlash(filepath.Join("prompts", name+".md")),
		)
		assertion, declared := assertions[name]
		if !declared {
			return nil, nil, fmt.Errorf(
				"pi condensed parity lacks semantic assertions for %s",
				name,
			)
		}
		skillContent, readErr := fs.ReadFile(
			assets,
			filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")),
		)
		if readErr != nil {
			return nil, nil, fmt.Errorf(
				"read Pi skill %s: %w",
				name,
				readErr,
			)
		}
		promptContent, readErr := fs.ReadFile(
			assets,
			filepath.ToSlash(filepath.Join("prompts", name+".md")),
		)
		if readErr != nil {
			return nil, nil, fmt.Errorf(
				"read Pi prompt %s: %w",
				name,
				readErr,
			)
		}
		frontmatter, parseErr := parseSkillSourceFrontmatter(skillContent)
		if parseErr != nil {
			return nil, nil, fmt.Errorf(
				"parse Pi skill %s frontmatter: %w",
				name,
				parseErr,
			)
		}
		if frontmatter.Name != name {
			return nil, nil, fmt.Errorf(
				"pi skill %s carries another name %s",
				name,
				frontmatter.Name,
			)
		}
		manualMarker := strings.Contains(
			strings.ToLower(frontmatter.Description),
			"manual-only",
		)
		shouldBeManual := source.InvocationPolicy() ==
			initplanning.SkillInvocationManualOnly
		if manualMarker != shouldBeManual {
			return nil, nil, fmt.Errorf(
				"pi skill %s invocation posture differs from canonical bundle",
				name,
			)
		}
		if err := requirePiSemanticMarkers(
			name+" skill",
			skillContent,
			assertion.skillMarkers,
		); err != nil {
			return nil, nil, err
		}
		if err := requirePiSemanticMarkers(
			name+" prompt",
			promptContent,
			assertion.promptMarkers,
		); err != nil {
			return nil, nil, err
		}
		if source.InvocationPolicy() ==
			initplanning.SkillInvocationManualOnly {
			manualSkills = append(manualSkills, name)
		}
	}
	sort.Strings(expectedSkills)
	sort.Strings(manualSkills)
	sort.Strings(expectedSkillPaths)
	sort.Strings(expectedPromptPaths)
	sort.Strings(skillPaths)
	sort.Strings(promptPaths)
	if !slices.Equal(skillPaths, expectedSkillPaths) {
		return nil, nil, fmt.Errorf(
			"pi skill carrier set differs from canonical bundle: got=%v want=%v",
			skillPaths,
			expectedSkillPaths,
		)
	}
	if !slices.Equal(promptPaths, expectedPromptPaths) {
		return nil, nil, fmt.Errorf(
			"pi prompt carrier set differs from canonical bundle: got=%v want=%v",
			promptPaths,
			expectedPromptPaths,
		)
	}
	return expectedSkills, manualSkills, nil
}

func requirePiSemanticMarkers(
	carrier string,
	content []byte,
	markers []string,
) error {
	lower := strings.ToLower(string(content))
	for _, marker := range markers {
		if !strings.Contains(lower, strings.ToLower(marker)) {
			return fmt.Errorf(
				"pi %s lacks semantic parity marker %q",
				carrier,
				marker,
			)
		}
	}
	return nil
}

func verifyPiToolCoverage(
	assets fs.FS,
	expected []string,
	kernelDigest string,
) ([]string, error) {
	content, err := fs.ReadFile(
		assets,
		"extensions/haft/tools.ts",
	)
	if err != nil {
		return nil, fmt.Errorf("read pi tool carrier: %w", err)
	}
	matches := currentPiToolNamePattern.FindAllSubmatch(content, -1)
	tools := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		name := string(match[1])
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("pi tool carrier repeats %s", name)
		}
		seen[name] = struct{}{}
		tools = append(tools, name)
	}
	sort.Strings(tools)
	want := slices.Clone(expected)
	sort.Strings(want)
	if !slices.Equal(tools, want) {
		return nil, fmt.Errorf(
			"pi native tool set differs from kernel catalog: got=%v want=%v",
			tools,
			want,
		)
	}
	if !strings.Contains(string(content), kernelDigest) {
		return nil, fmt.Errorf(
			"pi tool carrier lacks current kernel catalog digest",
		)
	}
	return tools, nil
}

func verifyPiKernelSourcePins(
	assets fs.FS,
	kernelDigest string,
) error {
	paths := []string{
		"skills/h-reason/SKILL.md",
		"skills/h-status/SKILL.md",
		"prompts/h-reason.md",
		"prompts/h-decide.md",
		"prompts/h-commission.md",
	}
	for _, path := range paths {
		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return fmt.Errorf("read pi source-pinned carrier %s: %w", path, err)
		}
		if !strings.Contains(string(content), kernelDigest) {
			return fmt.Errorf(
				"pi source-pinned carrier %s lacks current kernel catalog digest",
				path,
			)
		}
	}
	return nil
}

func currentKernelMCPToolNames() []string {
	server := fpf.NewServer("pi-parity")
	server.SetV5Handler(
		func(
			context.Context,
			string,
			json.RawMessage,
		) (string, error) {
			return "", nil
		},
	)
	memoryHandler := func(
		context.Context,
		json.RawMessage,
	) (string, error) {
		return "", nil
	}
	server.SetMemoryFullHandler(memoryHandler)
	server.SetMemoryReadHandler(memoryHandler)
	catalog := server.ToolCatalog()
	result := make([]string, len(catalog))
	for index, tool := range catalog {
		result[index] = tool.Name
	}
	sort.Strings(result)
	return result
}

func currentPiPackageDigest(
	packageRoot string,
	outputs []initplanning.RenderedOutput,
) (string, error) {
	if len(outputs) == 0 {
		return "", fmt.Errorf("pi package rendering has no outputs")
	}
	root, err := filepath.Abs(packageRoot)
	if err != nil {
		return "", fmt.Errorf("resolve pi package root: %w", err)
	}
	root = filepath.Clean(root)
	entries := make(
		[]currentPiPackageDigestEntry,
		0,
		len(outputs),
	)
	for _, output := range outputs {
		if output.Component() != initplanning.ComponentPackage {
			return "", fmt.Errorf(
				"pi package digest includes non-package output %s",
				output.Path(),
			)
		}
		if output.Mode().Perm() != 0o644 {
			return "", fmt.Errorf(
				"pi package output has non-canonical mode: %s",
				output.Path(),
			)
		}
		relative, err := filepath.Rel(root, output.Path())
		if err != nil ||
			relative == "." ||
			filepath.IsAbs(relative) ||
			relative == ".." ||
			strings.HasPrefix(
				relative,
				".."+string(filepath.Separator),
			) {
			return "", fmt.Errorf(
				"pi package output is outside package root: %s",
				output.Path(),
			)
		}
		entries = append(
			entries,
			currentPiPackageDigestEntry{
				Path:   filepath.ToSlash(relative),
				Digest: output.Digest(),
				Mode:   uint32(output.Mode().Perm()),
			},
		)
	}
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Path < entries[right].Path
	})
	for index := 1; index < len(entries); index++ {
		if entries[index-1].Path == entries[index].Path {
			return "", fmt.Errorf(
				"pi package digest repeats path %s",
				entries[index].Path,
			)
		}
	}
	wire := currentPiPackageDigestWire{
		Schema: "haft.pi-package-rendering/v1",
		Files:  entries,
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return "", fmt.Errorf("encode pi package digest: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", digest), nil
}

func piPackageOutputsFromFS(
	assets fs.FS,
	packageRoot string,
) ([]initplanning.RenderedOutput, error) {
	outputs := make([]initplanning.RenderedOutput, 0)
	err := fs.WalkDir(
		assets,
		".",
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			content, err := fs.ReadFile(assets, path)
			if err != nil {
				return err
			}
			target := filepath.Join(
				packageRoot,
				filepath.FromSlash(path),
			)
			output, err := initplanning.NewRenderedOutput(
				target,
				initplanning.ComponentPackage,
				content,
				0o644,
			)
			if err != nil {
				return err
			}
			outputs = append(outputs, output)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("render embedded Pi package: %w", err)
	}
	sort.Slice(outputs, func(left int, right int) bool {
		return outputs[left].Path() < outputs[right].Path()
	})
	return outputs, nil
}
