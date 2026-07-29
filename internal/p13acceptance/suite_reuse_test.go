package p13acceptance

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	p13ReuseEvidencePathEnvironmentKey   = "HAFT_P13_REUSE_ACCEPTANCE_EVIDENCE"
	p13ReuseEvidenceDigestEnvironmentKey = "HAFT_P13_REUSE_ACCEPTANCE_DIGEST"
	suiteDependencySchema                = "haft.p13.suite-dependency/v1"
)

type suiteReuseSource struct {
	CarrierPath   string
	CarrierDigest string
	Evidence      acceptanceEvidence
}

type suiteDependencyBasis struct {
	Schema             string              `json:"schema"`
	Profile            string              `json:"profile"`
	Suite              suiteSpec           `json:"suite"`
	GateContractDigest string              `json:"gate_contract_digest"`
	Source             byteIdentity        `json:"source"`
	RuntimeInputDigest string              `json:"runtime_input_digest"`
	Toolchain          suiteToolchainBasis `json:"toolchain"`
	FPFDigest          string              `json:"fpf_digest,omitempty"`
	ProjectBasisDigest string              `json:"project_basis_digest,omitempty"`
	GitStatusDigest    string              `json:"git_status_digest,omitempty"`
}

type suiteDependencyProfileBasis struct {
	Profile            string
	Source             byteIdentity
	RuntimeInputDigest string
	Toolchain          suiteToolchainBasis
	FPFDigest          string
	ProjectBasisDigest string
	GitStatusDigest    string
}

type suiteToolchainBasis struct {
	Family              string       `json:"family"`
	Platform            string       `json:"platform"`
	EnvironmentDigest   string       `json:"environment_digest"`
	ExecutableDigest    string       `json:"executable_digest"`
	GoVersion           string       `json:"go_version,omitempty"`
	GoEnvironment       string       `json:"go_environment,omitempty"`
	ModuleGraphDigest   string       `json:"module_graph_digest,omitempty"`
	GitVersion          string       `json:"git_version,omitempty"`
	BashVersion         string       `json:"bash_version,omitempty"`
	PythonRuntime       string       `json:"python_runtime,omitempty"`
	NodeVersion         string       `json:"node_version,omitempty"`
	PNPMVersion         string       `json:"pnpm_version,omitempty"`
	MixVersion          string       `json:"mix_version,omitempty"`
	ElixirVersion       string       `json:"elixir_version,omitempty"`
	InstalledDependency byteIdentity `json:"installed_dependency,omitempty"`
}

type suiteGateContract struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	PlanSpan string       `json:"plan_span"`
	Claims   []string     `json:"claims"`
	SuiteIDs []string     `json:"suite_ids"`
	Anchors  []testAnchor `json:"anchors"`
}

func TestSuiteReuseImportsOnlyExactDependencyClosure(t *testing.T) {
	identityDigest := sha256Prefixed([]byte("prior-identity"))
	unchangedDependency := sha256Prefixed([]byte("unchanged-dependency"))
	priorChangedDependency := sha256Prefixed([]byte("prior-changed-dependency"))
	currentChangedDependency := sha256Prefixed([]byte("current-changed-dependency"))
	suites := []suiteSpec{
		{ID: "unchanged", Kind: "exec"},
		{ID: "changed", Kind: "exec"},
	}
	source := suiteReuseSource{
		CarrierPath:   ".context/p13/prior.json",
		CarrierDigest: sha256Prefixed([]byte("prior-carrier")),
		Evidence: acceptanceEvidence{
			Suites: []suiteEvidence{
				passingSuiteEvidence(suites[0], unchangedDependency, identityDigest),
				passingSuiteEvidence(suites[1], priorChangedDependency, identityDigest),
			},
		},
	}
	imported, ok := importSuiteExecution(
		suites[0],
		unchangedDependency,
		map[string]struct{}{},
		source,
	)
	if !ok {
		t.Fatal("exact dependency closure was not reused")
	}
	if imported.Evidence.Provenance != suiteProvenanceImported ||
		imported.Evidence.ImportedFromCarrierPath != source.CarrierPath ||
		imported.Evidence.ImportedFromCarrierDigest != source.CarrierDigest {
		t.Fatalf("import provenance = %#v", imported.Evidence)
	}
	if _, ok := importSuiteExecution(
		suites[1],
		currentChangedDependency,
		map[string]struct{}{},
		source,
	); ok {
		t.Fatal("changed dependency closure reused stale suite evidence")
	}
}

