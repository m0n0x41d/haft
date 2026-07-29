package specflow

import (
	"github.com/m0n0x41d/haft/internal/project"
)

// Canonical PhaseIDs for the target-system and software-system spines.
const (
	PhaseTargetEnvironmentDraft PhaseID = "target.environment.draft"
	PhaseTargetRoleDraft        PhaseID = "target.role.draft"
	PhaseTargetBoundaryDraft    PhaseID = "target.boundary.draft"

	PhaseSoftwareRoleDraft                     PhaseID = "software.role.draft"
	PhaseSoftwareResponsibilityAllocationDraft PhaseID = "software.responsibility_allocation.draft"
	PhaseSoftwareFunctionalBehaviorDraft       PhaseID = "software.functional_behavior.draft"
	PhaseSoftwareProceduralBehaviorDraft       PhaseID = "software.procedural_behavior.draft"
	PhaseSoftwareInterfacesDraft               PhaseID = "software.interfaces.draft"
	PhaseSoftwareConstraintsDraft              PhaseID = "software.constraints.draft"
	PhaseSoftwareSelectedStructureDraft        PhaseID = "software.selected_structure.draft"

	// Development-version aliases keep old test fixtures and stored workflow
	// cursors readable during migration. New projections never emit these names.
	PhaseEnablingArchitectureDraft     = PhaseSoftwareRoleDraft
	PhaseEnablingWorkMethodsDraft      = PhaseSoftwareResponsibilityAllocationDraft
	PhaseEnablingEffectBoundariesDraft = PhaseSoftwareFunctionalBehaviorDraft
	PhaseEnablingAgentPolicyDraft      = PhaseSoftwareProceduralBehaviorDraft
	PhaseEnablingCommissionPolicyDraft = PhaseSoftwareInterfacesDraft
	PhaseEnablingRuntimePolicyDraft    = PhaseSoftwareConstraintsDraft
	PhaseEnablingEvidencePolicyDraft   = PhaseSoftwareSelectedStructureDraft
)

