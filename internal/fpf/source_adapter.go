package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

type SourceDocument struct {
	Path           string
	SourceRevision string
	Markdown       []byte
}

type SourceBundle struct {
	Readme SourceDocument
	Spec   SourceDocument
}

var sourcePatternLinkRE = regexp.MustCompile(`(?i)\b[A-K]\.[A-Za-z0-9]+(?:[.:][A-Za-z0-9]+)*\b`)
var sourceHeadingPatternIDRE = regexp.MustCompile(`(?i)^\s*[` + "`" + `*]*([A-K]\.[A-Za-z0-9]+(?:[.:][A-Za-z0-9]+)*)\b`)

// LoadSourceUnits loads the upstream monolith and its standalone README
// companion, then derives the query corpus. A missing VCS revision becomes an
// explicit content revision.
func LoadSourceUnits(readmePath, specPath, sourceRevision string) ([]SourceUnit, error) {
	snapshot, err := LoadPublicationSnapshot(readmePath, specPath, sourceRevision)
	if err != nil {
		return nil, err
	}
	return snapshot.SourceUnits(), nil
}

// BuildSourceUnits parses only source-owned structures. The embedded README in
// FPF-Spec.md owns the current practical-use card source units. The standalone
// Readme.md remains an exact snapshotted companion carrier, but it is not a
// duplicate semantic authority and need not be byte-identical to the embedded
// publication unit. ToC references and both carrier roots still fail closed.
func BuildSourceUnits(bundle SourceBundle) ([]SourceUnit, error) {
	if err := validateSourceBundle(bundle); err != nil {
		return nil, err
	}

	specAtlas, err := BuildPatternAtlas(
		bundle.Spec.Markdown,
		bundle.Spec.Path,
		bundle.Spec.SourceRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("parse FPF specification structure: %w", err)
	}

	readmeAtlas, err := BuildPatternAtlas(
		bundle.Readme.Markdown,
		bundle.Readme.Path,
		bundle.Readme.SourceRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("parse FPF README structure: %w", err)
	}
	if err := validateReadmeCarrierRoots(readmeAtlas, specAtlas); err != nil {
		return nil, err
	}

	publicationCatalog, err := parseSourceTOCCatalog(bundle.Spec, specAtlas)
	if err != nil {
		return nil, fmt.Errorf("parse FPF table of contents catalog: %w", err)
	}
	bodyCatalog := sourcePatternCatalog(publicationCatalog, specAtlas)

	practical, err := buildPracticalUseSourceUnits(bundle.Spec, specAtlas)
	if err != nil {
		return nil, err
	}
	preface, err := buildPrefaceSourceUnits(bundle.Spec, specAtlas)
	if err != nil {
		return nil, err
	}
	toc, err := buildTOCSourceUnits(bundle.Spec, specAtlas, bodyCatalog)
	if err != nil {
		return nil, err
	}
	bodies, sections, err := buildPatternSourceUnits(bundle.Spec, specAtlas)
	if err != nil {
		return nil, err
	}
	scopes, err := buildPatternScopeSourceUnits(bundle.Spec, specAtlas)
	if err != nil {
		return nil, err
	}
	bodies, toc, err = projectTOCRelationsToCanonicalUnits(bodies, toc, publicationCatalog)
	if err != nil {
		return nil, err
	}

	units := make([]SourceUnit, 0, len(practical)+len(preface)+len(toc)+len(bodies)+len(sections)+len(scopes))
	units = append(units, practical...)
	units = append(units, preface...)
	units = append(units, toc...)
	units = append(units, bodies...)
	units = append(units, sections...)
	units = append(units, scopes...)
	removeAmbiguousSourceIDs(units)
	resolveSourceDirectRefs(units)

	if err := ValidateSourceUnits(units); err != nil {
		return nil, err
	}
	if err := validateSourceReferences(units, publicationCatalog); err != nil {
		return nil, err
	}
	return units, nil
}

