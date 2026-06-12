package specflow

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

// LifecycleState is the operator-facing state of the spec lifecycle.
// It is a projection over WorkflowIntent + derived spec state, not a
// second source of authority.
type LifecycleState string

const (
	LifecycleStateReady          LifecycleState = "ready"
	LifecycleStateNeedsAction    LifecycleState = "needs_action"
	LifecycleStateNeedsHumanGate LifecycleState = "needs_human_gate"
	LifecycleStateNeedsTriage    LifecycleState = "needs_triage"
)

// LifecycleAction is the next lifecycle projection action surfaced by
// `haft spec status`, MCP, and agent skills. Mutations such as rebaseline and
// reopen remain explicit commands listed under AllowedCommands.
type LifecycleAction string

const (
	LifecycleActionNone    LifecycleAction = "none"
	LifecycleActionDraft   LifecycleAction = "draft"
	LifecycleActionClarify LifecycleAction = "clarify"
	LifecycleActionApprove LifecycleAction = "approve"
	LifecycleActionTriage  LifecycleAction = "triage"
)

// SpecLifecycleProjection is the typed UX contract surfaces render. It
// keeps object identity, carrier path, findings, and the underlying
// WorkflowIntent recoverable so the projection cannot become authority.
type SpecLifecycleProjection struct {
	State            LifecycleState             `json:"state"`
	Action           LifecycleAction            `json:"action"`
	Object           string                     `json:"object"`
	Phase            PhaseID                    `json:"phase,omitempty"`
	Audience         Audience                   `json:"audience,omitempty"`
	DocumentKind     project.SpecDocumentKind   `json:"document_kind,omitempty"`
	SectionKind      string                     `json:"section_kind,omitempty"`
	SectionID        string                     `json:"section_id,omitempty"`
	Carrier          string                     `json:"carrier,omitempty"`
	Why              string                     `json:"why"`
	HumanGate        string                     `json:"human_gate,omitempty"`
	AllowedCommands  []string                   `json:"allowed_commands,omitempty"`
	BlockedCommands  []string                   `json:"blocked_commands,omitempty"`
	BlockingFindings []project.SpecCheckFinding `json:"blocking_findings,omitempty"`
	WorkflowIntent   WorkflowIntent             `json:"workflow_intent"`
}

// ProjectLifecycle derives the next operator-facing lifecycle action.
// It performs no I/O and mutates nothing.
func ProjectLifecycle(state SpecState) SpecLifecycleProjection {
	intent := NextStep(state)
	section := projectionSection(state, intent)
	action := lifecycleAction(intent)

	return SpecLifecycleProjection{
		State:            lifecycleState(action, intent),
		Action:           action,
		Object:           lifecycleObject(action),
		Phase:            intent.Phase,
		Audience:         intent.Audience,
		DocumentKind:     intent.DocumentKind,
		SectionKind:      intent.SectionKind,
		SectionID:        strings.TrimSpace(section.ID),
		Carrier:          lifecycleCarrier(intent.DocumentKind, section),
		Why:              lifecycleWhy(intent, section),
		HumanGate:        lifecycleHumanGate(action),
		AllowedCommands:  lifecycleAllowedCommands(action, intent, section),
		BlockedCommands:  lifecycleBlockedCommands(action, section),
		BlockingFindings: intent.BlockingFindings,
		WorkflowIntent:   intent,
	}
}

func lifecycleAction(intent WorkflowIntent) LifecycleAction {
	if intent.Terminal {
		return LifecycleActionNone
	}
	if len(intent.BlockingFindings) == 0 {
		return LifecycleActionDraft
	}
	if findingsContainCode(intent.BlockingFindings, codeSpecSectionDrifted) {
		return LifecycleActionTriage
	}
	if findingsContainCode(intent.BlockingFindings, codeSpecSectionStale) {
		return LifecycleActionTriage
	}
	if findingsOnlyContainCode(intent.BlockingFindings, codeSpecSectionNeedsBaseline) {
		return LifecycleActionApprove
	}
	return LifecycleActionClarify
}

func lifecycleState(action LifecycleAction, intent WorkflowIntent) LifecycleState {
	if intent.Terminal {
		return LifecycleStateReady
	}
	if action == LifecycleActionApprove {
		return LifecycleStateNeedsHumanGate
	}
	if action == LifecycleActionTriage {
		return LifecycleStateNeedsTriage
	}
	return LifecycleStateNeedsAction
}

func lifecycleObject(action LifecycleAction) string {
	if action == LifecycleActionApprove {
		return "SpecSectionBaseline"
	}
	return "SpecSection"
}

