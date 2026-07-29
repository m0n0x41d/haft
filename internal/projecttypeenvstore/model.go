package projecttypeenvstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	baseArtifactCanonicalSchema       = "base-typeenv-artifact-payload.v1"
	extensionArtifactCanonicalSchema  = "project-typeenv-extension-artifact.v1"
	legacyRuntimeBasisCanonicalSchema = "runtime-evaluation-basis-artifact.v1"
	runtimeBasisCanonicalSchema       = "runtime-evaluation-basis-artifact.v2"
	compositeArtifactCanonicalSchema  = "project-typeenv-composite-artifact.v1"
)

var (
	ErrStoreRequired               = errors.New("project TypeEnv artifact store is required")
	ErrContextRequired             = errors.New("project TypeEnv artifact store context is required")
	ErrArtifactNotFound            = errors.New("project TypeEnv artifact is not found")
	ErrArtifactConflict            = errors.New("project TypeEnv artifact coordinate conflicts with stored bytes")
	ErrArtifactIntegrity           = errors.New("project TypeEnv artifact integrity check failed")
	ErrBaseNotExecutable           = errors.New("base TypeEnv artifact is not executable")
	ErrClosureInconsistent         = errors.New("project TypeEnv artifact closure is inconsistent")
	ErrRuntimeClosureRequired      = errors.New("runtime evaluation basis requires exact runtime mechanism artifacts")
	ErrRuntimeBasisRebuildRequired = errors.New("legacy runtime evaluation basis requires rebuild")
)

// ArtifactKind is the closed storage stratum. It is a persistence kind, not an
// FPF U.Kind and not a project-memory node kind.
type ArtifactKind string

const (
	ArtifactBaseTypeEnv      ArtifactKind = "base_type_env"
	ArtifactExtensionTypeEnv ArtifactKind = "project_type_env_extension"
	ArtifactRuntimeBasis     ArtifactKind = "runtime_evaluation_basis"
	ArtifactCompositeTypeEnv ArtifactKind = "project_type_env_composite"
)

func (kind ArtifactKind) valid() bool {
	switch kind {
	case ArtifactBaseTypeEnv,
		ArtifactExtensionTypeEnv,
		ArtifactRuntimeBasis,
		ArtifactCompositeTypeEnv:
		return true
	default:
		return false
	}
}

type artifactRecord struct {
	kind            ArtifactKind
	ref             string
	digest          string
	canonicalSchema string
	producerSchema  string
	canonical       []byte
}

func (record artifactRecord) clone() artifactRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record artifactRecord) metadataEqual(other artifactRecord) bool {
	return record.kind == other.kind &&
		record.ref == other.ref &&
		record.digest == other.digest &&
		record.canonicalSchema == other.canonicalSchema &&
		record.producerSchema == other.producerSchema
}

func (record artifactRecord) exactEqual(other artifactRecord) bool {
	return record.metadataEqual(other) && bytes.Equal(record.canonical, other.canonical)
}

func prepareBaseArtifact(
	artifact typeenv.BaseTypeEnvArtifact,
) (artifactRecord, typeenv.BaseTypeEnvArtifact, error) {
	if err := artifact.Verify(); err != nil {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			"",
			fmt.Errorf("verify supplied base artifact: %w", err),
		)
	}
	canonical := artifact.CanonicalBytes()
	decoded, err := typeenv.DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			"",
			fmt.Errorf("decode supplied base artifact: %w", err),
		)
	}
	ref, executable := decoded.TypeEnvRef()
	if !executable {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"%w: coverage-only B has no exact TypeEnvRef",
			ErrBaseNotExecutable,
		)
	}
	if decoded.Posture() != typeenv.CompiledEnvironment {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, fmt.Errorf(
			"%w: B posture is %q",
			ErrBaseNotExecutable,
			decoded.Posture().String(),
		)
	}
	if !bytes.Equal(canonical, decoded.CanonicalBytes()) ||
		artifact.Digest() != decoded.Digest() {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			ref.String(),
			fmt.Errorf("supplied base metadata does not match canonical bytes"),
		)
	}
	suppliedRef, suppliedHasRef := artifact.TypeEnvRef()
	if !suppliedHasRef || suppliedRef != ref ||
		artifact.CompilerSchemaVersion() != decoded.CompilerSchemaVersion() {
		return artifactRecord{}, typeenv.BaseTypeEnvArtifact{}, integrityError(
			ArtifactBaseTypeEnv,
			ref.String(),
			fmt.Errorf("supplied base identity or compiler schema does not match canonical bytes"),
		)
	}
	record := artifactRecord{
		kind:            ArtifactBaseTypeEnv,
		ref:             ref.String(),
		digest:          decoded.Digest().String(),
		canonicalSchema: baseArtifactCanonicalSchema,
		producerSchema:  decoded.CompilerSchemaVersion().String(),
		canonical:       decoded.CanonicalBytes(),
	}
	return record, decoded, nil
}

