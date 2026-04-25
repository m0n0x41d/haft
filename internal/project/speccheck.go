package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type SpecCheckReport struct {
	Level     string              `json:"level"`
	Documents []SpecCheckDocument `json:"documents"`
	Findings  []SpecCheckFinding  `json:"findings"`
	Summary   SpecCheckSummary    `json:"summary"`
}

type SpecCheckDocument struct {
	Path               string `json:"path"`
	Kind               string `json:"kind"`
	SpecSections       int    `json:"spec_sections"`
	ActiveSpecSections int    `json:"active_spec_sections"`
	TermMapEntries     int    `json:"term_map_entries"`
}

type SpecCheckFinding struct {
	Level     string `json:"level"`
	Code      string `json:"code"`
	Path      string `json:"path"`
	FieldPath string `json:"field_path,omitempty"`
	Line      int    `json:"line,omitempty"`
	SectionID string `json:"section_id,omitempty"`
	Message   string `json:"message"`
}

type SpecCheckSummary struct {
	TotalFindings      int `json:"total_findings"`
	SpecSections       int `json:"spec_sections"`
	ActiveSpecSections int `json:"active_spec_sections"`
	TermMapEntries     int `json:"term_map_entries"`
}

type SpecSection struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Title         string   `json:"title,omitempty"`
	StatementType string   `json:"statement_type"`
	ClaimLayer    string   `json:"claim_layer"`
	Status        string   `json:"status"`
	ValidUntil    string   `json:"valid_until,omitempty"`
	DependsOn     []string `json:"depends_on,omitempty"`
	TargetRefs    []string `json:"target_refs,omitempty"`
	DocumentKind  string   `json:"document_kind"`
	Path          string   `json:"path"`
	Line          int      `json:"line,omitempty"`
}

type SpecDocumentInput struct {
	Path    string
	Kind    string
	Content string
}

type specCheckCarrier struct {
	relativePath string
	kind         string
}

type fencedBlock struct {
	info      string
	body      string
	startLine int
}

type parsedSpecSection struct {
	id            string
	kind          string
	title         string
	statementType string
	claimLayer    string
	status        string
	validUntil    string
	dependsOn     []string
	targetRefs    []string
	line          int
}

type termMapEntry struct {
	term    string
	aliases []termMapAlias
	line    int
}

type termMapAlias struct {
	value     string
	term      string
	line      int
	fieldPath string
}

var specCheckCarriers = []specCheckCarrier{
	{relativePath: filepath.Join(".haft", "specs", "target-system.md"), kind: "target-system"},
	{relativePath: filepath.Join(".haft", "specs", "enabling-system.md"), kind: "enabling-system"},
	{relativePath: filepath.Join(".haft", "specs", "term-map.md"), kind: "term-map"},
}

var requiredSpecSectionFields = []string{
	"id",
	"kind",
	"statement_type",
	"claim_layer",
	"owner",
	"status",
}

var specSectionValueSets = map[string]map[string]struct{}{
	"statement_type": {
		"definition":    {},
		"admissibility": {},
		"duty":          {},
		"evidence":      {},
		"explanation":   {},
	},
	"claim_layer": {
		"object":      {},
		"description": {},
		"carrier":     {},
		"work":        {},
		"evidence":    {},
	},
	"owner": {
		"human":            {},
		"haft":             {},
		"agent":            {},
		"ci":               {},
		"external-carrier": {},
	},
	"status": {
		"draft":      {},
		"active":     {},
		"deprecated": {},
		"superseded": {},
	},
}

func CheckSpecificationSet(projectRoot string) (SpecCheckReport, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return newSpecCheckReport(), fmt.Errorf("project root is required")
	}

	report := newSpecCheckReport()
	documents, findings, err := loadSpecDocumentInputs(root)
	if err != nil {
		return report, err
	}

	report.Findings = append(report.Findings, findings...)

	checked := CheckSpecDocuments(documents)
	report.Documents = checked.Documents
	report.Findings = append(report.Findings, checked.Findings...)
	report.Summary = summarizeSpecCheck(report)

	return normalizeSpecCheckReport(report), nil
}

