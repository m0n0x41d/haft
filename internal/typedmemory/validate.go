package typedmemory

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const coreValidationRule = "typedmemory.validation-core.v1"

// ValidateMemoryChangeSet is the pure semantic gate. Snapshot loading, active
// TypeEnv selection, transactions, authority, and persistence stay outside.
func ValidateMemoryChangeSet(
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	changeSet MemoryChangeSet,
) ValidationVerdict {
	accumulator := validationAccumulator{}
	basisRevision := NewGraphRevision(0)
	if memorySnapshotPresent(snapshot) {
		basisRevision = snapshot.GraphRevision()
	}
	if !environment.Ref().valid() {
		required := diagnosticState("exact active TypeEnv")
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeActiveTypeEnv, required)
		accumulator.addUnderdeterminedWithWitness(
			DiagnosticTypeRuleUnavailable,
			"an exact active TypeEnv is required",
			"type_env",
			"inspect-project-active-typeenv",
			required,
			diagnosticState("absent"),
			basis,
		)
		return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
	}
	if !memorySnapshotPresent(snapshot) {
		required := diagnosticState("immutable project-memory snapshot")
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeSnapshot, required)
		accumulator.addUnderdeterminedWithWitness(
			DiagnosticTypeRuleUnavailable,
			"an immutable project-memory snapshot is required",
			"snapshot",
			"load-snapshot-at-the-requested-graph-revision",
			required,
			diagnosticState("absent"),
			basis,
		)
		return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
	}
	if snapshot.TypeEnvRef() != environment.Ref() {
		required := diagnosticReference(environment.Ref().String())
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeSnapshot, required)
		accumulator.addUnderdeterminedWithWitness(
			DiagnosticTypeRuleUnavailable,
			"snapshot was not loaded against the exact active TypeEnv",
			"snapshot.type_env_ref",
			"reload-the-snapshot-for-the-exact-active-typeenv",
			required,
			diagnosticReference(snapshot.TypeEnvRef().String()),
			basis,
		)
		return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
	}
	if !changeSet.valid() {
		accumulator.addInvalidWithWitness(
			DiagnosticMalformedValue,
			"MemoryChangeSet is empty or contains an invalid closed variant",
			"changes",
			coreValidatorGoverningBasis(),
			diagnosticState("non-empty closed MemoryChangeSet"),
			diagnosticState("empty or invalid closed variant"),
		)
		return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
	}

	locals := localDeclarations(changeSet)
	for index, change := range changeSet.changes {
		path := fmt.Sprintf("changes[%d]", index)
		changeOrdinal, exact := exactUint64FromNonNegativeInt(index)
		if !exact {
			accumulator.addInvalidWithWitness(
				DiagnosticMalformedValue,
				"MemoryChangeSet change ordinal exceeds the canonical uint64 range",
				path,
				coreValidatorGoverningBasis(),
				diagnosticState("uint64 change ordinal"),
				diagnosticState(strconv.Itoa(index)),
			)
			return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
		}
		validateMemoryChange(
			&accumulator,
			environment,
			registry,
			snapshot,
			changeSet,
			locals,
			change,
			changeOrdinal,
			path,
		)
	}
	return accumulator.verdict(changeSet, environment.Ref(), basisRevision)
}

func exactUint64FromNonNegativeInt(value int) (uint64, bool) {
	if value < 0 {
		return 0, false
	}
	canonical := strconv.Itoa(value)
	converted, err := strconv.ParseUint(canonical, 10, 64)
	return converted, err == nil
}

type validationAccumulator struct {
	diagnostics                 []Diagnostic
	validated                   []ValidatedMemoryChange
	observations                []AdmissionSnapshotObservation
	membershipReferenceUses     []ReferenceFillerAdmissionUse
	classificationReferenceUses []ClassificationReferenceFillerAdmissionUse
}

func (accumulator *validationAccumulator) addInvalidWithWitness(
	code DiagnosticCode,
	message string,
	path string,
	basis DiagnosticGoverningBasis,
	expected DiagnosticDatum,
	actual DiagnosticDatum,
) {
	diagnosticPath := DiagnosticPath{value: path}
	witness, _ := NewExpectedActualWitness(expected, actual)
	repairs := []RepairCandidate{defaultInvalidRepair(code, diagnosticPath, expected)}
	diagnostic, _ := NewInvalidDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		repairs,
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func (accumulator *validationAccumulator) addMissingResolution(
	code DiagnosticCode,
	message string,
	path string,
	repair string,
) {
	diagnosticPath := DiagnosticPath{value: path}
	required := genericRequiredDatum(code)
	witness, _ := NewMissingBasisWitness(
		required,
		"validation site could not observe an actual value without the missing basis",
	)
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
	repairPointer := RepairPointer{value: repair}
	repairs := []RepairCandidate{
		defaultMissingBasisRepair(code, basis, repairPointer, required),
	}
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		repairPointer,
		repairs,
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func (accumulator *validationAccumulator) addUnderdeterminedWithRequired(
	code DiagnosticCode,
	message string,
	path string,
	repair string,
	required DiagnosticDatum,
	basis DiagnosticGoverningBasis,
) {
	accumulator.addUnderdeterminedWithWitness(
		code,
		message,
		path,
		repair,
		required,
		diagnosticUnknown("the named validation basis is unavailable"),
		basis,
	)
}

func (accumulator *validationAccumulator) addUnderdeterminedWithWitness(
	code DiagnosticCode,
	message string,
	path string,
	repair string,
	required DiagnosticDatum,
	actual DiagnosticDatum,
	basis DiagnosticGoverningBasis,
) {
	diagnosticPath := DiagnosticPath{value: path}
	witness, _ := NewMissingBasisWitnessWithActual(required, actual)
	repairPointer := RepairPointer{value: repair}
	repairs := []RepairCandidate{
		defaultMissingBasisRepair(code, basis, repairPointer, required),
	}
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		code,
		message,
		diagnosticPath,
		witness,
		basis,
		repairPointer,
		repairs,
	)
	accumulator.diagnostics = append(accumulator.diagnostics, diagnostic)
}

func coreValidatorGoverningBasis() DiagnosticGoverningBasis {
	basis, _ := NewCoreValidatorBasis(RuleRef{value: coreValidationRule})
	return basis
}

func snapshotRuleGoverningBasis(rule RuleRef) DiagnosticGoverningBasis {
	basis, _ := NewSnapshotRuleBasis(rule)
	return basis
}

func declarationGoverningBasis(
	provenance DeclarationProvenance,
) DiagnosticGoverningBasis {
	if !validDeclarationProvenance(provenance) {
		return coreValidatorGoverningBasis()
	}
	basis, _ := NewKnownDeclarationBasis(provenance)
	return basis
}

func (accumulator validationAccumulator) verdict(
	candidate MemoryChangeSet,
	basisTypeEnv TypeEnvRef,
	basisRevision GraphRevision,
) ValidationVerdict {
	if hasDiagnosticPosture(accumulator.diagnostics, DiagnosticInvalid) {
		verdict, err := newInvalidVerdict(accumulator.diagnostics)
		if err == nil {
			return verdict
		}
		return validationInvariantFailure(err)
	}
	if len(accumulator.diagnostics) > 0 {
		verdict, err := newUnderdeterminedVerdict(accumulator.diagnostics)
		if err == nil {
			return verdict
		}
		return validationInvariantFailure(err)
	}
	basis, err := accumulator.admissionBasis(basisTypeEnv, basisRevision)
	if err != nil {
		return validationInvariantFailure(err)
	}
	verdict, err := newValidVerdict(
		candidate,
		newValidatedMemoryChangeSet(accumulator.validated),
		basis,
	)
	if err == nil {
		return verdict
	}
	return validationInvariantFailure(err)
}

func (accumulator validationAccumulator) admissionBasis(
	typeEnv TypeEnvRef,
	revision GraphRevision,
) (AdmissionBasis, error) {
	if len(accumulator.membershipReferenceUses) > 0 &&
		len(accumulator.classificationReferenceUses) > 0 {
		return nil, fmt.Errorf(
			"one admission cannot mix historical MemberOf and current C.3.2 classification evidence",
		)
	}
	if len(accumulator.classificationReferenceUses) > 0 {
		return NewContextSliceClassificationBasis(
			ContextSliceClassificationBasisInput{
				TypeEnv:       typeEnv,
				GraphRevision: revision,
				Observations:  accumulator.observations,
				ClassificationReferenceFillerAdmissionUses: accumulator.classificationReferenceUses,
			},
		)
	}
	if len(accumulator.membershipReferenceUses) == 0 {
		return NewSnapshotOnlyBasis(SnapshotOnlyBasisInput{
			TypeEnv:       typeEnv,
			GraphRevision: revision,
			Observations:  accumulator.observations,
		})
	}
	return NewContextSliceMembershipBasis(ContextSliceMembershipBasisInput{
		TypeEnv:                      typeEnv,
		GraphRevision:                revision,
		Observations:                 accumulator.observations,
		ReferenceFillerAdmissionUses: accumulator.membershipReferenceUses,
	})
}

func (accumulator *validationAccumulator) captureObservation(
	observation AdmissionSnapshotObservation,
	err error,
	path string,
) bool {
	if err == nil {
		accumulator.observations = append(accumulator.observations, observation)
		return true
	}
	accumulator.addMissingResolution(
		DiagnosticTypeRuleUnavailable,
		"validator could not seal an exact positive snapshot observation: "+err.Error(),
		path,
		"inspect-and-repair-typed-memory-validator",
	)
	return false
}

func (accumulator *validationAccumulator) captureReferenceUse(
	use ReferenceFillerAdmissionUse,
	err error,
	path string,
) bool {
	if err == nil {
		accumulator.membershipReferenceUses = append(
			accumulator.membershipReferenceUses,
			use,
		)
		return true
	}
	accumulator.addMissingResolution(
		DiagnosticTypeRuleUnavailable,
		"validator could not seal exact reference-filler admission evidence: "+err.Error(),
		path,
		"inspect-and-repair-typed-memory-validator",
	)
	return false
}

func (accumulator *validationAccumulator) captureClassificationReferenceUse(
	use ClassificationReferenceFillerAdmissionUse,
	err error,
	path string,
) bool {
	if err == nil {
		accumulator.classificationReferenceUses = append(
			accumulator.classificationReferenceUses,
			use,
		)
		return true
	}
	accumulator.addMissingResolution(
		DiagnosticTypeRuleUnavailable,
		"validator could not seal exact current classification admission evidence: "+err.Error(),
		path,
		"inspect-and-repair-typed-memory-validator",
	)
	return false
}

func validationInvariantFailure(cause error) ValidationVerdict {
	path, _ := NewDiagnosticPath("validation.core")
	repair, _ := NewRepairPointer("inspect-and-repair-typed-memory-validator")
	required := diagnosticState("closed validation verdict")
	witness, _ := NewMissingBasisWitnessWithActual(required, diagnosticText(cause.Error()))
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeValidator, required)
	repairCandidate, _ := NewRepairCandidate(
		RepairInspectBasis,
		repair,
		required,
		HumanChoiceNotClaimed,
	)
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		DiagnosticTypeRuleUnavailable,
		"typed-memory validator could not construct a closed verdict: "+cause.Error(),
		path,
		witness,
		basis,
		repair,
		[]RepairCandidate{repairCandidate},
	)
	verdict, _ := newUnderdeterminedVerdict([]Diagnostic{diagnostic})
	return verdict
}

