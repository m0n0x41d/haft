package projecttypeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvCompositeVerificationDomain = "project-typeenv-composite-verification.v1"
	projectTypeEnvCompositeVerificationPrefix = "project-typeenv-composite-verification:"
	projectTypeEnvCompositeVerificationSchema = "haft.fpf.projecttypeenv.composite-verification/v1"
	projectTypeEnvCompositeVerificationResult = "runtime_closure_accepted"
	projectTypeEnvLoweredEnvironmentDomainV1  = "project-typeenv-lowered-environment.v1"
	projectTypeEnvLoweredEnvironmentDomainV2  = "project-typeenv-lowered-environment.v2"
)

// ProjectTypeEnvCompositeVerificationRef is the strong identity of an exact
// successful final-lowering witness. It is deliberately distinct from C:
// C identifies the recipe, while this ref identifies proof that the recipe
// was lowered and its executable runtime closure accepted.
type ProjectTypeEnvCompositeVerificationRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvCompositeVerificationRef(
	raw string,
) (ProjectTypeEnvCompositeVerificationRef, error) {
	if !strings.HasPrefix(raw, projectTypeEnvCompositeVerificationPrefix) {
		return ProjectTypeEnvCompositeVerificationRef{}, fmt.Errorf(
			"project TypeEnv composite verification ref must start with %q",
			projectTypeEnvCompositeVerificationPrefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(
		strings.TrimPrefix(raw, projectTypeEnvCompositeVerificationPrefix),
	)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRef{}, fmt.Errorf(
			"project TypeEnv composite verification digest: %w",
			err,
		)
	}
	return ProjectTypeEnvCompositeVerificationRef{digest: digest}, nil
}

func (ref ProjectTypeEnvCompositeVerificationRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvCompositeVerificationRef) String() string {
	return projectTypeEnvCompositeVerificationPrefix + ref.digest.String()
}

// ProjectTypeEnvCompositeVerificationRecord is the immutable persisted receipt
// of one successful final lowering. Decoding and inspecting a receipt does not
// recreate the non-serializable Stage capability.
type ProjectTypeEnvCompositeVerificationRecord struct {
	ref                      ProjectTypeEnvCompositeVerificationRef
	baseRef                  typedmemory.TypeEnvRef
	baseArtifactDigest       typedmemory.SHA256Digest
	extensionRefs            []typedmemory.TypeEnvExtensionRef
	runtimeBasisRef          RuntimeEvaluationBasisRef
	compositeRef             typedmemory.TypeEnvRef
	loweredEnvironmentRef    typedmemory.TypeEnvRef
	loweredEnvironmentDigest typedmemory.SHA256Digest
	lowererSchema            string
	verificationSchema       string
	verificationResult       string
	canonicalBytes           []byte
}

func (record ProjectTypeEnvCompositeVerificationRecord) Ref() ProjectTypeEnvCompositeVerificationRef {
	return record.ref
}

func (record ProjectTypeEnvCompositeVerificationRecord) Digest() typedmemory.SHA256Digest {
	return record.ref.Digest()
}

func (record ProjectTypeEnvCompositeVerificationRecord) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return record.baseRef
}

func (record ProjectTypeEnvCompositeVerificationRecord) BaseArtifactDigest() typedmemory.SHA256Digest {
	return record.baseArtifactDigest
}

func (record ProjectTypeEnvCompositeVerificationRecord) ExtensionRefs() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), record.extensionRefs...)
}

func (record ProjectTypeEnvCompositeVerificationRecord) RuntimeEvaluationBasisRef() RuntimeEvaluationBasisRef {
	return record.runtimeBasisRef
}

func (record ProjectTypeEnvCompositeVerificationRecord) CompositeRef() typedmemory.TypeEnvRef {
	return record.compositeRef
}

func (record ProjectTypeEnvCompositeVerificationRecord) LoweredEnvironmentRef() typedmemory.TypeEnvRef {
	return record.loweredEnvironmentRef
}

