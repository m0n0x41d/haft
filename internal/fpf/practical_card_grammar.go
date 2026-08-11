package fpf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PracticalUseCardSource is the domain input to the practical-card grammar.
// It contains source-owned bytes and coordinates only; it performs no file,
// Git, index, or query operation.
type PracticalUseCardSource struct {
	SourceID       string
	Title          string
	Body           string
	SourcePath     string
	SourceRevision string
	StartLine      int
	EndLine        int
}

// SourceBlockKind is the closed semantic classification of a source-owned
// practical-card block. OtherAuthoredBlock preserves authored structure while
// explicitly excluding it from condition/result/boundary projection.
type SourceBlockKind string

const (
	SourceBlockConditionCue            SourceBlockKind = "condition_cue"
	SourceBlockEntryRoute              SourceBlockKind = "entry_route"
	SourceBlockResultBranch            SourceBlockKind = "result_branch"
	SourceBlockResultTest              SourceBlockKind = "result_test"
	SourceBlockConditionalContinuation SourceBlockKind = "conditional_continuation"
	SourceBlockBoundaryCue             SourceBlockKind = "boundary_cue"
	SourceBlockPublicCoarsening        SourceBlockKind = "public_coarsening"
	SourceBlockOtherAuthored           SourceBlockKind = "other_authored_block"
)

// SourceLabeledBlock preserves the source-authored label, body, display text,
// and exact source span used by the structural grammar.
type SourceLabeledBlock struct {
	Label        string
	Kind         SourceBlockKind
	Body         string
	AuthoredText string
	StartLine    int
	EndLine      int
}

// SourceUseCueProjection is the pure result consumed by BuildSourceUnits.
// DirectReferenceText is deliberately limited to admitted navigation/result
// blocks; arbitrary body prose never contributes a direct PatternID.
type SourceUseCueProjection struct {
	Blocks              []SourceLabeledBlock
	UseCues             SourceUseCues
	DirectReferenceText string
}

type SourceGrammarDiagnosticClass string

const (
	SourceGrammarMalformed   SourceGrammarDiagnosticClass = "source_publication_malformed"
	SourceGrammarUnsupported SourceGrammarDiagnosticClass = "adapter_grammar_unsupported"
)

// SourceGrammarDiagnostic is stable, source-coordinate-bearing failure data.
// It reports what the publication contains without interpreting unsupported
// prose as FPF meaning.
type SourceGrammarDiagnostic struct {
	Class                   SourceGrammarDiagnosticClass
	SourceID                string
	Title                   string
	SourcePath              string
	SourceRevision          string
	StartLine               int
	EndLine                 int
	LabelsDiscovered        []string
	LabelsRecognized        []string
	MissingSemanticCategory string
	Detail                  string
}

func (diagnostic SourceGrammarDiagnostic) Error() string {
	return fmt.Sprintf(
		"%s[%s %s:%d-%d revision=%s]: %s; labels_discovered=%q; labels_recognized=%q; missing_semantic_category=%s; reproduce=%q",
		diagnostic.Class,
		diagnostic.SourceID,
		diagnostic.SourcePath,
		diagnostic.StartLine,
		diagnostic.EndLine,
		diagnostic.SourceRevision,
		diagnostic.Detail,
		diagnostic.LabelsDiscovered,
		diagnostic.LabelsRecognized,
		diagnostic.MissingSemanticCategory,
		"go test ./internal/fpf -run 'PracticalUse|SourceUseCue|SourceGrammar|ProductionGrammar' -count=1",
	)
}

var practicalUseLabeledBlockRE = regexp.MustCompile(`^\s*-\s+\*\*(.+?)\.\*\*\s*(.*)$`)
var practicalUseBranchHeadingRE = regexp.MustCompile(`^\s*\*\*(Branch\s+.+?)\.\*\*\s*$`)
var practicalUseBranchResultChildRE = regexp.MustCompile(
	"^\\s*-\\s+(`[^`]*Solution ->[^`]*`.*)\\s*$",
)
var practicalUseLegacySolutionLabelRE = regexp.MustCompile(
	`(?i)^\s*-\s+\*\*((?:Template|Direct|Leading|Branch)\b.+?\bSolution\s*->)\*\*\s*(.*)$`,
)