func parseSourceTOCCatalog(document SourceDocument, atlas PatternAtlas) (map[string]SpecCatalogEntry, error) {
	root, ok := findAtlasNode(atlas.Nodes, isTOCRoot)
	if !ok {
		return nil, fmt.Errorf("FPF specification grammar: Table of Content H1 not found")
	}
	lines := splitPatternAtlasLines(document.Markdown)
	tocBody := patternAtlasLineRange(lines, root.StartLine, root.EndLine)
	catalog, err := ParseSpecCatalog(strings.NewReader(tocBody))
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

// projectTOCRelationsToCanonicalUnits is a pure cross-publication projection.
// A published pattern projects onto its body; a planned pattern without a
// body remains on its ToC row. Every relation keeps that exact row as source
// provenance, and publication later moves the derived edge without migration.
func projectTOCRelationsToCanonicalUnits(bodies, tocRows []SourceUnit, catalog map[string]SpecCatalogEntry) ([]SourceUnit, []SourceUnit, error) {
	projectedBodies := append([]SourceUnit(nil), bodies...)
	projectedTOC := append([]SourceUnit(nil), tocRows...)
	tocIndexByPatternID := make(map[string]int)
	for index, tocRow := range projectedTOC {
		if tocRow.PatternID == "" {
			continue
		}
		if _, exists := tocIndexByPatternID[tocRow.PatternID]; exists {
			return nil, nil, fmt.Errorf("FPF relation projection: duplicate ToC row for %s", tocRow.PatternID)
		}
		tocIndexByPatternID[tocRow.PatternID] = index
	}

	bodyIndexByPatternID := make(map[string]int, len(projectedBodies))
	addressablePatterns := make(map[string]struct{}, len(projectedBodies)+len(projectedTOC))
	for index, body := range projectedBodies {
		bodyIndexByPatternID[body.PatternID] = index
		addressablePatterns[body.PatternID] = struct{}{}
	}
	for _, tocRow := range projectedTOC {
		if tocRow.PatternID != "" {
			addressablePatterns[tocRow.PatternID] = struct{}{}
		}
	}

	for patternID, entry := range catalog {
		if len(entry.Edges) == 0 {
			continue
		}
		tocIndex, exists := tocIndexByPatternID[patternID]
		if !exists {
			return nil, nil, fmt.Errorf("FPF relation projection: pattern %s has no ToC provenance", patternID)
		}
		tocRow := projectedTOC[tocIndex]

		relations := make([]SourceRelation, 0, len(entry.Edges))
		for _, edge := range entry.Edges {
			kind, known := sourceRelationKind(edge.EdgeType)
			if !known {
				continue
			}
			if edge.FromPatternID != patternID {
				return nil, nil, fmt.Errorf("FPF relation projection: edge subject %s does not match catalog pattern %s", edge.FromPatternID, patternID)
			}
			targetClass := SourceRelationTargetClassLocalPattern
			_, targetExists := addressablePatterns[edge.ToPatternID]
			if !targetExists {
				switch {
				case isWildcardPatternReference(edge.ToPatternID):
					targetClass = SourceRelationTargetClassAuthoredNonlocal
				case sourcePatternBodyCorroboratesReference(
					projectedBodies,
					bodyIndexByPatternID,
					patternID,
					edge.ToPatternID,
				):
					targetClass = SourceRelationTargetClassUnresolvedAuthored
				default:
					return nil, nil, fmt.Errorf("FPF relation projection: %s %s targets missing publication pattern %s", patternID, kind, edge.ToPatternID)
				}
			}
			relations = append(relations, SourceRelation{
				Kind:            kind,
				TargetPatternID: edge.ToPatternID,
				TargetClass:     targetClass,
				Origin:          SourceRelationOriginTOCExplicit,
				Provenance:      tocRow.Provenance,
			})
		}

		if bodyIndex, hasBody := bodyIndexByPatternID[patternID]; hasBody {
			projectedBodies[bodyIndex].Relations = relations
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(tocRow.PublicationStatus), "planned") {
			return nil, nil, fmt.Errorf("FPF relation projection: non-planned pattern %s has no canonical body", patternID)
		}
		projectedTOC[tocIndex].Relations = relations
	}
	return projectedBodies, projectedTOC, nil
}

func sourcePatternBodyCorroboratesReference(
	bodies []SourceUnit,
	bodyIndexByPatternID map[string]int,
	subjectPatternID string,
	targetPatternID string,
) bool {
	bodyIndex, exists := bodyIndexByPatternID[subjectPatternID]
	if !exists {
		return false
	}
	refs := extractSourcePatternLinks(bodies[bodyIndex].Body)
	for _, ref := range refs {
		if ref == targetPatternID {
			return true
		}
	}
	return false
}

func isWildcardPatternReference(patternID string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(patternID))
	segments := strings.FieldsFunc(normalized, func(character rune) bool {
		return character == '.' || character == ':'
	})
	for _, segment := range segments {
		if segment == "X" {
			return true
		}
	}
	return false
}

func sourceRelationKind(edgeType SpecEdgeType) (SourceRelationKind, bool) {
	kinds := map[SpecEdgeType]SourceRelationKind{
		SpecEdgeTypeBuildsOn:        SourceRelationKindBuildsOn,
		SpecEdgeTypePrerequisiteFor: SourceRelationKindPrerequisiteFor,
		SpecEdgeTypeCoordinatesWith: SourceRelationKindCoordinatesWith,
		SpecEdgeTypeConstrains:      SourceRelationKindConstrains,
		SpecEdgeTypeInforms:         SourceRelationKindInforms,
		SpecEdgeTypeUsedBy:          SourceRelationKindUsedBy,
		SpecEdgeTypeRefines:         SourceRelationKindRefines,
		SpecEdgeTypeSpecialisedBy:   SourceRelationKindSpecialisedBy,
	}
	kind, exists := kinds[edgeType]
	return kind, exists
}

