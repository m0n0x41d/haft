package projecttypeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvExecutableSnapshotDomain = "haft.fpf.projecttypeenv.executable-snapshot.v1"
	ProjectTypeEnvExecutableSnapshotSchema = "haft.fpf.projecttypeenv.executable-snapshot/v1"

	maximumProjectTypeEnvExecutableSnapshotBytes = 64 << 20
	maximumProjectTypeEnvSnapshotExtensions      = 8 << 10
)

// ProjectTypeEnvExecutableSnapshotRecord is immutable data. It binds the
// exact reconstructable B/E/X/C closure, linked IR, final-lowerer receipt, and
// the semantic lowered-environment digest view. Decoding this record does not
// mint a final-lowerer or Stage capability.
type ProjectTypeEnvExecutableSnapshotRecord struct {
	typeEnvRef                   typedmemory.TypeEnvRef
	digest                       typedmemory.SHA256Digest
	loweredEnvironmentDigest     typedmemory.SHA256Digest
	sourceRevision               typedmemory.SourceRevision
	compilerSchemaVersion        typedmemory.CompilerSchemaVersion
	lowererSchemaVersion         string
	verificationRef              ProjectTypeEnvCompositeVerificationRef
	verificationCanonical        []byte
	loweredEnvironmentCanonical  []byte
	baseRef                      typedmemory.TypeEnvRef
	baseCanonical                []byte
	extensionRefs                []typedmemory.TypeEnvExtensionRef
	extensionCanonicals          [][]byte
	runtimeBasisRef              RuntimeEvaluationBasisRef
	runtimeBasisCanonical        []byte
	runtimeMechanismCanonicals   [][]byte
	registrationPolicyCanonicals [][]byte
	compositeCanonical           []byte
	linkedCanonical              []byte
	canonical                    []byte
}

func (record ProjectTypeEnvExecutableSnapshotRecord) TypeEnvRef() typedmemory.TypeEnvRef {
	return record.typeEnvRef
}

func (record ProjectTypeEnvExecutableSnapshotRecord) Digest() typedmemory.SHA256Digest {
	return record.digest
}

func (record ProjectTypeEnvExecutableSnapshotRecord) LoweredEnvironmentDigest() typedmemory.SHA256Digest {
	return record.loweredEnvironmentDigest
}

func (record ProjectTypeEnvExecutableSnapshotRecord) SourceRevision() typedmemory.SourceRevision {
	return record.sourceRevision
}

func (record ProjectTypeEnvExecutableSnapshotRecord) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return record.compilerSchemaVersion
}

func (record ProjectTypeEnvExecutableSnapshotRecord) LowererSchemaVersion() string {
	return record.lowererSchemaVersion
}

func (record ProjectTypeEnvExecutableSnapshotRecord) VerificationRef() ProjectTypeEnvCompositeVerificationRef {
	return record.verificationRef
}

func (record ProjectTypeEnvExecutableSnapshotRecord) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return record.baseRef
}

func (record ProjectTypeEnvExecutableSnapshotRecord) ExtensionRefs() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), record.extensionRefs...)
}

func (record ProjectTypeEnvExecutableSnapshotRecord) RuntimeEvaluationBasisRef() RuntimeEvaluationBasisRef {
	return record.runtimeBasisRef
}

func (record ProjectTypeEnvExecutableSnapshotRecord) CanonicalBytes() []byte {
	return append([]byte(nil), record.canonical...)
}

func (record ProjectTypeEnvExecutableSnapshotRecord) Verify() error {
	decoded, err := DecodeProjectTypeEnvExecutableSnapshotRecord(record.canonical)
	if err != nil {
		return fmt.Errorf("verify project TypeEnv executable snapshot record: %w", err)
	}
	if !projectTypeEnvExecutableSnapshotRecordsEqual(decoded, record) {
		return fmt.Errorf("project TypeEnv executable snapshot fields do not match canonical bytes")
	}
	return nil
}

// ProjectTypeEnvExecutableSnapshot carries the exact reconstructed executable
// TypeEnv. It is emitted by final lowering or restored only by replaying final
// lowering against the record's exact closure.
type ProjectTypeEnvExecutableSnapshot struct {
	record      ProjectTypeEnvExecutableSnapshotRecord
	environment typedmemory.TypeEnv
}

func (snapshot ProjectTypeEnvExecutableSnapshot) Record() ProjectTypeEnvExecutableSnapshotRecord {
	return cloneProjectTypeEnvExecutableSnapshotRecord(snapshot.record)
}

func (snapshot ProjectTypeEnvExecutableSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.record.TypeEnvRef()
}

func (snapshot ProjectTypeEnvExecutableSnapshot) Digest() typedmemory.SHA256Digest {
	return snapshot.record.Digest()
}

func (snapshot ProjectTypeEnvExecutableSnapshot) LoweredEnvironmentDigest() typedmemory.SHA256Digest {
	return snapshot.record.LoweredEnvironmentDigest()
}

