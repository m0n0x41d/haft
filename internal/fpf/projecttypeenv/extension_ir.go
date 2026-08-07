package projecttypeenv

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// SourceSpan is the exact inclusive line coordinate retained from the parsed
// Local-Practice carrier. It is descriptive source provenance, not execution
// order.
type SourceSpan struct {
	start uint64
	end   uint64
}

func (span SourceSpan) Start() uint64 { return span.start }

func (span SourceSpan) End() uint64 { return span.end }

func sourceSpan(value localpractice.SourceLineRange) SourceSpan {
	return SourceSpan{start: value.Start(), end: value.End()}
}

func (span SourceSpan) valid() bool {
	return span.start > 0 && span.end >= span.start
}

// SourceScalar retains one semantic scalar together with its exact source
// coordinate. Scalars synthesized from a closed source enum use the enclosing
// construct span because the parser deliberately exposes no narrower span.
type SourceScalar struct {
	value string
	span  SourceSpan
}

func (scalar SourceScalar) Value() string { return scalar.value }

func (scalar SourceScalar) Span() SourceSpan { return scalar.span }

func sourceScalar(value localpractice.SourceText) SourceScalar {
	return SourceScalar{value: value.Value(), span: sourceSpan(value.Span())}
}

func literalSourceScalar(value string, span localpractice.SourceLineRange) SourceScalar {
	return SourceScalar{value: value, span: sourceSpan(span)}
}

func (scalar SourceScalar) valid() bool {
	return scalar.value != "" &&
		utf8.ValidString(scalar.value) &&
		strings.TrimSpace(scalar.value) == scalar.value &&
		strings.IndexFunc(scalar.value, unicode.IsControl) < 0 &&
		scalar.span.valid()
}

type ExtensionCarrierIdentity struct {
	schemaVersion SourceScalar
	id            SourceScalar
	edition       SourceScalar
	digest        typedmemory.SHA256Digest
	carrierSpan   SourceSpan
	identitySpan  SourceSpan
}

func (identity ExtensionCarrierIdentity) SchemaVersion() SourceScalar {
	return identity.schemaVersion
}

func (identity ExtensionCarrierIdentity) ID() SourceScalar { return identity.id }

func (identity ExtensionCarrierIdentity) Edition() SourceScalar { return identity.edition }

func (identity ExtensionCarrierIdentity) Digest() typedmemory.SHA256Digest {
	return identity.digest
}

func (identity ExtensionCarrierIdentity) CarrierSpan() SourceSpan {
	return identity.carrierSpan
}

func (identity ExtensionCarrierIdentity) IdentitySpan() SourceSpan {
	return identity.identitySpan
}

type ResolvedExtensionPredecessor struct {
	coordinate ManifestCoordinate
	ref        typedmemory.TypeEnvExtensionRef
	source     SourceScalar
}

func (predecessor ResolvedExtensionPredecessor) Coordinate() ManifestCoordinate {
	return predecessor.coordinate
}

func (predecessor ResolvedExtensionPredecessor) Ref() typedmemory.TypeEnvExtensionRef {
	return predecessor.ref
}

func (predecessor ResolvedExtensionPredecessor) Source() SourceScalar {
	return predecessor.source
}

type ProjectSignatureManifestIR struct {
	coordinate       ManifestCoordinate
	id               SourceScalar
	version          SourceScalar
	hasState         bool
	publicationState SourceScalar
	predecessors     []ResolvedExtensionPredecessor
	provides         []SourceScalar
	span             SourceSpan
}

func (manifest ProjectSignatureManifestIR) Coordinate() ManifestCoordinate {
	return manifest.coordinate
}

func (manifest ProjectSignatureManifestIR) ID() SourceScalar { return manifest.id }

func (manifest ProjectSignatureManifestIR) Version() SourceScalar {
	return manifest.version
}

func (manifest ProjectSignatureManifestIR) PublicationState() (SourceScalar, bool) {
	return manifest.publicationState, manifest.hasState
}

func (manifest ProjectSignatureManifestIR) DirectPredecessors() []ResolvedExtensionPredecessor {
	return append([]ResolvedExtensionPredecessor(nil), manifest.predecessors...)
}

func (manifest ProjectSignatureManifestIR) Provides() []SourceScalar {
	return append([]SourceScalar(nil), manifest.provides...)
}

func (manifest ProjectSignatureManifestIR) Span() SourceSpan { return manifest.span }

type SourceFact struct {
	path  string
	value SourceScalar
}

func (fact SourceFact) Path() string { return fact.path }

func (fact SourceFact) Value() SourceScalar { return fact.value }

type SymbolicDependency struct {
	role   string
	target SourceScalar
}

func (dependency SymbolicDependency) Role() string { return dependency.role }

func (dependency SymbolicDependency) Target() SourceScalar { return dependency.target }

// SymbolicDeclaration is a self-reference-free source declaration. Facts keep
// the closed Local-Practice semantics and exact spans; dependencies name
// symbolic source coordinates only. No field can carry the not-yet-derived
// composite TypeEnvRef.
type SymbolicDeclaration struct {
	kind         localpractice.DeclarationKind
	symbol       SourceScalar
	span         SourceSpan
	exports      []SourceScalar
	facts        []SourceFact
	dependencies []SymbolicDependency
}

func (declaration SymbolicDeclaration) Kind() localpractice.DeclarationKind {
	return declaration.kind
}

func (declaration SymbolicDeclaration) Symbol() SourceScalar {
	return declaration.symbol
}

func (declaration SymbolicDeclaration) Span() SourceSpan { return declaration.span }

func (declaration SymbolicDeclaration) Exports() []SourceScalar {
	return append([]SourceScalar(nil), declaration.exports...)
}

func (declaration SymbolicDeclaration) Facts() []SourceFact {
	return append([]SourceFact(nil), declaration.facts...)
}

func (declaration SymbolicDeclaration) Dependencies() []SymbolicDependency {
	return append([]SymbolicDependency(nil), declaration.dependencies...)
}

// RelationDeclarationPosture exposes the current semantic classification
// without rewriting the exact historical declaration kind stored in sealed
// 1.0.0/1.1.0 carriers and E artifacts.
func (declaration SymbolicDeclaration) RelationDeclarationPosture() (
	localpractice.RelationDeclarationPosture,
	bool,
) {
	if declaration.kind != localpractice.DeclarationRelationSignature {
		return "", false
	}
	return localpractice.RelationDeclarationTypedFragment, true
}

