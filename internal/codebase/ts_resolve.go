package codebase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// tsAliasPattern is one `compilerOptions.paths` entry: a prefix/suffix around an
// optional `*` wildcard, with the baseUrl-relative replacement templates.
type tsAliasPattern struct {
	prefix       string
	suffix       string
	wildcard     bool
	replacements []string // project-relative templates; `*` receives the capture
}

type tsWorkspacePackage struct {
	dir     string
	entries map[string]string // "." / "./subpath" -> project-relative module base
}

// tsProjectResolution is the project-level rewrite surface for non-relative module
// specifiers: tsconfig path aliases (`@/x` → `src/x`) and monorepo workspace
// packages (`@scope/ui/w` → `packages/ui/w`). Both turn a specifier that LOOKS
// external into a project-relative base the symbol resolver can match.
type tsProjectResolution struct {
	aliases    []tsAliasPattern
	workspaces map[string]tsWorkspacePackage // package name -> package resolution surface
}

// tsResolutionCache memoizes the project resolution per project root. Loading
// parses tsconfig + walks workspace dirs, so re-running it for every file in a
// scan would be wasteful. The entry is invalidated when a root config file's
// mtime changes, so a mid-session edit to tsconfig/package.json takes effect on
// the next resolve without a restart. (A member package.json edit is not yet
// fingerprinted — that needs a re-index.)
var tsResolutionCache sync.Map // projectRoot -> tsResolutionEntry

type tsResolutionEntry struct {
	fingerprint int64
	res         tsProjectResolution
}

// tsConfigFingerprint covers local JSON config inheritance, workspace package
// manifests, and pnpm workspace declarations. It is stat-only and deterministic.
func tsConfigFingerprint(projectRoot string) int64 {
	var fp int64
	_ = walkProjectFiles(projectRoot, func(_ string, _ string, entry os.DirEntry) error {
		name := entry.Name()
		if filepath.Ext(name) != ".json" && name != "pnpm-workspace.yaml" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		fp = fp*131 + info.ModTime().UnixNano()
		fp = fp*131 + info.Size()
		return nil
	})
	return fp
}

func loadTSProjectResolution(projectRoot string) tsProjectResolution {
	fp := tsConfigFingerprint(projectRoot)
	if v, ok := tsResolutionCache.Load(projectRoot); ok {
		if e := v.(tsResolutionEntry); e.fingerprint == fp {
			return e.res
		}
	}
	res := buildTSProjectResolution(projectRoot)
	tsResolutionCache.Store(projectRoot, tsResolutionEntry{fingerprint: fp, res: res})
	return res
}

func buildTSProjectResolution(projectRoot string) tsProjectResolution {
	res := tsProjectResolution{workspaces: map[string]tsWorkspacePackage{}}
	res.aliases = loadTSAliases(projectRoot)
	res.workspaces = loadTSWorkspaces(projectRoot)
	return res
}

// loadTSAliases reads tsconfig.json (then jsconfig.json), follows local extends
// chains, and returns all baseUrl-relative target templates most-specific first.
// Package-based config presets and bundler-only aliases remain outside this port.
func loadTSAliases(projectRoot string) []tsAliasPattern {
	canonicalRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil
	}
	for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
		path := filepath.Join(canonicalRoot, name)
		if _, err := os.Stat(path); err == nil {
			return loadTSAliasConfig(canonicalRoot, path, map[string]bool{})
		}
	}
	return nil
}

