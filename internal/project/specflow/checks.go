package specflow

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

// Finding levels and codes used by Checks. Codes are stable identifiers
// that downstream surfaces and tests can match on.
const (
	FindingLevelError   = "error"
	FindingLevelWarning = "warn"

	codeFieldMissing             = "spec_field_missing"
	codeStatementTypeMissing     = "spec_statement_type_missing"
	codeStatementTypeInvalid     = "spec_statement_type_invalid"
	codeClaimLayerMissing        = "spec_claim_layer_missing"
	codeClaimLayerInvalid        = "spec_claim_layer_invalid"
	codeValidUntilMissing        = "spec_valid_until_missing"
	codeValidUntilUnparseable    = "spec_valid_until_unparseable"
	codeTermNotDefined           = "spec_term_not_defined"
	codeGuardLocationMissing     = "spec_guard_location_missing"
	codeBoundaryPerspectivesMissing = "spec_boundary_perspectives_missing"
)

// Canonical sets of admissible values are defined once in the `project`
// package (SpecSectionValid*). specflow re-exports them so phase
// definitions and surface code can reference a single source of truth
// without round-tripping through SpecCheck.
var (
	ValidStatementTypes = project.SpecSectionValidStatementTypes
	ValidClaimLayers    = project.SpecSectionValidClaimLayers
	ValidGuardLocations = project.SpecSectionValidGuardLocations
)

// RequireField asserts that a SpecSection field is non-empty. Field is
// the canonical YAML key (e.g. "id", "owner", "title"). Spec layer
// fields with their own dedicated checks (statement_type, claim_layer)
// belong in the Require* check named after the field, not here.
type RequireField struct {
	Field string
}

func (c RequireField) Name() string { return "require_field:" + c.Field }

func (c RequireField) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	value := sectionFieldValue(section, c.Field)
	if strings.TrimSpace(value) != "" {
		return nil
	}

	return []project.SpecCheckFinding{{
		Level:      FindingLevelError,
		Code:       codeFieldMissing,
		Path:       section.Path,
		FieldPath:  c.Field,
		Line:       section.Line,
		SectionID:  section.ID,
		Message:    fmt.Sprintf("section %q is missing required field %q", section.ID, c.Field),
		NextAction: fmt.Sprintf("populate %q on section %q", c.Field, section.ID),
	}}
}

// RequireStatementType asserts the section declares a statement_type from
// the canonical FPF vocabulary (rule | promise | gate | explanation |
// evidence | definition).
type RequireStatementType struct{}

func (RequireStatementType) Name() string { return "require_statement_type" }

func (RequireStatementType) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	value := strings.TrimSpace(section.StatementType)
	if value == "" {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeStatementTypeMissing,
			Path:       section.Path,
			FieldPath:  "statement_type",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q is missing statement_type", section.ID),
			NextAction: fmt.Sprintf("declare statement_type on %q (one of: %s)", section.ID, strings.Join(ValidStatementTypes, ", ")),
		}}
	}

	if !containsString(ValidStatementTypes, value) {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeStatementTypeInvalid,
			Path:       section.Path,
			FieldPath:  "statement_type",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q has unknown statement_type %q", section.ID, value),
			NextAction: fmt.Sprintf("set statement_type on %q to one of: %s", section.ID, strings.Join(ValidStatementTypes, ", ")),
		}}
	}

	return nil
}

// RequireClaimLayer asserts the section declares a claim_layer from the
// canonical Levenchuk vocabulary (object | method | work).
type RequireClaimLayer struct{}

func (RequireClaimLayer) Name() string { return "require_claim_layer" }

func (RequireClaimLayer) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	value := strings.TrimSpace(section.ClaimLayer)
	if value == "" {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeClaimLayerMissing,
			Path:       section.Path,
			FieldPath:  "claim_layer",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q is missing claim_layer", section.ID),
			NextAction: fmt.Sprintf("declare claim_layer on %q (one of: %s)", section.ID, strings.Join(ValidClaimLayers, ", ")),
		}}
	}

	if !containsString(ValidClaimLayers, value) {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeClaimLayerInvalid,
			Path:       section.Path,
			FieldPath:  "claim_layer",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q has unknown claim_layer %q", section.ID, value),
			NextAction: fmt.Sprintf("set claim_layer on %q to one of: %s", section.ID, strings.Join(ValidClaimLayers, ", ")),
		}}
	}

	return nil
}

// RequireValidUntil asserts the section declares a valid_until date that
// parses as RFC3339 or YYYY-MM-DD. Refresh discipline lives at the claim
// level, not only at evidence.
type RequireValidUntil struct{}

func (RequireValidUntil) Name() string { return "require_valid_until" }

func (RequireValidUntil) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	value := strings.TrimSpace(section.ValidUntil)
	if value == "" {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeValidUntilMissing,
			Path:       section.Path,
			FieldPath:  "valid_until",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q is missing valid_until", section.ID),
			NextAction: fmt.Sprintf("declare valid_until on %q (RFC3339 or YYYY-MM-DD)", section.ID),
		}}
	}

	if !looksLikeDate(value) {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeValidUntilUnparseable,
			Path:       section.Path,
			FieldPath:  "valid_until",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q valid_until %q is not a parseable date", section.ID, value),
			NextAction: fmt.Sprintf("set valid_until on %q to RFC3339 or YYYY-MM-DD", section.ID),
		}}
	}

	return nil
}

// RequireTermDefined asserts every term referenced in section.Terms is
// defined in the project term map. Cross-section check.
type RequireTermDefined struct{}

func (RequireTermDefined) Name() string { return "require_term_defined" }