type SignatureRowIR struct {
	name  string
	span  SourceSpan
	facts []SourceFact
}

func (row SignatureRowIR) Name() string { return row.name }

func (row SignatureRowIR) Span() SourceSpan { return row.span }

func (row SignatureRowIR) Facts() []SourceFact {
	return append([]SourceFact(nil), row.facts...)
}

type VocabularyRowIR struct {
	span         SourceSpan
	declarations []SymbolicDeclaration
}

func (row VocabularyRowIR) Span() SourceSpan { return row.span }

func (row VocabularyRowIR) Declarations() []SymbolicDeclaration {
	return cloneSymbolicDeclarations(row.declarations)
}

type ProjectSignatureRowsIR struct {
	span          SourceSpan
	subject       SignatureRowIR
	vocabulary    VocabularyRowIR
	laws          SignatureRowIR
	applicability SignatureRowIR
}

func (rows ProjectSignatureRowsIR) Span() SourceSpan { return rows.span }

func (rows ProjectSignatureRowsIR) SubjectBlock() SignatureRowIR {
	return cloneSignatureRow(rows.subject)
}

func (rows ProjectSignatureRowsIR) Vocabulary() VocabularyRowIR {
	return VocabularyRowIR{
		span:         rows.vocabulary.span,
		declarations: cloneSymbolicDeclarations(rows.vocabulary.declarations),
	}
}

func (rows ProjectSignatureRowsIR) Laws() SignatureRowIR {
	return cloneSignatureRow(rows.laws)
}

func (rows ProjectSignatureRowsIR) Applicability() SignatureRowIR {
	return cloneSignatureRow(rows.applicability)
}

// ProjectTypeEnvExtensionIR is the source-compiled E-layer input. The only
// TypeEnvRef it can contain is the exact already-compiled base B. Imports are
// exact predecessor E-refs. Concrete C-bound runtime references are
// inexpressible in this type.
type ProjectTypeEnvExtensionIR struct {
	baseTypeEnv    typedmemory.TypeEnvRef
	baseSource     SourceScalar
	boundedContext SourceScalar
	carrier        ExtensionCarrierIdentity
	manifest       ProjectSignatureManifestIR
	signature      ProjectSignatureRowsIR
	compiler       SourceScalar
}

func (ir ProjectTypeEnvExtensionIR) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return ir.baseTypeEnv
}

func (ir ProjectTypeEnvExtensionIR) BaseSource() SourceScalar { return ir.baseSource }

func (ir ProjectTypeEnvExtensionIR) BoundedContext() SourceScalar {
	return ir.boundedContext
}

func (ir ProjectTypeEnvExtensionIR) Carrier() ExtensionCarrierIdentity {
	return ir.carrier
}

func (ir ProjectTypeEnvExtensionIR) Manifest() ProjectSignatureManifestIR {
	return cloneProjectManifest(ir.manifest)
}

func (ir ProjectTypeEnvExtensionIR) Signature() ProjectSignatureRowsIR {
	return cloneProjectSignatureRows(ir.signature)
}

func (ir ProjectTypeEnvExtensionIR) CompilerVersion() SourceScalar {
	return ir.compiler
}

// CompileProjectTypeEnvExtensionIR consumes one accepted manifest node and
// resolves every direct source import to one exact predecessor E artifact.
// Caller order is irrelevant; missing, extra, duplicate, or coordinate-
// mismatched predecessors fail closed.
func CompileProjectTypeEnvExtensionIR(
	node ResolvedManifestNode,
	predecessors []ProjectTypeEnvExtensionArtifact,
) (ProjectTypeEnvExtensionIR, error) {
	parsed := node.Carrier()
	carrier := parsed.Carrier()
	base, err := typedmemory.ParseTypeEnvRef(carrier.BaseTypeEnvRef().Value())
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, fmt.Errorf("project extension base TypeEnv: %w", err)
	}
	digest, err := typedmemory.NewSHA256Digest(parsed.Digest().String())
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, fmt.Errorf("project extension source digest: %w", err)
	}
	manifest, err := compileProjectManifest(node, base, predecessors)
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, err
	}
	signature, err := compileProjectSignatureRows(carrier.Signature())
	if err != nil {
		return ProjectTypeEnvExtensionIR{}, err
	}
	identity := carrier.Identity()
	ir := ProjectTypeEnvExtensionIR{
		baseTypeEnv:    base,
		baseSource:     sourceScalar(carrier.BaseTypeEnvRef()),
		boundedContext: sourceScalar(carrier.BoundedContextRef()),
		carrier: ExtensionCarrierIdentity{
			schemaVersion: sourceScalar(carrier.SchemaVersion()),
			id:            sourceScalar(identity.ID()),
			edition:       sourceScalar(identity.Edition()),
			digest:        digest,
			carrierSpan:   sourceSpan(carrier.Span()),
			identitySpan:  sourceSpan(identity.Span()),
		},
		manifest:  manifest,
		signature: signature,
		compiler:  sourceScalar(carrier.CompilerVersion()),
	}
	return normalizeProjectTypeEnvExtensionIR(ir)
}

