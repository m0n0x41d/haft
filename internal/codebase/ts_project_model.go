package codebase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type tsExportTarget struct {
	fileBase   string
	symbolName string
}

type tsReexportTarget struct {
	moduleBase string
	exportName string
}

// tsProjectModel is the project-level module/export core shared by import and
// edge resolution. It is immutable after construction; callers only traverse
// its cycle-safe export graph.
type tsProjectModel struct {
	modules   map[string]bool
	direct    map[string]map[string][]tsExportTarget
	reexports map[string]map[string][]tsReexportTarget
	stars     map[string][]string
}

type tsProjectModelEntry struct {
	fingerprint string
	model       *tsProjectModel
}

var tsProjectModelCache sync.Map

// tsProjectSnapshot is the immutable TypeScript project context shared by all
// TS/JS/Vue resolvers in one index epoch. Project discovery and export-model
// construction happen once before per-file resolution begins.
type tsProjectSnapshot struct {
	resolution tsProjectResolution
	model      *tsProjectModel
}

// projectIndexSnapshot is the language-neutral epoch carrier consumed by
// prepared resolvers. Additional language-specific project snapshots can be
// added without changing the scanner's per-file loop.
type projectIndexSnapshot struct {
	typescript *tsProjectSnapshot
	sources    map[string]AdmittedSource
}

func newProjectIndexSnapshot(
	sources map[string]AdmittedSource,
	resolution tsProjectResolution,
) *projectIndexSnapshot {
	model := buildTSProjectModelFromAdmittedSources(sources, resolution)
	typescript := &tsProjectSnapshot{resolution: resolution, model: model}
	return &projectIndexSnapshot{
		typescript: typescript,
		sources:    cloneAdmittedSources(sources),
	}
}

func updateProjectIndexSnapshot(
	previous *projectIndexSnapshot,
	sources map[string]AdmittedSource,
	changed []string,
	deleted []string,
	resolution tsProjectResolution,
) *projectIndexSnapshot {
	model := cloneTSProjectModel(previous.typescript.model)
	touched := append([]string{}, changed...)
	touched = append(touched, deleted...)
	for _, relPath := range touched {
		model.removeModule(moduleBase(relPath))
	}
	files := append([]string{}, changed...)
	sort.Strings(files)
	for _, relPath := range files {
		source, admitted := sources[relPath]
		if !admitted || tsGrammarForExt(filepath.Ext(relPath)) == nil {
			continue
		}
		model.addModule(
			moduleBase(relPath),
			source.bytes(),
			filepath.Dir(relPath),
			resolution,
		)
	}
	typescript := &tsProjectSnapshot{resolution: resolution, model: model}
	return &projectIndexSnapshot{
		typescript: typescript,
		sources:    cloneAdmittedSources(sources),
	}
}

func cloneAdmittedSources(
	sources map[string]AdmittedSource,
) map[string]AdmittedSource {
	clone := make(map[string]AdmittedSource, len(sources))
	for path, source := range sources {
		clone[path] = source
	}
	return clone
}

func cloneTSProjectModel(source *tsProjectModel) *tsProjectModel {
	clone := &tsProjectModel{
		modules:   make(map[string]bool, len(source.modules)),
		direct:    make(map[string]map[string][]tsExportTarget, len(source.direct)),
		reexports: make(map[string]map[string][]tsReexportTarget, len(source.reexports)),
		stars:     make(map[string][]string, len(source.stars)),
	}
	for module, present := range source.modules {
		clone.modules[module] = present
	}
	for module, exports := range source.direct {
		clone.direct[module] = cloneTSExportTargets(exports)
	}
	for module, exports := range source.reexports {
		clone.reexports[module] = cloneTSReexportTargets(exports)
	}
	for module, stars := range source.stars {
		clone.stars[module] = append([]string{}, stars...)
	}
	return clone
}

func cloneTSExportTargets(source map[string][]tsExportTarget) map[string][]tsExportTarget {
	clone := make(map[string][]tsExportTarget, len(source))
	for name, targets := range source {
		clone[name] = append([]tsExportTarget{}, targets...)
	}
	return clone
}

func cloneTSReexportTargets(source map[string][]tsReexportTarget) map[string][]tsReexportTarget {
	clone := make(map[string][]tsReexportTarget, len(source))
	for name, targets := range source {
		clone[name] = append([]tsReexportTarget{}, targets...)
	}
	return clone
}

