package codebase

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PythonLang implements ModuleDetector and ImportParser for Python projects.
type PythonLang struct{}

func (p *PythonLang) Language() string    { return "python" }
func (p *PythonLang) Extensions() []string { return []string{".py"} }

// DetectModules discovers Python packages by looking for __init__.py or pyproject.toml.
func (p *PythonLang) DetectModules(projectRoot string) ([]Module, error) {
	// Check for Python project markers
	hasPyProject := false
	for _, marker := range []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(projectRoot, marker)); err == nil {
			hasPyProject = true
			break
		}
	}
	if !hasPyProject {
		return nil, nil
	}

	var modules []Module
	seen := make(map[string]bool)

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

		// Look for __init__.py to identify packages
		if d.Name() != "__init__.py" {
			return nil
		}

		dir := filepath.Dir(path)
		relDir, err := filepath.Rel(projectRoot, dir)
		if err != nil {
			return nil
		}

		if seen[relDir] {
			return nil
		}
		seen[relDir] = true

		fileCount := countFiles(dir, ".py")
		name := filepath.Base(dir)

		modules = append(modules, Module{
			ID:        moduleID(relDir),
			Path:      relDir,
			Name:      name,
			Lang:      "python",
			FileCount: fileCount,
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk Python project: %w", err)
	}

	// If no __init__.py found but project has .py files at root, add root as module
	if len(modules) == 0 {
		if countFiles(projectRoot, ".py") > 0 {
			modules = append(modules, Module{
				ID:        "mod-root",
				Path:      "",
				Name:      filepath.Base(projectRoot),
				Lang:      "python",
				FileCount: countFiles(projectRoot, ".py"),
			})
		}
	}

	return modules, nil
}

var (
	pyFromImportRe = regexp.MustCompile(`(?m)^\s*from\s+(\S+)\s+import\s+`)
	pyImportRe     = regexp.MustCompile(`(?m)^\s*import\s+(\S+)`)
)

// ParseImports extracts import edges from a Python source file.
func (p *PythonLang) ParseImports(filePath string, projectRoot string) ([]ImportEdge, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	relFile, _ := filepath.Rel(projectRoot, filePath)
	sourceDir := filepath.Dir(relFile)
	content := string(data)

	var edges []ImportEdge
	seen := make(map[string]bool)

	for _, re := range []*regexp.Regexp{pyFromImportRe, pyImportRe} {
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			importPath := m[1]

			// Handle relative imports (from . or from .foo)
			if strings.HasPrefix(importPath, ".") {
				dots := 0
				for _, c := range importPath {
					if c == '.' {
						dots++
					} else {
						break
					}
				}
				rest := importPath[dots:]
				// Go up 'dots-1' directories from source
				base := sourceDir
				for i := 1; i < dots; i++ {
					base = filepath.Dir(base)
				}
				if rest != "" {
					// Convert dotted path to directory path
					importPath = filepath.Join(base, strings.ReplaceAll(rest, ".", string(filepath.Separator)))
				} else {
					importPath = base
				}
			} else {
				// Absolute import — take the top-level package name
				parts := strings.SplitN(importPath, ".", 2)
				importPath = parts[0]
			}

			targetMod := moduleID(importPath)
			if seen[targetMod] {
				continue
			}
			seen[targetMod] = true

			edges = append(edges, ImportEdge{
				SourceModule: moduleID(sourceDir),
				TargetModule: targetMod,
				SourceFile:   relFile,
				ImportPath:   m[1],
			})
		}
	}

	return edges, nil
}
