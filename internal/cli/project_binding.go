package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	envProjectRoot         = "HAFT_PROJECT_ROOT"
	envLegacyProjectRoot   = "QUINT_PROJECT_ROOT"
	envExpectedProjectID   = "HAFT_EXPECTED_PROJECT_ID"
	projectRootSourceCWD   = "cwd"
	projectRootSourceEnv   = envProjectRoot
	projectRootSourceQuint = envLegacyProjectRoot
	projectRootSourceFlag  = "--project-root"
)

var (
	errProjectConfigMissing  = errors.New("project has .haft but no project.yaml")
	errExpectedProjectIDMiss = errors.New("expected project_id mismatch")
)

type projectRootInput struct {
	Path   string
	Source string
}

type ProjectBinding struct {
	RootSource        string
	SearchStart       string
	ProjectRoot       string
	HaftDir           string
	ProjectID         string
	ProjectName       string
	ExpectedProjectID string
	DBPath            string
	DBState           string
	ArtifactCount     int
}

func projectRootSearchStart() (string, error) {
	input, err := projectRootInputFromEnv()
	if err != nil {
		return "", err
	}
	return input.Path, nil
}

func projectRootInputFromEnv() (projectRootInput, error) {
	envRoot := strings.TrimSpace(os.Getenv(envProjectRoot))
	if envRoot != "" {
		return absProjectRootInput(envRoot, projectRootSourceEnv)
	}

	legacyRoot := strings.TrimSpace(os.Getenv(envLegacyProjectRoot))
	if legacyRoot != "" {
		return absProjectRootInput(legacyRoot, projectRootSourceQuint)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return projectRootInput{}, err
	}

	return absProjectRootInput(cwd, projectRootSourceCWD)
}

func projectRootInputFromExplicitOrEnv(projectRoot string) (projectRootInput, error) {
	explicitRoot := strings.TrimSpace(projectRoot)
	if explicitRoot != "" {
		return absProjectRootInput(explicitRoot, projectRootSourceFlag)
	}

	return projectRootInputFromEnv()
}

func absProjectRootInput(path string, source string) (projectRootInput, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return projectRootInput{}, err
	}

	return projectRootInput{
		Path:   absPath,
		Source: source,
	}, nil
}

func resolveProjectBinding() (ProjectBinding, error) {
	input, err := projectRootInputFromEnv()
	if err != nil {
		return ProjectBinding{}, err
	}

	expectedProjectID := strings.TrimSpace(os.Getenv(envExpectedProjectID))
	return resolveProjectBindingFromInput(input, expectedProjectID)
}

func resolveProjectBindingFromInput(input projectRootInput, expectedProjectID string) (ProjectBinding, error) {
	root, err := findProjectRootFrom(input.Path)
	if err != nil {
		return ProjectBinding{
			RootSource:        input.Source,
			SearchStart:       input.Path,
			ExpectedProjectID: strings.TrimSpace(expectedProjectID),
		}, fmt.Errorf("resolve project root from %s=%q: %w", input.Source, input.Path, err)
	}

	haftDir := filepath.Join(root, ".haft")
	binding := ProjectBinding{
		RootSource:        input.Source,
		SearchStart:       input.Path,
		ProjectRoot:       root,
		HaftDir:           haftDir,
		ExpectedProjectID: strings.TrimSpace(expectedProjectID),
	}

	cfg, err := project.Load(haftDir)
	if err != nil {
		return binding, err
	}
	if cfg == nil {
		return binding, errProjectConfigMissing
	}

	binding.ProjectID = cfg.ID
	binding.ProjectName = cfg.Name

	dbPath, err := cfg.DBPath()
	if err != nil {
		return binding, err
	}

	dbState, artifactCount := inspectProjectDB(dbPath)
	binding.DBPath = dbPath
	binding.DBState = dbState
	binding.ArtifactCount = artifactCount

	if binding.ExpectedProjectID != "" && binding.ExpectedProjectID != cfg.ID {
		return binding, fmt.Errorf("%w: expected %s, got %s", errExpectedProjectIDMiss, binding.ExpectedProjectID, cfg.ID)
	}

	return binding, nil
}

func expectedProjectIDForRoot(projectRoot string) string {
	cfg, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.ID
}

func inspectProjectDB(dbPath string) (string, int) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return "missing", 0
		}
		return "unreadable", 0
	}

	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{
		Scheme:   "file",
		Path:     dbPath,
		RawQuery: query.Encode(),
	}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return "unreadable", 0
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM artifacts").Scan(&count); err != nil {
		return "empty_ok_new_project", 0
	}
	if count == 0 {
		return "empty_ok_new_project", 0
	}

	return "ready", count
}

func formatProjectBindingDiagnostic(binding ProjectBinding) string {
	fields := []string{
		"source=" + presentBindingValue(binding.RootSource),
		"search_start=" + presentBindingValue(binding.SearchStart),
		"resolved_root=" + presentBindingValue(binding.ProjectRoot),
		"project_id=" + presentBindingValue(binding.ProjectID),
		"expected_project_id=" + presentBindingValue(binding.ExpectedProjectID),
		"db_path=" + presentBindingValue(binding.DBPath),
		"db_state=" + presentBindingValue(binding.DBState),
	}

	return strings.Join(fields, ", ")
}

func projectBindingError(binding ProjectBinding, err error) error {
	return fmt.Errorf("haft project binding failed: %w; %s", err, formatProjectBindingDiagnostic(binding))
}

func presentBindingValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<unset>"
	}
	return value
}
