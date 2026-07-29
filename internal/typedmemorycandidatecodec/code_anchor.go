package typedmemorycandidatecodec

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	codeAnchorLocatorCodecDomain      = "haft.local-practice.typed-memory.candidate-codec.code-anchor-locator.v1"
	fileCodeAnchorTargetCodecDomain   = "haft.local-practice.typed-memory.candidate-codec.code-anchor-target.file.v1"
	symbolCodeAnchorTargetCodecDomain = "haft.local-practice.typed-memory.candidate-codec.code-anchor-target.symbol.v1"
	fileCodeAnchorTargetVariant       = "File"
	symbolCodeAnchorTargetVariant     = "Symbol"
)

// CodeAnchorTarget is the closed File-or-Symbol target sum.
type CodeAnchorTarget interface {
	codeAnchorTargetVariant()
	valid() bool
	variantName() string
	canonicalPayload() []byte
	typedTarget(TextV1) (typedmemory.TypedValue, error)
}

type FileCodeAnchorTarget struct {
	path string
}

func NewFileCodeAnchorTarget(path string) (FileCodeAnchorTarget, error) {
	if err := validateRepositoryRelativePath(path); err != nil {
		return FileCodeAnchorTarget{}, err
	}
	return FileCodeAnchorTarget{path: path}, nil
}

func (target FileCodeAnchorTarget) Path() string { return target.path }

func (FileCodeAnchorTarget) codeAnchorTargetVariant() {}

func (target FileCodeAnchorTarget) valid() bool {
	return validateRepositoryRelativePath(target.path) == nil
}

func (FileCodeAnchorTarget) variantName() string {
	return fileCodeAnchorTargetVariant
}

func (target FileCodeAnchorTarget) canonicalPayload() []byte {
	path := encodeTextWire(target.path)
	writer := newCanonicalWriter(fileCodeAnchorTargetCodecDomain)
	writer = writer.addBytes(path)
	return writer.result()
}

func (target FileCodeAnchorTarget) typedTarget(
	textCodec TextV1,
) (typedmemory.TypedValue, error) {
	pathResult := textCodec.EncodeInput(target.path)
	path, ok := pathResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(pathResult)
	}
	record, err := newTypedRecord([]typedField{
		{name: "path", value: path.Value()},
	})
	if err != nil {
		return nil, err
	}
	return newTypedSum(fileCodeAnchorTargetVariant, record)
}

type SymbolCodeAnchorTarget struct {
	path   string
	symbol string
}

func NewSymbolCodeAnchorTarget(
	path string,
	symbol string,
) (SymbolCodeAnchorTarget, error) {
	if err := validateRepositoryRelativePath(path); err != nil {
		return SymbolCodeAnchorTarget{}, err
	}
	if err := validateText(symbol); err != nil {
		return SymbolCodeAnchorTarget{}, fmt.Errorf("symbol: %w", err)
	}
	return SymbolCodeAnchorTarget{path: path, symbol: symbol}, nil
}

func (target SymbolCodeAnchorTarget) Path() string { return target.path }

func (target SymbolCodeAnchorTarget) Symbol() string { return target.symbol }

func (SymbolCodeAnchorTarget) codeAnchorTargetVariant() {}

func (target SymbolCodeAnchorTarget) valid() bool {
	return validateRepositoryRelativePath(target.path) == nil &&
		validateText(target.symbol) == nil
}

func (SymbolCodeAnchorTarget) variantName() string {
	return symbolCodeAnchorTargetVariant
}

func (target SymbolCodeAnchorTarget) canonicalPayload() []byte {
	path := encodeTextWire(target.path)
	symbol := encodeTextWire(target.symbol)
	writer := newCanonicalWriter(symbolCodeAnchorTargetCodecDomain)
	writer = writer.addBytes(path)
	writer = writer.addBytes(symbol)
	return writer.result()
}

