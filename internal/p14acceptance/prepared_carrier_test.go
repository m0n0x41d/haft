package p14acceptance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	buildinfo "debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

const (
	p14SealInputEnvironmentKey  = "HAFT_P14_SEAL_PREPARED_INPUT"
	p14P13EvidencePathKey       = "HAFT_P13_VERIFY_ACCEPTANCE_EVIDENCE"
	p14P13EvidenceDigestKey     = "HAFT_P13_VERIFY_ACCEPTANCE_DIGEST"
	p14PreparedInputSchema      = "haft.p14.prepared-request-oracle-input/v2"
	p14PreparedCarrierSchema    = "haft.p14.prepared-request-oracle/v2"
	p14RequiredP13Schema        = "haft.p13.acceptance-evidence/v3"
	p14PreparedSemantics        = "Exact P14 requests and pre-install oracles prepared for later installed execution. This carrier is not performed Work, a live observation, passing evidence, release authority, or a release claim."
	p14BindingEmbeddedFrozen    = "embedded_frozen"
	p14BindingCarrierDigest     = "carrier_digest"
	p14BindingExecutionTime     = "execution_time_required"
	p14RequiredCandidateVersion = "9.2.0"
	p14CleanGitStatusDigest     = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	p14SyntheticBuildVCSTime    = "2026-08-11T14:19:50Z"
)

type preparedRequestOracleInput struct {
	Schema          string                `json:"schema"`
	Status          string                `json:"status"`
	ResultSemantics string                `json:"result_semantics"`
	ReleaseClaim    bool                  `json:"release_claim"`
	ContractRef     string                `json:"contract_ref"`
	ContractDigest  string                `json:"contract_digest"`
	P13Evidence     p13EvidenceBinding    `json:"p13_evidence"`
	FrozenBasis     frozenP14Basis        `json:"frozen_basis"`
	Bindings        []preparedP14Binding  `json:"bindings"`
	Scenarios       []preparedP14Scenario `json:"scenarios"`
}

type preparedP14Binding struct {
	Group         string `json:"group"`
	State         string `json:"state"`
	CarrierPath   string `json:"carrier_path,omitempty"`
	CarrierDigest string `json:"carrier_digest,omitempty"`
}

type p13EvidenceBinding struct {
	CarrierPath    string `json:"carrier_path"`
	CarrierDigest  string `json:"carrier_digest"`
	IdentityDigest string `json:"identity_digest"`
}

type frozenP14Basis struct {
	Candidate       candidateP14Basis       `json:"candidate"`
	SelectedProject selectedProjectP14Basis `json:"selected_project"`
	Carriers        []installedCarrierBasis `json:"carriers"`
}

type candidateP14Basis struct {
	GitHead             string `json:"git_head"`
	Version             string `json:"version,omitempty"`
	VersionCommit       string `json:"version_commit,omitempty"`
	VersionModified     *bool  `json:"version_modified,omitempty"`
	BuildVCS            string `json:"build_vcs"`
	BuildVCSRevision    string `json:"build_vcs_revision"`
	BuildVCSModified    *bool  `json:"build_vcs_modified"`
	BuildVCSTime        string `json:"build_vcs_time"`
	P13GitStatusDigest  string `json:"p13_git_status_digest"`
	DirtyStateDigest    string `json:"dirty_state_digest"`
	ExecutablePath      string `json:"executable_path"`
	ExecutableDigest    string `json:"executable_digest"`
	QueryDatabaseDigest string `json:"query_database_digest"`
	FPFRevision         string `json:"fpf_revision"`
	FPFSpecDigest       string `json:"fpf_spec_digest"`
	FPFReadmeDigest     string `json:"fpf_readme_digest"`
	BaseTypeEnvRef      string `json:"base_type_env_ref"`
	BaseTypeEnvDigest   string `json:"base_type_env_digest"`
}

type selectedProjectP14Basis struct {
	ProjectID                  string `json:"project_id"`
	ProjectRoot                string `json:"project_root"`
	ProfileGeneration          string `json:"profile_generation"`
	ProfilePayloadDigest       string `json:"profile_payload_digest"`
	ProfileBasisDigest         string `json:"profile_basis_digest"`
	TypeEnvHeadRef             string `json:"type_env_head_ref"`
	TypeEnvHeadRevision        int64  `json:"type_env_head_revision"`
	SelectedCompositeRef       string `json:"selected_composite_ref"`
	TypeEnvHeadStateDigest     string `json:"type_env_head_state_digest"`
	GraphRevision              int64  `json:"graph_revision"`
	GraphMaterializationDigest string `json:"graph_materialization_digest"`
}

type installedCarrierBasis struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type preparedP14Scenario struct {
	ID                       string               `json:"id"`
	SemanticRequestCanonical string               `json:"semantic_request_canonical"`
	SemanticRequestDigest    string               `json:"semantic_request_digest"`
	Requests                 []preparedP14Request `json:"requests"`
	Oracle                   preparedP14Oracle    `json:"oracle"`
}

type preparedP14Request struct {
	Surface               string `json:"surface"`
	Builder               string `json:"builder"`
	Encoding              string `json:"encoding"`
	CanonicalPayload      string `json:"canonical_payload"`
	PayloadDigest         string `json:"payload_digest"`
	SemanticRequestDigest string `json:"semantic_request_digest"`
}

type preparedP14Oracle struct {
	Kind                    string   `json:"kind"`
	NormalizationID         string   `json:"normalization_id,omitempty"`
	ExpectedResultDigest    string   `json:"expected_result_digest,omitempty"`
	ExpectedEffect          string   `json:"expected_effect"`
	PredicateIDs            []string `json:"predicate_ids,omitempty"`
	LocalOracleOutputDigest string   `json:"local_oracle_output_digest"`
}

type preparedRequestOracleCarrier struct {
	Schema            string                     `json:"schema"`
	Status            string                     `json:"status"`
	CarrierPath       string                     `json:"carrier_path"`
	PreparationDigest string                     `json:"preparation_digest"`
	Preparation       preparedRequestOracleInput `json:"preparation"`
}

type p13EvidenceEnvelope struct {
	Schema            string              `json:"schema"`
	Status            string              `json:"status"`
	ReleaseClaim      bool                `json:"release_claim"`
	CarrierPath       string              `json:"carrier_path"`
	IdentityDigest    string              `json:"identity_digest"`
	IdentityUnchanged bool                `json:"identity_unchanged"`
	StartIdentity     p13IdentityEnvelope `json:"start_identity"`
	EndIdentity       p13IdentityEnvelope `json:"end_identity"`
}

type p13IdentityEnvelope struct {
	ProjectID   string             `json:"project_id"`
	ProjectRoot string             `json:"project_root"`
	Git         p13GitEnvelope     `json:"git"`
	FPF         p13FPFEnvelope     `json:"fpf"`
	Profile     p13ProfileEnvelope `json:"profile"`
	TypeEnv     p13TypeEnvEnvelope `json:"type_env"`
	Graph       p13GraphEnvelope   `json:"graph"`
	Digest      string             `json:"digest"`
}

type p13GitEnvelope struct {
	Head         string `json:"head"`
	StatusDigest string `json:"status_digest"`
	StatusBytes  int    `json:"status_bytes"`
}

type p13FPFEnvelope struct {
	Head         string                 `json:"head"`
	SpecDigest   string                 `json:"spec_digest"`
	ReadmeDigest string                 `json:"readme_digest"`
	Embedded     p13EmbeddedFPFEnvelope `json:"embedded"`
}

type p13EmbeddedFPFEnvelope struct {
	BaseTypeEnvRef    string `json:"base_type_env_ref"`
	BaseTypeEnvDigest string `json:"base_type_env_digest"`
}

type p13ProfileEnvelope struct {
	Generation    string `json:"generation"`
	PayloadDigest string `json:"payload_digest"`
	BasisDigest   string `json:"basis_digest"`
}

type p13TypeEnvEnvelope struct {
	HeadRef              string `json:"head_ref"`
	HeadRevision         int64  `json:"head_revision"`
	SelectedCompositeRef string `json:"selected_composite_ref"`
	StateDigest          string `json:"state_digest"`
}

type p13GraphEnvelope struct {
	Revision              int64  `json:"revision"`
	MaterializationDigest string `json:"materialization_digest"`
}

