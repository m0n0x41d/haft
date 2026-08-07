package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	p14InitMatrixBuilderID            = "init.profile-matrix.v1"
	p14InitMatrixFixtureSchema        = "haft.p14.init-matrix-fixture/v4"
	p14InitMatrixSemanticSchema       = "haft.p14.init-matrix-semantic/v6"
	p14InitMatrixCLISurfaceSchema     = "haft.p14.init-matrix-cli/v3"
	p14InitMatrixNormalizedSchema     = "haft.p14.init-matrix-output/v4"
	p14InitMatrixLocalOracleSchema    = "haft.p14.init-matrix-local-oracle/v5"
	p14InitMatrixNormalizationID      = "p14.init.semantic-host-effects.v4"
	p14InitMatrixBindingGroup         = "init_matrix"
	p14InitMatrixScenarioID           = "fresh_initialization"
	p14InitMatrixLocalOracleTestRef   = "github.com/m0n0x41d/haft/internal/cli::TestTypedPublicCorePlanAndEffectInstallOnlyRequiredProfileCapabilities"
	p14InitLegacyCleanupOracleTestRef = "github.com/m0n0x41d/haft/internal/cli::TestTypedPublicLegacyCommandCleanupRemovesExactFilesAndPreservesForeignEntries"
	p14InitLegacyFixtureOracleTestRef = "github.com/m0n0x41d/haft/internal/p14acceptance::TestP14InitLegacyCommandFixturesExerciseExistingSixCaseMatrix"
	p14InitSupplementalProfileTestRef = "github.com/m0n0x41d/haft/internal/cli::TestOnboardProfileMatrixRunsReviewThroughPublicApplyCommand"
	p14InitSupplementalProfileBasis   = "p13_exact_source_test_receipt"
	p14InitMatrixPreparation          = "restore_templates_to_bound_execution_roots_then_execute_once"
)

type p14InitMatrixFixture struct {
	Schema string                     `json:"schema"`
	Cases  []p14InitMatrixFixtureCase `json:"cases"`
}

type p14InitMatrixFixtureCase struct {
	ID                    string `json:"id"`
	TemplateKind          string `json:"template_kind"`
	ProjectTemplateRoot   string `json:"project_template_root"`
	ProjectTemplateDigest string `json:"project_template_digest"`
	HomeTemplateRoot      string `json:"home_template_root"`
	HomeTemplateDigest    string `json:"home_template_digest"`
	ProjectExecutionRoot  string `json:"project_execution_root"`
	HomeExecutionRoot     string `json:"home_execution_root"`
}

type p14InitMatrixCasePolicy struct {
	ID                      string
	TemplateKind            string
	Argv                    []string
	Outcome                 string
	Applicability           string
	RequiredCarriers        []string
	ForbiddenCarriers       []string
	HostManifests           []p14InitMatrixHostManifest
	RemovedLegacyCommands   []string
	IndependentAgentSkills  bool
	ForbidExperimentalHosts bool
}

type p14InitMatrixHostManifest struct {
	Host           string   `json:"host"`
	Scope          string   `json:"scope"`
	AdapterEdition string   `json:"adapter_edition"`
	Components     []string `json:"components"`
}

func cloneP14InitMatrixHostManifests(
	source []p14InitMatrixHostManifest,
) []p14InitMatrixHostManifest {
	result := make([]p14InitMatrixHostManifest, len(source))
	for index, manifest := range source {
		result[index] = manifest
		result[index].Components = slices.Clone(manifest.Components)
	}
	return result
}

func cloneP14InitSupplementalProfiles(
	source []p14InitSupplementalProfilePolicy,
) []p14InitSupplementalProfilePolicy {
	result := make([]p14InitSupplementalProfilePolicy, len(source))
	for index, profile := range source {
		result[index] = profile
		result[index].MarkerPaths = slices.Clone(profile.MarkerPaths)
		result[index].ApplyArgv = slices.Clone(profile.ApplyArgv)
		result[index].Scopes = make(
			[]p14InitSupplementalProfileScope,
			len(profile.Scopes),
		)
		for scopeIndex, scope := range profile.Scopes {
			result[index].Scopes[scopeIndex] = scope
			result[index].Scopes[scopeIndex].EvidencePaths =
				slices.Clone(scope.EvidencePaths)
		}
	}
	return result
}