func (snapshot ProjectTypeEnvExecutableSnapshot) Environment() typedmemory.TypeEnv {
	return snapshot.environment
}

func (snapshot ProjectTypeEnvExecutableSnapshot) Verify() error {
	if err := snapshot.record.Verify(); err != nil {
		return err
	}
	if snapshot.environment.Ref() != snapshot.record.TypeEnvRef() {
		return fmt.Errorf("executable snapshot TypeEnvRef differs from C")
	}
	if snapshot.environment.SourceRevision() != snapshot.record.SourceRevision() {
		return fmt.Errorf("executable snapshot source revision differs from record")
	}
	if snapshot.environment.CompilerSchemaVersion() != snapshot.record.CompilerSchemaVersion() {
		return fmt.Errorf("executable snapshot compiler schema differs from record")
	}
	canonical, err := projectTypeEnvLoweredEnvironmentCanonical(
		snapshot.environment,
		snapshot.record.LowererSchemaVersion(),
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, snapshot.record.loweredEnvironmentCanonical) {
		return fmt.Errorf("executable snapshot environment differs from exact lowered bytes")
	}
	digest, err := digestProjectTypeEnvExecutableSnapshotBytes(canonical)
	if err != nil {
		return err
	}
	if digest != snapshot.record.LoweredEnvironmentDigest() {
		return fmt.Errorf("executable snapshot lowered digest differs from final-lowerer digest")
	}
	return nil
}

type projectTypeEnvExecutableSnapshotMaterial struct {
	typeEnvRef                   typedmemory.TypeEnvRef
	loweredEnvironmentDigest     typedmemory.SHA256Digest
	sourceRevision               typedmemory.SourceRevision
	compilerSchemaVersion        typedmemory.CompilerSchemaVersion
	lowererSchemaVersion         string
	verificationRef              ProjectTypeEnvCompositeVerificationRef
	verificationCanonical        []byte
	loweredEnvironmentCanonical  []byte
	baseRef                      typedmemory.TypeEnvRef
	baseCanonical                []byte
	extensionRefs                []typedmemory.TypeEnvExtensionRef
	extensionCanonicals          [][]byte
	runtimeBasisRef              RuntimeEvaluationBasisRef
	runtimeBasisCanonical        []byte
	runtimeMechanismCanonicals   [][]byte
	registrationPolicyCanonicals [][]byte
	compositeCanonical           []byte
	linkedCanonical              []byte
}

func sealProjectTypeEnvExecutableSnapshot(
	input ProjectTypeEnvCompositePreparationInput,
	environment typedmemory.TypeEnv,
	verification ProjectTypeEnvCompositeVerification,
) (ProjectTypeEnvExecutableSnapshot, error) {
	if err := verification.Verify(); err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, err
	}
	loweredCanonical, err := projectTypeEnvLoweredEnvironmentCanonical(
		environment,
		input.Composite.LowererSchemaVersion(),
	)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, err
	}
	loweredDigest, err := digestProjectTypeEnvExecutableSnapshotBytes(loweredCanonical)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, err
	}
	if loweredDigest != verification.LoweredEnvironmentDigest() {
		return ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"exact lowered bytes differ from final-lowerer verification digest",
		)
	}
	extensions := input.Linked.Extensions()
	extensionRefs := make([]typedmemory.TypeEnvExtensionRef, 0, len(extensions))
	extensionCanonicals := make([][]byte, 0, len(extensions))
	for _, extension := range extensions {
		artifact := extension.Artifact()
		extensionRefs = append(extensionRefs, artifact.Ref())
		extensionCanonicals = append(extensionCanonicals, artifact.CanonicalBytes())
	}
	material := projectTypeEnvExecutableSnapshotMaterial{
		typeEnvRef:                  environment.Ref(),
		loweredEnvironmentDigest:    loweredDigest,
		sourceRevision:              environment.SourceRevision(),
		compilerSchemaVersion:       environment.CompilerSchemaVersion(),
		lowererSchemaVersion:        input.Composite.LowererSchemaVersion(),
		verificationRef:             verification.Ref(),
		verificationCanonical:       verification.CanonicalBytes(),
		loweredEnvironmentCanonical: loweredCanonical,
		baseRef:                     input.Linked.BaseTypeEnvRef(),
		baseCanonical:               input.Base.CanonicalBytes(),
		extensionRefs:               extensionRefs,
		extensionCanonicals:         extensionCanonicals,
		runtimeBasisRef:             input.RuntimeBasis.Ref(),
		runtimeBasisCanonical:       input.RuntimeBasis.CanonicalBytes(),
		runtimeMechanismCanonicals: cloneProjectTypeEnvSnapshotBytes(
			input.RuntimeBasis.resolvedMechanisms,
		),
		registrationPolicyCanonicals: cloneProjectTypeEnvSnapshotBytes(
			input.RuntimeBasis.resolvedRegistrationPolicies,
		),
		compositeCanonical: input.Composite.CanonicalBytes(),
		linkedCanonical:    input.Linked.CanonicalBytes(),
	}
	canonical, err := encodeProjectTypeEnvExecutableSnapshot(material)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, err
	}
	record, err := DecodeProjectTypeEnvExecutableSnapshotRecord(canonical)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"reseal project TypeEnv executable snapshot: %w",
			err,
		)
	}
	snapshot := ProjectTypeEnvExecutableSnapshot{
		record:      record,
		environment: environment,
	}
	if err := snapshot.Verify(); err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, err
	}
	return snapshot, nil
}

