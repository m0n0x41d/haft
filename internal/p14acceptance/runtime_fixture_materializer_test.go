package p14acceptance

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	_ "modernc.org/sqlite"
)

const (
	p14RuntimeFixtureMaterializationEnvironmentKey = "HAFT_P14_MATERIALIZE_RUNTIME_FIXTURES"
	p14RuntimeFixtureMaterializationSchema         = "haft.p14.runtime-fixture-materialization-request/v1"
)

type p14RuntimeFixtureMaterializationRequest struct {
	Schema                  string `json:"schema"`
	P13EvidencePath         string `json:"p13_evidence_path"`
	CandidateExecutablePath string `json:"candidate_executable_path"`
}

func TestP14MaterializeRuntimeFixtureCarriers(t *testing.T) {
	requestPath := os.Getenv(p14RuntimeFixtureMaterializationEnvironmentKey)
	if requestPath == "" {
		t.Skip("set HAFT_P14_MATERIALIZE_RUNTIME_FIXTURES after final P13")
	}
	repositoryRoot, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	request, err := loadP14RuntimeFixtureMaterializationRequest(
		repositoryRoot,
		requestPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	p13Binding, p13Evidence, err := loadP14PassingP13Evidence(
		repositoryRoot,
		request.P13EvidencePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyP13EvidenceFreshViaHarness(
		repositoryRoot,
		p13Binding,
	); err != nil {
		t.Fatal(err)
	}
	candidateDigest, err := digestP14File(request.CandidateExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	memoryFixturePath, initFixturePath, err :=
		p14RuntimeFixtureCarrierPaths(candidateDigest)
	if err != nil {
		t.Fatal(err)
	}
	originalHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	memoryFixture, err := materializeP14GoldenMemoryFixture(
		repositoryRoot,
		originalHome,
		candidateDigest,
		p13Evidence.StartIdentity,
	)
	if err != nil {
		t.Fatal(err)
	}
	initFixture, err := materializeP14InitMatrixFixture(
		t,
		repositoryRoot,
		candidateDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	memoryRaw, err := json.MarshalIndent(memoryFixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	memoryRaw = append(memoryRaw, '\n')
	initRaw, err := marshalP14CanonicalJSON(initFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishP14NoClobber(
		repositoryRoot,
		memoryFixturePath,
		memoryRaw,
	); err != nil {
		t.Fatal(err)
	}
	if err := publishP14NoClobber(
		repositoryRoot,
		initFixturePath,
		initRaw,
	); err != nil {
		t.Fatal(err)
	}
	t.Logf(
		"P14_RUNTIME_FIXTURES memory=%s:%s init=%s:%s",
		memoryFixturePath,
		p14Digest(memoryRaw),
		initFixturePath,
		p14Digest(initRaw),
	)
}

func p14RuntimeFixtureCarrierPaths(
	candidateDigest string,
) (string, string, error) {
	if !validP14Digest(candidateDigest) {
		return "", "", fmt.Errorf(
			"P14 runtime fixture candidate digest is invalid",
		)
	}
	body := strings.TrimPrefix(candidateDigest, "sha256:")
	memory := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"fixtures",
		"golden_memory_fixture-"+body[:16]+".json",
	))
	init := filepath.ToSlash(filepath.Join(
		".context",
		"p14",
		"fixtures",
		"init_matrix_v4-"+body[:16]+".json",
	))
	return memory, init, nil
}

func TestP14RuntimeFixtureCarrierPathsAreCandidateScoped(
	t *testing.T,
) {
	firstMemory, firstInit, err := p14RuntimeFixtureCarrierPaths(
		p14TestDigest("first-candidate"),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondMemory, secondInit, err := p14RuntimeFixtureCarrierPaths(
		p14TestDigest("second-candidate"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if firstMemory == secondMemory ||
		firstInit == secondInit ||
		firstMemory == ".context/p14/fixtures/golden_memory_fixture.json" ||
		firstInit == ".context/p14/fixtures/init_matrix.json" {
		t.Fatalf(
			"P14 runtime fixture paths are not candidate-scoped: %q %q %q %q",
			firstMemory,
			firstInit,
			secondMemory,
			secondInit,
		)
	}
	if _, _, err := p14RuntimeFixtureCarrierPaths("not-a-digest"); err == nil {
		t.Fatal("P14 runtime fixture paths accepted an invalid candidate")
	}
}

func loadP14RuntimeFixtureMaterializationRequest(
	repositoryRoot string,
	requestPath string,
) (p14RuntimeFixtureMaterializationRequest, error) {
	path := requestPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(repositoryRoot, filepath.FromSlash(path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p14RuntimeFixtureMaterializationRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	request := p14RuntimeFixtureMaterializationRequest{}
	if err := decoder.Decode(&request); err != nil {
		return p14RuntimeFixtureMaterializationRequest{}, err
	}
	trailing := json.RawMessage{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return p14RuntimeFixtureMaterializationRequest{}, fmt.Errorf(
			"P14 runtime-fixture materialization request has trailing JSON",
		)
	}
	canonical, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return p14RuntimeFixtureMaterializationRequest{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(raw, canonical) {
		return p14RuntimeFixtureMaterializationRequest{}, fmt.Errorf(
			"P14 runtime-fixture materialization request is not canonical JSON",
		)
	}
	if request.Schema != p14RuntimeFixtureMaterializationSchema ||
		strings.TrimSpace(request.P13EvidencePath) == "" ||
		!filepath.IsAbs(request.CandidateExecutablePath) {
		return p14RuntimeFixtureMaterializationRequest{}, fmt.Errorf(
			"P14 runtime-fixture materialization request is invalid",
		)
	}
	return request, nil
}

func materializeP14GoldenMemoryFixture(
	repositoryRoot string,
	originalHome string,
	candidateDigest string,
	identity p13IdentityEnvelope,
) (p14MemoryReadFixture, error) {
	canonicalProjectRoot, err := filepath.EvalSymlinks(identity.ProjectRoot)
	if err != nil {
		return p14MemoryReadFixture{}, err
	}
	if canonicalProjectRoot != repositoryRoot {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"P14 memory fixture project differs from P13",
		)
	}
	fixture := syntheticP14MemoryReadFixture()
	fixture.Basis.TypeEnvDigest = strings.TrimPrefix(
		identity.TypeEnv.SelectedCompositeRef,
		"typeenv:",
	)
	fixture.Basis.GraphRevision = identity.Graph.Revision
	if fixture.Basis.GraphRevision <= 1 {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"P14 memory fixture needs a prior stale graph revision",
		)
	}
	fixture.StaleBasis.TypeEnvDigest = p14Digest(
		[]byte("p14-stale-basis:" + fixture.Basis.TypeEnvDigest),
	)
	fixture.StaleBasis.GraphRevision = fixture.Basis.GraphRevision - 1
	fixture.LegacyEntityRef = fixture.EntityRef
	fixture.Operations = syntheticP14MemoryOperationFixture(fixture)
	projectBasisDigest, err := observeP14SelectedProjectMemoryBasis(
		canonicalProjectRoot,
	)
	if err != nil {
		return p14MemoryReadFixture{}, err
	}
	fixture.Operations.SelectedProjectRoot = canonicalProjectRoot
	fixture.Operations.SelectedProjectBasisDigest = projectBasisDigest
	templateRoot := p14RuntimeFixtureTemplateRoot(
		repositoryRoot,
		candidateDigest,
	)
	homeTemplateRoot := filepath.Join(templateRoot, "memory-home")
	sourceDatabasePath := filepath.Join(
		originalHome,
		".haft",
		"projects",
		identity.ProjectID,
		"haft.db",
	)
	destinationDatabasePath := filepath.Join(
		homeTemplateRoot,
		".haft",
		"projects",
		identity.ProjectID,
		"haft.db",
	)
	if p14InstalledCLIPathExists(homeTemplateRoot) {
		return p14MemoryReadFixture{}, fmt.Errorf(
			"P14 memory home template already exists",
		)
	}
	if err := os.MkdirAll(
		filepath.Dir(destinationDatabasePath),
		0o700,
	); err != nil {
		return p14MemoryReadFixture{}, err
	}
	if err := snapshotP14SQLiteDatabase(
		sourceDatabasePath,
		destinationDatabasePath,
	); err != nil {
		return p14MemoryReadFixture{}, err
	}
	homeTemplateDigest, err := observeP14InitTree(homeTemplateRoot)
	if err != nil {
		return p14MemoryReadFixture{}, err
	}
	fixture.Operations.HomeTemplateRoot = homeTemplateRoot
	fixture.Operations.HomeTemplateDigest = homeTemplateDigest
	if err := validateP14MemoryReadFixtureShape(fixture); err != nil {
		return p14MemoryReadFixture{}, err
	}
	return fixture, nil
}

func materializeP14InitMatrixFixture(
	t *testing.T,
	repositoryRoot string,
	candidateDigest string,
) (p14InitMatrixFixture, error) {
	t.Helper()
	fixture := syntheticP14InitMatrixFixture()
	workspaceRoot, err := prepareP14InstalledCLIWorkspace(candidateDigest)
	if err != nil {
		return p14InitMatrixFixture{}, err
	}
	defer os.RemoveAll(workspaceRoot)
	templateRoot := p14RuntimeFixtureTemplateRoot(
		repositoryRoot,
		candidateDigest,
	)
	for index, policy := range p14InitMatrixPolicies() {
		projectExecutionRoot, homeExecutionRoot :=
			p14InstalledCLIInitExecutionRoots(
				workspaceRoot,
				p14InitMatrixScenarioID,
				policy.ID,
			)
		caseTemplateRoot := filepath.Join(templateRoot, "init", policy.ID)
		projectTemplateRoot := filepath.Join(caseTemplateRoot, "project")
		homeTemplateRoot := filepath.Join(caseTemplateRoot, "home")
		if p14InstalledCLIPathExists(caseTemplateRoot) {
			return p14InitMatrixFixture{}, fmt.Errorf(
				"P14 init template %q already exists",
				policy.ID,
			)
		}
		if err := materializeP14InitTemplateCase(
			t,
			policy,
			projectExecutionRoot,
			projectTemplateRoot,
			homeTemplateRoot,
		); err != nil {
			return p14InitMatrixFixture{}, err
		}
		projectDigest, err := observeP14InitTree(projectTemplateRoot)
		if err != nil {
			return p14InitMatrixFixture{}, err
		}
		homeDigest, err := observeP14InitTree(homeTemplateRoot)
		if err != nil {
			return p14InitMatrixFixture{}, err
		}
		fixture.Cases[index].ProjectTemplateRoot = projectTemplateRoot
		fixture.Cases[index].ProjectTemplateDigest = projectDigest
		fixture.Cases[index].HomeTemplateRoot = homeTemplateRoot
		fixture.Cases[index].HomeTemplateDigest = homeDigest
		fixture.Cases[index].ProjectExecutionRoot = projectExecutionRoot
		fixture.Cases[index].HomeExecutionRoot = homeExecutionRoot
		if err := os.RemoveAll(projectExecutionRoot); err != nil {
			return p14InitMatrixFixture{}, err
		}
	}
	if err := validateP14InitMatrixFixtureShape(fixture); err != nil {
		return p14InitMatrixFixture{}, err
	}
	return fixture, nil
}

func materializeP14InitTemplateCase(
	t *testing.T,
	policy p14InitMatrixCasePolicy,
	projectExecutionRoot string,
	projectTemplateRoot string,
	homeTemplateRoot string,
) error {
	t.Helper()
	if policy.TemplateKind == "profile_unavailable" {
		if err := os.MkdirAll(projectTemplateRoot, 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(homeTemplateRoot, 0o700); err != nil {
			return err
		}
		return materializeP14InitLegacyCommandFixtures(
			projectTemplateRoot,
			homeTemplateRoot,
		)
	}
	harness := profileadmissionfixture.New(t, projectExecutionRoot)
	switch policy.TemplateKind {
	case "single_software_scope":
		harness.AdmitSoftwareRevision(t, "p14-init")
	case "single_non_software_scope":
		harness.AdmitNonSoftwareRevision(t, "p14-init")
	case "mixed_scopes":
		harness.AdmitMixedRevision(t, "p14-init")
	default:
		return fmt.Errorf(
			"P14 init template kind %q is unsupported",
			policy.TemplateKind,
		)
	}
	databasePath := harness.DatabasePath()
	if err := harness.Close(); err != nil {
		return err
	}
	projectDatabaseRoot := filepath.Dir(databasePath)
	projectsRoot := filepath.Dir(projectDatabaseRoot)
	haftHomeRoot := filepath.Dir(projectsRoot)
	sourceHomeRoot := filepath.Dir(haftHomeRoot)
	if err := copyP14InstalledCLITree(
		projectExecutionRoot,
		projectTemplateRoot,
	); err != nil {
		return err
	}
	if err := copyP14InstalledCLITree(
		sourceHomeRoot,
		homeTemplateRoot,
	); err != nil {
		return err
	}
	return materializeP14InitLegacyCommandFixtures(
		projectTemplateRoot,
		homeTemplateRoot,
	)
}

func materializeP14InitLegacyCommandFixtures(
	projectRoot string,
	homeRoot string,
) error {
	for _, fixture := range p14InitLegacyCommandFixtures(
		projectRoot,
		homeRoot,
	) {
		if err := os.MkdirAll(filepath.Dir(fixture.Path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(
			fixture.Path,
			[]byte(fixture.Content),
			0o644,
		); err != nil {
			return err
		}
	}
	return nil
}

func p14RuntimeFixtureTemplateRoot(
	repositoryRoot string,
	candidateDigest string,
) string {
	digestBody := strings.TrimPrefix(candidateDigest, "sha256:")
	return filepath.Join(
		repositoryRoot,
		".context",
		"p14",
		"templates",
		digestBody[:16],
	)
}

func snapshotP14SQLiteDatabase(
	sourcePath string,
	destinationPath string,
) error {
	if !p14InstalledCLIRegularFile(sourcePath) {
		return fmt.Errorf("P14 source database is absent")
	}
	if p14InstalledCLIPathExists(destinationPath) {
		return fmt.Errorf("P14 destination database already exists")
	}
	sourceURL := url.URL{
		Scheme: "file",
		Path:   sourcePath,
	}
	query := sourceURL.Query()
	query.Set("mode", "ro")
	sourceURL.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", sourceURL.String())
	if err != nil {
		return err
	}
	quotedDestination := strings.ReplaceAll(destinationPath, "'", "''")
	statement := "VACUUM INTO '" + quotedDestination + "'"
	_, snapshotErr := database.ExecContext(context.Background(), statement)
	closeErr := database.Close()
	if snapshotErr != nil {
		return fmt.Errorf("snapshot P14 SQLite database: %w", snapshotErr)
	}
	if closeErr != nil {
		return closeErr
	}
	check, err := sql.Open("sqlite", sourceURLForP14ReadOnly(destinationPath))
	if err != nil {
		return err
	}
	row := check.QueryRowContext(context.Background(), "PRAGMA integrity_check")
	result := ""
	if err := row.Scan(&result); err != nil {
		_ = check.Close()
		return err
	}
	if result != "ok" {
		_ = check.Close()
		return fmt.Errorf("P14 SQLite snapshot integrity check = %q", result)
	}
	return check.Close()
}

func sourceURLForP14ReadOnly(path string) string {
	sourceURL := url.URL{
		Scheme: "file",
		Path:   path,
	}
	query := sourceURL.Query()
	query.Set("mode", "ro")
	sourceURL.RawQuery = query.Encode()
	return sourceURL.String()
}