func validateMemoryChange(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	candidate MemoryChangeSet,
	locals map[string]localDeclaration,
	change MemoryChange,
	changeOrdinal uint64,
	path string,
) {
	switch value := change.(type) {
	case DeclareEntity:
		validateDeclareEntity(accumulator, environment, snapshot, value, changeOrdinal, path)
	case ApplyIdentityChange:
		validateIdentityChange(
			accumulator,
			environment,
			snapshot,
			locals,
			value.change,
			changeOrdinal,
			path,
		)
	case InstantiateRelation:
		prefix, err := ComputeOrderedCandidatePrefix(candidate, changeOrdinal)
		if err != nil {
			accumulator.addMissingResolution(
				DiagnosticTypeRuleUnavailable,
				"validator could not seal the ordered candidate prefix: "+err.Error(),
				path,
				"inspect-and-repair-typed-memory-validator",
			)
			return
		}
		validateRelationInstantiation(
			accumulator,
			environment,
			registry,
			snapshot,
			locals,
			value.relation,
			changeOrdinal,
			prefix,
			path,
		)
	case AssertRelation:
		prefix, err := ComputeOrderedCandidatePrefix(candidate, changeOrdinal)
		if err != nil {
			accumulator.addMissingResolution(
				DiagnosticTypeRuleUnavailable,
				"validator could not seal the ordered candidate prefix: "+err.Error(),
				path,
				"inspect-and-repair-typed-memory-validator",
			)
			return
		}
		validateRelationalAssertionCandidate(
			accumulator,
			environment,
			registry,
			snapshot,
			locals,
			value.assertion,
			changeOrdinal,
			prefix,
			path,
		)
	case RetractAssertion:
		validateRetraction(accumulator, snapshot, value, changeOrdinal, path)
	default:
		accumulator.addInvalidWithWitness(
			DiagnosticMalformedValue,
			fmt.Sprintf("unknown MemoryChange variant %T", change),
			path,
			coreValidatorGoverningBasis(),
			diagnosticSet([]string{
				"DeclareEntity",
				"ApplyIdentityChange",
				"InstantiateRelation",
				"AssertRelation",
				"RetractAssertion",
			}),
			diagnosticText(fmt.Sprintf("%T", change)),
		)
	}
}

func validateDeclareEntity(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	change DeclareEntity,
	changeOrdinal uint64,
	path string,
) {
	if !contextActive(accumulator, environment, change.context, path+".bounded_context") {
		return
	}

	switch resolution := snapshot.ResolveEntity(change.entity, change.context).(type) {
	case ExactEntityResolution:
		if resolution.Entity() != change.entity || resolution.Context() != change.context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"entity lookup returned an uncorrelated exact resolution",
				path+".entity_id",
				"rebuild-or-inspect-entity-identity-index",
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticEntityAlreadyExists,
			fmt.Sprintf("entity %s already resolves exactly", change.entity.String()),
			path+".entity_id",
			coreValidatorGoverningBasis(),
			diagnosticState("entity absent in bounded context"),
			diagnosticReference(change.entity.String()),
		)
	case CandidateEntityResolution:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			fmt.Sprintf("entity %s has unresolved identity candidates", change.entity.String()),
			path+".entity_id",
			"inspect-entity-candidates-before-declaration",
		)
	case AbsentEntityResolution:
		if resolution.Entity() != change.entity ||
			resolution.Context() != change.context ||
			!resolution.Basis().valid() {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"entity lookup returned an uncorrelated absence resolution",
				path+".entity_id",
				"rebuild-or-inspect-entity-identity-index",
			)
			return
		}
		observation, err := NewEntityAbsentObservation(changeOrdinal, resolution)
		if !accumulator.captureObservation(observation, err, path+".entity_id") {
			return
		}
		declaration := newAdmittedEntityDeclaration(change)
		accumulator.validated = append(
			accumulator.validated,
			ValidatedDeclareEntity{declaration: declaration},
		)
	case UnknownEntityResolution:
		validateUnknownEntityResolution(
			accumulator,
			resolution,
			change.entity,
			change.context,
			path+".entity_id",
			"entity declaration requires an exact absence basis",
		)
	case UnsettledEntityResolution:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			fmt.Sprintf("entity %s has unresolved identity basis", change.entity.String()),
			path+".entity_id",
			"resolve-entity-identity-before-declaration",
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"entity identity lookup returned no recognized basis",
			path+".entity_id",
			"rebuild-or-inspect-entity-identity-index",
		)
	}
}

func validateIdentityChange(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	locals map[string]localDeclaration,
	change IdentityChange,
	changeOrdinal uint64,
	path string,
) {
	start := len(accumulator.diagnostics)
	switch value := change.(type) {
	case AdmitAlias:
		if !contextActive(accumulator, environment, value.context, path+".bounded_context") {
			return
		}
		validateExactEntityOrPriorDeclaration(
			accumulator,
			snapshot,
			locals,
			value.entity,
			value.context,
			changeOrdinal,
			path+".entity",
		)
		validateAliasAdmission(accumulator, snapshot, value, changeOrdinal, path)
	case SupersedeAlias:
		if !contextActive(accumulator, environment, value.context, path+".bounded_context") {
			return
		}
		validateExactEntity(accumulator, snapshot, value.entity, value.context, changeOrdinal, path+".entity")
		validateAliasSupersession(accumulator, snapshot, value, changeOrdinal, path)
		validateAliasAvailability(
			accumulator,
			snapshot,
			value.entity,
			value.replacement,
			value.context,
			changeOrdinal,
			path+".replacement",
		)
	case MergeEntities:
		addManualIdentityReconciliationRequired(
			accumulator,
			value.basis,
			ReconciliationMergeEntities,
			path+".reconciliation_basis_ref",
		)
	case SplitEntity:
		addManualIdentityReconciliationRequired(
			accumulator,
			value.basis,
			ReconciliationSplitEntity,
			path+".reconciliation_basis_ref",
		)
	default:
		accumulator.addInvalidWithWitness(
			DiagnosticMalformedValue,
			fmt.Sprintf("unknown IdentityChange variant %T", change),
			path,
			coreValidatorGoverningBasis(),
			diagnosticSet([]string{
				"AdmitAlias",
				"SupersedeAlias",
				"MergeEntities",
				"SplitEntity",
			}),
			diagnosticText(fmt.Sprintf("%T", change)),
		)
	}
	if len(accumulator.diagnostics) == start {
		accumulator.validated = append(accumulator.validated, ValidatedIdentityChange{change: change})
	}
}

func validateExactEntityOrPriorDeclaration(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	locals map[string]localDeclaration,
	entity EntityID,
	context BoundedContextRef,
	changeOrdinal uint64,
	path string,
) {
	if priorDeclarationNamesEntity(
		locals,
		entity,
		context,
		changeOrdinal,
	) {
		return
	}
	validateExactEntity(
		accumulator,
		snapshot,
		entity,
		context,
		changeOrdinal,
		path,
	)
}

func priorDeclarationNamesEntity(
	locals map[string]localDeclaration,
	entity EntityID,
	context BoundedContextRef,
	changeOrdinal uint64,
) bool {
	for _, declaration := range locals {
		if declaration.ordinal >= changeOrdinal {
			continue
		}
		if declaration.change.Entity() == entity &&
			declaration.change.Context() == context {
			return true
		}
	}
	return false
}

func addManualIdentityReconciliationRequired(
	accumulator *validationAccumulator,
	basisRef ReconciliationBasisRef,
	operation IdentityReconciliationOperation,
	path string,
) {
	required := diagnosticSet([]string{
		"manual_identity_reconciliation_required",
		"operation=" + string(operation),
		"basis=" + basisRef.String(),
	})
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
	accumulator.addUnderdeterminedWithRequired(
		DiagnosticReconciliationBasisUnresolved,
		"generic memory validation cannot authorize destructive identity reconciliation",
		path,
		"manual_identity_reconciliation_required",
		required,
		basis,
	)
}

func exactEntitySequence(left []EntityID, right []EntityID) bool {
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

func validateAliasAdmission(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	change AdmitAlias,
	changeOrdinal uint64,
	path string,
) {
	validateAliasAvailability(
		accumulator,
		snapshot,
		change.entity,
		change.alias,
		change.context,
		changeOrdinal,
		path+".alias",
	)
}

func validateAliasAvailability(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	entity EntityID,
	alias EntityAlias,
	context BoundedContextRef,
	changeOrdinal uint64,
	path string,
) {
	switch resolution := snapshot.ResolveAlias(alias, context).(type) {
	case BoundAliasResolution:
		if resolution.Alias() != alias || resolution.Context() != context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"alias lookup returned an uncorrelated bound resolution",
				path,
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		if resolution.Entity() == entity {
			accumulator.addInvalidWithWitness(
				DiagnosticAliasAlreadyBound,
				fmt.Sprintf("alias is already bound to entity %s", entity.String()),
				path,
				coreValidatorGoverningBasis(),
				diagnosticState("alias unbound"),
				diagnosticReference(entity.String()),
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticAliasAmbiguous,
			fmt.Sprintf("alias already resolves to entity %s", resolution.Entity().String()),
			path,
			coreValidatorGoverningBasis(),
			diagnosticReference(entity.String()),
			diagnosticReference(resolution.Entity().String()),
		)
	case CandidateAliasResolution:
		if resolution.Alias() != alias || resolution.Context() != context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"alias lookup returned candidates for another alias/context",
				path,
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"alias has unresolved entity candidates",
			path,
			"inspect-alias-candidates-before-admission",
		)
	case UnboundAliasResolution:
		if resolution.Alias() != alias ||
			resolution.Context() != context ||
			!resolution.Basis().valid() {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"alias lookup returned an uncorrelated availability resolution",
				path,
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		observation, err := NewAliasUnboundObservation(changeOrdinal, resolution)
		accumulator.captureObservation(observation, err, path)
		return
	case UnsettledAliasResolution:
		if resolution.Alias() != alias || resolution.Context() != context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"alias lookup returned unsettled basis for another alias/context",
				path,
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"alias availability remains unsettled",
			path,
			"resolve-alias-availability",
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"alias lookup returned no recognized basis",
			path,
			"inspect-alias-index",
		)
	}
}

func validateAliasSupersession(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	change SupersedeAlias,
	changeOrdinal uint64,
	path string,
) {
	switch resolution := snapshot.ResolveAlias(change.oldAlias, change.context).(type) {
	case BoundAliasResolution:
		if resolution.Alias() != change.oldAlias || resolution.Context() != change.context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"superseded alias lookup returned an uncorrelated bound resolution",
				path+".old_alias",
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		if resolution.Entity() != change.entity {
			accumulator.addInvalidWithWitness(
				DiagnosticAliasAmbiguous,
				"superseded alias resolves to a different entity",
				path+".old_alias",
				coreValidatorGoverningBasis(),
				diagnosticReference(change.entity.String()),
				diagnosticReference(resolution.Entity().String()),
			)
			return
		}
		observation, err := NewAliasBoundObservation(changeOrdinal, resolution)
		accumulator.captureObservation(observation, err, path+".old_alias")
	case CandidateAliasResolution:
		if resolution.Alias() != change.oldAlias || resolution.Context() != change.context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"superseded alias lookup returned candidates for another alias/context",
				path+".old_alias",
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"superseded alias is ambiguous",
			path+".old_alias",
			"inspect-alias-candidates-before-supersession",
		)
	case UnboundAliasResolution:
		if resolution.Alias() != change.oldAlias || resolution.Context() != change.context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"superseded alias lookup returned unbound evidence for another alias/context",
				path+".old_alias",
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticAliasNotBound,
			"superseded alias is known to be unbound",
			path+".old_alias",
			coreValidatorGoverningBasis(),
			diagnosticState("alias bound to the selected entity"),
			diagnosticState("alias unbound"),
		)
	case UnsettledAliasResolution:
		if resolution.Alias() != change.oldAlias || resolution.Context() != change.context {
			accumulator.addMissingResolution(
				DiagnosticIdentityBasisMissing,
				"superseded alias lookup returned unsettled evidence for another alias/context",
				path+".old_alias",
				"rebuild-or-inspect-alias-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"superseded alias does not resolve exactly",
			path+".old_alias",
			"resolve-the-current-alias-before-supersession",
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"alias lookup returned no recognized basis",
			path+".old_alias",
			"inspect-alias-index",
		)
	}
}