// DecodeProjectTypeEnvExecutableSnapshotRecord strictly decodes the immutable
// data record and all nested artifact codecs. It relinks B/E to validate the
// stored linker material, but deliberately does not run final lowering.
func DecodeProjectTypeEnvExecutableSnapshotRecord(
	canonical []byte,
) (ProjectTypeEnvExecutableSnapshotRecord, error) {
	material, err := decodeProjectTypeEnvExecutableSnapshot(canonical)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshotRecord{}, err
	}
	verified, err := verifyProjectTypeEnvExecutableSnapshotMaterial(material)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshotRecord{}, err
	}
	reencoded, err := encodeProjectTypeEnvExecutableSnapshot(verified.material)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshotRecord{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvExecutableSnapshotRecord{}, fmt.Errorf(
			"project TypeEnv executable snapshot is not canonical",
		)
	}
	digest, err := digestProjectTypeEnvExecutableSnapshotBytes(reencoded)
	if err != nil {
		return ProjectTypeEnvExecutableSnapshotRecord{}, err
	}
	return ProjectTypeEnvExecutableSnapshotRecord{
		typeEnvRef:                  verified.material.typeEnvRef,
		digest:                      digest,
		loweredEnvironmentDigest:    verified.material.loweredEnvironmentDigest,
		sourceRevision:              verified.material.sourceRevision,
		compilerSchemaVersion:       verified.material.compilerSchemaVersion,
		lowererSchemaVersion:        verified.material.lowererSchemaVersion,
		verificationRef:             verified.material.verificationRef,
		verificationCanonical:       append([]byte(nil), verified.material.verificationCanonical...),
		loweredEnvironmentCanonical: append([]byte(nil), verified.material.loweredEnvironmentCanonical...),
		baseRef:                     verified.material.baseRef,
		baseCanonical:               append([]byte(nil), verified.material.baseCanonical...),
		extensionRefs:               append([]typedmemory.TypeEnvExtensionRef(nil), verified.material.extensionRefs...),
		extensionCanonicals:         cloneProjectTypeEnvSnapshotBytes(verified.material.extensionCanonicals),
		runtimeBasisRef:             verified.material.runtimeBasisRef,
		runtimeBasisCanonical:       append([]byte(nil), verified.material.runtimeBasisCanonical...),
		runtimeMechanismCanonicals: cloneProjectTypeEnvSnapshotBytes(
			verified.material.runtimeMechanismCanonicals,
		),
		registrationPolicyCanonicals: cloneProjectTypeEnvSnapshotBytes(
			verified.material.registrationPolicyCanonicals,
		),
		compositeCanonical: append([]byte(nil), verified.material.compositeCanonical...),
		linkedCanonical:    append([]byte(nil), verified.material.linkedCanonical...),
		canonical:          append([]byte(nil), reencoded...),
	}, nil
}

// RestoreProjectTypeEnvExecutableSnapshot reruns the final lowerer against the
// caller-supplied exact closure and requires the newly emitted snapshot to
// byte-match the persisted record. Only this path recreates executable state.
func RestoreProjectTypeEnvExecutableSnapshot(
	record ProjectTypeEnvExecutableSnapshotRecord,
	input ProjectTypeEnvCompositePreparationInput,
) (
	ProjectTypeEnvExecutableSnapshot,
	ProjectTypeEnvCompositeVerification,
	error,
) {
	if err := record.Verify(); err != nil {
		return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{}, err
	}
	preparation := PrepareProjectTypeEnvComposite(input)
	if preparation.Rejected() {
		issues := preparation.Issues()
		if len(issues) == 0 {
			return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{},
				fmt.Errorf("final lowerer rejected executable snapshot restoration")
		}
		return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{},
			fmt.Errorf(
				"final lowerer rejected executable snapshot restoration: %s at %s: %s",
				issues[0].Code(),
				issues[0].Subject(),
				issues[0].Detail(),
			)
	}
	snapshot, exists := preparation.ExecutableSnapshot()
	if !exists {
		return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{},
			fmt.Errorf("final lowerer produced no executable snapshot")
	}
	verification, exists := preparation.Verification()
	if !exists {
		return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{},
			fmt.Errorf("final lowerer produced no verification capability")
	}
	if !projectTypeEnvExecutableSnapshotRecordsEqual(snapshot.record, record) {
		differences := projectTypeEnvExecutableSnapshotRecordDifferences(
			snapshot.record,
			record,
		)
		return ProjectTypeEnvExecutableSnapshot{}, ProjectTypeEnvCompositeVerification{},
			fmt.Errorf(
				"persisted executable snapshot does not byte-match final-lowerer result: %s",
				strings.Join(differences, ", "),
			)
	}
	return snapshot, verification, nil
}