func prepareExtensionArtifact(
	artifact projecttypeenv.ProjectTypeEnvExtensionArtifact,
) (artifactRecord, projecttypeenv.ProjectTypeEnvExtensionArtifact, error) {
	if err := artifact.Verify(); err != nil {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvExtensionArtifact{}, integrityError(
			ArtifactExtensionTypeEnv,
			"",
			fmt.Errorf("verify supplied extension artifact: %w", err),
		)
	}
	canonical := artifact.CanonicalBytes()
	decoded, err := projecttypeenv.DecodeProjectTypeEnvExtensionArtifact(canonical)
	if err != nil {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvExtensionArtifact{}, integrityError(
			ArtifactExtensionTypeEnv,
			"",
			fmt.Errorf("decode supplied extension artifact: %w", err),
		)
	}
	ref := decoded.Ref()
	if ref != artifact.Ref() || decoded.Digest() != artifact.Digest() ||
		decoded.IR().CompilerVersion().Value() != artifact.IR().CompilerVersion().Value() ||
		!bytes.Equal(canonical, decoded.CanonicalBytes()) {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvExtensionArtifact{}, integrityError(
			ArtifactExtensionTypeEnv,
			ref.String(),
			fmt.Errorf("supplied extension metadata does not match canonical bytes"),
		)
	}
	record := artifactRecord{
		kind:            ArtifactExtensionTypeEnv,
		ref:             ref.String(),
		digest:          decoded.Digest().String(),
		canonicalSchema: extensionArtifactCanonicalSchema,
		producerSchema:  decoded.IR().CompilerVersion().Value(),
		canonical:       decoded.CanonicalBytes(),
	}
	return record, decoded, nil
}

func prepareRuntimeBasisArtifact(
	artifact projecttypeenv.RuntimeEvaluationBasisArtifact,
) (artifactRecord, projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	if err := artifact.Verify(); err != nil {
		return artifactRecord{}, projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			"",
			fmt.Errorf("verify supplied runtime basis artifact: %w", err),
		)
	}
	canonical := artifact.CanonicalBytes()
	decoded, err := projecttypeenv.DecodeRuntimeEvaluationBasisArtifact(canonical)
	if err != nil {
		return artifactRecord{}, projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			"",
			fmt.Errorf("decode supplied runtime basis artifact: %w", err),
		)
	}
	ref := decoded.Ref()
	if ref != artifact.Ref() || decoded.Digest() != artifact.Digest() ||
		!bytes.Equal(canonical, decoded.CanonicalBytes()) {
		return artifactRecord{}, projecttypeenv.RuntimeEvaluationBasisArtifact{}, integrityError(
			ArtifactRuntimeBasis,
			ref.String(),
			fmt.Errorf("supplied runtime basis metadata does not match canonical bytes"),
		)
	}
	record := artifactRecord{
		kind:            ArtifactRuntimeBasis,
		ref:             ref.String(),
		digest:          decoded.Digest().String(),
		canonicalSchema: runtimeBasisCanonicalSchema,
		producerSchema:  runtimeBasisCanonicalSchema,
		canonical:       decoded.CanonicalBytes(),
	}
	return record, decoded, nil
}

func prepareCompositeArtifact(
	artifact projecttypeenv.ProjectTypeEnvCompositeArtifact,
) (artifactRecord, projecttypeenv.ProjectTypeEnvCompositeArtifact, error) {
	if err := artifact.Verify(); err != nil {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvCompositeArtifact{}, integrityError(
			ArtifactCompositeTypeEnv,
			"",
			fmt.Errorf("verify supplied composite artifact: %w", err),
		)
	}
	canonical := artifact.CanonicalBytes()
	decoded, err := projecttypeenv.DecodeProjectTypeEnvCompositeArtifact(canonical)
	if err != nil {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvCompositeArtifact{}, integrityError(
			ArtifactCompositeTypeEnv,
			"",
			fmt.Errorf("decode supplied composite artifact: %w", err),
		)
	}
	ref := decoded.Ref()
	if ref != artifact.Ref() || decoded.Digest() != artifact.Digest() ||
		decoded.LowererSchemaVersion() != artifact.LowererSchemaVersion() ||
		!bytes.Equal(canonical, decoded.CanonicalBytes()) {
		return artifactRecord{}, projecttypeenv.ProjectTypeEnvCompositeArtifact{}, integrityError(
			ArtifactCompositeTypeEnv,
			ref.String(),
			fmt.Errorf("supplied composite metadata does not match canonical bytes"),
		)
	}
	record := artifactRecord{
		kind:            ArtifactCompositeTypeEnv,
		ref:             ref.String(),
		digest:          decoded.Digest().String(),
		canonicalSchema: compositeArtifactCanonicalSchema,
		producerSchema:  decoded.LowererSchemaVersion(),
		canonical:       decoded.CanonicalBytes(),
	}
	return record, decoded, nil
}

