package specmigrationv2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var sourceHeadingPattern = regexp.MustCompile(`^##[ \t]+(ES(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3})(?:[ \t]+.*)?$`)

var sourceTitlePattern = regexp.MustCompile(`^#[ \t]+[^#\r\n].*$`)

var targetHeadingPattern = regexp.MustCompile(`^##[ \t]+(SS(?:\.[A-Za-z0-9][A-Za-z0-9_-]*)+\.[0-9]{3})(?:[ \t]+.*)?$`)

var sourceKindPattern = regexp.MustCompile(`^(?:creator-role|enabling\.[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*)$`)

var softwareSectionKinds = map[string]struct{}{
	"software.role":                      {},
	"software.responsibility_allocation": {},
	"software.functional_behavior":       {},
	"software.procedural_behavior":       {},
	"software.interfaces":                {},
	"software.constraints":               {},
	"software.selected_structure":        {},
}

type carrierLine struct {
	start   uint64
	end     uint64
	content string
}

type fencedSpecSection struct {
	start uint64
	end   uint64
	body  []byte
	data  specSectionYAML
}

type specSectionYAML struct {
	ID     string            `yaml:"id"`
	Spec   string            `yaml:"spec"`
	Kind   string            `yaml:"kind"`
	Status string            `yaml:"status"`
	Claims []atomicClaimYAML `yaml:"claims"`
}

type atomicClaimYAML struct {
	ID        string   `yaml:"id"`
	Class     string   `yaml:"class"`
	Statement string   `yaml:"statement"`
	Scope     []string `yaml:"scope"`
}