type verifiedProjectTypeEnvExecutableSnapshotMaterial struct {
	material projectTypeEnvExecutableSnapshotMaterial
}

func verifyProjectTypeEnvExecutableSnapshotMaterial(
	material projectTypeEnvExecutableSnapshotMaterial,
) (verifiedProjectTypeEnvExecutableSnapshotMaterial, error) {
	base, err := typeenv.DecodeBaseTypeEnvArtifact(material.baseCanonical)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot B: %w",
			err,
		)
	}
	baseRef, exists := base.TypeEnvRef()
	if !exists || baseRef != material.baseRef {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot B coordinate differs from canonical artifact",
		)
	}
	extensions := make([]ProjectTypeEnvExtensionArtifact, 0, len(material.extensionCanonicals))
	for index, canonical := range material.extensionCanonicals {
		extension, decodeErr := DecodeProjectTypeEnvExtensionArtifact(canonical)
		if decodeErr != nil {
			return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot E[%d]: %w",
				index,
				decodeErr,
			)
		}
		if extension.Ref() != material.extensionRefs[index] {
			return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"executable snapshot E[%d] coordinate differs from canonical artifact",
				index,
			)
		}
		extensions = append(extensions, extension)
	}
	runtimeBasis, err := DecodeRuntimeEvaluationBasisArtifact(material.runtimeBasisCanonical)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot X: %w",
			err,
		)
	}
	if runtimeBasis.Ref() != material.runtimeBasisRef {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot X coordinate differs from canonical artifact",
		)
	}
	runtimeMechanisms, err := decodeResolvedRuntimeMechanisms(
		material.runtimeMechanismCanonicals,
	)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot X runtime mechanisms: %w",
			err,
		)
	}
	registrationPolicies, err := decodeResolvedRegistrationPolicies(
		material.registrationPolicyCanonicals,
	)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot X registration policies: %w",
			err,
		)
	}
	runtimeBasis, err = ResolveRuntimeEvaluationBasisClosure(
		runtimeBasis,
		runtimeMechanisms,
		registrationPolicies,
	)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"resolve executable snapshot X closure: %w",
			err,
		)
	}
	if err := runtimeBasis.VerifyResolvedClosure(); err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"verify executable snapshot X closure: %w",
			err,
		)
	}
	composite, err := DecodeProjectTypeEnvCompositeArtifact(material.compositeCanonical)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot C: %w",
			err,
		)
	}
	if composite.Ref() != material.typeEnvRef {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot TypeEnvRef differs from C",
		)
	}
	if composite.BaseTypeEnvRef() != material.baseRef ||
		!projectTypeEnvExtensionRefsEqual(composite.ExtensionRefs(), material.extensionRefs) ||
		composite.RuntimeEvaluationBasisRef() != material.runtimeBasisRef ||
		composite.LowererSchemaVersion() != material.lowererSchemaVersion {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot closure differs from C recipe",
		)
	}
	verification, err := VerifyProjectTypeEnvCompositeVerificationRecord(
		material.verificationRef,
		material.verificationCanonical,
	)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot final-lowerer record: %w",
			err,
		)
	}
	if verification.BaseTypeEnvRef() != material.baseRef ||
		!projectTypeEnvExtensionRefsEqual(verification.ExtensionRefs(), material.extensionRefs) ||
		verification.RuntimeEvaluationBasisRef() != material.runtimeBasisRef ||
		verification.CompositeRef() != material.typeEnvRef ||
		verification.LoweredEnvironmentRef() != material.typeEnvRef ||
		verification.LoweredEnvironmentDigest() != material.loweredEnvironmentDigest ||
		verification.LowererSchemaVersion() != material.lowererSchemaVersion {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot metadata differs from final-lowerer record",
		)
	}
	loweredDigest, err := digestProjectTypeEnvExecutableSnapshotBytes(
		material.loweredEnvironmentCanonical,
	)
	if err != nil {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, err
	}
	if loweredDigest != material.loweredEnvironmentDigest {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot lowered bytes differ from final-lowerer digest",
		)
	}
	if base.SourceRevision() != material.sourceRevision ||
		base.CompilerSchemaVersion() != material.compilerSchemaVersion {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot source or compiler edition differs from B",
		)
	}
	resolution := LinkProjectTypeEnvCompositeIR(base, extensions)
	if resolution.Rejected() {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"relink executable snapshot B/E: %v",
			resolution.Issues(),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"relinked executable snapshot has no linked IR",
		)
	}
	if !bytes.Equal(linked.CanonicalBytes(), material.linkedCanonical) {
		return verifiedProjectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot linked IR differs from exact B/E closure",
		)
	}
	normalized := material
	normalized.baseCanonical = base.CanonicalBytes()
	normalized.extensionCanonicals = make([][]byte, 0, len(extensions))
	for _, extension := range extensions {
		normalized.extensionCanonicals = append(
			normalized.extensionCanonicals,
			extension.CanonicalBytes(),
		)
	}
	normalized.runtimeBasisCanonical = runtimeBasis.CanonicalBytes()
	normalized.runtimeMechanismCanonicals = cloneProjectTypeEnvSnapshotBytes(
		runtimeBasis.resolvedMechanisms,
	)
	normalized.registrationPolicyCanonicals = cloneProjectTypeEnvSnapshotBytes(
		runtimeBasis.resolvedRegistrationPolicies,
	)
	normalized.compositeCanonical = composite.CanonicalBytes()
	normalized.verificationCanonical = verification.CanonicalBytes()
	normalized.linkedCanonical = linked.CanonicalBytes()
	return verifiedProjectTypeEnvExecutableSnapshotMaterial{material: normalized}, nil
}

