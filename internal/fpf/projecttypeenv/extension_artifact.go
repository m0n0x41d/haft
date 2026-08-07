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
	projectExtensionCanonicalDomain = "haft.fpf.projecttypeenv.canonical.v1"
	projectExtensionArtifactDomain  = "project-typeenv-extension-artifact.v1"
	maximumProjectExtensionBytes    = 16 << 20
	maximumProjectExtensionEntries  = 64 << 10
)

// ProjectTypeEnvExtensionArtifact is an immutable, content-addressed E-layer
// carrier. Its reference is derived from the manifest ID and SHA-256 of the
// exact canonical bytes. It has no staging, persistence, lowering, or
// activation operation.
type ProjectTypeEnvExtensionArtifact struct {
	ref       typedmemory.TypeEnvExtensionRef
	canonical []byte
	ir        ProjectTypeEnvExtensionIR
}

func (artifact ProjectTypeEnvExtensionArtifact) Ref() typedmemory.TypeEnvExtensionRef {
	return artifact.ref
}

func (artifact ProjectTypeEnvExtensionArtifact) Digest() typedmemory.SHA256Digest {
	return artifact.ref.Digest()
}

func (artifact ProjectTypeEnvExtensionArtifact) ManifestCoordinate() ManifestCoordinate {
	return artifact.ir.manifest.coordinate
}

func (artifact ProjectTypeEnvExtensionArtifact) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonical...)
}

func (artifact ProjectTypeEnvExtensionArtifact) IR() ProjectTypeEnvExtensionIR {
	return cloneProjectTypeEnvExtensionIR(artifact.ir)
}

func (artifact ProjectTypeEnvExtensionArtifact) Verify() error {
	if len(artifact.canonical) == 0 {
		return fmt.Errorf("project TypeEnv extension artifact is empty")
	}
	decoded, err := DecodeProjectTypeEnvExtensionArtifact(artifact.canonical)
	if err != nil {
		return fmt.Errorf("verify project TypeEnv extension canonical bytes: %w", err)
	}
	if decoded.ref != artifact.ref {
		return fmt.Errorf("project TypeEnv extension artifact reference is not derived from its bytes")
	}
	storedCanonical, err := encodeProjectTypeEnvExtensionIR(artifact.ir)
	if err != nil {
		return fmt.Errorf("verify project TypeEnv extension stored IR: %w", err)
	}
	if !bytes.Equal(storedCanonical, artifact.canonical) {
		return fmt.Errorf("project TypeEnv extension stored IR does not exactly encode its canonical bytes")
	}
	return nil
}

// SealProjectTypeEnvExtension normalizes the symbolic IR, encodes one exact
// canonical payload, and returns only the result of decoding and resealing
// those bytes. No caller supplies an E-ref or digest.
func SealProjectTypeEnvExtension(
	ir ProjectTypeEnvExtensionIR,
) (ProjectTypeEnvExtensionArtifact, error) {
	normalized, err := normalizeProjectTypeEnvExtensionIR(ir)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	canonical, err := encodeProjectTypeEnvExtensionIR(normalized)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	artifact, err := DecodeProjectTypeEnvExtensionArtifact(canonical)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, fmt.Errorf("reseal project TypeEnv extension: %w", err)
	}
	return artifact, nil
}

// DecodeProjectTypeEnvExtensionArtifact accepts only exact canonical bytes,
// rebuilds strong base and predecessor references, normalizes the symbolic IR,
// and requires byte-for-byte re-encoding equality before deriving the E-ref.
func DecodeProjectTypeEnvExtensionArtifact(
	canonical []byte,
) (ProjectTypeEnvExtensionArtifact, error) {
	payload, err := decodeProjectExtensionEnvelope(canonical)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	if !utf8.Valid(payload) {
		return ProjectTypeEnvExtensionArtifact{}, fmt.Errorf("project TypeEnv extension payload contains invalid UTF-8")
	}
	var encoded projectExtensionCanonicalV1
	if err := decodeStrictProjectExtensionJSON(payload, &encoded); err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	ir, err := projectExtensionIRFromCanonical(encoded)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	normalized, err := normalizeProjectTypeEnvExtensionIR(ir)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	reencoded, err := encodeProjectTypeEnvExtensionIR(normalized)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvExtensionArtifact{}, fmt.Errorf("project TypeEnv extension payload is not canonical")
	}
	digest, err := projectExtensionDigest(reencoded)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	ref, err := deriveProjectExtensionRef(normalized.manifest.coordinate.ID(), digest)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	return ProjectTypeEnvExtensionArtifact{
		ref:       ref,
		canonical: append([]byte(nil), reencoded...),
		ir:        cloneProjectTypeEnvExtensionIR(normalized),
	}, nil
}