func compileProjectManifest(
	node ResolvedManifestNode,
	base typedmemory.TypeEnvRef,
	predecessors []ProjectTypeEnvExtensionArtifact,
) (ProjectSignatureManifestIR, error) {
	carrier := node.Carrier().Carrier()
	source := carrier.Manifest()
	artifacts := make(map[string]ProjectTypeEnvExtensionArtifact, len(predecessors))
	for _, artifact := range predecessors {
		if err := artifact.Verify(); err != nil {
			return ProjectSignatureManifestIR{}, fmt.Errorf("predecessor artifact: %w", err)
		}
		artifactBase := artifact.IR().BaseTypeEnvRef()
		if artifactBase != base {
			return ProjectSignatureManifestIR{}, fmt.Errorf(
				"predecessor artifact %q base TypeEnvRef %q does not match child base %q",
				artifact.ManifestCoordinate().String(),
				artifactBase.String(),
				base.String(),
			)
		}
		key := artifact.ManifestCoordinate().String()
		if _, exists := artifacts[key]; exists {
			return ProjectSignatureManifestIR{}, fmt.Errorf("duplicate predecessor artifact %q", key)
		}
		artifacts[key] = artifact
	}
	sourceImports := make(map[string]SourceScalar, len(source.Imports()))
	for _, item := range source.Imports() {
		scalar := sourceScalar(item.SignatureID())
		sourceImports[scalar.value] = scalar
	}
	resolved := node.Imports()
	if len(resolved) != len(sourceImports) {
		return ProjectSignatureManifestIR{}, fmt.Errorf("resolved manifest imports do not match source imports")
	}
	result := make([]ResolvedExtensionPredecessor, 0, len(resolved))
	for _, coordinate := range resolved {
		sourceImport, exists := sourceImports[coordinate.ID()]
		if !exists {
			return ProjectSignatureManifestIR{}, fmt.Errorf("resolved import %q is absent from source manifest", coordinate.String())
		}
		artifact, exists := artifacts[coordinate.String()]
		if !exists {
			return ProjectSignatureManifestIR{}, fmt.Errorf("missing exact predecessor artifact %q", coordinate.String())
		}
		if artifact.Ref().ID().String() != coordinate.ID() {
			return ProjectSignatureManifestIR{}, fmt.Errorf("predecessor %q E-ref ID does not match manifest ID", coordinate.String())
		}
		result = append(result, ResolvedExtensionPredecessor{
			coordinate: coordinate,
			ref:        artifact.Ref(),
			source:     sourceImport,
		})
		delete(artifacts, coordinate.String())
	}
	if len(artifacts) > 0 {
		return ProjectSignatureManifestIR{}, fmt.Errorf("predecessor set contains %d artifact(s) not imported by %q", len(artifacts), node.Coordinate().String())
	}
	provides := make([]SourceScalar, 0, len(source.Provides()))
	for _, item := range source.Provides() {
		provides = append(provides, sourceScalar(item.Symbol()))
	}
	state := SourceScalar{}
	publicationState, hasState := source.PublicationState()
	if hasState {
		state = literalSourceScalar(string(publicationState), source.Span())
	}
	return ProjectSignatureManifestIR{
		coordinate:       node.Coordinate(),
		id:               sourceScalar(source.ID()),
		version:          sourceScalar(source.Version()),
		hasState:         hasState,
		publicationState: state,
		predecessors:     result,
		provides:         provides,
		span:             sourceSpan(source.Span()),
	}, nil
}

func compileProjectSignatureRows(
	block localpractice.SignatureBlock,
) (ProjectSignatureRowsIR, error) {
	subject := block.SubjectBlock()
	subjectFacts := []SourceFact{
		newSourceFact("subject_kind", subject.SubjectKind()),
		newSourceFact("ranged_value_kind", subject.RangedValueKind()),
		newSourceFact("slice_set", subject.SliceSet()),
		newSourceFact("extent_rule", subject.ExtentRule()),
	}
	resultKind, hasResultKind := subject.ResultKind().Value()
	if hasResultKind {
		subjectFacts = append(subjectFacts, newSourceFact("result_kind", resultKind))
	}
	vocabulary := block.Vocabulary()
	declarations := make([]SymbolicDeclaration, 0, len(vocabulary.Declarations()))
	for _, declaration := range vocabulary.Declarations() {
		compiled, err := compileSymbolicDeclaration(declaration)
		if err != nil {
			return ProjectSignatureRowsIR{}, err
		}
		declarations = append(declarations, compiled)
	}
	laws := block.Laws()
	lawFacts := make([]SourceFact, 0, len(laws.ConstraintRefs())+len(laws.Invariants()))
	for _, reference := range laws.ConstraintRefs() {
		path := indexedPath("constraint_refs", len(lawFacts))
		lawFacts = append(lawFacts, newSourceFact(path, reference))
	}
	for index, invariant := range laws.Invariants() {
		path := indexedPath("invariants", index)
		lawFacts = append(lawFacts, newSourceFact(path, invariant))
	}
	applicability := block.Applicability()
	applicabilityFacts := []SourceFact{
		newSourceFact("bounded_context_ref", applicability.BoundedContextRef()),
	}
	for index, assumption := range applicability.Assumptions() {
		path := indexedPath("assumptions", index)
		applicabilityFacts = append(applicabilityFacts, newSourceFact(path, assumption))
	}
	return ProjectSignatureRowsIR{
		span: sourceSpan(block.Span()),
		subject: SignatureRowIR{
			name:  "subject_block",
			span:  sourceSpan(subject.Span()),
			facts: subjectFacts,
		},
		vocabulary: VocabularyRowIR{
			span:         sourceSpan(vocabulary.Span()),
			declarations: declarations,
		},
		laws: SignatureRowIR{
			name:  "laws",
			span:  sourceSpan(laws.Span()),
			facts: lawFacts,
		},
		applicability: SignatureRowIR{
			name:  "applicability",
			span:  sourceSpan(applicability.Span()),
			facts: applicabilityFacts,
		},
	}, nil
}

