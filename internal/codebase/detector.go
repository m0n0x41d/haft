package codebase

// Module represents a detected module/package in the codebase.
type Module struct {
	ID        string // auto-generated from path, e.g., "mod-internal-auth"
	Path      string // relative path from project root
	Name      string // human-readable name, e.g., "auth"
	Lang      string // go, js, ts, python, rust, mixed, unknown
	FileCount int    // number of source files
}

// ImportEdge represents a dependency between two modules.
type ImportEdge struct {
	SourceModule string // module that imports
	TargetModule string // module being imported
	SourceFile   string // file containing the import
	ImportPath   string // raw import path as written in source
}

// ModuleDetector discovers module/package boundaries in a project.
type ModuleDetector interface {
	// DetectModules walks the project and returns discovered modules.
	DetectModules(projectRoot string) ([]Module, error)

	// Language returns the language this detector handles.
	Language() string
}

// ImportParser extracts import/dependency edges from source files.
type ImportParser interface {
	// ParseImports extracts import edges from a single source file.
	// Returns edges with raw import paths. Caller resolves to modules.
	ParseImports(filePath string, projectRoot string) ([]ImportEdge, error)

	// Extensions returns file extensions this parser handles.
	Extensions() []string
}