func removeAmbiguousSourceIDs(units []SourceUnit) {
	positions := make(map[string][]int)
	for index, unit := range units {
		if unit.SourceID != "" {
			positions[strings.ToLower(unit.SourceID)] = append(positions[strings.ToLower(unit.SourceID)], index)
		}
	}

	for _, indexes := range positions {
		if len(indexes) <= 1 {
			continue
		}
		canonical := -1
		for _, index := range indexes {
			role := units[index].Role
			if role == SourceUnitRolePatternBody ||
				role == SourceUnitRolePatternScope ||
				(role == SourceUnitRoleTOCRow && units[index].PublicationStatus == "Planned") {
				if canonical >= 0 {
					canonical = -1
					break
				}
				canonical = index
			}
		}
		for _, index := range indexes {
			if index != canonical {
				units[index].SourceID = ""
			}
		}
	}
}

func validateSourceBundle(bundle SourceBundle) error {
	documents := []SourceDocument{bundle.Readme, bundle.Spec}
	for _, document := range documents {
		if strings.TrimSpace(document.Path) == "" {
			return fmt.Errorf("source path is required")
		}
		if strings.TrimSpace(document.SourceRevision) == "" {
			return fmt.Errorf("source revision is required for %s", document.Path)
		}
		if len(document.Markdown) == 0 {
			return fmt.Errorf("source markdown is empty for %s", document.Path)
		}
	}
	if bundle.Readme.SourceRevision != bundle.Spec.SourceRevision {
		return fmt.Errorf("README and specification revisions differ: %q != %q", bundle.Readme.SourceRevision, bundle.Spec.SourceRevision)
	}
	return nil
}

func validateReadmeCarrierRoots(readmeAtlas, specAtlas PatternAtlas) error {
	standaloneRoot, ok := findAtlasNode(readmeAtlas.Nodes, isStandaloneReadmeRoot)
	if !ok || standaloneRoot.StartLine != 1 {
		return fmt.Errorf("FPF README grammar: expected one leading H1 publication heading")
	}
	embeddedRoot, ok := findAtlasNode(specAtlas.Nodes, isEmbeddedReadmeRoot)
	if !ok {
		return fmt.Errorf("FPF specification grammar: embedded README H1 not found")
	}
	prefaceRoot, ok := findAtlasNode(specAtlas.Nodes, isPrefaceRoot)
	if !ok || prefaceRoot.StartLine <= embeddedRoot.StartLine {
		return fmt.Errorf("FPF specification grammar: Preface H1 must follow embedded README H1")
	}
	return nil
}

func buildPracticalUseSourceUnits(document SourceDocument, atlas PatternAtlas) ([]SourceUnit, error) {
	root, ok := findAtlasNode(atlas.Nodes, isPracticalUseRoot)
	if !ok {
		return nil, fmt.Errorf("FPF README grammar: Practical-Use Cards H2 not found")
	}

	lines := splitPatternAtlasLines(document.Markdown)
	units := make([]SourceUnit, 0)
	for _, node := range atlas.Nodes {
		if node.ParentNodeID != root.NodeID || node.Level != 3 {
			continue
		}

		sourceID, title := splitPracticalUseHeading(node.Heading)
		if sourceID == "" || title == "" {
			return nil, fmt.Errorf("FPF README grammar: practical-use heading %q lacks source id and title", node.Heading)
		}
		body := patternAtlasLineRange(lines, node.StartLine, node.EndLine)
		unit := newSourceUnit(
			"readme:practical_use_card:"+sourceUnitSlug(sourceID),
			sourceID,
			SourceUnitRolePracticalUseCard,
			title,
			body,
			"",
			"",
			document,
			node.StartLine,
			node.EndLine,
		)
		unit.AuthoredPhrases = extractReadmeAuthoredPhrases(body)
		unit.Keywords = sourceKeywords(title, body)
		projection, err := ParsePracticalUseCardSource(PracticalUseCardSource{
			SourceID:       sourceID,
			Title:          title,
			Body:           body,
			SourcePath:     document.Path,
			SourceRevision: document.SourceRevision,
			StartLine:      node.StartLine,
			EndLine:        node.EndLine,
		})
		if err != nil {
			return nil, err
		}
		unit.UseCues = projection.UseCues
		unit.DirectRefs = extractSourcePatternLinks(projection.DirectReferenceText)
		units = append(units, unit)
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("FPF README grammar: Practical-Use Cards contains no H3 cards")
	}
	return units, nil
}

func buildPrefaceSourceUnits(document SourceDocument, atlas PatternAtlas) ([]SourceUnit, error) {
	root, ok := findAtlasNode(atlas.Nodes, isPrefaceRoot)
	if !ok {
		return nil, fmt.Errorf("FPF specification grammar: Preface H1 not found")
	}

	lines := splitPatternAtlasLines(document.Markdown)
	units := make([]SourceUnit, 0)
	for _, node := range atlas.Nodes {
		if node.ParentNodeID != root.NodeID || node.Level != 2 {
			continue
		}
		body := patternAtlasLineRange(lines, node.StartLine, node.EndLine)
		title := cleanMarkdownText(node.Heading)
		units = append(units, newSourceUnit(
			"spec:preface:"+sourceUnitSlug(title),
			sourceUnitSlug(title),
			SourceUnitRolePreface,
			title,
			body,
			"",
			"",
			document,
			node.StartLine,
			node.EndLine,
		))
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("FPF specification grammar: Preface contains no H2 sections")
	}
	return units, nil
}

