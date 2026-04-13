package codebase

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// GoLang implements ModuleDetector and ImportParser for Go projects.
type GoLang struct{}

func (g *GoLang) Language() string     { return "go" }
func (g *GoLang) Extensions() []string { return []string{".go"} }

// DetectModules discovers Go packages by walking directories for .go files.
// Each directory containing .go files (excluding _test.go-only dirs) is a module.
func (g *GoLang) DetectModules(projectRoot string) ([]Module, error) {
	// Find all go.mod files — supports both root-level and nested (e.g., src/mcp/go.mod)
	var goModRoots []string
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && IsExcludedDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.Name() == "go.mod" {
			goModRoots = append(goModRoots, filepath.Dir(path))
		}
		return nil
	})

	if len(goModRoots) == 0 {
		return nil, nil // Not a Go project
	}

	var modules []Module
	seen := make(map[string]bool)

	for _, goModRoot := range goModRoots {
		err := filepath.WalkDir(goModRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if IsExcludedDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}

			if filepath.Ext(path) != ".go" {
				return nil
			}

			dir := filepath.Dir(path)
			relDir, err := filepath.Rel(projectRoot, dir)
			if err != nil {
				return nil
			}
			if relDir == "." {
				relDir = ""
			}

			if seen[relDir] {
				return nil
			}
			seen[relDir] = true

			fileCount := countFiles(dir, ".go")
			name := filepath.Base(dir)
			if relDir == "" {
				name = filepath.Base(projectRoot)
			}

			modules = append(modules, Module{
				ID:        moduleID(relDir),
				Path:      relDir,
				Name:      name,
				Lang:      "go",
				FileCount: fileCount,
			})

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk Go project at %s: %w", goModRoot, err)
		}
	}

	return modules, nil
}

// ParseImports extracts import edges from a Go source file using go/parser.
func (g *GoLang) ParseImports(filePath string, projectRoot string) ([]ImportEdge, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, nil // skip unparseable files
	}

	relFile, _ := filepath.Rel(projectRoot, filePath)
	sourceDir := filepath.Dir(relFile)

	// Find go.mod by walking up from the file's directory
	goModDir := findGoModDir(filepath.Dir(filePath), projectRoot)
	modulePath := ""
	goModPrefix := "" // relative path from projectRoot to go.mod dir
	if goModDir != "" {
		modulePath = readGoModulePathFromDir(goModDir)
		goModPrefix, _ = filepath.Rel(projectRoot, goModDir)
		if goModPrefix == "." {
			goModPrefix = ""
		}
	}

	var edges []ImportEdge
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Only include local imports (same module)
		if modulePath != "" && strings.HasPrefix(importPath, modulePath) {
			// Convert import path to relative directory, accounting for nested go.mod
			localPath := strings.TrimPrefix(importPath, modulePath)
			localPath = strings.TrimPrefix(localPath, "/")
			// Prepend go.mod directory prefix to get path relative to projectRoot
			if goModPrefix != "" {
				localPath = filepath.Join(goModPrefix, localPath)
			}

			edges = append(edges, ImportEdge{
				SourceModule: moduleID(sourceDir),
				TargetModule: moduleID(localPath),
				SourceFile:   relFile,
				ImportPath:   importPath,
			})
		}
	}

	return edges, nil
}

// findGoModDir walks up from dir to find the directory containing go.mod.
func findGoModDir(dir string, stopAt string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if dir == stopAt || dir == filepath.Dir(dir) {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

// readGoModulePathFromDir reads the module path from go.mod in the given directory.
func readGoModulePathFromDir(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

// moduleID generates a stable ID from a relative path.
func moduleID(relPath string) string {
	if relPath == "" || relPath == "." {
		return "mod-root"
	}
	// Replace path separators with dashes
	id := strings.ReplaceAll(relPath, string(filepath.Separator), "-")
	id = strings.ReplaceAll(id, "/", "-")
	return "mod-" + id
}

// countFiles counts files with a given extension in a directory (non-recursive).
func countFiles(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ext {
			count++
		}
	}
	return count
}