func compileSymbolicDeclaration(
	declaration localpractice.Declaration,
) (SymbolicDeclaration, error) {
	result := SymbolicDeclaration{
		kind:    declaration.Kind(),
		symbol:  sourceScalar(declaration.Symbol()),
		span:    sourceSpan(declaration.Span()),
		exports: []SourceScalar{sourceScalar(declaration.Symbol())},
	}
	switch value := declaration.(type) {
	case localpractice.BoundedContextDeclaration:
	case localpractice.ValueKindDeclaration:
	case localpractice.SubkindDeclaration:
		// A subkind relation has a declaration identity for exact source
		// provenance, but it does not introduce a separately addressable schema
		// symbol in typed memory.
		result.exports = nil
		result.addSourceFact("child_kind", value.ChildKind())
		result.addDependency("child_kind", value.ChildKind())
		result.addSourceFact("super_kind", value.SuperKind())
		result.addDependency("super_kind", value.SuperKind())
	case localpractice.RefKindDeclaration:
		result.addSourceFact("value_kind", value.ValueKind())
		result.addDependency("value_kind", value.ValueKind())
	case localpractice.EntitySetDefinitionDeclaration:
		result.addSourceFact("enumeration_rule", value.EnumerationRule())
		result.addDependency("enumeration_rule", value.EnumerationRule())
		policy := value.CandidatePolicy()
		result.addLiteralFact("candidate_policy.kind", string(policy.Kind()), policy.Span())
		if priorBatch, ok := policy.(localpractice.PriorBatchDeclarationsVisiblePolicy); ok {
			result.addSourceFact("candidate_policy.evaluation_rule", priorBatch.EvaluationRule())
			result.addDependency("candidate_policy.evaluation_rule", priorBatch.EvaluationRule())
		}
	case localpractice.KindSignatureDefinitionDeclaration:
		result.addSourceFact("value_kind", value.ValueKind())
		result.addDependency("value_kind", value.ValueKind())
		result.addLiteralFact("formality", value.Formality().String(), value.Span())
		for index, assumption := range value.Assumptions() {
			prefix := indexedPath("assumptions", index)
			result.addSourceFact(prefix+".carrier_ref", assumption.CarrierRef())
			result.addSourceFact(prefix+".edition", assumption.Edition())
			result.addSourceFact(prefix+".digest", assumption.Digest())
			result.addDependency(prefix+".carrier_ref", assumption.CarrierRef())
		}
		result.addSourceFact("definedness_rule", value.DefinednessRule())
		result.addDependency("definedness_rule", value.DefinednessRule())
		result.addSourceFact("evaluator_rule", value.EvaluatorRule())
		result.addDependency("evaluator_rule", value.EvaluatorRule())
		membershipBasis := value.MembershipBasis()
		result.addSourceFact("membership_basis.kind", membershipBasis.KindSource())
		if carrierFirst, ok := membershipBasis.(localpractice.CarrierFirstMembershipBasis); ok {
			result.addSourceFact(
				"membership_basis.adapter_rule",
				carrierFirst.AdapterRule(),
			)
			result.addDependency(
				"membership_basis.adapter_rule",
				carrierFirst.AdapterRule(),
			)
		}
		result.addSourceFact("entity_set", value.EntitySet())
		result.addDependency("entity_set", value.EntitySet())
	case localpractice.KindClassificationSignatureDeclaration:
		result.addSourceFact("local_kind", value.LocalKind())
		result.addDependency("local_kind", value.LocalKind())
		result.addSourceFact("candidate_value_kind", value.CandidateValueKind())
		result.addDependency("candidate_value_kind", value.CandidateValueKind())
		result.addLiteralFact("formality", value.Formality().String(), value.Span())
		result.addSourceFact("criterion_rule", value.CriterionRule())
		result.addDependency("criterion_rule", value.CriterionRule())
		result.addSourceFact("slice_conditions_rule", value.SliceConditionsRule())
		result.addDependency("slice_conditions_rule", value.SliceConditionsRule())
		referenceScheme := value.ReferenceScheme()
		result.addSourceFact("reference_scheme.carrier_ref", referenceScheme.CarrierRef())
		result.addDependency("reference_scheme.carrier_ref", referenceScheme.CarrierRef())
		result.addSourceFact("reference_scheme.edition", referenceScheme.Edition())
		result.addSourceFact("reference_scheme.digest", referenceScheme.Digest())
		for index, dependency := range value.Dependencies() {
			prefix := indexedPath("dependencies", index)
			result.addLiteralFact(prefix+".kind", string(dependency.Kind()), dependency.KindSource().Span())
			result.addSourceFact(prefix+".carrier_ref", dependency.CarrierRef())
			result.addDependency(prefix+".carrier_ref", dependency.CarrierRef())
			result.addSourceFact(prefix+".edition", dependency.Edition())
			result.addSourceFact(prefix+".digest", dependency.Digest())
		}
		extentRule := value.ExtentRule()
		result.addLiteralFact("extent_rule.kind", string(extentRule.Kind()), extentRule.Span())
		if declared, ok := extentRule.(localpractice.DeclaredKindClassificationExtentRule); ok {
			result.addSourceFact("extent_rule.rule_ref", declared.RuleRef())
			result.addDependency("extent_rule.rule_ref", declared.RuleRef())
		}
	case localpractice.TypedRelationDeclarationFragmentDeclaration:
		compileSlotFacts(&result, value.Slots())
	case localpractice.RuntimeEvaluatorInputDeclaration:
		result.addSourceFact("evaluator_requirement", value.EvaluatorRequirement())
		result.addDependency("evaluator_requirement", value.EvaluatorRequirement())
		compileSlotFacts(&result, value.Slots())
	case localpractice.ValueShapeDeclaration:
		compileValueShapeFacts(&result, value.Shape())
	case localpractice.CodecBindingDeclaration:
		result.addSourceFact("value_kind", value.ValueKind())
		result.addDependency("value_kind", value.ValueKind())
		result.addSourceFact("value_shape", value.ValueShape())
		result.addDependency("value_shape", value.ValueShape())
		result.addSourceFact("canonicalization_version", value.CanonicalizationVersion())
		for index, contract := range value.Contract() {
			result.addSourceFact(indexedPath("contract", index), contract)
		}
	case localpractice.RuntimeEvaluatorRequirementDeclaration:
		result.addSourceFact("rule_ref", value.RuleRef())
		result.addSourceFact("invocation_contract", value.InvocationContract())
	case localpractice.ConstraintDeclaration:
		compileConstraintFacts(&result, value.Rule())
	case localpractice.KindBridgeDeclaration:
		compileKindBridgeFacts(&result, value)
	default:
		return SymbolicDeclaration{}, fmt.Errorf("unsupported symbolic declaration %T", declaration)
	}
	return result, nil
}

func compileSlotFacts(result *SymbolicDeclaration, slots []localpractice.SlotSpec) {
	for _, slot := range slots {
		slotPrefix := keyedPath("slots", slot.SlotKind().Value())
		result.exports = append(result.exports, sourceScalar(slot.SlotKind()))
		result.addSourceFact(slotPrefix+".slot_kind", slot.SlotKind())
		result.addSourceFact(slotPrefix+".value_kind", slot.ValueKind())
		result.addDependency(slotPrefix+".value_kind", slot.ValueKind())
		reference := slot.ReferenceMode()
		result.addLiteralFact(
			slotPrefix+".ref_mode.kind",
			string(reference.Kind()),
			reference.Span(),
		)
		refKind, byReference := reference.(localpractice.RefKindReferenceMode)
		if !byReference {
			continue
		}
		result.addSourceFact(slotPrefix+".ref_mode.ref_kind", refKind.RefKind())
		result.addDependency(slotPrefix+".ref_mode.ref_kind", refKind.RefKind())
	}
}

