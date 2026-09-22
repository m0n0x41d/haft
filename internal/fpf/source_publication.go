package fpf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// PublicationDescriptor is a source-access identity, not a runtime ontology or
// an applicability route. Paths are relative to one pinned upstream Git tree.
type PublicationDescriptor struct {
	ID        string `json:"publication_id"`
	Kind      string `json:"publication_kind"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	Root      string `json:"root"`
}

type PublicationManifestEntry struct {
	PublicationDescriptor
	SourceRevision string `json:"source_revision"`
	DocumentDigest string `json:"document_digest"`
}

type PublicationInput struct {
	Descriptor PublicationDescriptor
	Document   SourceDocument
}

// SourceAccessSnapshot retains a Core compilation input separately from the
// complete read corpus. A DPF never enters CompileBaseTypeEnv through this API.
type SourceAccessSnapshot struct {
	core     PublicationSnapshot
	manifest []PublicationManifestEntry
	units    []SourceUnit
}

func (snapshot SourceAccessSnapshot) Core() PublicationSnapshot { return snapshot.core }
func (snapshot SourceAccessSnapshot) SourceUnits() []SourceUnit {
	return cloneSourceUnits(snapshot.units)
}
func (snapshot SourceAccessSnapshot) Manifest() []PublicationManifestEntry {
	return slices.Clone(snapshot.manifest)
}

func (snapshot SourceAccessSnapshot) ManifestJSON() ([]byte, error) {
	return json.Marshal(snapshot.manifest)
}

var dpfPatternHeading = regexp.MustCompile(`^([A-Z]{2,8}\.[0-9]+(?:\.[A-Za-z0-9]+)*)\s+[-—–]\s+(.+)$`)
var dpfSectionHeading = regexp.MustCompile(`^([A-Z]{2,8}\.[0-9]+(?:\.[A-Za-z0-9]+)*(?::[A-Za-z0-9.]+)?)\s+[-—–]\s+`)
var suitePublicationLink = regexp.MustCompile(`\]\(([A-Z][A-Z-]*-PRINCIPLES-FRAMEWORK\.md)\)`)

func BuildSourceAccessSnapshot(core PublicationSnapshot, inputs []PublicationInput) (SourceAccessSnapshot, error) {
	units := core.SourceUnits()
	coreDocuments := []PublicationInput{
		{Descriptor: PublicationDescriptor{ID: "fpf-ecosystem", Kind: "ecosystem_navigation", Namespace: "FPF-ECOSYSTEM", Path: "Readme.md"}, Document: core.Readme()},
		{Descriptor: PublicationDescriptor{ID: "fpf-core", Kind: "fpf_core", Namespace: "FPF", Path: "FPF-Spec.md"}, Document: core.Spec()},
	}
	manifest := make([]PublicationManifestEntry, 0, len(inputs)+2)
	for _, input := range coreDocuments {
		entry := publicationManifestEntry(input)
		manifest = append(manifest, entry)
		units = attachPublicationToUnits(units, entry, input.Document.Path)
	}
	readme := coreDocuments[0]
	readmeUnits, err := buildPublicationNavigation(readme)
	if err != nil {
		return SourceAccessSnapshot{}, err
	}
	units = append(units, readmeUnits...)
	seenIDs := map[string]bool{"fpf-core": true, "fpf-ecosystem": true}
	seenNamespaces := map[string]bool{"FPF": true, "FPF-ECOSYSTEM": true}
	seenPaths := map[string]bool{"FPF-Spec.md": true, "Readme.md": true}
	for _, input := range inputs {
		descriptor := input.Descriptor
		if seenIDs[descriptor.ID] || seenNamespaces[descriptor.Namespace] || seenPaths[descriptor.Path] {
			return SourceAccessSnapshot{}, fmt.Errorf("duplicate source publication identity, namespace or path: %s", descriptor.ID)
		}
		if input.Document.SourceRevision != core.Revision() {
			return SourceAccessSnapshot{}, fmt.Errorf("publication %s revision differs from Core", descriptor.ID)
		}
		seenIDs[descriptor.ID] = true
		seenNamespaces[descriptor.Namespace] = true
		seenPaths[descriptor.Path] = true
		derived, err := buildPublicationNavigation(input)
		if err != nil {
			return SourceAccessSnapshot{}, err
		}
		units = append(units, derived...)
		manifest = append(manifest, publicationManifestEntry(input))
	}
	slices.SortFunc(manifest, func(left, right PublicationManifestEntry) int { return strings.Compare(left.ID, right.ID) })
	if err := ValidateSourceUnits(units); err != nil {
		return SourceAccessSnapshot{}, err
	}
	return SourceAccessSnapshot{core: core, manifest: manifest, units: units}, nil
}

func publicationManifestEntry(input PublicationInput) PublicationManifestEntry {
	digest := digestSourceDocument(input.Document)
	return PublicationManifestEntry{PublicationDescriptor: input.Descriptor, SourceRevision: input.Document.SourceRevision, DocumentDigest: digest.String()}
}

func attachPublication(provenance SourceProvenance, entry PublicationManifestEntry) SourceProvenance {
	provenance.PublicationID = entry.ID
	provenance.PublicationKind = entry.Kind
	provenance.Namespace = entry.Namespace
	provenance.DocumentDigest = entry.DocumentDigest
	return provenance
}

func attachPublicationToUnits(units []SourceUnit, entry PublicationManifestEntry, path string) []SourceUnit {
	projected := make([]SourceUnit, len(units))
	for index, unit := range units {
		projected[index] = unit
		if unit.Provenance.SourcePath != path {
			continue
		}
		projected[index] = attachUnitPublication(unit, entry)
	}
	return projected
}

func attachUnitPublication(unit SourceUnit, entry PublicationManifestEntry) SourceUnit {
	projected := cloneSourceUnit(unit)
	projected.Provenance = attachPublication(projected.Provenance, entry)
	for ordinal, relation := range projected.Relations {
		projected.Relations[ordinal].Provenance = attachPublication(relation.Provenance, entry)
	}
	return projected
}

func buildPublicationNavigation(input PublicationInput) ([]SourceUnit, error) {
	document := input.Document
	descriptor := input.Descriptor
	atlas, err := BuildPatternAtlas(document.Markdown, document.Path, document.SourceRevision)
	if err != nil {
		return nil, err
	}
	if len(atlas.Nodes) == 0 {
		return nil, fmt.Errorf("publication %s has no root", descriptor.ID)
	}
	root := atlas.Nodes[0]
	if root.Level != 1 || root.StartLine != 1 {
		return nil, fmt.Errorf("publication %s must start with its H1 root", descriptor.ID)
	}
	if descriptor.Root != "" && root.Heading != descriptor.Root {
		return nil, fmt.Errorf("publication %s has unsupported root %q", descriptor.ID, root.Heading)
	}
	rootCount := 0
	for _, node := range atlas.Nodes {
		if node.Level == 1 && node.Heading == root.Heading {
			rootCount++
		}
	}
	if rootCount != 1 {
		return nil, fmt.Errorf("publication %s has duplicate roots", descriptor.ID)
	}
	lines := splitPatternAtlasLines(document.Markdown)
	whole := newSourceUnit("publication:"+descriptor.ID, descriptor.ID, SourceUnitRolePublication, root.Heading, string(document.Markdown), "", "", document, 1, len(lines))
	units := []SourceUnit{whole}
	patternCount := 0
	parent := ""
	parentEnd := 0
	for _, node := range atlas.Nodes {
		match := dpfPatternHeading.FindStringSubmatch(node.Heading)
		if descriptor.Kind == "engineering_dpf" && node.Level == 2 && len(match) == 3 {
			id := match[1]
			if !strings.HasPrefix(id, descriptor.Namespace+".") {
				return nil, fmt.Errorf("publication %s contains foreign pattern root %s", descriptor.ID, id)
			}
			body := patternAtlasLineRange(lines, node.StartLine, node.EndLine)
			unit := newSourceUnit("dpf:"+descriptor.Namespace+":pattern_body:"+id, id, SourceUnitRolePatternBody, match[2], body, "", "", document, node.StartLine, node.EndLine)
			unit.PatternID = id
			unit.Keywords = sourceKeywords(node.Heading, body)
			units = append(units, unit)
			parent = id
			parentEnd = node.EndLine
			patternCount++
			continue
		}
		if parent != "" && node.StartLine <= parentEnd {
			id := ""
			sectionMatch := dpfSectionHeading.FindStringSubmatch(node.Heading)
			if len(sectionMatch) == 2 {
				id = sectionMatch[1]
			}
			unit := newSourceUnit("dpf:"+descriptor.Namespace+":section:"+node.NodeID, id, SourceUnitRolePatternSection, node.Heading, node.Body, "", "", document, node.StartLine, node.OwnEndLine)
			unit.PatternID = id
			unit.ParentPatternID = parent
			units = append(units, unit)
			continue
		}
		body := patternAtlasLineRange(lines, node.StartLine, node.EndLine)
		unit := newSourceUnit("navigation:"+descriptor.ID+":"+node.NodeID, "", SourceUnitRoleNavigation, node.Heading, body, "", "", document, node.StartLine, node.EndLine)
		units = append(units, unit)
	}
	if descriptor.Kind == "engineering_dpf" && patternCount == 0 {
		return nil, fmt.Errorf("publication %s contains no patterns", descriptor.ID)
	}
	rows, err := buildDPFTOCRows(input, atlas, units)
	if err != nil {
		return nil, err
	}
	units = append(units, rows...)
	entry := publicationManifestEntry(input)
	units = attachPublicationToUnits(units, entry, document.Path)
	return units, nil
}

// ValidateEngineeringSuiteMembership checks the source-owned declared list
// against the explicit ingestion manifest. New members require a reviewed
// manifest change; arbitrary Markdown links never become executable inputs.
func ValidateEngineeringSuiteMembership(readme []byte) error {
	lines := splitPatternAtlasLines(readme)
	nodes, _ := parsePatternAtlasNodes(lines, "Engineering DPF Suite/README.md", "membership-inspection")
	isMembershipRoot := func(node PatternAtlasNode) bool { return node.Level == 3 && node.Heading == "Published DPFs" }
	if countAtlasRoots(nodes, isMembershipRoot) != 1 {
		return fmt.Errorf("suite requires exactly one Published DPFs membership list")
	}
	root, _ := findAtlasNode(nodes, isMembershipRoot)
	declared := map[string]bool{}
	for _, line := range lines[root.StartLine-1 : root.EndLine] {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		match := suitePublicationLink.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		if declared[match[1]] {
			return fmt.Errorf("suite declares duplicate publication %s", match[1])
		}
		declared[match[1]] = true
	}
	for _, descriptor := range EngineeringPublications() {
		if descriptor.Kind != "engineering_dpf" {
			continue
		}
		file := strings.TrimPrefix(descriptor.Path, "Engineering DPF Suite/")
		if !declared[file] {
			return fmt.Errorf("suite membership is missing declared publication %s", file)
		}
		delete(declared, file)
	}
	if len(declared) != 0 {
		return fmt.Errorf("suite has unsupported declared publications: %v", declared)
	}
	return nil
}

func buildDPFTOCRows(input PublicationInput, atlas PatternAtlas, units []SourceUnit) ([]SourceUnit, error) {
	if input.Descriptor.Kind != "engineering_dpf" {
		return nil, nil
	}
	if countAtlasRoots(atlas.Nodes, isTOCRoot) != 1 {
		return nil, fmt.Errorf("publication %s requires exactly one Table of Contents root", input.Descriptor.ID)
	}
	root, exists := findAtlasNode(atlas.Nodes, isTOCRoot)
	if !exists {
		return nil, fmt.Errorf("publication %s has no Table of Contents", input.Descriptor.ID)
	}
	patterns := map[string]string{}
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternBody {
			patterns[unit.PatternID] = unit.Title
		}
	}
	prefix := regexp.QuoteMeta(input.Descriptor.Namespace)
	rowID := regexp.MustCompile(`\[(` + prefix + `\.[0-9]+(?:\.[A-Za-z0-9]+)*)\s+[-—–]`)
	lines := splitPatternAtlasLines(input.Document.Markdown)
	rows := make([]SourceUnit, 0, len(patterns))
	seen := map[string]bool{}
	for line := root.StartLine; line <= root.EndLine; line++ {
		body := lines[line-1]
		if !strings.HasPrefix(strings.TrimSpace(body), "|") {
			continue
		}
		match := rowID.FindStringSubmatch(body)
		if len(match) != 2 {
			continue
		}
		id := match[1]
		title, exists := patterns[id]
		if !exists || seen[id] {
			return nil, fmt.Errorf("publication %s ToC has missing or duplicate body %s", input.Descriptor.ID, id)
		}
		seen[id] = true
		unit := newSourceUnit("dpf:"+input.Descriptor.Namespace+":toc:"+id, "", SourceUnitRoleTOCRow, title, body, "", "", input.Document, line, line)
		unit.PatternID = id
		unit.Keywords = sourceKeywords(title, body)
		rows = append(rows, unit)
	}
	if len(rows) != len(patterns) {
		return nil, fmt.Errorf("publication %s has %d ToC rows for %d bodies", input.Descriptor.ID, len(rows), len(patterns))
	}
	return rows, nil
}