func encodeProjectTypeEnvExecutableSnapshot(
	material projectTypeEnvExecutableSnapshotMaterial,
) ([]byte, error) {
	if len(material.extensionRefs) != len(material.extensionCanonicals) {
		return nil, fmt.Errorf("executable snapshot E refs and payloads differ in length")
	}
	if len(material.extensionRefs) > maximumProjectTypeEnvSnapshotExtensions {
		return nil, fmt.Errorf(
			"executable snapshot E count exceeds %d",
			maximumProjectTypeEnvSnapshotExtensions,
		)
	}
	if len(material.runtimeMechanismCanonicals) > maximumRuntimeEvaluationBasisPins {
		return nil, fmt.Errorf(
			"executable snapshot runtime mechanism count exceeds %d",
			maximumRuntimeEvaluationBasisPins,
		)
	}
	if len(material.registrationPolicyCanonicals) > maximumRuntimeEvaluationBasisPins {
		return nil, fmt.Errorf(
			"executable snapshot registration-policy count exceeds %d",
			maximumRuntimeEvaluationBasisPins,
		)
	}
	writer := newProjectTypeEnvExecutableSnapshotWriter()
	writer.addString(projectTypeEnvExecutableSnapshotDomain)
	writer.addString(ProjectTypeEnvExecutableSnapshotSchema)
	writer.addString(material.typeEnvRef.String())
	writer.addString(material.loweredEnvironmentDigest.String())
	writer.addString(material.sourceRevision.String())
	writer.addString(material.compilerSchemaVersion.String())
	writer.addString(material.lowererSchemaVersion)
	writer.addString(material.verificationRef.String())
	writer.addBytes(material.verificationCanonical)
	writer.addBytes(material.loweredEnvironmentCanonical)
	writer.addString(material.baseRef.String())
	writer.addBytes(material.baseCanonical)
	writer.addUint64(uint64(len(material.extensionRefs)))
	for index, ref := range material.extensionRefs {
		writer.addString(ref.String())
		writer.addBytes(material.extensionCanonicals[index])
	}
	writer.addString(material.runtimeBasisRef.String())
	writer.addBytes(material.runtimeBasisCanonical)
	writer.addUint64(uint64(len(material.runtimeMechanismCanonicals)))
	for _, canonical := range material.runtimeMechanismCanonicals {
		writer.addBytes(canonical)
	}
	writer.addUint64(uint64(len(material.registrationPolicyCanonicals)))
	for _, canonical := range material.registrationPolicyCanonicals {
		writer.addBytes(canonical)
	}
	writer.addBytes(material.compositeCanonical)
	writer.addBytes(material.linkedCanonical)
	return writer.bytes()
}