func (model *tsProjectModel) removeModule(module string) {
	delete(model.modules, module)
	delete(model.direct, module)
	delete(model.reexports, module)
	delete(model.stars, module)
}

var (
	tsDefaultDeclarationPattern = regexp.MustCompile(`(?m)\bexport\s+default\s+(?:async\s+)?(?:function|class)\s+([A-Za-z_$][\w$]*)`)
	tsDefaultIdentifierPattern  = regexp.MustCompile(`(?m)\bexport\s+default\s+([A-Za-z_$][\w$]*)\s*;?`)
	tsNamedDeclarationPattern   = regexp.MustCompile(`(?m)\bexport\s+(?:declare\s+)?(?:async\s+)?(?:function|class|const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)`)
	tsNamedReexportPattern      = regexp.MustCompile(`(?m)\bexport\s*\{([^}]*)\}\s*from\s*["']([^"']+)["']`)
	tsStarReexportPattern       = regexp.MustCompile(`(?m)\bexport\s*\*\s*from\s*["']([^"']+)["']`)
	tsLocalExportPattern        = regexp.MustCompile(`(?m)\bexport\s*\{([^}]*)\}\s*;?(?:\r?\n|$)`)
)

func loadTSProjectModel(
	projectRoot string,
	resolution tsProjectResolution,
) (*tsProjectModel, error) {
	fingerprint, err := tsProjectSourceFingerprint(projectRoot)
	if err != nil {
		return nil, err
	}
	if cached, ok := tsProjectModelCache.Load(projectRoot); ok {
		entry := cached.(tsProjectModelEntry)
		if entry.fingerprint == fingerprint {
			return entry.model, nil
		}
	}
	model, err := buildTSProjectModel(projectRoot, resolution)
	if err != nil {
		return nil, err
	}
	tsProjectModelCache.Store(projectRoot, tsProjectModelEntry{fingerprint: fingerprint, model: model})
	return model, nil
}