func TestPreparedRequestOracleCarrierIsClosedAndNoClobber(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	executableContract := executableP14Contract(contract)
	contractDigest := p14Digest(rawContract)
	input, err := completePreparedInputForTest(executableContract, contractDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePreparedRequestOracleInput(executableContract, input); err != nil {
		t.Fatal(err)
	}
	tampered := input
	tampered.Scenarios = slices.Clone(input.Scenarios)
	tampered.Scenarios[0].Requests = slices.Clone(input.Scenarios[0].Requests)
	tampered.Scenarios[0].Requests[0].PayloadDigest = p14TestDigest("tampered")
	if err := validatePreparedRequestOracleInput(executableContract, tampered); err == nil {
		t.Fatal("P14 prepared carrier accepted request-payload digest drift")
	}
	tamperedBinding := input
	tamperedBinding.Bindings = slices.Clone(input.Bindings)
	tamperedBinding.Bindings[3].CarrierDigest = p14TestDigest("different-fixture")
	tamperedBinding.Bindings[3].State = p14BindingExecutionTime
	if err := validatePreparedRequestOracleInput(executableContract, tamperedBinding); err == nil {
		t.Fatal("P14 prepared carrier accepted a missing frozen fixture binding")
	}
	root := t.TempDir()
	path, digest, err := persistPreparedRequestOracleCarrier(
		root,
		executableContract,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" || !validP14Digest(digest) {
		t.Fatalf("persisted P14 carrier = %q %q", path, digest)
	}
	persistedPath := filepath.Join(root, filepath.FromSlash(path))
	persisted, err := os.ReadFile(persistedPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePreparedRequestOracleCarrier(
		executableContract,
		persisted,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := persistPreparedRequestOracleCarrier(
		root,
		executableContract,
		input,
	); err == nil {
		t.Fatal("second P14 prepared carrier publication replaced existing bytes")
	}
}

func TestP14FullPreparedInputHasExecutableBuilderForEveryScenario(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	missing := missingP14ExecutableBuilders(contract)
	if len(missing) != 0 {
		t.Fatalf("P14 missing executable builders = %#v", missing)
	}
	input, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPreparedInputRequiresCurrentP13EvidenceSchema(t *testing.T) {
	root := t.TempDir()
	identityDigest := p14TestDigest("p13-identity")
	basis := syntheticFrozenP14BasisForP13()
	relativePath := filepath.Join(".context", "p13", "p13-acceptance.json")
	evidencePath := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o700); err != nil {
		t.Fatal(err)
	}
	evidence := syntheticP13EvidenceForP14(
		basis,
		identityDigest,
		p14RequiredP13Schema,
		relativePath,
	)
	input := preparedRequestOracleInput{
		P13Evidence: p13EvidenceBinding{
			CarrierPath:    filepath.ToSlash(relativePath),
			IdentityDigest: identityDigest,
		},
		FrozenBasis: basis,
	}
	currentRaw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, currentRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	input.P13Evidence.CarrierDigest = p14Digest(currentRaw)
	if err := verifyPreparedInputAgainstP13(root, input); err != nil {
		t.Fatalf("current P13 evidence schema rejected: %v", err)
	}

	evidence.Schema = "haft.p13.acceptance-evidence/v2"
	staleRaw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, staleRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	input.P13Evidence.CarrierDigest = p14Digest(staleRaw)
	if err := verifyPreparedInputAgainstP13(root, input); err == nil {
		t.Fatal("obsolete P13 evidence schema admitted as the current P14 basis")
	}
}

func TestP14SealPreparedRequestOracleCarrier(t *testing.T) {
	inputPath := os.Getenv(p14SealInputEnvironmentKey)
	if inputPath == "" {
		t.Skip("set HAFT_P14_SEAL_PREPARED_INPUT after P13 passes")
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
	canonicalInputPath, err := resolveP14PreparedInputPath(
		repositoryRoot,
		inputPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := loadPreparedRequestOracleInput(canonicalInputPath)
	if err != nil {
		t.Fatal(err)
	}
	contractDigest := p14Digest(rawContract)
	if input.ContractDigest != contractDigest {
		t.Fatal("P14 prepared input uses different request/oracle contract bytes")
	}
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreparedInputAgainstP13(repositoryRoot, input); err != nil {
		t.Fatal(err)
	}
	if err := verifyP13EvidenceFreshViaHarness(repositoryRoot, input.P13Evidence); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreparedInputCurrentBasis(repositoryRoot, input); err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistPreparedRequestOracleCarrier(
		repositoryRoot,
		contract,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P14_PREPARED_CARRIER path=%s digest=%s", path, digest)
}

func TestP14SealResolvesRepositoryRelativePreparedInput(t *testing.T) {
	repositoryRoot := t.TempDir()
	relativePath := filepath.Join(
		".context",
		"p14",
		p14PreparedInputFilePrefix+"fixture.json",
	)
	absolutePath := filepath.Join(repositoryRoot, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolutePath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveP14PreparedInputPath(
		repositoryRoot,
		filepath.ToSlash(relativePath),
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != expectedPath {
		t.Fatalf("resolved prepared input path = %q, want %q", resolved, expectedPath)
	}
}

func resolveP14PreparedInputPath(
	repositoryRoot string,
	path string,
) (string, error) {
	return resolveP14ExecutionCarrierPath(
		repositoryRoot,
		path,
		p14PreparedInputFilePrefix,
	)
}

func verifyP13EvidenceFreshViaHarness(
	repositoryRoot string,
	binding p13EvidenceBinding,
) error {
	goProgram, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("resolve Go for P13 freshness verification: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	args := []string{
		"test",
		"-p=1",
		"-count=1",
		"./internal/p13acceptance",
		"-run",
		"^TestP13VerifyAcceptanceEvidenceFresh$",
	}
	command := exec.CommandContext(ctx, goProgram, args...)
	command.Dir = repositoryRoot
	environment := os.Environ()
	environment, err = restoreP14AcceptedP13Environment(
		environment,
		goProgram,
	)
	if err != nil {
		return fmt.Errorf(
			"restore parent P13 PATH basis before nested go test: %w",
			err,
		)
	}
	environment = replaceP14EnvironmentValue(
		environment,
		p14P13EvidencePathKey,
		binding.CarrierPath,
	)
	environment = replaceP14EnvironmentValue(
		environment,
		p14P13EvidenceDigestKey,
		binding.CarrierDigest,
	)
	command.Env = environment
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("P13 evidence freshness verification timed out: %w", ctx.Err())
	}
	if err != nil {
		const outputLimit = 8 * 1024
		if len(output) > outputLimit {
			output = slices.Clone(output[len(output)-outputLimit:])
		}
		return fmt.Errorf(
			"P13 evidence freshness verification failed: %w\n%s",
			err,
			output,
		)
	}
	return nil
}

func restoreP14AcceptedP13Environment(
	environment []string,
	goProgram string,
) ([]string, error) {
	restored, err := restoreP14ParentGoTestPATH(environment, goProgram)
	if err != nil {
		return nil, err
	}
	// P13 binds these exact values into its acceptance identity. Recreate that
	// frozen execution contour for the nested freshness check instead of
	// stripping it and making every canonical P13 carrier appear stale.
	restored = replaceP14EnvironmentValue(restored, "GOFLAGS", "-p=1")
	restored = replaceP14EnvironmentValue(restored, "GOMAXPROCS", "1")
	return restored, nil
}

func restoreP14ParentGoTestPATH(
	environment []string,
	goProgram string,
) ([]string, error) {
	pathValue, found := p14EnvironmentValue(environment, "PATH")
	if !found {
		return nil, fmt.Errorf("parent go test PATH is absent")
	}
	pathEntries := filepath.SplitList(pathValue)
	if len(pathEntries) == 0 {
		return nil, fmt.Errorf("parent go test PATH is empty")
	}
	goBin := filepath.Clean(filepath.Dir(goProgram))
	firstEntry := filepath.Clean(pathEntries[0])
	if firstEntry != goBin {
		return nil, fmt.Errorf(
			"parent go test PATH starts with %q instead of resolved Go bin %q",
			firstEntry,
			goBin,
		)
	}
	parentPath := strings.Join(
		pathEntries[1:],
		string(os.PathListSeparator),
	)
	restored := replaceP14EnvironmentValue(
		environment,
		"PATH",
		parentPath,
	)
	return restored, nil
}

func p14EnvironmentValue(
	environment []string,
	key string,
) (string, bool) {
	prefix := key + "="
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		value := strings.TrimPrefix(entry, prefix)
		return value, true
	}
	return "", false
}

func replaceP14EnvironmentValue(
	environment []string,
	key string,
	value string,
) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, prefix+value)
	return filtered
}

func TestRestoreP14AcceptedP13EnvironmentRecoversAcceptedExecutionBasis(
	t *testing.T,
) {
	goProgram := filepath.Join(
		string(os.PathSeparator),
		"sdk",
		"bin",
		"go",
	)
	goBin := filepath.Dir(goProgram)
	acceptedPath := filepath.Join(
		string(os.PathSeparator),
		"usr",
		"bin",
	)
	outerPath := strings.Join(
		[]string{goBin, acceptedPath},
		string(os.PathListSeparator),
	)
	environment := []string{
		"GOFLAGS=-p=1",
		"GOMAXPROCS=1",
		"PATH=" + outerPath,
	}
	restored, err := restoreP14AcceptedP13Environment(
		environment,
		goProgram,
	)
	if err != nil {
		t.Fatal(err)
	}
	restoredPath, found := p14EnvironmentValue(restored, "PATH")
	if !found {
		t.Fatal("restored nested go test environment has no PATH")
	}
	innerPath := strings.Join(
		[]string{goBin, restoredPath},
		string(os.PathListSeparator),
	)
	if innerPath != outerPath {
		t.Fatalf(
			"nested go test PATH = %q, want accepted basis %q",
			innerPath,
			outerPath,
		)
	}
	goFlags, found := p14EnvironmentValue(restored, "GOFLAGS")
	if !found || goFlags != "-p=1" {
		t.Fatal("restored nested P13 environment omitted canonical GOFLAGS")
	}
	goMaxProcs, found := p14EnvironmentValue(restored, "GOMAXPROCS")
	if !found || goMaxProcs != "1" {
		t.Fatal("restored nested P13 environment omitted canonical GOMAXPROCS")
	}
}

func validatePreparedRequestOracleInput(
	contract requestOracleContract,
	input preparedRequestOracleInput,
) error {
	if input.Schema != p14PreparedInputSchema || input.Status != p14ContractStatus {
		return fmt.Errorf("P14 prepared input schema or status is invalid")
	}
	if input.ResultSemantics != p14PreparedSemantics || input.ReleaseClaim {
		return fmt.Errorf("P14 prepared input overstates Work, evidence, or release")
	}
	if input.ContractRef != p14ContractRelativePath || !validP14Digest(input.ContractDigest) {
		return fmt.Errorf("P14 prepared input contract binding is invalid")
	}
	if err := validateP13EvidenceBinding(input.P13Evidence); err != nil {
		return err
	}
	if err := validateFrozenP14Basis(input.FrozenBasis); err != nil {
		return err
	}
	if err := validatePreparedP14Bindings(contract, input.Bindings); err != nil {
		return err
	}
	if len(input.Scenarios) != len(contract.Scenarios) {
		return fmt.Errorf("P14 prepared scenario count = %d", len(input.Scenarios))
	}
	for index, scenario := range input.Scenarios {
		declared := contract.Scenarios[index]
		if err := validatePreparedP14Scenario(declared, scenario); err != nil {
			return err
		}
	}
	return nil
}

func validatePreparedP14Bindings(
	contract requestOracleContract,
	bindings []preparedP14Binding,
) error {
	if len(bindings) != len(contract.BindingGroups) {
		return fmt.Errorf("P14 prepared binding count = %d", len(bindings))
	}
	expectedStates := map[string]string{
		"candidate_basis":        p14BindingEmbeddedFrozen,
		"p13_evidence":           p14BindingEmbeddedFrozen,
		"selected_project_basis": p14BindingEmbeddedFrozen,
		"golden_memory_fixture":  p14BindingCarrierDigest,
		"init_matrix":            p14BindingCarrierDigest,
		"identifier_fixture":     p14BindingCarrierDigest,
		"restart_checkpoint":     p14BindingExecutionTime,
		"live_mcp_process":       p14BindingExecutionTime,
	}
	for index, binding := range bindings {
		expectedGroup := contract.BindingGroups[index]
		expectedState, present := expectedStates[expectedGroup]
		if !present || binding.Group != expectedGroup || binding.State != expectedState {
			return fmt.Errorf("P14 prepared binding %d policy differs", index)
		}
		if err := validatePreparedP14BindingCarrier(binding); err != nil {
			return err
		}
	}
	return nil
}

func validatePreparedP14BindingCarrier(binding preparedP14Binding) error {
	validators := map[string]func() error{
		p14BindingEmbeddedFrozen: func() error {
			if binding.CarrierPath != "" || binding.CarrierDigest != "" {
				return fmt.Errorf("P14 embedded binding %q duplicates its typed basis", binding.Group)
			}
			return nil
		},
		p14BindingCarrierDigest: func() error {
			clean := filepath.Clean(filepath.FromSlash(binding.CarrierPath))
			portable := filepath.ToSlash(clean)
			if filepath.IsAbs(clean) || clean == "." ||
				strings.HasPrefix(portable, "../") ||
				!strings.HasPrefix(portable, ".context/") ||
				!validP14Digest(binding.CarrierDigest) {
				return fmt.Errorf("P14 carrier binding %q is invalid", binding.Group)
			}
			return nil
		},
		p14BindingExecutionTime: func() error {
			if binding.CarrierPath != "" || binding.CarrierDigest != "" {
				return fmt.Errorf("P14 execution-time binding %q is prematurely materialized", binding.Group)
			}
			return nil
		},
	}
	validator, present := validators[binding.State]
	if !present {
		return fmt.Errorf("P14 binding %q has open state %q", binding.Group, binding.State)
	}
	return validator()
}

func validateP13EvidenceBinding(binding p13EvidenceBinding) error {
	clean := filepath.Clean(filepath.FromSlash(binding.CarrierPath))
	parent := filepath.Join(".context", "p13")
	if filepath.IsAbs(clean) || filepath.Dir(clean) != parent {
		return fmt.Errorf("P14 P13 evidence path is outside .context/p13")
	}
	if !validP14Digest(binding.CarrierDigest) ||
		!validP14Digest(binding.IdentityDigest) {
		return fmt.Errorf("P14 P13 evidence digest is invalid")
	}
	return nil
}

func validateFrozenP14Basis(basis frozenP14Basis) error {
	candidate := basis.Candidate
	if !fullP14GitRevision.MatchString(candidate.GitHead) ||
		!filepath.IsAbs(candidate.ExecutablePath) ||
		candidate.FPFRevision == "" || candidate.BaseTypeEnvRef == "" {
		return fmt.Errorf("P14 candidate basis identity is incomplete")
	}
	if err := validateP14CandidateBuildIdentity(candidate); err != nil {
		return err
	}
	if candidate.P13GitStatusDigest != p14CleanGitStatusDigest {
		return fmt.Errorf("P14 candidate P13 identity is not a clean Git checkout")
	}
	candidateDigests := []string{
		candidate.P13GitStatusDigest,
		candidate.DirtyStateDigest,
		candidate.ExecutableDigest,
		candidate.QueryDatabaseDigest,
		candidate.FPFSpecDigest,
		candidate.FPFReadmeDigest,
		candidate.BaseTypeEnvDigest,
	}
	if !allP14Digests(candidateDigests) {
		return fmt.Errorf("P14 candidate basis digest is invalid")
	}
	project := basis.SelectedProject
	if project.ProjectID == "" || !filepath.IsAbs(project.ProjectRoot) ||
		project.ProfileGeneration == "" || project.TypeEnvHeadRef == "" ||
		project.TypeEnvHeadRevision <= 0 || project.SelectedCompositeRef == "" ||
		project.GraphRevision <= 0 {
		return fmt.Errorf("P14 selected project basis is incomplete")
	}
	projectDigests := []string{
		project.ProfilePayloadDigest,
		project.ProfileBasisDigest,
		project.TypeEnvHeadStateDigest,
		project.GraphMaterializationDigest,
	}
	if !allP14Digests(projectDigests) {
		return fmt.Errorf("P14 selected project basis digest is invalid")
	}
	requiredKinds := []string{"agent_instructions", "skill_carriers_root", "mcp_config"}
	if len(basis.Carriers) != len(requiredKinds) {
		return fmt.Errorf("P14 frozen carrier count = %d", len(basis.Carriers))
	}
	seenKinds := make(map[string]struct{}, len(basis.Carriers))
	seenPaths := make(map[string]struct{}, len(basis.Carriers))
	for _, carrier := range basis.Carriers {
		if !slices.Contains(requiredKinds, carrier.Kind) ||
			!filepath.IsAbs(carrier.Path) || !validP14Digest(carrier.Digest) {
			return fmt.Errorf("P14 frozen carrier basis is invalid")
		}
		if _, duplicate := seenKinds[carrier.Kind]; duplicate {
			return fmt.Errorf("P14 frozen carrier kind %q is duplicated", carrier.Kind)
		}
		if _, duplicate := seenPaths[carrier.Path]; duplicate {
			return fmt.Errorf("P14 frozen carrier path %q is duplicated", carrier.Path)
		}
		seenKinds[carrier.Kind] = struct{}{}
		seenPaths[carrier.Path] = struct{}{}
	}
	return nil
}

func validateP14CandidateBuildIdentity(candidate candidateP14Basis) error {
	if candidate.Version != p14RequiredCandidateVersion ||
		candidate.VersionCommit != candidate.GitHead ||
		candidate.VersionModified == nil || *candidate.VersionModified ||
		candidate.BuildVCS != "git" ||
		candidate.BuildVCSRevision != candidate.GitHead ||
		candidate.BuildVCSModified == nil || *candidate.BuildVCSModified ||
		candidate.BuildVCSTime == "" {
		return fmt.Errorf("P14 candidate build identity is incomplete or inconsistent")
	}
	if _, err := time.Parse(time.RFC3339Nano, candidate.BuildVCSTime); err != nil {
		return fmt.Errorf("P14 candidate embedded VCS time is invalid: %w", err)
	}
	return nil
}

func validP14CandidateVersion(version string) bool {
	return version != "" && version == strings.TrimSpace(version) &&
		len(version) <= 128 && !strings.ContainsAny(version, "\r\n\t ")
}

type p14CandidateVersionObservation struct {
	Version          string
	Commit           string
	Modified         bool
	SourceTime       string
	BuildVCS         string
	BuildVCSRevision string
	BuildVCSModified bool
	BuildVCSTime     string
}

type p14CandidateVersionObserver func(
	string,
) (p14CandidateVersionObservation, error)

func syntheticP14CandidateVersionObservation(
	version string,
	revision string,
) p14CandidateVersionObservation {
	return p14CandidateVersionObservation{
		Version:          version,
		Commit:           revision,
		Modified:         false,
		SourceTime:       p14SyntheticBuildVCSTime,
		BuildVCS:         "git",
		BuildVCSRevision: revision,
		BuildVCSModified: false,
		BuildVCSTime:     p14SyntheticBuildVCSTime,
	}
}

type p14BuildVCSProvenance struct {
	VCS      string
	Revision string
	Modified bool
	Time     string
}

func observeP14CandidateVersion(
	executable string,
) (p14CandidateVersionObservation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "version")
	output, err := command.Output()
	if ctx.Err() != nil {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"observe P14 candidate version: %w",
			ctx.Err(),
		)
	}
	if err != nil {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"observe P14 candidate version: %w",
			err,
		)
	}
	observation, err := parseP14CandidateVersionOutput(output)
	if err != nil {
		return p14CandidateVersionObservation{}, err
	}
	info, err := buildinfo.ReadFile(executable)
	if err != nil {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"read P14 candidate embedded Go build identity: %w",
			err,
		)
	}
	provenance, err := p14BuildVCSProvenanceFromInfo(info)
	if err != nil {
		return p14CandidateVersionObservation{}, err
	}
	if observation.Commit != provenance.Revision ||
		observation.Modified != provenance.Modified ||
		observation.SourceTime != provenance.Time ||
		provenance.Modified {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"P14 candidate printed identity differs from embedded Go VCS identity",
		)
	}
	observation.BuildVCS = provenance.VCS
	observation.BuildVCSRevision = provenance.Revision
	observation.BuildVCSModified = provenance.Modified
	observation.BuildVCSTime = provenance.Time
	return observation, nil
}