func lifecycleWhy(intent WorkflowIntent, section project.SpecSection) string {
	if intent.Terminal {
		return strings.TrimSpace(intent.Reason)
	}
	if strings.TrimSpace(intent.Reason) != "" {
		return strings.TrimSpace(intent.Reason)
	}
	if strings.TrimSpace(section.ID) != "" && !sectionIsActive(section) {
		return fmt.Sprintf("section %q exists but is not active; continue drafting or mark it active after human review", section.ID)
	}
	if strings.TrimSpace(section.ID) == "" {
		return fmt.Sprintf("no section of kind %q exists yet", intent.SectionKind)
	}
	return fmt.Sprintf("phase %q is not satisfied", intent.Phase)
}

func lifecycleHumanGate(action LifecycleAction) string {
	if action == LifecycleActionApprove {
		return "operator reviews the active SpecSection and records a baseline"
	}
	if action == LifecycleActionTriage {
		return "operator chooses rebaseline, reopen, deprecate, or carrier rollback"
	}
	return ""
}

func lifecycleAllowedCommands(action LifecycleAction, intent WorkflowIntent, section project.SpecSection) []string {
	switch action {
	case LifecycleActionNone:
		return []string{"haft spec check", "haft spec coverage"}
	case LifecycleActionDraft:
		return []string{
			fmt.Sprintf("edit %s with a yaml spec-section for %s", lifecycleCarrier(intent.DocumentKind, section), intent.SectionKind),
			"haft spec check",
			"haft spec status",
		}
	case LifecycleActionClarify:
		return []string{
			fmt.Sprintf("resolve blocking findings in %s", lifecycleCarrier(intent.DocumentKind, section)),
			"haft spec check",
			"haft spec status",
		}
	case LifecycleActionApprove:
		return []string{fmt.Sprintf("haft spec approve %s", section.ID)}
	case LifecycleActionTriage:
		return []string{
			fmt.Sprintf("haft spec rebaseline %s --reason <reason>", section.ID),
			fmt.Sprintf("haft spec reopen %s --reason <reason>", section.ID),
			fmt.Sprintf("rollback carrier edit in %s", lifecycleCarrier(intent.DocumentKind, section)),
		}
	default:
		return nil
	}
}

func lifecycleBlockedCommands(action LifecycleAction, section project.SpecSection) []string {
	if action == LifecycleActionDraft || action == LifecycleActionClarify {
		return []string{"haft spec approve <section-id>", "haft spec rebaseline <section-id>"}
	}
	if action == LifecycleActionApprove {
		return []string{"haft spec rebaseline " + section.ID}
	}
	return nil
}

func lifecycleCarrier(kind project.SpecDocumentKind, section project.SpecSection) string {
	if strings.TrimSpace(section.Path) != "" {
		return strings.TrimSpace(section.Path)
	}
	switch kind {
	case project.SpecDocumentKindTargetSystem:
		return ".haft/specs/target-system.md"
	case project.SpecDocumentKindEnablingSystem:
		return ".haft/specs/enabling-system.md"
	case project.SpecDocumentKindTermMap:
		return ".haft/specs/term-map.md"
	default:
		return ""
	}
}

func projectionSection(state SpecState, intent WorkflowIntent) project.SpecSection {
	if sectionID := firstFindingSectionID(intent.BlockingFindings); sectionID != "" {
		return sectionByID(state.Set.Sections, sectionID)
	}

	phase, ok := FindPhase(intent.Phase)
	if !ok {
		return project.SpecSection{}
	}

	sections := state.SectionsForPhase(phase)
	if len(sections) == 0 {
		return project.SpecSection{}
	}
	return sections[0]
}

func firstFindingSectionID(findings []project.SpecCheckFinding) string {
	for _, finding := range findings {
		if strings.TrimSpace(finding.SectionID) != "" {
			return strings.TrimSpace(finding.SectionID)
		}
	}
	return ""
}

func sectionByID(sections []project.SpecSection, sectionID string) project.SpecSection {
	for _, section := range sections {
		if strings.TrimSpace(section.ID) == sectionID {
			return section
		}
	}
	return project.SpecSection{ID: sectionID}
}

func findingsContainCode(findings []project.SpecCheckFinding, code string) bool {
	for _, finding := range findings {
		if strings.TrimSpace(finding.Code) == code {
			return true
		}
	}
	return false
}

func findingsOnlyContainCode(findings []project.SpecCheckFinding, code string) bool {
	if len(findings) == 0 {
		return false
	}
	for _, finding := range findings {
		if strings.TrimSpace(finding.Code) != code {
			return false
		}
	}
	return true
}