func (record ProjectTypeEnvCompositeVerificationRecord) LoweredEnvironmentDigest() typedmemory.SHA256Digest {
	return record.loweredEnvironmentDigest
}

func (record ProjectTypeEnvCompositeVerificationRecord) LowererSchemaVersion() string {
	return record.lowererSchema
}

func (record ProjectTypeEnvCompositeVerificationRecord) CanonicalBytes() []byte {
	return append([]byte(nil), record.canonicalBytes...)
}

func (record ProjectTypeEnvCompositeVerificationRecord) Verify() error {
	if len(record.canonicalBytes) == 0 {
		return fmt.Errorf("project TypeEnv composite verification record is empty")
	}
	decoded, err := DecodeProjectTypeEnvCompositeVerificationRecord(record.canonicalBytes)
	if err != nil {
		return fmt.Errorf("verify project TypeEnv composite verification record: %w", err)
	}
	if decoded.ref != record.ref ||
		decoded.baseRef != record.baseRef ||
		decoded.baseArtifactDigest != record.baseArtifactDigest ||
		!projectTypeEnvExtensionRefsEqual(decoded.extensionRefs, record.extensionRefs) ||
		decoded.runtimeBasisRef != record.runtimeBasisRef ||
		decoded.compositeRef != record.compositeRef ||
		decoded.loweredEnvironmentRef != record.loweredEnvironmentRef ||
		decoded.loweredEnvironmentDigest != record.loweredEnvironmentDigest ||
		decoded.lowererSchema != record.lowererSchema ||
		decoded.verificationSchema != record.verificationSchema ||
		decoded.verificationResult != record.verificationResult ||
		!bytes.Equal(decoded.canonicalBytes, record.canonicalBytes) {
		return fmt.Errorf("project TypeEnv composite verification record fields do not match canonical bytes")
	}
	return nil
}

type projectTypeEnvCompositeVerificationCapability struct{}

// ProjectTypeEnvCompositeVerification is a non-serializable in-process
// capability minted only after PrepareProjectTypeEnvComposite has reverified
// B/E/X/C, lowered C, and accepted the runtime closure. Stage may accept this
// type; it must never accept a decoded record as a substitute.
type ProjectTypeEnvCompositeVerification struct {
	record     ProjectTypeEnvCompositeVerificationRecord
	capability *projectTypeEnvCompositeVerificationCapability
}

func (artifact ProjectTypeEnvCompositeVerification) Ref() ProjectTypeEnvCompositeVerificationRef {
	return artifact.record.Ref()
}

func (artifact ProjectTypeEnvCompositeVerification) Record() ProjectTypeEnvCompositeVerificationRecord {
	return artifact.record
}

func (artifact ProjectTypeEnvCompositeVerification) Digest() typedmemory.SHA256Digest {
	return artifact.record.Digest()
}

func (artifact ProjectTypeEnvCompositeVerification) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return artifact.record.BaseTypeEnvRef()
}

func (artifact ProjectTypeEnvCompositeVerification) BaseArtifactDigest() typedmemory.SHA256Digest {
	return artifact.record.BaseArtifactDigest()
}

func (artifact ProjectTypeEnvCompositeVerification) ExtensionRefs() []typedmemory.TypeEnvExtensionRef {
	return artifact.record.ExtensionRefs()
}

func (artifact ProjectTypeEnvCompositeVerification) RuntimeEvaluationBasisRef() RuntimeEvaluationBasisRef {
	return artifact.record.RuntimeEvaluationBasisRef()
}

func (artifact ProjectTypeEnvCompositeVerification) CompositeRef() typedmemory.TypeEnvRef {
	return artifact.record.CompositeRef()
}

func (artifact ProjectTypeEnvCompositeVerification) LoweredEnvironmentRef() typedmemory.TypeEnvRef {
	return artifact.record.LoweredEnvironmentRef()
}

func (artifact ProjectTypeEnvCompositeVerification) LoweredEnvironmentDigest() typedmemory.SHA256Digest {
	return artifact.record.LoweredEnvironmentDigest()
}