func parseP14CandidateVersionOutput(
	output []byte,
) (p14CandidateVersionObservation, error) {
	if len(output) == 0 || output[len(output)-1] != '\n' ||
		strings.ContainsRune(string(output), '\r') {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"P14 candidate version output is invalid",
		)
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "haft ") {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"P14 candidate version output is invalid",
		)
	}
	observation := p14CandidateVersionObservation{
		Version: strings.TrimPrefix(lines[0], "haft "),
	}
	commitSeen := false
	builtSeen := false
	sourceSeen := false
	for _, line := range lines[1:] {
		switch {
		case strings.HasPrefix(line, "  commit:  ") && !commitSeen:
			observation.Commit = strings.TrimPrefix(line, "  commit:  ")
			commitSeen = true
		case strings.HasPrefix(line, "  built:   ") && !builtSeen:
			if strings.TrimPrefix(line, "  built:   ") == "" {
				return p14CandidateVersionObservation{}, fmt.Errorf(
					"P14 candidate version output is invalid",
				)
			}
			builtSeen = true
		case strings.HasPrefix(line, "  source:  ") && !sourceSeen:
			observation.SourceTime = strings.TrimPrefix(line, "  source:  ")
			if observation.SourceTime == "" {
				return p14CandidateVersionObservation{}, fmt.Errorf(
					"P14 candidate version output is invalid",
				)
			}
			sourceSeen = true
		case line == "  modified: true" && !observation.Modified:
			observation.Modified = true
		default:
			return p14CandidateVersionObservation{}, fmt.Errorf(
				"P14 candidate version output is invalid",
			)
		}
	}
	if !validP14CandidateVersion(observation.Version) ||
		!commitSeen || !builtSeen ||
		!fullP14GitRevision.MatchString(observation.Commit) {
		return p14CandidateVersionObservation{}, fmt.Errorf(
			"P14 candidate version identity is invalid",
		)
	}
	return observation, nil
}

