package typedmemory

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// This corpus is a source-conformance oracle over the public typed-memory
// core. It deliberately does not claim that the production FPF compiler
// already lowers every U.Kind and relation below into the selected project
// TypeEnv. The exact source snapshot is pinned so a changed FPF publication
// forces a fresh semantic review instead of silently preserving these rules.
const (
	sourceConformanceFPFRevision   = "59c455329e49715c64dc1b16f22c1efba6a3cf6f"
	sourceConformanceFPFSpecDigest = "sha256:5f3381f62d6c652cf57489a2d83f12aa2d1ebec47ccb156e9499c9e96d675bcf"
)

type sourceConformanceSourceRange struct {
	pattern string
	start   uint64
	end     uint64
	digest  string
}

// sourceConformanceNetworkSelection is an oracle-only value for E.18.NET's
// four A.22 identity discriminators. It is deliberately not a RelationSpec:
// direct-member lists create no membership occurrence, and every selected
// cross-flow relation remains owned by its direct governing pattern.
type sourceConformanceNetworkSelection struct {
	directMembers              []ByReferenceCandidate
	selectedCrossFlowRelations []ByReferenceCandidate
	selectedNetworkConstraints []ByReferenceCandidate
	useFrame                   ByReferenceCandidate
	returnCondition            ByReferenceCandidate
}

var sourceConformanceSourceRanges = []sourceConformanceSourceRange{
	{pattern: "A.1", start: 1488, end: 1848, digest: "sha256:161d9ae6e9ae47f679c80b3c73f687f61213e220faffa41a966a7fcad6c3c908"},
	{pattern: "A.2.1", start: 3404, end: 3752, digest: "sha256:6b585cfd060767d855376f479289c50139a0c8e38df2b6c68fe890333e0ea91d"},
	{pattern: "A.2.8", start: 6688, end: 7016, digest: "sha256:1024331b6962afb75d01df2673b2cf9e92997da6e63e6455ad58f22a168df705"},
	{pattern: "A.2.8.PER", start: 7017, end: 7332, digest: "sha256:333f52fe0a2bfbdd0ec2d992eb84857728053500f141670b2088f2015e0a4017"},
	{pattern: "A.3.1", start: 7954, end: 8412, digest: "sha256:4fcfa192bbb823cf1d39d31095e40f40da073fcf63b5e795e9b8ea08fa2142ad"},
	{pattern: "A.3.2", start: 8619, end: 8950, digest: "sha256:737181fac774febb83353201179958910a4bb7a2e847969deae27689b53ea56f"},
	{pattern: "A.3.4", start: 9275, end: 9665, digest: "sha256:ac471a764c25596eebb0ebc59b2afc8898a69d9d2086c4a271bdd2a7d7e3ad6c"},
	{pattern: "A.6.REL", start: 12165, end: 12557, digest: "sha256:14c72fb1432631a55f9d9f130bab9104b8bc1e2df07899143ca4ef5b963c0be7"},
	{pattern: "A.6.0", start: 12558, end: 12974, digest: "sha256:6de81923e4712c4a375e4cf200d7a8fce212f9a435d6ee331448185115e6c3ac"},
	{pattern: "A.6.1", start: 12975, end: 13495, digest: "sha256:2e2a64099a23babd1e3990489e58d01cb4fea1a5ba716020075d366adb81244e"},
	{pattern: "A.6.P.WMR", start: 16515, end: 16945, digest: "sha256:b9dc3b3f800c04b0aee527975bf6560f58744b69e956569a7adc200254b7daa6"},
	{pattern: "A.6.5", start: 19176, end: 19546, digest: "sha256:a840af771d466e34710f0d676d3e6a23ad114b9a13408b5c52dab30d9d2559ef"},
	{pattern: "A.10", start: 22645, end: 23024, digest: "sha256:7ad6698498ffec3f0c9bb4868eac013521c7e92406c456132c0262fab9e2a586"},
	{pattern: "A.14", start: 24197, end: 24556, digest: "sha256:c6efcb8d43bc4f96a95dd6b9a1200b70b122d353d148bb2fc681cdf56dac404a"},
	{pattern: "A.15.1", start: 24956, end: 25550, digest: "sha256:b5eb867468054a9f9d6f977603ce46f2135258c84ac9361831d203d52065ee2a"},
	{pattern: "A.15.2", start: 25551, end: 25862, digest: "sha256:33b85898c4d0a3a2fe45b245bdbcc4f621b74de0258ba7a8d0d9f460fa981da4"},
	{pattern: "A.15.3", start: 25863, end: 26181, digest: "sha256:b81586c6895f1029c170e81ce459fdb361ba41b51d1563a3ec92cdac95a6359a"},
	{pattern: "A.15.PROD", start: 27882, end: 28443, digest: "sha256:9532791b4879c4713398bb2309c980c6e3ea959dfb08ad1cb31e16f90118a39f"},
	{pattern: "A.18", start: 29608, end: 29759, digest: "sha256:b868bdbd0f495a6b65ea7721b2e2cbc6e6236c056bdb7fa7aac51ffe7ab91e4e"},
	{pattern: "A.19.UNM", start: 32778, end: 33194, digest: "sha256:6ab506c49ee47472f0228a395cebe3cf7f8bd91296bdff796206e24a21fd7679"},
	{pattern: "A.19.UINDM", start: 33195, end: 33482, digest: "sha256:6f2f511e4432ccb29a8b8756d4f4ecc274b4d7c56126e29e3af7a10f93684115"},
	{pattern: "A.19.USCM", start: 33483, end: 33811, digest: "sha256:15cc863820ee814e55d3eaf15b289c3f2d177a9d8183b95d5bdf0fee1c0a7681"},
	{pattern: "A.19.ULSAM", start: 33812, end: 34107, digest: "sha256:df1186f4faf6af34b7bfbf2e446c3ddc9a0de9fe267fa885296fb76149100e62"},
	{pattern: "A.19.CPM", start: 34108, end: 34486, digest: "sha256:163ad299dbd0b09bb55e3603c7676556f2262b2e44056933a2100636e1490843"},
	{pattern: "A.19.SelectorMechanism", start: 34487, end: 34907, digest: "sha256:4cba7db789460a08dcf8e8f57a5f80e18852133240883ef3d40615f30af9261e"},
	{pattern: "C.2.1", start: 41905, end: 42497, digest: "sha256:0dc904083ece1841ebc3426025e0d003fac47c1336b8167f4aba49f6e55b4a20"},
	{pattern: "C.11", start: 46888, end: 47607, digest: "sha256:39b9dffcb6cb17573de3fa61a0c1485aca44912759b47cd9cae1cbea3125788f"},
	{pattern: "C.16", start: 48064, end: 48359, digest: "sha256:8d3094e45d3f66b45fc41b3c17d80b795d858d011da933236d2609909efe1ba0"},
	{pattern: "C.18", start: 49866, end: 50168, digest: "sha256:f236017dab806e47b33956f503364dcbf77bedd7ca40f758d8b41493ae3e72f4"},
	{pattern: "C.22", start: 51645, end: 52028, digest: "sha256:13ec100015b89aeeb25127421d8d38ead2992ba8bf27dcc6a896eb8d7800a83c"},
	{pattern: "C.22.2", start: 52511, end: 53168, digest: "sha256:e406442362b1d6e9007499c195a993a31656043ddf253d1dd2b66aaf3ca82ea7"},
	{pattern: "C.28", start: 56495, end: 57251, digest: "sha256:e6241de7b037be154e367ff8b5621ce71be5345f0a619c434bc05cacff0866f2"},
	{pattern: "C.30", start: 58619, end: 59326, digest: "sha256:adef223abb4d30c4a52c4ba1306efd5b17ac5004f0300fc3bae3790e8a74c908"},
	{pattern: "E.11.PUR", start: 79499, end: 79787, digest: "sha256:35cf14bfe57c2973781d4ff3f9ef5b8e173a297b731a2027b42b85a49a00755f"},
	{pattern: "E.17", start: 82706, end: 83294, digest: "sha256:3c67a248dea5efc1c4abcd57486497c5933cc5ad3db34e2d3841008582468718"},
	{pattern: "E.18", start: 85557, end: 86151, digest: "sha256:4017d6001db978b9c080e0fe98d1168f65611052883b169be54c55aeaf87e961"},
	{pattern: "E.18.NET", start: 87420, end: 87837, digest: "sha256:7208116c9a3ffa22d94f24f91088d4e733789a7414f1b4b9b130e7396376a295"},
	{pattern: "E.24.PUB", start: 91920, end: 92201, digest: "sha256:4b1a831eb690d99800ca3a2ef395a6623c3ba00398f3fc2dd4ad54604a3301cf"},
	{pattern: "E.24.UK", start: 92202, end: 92678, digest: "sha256:1d5efe3a4c4fbef8152d63de2607dac3b27813aaf3820db6fb0dfbf990400cb7"},
	{pattern: "F.6", start: 94536, end: 94897, digest: "sha256:6c660424932acc75c6c608f9ef7409f7b49a148abbd802def01c404ea0bad333"},
}

type sourceConformanceSource struct {
	location   SourceLocation
	provenance CompilerDerivedProvenance
}

type sourceConformanceRelationSpec struct {
	key     string
	pattern string
	slots   []sourceConformanceSlotSpec
}

type sourceConformanceSlotSpec struct {
	name string
	kind string
}

// These test-local RelationSignatures preserve only the exact participant-kind
// boundary needed by the category-error case. They are not substitutes for a
// complete source declaration and do not encode obtaining, occurrence identity,
// assertion modality, or compiler-lowering support.
var sourceConformanceRelationSpecs = []sourceConformanceRelationSpec{
	{
		key:     "system_struct_part_of",
		pattern: "A.14",
		slots: []sourceConformanceSlotSpec{
			{name: "Whole", kind: "U.System"},
			{name: "Part", kind: "U.System"},
		},
	},
	{
		key:     "episteme_constitution",
		pattern: "C.2.1",
		slots: []sourceConformanceSlotSpec{
			{name: "ClaimGraphSlot", kind: "U.ClaimGraph"},
			{name: "EntityOfConcernSlot", kind: "U.Entity"},
			{name: "ReferenceSchemeSlot", kind: "U.ReferenceScheme"},
		},
	},
	{
		key:     "episteme_publication",
		pattern: "E.24.PUB",
		// E.24.PUB owns the exact five-participant availability relation.
		// Publication Work and raw audience members are deliberately absent:
		// neither participates in this occurrence.
		slots: []sourceConformanceSlotSpec{
			{name: "SelectedEpistemeEditionSlot", kind: "U.Episteme"},
			{name: "AudienceDeclarationSlot", kind: "U.Episteme"},
			{name: "BoundedUseDeclarationSlot", kind: "U.Episteme"},
			{name: "PublicationFormSlot", kind: "PublicationForm"},
			{name: "PresentationCarrierSlot", kind: "U.PresentationCarrier"},
		},
	},
	{
		key:     "publication_form_expression",
		pattern: "E.24.PUB",
		slots: []sourceConformanceSlotSpec{
			{name: "ExpressedEpistemeEditionSlot", kind: "U.Episteme"},
			{name: "PublicationFormSlot", kind: "PublicationForm"},
			{name: "BoundedUseDeclarationSlot", kind: "U.Episteme"},
		},
	},
	{
		key:     "publication_form_bearing",
		pattern: "E.24.PUB",
		slots: []sourceConformanceSlotSpec{
			{name: "PresentationCarrierSlot", kind: "U.PresentationCarrier"},
			{name: "BornePublicationFormSlot", kind: "PublicationForm"},
		},
	},
	{
		key:     "simple_system_role_assignment",
		pattern: "A.2.1",
		// This oracle-local signature represents one directly declared simple
		// species under U.SystemRoleAssignment. The family has no permissive root
		// signature. Taxonomy, scheme, and interval stay in assertion or
		// interpretation content rather than becoming generic participants.
		slots: []sourceConformanceSlotSpec{
			{name: "HolderSystemSlot", kind: "U.System"},
			{name: "AssignedSystemRoleKindSlot", kind: "G3.AssignedSystemRoleKind"},
		},
	},
	{
		key:     "work_performed_under_assignment",
		pattern: "F.6",
		slots: []sourceConformanceSlotSpec{
			{name: "WorkOccurrenceSlot", kind: "U.Work"},
			{name: "SystemRoleAssignmentSlot", kind: "U.SystemRoleAssignment"},
		},
	},
	{
		key:     "episteme_empirical_grounding",
		pattern: "C.2.1",
		// covered=C, its claim-to-world mappings, and the maximal interval are
		// predicate and occurrence-identity content. They are not a third
		// world-side participant and therefore are not another SlotSpec here.
		slots: []sourceConformanceSlotSpec{
			{name: "GroundedEpistemeSlot", kind: "U.Episteme"},
			{name: "GroundingHolonSlot", kind: "U.Holon"},
		},
	},
	{
		key:     "target_effect_evidence",
		pattern: "A.10",
		// This oracle-local tuple checks only the minimum categories used by the
		// case. It is not A.10's complete evidence path and type admission does
		// not establish evidence use, provenance, currentness, or obtaining.
		slots: []sourceConformanceSlotSpec{
			{name: "TargetEffectSlot", kind: "U.Entity"},
			{name: "EvidenceProducingWorkSlot", kind: "U.Work"},
			{name: "EvidenceEpistemeSlot", kind: "U.Episteme"},
		},
	},
	{
		key:     "governed_work_order",
		pattern: "A.15.2",
		// This oracle-local predicate category stands for the exact local
		// PlanItem dependency predicate or condition carried in plan content.
		// A MethodDescription, display order, row order, and graph edge do not
		// establish that ordering.
		slots: []sourceConformanceSlotSpec{
			{name: "OrderPredicateSlot", kind: "G3.PlanItemOrderingPredicate"},
			{name: "WorkPlanSlot", kind: "U.WorkPlan"},
		},
	},
	{
		key:     "relation_occurrence_designation",
		pattern: "A.6.REL",
		// Oracle-local unary category boundary only. A designation is a value or
		// reference in a receiving episteme, not a source-declared unary relation.
		slots: []sourceConformanceSlotSpec{
			{name: "OccurrenceSlot", kind: "U.Relation"},
		},
	},
	{
		key:     "typed_relation_assertion",
		pattern: "A.6.REL",
		// Oracle-local comparison tuple. A relational assertion is episteme
		// content, not a source-declared binary world-side relation.
		slots: []sourceConformanceSlotSpec{
			{name: "AssertionEpistemeSlot", kind: "U.Episteme"},
			{name: "RelationKindSlot", kind: "U.Entity"},
		},
	},
	{
		key:     "episteme_edition",
		pattern: "C.2.1",
		slots: []sourceConformanceSlotSpec{
			{name: "EarlierEpistemeSlot", kind: "U.Episteme"},
			{name: "LaterEpistemeSlot", kind: "U.Episteme"},
		},
	},
	{
		key:     "system_episteme_use_boundary",
		pattern: "A.1",
		// This is an oracle-local two-position boundary, not a claim that A.1
		// declares a universal relation kind with this name. It keeps the acting
		// system separate from an episteme used in that action.
		slots: []sourceConformanceSlotSpec{
			{name: "ActingSystemSlot", kind: "U.System"},
			{name: "UsedEpistemeSlot", kind: "U.Episteme"},
		},
	},
	{
		key:     "flow_structure_valuation",
		pattern: "E.18",
		// This oracle-local boundary keeps the two categories distinct. Type
		// admission alone does not prove that the valuation resolves to this
		// exact TFS or only to its internal U.Transfer occurrences.
		slots: []sourceConformanceSlotSpec{
			{name: "TransformationFlowStructureSlot", kind: "G3.TransformationFlowStructure"},
			{name: "FlowValuationSlot", kind: "G3.FlowValuation"},
		},
	},
	{
		key:     "comparison_comparator_boundary",
		pattern: "A.19.CPM",
		// This oracle-local tuple checks only that the comparison subject and
		// explicit ComparatorSpec remain different categories, and that changing
		// the comparator changes this partial coordinate. It is not the identity
		// of an actual Compare application. That identity additionally binds the
		// profile pair, CN/CG specs, claim scope and selected context slices,
		// optional A.19 predicate, reference scheme and plane, evaluation window,
		// policies, eligibility, and the separate result binding.
		slots: []sourceConformanceSlotSpec{
			{name: "ComparisonSlot", kind: "G3.Comparison"},
			{name: "ComparatorSpecSlot", kind: "G3.ComparatorSpec"},
		},
	},
	{
		key:     "architecture_relation_boundary",
		pattern: "C.30",
		// Oracle-local minimum participant boundary for the direct current
		// ArchitectureRelation. Type admission does not establish that the
		// structure is constituted, its selected subject relations obtain, or
		// this architecture relation obtains.
		slots: []sourceConformanceSlotSpec{
			{name: "ArchitectureBearingHolonSlot", kind: "U.Holon"},
			{name: "SelectedStructureSlot", kind: "G3.SelectedStructure"},
		},
	},
	{
		key:     "architecture_claim_boundary",
		pattern: "C.30",
		// Oracle-local constitution tuple for current ArchitectureClaim. The
		// claim remains a C.2.1 episteme with one exact EntityOfConcern and
		// effective reference scheme; it is not the direct relation.
		slots: []sourceConformanceSlotSpec{
			{name: "ArchitectureClaimSlot", kind: "G3.ArchitectureClaim"},
			{name: "EntityOfConcernSlot", kind: "U.Entity"},
			{name: "ReferenceSchemeSlot", kind: "U.ReferenceScheme"},
		},
	},
	{
		key:     "architecture_candidate_basis",
		pattern: "C.30",
		// This oracle-local tuple mirrors the required current C.30
		// ArchitectureCandidateMove content used by this case. Optional current
		// relation, selected-structure, and ArchitectureClaim refs are omitted.
		// This is not a source relation, C.32 lowering, choice, or authorization.
		slots: []sourceConformanceSlotSpec{
			{name: "CandidateSetOrArchiveSlot", kind: "G3.ArchitectureCandidateSetOrArchive"},
			{name: "DescribedHolonSlot", kind: "U.Holon"},
			{name: "CandidateStructureSlot", kind: "G3.CandidateStructure"},
			{name: "AffectedCharacteristicSlot", kind: "G3.Characteristic"},
			{name: "CandidateMoveClaimSlot", kind: "G3.ArchitectureCandidateMoveClaim"},
			{name: "StopConditionSlot", kind: "G3.ArchitectureCandidateStopCondition"},
		},
	},
	{
		key:     "permission_exercise_boundary",
		pattern: "A.2.8.PER",
		// G3.PermissionGrant is an oracle-local narrowing of the source's literal
		// U.Relation slot to an occurrence that resolves to the exact grant kind.
		// Type admission remains weaker than the direct permission predicate.
		slots: []sourceConformanceSlotSpec{
			{name: "ExercisingWorkSlot", kind: "U.Work"},
			{name: "GrantedPermissionOccurrenceSlot", kind: "G3.PermissionGrant"},
		},
	},
	{
		key:     "wmr_recovery_boundary",
		pattern: "A.6.P.WMR",
		// Oracle-local method-output comparison tuple. WMR mints no U-kind,
		// world-side relation, or result record; admission here only checks that
		// overloaded cues and the four recovery-result categories stay distinct.
		slots: []sourceConformanceSlotSpec{
			{name: "ExactSubjectSlot", kind: "U.Entity"},
			{name: "ExactRelatedObjectSlot", kind: "U.Entity"},
			{name: "DirectGovernorSlot", kind: "G3.DirectGovernor"},
			{name: "ModalityAndExtentSlot", kind: "G3.RelationModality"},
			{name: "PolaritySlot", kind: "G3.RelationPolarity"},
			{name: "RecoveryPostureSlot", kind: "G3.RecoveryPosture"},
		},
	},
	{
		key:     "work_occurrence_basis",
		pattern: "A.15.1",
		// This oracle-local admission boundary includes the A.13 performer core,
		// including its obtaining assignment, plus the independent A.15.1 Work
		// basis. It does not encode the optional F.6 assignment-bound attribution.
		slots: []sourceConformanceSlotSpec{
			{name: "WorkOccurrenceSlot", kind: "U.Work"},
			{name: "ActualPerformerSystemSlot", kind: "U.System"},
			{name: "AgencyBasisAssignmentSlot", kind: "U.SystemRoleAssignment"},
			{name: "EnactedMethodSlot", kind: "U.Method"},
			{name: "TemporalExtentSlot", kind: "G3.TemporalExtent"},
			{name: "ContainingSystemSlot", kind: "U.System"},
		},
	},
	{
		key:     "workplan_intention_boundary",
		pattern: "A.15.2",
		// Oracle-local distinction tuple, not a membership-complete WorkPlan
		// record: reference scheme, horizon, and full PlanItem content remain
		// outside this bounded category-error case.
		slots: []sourceConformanceSlotSpec{
			{name: "WorkPlanSlot", kind: "U.WorkPlan"},
			{name: "PresentEntityOfConcernSlot", kind: "G3.IdentifiedPresentEoC"},
			{name: "FuturePerformanceDesignatorSlot", kind: "G3.FuturePerformanceDesignator"},
			{name: "PlanItemContentSlot", kind: "G3.PlanItemContent"},
		},
	},
	{
		key:     "production_claim_boundaries",
		pattern: "A.15.PROD",
		// These are simultaneous comparison positions in the oracle, not one
		// omnibus production relation or a claim that all branches obtain.
		slots: []sourceConformanceSlotSpec{
			{name: "ProductionWorkClaimSlot", kind: "G3.ProductionWorkClaim"},
			{name: "EntityInceptionClaimSlot", kind: "G3.EntityInceptionClaim"},
			{name: "ProductionCompletionClaimSlot", kind: "G3.ProductionCompletionClaim"},
		},
	},
	{
		key:     "mechanism_recognition_basis",
		pattern: "A.6.1",
		// Oracle-local recognition-input tuple. A.6.1's content outline is not a
		// mandatory record, direct relation, or set of SlotKinds.
		slots: []sourceConformanceSlotSpec{
			{name: "MechanismEpistemeSlot", kind: "U.Mechanism"},
			{name: "ClaimGraphSlot", kind: "U.ClaimGraph"},
			{name: "EntityOfConcernSlot", kind: "U.Entity"},
			{name: "ReferenceSchemeSlot", kind: "U.ReferenceScheme"},
			{name: "OperationAlgebraSlot", kind: "G3.OperationAlgebra"},
			{name: "LawSetSlot", kind: "G3.LawSet"},
			{name: "AdmissibilityConditionsSlot", kind: "G3.AdmissibilityConditions"},
			{name: "ApplicabilitySlot", kind: "G3.Applicability"},
		},
	},
	{
		key:     "system_recognition_basis",
		pattern: "A.1",
		// Oracle-local evaluator-input tuple. A type-level admission of these
		// inputs is not a positive U.System classification or world-side relation;
		// the kind-specific criterion and evaluation remain separate.
		slots: []sourceConformanceSlotSpec{
			{name: "ExactCandidateSlot", kind: "U.Entity"},
			{name: "ExactConstituentsSlot", kind: "G3.ConstituentSet"},
			{name: "ConstructivePartRelationsSlot", kind: "G3.PartRelationSet"},
			{name: "AssemblySlot", kind: "G3.Assembly"},
			{name: "ReidentificationRuleSlot", kind: "U.Episteme"},
			{name: "WholeLevelCharacteristicSlot", kind: "G3.WholeLevelCharacteristic"},
			{name: "LargerAssemblyCompatibilitySlot", kind: "G3.LargerAssemblyCompatibility"},
			{name: "SystemSpecificConditionSlot", kind: "G3.SystemActingCondition"},
		},
	},
	{
		key:     "planned_filling_boundary",
		pattern: "A.15.3",
		// Oracle-local comparison tuple over WorkPlan ClaimGraph content.
		// SlotFillingsPlanItem is not a U-kind, world-side relation, or record.
		slots: []sourceConformanceSlotSpec{
			{name: "WorkPlanSlot", kind: "U.WorkPlan"},
			{name: "IntendedPerformanceDesignatorSlot", kind: "G3.FuturePerformanceDesignator"},
			{name: "TargetDeclarationMemberSlot", kind: "G3.PlannedDeclarationMember"},
			{name: "PlannedValueSlot", kind: "G3.PlannedValue"},
		},
	},
	{
		key:     "actual_operation_binding_boundary",
		pattern: "A.6.1",
		// The direct declaration-local binding participants are the exact
		// application occurrence and the exact bound value. The declaration
		// member governs the predicate; the binding is the occurrence itself.
		slots: []sourceConformanceSlotSpec{
			{name: "ActualApplicationSlot", kind: "G3.ActualOperationApplication"},
			{name: "ActualBoundValueSlot", kind: "G3.ActualBoundValue"},
		},
	},
}

