// Package localpracticeruntime composes the shipped Haft typed-memory
// Local-Practice carrier into one exact executable B/E/X/C target and the
// process-installed runtime registry capable of executing that target.
//
// Construction is pure with respect to project state. It neither stages nor
// selects a ProjectTypeEnvHead and exposes no database or authority port.
package localpracticeruntime

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/projectmemoryreferencescheme"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

const (
	runtimeCarrier = "artifact:haft-production-local-practice-runtime"
)

// Target is the immutable result of compiling one exact base artifact and one
// exact Haft Local-Practice source carrier. Its accessors expose immutable
// values only; no accessor performs project activation.
type Target struct {
	base         typeenv.BaseTypeEnvArtifact
	extension    projecttypeenv.ProjectTypeEnvExtensionArtifact
	linked       projecttypeenv.LinkedProjectTypeEnvCompositeIR
	requirements projecttypeenv.CompositeRuntimeRequirementSet
	runtime      projecttypeenv.RuntimeEvaluationBasisArtifact
	mechanism    runtimemechanism.RuntimeMechanismArtifactV1
	policies     []recordmembershipregistration.RegistrationArtifactV1
	composite    projecttypeenv.ProjectTypeEnvCompositeArtifact
	preparation  projecttypeenv.ProjectTypeEnvCompositePreparation
	installed    projecttypeenvruntime.InstalledRuntimeRegistryInput
	registry     projecttypeenvruntime.ExactTargetRuntimeRegistry
}

func (target Target) Base() typeenv.BaseTypeEnvArtifact {
	return target.base
}

func (target Target) Extension() projecttypeenv.ProjectTypeEnvExtensionArtifact {
	return target.extension
}

func (target Target) Linked() projecttypeenv.LinkedProjectTypeEnvCompositeIR {
	return target.linked
}

func (target Target) Requirements() projecttypeenv.CompositeRuntimeRequirementSet {
	return target.requirements
}

func (target Target) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisArtifact {
	return target.runtime
}

func (target Target) Mechanism() runtimemechanism.RuntimeMechanismArtifactV1 {
	return target.mechanism
}

func (target Target) RegistrationPolicies() []recordmembershipregistration.RegistrationArtifactV1 {
	return append(
		[]recordmembershipregistration.RegistrationArtifactV1(nil),
		target.policies...,
	)
}

func (target Target) Composite() projecttypeenv.ProjectTypeEnvCompositeArtifact {
	return target.composite
}

func (target Target) Preparation() projecttypeenv.ProjectTypeEnvCompositePreparation {
	return target.preparation
}

func (target Target) InstalledRuntime() projecttypeenvruntime.InstalledRuntimeRegistryInput {
	return cloneInstalledRuntime(target.installed)
}

func (target Target) ExactRuntimeRegistry() (
	projecttypeenvruntime.ExactTargetRuntimeRegistry,
	bool,
) {
	if !target.registry.Valid() {
		return projecttypeenvruntime.ExactTargetRuntimeRegistry{}, false
	}
	return target.registry, true
}

// Build returns the immutable target compiled from the exact base artifact and
// source carrier bytes. Successful targets are reused process-wide for that
// exact input pair; failed builds are never cached. The final observation must
// match before a target is admitted to the cache.
func Build(
	base typeenv.BaseTypeEnvArtifact,
	source []byte,
) (Target, error) {
	return processTargetBuildCache.load(base, source, buildTargetUncached)
}