func validateExactEntity(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	entity EntityID,
	context BoundedContextRef,
	changeOrdinal uint64,
	path string,
) {
	switch resolution := snapshot.ResolveEntity(entity, context).(type) {
	case ExactEntityResolution:
		if resolution.Entity() != entity ||
			resolution.Context() != context ||
			!resolution.Basis().valid() {
			accumulator.addInvalidWithWitness(
				DiagnosticAliasAmbiguous,
				"entity lookup resolved to a different canonical identity",
				path,
				coreValidatorGoverningBasis(),
				diagnosticSet([]string{entity.String(), context.String()}),
				diagnosticSet([]string{
					resolution.Entity().String(),
					resolution.Context().String(),
				}),
			)
			return
		}
		observation, err := NewEntityExactObservation(changeOrdinal, resolution)
		accumulator.captureObservation(observation, err, path)
	case CandidateEntityResolution:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"entity identity remains ambiguous",
			path,
			"reconcile-entity-candidates",
		)
	case AbsentEntityResolution:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"entity is known absent in the requested context",
			path,
			"declare-or-resolve-the-entity-before-identity-change",
		)
	case UnknownEntityResolution:
		validateUnknownEntityResolution(
			accumulator,
			resolution,
			entity,
			context,
			path,
			"identity change requires an exact entity resolution basis",
		)
	case UnsettledEntityResolution:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"entity identity is unsettled",
			path,
			"settle-entity-identity",
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticIdentityBasisMissing,
			"entity lookup returned no recognized basis",
			path,
			"inspect-entity-index",
		)
	}
}

func validateUnknownEntityResolution(
	accumulator *validationAccumulator,
	resolution UnknownEntityResolution,
	entity EntityID,
	context BoundedContextRef,
	path string,
	requiredState string,
) {
	basisValues := make([]string, 0, len(resolution.MissingBasis()))
	for _, basis := range resolution.MissingBasis() {
		basisValues = append(basisValues, basis.String())
	}
	required := diagnosticState(requiredState)
	missingBasis := diagnosticSet(basisValues)
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, missingBasis)
	if resolution.Entity() != entity || resolution.Context() != context {
		expected := diagnosticSet([]string{entity.String(), context.String()})
		actual := diagnosticSet([]string{
			resolution.Entity().String(),
			resolution.Context().String(),
		})
		accumulator.addUnderdeterminedWithWitness(
			DiagnosticIdentityBasisMissing,
			"entity lookup returned missing bases for another entity/context",
			path,
			"rebuild-or-inspect-entity-identity-index",
			expected,
			actual,
			basis,
		)
		return
	}
	accumulator.addUnderdeterminedWithWitness(
		DiagnosticIdentityBasisMissing,
		"entity lookup is missing exact resolution bases",
		path,
		"recover-entity-resolution-basis:"+strings.Join(basisValues, ","),
		required,
		missingBasis,
		basis,
	)
}

func validateRelationInstantiation(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	locals map[string]localDeclaration,
	relation RelationInstantiation,
	changeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) {
	validateRelationalCandidate(
		accumulator,
		environment,
		registry,
		snapshot,
		locals,
		relation,
		changeOrdinal,
		candidatePrefix,
		path,
	)
}

func validateRelationalAssertionCandidate(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	locals map[string]localDeclaration,
	assertion RelationalAssertionCandidate,
	changeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) {
	validateRelationalCandidate(
		accumulator,
		environment,
		registry,
		snapshot,
		locals,
		assertion,
		changeOrdinal,
		candidatePrefix,
		path,
	)
}

func validateRelationalCandidate(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	locals map[string]localDeclaration,
	relation candidateRelationalCarrier,
	changeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) {
	start := len(accumulator.diagnostics)
	relationSlice := relation.Slice()
	relationContext := relationSlice.Context()
	if !contextActive(accumulator, environment, relationContext, path+".bounded_context") {
		return
	}

	fragmentRef := relation.RelationDeclarationFragmentRef()
	fragment, found := environment.TypedRelationDeclarationFragment(fragmentRef)
	if !found {
		required := diagnosticReference(fragmentRef.String())
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticSignatureNotActive,
			fmt.Sprintf(
				"typed relation declaration fragment %s is absent from the active TypeEnv",
				fragmentRef.String(),
			),
			path+".signature",
			"inspect-or-stage-the-required-typed-relation-declaration-fragment",
			required,
			basis,
		)
		return
	}
	if !fragmentContainsContext(fragment, relationContext) {
		accumulator.addInvalidWithWitness(
			DiagnosticSignatureContextMismatch,
			fmt.Sprintf(
				"typed relation declaration fragment %s does not declare context %s",
				fragmentRef.String(),
				relationContext.String(),
			),
			path+".bounded_context",
			declarationGoverningBasis(fragment.Provenance()),
			diagnosticFragmentContexts(fragment),
			diagnosticReference(relationContext.String()),
		)
	}
	validateNewAssertion(
		accumulator,
		snapshot,
		relation.Assertion(),
		changeOrdinal,
		path+".assertion_id",
	)

	candidateBindings := relation.Bindings()
	bindings := make(map[string]CandidateSlotBinding, len(candidateBindings))
	for _, binding := range candidateBindings {
		bindings[binding.Name().String()] = binding
		if _, exists := fragment.Slot(binding.Name()); !exists {
			accumulator.addInvalidWithWitness(
				DiagnosticUnknownSlot,
				fmt.Sprintf(
					"slot %s is not declared by typed relation declaration fragment %s",
					binding.Name().String(),
					fragmentRef.String(),
				),
				path+".slots."+binding.Name().String(),
				declarationGoverningBasis(fragment.Provenance()),
				diagnosticSlotNames(fragment),
				diagnosticReference(binding.Name().String()),
			)
		}
	}

	validatedBindings := make([]SlotBinding, 0, len(fragment.Slots()))
	referenceEvidence := make(map[string][]referenceAdmissionEvidence)
	for _, slot := range fragment.Slots() {
		binding, present := bindings[slot.SlotKind().String()]
		fillers := []CandidateSlotFiller(nil)
		if present {
			fillers = binding.Fillers()
		}
		slotPath := path + ".slots." + slot.SlotKind().String()
		if !present && slot.Cardinality().Minimum() > 0 {
			accumulator.addInvalidWithWitness(
				DiagnosticMissingSlot,
				fmt.Sprintf("required slot %s is absent", slot.SlotKind().String()),
				slotPath,
				declarationGoverningBasis(slot.Provenance()),
				NewDiagnosticCountDatum(slot.Cardinality().Minimum()),
				NewDiagnosticCountDatum(0),
			)
			continue
		}
		if !slot.Cardinality().Allows(uint64(len(fillers))) {
			accumulator.addInvalidWithWitness(
				DiagnosticCardinalityMismatch,
				fmt.Sprintf("slot %s has %d fillers outside its declared cardinality", slot.SlotKind().String(), len(fillers)),
				slotPath,
				declarationGoverningBasis(slot.Provenance()),
				diagnosticCardinality(slot.Cardinality()),
				NewDiagnosticCountDatum(uint64(len(fillers))),
			)
			continue
		}
		if len(fillers) == 0 {
			continue
		}

		validatedFillers := validateSlotFillers(
			accumulator,
			environment,
			registry,
			snapshot,
			relationSlice,
			locals,
			changeOrdinal,
			candidatePrefix,
			slot,
			fillers,
			slotPath,
		)
		if len(validatedFillers.fillers) == len(fillers) {
			binding := newSlotBinding(slot.SlotKind(), validatedFillers.fillers)
			validatedBindings = append(validatedBindings, binding)
			referenceEvidence[slot.SlotKind().String()] = validatedFillers.references
		}
	}
	validateSlotGroupConstraints(accumulator, environment, relation, bindings, path)

	if len(accumulator.diagnostics) == start {
		var final validatedRelationalCarrier
		var validatedChange ValidatedMemoryChange
		switch value := relation.(type) {
		case RelationInstantiation:
			instance := newRelationInstance(value, validatedBindings)
			final = instance
			validatedChange = ValidatedRelationInstance{relation: instance}
		case RelationalAssertionCandidate:
			assertion := newRelationalAssertion(value, validatedBindings)
			final = assertion
			validatedChange = ValidatedRelationalAssertion{assertion: assertion}
		default:
			accumulator.addMissingResolution(
				DiagnosticTypeRuleUnavailable,
				fmt.Sprintf("validator cannot finalize relational candidate %T", relation),
				path,
				"inspect-and-repair-typed-memory-validator",
			)
			return
		}
		validateRelationConstraints(
			accumulator,
			environment,
			final,
			path,
		)
		if len(accumulator.diagnostics) != start {
			return
		}
		captureRelationReferenceUses(
			accumulator,
			environment,
			changeOrdinal,
			final,
			referenceEvidence,
			path,
		)
		if len(accumulator.diagnostics) == start {
			accumulator.validated = append(accumulator.validated, validatedChange)
		}
	}
}

func validateRelationConstraints(
	accumulator *validationAccumulator,
	environment TypeEnv,
	relation validatedRelationalCarrier,
	path string,
) {
	view, err := newRelationConstraintEvaluationViewFromBindings(
		relation.RelationDeclarationFragmentRef(),
		relation.Bindings(),
	)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"validator could not construct the relation-constraint view: "+err.Error(),
			path+".relation_constraints",
			"inspect-and-repair-typed-memory-validator",
		)
		return
	}
	outcome := EvaluateRelationConstraints(environment, view)
	accumulator.diagnostics = append(
		accumulator.diagnostics,
		outcome.Diagnostics()...,
	)
}

type referenceAdmissionEvidence struct {
	filler     ReferenceFiller
	resolution AdmissionReferenceResolution
	proof      referenceAdmissionProof
}

