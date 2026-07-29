package projecttypeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	projectTypeEnvCompositeCanonicalDomain = "haft.fpf.projecttypeenv.composite.canonical.v1"
	projectTypeEnvCompositeArtifactDomain  = "project-typeenv-composite-artifact.v1"

	// ProjectTypeEnvCompositeLowererSchemaV1 is the sealed historical lowering
	// interpretation used before current C.3 KindClassification signatures.
	ProjectTypeEnvCompositeLowererSchemaV1 = "haft.fpf.projecttypeenv.composite-lowerer/v1"
	// ProjectTypeEnvCompositeLowererSchemaV2 adds the current C.3
	// KindClassification signature family to exact lowered-environment bytes.
	// It is recipe identity, not executable code or lowered TypeEnv bytes.
	ProjectTypeEnvCompositeLowererSchemaV2 = "haft.fpf.projecttypeenv.composite-lowerer/v2"

	maximumProjectTypeEnvCompositeArtifactBytes = 4 << 20
)

// ProjectTypeEnvCompositeArtifact is immutable content-addressed recipe C.
// Its identity binds exact B, canonical topological E refs, exact X, and the
// lowerer schema. It deliberately contains no lowered TypeEnv bytes, Stage,
// project head, mutable project state, or caller-supplied C.
type ProjectTypeEnvCompositeArtifact struct {
	ref            typedmemory.TypeEnvRef
	base           typedmemory.TypeEnvRef
	extensions     []typedmemory.TypeEnvExtensionRef
	runtimeBasis   RuntimeEvaluationBasisRef
	lowererSchema  string
	canonicalBytes []byte
}

func (artifact ProjectTypeEnvCompositeArtifact) Ref() typedmemory.TypeEnvRef {
	return artifact.ref
}

func (artifact ProjectTypeEnvCompositeArtifact) Digest() typedmemory.SHA256Digest {
	return artifact.ref.Digest()
}

func (artifact ProjectTypeEnvCompositeArtifact) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return artifact.base
}

func (artifact ProjectTypeEnvCompositeArtifact) ExtensionRefs() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), artifact.extensions...)
}

func (artifact ProjectTypeEnvCompositeArtifact) RuntimeEvaluationBasisRef() RuntimeEvaluationBasisRef {
	return artifact.runtimeBasis
}

func (artifact ProjectTypeEnvCompositeArtifact) LowererSchemaVersion() string {
	return artifact.lowererSchema
}

func (artifact ProjectTypeEnvCompositeArtifact) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonicalBytes...)
}

func (artifact ProjectTypeEnvCompositeArtifact) Verify() error {
	if len(artifact.canonicalBytes) == 0 {
		return fmt.Errorf("project TypeEnv composite artifact is empty")
	}
	decoded, err := DecodeProjectTypeEnvCompositeArtifact(artifact.canonicalBytes)
	if err != nil {
		return fmt.Errorf("verify project TypeEnv composite canonical bytes: %w", err)
	}
	if decoded.ref != artifact.ref {
		return fmt.Errorf("project TypeEnv composite reference is not derived from its bytes")
	}
	if decoded.base != artifact.base {
		return fmt.Errorf("project TypeEnv composite stored base does not match canonical recipe")
	}
	if !projectTypeEnvExtensionRefsEqual(decoded.extensions, artifact.extensions) {
		return fmt.Errorf("project TypeEnv composite stored extensions do not match canonical recipe")
	}
	if decoded.runtimeBasis != artifact.runtimeBasis {
		return fmt.Errorf("project TypeEnv composite stored runtime basis does not match canonical recipe")
	}
	if decoded.lowererSchema != artifact.lowererSchema {
		return fmt.Errorf("project TypeEnv composite stored lowerer schema does not match canonical recipe")
	}
	if !bytes.Equal(decoded.canonicalBytes, artifact.canonicalBytes) {
		return fmt.Errorf("project TypeEnv composite stored bytes are not canonical")
	}
	return nil
}

