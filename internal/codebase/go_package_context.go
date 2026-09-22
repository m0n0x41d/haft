package codebase

import (
	"context"
	"fmt"
	"path/filepath"
)

// goPackageContext is immutable during file resolution. The same package
// symbols, interfaces and type facts used by point resolution are prepared
// once per refresh batch, rather than reparsing every sibling for every file.
// It is never cached across batches: changed/deleted sources and a different
// symbol view must produce a new context from that batch's admitted bytes.
type goPackageContext struct {
	symbols    []CodeSymbol
	interfaces map[string]InterfaceDef
	facts      TypeFacts
}

func prepareGoPackageContexts(
	ctx context.Context,
	projectRoot string,
	files []string,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) (*projectIndexSnapshot, error) {
	packages := map[string]goPackageContext{}
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if filepath.Ext(path) != ".go" {
			continue
		}
		directory := filepath.Dir(path)
		if _, prepared := packages[directory]; prepared {
			continue
		}
		pkgSyms, err := symbols.GetByDir(ctx, directory)
		if err != nil {
			return nil, err
		}
		packageContext, err := collectGoPackageContext(ctx, projectRoot, pkgSyms, snapshot.sources)
		if err != nil {
			return nil, fmt.Errorf("prepare Go package %s: %w", directory, err)
		}
		packages[directory] = packageContext
	}
	prepared := *snapshot
	prepared.goPackages = packages
	return &prepared, nil
}

func (snapshot *projectIndexSnapshot) goPackageContext(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
) (goPackageContext, error) {
	sourcePath := source.Path().String()
	directory := filepath.Dir(sourcePath)
	if prepared, found := snapshot.goPackages[directory]; found {
		return prepared, nil
	}
	// Point consumers may supply a source snapshot without batch preparation.
	pkgSyms, err := symbols.GetByDir(ctx, directory)
	if err != nil {
		return goPackageContext{}, err
	}
	return collectGoPackageContext(ctx, projectRoot, pkgSyms, snapshot.sources)
}

func collectGoPackageContext(
	ctx context.Context,
	projectRoot string,
	symbols []CodeSymbol,
	sources map[string]AdmittedSource,
) (goPackageContext, error) {
	result := goPackageContext{
		symbols:    symbols,
		interfaces: map[string]InterfaceDef{},
		facts:      NewTypeFacts(),
	}
	files := distinctFiles(symbols)
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return goPackageContext{}, err
		}
		source, err := admittedProjectSource(projectRoot, path, sources)
		if err != nil {
			return goPackageContext{}, err
		}
		definitions, err := ExtractGoInterfacesFromSource(source)
		if err != nil {
			return goPackageContext{}, err
		}
		for _, definition := range definitions {
			result.interfaces[definition.Name] = definition
		}
		facts, err := ExtractGoTypeFactsFromSource(source)
		if err != nil {
			return goPackageContext{}, err
		}
		result.facts.merge(facts)
	}
	return result, nil
}
