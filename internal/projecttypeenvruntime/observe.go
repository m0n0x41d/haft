package projecttypeenvruntime

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

const targetRuntimeRegistryDigestDomain = "haft.projecttypeenvruntime.target-runtime-registry-observation.v2"

// InstalledRuntimeRegistryInput is the current process-owned runtime surface
// observed by the adapter. Mechanism catalogs and registration policies are
// declarations configured beside callable registries; they do not attest
// executable bytes.
type InstalledRuntimeRegistryInput struct {
	Codecs                                   typedmemory.CodecRegistry
	EntitySetEnumerationEvaluators           EntitySetEnumerationEvaluatorRegistry
	CandidateVisibilityEvaluators            CandidateVisibilityEvaluatorRegistry
	KindDefinednessEvaluators                KindDefinednessEvaluatorRegistry
	MemberOfEvaluators                       MemberOfEvaluatorRegistry
	KindClassificationEvaluators             KindClassificationEvaluatorRegistry
	ReferenceDesignationResolutionEvaluators ReferenceDesignationResolutionEvaluatorRegistry
	ClaimInterpretationEvaluators            ClaimInterpretationEvaluatorRegistry
	ClaimMeasurementEvaluators               ClaimMeasurementEvaluatorRegistry
	ClaimEvaluationEvaluators                ClaimEvaluationEvaluatorRegistry
	EpistemeConstitutionEvaluators           EpistemeConstitutionEvaluatorRegistry
	MechanismCatalogs                        []runtimemechanism.RuntimeMechanismArtifactV1
	RegistrationPolicies                     []recordmembershipregistration.RegistrationArtifactV1
}

// ObservationInput keeps target X separate from the installed process
// registries. A later Stage revalidator must separately require that this X is
// the exact runtime basis bound into the executable target C.
type ObservationInput struct {
	RuntimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact
	Installed    InstalledRuntimeRegistryInput
}

type mechanismIdentityKey struct {
	artifact string
	edition  string
	digest   string
}

type mechanismCoordinateKey struct {
	artifact string
	edition  string
}

type installedRuntimeIndex struct {
	catalogsByIdentity   map[mechanismIdentityKey][]runtimemechanism.RuntimeMechanismArtifactV1
	catalogsByCoordinate map[mechanismCoordinateKey][]runtimemechanism.RuntimeMechanismArtifactV1
	policiesByRef        map[string][]recordmembershipregistration.RegistrationArtifactV1
	policiesByEvaluator  map[string][]recordmembershipregistration.RegistrationArtifactV1
	allPolicies          []recordmembershipregistration.RegistrationArtifactV1
}

type runtimeComparison struct {
	issues          []Issue
	matchedCatalogs []runtimemechanism.RuntimeMechanismArtifactV1
	matchedPolicies TargetRegistrationPolicies
}

// ObserveCurrentTargetRuntime compares all exact target-X pins with the
// supplied process-owned registries. It never substitutes a default codec,
// evaluator, mechanism catalog, or registration policy.
func ObserveCurrentTargetRuntime(input ObservationInput) Resolution {
	if err := input.RuntimeBasis.VerifyResolvedClosure(); err != nil {
		issue := newIssue(
			IssueRuntimeBasisInvalid,
			"target runtime basis X",
			"one exact resolved RuntimeEvaluationBasisArtifact",
			err.Error(),
			"reload and verify the exact X closure",
		)
		return invalid{issues: []Issue{issue}}
	}
	index, invalidIssues := indexInstalledRuntime(input.Installed)
	if len(invalidIssues) > 0 {
		return invalid{issues: normalizeIssues(invalidIssues)}
	}
	comparison := compareInstalledRuntime(
		input.RuntimeBasis,
		input.Installed,
		index,
	)
	if len(comparison.issues) > 0 {
		return resolutionFromIssues(comparison.issues)
	}
	evaluators, err := selectExactTargetEvaluatorRegistries(
		input.RuntimeBasis,
		input.Installed,
	)
	if err != nil {
		issue := newIssue(
			IssueRuntimeBasisInvalid,
			"target evaluator registry refinement",
			"only exact evaluator registrations pinned by X",
			err.Error(),
			"rebuild the exact target evaluator registry from the matched X pins",
		)
		return invalid{issues: []Issue{issue}}
	}
	digest, err := deriveTargetRuntimeRegistryDigest(
		input.RuntimeBasis,
		input.Installed,
		comparison.matchedCatalogs,
		comparison.matchedPolicies,
	)
	if err != nil {
		issue := newIssue(
			IssueRuntimeBasisInvalid,
			"target runtime registry digest",
			"a canonical exact-coordinate digest",
			err.Error(),
			"repair the target runtime coordinate set",
		)
		return invalid{issues: []Issue{issue}}
	}
	state := exactTargetRuntimeRegistryState{
		runtimeBasis:                   input.RuntimeBasis,
		codecs:                         input.Installed.Codecs,
		entitySetEnumeration:           evaluators.entitySetEnumeration,
		candidateVisibility:            evaluators.candidateVisibility,
		kindDefinedness:                evaluators.kindDefinedness,
		memberOf:                       evaluators.memberOf,
		kindClassification:             evaluators.kindClassification,
		referenceDesignationResolution: evaluators.referenceDesignationResolution,
		claimInterpretation:            evaluators.claimInterpretation,
		claimMeasurement:               evaluators.claimMeasurement,
		claimEvaluation:                evaluators.claimEvaluation,
		epistemeConstitution:           evaluators.epistemeConstitution,
		mechanismCatalogs:              cloneMechanismCatalogs(comparison.matchedCatalogs),
		policies:                       cloneTargetRegistrationPolicies(comparison.matchedPolicies),
		digest:                         digest,
	}
	registry := ExactTargetRuntimeRegistry{
		state:      &state,
		capability: &exactTargetRuntimeRegistryCapability{},
	}
	if err := verifyExactTargetRuntimeRegistryState(state); err != nil {
		issue := newIssue(
			IssueRuntimeBasisInvalid,
			"matched target runtime registry",
			"a self-consistent exact in-process observation",
			err.Error(),
			"rebuild the process runtime registry from exact package registrations",
		)
		return invalid{issues: []Issue{issue}}
	}
	return matched{registry: registry}
}