func compileKindBridgeFacts(
	result *SymbolicDeclaration,
	bridge localpractice.KindBridgeDeclaration,
) {
	source := bridge.Source()
	result.addSourceFact("endpoints.source.bounded_context_ref", source.BoundedContextRef())
	result.addSourceFact("endpoints.source.edition", source.Edition())
	target := bridge.Target()
	result.addSourceFact("endpoints.target.bounded_context_ref", target.BoundedContextRef())
	result.addSourceFact("endpoints.target.edition", target.Edition())
	mapping := bridge.Mapping().(localpractice.NamedTargetKindMapping)
	result.addSourceFact("mapping.kind", mapping.KindSource())
	result.addSourceFact("mapping.source_kind", mapping.SourceKind())
	result.addDependency("mapping.source_kind", mapping.SourceKind())
	result.addSourceFact("mapping.target_kind", mapping.TargetKind())
	result.addDependency("mapping.target_kind", mapping.TargetKind())
	direction := bridge.Direction()
	result.addLiteralFact("direction", string(direction.Kind()), direction.Span())
	order := bridge.OrderPreservation()
	result.addLiteralFact("order_preservation", string(order.Kind()), order.Span())
	congruence := bridge.KindCongruence()
	result.addLiteralFact(
		"kind_congruence",
		strconv.FormatUint(uint64(congruence.Value()), 10),
		congruence.Span(),
	)
	for index, note := range bridge.LossNotes() {
		result.addSourceFact(indexedPath("loss_notes", index), note)
	}
	for index, area := range bridge.DefinednessArea() {
		result.addSourceFact(indexedPath("definedness_area", index), area)
	}
}

func compileValueShapeFacts(result *SymbolicDeclaration, shape localpractice.ValueShape) {
	result.addLiteralFact("shape.kind", string(shape.Kind()), shape.Span())
	switch value := shape.(type) {
	case localpractice.ScalarValueShape:
		result.addSourceFact("shape.scalar_kind", value.ScalarKind())
	case localpractice.RecordValueShape:
		for _, field := range value.Fields() {
			path := keyedPath("shape.fields", field.Name().Value())
			result.addSourceFact(path+".name", field.Name())
			result.addSourceFact(path+".shape", field.Shape())
			result.addDependency(path+".shape", field.Shape())
		}
	case localpractice.SumValueShape:
		for _, variant := range value.Variants() {
			path := keyedPath("shape.variants", variant.Name().Value())
			result.addSourceFact(path+".name", variant.Name())
			result.addSourceFact(path+".shape", variant.Shape())
			result.addDependency(path+".shape", variant.Shape())
		}
	case localpractice.CollectionValueShape:
		result.addSourceFact("shape.element", value.ElementShape())
		result.addDependency("shape.element", value.ElementShape())
	case localpractice.ClaimGraphValueShape:
	}
}

func compileConstraintFacts(result *SymbolicDeclaration, rule localpractice.ConstraintRule) {
	result.addLiteralFact("rule.kind", string(rule.Kind()), rule.Span())
	switch value := rule.(type) {
	case localpractice.KindDisjointConstraint:
		for index, kind := range value.Kinds() {
			path := indexedPath("rule.disjoint_kinds", index)
			result.addSourceFact(path, kind)
			result.addDependency(path, kind)
		}
	case localpractice.SlotGroupConstraint:
		result.addSourceFact("rule.relation", value.Relation())
		result.addDependency("constraint.relation", value.Relation())
		for index, slot := range value.Slots() {
			path := indexedPath("rule.slots", index)
			result.addSourceFact(path, slot)
			result.addDependency(path, slot)
		}
		result.addLiteralFact("rule.mode", string(value.Mode()), value.Span())
	case localpractice.SlotCardinalityConstraint:
		result.addSourceFact("rule.relation", value.Relation())
		result.addDependency("constraint.relation", value.Relation())
		result.addSourceFact("rule.slot", value.Slot())
		result.addDependency("constraint.slot", value.Slot())
		cardinality := value.Cardinality()
		minimum := strconv.FormatUint(cardinality.Minimum(), 10)
		result.addLiteralFact("rule.cardinality.minimum", minimum, cardinality.Span())
		maximum, bounded := cardinality.Maximum().Value()
		maximumValue := "unbounded"
		if bounded {
			maximumValue = strconv.FormatUint(maximum, 10)
		}
		result.addLiteralFact("rule.cardinality.maximum", maximumValue, cardinality.Span())
	case localpractice.ReferenceSlotSubsetConstraint:
		result.addSourceFact("rule.relation", value.Relation())
		result.addDependency("constraint.relation", value.Relation())
		result.addSourceFact("rule.subset", value.Subset())
		result.addDependency("constraint.subset", value.Subset())
		result.addSourceFact("rule.superset", value.Superset())
		result.addDependency("constraint.superset", value.Superset())
	case localpractice.ReferenceSlotPartitionConstraint:
		result.addSourceFact("rule.relation", value.Relation())
		result.addDependency("constraint.relation", value.Relation())
		result.addSourceFact("rule.whole", value.Whole())
		result.addDependency("constraint.whole", value.Whole())
		for index, part := range value.Parts() {
			path := indexedPath("rule.parts", index)
			result.addSourceFact(path, part)
			result.addDependency(path, part)
		}
	}
}

func newSourceFact(path string, value localpractice.SourceText) SourceFact {
	return SourceFact{path: path, value: sourceScalar(value)}
}

func (declaration *SymbolicDeclaration) addSourceFact(
	path string,
	value localpractice.SourceText,
) {
	declaration.facts = append(declaration.facts, newSourceFact(path, value))
}

func (declaration *SymbolicDeclaration) addLiteralFact(
	path string,
	value string,
	span localpractice.SourceLineRange,
) {
	fact := SourceFact{path: path, value: literalSourceScalar(value, span)}
	declaration.facts = append(declaration.facts, fact)
}

func (declaration *SymbolicDeclaration) addDependency(
	role string,
	target localpractice.SourceText,
) {
	dependency := SymbolicDependency{role: role, target: sourceScalar(target)}
	declaration.dependencies = append(declaration.dependencies, dependency)
}

