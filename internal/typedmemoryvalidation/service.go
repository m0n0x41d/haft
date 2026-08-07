package typedmemoryvalidation

import (
	"fmt"
	"reflect"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	DiagnosticMalformedValidationRequest = "malformed_validation_request"
	DiagnosticBasisResolutionUnavailable = "basis_resolution_unavailable"
	DiagnosticBasisResolutionMismatch    = "basis_resolution_selector_mismatch"
	DiagnosticProjectTypeEnvUnavailable  = "project_active_typeenv_unavailable"
	DiagnosticProjectSnapshotUnavailable = "project_snapshot_unavailable"
	DiagnosticExactBasisMismatch         = "exact_project_basis_mismatch"
	DiagnosticCandidateNotProject        = "candidate_not_project_snapshot"
	DiagnosticChangeSetBindFailed        = "change_set_bind_failed"
	DiagnosticDigestUnavailable          = "normalized_digest_unavailable"
	DiagnosticProjectionUnavailable      = "diagnostic_projection_unavailable"
)

const validationBindRule = "haft.memory.validate.v1.bind"

type Service struct {
	resolver BasisResolver
}

func NewService(resolver BasisResolver) (Service, error) {
	if !basisResolverPresent(resolver) {
		return Service{}, fmt.Errorf("typed-memory BasisResolver is required")
	}
	return Service{resolver: resolver}, nil
}

func (service Service) Validate(request typedmemorywire.ValidateRequest) Response {
	outcome := service.Evaluate(request)
	return presentOutcome(outcome)
}

func PresentOutcome(outcome Outcome) Response {
	return presentOutcome(outcome)
}

// Evaluate performs validation once and retains the sealed admission
// capability only on ValidOutcome. It has no persistence effects.
func (service Service) Evaluate(request typedmemorywire.ValidateRequest) Outcome {
	if !typedmemorywire.IsDecodedValidateRequest(request) {
		return invalidRequestOutcome(typedmemorywire.ContractVersionV1)
	}
	return service.evaluate(request)
}

type validationRequest interface {
	ContractVersion() string
	Action() string
	Basis() typedmemorywire.BasisSelector
	ChangeCount() int
	BindChangeSet(typedmemory.TypeEnvRef) (typedmemory.MemoryChangeSet, error)
}

type semanticDiagnosticRequest interface {
	usesSemanticDiagnosticPaths()
}

func (service Service) evaluate(request validationRequest) Outcome {
	contractVersion := validationContractVersion(request)
	if !validationRequestPresent(request) {
		return invalidRequestOutcome(contractVersion)
	}
	selector := request.Basis()
	if selector == nil {
		return invalidRequestOutcome(contractVersion)
	}
	resolution := service.resolver.Resolve(selector)
	if !basisResolutionPresent(resolution) {
		return unavailableResolutionOutcome(contractVersion, selector)
	}

	switch requested := selector.(type) {
	case typedmemorywire.BundledCandidateOpenWorldSelector:
		return service.evaluateBundled(
			contractVersion,
			request,
			requested,
			resolution,
		)
	case typedmemorywire.ProjectCurrentSelector:
		return service.evaluateCurrentProject(
			contractVersion,
			request,
			requested,
			resolution,
		)
	case typedmemorywire.ExactProjectSelector:
		return service.evaluateExactProject(
			contractVersion,
			request,
			requested,
			resolution,
		)
	default:
		return invalidRequestOutcome(contractVersion)
	}
}