func VerifyProjectTypeEnvExtensionArtifact(
	expected typedmemory.TypeEnvExtensionRef,
	canonical []byte,
) (ProjectTypeEnvExtensionArtifact, error) {
	parsed, err := typedmemory.ParseTypeEnvExtensionRef(expected.String())
	if err != nil || parsed != expected {
		return ProjectTypeEnvExtensionArtifact{}, fmt.Errorf("expected project TypeEnv extension reference is invalid")
	}
	artifact, err := DecodeProjectTypeEnvExtensionArtifact(canonical)
	if err != nil {
		return ProjectTypeEnvExtensionArtifact{}, err
	}
	if artifact.ref != expected {
		return ProjectTypeEnvExtensionArtifact{}, fmt.Errorf(
			"project TypeEnv extension reference %q does not match canonical bytes %q",
			expected.String(),
			artifact.ref.String(),
		)
	}
	return artifact, nil
}

type projectExtensionCanonicalV1 struct {
	BaseTypeEnv    string                     `json:"base_type_env_ref"`
	BaseSource     sourceScalarCanonicalV1    `json:"base_source"`
	BoundedContext sourceScalarCanonicalV1    `json:"bounded_context"`
	Carrier        carrierIdentityCanonicalV1 `json:"carrier"`
	Manifest       projectManifestCanonicalV1 `json:"manifest"`
	Signature      signatureRowsCanonicalV1   `json:"signature"`
	Compiler       sourceScalarCanonicalV1    `json:"compiler_version"`
}

type sourceScalarCanonicalV1 struct {
	Value string `json:"value"`
	Start uint64 `json:"start_line"`
	End   uint64 `json:"end_line"`
}

type sourceSpanCanonicalV1 struct {
	Start uint64 `json:"start_line"`
	End   uint64 `json:"end_line"`
}

type carrierIdentityCanonicalV1 struct {
	SchemaVersion sourceScalarCanonicalV1 `json:"schema_version"`
	ID            sourceScalarCanonicalV1 `json:"id"`
	Edition       sourceScalarCanonicalV1 `json:"edition"`
	SourceDigest  string                  `json:"source_digest"`
	CarrierSpan   sourceSpanCanonicalV1   `json:"carrier_span"`
	IdentitySpan  sourceSpanCanonicalV1   `json:"identity_span"`
}

type projectManifestCanonicalV1 struct {
	ID               sourceScalarCanonicalV1   `json:"id"`
	Version          sourceScalarCanonicalV1   `json:"version"`
	PublicationState *sourceScalarCanonicalV1  `json:"publication_state"`
	Predecessors     []predecessorCanonicalV1  `json:"direct_predecessors"`
	Provides         []sourceScalarCanonicalV1 `json:"provides"`
	Span             sourceSpanCanonicalV1     `json:"span"`
}

type predecessorCanonicalV1 struct {
	ManifestID      string                  `json:"manifest_id"`
	ManifestVersion string                  `json:"manifest_version"`
	ExtensionRef    string                  `json:"extension_ref"`
	Source          sourceScalarCanonicalV1 `json:"source"`
}

type signatureRowsCanonicalV1 struct {
	Span          sourceSpanCanonicalV1   `json:"span"`
	Subject       signatureRowCanonicalV1 `json:"subject_block"`
	Vocabulary    vocabularyCanonicalV1   `json:"vocabulary"`
	Laws          signatureRowCanonicalV1 `json:"laws"`
	Applicability signatureRowCanonicalV1 `json:"applicability"`
}

