package projecttypeenv

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type ProjectTypeEnvCompositeLoweringIssueCode string

const (
	CompositeLoweringIssueBaseInvalid       ProjectTypeEnvCompositeLoweringIssueCode = "base_invalid"
	CompositeLoweringIssueBaseMismatch      ProjectTypeEnvCompositeLoweringIssueCode = "base_mismatch"
	CompositeLoweringIssueLinkedInvalid     ProjectTypeEnvCompositeLoweringIssueCode = "linked_ir_invalid"
	CompositeLoweringIssueRuntimeInvalid    ProjectTypeEnvCompositeLoweringIssueCode = "runtime_basis_invalid"
	CompositeLoweringIssueRuntimeMismatch   ProjectTypeEnvCompositeLoweringIssueCode = "runtime_basis_mismatch"
	CompositeLoweringIssueCompositeInvalid  ProjectTypeEnvCompositeLoweringIssueCode = "composite_invalid"
	CompositeLoweringIssueCompositeMismatch ProjectTypeEnvCompositeLoweringIssueCode = "composite_recipe_mismatch"
	CompositeLoweringIssueDeclaration       ProjectTypeEnvCompositeLoweringIssueCode = "declaration_lowering_failed"
	CompositeLoweringIssueAvailability      ProjectTypeEnvCompositeLoweringIssueCode = "availability_lowering_failed"
	CompositeLoweringIssueBuild             ProjectTypeEnvCompositeLoweringIssueCode = "typeenv_build_failed"
	CompositeLoweringIssueRuntimeClosure    ProjectTypeEnvCompositeLoweringIssueCode = "runtime_closure_rejected"
	CompositeLoweringIssueVerification      ProjectTypeEnvCompositeLoweringIssueCode = "verification_sealing_failed"
)

// ProjectTypeEnvCompositeLoweringIssue is a deterministic failure witness.
// It never contains a partial TypeEnv.
type ProjectTypeEnvCompositeLoweringIssue struct {
	code    ProjectTypeEnvCompositeLoweringIssueCode
	subject string
	detail  string
	repair  string
}

func (issue ProjectTypeEnvCompositeLoweringIssue) Code() ProjectTypeEnvCompositeLoweringIssueCode {
	return issue.code
}

func (issue ProjectTypeEnvCompositeLoweringIssue) Subject() string { return issue.subject }

func (issue ProjectTypeEnvCompositeLoweringIssue) Detail() string { return issue.detail }

func (issue ProjectTypeEnvCompositeLoweringIssue) Repair() string { return issue.repair }

// ProjectTypeEnvCompositePreparationInput binds every exact ingredient. C is
// independently supplied only so the lowerer can re-derive and compare it;
// callers cannot choose a target TypeEnvRef.
type ProjectTypeEnvCompositePreparationInput struct {
	Base         typeenv.BaseTypeEnvArtifact
	Linked       LinkedProjectTypeEnvCompositeIR
	RuntimeBasis RuntimeEvaluationBasisArtifact
	Composite    ProjectTypeEnvCompositeArtifact
}

// ProjectTypeEnvCompositePreparation is a closed prepared/rejected result.
// Rejection makes an environment inexpressible; preparation carries no Stage,
// head selection, persistence, or authority effect.
type ProjectTypeEnvCompositePreparation interface {
	Rejected() bool
	Issues() []ProjectTypeEnvCompositeLoweringIssue
	Environment() (typedmemory.TypeEnv, bool)
	Verification() (ProjectTypeEnvCompositeVerification, bool)
	ExecutableSnapshot() (ProjectTypeEnvExecutableSnapshot, bool)
	projectTypeEnvCompositePreparationVariant()
}

type preparedProjectTypeEnvComposite struct {
	environment  typedmemory.TypeEnv
	verification ProjectTypeEnvCompositeVerification
	snapshot     ProjectTypeEnvExecutableSnapshot
}

func (preparedProjectTypeEnvComposite) Rejected() bool { return false }

func (preparedProjectTypeEnvComposite) Issues() []ProjectTypeEnvCompositeLoweringIssue {
	return nil
}