func p14InitSupplementalProfilePolicies() []p14InitSupplementalProfilePolicy {
	automatic := func(
		id string,
		markers []string,
		scopes []p14InitSupplementalProfileScope,
	) p14InitSupplementalProfilePolicy {
		return p14InitSupplementalProfilePolicy{
			ID:                 id,
			MarkerPaths:        slices.Clone(markers),
			InitialResult:      "profile_review_prepared",
			PreparationKind:    "automatic_detector_advisory",
			PreparedResult:     "profile_review_prepared",
			PreparedStatus:     "profile_review_ready",
			Scopes:             scopes,
			ApplyArgv:          []string{"onboard", "profile", "apply", "--json"},
			ApplyResult:        "profile_applied",
			ApplyStatus:        "needs_memory",
			ReplayDelivery:     "reused",
			EvidenceBasis:      p14InitSupplementalProfileBasis,
			ExactSourceTestRef: p14InitSupplementalProfileTestRef,
		}
	}
	manual := func(
		id string,
		markers []string,
		evidence []string,
	) p14InitSupplementalProfilePolicy {
		return p14InitSupplementalProfilePolicy{
			ID:              id,
			MarkerPaths:     slices.Clone(markers),
			InitialResult:   "needs_scope_review",
			PreparationKind: "explicit_manual_software_scope",
			PreparedResult:  "profile_review_prepared",
			PreparedStatus:  "profile_review_ready",
			Scopes: []p14InitSupplementalProfileScope{{
				RealizationKind: "software",
				EvidencePaths:   slices.Clone(evidence),
			}},
			ApplyArgv:          []string{"onboard", "profile", "apply", "--json"},
			ApplyResult:        "profile_applied",
			ApplyStatus:        "needs_memory",
			ReplayDelivery:     "reused",
			EvidenceBasis:      p14InitSupplementalProfileBasis,
			ExactSourceTestRef: p14InitSupplementalProfileTestRef,
		}
	}
	software := []p14InitSupplementalProfileScope{{
		RealizationKind: "software",
		EvidencePaths:   []string{},
	}}
	return []p14InitSupplementalProfilePolicy{
		automatic(
			"typescript",
			[]string{"package.json", "src/index.ts"},
			software,
		),
		automatic(
			"python",
			[]string{"pyproject.toml", "src/app.py"},
			software,
		),
		automatic(
			"rust",
			[]string{"Cargo.toml", "src/main.rs"},
			software,
		),
		manual(
			"zig_manual_fallback",
			[]string{"build.zig", "src/main.zig"},
			[]string{"build.zig"},
		),
		manual(
			"elixir_manual_fallback",
			[]string{"mix.exs", "lib/app.ex"},
			[]string{"mix.exs"},
		),
		manual(
			"dart_manual_fallback",
			[]string{"pubspec.yaml", "lib/main.dart"},
			[]string{"pubspec.yaml"},
		),
		automatic(
			"docs_only",
			[]string{"mkdocs.yml", "docs/intro.md", "docs/usage.md"},
			[]p14InitSupplementalProfileScope{{
				RealizationKind: "non_software",
				EvidencePaths:   []string{},
			}},
		),
		automatic(
			"mixed_software_and_model",
			[]string{"package.json", "src/index.ts", "models/current.onnx"},
			[]p14InitSupplementalProfileScope{
				{
					RealizationKind: "non_software",
					EvidencePaths:   []string{},
				},
				{
					RealizationKind: "software",
					EvidencePaths:   []string{},
				},
			},
		),
		manual(
			"empty_manual_fallback",
			[]string{},
			[]string{},
		),
	}
}

type p14InitMatrixSemanticRequest struct {
	Schema               string                             `json:"schema"`
	Fixture              p14InitMatrixFixture               `json:"fixture"`
	Cases                []p14InitMatrixSemanticCase        `json:"cases"`
	SupplementalProfiles []p14InitSupplementalProfilePolicy `json:"supplemental_profiles"`
}

type p14InitSupplementalProfilePolicy struct {
	ID                 string                            `json:"id"`
	MarkerPaths        []string                          `json:"marker_paths"`
	InitialResult      string                            `json:"initial_result"`
	PreparationKind    string                            `json:"preparation_kind"`
	PreparedResult     string                            `json:"prepared_result"`
	PreparedStatus     string                            `json:"prepared_status"`
	Scopes             []p14InitSupplementalProfileScope `json:"scopes"`
	ApplyArgv          []string                          `json:"apply_argv"`
	ApplyResult        string                            `json:"apply_result"`
	ApplyStatus        string                            `json:"apply_status"`
	ReplayDelivery     string                            `json:"replay_delivery"`
	EvidenceBasis      string                            `json:"evidence_basis"`
	ExactSourceTestRef string                            `json:"exact_source_test_ref"`
}

type p14InitSupplementalProfileScope struct {
	RealizationKind string   `json:"realization_kind"`
	EvidencePaths   []string `json:"evidence_paths"`
}

type p14InitMatrixSemanticCase struct {
	ID                      string                      `json:"id"`
	TemplateKind            string                      `json:"template_kind"`
	Argv                    []string                    `json:"argv"`
	Outcome                 string                      `json:"outcome"`
	Applicability           string                      `json:"applicability"`
	RequiredCarriers        []string                    `json:"required_carriers"`
	ForbiddenCarriers       []string                    `json:"forbidden_carriers"`
	HostManifests           []p14InitMatrixHostManifest `json:"host_manifests"`
	RemovedLegacyCommands   []string                    `json:"removed_legacy_commands"`
	IndependentAgentSkills  bool                        `json:"independent_agent_skills"`
	ForbidExperimentalHosts bool                        `json:"forbid_experimental_hosts"`
}