func decodeProjectTypeEnvExecutableSnapshot(
	canonical []byte,
) (projectTypeEnvExecutableSnapshotMaterial, error) {
	reader, err := newProjectTypeEnvExecutableSnapshotReader(canonical)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	domain, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode project TypeEnv executable snapshot domain: %w",
			err,
		)
	}
	if domain != projectTypeEnvExecutableSnapshotDomain {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"unexpected project TypeEnv executable snapshot domain %q",
			domain,
		)
	}
	schema, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode project TypeEnv executable snapshot schema: %w",
			err,
		)
	}
	if schema != ProjectTypeEnvExecutableSnapshotSchema {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"unexpected project TypeEnv executable snapshot schema %q",
			schema,
		)
	}
	typeEnvRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	loweredDigestRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	loweredDigest, err := typedmemory.NewSHA256Digest(loweredDigestRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	sourceRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	source, err := typedmemory.NewSourceRevision(sourceRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	compilerRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(compilerRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	lowerer, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot lowerer schema: %w",
			err,
		)
	}
	if lowerer == "" {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"decode executable snapshot lowerer schema: value is required",
		)
	}
	verificationRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	verificationRef, err := ParseProjectTypeEnvCompositeVerificationRef(verificationRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	verificationCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	loweredCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	baseRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	baseRef, err := typedmemory.ParseTypeEnvRef(baseRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	baseCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	extensionCount, err := reader.readUint64()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	if extensionCount > maximumProjectTypeEnvSnapshotExtensions {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot E count exceeds %d",
			maximumProjectTypeEnvSnapshotExtensions,
		)
	}
	extensionRefs := make([]typedmemory.TypeEnvExtensionRef, 0, int(extensionCount))
	extensionCanonicals := make([][]byte, 0, int(extensionCount))
	for index := uint64(0); index < extensionCount; index++ {
		refRaw, readErr := reader.readString()
		if readErr != nil {
			return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot E[%d] ref: %w",
				index,
				readErr,
			)
		}
		ref, parseErr := typedmemory.ParseTypeEnvExtensionRef(refRaw)
		if parseErr != nil {
			return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot E[%d] ref: %w",
				index,
				parseErr,
			)
		}
		payload, readErr := reader.readBytes()
		if readErr != nil {
			return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot E[%d] payload: %w",
				index,
				readErr,
			)
		}
		extensionRefs = append(extensionRefs, ref)
		extensionCanonicals = append(extensionCanonicals, payload)
	}
	runtimeRaw, err := reader.readString()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	runtimeRef, err := ParseRuntimeEvaluationBasisRef(runtimeRaw)
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	runtimeCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	runtimeMechanismCount, err := reader.readUint64()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	if runtimeMechanismCount > maximumRuntimeEvaluationBasisPins {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot runtime mechanism count exceeds %d",
			maximumRuntimeEvaluationBasisPins,
		)
	}
	runtimeMechanismCanonicals := make([][]byte, 0, int(runtimeMechanismCount))
	for index := uint64(0); index < runtimeMechanismCount; index++ {
		payload, readErr := reader.readBytes()
		if readErr != nil {
			return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot runtime mechanism %d: %w",
				index,
				readErr,
			)
		}
		runtimeMechanismCanonicals = append(runtimeMechanismCanonicals, payload)
	}
	registrationPolicyCount, err := reader.readUint64()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	if registrationPolicyCount > maximumRuntimeEvaluationBasisPins {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"executable snapshot registration-policy count exceeds %d",
			maximumRuntimeEvaluationBasisPins,
		)
	}
	registrationPolicyCanonicals := make([][]byte, 0, int(registrationPolicyCount))
	for index := uint64(0); index < registrationPolicyCount; index++ {
		payload, readErr := reader.readBytes()
		if readErr != nil {
			return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
				"decode executable snapshot registration policy %d: %w",
				index,
				readErr,
			)
		}
		registrationPolicyCanonicals = append(registrationPolicyCanonicals, payload)
	}
	compositeCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	linkedCanonical, err := reader.readBytes()
	if err != nil {
		return projectTypeEnvExecutableSnapshotMaterial{}, err
	}
	if !reader.done() {
		return projectTypeEnvExecutableSnapshotMaterial{}, fmt.Errorf(
			"project TypeEnv executable snapshot has %d trailing bytes",
			reader.remaining(),
		)
	}
	return projectTypeEnvExecutableSnapshotMaterial{
		typeEnvRef:                   typeEnvRef,
		loweredEnvironmentDigest:     loweredDigest,
		sourceRevision:               source,
		compilerSchemaVersion:        compiler,
		lowererSchemaVersion:         lowerer,
		verificationRef:              verificationRef,
		verificationCanonical:        verificationCanonical,
		loweredEnvironmentCanonical:  loweredCanonical,
		baseRef:                      baseRef,
		baseCanonical:                baseCanonical,
		extensionRefs:                extensionRefs,
		extensionCanonicals:          extensionCanonicals,
		runtimeBasisRef:              runtimeRef,
		runtimeBasisCanonical:        runtimeCanonical,
		runtimeMechanismCanonicals:   runtimeMechanismCanonicals,
		registrationPolicyCanonicals: registrationPolicyCanonicals,
		compositeCanonical:           compositeCanonical,
		linkedCanonical:              linkedCanonical,
	}, nil
}

type projectTypeEnvExecutableSnapshotWriter struct {
	buffer bytes.Buffer
	err    error
}

func newProjectTypeEnvExecutableSnapshotWriter() *projectTypeEnvExecutableSnapshotWriter {
	return &projectTypeEnvExecutableSnapshotWriter{}
}

func (writer *projectTypeEnvExecutableSnapshotWriter) addString(value string) {
	if writer.err != nil {
		return
	}
	if !utf8.ValidString(value) {
		writer.err = fmt.Errorf("executable snapshot string contains invalid UTF-8")
		return
	}
	writer.addBytes([]byte(value))
}

func (writer *projectTypeEnvExecutableSnapshotWriter) addBytes(value []byte) {
	if writer.err != nil {
		return
	}
	next := writer.buffer.Len() + 8 + len(value)
	if next > maximumProjectTypeEnvExecutableSnapshotBytes {
		writer.err = fmt.Errorf(
			"project TypeEnv executable snapshot exceeds %d bytes",
			maximumProjectTypeEnvExecutableSnapshotBytes,
		)
		return
	}
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.buffer.Write(length[:])
	writer.buffer.Write(value)
}

