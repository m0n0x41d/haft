// Package artifacts exposes the exact immutable Base TypeEnv artifacts needed
// to execute shipped historical Haft Local-Practice editions. The archive is
// keyed only by content identity; it has no "latest" or fallback lookup.
package artifacts

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	HistoricalV3Ref = "typeenv:sha256:973eeeed8e234b4ff0194662d80e204fe27ad5ba92c87840a6d1ed3a9d5d742d"
	HistoricalV4Ref = "typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6"

	historicalV3CompilerSchema = typeenv.BaseTypeEnvCompilerSchemaV3
	historicalV3SourceRevision = "6e7eeb93d7d6208877649ac999d52ab845640817"
	historicalV4CompilerSchema = typeenv.BaseTypeEnvCompilerSchemaV4
	historicalV4SourceRevision = "0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4"
)

var ErrExactArtifactNotFound = errors.New(
	"exact archived Base TypeEnv artifact is not installed",
)

type historicalArtifact struct {
	canonical      []byte
	compilerSchema string
	sourceRevision string
}

// historicalV3Canonical is the exact canonical artifact payload compiled from
// FPF revision 6e7eeb9 by compiler schema v3. It is replay input, not a current
// source publication and not a substitute for the aligned embedded FPF bundle.
//
//go:embed 6e7eeb9-cov2-v3.bin
var historicalV3Canonical []byte

// historicalV4Canonical is the exact canonical artifact payload compiled from
// FPF revision 0990ff1 by compiler schema v4. It remains available for exact
// replay after a newer FPF Base TypeEnv becomes current.
//
//go:embed 0990ff1-cov2-v4.bin
var historicalV4Canonical []byte

var historicalArtifacts = map[string]historicalArtifact{
	HistoricalV3Ref: {
		canonical:      historicalV3Canonical,
		compilerSchema: historicalV3CompilerSchema,
		sourceRevision: historicalV3SourceRevision,
	},
	HistoricalV4Ref: {
		canonical:      historicalV4Canonical,
		compilerSchema: historicalV4CompilerSchema,
		sourceRevision: historicalV4SourceRevision,
	},
}

// LoadExact decodes a private immutable value only when the requested identity
// is present in the shipped archive. Unknown identities fail closed.
func LoadExact(
	ref typedmemory.TypeEnvRef,
) (typeenv.BaseTypeEnvArtifact, error) {
	reference := ref.String()
	archived, exists := historicalArtifacts[reference]
	if !exists {
		return typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"%w: %s",
			ErrExactArtifactNotFound,
			reference,
		)
	}
	canonical := append([]byte(nil), archived.canonical...)
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"decode archived Base TypeEnv %s: %w",
			reference,
			err,
		)
	}
	if err := verifyHistoricalArtifact(artifact, ref, archived); err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	return artifact, nil
}

func verifyHistoricalArtifact(
	artifact typeenv.BaseTypeEnvArtifact,
	expected typedmemory.TypeEnvRef,
	archived historicalArtifact,
) error {
	if err := artifact.Verify(); err != nil {
		return fmt.Errorf("verify archived Base TypeEnv: %w", err)
	}
	actual, present := artifact.TypeEnvRef()
	if !present || actual != expected {
		return fmt.Errorf(
			"archived Base TypeEnv identity differs from requested %s",
			expected.String(),
		)
	}
	compilerSchema := artifact.CompilerSchemaVersion().String()
	if compilerSchema != archived.compilerSchema {
		return fmt.Errorf(
			"archived Base TypeEnv compiler schema %q differs from %q",
			compilerSchema,
			archived.compilerSchema,
		)
	}
	sourceRevision := artifact.SourceRevision().String()
	if sourceRevision != archived.sourceRevision {
		return fmt.Errorf(
			"archived Base TypeEnv source revision %q differs from %q",
			sourceRevision,
			archived.sourceRevision,
		)
	}
	return nil
}