func loadTSAliasConfig(projectRoot, configPath string, visiting map[string]bool) []tsAliasPattern {
	canonical, err := filepath.Abs(configPath)
	if err != nil || visiting[canonical] {
		return nil
	}
	visiting[canonical] = true
	defer delete(visiting, canonical)
	raw, err := os.ReadFile(canonical)
	if err != nil {
		return nil
	}
	var cfg struct {
		Extends         string `json:"extends"`
		CompilerOptions struct {
			BaseURL string              `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	if err := json.Unmarshal([]byte(stripJSONC(string(raw))), &cfg); err != nil {
		return nil
	}
	patterns := loadTSExtendedAliases(projectRoot, filepath.Dir(canonical), cfg.Extends, visiting)
	baseURL := cfg.CompilerOptions.BaseURL
	if baseURL == "" {
		baseURL = "."
	}
	configDir, err := filepath.Rel(projectRoot, filepath.Dir(canonical))
	if err != nil {
		return patterns
	}
	for key, targets := range cfg.CompilerOptions.Paths {
		if len(targets) == 0 {
			continue
		}
		prefix, suffix, wildcard := splitAliasKey(key)
		replacements := make([]string, 0, len(targets))
		for _, target := range targets {
			resolved := filepath.Join(configDir, baseURL, target)
			replacements = append(replacements, strings.TrimPrefix(filepath.ToSlash(resolved), "./"))
		}
		patterns = append(patterns, tsAliasPattern{
			prefix:       prefix,
			suffix:       suffix,
			wildcard:     wildcard,
			replacements: replacements,
		})
	}
	// Most specific first: longer prefix wins, then literal (non-wildcard) before
	// wildcard so an exact alias is preferred over a `*` match.
	sort.SliceStable(patterns, func(i, j int) bool {
		if len(patterns[i].prefix) != len(patterns[j].prefix) {
			return len(patterns[i].prefix) > len(patterns[j].prefix)
		}
		return !patterns[i].wildcard && patterns[j].wildcard
	})
	return patterns
}

func loadTSExtendedAliases(projectRoot, configDir, extended string, visiting map[string]bool) []tsAliasPattern {
	extended = strings.TrimSpace(extended)
	if extended == "" || (!strings.HasPrefix(extended, ".") && !filepath.IsAbs(extended)) {
		return nil
	}
	path := extended
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}
	if filepath.Ext(path) == "" {
		path += ".json"
	}
	return loadTSAliasConfig(projectRoot, path, visiting)
}

// splitAliasKey breaks a `paths` key around its single `*` wildcard.
func splitAliasKey(key string) (prefix, suffix string, wildcard bool) {
	if i := strings.IndexByte(key, '*'); i >= 0 {
		return key[:i], key[i+1:], true
	}
	return key, "", false
}

// loadTSWorkspaces maps each monorepo member package's declared name to its
// project-relative directory, from `package.json` `workspaces` (array or
// `{packages:[…]}`). One level of trailing `/*` glob is expanded. Returns an
// empty map for single-package repos (the common case pays nothing).
func loadTSWorkspaces(projectRoot string) map[string]tsWorkspacePackage {
	out := map[string]tsWorkspacePackage{}
	raw, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	globs := make([]string, 0)
	if err == nil {
		var root struct {
			Workspaces json.RawMessage `json:"workspaces"`
		}
		if json.Unmarshal([]byte(stripJSONC(string(raw))), &root) == nil && root.Workspaces != nil {
			globs = append(globs, parseWorkspaceGlobs(root.Workspaces)...)
		}
	}
	globs = append(globs, loadPNPMWorkspaceGlobs(projectRoot)...)
	for _, g := range uniqueStrings(globs) {
		for _, dir := range expandWorkspaceGlob(projectRoot, g) {
			workspace := readWorkspacePackage(projectRoot, dir)
			if workspace.name == "" {
				continue
			}
			out[workspace.name] = tsWorkspacePackage{dir: filepath.ToSlash(dir), entries: workspace.entries}
		}
	}
	return out
}

func loadPNPMWorkspaceGlobs(projectRoot string) []string {
	raw, err := os.ReadFile(filepath.Join(projectRoot, "pnpm-workspace.yaml"))
	if err != nil {
		return nil
	}
	var workspace struct {
		Packages []string `yaml:"packages"`
	}
	if yaml.Unmarshal(raw, &workspace) != nil {
		return nil
	}
	return workspace.Packages
}

// parseWorkspaceGlobs accepts both the array form (`["packages/*"]`) and the
// object form (`{"packages": ["packages/*"]}`).
func parseWorkspaceGlobs(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Packages
	}
	return nil
}

// expandWorkspaceGlob resolves one workspace glob to member directories. A
// trailing `/*` lists immediate subdirectories; any other entry is treated as a
// literal directory. `**` and deeper globs are out of scope for v1.
func expandWorkspaceGlob(projectRoot, glob string) []string {
	glob = strings.TrimSuffix(glob, "/")
	if !strings.HasSuffix(glob, "/*") {
		return []string{glob}
	}
	base := strings.TrimSuffix(glob, "/*")
	entries, err := os.ReadDir(filepath.Join(projectRoot, base))
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(base, e.Name()))
		}
	}
	return dirs
}

type tsWorkspaceDescriptor struct {
	name    string
	entries map[string]string
}

func readWorkspacePackage(projectRoot, relDir string) tsWorkspaceDescriptor {
	raw, err := os.ReadFile(filepath.Join(projectRoot, relDir, "package.json"))
	if err != nil {
		return tsWorkspaceDescriptor{}
	}
	var pkg struct {
		Name    string          `json:"name"`
		Exports json.RawMessage `json:"exports"`
		Types   string          `json:"types"`
		Module  string          `json:"module"`
		Main    string          `json:"main"`
	}
	if err := json.Unmarshal([]byte(stripJSONC(string(raw))), &pkg); err != nil {
		return tsWorkspaceDescriptor{}
	}
	entries := workspaceExportEntries(relDir, pkg.Exports)
	if _, ok := entries["."]; !ok {
		entry := firstNonEmptyString(pkg.Types, pkg.Module, pkg.Main, "index")
		entries["."] = moduleBase(filepath.Join(relDir, entry))
	}
	return tsWorkspaceDescriptor{name: pkg.Name, entries: entries}
}

func workspaceExportEntries(relDir string, raw json.RawMessage) map[string]string {
	entries := map[string]string{}
	if len(raw) == 0 {
		return entries
	}
	var direct string
	if json.Unmarshal(raw, &direct) == nil {
		entries["."] = moduleBase(filepath.Join(relDir, direct))
		return entries
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return entries
	}
	hasSubpaths := false
	for key := range object {
		if strings.HasPrefix(key, ".") {
			hasSubpaths = true
		}
	}
	if !hasSubpaths {
		if target := workspaceExportTarget(raw); target != "" {
			entries["."] = moduleBase(filepath.Join(relDir, target))
		}
		return entries
	}
	for key, value := range object {
		target := workspaceExportTarget(value)
		if target == "" {
			continue
		}
		entries[key] = moduleBase(filepath.Join(relDir, target))
	}
	return entries
}

func workspaceExportTarget(raw json.RawMessage) string {
	var direct string
	if json.Unmarshal(raw, &direct) == nil {
		return direct
	}
	var sequence []json.RawMessage
	if json.Unmarshal(raw, &sequence) == nil {
		for _, item := range sequence {
			if target := workspaceExportTarget(item); target != "" {
				return target
			}
		}
		return ""
	}
	var conditions map[string]json.RawMessage
	if json.Unmarshal(raw, &conditions) != nil {
		return ""
	}
	for _, condition := range []string{"types", "import", "module", "default", "require"} {
		value, ok := conditions[condition]
		if !ok {
			continue
		}
		if target := workspaceExportTarget(value); target != "" {
			return target
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func moduleBase(path string) string {
	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	for _, declarationExtension := range []string{".d.ts", ".d.mts", ".d.cts"} {
		if strings.HasSuffix(normalized, declarationExtension) {
			return strings.TrimSuffix(normalized, declarationExtension)
		}
	}
	extension := filepath.Ext(normalized)
	switch extension {
	case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs":
		return strings.TrimSuffix(normalized, extension)
	default:
		return normalized
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func resolveTSModuleSpecifiers(raw, fileDir string, res tsProjectResolution) ([]string, bool) {
	if strings.HasPrefix(raw, ".") {
		return []string{moduleBase(filepath.Join(fileDir, raw))}, true
	}
	if bases := resolveAlias(raw, res); len(bases) > 0 {
		return bases, true
	}
	if base, ok := resolveWorkspace(raw, res.workspaces); ok {
		return []string{base}, true
	}
	return nil, false
}

func resolveAlias(raw string, res tsProjectResolution) []string {
	for _, p := range res.aliases {
		captured, ok := matchAlias(raw, p)
		if !ok {
			continue
		}
		bases := make([]string, 0, len(p.replacements))
		for _, replacement := range p.replacements {
			resolved := strings.ReplaceAll(replacement, "*", captured)
			bases = append(bases, moduleBase(resolved))
		}
		return uniqueStrings(bases)
	}
	return nil
}

// matchAlias reports whether raw matches the pattern and returns the `*` capture.
func matchAlias(raw string, p tsAliasPattern) (string, bool) {
	if !p.wildcard {
		return "", raw == p.prefix
	}
	if !strings.HasPrefix(raw, p.prefix) || !strings.HasSuffix(raw, p.suffix) {
		return "", false
	}
	if len(raw) < len(p.prefix)+len(p.suffix) {
		return "", false
	}
	return raw[len(p.prefix) : len(raw)-len(p.suffix)], true
}

// resolveWorkspace rewrites `@scope/pkg/sub` to `<member dir>/sub` using the
// longest matching package name.
func resolveWorkspace(raw string, workspaces map[string]tsWorkspacePackage) (string, bool) {
	bestName := ""
	bestWorkspace := tsWorkspacePackage{}
	for name, workspace := range workspaces {
		if raw != name && !strings.HasPrefix(raw, name+"/") {
			continue
		}
		if len(name) > len(bestName) {
			bestName, bestWorkspace = name, workspace
		}
	}
	if bestName == "" {
		return "", false
	}
	sub := strings.TrimPrefix(raw[len(bestName):], "/")
	key := "."
	if sub != "" {
		key = "./" + sub
	}
	entry, ok := bestWorkspace.entries[key]
	if ok {
		return entry, true
	}
	if sub == "" {
		return moduleBase(bestWorkspace.dir), true
	}
	return moduleBase(filepath.Join(bestWorkspace.dir, sub)), true
}

// stripJSONC removes `//` and `/* */` comments and trailing commas so a tsconfig
// or package.json carrying the usual editor annotations parses as plain JSON. A
// string-aware single pass — comment markers and commas inside string literals
// are preserved.
func stripJSONC(src string) string {
	out := make([]byte, 0, len(src))
	inString, escaped := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			i++ // land on '/', loop's i++ steps past it
		case c == '}' || c == ']':
			out = dropTrailingComma(out)
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// dropTrailingComma trims trailing whitespace and a single trailing comma from the
// emitted buffer — called when a `}`/`]` closer is reached.
func dropTrailingComma(out []byte) []byte {
	j := len(out)
	for j > 0 && (out[j-1] == ' ' || out[j-1] == '\t' || out[j-1] == '\n' || out[j-1] == '\r') {
		j--
	}
	if j > 0 && out[j-1] == ',' {
		return out[:j-1]
	}
	return out
}