func LoadSpecSections(projectRoot string) ([]SpecSection, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return nil, fmt.Errorf("project root is required")
	}

	documents, _, err := loadSpecDocumentInputs(root)
	if err != nil {
		return nil, err
	}

	return SpecSectionsFromDocuments(documents), nil
}

func CheckSpecDocuments(documents []SpecDocumentInput) SpecCheckReport {
	report := newSpecCheckReport()
	seenSectionIDs := make(map[string]parsedSpecSection)
	seenTerms := make(map[string]termMapEntry)
	seenAliases := make(map[string]termMapAlias)

	for _, document := range documents {
		checked, sections, terms, findings := checkSpecDocument(document)

		report.Documents = append(report.Documents, checked)
		report.Findings = append(report.Findings, findings...)
		report.Findings = append(report.Findings, duplicateSectionFindings(document.Path, sections, seenSectionIDs)...)
		report.Findings = append(report.Findings, duplicateTermFindings(document.Path, terms, seenTerms)...)
		report.Findings = append(report.Findings, duplicateAliasFindings(document.Path, terms, seenAliases)...)
	}

	report.Summary = summarizeSpecCheck(report)

	return normalizeSpecCheckReport(report)
}

func SpecSectionsFromDocuments(documents []SpecDocumentInput) []SpecSection {
	sections := make([]SpecSection, 0)

	for _, document := range documents {
		if document.Kind == "term-map" {
			continue
		}

		_, parsedSections, _, _ := checkSpecDocument(document)
		for _, section := range parsedSections {
			sections = append(sections, specSectionFromParsed(document, section))
		}
	}

	sort.SliceStable(sections, func(i, j int) bool {
		left := sections[i]
		right := sections[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})

	return sections
}

func (report SpecCheckReport) HasFindings() bool {
	return report.Summary.TotalFindings > 0
}

func newSpecCheckReport() SpecCheckReport {
	return SpecCheckReport{
		Level:     "L0/L1/L1.5",
		Documents: []SpecCheckDocument{},
		Findings:  []SpecCheckFinding{},
		Summary:   SpecCheckSummary{},
	}
}

func loadSpecDocumentInputs(root string) ([]SpecDocumentInput, []SpecCheckFinding, error) {
	documents := make([]SpecDocumentInput, 0, len(specCheckCarriers))
	findings := make([]SpecCheckFinding, 0)

	for _, carrier := range specCheckCarriers {
		path := filepath.Join(root, carrier.relativePath)
		content, err := os.ReadFile(path)
		switch {
		case err == nil:
			documents = append(documents, SpecDocumentInput{
				Path:    filepath.ToSlash(carrier.relativePath),
				Kind:    carrier.kind,
				Content: string(content),
			})
		case os.IsNotExist(err):
			findings = append(findings, SpecCheckFinding{
				Level:   "L0",
				Code:    missingCarrierCode(carrier.kind),
				Path:    filepath.ToSlash(carrier.relativePath),
				Message: missingCarrierMessage(carrier.kind),
			})
		default:
			return documents, findings, fmt.Errorf("read spec carrier %s: %w", path, err)
		}
	}

	return documents, findings, nil
}

func checkSpecDocument(document SpecDocumentInput) (SpecCheckDocument, []parsedSpecSection, []termMapEntry, []SpecCheckFinding) {
	blocks, markdownFindings := parseFencedBlocks(document.Path, document.Content)
	checked := SpecCheckDocument{
		Path: filepath.ToSlash(document.Path),
		Kind: document.Kind,
	}

	switch document.Kind {
	case "term-map":
		terms, findings := checkTermMapBlocks(document.Path, blocks)
		checked.TermMapEntries = len(terms)

		return checked, nil, terms, append(markdownFindings, findings...)
	default:
		sections, findings := checkSpecSectionBlocks(document.Path, document.Kind, blocks)
		checked.SpecSections = len(sections)
		checked.ActiveSpecSections = countActiveSpecSections(sections)

		return checked, sections, nil, append(markdownFindings, findings...)
	}
}

func checkSpecSectionBlocks(path string, documentKind string, blocks []fencedBlock) ([]parsedSpecSection, []SpecCheckFinding) {
	sectionBlocks := filterFencedBlocks(blocks, isSpecSectionFence)
	findings := make([]SpecCheckFinding, 0)
	sections := make([]parsedSpecSection, 0, len(sectionBlocks))

	if len(sectionBlocks) == 0 {
		findings = append(findings, SpecCheckFinding{
			Level:   "L0",
			Code:    "spec_section_missing_block",
			Path:    filepath.ToSlash(path),
			Message: "spec carrier has no fenced `yaml spec-section` block",
		})

		return sections, findings
	}

	for _, block := range sectionBlocks {
		fields, err := parseYAMLMapping(block.body)
		if err != nil {
			findings = append(findings, SpecCheckFinding{
				Level:   "L0",
				Code:    "spec_section_invalid_yaml",
				Path:    filepath.ToSlash(path),
				Line:    block.startLine,
				Message: fmt.Sprintf("parse spec-section YAML: %v", err),
			})
			continue
		}

		section, sectionFindings := validateSpecSectionFields(path, block.startLine, fields)
		shapeFindings := validateSpecSectionShape(path, block.startLine, documentKind, fields, section)
		findings = append(findings, sectionFindings...)
		findings = append(findings, shapeFindings...)
		if section.id != "" {
			sections = append(sections, section)
		}
	}

	return sections, findings
}

func checkTermMapBlocks(path string, blocks []fencedBlock) ([]termMapEntry, []SpecCheckFinding) {
	termBlocks := filterFencedBlocks(blocks, isTermMapFence)
	findings := make([]SpecCheckFinding, 0)
	entries := make([]termMapEntry, 0)

	if len(termBlocks) == 0 {
		findings = append(findings, SpecCheckFinding{
			Level:   "L0",
			Code:    "term_map_missing_block",
			Path:    filepath.ToSlash(path),
			Message: "term-map carrier has no fenced YAML block",
		})

		return entries, findings
	}

	for _, block := range termBlocks {
		fields, err := parseYAMLMapping(block.body)
		if err != nil {
			findings = append(findings, SpecCheckFinding{
				Level:   "L0",
				Code:    "term_map_invalid_yaml",
				Path:    filepath.ToSlash(path),
				Line:    block.startLine,
				Message: fmt.Sprintf("parse term-map YAML: %v", err),
			})
			continue
		}

		blockEntries, entryFindings := extractTermMapEntries(path, block.startLine, fields)
		entries = append(entries, blockEntries...)
		findings = append(findings, entryFindings...)
	}

	if len(entries) == 0 && !containsFindingCode(findings, "term_map_missing_term") {
		findings = append(findings, SpecCheckFinding{
			Level:   "L1",
			Code:    "term_map_missing_term",
			Path:    filepath.ToSlash(path),
			Message: "term-map carrier has no entry with a `term` field",
		})
	}

	return entries, findings
}

func specSectionFromParsed(document SpecDocumentInput, section parsedSpecSection) SpecSection {
	return SpecSection{
		ID:            section.id,
		Kind:          section.kind,
		Title:         section.title,
		StatementType: section.statementType,
		ClaimLayer:    section.claimLayer,
		Status:        section.status,
		ValidUntil:    section.validUntil,
		DependsOn:     section.dependsOn,
		TargetRefs:    section.targetRefs,
		DocumentKind:  document.Kind,
		Path:          filepath.ToSlash(document.Path),
		Line:          section.line,
	}
}

func validateSpecSectionFields(path string, line int, fields map[string]any) (parsedSpecSection, []SpecCheckFinding) {
	findings := make([]SpecCheckFinding, 0)
	section := parsedSpecSection{
		line: line,
	}

	for _, field := range requiredSpecSectionFields {
		value, ok := scalarString(fields[field])
		if !ok {
			findings = append(findings, missingSpecSectionFieldFinding(path, line, section.id, field))
			continue
		}

		switch field {
		case "id":
			section.id = value
			if !isStableSpecSectionID(value) {
				findings = append(findings, SpecCheckFinding{
					Level:     "L0",
					Code:      "spec_section_unstable_id",
					Path:      filepath.ToSlash(path),
					FieldPath: "$.id",
					Line:      line,
					SectionID: value,
					Message:   "spec-section id must be a non-empty stable token without whitespace",
				})
			}
		case "kind":
			section.kind = value
		case "statement_type":
			section.statementType = value
		case "claim_layer":
			section.claimLayer = value
		case "status":
			section.status = value
		}
	}

	if title, ok := scalarString(fields["title"]); ok {
		section.title = title
	}
	if validUntil, ok := specSectionValidUntilString(fields["valid_until"]); ok {
		section.validUntil = validUntil
	}
	section.dependsOn = specSectionRefList(fields, "depends_on")
	section.targetRefs = specSectionRefList(fields, "target_refs")

	for field, allowed := range specSectionValueSets {
		value, ok := scalarString(fields[field])
		if !ok {
			continue
		}
		if _, valid := allowed[value]; !valid {
			findings = append(findings, SpecCheckFinding{
				Level:     "L1",
				Code:      "spec_section_invalid_field",
				Path:      filepath.ToSlash(path),
				FieldPath: "$." + field,
				Line:      line,
				SectionID: section.id,
				Message:   fmt.Sprintf("spec-section field %q has unsupported value %q", field, value),
			})
		}
	}

	return section, findings
}

func specSectionRefList(fields map[string]any, field string) []string {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	refs := make([]string, 0, len(items))
	for _, item := range items {
		ref, ok := strictString(item)
		if !ok {
			continue
		}
		if !isStableSpecSectionID(ref) {
			continue
		}

		refs = append(refs, ref)
	}

	return sortedUniqueStrings(refs)
}

func validateSpecSectionShape(path string, line int, documentKind string, fields map[string]any, section parsedSpecSection) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	findings = append(findings, validateOptionalRefList(path, line, section.id, fields, "depends_on", "spec_section_invalid_depends_on")...)
	findings = append(findings, validateOptionalRefList(path, line, section.id, fields, "target_refs", "spec_section_invalid_target_refs")...)
	findings = append(findings, validateOptionalEvidenceRequired(path, line, section.id, fields)...)
	findings = append(findings, validateOptionalValidUntil(path, line, section.id, fields, section)...)
	findings = append(findings, validateCarrierClaimAllowance(path, line, documentKind, fields, section)...)

	return findings
}

