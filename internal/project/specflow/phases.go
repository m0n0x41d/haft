package specflow

import (
	"github.com/m0n0x41d/haft/internal/project"
)

// Canonical PhaseIDs for the v7.0 target-system spine and enabling-system
// spine. Stable identifiers surfaces and tests can match on; new phases
// are appended only.
const (
	PhaseTargetEnvironmentDraft PhaseID = "target.environment.draft"
	PhaseTargetRoleDraft        PhaseID = "target.role.draft"
	PhaseTargetBoundaryDraft    PhaseID = "target.boundary.draft"

	PhaseEnablingArchitectureDraft     PhaseID = "enabling.architecture.draft"
	PhaseEnablingWorkMethodsDraft      PhaseID = "enabling.work_methods.draft"
	PhaseEnablingEffectBoundariesDraft PhaseID = "enabling.effect_boundaries.draft"
	PhaseEnablingAgentPolicyDraft      PhaseID = "enabling.agent_policy.draft"
	PhaseEnablingCommissionPolicyDraft PhaseID = "enabling.commission_policy.draft"
	PhaseEnablingRuntimePolicyDraft    PhaseID = "enabling.runtime_policy.draft"
	PhaseEnablingEvidencePolicyDraft   PhaseID = "enabling.evidence_policy.draft"
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

// enablingArchitectureDraft drafts the EnablingSystemSpec architecture
// section. Depends on the target boundary so the enabling structure is
// chosen *for* a known target rather than in a vacuum.
var enablingArchitectureDraft = Phase{
	ID:           PhaseEnablingArchitectureDraft,
	DependsOn:    []PhaseID{PhaseTargetBoundaryDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.architecture",
	PromptForUser: "What is the layered architecture of the enabling system " +
		"(the team, code, infrastructure that builds and operates the target)? " +
		"Name the layers, the dependency direction between them, and which layer " +
		"owns each capability. Layers are not folders — they are responsibilities " +
		"with a one-way dependency rule.",
	ContextForAgent: "Draft an EnablingSystemSpec architecture section. Read " +
		"package manifests, top-level directory structure, and any existing " +
		"architecture docs to ground the layers in the actual code. Apply the " +
		"FPF Signature Stack (L0..L4 layered claim landing) and the spec/" +
		"enabling-system/ARCHITECTURE.md convention internally; the YAML carries " +
		"the resolved layer names and dependency rules, not FPF citations.",
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

// enablingWorkMethodsDraft prescribes how each load-bearing artifact
// (specs, decisions, commissions, runtime runs, evidence) is produced.
var enablingWorkMethodsDraft = Phase{
	ID:           PhaseEnablingWorkMethodsDraft,
	DependsOn:    []PhaseID{PhaseEnablingArchitectureDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.work_methods",
	PromptForUser: "How is each load-bearing artifact produced? For specs, " +
		"decisions, commissions, runtime runs, and evidence — name the actor, " +
		"the trigger, and the deterministic check that closes each step.",
	ContextForAgent: "Draft an EnablingSystemSpec work-methods section. Apply " +
		"FRAME-09 (role/capability/method/work distinction) so each method names " +
		"its actor and what it actually produces, not what it 'could' do. " +
		"Apply X-STATEMENT-TYPE: each method is a duty (rule), not an " +
		"explanation; decompose mixed types.",
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

// enablingEffectBoundariesDraft declares which actor/surface may mutate
// what. Runs after architecture so the boundaries reference real layers.
var enablingEffectBoundariesDraft = Phase{
	ID:           PhaseEnablingEffectBoundariesDraft,
	DependsOn:    []PhaseID{PhaseEnablingArchitectureDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.effect_boundaries",
	PromptForUser: "Which actors and surfaces are allowed to mutate which " +
		"resources? Enumerate the effect boundaries: what each layer or surface " +
		"can write, what is read-only, and where mutation requires explicit " +
		"authorization (decision, commission, scope envelope).",
	ContextForAgent: "Draft an EnablingSystemSpec effect-boundaries section. " +
		"Apply CHR-10 Boundary Norm Square in your reasoning to decompose mixed " +
		"boundary statements (Law / Admissibility / Deontics / Evidence); " +
		"target_refs lists the four perspectives where relevant. Carrier text " +
		"records the resolved rules, not the CHR-10 vocabulary.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on", "target_refs",
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

// enablingAgentPolicyDraft declares which host agents are supported and
// what autonomy bounds apply.
var enablingAgentPolicyDraft = Phase{
	ID:           PhaseEnablingAgentPolicyDraft,
	DependsOn:    []PhaseID{PhaseEnablingEffectBoundariesDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.agent_policy",
	PromptForUser: "Which host agents are supported by this project, and what " +
		"autonomy bounds apply? State which agents may invoke which Haft tools, " +
		"under what authorization, and which actions remain human-decision " +
		"gates. (v7 Haft product supports Claude Code and Codex; project-local " +
		"policy may be narrower.)",
	ContextForAgent: "Draft an EnablingSystemSpec agent-policy section. Apply " +
		"X-TRANSFORMER (the human is the principal; the agent does not " +
		"self-improve) and CHR-10 (admissibility for what the agent may do, " +
		"deontics for the human's duties when delegating). The carrier names " +
		"agents and bounds; FPF references stay in your reasoning.",
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

// enablingCommissionPolicyDraft prescribes how WorkCommissions are
// authorized, scoped, and retired in this project.
var enablingCommissionPolicyDraft = Phase{
	ID:           PhaseEnablingCommissionPolicyDraft,
	DependsOn:    []PhaseID{PhaseEnablingEffectBoundariesDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.commission_policy",
	PromptForUser: "How does this project authorize work? State the rules for " +
		"creating, scoping, and retiring WorkCommissions: who may create one, " +
		"what the default scope (allowed_paths / forbidden_paths) is, and what " +
		"freshness gates must pass before execution.",
	ContextForAgent: "Draft an EnablingSystemSpec commission-policy section. " +
		"Apply X-SCOPE (every claim has explicit where + under what + when) so " +
		"scope rules are not 'always' / 'never' but bounded by repo, paths, " +
		"and time. WorkCommission is bounded authorization, not execution.",
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

// enablingRuntimePolicyDraft declares which surface owns runtime
// lifecycle and how runtime runs are isolated and observed.
var enablingRuntimePolicyDraft = Phase{
	ID:           PhaseEnablingRuntimePolicyDraft,
	DependsOn:    []PhaseID{PhaseEnablingCommissionPolicyDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.runtime_policy",
	PromptForUser: "Which surface starts and stops the harness runtime, and " +
		"how is each RuntimeRun isolated and observed? CLI / Desktop typically " +
		"own the runtime lifecycle; the MCP plugin does not own long-running " +
		"execution. State the project's rules.",
	ContextForAgent: "Draft an EnablingSystemSpec runtime-policy section. Apply " +
		"FRAME-09: the runtime's role is execute, the operator's role is " +
		"start/observe/stop. Carrier records who owns lifecycle, isolation " +
		"rules, and observability requirements; do not derive ownership from " +
		"folder names alone.",
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

// enablingEvidencePolicyDraft prescribes what counts as evidence in this
// project and how freshness is enforced.
var enablingEvidencePolicyDraft = Phase{
	ID:           PhaseEnablingEvidencePolicyDraft,
	DependsOn:    []PhaseID{PhaseEnablingRuntimePolicyDraft},
	DocumentKind: project.SpecDocumentKindEnablingSystem,
	SectionKind:  "enabling.evidence_policy",
	PromptForUser: "What evidence does this project require to call a claim " +
		"verified, and how is freshness enforced? State the admissible evidence " +
		"kinds (test, measurement, audit, etc.), the minimum congruence level " +
		"per claim class, and the refresh triggers.",
	ContextForAgent: "Draft an EnablingSystemSpec evidence-policy section. " +
		"Apply VER-01 (every claim anchored to evidence), VER-02 (decay -> " +
		"valid_until on evidence), VER-03 (R_eff is min not average), and " +
		"VER-07 (refresh triggers). The carrier records the project's actual " +
		"requirements; FPF citations stay in your reasoning.",
	ExpectedFields: []string{
		"id", "spec", "kind", "title", "owner", "statement_type", "claim_layer",
		"valid_until", "depends_on", "evidence_required",
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
		RequireGuardLocation{},
	},
}

// PhaseRegistry returns the canonical ordered list of phases for the
// v7.0 target-system + enabling-system spine. Order matters: NextStep
// walks this list and returns the first phase whose dependencies are
// satisfied and whose section is missing or incomplete.
//
// Enabling phases depend on target.boundary.draft so the enabling
// structure is chosen for a known target rather than in a vacuum
// (per spec/target-system/PROJECT_ONBOARDING_CONTRACT.md "Enabling
// spec starts only after target spec is admissible").
func PhaseRegistry() []Phase {
	return []Phase{
		targetEnvironmentDraft,
		targetRoleDraft,
		targetBoundaryDraft,
		enablingArchitectureDraft,
		enablingWorkMethodsDraft,
		enablingEffectBoundariesDraft,
		enablingAgentPolicyDraft,
		enablingCommissionPolicyDraft,
		enablingRuntimePolicyDraft,
		enablingEvidencePolicyDraft,
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
