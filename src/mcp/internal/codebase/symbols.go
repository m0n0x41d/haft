package codebase

import (
	gocontext "context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Symbol represents an extracted code symbol (function, type, class, etc.)
type Symbol struct {
	Name     string // symbol name
	Kind     string // "func", "type", "interface", "class", "method", "const"
	Line     int    // 1-based line number
	Exported bool   // starts with uppercase (Go) or is exported
}

// FileSymbols holds symbols extracted from a single file.
type FileSymbols struct {
	Path     string   // relative path from project root
	Language string   // "go", "python", "javascript", "typescript", "rust", "c", "cpp"
	Lines    int      // total line count
	Symbols  []Symbol // extracted symbols, sorted by line
}

// RepoMap is the complete symbol map for a repository.
type RepoMap struct {
	Files      []FileSymbols
	TotalFiles int
	TotalSyms  int
}

// languageInfo maps file extensions to tree-sitter languages and query patterns.
type languageInfo struct {
	name    string
	lang    *sitter.Language
	queries []queryPattern
}

type queryPattern struct {
	pattern string
	kind    string
}

var languages = map[string]*languageInfo{
	".go": {
		name: "go",
		lang: golang.GetLanguage(),
		queries: []queryPattern{
			{"(function_declaration name: (identifier) @name)", "func"},
			{"(method_declaration name: (field_identifier) @name)", "method"},
			{"(type_declaration (type_spec name: (type_identifier) @name))", "type"},
		},
	},
	".py": {
		name: "python",
		lang: python.GetLanguage(),
		queries: []queryPattern{
			{"(function_definition name: (identifier) @name)", "func"},
			{"(class_definition name: (identifier) @name)", "class"},
		},
	},
	".js": {
		name: "javascript",
		lang: javascript.GetLanguage(),
		queries: []queryPattern{
			{"(function_declaration name: (identifier) @name)", "func"},
			{"(class_declaration name: (identifier) @name)", "class"},
			{"(method_definition name: (property_identifier) @name)", "method"},
			{"(export_statement declaration: (function_declaration name: (identifier) @name))", "func"},
		},
	},
	".jsx": {
		name: "javascript",
		lang: javascript.GetLanguage(),
		queries: []queryPattern{
			{"(function_declaration name: (identifier) @name)", "func"},
			{"(class_declaration name: (identifier) @name)", "class"},
		},
	},
	".ts": {
		name: "typescript",
		lang: typescript.GetLanguage(),
		queries: []queryPattern{
			{"(function_declaration name: (identifier) @name)", "func"},
			{"(class_declaration name: (type_identifier) @name)", "class"},
			{"(interface_declaration name: (type_identifier) @name)", "interface"},
			{"(method_definition name: (property_identifier) @name)", "method"},
		},
	},
	".tsx": {
		name: "typescript",
		lang: typescript.GetLanguage(),
		queries: []queryPattern{
			{"(function_declaration name: (identifier) @name)", "func"},
			{"(class_declaration name: (type_identifier) @name)", "class"},
			{"(interface_declaration name: (type_identifier) @name)", "interface"},
		},
	},
	".rs": {
		name: "rust",
		lang: rust.GetLanguage(),
		queries: []queryPattern{
			{"(function_item name: (identifier) @name)", "func"},
			{"(struct_item name: (type_identifier) @name)", "type"},
			{"(enum_item name: (type_identifier) @name)", "type"},
			{"(trait_item name: (type_identifier) @name)", "interface"},
			{"(impl_item type: (type_identifier) @name)", "type"},
		},
	},
	".c": {
		name: "c",
		lang: c.GetLanguage(),
		queries: []queryPattern{
			{"(function_definition declarator: (function_declarator declarator: (identifier) @name))", "func"},
			{"(struct_specifier name: (type_identifier) @name)", "type"},
			{"(enum_specifier name: (type_identifier) @name)", "type"},
		},
	},
	".h": {
		name: "c",
		lang: c.GetLanguage(),
		queries: []queryPattern{
			{"(function_definition declarator: (function_declarator declarator: (identifier) @name))", "func"},
			{"(declaration declarator: (function_declarator declarator: (identifier) @name))", "func"},
			{"(struct_specifier name: (type_identifier) @name)", "type"},
		},
	},
	".cpp": {
		name: "cpp",
		lang: cpp.GetLanguage(),
		queries: []queryPattern{
			{"(function_definition declarator: (function_declarator declarator: (identifier) @name))", "func"},
			{"(class_specifier name: (type_identifier) @name)", "class"},
			{"(struct_specifier name: (type_identifier) @name)", "type"},
		},
	},
}

// BuildRepoMap scans the project and extracts symbols from all supported files.
func BuildRepoMap(projectRoot string, maxFiles int) (*RepoMap, error) {
	if maxFiles <= 0 {
		maxFiles = 500
	}

	var files []FileSymbols
	parser := sitter.NewParser()

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
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}

		ext := filepath.Ext(d.Name())
		langInfo, ok := languages[ext]
		if !ok {
			return nil // unsupported language
		}

		relPath, _ := filepath.Rel(projectRoot, path)
		fs, err := extractFileSymbols(parser, path, relPath, langInfo)
		if err != nil {
			return nil // skip unparseable files
		}
		if fs != nil {
			files = append(files, *fs)
		}
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

// extractFileSymbols parses one file and extracts symbols using tree-sitter queries.
func extractFileSymbols(parser *sitter.Parser, absPath, relPath string, langInfo *languageInfo) (*FileSymbols, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Skip very large files (>100KB)
	if len(content) > 100_000 {
		lines := strings.Count(string(content), "\n") + 1
		return &FileSymbols{
			Path:     relPath,
			Language: langInfo.name,
			Lines:    lines,
		}, nil
	}

	parser.SetLanguage(langInfo.lang)
	tree, err := parser.ParseCtx(gocontext.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	lines := strings.Count(string(content), "\n") + 1
	var symbols []Symbol

	for _, qp := range langInfo.queries {
		q, err := sitter.NewQuery([]byte(qp.pattern), langInfo.lang)
		if err != nil {
			continue // skip invalid queries
		}

		qc := sitter.NewQueryCursor()
		qc.Exec(q, tree.RootNode())

		for {
			match, ok := qc.NextMatch()
			if !ok {
				break
			}
			for _, capture := range match.Captures {
				name := capture.Node.Content(content)
				line := int(capture.Node.StartPoint().Row) + 1 // 0-based → 1-based

				exported := false
				if langInfo.name == "go" {
					exported = len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
				} else {
					exported = !strings.HasPrefix(name, "_")
				}

				symbols = append(symbols, Symbol{
					Name:     name,
					Kind:     qp.kind,
					Line:     line,
					Exported: exported,
				})
			}
		}
		q.Close()
	}

	// Sort by line number
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	// Deduplicate (same name+line from overlapping queries)
	symbols = dedup(symbols)

	return &FileSymbols{
		Path:     relPath,
		Language: langInfo.name,
		Lines:    lines,
		Symbols:  symbols,
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
// Respects a token budget (approximate: 4 chars ≈ 1 token).
func RenderRepoMap(rm *RepoMap, maxTokens int) string {
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	maxChars := maxTokens * 4

	var b strings.Builder
	b.WriteString("## Repository map\n\n")

	currentDir := ""
	for _, f := range rm.Files {
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
			names := make([]string, 0, len(f.Symbols))
			for _, s := range f.Symbols {
				if s.Exported {
					names = append(names, s.Name)
				}
			}
			if len(names) == 0 {
				// No exported symbols — show file only
				b.WriteString(fmt.Sprintf("  %s (%d lines)\n", filepath.Base(f.Path), f.Lines))
			} else {
				b.WriteString(fmt.Sprintf("  %s: %s\n", filepath.Base(f.Path), strings.Join(names, ", ")))
			}
		}

		// Check budget
		if b.Len() > maxChars {
			b.WriteString(fmt.Sprintf("\n... (%d more files)\n", rm.TotalFiles-len(rm.Files)))
			break
		}
	}

	return b.String()
}
