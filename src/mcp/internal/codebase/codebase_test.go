package codebase

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	stmts := []string{
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, lang TEXT, file_count INTEGER DEFAULT 0,
			last_scanned TEXT NOT NULL)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL, target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT,
			last_scanned TEXT NOT NULL,
			PRIMARY KEY (source_module, target_module, dep_type))`,
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER DEFAULT 1,
			status TEXT DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup: %v\nSQL: %s", err, s)
		}
	}
	return db
}

// --- Go Detection Tests ---

func TestGoDetectModules(t *testing.T) {
	root := t.TempDir()

	// Create a Go project structure
	writeFile(t, root, "go.mod", "module example.com/myapp\n\ngo 1.21\n")
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "internal/auth/auth.go", "package auth\n")
	writeFile(t, root, "internal/auth/middleware.go", "package auth\n")
	writeFile(t, root, "internal/db/store.go", "package db\n")
	writeFile(t, root, "pkg/utils/helpers.go", "package utils\n")

	detector := &GoLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 4 {
		t.Fatalf("expected ≥4 modules (root, auth, db, utils), got %d: %v", len(modules), moduleNames(modules))
	}

	// Check auth module has 2 files
	for _, m := range modules {
		if m.Name == "auth" {
			if m.FileCount != 2 {
				t.Errorf("auth module: expected 2 files, got %d", m.FileCount)
			}
			if m.Lang != "go" {
				t.Errorf("auth module: expected lang=go, got %s", m.Lang)
			}
		}
	}
}

func TestGoParseImports(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "go.mod", "module example.com/myapp\n\ngo 1.21\n")
	writeFile(t, root, "cmd/server/main.go", `package main

import (
	"fmt"
	"net/http"

	"example.com/myapp/internal/auth"
	"example.com/myapp/internal/db"
	"github.com/external/pkg"
)

func main() {
	fmt.Println(auth.Check(), db.Connect(), http.StatusOK, pkg.Do())
}
`)

	parser := &GoLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "cmd/server/main.go"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find 2 local imports (auth, db), skip stdlib and external
	if len(edges) != 2 {
		t.Fatalf("expected 2 local imports, got %d: %v", len(edges), edges)
	}

	targets := make(map[string]bool)
	for _, e := range edges {
		targets[e.TargetModule] = true
	}
	if !targets["mod-internal-auth"] {
		t.Error("missing import edge to mod-internal-auth")
	}
	if !targets["mod-internal-db"] {
		t.Error("missing import edge to mod-internal-db")
	}
}

func TestGoNotGoProject(t *testing.T) {
	root := t.TempDir()
	// No go.mod — should return nil
	detector := &GoLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil for non-Go project, got %v", modules)
	}
}

// --- JS/TS Detection Tests ---

func TestJSTSDetectModules(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "package.json", `{"name": "my-app", "workspaces": ["packages/*"]}`)
	writeFile(t, root, "src/index.ts", "export const x = 1;\n")
	writeFile(t, root, "packages/core/package.json", `{"name": "@my-app/core"}`)
	writeFile(t, root, "packages/core/src/index.ts", "export const core = 1;\n")
	writeFile(t, root, "packages/utils/package.json", `{"name": "@my-app/utils"}`)
	writeFile(t, root, "packages/utils/src/helper.ts", "export const h = 1;\n")

	detector := &JSTSLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 3 {
		t.Fatalf("expected ≥3 modules (root, core, utils), got %d: %v", len(modules), moduleNames(modules))
	}

	// Check workspace package names
	for _, m := range modules {
		if m.Path == "packages/core" && m.Name != "@my-app/core" {
			t.Errorf("core module: expected name=@my-app/core, got %s", m.Name)
		}
	}
}

func TestJSTSParseImports(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "package.json", `{"name": "app"}`)
	writeFile(t, root, "src/app.ts", `
