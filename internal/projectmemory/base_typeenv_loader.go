// Package projectmemory composes project-bound typed-memory capabilities
// without exposing project activation or host wiring.
package projectmemory

import (
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrTypeEnvSnapshotMissing = errors.New("project-memory TypeEnv snapshot is missing")
	ErrTypeEnvSnapshotFormat  = errors.New("project-memory TypeEnv snapshot format is unsupported")
	ErrTypeEnvSnapshotPayload = errors.New("project-memory TypeEnv snapshot payload is invalid")
	ErrTypeEnvSnapshotBasis   = errors.New("project-memory TypeEnv snapshot basis is inconsistent")
)

// BaseTypeEnvLoader reconstructs only the exact source-derived base TypeEnv
// snapshot format. Project extensions and activation remain separate effects.
type BaseTypeEnvLoader struct{}

var _ typedmemorystore.TypeEnvLoader = BaseTypeEnvLoader{}

func NewBaseTypeEnvLoader() BaseTypeEnvLoader {
	return BaseTypeEnvLoader{}
}

// NewBaseTypeEnvSnapshot converts one exact executable source-derived base
// artifact into the sole revision-zero graph snapshot format. It performs no
// storage, graph initialization, or project-TypeEnv head selection.
func NewBaseTypeEnvSnapshot(
	artifact typeenv.BaseTypeEnvArtifact,
) (typedmemorystore.TypeEnvSnapshot, error) {
	if err := artifact.Verify(); err != nil {
		return typedmemorystore.TypeEnvSnapshot{}, fmt.Errorf(
			"build base TypeEnv snapshot: verify artifact: %w",
			err,
		)
	}
	reference, executable := artifact.TypeEnvRef()
	if !executable {
		return typedmemorystore.TypeEnvSnapshot{}, fmt.Errorf(
			"build base TypeEnv snapshot: %w: coverage-only artifact has no executable TypeEnvRef",
			ErrTypeEnvSnapshotPayload,
		)
	}
	format, err := typedmemorystore.NewSnapshotFormat(
		typedmemorystore.BaseTypeEnvSnapshotFormat,
	)
	if err != nil {
		return typedmemorystore.TypeEnvSnapshot{}, fmt.Errorf(
			"build base TypeEnv snapshot format: %w",
			err,
		)
	}
	snapshot, err := typedmemorystore.NewTypeEnvSnapshotBuilder(reference).
		SetFormat(format).
		SetCanonicalBytes(artifact.CanonicalBytes()).
		SetSourceRevision(artifact.SourceRevision()).
		SetCompilerSchemaVersion(artifact.CompilerSchemaVersion()).
		Build()
	if err != nil {
		return typedmemorystore.TypeEnvSnapshot{}, fmt.Errorf(
			"build base TypeEnv snapshot: %w",
			err,
		)
	}
	return snapshot, nil
}

func (BaseTypeEnvLoader) LoadTypeEnv(
	snapshot typedmemorystore.TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	format := snapshot.Format().String()
	if format == "" {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, ErrTypeEnvSnapshotMissing
	}
	if format != typedmemorystore.BaseTypeEnvSnapshotFormat {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"%w: got %q, want %q",
			ErrTypeEnvSnapshotFormat,
			format,
			typedmemorystore.BaseTypeEnvSnapshotFormat,
		)
	}

	canonical := snapshot.CanonicalBytes()
	if len(canonical) == 0 {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, ErrTypeEnvSnapshotMissing
	}
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"%w: %w",
			ErrTypeEnvSnapshotPayload,
			err,
		)
	}
	if err := requireExactSnapshotBasis(snapshot, artifact); err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}

	environment, registry, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"%w: lower base TypeEnv: %w",
			ErrTypeEnvSnapshotPayload,
			err,
		)
	}
	if err := requireExactRuntimeBasis(snapshot, environment); err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	return environment, registry, nil
}

func requireExactSnapshotBasis(
	snapshot typedmemorystore.TypeEnvSnapshot,
	artifact typeenv.BaseTypeEnvArtifact,
) error {
	reference, executable := artifact.TypeEnvRef()
	if !executable {
		return fmt.Errorf(
			"%w: base artifact is coverage-only and has no executable TypeEnvRef",
			ErrTypeEnvSnapshotPayload,
		)
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{name: "TypeEnvRef", got: reference.String(), want: snapshot.Ref().String()},
		{name: "artifact digest", got: artifact.Digest().String(), want: snapshot.ArtifactDigest().String()},
		{name: "source revision", got: artifact.SourceRevision().String(), want: snapshot.SourceRevision().String()},
		{name: "compiler schema", got: artifact.CompilerSchemaVersion().String(), want: snapshot.CompilerSchemaVersion().String()},
	}
	for _, check := range checks {
		if check.got == check.want && check.want != "" {
			continue
		}
		return fmt.Errorf(
			"%w: %s=%q, snapshot=%q",
			ErrTypeEnvSnapshotBasis,
			check.name,
			check.got,
			check.want,
		)
	}
	return nil
}

func requireExactRuntimeBasis(
	snapshot typedmemorystore.TypeEnvSnapshot,
	environment typedmemory.TypeEnv,
) error {
	checks := []struct {
		name string
		got  string
		want string
	}{
		{name: "runtime TypeEnvRef", got: environment.Ref().String(), want: snapshot.Ref().String()},
		{name: "runtime source revision", got: environment.SourceRevision().String(), want: snapshot.SourceRevision().String()},
		{name: "runtime compiler schema", got: environment.CompilerSchemaVersion().String(), want: snapshot.CompilerSchemaVersion().String()},
	}
	for _, check := range checks {
		if check.got == check.want && check.want != "" {
			continue
		}
		return fmt.Errorf(
			"%w: %s=%q, snapshot=%q",
			ErrTypeEnvSnapshotBasis,
			check.name,
			check.got,
			check.want,
		)
	}
	return nil
}