func extractTermMapEntries(path string, line int, fields map[string]any) ([]termMapEntry, []SpecCheckFinding) {
	if rawEntries, ok := fields["entries"]; ok {
		return extractTermMapEntryList(path, line, rawEntries)
	}

	entry, findings := validateTermMapEntry(path, line, "$", fields)
	if entry.term == "" {
		return nil, findings
	}

	return []termMapEntry{entry}, findings
}

func extractTermMapEntryList(path string, line int, rawEntries any) ([]termMapEntry, []SpecCheckFinding) {
	entryItems, ok := rawEntries.([]any)
	if !ok {
		return nil, []SpecCheckFinding{{
			Level:   "L0",
			Code:    "term_map_invalid_entries",
			Path:    filepath.ToSlash(path),
			Line:    line,
			Message: "term-map `entries` must be a YAML list",
		}}
	}

	entries := make([]termMapEntry, 0, len(entryItems))
	findings := make([]SpecCheckFinding, 0)

	for index, item := range entryItems {
		entryFields, ok := item.(map[string]any)
		if !ok {
			findings = append(findings, SpecCheckFinding{
				Level:     "L0",
				Code:      "term_map_invalid_entries",
				Path:      filepath.ToSlash(path),
				FieldPath: indexedFieldPath("$.entries", index),
				Line:      line,
				Message:   "term-map entry must be a YAML mapping",
			})
			continue
		}

		entryPath := indexedFieldPath("$.entries", index)
		entry, entryFindings := validateTermMapEntry(path, line, entryPath, entryFields)
		findings = append(findings, entryFindings...)
		if entry.term == "" {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, findings
}

func validateTermMapEntry(path string, line int, entryPath string, fields map[string]any) (termMapEntry, []SpecCheckFinding) {
	findings := make([]SpecCheckFinding, 0)
	entry := termMapEntry{
		line: line,
	}

	term, termFindings := requiredTermMapStringField(path, line, entryPath, fields, "term", "term_map_missing_term")
	_, domainFindings := requiredTermMapStringField(path, line, entryPath, fields, "domain", "term_map_missing_domain")
	_, definitionFindings := requiredTermMapStringField(path, line, entryPath, fields, "definition", "term_map_missing_definition")
	notFindings := validateOptionalStringList(path, line, entryPath, fields, "not", "term_map_invalid_not")
	aliases, aliasFindings := extractTermMapAliases(path, line, entryPath, fields, term)

	findings = append(findings, termFindings...)
	findings = append(findings, domainFindings...)
	findings = append(findings, definitionFindings...)
	findings = append(findings, notFindings...)
	findings = append(findings, aliasFindings...)

	entry.term = term
	entry.aliases = aliases

	return entry, findings
}

func duplicateSectionFindings(path string, sections []parsedSpecSection, seen map[string]parsedSpecSection) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, section := range sections {
		previous, ok := seen[section.id]
		if !ok {
			seen[section.id] = section
			continue
		}

		findings = append(findings, SpecCheckFinding{
			Level:     "L1",
			Code:      "spec_section_duplicate_id",
			Path:      filepath.ToSlash(path),
			Line:      section.line,
			SectionID: section.id,
			Message:   fmt.Sprintf("spec-section id duplicates earlier section at line %d", previous.line),
		})
	}

	return findings
}

func duplicateTermFindings(path string, entries []termMapEntry, seen map[string]termMapEntry) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, entry := range entries {
		normalized := strings.ToLower(entry.term)
		previous, ok := seen[normalized]
		if !ok {
			seen[normalized] = entry
			continue
		}

		findings = append(findings, SpecCheckFinding{
			Level:   "L1",
			Code:    "term_map_duplicate_term",
			Path:    filepath.ToSlash(path),
			Line:    entry.line,
			Message: fmt.Sprintf("term %q duplicates earlier term at line %d", entry.term, previous.line),
		})
	}

	return findings
}