func (artifact ProjectTypeEnvCompositeVerification) LowererSchemaVersion() string {
	return artifact.record.LowererSchemaVersion()
}

func (artifact ProjectTypeEnvCompositeVerification) CanonicalBytes() []byte {
	return artifact.record.CanonicalBytes()
}

func (artifact ProjectTypeEnvCompositeVerification) Verify() error {
	if artifact.capability == nil {
		return fmt.Errorf("project TypeEnv composite verification capability was not minted by final lowerer")
	}
	return artifact.record.Verify()
}

func (artifact ProjectTypeEnvCompositeVerification) validCapability() bool {
	return artifact.capability != nil && artifact.Verify() == nil
}

type projectTypeEnvCompositeVerificationCanonicalV1 struct {
	BaseTypeEnvRef           string   `json:"base_type_env_ref"`
	BaseArtifactDigest       string   `json:"base_artifact_digest"`
	ExtensionRefs            []string `json:"extension_refs"`
	RuntimeBasisRef          string   `json:"runtime_evaluation_basis_ref"`
	CompositeRef             string   `json:"composite_ref"`
	LoweredEnvironmentRef    string   `json:"lowered_environment_ref"`
	LoweredEnvironmentDigest string   `json:"lowered_environment_digest"`
	LowererSchema            string   `json:"lowerer_schema"`
	VerificationSchema       string   `json:"verification_schema"`
	VerificationResult       string   `json:"verification_result"`
}

func sealProjectTypeEnvCompositeVerification(
	base typeenv.BaseTypeEnvArtifact,
	linked LinkedProjectTypeEnvCompositeIR,
	runtimeBasis RuntimeEvaluationBasisArtifact,
	composite ProjectTypeEnvCompositeArtifact,
	environment typedmemory.TypeEnv,
) (ProjectTypeEnvCompositeVerification, error) {
	digest, err := projectTypeEnvLoweredEnvironmentDigest(
		environment,
		composite.LowererSchemaVersion(),
	)
	if err != nil {
		return ProjectTypeEnvCompositeVerification{}, err
	}
	encoded := projectTypeEnvCompositeVerificationCanonicalV1{
		BaseTypeEnvRef:           linked.BaseTypeEnvRef().String(),
		BaseArtifactDigest:       base.Digest().String(),
		ExtensionRefs:            extensionRefStrings(projectTypeEnvCompositeExtensionRefs(linked.Extensions())),
		RuntimeBasisRef:          runtimeBasis.Ref().String(),
		CompositeRef:             composite.Ref().String(),
		LoweredEnvironmentRef:    environment.Ref().String(),
		LoweredEnvironmentDigest: digest.String(),
		LowererSchema:            composite.LowererSchemaVersion(),
		VerificationSchema:       projectTypeEnvCompositeVerificationSchema,
		VerificationResult:       projectTypeEnvCompositeVerificationResult,
	}
	canonical, err := encodeProjectTypeEnvCompositeVerification(encoded)
	if err != nil {
		return ProjectTypeEnvCompositeVerification{}, err
	}
	record, err := DecodeProjectTypeEnvCompositeVerificationRecord(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"reseal project TypeEnv composite verification: %w",
			err,
		)
	}
	return ProjectTypeEnvCompositeVerification{
		record:     record,
		capability: &projectTypeEnvCompositeVerificationCapability{},
	}, nil
}

// DecodeProjectTypeEnvCompositeVerificationRecord verifies canonical receipt
// bytes but deliberately returns no Stage capability.
func DecodeProjectTypeEnvCompositeVerificationRecord(
	canonical []byte,
) (ProjectTypeEnvCompositeVerificationRecord, error) {
	payload, err := decodeProjectTypeEnvCompositeVerificationEnvelope(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	if !utf8.Valid(payload) {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"project TypeEnv composite verification payload contains invalid UTF-8",
		)
	}
	encoded := projectTypeEnvCompositeVerificationCanonicalV1{}
	if err := decodeStrictProjectTypeEnvCompositeVerificationJSON(payload, &encoded); err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	record, err := projectTypeEnvCompositeVerificationFromCanonical(encoded)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	reencoded, err := encodeProjectTypeEnvCompositeVerification(encoded)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"project TypeEnv composite verification payload is not canonical",
		)
	}
	record.canonicalBytes = append([]byte(nil), reencoded...)
	record.ref, err = projectTypeEnvCompositeVerificationRef(reencoded)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	return record, nil
}

