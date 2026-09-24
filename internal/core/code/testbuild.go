package code

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/scanner"
	"time"
)

const TestBuildVersion = "haft.go-test-build/1"

// TestBuild is the local input closure of one Go test binary. Navigation's
// package graph is deliberately separate: only the selected directory has test
// variants, and its external test package is part of that same binary.
type TestBuild struct {
	Package         string
	Files           []string
	ExternalImports []string
}

// GoTestBuild performs no IO or execution. go/build reads only the captured
// virtual filesystem. A supported result pins local build inputs; standard and
// external module implementation bytes still require the declared toolchain and
// module basis. Unsupported composition returns an error, never an exact subset.
func (index Index) GoTestBuild(oraclePath string) (TestBuild, error) {
	var result TestBuild
	if _, err := GoTestEnvironment(index.Config); err != nil {
		return result, err
	}
	if !index.Complete || !index.Config.IncludeTests || index.Config.CGOEnabled || len(index.Config.ToolTags) != 0 {
		return result, fmt.Errorf("test build requires complete capture, tests enabled, and no cgo/custom tool tags")
	}
	files := map[string]File{}
	for _, f := range index.Files {
		files[f.Path] = f
		if f.Path == "go.work" || strings.HasPrefix(f.Path, "vendor/") || strings.Contains(f.Path, "/vendor/") {
			return result, fmt.Errorf("workspace/vendor test-build resolution is unsupported: %s", f.Path)
		}
	}
	module := modulePath(files["go.mod"].Raw)
	if module == "" || regexp.MustCompile(`(?m)^\s*replace\b`).Match(files["go.mod"].Raw) {
		return result, fmt.Errorf("one captured module without replacements is required for local test-build resolution")
	}
	for _, d := range index.Diagnostics {
		if strings.Contains(d.Code, "partial") || d.Code == "unresolved_local_import" {
			return result, fmt.Errorf("local test-build capture is incomplete: %s at %s", d.Code, d.Path)
		}
	}
	selection, err := capturedBuildContext(files, index.Config)
	if err != nil {
		return result, err
	}
	root := path.Dir(oraclePath)
	result.Package = module
	if root != "." {
		result.Package += "/" + root
	}
	seen, included, external := map[string]bool{}, map[string]bool{}, map[string]bool{}
	queue := []string{root}
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		if seen[dir] {
			continue
		}
		seen[dir] = true
		pkg, err := selection.ImportDir(path.Join(capturedBuildRoot, dir), 0)
		if err != nil {
			return result, fmt.Errorf("local test-build package %s: %w", dir, err)
		}
		// Default experiment/architecture-feature tags are compiler configuration,
		// not ordinary -tags. Their selection needs a separately supported adapter.
		for _, tag := range pkg.AllTags {
			prefix, _, dotted := strings.Cut(tag, ".")
			if dotted && (prefix == "goexperiment" || featureArchitecture(prefix)) || tag == "boringcrypto" {
				return result, fmt.Errorf("compiler feature build tag %s requires a separate test-build adapter", tag)
			}
		}
		if len(pkg.CgoFiles)+len(pkg.CFiles)+len(pkg.CXXFiles)+len(pkg.MFiles)+len(pkg.FFiles)+len(pkg.SwigFiles)+len(pkg.SwigCXXFiles) != 0 || len(pkg.EmbedPatterns)+len(pkg.TestEmbedPatterns)+len(pkg.XTestEmbedPatterns) != 0 {
			return result, fmt.Errorf("cgo/foreign source/embed test-build inputs are unsupported in %s", dir)
		}
		selected := append([]string{}, pkg.GoFiles...)
		imports := append([]string{}, pkg.Imports...)
		if dir == root {
			selected = append(selected, pkg.TestGoFiles...)
			selected = append(selected, pkg.XTestGoFiles...)
			imports = append(imports, pkg.TestImports...)
			imports = append(imports, pkg.XTestImports...)
			found := false
			for _, name := range append(append([]string{}, pkg.TestGoFiles...), pkg.XTestGoFiles...) {
				found = found || path.Join(dir, name) == oraclePath
			}
			if !found {
				return result, fmt.Errorf("oracle is not in the selected test-build variant")
			}
		}
		selected = append(selected, pkg.SFiles...)
		selected = append(selected, pkg.HFiles...)
		selected = append(selected, pkg.SysoFiles...)
		for _, name := range selected {
			p := path.Join(dir, name)
			f, ok := files[p]
			if !ok || f.Reason == "ignore_rule" || f.Status == "unresolved" || f.Status == "unsupported" {
				return result, fmt.Errorf("selected local test-build input unavailable: %s", p)
			}
			included[p] = true
		}
		for _, imported := range imports {
			if imported == module {
				queue = append(queue, ".")
			} else if strings.HasPrefix(imported, module+"/") {
				local := strings.TrimPrefix(imported, module+"/")
				if !normalizedPath(local) {
					return result, fmt.Errorf("invalid local import: %s", imported)
				}
				queue = append(queue, local)
			} else {
				external[imported] = true
			}
		}
	}
	// The assembler also reads local include files. Restrict this adapter to
	// literal same-directory includes, plus Go's named toolchain/generated headers.
	includes := []string{}
	for p := range included {
		if path.Ext(p) == ".s" || isHeader(p) {
			includes = append(includes, p)
		}
	}
	checked := map[string]bool{}
	for len(includes) > 0 {
		p := includes[0]
		includes = includes[1:]
		if checked[p] {
			continue
		}
		checked[p] = true
		names, err := assemblyIncludes(files[p].Raw)
		if err != nil {
			return result, fmt.Errorf("unresolved assembler include in %s: %w", p, err)
		}
		for _, name := range names {
			local := path.Join(path.Dir(p), name)
			if f, ok := files[local]; ok {
				if f.Reason == "ignore_rule" || f.Status == "unresolved" || f.Status == "unsupported" {
					return result, fmt.Errorf("assembler include unavailable: %s", local)
				}
				included[local] = true
				includes = append(includes, local)
			} else if !toolchainAssemblyHeader(name) {
				return result, fmt.Errorf("uncaptured assembler include %s in %s", name, p)
			}
		}
	}
	for p := range included {
		result.Files = append(result.Files, p)
	}
	for imported := range external {
		result.ExternalImports = append(result.ExternalImports, imported)
	}
	sort.Strings(result.Files)
	sort.Strings(result.ExternalImports)
	return result, nil
}