type p14InitMatrixCLISurface struct {
	Schema                string                     `json:"schema"`
	SemanticRequestDigest string                     `json:"semantic_request_digest"`
	Cases                 []p14InitMatrixCLICallCase `json:"cases"`
}

type p14InitMatrixCLICallCase struct {
	ID                    string   `json:"id"`
	ProjectTemplateRoot   string   `json:"project_template_root"`
	ProjectTemplateDigest string   `json:"project_template_digest"`
	HomeTemplateRoot      string   `json:"home_template_root"`
	HomeTemplateDigest    string   `json:"home_template_digest"`
	ProjectExecutionRoot  string   `json:"project_execution_root"`
	HomeExecutionRoot     string   `json:"home_execution_root"`
	Preparation           string   `json:"preparation"`
	Argv                  []string `json:"argv"`
}

type p14InitMatrixNormalizedOutput struct {
	Schema string                              `json:"schema"`
	Cases  []p14InitMatrixNormalizedCaseOutput `json:"cases"`
}

type p14InitMatrixNormalizedCaseOutput struct {
	ID                    string   `json:"id"`
	Outcome               string   `json:"outcome"`
	Applicability         string   `json:"applicability"`
	RequiredCarriers      []string `json:"required_carriers"`
	ForbiddenCarriers     []string `json:"forbidden_carriers"`
	RemovedLegacyCommands []string `json:"removed_legacy_commands"`
}

type p14InitMatrixLocalOracle struct {
	Schema                     string   `json:"schema"`
	FixtureDigest              string   `json:"fixture_digest"`
	SemanticRequestDigest      string   `json:"semantic_request_digest"`
	ExpectedResultDigest       string   `json:"expected_result_digest"`
	SupplementalProfilesDigest string   `json:"supplemental_profiles_digest"`
	SupplementalEvidenceBasis  string   `json:"supplemental_evidence_basis"`
	LocalOracleTests           []string `json:"local_oracle_tests"`
}

type p14InitTreeEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Digest string `json:"digest,omitempty"`
}

type p14InitLegacyCommandFixture struct {
	Path      string
	Content   string
	Host      string
	Removable bool
}

func p14InitLegacyCommandFixtures(
	projectRoot string,
	homeRoot string,
) []p14InitLegacyCommandFixture {
	return []p14InitLegacyCommandFixture{
		{
			Path: filepath.Join(
				homeRoot,
				".claude",
				"commands",
				"h-frame.md",
			),
			Content:   "legacy Claude command\n",
			Host:      "claude",
			Removable: true,
		},
		{
			Path: filepath.Join(
				homeRoot,
				".codex",
				"prompts",
				"h-frame.md",
			),
			Content:   "legacy Codex prompt\n",
			Host:      "codex",
			Removable: true,
		},
		{
			Path: filepath.Join(
				projectRoot,
				".claude",
				"commands",
				"h-frame.md",
			),
			Content: "wrong-scope legacy Claude command\n",
		},
		{
			Path: filepath.Join(
				homeRoot,
				".claude",
				"commands",
				"custom.md",
			),
			Content: "foreign Claude command\n",
		},
		{
			Path: filepath.Join(
				homeRoot,
				".claude",
				"commands",
				"h-frame.txt",
			),
			Content: "foreign Claude text file\n",
		},
		{
			Path: filepath.Join(
				homeRoot,
				".codex",
				"prompts",
				"custom.md",
			),
			Content: "foreign Codex prompt\n",
		},
		{
			Path: filepath.Join(
				homeRoot,
				".codex",
				"prompts",
				"h-frame.md.bak",
			),
			Content: "foreign Codex backup\n",
		},
		{
			Path: filepath.Join(
				homeRoot,
				".codex",
				"prompts",
				"nested",
				"h-frame.md",
			),
			Content: "nested Codex prompt\n",
		},
	}
}

