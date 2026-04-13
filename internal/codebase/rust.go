package codebase

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RustLang implements ModuleDetector and ImportParser for Rust projects.
type RustLang struct{}

func (r *RustLang) Language() string     { return "rust" }
func (r *RustLang) Extensions() []string { return []string{".rs"} }

// DetectModules discovers Rust crates/modules by looking for Cargo.toml and mod.rs/lib.rs.
func (r *RustLang) DetectModules(projectRoot string) ([]Module, error) {
	cargoPath := filepath.Join(projectRoot, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err != nil {
		return nil, nil // Not a Rust project
	}

	var modules []Module
	seen := make(map[string]bool)

	// Check for workspace members (Cargo.toml with [workspace])
	isWorkspace := isCargoWorkspace(cargoPath)

	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if IsExcludedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Detect crate boundaries via Cargo.toml
		if d.Name() == "Cargo.toml" && path != cargoPath {
			dir := filepath.Dir(path)
			relDir, err := filepath.Rel(projectRoot, dir)
			if err != nil {
				return nil
			}
			if !seen[relDir] {
				seen[relDir] = true
				modules = append(modules, Module{
					ID:        moduleID(relDir),
					Path:      relDir,
					Name:      filepath.Base(dir),
					Lang:      "rust",
					FileCount: countFiles(filepath.Join(dir, "src"), ".rs"),
				})
			}
			return nil
		}

		// Detect modules via mod.rs or lib.rs in src/
		if d.Name() == "mod.rs" || d.Name() == "lib.rs" || d.Name() == "main.rs" {
			dir := filepath.Dir(path)
			relDir, err := filepath.Rel(projectRoot, dir)
			if err != nil {
				return nil
			}
			if seen[relDir] {
				return nil
			}
			seen[relDir] = true

			fileCount := countFiles(dir, ".rs")
			name := filepath.Base(dir)
			if name == "src" {
				name = filepath.Base(filepath.Dir(dir))
			}

			modules = append(modules, Module{
				ID:        moduleID(relDir),
				Path:      relDir,
				Name:      name,
				Lang:      "rust",
				FileCount: fileCount,
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk Rust project: %w", err)
	}

	// If no sub-modules found, add root crate
	if len(modules) == 0 && !isWorkspace {
		srcDir := filepath.Join(projectRoot, "src")
		if _, err := os.Stat(srcDir); err == nil {
			modules = append(modules, Module{
				ID:        "mod-root",
				Path:      "",
				Name:      filepath.Base(projectRoot),
				Lang:      "rust",
				FileCount: countFiles(srcDir, ".rs"),
			})
		}
	}

	return modules, nil
}

var (
	rsUseRe = regexp.MustCompile(`(?m)^\s*use\s+(?:crate::)?(\w+)`)
	rsModRe = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?mod\s+(\w+)\s*;`)
)

// ParseImports extracts use/mod edges from a Rust source file.
func (r *RustLang) ParseImports(filePath string, projectRoot string) ([]ImportEdge, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	relFile, _ := filepath.Rel(projectRoot, filePath)
	sourceDir := filepath.Dir(relFile)
	content := string(data)

	var edges []ImportEdge
	seen := make(map[string]bool)

	// Extract `use crate::module` patterns
	for _, m := range rsUseRe.FindAllStringSubmatch(content, -1) {
		modName := m[1]
		// Skip std/external crate references
		if isRustStdModule(modName) {
			continue
		}
		targetMod := moduleID(modName)
		if seen[targetMod] {
			continue
		}
		seen[targetMod] = true

		edges = append(edges, ImportEdge{
			SourceModule: moduleID(sourceDir),
			TargetModule: targetMod,
			SourceFile:   relFile,
			ImportPath:   "crate::" + modName,
		})
	}

	// Extract `mod submodule;` declarations
	for _, m := range rsModRe.FindAllStringSubmatch(content, -1) {
		modName := m[1]
		if modName == "tests" {
			continue
		}
		// mod declarations reference a submodule in the same directory
		targetPath := filepath.Join(sourceDir, modName)
		targetMod := moduleID(targetPath)
		if seen[targetMod] {
			continue
		}
		seen[targetMod] = true

		edges = append(edges, ImportEdge{
			SourceModule: moduleID(sourceDir),
			TargetModule: targetMod,
			SourceFile:   relFile,
			ImportPath:   "mod " + modName,
		})
	}

	return edges, nil
}

func isCargoWorkspace(cargoPath string) bool {
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "[workspace]")
}

func isRustStdModule(name string) bool {
	stdMods := map[string]bool{
		"std": true, "core": true, "alloc": true, "collections": true,
		"env": true, "fmt": true, "fs": true, "io": true, "net": true,
		"os": true, "path": true, "sync": true, "thread": true,
		"time": true, "vec": true, "string": true, "hash": true,
		"iter": true, "mem": true, "ops": true, "result": true,
		"option": true, "convert": true, "default": true,
	}
	return stdMods[name]
}
