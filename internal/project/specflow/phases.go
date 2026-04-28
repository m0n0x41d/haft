package specflow

import (
	"github.com/m0n0x41d/haft/internal/project"
)

// Canonical PhaseIDs for the v7.0 target-system spine. Stable identifiers
// surfaces and tests can match on; new phases are appended only.
const (
	PhaseTargetEnvironmentDraft PhaseID = "target.environment.draft"
	PhaseTargetRoleDraft        PhaseID = "target.role.draft"
	PhaseTargetBoundaryDraft    PhaseID = "target.boundary.draft"
)

// targetEnvironmentDraft establishes the environment-change statement:
// what changes in the world when the target system runs. CHR-12 umbrella
// repair lives in the agent; carrier records the resolved statement.
var targetEnvironmentDraft = Phase{
	ID:           PhaseTargetEnvironmentDraft,
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

// PhaseRegistry returns the canonical ordered list of phases for the
// v7.0 target-system spine. Order matters: NextStep walks this list and
// returns the first phase whose dependencies are satisfied and whose
// section is missing or incomplete.
func PhaseRegistry() []Phase {
	return []Phase{
		targetEnvironmentDraft,
		targetRoleDraft,
		targetBoundaryDraft,
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
