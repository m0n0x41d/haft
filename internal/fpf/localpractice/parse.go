package localpractice

import (
	"encoding/hex"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads a source carrier without lowering its symbolic declarations or
// granting any TypeEnv, staging, admission, or activation authority. It does
// not impose SemVer policy, equate carrier identity with SignatureManifest
// identity, close imports/provides, or select a runtime codec implementation;
// those are linker and runtime-registry responsibilities.
func Parse(source []byte) (ParsedCarrier, error) {
	root, err := decodeSingleDocument(source)
	if err != nil {
		return ParsedCarrier{}, err
	}
	if err := rejectUnsafeYAML(root, source); err != nil {
		return ParsedCarrier{}, err
	}
	carrier, err := parseCarrier(root)
	if err != nil {
		return ParsedCarrier{}, err
	}
	return ParsedCarrier{
		carrier: carrier,
		digest:  digestSource(source),
	}, nil
}

func parseCarrier(root *yaml.Node) (Carrier, error) {
	fields, err := mappingFields(
		root,
		"local-practice carrier",
		[]string{
			"schema_version",
			"carrier",
			"base_type_env_ref",
			"bounded_context_ref",
			"compiler_version",
			"signature_manifest",
			"signature",
		},
		nil,
	)
	if err != nil {
		return Carrier{}, err
	}
	schemaVersion, err := sourceText(fields["schema_version"], "schema_version")
	if err != nil {
		return Carrier{}, err
	}
	if schemaVersion.value != SchemaVersion {
		return Carrier{}, fmt.Errorf(
			"unsupported local-practice schema_version %q; want %q",
			schemaVersion.value,
			SchemaVersion,
		)
	}
	identity, err := parseCarrierIdentity(fields["carrier"])
	if err != nil {
		return Carrier{}, err
	}
	identity.span, err = mappingFieldLineRange(root, "carrier")
	if err != nil {
		return Carrier{}, fmt.Errorf("carrier source span: %w", err)
	}
	baseTypeEnvRef, err := sourceText(fields["base_type_env_ref"], "base_type_env_ref")
	if err != nil {
		return Carrier{}, err
	}
	if err := validateTypeEnvRef(baseTypeEnvRef.value); err != nil {
		return Carrier{}, fmt.Errorf("base_type_env_ref: %w", err)
	}
	boundedContextRef, err := sourceText(
		fields["bounded_context_ref"],
		"bounded_context_ref",
	)
	if err != nil {
		return Carrier{}, err
	}
	compilerVersion, err := sourceText(fields["compiler_version"], "compiler_version")
	if err != nil {
		return Carrier{}, err
	}
	manifest, err := parseManifest(fields["signature_manifest"])
	if err != nil {
		return Carrier{}, err
	}
	manifest.span, err = mappingFieldLineRange(root, "signature_manifest")
	if err != nil {
		return Carrier{}, fmt.Errorf("signature_manifest source span: %w", err)
	}
	signature, err := parseSignatureBlock(fields["signature"])
	if err != nil {
		return Carrier{}, err
	}
	signature.span, err = mappingFieldLineRange(root, "signature")
	if err != nil {
		return Carrier{}, fmt.Errorf("signature source span: %w", err)
	}
	applicabilityContext := signature.applicability.boundedContextRef.value
	if applicabilityContext != boundedContextRef.value {
		return Carrier{}, fmt.Errorf(
			"signature.applicability.bounded_context_ref %q does not match carrier bounded_context_ref %q",
			applicabilityContext,
			boundedContextRef.value,
		)
	}
	span, err := nodeLineRange(root)
	if err != nil {
		return Carrier{}, fmt.Errorf("local-practice carrier source span: %w", err)
	}
	return Carrier{
		schemaVersion:     schemaVersion,
		identity:          identity,
		baseTypeEnvRef:    baseTypeEnvRef,
		boundedContextRef: boundedContextRef,
		compilerVersion:   compilerVersion,
		manifest:          manifest,
		signature:         signature,
		span:              span,
	}, nil
}

func parseCarrierIdentity(node *yaml.Node) (CarrierIdentity, error) {
	fields, err := mappingFields(
		node,
		"carrier",
		[]string{"id", "edition"},
		nil,
	)
	if err != nil {
		return CarrierIdentity{}, err
	}
	id, err := qualifiedSourceText(fields["id"], "carrier.id")
	if err != nil {
		return CarrierIdentity{}, err
	}
	edition, err := sourceText(fields["edition"], "carrier.edition")
	if err != nil {
		return CarrierIdentity{}, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return CarrierIdentity{}, fmt.Errorf("carrier source span: %w", err)
	}
	return CarrierIdentity{id: id, edition: edition, span: span}, nil
}

func parseManifest(node *yaml.Node) (SignatureManifest, error) {
	fields, err := mappingFields(
		node,
		"signature_manifest",
		[]string{"id", "version", "imports", "provides"},
		[]string{"publication_state"},
	)
	if err != nil {
		return SignatureManifest{}, err
	}
	id, err := qualifiedSourceText(fields["id"], "signature_manifest.id")
	if err != nil {
		return SignatureManifest{}, err
	}
	version, err := sourceText(fields["version"], "signature_manifest.version")
	if err != nil {
		return SignatureManifest{}, err
	}
	imports, err := parseManifestImports(fields["imports"])
	if err != nil {
		return SignatureManifest{}, err
	}
	provides, err := parseManifestProvides(fields["provides"])
	if err != nil {
		return SignatureManifest{}, err
	}
	publicationState := PublicationState("")
	hasPublicationState := false
	stateNode, hasState := fields["publication_state"]
	if hasState {
		parsed, stateErr := parsePublicationState(stateNode)
		if stateErr != nil {
			return SignatureManifest{}, stateErr
		}
		publicationState = parsed
		hasPublicationState = true
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return SignatureManifest{}, fmt.Errorf("signature_manifest source span: %w", err)
	}
	return SignatureManifest{
		id:                  id,
		version:             version,
		hasPublicationState: hasPublicationState,
		publicationState:    publicationState,
		imports:             imports,
		provides:            provides,
		span:                span,
	}, nil
}

func parsePublicationState(node *yaml.Node) (PublicationState, error) {
	text, err := sourceText(node, "signature_manifest.publication_state")
	if err != nil {
		return "", err
	}
	state := PublicationState(text.value)
	switch state {
	case PublicationDraft,
		PublicationCandidate,
		PublicationStable,
		PublicationDeprecated:
		return state, nil
	default:
		return "", fmt.Errorf(
			"signature_manifest.publication_state %q is not supported",
			text.value,
		)
	}
}

func parseManifestImports(node *yaml.Node) ([]ManifestImport, error) {
	items, err := sequenceItems(node, "signature_manifest.imports", true)
	if err != nil {
		return nil, err
	}
	imports := make([]ManifestImport, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		context := fmt.Sprintf("signature_manifest.imports[%d]", index)
		signatureID, parseErr := qualifiedSourceText(item, context)
		if parseErr != nil {
			return nil, parseErr
		}
		if _, exists := seen[signatureID.value]; exists {
			return nil, fmt.Errorf("signature_manifest.imports contains duplicate signature %q", signatureID.value)
		}
		seen[signatureID.value] = struct{}{}
		imports = append(imports, ManifestImport{signatureID: signatureID})
	}
	return imports, nil
}

func parseManifestProvides(node *yaml.Node) ([]ManifestProvide, error) {
	items, err := sequenceItems(node, "signature_manifest.provides", false)
	if err != nil {
		return nil, err
	}
	provides := make([]ManifestProvide, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		context := fmt.Sprintf("signature_manifest.provides[%d]", index)
		// The manifest exports a closed union of schema symbols. Most are
		// qualified names, while bounded-context exports are opaque
		// BoundedContextRefs. The compiler validates each provide against the
		// declaration kind that realizes it.
		symbol, parseErr := sourceText(item, context)
		if parseErr != nil {
			return nil, parseErr
		}
		if _, exists := seen[symbol.value]; exists {
			return nil, fmt.Errorf("signature_manifest.provides contains duplicate symbol %q", symbol.value)
		}
		seen[symbol.value] = struct{}{}
		provides = append(provides, ManifestProvide{symbol: symbol})
	}
	return provides, nil
}

func parseSignatureBlock(node *yaml.Node) (SignatureBlock, error) {
	fields, err := mappingFields(
		node,
		"signature",
		[]string{"subject_block", "vocabulary", "laws", "applicability"},
		nil,
	)
	if err != nil {
		return SignatureBlock{}, err
	}
	subjectBlock, err := parseSubjectBlock(fields["subject_block"])
	if err != nil {
		return SignatureBlock{}, err
	}
	vocabulary, err := parseVocabulary(fields["vocabulary"])
	if err != nil {
		return SignatureBlock{}, err
	}
	laws, err := parseLaws(fields["laws"])
	if err != nil {
		return SignatureBlock{}, err
	}
	applicability, err := parseApplicability(fields["applicability"])
	if err != nil {
		return SignatureBlock{}, err
	}
	subjectBlock.span, err = mappingFieldLineRange(node, "subject_block")
	if err != nil {
		return SignatureBlock{}, fmt.Errorf("signature.subject_block source span: %w", err)
	}
	vocabulary.span, err = mappingFieldLineRange(node, "vocabulary")
	if err != nil {
		return SignatureBlock{}, fmt.Errorf("signature.vocabulary source span: %w", err)
	}
	laws.span, err = mappingFieldLineRange(node, "laws")
	if err != nil {
		return SignatureBlock{}, fmt.Errorf("signature.laws source span: %w", err)
	}
	applicability.span, err = mappingFieldLineRange(node, "applicability")
	if err != nil {
		return SignatureBlock{}, fmt.Errorf("signature.applicability source span: %w", err)
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return SignatureBlock{}, fmt.Errorf("signature source span: %w", err)
	}
	return SignatureBlock{
		subjectBlock:  subjectBlock,
		vocabulary:    vocabulary,
		laws:          laws,
		applicability: applicability,
		span:          span,
	}, nil
}

func parseSubjectBlock(node *yaml.Node) (SubjectBlock, error) {
	fields, err := mappingFields(
		node,
		"signature.subject_block",
		[]string{"subject_kind", "ranged_value_kind", "slice_set", "extent_rule"},
		[]string{"result_kind"},
	)
	if err != nil {
		return SubjectBlock{}, err
	}
	subjectKind, err := valueKindText(fields["subject_kind"], "signature.subject_block.subject_kind")
	if err != nil {
		return SubjectBlock{}, err
	}
	rangedValueKind, err := valueKindText(
		fields["ranged_value_kind"],
		"signature.subject_block.ranged_value_kind",
	)
	if err != nil {
		return SubjectBlock{}, err
	}
	sliceSet, err := qualifiedSourceText(fields["slice_set"], "signature.subject_block.slice_set")
	if err != nil {
		return SubjectBlock{}, err
	}
	extentRule, err := sourceText(fields["extent_rule"], "signature.subject_block.extent_rule")
	if err != nil {
		return SubjectBlock{}, err
	}
	resultKind := OptionalSourceText{}
	resultNode, hasResult := fields["result_kind"]
	if hasResult {
		parsed, resultErr := valueKindText(resultNode, "signature.subject_block.result_kind")
		if resultErr != nil {
			return SubjectBlock{}, resultErr
		}
		resultKind = OptionalSourceText{present: true, value: parsed}
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return SubjectBlock{}, fmt.Errorf("signature.subject_block source span: %w", err)
	}
	return SubjectBlock{
		subjectKind:     subjectKind,
		rangedValueKind: rangedValueKind,
		sliceSet:        sliceSet,
		extentRule:      extentRule,
		resultKind:      resultKind,
		span:            span,
	}, nil
}

func parseLaws(node *yaml.Node) (Laws, error) {
	fields, err := mappingFields(
		node,
		"signature.laws",
		[]string{"constraint_refs", "invariants"},
		nil,
	)
	if err != nil {
		return Laws{}, err
	}
	constraintRefs, err := parseUniqueSourceTextSequence(
		fields["constraint_refs"],
		"signature.laws.constraint_refs",
		true,
		qualifiedSourceText,
	)
	if err != nil {
		return Laws{}, err
	}
	invariants, err := parseUniqueSourceTextSequence(
		fields["invariants"],
		"signature.laws.invariants",
		false,
		sourceText,
	)
	if err != nil {
		return Laws{}, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return Laws{}, fmt.Errorf("signature.laws source span: %w", err)
	}
	return Laws{constraintRefs: constraintRefs, invariants: invariants, span: span}, nil
}

func parseApplicability(node *yaml.Node) (Applicability, error) {
	fields, err := mappingFields(
		node,
		"signature.applicability",
		[]string{"bounded_context_ref", "assumptions"},
		nil,
	)
	if err != nil {
		return Applicability{}, err
	}
	boundedContextRef, err := sourceText(
		fields["bounded_context_ref"],
		"signature.applicability.bounded_context_ref",
	)
	if err != nil {
		return Applicability{}, err
	}
	assumptions, err := parseUniqueSourceTextSequence(
		fields["assumptions"],
		"signature.applicability.assumptions",
		false,
		sourceText,
	)
	if err != nil {
		return Applicability{}, err
	}
	span, err := nodeLineRange(node)
	if err != nil {
		return Applicability{}, fmt.Errorf("signature.applicability source span: %w", err)
	}
	return Applicability{
		boundedContextRef: boundedContextRef,
		assumptions:       assumptions,
		span:              span,
	}, nil
}

type sourceTextParser func(*yaml.Node, string) (SourceText, error)

func parseUniqueSourceTextSequence(
	node *yaml.Node,
	context string,
	allowEmpty bool,
	parser sourceTextParser,
) ([]SourceText, error) {
	items, err := sequenceItems(node, context, allowEmpty)
	if err != nil {
		return nil, err
	}
	values := make([]SourceText, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		itemContext := fmt.Sprintf("%s[%d]", context, index)
		value, parseErr := parser(item, itemContext)
		if parseErr != nil {
			return nil, parseErr
		}
		if _, exists := seen[value.value]; exists {
			return nil, fmt.Errorf("%s contains duplicate value %q", context, value.value)
		}
		seen[value.value] = struct{}{}
		values = append(values, value)
	}
	return values, nil
}

func valueKindText(node *yaml.Node, context string) (SourceText, error) {
	value, err := qualifiedSourceText(node, context)
	if err != nil {
		return SourceText{}, err
	}
	if strings.HasSuffix(value.value, "Slot") || strings.HasSuffix(value.value, "Ref") {
		return SourceText{}, fmt.Errorf("%s %q violates A.6.5 ValueKind suffix discipline", context, value.value)
	}
	return value, nil
}

func validateTypeEnvRef(value string) error {
	digest, found := strings.CutPrefix(value, "typeenv:")
	if !found {
		return fmt.Errorf("must start with %q", "typeenv:")
	}
	return validateSHA256Digest(digest)
}

func validateSHA256Digest(value string) error {
	hexValue, found := strings.CutPrefix(value, "sha256:")
	if !found {
		return fmt.Errorf("must start with %q", "sha256:")
	}
	decoded, err := hex.DecodeString(hexValue)
	if err != nil {
		return fmt.Errorf("must contain lowercase SHA-256 hex: %w", err)
	}
	if len(decoded) != 32 || hexValue != strings.ToLower(hexValue) {
		return fmt.Errorf("must contain exactly 64 lowercase SHA-256 hex characters")
	}
	return nil
}