func duplicateAliasFindings(path string, entries []termMapEntry, seen map[string]termMapAlias) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, entry := range entries {
		for _, alias := range entry.aliases {
			normalized := strings.ToLower(alias.value)
			previous, ok := seen[normalized]
			if !ok {
				seen[normalized] = alias
				continue
			}

			findings = append(findings, SpecCheckFinding{
				Level:     "L1.5",
				Code:      "term_map_duplicate_alias",
				Path:      filepath.ToSlash(path),
				FieldPath: alias.fieldPath,
				Line:      alias.line,
				Message:   fmt.Sprintf("alias %q for term %q duplicates earlier alias for term %q at line %d", alias.value, alias.term, previous.term, previous.line),
			})
		}
	}

	return findings
}

func validateOptionalRefList(path string, line int, sectionID string, fields map[string]any, field string, code string) []SpecCheckFinding {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	fieldPath := "$." + field
	items, ok := raw.([]any)
	if !ok {
		return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, code, fmt.Sprintf("spec-section `%s` must be a YAML list of stable string refs", field))}
	}

	findings := make([]SpecCheckFinding, 0)
	for index, item := range items {
		value, ok := strictString(item)
		if ok && isStableSpecSectionID(value) {
			continue
		}

		findings = append(findings, invalidSpecSectionShapeFinding(
			path,
			line,
			sectionID,
			indexedFieldPath(fieldPath, index),
			code,
			fmt.Sprintf("spec-section `%s` entries must be stable non-empty string refs", field),
		))
	}

	return findings
}