// SealProjectTypeEnvComposite verifies the linked B/E proof and exact X, then
// derives C from the canonical recipe. No caller supplies C.
func SealProjectTypeEnvComposite(
	linked LinkedProjectTypeEnvCompositeIR,
	runtimeBasis RuntimeEvaluationBasisArtifact,
) (ProjectTypeEnvCompositeArtifact, error) {
	return sealProjectTypeEnvCompositeAtSchema(
		linked,
		runtimeBasis,
		ProjectTypeEnvCompositeLowererSchemaV2,
	)
}

// ResealHistoricalProjectTypeEnvCompositeV1 reproduces the exact pre-current-
// C.3 recipe identity for edition-tagged replay. It is not a selectable current
// lowerer: any KindClassification declaration is rejected, and the executable
// preparation independently rejects a v1 recipe over a Base that already
// contains current classification signatures.
func ResealHistoricalProjectTypeEnvCompositeV1(
	linked LinkedProjectTypeEnvCompositeIR,
	runtimeBasis RuntimeEvaluationBasisArtifact,
) (ProjectTypeEnvCompositeArtifact, error) {
	for _, extension := range linked.Extensions() {
		declarations := extension.Artifact().IR().Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			if declaration.Kind() == localpractice.DeclarationKindClassificationSignature {
				return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
					"historical composite lowerer v1 cannot seal current KindClassification declarations",
				)
			}
		}
	}
	return sealProjectTypeEnvCompositeAtSchema(
		linked,
		runtimeBasis,
		ProjectTypeEnvCompositeLowererSchemaV1,
	)
}

func sealProjectTypeEnvCompositeAtSchema(
	linked LinkedProjectTypeEnvCompositeIR,
	runtimeBasis RuntimeEvaluationBasisArtifact,
	lowererSchema string,
) (ProjectTypeEnvCompositeArtifact, error) {
	verifiedLinked, err := verifyLinkedProjectTypeEnvCompositeIR(linked)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	if err := runtimeBasis.Verify(); err != nil {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"verify project TypeEnv composite runtime basis: %w",
			err,
		)
	}
	recipe := projectTypeEnvCompositeRecipe{
		base:          verifiedLinked.BaseTypeEnvRef(),
		extensions:    projectTypeEnvCompositeExtensionRefs(verifiedLinked.Extensions()),
		runtimeBasis:  runtimeBasis.Ref(),
		lowererSchema: lowererSchema,
	}
	canonical, err := encodeProjectTypeEnvCompositeRecipe(recipe)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	artifact, err := DecodeProjectTypeEnvCompositeArtifact(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"reseal project TypeEnv composite: %w",
			err,
		)
	}
	return artifact, nil
}

// DecodeProjectTypeEnvCompositeArtifact accepts exact canonical recipe bytes
// only. It does not lower or load the referenced B, E, or X artifacts.
func DecodeProjectTypeEnvCompositeArtifact(
	canonical []byte,
) (ProjectTypeEnvCompositeArtifact, error) {
	payload, err := decodeProjectTypeEnvCompositeEnvelope(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	if !utf8.Valid(payload) {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"project TypeEnv composite payload contains invalid UTF-8",
		)
	}
	encoded := projectTypeEnvCompositeCanonicalV1{}
	if err := decodeStrictProjectTypeEnvCompositeJSON(payload, &encoded); err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	recipe, err := projectTypeEnvCompositeRecipeFromCanonical(encoded)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	reencoded, err := encodeProjectTypeEnvCompositeRecipe(recipe)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"project TypeEnv composite payload is not canonical",
		)
	}
	ref, err := projectTypeEnvCompositeRef(reencoded)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	return ProjectTypeEnvCompositeArtifact{
		ref:            ref,
		base:           recipe.base,
		extensions:     append([]typedmemory.TypeEnvExtensionRef(nil), recipe.extensions...),
		runtimeBasis:   recipe.runtimeBasis,
		lowererSchema:  recipe.lowererSchema,
		canonicalBytes: append([]byte(nil), reencoded...),
	}, nil
}