import { Router } from 'express';
import { auth } from './auth';
import config from '../config';
const db = require('./db');
import './styles.css';
`)

	parser := &JSTSLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "src/app.ts"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find local imports: ./auth, ../config, ./db
	// Should skip: 'express' (external), './styles.css' (relative but from import '' form)
	if len(edges) < 2 {
		t.Fatalf("expected ≥2 local imports, got %d: %v", len(edges), edges)
	}
}

// --- Python Detection Tests ---

func TestPythonDetectModules(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "pyproject.toml", "[project]\nname = \"myapp\"\n")
	writeFile(t, root, "myapp/__init__.py", "")
	writeFile(t, root, "myapp/core.py", "# core\n")
	writeFile(t, root, "myapp/utils/__init__.py", "")
	writeFile(t, root, "myapp/utils/helpers.py", "# helpers\n")
	writeFile(t, root, "tests/__init__.py", "")
	writeFile(t, root, "tests/test_core.py", "# test\n")

	detector := &PythonLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 3 {
		t.Fatalf("expected ≥3 modules (myapp, utils, tests), got %d: %v", len(modules), moduleNames(modules))
	}
}

func TestPythonParseImports(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "pyproject.toml", "[project]\nname = \"myapp\"\n")
	writeFile(t, root, "myapp/__init__.py", "")
	writeFile(t, root, "myapp/app.py", `
import os
import sys
from myapp import core
from myapp.utils import helpers
from . import config
from ..tests import test_core
import json
`)

	parser := &PythonLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "myapp/app.py"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find: myapp, myapp.utils (absolute), config (relative), tests (relative)
	if len(edges) < 3 {
		t.Fatalf("expected ≥3 imports, got %d: %v", len(edges), edges)
	}
}

// --- Rust Detection Tests ---

func TestRustDetectModules(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "Cargo.toml", "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n")
	writeFile(t, root, "src/main.rs", "fn main() {}\n")
	writeFile(t, root, "src/lib.rs", "pub mod auth;\npub mod db;\n")
	writeFile(t, root, "src/auth/mod.rs", "pub fn check() {}\n")
	writeFile(t, root, "src/db/mod.rs", "pub fn connect() {}\n")

	detector := &RustLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 2 {
		t.Fatalf("expected ≥2 modules, got %d: %v", len(modules), moduleNames(modules))
	}
}

func TestRustParseImports(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "Cargo.toml", "[package]\nname = \"myapp\"\n")
	writeFile(t, root, "src/main.rs", `
use std::io;
use crate::auth;
use crate::db;
mod config;