func VerifyProjectTypeEnvCompositeVerificationRecord(
	expected ProjectTypeEnvCompositeVerificationRef,
	canonical []byte,
) (ProjectTypeEnvCompositeVerificationRecord, error) {
	parsed, err := ParseProjectTypeEnvCompositeVerificationRef(expected.String())
	if err != nil || parsed != expected {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"expected project TypeEnv composite verification reference is invalid",
		)
	}
	record, err := DecodeProjectTypeEnvCompositeVerificationRecord(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	if record.ref != expected {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"project TypeEnv composite verification ref %q does not match canonical bytes %q",
			expected.String(),
			record.ref.String(),
		)
	}
	return record, nil
}

// RestoreProjectTypeEnvCompositeVerification recreates the Stage capability
// after persistence only by rerunning the final lowerer against exact B/E/X/C
// and requiring its newly minted receipt to byte-match the stored record.
func RestoreProjectTypeEnvCompositeVerification(
	expected ProjectTypeEnvCompositeVerificationRef,
	canonical []byte,
	input ProjectTypeEnvCompositePreparationInput,
) (ProjectTypeEnvCompositeVerification, error) {
	if _, err := VerifyProjectTypeEnvCompositeVerificationRecord(expected, canonical); err != nil {
		return ProjectTypeEnvCompositeVerification{}, err
	}
	preparation := PrepareProjectTypeEnvComposite(input)
	if preparation.Rejected() {
		issues := preparation.Issues()
		if len(issues) == 0 {
			return ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
				"final lowerer rejected verification restoration",
			)
		}
		return ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"final lowerer rejected verification restoration: %s at %s: %s",
			issues[0].Code(),
			issues[0].Subject(),
			issues[0].Detail(),
		)
	}
	verification, exists := preparation.Verification()
	if !exists || !verification.validCapability() {
		return ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"final lowerer produced no trusted verification capability",
		)
	}
	if verification.Ref() != expected || !bytes.Equal(verification.CanonicalBytes(), canonical) {
		return ProjectTypeEnvCompositeVerification{}, fmt.Errorf(
			"stored verification record does not match recomputed final-lowerer result",
		)
	}
	return verification, nil
}

func projectTypeEnvCompositeVerificationFromCanonical(
	encoded projectTypeEnvCompositeVerificationCanonicalV1,
) (ProjectTypeEnvCompositeVerificationRecord, error) {
	baseRef, err := typedmemory.ParseTypeEnvRef(encoded.BaseTypeEnvRef)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification base ref: %w", err)
	}
	baseDigest, err := typedmemory.NewSHA256Digest(encoded.BaseArtifactDigest)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification base digest: %w", err)
	}
	extensions, err := parseVerificationExtensionRefs(encoded.ExtensionRefs)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, err
	}
	runtimeBasis, err := ParseRuntimeEvaluationBasisRef(encoded.RuntimeBasisRef)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification runtime basis: %w", err)
	}
	compositeRef, err := typedmemory.ParseTypeEnvRef(encoded.CompositeRef)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification composite ref: %w", err)
	}
	loweredRef, err := typedmemory.ParseTypeEnvRef(encoded.LoweredEnvironmentRef)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification lowered environment ref: %w", err)
	}
	if loweredRef != compositeRef {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"verification lowered environment ref must equal composite C",
		)
	}
	loweredDigest, err := typedmemory.NewSHA256Digest(encoded.LoweredEnvironmentDigest)
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf("verification lowered environment digest: %w", err)
	}
	if !supportedProjectTypeEnvCompositeLowererSchema(encoded.LowererSchema) {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"unsupported verification lowerer schema %q",
			encoded.LowererSchema,
		)
	}
	if encoded.VerificationSchema != projectTypeEnvCompositeVerificationSchema {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"unsupported verification schema %q",
			encoded.VerificationSchema,
		)
	}
	if encoded.VerificationResult != projectTypeEnvCompositeVerificationResult {
		return ProjectTypeEnvCompositeVerificationRecord{}, fmt.Errorf(
			"verification result must be %q",
			projectTypeEnvCompositeVerificationResult,
		)
	}
	return ProjectTypeEnvCompositeVerificationRecord{
		baseRef:                  baseRef,
		baseArtifactDigest:       baseDigest,
		extensionRefs:            extensions,
		runtimeBasisRef:          runtimeBasis,
		compositeRef:             compositeRef,
		loweredEnvironmentRef:    loweredRef,
		loweredEnvironmentDigest: loweredDigest,
		lowererSchema:            encoded.LowererSchema,
		verificationSchema:       encoded.VerificationSchema,
		verificationResult:       encoded.VerificationResult,
	}, nil
}