func VerifyProjectTypeEnvCompositeArtifact(
	expected typedmemory.TypeEnvRef,
	canonical []byte,
) (ProjectTypeEnvCompositeArtifact, error) {
	parsedExpected, err := typedmemory.ParseTypeEnvRef(expected.String())
	if err != nil || parsedExpected != expected {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"expected project TypeEnv composite reference is invalid",
		)
	}
	artifact, err := DecodeProjectTypeEnvCompositeArtifact(canonical)
	if err != nil {
		return ProjectTypeEnvCompositeArtifact{}, err
	}
	if artifact.ref != expected {
		return ProjectTypeEnvCompositeArtifact{}, fmt.Errorf(
			"project TypeEnv composite reference %q does not match canonical bytes %q",
			expected.String(),
			artifact.ref.String(),
		)
	}
	return artifact, nil
}

type projectTypeEnvCompositeRecipe struct {
	base          typedmemory.TypeEnvRef
	extensions    []typedmemory.TypeEnvExtensionRef
	runtimeBasis  RuntimeEvaluationBasisRef
	lowererSchema string
}

type projectTypeEnvCompositeCanonicalV1 struct {
	BaseTypeEnvRef            string   `json:"base_type_env_ref"`
	ExtensionRefs             []string `json:"extension_refs"`
	RuntimeEvaluationBasisRef string   `json:"runtime_evaluation_basis_ref"`
	LowererSchemaVersion      string   `json:"lowerer_schema_version"`
}

func verifyLinkedProjectTypeEnvCompositeIR(
	linked LinkedProjectTypeEnvCompositeIR,
) (LinkedProjectTypeEnvCompositeIR, error) {
	extensions := linked.Extensions()
	artifacts := make([]ProjectTypeEnvExtensionArtifact, 0, len(extensions))
	for _, extension := range extensions {
		artifacts = append(artifacts, extension.Artifact())
	}
	resolution := LinkProjectTypeEnvCompositeIR(linked.BaseArtifact(), artifacts)
	if resolution.Rejected() {
		return LinkedProjectTypeEnvCompositeIR{}, projectTypeEnvCompositeLinkError(
			resolution.Issues(),
		)
	}
	verified, exists := resolution.CompositeIR()
	if !exists {
		return LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"verified project TypeEnv composite link produced no IR",
		)
	}
	if linked.BaseTypeEnvRef() != verified.BaseTypeEnvRef() {
		return LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"project TypeEnv composite linked base does not match verified B",
		)
	}
	if !bytes.Equal(linked.CanonicalBytes(), verified.CanonicalBytes()) {
		return LinkedProjectTypeEnvCompositeIR{}, fmt.Errorf(
			"project TypeEnv composite linked IR is not the canonical verified B/E proof",
		)
	}
	return verified, nil
}

func projectTypeEnvCompositeLinkError(issues []LinkIssue) error {
	if len(issues) == 0 {
		return fmt.Errorf("project TypeEnv composite B/E link was rejected")
	}
	issue := issues[0]
	return fmt.Errorf(
		"project TypeEnv composite B/E link was rejected: %s at %s: %s; repair: %s",
		issue.Code(),
		issue.Location().String(),
		issue.Detail(),
		issue.Repair(),
	)
}

func projectTypeEnvCompositeExtensionRefs(
	extensions []LinkedCompositeExtension,
) []typedmemory.TypeEnvExtensionRef {
	refs := make([]typedmemory.TypeEnvExtensionRef, 0, len(extensions))
	for _, extension := range extensions {
		refs = append(refs, extension.Ref())
	}
	return refs
}