func (target SymbolCodeAnchorTarget) typedTarget(
	textCodec TextV1,
) (typedmemory.TypedValue, error) {
	pathResult := textCodec.EncodeInput(target.path)
	path, ok := pathResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(pathResult)
	}
	symbolResult := textCodec.EncodeInput(target.symbol)
	symbol, ok := symbolResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, rejectionError(symbolResult)
	}
	record, err := newTypedRecord([]typedField{
		{name: "path", value: path.Value()},
		{name: "symbol", value: symbol.Value()},
	})
	if err != nil {
		return nil, err
	}
	return newTypedSum(symbolCodeAnchorTargetVariant, record)
}

// CodeAnchorLocator is a candidate repository/revision/target record. The
// codec can prove only its structural and lexical constraints; callers remain
// responsible for the truth of repository identity and revision immutability.
type CodeAnchorLocator struct {
	repository string
	revision   string
	target     CodeAnchorTarget
}

func NewCodeAnchorLocator(
	repository string,
	revision string,
	target CodeAnchorTarget,
) (CodeAnchorLocator, error) {
	if err := validateText(repository); err != nil {
		return CodeAnchorLocator{}, fmt.Errorf("repository: %w", err)
	}
	if err := validateText(revision); err != nil {
		return CodeAnchorLocator{}, fmt.Errorf("revision: %w", err)
	}
	if target == nil || !target.valid() {
		return CodeAnchorLocator{}, fmt.Errorf("code-anchor target is outside the closed valid sum")
	}
	return CodeAnchorLocator{
		repository: repository,
		revision:   revision,
		target:     target,
	}, nil
}

func (locator CodeAnchorLocator) Repository() string { return locator.repository }

func (locator CodeAnchorLocator) Revision() string { return locator.revision }

func (locator CodeAnchorLocator) Target() CodeAnchorTarget { return locator.target }

func (locator CodeAnchorLocator) valid() bool {
	return validateText(locator.repository) == nil &&
		validateText(locator.revision) == nil &&
		locator.target != nil &&
		locator.target.valid()
}

// CodeAnchorLocatorV1 implements the candidate locator shape and path rules.
type CodeAnchorLocatorV1 struct {
	shape typedmemory.ValueShapeRef
	text  TextV1
}

func (codec CodeAnchorLocatorV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec CodeAnchorLocatorV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("CodeAnchorLocatorV1", codec.shape, expectedShape)
	}
	value, err := decodeCodeAnchorLocatorWire(inputBytes)
	if err != nil {
		return rejectMalformed(
			"CodeAnchorLocatorV1",
			"typed_value.code_anchor_locator",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec CodeAnchorLocatorV1) EncodeInput(
	value CodeAnchorLocator,
) typedmemory.CodecCanonicalization {
	if !value.valid() {
		err := fmt.Errorf("code-anchor locator is incomplete")
		return rejectMalformed(
			"CodeAnchorLocatorV1",
			"typed_value.code_anchor_locator",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec CodeAnchorLocatorV1) canonicalizeValue(
	value CodeAnchorLocator,
) typedmemory.CodecCanonicalization {
	repositoryResult := codec.text.EncodeInput(value.repository)
	repository, ok := repositoryResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return repositoryResult
	}
	revisionResult := codec.text.EncodeInput(value.revision)
	revision, ok := revisionResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return revisionResult
	}
	target, err := value.target.typedTarget(codec.text)
	if err != nil {
		return rejectMalformed(
			"CodeAnchorLocatorV1",
			"typed_value.code_anchor_locator.target",
			err,
		)
	}
	typed, err := newTypedRecord([]typedField{
		{name: "repository", value: repository.Value()},
		{name: "revision", value: revision.Value()},
		{name: "target", value: target},
	})
	if err != nil {
		return rejectMalformed(
			"CodeAnchorLocatorV1",
			"typed_value.code_anchor_locator",
			err,
		)
	}
	canonical := encodeCodeAnchorLocatorWire(value)
	return acceptCanonical("CodeAnchorLocatorV1", typed, canonical)
}

func validateRepositoryRelativePath(path string) error {
	if err := validateText(path); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains NUL")
	}
	if strings.Contains(path, "\\") {
		return fmt.Errorf("path must use slash separators")
	}
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must be repository-relative")
	}
	segments := strings.Split(path, "/")
	if looksLikeWindowsVolume(segments[0]) {
		return fmt.Errorf("path must not contain an absolute volume prefix")
	}
	for _, segment := range segments {
		if segment == "" {
			return fmt.Errorf("path contains an empty segment")
		}
		if segment == "." || segment == ".." {
			return fmt.Errorf("path contains a dot segment")
		}
	}
	return nil
}

