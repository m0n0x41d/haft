package p14acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

const (
	p14GeneratePreparedInputEnvironmentKey = "HAFT_P14_GENERATE_PREPARED_INPUT"
	p14GenerationRequestSchema             = "haft.p14.prepared-input-generation-request/v1"
	p14PreparedInputFilePrefix             = "p14-prepared-request-oracle-input-"
)

type p14PreparedInputGenerationRequest struct {
	Schema                  string `json:"schema"`
	P13EvidencePath         string `json:"p13_evidence_path"`
	CandidateExecutablePath string `json:"candidate_executable_path"`
	SkillCarriersRoot       string `json:"skill_carriers_root"`
	InstructionCarrierPath  string `json:"instruction_carrier_path"`
	MCPConfigCarrierPath    string `json:"mcp_config_carrier_path"`
	GoldenMemoryFixturePath string `json:"golden_memory_fixture_path"`
	InitMatrixFixturePath   string `json:"init_matrix_fixture_path"`
	IdentifierFixturePath   string `json:"identifier_fixture_path"`
}

type p14PreparedFixtureSet struct {
	Memory     p14MemoryReadFixture
	InitMatrix p14InitMatrixFixture
	Identifier p14IdentifierFixture
	Bindings   map[string]preparedP14Binding
}

type p14PreparedInputGeneratorDependencies struct {
	ObservePreparation  p14PreparationEvidenceObserver
	FPFExecutor         p14FPFProjectionExecutor
	CodeExploreExecutor p14CodeExploreExecutor
	MemoryReadExecutor  p14MemoryReadExecutor
	VerifyP13Fresh      func(string, p13EvidenceBinding) error
}