func decodeStoredRecord(record artifactRecord) (any, error) {
	if !record.kind.valid() {
		return nil, fmt.Errorf("artifact kind %q is invalid", record.kind)
	}
	switch record.kind {
	case ArtifactBaseTypeEnv:
		decoded, err := typeenv.DecodeBaseTypeEnvArtifact(record.canonical)
		if err != nil {
			return nil, err
		}
		prepared, verified, err := prepareBaseArtifact(decoded)
		if err != nil {
			return nil, err
		}
		if !prepared.exactEqual(record) {
			return nil, fmt.Errorf("stored base metadata does not match canonical bytes")
		}
		return verified, nil
	case ArtifactExtensionTypeEnv:
		decoded, err := projecttypeenv.DecodeProjectTypeEnvExtensionArtifact(record.canonical)
		if err != nil {
			return nil, err
		}
		prepared, verified, err := prepareExtensionArtifact(decoded)
		if err != nil {
			return nil, err
		}
		if !prepared.exactEqual(record) {
			return nil, fmt.Errorf("stored extension metadata does not match canonical bytes")
		}
		return verified, nil
	case ArtifactRuntimeBasis:
		if record.canonicalSchema == legacyRuntimeBasisCanonicalSchema {
			return nil, legacyRuntimeBasisRebuildError(record)
		}
		decoded, err := projecttypeenv.DecodeRuntimeEvaluationBasisArtifact(record.canonical)
		if err != nil {
			return nil, err
		}
		prepared, verified, err := prepareRuntimeBasisArtifact(decoded)
		if err != nil {
			return nil, err
		}
		if !prepared.exactEqual(record) {
			return nil, fmt.Errorf("stored runtime basis metadata does not match canonical bytes")
		}
		return verified, nil
	case ArtifactCompositeTypeEnv:
		decoded, err := projecttypeenv.DecodeProjectTypeEnvCompositeArtifact(record.canonical)
		if err != nil {
			return nil, err
		}
		prepared, verified, err := prepareCompositeArtifact(decoded)
		if err != nil {
			return nil, err
		}
		if !prepared.exactEqual(record) {
			return nil, fmt.Errorf("stored composite metadata does not match canonical bytes")
		}
		return verified, nil
	default:
		return nil, fmt.Errorf("artifact kind %q is unsupported", record.kind)
	}
}

func legacyRuntimeBasisRebuildError(record artifactRecord) error {
	if record.producerSchema != legacyRuntimeBasisCanonicalSchema {
		return fmt.Errorf(
			"legacy runtime basis producer schema is %q; want %q",
			record.producerSchema,
			legacyRuntimeBasisCanonicalSchema,
		)
	}
	ref, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(record.ref)
	if err != nil {
		return fmt.Errorf("legacy runtime basis reference: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(record.digest)
	if err != nil {
		return fmt.Errorf("legacy runtime basis digest: %w", err)
	}
	sum := sha256.Sum256(record.canonical)
	hexDigest := hex.EncodeToString(sum[:])
	expectedDigest, err := typedmemory.NewSHA256Digest("sha256:" + hexDigest)
	if err != nil {
		return fmt.Errorf("derive legacy runtime basis digest: %w", err)
	}
	if ref.Digest() != digest || digest != expectedDigest {
		return fmt.Errorf(
			"legacy runtime basis metadata does not match its preserved canonical bytes",
		)
	}
	return fmt.Errorf(
		"%w: preserved X %q uses %q without an independently supplied exact registration policy; rebuild X and every bound C",
		ErrRuntimeBasisRebuildRequired,
		record.ref,
		legacyRuntimeBasisCanonicalSchema,
	)
}

func validateTypeEnvRef(ref typedmemory.TypeEnvRef) error {
	parsed, err := typedmemory.ParseTypeEnvRef(ref.String())
	if err != nil || parsed != ref {
		return fmt.Errorf("exact TypeEnvRef is required")
	}
	return nil
}

func validateExtensionRef(ref typedmemory.TypeEnvExtensionRef) error {
	parsed, err := typedmemory.ParseTypeEnvExtensionRef(ref.String())
	if err != nil || parsed != ref {
		return fmt.Errorf("exact TypeEnvExtensionRef is required")
	}
	return nil
}

func validateRuntimeBasisRef(ref projecttypeenv.RuntimeEvaluationBasisRef) error {
	parsed, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(ref.String())
	if err != nil || parsed != ref {
		return fmt.Errorf("exact RuntimeEvaluationBasisRef is required")
	}
	return nil
}

func integrityError(kind ArtifactKind, ref string, cause error) error {
	return fmt.Errorf("%w: %s %q: %v", ErrArtifactIntegrity, kind, ref, cause)
}

func storedRecordReadError(kind ArtifactKind, ref string, cause error) error {
	if errors.Is(cause, ErrRuntimeBasisRebuildRequired) {
		return cause
	}
	return integrityError(kind, ref, cause)
}