func p14BuildVCSProvenanceFromInfo(
	info *debug.BuildInfo,
) (p14BuildVCSProvenance, error) {
	if info == nil ||
		info.Path != "github.com/m0n0x41d/haft/cmd/haft" ||
		info.Main.Path != "github.com/m0n0x41d/haft" {
		return p14BuildVCSProvenance{}, fmt.Errorf(
			"P14 candidate embedded Go build identity or module path is absent",
		)
	}
	values := make(map[string]string, 4)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs", "vcs.revision", "vcs.modified", "vcs.time":
			if _, duplicate := values[setting.Key]; duplicate {
				return p14BuildVCSProvenance{}, fmt.Errorf(
					"P14 candidate embedded Go build identity repeats %s",
					setting.Key,
				)
			}
			values[setting.Key] = setting.Value
		}
	}
	if values["vcs.modified"] != "false" {
		return p14BuildVCSProvenance{}, fmt.Errorf(
			"P14 candidate embedded vcs.modified must be the canonical false literal",
		)
	}
	provenance := p14BuildVCSProvenance{
		VCS:      values["vcs"],
		Revision: values["vcs.revision"],
		Modified: false,
		Time:     values["vcs.time"],
	}
	if provenance.VCS != "git" ||
		!fullP14GitRevision.MatchString(provenance.Revision) ||
		provenance.Time == "" {
		return p14BuildVCSProvenance{}, fmt.Errorf(
			"P14 candidate embedded Go VCS identity is incomplete",
		)
	}
	if _, err := time.Parse(time.RFC3339Nano, provenance.Time); err != nil {
		return p14BuildVCSProvenance{}, fmt.Errorf(
			"P14 candidate embedded vcs.time is invalid: %w",
			err,
		)
	}
	return provenance, nil
}

func TestP14CandidateVersionObservationRequiresExactBuildIdentity(t *testing.T) {
	commit := strings.Repeat("1", 40)
	sourceTime := "2026-08-11T14:19:50Z"
	clean, err := parseP14CandidateVersionOutput([]byte(
		"haft 9.2.0\n  commit:  " + commit +
			"\n  built:   exact\n  source:  " + sourceTime + "\n",
	))
	if err != nil {
		t.Fatal(err)
	}
	if clean.Version != "9.2.0" || clean.Commit != commit || clean.Modified ||
		clean.SourceTime != sourceTime {
		t.Fatalf("P14 candidate version identity differs: %#v", clean)
	}

	modified, err := parseP14CandidateVersionOutput([]byte(
		"haft 9.2.0\n  commit:  " + commit +
			"\n  built:   exact\n  source:  " + sourceTime +
			"\n  modified: true\n",
	))
	if err != nil || !modified.Modified {
		t.Fatalf("P14 candidate modified state was not observed: %#v, %v", modified, err)
	}

	for name, output := range map[string]string{
		"short_commit":   "haft 9.2.0\n  commit:  1111111\n  built:   exact\n",
		"missing_commit": "haft 9.2.0\n  built:   exact\n",
		"unknown_line":   "haft 9.2.0\n  commit:  " + commit + "\n  built:   exact\n  trusted: yes\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseP14CandidateVersionOutput([]byte(output)); err == nil {
				t.Fatal("P14 candidate version observation accepted open build identity")
			}
		})
	}
}

func TestP14EmbeddedGoBuildIdentityRejectsOpenOrNonCanonicalVCSSettings(
	t *testing.T,
) {
	valid := func() *debug.BuildInfo {
		return &debug.BuildInfo{
			Path: "github.com/m0n0x41d/haft/cmd/haft",
			Main: debug.Module{Path: "github.com/m0n0x41d/haft"},
			Settings: []debug.BuildSetting{
				{Key: "vcs", Value: "git"},
				{Key: "vcs.revision", Value: strings.Repeat("1", 40)},
				{Key: "vcs.modified", Value: "false"},
				{Key: "vcs.time", Value: p14SyntheticBuildVCSTime},
			},
		}
	}
	if _, err := p14BuildVCSProvenanceFromInfo(valid()); err != nil {
		t.Fatalf("canonical embedded Go VCS identity rejected: %v", err)
	}
	if _, err := p14BuildVCSProvenanceFromInfo(nil); err == nil {
		t.Fatal("nil Go build identity accepted")
	}

	tests := map[string]func(*debug.BuildInfo){
		"wrong_command_path": func(info *debug.BuildInfo) {
			info.Path = "github.com/m0n0x41d/haft/cmd/other"
		},
		"wrong_module_path": func(info *debug.BuildInfo) {
			info.Main.Path = "example.invalid/haft"
		},
		"missing_vcs": func(info *debug.BuildInfo) {
			info.Settings = info.Settings[1:]
		},
		"duplicate_vcs": func(info *debug.BuildInfo) {
			info.Settings = append(
				info.Settings,
				debug.BuildSetting{Key: "vcs", Value: "git"},
			)
		},
		"wrong_vcs": func(info *debug.BuildInfo) {
			info.Settings[0].Value = "hg"
		},
		"missing_revision": func(info *debug.BuildInfo) {
			info.Settings[1].Key = "unrelated"
		},
		"bad_revision": func(info *debug.BuildInfo) {
			info.Settings[1].Value = "1111111"
		},
		"missing_time": func(info *debug.BuildInfo) {
			info.Settings[3].Key = "unrelated"
		},
		"bad_time": func(info *debug.BuildInfo) {
			info.Settings[3].Value = "yesterday"
		},
		"modified_true": func(info *debug.BuildInfo) {
			info.Settings[2].Value = "true"
		},
		"modified_uppercase": func(info *debug.BuildInfo) {
			info.Settings[2].Value = "FALSE"
		},
		"modified_numeric": func(info *debug.BuildInfo) {
			info.Settings[2].Value = "0"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			info := valid()
			mutate(info)
			if _, err := p14BuildVCSProvenanceFromInfo(info); err == nil {
				t.Fatal("open or non-canonical embedded Go VCS identity accepted")
			}
		})
	}
}

type p14BuildInfoTestRepository struct {
	Root     string
	Revision string
	VCSTime  string
}