type signatureRowCanonicalV1 struct {
	Name  string                  `json:"name"`
	Span  sourceSpanCanonicalV1   `json:"span"`
	Facts []sourceFactCanonicalV1 `json:"facts"`
}

type vocabularyCanonicalV1 struct {
	Span         sourceSpanCanonicalV1            `json:"span"`
	Declarations []symbolicDeclarationCanonicalV1 `json:"declarations"`
}

type symbolicDeclarationCanonicalV1 struct {
	Kind         string                          `json:"kind"`
	Symbol       sourceScalarCanonicalV1         `json:"symbol"`
	Span         sourceSpanCanonicalV1           `json:"span"`
	Exports      []sourceScalarCanonicalV1       `json:"exports"`
	Facts        []sourceFactCanonicalV1         `json:"facts"`
	Dependencies []symbolicDependencyCanonicalV1 `json:"dependencies"`
}

type sourceFactCanonicalV1 struct {
	Path  string                  `json:"path"`
	Value sourceScalarCanonicalV1 `json:"value"`
}

type symbolicDependencyCanonicalV1 struct {
	Role   string                  `json:"role"`
	Target sourceScalarCanonicalV1 `json:"target"`
}

func encodeProjectTypeEnvExtensionIR(ir ProjectTypeEnvExtensionIR) ([]byte, error) {
	if err := validateProjectTypeEnvExtensionIR(ir); err != nil {
		return nil, err
	}
	encoded := projectExtensionCanonicalFromIR(ir)
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode project TypeEnv extension payload: %w", err)
	}
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("encoded project TypeEnv extension payload contains invalid UTF-8")
	}
	writer := newProjectExtensionWriter(projectExtensionArtifactDomain)
	writer.addBytes(payload)
	result := writer.bytes()
	if len(result) > maximumProjectExtensionBytes {
		return nil, fmt.Errorf("project TypeEnv extension artifact exceeds %d bytes", maximumProjectExtensionBytes)
	}
	return result, nil
}

func projectExtensionCanonicalFromIR(ir ProjectTypeEnvExtensionIR) projectExtensionCanonicalV1 {
	return projectExtensionCanonicalV1{
		BaseTypeEnv:    ir.baseTypeEnv.String(),
		BaseSource:     sourceScalarCanonical(ir.baseSource),
		BoundedContext: sourceScalarCanonical(ir.boundedContext),
		Carrier: carrierIdentityCanonicalV1{
			SchemaVersion: sourceScalarCanonical(ir.carrier.schemaVersion),
			ID:            sourceScalarCanonical(ir.carrier.id),
			Edition:       sourceScalarCanonical(ir.carrier.edition),
			SourceDigest:  ir.carrier.digest.String(),
			CarrierSpan:   sourceSpanCanonical(ir.carrier.carrierSpan),
			IdentitySpan:  sourceSpanCanonical(ir.carrier.identitySpan),
		},
		Manifest:  manifestCanonical(ir.manifest),
		Signature: signatureCanonical(ir.signature),
		Compiler:  sourceScalarCanonical(ir.compiler),
	}
}

func manifestCanonical(manifest ProjectSignatureManifestIR) projectManifestCanonicalV1 {
	predecessors := make([]predecessorCanonicalV1, 0, len(manifest.predecessors))
	for _, predecessor := range manifest.predecessors {
		predecessors = append(predecessors, predecessorCanonicalV1{
			ManifestID:      predecessor.coordinate.ID(),
			ManifestVersion: predecessor.coordinate.Version(),
			ExtensionRef:    predecessor.ref.String(),
			Source:          sourceScalarCanonical(predecessor.source),
		})
	}
	provides := sourceScalarsCanonical(manifest.provides)
	var state *sourceScalarCanonicalV1
	if manifest.hasState {
		encoded := sourceScalarCanonical(manifest.publicationState)
		state = &encoded
	}
	return projectManifestCanonicalV1{
		ID:               sourceScalarCanonical(manifest.id),
		Version:          sourceScalarCanonical(manifest.version),
		PublicationState: state,
		Predecessors:     predecessors,
		Provides:         provides,
		Span:             sourceSpanCanonical(manifest.span),
	}
}