type exactTargetEvaluatorRegistries struct {
	entitySetEnumeration           EntitySetEnumerationEvaluatorRegistry
	candidateVisibility            CandidateVisibilityEvaluatorRegistry
	kindDefinedness                KindDefinednessEvaluatorRegistry
	memberOf                       MemberOfEvaluatorRegistry
	kindClassification             KindClassificationEvaluatorRegistry
	referenceDesignationResolution ReferenceDesignationResolutionEvaluatorRegistry
	claimInterpretation            ClaimInterpretationEvaluatorRegistry
	claimMeasurement               ClaimMeasurementEvaluatorRegistry
	claimEvaluation                ClaimEvaluationEvaluatorRegistry
	epistemeConstitution           EpistemeConstitutionEvaluatorRegistry
}

func selectExactTargetEvaluatorRegistries(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed InstalledRuntimeRegistryInput,
) (exactTargetEvaluatorRegistries, error) {
	entitySetEnumeration, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
		installed.EntitySetEnumerationEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	candidateVisibility, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility,
		installed.CandidateVisibilityEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	kindDefinedness, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractKindDefinedness,
		installed.KindDefinednessEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	memberOf, err := selectExactMemberOfEvaluatorRegistry(
		runtimeBasis,
		installed.MemberOfEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	kindClassification, err := selectExactKindClassificationEvaluatorRegistry(
		runtimeBasis,
		installed.KindClassificationEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	referenceDesignationResolution, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution,
		installed.ReferenceDesignationResolutionEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	claimInterpretation, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation,
		installed.ClaimInterpretationEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	claimMeasurement, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement,
		installed.ClaimMeasurementEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	claimEvaluation, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation,
		installed.ClaimEvaluationEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	epistemeConstitution, err := selectExactTargetEvaluatorRegistry(
		runtimeBasis,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation,
		installed.EpistemeConstitutionEvaluators,
	)
	if err != nil {
		return exactTargetEvaluatorRegistries{}, err
	}
	return exactTargetEvaluatorRegistries{
		entitySetEnumeration:           entitySetEnumeration,
		candidateVisibility:            candidateVisibility,
		kindDefinedness:                kindDefinedness,
		memberOf:                       memberOf,
		kindClassification:             kindClassification,
		referenceDesignationResolution: referenceDesignationResolution,
		claimInterpretation:            claimInterpretation,
		claimMeasurement:               claimMeasurement,
		claimEvaluation:                claimEvaluation,
		epistemeConstitution:           epistemeConstitution,
	}, nil
}

func selectExactMemberOfEvaluatorRegistry(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed MemberOfEvaluatorRegistry,
) (MemberOfEvaluatorRegistry, error) {
	registrations := make([]memberofruntime.Registration, 0)
	for _, runtimePin := range runtimeBasis.Pins() {
		pin, evaluatorPin := runtimePin.(projecttypeenv.EvaluatorRuntimeMechanismPin)
		if !evaluatorPin ||
			pin.InvocationContract() != projecttypeenv.RuntimeMechanismContractMemberOf {
			continue
		}
		expected, err := evaluatorMechanismIdentity(pin)
		if err != nil {
			return MemberOfEvaluatorRegistry{}, err
		}
		lookup, err := installed.Lookup(pin.Rule(), expected)
		if err != nil {
			return MemberOfEvaluatorRegistry{}, err
		}
		found, exact := lookup.(memberofruntime.Found)
		if !exact {
			return MemberOfEvaluatorRegistry{}, fmt.Errorf(
				"exact evaluator %s is absent after runtime comparison",
				formatEvaluatorIdentity(pin.Rule(), expected),
			)
		}
		registrations = append(registrations, found.Registration())
	}
	return memberofruntime.NewRegistry(registrations)
}

func selectExactKindClassificationEvaluatorRegistry(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed KindClassificationEvaluatorRegistry,
) (KindClassificationEvaluatorRegistry, error) {
	registrations := make([]kindclassificationruntime.Registration, 0)
	for _, runtimePin := range runtimeBasis.Pins() {
		pin, evaluatorPin := runtimePin.(projecttypeenv.EvaluatorRuntimeMechanismPin)
		if !evaluatorPin ||
			pin.InvocationContract() != projecttypeenv.RuntimeMechanismContractKindClassification {
			continue
		}
		expected, err := evaluatorMechanismIdentity(pin)
		if err != nil {
			return KindClassificationEvaluatorRegistry{}, err
		}
		registration, found := installed.Registration(pin.Rule())
		if !found || registration.Identity() != expected {
			return KindClassificationEvaluatorRegistry{}, fmt.Errorf(
				"exact evaluator %s is absent after runtime comparison",
				formatEvaluatorIdentity(pin.Rule(), expected),
			)
		}
		registrations = append(registrations, registration)
	}
	return kindclassificationruntime.NewRegistry(registrations)
}

func selectExactTargetEvaluatorRegistry[Input, Output any](
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	installed typedmemoryevaluation.Registry[Input, Output],
) (typedmemoryevaluation.Registry[Input, Output], error) {
	registrations := make([]typedmemoryevaluation.Registration[Input, Output], 0)
	for _, runtimePin := range runtimeBasis.Pins() {
		pin, evaluatorPin := runtimePin.(projecttypeenv.EvaluatorRuntimeMechanismPin)
		if !evaluatorPin || pin.InvocationContract() != contract {
			continue
		}
		expected, err := evaluatorMechanismIdentity(pin)
		if err != nil {
			return typedmemoryevaluation.Registry[Input, Output]{}, err
		}
		lookup, err := installed.Lookup(pin.Rule(), expected)
		if err != nil {
			return typedmemoryevaluation.Registry[Input, Output]{}, err
		}
		found, exact := lookup.(typedmemoryevaluation.Found[Input, Output])
		if !exact {
			return typedmemoryevaluation.Registry[Input, Output]{}, fmt.Errorf(
				"exact evaluator %s is absent after runtime comparison",
				formatEvaluatorIdentity(pin.Rule(), expected),
			)
		}
		registrations = append(registrations, found.Registration())
	}
	return typedmemoryevaluation.NewRegistry(registrations)
}

func indexInstalledRuntime(
	installed InstalledRuntimeRegistryInput,
) (installedRuntimeIndex, []Issue) {
	index := installedRuntimeIndex{
		catalogsByIdentity:   make(map[mechanismIdentityKey][]runtimemechanism.RuntimeMechanismArtifactV1),
		catalogsByCoordinate: make(map[mechanismCoordinateKey][]runtimemechanism.RuntimeMechanismArtifactV1),
		policiesByRef:        make(map[string][]recordmembershipregistration.RegistrationArtifactV1),
		policiesByEvaluator:  make(map[string][]recordmembershipregistration.RegistrationArtifactV1),
		allPolicies:          append([]recordmembershipregistration.RegistrationArtifactV1(nil), installed.RegistrationPolicies...),
	}
	issues := make([]Issue, 0)
	for position, catalog := range installed.MechanismCatalogs {
		if err := catalog.Verify(); err != nil {
			issues = append(issues, newIssue(
				IssueMechanismCatalogInvalid,
				fmt.Sprintf("installed mechanism catalog[%d]", position),
				"an exact verified RuntimeMechanismArtifactV1",
				err.Error(),
				"repair or remove the malformed installed catalog",
			))
			continue
		}
		identity := catalog.Identity()
		identityKey := mechanismIdentityKey{
			artifact: identity.Artifact().String(),
			edition:  identity.Edition().String(),
			digest:   identity.Digest().String(),
		}
		coordinateKey := mechanismCoordinateKey{
			artifact: identityKey.artifact,
			edition:  identityKey.edition,
		}
		index.catalogsByIdentity[identityKey] = append(
			index.catalogsByIdentity[identityKey],
			catalog,
		)
		index.catalogsByCoordinate[coordinateKey] = append(
			index.catalogsByCoordinate[coordinateKey],
			catalog,
		)
	}
	for position, policy := range installed.RegistrationPolicies {
		if err := policy.Verify(); err != nil {
			issues = append(issues, newIssue(
				IssueRegistrationPolicyInvalid,
				fmt.Sprintf("installed registration policy[%d]", position),
				"an exact verified RegistrationArtifactV1",
				err.Error(),
				"repair or remove the malformed installed registration policy",
			))
			continue
		}
		ref := policy.Ref().String()
		index.policiesByRef[ref] = append(index.policiesByRef[ref], policy)
		rule := policy.Evaluator().Rule().String()
		index.policiesByEvaluator[rule] = append(
			index.policiesByEvaluator[rule],
			policy,
		)
	}
	return index, normalizeIssues(issues)
}

func compareInstalledRuntime(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed InstalledRuntimeRegistryInput,
	index installedRuntimeIndex,
) runtimeComparison {
	comparison := runtimeComparison{
		issues:          make([]Issue, 0),
		matchedPolicies: NoTargetRegistrationPolicies{},
	}
	mechanismPins := runtimeBasis.Pins()
	matchedCatalogs := make(map[mechanismIdentityKey]runtimemechanism.RuntimeMechanismArtifactV1)
	for _, pin := range mechanismPins {
		catalog, catalogIssues := matchMechanismCatalog(pin, index)
		comparison.issues = append(comparison.issues, catalogIssues...)
		if len(catalogIssues) == 0 {
			key := mechanismKeyFromPin(mechanismForRuntimePin(pin))
			matchedCatalogs[key] = catalog
		}
		comparison.issues = append(
			comparison.issues,
			compareCallablePresence(pin, installed)...,
		)
	}
	policies, policyIssues := matchRegistrationPolicies(runtimeBasis, index)
	comparison.issues = append(comparison.issues, policyIssues...)
	comparison.matchedPolicies = policies
	comparison.issues = append(
		comparison.issues,
		compareRegistrationPolicyCoordinates(runtimeBasis, policies)...,
	)
	comparison.matchedCatalogs = sortedMechanismCatalogs(matchedCatalogs)
	comparison.issues = normalizeIssues(comparison.issues)
	return comparison
}

func matchMechanismCatalog(
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
	index installedRuntimeIndex,
) (runtimemechanism.RuntimeMechanismArtifactV1, []Issue) {
	identityKey := mechanismKeyFromPin(mechanismForRuntimePin(pin))
	exact := index.catalogsByIdentity[identityKey]
	subject := runtimePinSubject(pin)
	if len(exact) > 1 {
		issue := newIssue(
			IssueMechanismCatalogDuplicate,
			subject,
			formatMechanismIdentity(identityKey),
			fmt.Sprintf("%d exact installed catalogs", len(exact)),
			"retain exactly one installed catalog for the target mechanism identity",
		)
		return runtimemechanism.RuntimeMechanismArtifactV1{}, []Issue{issue}
	}
	if len(exact) == 0 {
		coordinate := mechanismCoordinateKey{
			artifact: identityKey.artifact,
			edition:  identityKey.edition,
		}
		alternates := index.catalogsByCoordinate[coordinate]
		if len(alternates) > 0 {
			issue := newIssue(
				IssueMechanismCatalogDrift,
				subject,
				formatMechanismIdentity(identityKey),
				formatCatalogIdentities(alternates),
				"install the exact catalog digest pinned by X",
			)
			return runtimemechanism.RuntimeMechanismArtifactV1{}, []Issue{issue}
		}
		issue := newIssue(
			IssueMechanismCatalogMissing,
			subject,
			formatMechanismIdentity(identityKey),
			"absent",
			"install the exact mechanism catalog pinned by X",
		)
		return runtimemechanism.RuntimeMechanismArtifactV1{}, []Issue{issue}
	}
	catalog := exact[0]
	if !catalogContainsPin(catalog, pin) {
		issue := newIssue(
			IssueMechanismCatalogEntryDrift,
			subject,
			runtimePinSemanticCoordinate(pin),
			"exact catalog lacks the pinned role/contract/semantic tuple",
			"install the exact catalog entry selected by X",
		)
		return runtimemechanism.RuntimeMechanismArtifactV1{}, []Issue{issue}
	}
	coordinate := mechanismCoordinateKey{
		artifact: identityKey.artifact,
		edition:  identityKey.edition,
	}
	if len(index.catalogsByCoordinate[coordinate]) > 1 {
		issue := newIssue(
			IssueMechanismCatalogDuplicate,
			subject,
			"one digest at the installed artifact/edition coordinate",
			formatCatalogIdentities(index.catalogsByCoordinate[coordinate]),
			"remove the conflicting installed catalog coordinate",
		)
		return runtimemechanism.RuntimeMechanismArtifactV1{}, []Issue{issue}
	}
	return catalog, nil
}

func compareCallablePresence(
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
	installed InstalledRuntimeRegistryInput,
) []Issue {
	switch value := pin.(type) {
	case projecttypeenv.CodecRuntimeMechanismPin:
		if installed.Codecs.Contains(value.Codec()) {
			return nil
		}
		return []Issue{newIssue(
			IssueCodecImplementationMissing,
			runtimePinSubject(pin),
			value.Codec().String(),
			"absent",
			"register the exact codec implementation selected by X",
		)}
	case projecttypeenv.EvaluatorRuntimeMechanismPin:
		switch value.InvocationContract() {
		case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
			return compareEvaluatorRegistration(
				value,
				installed.EntitySetEnumerationEvaluators,
				"entity-set enumeration",
			)
		case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
			return compareEvaluatorRegistration(
				value,
				installed.CandidateVisibilityEvaluators,
				"candidate visibility",
			)
		case projecttypeenv.RuntimeMechanismContractKindDefinedness:
			return compareEvaluatorRegistration(
				value,
				installed.KindDefinednessEvaluators,
				"kind definedness",
			)
		case projecttypeenv.RuntimeMechanismContractMemberOf:
			return compareMemberOfEvaluatorRegistration(
				value,
				installed.MemberOfEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractKindClassification:
			return compareKindClassificationEvaluatorRegistration(
				value,
				installed.KindClassificationEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution:
			return compareEvaluatorRegistration(
				value,
				installed.ReferenceDesignationResolutionEvaluators,
				"reference designation resolution",
			)
		case projecttypeenv.RuntimeMechanismContractClaimInterpretation:
			return compareEvaluatorRegistration(
				value,
				installed.ClaimInterpretationEvaluators,
				"claim interpretation",
			)
		case projecttypeenv.RuntimeMechanismContractClaimMeasurement:
			return compareEvaluatorRegistration(
				value,
				installed.ClaimMeasurementEvaluators,
				"claim measurement",
			)
		case projecttypeenv.RuntimeMechanismContractClaimEvaluation:
			return compareEvaluatorRegistration(
				value,
				installed.ClaimEvaluationEvaluators,
				"claim evaluation",
			)
		case projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation:
			return compareEvaluatorRegistration(
				value,
				installed.EpistemeConstitutionEvaluators,
				"episteme constitution evaluation",
			)
		default:
			return []Issue{newIssue(
				IssueEvaluatorContractUnsupported,
				runtimePinSubject(pin),
				"a package-owned callable registry for "+value.InvocationContract().String(),
				"no typed registry adapter is implemented",
				"add a reviewed typed evaluator registry for this invocation contract",
			)}
		}
	case projecttypeenv.CarrierMembershipRuntimeMechanismPin:
		// The exact callable delivery boundary is introduced by the immutable
		// store adapter per source occurrence. Here X and the current policy
		// must bind its implementation coordinate; policy comparison below
		// establishes that configured identity.
		return nil
	default:
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			"target runtime pin",
			"one closed runtime-mechanism pin variant",
			fmt.Sprintf("%T", pin),
			"repair the target X pin algebra",
		)}
	}
}

func compareKindClassificationEvaluatorRegistration(
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry KindClassificationEvaluatorRegistry,
) []Issue {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			runtimePinSubject(pin),
			"a valid evaluator mechanism identity",
			err.Error(),
			"repair the target X evaluator pin",
		)}
	}
	registration, found := registry.Registration(pin.Rule())
	if !found {
		return []Issue{newIssue(
			IssueEvaluatorRegistrationMissing,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			"absent",
			"install the exact kind-classification evaluator registration",
		)}
	}
	if registration.Identity() != expected {
		return []Issue{newIssue(
			IssueEvaluatorIdentityDrift,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			formatEvaluatorIdentity(pin.Rule(), registration.Identity()),
			"replace the installed evaluator with the exact identity pinned by X",
		)}
	}
	return nil
}