func (writer *projectTypeEnvExecutableSnapshotWriter) addUint64(value uint64) {
	if writer.err != nil {
		return
	}
	if writer.buffer.Len()+8 > maximumProjectTypeEnvExecutableSnapshotBytes {
		writer.err = fmt.Errorf(
			"project TypeEnv executable snapshot exceeds %d bytes",
			maximumProjectTypeEnvExecutableSnapshotBytes,
		)
		return
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	writer.buffer.Write(encoded[:])
}

func (writer *projectTypeEnvExecutableSnapshotWriter) bytes() ([]byte, error) {
	if writer == nil {
		return nil, fmt.Errorf("project TypeEnv executable snapshot writer is required")
	}
	if writer.err != nil {
		return nil, writer.err
	}
	if writer.buffer.Len() == 0 {
		return nil, fmt.Errorf("project TypeEnv executable snapshot is empty")
	}
	return append([]byte(nil), writer.buffer.Bytes()...), nil
}

type projectTypeEnvExecutableSnapshotReader struct {
	data   []byte
	offset int
}

func newProjectTypeEnvExecutableSnapshotReader(
	canonical []byte,
) (*projectTypeEnvExecutableSnapshotReader, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("project TypeEnv executable snapshot bytes are required")
	}
	if len(canonical) > maximumProjectTypeEnvExecutableSnapshotBytes {
		return nil, fmt.Errorf(
			"project TypeEnv executable snapshot exceeds %d bytes",
			maximumProjectTypeEnvExecutableSnapshotBytes,
		)
	}
	return &projectTypeEnvExecutableSnapshotReader{
		data: append([]byte(nil), canonical...),
	}, nil
}

func (reader *projectTypeEnvExecutableSnapshotReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("executable snapshot string contains invalid UTF-8")
	}
	return string(value), nil
}