func newP14BuildInfoTestRepository(
	t *testing.T,
	marker string,
) p14BuildInfoTestRepository {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	commandRoot := filepath.Join(root, "cmd", "haft")
	if err := os.MkdirAll(commandRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	goMod := []byte("module github.com/m0n0x41d/haft\n\ngo 1.22\n")
	mainSource := []byte(fmt.Sprintf(`package main

import (
	"fmt"
	"os"
)

const buildMarker = %q

var (
	version = "dev"
	commit = "unknown"
	sourceTime = "unknown"
	modified = "false"
)

func main() {
	_ = buildMarker
	if len(os.Args) != 2 || os.Args[1] != "version" {
		os.Exit(2)
	}
	fmt.Printf("haft %%s\n  commit:  %%s\n  built:   fixture\n  source:  %%s\n", version, commit, sourceTime)
	if modified == "true" {
		fmt.Println("  modified: true")
	}
}
`, marker))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandRoot, "main.go"), mainSource, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "p14@example.invalid"},
		{"config", "user.name", "P14 fixture"},
		{"add", "go.mod", "cmd/haft/main.go"},
		{"commit", "-q", "-m", "P14 buildinfo fixture"},
	} {
		command := exec.Command("git", args...)
		command.Dir = root
		command.Env = append(
			os.Environ(),
			"GIT_AUTHOR_DATE="+p14SyntheticBuildVCSTime,
			"GIT_COMMITTER_DATE="+p14SyntheticBuildVCSTime,
		)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
	revisionCommand := exec.Command("git", "rev-parse", "HEAD")
	revisionCommand.Dir = root
	revisionRaw, err := revisionCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	return p14BuildInfoTestRepository{
		Root:     root,
		Revision: strings.TrimSpace(string(revisionRaw)),
		VCSTime:  p14SyntheticBuildVCSTime,
	}
}

func (repository p14BuildInfoTestRepository) build(
	t *testing.T,
	name string,
	goos string,
	goarch string,
	buildVCS bool,
	printedRevision string,
) (string, []byte) {
	t.Helper()
	outputRoot := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(outputRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(outputRoot, name)
	buildVCSValue := "false"
	if buildVCS {
		buildVCSValue = "true"
	}
	ldflags := strings.Join([]string{
		"-X=main.version=" + p14RequiredCandidateVersion,
		"-X=main.commit=" + printedRevision,
		"-X=main.sourceTime=" + repository.VCSTime,
		"-X=main.modified=false",
	}, " ")
	command := exec.Command(
		"go", "build",
		"-buildvcs="+buildVCSValue,
		"-trimpath",
		"-ldflags", ldflags,
		"-o", outputPath,
		"./cmd/haft",
	)
	command.Dir = repository.Root
	command.Env = append(
		os.Environ(),
		"GOWORK=off",
		"CGO_ENABLED=0",
		"GOOS="+goos,
		"GOARCH="+goarch,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build P14 buildinfo fixture: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	return outputPath, raw
}

func TestP14CandidateVersionObservationRequiresStampedGoVCS(t *testing.T) {
	repository := newP14BuildInfoTestRepository(t, "candidate")
	canonicalPath, _ := repository.build(
		t,
		"canonical",
		runtime.GOOS,
		runtime.GOARCH,
		true,
		repository.Revision,
	)
	observation, err := observeP14CandidateVersion(canonicalPath)
	if err != nil {
		t.Fatalf("canonical Go VCS candidate rejected: %v", err)
	}
	if observation.BuildVCSRevision != repository.Revision ||
		observation.BuildVCSTime != repository.VCSTime {
		t.Fatalf("canonical Go VCS candidate differs: %#v", observation)
	}

	withoutVCSPath, _ := repository.build(
		t,
		"without-vcs",
		runtime.GOOS,
		runtime.GOARCH,
		false,
		repository.Revision,
	)
	if _, err := observeP14CandidateVersion(withoutVCSPath); err == nil {
		t.Fatal("P14 candidate accepted spoofed printed identity built with -buildvcs=false")
	}

	mismatchedPath, _ := repository.build(
		t,
		"mismatched",
		runtime.GOOS,
		runtime.GOARCH,
		true,
		strings.Repeat("2", 40),
	)
	if _, err := observeP14CandidateVersion(mismatchedPath); err == nil {
		t.Fatal("P14 candidate accepted printed identity differing from embedded VCS")
	}
}

func TestP14FrozenBasisRejectsDirtyP13EvenWhenVersionClaimsClean(t *testing.T) {
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	input, err := completePreparedInputForTest(contract, p14Digest(rawContract))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFrozenP14Basis(input.FrozenBasis); err != nil {
		t.Fatalf("clean synthetic P14 basis rejected: %v", err)
	}
	if input.FrozenBasis.Candidate.VersionModified == nil ||
		*input.FrozenBasis.Candidate.VersionModified {
		t.Fatal("synthetic P14 candidate must make the spoofed clean-version case explicit")
	}

	dirty := input.FrozenBasis
	dirty.Candidate.P13GitStatusDigest = p14TestDigest("non-empty-git-status")
	if err := validateFrozenP14Basis(dirty); err == nil {
		t.Fatal("P14 frozen basis accepted dirty P13 while candidate version claimed clean")
	}

	wrongVersion := input.FrozenBasis
	wrongVersion.Candidate.Version = "9.1.0"
	if err := validateFrozenP14Basis(wrongVersion); err == nil {
		t.Fatal("P14 frozen basis accepted a predecessor candidate version")
	}
}

func validatePreparedP14Scenario(
	declared scenarioContract,
	scenario preparedP14Scenario,
) error {
	if scenario.ID != declared.ID || len(scenario.Requests) != len(declared.Surfaces) {
		return fmt.Errorf("P14 prepared scenario %q shape differs", scenario.ID)
	}
	semanticRequest := []byte(scenario.SemanticRequestCanonical)
	if !canonicalCompactJSON(semanticRequest) ||
		p14Digest(semanticRequest) != scenario.SemanticRequestDigest {
		return fmt.Errorf("P14 scenario %q semantic request is not exact canonical JSON", scenario.ID)
	}
	for index, request := range scenario.Requests {
		if request.Surface != declared.Surfaces[index] ||
			request.Builder != declared.RequestBuilder {
			return fmt.Errorf("P14 scenario %q request surface or builder differs", scenario.ID)
		}
		if request.Encoding != "canonical_json" &&
			request.Encoding != "argv_json" &&
			request.Encoding != "observation_protocol_json" {
			return fmt.Errorf("P14 scenario %q request encoding is invalid", scenario.ID)
		}
		payload := []byte(request.CanonicalPayload)
		if !canonicalCompactJSON(payload) || p14Digest(payload) != request.PayloadDigest {
			return fmt.Errorf("P14 scenario %q request payload is not exact canonical JSON", scenario.ID)
		}
		if request.SemanticRequestDigest != scenario.SemanticRequestDigest {
			return fmt.Errorf("P14 scenario %q surfaces do not share one semantic request", scenario.ID)
		}
	}
	validators := p14ExecutableScenarioValidators()
	validator, present := validators[declared.RequestBuilder]
	if !present {
		return fmt.Errorf(
			"P14 request builder %q has no executable validator",
			declared.RequestBuilder,
		)
	}
	if err := validator(declared, scenario); err != nil {
		return err
	}
	return validatePreparedP14Oracle(declared, scenario.Oracle)
}

type p14PreparedScenarioValidator func(
	scenarioContract,
	preparedP14Scenario,
) error

func p14ExecutableScenarioValidators() map[string]p14PreparedScenarioValidator {
	validators := map[string]p14PreparedScenarioValidator{
		p14ExistingRecordBackfillBuilderID: validateP14ExistingRecordBackfillPreparedScenario,
		p14FPFProjectionBuilderID:          validateP14FPFProjectionPreparedScenario,
		p14IdentifierNamespaceBuilderID:    validateP14IdentifierNamespacePreparedScenario,
		p14InitMatrixBuilderID:             validateP14InitMatrixPreparedScenario,
		p14OnboardProfileChangeBuilderID:   validateP14OnboardProfileChangePreparedScenario,
		p14SpecSectionProtocolBuilderID:    validateP14SpecSectionProtocolPreparedScenario,
	}
	for _, builderID := range p14CodeExploreBuilderIDs {
		validators[builderID] = validateP14CodeExplorePreparedScenario
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		validators[builderID] = validateP14MemoryReadPreparedScenario
	}
	for _, builderID := range p14LiveProtocolBuilderIDs {
		validators[builderID] = validateP14LiveProtocolPreparedScenario
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		validators[builderID] = validateP14MemoryOperationPreparedScenario
	}
	return validators
}

func executableP14Contract(contract requestOracleContract) requestOracleContract {
	validators := p14ExecutableScenarioValidators()
	scenarios := make([]scenarioContract, 0, len(contract.Scenarios))
	for _, scenario := range contract.Scenarios {
		if _, present := validators[scenario.RequestBuilder]; !present {
			continue
		}
		scenarios = append(scenarios, scenario)
	}
	result := contract
	result.Scenarios = scenarios
	return result
}

func missingP14ExecutableBuilders(contract requestOracleContract) []string {
	validators := p14ExecutableScenarioValidators()
	missing := make([]string, 0, len(contract.Scenarios))
	for _, scenario := range contract.Scenarios {
		if _, present := validators[scenario.RequestBuilder]; present {
			continue
		}
		missing = append(missing, scenario.RequestBuilder)
	}
	return missing
}

func validatePreparedP14Oracle(
	declared scenarioContract,
	oracle preparedP14Oracle,
) error {
	if oracle.Kind != declared.OracleKind ||
		oracle.ExpectedEffect != declared.ExpectedEffect ||
		!validP14Digest(oracle.LocalOracleOutputDigest) {
		return fmt.Errorf("P14 scenario %q oracle basis differs", declared.ID)
	}
	if oracle.Kind == "normalized_digest" {
		if oracle.NormalizationID == "" ||
			!validP14Digest(oracle.ExpectedResultDigest) ||
			len(oracle.PredicateIDs) != 0 {
			return fmt.Errorf("P14 scenario %q normalized oracle is incomplete", declared.ID)
		}
		return nil
	}
	if oracle.Kind == "live_predicate" {
		if oracle.NormalizationID != "" || oracle.ExpectedResultDigest != "" ||
			len(oracle.PredicateIDs) == 0 || hasBlankOrDuplicate(oracle.PredicateIDs) {
			return fmt.Errorf("P14 scenario %q live predicate oracle is incomplete", declared.ID)
		}
		return nil
	}
	return fmt.Errorf("P14 scenario %q oracle kind is open", declared.ID)
}

func loadPreparedRequestOracleInput(
	path string,
) (preparedRequestOracleInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return preparedRequestOracleInput{}, fmt.Errorf("read P14 prepared input: %w", err)
	}
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var input preparedRequestOracleInput
	if err := decoder.Decode(&input); err != nil {
		return preparedRequestOracleInput{}, fmt.Errorf("decode P14 prepared input: %w", err)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return preparedRequestOracleInput{}, fmt.Errorf("P14 prepared input has trailing JSON")
	}
	return input, nil
}

func verifyPreparedInputAgainstP13(
	repositoryRoot string,
	input preparedRequestOracleInput,
) error {
	evidencePath := filepath.FromSlash(input.P13Evidence.CarrierPath)
	path := filepath.Join(repositoryRoot, evidencePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P13 evidence for P14: %w", err)
	}
	if p14Digest(raw) != input.P13Evidence.CarrierDigest {
		return fmt.Errorf("P14 P13 evidence bytes changed")
	}
	var evidence p13EvidenceEnvelope
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return fmt.Errorf("decode P13 evidence for P14: %w", err)
	}
	if err := validatePassingP13Evidence(input.P13Evidence, evidence); err != nil {
		return fmt.Errorf(
			"P14 input is not bound to one passing unchanged P13 identity: %w",
			err,
		)
	}
	return comparePreparedBasisToP13(input.FrozenBasis, evidence.StartIdentity)
}