func compareMemberOfEvaluatorRegistration(
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry MemberOfEvaluatorRegistry,
) []Issue {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			runtimePinSubject(pin),
			"a valid evaluator mechanism identity",
			err.Error(),
			"repair the target X evaluator pin",
		)}
	}
	lookup, err := registry.Lookup(pin.Rule(), expected)
	if err != nil {
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			runtimePinSubject(pin),
			"a valid exact evaluator lookup",
			err.Error(),
			"repair the target evaluator coordinate",
		)}
	}
	switch result := lookup.(type) {
	case memberofruntime.Found:
		return nil
	case memberofruntime.Missing:
		return []Issue{newIssue(
			IssueEvaluatorRegistrationMissing,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			"absent",
			"install the exact MemberOf evaluator registration",
		)}
	case memberofruntime.Mismatch:
		return []Issue{newIssue(
			IssueEvaluatorIdentityDrift,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), result.ExpectedIdentity()),
			formatEvaluatorIdentity(pin.Rule(), result.RegisteredIdentity()),
			"replace the installed evaluator with the exact identity pinned by X",
		)}
	default:
		return []Issue{newIssue(
			IssueEvaluatorRegistrationMissing,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			"unsupported lookup result",
			"rebuild the immutable MemberOf evaluator registry",
		)}
	}
}

