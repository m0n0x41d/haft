package codebase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// JSTSLang implements ModuleDetector and ImportParser for JavaScript/TypeScript.
type JSTSLang struct{}

func (j *JSTSLang) Language() string { return "jsts" }
func (j *JSTSLang) Extensions() []string {
	return []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".mts"}
}

// DetectModules discovers JS/TS packages by looking for package.json files.
// Handles monorepo workspaces.
func (j *JSTSLang) DetectModules(projectRoot string) ([]Module, error) {
	rootPkg := filepath.Join(projectRoot, "package.json")
	if _, err := os.Stat(rootPkg); err != nil {
		return nil, nil // Not a JS/TS project
	}

	var modules []Module

	// Check for workspaces in root package.json
	workspaceDirs := j.readWorkspaces(rootPkg)

	if len(workspaceDirs) > 0 {
		// Monorepo: each workspace is a module
		for _, pattern := range workspaceDirs {
			matches, _ := filepath.Glob(filepath.Join(projectRoot, pattern, "package.json"))
			for _, m := range matches {
				dir := filepath.Dir(m)
				relDir, _ := filepath.Rel(projectRoot, dir)
				modules = append(modules, Module{
					ID:        moduleID(relDir),
					Path:      relDir,
					Name:      j.readPackageName(m),
					Lang:      "js",
					FileCount: countJSTSFiles(dir),
				})
			}
		}
	}

	// Add root as a module
	modules = append(modules, Module{
		ID:        moduleID(""),
		Path:      "",
		Name:      j.readPackageName(rootPkg),
		Lang:      "js",
		FileCount: countJSTSFiles(projectRoot),
	})

	// For single-package projects (no workspaces): detect sub-modules
	// via directories containing index.ts/index.js under src/
	if len(workspaceDirs) == 0 {
		srcDir := filepath.Join(projectRoot, "src")
		if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
			filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() {
					return nil
				}
				if IsExcludedDir(d.Name()) {
					return filepath.SkipDir
				}
				// Check for index.ts or index.js (barrel file = module boundary)
				for _, indexFile := range []string{"index.ts", "index.js", "index.tsx", "index.jsx"} {
					if _, err := os.Stat(filepath.Join(path, indexFile)); err == nil {
						relDir, _ := filepath.Rel(projectRoot, path)
						modules = append(modules, Module{
							ID:        moduleID(relDir),
							Path:      relDir,
							Name:      filepath.Base(path),
							Lang:      "js",
							FileCount: countJSTSFiles(path),
						})
						break
					}
				}
				return nil
			})
		}
	}

	return modules, nil
}

var (
	jsImportRe  = regexp.MustCompile(`(?m)^\s*import\s+.*?from\s+['"]([^'"]+)['"]`)
	jsRequireRe = regexp.MustCompile(`(?m)\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsReExportRe = regexp.MustCompile(`(?m)^\s*export\s+.*?from\s+['"]([^'"]+)['"]`)
)

// ParseImports extracts import/require edges from a JS/TS file.
func (j *JSTSLang) ParseImports(filePath string, projectRoot string) ([]ImportEdge, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	relFile, _ := filepath.Rel(projectRoot, filePath)
	sourceDir := filepath.Dir(relFile)
	content := string(data)

	var edges []ImportEdge
	seen := make(map[string]bool)

	for _, re := range []*regexp.Regexp{jsImportRe, jsRequireRe, jsReExportRe} {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			importPath := match[1]
			if !isLocalJSImport(importPath) {
				continue
			}

			// Resolve relative path to module
			resolved := filepath.Clean(filepath.Join(sourceDir, importPath))
			targetMod := moduleID(resolved)

			key := targetMod
			if seen[key] {
				continue
			}
			seen[key] = true

			edges = append(edges, ImportEdge{
				SourceModule: moduleID(sourceDir),
				TargetModule: targetMod,
				SourceFile:   relFile,
				ImportPath:   importPath,
			})
		}
	}

	return edges, nil
}

func isLocalJSImport(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func (j *JSTSLang) readWorkspaces(pkgPath string) []string {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var pkg struct {
		Workspaces interface{} `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	switch w := pkg.Workspaces.(type) {
	case []interface{}:
		var result []string
		for _, item := range w {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case map[string]interface{}:
		// { "packages": ["packages/*"] } format
		if pkgs, ok := w["packages"].([]interface{}); ok {
			var result []string
			for _, item := range pkgs {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func (j *JSTSLang) readPackageName(pkgPath string) string {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return filepath.Base(filepath.Dir(pkgPath))
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Name == "" {
		return filepath.Base(filepath.Dir(pkgPath))
	}
	return pkg.Name
}

func countJSTSFiles(dir string) int {
	jsExts := map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".mts": true}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && jsExts[filepath.Ext(e.Name())] {
			count++
		}
	}
	return count
}

func init() {
	_ = fmt.Sprintf // avoid unused import
}