fn main() {}
`)

	parser := &RustLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "src/main.rs"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find: auth, db (use crate::), config (mod declaration)
	// Should skip: std::io
	if len(edges) < 3 {
		t.Fatalf("expected ≥3 edges (auth, db, config), got %d: %v", len(edges), edges)
	}
}

// --- C/C++ Detection Tests ---

func TestCCppDetectModulesWithCompileCommands(t *testing.T) {
	root := t.TempDir()

	// Create a C project with compile_commands.json
	writeFile(t, root, "Makefile", "all: myapp\n")
	writeFile(t, root, "src/main.c", "#include <stdio.h>\nint main() { return 0; }\n")
	writeFile(t, root, "src/utils.c", "void helper() {}\n")
	writeFile(t, root, "src/net/socket.c", "void connect() {}\n")
	writeFile(t, root, "src/net/dns.c", "void resolve() {}\n")
	writeFile(t, root, "include/utils.h", "void helper();\n")

	ccj := `[
		{"directory": "` + root + `", "command": "gcc -Iinclude -c src/main.c -o main.o", "file": "src/main.c"},
		{"directory": "` + root + `", "command": "gcc -Iinclude -c src/utils.c -o utils.o", "file": "src/utils.c"},
		{"directory": "` + root + `", "command": "gcc -Iinclude -c src/net/socket.c -o net/socket.o", "file": "src/net/socket.c"},
		{"directory": "` + root + `", "command": "gcc -Iinclude -c src/net/dns.c -o net/dns.o", "file": "src/net/dns.c"}
	]`
	writeFile(t, root, "compile_commands.json", ccj)

	detector := &CCppLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 2 {
		t.Fatalf("expected >=2 modules (src, src/net), got %d: %v", len(modules), moduleNames(modules))
	}

	// Check src/net module has 2 files
	for _, m := range modules {
		if m.Path == "src/net" {
			if m.FileCount != 2 {
				t.Errorf("src/net module: expected 2 files, got %d", m.FileCount)
			}
			if m.Lang != "c_cpp" {
				t.Errorf("src/net module: expected lang=c_cpp, got %s", m.Lang)
			}
		}
	}
}

func TestCCppDetectModulesFallback(t *testing.T) {
	root := t.TempDir()

	// No compile_commands.json, just a Makefile and source files
	writeFile(t, root, "Makefile", "all: myapp\n")
	writeFile(t, root, "main.c", "int main() { return 0; }\n")
	writeFile(t, root, "lib/math.c", "int add(int a, int b) { return a+b; }\n")
	writeFile(t, root, "lib/math.h", "int add(int, int);\n")

	detector := &CCppLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) < 2 {
		t.Fatalf("expected >=2 modules (root, lib), got %d: %v", len(modules), moduleNames(modules))
	}
}

func TestCCppCompileCommandsBadPathsFallback(t *testing.T) {
	root := t.TempDir()

	// compile_commands.json exists but all file paths point outside the project
	writeFile(t, root, "Makefile", "all: myapp\n")
	writeFile(t, root, "src/main.c", "int main() { return 0; }\n")
	ccj := `[
		{"directory": "/nonexistent/build", "command": "gcc -c /nonexistent/src/main.c", "file": "/nonexistent/src/main.c"}
	]`
	writeFile(t, root, "compile_commands.json", ccj)

	detector := &CCppLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	// Should fall back to directory scan and find src/main.c
	if len(modules) == 0 {
		t.Fatal("expected fallback to directory scan when compile_commands.json paths don't resolve")
	}

	foundSrc := false
	for _, m := range modules {
		if m.Path == "src" {
			foundSrc = true
		}
	}
	if !foundSrc {
		t.Errorf("expected src module from fallback, got: %v", moduleNames(modules))
	}
}

func TestCCppNotCProject(t *testing.T) {
	root := t.TempDir()
	// No Makefile, no CMakeLists.txt, no compile_commands.json
	detector := &CCppLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil for non-C/C++ project, got %v", modules)
	}
}

func TestCCppParseImports(t *testing.T) {
	root := t.TempDir()

	writeFile(t, root, "Makefile", "all: myapp\n")
	writeFile(t, root, "include/utils.h", "void helper();\n")
	writeFile(t, root, "include/net/socket.h", "void connect();\n")
	writeFile(t, root, "src/main.c", `
#include <stdio.h>
#include <stdlib.h>
#include "utils.h"
#include "net/socket.h"
`)

	// compile_commands.json with -I flag pointing to include/
	ccj := `[
		{"directory": "` + root + `", "command": "gcc -I` + root + `/include -c src/main.c -o main.o", "file": "src/main.c"}
	]`
	writeFile(t, root, "compile_commands.json", ccj)

	// Clear the cache so this test gets fresh data
	compileCommandsCache = make(map[string][]compileCommand)

	parser := &CCppLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "src/main.c"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find: include (via utils.h) and include/net (via net/socket.h)
	// Should skip: stdio.h, stdlib.h (system includes with <>)
	if len(edges) < 2 {
		t.Fatalf("expected >=2 local import edges, got %d: %v", len(edges), edges)
	}

	targets := make(map[string]bool)
	for _, e := range edges {
		targets[e.TargetModule] = true
	}
	if !targets["mod-include"] {
		t.Error("missing import edge to mod-include (from utils.h)")
	}
	if !targets["mod-include-net"] {
		t.Error("missing import edge to mod-include-net (from net/socket.h)")
	}
}

func TestCCppParseImportsRelative(t *testing.T) {
	root := t.TempDir()

	// No compile_commands.json -- pure relative include resolution
	writeFile(t, root, "Makefile", "all: myapp\n")
	writeFile(t, root, "src/main.c", `