func buildTargetUncached(
	base typeenv.BaseTypeEnvArtifact,
	source []byte,
) (Target, error) {
	parsed, err := localpractice.Parse(source)
	if err != nil {
		return Target{}, fmt.Errorf("parse typed-memory Local-Practice carrier: %w", err)
	}
	manifest := projecttypeenv.ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	if manifest.Rejected() {
		return Target{}, fmt.Errorf(
			"resolve typed-memory Local-Practice manifest: %v",
			manifest.Issues(),
		)
	}
	bundle, present := manifest.Bundle()
	if !present {
		return Target{}, fmt.Errorf(
			"resolve typed-memory Local-Practice manifest: accepted result has no bundle",
		)
	}
	nodes := bundle.Nodes()
	if len(nodes) != 1 {
		return Target{}, fmt.Errorf(
			"resolve typed-memory Local-Practice manifest: got %d nodes, want exactly 1",
			len(nodes),
		)
	}
	ir, err := projecttypeenv.CompileProjectTypeEnvExtensionIR(nodes[0], nil)
	if err != nil {
		return Target{}, fmt.Errorf("compile typed-memory Local-Practice E: %w", err)
	}
	extension, err := projecttypeenv.SealProjectTypeEnvExtension(ir)
	if err != nil {
		return Target{}, fmt.Errorf("seal typed-memory Local-Practice E: %w", err)
	}
	link := projecttypeenv.LinkProjectTypeEnvCompositeIR(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
	)
	if link.Rejected() {
		return Target{}, fmt.Errorf(
			"link typed-memory Local-Practice B/E: %v",
			link.Issues(),
		)
	}
	linked, present := link.CompositeIR()
	if !present {
		return Target{}, fmt.Errorf(
			"link typed-memory Local-Practice B/E: accepted result has no linked IR",
		)
	}
	discovery := projecttypeenv.DiscoverProjectTypeEnvCompositeRuntimeRequirements(
		base,
		linked,
	)
	if discovery.Rejected() {
		return Target{}, fmt.Errorf(
			"discover typed-memory Local-Practice runtime requirements: %v",
			discovery.Issues(),
		)
	}
	requirements, present := discovery.RequiredSet()
	if !present || len(requirements.Requirements()) == 0 {
		return Target{}, fmt.Errorf(
			"discover typed-memory Local-Practice runtime requirements: exact requirement set is empty",
		)
	}
	runtimeEdition := parsed.Carrier().Identity().Edition().Value()
	runtime, mechanism, policies, err := buildRuntime(
		runtimeEdition,
		requirements.Requirements(),
	)
	if err != nil {
		return Target{}, err
	}
	composite, err := sealCompositeForLocalPracticeEdition(
		linked,
		runtime,
		runtimeEdition,
	)
	if err != nil {
		return Target{}, fmt.Errorf("seal typed-memory Local-Practice C: %w", err)
	}
	preparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtime,
			Composite:    composite,
		},
	)
	if preparation.Rejected() {
		return Target{}, fmt.Errorf(
			"prepare typed-memory Local-Practice C: %v",
			preparation.Issues(),
		)
	}
	environment, present := preparation.Environment()
	if !present {
		return Target{}, fmt.Errorf(
			"prepare typed-memory Local-Practice C: no executable environment",
		)
	}
	installed, err := buildInstalledRuntime(
		base,
		environment,
		requirements,
		mechanism,
		policies,
	)
	if err != nil {
		return Target{}, err
	}
	observation := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtime,
			Installed:    installed,
		},
	)
	matched, exact := observation.(projecttypeenvruntime.Matched)
	if !exact {
		return Target{}, fmt.Errorf(
			"observe installed typed-memory Local-Practice runtime: %s",
			observation.Kind(),
		)
	}
	registry, present := matched.Registry()
	if !present || !registry.Valid() {
		return Target{}, fmt.Errorf(
			"observe installed typed-memory Local-Practice runtime: exact registry is absent",
		)
	}
	return Target{
		base:         base,
		extension:    extension,
		linked:       linked,
		requirements: requirements,
		runtime:      runtime,
		mechanism:    mechanism,
		policies:     append([]recordmembershipregistration.RegistrationArtifactV1(nil), policies...),
		composite:    composite,
		preparation:  preparation,
		installed:    cloneInstalledRuntime(installed),
		registry:     registry,
	}, nil
}

func sealCompositeForLocalPracticeEdition(
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	runtime projecttypeenv.RuntimeEvaluationBasisArtifact,
	edition string,
) (projecttypeenv.ProjectTypeEnvCompositeArtifact, error) {
	historical := map[string]struct{}{
		"1.0.0": {},
		"1.1.0": {},
		"1.2.0": {},
	}
	if _, sealed := historical[edition]; sealed {
		return projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
			linked,
			runtime,
		)
	}
	return projecttypeenv.SealProjectTypeEnvComposite(linked, runtime)
}

func buildRuntime(
	runtimeEdition string,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
	[]recordmembershipregistration.RegistrationArtifactV1,
	error,
) {
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, err := mechanismEntry(requirement)
		if err != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{},
				runtimemechanism.RuntimeMechanismArtifactV1{},
				nil,
				err
		}
		entries = append(entries, entry)
	}
	carrier, err := typedmemory.NewCarrierRef(runtimeCarrier)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			fmt.Errorf("construct Local-Practice runtime carrier: %w", err)
	}
	edition, err := typedmemory.NewCarrierEdition(runtimeEdition)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			fmt.Errorf("construct Local-Practice runtime edition: %w", err)
	}
	mechanism, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		carrier,
		edition,
		entries,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			fmt.Errorf("seal Local-Practice runtime mechanism catalog: %w", err)
	}
	mechanismPin, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(
		mechanism,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			fmt.Errorf("pin Local-Practice runtime mechanism catalog: %w", err)
	}
	pins := make([]projecttypeenv.RuntimeEvaluationBasisPin, 0, len(requirements)+1)
	for _, requirement := range requirements {
		pin, pinErr := mechanismPinForRequirement(
			requirement,
			mechanismPin,
			mechanism,
		)
		if pinErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{},
				runtimemechanism.RuntimeMechanismArtifactV1{},
				nil,
				pinErr
		}
		pins = append(pins, pin)
	}
	policies, err := buildRegistrationPolicies(
		mechanism,
		requirements,
		runtimeEdition,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			err
	}
	for _, policy := range policies {
		policyPin, pinErr := projecttypeenv.NewRegistrationPolicyPin(policy)
		if pinErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{},
				runtimemechanism.RuntimeMechanismArtifactV1{},
				nil,
				fmt.Errorf("pin Local-Practice registration policy: %w", pinErr)
		}
		pins = append(pins, policyPin)
	}
	runtime, err := projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		pins,
		nil,
		nil,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			nil,
			fmt.Errorf("seal exact Local-Practice X: %w", err)
	}
	return runtime, mechanism, policies, nil
}

func mechanismEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	if codec, present := requirement.Codec(); present {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, present := requirement.Rule()
	if !present {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"Local-Practice runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	constructors := map[projecttypeenv.RuntimeMechanismInvocationContract]func(
		typedmemory.RuleRef,
	) (runtimemechanism.RuntimeMechanismEntryV1, error){
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:           runtimemechanism.NewEntitySetEnumerationEntry,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility:            runtimemechanism.NewCandidateVisibilityEntry,
		projecttypeenv.RuntimeMechanismContractKindDefinedness:                runtimemechanism.NewKindDefinednessEntry,
		projecttypeenv.RuntimeMechanismContractMemberOf:                       runtimemechanism.NewMemberOfEntry,
		projecttypeenv.RuntimeMechanismContractKindClassification:             runtimemechanism.NewKindClassificationEntry,
		projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:      runtimemechanism.NewCarrierMembershipDeliveryEntry,
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution: runtimemechanism.NewReferenceDesignationResolutionEntry,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation:            runtimemechanism.NewClaimInterpretationEntry,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement:               runtimemechanism.NewClaimMeasurementEntry,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation:                runtimemechanism.NewClaimEvaluationEntry,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation: runtimemechanism.NewEpistemeConstitutionEvaluationEntry,
	}
	constructor, present := constructors[requirement.InvocationContract()]
	if !present {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"unsupported Local-Practice runtime invocation contract %q",
			requirement.InvocationContract(),
		)
	}
	return constructor(rule)
}

func mechanismPinForRequirement(
	requirement projecttypeenv.CompositeRuntimeRequirement,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (projecttypeenv.RuntimeEvaluationMechanismPin, error) {
	if codec, present := requirement.Codec(); present {
		return projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec:            codec,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	rule, present := requirement.Rule()
	if !present {
		return nil, fmt.Errorf(
			"Local-Practice runtime requirement %q has no RuleRef",
			requirement.SemanticReference(),
		)
	}
	if requirement.Role() == projecttypeenv.RuntimeMechanismRoleCarrierMembership {
		return projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	return projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         requirement.InvocationContract(),
			Mechanism:        mechanism,
			ResolvedArtifact: &artifact,
		},
	)
}

type registrationMapping struct {
	manifest recordmapping.MappingManifestRef
	adapter  recordmapping.AdapterVersion
}

func buildRegistrationPolicies(
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	requirements []projecttypeenv.CompositeRuntimeRequirement,
	runtimeEdition string,
) ([]recordmembershipregistration.RegistrationArtifactV1, error) {
	rules := make(map[string]typedmemory.RuleRef)
	for _, requirement := range requirements {
		if requirement.InvocationContract() != projecttypeenv.RuntimeMechanismContractMemberOf {
			continue
		}
		rule, present := requirement.Rule()
		if !present {
			return nil, fmt.Errorf(
				"Local-Practice MemberOf requirement has no RuleRef",
			)
		}
		rules[rule.String()] = rule
	}
	ordered := make([]string, 0, len(rules))
	for raw := range rules {
		ordered = append(ordered, raw)
	}
	sort.Strings(ordered)
	policies := make([]recordmembershipregistration.RegistrationArtifactV1, 0, len(ordered))
	for _, raw := range ordered {
		mappings, err := registrationMappingsForRule(
			runtimeEdition,
			rules[raw],
		)
		if err != nil {
			return nil, err
		}
		policy, err := buildRegistrationPolicy(mechanism, rules[raw], mappings)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

func buildRegistrationPolicy(
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
	mappings []registrationMapping,
) (recordmembershipregistration.RegistrationArtifactV1, error) {
	identity := mechanism.Identity()
	evaluator, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"construct Local-Practice membership evaluator coordinate: %w",
			err,
		)
	}
	delivery, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
	if err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"construct Local-Practice membership delivery coordinate: %w",
			err,
		)
	}
	accepted, err := buildAcceptedMappings(mappings)
	if err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"construct Local-Practice accepted mappings: %w",
			err,
		)
	}
	policy, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       accepted,
		},
	)
	if err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"seal Local-Practice registration policy: %w",
			err,
		)
	}
	return policy, nil
}