func encodeProjectTypeEnvCompositeVerification(
	encoded projectTypeEnvCompositeVerificationCanonicalV1,
) ([]byte, error) {
	if _, err := projectTypeEnvCompositeVerificationFromCanonical(encoded); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode project TypeEnv composite verification: %w", err)
	}
	writer := newProjectTypeEnvCompositeWriter(projectTypeEnvCompositeVerificationDomain)
	writer.addBytes(payload)
	result := writer.bytes()
	if len(result) > maximumProjectTypeEnvCompositeArtifactBytes {
		return nil, fmt.Errorf("project TypeEnv composite verification exceeds byte limit")
	}
	return result, nil
}

func decodeProjectTypeEnvCompositeVerificationEnvelope(canonical []byte) ([]byte, error) {
	if len(canonical) == 0 || len(canonical) > maximumProjectTypeEnvCompositeArtifactBytes {
		return nil, fmt.Errorf("project TypeEnv composite verification canonical bytes are invalid")
	}
	reader := &projectTypeEnvCompositeReader{data: canonical}
	root, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite verification root: %w", err)
	}
	if root != projectTypeEnvCompositeCanonicalDomain {
		return nil, fmt.Errorf("unexpected project TypeEnv composite verification root %q", root)
	}
	domain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite verification domain: %w", err)
	}
	if domain != projectTypeEnvCompositeVerificationDomain {
		return nil, fmt.Errorf("unexpected project TypeEnv composite verification domain %q", domain)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite verification payload: %w", err)
	}
	if reader.offset != len(reader.data) {
		return nil, fmt.Errorf("project TypeEnv composite verification has trailing bytes")
	}
	return append([]byte(nil), payload...), nil
}

func decodeStrictProjectTypeEnvCompositeVerificationJSON(
	payload []byte,
	target *projectTypeEnvCompositeVerificationCanonicalV1,
) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode project TypeEnv composite verification JSON: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("project TypeEnv composite verification JSON has trailing value")
	}
	return fmt.Errorf("decode project TypeEnv composite verification trailing JSON: %w", err)
}

func projectTypeEnvCompositeVerificationRef(
	canonical []byte,
) (ProjectTypeEnvCompositeVerificationRef, error) {
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		return ProjectTypeEnvCompositeVerificationRef{}, err
	}
	return ProjectTypeEnvCompositeVerificationRef{digest: digest}, nil
}