func p14InitMatrixPolicies() []p14InitMatrixCasePolicy {
	profileCarriers := []string{
		"spec:software-system",
		"spec:term-map",
		"method:swe-core",
	}
	hostCarriers := []string{
		"host:claude_mcp",
		"host:claude_instructions",
		"skills:claude_bundle",
		"host:codex_mcp",
		"host:codex_instructions",
		"skills:agents_bundle",
	}
	forbidden := func(values ...string) []string {
		result := slices.Clone(profileCarriers)
		result = append(result, values...)
		return result
	}
	claudeProject := p14InitMatrixHostManifest{
		Host:           "claude",
		Scope:          "project",
		AdapterEdition: "claude.coherent.v2",
		Components:     []string{"instructions", "mcp"},
	}
	claudeUser := p14InitMatrixHostManifest{
		Host:           "claude",
		Scope:          "user",
		AdapterEdition: "claude.skills.v1",
		Components:     []string{"skills"},
	}
	codexProject := p14InitMatrixHostManifest{
		Host:           "codex",
		Scope:          "project",
		AdapterEdition: "codex.coherent.v2",
		Components:     []string{"instructions", "mcp"},
	}
	codexMCPOnly := p14InitMatrixHostManifest{
		Host:           "codex",
		Scope:          "project",
		AdapterEdition: "codex.coherent.v2",
		Components:     []string{"mcp"},
	}
	codexUser := p14InitMatrixHostManifest{
		Host:           "codex",
		Scope:          "user",
		AdapterEdition: "codex.skills.v1",
		Components:     []string{"skills"},
	}
	return []p14InitMatrixCasePolicy{
		{
			ID:            "core_only",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--core-only"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
			},
			ForbiddenCarriers: forbidden(hostCarriers...),
			HostManifests:     []p14InitMatrixHostManifest{},
		},
		{
			ID:            "claude_full",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--claude"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
				"host:claude_mcp",
				"host:claude_instructions",
				"skills:claude_bundle",
			},
			ForbiddenCarriers: forbidden(
				"host:codex_mcp",
				"host:codex_instructions",
				"skills:agents_bundle",
			),
			HostManifests: []p14InitMatrixHostManifest{
				claudeProject,
				claudeUser,
			},
			RemovedLegacyCommands: []string{"claude"},
		},
		{
			ID:            "codex_full",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--codex"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
				"host:codex_mcp",
				"host:codex_instructions",
				"skills:agents_bundle",
			},
			ForbiddenCarriers: forbidden(
				"host:claude_mcp",
				"host:claude_instructions",
				"skills:claude_bundle",
			),
			HostManifests: []p14InitMatrixHostManifest{
				codexProject,
				codexUser,
			},
			RemovedLegacyCommands: []string{"codex"},
		},
		{
			ID:            "codex_mcp_only",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--codex", "--mcp-only"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
				"host:codex_mcp",
			},
			ForbiddenCarriers: forbidden(
				"host:claude_mcp",
				"host:claude_instructions",
				"skills:claude_bundle",
				"host:codex_instructions",
				"skills:agents_bundle",
			),
			HostManifests: []p14InitMatrixHostManifest{
				codexMCPOnly,
			},
		},
		{
			ID:            "agents_skills_only",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--agents"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
				"skills:agents_bundle",
			},
			ForbiddenCarriers: forbidden(
				"host:claude_mcp",
				"host:claude_instructions",
				"skills:claude_bundle",
				"host:codex_mcp",
				"host:codex_instructions",
			),
			HostManifests:          []p14InitMatrixHostManifest{},
			IndependentAgentSkills: true,
		},
		{
			ID:            "all_stable_hosts",
			TemplateKind:  "profile_unavailable",
			Argv:          []string{"init", "--all"},
			Outcome:       "initialized",
			Applicability: "profile_underdetermined",
			RequiredCarriers: []string{
				"core:project_identity",
				"host:claude_mcp",
				"host:claude_instructions",
				"skills:claude_bundle",
				"host:codex_mcp",
				"host:codex_instructions",
				"skills:agents_bundle",
			},
			ForbiddenCarriers: slices.Clone(profileCarriers),
			HostManifests: []p14InitMatrixHostManifest{
				claudeProject,
				claudeUser,
				codexProject,
				codexUser,
			},
			RemovedLegacyCommands:   []string{"claude", "codex"},
			ForbidExperimentalHosts: true,
		},
	}
}