func deriveSourceSections(value []byte) ([]SourceSection, error) {
	lines := splitCarrierLines(value)
	headings, err := selectSourceHeadings(lines)
	if err != nil {
		return nil, err
	}
	if len(headings) == 0 {
		return nil, fmt.Errorf("source carrier contains no ## ES.<section> headings")
	}
	if err := validateSourcePrologue(value[:headings[0].start]); err != nil {
		return nil, err
	}
	fences, err := parseFencedSpecSections(value)
	if err != nil {
		return nil, err
	}
	if len(fences) != len(headings) {
		return nil, fmt.Errorf(
			"source carrier has %d ES headings and %d yaml spec-section blocks; exact inventory cannot be derived",
			len(headings),
			len(fences),
		)
	}
	sections := make([]SourceSection, 0, len(headings))
	seen := make(map[string]struct{}, len(headings))
	for index, heading := range headings {
		matches := sourceHeadingPattern.FindStringSubmatch(heading.content)
		id, idErr := NewSourceSectionID(matches[1])
		if idErr != nil {
			return nil, idErr
		}
		if _, exists := seen[id.String()]; exists {
			return nil, fmt.Errorf("derived source section %q is duplicated", id.String())
		}
		seen[id.String()] = struct{}{}
		end := uint64(len(value))
		if index+1 < len(headings) {
			end = headings[index+1].start
		}
		if err := validateSourceFence(id, heading.start, end, fences); err != nil {
			return nil, err
		}
		fragment := value[heading.start:end]
		length, lengthErr := NewByteLength(uint64(len(fragment)))
		if lengthErr != nil {
			return nil, lengthErr
		}
		digest := FragmentDigestOf(fragment)
		span, spanErr := NewExactByteSpan(heading.start, length, digest)
		if spanErr != nil {
			return nil, spanErr
		}
		section, sectionErr := NewSourceSection(id, span)
		if sectionErr != nil {
			return nil, sectionErr
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func parseTargetClaims(value []byte) ([]TargetAtomicClaimID, error) {
	lines := splitCarrierLines(value)
	headings, err := selectTargetHeadings(lines)
	if err != nil {
		return nil, err
	}
	fences, err := parseFencedSpecSections(value)
	if err != nil {
		return nil, err
	}
	if len(fences) == 0 {
		return nil, fmt.Errorf("target carrier contains no yaml spec-section blocks")
	}
	if len(headings) != len(fences) {
		return nil, fmt.Errorf(
			"target carrier has %d SS headings and %d yaml spec-section blocks; exact catalog cannot be derived",
			len(headings),
			len(fences),
		)
	}
	seenSections := make(map[string]struct{}, len(fences))
	seenClaims := map[string]struct{}{}
	claims := []TargetAtomicClaimID{}
	for index, fence := range fences {
		sectionID, sectionErr := NewTargetSectionID(fence.data.ID)
		if sectionErr != nil {
			return nil, fmt.Errorf("target spec-section at byte %d: %w", fence.start, sectionErr)
		}
		if _, exists := seenSections[sectionID.String()]; exists {
			return nil, fmt.Errorf("target section %q is duplicated", sectionID.String())
		}
		seenSections[sectionID.String()] = struct{}{}
		if fence.data.Spec != "software-system" {
			return nil, fmt.Errorf("target section %q must declare spec: software-system", sectionID.String())
		}
		if !validSoftwareSectionKind(fence.data.Kind) {
			return nil, fmt.Errorf("target section %q declares unknown software-system kind %q", sectionID.String(), fence.data.Kind)
		}
		if fence.data.Status != "draft" {
			return nil, fmt.Errorf("target review section %q must declare status: draft", sectionID.String())
		}
		headingMatches := targetHeadingPattern.FindStringSubmatch(headings[index].content)
		if headingMatches[1] != sectionID.String() {
			return nil, fmt.Errorf(
				"target heading %q contains yaml section %q",
				headingMatches[1],
				sectionID.String(),
			)
		}
		sectionEnd := uint64(len(value))
		if index+1 < len(headings) {
			sectionEnd = headings[index+1].start
		}
		if fence.start <= headings[index].start || fence.end > sectionEnd {
			return nil, fmt.Errorf(
				"target section %q yaml block is outside its exact heading boundary",
				sectionID.String(),
			)
		}
		if len(fence.data.Claims) == 0 {
			return nil, fmt.Errorf("target section %q has no atomic claims", sectionID.String())
		}
		for claimIndex, rawClaim := range fence.data.Claims {
			claim, claimErr := parseAtomicClaim(sectionID, claimIndex, rawClaim)
			if claimErr != nil {
				return nil, claimErr
			}
			if _, exists := seenClaims[claim.String()]; exists {
				return nil, fmt.Errorf("target atomic claim %q is duplicated", claim.String())
			}
			seenClaims[claim.String()] = struct{}{}
			claims = append(claims, claim)
		}
	}
	return claims, nil
}

func parseAtomicClaim(
	section TargetSectionID,
	index int,
	raw atomicClaimYAML,
) (TargetAtomicClaimID, error) {
	claim, err := NewTargetAtomicClaimID(raw.ID)
	if err != nil {
		return TargetAtomicClaimID{}, fmt.Errorf("target section %q claim %d: %w", section.String(), index, err)
	}
	if claim.Section().String() != section.String() {
		return TargetAtomicClaimID{}, fmt.Errorf(
			"target atomic claim %q belongs to %q, not enclosing section %q",
			claim.String(),
			claim.Section().String(),
			section.String(),
		)
	}
	class := claimClass(claim)
	if raw.Class != class {
		return TargetAtomicClaimID{}, fmt.Errorf(
			"target atomic claim %q class %q does not match ID class %q",
			claim.String(),
			raw.Class,
			class,
		)
	}
	if strings.TrimSpace(raw.Statement) == "" {
		return TargetAtomicClaimID{}, fmt.Errorf("target atomic claim %q has no statement", claim.String())
	}
	if len(raw.Scope) == 0 {
		return TargetAtomicClaimID{}, fmt.Errorf("target atomic claim %q has no scope", claim.String())
	}
	for _, scope := range raw.Scope {
		if strings.TrimSpace(scope) == "" {
			return TargetAtomicClaimID{}, fmt.Errorf("target atomic claim %q has an empty scope value", claim.String())
		}
	}
	return claim, nil
}

func claimClass(claim TargetAtomicClaimID) string {
	value := claim.String()
	lastDot := strings.LastIndex(value, ".")
	return value[lastDot+1 : lastDot+2]
}

func parseFencedSpecSections(value []byte) ([]fencedSpecSection, error) {
	lines := splitCarrierLines(value)
	result := []fencedSpecSection{}
	blocks, err := scanTopLevelMarkdown(lines)
	if err != nil {
		return nil, err
	}
	for _, block := range blocks.fences {
		if block.info != "yaml spec-section" {
			continue
		}
		openerIndex := block.openerIndex
		closingIndex := block.closingIndex
		line := lines[openerIndex]
		bodyStart := line.end
		bodyEnd := lines[closingIndex].start
		body := append([]byte{}, value[bodyStart:bodyEnd]...)
		var data specSectionYAML
		decoder := yaml.NewDecoder(bytes.NewReader(body))
		decoder.KnownFields(false)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("parse yaml spec-section at byte %d: %w", line.start, err)
		}
		var trailingDocument any
		trailingErr := decoder.Decode(&trailingDocument)
		if trailingErr == nil {
			return nil, fmt.Errorf("yaml spec-section at byte %d contains more than one YAML document", line.start)
		}
		if !errors.Is(trailingErr, io.EOF) {
			return nil, fmt.Errorf("parse trailing yaml spec-section document at byte %d: %w", line.start, trailingErr)
		}
		result = append(result, fencedSpecSection{
			start: line.start,
			end:   lines[closingIndex].end,
			body:  body,
			data:  data,
		})
	}
	return result, nil
}

func splitCarrierLines(value []byte) []carrierLine {
	result := []carrierLine{}
	start := 0
	for start < len(value) {
		relativeEnd := bytes.IndexByte(value[start:], '\n')
		end := len(value)
		if relativeEnd >= 0 {
			end = start + relativeEnd + 1
		}
		contentEnd := end
		if contentEnd > start && value[contentEnd-1] == '\n' {
			contentEnd--
		}
		if contentEnd > start && value[contentEnd-1] == '\r' {
			contentEnd--
		}
		result = append(result, carrierLine{
			start:   uint64(start),
			end:     uint64(end),
			content: string(value[start:contentEnd]),
		})
		start = end
	}
	return result
}

func selectSourceHeadings(lines []carrierLine) ([]carrierLine, error) {
	result := []carrierLine{}
	blocks, err := scanTopLevelMarkdown(lines)
	if err != nil {
		return nil, err
	}
	for _, heading := range blocks.headings {
		if heading.level == 1 && len(result) > 0 {
			return nil, fmt.Errorf(
				"source carrier contains unexpected top-level H1 inside the ES inventory %q",
				heading.line.content,
			)
		}
		if heading.level != 2 {
			continue
		}
		if !sourceHeadingPattern.MatchString(heading.line.content) {
			return nil, fmt.Errorf(
				"source carrier contains unclassified sibling H2 section %q",
				heading.line.content,
			)
		}
		result = append(result, heading.line)
	}
	return result, nil
}

func selectTargetHeadings(lines []carrierLine) ([]carrierLine, error) {
	result := []carrierLine{}
	blocks, err := scanTopLevelMarkdown(lines)
	if err != nil {
		return nil, err
	}
	for _, heading := range blocks.headings {
		if heading.level != 2 {
			continue
		}
		if !targetHeadingPattern.MatchString(heading.line.content) {
			return nil, fmt.Errorf(
				"target carrier contains unclassified sibling H2 section %q",
				heading.line.content,
			)
		}
		result = append(result, heading.line)
	}
	return result, nil
}

type topLevelHeading struct {
	level int
	line  carrierLine
}

type topLevelFence struct {
	openerIndex  int
	closingIndex int
	info         string
}

type topLevelMarkdown struct {
	headings []topLevelHeading
	fences   []topLevelFence
}

func scanTopLevelMarkdown(lines []carrierLine) (topLevelMarkdown, error) {
	result := topLevelMarkdown{headings: []topLevelHeading{}, fences: []topLevelFence{}}
	inHTMLComment := false
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line.content)
		if inHTMLComment {
			if strings.Contains(trimmed, "-->") {
				inHTMLComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") {
			inHTMLComment = !strings.Contains(trimmed, "-->")
			continue
		}
		opener, info := parseFenceOpeningWithInfo(line.content)
		if opener.valid() {
			closingIndex := findClosingFence(lines, index+1, opener)
			if closingIndex < 0 {
				return topLevelMarkdown{}, fmt.Errorf("top-level markdown fence at byte %d is not closed", line.start)
			}
			result.fences = append(result.fences, topLevelFence{
				openerIndex:  index,
				closingIndex: closingIndex,
				info:         info,
			})
			index = closingIndex
			continue
		}
		level := headingLevel(line.content)
		if level == 0 {
			continue
		}
		result.headings = append(result.headings, topLevelHeading{level: level, line: line})
	}
	if inHTMLComment {
		return topLevelMarkdown{}, fmt.Errorf("top-level markdown HTML comment is not closed")
	}
	return result, nil
}

func headingLevel(value string) int {
	if value != strings.TrimLeft(value, " ") {
		return 0
	}
	level := 0
	for level < len(value) && level < 6 && value[level] == '#' {
		level++
	}
	if level == 0 || level >= len(value) {
		return 0
	}
	if value[level] != ' ' && value[level] != '\t' {
		return 0
	}
	return level
}

type fenceOpening struct {
	marker byte
	length int
}

func (opening fenceOpening) valid() bool {
	return (opening.marker == '`' || opening.marker == '~') && opening.length >= 3
}

func parseFenceOpeningWithInfo(value string) (fenceOpening, string) {
	trimmed := strings.TrimLeft(value, " ")
	indent := len(value) - len(trimmed)
	if indent > 3 || len(trimmed) < 3 {
		return fenceOpening{}, ""
	}
	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return fenceOpening{}, ""
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == marker {
		length++
	}
	if length < 3 {
		return fenceOpening{}, ""
	}
	info := strings.TrimSpace(trimmed[length:])
	return fenceOpening{marker: marker, length: length}, info
}

func findClosingFence(
	lines []carrierLine,
	start int,
	opener fenceOpening,
) int {
	for index := start; index < len(lines); index++ {
		trimmed := strings.TrimLeft(lines[index].content, " ")
		indent := len(lines[index].content) - len(trimmed)
		if indent > 3 {
			continue
		}
		length := 0
		for length < len(trimmed) && trimmed[length] == opener.marker {
			length++
		}
		if length >= opener.length && strings.TrimSpace(trimmed[length:]) == "" {
			return index
		}
	}
	return -1
}

func validateSourcePrologue(value []byte) error {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return nil
	}
	if sourceTitlePattern.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("source carrier prologue contains unclassified bytes before the first ES section")
}