func buildTOCSourceUnits(document SourceDocument, atlas PatternAtlas, catalog map[string]SpecCatalogEntry) ([]SourceUnit, error) {
	root, ok := findAtlasNode(atlas.Nodes, isTOCRoot)
	if !ok {
		return nil, fmt.Errorf("FPF specification grammar: Table of Content H1 not found")
	}

	lines := splitPatternAtlasLines(document.Markdown)
	units := make([]SourceUnit, 0)
	seenCatalog := make(map[string]struct{})
	for lineIndex := root.StartLine; lineIndex < root.EndLine; lineIndex++ {
		rawLine := lines[lineIndex]
		cells, ok := parseTOCRow(rawLine)
		if !ok {
			continue
		}

		patternID := tocRowPatternID(cells, catalog)
		title := tocRowTitle(cells, patternID, catalog)
		if title == "" {
			continue
		}

		sourceID := patternID
		if _, hasPatternBody := catalog[patternID]; hasPatternBody {
			sourceID = ""
		}
		unitSuffix := sourceUnitSlug(title)
		if patternID != "" {
			unitSuffix = sourceUnitSlug(patternID)
			seenCatalog[patternID] = struct{}{}
		}
		if patternID == "" {
			unitSuffix = fmt.Sprintf("%s:%d", unitSuffix, lineIndex+1)
		}

		unit := newSourceUnit(
			"spec:toc_row:"+unitSuffix,
			sourceID,
			SourceUnitRoleTOCRow,
			title,
			rawLine,
			patternID,
			"",
			document,
			lineIndex+1,
			lineIndex+1,
		)
		unit.PublicationStatus = tocRowStatus(cells)
		if entry, exists := catalog[patternID]; exists {
			unit.AuthoredPhrases = append([]string(nil), entry.Queries...)
			unit.Keywords = append([]string(nil), entry.Keywords...)
		}
		searchVocabulary := tocRowSearchVocabulary(cells)
		if len(unit.AuthoredPhrases) == 0 {
			unit.AuthoredPhrases = parseQueries(searchVocabulary)
		}
		if len(unit.Keywords) == 0 {
			unit.Keywords = parseKeywords(searchVocabulary)
		}
		units = append(units, unit)
	}

	if len(units) == 0 {
		return nil, fmt.Errorf("FPF specification grammar: Table of Content contains no source rows")
	}
	for patternID := range catalog {
		if _, exists := seenCatalog[patternID]; !exists {
			return nil, fmt.Errorf("FPF specification grammar: catalog pattern %s has no structural ToC row", patternID)
		}
	}
	return units, nil
}

func buildPatternSourceUnits(document SourceDocument, atlas PatternAtlas) ([]SourceUnit, []SourceUnit, error) {
	lines := splitPatternAtlasLines(document.Markdown)
	bodies := make([]SourceUnit, 0, len(atlas.Cards))
	sections := make([]SourceUnit, 0, len(atlas.Nodes))

	for _, card := range atlas.Cards {
		body := patternAtlasLineRange(lines, card.CardStartLine, card.CardEndLine)
		unit := newSourceUnit(
			"spec:pattern_body:"+sourceUnitSlug(card.PatternID),
			card.PatternID,
			SourceUnitRolePatternBody,
			card.Title,
			body,
			card.PatternID,
			"",
			document,
			card.CardStartLine,
			card.CardEndLine,
		)
		bodies = append(bodies, unit)

		for _, node := range atlas.Nodes {
			if node.StartLine <= card.CardStartLine || node.StartLine > card.CardEndLine {
				continue
			}
			sourceID := sourceSectionID(node.Heading, card.PatternID)
			section := newSourceUnit(
				"spec:pattern_section:"+sourceUnitSlug(card.PatternID)+":"+node.NodeID,
				sourceID,
				SourceUnitRolePatternSection,
				cleanMarkdownText(node.Heading),
				node.Body,
				sourceID,
				card.PatternID,
				document,
				node.StartLine,
				node.OwnEndLine,
			)
			sections = append(sections, section)
		}
	}
	if len(bodies) == 0 || len(sections) == 0 {
		return nil, nil, fmt.Errorf("FPF specification grammar: pattern bodies and sections are required")
	}
	return bodies, sections, nil
}

func sourceSectionID(heading, parentPatternID string) string {
	match := sourceHeadingPatternIDRE.FindStringSubmatch(heading)
	if len(match) != 2 {
		return ""
	}
	candidate := normalizePatternID(match[1])
	if strings.HasPrefix(candidate, parentPatternID+":") || strings.HasPrefix(candidate, parentPatternID+".") {
		return candidate
	}
	return ""
}