func buildP14InitMatrixScenario(
	declared scenarioContract,
	fixture p14InitMatrixFixture,
) (preparedP14Scenario, error) {
	if err := validateP14InitMatrixContract(declared); err != nil {
		return preparedP14Scenario{}, err
	}
	if err := validateP14InitMatrixFixtureShape(fixture); err != nil {
		return preparedP14Scenario{}, err
	}
	policies := p14InitMatrixPolicies()
	semanticCases := make([]p14InitMatrixSemanticCase, 0, len(policies))
	callCases := make([]p14InitMatrixCLICallCase, 0, len(policies))
	normalizedCases := make([]p14InitMatrixNormalizedCaseOutput, 0, len(policies))
	for index, policy := range policies {
		fixtureCase := fixture.Cases[index]
		semanticCases = append(semanticCases, p14InitMatrixSemanticCase{
			ID:                policy.ID,
			TemplateKind:      policy.TemplateKind,
			Argv:              slices.Clone(policy.Argv),
			Outcome:           policy.Outcome,
			Applicability:     policy.Applicability,
			RequiredCarriers:  slices.Clone(policy.RequiredCarriers),
			ForbiddenCarriers: slices.Clone(policy.ForbiddenCarriers),
			HostManifests: cloneP14InitMatrixHostManifests(
				policy.HostManifests,
			),
			RemovedLegacyCommands: slices.Clone(
				policy.RemovedLegacyCommands,
			),
			IndependentAgentSkills:  policy.IndependentAgentSkills,
			ForbidExperimentalHosts: policy.ForbidExperimentalHosts,
		})
		callCases = append(callCases, p14InitMatrixCLICallCase{
			ID:                    policy.ID,
			ProjectTemplateRoot:   fixtureCase.ProjectTemplateRoot,
			ProjectTemplateDigest: fixtureCase.ProjectTemplateDigest,
			HomeTemplateRoot:      fixtureCase.HomeTemplateRoot,
			HomeTemplateDigest:    fixtureCase.HomeTemplateDigest,
			ProjectExecutionRoot:  fixtureCase.ProjectExecutionRoot,
			HomeExecutionRoot:     fixtureCase.HomeExecutionRoot,
			Preparation:           p14InitMatrixPreparation,
			Argv:                  slices.Clone(policy.Argv),
		})
		normalizedCases = append(normalizedCases, p14InitMatrixNormalizedCaseOutput{
			ID:                policy.ID,
			Outcome:           policy.Outcome,
			Applicability:     policy.Applicability,
			RequiredCarriers:  slices.Clone(policy.RequiredCarriers),
			ForbiddenCarriers: slices.Clone(policy.ForbiddenCarriers),
			RemovedLegacyCommands: slices.Clone(
				policy.RemovedLegacyCommands,
			),
		})
	}
	semantic := p14InitMatrixSemanticRequest{
		Schema:  p14InitMatrixSemanticSchema,
		Fixture: fixture,
		Cases:   semanticCases,
		SupplementalProfiles: cloneP14InitSupplementalProfiles(
			p14InitSupplementalProfilePolicies(),
		),
	}
	semanticBytes, err := marshalP14CanonicalJSON(semantic)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	semanticDigest := p14Digest(semanticBytes)
	surface := p14InitMatrixCLISurface{
		Schema:                p14InitMatrixCLISurfaceSchema,
		SemanticRequestDigest: semanticDigest,
		Cases:                 callCases,
	}
	surfaceBytes, err := marshalP14CanonicalJSON(surface)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	normalized := p14InitMatrixNormalizedOutput{
		Schema: p14InitMatrixNormalizedSchema,
		Cases:  normalizedCases,
	}
	normalizedBytes, err := marshalP14CanonicalJSON(normalized)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	fixtureBytes, err := marshalP14CanonicalJSON(fixture)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	supplementalBytes, err := marshalP14CanonicalJSON(
		semantic.SupplementalProfiles,
	)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	localOracle := p14InitMatrixLocalOracle{
		Schema:                     p14InitMatrixLocalOracleSchema,
		FixtureDigest:              p14Digest(fixtureBytes),
		SemanticRequestDigest:      semanticDigest,
		ExpectedResultDigest:       p14Digest(normalizedBytes),
		SupplementalProfilesDigest: p14Digest(supplementalBytes),
		SupplementalEvidenceBasis:  p14InitSupplementalProfileBasis,
		LocalOracleTests:           slices.Clone(declared.LocalOracleTests),
	}
	localOracleBytes, err := marshalP14CanonicalJSON(localOracle)
	if err != nil {
		return preparedP14Scenario{}, err
	}
	return preparedP14Scenario{
		ID:                       declared.ID,
		SemanticRequestCanonical: string(semanticBytes),
		SemanticRequestDigest:    semanticDigest,
		Requests: []preparedP14Request{
			{
				Surface:               "installed_cli",
				Builder:               declared.RequestBuilder,
				Encoding:              "argv_json",
				CanonicalPayload:      string(surfaceBytes),
				PayloadDigest:         p14Digest(surfaceBytes),
				SemanticRequestDigest: semanticDigest,
			},
		},
		Oracle: preparedP14Oracle{
			Kind:                    declared.OracleKind,
			NormalizationID:         p14InitMatrixNormalizationID,
			ExpectedResultDigest:    p14Digest(normalizedBytes),
			ExpectedEffect:          declared.ExpectedEffect,
			LocalOracleOutputDigest: p14Digest(localOracleBytes),
		},
	}, nil
}

func validateP14InitMatrixContract(declared scenarioContract) error {
	if declared.ID != p14InitMatrixScenarioID ||
		declared.RequestBuilder != p14InitMatrixBuilderID ||
		!slices.Equal(declared.Surfaces, []string{"installed_cli"}) ||
		declared.OracleKind != "normalized_digest" ||
		declared.ExpectedEffect != "fixture_write" ||
		!slices.Contains(declared.RequiredBindings, p14InitMatrixBindingGroup) ||
		!slices.Contains(
			declared.LocalOracleTests,
			p14InitMatrixLocalOracleTestRef,
		) ||
		!slices.Contains(
			declared.LocalOracleTests,
			p14InitLegacyCleanupOracleTestRef,
		) ||
		!slices.Contains(
			declared.LocalOracleTests,
			p14InitLegacyFixtureOracleTestRef,
		) ||
		!slices.Contains(
			declared.LocalOracleTests,
			p14InitSupplementalProfileTestRef,
		) {
		return fmt.Errorf("P14 init-matrix contract differs")
	}
	return nil
}

