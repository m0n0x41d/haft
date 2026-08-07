package project

import (
	"strings"
)

type SoftwareSystemMigrationDisposition string

const (
	SoftwareSystemMigrationSafe       SoftwareSystemMigrationDisposition = "safe"
	SoftwareSystemMigrationRetired    SoftwareSystemMigrationDisposition = "retired"
	SoftwareSystemMigrationUnresolved SoftwareSystemMigrationDisposition = "unresolved"
)

type SoftwareSystemMigrationSection struct {
	ID          string                             `json:"id"`
	LegacyKind  string                             `json:"legacy_kind"`
	NewKind     string                             `json:"new_kind,omitempty"`
	Disposition SoftwareSystemMigrationDisposition `json:"disposition"`
	Reason      string                             `json:"reason"`
}

type SoftwareSystemMigrationPlan struct {
	From                   string                           `json:"from"`
	To                     string                           `json:"to"`
	Sections               []SoftwareSystemMigrationSection `json:"sections"`
	UnresolvedCount        int                              `json:"unresolved_count"`
	Applicable             bool                             `json:"applicable"`
	PreservesIDs           bool                             `json:"preserves_ids"`
	RequiresSpecSync       bool                             `json:"requires_spec_sync"`
	BaselinesRequireReopen bool                             `json:"baselines_require_reopen"`
}

func PlanSoftwareSystemMigration(sections []SpecSection) SoftwareSystemMigrationPlan {
	items := make([]SoftwareSystemMigrationSection, 0, len(sections))
	unresolved := 0

	for _, section := range sections {
		item := classifySoftwareSystemMigrationSection(section)
		items = append(items, item)
		if item.Disposition == SoftwareSystemMigrationUnresolved {
			unresolved++
		}
	}

	return SoftwareSystemMigrationPlan{
		From:                   "enabling-system",
		To:                     "software-system",
		Sections:               items,
		UnresolvedCount:        unresolved,
		Applicable:             len(items) > 0 && unresolved == 0,
		PreservesIDs:           true,
		RequiresSpecSync:       true,
		BaselinesRequireReopen: true,
	}
}

func RenderSoftwareSystemCarrier(legacy string) string {
	retiredSectionIDs := softwareSystemMigrationRetiredSectionIDs(legacy)
	filtered := omitMarkdownSpecSections(legacy, retiredSectionIDs)
	replacements := []struct {
		old string
		new string
	}{
		{old: "# Enabling System Spec", new: "# Software System Spec"},
		{old: "spec: enabling-system", new: "spec: software-system"},
		{old: "document_kind: enabling-system", new: "document_kind: software-system"},
		{old: "kind: enabling.architecture", new: "kind: software.selected_structure"},
		{old: "kind: creator-role", new: "kind: software.role"},
		{old: "kind: enabling_system", new: "kind: software_system"},
		{old: "id: enabling_system", new: "id: software_system"},
	}

	result := filtered
	for _, replacement := range replacements {
		result = strings.ReplaceAll(result, replacement.old, replacement.new)
	}
	return result
}

func IsDefaultSoftwareSystemCarrier(content []byte) bool {
	actual := strings.TrimSpace(string(content))
	expected := strings.TrimSpace(softwareSystemSpecCarrierContent())
	return actual == expected
}

func NeedsSoftwareSystemMigration(haftDir string) bool {
	return legacyEnablingSystemCarrierExists(haftDir)
}

func softwareSystemMigrationRetiredSectionIDs(legacy string) map[string]bool {
	specSet := ProjectSpecificationSetFromDocuments([]SpecDocumentInput{{
		Path:    ".haft/specs/enabling-system.md",
		Kind:    string(SpecDocumentKindEnablingSystem),
		Content: legacy,
	}})
	retired := map[string]bool{}
	for _, section := range specSet.Sections {
		item := classifySoftwareSystemMigrationSection(section)
		if item.Disposition == SoftwareSystemMigrationRetired {
			retired[section.ID] = true
		}
	}
	return retired
}

func omitMarkdownSpecSections(source string, omittedIDs map[string]bool) string {
	if len(omittedIDs) == 0 {
		return source
	}
	parts := strings.Split(source, "\n## ")
	kept := []string{parts[0]}
	for _, part := range parts[1:] {
		chunk := "## " + part
		if markdownSpecSectionOmitted(chunk, omittedIDs) {
			continue
		}
		kept = append(kept, chunk)
	}
	return strings.Join(kept, "\n")
}

func markdownSpecSectionOmitted(chunk string, omittedIDs map[string]bool) bool {
	for _, line := range strings.Split(chunk, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "id:") {
			continue
		}
		id := strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
		return omittedIDs[id]
	}
	return false
}

func classifySoftwareSystemMigrationSection(section SpecSection) SoftwareSystemMigrationSection {
	kind := strings.TrimSpace(section.Kind)
	item := SoftwareSystemMigrationSection{
		ID:         section.ID,
		LegacyKind: kind,
	}
	if softwareSystemMigrationSectionRetired(section) {
		item.Disposition = SoftwareSystemMigrationRetired
		item.Reason = "deprecated or superseded legacy policy remains historical and does not become a software contract"
		return item
	}
	if section.Malformed || strings.TrimSpace(section.ID) == "" {
		item.Disposition = SoftwareSystemMigrationUnresolved
		item.Reason = "malformed legacy section must be repaired before migration"
		return item
	}
	if softwareSystemMigrationKind(kind) {
		item.NewKind = kind
		item.Disposition = SoftwareSystemMigrationSafe
		item.Reason = "explicit software-system classification is preserved"
		return item
	}

	switch kind {
	case "enabling.architecture":
		item.NewKind = "software.selected_structure"
		item.Disposition = SoftwareSystemMigrationSafe
		item.Reason = "selected software structure retains the section identity"
	case "creator-role":
		if softwareSystemMigrationPlaceholder(section) {
			item.NewKind = "software.role"
			item.Disposition = SoftwareSystemMigrationSafe
			item.Reason = "development placeholder maps to the software role placeholder"
			break
		}
		item.Disposition = SoftwareSystemMigrationUnresolved
		item.Reason = "creator-role is safe only for the exact draft carrier placeholder; classify product responsibility explicitly"
	default:
		item.Disposition = SoftwareSystemMigrationUnresolved
		item.Reason = "classify as a software contract or retire as harness/delivery policy before apply"
	}

	return item
}

func softwareSystemMigrationSectionRetired(section SpecSection) bool {
	status := strings.ToLower(strings.TrimSpace(section.Status))
	return status == string(SpecSectionStateDeprecated) || status == string(SpecSectionStateSuperseded)
}

func softwareSystemMigrationKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "software.role",
		"software.responsibility_allocation",
		"software.functional_behavior",
		"software.procedural_behavior",
		"software.interfaces",
		"software.constraints",
		"software.selected_structure":
		return true
	default:
		return false
	}
}

func softwareSystemMigrationPlaceholder(section SpecSection) bool {
	return strings.TrimSpace(section.ID) == "ES.placeholder.001" &&
		strings.EqualFold(strings.TrimSpace(section.Status), string(SpecSectionStateDraft)) &&
		strings.EqualFold(strings.TrimSpace(section.ClaimLayer), "carrier")
}