func signatureCanonical(rows ProjectSignatureRowsIR) signatureRowsCanonicalV1 {
	return signatureRowsCanonicalV1{
		Span:    sourceSpanCanonical(rows.span),
		Subject: signatureRowCanonical(rows.subject),
		Vocabulary: vocabularyCanonicalV1{
			Span:         sourceSpanCanonical(rows.vocabulary.span),
			Declarations: declarationsCanonical(rows.vocabulary.declarations),
		},
		Laws:          signatureRowCanonical(rows.laws),
		Applicability: signatureRowCanonical(rows.applicability),
	}
}

func signatureRowCanonical(row SignatureRowIR) signatureRowCanonicalV1 {
	return signatureRowCanonicalV1{
		Name:  row.name,
		Span:  sourceSpanCanonical(row.span),
		Facts: factsCanonical(row.facts),
	}
}

func declarationsCanonical(values []SymbolicDeclaration) []symbolicDeclarationCanonicalV1 {
	result := make([]symbolicDeclarationCanonicalV1, 0, len(values))
	for _, declaration := range values {
		dependencies := make([]symbolicDependencyCanonicalV1, 0, len(declaration.dependencies))
		for _, dependency := range declaration.dependencies {
			dependencies = append(dependencies, symbolicDependencyCanonicalV1{
				Role:   dependency.role,
				Target: sourceScalarCanonical(dependency.target),
			})
		}
		result = append(result, symbolicDeclarationCanonicalV1{
			Kind:         string(declaration.kind),
			Symbol:       sourceScalarCanonical(declaration.symbol),
			Span:         sourceSpanCanonical(declaration.span),
			Exports:      sourceScalarsCanonical(declaration.exports),
			Facts:        factsCanonical(declaration.facts),
			Dependencies: dependencies,
		})
	}
	return result
}

func factsCanonical(values []SourceFact) []sourceFactCanonicalV1 {
	result := make([]sourceFactCanonicalV1, 0, len(values))
	for _, fact := range values {
		result = append(result, sourceFactCanonicalV1{
			Path:  fact.path,
			Value: sourceScalarCanonical(fact.value),
		})
	}
	return result
}

func sourceScalarsCanonical(values []SourceScalar) []sourceScalarCanonicalV1 {
	result := make([]sourceScalarCanonicalV1, 0, len(values))
	for _, value := range values {
		result = append(result, sourceScalarCanonical(value))
	}
	return result
}

func sourceScalarCanonical(value SourceScalar) sourceScalarCanonicalV1 {
	return sourceScalarCanonicalV1{
		Value: value.value,
		Start: value.span.start,
		End:   value.span.end,
	}
}

func sourceSpanCanonical(value SourceSpan) sourceSpanCanonicalV1 {
	return sourceSpanCanonicalV1{Start: value.start, End: value.end}
}

func projectExtensionIRFromCanonical(
	encoded projectExtensionCanonicalV1,
) (ProjectTypeEnvExtensionIR, error) {
	base, err := typedmemory.ParseTypeEnvRef(encoded.BaseTypeEnv)
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, fmt.Errorf("decode project extension base TypeEnv: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(encoded.Carrier.SourceDigest)
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, fmt.Errorf("decode project extension source digest: %w", err)
	}
	manifest, err := manifestFromCanonical(encoded.Manifest)
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, err
	}
	return ProjectTypeEnvExtensionIR{
		baseTypeEnv:    base,
		baseSource:     sourceScalarFromCanonical(encoded.BaseSource),
		boundedContext: sourceScalarFromCanonical(encoded.BoundedContext),
		carrier: ExtensionCarrierIdentity{
			schemaVersion: sourceScalarFromCanonical(encoded.Carrier.SchemaVersion),
			id:            sourceScalarFromCanonical(encoded.Carrier.ID),
			edition:       sourceScalarFromCanonical(encoded.Carrier.Edition),
			digest:        digest,
			carrierSpan:   sourceSpanFromCanonical(encoded.Carrier.CarrierSpan),
			identitySpan:  sourceSpanFromCanonical(encoded.Carrier.IdentitySpan),
		},
		manifest:  manifest,
		signature: signatureFromCanonical(encoded.Signature),
		compiler:  sourceScalarFromCanonical(encoded.Compiler),
	}, nil
}