func validateP14InitMatrixFixtureShape(fixture p14InitMatrixFixture) error {
	policies := p14InitMatrixPolicies()
	if fixture.Schema != p14InitMatrixFixtureSchema ||
		len(fixture.Cases) != len(policies) {
		return fmt.Errorf("P14 init-matrix fixture shape differs")
	}
	seenProjectRoots := make(map[string]struct{}, len(fixture.Cases))
	seenHomeRoots := make(map[string]struct{}, len(fixture.Cases))
	seenExecutionRoots := make(map[string]struct{}, len(fixture.Cases)*2)
	for index, fixtureCase := range fixture.Cases {
		policy := policies[index]
		if fixtureCase.ID != policy.ID ||
			fixtureCase.TemplateKind != policy.TemplateKind ||
			!filepath.IsAbs(fixtureCase.ProjectTemplateRoot) ||
			!filepath.IsAbs(fixtureCase.HomeTemplateRoot) ||
			!filepath.IsAbs(fixtureCase.ProjectExecutionRoot) ||
			!filepath.IsAbs(fixtureCase.HomeExecutionRoot) ||
			fixtureCase.ProjectTemplateRoot == fixtureCase.HomeTemplateRoot ||
			fixtureCase.ProjectExecutionRoot == fixtureCase.HomeExecutionRoot ||
			fixtureCase.ProjectTemplateRoot == fixtureCase.ProjectExecutionRoot ||
			fixtureCase.HomeTemplateRoot == fixtureCase.HomeExecutionRoot ||
			!validP14Digest(fixtureCase.ProjectTemplateDigest) ||
			!validP14Digest(fixtureCase.HomeTemplateDigest) {
			return fmt.Errorf("P14 init-matrix fixture case %q is invalid", fixtureCase.ID)
		}
		if _, duplicate := seenProjectRoots[fixtureCase.ProjectTemplateRoot]; duplicate {
			return fmt.Errorf("P14 init-matrix repeats a project template root")
		}
		if _, duplicate := seenHomeRoots[fixtureCase.HomeTemplateRoot]; duplicate {
			return fmt.Errorf("P14 init-matrix repeats a home template root")
		}
		seenProjectRoots[fixtureCase.ProjectTemplateRoot] = struct{}{}
		seenHomeRoots[fixtureCase.HomeTemplateRoot] = struct{}{}
		executionRoots := []string{
			fixtureCase.ProjectExecutionRoot,
			fixtureCase.HomeExecutionRoot,
		}
		for _, executionRoot := range executionRoots {
			if _, duplicate := seenExecutionRoots[executionRoot]; duplicate {
				return fmt.Errorf("P14 init-matrix repeats an execution root")
			}
			seenExecutionRoots[executionRoot] = struct{}{}
		}
	}
	return nil
}

func validateP14InitMatrixPreparedScenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	semantic, err := decodeP14InitMatrixSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	expected, err := buildP14InitMatrixScenario(declared, semantic.Fixture)
	if err != nil {
		return err
	}
	expectedBytes, err := marshalP14CanonicalJSON(expected)
	if err != nil {
		return err
	}
	actualBytes, err := marshalP14CanonicalJSON(scenario)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("P14 init-matrix prepared scenario differs")
	}
	return nil
}

func decodeP14InitMatrixSemantic(
	raw []byte,
) (p14InitMatrixSemanticRequest, error) {
	semantic := p14InitMatrixSemanticRequest{}
	if err := decodeP14InitJSON(raw, &semantic, "semantic request"); err != nil {
		return p14InitMatrixSemanticRequest{}, err
	}
	if semantic.Schema != p14InitMatrixSemanticSchema {
		return p14InitMatrixSemanticRequest{}, fmt.Errorf(
			"P14 init-matrix semantic schema differs",
		)
	}
	return semantic, nil
}

func decodeP14InitMatrixFixture(raw []byte) (p14InitMatrixFixture, error) {
	fixture := p14InitMatrixFixture{}
	if err := decodeP14InitJSON(raw, &fixture, "fixture"); err != nil {
		return p14InitMatrixFixture{}, err
	}
	canonical, err := marshalP14CanonicalJSON(fixture)
	if err != nil {
		return p14InitMatrixFixture{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return p14InitMatrixFixture{}, fmt.Errorf(
			"P14 init-matrix fixture is not canonical JSON",
		)
	}
	return fixture, nil
}

func decodeP14InitJSON(raw []byte, target any, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode P14 init-matrix %s: %w", label, err)
	}
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return fmt.Errorf("P14 init-matrix %s has trailing JSON", label)
	}
	return nil
}