func normalizeProjectTypeEnvExtensionIR(
	ir ProjectTypeEnvExtensionIR,
) (ProjectTypeEnvExtensionIR, error) {
	result := cloneProjectTypeEnvExtensionIR(ir)
	sort.Slice(result.manifest.predecessors, func(left, right int) bool {
		return result.manifest.predecessors[left].coordinate.String() <
			result.manifest.predecessors[right].coordinate.String()
	})
	sortSourceScalars(result.manifest.provides)
	result.signature.subject.facts = normalizeSourceFacts(result.signature.subject.facts)
	result.signature.laws.facts = normalizeSourceFacts(result.signature.laws.facts)
	result.signature.applicability.facts = normalizeSourceFacts(result.signature.applicability.facts)
	for index := range result.signature.vocabulary.declarations {
		declaration := &result.signature.vocabulary.declarations[index]
		sortSourceScalars(declaration.exports)
		declaration.facts = normalizeSourceFacts(declaration.facts)
		declaration.dependencies = normalizeSymbolicDependencies(declaration.dependencies)
	}
	sort.Slice(result.signature.vocabulary.declarations, func(left, right int) bool {
		leftDeclaration := result.signature.vocabulary.declarations[left]
		rightDeclaration := result.signature.vocabulary.declarations[right]
		if leftDeclaration.symbol.value != rightDeclaration.symbol.value {
			return leftDeclaration.symbol.value < rightDeclaration.symbol.value
		}
		return leftDeclaration.kind < rightDeclaration.kind
	})
	if err := validateProjectTypeEnvExtensionIR(result); err != nil {
		return ProjectTypeEnvExtensionIR{}, err
	}
	return result, nil
}

func validateProjectTypeEnvExtensionIR(ir ProjectTypeEnvExtensionIR) error {
	parsedBase, err := typedmemory.ParseTypeEnvRef(ir.baseTypeEnv.String())
	if err != nil || parsedBase != ir.baseTypeEnv {
		return fmt.Errorf("project extension exact base TypeEnvRef is invalid")
	}
	if !ir.baseSource.valid() || ir.baseSource.value != ir.baseTypeEnv.String() {
		return fmt.Errorf("project extension base source does not match exact base TypeEnvRef")
	}
	if !ir.boundedContext.valid() {
		return fmt.Errorf("project extension bounded context is invalid")
	}
	if err := validateCarrierIdentity(ir.carrier); err != nil {
		return err
	}
	if !ir.compiler.valid() ||
		(ir.compiler.value != SupportedCompilerVersion &&
			ir.compiler.value != CurrentCompilerVersion) {
		return fmt.Errorf(
			"project extension compiler version must be %q or %q",
			SupportedCompilerVersion,
			CurrentCompilerVersion,
		)
	}
	if err := validateProjectManifest(ir.manifest, ir.carrier); err != nil {
		return err
	}
	if err := validateProjectSignatureRows(ir.signature); err != nil {
		return err
	}
	boundedContexts := factsAtPath(ir.signature.applicability.facts, "bounded_context_ref")
	if len(boundedContexts) != 1 || boundedContexts[0].value != ir.boundedContext.value {
		return fmt.Errorf("project extension Applicability bounded context does not match carrier")
	}
	for _, declaration := range ir.signature.vocabulary.declarations {
		if declaration.kind != localpractice.DeclarationBoundedContext {
			continue
		}
		if declaration.symbol.value != ir.boundedContext.value ||
			declaration.symbol.value != boundedContexts[0].value {
			return fmt.Errorf(
				"project extension bounded_context declaration %q does not match carrier root and Applicability %q",
				declaration.symbol.value,
				ir.boundedContext.value,
			)
		}
	}
	return validateManifestDeclarationClosure(ir.manifest, ir.signature.vocabulary.declarations)
}

func validateCarrierIdentity(identity ExtensionCarrierIdentity) error {
	if !identity.schemaVersion.valid() || identity.schemaVersion.value != localpractice.SchemaVersion {
		return fmt.Errorf("project extension source schema version is invalid")
	}
	if !identity.id.valid() || !validQualifiedName(identity.id.value) || !identity.edition.valid() {
		return fmt.Errorf("project extension carrier identity is invalid")
	}
	if !identity.carrierSpan.valid() || !identity.identitySpan.valid() {
		return fmt.Errorf("project extension carrier source span is invalid")
	}
	parsedDigest, err := typedmemory.NewSHA256Digest(identity.digest.String())
	if err != nil || parsedDigest != identity.digest {
		return fmt.Errorf("project extension source digest is invalid")
	}
	return nil
}

func validateProjectManifest(
	manifest ProjectSignatureManifestIR,
	carrier ExtensionCarrierIdentity,
) error {
	if !manifest.id.valid() || !manifest.version.valid() || !manifest.span.valid() {
		return fmt.Errorf("project extension source manifest is invalid")
	}
	if manifest.coordinate.ID() != manifest.id.value || manifest.coordinate.Version() != manifest.version.value {
		return fmt.Errorf("project extension manifest coordinate does not match source")
	}
	if !isCanonicalSemVer(manifest.version.value) {
		return fmt.Errorf("project extension manifest version is not canonical SemVer")
	}
	if manifest.id.value != carrier.id.value || manifest.version.value != carrier.edition.value {
		return fmt.Errorf("project extension manifest does not match carrier identity")
	}
	if manifest.hasState {
		if !manifest.publicationState.valid() || !validPublicationState(manifest.publicationState.value) {
			return fmt.Errorf("project extension publication state is invalid")
		}
	}
	if len(manifest.predecessors) > maximumProjectExtensionEntries ||
		len(manifest.provides) > maximumProjectExtensionEntries {
		return fmt.Errorf("project extension manifest exceeds %d entries", maximumProjectExtensionEntries)
	}
	seenImports := make(map[string]struct{}, len(manifest.predecessors))
	seenImportIDs := make(map[string]struct{}, len(manifest.predecessors))
	for _, predecessor := range manifest.predecessors {
		key := predecessor.coordinate.String()
		if !validQualifiedName(predecessor.coordinate.ID()) ||
			!isCanonicalSemVer(predecessor.coordinate.Version()) ||
			!predecessor.source.valid() ||
			predecessor.source.value != predecessor.coordinate.ID() {
			return fmt.Errorf("project extension predecessor coordinate is invalid")
		}
		if predecessor.coordinate.ID() == manifest.coordinate.ID() {
			return fmt.Errorf("project extension manifest cannot import its own signature ID")
		}
		if predecessor.ref.ID().String() != predecessor.coordinate.ID() {
			return fmt.Errorf("project extension predecessor E-ref ID does not match source import")
		}
		parsed, err := typedmemory.ParseTypeEnvExtensionRef(predecessor.ref.String())
		if err != nil || parsed != predecessor.ref {
			return fmt.Errorf("project extension predecessor E-ref is invalid")
		}
		if _, exists := seenImports[key]; exists {
			return fmt.Errorf("duplicate project extension predecessor %q", key)
		}
		if _, exists := seenImportIDs[predecessor.coordinate.ID()]; exists {
			return fmt.Errorf("duplicate project extension predecessor ID %q", predecessor.coordinate.ID())
		}
		seenImports[key] = struct{}{}
		seenImportIDs[predecessor.coordinate.ID()] = struct{}{}
	}
	seenProvides := make(map[string]struct{}, len(manifest.provides))
	for _, provided := range manifest.provides {
		if !provided.valid() {
			return fmt.Errorf("project extension manifest provide is invalid")
		}
		if _, exists := seenProvides[provided.value]; exists {
			return fmt.Errorf("duplicate project extension manifest provide %q", provided.value)
		}
		seenProvides[provided.value] = struct{}{}
	}
	if len(manifest.provides) == 0 {
		return fmt.Errorf("project extension manifest requires at least one provide")
	}
	return nil
}

