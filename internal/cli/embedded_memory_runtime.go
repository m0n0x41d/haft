package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var openEmbeddedMemoryDBFunc = openFPFDBContext

var processEmbeddedMemoryRuntimeCache embeddedMemoryRuntimeCache

// embeddedMemoryRuntime is the immutable executable projection of the exact
// source-derived TypeEnv embedded in fpf.db. It deliberately owns no database
// handle and has no project-memory dependency.
type embeddedMemoryRuntime struct {
	artifact    typeenv.BaseTypeEnvArtifact
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
}

// embeddedMemoryRuntimeCache owns one immutable source-derived runtime for the
// current process. The embedded database and its source bytes cannot change
// after the binary starts, so reloading and lowering that exact artifact for
// every command or test adds work without refreshing any observable basis.
//
// Failed and cancelled loads are deliberately not cached. A later caller can
// therefore retry after a transient extraction or context failure.
type embeddedMemoryRuntimeCache struct {
	mu       sync.Mutex
	runtime  embeddedMemoryRuntime
	ready    bool
	inFlight *embeddedMemoryRuntimeLoad
}

type embeddedMemoryRuntimeLoad struct {
	done       chan struct{}
	runtime    embeddedMemoryRuntime
	err        error
	panicked   bool
	panicValue any
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
	return processEmbeddedMemoryRuntimeCache.load(
		ctx,
		loadEmbeddedMemoryRuntimeUncached,
	)
}

func (cache *embeddedMemoryRuntimeCache) load(
	ctx context.Context,
	loader func(context.Context) (embeddedMemoryRuntime, error),
) (embeddedMemoryRuntime, error) {
	if ctx == nil {
		return embeddedMemoryRuntime{}, fmt.Errorf(
			"load embedded memory runtime: context is required",
		)
	}
	if loader == nil {
		return embeddedMemoryRuntime{}, fmt.Errorf(
			"load embedded memory runtime: loader is required",
		)
	}
	for {
		if err := ctx.Err(); err != nil {
			return embeddedMemoryRuntime{}, fmt.Errorf(
				"load embedded memory runtime: %w",
				err,
			)
		}

		cache.mu.Lock()
		if cache.ready {
			runtime := cache.runtime
			cache.mu.Unlock()
			if err := ctx.Err(); err != nil {
				return embeddedMemoryRuntime{}, fmt.Errorf(
					"load embedded memory runtime: %w",
					err,
				)
			}
			return runtime, nil
		}
		if cache.inFlight != nil {
			load := cache.inFlight
			cache.mu.Unlock()
			select {
			case <-load.done:
				if err := ctx.Err(); err != nil {
					return embeddedMemoryRuntime{}, fmt.Errorf(
						"load embedded memory runtime: %w",
						err,
					)
				}
				if load.panicked {
					panic(load.panicValue)
				}
				if errors.Is(load.err, context.Canceled) ||
					errors.Is(load.err, context.DeadlineExceeded) {
					continue
				}
				return load.runtime, load.err
			case <-ctx.Done():
				return embeddedMemoryRuntime{}, fmt.Errorf(
					"load embedded memory runtime: %w",
					ctx.Err(),
				)
			}
		}
		load := &embeddedMemoryRuntimeLoad{done: make(chan struct{})}
		cache.inFlight = load
		cache.mu.Unlock()

		var runtime embeddedMemoryRuntime
		var err error
		var panicValue any
		func() {
			defer func() {
				panicValue = recover()
			}()
			runtime, err = loader(ctx)
		}()
		if panicValue == nil && err == nil {
			if contextErr := ctx.Err(); contextErr != nil {
				err = fmt.Errorf(
					"load embedded memory runtime: %w",
					contextErr,
				)
				runtime = embeddedMemoryRuntime{}
			}
		}

		cache.mu.Lock()
		load.runtime = runtime
		load.err = err
		load.panicked = panicValue != nil
		load.panicValue = panicValue
		if panicValue == nil && err == nil {
			cache.runtime = runtime
			cache.ready = true
		}
		cache.inFlight = nil
		close(load.done)
		cache.mu.Unlock()
		if panicValue != nil {
			panic(panicValue)
		}
		return runtime, err
	}
}

func loadEmbeddedMemoryRuntimeUncached(
	ctx context.Context,
) (embeddedMemoryRuntime, error) {
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