func manifestFromCanonical(
	encoded projectManifestCanonicalV1,
) (ProjectSignatureManifestIR, error) {
	predecessors := make([]ResolvedExtensionPredecessor, 0, len(encoded.Predecessors))
	for _, value := range encoded.Predecessors {
		ref, err := typedmemory.ParseTypeEnvExtensionRef(value.ExtensionRef)
		if err != nil {
			return ProjectSignatureManifestIR{}, fmt.Errorf("decode predecessor E-ref: %w", err)
		}
		predecessors = append(predecessors, ResolvedExtensionPredecessor{
			coordinate: newManifestCoordinate(value.ManifestID, value.ManifestVersion),
			ref:        ref,
			source:     sourceScalarFromCanonical(value.Source),
		})
	}
	state := SourceScalar{}
	hasState := encoded.PublicationState != nil
	if hasState {
		state = sourceScalarFromCanonical(*encoded.PublicationState)
	}
	id := sourceScalarFromCanonical(encoded.ID)
	version := sourceScalarFromCanonical(encoded.Version)
	return ProjectSignatureManifestIR{
		coordinate:       newManifestCoordinate(id.value, version.value),
		id:               id,
		version:          version,
		hasState:         hasState,
		publicationState: state,
		predecessors:     predecessors,
		provides:         sourceScalarsFromCanonical(encoded.Provides),
		span:             sourceSpanFromCanonical(encoded.Span),
	}, nil
}

func signatureFromCanonical(encoded signatureRowsCanonicalV1) ProjectSignatureRowsIR {
	return ProjectSignatureRowsIR{
		span:    sourceSpanFromCanonical(encoded.Span),
		subject: signatureRowFromCanonical(encoded.Subject),
		vocabulary: VocabularyRowIR{
			span:         sourceSpanFromCanonical(encoded.Vocabulary.Span),
			declarations: declarationsFromCanonical(encoded.Vocabulary.Declarations),
		},
		laws:          signatureRowFromCanonical(encoded.Laws),
		applicability: signatureRowFromCanonical(encoded.Applicability),
	}
}

func signatureRowFromCanonical(encoded signatureRowCanonicalV1) SignatureRowIR {
	return SignatureRowIR{
		name:  encoded.Name,
		span:  sourceSpanFromCanonical(encoded.Span),
		facts: factsFromCanonical(encoded.Facts),
	}
}

func declarationsFromCanonical(
	values []symbolicDeclarationCanonicalV1,
) []SymbolicDeclaration {
	result := make([]SymbolicDeclaration, 0, len(values))
	for _, value := range values {
		dependencies := make([]SymbolicDependency, 0, len(value.Dependencies))
		for _, dependency := range value.Dependencies {
			dependencies = append(dependencies, SymbolicDependency{
				role:   dependency.Role,
				target: sourceScalarFromCanonical(dependency.Target),
			})
		}
		result = append(result, SymbolicDeclaration{
			kind:         localpractice.DeclarationKind(value.Kind),
			symbol:       sourceScalarFromCanonical(value.Symbol),
			span:         sourceSpanFromCanonical(value.Span),
			exports:      sourceScalarsFromCanonical(value.Exports),
			facts:        factsFromCanonical(value.Facts),
			dependencies: dependencies,
		})
	}
	return result
}

func factsFromCanonical(values []sourceFactCanonicalV1) []SourceFact {
	result := make([]SourceFact, 0, len(values))
	for _, value := range values {
		result = append(result, SourceFact{
			path:  value.Path,
			value: sourceScalarFromCanonical(value.Value),
		})
	}
	return result
}

func sourceScalarsFromCanonical(values []sourceScalarCanonicalV1) []SourceScalar {
	result := make([]SourceScalar, 0, len(values))
	for _, value := range values {
		result = append(result, sourceScalarFromCanonical(value))
	}
	return result
}