func projectTypeEnvCompositeRecipeFromCanonical(
	encoded projectTypeEnvCompositeCanonicalV1,
) (projectTypeEnvCompositeRecipe, error) {
	base, err := typedmemory.ParseTypeEnvRef(encoded.BaseTypeEnvRef)
	if err != nil {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite base reference: %w",
			err,
		)
	}
	if len(encoded.ExtensionRefs) > maximumCompositeExtensionArtifacts {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite contains %d extension refs; limit is %d",
			len(encoded.ExtensionRefs),
			maximumCompositeExtensionArtifacts,
		)
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, len(encoded.ExtensionRefs))
	seen := make(map[string]struct{}, len(encoded.ExtensionRefs))
	for index, raw := range encoded.ExtensionRefs {
		ref, parseErr := typedmemory.ParseTypeEnvExtensionRef(raw)
		if parseErr != nil {
			return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
				"project TypeEnv composite extension_refs[%d]: %w",
				index,
				parseErr,
			)
		}
		if _, exists := seen[ref.String()]; exists {
			return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
				"project TypeEnv composite repeats extension ref %q",
				ref.String(),
			)
		}
		seen[ref.String()] = struct{}{}
		extensions = append(extensions, ref)
	}
	runtimeBasis, err := ParseRuntimeEvaluationBasisRef(encoded.RuntimeEvaluationBasisRef)
	if err != nil {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite runtime basis reference: %w",
			err,
		)
	}
	recipe := projectTypeEnvCompositeRecipe{
		base:          base,
		extensions:    extensions,
		runtimeBasis:  runtimeBasis,
		lowererSchema: encoded.LowererSchemaVersion,
	}
	return normalizeProjectTypeEnvCompositeRecipe(recipe)
}

func normalizeProjectTypeEnvCompositeRecipe(
	recipe projectTypeEnvCompositeRecipe,
) (projectTypeEnvCompositeRecipe, error) {
	base, err := typedmemory.ParseTypeEnvRef(recipe.base.String())
	if err != nil || base != recipe.base {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite base reference is invalid",
		)
	}
	if len(recipe.extensions) > maximumCompositeExtensionArtifacts {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite contains %d extension refs; limit is %d",
			len(recipe.extensions),
			maximumCompositeExtensionArtifacts,
		)
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, len(recipe.extensions))
	seen := make(map[string]struct{}, len(recipe.extensions))
	for index, ref := range recipe.extensions {
		parsed, parseErr := typedmemory.ParseTypeEnvExtensionRef(ref.String())
		if parseErr != nil || parsed != ref {
			return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
				"project TypeEnv composite extension_refs[%d] is invalid",
				index,
			)
		}
		if _, exists := seen[ref.String()]; exists {
			return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
				"project TypeEnv composite repeats extension ref %q",
				ref.String(),
			)
		}
		seen[ref.String()] = struct{}{}
		extensions = append(extensions, ref)
	}
	runtimeBasis, err := ParseRuntimeEvaluationBasisRef(recipe.runtimeBasis.String())
	if err != nil || runtimeBasis != recipe.runtimeBasis {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"project TypeEnv composite runtime basis reference is invalid",
		)
	}
	if !supportedProjectTypeEnvCompositeLowererSchema(recipe.lowererSchema) {
		return projectTypeEnvCompositeRecipe{}, fmt.Errorf(
			"unsupported project TypeEnv composite lowerer schema %q",
			recipe.lowererSchema,
		)
	}
	return projectTypeEnvCompositeRecipe{
		base:          base,
		extensions:    extensions,
		runtimeBasis:  runtimeBasis,
		lowererSchema: recipe.lowererSchema,
	}, nil
}

func supportedProjectTypeEnvCompositeLowererSchema(value string) bool {
	return value == ProjectTypeEnvCompositeLowererSchemaV1 ||
		value == ProjectTypeEnvCompositeLowererSchemaV2
}

func encodeProjectTypeEnvCompositeRecipe(
	recipe projectTypeEnvCompositeRecipe,
) ([]byte, error) {
	normalized, err := normalizeProjectTypeEnvCompositeRecipe(recipe)
	if err != nil {
		return nil, err
	}
	extensionRefs := make([]string, 0, len(normalized.extensions))
	for _, ref := range normalized.extensions {
		extensionRefs = append(extensionRefs, ref.String())
	}
	encoded := projectTypeEnvCompositeCanonicalV1{
		BaseTypeEnvRef:            normalized.base.String(),
		ExtensionRefs:             extensionRefs,
		RuntimeEvaluationBasisRef: normalized.runtimeBasis.String(),
		LowererSchemaVersion:      normalized.lowererSchema,
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode project TypeEnv composite payload: %w", err)
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("encoded project TypeEnv composite payload contains invalid UTF-8")
	}
	writer := newProjectTypeEnvCompositeWriter(projectTypeEnvCompositeArtifactDomain)
	writer.addBytes(payload)
	result := writer.bytes()
	if len(result) > maximumProjectTypeEnvCompositeArtifactBytes {
		return nil, fmt.Errorf(
			"project TypeEnv composite artifact exceeds %d bytes",
			maximumProjectTypeEnvCompositeArtifactBytes,
		)
	}
	return result, nil
}