func (service Service) evaluateBundled(
	contractVersion string,
	request validationRequest,
	selector typedmemorywire.BundledCandidateOpenWorldSelector,
	resolution BasisResolution,
) Outcome {
	basis, matches := resolution.(*BundledCandidateOpenWorldBasis)
	if !matches || basis == nil || !typeEnvPresent(basis.environment) {
		return resolutionMismatchOutcome(contractVersion, selector, resolution)
	}
	projection := bundledProjection(selector, basis)
	_, err := request.BindChangeSet(basis.environment.Ref())
	if err != nil {
		return bindFailureOutcome(contractVersion, projection, err)
	}
	diagnostic := newUnderDiagnostic(
		DiagnosticCandidateNotProject,
		"the bundled FPF TypeEnv can lower this candidate but is not a project snapshot or admission basis",
		"$.basis",
		"resolved-project-memory-snapshot",
		"bundled-candidate-open-world",
		"validate-against-project-current-after-typed-memory-storage-is-active",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func (service Service) evaluateCurrentProject(
	contractVersion string,
	request validationRequest,
	selector typedmemorywire.ProjectCurrentSelector,
	resolution BasisResolution,
) Outcome {
	switch basis := resolution.(type) {
	case *ProjectBasisUnavailable:
		if basis == nil {
			return unavailableResolutionOutcome(contractVersion, selector)
		}
		return projectUnavailableOutcome(contractVersion, selector)
	case *ResolvedProjectBasis:
		if !resolvedProjectBasisPresent(basis) {
			return unavailableResolutionOutcome(contractVersion, selector)
		}
		projection := resolvedProjectProjection(selector, basis)
		return service.evaluateResolvedProject(
			contractVersion,
			request,
			basis,
			projection,
		)
	default:
		return resolutionMismatchOutcome(contractVersion, selector, resolution)
	}
}

func (service Service) evaluateExactProject(
	contractVersion string,
	request validationRequest,
	selector typedmemorywire.ExactProjectSelector,
	resolution BasisResolution,
) Outcome {
	switch basis := resolution.(type) {
	case *ProjectBasisUnavailable:
		if basis == nil {
			return unavailableResolutionOutcome(contractVersion, selector)
		}
		return projectUnavailableOutcome(contractVersion, selector)
	case *ExactProjectBasisMismatch:
		if basis == nil || basis.observedTypeEnv.Digest().String() == "" {
			return unavailableResolutionOutcome(contractVersion, selector)
		}
		if exactObservationMatches(selector, basis.observedTypeEnv, basis.observedGraphRevision) {
			return resolutionMismatchOutcome(contractVersion, selector, resolution)
		}
		return exactMismatchOutcome(
			contractVersion,
			selector,
			basis.observedTypeEnv,
			basis.observedGraphRevision,
		)
	case *ResolvedProjectBasis:
		if !resolvedProjectBasisPresent(basis) {
			return unavailableResolutionOutcome(contractVersion, selector)
		}
		if !exactSelectorMatches(selector, basis) {
			return exactMismatchOutcome(
				contractVersion,
				selector,
				basis.environment.Ref(),
				basis.snapshot.GraphRevision(),
			)
		}
		projection := resolvedProjectProjection(selector, basis)
		return service.evaluateResolvedProject(
			contractVersion,
			request,
			basis,
			projection,
		)
	default:
		return resolutionMismatchOutcome(contractVersion, selector, resolution)
	}
}

func (service Service) evaluateResolvedProject(
	contractVersion string,
	request validationRequest,
	basis *ResolvedProjectBasis,
	projection BasisProjection,
) Outcome {
	changeSet, err := request.BindChangeSet(basis.environment.Ref())
	if err != nil {
		return bindFailureOutcome(contractVersion, projection, err)
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		basis.environment,
		basis.codecs,
		basis.snapshot,
		changeSet,
	)
	pathProjector, pathErr := diagnosticPathProjectorFor(request)
	if pathErr != nil {
		return projectionUnavailableOutcome(contractVersion, projection, pathErr)
	}
	switch result := verdict.(type) {
	case typedmemory.Valid:
		outcome, outcomeErr := newValidOutcome(
			contractVersion,
			projection,
			changeSet,
			result,
		)
		if outcomeErr != nil {
			return digestUnavailableOutcome(contractVersion, projection, outcomeErr)
		}
		return outcome
	case typedmemory.Invalid:
		diagnostics, projectionErr := projectCoreDiagnostics(
			result.Diagnostics(),
			pathProjector,
		)
		if projectionErr != nil {
			return projectionUnavailableOutcome(
				contractVersion,
				projection,
				projectionErr,
			)
		}
		return newInvalidOutcome(contractVersion, projection, diagnostics)
	case typedmemory.Underdetermined:
		diagnostics, projectionErr := projectCoreDiagnostics(
			result.Diagnostics(),
			pathProjector,
		)
		if projectionErr != nil {
			return projectionUnavailableOutcome(
				contractVersion,
				projection,
				projectionErr,
			)
		}
		return newUnderdeterminedOutcome(
			contractVersion,
			projection,
			diagnostics,
		)
	default:
		return digestUnavailableOutcome(
			contractVersion,
			projection,
			fmt.Errorf("validator returned unknown verdict"),
		)
	}
}

func diagnosticPathProjectorFor(
	request validationRequest,
) (coreDiagnosticPathProjector, error) {
	if _, semantic := request.(semanticDiagnosticRequest); semantic {
		return semanticDiagnosticPathProjector{}, nil
	}
	return newDiagnosticPathProjector(request)
}

func exactSelectorMatches(
	selector typedmemorywire.ExactProjectSelector,
	basis *ResolvedProjectBasis,
) bool {
	return exactObservationMatches(
		selector,
		basis.environment.Ref(),
		basis.snapshot.GraphRevision(),
	)
}

func exactObservationMatches(
	selector typedmemorywire.ExactProjectSelector,
	observedTypeEnv typedmemory.TypeEnvRef,
	observedGraphRevision typedmemory.GraphRevision,
) bool {
	if selector.RequestedTypeEnvDigest() != observedTypeEnv.Digest() {
		return false
	}
	return selector.RequestedGraphRevision() == observedGraphRevision
}

func resolvedProjectBasisPresent(basis *ResolvedProjectBasis) bool {
	if basis == nil || !typeEnvPresent(basis.environment) {
		return false
	}
	if !memorySnapshotPresent(basis.snapshot) {
		return false
	}
	return basis.snapshot.TypeEnvRef() == basis.environment.Ref()
}

func projectCoreDiagnostics(
	values []typedmemory.Diagnostic,
	pathProjector coreDiagnosticPathProjector,
) ([]DiagnosticProjection, error) {
	diagnostics := make([]DiagnosticProjection, 0, len(values))
	for _, value := range values {
		path, err := pathProjector.project(
			value.Code(),
			value.Message(),
			value.Path(),
		)
		if err != nil {
			return nil, err
		}
		witness, err := projectDiagnosticWitness(value.Witness())
		if err != nil {
			return nil, err
		}
		basis, err := projectDiagnosticBasis(value.GoverningBasis())
		if err != nil {
			return nil, err
		}
		repairs, err := projectRepairCandidates(value.RepairCandidates())
		if err != nil {
			return nil, err
		}
		projection := DiagnosticProjection{
			posture:  value.Posture(),
			code:     string(value.Code()),
			message:  value.Message(),
			path:     path.value,
			pathKind: path.kind,
			witness:  witness,
			basis:    basis,
			repairs:  repairs,
		}
		diagnostics = append(diagnostics, projection)
	}
	return diagnostics, nil
}

func projectDiagnosticWitness(
	witness typedmemory.DiagnosticWitness,
) (DiagnosticWitnessProjection, error) {
	expected, err := projectDiagnosticDatum(witness.Expected())
	if err != nil {
		return DiagnosticWitnessProjection{}, err
	}
	actual, err := projectDiagnosticDatum(witness.Actual())
	if err != nil {
		return DiagnosticWitnessProjection{}, err
	}
	switch witness.(type) {
	case typedmemory.ExpectedActualWitness:
		return DiagnosticWitnessProjection{
			kind:     DiagnosticWitnessExpectedActual,
			expected: expected,
			actual:   actual,
		}, nil
	case typedmemory.MissingBasisWitness:
		return DiagnosticWitnessProjection{
			kind:     DiagnosticWitnessMissingBasis,
			expected: expected,
			actual:   actual,
		}, nil
	default:
		return DiagnosticWitnessProjection{}, fmt.Errorf(
			"unknown diagnostic witness %T",
			witness,
		)
	}
}

func projectDiagnosticDatum(
	datum typedmemory.DiagnosticDatum,
) (DiagnosticDatumProjection, error) {
	projection := DiagnosticDatumProjection{kind: datum.Kind()}
	switch datum.Kind() {
	case typedmemory.DiagnosticDatumText,
		typedmemory.DiagnosticDatumReference,
		typedmemory.DiagnosticDatumState,
		typedmemory.DiagnosticDatumUnknown:
		value, present := datum.Scalar()
		if !present {
			return DiagnosticDatumProjection{}, fmt.Errorf(
				"diagnostic datum %q has no scalar",
				datum.Kind(),
			)
		}
		projection.scalar = value
	case typedmemory.DiagnosticDatumCount:
		value, present := datum.Count()
		if !present {
			return DiagnosticDatumProjection{}, fmt.Errorf("diagnostic count is absent")
		}
		projection.count = value
		projection.hasCount = true
	case typedmemory.DiagnosticDatumSet:
		values, present := datum.SetValues()
		if !present {
			return DiagnosticDatumProjection{}, fmt.Errorf("diagnostic set is absent")
		}
		projection.setValues = values
	default:
		return DiagnosticDatumProjection{}, fmt.Errorf(
			"unknown diagnostic datum kind %q",
			datum.Kind(),
		)
	}
	return projection, nil
}

func projectDiagnosticBasis(
	basis typedmemory.DiagnosticGoverningBasis,
) (GoverningBasisProjection, error) {
	switch value := basis.(type) {
	case typedmemory.KnownDeclarationBasis:
		provenance, err := projectDeclarationProvenance(value.Provenance())
		if err != nil {
			return GoverningBasisProjection{}, err
		}
		return GoverningBasisProjection{
			kind:       value.Kind(),
			provenance: &provenance,
		}, nil
	case typedmemory.CoreValidatorBasis:
		return GoverningBasisProjection{
			kind: value.Kind(),
			rule: value.Rule().String(),
		}, nil
	case typedmemory.SnapshotRuleBasis:
		return GoverningBasisProjection{
			kind: value.Kind(),
			rule: value.Rule().String(),
		}, nil
	case typedmemory.MissingTypeEnvDeclarationBasis:
		subject, err := projectDiagnosticDatum(value.Subject())
		if err != nil {
			return GoverningBasisProjection{}, err
		}
		projection := GoverningBasisProjection{
			kind:       value.Kind(),
			typeEnvRef: value.TypeEnv().String(),
			subject:    &subject,
		}
		coverage, present := value.Coverage()
		if present {
			coverageProjection := projectCoverage(coverage)
			projection.coverage = &coverageProjection
		}
		return projection, nil
	case typedmemory.MissingRuntimeBasis:
		required, err := projectDiagnosticDatum(value.Required())
		if err != nil {
			return GoverningBasisProjection{}, err
		}
		return GoverningBasisProjection{
			kind:               value.Kind(),
			missingRuntimeKind: value.MissingKind(),
			required:           &required,
		}, nil
	default:
		return GoverningBasisProjection{}, fmt.Errorf(
			"unknown diagnostic governing basis %T",
			basis,
		)
	}
}

func projectRepairCandidates(
	values []typedmemory.RepairCandidate,
) ([]RepairProjection, error) {
	repairs := make([]RepairProjection, 0, len(values))
	for _, value := range values {
		target, err := projectDiagnosticDatum(value.Target())
		if err != nil {
			return nil, err
		}
		repairs = append(repairs, RepairProjection{
			kind:        value.Kind(),
			pointer:     value.Pointer().String(),
			target:      target,
			humanChoice: value.HumanChoiceRequirement(),
		})
	}
	return repairs, nil
}

func projectDeclarationProvenance(
	provenance typedmemory.DeclarationProvenance,
) (DeclarationProvenanceProjection, error) {
	switch value := provenance.(type) {
	case typedmemory.FPFSourceProvenance:
		return DeclarationProvenanceProjection{
			kind:           DeclarationProvenanceFPFSource,
			reference:      value.Reference().String(),
			compilerRuleID: value.CompilerRuleID().String(),
			sources:        []SourceLocationProjection{projectSourceLocation(value.Location())},
		}, nil
	case typedmemory.CompilerDerivedProvenance:
		inputs := value.Inputs()
		sources := make([]SourceLocationProjection, 0, len(inputs))
		for _, input := range inputs {
			sources = append(sources, projectSourceLocation(input))
		}
		return DeclarationProvenanceProjection{
			kind:           DeclarationProvenanceCompilerDerived,
			reference:      value.Reference().String(),
			compilerRuleID: value.CompilerRuleID().String(),
			sources:        sources,
		}, nil
	case typedmemory.ProjectSourceProvenance:
		manifest := value.ManifestBasis()
		lineRange := value.LineRange()
		return DeclarationProvenanceProjection{
			kind:              DeclarationProvenanceProjectSource,
			reference:         value.Reference().String(),
			compilerRuleID:    value.CompilerRuleID().String(),
			carrier:           value.Carrier().String(),
			edition:           value.Edition().String(),
			contentHash:       value.ContentHash().String(),
			startLine:         lineRange.Start(),
			endLine:           lineRange.End(),
			boundedContext:    value.BoundedContext().String(),
			baseTypeEnv:       value.BaseTypeEnv().String(),
			signatureBlockRow: value.SignatureBlockRow().String(),
			manifestRef:       manifest.Manifest().String(),
			manifestDirection: manifest.Direction().String(),
			manifestSymbol:    manifest.Symbol().String(),
		}, nil
	default:
		return DeclarationProvenanceProjection{}, fmt.Errorf(
			"unknown declaration provenance %T",
			provenance,
		)
	}
}

func projectSourceLocation(
	location typedmemory.SourceLocation,
) SourceLocationProjection {
	patternID := ""
	pattern, present := location.PatternID()
	if present {
		patternID = pattern.String()
	}
	lineRange := location.LineRange()
	return SourceLocationProjection{
		unitID:      location.UnitID().String(),
		revision:    location.Revision().String(),
		contentHash: location.ContentHash().String(),
		startLine:   lineRange.Start(),
		endLine:     lineRange.End(),
		patternID:   patternID,
	}
}

func projectCoverage(coverage typedmemory.CoverageEntry) CoverageProjection {
	return CoverageProjection{
		subject:   coverage.Subject().String(),
		posture:   coverage.Posture(),
		source:    projectSourceLocation(coverage.Source()),
		rationale: coverage.Rationale(),
	}
}

func invalidRequestOutcome(contractVersion string) Outcome {
	projection := BasisProjection{}
	diagnostic := newInvalidDiagnostic(
		DiagnosticMalformedValidationRequest,
		"a decoded haft.memory.validate.v1 request is required",
		"$",
		"decoded-request",
		"missing-or-invalid-request",
		validationBindRule,
	)
	return newInvalidOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func unavailableResolutionOutcome(
	contractVersion string,
	selector typedmemorywire.BasisSelector,
) Outcome {
	projection := requestedProjection(selector, BasisResolutionProjectMissing)
	diagnostic := newUnderDiagnostic(
		DiagnosticBasisResolutionUnavailable,
		"the server-owned basis resolver returned no usable resolution",
		"$.basis",
		"server-owned-basis-resolution",
		"unavailable",
		"repair-server-basis-resolver",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func resolutionMismatchOutcome(
	contractVersion string,
	selector typedmemorywire.BasisSelector,
	resolution BasisResolution,
) Outcome {
	actual := "unavailable"
	resolutionKind := BasisResolutionProjectMissing
	if basisResolutionPresent(resolution) {
		resolutionKind = resolution.Kind()
		actual = string(resolutionKind)
	}
	projection := requestedProjection(selector, resolutionKind)
	diagnostic := newUnderDiagnostic(
		DiagnosticBasisResolutionMismatch,
		"the server-owned resolution does not match the requested basis selector",
		"$.basis.kind",
		string(selector.Kind()),
		actual,
		"repair-server-basis-resolver",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func projectUnavailableOutcome(
	contractVersion string,
	selector typedmemorywire.BasisSelector,
) Outcome {
	projection := requestedProjection(selector, BasisResolutionProjectMissing)
	diagnostics := []DiagnosticProjection{
		newUnderDiagnostic(
			DiagnosticProjectTypeEnvUnavailable,
			"the project has no server-observed active typed-memory TypeEnv",
			"$.basis.type_env",
			"project-active-typeenv",
			"unavailable-pre-p8",
			"activate-an-exact-project-typeenv-through-the-sealed-lifecycle",
		),
		newUnderDiagnostic(
			DiagnosticProjectSnapshotUnavailable,
			"the project has no immutable typed-memory snapshot at an observed graph revision",
			"$.basis.graph_revision",
			"immutable-project-memory-snapshot",
			"unavailable-pre-p8",
			"initialize-typed-memory-storage-and-load-an-observed-snapshot",
		),
	}
	return newUnderdeterminedOutcome(contractVersion, projection, diagnostics)
}

func exactMismatchOutcome(
	contractVersion string,
	selector typedmemorywire.ExactProjectSelector,
	observedTypeEnv typedmemory.TypeEnvRef,
	observedGraphRevision typedmemory.GraphRevision,
) Outcome {
	projection := requestedProjection(selector, BasisResolutionExactMismatch)
	projection.typeEnvRef = observedTypeEnv.String()
	projection.graphRevision = observedGraphRevision.Value()
	projection.hasGraphRevision = true
	expected := fmt.Sprintf(
		"type_env_digest=%s graph_revision=%d",
		selector.RequestedTypeEnvDigest().String(),
		selector.RequestedGraphRevision().Value(),
	)
	actual := fmt.Sprintf(
		"type_env_ref=%s graph_revision=%d",
		observedTypeEnv.String(),
		observedGraphRevision.Value(),
	)
	diagnostic := newUnderDiagnostic(
		DiagnosticExactBasisMismatch,
		"the observed project basis does not satisfy the exact requested precondition; no fallback was used",
		"$.basis",
		expected,
		actual,
		"inspect-project-current-basis-and-submit-an-explicit-new-request",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func bindFailureOutcome(
	contractVersion string,
	projection BasisProjection,
	cause error,
) Outcome {
	diagnostic := newInvalidDiagnostic(
		DiagnosticChangeSetBindFailed,
		"the decoded candidate cannot bind to the server-resolved TypeEnv: "+cause.Error(),
		"$.change_set",
		"closed-memory-change-set-bound-to-resolved-typeenv",
		"binding-failed",
		validationBindRule,
	)
	return newInvalidOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func digestUnavailableOutcome(
	contractVersion string,
	projection BasisProjection,
	cause error,
) Outcome {
	diagnostic := newUnderDiagnostic(
		DiagnosticDigestUnavailable,
		"the validator could not produce the normalized change-set digest: "+cause.Error(),
		"$.change_set",
		"canonical-memory-change-set-digest",
		"unavailable",
		"inspect-and-repair-canonical-memory-change-set-encoding",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func projectionUnavailableOutcome(
	contractVersion string,
	projection BasisProjection,
	cause error,
) Outcome {
	diagnostic := newUnderDiagnostic(
		DiagnosticProjectionUnavailable,
		"the validator produced a diagnostic outside the public closed projection: "+cause.Error(),
		"$.diagnostics",
		"closed-diagnostic-witness-basis-and-repair-projection",
		"unavailable",
		"update-the-public-diagnostic-projection-before-retrying",
	)
	return newUnderdeterminedOutcome(
		contractVersion,
		projection,
		[]DiagnosticProjection{diagnostic},
	)
}

func requestedProjection(
	selector typedmemorywire.BasisSelector,
	resolution BasisResolutionKind,
) BasisProjection {
	projection := BasisProjection{
		requestedKind:  selector.Kind(),
		resolutionKind: resolution,
	}
	exact, isExact := selector.(typedmemorywire.ExactProjectSelector)
	if !isExact {
		return projection
	}
	projection.requestedTypeEnvDigest = exact.RequestedTypeEnvDigest().String()
	projection.requestedGraphRevision = exact.RequestedGraphRevision().Value()
	projection.hasExactRequest = true
	return projection
}

func bundledProjection(
	selector typedmemorywire.BundledCandidateOpenWorldSelector,
	basis *BundledCandidateOpenWorldBasis,
) BasisProjection {
	projection := requestedProjection(selector, BasisResolutionBundledCandidate)
	projection.typeEnvRef = basis.environment.Ref().String()
	return projection
}

func resolvedProjectProjection(
	selector typedmemorywire.BasisSelector,
	basis *ResolvedProjectBasis,
) BasisProjection {
	projection := requestedProjection(selector, BasisResolutionProject)
	projection.typeEnvRef = basis.environment.Ref().String()
	projection.graphRevision = basis.snapshot.GraphRevision().Value()
	projection.hasGraphRevision = true
	return projection
}

func newUnderDiagnostic(
	code string,
	message string,
	path string,
	expected string,
	actual string,
	repair string,
) DiagnosticProjection {
	expectedDatum := stateDatumProjection(expected)
	actualDatum := stateDatumProjection(actual)
	witness := DiagnosticWitnessProjection{
		kind:     DiagnosticWitnessMissingBasis,
		expected: expectedDatum,
		actual:   actualDatum,
	}
	required := copyDatumProjection(expectedDatum)
	basis := GoverningBasisProjection{
		kind:               typedmemory.DiagnosticBasisMissingRuntime,
		missingRuntimeKind: missingRuntimeKind(code),
		required:           &required,
	}
	repairKind := repairKind(code)
	repairProjection := RepairProjection{
		kind:        repairKind,
		pointer:     repair,
		target:      copyDatumProjection(expectedDatum),
		humanChoice: typedmemory.HumanChoiceNotClaimed,
	}
	return DiagnosticProjection{
		posture:  typedmemory.DiagnosticUnderdetermined,
		code:     code,
		message:  message,
		path:     path,
		pathKind: serviceDiagnosticPathKind(path),
		witness:  witness,
		basis:    basis,
		repairs:  []RepairProjection{repairProjection},
	}
}

func newInvalidDiagnostic(
	code string,
	message string,
	path string,
	expected string,
	actual string,
	rule string,
) DiagnosticProjection {
	expectedDatum := stateDatumProjection(expected)
	actualDatum := stateDatumProjection(actual)
	witness := DiagnosticWitnessProjection{
		kind:     DiagnosticWitnessExpectedActual,
		expected: expectedDatum,
		actual:   actualDatum,
	}
	basis := GoverningBasisProjection{
		kind: typedmemory.DiagnosticBasisCoreValidator,
		rule: rule,
	}
	repairProjection := RepairProjection{
		kind:        typedmemory.RepairChangeInput,
		pointer:     "repair-the-memory-change-set-at-" + path,
		target:      copyDatumProjection(expectedDatum),
		humanChoice: typedmemory.HumanChoiceNotClaimed,
	}
	return DiagnosticProjection{
		posture:  typedmemory.DiagnosticInvalid,
		code:     code,
		message:  message,
		path:     path,
		pathKind: serviceDiagnosticPathKind(path),
		witness:  witness,
		basis:    basis,
		repairs:  []RepairProjection{repairProjection},
	}
}

func stateDatumProjection(value string) DiagnosticDatumProjection {
	return DiagnosticDatumProjection{
		kind:   typedmemory.DiagnosticDatumState,
		scalar: value,
	}
}

func missingRuntimeKind(code string) typedmemory.MissingRuntimeBasisKind {
	switch code {
	case DiagnosticProjectTypeEnvUnavailable:
		return typedmemory.MissingRuntimeActiveTypeEnv
	case DiagnosticProjectSnapshotUnavailable,
		DiagnosticCandidateNotProject,
		DiagnosticExactBasisMismatch:
		return typedmemory.MissingRuntimeSnapshot
	case DiagnosticDigestUnavailable:
		return typedmemory.MissingRuntimeCoverage
	default:
		return typedmemory.MissingRuntimeResolution
	}
}

func repairKind(code string) typedmemory.RepairKind {
	switch code {
	case DiagnosticProjectTypeEnvUnavailable:
		return typedmemory.RepairExtendTypeEnv
	case DiagnosticProjectSnapshotUnavailable,
		DiagnosticCandidateNotProject:
		return typedmemory.RepairRefreshSnapshot
	case DiagnosticMalformedValidationRequest,
		DiagnosticChangeSetBindFailed,
		DiagnosticDigestUnavailable:
		return typedmemory.RepairChangeInput
	default:
		return typedmemory.RepairInspectBasis
	}
}

func newValidResponse(
	contractVersion string,
	basis BasisProjection,
	digest typedmemory.SHA256Digest,
) Response {
	return validResponse{
		responseBase: responseBase{
			contractVersion: contractVersion,
			verdict:         typedmemory.ValidationValid,
			basis:           basis,
		},
		digest: digest,
	}
}

func newInvalidResponse(
	contractVersion string,
	basis BasisProjection,
	diagnostics []DiagnosticProjection,
) Response {
	return invalidResponse{responseBase: responseBase{
		contractVersion: contractVersion,
		verdict:         typedmemory.ValidationInvalid,
		basis:           basis,
		diagnostics:     copyDiagnosticProjections(diagnostics),
	}}
}

func newUnderdeterminedResponse(
	contractVersion string,
	basis BasisProjection,
	diagnostics []DiagnosticProjection,
) Response {
	return underdeterminedResponse{responseBase: responseBase{
		contractVersion: contractVersion,
		verdict:         typedmemory.ValidationUnderdetermined,
		basis:           basis,
		diagnostics:     copyDiagnosticProjections(diagnostics),
	}}
}

func copyDiagnosticProjections(values []DiagnosticProjection) []DiagnosticProjection {
	result := make([]DiagnosticProjection, 0, len(values))
	for _, value := range values {
		copyValue := value
		copyValue.witness = copyWitnessProjection(value.witness)
		copyValue.basis = copyGoverningBasisProjection(value.basis)
		copyValue.repairs = copyRepairProjections(value.repairs)
		result = append(result, copyValue)
	}
	return result
}

func basisResolverPresent(resolver BasisResolver) bool {
	if resolver == nil {
		return false
	}
	value := reflect.ValueOf(resolver)
	if value.Kind() != reflect.Pointer {
		return true
	}
	return !value.IsNil()
}

func validationRequestPresent(request validationRequest) bool {
	if request == nil {
		return false
	}
	value := reflect.ValueOf(request)
	if value.Kind() != reflect.Pointer {
		return true
	}
	return !value.IsNil()
}

func validationContractVersion(request validationRequest) string {
	if !validationRequestPresent(request) {
		return typedmemorywire.ContractVersionV1
	}
	if request.ContractVersion() == typedmemorywire.ContractVersionV2 {
		return typedmemorywire.ContractVersionV2
	}
	return typedmemorywire.ContractVersionV1
}
