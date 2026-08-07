package codebase

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	ignore "github.com/sabhiram/go-gitignore"
)

// Registry maps file extensions to language detectors, import parsers, symbol
// adapters, and code-graph edge resolvers. Adding a language = one adapter type
// plus entries here; orchestration codes against ports, never a specific language.
type Registry struct {
	detectors      map[string]ModuleDetector // lang -> detector
	parsers        map[string]ImportParser   // extension -> parser
	resolvers      map[string]EdgeResolver   // extension -> code-graph edge resolver
	symbolAdapters map[string]SymbolAdapter  // extension -> canonical symbol extractor
}

// NewRegistry creates a registry with all supported languages.
func NewRegistry() *Registry {
	r := &Registry{
		detectors:      make(map[string]ModuleDetector),
		parsers:        make(map[string]ImportParser),
		resolvers:      make(map[string]EdgeResolver),
		symbolAdapters: make(map[string]SymbolAdapter),
	}

	// Register Go (detector + import parser + code-graph edge resolver)
	goImpl := &GoLang{}
	r.detectors["go"] = goImpl
	for _, ext := range goImpl.Extensions() {
		r.parsers[ext] = goImpl
		r.resolvers[ext] = goImpl
	}

	// Register JS/TS (detector + import parser + code-graph edge resolver:
	// class/interface `extends` + `implements` from explicit heritage clauses;
	// call edges deferred)
	jsImpl := &JSTSLang{}
	r.detectors["jsts"] = jsImpl
	for _, ext := range jsImpl.Extensions() {
		r.parsers[ext] = jsImpl
		r.resolvers[ext] = jsImpl
		r.symbolAdapters[ext] = jsImpl
	}
	vueImpl := &VueLang{}
	for _, ext := range vueImpl.Extensions() {
		r.resolvers[ext] = vueImpl
		r.symbolAdapters[ext] = vueImpl
	}

	// Register Python (detector + import parser + code-graph edge resolver:
	// class-inheritance `extends` edges; call edges deferred — Python dispatch is
	// dynamic and not soundly resolvable from the AST alone)
	pyImpl := &PythonLang{}
	r.detectors["python"] = pyImpl
	for _, ext := range pyImpl.Extensions() {
		r.parsers[ext] = pyImpl
		r.resolvers[ext] = pyImpl
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

// ResolverForFile returns the code-graph edge resolver for a file, or nil if
// no language adapter resolves edges for that extension (node extraction may
// still work — edges are a separate, incrementally-grown capability).
func (r *Registry) ResolverForFile(path string) EdgeResolver {
	ext := strings.ToLower(filepath.Ext(path))
	return r.resolvers[ext]
}

// IgnoreChecker determines if paths should be excluded from scanning.
// It respects .gitignore (local + global), .haftignore, and a minimal set of
// hardcoded dirs that should always be skipped (.git, .haft).
type IgnoreChecker struct {
	matchers []ignore.IgnoreParser
}

// NewIgnoreChecker builds an IgnoreChecker for the given project root.
// Reads .gitignore, global git ignore files, and .haftignore.
func NewIgnoreChecker(projectRoot string) *IgnoreChecker {
	ic := &IgnoreChecker{}

	// 1. Always-excluded (not configurable — these are never project code)
	ic.matchers = append(ic.matchers, ignore.CompileIgnoreLines(
		".git",
		".haft",
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

	// 4. .haftignore (project-specific overrides)
	if data, err := os.ReadFile(filepath.Join(projectRoot, ".haftignore")); err == nil {
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
	case ".git", ".haft", ".claude", ".context", "node_modules":
		return true
	}
	return false
}

// walkProjectFiles is the canonical filesystem boundary for derived project
// indexes. Every consumer sees the same hard exclusions and ignore carriers,
// so a project model cannot accidentally traverse dependency trees that the
// symbol scanner excludes.
func walkProjectFiles(
	projectRoot string,
	visit func(path string, relPath string, entry os.DirEntry) error,
) error {
	ignoreChecker := NewIgnoreChecker(projectRoot)
	return filepath.WalkDir(projectRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if entry.IsDir() {
			if IsExcludedDir(entry.Name()) {
				return filepath.SkipDir
			}
			if relPath != "." && ignoreChecker.IsIgnored(relPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoreChecker.IsIgnored(relPath) {
			return nil
		}
		return visit(path, relPath, entry)
	})
}