// ParsePracticalUseCardSource is the pure practical-card grammar port. It
// admits only explicit Markdown structures and exact label families. It never
// searches arbitrary prose for a result or chooses among result branches.
func ParsePracticalUseCardSource(source PracticalUseCardSource) (SourceUseCueProjection, error) {
	blocks, discovered, recognized, unsupported := parsePracticalUseBlocks(source)
	if len(unsupported) > 0 {
		return SourceUseCueProjection{}, sourceGrammarDiagnostic(
			source,
			SourceGrammarUnsupported,
			discovered,
			recognized,
			"admitted_result_block",
			fmt.Sprintf(
				"card contains source-owned result-like structure whose label family is not admitted: %s",
				strings.Join(unsupported, ", "),
			),
		)
	}
	return projectPracticalUseBlocks(source, blocks, discovered, recognized)
}

// ProjectPracticalUseCardSource preserves a source-owned result-like block
// with a new label family as a degraded result projection and returns an exact
// diagnostic for review. Missing core categories remain a hard error because
// no coherent practical-use projection can be produced. Indexing and refresh
// analysis share this domain port so adapters cannot silently disagree about
// the source-owned block projection.
func ProjectPracticalUseCardSource(
	source PracticalUseCardSource,
) (SourceUseCueProjection, []SourceGrammarDiagnostic, error) {
	blocks, discovered, recognized, unsupported := parsePracticalUseBlocks(source)
	diagnostics := make([]SourceGrammarDiagnostic, 0, 2)
	if len(unsupported) > 0 {
		diagnostic := sourceGrammarDiagnostic(
			source,
			SourceGrammarUnsupported,
			discovered,
			recognized,
			"admitted_result_block",
			fmt.Sprintf(
				"card contains source-owned result-like structure whose label family is not admitted: %s",
				strings.Join(unsupported, ", "),
			),
		)
		diagnostics = append(diagnostics, diagnostic)
		blocks = admitUnsupportedResultBlocks(blocks)
	}
	projection, err := projectPracticalUseBlocks(
		source,
		blocks,
		discovered,
		recognized,
	)
	if err != nil {
		var diagnostic SourceGrammarDiagnostic
		if !errors.As(err, &diagnostic) {
			return SourceUseCueProjection{}, diagnostics, err
		}
		diagnostics = append(diagnostics, diagnostic)
		return projection, diagnostics, nil
	}
	return projection, diagnostics, nil
}

func projectPracticalUseBlocks(
	source PracticalUseCardSource,
	blocks []SourceLabeledBlock,
	discovered []string,
	recognized []string,
) (SourceUseCueProjection, error) {

	conditionLines := make([]string, 0)
	resultLines := make([]string, 0)
	boundaryLines := make([]string, 0)
	directReferenceLines := make([]string, 0)
	hasResultBasis := false
	for _, block := range blocks {
		switch block.Kind {
		case SourceBlockConditionCue:
			conditionLines = append(conditionLines, block.Body)
		case SourceBlockEntryRoute:
			hasResultBasis = true
			resultLines = append(resultLines, block.AuthoredText)
			directReferenceLines = append(directReferenceLines, block.AuthoredText)
		case SourceBlockResultBranch:
			if block.Body != "" {
				hasResultBasis = true
			}
			resultLines = append(resultLines, block.AuthoredText)
			directReferenceLines = append(directReferenceLines, block.AuthoredText)
		case SourceBlockConditionalContinuation:
			hasResultBasis = true
			resultLines = append(resultLines, block.AuthoredText)
			directReferenceLines = append(directReferenceLines, block.AuthoredText)
		case SourceBlockResultTest:
			resultLines = append(resultLines, block.AuthoredText)
			directReferenceLines = append(directReferenceLines, block.AuthoredText)
		case SourceBlockBoundaryCue:
			boundaryLines = append(boundaryLines, block.Body)
		case SourceBlockPublicCoarsening, SourceBlockOtherAuthored:
			// These blocks remain inspectable but do not ground exact result
			// or direct-reference projections.
		}
	}

	missing := make([]string, 0, 3)
	if len(conditionLines) == 0 {
		missing = append(missing, "condition")
	}
	if !hasResultBasis {
		missing = append(missing, "first_result")
	}
	if len(boundaryLines) == 0 {
		missing = append(missing, "boundary")
	}
	projection := SourceUseCueProjection{
		Blocks: blocks,
		UseCues: SourceUseCues{
			ConditionText:   strings.Join(dedupeStrings(conditionLines), "\n"),
			FirstResultText: strings.Join(dedupeStrings(resultLines), "\n"),
			StopReturnText:  strings.Join(dedupeStrings(boundaryLines), "\n"),
		},
		DirectReferenceText: strings.Join(dedupeStrings(directReferenceLines), "\n"),
	}
	if len(missing) > 0 {
		return projection, sourceGrammarDiagnostic(
			source,
			SourceGrammarMalformed,
			discovered,
			recognized,
			strings.Join(missing, ","),
			"card lacks one or more required source-owned semantic block categories",
		)
	}
	return projection, nil
}

