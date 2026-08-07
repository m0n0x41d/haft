package typedmemoryvalidation

import (
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type BasisResolutionKind string

const (
	BasisResolutionBundledCandidate BasisResolutionKind = "bundled_candidate_open_world"
	BasisResolutionProject          BasisResolutionKind = "resolved_project_basis"
	BasisResolutionProjectMissing   BasisResolutionKind = "project_basis_unavailable"
	BasisResolutionExactMismatch    BasisResolutionKind = "exact_project_basis_mismatch"
)

// BasisResolver is the only authority which may resolve an untrusted wire
// selector. Implementations belong to the outer server shell. The request
// itself never supplies a TypeEnv, codec registry, or project snapshot.
type BasisResolver interface {
	Resolve(typedmemorywire.BasisSelector) BasisResolution
}

type BasisResolution interface {
	Kind() BasisResolutionKind
	basisResolutionVariant()
}

// BundledCandidateOpenWorldBasis carries the exact source-derived FPF runtime
// needed for structural lowering. It deliberately carries no MemorySnapshot
// and no GraphRevision.
type BundledCandidateOpenWorldBasis struct {
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
}

func NewBundledCandidateOpenWorldBasis(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
) (*BundledCandidateOpenWorldBasis, error) {
	if !typeEnvPresent(environment) {
		return nil, fmt.Errorf("bundled candidate requires an exact TypeEnv")
	}
	return &BundledCandidateOpenWorldBasis{
		environment: environment,
		codecs:      codecs,
	}, nil
}

func (*BundledCandidateOpenWorldBasis) Kind() BasisResolutionKind {
	return BasisResolutionBundledCandidate
}

func (*BundledCandidateOpenWorldBasis) basisResolutionVariant() {}

func (basis *BundledCandidateOpenWorldBasis) Environment() typedmemory.TypeEnv {
	if basis == nil {
		return typedmemory.TypeEnv{}
	}
	return basis.environment
}

func (basis *BundledCandidateOpenWorldBasis) Codecs() typedmemory.CodecRegistry {
	if basis == nil {
		return typedmemory.CodecRegistry{}
	}
	return basis.codecs
}

// ResolvedProjectBasis is the future P8 join and the P7 fixture seam. It is
// honest only when one immutable snapshot is correlated with the exact
// environment. Production P7 resolvers do not return this variant yet.
type ResolvedProjectBasis struct {
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
	snapshot    typedmemory.MemorySnapshot
}

func NewResolvedProjectBasis(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	snapshot typedmemory.MemorySnapshot,
) (*ResolvedProjectBasis, error) {
	if !typeEnvPresent(environment) {
		return nil, fmt.Errorf("resolved project basis requires an exact TypeEnv")
	}
	if !memorySnapshotPresent(snapshot) {
		return nil, fmt.Errorf("resolved project basis requires an immutable MemorySnapshot")
	}
	if snapshot.TypeEnvRef() != environment.Ref() {
		return nil, fmt.Errorf("project snapshot TypeEnvRef differs from resolved environment")
	}
	return &ResolvedProjectBasis{
		environment: environment,
		codecs:      codecs,
		snapshot:    snapshot,
	}, nil
}

func (*ResolvedProjectBasis) Kind() BasisResolutionKind {
	return BasisResolutionProject
}

func (*ResolvedProjectBasis) basisResolutionVariant() {}

func (basis *ResolvedProjectBasis) Environment() typedmemory.TypeEnv {
	if basis == nil {
		return typedmemory.TypeEnv{}
	}
	return basis.environment
}

func (basis *ResolvedProjectBasis) Codecs() typedmemory.CodecRegistry {
	if basis == nil {
		return typedmemory.CodecRegistry{}
	}
	return basis.codecs
}

func (basis *ResolvedProjectBasis) Snapshot() typedmemory.MemorySnapshot {
	if basis == nil {
		return nil
	}
	return basis.snapshot
}

// ProjectBasisUnavailable is the canonical pre-P8 project result. It states
// both missing facts instead of treating absent future tables as an empty
// graph.
type ProjectBasisUnavailable struct{}

func NewProjectBasisUnavailable() *ProjectBasisUnavailable {
	return &ProjectBasisUnavailable{}
}

func (*ProjectBasisUnavailable) Kind() BasisResolutionKind {
	return BasisResolutionProjectMissing
}

func (*ProjectBasisUnavailable) basisResolutionVariant() {}

// ExactProjectBasisMismatch records an observed server basis which does not
// satisfy an exact request. The service never falls back to that observation.
type ExactProjectBasisMismatch struct {
	observedTypeEnv       typedmemory.TypeEnvRef
	observedGraphRevision typedmemory.GraphRevision
}

func NewExactProjectBasisMismatch(
	observedTypeEnv typedmemory.TypeEnvRef,
	observedGraphRevision typedmemory.GraphRevision,
) (*ExactProjectBasisMismatch, error) {
	if observedTypeEnv.Digest().String() == "" {
		return nil, fmt.Errorf("exact project mismatch requires an observed TypeEnvRef")
	}
	return &ExactProjectBasisMismatch{
		observedTypeEnv:       observedTypeEnv,
		observedGraphRevision: observedGraphRevision,
	}, nil
}

func (*ExactProjectBasisMismatch) Kind() BasisResolutionKind {
	return BasisResolutionExactMismatch
}

func (*ExactProjectBasisMismatch) basisResolutionVariant() {}

func (basis *ExactProjectBasisMismatch) ObservedTypeEnvRef() typedmemory.TypeEnvRef {
	if basis == nil {
		return typedmemory.TypeEnvRef{}
	}
	return basis.observedTypeEnv
}

func (basis *ExactProjectBasisMismatch) ObservedGraphRevision() typedmemory.GraphRevision {
	if basis == nil {
		return typedmemory.GraphRevision{}
	}
	return basis.observedGraphRevision
}

func typeEnvPresent(environment typedmemory.TypeEnv) bool {
	return environment.Ref().Digest().String() != ""
}

func memorySnapshotPresent(snapshot typedmemory.MemorySnapshot) bool {
	if snapshot == nil {
		return false
	}
	value := reflect.ValueOf(snapshot)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func basisResolutionPresent(resolution BasisResolution) bool {
	if resolution == nil {
		return false
	}
	value := reflect.ValueOf(resolution)
	if value.Kind() != reflect.Pointer {
		return true
	}
	return !value.IsNil()
}