func looksLikeWindowsVolume(firstSegment string) bool {
	if len(firstSegment) != 2 || firstSegment[1] != ':' {
		return false
	}
	letter := firstSegment[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

func encodeCodeAnchorLocatorWire(value CodeAnchorLocator) []byte {
	repository := encodeTextWire(value.repository)
	revision := encodeTextWire(value.revision)
	writer := newCanonicalWriter(codeAnchorLocatorCodecDomain)
	writer = writer.addBytes(repository)
	writer = writer.addBytes(revision)
	writer = writer.addString(value.target.variantName())
	writer = writer.addBytes(value.target.canonicalPayload())
	return writer.result()
}

func decodeCodeAnchorLocatorWire(input []byte) (CodeAnchorLocator, error) {
	reader, err := newCanonicalReader(input, codeAnchorLocatorCodecDomain)
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	repositoryBytes, reader, err := reader.readBytes()
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	revisionBytes, reader, err := reader.readBytes()
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	variant, reader, err := reader.readString()
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	targetBytes, reader, err := reader.readBytes()
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return CodeAnchorLocator{}, err
	}
	repository, err := decodeTextWire(repositoryBytes)
	if err != nil {
		return CodeAnchorLocator{}, fmt.Errorf("repository: %w", err)
	}
	revision, err := decodeTextWire(revisionBytes)
	if err != nil {
		return CodeAnchorLocator{}, fmt.Errorf("revision: %w", err)
	}
	decoders := map[string]func([]byte) (CodeAnchorTarget, error){
		fileCodeAnchorTargetVariant:   decodeFileCodeAnchorTarget,
		symbolCodeAnchorTargetVariant: decodeSymbolCodeAnchorTarget,
	}
	decode, exists := decoders[variant]
	if !exists {
		return CodeAnchorLocator{}, fmt.Errorf("code-anchor target variant %q is unknown", variant)
	}
	target, err := decode(targetBytes)
	if err != nil {
		return CodeAnchorLocator{}, err
	}
	return NewCodeAnchorLocator(repository, revision, target)
}

func decodeFileCodeAnchorTarget(input []byte) (CodeAnchorTarget, error) {
	reader, err := newCanonicalReader(input, fileCodeAnchorTargetCodecDomain)
	if err != nil {
		return nil, err
	}
	pathBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	path, err := decodeTextWire(pathBytes)
	if err != nil {
		return nil, fmt.Errorf("file path: %w", err)
	}
	return NewFileCodeAnchorTarget(path)
}

func decodeSymbolCodeAnchorTarget(input []byte) (CodeAnchorTarget, error) {
	reader, err := newCanonicalReader(input, symbolCodeAnchorTargetCodecDomain)
	if err != nil {
		return nil, err
	}
	pathBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	symbolBytes, reader, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	path, err := decodeTextWire(pathBytes)
	if err != nil {
		return nil, fmt.Errorf("symbol path: %w", err)
	}
	symbol, err := decodeTextWire(symbolBytes)
	if err != nil {
		return nil, fmt.Errorf("symbol identity: %w", err)
	}
	return NewSymbolCodeAnchorTarget(path, symbol)
}

var _ typedmemory.CodecImplementation = CodeAnchorLocatorV1{}
