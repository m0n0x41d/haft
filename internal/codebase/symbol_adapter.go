package codebase

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/textsearch"
)

// SymbolAdapter is the per-language port for the code-graph node layer.
// Implementations turn one source file into the canonical SymbolSnapshot form;
// stores, drift detection, bindings, and repo-map rendering all consume that
// same form instead of maintaining their own language queries.
type SymbolAdapter interface {
	Extensions() []string
	SymbolLanguage(path string) string
	ExtractSymbolSnapshots(source AdmittedSource) ([]SymbolSnapshot, error)
}

// SymbolAdapterForFile returns the registered rich symbol adapter for a file.
// A nil result means the extension still uses the legacy tree-sitter queries.
func (r *Registry) SymbolAdapterForFile(path string) SymbolAdapter {
	ext := normalizedExtension(path)
	return r.symbolAdapters[ext]
}

// SupportsSymbols reports whether the rich adapter or the legacy extractor can
// produce symbol snapshots for the file.
func (r *Registry) SupportsSymbols(path string) bool {
	if r.SymbolAdapterForFile(path) != nil {
		return true
	}
	ext := normalizedExtension(path)
	_, ok := languages[ext]
	return ok
}

// SymbolLanguageForFile returns the persisted language name for a source file.
func (r *Registry) SymbolLanguageForFile(path string) (string, bool) {
	adapter := r.SymbolAdapterForFile(path)
	if adapter != nil {
		return adapter.SymbolLanguage(path), true
	}
	ext := normalizedExtension(path)
	info, ok := languages[ext]
	if !ok {
		return "", false
	}
	return info.name, true
}

// ReadSourceAdmission is the single filesystem shell before symbol parsing.
func (r *Registry) ReadSourceAdmission(
	projectRoot string,
	relPath string,
	budget IndexBudget,
	usage AdmissionUsage,
) (SourceAdmission, AdmissionUsage, error) {
	path, err := NewProjectPath(relPath)
	if err != nil {
		return nil, usage, err
	}
	languageName, supported := r.SymbolLanguageForFile(relPath)
	classCode := "supported"
	if !supported {
		languageName = "unknown"
		classCode = "unsupported_language"
	}
	ignoreChecker := NewIgnoreChecker(projectRoot)
	if ignoreChecker.IsIgnored(path.String()) {
		classCode = "ignored_path"
	}
	if supported &&
		classCode == "supported" &&
		textsearch.IsGeneratedPath(path.String()) {
		classCode = "generated_source"
	}
	language, err := NewSourceLanguage(languageName)
	if err != nil {
		return nil, usage, err
	}
	class, err := ParseSourceClass(classCode)
	if err != nil {
		return nil, usage, err
	}
	preParserSkip := class.String() == "unsupported_language" ||
		class.String() == "ignored_path" ||
		(class.String() == "generated_source" &&
			budget.GeneratedSources().String() == "exclude_generated")
	if preParserSkip {
		zero, _ := NewByteCount(0)
		observation, err := NewMetadataObservation(
			path,
			language,
			class,
			zero,
		)
		if err != nil {
			return nil, usage, err
		}
		return AdmitSource(observation, budget, usage)
	}
	observation, err := ObserveSource(
		projectRoot,
		path,
		language,
		class,
		budget,
	)
	if err != nil {
		reasonCode := "read_failure"
		if errors.Is(err, ErrSourceChanged) {
			reasonCode = "source_changed"
		}
		reason := sourceSkipReasons[reasonCode]
		zero, _ := NewByteCount(0)
		return newSkippedAdmission(
			path,
			reason,
			zero,
			budget.MaxFileBytes(),
			err.Error(),
			usage,
		)
	}
	return AdmitSource(observation, budget, usage)
}

// ExtractAdmittedSymbolSnapshots routes exact admitted bytes through the rich
// adapter or the legacy tree-sitter compatibility adapter.
func (r *Registry) ExtractAdmittedSymbolSnapshots(
	source AdmittedSource,
) ([]SymbolSnapshot, error) {
	if !source.valid() {
		return nil, fmt.Errorf("symbol extraction requires admitted source")
	}
	relPath := source.Path().String()
	adapter := r.SymbolAdapterForFile(relPath)
	if adapter != nil {
		snapshots, err := adapter.ExtractSymbolSnapshots(source)
		return normalizeSymbolSnapshots(snapshots), err
	}
	ext := normalizedExtension(relPath)
	info, ok := languages[ext]
	if !ok {
		return nil, nil
	}
	snapshots, err := extractLegacySymbolSnapshots(source, info)
	return normalizeSymbolSnapshots(snapshots), err
}

// SourceSkippedError keeps the compatibility path explicit when a caller has
// not yet migrated to SourceAdmission.
type SourceSkippedError struct {
	Info SourceSkipInfo
}

func (e SourceSkippedError) Error() string {
	return fmt.Sprintf(
		"source %s skipped: %s (%s)",
		e.Info.Path,
		e.Info.Reason,
		e.Info.Detail,
	)
}

// ExtractSymbolSnapshots is the temporary raw-path compatibility shell. It
// cannot bypass admission and never turns a skipped source into an empty file.
func (r *Registry) ExtractSymbolSnapshots(
	projectRoot string,
	relPath string,
) ([]SymbolSnapshot, error) {
	source, err := r.ReadAdmittedSource(
		projectRoot,
		relPath,
	)
	if err != nil {
		return nil, err
	}
	return r.ExtractAdmittedSymbolSnapshots(source)
}

// ReadAdmittedSource is the compatibility shell for one-file consumers. Batch
// scanners pass explicit budget and usage through ReadSourceAdmission instead.
func (r *Registry) ReadAdmittedSource(
	projectRoot string,
	relPath string,
) (AdmittedSource, error) {
	admission, _, err := r.ReadSourceAdmission(
		projectRoot,
		relPath,
		DefaultIndexBudget(),
		EmptyAdmissionUsage(),
	)
	if err != nil {
		return AdmittedSource{}, err
	}
	if admission.Kind().String() == "source_skipped" {
		info, err := SkippedSourceInfo(admission)
		if err != nil {
			return AdmittedSource{}, err
		}
		return AdmittedSource{}, SourceSkippedError{Info: info}
	}
	source, err := AdmittedSourceFrom(admission)
	if err != nil {
		return AdmittedSource{}, err
	}
	return source, nil
}

func normalizedExtension(path string) string {
	ext := filepath.Ext(path)
	return strings.ToLower(ext)
}