func parseVerificationExtensionRefs(
	raw []string,
) ([]typedmemory.TypeEnvExtensionRef, error) {
	if len(raw) > maximumCompositeExtensionArtifacts {
		return nil, fmt.Errorf("verification contains too many extension refs")
	}
	result := make([]typedmemory.TypeEnvExtensionRef, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for index, value := range raw {
		ref, err := typedmemory.ParseTypeEnvExtensionRef(value)
		if err != nil {
			return nil, fmt.Errorf("verification extension_refs[%d]: %w", index, err)
		}
		if _, exists := seen[ref.String()]; exists {
			return nil, fmt.Errorf("verification repeats extension ref %q", ref.String())
		}
		seen[ref.String()] = struct{}{}
		result = append(result, ref)
	}
	return result, nil
}

func extensionRefStrings(refs []typedmemory.TypeEnvExtensionRef) []string {
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.String())
	}
	return result
}

// projectTypeEnvLoweredEnvironmentDigest commits to the full public semantic
// materialization of the immutable TypeEnv. It is not a caller claim and is
// not used as C; it only authenticates the exact successful lowerer result.
func projectTypeEnvLoweredEnvironmentDigest(
	environment typedmemory.TypeEnv,
	lowererSchema string,
) (typedmemory.SHA256Digest, error) {
	canonical, err := projectTypeEnvLoweredEnvironmentCanonical(
		environment,
		lowererSchema,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	sum := sha256.Sum256(canonical)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
}

func projectTypeEnvLoweredEnvironmentCanonical(
	environment typedmemory.TypeEnv,
	lowererSchema string,
) ([]byte, error) {
	if environment.Ref().String() == "" {
		return nil, fmt.Errorf("lowered TypeEnv is required")
	}
	domain, err := projectTypeEnvLoweredEnvironmentDomain(lowererSchema)
	if err != nil {
		return nil, err
	}
	classificationSignatures := environment.KindClassificationSignatureDefinitions()
	if lowererSchema == ProjectTypeEnvCompositeLowererSchemaV1 &&
		len(classificationSignatures) > 0 {
		return nil, fmt.Errorf(
			"historical composite lowerer v1 cannot encode current KindClassification signatures",
		)
	}
	writer := newProjectTypeEnvCompositeWriter(domain)
	writer.addString(environment.Ref().String())
	writer.addString(environment.SourceRevision().String())
	writer.addString(environment.CompilerSchemaVersion().String())

	coverage := environment.CoverageManifest().Entries()
	writer.addBytes(canonicalCompositeCoverage(coverage))
	writer.addBytes(canonicalCompositeContexts(environment.BoundedContexts()))
	writer.addBytes(canonicalCompositeKinds(environment.KindDefinitions()))
	writer.addBytes(canonicalCompositeEntitySets(environment.EntitySetDefinitions()))
	writer.addBytes(canonicalCompositeKindSignatures(environment.KindSignatureDefinitions()))
	if lowererSchema == ProjectTypeEnvCompositeLowererSchemaV2 {
		writer.addBytes(canonicalCompositeKindClassificationSignatures(
			classificationSignatures,
		))
	}
	writer.addBytes(canonicalCompositeRefKinds(environment.RefKindDefinitions()))
	writer.addBytes(canonicalCompositeAvailabilities(environment.ContextKindAvailabilities()))
	writer.addBytes(canonicalCompositeSubkinds(environment.SubkindRelations()))
	writer.addBytes(canonicalCompositeBridges(environment.ContextBridges()))
	writer.addBytes(canonicalCompositeRelationFragments(
		environment.TypedRelationDeclarationFragments(),
	))
	writer.addBytes(canonicalCompositeShapes(environment.ValueShapes()))
	writer.addBytes(canonicalCompositeBindings(environment.ValueBindings()))
	writer.addBytes(canonicalCompositeConstraints(environment.Constraints()))
	return writer.bytes(), nil
}

func projectTypeEnvLoweredEnvironmentDomain(
	lowererSchema string,
) (string, error) {
	switch lowererSchema {
	case ProjectTypeEnvCompositeLowererSchemaV1:
		return projectTypeEnvLoweredEnvironmentDomainV1, nil
	case ProjectTypeEnvCompositeLowererSchemaV2:
		return projectTypeEnvLoweredEnvironmentDomainV2, nil
	default:
		return "", fmt.Errorf(
			"unsupported project TypeEnv composite lowerer schema %q",
			lowererSchema,
		)
	}
}

func canonicalCompositeCoverage(entries []typedmemory.CoverageEntry) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.coverage.v1")
	writer.addString(strconv.Itoa(len(entries)))
	for _, entry := range entries {
		location := entry.Source()
		lineRange := location.LineRange()
		patternID, patterned := location.PatternID()
		item := newProjectTypeEnvCompositeWriter("lowered-typeenv.coverage-entry.v1")
		item.addString(entry.Subject().String())
		item.addString(entry.Posture().String())
		item.addString(location.UnitID().String())
		item.addString(location.Revision().String())
		item.addString(location.ContentHash().String())
		item.addString(strconv.FormatUint(lineRange.Start(), 10))
		item.addString(strconv.FormatUint(lineRange.End(), 10))
		item.addString(strconv.FormatBool(patterned))
		item.addString(patternID.String())
		item.addString(entry.Rationale())
		writer.addBytes(item.bytes())
	}
	return writer.bytes()
}