func validateProjectSignatureRows(rows ProjectSignatureRowsIR) error {
	if !rows.span.valid() {
		return fmt.Errorf("project extension signature span is invalid")
	}
	if err := validateSignatureRow(rows.subject, "subject_block"); err != nil {
		return err
	}
	if !rows.vocabulary.span.valid() || len(rows.vocabulary.declarations) == 0 {
		return fmt.Errorf("project extension vocabulary row is invalid")
	}
	if len(rows.vocabulary.declarations) > maximumProjectExtensionEntries {
		return fmt.Errorf("project extension vocabulary exceeds %d declarations", maximumProjectExtensionEntries)
	}
	if err := validateSignatureRow(rows.laws, "laws"); err != nil {
		return err
	}
	if err := validateSignatureRow(rows.applicability, "applicability"); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(rows.vocabulary.declarations))
	for _, declaration := range rows.vocabulary.declarations {
		if err := validateSymbolicDeclaration(declaration); err != nil {
			return err
		}
		if _, exists := seen[declaration.symbol.value]; exists {
			return fmt.Errorf("duplicate symbolic declaration %q", declaration.symbol.value)
		}
		seen[declaration.symbol.value] = struct{}{}
	}
	return nil
}

func validateSignatureRow(row SignatureRowIR, expected string) error {
	if row.name != expected || !utf8.ValidString(row.name) || !row.span.valid() {
		return fmt.Errorf("project extension %s row is invalid", expected)
	}
	if len(row.facts) > maximumProjectExtensionEntries {
		return fmt.Errorf("project extension %s row exceeds %d facts", expected, maximumProjectExtensionEntries)
	}
	seen := make(map[string]struct{}, len(row.facts))
	for _, fact := range row.facts {
		if err := validateSourceFact(fact); err != nil {
			return fmt.Errorf("project extension %s row: %w", expected, err)
		}
		key := sourceFactKey(fact)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("project extension %s row contains a duplicate fact", expected)
		}
		seen[key] = struct{}{}
	}
	return validateSignatureRowShape(row)
}

func validateSymbolicDeclaration(declaration SymbolicDeclaration) error {
	if !validDeclarationKind(declaration.kind) || !declaration.symbol.valid() || !declaration.span.valid() {
		return fmt.Errorf("project extension symbolic declaration is invalid")
	}
	requiresOwnExport := declaration.kind != localpractice.DeclarationSubkind
	if requiresOwnExport && len(declaration.exports) == 0 {
		return fmt.Errorf("symbolic declaration %q has no explicit export", declaration.symbol.value)
	}
	if !requiresOwnExport && len(declaration.exports) != 0 {
		return fmt.Errorf("symbolic subkind declaration %q must not export a schema symbol", declaration.symbol.value)
	}
	if len(declaration.exports) > maximumProjectExtensionEntries ||
		len(declaration.facts) > maximumProjectExtensionEntries ||
		len(declaration.dependencies) > maximumProjectExtensionEntries {
		return fmt.Errorf("symbolic declaration %q exceeds canonical entry budget", declaration.symbol.value)
	}
	foundOwnSymbol := false
	seenExports := make(map[string]struct{}, len(declaration.exports))
	for _, exported := range declaration.exports {
		if !exported.valid() {
			return fmt.Errorf("symbolic declaration %q has an invalid export", declaration.symbol.value)
		}
		if exported.value == declaration.symbol.value {
			foundOwnSymbol = true
		}
		if _, exists := seenExports[exported.value]; exists {
			return fmt.Errorf("symbolic declaration %q repeats export %q", declaration.symbol.value, exported.value)
		}
		seenExports[exported.value] = struct{}{}
	}
	if requiresOwnExport && !foundOwnSymbol {
		return fmt.Errorf("symbolic declaration %q does not export its own symbol", declaration.symbol.value)
	}
	seenFacts := make(map[string]struct{}, len(declaration.facts))
	for _, fact := range declaration.facts {
		if err := validateSourceFact(fact); err != nil {
			return fmt.Errorf("symbolic declaration %q: %w", declaration.symbol.value, err)
		}
		if _, exists := seenFacts[fact.path]; exists {
			return fmt.Errorf("symbolic declaration %q repeats source fact path %q", declaration.symbol.value, fact.path)
		}
		seenFacts[fact.path] = struct{}{}
	}
	seenDependencies := make(map[string]struct{}, len(declaration.dependencies))
	for _, dependency := range declaration.dependencies {
		if dependency.role == "" || !utf8.ValidString(dependency.role) || !dependency.target.valid() {
			return fmt.Errorf("symbolic declaration %q has an invalid dependency", declaration.symbol.value)
		}
		if _, exists := seenDependencies[dependency.role]; exists {
			return fmt.Errorf("symbolic declaration %q repeats dependency role %q", declaration.symbol.value, dependency.role)
		}
		seenDependencies[dependency.role] = struct{}{}
	}
	return validateSymbolicDeclarationSchema(declaration)
}

func validateSourceFact(fact SourceFact) error {
	if fact.path == "" || !utf8.ValidString(fact.path) || !fact.value.valid() {
		return fmt.Errorf("source fact is invalid")
	}
	return nil
}