func projectTypeEnvCompositeRef(
	canonical []byte,
) (typedmemory.TypeEnvRef, error) {
	sum := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexDigest)
	if err != nil {
		return typedmemory.TypeEnvRef{}, err
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		return typedmemory.TypeEnvRef{}, err
	}
	return ref, nil
}

func projectTypeEnvExtensionRefsEqual(
	left []typedmemory.TypeEnvExtensionRef,
	right []typedmemory.TypeEnvExtensionRef,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func decodeStrictProjectTypeEnvCompositeJSON(
	payload []byte,
	target *projectTypeEnvCompositeCanonicalV1,
) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode project TypeEnv composite payload: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("project TypeEnv composite JSON has a trailing value")
	}
	return fmt.Errorf("decode project TypeEnv composite trailing JSON: %w", err)
}

type projectTypeEnvCompositeWriter struct {
	buffer bytes.Buffer
}

func newProjectTypeEnvCompositeWriter(domain string) projectTypeEnvCompositeWriter {
	writer := projectTypeEnvCompositeWriter{}
	writer.addString(projectTypeEnvCompositeCanonicalDomain)
	writer.addString(domain)
	return writer
}

func (writer *projectTypeEnvCompositeWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *projectTypeEnvCompositeWriter) addBytes(value []byte) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(len(value)))
	writer.buffer.Write(encoded[:])
	writer.buffer.Write(value)
}

func (writer projectTypeEnvCompositeWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type projectTypeEnvCompositeReader struct {
	data   []byte
	offset int
}

func decodeProjectTypeEnvCompositeEnvelope(canonical []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("project TypeEnv composite canonical bytes are required")
	}
	if len(canonical) > maximumProjectTypeEnvCompositeArtifactBytes {
		return nil, fmt.Errorf(
			"project TypeEnv composite canonical bytes exceed %d-byte limit",
			maximumProjectTypeEnvCompositeArtifactBytes,
		)
	}
	reader := &projectTypeEnvCompositeReader{data: canonical}
	root, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite root domain: %w", err)
	}
	if root != projectTypeEnvCompositeCanonicalDomain {
		return nil, fmt.Errorf("unexpected project TypeEnv composite root domain %q", root)
	}
	domain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite artifact domain: %w", err)
	}
	if domain != projectTypeEnvCompositeArtifactDomain {
		return nil, fmt.Errorf("unexpected project TypeEnv composite artifact domain %q", domain)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("decode project TypeEnv composite payload: %w", err)
	}
	if reader.offset != len(reader.data) {
		return nil, fmt.Errorf(
			"project TypeEnv composite payload has %d trailing bytes",
			len(reader.data)-reader.offset,
		)
	}
	return append([]byte(nil), payload...), nil
}

func (reader *projectTypeEnvCompositeReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("canonical domain contains invalid UTF-8")
	}
	return string(value), nil
}

func (reader *projectTypeEnvCompositeReader) readBytes() ([]byte, error) {
	if reader == nil || len(reader.data)-reader.offset < 8 {
		return nil, fmt.Errorf("unexpected end of length-prefixed field")
	}
	endLength := reader.offset + 8
	length := binary.BigEndian.Uint64(reader.data[reader.offset:endLength])
	reader.offset = endLength
	remaining := len(reader.data) - reader.offset
	//nolint:gosec // remaining is non-negative after the reader bounds check above.
	if length > uint64(remaining) {
		return nil, fmt.Errorf(
			"length-prefixed field %d exceeds remaining payload %d",
			length,
			remaining,
		)
	}
	if length > maximumProjectTypeEnvCompositeArtifactBytes {
		return nil, fmt.Errorf(
			"length-prefixed field exceeds %d bytes",
			maximumProjectTypeEnvCompositeArtifactBytes,
		)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := reader.data[reader.offset:end]
	reader.offset = end
	return value, nil
}