func TestP14GeneratePreparedRequestOracleInput(t *testing.T) {
	requestPath := os.Getenv(p14GeneratePreparedInputEnvironmentKey)
	if requestPath == "" {
		t.Skip("set HAFT_P14_GENERATE_PREPARED_INPUT after P13 passes")
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRequestOracleContract(repositoryRoot, contract); err != nil {
		t.Fatal(err)
	}
	request, err := loadP14PreparedInputGenerationRequest(
		repositoryRoot,
		requestPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	observer := agenthostrestart.NewOSEvidence()
	dependencies := p14PreparedInputGeneratorDependencies{
		ObservePreparation:  observer.CapturePreparation,
		FPFExecutor:         executeP14FPFProjectionCandidate,
		CodeExploreExecutor: executeP14CodeExploreCandidate,
		MemoryReadExecutor:  executeP14MemoryReadCandidate,
		VerifyP13Fresh:      verifyP13EvidenceFreshViaHarness,
	}
	path, digest, err := generateP14PreparedRequestOracleInput(
		context.Background(),
		repositoryRoot,
		contract,
		rawContract,
		request,
		dependencies,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P14_PREPARED_INPUT path=%s digest=%s", path, digest)
}

func TestP14PreparedInputGeneratorBuildsOneClosedNoClobberCarrier(
	t *testing.T,
) {
	sourceRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, request, preparation := prepareP14GeneratorTestRepository(
		t,
		rawContract,
	)
	requestRaw, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	requestRaw = append(requestRaw, '\n')
	requestPath := filepath.Join(
		repositoryRoot,
		".context",
		"p14",
		"prepared-input-generation-request.json",
	)
	writeP14GeneratorTestFile(t, requestPath, requestRaw)
	request, err = loadP14PreparedInputGenerationRequest(
		repositoryRoot,
		requestPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	freshnessChecks := 0
	dependencies := p14PreparedInputGeneratorDependencies{
		ObservePreparation: func(
			_ context.Context,
			_ agenthostrestart.PreparationRequest,
		) (agenthostrestart.PreparationEvidence, error) {
			return preparation, nil
		},
		FPFExecutor:         executeSyntheticP14FPFProjectionCandidate,
		CodeExploreExecutor: executeSyntheticP14CodeExploreCandidate,
		MemoryReadExecutor:  executeSyntheticP14MemoryReadCandidate,
		VerifyP13Fresh: func(
			_ string,
			_ p13EvidenceBinding,
		) error {
			freshnessChecks++
			return nil
		},
	}
	path, digest, err := generateP14PreparedRequestOracleInput(
		context.Background(),
		repositoryRoot,
		contract,
		rawContract,
		request,
		dependencies,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(
		filepath.Base(path),
		p14PreparedInputFilePrefix,
	) || !validP14Digest(digest) || freshnessChecks != 1 {
		t.Fatalf(
			"P14 generated input = path=%q digest=%q freshness=%d",
			path,
			digest,
			freshnessChecks,
		)
	}
	inputPath := filepath.Join(repositoryRoot, filepath.FromSlash(path))
	input, err := loadPreparedRequestOracleInput(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		t.Fatal(err)
	}
	if input.ReleaseClaim ||
		input.ResultSemantics != p14PreparedSemantics ||
		len(input.Scenarios) != len(contract.Scenarios) {
		t.Fatal("P14 generated input overstates or omits prepared-only coverage")
	}
	_, _, err = generateP14PreparedRequestOracleInput(
		context.Background(),
		repositoryRoot,
		contract,
		rawContract,
		request,
		dependencies,
	)
	if err == nil {
		t.Fatal("P14 prepared-input generator replaced an existing carrier")
	}
}

func generateP14PreparedRequestOracleInput(
	ctx context.Context,
	repositoryRoot string,
	contract requestOracleContract,
	rawContract []byte,
	request p14PreparedInputGenerationRequest,
	dependencies p14PreparedInputGeneratorDependencies,
) (string, string, error) {
	if err := validateP14PreparedInputGenerationRequest(request); err != nil {
		return "", "", err
	}
	if dependencies.ObservePreparation == nil ||
		dependencies.FPFExecutor == nil ||
		dependencies.CodeExploreExecutor == nil ||
		dependencies.MemoryReadExecutor == nil ||
		dependencies.VerifyP13Fresh == nil {
		return "", "", fmt.Errorf("P14 prepared-input generator dependency is absent")
	}
	canonicalRequest, err := canonicalizeP14PreparedInputGenerationRequest(request)
	if err != nil {
		return "", "", err
	}
	p13Binding, p13Evidence, err := loadP14PassingP13Evidence(
		repositoryRoot,
		canonicalRequest.P13EvidencePath,
	)
	if err != nil {
		return "", "", err
	}
	if err := dependencies.VerifyP13Fresh(repositoryRoot, p13Binding); err != nil {
		return "", "", err
	}
	fixtures, err := loadP14PreparedFixtureSet(
		repositoryRoot,
		canonicalRequest,
	)
	if err != nil {
		return "", "", err
	}
	preparationRequest := agenthostrestart.PreparationRequest{
		ProjectRoot:         repositoryRoot,
		CandidateHaftBinary: canonicalRequest.CandidateExecutablePath,
		SkillCarriersRoot:   canonicalRequest.SkillCarriersRoot,
		InstructionCarrier:  canonicalRequest.InstructionCarrierPath,
		MCPConfigCarrier:    canonicalRequest.MCPConfigCarrierPath,
	}
	preparation, err := dependencies.ObservePreparation(
		ctx,
		preparationRequest,
	)
	if err != nil {
		return "", "", fmt.Errorf("capture P14 prepared-input basis: %w", err)
	}
	basis, err := buildP14FrozenBasis(
		repositoryRoot,
		canonicalRequest,
		p13Evidence.StartIdentity,
		preparation,
	)
	if err != nil {
		return "", "", err
	}
	if err := validateP14PreparedFixtureBasis(fixtures, basis); err != nil {
		return "", "", err
	}
	bindings, err := buildP14PreparedBindings(
		contract.BindingGroups,
		fixtures.Bindings,
	)
	if err != nil {
		return "", "", err
	}
	sources := p14PreparedScenarioSources{
		Context:               ctx,
		Executable:            basis.Candidate.ExecutablePath,
		ProjectRoot:           basis.SelectedProject.ProjectRoot,
		ExecutableDigest:      basis.Candidate.ExecutableDigest,
		ExpectedFPFRevision:   basis.Candidate.FPFRevision,
		MemoryFixture:         fixtures.Memory,
		InitMatrixFixture:     fixtures.InitMatrix,
		IdentifierFixture:     fixtures.Identifier,
		FPFProjectionExecutor: dependencies.FPFExecutor,
		CodeExploreExecutor:   dependencies.CodeExploreExecutor,
		MemoryReadExecutor:    dependencies.MemoryReadExecutor,
	}
	scenarios, err := buildP14PreparedScenarios(contract, sources)
	if err != nil {
		return "", "", err
	}
	input := preparedRequestOracleInput{
		Schema:          p14PreparedInputSchema,
		Status:          p14ContractStatus,
		ResultSemantics: p14PreparedSemantics,
		ReleaseClaim:    false,
		ContractRef:     p14ContractRelativePath,
		ContractDigest:  p14Digest(rawContract),
		P13Evidence:     p13Binding,
		FrozenBasis:     basis,
		Bindings:        bindings,
		Scenarios:       scenarios,
	}
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		return "", "", err
	}
	if err := verifyPreparedInputAgainstP13(repositoryRoot, input); err != nil {
		return "", "", err
	}
	if err := verifyPreparedInputCurrentBasisWithObserver(
		ctx,
		repositoryRoot,
		input,
		dependencies.ObservePreparation,
	); err != nil {
		return "", "", err
	}
	return persistPreparedRequestOracleInput(
		repositoryRoot,
		contract,
		input,
	)
}

func validateP14PreparedFixtureBasis(
	fixtures p14PreparedFixtureSet,
	basis frozenP14Basis,
) error {
	selectedDigest := strings.TrimPrefix(
		basis.SelectedProject.SelectedCompositeRef,
		"typeenv:",
	)
	if fixtures.Memory.Basis.TypeEnvDigest != selectedDigest ||
		fixtures.Memory.Basis.GraphRevision != basis.SelectedProject.GraphRevision ||
		fixtures.Memory.Operations.SelectedProjectRoot !=
			basis.SelectedProject.ProjectRoot {
		return fmt.Errorf(
			"P14 golden-memory fixture differs from the passing P13 project basis",
		)
	}
	workspaceRoot, err := p14InstalledCLIWorkspaceRoot(
		basis.Candidate.ExecutableDigest,
	)
	if err != nil {
		return err
	}
	for _, fixtureCase := range fixtures.InitMatrix.Cases {
		projectRoot, homeRoot := p14InstalledCLIInitExecutionRoots(
			workspaceRoot,
			p14InitMatrixScenarioID,
			fixtureCase.ID,
		)
		if fixtureCase.ProjectExecutionRoot != projectRoot ||
			fixtureCase.HomeExecutionRoot != homeRoot {
			return fmt.Errorf(
				"P14 init-matrix execution roots differ from the candidate workspace",
			)
		}
	}
	return nil
}

func loadP14PreparedInputGenerationRequest(
	repositoryRoot string,
	requestPath string,
) (p14PreparedInputGenerationRequest, error) {
	path := requestPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(repositoryRoot, filepath.FromSlash(path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p14PreparedInputGenerationRequest{}, fmt.Errorf(
			"read P14 prepared-input generation request: %w",
			err,
		)
	}
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var request p14PreparedInputGenerationRequest
	if err := decoder.Decode(&request); err != nil {
		return p14PreparedInputGenerationRequest{}, fmt.Errorf(
			"decode P14 prepared-input generation request: %w",
			err,
		)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return p14PreparedInputGenerationRequest{}, fmt.Errorf(
			"P14 prepared-input generation request has trailing JSON",
		)
	}
	canonical, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return p14PreparedInputGenerationRequest{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return p14PreparedInputGenerationRequest{}, fmt.Errorf(
			"P14 prepared-input generation request is not canonical JSON",
		)
	}
	if err := validateP14PreparedInputGenerationRequest(request); err != nil {
		return p14PreparedInputGenerationRequest{}, err
	}
	return canonicalizeP14PreparedInputGenerationRequest(request)
}

func validateP14PreparedInputGenerationRequest(
	request p14PreparedInputGenerationRequest,
) error {
	if request.Schema != p14GenerationRequestSchema {
		return fmt.Errorf("P14 prepared-input generation request schema differs")
	}
	absolutePaths := []string{
		request.CandidateExecutablePath,
		request.SkillCarriersRoot,
		request.InstructionCarrierPath,
		request.MCPConfigCarrierPath,
	}
	for _, path := range absolutePaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf(
				"P14 prepared-input generation request needs absolute runtime paths",
			)
		}
	}
	relativePaths := []string{
		request.P13EvidencePath,
		request.GoldenMemoryFixturePath,
		request.InitMatrixFixturePath,
		request.IdentifierFixturePath,
	}
	for _, path := range relativePaths {
		if !validP14GenerationRelativePath(path) {
			return fmt.Errorf(
				"P14 prepared-input generation request has an invalid project path",
			)
		}
	}
	if filepath.Dir(filepath.FromSlash(request.P13EvidencePath)) !=
		filepath.Join(".context", "p13") {
		return fmt.Errorf("P14 generation P13 evidence is outside .context/p13")
	}
	fixtureParent := filepath.Join(".context", "p14", "fixtures")
	fixturePaths := relativePaths[1:]
	for _, path := range fixturePaths {
		if filepath.Dir(filepath.FromSlash(path)) != fixtureParent {
			return fmt.Errorf(
				"P14 generation fixture is outside .context/p14/fixtures",
			)
		}
	}
	if hasBlankOrDuplicate(absolutePaths) ||
		hasBlankOrDuplicate(relativePaths) {
		return fmt.Errorf("P14 generation paths are blank or duplicated")
	}
	return nil
}

func validP14GenerationRelativePath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	portable := filepath.ToSlash(clean)
	return path != "" &&
		!filepath.IsAbs(clean) &&
		clean != "." &&
		!strings.HasPrefix(portable, "../") &&
		portable == path
}

func canonicalizeP14PreparedInputGenerationRequest(
	request p14PreparedInputGenerationRequest,
) (p14PreparedInputGenerationRequest, error) {
	paths := []*string{
		&request.CandidateExecutablePath,
		&request.SkillCarriersRoot,
		&request.InstructionCarrierPath,
		&request.MCPConfigCarrierPath,
	}
	for _, path := range paths {
		canonical, err := filepath.EvalSymlinks(*path)
		if err != nil {
			return p14PreparedInputGenerationRequest{}, fmt.Errorf(
				"canonicalize P14 runtime path %s: %w",
				*path,
				err,
			)
		}
		*path = canonical
	}
	return request, nil
}

func loadP14PassingP13Evidence(
	repositoryRoot string,
	relativePath string,
) (p13EvidenceBinding, p13EvidenceEnvelope, error) {
	path, portable, err := resolveP14ProjectCarrier(
		repositoryRoot,
		relativePath,
		filepath.Join(".context", "p13"),
	)
	if err != nil {
		return p13EvidenceBinding{}, p13EvidenceEnvelope{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p13EvidenceBinding{}, p13EvidenceEnvelope{}, fmt.Errorf(
			"read P13 evidence for P14 generation: %w",
			err,
		)
	}
	var evidence p13EvidenceEnvelope
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return p13EvidenceBinding{}, p13EvidenceEnvelope{}, fmt.Errorf(
			"decode P13 evidence for P14 generation: %w",
			err,
		)
	}
	binding := p13EvidenceBinding{
		CarrierPath:    portable,
		CarrierDigest:  p14Digest(raw),
		IdentityDigest: evidence.IdentityDigest,
	}
	if err := validatePassingP13Evidence(binding, evidence); err != nil {
		return p13EvidenceBinding{}, p13EvidenceEnvelope{}, err
	}
	return binding, evidence, nil
}

func validatePassingP13Evidence(
	binding p13EvidenceBinding,
	evidence p13EvidenceEnvelope,
) error {
	if evidence.Schema != p14RequiredP13Schema ||
		evidence.Status != "passed" ||
		evidence.ReleaseClaim ||
		!evidence.IdentityUnchanged {
		return fmt.Errorf("P14 generation requires passing P13 v3 evidence")
	}
	if evidence.CarrierPath != binding.CarrierPath ||
		evidence.IdentityDigest != binding.IdentityDigest ||
		evidence.StartIdentity.Digest != binding.IdentityDigest ||
		evidence.EndIdentity.Digest != binding.IdentityDigest ||
		evidence.StartIdentity != evidence.EndIdentity {
		return fmt.Errorf(
			"P14 generation requires one unchanged P13 identity and carrier path",
		)
	}
	return validateP13EvidenceBinding(binding)
}

func loadP14PreparedFixtureSet(
	repositoryRoot string,
	request p14PreparedInputGenerationRequest,
) (p14PreparedFixtureSet, error) {
	memoryRaw, memoryBinding, err := loadP14PreparedFixtureCarrier(
		repositoryRoot,
		"golden_memory_fixture",
		request.GoldenMemoryFixturePath,
	)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	memory, err := decodeP14MemoryReadFixture(memoryRaw)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	if err := validateP14MemoryReadFixtureShape(memory); err != nil {
		return p14PreparedFixtureSet{}, err
	}
	initRaw, initBinding, err := loadP14PreparedFixtureCarrier(
		repositoryRoot,
		"init_matrix",
		request.InitMatrixFixturePath,
	)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	initMatrix, err := decodeP14InitMatrixFixture(initRaw)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	if err := validateP14InitMatrixFixtureShape(initMatrix); err != nil {
		return p14PreparedFixtureSet{}, err
	}
	identifierRaw, identifierBinding, err := loadP14PreparedFixtureCarrier(
		repositoryRoot,
		"identifier_fixture",
		request.IdentifierFixturePath,
	)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	identifier, err := decodeP14IdentifierFixture(identifierRaw)
	if err != nil {
		return p14PreparedFixtureSet{}, err
	}
	if err := validateP14IdentifierFixtureShape(identifier); err != nil {
		return p14PreparedFixtureSet{}, err
	}
	return p14PreparedFixtureSet{
		Memory:     memory,
		InitMatrix: initMatrix,
		Identifier: identifier,
		Bindings: map[string]preparedP14Binding{
			memoryBinding.Group:     memoryBinding,
			initBinding.Group:       initBinding,
			identifierBinding.Group: identifierBinding,
		},
	}, nil
}

func loadP14PreparedFixtureCarrier(
	repositoryRoot string,
	group string,
	relativePath string,
) ([]byte, preparedP14Binding, error) {
	path, portable, err := resolveP14ProjectCarrier(
		repositoryRoot,
		relativePath,
		filepath.Join(".context", "p14", "fixtures"),
	)
	if err != nil {
		return nil, preparedP14Binding{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, preparedP14Binding{}, fmt.Errorf(
			"read P14 fixture %q: %w",
			group,
			err,
		)
	}
	binding := preparedP14Binding{
		Group:         group,
		State:         p14BindingCarrierDigest,
		CarrierPath:   portable,
		CarrierDigest: p14Digest(raw),
	}
	return raw, binding, nil
}

func resolveP14ProjectCarrier(
	repositoryRoot string,
	relativePath string,
	requiredParent string,
) (string, string, error) {
	if !validP14GenerationRelativePath(relativePath) {
		return "", "", fmt.Errorf("P14 project carrier path is invalid")
	}
	if filepath.Dir(filepath.FromSlash(relativePath)) != requiredParent {
		return "", "", fmt.Errorf("P14 project carrier path has the wrong parent")
	}
	root, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize P14 repository root: %w", err)
	}
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize P14 project carrier: %w", err)
	}
	observedRelative, err := filepath.Rel(root, canonical)
	if err != nil {
		return "", "", fmt.Errorf("relativize P14 project carrier: %w", err)
	}
	portable := filepath.ToSlash(observedRelative)
	if portable != relativePath {
		return "", "", fmt.Errorf(
			"P14 project carrier resolves outside its exact project path",
		)
	}
	return canonical, portable, nil
}

func buildP14FrozenBasis(
	repositoryRoot string,
	request p14PreparedInputGenerationRequest,
	identity p13IdentityEnvelope,
	preparation agenthostrestart.PreparationEvidence,
) (frozenP14Basis, error) {
	selectedDigest := strings.TrimPrefix(
		identity.TypeEnv.SelectedCompositeRef,
		"typeenv:",
	)
	if !validP14Digest(selectedDigest) ||
		identity.TypeEnv.HeadRevision <= 0 ||
		identity.Graph.Revision <= 0 {
		return frozenP14Basis{}, fmt.Errorf(
			"P14 selected project basis is not digest-addressed or revisioned",
		)
	}
	expectedPreparation := []bool{
		preparation.RepositoryHead == identity.Git.Head,
		preparation.ExpectedFPFRevision == identity.FPF.Head,
		preparation.ExpectedTypeEnvDigest == selectedDigest,
		preparation.ExpectedTypeEnvHeadRevision ==
			uint64(identity.TypeEnv.HeadRevision),
		preparation.ExpectedGraphRevision == uint64(identity.Graph.Revision),
	}
	if slices.Contains(expectedPreparation, false) {
		return frozenP14Basis{}, fmt.Errorf(
			"P14 observed preparation differs from the passing P13 basis",
		)
	}
	canonicalRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return frozenP14Basis{}, fmt.Errorf(
			"canonicalize P14 selected project root: %w",
			err,
		)
	}
	canonicalProject, err := filepath.EvalSymlinks(identity.ProjectRoot)
	if err != nil {
		return frozenP14Basis{}, fmt.Errorf(
			"canonicalize P13 selected project root: %w",
			err,
		)
	}
	if canonicalRoot != canonicalProject {
		return frozenP14Basis{}, fmt.Errorf(
			"P14 generation project differs from the passing P13 project",
		)
	}
	queryDatabaseDigest, err := digestP14File(
		filepath.Join(repositoryRoot, "internal", "cli", "fpf.db"),
	)
	if err != nil {
		return frozenP14Basis{}, err
	}
	basis := frozenP14Basis{
		Candidate: candidateP14Basis{
			GitHead:             identity.Git.Head,
			P13GitStatusDigest:  identity.Git.StatusDigest,
			DirtyStateDigest:    preparation.DirtyStateDigest,
			ExecutablePath:      request.CandidateExecutablePath,
			ExecutableDigest:    preparation.DesiredHaftBinaryDigest,
			QueryDatabaseDigest: queryDatabaseDigest,
			FPFRevision:         identity.FPF.Head,
			FPFSpecDigest:       identity.FPF.SpecDigest,
			FPFReadmeDigest:     identity.FPF.ReadmeDigest,
			BaseTypeEnvRef:      identity.FPF.Embedded.BaseTypeEnvRef,
			BaseTypeEnvDigest:   identity.FPF.Embedded.BaseTypeEnvDigest,
		},
		SelectedProject: selectedProjectP14Basis{
			ProjectID:                  identity.ProjectID,
			ProjectRoot:                canonicalProject,
			ProfileGeneration:          identity.Profile.Generation,
			ProfilePayloadDigest:       identity.Profile.PayloadDigest,
			ProfileBasisDigest:         identity.Profile.BasisDigest,
			TypeEnvHeadRef:             identity.TypeEnv.HeadRef,
			TypeEnvHeadRevision:        identity.TypeEnv.HeadRevision,
			SelectedCompositeRef:       identity.TypeEnv.SelectedCompositeRef,
			TypeEnvHeadStateDigest:     identity.TypeEnv.StateDigest,
			GraphRevision:              identity.Graph.Revision,
			GraphMaterializationDigest: identity.Graph.MaterializationDigest,
		},
		Carriers: []installedCarrierBasis{
			{
				Kind:   "agent_instructions",
				Path:   request.InstructionCarrierPath,
				Digest: preparation.ExpectedInstructionDigest,
			},
			{
				Kind:   "skill_carriers_root",
				Path:   request.SkillCarriersRoot,
				Digest: preparation.ExpectedSkillCarriersDigest,
			},
			{
				Kind:   "mcp_config",
				Path:   request.MCPConfigCarrierPath,
				Digest: preparation.ExpectedMCPConfigDigest,
			},
		},
	}
	if err := validateFrozenP14Basis(basis); err != nil {
		return frozenP14Basis{}, err
	}
	return basis, nil
}