func validateOptionalEvidenceRequired(path string, line int, sectionID string, fields map[string]any) []SpecCheckFinding {
	raw, ok := fields["evidence_required"]
	if !ok {
		return nil
	}

	fieldPath := "$.evidence_required"
	items, ok := raw.([]any)
	if !ok {
		return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, "spec_section_invalid_evidence_required", "spec-section `evidence_required` must be a YAML list")}
	}

	findings := make([]SpecCheckFinding, 0)
	for index, item := range items {
		itemPath := indexedFieldPath(fieldPath, index)
		if _, ok := strictString(item); ok {
			continue
		}

		itemFields, ok := item.(map[string]any)
		if !ok {
			findings = append(findings, invalidSpecSectionShapeFinding(path, line, sectionID, itemPath, "spec_section_invalid_evidence_required", "evidence_required entries must be non-empty strings or mappings with kind and description"))
			continue
		}

		if _, ok := strictString(itemFields["kind"]); !ok {
			findings = append(findings, invalidSpecSectionShapeFinding(path, line, sectionID, joinFieldPath(itemPath, "kind"), "spec_section_invalid_evidence_required", "evidence_required mapping entries must include non-empty string `kind`"))
		}
		if _, ok := strictString(itemFields["description"]); !ok {
			findings = append(findings, invalidSpecSectionShapeFinding(path, line, sectionID, joinFieldPath(itemPath, "description"), "spec_section_invalid_evidence_required", "evidence_required mapping entries must include non-empty string `description`"))
		}
	}

	return findings
}

