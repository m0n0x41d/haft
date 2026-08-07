package localpractice

type KindClassificationDependencyKind string

const (
	KindClassificationDependencyAssumption     KindClassificationDependencyKind = "assumption"
	KindClassificationDependencyExternal       KindClassificationDependencyKind = "dependency"
	KindClassificationDependencyStandard       KindClassificationDependencyKind = "standard"
	KindClassificationDependencyVersion        KindClassificationDependencyKind = "version"
	KindClassificationDependencyUnit           KindClassificationDependencyKind = "unit"
	KindClassificationDependencyTemporalPolicy KindClassificationDependencyKind = "temporal_policy"
)

func (kind KindClassificationDependencyKind) valid() bool {
	switch kind {
	case KindClassificationDependencyAssumption,
		KindClassificationDependencyExternal,
		KindClassificationDependencyStandard,
		KindClassificationDependencyVersion,
		KindClassificationDependencyUnit,
		KindClassificationDependencyTemporalPolicy:
		return true
	default:
		return false
	}
}

type KindClassificationDependency struct {
	kind       KindClassificationDependencyKind
	kindSource SourceText
	carrierRef SourceText
	edition    SourceText
	digest     SourceText
	span       SourceLineRange
}

func (dependency KindClassificationDependency) Kind() KindClassificationDependencyKind {
	return dependency.kind
}

func (dependency KindClassificationDependency) KindSource() SourceText {
	return dependency.kindSource
}

func (dependency KindClassificationDependency) CarrierRef() SourceText {
	return dependency.carrierRef
}

func (dependency KindClassificationDependency) Edition() SourceText {
	return dependency.edition
}

func (dependency KindClassificationDependency) Digest() SourceText {
	return dependency.digest
}

func (dependency KindClassificationDependency) Span() SourceLineRange {
	return dependency.span
}

type KindClassificationReferenceSchemePin struct {
	carrierRef SourceText
	edition    SourceText
	digest     SourceText
	span       SourceLineRange
}

func (pin KindClassificationReferenceSchemePin) CarrierRef() SourceText {
	return pin.carrierRef
}

func (pin KindClassificationReferenceSchemePin) Edition() SourceText {
	return pin.edition
}

func (pin KindClassificationReferenceSchemePin) Digest() SourceText {
	return pin.digest
}

func (pin KindClassificationReferenceSchemePin) Span() SourceLineRange {
	return pin.span
}

type KindClassificationExtentRuleKind string

const (
	KindClassificationNoExtentRule       KindClassificationExtentRuleKind = "none"
	KindClassificationDeclaredExtentRule KindClassificationExtentRuleKind = "declared"
)

type KindClassificationExtentRule interface {
	Kind() KindClassificationExtentRuleKind
	Span() SourceLineRange
	kindClassificationExtentRuleVariant()
}

type NoKindClassificationExtentRule struct {
	span SourceLineRange
}

func (NoKindClassificationExtentRule) Kind() KindClassificationExtentRuleKind {
	return KindClassificationNoExtentRule
}

func (rule NoKindClassificationExtentRule) Span() SourceLineRange { return rule.span }

func (NoKindClassificationExtentRule) kindClassificationExtentRuleVariant() {}

type DeclaredKindClassificationExtentRule struct {
	ruleRef SourceText
	span    SourceLineRange
}

func (DeclaredKindClassificationExtentRule) Kind() KindClassificationExtentRuleKind {
	return KindClassificationDeclaredExtentRule
}

func (rule DeclaredKindClassificationExtentRule) RuleRef() SourceText {
	return rule.ruleRef
}

func (rule DeclaredKindClassificationExtentRule) Span() SourceLineRange {
	return rule.span
}

func (DeclaredKindClassificationExtentRule) kindClassificationExtentRuleVariant() {}

// KindClassificationSignatureDeclaration is the source-preserving current
// C.3.2 declaration. It has no EntitySet or MemberOf coordinate.
type KindClassificationSignatureDeclaration struct {
	symbol             SourceText
	localKind          SourceText
	candidateValueKind SourceText
	formality          SignatureFormality
	criterionRule      SourceText
	sliceConditions    SourceText
	referenceScheme    KindClassificationReferenceSchemePin
	dependencies       []KindClassificationDependency
	extentRule         KindClassificationExtentRule
	span               SourceLineRange
}

func (KindClassificationSignatureDeclaration) Kind() DeclarationKind {
	return DeclarationKindClassificationSignature
}

func (declaration KindClassificationSignatureDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration KindClassificationSignatureDeclaration) LocalKind() SourceText {
	return declaration.localKind
}

func (declaration KindClassificationSignatureDeclaration) CandidateValueKind() SourceText {
	return declaration.candidateValueKind
}

func (declaration KindClassificationSignatureDeclaration) Formality() SignatureFormality {
	return declaration.formality
}

func (declaration KindClassificationSignatureDeclaration) CriterionRule() SourceText {
	return declaration.criterionRule
}

func (declaration KindClassificationSignatureDeclaration) SliceConditionsRule() SourceText {
	return declaration.sliceConditions
}

func (declaration KindClassificationSignatureDeclaration) ReferenceScheme() KindClassificationReferenceSchemePin {
	return declaration.referenceScheme
}

func (declaration KindClassificationSignatureDeclaration) Dependencies() []KindClassificationDependency {
	return append([]KindClassificationDependency(nil), declaration.dependencies...)
}

func (declaration KindClassificationSignatureDeclaration) ExtentRule() KindClassificationExtentRule {
	return declaration.extentRule
}

func (declaration KindClassificationSignatureDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (KindClassificationSignatureDeclaration) declarationVariant() {}
