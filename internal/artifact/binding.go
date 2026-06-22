package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const (
	BindingTargetSymbol            = "symbol"
	BindingTargetRange             = "range"
	BindingTargetModule            = "module"
	BindingTargetGenerated         = "generated"
	BindingTargetCarrier           = "carrier"
	BindingTargetWholeFileFallback = "whole_file_fallback"
	BindingTargetSpecSection       = "spec_section"
	BindingTargetAPIContract       = "api_contract"
	BindingTargetInvariant         = "invariant"

	BindingScopeAuto      = "auto"
	BindingScopeModule    = "module"
	BindingScopeWholeFile = "whole_file"

	BindingResolutionSourceExplicitTargets       = "explicit_targets"
	BindingResolutionSourcePathClassification    = "path_classification"
	BindingResolutionSourceBindingScope          = "binding_scope"
	BindingResolutionSourceExplicitWholeFile     = "explicit_whole_file_scope"
	BindingResolutionSourceSingleSymbolFile      = "single_symbol_file"
	BindingResolutionSourceOperatorDecisionHint  = "operator_or_decision_hint"
	BindingResolutionSourceLanguageAdapterRange  = "language_adapter_range"
	BindingResolutionSourceWholeFileFallback     = "whole_file_fallback"
	BindingResolutionSourceUnknownAdapterPosture = "unknown_language_adapter_posture"
	BindingResolutionSourceMarkdownSection       = "markdown_section_target"
	BindingResolutionSourceTargetMarker          = "target_marker"
)

type BindingResolutionOptions struct {
	ExplicitTargets []BindingTarget
	Hints           []string
	Scope           string
	FallbackReason  string
	DecisionText    string
}

type BindingResolution struct {
	Targets     []BindingTarget
	Symbols     []AffectedSymbol
	Diagnostics []BindingDiagnostic
}

type BindingResolutionError struct {
	Diagnostics []BindingDiagnostic
}

func (err BindingResolutionError) Error() string {
	messages := make([]string, 0, len(err.Diagnostics))
	for _, diagnostic := range err.Diagnostics {
		if diagnostic.Severity != "block" {
			continue
		}
		if diagnostic.FilePath == "" {
			messages = append(messages, diagnostic.Message)
			continue
		}
		messages = append(messages, diagnostic.FilePath+": "+diagnostic.Message)
	}
	if len(messages) == 0 {
		return "binding resolution failed"
	}
	return "binding resolution blocked: " + strings.Join(messages, "; ")
}

func BindingResolutionStrategyOrder() []string {
	return []string{
		BindingResolutionSourceExplicitTargets,
		BindingResolutionSourcePathClassification,
		BindingResolutionSourceBindingScope,
		BindingResolutionSourceExplicitWholeFile,
		BindingResolutionSourceSingleSymbolFile,
		BindingResolutionSourceOperatorDecisionHint,
		BindingResolutionSourceLanguageAdapterRange,
		BindingResolutionSourceWholeFileFallback,
	}
}

func ResolveBindingTargets(
	projectRoot string,
	files []AffectedFile,
	options BindingResolutionOptions,
) (BindingResolution, error) {
	resolution := BindingResolution{}
	scope := strings.TrimSpace(options.Scope)
	if scope == "" {
		scope = BindingScopeAuto
	}

	if len(options.ExplicitTargets) > 0 {
		resolution.Targets = normalizeBindingTargets(attachExplicitTargetEvaluators(projectRoot, options.ExplicitTargets))
		resolution.Symbols = affectedSymbolsFromBindingTargets(resolution.Targets)
		return resolution, nil
	}

	if scope == BindingScopeWholeFile && strings.TrimSpace(options.FallbackReason) == "" {
		diagnostic := BindingDiagnostic{
			Kind:     "missing_fallback_reason",
			Severity: "block",
			Message:  "whole-file binding requires binding_fallback_reason",
		}
		return resolution, BindingResolutionError{Diagnostics: []BindingDiagnostic{diagnostic}}
	}

	for _, file := range files {
		fileResolution := resolveFileBindingTarget(projectRoot, file.Path, options, scope)
		resolution.Targets = append(resolution.Targets, fileResolution.Targets...)
		resolution.Symbols = append(resolution.Symbols, fileResolution.Symbols...)
		resolution.Diagnostics = append(resolution.Diagnostics, fileResolution.Diagnostics...)
	}

	resolution.Targets = normalizeBindingTargets(resolution.Targets)
	resolution.Symbols = normalizeAffectedSymbols(resolution.Symbols)

	if hasBlockingBindingDiagnostics(resolution.Diagnostics) {
		return resolution, BindingResolutionError{Diagnostics: resolution.Diagnostics}
	}

	return resolution, nil
}

