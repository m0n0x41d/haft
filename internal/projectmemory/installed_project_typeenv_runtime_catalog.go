package projectmemory

import (
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
)

var (
	ErrInstalledProjectTypeEnvRuntimeEntryInvalid = errors.New(
		"installed project TypeEnv runtime entry is invalid",
	)
	ErrInstalledProjectTypeEnvRuntimeCatalogInvalid = errors.New(
		"installed project TypeEnv runtime catalog is invalid",
	)
)

// InstalledProjectTypeEnvRuntimeEntry binds one process-owned callable surface
// to the exact X it can execute. The constructor proves that relationship; a
// persisted X is never allowed to select an unkeyed default runtime.
type InstalledProjectTypeEnvRuntimeEntry struct {
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef
	installed    projecttypeenvruntime.InstalledRuntimeRegistryInput
}

func NewInstalledProjectTypeEnvRuntimeEntry(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed projecttypeenvruntime.InstalledRuntimeRegistryInput,
) (InstalledProjectTypeEnvRuntimeEntry, error) {
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtimeBasis,
			Installed:    installed,
		},
	)
	matched, exact := resolution.(projecttypeenvruntime.Matched)
	if !exact {
		return InstalledProjectTypeEnvRuntimeEntry{}, fmt.Errorf(
			"%w: X %q does not match the supplied runtime (%s)",
			ErrInstalledProjectTypeEnvRuntimeEntryInvalid,
			runtimeBasis.Ref().String(),
			resolution.Kind().String(),
		)
	}
	registry, present := matched.Registry()
	if !present || !registry.Valid() {
		return InstalledProjectTypeEnvRuntimeEntry{}, fmt.Errorf(
			"%w: X %q exposed no exact target registry",
			ErrInstalledProjectTypeEnvRuntimeEntryInvalid,
			runtimeBasis.Ref().String(),
		)
	}
	matchedBasis, present := registry.RuntimeBasisRef()
	if !present || matchedBasis != runtimeBasis.Ref() {
		return InstalledProjectTypeEnvRuntimeEntry{}, fmt.Errorf(
			"%w: matched target registry differs from X %q",
			ErrInstalledProjectTypeEnvRuntimeEntryInvalid,
			runtimeBasis.Ref().String(),
		)
	}
	return InstalledProjectTypeEnvRuntimeEntry{
		runtimeBasis: runtimeBasis.Ref(),
		installed:    cloneInstalledRuntimeRegistry(installed),
	}, nil
}

func (entry InstalledProjectTypeEnvRuntimeEntry) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return entry.runtimeBasis
}

func (entry InstalledProjectTypeEnvRuntimeEntry) InstalledRuntime() projecttypeenvruntime.InstalledRuntimeRegistryInput {
	return cloneInstalledRuntimeRegistry(entry.installed)
}

// InstalledProjectTypeEnvRuntimeCatalog is an immutable exact-X dispatch
// table. Unknown X coordinates have no fallback and therefore fail closed.
type InstalledProjectTypeEnvRuntimeCatalog struct {
	byRuntimeBasis map[string]InstalledProjectTypeEnvRuntimeEntry
}

func NewInstalledProjectTypeEnvRuntimeCatalog(
	entries []InstalledProjectTypeEnvRuntimeEntry,
) (InstalledProjectTypeEnvRuntimeCatalog, error) {
	if len(entries) == 0 {
		return InstalledProjectTypeEnvRuntimeCatalog{}, fmt.Errorf(
			"%w: at least one exact-X entry is required",
			ErrInstalledProjectTypeEnvRuntimeCatalogInvalid,
		)
	}
	byRuntimeBasis := make(
		map[string]InstalledProjectTypeEnvRuntimeEntry,
		len(entries),
	)
	for _, entry := range entries {
		ref := entry.RuntimeBasis()
		parsed, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(ref.String())
		if err != nil || parsed != ref {
			return InstalledProjectTypeEnvRuntimeCatalog{}, fmt.Errorf(
				"%w: entry has no exact X coordinate",
				ErrInstalledProjectTypeEnvRuntimeCatalogInvalid,
			)
		}
		if _, exists := byRuntimeBasis[ref.String()]; exists {
			return InstalledProjectTypeEnvRuntimeCatalog{}, fmt.Errorf(
				"%w: duplicate X %q",
				ErrInstalledProjectTypeEnvRuntimeCatalogInvalid,
				ref.String(),
			)
		}
		byRuntimeBasis[ref.String()] = InstalledProjectTypeEnvRuntimeEntry{
			runtimeBasis: ref,
			installed:    entry.InstalledRuntime(),
		}
	}
	return InstalledProjectTypeEnvRuntimeCatalog{
		byRuntimeBasis: byRuntimeBasis,
	}, nil
}

func (catalog InstalledProjectTypeEnvRuntimeCatalog) Lookup(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisRef,
) (projecttypeenvruntime.InstalledRuntimeRegistryInput, bool) {
	entry, present := catalog.byRuntimeBasis[runtimeBasis.String()]
	if !present {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, false
	}
	return entry.InstalledRuntime(), true
}

func (catalog InstalledProjectTypeEnvRuntimeCatalog) Len() int {
	return len(catalog.byRuntimeBasis)
}