type sourceConformanceFixture struct {
	environment TypeEnv
	context     BoundedContextRef
	refKind     RefKindRef
	kinds       map[string]KindDefinition
	valueKinds  map[string]ValueKindRef
	signatures  map[string]RelationSignature
	sources     map[string]sourceConformanceSource
	testing     *testing.T
}

type sourceConformanceSnapshot struct {
	revision       GraphRevision
	environment    TypeEnv
	context        BoundedContextRef
	references     map[string]EntityID
	actualKinds    map[string]KindID
	notMembers     map[string]DeclarationProvenance
	assertions     map[string]AssertionState
	assertionCount uint64
	testing        *testing.T
}

type sourceConformanceBinding struct {
	slot   string
	filler ByReferenceCandidate
}

func TestSourceConformanceCategoryErrorCorpus(t *testing.T) {
	fixture := newSourceConformanceFixture(t)
	assertSourceConformanceDeclarationsAreSourceBacked(t, fixture)

	t.Run("episteme constitution does not make the episteme a structural system part", func(t *testing.T) {
		snapshot := fixture.snapshot()
		whole := fixture.reference(t, snapshot, "case1-system", "U.System")
		description := fixture.reference(t, snapshot, "case1-work-plan-episteme", "U.WorkPlan")
		claimGraph := fixture.reference(t, snapshot, "case1-claim-graph", "U.ClaimGraph")
		referenceScheme := fixture.reference(t, snapshot, "case1-reference-scheme", "U.ReferenceScheme")

		invalid := fixture.validate(t, snapshot, "system_struct_part_of", []sourceConformanceBinding{
			{slot: "Whole", filler: whole},
			{slot: "Part", filler: description},
		})
		assertSourceConformanceRejected(t, invalid, ValidationInvalid, DiagnosticEntityKindMismatch)

		valid := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
			{slot: "ClaimGraphSlot", filler: claimGraph},
			{slot: "EntityOfConcernSlot", filler: whole},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceTypeAdmission(t, valid)
	})

	t.Run("simple system-role assignment species needs holder and assigned-kind participants", func(t *testing.T) {
		snapshot := fixture.snapshot()
		system := fixture.reference(t, snapshot, "case2-system", "U.System")
		role := fixture.reference(t, snapshot, "case2-role", "G3.AssignedSystemRoleKind")
		unknown := fixture.reference(t, snapshot, "case2-unclassified-holder", "")

		invalid := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
			{slot: "HolderSystemSlot", filler: role},
			{slot: "AssignedSystemRoleKindSlot", filler: role},
		})
		assertSourceConformanceRejected(t, invalid, ValidationInvalid, DiagnosticEntityKindMismatch)

		valid := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
			{slot: "HolderSystemSlot", filler: system},
			{slot: "AssignedSystemRoleKindSlot", filler: role},
		})
		assertSourceConformanceTypeAdmission(t, valid)

		underdetermined := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
			{slot: "HolderSystemSlot", filler: unknown},
			{slot: "AssignedSystemRoleKindSlot", filler: role},
		})
		assertSourceConformanceRejected(t, underdetermined, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
	})

	t.Run("work plan remains an episteme and is not performed work", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"U.Method",
			"U.MethodDescription",
			"U.WorkPlan",
			"U.Work",
		})
		snapshot := fixture.snapshot()
		plan := fixture.reference(t, snapshot, "case3-plan", "U.WorkPlan")
		work := fixture.reference(t, snapshot, "case3-work", "U.Work")
		assignment := fixture.reference(t, snapshot, "case3-role-assignment", "U.SystemRoleAssignment")

		invalid := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: plan},
			{slot: "SystemRoleAssignmentSlot", filler: assignment},
		})
		assertSourceConformanceRejected(t, invalid, ValidationInvalid, DiagnosticEntityKindMismatch)
		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
				{slot: "WorkOccurrenceSlot", filler: work},
				{slot: "SystemRoleAssignmentSlot", filler: assignment},
			}),
		)
		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "episteme_empirical_grounding", []sourceConformanceBinding{
				{slot: "GroundedEpistemeSlot", filler: plan},
				{slot: "GroundingHolonSlot", filler: work},
			}),
		)
	})

	t.Run("episteme publication occurrence form and carrier stay distinct", func(t *testing.T) {
		snapshot := fixture.snapshot()
		carrier := fixture.reference(t, snapshot, "case4-carrier", "U.PresentationCarrier")
		episteme := fixture.reference(t, snapshot, "case4-episteme", "U.Episteme")
		audienceDeclaration := fixture.reference(t, snapshot, "case4-audience-declaration", "U.Episteme")
		boundedUseDeclaration := fixture.reference(t, snapshot, "case4-bounded-use-declaration", "U.Episteme")
		claimGraph := fixture.reference(t, snapshot, "case4-claim-graph", "U.ClaimGraph")
		referenceScheme := fixture.reference(t, snapshot, "case4-reference-scheme", "U.ReferenceScheme")
		publicationOccurrence := fixture.reference(t, snapshot, "case4-publication-occurrence", "G3.PublicationOccurrence")
		publicationForm := fixture.reference(t, snapshot, "case4-publication-form", "PublicationForm")

		if _, rejectedKindWasReintroduced := fixture.kinds["U.EpistemePublication"]; rejectedKindWasReintroduced {
			t.Fatal("source oracle reintroduced rejected U.EpistemePublication kind")
		}
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"U.Episteme",
			"U.ClaimGraph",
			"G3.PublicationOccurrence",
			"U.PresentationCarrier",
			"PublicationForm",
		})

		invalid := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
			{slot: "ClaimGraphSlot", filler: carrier},
			{slot: "EntityOfConcernSlot", filler: episteme},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceRejected(t, invalid, ValidationInvalid, DiagnosticEntityKindMismatch)

		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
				{slot: "ClaimGraphSlot", filler: claimGraph},
				{slot: "EntityOfConcernSlot", filler: episteme},
				{slot: "ReferenceSchemeSlot", filler: referenceScheme},
			}),
		)

		wrongPublication := fixture.validate(t, snapshot, "episteme_publication", []sourceConformanceBinding{
			{slot: "SelectedEpistemeEditionSlot", filler: publicationForm},
			{slot: "AudienceDeclarationSlot", filler: audienceDeclaration},
			{slot: "BoundedUseDeclarationSlot", filler: boundedUseDeclaration},
			{slot: "PublicationFormSlot", filler: publicationForm},
			{slot: "PresentationCarrierSlot", filler: carrier},
		})
		assertSourceConformanceRejected(t, wrongPublication, ValidationInvalid, DiagnosticEntityKindMismatch)

		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "episteme_publication", []sourceConformanceBinding{
				{slot: "SelectedEpistemeEditionSlot", filler: episteme},
				{slot: "AudienceDeclarationSlot", filler: audienceDeclaration},
				{slot: "BoundedUseDeclarationSlot", filler: boundedUseDeclaration},
				{slot: "PublicationFormSlot", filler: publicationForm},
				{slot: "PresentationCarrierSlot", filler: carrier},
			}),
		)

		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "publication_form_expression", []sourceConformanceBinding{
				{slot: "ExpressedEpistemeEditionSlot", filler: episteme},
				{slot: "PublicationFormSlot", filler: publicationForm},
				{slot: "BoundedUseDeclarationSlot", filler: boundedUseDeclaration},
			}),
		)

		wrongBearing := fixture.validate(t, snapshot, "publication_form_bearing", []sourceConformanceBinding{
			{slot: "PresentationCarrierSlot", filler: episteme},
			{slot: "BornePublicationFormSlot", filler: publicationForm},
		})
		assertSourceConformanceRejected(t, wrongBearing, ValidationInvalid, DiagnosticEntityKindMismatch)
		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "publication_form_bearing", []sourceConformanceBinding{
				{slot: "PresentationCarrierSlot", filler: carrier},
				{slot: "BornePublicationFormSlot", filler: publicationForm},
			}),
		)

		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "relation_occurrence_designation", []sourceConformanceBinding{
				{slot: "OccurrenceSlot", filler: publicationOccurrence},
			}),
		)
	})

	t.Run("task signature and problem card are not work", func(t *testing.T) {
		snapshot := fixture.snapshot()
		signature := fixture.reference(t, snapshot, "case5-task-signature", "C.22.TaskSignature")
		problem := fixture.reference(t, snapshot, "case5-problem-card", "C.22.2.ProblemCard")
		assignment := fixture.reference(t, snapshot, "case5-role-assignment", "U.SystemRoleAssignment")

		for label, candidate := range map[string]ByReferenceCandidate{
			"task-signature": signature,
			"problem-card":   problem,
		} {
			t.Run(label, func(t *testing.T) {
				performed := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
					{slot: "WorkOccurrenceSlot", filler: candidate},
					{slot: "SystemRoleAssignmentSlot", filler: assignment},
				})
				assertSourceConformanceRejected(t, performed, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}
	})

	t.Run("work is not the grounded episteme", func(t *testing.T) {
		snapshot := fixture.snapshot()
		work := fixture.reference(t, snapshot, "case6-observation-work", "U.Work")
		claim := fixture.reference(t, snapshot, "case6-grounded-episteme", "U.Episteme")

		invalid := fixture.validate(t, snapshot, "episteme_empirical_grounding", []sourceConformanceBinding{
			{slot: "GroundedEpistemeSlot", filler: work},
			{slot: "GroundingHolonSlot", filler: work},
		})
		assertSourceConformanceRejected(t, invalid, ValidationInvalid, DiagnosticEntityKindMismatch)
		assertSourceConformanceTypeAdmission(
			t,
			fixture.validate(t, snapshot, "episteme_empirical_grounding", []sourceConformanceBinding{
				{slot: "GroundedEpistemeSlot", filler: claim},
				{slot: "GroundingHolonSlot", filler: work},
			}),
		)
	})

	// Cases 7-14 mirror the master-plan order. Every positive below is only a
	// participant-kind admission; assertSourceConformanceTypeAdmission deliberately keeps
	// relation obtaining, occurrence identity, truth, authority, and Work out.
	t.Run("successful command or closed run is not target-effect evidence by itself", func(t *testing.T) {
		snapshot := fixture.snapshot()
		targetEffect := fixture.reference(t, snapshot, "case7-target-effect", "U.Entity")
		evidence := fixture.reference(t, snapshot, "case7-evidence-episteme", "U.Episteme")
		workOnlyCandidates := []struct {
			label     string
			candidate ByReferenceCandidate
		}{
			{
				label:     "successful-command",
				candidate: fixture.reference(t, snapshot, "case7-successful-command", "U.Work"),
			},
			{
				label:     "closed-run",
				candidate: fixture.reference(t, snapshot, "case7-closed-run", "U.Work"),
			},
		}
		for _, candidate := range workOnlyCandidates {
			t.Run(candidate.label, func(t *testing.T) {
				verdict := fixture.validate(t, snapshot, "target_effect_evidence", []sourceConformanceBinding{
					{slot: "TargetEffectSlot", filler: targetEffect},
					{slot: "EvidenceProducingWorkSlot", filler: candidate.candidate},
					{slot: "EvidenceEpistemeSlot", filler: candidate.candidate},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		explicitEvidenceRelation := fixture.validate(t, snapshot, "target_effect_evidence", []sourceConformanceBinding{
			{slot: "TargetEffectSlot", filler: targetEffect},
			{slot: "EvidenceProducingWorkSlot", filler: workOnlyCandidates[0].candidate},
			{slot: "EvidenceEpistemeSlot", filler: evidence},
		})
		assertSourceConformanceTypeAdmission(t, explicitEvidenceRelation)
	})

	t.Run("display and retrieval order do not become causal or work order", func(t *testing.T) {
		snapshot := fixture.snapshot()
		plan := fixture.reference(t, snapshot, "case8-work-plan", "U.WorkPlan")
		unsupportedOrderCues := []struct {
			label string
			kind  string
		}{
			{label: "graph-direction", kind: "G3.GraphDirection"},
			{label: "timestamp", kind: "G3.Timestamp"},
			{label: "retrieval-rank", kind: "G3.RetrievalRank"},
			{label: "readme-mantra-walkthrough-order", kind: "G3.PresentationOrder"},
		}
		for _, cue := range unsupportedOrderCues {
			t.Run(cue.label, func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case8-"+cue.label, cue.kind)
				verdict := fixture.validate(t, snapshot, "governed_work_order", []sourceConformanceBinding{
					{slot: "OrderPredicateSlot", filler: candidate},
					{slot: "WorkPlanSlot", filler: plan},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		orderPredicate := fixture.reference(t, snapshot, "case8-order-predicate", "G3.PlanItemOrderingPredicate")
		explicitOrderPredicate := fixture.validate(t, snapshot, "governed_work_order", []sourceConformanceBinding{
			{slot: "OrderPredicateSlot", filler: orderPredicate},
			{slot: "WorkPlanSlot", filler: plan},
		})
		assertSourceConformanceTypeAdmission(t, explicitOrderPredicate)
	})

	t.Run("assertion and representation objects are not obtaining occurrences", func(t *testing.T) {
		snapshot := fixture.snapshot()
		wrongOccurrenceCandidates := []struct {
			label string
			kind  string
		}{
			{label: "relational-assertion", kind: "G3.RelationalAssertion"},
			{label: "representation-row", kind: "G3.RepresentationRow"},
			{label: "graph-edge", kind: "G3.GraphEdge"},
			{label: "identifier", kind: "G3.Identifier"},
			{label: "reference", kind: "G3.Reference"},
			{label: "reifier", kind: "G3.Reifier"},
		}
		for _, candidate := range wrongOccurrenceCandidates {
			t.Run(candidate.label, func(t *testing.T) {
				filler := fixture.reference(t, snapshot, "case9-"+candidate.label, candidate.kind)
				verdict := fixture.validate(t, snapshot, "relation_occurrence_designation", []sourceConformanceBinding{
					{slot: "OccurrenceSlot", filler: filler},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		assertion := fixture.reference(t, snapshot, "case9-typed-assertion", "G3.RelationalAssertion")
		relationKind := fixture.reference(t, snapshot, "case9-relation-kind", "U.Entity")
		typedAssertion := fixture.validate(t, snapshot, "typed_relation_assertion", []sourceConformanceBinding{
			{slot: "AssertionEpistemeSlot", filler: assertion},
			{slot: "RelationKindSlot", filler: relationKind},
		})
		assertSourceConformanceTypeAdmission(t, typedAssertion)
	})

	t.Run("slot carrier field and designation stay distinct from the participant", func(t *testing.T) {
		snapshot := fixture.snapshot()
		wrongParticipantCandidates := []struct {
			label string
			kind  string
		}{
			{label: "slot-spec", kind: "G3.SlotSpec"},
			{label: "carrier-field", kind: "G3.CarrierField"},
			{label: "participant-designation", kind: "G3.ParticipantDesignation"},
		}
		role := fixture.reference(t, snapshot, "case11-role", "G3.AssignedSystemRoleKind")
		for _, candidate := range wrongParticipantCandidates {
			t.Run(candidate.label, func(t *testing.T) {
				filler := fixture.reference(t, snapshot, "case11-"+candidate.label, candidate.kind)
				verdict := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
					{slot: "HolderSystemSlot", filler: filler},
					{slot: "AssignedSystemRoleKindSlot", filler: role},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		designation := fixture.referenceTo(
			t,
			snapshot,
			"case11-system-designation",
			"case11-world-system",
			"U.System",
		)
		resolved := snapshot.ResolveReference(designation.Reference(), fixture.context)
		participant, ok := resolved.(ResolvedStrongReference)
		if !ok {
			t.Fatalf("designation resolution = %T; want ResolvedStrongReference", resolved)
		}
		if participant.Entity().String() != "entity:g3:case11-world-system" {
			t.Fatalf("designation resolved %s", participant.Entity().String())
		}
		if designation.Reference().ReferenceKey() == participant.Entity().String() {
			t.Fatal("designation reference collapsed into its world-side participant")
		}
		resolvedAssignment := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
			{slot: "HolderSystemSlot", filler: designation},
			{slot: "AssignedSystemRoleKindSlot", filler: role},
		})
		assertSourceConformanceTypeAdmission(t, resolvedAssignment)
	})

	t.Run("episteme identity needs all three intrinsic discriminators", func(t *testing.T) {
		snapshot := fixture.snapshot()
		claimGraph := fixture.reference(t, snapshot, "case12-claim-graph", "U.ClaimGraph")
		entityOfConcern := fixture.reference(t, snapshot, "case12-entity-of-concern", "U.Entity")
		referenceScheme := fixture.reference(t, snapshot, "case12-reference-scheme", "U.ReferenceScheme")
		unknownClaimGraph := fixture.reference(t, snapshot, "case12-unknown-claim-graph", "")
		unknownEntity := fixture.reference(t, snapshot, "case12-unknown-eoc", "")
		unknownScheme := fixture.reference(t, snapshot, "case12-unknown-scheme", "")
		missingBasisCases := []struct {
			label    string
			bindings []sourceConformanceBinding
		}{
			{
				label: "claim-graph-membership",
				bindings: []sourceConformanceBinding{
					{slot: "ClaimGraphSlot", filler: unknownClaimGraph},
					{slot: "EntityOfConcernSlot", filler: entityOfConcern},
					{slot: "ReferenceSchemeSlot", filler: referenceScheme},
				},
			},
			{
				label: "entity-of-concern-membership",
				bindings: []sourceConformanceBinding{
					{slot: "ClaimGraphSlot", filler: claimGraph},
					{slot: "EntityOfConcernSlot", filler: unknownEntity},
					{slot: "ReferenceSchemeSlot", filler: referenceScheme},
				},
			},
			{
				label: "reference-scheme-membership",
				bindings: []sourceConformanceBinding{
					{slot: "ClaimGraphSlot", filler: claimGraph},
					{slot: "EntityOfConcernSlot", filler: entityOfConcern},
					{slot: "ReferenceSchemeSlot", filler: unknownScheme},
				},
			},
		}
		for _, missing := range missingBasisCases {
			t.Run(missing.label, func(t *testing.T) {
				verdict := fixture.validate(t, snapshot, "episteme_constitution", missing.bindings)
				assertSourceConformanceRejected(t, verdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
			})
		}

		completeConstitution := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
			{slot: "ClaimGraphSlot", filler: claimGraph},
			{slot: "EntityOfConcernSlot", filler: entityOfConcern},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceTypeAdmission(t, completeConstitution)

		alternateClaimGraph := fixture.reference(t, snapshot, "case12-alternate-claim-graph", "U.ClaimGraph")
		alternateEntity := fixture.reference(t, snapshot, "case12-alternate-eoc", "U.Entity")
		alternateScheme := fixture.reference(t, snapshot, "case12-alternate-scheme", "U.ReferenceScheme")
		changedDiscriminators := []struct {
			label           string
			claimGraph      ByReferenceCandidate
			entityOfConcern ByReferenceCandidate
			referenceScheme ByReferenceCandidate
		}{
			{
				label:           "claim-graph",
				claimGraph:      alternateClaimGraph,
				entityOfConcern: entityOfConcern,
				referenceScheme: referenceScheme,
			},
			{
				label:           "entity-of-concern",
				claimGraph:      claimGraph,
				entityOfConcern: alternateEntity,
				referenceScheme: referenceScheme,
			},
			{
				label:           "reference-scheme",
				claimGraph:      claimGraph,
				entityOfConcern: entityOfConcern,
				referenceScheme: alternateScheme,
			},
		}
		coordinates := []string{sourceConformanceCoordinate(claimGraph, entityOfConcern, referenceScheme)}
		for _, changed := range changedDiscriminators {
			t.Run("changed-"+changed.label, func(t *testing.T) {
				changedConstitution := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
					{slot: "ClaimGraphSlot", filler: changed.claimGraph},
					{slot: "EntityOfConcernSlot", filler: changed.entityOfConcern},
					{slot: "ReferenceSchemeSlot", filler: changed.referenceScheme},
				})
				assertSourceConformanceTypeAdmission(t, changedConstitution)
			})
			coordinate := sourceConformanceCoordinate(
				changed.claimGraph,
				changed.entityOfConcern,
				changed.referenceScheme,
			)
			coordinates = append(coordinates, coordinate)
		}
		assertSourceConformanceDistinctCoordinates(t, coordinates)
	})

	t.Run("neighboring grounding view publication carrier and time do not change episteme identity", func(t *testing.T) {
		snapshot := fixture.snapshot()
		claimGraph := fixture.reference(t, snapshot, "case13-claim-graph", "U.ClaimGraph")
		episteme := fixture.reference(t, snapshot, "case13-episteme", "U.Episteme")
		referenceScheme := fixture.reference(t, snapshot, "case13-reference-scheme", "U.ReferenceScheme")
		neighbors := []struct {
			label     string
			candidate ByReferenceCandidate
		}{
			{label: "grounding-holon", candidate: fixture.reference(t, snapshot, "case13-grounding", "U.Work")},
			{label: "view", candidate: fixture.reference(t, snapshot, "case13-view", "U.Episteme")},
			{label: "publication-occurrence", candidate: fixture.reference(t, snapshot, "case13-publication", "U.Relation")},
			{label: "carrier", candidate: fixture.reference(t, snapshot, "case13-carrier", "U.PresentationCarrier")},
			{label: "timestamp", candidate: fixture.reference(t, snapshot, "case13-timestamp", "G3.Timestamp")},
		}
		for _, neighbor := range neighbors {
			t.Run(neighbor.label+"-as-identity", func(t *testing.T) {
				verdict := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
					{slot: "ClaimGraphSlot", filler: neighbor.candidate},
					{slot: "EntityOfConcernSlot", filler: episteme},
					{slot: "ReferenceSchemeSlot", filler: referenceScheme},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}
		constitution := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
			{slot: "ClaimGraphSlot", filler: claimGraph},
			{slot: "EntityOfConcernSlot", filler: episteme},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceTypeAdmission(t, constitution)

		groundingRelation := fixture.validate(t, snapshot, "episteme_empirical_grounding", []sourceConformanceBinding{
			{slot: "GroundedEpistemeSlot", filler: episteme},
			{slot: "GroundingHolonSlot", filler: neighbors[0].candidate},
		})
		assertSourceConformanceTypeAdmission(t, groundingRelation)
	})

	t.Run("edition continuity is not inferred from source or revision cues", func(t *testing.T) {
		snapshot := fixture.snapshot()
		earlier := fixture.reference(t, snapshot, "case14-earlier-episteme", "U.Episteme")
		later := fixture.reference(t, snapshot, "case14-later-episteme", "U.Episteme")
		unsupportedContinuityCues := []struct {
			label string
			kind  string
		}{
			{label: "source-pin", kind: "G3.SourcePin"},
			{label: "revision-order", kind: "G3.RevisionOrder"},
			{label: "content-similarity", kind: "G3.ContentSimilarity"},
			{label: "performed-revision-work", kind: "U.Work"},
		}
		for _, cue := range unsupportedContinuityCues {
			t.Run(cue.label, func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case14-"+cue.label, cue.kind)
				verdict := fixture.validate(t, snapshot, "episteme_edition", []sourceConformanceBinding{
					{slot: "EarlierEpistemeSlot", filler: earlier},
					{slot: "LaterEpistemeSlot", filler: candidate},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		unknownLater := fixture.reference(t, snapshot, "case14-unknown-later", "")
		underdetermined := fixture.validate(t, snapshot, "episteme_edition", []sourceConformanceBinding{
			{slot: "EarlierEpistemeSlot", filler: earlier},
			{slot: "LaterEpistemeSlot", filler: unknownLater},
		})
		assertSourceConformanceRejected(t, underdetermined, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		// C.2.1 has exactly two world-side edition participants. The source-use,
		// revision work, change facts, evidence, and direct continuation predicate
		// are deliberately not participant slots. This type-level admission proves
		// neither that the predicate obtains nor that an occurrence exists.
		editionParticipants := fixture.validate(t, snapshot, "episteme_edition", []sourceConformanceBinding{
			{slot: "EarlierEpistemeSlot", filler: earlier},
			{slot: "LaterEpistemeSlot", filler: later},
		})
		assertSourceConformanceTypeAdmission(t, editionParticipants)
	})

	t.Run("program code model and repository carriers do not become acting systems", func(t *testing.T) {
		snapshot := fixture.snapshot()
		usedEpisteme := fixture.reference(t, snapshot, "case15-used-episteme", "U.Episteme")
		nonSystemCues := []struct {
			label string
			kind  string
		}{
			{label: "claim-bearing-program-description", kind: "G3.ClaimBearingProgramDescriptionEpisteme"},
			{label: "code-carrier", kind: "G3.CodeCarrier"},
			{label: "claim-bearing-model", kind: "G3.ClaimBearingModelEpisteme"},
			{label: "repository-carrier", kind: "G3.RepositoryCarrier"},
		}
		for _, cue := range nonSystemCues {
			t.Run(cue.label, func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case15-"+cue.label, cue.kind)
				verdict := fixture.validate(t, snapshot, "system_episteme_use_boundary", []sourceConformanceBinding{
					{slot: "ActingSystemSlot", filler: candidate},
					{slot: "UsedEpistemeSlot", filler: usedEpisteme},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		unknownHolder := fixture.reference(t, snapshot, "case15-unknown-holder", "")
		unknownVerdict := fixture.validate(t, snapshot, "system_episteme_use_boundary", []sourceConformanceBinding{
			{slot: "ActingSystemSlot", filler: unknownHolder},
			{slot: "UsedEpistemeSlot", filler: usedEpisteme},
		})
		assertSourceConformanceRejected(t, unknownVerdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		operationalSystem := fixture.reference(t, snapshot, "case15-operational-system", "U.System")
		typedBoundary := fixture.validate(t, snapshot, "system_episteme_use_boundary", []sourceConformanceBinding{
			{slot: "ActingSystemSlot", filler: operationalSystem},
			{slot: "UsedEpistemeSlot", filler: usedEpisteme},
		})
		assertSourceConformanceTypeAdmission(t, typedBoundary)
	})

	t.Run("target labels role assignment work and entity-of-concern position stay distinct", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"U.System",
			"G3.TargetSystemLabel",
			"G3.SystemOfInterestLabel",
			"U.SystemRoleAssignment",
			"U.Work",
		})
		for _, forbiddenKind := range []string{
			"U.EntityOfConcern",
			"U.TargetSystem",
			"U.SystemOfInterest",
			"U.Production",
		} {
			if _, exists := fixture.kinds[forbiddenKind]; exists {
				t.Fatalf("oracle introduced forbidden shortcut kind %s", forbiddenKind)
			}
		}

		snapshot := fixture.snapshot()
		usedEpisteme := fixture.reference(t, snapshot, "case16-used-episteme", "U.Episteme")
		confusedActors := []struct {
			label string
			kind  string
		}{
			{label: "target-system-label", kind: "G3.TargetSystemLabel"},
			{label: "system-of-interest-label", kind: "G3.SystemOfInterestLabel"},
			{label: "role-assignment", kind: "U.SystemRoleAssignment"},
			{label: "project-work", kind: "U.Work"},
		}
		for _, confused := range confusedActors {
			t.Run(confused.label, func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case16-"+confused.label, confused.kind)
				verdict := fixture.validate(t, snapshot, "system_episteme_use_boundary", []sourceConformanceBinding{
					{slot: "ActingSystemSlot", filler: candidate},
					{slot: "UsedEpistemeSlot", filler: usedEpisteme},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		system := fixture.reference(t, snapshot, "case16-system", "U.System")
		role := fixture.reference(t, snapshot, "case16-role", "G3.AssignedSystemRoleKind")
		referenceScheme := fixture.reference(t, snapshot, "case16-reference-scheme", "U.ReferenceScheme")
		assignment := fixture.validate(t, snapshot, "simple_system_role_assignment", []sourceConformanceBinding{
			{slot: "HolderSystemSlot", filler: system},
			{slot: "AssignedSystemRoleKindSlot", filler: role},
		})
		assertSourceConformanceTypeAdmission(t, assignment)

		claimGraph := fixture.reference(t, snapshot, "case16-claim-graph", "U.ClaimGraph")
		constitution := fixture.validate(t, snapshot, "episteme_constitution", []sourceConformanceBinding{
			{slot: "ClaimGraphSlot", filler: claimGraph},
			{slot: "EntityOfConcernSlot", filler: system},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceTypeAdmission(t, constitution)
	})

	t.Run("one flow structure valuation references network publications work and plans stay distinct", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"G3.TransformationFlowStructure",
			"G3.FlowValuation",
			"G3.FlowPositionRef",
			"G3.SubflowRef",
			"G3.TransformationFlowStructureNetwork",
			"G3.ExposedFlowPositionRef",
			"G3.GraphProjection",
			"G3.TableProjection",
			"U.Work",
			"U.WorkPlan",
		})
		snapshot := fixture.snapshot()
		structure := fixture.reference(t, snapshot, "case17-structure", "G3.TransformationFlowStructure")
		valuation := fixture.reference(t, snapshot, "case17-valuation", "G3.FlowValuation")
		wrongStructureKinds := []string{
			"G3.FlowValuation",
			"G3.FlowPositionRef",
			"G3.SubflowRef",
			"G3.TransformationFlowStructureNetwork",
			"G3.ExposedFlowPositionRef",
			"G3.GraphProjection",
			"G3.TableProjection",
			"U.Work",
			"U.WorkPlan",
		}
		for _, kind := range wrongStructureKinds {
			candidate := fixture.reference(t, snapshot, "case17-structure-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "flow_structure_valuation", []sourceConformanceBinding{
				{slot: "TransformationFlowStructureSlot", filler: candidate},
				{slot: "FlowValuationSlot", filler: valuation},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		wrongValuationKinds := []string{
			"G3.TransformationFlowStructure",
			"G3.FlowPositionRef",
			"G3.SubflowRef",
			"G3.TransformationFlowStructureNetwork",
			"G3.ExposedFlowPositionRef",
			"G3.GraphProjection",
			"G3.TableProjection",
			"U.Work",
			"U.WorkPlan",
		}
		for _, kind := range wrongValuationKinds {
			candidate := fixture.reference(t, snapshot, "case17-valuation-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "flow_structure_valuation", []sourceConformanceBinding{
				{slot: "TransformationFlowStructureSlot", filler: structure},
				{slot: "FlowValuationSlot", filler: candidate},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		typedBoundary := fixture.validate(t, snapshot, "flow_structure_valuation", []sourceConformanceBinding{
			{slot: "TransformationFlowStructureSlot", filler: structure},
			{slot: "FlowValuationSlot", filler: valuation},
		})
		assertSourceConformanceTypeAdmission(t, typedBoundary)
	})

	t.Run("measurement comparison and selection categories remain distinct", func(t *testing.T) {
		categoryKinds := []string{
			"G3.Characteristic",
			"G3.Scale",
			"G3.MeasuredValue",
			"G3.Indicator",
			"G3.Score",
			"G3.ComparatorSpec",
			"G3.Normalization",
			"G3.Fold",
			"G3.Comparison",
			"G3.Selection",
			"G3.ArchiveFront",
		}
		assertSourceConformanceDistinctKinds(t, fixture, categoryKinds)
		snapshot := fixture.snapshot()
		comparison := fixture.reference(t, snapshot, "case18-comparison", "G3.Comparison")
		comparatorSpec := fixture.reference(t, snapshot, "case18-comparator-spec", "G3.ComparatorSpec")
		for _, kind := range categoryKinds {
			if kind == "G3.Comparison" {
				continue
			}
			candidate := fixture.reference(t, snapshot, "case18-comparison-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "comparison_comparator_boundary", []sourceConformanceBinding{
				{slot: "ComparisonSlot", filler: candidate},
				{slot: "ComparatorSpecSlot", filler: comparatorSpec},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		typedComparison := fixture.validate(t, snapshot, "comparison_comparator_boundary", []sourceConformanceBinding{
			{slot: "ComparisonSlot", filler: comparison},
			{slot: "ComparatorSpecSlot", filler: comparatorSpec},
		})
		assertSourceConformanceTypeAdmission(t, typedComparison)

		changedComparator := fixture.reference(t, snapshot, "case18-changed-comparator", "G3.ComparatorSpec")
		originalCoordinate := sourceConformancePartialComparisonCoordinate(comparison, comparatorSpec)
		changedCoordinate := sourceConformancePartialComparisonCoordinate(comparison, changedComparator)
		if originalCoordinate == changedCoordinate {
			t.Fatal("changed ComparatorSpec preserved the old partial comparison coordinate")
		}
	})

	t.Run("palette front evaluation recommendation choice authority work and effect are not one result", func(t *testing.T) {
		resultKinds := []string{
			"G3.CandidatePalette",
			"G3.ArchiveFront",
			"G3.EvaluationResult",
			"G3.Recommendation",
			"G3.Choice",
			"G3.DecisionRecord",
			"U.Work",
			"G3.TargetEffect",
		}
		assertSourceConformanceDistinctKinds(t, fixture, resultKinds)
		snapshot := fixture.snapshot()
		assignment := fixture.reference(t, snapshot, "case19-role-assignment", "U.SystemRoleAssignment")
		for _, kind := range resultKinds {
			if kind == "U.Work" {
				continue
			}
			candidate := fixture.reference(t, snapshot, "case19-work-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
				{slot: "WorkOccurrenceSlot", filler: candidate},
				{slot: "SystemRoleAssignmentSlot", filler: assignment},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		work := fixture.reference(t, snapshot, "case19-performed-work", "U.Work")
		typedWork := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: work},
			{slot: "SystemRoleAssignmentSlot", filler: assignment},
		})
		assertSourceConformanceTypeAdmission(t, typedWork)
	})

	t.Run("selected structure architecture relation claim candidate record ADR and observations stay distinct", func(t *testing.T) {
		architectureKinds := []string{
			"G3.SelectedStructure",
			"G3.CandidateStructure",
			"G3.ArchitectureRelation",
			"G3.ArchitectureClaim",
			"G3.ArchitectureDescription",
			"G3.ArchitectureCandidateSetOrArchive",
			"G3.ArchitectureCandidateMoveClaim",
			"G3.ArchitectureCandidateStopCondition",
			"G3.ADRCarrier",
			"G3.ExpectedStructureClaim",
			"G3.ObservedStructureClaim",
		}
		assertSourceConformanceDistinctKinds(t, fixture, architectureKinds)
		for _, draftPattern := range []string{"C.32", "C.32.PAD", "C.32.ADR"} {
			if _, lowered := fixture.sources[draftPattern]; lowered {
				t.Fatalf("Draft architecture family %s entered the compiled source oracle", draftPattern)
			}
		}
		for _, forbiddenKind := range []string{
			"C.32.ArchitectureCandidate",
			"C.32.PAD.ArchitectureDecision",
			"C.32.ADR.ArchitectureDecisionRecord",
		} {
			if _, lowered := fixture.kinds[forbiddenKind]; lowered {
				t.Fatalf("Draft architecture family introduced kind %s", forbiddenKind)
			}
		}

		snapshot := fixture.snapshot()
		holon := fixture.reference(t, snapshot, "case20-described-holon", "U.System")
		selected := fixture.reference(t, snapshot, "case20-selected-structure", "G3.SelectedStructure")
		relation := fixture.reference(t, snapshot, "case20-architecture-relation", "G3.ArchitectureRelation")
		claim := fixture.reference(t, snapshot, "case20-architecture-claim", "G3.ArchitectureClaim")
		referenceScheme := fixture.reference(t, snapshot, "case20-reference-scheme", "U.ReferenceScheme")
		for _, kind := range architectureKinds {
			if kind == "G3.SelectedStructure" {
				continue
			}
			candidate := fixture.reference(t, snapshot, "case20-selected-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "architecture_relation_boundary", []sourceConformanceBinding{
				{slot: "ArchitectureBearingHolonSlot", filler: holon},
				{slot: "SelectedStructureSlot", filler: candidate},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		unknownStructure := fixture.reference(t, snapshot, "case20-unknown-structure", "")
		unknownVerdict := fixture.validate(t, snapshot, "architecture_relation_boundary", []sourceConformanceBinding{
			{slot: "ArchitectureBearingHolonSlot", filler: holon},
			{slot: "SelectedStructureSlot", filler: unknownStructure},
		})
		assertSourceConformanceRejected(t, unknownVerdict, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		typedArchitecture := fixture.validate(t, snapshot, "architecture_relation_boundary", []sourceConformanceBinding{
			{slot: "ArchitectureBearingHolonSlot", filler: holon},
			{slot: "SelectedStructureSlot", filler: selected},
		})
		assertSourceConformanceTypeAdmission(t, typedArchitecture)

		relationOccurrence := fixture.validate(t, snapshot, "relation_occurrence_designation", []sourceConformanceBinding{
			{slot: "OccurrenceSlot", filler: relation},
		})
		assertSourceConformanceTypeAdmission(t, relationOccurrence)

		relationAsClaim := fixture.validate(t, snapshot, "architecture_claim_boundary", []sourceConformanceBinding{
			{slot: "ArchitectureClaimSlot", filler: relation},
			{slot: "EntityOfConcernSlot", filler: relation},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceRejected(t, relationAsClaim, ValidationInvalid, DiagnosticEntityKindMismatch)

		typedClaim := fixture.validate(t, snapshot, "architecture_claim_boundary", []sourceConformanceBinding{
			{slot: "ArchitectureClaimSlot", filler: claim},
			{slot: "EntityOfConcernSlot", filler: relation},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
		})
		assertSourceConformanceTypeAdmission(t, typedClaim)
	})

	t.Run("method description plan work transformation view model publication carrier and episteme stay distinct", func(t *testing.T) {
		semanticKinds := []string{
			"U.Method",
			"U.MethodDescription",
			"U.WorkPlan",
			"U.Work",
			"U.Transformation",
			"G3.Viewpoint",
			"G3.View",
			"G3.ClaimBearingModelEpisteme",
			"G3.PublicationOccurrence",
			"U.PresentationCarrier",
			"U.Episteme",
		}
		assertSourceConformanceDistinctKinds(t, fixture, semanticKinds)
		snapshot := fixture.snapshot()
		assignment := fixture.reference(t, snapshot, "case21-role-assignment", "U.SystemRoleAssignment")
		for _, kind := range semanticKinds {
			if kind == "U.Work" {
				continue
			}
			candidate := fixture.reference(t, snapshot, "case21-work-as-"+kind, kind)
			verdict := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
				{slot: "WorkOccurrenceSlot", filler: candidate},
				{slot: "SystemRoleAssignmentSlot", filler: assignment},
			})
			assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
		}

		work := fixture.reference(t, snapshot, "case21-work", "U.Work")
		orderPredicate := fixture.reference(t, snapshot, "case21-order-predicate", "G3.PlanItemOrderingPredicate")
		plan := fixture.reference(t, snapshot, "case21-plan", "U.WorkPlan")
		orderedPlan := fixture.validate(t, snapshot, "governed_work_order", []sourceConformanceBinding{
			{slot: "OrderPredicateSlot", filler: orderPredicate},
			{slot: "WorkPlanSlot", filler: plan},
		})
		assertSourceConformanceTypeAdmission(t, orderedPlan)

		publication := fixture.reference(t, snapshot, "case21-publication", "G3.PublicationOccurrence")
		publicationOccurrence := fixture.validate(t, snapshot, "relation_occurrence_designation", []sourceConformanceBinding{
			{slot: "OccurrenceSlot", filler: publication},
		})
		assertSourceConformanceTypeAdmission(t, publicationOccurrence)

		model := fixture.reference(t, snapshot, "case21-model", "G3.ClaimBearingModelEpisteme")
		modelGrounding := fixture.validate(t, snapshot, "episteme_empirical_grounding", []sourceConformanceBinding{
			{slot: "GroundedEpistemeSlot", filler: model},
			{slot: "GroundingHolonSlot", filler: work},
		})
		assertSourceConformanceTypeAdmission(t, modelGrounding)

		performed := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: work},
			{slot: "SystemRoleAssignmentSlot", filler: assignment},
		})
		assertSourceConformanceTypeAdmission(t, performed)
	})

	t.Run("AI suggestion does not become an architecture candidate without exact basis", func(t *testing.T) {
		snapshot := fixture.snapshot()
		candidateSet := fixture.reference(
			t,
			snapshot,
			"case22-candidate-set",
			"G3.ArchitectureCandidateSetOrArchive",
		)
		holon := fixture.reference(t, snapshot, "case22-described-holon", "U.System")
		candidateStructure := fixture.reference(t, snapshot, "case22-candidate-structure", "G3.CandidateStructure")
		characteristic := fixture.reference(t, snapshot, "case22-characteristic", "G3.Characteristic")
		stopCondition := fixture.reference(
			t,
			snapshot,
			"case22-stop-condition",
			"G3.ArchitectureCandidateStopCondition",
		)
		unknownSuggestion := fixture.reference(t, snapshot, "case22-ai-suggestion", "")
		unknownCandidate := fixture.validate(t, snapshot, "architecture_candidate_basis", []sourceConformanceBinding{
			{slot: "CandidateSetOrArchiveSlot", filler: candidateSet},
			{slot: "DescribedHolonSlot", filler: holon},
			{slot: "CandidateStructureSlot", filler: candidateStructure},
			{slot: "AffectedCharacteristicSlot", filler: characteristic},
			{slot: "CandidateMoveClaimSlot", filler: unknownSuggestion},
			{slot: "StopConditionSlot", filler: stopCondition},
		})
		assertSourceConformanceRejected(t, unknownCandidate, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		candidate := fixture.reference(
			t,
			snapshot,
			"case22-candidate-move-claim",
			"G3.ArchitectureCandidateMoveClaim",
		)
		selectedStructure := fixture.reference(t, snapshot, "case22-selected-structure", "G3.SelectedStructure")
		actualAsCandidate := fixture.validate(t, snapshot, "architecture_candidate_basis", []sourceConformanceBinding{
			{slot: "CandidateSetOrArchiveSlot", filler: candidateSet},
			{slot: "DescribedHolonSlot", filler: holon},
			{slot: "CandidateStructureSlot", filler: selectedStructure},
			{slot: "AffectedCharacteristicSlot", filler: characteristic},
			{slot: "CandidateMoveClaimSlot", filler: candidate},
			{slot: "StopConditionSlot", filler: stopCondition},
		})
		assertSourceConformanceRejected(t, actualAsCandidate, ValidationInvalid, DiagnosticEntityKindMismatch)

		incompleteCandidate := fixture.validate(t, snapshot, "architecture_candidate_basis", []sourceConformanceBinding{
			{slot: "CandidateSetOrArchiveSlot", filler: candidateSet},
			{slot: "DescribedHolonSlot", filler: holon},
			{slot: "CandidateStructureSlot", filler: candidateStructure},
			{slot: "AffectedCharacteristicSlot", filler: characteristic},
			{slot: "CandidateMoveClaimSlot", filler: candidate},
		})
		assertSourceConformanceRejected(t, incompleteCandidate, ValidationInvalid, DiagnosticMissingSlot)

		typedCandidateMove := fixture.validate(t, snapshot, "architecture_candidate_basis", []sourceConformanceBinding{
			{slot: "CandidateSetOrArchiveSlot", filler: candidateSet},
			{slot: "DescribedHolonSlot", filler: holon},
			{slot: "CandidateStructureSlot", filler: candidateStructure},
			{slot: "AffectedCharacteristicSlot", filler: characteristic},
			{slot: "CandidateMoveClaimSlot", filler: candidate},
			{slot: "StopConditionSlot", filler: stopCondition},
		})
		assertSourceConformanceTypeAdmission(t, typedCandidateMove)
	})

	t.Run("applicability admission permission gate exercise and work remain distinct", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"G3.Applicability",
			"G3.ConstraintFit",
			"G3.TypedMemoryAdmission",
			"G3.PermissionGrant",
			"G3.PermissionExercise",
			"G3.GateDecision",
			"U.Work",
		})
		snapshot := fixture.snapshot()
		work := fixture.reference(t, snapshot, "case23-work", "U.Work")
		grant := fixture.reference(t, snapshot, "case23-permission-grant", "G3.PermissionGrant")
		wrongGrantCandidates := []struct {
			label string
			kind  string
		}{
			{label: "applicability", kind: "G3.Applicability"},
			{label: "constraint-fit", kind: "G3.ConstraintFit"},
			{label: "typed-memory-admission", kind: "G3.TypedMemoryAdmission"},
			{label: "permission-exercise", kind: "G3.PermissionExercise"},
			{label: "gate-decision", kind: "G3.GateDecision"},
			{label: "performed-work", kind: "U.Work"},
		}
		for _, candidate := range wrongGrantCandidates {
			t.Run(candidate.label+"-as-grant", func(t *testing.T) {
				wrongGrant := fixture.reference(t, snapshot, "case23-"+candidate.label, candidate.kind)
				verdict := fixture.validate(t, snapshot, "permission_exercise_boundary", []sourceConformanceBinding{
					{slot: "ExercisingWorkSlot", filler: work},
					{slot: "GrantedPermissionOccurrenceSlot", filler: wrongGrant},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		exercise := fixture.validate(t, snapshot, "permission_exercise_boundary", []sourceConformanceBinding{
			{slot: "ExercisingWorkSlot", filler: work},
			{slot: "GrantedPermissionOccurrenceSlot", filler: grant},
		})
		assertSourceConformanceTypeAdmission(t, exercise)
		assertSourceConformanceSingleExplicitRelation(t, fixture, exercise, "permission_exercise_boundary")
	})

	t.Run("permission is not commitment permit carrier gate capability readiness work or non-violation", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"U.Commitment",
			"G3.PermissionGrant",
			"G3.PermissionExercise",
			"G3.PermitCarrier",
			"G3.GateDecision",
			"G3.Capability",
			"G3.Readiness",
			"G3.NonViolationFinding",
			"U.Work",
		})
		snapshot := fixture.snapshot()
		work := fixture.reference(t, snapshot, "case24-work", "U.Work")
		wrongPermissionCandidates := []struct {
			label string
			kind  string
		}{
			{label: "may-commitment", kind: "U.Commitment"},
			{label: "permit-carrier", kind: "G3.PermitCarrier"},
			{label: "gate-decision", kind: "G3.GateDecision"},
			{label: "capability", kind: "G3.Capability"},
			{label: "readiness", kind: "G3.Readiness"},
			{label: "non-violation-finding", kind: "G3.NonViolationFinding"},
			{label: "performed-work", kind: "U.Work"},
		}
		for _, candidate := range wrongPermissionCandidates {
			t.Run(candidate.label+"-as-grant", func(t *testing.T) {
				wrongGrant := fixture.reference(t, snapshot, "case24-"+candidate.label, candidate.kind)
				verdict := fixture.validate(t, snapshot, "permission_exercise_boundary", []sourceConformanceBinding{
					{slot: "ExercisingWorkSlot", filler: work},
					{slot: "GrantedPermissionOccurrenceSlot", filler: wrongGrant},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}
	})

	t.Run("network selection identity uses four discriminators and ignores rendering order", func(t *testing.T) {
		snapshot := fixture.snapshot()
		memberOne := fixture.reference(t, snapshot, "case25-member-one", "G3.TransformationFlowStructure")
		memberTwo := fixture.reference(t, snapshot, "case25-member-two", "G3.TransformationFlowStructure")
		nestedNetwork := fixture.reference(
			t,
			snapshot,
			"case25-nested-network",
			"G3.TransformationFlowStructureNetwork",
		)
		crossFlowRelation := fixture.reference(t, snapshot, "case25-cross-flow-relation", "U.Relation")
		constraintOne := fixture.reference(t, snapshot, "case25-constraint-one", "G3.SelectedNetworkConstraint")
		constraintTwo := fixture.reference(t, snapshot, "case25-constraint-two", "G3.SelectedNetworkConstraint")
		useFrame := fixture.reference(t, snapshot, "case25-use-frame", "G3.NetworkUseFrame")
		returnCondition := fixture.reference(t, snapshot, "case25-return-condition", "U.Entity")
		graphOrder := sourceConformanceNetworkSelection{
			directMembers:              []ByReferenceCandidate{nestedNetwork, memberTwo, memberOne},
			selectedCrossFlowRelations: []ByReferenceCandidate{crossFlowRelation},
			selectedNetworkConstraints: []ByReferenceCandidate{constraintTwo, constraintOne},
			useFrame:                   useFrame,
			returnCondition:            returnCondition,
		}
		tableOrder := sourceConformanceNetworkSelection{
			directMembers:              []ByReferenceCandidate{memberOne, nestedNetwork, memberTwo},
			selectedCrossFlowRelations: []ByReferenceCandidate{crossFlowRelation},
			selectedNetworkConstraints: []ByReferenceCandidate{constraintOne, constraintTwo},
			useFrame:                   useFrame,
			returnCondition:            returnCondition,
		}
		graphIdentity := sourceConformanceNetworkSelectionIdentity(t, graphOrder)
		tableIdentity := sourceConformanceNetworkSelectionIdentity(t, tableOrder)
		if graphIdentity != tableIdentity {
			t.Fatal("rendering order changed the four-discriminator network identity")
		}

		otherNestedNetwork := fixture.reference(
			t,
			snapshot,
			"case25-other-nested-network",
			"G3.TransformationFlowStructureNetwork",
		)
		changedMember := tableOrder
		changedMember.directMembers = []ByReferenceCandidate{memberOne, otherNestedNetwork, memberTwo}
		if graphIdentity == sourceConformanceNetworkSelectionIdentity(t, changedMember) {
			t.Fatal("changed direct member preserved the old network identity")
		}

		changedRelationRef := fixture.reference(t, snapshot, "case25-other-cross-flow-relation", "U.Relation")
		changedRelation := tableOrder
		changedRelation.selectedCrossFlowRelations = []ByReferenceCandidate{changedRelationRef}
		if graphIdentity == sourceConformanceNetworkSelectionIdentity(t, changedRelation) {
			t.Fatal("changed selected relation occurrence preserved the old network identity")
		}

		otherConstraint := fixture.reference(t, snapshot, "case25-other-constraint", "G3.SelectedNetworkConstraint")
		changedConstraint := tableOrder
		changedConstraint.selectedNetworkConstraints = []ByReferenceCandidate{constraintOne, otherConstraint}
		if graphIdentity == sourceConformanceNetworkSelectionIdentity(t, changedConstraint) {
			t.Fatal("changed applied network constraint preserved the old network identity")
		}

		otherUseFrame := fixture.reference(t, snapshot, "case25-other-use-frame", "G3.NetworkUseFrame")
		changedUseFrame := tableOrder
		changedUseFrame.useFrame = otherUseFrame
		if graphIdentity == sourceConformanceNetworkSelectionIdentity(t, changedUseFrame) {
			t.Fatal("changed network use frame preserved the old network identity")
		}

		otherReturnCondition := fixture.reference(t, snapshot, "case25-other-return-condition", "U.Entity")
		changedReturnCondition := tableOrder
		changedReturnCondition.returnCondition = otherReturnCondition
		if graphIdentity != sourceConformanceNetworkSelectionIdentity(t, changedReturnCondition) {
			t.Fatal("return condition incorrectly became a fifth network identity discriminator")
		}
	})

	t.Run("overloaded boundary words do not create a governor or erase recovery reasons", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"G3.InputWord",
			"G3.OutputWord",
			"G3.ResultWord",
			"G3.OutcomeWord",
			"G3.DeliverableWord",
			"G3.HandoffWord",
			"G3.DirectGovernor",
			"G3.MissingGovernor",
			"G3.MissingInformation",
			"G3.FactuallyUnsupported",
			"G3.PositivePolarity",
			"G3.GovernedNegativePolarity",
			"G3.AbsentPositiveSupport",
		})
		snapshot := fixture.snapshot()
		subject := fixture.reference(t, snapshot, "case27-subject", "U.Entity")
		related := fixture.reference(t, snapshot, "case27-related-object", "U.Entity")
		modality := fixture.reference(t, snapshot, "case27-modality", "G3.RelationModality")
		polarity := fixture.reference(t, snapshot, "case27-positive-polarity", "G3.PositivePolarity")
		recovery := fixture.reference(t, snapshot, "case27-missing-information", "G3.MissingInformation")
		overloadedWords := []struct {
			label string
			kind  string
		}{
			{label: "input", kind: "G3.InputWord"},
			{label: "output", kind: "G3.OutputWord"},
			{label: "result", kind: "G3.ResultWord"},
			{label: "outcome", kind: "G3.OutcomeWord"},
			{label: "deliverable", kind: "G3.DeliverableWord"},
			{label: "handoff", kind: "G3.HandoffWord"},
		}
		for _, word := range overloadedWords {
			t.Run(word.label+"-as-governor", func(t *testing.T) {
				cue := fixture.reference(t, snapshot, "case27-"+word.label, word.kind)
				verdict := fixture.validate(t, snapshot, "wmr_recovery_boundary", []sourceConformanceBinding{
					{slot: "ExactSubjectSlot", filler: subject},
					{slot: "ExactRelatedObjectSlot", filler: related},
					{slot: "DirectGovernorSlot", filler: cue},
					{slot: "ModalityAndExtentSlot", filler: modality},
					{slot: "PolaritySlot", filler: polarity},
					{slot: "RecoveryPostureSlot", filler: recovery},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		governor := fixture.reference(t, snapshot, "case27-direct-governor", "G3.DirectGovernor")
		absentSupport := fixture.reference(t, snapshot, "case27-absent-positive-support", "G3.AbsentPositiveSupport")
		unsupportedNegative := fixture.validate(t, snapshot, "wmr_recovery_boundary", []sourceConformanceBinding{
			{slot: "ExactSubjectSlot", filler: subject},
			{slot: "ExactRelatedObjectSlot", filler: related},
			{slot: "DirectGovernorSlot", filler: governor},
			{slot: "ModalityAndExtentSlot", filler: modality},
			{slot: "PolaritySlot", filler: absentSupport},
			{slot: "RecoveryPostureSlot", filler: recovery},
		})
		assertSourceConformanceRejected(t, unsupportedNegative, ValidationInvalid, DiagnosticEntityKindMismatch)

		unknownGovernor := fixture.reference(t, snapshot, "case27-unknown-governor", "")
		unknown := fixture.validate(t, snapshot, "wmr_recovery_boundary", []sourceConformanceBinding{
			{slot: "ExactSubjectSlot", filler: subject},
			{slot: "ExactRelatedObjectSlot", filler: related},
			{slot: "DirectGovernorSlot", filler: unknownGovernor},
			{slot: "ModalityAndExtentSlot", filler: modality},
			{slot: "PolaritySlot", filler: polarity},
			{slot: "RecoveryPostureSlot", filler: recovery},
		})
		assertSourceConformanceRejected(t, unknown, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		recoveryKinds := []string{
			"G3.MissingGovernor",
			"G3.MissingInformation",
			"G3.FactuallyUnsupported",
		}
		recoveryCoordinates := make([]string, 0, len(recoveryKinds))
		for index, recoveryKind := range recoveryKinds {
			posture := fixture.reference(t, snapshot, fmt.Sprintf("case27-recovery-%d", index), recoveryKind)
			verdict := fixture.validate(t, snapshot, "wmr_recovery_boundary", []sourceConformanceBinding{
				{slot: "ExactSubjectSlot", filler: subject},
				{slot: "ExactRelatedObjectSlot", filler: related},
				{slot: "DirectGovernorSlot", filler: governor},
				{slot: "ModalityAndExtentSlot", filler: modality},
				{slot: "PolaritySlot", filler: polarity},
				{slot: "RecoveryPostureSlot", filler: posture},
			})
			assertSourceConformanceTypeAdmission(t, verdict)
			recoveryCoordinates = append(recoveryCoordinates, posture.Reference().ReferenceKey())
		}
		assertSourceConformanceDistinctCoordinates(t, recoveryCoordinates)
	})

	t.Run("work records logs field bundles and successful commands are not work occurrences", func(t *testing.T) {
		snapshot := fixture.snapshot()
		actualPerformer := fixture.reference(t, snapshot, "case28-actual-performer", "U.System")
		assignment := fixture.reference(t, snapshot, "case28-covering-assignment", "U.SystemRoleAssignment")
		method := fixture.reference(t, snapshot, "case28-method", "U.Method")
		extent := fixture.reference(t, snapshot, "case28-temporal-extent", "G3.TemporalExtent")
		system := fixture.reference(t, snapshot, "case28-containing-system", "U.System")
		wrongOccurrences := []struct {
			label string
			kind  string
		}{
			{label: "record", kind: "G3.WorkRecord"},
			{label: "log", kind: "G3.WorkLog"},
			{label: "field-bundle", kind: "G3.WorkFieldBundle"},
			{label: "successful-command", kind: "G3.SuccessfulCommand"},
		}
		for _, wrong := range wrongOccurrences {
			t.Run(wrong.label+"-as-work", func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case28-"+wrong.label, wrong.kind)
				verdict := fixture.validate(t, snapshot, "work_occurrence_basis", []sourceConformanceBinding{
					{slot: "WorkOccurrenceSlot", filler: candidate},
					{slot: "ActualPerformerSystemSlot", filler: actualPerformer},
					{slot: "AgencyBasisAssignmentSlot", filler: assignment},
					{slot: "EnactedMethodSlot", filler: method},
					{slot: "TemporalExtentSlot", filler: extent},
					{slot: "ContainingSystemSlot", filler: system},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		unknownWork := fixture.reference(t, snapshot, "case28-unknown-work", "")
		unknown := fixture.validate(t, snapshot, "work_occurrence_basis", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: unknownWork},
			{slot: "ActualPerformerSystemSlot", filler: actualPerformer},
			{slot: "AgencyBasisAssignmentSlot", filler: assignment},
			{slot: "EnactedMethodSlot", filler: method},
			{slot: "TemporalExtentSlot", filler: extent},
			{slot: "ContainingSystemSlot", filler: system},
		})
		assertSourceConformanceRejected(t, unknown, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		work := fixture.reference(t, snapshot, "case28-work", "U.Work")
		identified := fixture.validate(t, snapshot, "work_occurrence_basis", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: work},
			{slot: "ActualPerformerSystemSlot", filler: actualPerformer},
			{slot: "AgencyBasisAssignmentSlot", filler: assignment},
			{slot: "EnactedMethodSlot", filler: method},
			{slot: "TemporalExtentSlot", filler: extent},
			{slot: "ContainingSystemSlot", filler: system},
		})
		assertSourceConformanceTypeAdmission(t, identified)
	})

	t.Run("workplan present entity of concern is not its future performance or plan item", func(t *testing.T) {
		snapshot := fixture.snapshot()
		plan := fixture.reference(t, snapshot, "case29-workplan", "U.WorkPlan")
		present := fixture.reference(t, snapshot, "case29-present-eoc", "G3.IdentifiedPresentEoC")
		future := fixture.reference(t, snapshot, "case29-future-performance", "G3.FuturePerformanceDesignator")
		planItem := fixture.reference(t, snapshot, "case29-plan-item", "G3.PlanItemContent")
		for label, candidate := range map[string]ByReferenceCandidate{
			"future-performance": future,
			"plan-item":          planItem,
		} {
			t.Run(label+"-as-present-eoc", func(t *testing.T) {
				verdict := fixture.validate(t, snapshot, "workplan_intention_boundary", []sourceConformanceBinding{
					{slot: "WorkPlanSlot", filler: plan},
					{slot: "PresentEntityOfConcernSlot", filler: candidate},
					{slot: "FuturePerformanceDesignatorSlot", filler: future},
					{slot: "PlanItemContentSlot", filler: planItem},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		assignment := fixture.reference(t, snapshot, "case29-role-assignment", "U.SystemRoleAssignment")
		futureAsWork := fixture.validate(t, snapshot, "work_performed_under_assignment", []sourceConformanceBinding{
			{slot: "WorkOccurrenceSlot", filler: future},
			{slot: "SystemRoleAssignmentSlot", filler: assignment},
		})
		assertSourceConformanceRejected(t, futureAsWork, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)

		boundary := fixture.validate(t, snapshot, "workplan_intention_boundary", []sourceConformanceBinding{
			{slot: "WorkPlanSlot", filler: plan},
			{slot: "PresentEntityOfConcernSlot", filler: present},
			{slot: "FuturePerformanceDesignatorSlot", filler: future},
			{slot: "PlanItemContentSlot", filler: planItem},
		})
		assertSourceConformanceTypeAdmission(t, boundary)
	})

	t.Run("production work inception completion and neighboring outcomes remain separate", func(t *testing.T) {
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"G3.ProductionWorkClaim",
			"G3.EntityInceptionClaim",
			"G3.ProductionCompletionClaim",
			"U.Work",
			"U.Transformation",
			"G3.DeliveryClaim",
			"G3.AcceptanceClaim",
			"G3.ReleaseClaim",
			"G3.PublicationOccurrence",
			"G3.AvailabilityClaim",
			"G3.TargetEffect",
		})
		snapshot := fixture.snapshot()
		productionWork := fixture.reference(t, snapshot, "case30-production-work-claim", "G3.ProductionWorkClaim")
		inception := fixture.reference(t, snapshot, "case30-inception-claim", "G3.EntityInceptionClaim")
		completion := fixture.reference(t, snapshot, "case30-completion-claim", "G3.ProductionCompletionClaim")
		wrongCompletionCandidates := []struct {
			label string
			kind  string
		}{
			{label: "production-work", kind: "G3.ProductionWorkClaim"},
			{label: "inception", kind: "G3.EntityInceptionClaim"},
			{label: "work", kind: "U.Work"},
			{label: "transformation", kind: "U.Transformation"},
			{label: "delivery", kind: "G3.DeliveryClaim"},
			{label: "acceptance", kind: "G3.AcceptanceClaim"},
			{label: "release", kind: "G3.ReleaseClaim"},
			{label: "publication", kind: "G3.PublicationOccurrence"},
			{label: "availability", kind: "G3.AvailabilityClaim"},
			{label: "target-effect", kind: "G3.TargetEffect"},
		}
		for _, wrong := range wrongCompletionCandidates {
			t.Run(wrong.label+"-as-completion", func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case30-"+wrong.label, wrong.kind)
				verdict := fixture.validate(t, snapshot, "production_claim_boundaries", []sourceConformanceBinding{
					{slot: "ProductionWorkClaimSlot", filler: productionWork},
					{slot: "EntityInceptionClaimSlot", filler: inception},
					{slot: "ProductionCompletionClaimSlot", filler: candidate},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		separateBranches := fixture.validate(t, snapshot, "production_claim_boundaries", []sourceConformanceBinding{
			{slot: "ProductionWorkClaimSlot", filler: productionWork},
			{slot: "EntityInceptionClaimSlot", filler: inception},
			{slot: "ProductionCompletionClaimSlot", filler: completion},
		})
		assertSourceConformanceTypeAdmission(t, separateBranches)
	})

	t.Run("runtime artifacts registries functions arguments and invocations are not U Mechanism", func(t *testing.T) {
		snapshot := fixture.snapshot()
		claimGraph := fixture.reference(t, snapshot, "case31-claim-graph", "U.ClaimGraph")
		entityOfConcern := fixture.reference(t, snapshot, "case31-operation-family", "U.Entity")
		referenceScheme := fixture.reference(t, snapshot, "case31-reference-scheme", "U.ReferenceScheme")
		operationAlgebra := fixture.reference(t, snapshot, "case31-operation-algebra", "G3.OperationAlgebra")
		laws := fixture.reference(t, snapshot, "case31-laws", "G3.LawSet")
		admissibility := fixture.reference(t, snapshot, "case31-admissibility", "G3.AdmissibilityConditions")
		applicability := fixture.reference(t, snapshot, "case31-applicability", "G3.Applicability")
		wrongMechanisms := []struct {
			label string
			kind  string
		}{
			{label: "runtime-artifact", kind: "G3.RuntimeMechanismArtifact"},
			{label: "codec-registry", kind: "G3.CodecRegistry"},
			{label: "evaluator-registry", kind: "G3.EvaluatorRegistry"},
			{label: "implementation-function", kind: "G3.ImplementationFunction"},
			{label: "compatible-argument", kind: "G3.CompatibleArgument"},
			{label: "successful-invocation", kind: "G3.SuccessfulInvocation"},
		}
		for _, wrong := range wrongMechanisms {
			t.Run(wrong.label+"-as-mechanism", func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case31-"+wrong.label, wrong.kind)
				verdict := fixture.validate(t, snapshot, "mechanism_recognition_basis", []sourceConformanceBinding{
					{slot: "MechanismEpistemeSlot", filler: candidate},
					{slot: "ClaimGraphSlot", filler: claimGraph},
					{slot: "EntityOfConcernSlot", filler: entityOfConcern},
					{slot: "ReferenceSchemeSlot", filler: referenceScheme},
					{slot: "OperationAlgebraSlot", filler: operationAlgebra},
					{slot: "LawSetSlot", filler: laws},
					{slot: "AdmissibilityConditionsSlot", filler: admissibility},
					{slot: "ApplicabilitySlot", filler: applicability},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		mechanism := fixture.reference(t, snapshot, "case31-mechanism", "U.Mechanism")
		constitution := fixture.validate(t, snapshot, "mechanism_recognition_basis", []sourceConformanceBinding{
			{slot: "MechanismEpistemeSlot", filler: mechanism},
			{slot: "ClaimGraphSlot", filler: claimGraph},
			{slot: "EntityOfConcernSlot", filler: entityOfConcern},
			{slot: "ReferenceSchemeSlot", filler: referenceScheme},
			{slot: "OperationAlgebraSlot", filler: operationAlgebra},
			{slot: "LawSetSlot", filler: laws},
			{slot: "AdmissibilityConditionsSlot", filler: admissibility},
			{slot: "ApplicabilitySlot", filler: applicability},
		})
		assertSourceConformanceTypeAdmission(t, constitution)
	})

	t.Run("repository profile labels roles capabilities components and host references do not establish U System", func(t *testing.T) {
		snapshot := fixture.snapshot()
		usedEpisteme := fixture.reference(t, snapshot, "case32-used-episteme", "U.Episteme")
		wrongSystems := []struct {
			label string
			kind  string
		}{
			{label: "repository", kind: "G3.RepositoryCarrier"},
			{label: "profile", kind: "G3.Profile"},
			{label: "target-label", kind: "G3.TargetSystemLabel"},
			{label: "role-assignment", kind: "U.SystemRoleAssignment"},
			{label: "capability", kind: "G3.Capability"},
			{label: "component-name", kind: "G3.ComponentName"},
			{label: "host-agent-reference", kind: "G3.HostAgentReference"},
		}
		for _, wrong := range wrongSystems {
			t.Run(wrong.label+"-as-system", func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case32-"+wrong.label, wrong.kind)
				verdict := fixture.validate(t, snapshot, "system_episteme_use_boundary", []sourceConformanceBinding{
					{slot: "ActingSystemSlot", filler: candidate},
					{slot: "UsedEpistemeSlot", filler: usedEpisteme},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		candidate := fixture.reference(t, snapshot, "case32-candidate", "U.Entity")
		constituents := fixture.reference(t, snapshot, "case32-constituents", "G3.ConstituentSet")
		partRelations := fixture.reference(t, snapshot, "case32-part-relations", "G3.PartRelationSet")
		assembly := fixture.reference(t, snapshot, "case32-assembly", "G3.Assembly")
		reidentification := fixture.reference(t, snapshot, "case32-reidentification", "U.Episteme")
		wholeCharacteristic := fixture.reference(t, snapshot, "case32-whole-characteristic", "G3.WholeLevelCharacteristic")
		compatibility := fixture.reference(t, snapshot, "case32-larger-assembly-compatibility", "G3.LargerAssemblyCompatibility")
		systemCondition := fixture.reference(t, snapshot, "case32-system-condition", "G3.SystemActingCondition")
		completeBasis := []sourceConformanceBinding{
			{slot: "ExactCandidateSlot", filler: candidate},
			{slot: "ExactConstituentsSlot", filler: constituents},
			{slot: "ConstructivePartRelationsSlot", filler: partRelations},
			{slot: "AssemblySlot", filler: assembly},
			{slot: "ReidentificationRuleSlot", filler: reidentification},
			{slot: "WholeLevelCharacteristicSlot", filler: wholeCharacteristic},
			{slot: "LargerAssemblyCompatibilitySlot", filler: compatibility},
			{slot: "SystemSpecificConditionSlot", filler: systemCondition},
		}
		recognitionInputs := fixture.validate(t, snapshot, "system_recognition_basis", completeBasis)
		assertSourceConformanceTypeAdmission(t, recognitionInputs)

		missingCompatibility := append([]sourceConformanceBinding(nil), completeBasis[:len(completeBasis)-2]...)
		missingCompatibility = append(missingCompatibility, completeBasis[len(completeBasis)-1])
		missing := fixture.validate(t, snapshot, "system_recognition_basis", missingCompatibility)
		assertSourceConformanceRejected(t, missing, ValidationInvalid, DiagnosticMissingSlot)

		repository := fixture.reference(t, snapshot, "case32-repository-as-constituents", "G3.RepositoryCarrier")
		wrongConstituents := append([]sourceConformanceBinding(nil), completeBasis...)
		wrongConstituents[1] = sourceConformanceBinding{slot: "ExactConstituentsSlot", filler: repository}
		wrongBasis := fixture.validate(t, snapshot, "system_recognition_basis", wrongConstituents)
		assertSourceConformanceRejected(t, wrongBasis, ValidationInvalid, DiagnosticEntityKindMismatch)

		unknownCondition := fixture.reference(t, snapshot, "case32-unknown-condition", "")
		unknownBasis := append([]sourceConformanceBinding(nil), completeBasis...)
		unknownBasis[len(unknownBasis)-1] = sourceConformanceBinding{slot: "SystemSpecificConditionSlot", filler: unknownCondition}
		underdetermined := fixture.validate(t, snapshot, "system_recognition_basis", unknownBasis)
		assertSourceConformanceRejected(t, underdetermined, ValidationUnderdetermined, DiagnosticTypeRuleUnavailable)
	})

	t.Run("planned filling targets exact declaration members and never creates actual binding", func(t *testing.T) {
		snapshot := fixture.snapshot()
		plan := fixture.reference(t, snapshot, "case33-workplan", "U.WorkPlan")
		future := fixture.reference(t, snapshot, "case33-future-designator", "G3.FuturePerformanceDesignator")
		plannedValue := fixture.reference(t, snapshot, "case33-planned-value", "G3.PlannedValue")
		wrongTargets := []struct {
			label string
			kind  string
		}{
			{label: "method-description-field", kind: "G3.MethodDescriptionField"},
			{label: "card-field", kind: "G3.CardField"},
			{label: "schema-field", kind: "G3.SchemaField"},
			{label: "type-compatible-value", kind: "G3.TypeCompatibleValue"},
		}
		for _, wrong := range wrongTargets {
			t.Run(wrong.label+"-as-declaration-member", func(t *testing.T) {
				candidate := fixture.reference(t, snapshot, "case33-"+wrong.label, wrong.kind)
				verdict := fixture.validate(t, snapshot, "planned_filling_boundary", []sourceConformanceBinding{
					{slot: "WorkPlanSlot", filler: plan},
					{slot: "IntendedPerformanceDesignatorSlot", filler: future},
					{slot: "TargetDeclarationMemberSlot", filler: candidate},
					{slot: "PlannedValueSlot", filler: plannedValue},
				})
				assertSourceConformanceRejected(t, verdict, ValidationInvalid, DiagnosticEntityKindMismatch)
			})
		}

		memberKinds := []string{
			"G3.RelationSlotSpecMember",
			"G3.OperationArgumentDeclaration",
			"G3.OperationResultDeclaration",
		}
		for index, memberKind := range memberKinds {
			member := fixture.reference(t, snapshot, fmt.Sprintf("case33-member-%d", index), memberKind)
			planned := fixture.validate(t, snapshot, "planned_filling_boundary", []sourceConformanceBinding{
				{slot: "WorkPlanSlot", filler: plan},
				{slot: "IntendedPerformanceDesignatorSlot", filler: future},
				{slot: "TargetDeclarationMemberSlot", filler: member},
				{slot: "PlannedValueSlot", filler: plannedValue},
			})
			assertSourceConformanceTypeAdmission(t, planned)
		}

		application := fixture.reference(t, snapshot, "case33-actual-application", "G3.ActualOperationApplication")
		actualValue := fixture.reference(t, snapshot, "case33-actual-value", "G3.ActualBoundValue")
		plannedRow := fixture.reference(t, snapshot, "case33-planned-row", "G3.PlannedFillingRow")
		assertSourceConformanceDistinctKinds(t, fixture, []string{
			"G3.PlannedFillingRow",
			"G3.ActualOperationBinding",
		})

		rowAsValue := fixture.validate(t, snapshot, "actual_operation_binding_boundary", []sourceConformanceBinding{
			{slot: "ActualApplicationSlot", filler: application},
			{slot: "ActualBoundValueSlot", filler: plannedRow},
		})
		assertSourceConformanceRejected(t, rowAsValue, ValidationInvalid, DiagnosticEntityKindMismatch)

		actual := fixture.validate(t, snapshot, "actual_operation_binding_boundary", []sourceConformanceBinding{
			{slot: "ActualApplicationSlot", filler: application},
			{slot: "ActualBoundValueSlot", filler: actualValue},
		})
		assertSourceConformanceTypeAdmission(t, actual)
		assertSourceConformanceSingleExplicitRelation(t, fixture, actual, "actual_operation_binding_boundary")
	})
}

// TestSourceConformanceCategoryErrorSourceSnapshotIsCurrent is the explicit drift gate.
// The semantic corpus and this gate are pinned to one reviewed source
// snapshot. Any future publication-byte, range, or body change fails before
// P13 can close against a different source meaning.
func TestSourceConformanceCategoryErrorSourceSnapshotIsCurrent(t *testing.T) {
	sourceConformanceSourcesFromCurrentCheckout(t)
}

func newSourceConformanceFixture(t *testing.T) sourceConformanceFixture {
	t.Helper()
	sources, coverage := reviewedSourceConformanceSources(t)
	ref := typeEnvTestTypeEnvRef(t, 0xd1)
	contextDefinition := typeEnvTestBoundedContext(t, "ctx:g3-source-conformance", sources["A.1"].provenance)

	kindPatterns := []struct {
		kind    string
		pattern string
	}{
		{kind: "U.Entity", pattern: "A.1"},
		{kind: "U.Holon", pattern: "A.1"},
		{kind: "U.System", pattern: "A.1"},
		{kind: "U.Structure", pattern: "E.18.NET"},
		{kind: "U.Episteme", pattern: "C.2.1"},
		{kind: "U.Relation", pattern: "A.6.REL"},
		{kind: "U.Signature", pattern: "A.6.0"},
		{kind: "U.Kind", pattern: "E.24.UK"},
		{kind: "G3.AssignedSystemRoleKind", pattern: "A.2.1"},
		{kind: "U.SystemRoleAssignment", pattern: "A.2.1"},
		{kind: "U.ReferenceScheme", pattern: "C.2.1"},
		{kind: "U.Method", pattern: "A.3.1"},
		{kind: "U.MethodDescription", pattern: "A.3.2"},
		{kind: "U.Transformation", pattern: "A.3.4"},
		{kind: "U.Work", pattern: "A.15.1"},
		{kind: "U.WorkPlan", pattern: "A.15.2"},
		{kind: "U.PresentationCarrier", pattern: "E.17"},
		{kind: "U.ClaimGraph", pattern: "C.2.1"},
		// PublicationForm is a source-local relation-position category in this
		// oracle, not a claim that FPF admits another durable U-kind.
		{kind: "PublicationForm", pattern: "E.24.PUB"},
		// These are oracle-local IDs for the source terms
		// TaskSignature@Context and ProblemCard@Context; neither admits a new
		// root U-kind.
		{kind: "C.22.TaskSignature", pattern: "C.22"},
		{kind: "C.22.2.ProblemCard", pattern: "C.22.2"},
		// The G3.* labels are oracle-local categories used only to keep
		// representation, designation, retrieval, and edition cues distinct from
		// the world-side kinds required by the tested relation position. They do
		// not claim new FPF root U-kinds or production compiler lowering.
		{kind: "G3.GraphDirection", pattern: "C.28"},
		{kind: "G3.Timestamp", pattern: "C.28"},
		{kind: "G3.RetrievalRank", pattern: "C.28"},
		{kind: "G3.PresentationOrder", pattern: "C.28"},
		{kind: "G3.RelationalAssertion", pattern: "A.6.REL"},
		{kind: "G3.RepresentationRow", pattern: "C.2.1"},
		{kind: "G3.GraphEdge", pattern: "C.2.1"},
		{kind: "G3.Identifier", pattern: "C.2.1"},
		{kind: "G3.Reference", pattern: "C.2.1"},
		{kind: "G3.Reifier", pattern: "C.2.1"},
		{kind: "G3.SlotSpec", pattern: "A.6.5"},
		{kind: "G3.CarrierField", pattern: "E.24.PUB"},
		{kind: "G3.ParticipantDesignation", pattern: "C.2.1"},
		{kind: "G3.SourcePin", pattern: "C.2.1"},
		{kind: "G3.RevisionOrder", pattern: "C.2.1"},
		{kind: "G3.ContentSimilarity", pattern: "C.2.1"},
		// Cases 15-22 and 25 use oracle-local category IDs. They preserve
		// source distinctions without claiming new FPF root kinds, production
		// compiler lowering, public schemas, or authority-bearing records.
		{kind: "G3.ClaimBearingProgramDescriptionEpisteme", pattern: "A.3.2"},
		{kind: "G3.CodeCarrier", pattern: "E.24.PUB"},
		{kind: "G3.ClaimBearingModelEpisteme", pattern: "C.2.1"},
		{kind: "G3.RepositoryCarrier", pattern: "E.24.PUB"},
		{kind: "G3.TargetSystemLabel", pattern: "E.24.PUB"},
		{kind: "G3.SystemOfInterestLabel", pattern: "E.24.PUB"},
		{kind: "G3.TransformationFlowStructure", pattern: "E.18"},
		{kind: "G3.FlowValuation", pattern: "E.18"},
		{kind: "G3.FlowPositionRef", pattern: "E.18"},
		{kind: "G3.SubflowRef", pattern: "E.18"},
		{kind: "G3.TransformationFlowStructureNetwork", pattern: "E.18.NET"},
		{kind: "G3.ExposedFlowPositionRef", pattern: "E.18.NET"},
		{kind: "G3.SelectedNetworkConstraint", pattern: "E.18.NET"},
		{kind: "G3.NetworkUseFrame", pattern: "E.18.NET"},
		{kind: "G3.GraphProjection", pattern: "E.17"},
		{kind: "G3.TableProjection", pattern: "E.17"},
		{kind: "G3.Characteristic", pattern: "A.18"},
		{kind: "G3.Scale", pattern: "A.18"},
		{kind: "G3.MeasuredValue", pattern: "C.16"},
		{kind: "G3.Indicator", pattern: "A.19.UINDM"},
		{kind: "G3.Score", pattern: "A.19.USCM"},
		{kind: "G3.ComparatorSpec", pattern: "A.19.CPM"},
		{kind: "G3.Normalization", pattern: "A.19.UNM"},
		{kind: "G3.Fold", pattern: "A.19.ULSAM"},
		{kind: "G3.Comparison", pattern: "A.19.CPM"},
		{kind: "G3.Selection", pattern: "A.19.SelectorMechanism"},
		{kind: "G3.CandidatePalette", pattern: "C.18"},
		{kind: "G3.ArchiveFront", pattern: "C.18"},
		{kind: "G3.EvaluationResult", pattern: "C.16"},
		{kind: "G3.Recommendation", pattern: "E.11.PUR"},
		{kind: "G3.Choice", pattern: "C.11"},
		{kind: "G3.DecisionRecord", pattern: "C.11"},
		{kind: "G3.TargetEffect", pattern: "A.10"},
		{kind: "G3.SelectedStructure", pattern: "C.30"},
		{kind: "G3.CandidateStructure", pattern: "C.30"},
		{kind: "G3.ArchitectureRelation", pattern: "C.30"},
		{kind: "G3.ArchitectureClaim", pattern: "C.30"},
		{kind: "G3.ArchitectureDescription", pattern: "C.30"},
		{kind: "G3.ArchitectureCandidateSetOrArchive", pattern: "C.30"},
		{kind: "G3.ArchitectureCandidateMoveClaim", pattern: "C.30"},
		{kind: "G3.ArchitectureCandidateStopCondition", pattern: "C.30"},
		{kind: "G3.ADRCarrier", pattern: "E.24.PUB"},
		{kind: "G3.ExpectedStructureClaim", pattern: "C.30"},
		{kind: "G3.ObservedStructureClaim", pattern: "C.30"},
		{kind: "G3.Viewpoint", pattern: "E.17"},
		{kind: "G3.View", pattern: "E.17"},
		{kind: "G3.PublicationOccurrence", pattern: "E.24.PUB"},
		// Current source categories for G3 cases 23-24 and 27-33. Every G3.*
		// identifier is oracle-local and exists only to make a source boundary
		// executable; it is not production lowering or a new public U-kind.
		{kind: "U.Commitment", pattern: "A.2.8"},
		{kind: "G3.Applicability", pattern: "A.6.1"},
		{kind: "G3.ConstraintFit", pattern: "A.6.1"},
		{kind: "G3.TypedMemoryAdmission", pattern: "A.6.1"},
		{kind: "G3.PermissionGrant", pattern: "A.2.8.PER"},
		{kind: "G3.PermissionExercise", pattern: "A.2.8.PER"},
		{kind: "G3.GateDecision", pattern: "A.2.8.PER"},
		{kind: "G3.PermitCarrier", pattern: "A.2.8.PER"},
		{kind: "G3.Capability", pattern: "A.2.8.PER"},
		{kind: "G3.Readiness", pattern: "A.2.8.PER"},
		{kind: "G3.NonViolationFinding", pattern: "A.2.8.PER"},
		{kind: "G3.DirectGovernor", pattern: "A.6.P.WMR"},
		{kind: "G3.RelationModality", pattern: "A.6.P.WMR"},
		{kind: "G3.RelationPolarity", pattern: "A.6.P.WMR"},
		{kind: "G3.RecoveryPosture", pattern: "A.6.P.WMR"},
		{kind: "G3.MissingGovernor", pattern: "A.6.P.WMR"},
		{kind: "G3.MissingInformation", pattern: "A.6.P.WMR"},
		{kind: "G3.FactuallyUnsupported", pattern: "A.6.P.WMR"},
		{kind: "G3.PositivePolarity", pattern: "A.6.P.WMR"},
		{kind: "G3.GovernedNegativePolarity", pattern: "A.6.P.WMR"},
		{kind: "G3.AbsentPositiveSupport", pattern: "A.6.P.WMR"},
		{kind: "G3.InputWord", pattern: "A.6.P.WMR"},
		{kind: "G3.OutputWord", pattern: "A.6.P.WMR"},
		{kind: "G3.ResultWord", pattern: "A.6.P.WMR"},
		{kind: "G3.OutcomeWord", pattern: "A.6.P.WMR"},
		{kind: "G3.DeliverableWord", pattern: "A.6.P.WMR"},
		{kind: "G3.HandoffWord", pattern: "A.6.P.WMR"},
		{kind: "G3.TemporalExtent", pattern: "A.15.1"},
		{kind: "G3.WorkRecord", pattern: "A.15.1"},
		{kind: "G3.WorkLog", pattern: "A.15.1"},
		{kind: "G3.WorkFieldBundle", pattern: "A.15.1"},
		{kind: "G3.SuccessfulCommand", pattern: "A.15.1"},
		{kind: "G3.IdentifiedPresentEoC", pattern: "A.15.2"},
		{kind: "G3.FuturePerformanceDesignator", pattern: "A.15.2"},
		{kind: "G3.PlanItemContent", pattern: "A.15.2"},
		{kind: "G3.PlanItemOrderingPredicate", pattern: "A.15.2"},
		{kind: "G3.ProductionWorkClaim", pattern: "A.15.PROD"},
		{kind: "G3.EntityInceptionClaim", pattern: "A.15.PROD"},
		{kind: "G3.ProductionCompletionClaim", pattern: "A.15.PROD"},
		{kind: "G3.DeliveryClaim", pattern: "A.15.PROD"},
		{kind: "G3.AcceptanceClaim", pattern: "A.15.PROD"},
		{kind: "G3.ReleaseClaim", pattern: "A.15.PROD"},
		{kind: "G3.AvailabilityClaim", pattern: "A.15.PROD"},
		{kind: "U.Mechanism", pattern: "A.6.1"},
		{kind: "G3.OperationAlgebra", pattern: "A.6.1"},
		{kind: "G3.LawSet", pattern: "A.6.1"},
		{kind: "G3.AdmissibilityConditions", pattern: "A.6.1"},
		{kind: "G3.RuntimeMechanismArtifact", pattern: "A.6.1"},
		{kind: "G3.CodecRegistry", pattern: "A.6.1"},
		{kind: "G3.EvaluatorRegistry", pattern: "A.6.1"},
		{kind: "G3.ImplementationFunction", pattern: "A.6.1"},
		{kind: "G3.CompatibleArgument", pattern: "A.6.1"},
		{kind: "G3.SuccessfulInvocation", pattern: "A.6.1"},
		{kind: "G3.Profile", pattern: "A.1"},
		{kind: "G3.ComponentName", pattern: "A.1"},
		{kind: "G3.HostAgentReference", pattern: "A.1"},
		{kind: "G3.ConstituentSet", pattern: "A.1"},
		{kind: "G3.PartRelationSet", pattern: "A.1"},
		{kind: "G3.Assembly", pattern: "A.1"},
		{kind: "G3.WholeLevelCharacteristic", pattern: "A.1"},
		{kind: "G3.LargerAssemblyCompatibility", pattern: "A.1"},
		{kind: "G3.SystemActingCondition", pattern: "A.1"},
		{kind: "G3.PlannedDeclarationMember", pattern: "A.15.3"},
		{kind: "G3.RelationSlotSpecMember", pattern: "A.15.3"},
		{kind: "G3.OperationArgumentDeclaration", pattern: "A.15.3"},
		{kind: "G3.OperationResultDeclaration", pattern: "A.15.3"},
		{kind: "G3.MethodDescriptionField", pattern: "A.15.3"},
		{kind: "G3.CardField", pattern: "A.15.3"},
		{kind: "G3.SchemaField", pattern: "A.15.3"},
		{kind: "G3.TypeCompatibleValue", pattern: "A.15.3"},
		{kind: "G3.PlannedValue", pattern: "A.15.3"},
		{kind: "G3.PlannedFillingRow", pattern: "A.15.3"},
		{kind: "G3.ActualOperationApplication", pattern: "A.6.1"},
		{kind: "G3.ActualBoundValue", pattern: "A.6.1"},
		{kind: "G3.ActualOperationBinding", pattern: "A.6.1"},
	}
	kinds := make(map[string]KindDefinition, len(kindPatterns))
	valueKinds := make(map[string]ValueKindRef, len(kindPatterns))
	for _, entry := range kindPatterns {
		definition := typeEnvTestKindDefinition(t, entry.kind, sources[entry.pattern].provenance)
		kinds[entry.kind] = definition
		valueKinds[entry.kind] = typeEnvTestValueKindRef(t, ref, definition.ID())
	}

	entityRefID := typeEnvTestRefKindID(t, "U.EntityRef")
	entityRef := typeEnvTestRefKindRef(t, ref, entityRefID)
	refDefinition, err := NewRefKindDefinition(
		entityRef,
		valueKinds["U.Entity"],
		sources["A.1"].provenance,
	)
	if err != nil {
		t.Fatalf("NewRefKindDefinition(): %v", err)
	}

	subkindSpecs := []struct {
		subkind   string
		superkind string
		pattern   string
	}{
		{subkind: "U.Holon", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "U.System", superkind: "U.Holon", pattern: "A.1"},
		{subkind: "U.Structure", superkind: "U.Entity", pattern: "E.18.NET"},
		{subkind: "U.Episteme", superkind: "U.Holon", pattern: "C.2.1"},
		{subkind: "U.Relation", superkind: "U.Entity", pattern: "A.6.REL"},
		{subkind: "U.Signature", superkind: "U.Episteme", pattern: "A.6.0"},
		{subkind: "U.Kind", superkind: "U.Entity", pattern: "E.24.UK"},
		{subkind: "G3.AssignedSystemRoleKind", superkind: "U.Kind", pattern: "A.2.1"},
		{subkind: "U.SystemRoleAssignment", superkind: "U.Relation", pattern: "A.2.1"},
		{subkind: "U.ReferenceScheme", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "U.Method", superkind: "U.Holon", pattern: "A.3.1"},
		{subkind: "U.MethodDescription", superkind: "U.Episteme", pattern: "A.3.2"},
		{subkind: "U.Work", superkind: "U.Holon", pattern: "A.15.1"},
		{subkind: "U.WorkPlan", superkind: "U.Episteme", pattern: "A.15.2"},
		{subkind: "U.PresentationCarrier", superkind: "U.Entity", pattern: "E.17"},
		{subkind: "U.ClaimGraph", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "PublicationForm", superkind: "U.Entity", pattern: "E.24.PUB"},
		{subkind: "C.22.TaskSignature", superkind: "U.Signature", pattern: "C.22"},
		{subkind: "C.22.2.ProblemCard", superkind: "U.Episteme", pattern: "C.22.2"},
		{subkind: "G3.GraphDirection", superkind: "U.Entity", pattern: "C.28"},
		{subkind: "G3.Timestamp", superkind: "U.Entity", pattern: "C.28"},
		{subkind: "G3.RetrievalRank", superkind: "U.Entity", pattern: "C.28"},
		{subkind: "G3.PresentationOrder", superkind: "U.Entity", pattern: "C.28"},
		{subkind: "G3.RelationalAssertion", superkind: "U.Episteme", pattern: "A.6.REL"},
		{subkind: "G3.RepresentationRow", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.GraphEdge", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.Identifier", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.Reference", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.Reifier", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.SlotSpec", superkind: "U.Entity", pattern: "A.6.5"},
		{subkind: "G3.CarrierField", superkind: "U.Entity", pattern: "E.24.PUB"},
		{subkind: "G3.ParticipantDesignation", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.SourcePin", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.RevisionOrder", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.ContentSimilarity", superkind: "U.Entity", pattern: "C.2.1"},
		{subkind: "G3.ClaimBearingProgramDescriptionEpisteme", superkind: "U.Episteme", pattern: "A.3.2"},
		{subkind: "G3.CodeCarrier", superkind: "U.PresentationCarrier", pattern: "E.24.PUB"},
		{subkind: "G3.ClaimBearingModelEpisteme", superkind: "U.Episteme", pattern: "C.2.1"},
		{subkind: "G3.RepositoryCarrier", superkind: "U.PresentationCarrier", pattern: "E.24.PUB"},
		{subkind: "G3.TargetSystemLabel", superkind: "U.Entity", pattern: "E.24.PUB"},
		{subkind: "G3.SystemOfInterestLabel", superkind: "U.Entity", pattern: "E.24.PUB"},
		{subkind: "G3.TransformationFlowStructure", superkind: "U.Entity", pattern: "E.18"},
		{subkind: "G3.FlowValuation", superkind: "U.Entity", pattern: "E.18"},
		{subkind: "G3.FlowPositionRef", superkind: "U.Entity", pattern: "E.18"},
		{subkind: "G3.SubflowRef", superkind: "U.Entity", pattern: "E.18"},
		{subkind: "G3.TransformationFlowStructureNetwork", superkind: "U.Structure", pattern: "E.18.NET"},
		{subkind: "G3.ExposedFlowPositionRef", superkind: "U.Entity", pattern: "E.18.NET"},
		{subkind: "G3.SelectedNetworkConstraint", superkind: "U.Entity", pattern: "E.18.NET"},
		{subkind: "G3.NetworkUseFrame", superkind: "U.Entity", pattern: "E.18.NET"},
		{subkind: "G3.GraphProjection", superkind: "U.PresentationCarrier", pattern: "E.17"},
		{subkind: "G3.TableProjection", superkind: "U.PresentationCarrier", pattern: "E.17"},
		{subkind: "G3.Characteristic", superkind: "U.Entity", pattern: "A.18"},
		{subkind: "G3.Scale", superkind: "U.Entity", pattern: "A.18"},
		{subkind: "G3.MeasuredValue", superkind: "U.Entity", pattern: "C.16"},
		{subkind: "G3.Indicator", superkind: "U.Entity", pattern: "A.19.UINDM"},
		{subkind: "G3.Score", superkind: "U.Entity", pattern: "A.19.USCM"},
		{subkind: "G3.ComparatorSpec", superkind: "U.Episteme", pattern: "A.19.CPM"},
		{subkind: "G3.Normalization", superkind: "U.Entity", pattern: "A.19.UNM"},
		{subkind: "G3.Fold", superkind: "U.Entity", pattern: "A.19.ULSAM"},
		{subkind: "G3.Comparison", superkind: "U.Entity", pattern: "A.19.CPM"},
		{subkind: "G3.Selection", superkind: "U.Entity", pattern: "A.19.SelectorMechanism"},
		{subkind: "G3.CandidatePalette", superkind: "U.Episteme", pattern: "C.18"},
		{subkind: "G3.ArchiveFront", superkind: "U.Episteme", pattern: "C.18"},
		{subkind: "G3.EvaluationResult", superkind: "U.Episteme", pattern: "C.16"},
		{subkind: "G3.Recommendation", superkind: "U.Episteme", pattern: "E.11.PUR"},
		{subkind: "G3.Choice", superkind: "U.Episteme", pattern: "C.11"},
		{subkind: "G3.DecisionRecord", superkind: "U.Episteme", pattern: "C.11"},
		{subkind: "G3.TargetEffect", superkind: "U.Entity", pattern: "A.10"},
		{subkind: "G3.SelectedStructure", superkind: "U.Structure", pattern: "C.30"},
		{subkind: "G3.CandidateStructure", superkind: "U.Structure", pattern: "C.30"},
		{subkind: "G3.ArchitectureRelation", superkind: "U.Relation", pattern: "C.30"},
		{subkind: "G3.ArchitectureClaim", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ArchitectureDescription", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ArchitectureCandidateSetOrArchive", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ArchitectureCandidateMoveClaim", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ArchitectureCandidateStopCondition", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ADRCarrier", superkind: "U.PresentationCarrier", pattern: "E.24.PUB"},
		{subkind: "G3.ExpectedStructureClaim", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.ObservedStructureClaim", superkind: "U.Episteme", pattern: "C.30"},
		{subkind: "G3.Viewpoint", superkind: "U.Episteme", pattern: "E.17"},
		{subkind: "G3.View", superkind: "U.Episteme", pattern: "E.17"},
		{subkind: "G3.PublicationOccurrence", superkind: "U.Relation", pattern: "E.24.PUB"},
		{subkind: "U.Commitment", superkind: "U.Relation", pattern: "A.2.8"},
		{subkind: "G3.Applicability", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.ConstraintFit", superkind: "U.Episteme", pattern: "A.6.1"},
		{subkind: "G3.TypedMemoryAdmission", superkind: "U.Episteme", pattern: "A.6.1"},
		{subkind: "G3.PermissionGrant", superkind: "U.Relation", pattern: "A.2.8.PER"},
		{subkind: "G3.PermissionExercise", superkind: "U.Relation", pattern: "A.2.8.PER"},
		{subkind: "G3.GateDecision", superkind: "U.Episteme", pattern: "A.2.8.PER"},
		{subkind: "G3.PermitCarrier", superkind: "U.PresentationCarrier", pattern: "A.2.8.PER"},
		{subkind: "G3.Capability", superkind: "U.Entity", pattern: "A.2.8.PER"},
		{subkind: "G3.Readiness", superkind: "U.Episteme", pattern: "A.2.8.PER"},
		{subkind: "G3.NonViolationFinding", superkind: "U.Episteme", pattern: "A.2.8.PER"},
		{subkind: "G3.DirectGovernor", superkind: "U.Episteme", pattern: "A.6.P.WMR"},
		{subkind: "G3.RelationModality", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.RelationPolarity", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.RecoveryPosture", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.MissingGovernor", superkind: "G3.RecoveryPosture", pattern: "A.6.P.WMR"},
		{subkind: "G3.MissingInformation", superkind: "G3.RecoveryPosture", pattern: "A.6.P.WMR"},
		{subkind: "G3.FactuallyUnsupported", superkind: "G3.RecoveryPosture", pattern: "A.6.P.WMR"},
		{subkind: "G3.PositivePolarity", superkind: "G3.RelationPolarity", pattern: "A.6.P.WMR"},
		{subkind: "G3.GovernedNegativePolarity", superkind: "G3.RelationPolarity", pattern: "A.6.P.WMR"},
		{subkind: "G3.AbsentPositiveSupport", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.InputWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.OutputWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.ResultWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.OutcomeWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.DeliverableWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.HandoffWord", superkind: "U.Entity", pattern: "A.6.P.WMR"},
		{subkind: "G3.TemporalExtent", superkind: "U.Entity", pattern: "A.15.1"},
		{subkind: "G3.WorkRecord", superkind: "U.Episteme", pattern: "A.15.1"},
		{subkind: "G3.WorkLog", superkind: "U.Episteme", pattern: "A.15.1"},
		{subkind: "G3.WorkFieldBundle", superkind: "U.Episteme", pattern: "A.15.1"},
		{subkind: "G3.SuccessfulCommand", superkind: "U.Episteme", pattern: "A.15.1"},
		{subkind: "G3.IdentifiedPresentEoC", superkind: "U.Entity", pattern: "A.15.2"},
		{subkind: "G3.FuturePerformanceDesignator", superkind: "U.Entity", pattern: "A.15.2"},
		{subkind: "G3.PlanItemContent", superkind: "U.Entity", pattern: "A.15.2"},
		{subkind: "G3.PlanItemOrderingPredicate", superkind: "U.Entity", pattern: "A.15.2"},
		{subkind: "G3.ProductionWorkClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.EntityInceptionClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.ProductionCompletionClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.DeliveryClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.AcceptanceClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.ReleaseClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "G3.AvailabilityClaim", superkind: "U.Episteme", pattern: "A.15.PROD"},
		{subkind: "U.Mechanism", superkind: "U.Episteme", pattern: "A.6.1"},
		{subkind: "G3.OperationAlgebra", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.LawSet", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.AdmissibilityConditions", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.RuntimeMechanismArtifact", superkind: "U.Episteme", pattern: "A.6.1"},
		{subkind: "G3.CodecRegistry", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.EvaluatorRegistry", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.ImplementationFunction", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.CompatibleArgument", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.SuccessfulInvocation", superkind: "U.Episteme", pattern: "A.6.1"},
		{subkind: "G3.Profile", superkind: "U.Episteme", pattern: "A.1"},
		{subkind: "G3.ComponentName", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.HostAgentReference", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.ConstituentSet", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.PartRelationSet", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.Assembly", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.WholeLevelCharacteristic", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.LargerAssemblyCompatibility", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.SystemActingCondition", superkind: "U.Entity", pattern: "A.1"},
		{subkind: "G3.PlannedDeclarationMember", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.RelationSlotSpecMember", superkind: "G3.PlannedDeclarationMember", pattern: "A.15.3"},
		{subkind: "G3.OperationArgumentDeclaration", superkind: "G3.PlannedDeclarationMember", pattern: "A.15.3"},
		{subkind: "G3.OperationResultDeclaration", superkind: "G3.PlannedDeclarationMember", pattern: "A.15.3"},
		{subkind: "G3.MethodDescriptionField", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.CardField", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.SchemaField", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.TypeCompatibleValue", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.PlannedValue", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.PlannedFillingRow", superkind: "U.Entity", pattern: "A.15.3"},
		{subkind: "G3.ActualOperationApplication", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.ActualBoundValue", superkind: "U.Entity", pattern: "A.6.1"},
		{subkind: "G3.ActualOperationBinding", superkind: "U.Relation", pattern: "A.6.1"},
	}

	signatures := make(map[string]RelationSignature, len(sourceConformanceRelationSpecs))
	for _, spec := range sourceConformanceRelationSpecs {
		signatures[spec.key] = newSourceConformanceRelationSignature(
			t,
			ref,
			contextDefinition.Ref(),
			entityRef,
			valueKinds,
			spec,
			sources[spec.pattern].provenance,
		)
	}

	builder := NewTypeEnvBuilder(ref).
		SetSourceRevision(typeEnvTestSourceRevision(t, sourceConformanceFPFRevision)).
		SetCompilerSchemaVersion(typeEnvTestCompilerVersion(t, "acceptance-g3-source-conformance-v2")).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextDefinition).
		AddRefKindDefinition(refDefinition)
	for _, entry := range kindPatterns {
		builder = builder.
			AddKindDefinition(kinds[entry.kind]).
			AddContextKindAvailability(typeEnvTestKindAvailability(
				contextDefinition.Ref(),
				kinds[entry.kind].ID(),
				sources[entry.pattern].provenance,
			))
	}
	for _, spec := range subkindSpecs {
		relation, relationErr := NewSubkindRelation(
			kinds[spec.subkind].ID(),
			kinds[spec.superkind].ID(),
			sources[spec.pattern].provenance,
		)
		if relationErr != nil {
			t.Fatalf("NewSubkindRelation(%s): %v", spec.subkind, relationErr)
		}
		builder = builder.AddSubkindRelation(relation)
	}
	for _, spec := range sourceConformanceRelationSpecs {
		builder = builder.AddRelationSignature(signatures[spec.key])
	}
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("build sourceConformance source-conformance TypeEnv: %v", err)
	}

	return sourceConformanceFixture{
		environment: environment,
		context:     contextDefinition.Ref(),
		refKind:     entityRef,
		kinds:       kinds,
		valueKinds:  valueKinds,
		signatures:  signatures,
		sources:     sources,
		testing:     t,
	}
}

func sourceConformanceSourcesFromCurrentCheckout(
	t *testing.T,
) (map[string]sourceConformanceSource, CoverageManifest) {
	t.Helper()
	content, err := os.ReadFile("../../data/FPF/FPF-Spec.md")
	if err != nil {
		t.Fatalf("read bundled FPF source: %v", err)
	}
	wholeDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	if wholeDigest != sourceConformanceFPFSpecDigest {
		t.Fatalf("FPF source digest = %s; want reviewed snapshot %s", wholeDigest, sourceConformanceFPFSpecDigest)
	}
	lines := strings.Split(string(content), "\n")
	for _, sourceRange := range sourceConformanceSourceRanges {
		if sourceRange.end > uint64(len(lines)) {
			t.Fatalf("%s range ends past source: %d > %d", sourceRange.pattern, sourceRange.end, len(lines))
		}
		body := strings.TrimSpace(strings.Join(lines[sourceRange.start-1:sourceRange.end], "\n"))
		wantHeader := "## " + sourceRange.pattern + " -"
		if !strings.HasPrefix(body, wantHeader) {
			t.Fatalf("%s source range does not start with %q", sourceRange.pattern, wantHeader)
		}
		bodyDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body)))
		if bodyDigest != sourceRange.digest {
			t.Fatalf(
				"%s source body digest = %s; want reviewed %s",
				sourceRange.pattern,
				bodyDigest,
				sourceRange.digest,
			)
		}
	}
	return reviewedSourceConformanceSources(t)
}

func reviewedSourceConformanceSources(
	t *testing.T,
) (map[string]sourceConformanceSource, CoverageManifest) {
	t.Helper()
	sources := make(map[string]sourceConformanceSource, len(sourceConformanceSourceRanges))
	entries := make([]CoverageEntry, 0, len(sourceConformanceSourceRanges))
	for _, sourceRange := range sourceConformanceSourceRanges {
		location := mustSourceConformanceSourceLocation(t, sourceRange, sourceRange.digest)
		provenance := mustSourceConformanceProvenance(t, sourceRange.pattern, location)
		sources[sourceRange.pattern] = sourceConformanceSource{
			location:   location,
			provenance: provenance,
		}
		subject, err := SourceUnitCoverage(location.UnitID())
		if err != nil {
			t.Fatalf("SourceUnitCoverage(%s): %v", sourceRange.pattern, err)
		}
		entry, err := NewCompiledCoverageEntry(subject, location)
		if err != nil {
			t.Fatalf("NewCompiledCoverageEntry(%s): %v", sourceRange.pattern, err)
		}
		entries = append(entries, entry)
	}
	coverage, err := NewCoverageManifest(entries)
	if err != nil {
		t.Fatalf("NewCoverageManifest(): %v", err)
	}
	return sources, coverage
}

func mustSourceConformanceSourceLocation(
	t *testing.T,
	sourceRange sourceConformanceSourceRange,
	bodyDigest string,
) SourceLocation {
	t.Helper()
	unitID, err := NewSourceUnitID("spec:pattern_body:" + strings.ReplaceAll(strings.ToLower(sourceRange.pattern), ".", "-"))
	if err != nil {
		t.Fatalf("NewSourceUnitID(%s): %v", sourceRange.pattern, err)
	}
	revision, err := NewSourceRevision(sourceConformanceFPFRevision)
	if err != nil {
		t.Fatalf("NewSourceRevision(): %v", err)
	}
	digest, err := NewSHA256Digest(bodyDigest)
	if err != nil {
		t.Fatalf("NewSHA256Digest(%s): %v", sourceRange.pattern, err)
	}
	lineRange, err := NewSourceLineRange(sourceRange.start, sourceRange.end)
	if err != nil {
		t.Fatalf("NewSourceLineRange(%s): %v", sourceRange.pattern, err)
	}
	patternID, err := NewPatternID(sourceRange.pattern)
	if err != nil {
		t.Fatalf("NewPatternID(%s): %v", sourceRange.pattern, err)
	}
	location, err := NewPatternedSourceLocation(unitID, revision, digest, lineRange, patternID)
	if err != nil {
		t.Fatalf("NewPatternedSourceLocation(%s): %v", sourceRange.pattern, err)
	}
	return location
}

func mustSourceConformanceProvenance(
	t *testing.T,
	pattern string,
	location SourceLocation,
) CompilerDerivedProvenance {
	t.Helper()
	ref, err := NewProvenanceRef("prov:g3-source-conformance:" + pattern)
	if err != nil {
		t.Fatalf("NewProvenanceRef(%s): %v", pattern, err)
	}
	ruleRaw := "acceptance.g3.source_conformance." + strings.ReplaceAll(strings.ToLower(pattern), ".", "_") + ".v2"
	rule, err := NewCompilerRuleID(ruleRaw)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(%s): %v", pattern, err)
	}
	provenance, err := NewCompilerDerivedProvenance(ref, []SourceLocation{location}, rule)
	if err != nil {
		t.Fatalf("NewCompilerDerivedProvenance(%s): %v", pattern, err)
	}
	return provenance
}

func newSourceConformanceRelationSignature(
	t *testing.T,
	ref TypeEnvRef,
	context BoundedContextRef,
	refKind RefKindRef,
	valueKinds map[string]ValueKindRef,
	spec sourceConformanceRelationSpec,
	provenance DeclarationProvenance,
) RelationSignature {
	t.Helper()
	slots := make([]SlotSpec, 0, len(spec.slots))
	for _, slotSpec := range spec.slots {
		target, err := NewReferenceSlotTarget(valueKinds[slotSpec.kind], refKind)
		if err != nil {
			t.Fatalf("NewReferenceSlotTarget(%s.%s): %v", spec.key, slotSpec.name, err)
		}
		slot, err := NewSlotSpec(
			typeEnvTestSlotKindID(t, slotSpec.name),
			target,
			ExactlyOneCardinality(),
			provenance,
		)
		if err != nil {
			t.Fatalf("NewSlotSpec(%s.%s): %v", spec.key, slotSpec.name, err)
		}
		slots = append(slots, slot)
	}
	signature, err := NewRelationSignature(
		typeEnvTestSignatureRef(t, ref, "G3."+spec.key),
		[]BoundedContextRef{context},
		slots,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature(%s): %v", spec.key, err)
	}
	return signature
}

func (fixture sourceConformanceFixture) snapshot() *sourceConformanceSnapshot {
	notMembers := map[string]DeclarationProvenance{}
	addNotMember := func(actual string, expected string, pattern string) {
		notMembers[sourceConformanceMembershipKey(fixture.kinds[actual].ID(), fixture.kinds[expected].ID())] = fixture.sources[pattern].provenance
	}
	addNotMembers := func(actuals []string, expected string, pattern string) {
		for _, actual := range actuals {
			addNotMember(actual, expected, pattern)
		}
	}
	addNotMember("U.WorkPlan", "U.System", "A.14")
	addNotMember("G3.AssignedSystemRoleKind", "U.System", "A.2.1")
	addNotMember("U.WorkPlan", "U.Work", "A.15.1")
	addNotMember("U.PresentationCarrier", "U.Episteme", "E.17")
	addNotMember("U.Episteme", "U.PresentationCarrier", "E.24.PUB")
	addNotMember("PublicationForm", "U.Episteme", "E.24.PUB")
	addNotMember("U.PresentationCarrier", "U.ClaimGraph", "C.2.1")
	addNotMember("C.22.TaskSignature", "U.Work", "C.22")
	addNotMember("C.22.2.ProblemCard", "U.Work", "C.22.2")
	addNotMember("U.Work", "U.Episteme", "E.24.UK")
	addNotMember("G3.GraphDirection", "U.MethodDescription", "C.28")
	addNotMember("G3.Timestamp", "U.MethodDescription", "C.28")
	addNotMember("G3.RetrievalRank", "U.MethodDescription", "C.28")
	addNotMember("G3.PresentationOrder", "U.MethodDescription", "C.28")
	addNotMembers([]string{
		"G3.GraphDirection",
		"G3.Timestamp",
		"G3.RetrievalRank",
		"G3.PresentationOrder",
	}, "G3.PlanItemOrderingPredicate", "A.15.2")
	addNotMember("G3.RelationalAssertion", "U.Relation", "A.6.REL")
	addNotMember("G3.RepresentationRow", "U.Relation", "C.2.1")
	addNotMember("G3.GraphEdge", "U.Relation", "C.2.1")
	addNotMember("G3.Identifier", "U.Relation", "C.2.1")
	addNotMember("G3.Reference", "U.Relation", "C.2.1")
	addNotMember("G3.Reifier", "U.Relation", "C.2.1")
	addNotMember("G3.SlotSpec", "U.System", "A.6.5")
	addNotMember("G3.CarrierField", "U.System", "E.24.PUB")
	addNotMember("G3.ParticipantDesignation", "U.System", "C.2.1")
	addNotMember("U.Work", "U.ClaimGraph", "C.2.1")
	addNotMember("U.Episteme", "U.ClaimGraph", "C.2.1")
	addNotMember("U.Relation", "U.ClaimGraph", "C.2.1")
	addNotMember("G3.Timestamp", "U.ClaimGraph", "C.2.1")
	addNotMember("G3.SourcePin", "U.Episteme", "C.2.1")
	addNotMember("G3.RevisionOrder", "U.Episteme", "C.2.1")
	addNotMember("G3.ContentSimilarity", "U.Episteme", "C.2.1")
	addNotMembers([]string{
		"G3.ClaimBearingProgramDescriptionEpisteme",
		"G3.CodeCarrier",
		"G3.ClaimBearingModelEpisteme",
		"G3.RepositoryCarrier",
		"G3.TargetSystemLabel",
		"G3.SystemOfInterestLabel",
		"U.SystemRoleAssignment",
		"U.Work",
	}, "U.System", "A.1")
	addNotMembers([]string{
		"G3.FlowValuation",
		"G3.FlowPositionRef",
		"G3.SubflowRef",
		"G3.TransformationFlowStructureNetwork",
		"G3.ExposedFlowPositionRef",
		"G3.GraphProjection",
		"G3.TableProjection",
		"U.Work",
		"U.WorkPlan",
	}, "G3.TransformationFlowStructure", "E.18")
	addNotMembers([]string{
		"G3.TransformationFlowStructure",
		"G3.FlowPositionRef",
		"G3.SubflowRef",
		"G3.TransformationFlowStructureNetwork",
		"G3.ExposedFlowPositionRef",
		"G3.GraphProjection",
		"G3.TableProjection",
		"U.Work",
		"U.WorkPlan",
	}, "G3.FlowValuation", "E.18")
	addNotMembers([]string{
		"G3.Characteristic",
		"G3.Scale",
		"G3.MeasuredValue",
		"G3.Indicator",
		"G3.Score",
		"G3.ComparatorSpec",
		"G3.Normalization",
		"G3.Fold",
		"G3.Selection",
		"G3.ArchiveFront",
	}, "G3.Comparison", "A.19.CPM")
	addNotMembers([]string{
		"G3.CandidatePalette",
		"G3.ArchiveFront",
		"G3.EvaluationResult",
		"G3.Recommendation",
		"G3.Choice",
		"G3.DecisionRecord",
		"G3.TargetEffect",
	}, "U.Work", "A.15.1")
	addNotMembers([]string{
		"G3.CandidateStructure",
		"G3.ArchitectureRelation",
		"G3.ArchitectureClaim",
		"G3.ArchitectureDescription",
		"G3.ArchitectureCandidateSetOrArchive",
		"G3.ArchitectureCandidateMoveClaim",
		"G3.ArchitectureCandidateStopCondition",
		"G3.ADRCarrier",
		"G3.ExpectedStructureClaim",
		"G3.ObservedStructureClaim",
	}, "G3.SelectedStructure", "C.30")
	addNotMember("G3.SelectedStructure", "G3.CandidateStructure", "C.30")
	addNotMember("G3.ArchitectureRelation", "G3.ArchitectureClaim", "C.30")
	addNotMembers([]string{
		"U.Method",
		"U.MethodDescription",
		"U.WorkPlan",
		"U.Transformation",
		"G3.Viewpoint",
		"G3.View",
		"G3.ClaimBearingModelEpisteme",
		"G3.PublicationOccurrence",
		"U.PresentationCarrier",
		"U.Episteme",
	}, "U.Work", "A.15.1")
	addNotMembers([]string{
		"G3.Applicability",
		"G3.ConstraintFit",
		"G3.TypedMemoryAdmission",
		"U.Commitment",
		"G3.PermissionExercise",
		"G3.GateDecision",
		"G3.PermitCarrier",
		"G3.Capability",
		"G3.Readiness",
		"G3.NonViolationFinding",
		"U.Work",
	}, "G3.PermissionGrant", "A.2.8.PER")
	addNotMembers([]string{
		"G3.InputWord",
		"G3.OutputWord",
		"G3.ResultWord",
		"G3.OutcomeWord",
		"G3.DeliverableWord",
		"G3.HandoffWord",
	}, "G3.DirectGovernor", "A.6.P.WMR")
	addNotMember("G3.AbsentPositiveSupport", "G3.RelationPolarity", "A.6.P.WMR")
	addNotMembers([]string{
		"G3.WorkRecord",
		"G3.WorkLog",
		"G3.WorkFieldBundle",
		"G3.SuccessfulCommand",
	}, "U.Work", "A.15.1")
	addNotMembers([]string{
		"G3.FuturePerformanceDesignator",
		"G3.PlanItemContent",
	}, "G3.IdentifiedPresentEoC", "A.15.2")
	addNotMembers([]string{
		"G3.EntityInceptionClaim",
		"G3.ProductionCompletionClaim",
	}, "G3.ProductionWorkClaim", "A.15.PROD")
	addNotMembers([]string{
		"G3.ProductionWorkClaim",
		"G3.ProductionCompletionClaim",
	}, "G3.EntityInceptionClaim", "A.15.PROD")
	addNotMembers([]string{
		"G3.ProductionWorkClaim",
		"G3.EntityInceptionClaim",
		"G3.DeliveryClaim",
		"G3.AcceptanceClaim",
		"G3.ReleaseClaim",
		"G3.PublicationOccurrence",
		"G3.AvailabilityClaim",
		"G3.TargetEffect",
		"U.Work",
		"U.Transformation",
	}, "G3.ProductionCompletionClaim", "A.15.PROD")
	addNotMembers([]string{
		"G3.RuntimeMechanismArtifact",
		"G3.CodecRegistry",
		"G3.EvaluatorRegistry",
		"G3.ImplementationFunction",
		"G3.CompatibleArgument",
		"G3.SuccessfulInvocation",
	}, "U.Mechanism", "A.6.1")
	addNotMembers([]string{
		"G3.Profile",
		"G3.Capability",
		"G3.ComponentName",
		"G3.HostAgentReference",
	}, "U.System", "A.1")
	addNotMember("G3.RepositoryCarrier", "G3.ConstituentSet", "A.1")
	addNotMembers([]string{
		"G3.MethodDescriptionField",
		"G3.CardField",
		"G3.SchemaField",
		"G3.TypeCompatibleValue",
	}, "G3.PlannedDeclarationMember", "A.15.3")
	addNotMembers([]string{
		"G3.PlannedFillingRow",
		"G3.TypeCompatibleValue",
	}, "G3.ActualOperationBinding", "A.6.1")
	addNotMember("G3.PlannedFillingRow", "G3.ActualBoundValue", "A.6.1")
	return &sourceConformanceSnapshot{
		revision:    NewGraphRevision(41),
		environment: fixture.environment,
		context:     fixture.context,
		references:  map[string]EntityID{},
		actualKinds: map[string]KindID{},
		notMembers:  notMembers,
		assertions:  map[string]AssertionState{},
		testing:     fixture.testing,
	}
}

func (fixture sourceConformanceFixture) reference(
	t *testing.T,
	snapshot *sourceConformanceSnapshot,
	label string,
	actualKind string,
) ByReferenceCandidate {
	t.Helper()
	return fixture.referenceTo(t, snapshot, label, label, actualKind)
}

func (fixture sourceConformanceFixture) referenceTo(
	t *testing.T,
	snapshot *sourceConformanceSnapshot,
	referenceLabel string,
	entityLabel string,
	actualKind string,
) ByReferenceCandidate {
	t.Helper()
	entity, err := NewEntityID("entity:g3:" + entityLabel)
	if err != nil {
		t.Fatalf("NewEntityID(%s): %v", entityLabel, err)
	}
	referenceID, err := NewReferenceID("entity:g3:" + referenceLabel)
	if err != nil {
		t.Fatalf("NewReferenceID(%s): %v", referenceLabel, err)
	}
	reference, err := NewPersistedRef(fixture.refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef(%s): %v", referenceLabel, err)
	}
	snapshot.references[reference.ReferenceKey()] = entity
	if actualKind != "" {
		snapshot.actualKinds[entity.String()] = fixture.kinds[actualKind].ID()
	}
	filler, err := NewByReferenceCandidate(reference)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(%s): %v", referenceLabel, err)
	}
	return filler
}

func (fixture sourceConformanceFixture) validate(
	t *testing.T,
	snapshot *sourceConformanceSnapshot,
	signatureKey string,
	bindings []sourceConformanceBinding,
) ValidationVerdict {
	t.Helper()
	snapshot.assertionCount++
	assertion, err := NewAssertionID(fmt.Sprintf("assertion:g3:%s:%d", signatureKey, snapshot.assertionCount))
	if err != nil {
		t.Fatalf("NewAssertionID(%s): %v", signatureKey, err)
	}
	absent, err := NewAbsentAssertionState(assertion, validationTestRule(t))
	if err != nil {
		t.Fatalf("NewAbsentAssertionState(%s): %v", signatureKey, err)
	}
	snapshot.assertions[assertion.String()] = absent
	relation := fixture.relationWithAssertion(t, assertion, signatureKey, bindings)
	changeSet := validationTestChangeSet(t, relation)
	return ValidateMemoryChangeSet(
		fixture.environment,
		NewCodecRegistry(),
		snapshot,
		changeSet,
	)
}

func (fixture sourceConformanceFixture) relationWithAssertion(
	t *testing.T,
	assertion AssertionID,
	signatureKey string,
	bindings []sourceConformanceBinding,
) RelationInstantiation {
	t.Helper()
	candidateBindings := make([]CandidateSlotBinding, 0, len(bindings))
	for _, input := range bindings {
		binding, bindingErr := NewCandidateSlotBinding(
			typeEnvTestSlotKindID(t, input.slot),
			[]CandidateSlotFiller{input.filler},
		)
		if bindingErr != nil {
			t.Fatalf("NewCandidateSlotBinding(%s.%s): %v", signatureKey, input.slot, bindingErr)
		}
		candidateBindings = append(candidateBindings, binding)
	}
	relation, err := NewRelationInstantiation(
		assertion,
		fixture.signatures[signatureKey].Ref(),
		validationTestContextSlice(t, fixture.context),
		candidateBindings,
		typeEnvTestProvenanceRef(t, "memory:g3-source-conformance:"+signatureKey),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation(%s): %v", signatureKey, err)
	}
	return relation
}

func (snapshot sourceConformanceSnapshot) GraphRevision() GraphRevision { return snapshot.revision }

func (snapshot sourceConformanceSnapshot) TypeEnvRef() TypeEnvRef { return snapshot.environment.Ref() }

func (snapshot sourceConformanceSnapshot) ResolveEntity(
	EntityID,
	BoundedContextRef,
) EntityResolution {
	return nil
}

func (snapshot sourceConformanceSnapshot) ResolveReference(
	reference StrongRef,
	context BoundedContextRef,
) StrongReferenceResolution {
	entity, exists := snapshot.references[reference.ReferenceKey()]
	if !exists {
		return nil
	}
	resolution, err := NewResolvedStrongReference(
		reference,
		entity,
		context,
		mustResolutionBasisRefForSourceConformance(),
	)
	if err != nil {
		return nil
	}
	return resolution
}

func (snapshot sourceConformanceSnapshot) EvaluateMemberOf(
	request MemberOfEvaluationRequest,
) MemberOfJudgement {
	query := request.Query()
	actual, known := snapshot.actualKinds[query.EntityID().String()]
	if known && snapshot.environment.IsSubkind(actual, query.ValueKind().ID()) {
		definition, _ := snapshot.environment.KindDefinition(actual)
		return validationTestMemberOfMemberWithView(
			snapshot.testing,
			query,
			definition.Provenance(),
			request.View(),
		)
	}
	provenance, knownNotMember := snapshot.notMembers[sourceConformanceMembershipKey(actual, query.ValueKind().ID())]
	if known && knownNotMember {
		return validationTestMemberOfNotMemberWithView(
			snapshot.testing,
			query,
			provenance,
			request.View(),
		)
	}
	missing, _ := MissingKindSignatureForMemberOf(query)
	repair, _ := NewRepairPointer("recover:g3:membership:" + query.EntityID().String() + ":" + query.ValueKind().ID().String())
	undefined, _ := NewMemberOfUndefined(request, []MemberOfMissingBasis{missing}, repair)
	return undefined
}

func (snapshot sourceConformanceSnapshot) AssertionState(assertion AssertionID) AssertionState {
	return snapshot.assertions[assertion.String()]
}

func (sourceConformanceSnapshot) ResolveAlias(EntityAlias, BoundedContextRef) AliasAvailability {
	return nil
}

func (sourceConformanceSnapshot) ResolveReconciliationBasis(
	ReconciliationBasisRef,
	BoundedContextRef,
) ReconciliationBasisResolution {
	return nil
}

func mustResolutionBasisRefForSourceConformance() ResolutionBasisRef {
	value, _ := NewResolutionBasisRef("snapshot:g3-source-conformance-reference-index")
	return value
}

func sourceConformanceMembershipKey(actual KindID, expected KindID) string {
	return actual.String() + "\x00" + expected.String()
}

func assertSourceConformanceDistinctKinds(
	t *testing.T,
	fixture sourceConformanceFixture,
	labels []string,
) {
	t.Helper()
	seen := map[string]struct{}{}
	for _, label := range labels {
		ref := fixture.valueKinds[label].String()
		if _, exists := seen[ref]; exists {
			t.Fatalf("%s collapsed into an existing ValueKindRef %s", label, ref)
		}
		seen[ref] = struct{}{}
	}
}

func sourceConformanceNetworkSelectionIdentity(
	t *testing.T,
	selection sourceConformanceNetworkSelection,
) string {
	t.Helper()
	members := sourceConformanceCanonicalReferenceSet(t, "direct members", selection.directMembers, 2)
	relations := sourceConformanceCanonicalReferenceSet(
		t,
		"selected cross-flow relations",
		selection.selectedCrossFlowRelations,
		1,
	)
	constraints := sourceConformanceCanonicalReferenceSet(
		t,
		"selected network constraints",
		selection.selectedNetworkConstraints,
		1,
	)
	useFrame := selection.useFrame.Reference().ReferenceKey()
	if useFrame == "" {
		t.Fatal("network use frame has an empty exact reference")
	}
	// E.18.NET explicitly keeps returnCondition outside StructureIdentity.
	_ = selection.returnCondition
	return strings.Join([]string{members, relations, constraints, useFrame}, "\x00")
}

func sourceConformanceCanonicalReferenceSet(
	t *testing.T,
	label string,
	values []ByReferenceCandidate,
	minimum int,
) string {
	t.Helper()
	if len(values) < minimum {
		t.Fatalf("%s = %d; want at least %d", label, len(values), minimum)
	}
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := value.Reference().ReferenceKey()
		if key == "" {
			t.Fatalf("%s contains an empty exact reference", label)
		}
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("%s contains duplicate exact reference %s", label, key)
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\x1f")
}

func sourceConformanceCoordinate(
	claimGraph ByReferenceCandidate,
	entityOfConcern ByReferenceCandidate,
	referenceScheme ByReferenceCandidate,
) string {
	parts := []string{
		claimGraph.Reference().ReferenceKey(),
		entityOfConcern.Reference().ReferenceKey(),
		referenceScheme.Reference().ReferenceKey(),
	}
	return strings.Join(parts, "\x00")
}

func sourceConformancePartialComparisonCoordinate(
	comparison ByReferenceCandidate,
	basis ByReferenceCandidate,
) string {
	parts := []string{
		comparison.Reference().ReferenceKey(),
		basis.Reference().ReferenceKey(),
	}
	return strings.Join(parts, "\x00")
}

func assertSourceConformanceDistinctCoordinates(t *testing.T, coordinates []string) {
	t.Helper()
	seen := map[string]struct{}{}
	for _, coordinate := range coordinates {
		if _, exists := seen[coordinate]; exists {
			t.Fatalf("duplicate episteme constitution coordinate %q", coordinate)
		}
		seen[coordinate] = struct{}{}
	}
}

func assertSourceConformanceDeclarationsAreSourceBacked(
	t *testing.T,
	fixture sourceConformanceFixture,
) {
	t.Helper()
	for _, spec := range sourceConformanceRelationSpecs {
		provenance, ok := fixture.signatures[spec.key].Provenance().(CompilerDerivedProvenance)
		if !ok {
			t.Fatalf("%s provenance = %T; want compiler-derived source oracle", spec.key, fixture.signatures[spec.key].Provenance())
		}
		inputs := provenance.Inputs()
		if len(inputs) != 1 {
			t.Fatalf("%s source inputs = %d; want one exact pattern body", spec.key, len(inputs))
		}
		pattern, present := inputs[0].PatternID()
		if !present || pattern.String() != spec.pattern {
			t.Fatalf("%s source PatternID = %q; want %q", spec.key, pattern.String(), spec.pattern)
		}
		if inputs[0].Revision().String() != sourceConformanceFPFRevision {
			t.Fatalf("%s source revision = %q", spec.key, inputs[0].Revision().String())
		}
		if inputs[0].ContentHash() != fixture.sources[spec.pattern].location.ContentHash() {
			t.Fatalf("%s source content hash is not the reviewed pattern-body hash", spec.key)
		}
	}
}

func assertSourceConformanceSingleExplicitRelation(
	t *testing.T,
	fixture sourceConformanceFixture,
	verdict ValidationVerdict,
	signatureKey string,
) RelationInstance {
	t.Helper()
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T; want Valid", verdict)
	}
	changes := valid.AdmissionBatch().ChangeSet().Changes()
	if len(changes) != 1 {
		t.Fatalf("admission changes = %d; want one explicit relation and zero inferred changes", len(changes))
	}
	validatedRelation, ok := changes[0].(ValidatedRelationInstance)
	if !ok {
		t.Fatalf("admission change = %T; want ValidatedRelationInstance", changes[0])
	}
	relation := validatedRelation.Relation()
	wantSignature := fixture.signatures[signatureKey].Ref()
	if relation.Signature() != wantSignature {
		t.Fatalf("relation signature = %s; want %s", relation.Signature().String(), wantSignature.String())
	}
	return relation
}

func assertSourceConformanceRejected(
	t *testing.T,
	verdict ValidationVerdict,
	wantKind ValidationVerdictKind,
	wantCodes ...DiagnosticCode,
) {
	t.Helper()
	if verdict.Kind() != wantKind {
		t.Fatalf("verdict = %s; want %s", verdict.Kind(), wantKind)
	}
	if _, admitted := verdict.(Valid); admitted {
		t.Fatal("rejected source-conformance case exposed an AdmissionBatch")
	}
	diagnostics := sourceConformanceVerdictDiagnostics(t, verdict)
	want := append([]DiagnosticCode(nil), wantCodes...)
	sort.Slice(want, func(left, right int) bool { return want[left] < want[right] })
	got := make([]DiagnosticCode, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		got = append(got, diagnostic.Code())
		assertSourceConformanceRepair(t, diagnostic)
	}
	sort.Slice(got, func(left, right int) bool { return got[left] < got[right] })
	if len(got) != len(want) {
		t.Fatalf("diagnostic codes = %v; want exactly %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("diagnostic codes = %v; want exactly %v", got, want)
		}
	}
}

func sourceConformanceVerdictDiagnostics(t *testing.T, verdict ValidationVerdict) []Diagnostic {
	t.Helper()
	switch typed := verdict.(type) {
	case Invalid:
		return typed.Diagnostics()
	case Underdetermined:
		return typed.Diagnostics()
	default:
		t.Fatalf("verdict %T has no rejection diagnostics", verdict)
		return nil
	}
}

func assertSourceConformanceRepair(t *testing.T, diagnostic Diagnostic) {
	t.Helper()
	repairs := diagnostic.RepairCandidates()
	if len(repairs) != 1 {
		t.Fatalf("%s repairs = %d; want one exact unselected repair", diagnostic.Code(), len(repairs))
	}
	repair := repairs[0]
	switch diagnostic.Code() {
	case DiagnosticEntityKindMismatch:
		if repair.Kind() != RepairChangeInput ||
			repair.HumanChoiceRequirement() != HumanChoiceRequired ||
			!strings.HasPrefix(repair.Pointer().String(), "change-candidate-at:") {
			t.Fatalf("entity-kind repair = %#v", repair)
		}
	case DiagnosticMissingSlot:
		if repair.Kind() != RepairChangeInput ||
			repair.HumanChoiceRequirement() != HumanChoiceNotClaimed ||
			!strings.HasPrefix(repair.Pointer().String(), "change-candidate-at:") {
			t.Fatalf("missing-slot repair = %#v", repair)
		}
	case DiagnosticTypeRuleUnavailable:
		if repair.Kind() != RepairRefreshSnapshot ||
			repair.HumanChoiceRequirement() != HumanChoiceNotClaimed ||
			!strings.HasPrefix(repair.Pointer().String(), "recover:g3:membership:") {
			t.Fatalf("missing-membership repair = %#v", repair)
		}
	default:
		t.Fatalf("unexpected diagnostic %s with repair %#v", diagnostic.Code(), repair)
	}
}

func assertSourceConformanceTypeAdmission(t *testing.T, verdict ValidationVerdict) {
	t.Helper()
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want type-level Valid", verdict, verdict.Kind())
	}
	if !valid.AdmissionBatch().IsValid() {
		t.Fatal("type-level Valid did not expose its sealed admission batch")
	}
	if valid.AdmissionBatch().Basis().Kind() != ContextSliceMembershipAdmissionBasis {
		t.Fatalf("admission basis = %s; want context-slice membership only", valid.AdmissionBatch().Basis().Kind().String())
	}
	// This oracle proves only that the participant designations satisfy the
	// reviewed source-derived kind projection. It does not establish that the
	// direct predicate obtains, individuate an occurrence, or turn the batch
	// into authority, truth, evidence, completion, or performed-Work receipt.
}