func digestP14File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read P14 digest source %s: %w", path, err)
	}
	return p14Digest(raw), nil
}

func buildP14PreparedBindings(
	groups []string,
	fixtures map[string]preparedP14Binding,
) ([]preparedP14Binding, error) {
	states := map[string]string{
		"candidate_basis":        p14BindingEmbeddedFrozen,
		"p13_evidence":           p14BindingEmbeddedFrozen,
		"selected_project_basis": p14BindingEmbeddedFrozen,
		"golden_memory_fixture":  p14BindingCarrierDigest,
		"init_matrix":            p14BindingCarrierDigest,
		"identifier_fixture":     p14BindingCarrierDigest,
		"restart_checkpoint":     p14BindingExecutionTime,
		"live_mcp_process":       p14BindingExecutionTime,
	}
	bindings := make([]preparedP14Binding, 0, len(groups))
	for _, group := range groups {
		state, present := states[group]
		if !present {
			return nil, fmt.Errorf("P14 binding group %q has no policy", group)
		}
		binding := preparedP14Binding{
			Group: group,
			State: state,
		}
		if state == p14BindingCarrierDigest {
			fixture, available := fixtures[group]
			if !available {
				return nil, fmt.Errorf(
					"P14 binding group %q has no fixture carrier",
					group,
				)
			}
			binding = fixture
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func persistPreparedRequestOracleInput(
	repositoryRoot string,
	contract requestOracleContract,
	input preparedRequestOracleInput,
) (string, string, error) {
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		return "", "", err
	}
	canonical, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode P14 prepared input: %w", err)
	}
	canonical = append(canonical, '\n')
	digest := p14Digest(canonical)
	digestBody := strings.TrimPrefix(digest, "sha256:")
	name := p14PreparedInputFilePrefix + digestBody[:16] + ".json"
	carrierPath := filepath.Join(".context", "p14", name)
	if err := publishP14NoClobber(
		repositoryRoot,
		carrierPath,
		canonical,
	); err != nil {
		return "", "", err
	}
	return filepath.ToSlash(carrierPath), digest, nil
}

func prepareP14GeneratorTestRepository(
	t *testing.T,
	rawContract []byte,
) (
	string,
	p14PreparedInputGenerationRequest,
	agenthostrestart.PreparationEvidence,
) {
	t.Helper()
	temporaryRoot := t.TempDir()
	repositoryRoot, err := filepath.EvalSymlinks(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	queryDatabasePath := filepath.Join(repositoryRoot, "internal", "cli", "fpf.db")
	writeP14GeneratorTestFile(t, queryDatabasePath, []byte("query database"))
	fpfSpecPath := filepath.Join(repositoryRoot, "data", "FPF", "FPF-Spec.md")
	fpfSpecRaw := []byte("# exact FPF specification\n")
	writeP14GeneratorTestFile(t, fpfSpecPath, fpfSpecRaw)
	fpfReadmePath := filepath.Join(repositoryRoot, "data", "FPF", "Readme.md")
	fpfReadmeRaw := []byte("# exact FPF readme\n")
	writeP14GeneratorTestFile(t, fpfReadmePath, fpfReadmeRaw)
	contractPath := filepath.Join(
		repositoryRoot,
		"internal",
		"p14acceptance",
		"contract.json",
	)
	writeP14GeneratorTestFile(t, contractPath, rawContract)

	candidatePath := filepath.Join(repositoryRoot, "bin", "haft")
	writeP14GeneratorTestFile(t, candidatePath, []byte("candidate"))
	candidateDigest := p14Digest([]byte("candidate"))
	instructionPath := filepath.Join(repositoryRoot, "AGENTS.md")
	writeP14GeneratorTestFile(t, instructionPath, []byte("instructions"))
	skillRoot := filepath.Join(repositoryRoot, "skills")
	skillPath := filepath.Join(skillRoot, "h-reason", "SKILL.md")
	writeP14GeneratorTestFile(t, skillPath, []byte("skill"))
	mcpConfigPath := filepath.Join(repositoryRoot, ".codex", "config.toml")
	writeP14GeneratorTestFile(t, mcpConfigPath, []byte("[mcp]\n"))

	basis := syntheticFrozenP14BasisForP13()
	basis.Candidate.FPFRevision = strings.Repeat("a", 40)
	basis.Candidate.FPFSpecDigest = p14Digest(fpfSpecRaw)
	basis.Candidate.FPFReadmeDigest = p14Digest(fpfReadmeRaw)
	basis.SelectedProject.ProjectRoot = repositoryRoot
	identityDigest := p14TestDigest("generator-p13-identity")
	p13RelativePath := ".context/p13/p13-acceptance-generator.json"
	evidence := syntheticP13EvidenceForP14(
		basis,
		identityDigest,
		p14RequiredP13Schema,
		p13RelativePath,
	)
	evidenceRaw, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	evidenceRaw = append(evidenceRaw, '\n')
	p13Path := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(p13RelativePath),
	)
	writeP14GeneratorTestFile(t, p13Path, evidenceRaw)

	memoryFixture := syntheticP14MemoryReadFixture()
	selectedDigest := strings.TrimPrefix(
		basis.SelectedProject.SelectedCompositeRef,
		"typeenv:",
	)
	memoryFixture.Basis.TypeEnvDigest = selectedDigest
	memoryFixture.Basis.GraphRevision = basis.SelectedProject.GraphRevision
	memoryFixture.StaleBasis.TypeEnvDigest = p14TestDigest("stale-generator")
	memoryFixture.StaleBasis.GraphRevision =
		basis.SelectedProject.GraphRevision - 1
	memoryFixture.Operations = syntheticP14MemoryOperationFixture(memoryFixture)
	memoryHomeTemplateRoot := filepath.Join(
		repositoryRoot,
		".context",
		"p14",
		"templates",
		"memory-home",
	)
	memoryHomeTemplateFile := filepath.Join(
		memoryHomeTemplateRoot,
		".haft",
		"seed.json",
	)
	writeP14GeneratorTestFile(t, memoryHomeTemplateFile, []byte("{}\n"))
	memoryHomeTemplateDigest, err := observeP14InitTree(
		memoryHomeTemplateRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	memoryFixture.Operations.HomeTemplateRoot = memoryHomeTemplateRoot
	memoryFixture.Operations.HomeTemplateDigest = memoryHomeTemplateDigest
	memoryRelativePath := ".context/p14/fixtures/golden_memory_fixture.json"

	initFixture := materializeP14GeneratorInitFixture(
		t,
		repositoryRoot,
		candidateDigest,
	)
	initRaw, err := marshalP14CanonicalJSON(initFixture)
	if err != nil {
		t.Fatal(err)
	}
	initRelativePath := ".context/p14/fixtures/init_matrix.json"
	initPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(initRelativePath),
	)
	writeP14GeneratorTestFile(t, initPath, initRaw)

	identifierFixture := syntheticP14IdentifierFixture()
	artifactRelativePath := filepath.ToSlash(
		filepath.Join(
			".haft",
			"notes",
			identifierFixture.ArtifactRef+".md",
		),
	)
	artifactRaw := []byte("# exact artifact\n")
	artifactPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(artifactRelativePath),
	)
	writeP14GeneratorTestFile(t, artifactPath, artifactRaw)
	identifierFixture.ArtifactCarrierPath = artifactRelativePath
	identifierFixture.ArtifactCarrierDigest = p14Digest(artifactRaw)
	projectIdentityPath := filepath.Join(repositoryRoot, ".haft", "project.yaml")
	writeP14GeneratorTestFile(
		t,
		projectIdentityPath,
		[]byte("id: qnt_generator\nname: p14-generator\n"),
	)
	selectedProjectBasisDigest, err := observeP14SelectedProjectMemoryBasis(
		repositoryRoot,
	)
	if err != nil {
		t.Fatal(err)
	}
	memoryFixture.Operations.SelectedProjectRoot = repositoryRoot
	memoryFixture.Operations.SelectedProjectBasisDigest =
		selectedProjectBasisDigest
	memoryRaw, err := json.MarshalIndent(memoryFixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	memoryRaw = append(memoryRaw, '\n')
	memoryPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(memoryRelativePath),
	)
	writeP14GeneratorTestFile(t, memoryPath, memoryRaw)
	identifierRaw, err := json.MarshalIndent(identifierFixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	identifierRaw = append(identifierRaw, '\n')
	identifierRelativePath := ".context/p14/fixtures/identifier_fixture.json"
	identifierPath := filepath.Join(
		repositoryRoot,
		filepath.FromSlash(identifierRelativePath),
	)
	writeP14GeneratorTestFile(t, identifierPath, identifierRaw)

	request := p14PreparedInputGenerationRequest{
		Schema:                  p14GenerationRequestSchema,
		P13EvidencePath:         p13RelativePath,
		CandidateExecutablePath: candidatePath,
		SkillCarriersRoot:       skillRoot,
		InstructionCarrierPath:  instructionPath,
		MCPConfigCarrierPath:    mcpConfigPath,
		GoldenMemoryFixturePath: memoryRelativePath,
		InitMatrixFixturePath:   initRelativePath,
		IdentifierFixturePath:   identifierRelativePath,
	}
	preparation := agenthostrestart.PreparationEvidence{
		RepositoryHead:              basis.Candidate.GitHead,
		DirtyStateDigest:            p14TestDigest("generator-dirty-state"),
		DesiredHaftBinaryDigest:     candidateDigest,
		ExpectedFPFRevision:         basis.Candidate.FPFRevision,
		ExpectedTypeEnvDigest:       selectedDigest,
		ExpectedTypeEnvHeadRevision: uint64(basis.SelectedProject.TypeEnvHeadRevision),
		ExpectedGraphRevision:       uint64(basis.SelectedProject.GraphRevision),
		ExpectedSkillCarriersDigest: p14TestDigest("generator-skills"),
		ExpectedInstructionDigest:   p14TestDigest("generator-instructions"),
		ExpectedMCPConfigDigest:     p14TestDigest("generator-mcp"),
	}
	return repositoryRoot, request, preparation
}

func materializeP14GeneratorInitFixture(
	t *testing.T,
	repositoryRoot string,
	candidateDigest string,
) p14InitMatrixFixture {
	t.Helper()
	fixture := syntheticP14InitMatrixFixture()
	workspaceRoot, err := p14InstalledCLIWorkspaceRoot(candidateDigest)
	if err != nil {
		t.Fatal(err)
	}
	for index, fixtureCase := range fixture.Cases {
		caseRoot := filepath.Join(
			repositoryRoot,
			".context",
			"p14",
			"templates",
			"init",
			fixtureCase.ID,
		)
		projectRoot := filepath.Join(caseRoot, "project")
		projectFile := filepath.Join(projectRoot, "project.seed")
		writeP14GeneratorTestFile(t, projectFile, []byte(fixtureCase.ID))
		projectDigest, err := observeP14InitTree(projectRoot)
		if err != nil {
			t.Fatal(err)
		}
		homeRoot := filepath.Join(caseRoot, "home")
		homeFile := filepath.Join(homeRoot, "home.seed")
		writeP14GeneratorTestFile(t, homeFile, []byte(fixtureCase.TemplateKind))
		homeDigest, err := observeP14InitTree(homeRoot)
		if err != nil {
			t.Fatal(err)
		}
		fixture.Cases[index].ProjectTemplateRoot = projectRoot
		fixture.Cases[index].ProjectTemplateDigest = projectDigest
		fixture.Cases[index].HomeTemplateRoot = homeRoot
		fixture.Cases[index].HomeTemplateDigest = homeDigest
		projectExecutionRoot, homeExecutionRoot :=
			p14InstalledCLIInitExecutionRoots(
				workspaceRoot,
				p14InitMatrixScenarioID,
				fixtureCase.ID,
			)
		fixture.Cases[index].ProjectExecutionRoot = projectExecutionRoot
		fixture.Cases[index].HomeExecutionRoot = homeExecutionRoot
	}
	return fixture
}

func writeP14GeneratorTestFile(
	t *testing.T,
	path string,
	content []byte,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
