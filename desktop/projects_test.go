package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestInitProjectCreatesProjectConfigAndRegistryEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "alpha")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	app := NewApp()
	info, err := app.InitProject(projectRoot)
	if err != nil {
		t.Fatalf("InitProject: %v", err)
	}

	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	if info.Path != absPath {
		t.Fatalf("expected path %q, got %q", absPath, info.Path)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".haft", "project.yaml")); err != nil {
		t.Fatalf("expected project config to exist: %v", err)
	}

	cfg, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected project database to exist: %v", err)
	}

	registry, err := loadRegistry()
	if err != nil {
		t.Fatalf("loadRegistry: %v", err)
	}

	if len(registry.Projects) != 1 {
		t.Fatalf("expected 1 registered project, got %d", len(registry.Projects))
	}
}

func TestStartupUsesExplicitProjectRootInsteadOfCurrentWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	firstProject := filepath.Join(workspace, "first")
	secondProject := filepath.Join(workspace, "second")

	if err := os.MkdirAll(firstProject, 0o755); err != nil {
		t.Fatalf("MkdirAll first: %v", err)
	}

	if err := os.MkdirAll(secondProject, 0o755); err != nil {
		t.Fatalf("MkdirAll second: %v", err)
	}

	setupApp := NewApp()
	if _, err := setupApp.InitProject(firstProject); err != nil {
		t.Fatalf("InitProject first: %v", err)
	}
	if _, err := setupApp.InitProject(secondProject); err != nil {
		t.Fatalf("InitProject second: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(currentDir)
	}()

	if err := os.Chdir(firstProject); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	app := NewApp()
	app.projectRoot = secondProject
	app.startup(context.Background())
	defer app.shutdown(context.Background())

	expectedRoot, err := filepath.Abs(secondProject)
	if err != nil {
		t.Fatalf("Abs second: %v", err)
	}

	if app.projectRoot != expectedRoot {
		t.Fatalf("expected project root %q, got %q", expectedRoot, app.projectRoot)
	}
}

func TestAtomicWriteFile_ConcurrentWritersLeaveValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop-projects.json")
	payloads := make([][]byte, 0, 12)

	for index := range 12 {
		payloads = append(payloads, []byte(
			strings.ReplaceAll(
				`{"projects":[{"path":"/tmp/project-INDEX","name":"project","id":"proj-INDEX"}]}`,
				"INDEX",
				string(rune('0'+index)),
			),
		))
	}

	var wg sync.WaitGroup
	for _, payload := range payloads {
		wg.Add(1)
		go func(payload []byte) {
			defer wg.Done()
			if err := atomicWriteFile(path, payload, 0o644); err != nil {
				t.Errorf("atomicWriteFile: %v", err)
			}
		}(payload)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var registry ProjectRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("written file must stay valid JSON, got %q: %v", string(data), err)
	}

	matchesKnownPayload := false
	for _, payload := range payloads {
		if string(data) == string(payload) {
			matchesKnownPayload = true
			break
		}
	}
	if !matchesKnownPayload {
		t.Fatalf("final file should match one complete payload, got %q", string(data))
	}
}