func canonicalCompositeContexts(values []typedmemory.BoundedContext) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.contexts.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.Ref().String())
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeKinds(values []typedmemory.KindDefinition) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.kinds.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.ID().String())
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeEntitySets(values []typedmemory.EntitySetDefinition) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.entity-sets.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeKindSignatures(values []typedmemory.KindSignatureDefinition) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.kind-signatures.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeKindClassificationSignatures(
	values []typedmemory.KindClassificationSignatureDefinition,
) []byte {
	writer := newProjectTypeEnvCompositeWriter(
		"lowered-typeenv.kind-classification-signatures.v1",
	)
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeRefKinds(values []typedmemory.RefKindDefinition) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.ref-kinds.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.Ref().String())
		writer.addString(value.ValueKind().String())
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeAvailabilities(values []typedmemory.ContextKindAvailability) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.availabilities.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeSubkinds(values []typedmemory.SubkindRelation) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.subkinds.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.Subkind().String())
		writer.addString(value.Superkind().String())
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeBridges(values []typedmemory.ContextBridge) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.bridges.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeRelationFragments(
	values []typedmemory.TypedRelationDeclarationFragment,
) []byte {
	// Preserve the v1 canonical domains: the selected change is a semantic
	// classification of the same exact edition-bound bytes, not a reidentity of
	// existing composite TypeEnvs.
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.relations.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		item := newProjectTypeEnvCompositeWriter("lowered-typeenv.relation.v1")
		item.addString(value.Ref().String())
		for _, context := range value.Contexts() {
			item.addString(context.String())
		}
		for _, slot := range value.Slots() {
			cardinality := slot.Cardinality()
			maximum, bounded := cardinality.Maximum().BoundedValue()
			item.addString(slot.SlotKind().String())
			item.addString(slot.RefMode().String())
			item.addString(slot.Target().CanonicalKey())
			item.addString(strconv.FormatUint(cardinality.Minimum(), 10))
			item.addString(strconv.FormatBool(bounded))
			item.addString(strconv.FormatUint(maximum, 10))
			item.addBytes(slot.Provenance().CanonicalBytes())
		}
		item.addBytes(value.Provenance().CanonicalBytes())
		writer.addBytes(item.bytes())
	}
	return writer.bytes()
}

func canonicalCompositeShapes(values []typedmemory.ValueShapeDeclaration) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.shapes.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.Ref().String())
		writer.addString(string(value.Shape().Kind()))
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeBindings(values []typedmemory.ValueBinding) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.bindings.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addString(value.ValueKind().String())
		writer.addString(value.ValueShape().String())
		writer.addString(value.Codec().String())
		writer.addBytes(value.Provenance().CanonicalBytes())
	}
	return writer.bytes()
}

func canonicalCompositeConstraints(values []typedmemory.ConstraintRule) []byte {
	writer := newProjectTypeEnvCompositeWriter("lowered-typeenv.constraints.v1")
	writer.addString(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.addBytes(value.CanonicalBytes())
	}
	return writer.bytes()
}
