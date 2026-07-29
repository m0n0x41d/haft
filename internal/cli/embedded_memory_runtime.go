package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var openEmbeddedMemoryDBFunc = openFPFDBContext

// embeddedMemoryRuntime is the immutable executable projection of the exact
// source-derived TypeEnv embedded in fpf.db. It deliberately owns no database
// handle and has no project-memory dependency.
type embeddedMemoryRuntime struct {
	artifact    typeenv.BaseTypeEnvArtifact
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
}

func (runtime embeddedMemoryRuntime) Artifact() typeenv.BaseTypeEnvArtifact {
	return runtime.artifact
}

func (runtime embeddedMemoryRuntime) Environment() typedmemory.TypeEnv {
	return runtime.environment
}

func (runtime embeddedMemoryRuntime) CodecRegistry() typedmemory.CodecRegistry {
	return runtime.codecs
}

func loadEmbeddedMemoryRuntime(
	ctx context.Context,
) (embeddedMemoryRuntime, error) {
	if ctx == nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("load embedded memory runtime: context is required")
	}
	database, cleanup, err := openEmbeddedMemoryDBFunc(ctx)
	if err != nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("open embedded FPF database: %w", err)
	}
	defer cleanup()

	runtime, err := loadEmbeddedMemoryRuntimeFromDB(ctx, database)
	if err != nil {
		return embeddedMemoryRuntime{}, err
	}
	return runtime, nil
}

func loadEmbeddedMemoryRuntimeFromDB(
	ctx context.Context,
	database *sql.DB,
) (embeddedMemoryRuntime, error) {
	if ctx == nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("load embedded memory runtime: context is required")
	}
	if database == nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("load embedded memory runtime: database is required")
	}

	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(ctx, database)
	if err != nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("load embedded TypeEnv artifact read-only: %w", err)
	}
	if err := verifyEmbeddedMemoryMetadata(ctx, database, artifact); err != nil {
		return embeddedMemoryRuntime{}, err
	}
	environment, codecs, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		return embeddedMemoryRuntime{}, fmt.Errorf("lower embedded TypeEnv runtime: %w", err)
	}
	return embeddedMemoryRuntime{
		artifact:    artifact,
		environment: environment,
		codecs:      codecs,
	}, nil
}

func verifyEmbeddedMemoryMetadata(
	ctx context.Context,
	database *sql.DB,
	artifact typeenv.BaseTypeEnvArtifact,
) error {
	reference, hasReference := artifact.TypeEnvRef()
	if !hasReference {
		return fmt.Errorf("embedded TypeEnv artifact is not executable")
	}
	checks := []struct {
		key  string
		want string
	}{
		{key: "schema_version", want: fpf.SpecIndexSchemaVersion},
		{key: "fpf_commit", want: artifact.SourceRevision().String()},
		{key: "typeenv_artifact_digest", want: artifact.Digest().String()},
		{key: "typeenv_ref", want: reference.String()},
		{key: "typeenv_compiler_schema_version", want: artifact.CompilerSchemaVersion().String()},
		{key: "typeenv_posture", want: artifact.Posture().String()},
		{key: "typeenv_source_revision", want: artifact.SourceRevision().String()},
	}
	for _, check := range checks {
		var got string
		err := database.
			QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, check.key).
			Scan(&got)
		if err != nil {
			return fmt.Errorf("read embedded FPF metadata %q: %w", check.key, err)
		}
		if got != check.want {
			return fmt.Errorf(
				"embedded FPF metadata %s=%q, want %q",
				check.key,
				got,
				check.want,
			)
		}
	}
	return nil
}