func validateOptionalValidUntil(path string, line int, sectionID string, fields map[string]any, section parsedSpecSection) []SpecCheckFinding {
	raw, ok := fields["valid_until"]
	if !ok {
		return nil
	}

	fieldPath := "$.valid_until"
	if raw == nil && section.status == "draft" && section.claimLayer == "carrier" {
		return nil
	}
	if raw == nil {
		return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, "spec_section_invalid_valid_until", "spec-section `valid_until` must be YYYY-MM-DD or RFC3339; null is only allowed on draft carrier placeholders")}
	}

	if _, ok := raw.(time.Time); ok {
		return nil
	}

	value, ok := strictString(raw)
	if !ok {
		return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, "spec_section_invalid_valid_until", "spec-section `valid_until` must be a date string")}
	}
	if isValidSpecDate(value) {
		return nil
	}

	return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, "spec_section_invalid_valid_until", "spec-section `valid_until` must be YYYY-MM-DD or RFC3339")}
}

func specSectionValidUntilString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case time.Time:
		return typed.Format("2006-01-02"), true
	default:
		return scalarString(typed)
	}
}

func validateCarrierClaimAllowance(path string, line int, documentKind string, fields map[string]any, section parsedSpecSection) []SpecCheckFinding {
	allowed := false
	raw, exists := fields["carrier_claim_allowed"]
	if exists {
		value, ok := raw.(bool)
		if !ok {
			return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, section.id, "$.carrier_claim_allowed", "spec_section_invalid_carrier_claim_allowed", "spec-section `carrier_claim_allowed` must be a boolean")}
		}

		allowed = value
	}

	if documentKind != "target-system" {
		return nil
	}
	if section.status != "active" {
		return nil
	}
	if section.claimLayer != "carrier" {
		return nil
	}
	if allowed {
		return nil
	}

	return []SpecCheckFinding{{
		Level:     "L1.5",
		Code:      "spec_section_mixed_authority",
		Path:      filepath.ToSlash(path),
		FieldPath: "$.claim_layer",
		Line:      line,
		SectionID: section.id,
		Message:   "active target-system sections must not use `claim_layer: carrier` unless `carrier_claim_allowed: true` is explicit",
	}}
}

func requiredTermMapStringField(path string, line int, entryPath string, fields map[string]any, field string, missingCode string) (string, []SpecCheckFinding) {
	raw, exists := fields[field]
	fieldPath := joinFieldPath(entryPath, field)
	if !exists || raw == nil {
		return "", []SpecCheckFinding{missingTermMapFieldFinding(path, line, fieldPath, field, missingCode)}
	}

	value, ok := strictString(raw)
	if ok {
		return value, nil
	}

	if _, ok := raw.(string); ok {
		return "", []SpecCheckFinding{missingTermMapFieldFinding(path, line, fieldPath, field, missingCode)}
	}

	return "", []SpecCheckFinding{{
		Level:     "L1.5",
		Code:      "term_map_invalid_field",
		Path:      filepath.ToSlash(path),
		FieldPath: fieldPath,
		Line:      line,
		Message:   fmt.Sprintf("term-map field `%s` must be a non-empty string", field),
	}}
}