type referenceAdmissionProof interface {
	canonicalBytes() []byte
	referenceAdmissionProofVariant()
}

type membershipReferenceAdmissionProof struct {
	required MemberOfMember
	disjoint []DisjointCounterUse
}

func (proof membershipReferenceAdmissionProof) canonicalBytes() []byte {
	disjoint := append([]DisjointCounterUse(nil), proof.disjoint...)
	sort.Slice(disjoint, func(left, right int) bool {
		return bytes.Compare(
			disjoint[left].CanonicalBytes(),
			disjoint[right].CanonicalBytes(),
		) < 0
	})
	writer := newCanonicalWriter("typed-memory.membership-reference-admission-proof.v1")
	writer.addBytes(proof.required.CanonicalBytes())
	writer.addUint64(uint64(len(disjoint)))
	for _, use := range disjoint {
		writer.addBytes(use.CanonicalBytes())
	}
	return writer.bytes()
}

func (membershipReferenceAdmissionProof) referenceAdmissionProofVariant() {}

type classificationReferenceAdmissionProof struct {
	required TrueKindClassification
	disjoint []ClassificationDisjointUse
}

func (proof classificationReferenceAdmissionProof) canonicalBytes() []byte {
	disjoint := append([]ClassificationDisjointUse(nil), proof.disjoint...)
	sort.Slice(disjoint, func(left, right int) bool {
		return bytes.Compare(
			disjoint[left].CanonicalBytes(),
			disjoint[right].CanonicalBytes(),
		) < 0
	})
	writer := newCanonicalWriter("typed-memory.classification-reference-admission-proof.v1")
	writer.addBytes(proof.required.CanonicalBytes())
	writer.addUint64(uint64(len(disjoint)))
	for _, use := range disjoint {
		writer.addBytes(use.CanonicalBytes())
	}
	return writer.bytes()
}

func (classificationReferenceAdmissionProof) referenceAdmissionProofVariant() {}

type validatedSlotFillerSet struct {
	fillers    []SlotFiller
	references []referenceAdmissionEvidence
}

func validateSlotFillers(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	snapshot MemorySnapshot,
	contextSlice ContextSlice,
	locals map[string]localDeclaration,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	slot SlotSpec,
	fillers []CandidateSlotFiller,
	path string,
) validatedSlotFillerSet {
	context := contextSlice.Context()
	validated := validatedSlotFillerSet{
		fillers:    make([]SlotFiller, 0, len(fillers)),
		references: make([]referenceAdmissionEvidence, 0, len(fillers)),
	}
	for index, filler := range fillers {
		fillerPath := fmt.Sprintf("%s.fillers[%d]", path, index)
		switch target := slot.Target().(type) {
		case ValueSlotTarget:
			value, ok := filler.(ByValueCandidate)
			if !ok {
				accumulator.addInvalidWithWitness(
					DiagnosticReferenceModeMismatch,
					"slot requires ByValue content",
					fillerPath,
					declarationGoverningBasis(slot.Provenance()),
					diagnosticState(SlotByValue.String()),
					diagnosticText(fmt.Sprintf("%T", filler)),
				)
				continue
			}
			verified, ok := validateByValue(
				accumulator,
				environment,
				registry,
				context,
				target,
				value,
				fillerPath,
				slot.Provenance(),
			)
			if ok {
				validated.fillers = append(validated.fillers, newValueFiller(verified))
			}
		case ReferenceSlotTarget:
			reference, ok := filler.(ByReferenceCandidate)
			if !ok {
				accumulator.addInvalidWithWitness(
					DiagnosticReferenceModeMismatch,
					"slot requires a RefKind reference",
					fillerPath,
					declarationGoverningBasis(slot.Provenance()),
					diagnosticState(SlotByReference.String()),
					diagnosticText(fmt.Sprintf("%T", filler)),
				)
				continue
			}
			evidence, admitted := validateByReference(
				accumulator,
				environment,
				snapshot,
				contextSlice,
				locals,
				evaluationChangeOrdinal,
				candidatePrefix,
				target,
				reference,
				fillerPath,
				slot.Provenance(),
			)
			if admitted {
				validated.fillers = append(validated.fillers, evidence.filler)
				validated.references = append(validated.references, evidence)
			}
		}
	}
	return validated
}

func captureRelationReferenceUses(
	accumulator *validationAccumulator,
	environment TypeEnv,
	changeOrdinal uint64,
	relation validatedRelationalCarrier,
	evidenceBySlot map[string][]referenceAdmissionEvidence,
	path string,
) {
	for _, binding := range relation.Bindings() {
		evidence := canonicalReferenceAdmissionEvidenceOrder(
			evidenceBySlot[binding.Name().String()],
		)
		matched := make([]bool, len(evidence))
		for fillerOrdinal, filler := range binding.Fillers() {
			reference, ok := filler.(ReferenceFiller)
			if !ok {
				continue
			}
			fillerPath := fmt.Sprintf(
				"%s.slots.%s.fillers[%d]",
				path,
				binding.Name().String(),
				fillerOrdinal,
			)
			evidenceIndex := matchingReferenceAdmissionEvidence(reference, evidence, matched)
			if evidenceIndex < 0 {
				accumulator.addMissingResolution(
					DiagnosticTypeRuleUnavailable,
					"admitted reference filler has no exact validation evidence",
					fillerPath,
					"inspect-and-repair-typed-memory-validator",
				)
				continue
			}
			matched[evidenceIndex] = true
			selected := evidence[evidenceIndex]
			fillerOrdinalValue, exact := exactUint64FromNonNegativeInt(fillerOrdinal)
			if !exact {
				accumulator.addMissingResolution(
					DiagnosticTypeRuleUnavailable,
					"relation filler ordinal exceeds the canonical uint64 range",
					fillerPath,
					"inspect-and-repair-typed-memory-validator",
				)
				continue
			}
			coordinate, err := newRelationFillerCoordinate(
				environment,
				changeOrdinal,
				relation,
				binding.Name(),
				fillerOrdinalValue,
			)
			if err != nil {
				accumulator.captureReferenceUse(nil, err, fillerPath)
				continue
			}
			captureReferenceAdmissionProof(
				accumulator,
				environment,
				coordinate,
				selected,
				fillerPath,
			)
		}
	}
}

func captureReferenceAdmissionProof(
	accumulator *validationAccumulator,
	environment TypeEnv,
	coordinate RelationFillerCoordinate,
	evidence referenceAdmissionEvidence,
	path string,
) {
	switch proof := evidence.proof.(type) {
	case membershipReferenceAdmissionProof:
		use, err := NewReferenceFillerAdmissionUse(
			ReferenceFillerAdmissionUseInput{
				TypeEnv:             environment,
				Coordinate:          coordinate,
				Resolution:          evidence.resolution,
				RequiredMembership:  proof.required,
				DisjointMemberships: proof.disjoint,
			},
		)
		accumulator.captureReferenceUse(use, err, path)
	case classificationReferenceAdmissionProof:
		use, err := NewClassificationReferenceFillerAdmissionUse(
			ClassificationReferenceFillerAdmissionUseInput{
				TypeEnv:                 environment,
				Coordinate:              coordinate,
				Resolution:              evidence.resolution,
				RequiredClassification:  proof.required,
				DisjointClassifications: proof.disjoint,
			},
		)
		accumulator.captureClassificationReferenceUse(use, err, path)
	default:
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"admitted reference filler has no recognized classification proof",
			path,
			"inspect-and-repair-typed-memory-validator",
		)
	}
}

func canonicalReferenceAdmissionEvidenceOrder(
	values []referenceAdmissionEvidence,
) []referenceAdmissionEvidence {
	ordered := append([]referenceAdmissionEvidence(nil), values...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftBytes := canonicalReferenceAdmissionEvidence(ordered[left])
		rightBytes := canonicalReferenceAdmissionEvidence(ordered[right])
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	return ordered
}

func canonicalReferenceAdmissionEvidence(
	evidence referenceAdmissionEvidence,
) []byte {
	writer := newCanonicalWriter("typed-memory.reference-admission-evidence-order.v1")
	writer.addBytes(canonicalSlotFiller(evidence.filler))
	writer.addBytes(evidence.resolution.CanonicalBytes())
	if evidence.proof != nil {
		writer.addBytes(evidence.proof.canonicalBytes())
	}
	return writer.bytes()
}

func matchingReferenceAdmissionEvidence(
	filler ReferenceFiller,
	evidence []referenceAdmissionEvidence,
	matched []bool,
) int {
	for index, candidate := range evidence {
		if matched[index] {
			continue
		}
		if candidate.filler.Reference() == filler.Reference() &&
			candidate.filler.Entity() == filler.Entity() {
			return index
		}
	}
	return -1
}

func validateByValue(
	accumulator *validationAccumulator,
	environment TypeEnv,
	registry CodecRegistry,
	context BoundedContextRef,
	target ValueSlotTarget,
	candidate ByValueCandidate,
	path string,
	provenance DeclarationProvenance,
) (VerifiedTypedValue, bool) {
	actualKind := candidate.Value().ValueKind()
	targetKind := target.ValueKind()
	if actualKind.TypeEnv() != environment.Ref() ||
		targetKind.TypeEnv() != environment.Ref() ||
		!environment.IsSubkind(actualKind.ID(), targetKind.ID()) {
		accumulator.addInvalidWithWitness(
			DiagnosticValueKindMismatch,
			"candidate ValueKind is neither the SlotSpec ValueKind nor a compatible subkind",
			path+".value_kind_ref",
			declarationGoverningBasis(provenance),
			diagnosticReference(targetKind.String()),
			diagnosticReference(actualKind.String()),
		)
		return nil, false
	}
	if !environment.HasKindInContext(context, actualKind.ID()) {
		required := diagnosticSet([]string{context.String(), actualKind.String()})
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticKindUnavailableInContext,
			fmt.Sprintf("ValueKind %s is unavailable in context %s", actualKind.String(), context.String()),
			path+".value_kind_ref",
			"inspect-or-compile-the-context-kind-availability",
			required,
			basis,
		)
		return nil, false
	}
	if !validateStaticKindDisjointness(
		accumulator,
		environment,
		actualKind,
		path+".value_kind_ref",
	) {
		return nil, false
	}
	binding, found := environment.ValueBinding(actualKind)
	if !found {
		required := diagnosticReference(actualKind.String())
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticValueBindingNotActive,
			fmt.Sprintf("ValueKind %s has no active shape/codec binding", actualKind.String()),
			path+".value_kind_ref",
			"inspect-or-stage-the-required-value-binding",
			required,
			basis,
		)
		return nil, false
	}

	switch result := VerifyTypedValue(registry, binding, candidate.Value()).(type) {
	case ValidTypedValue:
		return result.Value(), true
	case InvalidTypedValue:
		accumulator.diagnostics = append(
			accumulator.diagnostics,
			rebaseTypedValueDiagnostics(result.Diagnostics(), path+".value")...,
		)
	case UnderdeterminedTypedValue:
		accumulator.diagnostics = append(
			accumulator.diagnostics,
			rebaseTypedValueDiagnostics(result.Diagnostics(), path+".value")...,
		)
	}
	return nil, false
}