func resolveFileBindingTarget(
	projectRoot string,
	relPath string,
	options BindingResolutionOptions,
	scope string,
) BindingResolution {
	if generatedOrIgnoredPath(relPath) {
		return BindingResolution{Targets: []BindingTarget{{
			Kind:             BindingTargetGenerated,
			FilePath:         relPath,
			Reason:           "generated or local runtime path; no code-object binding required",
			ResolutionSource: BindingResolutionSourcePathClassification,
		}}}
	}

	if carrierOnlyPath(relPath) {
		return BindingResolution{Targets: []BindingTarget{{
			Kind:             BindingTargetCarrier,
			FilePath:         relPath,
			Reason:           "carrier path; no code-object binding required",
			ResolutionSource: BindingResolutionSourcePathClassification,
		}}}
	}

	if scope == BindingScopeModule {
		return BindingResolution{Targets: []BindingTarget{moduleBindingTarget(projectRoot, relPath)}}
	}

	if scope == BindingScopeWholeFile {
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, "", options.FallbackReason, BindingResolutionSourceExplicitWholeFile)}}
	}

	support := codebase.InspectFileBindingSupport(projectRoot, relPath)
	switch support.Posture {
	case codebase.BindingSupportSymbols:
		return resolveSymbolFileBinding(support, options)
	case codebase.BindingSupportRangeOnly:
		return BindingResolution{Targets: []BindingTarget{rangeBindingTarget(support.Ranges[0], support.Reason)}}
	case codebase.BindingSupportUnsupportedLanguage, codebase.BindingSupportReadFailed:
		reason := support.Reason
		if reason == "" {
			reason = "symbol and range extraction unavailable"
		}
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, support.Language, reason, support.Posture)}}
	default:
		return BindingResolution{Targets: []BindingTarget{wholeFileFallbackTarget(relPath, support.Language, "unknown language adapter posture", support.Posture)}}
	}
}

func resolveSymbolFileBinding(
	support codebase.FileBindingSupport,
	options BindingResolutionOptions,
) BindingResolution {
	if len(support.Symbols) == 1 && len(options.Hints) == 0 {
		target := symbolBindingTarget(support.Symbols[0], support.Language, BindingResolutionSourceSingleSymbolFile)
		return BindingResolution{
			Targets: []BindingTarget{target},
			Symbols: []AffectedSymbol{affectedSymbolFromSnapshot(support.Symbols[0])},
		}
	}

	matches := matchingBindingSymbols(support.Symbols, append(options.Hints, options.DecisionText))
	if len(matches) > 0 {
		targets := make([]BindingTarget, 0, len(matches))
		symbols := make([]AffectedSymbol, 0, len(matches))
		for _, match := range matches {
			targets = append(targets, symbolBindingTarget(match, support.Language, BindingResolutionSourceOperatorDecisionHint))
			symbols = append(symbols, affectedSymbolFromSnapshot(match))
		}
		return BindingResolution{Targets: targets, Symbols: symbols}
	}

	candidates := make([]string, 0, len(support.Symbols))
	for _, symbol := range support.Symbols {
		candidates = append(candidates, fmt.Sprintf("%s %s:%d", symbol.SymbolKind, symbol.SymbolName, symbol.Line))
	}

	return BindingResolution{
		Diagnostics: []BindingDiagnostic{{
			FilePath: support.FilePath,
			Kind:     "needs_binding_resolution",
			Severity: "block",
			Message:  fmt.Sprintf("multiple parseable symbols found; provide binding_hints or explicit binding_targets (%s)", strings.Join(candidates, ", ")),
		}},
	}
}

func moduleBindingTarget(projectRoot, relPath string) BindingTarget {
	modulePath := filepath.ToSlash(filepath.Dir(relPath))
	if modulePath == "." {
		modulePath = relPath
	}

	hash := sha256.Sum256([]byte(projectRoot + ":" + modulePath))
	return BindingTarget{
		Kind:             BindingTargetModule,
		FilePath:         relPath,
		ModulePath:       modulePath,
		ModuleHash:       hex.EncodeToString(hash[:]),
		Reason:           "explicit module binding scope",
		ResolutionSource: BindingResolutionSourceBindingScope,
	}
}