#include "../lib/math.h"
#include <string.h>
`)
	writeFile(t, root, "lib/math.h", "int add(int, int);\n")

	parser := &CCppLang{}
	edges, err := parser.ParseImports(filepath.Join(root, "src/main.c"), root)
	if err != nil {
		t.Fatal(err)
	}

	// Should find lib (via ../lib/math.h resolved from src/)
	if len(edges) != 1 {
		t.Fatalf("expected 1 local import edge, got %d: %v", len(edges), edges)
	}
	if edges[0].TargetModule != "mod-lib" {
		t.Errorf("expected target mod-lib, got %s", edges[0].TargetModule)
	}
}

// --- Scanner Integration Tests ---

func TestScannerFullPipeline(t *testing.T) {
	root := t.TempDir()
	db := setupTestDB(t)
	ctx := context.Background()

	// Create a Go project
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.21\n")
	writeFile(t, root, "main.go", `package main

import "example.com/app/internal/auth"

func main() { auth.Check() }
`)
	writeFile(t, root, "internal/auth/auth.go", "package auth\n\nfunc Check() {}\n")
	writeFile(t, root, "internal/db/store.go", "package db\n\nfunc Connect() {}\n")

	scanner := NewScanner(db)

	// Scan modules
	modules, err := scanner.ScanModules(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) < 3 {
		t.Fatalf("expected ≥3 modules, got %d", len(modules))
	}

	// Scan dependencies
	edges, err := scanner.ScanDependencies(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	// main should import auth
	foundAuthDep := false
	for _, e := range edges {
		if strings.Contains(e.TargetModule, "auth") {
			foundAuthDep = true
		}
	}
	if !foundAuthDep {
		t.Error("expected dependency edge to auth module")
	}

	// GetDependents should find root depends on auth
	deps, err := scanner.GetDependents(ctx, "mod-internal-auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) == 0 {
		t.Error("expected at least 1 dependent of auth module")
	}
}

func TestCoverageComputation(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert modules
	now := "2026-03-18T12:00:00Z"
	db.Exec(`INSERT INTO codebase_modules VALUES ('mod-auth', 'internal/auth', 'auth', 'go', 3, ?)`, now)
	db.Exec(`INSERT INTO codebase_modules VALUES ('mod-db', 'internal/db', 'db', 'go', 2, ?)`, now)
	db.Exec(`INSERT INTO codebase_modules VALUES ('mod-api', 'internal/api', 'api', 'go', 5, ?)`, now)

	// Insert a decision with affected_files in auth module
	db.Exec(`INSERT INTO artifacts VALUES ('dec-001', 'DecisionRecord', 1, 'active', '', '', 'Auth decision', 'body', '', ?, ?)`, now, now)
	db.Exec(`INSERT INTO affected_files VALUES ('dec-001', 'internal/auth/auth.go', '')`)
	db.Exec(`INSERT INTO affected_files VALUES ('dec-001', 'internal/auth/middleware.go', '')`)

	report, err := ComputeCoverage(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalModules != 3 {
		t.Errorf("expected 3 modules, got %d", report.TotalModules)
	}
	if report.CoveredCount != 1 {
		t.Errorf("expected 1 covered module, got %d", report.CoveredCount)
	}
	if report.BlindCount != 2 {
		t.Errorf("expected 2 blind modules, got %d", report.BlindCount)
	}

	// Check that auth is covered, db and api are blind
	for _, mc := range report.Modules {
		switch mc.Module.ID {
		case "mod-auth":
			if mc.Status != CoverageCovered {
				t.Errorf("auth should be covered, got %s", mc.Status)
			}
			if mc.DecisionCount != 1 {
				t.Errorf("auth should have 1 decision, got %d", mc.DecisionCount)
			}
		case "mod-db", "mod-api":
			if mc.Status != CoverageBlind {
				t.Errorf("%s should be blind, got %s", mc.Module.ID, mc.Status)
			}
		}
	}
}

// --- Self-hosting test ---

func TestSelfHostGoDetection(t *testing.T) {
	// Find quint-code project root (walk up from test file)
	root := findProjectRoot(t)
	if root == "" {
		t.Skip("could not find quint-code project root")
	}

	detector := &GoLang{}
	modules, err := detector.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) == 0 {
		t.Fatal("expected to detect Go modules in quint-code")
	}

	// Should find at least: root, cmd, db, internal/artifact, internal/fpf, internal/codebase
	expectedPaths := []string{"internal/artifact", "internal/fpf", "internal/codebase"}
	for _, expected := range expectedPaths {
		found := false
		for _, m := range modules {
			if m.Path == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find module at path %q, detected modules: %v", expected, moduleNames(modules))
		}
	}

	t.Logf("Detected %d Go modules in quint-code", len(modules))
	for _, m := range modules {
		t.Logf("  %s (%s) — %d files", m.Path, m.Name, m.FileCount)
	}
}

func TestSelfHostGoImports(t *testing.T) {
	root := findProjectRoot(t)
	if root == "" {
		t.Skip("could not find quint-code project root")
	}

	parser := &GoLang{}
	// Parse serve.go — should import internal/artifact and internal/codebase
	servePath := filepath.Join(root, "cmd/serve.go")
	if _, err := os.Stat(servePath); err != nil {
		t.Skip("serve.go not found")
	}

	edges, err := parser.ParseImports(servePath, root)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) == 0 {
		t.Fatal("expected imports from serve.go")
	}

	t.Logf("serve.go imports %d local modules", len(edges))
	for _, e := range edges {
		t.Logf("  → %s (from %s)", e.TargetModule, e.ImportPath)
	}
}

// --- Reference repo tests (real-world validation) ---

func TestReferenceRepoOpenSpec(t *testing.T) {
	root := findRepoRoot(t, "OpenSpec")
	if root == "" {
		t.Skip("OpenSpec reference repo not found")
	}

	d := &JSTSLang{}
	modules, err := d.DetectModules(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Detected %d JS/TS modules in OpenSpec:", len(modules))
	for _, m := range modules {
		t.Logf("  %-40s [%s] %d files", m.Path, m.Name, m.FileCount)
	}

	// OpenSpec has src/core, src/cli, src/utils, src/commands, src/telemetry, src/ui + root
	if len(modules) < 5 {
		t.Errorf("expected ≥5 modules for OpenSpec (root + src/* with index.ts), got %d", len(modules))
	}

	// Check that core module was detected
	foundCore := false
	for _, m := range modules {
		if strings.Contains(m.Path, "core") {
			foundCore = true
		}
	}
	if !foundCore {
		t.Error("expected to find src/core module in OpenSpec")
	}
}

func findRepoRoot(t *testing.T, repoName string) string {
	t.Helper()
	// Walk up to find quint-code project root, then check .context/repos/<repoName>
	projectRoot := findProjectRoot(t)
	if projectRoot == "" {
		return ""
	}
	// Go up from src/mcp to repo root
	repoRoot := filepath.Join(projectRoot, "..", "..", ".context", "repos", repoName)
	if _, err := os.Stat(repoRoot); err != nil {
		return ""
	}
	abs, _ := filepath.Abs(repoRoot)
	return abs
}

// --- helpers ---

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func moduleNames(modules []Module) []string {
	var names []string
	for _, m := range modules {
		names = append(names, m.Path+"("+m.Name+")")
	}
	return names
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from current directory looking for go.mod with quint-code module
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		modFile := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modFile); err == nil {
			if strings.Contains(string(data), "quint-code") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