func syntheticFrozenP14BasisForP13() frozenP14Basis {
	clean := false
	return frozenP14Basis{
		Candidate: candidateP14Basis{
			GitHead:            strings.Repeat("1", 40),
			Version:            p14RequiredCandidateVersion,
			VersionCommit:      strings.Repeat("1", 40),
			VersionModified:    &clean,
			BuildVCS:           "git",
			BuildVCSRevision:   strings.Repeat("1", 40),
			BuildVCSModified:   &clean,
			BuildVCSTime:       p14SyntheticBuildVCSTime,
			P13GitStatusDigest: p14CleanGitStatusDigest,
			FPFRevision:        strings.Repeat("2", 40),
			FPFSpecDigest:      p14TestDigest("fpf-spec"),
			FPFReadmeDigest:    p14TestDigest("fpf-readme"),
			BaseTypeEnvRef:     "typeenv:" + p14TestDigest("base"),
			BaseTypeEnvDigest:  p14TestDigest("base"),
		},
		SelectedProject: selectedProjectP14Basis{
			ProjectID:                  "qnt_e3149c17",
			ProjectRoot:                "/project",
			ProfileGeneration:          "v1",
			ProfilePayloadDigest:       p14TestDigest("profile"),
			ProfileBasisDigest:         p14TestDigest("profile-basis"),
			TypeEnvHeadRef:             "project-typeenv-head:qnt_e3149c17",
			TypeEnvHeadRevision:        2,
			SelectedCompositeRef:       "typeenv:" + p14TestDigest("composite"),
			TypeEnvHeadStateDigest:     p14TestDigest("head"),
			GraphRevision:              8,
			GraphMaterializationDigest: p14TestDigest("graph"),
		},
	}
}

func syntheticP13EvidenceForP14(
	basis frozenP14Basis,
	identityDigest string,
	schema string,
	carrierPath string,
) p13EvidenceEnvelope {
	identity := p13IdentityEnvelope{
		ProjectID:   basis.SelectedProject.ProjectID,
		ProjectRoot: basis.SelectedProject.ProjectRoot,
		Git: p13GitEnvelope{
			Head:         basis.Candidate.GitHead,
			StatusDigest: basis.Candidate.P13GitStatusDigest,
		},
		FPF: p13FPFEnvelope{
			Head:         basis.Candidate.FPFRevision,
			SpecDigest:   basis.Candidate.FPFSpecDigest,
			ReadmeDigest: basis.Candidate.FPFReadmeDigest,
			Embedded: p13EmbeddedFPFEnvelope{
				BaseTypeEnvRef:    basis.Candidate.BaseTypeEnvRef,
				BaseTypeEnvDigest: basis.Candidate.BaseTypeEnvDigest,
			},
		},
		Profile: p13ProfileEnvelope{
			Generation:    basis.SelectedProject.ProfileGeneration,
			PayloadDigest: basis.SelectedProject.ProfilePayloadDigest,
			BasisDigest:   basis.SelectedProject.ProfileBasisDigest,
		},
		TypeEnv: p13TypeEnvEnvelope{
			HeadRef:              basis.SelectedProject.TypeEnvHeadRef,
			HeadRevision:         basis.SelectedProject.TypeEnvHeadRevision,
			SelectedCompositeRef: basis.SelectedProject.SelectedCompositeRef,
			StateDigest:          basis.SelectedProject.TypeEnvHeadStateDigest,
		},
		Graph: p13GraphEnvelope{
			Revision:              basis.SelectedProject.GraphRevision,
			MaterializationDigest: basis.SelectedProject.GraphMaterializationDigest,
		},
		Digest: identityDigest,
	}
	return p13EvidenceEnvelope{
		Schema:            schema,
		Status:            "passed",
		ReleaseClaim:      false,
		CarrierPath:       filepath.ToSlash(carrierPath),
		IdentityDigest:    identityDigest,
		IdentityUnchanged: true,
		StartIdentity:     identity,
		EndIdentity:       identity,
	}
}

func comparePreparedBasisToP13(
	basis frozenP14Basis,
	identity p13IdentityEnvelope,
) error {
	candidate := basis.Candidate
	project := basis.SelectedProject
	matches := []bool{
		candidate.GitHead == identity.Git.Head,
		candidate.P13GitStatusDigest == identity.Git.StatusDigest,
		candidate.FPFRevision == identity.FPF.Head,
		candidate.FPFSpecDigest == identity.FPF.SpecDigest,
		candidate.FPFReadmeDigest == identity.FPF.ReadmeDigest,
		candidate.BaseTypeEnvRef == identity.FPF.Embedded.BaseTypeEnvRef,
		candidate.BaseTypeEnvDigest == identity.FPF.Embedded.BaseTypeEnvDigest,
		project.ProjectID == identity.ProjectID,
		project.ProjectRoot == identity.ProjectRoot,
		project.ProfileGeneration == identity.Profile.Generation,
		project.ProfilePayloadDigest == identity.Profile.PayloadDigest,
		project.ProfileBasisDigest == identity.Profile.BasisDigest,
		project.TypeEnvHeadRef == identity.TypeEnv.HeadRef,
		project.TypeEnvHeadRevision == identity.TypeEnv.HeadRevision,
		project.SelectedCompositeRef == identity.TypeEnv.SelectedCompositeRef,
		project.TypeEnvHeadStateDigest == identity.TypeEnv.StateDigest,
		project.GraphRevision == identity.Graph.Revision,
		project.GraphMaterializationDigest == identity.Graph.MaterializationDigest,
	}
	if slices.Contains(matches, false) {
		return fmt.Errorf("P14 frozen basis differs from the passing P13 identity")
	}
	return nil
}

func verifyPreparedInputCurrentBasis(
	repositoryRoot string,
	input preparedRequestOracleInput,
) error {
	observer := agenthostrestart.NewOSEvidence()
	return verifyPreparedInputCurrentBasisWithObserver(
		context.Background(),
		repositoryRoot,
		input,
		observer.CapturePreparation,
		observeP14CandidateVersion,
	)
}

type p14PreparationEvidenceObserver func(
	context.Context,
	agenthostrestart.PreparationRequest,
) (agenthostrestart.PreparationEvidence, error)