func compareEvaluatorRegistration[Input, Output any](
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry typedmemoryevaluation.Registry[Input, Output],
	contractName string,
) []Issue {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			runtimePinSubject(pin),
			"a valid evaluator mechanism identity",
			err.Error(),
			"repair the target X evaluator pin",
		)}
	}
	lookup, err := registry.Lookup(pin.Rule(), expected)
	if err != nil {
		return []Issue{newIssue(
			IssueRuntimeBasisInvalid,
			runtimePinSubject(pin),
			"a valid exact evaluator lookup",
			err.Error(),
			"repair the target evaluator coordinate",
		)}
	}
	switch result := lookup.(type) {
	case typedmemoryevaluation.Found[Input, Output]:
		return nil
	case typedmemoryevaluation.Missing[Input, Output]:
		return []Issue{newIssue(
			IssueEvaluatorRegistrationMissing,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			"absent",
			"install the exact "+contractName+" evaluator registration",
		)}
	case typedmemoryevaluation.Mismatch[Input, Output]:
		return []Issue{newIssue(
			IssueEvaluatorIdentityDrift,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), result.ExpectedIdentity()),
			formatEvaluatorIdentity(pin.Rule(), result.RegisteredIdentity()),
			"replace the installed evaluator with the exact identity pinned by X",
		)}
	default:
		return []Issue{newIssue(
			IssueEvaluatorRegistrationMissing,
			runtimePinSubject(pin),
			formatEvaluatorIdentity(pin.Rule(), expected),
			"unsupported lookup result",
			"rebuild the immutable evaluator registry",
		)}
	}
}

