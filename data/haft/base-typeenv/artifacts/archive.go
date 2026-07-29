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

	historicalV3CompilerSchema = typeenv.BaseTypeEnvCompilerSchemaV3
	historicalV3SourceRevision = "6e7eeb93d7d6208877649ac999d52ab845640817"
)

var ErrExactArtifactNotFound = errors.New(
	"exact archived Base TypeEnv artifact is not installed",
)

// historicalV3Canonical is the exact canonical artifact payload compiled from
// FPF revision 6e7eeb9 by compiler schema v3. It is replay input, not a current
// source publication and not a substitute for the aligned embedded FPF bundle.
//
//go:embed 6e7eeb9-cov2-v3.bin
var historicalV3Canonical []byte

// LoadExact decodes a private immutable value only when the requested identity
// is present in the shipped archive. Unknown identities fail closed.
func LoadExact(
	ref typedmemory.TypeEnvRef,
) (typeenv.BaseTypeEnvArtifact, error) {
	if ref.String() != HistoricalV3Ref {
		return typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"%w: %s",
			ErrExactArtifactNotFound,
			ref.String(),
		)
	}
	artifact, err := typeenv.DecodeBaseTypeEnvArtifact(
		append([]byte(nil), historicalV3Canonical...),
	)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"decode archived Base TypeEnv %s: %w",
			ref.String(),
			err,
		)
	}
	if err := verifyHistoricalV3(artifact, ref); err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	return artifact, nil
}

func verifyHistoricalV3(
	artifact typeenv.BaseTypeEnvArtifact,
	expected typedmemory.TypeEnvRef,
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
	if artifact.CompilerSchemaVersion().String() != historicalV3CompilerSchema {
		return fmt.Errorf(
			"archived Base TypeEnv compiler schema %q differs from %q",
			artifact.CompilerSchemaVersion().String(),
			historicalV3CompilerSchema,
		)
	}
	if artifact.SourceRevision().String() != historicalV3SourceRevision {
		return fmt.Errorf(
			"archived Base TypeEnv source revision %q differs from %q",
			artifact.SourceRevision().String(),
			historicalV3SourceRevision,
		)
	}
	return nil
}
