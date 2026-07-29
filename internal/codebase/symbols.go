package codebase

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

// Symbol represents an extracted code symbol (function, type, class, etc.)
type Symbol struct {
	Name     string // symbol name
	Kind     string // canonical kind: func, method, class, interface, type_alias, enum, constant, variable, property
	Line     int    // 1-based line number
	Exported bool   // starts with uppercase (Go) or is exported
}

// FileSymbols holds symbols extracted from a single file.
type FileSymbols struct {
	Path     string   // relative path from project root
	Language string   // "go", "python", "javascript", "typescript", "rust", "c", "cpp"
	Lines    int      // total line count
	Symbols  []Symbol // extracted symbols, sorted by line
	ModTime  int64    // modification time (unix nano) for recency sorting
}

// RepoMap is the complete symbol map for a repository.
type RepoMap struct {
	Files      []FileSymbols
	TotalFiles int
	TotalSyms  int
}

// languageInfo is the transitional metadata for languages still using the
// legacy symbol-body queries in symhash.go. JS/TS lives exclusively behind the
// registered SymbolAdapter, so it has no duplicate entry here.
type languageInfo struct {
	name string
	lang *sitter.Language
}

var languages = map[string]*languageInfo{
	".go": {
		name: "go",
		lang: golang.GetLanguage(),
	},
	".py": {
		name: "python",
		lang: python.GetLanguage(),
	},
	".rs": {
		name: "rust",
		lang: rust.GetLanguage(),
	},
	".c": {
		name: "c",
		lang: c.GetLanguage(),
	},
	".h": {
		name: "c",
		lang: c.GetLanguage(),
	},
	".cpp": {
		name: "cpp",
		lang: cpp.GetLanguage(),
	},
}