func buildTSProjectModel(
	projectRoot string,
	resolution tsProjectResolution,
) (*tsProjectModel, error) {
	registry := NewRegistry()
	budget := DefaultIndexBudget()
	usage := EmptyAdmissionUsage()
	sources := map[string]AdmittedSource{}
	err := walkProjectFiles(projectRoot, func(
		_ string,
		relPath string,
		_ os.DirEntry,
	) error {
		if tsGrammarForExt(filepath.Ext(relPath)) == nil {
			return nil
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
			if info.RequiresRetry() {
				return fmt.Errorf(
					"observe TypeScript project source %s: %s",
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
		sources[relPath] = source
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buildTSProjectModelFromAdmittedSources(sources, resolution), nil
}

func buildTSProjectModelFromAdmittedSources(
	sources map[string]AdmittedSource,
	resolution tsProjectResolution,
) *tsProjectModel {
	model := &tsProjectModel{
		modules:   map[string]bool{},
		direct:    map[string]map[string][]tsExportTarget{},
		reexports: map[string]map[string][]tsReexportTarget{},
		stars:     map[string][]string{},
	}
	files := make([]string, 0, len(sources))
	for relPath := range sources {
		files = append(files, relPath)
	}
	sort.Strings(files)
	for _, relPath := range files {
		if tsGrammarForExt(filepath.Ext(relPath)) == nil {
			continue
		}
		source := sources[relPath]
		model.addModule(
			moduleBase(relPath),
			source.bytes(),
			filepath.Dir(relPath),
			resolution,
		)
	}
	return model
}

func (model *tsProjectModel) addModule(module string, content []byte, fileDir string, resolution tsProjectResolution) {
	model.modules[module] = true
	model.direct[module] = map[string][]tsExportTarget{}
	model.reexports[module] = map[string][]tsReexportTarget{}
	for _, match := range tsDefaultDeclarationPattern.FindAllSubmatch(content, -1) {
		model.addDirect(module, "default", string(match[1]))
	}
	for _, match := range tsDefaultIdentifierPattern.FindAllSubmatch(content, -1) {
		name := string(match[1])
		if name == "function" || name == "class" {
			continue
		}
		model.addDirect(module, "default", name)
	}
	for _, match := range tsNamedDeclarationPattern.FindAllSubmatch(content, -1) {
		name := string(match[1])
		model.addDirect(module, name, name)
	}
	for _, match := range tsNamedReexportPattern.FindAllSubmatch(content, -1) {
		bases, local := resolveTSModuleSpecifiers(string(match[2]), fileDir, resolution)
		if !local {
			continue
		}
		for _, item := range parseTSExportList(string(match[1])) {
			for _, base := range bases {
				model.reexports[module][item.exported] = append(
					model.reexports[module][item.exported],
					tsReexportTarget{moduleBase: base, exportName: item.original},
				)
			}
		}
	}
	for _, match := range tsStarReexportPattern.FindAllSubmatch(content, -1) {
		bases, local := resolveTSModuleSpecifiers(string(match[1]), fileDir, resolution)
		if local {
			model.stars[module] = append(model.stars[module], bases...)
		}
	}
	contentWithoutReexports := tsNamedReexportPattern.ReplaceAll(content, nil)
	for _, match := range tsLocalExportPattern.FindAllSubmatch(contentWithoutReexports, -1) {
		for _, item := range parseTSExportList(string(match[1])) {
			model.addDirect(module, item.exported, item.original)
		}
	}
}

func (model *tsProjectModel) addDirect(module, exported, symbolName string) {
	model.direct[module][exported] = append(
		model.direct[module][exported],
		tsExportTarget{fileBase: module, symbolName: symbolName},
	)
}

type tsExportListItem struct {
	original string
	exported string
}

func parseTSExportList(list string) []tsExportListItem {
	items := make([]tsExportListItem, 0)
	for _, raw := range strings.Split(list, ",") {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) == 0 {
			continue
		}
		item := tsExportListItem{original: fields[0], exported: fields[0]}
		if len(fields) >= 3 && fields[1] == "as" {
			item.exported = fields[2]
		}
		items = append(items, item)
	}
	return items
}

// ResolveExport follows named and wildcard re-exports to terminal declarations.
// A visited (module, export) set makes cycles terminate without a hop limit.
func (model *tsProjectModel) ResolveExport(base, exportName string) []tsExportTarget {
	visited := map[string]bool{}
	resolved := model.resolveExport(base, exportName, visited)
	return dedupeTSExportTargets(resolved)
}

func (model *tsProjectModel) resolveExport(base, exportName string, visited map[string]bool) []tsExportTarget {
	modules := model.moduleCandidates(base)
	resolved := make([]tsExportTarget, 0)
	for _, module := range modules {
		key := module + "\x00" + exportName
		if visited[key] {
			continue
		}
		visited[key] = true
		resolved = append(resolved, model.direct[module][exportName]...)
		for _, target := range model.reexports[module][exportName] {
			resolved = append(resolved, model.resolveExport(target.moduleBase, target.exportName, visited)...)
		}
		if exportName != "default" {
			for _, star := range model.stars[module] {
				resolved = append(resolved, model.resolveExport(star, exportName, visited)...)
			}
		}
	}
	if len(modules) == 0 {
		return []tsExportTarget{{fileBase: base, symbolName: exportName}}
	}
	return resolved
}

func (model *tsProjectModel) moduleCandidates(base string) []string {
	candidates := []string{moduleBase(base), moduleBase(filepath.Join(base, "index"))}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if model.modules[candidate] {
			out = append(out, candidate)
		}
	}
	return uniqueStrings(out)
}

func dedupeTSExportTargets(targets []tsExportTarget) []tsExportTarget {
	seen := make(map[string]bool)
	out := make([]tsExportTarget, 0, len(targets))
	for _, target := range targets {
		key := target.fileBase + "\x00" + target.symbolName
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, target)
	}
	return out
}

func tsProjectSourceFingerprint(
	projectRoot string,
) (string, error) {
	lines := make([]string, 0)
	err := walkProjectFiles(projectRoot, func(
		_ string,
		relPath string,
		entry os.DirEntry,
	) error {
		if tsGrammarForExt(filepath.Ext(relPath)) == nil && !tsProjectConfigFile(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s\x00%d\x00%d", filepath.ToSlash(relPath), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func tsProjectConfigFile(name string) bool {
	if filepath.Ext(name) == ".json" {
		return true
	}
	switch name {
	case "pnpm-workspace.yaml":
		return true
	default:
		return false
	}
}