func evaluatorMechanismIdentity(
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
) (typedmemoryevaluation.MechanismIdentity, error) {
	mechanism := pin.Mechanism()
	return typedmemoryevaluation.NewMechanismIdentity(
		mechanism.Artifact(),
		mechanism.Edition(),
		mechanism.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	)
}

func exactEvaluatorCallableCoordinates(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed InstalledRuntimeRegistryInput,
) ([]string, error) {
	coordinates := make([]string, 0)
	for _, runtimePin := range runtimeBasis.Pins() {
		pin, evaluatorPin := runtimePin.(projecttypeenv.EvaluatorRuntimeMechanismPin)
		if !evaluatorPin {
			continue
		}
		var coordinate string
		var err error
		switch pin.InvocationContract() {
		case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.EntitySetEnumerationEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.CandidateVisibilityEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractKindDefinedness:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.KindDefinednessEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractMemberOf:
			coordinate, err = exactMemberOfEvaluatorCallableCoordinate(
				pin,
				installed.MemberOfEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractKindClassification:
			coordinate, err = exactKindClassificationEvaluatorCallableCoordinate(
				pin,
				installed.KindClassificationEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.ReferenceDesignationResolutionEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractClaimInterpretation:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.ClaimInterpretationEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractClaimMeasurement:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.ClaimMeasurementEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractClaimEvaluation:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.ClaimEvaluationEvaluators,
			)
		case projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation:
			coordinate, err = exactEvaluatorCallableCoordinate(
				pin,
				installed.EpistemeConstitutionEvaluators,
			)
		default:
			err = fmt.Errorf(
				"evaluator contract %q has no package-owned callable registry",
				pin.InvocationContract().String(),
			)
		}
		if err != nil {
			return nil, err
		}
		coordinates = append(coordinates, coordinate)
	}
	sort.Strings(coordinates)
	return coordinates, nil
}

func exactKindClassificationEvaluatorCallableCoordinate(
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry KindClassificationEvaluatorRegistry,
) (string, error) {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return "", err
	}
	registration, found := registry.Registration(pin.Rule())
	if !found || registration.Identity() != expected {
		return "", fmt.Errorf(
			"evaluator registry does not contain exact callable %s",
			formatEvaluatorIdentity(pin.Rule(), expected),
		)
	}
	identity := registration.Identity()
	coordinate := []string{
		pin.InvocationContract().String(),
		registration.RuleRef().String(),
		identity.ArtifactRef().String(),
		identity.Edition().String(),
		identity.Digest().String(),
		identity.Role().String(),
	}
	return strings.Join(coordinate, "\x1f"), nil
}

func exactMemberOfEvaluatorCallableCoordinate(
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry MemberOfEvaluatorRegistry,
) (string, error) {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return "", err
	}
	lookup, err := registry.Lookup(pin.Rule(), expected)
	if err != nil {
		return "", err
	}
	found, exact := lookup.(memberofruntime.Found)
	if !exact {
		return "", fmt.Errorf(
			"evaluator registry does not contain exact callable %s",
			formatEvaluatorIdentity(pin.Rule(), expected),
		)
	}
	registration := found.Registration()
	identity := registration.Identity()
	coordinate := []string{
		pin.InvocationContract().String(),
		registration.RuleRef().String(),
		identity.ArtifactRef().String(),
		identity.Edition().String(),
		identity.Digest().String(),
		identity.Role().String(),
	}
	return strings.Join(coordinate, "\x1f"), nil
}

func exactEvaluatorCallableCoordinate[Input, Output any](
	pin projecttypeenv.EvaluatorRuntimeMechanismPin,
	registry typedmemoryevaluation.Registry[Input, Output],
) (string, error) {
	expected, err := evaluatorMechanismIdentity(pin)
	if err != nil {
		return "", err
	}
	lookup, err := registry.Lookup(pin.Rule(), expected)
	if err != nil {
		return "", err
	}
	found, exact := lookup.(typedmemoryevaluation.Found[Input, Output])
	if !exact {
		return "", fmt.Errorf(
			"evaluator registry does not contain exact callable %s",
			formatEvaluatorIdentity(pin.Rule(), expected),
		)
	}
	registration := found.Registration()
	identity := registration.Identity()
	coordinate := []string{
		pin.InvocationContract().String(),
		registration.RuleRef().String(),
		identity.ArtifactRef().String(),
		identity.Edition().String(),
		identity.Digest().String(),
		identity.Role().String(),
	}
	return strings.Join(coordinate, "\x1f"), nil
}

