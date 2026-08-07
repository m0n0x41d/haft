package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// codeIntelDoctrine is the global conditional code-preflight invariant. Exact
// action fields and recovery contracts belong to tools/list.
const codeIntelDoctrine = `# Haft code preflight

Use ` + "`haft_query(action=\"explore\", query=\"...\")`" + ` to orient an area or
flow when no exact symbol is known. Before a non-mechanical edit where recorded
governance may be material, use ` + "`code_context`" + ` or ` + "`impact`" + ` on
the actual target.

Treat returned reasoning as relevance context, not automatic governing
authority. Inspect exact scope, status, coverage, and limiting reasons before
relying on a governing claim. Incomplete traversal and an empty caller list are
not safety claims. Purely mechanical work may record graph orientation as
` + "`not_applicable`" + `.`

// methodPackDoctrine is emitted in MCP initialize instructions so hosts learn
// the pull/close protocol before any tool call. Keep it short: the full method
// cards are task-local and retrieved through haft_method(action="pull").
const methodPackDoctrine = `# Haft MethodPack

When the selected scope requires the SWE MethodPack, call
` + "`haft_method(action=\"pull\")`" + ` before non-trivial code work, keep its
` + "`pull_id`" + `, and call ` + "`haft_method(action=\"close\")`" + ` with changed
files, gate evidence, verification, and explicit waivers before claiming
completion. Recover an open run with ` + "`status`" + ` or ` + "`show`" + ` after
compaction. A missing or non-applicable MethodPack does not block otherwise
authorized Work. Mechanical edits need no manufactured ceremony.`

// projectMemoryOrientation is deliberately limited to the four global memory
// and authority invariants allowed in MCP initialize. Task fields, project
// policy, profile applicability, and recovery procedures belong to tools/list
// or their dedicated read surfaces.
const projectMemoryOrientation = `# Haft project memory

## Conditional memory orientation

Orient structured project memory only when current Work is context-heavy,
multi-session, or reliance-bearing. Use ` + "`haft_query`" + ` with
` + "`action=\"memory\"`" + ` and its nested ` + "`memory_request`" + ` to resolve
a known name or alias, select exact identity by the current use rather than
rank, then read the smallest relevant neighborhood. Mode-specific fields are
defined by the tool schema. Missing basis, known absence, or abstention stays
visible and does not block unrelated authorized Work.

## Persistence gate

Do not persist merely because memory is empty or resolution returns
` + "`known_absent`" + `. Persistence requires an explicit operator save/record
request or a concrete durability-requiring receiving use, operator-named or
agent-inferred from current Work, with request provenance. When that use makes
stable identity necessary and identity coordinates are recoverable, establish
the minimum EntityOfConcern without asking for separate permission. Binding
decisions and commissions remain manual.

## Status is not authority

Use ` + "`haft_query(action=\"status\")`" + ` only when current Work relies on
recorded project state. Status is a read-only attention surface, not
authorization, evidence truth, gate passage, or a global next step. Drift and
maintenance cues do not block unrelated already-authorized Work.

## Manual decision and commission authority

Binding a decision or commissioning Work requires an explicit operator/manual
act. Generated text, recommendations, tool arguments, and schemas are not
approval receipts.`

// composeServerInstructions builds only the global MCP initialize invariants.
// Project workflow, profile applicability, exact task fields, effects, and
// recovery remain available through their dedicated status and tool surfaces.
type serverInstructionCompositionKind string

const (
	serverInstructionCompositionResolved        serverInstructionCompositionKind = "resolved"
	serverInstructionCompositionUnderdetermined serverInstructionCompositionKind = "underdetermined"
)

// scopedServerInstructionComposition is one profile-bound host-doctrine
// projection. Underdetermined is a single neutral typed result; it does not
// emit partial SWE pressure from whichever capability happened to resolve.
type scopedServerInstructionComposition struct {
	kind            serverInstructionCompositionKind
	instructions    string
	applicabilities []projectprofile.ScopedCapabilityApplicability
	included        []projectprofile.Capability
}