func rebaseTypedValueDiagnostics(
	diagnostics []Diagnostic,
	basePath string,
) []Diagnostic {
	rebased := make([]Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		relative := diagnostic.path.String()
		relative = strings.TrimPrefix(relative, "typed_value.")
		updated := diagnostic
		updated.path = DiagnosticPath{value: basePath + "." + relative}
		updated.repairs = rebaseDiagnosticRepairs(updated.repairs, updated.path)
		rebased = append(rebased, updated)
	}
	return rebased
}

func rebaseDiagnosticRepairs(
	repairs []RepairCandidate,
	path DiagnosticPath,
) []RepairCandidate {
	updated := append([]RepairCandidate(nil), repairs...)
	for index, repair := range updated {
		if repair.Kind() != RepairChangeInput {
			continue
		}
		pointer, _ := NewRepairPointer("change-candidate-at:" + path.String())
		rebased, _ := NewRepairCandidate(
			repair.Kind(),
			pointer,
			repair.Target(),
			repair.HumanChoiceRequirement(),
		)
		updated[index] = rebased
	}
	return updated
}

func validateByReference(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	contextSlice ContextSlice,
	locals map[string]localDeclaration,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	target ReferenceSlotTarget,
	candidate ByReferenceCandidate,
	path string,
	provenance DeclarationProvenance,
) (referenceAdmissionEvidence, bool) {
	context := contextSlice.Context()
	reference := candidate.Reference()
	if reference.RefKind() != target.ReferenceKind() {
		accumulator.addInvalidWithWitness(
			DiagnosticReferenceKindMismatch,
			"reference RefKind does not match the SlotSpec RefKind",
			path+".ref_kind",
			declarationGoverningBasis(provenance),
			diagnosticReference(target.ReferenceKind().String()),
			diagnosticReference(reference.RefKind().String()),
		)
		return referenceAdmissionEvidence{}, false
	}
	definition, found := environment.RefKindDefinition(target.ReferenceKind())
	if !found {
		required := diagnosticReference(target.ReferenceKind().String())
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticTypeRuleUnavailable,
			fmt.Sprintf("RefKind %s has no active definition", target.ReferenceKind().String()),
			path+".ref_kind",
			"inspect-or-stage-the-required-refkind-definition",
			required,
			basis,
		)
		return referenceAdmissionEvidence{}, false
	}
	if !environment.IsSubkind(target.ValueKind().ID(), definition.ValueKind().ID()) {
		accumulator.addInvalidWithWitness(
			DiagnosticReferenceKindMismatch,
			"SlotSpec ValueKind is not the RefKind referent kind or a compatible subkind",
			path+".ref_kind",
			declarationGoverningBasis(definition.Provenance()),
			diagnosticReference(definition.ValueKind().String()),
			diagnosticReference(target.ValueKind().String()),
		)
		return referenceAdmissionEvidence{}, false
	}
	if !environment.HasKindInContext(context, target.ValueKind().ID()) {
		required := diagnosticSet([]string{context.String(), target.ValueKind().String()})
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticKindUnavailableInContext,
			fmt.Sprintf("ValueKind %s is unavailable in context %s", target.ValueKind().String(), context.String()),
			path+".value_kind",
			"inspect-or-compile-the-context-kind-availability",
			required,
			basis,
		)
		return referenceAdmissionEvidence{}, false
	}

	filler, resolution, resolved := resolveReferenceForAdmission(
		accumulator,
		environment,
		snapshot,
		context,
		locals,
		target,
		reference,
		path,
	)
	if !resolved {
		return referenceAdmissionEvidence{}, false
	}
	entity := filler.Entity()
	currentEvidence, currentAdmitted, currentHandled := validateByCurrentClassification(
		accumulator,
		environment,
		snapshot,
		filler,
		resolution,
		entity,
		target,
		contextSlice,
		evaluationChangeOrdinal,
		candidatePrefix,
		path,
	)
	if currentHandled {
		return currentEvidence, currentAdmitted
	}

	query, err := NewMemberOfQuery(entity, target.ValueKind(), contextSlice)
	if err != nil {
		required := diagnosticSet([]string{
			entity.String(),
			target.ValueKind().String(),
			contextSlice.Ref().String(),
		})
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticTypeRuleUnavailable,
			"exact MemberOf query could not be constructed",
			path+".value_kind",
			"rebuild-the-complete-context-slice",
			required,
			basis,
		)
		return referenceAdmissionEvidence{}, false
	}
	view, err := memberOfEvaluationViewForResolution(
		environment.Ref(),
		snapshot.GraphRevision(),
		resolution,
		evaluationChangeOrdinal,
		candidatePrefix,
	)
	if err != nil {
		accumulator.addInvalidWithWitness(
			DiagnosticReferenceUnresolved,
			"reference cannot form an exact MemberOf evaluation view: "+err.Error(),
			path+".reference",
			coreValidatorGoverningBasis(),
			diagnosticState("reference resolved before use with exact evaluation basis"),
			diagnosticText(err.Error()),
		)
		return referenceAdmissionEvidence{}, false
	}
	request, err := NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"exact MemberOf evaluation request could not be constructed: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-typed-memory-validator",
		)
		return referenceAdmissionEvidence{}, false
	}
	judgement := snapshot.EvaluateMemberOf(request)
	if !validateMemberOfCorrelation(accumulator, request, judgement, path+".value_kind") {
		return referenceAdmissionEvidence{}, false
	}
	switch value := judgement.(type) {
	case MemberOfMember:
		disjoint, constraintsValid := validateDisjointMembershipConstraints(
			accumulator,
			environment,
			snapshot,
			entity,
			target.ValueKind(),
			value,
			contextSlice,
			view,
			path,
		)
		if !constraintsValid {
			return referenceAdmissionEvidence{}, false
		}
		return referenceAdmissionEvidence{
			filler:     filler,
			resolution: resolution,
			proof: membershipReferenceAdmissionProof{
				required: value,
				disjoint: disjoint,
			},
		}, true
	case MemberOfNotMember:
		accumulator.addInvalidWithWitness(
			DiagnosticEntityKindMismatch,
			"resolved referent is known not to belong to the SlotSpec ValueKind",
			path+".value_kind",
			snapshotRuleGoverningBasis(value.Basis().Evaluator()),
			diagnosticReference(target.ValueKind().String()),
			diagnosticState(NotMemberJudgement.String()),
		)
	case MemberOfUndefined:
		addUndefinedMemberOfDiagnostic(
			accumulator,
			"entity-kind membership basis is not available",
			path+".value_kind",
			value,
		)
	default:
		// validateMemberOfCorrelation rejects invalid or unknown variants.
	}
	return referenceAdmissionEvidence{}, false
}

func validateByCurrentClassification(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	filler ReferenceFiller,
	resolution AdmissionReferenceResolution,
	entity EntityID,
	target ReferenceSlotTarget,
	contextSlice ContextSlice,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) (referenceAdmissionEvidence, bool, bool) {
	localKind, err := NewLocalKindRef(
		target.ValueKind(),
		contextSlice.Context(),
	)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"exact current local kind could not be constructed: "+err.Error(),
			path+".value_kind",
			"recompile-the-active-typeenv",
		)
		return referenceAdmissionEvidence{}, false, true
	}
	signature, found := environment.KindClassificationSignatureDefinition(localKind)
	if !found {
		return referenceAdmissionEvidence{}, false, false
	}
	classificationSnapshot, supported := snapshot.(KindClassificationSnapshot)
	if !supported {
		required := diagnosticReference(signature.Criterion().String())
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticTypeRuleUnavailable,
			"current KindSignature has no kind-classification snapshot evaluator",
			path+".value_kind",
			"provide-kind-classification-evaluator",
			required,
			basis,
		)
		return referenceAdmissionEvidence{}, false, true
	}
	request, err := newReferenceKindClassificationRequest(
		entity,
		localKind,
		signature,
		contextSlice,
	)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"exact current kind-classification request could not be constructed: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-kind-classification-request",
		)
		return referenceAdmissionEvidence{}, false, true
	}
	judgement := evaluateKindClassificationForAdmission(
		classificationSnapshot,
		request,
		resolution,
		evaluationChangeOrdinal,
		candidatePrefix,
	)
	if !validateKindClassificationCorrelation(
		accumulator,
		request,
		judgement,
		path+".value_kind",
	) {
		return referenceAdmissionEvidence{}, false, true
	}
	switch value := judgement.(type) {
	case TrueKindClassification:
		disjoint, constraintsValid := validateDisjointClassificationConstraints(
			accumulator,
			environment,
			classificationSnapshot,
			entity,
			target.ValueKind(),
			contextSlice,
			resolution,
			evaluationChangeOrdinal,
			candidatePrefix,
			path,
		)
		if !constraintsValid {
			return referenceAdmissionEvidence{}, false, true
		}
		return referenceAdmissionEvidence{
			filler:     filler,
			resolution: resolution,
			proof: classificationReferenceAdmissionProof{
				required: value,
				disjoint: disjoint,
			},
		}, true, true
	case FalseKindClassification:
		accumulator.addInvalidWithWitness(
			DiagnosticEntityKindMismatch,
			"resolved referent is directly classified false for the SlotSpec local kind",
			path+".value_kind",
			snapshotRuleGoverningBasis(value.Basis().Criterion()),
			diagnosticState(KindClassificationTrue.String()),
			diagnosticState(KindClassificationFalse.String()),
		)
	case UnknownKindClassification:
		addUnknownKindClassificationDiagnostic(
			accumulator,
			"entity-kind classification basis is not available",
			path+".value_kind",
			value,
		)
	}
	return referenceAdmissionEvidence{}, false, true
}

func newReferenceKindClassificationRequest(
	entity EntityID,
	localKind LocalKindRef,
	signature KindClassificationSignatureDefinition,
	contextSlice ContextSlice,
) (KindClassificationRequest, error) {
	candidate, err := NewExactKindEntityCandidate(
		entity,
		signature.CandidateValueKind(),
	)
	if err != nil {
		return KindClassificationRequest{}, err
	}
	return NewKindClassificationRequest(KindClassificationRequestInput{
		Candidate:        candidate,
		LocalKind:        localKind,
		SignatureEdition: signature.Ref(),
		ContextSlice:     contextSlice,
	})
}

func validateDisjointClassificationConstraints(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot KindClassificationSnapshot,
	entity EntityID,
	valueKind ValueKindRef,
	contextSlice ContextSlice,
	resolution AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) ([]ClassificationDisjointUse, bool) {
	start := len(accumulator.diagnostics)
	uses := make([]ClassificationDisjointUse, 0)
	for _, rule := range environment.Constraints() {
		constraint, supported := rule.(KindDisjointConstraint)
		if !supported {
			continue
		}
		matchedKinds := disjointConstraintMatches(
			environment,
			constraint,
			valueKind.ID(),
		)
		if len(matchedKinds) > 1 {
			accumulator.addInvalidWithWitness(
				DiagnosticEntityKindMismatch,
				fmt.Sprintf(
					"ValueKind %s is a subkind of multiple mutually disjoint kinds",
					valueKind.String(),
				),
				path+".value_kind",
				declarationGoverningBasis(constraint.Provenance()),
				diagnosticState("at most one disjoint kind"),
				diagnosticKindIDs(matchedKinds),
			)
			continue
		}
		for _, matchedKind := range matchedKinds {
			for _, counterKind := range constraint.Kinds() {
				if counterKind == matchedKind {
					continue
				}
				use, valid := validateDisjointCounterClassification(
					accumulator,
					environment,
					snapshot,
					constraint,
					entity,
					counterKind,
					contextSlice,
					resolution,
					evaluationChangeOrdinal,
					candidatePrefix,
					path,
				)
				if valid {
					uses = append(uses, use)
				}
			}
		}
	}
	return uses, len(accumulator.diagnostics) == start
}