// ValidateSourceUnits checks the derived-index grammar without consulting any
// handcrafted routing table.
func ValidateSourceUnits(units []SourceUnit) error {
	if err := validateUniqueSourceIDs(units); err != nil {
		return err
	}
	counts := make(map[SourceUnitRole]int)
	seenIDs := make(map[string]struct{}, len(units))
	patternBodies := make(map[string]string)
	canonicalPatterns := make(map[string]SourceUnit)
	tocProvenanceByPatternID := make(map[string]SourceProvenance)
	for _, unit := range units {
		if !isSourceUnitRole(unit.Role) {
			return fmt.Errorf("source unit %q has unsupported role %q", unit.UnitID, unit.Role)
		}
		if strings.TrimSpace(unit.UnitID) == "" || strings.TrimSpace(unit.Title) == "" || strings.TrimSpace(unit.Body) == "" {
			return fmt.Errorf("source unit in role %s lacks id, title, or body", unit.Role)
		}
		if _, exists := seenIDs[unit.UnitID]; exists {
			return fmt.Errorf("duplicate source unit id %q", unit.UnitID)
		}
		seenIDs[unit.UnitID] = struct{}{}
		if err := validateSourceProvenance(unit); err != nil {
			return err
		}
		if err := validateRoleSpecificSourceGrammar(unit); err != nil {
			return err
		}
		if unit.Role == SourceUnitRolePatternBody {
			if existing, exists := patternBodies[unit.PatternID]; exists {
				return fmt.Errorf("pattern body %s is ambiguous between %s and %s", unit.PatternID, existing, unit.UnitID)
			}
			patternBodies[unit.PatternID] = unit.UnitID
		}
		if isCanonicalPatternRelationTarget(unit) {
			canonicalPatterns[unit.PatternID] = unit
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID != "" {
			tocProvenanceByPatternID[unit.PatternID] = unit.Provenance
		}
		counts[unit.Role]++
	}
	requiredRoles := []SourceUnitRole{
		SourceUnitRolePracticalUseCard,
		SourceUnitRolePreface,
		SourceUnitRoleTOCRow,
		SourceUnitRolePatternBody,
		SourceUnitRolePatternSection,
	}
	for _, role := range requiredRoles {
		if counts[role] == 0 {
			return fmt.Errorf("source unit grammar produced no %s units", role)
		}
	}
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternSection || unit.Role == SourceUnitRolePatternScope {
			if _, exists := patternBodies[unit.ParentPatternID]; !exists {
				return fmt.Errorf("%s %s has missing parent pattern body %s", unit.Role, unit.UnitID, unit.ParentPatternID)
			}
		}
		if err := validateSourceRelations(unit, canonicalPatterns, tocProvenanceByPatternID); err != nil {
			return err
		}
	}
	return nil
}