func sourceScalarFromCanonical(value sourceScalarCanonicalV1) SourceScalar {
	return SourceScalar{
		value: value.Value,
		span:  SourceSpan{start: value.Start, end: value.End},
	}
}

func sourceSpanFromCanonical(value sourceSpanCanonicalV1) SourceSpan {
	return SourceSpan{start: value.Start, end: value.End}
}

func projectExtensionDigest(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + hexDigest)
}

func deriveProjectExtensionRef(
	manifestID string,
	digest typedmemory.SHA256Digest,
) (typedmemory.TypeEnvExtensionRef, error) {
	id, err := typedmemory.NewExtensionID(manifestID)
	if err != nil {
		return typedmemory.TypeEnvExtensionRef{}, fmt.Errorf("derive project extension ID: %w", err)
	}
	raw := "typeenv-extension:" + id.String() + "@" + digest.String()
	ref, err := typedmemory.ParseTypeEnvExtensionRef(raw)
	if err != nil {
		return typedmemory.TypeEnvExtensionRef{}, fmt.Errorf("derive project extension reference: %w", err)
	}
	return ref, nil
}

func decodeStrictProjectExtensionJSON(
	payload []byte,
	target *projectExtensionCanonicalV1,
) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode project TypeEnv extension payload: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("project TypeEnv extension JSON has a trailing value")
	}
	return fmt.Errorf("decode project TypeEnv extension trailing JSON: %w", err)
}

type projectExtensionWriter struct {
	buffer bytes.Buffer
}

func newProjectExtensionWriter(domain string) projectExtensionWriter {
	writer := projectExtensionWriter{}
	writer.addString(projectExtensionCanonicalDomain)
	writer.addString(domain)
	return writer
}

func (writer *projectExtensionWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *projectExtensionWriter) addBytes(value []byte) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(len(value)))
	writer.buffer.Write(encoded[:])
	writer.buffer.Write(value)
}

func (writer projectExtensionWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type projectExtensionReader struct {
	data   []byte
	offset int
}

func decodeProjectExtensionEnvelope(canonical []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("project TypeEnv extension canonical bytes are required")
	}
	if len(canonical) > maximumProjectExtensionBytes {
		return nil, fmt.Errorf("project TypeEnv extension canonical bytes exceed %d-byte limit", maximumProjectExtensionBytes)
	}
	reader := &projectExtensionReader{data: canonical}
	root, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project extension root domain: %w", err)
	}
	if root != projectExtensionCanonicalDomain {
		return nil, fmt.Errorf("unexpected project extension root domain %q", root)
	}
	domain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode project extension artifact domain: %w", err)
	}
	if domain != projectExtensionArtifactDomain {
		return nil, fmt.Errorf("unexpected project extension artifact domain %q", domain)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("decode project extension payload: %w", err)
	}
	if reader.offset != len(reader.data) {
		return nil, fmt.Errorf("project TypeEnv extension payload has %d trailing bytes", len(reader.data)-reader.offset)
	}
	return append([]byte(nil), payload...), nil
}

func (reader *projectExtensionReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("canonical domain contains invalid UTF-8")
	}
	return string(value), nil
}

func (reader *projectExtensionReader) readBytes() ([]byte, error) {
	if reader == nil || len(reader.data)-reader.offset < 8 {
		return nil, fmt.Errorf("unexpected end of length-prefixed field")
	}
	endLength := reader.offset + 8
	length := binary.BigEndian.Uint64(reader.data[reader.offset:endLength])
	reader.offset = endLength
	remaining := len(reader.data) - reader.offset
	//nolint:gosec // remaining is non-negative after the reader bounds check above.
	if length > uint64(remaining) {
		return nil, fmt.Errorf("length-prefixed field %d exceeds remaining payload %d", length, remaining)
	}
	if length > maximumProjectExtensionBytes {
		return nil, fmt.Errorf("length-prefixed field exceeds %d bytes", maximumProjectExtensionBytes)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := reader.data[reader.offset:end]
	reader.offset = end
	return value, nil
}
