package main

import (
	"context"
	"os"
	"path/filepath"
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
