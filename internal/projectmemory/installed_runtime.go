package projectmemory

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	baseCodecRuntimeCatalogCarrier = "haft.runtime.catalog.fpf-base-codecs"
	baseCodecRuntimeCatalogVersion = "1.0.0"
)

// ComposeInstalledBaseTypeEnvRuntime turns one process-loaded executable base
// TypeEnv into the exact runtime input later compared with X. The catalog
// declares only the codec entrypoints actually present in the supplied
// immutable registry. It does not infer project-local evaluators, delivery
// boundaries, registration policies, or executable attestation.
func ComposeInstalledBaseTypeEnvRuntime(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
) (projecttypeenvruntime.InstalledRuntimeRegistryInput, error) {
	if environment.Ref().Digest().String() == "" {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{},
			fmt.Errorf("compose installed base TypeEnv runtime: exact TypeEnv is required")
	}
	if environment.SourceRevision().String() == "" {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{},
			fmt.Errorf(
				"compose installed base TypeEnv runtime: source revision is required",
			)
	}
	entries, err := baseCodecRuntimeEntries(environment, codecs)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	catalogs, err := sealBaseCodecRuntimeCatalog(
		environment.SourceRevision(),
		entries,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	return projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:            codecs,
		MechanismCatalogs: catalogs,
	}, nil
}

func baseCodecRuntimeEntries(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
) ([]runtimemechanism.RuntimeMechanismEntryV1, error) {
	bindings := environment.ValueBindings()
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		codec := binding.Codec()
		if !codecs.Contains(codec) {
			return nil, fmt.Errorf(
				"compose installed base TypeEnv runtime: codec %q is not installed",
				codec.String(),
			)
		}
		if _, exists := seen[codec.String()]; exists {
			continue
		}
		entry, err := runtimemechanism.NewCodecCanonicalizationEntry(codec)
		if err != nil {
			return nil, fmt.Errorf(
				"compose installed base TypeEnv runtime codec %q: %w",
				codec.String(),
				err,
			)
		}
		seen[codec.String()] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, nil
}

func sealBaseCodecRuntimeCatalog(
	source typedmemory.SourceRevision,
	entries []runtimemechanism.RuntimeMechanismEntryV1,
) ([]runtimemechanism.RuntimeMechanismArtifactV1, error) {
	if len(entries) == 0 {
		return []runtimemechanism.RuntimeMechanismArtifactV1{}, nil
	}
	carrier, err := typedmemory.NewCarrierRef(baseCodecRuntimeCatalogCarrier)
	if err != nil {
		return nil, fmt.Errorf("compose base codec runtime carrier: %w", err)
	}
	edition, err := typedmemory.NewCarrierEdition(
		baseCodecRuntimeCatalogVersion + "+fpf." + source.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("compose base codec runtime edition: %w", err)
	}
	catalog, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		carrier,
		edition,
		entries,
	)
	if err != nil {
		return nil, fmt.Errorf("seal base codec runtime catalog: %w", err)
	}
	return []runtimemechanism.RuntimeMechanismArtifactV1{catalog}, nil
}