func validateSourceFence(
	id SourceSectionID,
	start uint64,
	end uint64,
	fences []fencedSpecSection,
) error {
	matches := []fencedSpecSection{}
	for _, fence := range fences {
		if fence.start >= start && fence.end <= end {
			matches = append(matches, fence)
		}
	}
	if len(matches) != 1 {
		return fmt.Errorf("source section %q must contain exactly one yaml spec-section block", id.String())
	}
	if matches[0].data.ID != id.String() {
		return fmt.Errorf(
			"source heading %q contains yaml section %q",
			id.String(),
			matches[0].data.ID,
		)
	}
	if matches[0].data.Spec != "" && matches[0].data.Spec != "enabling-system" {
		return fmt.Errorf("source section %q must declare spec: enabling-system when spec is present", id.String())
	}
	if !sourceKindPattern.MatchString(matches[0].data.Kind) {
		return fmt.Errorf("source section %q declares invalid enabling-system kind %q", id.String(), matches[0].data.Kind)
	}
	if matches[0].data.Status != "draft" && matches[0].data.Status != "active" {
		return fmt.Errorf("source section %q must declare draft or active lifecycle status", id.String())
	}
	return nil
}

func validSoftwareSectionKind(value string) bool {
	_, exists := softwareSectionKinds[value]
	return exists
}