func assemblyIncludes(raw []byte) ([]string, error) {
	var lexer scanner.Scanner
	lexer.Init(bytes.NewReader(raw))
	lexer.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
	var scanErr error
	lexer.Error = func(_ *scanner.Scanner, message string) { scanErr = fmt.Errorf("%s", message) }
	names := []string{}
	for tok := lexer.Scan(); tok != scanner.EOF; tok = lexer.Scan() {
		if tok != '#' || lexer.Scan() != scanner.Ident || lexer.TokenText() != "include" {
			continue
		}
		if lexer.Scan() != scanner.String {
			return nil, fmt.Errorf("include must use one literal local filename")
		}
		name, err := strconv.Unquote(lexer.TokenText())
		if err != nil || !normalizedPath(name) || path.Base(name) != name {
			return nil, fmt.Errorf("include must stay in its captured package: %s", lexer.TokenText())
		}
		names = append(names, name)
	}
	return names, scanErr
}

func toolchainAssemblyHeader(name string) bool {
	switch name {
	case "textflag.h", "funcdata.h", "asm_amd64.h", "asm_ppc64x.h", "go_asm.h":
		return true
	}
	return false
}
func featureArchitecture(name string) bool {
	switch name {
	case "386", "amd64", "arm", "arm64", "mips", "mipsle", "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "wasm":
		return true
	}
	return false
}
func isHeader(p string) bool {
	switch path.Ext(p) {
	case ".h", ".hh", ".hpp", ".hxx":
		return true
	}
	return false
}
func isGoBuildInput(p string) bool {
	switch path.Ext(p) {
	case ".go", ".s", ".S", ".syso", ".c", ".cc", ".cpp", ".cxx", ".m", ".f", ".F", ".for", ".f90", ".swig", ".swigcxx":
		return true
	}
	return isHeader(p)
}

const capturedBuildRoot = "/haft-captured"

type capturedFileInfo struct {
	name string
	size int64
	dir  bool
}

func (f capturedFileInfo) Name() string { return f.name }
func (f capturedFileInfo) Size() int64  { return f.size }
func (f capturedFileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0700
	}
	return 0600
}
func (f capturedFileInfo) ModTime() time.Time { return time.Time{} }
func (f capturedFileInfo) IsDir() bool        { return f.dir }
func (f capturedFileInfo) Sys() any           { return nil }

func capturedBuildContext(files map[string]File, config Config) (build.Context, error) {
	match := toolchainPattern.FindStringSubmatch(config.Toolchain)
	if match == nil {
		return build.Context{}, fmt.Errorf("release toolchain required")
	}
	minor, _ := strconv.Atoi(match[1])
	selection := build.Context{GOOS: config.GOOS, GOARCH: config.GOARCH, Compiler: "gc", BuildTags: config.BuildTags}
	for n := 1; n <= minor; n++ {
		selection.ReleaseTags = append(selection.ReleaseTags, fmt.Sprintf("go1.%d", n))
	}
	dirs := map[string]map[string]fs.FileInfo{capturedBuildRoot: {}}
	for p, f := range files {
		full := path.Join(capturedBuildRoot, p)
		for child := full; child != capturedBuildRoot; child = path.Dir(child) {
			parent := path.Dir(child)
			if dirs[parent] == nil {
				dirs[parent] = map[string]fs.FileInfo{}
			}
			dirs[parent][path.Base(child)] = capturedFileInfo{path.Base(child), int64(len(f.Raw)), child != full}
		}
	}
	selection.JoinPath = path.Join
	selection.IsAbsPath = path.IsAbs
	selection.IsDir = func(p string) bool { _, ok := dirs[path.Clean(p)]; return ok }
	selection.ReadDir = func(p string) ([]fs.FileInfo, error) {
		entries, ok := dirs[path.Clean(p)]
		if !ok {
			return nil, fs.ErrNotExist
		}
		result := []fs.FileInfo{}
		for _, f := range entries {
			result = append(result, f)
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
		return result, nil
	}
	selection.OpenFile = func(p string) (io.ReadCloser, error) {
		name := strings.TrimPrefix(path.Clean(p), capturedBuildRoot+"/")
		f, ok := files[name]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return io.NopCloser(bytes.NewReader(f.Raw)), nil
	}
	return selection, nil
}