func verifyP14InitMatrixFixtureBinding(
	repositoryRoot string,
	input preparedRequestOracleInput,
) error {
	binding, err := preparedP14BindingByGroup(
		input.Bindings,
		p14InitMatrixBindingGroup,
	)
	if err != nil {
		return err
	}
	path := filepath.Join(repositoryRoot, filepath.FromSlash(binding.CarrierPath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 init-matrix fixture: %w", err)
	}
	if p14Digest(raw) != binding.CarrierDigest {
		return fmt.Errorf("P14 init-matrix fixture digest differs")
	}
	fixture, err := decodeP14InitMatrixFixture(raw)
	if err != nil {
		return err
	}
	if err := validateP14InitMatrixFixtureShape(fixture); err != nil {
		return err
	}
	scenario, err := preparedP14ScenarioByID(
		input.Scenarios,
		p14InitMatrixScenarioID,
	)
	if err != nil {
		return err
	}
	semantic, err := decodeP14InitMatrixSemantic(
		[]byte(scenario.SemanticRequestCanonical),
	)
	if err != nil {
		return err
	}
	fixtureBytes, err := marshalP14CanonicalJSON(fixture)
	if err != nil {
		return err
	}
	embeddedBytes, err := marshalP14CanonicalJSON(semantic.Fixture)
	if err != nil {
		return err
	}
	if !bytes.Equal(fixtureBytes, embeddedBytes) {
		return fmt.Errorf("P14 init-matrix scenario uses another fixture")
	}
	for _, fixtureCase := range fixture.Cases {
		projectDigest, err := observeP14InitTree(fixtureCase.ProjectTemplateRoot)
		if err != nil {
			return err
		}
		homeDigest, err := observeP14InitTree(fixtureCase.HomeTemplateRoot)
		if err != nil {
			return err
		}
		if projectDigest != fixtureCase.ProjectTemplateDigest ||
			homeDigest != fixtureCase.HomeTemplateDigest {
			return fmt.Errorf(
				"P14 init-matrix template tree differs for %q",
				fixtureCase.ID,
			)
		}
	}
	workspaceRoot, err := p14InstalledCLIWorkspaceRoot(
		input.FrozenBasis.Candidate.ExecutableDigest,
	)
	if err != nil {
		return err
	}
	for _, fixtureCase := range fixture.Cases {
		projectRoot, homeRoot := p14InstalledCLIInitExecutionRoots(
			workspaceRoot,
			p14InitMatrixScenarioID,
			fixtureCase.ID,
		)
		if fixtureCase.ProjectExecutionRoot != projectRoot ||
			fixtureCase.HomeExecutionRoot != homeRoot {
			return fmt.Errorf(
				"P14 init-matrix execution roots differ for %q",
				fixtureCase.ID,
			)
		}
	}
	return nil
}

func observeP14InitTree(root string) (string, error) {
	entries := make([]p14InitTreeEntry, 0)
	err := filepath.WalkDir(root, func(
		path string,
		entry fs.DirEntry,
		walkErr error,
	) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		item := p14InitTreeEntry{
			Path: filepath.ToSlash(relative),
			Mode: uint32(info.Mode().Perm()),
		}
		if entry.IsDir() {
			item.Kind = "directory"
			entries = append(entries, item)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("P14 init template contains non-regular path %s", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		item.Kind = "file"
		item.Digest = p14Digest(content)
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("observe P14 init template %s: %w", root, err)
	}
	slices.SortFunc(entries, func(left p14InitTreeEntry, right p14InitTreeEntry) int {
		return strings.Compare(left.Path, right.Path)
	})
	canonical, err := marshalP14CanonicalJSON(entries)
	if err != nil {
		return "", err
	}
	return p14Digest(canonical), nil
}

func syntheticP14InitMatrixFixture() p14InitMatrixFixture {
	policies := p14InitMatrixPolicies()
	cases := make([]p14InitMatrixFixtureCase, 0, len(policies))
	for _, policy := range policies {
		cases = append(cases, p14InitMatrixFixtureCase{
			ID:                    policy.ID,
			TemplateKind:          policy.TemplateKind,
			ProjectTemplateRoot:   "/synthetic/p14/init/" + policy.ID + "/project",
			ProjectTemplateDigest: p14TestDigest("init-project:" + policy.ID),
			HomeTemplateRoot:      "/synthetic/p14/init/" + policy.ID + "/home",
			HomeTemplateDigest:    p14TestDigest("init-home:" + policy.ID),
			ProjectExecutionRoot:  "/synthetic/p14/runtime/" + policy.ID + "/project",
			HomeExecutionRoot:     "/synthetic/p14/runtime/" + policy.ID + "/home",
		})
	}
	return p14InitMatrixFixture{
		Schema: p14InitMatrixFixtureSchema,
		Cases:  cases,
	}
}

func TestP14InitMatrixBuilderClosesSixProfileCases(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContractByBuilder(
		contract,
		p14InitMatrixBuilderID,
	)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14InitMatrixScenario(
		declared,
		syntheticP14InitMatrixFixture(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14InitMatrixPreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	tampered := scenario
	tampered.Requests = slices.Clone(scenario.Requests)
	tampered.Requests[0].CanonicalPayload = `{}`
	if err := validateP14InitMatrixPreparedScenario(declared, tampered); err == nil {
		t.Fatal("P14 init-matrix builder accepted surface drift")
	}
}

func TestP14InitMatrixPoliciesFreezeExactHostEffects(t *testing.T) {
	policies := p14InitMatrixPolicies()
	ids := make([]string, len(policies))
	for index, policy := range policies {
		ids[index] = policy.ID
	}
	if !slices.Equal(ids, []string{
		"core_only",
		"claude_full",
		"codex_full",
		"codex_mcp_only",
		"agents_skills_only",
		"all_stable_hosts",
	}) {
		t.Fatalf("P14 init subcases = %v", ids)
	}
	all := policies[len(policies)-1]
	if !slices.EqualFunc(
		all.HostManifests,
		[]p14InitMatrixHostManifest{
			{
				Host:           "claude",
				Scope:          "project",
				AdapterEdition: "claude.coherent.v2",
				Components:     []string{"instructions", "mcp"},
			},
			{
				Host:           "claude",
				Scope:          "user",
				AdapterEdition: "claude.skills.v1",
				Components:     []string{"skills"},
			},
			{
				Host:           "codex",
				Scope:          "project",
				AdapterEdition: "codex.coherent.v2",
				Components:     []string{"instructions", "mcp"},
			},
			{
				Host:           "codex",
				Scope:          "user",
				AdapterEdition: "codex.skills.v1",
				Components:     []string{"skills"},
			},
		},
		func(
			left p14InitMatrixHostManifest,
			right p14InitMatrixHostManifest,
		) bool {
			return left.Host == right.Host &&
				left.Scope == right.Scope &&
				left.AdapterEdition == right.AdapterEdition &&
				slices.Equal(left.Components, right.Components)
		},
	) ||
		!all.ForbidExperimentalHosts ||
		all.IndependentAgentSkills {
		t.Fatalf("P14 --all effects = %#v", all)
	}
	agents := policies[len(policies)-2]
	if !agents.IndependentAgentSkills ||
		len(agents.HostManifests) != 0 {
		t.Fatalf("P14 --agents effects = %#v", agents)
	}
}

func TestP14InitSupplementalProfileMatrixBindsNineSourceReceipts(
	t *testing.T,
) {
	profiles := p14InitSupplementalProfilePolicies()
	ids := make([]string, len(profiles))
	for index, profile := range profiles {
		ids[index] = profile.ID
		if profile.PreparedResult != "profile_review_prepared" ||
			profile.PreparedStatus != "profile_review_ready" ||
			!slices.Equal(
				profile.ApplyArgv,
				[]string{"onboard", "profile", "apply", "--json"},
			) ||
			profile.ApplyResult != "profile_applied" ||
			profile.ApplyStatus != "needs_memory" ||
			profile.ReplayDelivery != "reused" ||
			profile.EvidenceBasis != p14InitSupplementalProfileBasis ||
			profile.ExactSourceTestRef !=
				p14InitSupplementalProfileTestRef {
			t.Fatalf(
				"P14 supplemental profile %q contract differs: %#v",
				profile.ID,
				profile,
			)
		}
	}
	if !slices.Equal(ids, []string{
		"typescript",
		"python",
		"rust",
		"zig_manual_fallback",
		"elixir_manual_fallback",
		"dart_manual_fallback",
		"docs_only",
		"mixed_software_and_model",
		"empty_manual_fallback",
	}) {
		t.Fatalf("P14 supplemental profile IDs = %v", ids)
	}
	for _, index := range []int{0, 1, 2, 6, 7} {
		if profiles[index].InitialResult !=
			"profile_review_prepared" ||
			profiles[index].PreparationKind !=
				"automatic_detector_advisory" {
			t.Fatalf(
				"P14 automatic profile %q differs",
				profiles[index].ID,
			)
		}
	}
	for _, index := range []int{3, 4, 5, 8} {
		if profiles[index].InitialResult != "needs_scope_review" ||
			profiles[index].PreparationKind !=
				"explicit_manual_software_scope" ||
			len(profiles[index].Scopes) != 1 ||
			profiles[index].Scopes[0].RealizationKind != "software" {
			t.Fatalf(
				"P14 manual profile %q differs",
				profiles[index].ID,
			)
		}
	}
}