func matchRegistrationPolicies(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	index installedRuntimeIndex,
) (TargetRegistrationPolicies, []Issue) {
	targetPolicies, ok := runtimeBasis.ResolvedRegistrationPolicies()
	if !ok {
		issue := newIssue(
			IssueRuntimeBasisInvalid,
			"target registration-policy closure",
			"exact resolved registration-policy artifacts from X",
			"unavailable",
			"reload and verify the exact X registration-policy closure",
		)
		return invalidTargetRegistrationPolicies{}, []Issue{issue}
	}
	requiredRules := memberOfEvaluatorRules(runtimeBasis)
	issues := compareTargetPolicyRuleClosure(requiredRules, targetPolicies)
	targetRefs := make(map[string]struct{}, len(targetPolicies))
	matched := make([]recordmembershipregistration.RegistrationArtifactV1, 0, len(targetPolicies))
	for _, target := range targetPolicies {
		ref := target.Ref().String()
		targetRefs[ref] = struct{}{}
		exact := index.policiesByRef[ref]
		if len(exact) > 1 {
			issues = append(issues, newIssue(
				IssueRegistrationPolicyDuplicate,
				"registration policy "+target.Evaluator().Rule().String(),
				ref,
				fmt.Sprintf("%d exact installed policies", len(exact)),
				"retain exactly one installed policy for the target ref",
			))
			continue
		}
		if len(exact) == 0 {
			alternates := index.policiesByEvaluator[target.Evaluator().Rule().String()]
			code := IssueRegistrationPolicyMissing
			actual := "absent"
			if len(alternates) > 0 {
				code = IssueRegistrationPolicyDrift
				actual = formatPolicyRefs(alternates)
			}
			issues = append(issues, newIssue(
				code,
				"registration policy "+target.Evaluator().Rule().String(),
				ref,
				actual,
				"install the exact registration policy pinned by X for this MemberOf evaluator",
			))
			continue
		}
		matched = append(matched, exact[0])
	}
	for _, installed := range index.allPolicies {
		if _, expected := targetRefs[installed.Ref().String()]; expected {
			continue
		}
		issues = append(issues, newIssue(
			IssueUnexpectedRegistrationPolicy,
			"registration policy "+installed.Evaluator().Rule().String(),
			"only exact policy refs pinned by X",
			installed.Ref().String(),
			"remove the unexpected installed registration policy",
		))
	}
	if len(requiredRules) == 0 && len(targetPolicies) == 0 && len(issues) == 0 {
		return NoTargetRegistrationPolicies{}, nil
	}
	if len(issues) > 0 {
		return invalidTargetRegistrationPolicies{}, normalizeIssues(issues)
	}
	registry, err := newExactTargetRegistrationPolicyRegistry(matched)
	if err != nil {
		issue := newIssue(
			IssueRegistrationPolicyInvalid,
			"target registration-policy registry",
			"one exact immutable policy per MemberOf evaluator RuleRef",
			err.Error(),
			"repair the X registration-policy closure",
		)
		return invalidTargetRegistrationPolicies{}, []Issue{issue}
	}
	return registry, nil
}

func compareRegistrationPolicyCoordinates(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	policies TargetRegistrationPolicies,
) []Issue {
	exact, exists := policies.(ExactTargetRegistrationPolicyRegistry)
	if !exists {
		return nil
	}
	issues := make([]Issue, 0, exact.Len()*2)
	for _, artifact := range exact.artifacts {
		evaluator := artifact.Evaluator()
		delivery := artifact.SourceDeliveryBoundary()
		evaluatorMatched := false
		deliveryMatched := false
		for _, pin := range runtimeBasis.Pins() {
			switch value := pin.(type) {
			case projecttypeenv.EvaluatorRuntimeMechanismPin:
				if value.InvocationContract() != projecttypeenv.RuntimeMechanismContractMemberOf {
					continue
				}
				if policyCoordinateMatchesPin(evaluator, value.Rule(), value.Mechanism()) {
					evaluatorMatched = true
				}
			case projecttypeenv.CarrierMembershipRuntimeMechanismPin:
				if policyCoordinateMatchesPin(delivery, value.Rule(), value.Mechanism()) {
					deliveryMatched = true
				}
			}
		}
		if !evaluatorMatched {
			issues = append(issues, newIssue(
				IssuePolicyEvaluatorDrift,
				"registration-policy evaluator "+evaluator.Rule().String(),
				"one exact member_of evaluator pin with "+formatPolicyCoordinate(evaluator),
				"no matching X evaluator pin",
				"align the registration policy and target X evaluator coordinate",
			))
		}
		if !deliveryMatched {
			issues = append(issues, newIssue(
				IssuePolicySourceDeliveryDrift,
				"registration-policy source delivery "+delivery.Rule().String(),
				"one exact carrier-membership pin with "+formatPolicyCoordinate(delivery),
				"no matching X carrier-membership pin",
				"align the registration policy and target X delivery coordinate",
			))
		}
	}
	return normalizeIssues(issues)
}

func memberOfEvaluatorRules(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
) []typedmemory.RuleRef {
	byRule := make(map[string]typedmemory.RuleRef)
	for _, runtimePin := range runtimeBasis.Pins() {
		pin, evaluator := runtimePin.(projecttypeenv.EvaluatorRuntimeMechanismPin)
		if !evaluator ||
			pin.InvocationContract() != projecttypeenv.RuntimeMechanismContractMemberOf {
			continue
		}
		byRule[pin.Rule().String()] = pin.Rule()
	}
	keys := make([]string, 0, len(byRule))
	for key := range byRule {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]typedmemory.RuleRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, byRule[key])
	}
	return result
}

func compareTargetPolicyRuleClosure(
	required []typedmemory.RuleRef,
	policies []recordmembershipregistration.RegistrationArtifactV1,
) []Issue {
	requiredByRule := make(map[string]struct{}, len(required))
	for _, rule := range required {
		requiredByRule[rule.String()] = struct{}{}
	}
	policiesByRule := make(
		map[string][]recordmembershipregistration.RegistrationArtifactV1,
		len(policies),
	)
	for _, policy := range policies {
		rule := policy.Evaluator().Rule().String()
		policiesByRule[rule] = append(policiesByRule[rule], policy)
	}
	issues := make([]Issue, 0)
	for _, rule := range required {
		matches := policiesByRule[rule.String()]
		if len(matches) == 0 {
			issues = append(issues, newIssue(
				IssueRegistrationPolicyMissing,
				"registration policy "+rule.String(),
				"one exact X-pinned policy for this MemberOf evaluator RuleRef",
				"absent",
				"pin and resolve the exact family registration policy in X",
			))
			continue
		}
		if len(matches) > 1 {
			issues = append(issues, newIssue(
				IssueRegistrationPolicyDuplicate,
				"registration policy "+rule.String(),
				"one exact X-pinned policy for this MemberOf evaluator RuleRef",
				formatPolicyRefs(matches),
				"retain exactly one family registration policy in X",
			))
		}
	}
	for rule, matches := range policiesByRule {
		if _, expected := requiredByRule[rule]; expected {
			continue
		}
		issues = append(issues, newIssue(
			IssueUnexpectedRegistrationPolicy,
			"registration policy "+rule,
			"no policy for an unrequired MemberOf evaluator",
			formatPolicyRefs(matches),
			"remove the unexpected family policy from X",
		))
	}
	return normalizeIssues(issues)
}

