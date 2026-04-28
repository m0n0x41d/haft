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
	Level      string `json:"level"`
	Code       string `json:"code"`
	Path       string `json:"path"`
	FieldPath  string `json:"field_path,omitempty"`
	Line       int    `json:"line,omitempty"`
	SectionID  string `json:"section_id,omitempty"`
	Message    string `json:"message"`
	NextAction string `json:"next_action,omitempty"`
}

type SpecCheckSummary struct {
	TotalFindings      int `json:"total_findings"`
	SpecSections       int `json:"spec_sections"`
	ActiveSpecSections int `json:"active_spec_sections"`
	TermMapEntries     int `json:"term_map_entries"`
}

type ProjectSpecificationSet struct {
	Documents      []SpecDocument     `json:"documents"`
	Sections       []SpecSection      `json:"sections"`
	TermMapEntries []TermMapEntry     `json:"term_map_entries"`
	Findings       []SpecCheckFinding `json:"findings"`
}

type SpecDocumentKind string

const (
	SpecDocumentKindTargetSystem   SpecDocumentKind = "target-system"
	SpecDocumentKindEnablingSystem SpecDocumentKind = "enabling-system"
	SpecDocumentKindTermMap        SpecDocumentKind = "term-map"
)

type SpecDocument struct {
	Path           string           `json:"path"`
	Kind           SpecDocumentKind `json:"kind"`
	Sections       []SpecSection    `json:"sections,omitempty"`
	TermMapEntries []TermMapEntry   `json:"term_map_entries,omitempty"`
}

type SpecSectionState string

const (
	SpecSectionStateDraft      SpecSectionState = "draft"
	SpecSectionStateActive     SpecSectionState = "active"
	SpecSectionStateDeprecated SpecSectionState = "deprecated"
	SpecSectionStateSuperseded SpecSectionState = "superseded"
	SpecSectionStateStale      SpecSectionState = "stale"
	SpecSectionStateMalformed  SpecSectionState = "malformed"
)

type SpecSection struct {
	ID               string                    `json:"id"`
	Spec             string                    `json:"spec"`
	Kind             string                    `json:"kind"`
	Title            string                    `json:"title,omitempty"`
	StatementType    string                    `json:"statement_type"`
	ClaimLayer       string                    `json:"claim_layer"`
	Owner            string                    `json:"owner"`
	Status           string                    `json:"status"`
	ValidUntil       string                    `json:"valid_until,omitempty"`
	Terms            []string                  `json:"terms,omitempty"`
	DependsOn        []string                  `json:"depends_on,omitempty"`
	TargetRefs       []string                  `json:"target_refs,omitempty"`
	EvidenceRequired []SpecEvidenceRequirement `json:"evidence_required,omitempty"`
	DocumentKind     string                    `json:"document_kind"`
	Path             string                    `json:"path"`
	Line             int                       `json:"line,omitempty"`
	Malformed        bool                      `json:"malformed,omitempty"`
}

type SpecEvidenceRequirement struct {
	Kind        string `json:"kind,omitempty"`
	Description string `json:"description,omitempty"`
}