func validateSourceRelations(unit SourceUnit, canonicalPatterns map[string]SourceUnit, tocProvenanceByPatternID map[string]SourceProvenance) error {
	if len(unit.Relations) == 0 {
		return nil
	}
	if !isCanonicalPatternRelationTarget(unit) {
		return fmt.Errorf("source unit %s has relations outside a canonical published or planned pattern", unit.UnitID)
	}

	tocProvenance, exists := tocProvenanceByPatternID[unit.PatternID]
	if !exists {
		return fmt.Errorf("canonical pattern %s has relations without a ToC source row", unit.PatternID)
	}
	seen := make(map[string]struct{}, len(unit.Relations))
	for _, relation := range unit.Relations {
		if !isSourceRelationKind(relation.Kind) {
			return fmt.Errorf("canonical pattern %s has unsupported relation kind %q", unit.PatternID, relation.Kind)
		}
		if relation.Origin != SourceRelationOriginTOCExplicit {
			return fmt.Errorf("canonical pattern %s has unsupported relation origin %q", unit.PatternID, relation.Origin)
		}
		if relation.Provenance != tocProvenance {
			return fmt.Errorf("canonical pattern %s relation provenance is not its exact ToC row", unit.PatternID)
		}
		switch relation.TargetClass {
		case SourceRelationTargetClassLocalPattern:
			if _, targetExists := canonicalPatterns[relation.TargetPatternID]; !targetExists {
				return fmt.Errorf("canonical pattern %s relation %s targets missing canonical pattern %s", unit.PatternID, relation.Kind, relation.TargetPatternID)
			}
		case SourceRelationTargetClassAuthoredNonlocal:
			if !isWildcardPatternReference(relation.TargetPatternID) {
				return fmt.Errorf("canonical pattern %s relation %s has invalid authored nonlocal target %s", unit.PatternID, relation.Kind, relation.TargetPatternID)
			}
		case SourceRelationTargetClassUnresolvedAuthored:
			if _, targetExists := canonicalPatterns[relation.TargetPatternID]; targetExists {
				return fmt.Errorf("canonical pattern %s relation %s marks available target %s unresolved", unit.PatternID, relation.Kind, relation.TargetPatternID)
			}
			if isWildcardPatternReference(relation.TargetPatternID) {
				return fmt.Errorf("canonical pattern %s relation %s misclassifies wildcard target %s as unresolved", unit.PatternID, relation.Kind, relation.TargetPatternID)
			}
		default:
			return fmt.Errorf("canonical pattern %s relation %s has unsupported target class %q", unit.PatternID, relation.Kind, relation.TargetClass)
		}
		key := string(relation.Kind) + "\x00" + relation.TargetPatternID
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("canonical pattern %s has duplicate relation %s -> %s", unit.PatternID, relation.Kind, relation.TargetPatternID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func isCanonicalPatternRelationTarget(unit SourceUnit) bool {
	if unit.Role == SourceUnitRolePatternBody {
		return unit.SourceID == unit.PatternID
	}
	return unit.Role == SourceUnitRoleTOCRow &&
		strings.EqualFold(strings.TrimSpace(unit.PublicationStatus), "planned") &&
		unit.SourceID == unit.PatternID
}

func validateRoleSpecificSourceGrammar(unit SourceUnit) error {
	switch unit.Role {
	case SourceUnitRolePatternBody:
		if unit.PatternID == "" || unit.SourceID != unit.PatternID {
			return fmt.Errorf("pattern body %s must have one canonical source id equal to its pattern id", unit.UnitID)
		}
		return nil
	case SourceUnitRolePatternSection:
		if unit.ParentPatternID == "" {
			return fmt.Errorf("pattern section %s lacks parent pattern id", unit.UnitID)
		}
		return nil
	case SourceUnitRolePatternScope:
		if unit.SourceID == "" || unit.ParentPatternID == "" {
			return fmt.Errorf("pattern scope %s lacks source id or parent pattern id", unit.UnitID)
		}
		if unit.PatternID != unit.ParentPatternID {
			return fmt.Errorf("pattern scope %s must retain its governing parent pattern id", unit.SourceID)
		}
		return nil
	case SourceUnitRolePracticalUseCard:
		if unit.SourceID == "" {
			return fmt.Errorf("practical-use card %s lacks source id", unit.UnitID)
		}
	default:
		return nil
	}
	if strings.TrimSpace(unit.UseCues.ConditionText) == "" {
		return fmt.Errorf("practical-use card %s lacks source-owned condition cue", unit.SourceID)
	}
	if strings.TrimSpace(unit.UseCues.FirstResultText) == "" {
		return fmt.Errorf("practical-use card %s lacks source-owned first-result cue", unit.SourceID)
	}
	if strings.TrimSpace(unit.UseCues.StopReturnText) == "" {
		return fmt.Errorf("practical-use card %s lacks source-owned stop/return cue", unit.SourceID)
	}
	if len(unit.DirectRefs) == 0 {
		return fmt.Errorf("practical-use card %s lacks source-owned direct pattern references", unit.SourceID)
	}
	return nil
}

func validateSourceReferences(units []SourceUnit, catalog map[string]SpecCatalogEntry) error {
	patternBodies := make(map[string]struct{})
	tocRows := make(map[string]string)
	addressable := make(map[string]struct{})
	unresolvedByPattern := make(map[string]map[string]struct{})
	for _, unit := range units {
		if unit.Role == SourceUnitRolePatternBody {
			patternBodies[unit.PatternID] = struct{}{}
			addressable[sourceReferenceKey(unit.PatternID)] = struct{}{}
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID != "" {
			tocRows[unit.PatternID] = unit.PublicationStatus
			addressable[sourceReferenceKey(unit.PatternID)] = struct{}{}
		}
		if (unit.Role == SourceUnitRolePatternSection || unit.Role == SourceUnitRolePatternScope) && unit.SourceID != "" {
			addressable[sourceReferenceKey(unit.SourceID)] = struct{}{}
		}
		for _, relation := range unit.Relations {
			if relation.TargetClass != SourceRelationTargetClassUnresolvedAuthored {
				continue
			}
			targets := unresolvedByPattern[unit.PatternID]
			if targets == nil {
				targets = make(map[string]struct{})
				unresolvedByPattern[unit.PatternID] = targets
			}
			targets[relation.TargetPatternID] = struct{}{}
		}
	}

	for patternID, status := range tocRows {
		if _, exists := patternBodies[patternID]; !exists {
			if strings.EqualFold(strings.TrimSpace(status), "planned") {
				continue
			}
			return fmt.Errorf("FPF source reference %s has a ToC row but no pattern body", patternID)
		}
	}
	for patternID := range catalog {
		if _, exists := tocRows[patternID]; !exists {
			return fmt.Errorf("FPF source reference %s has catalog metadata but no ToC row", patternID)
		}
	}

	for _, unit := range units {
		if unit.Role != SourceUnitRolePracticalUseCard && unit.Role != SourceUnitRoleTOCRow {
			continue
		}
		for _, ref := range unit.DirectRefs {
			refKey := sourceReferenceKey(ref)
			if _, exists := addressable[refKey]; !exists {
				if _, sourceGap := unresolvedByPattern[unit.PatternID][ref]; sourceGap {
					continue
				}
				return fmt.Errorf(
					"source_reference_unresolved[%s %s]: directly references missing pattern source %s",
					unit.Role,
					unit.SourceID,
					ref,
				)
			}
		}
	}
	return nil
}

func validateSourceProvenance(unit SourceUnit) error {
	provenance := unit.Provenance
	if strings.TrimSpace(provenance.SourcePath) == "" || strings.TrimSpace(provenance.SourceRevision) == "" {
		return fmt.Errorf("source unit %s lacks source path or revision", unit.UnitID)
	}
	if provenance.StartLine <= 0 || provenance.EndLine < provenance.StartLine {
		return fmt.Errorf("source unit %s has invalid line range %d-%d", unit.UnitID, provenance.StartLine, provenance.EndLine)
	}
	if provenance.ContentHash != sourceContentHash(unit.Body) {
		return fmt.Errorf("source unit %s content hash mismatch", unit.UnitID)
	}
	return nil
}

func newSourceUnit(unitID, sourceID string, role SourceUnitRole, title, body, patternID, parentPatternID string, document SourceDocument, startLine, endLine int) SourceUnit {
	trimmedBody := strings.TrimSpace(body)
	return SourceUnit{
		UnitID:          unitID,
		SourceID:        strings.TrimSpace(sourceID),
		Role:            role,
		Title:           strings.TrimSpace(title),
		Body:            trimmedBody,
		PatternID:       normalizePatternID(patternID),
		ParentPatternID: normalizePatternID(parentPatternID),
		DirectRefs:      nil,
		Provenance: SourceProvenance{
			SourcePath:     document.Path,
			StartLine:      startLine,
			EndLine:        endLine,
			ContentHash:    sourceContentHash(trimmedBody),
			SourceRevision: document.SourceRevision,
		},
	}
}

func extractReadmeAuthoredPhrases(body string) []string {
	phrases := make([]string, 0)
	for _, line := range strings.Split(body, "\n") {
		clean := cleanMarkdownText(line)
		for _, marker := range []string{"Ask first ", "Ask: ", "Ask ", "ask first ", "ask: ", "ask "} {
			index := strings.Index(clean, marker)
			if index < 0 {
				continue
			}
			phrase := strings.TrimSpace(clean[index+len(marker):])
			if phrase != "" {
				phrases = append(phrases, phrase)
			}
			break
		}
	}
	return dedupeStrings(phrases)
}

func sourceKeywords(title, body string) []string {
	words := strings.Fields(strings.ToLower(cleanMarkdownText(title)))
	keywords := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.Trim(word, ".,:;!?()[]{}\"'")
		if len([]rune(word)) >= 3 {
			keywords = append(keywords, word)
		}
	}
	keywords = append(keywords, parseKeywords(body)...)
	return dedupeStrings(keywords)
}

func parseTOCRow(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil, false
	}
	cells := splitMarkdownTableRow(trimmed)
	if len(cells) < 2 || isMarkdownSeparatorRow(cells) || isTOCHeaderRow(cells) {
		return nil, false
	}
	return cells, true
}

func isTOCHeaderRow(cells []string) bool {
	first := strings.ToLower(cleanMarkdownText(cells[0]))
	return strings.Contains(first, "id") && strings.Contains(strings.ToLower(strings.Join(cells, " ")), "status")
}

func tocRowPatternID(cells []string, catalog map[string]SpecCatalogEntry) string {
	for _, cell := range cells[:min(2, len(cells))] {
		clean := cleanMarkdownText(cell)
		if strings.Contains(strings.ToLower(clean), "cluster") {
			continue
		}
		patternID := extractPatternID(clean)
		if _, exists := catalog[patternID]; exists {
			return patternID
		}
		if patternID != "" && isUnknownCanonicalPatternRef(patternID) {
			return patternID
		}
	}
	return ""
}

func sourcePatternCatalog(catalog map[string]SpecCatalogEntry, atlas PatternAtlas) map[string]SpecCatalogEntry {
	cardIDs := make(map[string]struct{}, len(atlas.Cards))
	for _, card := range atlas.Cards {
		cardIDs[card.PatternID] = struct{}{}
	}
	filtered := make(map[string]SpecCatalogEntry)
	for patternID, entry := range catalog {
		if _, exists := cardIDs[patternID]; exists {
			filtered[patternID] = entry
		}
	}
	return filtered
}

func resolveSourceDirectRefs(units []SourceUnit) {
	known := make(map[string]string)
	for _, unit := range units {
		for _, sourceID := range addressableSourceIDs(unit) {
			key := sourceReferenceKey(sourceID)
			known[key] = sourceID
		}
	}

	for index := range units {
		if units[index].Role == SourceUnitRoleTOCRow && strings.Contains(strings.ToLower(units[index].Title), "cluster") {
			units[index].DirectRefs = nil
			continue
		}
		refs := append([]string(nil), units[index].DirectRefs...)
		if units[index].Role != SourceUnitRolePracticalUseCard {
			refText := sourceDirectRefText(units[index])
			refs = extractSourcePatternLinks(refText)
		}
		refs = removeSourceReference(refs, units[index].SourceID)
		refs = removeSourceReference(refs, units[index].PatternID)
		filtered := make([]string, 0, len(refs))
		for _, ref := range refs {
			key := sourceReferenceKey(ref)
			canonical, exists := known[key]
			if exists {
				filtered = append(filtered, canonical)
				continue
			}
			if isDirectRefValidatedRole(units[index].Role) && isUnknownCanonicalPatternRef(ref) {
				filtered = append(filtered, ref)
			}
		}
		units[index].DirectRefs = dedupeStrings(filtered)
	}
}

func addressableSourceIDs(unit SourceUnit) []string {
	switch unit.Role {
	case SourceUnitRolePatternBody, SourceUnitRoleTOCRow:
		if unit.PatternID != "" {
			return []string{unit.PatternID}
		}
	case SourceUnitRolePatternSection, SourceUnitRolePatternScope:
		if unit.SourceID != "" {
			return []string{unit.SourceID}
		}
	}
	return nil
}

func sourceReferenceKey(value string) string {
	trimmed := strings.TrimSpace(value)
	return strings.ToLower(trimmed)
}

func removeSourceReference(values []string, value string) []string {
	valueKey := sourceReferenceKey(value)
	filtered := make([]string, 0, len(values))
	for _, candidate := range values {
		candidateKey := sourceReferenceKey(candidate)
		if valueKey != "" && candidateKey == valueKey {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func sourceDirectRefText(unit SourceUnit) string {
	switch unit.Role {
	case SourceUnitRoleTOCRow:
		return tocDirectRefText(unit.Body)
	default:
		return unit.Body
	}
}

func tocDirectRefText(body string) string {
	cells := splitMarkdownTableRow(body)
	if len(cells) >= 5 {
		return strings.Join(cells[4:], " ")
	}
	return ""
}

func extractSourcePatternLinks(text string) []string {
	matches := sourcePatternLinkRE.FindAllString(text, -1)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		refs = append(refs, normalizePatternID(match))
	}
	return dedupeStrings(refs)
}

func isDirectRefValidatedRole(role SourceUnitRole) bool {
	return role == SourceUnitRolePracticalUseCard || role == SourceUnitRoleTOCRow
}

func isUnknownCanonicalPatternRef(patternID string) bool {
	if len(patternID) < 3 || patternID[0] < 'A' || patternID[0] > 'K' || patternID[1] != '.' {
		return false
	}
	if strings.HasSuffix(patternID, ".X") || strings.HasSuffix(patternID, ":X") {
		return false
	}
	segment := strings.SplitN(patternID[2:], ".", 2)[0]
	for _, char := range segment {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}

func tocRowTitle(cells []string, patternID string, catalog map[string]SpecCatalogEntry) string {
	if entry, exists := catalog[patternID]; exists && strings.TrimSpace(entry.Title) != "" {
		return entry.Title
	}
	if patternID != "" && len(cells) > 1 {
		return cleanMarkdownText(cells[1])
	}
	return cleanMarkdownText(cells[0])
}

func tocRowStatus(cells []string) string {
	if len(cells) >= 3 {
		return cleanMarkdownText(cells[2])
	}
	if len(cells) >= 2 {
		return cleanMarkdownText(cells[1])
	}
	return ""
}

func tocRowSearchVocabulary(cells []string) string {
	const searchVocabularyColumn = 3
	if len(cells) <= searchVocabularyColumn {
		return ""
	}
	return cells[searchVocabularyColumn]
}

func splitPracticalUseHeading(heading string) (string, string) {
	clean := cleanMarkdownText(heading)
	for _, separator := range []string{" - ", " — "} {
		parts := strings.SplitN(clean, separator, 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	return "", ""
}

func findAtlasNode(nodes []PatternAtlasNode, predicate func(PatternAtlasNode) bool) (PatternAtlasNode, bool) {
	for _, node := range nodes {
		if predicate(node) {
			return node, true
		}
	}
	return PatternAtlasNode{}, false
}

func isStandaloneReadmeRoot(node PatternAtlasNode) bool {
	return node.Level == 1 && strings.Contains(strings.ToLower(cleanMarkdownText(node.Heading)), "core conceptual specification")
}

func isEmbeddedReadmeRoot(node PatternAtlasNode) bool {
	heading := strings.ToLower(cleanMarkdownText(node.Heading))
	return node.Level == 1 && strings.Contains(heading, "first principles framework") && strings.Contains(heading, "readme")
}

func isPrefaceRoot(node PatternAtlasNode) bool {
	return node.Level == 1 && strings.HasPrefix(strings.ToLower(cleanMarkdownText(node.Heading)), "preface")
}

func isTOCRoot(node PatternAtlasNode) bool {
	heading := strings.ToLower(cleanMarkdownText(node.Heading))
	return node.Level == 1 && (heading == "table of content" || heading == "table of contents")
}

func isPracticalUseRoot(node PatternAtlasNode) bool {
	heading := strings.ToLower(cleanMarkdownText(node.Heading))
	return node.Level == 2 && (heading == "practical-use cards" || heading == "practical use cards")
}

func sourceUnitSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(cleanMarkdownText(value)))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		letterOrNumber := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if letterOrNumber {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func removeString(values []string, value string) []string {
	filtered := make([]string, 0, len(values))
	for _, candidate := range values {
		if value == "" || candidate != value {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func resolvePublicationRevision(revision string, markdown []byte) string {
	trimmed := strings.TrimSpace(revision)
	if trimmed != "" {
		return trimmed
	}
	return "sha256:" + sourceContentHash(string(markdown))
}

func sourceContentHash(body string) string {
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:])
}