func (result scopedServerInstructionComposition) Valid() bool {
	if strings.TrimSpace(result.instructions) == "" ||
		len(result.applicabilities) != 2 {
		return false
	}
	methodPack := result.applicabilities[0]
	codeDoctrine := result.applicabilities[1]
	if !methodPack.Valid() ||
		!codeDoctrine.Valid() ||
		methodPack.Capability() != projectprofile.SWEMethodPackCapability ||
		codeDoctrine.Capability() != projectprofile.CodeDoctrineAndIndexCapability ||
		methodPack.ScopeID() != codeDoctrine.ScopeID() ||
		methodPack.ProfilePayloadDigest() != codeDoctrine.ProfilePayloadDigest() {
		return false
	}
	underdetermined := methodPack.Kind() == projectprofile.CapabilityUnderdetermined ||
		codeDoctrine.Kind() == projectprofile.CapabilityUnderdetermined
	if underdetermined {
		return result.kind == serverInstructionCompositionUnderdetermined &&
			len(result.included) == 0
	}
	expected := requiredServerInstructionCapabilities(result.applicabilities)
	return result.kind == serverInstructionCompositionResolved &&
		slices.Equal(result.included, expected)
}

func (result scopedServerInstructionComposition) Kind() serverInstructionCompositionKind {
	if !result.Valid() {
		return ""
	}
	return result.kind
}

func (result scopedServerInstructionComposition) Instructions() string {
	if !result.Valid() {
		return ""
	}
	return result.instructions
}

func (result scopedServerInstructionComposition) Applicabilities() []projectprofile.ScopedCapabilityApplicability {
	if !result.Valid() {
		return nil
	}
	return append(
		[]projectprofile.ScopedCapabilityApplicability{},
		result.applicabilities...,
	)
}

func composeServerInstructionsForApplicability(
	w *project.Workflow,
	applicability project.ProjectSpecificationSetApplicability,
) (scopedServerInstructionComposition, error) {
	methodPack, err := applicability.ScopedCapabilityApplicability(
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		return scopedServerInstructionComposition{}, err
	}
	codeDoctrine, err := applicability.ScopedCapabilityApplicability(
		projectprofile.CodeDoctrineAndIndexCapability,
	)
	if err != nil {
		return scopedServerInstructionComposition{}, err
	}
	applicabilities := []projectprofile.ScopedCapabilityApplicability{
		methodPack,
		codeDoctrine,
	}
	kind := serverInstructionCompositionResolved
	included := requiredServerInstructionCapabilities(applicabilities)
	if methodPack.Kind() == projectprofile.CapabilityUnderdetermined ||
		codeDoctrine.Kind() == projectprofile.CapabilityUnderdetermined {
		kind = serverInstructionCompositionUnderdetermined
		included = nil
	}
	result := scopedServerInstructionComposition{
		kind:            kind,
		instructions:    composeServerInstructionParts(w, included),
		applicabilities: applicabilities,
		included:        included,
	}
	if !result.Valid() {
		return scopedServerInstructionComposition{}, fmt.Errorf(
			"scoped server-instruction composition is invalid",
		)
	}
	return result, nil
}

func composeServerInstructionsForProfileResolution(
	w *project.Workflow,
	resolution projectSpecificationApplicabilityResolution,
) (string, error) {
	applicability, _, resolved := resolution.Resolved()
	if resolved {
		composition, err := composeServerInstructionsForApplicability(
			w,
			applicability,
		)
		if err != nil {
			return "", err
		}
		return composition.Instructions(), nil
	}
	return composeServerInstructionParts(w, nil), nil
}

func composeServerInstructionsForUnavailableProfile(
	w *project.Workflow,
) string {
	return composeServerInstructionParts(w, nil)
}

func requiredServerInstructionCapabilities(
	applicabilities []projectprofile.ScopedCapabilityApplicability,
) []projectprofile.Capability {
	required := make([]projectprofile.Capability, 0, len(applicabilities))
	for _, applicability := range applicabilities {
		if applicability.Kind() == projectprofile.CapabilityRequired {
			required = append(required, applicability.Capability())
		}
	}
	return required
}

func composeServerInstructionParts(
	_ *project.Workflow,
	_ []projectprofile.Capability,
) string {
	parts := []string{
		projectMemoryOrientation,
		methodPackDoctrine,
		codeIntelDoctrine,
	}
	return strings.Join(parts, "\n\n")
}