func buildAcceptedMappings(
	mappings []registrationMapping,
) ([]recordmembershipregistration.AcceptedMapping, error) {
	accepted := make(
		[]recordmembershipregistration.AcceptedMapping,
		0,
		len(mappings),
	)
	for _, mapping := range mappings {
		value, err := recordmembershipregistration.NewAcceptedMapping(
			recordmembershipregistration.AcceptedMappingInput{
				Manifest: mapping.manifest,
				Adapter:  mapping.adapter,
			},
		)
		if err != nil {
			return nil, err
		}
		accepted = append(accepted, value)
	}
	return accepted, nil
}

func registrationMappingsForRule(
	runtimeEdition string,
	rule typedmemory.RuleRef,
) ([]registrationMapping, error) {
	projectEntityRule, err := typedmemory.NewRuleRef(
		"haft.member-of.project-entity/v1",
	)
	if err != nil {
		return nil, err
	}
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	if rule == projectEntityRule {
		manifest, manifestErr :=
			projectmemory.CurrentProjectEntityUniverseMappingManifestV1()
		if manifestErr != nil {
			return nil, manifestErr
		}
		return []registrationMapping{{
			manifest: manifest.Ref(),
			adapter:  manifest.AdapterVersion(),
		}}, nil
	}
	if rule == recordRule {
		if runtimeEdition == "1.0.0" {
			shipped, shippedErr := projectRecordShippedV1AcceptedMappings()
			if shippedErr != nil {
				return nil, shippedErr
			}
			return acceptedRegistrationMappings(shipped), nil
		}
		if !currentRegistrationPolicyEdition(runtimeEdition) {
			return nil, fmt.Errorf(
				"unsupported Local-Practice registration-policy edition %q",
				runtimeEdition,
			)
		}
		note, noteErr := noteadapter.CurrentMappingManifestV1()
		if noteErr != nil {
			return nil, noteErr
		}
		problem, problemErr := problemcardadapter.CurrentMappingManifestV1()
		if problemErr != nil {
			return nil, problemErr
		}
		portfolio, portfolioErr :=
			solutionportfolioadapter.CurrentMappingManifestV1()
		if portfolioErr != nil {
			return nil, portfolioErr
		}
		comparison, comparisonErr :=
			portfoliocomparisonadapter.CurrentMappingManifestV1()
		if comparisonErr != nil {
			return nil, comparisonErr
		}
		specSection, specSectionErr :=
			specsectionadapter.CurrentMappingManifestV1()
		if specSectionErr != nil {
			return nil, specSectionErr
		}
		evidenceWork, evidenceWorkErr :=
			evidenceworkadapter.CurrentMappingManifestV1()
		if evidenceWorkErr != nil {
			return nil, evidenceWorkErr
		}
		decision, decisionErr :=
			decisionrecordadapter.CurrentMappingManifestV1()
		if decisionErr != nil {
			return nil, decisionErr
		}
		current := []registrationMapping{
			{
				manifest: note.Ref(),
				adapter:  note.AdapterVersion(),
			},
			{
				manifest: problem.Ref(),
				adapter:  problem.AdapterVersion(),
			},
			{
				manifest: portfolio.Ref(),
				adapter:  portfolio.AdapterVersion(),
			},
			{
				manifest: comparison.Ref(),
				adapter:  comparison.AdapterVersion(),
			},
			{
				manifest: specSection.Ref(),
				adapter:  specSection.AdapterVersion(),
			},
			{
				manifest: evidenceWork.Ref(),
				adapter:  evidenceWork.AdapterVersion(),
			},
			{
				manifest: decision.Ref(),
				adapter:  decision.AdapterVersion(),
			},
		}
		compatible, compatibilityErr :=
			ProjectRecordTargetReviewedCompatibilityMappingsV1()
		if compatibilityErr != nil {
			return nil, compatibilityErr
		}
		return appendAcceptedRegistrationMappings(current, compatible), nil
	}
	if rule == carrierfamily.CodeAnchorEvaluatorRuleV1() {
		family, familyErr :=
			carrierfamily.CurrentCodeAnchorMappingManifestV1()
		if familyErr != nil {
			return nil, familyErr
		}
		if runtimeEdition == "1.0.0" {
			shipped, shippedErr := codeAnchorShippedV1AcceptedMappings()
			if shippedErr != nil {
				return nil, shippedErr
			}
			return appendAcceptedRegistrationMappings(
				[]registrationMapping{{
					manifest: family.Ref(),
					adapter:  family.AdapterVersion(),
				}},
				shipped,
			), nil
		}
		if !currentRegistrationPolicyEdition(runtimeEdition) {
			return nil, fmt.Errorf(
				"unsupported Local-Practice registration-policy edition %q",
				runtimeEdition,
			)
		}
		task, taskErr := codeanchoradapter.CurrentMappingManifestV1()
		if taskErr != nil {
			return nil, taskErr
		}
		current := []registrationMapping{
			{
				manifest: family.Ref(),
				adapter:  family.AdapterVersion(),
			},
			{
				manifest: task.Ref(),
				adapter:  task.AdapterVersion(),
			},
		}
		compatible, compatibilityErr :=
			CodeAnchorTargetReviewedCompatibilityMappingsV1()
		if compatibilityErr != nil {
			return nil, compatibilityErr
		}
		return appendAcceptedRegistrationMappings(current, compatible), nil
	}
	manifest, err := carrierFamilyManifest(rule)
	if err != nil {
		return nil, err
	}
	return []registrationMapping{{
		manifest: manifest.Ref(),
		adapter:  manifest.AdapterVersion(),
	}}, nil
}