func verifyPreparedInputCurrentBasisWithObserver(
	ctx context.Context,
	repositoryRoot string,
	input preparedRequestOracleInput,
	observe p14PreparationEvidenceObserver,
	observeCandidate p14CandidateVersionObserver,
) error {
	candidate := input.FrozenBasis.Candidate
	carriers := input.FrozenBasis.Carriers
	carrierByKind := make(map[string]installedCarrierBasis, len(carriers))
	for _, carrier := range carriers {
		carrierByKind[carrier.Kind] = carrier
	}
	request := agenthostrestart.PreparationRequest{
		ProjectRoot:         repositoryRoot,
		CandidateHaftBinary: candidate.ExecutablePath,
		SkillCarriersRoot:   carrierByKind["skill_carriers_root"].Path,
		InstructionCarrier:  carrierByKind["agent_instructions"].Path,
		MCPConfigCarrier:    carrierByKind["mcp_config"].Path,
	}
	evidence, err := observe(ctx, request)
	if err != nil {
		return fmt.Errorf("capture current P14 restart basis: %w", err)
	}
	version, err := observeCandidate(candidate.ExecutablePath)
	if err != nil {
		return err
	}
	selectedDigest := strings.TrimPrefix(
		input.FrozenBasis.SelectedProject.SelectedCompositeRef,
		"typeenv:",
	)
	matches := []bool{
		version.Version == candidate.Version,
		version.Commit == candidate.VersionCommit,
		candidate.VersionModified != nil &&
			version.Modified == *candidate.VersionModified,
		version.BuildVCS == candidate.BuildVCS,
		version.BuildVCSRevision == candidate.BuildVCSRevision,
		candidate.BuildVCSModified != nil &&
			version.BuildVCSModified == *candidate.BuildVCSModified,
		version.BuildVCSTime == candidate.BuildVCSTime,
		evidence.RepositoryHead == candidate.GitHead,
		evidence.DirtyStateDigest == candidate.DirtyStateDigest,
		evidence.DesiredHaftBinaryDigest == candidate.ExecutableDigest,
		evidence.ExpectedFPFRevision == candidate.FPFRevision,
		evidence.ExpectedTypeEnvDigest == selectedDigest,
		evidence.ExpectedTypeEnvHeadRevision == uint64(input.FrozenBasis.SelectedProject.TypeEnvHeadRevision),
		evidence.ExpectedGraphRevision == uint64(input.FrozenBasis.SelectedProject.GraphRevision),
		evidence.ExpectedSkillCarriersDigest == carrierByKind["skill_carriers_root"].Digest,
		evidence.ExpectedInstructionDigest == carrierByKind["agent_instructions"].Digest,
		evidence.ExpectedMCPConfigDigest == carrierByKind["mcp_config"].Digest,
	}
	if slices.Contains(matches, false) {
		return fmt.Errorf("current P14 restart basis differs from the prepared input")
	}
	queryDatabase := filepath.Join(repositoryRoot, "internal", "cli", "fpf.db")
	if err := verifyP14FileDigest(queryDatabase, candidate.QueryDatabaseDigest); err != nil {
		return err
	}
	fpfSpecification := filepath.Join(repositoryRoot, "data", "FPF", "FPF-Spec.md")
	if err := verifyP14FileDigest(fpfSpecification, candidate.FPFSpecDigest); err != nil {
		return err
	}
	fpfReadme := filepath.Join(repositoryRoot, "data", "FPF", "Readme.md")
	if err := verifyP14FileDigest(fpfReadme, candidate.FPFReadmeDigest); err != nil {
		return err
	}
	if err := verifyPreparedP14BindingCarriers(repositoryRoot, input.Bindings); err != nil {
		return err
	}
	if err := verifyP14InitMatrixFixtureBinding(repositoryRoot, input); err != nil {
		return err
	}
	if err := verifyP14IdentifierFixtureBinding(repositoryRoot, input); err != nil {
		return err
	}
	return verifyP14MemoryReadFixtureBinding(repositoryRoot, input)
}

func verifyPreparedP14BindingCarriers(
	repositoryRoot string,
	bindings []preparedP14Binding,
) error {
	for _, binding := range bindings {
		if binding.State != p14BindingCarrierDigest {
			continue
		}
		path := filepath.Join(
			repositoryRoot,
			filepath.FromSlash(binding.CarrierPath),
		)
		if err := verifyP14FileDigest(path, binding.CarrierDigest); err != nil {
			return fmt.Errorf("verify P14 binding %q: %w", binding.Group, err)
		}
	}
	return nil
}

func verifyP14FileDigest(path string, expected string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read P14 frozen file %s: %w", path, err)
	}
	if p14Digest(raw) != expected {
		return fmt.Errorf("P14 frozen file %s digest changed", path)
	}
	return nil
}

func persistPreparedRequestOracleCarrier(
	repositoryRoot string,
	contract requestOracleContract,
	input preparedRequestOracleInput,
) (string, string, error) {
	if err := validatePreparedRequestOracleInput(contract, input); err != nil {
		return "", "", err
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return "", "", fmt.Errorf("encode P14 preparation digest basis: %w", err)
	}
	preparationDigest := p14Digest(inputBytes)
	digestBody := strings.TrimPrefix(preparationDigest, "sha256:")
	name := "p14-prepared-request-oracle-" + digestBody[:16] + ".json"
	carrierPath := filepath.Join(".context", "p14", name)
	carrier := preparedRequestOracleCarrier{
		Schema:            p14PreparedCarrierSchema,
		Status:            p14ContractStatus,
		CarrierPath:       filepath.ToSlash(carrierPath),
		PreparationDigest: preparationDigest,
		Preparation:       input,
	}
	if err := verifyPreparedRequestOracleCarrier(contract, carrier); err != nil {
		return "", "", err
	}
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("encode P14 prepared carrier: %w", err)
	}
	canonical = append(canonical, '\n')
	if err := publishP14NoClobber(repositoryRoot, carrierPath, canonical); err != nil {
		return "", "", err
	}
	return carrier.CarrierPath, p14Digest(canonical), nil
}

func decodePreparedRequestOracleCarrier(
	contract requestOracleContract,
	raw []byte,
) (preparedRequestOracleCarrier, error) {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var carrier preparedRequestOracleCarrier
	if err := decoder.Decode(&carrier); err != nil {
		return preparedRequestOracleCarrier{}, fmt.Errorf(
			"decode P14 prepared carrier: %w",
			err,
		)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return preparedRequestOracleCarrier{}, fmt.Errorf(
			"P14 prepared carrier has trailing JSON",
		)
	}
	if err := verifyPreparedRequestOracleCarrier(contract, carrier); err != nil {
		return preparedRequestOracleCarrier{}, err
	}
	canonical, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return preparedRequestOracleCarrier{}, fmt.Errorf(
			"reencode P14 prepared carrier: %w",
			err,
		)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return preparedRequestOracleCarrier{}, fmt.Errorf(
			"P14 prepared carrier is not canonical JSON",
		)
	}
	return carrier, nil
}

func verifyPreparedRequestOracleCarrier(
	contract requestOracleContract,
	carrier preparedRequestOracleCarrier,
) error {
	if carrier.Schema != p14PreparedCarrierSchema ||
		carrier.Status != p14ContractStatus ||
		!validP14Digest(carrier.PreparationDigest) {
		return fmt.Errorf("P14 prepared carrier header is invalid")
	}
	if err := validatePreparedRequestOracleInput(contract, carrier.Preparation); err != nil {
		return err
	}
	inputBytes, err := json.Marshal(carrier.Preparation)
	if err != nil {
		return fmt.Errorf("redigest P14 preparation basis: %w", err)
	}
	wantDigest := p14Digest(inputBytes)
	digestBody := strings.TrimPrefix(wantDigest, "sha256:")
	wantName := "p14-prepared-request-oracle-" + digestBody[:16] + ".json"
	wantPath := filepath.Join(".context", "p14", wantName)
	wantPath = filepath.ToSlash(wantPath)
	if carrier.PreparationDigest != wantDigest || carrier.CarrierPath != wantPath {
		return fmt.Errorf("P14 prepared carrier path or preparation digest differs")
	}
	return nil
}

func publishP14NoClobber(
	repositoryRoot string,
	carrierPath string,
	content []byte,
) error {
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return fmt.Errorf("open P14 repository root: %w", err)
	}
	defer root.Close()
	directoryPath := filepath.Dir(carrierPath)
	directory := filepath.ToSlash(directoryPath)
	if err := root.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create P14 carrier directory: %w", err)
	}
	if err := syncP14Directory(root, ".", "repository root"); err != nil {
		return err
	}
	if err := syncP14Directory(root, ".context", "context directory"); err != nil {
		return err
	}
	carrierRoot, err := root.OpenRoot(directory)
	if err != nil {
		return fmt.Errorf("open P14 carrier directory: %w", err)
	}
	defer carrierRoot.Close()
	finalName := filepath.Base(carrierPath)
	temporaryName, temporary, err := createP14Temporary(carrierRoot, finalName)
	if err != nil {
		return err
	}
	temporaryPresent := true
	defer func() {
		if temporaryPresent {
			_ = carrierRoot.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write P14 prepared carrier temporary: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync P14 prepared carrier temporary: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close P14 prepared carrier temporary: %w", err)
	}
	if err := carrierRoot.Link(temporaryName, finalName); err != nil {
		return fmt.Errorf("publish P14 prepared carrier without replacement: %w", err)
	}
	if err := carrierRoot.Remove(temporaryName); err != nil {
		return fmt.Errorf("remove P14 prepared carrier temporary: %w", err)
	}
	temporaryPresent = false
	if err := syncP14Directory(carrierRoot, ".", "carrier directory"); err != nil {
		return err
	}
	observed, err := carrierRoot.ReadFile(finalName)
	if err != nil {
		return fmt.Errorf("reread P14 prepared carrier: %w", err)
	}
	if !bytes.Equal(observed, content) {
		return fmt.Errorf("reread P14 prepared carrier changed bytes")
	}
	return nil
}

func syncP14Directory(root *os.Root, path string, label string) error {
	directory, err := root.Open(path)
	if err != nil {
		return fmt.Errorf("open P14 %s for sync: %w", label, err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync P14 %s: %w", label, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close P14 %s: %w", label, closeErr)
	}
	return nil
}

