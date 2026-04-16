package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
	return atomicWriteFile(path, data, 0o644)
}

var atomicWriteMu sync.Mutex

// atomicWriteFile serializes concurrent writers, writes to a unique temp file,
// then renames atomically. This avoids torn JSON writes under concurrent saves.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	atomicWriteMu.Lock()
	defer atomicWriteMu.Unlock()

	release, err := acquireAtomicWriteLock(path + ".lock")
	if err != nil {
		return err
	}
	defer release()

	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + "."
	tmpFile, err := os.CreateTemp(dir, prefix+"*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func acquireAtomicWriteLock(lockPath string) (func(), error) {
	deadline := time.Now().Add(2 * time.Second)

	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		switch {
		case err == nil:
			_ = file.Close()
			return func() {
				_ = os.Remove(lockPath)
			}, nil
		case errors.Is(err, os.ErrExist):
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("acquire write lock %s: timeout", lockPath)
			}
			time.Sleep(10 * time.Millisecond)
		default:
			return nil, err
		}
	}
}

// --- App binding methods for project management ---

// ListProjects returns all registered projects with stats.
func (a *App) ListProjects() ([]ProjectInfo, error) {
	dlog.Debug().Str("active_root", a.projectRoot).Msg("ListProjects called")

	reg, err := loadRegistry()
	if err != nil {
		dlog.Error().Err(err).Msg("ListProjects: loadRegistry failed")
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

	// Stable alphabetical sort — do NOT put active project first,
	// that causes visual jumping when switching projects.
	sort.Slice(infos, func(i, j int) bool {
		return strings.ToLower(infos[i].Name) < strings.ToLower(infos[j].Name)
	})

	names := make([]string, len(infos))
	activeName := ""
	for i, info := range infos {
		names[i] = info.Name
		if info.IsActive {
			activeName = info.Name
		}
	}
	dlog.Debug().
		Strs("projects", names).
		Str("active", activeName).
		Int("count", len(infos)).
		Msg("ListProjects result")

	return infos, nil
}

func (a *App) ListAllTasks() ([]TaskState, error) {
	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	if a.projectRoot != "" {
		_, _ = a.addProjectToRegistry(reg, a.projectRoot)
	}

	allTasks := make([]TaskState, 0)
	for _, rp := range reg.Projects {
		tasks, err := a.listTasksForProject(context.Background(), rp.Path)
		if err != nil {
			a.emitAppError("list all tasks", err)
			continue
		}

		allTasks = append(allTasks, tasks...)
	}

	sort.Slice(allTasks, func(i int, j int) bool {
		return allTasks[i].StartedAt > allTasks[j].StartedAt
	})

	return allTasks, nil
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

// AddProjectSmart adds a project directory — auto-initializes .haft/ if missing,
// registers in the project registry, and switches to it.
func (a *App) AddProjectSmart(path string) (*ProjectInfo, error) {
	dlog.Info().Str("path", path).Msg("AddProjectSmart called")

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
		return nil, fmt.Errorf("cannot access %s: %w", absPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", absPath)
	}

	// Auto-init if no .haft/ exists
	haftDir := filepath.Join(absPath, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		if _, initErr := a.InitProject(absPath); initErr != nil {
			return nil, fmt.Errorf("init project: %w", initErr)
		}
	}

	// Register
	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	rp, err := a.addProjectToRegistry(reg, absPath)
	if err != nil {
		return nil, err
	}

	// Switch to the new project
	if err := a.SwitchProject(absPath); err != nil {
		return nil, err
	}

	return &ProjectInfo{
		Path:     rp.Path,
		Name:     rp.Name,
		ID:       rp.ID,
		IsActive: true,
	}, nil
}

// SwitchProject changes the active project — closes current DB, opens new one.
// Validates the new project's DB is accessible BEFORE tearing down the old one.
func (a *App) SwitchProject(path string) error {
	dlog.Info().Str("path", path).Str("current_root", a.projectRoot).Msg("SwitchProject called")

	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	haftDir := filepath.Join(path, ".haft")
	if _, err := os.Stat(haftDir); os.IsNotExist(err) {
		return fmt.Errorf("no .haft/ directory in %s", path)
	}

	// Pre-validate: check that the new project's config and DB are accessible
	// BEFORE tearing down the current project. This prevents zombie state.
	newCfg, err := project.Load(haftDir)
	if err != nil || newCfg == nil {
		return fmt.Errorf("cannot load project config from %s: %w", haftDir, err)
	}
	newDBPath, err := newCfg.DBPath()
	if err != nil {
		return fmt.Errorf("cannot resolve DB path for %s: %w", path, err)
	}
	testDB, err := db.NewStore(newDBPath)
	if err != nil {
		return fmt.Errorf("cannot open DB for %s: %w", path, err)
	}
	testDB.Close() // validated — close the test connection

	if a.tasks != nil && a.tasks.hasRunningTasks() {
		return fmt.Errorf("cannot switch projects while desktop tasks are still running")
	}

	// Now safe to tear down — we know the new project is accessible.
	if a.governance != nil {
		a.governance.shutdown()
		a.governance = nil
	}

	if a.flows != nil {
		a.flows.shutdown()
		a.flows = nil
	}

	if a.terminals != nil {
		a.terminals.shutdown()
		a.terminals = nil
	}

	a.tasks = nil

	if a.dbConn != nil {
		a.dbConn.Close()
		a.dbConn = nil
		a.store = nil
	}

	// Re-init with validated project
	a.projectRoot = ""
	a.projectName = ""
	a.loadProject(path)

	// Verify loadProject succeeded
	if a.store == nil {
		return fmt.Errorf("failed to initialize project %s — store is nil after load", path)
	}

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

	// Check if already registered — update name to directory basename
	for i, rp := range reg.Projects {
		if rp.Path == absPath {
			if rp.Name != filepath.Base(absPath) {
				reg.Projects[i].Name = filepath.Base(absPath)
				_ = saveRegistry(reg)
			}
			return &reg.Projects[i], nil
		}
	}

	// Load project config
	haftDir := filepath.Join(absPath, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, err
	}

	// Always use directory name for display — it's what users recognize
	name := filepath.Base(absPath)
	id := ""
	if cfg != nil {
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

func (a *App) listTasksForProject(ctx context.Context, projectPath string) ([]TaskState, error) {
	if projectPath == "" {
		return []TaskState{}, nil
	}

	if a != nil && a.projectRoot == projectPath && a.tasks != nil {
		return a.tasks.list(ctx, projectPath)
	}

	haftDir := filepath.Join(projectPath, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, err
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return nil, err
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return nil, err
	}
	defer database.Close()

	store := newDesktopTaskStore(database.GetRawDB())
	return store.ListTasks(ctx, projectPath)
}
