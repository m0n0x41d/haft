package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project"
)

// ProjectRegistry tracks known projects for the desktop app.
// Stored in ~/.haft/desktop-projects.json.
type ProjectRegistry struct {
	Projects []RegisteredProject `json:"projects"`
}

type RegisteredProject struct {
	Path string `json:"path"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ProjectInfo is the view model returned to the frontend.
type ProjectInfo struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	ID            string `json:"id"`
	IsActive      bool   `json:"is_active"`
	ProblemCount  int    `json:"problem_count"`
	DecisionCount int    `json:"decision_count"`
	StaleCount    int    `json:"stale_count"`
}

func registryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".haft")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "desktop-projects.json"), nil
}

func loadRegistry() (*ProjectRegistry, error) {
	path, err := registryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectRegistry{}, nil
		}
		return nil, err
	}
	var reg ProjectRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return &ProjectRegistry{}, nil // corrupted → start fresh
	}
	return &reg, nil
}

func saveRegistry(reg *ProjectRegistry) error {
	path, err := registryPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// --- App binding methods for project management ---

// ListProjects returns all registered projects with stats.
func (a *App) ListProjects() ([]ProjectInfo, error) {
	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	if a.projectRoot != "" {
		_, _ = a.addProjectToRegistry(reg, a.projectRoot)
	}

	infos := make([]ProjectInfo, 0, len(reg.Projects))
	for _, rp := range reg.Projects {
		info := ProjectInfo{
			Path:     rp.Path,
			Name:     rp.Name,
			ID:       rp.ID,
			IsActive: rp.Path == a.projectRoot,
		}

		// Quick stats — open DB briefly if not active project
		if rp.Path == a.projectRoot && a.store != nil {
			problems, _ := a.store.ListActiveByKind(a.ctx, "ProblemCard", 1000)
			decisions, _ := a.store.ListActiveByKind(a.ctx, "DecisionRecord", 1000)
			stale, _ := a.store.FindStaleArtifacts(a.ctx)
			info.ProblemCount = len(problems)
			info.DecisionCount = len(decisions)
			info.StaleCount = len(stale)
		}

		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsActive != infos[j].IsActive {
			return infos[i].IsActive // active project first
		}
		return strings.ToLower(infos[i].Name) < strings.ToLower(infos[j].Name)
	})

	return infos, nil
}

// AddProject registers a new project by path.
func (a *App) AddProject(path string) (*ProjectInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Validate path has .haft/
	haftDir := filepath.Join(path, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no .haft/ directory found in %s", path)
	}

	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	rp, err := a.addProjectToRegistry(reg, path)
	if err != nil {
		return nil, err
	}

	return &ProjectInfo{
		Path:     rp.Path,
		Name:     rp.Name,
		ID:       rp.ID,
		IsActive: rp.Path == a.projectRoot,
	}, nil
}

func (a *App) InitProject(path string) (*ProjectInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("open project path %s: %w", absPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	haftDir := filepath.Join(absPath, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", haftDir, err)
	}

	cfg, err := project.Create(haftDir, absPath)
	if err != nil {
		return nil, fmt.Errorf("create project config: %w", err)
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return nil, fmt.Errorf("resolve project database: %w", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("initialize project database: %w", err)
	}
	_ = database.Close()

	return a.AddProject(absPath)
}

// SwitchProject changes the active project — closes current DB, opens new one.
func (a *App) SwitchProject(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	haftDir := filepath.Join(path, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		return fmt.Errorf("no .haft/ directory in %s", path)
	}

	if a.tasks != nil && a.tasks.hasRunningTasks() {
		return fmt.Errorf("cannot switch projects while desktop tasks are still running")
	}

	// Close current DB
	if a.dbConn != nil {
		a.dbConn.Close()
		a.dbConn = nil
		a.store = nil
	}

	// Re-init with new project
	a.projectRoot = path
	a.startup(a.ctx)

	return nil
}

// ScanForProjects searches common directories for .haft/ projects.
func (a *App) ScanForProjects() ([]ProjectInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	searchDirs := []string{
		filepath.Join(home, "Repos"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "projects"),
		filepath.Join(home, "src"),
		filepath.Join(home, "code"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "Developer"),
		filepath.Join(home, "dev"),
	}

	found := make(map[string]bool)
	var results []ProjectInfo

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		// Walk 2 levels deep looking for .haft/
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			checkPath := filepath.Join(dir, e.Name())
			scanProjectDir(checkPath, found, &results, 2)
		}
	}

	return results, nil
}

func scanProjectDir(dir string, found map[string]bool, results *[]ProjectInfo, depth int) {
	if depth <= 0 {
		return
	}

	haftDir := filepath.Join(dir, ".haft")
	if _, err := os.Stat(haftDir); err == nil && !found[dir] {
		found[dir] = true
		cfg, _ := project.Load(haftDir)
		name := filepath.Base(dir)
		id := ""
		if cfg != nil {
			name = cfg.Name
			id = cfg.ID
		}
		*results = append(*results, ProjectInfo{
			Path: dir,
			Name: name,
			ID:   id,
		})
		return // don't descend into .haft projects
	}

	// Descend one more level
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		scanProjectDir(filepath.Join(dir, e.Name()), found, results, depth-1)
	}
}

func (a *App) addProjectToRegistry(reg *ProjectRegistry, path string) (*RegisteredProject, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check if already registered
	for _, rp := range reg.Projects {
		if rp.Path == absPath {
			return &rp, nil
		}
	}

	// Load project config
	haftDir := filepath.Join(absPath, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(absPath)
	id := ""
	if cfg != nil {
		name = cfg.Name
		id = cfg.ID
	}

	rp := RegisteredProject{
		Path: absPath,
		Name: name,
		ID:   id,
	}
	reg.Projects = append(reg.Projects, rp)

	if err := saveRegistry(reg); err != nil {
		return nil, err
	}

	return &rp, nil
}
