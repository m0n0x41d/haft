package codebase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CCppLang implements ModuleDetector and ImportParser for C/C++ projects.
type CCppLang struct{}

func (c *CCppLang) Language() string { return "c_cpp" }
func (c *CCppLang) Extensions() []string {
	return []string{".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hxx"}
}

// compileCommand represents a single entry in compile_commands.json.
type compileCommand struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
}

// DetectModules discovers C/C++ modules by reading compile_commands.json,
// falling back to directory-based heuristics with Makefile/CMakeLists.txt markers.
func (c *CCppLang) DetectModules(projectRoot string) ([]Module, error) {
	// Try compile_commands.json first
	ccjPath := findCompileCommandsJSON(projectRoot)
	if ccjPath != "" {
		modules, err := c.detectFromCompileCommands(ccjPath, projectRoot)
		if err == nil && len(modules) > 0 {
			return modules, nil
		}
		// Fall through to directory scan if compile_commands.json
		// produced no modules (path resolution issues) or failed to parse.
	}

	// Fallback: look for project markers
	hasMarker := false
	for _, marker := range []string{"Makefile", "makefile", "GNUmakefile", "CMakeLists.txt", "meson.build"} {
		if _, err := os.Stat(filepath.Join(projectRoot, marker)); err == nil {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return nil, nil
	}

	return c.detectFromDirectoryStructure(projectRoot)
}

// detectFromCompileCommands groups source files from compile_commands.json into modules by directory.
func (c *CCppLang) detectFromCompileCommands(ccjPath, projectRoot string) ([]Module, error) {
	data, err := os.ReadFile(ccjPath)
	if err != nil {
		return nil, fmt.Errorf("read compile_commands.json: %w", err)
	}

	var commands []compileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil, fmt.Errorf("parse compile_commands.json: %w", err)
	}

	if len(commands) == 0 {
		return nil, nil
	}

	// Resolve symlinks in project root for reliable path matching.
	// macOS has /private/var symlinks and projects may be accessed via symlinks.
	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		canonicalRoot = projectRoot
	}
	canonicalRoot, _ = filepath.Abs(canonicalRoot)

	// Group source files by their directory (relative to project root)
	dirFiles := make(map[string]int)
	for _, cmd := range commands {
		srcFile := cmd.File
		// Resolve relative paths against the command's directory
		if !filepath.IsAbs(srcFile) {
			srcFile = filepath.Join(cmd.Directory, srcFile)
		}
		srcFile, err := filepath.Abs(srcFile)
		if err != nil {
			continue
		}
		// Resolve symlinks in source path too
		if resolved, err := filepath.EvalSymlinks(srcFile); err == nil {
			srcFile = resolved
		}
		rel, err := filepath.Rel(canonicalRoot, srcFile)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		dirFiles[dir]++
	}

	var modules []Module
	for dir, count := range dirFiles {
		name := filepath.Base(dir)
		id := moduleID(dir)
		if dir == "" {
			name = filepath.Base(projectRoot)
			id = "mod-root"
		}
		modules = append(modules, Module{
			ID:        id,
			Path:      dir,
			Name:      name,
			Lang:      "c_cpp",
			FileCount: count,
		})
	}

	// If all files ended up in root, that's fine -- single module project
	return modules, nil
}

// detectFromDirectoryStructure walks the project and groups C/C++ files by directory.
func (c *CCppLang) detectFromDirectoryStructure(projectRoot string) ([]Module, error) {
	dirFiles := make(map[string]int)

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
		if !isCCppSource(d.Name()) {
			return nil
		}

		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil
		}
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		dirFiles[dir]++
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk C/C++ project: %w", err)
	}

	if len(dirFiles) == 0 {
		return nil, nil
	}

	var modules []Module
	for dir, count := range dirFiles {
		name := filepath.Base(dir)
		id := moduleID(dir)
		if dir == "" {
			name = filepath.Base(projectRoot)
			id = "mod-root"
		}
		modules = append(modules, Module{
			ID:        id,
			Path:      dir,
			Name:      name,
			Lang:      "c_cpp",
			FileCount: count,
		})
	}

	return modules, nil
}

// findCompileCommandsJSON searches common locations for compile_commands.json.
func findCompileCommandsJSON(projectRoot string) string {
	candidates := []string{
		filepath.Join(projectRoot, "compile_commands.json"),
		filepath.Join(projectRoot, "build", "compile_commands.json"),
	}

	// Also check cmake-build-* directories
	entries, err := os.ReadDir(projectRoot)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "cmake-build-") {
				candidates = append(candidates, filepath.Join(projectRoot, e.Name(), "compile_commands.json"))
			}
		}
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

var (
	cIncludeLocalRe = regexp.MustCompile(`(?m)^\s*#\s*include\s+"([^"]+)"`)
)