func symbolBindingTarget(symbol codebase.SymbolSnapshot, language string, source string) BindingTarget {
	return BindingTarget{
		Kind:             BindingTargetSymbol,
		FilePath:         symbol.FilePath,
		Language:         language,
		SymbolName:       symbol.SymbolName,
		SymbolKind:       symbol.SymbolKind,
		Receiver:         symbol.Receiver,
		Line:             symbol.Line,
		EndLine:          symbol.EndLine,
		BodyHash:         symbol.Hash,
		Confidence:       "high",
		ResolutionSource: source,
	}
}

func rangeBindingTarget(snapshot codebase.StableRangeSnapshot, reason string) BindingTarget {
	return BindingTarget{
		Kind:             BindingTargetRange,
		FilePath:         snapshot.FilePath,
		Language:         snapshot.Language,
		Line:             snapshot.StartLine,
		EndLine:          snapshot.EndLine,
		AnchorHash:       snapshot.AnchorHash,
		TextHash:         snapshot.TextHash,
		NearestSymbol:    snapshot.NearestName,
		Reason:           reason,
		Confidence:       "medium",
		ResolutionSource: BindingResolutionSourceLanguageAdapterRange,
	}
}

func wholeFileFallbackTarget(relPath, language, reason, posture string) BindingTarget {
	source := BindingResolutionSourceWholeFileFallback
	if posture == BindingResolutionSourceExplicitWholeFile {
		source = BindingResolutionSourceExplicitWholeFile
	}
	return BindingTarget{
		Kind:             BindingTargetWholeFileFallback,
		FilePath:         relPath,
		Language:         language,
		Reason:           reason,
		WhySymbolFailed:  reason,
		WhyRangeFailed:   reason,
		LanguageSupport:  posture,
		Confidence:       "low",
		ResolutionSource: source,
	}
}

func affectedSymbolFromSnapshot(symbol codebase.SymbolSnapshot) AffectedSymbol {
	return AffectedSymbol{
		FilePath:   symbol.FilePath,
		SymbolName: symbol.SymbolName,
		SymbolKind: symbol.SymbolKind,
		Line:       symbol.Line,
		EndLine:    symbol.EndLine,
		Hash:       symbol.Hash,
	}
}

func affectedSymbolsFromBindingTargets(targets []BindingTarget) []AffectedSymbol {
	symbols := make([]AffectedSymbol, 0, len(targets))
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		symbols = append(symbols, AffectedSymbol{
			FilePath:   target.FilePath,
			SymbolName: target.SymbolName,
			SymbolKind: target.SymbolKind,
			Line:       target.Line,
			EndLine:    target.EndLine,
			Hash:       target.BodyHash,
		})
	}
	return normalizeAffectedSymbols(symbols)
}

func normalizeBindingTargets(targets []BindingTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	seen := make(map[string]struct{})
	for _, target := range targets {
		target.Kind = strings.TrimSpace(target.Kind)
		target.TargetRef = normalizeSemanticTargetRef(target.TargetRef)
		if strings.TrimSpace(target.FilePath) != "" {
			target.FilePath = normalizeProjectPath(target.FilePath)
		}
		target.Language = strings.TrimSpace(target.Language)
		target.SymbolName = strings.TrimSpace(target.SymbolName)
		target.SymbolKind = strings.TrimSpace(target.SymbolKind)
		target.Receiver = strings.TrimSpace(target.Receiver)
		target.BodyHash = strings.TrimSpace(target.BodyHash)
		target.AnchorHash = strings.TrimSpace(target.AnchorHash)
		target.TextHash = strings.TrimSpace(target.TextHash)
		if strings.TrimSpace(target.ModulePath) != "" {
			target.ModulePath = normalizeProjectPath(target.ModulePath)
		}
		target.ModuleHash = strings.TrimSpace(target.ModuleHash)
		target.Reason = strings.TrimSpace(target.Reason)
		target.WhySymbolFailed = strings.TrimSpace(target.WhySymbolFailed)
		target.WhyRangeFailed = strings.TrimSpace(target.WhyRangeFailed)
		target.LanguageSupport = strings.TrimSpace(target.LanguageSupport)
		target.Confidence = strings.TrimSpace(target.Confidence)
		target.ResolutionSource = strings.TrimSpace(target.ResolutionSource)
		if target.ResolutionSource == "" && semanticBindingTargetKind(target.Kind) {
			target.ResolutionSource = BindingResolutionSourceExplicitTargets
		}
		if target.Kind == "" || (target.FilePath == "" && target.TargetRef == "") {
			continue
		}
		key := strings.Join([]string{
			target.Kind,
			target.TargetRef,
			target.FilePath,
			target.SymbolKind,
			target.Receiver,
			target.SymbolName,
			target.ModulePath,
			target.TextHash,
		}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].SymbolName < out[j].SymbolName
	})
	return out
}

func semanticBindingTargetKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case BindingTargetSpecSection, BindingTargetAPIContract, BindingTargetInvariant:
		return true
	default:
		return false
	}
}

func normalizeSemanticTargetRef(ref string) string {
	return strings.TrimSpace(ref)
}

func normalizeAffectedSymbols(symbols []AffectedSymbol) []AffectedSymbol {
	out := make([]AffectedSymbol, 0, len(symbols))
	seen := make(map[string]struct{})
	for _, symbol := range symbols {
		symbol.FilePath = normalizeProjectPath(symbol.FilePath)
		symbol.SymbolName = strings.TrimSpace(symbol.SymbolName)
		symbol.SymbolKind = strings.TrimSpace(symbol.SymbolKind)
		symbol.Hash = strings.TrimSpace(symbol.Hash)
		if symbol.FilePath == "" || symbol.SymbolName == "" {
			continue
		}
		key := symbol.FilePath + "\x00" + symbol.SymbolKind + "\x00" + symbol.SymbolName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, symbol)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].SymbolName < out[j].SymbolName
	})
	return out
}

func attachExplicitTargetEvaluators(projectRoot string, targets []BindingTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, attachExplicitTargetEvaluator(projectRoot, target))
	}
	return out
}

func attachExplicitTargetEvaluator(projectRoot string, target BindingTarget) BindingTarget {
	if !semanticBindingTargetKind(target.Kind) {
		return target
	}
	if strings.TrimSpace(target.FilePath) == "" {
		return target
	}
	if strings.TrimSpace(target.TextHash) != "" {
		return target
	}

	section, ok := extractMarkedTargetRange(projectRoot, target.FilePath, target.TargetRef)
	if !ok {
		markdownSection, markdownOK := extractMarkdownTargetRange(projectRoot, target.FilePath, target.TargetRef)
		if !markdownOK {
			return target
		}
		section = markdownSection
	}
	target.Line = section.StartLine
	target.EndLine = section.EndLine
	target.AnchorHash = section.AnchorHash
	target.TextHash = section.TextHash
	if language, ok := codebase.LanguageForPath(target.FilePath); ok && strings.TrimSpace(target.Language) == "" {
		target.Language = language
	}
	if strings.TrimSpace(target.Confidence) == "" {
		target.Confidence = "medium"
	}
	if strings.TrimSpace(target.ResolutionSource) == "" {
		target.ResolutionSource = section.Source
	}
	return target
}

type markdownTargetRange struct {
	StartLine  int
	EndLine    int
	AnchorHash string
	TextHash   string
	Source     string
}

func extractMarkdownTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	token := semanticTargetLookupToken(targetRef)
	if token == "" {
		return markdownTargetRange{}, false
	}

	content, err := os.ReadFile(filepath.Join(projectRoot, relPath))
	if err != nil {
		return markdownTargetRange{}, false
	}
	lines := splitBindingTextLines(string(content))
	start, end, ok := markdownSpecSectionRange(lines, token)
	if !ok {
		start, end, ok = markdownHeadingRange(lines, token)
	}
	if !ok {
		return markdownTargetRange{}, false
	}
	text := strings.Join(lines[start-1:end], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  start,
		EndLine:    end,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceMarkdownSection,
	}, true
}

func extractMarkedTargetRange(projectRoot, relPath, targetRef string) (markdownTargetRange, bool) {
	targetRef = normalizeSemanticTargetRef(targetRef)
	if targetRef == "" {
		return markdownTargetRange{}, false
	}

	content, err := os.ReadFile(filepath.Join(projectRoot, relPath))
	if err != nil {
		return markdownTargetRange{}, false
	}
	lines := splitBindingTextLines(string(content))
	start := 0
	for index, line := range lines {
		if markedTargetRef(line) != targetRef {
			continue
		}
		start = index + 1
		break
	}
	if start == 0 {
		return markdownTargetRange{}, false
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		if markedTargetRef(lines[index]) != "" {
			end = index
			break
		}
	}
	if end < start {
		end = start
	}

	text := strings.Join(lines[start-1:end], "\n")
	normalized := normalizeBindingRangeText(text)
	anchorHash := sha256.Sum256([]byte(firstBindingNonEmptyLine(normalized)))
	textHash := sha256.Sum256([]byte(normalized))
	return markdownTargetRange{
		StartLine:  start,
		EndLine:    end,
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
		Source:     BindingResolutionSourceTargetMarker,
	}, true
}