type TermMapEntry struct {
	Term         string   `json:"term"`
	Domain       string   `json:"domain"`
	Definition   string   `json:"definition"`
	Not          []string `json:"not,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
	Owners       []string `json:"owners,omitempty"`
	Path         string   `json:"path"`
	Line         int      `json:"line,omitempty"`
	aliasEntries []termMapAlias
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

// LoadProjectSpecificationSet parses the project's spec carriers and
// returns the canonical ProjectSpecificationSet. Surfaces (CLI, MCP,
// Desktop) use this to feed `internal/project/specflow.DeriveState`
// without re-implementing the load + parse pipeline.
func LoadProjectSpecificationSet(projectRoot string) (ProjectSpecificationSet, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return ProjectSpecificationSet{}, fmt.Errorf("project root is required")
	}

	documents, loadFindings, err := loadSpecDocumentInputs(root)
	if err != nil {
		return ProjectSpecificationSet{}, err
	}

	specSet := ProjectSpecificationSetFromDocuments(documents)
	specSet.Findings = append(loadFindings, specSet.Findings...)

	return normalizeProjectSpecificationSet(specSet), nil
}

func CheckSpecDocuments(documents []SpecDocumentInput) SpecCheckReport {
	specSet := ProjectSpecificationSetFromDocuments(documents)
	return SpecCheckReportFromSpecificationSet(specSet)
}

func ProjectSpecificationSetFromDocuments(documents []SpecDocumentInput) ProjectSpecificationSet {
	specSet := newProjectSpecificationSet()
	seenSectionIDs := make(map[string]SpecSection)
	seenTerms := make(map[string]TermMapEntry)
	seenAliases := make(map[string]termMapAlias)

	for _, documentInput := range documents {
		document, findings := checkSpecDocument(documentInput)

		specSet.Documents = append(specSet.Documents, document)
		specSet.Sections = append(specSet.Sections, document.Sections...)
		specSet.TermMapEntries = append(specSet.TermMapEntries, document.TermMapEntries...)
		specSet.Findings = append(specSet.Findings, findings...)
		specSet.Findings = append(specSet.Findings, duplicateSectionFindings(document.Path, document.Sections, seenSectionIDs)...)
		specSet.Findings = append(specSet.Findings, duplicateTermFindings(document.Path, document.TermMapEntries, seenTerms)...)
		specSet.Findings = append(specSet.Findings, duplicateAliasFindings(document.Path, document.TermMapEntries, seenAliases)...)
	}
	specSet.Findings = append(specSet.Findings, projectSpecificationReadinessFindings(specSet)...)

	return normalizeProjectSpecificationSet(specSet)
}

func SpecCheckReportFromSpecificationSet(specSet ProjectSpecificationSet) SpecCheckReport {
	report := newSpecCheckReport()

	for _, document := range specSet.Documents {
		report.Documents = append(report.Documents, specCheckDocumentFromSpecDocument(document))
	}
	report.Findings = append(report.Findings, specSet.Findings...)

	report.Summary = summarizeSpecCheck(report)

	return normalizeSpecCheckReport(report)
}

func SpecSectionsFromDocuments(documents []SpecDocumentInput) []SpecSection {
	specSet := ProjectSpecificationSetFromDocuments(documents)
	sections := append([]SpecSection(nil), specSet.Sections...)

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

func newProjectSpecificationSet() ProjectSpecificationSet {
	return ProjectSpecificationSet{
		Documents:      []SpecDocument{},
		Sections:       []SpecSection{},
		TermMapEntries: []TermMapEntry{},
		Findings:       []SpecCheckFinding{},
	}
}

func specCheckDocumentFromSpecDocument(document SpecDocument) SpecCheckDocument {
	return SpecCheckDocument{
		Path:               filepath.ToSlash(document.Path),
		Kind:               string(document.Kind),
		SpecSections:       len(document.Sections),
		ActiveSpecSections: countActiveSpecSections(document.Sections),
		TermMapEntries:     len(document.TermMapEntries),
	}
}

func loadSpecDocumentInputs(root string) ([]SpecDocumentInput, []SpecCheckFinding, error) {
	documents := make([]SpecDocumentInput, 0, len(specCheckCarriers))
	findings := ignoredSpecCarrierFindings(root)

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

func ignoredSpecCarrierFindings(root string) []SpecCheckFinding {
	gitignorePath := filepath.Join(root, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	findings := make([]SpecCheckFinding, 0)
	for index, line := range lines {
		pattern := strings.TrimSpace(line)
		if !rootGitignoreIgnoresHaft(pattern) {
			continue
		}

		findings = append(findings, SpecCheckFinding{
			Level:      "L1",
			Code:       "spec_carriers_gitignored",
			Path:       ".gitignore",
			Line:       index + 1,
			Message:    ".gitignore ignores .haft/, so project specification carrier edits are local-only and cannot be reviewed from the repository patch",
			NextAction: "unignore .haft/specs and other reviewable projections, or keep the carriers local and record the dogfood state in tracked specs/tests",
		})
	}

	return findings
}

func rootGitignoreIgnoresHaft(pattern string) bool {
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "#") {
		return false
	}
	if strings.HasPrefix(pattern, "!") {
		return false
	}

	normalized := strings.TrimPrefix(pattern, "/")
	normalized = strings.TrimSuffix(normalized, "/")

	switch normalized {
	case ".haft", ".haft/**", ".haft/*":
		return true
	default:
		return false
	}
}

func checkSpecDocument(input SpecDocumentInput) (SpecDocument, []SpecCheckFinding) {
	blocks, markdownFindings := parseFencedBlocks(input.Path, input.Content)
	document := SpecDocument{
		Path: filepath.ToSlash(input.Path),
		Kind: SpecDocumentKind(strings.TrimSpace(input.Kind)),
	}

	switch input.Kind {
	case "term-map":
		terms, findings := checkTermMapBlocks(input.Path, blocks)
		document.TermMapEntries = terms

		return document, append(markdownFindings, findings...)
	default:
		sections, findings := checkSpecSectionBlocks(input.Path, input.Kind, blocks)
		document.Sections = sections

		return document, append(markdownFindings, findings...)
	}
}

func checkSpecSectionBlocks(path string, documentKind string, blocks []fencedBlock) ([]SpecSection, []SpecCheckFinding) {
	sectionBlocks := filterFencedBlocks(blocks, isSpecSectionFence)
	findings := make([]SpecCheckFinding, 0)
	sections := make([]SpecSection, 0, len(sectionBlocks))

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

		section, sectionFindings := validateSpecSectionFields(path, block.startLine, documentKind, fields)
		shapeFindings := validateSpecSectionShape(path, block.startLine, documentKind, fields, section)
		findings = append(findings, sectionFindings...)
		findings = append(findings, shapeFindings...)
		section.Malformed = len(sectionFindings)+len(shapeFindings) > 0
		if section.ID != "" {
			sections = append(sections, section)
		}
	}

	return sections, findings
}

func checkTermMapBlocks(path string, blocks []fencedBlock) ([]TermMapEntry, []SpecCheckFinding) {
	termBlocks := filterFencedBlocks(blocks, isTermMapFence)
	findings := make([]SpecCheckFinding, 0)
	entries := make([]TermMapEntry, 0)

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

func projectSpecificationReadinessFindings(specSet ProjectSpecificationSet) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	target, targetOK := specDocumentByKind(specSet.Documents, SpecDocumentKindTargetSystem)
	targetActive := targetOK && countActiveSpecSections(target.Sections) > 0
	if targetOK && !targetActive {
		findings = append(findings, noActiveSpecSectionFinding(
			target.Path,
			"target-system spec has no active sections; draft placeholders do not make the product specified",
			"run target-system onboarding and add human-approved active sections for environment change, target role, boundaries, interfaces, invariants, risks, and acceptance evidence",
		))
	}

	enabling, enablingOK := specDocumentByKind(specSet.Documents, SpecDocumentKindEnablingSystem)
	if enablingOK && !targetActive && countActiveSpecSections(enabling.Sections) == 0 {
		findings = append(findings, noActiveSpecSectionFinding(
			enabling.Path,
			"enabling-system spec has no active sections; draft placeholders do not authorize engineering governance",
			"after the target spec is admissible, add active enabling sections for creator roles, repo architecture, effect boundaries, test strategy, surfaces, and runtime policy",
		))
	}

	return findings
}

func specDocumentByKind(documents []SpecDocument, kind SpecDocumentKind) (SpecDocument, bool) {
	for _, document := range documents {
		if document.Kind != kind {
			continue
		}

		return document, true
	}

	return SpecDocument{}, false
}

func noActiveSpecSectionFinding(
	path string,
	message string,
	nextAction string,
) SpecCheckFinding {
	return SpecCheckFinding{
		Level:      "L1",
		Code:       "spec_carrier_no_active_sections",
		Path:       filepath.ToSlash(path),
		FieldPath:  "$.status",
		Message:    message,
		NextAction: nextAction,
	}
}

func validateSpecSectionFields(
	path string,
	line int,
	documentKind string,
	fields map[string]any,
) (SpecSection, []SpecCheckFinding) {
	findings := make([]SpecCheckFinding, 0)
	section := SpecSection{
		Spec:         strings.TrimSpace(documentKind),
		DocumentKind: strings.TrimSpace(documentKind),
		Path:         filepath.ToSlash(path),
		Line:         line,
	}

	for _, field := range requiredSpecSectionFields {
		value, ok := scalarString(fields[field])
		if !ok {
			findings = append(findings, missingSpecSectionFieldFinding(path, line, section.ID, field))
			continue
		}

		switch field {
		case "id":
			section.ID = value
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
			section.Kind = value
		case "statement_type":
			section.StatementType = value
		case "claim_layer":
			section.ClaimLayer = value
		case "owner":
			section.Owner = value
		case "status":
			section.Status = value
		}
	}

	if title, ok := scalarString(fields["title"]); ok {
		section.Title = title
	}
	if validUntil, ok := specSectionValidUntilString(fields["valid_until"]); ok {
		section.ValidUntil = validUntil
	}
	section.Terms = specSectionStringList(fields, "terms")
	section.DependsOn = specSectionRefList(fields, "depends_on")
	section.TargetRefs = specSectionRefList(fields, "target_refs")
	section.EvidenceRequired = specSectionEvidenceRequirements(fields)

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
				SectionID: section.ID,
				Message:   fmt.Sprintf("spec-section field %q has unsupported value %q", field, value),
			})
		}
	}

	return section, findings
}

func specSectionStringList(fields map[string]any, field string) []string {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := strictString(item)
		if !ok {
			continue
		}

		values = append(values, value)
	}

	return sortedUniqueStrings(values)
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

func specSectionEvidenceRequirements(fields map[string]any) []SpecEvidenceRequirement {
	raw, ok := fields["evidence_required"]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	requirements := make([]SpecEvidenceRequirement, 0, len(items))
	for _, item := range items {
		if description, ok := strictString(item); ok {
			requirements = append(requirements, SpecEvidenceRequirement{Description: description})
			continue
		}

		itemFields, ok := item.(map[string]any)
		if !ok {
			continue
		}

		kind, kindOK := strictString(itemFields["kind"])
		description, descriptionOK := strictString(itemFields["description"])
		if !kindOK || !descriptionOK {
			continue
		}

		requirements = append(requirements, SpecEvidenceRequirement{
			Kind:        kind,
			Description: description,
		})
	}

	return requirements
}

func validateSpecSectionShape(path string, line int, documentKind string, fields map[string]any, section SpecSection) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	findings = append(findings, validateOptionalSpecStringList(path, line, section.ID, fields, "terms", "spec_section_invalid_terms")...)
	findings = append(findings, validateOptionalRefList(path, line, section.ID, fields, "depends_on", "spec_section_invalid_depends_on")...)
	findings = append(findings, validateOptionalRefList(path, line, section.ID, fields, "target_refs", "spec_section_invalid_target_refs")...)
	findings = append(findings, validateOptionalEvidenceRequired(path, line, section.ID, fields)...)
	findings = append(findings, validateOptionalValidUntil(path, line, section.ID, fields, section)...)
	findings = append(findings, validateCarrierClaimAllowance(path, line, documentKind, fields, section)...)

	return findings
}

func extractTermMapEntries(path string, line int, fields map[string]any) ([]TermMapEntry, []SpecCheckFinding) {
	if rawEntries, ok := fields["entries"]; ok {
		return extractTermMapEntryList(path, line, rawEntries)
	}

	entry, findings := validateTermMapEntry(path, line, "$", fields)
	if entry.Term == "" {
		return nil, findings
	}

	return []TermMapEntry{entry}, findings
}

func extractTermMapEntryList(path string, line int, rawEntries any) ([]TermMapEntry, []SpecCheckFinding) {
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

	entries := make([]TermMapEntry, 0, len(entryItems))
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
		if entry.Term == "" {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, findings
}

func validateTermMapEntry(path string, line int, entryPath string, fields map[string]any) (TermMapEntry, []SpecCheckFinding) {
	findings := make([]SpecCheckFinding, 0)
	entry := TermMapEntry{
		Path: filepath.ToSlash(path),
		Line: line,
	}

	term, termFindings := requiredTermMapStringField(path, line, entryPath, fields, "term", "term_map_missing_term")
	domain, domainFindings := requiredTermMapStringField(path, line, entryPath, fields, "domain", "term_map_missing_domain")
	definition, definitionFindings := requiredTermMapStringField(path, line, entryPath, fields, "definition", "term_map_missing_definition")
	notFindings := validateOptionalStringList(path, line, entryPath, fields, "not", "term_map_invalid_not")
	aliases, aliasFindings := extractTermMapAliases(path, line, entryPath, fields, term)
	ownersFindings := validateOptionalStringList(path, line, entryPath, fields, "owners", "term_map_invalid_owners")

	findings = append(findings, termFindings...)
	findings = append(findings, domainFindings...)
	findings = append(findings, definitionFindings...)
	findings = append(findings, notFindings...)
	findings = append(findings, aliasFindings...)
	findings = append(findings, ownersFindings...)

	entry.Term = term
	entry.Domain = domain
	entry.Definition = definition
	entry.Not = termMapStringList(fields, "not")
	entry.Aliases = termMapAliasValues(aliases)
	entry.Owners = termMapStringList(fields, "owners")
	entry.aliasEntries = aliases

	return entry, findings
}

func duplicateSectionFindings(path string, sections []SpecSection, seen map[string]SpecSection) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, section := range sections {
		previous, ok := seen[section.ID]
		if !ok {
			seen[section.ID] = section
			continue
		}

		findings = append(findings, SpecCheckFinding{
			Level:     "L1",
			Code:      "spec_section_duplicate_id",
			Path:      filepath.ToSlash(path),
			Line:      section.Line,
			SectionID: section.ID,
			Message:   fmt.Sprintf("spec-section id duplicates earlier section at line %d", previous.Line),
		})
	}

	return findings
}

func duplicateTermFindings(path string, entries []TermMapEntry, seen map[string]TermMapEntry) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, entry := range entries {
		normalized := strings.ToLower(entry.Term)
		previous, ok := seen[normalized]
		if !ok {
			seen[normalized] = entry
			continue
		}

		findings = append(findings, SpecCheckFinding{
			Level:   "L1",
			Code:    "term_map_duplicate_term",
			Path:    filepath.ToSlash(path),
			Line:    entry.Line,
			Message: fmt.Sprintf("term %q duplicates earlier term at line %d", entry.Term, previous.Line),
		})
	}

	return findings
}

func duplicateAliasFindings(path string, entries []TermMapEntry, seen map[string]termMapAlias) []SpecCheckFinding {
	findings := make([]SpecCheckFinding, 0)

	for _, entry := range entries {
		for _, alias := range entry.aliasEntries {
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

func validateOptionalSpecStringList(path string, line int, sectionID string, fields map[string]any, field string, code string) []SpecCheckFinding {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	fieldPath := "$." + field
	items, ok := raw.([]any)
	if !ok {
		return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, sectionID, fieldPath, code, fmt.Sprintf("spec-section `%s` must be a YAML list of non-empty strings", field))}
	}

	findings := make([]SpecCheckFinding, 0)
	for index, item := range items {
		if _, ok := strictString(item); ok {
			continue
		}

		findings = append(findings, invalidSpecSectionShapeFinding(
			path,
			line,
			sectionID,
			indexedFieldPath(fieldPath, index),
			code,
			fmt.Sprintf("spec-section `%s` entries must be non-empty strings", field),
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

func validateOptionalValidUntil(path string, line int, sectionID string, fields map[string]any, section SpecSection) []SpecCheckFinding {
	raw, ok := fields["valid_until"]
	if !ok {
		return nil
	}

	fieldPath := "$.valid_until"
	if raw == nil && section.Status == "draft" && section.ClaimLayer == "carrier" {
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

func validateCarrierClaimAllowance(path string, line int, documentKind string, fields map[string]any, section SpecSection) []SpecCheckFinding {
	allowed := false
	raw, exists := fields["carrier_claim_allowed"]
	if exists {
		value, ok := raw.(bool)
		if !ok {
			return []SpecCheckFinding{invalidSpecSectionShapeFinding(path, line, section.ID, "$.carrier_claim_allowed", "spec_section_invalid_carrier_claim_allowed", "spec-section `carrier_claim_allowed` must be a boolean")}
		}

		allowed = value
	}

	if documentKind != "target-system" {
		return nil
	}
	if section.Status != "active" {
		return nil
	}
	if section.ClaimLayer != "carrier" {
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
		SectionID: section.ID,
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

func termMapStringList(fields map[string]any, field string) []string {
	raw, ok := fields[field]
	if !ok {
		return nil
	}

	items, ok := raw.([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := strictString(item)
		if !ok {
			continue
		}

		values = append(values, value)
	}

	return sortedUniqueStrings(values)
}

func termMapAliasValues(aliases []termMapAlias) []string {
	values := make([]string, 0, len(aliases))

	for _, alias := range aliases {
		values = append(values, alias.value)
	}

	return sortedUniqueStrings(values)
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

func countActiveSpecSections(sections []SpecSection) int {
	count := 0

	for _, section := range sections {
		if section.Status != "active" {
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
	for index := range report.Findings {
		report.Findings[index] = normalizeSpecCheckFinding(report.Findings[index])
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

func normalizeSpecCheckFinding(finding SpecCheckFinding) SpecCheckFinding {
	finding.Path = filepath.ToSlash(finding.Path)
	if finding.NextAction == "" {
		finding.NextAction = defaultSpecCheckNextAction(finding)
	}

	return finding
}

func defaultSpecCheckNextAction(finding SpecCheckFinding) string {
	switch finding.Code {
	case "spec_carriers_gitignored":
		return "unignore .haft/specs and other reviewable projections, or keep the carriers local and record the dogfood state in tracked specs/tests"
	case "spec_carrier_no_active_sections":
		return "keep placeholders draft and add human-approved active spec sections before treating readiness as passing"
	case "spec_carrier_missing_file":
		return "create the missing .haft/specs carrier or run `haft init` before relying on spec readiness"
	case "term_map_missing_file":
		return "create .haft/specs/term-map.md with term-map YAML entries for load-bearing spec terms"
	case "spec_section_missing_block":
		return "add a fenced `yaml spec-section` block with canonical fields to this spec carrier"
	case "term_map_missing_block":
		return "add a fenced YAML term-map block with entries for load-bearing spec terms"
	case "spec_section_invalid_yaml", "term_map_invalid_yaml", "markdown_unclosed_fence":
		return "fix the carrier syntax, then rerun `haft spec check --json`"
	case "spec_section_missing_field":
		return "add the missing canonical spec-section field, then rerun `haft spec check --json`"
	case "spec_section_unstable_id":
		return "replace the section id with a stable non-empty id that contains no whitespace"
	case "spec_section_unsupported_value":
		return "replace the field value with one of the canonical spec-section enum values"
	case "spec_section_invalid_terms":
		return "make `terms` a YAML list of non-empty strings"
	case "spec_section_invalid_depends_on", "spec_section_invalid_target_refs":
		return "make the reference field a YAML list of stable spec-section ids"
	case "spec_section_invalid_evidence_required":
		return "make each evidence requirement a non-empty string or an object with kind and description"
	case "spec_section_invalid_valid_until":
		return "use YYYY-MM-DD or RFC3339 valid_until; null is only allowed on draft carrier placeholders"
	case "spec_section_invalid_carrier_claim_allowed":
		return "set carrier_claim_allowed to true or false"
	case "spec_section_mixed_authority":
		return "change active target claims to claim_layer object/description/evidence, or explicitly mark a carrier-only section with carrier_claim_allowed"
	case "term_map_missing_term":
		return "add at least one term-map entry with term, domain, and definition before treating the spec set as ready"
	case "term_map_missing_domain", "term_map_missing_definition":
		return "complete the term-map entry with term, domain, and definition"
	case "term_map_invalid_entries":
		return "make `entries` a YAML list of term-map entry objects"
	case "term_map_invalid_not", "term_map_invalid_aliases", "term_map_invalid_owners":
		return "make the term-map field a YAML list of non-empty strings"
	case "term_map_duplicate_term":
		return "merge or domain-qualify duplicate term definitions"
	case "term_map_duplicate_alias":
		return "remove the duplicate alias or attach it to one canonical term"
	case "spec_section_duplicate_id":
		return "rename or merge duplicate spec sections so each id is unique"
	default:
		return "inspect the reported carrier location, repair the deterministic spec shape, and rerun `haft spec check --json`"
	}
}

func normalizeProjectSpecificationSet(specSet ProjectSpecificationSet) ProjectSpecificationSet {
	if specSet.Documents == nil {
		specSet.Documents = []SpecDocument{}
	}
	if specSet.Sections == nil {
		specSet.Sections = []SpecSection{}
	}
	if specSet.TermMapEntries == nil {
		specSet.TermMapEntries = []TermMapEntry{}
	}
	if specSet.Findings == nil {
		specSet.Findings = []SpecCheckFinding{}
	}
	for index := range specSet.Findings {
		specSet.Findings[index] = normalizeSpecCheckFinding(specSet.Findings[index])
	}

	for index := range specSet.Documents {
		if specSet.Documents[index].Sections == nil {
			specSet.Documents[index].Sections = []SpecSection{}
		}
		if specSet.Documents[index].TermMapEntries == nil {
			specSet.Documents[index].TermMapEntries = []TermMapEntry{}
		}
	}

	return specSet
}

func (section SpecSection) LifecycleState(now time.Time) SpecSectionState {
	if section.Malformed {
		return SpecSectionStateMalformed
	}

	switch strings.TrimSpace(section.Status) {
	case "draft":
		return SpecSectionStateDraft
	case "active":
		if validUntilExpired(section.ValidUntil, now) {
			return SpecSectionStateStale
		}

		return SpecSectionStateActive
	case "deprecated":
		return SpecSectionStateDeprecated
	case "superseded":
		return SpecSectionStateSuperseded
	default:
		return SpecSectionStateMalformed
	}
}