func createP14Temporary(
	root *os.Root,
	finalName string,
) (string, *os.File, error) {
	for range 16 {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, fmt.Errorf("generate P14 temporary name: %w", err)
		}
		suffix := hex.EncodeToString(random)
		name := "." + finalName + "." + suffix + ".tmp"
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if os.IsExist(err) {
			continue
		}
		return "", nil, fmt.Errorf("create P14 temporary carrier: %w", err)
	}
	return "", nil, fmt.Errorf("create unique P14 temporary carrier")
}

func completePreparedInputForTest(
	contract requestOracleContract,
	contractDigest string,
) (preparedRequestOracleInput, error) {
	clean := false
	memoryFixture := syntheticP14MemoryReadFixture()
	sources := p14PreparedScenarioSources{
		Context:               context.Background(),
		Executable:            "/synthetic/haft",
		ProjectRoot:           "/synthetic/project",
		ExecutableDigest:      p14TestDigest("synthetic-candidate"),
		CandidateVersion:      p14RequiredCandidateVersion,
		ExpectedFPFRevision:   strings.Repeat("a", 40),
		MemoryFixture:         memoryFixture,
		InitMatrixFixture:     syntheticP14InitMatrixFixture(),
		IdentifierFixture:     syntheticP14IdentifierFixture(),
		FPFProjectionExecutor: executeSyntheticP14FPFProjectionCandidate,
		CodeExploreExecutor:   executeSyntheticP14CodeExploreCandidate,
		MemoryReadExecutor:    executeSyntheticP14MemoryReadCandidate,
	}
	scenarios, err := buildP14PreparedScenarios(contract, sources)
	if err != nil {
		return preparedRequestOracleInput{}, err
	}
	return preparedRequestOracleInput{
		Schema:          p14PreparedInputSchema,
		Status:          p14ContractStatus,
		ResultSemantics: p14PreparedSemantics,
		ReleaseClaim:    false,
		ContractRef:     p14ContractRelativePath,
		ContractDigest:  contractDigest,
		P13Evidence: p13EvidenceBinding{
			CarrierPath:    ".context/p13/p13-acceptance-test.json",
			CarrierDigest:  p14TestDigest("p13-carrier"),
			IdentityDigest: p14TestDigest("p13-identity"),
		},
		FrozenBasis: frozenP14Basis{
			Candidate: candidateP14Basis{
				GitHead:             strings.Repeat("1", 40),
				Version:             p14RequiredCandidateVersion,
				VersionCommit:       strings.Repeat("1", 40),
				VersionModified:     &clean,
				BuildVCS:            "git",
				BuildVCSRevision:    strings.Repeat("1", 40),
				BuildVCSModified:    &clean,
				BuildVCSTime:        p14SyntheticBuildVCSTime,
				P13GitStatusDigest:  p14CleanGitStatusDigest,
				DirtyStateDigest:    p14TestDigest("dirty-state"),
				ExecutablePath:      "/private/tmp/haft-v9-p14-candidate",
				ExecutableDigest:    p14TestDigest("candidate"),
				QueryDatabaseDigest: p14TestDigest("query-db"),
				FPFRevision:         strings.Repeat("2", 40),
				FPFSpecDigest:       p14TestDigest("fpf-spec"),
				FPFReadmeDigest:     p14TestDigest("fpf-readme"),
				BaseTypeEnvRef:      "typeenv:" + p14TestDigest("base-ref"),
				BaseTypeEnvDigest:   p14TestDigest("base"),
			},
			SelectedProject: selectedProjectP14Basis{
				ProjectID:                  "qnt_e3149c17",
				ProjectRoot:                "/project",
				ProfileGeneration:          "v2",
				ProfilePayloadDigest:       p14TestDigest("profile"),
				ProfileBasisDigest:         p14TestDigest("profile-basis"),
				TypeEnvHeadRef:             "project-typeenv-head:test",
				TypeEnvHeadRevision:        2,
				SelectedCompositeRef:       "typeenv:" + p14TestDigest("composite"),
				TypeEnvHeadStateDigest:     p14TestDigest("head"),
				GraphRevision:              4,
				GraphMaterializationDigest: p14TestDigest("graph"),
			},
			Carriers: []installedCarrierBasis{
				{Kind: "agent_instructions", Path: "/project/AGENTS.md", Digest: p14TestDigest("agents")},
				{Kind: "skill_carriers_root", Path: "/skills", Digest: p14TestDigest("skill")},
				{Kind: "mcp_config", Path: "/project/.codex/config.toml", Digest: p14TestDigest("mcp")},
			},
		},
		Bindings:  syntheticPreparedP14Bindings(contract.BindingGroups),
		Scenarios: scenarios,
	}, nil
}

type p14PreparedScenarioSources struct {
	Context               context.Context
	Executable            string
	ProjectRoot           string
	ExecutableDigest      string
	CandidateVersion      string
	ExpectedFPFRevision   string
	MemoryFixture         p14MemoryReadFixture
	InitMatrixFixture     p14InitMatrixFixture
	IdentifierFixture     p14IdentifierFixture
	FPFProjectionExecutor p14FPFProjectionExecutor
	CodeExploreExecutor   p14CodeExploreExecutor
	MemoryReadExecutor    p14MemoryReadExecutor
}

type p14PreparedScenarioBuilder func(
	scenarioContract,
) (preparedP14Scenario, error)

func buildP14PreparedScenarios(
	contract requestOracleContract,
	sources p14PreparedScenarioSources,
) ([]preparedP14Scenario, error) {
	builders := p14PreparedScenarioBuilders(sources)
	scenarios := make([]preparedP14Scenario, 0, len(contract.Scenarios))
	for _, declared := range contract.Scenarios {
		builder, present := builders[declared.RequestBuilder]
		if !present {
			return nil, fmt.Errorf(
				"P14 request builder %q has no executable fixture builder",
				declared.RequestBuilder,
			)
		}
		scenario, err := builder(declared)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}
	return scenarios, nil
}

func p14PreparedScenarioBuilders(
	sources p14PreparedScenarioSources,
) map[string]p14PreparedScenarioBuilder {
	builders := map[string]p14PreparedScenarioBuilder{
		p14FPFProjectionBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14FPFQueryProjectionScenario(
				sources.Context,
				declared,
				sources.Executable,
				sources.ExecutableDigest,
				sources.ExpectedFPFRevision,
				sources.FPFProjectionExecutor,
			)
		},
		p14IdentifierNamespaceBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14IdentifierNamespaceScenario(
				declared,
				sources.IdentifierFixture,
			)
		},
		p14InitMatrixBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14InitMatrixScenario(
				declared,
				sources.InitMatrixFixture,
			)
		},
		p14ExistingRecordBackfillBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14ExistingRecordBackfillScenario(
				declared,
				sources.MemoryFixture.Operations,
			)
		},
		p14SpecSectionProtocolBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14SpecSectionProtocolScenario(declared)
		},
		p14OnboardProfileChangeBuilderID: func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14OnboardProfileChangeScenario(declared)
		},
	}
	for _, builderID := range p14CodeExploreBuilderIDs {
		builders[builderID] = func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14CodeExploreScenario(
				sources.Context,
				declared,
				sources.Executable,
				sources.ProjectRoot,
				sources.ExecutableDigest,
				sources.CodeExploreExecutor,
			)
		}
	}
	for _, builderID := range p14MemoryReadBuilderIDs {
		builders[builderID] = func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14MemoryReadScenario(
				sources.Context,
				declared,
				sources.MemoryFixture,
				sources.Executable,
				sources.ProjectRoot,
				sources.ExecutableDigest,
				sources.MemoryReadExecutor,
			)
		}
	}
	for _, builderID := range p14MemoryOperationBuilderIDs {
		builders[builderID] = func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14MemoryOperationScenario(
				declared,
				sources.MemoryFixture.Operations,
			)
		}
	}
	for _, builderID := range p14LiveProtocolBuilderIDs {
		builders[builderID] = func(
			declared scenarioContract,
		) (preparedP14Scenario, error) {
			return buildP14LiveProtocolScenarioWithCandidateVersion(
				declared,
				sources.CandidateVersion,
			)
		}
	}
	return builders
}

func syntheticPreparedP14Bindings(groups []string) []preparedP14Binding {
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
		binding := preparedP14Binding{
			Group: group,
			State: states[group],
		}
		if binding.State == p14BindingCarrierDigest {
			binding.CarrierPath = ".context/p14/fixtures/" + group + ".json"
			binding.CarrierDigest = p14TestDigest("binding:" + group)
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func canonicalCompactJSON(raw []byte) bool {
	if !json.Valid(raw) {
		return false
	}
	buffer := bytes.NewBuffer(make([]byte, 0, len(raw)))
	if err := json.Compact(buffer, raw); err != nil {
		return false
	}
	return bytes.Equal(buffer.Bytes(), raw)
}

func hasBlankOrDuplicate(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return true
		}
		if _, duplicate := seen[value]; duplicate {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func allP14Digests(values []string) bool {
	for _, value := range values {
		if !validP14Digest(value) {
			return false
		}
	}
	return true
}

func validP14Digest(value string) bool {
	raw := strings.TrimPrefix(value, "sha256:")
	if raw == value {
		return false
	}
	decoded, err := hex.DecodeString(raw)
	return err == nil && len(decoded) == sha256.Size
}

func p14Digest(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func p14TestDigest(value string) string {
	return p14Digest([]byte(value))
}