// BuildRepoMap scans the project and extracts symbols from all supported files.
func BuildRepoMap(projectRoot string, maxFiles int) (*RepoMap, error) {
	if maxFiles <= 0 {
		maxFiles = 500
	}
	registry := NewRegistry()
	defaultBudget := DefaultIndexBudget()
	fileLimit, err := NewFileCount(int64(maxFiles))
	if err != nil {
		return nil, err
	}
	budget, err := NewIndexBudget(IndexBudgetSpec{
		MaxFileBytes:     defaultBudget.MaxFileBytes(),
		MaxFiles:         fileLimit,
		MaxObservedBytes: defaultBudget.MaxObservedBytes(),
		MaxParseWorkers:  defaultBudget.MaxParseWorkers(),
		GeneratedSources: defaultBudget.GeneratedSources(),
	})
	if err != nil {
		return nil, err
	}
	files := make([]FileSymbols, 0)
	usage := EmptyAdmissionUsage()
	err = walkProjectFiles(projectRoot, func(
		path string,
		relPath string,
		_ os.DirEntry,
	) error {
		if !registry.SupportsSymbols(path) {
			return nil // unsupported language
		}
		admission, nextUsage, err := registry.ReadSourceAdmission(
			projectRoot,
			relPath,
			budget,
			usage,
		)
		if err != nil {
			return err
		}
		usage = nextUsage
		if admission.Kind().String() == "source_skipped" {
			info, err := SkippedSourceInfo(admission)
			if err != nil {
				return err
			}
			if info.Reason == "root_file_budget" {
				return filepath.SkipAll
			}
			if info.RequiresRetry() {
				return fmt.Errorf(
					"observe repository-map source %s: %s",
					relPath,
					info.Detail,
				)
			}
			return nil
		}
		source, err := AdmittedSourceFrom(admission)
		if err != nil {
			return err
		}
		fileSymbols, err := extractRepoMapFile(
			registry,
			path,
			source,
		)
		if err != nil {
			return err
		}
		files = append(files, fileSymbols)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk project: %w", err)
	}

	// Sort by path for stable output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	totalSyms := 0
	for _, f := range files {
		totalSyms += len(f.Symbols)
	}

	return &RepoMap{
		Files:      files,
		TotalFiles: len(files),
		TotalSyms:  totalSyms,
	}, nil
}

// extractRepoMapFile projects the canonical SymbolSnapshot output into the
// lightweight repository-map shape. RepoMap no longer owns a second language
// query set that can disagree with the graph, drift, or binding paths.
func extractRepoMapFile(
	registry *Registry,
	absPath string,
	source AdmittedSource,
) (FileSymbols, error) {
	snapshots, err := registry.ExtractAdmittedSymbolSnapshots(source)
	if err != nil {
		return FileSymbols{}, err
	}
	relPath := source.Path().String()
	content := source.bytes()
	lines := strings.Count(string(content), "\n") + 1
	symbols := make([]Symbol, 0, len(snapshots))
	for _, snapshot := range snapshots {
		symbols = append(symbols, Symbol{
			Name:     snapshot.SymbolName,
			Kind:     snapshot.SymbolKind,
			Line:     snapshot.Line,
			Exported: snapshot.Exported,
		})
	}

	// Sort by line number
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	// Deduplicate (same name+line from overlapping queries)
	symbols = dedup(symbols)

	// Get modification time for recency sorting
	var modTime int64
	if info, err := os.Stat(absPath); err == nil {
		modTime = info.ModTime().UnixNano()
	}

	return FileSymbols{
		Path:     relPath,
		Language: source.Language().String(),
		Lines:    lines,
		Symbols:  symbols,
		ModTime:  modTime,
	}, nil
}

func dedup(syms []Symbol) []Symbol {
	if len(syms) <= 1 {
		return syms
	}
	result := []Symbol{syms[0]}
	for _, s := range syms[1:] {
		last := result[len(result)-1]
		if s.Name == last.Name && s.Line == last.Line {
			continue
		}
		result = append(result, s)
	}
	return result
}

// RenderRepoMap formats the repo map for injection into the system prompt.
// Shows directory tree with exported symbols, kinds, and line counts.
// Files sorted by modification time (recent first) for relevance.
// Respects a token budget (approximate: 4 chars ≈ 1 token).
func RenderRepoMap(rm *RepoMap, maxTokens int) string {
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	maxChars := maxTokens * 4

	var b strings.Builder
	b.WriteString("## Repository map\n\n")

	// Sort files: recently modified first within each directory
	files := make([]FileSymbols, len(rm.Files))
	copy(files, rm.Files)
	sort.Slice(files, func(i, j int) bool {
		di, dj := filepath.Dir(files[i].Path), filepath.Dir(files[j].Path)
		if di != dj {
			return di < dj
		}
		// Within same dir, recent files first
		return files[i].ModTime > files[j].ModTime
	})

	currentDir := ""
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		if dir != currentDir {
			if currentDir != "" {
				b.WriteString("\n")
			}
			b.WriteString(dir + "/\n")
			currentDir = dir
		}

		if len(f.Symbols) == 0 {
			b.WriteString(fmt.Sprintf("  %s (%d lines)\n", filepath.Base(f.Path), f.Lines))
		} else {
			// Show symbols with kinds for better context
			parts := make([]string, 0, len(f.Symbols))
			for _, s := range f.Symbols {
				if s.Exported {
					if s.Kind != "" && s.Kind != "func" {
						// Show kind for non-functions (types, interfaces, classes)
						parts = append(parts, s.Kind+" "+s.Name)
					} else {
						parts = append(parts, s.Name)
					}
				}
			}
			if len(parts) == 0 {
				b.WriteString(fmt.Sprintf("  %s (%d lines)\n", filepath.Base(f.Path), f.Lines))
			} else {
				b.WriteString(fmt.Sprintf("  %s: %s\n", filepath.Base(f.Path), strings.Join(parts, ", ")))
			}
		}

		if b.Len() > maxChars {
			remaining := rm.TotalFiles - len(files)
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("\n... (%d more files)\n", remaining))
			}
			break
		}
	}

	return b.String()
}