func validateDisjointCounterClassification(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot KindClassificationSnapshot,
	constraint KindDisjointConstraint,
	entity EntityID,
	counterKind KindID,
	contextSlice ContextSlice,
	resolution AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
	path string,
) (ClassificationDisjointUse, bool) {
	valueKind, err := NewValueKindRef(environment.Ref(), counterKind)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"disjoint constraint contains an unusable current ValueKindRef: "+err.Error(),
			path+".value_kind",
			"recompile-the-active-typeenv",
		)
		return ClassificationDisjointUse{}, false
	}
	localKind, err := NewLocalKindRef(valueKind, contextSlice.Context())
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"disjoint constraint cannot form an exact current local kind: "+err.Error(),
			path+".value_kind",
			"recompile-the-active-typeenv",
		)
		return ClassificationDisjointUse{}, false
	}
	signature, found := environment.KindClassificationSignatureDefinition(localKind)
	if !found {
		required := diagnosticReference(localKind.String())
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticTypeRuleUnavailable,
			"disjoint counter-kind has no exact current KindSignature",
			path+".value_kind",
			"compile-the-current-counter-kind-signature",
			required,
			basis,
		)
		return ClassificationDisjointUse{}, false
	}
	request, err := newReferenceKindClassificationRequest(
		entity,
		localKind,
		signature,
		contextSlice,
	)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"exact disjoint kind-classification request could not be constructed: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-kind-classification-request",
		)
		return ClassificationDisjointUse{}, false
	}
	judgement := evaluateKindClassificationForAdmission(
		snapshot,
		request,
		resolution,
		evaluationChangeOrdinal,
		candidatePrefix,
	)
	if !validateKindClassificationCorrelation(
		accumulator,
		request,
		judgement,
		path+".value_kind",
	) {
		return ClassificationDisjointUse{}, false
	}
	switch value := judgement.(type) {
	case FalseKindClassification:
		use, useErr := NewClassificationDisjointUse(constraint.ID(), value)
		if useErr == nil {
			return use, true
		}
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"validator could not seal direct false disjoint classification: "+useErr.Error(),
			path+".value_kind",
			"inspect-and-repair-typed-memory-validator",
		)
	case TrueKindClassification:
		accumulator.addInvalidWithWitness(
			DiagnosticEntityKindMismatch,
			fmt.Sprintf(
				"referent is directly classified true for disjoint local kind %s",
				localKind.String(),
			),
			path+".value_kind",
			snapshotRuleGoverningBasis(value.Basis().Criterion()),
			diagnosticState(KindClassificationFalse.String()),
			diagnosticState(KindClassificationTrue.String()),
		)
	case UnknownKindClassification:
		addUnknownKindClassificationDiagnostic(
			accumulator,
			"disjoint counter-kind classification basis is not available",
			path+".value_kind",
			value,
		)
	}
	return ClassificationDisjointUse{}, false
}

func evaluateKindClassificationForAdmission(
	snapshot KindClassificationSnapshot,
	request KindClassificationRequest,
	resolution AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
) KindClassificationJudgement {
	admissionSnapshot, supported := snapshot.(KindClassificationAdmissionSnapshot)
	if !supported {
		return snapshot.EvaluateKindClassification(request)
	}
	return admissionSnapshot.EvaluateKindClassificationForAdmission(
		request,
		resolution,
		evaluationChangeOrdinal,
		candidatePrefix,
	)
}

func validateKindClassificationCorrelation(
	accumulator *validationAccumulator,
	request KindClassificationRequest,
	judgement KindClassificationJudgement,
	path string,
) bool {
	if KindClassificationJudgementMatchesRequest(request, judgement) {
		return true
	}
	actual := diagnosticText(fmt.Sprintf("%T", judgement))
	if KindClassificationJudgementValid(judgement) {
		actual = diagnosticSet([]string{
			judgement.Kind().String(),
			judgement.Request().Digest().String(),
		})
	}
	switch judgement.(type) {
	case TrueKindClassification, FalseKindClassification:
		accumulator.addInvalidWithWitness(
			DiagnosticTypeRuleUnavailable,
			"kind-classification evaluator returned a defined judgement with an uncorrelated or corrupt basis",
			path,
			coreValidatorGoverningBasis(),
			diagnosticReference(request.Digest().String()),
			actual,
		)
	default:
		required := diagnosticReference(request.Digest().String())
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
		accumulator.addUnderdeterminedWithWitness(
			DiagnosticTypeRuleUnavailable,
			"kind-classification evaluator returned no exactly correlated judgement",
			path,
			"rebuild-or-inspect-the-kind-classification-index",
			required,
			actual,
			basis,
		)
	}
	return false
}

func addUnknownKindClassificationDiagnostic(
	accumulator *validationAccumulator,
	message string,
	path string,
	judgement UnknownKindClassification,
) {
	reasons := judgement.Reasons()
	values := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		values = append(
			values,
			reason.Kind().String()+":"+reason.RepairPointer().String(),
		)
	}
	required := diagnosticState(KindClassificationTrue.String())
	actual := diagnosticSet(values)
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, actual)
	accumulator.addUnderdeterminedWithWitness(
		DiagnosticTypeRuleUnavailable,
		message,
		path,
		reasons[0].RepairPointer().String(),
		required,
		actual,
		basis,
	)
}

func resolveReferenceForAdmission(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	context BoundedContextRef,
	locals map[string]localDeclaration,
	target ReferenceSlotTarget,
	reference StrongRef,
	path string,
) (ReferenceFiller, AdmissionReferenceResolution, bool) {
	switch value := reference.(type) {
	case LocalRef:
		return resolveLocalReferenceForAdmission(accumulator, context, locals, value, path)
	case PersistedRef:
		return resolvePersistedReferenceForAdmission(
			accumulator,
			environment,
			snapshot,
			context,
			target,
			value,
			path,
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"reference has no recognized strong-reference variant",
			path+".reference",
			"inspect-the-exact-reference",
		)
		return ReferenceFiller{}, nil, false
	}
}

func resolveLocalReferenceForAdmission(
	accumulator *validationAccumulator,
	context BoundedContextRef,
	locals map[string]localDeclaration,
	reference LocalRef,
	path string,
) (ReferenceFiller, AdmissionReferenceResolution, bool) {
	declaration, declared := locals[reference.ReferenceKey()]
	if !declared {
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"batch-local reference has no declaration in this MemoryChangeSet",
			path+".reference",
			"declare-the-batch-local-entity-in-the-same-memory-change-set",
		)
		return ReferenceFiller{}, nil, false
	}
	if declaration.change.Context() != context {
		accumulator.addMissingResolution(
			DiagnosticContextBridgeMissing,
			"batch-local reference belongs to another bounded context; its source-kind membership and exact bridge cannot be proven in this snapshot",
			path+".reference",
			"persist-and-resolve-the-local-entity-through-an-exact-context-bridge",
		)
		return ReferenceFiller{}, nil, false
	}
	entity := declaration.change.Entity()
	if !entity.valid() {
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"batch-local declaration has no stable EntityID",
			path+".reference",
			"inspect-and-repair-the-batch-local-declaration",
		)
		return ReferenceFiller{}, nil, false
	}
	referenceID, err := NewReferenceID(entity.String())
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"batch-local EntityID cannot form a stable persisted reference",
			path+".reference",
			"inspect-and-repair-the-batch-local-declaration",
		)
		return ReferenceFiller{}, nil, false
	}
	persisted, err := NewPersistedRef(reference.RefKind(), referenceID)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"batch-local reference cannot be lowered to stable project identity",
			path+".reference",
			"inspect-and-repair-typed-memory-validator",
		)
		return ReferenceFiller{}, nil, false
	}
	resolution, err := NewSameBatchDeclarationResolution(SameBatchDeclarationResolutionInput{
		LocalReference:           reference,
		DeclarationChangeOrdinal: declaration.ordinal,
		Declaration:              declaration.change,
		PersistedReference:       persisted,
	})
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"batch-local reference could not produce exact admission evidence: "+err.Error(),
			path+".reference",
			"inspect-and-repair-typed-memory-validator",
		)
		return ReferenceFiller{}, nil, false
	}
	return newReferenceFiller(persisted, entity), resolution, true
}

func resolvePersistedReferenceForAdmission(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	context BoundedContextRef,
	target ReferenceSlotTarget,
	reference PersistedRef,
	path string,
) (ReferenceFiller, AdmissionReferenceResolution, bool) {
	switch resolution := snapshot.ResolveReference(reference, context).(type) {
	case ResolvedStrongReference:
		if !sameStrongReference(resolution.Reference(), reference) ||
			resolution.Context() != context ||
			!resolution.Entity().valid() {
			accumulator.addMissingResolution(
				DiagnosticReferenceUnresolved,
				"reference lookup returned an uncorrelated reference, context, or stable EntityID",
				path+".reference",
				"rebuild-or-inspect-the-reference-resolution-index",
			)
			return ReferenceFiller{}, nil, false
		}
		if absence, absent := snapshot.ResolveEntity(resolution.Entity(), context).(AbsentEntityResolution); absent {
			if absence.Entity() != resolution.Entity() || absence.Context() != context {
				accumulator.addMissingResolution(
					DiagnosticReferenceUnresolved,
					"entity lookup returned an uncorrelated absence token for a resolved persisted reference",
					path+".reference",
					"rebuild-or-inspect-the-reference-resolution-index",
				)
				return ReferenceFiller{}, nil, false
			}
			accumulator.addInvalidWithWitness(
				DiagnosticReferenceUnresolved,
				"persisted reference resolution contradicts the same snapshot's exact entity absence",
				path+".reference",
				coreValidatorGoverningBasis(),
				diagnosticState("persisted reference resolves to an entity present in the pre-state snapshot"),
				diagnosticState("the resolved entity is absent in that pre-state snapshot"),
			)
			return ReferenceFiller{}, nil, false
		}
		admissionResolution, err := NewSnapshotReferenceResolution(resolution)
		if err != nil {
			accumulator.addMissingResolution(
				DiagnosticReferenceUnresolved,
				"snapshot reference could not produce exact admission evidence: "+err.Error(),
				path+".reference",
				"inspect-and-repair-typed-memory-validator",
			)
			return ReferenceFiller{}, nil, false
		}
		return newReferenceFiller(reference, resolution.Entity()), admissionResolution, true
	case UnresolvedStrongReference:
		if !sameStrongReference(resolution.Reference(), reference) ||
			resolution.Context() != context {
			accumulator.addMissingResolution(
				DiagnosticReferenceUnresolved,
				"reference lookup returned an uncorrelated missing-basis token",
				path+".reference",
				"rebuild-or-inspect-the-reference-resolution-index",
			)
			return ReferenceFiller{}, nil, false
		}
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"strong reference does not resolve in the current snapshot",
			path+".reference",
			resolution.Repair().String(),
		)
	case MissingContextBridgeResolution:
		if !sameStrongReference(resolution.Reference(), reference) ||
			resolution.Context() != context ||
			resolution.TargetKind() != target.ValueKind().ID() {
			accumulator.addMissingResolution(
				DiagnosticReferenceUnresolved,
				"bridge lookup returned an uncorrelated resolution token",
				path+".reference",
				"rebuild-or-inspect-the-reference-resolution-index",
			)
			return ReferenceFiller{}, nil, false
		}
		if environment.HasContextBridge(
			resolution.SourceContext(),
			resolution.Context(),
			resolution.SourceKind(),
			resolution.TargetKind(),
		) {
			required := diagnosticReference(reference.ReferenceKey())
			basis, _ := NewMissingRuntimeBasis(MissingRuntimeSnapshot, required)
			accumulator.addUnderdeterminedWithRequired(
				DiagnosticReferenceUnresolved,
				"the required context bridge is active but the snapshot has not resolved through it",
				path+".reference",
				"reload-the-snapshot-through-the-active-context-bridge",
				required,
				basis,
			)
			return ReferenceFiller{}, nil, false
		}
		required := diagnosticSet([]string{
			resolution.SourceContext().String(),
			resolution.Context().String(),
			resolution.SourceKind().String(),
			resolution.TargetKind().String(),
		})
		basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticContextBridgeMissing,
			"cross-context reference requires an exact active TypeEnv bridge",
			path+".reference",
			resolution.Repair().String(),
			required,
			basis,
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticReferenceUnresolved,
			"reference lookup returned no recognized basis",
			path+".reference",
			"inspect-the-exact-reference",
		)
	}
	return ReferenceFiller{}, nil, false
}

