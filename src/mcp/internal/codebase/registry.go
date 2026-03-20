package codebase

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	ignore "github.com/sabhiram/go-gitignore"
)

// Registry maps file extensions to language detectors and import parsers.
type Registry struct {
	detectors map[string]ModuleDetector // lang -> detector
	parsers   map[string]ImportParser   // extension -> parser
}

// NewRegistry creates a registry with all supported languages.
func NewRegistry() *Registry {
	r := &Registry{
		detectors: make(map[string]ModuleDetector),
		parsers:   make(map[string]ImportParser),
	}

	// Register Go
	goImpl := &GoLang{}
	r.detectors["go"] = goImpl
	for _, ext := range goImpl.Extensions() {
		r.parsers[ext] = goImpl
	}

	// Register JS/TS
	jsImpl := &JSTSLang{}
	r.detectors["jsts"] = jsImpl
	for _, ext := range jsImpl.Extensions() {
		r.parsers[ext] = jsImpl
	}

	// Register Python
	pyImpl := &PythonLang{}
	r.detectors["python"] = pyImpl
	for _, ext := range pyImpl.Extensions() {
		r.parsers[ext] = pyImpl
	}

	// Register Rust
	rsImpl := &RustLang{}
	r.detectors["rust"] = rsImpl
	for _, ext := range rsImpl.Extensions() {
		r.parsers[ext] = rsImpl
	}

	// Register C/C++
	cImpl := &CCppLang{}
	r.detectors["c_cpp"] = cImpl
	for _, ext := range cImpl.Extensions() {
		r.parsers[ext] = cImpl
	}

	return r
}

// Detectors returns all registered module detectors.
func (r *Registry) Detectors() []ModuleDetector {
	var result []ModuleDetector
	for _, d := range r.detectors {
		result = append(result, d)
	}
	return result
}

// ParserForFile returns the import parser for a file, or nil if unsupported.
func (r *Registry) ParserForFile(path string) ImportParser {
	ext := strings.ToLower(filepath.Ext(path))
	return r.parsers[ext]
}

// IgnoreChecker determines if paths should be excluded from scanning.
// It respects .gitignore (local + global), .quintignore, and a minimal set of
// hardcoded dirs that should always be skipped (.git, .quint).
type IgnoreChecker struct {
	matchers []ignore.IgnoreParser
}

// NewIgnoreChecker builds an IgnoreChecker for the given project root.
// Reads .gitignore, global git ignore files, and .quintignore.
func NewIgnoreChecker(projectRoot string) *IgnoreChecker {
	ic := &IgnoreChecker{}

	// 1. Always-excluded (not configurable — these are never project code)
	ic.matchers = append(ic.matchers, ignore.CompileIgnoreLines(
		".git",
		".quint",
	))

	// 2. Global git ignore files
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{
			filepath.Join(home, ".gitignore"),
			filepath.Join(home, ".config", "git", "ignore"),
		} {
			if data, err := os.ReadFile(name); err == nil {
				lines := strings.Split(string(data), "\n")
				ic.matchers = append(ic.matchers, ignore.CompileIgnoreLines(lines...))
			}
		}
	}

	// 3. Project .gitignore (root level)
	if data, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore")); err == nil {
		lines := strings.Split(string(data), "\n")
		ic.matchers = append(ic.matchers, ignore.CompileIgnoreLines(lines...))
	}

	// 4. .quintignore (project-specific overrides)
	if data, err := os.ReadFile(filepath.Join(projectRoot, ".quintignore")); err == nil {
		lines := strings.Split(string(data), "\n")
		ic.matchers = append(ic.matchers, ignore.CompileIgnoreLines(lines...))
	}

	return ic
}

// IsIgnored returns true if the relative path should be excluded.
func (ic *IgnoreChecker) IsIgnored(relPath string) bool {
	for _, m := range ic.matchers {
		if m.MatchesPath(relPath) {
			return true
		}
	}
	return false
}

// defaultIgnoreChecker is a lazy-initialized ignore checker for the common case.
var (
	defaultIgnoreOnce    sync.Once
	defaultIgnoreChecker *IgnoreChecker
)

// GetIgnoreChecker returns a cached IgnoreChecker for the given project root.
func GetIgnoreChecker(projectRoot string) *IgnoreChecker {
	defaultIgnoreOnce.Do(func() {
		defaultIgnoreChecker = NewIgnoreChecker(projectRoot)
	})
	return defaultIgnoreChecker
}

// IsExcludedDir checks if a directory should be skipped during walking.
// Uses the IgnoreChecker if available, otherwise falls back to the dir name check.
func IsExcludedDir(name string) bool {
	// Minimal hardcoded set — only things that should NEVER be scanned
	switch name {
	case ".git", ".quint":
		return true
	}
	return false
}