// ParseImports extracts #include "..." edges from a C/C++ source file.
// System includes (#include <...>) are skipped since they're external.
func (c *CCppLang) ParseImports(filePath string, projectRoot string) ([]ImportEdge, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}

	relFile, _ := filepath.Rel(projectRoot, filePath)
	sourceDir := filepath.Dir(relFile)
	content := string(data)

	// Try to load include paths from compile_commands.json for better resolution
	includePaths := c.extractIncludePaths(filePath, projectRoot)

	var edges []ImportEdge
	seen := make(map[string]bool)

	for _, m := range cIncludeLocalRe.FindAllStringSubmatch(content, -1) {
		includePath := m[1]

		// Resolve the include to a directory (module)
		targetDir := c.resolveInclude(includePath, sourceDir, projectRoot, includePaths)
		if targetDir == "" {
			continue
		}

		targetMod := moduleID(targetDir)
		if seen[targetMod] {
			continue
		}
		seen[targetMod] = true

		// Don't create self-edges
		sourceMod := moduleID(sourceDir)
		if sourceMod == targetMod {
			continue
		}

		edges = append(edges, ImportEdge{
			SourceModule: sourceMod,
			TargetModule: targetMod,
			SourceFile:   relFile,
			ImportPath:   includePath,
		})
	}

	return edges, nil
}

// resolveInclude tries to find which directory an included file belongs to.
// It checks: relative to source, then each -I path from compile_commands.json.
func (c *CCppLang) resolveInclude(includePath, sourceDir, projectRoot string, includePaths []string) string {
	// 1. Relative to source file's directory
	candidate := filepath.Join(sourceDir, includePath)
	candidateAbs := filepath.Join(projectRoot, candidate)
	if _, err := os.Stat(candidateAbs); err == nil {
		dir := filepath.Dir(candidate)
		if dir == "." {
			return ""
		}
		return dir
	}

	// 2. Relative to each include path from compile_commands.json
	for _, incDir := range includePaths {
		// Make include path relative to project root
		relInc := incDir
		if filepath.IsAbs(incDir) {
			var err error
			relInc, err = filepath.Rel(projectRoot, incDir)
			if err != nil || strings.HasPrefix(relInc, "..") {
				continue
			}
		}

		candidate := filepath.Join(relInc, includePath)
		candidateAbs := filepath.Join(projectRoot, candidate)
		if _, err := os.Stat(candidateAbs); err == nil {
			dir := filepath.Dir(candidate)
			if dir == "." {
				return ""
			}
			return dir
		}
	}

	// 3. Try from project root
	candidate = includePath
	candidateAbs = filepath.Join(projectRoot, candidate)
	if _, err := os.Stat(candidateAbs); err == nil {
		dir := filepath.Dir(candidate)
		if dir == "." {
			return ""
		}
		return dir
	}

	return ""
}

// extractIncludePaths returns -I paths from compile_commands.json for the given file.
func (c *CCppLang) extractIncludePaths(filePath, projectRoot string) []string {
	ccjPath := findCompileCommandsJSON(projectRoot)
	if ccjPath == "" {
		return nil
	}

	data, err := os.ReadFile(ccjPath)
	if err != nil {
		return nil
	}
	var commands []compileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil
	}

	// Find the command for this file — resolve symlinks for reliable matching
	absFile, _ := filepath.Abs(filePath)
	if resolved, err := filepath.EvalSymlinks(absFile); err == nil {
		absFile = resolved
	}
	for _, cmd := range commands {
		cmdFile := cmd.File
		if !filepath.IsAbs(cmdFile) {
			cmdFile = filepath.Join(cmd.Directory, cmdFile)
		}
		cmdFileAbs, _ := filepath.Abs(cmdFile)
		if resolved, err := filepath.EvalSymlinks(cmdFileAbs); err == nil {
			cmdFileAbs = resolved
		}
		if cmdFileAbs != absFile {
			continue
		}

		return parseIncludeFlags(cmd)
	}

	return nil
}

// parseIncludeFlags extracts -I paths from a compile command.
func parseIncludeFlags(cmd compileCommand) []string {
	var paths []string

	// Use Arguments if available, otherwise split Command
	args := cmd.Arguments
	if len(args) == 0 && cmd.Command != "" {
		args = strings.Fields(cmd.Command)
	}

	for i, arg := range args {
		if arg == "-I" && i+1 < len(args) {
			path := args[i+1]
			if !filepath.IsAbs(path) {
				path = filepath.Join(cmd.Directory, path)
			}
			paths = append(paths, path)
		} else if strings.HasPrefix(arg, "-I") {
			path := arg[2:]
			if !filepath.IsAbs(path) {
				path = filepath.Join(cmd.Directory, path)
			}
			paths = append(paths, path)
		}
	}

	return paths
}

func isCCppSource(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return true
	}
	return false
}