func admitUnsupportedResultBlocks(
	blocks []SourceLabeledBlock,
) []SourceLabeledBlock {
	result := append([]SourceLabeledBlock(nil), blocks...)
	for index := range result {
		block := result[index]
		if block.Kind != SourceBlockOtherAuthored {
			continue
		}
		_, admitted := classifyPracticalUseLabel(block.Label)
		if admitted {
			continue
		}
		if !looksLikeResultStructure(block.Label, block.Body) {
			continue
		}
		result[index].Kind = SourceBlockResultBranch
	}
	return result
}

func parsePracticalUseBlocks(source PracticalUseCardSource) (
	[]SourceLabeledBlock,
	[]string,
	[]string,
	[]string,
) {
	lines := strings.Split(source.Body, "\n")
	blocks := make([]SourceLabeledBlock, 0)
	discovered := make([]string, 0)
	recognized := make([]string, 0)
	unsupported := make([]string, 0)
	activeBranchLabel := ""

	for index, rawLine := range lines {
		lineNumber := source.StartLine + index
		if source.StartLine <= 0 {
			lineNumber = index + 1
		}

		if match := practicalUseLabeledBlockRE.FindStringSubmatch(rawLine); len(match) == 3 {
			activeBranchLabel = ""
			label := strings.TrimSpace(match[1])
			body := strings.TrimSpace(match[2])
			kind, admitted := classifyPracticalUseLabel(label)
			authoredText := trimPracticalUseBullet(rawLine)
			blocks = append(blocks, SourceLabeledBlock{
				Label:        label,
				Kind:         kind,
				Body:         body,
				AuthoredText: authoredText,
				StartLine:    lineNumber,
				EndLine:      lineNumber,
			})
			discovered = append(discovered, label)
			if admitted {
				recognized = append(recognized, label)
			} else if looksLikeResultStructure(label, body) {
				unsupported = append(unsupported, fmt.Sprintf("%q at line %d", label, lineNumber))
			}
			continue
		}

		if match := practicalUseLegacySolutionLabelRE.FindStringSubmatch(rawLine); len(match) == 3 {
			activeBranchLabel = ""
			label := strings.TrimSpace(match[1])
			body := strings.TrimSpace(match[2])
			blocks = append(blocks, SourceLabeledBlock{
				Label:        label,
				Kind:         SourceBlockResultBranch,
				Body:         body,
				AuthoredText: trimPracticalUseBullet(rawLine),
				StartLine:    lineNumber,
				EndLine:      lineNumber,
			})
			discovered = append(discovered, label)
			recognized = append(recognized, label)
			continue
		}

		if match := practicalUseBranchHeadingRE.FindStringSubmatch(rawLine); len(match) == 2 {
			label := strings.TrimSpace(match[1])
			activeBranchLabel = label
			blocks = append(blocks, SourceLabeledBlock{
				Label:        label,
				Kind:         SourceBlockResultBranch,
				Body:         "",
				AuthoredText: strings.TrimSpace(rawLine),
				StartLine:    lineNumber,
				EndLine:      lineNumber,
			})
			discovered = append(discovered, label)
			recognized = append(recognized, label)
			continue
		}

		if strings.TrimSpace(rawLine) == "" {
			continue
		}
		authoredText := trimPracticalUseBullet(rawLine)
		if activeBranchLabel != "" {
			if match := practicalUseBranchResultChildRE.FindStringSubmatch(rawLine); len(match) == 2 {
				authoredText = strings.TrimSpace(match[1])
				blocks = append(blocks, SourceLabeledBlock{
					Label:        activeBranchLabel,
					Kind:         SourceBlockResultBranch,
					Body:         authoredText,
					AuthoredText: authoredText,
					StartLine:    lineNumber,
					EndLine:      lineNumber,
				})
				continue
			}
			activeBranchLabel = ""
		}
		if strings.Contains(authoredText, "Solution ->") {
			unsupported = append(
				unsupported,
				fmt.Sprintf("unlabeled result line at %d", lineNumber),
			)
		}
	}

	return blocks, dedupeStrings(discovered), dedupeStrings(recognized), dedupeStrings(unsupported)
}