func currentRegistrationPolicyEdition(edition string) bool {
	return edition == "1.1.0" || edition == "1.2.0"
}

func acceptedRegistrationMappings(
	accepted []recordmembershipregistration.AcceptedMapping,
) []registrationMapping {
	return appendAcceptedRegistrationMappings(nil, accepted)
}

func appendAcceptedRegistrationMappings(
	current []registrationMapping,
	compatible []recordmembershipregistration.AcceptedMapping,
) []registrationMapping {
	result := append([]registrationMapping(nil), current...)
	for _, mapping := range compatible {
		result = append(result, registrationMapping{
			manifest: mapping.Manifest(),
			adapter:  mapping.Adapter(),
		})
	}
	return result
}

func carrierFamilyManifest(
	rule typedmemory.RuleRef,
) (carrierfamily.MappingManifestV1, error) {
	constructors := map[string]func() (carrierfamily.MappingManifestV1, error){
		carrierfamily.CarrierEditionEvaluatorRuleV1().String():          carrierfamily.CurrentCarrierEditionMappingManifestV1,
		carrierfamily.ProjectClaimEvaluatorRuleV1().String():            carrierfamily.CurrentProjectClaimMappingManifestV1,
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1().String(): carrierfamily.CurrentPerformedWorkOccurrenceMappingManifestV1,
		carrierfamily.CodeAnchorEvaluatorRuleV1().String():              carrierfamily.CurrentCodeAnchorMappingManifestV1,
	}
	constructor, present := constructors[rule.String()]
	if !present {
		return carrierfamily.MappingManifestV1{}, fmt.Errorf(
			"unsupported Local-Practice MemberOf rule %q",
			rule.String(),
		)
	}
	return constructor()
}