// targetEnvironmentDraft establishes the environment-change statement:
// what changes in the world when the target system runs. CHR-12 umbrella
// repair lives in the agent; carrier records the resolved statement.
var targetEnvironmentDraft = Phase{
	ID:           PhaseTargetEnvironmentDraft,
	Required:     true,
	DocumentKind: project.SpecDocumentKindTargetSystem,
	SectionKind:  "target.environment",
	PromptForUser: "What change in the world does this target system bring about? Describe " +
		"the environment-change statement: what is observably different after the system runs " +
		"that was not before. Avoid umbrella words ('quality', 'better'); name the concrete " +
		"observable that flips.",
	ContextForAgent: "Draft a TargetSystemSpec environment section. Read repository carriers " +
		"(README, package manifests, top-level docs) to ground the statement. Apply FRAME-09 " +
		"role/capability/method/work distinction internally; do NOT cite FRAME-09 in the YAML. " +
		"Apply CHR-12 umbrella-word resolution to any vague terms before writing.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer", "valid_until",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

// targetRoleDraft establishes the target-system role: what role the
// system plays in the environment-change statement. Depends on
// environment having been drafted.
var targetRoleDraft = Phase{
	ID:           PhaseTargetRoleDraft,
	Required:     true,
	DependsOn:    []PhaseID{PhaseTargetEnvironmentDraft},
	DocumentKind: project.SpecDocumentKindTargetSystem,
	SectionKind:  "target.role",
	PromptForUser: "What role does the target system play in producing the environment change? " +
		"Distinguish role (what is assigned) from capability (what it can do), method (how it " +
		"does it), and work (what it actually did). Name the role explicitly.",
	ContextForAgent: "Draft a TargetSystemSpec role section. Apply FRAME-09 strict distinction " +
		"quad in your reasoning; the YAML carries only the resolved role name and rationale. " +
		"The role section depends on the environment section already existing in active or " +
		"draft state.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer", "valid_until",
		"depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

// targetBoundaryDraft establishes boundary statements via CHR-10
// Boundary Norm Square. Requires at least 4 stakeholder perspectives in
// target_refs (the four corners: Law / Admissibility / Deontics /
// Evidence). Boundary depends on role.
var targetBoundaryDraft = Phase{
	ID:           PhaseTargetBoundaryDraft,
	Required:     true,
	DependsOn:    []PhaseID{PhaseTargetRoleDraft},
	DocumentKind: project.SpecDocumentKindTargetSystem,
	SectionKind:  "target.boundary",
	PromptForUser: "What is in scope for this target system, and what is explicitly out of scope? " +
		"Enumerate at least four boundary perspectives: who or what defines the boundary, who " +
		"is admitted across it, who has duties because of it, and what evidence shows the " +
		"boundary holds.",
	ContextForAgent: "Draft a TargetSystemSpec boundary section. Apply CHR-10 Boundary Norm " +
		"Square (Law / Admissibility / Deontics / Evidence) in your reasoning; the YAML " +
		"target_refs lists the four perspective references. The carrier must NOT mention " +
		"CHR-10. Boundary depends on the role section already existing.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer", "valid_until",
		"depends_on", "target_refs",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
		RequireBoundaryPerspectives{Min: 4},
	},
}

var softwareRoleDraft = Phase{
	ID:              PhaseSoftwareRoleDraft,
	Required:        true,
	DependsOn:       []PhaseID{PhaseTargetBoundaryDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.role",
	PromptForUser:   "What role does the software system play in realizing the target system? Name the responsibility assigned to software, not the team, harness, or coding agent.",
	ContextForAgent: "Draft a SoftwareSystemSpec role section grounded in the active TargetSystemSpec. Keep product behavior distinct from the engineering system that creates the software.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareResponsibilityAllocationDraft = Phase{
	ID:              PhaseSoftwareResponsibilityAllocationDraft,
	DependsOn:       []PhaseID{PhaseSoftwareRoleDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.responsibility_allocation",
	PromptForUser:   "Which responsibilities belong to software, humans, and external systems? Make every handoff explicit.",
	ContextForAgent: "Draft responsibility allocation only where the target behavior crosses actor boundaries. Do not describe agent autonomy or delivery workflow.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareFunctionalBehaviorDraft = Phase{
	ID:              PhaseSoftwareFunctionalBehaviorDraft,
	Required:        true,
	DependsOn:       []PhaseID{PhaseSoftwareRoleDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.functional_behavior",
	PromptForUser:   "What functions, commands, inputs, outputs, and observable results must the software provide?",
	ContextForAgent: "Draft normative software behavior. Describe externally meaningful behavior before selected implementation structure.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareProceduralBehaviorDraft = Phase{
	ID:              PhaseSoftwareProceduralBehaviorDraft,
	DependsOn:       []PhaseID{PhaseSoftwareFunctionalBehaviorDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.procedural_behavior",
	PromptForUser:   "Which software states, transitions, sequences, retries, and failure outcomes matter to correctness?",
	ContextForAgent: "Draft procedural behavior when order or state matters. Describe runtime product behavior, not the development harness runtime.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareInterfacesDraft = Phase{
	ID:              PhaseSoftwareInterfacesDraft,
	Required:        true,
	DependsOn:       []PhaseID{PhaseSoftwareFunctionalBehaviorDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.interfaces",
	PromptForUser:   "What APIs, events, data contracts, and integration boundaries expose the software behavior?",
	ContextForAgent: "Draft stable software-facing interfaces and boundary contracts. Do not list repository tooling or agent integrations unless they are product interfaces.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareConstraintsDraft = Phase{
	ID:              PhaseSoftwareConstraintsDraft,
	Required:        true,
	DependsOn:       []PhaseID{PhaseSoftwareFunctionalBehaviorDraft, PhaseSoftwareInterfacesDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.constraints",
	PromptForUser:   "Which invariants, illegal states, security constraints, and failure boundaries must the software preserve?",
	ContextForAgent: "Draft falsifiable constraints on software behavior and state. Evidence requirements belong to each claim; project-wide delivery evidence policy does not.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

var softwareSelectedStructureDraft = Phase{
	ID:              PhaseSoftwareSelectedStructureDraft,
	DependsOn:       []PhaseID{PhaseSoftwareConstraintsDraft},
	DocumentKind:    project.SpecDocumentKindSoftwareSystem,
	SectionKind:     "software.selected_structure",
	PromptForUser:   "Which selected components, layers, ownership boundaries, and dependency rules realize the specified software behavior?",
	ContextForAgent: "Land only selected durable structure. Alternatives and rationale remain in SolutionPortfolio and DecisionRecord; plans and work remain outside the spec.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on",
	},
	Checks: []Check{
		RequireField{Field: "id"},
		RequireField{Field: "spec"},
		RequireField{Field: "kind"},
		RequireField{Field: "title"},
		RequireField{Field: "owner"},
		RequireStatementType{},
		RequireClaimLayer{},
		RequireValidUntil{},
	},
}

// PhaseRegistry returns the canonical ordered catalog for the target-system
// and software-system spines. NextStep routes only phases marked Required;
// optional phases remain discoverable for projects whose behavior needs them.
func PhaseRegistry() []Phase {
	return []Phase{
		targetEnvironmentDraft,
		targetRoleDraft,
		targetBoundaryDraft,
		softwareRoleDraft,
		softwareResponsibilityAllocationDraft,
		softwareFunctionalBehaviorDraft,
		softwareProceduralBehaviorDraft,
		softwareInterfacesDraft,
		softwareConstraintsDraft,
		softwareSelectedStructureDraft,
	}
}

// FindPhase returns the phase with the given ID, or false if absent.
func FindPhase(id PhaseID) (Phase, bool) {
	for _, phase := range PhaseRegistry() {
		if phase.ID == id {
			return phase, true
		}
	}
	return Phase{}, false
}