func validateSignatureRowShape(row SignatureRowIR) error {
	switch row.name {
	case "subject_block":
		required := []string{"subject_kind", "ranged_value_kind", "slice_set", "extent_rule"}
		for _, path := range required {
			if len(factsAtPath(row.facts, path)) != 1 {
				return fmt.Errorf("subject_block requires exactly one %s", path)
			}
		}
		if len(factsAtPath(row.facts, "result_kind")) > 1 {
			return fmt.Errorf("subject_block permits at most one result_kind")
		}
		for _, fact := range row.facts {
			if fact.path != "subject_kind" &&
				fact.path != "ranged_value_kind" &&
				fact.path != "slice_set" &&
				fact.path != "extent_rule" &&
				fact.path != "result_kind" {
				return fmt.Errorf("subject_block contains unknown semantic path %q", fact.path)
			}
		}
		return validateSubjectRowSemantics(row)
	case "laws":
		return validateLawsRowSemantics(row)
	case "applicability":
		if len(factsAtPath(row.facts, "bounded_context_ref")) != 1 {
			return fmt.Errorf("applicability requires exactly one bounded_context_ref")
		}
		return validateApplicabilityRowSemantics(row)
	}
	return nil
}

func factsAtPath(facts []SourceFact, path string) []SourceScalar {
	result := make([]SourceScalar, 0)
	for _, fact := range facts {
		if fact.path == path {
			result = append(result, fact.value)
		}
	}
	return result
}

func validateManifestDeclarationClosure(
	manifest ProjectSignatureManifestIR,
	declarations []SymbolicDeclaration,
) error {
	realized := make(map[string]struct{})
	for _, declaration := range declarations {
		for _, exported := range declaration.exports {
			if _, exists := realized[exported.value]; exists {
				return fmt.Errorf("symbol %q is exported by more than one symbolic declaration", exported.value)
			}
			realized[exported.value] = struct{}{}
		}
	}
	if len(realized) != len(manifest.provides) {
		return fmt.Errorf("source manifest provides %d symbols but symbolic declarations export %d", len(manifest.provides), len(realized))
	}
	for _, provided := range manifest.provides {
		if _, exists := realized[provided.value]; !exists {
			return fmt.Errorf("source manifest provide %q has no symbolic declaration export", provided.value)
		}
	}
	return nil
}

func validPublicationState(value string) bool {
	switch localpractice.PublicationState(value) {
	case localpractice.PublicationDraft,
		localpractice.PublicationCandidate,
		localpractice.PublicationStable,
		localpractice.PublicationDeprecated:
		return true
	default:
		return false
	}
}

func validDeclarationKind(kind localpractice.DeclarationKind) bool {
	switch kind {
	case localpractice.DeclarationBoundedContext,
		localpractice.DeclarationValueKind,
		localpractice.DeclarationSubkind,
		localpractice.DeclarationRefKind,
		localpractice.DeclarationEntitySet,
		localpractice.DeclarationKindSignature,
		localpractice.DeclarationKindClassificationSignature,
		localpractice.DeclarationRelationSignature,
		localpractice.DeclarationRuntimeEvaluatorInput,
		localpractice.DeclarationValueShape,
		localpractice.DeclarationCodecBinding,
		localpractice.DeclarationRuntimeEvaluatorRequirement,
		localpractice.DeclarationConstraint,
		localpractice.DeclarationKindBridge:
		return true
	default:
		return false
	}
}

func normalizeSourceFacts(values []SourceFact) []SourceFact {
	result := append([]SourceFact(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftKey := sourceFactKey(result[left])
		rightKey := sourceFactKey(result[right])
		return leftKey < rightKey
	})
	return result
}

func normalizeSymbolicDependencies(values []SymbolicDependency) []SymbolicDependency {
	result := append([]SymbolicDependency(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftKey := dependencyKey(result[left])
		rightKey := dependencyKey(result[right])
		return leftKey < rightKey
	})
	return result
}

func sortSourceScalars(values []SourceScalar) {
	sort.Slice(values, func(left, right int) bool {
		return sourceScalarKey(values[left]) < sourceScalarKey(values[right])
	})
}

func sourceScalarKey(value SourceScalar) string {
	return value.value + "\x00" + strconv.FormatUint(value.span.start, 10) + "\x00" + strconv.FormatUint(value.span.end, 10)
}

func sourceFactKey(value SourceFact) string {
	return value.path + "\x00" + sourceScalarKey(value.value)
}

func dependencyKey(value SymbolicDependency) string {
	return value.role + "\x00" + sourceScalarKey(value.target)
}

func indexedPath(prefix string, index int) string {
	return prefix + "[" + fmt.Sprintf("%06d", index) + "]"
}

func keyedPath(prefix string, key string) string {
	return prefix + "[" + strconv.Itoa(len(key)) + ":" + key + "]"
}

func cloneProjectTypeEnvExtensionIR(ir ProjectTypeEnvExtensionIR) ProjectTypeEnvExtensionIR {
	ir.manifest = cloneProjectManifest(ir.manifest)
	ir.signature = cloneProjectSignatureRows(ir.signature)
	return ir
}

func cloneProjectManifest(manifest ProjectSignatureManifestIR) ProjectSignatureManifestIR {
	manifest.predecessors = append([]ResolvedExtensionPredecessor(nil), manifest.predecessors...)
	manifest.provides = append([]SourceScalar(nil), manifest.provides...)
	return manifest
}

func cloneProjectSignatureRows(rows ProjectSignatureRowsIR) ProjectSignatureRowsIR {
	rows.subject = cloneSignatureRow(rows.subject)
	rows.vocabulary.declarations = cloneSymbolicDeclarations(rows.vocabulary.declarations)
	rows.laws = cloneSignatureRow(rows.laws)
	rows.applicability = cloneSignatureRow(rows.applicability)
	return rows
}

func cloneSignatureRow(row SignatureRowIR) SignatureRowIR {
	row.facts = append([]SourceFact(nil), row.facts...)
	return row
}

func cloneSymbolicDeclarations(values []SymbolicDeclaration) []SymbolicDeclaration {
	result := append([]SymbolicDeclaration(nil), values...)
	for index := range result {
		result[index].exports = append([]SourceScalar(nil), result[index].exports...)
		result[index].facts = append([]SourceFact(nil), result[index].facts...)
		result[index].dependencies = append([]SymbolicDependency(nil), result[index].dependencies...)
	}
	return result
}