func (reader *projectTypeEnvExecutableSnapshotReader) readBytes() ([]byte, error) {
	length, err := reader.readUint64()
	if err != nil {
		return nil, err
	}
	remaining := reader.remaining()
	//nolint:gosec // remaining is non-negative after readUint64 validates the reader bounds.
	if length > uint64(remaining) {
		return nil, fmt.Errorf(
			"executable snapshot field length %d exceeds remaining %d",
			length,
			remaining,
		)
	}
	if length > maximumProjectTypeEnvExecutableSnapshotBytes {
		return nil, fmt.Errorf(
			"executable snapshot field length %d exceeds remaining %d",
			length,
			remaining,
		)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := append([]byte(nil), reader.data[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func (reader *projectTypeEnvExecutableSnapshotReader) readUint64() (uint64, error) {
	if reader == nil || reader.remaining() < 8 {
		return 0, fmt.Errorf("unexpected end of executable snapshot")
	}
	end := reader.offset + 8
	value := binary.BigEndian.Uint64(reader.data[reader.offset:end])
	reader.offset = end
	return value, nil
}

func (reader *projectTypeEnvExecutableSnapshotReader) done() bool {
	return reader != nil && reader.offset == len(reader.data)
}

func (reader *projectTypeEnvExecutableSnapshotReader) remaining() int {
	if reader == nil || reader.offset > len(reader.data) {
		return 0
	}
	return len(reader.data) - reader.offset
}

func digestProjectTypeEnvExecutableSnapshotBytes(
	value []byte,
) (typedmemory.SHA256Digest, error) {
	if len(value) == 0 {
		return typedmemory.SHA256Digest{}, fmt.Errorf("canonical bytes are required")
	}
	sum := sha256.Sum256(value)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
}

func projectTypeEnvExecutableSnapshotRecordsEqual(
	left ProjectTypeEnvExecutableSnapshotRecord,
	right ProjectTypeEnvExecutableSnapshotRecord,
) bool {
	return left.typeEnvRef == right.typeEnvRef &&
		left.digest == right.digest &&
		left.loweredEnvironmentDigest == right.loweredEnvironmentDigest &&
		left.sourceRevision == right.sourceRevision &&
		left.compilerSchemaVersion == right.compilerSchemaVersion &&
		left.lowererSchemaVersion == right.lowererSchemaVersion &&
		left.verificationRef == right.verificationRef &&
		bytes.Equal(left.verificationCanonical, right.verificationCanonical) &&
		bytes.Equal(left.loweredEnvironmentCanonical, right.loweredEnvironmentCanonical) &&
		left.baseRef == right.baseRef &&
		bytes.Equal(left.baseCanonical, right.baseCanonical) &&
		projectTypeEnvExtensionRefsEqual(left.extensionRefs, right.extensionRefs) &&
		projectTypeEnvSnapshotByteSetsEqual(left.extensionCanonicals, right.extensionCanonicals) &&
		left.runtimeBasisRef == right.runtimeBasisRef &&
		bytes.Equal(left.runtimeBasisCanonical, right.runtimeBasisCanonical) &&
		projectTypeEnvSnapshotByteSetsEqual(
			left.runtimeMechanismCanonicals,
			right.runtimeMechanismCanonicals,
		) &&
		projectTypeEnvSnapshotByteSetsEqual(
			left.registrationPolicyCanonicals,
			right.registrationPolicyCanonicals,
		) &&
		bytes.Equal(left.compositeCanonical, right.compositeCanonical) &&
		bytes.Equal(left.linkedCanonical, right.linkedCanonical) &&
		bytes.Equal(left.canonical, right.canonical)
}

func projectTypeEnvExecutableSnapshotRecordDifferences(
	actual ProjectTypeEnvExecutableSnapshotRecord,
	expected ProjectTypeEnvExecutableSnapshotRecord,
) []string {
	comparisons := []struct {
		name  string
		equal bool
	}{
		{name: "type_env_ref", equal: actual.typeEnvRef == expected.typeEnvRef},
		{name: "snapshot_digest", equal: actual.digest == expected.digest},
		{name: "lowered_environment_digest", equal: actual.loweredEnvironmentDigest == expected.loweredEnvironmentDigest},
		{name: "source_revision", equal: actual.sourceRevision == expected.sourceRevision},
		{name: "compiler_schema_version", equal: actual.compilerSchemaVersion == expected.compilerSchemaVersion},
		{name: "lowerer_schema_version", equal: actual.lowererSchemaVersion == expected.lowererSchemaVersion},
		{name: "verification_ref", equal: actual.verificationRef == expected.verificationRef},
		{name: "verification_canonical", equal: bytes.Equal(actual.verificationCanonical, expected.verificationCanonical)},
		{name: "lowered_environment_canonical", equal: bytes.Equal(actual.loweredEnvironmentCanonical, expected.loweredEnvironmentCanonical)},
		{name: "base_ref", equal: actual.baseRef == expected.baseRef},
		{name: "base_canonical", equal: bytes.Equal(actual.baseCanonical, expected.baseCanonical)},
		{name: "extension_refs", equal: projectTypeEnvExtensionRefsEqual(actual.extensionRefs, expected.extensionRefs)},
		{name: "extension_canonicals", equal: projectTypeEnvSnapshotByteSetsEqual(actual.extensionCanonicals, expected.extensionCanonicals)},
		{name: "runtime_basis_ref", equal: actual.runtimeBasisRef == expected.runtimeBasisRef},
		{name: "runtime_basis_canonical", equal: bytes.Equal(actual.runtimeBasisCanonical, expected.runtimeBasisCanonical)},
		{name: "runtime_mechanism_canonicals", equal: projectTypeEnvSnapshotByteSetsEqual(actual.runtimeMechanismCanonicals, expected.runtimeMechanismCanonicals)},
		{name: "registration_policy_canonicals", equal: projectTypeEnvSnapshotByteSetsEqual(actual.registrationPolicyCanonicals, expected.registrationPolicyCanonicals)},
		{name: "composite_canonical", equal: bytes.Equal(actual.compositeCanonical, expected.compositeCanonical)},
		{name: "linked_canonical", equal: bytes.Equal(actual.linkedCanonical, expected.linkedCanonical)},
		{name: "snapshot_canonical", equal: bytes.Equal(actual.canonical, expected.canonical)},
	}
	differences := make([]string, 0, len(comparisons))
	for _, comparison := range comparisons {
		if !comparison.equal {
			differences = append(differences, comparison.name)
		}
	}
	return differences
}

func cloneProjectTypeEnvExecutableSnapshotRecord(
	record ProjectTypeEnvExecutableSnapshotRecord,
) ProjectTypeEnvExecutableSnapshotRecord {
	result := record
	result.verificationCanonical = append([]byte(nil), record.verificationCanonical...)
	result.loweredEnvironmentCanonical = append(
		[]byte(nil),
		record.loweredEnvironmentCanonical...,
	)
	result.baseCanonical = append([]byte(nil), record.baseCanonical...)
	result.extensionRefs = append([]typedmemory.TypeEnvExtensionRef(nil), record.extensionRefs...)
	result.extensionCanonicals = cloneProjectTypeEnvSnapshotBytes(record.extensionCanonicals)
	result.runtimeBasisCanonical = append([]byte(nil), record.runtimeBasisCanonical...)
	result.runtimeMechanismCanonicals = cloneProjectTypeEnvSnapshotBytes(
		record.runtimeMechanismCanonicals,
	)
	result.registrationPolicyCanonicals = cloneProjectTypeEnvSnapshotBytes(
		record.registrationPolicyCanonicals,
	)
	result.compositeCanonical = append([]byte(nil), record.compositeCanonical...)
	result.linkedCanonical = append([]byte(nil), record.linkedCanonical...)
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func cloneProjectTypeEnvSnapshotBytes(values [][]byte) [][]byte {
	result := make([][]byte, 0, len(values))
	for _, value := range values {
		result = append(result, append([]byte(nil), value...))
	}
	return result
}

func projectTypeEnvSnapshotByteSetsEqual(left [][]byte, right [][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}