func (preparation preparedProjectTypeEnvComposite) Environment() (typedmemory.TypeEnv, bool) {
	return preparation.environment, true
}

func (preparation preparedProjectTypeEnvComposite) Verification() (
	ProjectTypeEnvCompositeVerification,
	bool,
) {
	return preparation.verification, true
}

func (preparation preparedProjectTypeEnvComposite) ExecutableSnapshot() (
	ProjectTypeEnvExecutableSnapshot,
	bool,
) {
	return preparation.snapshot, true
}

func (preparedProjectTypeEnvComposite) projectTypeEnvCompositePreparationVariant() {}

type rejectedProjectTypeEnvComposite struct {
	issues []ProjectTypeEnvCompositeLoweringIssue
}

func (rejectedProjectTypeEnvComposite) Rejected() bool { return true }

func (preparation rejectedProjectTypeEnvComposite) Issues() []ProjectTypeEnvCompositeLoweringIssue {
	return append([]ProjectTypeEnvCompositeLoweringIssue(nil), preparation.issues...)
}

func (rejectedProjectTypeEnvComposite) Environment() (typedmemory.TypeEnv, bool) {
	return typedmemory.TypeEnv{}, false
}

func (rejectedProjectTypeEnvComposite) Verification() (
	ProjectTypeEnvCompositeVerification,
	bool,
) {
	return ProjectTypeEnvCompositeVerification{}, false
}

func (rejectedProjectTypeEnvComposite) ExecutableSnapshot() (
	ProjectTypeEnvExecutableSnapshot,
	bool,
) {
	return ProjectTypeEnvExecutableSnapshot{}, false
}

func (rejectedProjectTypeEnvComposite) projectTypeEnvCompositePreparationVariant() {}

type compositeSourceDeclaration struct {
	extension LinkedCompositeExtension
	value     SymbolicDeclaration
}

type compositeLoweredDeclarations struct {
	contexts                 []typedmemory.BoundedContext
	kinds                    []typedmemory.KindDefinition
	entitySets               []typedmemory.EntitySetDefinition
	kindSignatures           []typedmemory.KindSignatureDefinition
	classificationSignatures []typedmemory.KindClassificationSignatureDefinition
	refKinds                 []typedmemory.RefKindDefinition
	subkinds                 []typedmemory.SubkindRelation
	bridges                  []typedmemory.ContextBridge
	relationFragments        []typedmemory.TypedRelationDeclarationFragment
	shapes                   []typedmemory.ValueShapeDeclaration
	bindings                 []typedmemory.ValueBinding
	constraints              []typedmemory.ConstraintRule
}

// PrepareProjectTypeEnvComposite is the pure final B + E DAG + X -> C
// lowering boundary. Every input is reverified, every TypeEnv-scoped reference
// is rebound to the derived C, and the public runtime-closure resolver is the
// last gate. No effect is performed.
func PrepareProjectTypeEnvComposite(
	input ProjectTypeEnvCompositePreparationInput,
) ProjectTypeEnvCompositePreparation {
	verified, issue := verifyProjectTypeEnvCompositePreparationInput(input)
	if issue != nil {
		return rejectProjectTypeEnvComposite(*issue)
	}
	candidate, issue := lowerVerifiedProjectTypeEnvCompositeCandidate(verified)
	if issue != nil {
		return rejectProjectTypeEnvComposite(*issue)
	}

	closure := ResolveProjectTypeEnvCompositeRuntimeRequirements(
		verified.composite,
		candidate,
		verified.linked,
		verified.runtime,
	)
	if closure.Rejected() {
		return rejectProjectTypeEnvComposite(runtimeClosureLoweringIssues(closure.Issues())...)
	}
	verification, err := sealProjectTypeEnvCompositeVerification(
		verified.base,
		verified.linked,
		verified.runtime,
		verified.composite,
		candidate,
	)
	if err != nil {
		return rejectProjectTypeEnvComposite(newCompositeLoweringIssue(
			CompositeLoweringIssueVerification,
			verified.composite.Ref().String(),
			err.Error(),
			"repair the exact lowering result; no Stage may consume an unverified environment",
		))
	}
	snapshot, err := sealProjectTypeEnvExecutableSnapshot(
		input,
		candidate,
		verification,
	)
	if err != nil {
		return rejectProjectTypeEnvComposite(newCompositeLoweringIssue(
			CompositeLoweringIssueVerification,
			verified.composite.Ref().String(),
			"seal executable TypeEnv snapshot: "+err.Error(),
			"repair the exact closure until it round-trips through the executable snapshot codec",
		))
	}
	return preparedProjectTypeEnvComposite{
		environment:  candidate,
		verification: verification,
		snapshot:     snapshot,
	}
}