func validateOptionalStringList(path string, line int, entryPath string, fields map[string]any, field string, code string) []SpecCheckFinding {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	fieldPath := joinFieldPath(entryPath, field)
	items, ok := raw.([]any)
	if !ok {
		return []SpecCheckFinding{invalidTermMapShapeFinding(path, line, fieldPath, code, fmt.Sprintf("term-map `%s` must be a YAML list of non-empty strings", field))}
	}

	findings := make([]SpecCheckFinding, 0)
	for index, item := range items {
		if _, ok := strictString(item); ok {
			continue
		}

		findings = append(findings, invalidTermMapShapeFinding(path, line, indexedFieldPath(fieldPath, index), code, fmt.Sprintf("term-map `%s` entries must be non-empty strings", field)))
	}

	return findings
}

func extractTermMapAliases(path string, line int, entryPath string, fields map[string]any, term string) ([]termMapAlias, []SpecCheckFinding) {
	raw, ok := fields["aliases"]
	if !ok {
		return nil, nil
	}

	fieldPath := joinFieldPath(entryPath, "aliases")
	items, ok := raw.([]any)
	if !ok {
		return nil, []SpecCheckFinding{invalidTermMapShapeFinding(path, line, fieldPath, "term_map_invalid_aliases", "term-map `aliases` must be a YAML list of non-empty strings")}
	}

	aliases := make([]termMapAlias, 0, len(items))
	findings := make([]SpecCheckFinding, 0)
	for index, item := range items {
		aliasPath := indexedFieldPath(fieldPath, index)
		value, ok := strictString(item)
		if !ok {
			findings = append(findings, invalidTermMapShapeFinding(path, line, aliasPath, "term_map_invalid_aliases", "term-map `aliases` entries must be non-empty strings"))
			continue
		}

		aliases = append(aliases, termMapAlias{
			value:     value,
			term:      term,
			line:      line,
			fieldPath: aliasPath,
		})
	}

	return aliases, findings
}

func invalidSpecSectionShapeFinding(path string, line int, sectionID string, fieldPath string, code string, message string) SpecCheckFinding {
	return SpecCheckFinding{
		Level:     "L1.5",
		Code:      code,
		Path:      filepath.ToSlash(path),
		FieldPath: fieldPath,
		Line:      line,
		SectionID: sectionID,
		Message:   message,
	}
}

func invalidTermMapShapeFinding(path string, line int, fieldPath string, code string, message string) SpecCheckFinding {
	return SpecCheckFinding{
		Level:     "L1.5",
		Code:      code,
		Path:      filepath.ToSlash(path),
		FieldPath: fieldPath,
		Line:      line,
		Message:   message,
	}
}

func missingTermMapFieldFinding(path string, line int, fieldPath string, field string, code string) SpecCheckFinding {
	return SpecCheckFinding{
		Level:     "L1.5",
		Code:      code,
		Path:      filepath.ToSlash(path),
		FieldPath: fieldPath,
		Line:      line,
		Message:   fmt.Sprintf("term-map entry missing required field `%s`", field),
	}
}

func strictString(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	return text, true
}

func joinFieldPath(base string, field string) string {
	if base == "$" {
		return "$." + field
	}

	return base + "." + field
}

func indexedFieldPath(base string, index int) string {
	return fmt.Sprintf("%s[%d]", base, index)
}

func isValidSpecDate(value string) bool {
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}

	return false
}

func parseYAMLMapping(content string) (map[string]any, error) {
	fields := map[string]any{}
	if err := yaml.Unmarshal([]byte(content), &fields); err != nil {
		return nil, err
	}

	return fields, nil
}