func (RequireTermDefined) RunOn(section project.SpecSection, set project.ProjectSpecificationSet) []project.SpecCheckFinding {
	if len(section.Terms) == 0 {
		return nil
	}

	defined := make(map[string]struct{}, len(set.TermMapEntries))
	for _, entry := range set.TermMapEntries {
		defined[strings.ToLower(strings.TrimSpace(entry.Term))] = struct{}{}
	}

	var findings []project.SpecCheckFinding
	for _, term := range section.Terms {
		key := strings.ToLower(strings.TrimSpace(term))
		if key == "" {
			continue
		}
		if _, ok := defined[key]; ok {
			continue
		}

		findings = append(findings, project.SpecCheckFinding{
			Level:      FindingLevelError,
			Code:       codeTermNotDefined,
			Path:       section.Path,
			FieldPath:  "terms",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q references term %q not in term map", section.ID, term),
			NextAction: fmt.Sprintf("add term %q to .haft/specs/term-map.md or remove from %q", term, section.ID),
		})
	}

	return findings
}

// RequireGuardLocation asserts the section's evidence_required entries
// declare a recognized guard_location. Applies to invariant / illegal-
// state sections; other section kinds compose this check selectively.
type RequireGuardLocation struct{}

func (RequireGuardLocation) Name() string { return "require_guard_location" }

func (RequireGuardLocation) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	if len(section.EvidenceRequired) == 0 {
		return []project.SpecCheckFinding{{
			Level:      FindingLevelError,
			Code:       codeGuardLocationMissing,
			Path:       section.Path,
			FieldPath:  "evidence_required",
			Line:       section.Line,
			SectionID:  section.ID,
			Message:    fmt.Sprintf("section %q must declare at least one guard_location in evidence_required", section.ID),
			NextAction: fmt.Sprintf("add evidence_required entry with kind in [%s] on %q", strings.Join(ValidGuardLocations, ", "), section.ID),
		}}
	}

	var findings []project.SpecCheckFinding
	for index, requirement := range section.EvidenceRequired {
		kind := strings.TrimSpace(requirement.Kind)
		if kind == "" {
			findings = append(findings, project.SpecCheckFinding{
				Level:      FindingLevelError,
				Code:       codeGuardLocationMissing,
				Path:       section.Path,
				FieldPath:  fmt.Sprintf("evidence_required[%d].kind", index),
				Line:       section.Line,
				SectionID:  section.ID,
				Message:    fmt.Sprintf("section %q evidence_required[%d] is missing guard_location kind", section.ID, index),
				NextAction: fmt.Sprintf("set evidence_required[%d].kind on %q (one of: %s)", index, section.ID, strings.Join(ValidGuardLocations, ", ")),
			})
			continue
		}

		if !containsString(ValidGuardLocations, kind) {
			findings = append(findings, project.SpecCheckFinding{
				Level:      FindingLevelError,
				Code:       codeGuardLocationMissing,
				Path:       section.Path,
				FieldPath:  fmt.Sprintf("evidence_required[%d].kind", index),
				Line:       section.Line,
				SectionID:  section.ID,
				Message:    fmt.Sprintf("section %q evidence_required[%d].kind = %q is not a recognized guard_location", section.ID, index, kind),
				NextAction: fmt.Sprintf("set evidence_required[%d].kind on %q to one of: %s", index, section.ID, strings.Join(ValidGuardLocations, ", ")),
			})
		}
	}

	return findings
}

// RequireBoundaryPerspectives asserts the section enumerates at least the
// minimum number of stakeholder perspectives in section.TargetRefs. The
// CHR-10 Boundary Norm Square calls for 4; phases that compose this
// check declare the floor explicitly.
type RequireBoundaryPerspectives struct {
	Min int
}

func (c RequireBoundaryPerspectives) Name() string {
	return fmt.Sprintf("require_boundary_perspectives:min=%d", c.Min)
}

func (c RequireBoundaryPerspectives) RunOn(section project.SpecSection, _ project.ProjectSpecificationSet) []project.SpecCheckFinding {
	min := c.Min
	if min <= 0 {
		min = 4
	}

	count := 0
	for _, ref := range section.TargetRefs {
		if strings.TrimSpace(ref) != "" {
			count++
		}
	}

	if count >= min {
		return nil
	}

	return []project.SpecCheckFinding{{
		Level:      FindingLevelError,
		Code:       codeBoundaryPerspectivesMissing,
		Path:       section.Path,
		FieldPath:  "target_refs",
		Line:       section.Line,
		SectionID:  section.ID,
		Message:    fmt.Sprintf("section %q must enumerate at least %d boundary perspectives in target_refs (got %d)", section.ID, min, count),
		NextAction: fmt.Sprintf("add boundary perspective references to target_refs on %q (CHR-10: at minimum 4 stakeholder views)", section.ID),
	}}
}

// sectionFieldValue returns the trimmed value of a known SpecSection
// field by canonical YAML key. Unknown keys return "".
func sectionFieldValue(section project.SpecSection, field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "id":
		return section.ID
	case "spec":
		return section.Spec
	case "kind":
		return section.Kind
	case "title":
		return section.Title
	case "owner":
		return section.Owner
	case "status":
		return section.Status
	case "valid_until":
		return section.ValidUntil
	case "statement_type":
		return section.StatementType
	case "claim_layer":
		return section.ClaimLayer
	case "document_kind":
		return section.DocumentKind
	case "path":
		return section.Path
	default:
		return ""
	}
}

func containsString(set []string, value string) bool {
	return slices.Contains(set, value)
}

func looksLikeDate(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}