func buildInstalledRuntime(
	base typeenv.BaseTypeEnvArtifact,
	environment typedmemory.TypeEnv,
	requirements projecttypeenv.CompositeRuntimeRequirementSet,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
	policies []recordmembershipregistration.RegistrationArtifactV1,
) (projecttypeenvruntime.InstalledRuntimeRegistryInput, error) {
	codecs, err := buildCodecRegistry(base, environment)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	identity, err := mechanismIdentity(mechanism)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	enumeration, visibility, definedness, err := buildPrerequisiteRegistries(
		requirements,
		identity,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	memberOf, err := buildHistoricalMemberOfRegistry(
		requirements,
		policies,
		enumeration,
		visibility,
		definedness,
		identity,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	classification, err :=
		projectmemory.NewProjectKindClassificationEvaluatorRegistry(
			environment,
			codecs,
			identity,
		)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	referenceDesignationResolution, err := buildEvaluatorRegistryForContract(
		requirements.Requirements(),
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution,
		identity,
		typedmemoryevaluation.NewReferenceDesignationResolutionRegistry,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	claimInterpretation, err := buildEvaluatorRegistryForContract(
		requirements.Requirements(),
		projecttypeenv.RuntimeMechanismContractClaimInterpretation,
		identity,
		typedmemoryevaluation.NewClaimInterpretationRegistry,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	claimMeasurement, err := buildEvaluatorRegistryForContract(
		requirements.Requirements(),
		projecttypeenv.RuntimeMechanismContractClaimMeasurement,
		identity,
		typedmemoryevaluation.NewClaimMeasurementRegistry,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	claimEvaluation, err := buildEvaluatorRegistryForContract(
		requirements.Requirements(),
		projecttypeenv.RuntimeMechanismContractClaimEvaluation,
		identity,
		typedmemoryevaluation.NewClaimEvaluationRegistry,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	epistemeConstitution, err := buildEvaluatorRegistryForContract(
		requirements.Requirements(),
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation,
		identity,
		typedmemoryevaluation.NewEpistemeConstitutionEvaluationRegistry,
	)
	if err != nil {
		return projecttypeenvruntime.InstalledRuntimeRegistryInput{}, err
	}
	return projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:                                   codecs,
		EntitySetEnumerationEvaluators:           enumeration,
		CandidateVisibilityEvaluators:            visibility,
		KindDefinednessEvaluators:                definedness,
		MemberOfEvaluators:                       memberOf,
		KindClassificationEvaluators:             classification,
		ReferenceDesignationResolutionEvaluators: referenceDesignationResolution,
		ClaimInterpretationEvaluators:            claimInterpretation,
		ClaimMeasurementEvaluators:               claimMeasurement,
		ClaimEvaluationEvaluators:                claimEvaluation,
		EpistemeConstitutionEvaluators:           epistemeConstitution,
		MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
			mechanism,
		},
		RegistrationPolicies: append(
			[]recordmembershipregistration.RegistrationArtifactV1(nil),
			policies...,
		),
	}, nil
}

func buildHistoricalMemberOfRegistry(
	requirements projecttypeenv.CompositeRuntimeRequirementSet,
	policies []recordmembershipregistration.RegistrationArtifactV1,
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry,
	visibility typedmemoryevaluation.CandidateVisibilityRegistry,
	definedness typedmemoryevaluation.KindDefinednessRegistry,
	identity typedmemoryevaluation.MechanismIdentity,
) (memberofruntime.Registry, error) {
	if !runtimeSetRequiresContract(
		requirements,
		projecttypeenv.RuntimeMechanismContractMemberOf,
	) {
		return memberofruntime.NewRegistry(nil)
	}
	recordCarrier, err := typedmemoryevaluation.NewRecordMembershipRegistry(identity)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	recordPolicy, err := policyForRule(policies, recordRule)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	recordEngine, err := projectmemory.NewRecordMembershipAdmissionEngineBuilder().
		SetEntitySetEnumeration(enumeration).
		SetCandidateVisibility(visibility).
		SetKindDefinedness(definedness).
		SetRecordCarrierMembership(recordCarrier).
		SetRegistrationPolicy(recordPolicy).
		Build()
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	recordFamily, err := projectmemory.NewRecordMembershipEvaluatorRegistry(
		recordEngine,
	)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	entityEngine, err := projectmemory.NewProjectEntityMembershipAdmissionEngineBuilder().
		SetEntitySetEnumeration(enumeration).
		SetCandidateVisibility(visibility).
		SetKindDefinedness(definedness).
		SetMechanismIdentity(identity).
		Build()
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	entityFamily, err := projectmemory.NewProjectEntityMembershipEvaluatorRegistry(
		entityEngine,
	)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	carrierEngines, err := buildCarrierFamilyEngines(
		policies,
		enumeration,
		visibility,
		definedness,
		identity,
	)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	carrierFamilies, err :=
		projectmemory.NewCarrierFamilyMembershipEvaluatorRegistry(carrierEngines)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	registrations := append(
		recordFamily.Registrations(),
		entityFamily.Registrations()...,
	)
	registrations = append(registrations, carrierFamilies.Registrations()...)
	registry, err := memberofruntime.NewRegistry(registrations)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	return registry, nil
}

func runtimeSetRequiresContract(
	requirements projecttypeenv.CompositeRuntimeRequirementSet,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
) bool {
	for _, requirement := range requirements.Requirements() {
		if requirement.InvocationContract() == contract {
			return true
		}
	}
	return false
}

func buildEvaluatorRegistryForContract[Input, Output any](
	requirements []projecttypeenv.CompositeRuntimeRequirement,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	identity typedmemoryevaluation.MechanismIdentity,
	factory func(
		typedmemory.RuleRef,
		typedmemoryevaluation.MechanismIdentity,
	) (typedmemoryevaluation.Registry[Input, Output], error),
) (typedmemoryevaluation.Registry[Input, Output], error) {
	registrations := make(
		[]typedmemoryevaluation.Registration[Input, Output],
		0,
	)
	for _, requirement := range requirements {
		if requirement.InvocationContract() != contract {
			continue
		}
		rule, present := requirement.Rule()
		if !present {
			return typedmemoryevaluation.Registry[Input, Output]{}, fmt.Errorf(
				"Local-Practice evaluator contract %q has no RuleRef",
				contract.String(),
			)
		}
		registry, err := factory(rule, identity)
		if err != nil {
			return typedmemoryevaluation.Registry[Input, Output]{}, fmt.Errorf(
				"construct Local-Practice %q evaluator registry for %q: %w",
				contract.String(),
				rule.String(),
				err,
			)
		}
		registrations = append(
			registrations,
			registry.Registrations()...,
		)
	}
	registry, err := typedmemoryevaluation.NewRegistry(registrations)
	if err != nil {
		return typedmemoryevaluation.Registry[Input, Output]{}, fmt.Errorf(
			"seal Local-Practice %q evaluator registry: %w",
			contract.String(),
			err,
		)
	}
	return registry, nil
}

func buildCodecRegistry(
	base typeenv.BaseTypeEnvArtifact,
	environment typedmemory.TypeEnv,
) (typedmemory.CodecRegistry, error) {
	_, baseCodecs, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		environment.Ref(),
	)
	if err != nil {
		return typedmemory.CodecRegistry{}, fmt.Errorf(
			"lower base codecs at Local-Practice C: %w",
			err,
		)
	}
	suite, err := typedmemorycandidatecodec.NewSuite(environment.ValueShapes())
	if err != nil {
		return typedmemory.CodecRegistry{}, fmt.Errorf(
			"construct Local-Practice candidate codec suite: %w",
			err,
		)
	}
	local := map[string]typedmemory.CodecImplementation{
		"Haft.Codec.TextV1":                 suite.Text(),
		"Haft.Codec.EvidencePolarityV1":     suite.EvidencePolarity(),
		"Haft.Codec.CanonicalInstantV1":     suite.CanonicalInstant(),
		"Haft.Codec.EvidenceUseQualifierV1": suite.EvidenceUseQualifier(),
		"Haft.Codec.PerformedIntervalV1":    suite.PerformedInterval(),
		"Haft.Codec.CodeAnchorLocatorV1":    suite.CodeAnchorLocator(),
	}
	result := typedmemory.NewCodecRegistry()
	for _, binding := range environment.ValueBindings() {
		implementation, present := baseCodecs.Resolve(binding.Codec())
		if !present {
			implementation, present = local[binding.Codec().ID().String()]
		}
		if !present && binding.Codec().ID().String() == "Haft.Codec.ProjectMemoryReferenceSchemeV1" {
			referenceSchemeCodec, codecErr :=
				projectmemoryreferencescheme.NewTypedValueCodecV1(binding.ValueShape())
			if codecErr != nil {
				return typedmemory.CodecRegistry{}, fmt.Errorf(
					"construct project-memory ReferenceScheme codec: %w",
					codecErr,
				)
			}
			implementation = referenceSchemeCodec
			present = true
		}
		if !present {
			return typedmemory.CodecRegistry{}, fmt.Errorf(
				"Local-Practice codec %q has no installed implementation",
				binding.Codec().ID(),
			)
		}
		result, err = result.Register(binding.Codec(), implementation)
		if err != nil {
			return typedmemory.CodecRegistry{}, fmt.Errorf(
				"register Local-Practice codec %q: %w",
				binding.Codec().ID(),
				err,
			)
		}
	}
	return result, nil
}

func mechanismIdentity(
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
) (typedmemoryevaluation.MechanismIdentity, error) {
	identity := mechanism.Identity()
	result, err := typedmemoryevaluation.NewMechanismIdentity(
		identity.Artifact(),
		identity.Edition(),
		identity.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	)
	if err != nil {
		return typedmemoryevaluation.MechanismIdentity{}, fmt.Errorf(
			"construct Local-Practice evaluator identity: %w",
			err,
		)
	}
	return result, nil
}

func buildPrerequisiteRegistries(
	requirements projecttypeenv.CompositeRuntimeRequirementSet,
	identity typedmemoryevaluation.MechanismIdentity,
) (
	typedmemoryevaluation.EntitySetEnumerationRegistry,
	typedmemoryevaluation.CandidateVisibilityRegistry,
	typedmemoryevaluation.KindDefinednessRegistry,
	error,
) {
	enumeration := typedmemoryevaluation.EntitySetEnumerationRegistry{}
	visibility := typedmemoryevaluation.CandidateVisibilityRegistry{}
	definedness := typedmemoryevaluation.KindDefinednessRegistry{}
	for _, requirement := range requirements.Requirements() {
		rule, present := requirement.Rule()
		if !present {
			continue
		}
		var err error
		switch requirement.InvocationContract() {
		case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
			var one typedmemoryevaluation.EntitySetEnumerationRegistry
			one, err = typedmemoryevaluation.NewEntitySetEnumerationRegistry(
				rule,
				identity,
			)
			if err == nil {
				enumeration, err = typedmemoryevaluation.NewRegistry(
					append(enumeration.Registrations(), one.Registrations()...),
				)
			}
		case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
			var one typedmemoryevaluation.CandidateVisibilityRegistry
			one, err = typedmemoryevaluation.NewCandidateVisibilityRegistry(
				rule,
				identity,
			)
			if err == nil {
				visibility, err = typedmemoryevaluation.NewRegistry(
					append(visibility.Registrations(), one.Registrations()...),
				)
			}
		case projecttypeenv.RuntimeMechanismContractKindDefinedness:
			var one typedmemoryevaluation.KindDefinednessRegistry
			one, err = typedmemoryevaluation.NewKindDefinednessRegistry(
				rule,
				identity,
			)
			if err == nil {
				definedness, err = typedmemoryevaluation.NewRegistry(
					append(definedness.Registrations(), one.Registrations()...),
				)
			}
		}
		if err != nil {
			return typedmemoryevaluation.EntitySetEnumerationRegistry{},
				typedmemoryevaluation.CandidateVisibilityRegistry{},
				typedmemoryevaluation.KindDefinednessRegistry{},
				err
		}
	}
	return enumeration, visibility, definedness, nil
}

func buildCarrierFamilyEngines(
	policies []recordmembershipregistration.RegistrationArtifactV1,
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry,
	visibility typedmemoryevaluation.CandidateVisibilityRegistry,
	definedness typedmemoryevaluation.KindDefinednessRegistry,
	identity typedmemoryevaluation.MechanismIdentity,
) ([]projectmemory.CarrierFamilyMembershipAdmissionEngine, error) {
	inputs := []struct {
		builder projectmemory.CarrierFamilyMembershipAdmissionEngineBuilder
		rule    typedmemory.RuleRef
	}{
		{
			builder: projectmemory.NewCarrierEditionMembershipAdmissionEngineBuilder(),
			rule:    carrierfamily.CarrierEditionEvaluatorRuleV1(),
		},
		{
			builder: projectmemory.NewProjectClaimMembershipAdmissionEngineBuilder(),
			rule:    carrierfamily.ProjectClaimEvaluatorRuleV1(),
		},
		{
			builder: projectmemory.NewPerformedWorkOccurrenceMembershipAdmissionEngineBuilder(),
			rule:    carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1(),
		},
		{
			builder: projectmemory.NewCodeAnchorMembershipAdmissionEngineBuilder(),
			rule:    carrierfamily.CodeAnchorEvaluatorRuleV1(),
		},
	}
	engines := make(
		[]projectmemory.CarrierFamilyMembershipAdmissionEngine,
		0,
		len(inputs),
	)
	for _, input := range inputs {
		policy, err := policyForRule(policies, input.rule)
		if err != nil {
			return nil, err
		}
		engine, err := input.builder.
			SetEntitySetEnumeration(enumeration).
			SetCandidateVisibility(visibility).
			SetKindDefinedness(definedness).
			SetMechanismIdentity(identity).
			SetRegistrationPolicy(policy).
			Build()
		if err != nil {
			return nil, err
		}
		engines = append(engines, engine)
	}
	return engines, nil
}

func policyForRule(
	policies []recordmembershipregistration.RegistrationArtifactV1,
	rule typedmemory.RuleRef,
) (recordmembershipregistration.RegistrationArtifactV1, error) {
	var selected []recordmembershipregistration.RegistrationArtifactV1
	for _, policy := range policies {
		if policy.Evaluator().Rule() == rule {
			selected = append(selected, policy)
		}
	}
	if len(selected) != 1 {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"Local-Practice registration policies for %q = %d, want exactly 1",
			rule.String(),
			len(selected),
		)
	}
	return selected[0], nil
}

func cloneInstalledRuntime(
	input projecttypeenvruntime.InstalledRuntimeRegistryInput,
) projecttypeenvruntime.InstalledRuntimeRegistryInput {
	return projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:                                   input.Codecs,
		EntitySetEnumerationEvaluators:           input.EntitySetEnumerationEvaluators.Clone(),
		CandidateVisibilityEvaluators:            input.CandidateVisibilityEvaluators.Clone(),
		KindDefinednessEvaluators:                input.KindDefinednessEvaluators.Clone(),
		MemberOfEvaluators:                       input.MemberOfEvaluators.Clone(),
		KindClassificationEvaluators:             input.KindClassificationEvaluators.Clone(),
		ReferenceDesignationResolutionEvaluators: input.ReferenceDesignationResolutionEvaluators.Clone(),
		ClaimInterpretationEvaluators:            input.ClaimInterpretationEvaluators.Clone(),
		ClaimMeasurementEvaluators:               input.ClaimMeasurementEvaluators.Clone(),
		ClaimEvaluationEvaluators:                input.ClaimEvaluationEvaluators.Clone(),
		EpistemeConstitutionEvaluators:           input.EpistemeConstitutionEvaluators.Clone(),
		MechanismCatalogs: append(
			[]runtimemechanism.RuntimeMechanismArtifactV1(nil),
			input.MechanismCatalogs...,
		),
		RegistrationPolicies: append(
			[]recordmembershipregistration.RegistrationArtifactV1(nil),
			input.RegistrationPolicies...,
		),
	}
}