func memberOfEvaluationViewForResolution(
	typeEnv TypeEnvRef,
	graphRevision GraphRevision,
	resolution AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	candidatePrefix OrderedCandidatePrefix,
) (MemberOfEvaluationView, error) {
	switch value := resolution.(type) {
	case sameBatchDeclarationResolution:
		return NewProspectiveBatchView(ProspectiveBatchViewInput{
			TypeEnv:                  typeEnv,
			PreStateGraphRevision:    graphRevision,
			EvaluationChangeOrdinal:  evaluationChangeOrdinal,
			DeclarationChangeOrdinal: value.DeclarationChangeOrdinal(),
			Declaration:              value.Declaration(),
			LocalReference:           value.LocalReference(),
			PersistedReference:       value.PersistedReference(),
			OrderedCandidatePrefix:   candidatePrefix,
		})
	case snapshotReferenceResolution:
		return NewPersistedSnapshotView(typeEnv, graphRevision)
	default:
		return nil, fmt.Errorf("unsupported admission reference resolution %T", resolution)
	}
}

func validateDisjointMembershipConstraints(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	entity EntityID,
	valueKind ValueKindRef,
	required MemberOfMember,
	contextSlice ContextSlice,
	evaluationView MemberOfEvaluationView,
	path string,
) ([]DisjointCounterUse, bool) {
	start := len(accumulator.diagnostics)
	uses := make([]DisjointCounterUse, 0)
	for _, rule := range environment.Constraints() {
		constraint, ok := rule.(KindDisjointConstraint)
		if !ok {
			continue
		}
		matchedKinds := disjointConstraintMatches(environment, constraint, valueKind.ID())
		if len(matchedKinds) > 1 {
			accumulator.addInvalidWithWitness(
				DiagnosticEntityKindMismatch,
				fmt.Sprintf(
					"ValueKind %s is a subkind of multiple mutually disjoint kinds",
					valueKind.String(),
				),
				path+".value_kind",
				declarationGoverningBasis(constraint.Provenance()),
				diagnosticState("at most one disjoint kind"),
				diagnosticKindIDs(matchedKinds),
			)
			continue
		}
		for _, matchedKind := range matchedKinds {
			for _, disjointKind := range constraint.Kinds() {
				if disjointKind == matchedKind {
					continue
				}
				disjointRef, err := NewValueKindRef(environment.Ref(), disjointKind)
				if err != nil {
					required := diagnosticReference(disjointKind.String())
					basis, _ := NewMissingRuntimeBasis(MissingRuntimeDeclaration, required)
					accumulator.addUnderdeterminedWithRequired(
						DiagnosticTypeRuleUnavailable,
						"disjoint constraint contains an unusable ValueKindRef",
						path+".value_kind",
						"recompile-the-active-typeenv",
						required,
						basis,
					)
					continue
				}
				use, valid := validateDisjointCounterMembership(
					accumulator,
					environment,
					snapshot,
					constraint,
					matchedKind,
					required,
					entity,
					disjointRef,
					contextSlice,
					evaluationView,
					path,
				)
				if valid {
					uses = append(uses, use)
				}
			}
		}
	}
	return uses, len(accumulator.diagnostics) == start
}

func validateStaticKindDisjointness(
	accumulator *validationAccumulator,
	environment TypeEnv,
	valueKind ValueKindRef,
	path string,
) bool {
	start := len(accumulator.diagnostics)
	for _, rule := range environment.Constraints() {
		constraint, ok := rule.(KindDisjointConstraint)
		if !ok {
			continue
		}
		matchedKinds := disjointConstraintMatches(environment, constraint, valueKind.ID())
		if len(matchedKinds) <= 1 {
			continue
		}
		accumulator.addInvalidWithWitness(
			DiagnosticEntityKindMismatch,
			fmt.Sprintf(
				"ValueKind %s is a subkind of multiple mutually disjoint kinds",
				valueKind.String(),
			),
			path,
			declarationGoverningBasis(constraint.Provenance()),
			diagnosticState("at most one disjoint kind"),
			diagnosticKindIDs(matchedKinds),
		)
	}
	return len(accumulator.diagnostics) == start
}

func disjointConstraintMatches(
	environment TypeEnv,
	constraint KindDisjointConstraint,
	valueKind KindID,
) []KindID {
	matches := make([]KindID, 0, 1)
	for _, constrainedKind := range constraint.Kinds() {
		if environment.IsSubkind(valueKind, constrainedKind) {
			matches = append(matches, constrainedKind)
		}
	}
	return matches
}

func validateDisjointCounterMembership(
	accumulator *validationAccumulator,
	environment TypeEnv,
	snapshot MemorySnapshot,
	constraint KindDisjointConstraint,
	matchedOperand KindID,
	supportingMembership MemberOfMember,
	entity EntityID,
	disjointKind ValueKindRef,
	contextSlice ContextSlice,
	evaluationView MemberOfEvaluationView,
	path string,
) (DisjointCounterUse, bool) {
	query, err := NewMemberOfQuery(entity, disjointKind, contextSlice)
	if err != nil {
		required := diagnosticSet([]string{
			entity.String(),
			disjointKind.String(),
			contextSlice.Ref().String(),
		})
		basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
		accumulator.addUnderdeterminedWithRequired(
			DiagnosticTypeRuleUnavailable,
			"exact disjoint-kind MemberOf query could not be constructed",
			path+".value_kind",
			"rebuild-the-complete-context-slice",
			required,
			basis,
		)
		return nil, false
	}
	request, err := NewMemberOfEvaluationRequest(query, evaluationView)
	if err != nil {
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"exact disjoint MemberOf evaluation request could not be constructed: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-typed-memory-validator",
		)
		return nil, false
	}
	proofSnapshot, supportsEntailments := snapshot.(DisjointEntailmentSnapshot)
	if supportsEntailments {
		proof, found := proofSnapshot.ResolveDisjointEntailment(
			request,
			constraint.ID(),
			supportingMembership,
		)
		if found {
			return validateResolvedDisjointEntailment(
				accumulator,
				environment,
				constraint,
				matchedOperand,
				supportingMembership,
				disjointKind.ID(),
				request,
				proof,
				path,
			)
		}
	}
	judgement := snapshot.EvaluateMemberOf(request)
	if !validateMemberOfCorrelation(accumulator, request, judgement, path+".value_kind") {
		return nil, false
	}
	switch value := judgement.(type) {
	case MemberOfNotMember:
		use, err := NewDirectNotMemberUse(constraint.ID(), value)
		if err == nil {
			return use, true
		}
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"validator could not seal exact disjoint membership evidence: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-typed-memory-validator",
		)
	case MemberOfMember:
		accumulator.addInvalidWithWitness(
			DiagnosticEntityKindMismatch,
			fmt.Sprintf("referent is admitted to disjoint kind %s", disjointKind.String()),
			path+".value_kind",
			snapshotRuleGoverningBasis(value.Basis().Evaluator()),
			diagnosticState(NotMemberJudgement.String()),
			diagnosticState(MemberJudgement.String()),
		)
	case MemberOfUndefined:
		use, err := NewDisjointEntailmentUse(DisjointEntailmentUseInput{
			TypeEnv:              environment,
			Constraint:           constraint,
			SupportingMembership: supportingMembership,
			MatchedOperand:       matchedOperand,
			ExcludedOperand:      disjointKind.ID(),
		})
		if err == nil {
			return use, true
		}
		accumulator.addMissingResolution(
			DiagnosticTypeRuleUnavailable,
			"validator could not seal exact disjoint entailment evidence: "+err.Error(),
			path+".value_kind",
			"inspect-and-repair-typed-memory-validator",
		)
		return nil, false
	default:
		// validateMemberOfCorrelation rejects invalid or unknown variants.
	}
	return nil, false
}

func validateResolvedDisjointEntailment(
	accumulator *validationAccumulator,
	environment TypeEnv,
	constraint KindDisjointConstraint,
	matchedOperand KindID,
	supportingMembership MemberOfMember,
	excludedOperand KindID,
	request MemberOfEvaluationRequest,
	proof DisjointEntailmentUse,
	path string,
) (DisjointCounterUse, bool) {
	rebuilt, err := NewDisjointEntailmentUse(DisjointEntailmentUseInput{
		TypeEnv:              environment,
		Constraint:           constraint,
		SupportingMembership: supportingMembership,
		MatchedOperand:       matchedOperand,
		ExcludedOperand:      excludedOperand,
	})
	valid := err == nil &&
		proof != nil &&
		proof.Constraint() == constraint.ID() &&
		proof.ConstraintDigest() == rebuilt.ConstraintDigest() &&
		bytes.Equal(proof.ConstraintRule().CanonicalBytes(), rebuilt.ConstraintRule().CanonicalBytes()) &&
		proof.SupportingMembership().Digest() == supportingMembership.Digest() &&
		bytes.Equal(proof.SupportingMembership().CanonicalBytes(), supportingMembership.CanonicalBytes()) &&
		proof.CounterRequest().Digest() == request.Digest() &&
		bytes.Equal(proof.CounterRequest().CanonicalBytes(), request.CanonicalBytes()) &&
		proof.MatchedOperand() == matchedOperand &&
		proof.ExcludedOperand() == excludedOperand &&
		proof.Digest() == rebuilt.Digest() &&
		bytes.Equal(proof.CanonicalBytes(), rebuilt.CanonicalBytes())
	if valid {
		return rebuilt, true
	}
	message := "transaction snapshot returned a malformed or stale KindDisjoint entailment"
	if err != nil {
		message += ": " + err.Error()
	}
	accumulator.addMissingResolution(
		DiagnosticTypeRuleUnavailable,
		message,
		path+".value_kind",
		"revalidate-the-kind-disjoint-entailment",
	)
	return nil, false
}