func classifyPracticalUseLabel(label string) (SourceBlockKind, bool) {
	normalized := normalizePracticalUseLabel(label)
	switch normalized {
	case "situation and question":
		return SourceBlockConditionCue, true
	case "first route", "overloaded-word routes":
		return SourceBlockEntryRoute, true
	case "conditional walkthrough", "conditional continuation", "relation-like continuation", "existing-framework continuation":
		return SourceBlockConditionalContinuation, true
	case "result test":
		return SourceBlockResultTest, true
	case "boundaries":
		return SourceBlockBoundaryCue, true
	case "public coarsening":
		return SourceBlockPublicCoarsening, true
	case "optional obstacle":
		return SourceBlockOtherAuthored, true
	case "reusable viewpoint-family branch",
		"architecture-answer branch",
		"optional organization-proposal branch",
		"optional dependency-description branch":
		return SourceBlockResultBranch, true
	}
	for _, family := range []string{"template", "direct", "leading", "branch"} {
		if hasPracticalUseLabelFamily(normalized, family) {
			return SourceBlockResultBranch, true
		}
	}
	return SourceBlockOtherAuthored, false
}

func normalizePracticalUseLabel(label string) string {
	normalized := strings.TrimSpace(strings.ToLower(label))
	normalized = strings.ReplaceAll(normalized, "\u2010", "-")
	normalized = strings.ReplaceAll(normalized, "\u2011", "-")
	normalized = strings.ReplaceAll(normalized, "\u2012", "-")
	normalized = strings.ReplaceAll(normalized, "\u2013", "-")
	normalized = strings.ReplaceAll(normalized, "\u2014", "-")
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

func hasPracticalUseLabelFamily(label, family string) bool {
	return label == family ||
		strings.HasPrefix(label, family+" ") ||
		strings.HasPrefix(label, family+":") ||
		strings.HasPrefix(label, family+"-")
}

func looksLikeResultStructure(label, body string) bool {
	if strings.Contains(body, "Solution ->") {
		return true
	}
	normalized := normalizePracticalUseLabel(label)
	for _, marker := range []string{"result", "route", "template", "branch", "continuation"} {
		if hasPracticalUseLabelFamily(normalized, marker) ||
			strings.Contains(normalized, " "+marker+" ") ||
			strings.HasSuffix(normalized, " "+marker) {
			return true
		}
	}
	return false
}

func trimPracticalUseBullet(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "-") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	}
	return trimmed
}

func sourceGrammarDiagnostic(
	source PracticalUseCardSource,
	class SourceGrammarDiagnosticClass,
	discovered []string,
	recognized []string,
	missing string,
	detail string,
) SourceGrammarDiagnostic {
	return SourceGrammarDiagnostic{
		Class:                   class,
		SourceID:                strings.TrimSpace(source.SourceID),
		Title:                   strings.TrimSpace(source.Title),
		SourcePath:              strings.TrimSpace(source.SourcePath),
		SourceRevision:          strings.TrimSpace(source.SourceRevision),
		StartLine:               source.StartLine,
		EndLine:                 source.EndLine,
		LabelsDiscovered:        append([]string(nil), discovered...),
		LabelsRecognized:        append([]string(nil), recognized...),
		MissingSemanticCategory: missing,
		Detail:                  detail,
	}
}