func TestSuiteDependencyDigestBindsCommandAndOwningGate(t *testing.T) {
	basis := suiteDependencyBasis{
		Schema:             suiteDependencySchema,
		Profile:            "go_repository",
		Suite:              suiteSpec{ID: "go_normal", Kind: "go_test_all_non_desktop"},
		GateContractDigest: sha256Prefixed([]byte("gate-a")),
		Source:             byteIdentity{Digest: sha256Prefixed([]byte("source"))},
		RuntimeInputDigest: sha256Prefixed([]byte("inputs")),
		Toolchain: suiteToolchainBasis{
			Family:           "go",
			Platform:         "test",
			GoVersion:        "go-test",
			ExecutableDigest: sha256Prefixed([]byte("tool")),
		},
	}
	initial, err := digestCanonicalJSON(basis)
	if err != nil {
		t.Fatal(err)
	}
	changedCommand := basis
	changedCommand.Suite.Kind = "go_test_race_critical"
	commandDigest, err := digestCanonicalJSON(changedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if commandDigest == initial {
		t.Fatal("suite command change preserved dependency digest")
	}
	changedGate := basis
	changedGate.GateContractDigest = sha256Prefixed([]byte("gate-b"))
	gateDigest, err := digestCanonicalJSON(changedGate)
	if err != nil {
		t.Fatal(err)
	}
	if gateDigest == initial {
		t.Fatal("owning gate change preserved dependency digest")
	}
}

func TestSuiteReuseRejectsMissingCurrentGoAnchor(t *testing.T) {
	suite := suiteSpec{ID: "go_normal", Kind: "go_test_all_non_desktop"}
	dependencyDigest := sha256Prefixed([]byte("dependency"))
	identityDigest := sha256Prefixed([]byte("identity"))
	prior := passingSuiteEvidence(suite, dependencyDigest, identityDigest)
	source := suiteReuseSource{
		CarrierPath:   ".context/p13/prior.json",
		CarrierDigest: sha256Prefixed([]byte("carrier")),
		Evidence: acceptanceEvidence{
			Suites: []suiteEvidence{prior},
		},
	}
	required := map[string]struct{}{"package::TestRequired": {}}
	if _, ok := importSuiteExecution(
		suite,
		dependencyDigest,
		required,
		source,
	); ok {
		t.Fatal("Go suite without the current required anchor was reused")
	}
	source.Evidence.Suites[0].PassedAnchors = []string{"package::TestRequired"}
	if _, ok := importSuiteExecution(
		suite,
		dependencyDigest,
		required,
		source,
	); !ok {
		t.Fatal("Go suite with the exact current required anchor was not reused")
	}
}

func TestOptionalSuiteReuseSourceRequiresPathAndDigestTogether(t *testing.T) {
	t.Setenv(
		p13ReuseEvidencePathEnvironmentKey,
		".context/p13/prior.json",
	)
	t.Setenv(p13ReuseEvidenceDigestEnvironmentKey, "")
	if _, err := loadOptionalSuiteReuseSource(
		t.TempDir(),
		acceptanceManifest{},
	); err == nil {
		t.Fatal("reuse carrier path without exact digest was accepted")
	}
}

func TestFailedCarrierCanReuseOnlyItsIndividuallyPassingSuites(t *testing.T) {
	identityDigest := sha256Prefixed([]byte("failed-carrier-identity"))
	passing := passingSuiteEvidence(
		suiteSpec{ID: "passed", Kind: "exec"},
		sha256Prefixed([]byte("passed-dependency")),
		identityDigest,
	)
	evidence := acceptanceEvidence{
		Schema:            acceptanceEvidenceSchema,
		Status:            "failed",
		FinishedAt:        "2026-07-26T16:06:43.514992Z",
		IdentityDigest:    identityDigest,
		IdentityUnchanged: true,
		StartIdentity: acceptanceIdentity{
			Digest: identityDigest,
		},
		EndIdentity: acceptanceIdentity{
			Digest: identityDigest,
		},
		Suites: []suiteEvidence{
			passing,
			{
				ID:     "failed",
				Kind:   "exec",
				Status: "fail",
			},
		},
	}
	carrierPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.CarrierPath = carrierPath
	if err := validateReusableP13Evidence(evidence, carrierPath); err != nil {
		t.Fatal(err)
	}
	source := suiteReuseSource{
		CarrierPath:   carrierPath,
		CarrierDigest: sha256Prefixed([]byte("failed-carrier")),
		Evidence:      evidence,
	}
	if _, ok := importSuiteExecution(
		suiteSpec{ID: "passed", Kind: "exec"},
		passing.DependencyDigest,
		map[string]struct{}{},
		source,
	); !ok {
		t.Fatal("passing suite from failed carrier was not reusable")
	}
	if _, ok := importSuiteExecution(
		suiteSpec{ID: "failed", Kind: "exec"},
		sha256Prefixed([]byte("failed-dependency")),
		map[string]struct{}{},
		source,
	); ok {
		t.Fatal("failed suite was reused")
	}
	evidence.IdentityUnchanged = false
	if err := validateReusableP13Evidence(evidence, carrierPath); err == nil {
		t.Fatal("identity-changing failed carrier was reusable")
	}
}

func TestScopedSuiteSourceExcludesInstalledDependencies(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "packages", "haft-pi", "tests", "suite.ts")
	dependencyPath := filepath.Join(
		root,
		"packages",
		"haft-pi",
		"node_modules",
		"dependency.js",
	)
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dependencyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("source-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependencyPath, []byte("dependency-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	initial, err := captureScopedSource(
		root,
		[]string{"packages/haft-pi"},
		[]string{},
		[]string{"node_modules"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependencyPath, []byte("dependency-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	afterDependency, err := captureScopedSource(
		root,
		[]string{"packages/haft-pi"},
		[]string{},
		[]string{"node_modules"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if afterDependency.Digest != initial.Digest {
		t.Fatal("installed dependency bytes leaked into source dependency closure")
	}
	if err := os.WriteFile(sourcePath, []byte("source-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	afterSource, err := captureScopedSource(
		root,
		[]string{"packages/haft-pi"},
		[]string{},
		[]string{"node_modules"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if afterSource.Digest == initial.Digest {
		t.Fatal("suite source change preserved dependency closure")
	}
}

func passingSuiteEvidence(
	suite suiteSpec,
	dependencyDigest string,
	identityDigest string,
) suiteEvidence {
	return suiteEvidence{
		ID:                 suite.ID,
		Kind:               suite.Kind,
		Status:             "pass",
		Provenance:         suiteProvenanceExecuted,
		DependencyDigest:   dependencyDigest,
		InvocationCount:    1,
		InvocationDigest:   sha256Prefixed([]byte("invocation")),
		OutputDigest:       sha256Prefixed([]byte("output")),
		Invocations:        []commandInvocation{{Program: "test"}},
		PreIdentityDigest:  identityDigest,
		PostIdentityDigest: identityDigest,
		IdentityUnchanged:  true,
	}
}

func captureSuiteDependencyDigests(
	root string,
	manifest acceptanceManifest,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
) (map[string]string, error) {
	result := make(map[string]string, len(manifest.Suites))
	profileBases := make(map[string]suiteDependencyProfileBasis)
	for _, suite := range manifest.Suites {
		profile, err := suiteDependencyProfile(suite.ID)
		if err != nil {
			return nil, fmt.Errorf(
				"capture suite %q dependency basis: %w",
				suite.ID,
				err,
			)
		}
		profileBasis, found := profileBases[profile]
		if !found {
			profileBasis, err = captureSuiteDependencyProfileBasis(
				root,
				profile,
				inputs,
				identity,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"capture suite %q dependency profile: %w",
					suite.ID,
					err,
				)
			}
			profileBases[profile] = profileBasis
		}
		gateDigest, err := suiteGateContractDigest(suite.ID, manifest.Gates)
		if err != nil {
			return nil, fmt.Errorf(
				"capture suite %q gate contract: %w",
				suite.ID,
				err,
			)
		}
		basis := suiteDependencyBasis{
			Schema:             suiteDependencySchema,
			Profile:            profileBasis.Profile,
			Suite:              suite,
			GateContractDigest: gateDigest,
			Source:             profileBasis.Source,
			RuntimeInputDigest: profileBasis.RuntimeInputDigest,
			Toolchain:          profileBasis.Toolchain,
			FPFDigest:          profileBasis.FPFDigest,
			ProjectBasisDigest: profileBasis.ProjectBasisDigest,
			GitStatusDigest:    profileBasis.GitStatusDigest,
		}
		digest, err := digestCanonicalJSON(basis)
		if err != nil {
			return nil, fmt.Errorf(
				"digest suite %q dependency basis: %w",
				suite.ID,
				err,
			)
		}
		result[suite.ID] = digest
	}
	return result, nil
}

func captureSuiteDependencyProfileBasis(
	root string,
	profile string,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
) (suiteDependencyProfileBasis, error) {
	source, err := captureSuiteSourceIdentity(
		root,
		profile,
		inputs,
		identity,
	)
	if err != nil {
		return suiteDependencyProfileBasis{}, err
	}
	runtimeDigest, err := suiteRuntimeInputDigest(
		profile,
		inputs,
		identity,
		source,
	)
	if err != nil {
		return suiteDependencyProfileBasis{}, err
	}
	toolchain, err := captureSuiteToolchainBasis(root, profile, identity.Toolchain)
	if err != nil {
		return suiteDependencyProfileBasis{}, err
	}
	fpfDigest, projectBasisDigest, gitStatusDigest, err := suiteSemanticBasis(
		profile,
		identity,
	)
	if err != nil {
		return suiteDependencyProfileBasis{}, err
	}
	return suiteDependencyProfileBasis{
		Profile:            profile,
		Source:             source,
		RuntimeInputDigest: runtimeDigest,
		Toolchain:          toolchain,
		FPFDigest:          fpfDigest,
		ProjectBasisDigest: projectBasisDigest,
		GitStatusDigest:    gitStatusDigest,
	}, nil
}

func suiteDependencyProfile(suiteID string) (string, error) {
	profiles := map[string]string{
		"fpf_index_exact":    "fpf_index",
		"query_token_gate":   "fpf_query",
		"go_normal":          "go_repository",
		"go_race":            "go_repository",
		"go_vet":             "go_repository",
		"pi_test":            "pi",
		"pi_typecheck":       "pi",
		"open_sleigh_format": "open_sleigh",
		"open_sleigh_test":   "open_sleigh",
		"gofmt_check":        "go_format",
		"git_diff_check":     "git_diff",
	}
	profile, found := profiles[suiteID]
	if !found {
		return "", fmt.Errorf("suite %q has no dependency profile", suiteID)
	}
	return profile, nil
}

func captureSuiteSourceIdentity(
	root string,
	profile string,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
) (byteIdentity, error) {
	switch profile {
	case "fpf_index", "fpf_query":
		return captureFPFSuiteSource(root)
	case "go_repository":
		return identity.Source, nil
	case "pi":
		return captureScopedSource(
			root,
			[]string{"packages/haft-pi"},
			[]string{},
			[]string{"node_modules"},
		)
	case "open_sleigh":
		return captureScopedSource(
			root,
			[]string{"open-sleigh"},
			[]string{},
			[]string{"_build", "deps"},
		)
	case "go_format":
		return digestPaths(root, inputs.GoFiles)
	case "git_diff":
		return captureGitDiffInputIdentity(root)
	default:
		return byteIdentity{}, fmt.Errorf("unsupported dependency profile %q", profile)
	}
}

func captureGitDiffInputIdentity(root string) (byteIdentity, error) {
	gitProgram, err := resolveExecutable("git")
	if err != nil {
		return byteIdentity{}, err
	}
	diff, err := runIdentityCommand(
		root,
		gitProgram,
		"diff",
		"--binary",
		"--no-ext-diff",
		"--no-color",
	)
	if err != nil {
		return byteIdentity{}, fmt.Errorf("capture git diff check input: %w", err)
	}
	return byteIdentity{
		Digest:    sha256Prefixed(diff),
		FileCount: 0,
		ByteCount: int64(len(diff)),
	}, nil
}

func captureFPFSuiteSource(root string) (byteIdentity, error) {
	goPaths, err := collectGoBuildInputPaths(root)
	if err != nil {
		return byteIdentity{}, err
	}
	scopedPaths, err := collectScopedSourcePaths(
		root,
		[]string{"data/FPF"},
		[]string{
			"go.mod",
			"go.sum",
			"internal/cli/fpf.db",
			"scripts/fpf_query_token_count.requirements.txt",
			"scripts/fpf_query_token_gate.sh",
		},
		[]string{".git"},
	)
	if err != nil {
		return byteIdentity{}, err
	}
	paths := append(goPaths, scopedPaths...)
	paths = uniqueSortedPaths(paths)
	return digestPaths(root, paths)
}

func captureScopedSource(
	root string,
	roots []string,
	files []string,
	excludedDirectories []string,
) (byteIdentity, error) {
	paths, err := collectScopedSourcePaths(
		root,
		roots,
		files,
		excludedDirectories,
	)
	if err != nil {
		return byteIdentity{}, err
	}
	return digestPaths(root, paths)
}

func collectScopedSourcePaths(
	root string,
	roots []string,
	files []string,
	excludedDirectories []string,
) ([]string, error) {
	excluded := make(map[string]struct{}, len(excludedDirectories))
	for _, directory := range excludedDirectories {
		excluded[directory] = struct{}{}
	}
	pathSet := make(map[string]struct{})
	for _, relative := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect scoped source file %s: %w", relative, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("scoped source file %s is a directory", relative)
		}
		pathSet[filepath.Clean(relative)] = struct{}{}
	}
	for _, relativeRoot := range roots {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		walkErr := filepath.WalkDir(
			absoluteRoot,
			func(path string, entry os.DirEntry, walkErr error) error {
				return collectRelevantPath(
					root,
					path,
					entry,
					walkErr,
					excluded,
					pathSet,
				)
			},
		)
		if walkErr != nil {
			return nil, fmt.Errorf("walk scoped source root %s: %w", relativeRoot, walkErr)
		}
	}
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("scoped source identity is empty")
	}
	return paths, nil
}

func uniqueSortedPaths(paths []string) []string {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[filepath.Clean(path)] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for path := range set {
		result = append(result, path)
	}
	slices.Sort(result)
	return result
}

func suiteGateContractDigest(
	suiteID string,
	gates []gateSpec,
) (string, error) {
	contracts := make([]suiteGateContract, 0)
	for _, gate := range gates {
		if !slices.Contains(gate.SuiteIDs, suiteID) {
			continue
		}
		contracts = append(contracts, suiteGateContract{
			ID:       gate.ID,
			Title:    gate.Title,
			PlanSpan: gate.PlanSpan,
			Claims:   slices.Clone(gate.Claims),
			SuiteIDs: slices.Clone(gate.SuiteIDs),
			Anchors:  slices.Clone(gate.Anchors),
		})
	}
	if len(contracts) == 0 {
		return "", fmt.Errorf("suite %q has no owning gate contract", suiteID)
	}
	return digestCanonicalJSON(contracts)
}

func suiteRuntimeInputDigest(
	profile string,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
	source byteIdentity,
) (string, error) {
	value := struct {
		Profile         string `json:"profile"`
		GoPackages      string `json:"go_packages,omitempty"`
		GoFiles         string `json:"go_files,omitempty"`
		FPFSource       string `json:"fpf_source,omitempty"`
		GitStatusDigest string `json:"git_status_digest,omitempty"`
		SourceDigest    string `json:"source_digest"`
	}{
		Profile:      profile,
		SourceDigest: source.Digest,
	}
	switch profile {
	case "fpf_index", "fpf_query":
		value.FPFSource = identity.FPF.Source.Digest
	case "go_repository":
		value.GoPackages = inputs.GoPackageDigest
	case "go_format":
		value.GoFiles = inputs.GoFileDigest
	case "git_diff":
		value.GitStatusDigest = source.Digest
	case "pi", "open_sleigh":
	default:
		return "", fmt.Errorf("unsupported dependency profile %q", profile)
	}
	return digestCanonicalJSON(value)
}

func captureSuiteToolchainBasis(
	root string,
	profile string,
	toolchain toolchainIdentity,
) (suiteToolchainBasis, error) {
	basis := suiteToolchainBasis{
		Platform:          toolchain.Platform,
		EnvironmentDigest: toolchain.EnvironmentDigest,
		ExecutableDigest:  toolchain.ExecutableDigest,
	}
	switch profile {
	case "fpf_index", "go_repository", "go_format":
		basis.Family = "go"
		basis.GoVersion = toolchain.GoVersion
		basis.GoEnvironment = toolchain.GoEnvironment
		basis.ModuleGraphDigest = toolchain.ModuleGraphDigest
	case "fpf_query":
		basis.Family = "go-bash-python"
		basis.GoVersion = toolchain.GoVersion
		basis.GoEnvironment = toolchain.GoEnvironment
		basis.ModuleGraphDigest = toolchain.ModuleGraphDigest
		basis.BashVersion = toolchain.BashVersion
		basis.PythonRuntime = toolchain.PythonRuntime
	case "pi":
		dependency, err := digestDependencyRoots(
			root,
			[]string{"packages/haft-pi/node_modules"},
		)
		if err != nil {
			return suiteToolchainBasis{}, err
		}
		basis.Family = "node-pnpm"
		basis.NodeVersion = toolchain.NodeVersion
		basis.PNPMVersion = toolchain.PNPMVersion
		basis.InstalledDependency = dependency
	case "open_sleigh":
		dependency, err := digestDependencyRoots(
			root,
			[]string{"open-sleigh/deps"},
		)
		if err != nil {
			return suiteToolchainBasis{}, err
		}
		basis.Family = "beam"
		basis.MixVersion = toolchain.MixVersion
		basis.ElixirVersion = toolchain.ElixirVersion
		basis.InstalledDependency = dependency
	case "git_diff":
		basis.Family = "git"
		basis.GitVersion = toolchain.GitVersion
	default:
		return suiteToolchainBasis{}, fmt.Errorf(
			"unsupported dependency profile %q",
			profile,
		)
	}
	return basis, nil
}

func suiteSemanticBasis(
	profile string,
	identity acceptanceIdentity,
) (string, string, string, error) {
	switch profile {
	case "fpf_index", "fpf_query":
		fpfDigest, err := digestCanonicalJSON(identity.FPF)
		return fpfDigest, "", "", err
	case "go_repository":
		fpfDigest, err := digestCanonicalJSON(identity.FPF)
		if err != nil {
			return "", "", "", err
		}
		project := struct {
			Profile     profileIdentity `json:"profile"`
			TypeEnv     typeEnvIdentity `json:"type_env"`
			Graph       graphIdentity   `json:"graph"`
			SchemaState schemaIdentity  `json:"schema_state"`
		}{
			Profile:     identity.Profile,
			TypeEnv:     identity.TypeEnv,
			Graph:       identity.Graph,
			SchemaState: identity.SchemaState,
		}
		projectDigest, err := digestCanonicalJSON(project)
		return fpfDigest, projectDigest, "", err
	case "git_diff":
		return "", "", identity.Git.StatusDigest, nil
	case "pi", "open_sleigh", "go_format":
		return "", "", "", nil
	default:
		return "", "", "", fmt.Errorf(
			"unsupported dependency profile %q",
			profile,
		)
	}
}

func loadOptionalSuiteReuseSource(
	root string,
	_ acceptanceManifest,
) (suiteReuseSource, error) {
	carrierPath := os.Getenv(p13ReuseEvidencePathEnvironmentKey)
	expectedDigest := os.Getenv(p13ReuseEvidenceDigestEnvironmentKey)
	if carrierPath == "" && expectedDigest == "" {
		return suiteReuseSource{}, nil
	}
	if carrierPath == "" || !validPrefixedSHA256(expectedDigest) {
		return suiteReuseSource{}, fmt.Errorf(
			"%s and one canonical %s must be supplied together",
			p13ReuseEvidencePathEnvironmentKey,
			p13ReuseEvidenceDigestEnvironmentKey,
		)
	}
	cleanPath, err := cleanP13EvidencePath(carrierPath)
	if err != nil {
		return suiteReuseSource{}, err
	}
	evidence, raw, err := loadP13AcceptanceEvidence(root, cleanPath)
	if err != nil {
		return suiteReuseSource{}, err
	}
	if sha256Prefixed(raw) != expectedDigest {
		return suiteReuseSource{}, fmt.Errorf(
			"P13 suite reuse carrier bytes differ from the expected digest",
		)
	}
	if err := validateReusableP13Evidence(evidence, cleanPath); err != nil {
		return suiteReuseSource{}, err
	}
	return suiteReuseSource{
		CarrierPath:   cleanPath,
		CarrierDigest: expectedDigest,
		Evidence:      evidence,
	}, nil
}

func validateReusableP13Evidence(
	evidence acceptanceEvidence,
	carrierPath string,
) error {
	if evidence.Status == "passed" {
		return validatePassingP13Evidence(evidence, carrierPath)
	}
	wantPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		return err
	}
	if evidence.Schema != acceptanceEvidenceSchema ||
		evidence.Status != "failed" ||
		evidence.ReleaseClaim ||
		evidence.CarrierPath != carrierPath ||
		evidence.CarrierPath != wantPath ||
		!evidence.IdentityUnchanged ||
		len(evidence.Waivers) != 0 ||
		evidence.IdentityDigest == "" ||
		evidence.StartIdentity.Digest != evidence.IdentityDigest ||
		evidence.EndIdentity.Digest != evidence.IdentityDigest {
		return fmt.Errorf("P13 suite reuse source is not one unchanged failed carrier")
	}
	hasPassingSuite := false
	hasFailedSuite := false
	seenSuites := make(map[string]struct{}, len(evidence.Suites))
	for _, suite := range evidence.Suites {
		if _, found := seenSuites[suite.ID]; found || suite.ID == "" {
			return fmt.Errorf("P13 suite reuse source has a duplicate or empty suite ID")
		}
		seenSuites[suite.ID] = struct{}{}
		if suite.Status == "fail" {
			hasFailedSuite = true
			continue
		}
		if suite.Status != "pass" {
			return fmt.Errorf("P13 suite reuse source has unknown suite status %q", suite.Status)
		}
		if err := validatePassingP13Suite(evidence.IdentityDigest, suite); err != nil {
			return fmt.Errorf("P13 reusable suite %q is invalid: %w", suite.ID, err)
		}
		hasPassingSuite = true
	}
	if !hasPassingSuite || !hasFailedSuite {
		return fmt.Errorf("P13 failed reuse source must contain passing and failed suites")
	}
	return nil
}

func importSuiteExecution(
	suite suiteSpec,
	dependencyDigest string,
	requiredAnchors map[string]struct{},
	source suiteReuseSource,
) (suiteExecution, bool) {
	if source.CarrierPath == "" || !validPrefixedSHA256(dependencyDigest) {
		return suiteExecution{}, false
	}
	for _, prior := range source.Evidence.Suites {
		if prior.ID != suite.ID ||
			prior.Kind != suite.Kind ||
			prior.Status != "pass" ||
			prior.DependencyDigest != dependencyDigest {
			continue
		}
		if isGoTestSuite(suite.Kind) &&
			!stringSliceContainsSet(prior.PassedAnchors, requiredAnchors) {
			continue
		}
		evidence := cloneSuiteEvidence(prior)
		evidence.Provenance = suiteProvenanceImported
		evidence.ImportedFromCarrierPath = source.CarrierPath
		evidence.ImportedFromCarrierDigest = source.CarrierDigest
		return suiteExecution{
			Evidence:      evidence,
			PassedAnchors: stringSliceSet(evidence.PassedAnchors),
		}, true
	}
	return suiteExecution{}, false
}

func stringSliceContainsSet(
	values []string,
	required map[string]struct{},
) bool {
	observed := stringSliceSet(values)
	for requiredValue := range required {
		if _, found := observed[requiredValue]; !found {
			return false
		}
	}
	return true
}

func cloneSuiteEvidence(value suiteEvidence) suiteEvidence {
	value.PassedAnchors = slices.Clone(value.PassedAnchors)
	value.Invocations = cloneCommandInvocations(value.Invocations)
	return value
}

func stringSliceSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result[value] = struct{}{}
		}
	}
	return result
}