func verifyExactTargetRuntimeRegistryState(
	state exactTargetRuntimeRegistryState,
) error {
	policies := make([]recordmembershipregistration.RegistrationArtifactV1, 0)
	switch policySet := state.policies.(type) {
	case NoTargetRegistrationPolicies:
	case ExactTargetRegistrationPolicyRegistry:
		artifacts, ok := policySet.Artifacts()
		if !ok {
			return fmt.Errorf("target registration-policy registry is invalid")
		}
		policies = append(policies, artifacts...)
	default:
		return fmt.Errorf("target registration-policy posture is invalid")
	}
	installed := installedRuntimeInputFromState(state, policies)
	index, invalidIssues := indexInstalledRuntime(installed)
	if len(invalidIssues) > 0 {
		return fmt.Errorf("stored target runtime observation is invalid: %s", formatIssues(invalidIssues))
	}
	comparison := compareInstalledRuntime(
		state.runtimeBasis,
		installed,
		index,
	)
	if len(comparison.issues) > 0 {
		return fmt.Errorf("stored target runtime observation no longer verifies: %s", formatIssues(comparison.issues))
	}
	digest, err := deriveTargetRuntimeRegistryDigest(
		state.runtimeBasis,
		installed,
		comparison.matchedCatalogs,
		comparison.matchedPolicies,
	)
	if err != nil {
		return err
	}
	if digest != state.digest {
		return fmt.Errorf("target runtime coordinate digest mismatch")
	}
	return nil
}

func installedRuntimeInputFromState(
	state exactTargetRuntimeRegistryState,
	policies []recordmembershipregistration.RegistrationArtifactV1,
) InstalledRuntimeRegistryInput {
	return InstalledRuntimeRegistryInput{
		Codecs:                                   state.codecs,
		EntitySetEnumerationEvaluators:           state.entitySetEnumeration,
		CandidateVisibilityEvaluators:            state.candidateVisibility,
		KindDefinednessEvaluators:                state.kindDefinedness,
		MemberOfEvaluators:                       state.memberOf,
		KindClassificationEvaluators:             state.kindClassification,
		ReferenceDesignationResolutionEvaluators: state.referenceDesignationResolution,
		ClaimInterpretationEvaluators:            state.claimInterpretation,
		ClaimMeasurementEvaluators:               state.claimMeasurement,
		ClaimEvaluationEvaluators:                state.claimEvaluation,
		EpistemeConstitutionEvaluators:           state.epistemeConstitution,
		MechanismCatalogs:                        state.mechanismCatalogs,
		RegistrationPolicies:                     policies,
	}
}

func deriveTargetRuntimeRegistryDigest(
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed InstalledRuntimeRegistryInput,
	catalogs []runtimemechanism.RuntimeMechanismArtifactV1,
	policies TargetRegistrationPolicies,
) (typedmemory.SHA256Digest, error) {
	if err := runtimeBasis.VerifyResolvedClosure(); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	writer := targetRuntimeDigestWriter{}
	writer.addString(targetRuntimeRegistryDigestDomain)
	writer.addString(runtimeBasis.Ref().String())
	writer.addBytes(runtimeBasis.CanonicalBytes())
	ordered := append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), catalogs...)
	sort.Slice(ordered, func(left int, right int) bool {
		return compareCatalogIdentity(ordered[left], ordered[right]) < 0
	})
	if err := writer.addCount(len(ordered)); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	for _, catalog := range ordered {
		if err := catalog.Verify(); err != nil {
			return typedmemory.SHA256Digest{}, err
		}
		writer.addBytes(catalog.CanonicalBytes())
	}
	callables, err := exactEvaluatorCallableCoordinates(runtimeBasis, installed)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	if err := writer.addCount(len(callables)); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	for _, callable := range callables {
		writer.addString(callable)
	}
	switch value := policies.(type) {
	case NoTargetRegistrationPolicies:
		writer.addString("no_registration_policies")
	case ExactTargetRegistrationPolicyRegistry:
		artifacts, ok := value.Artifacts()
		if !ok {
			return typedmemory.SHA256Digest{}, fmt.Errorf("target registration-policy registry is invalid")
		}
		writer.addString("exact_registration_policy_registry")
		if err := writer.addCount(len(artifacts)); err != nil {
			return typedmemory.SHA256Digest{}, err
		}
		for _, artifact := range artifacts {
			writer.addString(artifact.Evaluator().Rule().String())
			writer.addBytes(artifact.CanonicalBytes())
		}
	default:
		return typedmemory.SHA256Digest{}, fmt.Errorf("target registration-policy posture is invalid")
	}
	sum := sha256.Sum256(writer.bytes())
	encoded := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(encoded)
}

type targetRuntimeDigestWriter struct {
	data []byte
}

func (writer *targetRuntimeDigestWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *targetRuntimeDigestWriter) addCount(value int) error {
	if value < 0 {
		return fmt.Errorf("target runtime digest count must be non-negative")
	}
	frame := make([]byte, 8)
	binary.BigEndian.PutUint64(
		frame,
		uint64(value), // #nosec G115 -- value is checked non-negative above.
	)
	writer.addBytes(frame)
	return nil
}

func (writer *targetRuntimeDigestWriter) addBytes(value []byte) {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	writer.data = append(writer.data, length...)
	writer.data = append(writer.data, value...)
}

func (writer targetRuntimeDigestWriter) bytes() []byte {
	return append([]byte(nil), writer.data...)
}

func mechanismKeyFromPin(
	pin projecttypeenv.RuntimeMechanismArtifactPin,
) mechanismIdentityKey {
	return mechanismIdentityKey{
		artifact: pin.Artifact().String(),
		edition:  pin.Edition().String(),
		digest:   pin.Digest().String(),
	}
}

