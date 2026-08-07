package artifact

import (
	"fmt"
	"os"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

func canonicalAffectedFile(file AffectedFile) (AffectedFile, error) {
	path, err := projectpath.Parse(file.Path)
	if err != nil {
		return AffectedFile{}, fmt.Errorf("affected file: %w", err)
	}
	file.Path = path.String()
	return file, nil
}

func canonicalAffectedFiles(files []AffectedFile) ([]AffectedFile, error) {
	result := make([]AffectedFile, 0, len(files))
	seen := make(map[string]string)
	for _, file := range files {
		canonical, err := canonicalAffectedFile(file)
		if err != nil {
			return nil, err
		}
		hash, exists := seen[canonical.Path]
		if exists && hash != canonical.Hash {
			return nil, fmt.Errorf(
				"affected file %q has conflicting canonical hashes",
				canonical.Path,
			)
		}
		if exists {
			continue
		}
		seen[canonical.Path] = canonical.Hash
		result = append(result, canonical)
	}
	return result, nil
}

func canonicalAffectedSymbol(
	symbol AffectedSymbol,
) (AffectedSymbol, error) {
	path, err := projectpath.Parse(symbol.FilePath)
	if err != nil {
		return AffectedSymbol{}, fmt.Errorf("affected symbol file: %w", err)
	}
	symbol.FilePath = path.String()
	return symbol, nil
}

func canonicalAffectedSymbols(
	symbols []AffectedSymbol,
) ([]AffectedSymbol, error) {
	result := make([]AffectedSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		canonical, err := canonicalAffectedSymbol(symbol)
		if err != nil {
			return nil, err
		}
		result = append(result, canonical)
	}
	return result, nil
}

func canonicalSymbolBinding(
	binding SymbolBinding,
) (SymbolBinding, error) {
	path, err := projectpath.Parse(binding.FilePath)
	if err != nil {
		return SymbolBinding{}, fmt.Errorf("symbol binding file: %w", err)
	}
	binding.FilePath = path.String()
	return binding, nil
}

func canonicalAffectedFilePaths(paths []string) ([]AffectedFile, error) {
	files := make([]AffectedFile, 0, len(paths))
	for _, raw := range paths {
		file, err := canonicalAffectedFile(AffectedFile{Path: raw})
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func resolveAffectedFile(projectRoot string, file AffectedFile) (string, error) {
	return resolveProjectFile(projectRoot, file.Path)
}

func resolveProjectFile(projectRoot string, rawPath string) (string, error) {
	path, err := projectpath.Parse(rawPath)
	if err != nil {
		return "", fmt.Errorf("project file: %w", err)
	}
	return projectpath.ResolveExisting(projectRoot, path)
}

func readProjectFile(projectRoot string, rawPath string) ([]byte, error) {
	resolved, err := resolveProjectFile(projectRoot, rawPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(resolved)
}