func markedTargetRef(line string) string {
	trimmed := strings.TrimSpace(line)
	const marker = "haft-target:"
	index := strings.Index(trimmed, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(trimmed[index+len(marker):])
	if cut := strings.IndexAny(value, " \t"); cut >= 0 {
		value = strings.TrimSpace(value[:cut])
	}
	return normalizeSemanticTargetRef(strings.Trim(value, `"'`))
}

func semanticTargetLookupToken(targetRef string) string {
	ref := strings.TrimSpace(targetRef)
	if ref == "" {
		return ""
	}
	if index := strings.LastIndex(ref, ":"); index >= 0 && index < len(ref)-1 {
		return strings.TrimSpace(ref[index+1:])
	}
	return ref
}

func markdownSpecSectionRange(lines []string, token string) (int, int, bool) {
	for index := 0; index < len(lines); index++ {
		if !strings.Contains(lines[index], "```") || !strings.Contains(lines[index], "spec-section") {
			continue
		}
		fenceStart := index + 1
		fenceEnd := len(lines)
		for closeIndex := index + 1; closeIndex < len(lines); closeIndex++ {
			if strings.HasPrefix(strings.TrimSpace(lines[closeIndex]), "```") {
				fenceEnd = closeIndex + 1
				break
			}
		}
		if !specSectionFenceContainsID(lines[index+1:fenceEnd-1], token) {
			continue
		}
		start := precedingMarkdownHeadingLine(lines, fenceStart)
		if start == 0 {
			start = fenceStart
		}
		end := nextMarkdownHeadingLine(lines, start)
		if end == 0 {
			end = len(lines)
		} else {
			end--
		}
		return start, end, true
	}
	return 0, 0, false
}

func specSectionFenceContainsID(lines []string, token string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"id:", "section_id:"} {
			if !strings.HasPrefix(trimmed, prefix) {
				continue
			}
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), `"'`)
			if value == token {
				return true
			}
		}
	}
	return false
}

func markdownHeadingRange(lines []string, token string) (int, int, bool) {
	for index, line := range lines {
		level, heading, ok := parseBindingMarkdownHeading(line)
		if !ok {
			continue
		}
		if !strings.Contains(heading, token) {
			continue
		}
		start := index + 1
		end := len(lines)
		for next := index + 1; next < len(lines); next++ {
			nextLevel, _, ok := parseBindingMarkdownHeading(lines[next])
			if ok && nextLevel <= level {
				end = next
				break
			}
		}
		return start, end, true
	}
	return 0, 0, false
}

func precedingMarkdownHeadingLine(lines []string, beforeLine int) int {
	for index := beforeLine - 2; index >= 0; index-- {
		if _, _, ok := parseBindingMarkdownHeading(lines[index]); ok {
			return index + 1
		}
	}
	return 0
}

func nextMarkdownHeadingLine(lines []string, afterLine int) int {
	afterLevel, _, ok := parseBindingMarkdownHeading(lines[afterLine-1])
	if !ok {
		afterLevel = 1
	}
	for index := afterLine; index < len(lines); index++ {
		level, _, ok := parseBindingMarkdownHeading(lines[index])
		if ok && level <= afterLevel {
			return index + 1
		}
	}
	return 0
}

func parseBindingMarkdownHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmed[level+1:]), true
}

func splitBindingTextLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func normalizeBindingRangeText(text string) string {
	lines := splitBindingTextLines(text)
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstBindingNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func matchingBindingSymbols(symbols []codebase.SymbolSnapshot, texts []string) []codebase.SymbolSnapshot {
	haystack := strings.ToLower(strings.Join(texts, "\n"))
	if strings.TrimSpace(haystack) == "" {
		return nil
	}
	matches := make([]codebase.SymbolSnapshot, 0, len(symbols))
	for _, symbol := range symbols {
		name := strings.ToLower(strings.TrimSpace(symbol.SymbolName))
		if name == "" {
			continue
		}
		if strings.Contains(haystack, name) {
			matches = append(matches, symbol)
		}
	}
	return matches
}

func hasBlockingBindingDiagnostics(diagnostics []BindingDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "block" {
			return true
		}
	}
	return false
}