func mechanismForRuntimePin(
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
) projecttypeenv.RuntimeMechanismArtifactPin {
	switch value := pin.(type) {
	case projecttypeenv.CodecRuntimeMechanismPin:
		return value.Mechanism()
	case projecttypeenv.EvaluatorRuntimeMechanismPin:
		return value.Mechanism()
	case projecttypeenv.CarrierMembershipRuntimeMechanismPin:
		return value.Mechanism()
	default:
		return projecttypeenv.RuntimeMechanismArtifactPin{}
	}
}

func catalogContainsPin(
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
) bool {
	for _, entry := range catalog.Entries() {
		if entry.Role().String() != string(pin.Role()) {
			continue
		}
		if entry.Contract().String() != pin.InvocationContract().String() {
			continue
		}
		if runtimeEntryMatchesPin(entry, pin) {
			return true
		}
	}
	return false
}

func runtimeEntryMatchesPin(
	entry runtimemechanism.RuntimeMechanismEntryV1,
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
) bool {
	switch value := pin.(type) {
	case projecttypeenv.CodecRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.CodecSemanticCoordinate)
		return matches && coordinate.Ref() == value.Codec()
	case projecttypeenv.EvaluatorRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.RuleSemanticCoordinate)
		return matches && coordinate.Ref() == value.Rule()
	case projecttypeenv.CarrierMembershipRuntimeMechanismPin:
		coordinate, matches := entry.Semantic().(runtimemechanism.RuleSemanticCoordinate)
		return matches && coordinate.Ref() == value.Rule()
	default:
		return false
	}
}

func policyCoordinateMatchesPin(
	coordinate recordmembershipregistration.MechanismCoordinate,
	rule typedmemory.RuleRef,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
) bool {
	return coordinate.Rule() == rule &&
		coordinate.Artifact() == mechanism.Artifact() &&
		coordinate.Edition() == mechanism.Edition() &&
		coordinate.Digest() == mechanism.Digest()
}

func sortedMechanismCatalogs(
	values map[mechanismIdentityKey]runtimemechanism.RuntimeMechanismArtifactV1,
) []runtimemechanism.RuntimeMechanismArtifactV1 {
	result := make([]runtimemechanism.RuntimeMechanismArtifactV1, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(left int, right int) bool {
		return compareCatalogIdentity(result[left], result[right]) < 0
	})
	return result
}

func compareCatalogIdentity(
	left runtimemechanism.RuntimeMechanismArtifactV1,
	right runtimemechanism.RuntimeMechanismArtifactV1,
) int {
	leftIdentity := left.Identity()
	rightIdentity := right.Identity()
	if order := cmp.Compare(
		leftIdentity.Artifact().String(),
		rightIdentity.Artifact().String(),
	); order != 0 {
		return order
	}
	if order := cmp.Compare(
		leftIdentity.Edition().String(),
		rightIdentity.Edition().String(),
	); order != 0 {
		return order
	}
	return cmp.Compare(
		leftIdentity.Digest().String(),
		rightIdentity.Digest().String(),
	)
}

func cloneMechanismCatalogs(
	values []runtimemechanism.RuntimeMechanismArtifactV1,
) []runtimemechanism.RuntimeMechanismArtifactV1 {
	return append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), values...)
}

func cloneTargetRegistrationPolicies(
	policies TargetRegistrationPolicies,
) TargetRegistrationPolicies {
	switch value := policies.(type) {
	case NoTargetRegistrationPolicies:
		return value
	case ExactTargetRegistrationPolicyRegistry:
		cloned, err := newExactTargetRegistrationPolicyRegistry(value.artifacts)
		if err != nil {
			return invalidTargetRegistrationPolicies{}
		}
		return cloned
	default:
		return invalidTargetRegistrationPolicies{}
	}
}

func runtimePinSubject(pin projecttypeenv.RuntimeEvaluationMechanismPin) string {
	return string(pin.Role()) + ":" + pin.InvocationContract().String() + ":" +
		runtimePinSemanticCoordinate(pin)
}

func runtimePinSemanticCoordinate(
	pin projecttypeenv.RuntimeEvaluationMechanismPin,
) string {
	switch value := pin.(type) {
	case projecttypeenv.CodecRuntimeMechanismPin:
		return value.Codec().String()
	case projecttypeenv.EvaluatorRuntimeMechanismPin:
		return value.Rule().String()
	case projecttypeenv.CarrierMembershipRuntimeMechanismPin:
		return value.Rule().String()
	default:
		return fmt.Sprintf("%T", pin)
	}
}

func formatMechanismIdentity(identity mechanismIdentityKey) string {
	return identity.artifact + "@" + identity.edition + "#" + identity.digest
}

func formatCatalogIdentities(
	catalogs []runtimemechanism.RuntimeMechanismArtifactV1,
) string {
	values := append([]runtimemechanism.RuntimeMechanismArtifactV1(nil), catalogs...)
	sort.Slice(values, func(left int, right int) bool {
		return compareCatalogIdentity(values[left], values[right]) < 0
	})
	formatted := make([]string, 0, len(values))
	for _, catalog := range values {
		identity := catalog.Identity()
		formatted = append(formatted, formatMechanismIdentity(mechanismIdentityKey{
			artifact: identity.Artifact().String(),
			edition:  identity.Edition().String(),
			digest:   identity.Digest().String(),
		}))
	}
	return "[" + strings.Join(formatted, ",") + "]"
}

func formatPolicyRefs(
	policies []recordmembershipregistration.RegistrationArtifactV1,
) string {
	values := make([]string, 0, len(policies))
	for _, policy := range policies {
		values = append(values, policy.Ref().String())
	}
	sort.Strings(values)
	return "[" + strings.Join(values, ",") + "]"
}

func formatPolicyCoordinate(
	coordinate recordmembershipregistration.MechanismCoordinate,
) string {
	return coordinate.Rule().String() + "@" +
		coordinate.Artifact().String() + "@" +
		coordinate.Edition().String() + "#" +
		coordinate.Digest().String()
}

func formatEvaluatorIdentity(
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) string {
	return rule.String() + "@" +
		identity.ArtifactRef().String() + "@" +
		identity.Edition().String() + "#" +
		identity.Digest().String()
}

func formatIssues(issues []Issue) string {
	values := make([]string, 0, len(issues))
	for _, issue := range normalizeIssues(issues) {
		values = append(values, string(issue.code)+":"+issue.subject)
	}
	return strings.Join(values, ",")
}