func lowerVerifiedProjectTypeEnvCompositeCandidate(
	verified verifiedProjectTypeEnvCompositePreparation,
) (typedmemory.TypeEnv, *ProjectTypeEnvCompositeLoweringIssue) {
	baseEnvironment, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		verified.base,
		verified.composite.Ref(),
	)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueBaseInvalid,
			verified.baseRef.String(),
			"lower exact FPF base at composite C: "+err.Error(),
			"repair and recompile the exact FPF base artifact",
		)
		return typedmemory.TypeEnv{}, &issue
	}

	sources := canonicalCompositeSourceDeclarations(verified.linked)
	lowered, err := lowerCompositeDeclarations(
		verified.linked,
		verified.composite.Ref(),
		baseEnvironment,
		sources,
	)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueDeclaration,
			verified.composite.Ref().String(),
			err.Error(),
			"repair the exact Local-Practice declaration and reseal E and C",
		)
		return typedmemory.TypeEnv{}, &issue
	}

	derivedAvailabilities, err := lowerCompositeContextKindAvailabilities(verified.linked)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueAvailability,
			verified.composite.Ref().String(),
			err.Error(),
			"repair the exact context, provider, use, or KindBridge grounds",
		)
		return typedmemory.TypeEnv{}, &issue
	}
	availabilities, err := mergeCompositeContextKindAvailabilities(
		baseEnvironment.ContextKindAvailabilities(),
		derivedAvailabilities,
	)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueAvailability,
			verified.composite.Ref().String(),
			err.Error(),
			"repair conflicting base and Local-Practice context-kind availability grounds",
		)
		return typedmemory.TypeEnv{}, &issue
	}

	candidate, err := buildCompositeTypeEnv(
		verified.composite.Ref(),
		baseEnvironment,
		lowered,
		availabilities,
	)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueBuild,
			verified.composite.Ref().String(),
			err.Error(),
			"repair the source declarations until the closed TypeEnv validators accept C",
		)
		return typedmemory.TypeEnv{}, &issue
	}
	return candidate, nil
}

type verifiedProjectTypeEnvCompositePreparation struct {
	base      typeenv.BaseTypeEnvArtifact
	baseRef   typedmemory.TypeEnvRef
	linked    LinkedProjectTypeEnvCompositeIR
	runtime   RuntimeEvaluationBasisArtifact
	composite ProjectTypeEnvCompositeArtifact
}