func validateMemberOfCorrelation(
	accumulator *validationAccumulator,
	request MemberOfEvaluationRequest,
	judgement MemberOfJudgement,
	path string,
) bool {
	mismatches := MemberOfJudgementMismatchesRequest(request, judgement)
	if len(mismatches) == 0 {
		return true
	}
	if memberOfDefinedJudgement(judgement) {
		accumulator.addInvalidWithWitness(
			DiagnosticTypeRuleUnavailable,
			"MemberOf evaluator returned a defined judgement with an uncorrelated or corrupt basis",
			path,
			coreValidatorGoverningBasis(),
			diagnosticReference(request.Digest().String()),
			diagnosticSet(memberOfMismatchValues(mismatches)),
		)
		return false
	}
	required := diagnosticSet(memberOfMismatchValues(mismatches))
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
	accumulator.addUnderdeterminedWithRequired(
		DiagnosticTypeRuleUnavailable,
		"MemberOf evaluator returned a judgement that is not exactly correlated with the query",
		path,
		"rebuild-or-inspect-the-memberof-evaluation-index",
		required,
		basis,
	)
	return false
}

func memberOfDefinedJudgement(judgement MemberOfJudgement) bool {
	switch judgement.(type) {
	case MemberOfMember, MemberOfNotMember:
		return true
	default:
		return false
	}
}

func memberOfMismatchValues(mismatches []MemberOfMismatch) []string {
	values := make([]string, 0, len(mismatches))
	for _, mismatch := range mismatches {
		value := mismatch.Kind().String()
		if mismatch.Expected() != "" {
			value += ":expected=" + mismatch.Expected()
		}
		if mismatch.Actual() != "" {
			value += ":actual=" + mismatch.Actual()
		}
		values = append(values, value)
	}
	return values
}

func addUndefinedMemberOfDiagnostic(
	accumulator *validationAccumulator,
	message string,
	path string,
	judgement MemberOfUndefined,
) {
	required := diagnosticSet(memberOfMissingBasisValues(judgement.MissingBasis()))
	basis, _ := NewMissingRuntimeBasis(MissingRuntimeResolution, required)
	accumulator.addUnderdeterminedWithRequired(
		DiagnosticTypeRuleUnavailable,
		message,
		path,
		judgement.Repair().String(),
		required,
		basis,
	)
}

func memberOfMissingBasisValues(values []MemberOfMissingBasis) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Kind().String()+":"+value.Subject())
	}
	return result
}

func sameStrongReference(left, right StrongRef) bool {
	return validStrongRef(left) &&
		validStrongRef(right) &&
		left.RefKind() == right.RefKind() &&
		left.ReferenceKey() == right.ReferenceKey()
}

func validateNewAssertion(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	assertion AssertionID,
	changeOrdinal uint64,
	path string,
) {
	switch state := snapshot.AssertionState(assertion).(type) {
	case AbsentAssertionState:
		if state.Assertion() != assertion || !state.Basis().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated absence token",
				path,
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		observation, err := NewAssertionAbsentObservation(changeOrdinal, state)
		accumulator.captureObservation(observation, err, path)
		return
	case ActiveAssertion:
		if state.Assertion() != assertion {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated active token",
				path,
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticAssertionAlreadyExists,
			fmt.Sprintf("assertion %s is already active", assertion.String()),
			path,
			coreValidatorGoverningBasis(),
			diagnosticState("assertion ID absent"),
			diagnosticReference(assertion.String()),
		)
	case RetractedAssertionState:
		if state.Assertion() != assertion || !state.Rule().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated retraction token",
				path,
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticAssertionAlreadyRetracted,
			fmt.Sprintf("assertion ID %s belongs to retained retraction history", assertion.String()),
			path,
			snapshotRuleGoverningBasis(state.Rule()),
			diagnosticState("assertion ID absent"),
			diagnosticState("retained retraction history"),
		)
	case UnknownAssertionState:
		if state.Assertion() != assertion || !state.Repair().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated unknown token",
				path,
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticAssertionNotFound,
			"assertion identity absence is not established",
			path,
			state.Repair().String(),
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticAssertionNotFound,
			"assertion lookup returned no recognized basis",
			path,
			"inspect-assertion-identity",
		)
	}
}

func validateRetraction(
	accumulator *validationAccumulator,
	snapshot MemorySnapshot,
	change RetractAssertion,
	changeOrdinal uint64,
	path string,
) {
	switch state := snapshot.AssertionState(change.assertion).(type) {
	case ActiveAssertion:
		if state.Assertion() != change.assertion {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated active token",
				path+".assertion_id",
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		observation, err := NewAssertionActiveObservation(changeOrdinal, state)
		if !accumulator.captureObservation(observation, err, path+".assertion_id") {
			return
		}
		accumulator.validated = append(accumulator.validated, ValidatedRetraction{change: change})
	case RetractedAssertionState:
		if state.Assertion() != change.assertion || !state.Rule().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated retraction token",
				path+".assertion_id",
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addInvalidWithWitness(
			DiagnosticAssertionAlreadyRetracted,
			"assertion is already retracted",
			path+".assertion_id",
			snapshotRuleGoverningBasis(state.Rule()),
			diagnosticState("assertion active"),
			diagnosticState("assertion retracted"),
		)
	case AbsentAssertionState:
		if state.Assertion() != change.assertion || !state.Basis().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated absence token",
				path+".assertion_id",
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticAssertionNotFound,
			"assertion is known absent from this snapshot",
			path+".assertion_id",
			"inspect-legacy-or-historical-assertion-source",
		)
	case UnknownAssertionState:
		if state.Assertion() != change.assertion || !state.Repair().valid() {
			accumulator.addMissingResolution(
				DiagnosticAssertionNotFound,
				"assertion lookup returned an uncorrelated unknown token",
				path+".assertion_id",
				"rebuild-or-inspect-assertion-index",
			)
			return
		}
		accumulator.addMissingResolution(
			DiagnosticAssertionNotFound,
			"assertion lookup is underdetermined",
			path+".assertion_id",
			state.Repair().String(),
		)
	default:
		accumulator.addMissingResolution(
			DiagnosticAssertionNotFound,
			"assertion lookup returned no recognized basis",
			path+".assertion_id",
			"inspect-assertion-index",
		)
	}
}

func validateSlotGroupConstraints(
	accumulator *validationAccumulator,
	environment TypeEnv,
	relation candidateRelationalCarrier,
	bindings map[string]CandidateSlotBinding,
	path string,
) {
	for _, rule := range environment.Constraints() {
		constraint, ok := rule.(SlotGroupConstraint)
		if !ok || constraint.RelationDeclarationFragmentRef() != relation.RelationDeclarationFragmentRef() {
			continue
		}
		present := 0
		for _, slot := range constraint.Slots() {
			if binding, exists := bindings[slot.String()]; exists && len(binding.Fillers()) > 0 {
				present++
			}
		}
		valid := slotGroupCountValid(constraint.Mode(), present, len(constraint.Slots()))
		if !valid {
			accumulator.addInvalidWithWitness(
				DiagnosticCardinalityMismatch,
				fmt.Sprintf("slot-group constraint %s is not satisfied", constraint.ID().String()),
				path+".slots",
				declarationGoverningBasis(constraint.Provenance()),
				diagnosticSlotGroup(constraint.Mode(), len(constraint.Slots())),
				NewDiagnosticCountDatum(uint64(present)),
			)
		}
	}
}

func slotGroupCountValid(mode SlotGroupMode, present, total int) bool {
	switch mode {
	case SlotGroupAllOrNone:
		return present == 0 || present == total
	case SlotGroupAtLeastOne:
		return present >= 1
	case SlotGroupExactlyOne:
		return present == 1
	default:
		return false
	}
}

func contextActive(
	accumulator *validationAccumulator,
	environment TypeEnv,
	context BoundedContextRef,
	path string,
) bool {
	if _, found := environment.BoundedContext(context); found {
		return true
	}
	required := diagnosticReference(context.String())
	basis, _ := NewMissingTypeEnvDeclarationBasis(environment.Ref(), required)
	accumulator.addUnderdeterminedWithRequired(
		DiagnosticContextNotActive,
		fmt.Sprintf("bounded context %s is absent from the active TypeEnv", context.String()),
		path,
		"inspect-or-stage-the-required-bounded-context",
		required,
		basis,
	)
	return false
}

func fragmentContainsContext(
	fragment TypedRelationDeclarationFragment,
	context BoundedContextRef,
) bool {
	for _, candidate := range fragment.Contexts() {
		if candidate == context {
			return true
		}
	}
	return false
}

func diagnosticFragmentContexts(fragment TypedRelationDeclarationFragment) DiagnosticDatum {
	values := make([]string, 0, len(fragment.Contexts()))
	for _, context := range fragment.Contexts() {
		values = append(values, context.String())
	}
	return diagnosticSet(values)
}

func diagnosticSlotNames(fragment TypedRelationDeclarationFragment) DiagnosticDatum {
	values := make([]string, 0, len(fragment.Slots()))
	for _, slot := range fragment.Slots() {
		values = append(values, slot.SlotKind().String())
	}
	return diagnosticSet(values)
}

func diagnosticCardinality(cardinality Cardinality) DiagnosticDatum {
	maximum, bounded := cardinality.Maximum().BoundedValue()
	if bounded {
		return diagnosticText(fmt.Sprintf("minimum=%d,maximum=%d", cardinality.Minimum(), maximum))
	}
	return diagnosticText(fmt.Sprintf("minimum=%d,maximum=unbounded", cardinality.Minimum()))
}

func diagnosticKindIDs(kinds []KindID) DiagnosticDatum {
	values := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, kind.String())
	}
	return diagnosticSet(values)
}

func diagnosticSlotGroup(mode SlotGroupMode, total int) DiagnosticDatum {
	return diagnosticText(fmt.Sprintf("mode=%s,total=%d", mode.String(), total))
}

type localDeclaration struct {
	change  DeclareEntity
	ordinal uint64
}

func localDeclarations(changeSet MemoryChangeSet) map[string]localDeclaration {
	locals := make(map[string]localDeclaration)
	for ordinal, change := range changeSet.changes {
		declaration, ok := change.(DeclareEntity)
		if !ok {
			continue
		}
		locals["local:"+declaration.localRef.String()] = localDeclaration{
			change:  declaration,
			ordinal: uint64(ordinal),
		}
	}
	return locals
}

func memorySnapshotPresent(snapshot MemorySnapshot) bool {
	if snapshot == nil {
		return false
	}
	value := reflect.ValueOf(snapshot)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}