func parseFencedBlocks(path string, content string) ([]fencedBlock, []SpecCheckFinding) {
	lines := strings.Split(content, "\n")
	blocks := make([]fencedBlock, 0)
	findings := make([]SpecCheckFinding, 0)
	bodyLines := make([]string, 0)
	info := ""
	startLine := 0
	inFence := false

	for index, line := range lines {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(line)
		fenceInfo, isFence := fenceMarkerInfo(trimmed)

		if !inFence && isFence {
			inFence = true
			info = fenceInfo
			startLine = lineNumber
			bodyLines = bodyLines[:0]
			continue
		}

		if inFence && isFence {
			blocks = append(blocks, fencedBlock{
				info:      info,
				body:      strings.Join(bodyLines, "\n"),
				startLine: startLine,
			})
			inFence = false
			info = ""
			startLine = 0
			continue
		}

		if inFence {
			bodyLines = append(bodyLines, line)
		}
	}

	if inFence {
		findings = append(findings, SpecCheckFinding{
			Level:   "L0",
			Code:    "markdown_unclosed_fence",
			Path:    filepath.ToSlash(path),
			Line:    startLine,
			Message: "markdown fenced block is not closed",
		})
	}

	return blocks, findings
}

func filterFencedBlocks(blocks []fencedBlock, predicate func(string) bool) []fencedBlock {
	filtered := make([]fencedBlock, 0, len(blocks))

	for _, block := range blocks {
		if !predicate(block.info) {
			continue
		}

		filtered = append(filtered, block)
	}

	return filtered
}

func isSpecSectionFence(info string) bool {
	fields := strings.Fields(info)
	if len(fields) < 2 {
		return false
	}

	return fields[0] == "yaml" && fields[1] == "spec-section"
}

func isTermMapFence(info string) bool {
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return false
	}
	if fields[0] != "yaml" {
		return false
	}
	if len(fields) == 1 {
		return true
	}

	return fields[1] == "term-map"
}

func fenceMarkerInfo(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}

	info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))

	return info, true
}

func scalarString(value any) (string, bool) {
	if value == nil {
		return "", false
	}

	text := ""
	switch typed := value.(type) {
	case string:
		text = typed
	default:
		text = fmt.Sprint(typed)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	return text, true
}

func isStableSpecSectionID(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}

	return !strings.ContainsAny(value, " \t\r\n")
}

func countActiveSpecSections(sections []parsedSpecSection) int {
	count := 0

	for _, section := range sections {
		if section.status != "active" {
			continue
		}

		count++
	}

	return count
}

func containsFindingCode(findings []SpecCheckFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code != code {
			continue
		}

		return true
	}

	return false
}

func missingSpecSectionFieldFinding(path string, line int, sectionID string, field string) SpecCheckFinding {
	return SpecCheckFinding{
		Level:     "L0",
		Code:      "spec_section_missing_field",
		Path:      filepath.ToSlash(path),
		FieldPath: "$." + field,
		Line:      line,
		SectionID: sectionID,
		Message:   fmt.Sprintf("spec-section missing required field %q", field),
	}
}

func missingCarrierCode(kind string) string {
	switch kind {
	case "term-map":
		return "term_map_missing_file"
	default:
		return "spec_carrier_missing_file"
	}
}

func missingCarrierMessage(kind string) string {
	switch kind {
	case "term-map":
		return "term-map carrier is missing"
	default:
		return fmt.Sprintf("%s spec carrier is missing", kind)
	}
}

func summarizeSpecCheck(report SpecCheckReport) SpecCheckSummary {
	summary := SpecCheckSummary{
		TotalFindings: len(report.Findings),
	}

	for _, document := range report.Documents {
		summary.SpecSections += document.SpecSections
		summary.ActiveSpecSections += document.ActiveSpecSections
		summary.TermMapEntries += document.TermMapEntries
	}

	return summary
}

func normalizeSpecCheckReport(report SpecCheckReport) SpecCheckReport {
	if report.Documents == nil {
		report.Documents = []SpecCheckDocument{}
	}
	if report.Findings == nil {
		report.Findings = []SpecCheckFinding{}
	}

	sort.SliceStable(report.Findings, func(i, j int) bool {
		left := report.Findings[i]
		right := report.Findings[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}

		return left.Message < right.Message
	})

	report.Summary = summarizeSpecCheck(report)

	return report
}