func verifyProjectTypeEnvCompositePreparationInput(
	input ProjectTypeEnvCompositePreparationInput,
) (verifiedProjectTypeEnvCompositePreparation, *ProjectTypeEnvCompositeLoweringIssue) {
	if err := input.Base.Verify(); err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueBaseInvalid,
			"compiled-fpf-base",
			err.Error(),
			"supply the exact verified executable BaseTypeEnvArtifact",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	baseRef, exists := input.Base.TypeEnvRef()
	if !exists {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueBaseInvalid,
			"compiled-fpf-base",
			"coverage-only base has no executable TypeEnvRef",
			"compile an executable FPF base before composing project memory",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	linked, err := verifyLinkedProjectTypeEnvCompositeIR(input.Linked)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueLinkedInvalid,
			"linked-B-E",
			err.Error(),
			"re-link the exact verified B and E artifacts",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if linked.BaseTypeEnvRef() != baseRef ||
		!bytes.Equal(linked.BaseArtifact().CanonicalBytes(), input.Base.CanonicalBytes()) {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueBaseMismatch,
			baseRef.String(),
			"explicit B does not byte-match the base authenticated by linked E",
			"supply the exact B used to build the linked composite IR",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if err := input.RuntimeBasis.Verify(); err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueRuntimeInvalid,
			"runtime-basis-X",
			err.Error(),
			"supply the exact verified RuntimeEvaluationBasisArtifact",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if err := input.RuntimeBasis.VerifyResolvedClosure(); err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueRuntimeInvalid,
			input.RuntimeBasis.Ref().String(),
			"resolved mechanism closure: "+err.Error(),
			"load the exact canonical mechanism artifacts pinned by X",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if err := input.Composite.Verify(); err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueCompositeInvalid,
			"composite-C",
			err.Error(),
			"supply the exact verified ProjectTypeEnvCompositeArtifact",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	resealed, err := sealProjectTypeEnvCompositeAtSchema(
		linked,
		input.RuntimeBasis,
		input.Composite.LowererSchemaVersion(),
	)
	if err != nil {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueCompositeMismatch,
			input.Composite.Ref().String(),
			"rederive C from exact B/E/X: "+err.Error(),
			"repair B, E, or X and reseal C",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if resealed.Ref() != input.Composite.Ref() ||
		!bytes.Equal(resealed.CanonicalBytes(), input.Composite.CanonicalBytes()) {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueCompositeMismatch,
			input.Composite.Ref().String(),
			"supplied C is not the exact canonical recipe derived from B, linked E, and X",
			"use the C returned by SealProjectTypeEnvComposite for these exact inputs",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	if input.Composite.RuntimeEvaluationBasisRef() != input.RuntimeBasis.Ref() {
		issue := newCompositeLoweringIssue(
			CompositeLoweringIssueRuntimeMismatch,
			input.RuntimeBasis.Ref().String(),
			"supplied X does not equal the runtime basis bound into C",
			"supply the exact X used to seal C",
		)
		return verifiedProjectTypeEnvCompositePreparation{}, &issue
	}
	return verifiedProjectTypeEnvCompositePreparation{
		base:      input.Base,
		baseRef:   baseRef,
		linked:    linked,
		runtime:   input.RuntimeBasis,
		composite: input.Composite,
	}, nil
}

func canonicalCompositeSourceDeclarations(
	linked LinkedProjectTypeEnvCompositeIR,
) []compositeSourceDeclaration {
	result := make([]compositeSourceDeclaration, 0)
	for _, extension := range linked.Extensions() {
		declarations := extension.Artifact().IR().Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			result = append(result, compositeSourceDeclaration{
				extension: extension,
				value:     declaration,
			})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftRef := result[left].extension.Ref().String()
		rightRef := result[right].extension.Ref().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		leftKind := result[left].value.Kind()
		rightKind := result[right].value.Kind()
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		return result[left].value.Symbol().Value() < result[right].value.Symbol().Value()
	})
	return result
}

func buildCompositeTypeEnv(
	ref typedmemory.TypeEnvRef,
	base typedmemory.TypeEnv,
	lowered compositeLoweredDeclarations,
	availabilities []typedmemory.ContextKindAvailability,
) (typedmemory.TypeEnv, error) {
	builder := typedmemory.NewTypeEnvBuilder(ref)
	builder = builder.SetSourceRevision(base.SourceRevision())
	builder = builder.SetCompilerSchemaVersion(base.CompilerSchemaVersion())
	builder = builder.SetCoverageManifest(base.CoverageManifest())
	for _, value := range base.BoundedContexts() {
		builder = builder.AddBoundedContext(value)
	}
	for _, value := range lowered.contexts {
		builder = builder.AddBoundedContext(value)
	}
	for _, value := range base.KindDefinitions() {
		builder = builder.AddKindDefinition(value)
	}
	for _, value := range lowered.kinds {
		builder = builder.AddKindDefinition(value)
	}
	for _, value := range base.EntitySetDefinitions() {
		builder = builder.AddEntitySetDefinition(value)
	}
	for _, value := range lowered.entitySets {
		builder = builder.AddEntitySetDefinition(value)
	}
	for _, value := range base.KindSignatureDefinitions() {
		builder = builder.AddKindSignatureDefinition(value)
	}
	for _, value := range lowered.kindSignatures {
		builder = builder.AddKindSignatureDefinition(value)
	}
	for _, value := range base.KindClassificationSignatureDefinitions() {
		builder = builder.AddKindClassificationSignatureDefinition(value)
	}
	for _, value := range lowered.classificationSignatures {
		builder = builder.AddKindClassificationSignatureDefinition(value)
	}
	for _, value := range base.RefKindDefinitions() {
		builder = builder.AddRefKindDefinition(value)
	}
	for _, value := range lowered.refKinds {
		builder = builder.AddRefKindDefinition(value)
	}
	for _, value := range availabilities {
		builder = builder.AddContextKindAvailability(value)
	}
	for _, value := range base.SubkindRelations() {
		builder = builder.AddSubkindRelation(value)
	}
	for _, value := range lowered.subkinds {
		builder = builder.AddSubkindRelation(value)
	}
	for _, value := range base.ContextBridges() {
		builder = builder.AddContextBridge(value)
	}
	for _, value := range lowered.bridges {
		builder = builder.AddContextBridge(value)
	}
	for _, value := range base.TypedRelationDeclarationFragments() {
		builder = builder.AddTypedRelationDeclarationFragment(value)
	}
	for _, value := range lowered.relationFragments {
		builder = builder.AddTypedRelationDeclarationFragment(value)
	}
	for _, value := range base.ValueShapes() {
		builder = builder.AddValueShape(value)
	}
	for _, value := range lowered.shapes {
		builder = builder.AddValueShape(value)
	}
	for _, value := range base.ValueBindings() {
		builder = builder.AddValueBinding(value)
	}
	for _, value := range lowered.bindings {
		builder = builder.AddValueBinding(value)
	}
	for _, value := range base.Constraints() {
		builder = builder.AddConstraint(value)
	}
	for _, value := range lowered.constraints {
		builder = builder.AddConstraint(value)
	}
	return builder.Build()
}

func newCompositeLoweringIssue(
	code ProjectTypeEnvCompositeLoweringIssueCode,
	subject string,
	detail string,
	repair string,
) ProjectTypeEnvCompositeLoweringIssue {
	return ProjectTypeEnvCompositeLoweringIssue{
		code:    code,
		subject: subject,
		detail:  detail,
		repair:  repair,
	}
}

func rejectProjectTypeEnvComposite(
	issues ...ProjectTypeEnvCompositeLoweringIssue,
) ProjectTypeEnvCompositePreparation {
	owned := append([]ProjectTypeEnvCompositeLoweringIssue(nil), issues...)
	sort.Slice(owned, func(left, right int) bool {
		leftKey := string(owned[left].code) + "\x00" + owned[left].subject + "\x00" + owned[left].detail
		rightKey := string(owned[right].code) + "\x00" + owned[right].subject + "\x00" + owned[right].detail
		return leftKey < rightKey
	})
	return rejectedProjectTypeEnvComposite{issues: owned}
}

func runtimeClosureLoweringIssues(
	issues []CompositeRuntimeRequirementIssue,
) []ProjectTypeEnvCompositeLoweringIssue {
	result := make([]ProjectTypeEnvCompositeLoweringIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, newCompositeLoweringIssue(
			CompositeLoweringIssueRuntimeClosure,
			issue.SemanticReference(),
			fmt.Sprintf("%s: %s", issue.Code(), issue.Detail()),
			issue.Repair(),
		))
	}
	if len(result) == 0 {
		result = append(result, newCompositeLoweringIssue(
			CompositeLoweringIssueRuntimeClosure,
			"runtime-closure",
			"runtime closure was rejected without a diagnostic",
			"repair the exact X mechanism closure",
		))
	}
	return result
}

func declarationsOfKind(
	sources []compositeSourceDeclaration,
	kind localpractice.DeclarationKind,
) []compositeSourceDeclaration {
	result := make([]compositeSourceDeclaration, 0)
	for _, source := range sources {
		if source.value.Kind() == kind {
			result = append(result, source)
		}
	}
	return result
}
